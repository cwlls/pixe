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

package cr2

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/handler/handlertest"
)

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

func TestHandler(t *testing.T) {
	handlertest.RunSuite(t, handlertest.SuiteConfig{
		NewHandler: func() domain.FileTypeHandler { return New() },
		Extensions: []string{".cr2"},
		MagicSignatures: []domain.MagicSignature{
			{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
		},
		BuildFakeFile:      buildFakeCR2,
		WrongExtension:     "test.jpg",
		MetadataCapability: domain.MetadataSidecar,
	})
}

// TestHandler_Detect_tiffWithoutCR verifies that a TIFF LE file without the
// "CR" signature at offset 8 is NOT detected as CR2. This is CR2-specific
// detection logic not covered by the shared suite.
func TestHandler_Detect_tiffWithoutCR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cr2")
	// TIFF LE header but no "CR" at offset 8
	data := []byte{
		0x49, 0x49, 0x2A, 0x00, // TIFF LE
		0x08, 0x00, 0x00, 0x00, // IFD0 at offset 8
		0x00, 0x00, // Not "CR"
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for TIFF without CR signature")
	}
}

// buildFakeCR2 writes a CR2 file with TIFF LE header + "CR" at offset 8.
// Structure:
//
//	Bytes 0-3: 0x49 0x49 0x2A 0x00 (TIFF LE)
//	Bytes 4-7: 0x0A 0x00 0x00 0x00 (IFD0 at offset 10)
//	Bytes 8-9: 0x43 0x52 ("CR" signature)
//	Bytes 10-11: 0x00 0x00 (0 entries)
//	Bytes 12-15: 0x00 0x00 0x00 0x00 (next IFD = 0)
func buildFakeCR2(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// IFD0 offset (10)
	_ = binary.Write(buf, binary.LittleEndian, uint32(10))

	// "CR" signature at offset 8
	buf.WriteByte(0x43)
	buf.WriteByte(0x52)

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
