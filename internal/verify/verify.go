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
)

// Result holds the outcome of a verify run.
type Result struct {
	Verified     int
	Mismatches   int
	Unrecognised int
}

// FileResult is the per-file outcome emitted to the output writer.
type FileResult struct {
	Path     string
	Status   string // "OK", "MISMATCH", "UNRECOGNISED"
	Expected string
	Actual   string
}

// Options configures a verify run.
type Options struct {
	Dir      string
	Hasher   *hash.Hasher
	Registry *discovery.Registry
	Output   io.Writer
}

// Run walks dir, parses checksums from filenames, recomputes hashes, and
// reports results. Returns a non-nil error only for fatal walk errors.
// Per-file mismatches are reported but do not cause Run to return an error —
// callers check Result.Mismatches to determine exit code.
func Run(opts Options) (Result, error) {
	var result Result

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

		// Parse the expected checksum from the filename.
		expected, ok := parseChecksum(name)
		if !ok {
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", path)
			result.Unrecognised++
			return nil
		}

		// Detect the file type.
		handler, err := opts.Registry.Detect(path)
		if err != nil || handler == nil {
			_, _ = fmt.Fprintf(opts.Output, "  UNRECOGNISED  %s\n", path)
			result.Unrecognised++
			return nil
		}

		// Recompute the hash via the handler's HashableReader.
		rc, err := handler.HashableReader(path)
		if err != nil {
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", path, err)
			result.Mismatches++
			return nil
		}
		actual, err := opts.Hasher.Sum(rc)
		_ = rc.Close()
		if err != nil {
			_, _ = fmt.Fprintf(opts.Output, "  ERROR         %s: %v\n", path, err)
			result.Mismatches++
			return nil
		}

		if actual == expected {
			_, _ = fmt.Fprintf(opts.Output, "  OK            %s\n", path)
			result.Verified++
		} else {
			_, _ = fmt.Fprintf(opts.Output, "  MISMATCH      %s\n    expected: %s\n    actual:   %s\n",
				path, expected, actual)
			result.Mismatches++
		}
		return nil
	})

	_, _ = fmt.Fprintf(opts.Output, "\nDone. verified=%d mismatches=%d unrecognised=%d\n",
		result.Verified, result.Mismatches, result.Unrecognised)

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
