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

package mp4

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeTempMP4 writes data to a temp file with .mp4 extension and returns its path.
func writeTempMP4(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fuzz.mp4")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeTempMP4: %v", err)
	}
	return path
}

// zeroLengthAtom returns a ftyp box followed by a zero-size atom (infinite-length).
func zeroLengthAtom() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18, // ftyp size = 24
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x6d, 0x70, 0x34, 0x32, // "mp42"
		0x00, 0x00, 0x00, 0x00, // minor version
		0x6d, 0x70, 0x34, 0x32, // compat "mp42"
		0x69, 0x73, 0x6f, 0x6d, // compat "isom"
		0x00, 0x00, 0x00, 0x00, // zero-size atom (infinite length)
		0x6d, 0x6f, 0x6f, 0x76, // "moov"
	}
}

// FuzzDetect fuzzes the Detect method of the MP4 handler.
func FuzzDetect(f *testing.F) {
	f.Add(buildMinimalMP4(0))
	f.Add(zeroLengthAtom())
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x08, 0x66, 0x74, 0x79, 0x70}) // ftyp header only
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0})                         // JPEG magic (cross-format)
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // all-zeros
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0x6d, 0x6f, 0x6f, 0x76}) // huge moov atom

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempMP4(t, data)
		h := New()
		_, _ = h.Detect(path)
	})
}

// FuzzExtractDate fuzzes the ExtractDate method of the MP4 handler.
func FuzzExtractDate(f *testing.F) {
	f.Add(buildMinimalMP4(0))
	f.Add(buildMinimalMP4(0xFFFFFFFF)) // max creation time
	f.Add(zeroLengthAtom())
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x08, 0x66, 0x74, 0x79, 0x70})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // adversarial sizes

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempMP4(t, data)
		h := New()
		_, _ = h.ExtractDate(path)
	})
}

// FuzzHashableReader fuzzes the HashableReader method of the MP4 handler.
func FuzzHashableReader(f *testing.F) {
	f.Add(buildMinimalMP4(0))
	f.Add(zeroLengthAtom())
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempMP4(t, data)
		h := New()
		rc, err := h.HashableReader(path)
		if err != nil {
			return
		}
		defer func() { _ = rc.Close() }()
		_, _ = io.ReadAll(rc)
	})
}
