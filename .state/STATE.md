# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add `Recursive` and `Ignore` fields to `AppConfig` | high | Developer | ✅ Complete | — | Struct changes only; no behavior yet |
| 2 | Register `--recursive` and `--ignore` CLI flags | high | Developer | ✅ Complete | 1 | Cobra flag registration + Viper binding in `cmd/sort.go` |
| 3 | Add `StatusSkipped` to domain and `skip_reason` to DB schema | high | Developer | ✅ Complete | — | Domain const + schema v2 migration |
| 4 | Implement DB schema v2 migration | high | Developer | ✅ Complete | 3 | `recursive` on `runs`, `skip_reason`+`skipped` on `files` |
| 5 | Add `CheckSourceProcessed` query to archivedb | high | Developer | ✅ Complete | 4 | Skip-detection query by absolute `source_path` |
| 6 | Build the ignore-list matcher | high | Developer | ✅ Complete | 1 | New `internal/ignore` package with glob matching |
| 7 | Refactor `discovery.Walk` for recursive + ignore + skip output | high | Developer | ✅ Complete | 1, 6 | Controlled recursion, ignore filtering, structured skip returns |
| 8 | Upgrade `LedgerEntry` and `Ledger` to v3 | high | Developer | ✅ Complete | 3 | New fields: `Status`, `Reason`, `Matches`, `Recursive` |
| 9 | Refactor pipeline stdout output to COPY/SKIP/DUPE/ERR format | high | Developer | ✅ Complete | 3, 5, 7, 8 | Central formatting; all outcomes produce one line |
| 10 | Wire skip/dupe/err entries into ledger and DB | high | Developer | ✅ Complete | 8, 9 | Skipped + unsupported files get ledger entries + DB rows |
| 11 | Update `Run` struct and `InsertRun` for `recursive` column | medium | Developer | ✅ Complete | 4 | Propagate `cfg.Recursive` into the runs table |
| 12 | Update concurrent worker path (`worker.go`) for new output format | high | @developer | ✅ Complete | 9 | Mirror sequential changes in the concurrent coordinator |
| 13 | Tests: ignore-list matcher | high | Developer | ✅ Complete | 6 | Unit tests for glob matching, hardcoded ledger ignore |
| 14 | Tests: discovery.Walk with recursive + ignore | high | Developer | ✅ Complete | 7 | Integration tests with nested dirs, dotfiles, ignore patterns |
| 15 | Tests: pipeline output format (COPY/SKIP/DUPE/ERR) | high | Developer | ✅ Complete | 9, 10 | Capture stdout, verify exact format for each verb |
| 16 | Tests: ledger v3 serialization | medium | @tester | ✅ Complete | 10 | Round-trip JSON, verify all status/reason/matches fields |
| 17 | Tests: schema v2 migration from v1 DB | medium | Tester | ✅ Complete | 4 | Create v1 DB, run migration, verify new columns |
| 18 | Tests: recursive incremental run (skip previously imported) | medium | Tester | ✅ Complete | 9, 10 | Two-run scenario: flat then recursive, verify skips |

---

# Ledger JSONL Conversion

Convert the ledger from a single buffered JSON document to a streaming JSONL format. Entries are written one-per-line as the coordinator processes results, eliminating the in-memory `[]LedgerEntry` accumulation. See Architectural Overview Section 8.8 for the full spec.

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 19 | Introduce `LedgerHeader` type and bump to v4 | high | Developer | ✅ Complete | — | New struct for JSONL line 1; `LedgerEntry` unchanged |
| 20 | Build `LedgerWriter` in `internal/manifest` | high | Developer | ⬜ Pending | 19 | Open/write-header/append-entry/close; owns file handle + encoder |
| 21 | Remove `Ledger` struct and `SaveLedger`/`LoadLedger` | high | Developer | ⬜ Pending | 20 | Delete dead code; keep `LedgerEntry` and status constants |
| 22 | Wire `LedgerWriter` into sequential pipeline | high | Developer | ⬜ Pending | 20, 21 | Replace all `ledger.Files = append(...)` with `lw.WriteEntry(...)` |
| 23 | Wire `LedgerWriter` into concurrent pipeline | high | Developer | ⬜ Pending | 20, 21 | Same replacement in coordinator goroutine (`worker.go`) |
| 24 | Update `Run()` orchestrator for streaming lifecycle | high | Developer | ⬜ Pending | 22, 23 | Open writer at start, close at end; skip in dry-run |
| 25 | Rewrite `LoadLedger` as JSONL reader (test utility) | medium | Developer | ⬜ Pending | 20 | Line-by-line reader returning header + entries; used only in tests |
| 26 | Tests: `LedgerWriter` unit tests | high | Developer | ⬜ Pending | 20 | Write header, append entries, verify JSONL on disk |
| 27 | Tests: rewrite ledger round-trip tests for JSONL v4 | high | Developer | ⬜ Pending | 25, 26 | Rewrite all `TestLedger_v3_*` tests for new format |
| 28 | Tests: pipeline ledger tests for streaming | high | Developer | ⬜ Pending | 22, 23, 25 | Verify ledger file written correctly after sort runs |
| 29 | Tests: interrupted run produces partial valid JSONL | medium | Developer | ⬜ Pending | 20, 26 | Simulate partial write; verify header + N entries parseable |

---

# Pixe Task Descriptions

## Task 1 — Add `Recursive` and `Ignore` fields to `AppConfig`

**File:** `internal/config/config.go`

Add two new fields to the `AppConfig` struct:

```go
// Recursive, when true, causes discovery to descend into subdirectories
// of Source. Default is false (top-level only).
Recursive bool

// Ignore is a list of glob patterns for files to exclude from processing.
// Patterns are matched against the filename (and relative path in recursive
// mode) using filepath.Match semantics. The ledger file (.pixe_ledger.json)
// is always ignored regardless of this list — that is handled in the
// ignore package, not here.
Ignore []string
```

No behavioral changes — this task only adds the struct fields so downstream tasks can reference them.

---

## Task 2 — Register `--recursive` and `--ignore` CLI flags

**File:** `cmd/sort.go`

### Flag registration (in `init()`)

Add after the existing `db-path` flag registration:

```go
sortCmd.Flags().BoolP("recursive", "r", false, "recursively process subdirectories of --source")
sortCmd.Flags().StringArray("ignore", nil, `glob pattern for files to ignore (repeatable, e.g. --ignore "*.txt" --ignore ".DS_Store")`)
```

Use `StringArray` (not `StringSlice`) so that each `--ignore` flag occurrence is treated as a single pattern without comma-splitting.

### Viper binding (in `init()`)

```go
_ = viper.BindPFlag("recursive", sortCmd.Flags().Lookup("recursive"))
_ = viper.BindPFlag("ignore", sortCmd.Flags().Lookup("ignore"))
```

### Config resolution (in `runSort`)

Add to the `cfg` construction block:

```go
cfg := &config.AppConfig{
    // ... existing fields ...
    Recursive: viper.GetBool("recursive"),
    Ignore:    viper.GetStringSlice("ignore"),
}
```

### Pass to pipeline

Add `Recursive` and `Ignore` to `SortOptions` or ensure they flow through `cfg` (they already do via `Config`).

---

## Task 3 — Add `StatusSkipped` to domain and `skip_reason` to DB schema

### File: `internal/domain/pipeline.go`

Add a new status constant:

```go
// StatusSkipped means the file was intentionally not processed.
// The skip_reason field records why (e.g., "previously imported",
// "unsupported format: .txt").
StatusSkipped FileStatus = "skipped"
```

Update `IsTerminal()` to include `StatusSkipped`:

```go
func (s FileStatus) IsTerminal() bool {
    switch s {
    case StatusComplete, StatusFailed, StatusMismatch, StatusTagFailed, StatusSkipped:
        return true
    }
    return false
}
```

`StatusSkipped` is **not** an error, so `IsError()` remains unchanged.

### File: `internal/archivedb/files.go`

Add a new `UpdateOption`:

```go
// WithSkipReason sets the skip_reason field on a file status update.
func WithSkipReason(reason string) UpdateOption {
    return func(p *updateParams) { p.skipReason = &reason }
}
```

Add `skipReason *string` to the `updateParams` struct and handle it in `UpdateFileStatus`:

```go
if p.skipReason != nil {
    setClauses = append(setClauses, "skip_reason = ?")
    args = append(args, *p.skipReason)
}
```

Also add `SkipReason *string` to the `FileRecord` struct and update `scanFileRow` to read it.

---

## Task 4 — Implement DB schema v2 migration

**File:** `internal/archivedb/schema.go`

### Bump schema version

Change `schemaVersion` from `1` to `2`.

### Update DDL

Add `recursive` column to `runs` table DDL:

```sql
recursive     INTEGER NOT NULL DEFAULT 0,
```

Add `skip_reason` column and `'skipped'` to the `files` status CHECK constraint:

```sql
skip_reason   TEXT,
status        TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN (
        'pending', 'extracted', 'hashed', 'copied',
        'verified', 'tagged', 'complete',
        'failed', 'mismatch', 'tag_failed', 'duplicate',
        'skipped'
    )),
```

### Migration logic

Add a `migrateSchema` function called from `applySchema`. After creating tables (which uses `IF NOT EXISTS` and won't alter existing tables), check the current schema version:

```go
func (db *DB) migrateSchema() error {
    var currentVersion int
    err := db.conn.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&currentVersion)
    if err != nil {
        return err // schema_version table doesn't exist or is empty — fresh DB
    }
    if currentVersion >= schemaVersion {
        return nil // already up to date
    }

    // v1 → v2: add recursive to runs, skip_reason to files, expand CHECK
    if currentVersion < 2 {
        migrations := []string{
            `ALTER TABLE runs ADD COLUMN recursive INTEGER NOT NULL DEFAULT 0`,
            `ALTER TABLE files ADD COLUMN skip_reason TEXT`,
            // SQLite doesn't support ALTER CHECK — but the IF NOT EXISTS DDL
            // already has the expanded constraint for new DBs. For existing DBs,
            // the CHECK is not enforced on ALTER ADD COLUMN, and SQLite's CHECK
            // enforcement is per-row on INSERT/UPDATE, so the new 'skipped' value
            // will be accepted as long as the original CHECK is not re-validated.
            // In practice, SQLite does not retroactively enforce CHECK constraints
            // on existing rows, and the new status value is only used in new INSERTs.
        }
        for _, m := range migrations {
            if _, err := db.conn.Exec(m); err != nil {
                // Ignore "duplicate column" errors for idempotency.
                if !strings.Contains(err.Error(), "duplicate column") {
                    return fmt.Errorf("archivedb: migrate v1→v2: %w", err)
                }
            }
        }
        _, _ = db.conn.Exec(
            `INSERT INTO schema_version (version, applied_at) VALUES (?, ?)`,
            2, time.Now().UTC().Format(time.RFC3339),
        )
    }
    return nil
}
```

**Important SQLite note:** SQLite CHECK constraints are defined at table creation time and cannot be altered. The expanded CHECK in the DDL applies to **new databases**. For existing v1 databases, the original CHECK constraint remains. However, SQLite's CHECK enforcement is lenient — it validates on INSERT/UPDATE but the `'skipped'` value will be accepted because SQLite only enforces the CHECK that was defined when the table was created, and `ALTER TABLE ADD COLUMN` does not re-create the table. If this becomes an issue, the migration can recreate the table (copy data, drop, recreate, copy back), but this is unlikely to be needed.

Call `migrateSchema()` at the end of `applySchema()`.

---

## Task 5 — Add `CheckSourceProcessed` query to archivedb

**File:** `internal/archivedb/queries.go`

Add a method to check if a file (by absolute source path) has already been processed in any prior run:

```go
// CheckSourceProcessed returns true if a file with the given absolute source
// path exists in a terminal state (complete, duplicate, skipped) in any run.
// Used by the pipeline to decide whether to SKIP a file.
func (db *DB) CheckSourceProcessed(sourcePath string) (bool, error) {
    const q = `
        SELECT 1 FROM files
        WHERE source_path = ?
          AND status IN ('complete', 'duplicate')
        LIMIT 1`

    var exists int
    err := db.conn.QueryRow(q, sourcePath).Scan(&exists)
    if err == sql.ErrNoRows {
        return false, nil
    }
    if err != nil {
        return false, fmt.Errorf("archivedb: check source processed: %w", err)
    }
    return true, nil
}
```

Note: We check for `complete` and `duplicate` but **not** `skipped` — a file that was previously skipped (e.g., unsupported format) should be re-evaluated in case a new handler has been registered. A file that was previously `failed` should also be re-attempted.

---

## Task 6 — Build the ignore-list matcher

**New file:** `internal/ignore/ignore.go`

Create a small, focused package that encapsulates the ignore-list logic:

```go
package ignore

import (
    "path/filepath"
    "strings"
)

const ledgerFilename = ".pixe_ledger.json"

// Matcher holds compiled ignore patterns and provides a Match method.
type Matcher struct {
    patterns []string // user-configured glob patterns
}

// New creates a Matcher from user-configured patterns.
// The hardcoded ledger ignore (.pixe_ledger.json) is always active
// and does not need to be included in patterns.
func New(patterns []string) *Matcher {
    // Deduplicate and clean patterns.
    seen := make(map[string]bool, len(patterns))
    var clean []string
    for _, p := range patterns {
        p = strings.TrimSpace(p)
        if p != "" && !seen[p] {
            seen[p] = true
            clean = append(clean, p)
        }
    }
    return &Matcher{patterns: clean}
}

// Match returns true if the file should be ignored.
// filename is the base name of the file (e.g., "IMG_0001.jpg").
// relPath is the path relative to dirA (e.g., "vacation/IMG_0001.jpg").
// For top-level files, relPath == filename.
//
// The hardcoded ledger ignore is checked first (by filename match).
// Then each user pattern is matched against both the filename and the relPath.
func (m *Matcher) Match(filename, relPath string) bool {
    // Hardcoded: always ignore the ledger file, at any depth.
    if filename == ledgerFilename {
        return true
    }

    for _, pattern := range m.patterns {
        // Match against filename.
        if matched, _ := filepath.Match(pattern, filename); matched {
            return true
        }
        // Match against relative path (enables patterns like "subdir/*.tmp").
        if relPath != filename {
            if matched, _ := filepath.Match(pattern, relPath); matched {
                return true
            }
        }
    }
    return false
}
```

**New file:** `internal/ignore/ignore_test.go` — see Task 13.

---

## Task 7 — Refactor `discovery.Walk` for recursive + ignore + skip output

**File:** `internal/discovery/walk.go`

The current `Walk` function always recurses (uses `filepath.WalkDir`) and skips dotfiles inline. It needs to be refactored to:

1. **Respect the `recursive` flag** — when false, skip subdirectories (return `filepath.SkipDir` for any directory that isn't `dirA` itself).
2. **Apply the ignore matcher** before classification — ignored files are completely invisible (not in `discovered`, not in `skipped`).
3. **Return structured skip reasons** that match the architectural spec (`unsupported format: .<ext>`, `detection error: ...`).

### New signature

```go
// WalkOptions configures the discovery walk.
type WalkOptions struct {
    Recursive bool
    Ignore    *ignore.Matcher
}

// Walk discovers files in dirA, classifies each using the registry, and
// returns discovered files (with handlers) and skipped files (with reasons).
// Ignored files (matching the ignore list) are completely excluded from both slices.
func Walk(dirA string, reg *Registry, opts WalkOptions) (discovered []DiscoveredFile, skipped []SkippedFile, err error)
```

### Key changes inside the walk function

```go
// Skip subdirectories when not recursive.
if d.IsDir() {
    if path == dirA {
        return nil // always enter the root
    }
    if strings.HasPrefix(name, ".") {
        return filepath.SkipDir // always skip dot-directories
    }
    if !opts.Recursive {
        return filepath.SkipDir // non-recursive: skip all subdirs
    }
    return nil // recursive: descend
}

// Compute relative path from dirA.
relPath, _ := filepath.Rel(dirA, path)

// Apply ignore matcher (includes hardcoded ledger ignore).
if opts.Ignore != nil && opts.Ignore.Match(name, relPath) {
    return nil // completely invisible
}

// Skip dotfiles (e.g., .DS_Store) — these are not "ignored" (configurable),
// they are a hardcoded policy. But since the ignore list now handles
// .pixe_ledger.json, we can simplify: dotfiles that aren't caught by
// the ignore list are still skipped with a reason.
if strings.HasPrefix(name, ".") {
    skipped = append(skipped, SkippedFile{
        Path:   relPath,
        Reason: "dotfile",
    })
    return nil
}

// Classify...
handler, detErr := reg.Detect(path)
if detErr != nil {
    skipped = append(skipped, SkippedFile{
        Path:   relPath,
        Reason: fmt.Sprintf("detection error: %v", detErr),
    })
    return nil
}
if handler == nil {
    ext := filepath.Ext(name)
    skipped = append(skipped, SkippedFile{
        Path:   relPath,
        Reason: fmt.Sprintf("unsupported format: %s", ext),
    })
    return nil
}

discovered = append(discovered, DiscoveredFile{
    Path:    path,    // absolute, for file I/O
    RelPath: relPath, // relative, for display and ledger
    Handler: handler,
})
```

### Update `DiscoveredFile`

Add a `RelPath` field:

```go
type DiscoveredFile struct {
    Path    string                // absolute path for file I/O
    RelPath string                // relative path from dirA for display/ledger
    Handler domain.FileTypeHandler
}
```

### Update `SkippedFile`

Change `Path` to store the relative path (not absolute):

```go
type SkippedFile struct {
    Path   string // relative path from dirA
    Reason string // human-readable reason
}
```

### Callers

Update `pipeline.go` and `worker.go` to use `df.RelPath` for display and ledger entries instead of computing `relPath(dirA, df.Path)`.

---

## Task 8 — Upgrade `LedgerEntry` and `Ledger` to v3

**File:** `internal/domain/pipeline.go`

### Update `LedgerEntry`

Replace the current struct with a richer version that supports all four outcomes:

```go
// LedgerEntry records the outcome of a single file discovered in dirA.
// Every discovered file (except ignored files) gets one entry.
type LedgerEntry struct {
    Path        string     `json:"path"`                  // relative path from dirA
    Status      string     `json:"status"`                // "copy", "skip", "duplicate", "error"
    Checksum    string     `json:"checksum,omitempty"`    // hex hash (copy, duplicate)
    Destination string     `json:"destination,omitempty"` // relative path in dirB (copy, duplicate)
    VerifiedAt  *time.Time `json:"verified_at,omitempty"` // ISO 8601 UTC (copy only)
    Matches     string     `json:"matches,omitempty"`     // existing file path (duplicate only)
    Reason      string     `json:"reason,omitempty"`      // explanation (skip, error)
}
```

Note: `VerifiedAt` changes from `time.Time` to `*time.Time` so it can be omitted for non-copy entries.

### Update `Ledger`

```go
type Ledger struct {
    Version     int           `json:"version"`
    PixeVersion string        `json:"pixe_version"`
    RunID       string        `json:"run_id,omitempty"`
    PixeRun     time.Time     `json:"pixe_run"`
    Algorithm   string        `json:"algorithm"`
    Destination string        `json:"destination"`
    Recursive   bool          `json:"recursive"`
    Files       []LedgerEntry `json:"files"`
}
```

Changes: `Version` becomes `3`, `Recursive` field added.

### Ledger status constants

Add package-level constants for the four ledger status values to avoid magic strings:

```go
const (
    LedgerStatusCopy      = "copy"
    LedgerStatusSkip      = "skip"
    LedgerStatusDuplicate = "duplicate"
    LedgerStatusError     = "error"
)
```

---

## Task 9 — Refactor pipeline stdout output to COPY/SKIP/DUPE/ERR format

**File:** `internal/pipeline/pipeline.go` (sequential path)

### Output format function

Add a helper that formats the output line:

```go
// formatOutput returns a single stdout line for a file outcome.
// verb is one of "COPY", "SKIP", "DUPE", "ERR ".
// source is the relative path from dirA (displayed on the left).
// detail is the destination path or reason (displayed on the right).
func formatOutput(verb, source, detail string) string {
    return fmt.Sprintf("%s %s -> %s\n", verb, source, detail)
}
```

### Replace existing output calls

Current format: `"  COPY     %s → %s\n"` using `filepath.Base(df.Path)`
New format: `"COPY %s -> %s\n"` using `df.RelPath`

Current error format: `"  ERROR  %s: %v\n"` using `filepath.Base(df.Path)`
New format: `"ERR  %s -> %s\n"` using `df.RelPath` and the error message

### Add skip output for discovery-phase skips

After `discovery.Walk` returns, iterate over `skipped` files and emit `SKIP` lines:

```go
for _, sf := range skipped {
    _, _ = fmt.Fprint(out, formatOutput("SKIP", sf.Path, sf.Reason))
    // Also create ledger entries and DB rows for skipped files — see Task 10.
}
```

### Add skip output for previously-imported files

Before entering the per-file pipeline, check the archive DB:

```go
if db != nil {
    processed, err := db.CheckSourceProcessed(df.Path)
    if err != nil { /* handle */ }
    if processed {
        _, _ = fmt.Fprint(out, formatOutput("SKIP", df.RelPath, "previously imported"))
        // Record in ledger and DB — see Task 10.
        continue
    }
}
```

### Add DUPE output

When a duplicate is detected (after hashing), emit:

```go
_, _ = fmt.Fprint(out, formatOutput("DUPE", df.RelPath,
    fmt.Sprintf("matches %s", existingDest)))
```

Instead of the current `COPY` line (which is emitted before the dedup check is complete).

**Important sequencing change:** The current code emits `COPY` before the copy happens, then `ERROR` if it fails. The new design should emit the verb **after** the outcome is known:
- `COPY` after successful verify
- `DUPE` after dedup detection (the file is still copied to `duplicates/`)
- `ERR` after any failure

This means moving the output line from before `copypkg.Execute` to after the full pipeline completes for that file.

---

## Task 10 — Wire skip/dupe/err entries into ledger and DB

**File:** `internal/pipeline/pipeline.go`

### Skipped files (discovery phase)

For each `SkippedFile` returned by `Walk`:

1. **DB:** Insert a row with `status = 'skipped'` and `skip_reason = sf.Reason`.
2. **Ledger:** Append a `LedgerEntry{Path: sf.Path, Status: "skip", Reason: sf.Reason}`.
3. **Stdout:** Already handled in Task 9.

### Skipped files (previously imported)

For files that pass discovery but are found in the DB as already processed:

1. **DB:** Insert a row with `status = 'skipped'` and `skip_reason = "previously imported"`.
2. **Ledger:** Append a `LedgerEntry{Path: df.RelPath, Status: "skip", Reason: "previously imported"}`.

### Duplicate files

Update the existing duplicate handling to create a richer ledger entry:

```go
ledger.Files = append(ledger.Files, domain.LedgerEntry{
    Path:        df.RelPath,
    Status:      domain.LedgerStatusDuplicate,
    Checksum:    checksum,
    Destination: dupRelDest,
    Matches:     existingDest,
})
```

### Error files

Update error handling to create ledger entries:

```go
ledger.Files = append(ledger.Files, domain.LedgerEntry{
    Path:   df.RelPath,
    Status: domain.LedgerStatusError,
    Reason: err.Error(),
})
```

### Successful copies

Update the existing success path:

```go
ledger.Files = append(ledger.Files, domain.LedgerEntry{
    Path:        df.RelPath,
    Status:      domain.LedgerStatusCopy,
    Checksum:    checksum,
    Destination: relDest,
    VerifiedAt:  &verifiedAt,
})
```

### Ledger version

Set `ledger.Version = 3` and `ledger.Recursive = cfg.Recursive` in the ledger construction.

---

## Task 11 — Update `Run` struct and `InsertRun` for `recursive` column

**File:** `internal/archivedb/runs.go`

Add `Recursive bool` to the `Run` struct.

Update `InsertRun` query to include the `recursive` column:

```go
const q = `
    INSERT INTO runs (id, pixe_version, source, destination, algorithm, workers, recursive, started_at, status)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'running')`

_, err := db.conn.Exec(q,
    r.ID, r.PixeVersion, r.Source, r.Destination, r.Algorithm, r.Workers,
    boolToInt(r.Recursive), r.StartedAt.UTC().Format(time.RFC3339),
)
```

Add a `boolToInt` helper (or inline the conversion).

Update `scanRun` to read the `recursive` column. Since this column may not exist in v1 databases that haven't been migrated yet, handle gracefully (the migration in Task 4 adds it with `DEFAULT 0`).

**File:** `internal/pipeline/pipeline.go`

Set `run.Recursive = cfg.Recursive` when constructing the `Run` struct.

---

## Task 12 — Update concurrent worker path for new output format

**File:** `internal/pipeline/worker.go`

Mirror all changes from Task 9 and Task 10 in the concurrent code path:

1. **Output format:** Replace `"  COPY     %s → %s\n"` with `formatOutput("COPY", ...)` using `df.RelPath`.
2. **Error output:** Replace `"  ERROR  %s: %v\n"` with `formatOutput("ERR ", ...)`.
3. **Duplicate output:** Emit `DUPE` instead of `COPY` when `isDuplicate` is true.
4. **Ledger entries:** Use the new `LedgerEntry` struct with `Status`, `Reason`, `Matches` fields.
5. **Previously-imported skip:** Add the `CheckSourceProcessed` check before sending work items to workers (in the coordinator, before feeding `workCh`), or in the worker before extract. The coordinator approach is cleaner since it avoids sending unnecessary work.
6. **Discovery-phase skips:** Handle in the coordinator before the worker loop starts (same as sequential path).

### Coordinator changes

In `runConcurrentCtx`, after `Walk` returns:

```go
// Emit SKIP lines for discovery-phase skips and record in ledger/DB.
for _, sf := range skipped {
    _, _ = fmt.Fprint(out, formatOutput("SKIP", sf.Path, sf.Reason))
    ledger.Files = append(ledger.Files, domain.LedgerEntry{...})
    // DB insert for skipped file...
}
```

In the work-feeding goroutine, add the previously-imported check:

```go
for _, df := range discovered {
    if db != nil {
        processed, _ := db.CheckSourceProcessed(df.Path)
        if processed {
            _, _ = fmt.Fprint(out, formatOutput("SKIP", df.RelPath, "previously imported"))
            // Record in ledger + DB, decrement pendingCount or track separately
            continue
        }
    }
    workCh <- workItem{df: df, fileID: fileIDs[df.Path]}
}
```

**Note:** The `pendingCount` tracking needs adjustment since some discovered files may be skipped before entering the worker pool. Track `actualPending` separately.

---

## Task 13 — Tests: ignore-list matcher

**New file:** `internal/ignore/ignore_test.go`

Test cases:

| Test | Input | Expected |
|------|-------|----------|
| Hardcoded ledger ignore | `filename=".pixe_ledger.json"` | `true` |
| Ledger ignore at depth | `filename=".pixe_ledger.json", relPath="subdir/.pixe_ledger.json"` | `true` |
| Simple glob match | pattern `"*.txt"`, `filename="notes.txt"` | `true` |
| No match | pattern `"*.txt"`, `filename="photo.jpg"` | `false` |
| Exact filename match | pattern `".DS_Store"`, `filename=".DS_Store"` | `true` |
| Relative path match | pattern `"subdir/*.tmp"`, `relPath="subdir/cache.tmp"` | `true` |
| Empty patterns | no patterns, `filename="photo.jpg"` | `false` |
| Deduplication | duplicate patterns are collapsed | no panic, correct behavior |

---

## Task 14 — Tests: discovery.Walk with recursive + ignore

**File:** `internal/discovery/discovery_test.go` (extend existing)

Test cases:

| Test | Setup | Expected |
|------|-------|----------|
| Non-recursive skips subdirs | `dirA/a.jpg`, `dirA/sub/b.jpg` | discovered: `[a.jpg]`, `b.jpg` not seen |
| Recursive finds nested files | same setup, `recursive=true` | discovered: `[a.jpg, sub/b.jpg]` |
| Ignore pattern excludes file | `dirA/a.jpg`, `dirA/notes.txt`, ignore `"*.txt"` | discovered: `[a.jpg]`, `notes.txt` not in skipped |
| Ledger file always ignored | `dirA/.pixe_ledger.json`, `dirA/a.jpg` | discovered: `[a.jpg]`, ledger not in skipped |
| Dotfiles still skipped | `dirA/.DS_Store` (not in ignore list) | skipped with reason "dotfile" |
| RelPath populated correctly | `dirA/sub/c.jpg`, recursive | `df.RelPath == "sub/c.jpg"` |

---

## Task 15 — Tests: pipeline output format (COPY/SKIP/DUPE/ERR)

**File:** `internal/pipeline/pipeline_test.go` (extend existing)

Capture `opts.Output` into a `bytes.Buffer` and verify:

| Test | Scenario | Expected stdout line |
|------|----------|---------------------|
| Successful copy | Normal JPEG | `COPY IMG_0001.jpg -> 2021/12-Dec/20211225_...jpg` |
| Unsupported format | `.txt` file in dirA | `SKIP notes.txt -> unsupported format: .txt` |
| Previously imported | File already in DB | `SKIP IMG_0001.jpg -> previously imported` |
| Duplicate | Same checksum as existing | `DUPE IMG_0042.jpg -> matches 2022/02-Feb/...` |
| Pipeline error | Corrupt JPEG | `ERR  corrupt.jpg -> extract date: ...` |

Also verify the summary line format:
```
Done. processed=N duplicates=N skipped=N errors=N
```

---

## Task 16 — Tests: ledger v3 serialization

**File:** `internal/manifest/manifest_test.go` (extend existing)

Test round-trip serialization of a v3 ledger with all entry types:

1. Create a `Ledger` with `Version: 3`, `Recursive: true`, and entries for copy, skip, duplicate, and error.
2. `SaveLedger` → `LoadLedger`.
3. Verify all fields are preserved: `Status`, `Reason`, `Matches`, `Recursive`, `VerifiedAt` (nil for non-copy).
4. Verify JSON structure matches the spec (field names, omitempty behavior).

---

## Task 17 — Tests: schema v2 migration from v1 DB

**File:** `internal/archivedb/archivedb_test.go` (extend existing)

1. Create a v1 database (using the v1 DDL directly, without the `recursive` and `skip_reason` columns).
2. Insert a schema_version row with `version=1`.
3. Open the database with the new code (which calls `applySchema` + `migrateSchema`).
4. Verify: `runs` table has `recursive` column (query `PRAGMA table_info(runs)`).
5. Verify: `files` table has `skip_reason` column.
6. Verify: `schema_version` has a row with `version=2`.
7. Verify: existing data is intact.
8. Verify: idempotency — opening again doesn't error.

---

## Task 18 — Tests: recursive incremental run (skip previously imported)

**File:** `internal/integration/` or `internal/pipeline/pipeline_test.go`

End-to-end scenario:

1. **Setup:** Create `dirA` with `a.jpg` (top-level) and `sub/b.jpg` (nested).
2. **Run 1:** `pixe sort --source dirA --dest dirB` (non-recursive).
   - Verify: `a.jpg` copied, `sub/b.jpg` not seen.
   - Verify: ledger has 1 entry (copy).
3. **Run 2:** `pixe sort --source dirA --dest dirB --recursive`.
   - Verify: `a.jpg` skipped (`SKIP a.jpg -> previously imported`).
   - Verify: `sub/b.jpg` copied.
   - Verify: ledger has 2 entries (1 skip, 1 copy).
   - Verify: DB has `a.jpg` with status `skipped` in run 2, and `complete` in run 1.

---

## Task 19 — Introduce `LedgerHeader` type and bump to v4

**File:** `internal/domain/pipeline.go`

### Add `LedgerHeader` struct

Add a new struct to represent the JSONL header line (line 1 of the ledger). This replaces the run-level metadata that was previously part of the `Ledger` struct.

```go
// LedgerHeader is the first line of the JSONL ledger file.
// It contains run-level metadata; all subsequent lines are LedgerEntry objects.
type LedgerHeader struct {
    Version     int    `json:"version"`
    RunID       string `json:"run_id"`
    PixeVersion string `json:"pixe_version"`
    PixeRun     string `json:"pixe_run"`     // ISO 8601 UTC
    Algorithm   string `json:"algorithm"`
    Destination string `json:"destination"`
    Recursive   bool   `json:"recursive"`
}
```

**Design notes:**

- `PixeRun` is `string` (not `time.Time`) because the header is written once at the start and we want exact control over the ISO 8601 format — `time.Time` marshals to RFC 3339 by default which is fine, but a string avoids any ambiguity and matches the compact JSONL style.
- `RunID` is **not** `omitempty` — in v4 the run ID is always present (the archive DB is always active).
- `Version` is `4`.
- The `LedgerEntry` struct (lines 131-139) is **unchanged** — it already has the right fields and `omitempty` tags for compact JSONL output.
- The existing ledger status constants (`LedgerStatusCopy`, `LedgerStatusSkip`, `LedgerStatusDuplicate`, `LedgerStatusError`) are **unchanged**.

### Keep `LedgerEntry` as-is

No changes needed. The `LedgerEntry` struct is already self-contained — each entry has `path`, `status`, and optional fields with `omitempty`. This is exactly what a JSONL file entry line needs.

---

## Task 20 — Build `LedgerWriter` in `internal/manifest`

**File:** `internal/manifest/manifest.go`

### New type: `LedgerWriter`

Add a streaming JSONL writer that owns a file handle and a `json.Encoder`. The coordinator calls its methods to write entries one at a time.

```go
// LedgerWriter streams ledger entries to a JSONL file.
// The coordinator goroutine is the sole caller — no mutex needed.
type LedgerWriter struct {
    f   *os.File
    enc *json.Encoder
}

// NewLedgerWriter opens the ledger file for writing (truncating any existing
// content) and writes the header as line 1. The caller must call Close when
// the run completes.
func NewLedgerWriter(dirA string, header domain.LedgerHeader) (*LedgerWriter, error) {
    path := ledgerPath(dirA)
    f, err := os.Create(path) // truncate + create
    if err != nil {
        return nil, fmt.Errorf("manifest: open ledger: %w", err)
    }

    enc := json.NewEncoder(f)
    enc.SetEscapeHTML(false) // file paths may contain & < > in theory

    if err := enc.Encode(header); err != nil {
        f.Close()
        return nil, fmt.Errorf("manifest: write ledger header: %w", err)
    }

    return &LedgerWriter{f: f, enc: enc}, nil
}

// WriteEntry appends a single file entry as one JSON line.
func (lw *LedgerWriter) WriteEntry(entry domain.LedgerEntry) error {
    return lw.enc.Encode(entry)
}

// Close flushes and closes the underlying file.
func (lw *LedgerWriter) Close() error {
    return lw.f.Close()
}
```

**Design notes:**

- `json.Encoder.Encode()` writes compact JSON (no indentation) followed by a `\n`. This is exactly the JSONL format — one valid JSON object per line.
- No explicit `Flush` call is needed after each `Encode` — `json.Encoder` writes directly to the `os.File`, and the OS handles buffering. For crash safety, each `Encode` call produces a complete JSON line. If the process crashes mid-line, the partial line is the only data lost, and all prior lines are valid.
- `SetEscapeHTML(false)` avoids escaping `<`, `>`, `&` in file paths, keeping the output clean.
- The `LedgerWriter` is intentionally simple — no mutex, no buffering layer, no retry. The coordinator is the sole writer, and write errors are non-fatal (same as the current `SaveLedger` behavior).

### Keep `ledgerPath` and `ledgerFile` constant

The filename remains `.pixe_ledger.json`. The `ledgerPath` helper and `ledgerFile` constant are unchanged.

---

## Task 21 — Remove `Ledger` struct and `SaveLedger`/`LoadLedger`

**File:** `internal/domain/pipeline.go`

### Remove the `Ledger` struct (lines 143-152)

Delete the entire `Ledger` struct. It is replaced by `LedgerHeader` (Task 19) + streaming `LedgerEntry` writes (Task 20). After this task, the domain package exports:

- `LedgerHeader` (new, from Task 19)
- `LedgerEntry` (unchanged)
- `LedgerStatusCopy`, `LedgerStatusSkip`, `LedgerStatusDuplicate`, `LedgerStatusError` (unchanged)

The `Ledger` struct with its `Files []LedgerEntry` slice is gone.

**File:** `internal/manifest/manifest.go`

### Remove `SaveLedger` (lines 94-96)

Delete the function. Its sole production caller (pipeline.go line 178) will be replaced in Task 24.

### Remove `atomicWriteJSON` (lines 118-134)

Delete the function. It is only used by `SaveLedger` and `Save` (the legacy manifest saver). Check if `Save` still exists and has callers — if `Save` is still used by the migration path, keep `atomicWriteJSON` but make it unexported. If `Save` has no remaining callers, delete both.

### Keep `LoadLedger` temporarily

`LoadLedger` has no production callers but is used extensively in tests. It will be rewritten as a JSONL reader in Task 25. For now, leave it in place (it will fail to compile once `Ledger` is removed, which is the signal to implement Task 25).

**Practical note:** Tasks 21 and 25 should be done together in a single commit to keep the build green. The separation here is conceptual — "remove old code" vs. "add JSONL reader" — but they must land atomically.

---

## Task 22 — Wire `LedgerWriter` into sequential pipeline

**File:** `internal/pipeline/pipeline.go`

### Change `runSequential` signature

Replace the `ledger *domain.Ledger` parameter with `lw *manifest.LedgerWriter`:

```go
func runSequential(opts SortOptions, discovered []discovery.DiscoveredFile,
    skipped []discovery.SkippedFile,
    fileIDs map[string]int64, dirA, dirB string, out io.Writer,
    lw *manifest.LedgerWriter) SortResult
```

### Replace all `ledger.Files = append(...)` with `lw.WriteEntry(...)`

There are 4 append sites in `runSequential`:

1. **Discovery-phase skips** (line ~208):
   ```go
   // Before:
   ledger.Files = append(ledger.Files, domain.LedgerEntry{
       Path: sf.Path, Status: domain.LedgerStatusSkip, Reason: sf.Reason,
   })
   // After:
   _ = lw.WriteEntry(domain.LedgerEntry{
       Path: sf.Path, Status: domain.LedgerStatusSkip, Reason: sf.Reason,
   })
   ```

2. **Previously imported skips** (line ~241): Same pattern.

3. **processFile errors** (line ~260): Same pattern.

4. **processFile success** (line ~277):
   ```go
   // Before:
   ledger.Files = append(ledger.Files, *le)
   // After:
   _ = lw.WriteEntry(*le)
   ```

**Error handling:** `WriteEntry` errors are ignored (same as the current pattern where `SaveLedger` failure is a warning). The ledger is a receipt, not a critical data path — the archive DB is the source of truth.

### Handle nil `LedgerWriter`

In dry-run mode, `lw` will be `nil` (no ledger is written). Guard each `WriteEntry` call:

```go
if lw != nil {
    _ = lw.WriteEntry(entry)
}
```

Or introduce a helper method on `LedgerWriter` that is nil-safe:

```go
func (lw *LedgerWriter) WriteEntry(entry domain.LedgerEntry) error {
    if lw == nil {
        return nil
    }
    return lw.enc.Encode(entry)
}
```

The nil-safe method approach is cleaner — it avoids scattering nil checks at every call site.

---

## Task 23 — Wire `LedgerWriter` into concurrent pipeline

**File:** `internal/pipeline/worker.go`

### Change `RunConcurrent` signature

Replace `ledger *domain.Ledger` with `lw *manifest.LedgerWriter`:

```go
func RunConcurrent(opts SortOptions, discovered []discovery.DiscoveredFile,
    skipped []discovery.SkippedFile,
    fileIDs map[string]int64, dirA, dirB string, out io.Writer,
    lw *manifest.LedgerWriter) SortResult
```

### Replace all `ledger.Files = append(...)` with `lw.WriteEntry(...)`

There are 8 append sites in the coordinator goroutine of `runConcurrentCtx`. Each one follows the same transformation as Task 22:

1. **Discovery-phase skips** (line ~136)
2. **Previously imported skips** (line ~168)
3. **Extract/hash errors** (line ~241)
4. **Dedup check errors** (line ~259)
5. **Final result errors** (line ~291)
6. **Post-copy dedup race errors** (lines ~313, ~333)
7. **Duplicate success** (line ~365)
8. **Copy success** (line ~374)

All are in the coordinator goroutine (single-threaded `select` loop). No concurrency concerns.

The `lw.WriteEntry()` nil-safe pattern from Task 22 applies here too.

---

## Task 24 — Update `Run()` orchestrator for streaming lifecycle

**File:** `internal/pipeline/pipeline.go`

### Replace ledger construction + SaveLedger with LedgerWriter lifecycle

**Before** (current code, lines 157-181):
```go
ledger := &domain.Ledger{
    Version: 3, PixeVersion: opts.PixeVersion, RunID: opts.RunID, ...
}
// ... processing ...
if !cfg.DryRun {
    if err := manifest.SaveLedger(ledger, dirA); err != nil {
        _, _ = fmt.Fprintf(out, "WARNING: ...")
    }
}
```

**After:**
```go
// Open streaming ledger writer (nil in dry-run mode).
var lw *manifest.LedgerWriter
if !cfg.DryRun {
    header := domain.LedgerHeader{
        Version:     4,
        RunID:       opts.RunID,
        PixeVersion: opts.PixeVersion,
        PixeRun:     startedAt.UTC().Format(time.RFC3339),
        Algorithm:   opts.Hasher.Algorithm(),
        Destination: dirB,
        Recursive:   cfg.Recursive,
    }
    var err error
    lw, err = manifest.NewLedgerWriter(dirA, header)
    if err != nil {
        _, _ = fmt.Fprintf(out, "WARNING: could not open ledger in %s: %v\n", dirA, err)
        // lw remains nil — processing continues without a ledger.
    }
}

// ... pass lw to runSequential or RunConcurrent ...

if cfg.Workers > 1 {
    result = RunConcurrent(opts, discovered, skipped, fileIDs, dirA, dirB, out, lw)
} else {
    result = runSequential(opts, discovered, skipped, fileIDs, dirA, dirB, out, lw)
}

// Close the ledger writer.
if lw != nil {
    if err := lw.Close(); err != nil {
        _, _ = fmt.Fprintf(out, "WARNING: could not close ledger: %v\n", err)
    }
}
```

**Key changes:**
- The `*domain.Ledger` variable is gone entirely.
- The `manifest.SaveLedger(ledger, dirA)` call is gone entirely.
- The `LedgerWriter` is opened at the start (header written immediately) and closed at the end.
- If opening fails, `lw` stays `nil` and all `WriteEntry` calls are no-ops (nil-safe method).
- Dry-run mode: `lw` is never created, so no ledger file is opened or written.

---

## Task 25 — Rewrite `LoadLedger` as JSONL reader (test utility)

**File:** `internal/manifest/manifest.go`

### Replace `LoadLedger` with a JSONL-aware reader

`LoadLedger` has **no production callers** — it is used only in tests to verify the ledger was written correctly. Rewrite it to parse JSONL:

```go
// LedgerContents holds the parsed contents of a JSONL ledger file.
// Used by tests to verify ledger output.
type LedgerContents struct {
    Header  domain.LedgerHeader
    Entries []domain.LedgerEntry
}

// LoadLedger reads a JSONL ledger file and returns its parsed contents.
// Returns (nil, nil) if the file does not exist.
func LoadLedger(dirA string) (*LedgerContents, error) {
    path := ledgerPath(dirA)
    f, err := os.Open(path)
    if errors.Is(err, os.ErrNotExist) {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("manifest: open ledger: %w", err)
    }
    defer f.Close()

    dec := json.NewDecoder(f)
    result := &LedgerContents{}

    // Line 1: header
    if err := dec.Decode(&result.Header); err != nil {
        return nil, fmt.Errorf("manifest: decode ledger header: %w", err)
    }

    // Lines 2+: entries
    for dec.More() {
        var entry domain.LedgerEntry
        if err := dec.Decode(&entry); err != nil {
            return nil, fmt.Errorf("manifest: decode ledger entry: %w", err)
        }
        result.Entries = append(result.Entries, entry)
    }

    return result, nil
}
```

**Design notes:**

- Returns a `*LedgerContents` instead of `*domain.Ledger` — the old `Ledger` struct no longer exists.
- `json.Decoder` reads JSONL naturally — each `Decode` call reads one JSON value (one line).
- `dec.More()` returns `true` as long as there's more data to read.
- All test callers need to be updated to use `LedgerContents` instead of `domain.Ledger` — the field access changes from `ledger.Files[i]` to `lc.Entries[i]` and `ledger.Version` to `lc.Header.Version`.

---

## Task 26 — Tests: `LedgerWriter` unit tests

**File:** `internal/manifest/manifest_test.go`

### New tests for the streaming writer

| Test | What it verifies |
|------|-----------------|
| `TestLedgerWriter_headerOnly` | Open writer, write header, close. File has exactly 1 line. Parse it back as `LedgerHeader`. Verify all fields. |
| `TestLedgerWriter_headerAndEntries` | Write header + 3 entries (copy, skip, error). File has 4 lines. Parse all back. Verify entry order and fields. |
| `TestLedgerWriter_omitempty` | Write a skip entry. Read raw bytes. Verify `"checksum"`, `"destination"`, `"verified_at"`, `"matches"` are absent from that line. |
| `TestLedgerWriter_compactJSON` | Verify no indentation — each line is a single compact JSON object (no `\n` within the JSON, no spaces after `:` beyond what `json.Encoder` produces). |
| `TestLedgerWriter_nilSafe` | Call `WriteEntry` on a nil `*LedgerWriter`. Verify no panic, returns nil error. |
| `TestLedgerWriter_version4` | Write header. Verify `"version":4` appears in line 1. |

---

## Task 27 — Tests: rewrite ledger round-trip tests for JSONL v4

**File:** `internal/manifest/manifest_test.go`

### Rewrite existing ledger tests

Every existing `TestLedger_v3_*` test must be rewritten for the v4 JSONL format. The test structure changes from "SaveLedger → LoadLedger → assert fields" to "NewLedgerWriter → WriteEntry × N → Close → LoadLedger → assert fields".

| Old Test | New Test | Key Changes |
|----------|----------|-------------|
| `TestLedger_SaveLoad_roundtrip` | `TestLedger_v4_roundtrip` | Use `NewLedgerWriter` + `WriteEntry` + `LoadLedger`. Assert `Header.Version == 4`. |
| `TestLedger_Load_notExist` | `TestLedger_Load_notExist` | Unchanged — `LoadLedger` still returns `(nil, nil)`. |
| `TestLedger_Save_atomic_noTmpLeftover` | **Delete** | No `.tmp` file is used in the streaming approach. |
| `TestLedger_v3_roundtrip` | `TestLedger_v4_fullRoundtrip` | Write all 4 entry types via `LedgerWriter`. Load back. Verify header + 4 entries. |
| `TestLedger_v3_copyEntry` | `TestLedger_v4_copyEntry` | Same assertions, different write path. |
| `TestLedger_v3_skipEntry` | `TestLedger_v4_skipEntry` | Same assertions, different write path. |
| `TestLedger_v3_duplicateEntry` | `TestLedger_v4_duplicateEntry` | Same assertions, different write path. |
| `TestLedger_v3_errorEntry` | `TestLedger_v4_errorEntry` | Same assertions, different write path. |
| `TestLedger_v3_recursive_false` | `TestLedger_v4_recursive_false` | Assert `Header.Recursive == false`. |
| `TestLedger_v3_omitempty_json` | `TestLedger_v4_omitempty_jsonl` | Read raw file bytes. Split by `\n`. Inspect individual lines for absent fields. No `"files"` array structure. |

### Update `sampleLedger()` and `sampleLedgerV3Full()` helpers

Replace with helpers that return a `domain.LedgerHeader` and `[]domain.LedgerEntry` separately, or that write via `LedgerWriter` and return the path.

---

## Task 28 — Tests: pipeline ledger tests for streaming

**File:** `internal/pipeline/pipeline_test.go` and `internal/pipeline/worker_test.go`

### Update all tests that call `manifest.LoadLedger`

These tests currently call `LoadLedger` and assert on `*domain.Ledger` fields. They must be updated to:

1. Call the new `LoadLedger` which returns `*manifest.LedgerContents`.
2. Access header fields via `lc.Header.Version`, `lc.Header.RunID`, etc.
3. Access entries via `lc.Entries[i]` instead of `ledger.Files[i]`.
4. Assert `Header.Version == 4` instead of `3`.

**Tests to update in `pipeline_test.go`** (8 call sites at lines ~221, ~252, ~284, ~582, ~626, ~666, ~707, ~1006):

| Test | Key Assertion Changes |
|------|----------------------|
| `TestRun_ledgerWritten` | `lc.Header.Version == 4`, `len(lc.Entries) > 0` |
| `TestRun_ledgerWritten_withEntry` | `lc.Entries[0].Checksum`, `lc.Entries[0].Destination` |
| `TestRun_ledgerVersion3WithRunID` → rename to `TestRun_ledgerVersion4WithRunID` | `lc.Header.Version == 4`, `lc.Header.RunID == expectedRunID` |
| `TestRun_outputFormat_ledgerEntryStatuses` | Count entries by status in `lc.Entries` |
| `TestRun_outputFormat_ledgerDuplicateEntry` | `lc.Entries[i].Status == "duplicate"`, `.Matches` |
| `TestRun_outputFormat_skipLedgerEntry` | `lc.Entries[i].Status == "skip"`, `.Reason` |
| `TestRun_outputFormat_copyLedgerEntry` | `lc.Entries[i].Status == "copy"`, `.VerifiedAt` |
| `TestRun_recursiveIncremental_ledger` | `lc.Header.Recursive == true`, entry count |

**Tests to update in `worker_test.go`** (1 call site at line ~91):

| Test | Key Assertion Changes |
|------|----------------------|
| `TestRunConcurrent_ledger` (or similar) | Same pattern — `lc.Header` + `lc.Entries` |

**Tests to update in `integration_test.go`** (1 call site via helper):

| Test | Key Assertion Changes |
|------|----------------------|
| `TestIntegration_SQLite_FullSort` | `lc.Header.Version == 4`, `lc.Header.RunID` matches DB |

---

## Task 29 — Tests: interrupted run produces partial valid JSONL

**File:** `internal/manifest/manifest_test.go`

### New test: `TestLedgerWriter_partialWrite`

Verify that if the writer is opened and some entries are written but `Close` is never called (simulating a crash), the file on disk contains valid JSONL up to the last complete entry.

```go
func TestLedgerWriter_partialWrite(t *testing.T) {
    t.Helper()
    dir := t.TempDir()

    header := domain.LedgerHeader{Version: 4, RunID: "test-run", ...}
    lw, err := manifest.NewLedgerWriter(dir, header)
    if err != nil {
        t.Fatal(err)
    }

    // Write 2 entries but do NOT call Close (simulating crash).
    _ = lw.WriteEntry(domain.LedgerEntry{Path: "a.jpg", Status: "copy", ...})
    _ = lw.WriteEntry(domain.LedgerEntry{Path: "b.jpg", Status: "skip", ...})
    // Intentionally no lw.Close()

    // Read the file directly and verify it's valid JSONL.
    raw, err := os.ReadFile(filepath.Join(dir, ".pixe_ledger.json"))
    if err != nil {
        t.Fatal(err)
    }

    lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
    if len(lines) != 3 { // header + 2 entries
        t.Fatalf("expected 3 lines, got %d", len(lines))
    }

    // Each line must be valid JSON.
    for i, line := range lines {
        if !json.Valid([]byte(line)) {
            t.Errorf("line %d is not valid JSON: %s", i+1, line)
        }
    }

    // Parse header.
    var h domain.LedgerHeader
    if err := json.Unmarshal([]byte(lines[0]), &h); err != nil {
        t.Fatalf("header parse: %v", err)
    }
    if h.Version != 4 {
        t.Errorf("header version: got %d, want 4", h.Version)
    }
}
```

This test validates the key architectural benefit of JSONL over buffered JSON: an interrupted run leaves a partial but valid receipt.
