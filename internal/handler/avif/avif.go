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

// Package avif implements the FileTypeHandler contract for AVIF images.
//
// AVIF (AV1 Image File Format) is an ISOBMFF container — the same box-based
// format used by HEIC, MP4, and CR3 — with AV1-compressed image data. It is
// the default photo format on iPhone 16+ (iOS 18) and is increasingly used
// by Android devices, Chrome, and web platforms.
//
// Relationship to HEIC:
//
//	AVIF and HEIC share nearly identical container structure. The key
//	difference is the ftyp major brand ("avif"/"avis" vs "heic"/"heif") and
//	the image codec (AV1 vs HEVC). From Pixe's perspective — detection, EXIF
//	extraction, and hashing — the two formats are structurally equivalent.
//
// EXIF extraction:
//
//	Spike result: github.com/dsoprea/go-heic-exif-extractor only recognises
//	the brands "mif1", "msf1", and "heic" — it rejects "avif"-branded files.
//	A minimal custom ISOBMFF meta box parser is therefore implemented here.
//	The parser navigates: ftyp → meta → iinf (item info) → iloc (item
//	locations) to locate the raw EXIF blob, then hands it to
//	github.com/rwcarlsen/goexif for standard EXIF tag parsing.
//
// Hashable region:
//
//	Full-file hash. AVIF's compressed AV1 payload within mdat cannot be
//	cleanly separated from container metadata without a full AV1 decoder.
//	AVIF files from cameras are not edited in place, so the full-file hash
//	is stable and deterministic.
//
// Magic bytes:
//
//	AVIF files are ISOBMFF containers. The ftyp box starts at offset 0:
//	  bytes 0-3: box size (big-endian uint32, variable)
//	  bytes 4-7: box type "ftyp" (0x66 0x74 0x79 0x70)
//	  bytes 8-11: major brand — "avif", "avis", or "mif1"
//	We match on the "ftyp" type bytes at offset 4 (same as HEIC).
//	Detect() additionally verifies the major brand is AVIF-specific.
package avif

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

// avifBrands is the set of ISOBMFF major brands that identify AVIF files.
var avifBrands = map[string]bool{
	"avif": true, // AV1 still image
	"avis": true, // AV1 image sequence
	"mif1": true, // generic HEIF — used by some AVIF encoders
}

// Handler implements domain.FileTypeHandler for AVIF images.
type Handler struct{}

// New returns a new AVIF Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".avif"}
}

// MagicBytes returns the ISOBMFF ftyp box signature.
// We match "ftyp" at offset 4 (after the 4-byte box size field),
// identical to the HEIC handler.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
	}
}

// Detect returns true if the file has a .avif extension AND contains the
// ISOBMFF "ftyp" box signature at offset 4 with an AVIF-specific major brand.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".avif" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("avif: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	// Read 12 bytes: box size (4) + "ftyp" (4) + major brand (4).
	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	// Verify "ftyp" at offset 4.
	if header[4] != 0x66 || header[5] != 0x74 || header[6] != 0x79 || header[7] != 0x70 {
		return false, nil
	}
	// Verify AVIF-specific major brand at offset 8–11.
	brand := string(header[8:12])
	return avifBrands[brand], nil
}

// ExtractDate reads the capture date from AVIF EXIF metadata using a minimal
// ISOBMFF meta box parser. Fallback chain: DateTimeOriginal → DateTime → anselsAdams.
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	t, err := extractAVIFDate(filePath)
	if err != nil {
		// Non-fatal: fall back to Ansel Adams date.
		return anselsAdams, nil
	}
	return t, nil
}

// HashableReader returns an io.ReadCloser over the full file contents.
// See package doc for rationale on full-file hashing for AVIF.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("avif: open %q: %w", filePath, err)
	}
	return f, nil
}

// MetadataSupport declares that AVIF uses XMP sidecar files.
// No pure-Go AVIF EXIF writer exists; writing into the ISOBMFF container
// risks corruption.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// ---------------------------------------------------------------------------
// ISOBMFF meta box parser — minimal implementation for EXIF extraction
// ---------------------------------------------------------------------------
//
// AVIF stores EXIF in the ISOBMFF meta box hierarchy:
//
//	ftyp → meta (FullBox) → iinf (item info) → infe entries → iloc (item locations)
//
// The parser:
//  1. Skips the ftyp box.
//  2. Scans top-level boxes for "meta".
//  3. Within meta, finds the "Exif" item ID via iinf/infe entries.
//  4. Reads the EXIF blob offset and length from the iloc box.
//  5. Reads the raw EXIF bytes and hands them to rwcarlsen/goexif.

// isobmffBox holds the parsed header of a single ISOBMFF box.
type isobmffBox struct {
	size    uint64 // total box size in bytes (including header)
	boxType string // 4-character box type
	offset  int64  // file offset of the first byte of this box
	hdrSize int64  // size of the box header (8 or 16 bytes)
}

// extractAVIFDate parses the AVIF ISOBMFF container to locate and decode
// the EXIF blob, then extracts the capture date.
func extractAVIFDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("avif: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return time.Time{}, fmt.Errorf("avif: stat %q: %w", filePath, err)
	}
	fileSize := fi.Size()

	// Step 1: scan top-level boxes to find "meta".
	var metaBox *isobmffBox
	pos := int64(0)
	for pos < fileSize {
		box, err := readBoxHeader(f, pos)
		if err != nil {
			break
		}
		if box.boxType == "meta" {
			metaBox = box
			break
		}
		// Advance past this box.
		pos += int64(box.size)
	}
	if metaBox == nil {
		return time.Time{}, fmt.Errorf("avif: no meta box found")
	}

	// Step 2: parse the meta box contents to find iinf and iloc.
	// meta is a FullBox: 4 bytes version+flags after the standard header.
	metaContentStart := metaBox.offset + metaBox.hdrSize + 4 // skip version+flags
	metaEnd := metaBox.offset + int64(metaBox.size)

	exifItemID, err := findExifItemID(f, metaContentStart, metaEnd)
	if err != nil {
		return time.Time{}, fmt.Errorf("avif: find Exif item: %w", err)
	}

	exifOffset, exifLen, err := findExifLocation(f, metaContentStart, metaEnd, exifItemID)
	if err != nil {
		return time.Time{}, fmt.Errorf("avif: find Exif location: %w", err)
	}

	// Step 3: read the raw EXIF bytes.
	if _, err := f.Seek(exifOffset, io.SeekStart); err != nil {
		return time.Time{}, fmt.Errorf("avif: seek to EXIF at %d: %w", exifOffset, err)
	}
	rawExif := make([]byte, exifLen)
	if _, err := io.ReadFull(f, rawExif); err != nil {
		return time.Time{}, fmt.Errorf("avif: read EXIF bytes: %w", err)
	}

	// AVIF EXIF items begin with a 4-byte offset to the TIFF header within
	// the blob (per ISO 23008-12 §6.5.11.1). Skip it.
	if len(rawExif) > 4 {
		skip := binary.BigEndian.Uint32(rawExif[:4])
		if int(skip)+4 <= len(rawExif) {
			rawExif = rawExif[4+skip:]
		}
	}

	// Step 4: parse EXIF and extract date.
	x, err := rwexif.Decode(bytes.NewReader(rawExif))
	if err != nil {
		return time.Time{}, fmt.Errorf("avif: decode EXIF: %w", err)
	}

	if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}
	if tag, err := x.Get(rwexif.DateTime); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("avif: no date tag found in EXIF")
}

// readBoxHeader reads the 8-byte (or 16-byte extended) ISOBMFF box header
// at the given file offset and returns an isobmffBox.
func readBoxHeader(r io.ReadSeeker, offset int64) (*isobmffBox, error) {
	if _, err := r.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	size := uint64(binary.BigEndian.Uint32(hdr[0:4]))
	boxType := string(hdr[4:8])
	hdrSize := int64(8)

	switch size {
	case 0:
		// Box extends to end of file — caller must handle.
		// We return size=0 as a sentinel; callers should treat it as
		// "rest of file" and stop scanning.
		return &isobmffBox{size: 0, boxType: boxType, offset: offset, hdrSize: hdrSize}, nil
	case 1:
		// Extended 64-bit size follows the 8-byte header.
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return nil, err
		}
		size = binary.BigEndian.Uint64(ext)
		hdrSize = 16
	}

	return &isobmffBox{size: size, boxType: boxType, offset: offset, hdrSize: hdrSize}, nil
}

// findExifItemID scans the iinf box within the meta box to find the item ID
// of the "Exif" item. Returns the item ID or an error if not found.
func findExifItemID(r io.ReadSeeker, metaContentStart, metaEnd int64) (uint16, error) {
	pos := metaContentStart
	for pos < metaEnd {
		box, err := readBoxHeader(r, pos)
		if err != nil || box.size == 0 {
			break
		}
		if box.boxType == "iinf" {
			// iinf is a FullBox: 4 bytes version+flags, then entry_count.
			// version 0: entry_count is uint16; version 1: uint32.
			innerStart := box.offset + box.hdrSize
			if _, err := r.Seek(innerStart, io.SeekStart); err != nil {
				return 0, err
			}
			var versionFlags [4]byte
			if _, err := io.ReadFull(r, versionFlags[:]); err != nil {
				return 0, err
			}
			version := versionFlags[0]

			var entryCount uint32
			if version == 0 {
				var ec uint16
				if err := binary.Read(r, binary.BigEndian, &ec); err != nil {
					return 0, err
				}
				entryCount = uint32(ec)
			} else {
				if err := binary.Read(r, binary.BigEndian, &entryCount); err != nil {
					return 0, err
				}
			}

			// Scan infe (item info entry) boxes.
			iinfEnd := box.offset + int64(box.size)
			infePos, err := r.Seek(0, io.SeekCurrent)
			if err != nil {
				return 0, err
			}
			for i := uint32(0); i < entryCount && infePos < iinfEnd; i++ {
				infe, err := readBoxHeader(r, infePos)
				if err != nil || infe.size == 0 {
					break
				}
				itemID, itemType, err := parseInfe(r, infe)
				if err == nil && itemType == "Exif" {
					return itemID, nil
				}
				infePos += int64(infe.size)
			}
			return 0, fmt.Errorf("avif: Exif item not found in iinf")
		}
		pos += int64(box.size)
	}
	return 0, fmt.Errorf("avif: iinf box not found in meta")
}

// parseInfe parses an infe (item info entry) FullBox and returns the item ID
// and item type string. Supports infe version 2 and 3 (the common cases for
// AVIF files produced by modern encoders).
func parseInfe(r io.ReadSeeker, box *isobmffBox) (itemID uint16, itemType string, err error) {
	if _, err = r.Seek(box.offset+box.hdrSize, io.SeekStart); err != nil {
		return
	}
	// FullBox: version (1 byte) + flags (3 bytes).
	var vf [4]byte
	if _, err = io.ReadFull(r, vf[:]); err != nil {
		return
	}
	version := vf[0]

	switch version {
	case 2:
		var id uint16
		if err = binary.Read(r, binary.BigEndian, &id); err != nil {
			return
		}
		itemID = id
		// item_protection_index (uint16) — skip.
		if _, err = r.Seek(2, io.SeekCurrent); err != nil {
			return
		}
		// item_type (4 bytes).
		var t [4]byte
		if _, err = io.ReadFull(r, t[:]); err != nil {
			return
		}
		itemType = string(t[:])
	case 3:
		var id uint32
		if err = binary.Read(r, binary.BigEndian, &id); err != nil {
			return
		}
		itemID = uint16(id)
		// item_protection_index (uint16) — skip.
		if _, err = r.Seek(2, io.SeekCurrent); err != nil {
			return
		}
		// item_type (4 bytes).
		var t [4]byte
		if _, err = io.ReadFull(r, t[:]); err != nil {
			return
		}
		itemType = string(t[:])
	default:
		err = fmt.Errorf("avif: unsupported infe version %d", version)
	}
	return
}

// findExifLocation scans the iloc box within the meta box to find the file
// offset and byte length of the EXIF item identified by exifItemID.
func findExifLocation(r io.ReadSeeker, metaContentStart, metaEnd int64, exifItemID uint16) (offset int64, length int64, err error) {
	pos := metaContentStart
	for pos < metaEnd {
		box, err := readBoxHeader(r, pos)
		if err != nil || box.size == 0 {
			break
		}
		if box.boxType == "iloc" {
			return parseIloc(r, box, exifItemID)
		}
		pos += int64(box.size)
	}
	return 0, 0, fmt.Errorf("avif: iloc box not found in meta")
}

// parseIloc parses the iloc (item location) FullBox and returns the file
// offset and byte length for the given item ID.
//
// iloc layout (simplified for version 0/1, offset_size=4, length_size=4,
// base_offset_size=4, index_size=0 — the common case for AVIF still images):
//
//	FullBox header (version + flags)
//	offset_size (4 bits) | length_size (4 bits)
//	base_offset_size (4 bits) | index_size (4 bits)   [version 1/2 only]
//	item_count (uint16 for v0/1, uint32 for v2)
//	for each item:
//	  item_ID (uint16 for v0/1, uint32 for v2)
//	  construction_method (uint16, v1/2 only)
//	  data_reference_index (uint16)
//	  base_offset (base_offset_size bytes)
//	  extent_count (uint16)
//	  for each extent:
//	    extent_index (index_size bytes, v1/2 only)
//	    extent_offset (offset_size bytes)
//	    extent_length (length_size bytes)
func parseIloc(r io.ReadSeeker, box *isobmffBox, targetItemID uint16) (offset int64, length int64, err error) {
	if _, err = r.Seek(box.offset+box.hdrSize, io.SeekStart); err != nil {
		return
	}
	// FullBox: version (1) + flags (3).
	var vf [4]byte
	if _, err = io.ReadFull(r, vf[:]); err != nil {
		return
	}
	version := vf[0]

	// offset_size | length_size packed in one byte.
	var sizes [2]byte
	if _, err = io.ReadFull(r, sizes[:]); err != nil {
		return
	}
	offsetSize := int(sizes[0] >> 4)
	lengthSize := int(sizes[0] & 0x0F)
	baseOffsetSize := int(sizes[1] >> 4)
	indexSize := 0
	if version == 1 || version == 2 {
		indexSize = int(sizes[1] & 0x0F)
	}

	// item_count.
	var itemCount uint32
	if version < 2 {
		var ic uint16
		if err = binary.Read(r, binary.BigEndian, &ic); err != nil {
			return
		}
		itemCount = uint32(ic)
	} else {
		if err = binary.Read(r, binary.BigEndian, &itemCount); err != nil {
			return
		}
	}

	for i := uint32(0); i < itemCount; i++ {
		// item_ID.
		var itemID uint32
		if version < 2 {
			var id uint16
			if err = binary.Read(r, binary.BigEndian, &id); err != nil {
				return
			}
			itemID = uint32(id)
		} else {
			if err = binary.Read(r, binary.BigEndian, &itemID); err != nil {
				return
			}
		}

		// construction_method (version 1/2 only).
		if version == 1 || version == 2 {
			if _, err = r.Seek(2, io.SeekCurrent); err != nil {
				return
			}
		}

		// data_reference_index (uint16).
		if _, err = r.Seek(2, io.SeekCurrent); err != nil {
			return
		}

		// base_offset.
		baseOffset, err2 := readUintN(r, baseOffsetSize)
		if err2 != nil {
			err = err2
			return
		}

		// extent_count.
		var extentCount uint16
		if err = binary.Read(r, binary.BigEndian, &extentCount); err != nil {
			return
		}

		for e := uint16(0); e < extentCount; e++ {
			// extent_index (version 1/2 only, index_size bytes).
			if (version == 1 || version == 2) && indexSize > 0 {
				if _, err = r.Seek(int64(indexSize), io.SeekCurrent); err != nil {
					return
				}
			}
			extOffset, err2 := readUintN(r, offsetSize)
			if err2 != nil {
				err = err2
				return
			}
			extLength, err2 := readUintN(r, lengthSize)
			if err2 != nil {
				err = err2
				return
			}

			if uint16(itemID) == targetItemID {
				// Found our item. Return the first extent only (EXIF is always
				// a single contiguous extent in practice).
				return int64(baseOffset + extOffset), int64(extLength), nil
			}
		}
	}
	return 0, 0, fmt.Errorf("avif: item ID %d not found in iloc", targetItemID)
}

// readUintN reads an n-byte big-endian unsigned integer from r.
// n must be 0, 4, or 8. Returns 0 for n==0 (absent field).
func readUintN(r io.Reader, n int) (uint64, error) {
	switch n {
	case 0:
		return 0, nil
	case 4:
		var v uint32
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return 0, err
		}
		return uint64(v), nil
	case 8:
		var v uint64
		if err := binary.Read(r, binary.BigEndian, &v); err != nil {
			return 0, err
		}
		return v, nil
	default:
		// Uncommon size — read byte by byte.
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		var v uint64
		for _, b := range buf {
			v = (v << 8) | uint64(b)
		}
		return v, nil
	}
}
