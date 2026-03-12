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

// Package dng implements the FileTypeHandler contract for Adobe DNG
// (Digital Negative) RAW images.
//
// DNG files are TIFF containers with standard EXIF IFDs. Date extraction,
// hashable region (embedded JPEG preview), and metadata write (no-op) are
// provided by the shared tiffraw.Base.
//
// Detection:
//
//	DNG files use the standard TIFF header (little-endian "II" 0x2A00 or
//	big-endian "MM" 0x002A). Since this header is shared with other
//	TIFF-based formats, the .dng extension is the primary discriminator.
//	Magic bytes confirm the file is a valid TIFF container.
package dng

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

// Handler implements domain.FileTypeHandler for DNG images.
type Handler struct {
	tiffraw.Base // embeds ExtractDate, HashableReader, WriteMetadataTags
}

// New returns a new DNG Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".dng"}
}

// MagicBytes returns the TIFF magic byte signatures.
// DNG uses standard TIFF headers — both little-endian and big-endian.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE ("II" + 42)
		{Offset: 0, Bytes: []byte{0x4D, 0x4D, 0x00, 0x2A}}, // TIFF BE ("MM" + 42)
	}
}

// Detect returns true if the file has a .dng extension AND begins with
// a valid TIFF header (little-endian or big-endian).
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".dng" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("dng: open %q: %w", filePath, err)
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
