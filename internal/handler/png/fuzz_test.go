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

package png

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeTempPNG writes data to a temp file with .png extension and returns its path.
func writeTempPNG(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fuzz.png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeTempPNG: %v", err)
	}
	return path
}

// fakePNGBytes returns a minimal valid PNG (signature + IHDR + IEND).
func fakePNGBytes() []byte {
	var buf []byte
	// PNG signature
	buf = append(buf, pngMagic...)
	// IHDR chunk: length=13, type="IHDR", data (13 bytes), CRC (4 bytes)
	ihdrData := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdrData[0:4], 1) // width=1
	binary.BigEndian.PutUint32(ihdrData[4:8], 1) // height=1
	ihdrData[8] = 8                              // bit depth
	ihdrData[9] = 2                              // color type (RGB)
	buf = append(buf, 0x00, 0x00, 0x00, 0x0D)    // length = 13
	buf = append(buf, 'I', 'H', 'D', 'R')
	buf = append(buf, ihdrData...)
	buf = append(buf, 0x00, 0x00, 0x00, 0x00) // CRC placeholder
	// IEND chunk
	buf = append(buf, 0x00, 0x00, 0x00, 0x00) // length = 0
	buf = append(buf, 'I', 'E', 'N', 'D')
	buf = append(buf, 0x00, 0x00, 0x00, 0x00) // CRC placeholder
	return buf
}

// truncatedPNGBytes returns a PNG with only the signature and partial IHDR.
func truncatedPNGBytes() []byte {
	var buf []byte
	buf = append(buf, pngMagic...)
	buf = append(buf, 0x00, 0x00, 0x00, 0x0D) // IHDR length
	buf = append(buf, 'I', 'H', 'D', 'R')
	buf = append(buf, 0x00, 0x00) // truncated — only 2 of 13 data bytes
	return buf
}

// FuzzDetect fuzzes the Detect method of the PNG handler.
func FuzzDetect(f *testing.F) {
	f.Add(fakePNGBytes())
	f.Add(truncatedPNGBytes())
	f.Add(pngMagic) // signature only
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0})                         // JPEG magic (cross-format)
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00})                         // TIFF LE magic (cross-format)
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // all-zeros

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempPNG(t, data)
		h := New()
		_, _ = h.Detect(path)
	})
}

// FuzzExtractDate fuzzes the ExtractDate method of the PNG handler.
func FuzzExtractDate(f *testing.F) {
	f.Add(fakePNGBytes())
	f.Add(truncatedPNGBytes())
	f.Add(pngMagic)
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}) // adversarial chunk lengths

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempPNG(t, data)
		h := New()
		_, _ = h.ExtractDate(path)
	})
}

// FuzzHashableReader fuzzes the HashableReader method of the PNG handler.
func FuzzHashableReader(f *testing.F) {
	f.Add(fakePNGBytes())
	f.Add(truncatedPNGBytes())
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		path := writeTempPNG(t, data)
		h := New()
		rc, err := h.HashableReader(path)
		if err != nil {
			return
		}
		defer func() { _ = rc.Close() }()
		_, _ = io.ReadAll(rc)
	})
}
