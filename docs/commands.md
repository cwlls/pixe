---
title: Commands
---

# Commands

## pixe sort

Organize media into a date-based archive.

Primary operation. Discovers files in the source directory, processes them through the pipeline, and writes organized output to the destination. When `--source` is omitted, the current working directory is used.

```bash
$ pixe sort --dest /path/to/archive [options]
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

### Examples

```bash
# Sort from current directory
$ pixe sort --dest ~/Archive

# Recursive sort with explicit source
$ pixe sort --source ~/Photos --dest ~/Archive --recursive

# Dry run to preview without copying
$ pixe sort --dest ~/Archive --dry-run

# With copyright tagging and duplicate skipping
$ pixe sort --dest ~/Archive --copyright "Copyright {{.Year}} My Family" --skip-duplicates

# Ignore OS junk files
$ pixe sort --dest ~/Archive --ignore ".DS_Store" --ignore "Thumbs.db"

# Sort without carrying sidecar files
$ pixe sort --dest ~/Archive --no-carry-sidecars
```

---

## pixe status

Report sort status of a source directory.

Read-only. Compares files on disk against the `.pixe_ledger.json` written by prior sort runs. No archive database or destination directory required — works entirely from the source directory. When `--source` is omitted, the current working directory is inspected.

```bash
$ pixe status [options]
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

Categories: `SORTED` · `DUPLICATE` · `ERRORED` · `UNSORTED` · `UNRECOGNIZED`

---

## pixe verify

Re-hash every file in the archive to confirm integrity.

Walks a previously sorted archive, parses checksums from filenames, recomputes data-only hashes, and reports mismatches. Use this to confirm your archive is intact after a disk migration or NAS transfer.

```bash
$ pixe verify --dir /path/to/archive [options]
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

---

## pixe resume

Resume an interrupted sort operation.

Finds the most recent interrupted run in the archive database and re-sorts from the original source directory. Files already marked complete are skipped automatically.

```bash
$ pixe resume --dir /path/to/archive
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

---

## pixe query

Read-only queries against the archive database.

Read-only interrogation of the archive SQLite database. No files are modified. All subcommands accept `--json` for machine-readable output.

```bash
$ pixe query <subcommand> --dir /path/to/archive [--json]
```

<!-- pixe:begin:query-flags -->
| Flag      | Default | Description                                                              |
| --------- | ------- | ------------------------------------------------------------------------ |
| -d, --dir |         | archive directory containing the database (required)                     |
| --db-path |         | explicit path to the SQLite archive database (overrides auto-resolution) |
| --json    | false   | emit JSON output instead of a table                                      |
<!-- pixe:end:query-flags -->

### Subcommands

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

### Examples

```bash
$ pixe query runs --dir ~/Archive
$ pixe query run a1b2c3d4 --dir ~/Archive
$ pixe query duplicates --dir ~/Archive --pairs
$ pixe query files --dir ~/Archive --from 2024-01-01 --to 2024-12-31
$ pixe query inventory --dir ~/Archive --json | jq '.results | length'
```

---

## pixe clean

Remove orphaned temp files and compact the archive database.

Maintenance command for a destination archive. Removes `.pixe-tmp` files left by interrupted runs, removes orphaned XMP sidecars, and optionally runs `VACUUM` on the SQLite database to reclaim space.

```bash
$ pixe clean --dir /path/to/archive [options]
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

---

## pixe version

Print version, commit, and build date.

```bash
$ pixe version
pixe v2.0.0 (commit: abc1234, built: 2026-03-11T10:30:00Z)
```

No flags. Prints the version string and exits.

---

## Configuration file

Pixe reads `.pixe.yaml` from the current directory, home directory, or `$XDG_CONFIG_HOME/pixe`. CLI flags take precedence over config file values. Environment variables prefixed `PIXE_` also override config values (e.g., `PIXE_ALGORITHM=sha256`).

```yaml
algorithm: sha1
workers: 8
recursive: false
skip_duplicates: false
copyright: "Copyright {{.Year}} My Family, all rights reserved"
camera_owner: "Wells Family"
ignore:
  - "*.txt"
  - ".DS_Store"
  - "Thumbs.db"
  - "*.aae"
  - "node_modules/"
```
