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

// Package raf provides a FileTypeHandler for Fujifilm's RAF raw image format.
//
// RAF uses a custom binary container (not TIFF, not ISOBMFF) with a fixed
// header, an offset directory, and three data regions: embedded JPEG preview,
// metadata container, and CFA (raw sensor data). Date extraction reads EXIF
// from the embedded JPEG. Hashing covers the CFA region only. Metadata is
// written via XMP sidecar.
//
// Container layout (Big Endian):
//
//	Offset  Size  Content
//	0x00    16    Magic: "FUJIFILMCCD-RAW " (ASCII, space-padded)
//	0x10    4     Format version (e.g., "0201")
//	0x14    8     Camera serial/ID
//	0x1C    32    Camera model name (null-terminated)
//	0x3C    4     RAF version string
//	        --- Offset directory (starting at byte 84) ---
//	0x54    4     JPEG preview offset
//	0x58    4     JPEG preview length
//	0x5C    4     Meta container offset
//	0x60    4     Meta container length
//	0x64    4     CFA (raw sensor data) offset
//	0x68    4     CFA (raw sensor data) length
//
// Magic bytes:
//
//	"FUJIFILMCCD-RAW " (16 bytes at offset 0). Fully distinctive — no
//	collision risk with any other format. Fits exactly within the registry's
//	16-byte magicReadSize.
//
// Metadata write:
//
//	No-op stub. RAF metadata is written via XMP sidecar by the pipeline.
package raf

import (
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

// RAF header constants. All offsets are into the fixed-layout RAF header.
// All multi-byte integers in the RAF container are big-endian.
const (
	rafMagic      = "FUJIFILMCCD-RAW " // 16 bytes at offset 0x00
	jpegOffsetPos = 0x54               // JPEG preview offset (uint32, big-endian)
	jpegLengthPos = 0x58               // JPEG preview length (uint32, big-endian)
	cfaOffsetPos  = 0x64               // CFA data offset (uint32, big-endian)
	cfaLengthPos  = 0x68               // CFA data length (uint32, big-endian)
	headerMinSize = 0x6C               // minimum bytes to read the full offset directory (108)
)

// Handler implements domain.FileTypeHandler for Fujifilm RAF RAW images.
type Handler struct{}

// New returns a new RAF Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".raf"}
}

// MagicBytes returns the RAF magic signature at offset 0.
// The 16-byte "FUJIFILMCCD-RAW " string is fully distinctive and fits
// exactly within the registry's 16-byte magicReadSize.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte(rafMagic)},
	}
}

// Detect returns true if the file has a .raf extension AND the first 16 bytes
// match the RAF magic string "FUJIFILMCCD-RAW ".
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".raf" {
		return false, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("raf: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	magic := make([]byte, len(rafMagic))
	if _, err := io.ReadFull(f, magic); err != nil {
		return false, nil // file too short
	}

	return string(magic) == rafMagic, nil
}

// ExtractDate reads the capture date from the EXIF metadata embedded in the
// RAF's JPEG preview section.
//
// Strategy:
//  1. Read the RAF header to locate the JPEG preview offset and length.
//  2. Seek to the JPEG offset and create a section reader bounded to the JPEG.
//  3. Decode EXIF from the JPEG stream using rwcarlsen/goexif.
//  4. Fallback chain: DateTimeOriginal → DateTime → anselsAdams.
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return anselsAdams, fmt.Errorf("raf: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	jpegOffset, jpegLength, err := readJPEGOffsets(f)
	if err != nil || jpegOffset == 0 || jpegLength == 0 {
		return anselsAdams, nil
	}

	sr := io.NewSectionReader(f, int64(jpegOffset), int64(jpegLength))
	x, err := rwexif.Decode(sr)
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

// HashableReader returns an io.ReadCloser scoped to the CFA (raw sensor data)
// region of the RAF file. The CFA offset and length are read from the RAF
// header's offset directory.
//
// This excludes the file header, embedded JPEG preview, and metadata container,
// ensuring that metadata edits (tagging, XMP sidecars) do not invalidate the
// content hash.
//
// Falls back to the full file if the CFA region cannot be determined.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("raf: open %q: %w", filePath, err)
	}

	cfaOffset, cfaLength, err := readCFAOffsets(f)
	if err != nil || cfaOffset == 0 || cfaLength == 0 {
		// Fallback: hash the full file.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("raf: seek %q: %w", filePath, err)
		}
		return f, nil
	}

	// Validate that the CFA region is within the file.
	fileSize, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("raf: seek end %q: %w", filePath, err)
	}
	if int64(cfaOffset) >= fileSize || int64(cfaOffset)+int64(cfaLength) > fileSize {
		// CFA region out of bounds — fall back to full file.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("raf: seek %q: %w", filePath, err)
		}
		return f, nil
	}

	sr := io.NewSectionReader(f, int64(cfaOffset), int64(cfaLength))
	return &sectionReadCloser{Reader: sr, Closer: f}, nil
}

// MetadataSupport declares that RAF uses XMP sidecar files for metadata.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// readHeader reads the first headerMinSize bytes of the RAF file and returns
// them as a slice. Returns an error if the file is too short.
func readHeader(r io.Reader) ([]byte, error) {
	buf := make([]byte, headerMinSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// readJPEGOffsets reads the RAF header and returns the JPEG preview offset
// and length from the offset directory. Returns zeros on any read error.
func readJPEGOffsets(r io.Reader) (offset, length uint32, err error) {
	buf, err := readHeader(r)
	if err != nil {
		return 0, 0, err
	}
	offset = binary.BigEndian.Uint32(buf[jpegOffsetPos:])
	length = binary.BigEndian.Uint32(buf[jpegLengthPos:])
	return offset, length, nil
}

// readCFAOffsets reads the RAF header and returns the CFA data offset and
// length from the offset directory. Returns zeros on any read error.
func readCFAOffsets(r io.Reader) (offset, length uint32, err error) {
	buf, err := readHeader(r)
	if err != nil {
		return 0, 0, err
	}
	offset = binary.BigEndian.Uint32(buf[cfaOffsetPos:])
	length = binary.BigEndian.Uint32(buf[cfaLengthPos:])
	return offset, length, nil
}

// sectionReadCloser wraps an io.Reader with a separate io.Closer.
// Used to return a bounded file section while keeping the underlying
// file open until the caller explicitly closes it.
type sectionReadCloser struct {
	Reader io.Reader
	Closer io.Closer
}

// Read implements io.Reader by reading from a bounded file section.
func (s *sectionReadCloser) Read(p []byte) (int, error) { return s.Reader.Read(p) }

// Close implements io.Closer by closing the underlying file.
func (s *sectionReadCloser) Close() error { return s.Closer.Close() }
