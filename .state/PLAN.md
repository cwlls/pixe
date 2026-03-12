# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Fix `extractFormats` to resolve inherited `MetadataSupport` from embedded `tiffraw.Base` | high | @developer | [x] complete | — | Fallback check added to detect tiffraw imports; ARW, CR2, DNG, NEF, PEF now show "XMP sidecar" |
| 2  | Exclude `handlertest` from format table output | high | @developer | [x] complete | — | Exclusion condition updated to include `handlertest` alongside `tiffraw` |
| 3  | Fix CI shallow-clone race condition: `actions/checkout` doesn't fetch tags | high | @developer | [x] complete | — | Added `fetch-tags: true` to actions/checkout@v4 in test job |
| 4  | Add test for inherited metadata capability detection | medium | @developer | [x] complete | 1 | `TestExtractFormats_tiffrawHandlersShowSidecar` verifies all TIFF handlers show "XMP sidecar" |
| 5  | Add test that `handlertest` is excluded from format table | medium | @developer | [x] complete | 2 | `TestExtractFormats_excludesHandlertest` verifies test infrastructure is not in output |
| 6  | Regenerate documentation files via `make docs` / `go run ./internal/docgen` | high | @developer | [x] complete | 1, 2 | README.md and docs/how-it-works.md format tables regenerated with correct metadata |
| 7  | Run full test suite to verify no regressions | medium | @tester | [x] complete | 1, 2, 3, 4, 5 | All 33 packages pass with `-race`; `make docs-check` passes |

---

## Parallelization Strategy

Tasks are grouped into waves. All tasks within a wave can be executed in parallel.

### Wave 1 — Fix all three bugs (Tasks 1, 2, 3)
All three fixes are independent: Task 1 and 2 are in `internal/docgen/extract.go`, Task 3 is in `.github/workflows/ci.yml`.

### Wave 2 — Add tests (Tasks 4, 5)
Both test additions are in `internal/docgen/docgen_test.go` and can be written in parallel.

### Wave 3 — Regenerate & verify (Tasks 6, 7)
Regenerate docs, then run the full test suite to confirm everything passes.


