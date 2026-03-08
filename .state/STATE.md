# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Project Scaffold & Go Module Init | High | @developer | ✅ Complete | — | Go module, directory layout, Cobra/Viper bootstrap |
| 2 | Core Domain Types & Interfaces | High | @developer | ✅ Complete | 1 | FileTypeHandler contract, pipeline types, config structs |
| 3 | Hashing Engine | High | @developer | ✅ Complete | 2 | Configurable hash.Hash factory, streaming io.Reader consumer |
| 4 | Manifest & Ledger Persistence | High | @developer | ✅ Complete | 2 | JSON read/write, atomic saves, per-file state tracking |
| 5 | File Discovery & Handler Registry | High | @developer | ✅ Complete | 2 | Walk dirA, extension match, magic-byte verify, skip dotfiles |
| 6 | Path Builder (Naming & Dedup) | High | @developer | ✅ Complete | 2, 3 | Deterministic output paths, duplicate routing |
| 7 | JPEG Filetype Module | High | @developer | ✅ Complete | 2, 3 | First concrete handler — proves the contract |
| 8 | Copy & Verify Engine | High | @developer | ✅ Complete | 3, 4, 6 | Streamed copy, post-copy re-hash, manifest updates |
| 9 | Sort Pipeline Orchestrator | High | @developer | ✅ Complete | 5, 7, 8 | Single-threaded first: discover → extract → hash → copy → verify |
| 10 | CLI: `pixe sort` Command | High | @developer | ✅ Complete | 9 | Cobra command, Viper flag binding, dry-run mode |
| 11 | Worker Pool & Concurrency | Medium | @developer | ✅ Complete | 9 | Coordinator + N workers, configurable --workers |
| 12 | HEIC Filetype Module | Medium | @developer | ✅ Complete | 7 | Second handler — validates contract generality |
| 13 | MP4 Filetype Module | Medium | @developer | ✅ Complete | 7 | Third handler — video keyframe hashing |
| 14 | Metadata Tagging Engine | Medium | @developer | ✅ Complete | 7, 8 | Copyright template, CameraOwner injection post-verify |
| 15 | CLI: `pixe verify` Command | Medium | @developer | ✅ Complete | 3, 5, 10 | Walk dirB, parse filename checksum, report mismatches |
| 16 | CLI: `pixe resume` Command | Medium | @developer | ✅ Complete | 4, 9, 10 | Load manifest, skip completed, re-enter pipeline |
| 17 | Integration Tests & Safety Audit | High | @tester | ✅ Complete | 10, 15, 16 | End-to-end with fixture files, interrupt simulation |
| 18 | Makefile & Build Tooling | Medium | @developer | ✅ Complete | 1 | help, build, test, lint, check, install targets; ldflags version injection |
| 19 | Version Package — Single Source of Truth | High | @developer | ⬜ Superseded | — | Superseded by Tasks 44–49 (idiomatic ldflags approach) |
| 20 | CLI: `pixe version` Command | High | @developer | ⬜ Superseded | 19 | Superseded by Task 44 (version vars + command collapsed into `cmd`) |
| 21 | Domain Structs — Add `PixeVersion` Field | High | @developer | ✅ Complete | 19 | Add field to `Manifest` and `Ledger` in `internal/domain/pipeline.go` |
| 22 | Pipeline — Populate `PixeVersion` at Runtime | High | @developer | ⬜ Superseded | 19, 21 | Superseded by Task 46 (pipeline reads `cmd.Version()` instead of `version.Version`) |
| 23 | Makefile — Retarget ldflags to `internal/version` | Medium | @developer | ⬜ Superseded | 19 | Superseded by Task 47 (Makefile delegates to GoReleaser) |
| 24 | Tests & Verification | High | @tester | ⬜ Superseded | 19, 20, 21, 22, 23 | Superseded by Task 49 (version tests removed; verification via build smoke test) |
| 25 | Lint Fixes — golangci-lint 0 issues | High | @developer | ✅ Complete | 1–24 | Fixed 30+ errcheck and unused lint violations across copy, discovery, heic, jpeg, mp4, verify, hash, manifest, pipeline packages; installed golangci-lint |
| 26 | Locale-Aware Month Directory — `pathbuilder` rewrite | High | @developer |  ✅ Complete | 6 | Change month dir from `2` to `02-Feb` (locale-aware); add `MonthDir()` helper |
| 27 | Update Tests — Month Directory Format | High | @developer |  ✅ Complete | 26 | Rewrite pathbuilder, pipeline, and integration tests for `MM-Mon` format |
| 28 | Tests & Verification — Full Suite Green | High | @tester |  ✅ Complete | 26, 27 | `go vet`, `go test -race ./...`, `make lint` all pass |
| 29 | Archive DB — `internal/archivedb` package & schema | High | @developer | ✅ Complete | 2 | SQLite database layer: Open, Close, schema creation, WAL mode, busy timeout |
| 30 | Archive DB — Run & File CRUD operations | High | @developer | ✅ Complete | 29 | InsertRun, UpdateRun, InsertFile, UpdateFile, dedup query, batch insert |
| 31 | Archive DB — Query methods | Medium | @developer | ✅ Complete | 30 | Query families: by source, date range, run, status, checksum, duplicates |
| 32 | DB Location Resolver — `internal/dblocator` package | High | @developer | ✅ Complete | 29 | Priority chain: --db-path → dbpath marker → local default; network mount detection; slug generation |
| 33 | Domain Types — SQLite-era updates | High | @developer | ✅ Complete | 2, 29 | Add `RunID` to Ledger, bump ledger version to 2, add `DBPath` to AppConfig |
| 34 | JSON Manifest Migration — `internal/migrate` package | High | @developer | ✅ Complete | 29, 30 | Auto-detect manifest.json, create synthetic run, import entries, rename to .migrated |
| 35 | Pipeline Refactor — Replace JSON manifest with archive DB | High | @developer | ✅ Complete | 29, 30, 32, 33 | Rewrite pipeline.go and worker.go to use archivedb instead of manifest.Save/Load |
| 36 | Pipeline — Cross-process dedup race handling | Medium | @developer | ✅ Complete | 35 | Post-commit dedup re-check, retroactive duplicate routing |
| 37 | CLI Updates — `--db-path` flag & resume rewrite | High | @developer | ✅ Complete | 32, 35 | Add --db-path to sort/resume, update resume to use DB discovery chain |
| 38 | Ledger Update — Add `run_id` field | Medium | @developer | ✅ Complete | 33, 35 | Wire run UUID into ledger creation, bump version to 2 |
| 39 | Archive DB — Unit tests | High | @tester | ✅ Complete | 29, 30, 31 | Schema creation, CRUD, queries, WAL concurrency, busy retry |
| 40 | DB Locator — Unit tests | High | @tester | ✅ Complete | 32 | Local/network detection, slug generation, dbpath marker read/write |
| 41 | Migration — Unit tests | High | @tester | ✅ Complete | 34 | JSON→SQLite migration, idempotency, synthetic run correctness |
| 42 | Integration Tests — SQLite pipeline end-to-end | High | @tester | ✅ Complete | 35, 36, 37, 38 | Full sort→verify→resume cycle using DB, concurrent run simulation |
| 43 | Tests & Verification — Full Suite Green | High | @tester | ✅ Complete | 39, 40, 41, 42 | `go vet`, `go test -race ./...`, `make lint` all pass |
| 44 | Version Vars & Command — Collapse into `cmd` | High | @developer | ✅ Complete | — | Move version vars + `fullVersion()` + `Version()` getter + `init()` into `cmd/version.go`; rewrite `pixe version` command |
| 45 | Delete `internal/version` Package | High | @developer | ✅ Complete | 44, 46 | Remove `internal/version/version.go` and `version_test.go`; remove stale import from any file |
| 46 | Pipeline — Switch to `cmd.Version()` | High | @developer | ✅ Complete | 44 | Replace `version.Version` with `cmd.Version()` in `pipeline.go` and `worker.go` |
| 47 | Makefile — Delegate to GoReleaser | High | @developer | ✅ Complete | 44 | Rewrite `build`/`install` targets to use `goreleaser build --single-target --snapshot`; keep `build-debug` as raw `go build` |
| 48 | GoReleaser — Fix ldflags Target | High | @developer | ✅ Complete | 44 | Retarget ldflags from `internal/version.*` to `cmd.version`, `cmd.commit`, `cmd.buildDate` |
| 49 | Tests & Verification — Version Refactor | High | @tester | ✅ Complete | 44, 45, 46, 47, 48 | Delete version_test.go; update manifest test fixtures; `go vet`, `go test -race ./...`, `make build && ./pixe version` |

---

# Pixe Task Descriptions

## Task 32 — DB Location Resolver — `internal/dblocator` Package

**Goal:** Implement the database location resolution logic: `--db-path` override → `dbpath` marker → local default, with network mount detection and slug generation for the fallback path.

**Architecture Reference:** Section 8.2 (Database Location)

**Depends on:** Task 29

**File to create: `internal/dblocator/dblocator.go`**

```go
// Package dblocator resolves the filesystem path for the Pixe archive database.
// It implements the priority chain: explicit --db-path → dbpath marker file →
// local default (dirB/.pixe/pixe.db), with automatic fallback to
// ~/.pixe/databases/<slug>.db when dirB is on a network filesystem.
package dblocator

// Location holds the resolved database path and metadata about the resolution.
type Location struct {
    // DBPath is the absolute path to the SQLite database file.
    DBPath string
    // IsRemote is true if dirB was detected as a network mount.
    IsRemote bool
    // MarkerNeeded is true if a dbpath marker should be written to dirB/.pixe/.
    MarkerNeeded bool
    // Notice is a user-facing message explaining the location choice.
    // Empty if the default local path was used.
    Notice string
}

// Resolve determines the database path for the given destination directory.
//
// Priority chain:
//  1. explicitPath (from --db-path flag) — used unconditionally if non-empty.
//  2. dirB/.pixe/dbpath marker file — if it exists, its contents are used.
//  3. dirB/.pixe/pixe.db — if dirB is on a local filesystem.
//  4. ~/.pixe/databases/<slug>.db — if dirB is on a network mount.
func Resolve(dirB string, explicitPath string) (*Location, error) { ... }

// WriteMarker writes the dbpath marker file at dirB/.pixe/dbpath
// containing the absolute path to the database.
func WriteMarker(dirB string, dbPath string) error { ... }

// ReadMarker reads the dbpath marker file at dirB/.pixe/dbpath.
// Returns ("", nil) if the marker does not exist.
func ReadMarker(dirB string) (string, error) { ... }
```

**File to create: `internal/dblocator/filesystem.go`**

```go
package dblocator

// isNetworkMount returns true if the given path resides on a network
// filesystem (NFS, SMB/CIFS, AFP). Uses OS-level filesystem type inspection.
func isNetworkMount(path string) (bool, error) { ... }
```

**Platform-specific implementation:**

**File: `internal/dblocator/filesystem_darwin.go`**
```go
//go:build darwin

package dblocator

import "syscall"

func isNetworkMount(path string) (bool, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(path, &stat); err != nil {
        return false, err
    }
    // Convert Fstypename [16]int8 to string.
    fstype := fstypeName(stat.Fstypename[:])
    // Network filesystem types on macOS.
    switch fstype {
    case "nfs", "smbfs", "afpfs", "webdav":
        return true, nil
    }
    return false, nil
}
```

**File: `internal/dblocator/filesystem_linux.go`**
```go
//go:build linux

package dblocator

import "syscall"

// Linux filesystem magic numbers for network mounts.
const (
    nfsMagic  = 0x6969
    smbMagic  = 0x517B
    smb2Magic = 0xFE534D42
    cifsMagic = 0xFF534D42
)

func isNetworkMount(path string) (bool, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(path, &stat); err != nil {
        return false, err
    }
    switch stat.Type {
    case nfsMagic, smbMagic, smb2Magic, cifsMagic:
        return true, nil
    }
    return false, nil
}
```

**Slug generation:**

```go
// slug generates a human-readable identifier for a dirB path.
// Format: <last-path-component>-<truncated-hash>.
// Example: "/Volumes/NAS/Photos/archive" → "archive-a1b2c3d4"
func slug(dirB string) string {
    abs, _ := filepath.Abs(dirB)
    base := strings.ToLower(filepath.Base(abs))
    // Sanitize: keep only alphanumeric and hyphens.
    base = sanitize(base)
    if base == "" {
        base = "pixe"
    }
    h := sha256.Sum256([]byte(abs))
    return fmt.Sprintf("%s-%x", base, h[:4])
}
```

**Marker file format:** Plain text, single line, the absolute path to the database file. No trailing newline.

**Acceptance Criteria:**
- `Resolve(dirB, "/explicit/path.db")` returns the explicit path with `MarkerNeeded=true`.
- `Resolve(dirB, "")` on a local filesystem returns `dirB/.pixe/pixe.db` with `MarkerNeeded=false`.
- `Resolve(dirB, "")` on a network mount returns `~/.pixe/databases/<slug>.db` with `MarkerNeeded=true` and a non-empty `Notice`.
- `WriteMarker` + `ReadMarker` round-trips the database path.
- `ReadMarker` returns `("", nil)` when no marker exists.
- `slug("/Volumes/NAS/Photos/archive")` returns `"archive-<8hex>"`.
- `slug("/")` returns `"pixe-<8hex>"` (edge case).
- Network mount detection works on macOS (darwin build tag).

---

## Task 33 — Domain Types — SQLite-Era Updates

**Goal:** Update the domain types and config struct to support the SQLite database: add `RunID` to the ledger, bump ledger version, and add `DBPath` to `AppConfig`.

**Architecture Reference:** Section 8.8 (Ledger v2), Section 9.1 (New Flag)

**Depends on:** Task 2, Task 29

### Files to modify

#### 1. `internal/config/config.go` — Add `DBPath` field

```go
type AppConfig struct {
    // ... existing fields ...

    // DBPath is an explicit path to the SQLite archive database.
    // If empty, the database location is auto-resolved (see dblocator package).
    DBPath string
}
```

#### 2. `internal/domain/pipeline.go` — Update Ledger struct

```go
// Ledger is the source-side record written to dirA/.pixe_ledger.json.
type Ledger struct {
    Version     int           `json:"version"`
    PixeVersion string        `json:"pixe_version"`
    RunID       string        `json:"run_id"`          // ← NEW: UUID linking to archive DB
    PixeRun     time.Time     `json:"pixe_run"`
    Algorithm   string        `json:"algorithm"`
    Destination string        `json:"destination"`
    Files       []LedgerEntry `json:"files"`
}
```

The `Version` field will be set to `2` when the ledger is created with a `RunID`. Existing code that creates ledgers with `Version: 1` will be updated in Task 38.

**Acceptance Criteria:**
- `AppConfig.DBPath` field exists and is a `string`.
- `Ledger.RunID` field exists with JSON tag `"run_id"`.
- `go build ./...` succeeds — the new fields are additive and don't break existing struct literals (Go named-field initialization is forward-compatible).
- Existing tests pass unchanged.

---

## Task 34 — JSON Manifest Migration — `internal/migrate` Package

**Goal:** Implement automatic migration from the JSON manifest to the SQLite database. When Pixe encounters `dirB/.pixe/manifest.json` but no database, it migrates all data into a new database, preserves the original file, and notifies the user.

**Architecture Reference:** Section 8.7 (Migration from JSON Manifest)

**Depends on:** Task 29, Task 30

**File to create: `internal/migrate/migrate.go`**

```go
// Package migrate handles automatic migration from the legacy JSON manifest
// (dirB/.pixe/manifest.json) to the SQLite archive database.
package migrate

import (
    "github.com/cwlls/pixe-go/internal/archivedb"
    "github.com/cwlls/pixe-go/internal/domain"
)

// Result holds the outcome of a migration attempt.
type Result struct {
    // Migrated is true if a migration was performed.
    Migrated bool
    // FileCount is the number of file entries migrated.
    FileCount int
    // Notice is a user-facing message describing what happened.
    Notice string
}

// MigrateIfNeeded checks for a legacy manifest.json at dirB/.pixe/ and,
// if found (and no .migrated version exists), migrates its contents into
// the provided database.
//
// Steps:
//  1. Check for dirB/.pixe/manifest.json — if absent, return (not migrated).
//  2. Check for dirB/.pixe/manifest.json.migrated — if present, skip (already done).
//  3. Read and parse the JSON manifest.
//  4. Create a synthetic run in the DB using manifest metadata.
//  5. Insert all file entries into the DB, mapping ManifestEntry fields to FileRecord.
//  6. Rename manifest.json → manifest.json.migrated.
//  7. Return the result with a user-facing notice.
func MigrateIfNeeded(db *archivedb.DB, dirB string) (*Result, error) { ... }
```

**Field mapping from `ManifestEntry` → `FileRecord`:**

| ManifestEntry field | FileRecord field | Notes |
|---|---|---|
| `Source` | `SourcePath` | Direct copy |
| `Destination` | `DestPath` | Direct copy (absolute) |
| — | `DestRel` | Computed: `strings.TrimPrefix(entry.Destination, manifest.Destination + "/")` |
| `Checksum` | `Checksum` | Direct copy |
| `Status` | `Status` | Direct copy (same enum values) |
| — | `IsDuplicate` | Inferred: `strings.Contains(destRel, "duplicates/")` |
| `ExtractedAt` | `ExtractedAt` | Direct copy |
| `CopiedAt` | `CopiedAt` | Direct copy |
| `VerifiedAt` | `VerifiedAt` | Direct copy |
| `TaggedAt` | `TaggedAt` | Direct copy |
| `Error` | `Error` | Direct copy |

**Synthetic run creation:**

```go
syntheticRun := &archivedb.Run{
    ID:          uuid.New().String(),  // or a deterministic UUID from manifest hash
    PixeVersion: manifest.PixeVersion,
    Source:      manifest.Source,
    Destination: manifest.Destination,
    Algorithm:   manifest.Algorithm,
    Workers:     manifest.Workers,
    StartedAt:   manifest.StartedAt,
    FinishedAt:  &manifest.StartedAt,  // best approximation
    Status:      "completed",          // the prior run is assumed complete
}
```

**UUID dependency:** Add `github.com/google/uuid` for UUID v4 generation:
```bash
go get github.com/google/uuid
```

**Acceptance Criteria:**
- Given a `dirB` with `manifest.json` containing 5 entries, `MigrateIfNeeded` creates a DB with 1 run and 5 files.
- The original `manifest.json` is renamed to `manifest.json.migrated`.
- Calling `MigrateIfNeeded` again (with `.migrated` present) returns `Migrated: false` — idempotent.
- Calling `MigrateIfNeeded` on a `dirB` with no manifest returns `Migrated: false`.
- The synthetic run has `status = "completed"`.
- File entries preserve all timestamps, checksums, and statuses.
- `IsDuplicate` is correctly inferred from the destination path.
- The `Result.Notice` contains the file count (e.g., `"Migrated 5 files from manifest.json → pixe.db"`).

---

## Task 35 — Pipeline Refactor — Replace JSON Manifest with Archive DB

**Goal:** Rewrite the pipeline orchestrator (`pipeline.go` and `worker.go`) to use `archivedb.DB` instead of `manifest.Save`/`manifest.Load`. This is the largest single task — it touches the core data flow.

**Architecture Reference:** Section 8.5 (Transaction Granularity), Section 8.6 (Database Lifecycle)

**Depends on:** Task 29, Task 30, Task 32, Task 33

### High-level changes

#### 1. `SortOptions` — Add DB reference

```go
type SortOptions struct {
    Config       *config.AppConfig
    Hasher       *hash.Hasher
    Registry     *discovery.Registry
    RunTimestamp string
    Output       io.Writer
    DB           *archivedb.DB   // ← NEW: archive database
    RunID        string          // ← NEW: UUID for this run
}
```

#### 2. `pipeline.Run()` — Rewrite flow

**Before (JSON):**
1. `manifest.Load(dirB)` → create or load manifest
2. Build dedup index from manifest entries (`map[checksum]destRel`)
3. Walk dirA, add new entries to manifest, `manifest.Save()`
4. Process each file, mutate `ManifestEntry`, `manifest.Save()` after each
5. Write ledger, final `manifest.Save()`

**After (SQLite):**
1. DB is already opened and passed in via `SortOptions.DB`
2. `db.InsertRun()` with `status = "running"`
3. Walk dirA, `db.InsertFiles()` batch-insert as `"pending"`
4. Dedup check: `db.CheckDuplicate(checksum)` — no in-memory map needed
5. Process each file, `db.UpdateFileStatus()` after each stage — commit per file
6. `db.CompleteRun()` at end
7. Write ledger with `RunID`

**Key difference:** The in-memory `dedupIndex map[string]string` is replaced by `db.CheckDuplicate(checksum)`. This is a SELECT query hitting the partial index — fast and memory-bounded.

#### 3. `worker.go` — Rewrite coordinator loop

The coordinator currently:
- Maintains `dedupIndex` in memory
- Calls `saveManifest()` after each file

**After:**
- Calls `db.CheckDuplicate()` for dedup decisions
- Calls `db.UpdateFileStatus()` after each file completes (commit per file)
- No more `saveManifest()` calls

Workers continue to operate the same way — they extract, hash, copy, verify, tag. The only change is that the coordinator writes to the DB instead of the JSON manifest.

#### 4. Remove `manifest.Save`/`manifest.Load` from pipeline

The `internal/manifest` package is **not deleted** — it's still needed for:
- `manifest.Load()` — used by the migration path (Task 34)
- `manifest.SaveLedger()` / `manifest.LoadLedger()` — ledger persistence is unchanged

But `manifest.Save()` is no longer called from the pipeline.

#### 5. `SortResult` — unchanged

The `SortResult` struct returned by `Run()` is unchanged. The summary statistics are computed the same way.

### Files to modify

- `internal/pipeline/pipeline.go` — major rewrite of `Run()` and `processFile()`
- `internal/pipeline/worker.go` — major rewrite of `RunConcurrent()` coordinator loop

### Files NOT modified

- `internal/manifest/manifest.go` — kept for migration and ledger
- `internal/copy/copy.go` — unchanged
- `internal/pathbuilder/pathbuilder.go` — unchanged
- `internal/discovery/` — unchanged

**Acceptance Criteria:**
- `pipeline.Run()` creates a run record in the DB with `status = "running"`.
- Each discovered file is inserted as `"pending"` via batch insert.
- Each file completion commits a status update to the DB.
- Dedup checks use `db.CheckDuplicate()` — no in-memory map.
- On successful completion, the run is marked `"completed"`.
- On context cancellation (Ctrl+C), the run is marked `"interrupted"`.
- The ledger is still written to `dirA` via `manifest.SaveLedger()`.
- `manifest.Save()` is no longer called anywhere in the pipeline.
- `go build ./...` succeeds.
- Existing pipeline tests are updated to provide a DB in `SortOptions`.

---

## Task 36 — Pipeline — Cross-Process Dedup Race Handling

**Goal:** Handle the race condition where two simultaneous `pixe sort` processes discover the same file (identical checksum) from different sources. The second process to commit should detect the conflict and retroactively route its copy to `duplicates/`.

**Architecture Reference:** Section 8.5 (Cross-Process Dedup Race Condition)

**Depends on:** Task 35

### Implementation

After a file is copied and verified, but before marking it `"complete"`, the coordinator performs a **post-commit dedup re-check**:

```go
// In the coordinator, after copy+verify succeeds:
existingDest, err := db.CheckDuplicate(checksum)
if err != nil {
    // handle error
}
if existingDest != "" {
    // Another process completed this checksum while we were copying.
    // Our copy is now a duplicate. Move it to the duplicates directory.
    dupDest := pathbuilder.Build(captureDate, checksum, ext, true, runTimestamp)
    if err := os.Rename(destPath, filepath.Join(dirB, dupDest)); err != nil {
        // handle error — file is still at destPath, mark as failed
    }
    // Update the file record with the new duplicate destination.
    db.UpdateFileStatus(fileID, "complete",
        WithDestination(filepath.Join(dirB, dupDest), dupDest),
        WithIsDuplicate(true),
    )
} else {
    // We're the first — mark complete at the original destination.
    db.UpdateFileStatus(fileID, "complete")
}
```

**Key insight:** The dedup check and the status update must happen within the same transaction to prevent a TOCTOU race between two processes both thinking they're first. Add a method:

```go
// CompleteFileWithDedupCheck atomically checks for an existing completed file
// with the same checksum and marks this file as complete. If a duplicate is
// detected, it returns the existing destination path so the caller can
// relocate the physical file.
func (db *DB) CompleteFileWithDedupCheck(fileID int64, checksum string) (existingDest string, err error) { ... }
```

This method runs within a single transaction:
1. `SELECT dest_rel FROM files WHERE checksum = ? AND status = 'complete' AND id != ? LIMIT 1`
2. If found: update file with `is_duplicate = 1`, return the existing dest
3. If not found: update file with `status = 'complete'`, return empty string

**Acceptance Criteria:**
- When two files with the same checksum are processed, the second one is correctly routed to `duplicates/`.
- The physical file is moved (renamed) to the duplicates directory.
- The DB record reflects `is_duplicate = 1` and the updated destination path.
- The operation is atomic — no window where both files appear as non-duplicates.

---

## Task 39 — Archive DB — Unit Tests

**Goal:** Comprehensive unit tests for the `internal/archivedb` package covering schema creation, CRUD operations, query methods, WAL concurrency, and busy retry behavior.

**Architecture Reference:** Section 8.3, 8.4, 8.5

**Depends on:** Tasks 29, 30, 31

**File to create: `internal/archivedb/archivedb_test.go`**

### Test cases

1. **`TestOpen_createsSchema`** — Open a new DB, verify all tables exist via `sqlite_master`.
2. **`TestOpen_idempotent`** — Open an existing DB, verify no errors and schema is intact.
3. **`TestOpen_WALMode`** — Verify `PRAGMA journal_mode` returns `wal`.
4. **`TestSchemaVersion`** — Verify `schema_version` table has version 1.
5. **`TestInsertRun_roundtrip`** — Insert a run, retrieve it, verify all fields.
6. **`TestCompleteRun`** — Insert a run, complete it, verify `finished_at` and `status`.
7. **`TestInterruptRun`** — Insert a run, interrupt it, verify status.
8. **`TestFindInterruptedRuns`** — Insert 3 runs (1 running, 1 completed, 1 interrupted), verify only the running one is returned.
9. **`TestInsertFile_roundtrip`** — Insert a file, retrieve by run, verify all fields.
10. **`TestInsertFiles_batch`** — Batch-insert 100 files, verify count.
11. **`TestUpdateFileStatus_progression`** — Walk a file through all pipeline stages, verify timestamps are set.
12. **`TestCheckDuplicate_found`** — Insert a completed file, check its checksum, verify dest_rel returned.
13. **`TestCheckDuplicate_notFound`** — Check a checksum that doesn't exist, verify empty string.
14. **`TestCheckDuplicate_ignoresNonComplete`** — Insert a file with status "hashed" (not complete), verify CheckDuplicate returns empty.
15. **`TestGetIncompleteFiles`** — Insert files in various states, verify only non-terminal ones returned.
16. **`TestListRuns`** — Insert 3 runs, verify returned in reverse chronological order with file counts.
17. **`TestFilesWithErrors`** — Insert files with error states, verify join with run source.
18. **`TestDuplicatePairs`** — Insert an original and a duplicate with same checksum, verify pairing.
19. **`TestConcurrentReaders`** — Open two connections to the same DB, read simultaneously, verify no errors.
20. **`TestBusyRetry`** — Simulate write contention between two connections, verify the second writer succeeds after retry.

All tests use `t.TempDir()` for database file isolation.

**Acceptance Criteria:**
- All 20 test cases pass.
- Tests run with `-race` flag without data race warnings.
- Tests complete in under 5 seconds.

---

## Task 40 — DB Locator — Unit Tests

**Goal:** Unit tests for the `internal/dblocator` package covering the resolution priority chain, slug generation, and marker file operations.

**Depends on:** Task 32

**File to create: `internal/dblocator/dblocator_test.go`**

### Test cases

1. **`TestResolve_explicitPath`** — Explicit path always wins, `MarkerNeeded=true`.
2. **`TestResolve_markerFile`** — Write a marker, resolve without explicit path, verify marker contents used.
3. **`TestResolve_localDefault`** — No marker, local filesystem, verify `dirB/.pixe/pixe.db`.
4. **`TestResolve_priorityOrder`** — Explicit > marker > default.
5. **`TestWriteMarker_ReadMarker_roundtrip`** — Write and read back.
6. **`TestReadMarker_notExists`** — Returns empty string, no error.
7. **`TestSlug_normalPath`** — Verify format: `<base>-<8hex>`.
8. **`TestSlug_rootPath`** — Edge case: `/` → `"pixe-<8hex>"`.
9. **`TestSlug_deterministic`** — Same input always produces same slug.
10. **`TestSlug_differentPaths`** — Different inputs produce different slugs.

**Note:** Network mount detection (`isNetworkMount`) is difficult to unit test without actual network mounts. Test it with a mock/stub or skip on CI with a build tag.

**Acceptance Criteria:**
- All test cases pass.
- Slug generation is deterministic and collision-resistant.
- Marker file round-trip works correctly.

---

## Task 41 — Migration — Unit Tests

**Goal:** Unit tests for the `internal/migrate` package covering JSON→SQLite migration, idempotency, and edge cases.

**Depends on:** Task 34

**File to create: `internal/migrate/migrate_test.go`**

### Test cases

1. **`TestMigrateIfNeeded_noManifest`** — No manifest.json → `Migrated: false`.
2. **`TestMigrateIfNeeded_alreadyMigrated`** — `.migrated` exists → `Migrated: false`.
3. **`TestMigrateIfNeeded_success`** — Manifest with 5 entries → DB has 1 run + 5 files, manifest renamed.
4. **`TestMigrateIfNeeded_preservesTimestamps`** — Verify all timestamp fields survive migration.
5. **`TestMigrateIfNeeded_preservesStatuses`** — Verify all status values map correctly.
6. **`TestMigrateIfNeeded_infersDuplicates`** — Entry with `duplicates/` in dest path → `is_duplicate = 1`.
7. **`TestMigrateIfNeeded_syntheticRunMetadata`** — Verify the synthetic run has correct pixe_version, source, algorithm, etc.
8. **`TestMigrateIfNeeded_idempotent`** — Call twice, second call is a no-op.

All tests create a real `manifest.json` file in `t.TempDir()` and a real SQLite database.

**Acceptance Criteria:**
- All 8 test cases pass.
- Migration is lossless — all data from the JSON manifest is present in the DB.
- The original `manifest.json` is preserved as `manifest.json.migrated`.

---

## Task 42 — Integration Tests — SQLite Pipeline End-to-End

**Goal:** End-to-end integration tests that exercise the full sort → verify → resume cycle using the SQLite database, including concurrent run simulation.

**Depends on:** Tasks 35, 36, 37, 38

**File to modify: `internal/integration/integration_test.go`**

### New test cases (add to existing integration test file)

1. **`TestIntegration_SQLite_FullSort`** — Sort fixture files, verify:
   - Database exists at `dirB/.pixe/pixe.db`.
   - `runs` table has 1 row with `status = "completed"`.
   - `files` table has correct count with all `status = "complete"`.
   - Dedup check returns correct results.
   - Ledger has `version: 2` and `run_id` matching the DB.

2. **`TestIntegration_SQLite_Resume`** — Sort 5 files, simulate interrupt (mark run as "running", reset 2 files to "pending"), resume, verify all 5 complete.

3. **`TestIntegration_SQLite_MultiSource`** — Sort from source A, then sort from source B into the same `dirB`. Verify:
   - 2 runs in the `runs` table.
   - Files from both sources in the `files` table.
   - Dedup works across runs (if source B has a file identical to source A, it's routed to duplicates).

4. **`TestIntegration_SQLite_Migration`** — Create a `dirB` with a legacy `manifest.json`, run `pixe sort` against it, verify:
   - Auto-migration occurred.
   - `manifest.json.migrated` exists.
   - DB contains the migrated entries plus the new sort's entries.

5. **`TestIntegration_SQLite_DryRun`** — Dry-run creates a run record but no file copies. Verify DB state.

6. **`TestIntegration_SQLite_NoDBPathMarker_LocalFS`** — On local filesystem, verify no `dbpath` marker is created.

### Updated existing tests

All existing integration tests that reference `manifest.json` or `manifest.Load()` must be updated to use the database. The `TestIntegration_FullSort`, `TestIntegration_Resume`, etc. should be updated to verify DB state instead of (or in addition to) manifest state.

**Acceptance Criteria:**
- All new integration tests pass.
- All updated existing integration tests pass.
- Tests run with `-race` flag without data race warnings.
- Multi-source test demonstrates cumulative registry behavior.
- Migration test demonstrates seamless JSON→SQLite transition.

---

## Task 43 — Tests & Verification — Full Suite Green

**Goal:** Verify the entire codebase compiles, passes all tests, and passes lint after the SQLite migration.

**Depends on:** Tasks 39, 40, 41, 42

### Verification commands

```bash
go vet ./...                                    # No warnings
go build ./...                                  # Compiles cleanly
go test -race -timeout 120s ./...               # All tests pass
make lint                                       # 0 issues
go mod tidy                                     # No diff
```

### Specific checks

1. **No stale JSON manifest references in pipeline:**
   ```bash
   # Should return zero matches in pipeline files:
   rg 'manifest\.Save\(' internal/pipeline/
   rg 'manifest\.Load\(' internal/pipeline/
   # manifest.SaveLedger is still valid — ledger is unchanged
   ```

2. **No in-memory dedup index in pipeline:**
   ```bash
   # The old dedupIndex map should be gone:
   rg 'dedupIndex' internal/pipeline/
   ```

3. **Dependency audit:**
   ```bash
   go mod tidy
   # New dependencies: modernc.org/sqlite, github.com/google/uuid
   # Verify no unexpected additions
   ```

4. **Build smoke test:**
   ```bash
   make build
   ./pixe sort --source /tmp/test-photos --dest /tmp/test-archive --dry-run
   # Verify DB is created at /tmp/test-archive/.pixe/pixe.db
   # Verify output shows normal sort behavior
   ```

### Acceptance Criteria

- `go vet ./...` — zero warnings.
- `go build ./...` — compiles cleanly.
- `go test -race -timeout 120s ./...` — all tests pass (unit + integration).
- `make lint` — 0 issues.
- `go mod tidy` produces no diff.
- No pipeline code references `manifest.Save()` or `manifest.Load()`.
- No pipeline code uses an in-memory `dedupIndex` map.
- The `internal/manifest` package is retained for ledger persistence and migration support only.

---
