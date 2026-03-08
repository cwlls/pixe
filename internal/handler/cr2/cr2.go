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

// Package cr2 implements the FileTypeHandler contract for Canon CR2 RAW images.
//
// CR2 files are TIFF containers with Canon-specific extensions. Unlike other
// TIFF-based RAW formats, CR2 has a unique signature: the standard TIFF LE
// header at offset 0 followed by "CR" (0x43 0x52) at offset 8. This allows
// more reliable detection beyond just extension matching.
package cr2

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

// Handler implements domain.FileTypeHandler for Canon CR2 RAW images.
type Handler struct {
	tiffraw.Base
}

// New returns a new CR2 Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".cr2"}
}

// MagicBytes returns the CR2 magic signature.
// CR2 uses TIFF LE header (4 bytes) + IFD offset (4 bytes) + "CR" at offset 8.
// We declare the TIFF LE header at offset 0 for the registry's fast-path check.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
	}
}

// Detect returns true if the file has a .cr2 extension AND begins with
// the TIFF LE header AND has "CR" at offset 8.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileExt(filePath))
	if ext != ".cr2" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("cr2: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 10)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil // file too short
	}
	// TIFF LE header at offset 0.
	tiffLE := header[0] == 0x49 && header[1] == 0x49 &&
		header[2] == 0x2A && header[3] == 0x00
	// "CR" signature at offset 8.
	crSig := header[8] == 0x43 && header[9] == 0x52
	return tiffLE && crSig, nil
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
