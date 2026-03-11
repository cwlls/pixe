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

package heic

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
)

// buildFakeHEIC creates a minimal file with the ISOBMFF ftyp box signature
// at offset 4 so Detect() returns true. It is not a valid HEIC image.
func buildFakeHEIC(t *testing.T, dir, name string) string {
	t.Helper()
	// ftyp box: size(4) + "ftyp"(4) + brand "heic"(4) + minor(4) = 16 bytes
	data := []byte{
		0x00, 0x00, 0x00, 0x18, // box size = 24
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x68, 0x65, 0x69, 0x63, // major brand "heic"
		0x00, 0x00, 0x00, 0x00, // minor version
		0x68, 0x65, 0x69, 0x63, // compat brand "heic"
		0x6d, 0x69, 0x66, 0x31, // compat brand "mif1"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("buildFakeHEIC: %v", err)
	}
	return path
}

func TestHandler_Extensions(t *testing.T) {
	h := New()
	exts := h.Extensions()
	want := map[string]bool{".heic": true, ".heif": true}
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
	h := New()
	sigs := h.MagicBytes()
	if len(sigs) == 0 {
		t.Fatal("MagicBytes returned empty slice")
	}
	sig := sigs[0]
	if sig.Offset != 4 {
		t.Errorf("magic offset = %d, want 4", sig.Offset)
	}
	// Should be "ftyp"
	want := []byte{0x66, 0x74, 0x79, 0x70}
	for i, b := range want {
		if i >= len(sig.Bytes) || sig.Bytes[i] != b {
			t.Errorf("magic bytes[%d] = 0x%02X, want 0x%02X", i, sig.Bytes[i], b)
		}
	}
}

func TestHandler_Detect_validHEIC(t *testing.T) {
	dir := t.TempDir()
	path := buildFakeHEIC(t, dir, "photo.heic")

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !ok {
		t.Error("Detect returned false for valid HEIC signature")
	}
}

func TestHandler_Detect_wrongExtension(t *testing.T) {
	dir := t.TempDir()
	// HEIC magic bytes but .jpg extension — should return false.
	path := buildFakeHEIC(t, dir, "photo.jpg")

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ok {
		t.Error("Detect should return false for .jpg extension even with HEIC content")
	}
}

func TestHandler_Detect_notHEIC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.heic")
	if err := os.WriteFile(path, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ok {
		t.Error("Detect should return false for JPEG bytes in .heic file")
	}
}

func TestHandler_ExtractDate_noEXIF_fallback(t *testing.T) {
	dir := t.TempDir()
	// Fake HEIC with no EXIF — should fall back to Ansel Adams.
	path := buildFakeHEIC(t, dir, "no_exif.heic")

	h := New()
	got, err := h.ExtractDate(path)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate fallback = %v, want %v", got, want)
	}
}

func TestHandler_HashableReader_returnsData(t *testing.T) {
	dir := t.TempDir()
	path := buildFakeHEIC(t, dir, "photo.heic")

	h := New()
	rc, err := h.HashableReader(path)
	if err != nil {
		t.Fatalf("HashableReader: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(data) == 0 {
		t.Error("HashableReader returned empty data")
	}
}

func TestHandler_HashableReader_deterministic(t *testing.T) {
	dir := t.TempDir()
	path := buildFakeHEIC(t, dir, "photo.heic")

	h := New()
	read := func() []byte {
		rc, err := h.HashableReader(path)
		if err != nil {
			t.Fatalf("HashableReader: %v", err)
		}
		defer func() { _ = rc.Close() }()
		data, _ := io.ReadAll(rc)
		return data
	}

	d1 := read()
	d2 := read()
	if len(d1) != len(d2) {
		t.Errorf("HashableReader not deterministic: %d vs %d bytes", len(d1), len(d2))
	}
}

func TestHandler_MetadataSupport(t *testing.T) {
	h := New()
	got := h.MetadataSupport()
	if got != domain.MetadataSidecar {
		t.Errorf("MetadataSupport() = %v, want MetadataSidecar", got)
	}
}

// TestHandler_WriteMetadataTags_noop verifies WriteMetadataTags is a no-op
// retained for interface compliance. The pipeline no longer calls this directly.
func TestHandler_WriteMetadataTags_noop(t *testing.T) {
	dir := t.TempDir()
	path := buildFakeHEIC(t, dir, "photo.heic")

	statBefore, _ := os.Stat(path)
	h := New()
	if err := h.WriteMetadataTags(path, domain.MetadataTags{Copyright: "Test"}); err != nil {
		t.Fatalf("WriteMetadataTags: %v", err)
	}
	statAfter, _ := os.Stat(path)
	if statBefore.ModTime() != statAfter.ModTime() {
		t.Error("WriteMetadataTags modified the file (should be no-op for HEIC)")
	}
}
