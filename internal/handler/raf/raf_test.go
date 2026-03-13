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

// ---------------------------------------------------------------------------
// Extensions
// ---------------------------------------------------------------------------

func TestHandler_Extensions(t *testing.T) {
	t.Parallel()
	h := New()
	exts := h.Extensions()
	if len(exts) != 1 || exts[0] != ".raf" {
		t.Fatalf("Expected [.raf], got %v", exts)
	}
}

// ---------------------------------------------------------------------------
// MagicBytes
// ---------------------------------------------------------------------------

func TestHandler_MagicBytes(t *testing.T) {
	t.Parallel()
	h := New()
	sigs := h.MagicBytes()
	if len(sigs) != 1 {
		t.Fatalf("Expected 1 magic signature, got %d", len(sigs))
	}
	if sigs[0].Offset != 0 {
		t.Fatalf("Expected offset 0, got %d", sigs[0].Offset)
	}
	if !bytes.Equal(sigs[0].Bytes, []byte(rafMagic)) {
		t.Fatalf("Magic bytes mismatch: got %q, want %q", sigs[0].Bytes, rafMagic)
	}
}

// ---------------------------------------------------------------------------
// Detect
// ---------------------------------------------------------------------------

func TestHandler_Detect_valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeRAF(t, dir, "test.raf")

	h := New()
	ok, err := h.Detect(filePath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !ok {
		t.Fatal("Detect should return true for valid RAF")
	}
}

func TestHandler_Detect_wrongExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeRAF(t, dir, "test.jpg")

	h := New()
	ok, err := h.Detect(filePath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for wrong extension")
	}
}

func TestHandler_Detect_wrongMagic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.raf")
	// Write null bytes — no RAF magic.
	if err := os.WriteFile(path, make([]byte, 64), 0o644); err != nil {
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

func TestHandler_Detect_emptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.raf")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect must not error on empty file, got: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for empty file")
	}
}

func TestHandler_Detect_tiffMagic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.raf")
	// TIFF little-endian magic: II*\0
	tiffLE := []byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if err := os.WriteFile(path, tiffLE, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for TIFF magic in .raf file")
	}
}

// ---------------------------------------------------------------------------
// ExtractDate
// ---------------------------------------------------------------------------

func TestHandler_ExtractDate_noEXIF_fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// buildFakeRAF embeds a minimal JPEG with no EXIF — must fall back.
	filePath := buildFakeRAF(t, dir, "test.raf")

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

// ---------------------------------------------------------------------------
// HashableReader
// ---------------------------------------------------------------------------

func TestHandler_HashableReader_returnsData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeRAF(t, dir, "test.raf")

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

func TestHandler_HashableReader_deterministic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeRAF(t, dir, "test.raf")

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

func TestHandler_HashableReader_returnsCFAData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfaData := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
	filePath := buildFakeRAFWithCFA(t, dir, "test.raf", cfaData)

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

	if !bytes.Equal(data, cfaData) {
		t.Fatalf("HashableReader returned %x, want CFA data %x", data, cfaData)
	}
}

// ---------------------------------------------------------------------------
// MetadataSupport
// ---------------------------------------------------------------------------

func TestHandler_MetadataSupport(t *testing.T) {
	t.Parallel()
	h := New()
	got := h.MetadataSupport()
	if got != domain.MetadataSidecar {
		t.Errorf("MetadataSupport() = %v, want MetadataSidecar", got)
	}
}

// ---------------------------------------------------------------------------
// WriteMetadataTags
// ---------------------------------------------------------------------------

func TestHandler_WriteMetadataTags_noop(t *testing.T) {
	t.Parallel()
	h := New()
	tags := domain.MetadataTags{Copyright: "test", CameraOwner: "test"}
	if err := h.WriteMetadataTags("dummy.raf", tags); err != nil {
		t.Fatalf("WriteMetadataTags should be no-op, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test fixture builders
// ---------------------------------------------------------------------------

// minimalJPEG is a minimal valid JPEG with no EXIF: SOI + APP0 (JFIF) + EOI.
// This ensures ExtractDate falls back to the Ansel Adams date.
var minimalJPEG = []byte{
	// SOI
	0xFF, 0xD8,
	// APP0 marker
	0xFF, 0xE0,
	// APP0 length (16 bytes including the length field itself)
	0x00, 0x10,
	// JFIF\0 identifier
	0x4A, 0x46, 0x49, 0x46, 0x00,
	// version 1.1
	0x01, 0x01,
	// aspect ratio units = 0 (no units)
	0x00,
	// X density = 1, Y density = 1
	0x00, 0x01, 0x00, 0x01,
	// thumbnail dimensions = 0x0
	0x00, 0x00,
	// EOI
	0xFF, 0xD9,
}

// buildFakeRAF writes a minimal valid RAF file with the standard fake CFA
// payload (0xDEADBEEFCAFEBABE). Use buildFakeRAFWithCFA for a custom payload.
func buildFakeRAF(t *testing.T, dir, name string) string {
	t.Helper()
	return buildFakeRAFWithCFA(t, dir, name, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE})
}

// buildFakeRAFWithCFA writes a minimal valid RAF file with the given CFA payload.
//
// File layout:
//
//	[RAF header: headerMinSize bytes]
//	[embedded JPEG: len(minimalJPEG) bytes]
//	[CFA data: len(cfaData) bytes]
//
// The offset directory in the header is populated with the correct offsets
// and lengths for the JPEG and CFA regions.
func buildFakeRAFWithCFA(t *testing.T, dir, name string, cfaData []byte) string {
	t.Helper()

	jpegOffset := uint32(headerMinSize)
	jpegLength := uint32(len(minimalJPEG))
	cfaOffset := jpegOffset + jpegLength
	cfaLength := uint32(len(cfaData))

	buf := new(bytes.Buffer)

	// --- RAF header ---

	// 0x00: magic (16 bytes)
	buf.WriteString(rafMagic)

	// 0x10: format version (4 bytes)
	buf.WriteString("0201")

	// 0x14: camera ID (8 bytes, zeros)
	buf.Write(make([]byte, 8))

	// 0x1C: camera model (32 bytes, null-padded)
	model := make([]byte, 32)
	copy(model, "FakeCamera")
	buf.Write(model)

	// 0x3C: RAF version (4 bytes)
	buf.WriteString("0100")

	// 0x40: unknown (20 bytes, zeros) — fills gap to 0x54
	buf.Write(make([]byte, 20))

	// 0x54: JPEG offset (4 bytes, big-endian)
	_ = binary.Write(buf, binary.BigEndian, jpegOffset)

	// 0x58: JPEG length (4 bytes, big-endian)
	_ = binary.Write(buf, binary.BigEndian, jpegLength)

	// 0x5C: meta offset (4 bytes, big-endian, 0 = no meta section)
	_ = binary.Write(buf, binary.BigEndian, uint32(0))

	// 0x60: meta length (4 bytes, big-endian, 0)
	_ = binary.Write(buf, binary.BigEndian, uint32(0))

	// 0x64: CFA offset (4 bytes, big-endian)
	_ = binary.Write(buf, binary.BigEndian, cfaOffset)

	// 0x68: CFA length (4 bytes, big-endian)
	_ = binary.Write(buf, binary.BigEndian, cfaLength)

	// Verify we've written exactly headerMinSize bytes so far.
	if buf.Len() != headerMinSize {
		t.Fatalf("buildFakeRAFWithCFA: header is %d bytes, want %d", buf.Len(), headerMinSize)
	}

	// --- Embedded JPEG ---
	buf.Write(minimalJPEG)

	// --- CFA data ---
	buf.Write(cfaData)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
