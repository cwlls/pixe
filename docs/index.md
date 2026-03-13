---
title: Home
nav_order: 1
permalink: /
description: Safe, deterministic photo and video sorting
---

# Pixe

Safe, deterministic photo and video sorting.

Your originals are never touched. Every copy is verified before it counts. Interrupted runs resume exactly where they left off.

[Get started →](install.md) · [View on GitHub](https://github.com/cwlls/pixe){:target="_blank" rel="noopener"}

---

## The problem with photo libraries

You've got thousands of photos on your phone, a camera SD card, an old hard drive, maybe a USB stick from a vacation years ago. They pile up in folders with names like `IMG_4821.jpg`, duplicated across devices, with no consistent organization. Every tool that promises to fix this either modifies your originals, locks you into a subscription, or leaves you wondering whether it actually worked.

**What if the sort crashes halfway through?**
Pixe tracks every file in a SQLite database as it goes. Interrupted runs resume automatically — already-processed files are skipped.

**How do I know the copy actually worked?**
Every file is hashed before and after copying. Only when the hashes match does Pixe consider the copy complete. Mismatches are flagged, never silently ignored.

**What if the same photo appears twice?**
Duplicate detection is checksum-based, not name-based. The same image under different filenames is still caught. Duplicates go to a separate folder, or can be skipped entirely.

**Will Pixe touch my original files?**
Never. Source files are strictly read-only. Pixe copies to a destination directory — it never modifies, moves, or renames anything in your source folder.

**Will my archive look the same next year?**
Yes. Output is deterministic. The same photo always produces the same filename and directory path. Re-running Pixe on the same source produces the same archive.

**Do I need exiftool or ffmpeg installed?**
No. Pixe is a single binary with no runtime dependencies. All EXIF parsing, RAW decoding, and metadata handling is pure Go — nothing to install separately.

**Can I see what's happening in real time?**
Yes. Add `--progress` to any sort or verify command for a live progress bar with ETA, status counters, and current file display.

---

## How it works

Every file passes through a linear pipeline. If any stage fails, the file is flagged and the pipeline continues with the next file — nothing is silently skipped.

```
discover → extract → hash → copy → verify → tag → complete
```

- **Discover** — walk source, classify by type
- **Extract** — read capture date from metadata
- **Hash** — checksum the complete file contents
- **Copy** — write to temp file in destination
- **Verify** — re-hash destination, confirm match
- **Tag** — optionally inject copyright metadata
- **Complete** — atomically rename temp → canonical path

→ [Read the full internals breakdown](how-it-works.md)

---

## Quick start

Install (requires Go 1.25+):

```bash
go install github.com/cwlls/pixe@latest
```

Sort your photos:

```bash
$ pixe sort --dest ~/Archive
$ pixe sort --source ~/Photos --dest ~/Archive --recursive
```

Example output:

```
COPY IMG_0001.jpg -> 2021/12-Dec/20211225_062223-1-7d97e98f.jpg
SKIP IMG_0002.jpg -> previously imported
DUPE IMG_0003.jpg -> matches 2021/12-Dec/20211225_062223-1-7d97e98f.jpg
ERR  corrupt.jpg  -> extract date: no EXIF data

Done. processed=4 duplicates=1 skipped=1 errors=1
```

→ [Installation guide](install.md) · [All commands](commands.md)
