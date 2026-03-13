---
layout: page
title: Changelog
section_label: History
permalink: /changelog/
---

*Newest changes at the top. Version numbers are derived directly from git tags.*

---

## [2.0.0] — 2026-03-11

### Added

- **`--progress` flag on `pixe sort` and `pixe verify`** — Opt-in live progress bar for the existing CLI commands. Only activates when stdout is a TTY; falls back to plain text otherwise.
  - Shows a gradient progress bar with file count and percentage, ETA estimate, current file being processed (sort mode), and status counters (copied/dupes/skipped/errors for sort; verified/mismatches/unrecognised for verify).
  - Example: `pixe sort --dest ~/Archive --progress` or `pixe verify --dir ~/Archive --progress`.

- **Internal: Pipeline event bus** (`internal/progress/`) — Pure stdlib package. The sort and verify pipelines now emit structured `progress.Event` values alongside their existing plain-text output. Both can be active simultaneously — `--progress` consumes events while plain text remains the default.

- **New dependencies**: bubbletea v1.3.10, bubbles v1.0.0, lipgloss v1.1.0 (used by the `--progress` bar and terminal styling).

## [1.9.0] — 2026-03-11

### Added

- **Source Sidecar Carry** — When sorting media files from a source directory, Pixe now detects and carries pre-existing `.aae` and `.xmp` sidecar files alongside their parent media files to the destination archive.
  - **On by default** — sidecar carry is enabled unless `--no-carry-sidecars` is passed.
  - **Association by stem matching** — `IMG_1234.xmp` associates with `IMG_1234.HEIC`; `IMG_1234.HEIC.xmp` (full-extension Adobe convention) is also supported and takes priority.
  - **Case-insensitive** — `img_1234.xmp` matches `IMG_1234.HEIC`.
  - **Destination naming** — sidecar is renamed to match the destination media file: `20211225_062223_7d97e98f.heic.aae`.
  - **XMP tag merge** — when a `.xmp` sidecar is carried AND `--copyright`/`--camera-owner` is configured, Pixe merges the tags into the carried sidecar instead of generating a new one. Source `.xmp` values are authoritative by default (existing fields preserved); `--overwrite-sidecar-tags` inverts this.
  - **Orphan sidecars** — sidecars with no matching media file are reported as `SKIP` with reason `orphan sidecar: no matching media file`.
  - **Dry-run** — `+sidecar` lines appear in output showing what would be carried, no files copied.
  - **Duplicates** — sidecars follow their parent to `duplicates/` when the parent is a duplicate; sidecars are not copied when `--skip-duplicates` is active.
- **New CLI flags on `pixe sort`:**
  - `--no-carry-sidecars` — disable sidecar carry (default: carry is enabled).
  - `--overwrite-sidecar-tags` — when merging tags into a carried `.xmp`, replace existing values instead of preserving them (default: preserve).
- **New stdout output format** — `+sidecar` lines appear indented under their parent media file, showing what sidecar was carried and to where.
- **Database schema v3** — new `carried_sidecars` column stores the JSON array of carried sidecar relative paths per file. The ledger entry gains a `sidecars` field.

## [1.8.0] — 2026-03-11

### Added

- **Enhanced ignore system** with three new capabilities:
  - **Recursive glob support** — `--ignore "**/*.txt"` now excludes `.txt` files at any depth. Uses `bmatcuk/doublestar/v4` library for glob matching.
  - **Directory-level ignore patterns** — Patterns ending with `/` (e.g., `--ignore "node_modules/"`) skip entire directories without descending. Patterns ending with `/**` also trigger directory skipping.
  - **`.pixeignore` files** — A `.pixeignore` file placed in the source directory (or any subdirectory) is loaded automatically. Patterns in it are scoped to that directory and its descendants. Format: one pattern per line, `#` comments, blank lines ignored. Negation (`!`) is NOT supported. The `.pixeignore` file itself is always invisible to the pipeline (hardcoded ignore, like `.pixe_ledger.json`).

## [1.7.0] — 2026-03-11

- **Features**:
  - `pixe clean` command: maintenance subcommand for archive hygiene with three responsibilities:
    - **Orphaned temp file cleanup** — Scans the destination archive for `.pixe-tmp` files left behind by interrupted sort runs and removes them.
    - **Orphaned XMP sidecar cleanup** — Detects Pixe-generated `.xmp` sidecar files whose corresponding media file no longer exists (regex-gated to avoid removing user-created XMP files).
    - **Database compaction** — Runs `VACUUM` on the archive SQLite database to reclaim space from long-lived archives with many runs. Includes an active-run safety guard that refuses to vacuum if a sort is currently in progress.
  - Flags: `--dir, -d` (required), `--db-path`, `--dry-run`, `--temp-only`, `--vacuum-only`. `--temp-only` and `--vacuum-only` are mutually exclusive.

## [v1.6.2] — 2026-03-11

- **Improvements**:
  - RAW file hashing strategy changed from embedded JPEG preview to raw sensor data for improved data integrity. JPEG previews are unstable (software tools like Lightroom can regenerate them, causing false-negative deduplication) and ambiguous (burst shots can produce identical previews for different exposures). Sensor data is the immutable ground truth.
  - `HashableReader` now navigates the TIFF IFD chain to locate the sensor data IFD for DNG, NEF, CR2, PEF, ARW. Falls back to full-file hash if no sensor data IFD is found.
  - CR3 `HashableReader` now navigates ISOBMFF `moov → trak → mdia → minf → stbl` to find the primary image track. Falls back to the full `mdat` box contents if track parsing fails.

## [v1.6.1] — 2026-03-11

- **Features**:
  - `--source` flag is now optional for both `pixe sort` and `pixe status` commands, defaulting to the current working directory when omitted.

## [v1.6.0] — 2026-03-11

- **Features**:
  - `pixe status` command: source-oriented, read-only command that reports the sorting status of a source directory by comparing files on disk against the `.pixe_ledger.json` left by prior `pixe sort` runs. No archive database or destination directory required.
  - Classifies every file into one of five categories: SORTED, DUPLICATE, ERRORED, UNSORTED, UNRECOGNIZED.
  - Flags: `--source` / `-s`, `--recursive` / `-r`, `--ignore`, `--json`.

## [v1.5.0] — 2026-03-11

- **Features**:
  - `--skip-duplicates` flag on `pixe sort`: skip copying duplicate files instead of copying to `duplicates/`.
  - Atomic copy-then-verify via temp file: writes to a uniquely-named temp file, verifies, then atomically renames to canonical path. A file at its canonical path in the archive is always complete and verified.

## [v1.4.0] — 2026-03-11

- **Features**:
  - `pixe query` command group: read-only interrogation of the archive SQLite database via 7 subcommands: `runs`, `run <id>`, `duplicates`, `errors`, `skipped`, `files`, `inventory`.
  - All subcommands support `--json` for machine-readable output.

## [v1.3.0] — 2026-03-11

- **Features**:
  - Metadata capability framework: `MetadataCapability` type and `MetadataSupport()` interface method added to `FileTypeHandler`.
  - JPEG declares `MetadataEmbed`; HEIC, MP4, CR3, and all TIFF-based RAW formats declare `MetadataSidecar`.
  - XMP sidecar package (`internal/xmp/`): generates Adobe-compatible XMP sidecar files for formats that cannot safely embed metadata.
  - Hybrid tagging strategy: pipeline routes metadata writes based on handler capability.

## [v1.2.0] — 2026-03-11

- **Features**:
  - `--recursive` flag (`-r`): descend into subdirectories of `--source` during sort.
  - `--ignore` flag: glob pattern for files to exclude from processing (repeatable).
  - `internal/ignore` package: glob matcher with hardcoded `.pixe_ledger.json` ignore at any depth.
  - Skip detection: files already imported in a prior run are skipped with `SKIP <path> -> previously imported`.
  - Ledger format upgraded to v4 JSONL: streaming write replaces buffered JSON array.

## [v1.1.1] — 2026-03-08

- **Bug Fixes**:
  - Replaced `goheif` with pure Go `heic-exif-extractor` for darwin/arm64 compatibility.

## [v1.1.0] — 2026-03-08

- **Features**:
  - RAW format support: DNG, NEF, CR2, CR3, PEF, ARW — all 9 handlers now registered in CLI commands.
  - Shared TIFF-RAW base (`internal/handler/tiffraw`): EXIF extraction and sensor data hashing for DNG, NEF, CR2, PEF, ARW.
  - CR3 handler using ISOBMFF container parsing (same approach as HEIC/MP4).

## [v1.0.0] — 2026-03-07

- **Features**:
  - SQLite archive database (`internal/archivedb`): cumulative registry of all files ever sorted, using CGo-free `modernc.org/sqlite` with WAL mode and busy timeout.
  - Database path resolution (`internal/dblocator`): explicit `--db-path` > marker file > local default; network mount detection for automatic fallback.
  - Auto-migration from legacy JSON manifest to SQLite: transparent on first run after upgrade.
  - Cross-process dedup race handling: atomically detects and routes duplicates when two `pixe sort` processes run simultaneously.

## [v0.10.0] — 2026-03-07

- **Features**:
  - Locale-aware month directory names (MM-Mon format).
  - Centralized version management via ldflags injection.

## [v0.9.6] — 2026-03-07

- **Features**:
  - Core domain types and interfaces.
  - HEIC and MP4 file type support.
  - Worker pool for parallel processing.
  - `pixe sort` CLI command.
  - Sort Pipeline Orchestrator, Copy & Verify Engine, Path Builder, hashing engine.

## [v0.9.5] — 2026-03-07

- Renamed module to `github.com/cwlls/pixe-go`.

## [v0.9.0–v0.9.4] — 2026-03-06/07

- Initial project scaffold, Go module initialization, Apache-2.0 license headers.

---

> All changes are tracked in the git history. For detailed commit logs, see the [full git log](https://github.com/cwlls/pixe-go/commits/main){:target="_blank" rel="noopener"}.
