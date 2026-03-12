# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Add missing package doc comments (`main`, `internal/domain/`) | high | @developer | [x] complete | — | AGENTS.md violations; only 2 packages affected |
| 2  | Document `cmd/` Cobra command variables (15 vars) | medium | @developer | [x] complete | — | All subcommand vars except `rootCmd` |
| 3  | Document `cmd/` RunE handler functions (13 funcs) | medium | @developer | [x] complete | — | All `run*` functions lack doc comments |
| 4  | Document other undocumented `cmd/` functions and flag vars | medium | @developer | [x] complete | — | `printFilesTable`, `printFilesJSON`, `printRunTable`, `printRunJSON`, `runQueryDuplicateList`, `runQueryDuplicatePairs`, flag vars |
| 5  | Document `LedgerStatus*` constants individually | medium | @developer | [x] complete | — | 4 constants in `domain/pipeline.go` |
| 6  | Add field-level doc comments to `domain/pipeline.go` structs | medium | @developer | [x] complete | 5 | `ManifestEntry` (9 fields), `Manifest` (8 fields) |
| 7  | Add field-level doc comments to `pipeline/pipeline.go` structs | medium | @developer | [x] complete | — | `SortOptions` (3 fields), `SortResult` (4 fields) |
| 8  | Add field-level doc comments to `verify/verify.go` structs | medium | @developer | [x] complete | — | `Result` (3 fields), `FileResult` (3 fields), `Options` (4 fields) |
| 9  | Add field-level doc comments to `manifest/manifest.go` struct | low | @developer | [x] complete | — | `LedgerContents` (2 fields) |
| 10 | Add field-level doc comments to `domain/handler.go` struct | low | @developer | [x] complete | — | `MagicSignature` (2 fields) |
| 11 | Standardize `exifDateFormat` doc comments across handlers | low | @developer | [x] complete | — | `heic`, `tiffraw`, `cr3` — match `jpeg` pattern |
| 12 | Add doc comments to unexported interface method implementations | low | @developer | [x] complete | — | `Read()`/`Close()` on `tiffraw` and `cr3` types |
| 13 | Add doc comments to unexported struct types in handlers | low | @developer | [x] complete | — | `mp4.sampleLocation`, `cr3.stscEntry` |
| 14 | Add doc comment to `tiffraw.newMultiSectionReader()` | low | @developer | [x] complete | — | Unexported constructor missing doc |
| 15 | Move `mp4.buildMinimalMP4()` to `mp4_test.go` | low | @developer | [x] complete | — | Test helper in production code |
| 16 | Add doc comment to `dblocator/filesystem_windows.go` `isNetworkMount` | low | @developer | [x] complete | — | Darwin/Linux variants are documented |
| 17 | Add inline doc comment to `discovery/walk.go` `DiscoveredFile.Handler` | low | @developer | [x] complete | — | Only undocumented field in struct |
| 18 | Add doc comments to `migrate/migrate.go` unexported constants | low | @developer | [x] complete | — | `pixeDir`, `manifestFile`, `migratedSuffix` |
| 19 | Fix mismatched comment in `cmd/status_test.go` | low | @developer | [x] complete | — | `statusFixturesDir` comment says "fixturesDir" |
| 20 | Run `make check` to verify no regressions | high | @tester | [x] complete | 1–19 | `make fmt-check && make vet && make test` |

---

## Task Descriptions

### Task 20: Run `make check` to verify no regressions

**Priority:** High — gate task.

Run `make check` (which executes `fmt-check`, `vet`, and unit tests) to verify that all documentation changes compile correctly, pass formatting checks, and introduce no test regressions. Also run `make lint` for golangci-lint validation.

```bash
make check && make lint
```

If any failures occur, fix them before marking this task complete.
