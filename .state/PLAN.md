# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add `MetadataCapability` type and `MetadataSupport()` to `FileTypeHandler` interface | High | Developer | [x] complete | — | Domain layer change; breaks all handler compilation until updated |
| 2 | Update `tiffraw.Base` with `MetadataSupport()` returning `MetadataSidecar` | High | Developer | [x] complete | 1 | Fixes DNG, NEF, CR2, PEF, ARW in one shot |
| 3 | Update standalone handlers (JPEG, HEIC, MP4, CR3) with `MetadataSupport()` | High | Developer | [x] complete | 1 | JPEG → Embed; HEIC, MP4, CR3 → Sidecar |
| 4 | Remove MP4 `WriteMetadataTags` stub implementation | High | Developer | [x] complete | 3 | Replace with no-op; remove dead udta comment |
| 5 | Create `internal/xmp/` package — XMP sidecar writer | High | Developer | [x] complete | 1 | New package; pure Go, no dependencies |
| 6 | Update `internal/tagging/` — add sidecar dispatch via `MetadataSupport()` | High | Developer | [x] complete | 1, 5 | Central routing: embed vs. sidecar vs. none |
| 7 | Update pipeline tagging stage (sequential path) | High | Developer | [x] complete | 6 | `pipeline.go` processFile |
| 8 | Update pipeline tagging stage (concurrent path) | High | Developer | [x] complete | 6 | `worker.go` runWorker |
| 9 | Update handler tests — replace `WriteMetadataTags_noop` with `MetadataSupport` tests | Medium | Developer | Pending | 2, 3 | 9 handler test files |
| 10 | Update `internal/tagging/` tests — cover sidecar dispatch | Medium | Developer | Pending | 6 | Mock handler with each capability value |
| 11 | Add `internal/xmp/` tests — XMP output validation | Medium | Developer | Pending | 5 | Template rendering, field omission, Adobe packet structure |
| 12 | Add pipeline integration test — sidecar written for RAW, embedded for JPEG | Medium | Developer | Pending | 7, 8 | End-to-end in `internal/integration/` |
| 13 | Run full test suite, lint, vet | High | Developer | Pending | 1–12 | `make check && make test-all` |

---

# Pixe Task Descriptions

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
