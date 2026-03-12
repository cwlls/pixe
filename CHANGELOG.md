# Changelog: Pixe

*This file contains the high-level progress of the project for the user. Contents appear with the newest changes at the top.*

---

## [2.0.0] -- 2026-03-11

### Added

- **`pixe gui` interactive terminal UI** — A full-featured TUI launched with `pixe gui`. Uses Bubble Tea + Lip Gloss (Charm Bracelet stack). Requires a TTY.
  - **Three tabs**: Sort (configure and run a sort with live progress bar, activity log, per-worker status), Verify (configure and run a verify with live progress bar and activity log), Status (background walk + ledger classification into 5 categories: Sorted, Duplicates, Errored, Unsorted, Unrecognised).
  - **Key bindings**: `Tab`/`Shift+Tab` cycle tabs; `1`/`2`/`3` jump to Sort/Verify/Status; `s` start sort; `v` start verify; `f` cycle activity log filter; `n` new run; `e` filter to errors; `j`/`k`/`↑`/`↓` scroll; `r` refresh; `q`/`Ctrl+C` quit.
  - **Flags**: Same as `pixe sort` — `--source`, `--dest`, `--workers`, `--algorithm`, `--copyright`, `--camera-owner`, `--dry-run`, `--db-path`, `--recursive`, `--skip-duplicates`, `--ignore`, `--no-carry-sidecars`, `--overwrite-sidecar-tags`.

- **`--progress` flag on `pixe sort` and `pixe verify`** — Opt-in live progress bar for the existing CLI commands. Only activates when stdout is a TTY; falls back to plain text otherwise.
  - Shows a gradient progress bar with file count and percentage, ETA estimate, current file being processed (sort mode), and status counters (copied/dupes/skipped/errors for sort; verified/mismatches/unrecognised for verify).
  - Example: `pixe sort --dest ~/Archive --progress` or `pixe verify --dir ~/Archive --progress`.

- **Internal: Pipeline event bus** (`internal/progress/`) — Pure stdlib package. The sort and verify pipelines now emit structured `progress.Event` values alongside their existing plain-text output. Both can be active simultaneously — the `--progress` flag and `pixe gui` consume events; plain text is the default.

- **New dependencies**: bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0.

- **Files Added**:
  - `cmd/gui.go` — Cobra command for the interactive TUI
  - `internal/progress/` — Pipeline event bus package
  - `internal/cli/progress.go` — Progress bar model (Bubble Tea)
  - `internal/tui/` — TUI package (Charm Bracelet stack)

## [1.8.0] -- 2026-03-11

### Added

- **Enhanced ignore system** with three new capabilities:
  - **Recursive glob support** — `--ignore "**/*.txt"` now excludes `.txt` files at any depth. Uses `bmatcuk/doublestar/v4` library for glob matching.
  - **Directory-level ignore patterns** — Patterns ending with `/` (e.g., `--ignore "node_modules/"`) skip entire directories without descending. Patterns ending with `/**` also trigger directory skipping.
  - **`.pixeignore` files** — A `.pixeignore` file placed in the source directory (or any subdirectory) is loaded automatically. Patterns in it are scoped to that directory and its descendants. Format: one pattern per line, `#` comments, blank lines ignored. Negation (`!`) is NOT supported. The `.pixeignore` file itself is always invisible to the pipeline (hardcoded ignore, like `.pixe_ledger.json`).

## [1.7.0] - 2026-03-11

- **Features**:
  - `pixe clean` command: maintenance subcommand for archive hygiene with three responsibilities:
    - **Orphaned temp file cleanup** — Scans the destination archive (dirB) for `.pixe-tmp` files left behind by interrupted sort runs and removes them.
    - **Orphaned XMP sidecar cleanup** — Detects Pixe-generated `.xmp` sidecar files whose corresponding media file no longer exists (regex-gated to `^\d{8}_\d{6}_[0-9a-f]+\..+\.xmp$` to avoid removing user-created XMP files).
    - **Database compaction** — Runs `VACUUM` on the archive SQLite database to reclaim space from long-lived archives with many runs. Includes an active-run safety guard that refuses to vacuum if a sort is currently in progress.
  - Flags: `--dir, -d` (required), `--db-path` (explicit database path), `--dry-run` (preview without modifying), `--temp-only` (skip database compaction), `--vacuum-only` (skip file scanning). `--temp-only` and `--vacuum-only` are mutually exclusive.

- **Files Added**:
  - `cmd/clean.go` — Full Cobra command implementation
  - `cmd/clean_test.go` — 13 unit tests
  - `internal/integration/clean_test.go` — 4 integration tests

- **Files Modified**:
  - `internal/archivedb/queries.go` — Added `Vacuum()` and `HasActiveRuns()` methods
  - `internal/archivedb/archivedb_test.go` — 6 unit tests for new DB methods
  - `.state/ARCHITECTURE.md` — Section 7.5 design spec added

## [v1.6.2] - 2026-03-11

- **Improvements**:
  - RAW file hashing strategy changed from embedded JPEG preview to raw sensor data for improved data integrity. JPEG previews are unstable (software tools like Lightroom can regenerate them, causing false-negative deduplication) and ambiguous (burst shots can produce identical previews for different exposures). Sensor data is the immutable ground truth.
  - `internal/handler/tiffraw/tiffraw.go`: `HashableReader` now navigates the TIFF IFD chain to locate the sensor data IFD (identified by non-JPEG compression type: `1`=uncompressed, `7`=lossless JPEG, `34713`=NEF compressed, etc.). Uses a `multiSectionReader` to stream all strips/tiles as a single contiguous byte sequence. Falls back to full-file hash if no sensor data IFD is found. Affects: DNG, NEF, CR2, PEF, ARW.
  - `internal/handler/cr3/cr3.go`: `HashableReader` now navigates ISOBMFF `moov → trak → mdia → minf → stbl` to find chunk offsets (`stco`/`co64`) and sample sizes (`stsz`) for the primary image track (largest total sample size). Falls back to the full `mdat` box contents if track parsing fails. Affects: CR3.
  - Performance note: Hashing sensor data reads more bytes than hashing the JPEG preview (20–80 MB vs 1–5 MB per file). This trade-off is accepted: data integrity is Pixe's first principle, and RAW file users expect processing overhead proportional to file size.

- **Test Coverage**:
  - Tests updated with new fixtures covering: sensor data extraction, preference over JPEG preview, multi-strip concatenation, tiled layout, JPEG-only fallback, no-mdat nil return, mdat fallback.

## [v1.6.1] - 2026-03-11

- **Features**:
  - `--source` flag is now optional for both `pixe sort` and `pixe status` commands, defaulting to the current working directory when omitted. Explicit `--source` still overrides the default.

- **Test Coverage**:
  - Added `TestSortCmd_sourceNotRequired` in `cmd/sort_test.go`.
  - Added `TestRunStatus_defaultsToCwd` and `TestRunStatus_sourceOverridesCwd` in `cmd/status_test.go`.

## [v1.6.0] - 2026-03-11

- **Features**:
  - `pixe status` command: source-oriented, read-only command that reports the sorting status of a source directory by comparing files on disk against the `.pixe_ledger.json` left by prior `pixe sort` runs. No archive database or destination directory required.
    - Walks the source directory using the same handler registry as `pixe sort`.
    - Loads the `.pixe_ledger.json` ledger file from the source directory.
    - Classifies every file into one of five categories: SORTED (ledger entry with `status: "copy"`), DUPLICATE (ledger entry with `status: "duplicate"`), ERRORED (ledger entry with `status: "error"`), UNSORTED (no ledger entry or `status: "skip"`), UNRECOGNIZED (no handler claims this file type).
    - Outputs a sectioned listing with a summary line.
    - Flags: `--source` / `-s` (required), `--recursive` / `-r` (default: false), `--ignore` (repeatable), `--json` (emit JSON output).
    - Exit code 0 always on success (unsorted files are not an error condition).

- **Test Coverage**:
  - Unit tests added: 13 tests in `cmd/status_test.go`.
  - Integration tests added: 4 tests in `internal/integration/status_test.go`.

## [v1.5.0] - 2026-03-11

- **Features**:
  - `--skip-duplicates` flag on `pixe sort`: skip copying duplicate files instead of copying to `duplicates/`. When active, duplicate files are detected and checksummed but not physically copied to `dirB`. DB row is marked `status='complete'`, `is_duplicate=1`, with NULL `dest_path`/`dest_rel`. Ledger entry includes `status:"duplicate"` and `checksum` but omits `destination` field.
  - Atomic copy-then-verify via temp file: `copy.Execute` now writes to a uniquely-named temp file (`.<basename>.pixe-tmp-<random>`) in the destination directory, never touching the canonical path during copy. `copy.Verify` re-hashes the temp file. `copy.Promote` atomically renames temp → canonical path only after verification passes. Guarantees: a file at its canonical path in `dirB` is always complete and verified; partial files never appear at canonical paths.

- **Improvements**:
  - `CheckDuplicate` now returns `"<duplicate>"` sentinel when a complete row exists with NULL `dest_rel`, ensuring skipped-duplicate rows are correctly detected as duplicates by subsequent runs.
  - Concurrent race condition fixed in no-DB mode: `memSeen` is now updated at assignment time, and `os.CreateTemp` ensures unique temp file names so concurrent workers never overwrite each other's temp files.

- **Bug Fixes**:
  - Interrupted run safety: orphaned temp files are left on disk but do not interfere with subsequent runs. They are identifiable by the `.pixe-tmp` suffix and can be cleaned by a future `pixe clean` command.

- **Test Coverage**:
  - Integration tests added: `TestSort_noPartialFilesOnInterrupt`, `TestSort_tempFileCleanupOnResume`, `TestSort_verifiedFileAtCanonicalPath`.

## [v1.4.0] - 2026-03-11

- **Features**:
  - `pixe query` command group: read-only interrogation of the archive SQLite database via 7 subcommands.
    - `pixe query runs` — list all sort runs with file counts, ordered by start time.
    - `pixe query run <id>` — show metadata and file list for a single run; supports short-prefix ID matching.
    - `pixe query duplicates` — list all duplicate files; `--pairs` flag shows each duplicate alongside its original.
    - `pixe query errors` — list all files in error states (`failed`, `mismatch`, `tag_failed`) across all runs.
    - `pixe query skipped` — list all skipped files with skip reasons.
    - `pixe query files` — filter archive files by capture date range (`--from`/`--to`), import date range (`--imported-from`/`--imported-to`), or source directory (`--source`).
    - `pixe query inventory` — list all canonical (complete, non-duplicate) files in the archive.
  - All `pixe query` subcommands support `--json` for machine-readable output (envelope: `query`, `dir`, `results`, `summary`).
  - New `archivedb` methods: `OpenReadOnly`, `AllSkipped`, `GetRunByPrefix`, `ArchiveStats`.

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
