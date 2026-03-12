# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Add Charm dependencies | high | @developer | [x] complete | — | `go get` bubbletea, bubbles, lipgloss, go-isatty |
| 2  | Event bus package (`internal/progress/`) | high | @developer | [x] complete | — | Pure stdlib; event types, Bus, PlainWriter |
| 3  | Instrument sort pipeline with event emission | high | @developer | [x] complete | 2 | Add `EventBus` to `SortOptions`, emit calls in pipeline.go + worker.go |
| 4  | Instrument verify pipeline with event emission | high | @developer | [x] complete | 2 | Add `EventBus` to `verify.Options`, emit calls in verify.go |
| 5  | Extract shared config/registry helpers in `cmd/` | medium | @developer | [x] complete | — | Factor `resolveConfig()` and `buildRegistry()` out of `runSort` for reuse by gui.go |
| 6  | CLI progress bar package (`internal/cli/`) | high | @developer | [x] complete | 1, 2 | ProgressModel, styles, ETA calculation |
| 7  | Wire `--progress` flag into `pixe sort` and `pixe verify` | high | @developer | [x] complete | 3, 4, 6 | Flag binding, TTY detection, bus+model wiring in sort.go and verify.go |
| 8  | TUI shared styles and keymap (`internal/tui/styles.go`, `keymap.go`) | medium | @developer | [x] complete | 1 | Adaptive Lip Gloss styles, KeyMap struct, help bindings |
| 9  | TUI shared components (`internal/tui/components.go`) | medium | @developer | [x] complete | 1, 8 | Styled progress bar wrapper, log viewport, error overlay, worker status pane |
| 10 | TUI root App model (`internal/tui/app.go`) | high | @developer | [x] complete | 8, 9 | Tab routing, header/footer, window size, global key handling |
| 11 | TUI Sort tab (`internal/tui/sort.go`) | high | @developer | [x] complete | 2, 5, 9, 10 | Configure → running → complete states, event bus bridge, activity log, worker pane |
| 12 | TUI Verify tab (`internal/tui/verify.go`) | high | @developer | [x] complete | 2, 5, 9, 10 | Configure → running → complete states, event bus bridge |
| 13 | TUI Status tab (`internal/tui/status.go`) | high | @developer | [x] complete | 5, 9, 10 | Walk + ledger load in background goroutine, category switching, scrollable lists |
| 14 | `pixe gui` Cobra command (`cmd/gui.go`) | high | @developer | [x] complete | 5, 10, 11, 12, 13 | Command definition, flag binding, `runGUI` wiring |
| 15 | Tests: event bus | high | @tester | [x] complete | 2 | Non-blocking emit, close semantics, PlainWriter output fidelity |
| 16 | Tests: CLI progress model | high | @tester | [x] complete | 6 | Model update tests — event→counter mapping, ETA, done transition |
| 17 | Tests: TUI models | medium | @tester | [x] complete | 10, 11, 12, 13 | Tab switching, sort state transitions, verify state transitions, status classification |

---

## Task Descriptions

*All 17 tasks have been completed and archived. See `.state/ARCHIVE.md` for the full implementation summary.*
