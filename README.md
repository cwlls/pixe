# Pixe

A safe, deterministic photo and video sorting utility.

## What It Does

Pixe organizes media files into a date-based directory structure with embedded integrity checksums. It extracts capture dates from file metadata, computes deterministic hashes, and produces a consistently organized archive — without ever modifying your source files. A SQLite archive database tracks every file ever processed, enabling duplicate detection, skip-on-resume, and queryable history across all runs.

## Key Principles

- **Safety above all else** — Source files are never modified or moved. Every copy is verified before being considered complete. Interrupted runs can always be resumed.
- **Native Go execution** — All functionality uses pure Go packages. No external dependencies like `exiftool` or `ffmpeg`.
- **Deterministic output** — Given the same inputs and configuration, Pixe always produces the same directory structure and filenames.
- **Modular design** — New file types are added by implementing a Go interface. The core engine is format-agnostic.

## How It Works

### Pipeline Stages

Each file passes through these stages:

```
discover → extract → hash → copy → verify → carry sidecars → tag → complete
```

1. **Discover** — Walk the source directory, classify files by type
2. **Extract** — Read capture date from file metadata
3. **Hash** — Compute checksum over the media payload (excluding metadata)
4. **Copy** — Write to the destination path
5. **Verify** — Re-read and re-hash the destination to confirm integrity
6. **Carry sidecars** — Detect and carry pre-existing `.aae` and `.xmp` sidecar files from source to destination
7. **Tag** — Optionally inject Copyright/CameraOwner metadata

### Output Format

Each file produces one output line:

```
COPY IMG_0001.jpg -> 2021/12-Dec/20211225_062223_abc123.jpg
     +sidecar IMG_0001.aae -> 2021/12-Dec/20211225_062223_abc123.jpg.aae
SKIP notes.txt -> unsupported format: .txt
SKIP IMG_0002.jpg -> previously imported
DUPE IMG_0003.jpg -> matches 2021/12-Dec/20211225_062223_abc123.jpg
ERR  corrupt.jpg -> extract date: no EXIF data
Done. processed=3 duplicates=1 skipped=2 errors=1
```

### Output Naming Convention

```
YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
```

- **Date/Time**: Extracted from EXIF `DateTimeOriginal` or `CreateDate`
- **Checksum**: Hex-encoded hash (default SHA-1, 40 characters)
- **Extension**: Lowercase, preserved from original

### Directory Structure

```
<dest>/<YYYY>/<MM-Mon>/<filename>
```

- Year: 4-digit (e.g., `2021`)
- Month: Zero-padded number + 3-letter abbreviation (e.g., `12-Dec`)

### Duplicates

When a file's checksum matches an already-processed file:

```
<dest>/duplicates/<run_timestamp>/<YYYY>/<MM-Mon>/<filename>
```

Use the `--skip-duplicates` flag to skip copying duplicate files entirely instead of copying them to the `duplicates/` directory. When active, duplicates are detected and checksummed but not physically copied to the destination.

### Archive Database & Ledger

- **Archive DB** (`<dest>/.pixe/<slug>.db`): SQLite database tracking all files ever processed across all runs. Enables duplicate detection, skip-on-resume, and queryable history. Automatically migrated from legacy JSON manifest if present.
- **Ledger** (`<source>/.pixe_ledger.json`): Streaming JSONL receipt written during each run. Line 1 is a header with run metadata; subsequent lines are one entry per file. Human-readable and crash-safe (partial writes are valid JSONL).

## Installation

### Install Latest Release

```bash
go install github.com/cwlls/pixe-go@latest
```

### Build from Source

```bash
git clone https://github.com/cwlls/pixe-go.git
cd pixe-go
make build
```

## Usage

### `pixe sort`

Sort media files from a source directory into an organized archive:

```bash
# source defaults to current directory
pixe sort --dest /path/to/archive [options]

# or with an explicit source
pixe sort --source /path/to/photos --dest /path/to/archive [options]
```

<!-- pixe:begin:sort-flags -->
| Flag                     | Default | Description                                                                                         |
| ------------------------ | ------- | --------------------------------------------------------------------------------------------------- |
| --config                 |         | config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)                                             |
| -w, --workers            | 0       | number of concurrent workers (0 = auto: runtime.NumCPU())                                           |
| -a, --algorithm          | sha1    | hash algorithm: md5, sha1 (default), sha256, blake3, xxhash                                         |
| -q, --quiet              | false   | suppress per-file output; show only the final summary                                               |
| -v, --verbose            | false   | show per-stage timing and debug information                                                         |
| --profile                |         | load a named config profile from ~/.pixe/profiles/<name>.yaml                                       |
| -s, --source             |         | source directory containing media files to sort (default: current directory)                        |
| -d, --dest               |         | destination directory for the organized archive (required)                                          |
| --copyright              |         | copyright template injected into destination files, e.g. "Copyright {{.Year}} My Family"            |
| --camera-owner           |         | camera owner string injected into destination files                                                 |
| --dry-run                | false   | preview operations without copying any files                                                        |
| --db-path                |         | explicit path to the SQLite archive database (overrides auto-resolution)                            |
| -r, --recursive          | false   | recursively process subdirectories of --source                                                      |
| --skip-duplicates        | false   | skip copying duplicate files instead of copying to duplicates/ directory                            |
| --ignore                 |         | glob pattern for files to ignore (repeatable, e.g. --ignore "*.txt" --ignore ".DS_Store")           |
| --no-carry-sidecars      | false   | disable carrying pre-existing .aae and .xmp sidecar files from source to destination                |
| --overwrite-sidecar-tags | false   | when merging tags into a carried .xmp sidecar, overwrite existing values instead of preserving them |
| --progress               | false   | show a live progress bar instead of per-file text output (requires a TTY)                           |
| --since                  |         | only process files with capture date on or after this date (format: YYYY-MM-DD)                     |
| --before                 |         | only process files with capture date on or before this date (format: YYYY-MM-DD)                    |
<!-- pixe:end:sort-flags -->

#### Ignore Patterns

The `--ignore` flag supports three pattern types:

1. **Simple glob patterns** — `--ignore "*.txt"` excludes `.txt` files in the source directory.

2. **Recursive globs** — `--ignore "**/*.txt"` excludes `.txt` files at any depth. Use `**` to match across directory boundaries.

3. **Directory patterns** — Patterns ending with `/` skip entire directories without descending:
   ```bash
   pixe sort --source /path/to/photos --dest /archive --ignore "node_modules/" --ignore "cache/"
   ```
   Patterns ending with `/**` also trigger directory skipping (e.g., `--ignore "temp/**"`).

4. **`.pixeignore` files** — Place a `.pixeignore` file in the source directory (or any subdirectory) to define patterns scoped to that location and its descendants:
   ```
   # .pixeignore in /path/to/photos
   *.tmp
   **/*.bak
   cache/
   .DS_Store
   ```
   Format: one pattern per line, `#` comments, blank lines ignored. Negation (`!`) is not supported. The `.pixeignore` file itself is always invisible to the pipeline.

**Examples:**
```bash
# Exclude all .txt files at any depth
pixe sort --source /photos --dest /archive --ignore "**/*.txt"

# Exclude multiple patterns
pixe sort --source /photos --dest /archive --ignore "*.tmp" --ignore "cache/" --ignore "**/*.bak"

# Use .pixeignore file (no CLI flags needed)
# Create /photos/.pixeignore with patterns, then:
pixe sort --source /photos --dest /archive
```

### `pixe verify`

Verify the integrity of a previously sorted archive:

```bash
pixe verify --dir /path/to/archive [options]
```

<!-- pixe:begin:verify-flags -->
| Flag            | Default | Description                                                               |
| --------------- | ------- | ------------------------------------------------------------------------- |
| --config        |         | config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)                   |
| -w, --workers   | 0       | number of concurrent workers (0 = auto: runtime.NumCPU())                 |
| -a, --algorithm | sha1    | hash algorithm: md5, sha1 (default), sha256, blake3, xxhash               |
| -q, --quiet     | false   | suppress per-file output; show only the final summary                     |
| -v, --verbose   | false   | show per-stage timing and debug information                               |
| --profile       |         | load a named config profile from ~/.pixe/profiles/<name>.yaml             |
| -d, --dir       |         | archive directory to verify (required)                                    |
| --progress      | false   | show a live progress bar instead of per-file text output (requires a TTY) |
<!-- pixe:end:verify-flags -->

Exit code `0` = all verified. Exit code `1` = one or more mismatches.

### `pixe resume`

Resume an interrupted sort operation:

```bash
pixe resume --dir /path/to/archive
```

<!-- pixe:begin:resume-flags -->
| Flag            | Default | Description                                                              |
| --------------- | ------- | ------------------------------------------------------------------------ |
| --config        |         | config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)                  |
| -w, --workers   | 0       | number of concurrent workers (0 = auto: runtime.NumCPU())                |
| -a, --algorithm | sha1    | hash algorithm: md5, sha1 (default), sha256, blake3, xxhash              |
| -q, --quiet     | false   | suppress per-file output; show only the final summary                    |
| -v, --verbose   | false   | show per-stage timing and debug information                              |
| --profile       |         | load a named config profile from ~/.pixe/profiles/<name>.yaml            |
| -d, --dir       |         | destination directory containing the archive database (required)         |
| --db-path       |         | explicit path to the SQLite archive database (overrides auto-resolution) |
<!-- pixe:end:resume-flags -->

Finds the most recent interrupted run in the archive database and re-sorts from the source directory. Files already marked complete are skipped automatically.

### `pixe query`

Query the archive database without modifying any files:

```bash
pixe query <subcommand> --dir /path/to/archive [--json]
```

<!-- pixe:begin:query-subs -->
| Subcommand | Description                                           |
| ---------- | ----------------------------------------------------- |
| runs       | List all sort runs recorded in the archive database   |
| run        | Show details for a specific sort run                  |
| duplicates | List all duplicate files in the archive               |
| errors     | List all files that encountered errors during sorting |
| skipped    | List all files that were skipped during sorting       |
| files      | Search for files in the archive by date or source     |
| inventory  | List all canonical files in the archive               |
<!-- pixe:end:query-subs -->

All subcommands accept `--json` for machine-readable output.

**Persistent flags** (inherited by all subcommands):

<!-- pixe:begin:query-flags -->
| Flag      | Default | Description                                                              |
| --------- | ------- | ------------------------------------------------------------------------ |
| -d, --dir |         | archive directory containing the database (required)                     |
| --db-path |         | explicit path to the SQLite archive database (overrides auto-resolution) |
| --json    | false   | emit JSON output instead of a table                                      |
<!-- pixe:end:query-flags -->

### `pixe status`

Report the sorting status of a source directory by comparing files on disk against the `.pixe_ledger.json` left by prior `pixe sort` runs. No archive database or destination directory is required — it works entirely from the source directory.

```bash
# source defaults to current directory
pixe status [options]

# or with an explicit source
pixe status --source /path/to/photos [options]
```

<!-- pixe:begin:status-flags -->
| Flag            | Default | Description                                                          |
| --------------- | ------- | -------------------------------------------------------------------- |
| --config        |         | config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)              |
| -w, --workers   | 0       | number of concurrent workers (0 = auto: runtime.NumCPU())            |
| -a, --algorithm | sha1    | hash algorithm: md5, sha1 (default), sha256, blake3, xxhash          |
| -q, --quiet     | false   | suppress per-file output; show only the final summary                |
| -v, --verbose   | false   | show per-stage timing and debug information                          |
| --profile       |         | load a named config profile from ~/.pixe/profiles/<name>.yaml        |
| -s, --source    |         | source directory to inspect (default: current directory)             |
| -r, --recursive | false   | recursively inspect subdirectories of --source                       |
| --ignore        |         | glob pattern for files to ignore (repeatable, e.g. --ignore "*.txt") |
| --json          | false   | emit JSON output instead of a human-readable listing                 |
<!-- pixe:end:status-flags -->

**How it works:**

1. Walks the source directory using the same handler registry as `pixe sort`
2. Loads the `.pixe_ledger.json` ledger file from the source directory
3. Classifies every file into one of five categories:
   - **SORTED** — ledger entry with `status: "copy"`; shows destination path
   - **DUPLICATE** — ledger entry with `status: "duplicate"`; shows the original it matched
   - **ERRORED** — ledger entry with `status: "error"`; shows the error reason
   - **UNSORTED** — no ledger entry (or `status: "skip"`); needs sorting
   - **UNRECOGNIZED** — no handler claims this file type (e.g. `.txt`)
4. Outputs a sectioned listing with a summary line

**Example output (human-readable):**

```
Source: /Users/wells/photos
Ledger: run a1b2c3d4, 2026-03-06T10:30:00Z (recursive: no)

SORTED (247 files)
  IMG_0001.jpg  → 2021/12-Dec/20211225_062223_7d97e98f...jpg
  IMG_0002.jpg  → 2022/02-Feb/20220202_123101_447d3060...jpg

DUPLICATE (3 files)
  IMG_0042.jpg  → matches 2022/02-Feb/20220202_123101_447d3060...jpg

ERRORED (1 file)
  corrupt.jpg   → EXIF parse failed: truncated IFD at offset 0x1A

UNSORTED (12 files)
  IMG_5001.jpg
  vacation/IMG_6001.jpg

UNRECOGNIZED (2 files)
  notes.txt     → unsupported format: .txt

265 total | 247 sorted | 3 duplicates | 1 errored | 12 unsorted | 2 unrecognized
```

Exit code `0` always on success (unsorted files are not an error condition).

### `pixe clean`

Perform maintenance on an archive: remove orphaned temp files, clean up orphaned XMP sidecars, and optionally compact the database.

```bash
pixe clean --dir /path/to/archive [options]
```

<!-- pixe:begin:clean-flags -->
| Flag          | Default | Description                                              |
| ------------- | ------- | -------------------------------------------------------- |
| -d, --dir     |         | destination directory (dirB) to clean (required)         |
| --db-path     |         | explicit path to the SQLite archive database             |
| --dry-run     | false   | preview what would be cleaned without modifying anything |
| --temp-only   | false   | only clean orphaned files, skip database compaction      |
| --vacuum-only | false   | only compact the database, skip file scanning            |
<!-- pixe:end:clean-flags -->

**What it cleans:**

- **Orphaned temp files** — `.pixe-tmp` files left behind by interrupted sort runs
- **Orphaned XMP sidecars** — Pixe-generated `.xmp` files whose corresponding media file no longer exists (regex-gated to avoid removing user-created XMP files)
- **Database compaction** — Runs `VACUUM` to reclaim space from long-lived archives (skipped if a sort is currently in progress)

## Configuration File

Pixe reads configuration from `.pixe.yaml` in the current directory, home directory, or `$XDG_CONFIG_HOME/pixe`. Configuration is merged with CLI flags — CLI flags take precedence.

Example `.pixe.yaml`:

```yaml
algorithm: sha1
workers: 8
copyright: "Copyright {{.Year}} My Family, all rights reserved"
camera_owner: "Wells Family"
```

Environment variables prefixed with `PIXE_` also override config file values (e.g., `PIXE_WORKERS=4`).

## Supported File Types

<!-- pixe:begin:format-table -->
| Format  | Extensions   | Metadata      |
| ------- | ------------ | ------------- |
| ARW     | .arw         | XMP sidecar   |
| AVIF    | .avif        | XMP sidecar   |
| CR2     | .cr2         | XMP sidecar   |
| CR3     | .cr3         | XMP sidecar   |
| DNG     | .dng         | XMP sidecar   |
| HEIC    | .heic, .heif | XMP sidecar   |
| JPEG    | .jpg, .jpeg  | Embedded EXIF |
| MP4/MOV | .mp4, .mov   | XMP sidecar   |
| NEF     | .nef         | XMP sidecar   |
| ORF     | .orf         | XMP sidecar   |
| PEF     | .pef         | XMP sidecar   |
| PNG     | .png         | XMP sidecar   |
| RW2     | .rw2         | XMP sidecar   |
| TIFF    | .tif, .tiff  | XMP sidecar   |
<!-- pixe:end:format-table -->

### Date Fallback Chain

Each file type extracts dates in this order:

1. EXIF `DateTimeOriginal`
2. EXIF `CreateDate`
3. Default: **February 20, 1902** (Ansel Adams' birthday) — files with unknown dates are identifiable by their `19020220` prefix

### RAW Hashing Strategy

RAW files (DNG, NEF, CR2, PEF, ARW, CR3) are hashed on their raw sensor data, not the embedded JPEG preview. JPEG previews can be regenerated by software tools like Lightroom, making them an unreliable deduplication key; sensor data is immutable and represents the true capture. If sensor data cannot be extracted, the full file is hashed as a fallback. Hashing sensor data reads more bytes than a JPEG preview (20–80 MB vs 1–5 MB per file), but accuracy is prioritized over speed.

## Safety Guarantees

- **Source is read-only** — Pixe never modifies, renames, moves, or deletes source files. Only `.pixe_ledger.json` is written to the source directory.
- **Atomic copy-then-verify** — Files are first written to a uniquely-named temp file (`.<name>.pixe-tmp-*`) in the destination directory, then independently re-hashed. Only after verification passes is the temp file atomically renamed to its canonical path. A file at its canonical location is always complete and verified — partial files never appear at canonical paths.
- **Database-backed resumability** — The SQLite archive database tracks each file's state across all runs. Interrupted runs resume without re-processing completed files.
- **Crash-safe ledger** — The JSONL ledger is written one line at a time. An interrupted run leaves a partial but fully valid receipt.
- **No silent data loss** — Hash mismatches, copy failures, and unrecognized files are always reported. Pixe never exits silently on error.

## Project Status

This project is under active development. Implementation progress is tracked in [STATE.md](.state/STATE.md).

---

Documentation: https://github.com/cwlls/pixe-go
