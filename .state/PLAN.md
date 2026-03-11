# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add `MetadataCapability` type and `MetadataSupport()` to `FileTypeHandler` interface | High | Developer | Pending | — | Domain layer change; breaks all handler compilation until updated |
| 2 | Update `tiffraw.Base` with `MetadataSupport()` returning `MetadataSidecar` | High | Developer | Pending | 1 | Fixes DNG, NEF, CR2, PEF, ARW in one shot |
| 3 | Update standalone handlers (JPEG, HEIC, MP4, CR3) with `MetadataSupport()` | High | Developer | Pending | 1 | JPEG → Embed; HEIC, MP4, CR3 → Sidecar |
| 4 | Remove MP4 `WriteMetadataTags` stub implementation | High | Developer | Pending | 3 | Replace with no-op; remove dead udta comment |
| 5 | Create `internal/xmp/` package — XMP sidecar writer | High | Developer | Pending | 1 | New package; pure Go, no dependencies |
| 6 | Update `internal/tagging/` — add sidecar dispatch via `MetadataSupport()` | High | Developer | Pending | 1, 5 | Central routing: embed vs. sidecar vs. none |
| 7 | Update pipeline tagging stage (sequential path) | High | Developer | Pending | 6 | `pipeline.go` processFile |
| 8 | Update pipeline tagging stage (concurrent path) | High | Developer | Pending | 6 | `worker.go` runWorker |
| 9 | Update handler tests — replace `WriteMetadataTags_noop` with `MetadataSupport` tests | Medium | Developer | Pending | 2, 3 | 9 handler test files |
| 10 | Update `internal/tagging/` tests — cover sidecar dispatch | Medium | Developer | Pending | 6 | Mock handler with each capability value |
| 11 | Add `internal/xmp/` tests — XMP output validation | Medium | Developer | Pending | 5 | Template rendering, field omission, Adobe packet structure |
| 12 | Add pipeline integration test — sidecar written for RAW, embedded for JPEG | Medium | Developer | Pending | 7, 8 | End-to-end in `internal/integration/` |
| 13 | Run full test suite, lint, vet | High | Developer | Pending | 1–12 | `make check && make test-all` |

---

# Pixe Task Descriptions

## Task 1 — Add `MetadataCapability` type and `MetadataSupport()` to `FileTypeHandler` interface

**File:** `internal/domain/handler.go`

Add the `MetadataCapability` enum type and extend the `FileTypeHandler` interface with a `MetadataSupport()` method. This is the foundational change — every handler must implement this method before the project compiles again.

**Changes:**

1. Add the type and constants after the `MetadataTags` block (before the interface):

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
```

2. Add `MetadataSupport()` to the `FileTypeHandler` interface, between `HashableReader` and `WriteMetadataTags`:

```go
// MetadataSupport declares this handler's metadata tagging capability.
// The pipeline uses this to decide between embedded writes, XMP sidecar
// generation, or skipping tagging entirely.
MetadataSupport() MetadataCapability
```

3. Update the `WriteMetadataTags` doc comment to clarify it is only called for `MetadataEmbed` handlers:

```go
// WriteMetadataTags injects metadata tags directly into the file.
// Only called when MetadataSupport() returns MetadataEmbed.
// Must be a no-op when tags.IsEmpty() is true.
WriteMetadataTags(filePath string, tags MetadataTags) error
```

**Verification:** `go vet ./internal/domain/...` passes. All handler packages will fail to compile until Tasks 2–3 are complete — that's expected.

---

## Task 2 — Update `tiffraw.Base` with `MetadataSupport()`

**File:** `internal/handler/tiffraw/tiffraw.go`

Add `MetadataSupport()` to the `Base` struct. This single addition satisfies the interface for all 5 TIFF-based RAW handlers (DNG, NEF, CR2, PEF, ARW) via embedding.

**Changes:**

1. Add the method immediately before the existing `WriteMetadataTags`:

```go
// MetadataSupport declares that TIFF-based RAW formats use XMP sidecar
// files for metadata tagging. Writing into proprietary RAW containers
// risks corruption of archival originals.
func (b *Base) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}
```

2. Update the `WriteMetadataTags` comment to note it is retained for interface compliance but never called by the pipeline:

```go
// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (b *Base) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}
```

**Verification:** `go build ./internal/handler/dng/... ./internal/handler/nef/... ./internal/handler/cr2/... ./internal/handler/pef/... ./internal/handler/arw/...` — all five should compile.

---

## Task 3 — Update standalone handlers (JPEG, HEIC, MP4, CR3) with `MetadataSupport()`

**Files:**
- `internal/handler/jpeg/jpeg.go`
- `internal/handler/heic/heic.go`
- `internal/handler/mp4/mp4.go`
- `internal/handler/cr3/cr3.go`

Add `MetadataSupport()` to each handler.

### JPEG (`MetadataEmbed`)

Add before `WriteMetadataTags`:

```go
// MetadataSupport declares that JPEG supports safe in-file EXIF writing.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataEmbed
}
```

No changes to `WriteMetadataTags` — it remains the full EXIF implementation.

### HEIC (`MetadataSidecar`)

Add before `WriteMetadataTags`:

```go
// MetadataSupport declares that HEIC uses XMP sidecar files.
// HEIC EXIF write is not yet supported in pure Go.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}
```

Update `WriteMetadataTags` comment to match tiffraw pattern (retained for interface compliance, never called).

### MP4 (`MetadataSidecar`)

Add before `WriteMetadataTags`:

```go
// MetadataSupport declares that MP4/MOV uses XMP sidecar files.
// Embedded udta atom writing may be added in a future enhancement.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}
```

Update `WriteMetadataTags` comment to match tiffraw pattern.

### CR3 (`MetadataSidecar`)

Add before `WriteMetadataTags`:

```go
// MetadataSupport declares that CR3 uses XMP sidecar files.
// CR3 metadata write is not supported in pure Go.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}
```

Update `WriteMetadataTags` comment to match tiffraw pattern.

**Verification:** `go build ./internal/handler/...` — all 9 handler packages compile. The full project should now compile again.

---

## Task 4 — Remove MP4 `WriteMetadataTags` stub implementation

**File:** `internal/handler/mp4/mp4.go`

The existing `WriteMetadataTags` on the MP4 handler has a lengthy doc comment describing planned `udta/©cpy` and `udta/©own` atom writing. Since MP4 is now `MetadataSidecar` and the pipeline will never call `WriteMetadataTags` on it, simplify the method to a clean no-op matching the pattern used by tiffraw, HEIC, and CR3.

**Replace** the current method (lines ~167–177) with:

```go
// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}
```

**Verification:** `go build ./internal/handler/mp4/...`

---

## Task 5 — Create `internal/xmp/` package — XMP sidecar writer

**New files:**
- `internal/xmp/xmp.go`
- `internal/xmp/xmp_test.go` (Task 11)

This package is responsible for generating standards-compliant XMP sidecar files. It has no handler-specific knowledge — it receives `domain.MetadataTags` and a destination path, and writes the `.xmp` file.

### `internal/xmp/xmp.go`

```go
// Package xmp generates Adobe-compatible XMP sidecar files for media
// formats that cannot safely embed metadata. The sidecar follows the
// Adobe naming convention: <filename>.<ext>.xmp.
package xmp
```

**Public API:**

```go
// SidecarPath returns the XMP sidecar path for the given media file.
// It appends ".xmp" to the full filename (Adobe convention).
// Example: "/archive/2021/12-Dec/20211225_062223_abc123.arw"
//       → "/archive/2021/12-Dec/20211225_062223_abc123.arw.xmp"
func SidecarPath(mediaPath string) string {
	return mediaPath + ".xmp"
}

// WriteSidecar generates and writes an XMP sidecar file alongside the
// media file at mediaPath. The sidecar contains the provided metadata
// tags in a standards-compliant XMP packet.
//
// Returns nil if tags.IsEmpty() — no sidecar is written.
// Returns an error if the file cannot be created or written.
func WriteSidecar(mediaPath string, tags domain.MetadataTags) error
```

**Implementation details:**

- Use `text/template` to render the XMP packet. Define the template as a `const` string.
- The XMP template must include:
  - `<?xpacket begin="﻿" id="W5M0MpCehiHzreSzNTczkc9d"?>` header (BOM + standard UUID)
  - `<x:xmpmeta>` / `<rdf:RDF>` / `<rdf:Description>` wrapper
  - `dc:rights` with `rdf:Alt` / `rdf:li xml:lang="x-default"` — only if `Copyright != ""`
  - `xmpRights:Marked` set to `True` — only if `Copyright != ""`
  - `aux:OwnerName` — only if `CameraOwner != ""`
  - `<?xpacket end="w"?>` footer
- Namespace declarations on `rdf:Description`: `dc`, `xmpRights`, `aux` — only include namespaces for fields that are present.
- Write atomically: write to `<path>.tmp`, then `os.Rename` to final path. This prevents partial sidecar files on crash.
- File permissions: `0644`.

**Error wrapping:** `fmt.Errorf("xmp: write sidecar %q: %w", sidecarPath, err)`

**No external dependencies.** Pure stdlib (`text/template`, `os`, `fmt`, `path/filepath`).

---

## Task 6 — Update `internal/tagging/` — add sidecar dispatch via `MetadataSupport()`

**File:** `internal/tagging/tagging.go`

The `Apply` function currently delegates unconditionally to `handler.WriteMetadataTags`. It must now check `handler.MetadataSupport()` and route accordingly.

**Updated `Apply` function:**

```go
// Apply persists metadata tags for the file at destPath. The strategy
// depends on the handler's declared MetadataSupport capability:
//
//   - MetadataEmbed:   calls handler.WriteMetadataTags (in-file EXIF/atoms)
//   - MetadataSidecar: writes an XMP sidecar via xmp.WriteSidecar
//   - MetadataNone:    no-op, returns nil
//
// Returns nil immediately when tags.IsEmpty().
func Apply(destPath string, handler domain.FileTypeHandler, tags domain.MetadataTags) error {
	if tags.IsEmpty() {
		return nil
	}
	switch handler.MetadataSupport() {
	case domain.MetadataEmbed:
		if err := handler.WriteMetadataTags(destPath, tags); err != nil {
			return fmt.Errorf("tagging: embed metadata in %q: %w", destPath, err)
		}
	case domain.MetadataSidecar:
		if err := xmp.WriteSidecar(destPath, tags); err != nil {
			return fmt.Errorf("tagging: write sidecar for %q: %w", destPath, err)
		}
	case domain.MetadataNone:
		// No tagging for this format.
	}
	return nil
}
```

**New import:** `"github.com/cwlls/pixe-go/internal/xmp"`

**`RenderCopyright` is unchanged** — it remains a pure template renderer with no handler awareness.

---

## Task 7 — Update pipeline tagging stage (sequential path)

**File:** `internal/pipeline/pipeline.go` (lines ~387–401)

Replace the direct `handler.WriteMetadataTags` call with `tagging.Apply`, which now handles the embed/sidecar/none routing internally.

**Replace** the current tagging block:

```go
// --- Tag (optional) ---
tags := resolveTags(cfg, captureDate)
if !tags.IsEmpty() {
    if err := df.Handler.WriteMetadataTags(absDest, tags); err != nil {
        ...
    } else {
        ...
    }
}
```

**With:**

```go
// --- Tag (optional) ---
tags := resolveTags(cfg, captureDate)
if !tags.IsEmpty() {
    if err := tagging.Apply(absDest, df.Handler, tags); err != nil {
        if db != nil {
            _ = db.UpdateFileStatus(fileID, "tag_failed", archivedb.WithError(err.Error()))
        }
        _, _ = fmt.Fprintf(out, "  WARNING  tag failed for %s: %v\n", df.RelPath, err)
        // Tag failure is non-fatal: file is copied and verified.
    } else {
        if db != nil {
            _ = db.UpdateFileStatus(fileID, "tagged")
        }
    }
}
```

**New import:** `"github.com/cwlls/pixe-go/internal/tagging"` (if not already imported — check whether the pipeline currently imports it or duplicates the logic inline).

**Note:** The pipeline currently has its own `renderCopyright` and `resolveTags` functions that duplicate `tagging.RenderCopyright`. These can optionally be refactored to call `tagging.RenderCopyright` directly, but that is a cleanup — not required for correctness. The critical change is routing through `tagging.Apply`.

---

## Task 8 — Update pipeline tagging stage (concurrent path)

**File:** `internal/pipeline/worker.go` (lines ~509–523)

Same change as Task 7, but in the `runWorker` function.

**Replace** the current tagging block:

```go
// --- Tag ---
tags := resolveTags(opts.Config, captureDate)
if !tags.IsEmpty() {
    if err := item.df.Handler.WriteMetadataTags(assign.absDest, tags); err != nil {
        ...
    } else {
        ...
    }
}
```

**With:**

```go
// --- Tag ---
tags := resolveTags(opts.Config, captureDate)
if !tags.IsEmpty() {
    if err := tagging.Apply(assign.absDest, item.df.Handler, tags); err != nil {
        if db != nil {
            _ = db.UpdateFileStatus(item.fileID, "tag_failed", archivedb.WithError(err.Error()))
        }
        _, _ = fmt.Fprintf(out, "  WARNING  tag failed for %s: %v\n",
            item.df.RelPath, err)
    } else {
        if db != nil {
            _ = db.UpdateFileStatus(item.fileID, "tagged")
        }
    }
}
```

**New import:** `"github.com/cwlls/pixe-go/internal/tagging"` (same as Task 7).

---

## Task 9 — Update handler tests — replace `WriteMetadataTags_noop` with `MetadataSupport` tests

**Files (9 test files):**
- `internal/handler/tiffraw/tiffraw_test.go`
- `internal/handler/dng/dng_test.go`
- `internal/handler/nef/nef_test.go`
- `internal/handler/cr2/cr2_test.go`
- `internal/handler/cr3/cr3_test.go`
- `internal/handler/pef/pef_test.go`
- `internal/handler/arw/arw_test.go`
- `internal/handler/heic/heic_test.go`
- `internal/handler/mp4/mp4_test.go`

For each handler, add a `TestHandler_MetadataSupport` test that asserts the correct `MetadataCapability` value. The existing `TestHandler_WriteMetadataTags_noop` tests should be kept (they still verify the no-op contract) but their comments should be updated to note that the pipeline no longer calls this method directly.

**Pattern for sidecar handlers (all except JPEG):**

```go
func TestHandler_MetadataSupport(t *testing.T) {
	h := New()
	got := h.MetadataSupport()
	if got != domain.MetadataSidecar {
		t.Errorf("MetadataSupport() = %v, want MetadataSidecar", got)
	}
}
```

**Pattern for JPEG:**

```go
func TestHandler_MetadataSupport(t *testing.T) {
	h := New()
	got := h.MetadataSupport()
	if got != domain.MetadataEmbed {
		t.Errorf("MetadataSupport() = %v, want MetadataEmbed", got)
	}
}
```

Also add `MetadataSupport()` to the `mockHandler` in `internal/tagging/tagging_test.go` (needed for Task 10).

**Verification:** `go test -race -timeout 120s ./internal/handler/...`

---

## Task 10 — Update `internal/tagging/` tests — cover sidecar dispatch

**File:** `internal/tagging/tagging_test.go`

The existing `mockHandler` needs a `MetadataSupport()` method. Add new test cases that exercise all three capability branches.

**Update `mockHandler`:**

```go
type mockHandler struct {
	writeCalled     bool
	writeTags       domain.MetadataTags
	writeErr        error
	metadataSupport domain.MetadataCapability
}

func (m *mockHandler) MetadataSupport() domain.MetadataCapability {
	return m.metadataSupport
}
```

**New test cases:**

1. `TestApply_embed_callsWriteMetadataTags` — handler returns `MetadataEmbed`, verify `WriteMetadataTags` is called.
2. `TestApply_sidecar_writesXMPFile` — handler returns `MetadataSidecar`, verify `.xmp` file is created at the correct path with correct content. Use `t.TempDir()` to create a dummy media file, call `Apply`, then read and validate the sidecar.
3. `TestApply_sidecar_copyrightOnly` — only Copyright set, verify `aux:OwnerName` is absent from XMP.
4. `TestApply_sidecar_cameraOwnerOnly` — only CameraOwner set, verify `dc:rights` is absent from XMP.
5. `TestApply_none_skipsEverything` — handler returns `MetadataNone`, verify no file is written and no handler method is called.
6. `TestApply_sidecar_errorPropagation` — make the temp dir read-only, verify error is returned and wrapped.

**Verification:** `go test -race -timeout 120s ./internal/tagging/...`

---

## Task 11 — Add `internal/xmp/` tests — XMP output validation

**New file:** `internal/xmp/xmp_test.go`

**Test cases:**

1. `TestWriteSidecar_bothFields` — Copyright + CameraOwner set. Verify:
   - File created at `<mediaPath>.xmp`
   - Contains `<?xpacket begin=` header and `<?xpacket end="w"?>` footer
   - Contains `dc:rights` with correct copyright text
   - Contains `xmpRights:Marked` = `True`
   - Contains `aux:OwnerName` with correct owner text
   - Valid XML (parse with `encoding/xml`)

2. `TestWriteSidecar_copyrightOnly` — Only Copyright set. Verify `aux:OwnerName` is absent, `dc:rights` is present.

3. `TestWriteSidecar_cameraOwnerOnly` — Only CameraOwner set. Verify `dc:rights` and `xmpRights:Marked` are absent, `aux:OwnerName` is present.

4. `TestWriteSidecar_emptyTags_noFile` — Both fields empty. Verify no file is created.

5. `TestSidecarPath` — Table-driven: verify `SidecarPath` appends `.xmp` correctly for various extensions (`.arw`, `.dng`, `.mp4`, `.heic`).

6. `TestWriteSidecar_atomicWrite` — Verify no `.tmp` file remains after successful write.

7. `TestWriteSidecar_errorOnBadPath` — Pass a non-existent directory, verify error is returned with `"xmp:"` prefix.

**Verification:** `go test -race -timeout 120s ./internal/xmp/...`

---

## Task 12 — Add pipeline integration test — sidecar written for RAW, embedded for JPEG

**File:** `internal/integration/sidecar_test.go` (new file in existing integration test directory)

End-to-end test that runs the pipeline with `--copyright` and `--camera-owner` configured, using fixture files for at least one JPEG and one RAW format (e.g., DNG or use a minimal synthetic TIFF-based file).

**Verify:**
- JPEG destination file has EXIF Copyright and CameraOwnerName tags embedded (read back with the EXIF library).
- JPEG destination has **no** `.xmp` sidecar.
- RAW destination file is unmodified (byte-identical to source after copy).
- RAW destination has a `.xmp` sidecar with correct content.
- Sidecar follows the Adobe naming convention (`<filename>.<ext>.xmp`).
- Database status for both files is `"tagged"` (not `"tag_failed"`).

**Verification:** `go test -race -timeout 120s ./internal/integration/ -run TestSidecar`

---

## Task 13 — Run full test suite, lint, vet

Run the complete validation suite to confirm nothing is broken:

```bash
make check        # fmt-check + vet + unit tests
make test-all     # all tests including integration
make lint         # golangci-lint
```

All must pass green. Fix any issues found and re-run.
