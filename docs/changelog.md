---
title: Changelog
nav_order: 10
---

# Changelog

*Newest changes at the top. Version numbers are derived directly from git tags.*

> **Version scheme change (March 2026):** Pixe migrated from three-segment semver (`v2.7.3`) to
> two-segment `major.minor` (`v0.23`). All releases were re-tagged within the `v0.x` range.
> The changelog entries below retain their original version numbers for historical reference.
> See [ARCHITECTURE.md §3.2](https://github.com/cwlls/pixe/blob/main/.state/ARCHITECTURE.md)
> for the complete old→new tag mapping.

<!-- pixe:begin:changelog -->

## [Unreleased] -- Config File Bug Fix: Alias Sigil and Parse Error Reporting

### Bug Fixes

- **Fixed YAML parse failure with alias syntax** — The alias prefix was changed from `@` to `+`. The `@` character is reserved in YAML 1.1 (as a tag indicator), causing config files with unquoted alias values (e.g., `dest: @nas`) to silently fail to parse with `yaml: found character that cannot start any token`. The `+` character is inert in YAML, shells, and environment variables — no quoting or escaping required in any context. This is a **breaking change** for any existing config files or scripts using `@`-prefixed aliases; update them to use `+` instead.

- **Fixed silent config file parse errors** — The `initConfig()` function previously swallowed all `viper.ReadInConfig()` errors silently, giving users no feedback when their config file failed to parse. Now:
  - **Config not found** (expected case) — remains silent.
  - **Auto-discovered config parse error** — prints a warning to stderr with the file path and error details, then continues (non-fatal).
  - **Explicit `--config` failure** — prints an error to stderr and exits with a fatal error (the user explicitly requested this file).

### Changed

- **Alias prefix in all contexts** — Updated from `@` to `+` in:
  - `cmd/helpers.go` — `resolveAlias()` function
  - `cmd/helpers_test.go` — all 8 test functions
  - `docs/configuration.md` — all 8 alias examples
  - `.state/ARCHITECTURE.md` — §4.15 and §7.2

### Added

- **Config parse error tests** — New `cmd/root_test.go` with 6 test functions covering valid config, malformed YAML (auto-discovery and explicit), missing files, and `+` sigil in config values.

---

## [Unreleased] -- Lint Fixes

### Chore

- **Fixed ineffassign lint errors in progress reader tests** — Resolved three `ineffassign` violations in `internal/progress/reader_test.go` where `err` return values were assigned but never checked before being overwritten. Changed unused assignments to blank identifiers (`_`) in `TestProgressReader_NilBus`, `TestProgressReader_EOFFinalEvent`, and `TestProgressReader_UnknownSize`. All tests pass; `make lint` clean.

---

## [Unreleased] -- UX Improvements: Ledger Prompt, Sidecar Annotations, Run Duration, Truncation Ellipsis

### Added

- **Interactive ledger write failure prompt** — When the pipeline cannot create the JSONL ledger in the source directory (e.g., read-only filesystem, permission denied), the user is prompted before processing begins. The prompt explains the consequence and offers a choice to continue without a ledger or cancel. Cancellation exits with code 0 (not an error).

- **`--yes` / `-y` flag** — Auto-accept interactive prompts (e.g., continue without ledger). Useful for scripting and CI environments.

- **`--no-ledger` flag** — Explicitly skip ledger creation without prompting or warning. Recommended for scripts that intentionally run without a ledger.

- **Inline sidecar annotations in sort output** — When a file has associated sidecar files (`.xmp`, `.aae`) that were carried alongside it, this is now indicated with an inline annotation on the parent file's output line rather than a separate sub-line. Format: `[+xmp]`, `[+aae]`, or `[+xmp +aae]` if multiple sidecars are present. Example: `COPY IMG_0001.jpg -> 2021/12-Dec/20211225_062223-1-7d97e98f.jpg [+aae]`. This keeps the output compact — one line per file, always.

- **Inline sidecar annotations in verify output** — The verify command now recognizes sidecar files (`.xmp`, `.aae`) as expected artifacts and does not report them as unrecognised. Associated sidecars are shown as inline annotations on the parent file's verification line (same format as sort). Orphaned sidecars (no matching parent) are still reported as unrecognised.

- **Run duration tracking and display** — Sort run elapsed time is now tracked and displayed across all contexts:
   - Sort summary includes a second line with elapsed time: `(1m 23s)`
   - `query run <id>` header includes a `Duration:` line
   - `query runs` table includes a `DURATION` column
   - JSON output includes `duration_seconds` field (float64)
   - Duration is computed on the fly from `started_at` and `finished_at` — no schema change
   - TUI progress bar (`--progress` mode) shows total elapsed time in the final status counter line for both `sort` and `verify` commands (e.g., `copied: 38  │  dupes: 2  │  skipped: 1  │  errors: 0  (1m 23s)`)

- **Truncation ellipsis in query output** — Truncated checksums and run IDs in table display now show a trailing ellipsis (`…`) to visually indicate the value is not complete. Example: `7d97e98f…` instead of `7d97e98f`. Full values are always available in `--json` output.

### Changed

- **Sort summary format** — Now includes elapsed time on a second line after the "Done." line.

---

## [Unreleased] -- Output Formatting, Copyright Syntax, Config File Dest, Flag Consistency

### Changed

- **Unified destination flag naming** — All commands (`verify`, `resume`, `clean`, `query`, `stats`) now use `--dest` / `-d` for destination directories, matching the `sort` command. Improves consistency across the CLI.

- **Removed `MarkFlagRequired` for destination** — The `--dest` flag is no longer marked as required by Cobra, allowing `dest:` in `.pixe.yaml` or `PIXE_DEST` environment variable to satisfy the requirement without a CLI flag. Manual validation still ensures the destination is provided.

- **Copyright token syntax** — Changed copyright template syntax from `{{.Year}}` to `{year}` to match path template tokens. Supports `{year}`, `{month}`, `{monthname}`, `{day}`. Copyright templates are now parsed and validated at startup, catching syntax errors early.

- **Destination path prefixes in output** — Sort and status output now prefix destination paths with `...<basename>/` for clarity (e.g., `...Photos/2026/01-Jan/photo.jpg` instead of bare `2026/01-Jan/photo.jpg`). This cosmetic change improves readability when working with multiple archives.

### Documentation

- **Updated command documentation** — `docs/commands.md` and `README.md` regenerated to reflect new flag names, copyright syntax, and output format.

---

## [Unreleased] -- Documentation Fixes

### Added

- **Changelog sync via docgen** — `docs/changelog.md` is now a generated file. `extractChangelog()` in `internal/docgen/extract.go` reads the root `CHANGELOG.md`, strips the title and preamble, and injects the full version history into `docs/changelog.md` via the existing marker-based injection system. Running `make docs` keeps both files in sync; `make docs-check` (CI gate) detects drift. Three new tests added to `internal/docgen/docgen_test.go`.

### Bug Fixes

- **Full documentation reconciliation pass** — Resolved all 19 discrepancies identified in the deep scan. Updated source code comments in `internal/domain/handler.go` to accurately reflect full-file hashing behavior. Fixed documentation across 8 files (`docs/how-it-works.md`, `docs/technical.md`, `docs/adding-formats.md`, `docs/commands.md`, `docs/index.md`, `README.md`, `AGENTS.md`, `.state/ARCHITECTURE.md`) to align with current implementation: hashing strategy (full-file, not payload-only), output naming convention (added ALGO_ID), tagging approach (XMP sidecars for all formats), pipeline stages (8 stages including sidecar carry), handler registration (single `buildRegistry()` in `cmd/helpers.go`), Go version requirement (1.25+), and project layout (added 6 missing packages). Added `pixe stats` command documentation. All changes verified with `make docs-check` and `make check`.

- **Fixed docgen blank-line injection for GitHub Pages kramdown** — `injectContent()` in `internal/docgen/inject.go` was not emitting blank lines between begin/end markers and injected content. GitHub Pages (kramdown) requires a blank line before Markdown tables for correct rendering. Updated `injectContent()` to append blank lines before and after trimmed content, and updated test assertions in `docgen_test.go` to match. All 27 docgen tests pass; documentation regenerated via `go run ./internal/docgen`.

- **Fixed GHA integration test flakiness** — `TestVerbosity_Verbose` was checking for `"ms)"` in timing output, but on fast systems (GHA Linux runners), file processing takes <1ms, producing `"(0s)"` instead of `"(Xms)"`. Changed assertion to check for parenthesized timing info without requiring a specific duration unit, making the test system-speed agnostic.

### Chore

- **Fixed repository references after rename** — Updated all import paths and documentation references from old package name to `github.com/cwlls/pixe`.

- **Added CI badges to README.md** — Added GitHub Actions workflow status badges.

---

## [v2.6.2] -- 2026-03-13

### Chore

- **Fixed repository references after rename** — Updated all import paths and documentation references across 96 files to reflect the new package name.

---

## [v2.6.1] -- 2026-03-13

### Added

- **Graceful signal handling for SIGINT/SIGTERM** — The sort and resume pipelines now respond gracefully to interrupt signals. Workers drain cleanly, finishing their current file before exiting. A second signal restores default behavior for hard exit. Implemented via `signal.NotifyContext` wired to pipeline context.

### Bug Fixes

- **Fixed pipeline terminal UpdateFileStatus error handling** — Intermediate database status updates in worker goroutines now properly check for errors and log them without interrupting the pipeline.

- **Fixed manifest SafeLedgerWriter thread safety** — Serialized `WriteEntry` calls to prevent concurrent writes to the ledger file.

- **Fixed copy sidecar sync to disk** — Added `Sync()` call before `Close()` in `CopySidecar` to ensure data is flushed to stable storage.

- **Fixed copy temp file cleanup on all error paths** — Orphaned temp files are now properly cleaned up when copy operations fail.

---

## [v2.6.0] -- 2026-03-13

### Added

- **Configurable destination path templates** — New `--path-template` flag enables custom destination directory structures using token-based syntax (e.g., `{YYYY}/{MM}/{DD}/{FILENAME}`). Supports 8 tokens: `YYYY`, `MM`, `DD`, `HOUR`, `MIN`, `SEC`, `FILENAME`, `EXT`. Full validation and error handling included.

- **Destination aliases** — Shorthand aliases for common path templates (e.g., `--path-template "yearly"` expands to `{YYYY}/{FILENAME}`). Reduces command-line verbosity for standard layouts.

### Changed

- **Updated pathbuilder.Build() signature** — Now accepts optional `*Template` parameter for custom path construction. Existing code using default date-based paths is unaffected.

### Documentation

- **Updated README and docs for v2.6.0** — Refreshed documentation to reflect new path template and alias features.

---

## [v2.5.0] -- 2026-03-13

### Changed

- **Refactored all handlers to full-file hashing and sidecar-only metadata** — Destination files are now byte-identical copies of their source. Metadata is expressed exclusively via XMP sidecars — no handler modifies any file. This eliminates the need for EXIF writing libraries and simplifies the codebase.
  - Removed EXIF embedding from JPEG, MP4, CR3, and TIFF-RAW handlers
  - All metadata writes now route through XMP sidecar generation
  - Reduces handler complexity and improves data integrity

### Added

- **RAF handler for Fujifilm RAW support** — New `internal/handler/raf/` package adds support for `.raf` files (Fujifilm RAW format). Implements EXIF date extraction and full-file hashing.

### Documentation

- **Updated ARCHITECTURE for full-file hashing and sidecar-only metadata** — Documented the shift to sidecar-only metadata strategy and its implications for handler design.

---

## [v2.4.1] -- 2026-03-12

### Chore

- **Updated ARCHITECTURE file** — Documentation updates to reflect current project state.

---

## [v2.4.0] -- 2026-03-12

### Added

- **`internal/handler/tiff/` — Standalone TIFF handler** — Support for `.tif` and `.tiff` files produced by scanners and professional workflows. Embeds `tiffraw.Base` for EXIF extraction and supports both TIFF little-endian and big-endian magic bytes.

- **`internal/handler/avif/` — AVIF handler** — Support for `.avif` files (AV1 Image File Format), the HEIC successor used by iPhone 16+ and modern Android devices. Implements custom ISOBMFF `meta`/`iinf`/`iloc` box parser for EXIF date extraction (the `go-heic-exif-extractor` library does not support AVIF brands).

### Changed

- **Unified handler registry in `cmd/helpers.go`** — `buildRegistry()` now registers 14 handlers (added AVIF after HEIC, TIFF last to avoid claiming RAW files). Handler registration is now centralized and consistent across all CLI commands.

- **Refactored `cmd/resume.go` and `cmd/status.go`** — Removed stale inline handler registration blocks (both were missing PNG, ORF, RW2 handlers). Both commands now call `buildRegistry()` for consistency.

---

## [v2.3.1] -- 2026-03-12

### Bug Fixes

- **Addressed code review findings from v2.3.0 audit** — Fixed issues identified during comprehensive code review of v2.3.0 features.

---

## [v2.3.0] -- 2026-03-12

### Added

- **Testing & Quality (Section 16)** — Comprehensive test infrastructure improvements:
  - **Fuzz testing for all handler packages** — Added `fuzz_test.go` files to JPEG, HEIC, AVIF, MP4, CR3, PNG, and TIFF-RAW handlers. Each fuzz test covers `Detect()`, `ExtractDate()`, and `HashableReader()` methods with seed corpus from valid files, truncated variants, and cross-format inputs. Fuzz tests are run via `make fuzz` (30s per target).
  - **Expanded fixture corpus with edge-case helpers** — Created `internal/handler/handlertest/helpers.go` with 5 shared edge-case fixture builders: `BuildEmptyFile`, `BuildMagicOnly`, `BuildTruncatedFile`, `BuildWithFilename`, `BuildSymlink`. These enable testing of zero-byte files, magic-bytes-only files, truncated structures, Unicode filenames, and symlinks.
  - **Extended `handlertest.RunSuite()` with edge-case subtests** — Added 8 new subtests (tests 11–18) to the handler test suite, covering empty files, magic-only files, truncated files, corrupt EXIF, mismatched extensions, and symlinks. All edge-case tests enforce crash-resistance (no panic) rather than correctness.
  - **Discovery-level edge-case tests** — Added 6 new tests to `internal/discovery/discovery_test.go` covering symlinks to files/directories, symlink loops, unreadable files/directories, and Unicode directory names.
  - **Centralized benchmark suite** — Created `internal/benchmark/` package with 5 benchmark files covering hash throughput, copy performance, database latency, directory walk speed, and path construction time.
  - **Property-based tests for pathbuilder** — Added `internal/pathbuilder/pathbuilder_prop_test.go` with 6 property-based tests using `testing/quick` (10,000 iterations each).
  - **Makefile targets** — Added `make fuzz` and `make bench` for running fuzz tests and benchmarks.

- **Configurable hash algorithms (I2)** — Added support for MD5, BLAKE3, and xxHash-64 in addition to existing SHA-1 and SHA-256. Algorithm selection via `--algorithm` flag. Destination filenames are tagged with algorithm ID (e.g., `_sha256`, `_blake3`) for transparency.

- **ORF (Olympus) and RW2 (Panasonic) RAW handlers** — New handlers for Olympus and Panasonic RAW formats, expanding RAW support to 11 formats total.

- **PNG file format handler** — Support for `.png` files with EXIF date extraction.

- **`pixe stats` command** — New archive dashboard command for viewing archive statistics and inventory.

- **Config auto-discovery in source directory** — Pixe now searches the source directory for `.pixerc` configuration files, enabling per-project settings.

- **Colorized terminal output with TTY auto-detection** — Output is automatically colorized when writing to a terminal, with `--no-color` flag to disable.

- **`--quiet` and `--verbose` verbosity levels** — New flags for controlling output verbosity. `--quiet` suppresses non-essential output; `--verbose` shows detailed processing information.

- **`--since` and `--before` date filter flags** — New flags on sort command to filter files by capture date range.

- **Config profiles** — `--profile` flag enables loading named configuration profiles from `.pixerc`.

### Changed

- **Migrated docs site from custom Jekyll theme to GitHub Pages Slate theme** — Simplified documentation site configuration and improved visual consistency.

### Removed

- **Removed TUI package and gui subcommand** — The interactive terminal UI (Bubble Tea-based) has been removed. Users can use the CLI commands with the new `--progress` flag for live progress bars instead.

---

## [v2.2.10] -- 2026-03-12

### Bug Fixes

- **Fixed format table metadata detection for TIFF-based handlers** — The docgen `extractFormats` function only checked each handler's own source file for `MetadataSupport()`. TIFF-based handlers (ARW, CR2, DNG, NEF, PEF) inherit this method from `tiffraw.Base` via struct embedding. Added import-path fallback detection in `parseHandlerPackage` to correctly identify inherited capabilities.

- **Fixed phantom `handlertest` row in format table** — The test infrastructure package was not excluded from the format table scan. Added `handlertest` to the exclusion list alongside `tiffraw`.

- **Fixed CI documentation check race condition** — Added `fetch-tags: true` to `actions/checkout@v4` in the CI test job. Shallow clones (depth=1) exclude git tags, causing `git describe --tags` to fail and `extractVersion()` to return `"dev"` instead of the real version, making `docs-check` always report stale docs.

### Testing

- Added `TestExtractFormats_tiffrawHandlersShowSidecar` — verifies all TIFF-based handlers show "XMP sidecar"
- Added `TestExtractFormats_excludesHandlertest` — verifies test infrastructure is excluded
- Regenerated `README.md` and `docs/how-it-works.md` format tables with all 9 handlers and correct metadata columns

---

## [2.2.9] -- 2026-03-12

### Testing & Validation

- **Upgraded test suite from B+ to A grade** — Expanded test coverage across all packages to achieve comprehensive validation. Full test suite passes with `-race` flag; `make check` (fmt-check + vet + unit tests) clean; zero lint warnings, zero vet warnings.

- **Regenerated documentation** — Updated README.md and docs/how-it-works.md to reflect current handler capabilities and metadata support matrix.

---

## [2.2.8] -- 2026-03-12

### Chore

- **CI/workflow improvements** — Updated GitHub Actions workflows and removed deprecated dispatch options.

- **State cleanup** — Archived completed remediation tasks.

---

## [2.2.7] -- 2026-03-12

### Code Quality & Maintainability

- **Refactored `scanFileWithSource` to eliminate duplication** — Removed ~70 lines of duplicated scan logic by reusing `scanFileRow` helper, reducing maintenance burden.

---

## [2.2.6] -- 2026-03-12

### Improvements

- **Added durability guarantee in copy operation** — `out.Sync()` now called before `out.Close()` in `copy.Execute()` to ensure data is flushed to stable storage before returning.

---

## [2.2.5] -- 2026-03-12

### Security & Stability Fixes

- **XML-escaped user input in XMP template generation** — Copyright and camera owner strings are now XML-escaped before XMP template rendering, preventing malformed XML from special characters (`<`, `>`, `&`, `"`).

### Performance Improvements

- **Streamed JPEG SOS payload extraction** — Replaced `os.ReadFile` (which loaded entire JPEG into memory) with `io.ReadSeeker`-based streaming. Marker headers are scanned sequentially; only the SOS-to-EOI section is hashed. Reduces memory footprint for large panoramic JPEGs (200+ MB) by orders of magnitude.

- **Streamed MP4 keyframe extraction** — Replaced buffering all keyframes into `bytes.Buffer` with `io.MultiReader` over `io.SectionReader` instances. Eliminates hundreds of megabytes of memory usage for 4K video with many keyframes.

---

## [2.2.4] -- 2026-03-12

### Improvements

- **Track and warn on ledger write failures** — Non-fatal ledger write errors are now logged as warnings and do not interrupt the sort pipeline. Coordinator continues processing while monitoring ledger health.

- **Fixed sidecar display year cosmetic bug** — Capture date now flows through `workerFinalResult` struct to coordinator, enabling correct `{{.Year}}` resolution in sidecar lines (was resolving to year 1 due to `time.Time{}`).

---

## [2.2.3] -- 2026-03-12

### Security & Stability Fixes

- **Added symlink detection to discovery walk** — Symlinks are now explicitly detected and skipped with logged reason, preventing accidental processing of files outside the source directory.

### Code Quality & Maintainability

- **Documented intermediate DB status updates as best-effort** — Added explanatory comments at `db.UpdateFileStatus` call sites in worker goroutines, clarifying that intermediate state tracking is observational and non-fatal. Coordinator owns terminal states.

---

## [2.2.2] -- 2026-03-12

### Security & Stability Fixes

- **Eliminated template injection risk in copyright rendering** — Replaced `text/template` with simple `strings.ReplaceAll` for `{{.Year}}` substitution. Removed unnecessary template parsing complexity and eliminated duplicate `renderCopyright` implementations.

---

## [2.2.1] -- 2026-03-12

### Security & Stability Fixes

- **Fixed Windows path separator bug in file extension detection** — A custom `fileExt` function was copy-pasted across 10 files and only checked for `/` as a separator, breaking on Windows paths with dots in directory names (e.g., `C:\photos\no-ext-dir.backup\IMG_0001`). Extracted shared `fileutil.Ext()` using `filepath.Ext` for cross-platform correctness.

### Code Quality & Maintainability

- **Added compile-time interface checks** — JPEG, HEIC, and MP4 handlers now include `var _ domain.FileTypeHandler = (*Handler)(nil)` to catch interface drift at compile time.

---

## [2.2.0] -- 2026-03-12

### Added

- **`docgen` tool to eliminate documentation drift** — Automated documentation generator that extracts handler metadata, format support matrix, and version information directly from source code. Regenerates README.md and docs/how-it-works.md on every build to ensure docs stay in sync with implementation.

---

## [2.1.0] -- 2026-03-12

### Documentation

- **Comprehensive documentation across codebase** — Added detailed package comments, architectural overview, and design rationale to all major packages. Established documentation standards for future development.

---

## [2.0.4] -- 2026-03-12

### Test Coverage

- **Added uppercase extension test coverage** — Identified and closed a coverage gap in the discovery and integration test suites for uppercase-extension source files (e.g., `photo.JPG`, `IMG_0001.JPEG`). The feature was already working correctly via `strings.ToLower` in the fast-path handler lookup, but lacked explicit test validation.
  - **Unit tests** (`internal/discovery/discovery_test.go`):
    - `TestRegistry_uppercaseExtension_detected` — Table-driven test covering JPG, JPEG, DNG, NEF, MP4, MOV extensions; proves case-insensitive fast-path lookup
    - `TestWalk_uppercaseExtensionDiscovered` — Verifies `photo.JPG` with valid JPEG magic lands in `discovered`, not `skipped`
    - `TestWalk_mixedCaseExtensions` — Confirms `lower.jpg`, `upper.JPG`, `mixed.Jpg` all discovered correctly
  - **Integration tests** (`internal/integration/integration_test.go`):
    - `TestIntegration_UppercaseExtension` — Real `IMG_0001.JPG` fixture: processed=1, correct date path, destination ext=`.jpg` (normalized)
    - `TestIntegration_UppercaseExtension_MixedBatch` — Three files (a.jpg, b.JPG, c.JPEG): all processed, all destinations have lowercase extensions
  - **Result:** `make lint` → 0 issues | `make test-all` → all packages pass

---

## [2.0.3] -- 2026-03-11

### Bug Fixes

- **Fixed `pixe gui --dest /path` silently ignored (Viper pflag collision)** — Both `sortCmd` and `guiCmd` called `viper.BindPFlag("dest", ...)` on the global Viper instance. Since Viper stores only one pflag per key, the last `init()` to run won, causing `--dest` (and other flags) passed to `pixe gui` to be silently ignored. Now, `cmd/gui.go` uses `resolveGUIConfig(*cobra.Command)` which reads flag values directly from the cobra flag set, bypassing Viper entirely. All `viper.BindPFlag` calls removed from `gui.go`'s `init()`.

- **Implemented missing in-TUI settings editor** — The `[e]` key was advertised in the UI but never handled. No settings editor existed. Added `sortStateEdit` state to `SortModel` with two `textinput` fields (Source, Destination). `[e]` enters edit mode; `Tab`/`Shift+Tab` cycles focus; `Enter` saves; `Esc` cancels. `viewConfigure()` now always shows the `[e] Edit Settings` hint.

---

## [2.0.2] -- 2026-03-11

### Improvements

- **Lint violations fixed** — Resolved QF1012 (staticcheck) violations in `internal/tui` package by replacing `sb.WriteString(fmt.Sprintf(...))` calls with `fmt.Fprintf(&sb, ...)` for improved efficiency (17 occurrences across `sort.go`, `status.go`, `verify.go`). Removed unused `counterStyle` and `errorCounterStyle` variables from `internal/tui/styles.go`. Result: `make lint` → 0 issues.

---

## [2.0.1] -- 2026-03-11

### Bug Fixes

- **Fixed nil pointer dereference panic in dry-run duplicate detection** — When `--dry-run` was enabled and a duplicate file was detected, `processFile` returned `(nil, true, nil)`. The caller in `runSequential` dereferenced `le.Matches` and `le.Destination` without a nil guard, causing a SIGSEGV panic in `TestIntegration_SQLite_DryRun`. Now, when a duplicate is detected in dry-run mode, a proper `*domain.LedgerEntry` is constructed with `Status: LedgerStatusDuplicate`, `Destination`, and `Matches` populated, preventing the panic.

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
  - Renamed the module to `github.com/cwlls/pixe` for better clarity and consistency.

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

<!-- pixe:end:changelog -->
