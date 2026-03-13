// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package verify implements the archive integrity verification logic for
// the `pixe verify` command.
//
// It walks a sorted dirB, parses the checksum and algorithm embedded in each
// filename, recomputes the data-only hash via the registered FileTypeHandler,
// and reports mismatches.
//
// Two filename formats are supported:
//
//	New (I2+): YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>.<ext>  — algorithm auto-detected from ID
//	Legacy:    YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>        — algorithm inferred from digest length
package verify

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cwlls/pixe/internal/discovery"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/progress"
)

// Result holds the outcome of a verify run.
type Result struct {
	Verified     int // Verified is the number of files whose checksums matched.
	Mismatches   int // Mismatches is the number of files whose checksums did not match.
	Unrecognised int // Unrecognised is the number of files not parseable by any handler.
}

// FileResult is the per-file outcome emitted to the output writer.
type FileResult struct {
	Path     string // Path is the relative path of the file within dirB.
	Status   string // "OK", "MISMATCH", "UNRECOGNISED"
	Expected string // Expected is the checksum parsed from the filename.
	Actual   string // Actual is the checksum recomputed from the file's data.
}

// Options configures a verify run.
type Options struct {
	// Dir is the destination directory (dirB) to verify.
	Dir string
	// Hasher computes content checksums for verification.
	Hasher *hash.Hasher
	// Registry maps file extensions to their FileTypeHandler implementations.
	Registry *discovery.Registry
	// Output is where per-file result lines are written.
	Output io.Writer
	// EventBus, when non-nil, receives structured progress events alongside
	// the plain-text Output writer. Both can be active simultaneously.
	// When nil, no events are emitted (existing behaviour is unchanged).
	EventBus *progress.Bus
	// Workers is the number of concurrent verification workers. When <= 1,
	// verification runs sequentially (existing behaviour). When > 1, a worker
	// pool is used for parallel hash computation.
	Workers int
	// Context, when non-nil, is used for graceful cancellation (e.g. SIGINT).
	// Workers finish their current file before exiting. When nil,
	// context.Background() is used (no cancellation).
	Context context.Context
}

// emitVerify sends an event to the bus if one is configured. It is a no-op
// when bus is nil, so callers do not need to guard every call site.
func emitVerify(bus *progress.Bus, e progress.Event) {
	if bus != nil {
		bus.Emit(e)
	}
}

// verifyItem is sent from the coordinator to a worker.
type verifyItem struct {
	path     string // absolute path
	relPath  string // relative to opts.Dir
	name     string // filename (for parseChecksum)
	fileSize int64  // from os.Stat; 0 if stat failed
}

// verifyResult is sent from a worker back to the coordinator.
type verifyResult struct {
	relPath  string
	absPath  string
	workerID int
	status   string // "OK", "MISMATCH", "UNRECOGNISED", "ERROR"
	expected string
	actual   string
	err      error
}

// Run walks dir, parses checksums from filenames, recomputes hashes, and
// reports results. Returns a non-nil error only for fatal walk errors.
// Per-file mismatches are reported but do not cause Run to return an error —
// callers check Result.Mismatches to determine exit code.
//
// When opts.Workers > 1, Run uses a worker pool for parallel hash computation.
// When opts.Workers <= 1, verification runs sequentially.
func Run(opts Options) (Result, error) {
	if opts.Workers > 1 {
		return runConcurrent(opts)
	}
	return runSequential(opts)
}

// runSequential walks the directory sequentially, verifying each file's
// checksum against the filename-embedded value. It emits events to the bus
// (if configured) and writes per-file results to opts.Output.
func runSequential(opts Options) (Result, error) {
	var result Result

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Pre-scan: count non-directory, non-dotfile entries so consumers can
	// show a determinate progress bar. This is a lightweight walk that does
	// not open any files.
	total := 0
	_ = filepath.WalkDir(opts.Dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // ignore errors in pre-scan
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != opts.Dir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasPrefix(d.Name(), ".") {
			total++
		}
		return nil
	})

	emitVerify(opts.EventBus, progress.Event{
		Kind:  progress.EventVerifyStart,
		Total: total,
	})

	// hasherCache avoids re-allocating hashers for each file in mixed-algorithm archives.
	// Seeded with the configured default hasher.
	hasherCache := map[string]*hash.Hasher{
		opts.Hasher.Algorithm(): opts.Hasher,
	}

	completed := 0
	err := filepath.WalkDir(opts.Dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Check for cancellation between files.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// Skip dot-directories entirely (e.g. .pixe/ containing manifest.json).
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != opts.Dir {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip dotfiles (e.g. .pixe_ledger.json).
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Compute relative path for event reporting.
		relPath, _ := filepath.Rel(opts.Dir, path)

		// Stat for file size (used in progress events).
		var fileSize int64
		if fi, statErr := os.Stat(path); statErr == nil {
			fileSize = fi.Size()
		}

		// Emit file-start event for the UI.
		emitVerify(opts.EventBus, progress.Event{
			Kind:     progress.EventVerifyFileStart,
			RelPath:  relPath,
			AbsPath:  path,
			WorkerID: 0,
			FileSize: fileSize,
		})

		// Parse the expected checksum and algorithm from the filename.
		expected, algo, ok := parseChecksum(name)
		if !ok {
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", path)
			result.Unrecognised++
			completed++
			emitVerify(opts.EventBus, progress.Event{
				Kind:      progress.EventVerifyUnrecognised,
				RelPath:   relPath,
				AbsPath:   path,
				WorkerID:  0,
				Completed: completed,
				Total:     total,
			})
			return nil
		}

		// Resolve the hasher for this file.
		fileHasher := resolveHasher(opts.Hasher, algo, hasherCache)

		// Detect the file type.
		handler, err := opts.Registry.Detect(path)
		if err != nil || handler == nil {
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", path)
			result.Unrecognised++
			completed++
			emitVerify(opts.EventBus, progress.Event{
				Kind:      progress.EventVerifyUnrecognised,
				RelPath:   relPath,
				AbsPath:   path,
				WorkerID:  0,
				Completed: completed,
				Total:     total,
			})
			return nil
		}

		// Recompute the hash via the handler's HashableReader.
		rc, err := handler.HashableReader(path)
		if err != nil {
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", path, err)
			result.Mismatches++
			completed++
			emitVerify(opts.EventBus, progress.Event{
				Kind:             progress.EventVerifyMismatch,
				RelPath:          relPath,
				AbsPath:          path,
				ExpectedChecksum: expected,
				Reason:           err.Error(),
				Err:              err,
				WorkerID:         0,
				Completed:        completed,
				Total:            total,
			})
			return nil
		}
		// Wrap with ProgressReader for byte-level progress (no-op when bus is nil).
		hashReader := progress.NewProgressReader(rc, opts.EventBus, relPath, 0, "HASH", fileSize)
		actual, err := fileHasher.Sum(hashReader)
		_ = rc.Close()
		if err != nil {
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", path, err)
			result.Mismatches++
			completed++
			emitVerify(opts.EventBus, progress.Event{
				Kind:             progress.EventVerifyMismatch,
				RelPath:          relPath,
				AbsPath:          path,
				ExpectedChecksum: expected,
				Reason:           err.Error(),
				Err:              err,
				WorkerID:         0,
				Completed:        completed,
				Total:            total,
			})
			return nil
		}

		completed++
		if actual == expected {
			_, _ = fmt.Fprintf(opts.Output, "  OK            %s\n", path)
			result.Verified++
			emitVerify(opts.EventBus, progress.Event{
				Kind:      progress.EventVerifyOK,
				RelPath:   relPath,
				AbsPath:   path,
				Checksum:  actual,
				WorkerID:  0,
				Completed: completed,
				Total:     total,
			})
		} else {
			_, _ = fmt.Fprintf(opts.Output, "  MISMATCH      %s\n    expected: %s\n    actual:   %s\n",
				path, expected, actual)
			result.Mismatches++
			emitVerify(opts.EventBus, progress.Event{
				Kind:             progress.EventVerifyMismatch,
				RelPath:          relPath,
				AbsPath:          path,
				ExpectedChecksum: expected,
				ActualChecksum:   actual,
				WorkerID:         0,
				Completed:        completed,
				Total:            total,
			})
		}
		return nil
	})

	_, _ = fmt.Fprintf(opts.Output, "\nDone. verified=%d mismatches=%d unrecognised=%d\n",
		result.Verified, result.Mismatches, result.Unrecognised)

	emitVerify(opts.EventBus, progress.Event{
		Kind:  progress.EventVerifyDone,
		Total: total,
		Summary: &progress.RunSummary{
			Verified:     result.Verified,
			Mismatches:   result.Mismatches,
			Unrecognised: result.Unrecognised,
		},
	})

	return result, err
}

// runConcurrent verifies files using a worker pool. The coordinator goroutine
// pre-scans the directory to collect all items and count the total, then feeds
// items to workers; workers own I/O (read + hash) and send results back to the
// coordinator for aggregation and event emission. Workers emit EventByteProgress
// events during hash computation, enabling per-worker progress lines in the UI.
func runConcurrent(opts Options) (Result, error) {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	workers := opts.Workers

	// Pre-scan: collect all items and count total for the progress bar.
	var items []verifyItem
	_ = filepath.WalkDir(opts.Dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != opts.Dir {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		relPath, _ := filepath.Rel(opts.Dir, path)
		var fileSize int64
		if fi, statErr := os.Stat(path); statErr == nil {
			fileSize = fi.Size()
		}
		items = append(items, verifyItem{
			path:     path,
			relPath:  relPath,
			name:     name,
			fileSize: fileSize,
		})
		return nil
	})

	total := len(items)
	emitVerify(opts.EventBus, progress.Event{
		Kind:  progress.EventVerifyStart,
		Total: total,
	})

	workCh := make(chan verifyItem, workers*2)
	resultCh := make(chan verifyResult, workers*2)

	// Launch worker goroutines.
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runVerifyWorker(ctx, workerID, opts, workCh, resultCh)
		}(i + 1) // WorkerIDs start at 1 (0 is reserved for sequential/coordinator)
	}

	// Close resultCh when all workers are done.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Feed items to workers.
	go func() {
		defer close(workCh)
		for _, item := range items {
			select {
			case <-ctx.Done():
				return
			case workCh <- item:
			}
		}
	}()

	// Coordinator: aggregate results and emit events.
	var result Result
	completed := 0
	for res := range resultCh {
		completed++
		switch res.status {
		case "OK":
			_, _ = fmt.Fprintf(opts.Output, "  OK            %s\n", res.absPath)
			result.Verified++
			emitVerify(opts.EventBus, progress.Event{
				Kind:      progress.EventVerifyOK,
				RelPath:   res.relPath,
				AbsPath:   res.absPath,
				Checksum:  res.actual,
				WorkerID:  res.workerID,
				Completed: completed,
				Total:     total,
			})
		case "MISMATCH":
			_, _ = fmt.Fprintf(opts.Output, "  MISMATCH      %s\n    expected: %s\n    actual:   %s\n",
				res.absPath, res.expected, res.actual)
			result.Mismatches++
			emitVerify(opts.EventBus, progress.Event{
				Kind:             progress.EventVerifyMismatch,
				RelPath:          res.relPath,
				AbsPath:          res.absPath,
				ExpectedChecksum: res.expected,
				ActualChecksum:   res.actual,
				WorkerID:         res.workerID,
				Completed:        completed,
				Total:            total,
			})
		case "ERROR":
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", res.absPath, res.err)
			result.Mismatches++
			emitVerify(opts.EventBus, progress.Event{
				Kind:             progress.EventVerifyMismatch,
				RelPath:          res.relPath,
				AbsPath:          res.absPath,
				ExpectedChecksum: res.expected,
				Reason:           res.err.Error(),
				Err:              res.err,
				WorkerID:         res.workerID,
				Completed:        completed,
				Total:            total,
			})
		case "UNRECOGNISED":
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", res.absPath)
			result.Unrecognised++
			emitVerify(opts.EventBus, progress.Event{
				Kind:      progress.EventVerifyUnrecognised,
				RelPath:   res.relPath,
				AbsPath:   res.absPath,
				WorkerID:  res.workerID,
				Completed: completed,
				Total:     total,
			})
		}
	}

	_, _ = fmt.Fprintf(opts.Output, "\nDone. verified=%d mismatches=%d unrecognised=%d\n",
		result.Verified, result.Mismatches, result.Unrecognised)

	emitVerify(opts.EventBus, progress.Event{
		Kind:  progress.EventVerifyDone,
		Total: total,
		Summary: &progress.RunSummary{
			Verified:     result.Verified,
			Mismatches:   result.Mismatches,
			Unrecognised: result.Unrecognised,
		},
	})

	return result, nil
}

// runVerifyWorker processes items from workCh and sends results to resultCh.
// Each worker maintains its own hasher cache to avoid re-allocating hashers
// for each file. Workers emit EventVerifyFileStart and EventByteProgress events
// to the bus (if configured) during hash computation.
func runVerifyWorker(ctx context.Context, workerID int, opts Options, workCh <-chan verifyItem, resultCh chan<- verifyResult) {
	// Per-worker hasher cache to avoid re-allocating hashers for each file.
	hasherCache := map[string]*hash.Hasher{
		opts.Hasher.Algorithm(): opts.Hasher,
	}

	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-workCh:
			if !ok {
				return
			}

			// Emit file-start event for the UI.
			emitVerify(opts.EventBus, progress.Event{
				Kind:     progress.EventVerifyFileStart,
				RelPath:  item.relPath,
				AbsPath:  item.path,
				WorkerID: workerID,
				FileSize: item.fileSize,
			})

			// Parse the expected checksum and algorithm from the filename.
			expected, algo, ok := parseChecksum(item.name)
			if !ok {
				resultCh <- verifyResult{
					relPath:  item.relPath,
					absPath:  item.path,
					workerID: workerID,
					status:   "UNRECOGNISED",
				}
				continue
			}

			// Resolve the hasher for this file.
			fileHasher := resolveHasher(opts.Hasher, algo, hasherCache)

			// Detect the file type.
			handler, err := opts.Registry.Detect(item.path)
			if err != nil || handler == nil {
				resultCh <- verifyResult{
					relPath:  item.relPath,
					absPath:  item.path,
					workerID: workerID,
					status:   "UNRECOGNISED",
				}
				continue
			}

			// Recompute the hash via the handler's HashableReader.
			rc, err := handler.HashableReader(item.path)
			if err != nil {
				resultCh <- verifyResult{
					relPath:  item.relPath,
					absPath:  item.path,
					workerID: workerID,
					status:   "ERROR",
					expected: expected,
					err:      err,
				}
				continue
			}
			// Wrap with ProgressReader for byte-level progress (no-op when bus is nil).
			hashReader := progress.NewProgressReader(rc, opts.EventBus, item.relPath, workerID, "HASH", item.fileSize)
			actual, err := fileHasher.Sum(hashReader)
			_ = rc.Close()
			if err != nil {
				resultCh <- verifyResult{
					relPath:  item.relPath,
					absPath:  item.path,
					workerID: workerID,
					status:   "ERROR",
					expected: expected,
					err:      err,
				}
				continue
			}

			if actual == expected {
				resultCh <- verifyResult{
					relPath:  item.relPath,
					absPath:  item.path,
					workerID: workerID,
					status:   "OK",
					expected: expected,
					actual:   actual,
				}
			} else {
				resultCh <- verifyResult{
					relPath:  item.relPath,
					absPath:  item.path,
					workerID: workerID,
					status:   "MISMATCH",
					expected: expected,
					actual:   actual,
				}
			}
		}
	}
}

// resolveHasher returns the appropriate hasher for the given algorithm name.
// It uses the cache to avoid re-allocating hashers for each file.
func resolveHasher(defaultHasher *hash.Hasher, algo string, cache map[string]*hash.Hasher) *hash.Hasher {
	if algo == "" || algo == defaultHasher.Algorithm() {
		return defaultHasher
	}
	if cached, exists := cache[algo]; exists {
		return cached
	}
	newH, err := hash.NewHasher(algo)
	if err != nil {
		return defaultHasher
	}
	cache[algo] = newH
	return newH
}

// parseChecksum extracts the checksum and algorithm from a Pixe filename.
//
// Two formats are supported:
//
//	New (I2+): YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>.<ext>
//	  Detected by '-' at position 15 of the stem. The algorithm name is
//	  resolved from the numeric ID via hash.AlgorithmNameByID().
//
//	Legacy:    YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
//	  Detected by '_' at position 15 of the stem. The algorithm is inferred
//	  from the digest length: 40="sha1", 64="sha256". Returns algorithm=""
//	  when the length is ambiguous — the caller should fall back to opts.Hasher.
//
// Returns (checksum, algorithm, true) on success, or ("", "", false) when the
// filename does not match either Pixe format.
func parseChecksum(filename string) (checksum string, algorithm string, ok bool) {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	// Minimum stem length: "YYYYMMDD_HHMMSS" = 15 chars, plus delimiter + at least 1 char.
	if len(base) < 17 {
		return "", "", false
	}

	switch base[15] {
	case '-':
		// New format: YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>
		rest := base[16:] // everything after the first '-'
		dashIdx := strings.IndexByte(rest, '-')
		if dashIdx < 1 {
			return "", "", false
		}
		idStr := rest[:dashIdx]
		checksum = rest[dashIdx+1:]
		if len(checksum) < 8 {
			return "", "", false
		}
		// Parse the numeric algorithm ID (must be all digits).
		id := 0
		for _, c := range idStr {
			if c < '0' || c > '9' {
				return "", "", false
			}
			id = id*10 + int(c-'0')
		}
		algo := hash.AlgorithmNameByID(id)
		if algo == "" {
			return "", "", false // unknown ID
		}
		return checksum, algo, true

	case '_':
		// Legacy format: YYYYMMDD_HHMMSS_<CHECKSUM>
		checksum = base[16:]
		if len(checksum) < 8 {
			return "", "", false
		}
		switch len(checksum) {
		case 40:
			return checksum, "sha1", true
		case 64:
			return checksum, "sha256", true
		default:
			// Ambiguous length — return the checksum but leave algorithm empty.
			// The caller will fall back to opts.Hasher.
			return checksum, "", true
		}

	default:
		return "", "", false
	}
}
