---
title: Technical Design
---

# Technical Design

Pixe makes specific engineering choices that distinguish it from other photo sorting tools. This page explains those choices and why they matter for a tool whose job is to handle irreplaceable media.

---

## Source files are never touched

Pixe treats the source directory as strictly read-only. It never modifies, renames, moves, or deletes files in your source folder. The only thing it writes there is a `.pixe_ledger.json` receipt — a record of what it saw and what it did.

This constraint is non-negotiable. Family photos, RAW files from a shoot, archival scans — these are irreplaceable. A tool that modifies originals as part of its workflow introduces risk that simply should not exist. Even a metadata write to the wrong byte offset can corrupt a file. Pixe eliminates that risk entirely by never touching the source.

---

## Copy-then-verify

Every file Pixe copies goes through a two-phase integrity check. First, the file is written to a temporary location in the destination. Then it is independently re-read and re-hashed. Only when the checksum of the destination matches the checksum of the source does Pixe consider the copy complete — and only then does it rename the temp file to its canonical path.

This matters because storage hardware is not perfectly reliable. USB drives drop bytes. NAS connections hiccup mid-transfer. Disks develop bad sectors that the filesystem silently accepts. A checksum on both sides catches what the OS won't tell you. If there's a mismatch, the temp file is deleted, the source is untouched, and the file is flagged for retry. A file at its canonical path in the archive is always verified.

---

## Deterministic output

Given the same input files and the same configuration, Pixe always produces the same archive structure. The same photo always maps to the same filename and the same directory path. Re-running Pixe on the same source produces the same archive.

This property enables confidence in several workflows: you can re-run Pixe on a source you've already sorted and know that already-imported files will be skipped and nothing will be duplicated. You can merge archives from different sources and reason about what will happen. You can verify an archive years later by re-running with the same algorithm and comparing checksums. Determinism is what makes a tool trustworthy over time.

---

## No external dependencies

Pixe is a single binary. There is no exiftool to install, no ffmpeg, no ImageMagick, no Python runtime. All EXIF parsing, RAW container decoding, video metadata extraction, and file hashing is implemented in pure Go and compiled into the binary.

This matters for long-term archival. A tool that depends on a chain of external software is a tool that may stop working when one of those dependencies changes its behavior, drops a flag, or becomes unavailable. Pixe's dependency surface is: the binary, and the files you give it. That's it. Install it once, run it in ten years, and it will behave identically.

---

## Crash-safe by design

Pixe is designed to survive interruption at any point without data loss or corruption.

Each file's completion is committed individually to the SQLite database. If the process is killed mid-run, at most one in-flight file is lost — every file committed before the interruption is safe. The JSONL ledger in the source directory is streamed line-by-line as files are processed, so an interrupted run leaves a valid (if incomplete) receipt. Temp files in the destination are left on disk but are harmless — they don't interfere with subsequent runs and can be cleaned with `pixe clean`.

Sorting 50,000 photos takes hours. Power failures happen. Laptops sleep. `pixe resume` picks up exactly where the last run left off.

---

## Content-based deduplication

Pixe identifies duplicates by checksum of the complete file contents — not by filename, path, or metadata. This means:

`IMG_0001.jpg` from your phone and `IMG_0001.jpg` from your partner's phone are correctly identified as different files, even though they have the same name. The same photo renamed to `vacation_sunset.jpg` is correctly identified as a duplicate of the original, even though the name is different.

Filenames are meaningless for identity. Content is truth.
