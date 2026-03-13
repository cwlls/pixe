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
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/progress"
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
	Dir      string              // Dir is the destination directory (dirB) to verify.
	Hasher   *hash.Hasher        // Hasher computes content checksums for verification.
	Registry *discovery.Registry // Registry maps file extensions to their FileTypeHandler implementations.
	Output   io.Writer           // Output is where per-file result lines are written.
	// EventBus, when non-nil, receives structured progress events alongside
	// the plain-text Output writer. Both can be active simultaneously.
	// When nil, no events are emitted (existing behaviour is unchanged).
	EventBus *progress.Bus
}

// emitVerify sends an event to the bus if one is configured. It is a no-op
// when bus is nil, so callers do not need to guard every call site.
func emitVerify(bus *progress.Bus, e progress.Event) {
	if bus != nil {
		bus.Emit(e)
	}
}

// Run walks dir, parses checksums from filenames, recomputes hashes, and
// reports results. Returns a non-nil error only for fatal walk errors.
// Per-file mismatches are reported but do not cause Run to return an error —
// callers check Result.Mismatches to determine exit code.
func Run(opts Options) (Result, error) {
	var result Result

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
				Completed: completed,
				Total:     total,
			})
			return nil
		}

		// Resolve the hasher for this file. New-format files carry the algorithm
		// ID in the filename; legacy files use length inference. Fall back to the
		// configured opts.Hasher when the algorithm cannot be determined.
		fileHasher := opts.Hasher
		if algo != "" && algo != opts.Hasher.Algorithm() {
			if cached, exists := hasherCache[algo]; exists {
				fileHasher = cached
			} else {
				newH, newErr := hash.NewHasher(algo)
				if newErr == nil {
					hasherCache[algo] = newH
					fileHasher = newH
				}
				// If NewHasher fails (unknown algo), fall back to opts.Hasher.
			}
		}

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
				Completed:        completed,
				Total:            total,
			})
			return nil
		}
		actual, err := fileHasher.Sum(rc)
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
