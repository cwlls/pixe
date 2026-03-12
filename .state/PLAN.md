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
| 12  | Commit all changes                                | 12       | @committer | [x]    | 11         | Wave 5 — complete                        |

---

## Sprint: A4 (TIFF Handler) + A2 (AVIF Handler)

### Task Summary

| #   | Task                                                          | Priority | Agent      | Status | Depends On | Notes                                    |
|:----|:--------------------------------------------------------------|:---------|:-----------|:-------|:-----------|:-----------------------------------------|
| 13  | A4 — Implement standalone TIFF handler                        | 1        | @developer | [x]    | —          | Wave 1                                   |
| 14  | A2 — AVIF EXIF extraction spike                               | 2        | @developer | [x]    | —          | Wave 1 — custom parser needed            |
| 15  | A2 — Implement AVIF handler                                   | 3        | @developer | [x]    | 14         | Wave 2                                   |
| 16  | Consolidate handler registration (`resume.go`, `status.go`)   | 4        | @developer | [x]    | 13, 15     | Wave 3                                   |
| 17  | Register TIFF + AVIF handlers in `buildRegistry()`            | 5        | @developer | [x]    | 13, 15, 16 | Wave 3                                   |
| 18  | Verify all tests pass (`make check`)                          | 6        | @tester    | [x]    | 16, 17     | Wave 4 — complete                        |
| 19  | Commit all changes                                            | 7        | @committer | [~]    | 18         | Wave 5                                   |

---

### Parallelization Strategy

#### Wave 1 — Independent work (tasks 13, 14)
Tasks 13 and 14 touch completely different packages and have no file overlap. They can be executed in parallel.

#### Wave 2 — AVIF handler implementation (task 15)
Depends on the spike result from task 14 to determine the EXIF extraction approach.

#### Wave 3 — Registration and consolidation (tasks 16, 17)
Both tasks modify `cmd/` files. Task 16 refactors `resume.go` and `status.go` to call `buildRegistry()`. Task 17 adds the two new handlers to `buildRegistry()` in `helpers.go`. These two tasks touch different files and can run in parallel, but both depend on the handler packages existing (tasks 13, 15).

#### Wave 4 — Validation (task 18)
Runs `make check` (fmt-check + vet + unit tests) to validate everything compiles and passes.

#### Wave 5 — Commit (task 19)

---



#### Task 19 — Commit all changes

Commit the complete sprint as a single conventional commit.

**Acceptance criteria:**
- Clean `git status` after commit
- Commit message follows project conventions
