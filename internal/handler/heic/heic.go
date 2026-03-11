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

// Package heic implements the FileTypeHandler contract for HEIC/HEIF images.
//
// Package selection:
//   - EXIF read: github.com/dsoprea/go-heic-exif-extractor — pure Go, extracts
//     the raw EXIF blob from the ISOBMFF container; the blob is then parsed by
//     github.com/dsoprea/go-exif/v2.
//   - EXIF write: Not yet supported for HEIC. WriteMetadataTags is a no-op
//     pending a pure-Go HEIC EXIF write library. Files are still copied and
//     verified correctly; tags are simply not injected.
//
// Hashable region:
//
//	HEIC is an ISOBMFF container. The hashable region is defined as the full
//	file content minus the EXIF APP1 blob. In practice, because we cannot
//	easily strip only the EXIF from the mdat atom without a full HEIC encoder,
//	we hash the entire file. This is consistent with the principle that the
//	checksum identifies the media content — HEIC files from cameras are
//	typically not edited in place, so the full-file hash is stable.
//
// Magic bytes:
//
//	HEIC files are ISOBMFF containers. The ftyp box starts at offset 0:
//	  bytes 0-3: box size (big-endian uint32, variable)
//	  bytes 4-7: box type "ftyp" (0x66 0x74 0x79 0x70)
//	  bytes 8-11: major brand — "heic", "heix", "hevc", "hevx", or "mif1"
//	We match on the "ftyp" type bytes at offset 4.
package heic

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dsoprea/go-heic-exif-extractor"

	"github.com/cwlls/pixe-go/internal/domain"
)

// anselsAdams is the fallback date when no EXIF date can be extracted.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

const exifDateFormat = "2006:01:02 15:04:05"

// Handler implements domain.FileTypeHandler for HEIC/HEIF images.
type Handler struct{}

// New returns a new HEIC Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".heic", ".heif"}
}

// MagicBytes returns the ISOBMFF ftyp box signature.
// We match "ftyp" at offset 4 (after the 4-byte box size field).
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
	}
}

// Detect returns true if the file has a .heic/.heif extension AND contains
// the ISOBMFF "ftyp" box signature at offset 4.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileExt(filePath))
	if ext != ".heic" && ext != ".heif" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("heic: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	// Check "ftyp" at offset 4.
	return header[4] == 0x66 && header[5] == 0x74 && header[6] == 0x79 && header[7] == 0x70, nil
}

// ExtractDate reads the capture date from HEIC EXIF metadata.
// Fallback chain: DateTimeOriginal → CreateDate → anselsAdams.
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	heicParser := heicexif.NewHeicExifMediaParser()
	heicContext, err := heicParser.ParseFile(filePath)
	if err != nil {
		return anselsAdams, nil
	}

	heicExif := heicContext.(heicexif.HeicExifContext)
	rootIfd, _, err := heicExif.Exif()
	if err != nil {
		return anselsAdams, nil
	}

	dateTimeTags, err := rootIfd.FindTagWithName("DateTimeOriginal")
	if err == nil && len(dateTimeTags) > 0 {
		if s, _ := dateTimeTags[0].Format(); s != "" {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	dateTimeTags, err = rootIfd.FindTagWithName("DateTime")
	if err == nil && len(dateTimeTags) > 0 {
		if s, _ := dateTimeTags[0].Format(); s != "" {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t.UTC(), nil
			}
		}
	}

	return anselsAdams, nil
}

// HashableReader returns an io.ReadCloser over the full file contents.
// See package doc for rationale on full-file hashing for HEIC.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("heic: open %q: %w", filePath, err)
	}
	return f, nil
}

// MetadataSupport declares that HEIC uses XMP sidecar files.
// HEIC EXIF write is not yet supported in pure Go.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
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
