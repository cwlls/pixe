# Completed Features Archive

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
