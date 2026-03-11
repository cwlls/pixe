# Architectural Overview: Pixe

## 1. Vision & Strategy

**Pixe** is a Go-based photo and video sorting utility that safely organizes irreplaceable media files into a deterministic, date-based directory structure with embedded integrity checksums. It is designed for personal and family media archives where data loss is unacceptable.

### Core Problem

Media libraries accumulate across devices, cameras, and cloud exports with inconsistent naming and no structural organization. Pixe provides a single, repeatable operation that transforms a flat directory of media into a chronologically organized, integrity-verified archive — without ever risking the originals.

### North Star Principles

1. **Safety above all else.** Source files are never modified or moved. Every copy is verified before being considered complete. An interrupted run can always be resumed.
2. **Native Go execution.** All functionality — metadata extraction, hashing, file operations — uses native Go packages. No shelling out to `exiftool`, `ffmpeg`, or other external binaries.
3. **Deterministic output.** Given the same input files, configuration, and system locale, Pixe always produces the same directory structure and filenames. (Month directory names are locale-aware; see Section 4.5.)
4. **Modular by design.** New file types are added by implementing a Go interface. The core engine is format-agnostic.

---

## 2. Technical Stack

| Concern | Choice | Rationale |
|---|---|---|
| **Language** | Go | Performance, concurrency primitives, single-binary distribution |
| **CLI Framework** | `spf13/cobra` | Industry-standard Go CLI framework, subcommand support |
| **Configuration** | `spf13/viper` | Config file + env var + flag merging, pairs with Cobra |
| **Image EXIF** | Native Go packages (e.g., `rwcarlsen/goexif` or equivalent) | No external binary dependency |
| **HEIC Parsing** | Native Go package (to be evaluated) | Must support EXIF extraction and data-region isolation |
| **MP4 Parsing** | Native Go package (e.g., `abema/go-mp4` or equivalent) | Atom-level access for metadata and keyframe extraction |
| **TIFF/RAW Parsing** | Native Go TIFF parser (e.g., `golang.org/x/image/tiff` or equivalent) | IFD traversal for EXIF extraction and embedded JPEG preview location in TIFF-based RAW formats (DNG, NEF, CR2, PEF, ARW) |
| **CR3 Parsing** | ISOBMFF parser (reuses HEIC/MP4 approach) | Canon RAW X container; box-based EXIF and JPEG preview extraction |
| **Hashing** | `crypto/sha1` (default), `crypto/sha256`, others via `crypto` stdlib | Configurable algorithm, SHA-1 default for filename brevity |
| **Persistence** | SQLite database (CGo-free: `modernc.org/sqlite`) | Cumulative registry, concurrent-safe, queryable; see Section 8 |

---

## 3. Version Management

### 3.1 Source of Truth: Git Tags

Pixe follows the **idiomatic Go convention**: the git tag is the single source of truth for the version string. There is no version literal anywhere in Go source code. All three version fields — `Version`, `Commit`, and `BuildDate` — are injected at build time via Go linker flags (`-ldflags -X`).

The version variables live in the **`cmd` package** (not a separate `internal/version` package), co-located with the CLI commands that use them. This is the standard pattern for small-to-medium Go CLIs.

**Location:** `cmd/version.go`

```go
package cmd

// Version fields — injected at build time via -ldflags -X.
// When built without ldflags (e.g. plain `go build` or `go test`), these
// retain their dev defaults.
var (
    version   = "dev"
    commit    = "unknown"
    buildDate = "unknown"
)

func init() {
    // If this is a dev build (no ldflags), enrich the version string
    // with the current commit hash for traceability.
    if version == "dev" && commit != "unknown" {
        version = "dev-" + commit
    }
}
```

- `version` follows **Semantic Versioning** (`MAJOR.MINOR.PATCH`), without the `v` prefix; the `v` is prepended at display time.
- All three fields are **unexported `var`s** — settable by ldflags, invisible outside `cmd`.
- On a **dev build** (plain `go build` with no ldflags), `version` stays `"dev"` and `commit` stays `"unknown"`, so the output reads `pixe dev`.
- On a **Makefile build** (`make build`), the Makefile injects `commit` from `git rev-parse --short HEAD` but does **not** inject `version`. The `init()` function detects this combination and produces `version = "dev-2159446"`, making local builds traceable to their exact commit.
- On a **release build** (GoReleaser or `goreleaser build`), all three fields are injected from the git tag, commit SHA, and build timestamp.

### 3.2 Build Tooling

GoReleaser is the **single authority** for how binaries are built. The `.goreleaser.yaml` configuration defines the ldflags, target platforms, and archive formats. The Makefile delegates to GoReleaser rather than duplicating build logic.

#### GoReleaser (`/.goreleaser.yaml`)

GoReleaser derives `{{.Version}}` from the git tag (stripping the `v` prefix), `{{.Commit}}` from the git SHA, and `{{.Date}}` from the build timestamp. These are injected via:

```yaml
builds:
  - ldflags:
      - >-
        -s -w
        -X github.com/cwlls/pixe-go/cmd.version={{.Version}}
        -X github.com/cwlls/pixe-go/cmd.commit={{.Commit}}
        -X github.com/cwlls/pixe-go/cmd.buildDate={{.Date}}
```

This is the canonical build path for releases. `goreleaser release` creates tagged release artifacts; `goreleaser build --single-target --snapshot` creates local binaries using the same ldflags logic.

#### Makefile (`/Makefile`)

The Makefile uses `goreleaser build` for all binary compilation, ensuring a single source of build truth:

```makefile
build:  ## Build pixe for the current platform via GoReleaser
	goreleaser build --single-target --snapshot --clean -o ./pixe

build-debug:  ## Build without stripping symbols (for dlv) — bypasses GoReleaser
	go build -gcflags "all=-N -l" -o ./pixe .
```

- `make build` invokes `goreleaser build --single-target --snapshot`, which builds for the current OS/arch only, uses the ldflags from `.goreleaser.yaml`, and places the binary at `./pixe`. The `--snapshot` flag allows building from untagged commits (the version field will resolve to the snapshot version, which GoReleaser derives from the latest tag + commit offset).
- `make build-debug` is the sole exception — it bypasses GoReleaser to produce an unstripped binary for debugger attachment. In this mode, all version fields retain their defaults (`"dev"`, `"unknown"`).
- `make install` uses `goreleaser build` and then copies the binary to `$GOPATH/bin`.

> **Key benefit:** Build logic is defined in exactly one place (`.goreleaser.yaml`). The Makefile, CI, and release pipeline all use the same ldflags. No drift is possible.

### 3.3 Accessor Function

A package-level function in `cmd/version.go` formats the full human-readable version string:

```go
// fullVersion returns the human-readable version string, e.g.:
//   Release:  "pixe v0.10.0 (commit: abc1234, built: 2026-03-06T10:30:00Z)"
//   Dev:      "pixe dev-2159446 (commit: 2159446, built: unknown)"
func fullVersion() string {
    return fmt.Sprintf("pixe v%s (commit: %s, built: %s)", version, commit, buildDate)
}
```

This is used by the `pixe version` CLI command and is the canonical display format.

### 3.4 Consumers

| Consumer | What it reads | How |
|---|---|---|
| **`pixe version` CLI command** | `fullVersion()` | Prints the formatted string to stdout |
| **Pipeline (Manifest, Ledger)** | `cmd.Version()` | Exported getter; records which Pixe version produced each run |
| **Archive database (`pixe.db`)** | `cmd.Version()` | Same — stamped into the `runs.pixe_version` column |

Because the version vars are unexported, an **exported getter** is provided for internal consumers:

```go
// Version returns the current version string for use by internal packages
// (e.g., pipeline stamping into manifests and ledgers).
func Version() string { return version }
```

### 3.5 Version Bump Process (Release)

To release a new version:

1. Commit all changes.
2. Tag: `git tag v0.11.0`
3. Push: `git push origin v0.11.0`
4. GoReleaser extracts `0.11.0` from the tag and injects it into the binary.

**No Go source file needs to change for a version bump.** The git tag is the sole input.

### 3.6 Dev Build Version Strings

| Build Method | `version` | `commit` | Example Output |
|---|---|---|---|
| `go build .` (bare) | `dev` | `unknown` | `pixe vdev (commit: unknown, built: unknown)` |
| `make build` | `dev` (snapshot) | `abc1234` | `pixe vdev-abc1234 (commit: abc1234, built: unknown)` |
| `goreleaser build --snapshot` | snapshot ver | `abc1234` | `pixe v0.10.0-next (commit: abc1234, built: 2026-03-07...)` |
| `goreleaser release` (tagged) | `0.10.0` | `abc1234` | `pixe v0.10.0 (commit: abc1234, built: 2026-03-07...)` |

> **Note on persistent artifacts:** When `version` is `"dev"` or starts with `"dev-"`, the pipeline stamps this string into manifests and ledgers. This is intentional — it makes dev-produced artifacts immediately identifiable. Production archives should always be produced from tagged release builds.

---

## 4. Conceptual Design

### 4.1 High-Level Data Flow

```
dirA (read-only source)          dirB (organized destination)
┌──────────────────┐             ┌──────────────────────────────────────┐
│ IMG_0001.jpg     │  ────────>  │ 2021/12-Dec/                         │
│ IMG_0002.jpg     │   discover  │   20211225_062223_7d97e98f...jpg      │
│ IMG_1234.jpg     │   filter    │ 2022/02-Feb/                         │
│ VID_0010.mp4     │   extract   │   20220202_123101_447d3060...jpg      │
│ notes.txt        │   hash      │ 2022/03-Mar/                         │
│ subfolder/       │   copy      │   20220316_232122_321c7d6f...jpg      │
│   IMG_5678.jpg   │   verify    │ duplicates/                          │
│                  │   tag       │   20260306_103000/                    │
│ .pixe_ledger.json│  (ignored)  │     2022/02-Feb/                     │
└──────────────────┘             │       20220202_123101_447d...jpg      │
                                 │ .pixe/                               │
  stdout:                        │   pixe.db  (or dbpath marker)        │
  COPY IMG_0001.jpg -> 2021/...  └──────────────────────────────────────┘
  SKIP IMG_1234.jpg -> previously imported
  DUPE IMG_0002.jpg -> matches 2022/02-Feb/20220202...jpg
  ERR  notes.txt    -> unsupported format: .txt
```

### 4.2 Pipeline Stages

Each file passes through these stages, tracked in the archive database:

```
pending → extracted → hashed → copied → verified → tagged → complete
                                   ↓         ↓         ↓
                                 failed   mismatch   tag_failed
```

1. **Pending** — File discovered in `dirA`, not yet processed.
2. **Extracted** — Filetype module has read the file, extracted the capture date, and identified the hashable data region.
3. **Hashed** — Checksum computed over the media payload (data only, excluding metadata).
4. **Copied** — File written to a temporary file (`.<filename>.pixe-tmp`) in the destination directory within `dirB`. The file does not yet exist at its canonical path. See Section 4.10 for the atomic copy design.
5. **Verified** — Temporary file re-read and checksum recomputed; matches the source hash. On success, the temp file is atomically renamed to its canonical destination path. On mismatch, the temp file is deleted (the source in `dirA` is untouched and the file can be reprocessed). See Section 4.10.
6. **Tagged** — Optional metadata persisted to the destination. The pipeline queries the handler's `MetadataSupport()` capability to determine the strategy:
   - **`MetadataEmbed`** → Tags written directly into the destination file (e.g., JPEG EXIF).
   - **`MetadataSidecar`** → XMP sidecar file written alongside the destination file (e.g., `*.arw.xmp`).
   - **`MetadataNone`** → Tagging skipped entirely; stage advances directly to complete.
   - If no tags are configured (`tags.IsEmpty()`), the stage is skipped regardless of capability.
7. **Complete** — All operations successful. Recorded in ledger.

Error states (`failed`, `mismatch`, `tag_failed`) halt processing for that file and flag it for user attention. A `tag_failed` status means the media file was successfully copied and verified, but metadata persistence (embedded or sidecar) failed. The file is still present in `dirB`.

### 4.3 Pipeline Output

Every file discovered in `dirA` produces exactly one line of stdout output, regardless of outcome. This provides a complete, auditable record of what happened during the run. The output format mirrors `COPY` for all four outcomes:

```
COPY <source_filename> -> <destination_relative_path>
SKIP <source_filename> -> <reason>
DUPE <source_filename> -> <reason>
ERR  <source_filename> -> <reason>
```

#### Output Verbs

| Verb | Meaning | Example |
|---|---|---|
| **`COPY`** | File successfully processed and copied to `dirB` | `COPY IMG_0001.jpg -> 2021/12-Dec/20211225_062223_7d97e98f...jpg` |
| **`SKIP`** | File not copied because it was already processed | `SKIP IMG_0001.jpg -> previously imported` |
| **`SKIP`** | File not copied because its type is not recognized | `SKIP notes.txt -> unsupported format: .txt` |
| **`DUPE`** | File is a content duplicate of an already-archived file | `DUPE IMG_0042.jpg -> matches 2022/02-Feb/20220202_123101_447d3060...jpg` |
| **`ERR`** | File processing failed at some pipeline stage | `ERR  IMG_9999.jpg -> EXIF parse failed: truncated IFD at offset 0x1A` |

**Skip reasons:**

- `previously imported` — The file's source path was already processed in a prior run (found in the archive database with a terminal status).
- `unsupported format: .<ext>` — No registered `FileTypeHandler` claims this file extension, or magic-byte verification failed.

**Duplicate reasons:**

- `matches <dest_rel>` — The file's content checksum matches an already-archived file. The `<dest_rel>` is the relative path within `dirB` of the existing copy. By default, the duplicate is still physically copied to `duplicates/<run_timestamp>/...` for user review, and the `DUPE` line confirms this routing. When `--skip-duplicates` is active, no copy occurs — the `DUPE` line is emitted and the file is recorded in the database and ledger, but no bytes are written to `dirB`. See Section 4.6.

**Error reasons:**

- Freetext description of the failure, drawn from the error returned by the pipeline stage that failed (e.g., `EXIF parse failed: ...`, `copy failed: permission denied`, `verification mismatch: expected abc123, got def456`).

#### Ledger Recording

All four outcomes — `COPY`, `SKIP`, `DUPE`, `ERR` — are streamed to the JSONL ledger (see Section 8.8). Every file discovered in `dirA` is appended as an independent JSON line with a `status` field indicating its outcome. Entries are written in processing order as the coordinator finalizes each result. This ensures the ledger is a complete manifest of what Pixe saw and decided for every file in the source directory.

### 4.4 Ignore List

Pixe maintains a list of **glob patterns** for files that should be completely invisible to the pipeline — not discovered, not counted, not reported, and not recorded in the ledger. Ignored files are as if they do not exist in `dirA`.

#### Hardcoded Ignores

The following pattern is always ignored, regardless of configuration:

- `.pixe_ledger.json` — Pixe's own ledger file. Without this, Pixe would discover its own ledger when re-processing a source directory and report it as an unrecognized file type.

This is the **only** hardcoded entry. All other ignore patterns are user-configured.

#### User-Configured Ignores

Additional ignore patterns are specified via CLI flag or config file, using standard glob syntax (as implemented by Go's `filepath.Match`):

**CLI flag:** `--ignore <glob>` (repeatable — each occurrence adds one pattern)

```bash
pixe sort --dest ./archive --ignore "*.txt" --ignore ".DS_Store" --ignore "Thumbs.db"
```

**Config file (`.pixe.yaml`):**

```yaml
ignore:
  - "*.txt"
  - ".DS_Store"
  - "Thumbs.db"
  - "*.aae"
```

Patterns from the CLI flag and config file are **merged** (additive). The hardcoded ledger ignore is always present in addition to user patterns.

#### Matching Behavior

- Patterns are matched against the **filename only** (not the full path) for files in the top-level directory.
- When `--recursive` is enabled, patterns are matched against the **relative path from `dirA`** as well as the filename. This allows patterns like `subfolder/*.tmp` or `**/Thumbs.db`.
- Matching uses Go's `filepath.Match` semantics (supports `*`, `?`, `[...]` character classes, but not `**` recursive glob — `**` support may be added via `doublestar` library if needed).
- Directories themselves are never ignored — only files within them. (A future enhancement could support directory-level ignore patterns for recursive mode.)

### 4.5 Output Naming Convention

```
YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
```

- **Date/Time**: Extracted from file metadata (see Section 4.7).
- **Checksum**: Hex-encoded hash of the media payload. Default SHA-1 (40 hex characters).
- **Extension**: Lowercase, preserved from original.

**Directory structure:**

```
<dirB>/<YYYY>/<MM>-<Mon>/<filename>
```

- Year: 4-digit.
- Month: Zero-padded two-digit number, a hyphen, and the locale-aware three-letter title-cased month abbreviation (e.g., `01-Jan`, `02-Feb`, `03-Mar`, …, `12-Dec`). The abbreviation is derived from the user's system locale, so a French locale would produce `03-Mar` → `03-Mars` (or the locale's equivalent short form). The number is always zero-padded to two digits.

> **Note:** This format applies only to the month **directory name**. The filename retains its existing `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` format with a zero-padded numeric month.

### 4.6 Duplicate Handling

When a file's checksum matches an already-processed file (same data payload), the behavior depends on the `--skip-duplicates` flag.

#### Default Behavior (copy duplicates)

Without `--skip-duplicates`, the duplicate is physically copied to a quarantine directory:

```
<dirB>/duplicates/<run_timestamp>/<YYYY>/<MM>-<Mon>/<filename>
```

- `<run_timestamp>`: ISO-ish format of the Pixe invocation time (e.g., `20260306_103000`).
- The subdirectory structure mirrors the normal import layout, as if `duplicates/<run_timestamp>/` were the root of `dirB`. The month directory uses the same `<MM>-<Mon>` format as the primary archive.
- This preserves the duplicate for user review without polluting the primary archive.

#### Skip Duplicates Mode (`--skip-duplicates`)

When `--skip-duplicates` is active, the pipeline skips the copy entirely for files whose checksum matches an already-archived file. No bytes are written to `dirB`. This eliminates the I/O cost of copying files that are already in the archive — particularly valuable for re-import workflows where a large percentage of source files are already archived.

**What happens:**

1. The file is still fully processed through the extract and hash stages (the checksum must be computed to determine it is a duplicate).
2. The coordinator's pre-copy dedup check identifies the match.
3. The copy, verify, and tag stages are skipped entirely.
4. A `DUPE` line is emitted to stdout: `DUPE IMG_0042.jpg -> matches 2022/02-Feb/20220202...jpg`
5. A database row is recorded with `status = 'complete'`, `is_duplicate = 1`, the computed checksum, and `dest_path`/`dest_rel` left NULL (no file was written).
6. A ledger entry is appended with `status: "duplicate"`, `checksum`, and `matches` fields, but no `destination` field (consistent with `omitempty` — absence of `destination` signals no physical copy was made).

**Safety rationale:** The default remains "copy duplicates" because the safety-first principle says: when in doubt, preserve the file somewhere. Users who know they want to skip can explicitly opt in. The source files in `dirA` are never modified regardless of this flag — if a skipped duplicate needs to be recovered, it can always be re-imported by running without `--skip-duplicates`.

### 4.7 Date Fallback Chain

Each filetype module extracts dates using format-appropriate methods. The **authoritative fallback chain** is:

1. **EXIF `DateTimeOriginal`** — Most reliable; represents shutter actuation.
2. **EXIF `CreateDate`** — Secondary; may differ for edited files.
3. **Default date: February 20, 1902** — Ansel Adams' birthday. Used when no metadata date is available. This makes "unknown date" files immediately identifiable in the archive by their `19020220` prefix.

Filesystem timestamps (`ModTime`, `CreationTime`) are explicitly **not used** — they are unreliable across copies, cloud syncs, and OS transfers.

### 4.8 Metadata Tagging (Optional)

On copy to `dirB`, Pixe can inject select metadata tags. The **tagging strategy** depends on the file format's capabilities — some formats support safe embedded metadata writing, while others receive an XMP sidecar file instead. Tags are **never written to the source**.

| Tag | CLI Flag | Template Support | Example |
|---|---|---|---|
| **Copyright** | `--copyright` | Yes — `{{.Year}}` expands to the file's capture year | `"Copyright {{.Year}} My Family, all rights reserved"` |
| **CameraOwner** | `--camera-owner` | No — freetext string | `"Wells Family"` |

- Both tags are optional. If omitted, no tagging occurs (no embedded writes, no sidecar files).
- Tagging occurs **after** copy and verify — the checksum reflects the original data, not the tagged version.

#### 4.8.1 Hybrid Tagging Strategy

Each `FileTypeHandler` declares its metadata capability via `MetadataSupport()` (see Section 6.1). The pipeline uses this to select the tagging strategy:

| Capability | Strategy | Formats | Rationale |
|---|---|---|---|
| **`MetadataEmbed`** | Write tags directly into the destination file's native metadata structure | JPEG | Maximum portability — metadata travels inside the file and is visible to all consumer apps (Lightroom, Photos, etc.) |
| **`MetadataSidecar`** | Write an XMP sidecar file alongside the destination file | HEIC, MP4/MOV, DNG, NEF, CR2, CR3, PEF, ARW | Avoids risky writes into proprietary or complex containers while still ensuring metadata is associated with the file |
| **`MetadataNone`** | No tagging at all | (none currently) | Reserved for future formats where even sidecar association is meaningless |

#### 4.8.2 XMP Sidecar Files

When a handler declares `MetadataSidecar`, the pipeline writes a standards-compliant XMP sidecar file alongside the destination copy in `dirB`.

**Naming convention (Adobe/Lightroom standard):**

```
<original_filename>.<original_ext>.xmp
```

Example: A file sorted to `2021/12-Dec/20211225_062223_7d97e98f...arw` produces a sidecar at `2021/12-Dec/20211225_062223_7d97e98f...arw.xmp`.

This follows the Adobe convention where the sidecar name includes the full original extension, ensuring unambiguous association when multiple formats share the same stem (e.g., a `.dng` and `.jpg` shot pair).

**XMP content:** The sidecar contains a minimal, valid XMP packet with the same two metadata fields currently supported:

```xml
<?xpacket begin="﻿" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:dc="http://purl.org/dc/elements/1.1/"
      xmlns:xmpRights="http://ns.adobe.com/xap/1.0/rights/"
      xmlns:aux="http://ns.adobe.com/exif/1.0/aux/">
      <dc:rights>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">Copyright 2021 My Family, all rights reserved</rdf:li>
        </rdf:Alt>
      </dc:rights>
      <xmpRights:Marked>True</xmpRights:Marked>
      <aux:OwnerName>Wells Family</aux:OwnerName>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
```

**Key design points:**

- **`dc:rights`** maps to the Copyright field (using `rdf:Alt` with `x-default` language tag per XMP spec).
- **`xmpRights:Marked`** is set to `True` to indicate the file is rights-managed (standard practice when Copyright is present).
- **`aux:OwnerName`** maps to the CameraOwner field (same EXIF `CameraOwnerName` 0xA430 semantic, expressed in the XMP `aux` namespace).
- Fields are omitted from the XMP when their corresponding tag value is empty (e.g., if only `--copyright` is set, `aux:OwnerName` is absent).
- The XMP packet uses the `<?xpacket?>` processing instructions for compatibility with Adobe tools.

**Sidecar lifecycle:**

- Sidecars are written to the **destination directory only** — never to `dirA` (preserving the "dirA is read-only" constraint).
- Sidecars are **generated artifacts**, not copied files. They are not subject to the copy-then-verify integrity flow (no hash check).
- Sidecar write failure is **non-fatal** (same as embedded tag failure): the pipeline logs a warning, sets the DB status to `tag_failed`, and continues. The media file itself is already safely copied and verified.
- In **dry-run mode**, sidecar files are not written (same as embedded tags).
- Sidecars for **duplicate files** are written alongside the duplicate copy in `duplicates/<run_timestamp>/...`, mirroring the media file's routing.

### 4.9 Recursive Source Processing

By default, Pixe processes only the **top-level files** in `dirA`. Subdirectories are not traversed. The `--recursive` / `-r` flag enables recursive descent into all subdirectories of `dirA`.

#### Default Behavior (non-recursive)

```bash
pixe sort --dest ./archive                       # source defaults to cwd
pixe sort --source ./photos --dest ./archive     # explicit source
```

Only files directly inside the source directory are discovered. Subdirectories like `./photos/vacation/` are silently ignored.

#### Recursive Behavior

```bash
pixe sort --dest ./archive --recursive                    # source defaults to cwd
pixe sort --source ./photos --dest ./archive --recursive  # explicit source
```

All files in `./photos/` and all nested subdirectories (e.g., `./photos/vacation/IMG_0001.jpg`, `./photos/2024/trip/VID_0010.mp4`) are discovered and processed. The source directory structure has **no effect** on the destination structure — all files are organized into `dirB` by their capture date regardless of where they were found in the source tree.

#### File Identity in Recursive Mode

When recursive mode is enabled, files are identified by their **relative path from `dirA`** throughout the system:

- **Stdout output**: `COPY vacation/IMG_0001.jpg -> 2024/07-Jul/20240715_143022_abc123...jpg`
- **Ledger entries**: `"path": "vacation/IMG_0001.jpg"`
- **Database `source_path`**: Absolute path as always (e.g., `/Users/wells/photos/vacation/IMG_0001.jpg`)
- **Skip detection**: The archive database is queried by absolute `source_path`, so a file processed in a prior non-recursive run of the same `dirA` will be correctly skipped when a subsequent recursive run encounters it.

#### Incremental Recursive Runs

A common workflow is to first run Pixe non-recursively on a `dirA`, then later run it recursively on the same `dirA` to pick up files in subdirectories:

```bash
# First run: processes only top-level files (from cwd, or with explicit --source)
pixe sort --dest ./archive
pixe sort --source ./photos --dest ./archive

# Later run: processes everything, skipping already-imported top-level files
pixe sort --dest ./archive --recursive
pixe sort --source ./photos --dest ./archive --recursive
```

The second run will:
1. Discover all files (top-level + nested).
2. Skip top-level files already recorded in the archive database (stdout: `SKIP IMG_0001.jpg -> previously imported`).
3. Process newly discovered files from subdirectories.
4. Stream entries to the JSONL ledger at `dirA/.pixe_ledger.json` as each file is processed — both skipped and newly processed files appear as individual JSON lines, using relative paths.

#### Ledger Placement

Regardless of recursion depth, a **single ledger** is written at the root of `dirA` (`dirA/.pixe_ledger.json`). There is no per-subdirectory ledger. All file paths within the ledger use **relative paths from `dirA`** (e.g., `vacation/IMG_0001.jpg`, not the absolute path).

#### Ignore Patterns in Recursive Mode

The ignore list (Section 4.4) applies at every level of the directory tree. Patterns are matched against both the **filename** and the **relative path from `dirA`**. The hardcoded `.pixe_ledger.json` ignore matches by filename, so a ledger file at any depth (should one exist from a prior run targeting a subdirectory) is ignored.

### 4.10 Atomic Copy via Temp File

Pixe uses an **atomic copy pattern** to ensure that a file at its canonical path in `dirB` is always complete and verified. No partial or unverified files ever appear at canonical archive paths.

#### Write Flow

1. **Copy to temp file.** The source file is streamed to `<dest_dir>/.<filename>.pixe-tmp` using `os.O_CREATE|os.O_TRUNC`. The temp file lives in the same directory as the final destination to guarantee that `os.Rename` is an atomic same-filesystem operation.
2. **Verify the temp file.** The temp file is re-read through the handler's `HashableReader` and the checksum is recomputed. The result is compared against the source hash computed during the hashing stage.
3. **On verification success:** The temp file is atomically renamed to the canonical destination path via `os.Rename`. The rename is atomic on all supported filesystems (local, NFS, SMB) when source and destination are in the same directory.
4. **On verification failure:** The temp file is **deleted** immediately. The source file in `dirA` is untouched and can always be reprocessed. The DB row is updated to `status = 'mismatch'` with the error details. Stdout emits `ERR` with the mismatch information.

#### Temp File Naming

The temp file name follows the pattern `.<original_filename>.pixe-tmp-<random_suffix>`:

```
.<YYYYMMDD_HHMMSS_CHECKSUM.ext>.pixe-tmp-<random>
```

Example: A file destined for `2021/12-Dec/20211225_062223_7d97e98f...jpg` is first written to `2021/12-Dec/.20211225_062223_7d97e98f...jpg.pixe-tmp-abc123` (or similar unique suffix).

The leading dot makes temp files hidden on Unix systems. The `.pixe-tmp-<random>` suffix makes them unambiguously identifiable as Pixe artifacts, distinct from any media file. The random suffix is generated via `os.CreateTemp` to ensure that concurrent workers processing files with the same destination path do not overwrite each other's temp files.

**Note on `TempPath()` function:** The `copy.TempPath()` helper function returns the deterministic pattern `.<basename>.pixe-tmp` (without the random suffix). This is used by tests and for identifying orphaned temp files by prefix. The actual temp files created by `Execute()` use the unique `os.CreateTemp` pattern.

#### Interrupted Run Behavior

If the process is killed mid-copy or mid-verify, the temp file is left on disk. This is safe because:

- The canonical destination path **never exists** in a partial state. Any file at a canonical path has passed verification.
- The DB row for the interrupted file will be in a non-terminal state (`hashed` or earlier — the `copied` status is not set until the temp file is written, and `verified` is not set until after the rename).
- On **`pixe resume`**, the file is reprocessed. Because `Execute()` uses `os.CreateTemp` to generate a unique temp file name, the new copy creates a **new unique temp file** rather than overwriting the orphaned one. The orphan is left on disk but does not interfere with the new run.
- Over time, if orphaned temp files accumulate without a resume, `pixe clean` can scan for and remove them by suffix matching on `.pixe-tmp` (see Section 7.5). Orphaned temp files are self-healing via `pixe clean`.

#### Interaction with Cross-Process Dedup

The atomic copy pattern interacts cleanly with the existing cross-process dedup race handling (Section 8.5). When a race is detected at `CompleteFileWithDedupCheck`, the file has already been renamed to its canonical path (it passed verification). The race handler relocates it to `duplicates/` via `os.Rename` as before — this is still a same-filesystem atomic rename.

#### Tagging After Rename

Metadata tagging (Section 4.8) occurs **after** the rename to the canonical path. The file is verified and in its final location when tags are written. This preserves the existing behavior where the checksum reflects the original data, not the tagged version. If tagging fails, the file remains at its canonical path with `status = 'tag_failed'`.

---

## 5. Global Constraints

> [!IMPORTANT]
> ### 5.1 Operational Safety
> - **`dirA` is read-only.** Pixe never modifies, renames, moves, or deletes source files. The sole exception is writing a `.pixe_ledger.json` file into `dirA` to record what was processed.
> - **Atomic copy-then-verify.** Every file is first written to a temporary file (`.<filename>.pixe-tmp`) in the destination directory, then independently re-read and re-hashed. Only after verification passes is the temp file atomically renamed to its canonical path. A file at its canonical location in `dirB` is always verified. See Section 4.10.
> - **Database-backed resumability.** A SQLite database tracks per-file state across all runs. Interrupted runs resume from the last committed state. Each file completion is committed individually for crash safety.
> - **Streaming ledger in `dirA`.** A `.pixe_ledger.json` (JSONL format) is streamed to the source directory as files are processed. The header line includes a `run_id` linking back to the archive database. Each file entry is appended as an independent JSON line the moment the coordinator finalizes its result. An interrupted run leaves a partial but valid JSONL file — every line written before interruption is a complete, parseable JSON object.
> - **No silent outcomes.** Every discovered file produces exactly one line of stdout output (`COPY`, `SKIP`, `DUPE`, or `ERR`) and a corresponding ledger entry. Hash mismatches, copy failures, unrecognized files, duplicates, and skipped files are never silent. Pixe never exits without accounting for every file it saw.
> - **Concurrent-run safety.** Multiple `pixe sort` processes may target the same `dirB` simultaneously. The SQLite database uses WAL mode and busy-retry to ensure integrity without requiring external coordination.

> [!IMPORTANT]
> ### 5.2 Native Execution
> - **No external binary dependencies.** All metadata parsing, hashing, and file operations use pure Go packages or C libraries accessible via cgo only as a last resort.
> - **No `os/exec` calls** for core functionality. The binary must be self-contained.

> [!IMPORTANT]
> ### 5.3 Concurrency Model
> - Pixe uses a **worker pool** pattern for parallel file processing.
> - Worker count is **configurable** via `--workers` flag (default: sensible auto-detect based on `runtime.NumCPU()`).
> - Workers handle the full pipeline per file: extract → hash → copy (to temp file) → verify → rename → tag.
> - A **coordinator goroutine** manages database writes, deduplication queries, ledger appends, and progress reporting. The coordinator is the **sole writer** to both the archive database and the JSONL ledger file — workers never write to either directly.
> - `dirA` and `dirB` may reside on **different filesystems** (local, NAS, SMB). Pixe always uses copy (never `os.Rename` across filesystems).
> - **Cross-process concurrency:** Multiple `pixe sort` processes may target the same `dirB` from different sources. SQLite WAL mode permits concurrent reads with serialized writes. Each process operates within its own `run_id` context. Write contention is handled via `SQLITE_BUSY` retry with exponential backoff.

> [!IMPORTANT]
> ### 5.4 Scalability
> - Must handle from tens to hundreds of thousands of files in a single run, and cumulative archives of unbounded size.
> - Memory usage should be bounded — files are streamed, not loaded entirely into memory (except where format parsing requires it).
> - The deduplication index is persisted in the SQLite database with indexed lookups, eliminating the need to load all checksums into memory at startup. At 100K+ files, this replaces the prior approach of deserializing an entire JSON manifest to build an in-memory map.

---

## 6. Filetype Module Contract

New file types are added by implementing a Go interface. The core engine treats all files uniformly through this contract.

### 6.1 Interface Definition (Conceptual)

```go
// MetadataCapability declares how a handler supports metadata tagging.
type MetadataCapability int

const (
    // MetadataNone indicates the format cannot receive metadata at all.
    // The pipeline skips tagging entirely for this handler.
    MetadataNone MetadataCapability = iota

    // MetadataEmbed indicates the format supports safe in-file metadata writing.
    // The pipeline calls WriteMetadataTags to inject tags directly into the file.
    MetadataEmbed

    // MetadataSidecar indicates the format cannot safely embed metadata.
    // The pipeline writes an XMP sidecar file alongside the destination copy.
    MetadataSidecar
)

type FileTypeHandler interface {
    // Detect returns true if this handler can process the given file.
    // Detection is magic-byte verified after initial extension-based assumption.
    Detect(filePath string) (bool, error)

    // ExtractDate returns the capture date/time from the file's metadata.
    // Implementations define their own fallback chain per the global policy.
    ExtractDate(filePath string) (time.Time, error)

    // HashableReader returns an io.Reader over the media payload only,
    // excluding metadata. The core engine hashes this stream.
    HashableReader(filePath string) (io.Reader, error)

    // MetadataSupport declares this handler's metadata tagging capability.
    // The pipeline uses this to decide between embedded writes, XMP sidecar
    // generation, or skipping tagging entirely. See Section 4.8.1.
    MetadataSupport() MetadataCapability

    // WriteMetadataTags injects metadata tags directly into the file.
    // Only called when MetadataSupport() returns MetadataEmbed.
    // Must be a no-op when tags.IsEmpty() is true.
    WriteMetadataTags(filePath string, tags MetadataTags) error

    // Extensions returns the lowercase file extensions this handler claims
    // (used for initial fast-path detection before magic byte verification).
    Extensions() []string

    // MagicBytes returns the byte signatures used to verify file type.
    MagicBytes() []MagicSignature
}
```

**Interface changes from prior design:**

- **Added:** `MetadataSupport() MetadataCapability` — replaces the implicit "call `WriteMetadataTags` on everything and hope it does the right thing" pattern with an explicit capability query.
- **Narrowed:** `WriteMetadataTags` is now only called for `MetadataEmbed` handlers. Handlers returning `MetadataSidecar` or `MetadataNone` no longer need to implement `WriteMetadataTags` as a no-op stub — the pipeline never calls it. However, the method remains on the interface for compile-time safety; sidecar/none handlers should still implement it as a no-op that returns nil.
- **XMP sidecar writing** is handled by the pipeline (or a shared `xmp` package), not by individual handlers. The handler only declares its capability; the pipeline owns the sidecar generation logic.

### 6.2 Detection Strategy

1. **Extension-based assumption**: File extension is matched against registered handlers for a fast initial classification.
2. **Magic-byte verification**: The file header is read and compared against the handler's declared magic byte signatures.
3. If magic bytes **do not confirm** the extension-based assumption, the file is reclassified or flagged as unrecognized.

### 6.3 Filetype Modules

| Module | Extensions | Date Source | Hashable Region | `MetadataSupport()` | Tagging Strategy |
|---|---|---|---|---|---|
| **JPEG** | `.jpg`, `.jpeg` | EXIF `DateTimeOriginal` / `CreateDate` | Full image data (pixel payload) | `MetadataEmbed` | EXIF Copyright (IFD0), CameraOwnerName (ExifIFD 0xA430) written in-file |
| **HEIC** | `.heic`, `.heif` | EXIF `DateTimeOriginal` / `CreateDate` | Image data payload | `MetadataSidecar` | XMP sidecar (`*.heic.xmp`) |
| **MP4** | `.mp4`, `.mov` | QuickTime `CreationDate` / `mvhd` atom | Collected keyframe data | `MetadataSidecar` | XMP sidecar (`*.mp4.xmp`, `*.mov.xmp`) |
| **DNG** | `.dng` | EXIF `DateTimeOriginal` / `CreateDate` | Raw sensor data strips | `MetadataSidecar` | XMP sidecar (`*.dng.xmp`) |
| **NEF** | `.nef` | EXIF `DateTimeOriginal` / `CreateDate` | Raw sensor data strips | `MetadataSidecar` | XMP sidecar (`*.nef.xmp`) |
| **CR2** | `.cr2` | EXIF `DateTimeOriginal` / `CreateDate` | Raw sensor data strips | `MetadataSidecar` | XMP sidecar (`*.cr2.xmp`) |
| **CR3** | `.cr3` | ISOBMFF container metadata (same approach as HEIC/MP4) | Raw sensor data (ISOBMFF `mdat`) | `MetadataSidecar` | XMP sidecar (`*.cr3.xmp`) |
| **PEF** | `.pef` | EXIF `DateTimeOriginal` / `CreateDate` | Raw sensor data strips | `MetadataSidecar` | XMP sidecar (`*.pef.xmp`) |
| **ARW** | `.arw` | EXIF `DateTimeOriginal` / `CreateDate` | Raw sensor data strips | `MetadataSidecar` | XMP sidecar (`*.arw.xmp`) |

### 6.4 RAW Handler Architecture

RAW image support follows a **shared base + thin wrapper** pattern. Six RAW formats are supported: DNG, NEF (Nikon), CR2 (Canon), CR3 (Canon), PEF (Pentax), and ARW (Sony). CRW (legacy Canon pre-2004) is explicitly out of scope due to its obsolete format and lack of pure-Go library support.

#### 6.4.1 Why a Shared Base

Five of the six supported RAW formats — DNG, NEF, CR2, PEF, and ARW — are TIFF-based containers with standard EXIF IFDs. They share identical logic for date extraction, hashable region identification, and metadata write behavior. Duplicating this logic across five separate packages would be wasteful and error-prone. Instead, a shared `tiffraw` base package provides the common implementation, and each format supplies only its unique identity: extensions, magic bytes, and detection logic.

CR3 is the exception. It uses an ISOBMFF container (like HEIC and MP4) rather than TIFF. It gets its own standalone handler following the ISOBMFF extraction approach already established by the HEIC handler.

#### 6.4.2 Package Layout

```
internal/handler/
├── tiffraw/              ← shared base for TIFF-based RAW formats
│   └── tiffraw.go        ← Base struct with common ExtractDate, HashableReader, WriteMetadataTags
├── dng/
│   ├── dng.go            ← thin wrapper: Extensions, MagicBytes, Detect → delegates to tiffraw.Base
│   └── dng_test.go
├── nef/
│   ├── nef.go
│   └── nef_test.go
├── cr2/
│   ├── cr2.go
│   └── cr2_test.go
├── cr3/
│   ├── cr3.go            ← standalone ISOBMFF-based handler (not using tiffraw)
│   └── cr3_test.go
├── pef/
│   ├── pef.go
│   └── pef_test.go
└── arw/
    ├── arw.go
    └── arw_test.go
```

#### 6.4.3 Shared Base: `tiffraw.Base`

The `tiffraw` package provides a `Base` struct that implements the three format-agnostic methods of the `FileTypeHandler` interface:

```go
package tiffraw

// Base provides shared logic for TIFF-based RAW formats.
// Per-format handlers embed this struct and supply their own
// Extensions(), MagicBytes(), and Detect() methods.
type Base struct{}

func (b *Base) ExtractDate(filePath string) (time.Time, error)          { /* EXIF parsing */ }
func (b *Base) HashableReader(filePath string) (io.ReadCloser, error)   { /* JPEG preview extraction */ }
func (b *Base) WriteMetadataTags(filePath string, tags MetadataTags) error { /* no-op stub */ }
```

Each per-format handler (e.g., `dng.Handler`) embeds `tiffraw.Base` and adds only:
- `Extensions()` — returns the format-specific extension(s)
- `MagicBytes()` — returns the format-specific magic byte signature(s)
- `Detect()` — extension check + magic byte verification

This is standard Go composition via embedding — no inheritance, no interface gymnastics.

#### 6.4.4 Date Extraction (TIFF-based RAW)

All TIFF-based RAW formats store standard EXIF metadata in IFD0 and sub-IFDs. Date extraction follows the same fallback chain used by JPEG:

1. **EXIF `DateTimeOriginal`** (tag 0x9003) — preferred
2. **EXIF `DateTime`** (tag 0x0132, IFD0) — fallback
3. **Ansel Adams date** (`1902-02-20`) — sentinel for undated files

The TIFF container is parsed to locate the EXIF IFDs, then standard EXIF tag reading applies. A pure-Go TIFF parser (e.g., `golang.org/x/image/tiff` or equivalent) provides the IFD traversal.

#### 6.4.5 Date Extraction (CR3)

CR3 files use the ISOBMFF container format, the same box-based structure used by HEIC and MP4. Date extraction follows the ISOBMFF approach already established by the HEIC handler:

1. Parse the ISOBMFF container to locate the EXIF blob (typically within a `moov` → `meta` → `xml ` or `Exif` box path, depending on the Canon implementation).
2. Extract the raw EXIF bytes from the container.
3. Parse with the standard EXIF library and apply the same fallback chain: `DateTimeOriginal` → `DateTime` → Ansel Adams date.

#### 6.4.6 Hashable Region: Raw Sensor Data

All supported RAW formats contain the actual sensor data payload — the irreducible content that distinguishes one exposure from another. The `HashableReader()` method extracts this sensor data and returns it as the hashable region.

**Why the raw sensor data (not the embedded JPEG preview)?**

RAW files embed a full-resolution JPEG preview that could serve as a faster hashing proxy. However, the JPEG preview is not a reliable stand-in for the sensor data:

- **Preview instability.** Software tools (Lightroom, Capture One, camera firmware updates) may regenerate or modify the embedded JPEG preview without touching the sensor data. Two copies of the same RAW file processed by different tools would produce different JPEG preview hashes, causing Pixe to treat identical sensor data as unique files — a false negative that wastes archive space with undetectable duplicates.
- **Preview ambiguity.** In rare cases, different exposures (e.g., burst shots at the same instant) could produce byte-identical JPEG previews despite containing different sensor data — a false positive. While Pixe fails safely in this case (the "duplicate" is still preserved), it misrepresents the archive's contents.
- **Sensor data is the ground truth.** The raw sensor data is the immutable, authoritative content of a RAW file. It is never modified by post-processing tools, metadata editors, or firmware updates. Hashing the sensor data produces a checksum that is a true content fingerprint — stable across software workflows and deterministic across copies of the same file regardless of how the embedded preview was generated.
- **Safety over speed.** Hashing sensor data requires reading more bytes than hashing the JPEG preview (the sensor payload is typically 20-80 MB vs. 1-5 MB for the preview). This trade-off is accepted: RAW file users expect processing overhead proportional to file size, and data integrity is Pixe's first principle. A false dedup decision is far more costly than the I/O time to read the sensor data.
- If the sensor data cannot be located or extracted, the handler falls back to hashing the full file (same safety-first approach as other handlers).

**Extraction approach for TIFF-based formats (DNG, NEF, CR2, PEF, ARW):**

The raw sensor data is stored in a primary IFD (typically IFD0 or a sub-IFD) with a proprietary or format-specific compression scheme (uncompressed, lossless JPEG, Huffman, or vendor-specific). The handler navigates the TIFF IFD chain to locate the sensor data IFD — distinguished from the JPEG preview IFD by its compression type (not JPEG compression `6`) and typically by `NewSubfileType = 0` (full-resolution image). Once identified, the handler reads the `StripOffsets` and `StripByteCounts` (or `TileOffsets` and `TileByteCounts` for tiled formats) and returns a reader that streams the raw sensor bytes from those regions.

**Distinguishing sensor data from JPEG preview in the IFD chain:**

TIFF-based RAW files typically contain multiple IFDs, some holding JPEG preview images and one holding the actual sensor data. The sensor data IFD is identified by:

1. **Compression tag:** The sensor data uses a non-JPEG compression value. Common values include `1` (uncompressed), `7` (lossless JPEG, used by many Nikon NEF and Canon CR2 files), `34713` (Nikon NEF compressed), `34892` (lossy JPEG, used by some DNG files), or other vendor-specific values. JPEG preview IFDs use compression `6` (standard JPEG) or `7` in a thumbnail context.
2. **Image dimensions:** The sensor data IFD contains the full-resolution image dimensions (`ImageWidth` × `ImageLength`), which are significantly larger than any preview or thumbnail IFD.
3. **`NewSubfileType` tag:** When present, a value of `0` indicates the primary (full-resolution) image. Preview IFDs typically use `1` (reduced-resolution) or are in IFD1 (the traditional thumbnail IFD).

The handler uses these signals in combination to reliably select the sensor data IFD. If ambiguity remains, the handler selects the IFD with the largest data payload (by total `StripByteCounts` or `TileByteCounts`).

**Extraction approach for CR3:**

CR3 files use the ISOBMFF container format. The raw sensor data is stored in the `mdat` (media data) box, referenced by track metadata in the `moov` box. The handler navigates the ISOBMFF box structure to locate the sensor data track (typically the primary image item or the largest `trak` entry), determines the byte offset and length of the sensor data within `mdat`, and returns a reader over that region. If the sensor data track cannot be isolated, the handler falls back to hashing the full `mdat` box contents.

#### 6.4.7 Metadata Capability: XMP Sidecar

All RAW handlers declare `MetadataSidecar` as their metadata capability. Writing metadata into proprietary RAW containers risks corruption of archival originals and is not reliably possible in pure Go. Instead, the pipeline writes an XMP sidecar file alongside the destination copy (see Section 4.8.2).

**Shared base implementation (`tiffraw.Base`):**

```go
func (b *Base) MetadataSupport() domain.MetadataCapability {
    return domain.MetadataSidecar
}

func (b *Base) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
    // Never called — pipeline checks MetadataSupport() first.
    // Retained as no-op for interface compliance.
    return nil
}
```

The CR3 handler (standalone, not using `tiffraw.Base`) implements the same pattern independently.

**What changed:** Previously, all non-JPEG handlers had no-op `WriteMetadataTags` stubs and the pipeline would call them unconditionally, silently doing nothing. The database would record these files as "tagged" even though no metadata was written. With the new design, the pipeline checks `MetadataSupport()` and routes sidecar-capable handlers to XMP generation. The database status accurately reflects whether metadata was actually persisted.

#### 6.4.8 Magic Byte Signatures

Each RAW format has a distinct file header that enables reliable magic-byte detection:

| Format | Magic Bytes | Offset | Notes |
|---|---|---|---|
| **DNG** | `49 49 2A 00` or `4D 4D 00 2A` | 0 | TIFF little-endian or big-endian header (same as TIFF; DNG is distinguished by the presence of a DNGVersion tag in IFD0 — detection must check beyond magic bytes) |
| **NEF** | `49 49 2A 00` | 0 | TIFF LE header; Nikon-specific maker note IFDs distinguish from generic TIFF. Extension `.nef` is the primary discriminator. |
| **CR2** | `49 49 2A 00` + `43 52` at offset 8 | 0 | TIFF LE header with `CR` signature bytes at offset 8–9 |
| **CR3** | `66 74 79 70` ("ftyp") | 4 | ISOBMFF container; `ftyp` brand is `crx ` (Canon RAW X) |
| **PEF** | `49 49 2A 00` | 0 | TIFF LE header; Pentax-specific. Extension `.pef` is the primary discriminator. |
| **ARW** | `49 49 2A 00` | 0 | TIFF LE header; Sony-specific. Extension `.arw` is the primary discriminator. |

> **Important:** Several TIFF-based RAW formats share the same TIFF magic bytes (`49 49 2A 00`). For these formats, the **extension-based fast path** in the registry is the primary discriminator, with magic bytes serving only to confirm the file is a valid TIFF container. CR2 is the notable exception — it has additional signature bytes at offset 8 that uniquely identify it. The registry's two-phase detection (extension first, then magic byte verification) handles this gracefully.

#### 6.4.9 Handler Registration

All RAW handlers are registered in the same three locations as existing handlers (`cmd/sort.go`, `cmd/verify.go`, `cmd/resume.go`):

```go
reg.Register(dnghandler.New())
reg.Register(nefhandler.New())
reg.Register(cr2handler.New())
reg.Register(cr3handler.New())
reg.Register(pefhandler.New())
reg.Register(arwhandler.New())
```

Registration order matters for the TIFF-based formats that share magic bytes. Since the extension-based fast path resolves first, registration order only affects the fallback path (files with mismatched extensions). JPEG must remain registered before the TIFF-based RAW handlers to avoid false matches on TIFF magic bytes.

---

## 7. CLI Structure

Built with `spf13/cobra`. Configuration layered via `spf13/viper` (flags > config file > defaults).

### 7.1 Commands

```
pixe sort     [--source <dirA>] --dest <dirB> [options]
pixe verify   --dir <dirB>
pixe resume   --dir <dirB>
pixe query    <subcommand> --dir <dirB> [options]
pixe status   [--source <dirA>] [options]
pixe clean    --dir <dirB> [options]
pixe version
```

#### `pixe sort`
Primary operation. Discovers files in `dirA`, processes them through the pipeline, and writes organized output to `dirB`. When `--source` is omitted, the current working directory is used as `dirA`.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--source` | `-s` | cwd | Source directory (read-only). Defaults to the current working directory; flag overrides. |
| `--dest` | `-d` | (required) | Destination directory |
| `--workers` | | auto | Number of concurrent workers |
| `--algorithm` | | `sha1` | Hash algorithm (`sha1`, `sha256`, etc.) |
| `--copyright` | | (none) | Copyright string template. `{{.Year}}` supported. |
| `--camera-owner` | | (none) | CameraOwner freetext string |
| `--dry-run` | | `false` | Preview operations without copying |
| `--recursive` | `-r` | `false` | Recursively process subdirectories of `--source` |
| `--skip-duplicates` | | `false` | Skip copying files whose checksum matches an already-archived file. Emits `DUPE` on stdout and records in DB/ledger, but writes no bytes to `dirB`. See Section 4.6. |
| `--ignore` | | (none) | Glob pattern for files to ignore. Repeatable: each `--ignore` adds one pattern. Merged with patterns from config file. |

#### `pixe verify`
Walks a previously sorted `dirB`, parses checksums from filenames, recomputes data-only hashes, and reports mismatches.

| Flag | Default | Description |
|---|---|---|
| `--dir` | (required) | Directory to verify |
| `--algorithm` | `sha1` | Hash algorithm (must match what was used during sort) |
| `--workers` | auto | Number of concurrent workers |

#### `pixe resume`
Locates the archive database for `dirB` (via `--db-path`, `dbpath` marker, or default local path) and resumes an interrupted sort operation by reprocessing files in non-terminal states.

| Flag | Default | Description |
|---|---|---|
| `--dir` | (required) | Destination directory associated with the archive database |
| `--db-path` | (auto-detected) | Explicit path to the SQLite database file |

#### `pixe version`
Prints the version, git commit, and build date in a single human-readable line, then exits. Implemented as a standard Cobra subcommand in `cmd/version.go`.

**Output format:**

```
pixe v0.10.0 (commit: abc1234, built: 2026-03-06T10:30:00Z)
```

No flags. This command calls `fullVersion()` (package-private in `cmd`) and prints to stdout. The version variables are injected at build time via ldflags (see Section 3).

#### `pixe status`
Shows the sorting status of a source directory by comparing the files currently on disk against the ledger left by prior `pixe sort` runs. This is a **source-oriented** command — it answers "what in this folder still needs sorting?" without requiring access to any destination archive or database. When `--source` is omitted, the current working directory is inspected. See **Section 7.4** for full design details.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--source` | `-s` | cwd | Source directory to inspect. Defaults to the current working directory; flag overrides. |
| `--recursive` | `-r` | `false` | Recursively inspect subdirectories of `--source` |
| `--ignore` | | (none) | Glob pattern for files to ignore. Repeatable. Merged with config file patterns. |
| `--json` | | `false` | Emit output as JSON instead of human-readable table |

#### `pixe clean`
Maintenance command for a destination archive. Removes orphaned temp files (`.pixe-tmp`) and XMP sidecars left by interrupted runs, and optionally compacts the archive database via VACUUM. See **Section 7.5** for full design details.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--dir` | `-d` | (required) | Destination directory (`dirB`) to clean. |
| `--db-path` | | (auto-detected) | Explicit path to the SQLite database file. |
| `--dry-run` | | `false` | Preview what would be cleaned without modifying anything. |
| `--temp-only` | | `false` | Only clean orphaned files. Skip database compaction. |
| `--vacuum-only` | | `false` | Only compact the database. Skip file scanning. Mutually exclusive with `--temp-only`. |

#### `pixe query <subcommand>`
Read-only interrogation of the archive database. Exposes the query patterns described in Section 8.4 as user-facing subcommands. No files are modified — this is purely a reporting tool.

**Parent-level flags** (inherited by all subcommands):

| Flag | Default | Description |
|---|---|---|
| `--dir` | (required) | Destination directory (`dirB`) associated with the archive database |
| `--db-path` | (auto-detected) | Explicit path to the SQLite database file. Overrides automatic location logic. |
| `--json` | `false` | Emit output as a JSON array instead of human-readable table. See Section 7.3.3. |

Database discovery follows the same priority chain as `sort` and `resume`: `--db-path` flag → `dirB/.pixe/dbpath` marker → `dirB/.pixe/pixe.db`.

See **Section 7.3** for full subcommand details.

### 7.2 Configuration File

Viper supports a `.pixe.yaml` (or `.pixe.toml`, `.pixe.json`) configuration file for persistent defaults:

```yaml
algorithm: sha1
workers: 8
copyright: "Copyright {{.Year}} My Family, all rights reserved"
camera_owner: "Wells Family"
recursive: false
skip_duplicates: false
ignore:
  - "*.txt"
  - ".DS_Store"
  - "Thumbs.db"
```

The `ignore` key is a list of glob patterns. Patterns from the config file are merged with any `--ignore` CLI flags (additive). The hardcoded `.pixe_ledger.json` ignore is always active regardless of config.

### 7.3 Query Command

`pixe query` is a **read-only** command group that exposes the archive database to the end user. It uses Cobra's nested subcommand pattern — each query type is its own subcommand with its own flags, allowing parameter shapes to diverge naturally as the query surface grows.

#### 7.3.1 Subcommands

##### `pixe query runs`

Lists all sort runs recorded in the archive database, with file counts.

```bash
pixe query runs --dir ./archive
```

| Flag | Default | Description |
|---|---|---|
| (none beyond parent) | | |

**Output columns:** Run ID (truncated to 8 chars in table mode), Pixe version, source directory, started at, finished at, status, file count.

**DB method:** `ListRuns()` → `[]*RunSummary`

**Human-readable example:**

```
RUN ID    VERSION  SOURCE              STARTED              STATUS      FILES
a1b2c3d4  0.10.0   /Users/wells/photos 2026-03-06 10:30:00  completed   1,247
e5f6a7b8  0.10.0   /Users/wells/dcim   2026-03-05 14:15:00  completed     384
c9d0e1f2  0.9.0    /Users/wells/photos 2026-02-28 09:00:00  interrupted   892

3 runs | 2,523 total files
```

##### `pixe query run <id>`

Shows all files processed in a specific run.

```bash
pixe query run a1b2c3d4 --dir ./archive
```

| Flag | Default | Description |
|---|---|---|
| (positional) | (required) | Run ID. Supports prefix matching — if `a1b2c3d4` uniquely identifies a run, the full UUID is not required. |

**Output columns:** Source filename, status, destination (relative), checksum (truncated), capture date.

**DB method:** `GetFilesByRun(runID)` → `[]*FileRecord`

**Prefix matching:** The run ID argument is matched against the `runs.id` column using a `LIKE ?%` prefix query. If exactly one run matches, that run is used. If zero or multiple runs match, the command exits with an error listing the ambiguous matches. This allows users to type `pixe query run a1b2` instead of the full UUID.

**Human-readable example:**

```
Run a1b2c3d4-e5f6-7890-abcd-ef1234567890
  Version:  0.10.0
  Source:   /Users/wells/photos
  Started:  2026-03-06 10:30:00 UTC
  Finished: 2026-03-06 10:42:15 UTC
  Status:   completed

SOURCE FILE          STATUS    DESTINATION                                          CAPTURE DATE
IMG_0001.jpg         complete  2021/12-Dec/20211225_062223_7d97e98f...jpg            2021-12-25
IMG_0002.jpg         duplicate duplicates/20260306_103000/2022/02-Feb/20220202...jpg 2022-02-02
notes.txt            skipped   —                                                    —
corrupt.jpg          failed    —                                                    —

1,247 files | 1,180 complete | 42 duplicates | 15 skipped | 10 errors
```

##### `pixe query duplicates`

Lists all files flagged as duplicates across all runs.

```bash
pixe query duplicates --dir ./archive
pixe query duplicates --dir ./archive --pairs
```

| Flag | Default | Description |
|---|---|---|
| `--pairs` | `false` | Show each duplicate alongside the original it matches (joined by checksum). |

**Without `--pairs`:**

**DB method:** `AllDuplicates()` → `[]*FileRecord`

**Output columns:** Source path, destination (in `duplicates/`), checksum (truncated), capture date.

**With `--pairs`:**

**DB method:** `DuplicatePairs()` → `[]*DuplicatePair`

**Output columns:** Duplicate source path, duplicate destination, original destination.

**Human-readable example (`--pairs`):**

```
DUPLICATE SOURCE                DUPLICATE DEST                                       ORIGINAL
/Users/wells/photos/IMG_0002.jpg duplicates/20260306.../2022/02-Feb/20220202...jpg   2022/02-Feb/20220202_123101_447d3060...jpg
/Users/wells/dcim/DSC_4521.jpg   duplicates/20260305.../2024/07-Jul/20240715...jpg   2024/07-Jul/20240715_143022_abc12345...jpg

2 duplicate pairs
```

##### `pixe query errors`

Lists all files in error states (`failed`, `mismatch`, `tag_failed`) across all runs.

```bash
pixe query errors --dir ./archive
```

| Flag | Default | Description |
|---|---|---|
| (none beyond parent) | | |

**DB method:** `FilesWithErrors()` → `[]*FileWithSource`

**Output columns:** Source path, status, error message, run source directory.

**Human-readable example:**

```
SOURCE PATH                       STATUS      ERROR                                    RUN SOURCE
/Users/wells/photos/corrupt.jpg   failed      EXIF parse failed: truncated IFD         /Users/wells/photos
/Users/wells/dcim/DSC_9999.jpg    mismatch    expected abc123, got def456               /Users/wells/dcim
/Users/wells/photos/IMG_5000.jpg  tag_failed  permission denied: write metadata         /Users/wells/photos

3 errors | 1 failed | 1 mismatch | 1 tag_failed
```

##### `pixe query skipped`

Lists all files that were skipped (previously imported or unsupported format) across all runs.

```bash
pixe query skipped --dir ./archive
```

| Flag | Default | Description |
|---|---|---|
| (none beyond parent) | | |

**DB method:** New method `AllSkipped()` → `[]*FileRecord` (query: `SELECT ... FROM files WHERE status = 'skipped' ORDER BY id`). Follows the same pattern as `AllDuplicates()`.

**Output columns:** Source path, skip reason.

**Human-readable example:**

```
SOURCE PATH                        REASON
/Users/wells/photos/notes.txt      unsupported format: .txt
/Users/wells/photos/.DS_Store      unsupported format: .DS_Store
/Users/wells/photos/IMG_0001.jpg   previously imported

3 skipped files | 2 unsupported format | 1 previously imported
```

##### `pixe query files`

Flexible file search with date-range and source-directory filters. At least one filter flag is required.

```bash
pixe query files --dir ./archive --from 2024-01-01 --to 2024-12-31
pixe query files --dir ./archive --imported-from 2026-03-01 --imported-to 2026-03-07
pixe query files --dir ./archive --source /Users/wells/photos
```

| Flag | Default | Description |
|---|---|---|
| `--from` | (none) | Start of capture date range (inclusive). Format: `YYYY-MM-DD`. |
| `--to` | (none) | End of capture date range (inclusive). Format: `YYYY-MM-DD`. |
| `--imported-from` | (none) | Start of import/verification date range (inclusive). Format: `YYYY-MM-DD`. |
| `--imported-to` | (none) | End of import/verification date range (inclusive). Format: `YYYY-MM-DD`. |
| `--source` | (none) | Filter to files imported from this source directory (absolute path). |

**Validation:** At least one filter flag must be provided. The `--from`/`--to` pair and `--imported-from`/`--imported-to` pair are mutually exclusive with each other — a single invocation queries by capture date range, import date range, or source directory, but not multiple at once. If only `--from` is provided (no `--to`), the range extends to the present. If only `--to` is provided (no `--from`), the range starts from the earliest record.

**DB methods:**
- `--from`/`--to` → `FilesByCaptureDateRange(start, end)` → `[]*FileRecord`
- `--imported-from`/`--imported-to` → `FilesByImportDateRange(start, end)` → `[]*FileRecord`
- `--source` → `FilesBySource(sourceDir)` → `[]*FileRecord`

**Output columns:** Source path, destination (relative), checksum (truncated), capture date, status.

**Human-readable example:**

```
SOURCE PATH                        DESTINATION                                    CHECKSUM   CAPTURE DATE  STATUS
/Users/wells/photos/IMG_3001.jpg   2024/07-Jul/20240715_143022_abc12345...jpg     abc12345   2024-07-15    complete
/Users/wells/photos/IMG_3002.jpg   2024/07-Jul/20240715_150100_def67890...jpg     def67890   2024-07-15    complete
/Users/wells/photos/VID_0050.mp4   2024/08-Aug/20240820_091500_1a2b3c4d...mp4     1a2b3c4d   2024-08-20    complete

3 files | capture range: 2024-07-15 to 2024-08-20
```

##### `pixe query inventory`

Lists the canonical archive contents — all complete, non-duplicate files. This is the "what does my archive actually contain?" view.

```bash
pixe query inventory --dir ./archive
```

| Flag | Default | Description |
|---|---|---|
| (none beyond parent) | | |

**DB method:** `ArchiveInventory()` → `[]*InventoryEntry`

**Output columns:** Destination path (relative), checksum, capture date.

**Human-readable example:**

```
DESTINATION                                          CHECKSUM                                  CAPTURE DATE
2021/12-Dec/20211225_062223_7d97e98f8af710c7...jpg   7d97e98f8af710c7e7fe703abc8f639e0ee507c4  2021-12-25
2022/02-Feb/20220202_123101_447d3060abc12345...jpg   447d3060abc1234567890abcdef1234567890abcd  2022-02-02
2024/07-Jul/20240715_143022_abc1234567890abc...jpg   abc1234567890abcdef1234567890abcdef123456  2024-07-15

8,421 files | capture range: 2021-12-25 to 2024-08-20 | total size: 142.3 GB
```

#### 7.3.2 Human-Readable Output

The default output mode is a **fixed-width columnar table** suitable for terminal viewing. Design principles:

- **Column headers** are uppercase, left-aligned.
- **Long values** are truncated with `...` to fit terminal width. Checksums are truncated to 8 hex characters in table mode (the full checksum is available via `--json`). File paths are truncated from the left (showing the most relevant suffix).
- **Summary line** at the bottom of every query result. The summary provides aggregate counts and any relevant statistics (total files, breakdowns by status, date ranges, total size where applicable). The summary is separated from the table by a blank line.
- **Empty results** produce a single line: `No <items> found.` (e.g., `No duplicates found.`, `No errors found.`).
- **Run ID truncation** in table mode: UUIDs are shown as the first 8 characters. The full UUID is available via `--json` or `pixe query run <prefix>`.

#### 7.3.3 JSON Output

When `--json` is passed, the output is a single **JSON object** (not JSONL) written to stdout. This is designed for piping to `jq`, scripting, and programmatic consumption.

**Structure:**

```json
{
  "query": "<subcommand name>",
  "dir": "/absolute/path/to/dirB",
  "results": [ ... ],
  "summary": {
    "total": 42,
    ...
  }
}
```

- **`query`** — the subcommand name (e.g., `"runs"`, `"duplicates"`, `"errors"`).
- **`dir`** — the resolved `dirB` path.
- **`results`** — a JSON array of result objects. Each object contains the full, untruncated data for every field (full UUIDs, full checksums, full paths). Field names use `snake_case` matching the database column names.
- **`summary`** — the same aggregate statistics shown in the human-readable summary line, structured as a JSON object.

**JSON uses `omitempty` semantics** — null/empty fields are omitted from result objects, consistent with the ledger format convention.

**Example (`pixe query errors --dir ./archive --json`):**

```json
{
  "query": "errors",
  "dir": "/Users/wells/archive",
  "results": [
    {
      "source_path": "/Users/wells/photos/corrupt.jpg",
      "status": "failed",
      "error": "EXIF parse failed: truncated IFD at offset 0x1A",
      "run_source": "/Users/wells/photos"
    }
  ],
  "summary": {
    "total": 1,
    "failed": 1,
    "mismatch": 0,
    "tag_failed": 0
  }
}
```

#### 7.3.4 Database Interaction

`pixe query` opens the database in **read-only mode**. No writes are performed — no run records, no file records, no schema modifications. The database is opened, queried, and closed.

If the database does not exist at the resolved path, the command exits with a clear error: `Error: no archive database found for <dirB>. Run 'pixe sort' first to create one.`

#### 7.3.5 New Database Methods Required

The existing `queries.go` in `internal/archivedb/` provides most of the needed methods. The following additions are required:

| Method | Query | Purpose |
|---|---|---|
| `AllSkipped()` | `SELECT source_path, skip_reason, ... FROM files WHERE status = 'skipped' ORDER BY id` | Backing query for `pixe query skipped` |
| `GetRunByPrefix(prefix)` | `SELECT ... FROM runs WHERE id LIKE ? ORDER BY started_at DESC` | Prefix-match lookup for `pixe query run <id>` |
| `ArchiveStats()` | Aggregate query: total files, total duplicates, total errors, total skipped, total size, date range, run count | Backing query for summary statistics (used by all subcommands that need aggregate data beyond a simple `len(results)`) |

`AllSkipped()` follows the exact pattern of `AllDuplicates()` — same scan logic, different WHERE clause.

`GetRunByPrefix()` returns `([]*Run, error)` — the caller checks `len(results)` for 0 (not found) or >1 (ambiguous) and formats the appropriate error message. This keeps the ambiguity logic in the CLI layer, not the database layer.

`ArchiveStats()` is a new aggregate query that returns a stats struct. It powers the summary lines across subcommands and could eventually back a standalone `pixe query stats` command if desired.

#### 7.3.6 Package Layout

```
cmd/
├── query.go              ← parent `pixe query` command, defines --dir, --db-path, --json
├── query_runs.go         ← `pixe query runs` subcommand
├── query_run.go          ← `pixe query run <id>` subcommand
├── query_duplicates.go   ← `pixe query duplicates` subcommand
├── query_errors.go       ← `pixe query errors` subcommand
├── query_skipped.go      ← `pixe query skipped` subcommand
├── query_files.go        ← `pixe query files` subcommand
├── query_inventory.go    ← `pixe query inventory` subcommand
```

Each subcommand file follows the existing Cobra pattern established by `sort.go`, `verify.go`, and `resume.go`. The parent command (`query.go`) handles:

1. Database discovery and opening (shared `PersistentPreRunE` on the parent command).
2. Passing the `*archivedb.DB` handle to subcommands via Cobra's context or a package-level variable (same pattern used by `resume.go`).
3. The `--json` flag (read by subcommands to choose output formatting).

Subcommands are responsible for:

1. Defining their own flags.
2. Calling the appropriate `archivedb` method.
3. Formatting output (table or JSON) based on the `--json` flag.

A shared output formatting helper (e.g., `queryOutput(results, summary, jsonFlag)`) may be extracted if the pattern proves repetitive across subcommands, but this is an implementation detail — each subcommand can start with its own formatting and refactor later.

### 7.4 Status Command

`pixe status` is a **source-oriented, read-only** command that reports the sorting status of a source directory (`dirA`). When `--source` is omitted, it defaults to the current working directory — the most natural invocation is simply `pixe status` from within a folder of photos. It compares the files currently on disk against the JSONL ledger (`dirA/.pixe_ledger.json`) left by prior `pixe sort` runs, and produces a categorized listing of what has been sorted, what hasn't, and what Pixe can't handle.

#### 7.4.1 Core Concept: Ledger-Only, No Database Required

Unlike `pixe query` (which requires access to the archive database in `dirB`), `pixe status` operates **entirely from `dirA`**. Its only data source beyond the filesystem is the `.pixe_ledger.json` file. This design has several advantages:

- **Works offline from the archive.** The destination drive (NAS, external disk) does not need to be mounted.
- **No database dependency.** No SQLite, no `--db-path`, no `--dest` flag.
- **Fast.** Parsing a JSONL ledger is trivial compared to opening a database and running queries.
- **Answers the natural question.** When a user looks at a folder of photos, they want to know: "Have I sorted these yet?" The answer is in the ledger, right next to the photos.

If no ledger exists in `dirA`, every discovered file is reported as unsorted. The command emits a notice: `No ledger found — no prior sort runs recorded for this directory.`

#### 7.4.2 Algorithm

1. **Walk `dirA`** using the same `discovery.Walk()` function used by `pixe sort`, with the same `WalkOptions` (recursive flag, ignore matcher). This produces two slices: `discovered` (files with a matched handler) and `skipped` (unrecognized files — dotfiles, unsupported formats, detection errors).

2. **Load the ledger** via `manifest.LoadLedger(dirA)`. Parse all `LedgerEntry` objects into a map keyed by `path` (the relative path from `dirA`). If the ledger does not exist, the map is empty.

3. **Classify each discovered file** by looking up its relative path in the ledger map:
   - **Sorted** — The ledger contains an entry with `status: "copy"` for this path. The file was successfully copied to the archive.
   - **Duplicate** — The ledger contains an entry with `status: "duplicate"`. The file's content already existed in the archive.
   - **Errored** — The ledger contains an entry with `status: "error"`. The file was attempted but failed.
   - **Unsorted** — No ledger entry exists for this path, or the ledger entry has `status: "skip"` with reason `"previously imported"` from a different context. Practically: any file not in the ledger is unsorted.

4. **Classify each skipped file** from the discovery walk:
   - **Unrecognized** — Files that no handler claims (unsupported extension, magic byte mismatch). These are reported separately so the user knows they exist but Pixe cannot process them.

5. **Produce output** — categorized file listing with a summary line.

#### 7.4.3 Ledger Multi-Run Handling

The ledger file is **truncated** at the start of each `pixe sort` run (see Section 8.8). This means the ledger always reflects the **most recent run only**. Consequently:

- If a user runs `pixe sort` on a subset of `dirA` (e.g., non-recursive), then adds files and runs `pixe status --recursive`, the ledger will only contain entries from the last run. Files sorted in earlier runs (whose entries were overwritten) will appear as "unsorted."
- This is an acceptable trade-off for the ledger-only design. The ledger is a receipt of the last run, not a cumulative history. For cumulative history, `pixe query files --source <dirA>` queries the archive database.

**Mitigation:** The status output includes a header line showing the ledger's run metadata (run ID, timestamp, whether recursive was enabled) so the user understands the context of the status report.

#### 7.4.4 Output Format

**Default: human-readable listing**

The output is organized into sections, each with a header. Sections with zero files are omitted.

```
Source: /Users/wells/photos
Ledger: run a1b2c3d4, 2026-03-06 10:30:00 UTC (recursive: no)

SORTED (247 files)
  IMG_0001.jpg         → 2021/12-Dec/20211225_062223_7d97e98f...jpg
  IMG_0002.jpg         → 2022/02-Feb/20220202_123101_447d3060...jpg
  VID_0010.mp4         → 2022/03-Mar/20220316_232122_321c7d6f...mp4

DUPLICATE (3 files)
  IMG_0042.jpg         → matches 2022/02-Feb/20220202_123101_447d3060...jpg
  IMG_0099.jpg         → matches 2024/07-Jul/20240715_143022_9f8e7d6c...jpg
  IMG_0100.jpg         → matches 2024/07-Jul/20240715_143022_9f8e7d6c...jpg

ERRORED (1 file)
  corrupt.jpg          → EXIF parse failed: truncated IFD at offset 0x1A

UNSORTED (12 files)
  IMG_5001.jpg
  IMG_5002.jpg
  vacation/IMG_6001.jpg
  vacation/IMG_6002.jpg

UNRECOGNIZED (2 files)
  notes.txt            → unsupported format: .txt
  readme.pdf           → unsupported format: .pdf

265 total | 247 sorted | 3 duplicates | 1 errored | 12 unsorted | 2 unrecognized
```

**Design notes:**

- **SORTED** files show the `destination` field from the ledger entry — the relative path within `dirB` where the file was copied. This is truncated in table mode for terminal width.
- **DUPLICATE** files show the `matches` field — the relative path of the existing archive file this duplicate matched.
- **ERRORED** files show the `reason` field from the ledger entry.
- **UNSORTED** files have no additional information — they simply haven't been processed yet.
- **UNRECOGNIZED** files show the skip reason from the discovery walk (e.g., `unsupported format: .txt`).
- The **summary line** at the bottom provides a complete accounting. Every file discovered on disk appears in exactly one category. The total equals the sum of all categories.
- Files are sorted **alphabetically by relative path** within each section.

**No-ledger output:**

When no ledger exists, the SORTED, DUPLICATE, and ERRORED sections are empty (omitted), and the output reflects that all recognized files are unsorted:

```
Source: /Users/wells/photos
Ledger: none found — no prior sort runs recorded for this directory.

UNSORTED (250 files)
  IMG_0001.jpg
  IMG_0002.jpg
  ...

UNRECOGNIZED (2 files)
  notes.txt            → unsupported format: .txt
  readme.pdf           → unsupported format: .pdf

252 total | 250 unsorted | 2 unrecognized
```

#### 7.4.5 JSON Output

When `--json` is passed, the output is a single JSON object:

```json
{
  "source": "/Users/wells/photos",
  "ledger": {
    "run_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "pixe_version": "0.10.0",
    "timestamp": "2026-03-06T10:30:00Z",
    "recursive": false
  },
  "sorted": [
    {"path": "IMG_0001.jpg", "destination": "2021/12-Dec/20211225_062223_7d97e98f...jpg"},
    {"path": "IMG_0002.jpg", "destination": "2022/02-Feb/20220202_123101_447d3060...jpg"}
  ],
  "duplicates": [
    {"path": "IMG_0042.jpg", "matches": "2022/02-Feb/20220202_123101_447d3060...jpg"}
  ],
  "errored": [
    {"path": "corrupt.jpg", "reason": "EXIF parse failed: truncated IFD at offset 0x1A"}
  ],
  "unsorted": [
    {"path": "IMG_5001.jpg"},
    {"path": "vacation/IMG_6001.jpg"}
  ],
  "unrecognized": [
    {"path": "notes.txt", "reason": "unsupported format: .txt"}
  ],
  "summary": {
    "total": 265,
    "sorted": 247,
    "duplicates": 3,
    "errored": 1,
    "unsorted": 12,
    "unrecognized": 2
  }
}
```

When no ledger exists, the `ledger` field is `null` and the `sorted`, `duplicates`, and `errored` arrays are empty.

Empty arrays are included in JSON output (not omitted) for consistent structure.

#### 7.4.6 Handler Registration

`pixe status` requires the same handler registry as `pixe sort` — it uses `discovery.Walk()` which needs handlers for file detection. The same registration block used in `sort.go` is replicated in `status.go`:

```go
reg := discovery.NewRegistry()
reg.Register(jpeghandler.New())
reg.Register(heichandler.New())
reg.Register(mp4handler.New())
reg.Register(dnghandler.New())
reg.Register(nefhandler.New())
reg.Register(cr2handler.New())
reg.Register(cr3handler.New())
reg.Register(pefhandler.New())
reg.Register(arwhandler.New())
```

This ensures that `pixe status` and `pixe sort` have identical file recognition behavior — a file classified as "unsorted" by `status` will be picked up by the next `sort` run.

#### 7.4.7 Package Layout

```
cmd/
├── status.go             ← `pixe status` command definition, flag binding, RunE
```

The command logic is self-contained in a single file. It:

1. Reads flags from Viper (`source`, `recursive`, `ignore`, `json`). If `source` is empty, defaults to the current working directory (`os.Getwd()`).
2. Builds the handler registry and ignore matcher.
3. Calls `discovery.Walk()` to discover files on disk.
4. Calls `manifest.LoadLedger()` to load the ledger.
5. Classifies files into the five categories.
6. Formats and prints the output (table or JSON).

No new internal packages are needed. The command composes existing packages (`discovery`, `manifest`, `ignore`) without modification.

#### 7.4.8 Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success — status report produced (regardless of whether unsorted files exist) |
| `1` | Error — source directory not found, unreadable, or other fatal error |

The presence of unsorted files is not an error condition — it's the expected state before running `pixe sort`. A future enhancement could add a `--check` flag that exits with code `2` when unsorted files exist (useful for scripting/CI), but this is out of scope for the initial implementation.

### 7.5 Clean Command

`pixe clean` is a **destination-oriented maintenance command** that performs housekeeping operations on a `dirB` archive. It combines two distinct cleanup responsibilities — orphaned artifact removal and database compaction — into a single command that a user runs periodically on an established archive.

#### 7.5.1 Core Responsibilities

| Responsibility | What it does | Why it's needed |
|---|---|---|
| **Orphaned temp file cleanup** | Scans `dirB` for `.pixe-tmp` files left behind by interrupted `pixe sort` runs and removes them. | Interrupted runs (crash, Ctrl+C, power loss) leave temp files on disk. These are harmless (they don't interfere with subsequent runs) but waste space and clutter the archive. See Section 4.10. |
| **Orphaned XMP sidecar cleanup** | Detects `.xmp` sidecar files in `dirB` that have no corresponding media file and removes them. | If a run is interrupted after writing an XMP sidecar but before the media file's temp file is renamed to its canonical path, the sidecar is left orphaned — associated with a media file that was never finalized. |
| **Database compaction** | Runs `VACUUM` on the archive SQLite database to reclaim space. | Long-lived archives accumulate deleted rows and fragmentation from many runs. `VACUUM` rebuilds the database file, reclaiming disk space and improving query performance. |

#### 7.5.2 Command Signature

```bash
pixe clean --dir <dirB> [options]
```

| Flag | Short | Default | Description |
|---|---|---|---|
| `--dir` | `-d` | (required) | Destination directory (`dirB`) to clean. |
| `--db-path` | | (auto-detected) | Explicit path to the SQLite database file. Overrides automatic location logic. |
| `--dry-run` | | `false` | Preview what would be cleaned without deleting files or running VACUUM. Lists all orphaned artifacts and reports current database size. |
| `--temp-only` | | `false` | Only clean orphaned temp files and XMP sidecars. Skip database compaction. |
| `--vacuum-only` | | `false` | Only run database compaction. Skip temp file and sidecar scanning. |

**Flag validation:** `--temp-only` and `--vacuum-only` are mutually exclusive. If both are specified, the command exits with an error. When neither is specified, both operations are performed (the default).

#### 7.5.3 Orphaned Temp File Cleanup

**Detection:** The command walks `dirB` recursively using `filepath.Walk` and identifies orphaned temp files by the presence of `.pixe-tmp` in the filename. This catches both the deterministic pattern (`.<basename>.pixe-tmp`) and the `os.CreateTemp` pattern (`.<basename>.pixe-tmp-<random>`), as both contain the `.pixe-tmp` substring.

**Walk scope:** The walk covers all of `dirB`, including year/month subdirectories, the `duplicates/` tree, and the `.pixe/` directory. The walk does **not** descend into symlinks.

**Deletion:** Each identified temp file is removed via `os.Remove`. Removal failures (e.g., permission denied) are reported as warnings but do not halt the scan — the command continues to the next file.

**Output:** Each deleted temp file produces one line of stdout output:

```
REMOVE .20211225_062223_7d97e98f...jpg.pixe-tmp-abc123  (2021/12-Dec/)
REMOVE .20220202_123101_447d3060...mp4.pixe-tmp-def456  (2022/02-Feb/)
```

The format is `REMOVE <filename>  (<parent_directory_relative_to_dirB>/)`. This follows Pixe's one-line-per-file output convention.

In **dry-run mode**, the verb changes to `WOULD REMOVE` and no files are deleted:

```
WOULD REMOVE .20211225_062223_7d97e98f...jpg.pixe-tmp-abc123  (2021/12-Dec/)
WOULD REMOVE .20220202_123101_447d3060...mp4.pixe-tmp-def456  (2022/02-Feb/)
```

#### 7.5.4 Orphaned XMP Sidecar Cleanup

**Detection:** During the same `filepath.Walk` used for temp file scanning, the command identifies `.xmp` sidecar files and checks whether a corresponding media file exists at the expected path.

A sidecar file at path `<dir>/<stem>.<media_ext>.xmp` is considered orphaned if no file exists at `<dir>/<stem>.<media_ext>`. For example, if `2021/12-Dec/20211225_071500_a3b4c5d6...arw.xmp` exists but `2021/12-Dec/20211225_071500_a3b4c5d6...arw` does not, the sidecar is orphaned.

**Scope limitation:** Only `.xmp` files whose name matches the Pixe naming convention (`<YYYYMMDD_HHMMSS_CHECKSUM>.<media_ext>.xmp`) are considered. This prevents accidentally removing user-created or third-party XMP files that happen to be in `dirB`. The detection uses a regex match on the sidecar filename: `^\d{8}_\d{6}_[0-9a-f]+\..+\.xmp$`.

**Deletion and output:** Same pattern as temp file cleanup — `REMOVE` / `WOULD REMOVE` per file, warnings on failure.

```
REMOVE 20211225_071500_a3b4c5d6...arw.xmp  (2021/12-Dec/)  orphaned sidecar
```

#### 7.5.5 Database Compaction

**Database resolution:** The archive database is located via the same priority chain used by `pixe sort`, `pixe resume`, and `pixe query`: `--db-path` flag → `dirB/.pixe/dbpath` marker → `dirB/.pixe/pixe.db`. Resolution uses `dblocator.Resolve()`.

**Pre-flight safety check:** Before running VACUUM, the command queries the `runs` table for any rows with `status = 'running'`. If an active run is detected, the command refuses to VACUUM and emits a clear error:

```
Error: cannot vacuum — active sort run detected (run a1b2c3d4, started 2026-03-06 10:30:00 UTC).
Complete or interrupt the active run before running 'pixe clean'.
```

This is an application-level guard, not a substitute for SQLite's own locking. It catches the common case of a user running `pixe clean` while a sort is in progress on the same archive. If the `runs` row is stale (e.g., the sort process crashed without marking the run as interrupted), the user can resolve it via `pixe resume` first.

**VACUUM execution:** The command opens the database in read-write mode via `archivedb.Open()` and calls a new `Vacuum()` method on the `DB` struct. VACUUM rebuilds the entire database file, requiring temporary disk space roughly equal to the current database size.

**Size reporting:** The command reports the database file size before and after VACUUM:

```
Database: /Users/wells/archive/.pixe/pixe.db
  Size before: 12.4 MB
  Size after:  8.1 MB
  Reclaimed:   4.3 MB (34.7%)
```

In **dry-run mode**, only the current size is reported:

```
Database: /Users/wells/archive/.pixe/pixe.db
  Current size: 12.4 MB
  (dry-run: VACUUM not executed)
```

If the database does not exist, the compaction step is skipped with a notice: `No archive database found — skipping compaction.` This is not an error; it allows `pixe clean --temp-only` to work on a `dirB` that has orphaned temp files but no database (e.g., after a failed first run).

#### 7.5.6 Combined Output

When both operations are performed (the default), the output is structured in two sections with a summary at the end:

```
Cleaning /Users/wells/archive

Orphaned files:
  REMOVE .20211225_062223_7d97e98f...jpg.pixe-tmp-abc123  (2021/12-Dec/)
  REMOVE .20220202_123101_447d3060...mp4.pixe-tmp-def456  (2022/02-Feb/)
  REMOVE 20211225_071500_a3b4c5d6...arw.xmp               (2021/12-Dec/)  orphaned sidecar

Database compaction:
  Size before: 12.4 MB
  Size after:  8.1 MB
  Reclaimed:   4.3 MB (34.7%)

Cleaned 2 temp files, 1 orphaned sidecar | Reclaimed 4.3 MB from database
```

When no orphaned files are found:

```
Cleaning /Users/wells/archive

Orphaned files:
  No orphaned files found.

Database compaction:
  Size before: 8.1 MB
  Size after:  8.1 MB
  Reclaimed:   0 B (0.0%)

No orphaned files found | Reclaimed 0 B from database
```

#### 7.5.7 Database Method: `Vacuum()`

A new method is added to `archivedb.DB`:

```go
// Vacuum rebuilds the database file, reclaiming space from deleted rows
// and reducing fragmentation. Requires exclusive access — no concurrent
// readers or writers should be active.
func (db *DB) Vacuum() error {
    _, err := db.conn.Exec("VACUUM")
    if err != nil {
        return fmt.Errorf("archivedb: vacuum: %w", err)
    }
    return nil
}
```

A companion method exposes the active-run check:

```go
// HasActiveRuns returns true if any run has status = 'running'.
// Used by pixe clean to guard against vacuuming while a sort is in progress.
func (db *DB) HasActiveRuns() (bool, error) {
    var count int
    err := db.conn.QueryRow(`SELECT COUNT(*) FROM runs WHERE status = 'running'`).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("archivedb: check active runs: %w", err)
    }
    return count > 0, nil
}
```

#### 7.5.8 Package Layout

```
cmd/
├── clean.go              ← `pixe clean` command definition, flag binding, RunE
```

The command logic is self-contained in a single file. It:

1. Reads flags from Viper (`clean_dir`, `clean_db_path`, `dry_run`, `temp_only`, `vacuum_only`).
2. Validates flag combinations (`--temp-only` and `--vacuum-only` are mutually exclusive).
3. Resolves the absolute path of `--dir`.
4. If temp cleanup is enabled: walks `dirB` with `filepath.Walk`, identifies and removes orphaned temp files and XMP sidecars, prints per-file output.
5. If compaction is enabled: resolves the database via `dblocator.Resolve()`, opens in read-write mode via `archivedb.Open()`, checks for active runs via `HasActiveRuns()`, runs `Vacuum()`, reports size change.
6. Prints the summary line.

No handler registry is needed — `pixe clean` does not process media files. No new internal packages are required; the command composes `archivedb`, `dblocator`, `filepath.Walk`, and `os.Remove`.

#### 7.5.9 Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success — cleanup completed (even if nothing needed cleaning) |
| `1` | Error — `dirB` not found, database access failed, active run detected (when VACUUM requested), or other fatal error |

Individual file removal failures (permission denied on a specific temp file) are **warnings**, not fatal errors. The command continues and exits `0` if the overall operation completed. The summary line notes any removal failures.

---

## 8. Archive Database & Ledger Design

### 8.1 Overview

Pixe uses a **SQLite database** as the cumulative registry for each destination archive. Every `pixe sort` run enriches the database with new file records, building a permanent, queryable history of the entire archive. This replaces the earlier JSON manifest approach, providing indexed queries, concurrent-process safety, and bounded startup cost regardless of archive size.

The database is the **single source of truth** for:
- What files exist in the archive and where they came from
- Deduplication state (checksum lookups)
- Run history and audit trail
- Per-file pipeline state for crash recovery and resume

### 8.2 Database Location

The database location is determined by a priority chain:

1. **Explicit override** — `--db-path` flag or `db_path` config setting. If set, this path is used unconditionally.
2. **Local filesystem** — If `dirB` resides on a local filesystem, the database is stored at `dirB/.pixe/pixe.db`.
3. **Network mount fallback** — If `dirB` is detected to be on a network filesystem (NFS, SMB/CIFS, AFP), the database is stored at `~/.pixe/databases/<slug>.db` and a notice is emitted to the user explaining the location and why.

**The `<slug>` format** for the fallback path is derived from the `dirB` path: the last path component (lowercased, sanitized) followed by a hyphen and a truncated hash of the full absolute path. Example: for `dirB=/Volumes/NAS/Photos/archive`, the slug might be `archive-a1b2c3d4`, yielding `~/.pixe/databases/archive-a1b2c3d4.db`.

**Network mount detection** uses OS-level filesystem type inspection (e.g., `statfs` on macOS/Linux) to identify non-local mounts. SQLite relies on POSIX file locking semantics that NFS and SMB do not reliably honor, making local storage essential for database integrity.

#### Discoverability Marker (`dirB/.pixe/dbpath`)

When the database is stored **outside** `dirB` (due to network mount fallback or explicit `--db-path`), a plain-text marker file is written at `dirB/.pixe/dbpath` containing the absolute path to the database file. This allows commands like `pixe resume --dir <dirB>` to locate the database without the user needing to specify `--db-path`.

When the database lives at the default local path (`dirB/.pixe/pixe.db`), no `dbpath` marker is written — the default location is checked first.

**Lookup order for database discovery:**
1. `--db-path` flag (if provided)
2. `dirB/.pixe/dbpath` marker file (if exists, read and use its contents)
3. `dirB/.pixe/pixe.db` (default local path)

### 8.3 Schema Design

The database uses two primary tables — `runs` and `files` — with a foreign key relationship.

#### `runs` Table

Records each `pixe sort` invocation. A run is created at the start and represents a single execution context.

```sql
CREATE TABLE runs (
    id            TEXT PRIMARY KEY,   -- UUID v4, the run_id
    pixe_version  TEXT NOT NULL,      -- e.g., "0.10.0"
    source        TEXT NOT NULL,      -- absolute path to dirA
    destination   TEXT NOT NULL,      -- absolute path to dirB
    algorithm     TEXT NOT NULL,      -- e.g., "sha1", "sha256"
    workers       INTEGER NOT NULL,   -- worker count for this run
    recursive     INTEGER NOT NULL DEFAULT 0,  -- 1 if --recursive was enabled
    started_at    TEXT NOT NULL,      -- ISO 8601 UTC timestamp
    finished_at   TEXT,               -- NULL if still running or interrupted
    status        TEXT NOT NULL       -- "running", "completed", "interrupted"
        CHECK (status IN ('running', 'completed', 'interrupted'))
);
```

#### `files` Table

Records every file processed across all runs. Each row represents one file's journey through the pipeline.

```sql
CREATE TABLE files (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    source_path   TEXT NOT NULL,      -- absolute path to file in dirA
    dest_path     TEXT,               -- absolute path to file in dirB (NULL until copied)
    dest_rel      TEXT,               -- relative path within dirB (NULL until copied)
    checksum      TEXT,               -- hex-encoded hash (NULL until hashed)
    status        TEXT NOT NULL       -- pipeline stage or terminal outcome
        CHECK (status IN (
            'pending', 'extracted', 'hashed', 'copied',
            'verified', 'tagged', 'complete',
            'failed', 'mismatch', 'tag_failed', 'duplicate',
            'skipped'
        )),
    skip_reason   TEXT,               -- reason for skip (NULL unless status = 'skipped')
    is_duplicate  INTEGER NOT NULL DEFAULT 0,  -- 1 if routed to duplicates/
    capture_date  TEXT,               -- ISO 8601, extracted from metadata
    file_size     INTEGER,            -- source file size in bytes
    extracted_at  TEXT,               -- ISO 8601 UTC
    hashed_at     TEXT,               -- ISO 8601 UTC
    copied_at     TEXT,               -- ISO 8601 UTC
    verified_at   TEXT,               -- ISO 8601 UTC
    tagged_at     TEXT,               -- ISO 8601 UTC
    error         TEXT                -- error message if failed
);
```

#### Indexes

```sql
CREATE INDEX idx_files_checksum ON files(checksum) WHERE status = 'complete';
CREATE INDEX idx_files_run_id ON files(run_id);
CREATE INDEX idx_files_status ON files(status);
CREATE INDEX idx_files_source ON files(source_path);
CREATE INDEX idx_files_capture_date ON files(capture_date);
```

#### Schema Versioning

The database includes a `schema_version` table for forward-compatible migrations:

```sql
CREATE TABLE schema_version (
    version    INTEGER NOT NULL,
    applied_at TEXT NOT NULL
);
-- Initial row: INSERT INTO schema_version VALUES (1, '2026-03-07T...');
```

Future Pixe versions can check this table and apply incremental migrations.

> **Note:** The addition of `recursive` to `runs`, and `skipped` status + `skip_reason` to `files`, constitutes a schema version bump (v1 → v2). Existing databases are migrated by adding the new columns with defaults (`recursive = 0`, `skip_reason = NULL`) and inserting a new `schema_version` row. The `skipped` status value is additive to the CHECK constraint and does not affect existing rows.

### 8.4 Query Patterns

The schema supports the following query families, all served by indexed lookups. Queries marked with **CLI** are exposed to end users via `pixe query` subcommands (see Section 7.3). Queries marked with **Internal** are used by the sort pipeline and are not directly user-facing.

| Query | SQL Pattern | Exposure |
|---|---|---|
| **Dedup check** | `SELECT dest_rel FROM files WHERE checksum = ? AND status = 'complete' LIMIT 1` | Internal |
| **Skip check** | `SELECT id FROM files WHERE source_path = ? AND status IN ('complete', 'duplicate') LIMIT 1` | Internal |
| **Run history** | `SELECT r.*, COUNT(f.id) FROM runs r LEFT JOIN files f ... GROUP BY r.id ORDER BY started_at DESC` | **CLI:** `pixe query runs` |
| **Run detail** | `SELECT * FROM files WHERE run_id = ?` | **CLI:** `pixe query run <id>` |
| **Run by prefix** | `SELECT * FROM runs WHERE id LIKE ?% ORDER BY started_at DESC` | **CLI:** `pixe query run <id>` (prefix resolution) |
| **All duplicates** | `SELECT * FROM files WHERE is_duplicate = 1` | **CLI:** `pixe query duplicates` |
| **Duplicate pairs** | `SELECT d.source_path, d.dest_rel, o.dest_rel FROM files d JOIN files o ON d.checksum = o.checksum AND o.is_duplicate = 0 AND o.status = 'complete' WHERE d.is_duplicate = 1` | **CLI:** `pixe query duplicates --pairs` |
| **All errors/mismatches** | `SELECT f.*, r.source FROM files f JOIN runs r ON f.run_id = r.id WHERE f.status IN ('failed', 'mismatch', 'tag_failed')` | **CLI:** `pixe query errors` |
| **All skipped** | `SELECT source_path, skip_reason FROM files WHERE status = 'skipped'` | **CLI:** `pixe query skipped` |
| **Files from source** | `SELECT * FROM files f JOIN runs r ON r.id = f.run_id WHERE r.source = ?` | **CLI:** `pixe query files --source` |
| **Files by capture date range** | `SELECT * FROM files WHERE capture_date BETWEEN ? AND ? AND status = 'complete'` | **CLI:** `pixe query files --from/--to` |
| **Files by import date range** | `SELECT * FROM files WHERE verified_at BETWEEN ? AND ?` | **CLI:** `pixe query files --imported-from/--imported-to` |
| **Archive inventory** | `SELECT dest_rel, checksum, capture_date FROM files WHERE status = 'complete' AND is_duplicate = 0` | **CLI:** `pixe query inventory` |
| **Archive stats** | Aggregate: `COUNT`, `SUM(file_size)`, `MIN/MAX(capture_date)`, grouped by status | **CLI:** summary lines on all subcommands |

### 8.5 Concurrency & Integrity

#### WAL Mode

The database is opened in **Write-Ahead Logging (WAL) mode** (`PRAGMA journal_mode=WAL`). This allows concurrent readers while a writer is active, which is critical for multi-process access.

#### Busy Retry

When a write is blocked by another process, SQLite returns `SQLITE_BUSY`. Pixe configures a **busy timeout** (e.g., 5 seconds) via `PRAGMA busy_timeout=5000`, causing SQLite to retry automatically rather than failing immediately.

#### Transaction Granularity

Each file completion is committed in its own transaction. This provides the same crash-safety guarantee as the prior JSON approach (at most one in-flight file is lost on crash), but with dramatically lower overhead — a single row INSERT/UPDATE versus reserializing the entire manifest.

#### Cross-Process Dedup Race Condition

When two simultaneous runs discover the same file (identical checksum) from different sources:

1. Both processes query `SELECT ... WHERE checksum = ?` — both see "not yet imported."
2. Both copy the file to a temp file in `dirB`, verify it, and rename to the canonical path.
3. The first to commit its INSERT wins. The second process, when it commits, detects the conflict (the checksum now exists with `status = 'complete'`) and retroactively routes its copy to `duplicates/` via `os.Rename`.

This is handled at the application level after commit, not via database constraints, since the duplicate file has already been physically written and verified. The result is safe and correct — no data loss, duplicates are properly categorized. The atomic copy pattern (Section 4.10) ensures that the file at the canonical path was verified before the rename, so the race handler is always relocating a known-good file.

### 8.6 Database Lifecycle

#### Initialization

On first run against a `dirB` with no existing database:
1. Determine database location (see Section 8.2).
2. Create the database file and apply the schema.
3. Write the `dbpath` marker if the database is stored outside `dirB`.
4. Create a `runs` row with `status = 'running'`.

#### Run Completion

1. Update the `runs` row: set `finished_at` and `status = 'completed'`.
2. Close the JSONL ledger file handle (see Section 8.8). The ledger has been streamed progressively throughout the run — no final write is needed.

#### Interrupted Run Recovery

On startup, if a `runs` row exists with `status = 'running'`:
1. The run was interrupted (crash, Ctrl+C, power loss).
2. All `files` rows in non-terminal states for that run are candidates for reprocessing.
3. The `pixe resume` command queries these rows and re-enters the pipeline at the appropriate stage for each file.

### 8.7 Migration from JSON Manifest

When Pixe encounters an existing `dirB/.pixe/manifest.json` but no `pixe.db`, it performs an **automatic migration**:

1. **Create** the SQLite database (at the appropriate location per Section 8.2).
2. **Create a synthetic run** in the `runs` table representing the prior JSON-based import, using metadata from the JSON manifest (`pixe_version`, `source`, `destination`, `algorithm`, `started_at`).
3. **Import all file entries** from the JSON manifest into the `files` table, preserving all timestamps, checksums, statuses, and paths. The `run_id` references the synthetic run.
4. **Rename** the original manifest to `dirB/.pixe/manifest.json.migrated` (preserved, not deleted).
5. **Write** the `dbpath` marker if applicable.
6. **Emit a user-facing notice**: `"Migrated N files from manifest.json → pixe.db"`.
7. **Proceed** with the current sort operation normally.

The migration is idempotent — if `manifest.json.migrated` already exists, the migration is skipped (the DB is assumed to be authoritative).

### 8.8 Ledger (`dirA/.pixe_ledger.json`)

The ledger is a **streaming JSONL receipt** left in the source directory. It records the outcome for **every file** Pixe discovered in `dirA` during the run — not just successful copies. This makes the ledger a full manifest of what Pixe saw and decided for every file in the source directory.

#### Format: JSONL (JSON Lines)

The ledger uses the [JSONL format](https://jsonlines.org/): every line is an independent, valid JSON object terminated by a newline (`\n`). There is no enclosing array or outer object. This enables **streaming writes** — each entry is appended to the file as it is processed, rather than buffering the entire run in memory and serializing at the end.

**Line 1** is a **header object** containing run-level metadata. All subsequent lines are **file entry objects**, one per discovered file, appended in processing order.

#### Ledger Format (v4)

```jsonl
{"version":4,"run_id":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","pixe_version":"0.10.0","pixe_run":"2026-03-06T10:30:00Z","algorithm":"sha1","destination":"/path/to/dirB","recursive":true}
{"path":"IMG_0001.jpg","status":"copy","checksum":"7d97e98f8af710c7e7fe703abc8f639e0ee507c4","destination":"2021/12-Dec/20211225_062223_7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg","verified_at":"2026-03-06T10:30:03Z"}
{"path":"IMG_0002.jpg","status":"skip","reason":"previously imported"}
{"path":"vacation/IMG_0042.jpg","status":"duplicate","checksum":"447d3060abc123...","destination":"duplicates/20260306_103000/2022/02-Feb/20220202_123101_447d3060...jpg","matches":"2022/02-Feb/20220202_123101_447d3060...jpg"}
{"path":"vacation/IMG_0099.jpg","status":"duplicate","checksum":"9f8e7d6c5b4a...","matches":"2024/07-Jul/20240715_143022_9f8e7d6c...jpg"}
{"path":"notes.txt","status":"skip","reason":"unsupported format: .txt"}
{"path":"corrupt.jpg","status":"error","reason":"EXIF parse failed: truncated IFD at offset 0x1A"}
```

> **Note:** The two `duplicate` entries above illustrate the difference between default and `--skip-duplicates` modes. `IMG_0042.jpg` was copied to `duplicates/` (has a `destination`). `IMG_0099.jpg` was skipped (no `destination`, only `matches`). Both are valid `duplicate` entries — the presence or absence of `destination` distinguishes them.

#### Header Object (Line 1)

The header is written once at the start of the run, before any files are processed.

| Field | Type | Description |
|---|---|---|
| `version` | `int` | Ledger schema version. Currently `4`. |
| `run_id` | `string` | UUID linking this ledger to the archive database `runs` table. |
| `pixe_version` | `string` | Pixe binary version that produced this ledger. |
| `pixe_run` | `string` | ISO 8601 UTC timestamp of when the sort run started. |
| `algorithm` | `string` | Hash algorithm used (`sha1`, `sha256`). |
| `destination` | `string` | Absolute path to `dirB`. |
| `recursive` | `bool` | Whether `--recursive` was active for this run. |

#### File Entry Objects (Lines 2+)

Each file entry is a self-contained JSON object. Entries are appended one at a time as the coordinator processes results from workers.

| Field | Presence | Description |
|---|---|---|
| `path` | Always | Relative path from `dirA`. Top-level files are just the filename; recursive files include the subdirectory prefix (e.g., `vacation/IMG_0042.jpg`). |
| `status` | Always | One of: `copy`, `skip`, `duplicate`, `error`. Corresponds to the stdout verbs `COPY`, `SKIP`, `DUPE`, `ERR`. |
| `checksum` | `copy`, `duplicate` | Hex-encoded content hash. Present when the file was hashed (even if it was then identified as a duplicate). |
| `destination` | `copy`, `duplicate` (when copied) | Relative path within `dirB` where the file was written. For copied duplicates, this is the path under `duplicates/`. Absent when `--skip-duplicates` is active (no file was written). |
| `verified_at` | `copy` | ISO 8601 UTC timestamp of successful verification. |
| `matches` | `duplicate` | Relative path within `dirB` of the existing file this duplicate matches. |
| `reason` | `skip`, `error` | Human-readable explanation of why the file was skipped or why processing failed. Same text shown on stdout. |

Fields use `omitempty` semantics — absent from the JSON when zero-valued. Each line is compact (no indentation) to keep the JSONL format clean and `jq`-friendly.

#### Write Strategy: Streaming Appends

The ledger is **not** buffered in memory. Instead, a `LedgerWriter` opens the file at the start of the run, writes the header line, and then appends each file entry as a single JSON line as results arrive from the pipeline.

**Write flow:**

1. **Run start:** Open `dirA/.pixe_ledger.json` for writing (truncate). Write the header object as line 1. Flush.
2. **During processing:** The coordinator goroutine — the sole writer — appends one JSON line per file as each result is finalized. Each append is a `json.Marshal` + `\n` + flush.
3. **Run end:** Close the file. No rename step is needed — the file has been written progressively throughout the run.

**Key design points:**

- The **coordinator goroutine is the sole writer** to the ledger file. Workers never write to the ledger directly. This eliminates the need for a mutex on the file handle — ownership is structural, not lock-based.
- The in-memory `Ledger` struct with its `[]LedgerEntry` slice is **eliminated**. The `LedgerWriter` holds only the open file handle and a `json.Encoder`. Per-file entries are serialized and flushed immediately, then discarded. Memory usage is O(1) regardless of file count.
- File entries appear in **processing order**, not discovery order. In concurrent mode, the order depends on which workers finish first. This is acceptable — the ledger is a receipt, not a sorted index.
- An **interrupted run** (crash, Ctrl+C) leaves a partial but valid JSONL file. Every line written before the interruption is a complete, parseable JSON object. The header is always present (written first). This is strictly better than the prior atomic-write approach, which produced no ledger at all on interruption.
- In **dry-run mode**, the ledger is not written (same as before).

#### Changes from v3

- **Format change:** Single JSON object → JSONL (one JSON object per line).
- **`version` bumped to `4`.**
- **Header/entry split:** Run metadata is on line 1; file entries are on lines 2+. The `files` array wrapper is removed.
- **Streaming writes:** Entries are appended during the run instead of buffered and written atomically at the end. The `.tmp` + rename pattern is removed.
- **In-memory `Ledger` struct eliminated.** Replaced by a `LedgerWriter` that holds an open file handle and streams entries.
- **Partial ledger on interruption.** An interrupted run now leaves a valid (but incomplete) JSONL ledger rather than no ledger at all.

#### Ledger Placement

A single ledger is always written at `dirA/.pixe_ledger.json`, regardless of whether `--recursive` is enabled. In recursive mode, file paths within the ledger use relative paths from `dirA` (e.g., `vacation/IMG_0042.jpg`).

The ledger is the **only file** Pixe writes to `dirA`.

### 8.9 Filesystem Layout

```
dirB (organized destination)
├── 2021/
│   └── 12-Dec/
│       ├── 20211225_062223_7d97e98f...jpg       ← JPEG: tags embedded in file
│       ├── 20211225_071500_a3b4c5d6...arw       ← RAW: tags in sidecar
│       └── 20211225_071500_a3b4c5d6...arw.xmp   ← XMP sidecar (Adobe convention)
├── 2022/
│   ├── 02-Feb/
│   │   ├── 20220202_123101_447d3060...jpg
│   │   ├── 20220202_130000_e5f6a7b8...dng
│   │   └── 20220202_130000_e5f6a7b8...dng.xmp
│   └── 03-Mar/
│       ├── 20220316_232122_321c7d6f...mp4
│       └── 20220316_232122_321c7d6f...mp4.xmp
├── duplicates/
│   └── 20260306_103000/
│       └── 2022/02-Feb/
│           ├── 20220202_123101_447d...jpg
│           ├── 20220202_130000_e5f6...nef
│           └── 20220202_130000_e5f6...nef.xmp   ← sidecars follow dupes too
└── .pixe/
    ├── pixe.db              ← SQLite database (if dirB is local)
    ├── pixe.db-wal          ← WAL file (transient, managed by SQLite)
    ├── pixe.db-shm          ← shared memory file (transient, managed by SQLite)
    └── dbpath               ← marker file (only if DB is stored elsewhere)

~/.pixe/                      ← user-level directory (created only when needed)
└── databases/
    └── archive-a1b2c3d4.db  ← database for a network-mounted dirB
```

> **Note:** XMP sidecar files (`.xmp`) only appear when the user has configured `--copyright` and/or `--camera-owner`. If no metadata tags are configured, no sidecars are generated and the layout is identical to the pre-sidecar design.

> **Note:** During active copy operations, temporary files with the pattern `.<filename>.pixe-tmp` may be visible in destination directories (e.g., `2021/12-Dec/.20211225_062223_7d97e98f...jpg.pixe-tmp`). These are transient artifacts of the atomic copy process (Section 4.10) and are renamed to their canonical paths upon successful verification. Orphaned temp files from interrupted runs are self-healing on `pixe resume` and can be cleaned via `pixe clean` (Section 7.5).

---

## 9. CLI Additions

### 9.1 New Flags

| Command | Flag | Short | Default | Config Key | Description |
|---|---|---|---|---|---|
| `pixe sort` | `--db-path` | | (auto-detected) | `db_path` | Explicit path to the SQLite database file. Overrides all automatic location logic. |
| `pixe sort` | `--recursive` | `-r` | `false` | `recursive` | Recursively process subdirectories of `--source`. Default is top-level only. |
| `pixe sort` | `--skip-duplicates` | | `false` | `skip_duplicates` | Skip copying files whose checksum matches an already-archived file. Emits `DUPE` on stdout and records in DB/ledger, but writes no bytes to `dirB`. Default is to copy duplicates to `duplicates/<run_timestamp>/...` for user review. See Section 4.6. |
| `pixe sort` | `--ignore` | | (none) | `ignore` (list) | Glob pattern for files to ignore. Repeatable on CLI; list in config. Merged additively. `.pixe_ledger.json` is always ignored (hardcoded). |
| `pixe clean` | `--dir` | `-d` | (required) | `clean_dir` | Destination directory (`dirB`) to clean. |
| `pixe clean` | `--db-path` | | (auto-detected) | `clean_db_path` | Explicit path to the SQLite database file. Overrides automatic location logic. |
| `pixe clean` | `--dry-run` | | `false` | — | Preview what would be cleaned without deleting files or running VACUUM. |
| `pixe clean` | `--temp-only` | | `false` | — | Only clean orphaned temp files and XMP sidecars. Skip database compaction. |
| `pixe clean` | `--vacuum-only` | | `false` | — | Only run database compaction. Skip artifact scanning. Mutually exclusive with `--temp-only`. |

All flags are supported via config file and environment variable (e.g., `PIXE_RECURSIVE`, `PIXE_SKIP_DUPLICATES`, `PIXE_IGNORE`). The `--ignore` flag can appear multiple times on the command line, each specifying one glob pattern. In the config file, `ignore` is a YAML list.

### 9.2 Updated `pixe resume`

The `resume` command now locates the database via the same discovery chain (flag → `dbpath` marker → default local path) and queries for the interrupted run's incomplete files.

---

## 10. Open Questions & Future Considerations

These items are explicitly **out of scope** for the current build but are acknowledged for future planning:

1. **Source sidecar association** (`.aae`, existing `.xmp`) — When sorting from `dirA`, should Pixe detect and carry along sidecar files that already exist alongside source media files? Currently, only Pixe-generated XMP sidecars (written to `dirB` during tagging) are handled. Pre-existing sidecars in `dirA` are treated as unrecognized files.
2. **CRW format** (legacy Canon pre-2004) — Excluded from current RAW support due to its obsolete proprietary format and lack of pure-Go library support. Could be revisited if demand arises.
3. **MP4/MOV embedded metadata writing** — MP4 currently uses `MetadataSidecar`. A future enhancement could implement `udta/©cpy` and `udta/©own` atom writing in pure Go and promote MP4 to `MetadataEmbed`, eliminating the sidecar for video files.
4. **HEIC embedded metadata writing** — HEIC currently uses `MetadataSidecar`. If a reliable pure-Go HEIC EXIF writer becomes available, HEIC could be promoted to `MetadataEmbed`. The `MetadataCapability` enum makes this a one-line change per handler.
5. **Web UI / TUI** — Progress visualization beyond CLI output.
6. **Cloud storage targets** — `dirB` on S3, GCS, etc.
7. **GPS/location-based organization** — Subdirectories by location in addition to date.
8. ~~**`pixe query` CLI command**~~ — **Promoted to Section 7.3.** No longer a future consideration.
9. ~~**`pixe clean` command**~~ — **Promoted to Section 7.5.** No longer a future consideration.
10. **Multi-archive federation** — Querying across multiple `dirB` databases from a single command.
11. **`**` recursive glob support in ignore patterns** — Go's `filepath.Match` does not support `**`. A library like `bmatcuk/doublestar` could enable patterns like `**/Thumbs.db`. Currently, ignore patterns match against filename and single-level relative paths.
12. **Directory-level ignore patterns** — In recursive mode, allow ignoring entire subdirectories (e.g., `--ignore ".git/"` to skip `.git` trees). Currently only files are subject to ignore matching.
13. **`.pixeignore` file** — A `.gitignore`-style file in `dirA` that specifies ignore patterns, complementing the CLI flag and config file. Lower priority than the other configuration sources.
14. **Extended XMP fields** — The current XMP sidecar writes only Copyright and CameraOwner. Future work could add additional fields (keywords, captions, GPS coordinates, star ratings) to the `MetadataTags` struct and XMP template.
15. **Split-brain network dedup (multi-machine NAS)** — When two machines run `pixe sort` against the same NAS `dirB`, each with its own local `~/.pixe/databases/<slug>.db`, there is no shared state for dedup. Both may write the same file to the primary archive without detecting the collision. A filesystem-level locking strategy using `O_EXCL` temp file creation could address this — the OS guarantees atomicity of `O_EXCL` even over modern SMB/NFS. Deferred until the multi-machine NAS workflow is actively used.
