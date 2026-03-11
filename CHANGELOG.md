# Changelog: Pixe

*This file contains the high-level progress of the project for the user. Contents appear with the newest changes at the top.*

---

## [Unreleased]

- **Features**:
  - Ledger format upgraded to v4 JSONL: streaming write replaces buffered JSON array.
  - `LedgerHeader` struct written as line 1 of the ledger; subsequent lines are individual `LedgerEntry` objects.
  - `LedgerWriter` type in `internal/manifest`: nil-safe `WriteEntry` and `Close` methods; coordinator goroutine is sole writer, no mutex needed.
  - Crash-safe ledger: each entry is flushed as it is written; partial writes produce valid JSONL up to the last complete line.
  - Dry-run mode produces no ledger file (`LedgerWriter` stays nil; all calls are no-ops).

- **Removals**:
  - Removed `Ledger` struct, `SaveLedger`, and atomic `.tmp`+rename pattern from `internal/manifest`.
  - `LoadLedger` rewritten as JSONL reader returning `*LedgerContents{Header, Entries}` (test utility only).

## [v0.12.0] - 2026-03-11

- **Features**:
  - `--recursive` flag (`-r`): descend into subdirectories of `--source` during sort.
  - `--ignore` flag: glob pattern for files to exclude from processing (repeatable; e.g. `--ignore "*.txt" --ignore ".DS_Store"`).
  - `internal/ignore` package: glob matcher with hardcoded `.pixe_ledger.json` ignore at any depth.
  - Skip detection: files already imported in a prior run are skipped with `SKIP <path> -> previously imported`.
  - Schema v2 migration: `recursive` column added to `runs` table; `skip_reason` column and `skipped` status added to `files` table.

- **Improvements**:
  - Pipeline output format standardized: `COPY`, `SKIP`, `DUPE`, `ERR ` verbs with `->` separator on every line.
  - Summary line added: `Done. processed=N duplicates=N skipped=N errors=N`.
  - All outcomes (copy, skip, duplicate, error) now produce both a ledger entry and a DB row.
  - Discovery-phase skips (unsupported format, dotfiles) recorded in ledger and DB.

## [v0.11.0] - 2026-03-08

- **Features**:
  - SQLite archive database (`internal/archivedb`): cumulative registry of all files ever sorted, using CGo-free `modernc.org/sqlite` with WAL mode and busy timeout.
  - Database path resolution (`internal/dblocator`): explicit `--db-path` > marker file > local default; network mount detection for automatic fallback.
  - Auto-migration from legacy JSON manifest to SQLite (`internal/migrate`): transparent on first run after upgrade.
  - Cross-process dedup race handling: `CompleteFileWithDedupCheck` atomically detects and routes duplicates when two `pixe sort` processes run simultaneously.
  - `--db-path` flag on `pixe sort` and `pixe resume`.
  - Run ID (UUID) written to ledger, linking the human-readable receipt to the archive database record.
  - RAW format support: DNG, NEF, CR2, CR3, PEF, ARW — all 9 handlers now registered in CLI commands.
  - Shared TIFF-RAW base (`internal/handler/tiffraw`): EXIF extraction and embedded JPEG preview for DNG, NEF, CR2, PEF, ARW.
  - CR3 handler uses ISOBMFF container parsing (same approach as HEIC/MP4).

- **Improvements**:
  - `pixe resume` rewritten to use database discovery chain instead of JSON manifest.
  - Ledger bumped to v3 with `run_id`, `recursive`, `status`, `reason`, and `matches` fields per entry.

## [v0.10.0] - 2026-03-07

- **Features**:
  - Locale-aware month directory names (MM-Mon format) for better internationalization.
  - Centralized version management to ensure consistent versioning across all components.

- **Improvements**:
  - Fixed issues with template versioning and ldflags path in goreleaser configuration.
  - Improved JPEG entropy data parsing by correctly identifying the EOI marker.
  - Enhanced Go linting workflows by removing deprecated configurations and updating to the latest golangci-lint version.

- **Bug Fixes**:
  - Resolved all golangci-lint violations in the codebase.
  - Fixed formatting issues in developer documentation.
  - Corrected release permissions and version bumping logic.

- **Other**:
  - Updated release configuration (`release.yml`) and added comprehensive linting and testing workflows for GitHub Actions.

## [v0.9.6] - 2026-03-06

- **Features**:
  - Implemented core domain types and interfaces for a robust foundation.
  - Added support for HEIC and MP4 file types through new handlers and processing pipelines.
  - Introduced a worker pool for efficient parallel processing of file operations.
  - Added the `pixe sort` CLI command to enable sorting of files by metadata.

- **Engine Implementations**:
  - Built the Sort Pipeline Orchestrator to manage the sorting workflow.
  - Developed the Copy & Verify Engine to ensure data integrity during operations.
  - Implemented the Path Builder to construct file paths dynamically.
  - Added a hashing engine with persistent manifest storage for file discovery and verification.

- **Other**:
  - Marked all related tasks (11-16) as complete in the project state.
  - Added a Makefile with common development targets to streamline local development.
  - Conducted integration tests and a safety audit to validate system reliability.

## [v0.9.5] - 2026-03-06

- **Refactor**:
  - Renamed the module to `github.com/cwlls/pixe-go` for better clarity and consistency.

- **Documentation**:
  - Added a project README to document the project's purpose and setup.
  - Updated the architectural overview to include version management details.

## [v0.9.4] - 2026-03-06

- **Chore**:
  - Removed a duplicate LICENSE file and added Apache-2.0 license headers to all source files.

## [v0.9.3] - 2026-03-06

- **Initial Commit**:
  - Project scaffold and Go module initialized.

- **Foundation**:
  - Established the core domain structure and interfaces.

## [v0.9.2] - 2026-03-06

- **Initial Commit**:
  - Project scaffold and Go module initialized.

## [v0.9.1] - 2026-03-06

- **Initial Commit**:
  - Project scaffold and Go module initialized.

## [v0.9.0] - 2026-03-06

- **Initial Commit**:
  - Project scaffold and Go module initialized.

> All changes are tracked in the git history. For detailed commit logs, see the full git log.

*Note: Version numbers are derived directly from git tags. Semantic versioning is followed with major, minor, and patch updates reflecting feature additions, improvements, and bug fixes.*
