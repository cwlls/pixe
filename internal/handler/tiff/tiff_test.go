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

package tiff

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/handler/handlertest"
)

func TestHandler(t *testing.T) {
	handlertest.RunSuite(t, handlertest.SuiteConfig{
		NewHandler: func() domain.FileTypeHandler { return New() },
		Extensions: []string{".tif", ".tiff"},
		MagicSignatures: []domain.MagicSignature{
			{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
			{Offset: 0, Bytes: []byte{0x4D, 0x4D, 0x00, 0x2A}}, // TIFF BE
		},
		BuildFakeFile:      buildFakeTIFF,
		WrongExtension:     "test.jpg",
		MetadataCapability: domain.MetadataSidecar,
	})
}

// buildFakeTIFF writes a minimal valid TIFF LE file at the given path.
// The file contains: 8-byte TIFF LE header + empty IFD0 (0 entries) +
// next-IFD offset of 0 (end of chain). This is sufficient for Detect()
// and HashableReader() to operate without error.
func buildFakeTIFF(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker: little-endian ("II")
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic number (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// Offset to IFD0 (8 bytes from start of file)
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 0 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))

	// Next IFD offset (0 = end of chain)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
