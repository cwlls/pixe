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

// Package nef implements the FileTypeHandler contract for Nikon NEF RAW images.
//
// NEF files are TIFF containers with Nikon-specific maker note IFDs.
// The .nef extension is the primary discriminator. Magic bytes confirm
// the file is a valid TIFF container (little-endian only — Nikon always
// uses LE byte order).
package nef

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

// Handler implements domain.FileTypeHandler for Nikon NEF RAW images.
type Handler struct {
	tiffraw.Base
}

// New returns a new NEF Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".nef"}
}

// MagicBytes returns the TIFF LE magic signature.
// Nikon NEF files always use little-endian byte order.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
	}
}

// Detect returns true if the file has a .nef extension AND begins with
// the TIFF little-endian header.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".nef" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("nef: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	return header[0] == 0x49 && header[1] == 0x49 &&
		header[2] == 0x2A && header[3] == 0x00, nil
}
