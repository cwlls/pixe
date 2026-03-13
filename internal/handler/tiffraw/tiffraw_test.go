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

	"github.com/cwlls/pixe/internal/domain" // for MetadataTags
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

// TestBase_HashableReader_returnsFullFile verifies that HashableReader returns
// the complete file contents (full-file hash).
func TestBase_HashableReader_returnsFullFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := buildMinimalTIFF(t, dir, "test.dng")

	b := &Base{}
	rc, err := b.HashableReader(filePath)
	if err != nil {
		t.Fatalf("HashableReader failed: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	want, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("HashableReader returned %d bytes, want full file %d bytes", len(got), len(want))
	}
}

// TestBase_HashableReader_deterministic verifies that two calls to
// HashableReader return identical bytes.
func TestBase_HashableReader_deterministic(t *testing.T) {
	t.Parallel()
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
