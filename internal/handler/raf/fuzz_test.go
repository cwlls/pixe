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

package raf

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeTempRAF writes data to a temp file with .raf extension and returns its path.
func writeTempRAF(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fuzz.raf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeTempRAF: %v", err)
	}
	return path
}

// validRAFHeader returns a minimal valid RAF header (headerMinSize bytes) with
// zeroed JPEG and CFA offsets/lengths. Used as a fuzz seed.
func validRAFHeader() []byte {
	buf := make([]byte, headerMinSize)
	copy(buf[0:], rafMagic)
	copy(buf[0x10:], "0201")
	copy(buf[0x3C:], "0100")
	return buf
}

// FuzzDetect fuzzes the Detect method of the RAF handler.
// Only panics are failures — errors and false returns are valid.
func FuzzDetect(f *testing.F) {
	f.Add(validRAFHeader())
	f.Add(append(validRAFHeader(), minimalJPEG...))
	f.Add([]byte(rafMagic)) // magic only, no offset directory
	f.Add([]byte{})
	// TIFF LE magic (cross-format)
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	// JPEG magic (cross-format)
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0})
	// All-zeros
	f.Add(make([]byte, 16))
	// All-0xFF
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempRAF(t, data)
		h := New()
		_, _ = h.Detect(path)
	})
}

// FuzzExtractDate fuzzes the ExtractDate method of the RAF handler.
// Only panics are failures.
func FuzzExtractDate(f *testing.F) {
	f.Add(validRAFHeader())
	f.Add(append(validRAFHeader(), minimalJPEG...))
	f.Add([]byte(rafMagic))
	f.Add([]byte{})
	// Adversarial: RAF magic + max uint32 offsets (0xFFFFFFFF) — must not panic.
	adversarial := make([]byte, headerMinSize)
	copy(adversarial[0:], rafMagic)
	for i := jpegOffsetPos; i < headerMinSize; i++ {
		adversarial[i] = 0xFF
	}
	f.Add(adversarial)

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempRAF(t, data)
		h := New()
		_, _ = h.ExtractDate(path)
	})
}

// FuzzHashableReader fuzzes the HashableReader method of the RAF handler.
// Only panics are failures. The reader is fully drained to exercise all paths.
func FuzzHashableReader(f *testing.F) {
	f.Add(validRAFHeader())
	f.Add(append(validRAFHeader(), minimalJPEG...))
	f.Add([]byte{})
	// Adversarial: max offsets
	adversarial := make([]byte, headerMinSize)
	copy(adversarial[0:], rafMagic)
	for i := cfaOffsetPos; i < headerMinSize; i++ {
		adversarial[i] = 0xFF
	}
	f.Add(adversarial)

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempRAF(t, data)
		h := New()
		rc, err := h.HashableReader(path)
		if err != nil {
			return
		}
		defer func() { _ = rc.Close() }()
		_, _ = io.ReadAll(rc)
	})
}
