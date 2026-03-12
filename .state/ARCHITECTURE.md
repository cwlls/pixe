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
| **Glob Matching** | `bmatcuk/doublestar/v4` | `**` recursive globs, `{alt}` alternatives; superset of `filepath.Match`; see Section 4.11 |
| **TUI Framework** | `charmbracelet/bubbletea` | Elm-architecture TUI framework; powers CLI progress bars (Section 12) and interactive TUI (Section 13) |
| **TUI Components** | `charmbracelet/bubbles` | Pre-built components: progress bars, viewports, spinners, key bindings, help |
| **TUI Styling** | `charmbracelet/lipgloss` | Terminal styling with adaptive colors; respects user's terminal color scheme |

---

## 2.1 v2.3.0 Feature Additions

The following features were added in v2.3.0 (2026-03-12):

### New File Format Handlers

Three new file format handlers extend archive support:

- **PNG Handler** (`internal/handler/png/`) — Supports `.png` files with EXIF extraction from `eXIf` chunks (PNG 1.5+) or `tEXt` chunks. Falls back to Ansel Adams date when no metadata is present. Uses full-file hashing (PNG compression is unstable across re-encoding). Declares `MetadataSidecar` capability for XMP sidecar tagging.

- **ORF Handler** (`internal/handler/orf/`) — Supports Olympus `.orf` RAW files. TIFF-based format using `tiffraw.Base` for shared logic. Detects both Olympus-specific `IIRO` header and standard TIFF LE signatures.

- **RW2 Handler** (`internal/handler/rw2/`) — Supports Panasonic `.rw2` RAW files. TIFF-based format using `tiffraw.Base`. Detects Panasonic-specific `49 49 55 00` header signature.

All three handlers are registered in `cmd/helpers.go` `buildRegistry()` and tested via `handlertest.RunSuite()`.

### Colorized Terminal Output

A new `Formatter` type in `internal/pipeline/format.go` applies Lip Gloss styling to status verbs when stdout is a TTY:

- `COPY` → green (`#85e89d` dark, `#22863a` light)
- `DUPE` → yellow (`#ffdf5d` dark, `#b08800` light)
- `ERR` → red bold (`#f97583` dark, `#cb2431` light)
- `SKIP` → dim gray (`#6a737d` dark, `#959da5` light)

TTY detection uses `isatty.IsTerminal(os.Stdout.Fd())`. The `NO_COLOR` environment variable is respected (standard convention). Color is disabled in `--quiet` mode (no per-file output to colorize).

### Verbosity Levels

Two new persistent flags on `rootCmd`:

- `--quiet` / `-q` — Suppresses per-file output (`COPY`/`SKIP`/`DUPE`/`ERR` lines). Only the final summary is emitted. Maps to `cfg.Verbosity = -1`.
- `--verbose` / `-v` — Adds per-stage timing info to output (extract, hash, copy, verify, tag durations) and worker utilization stats. Maps to `cfg.Verbosity = 1`.

These flags are mutually exclusive. Normal mode (`Verbosity = 0`) is the default.

### Date Filter Flags

Two new flags on `pixe sort`:

- `--since <YYYY-MM-DD>` — Skips files with capture date before the specified date (inclusive).
- `--before <YYYY-MM-DD>` — Skips files with capture date after the specified date (inclusive). Internally parsed as end-of-day (`23:59:59.999`).

Both can be combined for a date range. Skipped files emit `SKIP <filename> -> outside date range` and are recorded in the DB with `status = 'skipped'` and `skip_reason = 'outside date range'`. The Ansel Adams fallback date (`1902-02-20`) is subject to filtering.

### Config Auto-Discovery

When `pixe sort` is invoked, the source directory (`dirA`) is checked for a `.pixe.yaml` file. If found, its configuration is merged with the global config, respecting the priority chain:

**CLI flags > source-local `.pixe.yaml` > profile config > global `.pixe.yaml` > env vars > defaults**

This is implemented via a two-phase approach in `cmd/sort.go`: after `resolveConfig()` returns, the source directory is checked, and if a local config exists, `mergeSourceConfig()` merges eligible keys (those not explicitly set via CLI flags) into the global Viper instance, then `resolveConfig()` is called again.

### Config Profiles

A new `--profile <name>` persistent flag on `rootCmd` enables loading named configuration profiles:

```bash
pixe sort --profile family --dest ./archive
```

Profiles are searched in:
1. `~/.pixe/profiles/<name>.yaml`
2. `$XDG_CONFIG_HOME/pixe/profiles/<name>.yaml`

Profile files use the same YAML format as `.pixe.yaml`. The `loadProfile()` helper in `cmd/helpers.go` handles discovery and merging. Profiles are merged after source-local config in the priority chain.

### `pixe stats` Command

A new `pixe stats` command provides an archive dashboard:

```bash
pixe stats --dir ./archive
```

Displays:
- Total files and size
- Duplicate and error counts (broken down by type: failed, mismatch, tag_failed)
- Skipped file count
- Date range (earliest to latest capture date)
- Last import timestamp
- Total run count
- Format breakdown (file counts by extension, sorted by frequency)

Backed by new database methods in `internal/archivedb/queries.go`:
- `FormatBreakdown() (map[string]int, error)` — Returns file counts grouped by extension
- `LastRunDate() (*time.Time, error)` — Returns the timestamp of the most recent completed run

The command supports `--json` output for scripting.

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
│ IMG_1234.aae     │   extract   │   20220202_123101_447d3060...jpg      │
│ DSC_5678.nef     │   hash      │   20220202_123101_447d3060...jpg.xmp  │
│ DSC_5678.xmp     │   copy      │ 2022/03-Mar/                         │
│ VID_0010.mp4     │   verify    │   20220316_232122_321c7d6f...nef      │
│ notes.txt        │   carry     │   20220316_232122_321c7d6f...nef.xmp  │
│ subfolder/       │   tag       │ duplicates/                          │
│   IMG_5678.jpg   │             │   20260306_103000/                    │
│                  │             │     2022/02-Feb/                     │
│ .pixe_ledger.json│  (ignored)  │       20220202_123101_447d...jpg      │
└──────────────────┘             │ .pixe/                               │
                                 │   pixe.db  (or dbpath marker)        │
  stdout:                        └──────────────────────────────────────┘
  COPY IMG_0001.jpg -> 2021/...
  COPY IMG_1234.jpg -> 2022/...
       +sidecar IMG_1234.aae -> 2022/...aae
  COPY DSC_5678.nef -> 2022/...
       +sidecar DSC_5678.xmp -> 2022/...xmp (merge tags)
  DUPE IMG_0002.jpg -> matches 2022/02-Feb/20220202...jpg
  ERR  notes.txt    -> unsupported format: .txt
```

### 4.2 Pipeline Stages

Each file passes through these stages, tracked in the archive database:

```
pending → extracted → hashed → copied → verified → sidecars carried → tagged → complete
                                    ↓         ↓                          ↓
                                  failed   mismatch                  tag_failed
```

1. **Pending** — File discovered in `dirA`, not yet processed.
2. **Extracted** — Filetype module has read the file, extracted the capture date, and identified the hashable data region.
3. **Hashed** — Checksum computed over the media payload (data only, excluding metadata).
4. **Copied** — File written to a temporary file (`.<filename>.pixe-tmp`) in the destination directory within `dirB`. The file does not yet exist at its canonical path. See Section 4.10 for the atomic copy design.
5. **Verified** — Temporary file re-read and checksum recomputed; matches the source hash. On success, the temp file is atomically renamed to its canonical destination path. On mismatch, the temp file is deleted (the source in `dirA` is untouched and the file can be reprocessed). See Section 4.10.
6. **Sidecars Carried** — Pre-existing sidecar files (`.aae`, `.xmp`) associated with the media file during discovery are copied to the destination directory, renamed to match the parent's new filename. Sidecar carry failure is non-fatal — the media file is already safely in place. Skipped when `--no-carry-sidecars` is active or when the file has no associated sidecars. See Section 4.12.
7. **Tagged** — Optional metadata persisted to the destination. The pipeline queries the handler's `MetadataSupport()` capability to determine the strategy:
   - **`MetadataEmbed`** → Tags written directly into the destination file (e.g., JPEG EXIF).
   - **`MetadataSidecar`** → XMP sidecar file written alongside the destination file (e.g., `*.arw.xmp`). If a source `.xmp` sidecar was carried in the previous stage, Pixe merges tags into it instead of generating a new file. See Section 4.12.6.
   - **`MetadataNone`** → Tagging skipped entirely; stage advances directly to complete.
   - If no tags are configured (`tags.IsEmpty()`), the stage is skipped regardless of capability.
8. **Complete** — All operations successful. Recorded in ledger.

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
```

Patterns from the CLI flag and config file are **merged** (additive). The hardcoded ledger ignore is always present in addition to user patterns.

#### Matching Behavior

- Patterns are matched against the **filename only** (not the full path) for files in the top-level directory.
- When `--recursive` is enabled, patterns are matched against the **relative path from `dirA`** as well as the filename. This allows patterns like `subfolder/*.tmp` or `**/Thumbs.db`.
- Matching uses `bmatcuk/doublestar/v4` semantics, which is a superset of `filepath.Match` and adds `**` recursive globs, `{alt1,alt2}` alternatives, and character classes.
- Patterns ending with `/` match **directories only** and cause the entire directory tree to be skipped.
- `.pixeignore` files placed in source directories are loaded automatically and their patterns scoped to that directory subtree.

> **Full specification:** See Section 4.11 for the complete implementation details of the enhanced ignore system.

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

**Interaction with source sidecar carry:** When the pipeline has carried a pre-existing `.xmp` sidecar from `dirA` (see Section 4.12), the tagging stage **merges** Pixe's tags into the carried sidecar rather than generating a new one from the template. This preserves existing XMP data (develop settings, ratings, keywords, etc.) while injecting Pixe's copyright and ownership fields. See Section 4.12.6 for the full merge rules and overwrite policy.

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

### 4.11 Enhanced Ignore System (`.gitignore`-Style)

Pixe's ignore system is enhanced with three capabilities that together bring it to `.gitignore`-level expressiveness: `**` recursive glob support via the `doublestar` library, directory-level ignore patterns via trailing-slash convention, and `.pixeignore` files that travel with the source directory.

#### 4.11.1 Pattern Matching: `doublestar` Library

The `filepath.Match` function used by the current `Matcher` does not support `**` recursive globs or `{alt1,alt2}` alternatives. Pixe replaces it with `bmatcuk/doublestar/v4`, a well-established Go library (MIT, v4 stable, 690+ importers, zero dependencies) that provides a drop-in replacement with full `**` support.

**Dependency addition:**

```
go get github.com/bmatcuk/doublestar/v4
```

**Matching function replacement:**

| Before | After |
|---|---|
| `filepath.Match(pattern, name)` | `doublestar.Match(pattern, name)` |

The `doublestar.Match` function uses forward slashes as path separators (matching Go's `path` package convention). Since Pixe's `relPath` values use `filepath.Separator`, patterns and paths are normalized to forward slashes before matching via `filepath.ToSlash()`.

**New pattern capabilities:**

| Pattern | Matches | Example |
|---|---|---|
| `**/*.tmp` | All `.tmp` files at any depth | `a/b/c/scratch.tmp` |
| `**/Thumbs.db` | `Thumbs.db` at any depth | `vacation/Thumbs.db` |
| `backups/**` | All files under `backups/` | `backups/old/photo.jpg` |
| `a/**/b` | `b` under `a/` at any depth | `a/x/y/b` |
| `*.{txt,log}` | Files with `.txt` or `.log` extension | `notes.txt`, `app.log` |
| `[Tt]humbs.db` | Case-variant matches | `Thumbs.db`, `thumbs.db` |

All existing patterns (`*.txt`, `.DS_Store`, `subdir/*.tmp`) continue to work identically — `doublestar.Match` is a superset of `filepath.Match`.

#### 4.11.2 Directory-Level Ignore Patterns

Following `.gitignore` convention, a pattern ending with `/` matches only directories, not files. When a directory matches, the walk skips it entirely — `filepath.SkipDir` is returned, avoiding the I/O cost of descending into the ignored tree.

**Trailing-slash semantics:**

| Pattern | Matches | Does NOT match |
|---|---|---|
| `node_modules/` | Directory named `node_modules` (and all contents) | A file named `node_modules` |
| `.git/` | Directory named `.git` (and all contents) | A file named `.git` |
| `**/cache/` | Directory named `cache` at any depth | A file named `cache` |
| `backups/old/` | Directory `old` under `backups/` | A file `backups/old` |

**Without trailing slash**, a pattern matches both files and directories (existing behavior):

| Pattern | Matches |
|---|---|
| `node_modules` | Both a file and a directory named `node_modules` |
| `*.tmp` | Files named `*.tmp` at the current level |

**Implementation in `Matcher`:** The `Matcher` struct gains a new method `MatchDir(dirname, relDirPath string) bool` that is called by `discovery.Walk` when encountering a directory entry. This method:

1. Iterates over all patterns.
2. For patterns ending with `/`, strips the trailing slash and matches the directory name/path against the remainder using `doublestar.Match`.
3. For patterns containing `**` without a trailing slash, checks whether the pattern would match all contents of the directory (e.g., `backups/**` implies skipping the `backups/` directory).
4. Returns `true` if the directory should be skipped.

The existing `Match(filename, relPath string) bool` method continues to handle file-level matching. It ignores patterns ending with `/` (those are directory-only).

**Integration with `discovery.Walk`:** The directory handling block in `Walk` (currently lines 73-83) is updated:

```go
// --- Directory handling ---
if d.IsDir() {
    if path == dirA {
        return nil // always enter the root
    }
    if strings.HasPrefix(name, ".") {
        return filepath.SkipDir // always skip dot-directories
    }
    if !opts.Recursive {
        return filepath.SkipDir // non-recursive: skip all subdirs
    }
    // NEW: check user-configured directory ignore patterns.
    if opts.Ignore != nil {
        relDir, _ := filepath.Rel(dirA, path)
        if opts.Ignore.MatchDir(name, relDir) {
            return filepath.SkipDir
        }
    }
    return nil // recursive: descend
}
```

**Interaction with hardcoded dot-directory skip:** The existing hardcoded `strings.HasPrefix(name, ".")` check runs *before* the user ignore check. This means dot-directories like `.git/` are already skipped regardless of user patterns. The user pattern `".git/"` would be redundant but harmless. The hardcoded check is retained for safety — it predates the ignore system and should remain as a belt-and-suspenders guarantee.

#### 4.11.3 `.pixeignore` File

A `.pixeignore` file placed in a source directory specifies ignore patterns that travel with the photos. This is analogous to `.gitignore` — patterns are scoped to the directory containing the file and its subdirectories.

**File format:**

```
# Lines starting with # are comments.
# Blank lines are ignored.
# Patterns follow the same syntax as --ignore flags.

*.txt
*.log
.DS_Store
Thumbs.db
node_modules/
**/cache/
backups/**
```

- One pattern per line.
- Lines starting with `#` are comments.
- Blank lines are separators (ignored).
- Leading and trailing whitespace is trimmed.
- Patterns use the same `doublestar` syntax as `--ignore` flags, including `**`, `{alt}`, character classes, and trailing-slash directory matching.

**Negation patterns (`!`) are NOT supported.** `.gitignore` supports `!` prefix to re-include previously excluded files. Pixe omits this feature for simplicity — the ignore system is additive only. All patterns from all sources are merged into a single exclusion set. If a file matches any pattern from any source, it is ignored. This avoids the complexity of ordered pattern precedence across multiple sources.

**Nested `.pixeignore` files:** Following `.gitignore` convention, `.pixeignore` files in subdirectories of `dirA` are respected during recursive walks. Each `.pixeignore` file applies to files in its directory and all descendant directories. Patterns in a nested `.pixeignore` are relative to the directory containing that file.

**Loading strategy:** `.pixeignore` files are discovered and loaded lazily during `discovery.Walk`. When the walk enters a directory, it checks for a `.pixeignore` file and, if found, parses it and pushes its patterns onto a stack. When the walk leaves the directory, the patterns are popped. This mirrors Git's approach and avoids pre-scanning the entire tree.

**Implementation approach:** The `Matcher` struct is extended to support a stack of pattern scopes:

```go
type patternScope struct {
    basePath string   // relative path from dirA to the directory containing .pixeignore
    patterns []string // patterns from this .pixeignore file
}

type Matcher struct {
    global []string        // patterns from CLI flags, config file, hardcoded
    scopes []patternScope  // stack of .pixeignore scopes (pushed/popped during walk)
}
```

When matching a file at `relPath`, the matcher checks:
1. All `global` patterns (against both filename and relPath, as today).
2. All active `scopes` — for each scope, the file's path relative to the scope's `basePath` is computed, and the scope's patterns are matched against that relative path.

**Priority chain (all additive — no overrides, no negation):**

| Priority | Source | Scope |
|---|---|---|
| 1 (always active) | Hardcoded: `.pixe_ledger.json`, `.pixeignore` | Filename match at any depth |
| 2 (if file exists) | `.pixeignore` in `dirA` and subdirectories | Relative to containing directory |
| 3 (if configured) | Config file `ignore:` list (`.pixe.yaml`) | Global — all files |
| 4 (if specified) | CLI `--ignore` flags | Global — all files |

All sources are **merged additively**. A file is ignored if it matches any pattern from any source. There is no override mechanism — a pattern cannot "un-ignore" a file excluded by another source.

**Auto-ignored files:** The `.pixeignore` file itself is added to the hardcoded ignore list alongside `.pixe_ledger.json`. This prevents `.pixeignore` from appearing as "unrecognized" in `pixe status` output or being processed by the sort pipeline.

```go
const (
    ledgerFilename    = ".pixe_ledger.json"
    pixeignoreFilename = ".pixeignore"
)
```

Both are matched by filename at any depth (same as the existing ledger ignore).

#### 4.11.4 Updated `Matcher` API

The `Matcher` struct in `internal/ignore/` is updated with the following API:

```go
// New creates a Matcher from global patterns (CLI flags + config file).
// The hardcoded ignores (.pixe_ledger.json, .pixeignore) are always active.
func New(patterns []string) *Matcher

// Match reports whether a file should be ignored.
// filename is the base name; relPath is the path relative to dirA.
func (m *Matcher) Match(filename, relPath string) bool

// MatchDir reports whether a directory should be skipped entirely.
// dirname is the base name; relDirPath is the path relative to dirA.
func (m *Matcher) MatchDir(dirname, relDirPath string) bool

// PushScope loads a .pixeignore file and pushes its patterns onto the scope stack.
// basePath is the relative path from dirA to the directory containing the file.
// Returns true if a .pixeignore file was found and loaded.
func (m *Matcher) PushScope(basePath string, pixeignorePath string) bool

// PopScope removes the most recently pushed scope from the stack.
func (m *Matcher) PopScope()
```

**Thread safety:** The `Matcher` is used by a single goroutine (the `discovery.Walk` caller). The scope stack is not accessed concurrently. No mutex is needed.

#### 4.11.5 Updated Section References

- **Section 4.4 (Ignore List):** The description of matching behavior is superseded by this section. Section 4.4 remains as the conceptual introduction; this section (4.11) provides the full implementation specification.
- **Section 2 (Technical Stack):** Add `bmatcuk/doublestar/v4` to the dependency table.
- **Section 4.9 (Recursive Source Processing):** The ignore pattern behavior in recursive mode now includes `.pixeignore` scoping and directory-level skipping.

#### 4.11.6 Examples

**Config file (`.pixe.yaml`):**

```yaml
ignore:
  - "*.txt"
  - "*.log"
  - ".DS_Store"
  - "Thumbs.db"
  - "node_modules/"
  - "**/cache/"
```

**CLI:**

```bash
pixe sort --dest ./archive --ignore "*.txt" --ignore "node_modules/" --ignore "**/cache/"
```

**`.pixeignore` file in `dirA`:**

```
# OS junk
.DS_Store
Thumbs.db

# Build artifacts
node_modules/
dist/
*.tmp

# Deep patterns
**/cache/
**/.thumbnails/
```

**`.pixeignore` file in `dirA/raw-exports/`:**

```
# Ignore Lightroom preview caches in this subtree
Previews.lrdata/
*.lrprev
```

**Combined effect:** When running `pixe sort --source ./photos --dest ./archive --recursive`, patterns from all sources are merged. A file at `photos/raw-exports/Previews.lrdata/thumb.lrprev` would be ignored by both the nested `.pixeignore` patterns (`Previews.lrdata/` and `*.lrprev`).

### 4.12 Source Sidecar Carry

When sorting media from `dirA`, Pixe detects and carries pre-existing **sidecar files** that sit alongside source media files. Carried sidecars are copied to the same destination directory as their parent media file, renamed to match the parent's new filename, and treated as attachments — not independently tracked files. This preserves companion data (Apple edit instructions, Lightroom develop settings, ratings, keywords, etc.) that would otherwise be silently left behind.

**Default:** Enabled. Opt out with `--no-carry-sidecars` (config key: `carry_sidecars: false`).

#### 4.12.1 Supported Sidecar Extensions

The initial set of recognized sidecar extensions is:

| Extension | Origin | Content |
|---|---|---|
| `.aae` | Apple (iOS/macOS Photos) | Non-destructive edit instructions (XML plist) |
| `.xmp` | Adobe (Lightroom, Camera Raw, Capture One, etc.) | XMP metadata packet — develop settings, ratings, keywords, GPS, copyright, etc. |

This set is intentionally small and may be expanded in future versions (e.g., `.thm`, `.pp3`, `.dop`). The supported extensions are defined as a package-level constant slice, not user-configurable — adding new extensions is a code change, not a config change.

#### 4.12.2 Association Rule

Sidecars are associated with their parent media file by **stem matching within the same directory**:

- A file `IMG_1234.xmp` in the same directory as `IMG_1234.HEIC` is associated with the HEIC file.
- A file `IMG_1234.HEIC.xmp` (Adobe full-extension convention) in the same directory as `IMG_1234.HEIC` is also associated with the HEIC file.
- Association is **case-insensitive** on the stem (e.g., `img_1234.xmp` matches `IMG_1234.HEIC`).
- A sidecar can only associate with **one** parent. If multiple media files share a stem (e.g., `IMG_1234.HEIC` and `IMG_1234.JPG`), the sidecar associates with whichever media file the discovery phase encounters first. This is an inherent ambiguity in stem-based matching; the user can resolve it by using the full-extension convention (`IMG_1234.HEIC.xmp`) which is unambiguous.
- Sidecars without a matching parent media file in the same directory are **skipped** with reason `"orphan sidecar: no matching media file"` and reported in stdout and ledger.

#### 4.12.3 Discovery Integration

Sidecar detection happens during the **discovery phase** (`internal/discovery/`), after handler-based file classification:

1. `Walk()` first classifies all files as usual (discovered, skipped, or ignored).
2. A second pass scans the skipped files for recognized sidecar extensions.
3. For each sidecar candidate, `Walk()` searches the discovered files for a parent with a matching stem (or full filename for the `<name>.<ext>.xmp` convention) in the same directory.
4. Matched sidecars are **removed from the skipped list** and attached to their parent `DiscoveredFile`.
5. Unmatched sidecars remain in the skipped list with reason `"orphan sidecar: no matching media file"`.

The `DiscoveredFile` struct gains a new field:

```go
type DiscoveredFile struct {
    Path     string                  // absolute path for file I/O
    RelPath  string                  // relative path from dirA for display and ledger
    Handler  domain.FileTypeHandler  // resolved handler
    Sidecars []SidecarFile           // pre-existing sidecars from dirA (may be empty)
}

type SidecarFile struct {
    Path    string // absolute path in dirA
    RelPath string // relative path from dirA
    Ext     string // normalized lowercase extension (e.g., ".aae", ".xmp")
}
```

When `--no-carry-sidecars` is active, the second pass is skipped entirely. Sidecar files remain in the skipped list with their original reason (`"unsupported format: .xmp"`, etc.) — the same behavior as before this feature existed.

#### 4.12.4 Destination Naming

Carried sidecars are renamed to match the parent media file's destination filename, using the **full-extension convention** (Adobe standard):

```
<parent_dest_filename>.<sidecar_ext>
```

Examples:

| Source (dirA) | Parent destination (dirB) | Sidecar destination (dirB) |
|---|---|---|
| `IMG_1234.aae` | `2021/12-Dec/20211225_062223_7d97e98f.heic` | `2021/12-Dec/20211225_062223_7d97e98f.heic.aae` |
| `IMG_1234.xmp` | `2021/12-Dec/20211225_062223_7d97e98f.heic` | `2021/12-Dec/20211225_062223_7d97e98f.heic.xmp` |
| `IMG_1234.HEIC.xmp` | `2021/12-Dec/20211225_062223_7d97e98f.heic` | `2021/12-Dec/20211225_062223_7d97e98f.heic.xmp` |
| `DSC_5678.xmp` | `2022/03-Mar/20220316_232122_321c7d6f.nef` | `2022/03-Mar/20220316_232122_321c7d6f.nef.xmp` |

This ensures unambiguous association in the destination archive and compatibility with Adobe tools that expect the `<filename>.<ext>.xmp` convention.

#### 4.12.5 Pipeline Integration

Sidecar carry slots into the existing pipeline between **verify** and **tag**:

```
pending → extracted → hashed → copied → verified → sidecars carried → tagged → complete
```

For each `DiscoveredFile` with a non-empty `Sidecars` slice:

1. **After verify succeeds** (the parent media file is at its canonical destination path), each sidecar is copied to its derived destination path.
2. Sidecar copy uses a **simple file copy** — no temp-file-then-rename atomicity, no hash verification. Sidecars are small metadata files, not irreplaceable media payloads. The source in `dirA` is always available for re-copy. This matches the treatment of Pixe-generated XMP sidecars (Section 4.8.2: "Sidecars are generated artifacts, not copied files. They are not subject to the copy-then-verify integrity flow").
3. **Sidecar copy failure is non-fatal.** If a sidecar fails to copy, the pipeline logs a warning to stdout and continues. The parent media file is already safely copied and verified. The file's status is **not** downgraded to `tag_failed` for a sidecar copy failure — the media file is the primary artifact. A warning line is emitted: `WARN <source_filename> -> sidecar carry failed: <sidecar_relpath>: <error>`.
4. **After sidecars are carried**, the tag stage proceeds as usual. For XMP sidecars specifically, the tag stage behavior depends on whether a source `.xmp` was carried — see Section 4.12.6.

#### 4.12.6 XMP Sidecar Merge (Tag Interaction)

When a source `.xmp` sidecar has been carried to the destination AND the user has configured metadata tags (`--copyright` and/or `--camera-owner`), the tagging stage **merges** Pixe's tags into the carried `.xmp` rather than generating a new one. This preserves the user's existing XMP data (develop settings, ratings, keywords, etc.) while adding Pixe's metadata.

**Interaction matrix:**

| Source has `.xmp`? | Tags configured? | Behavior |
|---|---|---|
| No | No | Nothing. No sidecar carried, no sidecar generated. |
| No | Yes | Pixe generates `.xmp` from template (existing behavior, unchanged). |
| Yes | No | Source `.xmp` carried as-is. No tag injection. |
| Yes | Yes | Source `.xmp` carried, then Pixe merges tags into it (see merge rules below). |

**Merge rules:**

The `xmp` package gains a `MergeTags(sidecarPath string, tags MetadataTags, overwrite bool) error` function that:

1. Parses the existing XMP file at `sidecarPath`.
2. For each configured tag (`dc:rights`/`xmpRights:Marked`, `aux:OwnerName`):
   - If the field **does not exist** in the XMP: inject it (add the element and, if necessary, the namespace declaration).
   - If the field **already exists** in the XMP and `overwrite` is `false` (default): **leave the existing value intact**. The source is authoritative.
   - If the field **already exists** in the XMP and `overwrite` is `true`: **replace** the existing value with Pixe's configured value.
3. Adds missing namespace declarations (`xmlns:dc`, `xmlns:xmpRights`, `xmlns:aux`) to the `rdf:Description` element as needed. Existing namespace declarations are preserved.
4. Writes the modified XMP back to the same path (atomic write via temp file + rename, same as `WriteSidecar`).

**Overwrite control:**

- **Default:** Source values are authoritative (`overwrite: false`). Pixe only fills in missing fields.
- **Override:** `--overwrite-sidecar-tags` flag (config key: `overwrite_sidecar_tags: true`) causes Pixe to replace existing values with its configured tags.

**`.aae` files are never modified.** The merge behavior applies exclusively to `.xmp` sidecars. Apple `.aae` files are carried verbatim — Pixe has no reason to write into Apple edit instruction plists.

#### 4.12.7 Duplicate Handling

Sidecars follow their parent media file's duplicate routing:

- **Default mode (copy duplicates):** If the parent is routed to `duplicates/<run_timestamp>/...`, its sidecars are copied there too, with the same naming convention.
- **Skip duplicates mode (`--skip-duplicates`):** If the parent is skipped (no copy), sidecars are also skipped. No sidecar bytes are written to `dirB`.

#### 4.12.8 Database & Ledger Tracking

Carried sidecars are **attachments to their parent file** — they do not get their own rows in the `files` table. This is consistent with how Pixe-generated XMP sidecars are already handled (they are filesystem artifacts with no DB presence).

**Schema addition:** A new nullable text column on the `files` table:

```sql
ALTER TABLE files ADD COLUMN carried_sidecars TEXT;
```

The `carried_sidecars` column stores a JSON array of the sidecar destination relative paths, or `NULL` when no sidecars were carried. Example value: `["2021/12-Dec/20211225_062223_7d97e98f.heic.aae","2021/12-Dec/20211225_062223_7d97e98f.heic.xmp"]`.

This is sufficient for auditability (knowing which sidecars were carried for a given file) without the overhead of a separate table or per-sidecar rows. The column is additive (`ALTER TABLE ADD COLUMN`) and nullable, so existing databases migrate seamlessly.

**Ledger:** The `LedgerEntry` gains an optional `sidecars` field (omitted when empty, consistent with `omitempty` convention):

```json
{"path":"IMG_1234.HEIC","status":"complete","checksum":"7d97e98f...","destination":"2021/12-Dec/20211225_062223_7d97e98f.heic","sidecars":["2021/12-Dec/20211225_062223_7d97e98f.heic.aae","2021/12-Dec/20211225_062223_7d97e98f.heic.xmp"]}
```

#### 4.12.9 Dry-Run Mode

In `--dry-run` mode, sidecar association is still performed during discovery (so the user can see what would be carried), but no sidecar files are copied or modified. The stdout output includes the sidecar association:

```
COPY IMG_1234.HEIC -> 2021/12-Dec/20211225_062223_7d97e98f.heic (dry run)
     +sidecar IMG_1234.aae -> 2021/12-Dec/20211225_062223_7d97e98f.heic.aae
     +sidecar IMG_1234.xmp -> 2021/12-Dec/20211225_062223_7d97e98f.heic.xmp (merge tags)
```

#### 4.12.10 `pixe clean` Interaction

The `pixe clean` command (Section 7.5) already handles orphaned XMP sidecars from interrupted runs. Carried sidecars that were partially written during an interrupted run are **not** automatically cleaned by `pixe clean` — they are indistinguishable from intentionally carried sidecars without additional bookkeeping. This is acceptable because:

- Sidecar files are small (typically < 100 KB).
- An interrupted carry leaves a complete or partial sidecar file that is harmless alongside the media file.
- Re-running `pixe sort` will re-carry the sidecar correctly (the parent media file will be skipped as "previously imported", but the sidecar carry is idempotent if the parent's destination already exists).

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
├── handlertest/          ← shared test suite for FileTypeHandler implementations
│   └── suite.go          ← RunSuite() function: 10-test harness for TIFF-based handlers
├── tiffraw/              ← shared base for TIFF-based RAW formats
│   └── tiffraw.go        ← Base struct with common ExtractDate, HashableReader, WriteMetadataTags
├── dng/
│   ├── dng.go            ← thin wrapper: Extensions, MagicBytes, Detect → delegates to tiffraw.Base
│   └── dng_test.go       ← uses handlertest.RunSuite()
├── nef/
│   ├── nef.go
│   └── nef_test.go       ← uses handlertest.RunSuite()
├── cr2/
│   ├── cr2.go
│   └── cr2_test.go       ← uses handlertest.RunSuite()
├── cr3/
│   ├── cr3.go            ← standalone ISOBMFF-based handler (not using tiffraw)
│   └── cr3_test.go
├── pef/
│   ├── pef.go
│   └── pef_test.go       ← uses handlertest.RunSuite()
├── arw/
│   ├── arw.go
│   └── arw_test.go       ← uses handlertest.RunSuite()
├── jpeg/
│   ├── jpeg.go
│   └── jpeg_test.go
├── heic/
│   ├── heic.go
│   └── heic_test.go
├── mp4/
│   ├── mp4.go
│   └── mp4_test.go
└── tiffraw/
    ├── tiffraw.go
    └── tiffraw_test.go
```

#### 6.4.3 Shared Test Suite: `handlertest`

The `handlertest` package provides a reusable test harness (`RunSuite()` function) that exercises the 10 standard behaviors shared by all `FileTypeHandler` implementations. This eliminates ~900 lines of test duplication across the five TIFF-based RAW handler test files (DNG, NEF, CR2, PEF, ARW).

**The 10-test suite covers:**

1. `Extensions()` — handler returns expected file extensions
2. `MagicBytes()` — handler returns expected magic byte signatures
3. `Detect(validFile)` — detection succeeds for valid files
4. `Detect(wrongExtension)` — detection fails for files with wrong extension
5. `Detect(wrongMagic)` — detection fails for files with wrong magic bytes
6. `ExtractDate(noEXIF)` — fallback to Ansel Adams date when no EXIF present
7. `HashableReader(returnsData)` — reader returns non-empty data
8. `HashableReader(deterministic)` — two reads return identical data
9. `MetadataSupport()` — handler declares correct metadata capability
10. `WriteMetadataTags(noop)` — tagging returns no error

**Usage pattern (per-handler test file):**

```go
// internal/handler/dng/dng_test.go
func TestHandler(t *testing.T) {
    handlertest.RunSuite(t, handlertest.SuiteConfig{
        NewHandler: func() domain.FileTypeHandler { return dng.New() },
        Extensions: []string{".dng"},
        MagicSignatures: []domain.MagicSignature{
            {Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}}, // little-endian TIFF
        },
        BuildFakeFile: buildFakeDNG,
        WrongExtension: "test.jpg",
        MetadataCapability: domain.MetadataSidecar,
    })
}
```

Each TIFF-based handler test file (DNG, NEF, CR2, PEF, ARW) calls `RunSuite()` with handler-specific configuration. The suite runs all 10 subtests in parallel (via `t.Parallel()` on each subtest) using `t.TempDir()` for isolated file I/O.

#### 6.4.4 Shared Base: `tiffraw.Base`

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

#### 6.4.5 Date Extraction (TIFF-based RAW)

All TIFF-based RAW formats store standard EXIF metadata in IFD0 and sub-IFDs. Date extraction follows the same fallback chain used by JPEG:

1. **EXIF `DateTimeOriginal`** (tag 0x9003) — preferred
2. **EXIF `DateTime`** (tag 0x0132, IFD0) — fallback
3. **Ansel Adams date** (`1902-02-20`) — sentinel for undated files

The TIFF container is parsed to locate the EXIF IFDs, then standard EXIF tag reading applies. A pure-Go TIFF parser (e.g., `golang.org/x/image/tiff` or equivalent) provides the IFD traversal.

#### 6.4.6 Date Extraction (CR3)

CR3 files use the ISOBMFF container format, the same box-based structure used by HEIC and MP4. Date extraction follows the ISOBMFF approach already established by the HEIC handler:

1. Parse the ISOBMFF container to locate the EXIF blob (typically within a `moov` → `meta` → `xml ` or `Exif` box path, depending on the Canon implementation).
2. Extract the raw EXIF bytes from the container.
3. Parse with the standard EXIF library and apply the same fallback chain: `DateTimeOriginal` → `DateTime` → Ansel Adams date.

#### 6.4.7 Hashable Region: Raw Sensor Data

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

#### 6.4.8 Metadata Capability: XMP Sidecar

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

#### 6.4.9 Magic Byte Signatures

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

#### 6.4.10 Handler Registration

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
| `--no-carry-sidecars` | | `false` | Disable source sidecar carry. When set, pre-existing `.aae` and `.xmp` files in `dirA` are not carried alongside their parent media file. See Section 4.12. |
| `--overwrite-sidecar-tags` | | `false` | When merging Pixe tags into a carried `.xmp` sidecar, overwrite existing values. Default preserves source values (source is authoritative). See Section 4.12.6. |

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
carry_sidecars: true
overwrite_sidecar_tags: false
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
| `sidecars` | `copy` (when sidecars carried) | JSON array of relative paths within `dirB` where carried sidecar files were written. Absent when no sidecars were carried or when `--no-carry-sidecars` is active. See Section 4.12.8. |
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
│       └── 20211225_071500_a3b4c5d6...arw.xmp   ← XMP sidecar (Pixe-generated or carried from source)
├── 2022/
│   ├── 02-Feb/
│   │   ├── 20220202_123101_447d3060...jpg
│   │   ├── 20220202_130000_e5f6a7b8...dng
│   │   ├── 20220202_130000_e5f6a7b8...dng.xmp
│   │   ├── 20220202_143000_b1c2d3e4...heic      ← HEIC with carried sidecars
│   │   ├── 20220202_143000_b1c2d3e4...heic.aae  ← carried .aae (Apple edits)
│   │   └── 20220202_143000_b1c2d3e4...heic.xmp  ← carried .xmp (with merged Pixe tags)
│   └── 03-Mar/
│       ├── 20220316_232122_321c7d6f...mp4
│       └── 20220316_232122_321c7d6f...mp4.xmp
├── duplicates/
│   └── 20260306_103000/
│       └── 2022/02-Feb/
│           ├── 20220202_123101_447d...jpg
│           ├── 20220202_130000_e5f6...nef
│           ├── 20220202_130000_e5f6...nef.xmp    ← sidecars follow dupes too
│           └── 20220202_130000_e5f6...nef.aae    ← carried sidecars follow dupes too
└── .pixe/
    ├── pixe.db              ← SQLite database (if dirB is local)
    ├── pixe.db-wal          ← WAL file (transient, managed by SQLite)
    ├── pixe.db-shm          ← shared memory file (transient, managed by SQLite)
    └── dbpath               ← marker file (only if DB is stored elsewhere)

~/.pixe/                      ← user-level directory (created only when needed)
└── databases/
    └── archive-a1b2c3d4.db  ← database for a network-mounted dirB
```

> **Note:** XMP sidecar files (`.xmp`) may appear from two sources: (1) Pixe-generated sidecars when `--copyright` and/or `--camera-owner` are configured, or (2) pre-existing source sidecars carried from `dirA` (see Section 4.12). When both apply, Pixe merges its tags into the carried source sidecar rather than generating a separate file. Carried `.aae` files (Apple edit instructions) are copied verbatim and never modified by Pixe.

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
| `pixe sort` | `--no-carry-sidecars` | | `false` | `carry_sidecars: false` | Disable source sidecar carry. Pre-existing `.aae` and `.xmp` files in `dirA` are not carried alongside their parent media file. See Section 4.12. |
| `pixe sort` | `--overwrite-sidecar-tags` | | `false` | `overwrite_sidecar_tags` | When merging Pixe tags into a carried `.xmp` sidecar, overwrite existing values instead of preserving them. See Section 4.12.6. |
| `pixe clean` | `--dir` | `-d` | (required) | `clean_dir` | Destination directory (`dirB`) to clean. |
| `pixe clean` | `--db-path` | | (auto-detected) | `clean_db_path` | Explicit path to the SQLite database file. Overrides automatic location logic. |
| `pixe clean` | `--dry-run` | | `false` | — | Preview what would be cleaned without deleting files or running VACUUM. |
| `pixe clean` | `--temp-only` | | `false` | — | Only clean orphaned temp files and XMP sidecars. Skip database compaction. |
| `pixe clean` | `--vacuum-only` | | `false` | — | Only run database compaction. Skip artifact scanning. Mutually exclusive with `--temp-only`. |

All flags are supported via config file and environment variable (e.g., `PIXE_RECURSIVE`, `PIXE_SKIP_DUPLICATES`, `PIXE_IGNORE`, `PIXE_CARRY_SIDECARS`, `PIXE_OVERWRITE_SIDECAR_TAGS`). The `--ignore` flag can appear multiple times on the command line, each specifying one glob pattern. In the config file, `ignore` is a YAML list. The `--no-carry-sidecars` flag maps to the config key `carry_sidecars: false` (note the inverted polarity — the config key is positive, the flag is negative).

### 9.2 Updated `pixe resume`

The `resume` command now locates the database via the same discovery chain (flag → `dbpath` marker → default local path) and queries for the interrupted run's incomplete files.

---

## 10. Documentation Site (`docs/`)

### 10.1 Overview

The Pixe documentation site is a Jekyll-based static site deployed to GitHub Pages from the `docs/` directory. It replaces the prior single-file `docs/index.html` with a multi-page site built from Markdown content files, a custom local theme, and Jekyll's standard layout/include system. The site preserves the visual identity of the original single-page design: dark background (`#0c0c0c`), warm amber accent (`#b5a642`, "darkroom safelight"), monospace brand typography, card-based content blocks, and a minimal, developer-focused aesthetic.

### 10.2 Design Goals

1. **Preserve the visual identity.** The site must look and feel like the existing `index.html` — same color palette, same typography, same card and grid components. A visitor who saw the old site should recognize the new one immediately.
2. **Content in Markdown.** All page content is authored in Markdown files with YAML front matter. No content lives in HTML layout files. This makes documentation contributions accessible to anyone who can edit Markdown.
3. **Local theme, not a gem.** The theme (layouts, includes, SCSS) lives inside `docs/` as standard Jekyll convention directories. It is not packaged as a standalone gem — it exists solely to serve this project.
4. **Top navigation.** The site uses a sticky top nav bar (carried over from the original design) for primary navigation between pages. No sidebar. The nav links update across pages via a `_data/navigation.yml` data file.
5. **Landing page preserves the marketing feel.** The homepage retains the hero section, condensed "why" summary, pipeline visualization, and call-to-action buttons from the original design. Detailed reference content moves to inner pages.
6. **Standard GitHub Pages deployment.** Built and served by GitHub Pages' built-in Jekyll support. Base URL: `https://cwlls.github.io/pixe-go/`.

### 10.3 Site Structure

#### 10.3.1 Directory Layout

```
docs/
├── _config.yml                  ← Jekyll configuration
├── _data/
│   └── navigation.yml           ← Top nav link definitions
├── _includes/
│   ├── head.html                ← <head> block: meta, CSS links
│   ├── nav.html                 ← Sticky top navigation bar
│   ├── footer.html              ← Site footer
│   ├── hero.html                ← Homepage hero section (used only by landing layout)
│   ├── pipeline.html            ← Pipeline visualization component
│   └── format-grid.html         ← Supported file types grid component
├── _layouts/
│   ├── default.html             ← Base layout: head, nav, {{ content }}, footer
│   ├── landing.html             ← Homepage layout: extends default, adds hero + marketing sections
│   └── page.html                ← Inner page layout: extends default, adds section-label + title + prose container
├── _sass/
│   ├── _variables.scss          ← CSS custom properties (color palette, fonts, spacing, radius)
│   ├── _reset.scss              ← Box-sizing reset, base element styles
│   ├── _typography.scss         ← Headings, paragraphs, links, inline code
│   ├── _nav.scss                ← Sticky nav bar, brand, nav links, GitHub button
│   ├── _layout.scss             ← Container, section spacing, responsive breakpoints
│   ├── _hero.scss               ← Hero section: title, subtitle, promise, action buttons
│   ├── _cards.scss              ← Problem grid cards, format grid cards, AI card
│   ├── _code.scss               ← Code blocks, terminal output styling, pre-label
│   ├── _tables.scss             ← Flag tables, command reference tables
│   ├── _components.scss         ← Pipeline visualization, callouts, step lists, cmd-block accordion
│   ├── _footer.scss             ← Footer layout and links
│   └── _utilities.scss          ← Margin helpers, .dim class, .section-label
├── assets/
│   └── css/
│       └── main.scss            ← SCSS entry point: imports all partials
├── index.md                     ← Homepage content (uses landing layout)
├── install.md                   ← Installation & quick start
├── commands.md                  ← Complete command reference (all commands, all flags)
├── how-it-works.md              ← Pipeline, output format, naming, archive database, file types
├── technical.md                 ← Technical benefits: safety, determinism, integrity, native Go
├── contributing.md              ← Contributing guide
├── adding-formats.md            ← Developer guide: implementing a new FileTypeHandler
├── changelog.md                 ← Changelog (pulls from or mirrors root CHANGELOG.md)
├── ai.md                        ← AI collaboration statement
└── .nojekyll                    ← NOT present (we want Jekyll processing)
```

#### 10.3.2 Jekyll Configuration (`_config.yml`)

```yaml
title: Pixe
description: Safe, deterministic photo and video sorting
url: "https://cwlls.github.io"
baseurl: "/pixe-go"
markdown: kramdown
kramdown:
  input: GFM
  syntax_highlighter: rouge
  syntax_highlighter_opts:
    default_lang: bash
sass:
  sass_dir: _sass
  style: compressed
exclude:
  - README.md
  - Gemfile
  - Gemfile.lock
```

#### 10.3.3 Navigation Data (`_data/navigation.yml`)

```yaml
- title: Install
  url: /install/
- title: Commands
  url: /commands/
- title: How It Works
  url: /how-it-works/
- title: Technical
  url: /technical/
- title: Contributing
  url: /contributing/
- title: Changelog
  url: /changelog/
```

The nav include iterates over this list and highlights the current page. The "pixe" brand link and "GitHub ↗" button are hardcoded in the nav include (not data-driven) to match the original design.

### 10.4 Theme Specification

The theme is a direct translation of the inline CSS from the original `index.html` into Jekyll's SCSS partial system. Every CSS custom property, every component class, and every responsive breakpoint from the original is preserved. The theme is not a generic documentation theme — it is purpose-built for Pixe's visual identity.

#### 10.4.1 Design Tokens (`_variables.scss`)

Extracted verbatim from the original `:root` block:

| Token | Value | Purpose |
|---|---|---|
| `--bg` | `#0c0c0c` | Page background |
| `--bg-raised` | `#141414` | Elevated surface (AI section background) |
| `--bg-card` | `#181818` | Card/panel background |
| `--border` | `#262626` | Default border color |
| `--border-hi` | `#333` | Hover/active border |
| `--text` | `#e8e8e8` | Primary text |
| `--text-dim` | `#888` | Secondary/descriptive text |
| `--text-faint` | `#555` | Tertiary/muted text |
| `--accent` | `#b5a642` | Warm amber — brand color, links, highlights |
| `--accent-lo` | `#b5a64222` | Accent at low opacity (tag backgrounds, active pipeline steps) |
| `--green` | `#4a9e6e` | Success/positive indicators |
| `--green-lo` | `#4a9e6e18` | Green at low opacity (callout backgrounds) |
| `--red` | `#c0554a` | Error indicators |
| `--mono` | `'SF Mono', 'Cascadia Code', 'Fira Mono', 'Consolas', monospace` | Code, brand, labels |
| `--sans` | `-apple-system, BlinkMacSystemFont, 'Segoe UI', system-ui, sans-serif` | Body text |
| `--radius` | `6px` | Border radius |
| `--nav-h` | `56px` | Nav bar height |

#### 10.4.2 Component Inventory

Every visual component from the original site is preserved as a reusable CSS class. Components used on multiple pages are styled globally. Components unique to the homepage (hero, problem grid) are scoped but available for reuse.

| Component | CSS Classes | Used On |
|---|---|---|
| **Sticky nav** | `.nav-brand`, `.nav-links`, `.nav-spacer`, `.nav-gh` | All pages (include) |
| **Hero section** | `.hero-tag`, `.hero-title`, `.hero-sub`, `.hero-promise`, `.hero-actions`, `.btn`, `.btn-primary`, `.btn-ghost` | Homepage only |
| **Section label** | `.section-label` | All content pages |
| **Problem grid** | `.problem-grid`, `.problem-card`, `.problem-q`, `.problem-a`, `.tag-ok` | Homepage |
| **Pipeline visualization** | `.pipeline`, `.pipeline-step`, `.pipeline-step.active`, `.pipeline-arrow` | Homepage, How It Works |
| **Code blocks** | `pre`, `code`, `.pre-label`, `.term-prompt`, `.term-cmd`, `.term-copy`, `.term-skip`, `.term-dupe`, `.term-err`, `.term-done`, `.term-comment` | Multiple pages |
| **Format grid** | `.format-grid`, `.format-card`, `.format-name`, `.format-ext` | How It Works |
| **Flag tables** | `.flag-table-wrap`, `.flag-table` | Commands |
| **Command accordion** | `.cmd-block`, `.cmd-header`, `.cmd-name`, `.cmd-desc`, `.cmd-toggle`, `.cmd-body` | Commands |
| **Contributing steps** | `.contribute-steps`, `.step`, `.step-num`, `.step-text` | Contributing |
| **AI card** | `.ai-card`, `.ai-ref` | AI statement |
| **Callout** | `.callout` | Multiple pages |
| **Footer** | `.footer-inner`, `.footer-brand`, `.footer-links`, `.footer-spacer`, `.footer-copy` | All pages (include) |

#### 10.4.3 Markdown Rendering Styles

The `page` layout wraps content in a `.container` div with max-width `880px` (matching the original). Standard Markdown elements receive styling that matches the original design:

| Markdown Element | Rendered Style |
|---|---|
| `# H1` | Same as `.hero-title` but smaller (used as page title, rendered by layout) |
| `## H2` | `1.75rem`, weight `700`, letter-spacing `-0.02em`, color `--text` |
| `### H3` | `1.1rem`, weight `600`, `2rem` top margin, color `--text` |
| `p` | `1rem` bottom margin, color `--text`, line-height `1.65` |
| `` `inline code` `` | `0.85em`, `--bg-card` background, `1px solid --border`, `--accent` color |
| ```` ``` code blocks ```` | `#0a0a0a` background, `1px solid --border`, `0.82rem` font, `1.7` line-height |
| `> blockquote` | Left border `2px solid --accent`, padding-left `1rem`, color `--text-dim` |
| Tables | Same styling as `.flag-table` — monospace first column in `--accent`, `--border` row separators |
| `- list items` | Color `--text-dim`, `0.5rem` spacing, left padding `1.5rem` |
| `**bold**` | Color `--text` (stands out against `--text-dim` surrounding text) |
| Horizontal rules `---` | `1px solid --border`, `3rem` vertical margin |

#### 10.4.4 Rouge Syntax Highlighting

Code blocks use Rouge (GitHub Pages' built-in highlighter) with a custom dark theme that matches the terminal styling from the original site:

| Token Type | Color | Maps to Original |
|---|---|---|
| Comment | `--text-faint` (`#555`) | `.term-comment` |
| String | `--green` (`#4a9e6e`) | `.term-copy` |
| Keyword | `--accent` (`#b5a642`) | `.term-dupe` |
| Name/Function | `--text` (`#e8e8e8`) | `.term-cmd` |
| Error | `--red` (`#c0554a`) | `.term-err` |
| Background | `#0a0a0a` | `pre` background |

The Rouge theme is defined as a SCSS partial (`_sass/_code.scss`) using Rouge's class selectors (`.highlight .c`, `.highlight .s`, etc.).

#### 10.4.5 Responsive Behavior

Preserved from the original, breakpoint at `640px`:

- Problem grid collapses to single column
- Nav links collapse (hidden on mobile; the brand and GitHub button remain)
- H2 shrinks to `1.4rem`
- Pipeline component reduces padding
- Footer stacks vertically

### 10.5 Page Content Specification

#### 10.5.1 Homepage (`index.md`)

**Layout:** `landing`

The homepage preserves the marketing/landing page feel. It contains:

1. **Hero section** — Rendered by the `hero.html` include (not Markdown). Preserves the version tag, `pixe` title in monospace, subtitle, promise paragraph, and two CTA buttons ("Get Started" → `/install/`, "View on GitHub" → repo). The version tag value is set in `_config.yml` as a site variable (`version: v1.8.0`) so it can be updated in one place.
2. **Why Pixe (condensed)** — The six problem/answer cards from the original, rendered via the `hero.html` include or as a Markdown section with HTML card markup. This stays on the homepage because it's the primary value proposition.
3. **Pipeline visualization** — The discover → extract → hash → copy → verify → tag → complete pipeline, rendered via the `pipeline.html` include. Brief one-line descriptions of each stage.
4. **Quick start snippet** — A condensed install + first-command block (3-4 lines). Links to the full Install page.

Content that moves to inner pages:
- Detailed output format and naming → How It Works
- Supported file types grid → How It Works
- Full command reference with flag tables → Commands
- Configuration file details → Commands
- Contributing steps → Contributing
- AI statement → AI

#### 10.5.2 Installation (`install.md`)

**Layout:** `page`  
**Section label:** `Installation`  
**Title:** Get started in minutes

Content (carried from the original Install section):
- Install via Go (`go install ...`)
- Build from source (`git clone`, `make build`)
- Quick start examples (sort, status, verify)
- Dry-run tip callout
- Link to the Commands page for full reference

#### 10.5.3 Command Reference (`commands.md`)

**Layout:** `page`  
**Section label:** `Reference`  
**Title:** Commands

This is the comprehensive command reference page. Every command gets its own section with:
- Command signature
- Description paragraph
- Flag table (using the `.flag-table-wrap` / `.flag-table` styling)
- Usage examples in terminal-styled code blocks

Commands covered (in order):
1. **`pixe sort`** — All flags from Section 7.1, including `--source`, `--dest`, `--workers`, `--algorithm`, `--recursive`, `--ignore`, `--skip-duplicates`, `--copyright`, `--camera-owner`, `--dry-run`, `--db-path`
2. **`pixe status`** — Flags: `--source`, `--recursive`, `--ignore`, `--json`
3. **`pixe verify`** — Flags: `--dir`, `--algorithm`, `--workers`
4. **`pixe resume`** — Flags: `--dir`, `--db-path`
5. **`pixe query`** — Parent flags (`--dir`, `--db-path`, `--json`) plus each subcommand: `runs`, `run <id>`, `duplicates`, `errors`, `skipped`, `files`, `inventory`
6. **`pixe clean`** — Flags: `--dir`, `--db-path`, `--dry-run`, `--temp-only`, `--vacuum-only`
7. **`pixe version`**

Each command uses the expandable accordion component (`.cmd-block`) from the original design, with the sort command expanded by default.

Ends with a **Configuration file** section explaining `.pixe.yaml` with a YAML example.

#### 10.5.4 How It Works (`how-it-works.md`)

**Layout:** `page`  
**Section label:** `Internals`  
**Title:** How it works

Content (expanded from the original "How It Works" section):
- **Pipeline stages** — Full pipeline diagram with the `pipeline.html` include, plus detailed descriptions of each stage (pending → extracted → hashed → copied → verified → tagged → complete) and error states
- **Output format** — The four outcome verbs (COPY, SKIP, DUPE, ERR) with terminal-styled examples
- **Output naming** — `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` convention, directory structure `<YYYY>/<MM-Mon>/`, locale-aware month names
- **Date fallback chain** — EXIF DateTimeOriginal → CreateDate → Ansel Adams birthday (1902-02-20)
- **Duplicate handling** — Default copy-to-quarantine behavior vs `--skip-duplicates`
- **Archive database** — SQLite at `<dest>/.pixe/<slug>.db`, what it tracks, WAL mode
- **Supported file types** — The format grid (via `format-grid.html` include) with all 9 formats: JPEG, HEIC, MP4/MOV, DNG, NEF, CR2, CR3, PEF, ARW

#### 10.5.5 Technical Benefits (`technical.md`)

**Layout:** `page`  
**Section label:** `Design`  
**Title:** Why Pixe is built this way

A concise, focused page explaining the technical design values that distinguish Pixe. Not a rehash of features — a statement of engineering principles and why they matter for photo archiving. Sections:

1. **Source files are never touched** — Read-only `dirA` constraint. Only the ledger is written. Why this matters for irreplaceable media.
2. **Copy-then-verify** — Atomic temp file write, independent re-hash, rename only on match. Why checksums on both sides matter (bit rot, flaky USB, NAS packet loss).
3. **Deterministic output** — Same input always produces the same archive structure. Why this enables confidence in re-runs and multi-source merges.
4. **No external dependencies** — Single binary, no exiftool, no ffmpeg. All parsing in pure Go. Why this matters for long-term archival (the tool works in 10 years without a dependency chain).
5. **Crash-safe by design** — Per-file transaction commits, streaming JSONL ledger, atomic rename. An interrupted run loses at most one in-flight file.
6. **Content-based deduplication** — Checksum of the media payload, not filenames. Why `IMG_0001.jpg` from two different cameras are correctly handled.

Each section is 2-3 short paragraphs. No code examples — this page is prose-focused for a reader evaluating whether to trust Pixe with their photo archive.

#### 10.5.6 Contributing (`contributing.md`)

**Layout:** `page`  
**Section label:** `Open Source`  
**Title:** Contributing to Pixe

Content (carried from the original Contributing section):
- Opening paragraph about Apache 2.0 and what contributions are welcome
- The 5-step contributing flow (using the `.contribute-steps` / `.step` component)
- Link to GitHub Issues

#### 10.5.7 Adding a New Format (`adding-formats.md`)

**Layout:** `page`  
**Section label:** `Developer Guide`  
**Title:** Adding a new file format

A developer-facing guide for implementing a new `FileTypeHandler`. Sections:

1. **Overview** — Pixe's format support is modular. Each format is an isolated package that implements the `FileTypeHandler` interface. The core pipeline requires no changes.
2. **The `FileTypeHandler` interface** — Full interface listing with doc comments (from ARCHITECTURE.md Section 6.1). Each method explained in plain language.
3. **Step-by-step walkthrough** — Using a hypothetical format (e.g., WEBP) as an example:
   - Create the package directory (`internal/handler/webp/`)
   - Implement `Extensions()` and `MagicBytes()` for detection
   - Implement `Detect()` with extension + magic byte verification
   - Implement `ExtractDate()` with the date fallback chain
   - Implement `HashableReader()` — what the "hashable region" means and how to identify it
   - Implement `MetadataSupport()` — choosing between Embed, Sidecar, or None
   - Implement `WriteMetadataTags()` (or no-op stub for Sidecar/None)
   - Register in `cmd/sort.go`, `cmd/verify.go`, `cmd/resume.go`, `cmd/status.go`
   - Write tests using the project's testing conventions
4. **TIFF-based RAW shortcut** — If the new format is TIFF-based, embed `tiffraw.Base` and supply only Extensions/MagicBytes/Detect. Reference existing handlers (DNG, NEF, etc.) as templates.
5. **Testing conventions** — stdlib only, `t.TempDir()`, `t.Helper()`, `-race` always, fixture files

#### 10.5.8 Changelog (`changelog.md`)

**Layout:** `page`  
**Section label:** `History`  
**Title:** Changelog

This page renders the project changelog. The content is hand-authored in `docs/changelog.md` and updated as part of the release process. The changelog is editorial — it benefits from curation and narrative that automated extraction cannot provide.

> **Note:** The changelog is one of the hand-authored pages that the `docgen` tool (Section 15) does not touch. See Section 15.6 for the full page classification.

#### 10.5.9 AI Statement (`ai.md`)

**Layout:** `page`  
**Section label:** `Transparency`  
**Title:** Built with AI collaboration

Content carried directly from the original AI section, using the `.ai-card` component for the statement block. Includes the external reference link to `daplin.org/ai-collaboration.html`.

### 10.6 Layouts

#### 10.6.1 `default.html`

The base layout. Every page uses this (directly or via inheritance). Structure:

```html
<!DOCTYPE html>
<html lang="en">
<head>{% include head.html %}</head>
<body>
  {% include nav.html %}
  {{ content }}
  {% include footer.html %}
  <script>/* accordion toggle function, if any cmd-blocks are on the page */</script>
</body>
</html>
```

#### 10.6.2 `landing.html`

Extends `default.html`. Used only by `index.md`. Wraps content in the hero + marketing section structure:

```html
---
layout: default
---
{% include hero.html %}
<section id="why">
  <div class="container">
    {{ content }}
  </div>
</section>
```

The hero include reads `site.version`, `site.description`, and `site.tagline` from `_config.yml` for the tag, subtitle, and promise text. The `version` field in `_config.yml` is maintained by the `docgen` tool (Section 15) — it is extracted from the latest git tag and injected via marker-based replacement. The Markdown content in `index.md` provides the "Why Pixe" cards and pipeline section.

#### 10.6.3 `page.html`

Extends `default.html`. Used by all inner pages. Adds the section label, page title, and prose container:

```html
---
layout: default
---
<section>
  <div class="container">
    {% if page.section_label %}
      <div class="section-label">{{ page.section_label }}</div>
    {% endif %}
    <h2>{{ page.title }}</h2>
    {{ content }}
  </div>
</section>
```

Pages set `section_label` and `title` in their front matter:

```yaml
---
layout: page
title: Commands
section_label: Reference
---
```

### 10.7 Includes

| Include | Purpose | Used By |
|---|---|---|
| `head.html` | `<meta>` tags, `<title>`, CSS `<link>` to `assets/css/main.css` | `default.html` |
| `nav.html` | Sticky top nav bar. Iterates `site.data.navigation` for links. Highlights current page via `page.url` comparison. Hardcodes brand link and GitHub button. | `default.html` |
| `footer.html` | Footer with brand, links (GitHub, Issues, License, AI), copyright | `default.html` |
| `hero.html` | Hero section with version tag, title, subtitle, promise, CTA buttons | `landing.html` |
| `pipeline.html` | Pipeline step visualization (discover → ... → complete) | `index.md`, `how-it-works.md` |
| `format-grid.html` | Supported file types grid (JPEG, HEIC, MP4, DNG, NEF, CR2, CR3, PEF, ARW) | `how-it-works.md` |

### 10.8 Interactive Components

The command accordion (`.cmd-block` expand/collapse) from the original site is preserved on the Commands page. The toggle is powered by a minimal inline `<script>` (same as the original):

```javascript
function toggle(header) {
  const block = header.closest('.cmd-block');
  block.classList.toggle('open');
}
```

This is the only JavaScript on the site. No build tools, no npm, no bundler.

### 10.9 Migration from Single-File Site

The original `docs/index.html` is removed and replaced by the Jekyll site structure. The migration is a content decomposition — all text, code examples, and visual components from the original are distributed across the new pages and theme files. No content is lost; content is only reorganized and, in the case of the Technical and Adding Formats pages, newly authored.

**What happens to the original file:**
- `docs/index.html` is deleted from the repository.
- Its CSS becomes the SCSS partials in `_sass/`.
- Its HTML structure becomes the layouts and includes in `_layouts/` and `_includes/`.
- Its content becomes the Markdown files in `docs/`.
- Its JavaScript (the accordion toggle) moves to the `default.html` layout.

### 10.10 Deployment

The site is deployed via GitHub Pages using Jekyll's built-in processing. Configuration:

- **Source:** `docs/` directory on the default branch
- **Base URL:** `https://cwlls.github.io/pixe-go/`
- **Build:** GitHub Pages' built-in Jekyll (no GitHub Actions workflow needed for the basic case)
- **CNAME:** None (standard GitHub Pages URL)

**Stale workflow:** The `.github/workflows/pages.yml` file is an earlier artifact that predates GitHub's automatic Pages deployment. It should be removed — GitHub Pages is configured via repository settings to build from the `docs/` directory using its built-in Jekyll support. See Section 15.9 for the CI workflow updates that replace it.

No `Gemfile` is strictly required for GitHub Pages' built-in Jekyll, but one may be added for local development (`bundle exec jekyll serve`):

```ruby
source "https://rubygems.org"
gem "jekyll", "~> 4.3"
gem "kramdown-parser-gfm"
```

---

## 11. Pipeline Event Bus

### 11.1 Overview

The pipeline event bus is a **structured event channel** that decouples the sort/verify pipelines from their output presentation. Instead of writing directly to an `io.Writer` via `fmt.Fprintf`, the pipeline emits typed event structs onto a channel. Consumers — the plain-text CLI writer, the CLI progress bar, or the full TUI — subscribe to this channel and render events in their own way.

This is the foundational layer that enables both the opt-in CLI progress bars (Section 12) and the interactive TUI (Section 13) without modifying the pipeline's core logic.

### 11.2 Design Principles

1. **Non-breaking adoption.** The existing `io.Writer`-based output continues to work. The event bus is an **addition**, not a replacement. When no event subscriber is configured, the pipeline falls back to the current `fmt.Fprintf(out, ...)` behaviour. This means all existing tests, piped output, and scripted workflows are unaffected.
2. **Typed events, not strings.** Events carry structured data (file path, status, checksum, duration, error, worker ID). Consumers format this data however they choose. The pipeline never formats presentation strings — it emits facts.
3. **Unidirectional flow.** Events flow from the pipeline to consumers. Consumers never send commands back through the event bus. Pipeline control (pause, cancel) uses `context.Context`, not the event channel.
4. **Non-blocking sends.** The pipeline must never block on a slow consumer. Events are sent on a buffered channel. If the buffer is full, the event is dropped. Correctness is in the database and ledger — the event bus is for observation, not for the audit trail.

### 11.3 Event Types

```go
package progress

import "time"

// EventKind identifies the type of pipeline event.
type EventKind int

const (
    // Discovery phase events.
    EventDiscoverStart  EventKind = iota // Walk began; Total field set.
    EventDiscoverDone                    // Walk complete; Total, Skipped fields set.

    // Per-file lifecycle events.
    EventFileStart                       // File processing began.
    EventFileExtracted                   // Date extracted.
    EventFileHashed                      // Checksum computed.
    EventFileCopied                      // Temp file written.
    EventFileVerified                    // Hash verified, promoted to canonical path.
    EventFileTagged                      // Metadata written (embed or sidecar).
    EventFileComplete                    // Terminal success state.
    EventFileDuplicate                   // File identified as duplicate.
    EventFileSkipped                     // File skipped (previously imported or unsupported).
    EventFileError                       // File failed at some stage.

    // Sidecar events.
    EventSidecarCarried                  // Sidecar file copied alongside parent.
    EventSidecarFailed                   // Sidecar carry failed (non-fatal).

    // Run-level events.
    EventRunComplete                     // All files processed; summary fields set.

    // Verify-specific events.
    EventVerifyStart                     // Verify walk began.
    EventVerifyOK                        // File checksum matches.
    EventVerifyMismatch                  // File checksum does not match.
    EventVerifyUnrecognised              // File not parseable by any handler.
    EventVerifyDone                      // Verify complete.
)

// Event is the universal pipeline event. Not all fields are populated for
// every EventKind — consumers check the Kind and read the relevant fields.
type Event struct {
    Kind      EventKind
    Timestamp time.Time

    // File identity.
    RelPath  string // relative path from dirA (sort) or dirB (verify).
    AbsPath  string // absolute path for consumers that need it.
    WorkerID int    // which worker is handling this file (-1 for coordinator).

    // Pipeline progress.
    Total     int // total files discovered (set on EventDiscoverDone).
    Completed int // files finished so far (incremented on terminal events).
    Skipped   int // files skipped during discovery.

    // File outcome data (populated progressively).
    Checksum    string
    Destination string // relative path within dirB.
    CaptureDate time.Time
    FileSize    int64

    // Duplicate info.
    IsDuplicate bool
    MatchesDest string // dest of the existing file this one matches.

    // Skip/error info.
    Reason string // skip reason or error message.
    Err    error  // underlying error (EventFileError, EventVerifyMismatch).

    // Verify-specific.
    ExpectedChecksum string
    ActualChecksum   string

    // Sidecar info.
    SidecarRelPath string
    SidecarExt     string

    // Summary (EventRunComplete / EventVerifyDone).
    Summary *RunSummary
}

// RunSummary aggregates the final counts for a completed run.
type RunSummary struct {
    Processed    int
    Duplicates   int
    Skipped      int
    Errors       int
    Duration     time.Duration
    // Verify-specific.
    Verified     int
    Mismatches   int
    Unrecognised int
}
```

### 11.4 Event Bus Interface

```go
package progress

// Bus is the event distribution mechanism. The pipeline calls Emit for each
// event; consumers receive events from the channel returned by Subscribe.
type Bus struct {
    ch     chan Event
    closed chan struct{}
}

// NewBus creates a Bus with the given buffer size. A buffer of 256 is
// recommended — large enough to absorb bursts from concurrent workers
// without blocking, small enough to bound memory.
func NewBus(bufferSize int) *Bus

// Emit sends an event to all subscribers. If the buffer is full, the event
// is dropped (non-blocking). The pipeline must never stall on a slow consumer.
func (b *Bus) Emit(e Event)

// Events returns the receive-only channel for consumers to range over.
// The channel is closed when Close is called.
func (b *Bus) Events() <-chan Event

// Close signals that no more events will be emitted. Consumers ranging
// over Events() will exit their loop.
func (b *Bus) Close()
```

### 11.5 Integration with `SortOptions` and `VerifyOptions`

The event bus is an **optional** field on the existing options structs. When nil, the pipeline uses the current `io.Writer` path unchanged.

```go
// SortOptions gains one new field:
type SortOptions struct {
    // ... existing fields ...

    // EventBus, when non-nil, receives structured progress events.
    // The plain-text Output writer is still used for the audit log
    // when EventBus is set — both can be active simultaneously.
    EventBus *progress.Bus
}
```

```go
// verify.Options gains one new field:
type Options struct {
    // ... existing fields ...
    EventBus *progress.Bus
}
```

**Emission points in the pipeline:**

The pipeline code gains `emit()` helper calls at each stage transition. These are additive — the existing `fmt.Fprintf(out, ...)` calls remain in place. The emit calls are guarded by a nil check:

```go
// In pipeline.go or worker.go:
func emit(bus *progress.Bus, e progress.Event) {
    if bus != nil {
        e.Timestamp = time.Now()
        bus.Emit(e)
    }
}
```

This means every `fmt.Fprintf(out, formatOutput(...))` call is paired with a corresponding `emit(bus, Event{...})` call. The `io.Writer` path is the audit trail; the event bus is the observation channel.

### 11.6 Plain-Text Writer as Event Consumer

To demonstrate the pattern and prepare for eventual removal of inline `fmt.Fprintf` calls, a `PlainWriter` consumer is provided:

```go
package progress

// PlainWriter consumes events and writes the traditional plain-text output
// (COPY, SKIP, DUPE, ERR lines) to an io.Writer. It is the default consumer
// when no TUI or progress bar is active.
type PlainWriter struct {
    w io.Writer
}

func NewPlainWriter(w io.Writer) *PlainWriter

// Run reads events from the bus and writes formatted output until the
// channel is closed. Intended to be called in a goroutine.
func (pw *PlainWriter) Run(events <-chan Event)
```

**Migration path:** In the first implementation, the pipeline retains its inline `fmt.Fprintf` calls AND emits events. The `PlainWriter` is not yet wired in as the sole output path — it exists as a reference implementation and for testing. A future refactor can remove the inline writes and route all output through the event bus, but this is not required for the initial TUI work.

### 11.7 Package Layout

```
internal/
└── progress/
    ├── event.go          ← Event, EventKind, RunSummary types
    ├── bus.go            ← Bus implementation (channel, Emit, Close)
    ├── plainwriter.go    ← PlainWriter consumer (traditional CLI output)
    └── progress_test.go  ← Bus tests (non-blocking emit, close semantics)
```

The `progress` package has **zero dependencies** on any Charm library. It is pure Go stdlib. The Charm dependencies are confined to the `cli` and `tui` packages that consume events (Sections 12 and 13).

---

## 12. CLI Progress Bars

### 12.1 Overview

The CLI progress bar is an **opt-in enhancement** to the standard `pixe sort` and `pixe verify` commands. When activated, it replaces the scrolling per-file text output with a compact, live-updating progress display rendered using the Bubbles `progress` component from the Charm ecosystem. The plain-text per-file log continues to be written to the JSONL ledger and the archive database — the progress bar affects only what the user sees on screen during the run.

### 12.2 Activation

| Method | Behavior |
|---|---|
| `--progress` flag | Enables the progress bar display. Available on `pixe sort` and `pixe verify`. |
| Default (no flag) | Traditional scrolling text output (unchanged). |
| Piped stdout | Progress bar is automatically disabled even if `--progress` is set. Detection via `mattn/go-isatty` (already an indirect dependency). |

The `--progress` flag is a simple boolean. There is no `--no-progress` inverse — the default is off.

### 12.3 Visual Design

The progress bar follows the terminal's current color scheme. No hardcoded colors — Lip Gloss adaptive colors are used so the display works on both light and dark terminals.

**Layout during a sort run:**

```
pixe sort ─ /Users/wells/photos → /Users/wells/archive

  Total   ████████████████░░░░░░░░░░░░░░░░  1,247 / 3,891  (32.0%)  ETA 2m14s
  Current extracting IMG_5678.jpg

  copied 1,180 │ duplicates 42 │ skipped 15 │ errors 2
```

**Layout during a verify run:**

```
pixe verify ─ /Users/wells/archive

  Total   ████████████████████░░░░░░░░░░░░  8,421 / 12,634  (66.7%)  ETA 45s

  verified 8,400 │ mismatches 2 │ unrecognised 19
```

**Components:**

| Element | Implementation |
|---|---|
| Header line | Static — command name, source/dest paths. Lip Gloss styled (dim). |
| Total progress bar | Bubbles `progress.Model`. Percentage derived from `Completed / Total`. |
| ETA | Computed from elapsed time and completion ratio. Updated every second. |
| Current file | The `RelPath` from the most recent `EventFileStart`. Truncated to terminal width. |
| Status counters | Live-updated from terminal events. Lip Gloss styled with adaptive colors. |

### 12.4 Architecture

The CLI progress bar is a **lightweight Bubble Tea program** — it uses the Bubble Tea runtime for its update loop and terminal control (alternate screen is NOT used; the progress bar renders inline), but the "model" is minimal: it subscribes to the event bus and updates counters.

```go
package cli

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/progress"
    "github.com/charmbracelet/lipgloss"
)

// ProgressModel is the Bubble Tea model for the inline CLI progress bar.
type ProgressModel struct {
    bus         *progress.Bus
    bar         progress.Model
    total       int
    completed   int
    copied      int
    duplicates  int
    skipped     int
    errors      int
    currentFile string
    startedAt   time.Time
    width       int
    done        bool
}
```

**Integration with `cmd/sort.go`:**

When `--progress` is set and stdout is a TTY:

1. Create the event bus: `bus := progress.NewBus(256)`
2. Set `opts.EventBus = bus` on the `SortOptions`.
3. Create the Bubble Tea program with `ProgressModel`.
4. Run the pipeline in a goroutine.
5. Run the Bubble Tea program on the main goroutine (it owns the terminal).
6. When the pipeline completes, it closes the bus, which sends a final message to the Bubble Tea model, causing it to return `tea.Quit`.

```go
// In cmd/sort.go, within runSort():
if showProgress && isatty.IsTerminal(os.Stdout.Fd()) {
    bus := progress.NewBus(256)
    opts.EventBus = bus
    opts.Output = io.Discard // suppress inline text when progress bar is active

    model := cli.NewProgressModel(bus, cfg.Source, cfg.Destination)
    p := tea.NewProgram(model)

    go func() {
        result, err = pipeline.Run(opts)
        bus.Close()
    }()

    if _, runErr := p.Run(); runErr != nil {
        return fmt.Errorf("progress display: %w", runErr)
    }
}
```

**Key design point:** When the progress bar is active, `opts.Output` is set to `io.Discard`. The per-file text lines are suppressed — the user sees only the progress bar. The ledger and database continue to record everything. After the Bubble Tea program exits, the final summary line is printed normally.

### 12.5 Verify Integration

The same `ProgressModel` is reused for `pixe verify`, with a different header and counter labels. The model accepts a `mode` parameter (`"sort"` or `"verify"`) that controls which counters are displayed.

### 12.6 New Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | latest stable | Bubble Tea runtime for the progress bar update loop |
| `github.com/charmbracelet/bubbles` | latest stable | `progress` component (the actual bar widget) |
| `github.com/charmbracelet/lipgloss` | latest stable | Styling (adaptive colors, alignment, borders) |
| `github.com/mattn/go-isatty` | (already indirect) | TTY detection for auto-disabling progress bar |

### 12.7 New Flags

| Command | Flag | Default | Description |
|---|---|---|---|
| `pixe sort` | `--progress` | `false` | Show a live progress bar instead of per-file text output. Auto-disabled when stdout is not a TTY. |
| `pixe verify` | `--progress` | `false` | Same behavior for verify runs. |

### 12.8 Package Layout

```
internal/
└── cli/
    ├── progress.go       ← ProgressModel (Bubble Tea model for inline progress bar)
    ├── styles.go         ← Lip Gloss styles (adaptive colors, no hardcoded palette)
    └── progress_test.go  ← Model update tests
```

The `cli` package depends on Bubble Tea, Bubbles, and Lip Gloss. It consumes events from `internal/progress` but has no dependency on the `tui` package (Section 13).

---

## 13. TUI Application (`pixe gui`)

### 13.1 Overview

`pixe gui` launches an interactive terminal application built with Bubble Tea. It provides a tabbed interface for the three primary operations — **Sort**, **Verify**, and **Status** — with live progress visualization, scrollable activity logs, and detailed error inspection. The TUI is a self-contained Cobra subcommand that composes the same internal packages used by the CLI commands.

### 13.2 Command Signature

```bash
pixe gui [--source <dirA>] [--dest <dirB>] [options]
```

The `gui` command accepts the same flags as `sort` (source, dest, workers, algorithm, copyright, camera-owner, recursive, ignore, etc.) for pre-configuration. Flags that are not provided can be configured interactively within the TUI before starting a run.

| Flag | Default | Description |
|---|---|---|
| `--source` / `-s` | cwd | Pre-set the source directory. |
| `--dest` / `-d` | (none) | Pre-set the destination directory. |
| All `sort` flags | (same defaults) | Pre-configure sort options. |

### 13.3 Visual Design

The TUI uses the terminal's native color scheme via Lip Gloss **adaptive colors** (`lipgloss.AdaptiveColor`). No hardcoded hex values — the interface adapts to light and dark terminals automatically. Styling uses subtle borders, dim text for secondary information, and the terminal's accent/highlight colors for active elements.

#### 13.3.1 Overall Layout

```
┌─────────────────────────────────────────────────────────────────────────┐
│  pixe gui                                              [Sort] [Verify] [Status]  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  (active tab content — see below)                                       │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│  ? help  tab switch  q quit                                             │
└─────────────────────────────────────────────────────────────────────────┘
```

- **Header bar:** Application name on the left, tab buttons on the right. Active tab is highlighted.
- **Content area:** Fills the remaining terminal height. Content varies by tab.
- **Footer bar:** Context-sensitive key bindings. Updates based on active tab and state.

#### 13.3.2 Sort Tab

The Sort tab has three states: **configure**, **running**, and **complete**.

**Configure state** — Shown before a sort run starts. Displays the current configuration and a "Start Sort" action:

```
  Source:      /Users/wells/photos
  Destination: /Users/wells/archive
  Workers:     8  │  Algorithm: sha1  │  Recursive: yes
  Copyright:   Copyright {{.Year}} My Family
  Camera Owner: Wells Family

  [s] Start Sort    [e] Edit Settings
```

If `--dest` was not provided on the command line, the configure state prompts for it before enabling the start action.

**Running state** — Active during a sort run. Three panes arranged vertically:

```
  Total   ████████████████░░░░░░░░░░░░░░░░  1,247 / 3,891  (32.0%)  ETA 2m14s
  ─────────────────────────────────────────────────────────────────────────
  copied 1,180 │ duplicates 42 │ skipped 15 │ errors 2
  ─────────────────────────────────────────────────────────────────────────
  COPY IMG_0001.jpg → 2021/12-Dec/20211225_062223_7d97e98f...jpg
  COPY IMG_0002.jpg → 2022/02-Feb/20220202_123101_447d3060...jpg
  DUPE IMG_0042.jpg → matches 2022/02-Feb/20220202...jpg
  COPY IMG_0043.jpg → 2022/02-Feb/20220202_130000_e5f6a7b8...jpg
  ERR  corrupt.jpg  → EXIF parse failed: truncated IFD at offset 0x1A
  COPY IMG_0044.jpg → 2022/03-Mar/20220316_232122_321c7d6f...jpg
  │ (scrollable — j/k or arrow keys)
  ─────────────────────────────────────────────────────────────────────────
  Worker 1: hashing  IMG_5678.jpg
  Worker 2: copying  DSC_9012.nef
  Worker 3: verifying IMG_3456.heic
  Worker 4: idle
```

| Pane | Content | Interaction |
|---|---|---|
| **Progress** (top) | Bubbles progress bar, ETA, status counters. | Read-only. |
| **Activity log** (middle) | Scrollable viewport of per-file outcome lines. Most recent at bottom. | `j`/`k` or `↑`/`↓` to scroll. `f` to filter by status (COPY/DUPE/ERR). `G` to jump to bottom (follow mode). |
| **Workers** (bottom) | Per-worker current activity. One line per worker showing stage + filename. | Read-only. Collapses to a single summary line when terminal height is small. |

**Error inspection:** When the activity log cursor is on an `ERR` line, pressing `Enter` opens a detail overlay showing the full error message, file path, and the pipeline stage where it failed. Press `Esc` to dismiss.

**Complete state** — Shown after the run finishes:

```
  Sort complete ─ 3,891 files in 4m32s

  copied 3,832 │ duplicates 42 │ skipped 15 │ errors 2

  COPY  3,832  ████████████████████████████████████████████  98.5%
  DUPE     42  █░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   1.1%
  SKIP     15  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   0.4%
  ERR       2  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░   0.1%

  [Enter] View activity log    [e] View errors    [n] New sort    [q] Quit
```

#### 13.3.3 Verify Tab

Similar to Sort but simpler — no configuration state, just a directory picker and run/complete states.

**Running state:**

```
  Total   ████████████████████░░░░░░░░░░░░  8,421 / 12,634  (66.7%)  ETA 45s
  ─────────────────────────────────────────────────────────────────────────
  verified 8,400 │ mismatches 2 │ unrecognised 19
  ─────────────────────────────────────────────────────────────────────────
    OK          2024/07-Jul/20240715_143022_abc12345...jpg
    OK          2024/07-Jul/20240715_150100_def67890...jpg
    MISMATCH    2024/08-Aug/20240820_091500_1a2b3c4d...mp4
      expected: 1a2b3c4d...  actual: 9f8e7d6c...
    OK          2024/08-Aug/20240820_100000_e5f6a7b8...jpg
  │ (scrollable)
```

**Complete state:**

```
  Verify complete ─ 12,634 files in 6m18s

  verified 12,613 │ mismatches 2 │ unrecognised 19

  [Enter] View details    [e] View mismatches    [n] New verify    [q] Quit
```

#### 13.3.4 Status Tab

The Status tab renders the same categorized report as `pixe status`, but in an interactive, browsable format.

```
  Source: /Users/wells/photos
  Ledger: run a1b2c3d4, 2026-03-06 10:30:00 UTC (recursive: no)
  ─────────────────────────────────────────────────────────────────────────
  265 total │ 247 sorted │ 3 duplicates │ 1 errored │ 12 unsorted │ 2 unrecognized
  ─────────────────────────────────────────────────────────────────────────

  [1] Sorted (247)  [2] Duplicates (3)  [3] Errored (1)  [4] Unsorted (12)  [5] Unrecognized (2)

  UNSORTED (12 files)                                         ← active section
    IMG_5001.jpg
    IMG_5002.jpg
    vacation/IMG_6001.jpg
    vacation/IMG_6002.jpg
    vacation/IMG_6003.jpg
    vacation/IMG_6004.jpg
  │ (scrollable)
```

The user presses `1`–`5` to switch between file categories. Each category is a scrollable list. The status report is computed once when the tab is selected (or when the user presses `r` to refresh).

### 13.4 Architecture

#### 13.4.1 Bubble Tea Model Hierarchy

The TUI uses a **nested model** pattern — a root model delegates to tab-specific sub-models:

```go
package tui

// App is the root Bubble Tea model. It owns the tab bar, footer,
// and delegates to the active tab's sub-model.
type App struct {
    activeTab int           // 0=Sort, 1=Verify, 2=Status
    tabs      []string      // ["Sort", "Verify", "Status"]
    sort      SortModel     // Sort tab sub-model
    verify    VerifyModel   // Verify tab sub-model
    status    StatusModel   // Status tab sub-model
    width     int
    height    int
    keymap    KeyMap
}
```

Each sub-model implements its own `Update` and `View` methods. The root `App.Update` routes key presses and window size messages to the active sub-model, and handles tab switching globally.

#### 13.4.2 Event Bus Integration

The Sort and Verify sub-models each own their own `progress.Bus` instance, created when a run starts. Events from the bus are fed into the Bubble Tea runtime via a `tea.Cmd` that listens on the event channel:

```go
// listenForEvents returns a tea.Cmd that waits for the next event
// from the bus and wraps it as a tea.Msg.
func listenForEvents(bus *progress.Bus) tea.Cmd {
    return func() tea.Msg {
        event, ok := <-bus.Events()
        if !ok {
            return busClosedMsg{}
        }
        return eventMsg(event)
    }
}
```

The sub-model's `Update` method handles `eventMsg` by updating counters, appending to the activity log viewport, and re-issuing `listenForEvents` to get the next event. This is the standard Bubble Tea pattern for channel-to-model bridging.

#### 13.4.3 Pipeline Execution

Sort and Verify runs execute in a **background goroutine**. The Bubble Tea program owns the main goroutine (terminal control). The pattern:

```go
func (m SortModel) startRun() tea.Cmd {
    return func() tea.Msg {
        bus := progress.NewBus(256)
        m.bus = bus

        go func() {
            result, err := pipeline.Run(m.buildSortOptions(bus))
            bus.Close()
        }()

        return runStartedMsg{bus: bus}
    }
}
```

The `runStartedMsg` triggers the sub-model to transition from "configure" to "running" state and begin listening for events.

#### 13.4.4 Shared Components

Several Bubble Tea components are shared across tabs:

| Component | Bubbles Package | Used By |
|---|---|---|
| Progress bar | `bubbles/progress` | Sort (running), Verify (running) |
| Viewport (scrollable log) | `bubbles/viewport` | Sort (activity log), Verify (activity log), Status (file lists) |
| Spinner | `bubbles/spinner` | Sort (current file), Verify (current file) |
| Key bindings/help | `bubbles/key`, `bubbles/help` | All tabs (footer) |

#### 13.4.5 Status Tab Data Flow

Unlike Sort and Verify (which use the event bus), the Status tab calls `discovery.Walk()` and `manifest.LoadLedger()` synchronously in a background goroutine, then sends the classified results as a single `tea.Msg` to the model. There is no streaming progress for status — the walk is fast and the result is rendered all at once.

### 13.5 Key Bindings

| Key | Context | Action |
|---|---|---|
| `Tab` / `Shift+Tab` | Global | Switch to next/previous tab |
| `1`, `2`, `3` | Global | Jump to Sort, Verify, Status tab |
| `q` / `Ctrl+C` | Global | Quit the TUI |
| `?` | Global | Toggle help overlay |
| `s` | Sort (configure) | Start sort run |
| `e` | Sort (configure) | Edit settings (inline prompts) |
| `j` / `k` / `↑` / `↓` | Running / Complete | Scroll activity log |
| `G` | Running | Jump to bottom of log (follow mode) |
| `g` | Running | Jump to top of log |
| `f` | Running | Cycle filter: All → COPY → DUPE → ERR → SKIP → All |
| `Enter` | Running (on ERR line) | Open error detail overlay |
| `Esc` | Overlay open | Close overlay |
| `1`–`5` | Status tab | Switch file category |
| `r` | Status tab | Refresh (re-walk and re-classify) |
| `n` | Sort/Verify (complete) | Start a new run |

### 13.6 Terminal Requirements

- **Minimum size:** 80 columns × 24 rows. Below this, the TUI displays a "terminal too small" message.
- **Alternate screen:** The TUI uses Bubble Tea's alternate screen mode (`tea.WithAltScreen()`). On exit, the user's terminal is restored to its prior state.
- **Mouse support:** Not enabled. All interaction is keyboard-driven.
- **Color support:** Lip Gloss adaptive colors degrade gracefully. On 16-color terminals, the TUI is functional but less visually distinct. On true-color terminals, borders and highlights are crisper.

### 13.7 Interaction with Existing Commands

`pixe gui` is a **parallel entry point**, not a replacement. The existing `pixe sort`, `pixe verify`, and `pixe status` commands are unchanged. The TUI composes the same internal packages:

| TUI Tab | Calls | Same as CLI command |
|---|---|---|
| Sort | `pipeline.Run()` | `pixe sort` |
| Verify | `verify.Run()` | `pixe verify` |
| Status | `discovery.Walk()` + `manifest.LoadLedger()` | `pixe status` |

The TUI does not use Cobra to invoke subcommands — it calls the internal functions directly, bypassing the CLI layer. Configuration is resolved from the flags passed to `pixe gui` plus any interactive edits, then passed as `*config.AppConfig` to the pipeline.

### 13.8 New Dependencies

No additional dependencies beyond those added in Section 12. The TUI uses the same Bubble Tea, Bubbles, and Lip Gloss packages. The `viewport` and `help` components from Bubbles are the only additions (they are part of the `bubbles` module, not separate dependencies).

### 13.9 Package Layout

```
internal/
├── progress/                ← Event bus (Section 11) — no Charm deps
│   ├── event.go
│   ├── bus.go
│   ├── plainwriter.go
│   └── progress_test.go
├── cli/                     ← CLI progress bar (Section 12) — Charm deps
│   ├── progress.go
│   ├── styles.go
│   └── progress_test.go
└── tui/                     ← Full TUI application (Section 13) — Charm deps
    ├── app.go               ← Root App model (tab routing, layout)
    ├── sort.go              ← SortModel (configure → running → complete)
    ├── verify.go            ← VerifyModel (configure → running → complete)
    ├── status.go            ← StatusModel (walk → display → filter)
    ├── components.go        ← Shared components (styled progress bar, log viewport, error overlay)
    ├── keymap.go            ← Key binding definitions
    ├── styles.go            ← Lip Gloss styles (adaptive colors only)
    └── tui_test.go          ← Model update tests
cmd/
├── gui.go                   ← `pixe gui` Cobra command definition
```

### 13.10 CLI Command Definition

```go
// cmd/gui.go

var guiCmd = &cobra.Command{
    Use:   "gui",
    Short: "Launch the interactive terminal interface",
    Long: `Launches a full-screen interactive terminal interface for sorting,
verifying, and inspecting media archives. Provides live progress visualization,
scrollable activity logs, and error inspection.

The TUI composes the same operations as the sort, verify, and status CLI
commands in a tabbed interface.`,
    RunE: runGUI,
}
```

The `runGUI` function resolves configuration from flags/Viper (same as `runSort`), constructs the `tui.App` model with the pre-resolved config, and starts the Bubble Tea program:

```go
func runGUI(cmd *cobra.Command, args []string) error {
    cfg := resolveConfig() // shared with sort — extract to helper

    app := tui.NewApp(tui.AppOptions{
        Config:   cfg,
        Registry: buildRegistry(),
        // ... other shared setup
    })

    p := tea.NewProgram(app, tea.WithAltScreen())
    _, err := p.Run()
    return err
}
```

---

## 14. Open Questions & Future Considerations

These items are explicitly **out of scope** for the current build but are acknowledged for future planning:

1. ~~**Source sidecar association** (`.aae`, existing `.xmp`)~~ — **Promoted to Section 4.12.** No longer a future consideration.
2. ~~**CRW format** (legacy Canon pre-2004) — Excluded from current RAW support due to its obsolete proprietary format and lack of pure-Go library support. Could be revisited if demand arises.~~ no longer under consideration
3. **MP4/MOV embedded metadata writing** — MP4 currently uses `MetadataSidecar`. A future enhancement could implement `udta/©cpy` and `udta/©own` atom writing in pure Go and promote MP4 to `MetadataEmbed`, eliminating the sidecar for video files.
4. **HEIC embedded metadata writing** — HEIC currently uses `MetadataSidecar`. If a reliable pure-Go HEIC EXIF writer becomes available, HEIC could be promoted to `MetadataEmbed`. The `MetadataCapability` enum makes this a one-line change per handler.
5. ~~**Web UI / TUI** — Progress visualization beyond CLI output.~~ **Promoted to Sections 11–13.** No longer a future consideration.
6. **Cloud storage targets** — `dirB` on S3, GCS, etc.
7. ~~**GPS/location-based organization** — Subdirectories by location in addition to date.~~ no longer under consideration
8. ~~**`pixe query` CLI command**~~ — **Promoted to Section 7.3.** No longer a future consideration.
9. ~~**`pixe clean` command**~~ — **Promoted to Section 7.5.** No longer a future consideration.
10. **Multi-archive federation** — Querying across multiple `dirB` databases from a single command.
11. ~~**`**` recursive glob support in ignore patterns**~~ — **Promoted to Section 4.11.** No longer a future consideration.
12. ~~**Directory-level ignore patterns**~~ — **Promoted to Section 4.11.** No longer a future consideration.
13. ~~**`.pixeignore` file**~~ — **Promoted to Section 4.11.** No longer a future consideration.
14. **Extended XMP fields** — The current XMP sidecar writes only Copyright and CameraOwner. Future work could add additional fields (keywords, captions, GPS coordinates, star ratings) to the `MetadataTags` struct and XMP template.
15. **Split-brain network dedup (multi-machine NAS)** — When two machines run `pixe sort` against the same NAS `dirB`, each with its own local `~/.pixe/databases/<slug>.db`, there is no shared state for dedup. Both may write the same file to the primary archive without detecting the collision. A filesystem-level locking strategy using `O_EXCL` temp file creation could address this — the OS guarantees atomicity of `O_EXCL` even over modern SMB/NFS. Deferred until the multi-machine NAS workflow is actively used.
16. ~~**Documentation site**~~ — **Promoted to Section 10.** No longer a future consideration.
17. ~~**Documentation generation from godoc**~~ — **Promoted to Section 15.** No longer a future consideration.

---

## 15. Documentation Generation

### 15.1 Overview

Pixe's codebase conforms strictly to godoc standards — every package, every exported type, every function, constant, and struct field has a doc comment. This is a rich, authoritative source of truth that the hand-authored documentation (`docs/`, `README.md`) currently duplicates and risks drifting from. Section 15 defines a lightweight generation pipeline that extracts machine-sourced facts from the Go source code and injects them into documentation files, while preserving hand-authored narrative prose.

**Core principle: the Go source is the single source of truth for code-derived facts.** Version strings, interface definitions, CLI flags, supported formats, and package descriptions are extracted from code — never maintained as a second copy in Markdown.

### 15.2 Design Goals

1. **Hybrid documents, not full generation.** Most documentation pages contain hand-authored narrative that no tool should touch. The generation pipeline injects code-sourced fragments into designated regions of existing files, leaving everything else untouched.
2. **Marker-based injection.** Generated regions are delimited by HTML comment markers in the Markdown source. The tool replaces content between markers; content outside markers is never modified.
3. **Go AST extraction.** Code facts are extracted by parsing the Go AST directly — no built binary required, no `go doc` or `--help` parsing. This is reliable, fast, and works in CI without a build step.
4. **Staleness detection.** A `make docs-check` target compares the current generated output against the committed files and fails if they differ. This integrates into CI alongside `fmt-check` and `vet`.
5. **Developer ergonomics.** Running `make docs` regenerates all injectable sections. The workflow is: edit code → `make docs` → commit. No separate documentation step to forget.

### 15.3 Marker Syntax

Generated regions in Markdown files use paired HTML comment markers:

```markdown
Some hand-authored prose above.

<!-- pixe:begin:section-name -->
This content is replaced by `docgen` on every run.
Do not edit manually — changes will be overwritten.
<!-- pixe:end:section-name -->

More hand-authored prose below.
```

**Rules:**

- Markers are HTML comments, invisible in rendered Markdown/HTML.
- The `section-name` is a unique identifier within the file (e.g., `interface`, `format-table`, `sort-flags`, `version`).
- Everything between `begin` and `end` markers (inclusive of the markers themselves) is replaced. The markers are re-emitted in the output so the file remains re-processable.
- Content outside markers is never read, modified, or reformatted by the tool.
- A file with no markers is ignored by the tool (fully hand-authored).
- Markers can appear in any file type: `.md`, `.yml`, `.html`.

### 15.4 Extraction Targets

The `docgen` tool extracts the following categories of facts from the Go source:

#### 15.4.1 Version String

**Source:** `docs/_config.yml` field `version`, `docs/_includes/hero.html` if it references the version.

**Extraction:** Parse `cmd/version.go` for the `version` variable's default value. For tagged builds, read the latest git tag via `git describe --tags --abbrev=0`. The injected value is the latest git tag (e.g., `v2.0.0`).

**Target markers:**

```yaml
# In docs/_config.yml:
# <!-- pixe:begin:version -->
version: "v2.0.0"
# <!-- pixe:end:version -->
```

> **Note:** YAML comment markers (`#`) are used instead of HTML comments since `_config.yml` is not rendered as HTML. The tool recognizes both `<!-- pixe:begin:... -->` and `# <!-- pixe:begin:... -->` patterns.

#### 15.4.2 `FileTypeHandler` Interface

**Source:** `internal/domain/handler.go` — the `FileTypeHandler` interface definition and `MetadataCapability` type.

**Extraction:** Parse the Go AST for the `FileTypeHandler` interface type spec and the `MetadataCapability` const block. Emit as a fenced Go code block with doc comments preserved.

**Target files:**
- `docs/adding-formats.md` — the interface listing in the "The `FileTypeHandler` interface" section.
- `README.md` — not currently shown, but available if desired.

**Marker example in `docs/adding-formats.md`:**

```markdown
### The `FileTypeHandler` interface

<!-- pixe:begin:interface -->
```go
// MetadataCapability declares how a handler supports metadata tagging.
type MetadataCapability int
...
```
<!-- pixe:end:interface -->
```

#### 15.4.3 CLI Flags (Cobra Command Definitions)

**Source:** `cmd/sort.go`, `cmd/verify.go`, `cmd/resume.go`, `cmd/status.go`, `cmd/clean.go`, `cmd/gui.go`, `cmd/query.go`.

**Extraction:** Parse the Go AST for Cobra flag registration calls — `Flags().StringVarP()`, `Flags().BoolVarP()`, `Flags().IntVarP()`, `PersistentFlags().*`, etc. Extract:
- Flag name (long form)
- Short form (if any)
- Default value
- Usage description string

Also extract the command's `Use`, `Short`, and `Long` fields from the `cobra.Command` struct literal.

**Target files:**
- `docs/commands.md` — flag tables for each command.
- `README.md` — flag tables in the usage section.

**Marker example in `docs/commands.md`:**

```html
<!-- pixe:begin:sort-flags -->
<table class="flag-table">
  <thead><tr><th>Flag</th><th>Description</th></tr></thead>
  <tbody>
    <tr><td>-s, --source</td><td>Source directory (default: current working directory)</td></tr>
    ...
  </tbody>
</table>
<!-- pixe:end:sort-flags -->
```

**Marker example in `README.md`:**

```markdown
<!-- pixe:begin:sort-flags -->
| Flag | Description |
|------|-------------|
| `-s, --source` | Source directory containing media files (default: current directory) |
...
<!-- pixe:end:sort-flags -->
```

> **Note:** The same extraction produces different output formats depending on the target file. `docs/commands.md` uses HTML tables (for the accordion styling); `README.md` uses Markdown tables. The `docgen` tool maintains a format mapping per target file, or the marker itself specifies the format: `<!-- pixe:begin:sort-flags format=html -->` vs. `<!-- pixe:begin:sort-flags format=markdown -->`.

#### 15.4.4 Supported Format Table

**Source:** `internal/handler/*/` — each handler package's `Extensions()` return value, `MetadataSupport()` return value, and the package-level doc comment (which describes the date extraction and hashable region strategy).

**Extraction:** For each handler package under `internal/handler/`:
1. Parse the `Extensions()` method body for the returned string slice literal.
2. Parse the `MetadataSupport()` method body for the returned `MetadataCapability` constant.
3. Extract the package-level `// Package` doc comment.

Emit as a table row per handler.

**Target files:**
- `docs/how-it-works.md` — the "Supported file types" section (or update the `format-grid.html` include data).
- `README.md` — the "Supported File Types" table.

**Marker example in `README.md`:**

```markdown
<!-- pixe:begin:format-table -->
| Format | Extensions | Metadata |
|--------|------------|----------|
| JPEG | `.jpg`, `.jpeg` | Embedded EXIF |
| HEIC | `.heic`, `.heif` | XMP sidecar |
| MP4/MOV | `.mp4`, `.mov` | XMP sidecar |
| DNG | `.dng` | XMP sidecar |
| NEF | `.nef` | XMP sidecar |
| CR2 | `.cr2` | XMP sidecar |
| CR3 | `.cr3` | XMP sidecar |
| PEF | `.pef` | XMP sidecar |
| ARW | `.arw` | XMP sidecar |
<!-- pixe:end:format-table -->
```

**Implementation note (resolved):** The `extractFormats` function in `internal/docgen/extract.go` correctly handles inherited `MetadataSupport()` methods from embedded `tiffraw.Base`. TIFF-based handlers (ARW, CR2, DNG, NEF, PEF) inherit `MetadataSupport() → domain.MetadataSidecar` via struct embedding. The extraction logic detects this by checking whether the handler's source file imports the `tiffraw` package; if so, it sets `Metadata = "XMP sidecar"`. The `handlertest` package (test infrastructure) is excluded from the format table output. The CI workflow includes `fetch-tags: true` in the `actions/checkout@v4` step to ensure `git describe --tags` returns the correct version during documentation generation.

#### 15.4.5 Package Reference (Developer Documentation)

**Source:** All packages under `internal/` and `cmd/` — package-level `// Package` doc comments.

**Extraction:** For each package, extract the package name and its full doc comment. Emit as a structured listing grouped by category (core engine, handlers, CLI, infrastructure).

**Target file:** `docs/packages.md` — a new developer-facing page listing all internal packages with their godoc descriptions. This page is **fully generated** (no hand-authored prose between markers — the entire content section is a single marker block).

**Example output:**

```markdown
### Core Engine

**`internal/pipeline`** — Sort orchestrator. Coordinates the full file processing pipeline...

**`internal/discovery`** — File walking and handler registry. Walks the source directory...

**`internal/copy`** — Atomic file copy and post-copy verification engine...

### File Type Handlers

**`internal/handler/jpeg`** — FileTypeHandler for JPEG images...

**`internal/handler/heic`** — FileTypeHandler for HEIC/HEIF images...

...
```

**Navigation:** Add `packages.md` to `docs/_data/navigation.yml` under a "Developer" or "Internals" grouping. This creates a clear delineation between end-user pages (Install, Commands, How It Works) and developer pages (Adding Formats, Package Reference, Contributing).

#### 15.4.6 Query Subcommands

**Source:** `cmd/query_*.go` — each query subcommand's `cobra.Command` definition.

**Extraction:** Same AST-based approach as CLI flags (Section 15.4.3). Extract the subcommand name, description, and any subcommand-specific flags.

**Target files:**
- `docs/commands.md` — the query subcommand table.
- `README.md` — the query subcommand table.

### 15.5 The `docgen` Tool

#### 15.5.1 Implementation

The tool is a standalone Go program at `internal/docgen/` with a `main.go` entry point, invoked via `go run ./internal/docgen`. It is not a shipped binary — it is a development-time tool that runs during the documentation build.

```
internal/
└── docgen/
    ├── main.go           ← Entry point: orchestrates extraction and injection
    ├── extract.go        ← AST-based extraction functions (flags, interfaces, handlers, packages)
    ├── inject.go         ← Marker-based file injection (read file, replace between markers, write)
    ├── formats.go        ← Output formatters (Markdown table, HTML table, code block, YAML)
    └── docgen_test.go    ← Tests for extraction and injection logic
```

**Key design decisions:**

- **Pure `go/ast` + `go/parser`.** No third-party AST libraries. The stdlib parser is sufficient for extracting struct literals, method return values, and const blocks.
- **No `go/types` or `go/packages`.** The tool parses individual files, not full type-checked packages. This keeps it fast and avoids the complexity of loading the full module dependency graph.
- **Deterministic output.** Given the same source files, the tool always produces the same output. No timestamps, no random ordering. Handler packages are sorted alphabetically. Flags are emitted in the order they appear in the source file.
- **Idempotent.** Running the tool twice produces the same result. The tool reads the existing file, replaces marker regions, and writes back only if the content changed. Unchanged files are not touched (preserving filesystem timestamps for build tools).

#### 15.5.2 Injection Algorithm

```
For each target file in the manifest:
  1. Read the file into memory.
  2. Scan for <!-- pixe:begin:NAME --> / <!-- pixe:end:NAME --> marker pairs.
  3. For each marker pair:
     a. Look up the extraction function for NAME.
     b. Run the extraction (AST parse, git tag, etc.).
     c. Format the result for the target file's format (Markdown, HTML, YAML).
     d. Replace the content between markers (inclusive) with:
        <!-- pixe:begin:NAME -->
        <generated content>
        <!-- pixe:end:NAME -->
  4. If any replacements were made and the content differs from the original:
     Write the file.
```

**Error handling:**

- Missing marker pairs (begin without end, or end without begin) → fatal error with file and line number.
- Unknown section name (no extraction function registered) → warning, marker left unchanged.
- AST parse failure on a source file → fatal error with file path and parse error.
- Target file not found → fatal error.

#### 15.5.3 Target Manifest

The tool maintains a hardcoded manifest of target files and their marker-to-extractor mappings:

```go
var targets = []Target{
    {
        File: "docs/_config.yml",
        Sections: map[string]Extractor{
            "version": extractVersion,
        },
    },
    {
        File: "docs/adding-formats.md",
        Sections: map[string]Extractor{
            "interface": extractInterface,
        },
    },
    {
        File: "docs/commands.md",
        Sections: map[string]Extractor{
            "sort-flags":    extractFlags("cmd/sort.go", "html"),
            "sort-desc":     extractCommandDesc("cmd/sort.go"),
            "status-flags":  extractFlags("cmd/status.go", "html"),
            "verify-flags":  extractFlags("cmd/verify.go", "html"),
            "resume-flags":  extractFlags("cmd/resume.go", "html"),
            "clean-flags":   extractFlags("cmd/clean.go", "html"),
            "gui-flags":     extractFlags("cmd/gui.go", "html"),
            "query-flags":   extractFlags("cmd/query.go", "html"),
            "query-subs":    extractQuerySubcommands("html"),
        },
    },
    {
        File: "docs/how-it-works.md",
        Sections: map[string]Extractor{
            "format-table": extractFormats("html"),
        },
    },
    {
        File: "docs/packages.md",
        Sections: map[string]Extractor{
            "package-list": extractPackageReference,
        },
    },
    {
        File: "README.md",
        Sections: map[string]Extractor{
            "sort-flags":    extractFlags("cmd/sort.go", "markdown"),
            "verify-flags":  extractFlags("cmd/verify.go", "markdown"),
            "resume-flags":  extractFlags("cmd/resume.go", "markdown"),
            "status-flags":  extractFlags("cmd/status.go", "markdown"),
            "clean-flags":   extractFlags("cmd/clean.go", "markdown"),
            "gui-flags":     extractFlags("cmd/gui.go", "markdown"),
            "query-flags":   extractFlags("cmd/query.go", "markdown"),
            "query-subs":    extractQuerySubcommands("markdown"),
            "format-table":  extractFormats("markdown"),
        },
    },
}
```

This manifest is the single place that defines what gets generated and where. Adding a new injectable section means adding one entry here and one extraction function.

### 15.6 Page Classification

Documentation pages fall into three categories based on their relationship to generated content:

| Category | Pages | Description |
|---|---|---|
| **Hand-authored** | `ai.md`, `technical.md`, `contributing.md`, `changelog.md`, `install.md`, `index.md` | Fully written by humans (or AI agents). No markers. The `docgen` tool does not touch these files. |
| **Hybrid** | `adding-formats.md`, `commands.md`, `how-it-works.md`, `README.md` | Hand-authored narrative prose with injected code-sourced sections delimited by markers. Humans write the story; the tool fills in the facts. |
| **Generated** | `packages.md` (new) | Content is entirely generated from code. The file has front matter (hand-authored) and a single large marker block. Editing the content section is pointless — it will be overwritten. |

### 15.7 New Page: Package Reference (`docs/packages.md`)

A new developer-facing documentation page that provides a browsable overview of all internal packages, extracted from their godoc comments.

**Front matter:**

```yaml
---
layout: page
title: Package Reference
section_label: Developer Guide
permalink: /packages/
---
```

**Content structure:**

The page opens with a brief hand-authored introduction, then a single generated section:

```markdown
Pixe's internal packages are organized by responsibility. Each package has a
comprehensive godoc comment describing its purpose, design decisions, and key
types. This page is generated from those comments — the source code is the
authoritative reference.

For the full API surface, run `go doc ./internal/<package>` locally or browse
the source on GitHub.

<!-- pixe:begin:package-list -->
(generated content — grouped package listings with doc comments)
<!-- pixe:end:package-list -->
```

**Grouping:** Packages are organized into logical groups:

| Group | Packages |
|---|---|
| **Core Engine** | `pipeline`, `discovery`, `copy`, `verify`, `hash`, `pathbuilder` |
| **Data & Persistence** | `archivedb`, `manifest`, `migrate`, `dblocator`, `domain`, `config` |
| **File Type Handlers** | `handler/jpeg`, `handler/heic`, `handler/mp4`, `handler/tiffraw`, `handler/dng`, `handler/nef`, `handler/cr2`, `handler/cr3`, `handler/pef`, `handler/arw` |
| **Metadata** | `tagging`, `xmp`, `ignore` |
| **User Interface** | `progress`, `cli`, `tui` |

The grouping is defined in the `docgen` tool, not derived from the filesystem. New packages are added to the appropriate group in the tool's configuration.

**Navigation update:** `docs/_data/navigation.yml` gains a new entry:

```yaml
- title: Packages
  url: /packages/
```

Placed after "Adding Formats" and before "Contributing" to group the developer-facing pages together.

### 15.8 Makefile Integration

Two new targets are added to the Makefile:

```makefile
# ---------- documentation -----------------------------------
docs: ## Regenerate documentation from source code
	go run ./internal/docgen

docs-check: ## Check that generated docs are up to date (CI gate)
	@go run ./internal/docgen --check
	@echo "Documentation is up to date."
```

**`make docs`** runs the tool in write mode. It extracts all facts, injects into all target files, and writes any changed files. Output:

```
docs: updated docs/commands.md (sort-flags, verify-flags, clean-flags)
docs: updated docs/packages.md (package-list)
docs: docs/adding-formats.md is up to date
docs: README.md is up to date
docs: updated docs/_config.yml (version)
```

**`make docs-check`** runs the tool in check mode (`--check` flag). It performs all extractions and comparisons but writes nothing. If any target file would change, it exits with code 1 and lists the stale files:

```
docs-check: STALE docs/commands.md (sort-flags differs)
docs-check: STALE docs/_config.yml (version differs)
Error: 2 files are out of date. Run 'make docs' to update.
```

**CI integration:** The `check` target is updated to include `docs-check`:

```makefile
check: fmt-check vet test-unit docs-check ## Run fmt-check + vet + unit tests + docs-check (fast CI gate)
```

This ensures that documentation staleness is caught in the same pre-commit gate as formatting and test failures.

### 15.9 CI Workflow Update

The existing CI workflow (`.github/workflows/ci.yml`) gains a `docs-check` step in the `test` job:

```yaml
- name: Check generated docs are up to date
  run: go run ./internal/docgen --check
```

This runs after tests and before any deployment step. It requires only Go (already set up) and the repository source — no build step, no external tools.

**The stale `pages.yml` workflow** (`.github/workflows/pages.yml`) can be removed. GitHub Pages is configured via the repository settings to build from the `docs/` directory using its built-in Jekyll support — no custom workflow is needed for deployment. The `pages.yml` workflow was an earlier artifact that is superseded by GitHub's automatic Pages deployment.

### 15.10 README Documentation Strategy

The `README.md` retains its current verbosity — it is the "single-page reference" for users who prefer not to navigate to the docs site. However, the following sections transition from hand-maintained to marker-injected:

| README Section | Marker Name | Extraction Source |
|---|---|---|
| `pixe sort` flag table | `sort-flags` | `cmd/sort.go` |
| `pixe verify` flag table | `verify-flags` | `cmd/verify.go` |
| `pixe resume` flag table | `resume-flags` | `cmd/resume.go` |
| `pixe status` flag table | `status-flags` | `cmd/status.go` |
| `pixe clean` flag table | `clean-flags` | `cmd/clean.go` |
| `pixe gui` flag table | `gui-flags` | `cmd/gui.go` |
| `pixe query` flag table | `query-flags` | `cmd/query.go` |
| Query subcommands table | `query-subs` | `cmd/query_*.go` |
| Supported File Types table | `format-table` | `internal/handler/*/` |

Sections that remain hand-authored in the README:
- "What It Does" introduction
- "Key Principles" bullets
- "How It Works" pipeline description
- "Output Format" example
- "Output Naming Convention"
- "Duplicates" explanation
- "Archive Database & Ledger" description
- "Safety Guarantees" list
- "Installation" instructions
- "Configuration File" example
- "Date Fallback Chain"
- "RAW Hashing Strategy"
- "Project Status"

This split keeps the README's narrative voice while eliminating the most common source of drift: flag tables and format lists that must be updated whenever a command gains a new flag or a new handler is added.

### 15.11 Interaction with Existing Documentation Workflow

**Adding a new CLI flag:**
1. Add the flag in `cmd/<command>.go` (Cobra registration).
2. Run `make docs`.
3. The flag automatically appears in `docs/commands.md` and `README.md`.
4. Commit the code change and the updated docs together.

**Adding a new file format handler:**
1. Create the handler package under `internal/handler/<format>/`.
2. Register in `cmd/sort.go`, `cmd/verify.go`, `cmd/resume.go`, `cmd/status.go`.
3. Add the package to the appropriate group in `docgen`'s package grouping config.
4. Run `make docs`.
5. The format appears in the format table in `docs/how-it-works.md` and `README.md`. The package appears in `docs/packages.md`.
6. Update `docs/adding-formats.md` if the new format introduces a novel pattern (hand-authored — this is narrative, not facts).
7. Update `docs/changelog.md` (hand-authored).

**Releasing a new version:**
1. Tag the release: `git tag v2.1.0`.
2. Run `make docs` — the version in `docs/_config.yml` updates to `v2.1.0`.
3. Update `docs/changelog.md` (hand-authored).
4. Commit and push.

**Editing hand-authored pages:**
1. Edit the `.md` file directly. No tool involvement.
2. If the file has markers, avoid editing content between markers (it will be overwritten).
3. `make docs-check` will pass because the markers haven't changed.

### 15.12 Navigation Update

The `docs/_data/navigation.yml` is updated to reflect the audience delineation — end-user pages first, developer pages grouped together:

```yaml
# End-user documentation
- title: Install
  url: /install/
- title: Commands
  url: /commands/
- title: How It Works
  url: /how-it-works/
- title: Technical
  url: /technical/

# Developer documentation
- title: Adding Formats
  url: /adding-formats/
- title: Packages
  url: /packages/
- title: Contributing
  url: /contributing/

# Project
- title: Changelog
  url: /changelog/
```

The "AI" page (`ai.md`) is intentionally omitted from the primary navigation — it is linked from the footer and the Contributing page but is not a primary navigation destination. This keeps the nav bar focused.

### 15.13 Future Considerations

- **Changelog generation** — The changelog (`docs/changelog.md`) is currently hand-authored. A future enhancement could extract changelog entries from git tags and their annotated messages, or from a structured `CHANGELOG.md` at the repo root. This is deferred because changelogs benefit from editorial curation that automated extraction cannot provide.
- **`go doc` server link** — The packages page could link to `pkg.go.dev/github.com/cwlls/pixe-go/internal/...` for full API documentation. However, `internal/` packages are not visible on pkg.go.dev by design. A self-hosted `godoc` or `pkgsite` instance could be added as a future GitHub Pages deployment, but this adds complexity for marginal benefit given the package reference page.
- **Cobra `--help` output validation** — A future CI step could build the binary and compare `pixe sort --help` output against the generated flag tables to catch cases where the AST extraction diverges from runtime behavior (e.g., flags registered in `init()` functions that the AST parser doesn't follow). This is a belt-and-suspenders check, not a primary concern.
