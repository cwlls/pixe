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
| 50 | TIFF-RAW Shared Base — `internal/handler/tiffraw` | High | @developer | ⬜ Pending | 2, 7 | Shared `Base` struct: EXIF date extraction via TIFF IFD parsing, embedded JPEG preview extraction for `HashableReader`, no-op `WriteMetadataTags` |
| 51 | DNG Filetype Module — `internal/handler/dng` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; DNG-specific extensions, magic bytes (TIFF LE/BE), Detect with DNGVersion tag check |
| 52 | NEF Filetype Module — `internal/handler/nef` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.nef` extension, TIFF LE magic, extension-primary detection |
| 53 | CR2 Filetype Module — `internal/handler/cr2` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.cr2` extension, TIFF LE magic + `CR` at offset 8 |
| 54 | PEF Filetype Module — `internal/handler/pef` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.pef` extension, TIFF LE magic, extension-primary detection |
| 55 | ARW Filetype Module — `internal/handler/arw` | High | @developer | ⬜ Pending | 50 | Thin wrapper embedding `tiffraw.Base`; `.arw` extension, TIFF LE magic, extension-primary detection |
| 56 | CR3 Filetype Module — `internal/handler/cr3` | High | @developer | ⬜ Pending | 12 | Standalone ISOBMFF-based handler; EXIF via ISOBMFF box extraction (like HEIC), JPEG preview from container, no-op write |
| 57 | RAW Handler Registration — Wire into CLI | High | @developer | ⬜ Pending | 51, 52, 53, 54, 55, 56 | Register all 6 RAW handlers + HEIC + MP4 in `cmd/sort.go`, `cmd/verify.go`, `cmd/resume.go` |
| 58 | RAW Handlers — Unit Tests | High | @tester | ⬜ Pending | 50, 51, 52, 53, 54, 55, 56 | Per-handler tests: Extensions, MagicBytes, Detect, ExtractDate fallback, HashableReader determinism, WriteMetadataTags no-op |
| 59 | RAW Handlers — Integration Tests | High | @tester | ⬜ Pending | 57, 58 | End-to-end sort with RAW fixture files, verify DB records, verify output naming with `.dng`/`.nef`/`.cr2`/`.cr3`/`.pef`/`.arw` extensions |
| 60 | Tests & Verification — Full Suite Green (RAW) | High | @tester | ⬜ Pending | 58, 59 | `go vet`, `go test -race ./...`, `make lint`, `go mod tidy` all pass with RAW handlers |

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

## Task 50 — TIFF-RAW Shared Base — `internal/handler/tiffraw`

**Goal:** Create the shared base package that provides `ExtractDate`, `HashableReader`, and `WriteMetadataTags` for all TIFF-based RAW formats (DNG, NEF, CR2, PEF, ARW). Per-format handlers embed this struct and supply only their identity methods.

**Architecture Reference:** Section 6.4.1–6.4.7

**Depends on:** Task 2 (domain types), Task 7 (JPEG handler — establishes patterns)

**File to create: `internal/handler/tiffraw/tiffraw.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 ...

// Package tiffraw provides shared logic for TIFF-based RAW image formats.
//
// Five RAW formats — DNG, NEF (Nikon), CR2 (Canon), PEF (Pentax), and
// ARW (Sony) — are all TIFF containers with standard EXIF IFDs. They share
// identical logic for date extraction, hashable region identification, and
// metadata write behavior. This package provides a Base struct that
// implements the three format-agnostic methods of the FileTypeHandler
// interface. Per-format handlers embed Base and supply only Extensions(),
// MagicBytes(), and Detect().
//
// Date extraction:
//   Parses the TIFF IFD chain to locate EXIF sub-IFDs, then reads
//   DateTimeOriginal (tag 0x9003) and DateTime (tag 0x0132) with the
//   standard fallback chain: DateTimeOriginal → DateTime → Ansel Adams date.
//
// Hashable region:
//   Extracts the embedded full-resolution JPEG preview image. TIFF-based
//   RAW files store this in a secondary IFD (often IFD1 or a sub-IFD) with
//   NewSubfileType = 0 (full-resolution) and Compression = 6 (JPEG).
//   The handler navigates the IFD chain, locates the JPEG strip/tile
//   offsets and byte counts, and returns a reader over that region.
//   Falls back to full-file hash if the JPEG preview cannot be extracted.
//
// Metadata write:
//   No-op stub. RAW files are archival originals — writing metadata into
//   proprietary containers risks corruption.
package tiffraw

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    rwexif "github.com/rwcarlsen/goexif/exif"

    "github.com/cwlls/pixe-go/internal/domain"
)

// anselsAdams is the fallback date when no EXIF date can be extracted.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

const exifDateFormat = "2006:01:02 15:04:05"

// TIFF IFD tag IDs used for JPEG preview extraction.
const (
    tagNewSubfileType  = 0x00FE
    tagCompression     = 0x0103
    tagStripOffsets    = 0x0111
    tagStripByteCounts = 0x0117
    tagJPEGOffset      = 0x0201 // JPEGInterchangeFormat (IFD1 thumbnail)
    tagJPEGLength      = 0x0202 // JPEGInterchangeFormatLength
)

// Base provides shared logic for TIFF-based RAW formats.
// Per-format handlers embed this struct and supply their own
// Extensions(), MagicBytes(), and Detect() methods.
type Base struct{}

// ExtractDate reads the capture date from EXIF metadata embedded in the
// TIFF container. Fallback chain: DateTimeOriginal → DateTime → Ansel Adams.
func (b *Base) ExtractDate(filePath string) (time.Time, error) { ... }

// HashableReader returns an io.ReadCloser over the embedded full-resolution
// JPEG preview image. If the preview cannot be extracted, falls back to
// returning a reader over the entire file.
func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) { ... }

// WriteMetadataTags is a no-op for TIFF-based RAW formats.
func (b *Base) WriteMetadataTags(filePath string, tags domain.MetadataTags) error {
    // RAW metadata write not supported in pure Go.
    return nil
}
```

### Implementation Details

#### `ExtractDate` implementation

The TIFF container is a valid input for `rwcarlsen/goexif` — the same library used by the JPEG handler. RAW files that are TIFF-based can be decoded directly:

```go
func (b *Base) ExtractDate(filePath string) (time.Time, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return anselsAdams, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    x, err := rwexif.Decode(f)
    if err != nil {
        return anselsAdams, nil // No EXIF — use fallback
    }

    // 1. DateTimeOriginal
    if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    // 2. DateTime (IFD0)
    if tag, err := x.Get(rwexif.DateTime); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    return anselsAdams, nil
}
```

#### `HashableReader` implementation

This is the most complex method. It must:

1. Read the TIFF header to determine byte order (LE `II` or BE `MM`).
2. Navigate the IFD chain (IFD0 → IFD1 → sub-IFDs).
3. For each IFD, check for a JPEG preview:
   - **IFD1 thumbnail path:** Look for `JPEGInterchangeFormat` (tag 0x0201) and `JPEGInterchangeFormatLength` (tag 0x0202). This is the standard TIFF thumbnail location.
   - **Sub-IFD path:** Look for IFDs where `NewSubfileType = 0` (full-resolution) and `Compression = 6` (JPEG). Read `StripOffsets` (tag 0x0111) and `StripByteCounts` (tag 0x0117) to locate the JPEG data.
4. Select the **largest** JPEG preview found (by byte count) — this is the full-resolution preview.
5. Return an `io.ReadCloser` over that byte range using `io.SectionReader`.
6. If no JPEG preview is found, fall back to returning `os.Open(filePath)` (full file).

```go
// jpegPreview holds the offset and size of an embedded JPEG preview.
type jpegPreview struct {
    offset int64
    size   int64
}

func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
    }

    preview, err := findLargestJPEGPreview(f)
    if err != nil || preview == nil {
        // Fallback: hash the full file.
        if _, err := f.Seek(0, io.SeekStart); err != nil {
            _ = f.Close()
            return nil, fmt.Errorf("tiffraw: seek %q: %w", filePath, err)
        }
        return f, nil
    }

    // Return a reader scoped to the JPEG preview region.
    sr := io.NewSectionReader(f, preview.offset, preview.size)
    return &sectionReadCloser{Reader: sr, Closer: f}, nil
}

// sectionReadCloser wraps an io.Reader with a separate io.Closer,
// allowing the caller to close the underlying file when done reading
// a section of it.
type sectionReadCloser struct {
    Reader io.Reader
    Closer io.Closer
}

func (s *sectionReadCloser) Read(p []byte) (int, error) { return s.Reader.Read(p) }
func (s *sectionReadCloser) Close() error               { return s.Closer.Close() }

// findLargestJPEGPreview parses the TIFF IFD chain and returns the
// offset and size of the largest embedded JPEG preview image.
// Returns nil if no JPEG preview is found.
func findLargestJPEGPreview(r io.ReadSeeker) (*jpegPreview, error) { ... }
```

The `findLargestJPEGPreview` function must:
- Read bytes 0–1 to determine byte order (`II` = little-endian, `MM` = big-endian).
- Read bytes 2–3 to verify TIFF magic (`42` in the determined byte order).
- Read bytes 4–7 for the offset to IFD0.
- Walk each IFD: read the entry count (2 bytes), then each 12-byte IFD entry.
- For each IFD, collect tag values for `NewSubfileType`, `Compression`, `StripOffsets`, `StripByteCounts`, `JPEGInterchangeFormat`, `JPEGInterchangeFormatLength`.
- After processing all entries in an IFD, check if it contains a JPEG preview and record it.
- Read the 4-byte "next IFD offset" at the end of each IFD to continue the chain (0 = end).
- Also follow `SubIFDs` (tag 0x014A) pointers if present — some RAW formats store the full-res preview in a sub-IFD rather than IFD1.
- Return the largest preview found.

#### `fileExt` helper

Include the same `fileExt` helper used by other handlers (duplicated per package, consistent with existing pattern):

```go
func fileExt(path string) string {
    for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
        if path[i] == '.' {
            return path[i:]
        }
    }
    return ""
}
```

### Acceptance Criteria

- `Base.ExtractDate()` correctly reads `DateTimeOriginal` from a TIFF-based RAW file.
- `Base.ExtractDate()` falls back to `DateTime` when `DateTimeOriginal` is absent.
- `Base.ExtractDate()` returns Ansel Adams date when no EXIF is present.
- `Base.HashableReader()` returns the embedded JPEG preview bytes when present.
- `Base.HashableReader()` falls back to full-file reader when no JPEG preview is found.
- `Base.HashableReader()` returns a deterministic byte stream (same bytes on repeated calls).
- `Base.WriteMetadataTags()` is a no-op that returns nil.
- The `sectionReadCloser` properly closes the underlying file handle.
- `go build ./...` succeeds.

---

## Task 51 — DNG Filetype Module — `internal/handler/dng`

**Goal:** Create the DNG handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/dng/dng.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 ...

// Package dng implements the FileTypeHandler contract for Adobe DNG
// (Digital Negative) RAW images.
//
// DNG files are TIFF containers with standard EXIF IFDs. Date extraction,
// hashable region (embedded JPEG preview), and metadata write (no-op) are
// provided by the shared tiffraw.Base.
//
// Detection:
//   DNG files use the standard TIFF header (little-endian "II" 0x2A00 or
//   big-endian "MM" 0x002A). Since this header is shared with other
//   TIFF-based formats, the .dng extension is the primary discriminator.
//   Magic bytes confirm the file is a valid TIFF container.
package dng

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

// Handler implements domain.FileTypeHandler for DNG images.
type Handler struct {
    tiffraw.Base // embeds ExtractDate, HashableReader, WriteMetadataTags
}

// New returns a new DNG Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
    return []string{".dng"}
}

// MagicBytes returns the TIFF magic byte signatures.
// DNG uses standard TIFF headers — both little-endian and big-endian.
func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE ("II" + 42)
        {Offset: 0, Bytes: []byte{0x4D, 0x4D, 0x00, 0x2A}}, // TIFF BE ("MM" + 42)
    }
}

// Detect returns true if the file has a .dng extension AND begins with
// a valid TIFF header (little-endian or big-endian).
func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".dng" {
        return false, nil
    }
    f, err := os.Open(filePath)
    if err != nil {
        return false, fmt.Errorf("dng: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    header := make([]byte, 4)
    if _, err := io.ReadFull(f, header); err != nil {
        return false, nil // file too short
    }
    // Check for TIFF LE or BE header.
    le := header[0] == 0x49 && header[1] == 0x49 && header[2] == 0x2A && header[3] == 0x00
    be := header[0] == 0x4D && header[1] == 0x4D && header[2] == 0x00 && header[3] == 0x2A
    return le || be, nil
}

func fileExt(path string) string {
    for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
        if path[i] == '.' {
            return path[i:]
        }
    }
    return ""
}
```

### Acceptance Criteria

- `Handler` satisfies `domain.FileTypeHandler` (compile-time check via `var _ domain.FileTypeHandler = (*Handler)(nil)`).
- `Extensions()` returns `[".dng"]`.
- `MagicBytes()` returns two signatures (TIFF LE and TIFF BE).
- `Detect()` returns true for `.dng` files with valid TIFF headers.
- `Detect()` returns false for `.dng` files with non-TIFF content.
- `Detect()` returns false for non-`.dng` extensions even with TIFF content.
- `ExtractDate()`, `HashableReader()`, `WriteMetadataTags()` are inherited from `tiffraw.Base`.
- `go build ./...` succeeds.

---

## Task 52 — NEF Filetype Module — `internal/handler/nef`

**Goal:** Create the Nikon NEF handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/nef/nef.go`**

```go
// Package nef implements the FileTypeHandler contract for Nikon NEF RAW images.
//
// NEF files are TIFF containers with Nikon-specific maker note IFDs.
// The .nef extension is the primary discriminator. Magic bytes confirm
// the file is a valid TIFF container (little-endian only — Nikon always
// uses LE byte order).
package nef

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".nef"}
}

func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".nef" {
        return false, nil
    }
    f, err := os.Open(filePath)
    if err != nil {
        return false, fmt.Errorf("nef: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    header := make([]byte, 4)
    if _, err := io.ReadFull(f, header); err != nil {
        return false, nil
    }
    return header[0] == 0x49 && header[1] == 0x49 &&
        header[2] == 0x2A && header[3] == 0x00, nil
}

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 51 but for `.nef` extension.
- Only TIFF LE magic (Nikon always uses little-endian).
- `go build ./...` succeeds.

---

## Task 53 — CR2 Filetype Module — `internal/handler/cr2`

**Goal:** Create the Canon CR2 handler as a thin wrapper around `tiffraw.Base`. CR2 has a unique detection advantage: it includes `CR` signature bytes at offset 8–9.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/cr2/cr2.go`**

```go
// Package cr2 implements the FileTypeHandler contract for Canon CR2 RAW images.
//
// CR2 files are TIFF containers with Canon-specific extensions. Unlike other
// TIFF-based RAW formats, CR2 has a unique signature: the standard TIFF LE
// header at offset 0 followed by "CR" (0x43 0x52) at offset 8. This allows
// more reliable detection beyond just extension matching.
package cr2

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".cr2"}
}

// MagicBytes returns the CR2 magic signature.
// CR2 uses TIFF LE header (4 bytes) + IFD offset (4 bytes) + "CR" at offset 8.
// We declare the TIFF LE header at offset 0 for the registry's fast-path check.
func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

// Detect returns true if the file has a .cr2 extension AND begins with
// the TIFF LE header AND has "CR" at offset 8.
func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".cr2" {
        return false, nil
    }
    f, err := os.Open(filePath)
    if err != nil {
        return false, fmt.Errorf("cr2: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    header := make([]byte, 10)
    if _, err := io.ReadFull(f, header); err != nil {
        return false, nil // file too short
    }
    // TIFF LE header at offset 0.
    tiffLE := header[0] == 0x49 && header[1] == 0x49 &&
        header[2] == 0x2A && header[3] == 0x00
    // "CR" signature at offset 8.
    crSig := header[8] == 0x43 && header[9] == 0x52
    return tiffLE && crSig, nil
}

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 51 but for `.cr2` extension.
- `Detect()` checks both TIFF LE header AND `CR` at offset 8.
- `Detect()` returns false for a TIFF file without `CR` at offset 8 (even with `.cr2` extension).
- `go build ./...` succeeds.

---

## Task 54 — PEF Filetype Module — `internal/handler/pef`

**Goal:** Create the Pentax PEF handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/pef/pef.go`**

```go
// Package pef implements the FileTypeHandler contract for Pentax PEF RAW images.
//
// PEF files are TIFF containers with Pentax-specific maker note IFDs.
// The .pef extension is the primary discriminator. Magic bytes confirm
// the file is a valid TIFF container (little-endian only).
package pef

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".pef"}
}

func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".pef" {
        return false, nil
    }
    // ... same TIFF LE header check pattern as NEF ...
}

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 52 but for `.pef` extension.
- `go build ./...` succeeds.

---

## Task 55 — ARW Filetype Module — `internal/handler/arw`

**Goal:** Create the Sony ARW handler as a thin wrapper around `tiffraw.Base`.

**Architecture Reference:** Section 6.4.2, 6.4.8

**Depends on:** Task 50

**File to create: `internal/handler/arw/arw.go`**

```go
// Package arw implements the FileTypeHandler contract for Sony ARW RAW images.
//
// ARW files are TIFF containers with Sony-specific maker note IFDs.
// The .arw extension is the primary discriminator. Magic bytes confirm
// the file is a valid TIFF container (little-endian only).
package arw

import (
    "fmt"
    "io"
    "os"
    "strings"

    "github.com/cwlls/pixe-go/internal/domain"
    "github.com/cwlls/pixe-go/internal/handler/tiffraw"
)

type Handler struct {
    tiffraw.Base
}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".arw"}
}

func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // TIFF LE
    }
}

func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".arw" {
        return false, nil
    }
    // ... same TIFF LE header check pattern as NEF ...
}

func fileExt(path string) string { /* same as other handlers */ }
```

### Acceptance Criteria

- Same structural criteria as Task 52 but for `.arw` extension.
- `go build ./...` succeeds.

---

## Task 56 — CR3 Filetype Module — `internal/handler/cr3`

**Goal:** Create the Canon CR3 handler as a standalone ISOBMFF-based handler. CR3 does **not** embed `tiffraw.Base` — it uses the ISOBMFF container approach established by the HEIC handler.

**Architecture Reference:** Section 6.4.2, 6.4.5, 6.4.6, 6.4.8

**Depends on:** Task 12 (HEIC handler — establishes ISOBMFF patterns)

**File to create: `internal/handler/cr3/cr3.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 ...

// Package cr3 implements the FileTypeHandler contract for Canon CR3 RAW images.
//
// CR3 files use the ISOBMFF container format (like HEIC and MP4), not TIFF.
// This handler is standalone and does not use the tiffraw shared base.
//
// Date extraction:
//   CR3 stores EXIF metadata within the ISOBMFF box structure. The handler
//   parses the container to locate the EXIF blob, then uses rwcarlsen/goexif
//   for standard EXIF tag reading. Fallback chain: DateTimeOriginal →
//   DateTime → Ansel Adams date.
//
// Hashable region:
//   The embedded full-resolution JPEG preview is extracted from the ISOBMFF
//   container. Falls back to full-file hash if extraction fails.
//
// Magic bytes:
//   "ftyp" at offset 4 (same as HEIC/MP4). The ftyp brand "crx " (Canon
//   RAW X) distinguishes CR3 from other ISOBMFF formats. Detection checks
//   both the ftyp box type and the "crx " brand.
//
// Metadata write:
//   No-op stub.
package cr3

import (
    "bytes"
    "encoding/binary"
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    rwexif "github.com/rwcarlsen/goexif/exif"

    "github.com/cwlls/pixe-go/internal/domain"
)

var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

const exifDateFormat = "2006:01:02 15:04:05"

type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) Extensions() []string {
    return []string{".cr3"}
}

// MagicBytes returns the ISOBMFF ftyp box signature at offset 4.
func (h *Handler) MagicBytes() []domain.MagicSignature {
    return []domain.MagicSignature{
        {Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
    }
}

// Detect returns true if the file has a .cr3 extension AND contains the
// ISOBMFF "ftyp" box with "crx " brand.
func (h *Handler) Detect(filePath string) (bool, error) {
    ext := strings.ToLower(fileExt(filePath))
    if ext != ".cr3" {
        return false, nil
    }
    f, err := os.Open(filePath)
    if err != nil {
        return false, fmt.Errorf("cr3: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    header := make([]byte, 12)
    if _, err := io.ReadFull(f, header); err != nil {
        return false, nil // file too short
    }
    // Check "ftyp" at offset 4.
    ftyp := header[4] == 0x66 && header[5] == 0x74 &&
        header[6] == 0x79 && header[7] == 0x70
    if !ftyp {
        return false, nil
    }
    // Check major brand "crx " at offset 8.
    brand := string(header[8:12])
    return brand == "crx ", nil
}
```

### ExtractDate implementation

CR3 stores EXIF data in a specific box path within the ISOBMFF container. The approach:

1. Parse the ISOBMFF box structure to find the EXIF data. CR3 typically stores EXIF in a `moov/uuid` box (Canon uses a UUID-type box) or within a `moov/meta/iinf` path.
2. Alternatively, some CR3 files embed a TIFF-structured EXIF blob that can be located by scanning for the TIFF header signature (`II*\0` or `MM\0*`) within specific boxes.
3. Once the raw EXIF bytes are extracted, parse with `rwexif.Decode()` and apply the standard fallback chain.

```go
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return anselsAdams, fmt.Errorf("cr3: open %q: %w", filePath, err)
    }
    defer func() { _ = f.Close() }()

    exifBytes, err := extractCR3Exif(f)
    if err != nil || len(exifBytes) == 0 {
        return anselsAdams, nil
    }

    x, err := rwexif.Decode(bytes.NewReader(exifBytes))
    if err != nil {
        return anselsAdams, nil
    }

    // 1. DateTimeOriginal
    if tag, err := x.Get(rwexif.DateTimeOriginal); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    // 2. DateTime (IFD0)
    if tag, err := x.Get(rwexif.DateTime); err == nil {
        if s, err := tag.StringVal(); err == nil {
            if t, err := time.Parse(exifDateFormat, strings.TrimSpace(s)); err == nil {
                return t.UTC(), nil
            }
        }
    }

    return anselsAdams, nil
}

// extractCR3Exif parses the ISOBMFF box structure to locate and extract
// the raw EXIF bytes from a CR3 file.
func extractCR3Exif(r io.ReadSeeker) ([]byte, error) { ... }
```

### HashableReader implementation

The embedded JPEG preview in CR3 is typically stored in a `moov/trak` structure or a dedicated preview track. The handler navigates the ISOBMFF box tree to find the largest JPEG blob:

```go
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("cr3: open %q: %w", filePath, err)
    }

    preview, err := findCR3JpegPreview(f)
    if err != nil || preview == nil {
        // Fallback: hash the full file.
        if _, err := f.Seek(0, io.SeekStart); err != nil {
            _ = f.Close()
            return nil, fmt.Errorf("cr3: seek %q: %w", filePath, err)
        }
        return f, nil
    }

    sr := io.NewSectionReader(f, preview.offset, preview.size)
    return &sectionReadCloser{Reader: sr, Closer: f}, nil
}

// findCR3JpegPreview navigates the ISOBMFF box structure to locate
// the embedded full-resolution JPEG preview.
func findCR3JpegPreview(r io.ReadSeeker) (*jpegPreview, error) { ... }
```

### WriteMetadataTags

```go
func (h *Handler) WriteMetadataTags(filePath string, tags domain.MetadataTags) error {
    // CR3 metadata write not supported in pure Go.
    return nil
}
```

### ISOBMFF Box Parsing

The CR3 handler needs a lightweight ISOBMFF box parser. This can be implemented inline (no external dependency needed) since the box structure is simple:

```go
// isobmffBox represents a single ISOBMFF box header.
type isobmffBox struct {
    boxType string // 4-char type code
    size    int64  // total box size including header
    offset  int64  // file offset where the box starts
    dataOff int64  // file offset where the box data starts (after header)
}

// readBox reads the next ISOBMFF box header from the current position.
func readBox(r io.ReadSeeker) (*isobmffBox, error) { ... }

// walkBoxes iterates over top-level boxes in the given range.
func walkBoxes(r io.ReadSeeker, start, end int64) ([]*isobmffBox, error) { ... }
```

### Acceptance Criteria

- `Handler` satisfies `domain.FileTypeHandler`.
- `Extensions()` returns `[".cr3"]`.
- `MagicBytes()` returns ftyp at offset 4.
- `Detect()` checks both ftyp box AND `crx ` brand.
- `Detect()` returns false for HEIC files (ftyp + `heic` brand).
- `Detect()` returns false for MP4 files (ftyp + `isom`/`mp41` brand).
- `ExtractDate()` extracts dates from CR3 EXIF via ISOBMFF parsing.
- `HashableReader()` extracts the embedded JPEG preview or falls back to full file.
- `WriteMetadataTags()` is a no-op.
- `go build ./...` succeeds.

---

## Task 57 — RAW Handler Registration — Wire into CLI

**Goal:** Register all 6 RAW handlers (plus the existing but unregistered HEIC and MP4 handlers) in the three CLI command files.

**Architecture Reference:** Section 6.4.9

**Depends on:** Tasks 51, 52, 53, 54, 55, 56

### Files to modify

#### `cmd/sort.go`

Add imports and registration calls:

```go
import (
    // ... existing imports ...
    heichandler "github.com/cwlls/pixe-go/internal/handler/heic"
    mp4handler  "github.com/cwlls/pixe-go/internal/handler/mp4"
    dnghandler  "github.com/cwlls/pixe-go/internal/handler/dng"
    nefhandler  "github.com/cwlls/pixe-go/internal/handler/nef"
    cr2handler  "github.com/cwlls/pixe-go/internal/handler/cr2"
    cr3handler  "github.com/cwlls/pixe-go/internal/handler/cr3"
    pefhandler  "github.com/cwlls/pixe-go/internal/handler/pef"
    arwhandler  "github.com/cwlls/pixe-go/internal/handler/arw"
)

// In the registry setup (after jpeghandler.New()):
reg.Register(jpeghandler.New())   // existing
reg.Register(heichandler.New())   // was implemented but not registered
reg.Register(mp4handler.New())    // was implemented but not registered
reg.Register(dnghandler.New())    // new
reg.Register(nefhandler.New())    // new
reg.Register(cr2handler.New())    // new
reg.Register(cr3handler.New())    // new
reg.Register(pefhandler.New())    // new
reg.Register(arwhandler.New())    // new
```

#### `cmd/verify.go`

Same imports and registration pattern.

#### `cmd/resume.go`

Same imports and registration pattern.

### Registration Order

JPEG must be registered first. The TIFF-based RAW handlers share magic bytes with each other but have distinct extensions, so their relative order doesn't matter for the extension-based fast path. CR3 shares the ftyp magic with HEIC and MP4, but again, extensions disambiguate.

### Acceptance Criteria

- All 9 handlers (JPEG + HEIC + MP4 + 6 RAW) are registered in `cmd/sort.go`, `cmd/verify.go`, and `cmd/resume.go`.
- JPEG is registered first in all three files.
- `go build ./...` succeeds.
- `./pixe sort --source <dir-with-raw-files> --dest <dirB> --dry-run` discovers and classifies RAW files correctly.

---

## Task 58 — RAW Handlers — Unit Tests

**Goal:** Comprehensive unit tests for the `tiffraw` base and all 6 RAW handler packages.

**Depends on:** Tasks 50, 51, 52, 53, 54, 55, 56

### Files to create

#### `internal/handler/tiffraw/tiffraw_test.go`

Test the shared base logic:

1. **`TestBase_ExtractDate_noEXIF_fallback`** — File with no EXIF returns Ansel Adams date.
2. **`TestBase_ExtractDate_withDateTimeOriginal`** — File with `DateTimeOriginal` returns correct date (requires a real or synthetic TIFF fixture with EXIF).
3. **`TestBase_HashableReader_fullFileFallback`** — File with no embedded JPEG preview returns full file content.
4. **`TestBase_HashableReader_deterministic`** — Two calls return identical bytes.
5. **`TestBase_WriteMetadataTags_noop`** — Returns nil, file unchanged.

**Test fixture strategy:** Create a `buildFakeTIFF(t, dir, name)` helper that writes a minimal valid TIFF file (8-byte header + minimal IFD0). For EXIF tests, create `buildTIFFWithEXIF(t, dir, name, dateStr)` that includes a DateTimeOriginal tag. For JPEG preview tests, create `buildTIFFWithJPEGPreview(t, dir, name)` that embeds a small JPEG blob in IFD1.

#### Per-format test files

Each format gets its own test file following the HEIC test pattern:

**`internal/handler/dng/dng_test.go`**
**`internal/handler/nef/nef_test.go`**
**`internal/handler/cr2/cr2_test.go`**
**`internal/handler/cr3/cr3_test.go`**
**`internal/handler/pef/pef_test.go`**
**`internal/handler/arw/arw_test.go`**

Each test file includes:

1. **`TestHandler_Extensions`** — Verify correct extensions returned.
2. **`TestHandler_MagicBytes`** — Verify correct magic signatures.
3. **`TestHandler_Detect_valid`** — Correct extension + correct magic → true.
4. **`TestHandler_Detect_wrongExtension`** — Correct magic but wrong extension → false.
5. **`TestHandler_Detect_wrongMagic`** — Correct extension but wrong magic → false.
6. **`TestHandler_ExtractDate_noEXIF_fallback`** — Falls back to Ansel Adams.
7. **`TestHandler_HashableReader_returnsData`** — Returns non-empty data.
8. **`TestHandler_HashableReader_deterministic`** — Two calls return identical bytes.
9. **`TestHandler_WriteMetadataTags_noop`** — No-op, file unchanged.

**CR2-specific additional test:**
10. **`TestHandler_Detect_tiffWithoutCR`** — TIFF LE header but no `CR` at offset 8 → false.

**CR3-specific additional tests:**
10. **`TestHandler_Detect_heicBrand`** — ftyp + `heic` brand → false.
11. **`TestHandler_Detect_mp4Brand`** — ftyp + `isom` brand → false.

**Test fixture helpers per format:**

- `buildFakeDNG(t, dir, name)` — minimal TIFF LE file with `.dng` extension
- `buildFakeNEF(t, dir, name)` — minimal TIFF LE file with `.nef` extension
- `buildFakeCR2(t, dir, name)` — TIFF LE + `CR` at offset 8 with `.cr2` extension
- `buildFakeCR3(t, dir, name)` — ISOBMFF ftyp box with `crx ` brand
- `buildFakePEF(t, dir, name)` — minimal TIFF LE file with `.pef` extension
- `buildFakeARW(t, dir, name)` — minimal TIFF LE file with `.arw` extension

### Acceptance Criteria

- All test cases pass across all 7 test files.
- Tests run with `-race` flag without data race warnings.
- Each handler satisfies `domain.FileTypeHandler` (compile-time interface check in each test file).
- Test fixtures are minimal synthetic files — no real camera RAW files checked into the repo.

---

## Task 59 — RAW Handlers — Integration Tests

**Goal:** End-to-end integration tests that exercise the full sort pipeline with RAW fixture files, verifying correct discovery, classification, date extraction, hashing, and output naming.

**Depends on:** Tasks 57, 58

**File to modify: `internal/integration/integration_test.go`**

### New test cases

1. **`TestIntegration_RAW_Discovery`** — Place fixture files for all 6 RAW formats in `dirA`. Run discovery with all handlers registered. Verify:
   - Each file is discovered (not skipped).
   - Each file is matched to the correct handler.
   - No files are misclassified.

2. **`TestIntegration_RAW_FullSort`** — Sort a mix of JPEG + RAW files. Verify:
   - RAW files are copied to `dirB` with correct `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` naming.
   - Extensions are preserved lowercase (`.dng`, `.nef`, `.cr2`, `.cr3`, `.pef`, `.arw`).
   - Database records show `status = "complete"` for all files.
   - Checksums are non-empty and deterministic.

3. **`TestIntegration_RAW_DuplicateDetection`** — Copy the same RAW file twice (different filenames, same content). Verify the second is routed to `duplicates/`.

4. **`TestIntegration_RAW_MixedWithJPEG`** — Sort a directory containing both JPEG and RAW files from the same camera (same dates). Verify both are sorted correctly into the same date directories.

5. **`TestIntegration_RAW_Verify`** — Sort RAW files, then run `pixe verify` on `dirB`. Verify all checksums match.

### Test fixture generation

Use the `buildFake*` helpers from Task 58 to create synthetic RAW files in `t.TempDir()`. These are minimal valid files — they won't produce real images but will exercise the full pipeline.

### Acceptance Criteria

- All 5 integration tests pass.
- Tests run with `-race` flag without data race warnings.
- RAW files flow through the complete pipeline: discover → extract → hash → copy → verify → complete.
- Output filenames use the correct lowercase extensions.
- Database records are correct for all RAW file types.

---

## Task 60 — Tests & Verification — Full Suite Green (RAW)

**Goal:** Verify the entire codebase compiles, passes all tests, and passes lint after adding RAW handler support.

**Depends on:** Tasks 58, 59

### Verification commands

```bash
go vet ./...                                    # No warnings
go build ./...                                  # Compiles cleanly
go test -race -timeout 120s ./...               # All tests pass
make lint                                       # 0 issues
go mod tidy                                     # No diff
```

### Specific checks

1. **All handlers registered:**
   ```bash
   # Each of these should appear in sort.go, verify.go, and resume.go:
   rg 'reg\.Register\(' cmd/sort.go cmd/verify.go cmd/resume.go
   # Expected: 9 registrations per file (jpeg, heic, mp4, dng, nef, cr2, cr3, pef, arw)
   ```

2. **Interface compliance:**
   ```bash
   # Each handler package should have a compile-time interface check:
   rg 'var _ domain.FileTypeHandler' internal/handler/
   ```

3. **No new dependencies beyond what's expected:**
   ```bash
   go mod tidy
   git diff go.mod go.sum
   # The only new dependency should be the TIFF parser if one was added.
   # rwcarlsen/goexif is already a dependency.
   ```

4. **Build smoke test:**
   ```bash
   make build
   # Create test files and verify discovery:
   mkdir -p /tmp/raw-test
   # (create minimal test files with correct headers)
   ./pixe sort --source /tmp/raw-test --dest /tmp/raw-archive --dry-run
   ```

### Acceptance Criteria

- `go vet ./...` — zero warnings.
- `go build ./...` — compiles cleanly.
- `go test -race -timeout 120s ./...` — all tests pass (unit + integration, including all prior tests).
- `make lint` — 0 issues.
- `go mod tidy` produces no diff.
- All 9 handlers are registered in all 3 CLI command files.
- No regressions in existing JPEG, HEIC, or MP4 handler tests.
