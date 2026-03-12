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

// Package orf implements the FileTypeHandler contract for Olympus ORF RAW images.
//
// ORF files are TIFF containers with Olympus-specific maker note IFDs.
// The .orf extension is the primary discriminator. Magic bytes confirm
// the file is a valid ORF or TIFF container (little-endian only).
package orf

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

// Handler implements domain.FileTypeHandler for Olympus ORF RAW images.
type Handler struct {
	tiffraw.Base
}

// New returns a new ORF Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".orf"}
}

// MagicBytes returns the magic signatures for Olympus ORF files.
// ORF files use either the Olympus-specific "IIRO" header or the
// standard TIFF little-endian header depending on the camera model.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte{0x49, 0x49, 0x52, 0x4F}}, // IIRO Olympus ORF LE
		{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // Standard TIFF LE
	}
}

// Detect returns true if the file has a .orf extension AND begins with
// either the Olympus "IIRO" header or the standard TIFF little-endian header.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".orf" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("orf: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}

	// IIRO: Olympus ORF LE
	if header[0] == 0x49 && header[1] == 0x49 &&
		header[2] == 0x52 && header[3] == 0x4F {
		return true, nil
	}
	// Standard TIFF LE (some ORF variants)
	if header[0] == 0x49 && header[1] == 0x49 &&
		header[2] == 0x2A && header[3] == 0x00 {
		return true, nil
	}

	return false, nil
}
