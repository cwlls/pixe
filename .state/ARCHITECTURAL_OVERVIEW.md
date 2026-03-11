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
4. **Copied** — File written to its destination path in `dirB`.
5. **Verified** — Destination file re-read and checksum recomputed; matches the source hash.
6. **Tagged** — Optional EXIF tags (Copyright, CameraOwner) injected into the destination copy.
7. **Complete** — All operations successful. Recorded in ledger.

Error states (`failed`, `mismatch`, `tag_failed`) halt processing for that file and flag it for user attention.

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

- `matches <dest_rel>` — The file's content checksum matches an already-archived file. The `<dest_rel>` is the relative path within `dirB` of the existing copy. The duplicate is still physically copied to `duplicates/<run_timestamp>/...` for user review, and the `DUPE` line confirms this routing.

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
pixe sort --source ./photos --dest ./archive --ignore "*.txt" --ignore ".DS_Store" --ignore "Thumbs.db"
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

When a file's checksum matches an already-processed file (same data payload):

```
<dirB>/duplicates/<run_timestamp>/<YYYY>/<MM>-<Mon>/<filename>
```

- `<run_timestamp>`: ISO-ish format of the Pixe invocation time (e.g., `20260306_103000`).
- The subdirectory structure mirrors the normal import layout, as if `duplicates/<run_timestamp>/` were the root of `dirB`. The month directory uses the same `<MM>-<Mon>` format as the primary archive.
- This preserves the duplicate for user review without polluting the primary archive.

### 4.7 Date Fallback Chain

Each filetype module extracts dates using format-appropriate methods. The **authoritative fallback chain** is:

1. **EXIF `DateTimeOriginal`** — Most reliable; represents shutter actuation.
2. **EXIF `CreateDate`** — Secondary; may differ for edited files.
3. **Default date: February 20, 1902** — Ansel Adams' birthday. Used when no metadata date is available. This makes "unknown date" files immediately identifiable in the archive by their `19020220` prefix.

Filesystem timestamps (`ModTime`, `CreationTime`) are explicitly **not used** — they are unreliable across copies, cloud syncs, and OS transfers.

### 4.8 Metadata Tagging (Optional)

On copy to `dirB`, Pixe can inject select EXIF/metadata tags into the destination file. These are **never written to the source**.

| Tag | CLI Flag | Template Support | Example |
|---|---|---|---|
| **Copyright** | `--copyright` | Yes — `{{.Year}}` expands to the file's capture year | `"Copyright {{.Year}} My Family, all rights reserved"` |
| **CameraOwner** | `--camera-owner` | No — freetext string | `"Wells Family"` |

- Both tags are optional. If omitted, no tagging occurs.
- Tagging occurs **after** copy and verify — the checksum reflects the original data, not the tagged version.
- Each filetype module defines how these tags are written for its format (EXIF for JPEG/HEIC, metadata atoms for MP4, etc.).

### 4.9 Recursive Source Processing

By default, Pixe processes only the **top-level files** in `dirA`. Subdirectories are not traversed. The `--recursive` / `-r` flag enables recursive descent into all subdirectories of `dirA`.

#### Default Behavior (non-recursive)

```bash
pixe sort --source ./photos --dest ./archive
```

Only files directly inside `./photos/` are discovered. Subdirectories like `./photos/vacation/` are silently ignored.

#### Recursive Behavior

```bash
pixe sort --source ./photos --dest ./archive --recursive
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
# First run: processes only top-level files
pixe sort --source ./photos --dest ./archive

# Later run: processes everything, skipping already-imported top-level files
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

---

## 5. Global Constraints

> [!IMPORTANT]
> ### 5.1 Operational Safety
> - **`dirA` is read-only.** Pixe never modifies, renames, moves, or deletes source files. The sole exception is writing a `.pixe_ledger.json` file into `dirA` to record what was processed.
> - **Copy-then-verify.** Every file is copied to `dirB`, then the destination is independently re-read and re-hashed to confirm integrity.
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
> - Workers handle the full pipeline per file: extract → hash → copy → verify → tag.
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

    // WriteMetadataTags injects optional EXIF/metadata tags into the
    // destination file. Called after copy and verify.
    WriteMetadataTags(filePath string, tags MetadataTags) error

    // Extensions returns the lowercase file extensions this handler claims
    // (used for initial fast-path detection before magic byte verification).
    Extensions() []string

    // MagicBytes returns the byte signatures used to verify file type.
    MagicBytes() []MagicSignature
}
```

### 6.2 Detection Strategy

1. **Extension-based assumption**: File extension is matched against registered handlers for a fast initial classification.
2. **Magic-byte verification**: The file header is read and compared against the handler's declared magic byte signatures.
3. If magic bytes **do not confirm** the extension-based assumption, the file is reclassified or flagged as unrecognized.

### 6.3 Filetype Modules

| Module | Extensions | Date Source | Hashable Region | Tag Support |
|---|---|---|---|---|
| **JPEG** | `.jpg`, `.jpeg` | EXIF `DateTimeOriginal` / `CreateDate` | Full image data (pixel payload) | EXIF Copyright, CameraOwner |
| **HEIC** | `.heic`, `.heif` | EXIF `DateTimeOriginal` / `CreateDate` | Image data payload | EXIF Copyright, CameraOwner |
| **MP4** | `.mp4`, `.mov` | QuickTime `CreationDate` / `mvhd` atom | Collected keyframe data | Metadata atom Copyright, CameraOwner |
| **DNG** | `.dng` | EXIF `DateTimeOriginal` / `CreateDate` | Embedded full-resolution JPEG preview | No (stub) |
| **NEF** | `.nef` | EXIF `DateTimeOriginal` / `CreateDate` | Embedded full-resolution JPEG preview | No (stub) |
| **CR2** | `.cr2` | EXIF `DateTimeOriginal` / `CreateDate` | Embedded full-resolution JPEG preview | No (stub) |
| **CR3** | `.cr3` | ISOBMFF container metadata (same approach as HEIC/MP4) | Embedded full-resolution JPEG preview | No (stub) |
| **PEF** | `.pef` | EXIF `DateTimeOriginal` / `CreateDate` | Embedded full-resolution JPEG preview | No (stub) |
| **ARW** | `.arw` | EXIF `DateTimeOriginal` / `CreateDate` | Embedded full-resolution JPEG preview | No (stub) |

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

#### 6.4.6 Hashable Region: Embedded JPEG Preview

All supported RAW formats embed a full-resolution JPEG preview image within the file. The `HashableReader()` method extracts this embedded JPEG and returns it as the hashable region.

**Why the embedded JPEG preview?**

- RAW files from cameras are never edited in place — the raw sensor data is immutable.
- Hashing the full file would work (as with HEIC) but would be slower for large RAW files (50-100+ MB).
- The embedded JPEG preview is a stable, well-defined region that is generated from the sensor data at capture time. It provides a meaningful content fingerprint without needing to parse the proprietary raw sensor data.
- If the embedded JPEG cannot be located or extracted, the handler falls back to hashing the full file (same safety-first approach as other handlers).

**Extraction approach for TIFF-based formats:**

The full-resolution JPEG preview is typically stored in a secondary IFD (often IFD1 or a sub-IFD) with `NewSubfileType = 0` (full-resolution image) and `Compression = 6` (JPEG). The handler navigates the IFD chain, locates the JPEG strip/tile offsets and byte counts, and returns a reader over that region.

**Extraction approach for CR3:**

The embedded JPEG is located within the ISOBMFF container, typically in a `moov` → `trak` → `mdat` path or a dedicated preview box. The handler navigates the box structure to extract the JPEG data.

#### 6.4.7 Metadata Write: No-Op Stub

All RAW handlers implement `WriteMetadataTags()` as a **no-op stub**, identical to the HEIC and MP4 approach. RAW files are archival originals — writing metadata into proprietary RAW containers risks corruption and offers little value since the destination copy is already organized and named by Pixe.

```go
func (h *Handler) WriteMetadataTags(filePath string, tags domain.MetadataTags) error {
    // RAW metadata write not supported in pure Go.
    return nil
}
```

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
pixe sort     --source <dirA> --dest <dirB> [options]
pixe verify   --dir <dirB>
pixe resume   --dir <dirB>
pixe version
```

#### `pixe sort`
Primary operation. Discovers files in `dirA`, processes them through the pipeline, and writes organized output to `dirB`.

| Flag | Short | Default | Description |
|---|---|---|---|
| `--source` | | (required) | Source directory (read-only) |
| `--dest` | | (required) | Destination directory |
| `--workers` | | auto | Number of concurrent workers |
| `--algorithm` | | `sha1` | Hash algorithm (`sha1`, `sha256`, etc.) |
| `--copyright` | | (none) | Copyright string template. `{{.Year}}` supported. |
| `--camera-owner` | | (none) | CameraOwner freetext string |
| `--dry-run` | | `false` | Preview operations without copying |
| `--recursive` | `-r` | `false` | Recursively process subdirectories of `--source` |
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

### 7.2 Configuration File

Viper supports a `.pixe.yaml` (or `.pixe.toml`, `.pixe.json`) configuration file for persistent defaults:

```yaml
algorithm: sha1
workers: 8
copyright: "Copyright {{.Year}} My Family, all rights reserved"
camera_owner: "Wells Family"
recursive: false
ignore:
  - "*.txt"
  - ".DS_Store"
  - "Thumbs.db"
```

The `ignore` key is a list of glob patterns. Patterns from the config file are merged with any `--ignore` CLI flags (additive). The hardcoded `.pixe_ledger.json` ignore is always active regardless of config.

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

The schema supports the following query families, all served by indexed lookups:

| Query | SQL Pattern |
|---|---|
| **Dedup check** | `SELECT dest_rel FROM files WHERE checksum = ? AND status = 'complete' LIMIT 1` |
| **Files from source** | `SELECT * FROM files WHERE source_path LIKE ? AND run_id IN (SELECT id FROM runs WHERE source = ?)` |
| **Files by capture date range** | `SELECT * FROM files WHERE capture_date BETWEEN ? AND ? AND status = 'complete'` |
| **Files by import date range** | `SELECT * FROM files WHERE verified_at BETWEEN ? AND ?` |
| **Run history** | `SELECT * FROM runs ORDER BY started_at DESC` |
| **Run detail** | `SELECT * FROM files WHERE run_id = ?` |
| **All errors/mismatches** | `SELECT f.*, r.source FROM files f JOIN runs r ON f.run_id = r.id WHERE f.status IN ('failed', 'mismatch', 'tag_failed')` |
| **All duplicates** | `SELECT * FROM files WHERE is_duplicate = 1` |
| **Duplicate pairs** | `SELECT d.source_path, d.dest_path, o.dest_path AS original FROM files d JOIN files o ON d.checksum = o.checksum AND o.is_duplicate = 0 AND o.status = 'complete' WHERE d.is_duplicate = 1` |
| **All skipped** | `SELECT source_path, skip_reason FROM files WHERE status = 'skipped'` |
| **Skip check** | `SELECT id FROM files WHERE source_path = ? AND status IN ('complete', 'duplicate') LIMIT 1` |
| **Archive inventory** | `SELECT dest_rel, checksum, capture_date FROM files WHERE status = 'complete' AND is_duplicate = 0` |

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
2. Both copy the file to `dirB`.
3. The first to commit its INSERT wins. The second process, when it commits, detects the conflict (the checksum now exists with `status = 'complete'`) and retroactively routes its copy to `duplicates/`.

This is handled at the application level after commit, not via database constraints, since the duplicate file has already been physically written. The result is safe and correct — no data loss, duplicates are properly categorized.

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
{"path":"notes.txt","status":"skip","reason":"unsupported format: .txt"}
{"path":"corrupt.jpg","status":"error","reason":"EXIF parse failed: truncated IFD at offset 0x1A"}
```

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
| `destination` | `copy`, `duplicate` | Relative path within `dirB` where the file was written. For duplicates, this is the path under `duplicates/`. |
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
│       └── 20211225_062223_7d97e98f...jpg
├── 2022/
│   ├── 02-Feb/
│   │   └── 20220202_123101_447d3060...jpg
│   └── 03-Mar/
│       └── 20220316_232122_321c7d6f...jpg
├── duplicates/
│   └── 20260306_103000/
│       └── 2022/02-Feb/
│           └── 20220202_123101_447d...jpg
└── .pixe/
    ├── pixe.db              ← SQLite database (if dirB is local)
    ├── pixe.db-wal          ← WAL file (transient, managed by SQLite)
    ├── pixe.db-shm          ← shared memory file (transient, managed by SQLite)
    └── dbpath               ← marker file (only if DB is stored elsewhere)

~/.pixe/                      ← user-level directory (created only when needed)
└── databases/
    └── archive-a1b2c3d4.db  ← database for a network-mounted dirB
```

---

## 9. CLI Additions

### 9.1 New Flags

| Command | Flag | Short | Default | Config Key | Description |
|---|---|---|---|---|---|
| `pixe sort` | `--db-path` | | (auto-detected) | `db_path` | Explicit path to the SQLite database file. Overrides all automatic location logic. |
| `pixe sort` | `--recursive` | `-r` | `false` | `recursive` | Recursively process subdirectories of `--source`. Default is top-level only. |
| `pixe sort` | `--ignore` | | (none) | `ignore` (list) | Glob pattern for files to ignore. Repeatable on CLI; list in config. Merged additively. `.pixe_ledger.json` is always ignored (hardcoded). |

All flags are supported via config file and environment variable (e.g., `PIXE_RECURSIVE`, `PIXE_IGNORE`). The `--ignore` flag can appear multiple times on the command line, each specifying one glob pattern. In the config file, `ignore` is a YAML list.

### 9.2 Updated `pixe resume`

The `resume` command now locates the database via the same discovery chain (flag → `dbpath` marker → default local path) and queries for the interrupted run's incomplete files.

---

## 10. Open Questions & Future Considerations

These items are explicitly **out of scope** for the current build but are acknowledged for future planning:

1. **Sidecar files** (`.xmp`, `.aae`) — Should they follow their parent media file?
2. **CRW format** (legacy Canon pre-2004) — Excluded from current RAW support due to its obsolete proprietary format and lack of pure-Go library support. Could be revisited if demand arises.
3. **RAW metadata write support** — Currently all RAW handlers use no-op stubs. Future work could add EXIF write support for formats where pure-Go libraries mature.
4. **Web UI / TUI** — Progress visualization beyond CLI output.
5. **Cloud storage targets** — `dirB` on S3, GCS, etc.
6. **GPS/location-based organization** — Subdirectories by location in addition to date.
7. **`pixe query` CLI command** — Expose the database query patterns (Section 8.4) as user-facing subcommands (e.g., `pixe query --duplicates`, `pixe query --errors`, `pixe query --from-source <path>`).
8. **Database compaction/maintenance** — `VACUUM` command exposure for long-lived archives.
9. **Multi-archive federation** — Querying across multiple `dirB` databases from a single command.
10. **`**` recursive glob support in ignore patterns** — Go's `filepath.Match` does not support `**`. A library like `bmatcuk/doublestar` could enable patterns like `**/Thumbs.db`. Currently, ignore patterns match against filename and single-level relative paths.
11. **Directory-level ignore patterns** — In recursive mode, allow ignoring entire subdirectories (e.g., `--ignore ".git/"` to skip `.git` trees). Currently only files are subject to ignore matching.
12. **`.pixeignore` file** — A `.gitignore`-style file in `dirA` that specifies ignore patterns, complementing the CLI flag and config file. Lower priority than the other configuration sources.
