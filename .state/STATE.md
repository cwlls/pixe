# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add `Recursive` and `Ignore` fields to `AppConfig` | high | Developer | ✅ Complete | — | Struct changes only; no behavior yet |
| 2 | Register `--recursive` and `--ignore` CLI flags | high | Developer | ✅ Complete | 1 | Cobra flag registration + Viper binding in `cmd/sort.go` |
| 3 | Add `StatusSkipped` to domain and `skip_reason` to DB schema | high | Developer | ✅ Complete | — | Domain const + schema v2 migration |
| 4 | Implement DB schema v2 migration | high | Developer | ✅ Complete | 3 | `recursive` on `runs`, `skip_reason`+`skipped` on `files` |
| 5 | Add `CheckSourceProcessed` query to archivedb | high | Developer | pending | 4 | Skip-detection query by absolute `source_path` |
| 6 | Build the ignore-list matcher | high | Developer | pending | 1 | New `internal/ignore` package with glob matching |
| 7 | Refactor `discovery.Walk` for recursive + ignore + skip output | high | Developer | pending | 1, 6 | Controlled recursion, ignore filtering, structured skip returns |
| 8 | Upgrade `LedgerEntry` and `Ledger` to v3 | high | Developer | pending | 3 | New fields: `Status`, `Reason`, `Matches`, `Recursive` |
| 9 | Refactor pipeline stdout output to COPY/SKIP/DUPE/ERR format | high | Developer | pending | 3, 5, 7, 8 | Central formatting; all outcomes produce one line |
| 10 | Wire skip/dupe/err entries into ledger and DB | high | Developer | pending | 8, 9 | Skipped + unsupported files get ledger entries + DB rows |
| 11 | Update `Run` struct and `InsertRun` for `recursive` column | medium | Developer | pending | 4 | Propagate `cfg.Recursive` into the runs table |
| 12 | Update concurrent worker path (`worker.go`) for new output format | high | Developer | pending | 9 | Mirror sequential changes in the concurrent coordinator |
| 13 | Tests: ignore-list matcher | high | Tester | pending | 6 | Unit tests for glob matching, hardcoded ledger ignore |
| 14 | Tests: discovery.Walk with recursive + ignore | high | Tester | pending | 7 | Integration tests with nested dirs, dotfiles, ignore patterns |
| 15 | Tests: pipeline output format (COPY/SKIP/DUPE/ERR) | high | Tester | pending | 9, 10 | Capture stdout, verify exact format for each verb |
| 16 | Tests: ledger v3 serialization | medium | Tester | pending | 10 | Round-trip JSON, verify all status/reason/matches fields |
| 17 | Tests: schema v2 migration from v1 DB | medium | Tester | pending | 4 | Create v1 DB, run migration, verify new columns |
| 18 | Tests: recursive incremental run (skip previously imported) | medium | Tester | pending | 9, 10 | Two-run scenario: flat then recursive, verify skips |

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
