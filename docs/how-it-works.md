---
title: How It Works
nav_order: 5
---

# How It Works

Every file passes through a linear pipeline. If any stage fails, the file is flagged and the pipeline continues with the next file — nothing is silently skipped.

```
discover → extract → hash → copy → verify → carry sidecars → tag → complete
```

## Pipeline stages

Each file moves through these stages in order, with its state tracked in the archive database at every step:

**Discover** — The source directory is walked and each file is classified by its handler (JPEG, HEIC, RAW, etc.) using extension-based detection followed by magic-byte verification. Files with no matching handler are recorded as skipped.

**Extract** — The handler reads the file's metadata to extract the capture date. The fallback chain is: EXIF `DateTimeOriginal` → EXIF `CreateDate` → February 20, 1902 (Ansel Adams' birthday, used as a sentinel for undated files).

**Hash** — A checksum is computed over the complete file contents. Destination files are byte-identical copies of their source, and metadata is expressed exclusively via XMP sidecars — the full-file hash remains stable regardless of tagging operations.

**Copy** — The file is written to a temporary file (`.<filename>.pixe-tmp-<random>`) in the destination directory. The temp file lives in the same directory as the final destination to guarantee that the rename in the next step is atomic.

**Verify** — The temp file is independently re-read and re-hashed. If the checksum matches the source hash, the file is good. If it doesn't match, the temp file is deleted and the file is flagged as a mismatch — the source is untouched and can be reprocessed.

**Carry sidecars** — Pre-existing `.aae` and `.xmp` sidecar files associated with the media file are copied to the destination alongside it. Sidecars are matched by stem (case-insensitive). Sidecar carry failure is non-fatal — the media file is still marked complete.

**Tag** — If `--copyright` or `--camera-owner` is configured, metadata is injected. All formats receive an XMP sidecar file alongside the destination copy. When a `.xmp` sidecar was carried from the source, tags are merged into it instead of generating a new one — existing values in the source `.xmp` are preserved by default (`--overwrite-sidecar-tags` inverts this). If no tags are configured, this stage is skipped.

**Complete** — The temp file is atomically renamed to its canonical destination path. A file at its canonical path in the archive is always complete and verified.

**Error states** — `failed` (pipeline error), `mismatch` (hash mismatch after copy), `tag_failed` (media copied and verified, but metadata write failed). Error states halt processing for that file only; the pipeline continues with the next.

---

## Output format

Every file discovered in the source produces exactly one line of output:

```
COPY IMG_0001.jpg -> 2021/12-Dec/20211225_062223-1-7d97e98f.jpg
SKIP IMG_0002.jpg -> previously imported
DUPE IMG_0003.jpg -> matches 2021/12-Dec/20211225_062223-1-7d97e98f.jpg
ERR  corrupt.jpg  -> extract date: no EXIF data

Done. processed=4 duplicates=1 skipped=1 errors=1
```

| Verb | Meaning |
|---|---|
| `COPY` | File successfully processed and copied to the archive |
| `SKIP` | File not copied — already imported in a prior run, or unsupported format |
| `DUPE` | File is a content duplicate of an already-archived file |
| `ERR` | File processing failed at some pipeline stage |

---

## Carry sidecars

When sorting media files, Pixe automatically detects and carries pre-existing `.aae` and `.xmp` sidecar files from the source directory to the destination archive. This preserves metadata and edits that were created alongside the original media files.

**How it works:**

- **Automatic detection** — Sidecars are matched to their parent media file by stem (filename without extension). `IMG_1234.xmp` associates with `IMG_1234.HEIC`; the full-extension Adobe convention `IMG_1234.HEIC.xmp` is also supported and takes priority.
- **Case-insensitive matching** — `img_1234.xmp` matches `IMG_1234.HEIC`.
- **Destination naming** — The sidecar is renamed to match the destination media file. For example, if `IMG_1234.HEIC` becomes `20211225_062223-1-7d97e98f.heic`, then `IMG_1234.aae` becomes `20211225_062223-1-7d97e98f.heic.aae`.
- **Enabled by default** — Use `--no-carry-sidecars` to disable sidecar carry entirely.
- **Orphan sidecars** — Sidecars with no matching media file are reported as `SKIP` with reason `orphan sidecar: no matching media file`.
- **Dry-run preview** — In dry-run mode, `+sidecar` lines appear in the output showing what would be carried, without copying any files.
- **Duplicate handling** — When a media file is a duplicate, its sidecars follow it to the `duplicates/` directory. When `--skip-duplicates` is active, sidecars are not copied.

**Output format:**

```
COPY IMG_1234.HEIC -> 2021/12-Dec/20211225_062223-1-7d97e98f.heic
     +sidecar IMG_1234.aae -> 2021/12-Dec/20211225_062223-1-7d97e98f.heic.aae
     +sidecar IMG_1234.xmp -> 2021/12-Dec/20211225_062223-1-7d97e98f.heic.xmp (merge tags)
```

**XMP tag merge:**

When a `.xmp` sidecar is carried AND `--copyright` or `--camera-owner` is configured, Pixe merges the new tags into the carried sidecar instead of generating a new one. By default, existing values in the source `.xmp` are preserved (source is authoritative). Use `--overwrite-sidecar-tags` to replace existing values with the new tags instead.

---

## Output naming

Files are named using a deterministic scheme that encodes the capture date, hash algorithm, and content checksum:

```
YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>
```

The algorithm ID is a single digit: `0`=MD5, `1`=SHA-1 (default), `2`=SHA-256, `3`=BLAKE3, `4`=xxHash. This allows `pixe verify` to auto-detect the correct algorithm from the filename without consulting the database.

Organized into a date-based directory structure:

```
<dest>/<YYYY>/<MM-Mon>/<filename>
```

**Example:** `Archive/2021/12-Dec/20211225_062223-1-7d97e98fbc14....jpg`

The month directory uses a locale-aware three-letter abbreviation (`01-Jan`, `02-Feb`, ..., `12-Dec`). On a French locale, this would produce `03-Mars` instead of `03-Mar`. The numeric prefix is always zero-padded.

Files without readable EXIF dates fall back to **February 20, 1902** (Ansel Adams' birthday) — making undated files easy to identify by their `19020220` prefix.

---

## Date fallback chain

Pixe extracts capture dates using this priority order:

1. **EXIF `DateTimeOriginal`** — Most reliable; represents shutter actuation.
2. **EXIF `CreateDate`** — Secondary; may differ for edited files.
3. **February 20, 1902** — Ansel Adams' birthday. Used when no metadata date is available.

Filesystem timestamps (`ModTime`, `CreationTime`) are explicitly **not used** — they are unreliable across copies, cloud syncs, and OS transfers.

---

## Duplicate handling

When a file's checksum matches an already-archived file, the behavior depends on the `--skip-duplicates` flag.

**Default behavior** — The duplicate is physically copied to a quarantine directory for user review:

```
<dest>/duplicates/<run_timestamp>/<YYYY>/<MM-Mon>/<filename>
```

**With `--skip-duplicates`** — No bytes are written to the destination. The file is detected, checksummed, and recorded in the database and ledger as a duplicate, but no copy is made. This eliminates the I/O cost of copying files already in the archive.

---

## Archive database

A SQLite database at `<dest>/.pixe/<slug>.db` tracks every file across every run. It enables:

- **Duplicate detection** — Indexed checksum lookups without loading all checksums into memory
- **Skip-on-resume** — Files already processed in prior runs are skipped automatically
- **Queryable history** — `pixe query` exposes the full run history, duplicates, errors, and inventory

The database uses WAL mode for concurrent-process safety. Multiple `pixe sort` processes can target the same archive simultaneously without corruption.

---

## Live progress

Add `--progress` to `pixe sort` or `pixe verify` to replace per-file text output with a live progress bar. Shows a gradient bar, file count, ETA, current file, and status counters. Only activates when stdout is a TTY; plain text is the default.

The progress bar is powered by a pipeline event bus (`internal/progress/`) — a pure stdlib, non-blocking channel that the sort and verify pipelines emit structured events into. The plain-text output and the event bus are always active simultaneously; `--progress` simply consumes events instead of printing text.

---

## Supported file types

<!-- pixe:begin:format-table -->

| Format  | Extensions   | Metadata    |
| ------- | ------------ | ----------- |
| ARW     | .arw         | XMP sidecar |
| AVIF    | .avif        | XMP sidecar |
| CR2     | .cr2         | XMP sidecar |
| CR3     | .cr3         | XMP sidecar |
| DNG     | .dng         | XMP sidecar |
| HEIC    | .heic, .heif | XMP sidecar |
| JPEG    | .jpg, .jpeg  | XMP sidecar |
| MP4/MOV | .mp4, .mov   | XMP sidecar |
| NEF     | .nef         | XMP sidecar |
| ORF     | .orf         | XMP sidecar |
| PEF     | .pef         | XMP sidecar |
| PNG     | .png         | XMP sidecar |
| RAF     | .raf         | XMP sidecar |
| RW2     | .rw2         | XMP sidecar |
| TIFF    | .tif, .tiff  | XMP sidecar |

<!-- pixe:end:format-table -->

All formats support the full pipeline: date extraction, content hashing, copy-then-verify, and metadata tagging via XMP sidecar.
