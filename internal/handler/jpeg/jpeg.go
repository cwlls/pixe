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

// Package jpeg implements the FileTypeHandler contract for JPEG images.
//
// Package selection:
//   - EXIF read:  github.com/rwcarlsen/goexif — pure Go, handles DateTimeOriginal
//     and CreateDate from IFD0/ExifIFD.
//   - EXIF write: github.com/dsoprea/go-exif/v3 +
//     github.com/dsoprea/go-jpeg-image-structure/v2 — pure Go, allows
//     rebuilding the JPEG APP1 segment with updated tags.
//
// Hashable region:
//
//	The JPEG media payload is defined as the raw bytes from the SOS marker
//	(0xFF 0xDA) through to and including the EOI marker (0xFF 0xD9). This
//	excludes all APP markers (EXIF, ICC, XMP, etc.) so that metadata edits
//	do not invalidate the stored checksum.
package jpeg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	goexif "github.com/dsoprea/go-exif/v3"
	jpegstructure "github.com/dsoprea/go-jpeg-image-structure/v2"
	rwexif "github.com/rwcarlsen/goexif/exif"

	"github.com/cwlls/pixe-go/internal/domain"
)

// anselsAdams is the fallback date when no EXIF date can be extracted.
// February 20, 1902 — Ansel Adams' birthday.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

// exifDateFormat is the EXIF date/time string format.
const exifDateFormat = "2006:01:02 15:04:05"

// Handler implements domain.FileTypeHandler for JPEG images.
type Handler struct{}

// New returns a new JPEG Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".jpg", ".jpeg"}
}

// MagicBytes returns the JPEG SOI magic signature.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte{0xFF, 0xD8, 0xFF}},
	}
}

// Detect returns true if the file has a .jpg/.jpeg extension AND begins with
// the JPEG SOI magic bytes 0xFF 0xD8 0xFF.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileExt(filePath))
	if ext != ".jpg" && ext != ".jpeg" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("jpeg: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 3)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	return header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF, nil
}

// ExtractDate reads the capture date from EXIF metadata using the fallback chain:
//  1. DateTimeOriginal
//  2. CreateDate (DateTime in IFD0)
//  3. anselsAdams (1902-02-20) — makes undated files identifiable by prefix
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return anselsAdams, fmt.Errorf("jpeg: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	x, err := rwexif.Decode(f)
	if err != nil {
		// No EXIF data at all — use fallback.
		return anselsAdams, nil
	}

	// 1. DateTimeOriginal — parse as UTC to avoid timezone issues with goexif's DateTime().
	if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	// 2. CreateDate (DateTime tag in IFD0) — parse as UTC.
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

// HashableReader returns an io.ReadCloser over the JPEG media payload —
// the raw bytes from the SOS marker (0xFF 0xDA) through to and including
// the EOI marker (0xFF 0xD9). All APP markers (EXIF, ICC, XMP, etc.) are
// excluded so that metadata edits do not change the checksum.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("jpeg: read %q: %w", filePath, err)
	}

	payload, err := extractSOSPayload(data)
	if err != nil {
		return nil, fmt.Errorf("jpeg: extract SOS payload from %q: %w", filePath, err)
	}

	return io.NopCloser(bytes.NewReader(payload)), nil
}

// WriteMetadataTags injects Copyright and CameraOwner EXIF tags into the
// destination JPEG file. It is a no-op when tags.IsEmpty() is true.
// The file is rewritten in-place using go-jpeg-image-structure.
func (h *Handler) WriteMetadataTags(filePath string, tags domain.MetadataTags) error {
	if tags.IsEmpty() {
		return nil
	}

	parser := jpegstructure.NewJpegMediaParser()
	intfc, err := parser.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("jpeg: parse %q for tagging: %w", filePath, err)
	}

	sl := intfc.(*jpegstructure.SegmentList)

	rootIb, err := sl.ConstructExifBuilder()
	if err != nil {
		// No existing EXIF block — skip tagging gracefully rather than failing.
		// A future enhancement could create EXIF from scratch here.
		return nil
	}

	if tags.Copyright != "" {
		ifdIb, err := goexif.GetOrCreateIbFromRootIb(rootIb, "IFD0")
		if err != nil {
			return fmt.Errorf("jpeg: get IFD0 builder: %w", err)
		}
		if err := ifdIb.SetStandardWithName("Copyright", tags.Copyright); err != nil {
			return fmt.Errorf("jpeg: set Copyright tag: %w", err)
		}
	}

	if tags.CameraOwner != "" {
		// CameraOwnerName (0xA430) lives in the ExifIFD sub-IFD.
		exifIb, err := goexif.GetOrCreateIbFromRootIb(rootIb, "IFD0/Exif0")
		if err != nil {
			return fmt.Errorf("jpeg: get ExifIFD builder: %w", err)
		}
		if err := exifIb.SetStandardWithName("CameraOwnerName", tags.CameraOwner); err != nil {
			return fmt.Errorf("jpeg: set CameraOwnerName tag: %w", err)
		}
	}

	if err := sl.SetExif(rootIb); err != nil {
		return fmt.Errorf("jpeg: set EXIF in segment list: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("jpeg: open %q for writing: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	if err := sl.Write(f); err != nil {
		return fmt.Errorf("jpeg: write tagged JPEG to %q: %w", filePath, err)
	}

	return nil
}

// extractSOSPayload scans the JPEG byte stream and returns the bytes from
// the SOS marker (0xFF 0xDA) through to and including the EOI (0xFF 0xD9).
// This is the media payload used for hashing.
func extractSOSPayload(data []byte) ([]byte, error) {
	i := 0
	n := len(data)

	for i < n-1 {
		if data[i] != 0xFF {
			return nil, fmt.Errorf("expected 0xFF at offset %d, got 0x%02X", i, data[i])
		}
		marker := data[i+1]

		// SOI — no length field.
		if marker == 0xD8 {
			i += 2
			continue
		}
		// EOI before SOS — malformed.
		if marker == 0xD9 {
			break
		}
		// SOS — return from here to end of file (includes EOI).
		if marker == 0xDA {
			return data[i:], nil
		}
		// RST0–RST7 — no length field.
		if marker >= 0xD0 && marker <= 0xD7 {
			i += 2
			continue
		}
		// All other markers carry a 2-byte big-endian length.
		if i+3 >= n {
			return nil, fmt.Errorf("truncated JPEG at offset %d", i)
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		i += 2 + segLen
	}

	return nil, fmt.Errorf("SOS marker not found in JPEG data")
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
