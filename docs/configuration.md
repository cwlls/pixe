---
title: Configuration
nav_order: 4
---

# Configuration

Pixe is configured through a layered system of CLI flags, environment variables, config files, and profiles. Understanding the resolution order tells you exactly which value wins when the same setting appears in multiple places.

---

## Precedence order

Settings are resolved from highest to lowest priority. The first source that provides a value wins — lower-priority sources are ignored for that key.

| Priority | Source | Example |
|:---------|:-------|:--------|
| 1 (highest) | CLI flags | `--algorithm sha256` |
| 2 | Environment variables | `PIXE_ALGORITHM=sha256` |
| 3 | Source-local config | `<source-dir>/.pixe.yaml` |
| 4 | Named profile | `~/.pixe/profiles/<name>.yaml` (via `--profile`) |
| 5 | Global config file | `~/.pixe.yaml` or `$XDG_CONFIG_HOME/pixe/.pixe.yaml` |
| 6 (lowest) | Built-in defaults | Hardcoded in source |

**CLI flags always win.** If you pass `--algorithm sha256` on the command line, no config file or environment variable can override it for that run.

**Environment variables rank above config files.** A `PIXE_ALGORITHM=sha256` export in your shell overrides whatever `algorithm:` is set to in `.pixe.yaml`. This is Viper's standard resolution order.

**Source-local config is auto-detected.** When you run `pixe sort --source /path/to/photos`, Pixe looks for `/path/to/photos/.pixe.yaml` and merges it automatically — no flag required. Only the 9 sort-relevant keys are merged from source-local config (see [Source-local config](#source-local-config)).

**Profiles are explicitly selected.** Pass `--profile <name>` to load `~/.pixe/profiles/<name>.yaml`. Profiles use the same merge rules as source-local config.

---

## Global config file

Pixe searches for `.pixe.yaml` in the following locations, in order. The first file found is used.

1. `./.pixe.yaml` — current working directory
2. `$HOME/.pixe.yaml` — your home directory
3. `$XDG_CONFIG_HOME/pixe/.pixe.yaml` — XDG config directory (Linux/macOS)

The file is optional. If none is found, Pixe runs with CLI flags and defaults only.

Use `--config /path/to/file.yaml` to specify an explicit config file path, bypassing the search.

---

## Settings reference

### Config file keys

These keys can be set in `.pixe.yaml` (global, source-local, or profile):

| Config key | CLI flag | Type | Default | Description |
|:-----------|:---------|:-----|:--------|:------------|
| `dest` | `-d, --dest` | string | (required) | Destination archive directory. Supports `@alias` syntax — see [Destination aliases](#destination-aliases). |
| `algorithm` | `-a, --algorithm` | string | `sha1` | Hash algorithm for checksums and filenames. Options: `md5`, `sha1`, `sha256`, `blake3`, `xxhash`. |
| `workers` | `-w, --workers` | int | CPU count | Number of concurrent pipeline workers. `0` means auto-detect (`runtime.NumCPU()`). |
| `recursive` | `-r, --recursive` | bool | `false` | Descend into subdirectories of the source directory. |
| `skip_duplicates` | `--skip-duplicates` | bool | `false` | Skip copying duplicate files. Duplicates are recorded in the database but no bytes are written to `duplicates/`. |
| `copyright` | `--copyright` | string | (disabled) | Copyright metadata template. Uses `{token}` syntax: `{year}`, `{month}`, `{monthname}`, `{day}`. Example: `"Copyright {year} The Wells Family"`. |
| `camera_owner` | `--camera-owner` | string | (disabled) | Camera owner string written to XMP sidecar metadata. |
| `path_template` | `--path-template` | string | `{year}/{month}-{monthname}` | Token-based template for the destination directory structure. Available tokens: `{year}`, `{month}`, `{monthname}`, `{day}`, `{hour}`, `{minute}`, `{second}`, `{ext}`. The filename itself is not configurable. |
| `no_carry_sidecars` | `--no-carry-sidecars` | bool | `false` | Disable carrying pre-existing `.aae` and `.xmp` sidecar files from source to destination. Sidecars are carried by default. |
| `overwrite_sidecar_tags` | `--overwrite-sidecar-tags` | bool | `false` | When merging tags into a carried `.xmp` sidecar, overwrite existing values. Default preserves existing values (source is authoritative). |
| `db_path` | `--db-path` | string | (auto-resolved) | Explicit path to the SQLite archive database. Overrides the default location (`<dest>/.pixe/pixe.db`) and network-mount detection. |
| `dry_run` | `--dry-run` | bool | `false` | Preview mode. Extracts and hashes files but skips all copy, verify, and tag operations. |
| `ignore` | `--ignore` | list | (none) | Glob patterns for files to exclude from discovery. Repeatable on the CLI (`--ignore "*.txt" --ignore ".DS_Store"`). Merged additively across config sources. |
| `since` | `--since` | string | (none) | Only process files with a capture date on or after this date. Format: `YYYY-MM-DD`. |
| `before` | `--before` | string | (none) | Only process files with a capture date on or before this date. Format: `YYYY-MM-DD`. |
| `aliases` | (none) | map | (none) | Destination aliases. Maps short names to filesystem paths. See [Destination aliases](#destination-aliases). |

### CLI-only flags

These flags are not read from config files. They can only be set on the command line or via environment variables.

| CLI flag | Type | Default | Description |
|:---------|:-----|:--------|:------------|
| `--config` | string | (auto-discovered) | Explicit config file path. Bypasses the standard search order. |
| `--profile` | string | (none) | Named profile to load from `~/.pixe/profiles/<name>.yaml`. |
| `-q, --quiet` | bool | `false` | Suppress per-file output. Show only the final summary line. Mutually exclusive with `--verbose`. |
| `-v, --verbose` | bool | `false` | Enable verbose output with per-stage timing. Mutually exclusive with `--quiet`. |
| `--progress` | bool | `false` | Show a live progress bar with per-worker status instead of scrolling text. Requires a TTY. |
| `-y, --yes` | bool | `false` | Auto-accept interactive prompts. Currently used when the ledger cannot be created in the source directory — continues without a ledger instead of prompting. Useful for scripting. |
| `--no-ledger` | bool | `false` | Explicitly skip ledger creation without prompting or printing a warning. Use this in scripts that intentionally run without a source-side ledger. |

---

## Source-local config

When you run `pixe sort`, Pixe automatically looks for a `.pixe.yaml` file in the `--source` directory. If found, it is merged into the active configuration before the sort begins.

This is useful for camera-specific or project-specific settings that should always apply to a particular source directory — without having to pass flags every time.

**Which keys are merged from source-local config:**

`dest`, `copyright`, `camera_owner`, `algorithm`, `recursive`, `skip_duplicates`, `no_carry_sidecars`, `overwrite_sidecar_tags`, `path_template`

**Additive merge for `ignore` and `aliases`:** Patterns and aliases from the source-local config are appended to (not replaced by) the global config values. This means you can define global ignores in `~/.pixe.yaml` and add source-specific ignores in the source directory's `.pixe.yaml` — both sets apply.

**CLI flags always win.** If a key was explicitly set via a CLI flag, the source-local value is ignored for that key.

Example — a source directory with its own config:

```yaml
# /Volumes/CameraSD/.pixe.yaml
dest: @nas
path_template: "{year}/{month}-{monthname}/{day}"
ignore:
  - "MISC/"
  - "*.thm"
```

Running `pixe sort --source /Volumes/CameraSD` will automatically pick this up.

---

## Named profiles

Profiles let you define named sets of settings for different workflows — different cameras, different archives, different algorithms.

**Loading a profile:**

```bash
pixe sort --dest ~/Archive --profile family-photos
```

**Profile file location** (searched in order):

1. `~/.pixe/profiles/<name>.yaml`
2. `$XDG_CONFIG_HOME/pixe/profiles/<name>.yaml`

**Merge rules:** Same as source-local config. Only keys not already set via CLI flags are merged. `ignore` and `aliases` are merged additively.

**Priority:** CLI flags > source-local config > profile > global config.

Example profile at `~/.pixe/profiles/raw-archive.yaml`:

```yaml
algorithm: sha256
path_template: "{year}/{month}-{monthname}/{day}"
copyright: "Copyright {year} Chris Wells"
camera_owner: "Chris Wells"
```

---

## Destination aliases

Aliases let you use short names instead of long filesystem paths for `--dest`. This is especially useful for network mounts or paths that differ between machines.

**Define aliases** in `~/.pixe.yaml` (or a source-local `.pixe.yaml`):

```yaml
aliases:
  nas: /Volumes/NAS/Photos
  backup: /Volumes/Backup/Archive
  local: ~/Pictures/Sorted
```

**Use an alias** with the `@` prefix:

```bash
pixe sort --dest @nas
pixe sort --dest @backup --recursive
```

**Tilde expansion** is applied to alias values — `~/Pictures/Sorted` is expanded to the full home directory path.

**Resolution rules:**

1. If `--dest` starts with `@`, the remainder is looked up in the `aliases` map.
2. If the alias is not found, Pixe exits with an error listing available aliases.
3. If `--dest` does not start with `@`, it is used as a literal path (unchanged behavior).
4. Alias resolution happens after config merging but before destination validation.

**Config layering:** Aliases from a source-local `.pixe.yaml` are merged additively with global aliases. On collision, source-local wins. This lets a camera-specific source directory define `@default` pointing to its preferred archive.

**Aliases can be set anywhere `dest` is accepted:** `--dest @name` CLI flag, `dest: "@name"` in `.pixe.yaml`, or `PIXE_DEST=@name` environment variable.

---

## Environment variables

Any config key can be set via an environment variable prefixed with `PIXE_`. Viper maps the key name to `SCREAMING_SNAKE_CASE` automatically.

| Config key | Environment variable |
|:-----------|:--------------------|
| `dest` | `PIXE_DEST` |
| `algorithm` | `PIXE_ALGORITHM` |
| `workers` | `PIXE_WORKERS` |
| `recursive` | `PIXE_RECURSIVE` |
| `skip_duplicates` | `PIXE_SKIP_DUPLICATES` |
| `copyright` | `PIXE_COPYRIGHT` |
| `camera_owner` | `PIXE_CAMERA_OWNER` |
| `path_template` | `PIXE_PATH_TEMPLATE` |
| `no_carry_sidecars` | `PIXE_NO_CARRY_SIDECARS` |
| `overwrite_sidecar_tags` | `PIXE_OVERWRITE_SIDECAR_TAGS` |
| `db_path` | `PIXE_DB_PATH` |
| `dry_run` | `PIXE_DRY_RUN` |
| `since` | `PIXE_SINCE` |
| `before` | `PIXE_BEFORE` |

Boolean values accept `true`, `false`, `1`, or `0`.

Environment variables rank above config files but below CLI flags.

---

## Full annotated example

A complete `.pixe.yaml` showing every supported key:

```yaml
# Destination archive directory (required unless passed via --dest).
# Supports @alias syntax if aliases are defined below.
dest: /Volumes/NAS/Photos

# Hash algorithm for checksums embedded in filenames.
# Options: md5, sha1 (default), sha256, blake3, xxhash
algorithm: sha1

# Number of concurrent pipeline workers.
# 0 = auto-detect (runtime.NumCPU())
workers: 0

# Descend into subdirectories of the source directory.
recursive: false

# Skip copying duplicate files (record in DB only, no bytes written).
skip_duplicates: false

# Copyright metadata template. Tokens: {year}, {month}, {monthname}, {day}.
# Remove or leave empty to disable copyright tagging.
copyright: "Copyright {year} The Wells Family"

# Camera owner string written to XMP sidecar metadata.
# Remove or leave empty to disable.
camera_owner: "Wells Family"

# Directory structure template for the destination archive.
# Default: {year}/{month}-{monthname}  →  2021/12-Dec/
# Available tokens: {year}, {month}, {monthname}, {day}, {hour}, {minute}, {second}, {ext}
path_template: "{year}/{month}-{monthname}"

# Disable carrying pre-existing .aae and .xmp sidecars from source to destination.
# Default is false (sidecars ARE carried).
no_carry_sidecars: false

# When merging tags into a carried .xmp sidecar, overwrite existing values.
# Default is false (existing values in source .xmp are preserved).
overwrite_sidecar_tags: false

# Explicit SQLite database path. Leave empty for auto-resolution.
# db_path: /path/to/custom/pixe.db

# Glob patterns for files to exclude from discovery.
# Merged additively with patterns from source-local configs.
ignore:
  - ".DS_Store"
  - "Thumbs.db"
  - "*.thm"
  - "*.aae"
  - "node_modules/"

# Date filters (YYYY-MM-DD format). Both are optional.
# since: "2024-01-01"
# before: "2024-12-31"

# Destination aliases. Use @name with --dest to reference these paths.
# Tilde (~) is expanded to your home directory.
aliases:
  nas: /Volumes/NAS/Photos
  backup: /Volumes/Backup/Archive
  local: ~/Pictures/Sorted
```
