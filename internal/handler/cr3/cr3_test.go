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

package cr3

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
	t.Parallel()
	h := New()
	exts := h.Extensions()
	if len(exts) != 1 || exts[0] != ".cr3" {
		t.Fatalf("Expected [.cr3], got %v", exts)
	}
}

// TestHandler_MagicBytes verifies the correct magic signature is returned.
func TestHandler_MagicBytes(t *testing.T) {
	t.Parallel()
	h := New()
	sigs := h.MagicBytes()
	if len(sigs) != 1 {
		t.Fatalf("Expected 1 magic signature, got %d", len(sigs))
	}
	if sigs[0].Offset != 4 || !bytes.Equal(sigs[0].Bytes, []byte{0x66, 0x74, 0x79, 0x70}) {
		t.Fatalf("Magic signature mismatch: %v", sigs[0])
	}
}

// TestHandler_Detect_valid verifies detection with correct extension and magic.
func TestHandler_Detect_valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeCR3(t, dir, "test.cr3")

	h := New()
	ok, err := h.Detect(filePath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !ok {
		t.Fatal("Detect should return true for valid CR3")
	}
}

// TestHandler_Detect_wrongExtension verifies detection fails with wrong extension.
func TestHandler_Detect_wrongExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeCR3(t, dir, "test.heic")

	h := New()
	ok, err := h.Detect(filePath)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for wrong extension")
	}
}

// TestHandler_Detect_heicBrand verifies detection fails for HEIC brand.
func TestHandler_Detect_heicBrand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cr3")

	// ftyp box with "heic" brand instead of "crx "
	ftyp := []byte{
		0x00, 0x00, 0x00, 0x14, // size = 20
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x68, 0x65, 0x69, 0x63, // "heic" brand
		0x00, 0x00, 0x00, 0x01, // minor version
		0x68, 0x65, 0x69, 0x63, // compat "heic"
	}
	if err := os.WriteFile(path, ftyp, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for HEIC brand")
	}
}

// TestHandler_Detect_mp4Brand verifies detection fails for MP4 brand.
func TestHandler_Detect_mp4Brand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cr3")

	// ftyp box with "isom" brand instead of "crx "
	ftyp := []byte{
		0x00, 0x00, 0x00, 0x14, // size = 20
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x69, 0x73, 0x6F, 0x6D, // "isom" brand
		0x00, 0x00, 0x00, 0x01, // minor version
		0x69, 0x73, 0x6F, 0x6D, // compat "isom"
	}
	if err := os.WriteFile(path, ftyp, 0o644); err != nil {
		t.Fatal(err)
	}

	h := New()
	ok, err := h.Detect(path)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if ok {
		t.Fatal("Detect should return false for MP4 brand")
	}
}

// TestHandler_ExtractDate_noEXIF_fallback verifies fallback to Ansel Adams date.
func TestHandler_ExtractDate_noEXIF_fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeCR3(t, dir, "test.cr3")

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
// The fake CR3 has no moov track metadata, so it falls back to the full mdat.
func TestHandler_HashableReader_returnsData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeCR3(t, dir, "test.cr3")

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
	t.Parallel()
	dir := t.TempDir()
	filePath := buildFakeCR3(t, dir, "test.cr3")

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

// TestHandler_HashableReader_returnsSensorData verifies that when a CR3 file
// has moov track metadata pointing into mdat, HashableReader returns the
// sensor data region (not the full mdat).
func TestHandler_HashableReader_returnsSensorData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sensorBytes := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
	filePath := buildCR3WithSensorData(t, dir, "test.cr3", sensorBytes)

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

	if !bytes.Equal(data, sensorBytes) {
		t.Fatalf("HashableReader returned %x, want sensor data %x", data, sensorBytes)
	}
}

// TestFindCR3SensorData_noMdat verifies that findCR3SensorData returns nil
// for a CR3 file with no mdat box.
func TestFindCR3SensorData_noMdat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cr3")

	// CR3 with only ftyp box, no mdat.
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, uint32(20))
	buf.WriteString("ftyp")
	buf.WriteString("crx ")
	_ = binary.Write(buf, binary.BigEndian, uint32(1))
	buf.WriteString("crx ")

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	sensor, err := findCR3SensorData(f)
	if err != nil {
		t.Fatalf("findCR3SensorData returned error: %v", err)
	}
	if sensor != nil {
		t.Fatalf("findCR3SensorData should return nil for CR3 with no mdat, got %+v", sensor)
	}
}

// TestFindCR3SensorData_fallbackFullMdat verifies that when moov track
// metadata cannot be parsed, findCR3SensorData falls back to the full mdat.
func TestFindCR3SensorData_fallbackFullMdat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// buildFakeCR3 has a moov box with no trak children, so track parsing
	// returns nil and the fallback to full mdat is triggered.
	filePath := buildFakeCR3(t, dir, "test.cr3")

	f, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	sensor, err := findCR3SensorData(f)
	if err != nil {
		t.Fatalf("findCR3SensorData returned error: %v", err)
	}
	if sensor == nil {
		t.Fatal("findCR3SensorData should fall back to mdat, got nil")
	}
	if sensor.size == 0 {
		t.Fatal("fallback mdat sensor region has zero size")
	}
}

// TestHandler_MetadataSupport verifies that the CR3 handler declares MetadataSidecar.
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
	h := New()
	tags := domain.MetadataTags{Copyright: "test", CameraOwner: "test"}
	err := h.WriteMetadataTags("dummy.cr3", tags)
	if err != nil {
		t.Fatalf("WriteMetadataTags should be no-op, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test fixture builders
// ---------------------------------------------------------------------------

// buildFakeCR3 writes a minimal CR3 file with ftyp box, a moov box (no trak
// children), and an mdat box with dummy sensor data bytes.
//
// Structure:
//
//	ftyp box: size(4) + "ftyp"(4) + "crx "(4) + minor_version(4) + compat(4) = 20 bytes
//	moov box: size(4) + "moov"(4) = 8 bytes (empty, no children)
//	mdat box: size(4) + "mdat"(4) + dummy data(8) = 16 bytes
func buildFakeCR3(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// ftyp box: size = 20
	_ = binary.Write(buf, binary.BigEndian, uint32(20))
	buf.WriteString("ftyp")
	buf.WriteString("crx ")
	_ = binary.Write(buf, binary.BigEndian, uint32(1)) // minor version
	buf.WriteString("crx ")                            // compat

	// moov box: size = 8 (header only, no children — triggers fallback)
	_ = binary.Write(buf, binary.BigEndian, uint32(8))
	buf.WriteString("moov")

	// mdat box: size = 16 (8 bytes of dummy sensor data)
	_ = binary.Write(buf, binary.BigEndian, uint32(16))
	buf.WriteString("mdat")
	_ = binary.Write(buf, binary.BigEndian, uint64(0xDEADBEEFCAFEBABE)) // dummy sensor data

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildCR3WithSensorData writes a CR3 file with a complete moov → trak →
// mdia → minf → stbl structure pointing to known sensor data bytes in mdat.
//
// This fixture exercises the full track-parsing path in findCR3SensorData.
//
// Layout (all sizes in bytes):
//
//	ftyp (20) | moov (variable) | mdat (8 + len(sensorBytes))
//
// The moov box contains a single trak with stco pointing to the mdat data offset.
func buildCR3WithSensorData(t *testing.T, dir, name string, sensorBytes []byte) string {
	t.Helper()

	// We build the file in two passes:
	// Pass 1: build moov content (stbl with placeholder offsets).
	// Pass 2: fix up the stco chunk offset once we know the mdat position.
	//
	// Simpler approach: pre-calculate all offsets.

	ftypSize := uint32(20)

	// Build stbl content: stsz + stco
	// stsz: version(1)+flags(3)+sample_size(4)+sample_count(4)+[sizes] = 12 + 4*1 = 16 bytes
	// stco: version(1)+flags(3)+entry_count(4)+[offsets] = 8 + 4*1 = 12 bytes
	// stbl box header = 8 bytes
	// stbl total = 8 + 16 + 12 = 36 bytes... but we need to include stsz and stco box headers.
	// Each child box has an 8-byte header.
	// stsz box: 8 (hdr) + 4 (version+flags) + 4 (sample_size=0) + 4 (sample_count=1) + 4 (entry) = 24 bytes
	// stco box: 8 (hdr) + 4 (version+flags) + 4 (entry_count=1) + 4 (offset) = 20 bytes
	// stbl box: 8 (hdr) + 24 + 20 = 52 bytes

	stszSize := uint32(24)
	stcoSize := uint32(20)
	stblSize := uint32(8) + stszSize + stcoSize // 52

	// minf box: 8 (hdr) + stbl = 8 + 52 = 60
	minfSize := uint32(8) + stblSize // 60

	// mdia box: 8 (hdr) + minf = 8 + 60 = 68
	mdiaSize := uint32(8) + minfSize // 68

	// trak box: 8 (hdr) + mdia = 8 + 68 = 76
	trakSize := uint32(8) + mdiaSize // 76

	// moov box: 8 (hdr) + trak = 8 + 76 = 84
	moovSize := uint32(8) + trakSize // 84

	// mdat box: 8 (hdr) + sensorBytes
	mdatSize := uint32(8) + uint32(len(sensorBytes))

	// mdat data starts at: ftypSize + moovSize + 8 (mdat header)
	mdatDataOffset := ftypSize + moovSize + 8

	buf := new(bytes.Buffer)

	// ftyp box.
	_ = binary.Write(buf, binary.BigEndian, ftypSize)
	buf.WriteString("ftyp")
	buf.WriteString("crx ")
	_ = binary.Write(buf, binary.BigEndian, uint32(1))
	buf.WriteString("crx ")

	// moov box.
	_ = binary.Write(buf, binary.BigEndian, moovSize)
	buf.WriteString("moov")

	// trak box.
	_ = binary.Write(buf, binary.BigEndian, trakSize)
	buf.WriteString("trak")

	// mdia box.
	_ = binary.Write(buf, binary.BigEndian, mdiaSize)
	buf.WriteString("mdia")

	// minf box.
	_ = binary.Write(buf, binary.BigEndian, minfSize)
	buf.WriteString("minf")

	// stbl box.
	_ = binary.Write(buf, binary.BigEndian, stblSize)
	buf.WriteString("stbl")

	// stsz box: uniform sample size = 0 (per-sample), sample_count = 1, entry = len(sensorBytes).
	_ = binary.Write(buf, binary.BigEndian, stszSize)
	buf.WriteString("stsz")
	_ = binary.Write(buf, binary.BigEndian, uint32(0)) // version + flags
	_ = binary.Write(buf, binary.BigEndian, uint32(0)) // sample_size = 0 (per-sample)
	_ = binary.Write(buf, binary.BigEndian, uint32(1)) // sample_count = 1
	_ = binary.Write(buf, binary.BigEndian, uint32(len(sensorBytes)))

	// stco box: 1 chunk offset pointing to mdat data.
	_ = binary.Write(buf, binary.BigEndian, stcoSize)
	buf.WriteString("stco")
	_ = binary.Write(buf, binary.BigEndian, uint32(0))      // version + flags
	_ = binary.Write(buf, binary.BigEndian, uint32(1))      // entry_count = 1
	_ = binary.Write(buf, binary.BigEndian, mdatDataOffset) // chunk offset

	// mdat box.
	_ = binary.Write(buf, binary.BigEndian, mdatSize)
	buf.WriteString("mdat")
	buf.Write(sensorBytes)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
