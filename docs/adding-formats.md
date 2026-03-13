---
title: Adding a New File Format
---

# Adding a New File Format

Pixe's format support is modular. Each format is an isolated package under `internal/handler/`. The core pipeline is format-agnostic — it processes files through the `FileTypeHandler` interface without knowing anything about JPEG, HEIC, or RAW internals. Adding a new format means implementing that interface and registering the handler in the CLI commands. The pipeline itself requires no changes.

---

## The `FileTypeHandler` interface

<!-- pixe:begin:interface -->
```go
// MetadataCapability declares how a handler supports metadata tagging.
type MetadataCapability int

const (
	// MetadataNone indicates the format cannot receive metadata at all.
	// The pipeline skips tagging entirely for this handler.
	MetadataNone MetadataCapability = iota

	// MetadataEmbed indicates the format supports safe in-file metadata writing.
	// The pipeline calls WriteMetadataTags to inject tags directly into the file.
	MetadataEmbed

	// MetadataSidecar indicates the format cannot safely embed metadata.
	// The pipeline writes an XMP sidecar file alongside the destination copy.
	MetadataSidecar
)

// FileTypeHandler is the contract every filetype module must implement.
// The core engine is format-agnostic and interacts with all media files
// exclusively through this interface.
//
// Detection strategy:
//  1. The registry performs an initial fast-path match on file extension
//     using Extensions().
//  2. Magic bytes are then read from the file header and compared against
//     MagicBytes() to confirm the type. If they do not match, the file
//     may be reclassified or flagged as unrecognized.
//
// Hashable region:
//
//	Each handler defines what constitutes the "media payload" for its
//	format — the bytes that are hashed and embedded in the output filename.
//	This region excludes metadata so that metadata edits (e.g. tagging)
//	do not invalidate the checksum.
type FileTypeHandler interface {
	// Detect returns true if this handler can process the given file.
	// Implementations should verify magic bytes at the file header after
	// the registry has already performed an extension-based pre-filter.
	Detect(filePath string) (bool, error)

	// ExtractDate returns the capture date/time from the file's metadata.
	// Each implementation defines its own format-appropriate fallback chain.
	// The global policy is: DateTimeOriginal → CreateDate → 1902-02-20 00:00:00 UTC
	// (Ansel Adams' birthday), making undated files immediately identifiable.
	ExtractDate(filePath string) (time.Time, error)

	// HashableReader returns an io.Reader scoped to the media payload only,
	// excluding all metadata. The core engine pipes this reader through the
	// configured hash algorithm. Callers are responsible for closing any
	// underlying file handles; implementations should return a reader that
	// holds an open file and document that the caller must close it.
	HashableReader(filePath string) (io.ReadCloser, error)

	// MetadataSupport declares this handler's metadata tagging capability.
	// The pipeline uses this to decide between embedded writes, XMP sidecar
	// generation, or skipping tagging entirely.
	MetadataSupport() MetadataCapability

	// WriteMetadataTags injects metadata tags directly into the file.
	// Only called when MetadataSupport() returns MetadataEmbed.
	// Must be a no-op when tags.IsEmpty() is true.
	WriteMetadataTags(filePath string, tags MetadataTags) error

	// Extensions returns the lowercase file extensions this handler claims,
	// used for the initial fast-path detection before magic byte verification.
	// Example: []string{".jpg", ".jpeg"}
	Extensions() []string

	// MagicBytes returns the byte signatures used to confirm file type.
	// Multiple signatures may be returned for formats with variant headers.
	MagicBytes() []MagicSignature
}
```
<!-- pixe:end:interface -->

**Method-by-method:**

- **`Extensions()`** — Return the lowercase extensions your format uses (e.g., `[]string{".webp"}`). Used for the fast-path extension check before magic-byte verification.
- **`MagicBytes()`** — Return the byte signatures at known offsets that uniquely identify your format. Used to confirm the file is actually what the extension claims.
- **`Detect()`** — Check the extension, then verify magic bytes. Return `true` if this handler should process the file.
- **`ExtractDate()`** — Parse the file's metadata to find the capture date. Apply the fallback chain: `DateTimeOriginal` → `CreateDate` → February 20, 1902.
- **`HashableReader()`** — Return a reader over the media payload only — the pixel data, sensor data, or video frames — not the metadata wrapper. This is what gets checksummed for deduplication and integrity verification.
- **`MetadataSupport()`** — Declare your capability. Use `MetadataEmbed` only if you can safely write tags directly into the file format. Use `MetadataSidecar` for most formats — the pipeline will write an XMP sidecar alongside the destination copy. Use `MetadataNone` if metadata association is meaningless for this format.
- **`WriteMetadataTags()`** — Implement in-file tag writing for `MetadataEmbed` handlers. For `MetadataSidecar` and `MetadataNone` handlers, implement as a no-op that returns `nil` (required for interface compliance, but never called by the pipeline).

---

## Step-by-step walkthrough

Using WEBP as a hypothetical example.

**1. Create the package**

```
internal/handler/webp/
├── webp.go
└── webp_test.go
```

**2. Define the handler struct and constructor**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...

// Package webp implements FileTypeHandler for WebP images.
package webp

import (
    "io"
    "time"

    "github.com/cwlls/pixe-go/internal/domain"
)

// Handler processes WebP image files.
type Handler struct{}

// New returns a new WebP handler.
func New() *Handler { return &Handler{} }
```

**3. Implement `Extensions()` and `MagicBytes()`**

```go
func (h *Handler) Extensions() []string {
    return []string{".webp"}
}

func (h *Handler) MagicBytes() []domain.MagicSignature {
    // WebP: RIFF at offset 0, WEBP at offset 8
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte("RIFF")},
        {Offset: 8, Bytes: []byte("WEBP")},
    }
}
```

**4. Implement `Detect()`**

```go
func (h *Handler) Detect(filePath string) (bool, error) {
    // Extension check is handled by the registry before Detect is called.
    // Here we verify the magic bytes.
    f, err := os.Open(filePath)
    if err != nil {
        return false, fmt.Errorf("webp: detect: %w", err)
    }
    defer f.Close()

    buf := make([]byte, 12)
    if _, err := io.ReadFull(f, buf); err != nil {
        return false, nil
    }
    return string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP", nil
}
```

**5. Implement `ExtractDate()`**

Parse EXIF from the WEBP container (WEBP embeds EXIF in an `EXIF` chunk). Apply the standard fallback chain:

```go
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
    // ... parse EXIF from WEBP EXIF chunk ...
    // Apply fallback chain: DateTimeOriginal → CreateDate → Ansel Adams date
    fallback := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
    // return parsed date, or fallback if no EXIF found
    return fallback, nil
}
```

**6. Implement `HashableReader()`**

Return a reader over the VP8/VP8L/VP8X image data payload, excluding the RIFF/WEBP header and EXIF chunk:

```go
func (h *Handler) HashableReader(filePath string) (io.Reader, error) {
    // ... locate and return reader over image data payload ...
    // If payload cannot be isolated, fall back to full file
    return os.Open(filePath)
}
```

**7. Implement `MetadataSupport()` and `WriteMetadataTags()`**

For a new format, default to `MetadataSidecar` — the pipeline will write an XMP sidecar alongside the destination copy:

```go
func (h *Handler) MetadataSupport() domain.MetadataCapability {
    return domain.MetadataSidecar
}

func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
    // Never called — pipeline checks MetadataSupport() first.
    return nil
}
```

**8. Register in CLI commands**

Add `reg.Register(webphandler.New())` to the handler registry in all four command files:

- `cmd/sort.go`
- `cmd/verify.go`
- `cmd/resume.go`
- `cmd/status.go`

---

## TIFF-based RAW shortcut

If your format is TIFF-based (DNG, NEF, CR2, PEF, ARW all are), embed `tiffraw.Base` instead of implementing `ExtractDate`, `HashableReader`, and `WriteMetadataTags` from scratch. The base provides the shared TIFF IFD traversal logic. You only need to supply `Extensions()`, `MagicBytes()`, and `Detect()`:

```go
package myfmt

import "github.com/cwlls/pixe-go/internal/handler/tiffraw"

type Handler struct{ tiffraw.Base }

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string { return []string{".myfmt"} }

func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

func (h *Handler) Detect(filePath string) (bool, error) {
    // ... extension + magic byte check ...
}
```

Reference the existing DNG, NEF, CR2, PEF, or ARW handlers in `internal/handler/` as templates — they are each ~50 lines.

---

## Testing conventions

- **No testify.** Use stdlib `testing` only. `t.Fatal`, `t.Errorf`, `t.Helper`.
- **`t.TempDir()`** for all filesystem tests — auto-cleaned after the test.
- **`t.Helper()`** on all helper functions.
- **`-race` always.** The Makefile sets this; run `go test -race ./internal/handler/webp/...`.
- **Test names:** `TestHandler_methodOrBehavior` (e.g., `TestHandler_detectValidWebP`, `TestHandler_extractDateFallback`).
- **Fixture files** in `testdata/` — small, minimal files that exercise the specific code path.
- **Table-driven tests** for multiple input cases using anonymous struct slices with `want`/`got` pattern.
