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

// Package cr3 implements the FileTypeHandler contract for Canon CR3 RAW images.
//
// CR3 files use the ISOBMFF container format (like HEIC and MP4), not TIFF.
// This handler is standalone and does not use the tiffraw shared base.
//
// Date extraction:
//
//	CR3 stores EXIF metadata within the ISOBMFF box structure. The handler
//	parses the container to locate the EXIF blob (typically in a moov/uuid
//	box using Canon's UUID), then uses rwcarlsen/goexif for standard EXIF
//	tag reading. Fallback chain: DateTimeOriginal → DateTime → Ansel Adams date.
//
// Hashable region:
//
//	The complete file contents. Destination files are byte-identical copies
//	of their source; metadata is expressed via XMP sidecar only.
//
// Magic bytes:
//
//	"ftyp" at offset 4 (same as HEIC/MP4). The ftyp brand "crx " (Canon
//	RAW X) distinguishes CR3 from other ISOBMFF formats. Detection checks
//	both the ftyp box type and the "crx " brand.
//
// Metadata write:
//
//	No-op stub. CR3 metadata write not supported in pure Go.
package cr3

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	rwexif "github.com/rwcarlsen/goexif/exif"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/fileutil"
)

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

// anselsAdams is the fallback date when no EXIF date can be extracted.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

// exifDateFormat is the EXIF date/time string format.
const exifDateFormat = "2006:01:02 15:04:05"

// Handler implements domain.FileTypeHandler for Canon CR3 RAW images.
type Handler struct{}

// New returns a new CR3 Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".cr3"}
}

// MagicBytes returns the ISOBMFF ftyp box signature at offset 4.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
	}
}

// Detect returns true if the file has a .cr3 extension AND contains the
// ISOBMFF "ftyp" box with "crx " brand.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".cr3" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("cr3: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	// Check "ftyp" at offset 4.
	ftyp := header[4] == 0x66 && header[5] == 0x74 &&
		header[6] == 0x79 && header[7] == 0x70
	if !ftyp {
		return false, nil
	}
	// Check major brand "crx " at offset 8.
	brand := string(header[8:12])
	return brand == "crx ", nil
}

// ExtractDate reads the capture date from CR3 EXIF metadata.
// Fallback chain: DateTimeOriginal → DateTime → anselsAdams.
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return anselsAdams, fmt.Errorf("cr3: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	exifBytes, err := extractCR3Exif(f)
	if err != nil || len(exifBytes) == 0 {
		return anselsAdams, nil
	}

	x, err := rwexif.Decode(bytes.NewReader(exifBytes))
	if err != nil {
		return anselsAdams, nil
	}

	// 1. DateTimeOriginal.
	if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	// 2. DateTime (IFD0).
	if tag, err := x.Get(rwexif.DateTime); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	return anselsAdams, nil
}

// HashableReader returns an io.ReadCloser over the complete file contents.
// Destination files are byte-identical copies of their source; the full-file
// hash ensures re-verification is always a simple open-and-hash operation.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cr3: open %q: %w", filePath, err)
	}
	return f, nil
}

// MetadataSupport declares that CR3 uses XMP sidecar files.
// CR3 metadata write is not supported in pure Go.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// isobmffBox represents a single ISOBMFF box header.
type isobmffBox struct {
	boxType string // 4-char type code
	size    int64  // total box size including header
	offset  int64  // file offset where the box starts
	dataOff int64  // file offset where the box data starts (after header)
}

// readBox reads the next ISOBMFF box header from the current position.
func readBox(r io.ReadSeeker) (*isobmffBox, error) {
	offset, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}

	size := int64(binary.BigEndian.Uint32(hdr[0:4]))
	boxType := string(hdr[4:8])

	dataOff := offset + 8

	// Extended size: size == 1 means the next 8 bytes hold the actual size.
	if size == 1 {
		var extSize [8]byte
		if _, err := io.ReadFull(r, extSize[:]); err != nil {
			return nil, err
		}
		size = int64(binary.BigEndian.Uint64(extSize[:]))
		dataOff = offset + 16
	}

	// size == 0 means "to end of file" — we'll handle this as a large number.
	if size == 0 {
		size = 1<<62 - 1
	}

	return &isobmffBox{
		boxType: boxType,
		size:    size,
		offset:  offset,
		dataOff: dataOff,
	}, nil
}

// walkBoxes iterates over top-level boxes in the given byte range [start, end).
func walkBoxes(r io.ReadSeeker, start, end int64) ([]*isobmffBox, error) {
	if _, err := r.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}

	var boxes []*isobmffBox
	pos := start
	for pos < end {
		box, err := readBox(r)
		if err != nil {
			break
		}
		boxes = append(boxes, box)
		next := box.offset + box.size
		if next <= pos {
			break // prevent infinite loop on malformed data
		}
		pos = next
		if _, err := r.Seek(pos, io.SeekStart); err != nil {
			break
		}
	}
	return boxes, nil
}

// extractCR3Exif parses the ISOBMFF box structure to locate and extract
// the raw EXIF bytes from a CR3 file.
//
// CR3 stores EXIF in a uuid box within moov, using Canon's UUID:
// 85c0b687-820f-11e0-8111-f4ce462b6a48
// We scan for a TIFF header signature within uuid boxes as a fallback.
func extractCR3Exif(r io.ReadSeeker) ([]byte, error) {
	// Get file size.
	fileSize, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	// Walk top-level boxes.
	topBoxes, err := walkBoxes(r, 0, fileSize)
	if err != nil {
		return nil, err
	}

	for _, box := range topBoxes {
		if box.boxType != "moov" {
			continue
		}
		// Walk moov children.
		moovEnd := box.offset + box.size
		moovBoxes, err := walkBoxes(r, box.dataOff, moovEnd)
		if err != nil {
			continue
		}
		for _, child := range moovBoxes {
			if child.boxType == "uuid" {
				// Read the UUID (16 bytes) + remaining data.
				dataSize := child.size - (child.dataOff - child.offset)
				if dataSize < 16 {
					continue
				}
				if _, err := r.Seek(child.dataOff, io.SeekStart); err != nil {
					continue
				}
				uuidAndData := make([]byte, dataSize)
				if _, err := io.ReadFull(r, uuidAndData); err != nil {
					continue
				}
				// Look for a TIFF header within the uuid box data.
				exifBytes := findTIFFHeader(uuidAndData)
				if len(exifBytes) > 0 {
					return exifBytes, nil
				}
			}
		}
	}

	return nil, nil
}

// findTIFFHeader scans data for a TIFF header (II*\0 or MM\0*) and returns
// the bytes from that point onward.
func findTIFFHeader(data []byte) []byte {
	for i := 0; i+4 <= len(data); i++ {
		// Little-endian TIFF: "II" + 0x2A + 0x00
		if data[i] == 0x49 && data[i+1] == 0x49 && data[i+2] == 0x2A && data[i+3] == 0x00 {
			return data[i:]
		}
		// Big-endian TIFF: "MM" + 0x00 + 0x2A
		if data[i] == 0x4D && data[i+1] == 0x4D && data[i+2] == 0x00 && data[i+3] == 0x2A {
			return data[i:]
		}
	}
	return nil
}
