# Architectural Overview: Pixe

## 1. Vision & Strategy

**Pixe** is a Go-based photo and video sorting utility that safely organizes irreplaceable media files into a deterministic, date-based directory structure with embedded integrity checksums.

### Core Problem

Media libraries accumulate across devices, cameras, and cloud exports with inconsistent naming and no structural organization. Pixe provides a single, repeatable operation that transforms a flat directory of media into a chronologically organized, integrity-verified archive — without ever risking the originals.

### North Star Principles

1. **Safety above all else.** No file is ever modified — source or destination. Destination files are byte-identical copies of their source. Every copy is verified before being considered complete.
2. **Native Go execution.** All functionality uses native Go packages. No shelling out to external binaries.
3. **Deterministic output.** Given the same input files, configuration, and system locale, Pixe always produces the same directory structure and filenames.
4. **Modular by design.** New file types are added by implementing a Go interface.

---

## 2. Technical Stack

| Concern | Choice | Rationale |
|---|---|---|
| **Language** | Go | Performance, concurrency primitives, single-binary distribution |
| **CLI Framework** | `spf13/cobra` | Industry-standard Go CLI framework, subcommand support |
| **Configuration** | `spf13/viper` | Config file + env var + flag merging, pairs with Cobra |
| **Image EXIF (read)** | `rwcarlsen/goexif` | No external binary dependency; read-only — no EXIF writing libraries |
| **HEIC/AVIF Parsing** | `dsoprea/go-heic-exif-extractor` | ISOBMFF container-level EXIF extraction |
| **MP4 Parsing** | `abema/go-mp4` | Atom-level access for metadata and keyframe extraction |
| **Hashing** | `crypto/md5`, `crypto/sha1` (default), `crypto/sha256`, `github.com/zeebo/blake3`, `github.com/cespare/xxhash/v2` | Configurable algorithm with numeric ID embedded in filename |
| **Persistence** | SQLite (`modernc.org/sqlite`, CGo-free) | Cumulative registry, concurrent-safe, queryable |
| **Glob Matching** | `bmatcuk/doublestar/v4` | `**` recursive globs, `{alt}` alternatives; superset of `filepath.Match` |
| **Progress Bar** | `charmbracelet/bubbletea` | Elm-architecture TUI; powers opt-in CLI progress bars |
| **Progress Components** | `charmbracelet/bubbles` | Pre-built progress bar widget |
| **Terminal Styling** | `charmbracelet/lipgloss` | Styled text output with adaptive colors |

---

## 3. Version Management

Pixe follows the idiomatic Go convention: the **git tag is the single source of truth** for the version string. No version literals exist in Go source code.

- **Location:** `cmd/version.go` — unexported `version`, `commit`, `buildDate` vars injected via `-ldflags -X`.
- **Build:** GoReleaser (`.goreleaser.yaml`) is the single authority for how binaries are built. The Makefile delegates to `goreleaser build`.
- **Dev builds:** Plain `go build` → `"dev"`. `make build` → `"dev-<commit>"`. Tagged releases → `"0.10.0"`.
- **Consumers:** `pixe version` CLI, pipeline manifest stamping, archive database `runs.pixe_version`.
- **Version bump:** Tag + push. No source file changes needed.

See `cmd/version.go` for the full implementation including `fullVersion()` and the exported `Version()` getter.

---

## 4. Conceptual Design

### 4.1 High-Level Data Flow

```
dirA (read-only source)          dirB (organized destination)
┌──────────────────┐             ┌───────────────────────────────────────────┐
│ IMG_0001.jpg     │  ────────>  │ 2021/12-Dec/                              │
│ IMG_0002.jpg     │   discover  │   20211225_062223-1-7d97e98f...jpg        │
│ DSC_5678.nef     │   filter    │ 2022/02-Feb/                              │
│ DSC_5678.xmp     │   extract   │   20220202_123101-1-447d3060...jpg        │
│ VID_0010.mp4     │   hash      │ 2022/03-Mar/                              │
│ notes.txt        │   copy      │   20220316_232122-1-321c7d6f...nef        │
│                  │   verify    │   20220316_232122-1-321c7d6f...nef.xmp    │
│ .pixe_ledger.json│   carry     │ duplicates/                               │
└──────────────────┘   tag       │   20260306_103000/...                     │
                                 │ .pixe/                                    │
                                 │   pixe.db                                │
                                 └───────────────────────────────────────────┘
```

### 4.2 Pipeline Stages

Each file passes through these stages, tracked in the archive database:

```
pending → extracted → hashed → copied → verified → sidecars carried → tagged → complete
                                     ↓         ↓                          ↓
                                   failed   mismatch                  tag_failed
```

1. **Pending** — Discovered in `dirA`.
2. **Extracted** — Capture date extracted, hashable data region identified.
3. **Hashed** — Checksum computed over the complete file contents.
4. **Copied** — Written to temp file (`.<filename>.pixe-tmp`) in destination directory.
5. **Verified** — Temp file re-hashed; on match, atomically renamed to canonical path. On mismatch, temp file deleted.
6. **Sidecars Carried** — Pre-existing `.aae`/`.xmp` sidecars copied alongside parent. Non-fatal on failure.
7. **Tagged** — Metadata persisted via XMP sidecar for all formats. Handler's `MetadataSupport()` capability is consulted but all current handlers return `MetadataSidecar`.
8. **Complete** — All operations successful.

### 4.3 Pipeline Output

Every discovered file produces exactly one stdout line:

| Verb | Meaning |
|---|---|
| `COPY` | Successfully processed and copied |
| `SKIP` | Not copied — previously imported or unsupported format |
| `DUPE` | Content duplicate of an already-archived file |
| `ERR` | Processing failed at some pipeline stage |

**Sidecar annotations:** When a file has associated sidecar files (`.xmp`, `.aae`) that were carried alongside it, this is indicated with an **inline annotation** on the parent file's output line rather than a separate sub-line. Format: `[+xmp]`, `[+aae]`, or `[+xmp +aae]` if multiple sidecars are present. This keeps the output compact — one line per file, always.

```
COPY IMG_0001.jpg -> ...Photos/2021/12-Dec/20211225_062223-1-abc123.jpg [+xmp]
COPY DSC_5678.nef -> ...Photos/2022/03-Mar/20220316_232122-1-321c7d6f.nef [+xmp +aae]
```

This replaces the previous `+sidecar` sub-line pattern. The inline annotation is used consistently across sort output, the `PlainWriter` event consumer, and verify output (see §7.6).

**Destination path display:** The destination side of sort output includes an **ellipsis prefix with the `--dest` directory's basename**, making it immediately clear which archive the file was sorted into without printing the full absolute path. Format: `...<basename>/<template-path>/<filename>`.

Example: if `--dest /Volumes/NAS/Photos`, the output reads:

```
COPY IMG_0001.jpg -> ...Photos/2021/12-Dec/20211225_062223-1-abc123.jpg
```

This applies to all output paths in sort (`COPY`, `DUPE`, `DRY-RUN`), the `PlainWriter` event consumer, and the `status` command's SORTED section (which displays the destination recorded in the ledger). The ellipsis prefix communicates that the path is relative to a known destination root, not the current working directory.

**Sort summary:** At the end of a sort, a two-line summary is printed:

```
Done. processed=4 duplicates=0 skipped=0 errors=0
(1m 23s)
```

The elapsed time is computed from the pipeline's `startedAt` timestamp. The duration is also sent via the event bus (`RunSummary.Duration`) for the progress bar UI. See §4.13 for how duration is displayed in other contexts.

All outcomes are also streamed to the JSONL ledger and recorded in the database. Colorized output is applied when stdout is a TTY (respects `NO_COLOR`). Suppressed in `--quiet` mode.

### 4.4 Ignore System

Pixe uses `.gitignore`-style ignore patterns powered by `bmatcuk/doublestar/v4`.

- **Hardcoded ignores:** `.pixe_ledger.json`, `.pixeignore`
- **User sources:** `--ignore` CLI flags, `.pixe.yaml` `ignore:` list, `.pixeignore` files in source directories
- **Capabilities:** `**` recursive globs, `{alt}` alternatives, trailing-slash directory matching, nested `.pixeignore` scoping
- **Priority:** All sources merged additively. No negation (`!`) support.

Implementation: `internal/ignore/` — `Matcher` with `Match()`, `MatchDir()`, `PushScope()`, `PopScope()`.

### 4.5 Output Naming Convention

```
YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>
```

- **Algorithm ID:** Single-digit numeric identifier (0=MD5, 1=SHA-1 [default], 2=SHA-256, 3=BLAKE3, 4=xxHash-64). IDs are permanent.
- **Directory structure:** `<dirB>/<YYYY>/<MM>-<Mon>/` — month abbreviation is locale-aware. Configurable via path template (see §4.5.1).
- **Legacy format:** `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` (pre-I2, no algorithm ID). Detected by `_` vs `-` at position 15. Legacy files coexist indefinitely — no mass-rename.

Implementation: `internal/pathbuilder/` — `Build()` function.

#### 4.5.1 Configurable Path Templates

The **directory structure** leading to the filename is user-configurable via `--path-template` flag or `path_template` config key. The **filename itself is not configurable** — it always follows the `YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>` format. This preserves the determinism guarantee and ensures `pixe verify` can always extract the checksum and algorithm from the filename without consulting the database.

**Syntax:** Simple token-based substitution using `{token}` placeholders. No logic, no conditionals, no pipes — just direct variable replacement. This is intentionally simpler than Go `text/template` to prevent users from introducing non-deterministic or fragile path expressions.

**Default template:** `{year}/{month}-{monthname}` — produces the same output as the pre-template hardcoded structure.

**Available tokens:**

| Token | Description | Example |
|---|---|---|
| `{year}` | 4-digit capture year | `2021` |
| `{month}` | 2-digit zero-padded capture month | `12` |
| `{monthname}` | Locale-aware 3-letter month abbreviation | `Dec` |
| `{day}` | 2-digit zero-padded capture day | `25` |
| `{hour}` | 2-digit zero-padded capture hour (24h) | `06` |
| `{minute}` | 2-digit zero-padded capture minute | `22` |
| `{second}` | 2-digit zero-padded capture second | `23` |
| `{ext}` | Lowercase extension without dot | `jpg` |

**Examples:**

| Template | Result for 2021-12-25 06:22:23 |
|---|---|
| `{year}/{month}-{monthname}` | `2021/12-Dec/` (default) |
| `{year}/{month}/{day}` | `2021/12/25/` |
| `{year}` | `2021/` |
| `{year}/{month}-{monthname}/{day}` | `2021/12-Dec/25/` |
| `{year}/{ext}` | `2021/jpg/` |

**Validation rules (enforced at startup, before any files are processed):**

1. Template must not be empty.
2. All `{...}` tokens must be from the known set above. Unknown tokens are a fatal error.
3. Template must not contain path-traversal components (`.` or `..`).
4. Template must not start with `/` (it is always relative to `dirB`).
5. Template must not contain characters invalid in directory names (`:`, `*`, `?`, `"`, `<`, `>`, `|`, null byte).

**Duplicate routing is not affected.** Duplicates always use `duplicates/<run_timestamp>/<template-expanded-path>/` — the `duplicates/` prefix and run timestamp are hardcoded and prepended to whatever the template produces.

**Interaction with `pixe verify`:** Since the filename format is fixed, `parseChecksum()` continues to work unchanged regardless of the directory template. No verify changes needed.

**Config layering:** `path_template` follows the standard priority chain (CLI flag > source-local `.pixe.yaml` > profile > global config > default). This allows a source-local config to override the template for a specific import source.

### 4.6 Duplicate Handling

- **Default:** Duplicates copied to `<dirB>/duplicates/<run_timestamp>/<YYYY>/<MM-Mon>/`.
- **`--skip-duplicates`:** No bytes written. `DUPE` emitted, recorded in DB with `dest_path = NULL`.

### 4.7 Date Fallback Chain

1. EXIF `DateTimeOriginal` — most reliable.
2. EXIF `CreateDate` / `DateTime` — secondary.
3. **February 20, 1902** (Ansel Adams' birthday) — sentinel for undated files.

Filesystem timestamps are explicitly **not used**.

### 4.8 Metadata Tagging

Tags (`--copyright`, `--camera-owner`) are written **after** copy and verify. All formats use XMP sidecars — no handler modifies destination files.

**Copyright template syntax:** The `--copyright` flag uses the same `{token}` syntax as `--path-template` (see §4.5.1). The template is rendered by the same underlying expansion method used for path templates — not Go `text/template` and not `strings.ReplaceAll`. This ensures consistent syntax, consistent validation, and consistent error messages across all user-facing templates.

**Available tokens for copyright:**

| Token | Description | Example |
|---|---|---|
| `{year}` | 4-digit capture year | `2021` |
| `{month}` | 2-digit zero-padded capture month | `12` |
| `{monthname}` | Locale-aware 3-letter month abbreviation | `Dec` |
| `{day}` | 2-digit zero-padded capture day | `25` |

Example: `--copyright "Copyright {year} The Wells Family"` → `"Copyright 2021 The Wells Family"`

The `{hour}`, `{minute}`, `{second}`, and `{ext}` tokens from path templates are **not available** in copyright templates — they have no practical use in a copyright string. Unknown tokens produce a validation error at startup, consistent with path template behavior.

> **Migration note:** The previous `{{.Year}}` Go-template-style syntax is replaced by `{year}`. This is a breaking change to the copyright flag/config value.

| Capability | Strategy | Formats |
|---|---|---|
| `MetadataEmbed` | Write directly into file | (none currently — capability retained in interface for future use) |
| `MetadataSidecar` | XMP sidecar file (`*.ext.xmp`) | All formats including JPEG |
| `MetadataNone` | Skip | (none currently) |

XMP sidecars follow the Adobe naming convention (`<filename>.<ext>.xmp`) and use standard XMP namespaces (`dc:rights`, `xmpRights:Marked`, `aux:OwnerName`). Implementation: `internal/xmp/`.

> **Design rationale:** Pixe never modifies any file — source or destination. Destination files are byte-identical copies of their source. Metadata is always expressed as an accompanying XMP sidecar. This strengthens the integrity guarantee: a verified copy can be re-verified at any point in the future and the hash will always match, regardless of what tagging operations were performed. The `MetadataEmbed` capability is retained in the `FileTypeHandler` interface as a future extension point but no handler currently uses it.

### 4.9 Recursive Source Processing

`--recursive` / `-r` enables descent into subdirectories. Files identified by relative path from `dirA`. A single ledger at `dirA/.pixe_ledger.json` regardless of recursion depth.

### 4.10 Atomic Copy via Temp File

Files are written to `.<filename>.pixe-tmp-<random>` in the destination directory, verified, then atomically renamed via `os.Rename`. Partial/unverified files never appear at canonical paths. Orphaned temp files from interrupted runs are cleaned by `pixe clean`.

Implementation: `internal/copy/` — `Execute()` function.

### 4.11 Source Sidecar Carry

Enabled by default (`--no-carry-sidecars` to disable). During discovery, `.aae` and `.xmp` files are associated with parent media by stem matching (case-insensitive, supports both `IMG.xmp` and `IMG.HEIC.xmp` conventions).

- Carried sidecars are renamed to match parent's destination filename.
- When a carried `.xmp` exists AND tags are configured, Pixe **merges** tags into the carried sidecar (preserving existing XMP data). Controlled by `--overwrite-sidecar-tags`.
- Sidecar carry failure is non-fatal. Sidecars follow duplicate routing.

Implementation: `internal/discovery/sidecar.go` — `associateSidecars()`.

### 4.12 Ledger Write Failure — User Prompt

When the pipeline attempts to create the JSONL ledger in `dirA` and fails (e.g., read-only filesystem, permission denied), the user is prompted before processing begins. This is the **only interactive prompt** in the codebase.

**Prompt behavior:**

```
Warning: cannot create ledger in /path/to/dirA: permission denied
Without a ledger, this source directory will have no record of which files were sorted.
Continue without ledger? [y/N]
```

The prompt is brief — one line stating what happened, one line stating the consequence, one line with the choice. Default is **No** (cancel). Cancellation exits with **exit code 0** and a "Cancelled." message — it is not an error.

**Non-interactive overrides:**

| Flag | Behavior |
|---|---|
| `--yes` / `-y` | Auto-accept: continue without the ledger (no prompt shown) |
| `--no-ledger` | Explicitly acknowledge running without a ledger (no prompt shown, no warning) |

Both flags suppress the interactive prompt. The difference: `--yes` is a general "answer yes to prompts" flag (available for future prompts), while `--no-ledger` is a specific declaration of intent. Either flag results in `slw` staying `nil` and the sort proceeding without a ledger — identical to the current behavior when the warning was silently absorbed.

**Stdin detection:** When stdin is not a TTY (piped input, cron, CI), the prompt cannot be shown. In this case, the behavior matches `--yes` — a warning is printed and the sort continues. This ensures non-interactive usage is never blocked by a prompt. The `--no-ledger` flag is the recommended way to suppress the warning in scripts.

**Scope:** The prompt is implemented in `internal/pipeline/` (where the ledger is opened). The `SortOptions` struct gains `Yes bool` and `NoLedger bool` fields, populated by `cmd/sort.go` from the CLI flags. The prompt reads from `os.Stdin` and writes to `opts.Output`. No new dependencies — `bufio.NewReader(os.Stdin)` is sufficient.

**Database is unaffected.** The archive database in `dirB` is separate from the ledger in `dirA`. A ledger failure does not affect DB recording. The prompt is specifically about the source-side JSONL ledger.

### 4.13 Run Duration Tracking

Sort run elapsed time is tracked and displayed across all contexts where run information appears.

**Storage:** Duration is **computed on the fly** from `started_at` and `finished_at` in the `runs` table — no additional column. When `finished_at` is NULL (interrupted runs), duration is not displayed (or displayed as "—").

**Display locations:**

| Context | Format | Example |
|---|---|---|
| Sort summary (plain text) | Second line in parentheses | `(1m 23s)` |
| Sort summary (event bus) | `RunSummary.Duration` field | `time.Duration` |
| `query run <id>` header | `Duration:` key/value line (after `Finished:`) | `Duration:    1m 23s` |
| `query runs` table | `DURATION` column | `1m 23s` |
| `query run --json` | `"duration_seconds"` field (numeric) | `83.0` |
| `query runs --json` | `"duration_seconds"` per run | `83.0` |

**Formatting rules:**
- Sub-second: `0.8s`
- Seconds: `23s`
- Minutes: `1m 23s`
- Hours: `1h 5m 12s`
- Interrupted/in-progress runs: `—` (em-dash) in table output, `null` in JSON output.

**Implementation:** A shared `formatDuration(started, finished *time.Time) string` helper in `cmd/query_format.go` handles the computation and formatting. The `archivedb.Run` struct already has `StartedAt time.Time` and `FinishedAt *time.Time` — no struct changes needed.

### 4.14 Additional Features

- **Date filters:** `--since` and `--before` flags filter by capture date. Skipped files recorded with `skip_reason = 'outside date range'`.
- **Config auto-discovery:** Source-local `.pixe.yaml` merged with global config. Priority: CLI flags > source-local > profile > global > env > defaults.
- **Config profiles:** `--profile <name>` loads from `~/.pixe/profiles/<name>.yaml`.
- **Verbosity:** `--quiet` (suppresses per-file output), `--verbose` (adds timing info). Mutually exclusive.
- **`pixe stats`:** Archive dashboard showing totals, format breakdown, date range. Supports `--json`.
- **Destination aliases:** See §4.15.

### 4.15 Destination Aliases

`pixe sort --dest @nas` resolves the `@`-prefixed alias to a filesystem path configured in `.pixe.yaml` under the `aliases` map. This saves typing long or environment-specific paths on every invocation.

**Configuration:**

```yaml
# ~/.pixe.yaml (global) or <source>/.pixe.yaml (source-local)
aliases:
  nas: /Volumes/NAS/Photos
  backup: /Volumes/Backup/Archive
  local: ~/Pictures/Sorted
```

**Resolution rules:**

1. If the `--dest` value (from CLI flag, config file, or env var) starts with `@`, the remainder is looked up in the `aliases` map.
2. If the alias is not found, Pixe exits with a fatal error listing the available aliases.
3. If the `--dest` value does not start with `@`, it is used as a literal path (existing behavior, unchanged).
4. Alias resolution happens after config merging but before destination validation — the resolved path goes through the same existence/creation checks as any literal path.
5. Aliases can be used anywhere `dest` is accepted: `--dest` CLI flag, `dest:` key in `.pixe.yaml`, or `PIXE_DEST` env var.

**Config layering:** Aliases follow the standard merge priority. A source-local `.pixe.yaml` can define aliases that augment (not replace) global aliases. On collision, source-local wins. This allows a camera-specific source directory to define `@default` pointing to its preferred archive.

**Scope:** Alias resolution is implemented in the `cmd/` layer only — no packages below `cmd/` are aware of aliases. By the time `config.AppConfig.Destination` is populated, it contains the resolved filesystem path.

---

## 5. Global Constraints

> [!IMPORTANT]
> ### 5.1 Operational Safety
> - **`dirA` is read-only.** Only `.pixe_ledger.json` is written there.
> - **No file modification.** Destination files are byte-identical copies of their source. Metadata is expressed exclusively via XMP sidecars — never written into media files.
> - **Atomic copy-then-verify.** Temp file → re-hash → rename on match.
> - **Full-file hashing.** All handlers hash the complete file contents. Verification is a simple full-file re-hash — no format-specific scoping.
> - **Database-backed resumability.** Per-file state tracked across runs.
> - **Streaming JSONL ledger.** Partial but valid on interruption.
> - **No silent outcomes.** Every file produces stdout output and a ledger entry. Ledger write failures prompt the user before processing begins (see §4.12).
> - **Concurrent-run safety.** SQLite WAL mode + busy-retry.

> [!IMPORTANT]
> ### 5.2 Native Execution
> - No external binary dependencies. No `os/exec` calls for core functionality.

> [!IMPORTANT]
> ### 5.3 Concurrency Model
> - Worker pool pattern. Configurable via `--workers`.
> - Workers handle full pipeline per file. Coordinator goroutine owns DB writes, dedup queries, ledger appends, and progress reporting.
> - Cross-process safety via SQLite WAL mode + `SQLITE_BUSY` retry.

> [!IMPORTANT]
> ### 5.4 Scalability
> - Handles tens to hundreds of thousands of files per run.
> - Memory bounded — files streamed, dedup index in SQLite with indexed lookups.

---

## 6. Filetype Module Contract

### 6.1 Interface

The `FileTypeHandler` interface (defined in `internal/domain/handler.go`) is the extension point for new formats:

- `Detect(filePath) (bool, error)` — Magic-byte verified after extension-based assumption.
- `ExtractDate(filePath) (time.Time, error)` — Capture date with fallback chain.
- `HashableReader(filePath) (io.ReadCloser, error)` — Complete file contents.
- `MetadataSupport() MetadataCapability` — Embed, Sidecar, or None.
- `WriteMetadataTags(filePath, tags) error` — Only called for `MetadataEmbed` handlers.
- `Extensions() []string` — Claimed extensions for fast-path detection.
- `MagicBytes() []MagicSignature` — File header signatures.

### 6.2 Supported Formats (15 handlers)

| Handler | Extensions | Container | Metadata | Notes |
|---|---|---|---|---|
| JPEG | `.jpg`, `.jpeg` | — | Sidecar | |
| HEIC | `.heic`, `.heif` | ISOBMFF | Sidecar | |
| AVIF | `.avif` | ISOBMFF | Sidecar | iPhone 16+, modern Android |
| PNG | `.png` | PNG chunks | Sidecar | `eXIf`/`tEXt` EXIF extraction |
| MP4 | `.mp4`, `.mov` | ISOBMFF | Sidecar | QuickTime atoms |
| TIFF | `.tif`, `.tiff` | TIFF | Sidecar | Must register last among TIFF-based handlers |
| DNG | `.dng` | TIFF | Sidecar | |
| NEF | `.nef` | TIFF | Sidecar | Nikon |
| CR2 | `.cr2` | TIFF | Sidecar | Canon |
| CR3 | `.cr3` | ISOBMFF | Sidecar | Canon RAW X |
| PEF | `.pef` | TIFF | Sidecar | Pentax |
| ARW | `.arw` | TIFF | Sidecar | Sony |
| ORF | `.orf` | TIFF | Sidecar | Olympus |
| RW2 | `.rw2` | TIFF | Sidecar | Panasonic |
| RAF | `.raf` | RAF (custom) | Sidecar | Fujifilm |

### 6.3 RAW Handler Architecture

**Shared base + thin wrapper pattern.** Eight TIFF-based formats embed `tiffraw.Base` (`internal/handler/tiffraw/`), which provides `ExtractDate()`, `HashableReader()`, `MetadataSupport()`, and `WriteMetadataTags()`. Each wrapper supplies only `Extensions()`, `MagicBytes()`, and `Detect()`.

CR3 and RAF are exceptions — they use non-TIFF containers and have standalone handlers.

**Hashable region:** Full file for all handlers. Every handler's `HashableReader()` returns a reader over the complete file contents. This ensures destination files can be re-verified at any time with a simple full-file hash, and eliminates format-specific hash scoping logic.

**Registration:** All handlers registered via `buildRegistry()` in `cmd/helpers.go` — single source of truth. TIFF handler registered last to avoid claiming RAW files with standard TIFF magic bytes.

### 6.4 RAF Handler (Fujifilm)

RAF is Fujifilm's proprietary RAW container. Unlike every other RAW format in Pixe, it is neither TIFF-based nor ISOBMFF-based — it uses a custom binary layout with a fixed header, an offset directory, and three data regions.

**Container layout (Big Endian):**

```
Offset  Size  Content
0x00    16    Magic: "FUJIFILMCCD-RAW " (ASCII, space-padded)
0x10    4     Format version (e.g., "0201")
0x14    8     Camera serial/ID
0x1C    32    Camera model name (null-terminated)
0x3C    4     RAF version string
        --- Offset directory (starting at byte 84) ---
0x54    4     JPEG preview offset
0x58    4     JPEG preview length
0x5C    4     Meta container offset
0x60    4     Meta container length
0x64    4     CFA (raw sensor data) offset
0x68    4     CFA (raw sensor data) length
```

All multi-byte integers are big-endian. The offset directory version field (at byte 0x54 minus some preamble) has known values `0100` and `0159`, but the three offset/length pairs are at the same positions regardless of version.

**Date extraction strategy:**
1. Read the 4-byte JPEG offset at position 0x54 (big-endian).
2. Seek to that offset — the embedded JPEG is a standard JFIF/EXIF image containing the full EXIF metadata block.
3. Parse the JPEG's APP1 segment with `rwcarlsen/goexif` to extract `DateTimeOriginal` → `DateTime` → Ansel Adams fallback.

This is a single offset lookup followed by standard EXIF parsing — simpler than CR3's nested ISOBMFF/UUID traversal.

**Hashable region:** Full file. Consistent with all other handlers — `HashableReader()` returns a reader over the complete file contents.

**Magic bytes:** `"FUJIFILMCCD-RAW "` (16 bytes at offset 0) — fully distinctive, no collision risk with any other format. Fits exactly within the registry's 16-byte `magicReadSize`.

**Detection:** Extension check (`.raf`) AND 16-byte magic verification. No secondary brand check needed (unlike ISOBMFF formats).

**Metadata:** `MetadataSidecar`. Tags written via XMP sidecar (`.raf.xmp`).

**Scope:** Single handler covers all Fujifilm cameras from S2 Pro (2002) through current X-series and GFX models. The header structure and offset directory layout are consistent across all generations — differences in sensor layout (Bayer vs. X-Trans) and compression algorithms are irrelevant to Pixe's sorting pipeline.

**Embedded XMP:** Some cameras write an XMP packet inside the embedded JPEG preview (used for in-camera ratings). The RAF handler ignores this — it is not relevant to sorting.

**Implementation:** `internal/handler/raf/` — standalone handler (no embedded base). Three files: `raf.go`, `raf_test.go`, `fuzz_test.go`.

### 6.5 Shared Test Suite

`handlertest.RunSuite()` provides an 18-test harness (10 standard + 8 edge-case) covering detection, date extraction, hashing, metadata capability, and crash resistance on pathological inputs (empty files, truncated files, corrupt EXIF, mismatched extensions).

---

## 7. CLI Structure

Built with `spf13/cobra`. Configuration layered via `spf13/viper` (flags > source-local config > profile > global config > env > defaults).

### 7.1 Commands

| Command | Purpose |
|---|---|
| `pixe sort` | Primary operation. Discover → process → copy to `dirB`. |
| `pixe verify` | Walk `dirB` (`--dest`), recompute hashes, report mismatches. Auto-detects algorithm from filename. |
| `pixe resume` | Resume interrupted sort from archive database. |
| `pixe query <sub>` | Read-only DB interrogation: `runs`, `run <id>`, `duplicates`, `errors`, `skipped`, `files`, `inventory`. |
| `pixe status` | Source-oriented, ledger-only report of sorting status. No DB required. |
| `pixe stats` | Archive dashboard: totals, format breakdown, date range. Supports `--json`. |
| `pixe clean` | Remove orphaned `.pixe-tmp` files and XMP sidecars; optionally `VACUUM` the database. |
| `pixe version` | Print version, commit, build date. |

**Consistent `--dest` flag across all commands:** Every command that accepts a destination/archive directory uses `--dest` / `-d`. This includes `sort`, `verify`, `resume`, `clean`, `query`, and `stats`. The previous `--dir` flag on verify, resume, clean, query, and stats is renamed to `--dest` for consistency with sort. The `-d` shorthand is unchanged (it was already `-d` on all commands).

Key flags are defined in each command's source file under `cmd/`. See `cmd/helpers.go` for shared configuration resolution (`resolveConfig()`, `buildRegistry()`).

### 7.2 Configuration File

`.pixe.yaml` supports: `dest`, `algorithm`, `workers`, `copyright`, `camera_owner`, `recursive`, `skip_duplicates`, `carry_sidecars`, `overwrite_sidecar_tags`, `ignore` (list), `path_template` (string, see §4.5.1), `aliases` (map of name→path, see §4.15).

**Required-flag validation:** Commands that require `--dest` (sort, verify, resume, clean, query, stats) must **not** use Cobra's `MarkFlagRequired`. Cobra's required-flag check runs before Viper config merging, so a `dest` value in `.pixe.yaml` is rejected because the CLI flag was not explicitly provided. Instead, these commands validate the resolved value after `resolveConfig()` (or equivalent Viper reads) and return a clear error if the value is empty. This allows `dest:` in the config file, `PIXE_DEST` env var, or source-local `.pixe.yaml` to satisfy the requirement without a CLI flag.

### 7.3 Query Command

`pixe query` opens the DB in read-only mode. Supports `--json` for structured output. All subcommands produce fixed-width columnar tables (default) or JSON. Run IDs support prefix matching. See `cmd/query_*.go` for subcommand implementations.

**Truncation with ellipsis:** In table display mode, checksums and run IDs are truncated to the first 8 characters for readability. Truncated values display a trailing ellipsis (`…`) to visually indicate the value is not complete:

```
RUN ID       VERSION   SOURCE              STARTED               STATUS     FILES
a1b2c3d4…    0.10.0    /path/to/source     2026-03-12 14:30:00   completed  42

SOURCE FILE              STATUS     DESTINATION                          CHECKSUM     CAPTURE DATE
IMG_0001.jpg             complete   2021/12-Dec/20211225_062223-1-...    7d97e98f…    2021-12-25
```

Full (untruncated) values are always available in `--json` output. The `truncChecksum()` and `truncID()` helpers both append `…` when the value exceeds 8 characters.

**Run duration in query output:** The `query run <id>` header block includes a `Duration:` line (computed from `started_at` and `finished_at`). The `query runs` table includes a `DURATION` column. See §4.13 for formatting rules.

### 7.4 Status Command

Operates entirely from `dirA` — compares files on disk against `.pixe_ledger.json`. Classifies files as Sorted, Duplicate, Errored, Unsorted, or Unrecognized. No database dependency. Supports `--json`.

### 7.5 Clean Command

Combines orphaned artifact removal (`.pixe-tmp` files, orphaned XMP sidecars) and database compaction (`VACUUM`). Supports `--dry-run`, `--temp-only`, `--vacuum-only` (mutually exclusive). Guards against vacuuming during active sort runs.

### 7.6 Verify Sidecar Awareness

The verify command walks `dirB` and re-hashes media files to confirm integrity. Sidecar files (`.xmp`, `.aae`) are **not** unknown files — they are expected artifacts of the sort process and should not be reported as `UNRECOGNISED`.

**Behavior:**

1. **Skip sidecar files during walk.** When the verify walker encounters a file with a `.xmp` or `.aae` extension, it does not attempt to parse a checksum from the filename, does not hash it, and does not count it as unrecognised. Sidecar files are silently collected during the walk.
2. **Associate sidecars with parent files.** After (or during) the walk, sidecar files are matched to their parent media files by filename stem. For example, `20211225_062223-1-abc123.arw.xmp` is associated with `20211225_062223-1-abc123.arw`.
3. **Inline annotation on parent.** When a verified file has associated sidecars, the verify output includes an inline annotation — the same `[+xmp]` / `[+aae]` format used by sort (see §4.3):

```
  OK          20211225_062223-1-abc123.arw [+xmp]
  OK          20211225_062223-1-abc123.jpg [+xmp +aae]
  MISMATCH    20220202_123101-1-447d3060.jpg [+xmp]
```

4. **Orphaned sidecars.** A sidecar file with no matching parent media file in the same directory is reported as `UNRECOGNISED` — it genuinely is unexpected. This handles cases where a media file was manually deleted but its sidecar was left behind.
5. **Verify summary.** The summary line remains `verified=X mismatches=Y unrecognised=Z`. Sidecar files that are successfully associated with parents do not increment any counter. Orphaned sidecars increment `unrecognised`.

**Implementation:** The sidecar extension check is a simple set lookup (`".xmp"`, `".aae"`). No handler registry involvement — sidecars are identified purely by extension. The association logic mirrors `internal/discovery/sidecar.go` but operates on destination filenames (which follow the Pixe naming convention).

---

## 8. Archive Database & Ledger

### 8.1 Overview

SQLite database at `dirB/.pixe/pixe.db` (or `~/.pixe/databases/<slug>.db` for network mounts). Single source of truth for archive state, dedup, run history, and crash recovery.

### 8.2 Database Location

Priority: `--db-path` flag → `dirB/.pixe/dbpath` marker → `dirB/.pixe/pixe.db`. Network mount detection via OS-level `statfs`. A `dbpath` marker file is written when the DB is stored outside `dirB`.

### 8.3 Schema

Two primary tables: `runs` and `files`. See `internal/archivedb/` for the full schema.

- **`runs`:** `id` (UUID), `pixe_version`, `source`, `destination`, `algorithm`, `workers`, `recursive`, `started_at`, `finished_at`, `status` (running/completed/interrupted). Duration is computed on the fly from `started_at` and `finished_at` — no dedicated column (see §4.13).
- **`files`:** `run_id` (FK), `source_path`, `dest_path`, `dest_rel`, `checksum`, `algorithm`, `status` (12 valid states), `skip_reason`, `is_duplicate`, `capture_date`, `file_size`, timestamps per stage, `error`, `carried_sidecars` (JSON array).
- **Indexes** on `checksum` (where complete), `run_id`, `status`, `source_path`, `capture_date`.
- **Schema versioning** via `schema_version` table. Migrations are additive (`ALTER TABLE ADD COLUMN`).

### 8.4 Concurrency

WAL mode, busy timeout (5s), per-file transaction commits. Cross-process dedup races handled at application level after commit (loser relocates to `duplicates/`).

### 8.5 Ledger (`dirA/.pixe_ledger.json`)

Streaming JSONL format. Line 1 is a header (run metadata), subsequent lines are per-file entries. Written by the coordinator goroutine. Truncated at start of each run. Partial but valid on interruption.

Current version: `5`. Fields: `path`, `status` (copy/skip/duplicate/error), `checksum`, `destination`, `verified_at`, `sidecars`, `matches`, `reason`.

**Write failure handling:** If the ledger cannot be created (permission denied, read-only filesystem, etc.), the user is prompted before processing begins. See §4.12 for the full prompt design, `--yes`, and `--no-ledger` flags.

### 8.6 Migration

Automatic migration from legacy `manifest.json` → SQLite on first encounter. Original preserved as `manifest.json.migrated`.

---

## 9. Pipeline Event Bus

A structured event channel (`internal/progress/`) decouples pipelines from output presentation.

- **Typed events** (`Event` struct with `EventKind`) emitted at each pipeline stage transition.
- **Non-blocking sends** on a buffered channel (256). Events dropped if buffer full — correctness is in DB/ledger.
- **Optional integration** via `SortOptions.EventBus` / `VerifyOptions.EventBus`. When nil, pipeline uses existing `io.Writer` path.
- **`PlainWriter`** consumer writes traditional text output from events.
- **Zero Charm dependencies** in the `progress` package — pure stdlib.

See `internal/progress/event.go` for the full `EventKind` enum and `Event` struct.

### 9.1 Byte-Level Progress Events

The event bus supports sub-file progress reporting via `EventByteProgress` events. These are emitted during I/O-heavy pipeline stages (copy, hash, verify) to enable per-file progress bars in the UI.

**New event kind:**

| EventKind | When Emitted | Key Fields |
|---|---|---|
| `EventByteProgress` | Periodically during copy/hash/verify I/O | `RelPath`, `WorkerID`, `BytesWritten`, `BytesTotal`, `Stage` |

**New `Event` fields:**

| Field | Type | Description |
|---|---|---|
| `BytesWritten` | `int64` | Bytes processed so far in the current stage |
| `BytesTotal` | `int64` | Total file size (from `os.Stat` at pipeline entry) |
| `Stage` | `string` | Current pipeline stage label (see §10.2) |

**Emission strategy:**

- A `ProgressReader` wrapper (in `internal/progress/`) wraps any `io.Reader` and emits `EventByteProgress` at regular intervals (100ms default). A `ProgressWriter` wrapper wraps any `io.Writer` and emits at the same interval. This avoids flooding the bus with per-32KB-buffer events.
- The `ProgressReader` is injected at the call sites in `internal/verify/` (for hash computation) and in `internal/copy/` (for `Verify`), only when an event bus is configured. The `ProgressWriter` is injected in `internal/copy/` (for `Execute`). When the bus is nil, the raw reader/writer is used unchanged — zero overhead for non-progress mode.
- `BytesTotal` is populated from the `os.Stat` performed at the start of `processFile()` / `runWorker()`. The file size is carried on the `Event.FileSize` field (already present in the `Event` struct) and set on `EventFileStart`.

**Throttling:** Both `ProgressReader` and `ProgressWriter` use a time-based throttle (100ms interval) to ensure consistent UI update rates regardless of I/O speed. `ProgressReader` emits a final "100%" event when the reader reaches EOF. `ProgressWriter` provides an `EmitFinal()` method that the pipeline calls after `copy.Execute` completes to ensure the UI sees 100% before the stage-transition event arrives.

### 9.2 File Size on EventFileStart

`EventFileStart` now carries `FileSize` (populated from `os.Stat`). This allows the UI to display file sizes in worker lines and compute byte-level percentages before any `EventByteProgress` arrives. The stat is already performed in the pipeline for mtime preservation — no additional syscall.

---

## 10. CLI Progress Display

Opt-in via `--progress` flag on `sort` and `verify`. Bubble Tea program rendering a multi-line progress display with an overall progress bar and per-worker file status. Auto-disabled when stdout is not a TTY.

When active, `opts.Output` is set to `io.Discard` (progress display replaces scrolling text). Ledger and database continue recording.

Implementation: `internal/cli/` — `ProgressModel` struct.

### 10.1 Startup Hang Fix (Signal Handler Conflict)

**Bug:** When `--progress` is used, the sort command hangs indefinitely on startup until Ctrl+C is pressed, at which point the sort proceeds normally with the progress bar visible.

**Root cause:** In `cmd/sort.go`, `signal.NotifyContext()` registers a Go-level signal handler for `SIGINT`/`SIGTERM` **before** `tea.NewProgram(model).Run()` starts. Bubble Tea's `Run()` also installs its own signal handler for `SIGINT` (to deliver `tea.KeyMsg{Type: tea.KeyCtrlC}` to the model). Go's `signal.Notify` is additive — both handlers receive the signal. However, `signal.NotifyContext` consumes the signal from the OS, and Bubble Tea's internal signal handler never fires. The result: Bubble Tea's terminal initialization completes, but its internal signal relay goroutine is blocked waiting for a signal that the Go runtime's `signal.NotifyContext` intercepts first. This manifests as a hang because Bubble Tea's `Run()` enters its event loop but the terminal is not fully configured — specifically, Bubble Tea waits for its input reader goroutine to start, and the raw-mode terminal switch may be gated on signal setup completion.

When the user presses Ctrl+C: `signal.NotifyContext` fires and cancels the context. The pipeline goroutine's context is cancelled. But Bubble Tea also receives the `SIGINT` (the second signal after `NotifyContext` restores default handling via `stopSignals()`), which causes Bubble Tea's `Run()` to unblock. The pipeline has already started (discovery was happening during the "hang"), so it proceeds with whatever work remains.

**Fix:** Move `signal.NotifyContext` registration to **after** `p.Run()` returns, or better: when `useProgress` is true, do **not** call `signal.NotifyContext` at all. Instead, let Bubble Tea own signal handling. The Bubble Tea model's `Update()` already handles `ctrl+c` / `q` by returning `tea.Quit`. When Bubble Tea quits, `p.Run()` returns, and the `cmd/sort.go` code can cancel the pipeline context explicitly (via a `context.WithCancel` that the progress-mode branch controls). This avoids the dual-handler conflict entirely.

**Implementation pattern:**

```
if useProgress {
    ctx, cancel := context.WithCancel(cmd.Context())   // no signal.NotifyContext
    defer cancel()
    // ... launch pipeline goroutine with ctx ...
    // ... p.Run() blocks (Bubble Tea owns signals) ...
    cancel()  // cancel pipeline context when Bubble Tea exits
    <-done
} else {
    ctx, stopSignals := signal.NotifyContext(...)       // existing behavior
    defer stopSignals()
    // ... run pipeline synchronously ...
}
```

Bubble Tea handles Ctrl+C → model returns `tea.Quit` → `p.Run()` returns → `cancel()` fires → pipeline drains gracefully via `ctx.Done()`. No signal conflict.

### 10.2 Display Layout

The progress display renders a fixed-height, multi-line view:

```
pixe sort  /path/to/source → /path/to/dest

 ████████████░░░░░░░░░░░░░░░░░░  42 / 145  (29%)  ETA 1m 23s

 HASH    IMG_0042.jpg           ████████░░  78%   12.4 MB   ~2s
 COPY    DSC_1234.nef           ██████████  100%  24.8 MB   ~0s
 VERIFY  IMG_0039.jpg           ██████░░░░  61%    8.1 MB   ~1s
 TAG     IMG_0038.heic          ████████░░  done   4.2 MB

 copied: 38  │  dupes: 2  │  skipped: 1  │  errors: 0
```

**Structure (top to bottom):**

1. **Header line** — command, source, destination.
2. **Overall progress bar** — files completed / total files (file-count based, not byte-based). Percentage and ETA.
3. **Worker lines** — one line per active worker. Number of visible lines = `cfg.Workers` (or 1 for sequential mode). Each line shows:
   - **Stage label** (first column, fixed width) — the pipeline stage the file is currently in.
   - **Filename** — basename of the file being processed (truncated if needed).
   - **Per-file progress bar** — byte-level progress within the current I/O stage (copy, hash, verify). Non-I/O stages (extract, tag) show a spinner or "done".
   - **Percentage** — byte-level percentage, or "done" for non-I/O stages.
   - **File size** — human-readable (e.g., "12.4 MB").
   - **Time estimate** — per-file ETA based on byte throughput in the current stage.
4. **Status counters** — aggregate counts (copied, dupes, skipped, errors for sort; verified, mismatches, unrecognised for verify).

**Stage labels:**

| Label | Pipeline Stage | Has Byte Progress? |
|---|---|---|
| `DISC` | Discovery/walk phase (pre-file) | No (overall bar only) |
| `HASH` | Computing checksum (`hash.Sum`) | Yes |
| `COPY` | Streaming to temp file (`copy.Execute`) | Yes |
| `VERIFY` | Re-hashing temp file (`copy.Verify`) | Yes |
| `TAG` | Writing metadata sidecar | No |
| `DONE` | File complete, waiting for next assignment | No |

For verify mode, the stage labels are:

| Label | Verify Stage | Has Byte Progress? |
|---|---|---|
| `HASH` | Re-computing checksum for verification | Yes |
| `DONE` | File verified | No |

**Worker line lifecycle:**

1. `EventFileStart` → worker line appears with filename and stage `HASH`.
2. `EventByteProgress` → per-file progress bar updates.
3. `EventFileExtracted` → (no visible stage change — extraction is sub-second).
4. `EventFileHashed` → stage changes to `COPY`, byte progress resets to 0.
5. `EventFileCopied` → stage changes to `VERIFY`, byte progress resets to 0.
6. `EventFileVerified` → stage changes to `TAG`.
7. `EventFileTagged` / `EventFileComplete` / `EventFileDuplicate` → worker line cleared (ready for next file).
8. When no file is assigned, the worker line is blank (not rendered).

**Idle workers:** Worker lines are only rendered for workers that are actively processing a file. If 4 workers are configured but only 2 have active files (e.g., near the end of a run), only 2 worker lines are shown. This keeps the display compact.

### 10.3 Model State

The `ProgressModel` struct tracks per-worker state in addition to aggregate counters:

```go
type WorkerState struct {
    WorkerID     int
    RelPath      string    // filename being processed
    Stage        string    // current stage label
    FileSize     int64     // total bytes
    BytesWritten int64     // bytes processed in current I/O stage
    StageStart   time.Time // when the current stage began (for per-file ETA)
}
```

The model maintains a `map[int]*WorkerState` keyed by `WorkerID`. Entries are created on `EventFileStart` and removed on terminal events. The `View()` function iterates over active workers in `WorkerID` order to render stable, non-jumping lines.

### 10.4 Discovery Phase Display

During the discovery walk (before `EventDiscoverDone`), the overall progress bar shows an indeterminate state (e.g., a spinner or pulsing bar) with the text "Discovering files..." instead of "0 / 0 (0%)". This provides immediate visual feedback that the command is running, eliminating the perception of a hang even for large directories.

Once `EventDiscoverDone` arrives, the bar switches to determinate mode with the file count.

### 10.5 Verify Parallelization

The `pixe verify` command is parallelized to match the sort command's worker pool pattern:

**Design:**

- `verify.Options` gains a `Workers int` field (defaults to `cfg.Workers` from `--workers` flag, minimum 1).
- When `Workers > 1`, `verify.Run()` uses a worker pool: the coordinator goroutine walks the directory and feeds file paths into a work channel; workers read files, recompute hashes, and send results back.
- Workers own I/O (reading files, computing hashes). The coordinator owns result aggregation and event emission (to maintain ordered `Completed` counters).
- Each worker emits `EventByteProgress` events with its `WorkerID` during hash computation, enabling per-worker progress lines in the UI.
- The `--workers` flag on the verify command uses the same Viper key as sort (`workers`), so a global config value applies to both commands.

**Event changes for verify:**

- `EventVerifyOK`, `EventVerifyMismatch`, and `EventVerifyUnrecognised` gain a `WorkerID` field (currently unused — set to 0 for the single-threaded walker).
- New: `EventVerifyFileStart` — emitted when a worker begins processing a file. Carries `RelPath`, `WorkerID`, `FileSize`. This enables the UI to show the file in the worker line before any byte progress arrives.

**Verify worker line stages:**

| Label | Stage | Has Byte Progress? |
|---|---|---|
| `HASH` | Reading and hashing file contents | Yes |

Verify workers only have one I/O stage (hash), so the per-file progress bar directly reflects hash progress. The worker line appears on `EventVerifyFileStart` and disappears on the terminal event (`EventVerifyOK` / `EventVerifyMismatch` / `EventVerifyUnrecognised`).

**Concurrency safety:** Verify is read-only (no DB writes, no file mutations), so workers need no coordinator-mediated serialization. The only shared state is the `Result` struct, which the coordinator updates from the result channel.

### 10.6 Bubble Tea Program Configuration

The Bubble Tea program is created with `tea.WithoutSignalHandler()` when running in progress mode. This prevents Bubble Tea from installing its own `SIGINT` handler, which would conflict with the explicit `context.WithCancel` pattern described in §10.1. Instead, the model handles `ctrl+c` via `tea.KeyMsg` (which Bubble Tea delivers from its raw-mode terminal input reader, independent of OS signals). This is cleaner than letting two signal handlers compete.

Additionally, `tea.WithoutSignals()` is **not** used (that would disable the `tea.KeyMsg` delivery for Ctrl+C). The distinction:
- `tea.WithoutSignalHandler()` — disables the OS-level `signal.Notify` for `SIGINT`. Bubble Tea still reads Ctrl+C from stdin as a key event.
- The model's `Update()` handles `tea.KeyMsg` for "ctrl+c" by returning `tea.Quit`, which causes `p.Run()` to return, which triggers the `cancel()` on the pipeline context.

---

## 11. Documentation Site (`docs/`)

Jekyll-based static site deployed to GitHub Pages from `docs/`. Uses the **Just the Docs** theme (`just-the-docs`) — a documentation-focused Jekyll theme with built-in sidebar navigation, search, and breadcrumbs.

### 11.1 Content Principles

- **Strict markdown.** All `.md` files in `docs/` are written in standard GitHub-Flavored Markdown. No custom CSS classes, no `<div>` layouts, no inline styles, no `onclick` handlers.
- **HTML is the exception, not the rule.** An occasional HTML tag (e.g., an anchor with `target="_blank"`) is acceptable when markdown has no equivalent. HTML blocks for layout, styling, or interactivity are not.
- **No custom theme assets.** No `_sass/`, `_layouts/`, `_includes/`, or `assets/css/` directories. The Just the Docs theme provides all styling. The site should work with zero local theme overrides.
- **No custom JavaScript.** No `<script>` tags, no accordion toggles, no interactive elements. Content is static markdown rendered by the theme.

### 11.2 Theme Migration: Slate → Just the Docs

The site has migrated from `jekyll-theme-slate` to `just-the-docs`. Slate was a single-page presentation theme with no built-in multi-page navigation — it had no sidebar, no top nav bar, no search, and no breadcrumbs. With 11+ pages (and growing), navigating the site required returning to the index page every time. Just the Docs is purpose-built for documentation sites and provides all of this out of the box via `_config.yml` configuration.

**What changes:**

1. **`_config.yml`** — Replace `theme: jekyll-theme-slate` with `remote_theme: just-the-docs/just-the-docs`. Add Just the Docs configuration keys (see §11.3).
2. **`Gemfile`** — Replace the `github-pages` gem with `jekyll-remote-theme` plugin and the `just-the-docs` gem for local preview.
3. **Front matter on every `.md` file** — Add `nav_order` (integer) to control sidebar ordering. Some pages gain `parent` for hierarchical grouping. The `title` key (already present on all pages) becomes the sidebar link label.
4. **`index.md`** — Remove the manual navigation link list (the sidebar replaces it). The landing page content (project description, quick start, FAQ) remains. Add `nav_order: 1` to front matter.
5. **Remove manual "back to index" links** — Individual pages no longer need inline navigation links to other pages. The persistent sidebar handles all cross-page navigation.

**What does NOT change:**

- All markdown content within pages remains identical. No HTML rewriting needed.
- Marker-based docgen injection (`<!-- pixe:begin:... -->`) works the same — markers are HTML comments, invisible in any theme.
- `_config.yml` version marker (`# <!-- pixe:begin:version -->`) is preserved.
- No `_layouts/`, `_includes/`, `_sass/`, or `assets/` directories are added. Just the Docs is used as a remote theme with zero local overrides.

### 11.3 Theme Configuration

`docs/_config.yml` after migration:

```yaml
remote_theme: just-the-docs/just-the-docs
plugins:
  - jekyll-remote-theme

title: Pixe
description: Safe, deterministic photo and video sorting
url: "https://cwlls.github.io"
baseurl: "/pixe"

# <!-- pixe:begin:version -->
version: "v2.6.2"
# <!-- pixe:end:version -->

# Just the Docs configuration
search_enabled: true
search:
  heading_level: 2
  previews: 3

# Aux links (top-right corner)
aux_links:
  "View on GitHub":
    - "https://github.com/cwlls/pixe"
aux_links_new_tab: true

# Footer
gh_edit_link: false

# Color scheme
color_scheme: light

# Heading anchor links
heading_anchors: true

exclude:
  - README.md
  - Gemfile
  - Gemfile.lock
```

`docs/Gemfile` after migration:

```ruby
source "https://rubygems.org"

gem "jekyll", "~> 4.3"
gem "just-the-docs"
gem "jekyll-remote-theme"
```

### 11.4 Navigation Structure

Just the Docs generates the sidebar automatically from front matter on each page. Navigation order is controlled by `nav_order` (lower numbers appear first). All pages are top-level — no nested parent/child hierarchy needed for this site's size.

| Page | `nav_order` | Sidebar Label |
|---|---|---|
| `index.md` | 1 | Home |
| `install.md` | 2 | Installation |
| `commands.md` | 3 | Commands |
| `configuration.md` | 4 | Configuration |
| `how-it-works.md` | 5 | How It Works |
| `technical.md` | 6 | Technical Design |
| `adding-formats.md` | 7 | Adding a New Format |
| `packages.md` | 8 | Package Reference |
| `contributing.md` | 9 | Contributing |
| `changelog.md` | 10 | Changelog |
| `ai.md` | 11 | AI Collaboration |

Front matter example for a typical page:

```yaml
---
title: Commands
nav_order: 3
---
```

The `index.md` landing page uses:

```yaml
---
title: Home
nav_order: 1
permalink: /
---
```

The manual navigation list currently in `index.md` (lines 17–27) is removed — the sidebar replaces it. The rest of `index.md` (project description, FAQ, quick start) is unchanged.

### 11.5 New Page: `configuration.md`

A new `docs/configuration.md` page is created to comprehensively document the configuration system. The config file section currently at the bottom of `commands.md` (lines 285–302) is removed from `commands.md` and replaced by a cross-reference link to the new page.

**Page structure:**

1. **Precedence order** — The first section. Documents the full resolution chain from highest to lowest priority:

   | Priority | Source | Example |
   |---|---|---|
   | 1 (highest) | CLI flags | `--algorithm sha256` |
   | 2 | Environment variables | `PIXE_ALGORITHM=sha256` |
   | 3 | Source-local config | `<source-dir>/.pixe.yaml` |
   | 4 | Named profile | `~/.pixe/profiles/<name>.yaml` (via `--profile`) |
   | 5 | Global config file | `~/.pixe.yaml` or `$XDG_CONFIG_HOME/pixe/.pixe.yaml` |
   | 6 (lowest) | Built-in defaults | Hardcoded in source |

   Explains that Viper resolves in this order. CLI flags always win. Env vars are prefixed `PIXE_` and use `SCREAMING_SNAKE_CASE` (e.g., `PIXE_SKIP_DUPLICATES=true`). Source-local config is automatically loaded from the `--source` directory. Profiles are explicitly selected via `--profile <name>`.

2. **Global config file** — Search paths in order: `./.pixe.yaml` (current directory), `$HOME/.pixe.yaml`, `$XDG_CONFIG_HOME/pixe/.pixe.yaml`. YAML format. Full annotated example showing every valid key.

3. **Settings reference table** — Every valid config key, its CLI flag counterpart, type, default, and description:

   | Config Key | CLI Flag | Type | Default | Description |
   |---|---|---|---|---|
   | `dest` | `-d, --dest` | string | (required) | Destination archive directory. Supports `@alias` syntax (see Aliases). |
   | `algorithm` | `-a, --algorithm` | string | `sha1` | Hash algorithm: `md5`, `sha1`, `sha256`, `blake3`, `xxhash` |
   | `workers` | `-w, --workers` | int | CPU count | Concurrent worker count |
   | `recursive` | `-r, --recursive` | bool | `false` | Descend into source subdirectories |
   | `skip_duplicates` | `--skip-duplicates` | bool | `false` | Skip copying duplicate files (record only) |
   | `copyright` | `--copyright` | string | (disabled) | Copyright template with `{year}`, `{month}`, `{monthname}`, `{day}` tokens |
   | `camera_owner` | `--camera-owner` | string | (disabled) | Camera owner metadata string |
   | `path_template` | `--path-template` | string | `{year}/{month}-{monthname}` | Directory structure template (see §4.5.1 in architecture) |
   | `no_carry_sidecars` | `--no-carry-sidecars` | bool | `false` | Disable sidecar file carry (`.aae`, `.xmp`) |
   | `overwrite_sidecar_tags` | `--overwrite-sidecar-tags` | bool | `false` | Overwrite existing XMP values during sidecar merge |
   | `db_path` | `--db-path` | string | (auto-resolved) | Explicit SQLite database path |
   | `dry_run` | `--dry-run` | bool | `false` | Preview mode — no copy, verify, or tag operations |
   | `ignore` | `--ignore` | list | (none) | Glob patterns to exclude from discovery |
   | `since` | `--since` | string | (none) | Date filter: process files on or after `YYYY-MM-DD` |
   | `before` | `--before` | string | (none) | Date filter: process files on or before `YYYY-MM-DD` |
   | `aliases` | (none) | map | (none) | Destination aliases: `name: /path` (see Aliases section) |

   **Flags not available in config files** (CLI-only, documented here for completeness):

   | CLI Flag | Type | Default | Description |
   |---|---|---|---|
   | `--config` | string | (auto-discovered) | Explicit config file path |
   | `--profile` | string | (none) | Named profile to load |
   | `-q, --quiet` | bool | `false` | Suppress per-file output |
   | `-v, --verbose` | bool | `false` | Enable verbose output |
   | `--progress` | bool | `false` | Show live progress bar UI |
   | `-y, --yes` | bool | `false` | Auto-accept interactive prompts (e.g., ledger write failure) |
   | `--no-ledger` | bool | `false` | Explicitly skip ledger creation (no prompt, no warning) |

4. **Source-local config** — Explains that a `.pixe.yaml` in the `--source` directory is automatically detected and merged. Documents which keys are merged from source-local config (the 9 keys handled by `mergeSourceConfig`: `dest`, `copyright`, `camera_owner`, `algorithm`, `recursive`, `skip_duplicates`, `no_carry_sidecars`, `overwrite_sidecar_tags`, `path_template`). Notes that `ignore` and `aliases` are merged additively (union, not replacement).

5. **Named profiles** — Explains `--profile <name>` loading from `~/.pixe/profiles/<name>.yaml` or `$XDG_CONFIG_HOME/pixe/profiles/<name>.yaml`. Same merge rules as source-local config. Use case: different settings for different cameras or archive destinations.

6. **Destination aliases** — Explains the `aliases` map and `@name` syntax for `--dest`. Resolution rules, config layering (source-local aliases augment global aliases, collision = source-local wins), error on unknown alias. Full example.

7. **Environment variables** — Lists the `PIXE_` prefix convention. Viper's `AutomaticEnv()` means any config key can be set via `PIXE_<KEY>` (e.g., `PIXE_ALGORITHM`, `PIXE_WORKERS`, `PIXE_DEST`). Bool values accept `true`/`false`/`1`/`0`.

8. **Full annotated example** — A complete `.pixe.yaml` showing every key with inline comments.

**Cross-references:** `commands.md` gains a brief note at the bottom: "For configuration file documentation, precedence rules, profiles, and aliases, see [Configuration](configuration.md)." The config section currently in `commands.md` (lines 285–302) is removed.

### 11.6 Changes to `commands.md`

Two changes to the existing commands page:

1. **Remove config file section** (lines 285–302) — replaced by `configuration.md`. Add a cross-reference link.
2. **Add `--yes` and `--no-ledger` to the sort flags table** — These flags are registered in `cmd/sort.go` but are currently missing from the docgen-injected flag table. The `extractFlags` AST parser should already be picking them up; if the generated table includes them, a `make docs` run will add them automatically. If the parser is not extracting them (e.g., because they use a different registration pattern), the extractor needs a fix.

### 11.7 Previous Migrations (Completed)

The migration from the original custom theme to Slate has been completed. The migration from Slate to Just the Docs (described in §11.2) has also been completed.

Historical custom theme artifacts that were previously deleted: `_sass/`, `_layouts/`, `_includes/`, `assets/`, `_data/`, `_site/`, `.jekyll-cache/`.

---

## 12. Documentation Generation (`docgen`)

`internal/docgen/` — a development-time Go tool (`go run ./internal/docgen`) that extracts code-sourced facts from the Go AST and injects them into documentation files via marker-based replacement.

- **Marker syntax:** `<!-- pixe:begin:section-name -->` / `<!-- pixe:end:section-name -->` — content between markers is replaced. The markers themselves are HTML comments, which is an acceptable use of HTML in the markdown files (they are invisible in rendered output).
- **Output format:** All generated content is **markdown** — markdown tables, markdown lists, fenced code blocks. No HTML tables or HTML layout in generated output.
- **Extraction targets:** Version string (git tag), `FileTypeHandler` interface, CLI flags (Cobra registrations), supported format table, package reference (godoc comments), query subcommands, changelog.
- **Page classification:** Hand-authored (no markers), Hybrid (narrative + injected facts), Generated (`packages.md`, `changelog.md`).
- **Makefile:** `make docs` (regenerate), `make docs-check` (CI staleness gate).
- **Docgen output:** The `docgen` tool emits markdown tables for all documentation targets. All generated content is markdown-only — no HTML tables or HTML layout.

### 12.1 Changelog Sync

The root `CHANGELOG.md` is the single source of truth for the project's version history. The docs site page `docs/changelog.md` is a **generated** file — its version history content is injected by docgen from `CHANGELOG.md`, ensuring the two never drift apart.

**Design:**

- **Source of truth:** `CHANGELOG.md` (root) — maintained manually or by tooling. Contains the full version history in [Keep a Changelog](https://keepachangelog.com/) format.
- **Target:** `docs/changelog.md` — a Jekyll page with front matter (`title: Changelog`) and `<!-- pixe:begin:changelog -->` / `<!-- pixe:end:changelog -->` markers. Everything between the markers is replaced on each `make docs` run.
- **Extractor:** `extractChangelog()` in `internal/docgen/extract.go` — reads `CHANGELOG.md`, strips the title line (`# Changelog: Pixe`) and the preamble italics line, and returns the remaining content as-is. No reformatting — the changelog content is already valid markdown.
- **Target registration:** `buildTargets()` in `main.go` includes `docs/changelog.md` with a single `changelog` section mapped to `extractChangelog`.
- **Staleness gate:** `make docs-check` detects drift between `CHANGELOG.md` and `docs/changelog.md` the same way it detects drift in all other docgen targets.

**Workflow:** Edit `CHANGELOG.md` → run `make docs` → `docs/changelog.md` is updated automatically. CI enforces freshness via `make docs-check`.

### 12.2 Pre-Commit Hook

A Git pre-commit hook prevents stale documentation from being committed. This catches the problem locally — before CI — eliminating the "push, wait for CI, see docs-check failure, run `make docs`, push again" cycle.

**Implementation:** A shell script at `scripts/pre-commit` that is installed into `.git/hooks/pre-commit`. Not a third-party framework (no `pre-commit`, `husky`, or `lefthook` — the project has no Node.js or Python dependencies to justify those tools).

**Hook behavior:**

1. Run `go run ./internal/docgen --check` (same command as CI).
2. If it exits 0, the commit proceeds.
3. If it exits non-zero, the commit is blocked with a message:

   ```
   Documentation is out of date. Run 'make docs' and stage the changes.
   Stale files:
     docs/commands.md
     README.md
   ```

4. The hook is **fast** — docgen extracts from the Go AST and compares strings. No compilation of the main binary, no test execution. Typical runtime: < 2 seconds.

**Installation:**

- A new `make install-hooks` Makefile target copies `scripts/pre-commit` to `.git/hooks/pre-commit` and sets the executable bit.
- `make install-hooks` is documented in `AGENTS.md` and `docs/contributing.md`.
- The hook is **opt-in** — it is not automatically installed by `make build` or `make deps`. Contributors must run `make install-hooks` once after cloning. This is deliberate: auto-installing hooks is surprising behavior, and some workflows (CI, automated tooling) should not have hooks.
- A `make uninstall-hooks` target removes the hook.

**Script (`scripts/pre-commit`):**

```bash
#!/usr/bin/env bash
set -euo pipefail

# Only check docs if any relevant source files are staged.
# Relevant files: cmd/*.go, internal/domain/handler.go, internal/handler/**/*.go,
# internal/docgen/*.go, CHANGELOG.md, docs/*.md, docs/_config.yml
RELEVANT_PATTERNS="cmd/.*\.go|internal/domain/handler\.go|internal/handler/.*\.go|internal/docgen/.*\.go|CHANGELOG\.md|docs/.*\.md|docs/_config\.yml"

STAGED=$(git diff --cached --name-only --diff-filter=ACMR)
if ! echo "$STAGED" | grep -qE "$RELEVANT_PATTERNS"; then
    exit 0  # No relevant files staged — skip docs check
fi

echo "pre-commit: checking generated documentation..."
if ! go run ./internal/docgen --check 2>&1; then
    echo ""
    echo "Run 'make docs' to regenerate, then 'git add' the updated files."
    exit 1
fi
```

**Key design decisions:**

- **Selective execution.** The hook only runs `docgen --check` when staged files include sources that could affect generated docs (cmd/*.go, handler files, CHANGELOG.md, etc.). Commits that only touch tests, configs, or non-doc Go files skip the check entirely — zero overhead for most commits.
- **No auto-fix.** The hook does not run `make docs` automatically. Auto-modifying the working tree during a commit is surprising and can cause confusion with partial staging (`git add -p`). The developer runs `make docs` explicitly, reviews the diff, and stages.
- **No `--no-verify` shame.** The hook prints a clear, actionable message. Developers who need to bypass it for a valid reason can use `git commit --no-verify`.

**Makefile additions:**

```makefile
install-hooks: ## Install git pre-commit hook for docs freshness check
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed."

uninstall-hooks: ## Remove git pre-commit hook
	rm -f .git/hooks/pre-commit
	@echo "Pre-commit hook removed."
```

**CI is unchanged.** The pre-commit hook is a local convenience — CI remains the authoritative gate. The `go run ./internal/docgen --check` step in `.github/workflows/ci.yml` stays exactly as-is.

### 12.3 CI Failure Resolution

The docs-check CI step that was failing due to stale documentation (`docs/commands.md`, `README.md`, `docs/changelog.md`) has been resolved. The fix involved:

1. Running `make docs` to regenerate all marker-injected content.
2. Adding new flags (`--yes`, `--no-ledger`) to the sort flags tables, updating changelog content, and syncing other recent source changes.
3. Committing the regenerated files.

The pre-commit hook (§12.2) now prevents recurrence by catching stale documentation before commits are made.

---

## 13. Open Questions & Future Considerations

1. **Re-enabling `MetadataEmbed` for JPEG** — The interface capability is retained. A future decision could re-enable EXIF embedding for JPEG destinations if the tradeoff (stronger tagging vs. byte-identical copies) shifts.
2. **Cloud storage targets** — `dirB` on S3, GCS, etc.
3. **Multi-archive federation** — Querying across multiple `dirB` databases.
4. **Extended XMP fields** — Additional fields beyond Copyright and CameraOwner (keywords, captions, GPS, ratings).
5. **Split-brain network dedup** — Multi-machine NAS scenarios with separate local databases. `O_EXCL` temp file locking strategy deferred.

---

## 14. Testing & Quality

All testing initiatives are fully implemented.

### 14.1 Fuzz Testing

Go's `testing.F` fuzzer targets `Detect()`, `ExtractDate()`, and `HashableReader()` for 8 handler packages: JPEG, HEIC, AVIF, MP4, CR3, PNG, RAF, and tiffraw. TIFF-based wrappers (DNG, NEF, etc.) are covered via the tiffraw fuzz test.

- Fuzz files: `internal/handler/<format>/fuzz_test.go`
- Only failure condition is a panic — errors and fallbacks are valid.
- Corpus entries committed to `testdata/fuzz/` for regression.
- `make fuzz` runs all targets (30s each).

### 14.2 Benchmark Suite

Centralized at `internal/benchmark/` with 5 benchmark files:

| Benchmark | What It Measures |
|---|---|
| `hash_bench_test.go` | Throughput per algorithm × file size |
| `copy_bench_test.go` | Atomic copy-then-verify throughput |
| `db_bench_test.go` | Insert, dedup check, skip check latency × DB size |
| `discovery_bench_test.go` | Walk speed × tree size and structure |
| `pathbuilder_bench_test.go` | Path construction speed |

Fixtures generated at runtime (not committed). `make bench` runs the full suite.

### 14.3 Fixture Corpus

8 edge-case helpers in `handlertest` package: `BuildEmptyFile`, `BuildMagicOnly`, `BuildTruncatedFile`, `BuildWithFilename`, `BuildSymlink`, plus handler-specific corrupt EXIF builders. `RunSuite()` exercises 18 subtests including crash-resistance on pathological inputs.

Discovery-level edge-case tests cover symlinks, permissions, Unicode paths.

### 14.4 Property-Based Testing

6 properties verified for `pathbuilder.Build()` via `testing/quick` (5,000–10,000 iterations each): determinism, valid path characters, correct structure, extension preservation, date encoding, algorithm ID presence.

File: `internal/pathbuilder/pathbuilder_prop_test.go`. Runs as part of `make test`.
