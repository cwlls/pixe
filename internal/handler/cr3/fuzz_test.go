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

package cr3

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeTempCR3 writes data to a temp file with .cr3 extension and returns its path.
func writeTempCR3(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fuzz.cr3")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeTempCR3: %v", err)
	}
	return path
}

// fakeHEICBrand returns a minimal ftyp box with "heic" brand (cross-format seed).
func fakeHEICBrand() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18,
		0x66, 0x74, 0x79, 0x70,
		0x68, 0x65, 0x69, 0x63, // "heic"
		0x00, 0x00, 0x00, 0x00,
		0x68, 0x65, 0x69, 0x63,
		0x6d, 0x69, 0x66, 0x31,
	}
}

// fakeAVIFBrand returns a minimal ftyp box with "avif" brand (cross-format seed).
func fakeAVIFBrand() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18,
		0x66, 0x74, 0x79, 0x70,
		0x61, 0x76, 0x69, 0x66, // "avif"
		0x00, 0x00, 0x00, 0x00,
		0x61, 0x76, 0x69, 0x66,
		0x6D, 0x69, 0x66, 0x31,
	}
}

// FuzzDetect fuzzes the Detect method of the CR3 handler.
func FuzzDetect(f *testing.F) {
	// Use a temp dir for the fake CR3 seed.
	f.Add([]byte{
		0x00, 0x00, 0x00, 0x14, // ftyp size = 20
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x63, 0x72, 0x78, 0x20, // "crx "
		0x00, 0x00, 0x00, 0x01, // minor version
		0x63, 0x72, 0x78, 0x20, // compat "crx "
	})
	f.Add(fakeHEICBrand()) // cross-format: HEIC
	f.Add(fakeAVIFBrand()) // cross-format: AVIF
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x08, 0x66, 0x74, 0x79, 0x70}) // ftyp header only
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0})                         // JPEG magic (cross-format)
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // all-zeros

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempCR3(t, data)
		h := New()
		_, _ = h.Detect(path)
	})
}

// FuzzExtractDate fuzzes the ExtractDate method of the CR3 handler.
func FuzzExtractDate(f *testing.F) {
	f.Add([]byte{
		0x00, 0x00, 0x00, 0x14,
		0x66, 0x74, 0x79, 0x70,
		0x63, 0x72, 0x78, 0x20,
		0x00, 0x00, 0x00, 0x01,
		0x63, 0x72, 0x78, 0x20,
	})
	f.Add(fakeHEICBrand())
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x08, 0x66, 0x74, 0x79, 0x70})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // adversarial sizes

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempCR3(t, data)
		h := New()
		_, _ = h.ExtractDate(path)
	})
}

// FuzzHashableReader fuzzes the HashableReader method of the CR3 handler.
func FuzzHashableReader(f *testing.F) {
	f.Add([]byte{
		0x00, 0x00, 0x00, 0x14,
		0x66, 0x74, 0x79, 0x70,
		0x63, 0x72, 0x78, 0x20,
		0x00, 0x00, 0x00, 0x01,
		0x63, 0x72, 0x78, 0x20,
	})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempCR3(t, data)
		h := New()
		rc, err := h.HashableReader(path)
		if err != nil {
			return
		}
		defer func() { _ = rc.Close() }()
		_, _ = io.ReadAll(rc)
	})
}
