# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Add `Vacuum()` and `HasActiveRuns()` methods to `archivedb.DB` | high | @developer | [ ] pending | — | Two new methods on the existing `DB` struct in `internal/archivedb/` |
| 2  | Add unit tests for `Vacuum()` and `HasActiveRuns()` | high | @developer | [ ] pending | 1 | Tests in `internal/archivedb/archivedb_test.go` |
| 3  | Implement `cmd/clean.go` — Cobra command, flag binding, `runClean` orchestrator | high | @developer | [ ] pending | 1 | Single file; follows `resume.go` pattern |
| 4  | Implement orphaned temp file scanner in `runClean` | high | @developer | [ ] pending | 3 | `filepath.Walk` with `.pixe-tmp` substring match |
| 5  | Implement orphaned XMP sidecar scanner in `runClean` | high | @developer | [ ] pending | 3 | Regex-gated `.xmp` detection during same walk |
| 6  | Implement database compaction logic in `runClean` | high | @developer | [ ] pending | 1, 3 | Active-run guard + VACUUM + size reporting |
| 7  | Implement dry-run mode for all clean operations | medium | @developer | [ ] pending | 4, 5, 6 | `WOULD REMOVE` verb; skip VACUUM; report current size only |
| 8  | Implement combined output formatting and summary line | medium | @developer | [ ] pending | 4, 5, 6 | Structured sections + summary per Section 7.5.6 |
| 9  | Add unit tests for `cmd/clean.go` | high | @tester | [ ] pending | 3, 4, 5, 6, 7, 8 | Temp file detection, sidecar orphan logic, flag validation, dry-run |
| 10 | Add integration test for `pixe clean` | medium | @tester | [ ] pending | 9 | End-to-end in `internal/integration/`; create orphans, run clean, verify removal |
| 11 | Run `make check` — ensure lint, vet, and all tests pass | high | @developer | [ ] pending | 10 | Gate before commit |


---

## Task Descriptions

### Task 1 — Add `Vacuum()` and `HasActiveRuns()` methods to `archivedb.DB`

**File:** `internal/archivedb/queries.go`

Add two new methods to the existing `DB` struct. These are the database-layer building blocks for `pixe clean`.

```go
// Vacuum rebuilds the database file, reclaiming space from deleted rows
// and reducing fragmentation. Requires exclusive access — no concurrent
// readers or writers should be active.
func (db *DB) Vacuum() error {
    _, err := db.conn.Exec("VACUUM")
    if err != nil {
        return fmt.Errorf("archivedb: vacuum: %w", err)
    }
    return nil
}

// HasActiveRuns returns true if any run has status = 'running'.
// Used by pixe clean to guard against vacuuming while a sort is in progress.
func (db *DB) HasActiveRuns() (bool, error) {
    var count int
    err := db.conn.QueryRow(`SELECT COUNT(*) FROM runs WHERE status = 'running'`).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("archivedb: check active runs: %w", err)
    }
    return count > 0, nil
}
```

**Conventions:**
- Error wrapping uses `"archivedb: <context>: %w"` prefix (matches existing pattern in `runs.go`, `files.go`).
- `Vacuum()` returns only `error` (no result needed).
- `HasActiveRuns()` returns `(bool, error)` — the caller (`cmd/clean.go`) formats the user-facing error message with run details.

---

### Task 2 — Add unit tests for `Vacuum()` and `HasActiveRuns()`

**File:** `internal/archivedb/archivedb_test.go`

Add tests following the existing test patterns in this file (white-box, `testing` stdlib only, `t.TempDir()` for DB files, `openTestDB(t)` helper if one exists).

**Test cases for `HasActiveRuns()`:**
- `TestHasActiveRuns_noRuns` — empty DB → returns `(false, nil)`.
- `TestHasActiveRuns_completedOnly` — insert a run with `status = 'completed'` → returns `(false, nil)`.
- `TestHasActiveRuns_withRunning` — insert a run with `status = 'running'` → returns `(true, nil)`.
- `TestHasActiveRuns_mixedStatuses` — insert runs with `'completed'`, `'interrupted'`, and `'running'` → returns `(true, nil)`.

**Test cases for `Vacuum()`:**
- `TestVacuum_emptyDB` — VACUUM on a fresh DB succeeds without error.
- `TestVacuum_afterInserts` — insert rows, delete some, VACUUM succeeds. Verify DB file still opens and queries work after VACUUM.

---

### Task 3 — Implement `cmd/clean.go` — Cobra command, flag binding, `runClean` orchestrator

**File:** `cmd/clean.go`

Create the Cobra command following the `resume.go` pattern (standalone command with `RunE`). This task covers the command skeleton and flag wiring only — the actual cleanup logic is in Tasks 4–6.

**Structure:**

```go
package cmd

// Apache 2.0 header

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    "github.com/cwlls/pixe-go/internal/archivedb"
    "github.com/cwlls/pixe-go/internal/dblocator"
)

var cleanCmd = &cobra.Command{
    Use:   "clean",
    Short: "Remove orphaned temp files and compact the archive database",
    Long:  `Clean performs maintenance on a destination archive (dirB). ...`,
    RunE:  runClean,
}

func runClean(cmd *cobra.Command, args []string) error {
    // 1. Read flags from Viper
    dir := viper.GetString("clean_dir")
    dbPath := viper.GetString("clean_db_path")
    dryRun := viper.GetBool("clean_dry_run")
    tempOnly := viper.GetBool("clean_temp_only")
    vacuumOnly := viper.GetBool("clean_vacuum_only")

    // 2. Validate: --temp-only and --vacuum-only are mutually exclusive
    if tempOnly && vacuumOnly {
        return fmt.Errorf("--temp-only and --vacuum-only are mutually exclusive")
    }

    // 3. Resolve absolute path of --dir, validate it exists and is a directory
    dir, err := filepath.Abs(dir)
    // ... os.Stat validation ...

    out := cmd.OutOrStdout()
    fmt.Fprintf(out, "Cleaning %s\n\n", dir)

    // 4. Orphaned file cleanup (Tasks 4 & 5)
    // 5. Database compaction (Task 6)
    // 6. Summary line (Task 8)
    return nil
}

func init() {
    rootCmd.AddCommand(cleanCmd)

    cleanCmd.Flags().StringP("dir", "d", "", "destination directory (dirB) to clean (required)")
    cleanCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database")
    cleanCmd.Flags().Bool("dry-run", false, "preview what would be cleaned without modifying anything")
    cleanCmd.Flags().Bool("temp-only", false, "only clean orphaned files, skip database compaction")
    cleanCmd.Flags().Bool("vacuum-only", false, "only compact the database, skip file scanning")

    _ = cleanCmd.MarkFlagRequired("dir")

    _ = viper.BindPFlag("clean_dir", cleanCmd.Flags().Lookup("dir"))
    _ = viper.BindPFlag("clean_db_path", cleanCmd.Flags().Lookup("db-path"))
    _ = viper.BindPFlag("clean_dry_run", cleanCmd.Flags().Lookup("dry-run"))
    _ = viper.BindPFlag("clean_temp_only", cleanCmd.Flags().Lookup("temp-only"))
    _ = viper.BindPFlag("clean_vacuum_only", cleanCmd.Flags().Lookup("vacuum-only"))
}
```

**Viper key namespacing:** All keys prefixed with `clean_` to avoid collisions (follows the convention established by `resume_dir`, `status_source`, `query_dir`, etc.).

**No handler registry needed.** This command does not process media files.

---

### Task 4 — Implement orphaned temp file scanner in `runClean`

**File:** `cmd/clean.go` (within `runClean` or as a helper function)

Implement the `filepath.Walk` over `dirB` that identifies and removes orphaned `.pixe-tmp` files.

**Detection logic:**
```go
// isTempFile returns true if the filename contains the .pixe-tmp marker.
func isTempFile(name string) bool {
    return strings.Contains(name, ".pixe-tmp")
}
```

**Walk implementation:**
- Use `filepath.Walk(dir, ...)` — covers all subdirectories including `duplicates/` and `.pixe/`.
- For each file (not directory): check `isTempFile(filepath.Base(path))`.
- If match and not dry-run: `os.Remove(path)`. On error, log warning to stderr, increment `removeErrors` counter, continue.
- If match and dry-run: skip removal.
- Print per-file output: `REMOVE <filename>  (<relativeParentDir>/)` or `WOULD REMOVE ...`.
- Track counts: `tempFilesRemoved int`, `removeErrors int`.

**Output format (per Architecture Section 7.5.3):**
```
REMOVE .20211225_062223_7d97e98f...jpg.pixe-tmp-abc123  (2021/12-Dec/)
```

The relative parent directory is computed as `filepath.Rel(dir, filepath.Dir(path))`.

---

### Task 5 — Implement orphaned XMP sidecar scanner in `runClean`

**File:** `cmd/clean.go` (within the same `filepath.Walk` callback from Task 4)

During the same walk, detect orphaned `.xmp` sidecar files.

**Detection logic:**
```go
// pixeXMPPattern matches Pixe-generated XMP sidecar filenames:
//   YYYYMMDD_HHMMSS_<hex_checksum>.<media_ext>.xmp
var pixeXMPPattern = regexp.MustCompile(`^\d{8}_\d{6}_[0-9a-f]+\..+\.xmp$`)

// isOrphanedSidecar returns true if the file is a Pixe-generated .xmp sidecar
// whose corresponding media file does not exist.
func isOrphanedSidecar(path string) bool {
    name := filepath.Base(path)
    if !pixeXMPPattern.MatchString(name) {
        return false
    }
    // Strip the trailing ".xmp" to get the expected media file path.
    mediaPath := strings.TrimSuffix(path, ".xmp")
    _, err := os.Stat(mediaPath)
    return os.IsNotExist(err)
}
```

**Key safety constraint:** Only `.xmp` files matching the Pixe naming convention (`^\d{8}_\d{6}_[0-9a-f]+\..+\.xmp$`) are considered. This prevents accidentally removing user-created or Lightroom-generated XMP files.

**Output format (per Architecture Section 7.5.4):**
```
REMOVE 20211225_071500_a3b4c5d6...arw.xmp  (2021/12-Dec/)  orphaned sidecar
```

Track count: `orphanedSidecarsRemoved int`.

---

### Task 6 — Implement database compaction logic in `runClean`

**File:** `cmd/clean.go` (within `runClean`)

Implement the VACUUM workflow with the active-run safety guard.

**Steps:**
1. Resolve DB path via `dblocator.Resolve(dir, dbPath)`.
2. If DB file does not exist at resolved path: print `No archive database found — skipping compaction.` and return (not an error).
3. Get file size before: `os.Stat(loc.DBPath)` → `sizeBefore`.
4. Open DB in read-write mode: `archivedb.Open(loc.DBPath)`. Defer close.
5. Call `db.HasActiveRuns()`. If true, return a fatal error:
   ```
   Error: cannot vacuum — active sort run detected (run <id>, started <time>).
   Complete or interrupt the active run before running 'pixe clean'.
   ```
   To include run details in the error, call `db.FindInterruptedRuns()` (which returns runs with `status = 'running'`) and format the first result.
6. If dry-run: print current size, `(dry-run: VACUUM not executed)`, return.
7. Call `db.Vacuum()`.
8. Get file size after: `os.Stat(loc.DBPath)` → `sizeAfter`.
9. Print size report:
   ```
   Database: <path>
     Size before: <human>
     Size after:  <human>
     Reclaimed:   <human> (<percent>%)
   ```

**Human-readable size formatting:** Use a helper function `formatBytes(n int64) string` that produces `"12.4 MB"`, `"8.1 MB"`, `"0 B"`, etc. Use binary units (KB=1024, MB=1024², etc.) or decimal (1000-based) — match the convention used elsewhere in the project. If no convention exists, use decimal (SI) units for simplicity.

---

### Task 7 — Implement dry-run mode for all clean operations

**File:** `cmd/clean.go`

Thread the `dryRun bool` through all operations:

- **Temp file cleanup (Task 4):** When `dryRun`, change output verb from `REMOVE` to `WOULD REMOVE`. Skip `os.Remove()` call.
- **Sidecar cleanup (Task 5):** Same — `WOULD REMOVE` verb, skip deletion.
- **Database compaction (Task 6):** When `dryRun`, print `Current size: <human>` and `(dry-run: VACUUM not executed)`. Skip `db.Vacuum()` call.
- **Summary line (Task 8):** When `dryRun`, prefix summary with `(dry-run)` or adjust wording to indicate nothing was actually modified.

This task is about ensuring the `dryRun` flag is consistently respected across all three operations. The individual tasks (4, 5, 6) should include the `dryRun` conditional in their initial implementation, but this task ensures the behavior is coherent end-to-end.

---

### Task 8 — Implement combined output formatting and summary line

**File:** `cmd/clean.go`

Implement the structured output per Architecture Section 7.5.6.

**Output structure:**
```
Cleaning <dirB>

Orphaned files:
  REMOVE ...   (or "No orphaned files found.")

Database compaction:
  Size before: ...
  Size after:  ...
  Reclaimed:   ...

Cleaned N temp files, M orphaned sidecars | Reclaimed X from database
```

**Section headers:**
- `"Orphaned files:"` — printed when `!vacuumOnly`. If no orphans found, print `"  No orphaned files found."`.
- `"Database compaction:"` — printed when `!tempOnly`. If DB not found, print `"  No archive database found — skipping compaction."`.

**Summary line:** Always printed. Combines file cleanup counts and DB reclamation. Examples:
- `Cleaned 2 temp files, 1 orphaned sidecar | Reclaimed 4.3 MB from database`
- `No orphaned files found | Reclaimed 0 B from database`
- `Cleaned 3 temp files, 0 orphaned sidecars` (when `--temp-only`)
- `Reclaimed 4.3 MB from database` (when `--vacuum-only`)

Use `cmd.OutOrStdout()` for all output (consistent with `status.go` pattern).

---

### Task 9 — Add unit tests for `cmd/clean.go`

**File:** `cmd/clean_test.go`

Follow the existing test patterns in `cmd/sort_test.go` and `cmd/status_test.go`. Tests are white-box (`package cmd`), use `t.TempDir()`, and the stdlib `testing` package only.

**Test cases:**

1. **`TestRunClean_flagValidation`** — `--temp-only` and `--vacuum-only` together returns error.
2. **`TestRunClean_tempFileDetection`** — Create a `dirB` with:
   - A normal file (`2021/12-Dec/20211225_062223_abc.jpg`)
   - A `.pixe-tmp` file (`2021/12-Dec/.20211225_062223_abc.jpg.pixe-tmp-xyz`)
   - A `.pixe-tmp` file with no random suffix (`2022/01-Jan/.file.jpg.pixe-tmp`)
   - Run clean with `--temp-only`. Verify both temp files are removed, normal file is untouched.
3. **`TestRunClean_orphanedSidecarDetection`** — Create a `dirB` with:
   - A media file + its sidecar (both present) → sidecar NOT removed.
   - A sidecar with no corresponding media file → sidecar IS removed.
   - A non-Pixe `.xmp` file (e.g., `notes.xmp`) → NOT removed.
4. **`TestRunClean_dryRunNoModification`** — Create orphaned files, run with `--dry-run`. Verify all files still exist after the command. Verify output contains `WOULD REMOVE`.
5. **`TestRunClean_vacuumActiveRunGuard`** — Create a DB with a `status = 'running'` run. Run clean with `--vacuum-only`. Verify it returns an error mentioning "active sort run".
6. **`TestRunClean_vacuumSuccess`** — Create a DB, insert some data, run clean with `--vacuum-only`. Verify no error.
7. **`TestRunClean_noDatabaseSkipsCompaction`** — Create a `dirB` with no `.pixe/` directory. Run clean (default mode). Verify temp cleanup runs, compaction is skipped with notice, exit code 0.

---

### Task 10 — Add integration test for `pixe clean`

**File:** `internal/integration/clean_test.go`

End-to-end test that exercises the full `pixe clean` command. Follows the patterns in the existing integration test files.

**Test scenario:**
1. Set up a `dirB` with a valid archive structure (year/month dirs, some media files).
2. Create a `.pixe/pixe.db` database with a completed run and some file records.
3. Plant orphaned artifacts:
   - A `.pixe-tmp` file in a year/month directory.
   - An orphaned `.xmp` sidecar (matching Pixe naming convention, no corresponding media file).
4. Run `pixe clean --dir <dirB>`.
5. Assert:
   - The `.pixe-tmp` file is gone.
   - The orphaned `.xmp` sidecar is gone.
   - Legitimate media files and their sidecars are untouched.
   - The database is still valid (can be opened and queried).
   - Stdout contains `REMOVE` lines for the deleted files.
   - Stdout contains the database compaction report.
   - Exit code is 0.

**Dry-run variant:**
6. Re-plant orphans. Run `pixe clean --dir <dirB> --dry-run`.
7. Assert all orphaned files still exist. Output contains `WOULD REMOVE`.

---

### Task 11 — Run `make check` — ensure lint, vet, and all tests pass

Run the full pre-commit gate:

```bash
make check          # fmt-check + vet + unit tests
make test-all       # includes integration tests
make lint           # golangci-lint
```

Fix any issues surfaced by the linter or tests. This is the final gate before committing.
