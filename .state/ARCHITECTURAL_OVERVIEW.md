# Architectural Overview: Pixe

## 1. Vision & Strategy

**Pixe** is a Go-based photo and video sorting utility that safely organizes irreplaceable media files into a deterministic, date-based directory structure with embedded integrity checksums. It is designed for personal and family media archives where data loss is unacceptable.

### Core Problem

Media libraries accumulate across devices, cameras, and cloud exports with inconsistent naming and no structural organization. Pixe provides a single, repeatable operation that transforms a flat directory of media into a chronologically organized, integrity-verified archive — without ever risking the originals.

### North Star Principles

1. **Safety above all else.** Source files are never modified or moved. Every copy is verified before being considered complete. An interrupted run can always be resumed.
2. **Native Go execution.** All functionality — metadata extraction, hashing, file operations — uses native Go packages. No shelling out to `exiftool`, `ffmpeg`, or other external binaries.
3. **Deterministic output.** Given the same input files and configuration, Pixe always produces the same directory structure and filenames.
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
| **Hashing** | `crypto/sha1` (default), `crypto/sha256`, others via `crypto` stdlib | Configurable algorithm, SHA-1 default for filename brevity |
| **Persistence** | JSON manifest file | Lightweight, human-readable, resume-capable |

---

## 3. Version Management

### 3.1 Single Source of Truth

Pixe's version is defined in a dedicated, importable Go source file at `internal/version/version.go`. This file is the **single source of truth** for version information across the entire project — CLI output, manifest recording, and any future consumer.

**Location:** `internal/version/version.go`

```go
// Package version provides the centralized version constant for Pixe.
// This is the single source of truth — update the Version constant here
// when cutting a new release.
package version

// Version is the semantic version of Pixe (without the "v" prefix).
// Update this value when cutting a new release.
const Version = "0.9.0"
```

- The version follows **Semantic Versioning** (`MAJOR.MINOR.PATCH`).
- The constant is stored **without** the `v` prefix; the `v` is prepended at display time.
- Initial version: **`0.9.0`**.

### 3.2 Build-Time Metadata

In addition to the hardcoded version, the build system injects **commit hash** and **build timestamp** via Go linker flags (`-ldflags -X`). These are stored as package-level variables (not constants) so they can be overwritten at link time:

```go
// Commit is the short git SHA, injected at build time via -ldflags.
var Commit = "unknown"

// BuildDate is the UTC build timestamp, injected at build time via -ldflags.
var BuildDate = "unknown"
```

The Makefile wires these with:

```makefile
LDFLAGS := -s -w \
    -X '$(MODULE)/internal/version.Commit=$(COMMIT)' \
    -X '$(MODULE)/internal/version.BuildDate=$(BUILD_DATE)'
```

> **Note:** The Makefile does **not** override `Version` via ldflags. The Go source file is authoritative. When bumping a release, update `version.go` — the Makefile reads nothing else for the version number.

### 3.3 Accessor Function

A convenience function formats the full human-readable version string:

```go
// Full returns the human-readable version string, e.g.:
//   "pixe v0.9.0 (commit: abc1234, built: 2026-03-06T10:30:00Z)"
func Full() string
```

This is the canonical display format used by the `pixe version` CLI command and available to any internal package that needs it.

### 3.4 Consumers

| Consumer | What it reads | Why |
|---|---|---|
| **`pixe version` CLI command** | `version.Full()` | Human-readable output |
| **Manifest (`manifest.json`)** | `version.Version` | Records which Pixe version produced the archive (future-proofing for format migrations) |
| **Ledger (`.pixe_ledger.json`)** | `version.Version` | Same rationale as manifest |

### 3.5 Version Bump Process

To release a new version:

1. Update the `Version` constant in `internal/version/version.go`.
2. Commit, tag (`git tag v0.9.0`), push.
3. `make build` or `make install` automatically picks up the new constant and injects `Commit`/`BuildDate`.

No other files need to change for a version bump.

---

## 4. Conceptual Design

### 4.1 High-Level Data Flow

```
dirA (read-only source)          dirB (organized destination)
┌──────────────────┐             ┌──────────────────────────────────┐
│ IMG_0001.jpg     │  ────────>  │ 2021/12/                         │
│ IMG_0002.jpg     │   discover  │   20211225_062223_7d97e98f...jpg  │
│ IMG_1234.jpg     │   extract   │ 2022/2/                          │
│ VID_0010.mp4     │   hash      │   20220202_123101_447d3060...jpg  │
│                  │   copy      │ 2022/3/                          │
│                  │   verify    │   20220316_232122_321c7d6f...jpg  │
│                  │   tag       │ duplicates/                      │
│                  │             │   20260306_103000/                │
│ .pixe_ledger.json│             │     2022/2/                      │
└──────────────────┘             │       20220202_123101_447d...jpg  │
                                 │ .pixe/                           │
                                 │   manifest.json                  │
                                 └──────────────────────────────────┘
```

### 4.2 Pipeline Stages

Each file passes through these stages, tracked in the manifest:

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

### 4.3 Output Naming Convention

```
YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
```

- **Date/Time**: Extracted from file metadata (see Section 4.5).
- **Checksum**: Hex-encoded hash of the media payload. Default SHA-1 (40 hex characters).
- **Extension**: Lowercase, preserved from original.

**Directory structure:**

```
<dirB>/<YYYY>/<M>/<filename>
```

- Year: 4-digit.
- Month: Non-zero-padded integer (1–12).

### 4.4 Duplicate Handling

When a file's checksum matches an already-processed file (same data payload):

```
<dirB>/duplicates/<run_timestamp>/<YYYY>/<M>/<filename>
```

- `<run_timestamp>`: ISO-ish format of the Pixe invocation time (e.g., `20260306_103000`).
- The subdirectory structure mirrors the normal import layout, as if `duplicates/<run_timestamp>/` were the root of `dirB`.
- This preserves the duplicate for user review without polluting the primary archive.

### 4.5 Date Fallback Chain

Each filetype module extracts dates using format-appropriate methods. The **authoritative fallback chain** is:

1. **EXIF `DateTimeOriginal`** — Most reliable; represents shutter actuation.
2. **EXIF `CreateDate`** — Secondary; may differ for edited files.
3. **Default date: February 20, 1902** — Ansel Adams' birthday. Used when no metadata date is available. This makes "unknown date" files immediately identifiable in the archive by their `19020220` prefix.

Filesystem timestamps (`ModTime`, `CreationTime`) are explicitly **not used** — they are unreliable across copies, cloud syncs, and OS transfers.

### 4.6 Metadata Tagging (Optional)

On copy to `dirB`, Pixe can inject select EXIF/metadata tags into the destination file. These are **never written to the source**.

| Tag | CLI Flag | Template Support | Example |
|---|---|---|---|
| **Copyright** | `--copyright` | Yes — `{{.Year}}` expands to the file's capture year | `"Copyright {{.Year}} My Family, all rights reserved"` |
| **CameraOwner** | `--camera-owner` | No — freetext string | `"Wells Family"` |

- Both tags are optional. If omitted, no tagging occurs.
- Tagging occurs **after** copy and verify — the checksum reflects the original data, not the tagged version.
- Each filetype module defines how these tags are written for its format (EXIF for JPEG/HEIC, metadata atoms for MP4, etc.).

---

## 5. Global Constraints

> [!IMPORTANT]
> ### 5.1 Operational Safety
> - **`dirA` is read-only.** Pixe never modifies, renames, moves, or deletes source files. The sole exception is writing a `.pixe_ledger.json` file into `dirA` to record what was processed.
> - **Copy-then-verify.** Every file is copied to `dirB`, then the destination is independently re-read and re-hashed to confirm integrity.
> - **Manifest-based resumability.** A manifest at `dirB/.pixe/manifest.json` tracks per-file state. Interrupted runs resume from the last known-good state.
> - **Ledger in `dirA`.** A `.pixe_ledger.json` is written to the source directory, recording which files were successfully processed. This can be verified against later.
> - **No silent data loss.** Hash mismatches, copy failures, and unrecognized files are always reported. Pixe never exits silently on error.

> [!IMPORTANT]
> ### 5.2 Native Execution
> - **No external binary dependencies.** All metadata parsing, hashing, and file operations use pure Go packages or C libraries accessible via cgo only as a last resort.
> - **No `os/exec` calls** for core functionality. The binary must be self-contained.

> [!IMPORTANT]
> ### 5.3 Concurrency Model
> - Pixe uses a **worker pool** pattern for parallel file processing.
> - Worker count is **configurable** via `--workers` flag (default: sensible auto-detect based on `runtime.NumCPU()`).
> - Workers handle the full pipeline per file: extract → hash → copy → verify → tag.
> - A **coordinator goroutine** manages the manifest, deduplication index, and progress reporting.
> - `dirA` and `dirB` may reside on **different filesystems** (local, NAS, SMB). Pixe always uses copy (never `os.Rename` across filesystems).

> [!IMPORTANT]
> ### 5.4 Scalability
> - Must handle from tens to tens of thousands of files in a single run.
> - Memory usage should be bounded — files are streamed, not loaded entirely into memory (except where format parsing requires it).
> - The deduplication index (checksum → path) is held in memory for the duration of a run.

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

### 6.3 Initial Filetype Modules

| Module | Extensions | Date Source | Hashable Region | Tag Support |
|---|---|---|---|---|
| **JPEG** | `.jpg`, `.jpeg` | EXIF `DateTimeOriginal` / `CreateDate` | Full image data (pixel payload) | EXIF Copyright, CameraOwner |
| **HEIC** | `.heic`, `.heif` | EXIF `DateTimeOriginal` / `CreateDate` | Image data payload | EXIF Copyright, CameraOwner |
| **MP4** | `.mp4`, `.mov` | QuickTime `CreationDate` / `mvhd` atom | Collected keyframe data | Metadata atom Copyright, CameraOwner |

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

| Flag | Default | Description |
|---|---|---|
| `--source` | (required) | Source directory (read-only) |
| `--dest` | (required) | Destination directory |
| `--workers` | auto | Number of concurrent workers |
| `--algorithm` | `sha1` | Hash algorithm (`sha1`, `sha256`, etc.) |
| `--copyright` | (none) | Copyright string template. `{{.Year}}` supported. |
| `--camera-owner` | (none) | CameraOwner freetext string |
| `--dry-run` | `false` | Preview operations without copying |

#### `pixe verify`
Walks a previously sorted `dirB`, parses checksums from filenames, recomputes data-only hashes, and reports mismatches.

| Flag | Default | Description |
|---|---|---|
| `--dir` | (required) | Directory to verify |
| `--algorithm` | `sha1` | Hash algorithm (must match what was used during sort) |
| `--workers` | auto | Number of concurrent workers |

#### `pixe resume`
Reads the manifest in `dirB/.pixe/manifest.json` and resumes an interrupted sort operation.

| Flag | Default | Description |
|---|---|---|
| `--dir` | (required) | Destination directory containing `.pixe/manifest.json` |

#### `pixe version`
Prints the version, git commit, and build date in a single human-readable line, then exits. Implemented as a standard Cobra subcommand in `cmd/version.go`.

**Output format:**

```
pixe v0.9.0 (commit: abc1234, built: 2026-03-06T10:30:00Z)
```

No flags. This command calls `version.Full()` from `internal/version` and prints to stdout.

### 7.2 Configuration File

Viper supports a `.pixe.yaml` (or `.pixe.toml`, `.pixe.json`) configuration file for persistent defaults:

```yaml
algorithm: sha1
workers: 8
copyright: "Copyright {{.Year}} My Family, all rights reserved"
camera_owner: "Wells Family"
```

---

## 8. Manifest & Ledger Design

### 8.1 Manifest (`dirB/.pixe/manifest.json`)

The manifest is Pixe's operational journal. It is written to the **destination** directory and tracks the state of every file in the current (or most recent) run.

```json
{
  "version": 1,
  "pixe_version": "0.9.0",
  "source": "/path/to/dirA",
  "destination": "/path/to/dirB",
  "algorithm": "sha1",
  "started_at": "2026-03-06T10:30:00Z",
  "workers": 4,
  "files": [
    {
      "source": "/path/to/dirA/IMG_0001.jpg",
      "destination": "/path/to/dirB/2021/12/20211225_062223_7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg",
      "checksum": "7d97e98f8af710c7e7fe703abc8f639e0ee507c4",
      "status": "complete",
      "extracted_at": "2026-03-06T10:30:01Z",
      "copied_at": "2026-03-06T10:30:02Z",
      "verified_at": "2026-03-06T10:30:03Z",
      "tagged_at": "2026-03-06T10:30:03Z"
    }
  ]
}
```

### 8.2 Ledger (`dirA/.pixe_ledger.json`)

The ledger is a **minimal record** left in the source directory confirming which files were successfully processed. It enables future verification without needing the manifest.

```json
{
  "version": 1,
  "pixe_version": "0.9.0",
  "pixe_run": "2026-03-06T10:30:00Z",
  "algorithm": "sha1",
  "destination": "/path/to/dirB",
  "files": [
    {
      "path": "IMG_0001.jpg",
      "checksum": "7d97e98f8af710c7e7fe703abc8f639e0ee507c4",
      "destination": "2021/12/20211225_062223_7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg",
      "verified_at": "2026-03-06T10:30:03Z"
    }
  ]
}
```

---

## 9. Open Questions & Future Considerations

These items are explicitly **out of scope** for the initial build but are acknowledged for future planning:

1. **Sidecar files** (`.xmp`, `.aae`) — Should they follow their parent media file?
2. **RAW formats** (`.cr2`, `.arw`, `.dng`) — Natural candidates for new filetype modules.
3. **Database backend** — If scale exceeds tens of thousands, a SQLite manifest may outperform JSON.
4. **Web UI / TUI** — Progress visualization beyond CLI output.
5. **Cloud storage targets** — `dirB` on S3, GCS, etc.
6. **GPS/location-based organization** — Subdirectories by location in addition to date.
