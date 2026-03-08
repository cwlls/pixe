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

package pef

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
)

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

// TestHandler_Extensions verifies the correct extension is returned.
func TestHandler_Extensions(t *testing.T) {
	h := New()
	exts := h.Extensions()
	if len(exts) != 1 || exts[0] != ".pef" {
		t.Fatalf("Expected [.pef], got %v", exts)
	}
}

// TestHandler_MagicBytes verifies the correct magic signature is returned.
func TestHandler_MagicBytes(t *testing.T) {
	h := New()
	sigs := h.MagicBytes()
	if len(sigs) != 1 {
		t.Fatalf("Expected 1 magic signature, got %d", len(sigs))
	}
	if sigs[0].Offset != 0 || !bytes.Equal(sigs[0].Bytes, []byte{0x49, 0x49, 0x2A, 0x00}) {
		t.Fatalf("Magic signature mismatch: %v", sigs[0])
	}
}

// TestHandler_Detect_valid verifies detection with correct extension and magic.
func TestHandler_Detect_valid(t *testing.T) {
	dir := t.TempDir()
	filePath := buildFakePEF(t, dir, "test.pef")

	h := New()
	ok, err := h.Detect(filePath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !ok {
		t.Fatal("Detect should return true for valid PEF")
	}
}

// TestHandler_Detect_wrongExtension verifies detection fails with wrong extension.
func TestHandler_Detect_wrongExtension(t *testing.T) {
	dir := t.TempDir()
	filePath := buildFakePEF(t, dir, "test.jpg")

	h := New()
	ok, err := h.Detect(filePath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for wrong extension")
	}
}

// TestHandler_Detect_wrongMagic verifies detection fails with wrong magic.
func TestHandler_Detect_wrongMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pef")
	if err := os.WriteFile(path, []byte{0x00, 0x00, 0x00, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for wrong magic")
	}
}

// TestHandler_ExtractDate_noEXIF_fallback verifies fallback to Ansel Adams date.
func TestHandler_ExtractDate_noEXIF_fallback(t *testing.T) {
	dir := t.TempDir()
	filePath := buildFakePEF(t, dir, "test.pef")

	h := New()
	date, err := h.ExtractDate(filePath)
	if err != nil {
		t.Fatalf("ExtractDate failed: %v", err)
	}

	expected := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !date.Equal(expected) {
		t.Fatalf("Expected Ansel Adams date %v, got %v", expected, date)
	}
}

// TestHandler_HashableReader_returnsData verifies non-empty data is returned.
func TestHandler_HashableReader_returnsData(t *testing.T) {
	dir := t.TempDir()
	filePath := buildFakePEF(t, dir, "test.pef")

	h := New()
	rc, err := h.HashableReader(filePath)
	if err != nil {
		t.Fatalf("HashableReader failed: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("HashableReader returned empty data")
	}
}

// TestHandler_HashableReader_deterministic verifies two calls return identical bytes.
func TestHandler_HashableReader_deterministic(t *testing.T) {
	dir := t.TempDir()
	filePath := buildFakePEF(t, dir, "test.pef")

	h := New()

	rc1, err := h.HashableReader(filePath)
	if err != nil {
		t.Fatalf("First HashableReader failed: %v", err)
	}
	data1, err := io.ReadAll(rc1)
	if err != nil {
		t.Fatalf("First ReadAll failed: %v", err)
	}
	_ = rc1.Close()

	rc2, err := h.HashableReader(filePath)
	if err != nil {
		t.Fatalf("Second HashableReader failed: %v", err)
	}
	data2, err := io.ReadAll(rc2)
	if err != nil {
		t.Fatalf("Second ReadAll failed: %v", err)
	}
	_ = rc2.Close()

	if !bytes.Equal(data1, data2) {
		t.Fatal("HashableReader returned different data on second call")
	}
}

// TestHandler_WriteMetadataTags_noop verifies WriteMetadataTags is a no-op.
func TestHandler_WriteMetadataTags_noop(t *testing.T) {
	h := New()
	tags := domain.MetadataTags{Copyright: "test", CameraOwner: "test"}
	err := h.WriteMetadataTags("dummy.pef", tags)
	if err != nil {
		t.Fatalf("WriteMetadataTags should be no-op, got error: %v", err)
	}
}

// buildFakePEF writes a minimal valid TIFF LE file with .pef extension.
func buildFakePEF(t *testing.T, dir, name string) string {
	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

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
