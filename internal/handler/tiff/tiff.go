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

// Package tiff implements the FileTypeHandler contract for standalone TIFF images.
//
// TIFF (.tif, .tiff) files are produced by flatbed scanners, professional
// editing workflows (Photoshop, GIMP, Affinity Photo), scientific imaging
// instruments, and some medium/large-format digital cameras. TIFF is the
// base container format that all TIFF-based RAW formats (DNG, NEF, CR2,
// PEF, ARW, ORF, RW2) derive from, so the shared tiffraw.Base provides
// all format-agnostic logic: date extraction, hashable region identification
// (raw image data strips/tiles), and metadata write (no-op).
//
// Detection:
//
//	TIFF files use the standard TIFF header — either little-endian ("II" +
//	0x2A 0x00) or big-endian ("MM" + 0x00 0x2A). This header is shared with
//	DNG, NEF, PEF, ARW, and other TIFF-based RAW formats. The .tif/.tiff
//	extension is therefore the primary discriminator; magic bytes confirm
//	the file is a valid TIFF container.
//
//	Registration order: this handler must be registered AFTER all TIFF-based
//	RAW handlers in the discovery registry. The extension-based fast path
//	resolves first, so a .dng file is never offered to this handler. The
//	ordering constraint only matters for the magic-byte fallback path.
//
// Hashable region:
//
//	Delegated to tiffraw.Base, which navigates the TIFF IFD chain to locate
//	the primary image data strips/tiles and returns a reader over that region.
//	Falls back to full-file hash if the image data cannot be extracted.
//
// Metadata:
//
//	Declares MetadataSidecar. Writing metadata into arbitrary TIFF files
//	risks corruption of scanner originals and professionally edited masters.
//	XMP sidecar files are written alongside the destination copy instead.
package tiff

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/fileutil"
	"github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

// Handler implements domain.FileTypeHandler for standalone TIFF images.
type Handler struct {
	tiffraw.Base // embeds ExtractDate, HashableReader, MetadataSupport, WriteMetadataTags
}

// New returns a new TIFF Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".tif", ".tiff"}
}

// MagicBytes returns the TIFF magic byte signatures.
// TIFF supports both little-endian and big-endian byte orders.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE ("II" + 42)
		{Offset: 0, Bytes: []byte{0x4D, 0x4D, 0x00, 0x2A}}, // TIFF BE ("MM" + 42)
	}
}

// Detect returns true if the file has a .tif/.tiff extension AND begins with
// a valid TIFF header (little-endian or big-endian).
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".tif" && ext != ".tiff" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("tiff: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	// Check for TIFF LE or BE header.
	le := header[0] == 0x49 && header[1] == 0x49 && header[2] == 0x2A && header[3] == 0x00
	be := header[0] == 0x4D && header[1] == 0x4D && header[2] == 0x00 && header[3] == 0x2A
	return le || be, nil
}
