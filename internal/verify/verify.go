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
// It walks a sorted dirB, parses the checksum embedded in each filename
// (format: YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>), recomputes the data-only hash
// via the registered FileTypeHandler, and reports mismatches.
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

		// Parse the expected checksum from the filename.
		expected, ok := parseChecksum(name)
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
		actual, err := opts.Hasher.Sum(rc)
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

// parseChecksum extracts the checksum from a Pixe filename.
// Expected format: YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
// The checksum is the segment between the second underscore and the dot.
func parseChecksum(filename string) (string, bool) {
	// Strip extension.
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	// Split on "_" — expect at least 3 parts: date, time, checksum.
	parts := strings.SplitN(base, "_", 3)
	if len(parts) != 3 {
		return "", false
	}
	checksum := parts[2]
	if len(checksum) < 8 { // sanity: SHA-1 is 40, SHA-256 is 64
		return "", false
	}
	return checksum, true
}
