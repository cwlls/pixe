# Implementation — Test Suite Audit Remediation

Source: `analysis/test_suite_audit_20260312.md` (Grade: B+ → target: A)

## Task Summary

| #   | Task                                                  | Priority | Agent      | Status | Depends On | Notes                                    |
|:----|:------------------------------------------------------|:---------|:-----------|:-------|:-----------|:-----------------------------------------|
| 1   | Add `t.Helper()` to 5 `buildFake*` functions          | High     | @developer | [x]    | —          | Trivial one-line fix per file            |
| 2   | Create shared `handlertest` suite package              | High     | @developer | [x]    | —          | New `internal/handler/handlertest/`      |
| 3   | Refactor TIFF-based handler tests to use shared suite  | High     | @developer | [x]    | 2          | DNG, NEF, CR2, ARW, PEF — ~900 lines removed |
| 4   | Add `internal/verify/` test file                       | Critical | @developer | [x]    | —          | 0% → ~80%+ coverage                     |
| 5   | Add `t.Parallel()` to pure-function packages           | High     | @developer | [x]    | —          | `pathbuilder`, `hash`, `domain`, `fileutil`, `xmp` |
| 6   | Add `t.Parallel()` to handler tests                    | High     | @developer | [x]    | 3          | All 10 handler packages                 |
| 7   | Add adversarial filename tests                         | Medium   | @developer | [x]    | 4          | `parseChecksum` + `pathbuilder`          |
| 8   | Increase `handler/mp4` coverage (32% → 70%+)           | Medium   | @developer | [x]    | —          | Truncated files, missing atoms, streaming |
| 9   | Increase `handler/jpeg` coverage (37.6% → 70%+)        | Medium   | @developer | [x]    | —          | Corrupt EXIF, missing JFIF, truncated    |
| 10  | Add concurrent DB access tests to `archivedb`          | Medium   | @developer | [x]    | —          | Concurrent read/write, deadlock detection |
| 11  | Run full test suite and verify green                   | High     | @tester    | [x]    | 1–10       | `make test-all` with `-race`             |
| 12  | Commit all changes                                     | —        | @committer | [~]    | 11         | Single or grouped commits                |

---

## Parallelization Strategy

Tasks are grouped into waves. All tasks within a wave can be executed in parallel.

### Wave 1 — Independent, no cross-dependencies
- **Task 1**: Add `t.Helper()` to `buildFake*` functions
- **Task 2**: Create shared `handlertest` suite package
- **Task 4**: Add `internal/verify/` tests
- **Task 5**: Add `t.Parallel()` to pure-function packages
- **Task 8**: Increase `handler/mp4` coverage
- **Task 9**: Increase `handler/jpeg` coverage
- **Task 10**: Add concurrent DB access tests to `archivedb`

### Wave 2 — Depends on Wave 1 outputs
- **Task 3**: Refactor TIFF handler tests → shared suite (depends on Task 2)
- **Task 6**: Add `t.Parallel()` to handler tests (depends on Task 3)
- **Task 7**: Add adversarial filename tests (depends on Task 4 for `parseChecksum` context)

### Wave 3 — Validation
- **Task 11**: Full test suite verification (depends on all above)

### Wave 4 — Ship
- **Task 12**: Commit (depends on Task 11)

---

## Task Descriptions

### Task 1 — Add `t.Helper()` to 5 `buildFake*` functions

**Priority:** High | **Effort:** Trivial | **Agent:** @developer

Add `t.Helper()` as the first line of each of these test helpers so failure line numbers point to the calling test, not the helper:

| File | Function |
|:-----|:---------|
| `internal/handler/nef/nef_test.go` | `buildFakeNEF(t *testing.T, dir, name string)` |
| `internal/handler/cr2/cr2_test.go` | `buildFakeCR2(t *testing.T, dir, name string)` |
| `internal/handler/pef/pef_test.go` | `buildFakePEF(t *testing.T, dir, name string)` |
| `internal/handler/dng/dng_test.go` | `buildFakeDNG(t *testing.T, dir, name string)` |
| `internal/handler/arw/arw_test.go` | `buildFakeARW(t *testing.T, dir, name string)` |

Pattern — add as first line of each function body:
```go
t.Helper()
```

**Verification:** `make test` passes; `grep -r 't.Helper()' internal/handler/` shows 5 new occurrences.

---

### Task 2 — Create shared `handlertest` suite package

**Priority:** High | **Effort:** Medium | **Agent:** @developer

Create `internal/handler/handlertest/suite.go` — a reusable test harness that exercises the 10 standard behaviors shared by all TIFF-based handlers. This eliminates ~900 lines of duplication across 5 handler test files.

**Package:** `internal/handler/handlertest`

**File:** `suite.go`

```go
// Package handlertest provides a shared test suite for FileTypeHandler
// implementations that delegate to tiffraw.Base. Each handler test file
// calls RunSuite with handler-specific configuration to exercise the
// standard 10 behaviors without duplicating test logic.
package handlertest
```

**Key type:**

```go
// SuiteConfig configures the shared handler test suite.
type SuiteConfig struct {
    // NewHandler returns a fresh handler instance.
    NewHandler func() domain.FileTypeHandler

    // Extensions is the expected return value of handler.Extensions().
    Extensions []string

    // MagicSignatures is the expected return value of handler.MagicBytes().
    MagicSignatures []domain.MagicSignature

    // BuildFakeFile writes a minimal valid file for this format into dir
    // with the given name and returns the absolute path.
    BuildFakeFile func(t *testing.T, dir, name string) string

    // WrongExtension is a filename with an incorrect extension for this handler
    // (e.g., "test.jpg" for a DNG handler). Used in Detect_wrongExtension test.
    WrongExtension string

    // MetadataCapability is the expected MetadataSupport() return value.
    MetadataCapability domain.MetadataCapability
}
```

**Key function:**

```go
// RunSuite runs the standard 10-test handler suite against the provided config.
// It should be called from each handler's test file.
func RunSuite(t *testing.T, cfg SuiteConfig)
```

The 10 subtests (run via `t.Run`):
1. `Extensions` — `handler.Extensions()` matches `cfg.Extensions`
2. `MagicBytes` — `handler.MagicBytes()` matches `cfg.MagicSignatures`
3. `Detect/valid` — `Detect(validFile)` returns `true, nil`
4. `Detect/wrongExtension` — `Detect(wrongExtFile)` returns `false, nil`
5. `Detect/wrongMagic` — `Detect(wrongMagicFile)` returns `false, nil`
6. `ExtractDate/noEXIF_fallback` — returns Ansel Adams date `1902-02-20T00:00:00Z`
7. `HashableReader/returnsData` — non-empty `io.ReadAll`
8. `HashableReader/deterministic` — two calls return `bytes.Equal` data
9. `MetadataSupport` — matches `cfg.MetadataCapability`
10. `WriteMetadataTags/noop` — no error returned

Each subtest should call `t.Parallel()` where safe (all use `t.TempDir()`).

**Verification:** Package compiles: `go build ./internal/handler/handlertest/...`

---

### Task 3 — Refactor TIFF-based handler tests to use shared suite

**Priority:** High | **Effort:** Medium | **Agent:** @developer  
**Depends on:** Task 2

Replace the body of each of these 5 test files with a single `TestHandler` function that calls `handlertest.RunSuite(t, cfg)`:

| File | Handler | Lines before | Lines after (approx) |
|:-----|:--------|:-------------|:---------------------|
| `internal/handler/dng/dng_test.go` | `dng.New()` | 222 | ~50 |
| `internal/handler/nef/nef_test.go` | `nef.New()` | 217 | ~50 |
| `internal/handler/arw/arw_test.go` | `arw.New()` | 217 | ~50 |
| `internal/handler/pef/pef_test.go` | `pef.New()` | 217 | ~50 |
| `internal/handler/cr2/cr2_test.go` | `cr2.New()` | 252 | ~50 |

Each refactored file should:
1. Keep the Apache 2.0 copyright header.
2. Keep the compile-time interface check: `var _ domain.FileTypeHandler = (*Handler)(nil)`.
3. Move the `buildFake*` helper into the `SuiteConfig.BuildFakeFile` field (inline closure or keep as a local function — developer's choice).
4. Call `handlertest.RunSuite(t, handlertest.SuiteConfig{...})`.

**CR2 note:** CR2 has 252 lines (vs ~217 for others) — check if it has extra tests beyond the standard 10. If so, keep those as additional test functions alongside the suite call.

**Verification:** `go test -race -timeout 120s ./internal/handler/dng/... ./internal/handler/nef/... ./internal/handler/arw/... ./internal/handler/pef/... ./internal/handler/cr2/...` — all pass, same test count (10 per handler).

---

### Task 4 — Add `internal/verify/` test file

**Priority:** Critical | **Effort:** Medium | **Agent:** @developer

Create `internal/verify/verify_test.go` — the single biggest coverage gap in the suite. Target: 80%+ coverage of `verify.go` (256 lines).

**Package:** `package verify` (white-box testing)

**Test setup pattern:** Each test creates a temp dir simulating a sorted `dirB`, writes files with Pixe-format filenames (`YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>`), registers a handler in a `discovery.Registry`, creates a `hash.Hasher`, and calls `verify.Run(opts)`.

**Required helper:**

```go
// writeTestFile creates a file at dir/relPath with the given content.
// relPath may include subdirectories (e.g., "2024/07-Jul/filename.jpg").
func writeTestFile(t *testing.T, dir, relPath string, content []byte) string
```

**Required test cases (from audit §2.2, GAP-1):**

1. **`TestRun_allFilesVerified`** — Happy path. Create 2-3 files with correct checksums embedded in filenames. Verify `result.Verified == N`, `result.Mismatches == 0`, `result.Unrecognised == 0`. Check stdout contains `OK` for each file.

2. **`TestRun_mismatchDetected`** — Write a file whose content doesn't match the checksum in its filename (e.g., embed checksum "aaaa..." but write different bytes). Verify `result.Mismatches == 1`. Check stdout contains `MISMATCH`.

3. **`TestRun_unrecognisedFile`** — Place a file with a valid Pixe filename but an extension not claimed by any registered handler (e.g., `.xyz`). Verify `result.Unrecognised == 1`. Check stdout contains `UNRECOGNISED`.

4. **`TestRun_unparsableFilename`** — Place a file whose name doesn't match the `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` format (e.g., `random_photo.jpg`). Verify `result.Unrecognised == 1`.

5. **`TestRun_dotfilesSkipped`** — Place dotfiles (`.DS_Store`) and a dot-directory (`.pixe/`) with files inside. Verify they are not counted in any result field. Verify stdout does not mention them.

6. **`TestRun_emptyDirectory`** — Empty dir. Verify `result == Result{}` (all zeros). No error returned.

7. **`TestParseChecksum_validFormats`** — Table-driven test for `parseChecksum()`:
   - `"20211225_062223_7d97e98f1234567890abcdef12345678abcdef12.jpg"` → `("7d97e98f1234567890abcdef12345678abcdef12", true)` (SHA-1, 40 chars)
   - `"20220202_123101_abcdef0123456789.heic"` → `("abcdef0123456789", true)` (16 chars, above 8-char minimum)
   - `"19020220_000000_12345678.dng"` → `("12345678", true)` (exactly 8 chars, minimum)

8. **`TestParseChecksum_invalidFormats`** — Table-driven:
   - `"random_photo.jpg"` → `("", false)` — only 2 underscore-separated parts
   - `"20211225_062223_short.jpg"` → `("short", false)` — checksum < 8 chars
   - `"noextension"` → `("", false)` — no extension
   - `"20211225_062223.jpg"` → `("", false)` — only 2 parts after split

**Handler for tests:** Use the JPEG handler (`internal/handler/jpeg`) registered in a `discovery.Registry`, or create a minimal stub handler that claims `.jpg` and returns an `io.NopCloser(bytes.NewReader(content))` from `HashableReader`. The stub approach is simpler and avoids depending on real JPEG parsing in unit tests.

**Verification:** `go test -race -timeout 120s -cover ./internal/verify/...` — all pass, coverage ≥ 80%.

---

### Task 5 — Add `t.Parallel()` to pure-function packages

**Priority:** High | **Effort:** Low | **Agent:** @developer

Add `t.Parallel()` to the top of every test function (and every `t.Run` subtest) in these packages, which have no shared mutable state:

| Package | Test file(s) | Approx test functions |
|:--------|:-------------|:----------------------|
| `internal/pathbuilder` | `pathbuilder_test.go` | ~15 |
| `internal/hash` | `hasher_test.go` | ~10 |
| `internal/domain` | `pipeline_test.go` | ~8 |
| `internal/fileutil` | `fileutil_test.go` | ~6 |
| `internal/xmp` | `xmp_test.go` | ~12 |

**Pattern:**
```go
func TestFoo(t *testing.T) {
    t.Parallel()
    // ... existing test body
}
```

For table-driven tests with `t.Run`:
```go
for _, tc := range tests {
    tc := tc // capture range variable
    t.Run(tc.name, func(t *testing.T) {
        t.Parallel()
        // ... existing subtest body
    })
}
```

**Important:** Check for `TestMain` in each file — if present, verify it doesn't set global state that would conflict with parallel execution. `pathbuilder` has `TestMain` that pins locale — this is safe because it runs before any tests.

**Verification:** `go test -race -timeout 120s ./internal/pathbuilder/... ./internal/hash/... ./internal/domain/... ./internal/fileutil/... ./internal/xmp/...` — all pass.

---

### Task 6 — Add `t.Parallel()` to handler tests

**Priority:** High | **Effort:** Low | **Agent:** @developer  
**Depends on:** Task 3

After the TIFF handler tests are refactored to use the shared suite (Task 3), add `t.Parallel()` to:

1. The `RunSuite` function's subtests (in `handlertest/suite.go` — done once, applies to all 5 TIFF handlers).
2. The remaining handler test files that were NOT refactored:
   - `internal/handler/jpeg/jpeg_test.go`
   - `internal/handler/heic/heic_test.go`
   - `internal/handler/mp4/mp4_test.go`
   - `internal/handler/cr3/cr3_test.go`
   - `internal/handler/tiffraw/tiffraw_test.go`

**Safety check:** Handler tests use `t.TempDir()` for file I/O — each test gets an isolated directory, so parallel execution is safe. Verify no handler test reads/writes global state.

**Verification:** `go test -race -timeout 120s ./internal/handler/...` — all pass.

---

### Task 7 — Add adversarial filename tests

**Priority:** Medium | **Effort:** Low | **Agent:** @developer  
**Depends on:** Task 4

Add adversarial/negative test cases to `internal/verify/verify_test.go` and `internal/pathbuilder/pathbuilder_test.go`:

**In `verify_test.go` — extend `TestParseChecksum_invalidFormats`:**
- `"../../etc/passwd"` → `("", false)` — path traversal
- `"20211225_062223_" + strings.Repeat("a", 1000) + ".jpg"` → valid (long checksum is accepted — document this)
- `"20211225_062223_ghijklmn.jpg"` → valid (non-hex chars are accepted — document this as a known limitation per audit §2.2)
- Empty string `""` → `("", false)`
- `"20211225_062223_\x00checksum.jpg"` → test behavior with null byte

**In `pathbuilder_test.go` — new test function `TestBuildPath_adversarialFilenames`:**
- Filename with path traversal: `../../etc/passwd`
- Filename with null bytes
- Extremely long filename (> 255 chars)
- Unicode combining characters (e.g., `café` vs `café` — NFC vs NFD)

These tests document expected behavior and prevent regressions. They do not necessarily need to reject the inputs — they just need to produce safe, deterministic output.

**Verification:** `go test -race -timeout 120s ./internal/verify/... ./internal/pathbuilder/...` — all pass.

---

### Task 8 — Increase `handler/mp4` coverage (32% → 70%+)

**Priority:** Medium | **Effort:** Medium | **Agent:** @developer

Add test cases to `internal/handler/mp4/mp4_test.go` targeting uncovered error paths:

1. **Truncated MP4 file** — File with valid `ftyp` box header but truncated before `moov`/`mvhd`. Verify `ExtractDate` returns the Ansel Adams fallback date (not an error).
2. **Missing `mvhd` atom** — Valid MP4 container with `ftyp` and `moov` but no `mvhd` inside `moov`. Verify fallback date.
3. **Zero creation time in `mvhd`** — `mvhd` atom with `creation_time = 0`. Verify fallback date.
4. **`HashableReader` streaming** — Verify `HashableReader` returns data for a file with `mdat` box. Verify deterministic output.
5. **Detect with wrong magic** — File with `.mp4` extension but non-MP4 content.
6. **Empty file** — Zero-byte `.mp4` file. Verify `Detect` returns `false` without error.

**Approach:** Build synthetic MP4 files in-memory using the ISO BMFF box structure (4-byte size + 4-byte type + payload). A `buildFakeMP4` helper should construct minimal valid containers.

**Verification:** `go test -race -timeout 120s -cover ./internal/handler/mp4/...` — coverage ≥ 70%.

---

### Task 9 — Increase `handler/jpeg` coverage (37.6% → 70%+)

**Priority:** Medium | **Effort:** Medium | **Agent:** @developer

Add test cases to `internal/handler/jpeg/jpeg_test.go` targeting uncovered error paths:

1. **Corrupt EXIF** — JPEG with valid SOI marker but malformed EXIF APP1 segment. Verify `ExtractDate` returns fallback date.
2. **Missing JFIF/EXIF markers** — Bare JPEG (SOI + image data + EOI, no APP0/APP1). Verify `ExtractDate` returns fallback date.
3. **Truncated file** — JPEG with SOI but truncated mid-segment. Verify `ExtractDate` handles gracefully.
4. **`HashableReader` error paths** — Non-existent file path. Verify error returned.
5. **`Detect` edge cases** — File with `.jpg` extension but PNG content. Verify `Detect` returns `false`.
6. **`WriteMetadataTags`** — Test with both tags set, one tag set, and empty tags. Verify EXIF is written correctly (or no-op for empty).

**Approach:** Build synthetic JPEG files using the marker structure (FFD8 SOI + segments + FFD9 EOI). Use existing `testdata/` fixtures where appropriate.

**Verification:** `go test -race -timeout 120s -cover ./internal/handler/jpeg/...` — coverage ≥ 70%.

---

### Task 10 — Add concurrent DB access tests to `archivedb`

**Priority:** Medium | **Effort:** Medium | **Agent:** @developer

Add a test function to `internal/archivedb/archivedb_test.go`:

**`TestDB_concurrentAccess`** — Spawn N goroutines (e.g., 10) that simultaneously:
- Insert file records (`InsertFile` or equivalent)
- Query file records (`QueryByChecksum`, `QueryBySourcePath`, or equivalent)
- Insert run records

Use `sync.WaitGroup` to coordinate. Run with `-race` to detect data races. Verify:
- No panics or deadlocks (use `t.Deadline()` or a timeout context)
- All inserted records are queryable after goroutines complete
- No data corruption (row counts match expected)

This validates the architecture's "coordinator goroutine owns DB writes; workers own I/O" pattern at the database layer.

**Verification:** `go test -race -timeout 120s ./internal/archivedb/...` — passes with no race conditions.

---

### Task 11 — Run full test suite and verify green

**Priority:** High | **Effort:** Low | **Agent:** @tester  
**Depends on:** Tasks 1–10

Run the complete validation:

```bash
make fmt-check          # formatting gate
make vet                # go vet
make lint               # golangci-lint
make test               # unit tests with -race
make test-integration   # integration tests
```

All must pass. If any fail, report the failure back for the responsible task to be fixed.

---

### Task 12 — Commit all changes

**Priority:** — | **Effort:** Trivial | **Agent:** @committer  
**Depends on:** Task 11

Commit the test suite improvements. Suggested commit structure (single or multiple — developer's discretion):

- `test(verify): add comprehensive test suite for verify package`
- `refactor(handler): extract shared handlertest suite, deduplicate TIFF handler tests`
- `test(handler): add t.Helper() to buildFake* functions`
- `test: add t.Parallel() across pure-function and handler packages`
- `test(handler): increase mp4 and jpeg handler coverage`
- `test(archivedb): add concurrent DB access tests`
- `test(verify,pathbuilder): add adversarial filename tests`
