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
	b := &Base{}
	got := b.MetadataSupport()
	if got != domain.MetadataSidecar {
		t.Errorf("MetadataSupport() = %v, want MetadataSidecar", got)
	}
}

// TestBase_WriteMetadataTags_noop verifies that WriteMetadataTags is a no-op
// retained for interface compliance. The pipeline no longer calls this directly.
func TestBase_WriteMetadataTags_noop(t *testing.T) {
	b := &Base{}
	tags := domain.MetadataTags{Copyright: "test", CameraOwner: "test"}
	err := b.WriteMetadataTags("dummy.dng", tags)
	if err != nil {
		t.Fatalf("WriteMetadataTags should be no-op, got error: %v", err)
	}
}

// TestBase_HashableReader_fullFileFallback verifies that a file with no
// embedded JPEG preview returns the full file content.
func TestBase_HashableReader_fullFileFallback(t *testing.T) {
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
	dir := t.TempDir()
	filePath := buildMinimalTIFF(t, dir, "test.dng")

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

// TestBase_HashableReader_withJPEGPreview verifies that a file with an
// embedded JPEG preview in IFD1 returns the JPEG bytes.
func TestBase_HashableReader_withJPEGPreview(t *testing.T) {
	dir := t.TempDir()
	filePath := buildTIFFWithJPEGPreview(t, dir, "test.dng")

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
		t.Fatal("HashableReader returned empty data")
	}

	// Verify it starts with JPEG SOI marker.
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Fatal("HashableReader did not return JPEG data (missing SOI marker)")
	}

	// Verify it ends with JPEG EOI marker.
	if len(data) < 2 || data[len(data)-2] != 0xFF || data[len(data)-1] != 0xD9 {
		t.Fatal("HashableReader did not return complete JPEG (missing EOI marker)")
	}
}

// buildMinimalTIFF writes a minimal valid TIFF LE file to disk.
// Structure:
//
//	Bytes 0-1: 0x49 0x49 (LE)
//	Bytes 2-3: 0x2A 0x00 (magic 42 LE)
//	Bytes 4-7: 0x08 0x00 0x00 0x00 (IFD0 at offset 8)
//	Bytes 8-9: 0x00 0x00 (0 entries in IFD0)
//	Bytes 10-13: 0x00 0x00 0x00 0x00 (next IFD = 0, end of chain)
func buildMinimalTIFF(t *testing.T, dir, name string) string {
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

// buildTIFFWithJPEGPreview writes a TIFF LE file with IFD0 pointing to IFD1,
// and IFD1 containing JPEGInterchangeFormat (tag 0x0201) and
// JPEGInterchangeFormatLength (tag 0x0202) pointing to a small JPEG blob.
func buildTIFFWithJPEGPreview(t *testing.T, dir, name string) string {
	// Minimal JPEG blob: SOI + APP0 + EOI
	jpegData := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}

	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// IFD0 offset (8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 1 entry (pointing to IFD1)
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))

	// IFD0 entry: SubIFDs tag (0x014A)
	// tag(2) + type(2) + count(4) + value/offset(4)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0x014A)) // SubIFDs tag
	_ = binary.Write(buf, binary.LittleEndian, uint16(4))      // type LONG
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))      // count
	_ = binary.Write(buf, binary.LittleEndian, uint32(50))     // offset to IFD1

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	// Padding to reach offset 50 for IFD1
	for buf.Len() < 50 {
		buf.WriteByte(0x00)
	}

	// IFD1: 2 entries (JPEG offset and length)
	_ = binary.Write(buf, binary.LittleEndian, uint16(2))

	// Entry 1: JPEGInterchangeFormat (0x0201)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0x0201)) // tag
	_ = binary.Write(buf, binary.LittleEndian, uint16(4))      // type LONG
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))      // count
	jpegOffset := uint32(buf.Len() + 12 + 4 + 4)               // after this entry and next IFD offset
	_ = binary.Write(buf, binary.LittleEndian, jpegOffset)

	// Entry 2: JPEGInterchangeFormatLength (0x0202)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0x0202)) // tag
	_ = binary.Write(buf, binary.LittleEndian, uint16(4))      // type LONG
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))      // count
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(jpegData)))

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	// Append JPEG data
	buf.Write(jpegData)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
