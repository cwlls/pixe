# Implementation Plan

## Feature: Post-Sort Remediation (OVERVIEW §13)

Five interconnected features: `pixe doctor` (problem summary + advice), `pixe retry` (targeted error retry), inline post-sort hints, `--list` flag on query subcommands, and `--run` scoping on query subcommands.

## Task Summary

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add `MostRecentRunBySource()` and run-scoped query methods to archivedb | high | @developer | [x] complete | — | Foundation for doctor, retry, and --run scoping |
| 2 | Create `internal/doctor/` diagnosis engine package | high | @developer | [x] complete | — | Pure logic, no DB/manifest dependency |
| 3 | Add `--run` flag to `query errors`, `query skipped`, `query duplicates` | high | @developer | [x] complete | 1 | Extends existing query subcommands |
| 4 | Add `--list` flag to `query errors`, `query skipped`, `query duplicates` | high | @developer | [x] complete | 3 | Depends on --run for scoped list export |
| 5 | Create `cmd/doctor.go` — ledger-based default mode | high | @developer | [x] complete | 2 | Reads ledger, delegates to doctor engine |
| 6 | Add DB-backed mode to `cmd/doctor.go` | medium | @developer | [x] complete | 1, 5 | Adds --dest, --run flags; richer categorization |
| 7 | Add `--advice` mode to `cmd/doctor.go` | medium | @developer | [x] complete | 5 | Adds category descriptions to output |
| 8 | Extend pipeline to accept pre-built file list | high | @developer | [x] complete | — | Add optional `Files` field to `SortOptions` |
| 9 | Create `cmd/retry.go` — targeted error retry | high | @developer | [x] complete | 1, 8 | Queries errored files, feeds to pipeline |
| 10 | Add inline post-sort hints to `cmd/sort.go` | medium | @developer | [x] complete | — | Static hint text after summary |
| 11 | Tests for `internal/doctor/` | high | @tester | [x] complete | 2 | Unit tests for classify + summarize |
| 12 | Tests for new archivedb query methods | high | @tester | [x] complete | 1 | Run-scoped queries, MostRecentRunBySource |
| 13 | Tests for `cmd/doctor.go` | medium | @tester | [x] complete | 5, 6, 7 | Ledger mode, DB mode, --advice, --json, edge cases |
| 14 | Tests for `cmd/retry.go` | medium | @tester | [x] complete | 9 | Retry flow, missing source files, tag_failed shortcut |
| 15 | Tests for `--list` and `--run` on query subcommands | medium | @tester | [x] complete | 3, 4 | Flag interaction, mutual exclusivity, output format |
| 16 | Tests for inline post-sort hints | low | @tester | [x] complete | 10 | Hint suppression in quiet/progress/pipe modes |
| 17 | Update docgen for new commands | low | @developer | [ ] pending | 5, 9 | Register doctor + retry in docgen extraction |
| 18 | Commit: post-sort remediation features | low | @committer | [ ] pending | 1–16 | Single feature commit or per-wave commits |

---

## Parallelization Strategy

### Wave 1 — Foundations (no interdependencies)
Tasks **1, 2, 8, 10** can all run in parallel.

- **Task 1** (archivedb queries) touches only `internal/archivedb/` — no overlap with other tasks.
- **Task 2** (doctor engine) creates a new package `internal/doctor/` — no overlap.
- **Task 8** (pipeline file list) touches only `internal/pipeline/pipeline.go` — no overlap.
- **Task 10** (sort hints) touches only `cmd/sort.go` — no overlap.

### Wave 2 — Commands and query extensions (depend on Wave 1)
Tasks **3, 4, 5, 9** can run in parallel once their Wave 1 dependencies are met.

- **Task 3** (--run flag) depends on Task 1. Touches `cmd/query_errors.go`, `cmd/query_skipped.go`, `cmd/query_duplicates.go`.
- **Task 4** (--list flag) depends on Task 3. Touches the same three files — must run after Task 3.
- **Task 5** (doctor command, ledger mode) depends on Task 2. Creates `cmd/doctor.go`.
- **Task 9** (retry command) depends on Tasks 1 and 8. Creates `cmd/retry.go`.

### Wave 3 — Doctor enhancements (depend on Wave 2)
Tasks **6, 7** run sequentially after Task 5.

- **Task 6** (doctor DB mode) depends on Tasks 1 and 5.
- **Task 7** (doctor --advice) depends on Task 5.

### Wave 4 — Tests (depend on their implementation tasks)
Tasks **11, 12, 13, 14, 15, 16** can run in parallel, each gated on its implementation dependency.

### Wave 5 — Finalization
Tasks **17, 18** run last.

---

## Task Descriptions

### Task 1: Add `MostRecentRunBySource()` and run-scoped query methods to archivedb

**File:** `internal/archivedb/queries.go`

**New methods on `*DB`:**

1. **`MostRecentRunBySource(sourceDir string) (*Run, error)`**
   - Query: `SELECT * FROM runs WHERE source = ? AND status IN ('completed', 'interrupted') ORDER BY started_at DESC LIMIT 1`
   - Returns `(nil, nil)` if no matching run found (consistent with `GetRun()` pattern).
   - Used by `doctor` (default scoping) and `retry` (default scoping).

2. **`MostRecentRun() (*Run, error)`**
   - Query: `SELECT * FROM runs WHERE status IN ('completed', 'interrupted') ORDER BY started_at DESC LIMIT 1`
   - Returns `(nil, nil)` if no runs exist.
   - Used when neither `--run` nor `--source` is provided.

3. **`FilesWithErrorsByRun(runID string) ([]*FileWithSource, error)`**
   - Same query as `FilesWithErrors()` but adds `AND f.run_id = ?` clause.
   - When `runID` is empty, behaves identically to `FilesWithErrors()` (no filter).

4. **`AllSkippedByRun(runID string) ([]*FileRecord, error)`**
   - Same query as `AllSkipped()` but adds `AND run_id = ?` clause.
   - When `runID` is empty, behaves identically to `AllSkipped()`.

5. **`AllDuplicatesByRun(runID string) ([]*FileRecord, error)`**
   - Same query as `AllDuplicates()` but adds `AND run_id = ?` clause.
   - When `runID` is empty, behaves identically to `AllDuplicates()`.

6. **`DuplicatePairsByRun(runID string) ([]*DuplicatePair, error)`**
   - Same query as `DuplicatePairs()` but adds `AND d.run_id = ?` clause.
   - When `runID` is empty, behaves identically to `DuplicatePairs()`.

**Design decision — empty-string-means-no-filter:** Rather than creating separate `ByRun` methods, an alternative is to modify the existing methods to accept an optional `runID string` parameter. However, this changes existing call sites. The `ByRun` suffix approach is safer — existing callers are untouched, and the new methods are explicit about their filtering behavior. The existing unfiltered methods can be refactored later to delegate to the `ByRun` variants with `""`.

**Pattern to follow:** Use `scanFileRecord()` / `scanFileWithSource()` helpers already in the file. Use `const` blocks for SQL strings. Use `?` placeholders.

**Run ID resolution:** The `--run` flag accepts prefix matches. The `cmd/` layer calls `queryDB.GetRunByPrefix(prefix)` (already exists, line 329) to resolve the prefix to a full UUID before passing it to these methods. The `ByRun` methods receive the full UUID, not a prefix.

**Error wrapping:** `fmt.Errorf("archivedb: <method>: %w", err)` — consistent with existing methods.

---

### Task 2: Create `internal/doctor/` diagnosis engine package

**New package:** `internal/doctor/`

**Files to create:**

1. **`doc.go`** — Package comment:
   ```go
   // Package doctor provides a diagnosis engine that categorizes pipeline errors
   // and skips into human-readable categories with plain-language explanations.
   // It operates on abstract Entry values and has no dependency on archivedb or
   // manifest packages.
   package doctor
   ```

2. **`categories.go`** — Static category definitions.

   **Types:**
   ```go
   type Severity string
   const (
       SeverityActionable    Severity = "actionable"
       SeverityInformational Severity = "informational"
   )

   type Category struct {
       Name        string   // e.g., "Corrupted metadata"
       Severity    Severity
       Description string   // 2-3 sentence advice text (used by --advice mode)
       section     string   // "errors", "skipped", "duplicates" — unexported, for grouping
       patterns    []string // substring matches against reason — unexported
       statuses    []string // ledger statuses this category applies to — unexported
   }
   ```

   **Exported variable:** `var Categories []Category` — the ordered list of all 14 categories (13 named + 1 uncategorized catch-all). Order determines display priority: actionable categories first within each section.

   **Category definitions** (from OVERVIEW §13.2.3):
   - Errors: Corrupted metadata, Corrupt file structure, Disk/permission error, Copy failure, Integrity mismatch, Tag injection failed, Database error, Uncategorized
   - Skipped: Unsupported format, Previously imported, Outside date range, Symlink, Dotfile, Detection error

   Each category's `patterns` field contains the substring(s) to match against the `Reason` field. The `statuses` field contains the ledger statuses (`"error"`, `"skip"`) this category applies to. The uncategorized catch-all has empty `patterns` and matches any unmatched entry.

3. **`classify.go`** — Classification function.

   ```go
   // Classify returns the Category that best matches the given status and reason.
   // It iterates Categories in order and returns the first match. If no pattern
   // matches, returns the Uncategorized catch-all.
   func Classify(status, reason string) *Category
   ```

   Matching logic: for each category, check if `status` is in `category.statuses`, then check if any `category.patterns` substring appears in `reason`. First match wins. The uncategorized category is always last and matches everything.

4. **`summarize.go`** — Report generation.

   **Types:**
   ```go
   // Entry is the minimal input the engine needs. Populated from either
   // ledger entries or DB file records by the cmd/ layer.
   type Entry struct {
       Path   string // relative path from source
       Status string // ledger status: "error", "skip", "duplicate", "copy"
       Reason string // error message or skip reason
   }

   type CategoryResult struct {
       Category *Category
       Count    int
       Files    []string // paths of files in this category
   }

   type SectionReport struct {
       Total      int
       Categories []CategoryResult // ordered by category definition order, zero-count omitted
   }

   type Report struct {
       Errors     SectionReport
       Skipped    SectionReport
       Duplicates SectionReport
   }

   // Summarize takes a list of problem entries and produces a categorized report.
   // Entries with status "copy" are ignored (they are not problems).
   func Summarize(entries []Entry) *Report
   ```

   `Summarize` iterates entries, calls `Classify` for each, and accumulates counts per category. Returns a `Report` with zero-count categories omitted from each section's `Categories` slice.

**Package constraints:**
- No imports from `archivedb`, `manifest`, `domain`, or any other `internal/` package.
- Only stdlib imports (`strings`, `sort`).
- The `Entry` struct is intentionally minimal — the `cmd/` layer maps from `LedgerEntry` or `FileRecord` to `Entry`.

---

### Task 3: Add `--run` flag to `query errors`, `query skipped`, `query duplicates`

**Files:** `cmd/query_errors.go`, `cmd/query_skipped.go`, `cmd/query_duplicates.go`

**For each file:**

1. **Add flag in `init()`:**
   ```go
   xxxCmd.Flags().String("run", "", "filter to a specific run (prefix match)")
   ```
   No Viper binding needed — these are simple local flags read via `cmd.Flags().GetString("run")` in `RunE`. (Follows the `--pairs` pattern in `query_duplicates.go` which uses a package-level `bool` var, but `--run` is simpler as a direct flag read since it's not needed outside `RunE`.)

2. **Resolve run ID in `RunE`:**
   ```go
   runFilter, _ := cmd.Flags().GetString("run")
   var runID string
   if runFilter != "" {
       runs, err := queryDB.GetRunByPrefix(runFilter)
       if err != nil { return ... }
       if len(runs) == 0 { return fmt.Errorf("no run matching %q", runFilter) }
       if len(runs) > 1 { return fmt.Errorf("ambiguous run prefix %q matches %d runs", runFilter, len(runs)) }
       runID = runs[0].ID
   }
   ```

3. **Call the new `ByRun` method:**
   - `query_errors.go`: Replace `queryDB.FilesWithErrors()` with `queryDB.FilesWithErrorsByRun(runID)`.
   - `query_skipped.go`: Replace `queryDB.AllSkipped()` with `queryDB.AllSkippedByRun(runID)`.
   - `query_duplicates.go`: Replace `queryDB.AllDuplicates()` with `queryDB.AllDuplicatesByRun(runID)`. Also replace `queryDB.DuplicatePairs()` with `queryDB.DuplicatePairsByRun(runID)`.

4. **Update summary line** to indicate run scoping when active:
   - Append ` (run <truncated-id>)` to the summary string when `runID != ""`.

---

### Task 4: Add `--list` flag to `query errors`, `query skipped`, `query duplicates`

**Files:** `cmd/query_errors.go`, `cmd/query_skipped.go`, `cmd/query_duplicates.go`

**For each file:**

1. **Add flag in `init()`:**
   ```go
   xxxCmd.Flags().Bool("list", false, "output one source file path per line")
   ```

2. **Add mutual exclusivity check at top of `RunE`:**
   ```go
   listMode, _ := cmd.Flags().GetBool("list")
   if listMode && viper.GetBool("query_json") {
       return fmt.Errorf("--list and --json are mutually exclusive")
   }
   ```

3. **Add list output branch** before the existing JSON/table branches:
   ```go
   if listMode {
       for _, f := range results {
           fmt.Fprintln(os.Stdout, f.SourcePath)
       }
       return nil
   }
   ```
   - For `query_errors.go`: `f.SourcePath` from `FileWithSource`.
   - For `query_skipped.go`: `f.SourcePath` from `FileRecord`.
   - For `query_duplicates.go`: `f.SourcePath` from `FileRecord`. When `--pairs` is also set, `--list` takes precedence (still outputs source paths only).

4. **Output format:** One absolute path per line. No headers, no summary, no color. Trailing newline after last path.

---

### Task 5: Create `cmd/doctor.go` — ledger-based default mode

**New file:** `cmd/doctor.go`

**Command registration:**
```go
var doctorCmd = &cobra.Command{
    Use:   "doctor",
    Short: "Diagnose problems from the last sort run",
    Long:  "Summarize errors, skips, and duplicates from the most recent sort. " +
           "Run from a source directory to read the ledger, or use --dest for DB-backed mode.",
    RunE:  runDoctor,
}
```

**Flags (in `init()`):**
```go
doctorCmd.Flags().Bool("advice", false, "show plain-language explanations for each problem category")
doctorCmd.Flags().StringP("source", "s", "", "source directory (default: current directory)")
doctorCmd.Flags().StringP("dest", "d", "", "archive directory (enables DB-backed mode)")
doctorCmd.Flags().String("run", "", "specific run ID, prefix match (requires --dest)")
doctorCmd.Flags().Bool("json", false, "output as JSON")

_ = viper.BindPFlag("doctor_dest", doctorCmd.Flags().Lookup("dest"))
_ = viper.BindPFlag("doctor_json", doctorCmd.Flags().Lookup("json"))
```

**`runDoctor` logic (ledger mode only — DB mode is Task 6):**

1. Resolve source directory: `--source` flag, or CWD via `os.Getwd()`.
2. If `--run` is provided without `--dest`, return error: `"--run requires --dest (the archive directory containing the database)"`.
3. If `--dest` is provided, delegate to DB mode (Task 6 — initially return `"DB mode not yet implemented"`).
4. Call `manifest.LoadLedger(sourceDir)`.
5. If `nil, nil` returned: print `"No ledger found in <dir>.\nRun pixe sort from this directory first, or use -s to specify a source directory."` and return nil (exit 0).
6. Convert `LedgerContents` to `[]doctor.Entry`:
   ```go
   var entries []doctor.Entry
   for _, e := range ledger.Entries {
       if e.Status == domain.LedgerStatusCopy {
           continue // not a problem
       }
       entries = append(entries, doctor.Entry{
           Path:   e.Path,
           Status: e.Status,
           Reason: e.Reason,
       })
   }
   ```
7. Call `doctor.Summarize(entries)` to get a `*doctor.Report`.
8. **Render header line:** `"Last sort: <date> → ...<dest-basename>"` — date from `ledger.Header.PixeRun` (ISO 8601), dest basename from `ledger.Header.Destination`.
9. **If `--json`:** marshal the report as JSON (see §13.2.6 in OVERVIEW) and return.
10. **If report has no problems** (all section totals are 0): print `"No problems found. <N> files sorted successfully."` and return.
11. **Render summary lines** (default mode):
    - For each section (errors, skipped, duplicates) with total > 0:
      - Print `"  <total> <section> — <cat1> (<count>), <cat2> (<count>)"`.
    - Print blank line, then `"Run pixe doctor --advice for details and suggested actions."`.
12. **If `--advice`:** delegate to advice renderer (Task 7 — initially skip).

**Helper functions in `cmd/doctor.go`:**
- `renderDoctorSummary(w io.Writer, report *doctor.Report)` — default mode output.
- `renderDoctorAdvice(w io.Writer, report *doctor.Report, destPath string)` — advice mode output (Task 7).
- `renderDoctorJSON(w io.Writer, report *doctor.Report, header domain.LedgerHeader, source string) error` — JSON output.
- `countSuccessful(entries []domain.LedgerEntry) int` — count entries with status "copy" for the "no problems" message.

---

### Task 6: Add DB-backed mode to `cmd/doctor.go`

**File:** `cmd/doctor.go` (extends Task 5)

**When `--dest` is provided:**

1. Resolve dest via `resolveDest("doctor_dest")`.
2. Open DB read-only (follow `query.go` pattern: `dblocator.Locate()` → `archivedb.OpenReadOnly()`).
3. Resolve target run:
   - If `--run` provided: `queryDB.GetRunByPrefix(runFilter)` → validate single match.
   - If `--source` provided: `queryDB.MostRecentRunBySource(absSourceDir)`.
   - Otherwise: `queryDB.MostRecentRun()`.
   - If no run found: print `"No completed runs found in this archive."` and return.
4. Query errored files: `db.FilesWithErrorsByRun(run.ID)`.
5. Query skipped files: `db.AllSkippedByRun(run.ID)`.
6. Query duplicate count: `db.AllDuplicatesByRun(run.ID)`.
7. Convert to `[]doctor.Entry`:
   - For error files: `Entry{Path: f.SourcePath, Status: string(f.Status), Reason: deref(f.Error)}` — note: DB mode uses the actual `FileStatus` string (`"failed"`, `"mismatch"`, `"tag_failed"`) not the ledger status `"error"`. The doctor engine's `Classify` function handles both.
   - For skipped files: `Entry{Path: f.SourcePath, Status: "skip", Reason: deref(f.SkipReason)}`.
   - For duplicates: `Entry{Path: f.SourcePath, Status: "duplicate", Reason: ""}`.
8. Call `doctor.Summarize(entries)` and render using the same output functions.
9. Header line uses `run.StartedAt` for date and `run.Destination` for dest basename.

**Classify enhancement for DB mode:** The `Classify` function in `internal/doctor/classify.go` must handle both ledger statuses (`"error"`, `"skip"`) and DB statuses (`"failed"`, `"mismatch"`, `"tag_failed"`, `"skipped"`). The category `statuses` field should include both forms. For example, the "Integrity mismatch" category matches status `"mismatch"` directly (DB mode) and also matches status `"error"` with pattern `"verify:"` (ledger mode).

**Update `categories.go`:** Each category's `statuses` slice includes both ledger and DB status strings:
- Error categories: `["error", "failed"]` (plus `"mismatch"` for integrity, `"tag_failed"` for tag failure).
- Skip categories: `["skip", "skipped"]`.

---

### Task 7: Add `--advice` mode to `cmd/doctor.go`

**File:** `cmd/doctor.go` (extends Tasks 5/6)

**Implement `renderDoctorAdvice(w io.Writer, report *doctor.Report, destPath string)`:**

1. For each section (ERRORS, SKIPPED, DUPLICATES) with total > 0:
   - Print section header: `"ERRORS (<N> files)"` (bold if TTY).
   - For each category in the section:
     - Print category title with count: `"  <Name> (<N> files)"`.
     - Print category description (from `Category.Description`), indented 2 spaces, word-wrapped to ~76 chars.
   - Print suggested actions at bottom of section:
     - Errors: `"→ Re-run sort to retry all <N> errored files (they are NOT marked as processed)."` and `"→ Or run: pixe retry -d <dest>"` (only if `destPath` is non-empty).
     - Skipped: no suggested actions (informational).
     - Duplicates: `"→ To review: pixe query duplicates -d <dest> --list"` (only if `destPath` is non-empty).
   - Print blank line between sections.

2. The `destPath` parameter comes from:
   - `--dest` flag value (if provided), or
   - `ledger.Header.Destination` (if in ledger mode), or
   - empty string (suggested commands omit the `-d` flag).

**Word wrapping:** A simple `wrapText(text string, indent int, width int) string` helper. No external dependency — just split on spaces and accumulate lines. Width defaults to 76.

---

### Task 8: Extend pipeline to accept pre-built file list

**File:** `internal/pipeline/pipeline.go`

**Add new field to `SortOptions`:**
```go
type SortOptions struct {
    // ... existing fields ...

    // RetryFiles, when non-nil, bypasses discovery.Walk and processes only
    // these files. Used by the retry command to re-process errored files
    // without re-walking the source directory.
    RetryFiles []discovery.DiscoveredFile
}
```

**Modify `Run()` function:**

At the discovery step (currently line ~198), add a branch:

```go
var discovered []discovery.DiscoveredFile
var skipped []discovery.SkippedFile

if opts.RetryFiles != nil {
    discovered = opts.RetryFiles
    // No skipped files in retry mode — we already know what to process.
} else {
    discovered, skipped, err = discovery.Walk(dirA, opts.Registry, walkOpts)
    if err != nil {
        return SortResult{}, fmt.Errorf("pipeline: walk: %w", err)
    }
}
```

**No other changes to `Run()`:** The rest of the pipeline (batch insert, process loop, ledger, summary) works the same regardless of how `discovered` was populated. The `CheckSourceProcessed` call (line ~361) will correctly skip files that were already completed in a prior run (e.g., if the user runs retry twice).

**Sidecar handling in retry mode:** `RetryFiles` entries need their `Sidecars` field populated. The retry command (Task 9) must re-discover sidecars for each file. This can be done by calling `discovery.AssociateSidecars()` if it's exported, or by constructing `DiscoveredFile` entries with empty `Sidecars` (sidecars are non-fatal — the file will still be processed, just without sidecar carry). The simpler approach (empty sidecars) is acceptable for v1 of retry.

---

### Task 9: Create `cmd/retry.go` — targeted error retry

**New file:** `cmd/retry.go`

**Command registration:**
```go
var retryCmd = &cobra.Command{
    Use:   "retry",
    Short: "Retry errored files from a previous sort run",
    Long:  "Re-process only the files that failed, had mismatches, or had tag failures " +
           "in a specific run. Creates a new run for auditability.",
    RunE:  runRetry,
}
```

**Flags (in `init()`):**
```go
rootCmd.AddCommand(retryCmd)

retryCmd.Flags().StringP("dest", "d", "", "archive directory (required)")
retryCmd.Flags().StringP("source", "s", "", "source directory (for scoping to most recent run)")
retryCmd.Flags().String("run", "", "specific run ID (prefix match)")
retryCmd.Flags().Bool("dry-run", false, "preview what would be retried")
retryCmd.Flags().IntP("workers", "w", 0, "concurrent workers")
retryCmd.Flags().Bool("progress", false, "show progress bar")

_ = viper.BindPFlag("retry_dest", retryCmd.Flags().Lookup("dest"))
_ = viper.BindPFlag("retry_db_path", retryCmd.Flags().Lookup("db-path"))  // if added
_ = viper.BindPFlag("retry_workers", retryCmd.Flags().Lookup("workers"))
```

**`runRetry` logic:**

1. **Resolve dest:** `resolveDest("retry_dest")`. Error if empty.
2. **Open DB** (read-write — retry creates a new run): follow `resume.go` pattern. Use `archivedb.Open()`, not `OpenReadOnly()`.
3. **Resolve target run:**
   - If `--run` provided: `db.GetRunByPrefix(prefix)` → validate single match → `targetRun = runs[0]`.
   - If `--source` provided: `db.MostRecentRunBySource(absSourceDir)`.
   - Otherwise: `db.MostRecentRun()`.
   - If no run found: return error `"no completed runs found"`.
4. **Query errored files:** `db.FilesWithErrorsByRun(targetRun.ID)`.
5. **Filter to error statuses only:** The `FilesWithErrorsByRun` already filters to `failed`, `mismatch`, `tag_failed`.
6. **Validate source files exist:**
   ```go
   var retryFiles []discovery.DiscoveredFile
   var missing int
   for _, f := range errorFiles {
       if _, err := os.Stat(f.SourcePath); os.IsNotExist(err) {
           fmt.Fprintf(out, "SKIP %s → source file no longer exists\n", filepath.Base(f.SourcePath))
           missing++
           continue
       }
       handler := registry.DetectFile(f.SourcePath)  // re-detect handler
       if handler == nil {
           fmt.Fprintf(out, "SKIP %s → no handler found\n", filepath.Base(f.SourcePath))
           missing++
           continue
       }
       retryFiles = append(retryFiles, discovery.DiscoveredFile{
           Path:    f.SourcePath,
           RelPath: relPath(f.SourcePath, targetRun.Source),
           Handler: handler,
       })
   }
   ```
7. **If no files to retry:** print `"No files to retry."` and return nil.
8. **If `--dry-run`:** print each file that would be retried and return.
9. **Build config** from target run (follow `resume.go` pattern, lines 155-170):
   - `Source` = `targetRun.Source`
   - `Destination` = resolved dest
   - `Algorithm` = `targetRun.Algorithm`
   - `Workers` = `resolveWorkers("retry_workers")`
   - `Recursive` = `targetRun.Recursive`
   - Other settings from current Viper config.
10. **Build `SortOptions`** with `RetryFiles: retryFiles`.
11. **Call `pipeline.Run(opts)`.**
12. **Handle result:** same pattern as `sort.go` — exit code 1 if errors > 0.

**Tag-failed optimization (future):** For `tag_failed` files, the file is already copied and verified. Ideally, retry would skip directly to the tag stage. For v1, the full pipeline re-processes these files (the copy stage will detect the existing destination and either skip or overwrite). This is safe but slightly wasteful. A future optimization could add a `RetryStage` field to `DiscoveredFile` or `SortOptions` to indicate where to resume.

---

### Task 10: Add inline post-sort hints to `cmd/sort.go`

**File:** `cmd/sort.go`

**Location:** After the pipeline returns `result` (currently around line 249) and before the error check (line 252). The summary is printed by the pipeline itself (in `pipeline.go` lines 283-285), so hints must be printed in `sort.go` after `pipeline.Run()` returns.

**Implementation:**

```go
result, err := pipeline.Run(opts)
if err != nil {
    return err
}

// Print contextual hints (only when interactive, not quiet, not progress mode)
if !cfg.Quiet && !useProgress && isTerminal(os.Stdout) {
    printSortHints(os.Stdout, result, cfg.Destination)
}

if result.Errors > 0 {
    return fmt.Errorf("%d file(s) failed to process — check output above", result.Errors)
}
```

**`printSortHints` function (in `cmd/sort.go`):**

```go
func printSortHints(w io.Writer, result pipeline.SortResult, dest string) {
    if result.Errors == 0 && result.Skipped == 0 {
        return // clean run — no hints
    }
    fmt.Fprintln(w) // blank line after summary
    if result.Errors > 0 {
        fmt.Fprintf(w, "  %d files had errors. Run pixe doctor for a summary, or pixe doctor --advice for help.\n", result.Errors)
        fmt.Fprintf(w, "  To retry just the errors: pixe retry -d %s\n", dest)
    } else if result.Skipped > 0 {
        fmt.Fprintf(w, "  %d files skipped. Run pixe doctor for a summary.\n", result.Skipped)
    }
}
```

**`isTerminal` helper:** Check if `w` is a TTY. Use `golang.org/x/term` if already a dependency, or `os.Stdout.Fd()` with a syscall check. Check what the codebase already uses for TTY detection and reuse that pattern.

**Suppression rules:**
- `--quiet`: `cfg.Quiet` is true → no hints.
- `--progress`: `useProgress` is true → no hints (progress UI has its own completion display).
- Piped output: `isTerminal(os.Stdout)` is false → no hints.

---

### Task 11: Tests for `internal/doctor/`

**New file:** `internal/doctor/doctor_test.go`

**Test cases for `Classify`:**
- `TestClassify_extractDateError` — status `"error"`, reason `"extract date: invalid EXIF IFD"` → "Corrupted metadata".
- `TestClassify_hashPayloadError` — status `"error"`, reason `"hash payload: unexpected EOF"` → "Corrupt file structure".
- `TestClassify_permissionDenied` — status `"error"`, reason `"copy: open /path: permission denied"` → "Disk / permission error".
- `TestClassify_mismatchStatus` — status `"mismatch"`, reason `""` → "Integrity mismatch".
- `TestClassify_tagFailedStatus` — status `"tag_failed"`, reason `""` → "Tag injection failed".
- `TestClassify_unsupportedFormat` — status `"skip"`, reason `"unsupported format: .txt"` → "Unsupported format".
- `TestClassify_previouslyImported` — status `"skip"`, reason `"previously imported"` → "Previously imported".
- `TestClassify_outsideDateRange` — status `"skip"`, reason `"outside date range: before 2023-01-01"` → "Outside date range".
- `TestClassify_unknownError` — status `"error"`, reason `"something completely new"` → "Uncategorized".
- `TestClassify_dbStatuses` — status `"failed"`, `"skipped"` (DB forms) map correctly.

**Test cases for `Summarize`:**
- `TestSummarize_mixedEntries` — mix of errors, skips, duplicates → correct counts per category.
- `TestSummarize_emptyInput` — no entries → all section totals are 0.
- `TestSummarize_copyEntriesIgnored` — entries with status `"copy"` are not counted.
- `TestSummarize_zeroCategoriesOmitted` — categories with 0 files don't appear in the report.
- `TestSummarize_filesTracked` — each category's `Files` slice contains the correct paths.

**Pattern:** Table-driven tests with `want`/`got`. Use `t.Helper()` on helpers. Same package (`package doctor`).

---

### Task 12: Tests for new archivedb query methods

**File:** `internal/archivedb/queries_test.go` (extend existing file)

**Test cases:**
- `TestMostRecentRunBySource_found` — insert 3 runs with different sources, verify correct run returned.
- `TestMostRecentRunBySource_notFound` — no runs for source → `(nil, nil)`.
- `TestMostRecentRunBySource_ignoresRunning` — running (interrupted) runs are excluded... actually per the query they're included. Verify `'completed'` and `'interrupted'` are both returned.
- `TestMostRecentRun_found` — returns most recent across all sources.
- `TestMostRecentRun_empty` — no runs → `(nil, nil)`.
- `TestFilesWithErrorsByRun_filtered` — insert errors across 2 runs, verify filtering.
- `TestFilesWithErrorsByRun_emptyRunID` — empty string returns all errors (same as `FilesWithErrors()`).
- `TestAllSkippedByRun_filtered` — same pattern.
- `TestAllDuplicatesByRun_filtered` — same pattern.

**Pattern:** Use `openTestDB(t)` helper. Insert test data via `InsertRun` + `InsertFile` + `UpdateFileStatus`. Follow existing test patterns in the file.

---

### Task 13: Tests for `cmd/doctor.go`

**File:** `cmd/doctor_test.go`

**Test cases:**
- `TestDoctor_ledgerMode_summary` — create a temp dir with a `.pixe_ledger.json` containing errors/skips/duplicates. Run doctor. Verify summary output format.
- `TestDoctor_ledgerMode_noProblems` — ledger with only `"copy"` entries → "No problems found" message.
- `TestDoctor_ledgerMode_noLedger` — empty dir → "No ledger found" message.
- `TestDoctor_advice_showsDescriptions` — `--advice` flag → output contains category descriptions.
- `TestDoctor_advice_showsSuggestedActions` — `--advice` with dest in ledger header → output contains copy-pasteable commands.
- `TestDoctor_json_structure` — `--json` → valid JSON with expected structure.
- `TestDoctor_runWithoutDest_errors` — `--run` without `--dest` → error message.
- `TestDoctor_dbMode_summary` — (requires test DB) insert run + files, run doctor with `--dest` → correct summary.

**Pattern:** Use `t.TempDir()` for ledger files. For DB tests, use `openTestDB(t)` or create a temp DB. May need to execute the command via `cobra.Command.Execute()` or call `runDoctor` directly with a test command context.

---

### Task 14: Tests for `cmd/retry.go`

**File:** `cmd/retry_test.go` or `internal/integration/retry_test.go`

**Test cases:**
- `TestRetry_retriesFailedFiles` — insert a completed run with 2 failed files and 3 complete files. Run retry. Verify only the 2 failed files are in `RetryFiles`.
- `TestRetry_missingSourceSkipped` — errored file's source no longer exists → printed as SKIP, not passed to pipeline.
- `TestRetry_noErrors_exitsCleanly` — run with 0 errors → "No files to retry." message.
- `TestRetry_createsNewRun` — after retry, a new run exists in the DB with a different ID.
- `TestRetry_runScoping` — `--run` flag correctly selects the specified run.
- `TestRetry_sourceScoping` — `--source` flag finds most recent run for that source.
- `TestRetry_dryRun` — `--dry-run` lists files without processing.

**Note:** Full pipeline integration tests (actually copying files) belong in `internal/integration/`. Unit tests in `cmd/` can test the run-resolution and file-list-building logic by mocking or using a test DB.

---

### Task 15: Tests for `--list` and `--run` on query subcommands

**File:** Extend existing test files or create `cmd/query_list_test.go`

**Test cases:**
- `TestQueryErrors_list_outputFormat` — `--list` produces one path per line, no headers, no summary.
- `TestQueryErrors_list_andJson_mutuallyExclusive` — both flags → error.
- `TestQueryErrors_run_filtersCorrectly` — `--run` with a valid prefix → only that run's errors.
- `TestQueryErrors_run_ambiguous` — prefix matches multiple runs → error.
- `TestQueryErrors_run_notFound` — prefix matches nothing → error.
- `TestQuerySkipped_list_outputFormat` — same pattern for skipped.
- `TestQueryDuplicates_list_outputFormat` — same pattern for duplicates.
- `TestQueryDuplicates_list_withPairs` — `--list` takes precedence over `--pairs`.

---

### Task 16: Tests for inline post-sort hints

**File:** `cmd/sort_test.go` (extend existing) or `cmd/hints_test.go`

**Test cases:**
- `TestPrintSortHints_errorsPresent` — errors > 0 → hint includes "pixe doctor" and "pixe retry".
- `TestPrintSortHints_skippedOnly` — skipped > 0, errors = 0 → hint includes "pixe doctor" only.
- `TestPrintSortHints_cleanRun` — errors = 0, skipped = 0 → no output.
- `TestPrintSortHints_destInRetryCommand` — dest value appears in the suggested retry command.

**Note:** `printSortHints` writes to an `io.Writer`, so tests can use a `bytes.Buffer`. TTY detection and suppression logic is tested at the integration level or by verifying the conditional in `sort.go`.

---

### Task 17: Update docgen for new commands

**File:** `internal/docgen/` (extend existing extraction)

- Register `doctor` and `retry` commands in `buildTargets()` so their flags appear in `docs/commands.md`.
- Run `make docs` to regenerate.
- Verify `make docs-check` passes.

**Note:** This is low priority — the commands should be functional before documenting them.

---

### Task 18: Commit

Commit strategy TBD — either one feature commit per wave or a single commit for the entire feature set. Follow the repository's commit message conventions (see `git log` for style).
