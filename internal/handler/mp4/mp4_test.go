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

package mp4

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
)

// writeFixture writes data to dir/name and returns the path.
func writeFixture(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeFixture: %v", err)
	}
	return path
}

func TestHandler_Extensions(t *testing.T) {
	h := New()
	exts := h.Extensions()
	want := map[string]bool{".mp4": true, ".mov": true}
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
	// All signatures should be at offset 4.
	for _, sig := range sigs {
		if sig.Offset != 4 {
			t.Errorf("magic offset = %d, want 4", sig.Offset)
		}
	}
}

func TestHandler_Detect_validMP4(t *testing.T) {
	dir := t.TempDir()
	data := buildMinimalMP4(0)
	path := writeFixture(t, dir, "video.mp4", data)

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !ok {
		t.Error("Detect returned false for valid MP4")
	}
}

func TestHandler_Detect_wrongExtension(t *testing.T) {
	dir := t.TempDir()
	data := buildMinimalMP4(0)
	path := writeFixture(t, dir, "video.jpg", data) // wrong extension

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ok {
		t.Error("Detect should return false for .jpg extension even with MP4 content")
	}
}

func TestHandler_Detect_notMP4(t *testing.T) {
	dir := t.TempDir()
	path := writeFixture(t, dir, "fake.mp4", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01})

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ok {
		t.Error("Detect should return false for JPEG bytes in .mp4 file")
	}
}

func TestHandler_ExtractDate_withCreationTime(t *testing.T) {
	// 2021-12-25 06:22:23 UTC expressed as seconds since 1904-01-01.
	target := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	secs := uint32(target.Sub(mp4Epoch).Seconds())

	dir := t.TempDir()
	data := buildMinimalMP4(secs)
	path := writeFixture(t, dir, "video.mp4", data)

	h := New()
	got, err := h.ExtractDate(path)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	if !got.Equal(target) {
		t.Errorf("ExtractDate = %v, want %v", got, target)
	}
}

func TestHandler_ExtractDate_zeroTime_fallback(t *testing.T) {
	dir := t.TempDir()
	data := buildMinimalMP4(0) // creation time = 0 → fallback
	path := writeFixture(t, dir, "video.mp4", data)

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
	data := buildMinimalMP4(0)
	path := writeFixture(t, dir, "video.mp4", data)

	h := New()
	rc, err := h.HashableReader(path)
	if err != nil {
		t.Fatalf("HashableReader: %v", err)
	}
	defer func() { _ = rc.Close() }()

	payload, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(payload) == 0 {
		t.Error("HashableReader returned empty payload")
	}
}

func TestHandler_HashableReader_deterministic(t *testing.T) {
	dir := t.TempDir()
	data := buildMinimalMP4(0)
	path := writeFixture(t, dir, "video.mp4", data)

	h := New()
	read := func() []byte {
		rc, err := h.HashableReader(path)
		if err != nil {
			t.Fatalf("HashableReader: %v", err)
		}
		defer func() { _ = rc.Close() }()
		b, _ := io.ReadAll(rc)
		return b
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
	data := buildMinimalMP4(0)
	path := writeFixture(t, dir, "video.mp4", data)

	statBefore, _ := os.Stat(path)
	h := New()
	if err := h.WriteMetadataTags(path, domain.MetadataTags{Copyright: "Test"}); err != nil {
		t.Fatalf("WriteMetadataTags: %v", err)
	}
	statAfter, _ := os.Stat(path)
	if statBefore.ModTime() != statAfter.ModTime() {
		t.Error("WriteMetadataTags modified the file (should be no-op for MP4)")
	}
}
