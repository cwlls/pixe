# Completed Features Archive

## Documentation Hygiene: Godoc Completeness (19 Tasks)

**Completion Date:** March 12, 2026

**Status:** ✅ COMPLETE

### Summary

Completed comprehensive documentation audit and remediation across the entire codebase. All 19 documentation tasks were successfully implemented, bringing the project into full compliance with AGENTS.md documentation standards. Every exported symbol, unexported struct type, and interface method implementation now carries appropriate godoc comments.

### Implementation Overview

All 19 tasks were successfully completed:

#### Package-Level Documentation (Tasks 1)
- **`main.go`** — Added package doc comment: `// Pixe sorts photographs and raw files into a date-based archive structure.`
- **`internal/domain/handler.go`** — Added comprehensive package doc comment for the `domain` package

#### Command Variables & Handlers (Tasks 2-4)
- **15 Cobra command variables** documented across `cmd/` package:
  - `sortCmd`, `verifyCmd`, `statusCmd`, `cleanCmd`, `resumeCmd`, `versionCmd`, `guiCmd`
  - `queryCmd`, `queryRunsCmd`, `queryRunCmd`, `queryFilesCmd`, `queryErrorsCmd`, `queryDuplicatesCmd`, `querySkippedCmd`, `queryInventoryCmd`
  
- **13 RunE handler functions** documented:
  - `runSort`, `runVerify`, `runStatus`, `runClean`, `runResume`, `runGUI`
  - `runQueryRuns`, `runQueryRun`, `runQueryFiles`, `runQueryErrors`, `runQueryDuplicates`, `runQuerySkipped`, `runQueryInventory`
  
- **6 additional functions** documented:
  - `printFilesTable`, `printFilesJSON`, `printRunTable`, `printRunJSON`, `runQueryDuplicateList`, `runQueryDuplicatePairs`
  
- **6 flag variables** documented:
  - `filesFrom`, `filesTo`, `filesImportedFrom`, `filesImportedTo`, `filesSource`, `duplicatePairs`

#### Constant Documentation (Task 5)
- **`LedgerStatus*` constants** — Replaced group comment with individual per-constant doc comments:
  - `LedgerStatusCopy`, `LedgerStatusSkip`, `LedgerStatusDuplicate`, `LedgerStatusError`

#### Struct Field Documentation (Tasks 6-10)
- **`domain/pipeline.go`** — 17 exported struct fields across 2 structs:
  - `ManifestEntry` (9 fields): Source, Destination, Checksum, Status, ExtractedAt, CopiedAt, VerifiedAt, TaggedAt, Error
  - `Manifest` (8 fields): Version, PixeVersion, Source, Destination, Algorithm, StartedAt, Workers, Files

- **`pipeline/pipeline.go`** — 7 exported struct fields across 2 structs:
  - `SortOptions` (4 fields): Config, Hasher, Registry, RunTimestamp
  - `SortResult` (4 fields): Processed, Duplicates, Skipped, Errors

- **`verify/verify.go`** — 10 exported struct fields across 3 structs:
  - `Result` (3 fields): Verified, Mismatches, Unrecognised
  - `FileResult` (3 fields): Path, Expected, Actual
  - `Options` (4 fields): Dir, Hasher, Registry, Output

- **`manifest/manifest.go`** — 2 exported struct fields:
  - `LedgerContents` (2 fields): Header, Entries

- **`domain/handler.go`** — 2 exported struct fields:
  - `MagicSignature` (2 fields): Offset, Bytes

#### Handler Package Documentation (Tasks 11-14)
- **`exifDateFormat` constants** — Standardized doc comments across 3 handlers:
  - `internal/handler/heic/heic.go`
  - `internal/handler/tiffraw/tiffraw.go`
  - `internal/handler/cr3/cr3.go`

- **Interface method implementations** — Added doc comments to 4 methods:
  - `multiSectionReader.Read()` and `Close()` in `tiffraw.go`
  - `sectionReadCloser.Read()` and `Close()` in `cr3.go`

- **Unexported struct types** — Added doc comments to 2 structs:
  - `sampleLocation` in `mp4.go`
  - `stscEntry` in `cr3.go`

- **Unexported constructors** — Added doc comment to `newMultiSectionReader()` in `tiffraw.go`

#### Code Refactoring (Task 15)
- **`mp4.buildMinimalMP4()`** — Moved from production code (`mp4.go`) to test file (`mp4_test.go`)
  - Function is test-only helper; now properly located
  - Added necessary imports (`bytes`, `encoding/binary`) to test file
  - Removed from production code

#### Cross-Package Documentation (Tasks 16-18)
- **`dblocator/filesystem_windows.go`** — Added doc comment to `isNetworkMount()` function
- **`discovery/walk.go`** — Added inline doc comment to `DiscoveredFile.Handler` field
- **`migrate/migrate.go`** — Added per-constant doc comments to 3 unexported constants:
  - `pixeDir`, `manifestFile`, `migratedSuffix`

#### Test Fixes (Task 19)
- **`cmd/status_test.go`** — Fixed mismatched comment: `// fixturesDir` → `// statusFixturesDir`

### Files Modified

**Core packages:**
- `main.go` — Package doc comment
- `internal/domain/handler.go` — Package doc comment, field comments
- `internal/domain/pipeline.go` — Constant comments, field comments
- `cmd/sort.go`, `cmd/verify.go`, `cmd/status.go`, `cmd/clean.go`, `cmd/resume.go`, `cmd/version.go`, `cmd/gui.go` — Command variable and function doc comments
- `cmd/query.go`, `cmd/query_runs.go`, `cmd/query_run.go`, `cmd/query_files.go`, `cmd/query_errors.go`, `cmd/query_duplicates.go`, `cmd/query_skipped.go`, `cmd/query_inventory.go` — Command and function doc comments

**Internal packages:**
- `internal/pipeline/pipeline.go` — Struct field comments
- `internal/verify/verify.go` — Struct field comments
- `internal/manifest/manifest.go` — Struct field comments
- `internal/handler/heic/heic.go`, `internal/handler/tiffraw/tiffraw.go`, `internal/handler/cr3/cr3.go` — Constant and method comments
- `internal/handler/mp4/mp4.go`, `internal/handler/mp4/mp4_test.go` — Function relocation
- `internal/dblocator/filesystem_windows.go` — Function doc comment
- `internal/discovery/walk.go` — Field comment
- `internal/migrate/migrate.go` — Constant comments
- `cmd/status_test.go` — Comment fix

### Quality Assurance

✅ `make check` → All formatting, vet, and unit tests pass
✅ `make lint` → 0 issues (golangci-lint)
✅ All 28 test packages pass
✅ No regressions in existing functionality
✅ Full AGENTS.md compliance achieved

### Verification

✅ All 19 tasks marked complete in PLAN.md
✅ All 30 files modified with appropriate documentation
✅ Full test suite passes: `make check && make lint`
✅ Zero documentation violations remaining
✅ Ready for production use

---

## Uppercase Extension Test Coverage (5 Tests)

**Completion Date:** March 12, 2026

**Status:** ✅ COMPLETE

### Summary

Added comprehensive test coverage for uppercase-extension source files (e.g., `photo.JPG`, `IMG_0001.JPEG`). The feature was already working correctly via `strings.ToLower` in the fast-path handler lookup, but lacked explicit test validation. This work closes a coverage gap and provides regression protection.

### Implementation Overview

All 5 tests were successfully added:

#### Unit Tests — Discovery Package (3 tests)

- **`TestRegistry_uppercaseExtension_detected`** — Table-driven test covering JPG, JPEG, DNG, NEF, MP4, MOV extensions
  - Proves `Registry.Detect()` fast-path lookup is case-insensitive via `strings.ToLower`
  - Validates that uppercase extensions resolve to the correct handler

- **`TestWalk_uppercaseExtensionDiscovered`** — File classification test
  - Verifies `photo.JPG` with valid JPEG magic lands in `discovered`, not `skipped`
  - Confirms magic-based validation works for uppercase extensions

- **`TestWalk_mixedCaseExtensions`** — Mixed-case batch test
  - Confirms `lower.jpg`, `upper.JPG`, `mixed.Jpg` all discovered correctly
  - Validates case-insensitive matching across multiple files

#### Integration Tests — Full Pipeline (2 tests)

- **`TestIntegration_UppercaseExtension`** — Real fixture test
  - Uses `IMG_0001.JPG` fixture with valid JPEG data
  - Verifies: processed=1, correct date path, destination ext=`.jpg` (normalized to lowercase)
  - Confirms end-to-end pipeline handles uppercase extensions

- **`TestIntegration_UppercaseExtension_MixedBatch`** — Multi-file batch test
  - Three files: `a.jpg`, `b.JPG`, `c.JPEG`
  - Verifies: all processed, all destinations have lowercase extensions
  - Validates batch processing consistency

### Files Modified

- `internal/discovery/discovery_test.go` — Added 3 unit tests
- `internal/integration/integration_test.go` — Added 2 integration tests

### Quality Assurance

✅ `make lint` → 0 issues
✅ `make test-all` → all packages pass
✅ No regressions in existing functionality
✅ Coverage gap closed

### Verification

✅ All 5 tests added and passing
✅ No lint issues introduced
✅ Full test suite passes
✅ Ready for production use

---

## Progress Tracking & TUI Feature (17 Tasks)

**Completion Date:** March 11, 2026

**Status:** ✅ COMPLETE

### Summary

The **Progress Tracking & TUI Feature** adds real-time event-driven progress monitoring and a full-featured terminal user interface (TUI) to Pixe. Users can now track sort and verify operations with live progress bars, activity logs, worker status panes, and an interactive GUI for configuration and status inspection.

### Implementation Overview

All 17 tasks were successfully completed:

#### Dependencies & Infrastructure (Tasks 1-2)
- Added Charm Bracelet stack: `bubbletea`, `bubbles`, `lipgloss`, `go-isatty`
- Created `internal/progress/` package with pure stdlib event bus:
  - `event.go` — 19 `EventKind` constants, `Event` struct, `RunSummary` struct
  - `bus.go` — Non-blocking `Bus` with buffered channels, `Emit()`, `Close()`, `Events()`
  - `plainwriter.go` — Reference implementation proving event bus carries full CLI output data

#### Pipeline Instrumentation (Tasks 3-4)
- **Sort pipeline** (`internal/pipeline/pipeline.go`, `worker.go`):
  - Added `EventBus *progress.Bus` to `SortOptions`
  - Emit `EventDiscoverDone`, `EventFileStart`, `EventFileExtracted`, `EventFileHashed`, `EventFileCopied`, `EventFileVerified`, `EventFileTagged`, `EventSidecarCarried`, `EventSidecarFailed`, `EventFileComplete`, `EventFileSkipped`, `EventFileDuplicate`, `EventFileError`, `EventRunComplete`
  - Accurate `Completed` counter in both sequential and concurrent modes
  
- **Verify pipeline** (`internal/verify/verify.go`):
  - Added `EventBus *progress.Bus` to `verify.Options`
  - Emit `EventVerifyStart`, `EventVerifyOK`, `EventVerifyMismatch`, `EventVerifyUnrecognised`, `EventVerifyDone`
  - Pre-scan for total file count (deterministic progress bar)

#### CLI Helpers & Flags (Tasks 5, 7)
- **`cmd/helpers.go`** — Extracted shared setup:
  - `resolveConfig()` — Viper → `*config.AppConfig`
  - `buildRegistry()` — Populate `discovery.Registry` with all handlers
  - `openArchiveDB()` — Resolve DB path, open, handle migration
  
- **`--progress` flag** in `cmd/sort.go` and `cmd/verify.go`:
  - TTY detection via `isatty.IsTerminal()`
  - Background goroutine for pipeline, foreground for Bubble Tea program
  - Event bus wiring, `io.Discard` for per-file output suppression

#### CLI Progress Bar (Task 6)
- **`internal/cli/styles.go`** — Adaptive Lip Gloss styles (no hardcoded colors)
- **`internal/cli/progress.go`** — `ProgressModel` Bubble Tea component:
  - Counters: total, completed, copied, duplicates, skipped, errors
  - ETA calculation: `elapsed / (completed/total) - elapsed`
  - Mode-aware labels ("sort" vs "verify")
  - Event-driven updates, window resize handling, quit keys

#### TUI Infrastructure (Tasks 8-10)
- **`internal/tui/styles.go`** — 15+ adaptive Lip Gloss styles for tabs, headers, footers, borders, counters, logs, workers, overlays
- **`internal/tui/keymap.go`** — `KeyMap` struct with global, sort, log, overlay, status bindings; `ShortHelp()` and `FullHelp()` methods
- **`internal/tui/components.go`** — Reusable sub-components:
  - `styledProgress` — Wraps Bubbles progress with TUI styling
  - `activityLog` — Scrollable viewport with filtering and follow mode
  - `workerPane` — Per-worker status display with collapse-to-summary
  - `errorOverlay` — Modal error detail box
  
- **`internal/tui/app.go`** — Root `App` model:
  - Three tabs: Sort, Verify, Status
  - Tab switching (Tab, ShiftTab, 1/2/3 keys)
  - Global key handling (q, ctrl+c, ?)
  - Window size propagation
  - Context-sensitive footer help

#### TUI Tabs (Tasks 11-13)
- **`internal/tui/sort.go`** — `SortModel`:
  - States: configure → running → complete
  - Configure: config summary, [s] start, [e] edit, [n] new
  - Running: event bus bridge, progress bar, activity log, worker pane, error overlay
  - Complete: summary with breakdown bars, [e] filter errors, [n] new run
  - Event-to-log formatting with adaptive colors
  
- **`internal/tui/verify.go`** — `VerifyModel`:
  - States: configure → running → complete
  - Configure: directory input, [v] start verify
  - Running: `EventVerifyOK`, `EventVerifyMismatch`, `EventVerifyUnrecognised` handling
  - Complete: summary with mismatch count, [e] filter mismatches
  
- **`internal/tui/status.go`** — `StatusModel`:
  - States: loading → ready
  - Background goroutine: `discovery.Walk()` + `manifest.LoadLedger()` classification
  - Five categories: sorted, duplicates, errored, unsorted, unrecognized
  - Category switching (1–5 keys), refresh (r key), scrollable file lists
  - Spinner during load, summary counters in ready state

#### GUI Command (Task 14)
- **`cmd/gui.go`** — `pixe gui` Cobra command:
  - Registers all sort flags (source, dest, workers, algorithm, copyright, camera-owner, dry-run, db-path, recursive, skip-duplicates, ignore, no-carry-sidecars, overwrite-sidecar-tags)
  - `runGUI()` wiring: `resolveConfig()`, `buildRegistry()`, `Hasher`, `AppOptions`, `tea.NewProgram()`
  - Alt-screen mode for clean TUI rendering
  - Lazy DB opening (Sort tab opens on run start, Status tab doesn't need DB)

#### Testing (Tasks 15-17)
- **`internal/progress/progress_test.go`** — 8 tests:
  - `TestBus_EmitAndReceive` — Field preservation
  - `TestBus_NonBlockingEmit` — Buffer overflow handling
  - `TestBus_Close` — Channel closure
  - `TestBus_CloseIdempotent` — Safe double-close
  - `TestBus_EmitAfterClose` — Silent drop
  - `TestBus_TimestampSet` — Automatic timestamp injection
  - `TestPlainWriter_SortOutput` — Plain-text format fidelity
  - `TestPlainWriter_VerifyOutput` — Verify event formatting
  
- **`internal/cli/progress_test.go`** — 9 tests:
  - `TestProgressModel_Init` — Command initialization
  - `TestProgressModel_CounterUpdates` — Event→counter mapping
  - `TestProgressModel_DiscoverDone` — Total count setting
  - `TestProgressModel_CurrentFile` — Current file tracking
  - `TestProgressModel_DoneOnRunComplete` — Quit transition
  - `TestProgressModel_DoneOnBusClosed` — Bus closure handling
  - `TestProgressModel_WindowResize` — Width/height updates
  - `TestProgressModel_ViewContainsCounters` — Output validation
  - `TestProgressModel_VerifyMode` — Mode-specific labels
  
- **`internal/tui/tui_test.go`** — 10 tests:
  - `TestApp_TabSwitching` — Tab cycling and direct jump
  - `TestApp_WindowSize` — Size propagation
  - `TestApp_QuitKeys` — Quit handling
  - `TestSortModel_StateTransitions` — State machine
  - `TestSortModel_EventCounters` — Counter accuracy
  - `TestSortModel_ActivityLogAppend` — Log formatting
  - `TestSortModel_FilterCycle` — Filter rotation
  - `TestVerifyModel_StateTransitions` — Verify state machine
  - `TestStatusModel_CategorySwitching` — Category navigation
  - `TestStatusModel_Refresh` — Refresh cycle

### Files Created

**New packages:**
- `internal/progress/` — Event bus (event.go, bus.go, plainwriter.go)
- `internal/cli/` — Progress bar (styles.go, progress.go, progress_test.go)
- `internal/tui/` — Full TUI (styles.go, keymap.go, components.go, app.go, sort.go, verify.go, status.go, tui_test.go)

**New commands:**
- `cmd/gui.go` — `pixe gui` command

**New helpers:**
- `cmd/helpers.go` — `resolveConfig()`, `buildRegistry()`, `openArchiveDB()`

### Architecture Documentation

Section 11 of `.state/ARCHITECTURE.md` documents the complete feature:

- **11.1** — Event-driven architecture overview
- **11.2** — Event bus design (non-blocking, buffered channels)
- **11.3** — Event kinds (19 constants)
- **11.4** — Pipeline instrumentation (sort and verify)
- **11.5** — CLI progress bar (Bubble Tea model, ETA calculation)
- **11.6** — TUI architecture (tab-based, state machines)
- **11.7** — TUI components (progress, log, workers, overlay)
- **11.8** — GUI command integration

### Key Design Decisions

1. **Non-blocking event emission** — Events are dropped on buffer overflow; progress is best-effort, never blocks pipeline.

2. **Event bus is pure stdlib** — Zero external dependencies in `internal/progress/`; all Charm deps isolated to `internal/cli/` and `internal/tui/`.

3. **Lazy database opening** — GUI doesn't open archive DB upfront; Sort tab opens it when run starts (dest path may be edited interactively).

4. **Adaptive colors only** — All Lip Gloss styles use `lipgloss.AdaptiveColor` for light/dark terminal compatibility.

5. **Background pipeline, foreground TUI** — Pipeline runs in goroutine, Bubble Tea program runs on main goroutine for clean event loop.

6. **Event-to-log formatting** — Each terminal event is formatted into a styled activity log line; PlainWriter proves event bus carries full output data.

7. **State machines for tabs** — Each tab (Sort, Verify, Status) is a Bubble Tea model with explicit state transitions (configure→running→complete, loading→ready).

### Testing Coverage

- **8 event bus tests** covering emit, close, timestamp, plain-text output
- **9 CLI progress tests** covering counters, ETA, window resize, mode-specific labels
- **10 TUI tests** covering tab switching, state transitions, event handling, category navigation
- **All existing tests pass** — no regressions

### Verification

✅ All 17 tasks marked complete in PLAN.md
✅ Architecture documentation complete and accurate
✅ All required packages and commands created
✅ Full test suite passes: `make check && make test-all`
✅ No lint issues
✅ Ready for production use

---

## Source Sidecar Carry Feature (22 Tasks)

**Completion Date:** March 11, 2026

**Status:** ✅ COMPLETE

### Summary

The **Source Sidecar Carry** feature enables Pixe to detect and carry pre-existing sidecar files (`.aae`, `.xmp`) from the source directory alongside their parent media files during the sort operation. Carried sidecars are renamed to match their parent's destination filename and optionally have metadata tags merged into `.xmp` files.

### Implementation Overview

All 22 tasks were successfully completed:

#### Configuration & CLI (Tasks 1-2)
- Added `CarrySidecars` and `OverwriteSidecarTags` fields to `AppConfig`
- Added `--no-carry-sidecars` and `--overwrite-sidecar-tags` CLI flags with proper Viper bindings

#### Discovery & Association (Tasks 3-5)
- Created `SidecarFile` struct with path, relative path, and extension fields
- Extended `DiscoveredFile` with `Sidecars []SidecarFile` field
- Implemented two-pass sidecar association algorithm in `Walk()`:
  - First pass: classify files by handler
  - Second pass: match sidecars to parents by stem (case-insensitive) or full-extension convention
  - Unmatched sidecars marked as orphans

#### Database Schema (Tasks 6-7)
- Added schema migration v2→v3 with `carried_sidecars TEXT` column
- Implemented `CarriedSidecars *string` field on `FileRecord`
- Added `WithCarriedSidecars()` functional option for DB updates
- Updated all SELECT queries to include the new column

#### Ledger & Domain (Task 8)
- Added `Sidecars []string` field to `LedgerEntry` with `omitempty` tag

#### XMP Merge (Tasks 9-10)
- Implemented `xmp.MergeTags()` function for non-destructive tag injection into carried `.xmp` files
- Supports overwrite control via `--overwrite-sidecar-tags` flag
- Preserves existing XMP data while adding/merging Pixe metadata
- Added namespace declarations as needed

#### Tagging Integration (Task 10)
- Created `ApplyWithSidecars()` wrapper function in `tagging` package
- Routes to `MergeTags()` when carried `.xmp` is present
- Falls back to `WriteSidecar()` for generated sidecars

#### Copy Helper (Task 11)
- Implemented `CopySidecar()` in `internal/copy/` for simple file copying
- No temp-file atomicity or hash verification (sidecars are small metadata files)

#### Pipeline Integration (Tasks 12-14)
- Integrated sidecar carry into sequential `processFile()` in `pipeline.go`
- Integrated sidecar carry into concurrent worker path in `worker.go`
- Wired sidecar paths to DB updates and ledger entries
- Sidecar copy failures are non-fatal (logged as warnings)

#### Output & Dry-Run (Tasks 15-16)
- Added `+sidecar` stdout output lines after COPY lines
- Implemented dry-run support showing what would be carried without copying
- Added `(merge tags)` annotation for `.xmp` sidecars with configured tags

#### Duplicate Handling (Task 17)
- Sidecars follow parent to `duplicates/` directory in default mode
- Sidecars skipped when parent is skipped in `--skip-duplicates` mode

#### Testing (Tasks 18-21)
- **Unit tests for discovery** (`internal/discovery/sidecar_test.go`):
  - Stem matching, full-extension matching, case-insensitive matching
  - Multiple sidecars per parent, orphan detection
  - Ambiguous stem resolution, carry disabled mode
  - Subdirectory isolation

- **Unit tests for XMP merge** (`internal/xmp/merge_test.go`):
  - Injection into empty description, preservation of existing fields
  - Overwrite mode, namespace addition
  - Empty tags handling, atomic writes
  - Malformed XMP error handling

- **Unit tests for tagging** (`internal/tagging/tagging_test.go`):
  - Merge path vs generate path, overwrite flag propagation
  - Empty tags with carried XMP

- **Integration tests** (`internal/integration/sidecar_carry_test.go`):
  - Basic carry with `.aae` and `.xmp` sidecars
  - XMP merge with tag injection
  - Carry disabled mode, duplicate routing
  - Skip-duplicates mode, dry-run mode
  - Overwrite sidecar tags, DB and ledger verification

#### Quality Assurance (Task 22)
- Full test suite passes: `make check && make test-all`
- No lint issues introduced
- All existing tests continue to pass

### Files Created

**New files:**
- `internal/discovery/sidecar.go` — SidecarFile struct, sidecar extensions, associateSidecars()
- `internal/discovery/sidecar_test.go` — 14 unit tests
- `internal/xmp/merge.go` — MergeTags() implementation
- `internal/xmp/merge_test.go` — 14 unit tests
- `internal/copy/sidecar.go` — CopySidecar() helper
- `internal/integration/sidecar_carry_test.go` — 10 end-to-end tests

**Modified files:**
- `internal/config/config.go` — Added CarrySidecars, OverwriteSidecarTags
- `cmd/sort.go` — Added CLI flags and Viper bindings
- `internal/discovery/walk.go` — Added Sidecars field, CarrySidecars option, wired associateSidecars()
- `internal/archivedb/schema.go` — Schema v2→v3 migration
- `internal/archivedb/files.go` — CarriedSidecars field, WithCarriedSidecars() option
- `internal/archivedb/queries.go` — Updated SELECT queries
- `internal/archivedb/archivedb_test.go` — Updated migration tests
- `internal/domain/pipeline.go` — Added Sidecars to LedgerEntry
- `internal/tagging/tagging.go` — Added ApplyWithSidecars()
- `internal/tagging/tagging_test.go` — 6 new tests
- `internal/pipeline/pipeline.go` — Sidecar carry in processFile(), emitSidecarLines(), dry-run
- `internal/pipeline/worker.go` — Sidecar carry in runWorker, coordinator wiring
- `internal/ignore/ignore.go` — Fixed pre-existing lint issue

### Architecture Documentation

Section 4.12 of `.state/ARCHITECTURE.md` documents the complete feature:

- **4.12.1** — Supported sidecar extensions (.aae, .xmp)
- **4.12.2** — Association rules (stem matching, case-insensitive, same directory)
- **4.12.3** — Discovery integration (two-pass algorithm)
- **4.12.4** — Destination naming (full-extension convention)
- **4.12.5** — Pipeline integration (between verify and tag stages)
- **4.12.6** — XMP merge behavior (preserve vs overwrite)
- **4.12.7** — Duplicate handling (follow parent routing)
- **4.12.8** — Database & ledger tracking (carried_sidecars column)
- **4.12.9** — Dry-run mode support
- **4.12.10** — pixe clean interaction

### Key Design Decisions

1. **Non-destructive sidecar copy** — No temp-file atomicity or hash verification; sidecars are small metadata files and the source is always available for re-copy.

2. **Sidecar copy failure is non-fatal** — Media file is already safely copied and verified; sidecar failures are logged as warnings and do not downgrade file status.

3. **XMP merge preserves source data** — By default, existing XMP values are authoritative; Pixe only fills in missing fields. Overwrite mode is opt-in via `--overwrite-sidecar-tags`.

4. **Sidecars are attachments** — No separate DB rows; tracked as JSON array in `carried_sidecars` column alongside parent file record.

5. **Full-extension naming convention** — Carried sidecars use `<parent_dest_filename>.<sidecar_ext>` (e.g., `20211225_062223_7d97e98f.heic.aae`) for unambiguous association and Adobe tool compatibility.

6. **Same-directory association only** — Sidecars only match parents in the same directory; no cross-directory matching to avoid ambiguity.

### Testing Coverage

- **14 discovery tests** covering stem/full-ext matching, case-insensitivity, orphans, ambiguity, disabled mode
- **14 XMP merge tests** covering injection, preservation, overwrite, namespace handling, atomicity
- **6 tagging tests** covering merge vs generate paths, overwrite propagation
- **10 integration tests** covering full pipeline scenarios with various flag combinations
- **All existing tests pass** — no regressions

### Verification

✅ All 22 tasks marked complete in PLAN.md
✅ Architecture documentation complete and accurate
✅ All required files created and implemented
✅ Full test suite passes
✅ No lint issues
✅ Ready for production use

---

## GUI Flag Handling & Settings Editor (2 Bug Fixes)

**Completion Date:** March 11, 2026

**Status:** ✅ COMPLETE

### Summary

Two critical bugs in the `pixe gui` command were identified and fixed:

1. **Viper pflag collision** — Both `sortCmd` and `guiCmd` called `viper.BindPFlag("dest", ...)` on the global Viper instance, causing the last `init()` to win. Flags passed to `pixe gui` (e.g., `--dest /path`) were silently ignored.

2. **Missing settings editor** — The `[e]` key was advertised in the UI but never implemented. Users had no in-TUI way to change source or destination directories.

### Implementation Overview

#### Bug 1: Viper pflag Collision Fix
- **Root cause:** Global Viper instance stores only one pflag per key. When `guiCmd.init()` ran after `sortCmd.init()`, it overwrote the pflag bindings, causing `--dest` and other flags to be ignored in GUI mode.
- **Solution:** Implemented `resolveGUIConfig(*cobra.Command)` in `cmd/gui.go` that reads flag values directly from the cobra flag set, bypassing Viper entirely.
- **Changes:**
  - Removed all `viper.BindPFlag()` calls from `gui.go`'s `init()` function
  - Added `resolveGUIConfig()` helper that extracts flag values from `cmd.Flags()` and populates `*config.AppConfig`
  - Updated `runGUI()` to call `resolveGUIConfig()` instead of `resolveConfig()`
  - Result: `pixe gui --dest /path` now correctly passes the destination to the TUI

#### Bug 2: Settings Editor Implementation
- **Root cause:** The `[e]` key hint was shown in `viewConfigure()` but no handler existed. The feature was never implemented.
- **Solution:** Added a full settings editor state to `SortModel` with interactive text input fields.
- **Changes:**
  - Added `sortStateEdit` state to `SortModel` state machine
  - Implemented two `textinput.Model` fields for Source and Destination paths
  - Key bindings:
    - `[e]` — Enter edit mode from configure state
    - `Tab` / `Shift+Tab` — Cycle focus between Source and Destination fields
    - `Enter` — Save changes and return to configure state
    - `Esc` — Cancel without saving
  - Updated `viewConfigure()` to always show `[e] Edit Settings` hint
  - Integrated with `SortModel.Update()` to handle edit state transitions
  - Added `viewEdit()` function to render the editor UI with styled text input fields

#### Testing
- **`cmd/gui_test.go`** (new) — Tests for `resolveGUIConfig()` flag resolution
- **`internal/tui/tui_test.go`** (updated) — Added tests for edit state transitions and text input handling
- **`internal/tui/sort.go`** (updated) — Full state machine integration

#### Dependencies
- Added transitive dependency: `github.com/atotto/clipboard` (required by `bubbles/textinput`)
- Updated `go.mod` and `go.sum`

### Files Modified

- `cmd/gui.go` — Removed Viper bindings, added `resolveGUIConfig()`, updated `runGUI()`
- `cmd/gui_test.go` — New file with flag resolution tests
- `internal/tui/sort.go` — Added `sortStateEdit`, text input fields, edit state handling, `viewEdit()`
- `internal/tui/tui_test.go` — Added edit state transition tests
- `go.mod` / `go.sum` — Added clipboard transitive dependency

### Quality Assurance

✅ `make lint` → 0 issues
✅ `make test-all` → all packages pass
✅ `pixe gui --dest /path` now correctly receives the flag
✅ `[e]` key now opens interactive settings editor
✅ No regressions in existing functionality

### Verification

✅ Both bugs fixed and verified
✅ Full test suite passes
✅ No lint issues
✅ Ready for production use
