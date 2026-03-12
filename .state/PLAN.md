# Implementation Plan

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Add `CarrySidecars` and `OverwriteSidecarTags` to `AppConfig` | high | @developer | [ ] pending | — | Config layer only |
| 2  | Add `--no-carry-sidecars` and `--overwrite-sidecar-tags` CLI flags | high | @developer | [ ] pending | 1 | Cobra flags + Viper bindings in `cmd/sort.go` |
| 3  | Add `SidecarFile` type to `internal/discovery/` | high | @developer | [ ] pending | — | New struct, package-level sidecar extension list |
| 4  | Add `Sidecars` field to `DiscoveredFile` | high | @developer | [ ] pending | 3 | Extend existing struct |
| 5  | Implement sidecar association in `Walk()` | high | @developer | [ ] pending | 3, 4 | Two-pass: classify then match sidecars to parents |
| 6  | Add `carried_sidecars` column via schema migration v2→v3 | high | @developer | [ ] pending | — | `ALTER TABLE files ADD COLUMN carried_sidecars TEXT` |
| 7  | Add `CarriedSidecars` field to `FileRecord` and update scan/insert | high | @developer | [ ] pending | 6 | JSON-encoded `[]string`, nullable |
| 8  | Add `Sidecars` field to `LedgerEntry` | medium | @developer | [ ] pending | — | `[]string` with `omitempty` |
| 9  | Implement `xmp.MergeTags()` | high | @developer | [ ] pending | — | Parse existing XMP, inject/merge tags, add namespaces |
| 10 | Update `tagging.Apply()` to support XMP merge path | high | @developer | [ ] pending | 9 | New signature or wrapper; check for carried `.xmp` |
| 11 | Add sidecar copy helper to `internal/copy/` | medium | @developer | [ ] pending | — | Simple file copy (no temp/verify) |
| 12 | Integrate sidecar carry into pipeline `processFile()` | high | @developer | [ ] pending | 4, 5, 10, 11 | Between verify and tag stages |
| 13 | Integrate sidecar carry into concurrent worker path | high | @developer | [ ] pending | 12 | Mirror sequential logic in `worker.go` |
| 14 | Wire sidecar data into DB update and ledger write | high | @developer | [ ] pending | 7, 8, 12 | Pass carried sidecar paths to coordinator |
| 15 | Add `+sidecar` stdout output lines | medium | @developer | [ ] pending | 12 | After COPY line, indented sidecar lines |
| 16 | Handle dry-run mode for sidecar carry | medium | @developer | [ ] pending | 12, 15 | Show association without copying |
| 17 | Handle duplicate routing for sidecars | medium | @developer | [ ] pending | 12 | Sidecars follow parent to `duplicates/` or skip |
| 18 | Write unit tests for sidecar discovery and association | high | @tester | [ ] pending | 5 | Stem match, full-ext match, case-insensitive, orphan |
| 19 | Write unit tests for `xmp.MergeTags()` | high | @tester | [ ] pending | 9 | Inject missing, skip existing, overwrite mode, namespace add |
| 20 | Write unit tests for updated `tagging.Apply()` | medium | @tester | [ ] pending | 10 | Merge path vs generate path |
| 21 | Write integration tests for end-to-end sidecar carry | medium | @tester | [ ] pending | 12, 13, 14 | Full pipeline with `.aae` and `.xmp` sidecars |
| 22 | Run full test suite and lint | high | @tester | [ ] pending | 1–17 | `make check && make test-all` |

---

## Task Descriptions

### Task 1: Add `CarrySidecars` and `OverwriteSidecarTags` to `AppConfig`

**File:** `internal/config/config.go`

Add two new fields to the `AppConfig` struct:

```go
// CarrySidecars controls whether pre-existing sidecar files (.aae, .xmp)
// in dirA are carried alongside their parent media file to dirB.
// Default is true (enabled). Set to false via --no-carry-sidecars.
CarrySidecars bool

// OverwriteSidecarTags controls the merge behavior when Pixe injects
// metadata tags into a carried .xmp sidecar that already contains those
// fields. When false (default), existing values in the source .xmp are
// preserved (source is authoritative). When true, Pixe's configured
// --copyright and --camera-owner values replace existing values.
OverwriteSidecarTags bool
```

Note the inverted polarity: the CLI flag is `--no-carry-sidecars` (negative) but the config field and config file key are positive (`CarrySidecars`, `carry_sidecars`). The default value is `true`.

---

### Task 2: Add `--no-carry-sidecars` and `--overwrite-sidecar-tags` CLI flags

**File:** `cmd/sort.go`

In `init()`, register two new flags:

```go
sortCmd.Flags().Bool("no-carry-sidecars", false, "disable carrying pre-existing .aae and .xmp sidecar files from source to destination")
sortCmd.Flags().Bool("overwrite-sidecar-tags", false, "when merging tags into a carried .xmp sidecar, overwrite existing values instead of preserving them")
```

Bind to Viper. Note the polarity inversion for `carry_sidecars`:

```go
_ = viper.BindPFlag("no_carry_sidecars", sortCmd.Flags().Lookup("no-carry-sidecars"))
_ = viper.BindPFlag("overwrite_sidecar_tags", sortCmd.Flags().Lookup("overwrite-sidecar-tags"))
```

In `runSort()`, populate the config:

```go
CarrySidecars:        !viper.GetBool("no_carry_sidecars"),
OverwriteSidecarTags: viper.GetBool("overwrite_sidecar_tags"),
```

The config file key `carry_sidecars` defaults to `true`. When the user sets `carry_sidecars: false` in `.pixe.yaml`, it has the same effect as `--no-carry-sidecars`. The Viper binding uses the negative flag name (`no_carry_sidecars`) so the two don't conflict — the `runSort` function inverts it when populating `AppConfig`.

---

### Task 3: Add `SidecarFile` type to `internal/discovery/`

**File:** `internal/discovery/walk.go` (or a new `internal/discovery/sidecar.go`)

Define the `SidecarFile` struct and the recognized sidecar extensions:

```go
// SidecarFile represents a pre-existing sidecar file in dirA that is
// associated with a parent media file by stem matching.
type SidecarFile struct {
    Path    string // absolute path in dirA
    RelPath string // relative path from dirA
    Ext     string // normalized lowercase extension (e.g., ".aae", ".xmp")
}

// sidecarExtensions is the set of file extensions recognized as sidecars.
// This is a package-level constant — not user-configurable.
var sidecarExtensions = map[string]bool{
    ".aae": true,
    ".xmp": true,
}
```

---

### Task 4: Add `Sidecars` field to `DiscoveredFile`

**File:** `internal/discovery/walk.go`

Extend the existing `DiscoveredFile` struct:

```go
type DiscoveredFile struct {
    Path     string                  // absolute path for file I/O
    RelPath  string                  // relative path from dirA for display and ledger
    Handler  domain.FileTypeHandler  // resolved handler
    Sidecars []SidecarFile           // pre-existing sidecars from dirA (may be empty)
}
```

This is a non-breaking change — existing code that doesn't reference `Sidecars` continues to work (zero-value is `nil` slice).

---

### Task 5: Implement sidecar association in `Walk()`

**File:** `internal/discovery/walk.go` (or `internal/discovery/sidecar.go`)

After the existing `filepath.WalkDir` completes, add a second pass that runs only when `carrySidecars` is true. The `WalkOptions` struct needs a new field:

```go
type WalkOptions struct {
    Recursive      bool
    Ignore         *ignore.Matcher
    CarrySidecars  bool // when true, associate sidecar files with parent media files
}
```

**Second-pass algorithm:**

1. Build an index of discovered files keyed by `(directory, lowercase_stem)` for O(1) lookup. For each discovered file, also index by `(directory, lowercase_full_filename)` to support the `IMG_1234.HEIC.xmp` convention.
2. Iterate the `skipped` slice. For each entry whose extension (lowercased) is in `sidecarExtensions`:
   a. Extract the sidecar's directory and stem. For a file like `IMG_1234.xmp`, the stem is `IMG_1234`. For `IMG_1234.HEIC.xmp`, the "stem" is `IMG_1234.HEIC` — check the full-filename index first (exact match for the `<name>.<ext>.xmp` convention), then fall back to stem match.
   b. Look up the parent in the index. If found, append a `SidecarFile` to the parent's `Sidecars` slice and mark this skipped entry for removal.
   c. If no parent found, update the skipped entry's reason to `"orphan sidecar: no matching media file"`.
3. Remove matched sidecars from the `skipped` slice.

**Edge case:** If multiple media files share a stem (e.g., `IMG_1234.HEIC` and `IMG_1234.JPG`), the sidecar associates with whichever was indexed first (discovery order). The full-extension convention (`IMG_1234.HEIC.xmp`) is unambiguous and always preferred.

---

### Task 6: Add `carried_sidecars` column via schema migration v2→v3

**File:** `internal/archivedb/schema.go`

1. Bump `schemaVersion` from `2` to `3`.
2. Add `carried_sidecars TEXT` to the `schemaDDL` `files` table definition (for new databases).
3. Add a v2→v3 migration in `migrateSchema()`:

```go
if currentVersion < 3 {
    migrations := []string{
        `ALTER TABLE files ADD COLUMN carried_sidecars TEXT`,
    }
    for _, m := range migrations {
        if _, err := db.conn.Exec(m); err != nil {
            if !strings.Contains(err.Error(), "duplicate column") {
                return fmt.Errorf("archivedb: migrate v2→v3: %w", err)
            }
        }
    }
    _, _ = db.conn.Exec(
        `INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (?, ?)`,
        3, time.Now().UTC().Format(time.RFC3339),
    )
}
```

The column is nullable `TEXT` storing a JSON array (e.g., `["2021/12-Dec/20211225_062223_7d97e98f.heic.aae"]`) or `NULL` when no sidecars were carried.

---

### Task 7: Add `CarriedSidecars` field to `FileRecord` and update scan/insert

**File:** `internal/archivedb/files.go`

1. Add to `FileRecord`:

```go
CarriedSidecars *string // JSON array of sidecar dest_rel paths, or nil
```

2. Add to `updateParams`:

```go
carriedSidecars *string
```

3. Add a functional option:

```go
func WithCarriedSidecars(paths []string) UpdateOption {
    return func(p *updateParams) {
        if len(paths) > 0 {
            data, _ := json.Marshal(paths)
            s := string(data)
            p.carriedSidecars = &s
        }
    }
}
```

4. Update the `UpdateFileStatus` method's SQL to include `carried_sidecars = ?` when the param is non-nil.
5. Update all `Scan` calls that read `FileRecord` to include the new column.

---

### Task 8: Add `Sidecars` field to `LedgerEntry`

**File:** `internal/domain/pipeline.go`

```go
type LedgerEntry struct {
    Path        string     `json:"path"`
    Status      string     `json:"status"`
    Checksum    string     `json:"checksum,omitempty"`
    Destination string     `json:"destination,omitempty"`
    VerifiedAt  *time.Time `json:"verified_at,omitempty"`
    Sidecars    []string   `json:"sidecars,omitempty"`    // carried sidecar dest_rel paths
    Matches     string     `json:"matches,omitempty"`
    Reason      string     `json:"reason,omitempty"`
}
```

The `omitempty` tag ensures the field is absent from JSON when the slice is nil/empty.

---

### Task 9: Implement `xmp.MergeTags()`

**File:** `internal/xmp/xmp.go` (or a new `internal/xmp/merge.go`)

Add a new exported function:

```go
// MergeTags reads the existing XMP sidecar at sidecarPath, injects the
// provided metadata tags, and writes the result back atomically.
//
// When overwrite is false (default), existing values for dc:rights,
// xmpRights:Marked, and aux:OwnerName are preserved — Pixe only fills
// in fields that are missing from the source XMP.
//
// When overwrite is true, Pixe's configured values replace any existing
// values for those fields.
//
// Missing namespace declarations (xmlns:dc, xmlns:xmpRights, xmlns:aux)
// are added to the rdf:Description element as needed.
//
// Returns nil if tags.IsEmpty() — no modification is made.
func MergeTags(sidecarPath string, tags domain.MetadataTags, overwrite bool) error
```

**Implementation approach:**

Use `encoding/xml` to parse the XMP file. The XMP structure is:

```
<?xpacket ...?>
<x:xmpmeta>
  <rdf:RDF>
    <rdf:Description rdf:about="" xmlns:...>
      ... fields ...
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
```

Strategy: Read the file as bytes. Use a lightweight XML approach — since XMP files are small and well-structured, parse with `encoding/xml` into a generic structure, manipulate the DOM, and re-serialize. Alternatively, use string/regex-based insertion if the XML structure is predictable enough (XMP files follow a rigid schema).

Recommended approach: Parse with `encoding/xml` using a custom struct that captures the `rdf:Description` element's attributes (namespace declarations) and child elements. This is more robust than regex.

Key operations:
1. Read file contents.
2. Parse to find the `rdf:Description` element.
3. Check for existing `dc:rights`, `xmpRights:Marked`, `aux:OwnerName` elements.
4. For each tag in `MetadataTags`:
   - If the corresponding element is missing: add it (and its namespace declaration if needed).
   - If the element exists and `overwrite` is false: skip.
   - If the element exists and `overwrite` is true: replace the value.
5. Serialize back to bytes.
6. Atomic write (temp file + rename).

**Important:** Preserve the `<?xpacket?>` wrapper and any other content outside the fields Pixe manages. The merge must be non-destructive to all XMP data Pixe doesn't own.

---

### Task 10: Update `tagging.Apply()` to support XMP merge path

**File:** `internal/tagging/tagging.go`

The `Apply` function needs to know whether a source `.xmp` sidecar was carried to the destination. Add a new function or extend the existing one:

```go
// ApplyWithSidecars persists metadata tags, accounting for carried source
// sidecars. When a .xmp sidecar was carried (carriedXMP is non-empty),
// the MetadataSidecar path merges tags into the existing file instead of
// generating a new one.
func ApplyWithSidecars(destPath string, handler domain.FileTypeHandler, tags domain.MetadataTags, carriedXMP string, overwrite bool) error {
    if tags.IsEmpty() {
        return nil
    }
    switch handler.MetadataSupport() {
    case domain.MetadataEmbed:
        if err := handler.WriteMetadataTags(destPath, tags); err != nil {
            return fmt.Errorf("tagging: embed metadata in %q: %w", destPath, err)
        }
    case domain.MetadataSidecar:
        if carriedXMP != "" {
            // Merge into the carried source .xmp sidecar.
            if err := xmp.MergeTags(carriedXMP, tags, overwrite); err != nil {
                return fmt.Errorf("tagging: merge tags into carried sidecar %q: %w", carriedXMP, err)
            }
        } else {
            // No carried sidecar — generate from template (existing behavior).
            if err := xmp.WriteSidecar(destPath, tags); err != nil {
                return fmt.Errorf("tagging: write sidecar for %q: %w", destPath, err)
            }
        }
    case domain.MetadataNone:
        // No tagging for this format.
    }
    return nil
}
```

The existing `Apply()` function can remain as a convenience wrapper that calls `ApplyWithSidecars(destPath, handler, tags, "", false)` for backward compatibility.

---

### Task 11: Add sidecar copy helper to `internal/copy/`

**File:** `internal/copy/copy.go` (or a new `internal/copy/sidecar.go`)

Add a simple file copy function for sidecars — no temp file, no hash verification:

```go
// CopySidecar copies a sidecar file from src to dest. Unlike Execute,
// this is a simple copy without temp-file atomicity or hash verification.
// Sidecars are small metadata files; the source in dirA is always
// available for re-copy if needed.
//
// Parent directories are created if they do not exist.
func CopySidecar(src, dest string) error
```

Implementation: `os.Open(src)` → `os.Create(dest)` → `io.Copy` → close both → preserve mtime. Create parent dirs with `os.MkdirAll`. Wrap errors with `fmt.Errorf("copy: sidecar %q → %q: %w", src, dest, err)`.

---

### Task 12: Integrate sidecar carry into pipeline `processFile()`

**File:** `internal/pipeline/pipeline.go`

In the `processFile()` function (sequential path), after the verify+promote stage succeeds and before the tag stage:

1. If `cfg.CarrySidecars` is true and `df.Sidecars` is non-empty:
   a. For each `SidecarFile` in `df.Sidecars`:
      - Derive the destination path: `absDest + sidecar.Ext` (e.g., `/archive/2021/12-Dec/20211225_062223_7d97e98f.heic` + `.aae` → `.heic.aae`).
      - Call `copypkg.CopySidecar(sidecar.Path, sidecarDest)`.
      - On failure: log `WARN` to output, continue (non-fatal).
      - On success: record the sidecar dest_rel path in a `[]string` for later DB/ledger use.
   b. Track whether a `.xmp` sidecar was carried (needed for the tag stage).

2. In the tag stage, pass the carried XMP destination path (if any) to `tagging.ApplyWithSidecars()` along with `cfg.OverwriteSidecarTags`.

3. After the tag stage, pass the carried sidecar paths to the DB update call via `archivedb.WithCarriedSidecars(paths)`.

4. Include the sidecar paths in the `LedgerEntry.Sidecars` field.

**Sidecar destination path derivation helper** (can live in `internal/pathbuilder/` or inline):

```go
// SidecarDestPath returns the destination path for a carried sidecar,
// given the parent media file's absolute destination path and the
// sidecar's lowercase extension.
//
// Example: SidecarDestPath("/archive/2021/12-Dec/20211225_abc.heic", ".aae")
//        → "/archive/2021/12-Dec/20211225_abc.heic.aae"
func SidecarDestPath(mediaDestPath, sidecarExt string) string {
    return mediaDestPath + sidecarExt
}
```

---

### Task 13: Integrate sidecar carry into concurrent worker path

**File:** `internal/pipeline/worker.go`

Mirror the sidecar carry logic from Task 12 in the worker's `processAssignment()` function. The same stages apply:

1. After verify+promote, copy sidecars.
2. Pass carried XMP path to the tag stage.
3. Include sidecar paths in the result sent back to the coordinator.

The coordinator already handles DB writes and ledger appends — it just needs to receive the sidecar paths from the worker result and pass them through.

Update the worker result struct (or the channel message type) to include `CarriedSidecars []string`.

---

### Task 14: Wire sidecar data into DB update and ledger write

**File:** `internal/pipeline/pipeline.go`, `internal/pipeline/worker.go`

Ensure the coordinator:

1. Calls `db.UpdateFileStatus(fileID, status, archivedb.WithCarriedSidecars(sidecarPaths))` when completing a file that had sidecars carried.
2. Sets `ledgerEntry.Sidecars = sidecarRelPaths` before writing the ledger entry.

This is the final wiring — Tasks 7 and 8 define the data structures, this task connects them.

---

### Task 15: Add `+sidecar` stdout output lines

**File:** `internal/pipeline/pipeline.go`, `internal/pipeline/worker.go`

After emitting the `COPY` line for a file, emit indented `+sidecar` lines for each carried sidecar:

```
COPY IMG_1234.HEIC -> 2021/12-Dec/20211225_062223_7d97e98f.heic
     +sidecar IMG_1234.aae -> 2021/12-Dec/20211225_062223_7d97e98f.heic.aae
     +sidecar IMG_1234.xmp -> 2021/12-Dec/20211225_062223_7d97e98f.heic.xmp (merge tags)
```

The `(merge tags)` suffix appears only when the sidecar is `.xmp` AND tags are configured. Use the `syncWriter` for thread-safe output in the concurrent path.

---

### Task 16: Handle dry-run mode for sidecar carry

**File:** `internal/pipeline/pipeline.go`

In dry-run mode:
- Sidecar association still happens during discovery (Task 5 runs regardless).
- The `+sidecar` output lines are emitted (with `(dry run)` suffix or as part of the parent's dry-run output).
- No sidecar files are copied or modified.
- The `(merge tags)` annotation still appears to show what *would* happen.

---

### Task 17: Handle duplicate routing for sidecars

**File:** `internal/pipeline/pipeline.go`, `internal/pipeline/worker.go`

When a file is identified as a duplicate:

- **Default mode (copy duplicates):** The parent is routed to `duplicates/<run_timestamp>/...`. Sidecars are copied there too, using the same `SidecarDestPath()` derivation against the duplicate destination path.
- **Skip duplicates mode (`--skip-duplicates`):** No copy occurs for the parent, so no sidecars are copied either. The ledger entry has no `sidecars` field (consistent with no `destination` field).

---

### Task 18: Write unit tests for sidecar discovery and association

**File:** `internal/discovery/walk_test.go` (or `internal/discovery/sidecar_test.go`)

Test cases:
- **Stem match:** `IMG_1234.xmp` associates with `IMG_1234.HEIC` → sidecar in parent's `Sidecars` slice.
- **Full-extension match:** `IMG_1234.HEIC.xmp` associates with `IMG_1234.HEIC`.
- **Case-insensitive:** `img_1234.xmp` associates with `IMG_1234.HEIC`.
- **Multiple sidecars:** `IMG_1234.aae` and `IMG_1234.xmp` both associate with `IMG_1234.HEIC`.
- **Orphan sidecar:** `ORPHAN.xmp` with no matching media file → remains in skipped with reason `"orphan sidecar: no matching media file"`.
- **Ambiguous stem:** `IMG_1234.HEIC` and `IMG_1234.JPG` both exist; `IMG_1234.xmp` associates with one (deterministic based on discovery order).
- **Full-extension disambiguates:** `IMG_1234.HEIC.xmp` associates with `IMG_1234.HEIC` even when `IMG_1234.JPG` exists.
- **Carry disabled:** `CarrySidecars: false` → no association, sidecars remain in skipped as `"unsupported format: .xmp"`.
- **Sidecar in subdirectory (recursive):** Sidecar only matches parent in the same directory, not a parent in a different directory.

---

### Task 19: Write unit tests for `xmp.MergeTags()`

**File:** `internal/xmp/xmp_test.go` (or `internal/xmp/merge_test.go`)

Test cases:
- **Inject into empty description:** Source XMP has `rdf:Description` with no copyright/owner fields → fields are added, namespace declarations are added.
- **Inject with existing unrelated fields:** Source XMP has Lightroom develop settings → those are preserved, Pixe fields are added.
- **Skip existing (overwrite=false):** Source XMP already has `dc:rights` → Pixe's copyright is NOT injected; existing value preserved.
- **Overwrite existing (overwrite=true):** Source XMP already has `dc:rights` → Pixe's copyright replaces it.
- **Partial overlap:** Source has `dc:rights` but not `aux:OwnerName`; overwrite=false → copyright preserved, owner injected.
- **Namespace addition:** Source XMP has `xmlns:dc` but not `xmlns:aux` → `xmlns:aux` is added to `rdf:Description`.
- **Empty tags:** `tags.IsEmpty()` → file is not modified.
- **Atomic write:** Verify temp file is used and renamed.
- **Malformed XMP:** Graceful error return, source file not corrupted.

---

### Task 20: Write unit tests for updated `tagging.Apply()`

**File:** `internal/tagging/tagging_test.go`

Test cases:
- **No carried XMP, MetadataSidecar handler:** Calls `xmp.WriteSidecar` (existing behavior).
- **Carried XMP, MetadataSidecar handler:** Calls `xmp.MergeTags` instead of `WriteSidecar`.
- **Carried XMP, MetadataEmbed handler:** Ignores carried XMP, calls `WriteMetadataTags` (embed path unchanged).
- **Empty tags with carried XMP:** No-op (no merge attempted).
- **Overwrite flag propagation:** `overwrite=true` is passed through to `MergeTags`.

---

### Task 21: Write integration tests for end-to-end sidecar carry

**File:** `internal/integration/sidecar_carry_test.go`

End-to-end tests using the full pipeline:

- **Basic carry:** Source has `IMG.HEIC` + `IMG.aae` → both appear in `dirB`, `.aae` renamed to match.
- **XMP carry with tag merge:** Source has `DSC.NEF` + `DSC.xmp` (with Lightroom settings), `--copyright` configured → destination `.xmp` has both Lightroom settings and Pixe copyright.
- **XMP carry without tags:** Source has `DSC.NEF` + `DSC.xmp`, no `--copyright` → `.xmp` carried verbatim.
- **Carry disabled:** `--no-carry-sidecars` → sidecars not carried, reported as skipped.
- **Duplicate with sidecars:** Duplicate file with sidecars → sidecars follow to `duplicates/`.
- **Skip-duplicates with sidecars:** `--skip-duplicates` → no sidecars copied.
- **Dry-run with sidecars:** `--dry-run` → `+sidecar` lines in output, no files copied.
- **Overwrite sidecar tags:** Source `.xmp` has existing copyright, `--overwrite-sidecar-tags` → Pixe's copyright replaces it.
- **DB and ledger verification:** Check `carried_sidecars` column in DB and `sidecars` field in ledger.

---

### Task 22: Run full test suite and lint

Run `make check && make test-all` to verify all existing tests still pass and no lint issues were introduced. Fix any failures before marking the feature complete.
