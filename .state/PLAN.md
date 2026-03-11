# Implementation State

**Status:** Active — Atomic Copy + Skip Duplicates (Architecture §4.6, §4.10)

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Atomic copy: refactor `copy.Execute` to write to temp file | High | @developer | [ ] pending | — | Core safety improvement |
| 2  | Atomic copy: verify-then-rename in `copy` package | High | @developer | [ ] pending | 1 | New `Verify` → `Rename` flow |
| 3  | Atomic copy: update sequential pipeline (`processFile`) | High | @developer | [ ] pending | 2 | Wire new copy flow into pipeline.go |
| 4  | Atomic copy: update concurrent pipeline (`runWorker`) | High | @developer | [ ] pending | 2 | Wire new copy flow into worker.go |
| 5  | Atomic copy: update mismatch handling (delete temp, not preserve) | High | @developer | [ ] pending | 2 | Behavioral change from current design |
| 6  | Atomic copy: tests for copy package | High | @tester | [ ] pending | 2 | Unit tests for temp file, rename, mismatch cleanup |
| 7  | Atomic copy: integration tests | Medium | @tester | [ ] pending | 3, 4 | End-to-end: interrupted copy leaves no partial files |
| 8  | Skip-duplicates: add `SkipDuplicates` to `AppConfig` | High | @developer | [ ] pending | — | Config struct change |
| 9  | Skip-duplicates: add `--skip-duplicates` CLI flag | High | @developer | [ ] pending | 8 | Flag, Viper binding, config construction |
| 10 | Skip-duplicates: sequential pipeline skip path | High | @developer | [ ] pending | 8 | Short-circuit in `processFile` after dedup check |
| 11 | Skip-duplicates: concurrent pipeline skip path | High | @developer | [ ] pending | 8 | Coordinator skips worker dispatch for dupes |
| 12 | Skip-duplicates: ledger entry for skipped duplicates | High | @developer | [ ] pending | 10, 11 | `destination` omitted, `matches` present |
| 13 | Skip-duplicates: DB row for skipped duplicates | High | @developer | [ ] pending | 10, 11 | `dest_path`/`dest_rel` NULL, `is_duplicate=1` |
| 14 | Skip-duplicates: tests | High | @tester | [ ] pending | 10, 11, 12, 13 | Unit + integration |
| 15 | Update ARCHITECTURE.md cross-references if needed | Low | @scribe | [ ] pending | 7, 14 | Final consistency pass |

---

# Task Descriptions

## Task 1 — Atomic copy: refactor `copy.Execute` to write to temp file

**File:** `internal/copy/copy.go` — `Execute` function (lines 62–105)

**Current behavior:** `Execute(src, dest)` opens the final destination path directly with `os.O_WRONLY|os.O_CREATE|os.O_TRUNC` and streams the source into it. If the process is killed mid-copy, a partial file exists at the canonical archive path.

**Required change:** Modify `Execute` to accept a **temp file path** as the destination, or compute it internally. The function should:

1. Compute the temp path: `filepath.Join(filepath.Dir(dest), "."+filepath.Base(dest)+".pixe-tmp")`
2. Create parent directories via `os.MkdirAll` (same as today).
3. Open the temp path with `os.O_WRONLY|os.O_CREATE|os.O_TRUNC` (same flags, different path).
4. Stream via `io.CopyBuffer` with the existing 32 KB buffer.
5. Close the temp file and check the close error.
6. Preserve source mtime on the temp file via `os.Chtimes` (same as today).
7. Return the temp path to the caller (or return it via a new return value).

**Recommended signature change:**

```go
// Execute copies src to a temporary file adjacent to dest and returns the
// temp file path. The caller is responsible for renaming or deleting the
// temp file after verification.
func Execute(src, dest string) (tmpPath string, err error)
```

Alternatively, export a `TempPath(dest string) string` helper so callers can compute the temp path independently (useful for the verify and rename steps).

**Key constraint:** The temp file MUST be in the same directory as `dest` so that `os.Rename` is an atomic same-filesystem operation.

---

## Task 2 — Atomic copy: verify-then-rename in `copy` package

**File:** `internal/copy/copy.go` — `Verify` function (lines 113–143)

**Current behavior:** `Verify(dest, expectedChecksum, handler, hasher)` re-reads the file at `dest` (the final path) and compares checksums. On mismatch, the file is preserved for debugging.

**Required changes:**

1. `Verify` should accept the **temp file path** (not the final dest) as its first argument. The function signature does not change — the caller simply passes the temp path instead of the final path.

2. Add a new `Rename(tmpPath, dest string) error` function that performs `os.Rename(tmpPath, dest)`. This is a thin wrapper but provides a single place for error wrapping and any future logging.

3. Add a `CleanupTempFile(tmpPath string) error` function that deletes the temp file. Called on verification mismatch. Wraps `os.Remove` with error context.

**New exported functions:**

```go
// TempPath returns the temporary file path for a given destination.
// The temp file is in the same directory as dest, with a leading dot
// and .pixe-tmp suffix: <dir>/.<basename>.pixe-tmp
func TempPath(dest string) string

// Promote atomically renames the verified temp file to its canonical
// destination path. Returns an error if the rename fails.
func Promote(tmpPath, dest string) error

// CleanupTempFile removes an unverified temp file. Called on verification
// mismatch. Logs a warning if removal fails but does not return an error
// (the mismatch error is the primary failure).
func CleanupTempFile(tmpPath string)
```

**Behavioral change on mismatch:** Today, a mismatched file is preserved at the final path for debugging. With atomic copy, a mismatched file is a temp file that is **deleted** — the source in `dirA` is untouched and can be reprocessed. This is a deliberate safety improvement: the canonical path never contains unverified data.

---

## Task 3 — Atomic copy: update sequential pipeline (`processFile`)

**File:** `internal/pipeline/pipeline.go` — `processFile` function (lines 300–472)

**Current flow (lines 367–386):**

```
copy.Execute(src, absDest)  →  db.UpdateFileStatus("copied")
copy.Verify(absDest, ...)   →  db.UpdateFileStatus("verified") or ("mismatch")
```

**New flow:**

```
tmpPath := copy.TempPath(absDest)
copy.Execute(src, absDest)          // now returns tmpPath, writes to temp
db.UpdateFileStatus("copied")       // status reflects temp file written
vr := copy.Verify(tmpPath, ...)     // verify the TEMP file
if !vr.Success:
    copy.CleanupTempFile(tmpPath)    // delete temp file
    db.UpdateFileStatus("mismatch")
    return error
copy.Promote(tmpPath, absDest)      // atomic rename
db.UpdateFileStatus("verified")     // now at canonical path
```

**Key points:**
- The `"copied"` status now means "temp file written, not yet verified."
- The `"verified"` status now means "verified AND renamed to canonical path."
- The `WithDestination(absDest, relDest)` option on the `"copied"` update still records the intended final path (not the temp path) — the DB tracks where the file will end up, not where the temp file is.
- On mismatch, `CleanupTempFile` is called before returning the error. The DB records `"mismatch"` with the error details.

---

## Task 4 — Atomic copy: update concurrent pipeline (`runWorker`)

**File:** `internal/pipeline/worker.go` — `runWorker` function (lines 394–536)

**Same transformation as Task 3**, applied to the worker's copy/verify block (lines 482–508):

```
tmpPath := copy.TempPath(assign.absDest)
copy.Execute(src, assign.absDest)    // writes to temp
db.UpdateFileStatus("copied", ...)
vr := copy.Verify(tmpPath, ...)      // verify temp
if !vr.Success:
    copy.CleanupTempFile(tmpPath)
    db.UpdateFileStatus("mismatch", ...)
    doneCh <- workerFinalResult{err: ...}
    continue
copy.Promote(tmpPath, assign.absDest)  // atomic rename
db.UpdateFileStatus("verified")
```

**Also update the coordinator's post-copy dedup race handler** (lines 310–350 in `runConcurrentCtx`). When a race is detected and the file needs to be relocated to `duplicates/`, the file is already at its canonical path (it passed verification and was renamed). The `os.Rename(absDest, dupAbsDest)` call remains unchanged — it's relocating a verified file, not a temp file.

---

## Task 5 — Atomic copy: update mismatch handling (delete temp, not preserve)

**Files:** `internal/copy/copy.go`, `internal/pipeline/pipeline.go`, `internal/pipeline/worker.go`

**Current behavior:** On verification mismatch, the destination file is intentionally preserved at its final path for user inspection.

**New behavior:** On verification mismatch, the temp file is **deleted** via `copy.CleanupTempFile(tmpPath)`. The canonical path never exists. The source file in `dirA` is untouched.

**Rationale:** With atomic copy, preserving a corrupt temp file serves no purpose — the user can always re-run to reprocess the file. The safety improvement is that the canonical archive path is guaranteed to contain only verified files.

**Implementation:** This is handled in Tasks 3 and 4 (the pipeline changes). This task is a reminder to:
1. Remove any comments in `copy.Verify` about "intentionally not deleting" the file.
2. Update the `CopyResult` struct doc comment if it references preservation behavior.
3. Ensure the `ERR` stdout line and DB `mismatch` status still include the expected vs. actual checksums for diagnostics.

---

## Task 6 — Atomic copy: tests for copy package

**File:** `internal/copy/copy_test.go`

**New test cases:**

| Test | What it verifies |
|------|-----------------|
| `TestExecute_writesToTempFile` | `Execute` creates `.<name>.pixe-tmp` in the dest directory, not the final path |
| `TestTempPath_format` | `TempPath` produces correct `.<basename>.pixe-tmp` pattern |
| `TestPromote_atomicRename` | `Promote` renames temp to final; final exists, temp does not |
| `TestPromote_parentDirExists` | `Promote` works when parent dir was created by `Execute` |
| `TestCleanupTempFile_removesFile` | `CleanupTempFile` deletes the temp file |
| `TestCleanupTempFile_missingFile` | `CleanupTempFile` does not error if file already gone |
| `TestVerify_mismatchDeletesTemp` | Full flow: Execute → Verify (with bad checksum) → temp file is gone |
| `TestVerify_successThenPromote` | Full flow: Execute → Verify (good checksum) → Promote → file at canonical path |

**Testing pattern:** Use `t.TempDir()` for all filesystem tests. Create a source file with known content, run the flow, assert file existence/absence and checksums.

---

## Task 7 — Atomic copy: integration tests

**File:** `internal/integration/` (new test file or addition to existing)

**Test cases:**

| Test | What it verifies |
|------|-----------------|
| `TestSort_noPartialFilesOnInterrupt` | Start a sort, kill the pipeline mid-copy (via context cancellation), verify no files exist at canonical paths — only `.pixe-tmp` files (or nothing if the copy hadn't started) |
| `TestSort_tempFileCleanupOnResume` | Create an orphaned `.pixe-tmp` file, run `pixe resume`, verify the temp file is overwritten and the final file is correct |
| `TestSort_verifiedFileAtCanonicalPath` | Normal sort completes, every file at a canonical path passes independent hash verification |

---

## Task 8 — Skip-duplicates: add `SkipDuplicates` to `AppConfig`

**File:** `internal/config/config.go` — `AppConfig` struct (lines 22–63)

**Change:** Add a new field after `Recursive`:

```go
type AppConfig struct {
    // ... existing fields ...
    Recursive      bool
    SkipDuplicates bool     // skip copying files whose checksum matches an archived file
    Ignore         []string
}
```

**Convention:** Follows the existing pattern — plain `bool`, no pointer, no default value needed (zero value `false` is the correct default).

---

## Task 9 — Skip-duplicates: add `--skip-duplicates` CLI flag

**File:** `cmd/sort.go` — `init()` function (lines 194–219) and `runSort` (lines 66–77)

**Three additions:**

1. **Flag declaration** (after line 204, the `--recursive` flag):
   ```go
   sortCmd.Flags().Bool("skip-duplicates", false,
       "Skip copying duplicate files instead of copying to duplicates/ directory")
   ```

2. **Viper binding** (after line 218):
   ```go
   _ = viper.BindPFlag("skip_duplicates", sortCmd.Flags().Lookup("skip-duplicates"))
   ```

3. **Config construction** (after `Recursive` in the `cfg` literal):
   ```go
   SkipDuplicates: viper.GetBool("skip_duplicates"),
   ```

**Config file support:** Already documented in ARCHITECTURE.md §7.2 as `skip_duplicates: false`. Viper handles the mapping automatically via `GetBool("skip_duplicates")`.

---

## Task 10 — Skip-duplicates: sequential pipeline skip path

**File:** `internal/pipeline/pipeline.go` — `processFile` function

**Current flow after dedup check (lines 337–353):**

```go
// isDuplicate is set, existingDestForLedger is set
relDest := pathbuilder.Build(captureDate, checksum, ext, isDuplicate, opts.RunTimestamp)
// ... proceeds to copy, verify, tag ...
```

**New flow — insert between the dedup check and pathbuilder.Build:**

```go
if isDuplicate && opts.Config.SkipDuplicates {
    // Skip the copy entirely. Record in DB and return ledger entry.
    if db != nil {
        _ = db.UpdateFileStatus(fileID, "complete",
            archivedb.WithChecksum(checksum),
            archivedb.WithIsDuplicate(true))
    }
    le := &domain.LedgerEntry{
        Path:     df.RelPath,
        Status:   domain.LedgerStatusDuplicate,
        Checksum: checksum,
        Matches:  existingDestForLedger,
        // Destination intentionally omitted — no file was written
    }
    return le, true, nil
}
```

**Key points:**
- The `Destination` field is left empty (zero value `""`) — `omitempty` ensures it's absent from the JSON.
- The DB row gets `status = 'complete'`, `is_duplicate = 1`, `checksum` set, but `dest_path`/`dest_rel` remain NULL.
- The `Matches` field tells the user where the existing copy lives.
- The `opts.Config` reference requires `SortOptions` to carry the config — verify this is already the case (it is: `SortOptions.Config *config.AppConfig`).

---

## Task 11 — Skip-duplicates: concurrent pipeline skip path

**File:** `internal/pipeline/worker.go` — coordinator in `runConcurrentCtx` (lines 252–284)

**Current flow:** After the coordinator determines `isDuplicate`, it always sends a `destAssignment` to the worker, which then copies the file to the duplicates directory.

**New flow:** When `isDuplicate && opts.Config.SkipDuplicates`, the coordinator should **not** send a `destAssignment` to the worker. Instead, it should:

1. Update the DB directly (same as Task 10).
2. Construct the ledger entry.
3. Write the stdout `DUPE` line.
4. Write the ledger entry.
5. Increment the `completed` counter.
6. **Send a "skip" signal** to the worker so it doesn't block waiting on `assignCh`.

**Implementation options:**

**Option A — Add a `skip` field to `destAssignment`:**

```go
type destAssignment struct {
    absDest               string
    relDest               string
    isDuplicate           bool
    existingDestForLedger string
    skipCopy              bool   // NEW: coordinator decided to skip this file
}
```

The worker checks `assign.skipCopy` and, if true, sends a no-op `workerFinalResult` back immediately without doing any I/O. The coordinator then handles it in the `doneCh` select case.

**Option B — Handle entirely in the coordinator:** The coordinator writes the DUPE result directly (bypassing the worker entirely) and sends a skip signal so the worker can move on to the next item. This is cleaner but requires the coordinator to handle the `completed++` accounting for skipped dupes separately from the `doneCh` path.

**Recommendation:** Option A is simpler and preserves the existing coordinator/worker communication pattern. The worker already handles various early-exit paths (copy failure, verify failure). Adding a `skipCopy` check at the top of the copy block is minimal.

---

## Task 12 — Skip-duplicates: ledger entry for skipped duplicates

**Files:** `internal/domain/pipeline.go`, `internal/pipeline/pipeline.go`, `internal/pipeline/worker.go`

**No struct changes needed.** The existing `LedgerEntry` struct already supports the skipped-duplicate case:

```go
le := &domain.LedgerEntry{
    Path:     df.RelPath,
    Status:   domain.LedgerStatusDuplicate,  // "duplicate"
    Checksum: checksum,
    Matches:  existingDestForLedger,
    // Destination: "" — omitted via omitempty
}
```

**Verification:** Ensure that both the sequential path (Task 10) and concurrent path (Task 11) produce identical ledger entries for skipped duplicates. The stdout `DUPE` line format is also unchanged: `DUPE <filename> -> matches <existing_dest>`.

---

## Task 13 — Skip-duplicates: DB row for skipped duplicates

**File:** `internal/archivedb/files.go`

**Current behavior for copied duplicates:** The DB row gets `status = 'complete'`, `is_duplicate = 1`, `dest_path` and `dest_rel` set to the `duplicates/...` path, and `checksum` set.

**New behavior for skipped duplicates:** The DB row gets `status = 'complete'`, `is_duplicate = 1`, `checksum` set, but `dest_path` and `dest_rel` remain NULL (no file was written).

**Verify:** The existing `UpdateFileStatus` with `WithIsDuplicate(true)` and `WithChecksum(checksum)` options should handle this correctly — `dest_path`/`dest_rel` are only set when `WithDestination` is explicitly passed. Confirm that `CompleteFileWithDedupCheck` is NOT called for skipped duplicates (it shouldn't be — the skip happens before the copy, so there's no post-copy race to check).

**Also verify:** Existing queries that filter on `is_duplicate = 1` (e.g., `AllDuplicates()`, `DuplicatePairs()`) will include skipped duplicates. The `DuplicatePairs` query joins on `dest_rel`, which will be NULL for skipped dupes — ensure this doesn't produce incorrect results. May need a `WHERE dest_rel IS NOT NULL` guard on the duplicate side of the join, or handle NULL gracefully in the result formatting.

---

## Task 14 — Skip-duplicates: tests

**Files:** `internal/pipeline/pipeline_test.go`, `internal/pipeline/worker_test.go`, `internal/integration/`

**Unit test cases:**

| Test | What it verifies |
|------|-----------------|
| `TestProcessFile_skipDuplicates_noFileCopied` | With `SkipDuplicates=true`, a duplicate file produces a `DUPE` ledger entry with no `Destination`, and no file exists in `dirB` |
| `TestProcessFile_skipDuplicates_dbRowCorrect` | DB row has `status='complete'`, `is_duplicate=1`, `checksum` set, `dest_path` NULL |
| `TestProcessFile_defaultDuplicates_stillCopied` | With `SkipDuplicates=false` (default), duplicates are still copied to `duplicates/` — regression guard |
| `TestRunConcurrent_skipDuplicates` | Concurrent pipeline with `SkipDuplicates=true` correctly skips duplicate copies |

**Integration test cases:**

| Test | What it verifies |
|------|-----------------|
| `TestSort_skipDuplicates_endToEnd` | Full sort with `--skip-duplicates`: source has files already in archive, verify no `duplicates/` directory created, DUPE lines on stdout, ledger entries correct |
| `TestSort_skipDuplicates_ledgerFormat` | Parse the JSONL ledger and verify skipped duplicate entries have `checksum` and `matches` but no `destination` |

**Query test cases:**

| Test | What it verifies |
|------|-----------------|
| `TestAllDuplicates_includesSkipped` | `AllDuplicates()` returns both copied and skipped duplicates |
| `TestDuplicatePairs_handlesNullDestRel` | `DuplicatePairs()` handles skipped duplicates (NULL `dest_rel`) gracefully |

---

## Task 15 — Update ARCHITECTURE.md cross-references if needed

**File:** `.state/ARCHITECTURE.md`

**Scope:** After all implementation is complete, do a final consistency pass to ensure:
- Section references (§4.6, §4.10, §5.1, §8.5) still accurately describe the implemented behavior.
- Any implementation decisions that diverged from the architecture are reflected.
- The `pixe clean` future consideration (§10 item 9) still accurately describes the temp file cleanup strategy.

This is a documentation-only task with no code changes.
