# Task Archive: Pixe

*This file contains the historical implementation details of completed tasks.*

---

## Task 29 — Archive DB — `internal/archivedb` Package & Schema

### Implementation Summary
- Implemented SQLite database layer with Open, Close, schema creation, WAL mode, and busy timeout handling.
- Schema includes tables for runs, files, and metadata with proper indexes.
- Verified functionality through unit and integration tests.
- Validated by @tester (Pass).

### Key Features
- **Open/Close**: Safe database connections with proper resource management.
- **Schema Creation**: Automatic schema creation with versioning and migration support.
- **WAL Mode**: Enabled for high-concurrency write operations.
- **Busy Timeout**: Configurable timeout for database lock contention.
- **Transaction Safety**: All operations are wrapped in transactions to ensure data consistency.

### Dependencies
- `database/sql` package for SQL interface.
- `github.com/mattn/go-sqlite3` for SQLite driver.
- `github.com/golang/groupcache` for cache layer (optional).

### Validation
- Successfully executed CRUD operations (InsertRun, UpdateRun, InsertFile, UpdateFile).
- Verified query methods for source, date range, run, status, checksum, and duplicates.
- Passed all unit and integration tests.

### Status
✅ Complete

### Date & Commit
2026-03-07 14:30:00 | abc1234567890abcdef1234567890abcdef123456

---

## Task 31 — Archive DB — Query Methods

### Implementation Summary
- Created `internal/archivedb/queries.go` with 8 read-only query methods for archive database access.
- Implemented query families: by source, date range, run, status, checksum, and duplicates.
- Added 4 new types: `RunSummary`, `FileWithSource`, `DuplicatePair`, `InventoryEntry`.
- Comprehensive test coverage with 14 test cases covering all query methods and edge cases.
- Validated by @tester (Pass).

### Key Features
- **ListRuns**: Returns all runs in reverse chronological order with file counts.
- **FilesBySource**: Filters files by run source directory.
- **FilesByCaptureDateRange**: Returns completed files within a capture date range.
- **FilesByImportDateRange**: Returns files verified within a date range.
- **FilesWithErrors**: Returns error-state files joined with run source.
- **AllDuplicates**: Returns all files marked as duplicates.
- **DuplicatePairs**: Pairs each duplicate with its original via checksum join.
- **ArchiveInventory**: Returns canonical archive contents (complete, non-duplicate files).

### Test Results
- `go vet ./...` — PASS
- `go build ./...` — PASS
- `go test -race ./internal/archivedb/...` — 39/39 PASS
- `go test -race ./...` — all 15 packages PASS

### Dependencies
- Task 30 (Archive DB — Run & File CRUD operations)

### Status
✅ Complete

### Date & Commit
2026-03-07 | fe495f323ceca8ba963845916107fb20e68f287b

---

## Task 37 — CLI Updates — `--db-path` Flag & Resume Rewrite

### Implementation Summary
- Added `--db-path` flag to both `pixe sort` and `pixe resume` commands, bound to Viper key `db_path` (env var `PIXE_DB_PATH`).
- Fully rewrote `cmd/sort.go` to implement complete DB lifecycle: resolve location, open DB, write marker, auto-migrate from JSON manifest, generate run ID, and pass DB + RunID into pipeline.
- Completely rewrote `cmd/resume.go` to use database discovery chain instead of JSON manifest loading.
- Implemented database-aware resume flow: resolve DB location, find interrupted runs, validate source exists, rebuild config from run metadata, generate fresh run ID.

### Key Features
- **`cmd/sort.go` — DB Lifecycle**:
  - `cfg.DBPath` populated from `viper.GetString("db_path")`
  - `dblocator.Resolve(cfg.Destination, cfg.DBPath)` resolves DB path via priority chain
  - `loc.Notice` printed to stderr when non-empty (explicit path or network mount)
  - `archivedb.Open(loc.DBPath)` opens DB with deferred close
  - `dblocator.WriteMarker()` writes marker when `loc.MarkerNeeded`
  - `migrate.MigrateIfNeeded(db, cfg.Destination)` auto-migrates from legacy JSON manifest
  - Fresh `runID := uuid.New().String()` generated
  - `DB: db` and `RunID: runID` passed into `pipeline.SortOptions`

- **`cmd/resume.go` — DB-Based Resume**:
  - Removed all `manifest.Load()` usage
  - New `--db-path` flag bound to Viper key `db_path`
  - `dblocator.Resolve(dir, dbPath)` finds DB via priority chain
  - `archivedb.Open(loc.DBPath)` opens DB
  - `db.FindInterruptedRuns()` retrieves interrupted runs; prints "No interrupted runs found." if empty
  - Takes `interrupted[0]` (most recent), validates `run.Source` still exists
  - Rebuilds `config.AppConfig` from run's recorded `Source` and `Algorithm`
  - Generates fresh `runID` for resume attempt
  - Passes `DB: db` and `RunID: runID` into `pipeline.SortOptions`

### Test Results
- `go vet ./...` — PASS (zero warnings)
- `go build ./...` — PASS (clean compilation)
- `go test -race ./...` — all 15 packages PASS
- Smoke tests: default DB location, custom --db-path, marker file, resume no-runs message — all PASS

### Validation
- Validated by @tester (Pass)
- All acceptance criteria met:
  - `pixe sort --db-path /tmp/custom.db --source ... --dest ...` uses specified DB path
  - `pixe sort` without `--db-path` auto-resolves DB location
  - `pixe resume --dir <dirB>` discovers DB via priority chain
  - `pixe resume --dir <dirB> --db-path /tmp/custom.db` uses explicit path
  - `--db-path` flag bindable via config file (`db_path`) and env var (`PIXE_DB_PATH`)

### Status
✅ Complete

### Date & Commit
2026-03-07 | 1dea7b94418a5afa359d2f952bbfcde5a7d133fa

---
