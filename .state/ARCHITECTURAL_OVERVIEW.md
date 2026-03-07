# Architectural Overview: Pixe

## 1. Vision & Strategy

**Pixe** is a Go-based photo and video sorting utility that safely organizes irreplaceable media files into a deterministic, date-based directory structure with embedded integrity checksums. It is designed for personal and family media archives where data loss is unacceptable.

### Core Problem

Media libraries accumulate across devices, cameras, and cloud exports with inconsistent naming and no structural organization. Pixe provides a single, repeatable operation that transforms a flat directory of media into a chronologically organized, integrity-verified archive — without ever risking the originals.

### North Star Principles

1. **Safety above all else.** Source files are never modified or moved. Every copy is verified before being considered complete. An interrupted run can always be resumed.
2. **Native Go execution.** All functionality — metadata extraction, hashing, file operations — uses native Go packages. No shelling out to `exiftool`, `ffmpeg`, or other external binaries.
3. **Deterministic output.** Given the same input files, configuration, and system locale, Pixe always produces the same directory structure and filenames. (Month directory names are locale-aware; see Section 4.3.)
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
│ IMG_1234.jpg     │   extract   │ 2022/02-Feb/                         │
│ VID_0010.mp4     │   hash      │   20220202_123101_447d3060...jpg      │
│                  │   copy      │ 2022/03-Mar/                         │
│                  │   verify    │   20220316_232122_321c7d6f...jpg      │
│                  │   tag       │ duplicates/                          │
│                  │             │   20260306_103000/                    │
│ .pixe_ledger.json│             │     2022/02-Feb/                     │
└──────────────────┘             │       20220202_123101_447d...jpg      │
                                 │ .pixe/                               │
                                 │   pixe.db  (or dbpath marker)        │
                                 └──────────────────────────────────────┘
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

### 4.3 Output Naming Convention

```
YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
```

- **Date/Time**: Extracted from file metadata (see Section 4.5).
- **Checksum**: Hex-encoded hash of the media payload. Default SHA-1 (40 hex characters).
- **Extension**: Lowercase, preserved from original.

**Directory structure:**

```
<dirB>/<YYYY>/<MM>-<Mon>/<filename>
```

- Year: 4-digit.
- Month: Zero-padded two-digit number, a hyphen, and the locale-aware three-letter title-cased month abbreviation (e.g., `01-Jan`, `02-Feb`, `03-Mar`, …, `12-Dec`). The abbreviation is derived from the user's system locale, so a French locale would produce `03-Mar` → `03-Mars` (or the locale's equivalent short form). The number is always zero-padded to two digits.

> **Note:** This format applies only to the month **directory name**. The filename retains its existing `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` format with a zero-padded numeric month.

### 4.4 Duplicate Handling

When a file's checksum matches an already-processed file (same data payload):

```
<dirB>/duplicates/<run_timestamp>/<YYYY>/<MM>-<Mon>/<filename>
```

- `<run_timestamp>`: ISO-ish format of the Pixe invocation time (e.g., `20260306_103000`).
- The subdirectory structure mirrors the normal import layout, as if `duplicates/<run_timestamp>/` were the root of `dirB`. The month directory uses the same `<MM>-<Mon>` format as the primary archive.
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
> - **Database-backed resumability.** A SQLite database tracks per-file state across all runs. Interrupted runs resume from the last committed state. Each file completion is committed individually for crash safety.
> - **Ledger in `dirA`.** A `.pixe_ledger.json` is written to the source directory, recording which files were successfully processed. Each ledger entry includes a `run_id` linking back to the archive database for full queryability.
> - **No silent data loss.** Hash mismatches, copy failures, and unrecognized files are always reported. Pixe never exits silently on error.
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
> - A **coordinator goroutine** manages database writes, deduplication queries, and progress reporting.
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
```

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
    status        TEXT NOT NULL       -- pipeline stage
        CHECK (status IN (
            'pending', 'extracted', 'hashed', 'copied',
            'verified', 'tagged', 'complete',
            'failed', 'mismatch', 'tag_failed', 'duplicate'
        )),
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
2. Write the ledger to `dirA` (see Section 8.8).

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

The ledger remains a **lightweight JSON receipt** left in the source directory. It confirms which files were successfully processed and links back to the archive database for full details.

```json
{
  "version": 2,
  "pixe_version": "0.10.0",
  "run_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "pixe_run": "2026-03-06T10:30:00Z",
  "algorithm": "sha1",
  "destination": "/path/to/dirB",
  "files": [
    {
      "path": "IMG_0001.jpg",
      "checksum": "7d97e98f8af710c7e7fe703abc8f639e0ee507c4",
      "destination": "2021/12-Dec/20211225_062223_7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg",
      "verified_at": "2026-03-06T10:30:03Z"
    }
  ]
}
```

**Changes from v1:**
- `version` bumped to `2`.
- `run_id` field added — the UUID of the run in the archive database. This allows a user to query the database for full details: `SELECT * FROM files WHERE run_id = '<run_id>'`.

The ledger is still written atomically (write `.tmp`, then rename) and is the **only file** Pixe writes to `dirA`.

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

### 9.1 New Flag

| Command | Flag | Default | Description |
|---|---|---|---|
| `pixe sort` | `--db-path` | (auto-detected) | Explicit path to the SQLite database file. Overrides all automatic location logic. |

This flag is also supported via config file (`db_path`) and environment variable (`PIXE_DB_PATH`).

### 9.2 Updated `pixe resume`

The `resume` command now locates the database via the same discovery chain (flag → `dbpath` marker → default local path) and queries for the interrupted run's incomplete files.

---

## 10. Open Questions & Future Considerations

These items are explicitly **out of scope** for the current build but are acknowledged for future planning:

1. **Sidecar files** (`.xmp`, `.aae`) — Should they follow their parent media file?
2. **RAW formats** (`.cr2`, `.arw`, `.dng`) — Natural candidates for new filetype modules.
3. **Web UI / TUI** — Progress visualization beyond CLI output.
4. **Cloud storage targets** — `dirB` on S3, GCS, etc.
5. **GPS/location-based organization** — Subdirectories by location in addition to date.
6. **`pixe query` CLI command** — Expose the database query patterns (Section 8.4) as user-facing subcommands (e.g., `pixe query --duplicates`, `pixe query --errors`, `pixe query --from-source <path>`).
7. **Database compaction/maintenance** — `VACUUM` command exposure for long-lived archives.
8. **Multi-archive federation** — Querying across multiple `dirB` databases from a single command.
