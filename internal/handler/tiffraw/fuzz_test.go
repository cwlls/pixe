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

package tiffraw

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeTempFile writes data to a temp file with the given extension and returns
// its path. The file is placed in t.TempDir() and auto-cleaned.
func writeTempFile(t *testing.T, data []byte, ext string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fuzz"+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	return path
}

// FuzzExtractDateLE fuzzes ExtractDate on little-endian TIFF-based files.
// The only failure condition is a panic — errors and fallback dates are acceptable.
func FuzzExtractDateLE(f *testing.F) {
	// Seed corpus: valid TIFF LE header, empty, header only, adversarial offsets.
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // minimal LE TIFF
	f.Add([]byte{})                                                                                   // empty
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00})                                                             // header only
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0})                                                             // JPEG magic (cross-format)
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})                                     // all-ones (adversarial offsets)

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempFile(t, data, ".dng")
		b := &Base{}
		_, _ = b.ExtractDate(path)
	})
}

// FuzzExtractDateBE fuzzes ExtractDate on big-endian TIFF-based files.
func FuzzExtractDateBE(f *testing.F) {
	f.Add([]byte{0x4D, 0x4D, 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // minimal BE TIFF
	f.Add([]byte{})
	f.Add([]byte{0x4D, 0x4D, 0x00, 0x2A})                         // header only
	f.Add([]byte{0x89, 0x50, 0x4E, 0x47})                         // PNG magic (cross-format)
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // all-zeros

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempFile(t, data, ".nef")
		b := &Base{}
		_, _ = b.ExtractDate(path)
	})
}

// FuzzHashableReader fuzzes the HashableReader method of tiffraw.Base.
// If a reader is returned, it is fully drained to exercise the read path.
func FuzzHashableReader(f *testing.F) {
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x4D, 0x4D, 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{})
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempFile(t, data, ".dng")
		b := &Base{}
		rc, err := b.HashableReader(path)
		if err != nil {
			return
		}
		defer func() { _ = rc.Close() }()
		_, _ = io.ReadAll(rc)
	})
}
