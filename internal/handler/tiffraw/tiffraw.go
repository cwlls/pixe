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

// Package tiffraw provides shared logic for TIFF-based RAW image formats.
//
// Five RAW formats — DNG, NEF (Nikon), CR2 (Canon), PEF (Pentax), and
// ARW (Sony) — are all TIFF containers with standard EXIF IFDs. They share
// identical logic for date extraction, hashable region identification, and
// metadata write behavior. This package provides a Base struct that
// implements the three format-agnostic methods of the FileTypeHandler
// interface. Per-format handlers embed Base and supply only Extensions(),
// MagicBytes(), and Detect().
//
// Date extraction:
//
//	Parses the TIFF IFD chain to locate EXIF sub-IFDs, then reads
//	DateTimeOriginal (tag 0x9003) and DateTime (tag 0x0132) with the
//	standard fallback chain: DateTimeOriginal → DateTime → Ansel Adams date.
//
// Hashable region:
//
//	Extracts the embedded full-resolution JPEG preview image. TIFF-based
//	RAW files store this in a secondary IFD (often IFD1 or a sub-IFD) with
//	NewSubfileType = 0 (full-resolution) and Compression = 6 (JPEG).
//	The handler navigates the IFD chain, locates the JPEG strip/tile
//	offsets and byte counts, and returns a reader over that region.
//	Falls back to full-file hash if the JPEG preview cannot be extracted.
//
// Metadata write:
//
//	No-op stub. RAW files are archival originals — writing metadata into
//	proprietary containers risks corruption.
package tiffraw

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	rwexif "github.com/rwcarlsen/goexif/exif"

	"github.com/cwlls/pixe-go/internal/domain"
)

// anselsAdams is the fallback date when no EXIF date can be extracted.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

const exifDateFormat = "2006:01:02 15:04:05"

// TIFF IFD tag IDs used for JPEG preview extraction.
const (
	tagNewSubfileType  = 0x00FE
	tagCompression     = 0x0103
	tagStripOffsets    = 0x0111
	tagStripByteCounts = 0x0117
	tagSubIFDs         = 0x014A
	tagJPEGOffset      = 0x0201 // JPEGInterchangeFormat (IFD1 thumbnail)
	tagJPEGLength      = 0x0202 // JPEGInterchangeFormatLength
)

// Base provides shared logic for TIFF-based RAW formats.
// Per-format handlers embed this struct and supply their own
// Extensions(), MagicBytes(), and Detect() methods.
type Base struct{}

// ExtractDate reads the capture date from EXIF metadata embedded in the
// TIFF container. Fallback chain: DateTimeOriginal → DateTime → Ansel Adams.
func (b *Base) ExtractDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return anselsAdams, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	x, err := rwexif.Decode(f)
	if err != nil {
		// No EXIF data — use fallback.
		return anselsAdams, nil
	}

	// 1. DateTimeOriginal — parse as UTC to avoid timezone issues.
	if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	// 2. DateTime (IFD0) — parse as UTC.
	if tag, err := x.Get(rwexif.DateTime); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	// 3. Fallback.
	return anselsAdams, nil
}

// HashableReader returns an io.ReadCloser over the embedded full-resolution
// JPEG preview image. If the preview cannot be extracted, falls back to
// returning a reader over the entire file.
func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
	}

	preview, err := findLargestJPEGPreview(f)
	if err != nil || preview == nil {
		// Fallback: hash the full file.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("tiffraw: seek %q: %w", filePath, err)
		}
		return f, nil
	}

	// Return a reader scoped to the JPEG preview region.
	sr := io.NewSectionReader(f, preview.offset, preview.size)
	return &sectionReadCloser{Reader: sr, Closer: f}, nil
}

// WriteMetadataTags is a no-op for TIFF-based RAW formats.
// RAW metadata write not supported in pure Go — writing into proprietary
// containers risks corruption of archival originals.
func (b *Base) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// jpegPreview holds the offset and size of an embedded JPEG preview.
type jpegPreview struct {
	offset int64
	size   int64
}

// sectionReadCloser wraps an io.Reader with a separate io.Closer,
// allowing the caller to close the underlying file when done reading
// a section of it.
type sectionReadCloser struct {
	Reader io.Reader
	Closer io.Closer
}

func (s *sectionReadCloser) Read(p []byte) (int, error) { return s.Reader.Read(p) }
func (s *sectionReadCloser) Close() error               { return s.Closer.Close() }

// findLargestJPEGPreview parses the TIFF IFD chain and returns the
// offset and size of the largest embedded JPEG preview image.
// Returns nil if no JPEG preview is found.
func findLargestJPEGPreview(r io.ReadSeeker) (*jpegPreview, error) {
	// Read TIFF header (8 bytes).
	header := make([]byte, 8)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("tiffraw: read header: %w", err)
	}

	// Determine byte order.
	var order binary.ByteOrder
	switch {
	case header[0] == 0x49 && header[1] == 0x49:
		order = binary.LittleEndian
	case header[0] == 0x4D && header[1] == 0x4D:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("tiffraw: invalid byte order marker")
	}

	// Verify TIFF magic (42).
	magic := order.Uint16(header[2:4])
	if magic != 42 {
		return nil, fmt.Errorf("tiffraw: invalid TIFF magic: %d", magic)
	}

	// Offset to IFD0.
	ifd0Offset := int64(order.Uint32(header[4:8]))

	var largest *jpegPreview

	// Walk the IFD chain: IFD0 → IFD1 → ...
	offset := ifd0Offset
	for offset != 0 {
		preview, subIFDOffsets, nextOffset, err := parseIFD(r, offset, order)
		if err != nil {
			break
		}

		if preview != nil && (largest == nil || preview.size > largest.size) {
			largest = preview
		}

		// Follow SubIFD pointers (tag 0x014A).
		for _, subOffset := range subIFDOffsets {
			subPreview, _, _, err := parseIFD(r, subOffset, order)
			if err != nil {
				continue
			}
			if subPreview != nil && (largest == nil || subPreview.size > largest.size) {
				largest = subPreview
			}
		}

		offset = nextOffset
	}

	return largest, nil
}

// ifdEntry holds the parsed values of a single TIFF IFD entry that we care about.
type ifdValues struct {
	newSubfileType  uint32
	compression     uint32
	stripOffsets    []uint32
	stripByteCounts []uint32
	subIFDOffsets   []int64
	jpegOffset      uint32
	jpegLength      uint32
}

// parseIFD reads a TIFF IFD at the given offset and extracts JPEG preview info.
// Returns the JPEG preview (if any), sub-IFD offsets, the next IFD offset, and any error.
func parseIFD(r io.ReadSeeker, offset int64, order binary.ByteOrder) (*jpegPreview, []int64, int64, error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return nil, nil, 0, fmt.Errorf("tiffraw: seek to IFD at %d: %w", offset, err)
	}

	// Read entry count (2 bytes).
	var count uint16
	if err := binary.Read(r, order, &count); err != nil {
		return nil, nil, 0, fmt.Errorf("tiffraw: read IFD entry count: %w", err)
	}

	vals := &ifdValues{}

	// Each IFD entry is 12 bytes: tag(2) + type(2) + count(4) + value/offset(4).
	entries := make([]byte, int(count)*12)
	if _, err := io.ReadFull(r, entries); err != nil {
		return nil, nil, 0, fmt.Errorf("tiffraw: read IFD entries: %w", err)
	}

	for i := 0; i < int(count); i++ {
		e := entries[i*12 : i*12+12]
		tag := order.Uint16(e[0:2])
		typ := order.Uint16(e[2:4])
		cnt := order.Uint32(e[4:8])
		valOff := e[8:12]

		switch tag {
		case tagNewSubfileType:
			vals.newSubfileType = order.Uint32(valOff)

		case tagCompression:
			vals.compression = uint32(order.Uint16(valOff))

		case tagStripOffsets:
			vals.stripOffsets = readUint32Array(r, order, typ, cnt, valOff)

		case tagStripByteCounts:
			vals.stripByteCounts = readUint32Array(r, order, typ, cnt, valOff)

		case tagSubIFDs:
			// SubIFDs tag — value is one or more IFD offsets.
			offsets := readUint32Array(r, order, typ, cnt, valOff)
			for _, o := range offsets {
				vals.subIFDOffsets = append(vals.subIFDOffsets, int64(o))
			}

		case tagJPEGOffset:
			vals.jpegOffset = order.Uint32(valOff)

		case tagJPEGLength:
			vals.jpegLength = order.Uint32(valOff)
		}
	}

	// Read next IFD offset (4 bytes after entries).
	var nextOffsetRaw uint32
	if err := binary.Read(r, order, &nextOffsetRaw); err != nil {
		nextOffsetRaw = 0
	}
	nextOffset := int64(nextOffsetRaw)

	// Determine if this IFD contains a JPEG preview.
	var preview *jpegPreview

	// Path 1: JPEGInterchangeFormat / JPEGInterchangeFormatLength (IFD1 thumbnail path).
	if vals.jpegOffset > 0 && vals.jpegLength > 0 {
		preview = &jpegPreview{
			offset: int64(vals.jpegOffset),
			size:   int64(vals.jpegLength),
		}
	}

	// Path 2: Strip-based JPEG (NewSubfileType=0, Compression=6).
	// This is the full-resolution preview path used by some RAW formats.
	if vals.compression == 6 && len(vals.stripOffsets) > 0 && len(vals.stripByteCounts) > 0 {
		// Calculate total size across all strips.
		var totalSize int64
		for _, bc := range vals.stripByteCounts {
			totalSize += int64(bc)
		}
		if totalSize > 0 {
			// Use the first strip offset as the start.
			stripPreview := &jpegPreview{
				offset: int64(vals.stripOffsets[0]),
				size:   totalSize,
			}
			// Prefer the larger of the two paths.
			if preview == nil || stripPreview.size > preview.size {
				preview = stripPreview
			}
		}
	}

	return preview, vals.subIFDOffsets, nextOffset, nil
}

// readUint32Array reads an array of uint32 values from a TIFF IFD entry.
// For SHORT (type 3) values, each is 2 bytes. For LONG (type 4), each is 4 bytes.
// If the total size fits in 4 bytes, the values are stored inline in valOff.
// Otherwise, valOff is an offset to the actual data.
func readUint32Array(r io.ReadSeeker, order binary.ByteOrder, typ uint16, cnt uint32, valOff []byte) []uint32 {
	const (
		typeShort = 3
		typeLong  = 4
	)

	var bytesPerVal uint32
	switch typ {
	case typeShort:
		bytesPerVal = 2
	case typeLong:
		bytesPerVal = 4
	default:
		// Unsupported type for this tag — return single value interpretation.
		return []uint32{order.Uint32(valOff)}
	}

	totalBytes := bytesPerVal * cnt
	var data []byte

	if totalBytes <= 4 {
		// Values fit inline.
		data = valOff[:totalBytes]
	} else {
		// Values are at the offset pointed to by valOff.
		off := int64(order.Uint32(valOff))
		saved, err := r.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil
		}
		if _, err := r.Seek(off, io.SeekStart); err != nil {
			return nil
		}
		data = make([]byte, totalBytes)
		if _, err := io.ReadFull(r, data); err != nil {
			_, _ = r.Seek(saved, io.SeekStart)
			return nil
		}
		_, _ = r.Seek(saved, io.SeekStart)
	}

	result := make([]uint32, cnt)
	for i := uint32(0); i < cnt; i++ {
		switch typ {
		case typeShort:
			result[i] = uint32(order.Uint16(data[i*2 : i*2+2]))
		case typeLong:
			result[i] = order.Uint32(data[i*4 : i*4+4])
		}
	}
	return result
}

// fileExt returns the file extension including the leading dot, or "".
func fileExt(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}
