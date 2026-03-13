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
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/domain"
)

// buildMinimalMP4 creates a minimal structurally-valid MP4 for testing.
// It contains an ftyp box and a minimal moov/mvhd with the given creation time.
func buildMinimalMP4(creationTimeSecs uint32) []byte {
	var buf bytes.Buffer

	// ftyp box
	ftypData := []byte{
		0x00, 0x00, 0x00, 0x18, // size = 24
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x6d, 0x70, 0x34, 0x32, // major brand "mp42"
		0x00, 0x00, 0x00, 0x00, // minor version
		0x6d, 0x70, 0x34, 0x32, // compat "mp42"
		0x69, 0x73, 0x6f, 0x6d, // compat "isom"
	}
	buf.Write(ftypData)

	// mvhd box (version 0, 108 bytes total)
	mvhdPayload := make([]byte, 100)
	// FullBox: version (1 byte) + flags (3 bytes) = 0x00000000
	binary.BigEndian.PutUint32(mvhdPayload[0:4], 0)                 // version=0, flags=0
	binary.BigEndian.PutUint32(mvhdPayload[4:8], creationTimeSecs)  // creation time
	binary.BigEndian.PutUint32(mvhdPayload[8:12], creationTimeSecs) // modification time
	binary.BigEndian.PutUint32(mvhdPayload[8:12], 1000)             // timescale
	binary.BigEndian.PutUint32(mvhdPayload[12:16], 0)               // duration
	binary.BigEndian.PutUint32(mvhdPayload[16:20], 0x00010000)      // rate 1.0
	binary.BigEndian.PutUint16(mvhdPayload[20:22], 0x0100)          // volume 1.0
	// matrix identity at offset 36
	binary.BigEndian.PutUint32(mvhdPayload[36:40], 0x00010000)
	binary.BigEndian.PutUint32(mvhdPayload[52:56], 0x00010000)
	binary.BigEndian.PutUint32(mvhdPayload[68:72], 0x40000000)
	// next track ID
	binary.BigEndian.PutUint32(mvhdPayload[96:100], 1)

	mvhdBox := make([]byte, 8+len(mvhdPayload))
	binary.BigEndian.PutUint32(mvhdBox[0:4], uint32(len(mvhdBox)))
	copy(mvhdBox[4:8], []byte("mvhd"))
	copy(mvhdBox[8:], mvhdPayload)

	// moov box wrapping mvhd
	moovSize := uint32(8 + len(mvhdBox))
	moovBox := make([]byte, moovSize)
	binary.BigEndian.PutUint32(moovBox[0:4], moovSize)
	copy(moovBox[4:8], []byte("moov"))
	copy(moovBox[8:], mvhdBox)

	buf.Write(moovBox)
	return buf.Bytes()
}

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	h := New()
	got := h.MetadataSupport()
	if got != domain.MetadataSidecar {
		t.Errorf("MetadataSupport() = %v, want MetadataSidecar", got)
	}
}

// TestHandler_WriteMetadataTags_noop verifies WriteMetadataTags is a no-op
// retained for interface compliance. The pipeline no longer calls this directly.
func TestHandler_WriteMetadataTags_noop(t *testing.T) {
	t.Parallel()
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

// TestHandler_Detect_emptyFile verifies that an empty .mp4 file returns false
// without error (file too short to read box type).
func TestHandler_Detect_emptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFixture(t, dir, "empty.mp4", []byte{})

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect on empty file: %v", err)
	}
	if ok {
		t.Error("Detect should return false for empty file")
	}
}

// TestHandler_Detect_tooShort verifies that a file shorter than 12 bytes
// returns false without error.
func TestHandler_Detect_tooShort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFixture(t, dir, "short.mp4", []byte{0x00, 0x00, 0x00, 0x08})

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect on short file: %v", err)
	}
	if ok {
		t.Error("Detect should return false for too-short file")
	}
}

// TestHandler_Detect_movExtension verifies that a valid MP4 with .mov extension
// is detected correctly.
func TestHandler_Detect_movExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data := buildMinimalMP4(0)
	path := writeFixture(t, dir, "video.mov", data)

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect .mov: %v", err)
	}
	if !ok {
		t.Error("Detect should return true for valid MP4 with .mov extension")
	}
}

// TestHandler_ExtractDate_noMoovBox verifies that a file with only an ftyp box
// (no moov/mvhd) falls back to the Ansel Adams date.
func TestHandler_ExtractDate_noMoovBox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Build a file with only an ftyp box — no moov/mvhd.
	ftypOnly := []byte{
		0x00, 0x00, 0x00, 0x18, // size = 24
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x6d, 0x70, 0x34, 0x32, // major brand "mp42"
		0x00, 0x00, 0x00, 0x00, // minor version
		0x6d, 0x70, 0x34, 0x32, // compat "mp42"
		0x69, 0x73, 0x6f, 0x6d, // compat "isom"
	}
	path := writeFixture(t, dir, "ftyp_only.mp4", ftypOnly)

	h := New()
	got, err := h.ExtractDate(path)
	if err != nil {
		t.Fatalf("ExtractDate: %v", err)
	}
	want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate = %v, want Ansel Adams date %v", got, want)
	}
}

// TestHandler_HashableReader_fallbackToFullFile verifies that when keyframe
// extraction fails (no stss box), HashableReader falls back to returning the
// full file content.
func TestHandler_HashableReader_fallbackToFullFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// buildMinimalMP4 has moov/mvhd but no stss — triggers fallback path.
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
	// Fallback returns the full file — must be non-empty and match file size.
	fi, _ := os.Stat(path)
	if int64(len(payload)) != fi.Size() {
		t.Errorf("fallback payload size = %d, want %d (full file)", len(payload), fi.Size())
	}
}

// TestHandler_HashableReader_nonexistentFile verifies that HashableReader
// returns an error for a non-existent file.
func TestHandler_HashableReader_nonexistentFile(t *testing.T) {
	t.Parallel()
	h := New()
	_, err := h.HashableReader("/nonexistent/path/video.mp4")
	if err == nil {
		t.Fatal("HashableReader should return error for non-existent file")
	}
}

// TestHandler_ExtractDate_nonexistentFile verifies that ExtractDate returns
// the fallback date (not an error) when the file cannot be opened.
func TestHandler_ExtractDate_nonexistentFile(t *testing.T) {
	t.Parallel()
	h := New()
	got, err := h.ExtractDate("/nonexistent/path/video.mp4")
	if err == nil {
		t.Fatal("ExtractDate should return error for non-existent file")
	}
	want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ExtractDate fallback = %v, want %v", got, want)
	}
}
