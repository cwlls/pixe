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
	"time"

	"github.com/cwlls/pixe/internal/domain"
)

const (
	fixtureWithExif  = "testdata/with_exif_date.jpg"
	fixtureWithExif2 = "testdata/with_exif_date2.jpg"
	fixtureNoExif    = "testdata/no_exif.jpg"
)

func TestHandler_Extensions(t *testing.T) {
	t.Parallel()
	h := New()
	exts := h.Extensions()
	want := map[string]bool{".jpg": true, ".jpeg": true}
	for _, e := range exts {
		if !want[e] {
			t.Errorf("unexpected extension %q", e)
		}
		delete(want, e)
	}
	for e := range want {
		t.Errorf("missing extension %q", e)
	}
}

func TestHandler_MagicBytes(t *testing.T) {
	t.Parallel()
	h := New()
	sigs := h.MagicBytes()
	if len(sigs) == 0 {
		t.Fatal("MagicBytes returned empty slice")
	}
	sig := sigs[0]
	if sig.Offset != 0 {
		t.Errorf("magic offset = %d, want 0", sig.Offset)
	}
	if len(sig.Bytes) < 3 || sig.Bytes[0] != 0xFF || sig.Bytes[1] != 0xD8 || sig.Bytes[2] != 0xFF {
		t.Errorf("magic bytes = %v, want [0xFF 0xD8 0xFF ...]", sig.Bytes)
	}
}

func TestHandler_Detect_validJPEG(t *testing.T) {
	t.Parallel()
	h := New()
	ok, err := h.Detect(fixtureWithExif)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !ok {
		t.Error("Detect returned false for valid JPEG")
	}
}

func TestHandler_Detect_wrongExtension(t *testing.T) {
	t.Parallel()
	h := New()
	// Copy the JPEG to a .png path — extension mismatch should return false.
	dir := t.TempDir()
	dst := filepath.Join(dir, "photo.png")
	copyFile(t, fixtureWithExif, dst)

	ok, err := h.Detect(dst)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ok {
		t.Error("Detect should return false for .png extension even with JPEG content")
	}
}

func TestHandler_Detect_notJPEG(t *testing.T) {
	t.Parallel()
	h := New()
	dir := t.TempDir()
	f := filepath.Join(dir, "fake.jpg")
	if err := os.WriteFile(f, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := h.Detect(f)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ok {
		t.Error("Detect should return false for non-JPEG bytes")
	}
}

func TestHandler_ExtractDate_withEXIF(t *testing.T) {
	t.Parallel()
	h := New()
	got, err := h.ExtractDate(fixtureWithExif)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	// Fixture was built with date "2021:12:25 06:22:23"
	want := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate = %v, want %v", got, want)
	}
}

func TestHandler_ExtractDate_noEXIF_fallback(t *testing.T) {
	t.Parallel()
	h := New()
	got, err := h.ExtractDate(fixtureNoExif)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	// Should fall back to Ansel Adams' birthday.
	want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate fallback = %v, want %v (Ansel Adams)", got, want)
	}
}

func TestHandler_HashableReader_returnsData(t *testing.T) {
	t.Parallel()
	h := New()
	rc, err := h.HashableReader(fixtureWithExif)
	if err != nil {
		t.Fatalf("HashableReader: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(data) == 0 {
		t.Error("HashableReader returned empty payload")
	}
	// Full-file hash: payload must start with JPEG SOI marker (0xFF 0xD8).
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Errorf("payload should start with SOI (0xFF 0xD8), got 0x%02X 0x%02X", data[0], data[1])
	}
}

func TestHandler_HashableReader_noExif_stillWorks(t *testing.T) {
	t.Parallel()
	h := New()
	rc, err := h.HashableReader(fixtureNoExif)
	if err != nil {
		t.Fatalf("HashableReader on no-EXIF file: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(data) == 0 {
		t.Error("HashableReader returned empty payload for no-EXIF file")
	}
}

func TestHandler_HashableReader_deterministic(t *testing.T) {
	t.Parallel()
	// Hashing the same file twice must produce the same bytes.
	h := New()

	read := func() []byte {
		rc, err := h.HashableReader(fixtureWithExif)
		if err != nil {
			t.Fatalf("HashableReader: %v", err)
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		return data
	}

	d1 := read()
	d2 := read()
	if len(d1) != len(d2) {
		t.Errorf("HashableReader not deterministic: lengths %d vs %d", len(d1), len(d2))
	}
}

// TestHandler_HashableReader_fullFile verifies that HashableReader returns the
// complete file contents (full-file hash, not a data-region subset).
func TestHandler_HashableReader_fullFile(t *testing.T) {
	t.Parallel()
	h := New()
	rc, err := h.HashableReader(fixtureWithExif)
	if err != nil {
		t.Fatalf("HashableReader: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	want, err := os.ReadFile(fixtureWithExif)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if len(got) != len(want) {
		t.Errorf("HashableReader returned %d bytes, want %d (full file)", len(got), len(want))
	}
}

func TestHandler_MetadataSupport(t *testing.T) {
	t.Parallel()
	h := New()
	got := h.MetadataSupport()
	if got != domain.MetadataSidecar {
		t.Errorf("MetadataSupport() = %v, want MetadataSidecar", got)
	}
}

// TestHandler_WriteMetadataTags_noop verifies that WriteMetadataTags is a
// no-op retained for interface compliance — it never modifies any file.
func TestHandler_WriteMetadataTags_noop(t *testing.T) {
	t.Parallel()
	h := New()
	dir := t.TempDir()
	dst := filepath.Join(dir, "photo.jpg")
	copyFile(t, fixtureWithExif, dst)

	statBefore, _ := os.Stat(dst)
	tags := domain.MetadataTags{Copyright: "Copyright 2026 Test", CameraOwner: "Test Owner"}
	if err := h.WriteMetadataTags(dst, tags); err != nil {
		t.Fatalf("WriteMetadataTags: %v", err)
	}
	statAfter, _ := os.Stat(dst)
	// File must be completely untouched — no modification, even with non-empty tags.
	if statBefore.ModTime() != statAfter.ModTime() {
		t.Error("WriteMetadataTags modified the file — it must be a no-op")
	}
	if statBefore.Size() != statAfter.Size() {
		t.Error("WriteMetadataTags changed file size — it must be a no-op")
	}
}

// buildMinimalJPEG builds a minimal valid JPEG: SOI + APP0 (JFIF) + SOS + EOI.
// It has no EXIF data, so ExtractDate should return the Ansel Adams fallback.
func buildMinimalJPEG() []byte {
	var buf []byte
	// SOI
	buf = append(buf, 0xFF, 0xD8)
	// APP0 (JFIF) — minimal 16-byte segment
	app0 := []byte{
		0xFF, 0xE0, // APP0 marker
		0x00, 0x10, // length = 16 (includes the 2 length bytes)
		0x4A, 0x46, 0x49, 0x46, 0x00, // "JFIF\0"
		0x01, 0x01, // version 1.1
		0x00,       // aspect ratio units
		0x00, 0x01, // X density
		0x00, 0x01, // Y density
		0x00, 0x00, // thumbnail size
	}
	buf = append(buf, app0...)
	// SOS marker (start of scan)
	buf = append(buf, 0xFF, 0xDA)
	// SOS segment length (must be ≥ 2)
	buf = append(buf, 0x00, 0x08)
	// SOS header: 1 component, Cs=1, Td=0, Ta=0, Ss=0, Se=63, Ah=0, Al=0
	buf = append(buf, 0x01, 0x01, 0x00, 0x3F, 0x00, 0x00)
	// Minimal entropy-coded data (a few bytes)
	buf = append(buf, 0x7F, 0xA0)
	// EOI
	buf = append(buf, 0xFF, 0xD9)
	return buf
}

// buildCorruptEXIFJPEG builds a JPEG with a malformed APP1 (EXIF) segment.
func buildCorruptEXIFJPEG() []byte {
	var buf []byte
	// SOI
	buf = append(buf, 0xFF, 0xD8)
	// APP1 with corrupt EXIF data (wrong magic)
	corruptPayload := []byte{
		0x45, 0x58, 0x49, 0x46, 0x00, 0x00, // "EXIF\0\0" header
		0xFF, 0xFF, 0xFF, 0xFF, // garbage TIFF header
	}
	segLen := uint16(len(corruptPayload) + 2) // +2 for length field itself
	buf = append(buf, 0xFF, 0xE1)             // APP1 marker
	buf = append(buf, byte(segLen>>8), byte(segLen))
	buf = append(buf, corruptPayload...)
	// SOS
	buf = append(buf, 0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x3F, 0x00, 0x00)
	buf = append(buf, 0x7F, 0xA0)
	// EOI
	buf = append(buf, 0xFF, 0xD9)
	return buf
}

func TestHandler_ExtractDate_corruptEXIF_fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt_exif.jpg")
	if err := os.WriteFile(path, buildCorruptEXIFJPEG(), 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	got, err := h.ExtractDate(path)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate corrupt EXIF = %v, want Ansel Adams %v", got, want)
	}
}

func TestHandler_ExtractDate_bareJPEG_fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bare.jpg")
	if err := os.WriteFile(path, buildMinimalJPEG(), 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	got, err := h.ExtractDate(path)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate bare JPEG = %v, want Ansel Adams %v", got, want)
	}
}

func TestHandler_ExtractDate_truncatedFile_fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Just SOI + a few bytes — truncated before any APP segment.
	path := filepath.Join(dir, "truncated.jpg")
	if err := os.WriteFile(path, []byte{0xFF, 0xD8, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	got, err := h.ExtractDate(path)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate truncated = %v, want Ansel Adams %v", got, want)
	}
}

func TestHandler_HashableReader_nonexistentFile(t *testing.T) {
	t.Parallel()
	h := New()
	_, err := h.HashableReader("/nonexistent/path/photo.jpg")
	if err == nil {
		t.Fatal("HashableReader should return error for non-existent file")
	}
}

// TestHandler_HashableReader_truncatedJPEG verifies that a truncated JPEG
// (SOI only) is still hashable — full-file hashing has no SOS requirement.
func TestHandler_HashableReader_truncatedJPEG(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "truncated.jpg")
	// SOI only — no segments, no SOS
	if err := os.WriteFile(path, []byte{0xFF, 0xD8}, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	rc, err := h.HashableReader(path)
	if err != nil {
		t.Fatalf("HashableReader should succeed for any openable file, got: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	// Should return the 2 SOI bytes.
	if len(data) != 2 {
		t.Errorf("expected 2 bytes for SOI-only file, got %d", len(data))
	}
}

func TestHandler_Detect_pngContentJpgExtension(t *testing.T) {
	t.Parallel()
	// PNG magic bytes in a .jpg file — Detect should return false.
	h := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.jpg")
	// PNG magic: 0x89 0x50 0x4E 0x47
	if err := os.WriteFile(path, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ok {
		t.Error("Detect should return false for PNG content in .jpg file")
	}
}

// copyFile copies src to dst for test isolation.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("copyFile read %q: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("copyFile write %q: %v", dst, err)
	}
}
