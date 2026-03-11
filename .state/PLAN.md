# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add `MetadataCapability` type and `MetadataSupport()` to `FileTypeHandler` interface | High | Developer | [x] complete | — | Domain layer change; breaks all handler compilation until updated |
| 2 | Update `tiffraw.Base` with `MetadataSupport()` returning `MetadataSidecar` | High | Developer | [x] complete | 1 | Fixes DNG, NEF, CR2, PEF, ARW in one shot |
| 3 | Update standalone handlers (JPEG, HEIC, MP4, CR3) with `MetadataSupport()` | High | Developer | [x] complete | 1 | JPEG → Embed; HEIC, MP4, CR3 → Sidecar |
| 4 | Remove MP4 `WriteMetadataTags` stub implementation | High | Developer | [x] complete | 3 | Replace with no-op; remove dead udta comment |
| 5 | Create `internal/xmp/` package — XMP sidecar writer | High | Developer | [x] complete | 1 | New package; pure Go, no dependencies |
| 6 | Update `internal/tagging/` — add sidecar dispatch via `MetadataSupport()` | High | Developer | [x] complete | 1, 5 | Central routing: embed vs. sidecar vs. none |
| 7 | Update pipeline tagging stage (sequential path) | High | Developer | [x] complete | 6 | `pipeline.go` processFile |
| 8 | Update pipeline tagging stage (concurrent path) | High | Developer | [x] complete | 6 | `worker.go` runWorker |
| 9 | Update handler tests — replace `WriteMetadataTags_noop` with `MetadataSupport` tests | Medium | Developer | [x] complete | 2, 3 | 9 handler test files |
| 10 | Update `internal/tagging/` tests — cover sidecar dispatch | Medium | Developer | [x] complete | 6 | Mock handler with each capability value |
| 11 | Add `internal/xmp/` tests — XMP output validation | Medium | Developer | [x] complete | 5 | Template rendering, field omission, Adobe packet structure |
| 12 | Add pipeline integration test — sidecar written for RAW, embedded for JPEG | Medium | Developer | [x] complete | 7, 8 | End-to-end in `internal/integration/` |
| 13 | Run full test suite, lint, vet | High | Developer | [x] complete | 1–12 | `make check && make test-all` |

