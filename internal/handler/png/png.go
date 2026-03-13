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

// Package png implements the FileTypeHandler contract for PNG images.
//
// PNG files use a chunk-based format. EXIF data may be present in an eXIf
// chunk (PNG 1.5+, adopted 2017) or date information in tEXt/iTXt chunks.
// The hashable region is the full file — PNG pixel data is compressed and
// interleaved with filter bytes, providing no clean data-only separation.
package png

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	rwexif "github.com/rwcarlsen/goexif/exif"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/fileutil"
)

// pngMagic is the 8-byte PNG file signature.
var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

// anselsAdams is the fallback date when no metadata date is found.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

// exifDateFormat is the EXIF date/time string format.
const exifDateFormat = "2006:01:02 15:04:05"

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

// Handler implements domain.FileTypeHandler for PNG images.
type Handler struct{}

// New returns a new PNG Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".png"}
}

// MagicBytes returns the PNG file signature.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: pngMagic},
	}
}

// Detect returns true if the file has a .png extension AND begins with
// the PNG file signature.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".png" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("png: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 8)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	for i, b := range pngMagic {
		if header[i] != b {
			return false, nil
		}
	}
	return true, nil
}

// ExtractDate extracts the capture date from the PNG file.
// It scans for an eXIf chunk first, then falls back to tEXt chunks
// with date-like keys, and finally to the Ansel Adams date.
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return anselsAdams, fmt.Errorf("png: open for date extraction %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	// Skip the 8-byte PNG signature.
	if _, err := f.Seek(8, io.SeekStart); err != nil {
		return anselsAdams, nil
	}

	// Scan chunks looking for eXIf or tEXt with date info.
	var textDate string
	for {
		// Read chunk length (4 bytes, big-endian) and type (4 bytes).
		var length uint32
		if err := binary.Read(f, binary.BigEndian, &length); err != nil {
			break // EOF or read error — done scanning
		}
		chunkType := make([]byte, 4)
		if _, err := io.ReadFull(f, chunkType); err != nil {
			break
		}
		ct := string(chunkType)

		if ct == "eXIf" && length > 0 {
			// eXIf chunk contains raw EXIF bytes.
			exifData := make([]byte, length)
			if _, err := io.ReadFull(f, exifData); err == nil {
				if t, parseErr := parseEXIFDate(exifData); parseErr == nil {
					return t, nil
				}
			}
			// Skip CRC (4 bytes).
			_, _ = f.Seek(4, io.SeekCurrent)
			continue
		}

		if ct == "tEXt" && length > 0 && length < 65536 {
			data := make([]byte, length)
			if _, err := io.ReadFull(f, data); err == nil {
				// tEXt format: keyword\0value
				if idx := bytes.IndexByte(data, 0); idx >= 0 {
					key := string(data[:idx])
					val := string(data[idx+1:])
					if strings.EqualFold(key, "Creation Time") && textDate == "" {
						textDate = val
					}
				}
			}
			// Skip CRC.
			_, _ = f.Seek(4, io.SeekCurrent)
			continue
		}

		// Skip chunk data + CRC (4 bytes).
		_, _ = f.Seek(int64(length)+4, io.SeekCurrent)
	}

	// Try parsing tEXt Creation Time.
	if textDate != "" {
		for _, layout := range []string{
			time.RFC1123,
			time.RFC1123Z,
			time.RFC3339,
			exifDateFormat,
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
		} {
			if t, err := time.Parse(layout, textDate); err == nil {
				return t, nil
			}
		}
	}

	return anselsAdams, nil
}

// parseEXIFDate parses EXIF bytes and extracts DateTimeOriginal or DateTime.
func parseEXIFDate(data []byte) (time.Time, error) {
	x, err := rwexif.Decode(bytes.NewReader(data))
	if err != nil {
		return time.Time{}, err
	}

	// Try DateTimeOriginal first.
	if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t, nil
			}
		}
	}

	// Fallback to DateTime.
	if tag, err := x.Get(rwexif.DateTime); err == nil {
		if s, err := tag.StringVal(); err == nil {
			if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("no date found in EXIF")
}

// HashableReader returns a reader over the entire PNG file.
// PNG has no separable data-only region — full-file hash is the safe choice.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("png: open for hashing %q: %w", filePath, err)
	}
	return f, nil
}

// MetadataSupport returns MetadataSidecar. Writing into PNG chunks is fragile
// and not widely supported by photo management tools.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op for PNG — metadata is written via XMP sidecar.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}
