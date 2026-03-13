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
//	The complete file contents. Destination files are byte-identical copies
//	of their source; metadata is expressed via XMP sidecar only.
//
// Metadata write:
//
//	No-op stub. RAW files are archival originals — writing metadata into
//	proprietary containers risks corruption.
package tiffraw

import (
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

// HashableReader returns an io.ReadCloser over the complete file contents.
// Destination files are byte-identical copies of their source; the full-file
// hash ensures re-verification is always a simple open-and-hash operation.
func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
	}
	return f, nil
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
