# Changelog: Pixe

*This file contains the high-level progress of the project for the user. Contents appear with the newest changes at the top.*

---

## [Unreleased] - In Development

- **Features (In Progress)**:
  - Handler metadata tests: `MetadataSupport()` tests added to all handler test files (Task 9 pending).
  - Pipeline integration test: end-to-end test for sidecar written for RAW, embedded for JPEG (Task 12 pending).
  - Full test suite validation: `make check && make test-all` (Task 13 pending).

## [v1.3.0] - 2026-03-11

- **Features**:
  - Metadata capability framework: `MetadataCapability` type and `MetadataSupport()` interface method added to `FileTypeHandler`.
  - Handler metadata declarations: JPEG declares `MetadataEmbed`; HEIC, MP4, CR3, and all TIFF-based RAW formats (DNG, NEF, CR2, PEF, ARW) declare `MetadataSidecar`.
  - XMP sidecar package (`internal/xmp/`): generates Adobe-compatible XMP sidecar files for formats that cannot safely embed metadata.
    - `SidecarPath(mediaPath)` — returns the `.xmp` sidecar path (Adobe convention).
    - `WriteSidecar(mediaPath, tags)` — renders and atomically writes XMP packet with conditional namespace declarations.
    - Pure Go implementation using `text/template`; no external dependencies.
  - Hybrid tagging strategy: pipeline routes metadata writes based on handler capability.
    - `MetadataEmbed` → calls `handler.WriteMetadataTags` (in-file EXIF/atoms).
    - `MetadataSidecar` → writes XMP sidecar via `xmp.WriteSidecar`.
    - `MetadataNone` → no-op, skips tagging entirely.
  - Updated `internal/tagging/tagging.go`: `Apply()` function now dispatches via `handler.MetadataSupport()`.
  - Updated `internal/pipeline/pipeline.go`: sequential sort path now uses `tagging.Apply()` for routing.
  - Updated `internal/pipeline/worker.go`: concurrent worker path now uses `tagging.Apply()` for routing.

- **Improvements**:
  - Clarified `WriteMetadataTags` contract: only called for `MetadataEmbed` handlers. Sidecar/none handlers implement as no-op for interface compliance.
  - MP4 handler: removed lengthy udta atom comment; simplified to clean no-op matching tiffraw and HEIC pattern.
  - Mock handlers in tests updated with `MetadataSupport()` method for compilation.

- **Test Coverage**:
  - `internal/copy/copy_test.go`: added `MetadataSupport()` to `stubHandler`.
  - `internal/discovery/discovery_test.go`: added `MetadataSupport()` to `mockHandler`.
  - `internal/tagging/tagging_test.go`: expanded test suite to cover all three dispatch branches (embed, sidecar, none).

## [v1.2.0] - 2026-03-11

- **Features**:
  - `--recursive` flag (`-r`): descend into subdirectories of `--source` during sort.
  - `--ignore` flag: glob pattern for files to exclude from processing (repeatable; e.g. `--ignore "*.txt" --ignore ".DS_Store"`).
  - `internal/ignore` package: glob matcher with hardcoded `.pixe_ledger.json` ignore at any depth.
  - Skip detection: files already imported in a prior run are skipped with `SKIP <path> -> previously imported`.
  - Schema v2 migration: `recursive` column added to `runs` table; `skip_reason` column and `skipped` status added to `files` table.
  - Ledger format upgraded to v4 JSONL: streaming write replaces buffered JSON array.
  - `LedgerHeader` struct written as line 1 of the ledger; subsequent lines are individual `LedgerEntry` objects.
  - `LedgerWriter` type in `internal/manifest`: nil-safe `WriteEntry` and `Close` methods; coordinator goroutine is sole writer, no mutex needed.
  - Crash-safe ledger: each entry is flushed as it is written; partial writes produce valid JSONL up to the last complete line.
  - Dry-run mode produces no ledger file (`LedgerWriter` stays nil; all calls are no-ops).

- **Improvements**:
  - Pipeline output format standardized: `COPY`, `SKIP`, `DUPE`, `ERR ` verbs with `->` separator on every line.
  - Summary line added: `Done. processed=N duplicates=N skipped=N errors=N`.
  - All outcomes (copy, skip, duplicate, error) now produce both a ledger entry and a DB row.
  - Discovery-phase skips (unsupported format, dotfiles) recorded in ledger and DB.

- **Removals**:
  - Removed `Ledger` struct, `SaveLedger`, and atomic `.tmp`+rename pattern from `internal/manifest`.
  - `LoadLedger` rewritten as JSONL reader returning `*LedgerContents{Header, Entries}` (test utility only).

## [v1.1.1] - 2026-03-08

- **Bug Fixes**:
  - Fixed error return handling in test file fixtures.
  - Replaced `goheif` with pure Go `heic-exif-extractor` for darwin/arm64 compatibility.

## [v1.1.0] - 2026-03-08

- **Features**:
  - RAW format support: DNG, NEF, CR2, CR3, PEF, ARW — all 9 handlers now registered in CLI commands.
  - Shared TIFF-RAW base (`internal/handler/tiffraw`): EXIF extraction and embedded JPEG preview for DNG, NEF, CR2, PEF, ARW.
  - CR3 handler using ISOBMFF container parsing (same approach as HEIC/MP4).
  - Added integration tests for RAW handler pipeline.

- **Bug Fixes**:
  - Fixed errcheck lint warnings in `resume` and `sort` commands.

## [v1.0.2] - 2026-03-07

- **Bug Fixes**:
  - Updated goreleaser config to use non-deprecated `archives` format key.
  - Fixed deprecated `StringToPtr` → `UTF16PtrFromString` in dblocator (Windows).

## [v1.0.1] - 2026-03-07

- **Bug Fixes**:
  - Added Windows network mount detection to dblocator.
  - Updated `.gitignore`.

## [v1.0.0] - 2026-03-07

- **Features**:
  - SQLite archive database (`internal/archivedb`): cumulative registry of all files ever sorted, using CGo-free `modernc.org/sqlite` with WAL mode and busy timeout.
  - Database path resolution (`internal/dblocator`): explicit `--db-path` > marker file > local default; network mount detection for automatic fallback.
  - Auto-migration from legacy JSON manifest to SQLite (`internal/migrate`): transparent on first run after upgrade.
  - Cross-process dedup race handling: `CompleteFileWithDedupCheck` atomically detects and routes duplicates when two `pixe sort` processes run simultaneously.
  - `--db-path` flag on `pixe sort` and `pixe resume`.
  - Run ID (UUID) written to ledger, linking the human-readable receipt to the archive database record.

- **Improvements**:
  - `pixe resume` rewritten to use database discovery chain instead of JSON manifest.
  - Ledger bumped to v2 with `run_id` field.
  - Version management refactored: `internal/version` package replaced with idiomatic ldflags injection into `cmd` package.

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

## [v0.9.6] - 2026-03-07

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

## [v0.9.5] - 2026-03-07

- **Refactor**:
  - Renamed the module to `github.com/cwlls/pixe-go` for better clarity and consistency.

- **Documentation**:
  - Added a project README to document the project's purpose and setup.
  - Updated the architectural overview to include version management details.

## [v0.9.4] - 2026-03-07

- **Chore**:
  - Removed a duplicate LICENSE file and added Apache-2.0 license headers to all source files.

## [v0.9.3] - 2026-03-07

- **Initial Commit**:
  - Project scaffold and Go module initialized.

- **Foundation**:
  - Established the core domain structure and interfaces.

## [v0.9.2] - 2026-03-07

- **Initial Commit**:
  - Project scaffold and Go module initialized.

## [v0.9.1] - 2026-03-07

- **Initial Commit**:
  - Project scaffold and Go module initialized.

## [v0.9.0] - 2026-03-06

- **Initial Commit**:
  - Project scaffold and Go module initialized.

> All changes are tracked in the git history. For detailed commit logs, see the full git log.

*Note: Version numbers are derived directly from git tags. Semantic versioning is followed with major, minor, and patch updates reflecting feature additions, improvements, and bug fixes.*
