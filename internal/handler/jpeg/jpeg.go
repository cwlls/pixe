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
//   - EXIF read: github.com/rwcarlsen/goexif — pure Go, handles DateTimeOriginal
//     and CreateDate from IFD0/ExifIFD.
//
// Hashable region:
//
//	The complete file contents. Destination files are byte-identical copies
//	of their source; metadata is expressed via XMP sidecar only.
package jpeg

import (
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
	ext := strings.ToLower(fileutil.Ext(filePath))
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

// HashableReader returns an io.ReadCloser over the complete file contents.
// Destination files are byte-identical copies of their source; the full-file
// hash ensures re-verification is always a simple open-and-hash operation.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("jpeg: open %q: %w", filePath, err)
	}
	return f, nil
}

// MetadataSupport declares that JPEG uses XMP sidecar files for metadata.
// Destination files are never modified — metadata is expressed exclusively
// via an accompanying .jpg.xmp sidecar.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}
