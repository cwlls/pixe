# Implementation State

**Status:** Active — Atomic Copy Complete, Skip Duplicates Pending (Architecture §4.6, §4.10)

## Summary

**Completed:** Tasks 1–6 (Atomic Copy feature fully implemented and tested)
**Pending:** Tasks 7–15 (Skip Duplicates feature and integration tests)

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 7  | Atomic copy: integration tests | Medium | @tester | [ ] pending | 3, 4 | End-to-end: interrupted copy leaves no partial files |
| 8  | Skip-duplicates: add `SkipDuplicates` to `AppConfig` | High | @developer | [ ] pending | — | Config struct change |
| 9  | Skip-duplicates: add `--skip-duplicates` CLI flag | High | @developer | [ ] pending | 8 | Flag, Viper binding, config construction |
| 10 | Skip-duplicates: sequential pipeline skip path | High | @developer | [ ] pending | 8 | Short-circuit in `processFile` after dedup check |
| 11 | Skip-duplicates: concurrent pipeline skip path | High | @developer | [ ] pending | 8 | Coordinator skips worker dispatch for dupes |
| 12 | Skip-duplicates: ledger entry for skipped duplicates | High | @developer | [ ] pending | 10, 11 | `destination` omitted, `matches` present |
| 13 | Skip-duplicates: DB row for skipped duplicates | High | @developer | [ ] pending | 10, 11 | `dest_path`/`dest_rel` NULL, `is_duplicate=1` |
| 14 | Skip-duplicates: tests | High | @tester | [ ] pending | 10, 11, 12, 13 | Unit + integration |
| 15 | Update ARCHITECTURE.md cross-references if needed | Low | @scribe | [ ] pending | 7, 14 | Final consistency pass |

---

## Completed Tasks (Archived)

### Tasks 1–6: Atomic Copy Implementation

All atomic copy tasks have been **successfully completed** and are now archived. The implementation includes:

- **Task 1:** `copy.Execute` refactored to write to temp file (`.<name>.pixe-tmp`)
- **Task 2:** `copy.Verify`, `copy.Promote`, and `copy.CleanupTempFile` functions implemented
- **Task 3:** Sequential pipeline (`processFile`) updated with atomic copy flow
- **Task 4:** Concurrent pipeline (`runWorker`) updated with atomic copy flow
- **Task 5:** Mismatch handling updated to delete temp files instead of preserving them
- **Task 6:** Comprehensive unit tests (19 tests) covering all atomic copy scenarios

**Implementation files:**
- `internal/copy/copy.go` — Core atomic copy engine (TempPath, Execute, Verify, Promote, CleanupTempFile)
- `internal/copy/copy_test.go` — 19 unit tests with full coverage
- `internal/pipeline/pipeline.go` — Sequential pipeline integration
- `internal/pipeline/worker.go` — Concurrent pipeline integration

**Key design points:**
- Temp files are placed in the same directory as the final destination to ensure atomic `os.Rename` on the same filesystem
- Verification happens on the temp file before promotion to canonical path
- On mismatch, temp file is deleted; source in `dirA` remains untouched for reprocessing
- Package doc comment updated to describe the atomic safety model
- All tests use `t.TempDir()` for isolation and cleanup

**Architecture alignment:** Implementation matches Section 4.10 of ARCHITECTURE.md exactly.
