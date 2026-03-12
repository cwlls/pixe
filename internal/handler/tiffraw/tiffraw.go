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
//	Extracts the raw sensor data payload. TIFF-based RAW files store sensor
//	data in a primary IFD with a non-JPEG compression scheme (uncompressed,
//	lossless JPEG compression=7, or vendor-specific). The handler navigates
//	the IFD chain, distinguishes sensor data IFDs from JPEG preview IFDs by
//	compression type and image dimensions, and returns a reader over the
//	sensor data strips/tiles. Falls back to full-file hash if the sensor
//	data cannot be extracted.
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

// exifDateFormat is the EXIF date/time string format.
const exifDateFormat = "2006:01:02 15:04:05"

// TIFF IFD tag IDs used for sensor data and preview extraction.
const (
	tagImageWidth      = 0x0100
	tagImageLength     = 0x0101
	tagNewSubfileType  = 0x00FE
	tagCompression     = 0x0103
	tagStripOffsets    = 0x0111
	tagStripByteCounts = 0x0117
	tagTileOffsets     = 0x0144
	tagTileByteCounts  = 0x0145
	tagSubIFDs         = 0x014A
	tagJPEGOffset      = 0x0201 // JPEGInterchangeFormat (IFD1 thumbnail)
	tagJPEGLength      = 0x0202 // JPEGInterchangeFormatLength
)

// compressionJPEG is the standard JPEG compression value used in preview IFDs.
// Sensor data uses other values (1=uncompressed, 7=lossless JPEG, 34713=NEF, etc.).
const compressionJPEG = 6

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

// HashableReader returns an io.ReadCloser over the raw sensor data payload.
// It navigates the TIFF IFD chain to locate the sensor data IFD (identified
// by non-JPEG compression and largest image dimensions), then returns a
// reader that streams all sensor data strips/tiles in order.
// Falls back to the full file if sensor data cannot be extracted.
func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
	}

	sensor, err := findSensorData(f)
	if err != nil || sensor == nil {
		// Fallback: hash the full file.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("tiffraw: seek %q: %w", filePath, err)
		}
		return f, nil
	}

	// Return a reader that streams all sensor data strips/tiles in order.
	return newMultiSectionReader(f, sensor.offsets, sensor.byteCounts), nil
}

// MetadataSupport declares that TIFF-based RAW formats use XMP sidecar
// files for metadata tagging. Writing into proprietary RAW containers
// risks corruption of archival originals.
func (b *Base) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (b *Base) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// sensorRegion holds the file offsets and byte counts of the raw sensor
// data strips or tiles within a TIFF-based RAW file.
type sensorRegion struct {
	offsets    []int64 // file offsets of each strip/tile
	byteCounts []int64 // byte count of each strip/tile
	totalSize  int64   // sum of all byteCounts
}

// multiSectionReader streams multiple non-contiguous file regions as a
// single contiguous byte sequence. Used to read sensor data strips/tiles
// that may be scattered across the file.
type multiSectionReader struct {
	file  *os.File
	multi io.Reader
}

// newMultiSectionReader returns an io.ReadCloser that reads sequentially
// across multiple byte ranges within a single file, used to stream
// non-contiguous raw sensor data strips for hashing.
func newMultiSectionReader(f *os.File, offsets, byteCounts []int64) *multiSectionReader {
	readers := make([]io.Reader, len(offsets))
	for i := range offsets {
		readers[i] = io.NewSectionReader(f, offsets[i], byteCounts[i])
	}
	return &multiSectionReader{
		file:  f,
		multi: io.MultiReader(readers...),
	}
}

// Read implements io.Reader by reading sequentially across multiple file sections.
func (m *multiSectionReader) Read(p []byte) (int, error) { return m.multi.Read(p) }

// Close implements io.Closer by closing the underlying file.
func (m *multiSectionReader) Close() error { return m.file.Close() }

// ifdValues holds the parsed values of a single TIFF IFD that we care about.
type ifdValues struct {
	newSubfileType  uint32
	compression     uint32
	imageWidth      uint32
	imageLength     uint32
	stripOffsets    []uint32
	stripByteCounts []uint32
	tileOffsets     []uint32
	tileByteCounts  []uint32
	subIFDOffsets   []int64
	jpegOffset      uint32
	jpegLength      uint32
}

// ifdCandidate is a parsed IFD with its computed sensor data payload size.
type ifdCandidate struct {
	vals       *ifdValues
	offsets    []int64
	byteCounts []int64
	totalSize  int64
}

// findSensorData parses the TIFF IFD chain and returns the raw sensor data
// region. It selects the IFD that contains the primary sensor data by:
//  1. Excluding IFDs with standard JPEG compression (compressionJPEG = 6).
//  2. Preferring IFDs with NewSubfileType = 0 (full-resolution primary image).
//  3. Among remaining candidates, selecting the one with the largest total
//     data payload (most bytes = most likely to be sensor data).
//
// Returns nil if no suitable sensor data IFD is found.
func findSensorData(r io.ReadSeeker) (*sensorRegion, error) {
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

	var candidates []*ifdCandidate

	// collectCandidate parses an IFD and, if it has strip/tile data and is
	// not a standard JPEG preview, adds it to the candidates list.
	var collectCandidate func(offset int64)
	collectCandidate = func(offset int64) {
		vals, subIFDOffsets, nextOffset, err := parseIFD(r, offset, order)
		if err != nil {
			return
		}

		// Build candidate from strip or tile data.
		cand := buildCandidate(vals)
		if cand != nil {
			candidates = append(candidates, cand)
		}

		// Follow SubIFD pointers (tag 0x014A).
		for _, subOffset := range subIFDOffsets {
			subVals, _, _, err := parseIFD(r, subOffset, order)
			if err != nil {
				continue
			}
			subCand := buildCandidate(subVals)
			if subCand != nil {
				candidates = append(candidates, subCand)
			}
		}

		// Walk to next IFD in chain.
		if nextOffset != 0 {
			collectCandidate(nextOffset)
		}
	}

	collectCandidate(ifd0Offset)

	if len(candidates) == 0 {
		return nil, nil
	}

	// Select the best candidate:
	// 1. Prefer NewSubfileType == 0 (full-resolution primary image).
	// 2. Among ties, prefer the largest total payload.
	var best *ifdCandidate
	for _, c := range candidates {
		if best == nil {
			best = c
			continue
		}
		// Prefer primary image (NewSubfileType == 0) over reduced-resolution.
		bestIsPrimary := best.vals.newSubfileType == 0
		candIsPrimary := c.vals.newSubfileType == 0
		if candIsPrimary && !bestIsPrimary {
			best = c
			continue
		}
		if bestIsPrimary && !candIsPrimary {
			continue
		}
		// Both same subfile type — prefer larger payload.
		if c.totalSize > best.totalSize {
			best = c
		}
	}

	if best == nil || best.totalSize == 0 {
		return nil, nil
	}

	return &sensorRegion{
		offsets:    best.offsets,
		byteCounts: best.byteCounts,
		totalSize:  best.totalSize,
	}, nil
}

// buildCandidate constructs an ifdCandidate from parsed IFD values.
// Returns nil if the IFD uses standard JPEG compression (it's a preview)
// or has no strip/tile data.
func buildCandidate(vals *ifdValues) *ifdCandidate {
	// Exclude standard JPEG preview IFDs.
	if vals.compression == compressionJPEG {
		return nil
	}

	var offsets, byteCounts []int64
	var totalSize int64

	if len(vals.stripOffsets) > 0 && len(vals.stripByteCounts) > 0 {
		// Strip-based layout.
		n := len(vals.stripOffsets)
		if len(vals.stripByteCounts) < n {
			n = len(vals.stripByteCounts)
		}
		offsets = make([]int64, n)
		byteCounts = make([]int64, n)
		for i := 0; i < n; i++ {
			offsets[i] = int64(vals.stripOffsets[i])
			byteCounts[i] = int64(vals.stripByteCounts[i])
			totalSize += byteCounts[i]
		}
	} else if len(vals.tileOffsets) > 0 && len(vals.tileByteCounts) > 0 {
		// Tile-based layout.
		n := len(vals.tileOffsets)
		if len(vals.tileByteCounts) < n {
			n = len(vals.tileByteCounts)
		}
		offsets = make([]int64, n)
		byteCounts = make([]int64, n)
		for i := 0; i < n; i++ {
			offsets[i] = int64(vals.tileOffsets[i])
			byteCounts[i] = int64(vals.tileByteCounts[i])
			totalSize += byteCounts[i]
		}
	}

	if totalSize == 0 {
		return nil
	}

	return &ifdCandidate{
		vals:       vals,
		offsets:    offsets,
		byteCounts: byteCounts,
		totalSize:  totalSize,
	}
}

// parseIFD reads a TIFF IFD at the given offset and extracts all relevant
// tag values. Returns the parsed values, sub-IFD offsets, the next IFD
// offset, and any error.
func parseIFD(r io.ReadSeeker, offset int64, order binary.ByteOrder) (*ifdValues, []int64, int64, error) {
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
		case tagImageWidth:
			vals.imageWidth = readUint32Scalar(order, typ, valOff)

		case tagImageLength:
			vals.imageLength = readUint32Scalar(order, typ, valOff)

		case tagNewSubfileType:
			vals.newSubfileType = order.Uint32(valOff)

		case tagCompression:
			vals.compression = uint32(order.Uint16(valOff))

		case tagStripOffsets:
			vals.stripOffsets = readUint32Array(r, order, typ, cnt, valOff)

		case tagStripByteCounts:
			vals.stripByteCounts = readUint32Array(r, order, typ, cnt, valOff)

		case tagTileOffsets:
			vals.tileOffsets = readUint32Array(r, order, typ, cnt, valOff)

		case tagTileByteCounts:
			vals.tileByteCounts = readUint32Array(r, order, typ, cnt, valOff)

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

	return vals, vals.subIFDOffsets, nextOffset, nil
}

// readUint32Scalar reads a single uint32 value from a TIFF IFD entry's
// value/offset field. Handles both SHORT (2-byte) and LONG (4-byte) types.
func readUint32Scalar(order binary.ByteOrder, typ uint16, valOff []byte) uint32 {
	const typeShort = 3
	if typ == typeShort {
		return uint32(order.Uint16(valOff))
	}
	return order.Uint32(valOff)
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
