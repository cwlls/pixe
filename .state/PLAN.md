# Implementation State

**Status:** Complete — All Tasks Finished (Tasks 1–15)

## Summary

**Completed:** Tasks 1–15 (Atomic Copy, Skip Duplicates, Architecture consistency pass)

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Atomic copy: refactor `copy.Execute` to write to temp file | High | @developer | [x] complete | — | Core safety improvement |
| 2  | Atomic copy: verify-then-rename in `copy` package | High | @developer | [x] complete | 1 | New `Verify` → `Rename` flow |
| 3  | Atomic copy: update sequential pipeline (`processFile`) | High | @developer | [x] complete | 2 | Wire new copy flow into pipeline.go |
| 4  | Atomic copy: update concurrent pipeline (`runWorker`) | High | @developer | [x] complete | 2 | Wire new copy flow into worker.go |
| 5  | Atomic copy: update mismatch handling (delete temp, not preserve) | High | @developer | [x] complete | 2 | Behavioral change from current design |
| 6  | Atomic copy: tests for copy package | High | @tester | [x] complete | 2 | Unit tests for temp file, rename, mismatch cleanup |
| 7  | Atomic copy: integration tests | Medium | @tester | [x] complete | 3, 4 | End-to-end: interrupted copy leaves no partial files |
| 8  | Skip-duplicates: add `SkipDuplicates` to `AppConfig` | High | @developer | [x] complete | — | Config struct change |
| 9  | Skip-duplicates: add `--skip-duplicates` CLI flag | High | @developer | [x] complete | 8 | Flag, Viper binding, config construction |
| 10 | Skip-duplicates: sequential pipeline skip path | High | @developer | [x] complete | 8 | Short-circuit in `processFile` after dedup check |
| 11 | Skip-duplicates: concurrent pipeline skip path | High | @developer | [x] complete | 8 | Coordinator skips worker dispatch for dupes |
| 12 | Skip-duplicates: ledger entry for skipped duplicates | High | @developer | [x] complete | 10, 11 | `destination` omitted, `matches` present |
| 13 | Skip-duplicates: DB row for skipped duplicates | High | @developer | [x] complete | 10, 11 | `dest_path`/`dest_rel` NULL, `is_duplicate=1` |
| 14 | Skip-duplicates: tests | High | @tester | [x] complete | 10, 11, 12, 13 | Unit + integration |
| 15 | Update ARCHITECTURE.md cross-references if needed | Low | @scribe | [x] complete | 7, 14 | Final consistency pass |

---

# Task Descriptions

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
