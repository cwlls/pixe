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
| **TIFF/RAW Parsing** | `golang.org/x/image/tiff` | IFD traversal for EXIF extraction in TIFF-based RAW formats |
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
- **Directory structure:** `<dirB>/<YYYY>/<MM>-<Mon>/` — month abbreviation is locale-aware.
- **Legacy format:** `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` (pre-I2, no algorithm ID). Detected by `_` vs `-` at position 15. Legacy files coexist indefinitely — no mass-rename.

Implementation: `internal/pathbuilder/` — `Build()` function.

### 4.6 Duplicate Handling

- **Default:** Duplicates copied to `<dirB>/duplicates/<run_timestamp>/<YYYY>/<MM-Mon>/`.
- **`--skip-duplicates`:** No bytes written. `DUPE` emitted, recorded in DB with `dest_path = NULL`.

### 4.7 Date Fallback Chain

1. EXIF `DateTimeOriginal` — most reliable.
2. EXIF `CreateDate` / `DateTime` — secondary.
3. **February 20, 1902** (Ansel Adams' birthday) — sentinel for undated files.

Filesystem timestamps are explicitly **not used**.

### 4.8 Metadata Tagging

Tags (`--copyright` with `{{.Year}}` template, `--camera-owner`) are written **after** copy and verify. All formats use XMP sidecars — no handler modifies destination files.

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

### 4.12 Additional Features

- **Date filters:** `--since` and `--before` flags filter by capture date. Skipped files recorded with `skip_reason = 'outside date range'`.
- **Config auto-discovery:** Source-local `.pixe.yaml` merged with global config. Priority: CLI flags > source-local > profile > global > env > defaults.
- **Config profiles:** `--profile <name>` loads from `~/.pixe/profiles/<name>.yaml`.
- **Verbosity:** `--quiet` (suppresses per-file output), `--verbose` (adds timing info). Mutually exclusive.
- **`pixe stats`:** Archive dashboard showing totals, format breakdown, date range. Supports `--json`.

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
> - **No silent outcomes.** Every file produces stdout output and a ledger entry.
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
| `pixe verify` | Walk `dirB`, recompute hashes, report mismatches. Auto-detects algorithm from filename. |
| `pixe resume` | Resume interrupted sort from archive database. |
| `pixe query <sub>` | Read-only DB interrogation: `runs`, `run <id>`, `duplicates`, `errors`, `skipped`, `files`, `inventory`. |
| `pixe status` | Source-oriented, ledger-only report of sorting status. No DB required. |
| `pixe stats` | Archive dashboard: totals, format breakdown, date range. Supports `--json`. |
| `pixe clean` | Remove orphaned `.pixe-tmp` files and XMP sidecars; optionally `VACUUM` the database. |
| `pixe version` | Print version, commit, build date. |

Key flags are defined in each command's source file under `cmd/`. See `cmd/helpers.go` for shared configuration resolution (`resolveConfig()`, `buildRegistry()`).

### 7.2 Configuration File

`.pixe.yaml` supports: `algorithm`, `workers`, `copyright`, `camera_owner`, `recursive`, `skip_duplicates`, `carry_sidecars`, `overwrite_sidecar_tags`, `ignore` (list).

### 7.3 Query Command

`pixe query` opens the DB in read-only mode. Supports `--json` for structured output. All subcommands produce fixed-width columnar tables (default) or JSON. Run IDs support prefix matching. See `cmd/query_*.go` for subcommand implementations.

### 7.4 Status Command

Operates entirely from `dirA` — compares files on disk against `.pixe_ledger.json`. Classifies files as Sorted, Duplicate, Errored, Unsorted, or Unrecognized. No database dependency. Supports `--json`.

### 7.5 Clean Command

Combines orphaned artifact removal (`.pixe-tmp` files, orphaned XMP sidecars) and database compaction (`VACUUM`). Supports `--dry-run`, `--temp-only`, `--vacuum-only` (mutually exclusive). Guards against vacuuming during active sort runs.

---

## 8. Archive Database & Ledger

### 8.1 Overview

SQLite database at `dirB/.pixe/pixe.db` (or `~/.pixe/databases/<slug>.db` for network mounts). Single source of truth for archive state, dedup, run history, and crash recovery.

### 8.2 Database Location

Priority: `--db-path` flag → `dirB/.pixe/dbpath` marker → `dirB/.pixe/pixe.db`. Network mount detection via OS-level `statfs`. A `dbpath` marker file is written when the DB is stored outside `dirB`.

### 8.3 Schema

Two primary tables: `runs` and `files`. See `internal/archivedb/` for the full schema.

- **`runs`:** `id` (UUID), `pixe_version`, `source`, `destination`, `algorithm`, `workers`, `recursive`, `started_at`, `finished_at`, `status` (running/completed/interrupted).
- **`files`:** `run_id` (FK), `source_path`, `dest_path`, `dest_rel`, `checksum`, `algorithm`, `status` (12 valid states), `skip_reason`, `is_duplicate`, `capture_date`, `file_size`, timestamps per stage, `error`, `carried_sidecars` (JSON array).
- **Indexes** on `checksum` (where complete), `run_id`, `status`, `source_path`, `capture_date`.
- **Schema versioning** via `schema_version` table. Migrations are additive (`ALTER TABLE ADD COLUMN`).

### 8.4 Concurrency

WAL mode, busy timeout (5s), per-file transaction commits. Cross-process dedup races handled at application level after commit (loser relocates to `duplicates/`).

### 8.5 Ledger (`dirA/.pixe_ledger.json`)

Streaming JSONL format. Line 1 is a header (run metadata), subsequent lines are per-file entries. Written by the coordinator goroutine. Truncated at start of each run. Partial but valid on interruption.

Current version: `5`. Fields: `path`, `status` (copy/skip/duplicate/error), `checksum`, `destination`, `verified_at`, `sidecars`, `matches`, `reason`.

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

---

## 10. CLI Progress Bars

Opt-in via `--progress` flag on `sort` and `verify`. Lightweight Bubble Tea program rendering an inline progress bar with ETA, status counters, and current file. Auto-disabled when stdout is not a TTY.

When active, `opts.Output` is set to `io.Discard` (progress bar replaces scrolling text). Ledger and database continue recording.

Implementation: `internal/cli/` — `ProgressModel` struct.

---

## 11. Documentation Site (`docs/`)

Jekyll-based static site deployed to GitHub Pages from `docs/`. Uses the **GitHub Pages Slate theme** (`jekyll-theme-slate`) — a stock remote theme with no local overrides.

### 11.1 Content Principles

- **Strict markdown.** All `.md` files in `docs/` are written in standard GitHub-Flavored Markdown. No custom CSS classes, no `<div>` layouts, no inline styles, no `onclick` handlers.
- **HTML is the exception, not the rule.** An occasional HTML tag (e.g., an anchor with `target="_blank"`) is acceptable when markdown has no equivalent. HTML blocks for layout, styling, or interactivity are not.
- **No custom theme assets.** No `_sass/`, `_layouts/`, `_includes/`, or `assets/css/` directories. The Slate theme provides all styling. The site should work with zero local theme overrides.
- **No custom JavaScript.** No `<script>` tags, no accordion toggles, no interactive elements. Content is static markdown rendered by the theme.

### 11.2 Theme Configuration

The site uses the `jekyll-theme-slate` remote theme via the `github-pages` gem. Configuration in `_config.yml`:

- `theme: jekyll-theme-slate` — stock Slate theme, no `remote_theme` needed when using the `github-pages` gem.
- `plugins: [jekyll-remote-theme]` — not required when using `theme:` with a GitHub Pages supported theme.
- No `_layouts/`, `_includes/`, or `_sass/` directories — all provided by the theme.

### 11.3 Navigation

Slate provides a sidebar-style layout. Navigation between pages uses a markdown list or table at the top of `index.md` (the landing page) linking to all other pages. Individual pages link back to the index and to logically adjacent pages via standard markdown links. No `_data/navigation.yml` — navigation is inline markdown.

### 11.4 Pages

| Page | Content |
|---|---|
| `index.md` | Landing page: project description, key guarantees, quick-start commands, navigation links to all other pages |
| `install.md` | Installation methods (go install, build from source) and first-run examples |
| `commands.md` | Full command reference with flag tables (markdown tables) and examples (fenced code blocks) |
| `how-it-works.md` | Pipeline stages, output format, naming convention, duplicate handling, sidecar carry, supported formats |
| `technical.md` | Design rationale: read-only source, copy-then-verify, determinism, no external deps, crash safety |
| `adding-formats.md` | Developer guide for implementing `FileTypeHandler` with code examples |
| `contributing.md` | Contribution workflow: issue-first, clone/build, test, conventions, PR |
| `changelog.md` | Version history |
| `packages.md` | Generated package reference (docgen output) |
| `ai.md` | AI collaboration transparency statement |

### 11.5 Content Migration

The migration from the custom theme to Slate has been completed:

1. **Deleted** all custom theme directories: `_sass/`, `_layouts/`, `_includes/`, `assets/`, `_data/`, `_site/`, `.jekyll-cache/`.
2. **Updated** `_config.yml` to use `theme: jekyll-theme-slate` and removed `sass:` configuration.
3. **Updated** `Gemfile` to use `github-pages` gem (which bundles `jekyll-theme-slate`).
4. **Rewrote** every `.md` file to strict markdown:
   - Replaced all HTML `<table>` blocks with markdown tables.
   - Replaced all `<div class="...">` layout blocks with markdown equivalents (headings, lists, blockquotes).
   - Replaced all `<pre>` terminal-styled blocks with fenced code blocks.
   - Replaced `<span class="term-...">` styled output with plain text in code blocks.
   - Removed all `layout:` and `section_label:` front matter keys (Slate uses `default` layout automatically). Kept `title:` only.
   - Converted the `index.md` landing page from full-HTML sections to a markdown document with headings, paragraphs, and code blocks.
   - Converted the `commands.md` accordion pattern to flat markdown sections (one `###` per command, markdown flag tables, fenced code examples).
   - Converted `contributing.md` numbered steps from styled `<div>` blocks to a markdown ordered list.
   - Converted `ai.md` from a styled `<div class="ai-card">` to plain markdown paragraphs.
5. **Removed** `Gemfile.lock` and regenerated after Gemfile update.

### 11.6 Files to Delete

The following files and directories are artifacts of the custom theme and must be removed:

- `docs/_sass/` (entire directory — 11 SCSS partials)
- `docs/_layouts/` (entire directory — `default.html`, `landing.html`, `page.html`)
- `docs/_includes/` (entire directory — `head.html`, `nav.html`, `footer.html`, `hero.html`, `pipeline.html`, `format-grid.html`)
- `docs/assets/` (entire directory — `css/main.scss`)
- `docs/_data/` (entire directory — `navigation.yml`)
- `docs/_site/` (build output, should be gitignored)
- `docs/.jekyll-cache/` (build cache, should be gitignored)

---

## 12. Documentation Generation (`docgen`)

`internal/docgen/` — a development-time Go tool (`go run ./internal/docgen`) that extracts code-sourced facts from the Go AST and injects them into documentation files via marker-based replacement.

- **Marker syntax:** `<!-- pixe:begin:section-name -->` / `<!-- pixe:end:section-name -->` — content between markers is replaced. The markers themselves are HTML comments, which is an acceptable use of HTML in the markdown files (they are invisible in rendered output).
- **Output format:** All generated content is **markdown** — markdown tables, markdown lists, fenced code blocks. No HTML tables or HTML layout in generated output.
- **Extraction targets:** Version string (git tag), `FileTypeHandler` interface, CLI flags (Cobra registrations), supported format table, package reference (godoc comments), query subcommands.
- **Page classification:** Hand-authored (no markers), Hybrid (narrative + injected facts), Generated (`packages.md`).
- **Makefile:** `make docs` (regenerate), `make docs-check` (CI staleness gate).
- **Docgen output:** The `docgen` tool emits markdown tables for all documentation targets. All generated content is markdown-only — no HTML tables or HTML layout.

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
