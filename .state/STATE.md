# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 50 | TIFF-RAW Shared Base — `internal/handler/tiffraw` | High | @developer | ⬜ Pending | 2, 7 | Shared `Base` struct: EXIF date extraction via TIFF IFD parsing, embedded JPEG preview extraction for `HashableReader`, no-op `WriteMetadataTags` |
| 51 | DNG Filetype Module — `internal/handler/dng` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; DNG-specific extensions, magic bytes (TIFF LE/BE), Detect with DNGVersion tag check |
| 52 | NEF Filetype Module — `internal/handler/nef` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.nef` extension, TIFF LE magic, extension-primary detection |
| 53 | CR2 Filetype Module — `internal/handler/cr2` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.cr2` extension, TIFF LE magic + `CR` at offset 8 |
| 54 | PEF Filetype Module — `internal/handler/pef` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.pef` extension, TIFF LE magic, extension-primary detection |
| 55 | ARW Filetype Module — `internal/handler/arw` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.arw` extension, TIFF LE magic, extension-primary detection |
| 56 | CR3 Filetype Module — `internal/handler/cr3` | High | @developer | ⬜ Pending | 12 | Standalone ISOBMFF-based handler; EXIF via ISOBMFF box extraction (like HEIC), JPEG preview from container, no-op write |
| 57 | RAW Handler Registration — Wire into CLI | High | @developer | ⬜ Pending | 51, 52, 53, 54, 55, 56 | Register all 6 RAW handlers + HEIC + MP4 in `cmd/sort.go`, `cmd/verify.go`, `cmd/resume.go` |
| 58 | RAW Handlers — Unit Tests | High | @tester | ⬜ Pending | 50, 51, 52, 53, 54, 55, 56 | Per-handler tests: Extensions, MagicBytes, Detect, ExtractDate fallback, HashableReader determinism, WriteMetadataTags no-op |
| 59 | RAW Handlers — Integration Tests | High | @tester | ⬜ Pending | 57, 58 | End-to-end sort with RAW fixture files, verify DB records, verify output naming with `.dng`/`.nef`/`.cr2`/`.cr3`/`.pef`/`.arw` extensions |
| 60 | Tests & Verification — Full Suite Green (RAW) | High | @tester | ⬜ Pending | 58, 59 | `go vet`, `go test -race ./...`, `make lint`, `go mod tidy` all pass with RAW handlers |

---

# Pixe Task Descriptions

## Task 50 — TIFF-RAW Shared Base — `internal/handler/tiffraw`

**Goal:** Create the shared base package that provides `ExtractDate`, `HashableReader`, and `WriteMetadataTags` for all TIFF-based RAW formats (DNG, NEF, CR2, PEF, ARW). Per-format handlers embed this struct and supply only their identity methods.

**Architecture Reference:** Section 6.4.1–6.4.7

**Depends on:** Task 2 (domain types), Task 7 (JPEG handler — establishes patterns)

**File to create: `internal/handler/tiffraw/tiffraw.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 ...

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
//   Parses the TIFF IFD chain to locate EXIF sub-IFDs, then reads
//   DateTimeOriginal (tag 0x9003) and DateTime (tag 0x0132) with the
//   standard fallback chain: DateTimeOriginal → DateTime → Ansel Adams date.
//
// Hashable region:
//   Extracts the embedded full-resolution JPEG preview image. TIFF-based
//   RAW files store this in a secondary IFD (often IFD1 or a sub-IFD) with
//   NewSubfileType = 0 (full-resolution) and Compression = 6 (JPEG).
//   The handler navigates the IFD chain, locates the JPEG strip/tile
//   offsets and byte counts, and returns a reader over that region.
//   Falls back to full-file hash if the JPEG preview cannot be extracted.
//
// Metadata write:
//   No-op stub. RAW files are archival originals — writing metadata into
//   proprietary containers risks corruption.
package tiffraw

import (
    "bytes"
    "encoding/binary"
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

const exifDateFormat = "2006:01:02 15:04:05"

// TIFF IFD tag IDs used for JPEG preview extraction.
const (
    tagNewSubfileType  = 0x00FE
    tagCompression     = 0x0103
    tagStripOffsets    = 0x0111
    tagStripByteCounts = 0x0117
    tagJPEGOffset      = 0x0201 // JPEGInterchangeFormat (IFD1 thumbnail)
    tagJPEGLength      = 0x0202 // JPEGInterchangeFormatLength
)

// Base provides shared logic for TIFF-based RAW formats.
// Per-format handlers embed this struct and supply their own
// Extensions(), MagicBytes(), and Detect() methods.
type Base struct{}

// ExtractDate reads the capture date from EXIF metadata embedded in the
// TIFF container. Fallback chain: DateTimeOriginal → DateTime → Ansel Adams.
func (b *Base) ExtractDate(filePath string) (time.Time, error) { ... }

// HashableReader returns an io.ReadCloser over the embedded full-resolution
// JPEG preview image. If the preview cannot be extracted, falls back to
// returning a reader over the entire file.
func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) { ... }

// WriteMetadataTags is a no-op for TIFF-based RAW formats.
func (b *Base) WriteMetadataTags(filePath string, tags domain.MetadataTags) error {
    // RAW metadata write not supported in pure Go.
    return nil
}
```

### Implementation Details

#### `ExtractDate` implementation

The TIFF container is a valid input for `rwcarlsen/goexif` — the same library used by the JPEG handler. RAW files that are TIFF-based can be decoded directly:

```go
func (b *Base) ExtractDate(filePath string) (time.Time, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return anselsAdams, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    x, err := rwexif.Decode(f)
    if err != nil {
        return anselsAdams, nil // No EXIF — use fallback
    }

    // 1. DateTimeOriginal
    if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    // 2. DateTime (IFD0)
    if tag, err := x.Get(rwexif.DateTime); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    return anselsAdams, nil
}
```

#### `HashableReader` implementation

This is the most complex method. It must:

1. Read the TIFF header to determine byte order (LE `II` or BE `MM`).
2. Navigate the IFD chain (IFD0 → IFD1 → sub-IFDs).
3. For each IFD, check for a JPEG preview:
   - **IFD1 thumbnail path:** Look for `JPEGInterchangeFormat` (tag 0x0201) and `JPEGInterchangeFormatLength` (tag 0x0202). This is the standard TIFF thumbnail location.
   - **Sub-IFD path:** Look for IFDs where `NewSubfileType = 0` (full-resolution) and `Compression = 6` (JPEG). Read `StripOffsets` (tag 0x0111) and `StripByteCounts` (tag 0x0117) to locate the JPEG data.
4. Select the **largest** JPEG preview found (by byte count) — this is the full-resolution preview.
5. Return an `io.ReadCloser` over that byte range using `io.SectionReader`.
6. If no JPEG preview is found, fall back to returning `os.Open(filePath)` (full file).

```go
// jpegPreview holds the offset and size of an embedded JPEG preview.
type jpegPreview struct {
    offset int64
    size   int64
}

func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
    }

    preview, err := findLargestJPEGPreview(f)
    if err != nil || preview == nil {
        // Fallback: hash the full file.
        if _, err := f.Seek(0, io.SeekStart); err != nil {
            _ = f.Close()
            return nil, fmt.Errorf("tiffraw: seek %q: %w", filePath, err)
        }
        return f, nil
    }

    // Return a reader scoped to the JPEG preview region.
    sr := io.NewSectionReader(f, preview.offset, preview.size)
    return &sectionReadCloser{Reader: sr, Closer: f}, nil
}

// sectionReadCloser wraps an io.Reader with a separate io.Closer,
// allowing the caller to close the underlying file when done reading
// a section of it.
type sectionReadCloser struct {
    Reader io.Reader
    Closer io.Closer
}

func (s *sectionReadCloser) Read(p []byte) (int, error) { return s.Reader.Read(p) }
func (s *sectionReadCloser) Close() error               { return s.Closer.Close() }

// findLargestJPEGPreview parses the TIFF IFD chain and returns the
// offset and size of the largest embedded JPEG preview image.
// Returns nil if no JPEG preview is found.
func findLargestJPEGPreview(r io.ReadSeeker) (*jpegPreview, error) { ... }
```

The `findLargestJPEGPreview` function must:
- Read bytes 0–1 to determine byte order (`II` = little-endian, `MM` = big-endian).
- Read bytes 2–3 to verify TIFF magic (`42` in the determined byte order).
- Read bytes 4–7 for the offset to IFD0.
- Walk each IFD: read the entry count (2 bytes), then each 12-byte IFD entry.
- For each IFD, collect tag values for `NewSubfileType`, `Compression`, `StripOffsets`, `StripByteCounts`, `JPEGInterchangeFormat`, `JPEGInterchangeFormatLength`.
- After processing all entries in an IFD, check if it contains a JPEG preview and record it.
- Read the 4-byte "next IFD offset" at the end of each IFD to continue the chain (0 = end).
- Also follow `SubIFDs` (tag 0x014A) pointers if present — some RAW formats store the full-res preview in a sub-IFD rather than IFD1.
- Return the largest preview found.

#### `fileExt` helper

Include the same `fileExt` helper used by other handlers (duplicated per package, consistent with existing pattern):

```go
func fileExt(path string) string {
    for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
        if path[i] == '.' {
            return path[i:]
        }
    }
    return ""
}
```

### Acceptance Criteria

- `Base.ExtractDate()` correctly reads `DateTimeOriginal` from a TIFF-based RAW file.
- `Base.ExtractDate()` falls back to `DateTime` when `DateTimeOriginal` is absent.
- `Base.ExtractDate()` returns Ansel Adams date when no EXIF is present.
- `Base.HashableReader()` returns the embedded JPEG preview bytes when present.
- `Base.HashableReader()` falls back to full-file reader when no JPEG preview is found.
- `Base.HashableReader()` returns a deterministic byte stream (same bytes on repeated calls).
- `Base.WriteMetadataTags()` is a no-op that returns nil.
- The `sectionReadCloser` properly closes the underlying file handle.
- `go build ./...` succeeds.

---

## Task 51 — DNG Filetype Module — `internal/handler/dng`

**Goal:** Create the DNG handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/dng/dng.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 ...

// Package dng implements the FileTypeHandler contract for Adobe DNG
// (Digital Negative) RAW images.
//
// DNG files are TIFF containers with standard EXIF IFDs. Date extraction,
// hashable region (embedded JPEG preview), and metadata write (no-op) are
// provided by the shared tiffraw.Base.
//
// Detection:
//   DNG files use the standard TIFF header (little-endian "II" 0x2A00 or
//   big-endian "MM" 0x002A). Since this header is shared with other
//   TIFF-based formats, the .dng extension is the primary discriminator.
//   Magic bytes confirm the file is a valid TIFF container.
package dng

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

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
    ext := strings.ToLower(fileExt(filePath))
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

func fileExt(path string) string {
    for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
        if path[i] == '.' {
            return path[i:]
        }
    }
    return ""
}
```

### Acceptance Criteria

- `Handler` satisfies `domain.FileTypeHandler` (compile-time check via `var _ domain.FileTypeHandler = (*Handler)(nil)`).
- `Extensions()` returns `[".dng"]`.
- `MagicBytes()` returns two signatures (TIFF LE and TIFF BE).
- `Detect()` returns true for `.dng` files with valid TIFF headers.
- `Detect()` returns false for `.dng` files with non-TIFF content.
- `Detect()` returns false for non-`.dng` extensions even with TIFF content.
- `ExtractDate()`, `HashableReader()`, `WriteMetadataTags()` are inherited from `tiffraw.Base`.
- `go build ./...` succeeds.

---

## Task 52 — NEF Filetype Module — `internal/handler/nef`

**Goal:** Create the Nikon NEF handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/nef/nef.go`**

```go
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
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".nef"}
}

func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
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
        return false, nil
    }
    return header[0] == 0x49 && header[1] == 0x49 &&
        header[2] == 0x2A && header[3] == 0x00, nil
}

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 51 but for `.nef` extension.
- Only TIFF LE magic (Nikon always uses little-endian).
- `go build ./...` succeeds.

---

## Task 53 — CR2 Filetype Module — `internal/handler/cr2`

**Goal:** Create the Canon CR2 handler as a thin wrapper around `tiffraw.Base`. CR2 has a unique detection advantage: it includes `CR` signature bytes at offset 8–9.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/cr2/cr2.go`**

```go
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

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

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

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 51 but for `.cr2` extension.
- `Detect()` checks both TIFF LE header AND `CR` at offset 8.
- `Detect()` returns false for a TIFF file without `CR` at offset 8 (even with `.cr2` extension).
- `go build ./...` succeeds.

---

## Task 54 — PEF Filetype Module — `internal/handler/pef`

**Goal:** Create the Pentax PEF handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/pef/pef.go`**

```go
// Package pef implements the FileTypeHandler contract for Pentax PEF RAW images.
//
// PEF files are TIFF containers with Pentax-specific maker note IFDs.
// The .pef extension is the primary discriminator. Magic bytes confirm
// the file is a valid TIFF container (little-endian only).
package pef

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".pef"}
}

func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".pef" {
        return false, nil
    }
    // ... same TIFF LE header check pattern as NEF ...
}

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 52 but for `.pef` extension.
- `go build ./...` succeeds.

---

## Task 55 — ARW Filetype Module — `internal/handler/arw`

**Goal:** Create the Sony ARW handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/arw/arw.go`**

```go
// Package arw implements the FileTypeHandler contract for Sony ARW RAW images.
//
// ARW files are TIFF containers with Sony-specific maker note IFDs.
// The .arw extension is the primary discriminator. Magic bytes confirm
// the file is a valid TIFF container (little-endian only).
package arw

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".arw"}
}

func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".arw" {
        return false, nil
    }
    // ... same TIFF LE header check pattern as NEF ...
}

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 52 but for `.arw` extension.
- `go build ./...` succeeds.

---

## Task 56 — CR3 Filetype Module — `internal/handler/cr3`

**Goal:** Create the Canon CR3 handler as a standalone ISOBMFF-based handler. CR3 does **not** embed `tiffraw.Base` — it uses the ISOBMFF container approach established by the HEIC handler.

**Architecture Reference:** Section 6.4.2, 6.4.5, 6.4.6, 6.4.8

**Depends on:** Task 12 (HEIC handler — establishes ISOBMFF patterns)

**File to create: `internal/handler/cr3/cr3.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 ...

// Package cr3 implements the FileTypeHandler contract for Canon CR3 RAW images.
//
// CR3 files use the ISOBMFF container format (like HEIC and MP4), not TIFF.
// This handler is standalone and does not use the tiffraw shared base.
//
// Date extraction:
//   CR3 stores EXIF metadata within the ISOBMFF box structure. The handler
//   parses the container to locate the EXIF blob, then uses rwcarlsen/goexif
//   for standard EXIF tag reading. Fallback chain: DateTimeOriginal →
//   DateTime → Ansel Adams date.
//
// Hashable region:
//   The embedded full-resolution JPEG preview is extracted from the ISOBMFF
//   container. Falls back to full-file hash if extraction fails.
//
// Magic bytes:
//   "ftyp" at offset 4 (same as HEIC/MP4). The ftyp brand "crx " (Canon
//   RAW X) distinguishes CR3 from other ISOBMFF formats. Detection checks
//   both the ftyp box type and the "crx " brand.
//
// Metadata write:
//   No-op stub.
package cr3

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    rwexif "github.com/rwcarlsen/goexif/exif"

    "github.com/cwlls/pixe-go/internal/domain"
)

var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

const exifDateFormat = "2006:01:02 15:04:05"

type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".cr3"}
}

// MagicBytes returns the ISOBMFF ftyp box signature at offset 4.
func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
    }
}

// Detect returns true if the file has a .cr3 extension AND contains the
// ISOBMFF "ftyp" box with "crx " brand.
func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".cr3" {
        return false, nil
    }
    f, err := os.Open(filePath)
    if err != nil {
        return false, fmt.Errorf("cr3: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    header := make([]byte, 12)
    if _, err := io.ReadFull(f, header); err != nil {
        return false, nil // file too short
    }
    // Check "ftyp" at offset 4.
    ftyp := header[4] == 0x66 && header[5] == 0x74 &&
        header[6] == 0x79 && header[7] == 0x70
    if !ftyp {
        return false, nil
    }
    // Check major brand "crx " at offset 8.
    brand := string(header[8:12])
    return brand == "crx ", nil
}
```

### ExtractDate implementation

CR3 stores EXIF data in a specific box path within the ISOBMFF container. The approach:

1. Parse the ISOBMFF box structure to find the EXIF data. CR3 typically stores EXIF in a `moov/uuid` box (Canon uses a UUID-type box) or within a `moov/meta/iinf` path.
2. Alternatively, some CR3 files embed a TIFF-structured EXIF blob that can be located by scanning for the TIFF header signature (`II*\0` or `MM\0*`) within specific boxes.
3. Once the raw EXIF bytes are extracted, parse with `rwexif.Decode()` and apply the standard fallback chain.

```go
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return anselsAdams, fmt.Errorf("cr3: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    exifBytes, err := extractCR3Exif(f)
    if err != nil || len(exifBytes) == 0 {
        return anselsAdams, nil
    }

    x, err := rwexif.Decode(bytes.NewReader(exifBytes))
    if err != nil {
        return anselsAdams, nil
    }

    // 1. DateTimeOriginal
    if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    // 2. DateTime (IFD0)
    if tag, err := x.Get(rwexif.DateTime); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    return anselsAdams, nil
}

// extractCR3Exif parses the ISOBMFF box structure to locate and extract
// the raw EXIF bytes from a CR3 file.
func extractCR3Exif(r io.ReadSeeker) ([]byte, error) { ... }
```

### HashableReader implementation

The embedded JPEG preview in CR3 is typically stored in a `moov/trak` structure or a dedicated preview track. The handler navigates the ISOBMFF box tree to find the largest JPEG blob:

```go
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("cr3: open %q: %w", filePath, err)
    }

    preview, err := findCR3JpegPreview(f)
    if err != nil || preview == nil {
        // Fallback: hash the full file.
        if _, err := f.Seek(0, io.SeekStart); err != nil {
            _ = f.Close()
            return nil, fmt.Errorf("cr3: seek %q: %w", filePath, err)
        }
        return f, nil
    }

    sr := io.NewSectionReader(f, preview.offset, preview.size)
    return &sectionReadCloser{Reader: sr, Closer: f}, nil
}

// findCR3JpegPreview navigates the ISOBMFF box structure to locate
// the embedded full-resolution JPEG preview.
func findCR3JpegPreview(r io.ReadSeeker) (*jpegPreview, error) { ... }
```

### WriteMetadataTags

```go
func (h *Handler) WriteMetadataTags(filePath string, tags domain.MetadataTags) error {
    // CR3 metadata write not supported in pure Go.
    return nil
}
```

### ISOBMFF Box Parsing

The CR3 handler needs a lightweight ISOBMFF box parser. This can be implemented inline (no external dependency needed) since the box structure is simple:

```go
// isobmffBox represents a single ISOBMFF box header.
type isobmffBox struct {
    boxType string // 4-char type code
    size    int64  // total box size including header
    offset  int64  // file offset where the box starts
    dataOff int64  // file offset where the box data starts (after header)
}

// readBox reads the next ISOBMFF box header from the current position.
func readBox(r io.ReadSeeker) (*isobmffBox, error) { ... }

// walkBoxes iterates over top-level boxes in the given range.
func walkBoxes(r io.ReadSeeker, start, end int64) ([]*isobmffBox, error) { ... }
```

### Acceptance Criteria

- `Handler` satisfies `domain.FileTypeHandler`.
- `Extensions()` returns `[".cr3"]`.
- `MagicBytes()` returns ftyp at offset 4.
- `Detect()` checks both ftyp box AND `crx ` brand.
- `Detect()` returns false for HEIC files (ftyp + `heic` brand).
- `Detect()` returns false for MP4 files (ftyp + `isom`/`mp41` brand).
- `ExtractDate()` extracts dates from CR3 EXIF via ISOBMFF parsing.
- `HashableReader()` extracts the embedded JPEG preview or falls back to full file.
- `WriteMetadataTags()` is a no-op.
- `go build ./...` succeeds.

---

## Task 57 — RAW Handler Registration — Wire into CLI

**Goal:** Register all 6 RAW handlers (plus the existing but unregistered HEIC and MP4 handlers) in the three CLI command files.

**Architecture Reference:** Section 6.4.9

**Depends on:** Tasks 51, 52, 53, 54, 55, 56

### Files to modify

#### `cmd/sort.go`

Add imports and registration calls:

```go
import (
    // ... existing imports ...
    heichandler "github.com/cwlls/pixe-go/internal/handler/heic"
    mp4handler  "github.com/cwlls/pixe-go/internal/handler/mp4"
    dnghandler  "github.com/cwlls/pixe-go/internal/handler/dng"
    nefhandler  "github.com/cwlls/pixe-go/internal/handler/nef"
    cr2handler  "github.com/cwlls/pixe-go/internal/handler/cr2"
    cr3handler  "github.com/cwlls/pixe-go/internal/handler/cr3"
    pefhandler  "github.com/cwlls/pixe-go/internal/handler/pef"
    arwhandler  "github.com/cwlls/pixe-go/internal/handler/arw"
)

// In the registry setup (after jpeghandler.New()):
reg.Register(jpeghandler.New())   // existing
reg.Register(heichandler.New())   // was implemented but not registered
reg.Register(mp4handler.New())    // was implemented but not registered
reg.Register(dnghandler.New())    // new
reg.Register(nefhandler.New())    // new
reg.Register(cr2handler.New())    // new
reg.Register(cr3handler.New())    // new
reg.Register(pefhandler.New())    // new
reg.Register(arwhandler.New())    // new
```

#### `cmd/verify.go`

Same imports and registration pattern.

#### `cmd/resume.go`

Same imports and registration pattern.

### Registration Order

JPEG must be registered first. The TIFF-based RAW handlers share magic bytes with each other but have distinct extensions, so their relative order doesn't matter for the extension-based fast path. CR3 shares the ftyp magic with HEIC and MP4, but again, extensions disambiguate.

### Acceptance Criteria

- All 9 handlers (JPEG + HEIC + MP4 + 6 RAW) are registered in `cmd/sort.go`, `cmd/verify.go`, and `cmd/resume.go`.
- JPEG is registered first in all three files.
- `go build ./...` succeeds.
- `./pixe sort --source <dir-with-raw-files> --dest <dirB> --dry-run` discovers and classifies RAW files correctly.

---

## Task 58 — RAW Handlers — Unit Tests

**Goal:** Comprehensive unit tests for the `tiffraw` base and all 6 RAW handler packages.

**Depends on:** Tasks 50, 51, 52, 53, 54, 55, 56

### Files to create

#### `internal/handler/tiffraw/tiffraw_test.go`

Test the shared base logic:

1. **`TestBase_ExtractDate_noEXIF_fallback`** — File with no EXIF returns Ansel Adams date.
2. **`TestBase_ExtractDate_withDateTimeOriginal`** — File with `DateTimeOriginal` returns correct date (requires a real or synthetic TIFF fixture with EXIF).
3. **`TestBase_HashableReader_fullFileFallback`** — File with no embedded JPEG preview returns full file content.
4. **`TestBase_HashableReader_deterministic`** — Two calls return identical bytes.
5. **`TestBase_WriteMetadataTags_noop`** — Returns nil, file unchanged.

**Test fixture strategy:** Create a `buildFakeTIFF(t, dir, name)` helper that writes a minimal valid TIFF file (8-byte header + minimal IFD0). For EXIF tests, create `buildTIFFWithEXIF(t, dir, name, dateStr)` that includes a DateTimeOriginal tag. For JPEG preview tests, create `buildTIFFWithJPEGPreview(t, dir, name)` that embeds a small JPEG blob in IFD1.

#### Per-format test files

Each format gets its own test file following the HEIC test pattern:

**`internal/handler/dng/dng_test.go`**
**`internal/handler/nef/nef_test.go`**
**`internal/handler/cr2/cr2_test.go`**
**`internal/handler/cr3/cr3_test.go`**
**`internal/handler/pef/pef_test.go`**
**`internal/handler/arw/arw_test.go`**

Each test file includes:

1. **`TestHandler_Extensions`** — Verify correct extensions returned.
2. **`TestHandler_MagicBytes`** — Verify correct magic signatures.
3. **`TestHandler_Detect_valid`** — Correct extension + correct magic → true.
4. **`TestHandler_Detect_wrongExtension`** — Correct magic but wrong extension → false.
5. **`TestHandler_Detect_wrongMagic`** — Correct extension but wrong magic → false.
6. **`TestHandler_ExtractDate_noEXIF_fallback`** — Falls back to Ansel Adams.
7. **`TestHandler_HashableReader_returnsData`** — Returns non-empty data.
8. **`TestHandler_HashableReader_deterministic`** — Two calls return identical bytes.
9. **`TestHandler_WriteMetadataTags_noop`** — No-op, file unchanged.

**CR2-specific additional test:**
10. **`TestHandler_Detect_tiffWithoutCR`** — TIFF LE header but no `CR` at offset 8 → false.

**CR3-specific additional tests:**
10. **`TestHandler_Detect_heicBrand`** — ftyp + `heic` brand → false.
11. **`TestHandler_Detect_mp4Brand`** — ftyp + `isom` brand → false.

**Test fixture helpers per format:**

- `buildFakeDNG(t, dir, name)` — minimal TIFF LE file with `.dng` extension
- `buildFakeNEF(t, dir, name)` — minimal TIFF LE file with `.nef` extension
- `buildFakeCR2(t, dir, name)` — TIFF LE + `CR` at offset 8 with `.cr2` extension
- `buildFakeCR3(t, dir, name)` — ISOBMFF ftyp box with `crx ` brand
- `buildFakePEF(t, dir, name)` — minimal TIFF LE file with `.pef` extension
- `buildFakeARW(t, dir, name)` — minimal TIFF LE file with `.arw` extension

### Acceptance Criteria

- All test cases pass across all 7 test files.
- Tests run with `-race` flag without data race warnings.
- Each handler satisfies `domain.FileTypeHandler` (compile-time interface check in each test file).
- Test fixtures are minimal synthetic files — no real camera RAW files checked into the repo.

---

## Task 59 — RAW Handlers — Integration Tests

**Goal:** End-to-end integration tests that exercise the full sort pipeline with RAW fixture files, verifying correct discovery, classification, date extraction, hashing, and output naming.

**Depends on:** Tasks 57, 58

**File to modify: `internal/integration/integration_test.go`**

### New test cases

1. **`TestIntegration_RAW_Discovery`** — Place fixture files for all 6 RAW formats in `dirA`. Run discovery with all handlers registered. Verify:
   - Each file is discovered (not skipped).
   - Each file is matched to the correct handler.
   - No files are misclassified.

2. **`TestIntegration_RAW_FullSort`** — Sort a mix of JPEG + RAW files. Verify:
   - RAW files are copied to `dirB` with correct `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` naming.
   - Extensions are preserved lowercase (`.dng`, `.nef`, `.cr2`, `.cr3`, `.pef`, `.arw`).
   - Database records show `status = "complete"` for all files.
   - Checksums are non-empty and deterministic.

3. **`TestIntegration_RAW_DuplicateDetection`** — Copy the same RAW file twice (different filenames, same content). Verify the second is routed to `duplicates/`.

4. **`TestIntegration_RAW_MixedWithJPEG`** — Sort a directory containing both JPEG and RAW files from the same camera (same dates). Verify both are sorted correctly into the same date directories.

5. **`TestIntegration_RAW_Verify`** — Sort RAW files, then run `pixe verify` on `dirB`. Verify all checksums match.

### Test fixture generation

Use the `buildFake*` helpers from Task 58 to create synthetic RAW files in `t.TempDir()`. These are minimal valid files — they won't produce real images but will exercise the full pipeline.

### Acceptance Criteria

- All 5 integration tests pass.
- Tests run with `-race` flag without data race warnings.
- RAW files flow through the complete pipeline: discover → extract → hash → copy → verify → complete.
- Output filenames use the correct lowercase extensions.
- Database records are correct for all RAW file types.

---

## Task 60 — Tests & Verification — Full Suite Green (RAW)

**Goal:** Verify the entire codebase compiles, passes all tests, and passes lint after adding RAW handler support.

**Depends on:** Tasks 58, 59

### Verification commands

```bash
go vet ./...                                    # No warnings
go build ./...                                  # Compiles cleanly
go test -race -timeout 120s ./...               # All tests pass
make lint                                       # 0 issues
go mod tidy                                     # No diff
```

### Specific checks

1. **All handlers registered:**
   ```bash
   # Each of these should appear in sort.go, verify.go, and resume.go:
   rg 'reg\.Register\(' cmd/sort.go cmd/verify.go cmd/resume.go
   # Expected: 9 registrations per file (jpeg, heic, mp4, dng, nef, cr2, cr3, pef, arw)
   ```

2. **Interface compliance:**
   ```bash
   # Each handler package should have a compile-time interface check:
   rg 'var _ domain.FileTypeHandler' internal/handler/
   ```

3. **No new dependencies beyond what's expected:**
   ```bash
   go mod tidy
   git diff go.mod go.sum
   # The only new dependency should be the TIFF parser if one was added.
   # rwcarlsen/goexif is already a dependency.
   ```

4. **Build smoke test:**
   ```bash
   make build
   # Create test files and verify discovery:
   mkdir -p /tmp/raw-test
   # (create minimal test files with correct headers)
   ./pixe sort --source /tmp/raw-test --dest /tmp/raw-archive --dry-run
   ```

### Acceptance Criteria

- `go vet ./...` — zero warnings.
- `go build ./...` — compiles cleanly.
- `go test -race -timeout 120s ./...` — all tests pass (unit + integration, including all prior tests).
- `make lint` — 0 issues.
- `go mod tidy` produces no diff.
- All 9 handlers are registered in all 3 CLI command files.
- No regressions in existing JPEG, HEIC, or MP4 handler tests.
