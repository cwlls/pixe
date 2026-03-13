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

// Package mp4 implements the FileTypeHandler contract for MP4/MOV video files.
//
// Package selection:
//   - Atom parsing: github.com/abema/go-mp4 — pure Go, atom-level access to
//     mvhd (creation date).
//
// Date extraction:
//
//	QuickTime/MP4 stores creation time as seconds since 1904-01-01 00:00:00 UTC
//	in the mvhd box. We convert this to a time.Time and apply the fallback chain.
//
// Hashable region:
//
//	The complete file contents. Destination files are byte-identical copies
//	of their source; metadata is expressed via XMP sidecar only.
//
// Magic bytes:
//
//	MP4/MOV files are ISOBMFF containers. The ftyp box starts at offset 0:
//	  bytes 0-3: box size (big-endian uint32)
//	  bytes 4-7: box type "ftyp" (0x66 0x74 0x79 0x70)
//	We match on "ftyp" at offset 4, same as HEIC.
//	MOV files may use "wide" or "mdat" as the first box — we also accept
//	"moov" at offset 4 as a secondary signature.
package mp4

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	mp4lib "github.com/abema/go-mp4"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/fileutil"
)

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

// mp4Epoch is the QuickTime/MP4 epoch: 1904-01-01 00:00:00 UTC.
var mp4Epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

// anselsAdams is the fallback date when no creation time can be extracted.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

// Handler implements domain.FileTypeHandler for MP4/MOV video files.
type Handler struct{}

// New returns a new MP4 Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".mp4", ".mov"}
}

// MagicBytes returns the ISOBMFF ftyp box signature at offset 4.
// Also accepts "moov" at offset 4 for bare MOV files without ftyp.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
		{Offset: 4, Bytes: []byte{0x6d, 0x6f, 0x6f, 0x76}}, // "moov"
		{Offset: 4, Bytes: []byte{0x77, 0x69, 0x64, 0x65}}, // "wide"
		{Offset: 4, Bytes: []byte{0x6d, 0x64, 0x61, 0x74}}, // "mdat"
	}
}

// Detect returns true if the file has a .mp4/.mov extension AND contains
// a recognised ISOBMFF box type at offset 4.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".mp4" && ext != ".mov" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("mp4: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil
	}

	boxType := string(header[4:8])
	switch boxType {
	case "ftyp", "moov", "wide", "mdat":
		return true, nil
	}
	return false, nil
}

// ExtractDate reads the creation time from the mvhd atom.
// Fallback chain: mvhd CreationTime → anselsAdams.
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return anselsAdams, fmt.Errorf("mp4: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	boxes, err := mp4lib.ExtractBoxWithPayload(f, nil, mp4lib.BoxPath{
		mp4lib.BoxTypeMoov(),
		mp4lib.BoxTypeMvhd(),
	})
	if err != nil || len(boxes) == 0 {
		return anselsAdams, nil
	}

	mvhd, ok := boxes[0].Payload.(*mp4lib.Mvhd)
	if !ok {
		return anselsAdams, nil
	}

	secs := mvhd.GetCreationTime()
	if secs == 0 {
		return anselsAdams, nil
	}

	t := mp4Epoch.Add(time.Duration(secs) * time.Second)
	return t.UTC(), nil
}

// HashableReader returns an io.ReadCloser over the complete file contents.
// Destination files are byte-identical copies of their source; the full-file
// hash ensures re-verification is always a simple open-and-hash operation.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("mp4: open %q: %w", filePath, err)
	}
	return f, nil
}

// MetadataSupport declares that MP4/MOV uses XMP sidecar files.
// Embedded udta atom writing may be added in a future enhancement.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}
