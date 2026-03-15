# Implementation Plan

## Feature: Config file bug fix — alias sigil change (`@` → `+`) and silent parse error reporting

Two bugs prevent `.pixe.yaml` config files from working reliably:
1. The `@` alias prefix is a YAML reserved character, causing the entire config file to silently fail to parse when `dest: @name` is used unquoted.
2. `initConfig()` silently swallows all config parse errors, giving users no feedback when their config is not loaded.

Reference: ARCHITECTURE.md §4.15, §7.2

## Task Summary

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Change alias sigil from `@` to `+` in `resolveAlias()` | high | @developer | [x] complete | — | Core logic change |
| 2 | Update `resolveAlias()` tests for `+` sigil | high | @developer | [x] complete | 1 | All 8 test functions in `helpers_test.go` |
| 3 | Add config parse error reporting to `initConfig()` | high | @developer | [x] complete | — | `cmd/root.go` — distinguish not-found vs parse error |
| 4 | Add `initConfig()` error reporting tests | high | @developer | [x] complete | 3 | New test file `cmd/root_test.go` or add to existing |
| 5 | Update `docs/configuration.md` — sigil `@` → `+` | medium | @developer | [x] complete | 1 | 8 occurrences across the file |
| 6 | Run full test suite and lint | high | @tester | [x] complete | 1, 2, 3, 4, 5 | `make check` must pass |
| 7 | Commit | low | @committer | [x] complete | 6 | — |

---

## Parallelization Strategy

**Wave 1** (parallel — no dependencies between them):
- Task 1: sigil change in `resolveAlias()`
- Task 3: `initConfig()` error reporting

**Wave 2** (parallel — each depends only on its Wave 1 counterpart):
- Task 2: `resolveAlias()` tests (depends on 1)
- Task 4: `initConfig()` tests (depends on 3)
- Task 5: docs update (depends on 1)

**Wave 3** (sequential):
- Task 6: full test suite + lint (depends on all above)
- Task 7: commit (depends on 6)


