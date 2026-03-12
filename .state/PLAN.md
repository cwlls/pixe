# Implementation

## Task Summary

| #   | Task                                              | Priority | Agent      | Status | Depends On | Notes                                    |
|:----|:--------------------------------------------------|:---------|:-----------|:-------|:-----------|:-----------------------------------------|
| 1   | B5 — Date filter flags (`--since`, `--before`)    | 1        | @developer | [x]    | —          | Wave 1                                   |
| 2   | D3 — Verbosity levels (`--quiet`, `--verbose`)    | 2        | @developer | [x]    | —          | Wave 1                                   |
| 3   | D4 — Colorized terminal output                    | 3        | @developer | [x]    | 2          | Wave 2 (depends on verbosity plumbing)   |
| 4   | E1 — Config auto-discovery in `dirA`              | 4        | @developer | [x]    | —          | Wave 1                                   |
| 5   | C4 — `pixe stats` command                         | 5        | @developer | [x]    | —          | Wave 1                                   |
| 6   | A3 — PNG handler                                  | 6        | @developer | [x]    | —          | Wave 1                                   |
| 7   | A6 — ORF handler (Olympus RAW)                    | 7        | @developer | [x]    | —          | Wave 1                                   |
| 8   | A6 — RW2 handler (Panasonic RAW)                  | 8        | @developer | [x]    | —          | Wave 1                                   |
| 9   | Register new handlers in CLI + tests              | 9        | @developer | [x]    | 6, 7, 8    | Wave 2                                   |
| 10  | E2 — Config profiles (`--profile`)                | 10       | @developer | [x]    | 4          | Wave 3 (depends on config auto-discovery)|
| 11  | Integration tests for new features                | 11       | @tester    | [x]    | 1–10       | Wave 4 — complete                        |
| 12  | Commit all changes                                | 12       | @committer | [~]    | 11         | Wave 5 — in-process                      |

---

## Parallelization Strategy

Tasks are grouped into waves. All tasks within a wave can be executed in parallel.

### Wave 1 — Independent features (tasks 1, 2, 4, 5, 6, 7, 8)
All seven tasks touch different packages with no overlapping files. They can be developed in parallel.

### Wave 2 — Dependent features (tasks 3, 9)
- Task 3 (colorized output) depends on task 2 (verbosity levels) because color rendering must respect `--quiet` mode.
- Task 9 (handler registration) depends on tasks 6, 7, 8 (the handlers themselves).

### Wave 3 — Config profiles (task 10)
Depends on task 4 (config auto-discovery) because profiles extend the config loading chain.

### Wave 4 — Integration tests (task 11)
Validates all features end-to-end.

### Wave 5 — Commit (task 12)

---

## Task Descriptions

All tasks 1–11 have been completed and validated. Task 12 (commit) is pending.
