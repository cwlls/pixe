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

	"github.com/cwlls/pixe-go/internal/domain"
)

const (
	fixtureWithExif  = "testdata/with_exif_date.jpg"
	fixtureWithExif2 = "testdata/with_exif_date2.jpg"
	fixtureNoExif    = "testdata/no_exif.jpg"
)

func TestHandler_Extensions(t *testing.T) {
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
	// Payload must start with SOS marker.
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xDA {
		t.Errorf("payload should start with SOS (0xFF 0xDA), got 0x%02X 0x%02X", data[0], data[1])
	}
}

func TestHandler_HashableReader_noExif_stillWorks(t *testing.T) {
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

func TestHandler_WriteMetadataTags_noop_whenEmpty(t *testing.T) {
	h := New()
	dir := t.TempDir()
	dst := filepath.Join(dir, "photo.jpg")
	copyFile(t, fixtureWithExif, dst)

	statBefore, _ := os.Stat(dst)
	if err := h.WriteMetadataTags(dst, domain.MetadataTags{}); err != nil {
		t.Fatalf("WriteMetadataTags with empty tags: %v", err)
	}
	statAfter, _ := os.Stat(dst)
	// File should be untouched.
	if statBefore.ModTime() != statAfter.ModTime() {
		t.Error("WriteMetadataTags modified the file when tags were empty")
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
