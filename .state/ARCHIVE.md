# Completed Features Archive

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
