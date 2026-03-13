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

package orf

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
		Extensions: []string{".orf"},
		MagicSignatures: []domain.MagicSignature{
			{Offset: 0, Bytes: []byte{0x49, 0x49, 0x52, 0x4F}}, // IIRO Olympus ORF LE
			{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // Standard TIFF LE
		},
		BuildFakeFile:      buildFakeORF,
		WrongExtension:     "test.jpg",
		MetadataCapability: domain.MetadataSidecar,
	})
}

// buildFakeORF writes a minimal valid Olympus ORF file with the IIRO header.
func buildFakeORF(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker (LE) + Olympus ORF magic "IIRO"
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	buf.WriteByte(0x52)
	buf.WriteByte(0x4F)

	// IFD0 offset (8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 0 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
