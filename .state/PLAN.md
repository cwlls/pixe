# Pixe Implementation State

**Status:** `pixe query` command — expose archive database queries to end users (Architecture §7.3).

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add new database methods (`OpenReadOnly`, `AllSkipped`, `GetRunByPrefix`, `ArchiveStats`) | high | @developer | [ ] pending | — | Three new query methods + a read-only constructor in `internal/archivedb/` |
| 2 | Create parent command and shared formatting (`cmd/query.go`, `cmd/query_format.go`) | high | @developer | [ ] pending | 1 | Cobra parent command with `PersistentPreRunE` for DB setup; shared table/JSON output helpers |
| 3 | Implement query subcommands (`runs`, `run`, `duplicates`, `errors`, `skipped`, `files`, `inventory`) | high | @developer | [ ] pending | 2 | Seven subcommand files in `cmd/`, one per query type |
| 4 | Tests and verification | high | @tester | [ ] pending | 3 | Unit tests for new DB methods, integration tests for CLI subcommands, `make check && make lint` |
| 5 | Commit `pixe query` feature | low | @committer | [ ] pending | 4 | `feat: add pixe query command for archive database interrogation` |

---

# Pixe Task Descriptions

## Task 1 — New database methods

**Files:** `internal/archivedb/archivedb.go`, `internal/archivedb/queries.go`

All additions follow existing patterns in `queries.go`. Every new method gets a corresponding test.

### 1a. `OpenReadOnly(path string) (*DB, error)`

Add to `archivedb.go`. A new constructor for read-only access:

- **Do not** call `os.MkdirAll` (the DB must already exist).
- **Do not** call `applySchema()` (read-only callers should never create tables).
- Check `os.Stat(path)` first; if the file does not exist, return a clear error: `fmt.Errorf("archivedb: database not found: %s", path)`.
- Open with `sql.Open("sqlite", path+"?mode=ro")` to enforce read-only at the driver level.
- Still call `applyPragmas()` (WAL mode and busy timeout are safe and beneficial for readers).
- `conn.SetMaxOpenConns(1)` as with `Open()`.

```go
func OpenReadOnly(path string) (*DB, error) {
    if _, err := os.Stat(path); err != nil {
        return nil, fmt.Errorf("archivedb: database not found: %s", path)
    }
    conn, err := sql.Open("sqlite", path+"?mode=ro")
    if err != nil {
        return nil, fmt.Errorf("archivedb: open database read-only: %w", err)
    }
    conn.SetMaxOpenConns(1)
    db := &DB{conn: conn, path: path}
    if err := db.applyPragmas(); err != nil {
        _ = conn.Close()
        return nil, err
    }
    return db, nil
}
```

**Tests** (`archivedb_test.go`):
- `TestOpenReadOnly_notFound` — non-existent path returns error containing `"database not found"`.
- `TestOpenReadOnly_success` — create a DB with `Open()`, close it, reopen with `OpenReadOnly()`, call `ListRuns()` to confirm reads work.

### 1b. `AllSkipped() ([]*FileRecord, error)`

Add to `queries.go`. Exact clone of `AllDuplicates()` with a different WHERE clause:

```go
func (db *DB) AllSkipped() ([]*FileRecord, error) {
    const q = `
        SELECT id, run_id, source_path, dest_path, dest_rel, checksum,
               status, is_duplicate, capture_date, file_size,
               extracted_at, hashed_at, copied_at, verified_at, tagged_at, error,
               skip_reason
        FROM files
        WHERE status = 'skipped'
        ORDER BY id`

    rows, err := db.conn.Query(q)
    if err != nil {
        return nil, fmt.Errorf("archivedb: all skipped: %w", err)
    }
    defer func() { _ = rows.Close() }()

    return scanFileRows(rows)
}
```

**Test** (`queries_test.go`): Mirror `TestAllDuplicates` — insert files with `status = 'skipped'` and `skip_reason` set, verify they are returned. Insert files with other statuses, verify they are excluded.

### 1c. `GetRunByPrefix(prefix string) ([]*Run, error)`

Add to `queries.go`. Returns all runs whose ID starts with `prefix`:

```go
func (db *DB) GetRunByPrefix(prefix string) ([]*Run, error) {
    const q = `
        SELECT id, pixe_version, source, destination, algorithm, workers,
               recursive, started_at, finished_at, status
        FROM runs
        WHERE id LIKE ?
        ORDER BY started_at DESC`

    rows, err := db.conn.Query(q, prefix+"%")
    if err != nil {
        return nil, fmt.Errorf("archivedb: get run by prefix: %w", err)
    }
    defer func() { _ = rows.Close() }()

    var runs []*Run
    for rows.Next() {
        r, err := scanRun(rows)
        if err != nil {
            return nil, err
        }
        runs = append(runs, r)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("archivedb: get run by prefix iterate: %w", err)
    }
    return runs, nil
}
```

Uses the existing `scanRun` helper from `runs.go` (the one used by `GetRun`). If `scanRun` currently only accepts `*sql.Row`, refactor it to accept a `scanner` interface (`Scan(...any) error`) so it works with both `*sql.Row` and `*sql.Rows`.

**Tests** (`queries_test.go`):
- Insert 3 runs with IDs `"aaaa1111-..."`, `"aaaa2222-..."`, `"bbbb3333-..."`.
- `GetRunByPrefix("aaaa")` → returns 2 runs.
- `GetRunByPrefix("aaaa1")` → returns 1 run.
- `GetRunByPrefix("cccc")` → returns 0 runs (empty slice, no error).

### 1d. `ArchiveStats() (*ArchiveStats, error)`

Add to `queries.go`. New type + aggregate query:

```go
type ArchiveStats struct {
    TotalFiles      int
    Complete        int
    Duplicates      int
    Failed          int
    Mismatches      int
    TagFailed       int
    Skipped         int
    TotalSize       int64
    RunCount        int
    EarliestCapture *time.Time
    LatestCapture   *time.Time
}
```

Implementation uses two queries (simpler and more readable than a single complex query):

**Query 1 — file stats:**
```sql
SELECT
    COUNT(*) AS total_files,
    SUM(CASE WHEN status = 'complete' AND is_duplicate = 0 THEN 1 ELSE 0 END),
    SUM(CASE WHEN is_duplicate = 1 THEN 1 ELSE 0 END),
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END),
    SUM(CASE WHEN status = 'mismatch' THEN 1 ELSE 0 END),
    SUM(CASE WHEN status = 'tag_failed' THEN 1 ELSE 0 END),
    SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END),
    COALESCE(SUM(file_size), 0),
    MIN(capture_date),
    MAX(capture_date)
FROM files
```

**Query 2 — run count:**
```sql
SELECT COUNT(*) FROM runs
```

**Test** (`queries_test.go`): Insert a run with files in various statuses (3 complete, 2 duplicate, 1 failed, 1 mismatch, 1 skipped) with known `file_size` and `capture_date` values. Verify all fields of the returned `ArchiveStats`.

---

## Task 2 — Parent command and shared formatting

**Files:** `cmd/query.go`, `cmd/query_format.go`

### 2a. `cmd/query.go` — Parent command

Apache 2.0 header. Package `cmd`.

```go
var (
    queryDB  *archivedb.DB   // set by PersistentPreRunE, used by subcommands
    queryDir string          // resolved absolute path to dirB
    jsonOut  bool            // --json flag
)

var queryCmd = &cobra.Command{
    Use:   "query",
    Short: "Query the archive database",
    Long:  `Read-only interrogation of the archive database. ...`,
    PersistentPreRunE: openQueryDB,
    PersistentPostRunE: closeQueryDB,
}
```

**`openQueryDB`** logic:
1. Read `--dir` flag (required). Resolve to absolute path. Validate it exists and is a directory.
2. Read `--db-path` flag (optional).
3. Call `dblocator.Resolve(dir, dbPath)` — same as `sort` and `resume`.
4. Call `archivedb.OpenReadOnly(loc.DBPath)`. If error contains `"database not found"`, print: `Error: no archive database found for <dirB>. Run 'pixe sort' first to create one.`
5. Store in `queryDB` and `queryDir`.

**`closeQueryDB`** logic: `queryDB.Close()`.

**Flags** (persistent, inherited by all subcommands):
- `--dir` / `-d` (string, required) — bound to Viper key `query_dir` (avoids collision with `resume_dir`).
- `--db-path` (string, optional) — bound to Viper key `query_db_path`.
- `--json` (bool, default false) — stored in `jsonOut` package var.

**`init()`**: `rootCmd.AddCommand(queryCmd)`, register flags, bind to Viper.

### 2b. `cmd/query_format.go` — Shared output helpers

Apache 2.0 header. Package `cmd`.

Two core helpers used by all subcommands:

```go
// queryResult is the top-level JSON envelope for --json output.
type queryResult struct {
    Query   string `json:"query"`
    Dir     string `json:"dir"`
    Results any    `json:"results"`
    Summary any    `json:"summary"`
}

// printQueryJSON marshals a queryResult to stdout as indented JSON.
func printQueryJSON(w io.Writer, query string, results any, summary any) error {
    qr := queryResult{
        Query:   query,
        Dir:     queryDir,
        Results: results,
        Summary: summary,
    }
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(qr)
}

// printQueryTable prints a fixed-width columnar table with a summary line.
// headers: column names (uppercase). rows: one []string per row.
// summary: printed after a blank line separator.
func printQueryTable(w io.Writer, headers []string, rows [][]string, summary string) {
    // 1. Compute column widths (max of header width and all row values).
    // 2. Print header row with padding.
    // 3. Print each data row with padding.
    // 4. Print blank line + summary.
    // If len(rows) == 0, print nothing (caller handles empty message).
}

// truncChecksum returns the first 8 characters of a checksum for table display.
func truncChecksum(s string) string

// truncID returns the first 8 characters of a UUID for table display.
func truncID(s string) string

// formatDate formats a *time.Time as "YYYY-MM-DD" for table display, or "—" if nil.
func formatDate(t *time.Time) string

// formatDateTime formats a time.Time as "YYYY-MM-DD HH:MM:SS" for table display.
func formatDateTime(t time.Time) string

// commaInt formats an integer with comma separators (e.g., 1,247).
func commaInt(n int) string
```

---

## Task 3 — Query subcommands

**Files:** `cmd/query_runs.go`, `cmd/query_run.go`, `cmd/query_duplicates.go`, `cmd/query_errors.go`, `cmd/query_skipped.go`, `cmd/query_files.go`, `cmd/query_inventory.go`

All files: Apache 2.0 header, package `cmd`. Each follows the same structure:

1. Define `var <name>Cmd = &cobra.Command{...}` with `RunE`.
2. `init()` registers on `queryCmd` and defines subcommand-specific flags.
3. `RunE` calls the appropriate `queryDB` method, formats output via the shared helpers.

### 3a. `cmd/query_runs.go` — `pixe query runs`

- `Use: "runs"`, no additional flags.
- Calls `queryDB.ListRuns()`.
- Empty: `fmt.Fprintln(out, "No runs found.")`, return nil.
- Table columns: `RUN ID` (8-char), `VERSION`, `SOURCE`, `STARTED`, `STATUS`, `FILES`.
- Summary: `"N runs | M total files"` (sum `FileCount` across all results).
- JSON results: define a local struct with `json:"..."` tags mapping `RunSummary` fields. Include full UUID in `id` field.

### 3b. `cmd/query_run.go` — `pixe query run <id>`

- `Use: "run"`, `Args: cobra.ExactArgs(1)`.
- Call `queryDB.GetRunByPrefix(args[0])`.
  - 0 matches → `return fmt.Errorf("no run found matching prefix %q", args[0])`.
  - \>1 matches → `return fmt.Errorf("ambiguous prefix %q matches %d runs: %s", args[0], len(runs), joinIDs(runs))`.
  - 1 match → proceed with `runs[0].ID`.
- Call `queryDB.GetRun(fullID)` for run metadata.
- Call `queryDB.GetFilesByRun(fullID)` for file list.
- Table mode: print a header block (key-value pairs for run metadata), then a file table with columns `SOURCE FILE` (basename only), `STATUS`, `DESTINATION`, `CHECKSUM` (8-char), `CAPTURE DATE`.
- Summary: count files by status → `"N files | X complete | Y duplicates | Z skipped | W errors"`.
- JSON: `{"query":"run","dir":"...","results":{"run":{...},"files":[...]},"summary":{...}}`.

### 3c. `cmd/query_duplicates.go` — `pixe query duplicates`

- `Use: "duplicates"`, flag `--pairs` (bool).
- Without `--pairs`: call `queryDB.AllDuplicates()`. Table columns: `SOURCE PATH`, `DESTINATION`, `CHECKSUM` (8-char), `CAPTURE DATE`. Summary: `"N duplicates"`.
- With `--pairs`: call `queryDB.DuplicatePairs()`. Table columns: `DUPLICATE SOURCE`, `DUPLICATE DEST`, `ORIGINAL`. Summary: `"N duplicate pairs"`.
- Empty: `"No duplicates found."`

### 3d. `cmd/query_errors.go` — `pixe query errors`

- `Use: "errors"`, no additional flags.
- Calls `queryDB.FilesWithErrors()`.
- Table columns: `SOURCE PATH`, `STATUS`, `ERROR`, `RUN SOURCE`.
- Summary: count by status → `"N errors | X failed | Y mismatch | Z tag_failed"`.
- Empty: `"No errors found."`

### 3e. `cmd/query_skipped.go` — `pixe query skipped`

- `Use: "skipped"`, no additional flags.
- Calls `queryDB.AllSkipped()`.
- Table columns: `SOURCE PATH`, `REASON` (from `SkipReason` field; display `"—"` if nil).
- Summary: count by reason prefix → `"N skipped files | X unsupported format | Y previously imported"`. Use `strings.HasPrefix(reason, "unsupported")` and `strings.HasPrefix(reason, "previously")` for bucketing.
- Empty: `"No skipped files found."`

### 3f. `cmd/query_files.go` — `pixe query files`

- `Use: "files"`, flags: `--from`, `--to`, `--imported-from`, `--imported-to` (string), `--source` (string).
- **Validation in `RunE`:**
  - At least one flag must be set. If none → `return fmt.Errorf("at least one filter flag is required (--from/--to, --imported-from/--imported-to, or --source)")`.
  - `--from`/`--to` and `--imported-from`/`--imported-to` are mutually exclusive. If both sets present → error.
  - `--source` is exclusive with date flags. If combined → error.
- **Date parsing:** `time.Parse("2006-01-02", value)`. If only `--from` set, default `--to` to `time.Now()`. If only `--to` set, default `--from` to `time.Date(1900, 1, 1, ...)`. For `--imported-*`, same logic.
- **Routing:** call `FilesByCaptureDateRange`, `FilesByImportDateRange`, or `FilesBySource` based on which flags are set.
- Table columns: `SOURCE PATH`, `DESTINATION`, `CHECKSUM` (8-char), `CAPTURE DATE`, `STATUS`.
- Summary: `"N files | capture range: X to Y"` (derive from results min/max capture date).
- Empty: `"No files found."`

### 3g. `cmd/query_inventory.go` — `pixe query inventory`

- `Use: "inventory"`, no additional flags.
- Calls `queryDB.ArchiveInventory()`.
- Table columns: `DESTINATION`, `CHECKSUM`, `CAPTURE DATE`.
- Summary: `"N files | capture range: X to Y"` (derive from results min/max capture date).
- Empty: `"No files in archive."`

---

## Task 4 — Tests and verification

### 4a. Unit tests for new DB methods

**File:** `internal/archivedb/queries_test.go` (extend existing file)

Add tests for `AllSkipped`, `GetRunByPrefix`, `ArchiveStats` as described in Task 1. Use the existing `openTestDB(t)` helper and test patterns already in the file.

**File:** `internal/archivedb/archivedb_test.go` (extend existing file)

Add tests for `OpenReadOnly` as described in Task 1a.

### 4b. Integration tests for CLI subcommands

**File:** `internal/integration/query_test.go`

Test flow:
1. Create `t.TempDir()` as `dirB`.
2. Create `dirB/.pixe/` directory.
3. Open DB with `archivedb.Open(dirB/.pixe/pixe.db)`.
4. Insert synthetic data: 2 runs, files in all statuses (complete ×3, duplicate ×2, failed ×1, mismatch ×1, tag_failed ×1, skipped ×2) with realistic `source_path`, `dest_rel`, `checksum`, `capture_date`, `file_size`, `skip_reason`, `error` values.
5. Close DB.
6. For each subcommand, test both table and JSON output by executing the Cobra command tree directly (set `rootCmd` args and capture stdout). Alternatively, use `os/exec` against the built binary.
7. Verify table output: contains expected column headers, expected row count, summary line.
8. Verify JSON output: `json.Unmarshal` succeeds, top-level keys `query`/`dir`/`results`/`summary` present, `results` array length matches expected count.

### 4c. Lint and regression check

Run `make check` (fmt-check + vet + unit tests with `-race`). Run `make lint`. Ensure all new files have the Apache 2.0 copyright header. Verify no existing tests are broken.
