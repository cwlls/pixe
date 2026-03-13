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

package jpeg

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeTempJPEG writes data to a temp file with .jpg extension and returns its path.
func writeTempJPEG(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fuzz.jpg")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeTempJPEG: %v", err)
	}
	return path
}

// FuzzDetect fuzzes the Detect method of the JPEG handler with arbitrary byte
// sequences. The only failure condition is a panic — errors are acceptable.
func FuzzDetect(f *testing.F) {
	// Seed corpus: valid JPEG, corrupt EXIF JPEG, empty, SOI only, cross-format.
	f.Add(buildMinimalJPEG())
	f.Add(buildCorruptEXIFJPEG())
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xD8})             // SOI only
	f.Add([]byte{0xFF, 0xD8, 0xFF})       // SOI + partial marker
	f.Add([]byte{0x89, 0x50, 0x4E, 0x47}) // PNG magic (cross-format)
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00}) // TIFF LE magic (cross-format)
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // all-ones

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempJPEG(t, data)
		h := New()
		_, _ = h.Detect(path)
	})
}

// FuzzExtractDate fuzzes the ExtractDate method of the JPEG handler.
func FuzzExtractDate(f *testing.F) {
	f.Add(buildMinimalJPEG())
	f.Add(buildCorruptEXIFJPEG())
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xD8})
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x08, 0x45, 0x78, 0x69, 0x66}) // truncated APP1

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempJPEG(t, data)
		h := New()
		_, _ = h.ExtractDate(path)
	})
}

// FuzzHashableReader fuzzes the HashableReader method of the JPEG handler.
// If a reader is returned, it is fully drained to exercise the read path.
func FuzzHashableReader(f *testing.F) {
	f.Add(buildMinimalJPEG())
	f.Add(buildCorruptEXIFJPEG())
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xD8})

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempJPEG(t, data)
		h := New()
		rc, err := h.HashableReader(path)
		if err != nil {
			return
		}
		defer func() { _ = rc.Close() }()
		_, _ = io.ReadAll(rc)
	})
}
