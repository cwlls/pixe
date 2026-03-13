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
	path        string   // absolute path
	relPath     string   // relative to opts.Dir
	name        string   // filename (for parseChecksum)
	fileSize    int64    // from os.Stat; 0 if stat failed
	sidecarExts []string // extensions of associated sidecar files (e.g. [".xmp"])
}

// verifyResult is sent from a worker back to the coordinator.
type verifyResult struct {
	relPath     string
	absPath     string
	workerID    int
	status      string // "OK", "MISMATCH", "UNRECOGNISED", "ERROR"
	expected    string
	actual      string
	err         error
	sidecarExts []string // extensions of associated sidecar files
}

// isSidecarFile reports whether the given filename is a sidecar file that
// should be skipped during verification (not hashed, not counted as unrecognised).
// Sidecar files are identified purely by extension.
func isSidecarFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".xmp" || ext == ".aae"
}

// associateSidecars matches sidecar filenames to their parent media filenames
// within the same directory. A sidecar is associated with a parent when the
// sidecar filename starts with the parent's basename (e.g.
// "20211225_062223-1-abc123.arw.xmp" → parent "20211225_062223-1-abc123.arw").
//
// Returns:
//   - parentSidecars: map from media filename → list of sidecar extensions
//   - orphans: sidecar filenames with no matching parent in mediaNames
func associateSidecars(mediaNames []string, sidecarNames []string) (parentSidecars map[string][]string, orphans []string) {
	parentSidecars = make(map[string][]string)
	for _, sc := range sidecarNames {
		matched := false
		for _, media := range mediaNames {
			// A sidecar matches if its name starts with the media filename
			// (e.g. "photo.arw.xmp" starts with "photo.arw").
			if strings.HasPrefix(sc, media) && len(sc) > len(media) {
				ext := strings.ToLower(sc[len(media):])
				// Verify the remainder is a known sidecar extension.
				if ext == ".xmp" || ext == ".aae" {
					parentSidecars[media] = append(parentSidecars[media], ext)
					matched = true
					break
				}
			}
		}
		if !matched {
			orphans = append(orphans, sc)
		}
	}
	return parentSidecars, orphans
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

	// Pre-scan: collect all items (media files and sidecars) and build the
	// sidecar association map. Sidecars are grouped by directory so they can
	// be matched to their parent media files.
	//
	// sidecarsByDir maps directory path → list of sidecar filenames in that dir.
	// mediaByDir maps directory path → list of media filenames in that dir.
	sidecarsByDir := make(map[string][]string)
	mediaByDir := make(map[string][]string)

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
		dir := filepath.Dir(path)
		if isSidecarFile(name) {
			sidecarsByDir[dir] = append(sidecarsByDir[dir], name)
		} else {
			mediaByDir[dir] = append(mediaByDir[dir], name)
		}
		return nil
	})

	// Build per-directory sidecar association maps.
	// parentSidecarsByDir maps dir → (media filename → []sidecar extensions).
	// orphansByDir maps dir → []orphan sidecar filenames.
	type dirSidecars struct {
		parentMap map[string][]string
		orphans   []string
	}
	dirSidecarMap := make(map[string]dirSidecars)
	for dir, sidecars := range sidecarsByDir {
		pm, orphans := associateSidecars(mediaByDir[dir], sidecars)
		dirSidecarMap[dir] = dirSidecars{parentMap: pm, orphans: orphans}
	}

	// Count total verifiable files (media only; sidecars are not counted).
	total := 0
	for _, names := range mediaByDir {
		total += len(names)
	}
	// Orphaned sidecars also count toward total (they will be UNRECOGNISED).
	for _, ds := range dirSidecarMap {
		total += len(ds.orphans)
	}

	emitVerify(opts.EventBus, progress.Event{
		Kind:  progress.EventVerifyStart,
		Total: total,
	})

	// hasherCache avoids re-allocating hashers for each file in mixed-algorithm archives.
	// Seeded with the configured default hasher.
	hasherCache := map[string]*hash.Hasher{
		opts.Hasher.Algorithm(): opts.Hasher,
	}

	// Track which orphaned sidecars have been reported (to avoid double-reporting).
	reportedOrphans := make(map[string]bool)

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

		// Skip sidecar files — they are handled via inline annotation on the
		// parent file's output line. Orphaned sidecars are reported below.
		if isSidecarFile(name) {
			dir := filepath.Dir(path)
			relPath, _ := filepath.Rel(opts.Dir, path)
			if ds, ok := dirSidecarMap[dir]; ok {
				for _, orphan := range ds.orphans {
					if orphan == name && !reportedOrphans[path] {
						reportedOrphans[path] = true
						_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", relPath)
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
					}
				}
			}
			return nil
		}

		// Compute relative path for event reporting.
		relPath, _ := filepath.Rel(opts.Dir, path)

		// Look up associated sidecar extensions for this media file.
		dir := filepath.Dir(path)
		var scExts []string
		if ds, ok := dirSidecarMap[dir]; ok {
			scExts = ds.parentMap[name]
		}

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
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", relPath)
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
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", relPath)
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
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", relPath, err)
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
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", relPath, err)
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
		annotation := formatVerifySidecarAnnotation(scExts)
		if actual == expected {
			_, _ = fmt.Fprintf(opts.Output, "  OK            %s%s\n", relPath, annotation)
			result.Verified++
			emitVerify(opts.EventBus, progress.Event{
				Kind:        progress.EventVerifyOK,
				RelPath:     relPath,
				AbsPath:     path,
				Checksum:    actual,
				SidecarExts: scExts,
				WorkerID:    0,
				Completed:   completed,
				Total:       total,
			})
		} else {
			_, _ = fmt.Fprintf(opts.Output, "  MISMATCH      %s%s\n    expected: %s\n    actual:   %s\n",
				relPath, annotation, expected, actual)
			result.Mismatches++
			emitVerify(opts.EventBus, progress.Event{
				Kind:             progress.EventVerifyMismatch,
				RelPath:          relPath,
				AbsPath:          path,
				ExpectedChecksum: expected,
				ActualChecksum:   actual,
				SidecarExts:      scExts,
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

// formatVerifySidecarAnnotation builds the inline sidecar annotation string
// for verify output (e.g. []string{".xmp"} → " [+xmp]"). Returns "" when empty.
func formatVerifySidecarAnnotation(exts []string) string {
	if len(exts) == 0 {
		return ""
	}
	result := " ["
	for i, ext := range exts {
		if i > 0 {
			result += " "
		}
		if len(ext) > 1 && ext[0] == '.' {
			result += "+" + ext[1:]
		} else {
			result += "+" + ext
		}
	}
	result += "]"
	return result
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

	// Pre-scan: collect all items and build sidecar association maps.
	// Sidecars are grouped by directory so they can be matched to parent media files.
	sidecarsByDir := make(map[string][]string)
	mediaByDir := make(map[string][]string)
	var allPaths []struct {
		path     string
		relPath  string
		name     string
		fileSize int64
	}

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
		dir := filepath.Dir(path)
		if isSidecarFile(name) {
			sidecarsByDir[dir] = append(sidecarsByDir[dir], name)
		} else {
			mediaByDir[dir] = append(mediaByDir[dir], name)
			allPaths = append(allPaths, struct {
				path     string
				relPath  string
				name     string
				fileSize int64
			}{path, relPath, name, fileSize})
		}
		return nil
	})

	// Build per-directory sidecar association maps.
	type dirSidecars struct {
		parentMap map[string][]string
		orphans   []string
	}
	dirSidecarMap := make(map[string]dirSidecars)
	for dir, sidecars := range sidecarsByDir {
		pm, orphans := associateSidecars(mediaByDir[dir], sidecars)
		dirSidecarMap[dir] = dirSidecars{parentMap: pm, orphans: orphans}
	}

	// Build the items list with sidecar associations.
	var items []verifyItem
	for _, p := range allPaths {
		dir := filepath.Dir(p.path)
		var scExts []string
		if ds, ok := dirSidecarMap[dir]; ok {
			scExts = ds.parentMap[p.name]
		}
		items = append(items, verifyItem{
			path:        p.path,
			relPath:     p.relPath,
			name:        p.name,
			fileSize:    p.fileSize,
			sidecarExts: scExts,
		})
	}

	// Collect orphaned sidecars as UNRECOGNISED items.
	var orphanItems []verifyItem
	for dir, ds := range dirSidecarMap {
		for _, orphan := range ds.orphans {
			absPath := filepath.Join(dir, orphan)
			relPath, _ := filepath.Rel(opts.Dir, absPath)
			orphanItems = append(orphanItems, verifyItem{
				path:    absPath,
				relPath: relPath,
				name:    orphan,
			})
		}
	}

	total := len(items) + len(orphanItems)
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

	// Report orphaned sidecars as UNRECOGNISED before processing media results.
	for _, orphan := range orphanItems {
		completed++
		_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", orphan.relPath)
		result.Unrecognised++
		emitVerify(opts.EventBus, progress.Event{
			Kind:      progress.EventVerifyUnrecognised,
			RelPath:   orphan.relPath,
			AbsPath:   orphan.path,
			WorkerID:  0,
			Completed: completed,
			Total:     total,
		})
	}

	for res := range resultCh {
		completed++
		annotation := formatVerifySidecarAnnotation(res.sidecarExts)
		switch res.status {
		case "OK":
			_, _ = fmt.Fprintf(opts.Output, "  OK            %s%s\n", res.relPath, annotation)
			result.Verified++
			emitVerify(opts.EventBus, progress.Event{
				Kind:        progress.EventVerifyOK,
				RelPath:     res.relPath,
				AbsPath:     res.absPath,
				Checksum:    res.actual,
				SidecarExts: res.sidecarExts,
				WorkerID:    res.workerID,
				Completed:   completed,
				Total:       total,
			})
		case "MISMATCH":
			_, _ = fmt.Fprintf(opts.Output, "  MISMATCH      %s%s\n    expected: %s\n    actual:   %s\n",
				res.relPath, annotation, res.expected, res.actual)
			result.Mismatches++
			emitVerify(opts.EventBus, progress.Event{
				Kind:             progress.EventVerifyMismatch,
				RelPath:          res.relPath,
				AbsPath:          res.absPath,
				ExpectedChecksum: res.expected,
				ActualChecksum:   res.actual,
				SidecarExts:      res.sidecarExts,
				WorkerID:         res.workerID,
				Completed:        completed,
				Total:            total,
			})
		case "ERROR":
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", res.relPath, res.err)
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
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", res.relPath)
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
					relPath:     item.relPath,
					absPath:     item.path,
					workerID:    workerID,
					status:      "UNRECOGNISED",
					sidecarExts: item.sidecarExts,
				}
				continue
			}

			// Resolve the hasher for this file.
			fileHasher := resolveHasher(opts.Hasher, algo, hasherCache)

			// Detect the file type.
			handler, err := opts.Registry.Detect(item.path)
			if err != nil || handler == nil {
				resultCh <- verifyResult{
					relPath:     item.relPath,
					absPath:     item.path,
					workerID:    workerID,
					status:      "UNRECOGNISED",
					sidecarExts: item.sidecarExts,
				}
				continue
			}

			// Recompute the hash via the handler's HashableReader.
			rc, err := handler.HashableReader(item.path)
			if err != nil {
				resultCh <- verifyResult{
					relPath:     item.relPath,
					absPath:     item.path,
					workerID:    workerID,
					status:      "ERROR",
					expected:    expected,
					err:         err,
					sidecarExts: item.sidecarExts,
				}
				continue
			}
			// Wrap with ProgressReader for byte-level progress (no-op when bus is nil).
			hashReader := progress.NewProgressReader(rc, opts.EventBus, item.relPath, workerID, "HASH", item.fileSize)
			actual, err := fileHasher.Sum(hashReader)
			_ = rc.Close()
			if err != nil {
				resultCh <- verifyResult{
					relPath:     item.relPath,
					absPath:     item.path,
					workerID:    workerID,
					status:      "ERROR",
					expected:    expected,
					err:         err,
					sidecarExts: item.sidecarExts,
				}
				continue
			}

			if actual == expected {
				resultCh <- verifyResult{
					relPath:     item.relPath,
					absPath:     item.path,
					workerID:    workerID,
					status:      "OK",
					expected:    expected,
					actual:      actual,
					sidecarExts: item.sidecarExts,
				}
			} else {
				resultCh <- verifyResult{
					relPath:     item.relPath,
					absPath:     item.path,
					workerID:    workerID,
					status:      "MISMATCH",
					expected:    expected,
					actual:      actual,
					sidecarExts: item.sidecarExts,
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
