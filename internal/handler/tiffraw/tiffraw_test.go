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

package tiffraw

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain" // for MetadataTags
)

// TestBase_MetadataSupport verifies that tiffraw.Base declares MetadataSidecar.
func TestBase_MetadataSupport(t *testing.T) {
	t.Parallel()
	b := &Base{}
	got := b.MetadataSupport()
	if got != domain.MetadataSidecar {
		t.Errorf("MetadataSupport() = %v, want MetadataSidecar", got)
	}
}

// TestBase_WriteMetadataTags_noop verifies that WriteMetadataTags is a no-op
// retained for interface compliance. The pipeline no longer calls this directly.
func TestBase_WriteMetadataTags_noop(t *testing.T) {
	t.Parallel()
	b := &Base{}
	tags := domain.MetadataTags{Copyright: "test", CameraOwner: "test"}
	err := b.WriteMetadataTags("dummy.dng", tags)
	if err != nil {
		t.Fatalf("WriteMetadataTags should be no-op, got error: %v", err)
	}
}

// TestBase_HashableReader_fullFileFallback verifies that a file with no
// sensor data IFD returns the full file content.
func TestBase_HashableReader_fullFileFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildMinimalTIFF(t, dir, "test.dng")

	b := &Base{}
	rc, err := b.HashableReader(filePath)
	if err != nil {
		t.Fatalf("HashableReader failed: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("HashableReader returned empty data for fallback")
	}

	// Verify it's the full file content.
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !bytes.Equal(data, fileData) {
		t.Fatal("HashableReader did not return full file content")
	}
}

// TestBase_HashableReader_deterministic verifies that two calls to
// HashableReader return identical bytes.
func TestBase_HashableReader_deterministic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildTIFFWithSensorData(t, dir, "test.dng")

	b := &Base{}

	rc1, err := b.HashableReader(filePath)
	if err != nil {
		t.Fatalf("First HashableReader failed: %v", err)
	}
	data1, err := io.ReadAll(rc1)
	if err != nil {
		t.Fatalf("First ReadAll failed: %v", err)
	}
	_ = rc1.Close()

	rc2, err := b.HashableReader(filePath)
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

// TestBase_ExtractDate_noEXIF_fallback verifies that a file with no EXIF
// returns the Ansel Adams fallback date.
func TestBase_ExtractDate_noEXIF_fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildMinimalTIFF(t, dir, "test.dng")

	b := &Base{}
	date, err := b.ExtractDate(filePath)
	if err != nil {
		t.Fatalf("ExtractDate failed: %v", err)
	}

	expected := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
	if !date.Equal(expected) {
		t.Fatalf("Expected Ansel Adams date %v, got %v", expected, date)
	}
}

// TestBase_HashableReader_withSensorData verifies that a file with a sensor
// data IFD (Compression=7, non-JPEG) returns the sensor data bytes.
func TestBase_HashableReader_withSensorData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sensorBytes := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
	filePath := buildTIFFWithSensorDataBytes(t, dir, "test.dng", sensorBytes)

	b := &Base{}
	rc, err := b.HashableReader(filePath)
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

// TestBase_HashableReader_prefersNonJPEGCompression verifies that when a TIFF
// contains both a JPEG preview IFD (Compression=6) and a sensor data IFD
// (Compression=7), HashableReader returns the sensor data, not the JPEG preview.
func TestBase_HashableReader_prefersNonJPEGCompression(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sensorBytes := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
	filePath := buildTIFFWithSensorAndJPEGPreview(t, dir, "test.dng", sensorBytes)

	b := &Base{}
	rc, err := b.HashableReader(filePath)
	if err != nil {
		t.Fatalf("HashableReader failed: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	// Must be the sensor data, not the JPEG preview.
	if !bytes.Equal(data, sensorBytes) {
		t.Fatalf("HashableReader returned %x, want sensor data %x (not JPEG preview)", data, sensorBytes)
	}

	// Must NOT start with JPEG SOI marker.
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		t.Fatal("HashableReader returned JPEG preview data instead of sensor data")
	}
}

// TestBase_HashableReader_multipleStrips verifies that a sensor data IFD with
// multiple strips is read as a single concatenated byte sequence.
func TestBase_HashableReader_multipleStrips(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	strip1 := []byte{0x01, 0x02, 0x03, 0x04}
	strip2 := []byte{0x05, 0x06, 0x07, 0x08}
	filePath := buildTIFFWithMultipleStrips(t, dir, "test.dng", strip1, strip2)

	b := &Base{}
	rc, err := b.HashableReader(filePath)
	if err != nil {
		t.Fatalf("HashableReader failed: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := append(strip1, strip2...)
	if !bytes.Equal(data, expected) {
		t.Fatalf("HashableReader returned %x, want concatenated strips %x", data, expected)
	}
}

// TestBase_HashableReader_tiledSensorData verifies that a sensor data IFD
// using TileOffsets/TileByteCounts is read correctly.
func TestBase_HashableReader_tiledSensorData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tile1 := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	tile2 := []byte{0xEE, 0xFF, 0x11, 0x22}
	filePath := buildTIFFWithTiles(t, dir, "test.dng", tile1, tile2)

	b := &Base{}
	rc, err := b.HashableReader(filePath)
	if err != nil {
		t.Fatalf("HashableReader failed: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := append(tile1, tile2...)
	if !bytes.Equal(data, expected) {
		t.Fatalf("HashableReader returned %x, want concatenated tiles %x", data, expected)
	}
}

// TestFindSensorData_noSensorIFD verifies that findSensorData returns nil
// for a TIFF that contains only a JPEG preview IFD (Compression=6).
func TestFindSensorData_noSensorIFD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildTIFFWithOnlyJPEGPreview(t, dir, "test.dng")

	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = f.Close() }()

	sensor, err := findSensorData(f)
	if err != nil {
		t.Fatalf("findSensorData returned error: %v", err)
	}
	if sensor != nil {
		t.Fatalf("findSensorData should return nil for JPEG-only TIFF, got %+v", sensor)
	}
}

// ---------------------------------------------------------------------------
// Test fixture builders
// ---------------------------------------------------------------------------

// buildMinimalTIFF writes a minimal valid TIFF LE file to disk.
// Structure:
//
//	Bytes 0-1: 0x49 0x49 (LE)
//	Bytes 2-3: 0x2A 0x00 (magic 42 LE)
//	Bytes 4-7: 0x08 0x00 0x00 0x00 (IFD0 at offset 8)
//	Bytes 8-9: 0x00 0x00 (0 entries in IFD0)
//	Bytes 10-13: 0x00 0x00 0x00 0x00 (next IFD = 0, end of chain)
func buildMinimalTIFF(t *testing.T, dir, name string) string {
	t.Helper()
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

// buildTIFFWithSensorData writes a TIFF with a sensor data IFD (Compression=7,
// lossless JPEG) containing known bytes. Used for determinism tests.
func buildTIFFWithSensorData(t *testing.T, dir, name string) string {
	t.Helper()
	return buildTIFFWithSensorDataBytes(t, dir, name, []byte{0xDE, 0xAD, 0xBE, 0xEF})
}

// buildTIFFWithSensorDataBytes writes a TIFF LE file with IFD0 containing a
// sensor data entry: Compression=7 (lossless JPEG), StripOffsets and
// StripByteCounts pointing to the provided sensorBytes.
//
// Layout:
//
//	[0]   TIFF header (8 bytes)
//	[8]   IFD0: 3 entries (Compression, StripOffsets, StripByteCounts)
//	      entry count (2) + 3×12 bytes + next IFD offset (4) = 42 bytes
//	[50]  sensor data bytes
func buildTIFFWithSensorDataBytes(t *testing.T, dir, name string, sensorBytes []byte) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// TIFF header (LE, magic 42, IFD0 at offset 8).
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 3 entries.
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))

	// Sensor data will be placed at offset 8 + 2 + 3*12 + 4 = 50.
	sensorOffset := uint32(50)

	// Entry 1: Compression = 7 (lossless JPEG — sensor data, not preview).
	writeIFDEntry(buf, binary.LittleEndian, tagCompression, 3 /*SHORT*/, 1, uint32(7))

	// Entry 2: StripOffsets — inline single value.
	writeIFDEntry(buf, binary.LittleEndian, tagStripOffsets, 4 /*LONG*/, 1, sensorOffset)

	// Entry 3: StripByteCounts — inline single value.
	writeIFDEntry(buf, binary.LittleEndian, tagStripByteCounts, 4 /*LONG*/, 1, uint32(len(sensorBytes)))

	// Next IFD offset (0 = end).
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	// Pad to sensorOffset.
	for buf.Len() < int(sensorOffset) {
		buf.WriteByte(0x00)
	}

	// Sensor data.
	buf.Write(sensorBytes)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildTIFFWithSensorAndJPEGPreview writes a TIFF with:
//   - IFD0: sensor data IFD (Compression=7, StripOffsets/StripByteCounts)
//   - IFD1: JPEG preview IFD (Compression=6, JPEGInterchangeFormat/Length)
//
// The sensor data bytes are provided; the JPEG preview is a minimal JPEG blob.
func buildTIFFWithSensorAndJPEGPreview(t *testing.T, dir, name string, sensorBytes []byte) string {
	t.Helper()

	// Minimal JPEG blob: SOI + APP0 + EOI
	jpegData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}

	buf := new(bytes.Buffer)

	// TIFF header: IFD0 at offset 8.
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 3 entries (Compression=7, StripOffsets, StripByteCounts).
	// IFD0 occupies: 2 + 3*12 + 4 = 42 bytes → ends at offset 50.
	// IFD1 starts at offset 50.
	// IFD1: 3 entries (Compression=6, JPEGOffset, JPEGLength).
	// IFD1 occupies: 2 + 3*12 + 4 = 42 bytes → ends at offset 92.
	// Sensor data at offset 92.
	// JPEG data after sensor data.

	ifd1Offset := uint32(50)
	sensorOffset := uint32(92)
	jpegOffset := sensorOffset + uint32(len(sensorBytes))

	// IFD0: 3 entries.
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))
	writeIFDEntry(buf, binary.LittleEndian, tagCompression, 3, 1, uint32(7))
	writeIFDEntry(buf, binary.LittleEndian, tagStripOffsets, 4, 1, sensorOffset)
	writeIFDEntry(buf, binary.LittleEndian, tagStripByteCounts, 4, 1, uint32(len(sensorBytes)))
	// Next IFD = IFD1.
	_ = binary.Write(buf, binary.LittleEndian, ifd1Offset)

	// Pad to IFD1.
	for buf.Len() < int(ifd1Offset) {
		buf.WriteByte(0x00)
	}

	// IFD1: 3 entries (JPEG preview).
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))
	writeIFDEntry(buf, binary.LittleEndian, tagCompression, 3, 1, uint32(6)) // standard JPEG
	writeIFDEntry(buf, binary.LittleEndian, tagJPEGOffset, 4, 1, jpegOffset)
	writeIFDEntry(buf, binary.LittleEndian, tagJPEGLength, 4, 1, uint32(len(jpegData)))
	// Next IFD = 0 (end).
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	// Pad to sensorOffset.
	for buf.Len() < int(sensorOffset) {
		buf.WriteByte(0x00)
	}

	// Sensor data.
	buf.Write(sensorBytes)

	// JPEG preview data.
	buf.Write(jpegData)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildTIFFWithMultipleStrips writes a TIFF with a sensor data IFD containing
// two strips at non-contiguous offsets. The strips are provided as separate
// byte slices.
//
// Layout:
//
//	[0]   TIFF header (8 bytes)
//	[8]   IFD0: 3 entries (Compression, StripOffsets×2, StripByteCounts×2)
//	      StripOffsets and StripByteCounts are arrays → stored at external offsets.
//	      entry count (2) + 3*12 + 4 = 42 bytes → IFD0 ends at offset 50.
//	[50]  StripOffsets array (2 × uint32 = 8 bytes) → at offset 50
//	[58]  StripByteCounts array (2 × uint32 = 8 bytes) → at offset 58
//	[66]  strip1 bytes
//	[66+len(strip1)] strip2 bytes
func buildTIFFWithMultipleStrips(t *testing.T, dir, name string, strip1, strip2 []byte) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// TIFF header.
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// Offsets for external arrays.
	stripOffsetsArrayAt := uint32(50)
	stripByteCountsArrayAt := uint32(58)
	strip1At := uint32(66)
	strip2At := strip1At + uint32(len(strip1))

	// IFD0: 3 entries.
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))

	// Compression = 7 (lossless JPEG / sensor data).
	writeIFDEntry(buf, binary.LittleEndian, tagCompression, 3, 1, uint32(7))

	// StripOffsets: count=2, stored at external offset.
	writeIFDEntryExternal(buf, binary.LittleEndian, tagStripOffsets, 4, 2, stripOffsetsArrayAt)

	// StripByteCounts: count=2, stored at external offset.
	writeIFDEntryExternal(buf, binary.LittleEndian, tagStripByteCounts, 4, 2, stripByteCountsArrayAt)

	// Next IFD = 0.
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	// Pad to stripOffsetsArrayAt.
	for buf.Len() < int(stripOffsetsArrayAt) {
		buf.WriteByte(0x00)
	}

	// StripOffsets array: [strip1At, strip2At].
	_ = binary.Write(buf, binary.LittleEndian, strip1At)
	_ = binary.Write(buf, binary.LittleEndian, strip2At)

	// StripByteCounts array: [len(strip1), len(strip2)].
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(strip1)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(strip2)))

	// Pad to strip1At.
	for buf.Len() < int(strip1At) {
		buf.WriteByte(0x00)
	}

	// Strip data.
	buf.Write(strip1)
	buf.Write(strip2)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildTIFFWithTiles writes a TIFF with a sensor data IFD using
// TileOffsets/TileByteCounts (tiled layout) instead of strips.
//
// Layout:
//
//	[0]   TIFF header (8 bytes)
//	[8]   IFD0: 3 entries (Compression, TileOffsets×2, TileByteCounts×2)
//	      entry count (2) + 3*12 + 4 = 42 bytes → IFD0 ends at offset 50.
//	[50]  TileOffsets array (2 × uint32 = 8 bytes)
//	[58]  TileByteCounts array (2 × uint32 = 8 bytes)
//	[66]  tile1 bytes
//	[66+len(tile1)] tile2 bytes
func buildTIFFWithTiles(t *testing.T, dir, name string, tile1, tile2 []byte) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// TIFF header.
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	tileOffsetsArrayAt := uint32(50)
	tileByteCountsArrayAt := uint32(58)
	tile1At := uint32(66)
	tile2At := tile1At + uint32(len(tile1))

	// IFD0: 3 entries.
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))

	// Compression = 7.
	writeIFDEntry(buf, binary.LittleEndian, tagCompression, 3, 1, uint32(7))

	// TileOffsets: count=2, external.
	writeIFDEntryExternal(buf, binary.LittleEndian, tagTileOffsets, 4, 2, tileOffsetsArrayAt)

	// TileByteCounts: count=2, external.
	writeIFDEntryExternal(buf, binary.LittleEndian, tagTileByteCounts, 4, 2, tileByteCountsArrayAt)

	// Next IFD = 0.
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	// Pad to tileOffsetsArrayAt.
	for buf.Len() < int(tileOffsetsArrayAt) {
		buf.WriteByte(0x00)
	}

	// TileOffsets array.
	_ = binary.Write(buf, binary.LittleEndian, tile1At)
	_ = binary.Write(buf, binary.LittleEndian, tile2At)

	// TileByteCounts array.
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(tile1)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(tile2)))

	// Pad to tile1At.
	for buf.Len() < int(tile1At) {
		buf.WriteByte(0x00)
	}

	// Tile data.
	buf.Write(tile1)
	buf.Write(tile2)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildTIFFWithOnlyJPEGPreview writes a TIFF with only a JPEG preview IFD
// (Compression=6, JPEGInterchangeFormat/Length). No sensor data IFD.
// Used to verify that findSensorData returns nil for JPEG-only TIFFs.
func buildTIFFWithOnlyJPEGPreview(t *testing.T, dir, name string) string {
	t.Helper()

	jpegData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}

	buf := new(bytes.Buffer)

	// TIFF header.
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 3 entries (JPEG preview only).
	// IFD0 ends at offset 50. JPEG data at offset 50.
	jpegOffset := uint32(50)

	_ = binary.Write(buf, binary.LittleEndian, uint16(3))
	writeIFDEntry(buf, binary.LittleEndian, tagCompression, 3, 1, uint32(6)) // standard JPEG
	writeIFDEntry(buf, binary.LittleEndian, tagJPEGOffset, 4, 1, jpegOffset)
	writeIFDEntry(buf, binary.LittleEndian, tagJPEGLength, 4, 1, uint32(len(jpegData)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD = 0

	// Pad to jpegOffset.
	for buf.Len() < int(jpegOffset) {
		buf.WriteByte(0x00)
	}

	buf.Write(jpegData)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ---------------------------------------------------------------------------
// IFD entry write helpers
// ---------------------------------------------------------------------------

// writeIFDEntry writes a 12-byte TIFF IFD entry with an inline value.
// tag(2) + type(2) + count(4) + value(4)
// For SHORT type, the value is written as uint16 in the first 2 bytes of the
// value field (little-endian), with the remaining 2 bytes zeroed.
func writeIFDEntry(buf *bytes.Buffer, order binary.ByteOrder, tag uint16, typ uint16, count uint32, value uint32) {
	_ = binary.Write(buf, order, tag)
	_ = binary.Write(buf, order, typ)
	_ = binary.Write(buf, order, count)
	if typ == 3 { // SHORT
		_ = binary.Write(buf, order, uint16(value))
		_ = binary.Write(buf, order, uint16(0))
	} else {
		_ = binary.Write(buf, order, value)
	}
}

// writeIFDEntryExternal writes a 12-byte TIFF IFD entry where the value
// field is an offset to external data (used when count*typeSize > 4).
func writeIFDEntryExternal(buf *bytes.Buffer, order binary.ByteOrder, tag uint16, typ uint16, count uint32, offset uint32) {
	_ = binary.Write(buf, order, tag)
	_ = binary.Write(buf, order, typ)
	_ = binary.Write(buf, order, count)
	_ = binary.Write(buf, order, offset)
}
