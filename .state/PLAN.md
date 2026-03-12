# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Extract shared `fileutil.Ext()` and replace all 10 `fileExt` copies | high | @developer | [x] complete | — | HIGH-1 + HIGH-2: Fix Windows bug and eliminate duplication in one task |
| 2  | Replace `text/template` with `strings.ReplaceAll` for copyright rendering | high | @developer | [x] complete | — | CRITICAL-1: Eliminate template injection risk; also deduplicate (MED-2) |
| 3  | Add symlink detection to discovery walk | high | @developer | [x] complete | — | HIGH-3: Skip symlinks with logged reason |
| 4  | Add ledger write error tracking | high | @developer | [x] complete | — | HIGH-5: Track "ledger healthy" flag, warn on first failure |
| 5  | XML-escape XMP template values | high | @developer | [x] complete | — | MED-7: Prevent malformed XML from user-supplied copyright/owner strings |
| 6  | Add compile-time interface checks for JPEG, HEIC, MP4 handlers | medium | @developer | [x] complete | — | MED-1: Add `var _ domain.FileTypeHandler = (*Handler)(nil)` to 3 files |
| 7  | Stream JPEG SOS payload extraction | high | @developer | [x] complete | — | CRITICAL-2: Replace `os.ReadFile` with `io.ReadSeeker`-based streaming |
| 8  | Stream MP4 keyframe payload extraction | high | @developer | [x] complete | — | CRITICAL-3: Return `io.Reader` over keyframe sections instead of buffering all |
| 9  | Add `out.Sync()` before close in `copy.Execute` | medium | @developer | [x] complete | — | LOW-1: Durability guarantee for copy-then-verify |
| 10 | Document intermediate DB status updates as best-effort | medium | @developer | [x] complete | — | HIGH-4: Add code comments and update architecture doc |
| 11 | Refactor `scanFileWithSource` to reuse `scanFileRow` | medium | @developer | [x] complete | — | MED-6: Eliminate ~90 lines of duplicated scan logic |
| 12 | Pass capture date through `workerFinalResult` for sidecar display | low | @developer | [x] complete | — | MED-4: Fix cosmetic `time.Time{}` → year 1 in sidecar line output |
| 13 | Tests for all remediation tasks | high | @tester | [x] complete | 1–12 | Verify all fixes with targeted tests; run full suite |
| 14 | Run `make check` and verify clean build | high | @tester | [x] complete | 13 | Full lint + vet + test gate |
| 15 | Commit remediation changes | medium | @committer | [ ] pending | 14 | Single or grouped commits per logical change |

---

## Parallelization Strategy

Tasks are grouped into waves. All tasks within a wave can be executed in parallel.

**Wave 1 — Independent, no cross-file conflicts:**
- Task 1 (fileutil.Ext) — touches handler packages + discovery/registry
- Task 2 (copyright rendering) — touches pipeline/pipeline.go + tagging/tagging.go
- Task 3 (symlink detection) — touches discovery/walk.go
- Task 4 (ledger error tracking) — touches pipeline/pipeline.go + pipeline/worker.go + manifest/
- Task 5 (XMP escaping) — touches xmp/xmp.go
- Task 6 (interface checks) — touches jpeg/jpeg.go, heic/heic.go, mp4/mp4.go (header only)

> **Conflict note:** Tasks 2 and 4 both touch `pipeline/pipeline.go`. Task 2 modifies `renderCopyright` (lines 714–731) and its call site. Task 4 modifies `lw.WriteEntry` call sites (lines 254, 302, 334, 380). These are in different regions of the file and can be parallelized if the developer is careful, but sequential execution within the file is safer. Task 6 touches handler files at the top (adding a `var _` line) while Task 1 touches them at the bottom (replacing `fileExt`), so these can be parallel.

**Wave 2 — Streaming refactors (more complex, independent of each other):**
- Task 7 (JPEG streaming) — touches jpeg/jpeg.go
- Task 8 (MP4 streaming) — touches mp4/mp4.go

**Wave 3 — Smaller independent fixes:**
- Task 9 (copy.Sync) — touches copy/copy.go
- Task 10 (DB docs) — touches worker.go comments + architecture doc
- Task 11 (scan refactor) — touches archivedb/queries.go + archivedb/files.go
- Task 12 (sidecar display date) — touches pipeline/worker.go

**Wave 4 — Verification:**
- Task 13 (tests)
- Task 14 (full check)
- Task 15 (commit)

---

## Task Descriptions

### Task 1: Extract shared `fileutil.Ext()` and replace all 10 `fileExt` copies

**Audit refs:** HIGH-1 (Windows path separator bug), HIGH-2 (DRY violation across 10 files)

**Problem:** A custom `fileExt` function is copy-pasted across 10 files. It only checks for `/` as a path separator, breaking on Windows paths with dots in directory names (e.g., `C:\photos\no-ext-dir.backup\IMG_0001` would incorrectly return `.backup\IMG_0001`). The `.goreleaser.yaml` targets Windows, so this is a real bug.

**Implementation:**

1. Create `internal/fileutil/ext.go`:

```go
// Package fileutil provides shared file-path utilities used across handler
// and discovery packages.
package fileutil

import "path/filepath"

// Ext returns the file extension including the leading dot, or "" if none.
// It delegates to filepath.Ext, which correctly handles both Unix and Windows
// path separators.
func Ext(path string) string {
	return filepath.Ext(path)
}
```

2. Create `internal/fileutil/ext_test.go` with test cases:
   - Standard extension: `"/photos/IMG_0001.jpg"` → `".jpg"`
   - No extension: `"/photos/IMG_0001"` → `""`
   - Dot in directory (the Windows bug): `"/photos/dir.backup/IMG_0001"` → `""`
   - Windows path with dot dir: `"C:\\photos\\dir.backup\\IMG_0001"` → `""`
   - Multiple dots: `"/photos/file.tar.gz"` → `".gz"`
   - Hidden file: `"/photos/.hidden"` → `".hidden"` (matches `filepath.Ext` behavior)

3. In each of the 10 files, remove the local `fileExt` function and replace all call sites with `fileutil.Ext()`:

   **Files to modify:**
   - `internal/handler/jpeg/jpeg.go` (line 286–294) — remove `fileExt`, add import `"github.com/cwlls/pixe-go/internal/fileutil"`, replace calls
   - `internal/handler/heic/heic.go` (line 161+) — same
   - `internal/handler/mp4/mp4.go` (line 362+) — same
   - `internal/handler/cr3/cr3.go` (line 696+) — same
   - `internal/handler/dng/dng.go` (line 89+) — same
   - `internal/handler/nef/nef.go` (line 79+) — same
   - `internal/handler/cr2/cr2.go` (line 84+) — same
   - `internal/handler/pef/pef.go` (line 78+) — same
   - `internal/handler/arw/arw.go` (line 78+) — same
   - `internal/discovery/registry.go` (line 98+) — same

4. Add Apache 2.0 copyright header to new files.

**Verification:** `go build ./...` succeeds; `go test -race ./internal/fileutil/... ./internal/handler/... ./internal/discovery/...` passes; `make lint` clean.

---

### Task 2: Replace `text/template` with `strings.ReplaceAll` for copyright rendering

**Audit refs:** CRITICAL-1 (template injection risk), MED-2 (renderCopyright duplicated)

**Problem:** The `--copyright` flag value is parsed as a Go `text/template`, which is unnecessarily complex for a single `{{.Year}}` substitution. The function is also duplicated between `pipeline.renderCopyright` (unexported) and `tagging.RenderCopyright` (exported).

**Implementation:**

1. Modify `internal/tagging/tagging.go`:
   - Replace the `text/template` implementation with `strings.ReplaceAll`:
   ```go
   func RenderCopyright(tmplStr string, date time.Time) string {
       if tmplStr == "" {
           return ""
       }
       return strings.ReplaceAll(tmplStr, "{{.Year}}", strconv.Itoa(date.Year()))
   }
   ```
   - Remove the `copyrightData` struct.
   - Remove the `"text/template"` import (add `"strconv"` and `"strings"` if not already present).

2. Modify `internal/pipeline/pipeline.go`:
   - Delete the `copyrightData` struct (lines 715–717).
   - Delete the `renderCopyright` function (lines 720–731).
   - Replace all call sites of `renderCopyright(...)` with `tagging.RenderCopyright(...)`.
   - Add import for `"github.com/cwlls/pixe-go/internal/tagging"` if not already present.
   - Remove `"text/template"` import if no longer used.

3. Update tests in `internal/tagging/` to verify:
   - `RenderCopyright("Copyright {{.Year}}", date)` → `"Copyright 2021"` (for a 2021 date)
   - `RenderCopyright("No template", date)` → `"No template"` (passthrough)
   - `RenderCopyright("", date)` → `""` (empty guard)
   - `RenderCopyright("{{.Year}}-{{.Year}}", date)` → `"2021-2021"` (multiple occurrences)
   - `RenderCopyright("{{.Foo}}", date)` → `"{{.Foo}}"` (unknown placeholder preserved as-is — this is a behavior change from template, which would error; the new behavior is safer)

**Verification:** `go test -race ./internal/tagging/... ./internal/pipeline/...` passes.

---

### Task 3: Add symlink detection to discovery walk

**Audit ref:** HIGH-3 (no symlink handling)

**Problem:** `filepath.WalkDir` does not follow symlinks for entries within the walk, but the code does not explicitly detect or skip them. A symlink pointing outside `dirA` could cause files from unexpected locations to be opened and read for magic byte detection.

**Implementation:**

1. Modify `internal/discovery/walk.go` in the `WalkDir` callback, after the directory handling block and before file processing:

```go
// Skip symlinks — they could point outside dirA or create loops.
if d.Type()&fs.ModeSymlink != 0 {
    relPath, _ := filepath.Rel(dirA, path)
    if relPath == "" {
        relPath = name
    }
    result.Skipped = append(result.Skipped, SkippedFile{
        Path:    path,
        RelPath: relPath,
        Reason:  "symlink",
    })
    return nil
}
```

2. Add a test `TestWalk_skipsSymlinks` in `internal/discovery/walk_test.go`:
   - Create a temp dir with a regular file and a symlink to it.
   - Run `Walk` and verify the symlink appears in `Skipped` with reason `"symlink"`.
   - Verify the regular file is still discovered normally.

**Note:** `filepath.WalkDir` does NOT report symlinks as directories (even if they point to directories), so the symlink check should be placed in the file-handling section. However, `filepath.WalkDir` does NOT follow symlinks at all for non-root entries — it reports them with `d.Type()` including `fs.ModeSymlink`. The check is still valuable for explicit skip reporting.

**Verification:** `go test -race ./internal/discovery/...` passes.

---

### Task 4: Add ledger write error tracking

**Audit ref:** HIGH-5 (ledger write errors silently ignored)

**Problem:** All ~13 `lw.WriteEntry(...)` calls across `pipeline.go` and `worker.go` discard errors with `_ =`. If the ledger file becomes unwritable mid-run, no warning is emitted.

**Implementation:**

1. Add a `ledgerHealthy` tracking mechanism. The simplest approach: add a helper method to the pipeline or a wrapper around `LedgerWriter`:

   In `internal/manifest/manifest.go`, add a `SafeLedgerWriter` wrapper:

   ```go
   // SafeLedgerWriter wraps a LedgerWriter and tracks write health.
   // On the first write error, it logs a warning to the provided writer
   // and suppresses subsequent warnings. All write errors are still
   // silently absorbed — ledger failure is non-fatal.
   type SafeLedgerWriter struct {
       lw      *LedgerWriter
       out     io.Writer
       failed  bool
       mu      sync.Mutex // protects failed flag in concurrent mode
   }

   // NewSafeLedgerWriter wraps lw with error tracking. If lw is nil,
   // all writes are no-ops.
   func NewSafeLedgerWriter(lw *LedgerWriter, out io.Writer) *SafeLedgerWriter

   // WriteEntry delegates to the underlying LedgerWriter. On the first
   // error, a warning is printed to out.
   func (sw *SafeLedgerWriter) WriteEntry(entry domain.LedgerEntry)
   ```

2. In `internal/pipeline/pipeline.go` and `worker.go`:
   - Replace `lw *manifest.LedgerWriter` with `lw *manifest.SafeLedgerWriter` in the relevant function signatures or struct fields.
   - Replace all `_ = lw.WriteEntry(...)` with `lw.WriteEntry(...)` (the safe wrapper handles errors internally).
   - Construct the `SafeLedgerWriter` at pipeline initialization where the `LedgerWriter` is created.

3. Add test `TestSafeLedgerWriter_warnsOnFirstError`:
   - Create a `SafeLedgerWriter` with a writer that returns errors.
   - Write two entries; verify warning is emitted once to the output buffer.

**Verification:** `go test -race ./internal/manifest/... ./internal/pipeline/...` passes.

---

### Task 5: XML-escape XMP template values

**Audit ref:** MED-7 (XMP template does not escape user input)

**Problem:** The XMP template uses `text/template` which does not XML-escape values. A copyright string containing `<`, `>`, `&`, or `"` produces invalid XML. The merge path in `merge.go` correctly uses `xmlEscape()`, but the template generation path does not.

**Implementation:**

1. In `internal/xmp/xmp.go`, XML-escape the tag values before passing them to the template. The `xmlEscape` function already exists in `merge.go` — move it to a shared location within the package (or make it package-level if it isn't already):

   ```go
   // In the Generate function (or wherever the template is executed),
   // escape values before passing to the template:
   data := xmpData{
       Copyright:   xmlEscape(copyright),
       CameraOwner: xmlEscape(cameraOwner),
   }
   ```

2. Verify `xmlEscape` handles: `&` → `&amp;`, `<` → `&lt;`, `>` → `&gt;`, `"` → `&quot;`, `'` → `&apos;`.

3. Add test `TestGenerate_escapesXMLSpecialChars`:
   - Generate XMP with copyright `"© Smith & Jones <2024>"`.
   - Verify output contains `&amp;` and `&lt;` and `&gt;`.
   - Verify the output is valid XML (parse with `encoding/xml`).

**Verification:** `go test -race ./internal/xmp/...` passes.

---

### Task 6: Add compile-time interface checks for JPEG, HEIC, MP4 handlers

**Audit ref:** MED-1 (missing compile-time interface checks)

**Problem:** 6 of 9 handlers have `var _ domain.FileTypeHandler = (*Handler)(nil)` but JPEG, HEIC, and MP4 do not.

**Implementation:**

Add the following line near the top of each file (after imports, before the `Handler` struct definition), matching the convention used by the other handlers:

1. `internal/handler/jpeg/jpeg.go`: Add `var _ domain.FileTypeHandler = (*Handler)(nil)` after the import block.
2. `internal/handler/heic/heic.go`: Same.
3. `internal/handler/mp4/mp4.go`: Same.

Each file will need the `domain` import: `"github.com/cwlls/pixe-go/internal/domain"`. Check if it's already imported (it likely is, since the handlers return `domain.FileTypeHandler` types).

**Note:** Use the correct import alias if the file uses one. The handler files import domain directly (no alias).

**Verification:** `go build ./internal/handler/...` succeeds (the compile-time check is the test).

---

### Task 7: Stream JPEG SOS payload extraction

**Audit ref:** CRITICAL-2 (JPEG HashableReader loads entire file into memory)

**Problem:** `HashableReader` calls `os.ReadFile(filePath)` which loads the entire JPEG into memory. For large panoramic JPEGs (200+ MB) with concurrent workers, this causes excessive memory usage (N × max_file_size).

**Implementation:**

Refactor `internal/handler/jpeg/jpeg.go` to use streaming I/O:

1. Modify `HashableReader` to open the file and scan JPEG markers sequentially using an `io.ReadSeeker`:

```go
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("jpeg: open %q: %w", filePath, err)
    }

    sosOffset, err := findSOSOffset(f)
    if err != nil {
        f.Close()
        return nil, fmt.Errorf("jpeg: find SOS in %q: %w", filePath, err)
    }

    // Get file size for the SOS-to-EOF section.
    fi, err := f.Stat()
    if err != nil {
        f.Close()
        return nil, fmt.Errorf("jpeg: stat %q: %w", filePath, err)
    }

    // Return a SectionReader over the SOS-to-EOI range.
    // The SOS marker starts the entropy-coded segment which runs to EOI.
    sr := io.NewSectionReader(f, sosOffset, fi.Size()-sosOffset)
    return &fileReadCloser{Reader: sr, file: f}, nil
}
```

2. Implement `findSOSOffset(r io.ReadSeeker) (int64, error)` that reads JPEG marker headers sequentially:
   - Read 2-byte SOI marker (0xFF 0xD8).
   - Loop: read 2-byte marker, read 2-byte length, seek past the segment.
   - When SOS marker (0xFF 0xDA) is found, return the current offset.
   - This reads only marker headers (a few KB) regardless of file size.

3. Create a `fileReadCloser` struct that wraps an `io.Reader` and an `*os.File` for proper cleanup:

```go
type fileReadCloser struct {
    io.Reader
    file *os.File
}

func (frc *fileReadCloser) Close() error {
    return frc.file.Close()
}
```

4. The existing `extractSOSPayload(data []byte)` function can be kept for internal use or removed if no longer needed. If `ExtractDate` also uses it, consider whether it too should be refactored (lower priority — date extraction reads much less data).

5. Update tests to verify:
   - `HashableReader` returns the same checksum as the old implementation for test fixtures.
   - `HashableReader` works on the existing JPEG test fixtures.
   - The returned reader is properly closeable.

**Verification:** `go test -race ./internal/handler/jpeg/...` passes; checksums match previous behavior for all test fixtures.

---

### Task 8: Stream MP4 keyframe payload extraction

**Audit ref:** CRITICAL-3 (MP4 keyframe extraction loads all keyframes into memory)

**Problem:** `extractKeyframePayload` reads all keyframe data into a `bytes.Buffer`. For 4K video with many keyframes, this could be hundreds of megabytes.

**Implementation:**

Refactor `internal/handler/mp4/mp4.go` to return a streaming reader:

1. Instead of collecting all keyframe bytes into a `bytes.Buffer`, build a slice of `io.SectionReader` instances (one per keyframe) and combine them with `io.MultiReader`. This mirrors the pattern used by `tiffraw.multiSectionReader`.

```go
func extractKeyframePayload(f *os.File, stss *StssBox, sampleOffsets []sampleOffset) (io.Reader, error) {
    var readers []io.Reader
    for _, sampleNum := range stss.SampleNumber {
        idx := int(sampleNum) - 1
        if idx < 0 || idx >= len(sampleOffsets) {
            continue
        }
        so := sampleOffsets[idx]
        if so.size == 0 {
            continue
        }
        readers = append(readers, io.NewSectionReader(f, int64(so.offset), int64(so.size)))
    }
    if len(readers) == 0 {
        return strings.NewReader(""), nil
    }
    return io.MultiReader(readers...), nil
}
```

2. Update `HashableReader` to:
   - Open the file.
   - Call `extractKeyframePayload` to get the streaming reader.
   - Return a composite `ReadCloser` that reads from the `MultiReader` and closes the file.

3. Update the function signature from `([]byte, error)` to `(io.Reader, error)` and adjust all callers.

4. Verify checksums match the old implementation for test fixtures.

**Verification:** `go test -race ./internal/handler/mp4/...` passes; checksums match previous behavior.

---

### Task 9: Add `out.Sync()` before close in `copy.Execute`

**Audit ref:** LOW-1 (no sync before close)

**Problem:** After `io.CopyBuffer`, the temp file is closed without `Sync()`. On power failure, data may not be flushed to stable storage.

**Implementation:**

In `internal/copy/copy.go`, add `out.Sync()` before `out.Close()`:

```go
// Flush to stable storage before closing.
if err := out.Sync(); err != nil {
    _ = out.Close()
    return "", fmt.Errorf("copy: sync temp file %q: %w", tmpPath, err)
}

if err := out.Close(); err != nil {
    return "", fmt.Errorf("copy: close temp file %q: %w", tmpPath, err)
}
```

**Verification:** `go test -race ./internal/copy/...` passes. No behavioral change for tests — `Sync` is a no-op on most test filesystems.

---

### Task 10: Document intermediate DB status updates as best-effort

**Audit ref:** HIGH-4 (worker DB writes not fully serialized)

**Problem:** Workers call `db.UpdateFileStatus` directly for intermediate states, contradicting the architecture doc's claim that "coordinator goroutine owns DB writes." The `_ = db.UpdateFileStatus(...)` pattern silently drops errors.

**Implementation:**

1. Add code comments in `internal/pipeline/worker.go` at each `_ = db.UpdateFileStatus(...)` call site:

```go
// Best-effort intermediate status update. Workers write intermediate
// states (extracted, hashed) directly; the coordinator owns terminal
// states (complete, failed). Errors are intentionally discarded —
// intermediate tracking is observational, not required for correctness.
// The SQLite busy timeout (5s) handles contention from concurrent workers.
_ = db.UpdateFileStatus(item.fileID, "extracted", ...)
```

2. No code changes to behavior — this is documentation only.

**Verification:** `make lint` passes (no dead code warnings from the `_ =` pattern).

---

### Task 11: Refactor `scanFileWithSource` to reuse `scanFileRow`

**Audit ref:** MED-6 (~90 lines of duplicated scan logic)

**Problem:** `scanFileWithSource` in `queries.go` duplicates nearly all of `scanFileRow`'s logic from `files.go`, with the addition of one extra column (`run_source`).

**Implementation:**

1. In `internal/archivedb/files.go`, ensure `scanFileRow` accepts a `scanner` interface (it already does).

2. In `internal/archivedb/queries.go`, refactor `scanFileWithSource` to:
   - Use a wrapper scanner that scans the extra `run_source` column alongside the standard columns.
   - Or restructure the query to return the standard columns first, scan them with `scanFileRow`, then scan the extra column separately.

   The cleanest approach: modify the SQL query for `FileWithSource` to return `run_source` as the last column, then:

   ```go
   func scanFileWithSource(rows *sql.Rows) (*FileWithSource, error) {
       // Scan all standard columns + run_source in one pass.
       // We need a custom scanner since scanFileRow uses a scanner interface.
       fws := &FileWithSource{}
       
       // Scan standard FileRecord fields using shared helper.
       fr, runSource, err := scanFileRowWithExtra(rows)
       if err != nil {
           return nil, err
       }
       fws.FileRecord = *fr
       fws.RunSource = runSource
       return fws, nil
   }
   ```

   Alternatively, extract the post-scan null-field assignment logic into a shared helper function `assignNullFields(fr *FileRecord, nullFields ...)` that both `scanFileRow` and `scanFileWithSource` call.

3. Remove the duplicated `parseOptTime` closure and null-field assignment block from `scanFileWithSource`.

**Verification:** `go test -race ./internal/archivedb/...` passes; all existing query tests still pass.

---

### Task 12: Pass capture date through `workerFinalResult` for sidecar display

**Audit ref:** MED-4 (sidecar lines use `time.Time{}` for tag resolution)

**Problem:** When the coordinator emits sidecar lines for completed files in the concurrent path, it calls `resolveTags(opts.Config, time.Time{})` with a zero time, causing `{{.Year}}` to resolve to year 1.

**Implementation:**

1. In `internal/pipeline/worker.go`, add `CaptureDate time.Time` to the `workerFinalResult` struct (or whatever struct carries the worker's result back to the coordinator).

2. In the worker goroutine, populate `CaptureDate` from the extracted metadata before sending the result.

3. In the coordinator's result-handling code (where `emitSidecarLines` is called), use `result.CaptureDate` instead of `time.Time{}`:

```go
// Before:
resolveTags(opts.Config, time.Time{})

// After:
resolveTags(opts.Config, result.CaptureDate)
```

**Verification:** `go test -race ./internal/pipeline/...` passes. Verify with a test that the sidecar display line contains the correct year.

---

### Task 13: Tests for all remediation tasks

**Agent:** @tester

Run the full test suite to verify all changes:

```bash
make test          # unit tests
make test-all      # including integration
make lint          # golangci-lint
make vet           # go vet
```

Specific test areas to verify:
- `go test -race ./internal/fileutil/...` — new package
- `go test -race ./internal/handler/...` — fileExt replacement + streaming + interface checks
- `go test -race ./internal/discovery/...` — symlink handling + fileExt replacement
- `go test -race ./internal/tagging/...` — copyright rendering
- `go test -race ./internal/pipeline/...` — copyright dedup + ledger wrapper
- `go test -race ./internal/xmp/...` — XML escaping
- `go test -race ./internal/copy/...` — sync before close
- `go test -race ./internal/archivedb/...` — scan refactor
- `go test -race ./internal/manifest/...` — SafeLedgerWriter

---

### Task 14: Run `make check` and verify clean build

**Agent:** @tester

```bash
make check         # fmt-check + vet + unit tests
make build         # GoReleaser snapshot build
```

Verify:
- Zero lint warnings
- Zero vet warnings
- All tests pass with `-race`
- Binary builds successfully

---

### Task 15: Commit remediation changes

**Agent:** @committer

Group commits logically:
1. `fix: extract shared fileutil.Ext to fix Windows path bug and eliminate duplication` (Task 1)
2. `fix: replace text/template with string replacement for copyright rendering` (Task 2)
3. `fix: skip symlinks during discovery walk` (Task 3)
4. `fix: track and warn on ledger write failures` (Task 4)
5. `fix: XML-escape user input in XMP template generation` (Task 5)
6. `chore: add compile-time interface checks for JPEG, HEIC, MP4 handlers` (Task 6)
7. `perf: stream JPEG SOS payload extraction to reduce memory usage` (Task 7)
8. `perf: stream MP4 keyframe extraction to reduce memory usage` (Task 8)
9. `fix: sync temp file to disk before close in copy.Execute` (Task 9)
10. `docs: document intermediate DB status updates as best-effort` (Task 10)
11. `refactor: deduplicate scanFileWithSource by reusing scanFileRow` (Task 11)
12. `fix: pass capture date through worker result for correct sidecar display` (Task 12)
