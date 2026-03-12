# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Add Charm dependencies | high | @developer | [ ] pending | — | `go get` bubbletea, bubbles, lipgloss, go-isatty |
| 2  | Event bus package (`internal/progress/`) | high | @developer | [ ] pending | — | Pure stdlib; event types, Bus, PlainWriter |
| 3  | Instrument sort pipeline with event emission | high | @developer | [ ] pending | 2 | Add `EventBus` to `SortOptions`, emit calls in pipeline.go + worker.go |
| 4  | Instrument verify pipeline with event emission | high | @developer | [ ] pending | 2 | Add `EventBus` to `verify.Options`, emit calls in verify.go |
| 5  | Extract shared config/registry helpers in `cmd/` | medium | @developer | [ ] pending | — | Factor `resolveConfig()` and `buildRegistry()` out of `runSort` for reuse by gui.go |
| 6  | CLI progress bar package (`internal/cli/`) | high | @developer | [ ] pending | 1, 2 | ProgressModel, styles, ETA calculation |
| 7  | Wire `--progress` flag into `pixe sort` and `pixe verify` | high | @developer | [ ] pending | 3, 4, 6 | Flag binding, TTY detection, bus+model wiring in sort.go and verify.go |
| 8  | TUI shared styles and keymap (`internal/tui/styles.go`, `keymap.go`) | medium | @developer | [ ] pending | 1 | Adaptive Lip Gloss styles, KeyMap struct, help bindings |
| 9  | TUI shared components (`internal/tui/components.go`) | medium | @developer | [ ] pending | 1, 8 | Styled progress bar wrapper, log viewport, error overlay, worker status pane |
| 10 | TUI root App model (`internal/tui/app.go`) | high | @developer | [ ] pending | 8, 9 | Tab routing, header/footer, window size, global key handling |
| 11 | TUI Sort tab (`internal/tui/sort.go`) | high | @developer | [ ] pending | 2, 5, 9, 10 | Configure → running → complete states, event bus bridge, activity log, worker pane |
| 12 | TUI Verify tab (`internal/tui/verify.go`) | high | @developer | [ ] pending | 2, 5, 9, 10 | Configure → running → complete states, event bus bridge |
| 13 | TUI Status tab (`internal/tui/status.go`) | high | @developer | [ ] pending | 5, 9, 10 | Walk + ledger load in background goroutine, category switching, scrollable lists |
| 14 | `pixe gui` Cobra command (`cmd/gui.go`) | high | @developer | [ ] pending | 5, 10, 11, 12, 13 | Command definition, flag binding, `runGUI` wiring |
| 15 | Tests: event bus | high | @tester | [ ] pending | 2 | Non-blocking emit, close semantics, PlainWriter output fidelity |
| 16 | Tests: CLI progress model | high | @tester | [ ] pending | 6 | Model update tests — event→counter mapping, ETA, done transition |
| 17 | Tests: TUI models | medium | @tester | [ ] pending | 10, 11, 12, 13 | Tab switching, sort state transitions, verify state transitions, status classification |

---

## Task Descriptions

### Task 1 — Add Charm Dependencies

**Agent:** @developer
**Priority:** high
**Depends on:** —

Add the Charm Bracelet stack and promote `go-isatty` to a direct dependency:

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/mattn/go-isatty@latest
```

Run `go mod tidy` after. Verify the build still passes (`make check`). No code changes beyond `go.mod` / `go.sum`.

---

### Task 2 — Event Bus Package (`internal/progress/`)

**Agent:** @developer
**Priority:** high
**Depends on:** —

Create `internal/progress/` with three files. This package has **zero external dependencies** — pure stdlib only.

**`event.go`** — Define the `EventKind` enum (19 constants per Section 11.3), the `Event` struct, and the `RunSummary` struct. All fields exactly as specified in the architecture. Include the Apache 2.0 header and package doc comment.

**`bus.go`** — Implement the `Bus` struct:
- `NewBus(bufferSize int) *Bus` — creates a buffered channel of `Event` and a `closed` signal channel.
- `Emit(e Event)` — non-blocking send via `select` with `default` (drop on full buffer). Sets `e.Timestamp = time.Now()` before send. No-op after `Close()`.
- `Events() <-chan Event` — returns the receive-only channel.
- `Close()` — closes the event channel. Safe to call multiple times (guard with `sync.Once`).

**`plainwriter.go`** — Implement `PlainWriter`:
- `NewPlainWriter(w io.Writer) *PlainWriter`
- `Run(events <-chan Event)` — ranges over the channel. For each event, formats the traditional plain-text output line (`COPY`, `SKIP`, `DUPE`, `ERR `, sidecar lines, summary). This is a reference implementation that proves the event bus carries enough data to reproduce the existing CLI output exactly.

---

### Task 3 — Instrument Sort Pipeline with Event Emission

**Agent:** @developer
**Priority:** high
**Depends on:** 2

Modify the sort pipeline to emit events alongside existing `fmt.Fprintf` output. All changes are additive — no existing output is removed.

**`internal/pipeline/pipeline.go`:**
1. Add `EventBus *progress.Bus` field to `SortOptions`.
2. Add a package-level helper: `func emit(bus *progress.Bus, e progress.Event)` — nil-guarded, sets timestamp, calls `bus.Emit`.
3. In `Run()`:
   - After `discovery.Walk()` returns: emit `EventDiscoverDone` with `Total = len(discovered)`, `Skipped = len(skipped)`.
   - After the final summary `fmt.Fprintf`: emit `EventRunComplete` with a populated `RunSummary`.
4. In `runSequential()`:
   - Before each `processFile` call: emit `EventFileStart` with `RelPath`, `WorkerID: -1`.
   - After each terminal outcome (COPY/SKIP/DUPE/ERR): emit the corresponding terminal event (`EventFileComplete`, `EventFileSkipped`, `EventFileDuplicate`, `EventFileError`) with `Completed` incremented.
5. In `processFile()`:
   - After `ExtractDate`: emit `EventFileExtracted`.
   - After `Hasher.Sum`: emit `EventFileHashed` with `Checksum`.
   - After `copypkg.Execute`: emit `EventFileCopied`.
   - After `copypkg.Promote`: emit `EventFileVerified`.
   - After `tagging.ApplyWithSidecars`: emit `EventFileTagged`.
   - For each sidecar carried: emit `EventSidecarCarried` or `EventSidecarFailed`.

**`internal/pipeline/worker.go`:**
1. Thread `opts.EventBus` through to `runWorker` (pass as parameter or access via `opts`).
2. In `RunConcurrent()`:
   - Emit `EventFileSkipped` for discovery-phase skips and previously-imported skips.
   - Emit terminal events in the coordinator's `doneCh` handler.
3. In `runWorker()`:
   - Emit `EventFileStart` with `WorkerID: id` when pulling a work item.
   - Emit `EventFileExtracted`, `EventFileHashed` after each stage.
   - Emit `EventFileCopied`, `EventFileVerified`, `EventFileTagged` after each stage.
   - Emit `EventSidecarCarried`/`EventSidecarFailed` for sidecar operations.

**Key constraint:** The `Completed` counter on terminal events must be accurate. In sequential mode, the pipeline tracks this locally. In concurrent mode, the coordinator tracks it (it already has `completed` counter). Pass the current count on each terminal event.

---

### Task 4 — Instrument Verify Pipeline with Event Emission

**Agent:** @developer
**Priority:** high
**Depends on:** 2

Modify `internal/verify/verify.go`:

1. Add `EventBus *progress.Bus` field to `verify.Options`.
2. At the start of `Run()`: emit `EventVerifyStart`.
3. For each file in the walk:
   - On successful hash match: emit `EventVerifyOK` with `RelPath`, `Completed` counter.
   - On mismatch: emit `EventVerifyMismatch` with `RelPath`, `ExpectedChecksum`, `ActualChecksum`.
   - On unrecognised: emit `EventVerifyUnrecognised` with `RelPath`.
   - On error: emit `EventVerifyMismatch` with `Err`.
4. After the walk: emit `EventVerifyDone` with a populated `RunSummary` (Verified, Mismatches, Unrecognised).

The `Total` field requires a pre-count. Since `filepath.WalkDir` doesn't provide a total upfront, either:
- (a) Do a fast pre-scan counting non-directory entries before the main walk, or
- (b) Set `Total = -1` (unknown) and let consumers show an indeterminate progress bar until the walk completes.

Option (a) is preferred for UX — the progress bar needs a denominator. The pre-scan is a lightweight `WalkDir` that counts files without opening them.

---

### Task 5 — Extract Shared Config/Registry Helpers in `cmd/`

**Agent:** @developer
**Priority:** medium
**Depends on:** —

Factor common setup logic out of `runSort` in `cmd/sort.go` into shared helpers that `cmd/gui.go` can also call. Create `cmd/helpers.go`:

```go
// resolveConfig reads Viper values and returns a populated *config.AppConfig.
// Used by runSort and runGUI.
func resolveConfig() (*config.AppConfig, error)

// buildRegistry creates and populates a discovery.Registry with all
// supported file type handlers. Used by sort, verify, status, and gui.
func buildRegistry() *discovery.Registry

// openArchiveDB resolves the DB location, opens it, handles migration,
// and returns the DB handle + cleanup function. Used by sort, resume, and gui.
func openArchiveDB(cfg *config.AppConfig) (*archivedb.DB, func(), error)
```

Update `runSort` to call these helpers instead of inline setup. Verify all existing tests still pass.

---

### Task 6 — CLI Progress Bar Package (`internal/cli/`)

**Agent:** @developer
**Priority:** high
**Depends on:** 1, 2

Create `internal/cli/` with three files.

**`styles.go`** — Define Lip Gloss styles using **only adaptive colors** (`lipgloss.AdaptiveColor`). No hardcoded hex values. Styles needed:
- `headerStyle` — dim text for the command header line.
- `barStyle` — progress bar gradient (use Bubbles progress default, which adapts).
- `counterStyle` — for status counters (copied, duplicates, skipped, errors).
- `errorCountStyle` — slightly emphasized for the error counter.
- `currentFileStyle` — dim, truncated to terminal width.
- `etaStyle` — dim, right-aligned.

**`progress.go`** — Implement `ProgressModel`:
- Fields: `bus *progress.Bus`, `bar bubbles.progress.Model`, counters (total, completed, copied, duplicates, skipped, errors), `currentFile string`, `startedAt time.Time`, `width int`, `done bool`, `mode string` ("sort" or "verify").
- `NewProgressModel(bus, source, dest, mode)` — constructor.
- `Init() tea.Cmd` — returns `listenForEvents(bus)`.
- `Update(msg tea.Msg) (tea.Model, tea.Cmd)`:
  - `tea.WindowSizeMsg` → update width, resize progress bar.
  - `eventMsg` (wrapping `progress.Event`) → update counters based on `Kind`. For `EventFileStart`, update `currentFile`. For terminal events, increment `completed` and the appropriate counter. For `EventRunComplete`/`EventVerifyDone`, set `done = true`, return `tea.Quit`. Re-issue `listenForEvents`.
  - `busClosedMsg` → set `done = true`, return `tea.Quit`.
  - `tea.KeyMsg` "q" or "ctrl+c" → return `tea.Quit`.
  - `tickMsg` (1-second timer) → re-render for ETA update.
- `View() string`:
  - Line 1: header (command name, source → dest).
  - Line 2: blank.
  - Line 3: `Total   [progress bar]  X / Y  (Z%)  ETA Xm Ys`.
  - Line 4: `Current <stage> <filename>` (sort mode only).
  - Line 5: blank.
  - Line 6: status counters separated by `│`.
- ETA calculation: `elapsed / (completed/total) - elapsed`. Display as `Xm Ys` or `< 1s`.

**`progress_test.go`** — Test the model's `Update` method with synthetic events. Verify counter increments, done transition, and that `View()` output contains expected strings.

---

### Task 7 — Wire `--progress` Flag into `pixe sort` and `pixe verify`

**Agent:** @developer
**Priority:** high
**Depends on:** 3, 4, 6

**`cmd/sort.go`:**
1. Add `--progress` flag: `sortCmd.Flags().Bool("progress", false, "show a live progress bar instead of per-file text output")`.
2. Bind to Viper: `viper.BindPFlag("progress", sortCmd.Flags().Lookup("progress"))`.
3. In `runSort()`, after building `opts` but before `pipeline.Run()`:
   - If `viper.GetBool("progress")` is true AND `isatty.IsTerminal(os.Stdout.Fd())`:
     - Create `bus := progress.NewBus(256)`.
     - Set `opts.EventBus = bus`.
     - Set `opts.Output = io.Discard`.
     - Create `model := cli.NewProgressModel(bus, cfg.Source, cfg.Destination, "sort")`.
     - Create `p := tea.NewProgram(model)`.
     - Run `pipeline.Run(opts)` in a goroutine; on return, call `bus.Close()`.
     - Run `p.Run()` on the main goroutine.
     - After `p.Run()` returns, print the final summary line.
   - Otherwise: run `pipeline.Run(opts)` synchronously as today.

**`cmd/verify.go`:**
1. Add `--progress` flag (same pattern).
2. Same wiring: bus → `opts.EventBus`, `io.Discard` → `opts.Output`, background goroutine for `verify.Run`, foreground for `tea.NewProgram`.

---

### Task 8 — TUI Shared Styles and Keymap

**Agent:** @developer
**Priority:** medium
**Depends on:** 1

Create `internal/tui/styles.go` and `internal/tui/keymap.go`.

**`styles.go`** — Lip Gloss styles using adaptive colors only:
- `tabActiveStyle`, `tabInactiveStyle` — for tab bar buttons.
- `headerStyle` — app name + tab bar container.
- `footerStyle` — key binding hints.
- `borderStyle` — subtle horizontal dividers between panes.
- `progressBarStyle` — wrapper for the Bubbles progress component.
- `counterStyle`, `errorCounterStyle` — status counter labels.
- `logCopyStyle`, `logDupeStyle`, `logSkipStyle`, `logErrStyle` — per-verb colors in the activity log.
- `workerActiveStyle`, `workerIdleStyle` — worker status pane.
- `overlayStyle` — error detail overlay (bordered box).
- `dimStyle` — secondary/descriptive text.
- `categoryActiveStyle`, `categoryInactiveStyle` — status tab category selectors.

**`keymap.go`** — Define `KeyMap` struct using `bubbles/key`:
- Global bindings: `Tab`, `ShiftTab`, `Quit` (q, ctrl+c), `Help` (?), `Tab1`/`Tab2`/`Tab3` (1, 2, 3).
- Sort bindings: `StartSort` (s), `EditSettings` (e), `NewRun` (n).
- Log bindings: `ScrollUp` (k, up), `ScrollDown` (j, down), `Top` (g), `Bottom` (G), `Filter` (f), `Inspect` (enter).
- Overlay bindings: `Close` (esc).
- Status bindings: `Category1`–`Category5` (1–5), `Refresh` (r).
- `ShortHelp()` and `FullHelp()` methods for the `bubbles/help` component.

---

### Task 9 — TUI Shared Components (`internal/tui/components.go`)

**Agent:** @developer
**Priority:** medium
**Depends on:** 1, 8

Create `internal/tui/components.go` with reusable sub-components:

**`styledProgress`** — Wraps `bubbles/progress.Model` with the TUI's adaptive styling. Methods:
- `newStyledProgress(width int) styledProgress`
- `SetPercent(pct float64)`
- `View() string` — renders the bar with percentage, count, and ETA.

**`activityLog`** — Wraps `bubbles/viewport.Model` for the scrollable activity log:
- `newActivityLog(width, height int) activityLog`
- `Append(line string)` — adds a line, auto-scrolls to bottom if in follow mode.
- `SetFilter(filter string)` — filters visible lines by verb (COPY, DUPE, ERR, SKIP, or "" for all).
- `ToggleFollow()` — toggles auto-scroll.
- `Update(msg tea.Msg) (activityLog, tea.Cmd)` — delegates to viewport.
- `View() string`

**`workerPane`** — Displays per-worker status:
- `newWorkerPane(numWorkers int) workerPane`
- `SetWorkerStatus(workerID int, stage, filename string)`
- `SetWorkerIdle(workerID int)`
- `View(width, maxHeight int) string` — renders worker lines; collapses to summary if `maxHeight` is too small.

**`errorOverlay`** — Modal overlay for error details:
- `newErrorOverlay() errorOverlay`
- `Show(relPath, stage, errMsg string)` — populates and makes visible.
- `Hide()`
- `Visible() bool`
- `View(width, height int) string` — centered bordered box with error details.

---

### Task 10 — TUI Root App Model (`internal/tui/app.go`)

**Agent:** @developer
**Priority:** high
**Depends on:** 8, 9

Create `internal/tui/app.go`:

**`AppOptions`** struct — passed from `cmd/gui.go`:
```go
type AppOptions struct {
    Config   *config.AppConfig
    Registry *discovery.Registry
    Hasher   *hash.Hasher
    Version  string
}
```

**`App`** struct (the root `tea.Model`):
- Fields: `activeTab int`, `tabs []string` (["Sort", "Verify", "Status"]), `sort SortModel`, `verify VerifyModel`, `status StatusModel`, `width int`, `height int`, `keymap KeyMap`, `help help.Model`.
- `NewApp(opts AppOptions) App` — constructor. Initializes all three sub-models with shared config/registry.
- `Init() tea.Cmd` — returns `tea.Batch` of sub-model inits (only the active tab's init matters, but all can be batched).
- `Update(msg tea.Msg) (tea.Model, tea.Cmd)`:
  - `tea.WindowSizeMsg` → store width/height, propagate to all sub-models.
  - `tea.KeyMsg`:
    - Global keys (Tab, ShiftTab, 1/2/3, q, ctrl+c, ?) handled here.
    - All other keys delegated to the active sub-model's `Update`.
  - All other messages delegated to the active sub-model.
- `View() string`:
  - Header bar: `pixe gui` left-aligned, tab buttons right-aligned. Active tab highlighted.
  - Content area: active sub-model's `View()`, sized to fill `height - headerHeight - footerHeight`.
  - Footer bar: context-sensitive help from the active sub-model's key bindings.

Tab switching: when switching tabs, the new tab's sub-model receives a `tabActivatedMsg` so it can refresh if needed (e.g., Status tab re-walks on activation).

---

### Task 11 — TUI Sort Tab (`internal/tui/sort.go`)

**Agent:** @developer
**Priority:** high
**Depends on:** 2, 5, 9, 10

Create `internal/tui/sort.go`:

**`SortModel`** struct:
- Fields: `state` (configure/running/complete), `config *config.AppConfig`, `registry *discovery.Registry`, `hasher *hash.Hasher`, `version string`, `bus *progress.Bus`, `progress styledProgress`, `log activityLog`, `workers workerPane`, `overlay errorOverlay`, `result *progress.RunSummary`, `filter string`, `width int`, `height int`.

**Configure state:**
- `View()` renders the current config summary (source, dest, workers, algorithm, recursive, copyright, camera-owner) and action hints ([s] Start Sort, [e] Edit Settings).
- `Update`: `s` key → call `startRun()` which creates a bus, builds `SortOptions` (using the extracted `resolveConfig`/`buildRegistry` helpers), launches `pipeline.Run` in a goroutine, transitions to running state, returns `listenForEvents` cmd.
- If `--dest` is empty, the `s` key is disabled and a prompt is shown.

**Running state:**
- `Update`: handles `eventMsg` — updates progress bar, appends formatted lines to activity log, updates worker pane. Handles scroll keys, filter toggle, enter for error inspection.
- `View()` renders the three-pane layout (progress + counters, activity log, worker pane). If overlay is visible, renders it on top.
- On `busClosedMsg` or `EventRunComplete`: transition to complete state.

**Complete state:**
- `View()` renders the summary with breakdown bars (COPY/DUPE/SKIP/ERR proportional bars).
- `Update`: `Enter` → scroll through activity log, `e` → filter to errors, `n` → reset to configure state, `q` → quit.

**Event-to-log formatting:** Each terminal event is formatted into a styled line for the activity log:
- `EventFileComplete` → `COPY relPath → destination` (green-ish via adaptive color).
- `EventFileDuplicate` → `DUPE relPath → matches dest` (yellow-ish).
- `EventFileSkipped` → `SKIP relPath → reason` (dim).
- `EventFileError` → `ERR  relPath → reason` (red-ish).

---

### Task 12 — TUI Verify Tab (`internal/tui/verify.go`)

**Agent:** @developer
**Priority:** high
**Depends on:** 2, 5, 9, 10

Create `internal/tui/verify.go`:

**`VerifyModel`** struct:
- Fields: `state` (configure/running/complete), `dir string`, `registry`, `hasher`, `bus`, `progress`, `log`, `overlay`, `result`, `width`, `height`.

**Configure state:**
- Shows the directory to verify and action hints. If `--dest` was provided to `pixe gui`, it's pre-filled.
- `v` key starts the verify run.

**Running state:**
- Same event bus bridge pattern as Sort. Events: `EventVerifyOK`, `EventVerifyMismatch`, `EventVerifyUnrecognised`.
- Activity log shows `OK`, `MISMATCH`, `UNRECOGNISED` lines.
- Progress bar shows total/completed.

**Complete state:**
- Summary: verified count, mismatches, unrecognised.
- `e` key filters to mismatches. `n` starts a new verify.

---

### Task 13 — TUI Status Tab (`internal/tui/status.go`)

**Agent:** @developer
**Priority:** high
**Depends on:** 5, 9, 10

Create `internal/tui/status.go`:

**`StatusModel`** struct:
- Fields: `state` (loading/ready), `source string`, `config *config.AppConfig`, `registry *discovery.Registry`, `categories []categoryData` (5 categories: sorted, duplicates, errored, unsorted, unrecognized), `activeCategory int`, `viewport viewport.Model`, `summary statusSummary`, `ledgerInfo string`, `width`, `height`.

**`categoryData`** struct: `name string`, `count int`, `files []statusFile`.
**`statusFile`** struct: `relPath string`, `detail string` (destination, match, reason, or empty).

**Loading state:**
- On `tabActivatedMsg` or `r` key: launch a background goroutine that calls `discovery.Walk()` and `manifest.LoadLedger()`, classifies files (same algorithm as `cmd/status.go` Section 7.4.2), and sends a `statusResultMsg` back to the model.
- `View()` shows a spinner + "Loading...".

**Ready state:**
- `View()` renders: source line, ledger info line, summary counters, category selector ([1]–[5] with counts), and the active category's file list in a scrollable viewport.
- `Update`: `1`–`5` keys switch categories and repopulate the viewport. `r` refreshes. Scroll keys navigate the viewport.

---

### Task 14 — `pixe gui` Cobra Command (`cmd/gui.go`)

**Agent:** @developer
**Priority:** high
**Depends on:** 5, 10, 11, 12, 13

Create `cmd/gui.go`:

1. Define `guiCmd` with `Use: "gui"`, `Short`, `Long` descriptions, `RunE: runGUI`.
2. Register the same flags as `sortCmd` (source, dest, workers, algorithm, copyright, camera-owner, dry-run, db-path, recursive, skip-duplicates, ignore, no-carry-sidecars, overwrite-sidecar-tags). Bind to Viper with the same keys.
3. In `init()`: `rootCmd.AddCommand(guiCmd)`.
4. `runGUI` function:
   - Call `resolveConfig()` to get `*config.AppConfig`.
   - Call `buildRegistry()` to get `*discovery.Registry`.
   - Build `*hash.Hasher`.
   - Construct `tui.AppOptions` with config, registry, hasher, version.
   - Create `tui.NewApp(opts)`.
   - Create `tea.NewProgram(app, tea.WithAltScreen())`.
   - Call `p.Run()`.
   - Return any error.

**Note:** The `gui` command does NOT open the archive database upfront — the Sort tab opens it when a run starts (it needs the resolved dest path, which may be edited interactively). The Status tab doesn't need a database at all.

---

### Task 15 — Tests: Event Bus

**Agent:** @tester
**Priority:** high
**Depends on:** 2

Write `internal/progress/progress_test.go`:

1. **`TestBus_EmitAndReceive`** — Create a bus, emit 5 events, receive all 5 from `Events()` channel. Verify fields are preserved.
2. **`TestBus_NonBlockingEmit`** — Create a bus with buffer size 1. Emit 100 events rapidly. Verify the bus does not block (the test completes within a timeout). Some events will be dropped — that's correct.
3. **`TestBus_Close`** — Create a bus, emit events, close it. Verify the `Events()` channel is closed (range loop exits).
4. **`TestBus_CloseIdempotent`** — Call `Close()` twice. No panic.
5. **`TestBus_EmitAfterClose`** — Emit after close. No panic, event is silently dropped.
6. **`TestBus_TimestampSet`** — Emit an event with zero timestamp. Verify the received event has a non-zero timestamp.
7. **`TestPlainWriter_SortOutput`** — Create a bus and PlainWriter. Emit a sequence of sort events (skip, copy, dupe, error, run complete). Verify the PlainWriter's output matches the expected plain-text format (COPY, SKIP, DUPE, ERR lines + summary).
8. **`TestPlainWriter_VerifyOutput`** — Same for verify events (OK, MISMATCH, UNRECOGNISED, done).

---

### Task 16 — Tests: CLI Progress Model

**Agent:** @tester
**Priority:** high
**Depends on:** 6

Write `internal/cli/progress_test.go`:

1. **`TestProgressModel_Init`** — Verify `Init()` returns a non-nil `tea.Cmd`.
2. **`TestProgressModel_CounterUpdates`** — Send synthetic `eventMsg` values through `Update()`. Verify `copied`, `duplicates`, `skipped`, `errors` counters increment correctly for each event kind.
3. **`TestProgressModel_DiscoverDone`** — Send `EventDiscoverDone` with `Total: 100`. Verify `total` is set.
4. **`TestProgressModel_CurrentFile`** — Send `EventFileStart` with `RelPath: "IMG_001.jpg"`. Verify `currentFile` is updated.
5. **`TestProgressModel_DoneOnRunComplete`** — Send `EventRunComplete`. Verify the model returns `tea.Quit`.
6. **`TestProgressModel_DoneOnBusClosed`** — Send `busClosedMsg`. Verify `tea.Quit`.
7. **`TestProgressModel_WindowResize`** — Send `tea.WindowSizeMsg`. Verify `width` is updated and progress bar is resized.
8. **`TestProgressModel_ViewContainsCounters`** — After updating with events, verify `View()` output contains the expected counter strings.
9. **`TestProgressModel_VerifyMode`** — Create with `mode: "verify"`. Verify `View()` shows "verified" and "mismatches" labels instead of "copied" and "duplicates".

---

### Task 17 — Tests: TUI Models

**Agent:** @tester
**Priority:** medium
**Depends on:** 10, 11, 12, 13

Write `internal/tui/tui_test.go`:

1. **`TestApp_TabSwitching`** — Create an App. Send Tab key. Verify `activeTab` cycles 0→1→2→0. Send ShiftTab. Verify reverse cycle. Send "1", "2", "3" keys. Verify direct jump.
2. **`TestApp_WindowSize`** — Send `tea.WindowSizeMsg`. Verify width/height propagated to all sub-models.
3. **`TestApp_QuitKeys`** — Send "q". Verify `tea.Quit` returned. Send "ctrl+c". Same.
4. **`TestSortModel_StateTransitions`** — Verify initial state is "configure". Simulate start (mock pipeline). Verify transition to "running". Send `EventRunComplete`. Verify transition to "complete". Send "n". Verify back to "configure".
5. **`TestSortModel_EventCounters`** — In running state, send a sequence of events. Verify counters match.
6. **`TestSortModel_ActivityLogAppend`** — Send terminal events. Verify the activity log viewport content contains the formatted lines.
7. **`TestSortModel_FilterCycle`** — In running state, send "f" key repeatedly. Verify filter cycles through All→COPY→DUPE→ERR→SKIP→All.
8. **`TestVerifyModel_StateTransitions`** — Same pattern as sort: configure → running → complete.
9. **`TestStatusModel_CategorySwitching`** — In ready state, send "1"–"5" keys. Verify `activeCategory` updates and viewport content changes.
10. **`TestStatusModel_Refresh`** — Send "r" key. Verify state transitions to loading then back to ready.
