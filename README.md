# Pixe

A safe, deterministic photo and video sorting utility.

## What It Does

Pixe organizes media files into a date-based directory structure with embedded integrity checksums. It extracts capture dates from file metadata, computes deterministic hashes, and produces a consistently organized archive — without ever modifying your source files.

## Key Principles

- **Safety above all else** — Source files are never modified or moved. Every copy is verified before being considered complete. Interrupted runs can always be resumed.
- **Native Go execution** — All functionality uses pure Go packages. No external dependencies like `exiftool` or `ffmpeg`.
- **Deterministic output** — Given the same inputs and configuration, Pixe always produces the same directory structure and filenames.
- **Modular design** — New file types are added by implementing a Go interface. The core engine is format-agnostic.

## How It Works

### Pipeline Stages

Each file passes through these stages:

```
discover → extract → hash → copy → verify → tag → complete
```

1. **Discover** — Walk the source directory, classify files by type
2. **Extract** — Read capture date from file metadata
3. **Hash** — Compute checksum over the media payload (excluding metadata)
4. **Copy** — Write to the destination path
5. **Verify** — Re-read and re-hash the destination to confirm integrity
6. **Tag** — Optionally inject Copyright/CameraOwner metadata

### Output Naming Convention

```
YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
```

- **Date/Time**: Extracted from EXIF `DateTimeOriginal` or `CreateDate`
- **Checksum**: Hex-encoded hash (default SHA-1, 40 characters)
- **Extension**: Lowercase, preserved from original

### Directory Structure

```
<dest>/<YYYY>/<M>/<filename>
```

- Year: 4-digit (e.g., `2021`)
- Month: Non-zero-padded (e.g., `2`, not `02`)

### Duplicates

When a file's checksum matches an already-processed file:

```
<dest>/duplicates/<run_timestamp>/<YYYY>/<M>/<filename>
```

### Manifest & Ledger

- **Manifest** (`<dest>/.pixe/manifest.json`): Tracks per-file processing state, enables resume after interruption
- **Ledger** (`<source>/.pixe_ledger.json`): Records successfully processed files in the source directory

## Installation

### Install Latest Release

```bash
go install github.com/cwlls/pixe-go@latest
```

### Build from Source

```bash
git clone https://github.com/cwlls/pixe-go.git
cd pixe-go
go build -o pixe .
```

## Usage

### `pixe sort`

Sort media files from a source directory into an organized archive:

```bash
pixe sort --source /path/to/photos --dest /path/to/archive [options]
```

| Flag | Description |
|------|-------------|
| `-s, --source` | Source directory containing media files (required) |
| `-d, --dest` | Destination directory for the organized archive (required) |
| `-w, --workers` | Number of concurrent workers (default: auto-detect) |
| `-a, --algorithm` | Hash algorithm: `sha1`, `sha256` (default: `sha1`) |
| `--copyright` | Copyright template, e.g. `"Copyright {{.Year}} My Family"` |
| `--camera-owner` | Camera owner string to inject |
| `--dry-run` | Preview operations without copying |

### `pixe verify`

Verify the integrity of a previously sorted archive:

```bash
pixe verify --dir /path/to/archive [options]
```

| Flag | Description |
|------|-------------|
| `-d, --dir` | Archive directory to verify (required) |
| `-w, --workers` | Number of concurrent workers (default: auto-detect) |
| `-a, --algorithm` | Hash algorithm (must match what was used during sort) |

Exit code `0` = all verified. Exit code `1` = one or more mismatches.

### `pixe resume`

Resume an interrupted sort operation:

```bash
pixe resume --dir /path/to/archive
```

| Flag | Description |
|------|-------------|
| `-d, --dir` | Destination directory containing `.pixe/manifest.json` (required) |

Files marked `complete` are skipped. Files in `copied` state are re-verified. Earlier states re-enter the pipeline.

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

| Format | Extensions | Status |
|--------|------------|--------|
| JPEG   | `.jpg`, `.jpeg` | Complete |
| HEIC   | `.heic`, `.heif` | In Progress |
| MP4    | `.mp4`, `.mov` | In Progress |

### Date Fallback Chain

Each file type extracts dates in this order:

1. EXIF `DateTimeOriginal`
2. EXIF `CreateDate`
3. Default: **February 20, 1902** (Ansel Adams' birthday) — files with unknown dates are identifiable by their `19020220` prefix

## Safety Guarantees

- **Source is read-only** — Pixe never modifies, renames, moves, or deletes source files. Only `.pixe_ledger.json` is written to the source directory.
- **Copy-then-verify** — Every file is copied to the destination, then independently re-hashed to confirm integrity.
- **Manifest-based resumability** — A manifest tracks each file's state. Interrupted runs resume from the last known-good state.
- **No silent data loss** — Hash mismatches, copy failures, and unrecognized files are always reported. Pixe never exits silently on error.

## Project Status

This project is under active development. Implementation progress is tracked in [STATE.md](.state/STATE.md).

Current focus: Core pipeline implementation (discovery, hashing, manifest persistence).

---

Documentation: https://github.com/cwlls/pixe-go
