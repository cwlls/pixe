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
//	The raw sensor data is extracted from the ISOBMFF container. CR3 stores
//	sensor data in the mdat box, referenced by track metadata in the moov
//	box. The handler navigates moov → trak → mdia → minf → stbl to locate
//	chunk offsets (stco/co64) and sample sizes (stsz) for the primary image
//	track (largest total sample size). Returns a reader over that sensor data
//	region. Falls back to the full mdat box contents if track parsing fails.
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
	ext := strings.ToLower(fileExt(filePath))
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

// HashableReader returns an io.ReadCloser over the raw sensor data extracted
// from the CR3 ISOBMFF container. It navigates moov track metadata to locate
// the primary image track's data within mdat. Falls back to the full file.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cr3: open %q: %w", filePath, err)
	}

	sensor, err := findCR3SensorData(f)
	if err != nil || sensor == nil {
		// Fallback: hash the full file.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("cr3: seek %q: %w", filePath, err)
		}
		return f, nil
	}

	sr := io.NewSectionReader(f, sensor.offset, sensor.size)
	return &sectionReadCloser{Reader: sr, Closer: f}, nil
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

// sensorRegion holds the offset and size of the raw sensor data within
// the CR3 mdat box.
type sensorRegion struct {
	offset int64
	size   int64
}

// sectionReadCloser wraps an io.Reader with a separate io.Closer.
type sectionReadCloser struct {
	Reader io.Reader
	Closer io.Closer
}

// Read implements io.Reader by reading from a bounded file section.
func (s *sectionReadCloser) Read(p []byte) (int, error) { return s.Reader.Read(p) }

// Close implements io.Closer by closing the underlying file.
func (s *sectionReadCloser) Close() error { return s.Closer.Close() }

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

// findCR3SensorData navigates the ISOBMFF box structure to locate the raw
// sensor data within the mdat box. It parses moov → trak → mdia → minf →
// stbl to find chunk offsets (stco/co64) and sample sizes (stsz) for the
// primary image track (largest total sample size).
//
// Falls back to returning the entire mdat box contents if track parsing fails.
func findCR3SensorData(r io.ReadSeeker) (*sensorRegion, error) {
	fileSize, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	topBoxes, err := walkBoxes(r, 0, fileSize)
	if err != nil {
		return nil, err
	}

	// Locate moov and mdat boxes.
	var moovBox, mdatBox *isobmffBox
	for _, box := range topBoxes {
		switch box.boxType {
		case "moov":
			moovBox = box
		case "mdat":
			if mdatBox == nil || box.size > mdatBox.size {
				mdatBox = box // prefer the largest mdat
			}
		}
	}

	if mdatBox == nil {
		return nil, nil
	}

	mdatDataStart := mdatBox.dataOff
	mdatDataSize := mdatBox.size - (mdatBox.dataOff - mdatBox.offset)

	// If no moov box, fall back to the full mdat contents.
	if moovBox == nil {
		return &sensorRegion{offset: mdatDataStart, size: mdatDataSize}, nil
	}

	// Parse moov to find the primary image track.
	sensor, err := findPrimaryTrackInMoov(r, moovBox)
	if err != nil || sensor == nil {
		// Fall back to full mdat.
		return &sensorRegion{offset: mdatDataStart, size: mdatDataSize}, nil
	}

	return sensor, nil
}

// findPrimaryTrackInMoov walks moov child boxes to find trak boxes, parses
// each track's stbl (sample table), and returns the sensor region for the
// track with the largest total sample size (the primary image track).
func findPrimaryTrackInMoov(r io.ReadSeeker, moovBox *isobmffBox) (*sensorRegion, error) {
	moovEnd := moovBox.offset + moovBox.size
	moovChildren, err := walkBoxes(r, moovBox.dataOff, moovEnd)
	if err != nil {
		return nil, err
	}

	var bestSensor *sensorRegion
	var bestSize int64

	for _, child := range moovChildren {
		if child.boxType != "trak" {
			continue
		}
		sensor, totalSize, err := parseTrak(r, child)
		if err != nil || sensor == nil {
			continue
		}
		if sensor != nil && totalSize > bestSize {
			bestSensor = sensor
			bestSize = totalSize
		}
	}

	return bestSensor, nil
}

// parseTrak navigates trak → mdia → minf → stbl to extract chunk offsets
// and sample sizes. Returns the sensor region and total sample size.
func parseTrak(r io.ReadSeeker, trakBox *isobmffBox) (*sensorRegion, int64, error) {
	trakEnd := trakBox.offset + trakBox.size
	trakChildren, err := walkBoxes(r, trakBox.dataOff, trakEnd)
	if err != nil {
		return nil, 0, err
	}

	for _, child := range trakChildren {
		if child.boxType != "mdia" {
			continue
		}
		sensor, totalSize, err := parseMdia(r, child)
		if err != nil || sensor == nil {
			continue
		}
		return sensor, totalSize, nil
	}

	return nil, 0, nil
}

// parseMdia navigates mdia → minf → stbl.
func parseMdia(r io.ReadSeeker, mdiaBox *isobmffBox) (*sensorRegion, int64, error) {
	mdiaEnd := mdiaBox.offset + mdiaBox.size
	mdiaChildren, err := walkBoxes(r, mdiaBox.dataOff, mdiaEnd)
	if err != nil {
		return nil, 0, err
	}

	for _, child := range mdiaChildren {
		if child.boxType != "minf" {
			continue
		}
		sensor, totalSize, err := parseMinf(r, child)
		if err != nil || sensor == nil {
			continue
		}
		return sensor, totalSize, nil
	}

	return nil, 0, nil
}

// parseMinf navigates minf → stbl.
func parseMinf(r io.ReadSeeker, minfBox *isobmffBox) (*sensorRegion, int64, error) {
	minfEnd := minfBox.offset + minfBox.size
	minfChildren, err := walkBoxes(r, minfBox.dataOff, minfEnd)
	if err != nil {
		return nil, 0, err
	}

	for _, child := range minfChildren {
		if child.boxType != "stbl" {
			continue
		}
		sensor, totalSize, err := parseStbl(r, child)
		if err != nil || sensor == nil {
			continue
		}
		return sensor, totalSize, nil
	}

	return nil, 0, nil
}

// stblData holds the parsed sample table data needed to locate sensor data.
type stblData struct {
	chunkOffsets []int64 // from stco or co64
	sampleSizes  []int64 // from stsz (uniform or per-sample)
	// stsc: sample-to-chunk mapping (first_chunk, samples_per_chunk, sample_desc_idx)
	stscEntries []stscEntry
}

// stscEntry represents a sample-to-chunk table entry in an ISOBMFF track.
type stscEntry struct {
	firstChunk      uint32
	samplesPerChunk uint32
}

// parseStbl reads stco/co64, stsz, and stsc boxes from the sample table.
// Returns the sensor region (contiguous byte range covering all samples)
// and total sample size.
func parseStbl(r io.ReadSeeker, stblBox *isobmffBox) (*sensorRegion, int64, error) {
	stblEnd := stblBox.offset + stblBox.size
	stblChildren, err := walkBoxes(r, stblBox.dataOff, stblEnd)
	if err != nil {
		return nil, 0, err
	}

	data := &stblData{}

	for _, child := range stblChildren {
		switch child.boxType {
		case "stco":
			data.chunkOffsets = parseStco(r, child)
		case "co64":
			data.chunkOffsets = parseCo64(r, child)
		case "stsz":
			data.sampleSizes = parseStsz(r, child)
		case "stsc":
			data.stscEntries = parseStsc(r, child)
		}
	}

	if len(data.chunkOffsets) == 0 || len(data.sampleSizes) == 0 {
		return nil, 0, nil
	}

	// Compute total sample size.
	var totalSize int64
	for _, s := range data.sampleSizes {
		totalSize += s
	}
	if totalSize == 0 {
		return nil, 0, nil
	}

	// Find the contiguous byte range covering all samples.
	// Use the first chunk offset as the start and totalSize as the extent.
	// This is a simplification that works for single-chunk tracks (common in CR3).
	// For multi-chunk tracks, we use the minimum offset and span to the end.
	minOffset := data.chunkOffsets[0]
	for _, off := range data.chunkOffsets {
		if off < minOffset {
			minOffset = off
		}
	}

	return &sensorRegion{
		offset: minOffset,
		size:   totalSize,
	}, totalSize, nil
}

// parseStco reads a stco (32-bit chunk offset) box.
func parseStco(r io.ReadSeeker, box *isobmffBox) []int64 {
	if _, err := r.Seek(box.dataOff, io.SeekStart); err != nil {
		return nil
	}
	// version(1) + flags(3) + entry_count(4)
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil
	}
	count := binary.BigEndian.Uint32(header[4:8])
	if count == 0 || count > 1<<20 { // sanity cap
		return nil
	}
	offsets := make([]int64, count)
	for i := uint32(0); i < count; i++ {
		var v uint32
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return offsets[:i]
		}
		offsets[i] = int64(v)
	}
	return offsets
}

// parseCo64 reads a co64 (64-bit chunk offset) box.
func parseCo64(r io.ReadSeeker, box *isobmffBox) []int64 {
	if _, err := r.Seek(box.dataOff, io.SeekStart); err != nil {
		return nil
	}
	// version(1) + flags(3) + entry_count(4)
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil
	}
	count := binary.BigEndian.Uint32(header[4:8])
	if count == 0 || count > 1<<20 {
		return nil
	}
	offsets := make([]int64, count)
	for i := uint32(0); i < count; i++ {
		var v uint64
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return offsets[:i]
		}
		offsets[i] = int64(v)
	}
	return offsets
}

// parseStsz reads a stsz (sample size) box.
// Returns per-sample sizes. If uniform size is set, expands to a slice.
func parseStsz(r io.ReadSeeker, box *isobmffBox) []int64 {
	if _, err := r.Seek(box.dataOff, io.SeekStart); err != nil {
		return nil
	}
	// version(1) + flags(3) + sample_size(4) + sample_count(4)
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil
	}
	uniformSize := binary.BigEndian.Uint32(header[4:8])
	count := binary.BigEndian.Uint32(header[8:12])
	if count == 0 || count > 1<<24 {
		return nil
	}

	sizes := make([]int64, count)
	if uniformSize != 0 {
		// All samples have the same size.
		for i := range sizes {
			sizes[i] = int64(uniformSize)
		}
		return sizes
	}

	// Per-sample sizes.
	for i := uint32(0); i < count; i++ {
		var v uint32
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return sizes[:i]
		}
		sizes[i] = int64(v)
	}
	return sizes
}

// parseStsc reads a stsc (sample-to-chunk) box.
func parseStsc(r io.ReadSeeker, box *isobmffBox) []stscEntry {
	if _, err := r.Seek(box.dataOff, io.SeekStart); err != nil {
		return nil
	}
	// version(1) + flags(3) + entry_count(4)
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil
	}
	count := binary.BigEndian.Uint32(header[4:8])
	if count == 0 || count > 1<<20 {
		return nil
	}
	entries := make([]stscEntry, count)
	for i := uint32(0); i < count; i++ {
		var firstChunk, samplesPerChunk, sampleDescIdx uint32
		if err := binary.Read(r, binary.BigEndian, &firstChunk); err != nil {
			return entries[:i]
		}
		if err := binary.Read(r, binary.BigEndian, &samplesPerChunk); err != nil {
			return entries[:i]
		}
		if err := binary.Read(r, binary.BigEndian, &sampleDescIdx); err != nil {
			return entries[:i]
		}
		entries[i] = stscEntry{
			firstChunk:      firstChunk,
			samplesPerChunk: samplesPerChunk,
		}
	}
	return entries
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

// fileExt returns the file extension including the leading dot, or "".
func fileExt(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}
