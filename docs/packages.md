---
layout: page
title: Package Reference
section_label: Developer Guide
permalink: /packages/
---

This page is generated from the Go package doc comments in the source tree. Each entry reflects the `// Package` comment at the top of the package's primary `.go` file. Run `make docs` to regenerate after adding or updating packages.

<!-- pixe:begin:package-list -->
### Core Engine

**`internal/pipeline`** — Package pipeline implements the sort orchestrator that wires together discovery, extraction, hashing, path building, copy, verify, tagging, and archive-DB / ledger persistence.

**`internal/discovery`** — Package discovery walks a source directory and classifies files using a registry of FileTypeHandler implementations.

**`internal/copy`** — Package copy provides the atomic file copy and post-copy verification engine for the Pixe sort pipeline.

**`internal/verify`** — Package verify implements the archive integrity verification logic for the `pixe verify` command.

**`internal/hash`** — Package hash provides a configurable, streaming file hashing engine. It wraps Go's stdlib crypto primitives behind a uniform interface so the rest of the pipeline never imports crypto packages directly.

**`internal/pathbuilder`** — Package pathbuilder constructs deterministic output paths for sorted media files using the Pixe naming convention:

### Data & Persistence

**`internal/archivedb`** — Package archivedb provides SQLite-backed persistence for the Pixe archive database. It replaces the earlier JSON manifest with a cumulative registry that tracks all files ever sorted into a destination archive across all runs.

**`internal/manifest`** — Package manifest handles persistence of the Pixe manifest and ledger files.

**`internal/migrate`** — Package migrate handles automatic migration from the legacy JSON manifest (dirB/.pixe/manifest.json) to the SQLite archive database.

**`internal/dblocator`** — Package dblocator resolves the filesystem path for the Pixe archive database. It implements the priority chain: explicit --db-path → dbpath marker file → local default (dirB/.pixe/pixe.db), with automatic fallback to ~/.pixe/databases/<slug>.db when dirB is on a network filesystem.

**`internal/domain`** — Package domain defines shared types used across internal packages, including the FileTypeHandler interface, pipeline status types, and ledger/manifest structures.

**`internal/config`** — Package config provides the resolved runtime configuration for Pixe, populated from CLI flags, config file, and environment variables via Viper.

### File Type Handlers

**`internal/handler/jpeg`** — Package jpeg implements the FileTypeHandler contract for JPEG images.

**`internal/handler/heic`** — Package heic implements the FileTypeHandler contract for HEIC/HEIF images.

**`internal/handler/mp4`** — Package mp4 implements the FileTypeHandler contract for MP4/MOV video files.

**`internal/handler/tiffraw`** — Package tiffraw provides shared logic for TIFF-based RAW image formats.

**`internal/handler/dng`** — Package dng implements the FileTypeHandler contract for Adobe DNG (Digital Negative) RAW images.

**`internal/handler/nef`** — Package nef implements the FileTypeHandler contract for Nikon NEF RAW images.

**`internal/handler/cr2`** — Package cr2 implements the FileTypeHandler contract for Canon CR2 RAW images.

**`internal/handler/cr3`** — Package cr3 implements the FileTypeHandler contract for Canon CR3 RAW images.

**`internal/handler/pef`** — Package pef implements the FileTypeHandler contract for Pentax PEF RAW images.

**`internal/handler/arw`** — Package arw implements the FileTypeHandler contract for Sony ARW RAW images.

### Metadata

**`internal/tagging`** — Package tagging handles Copyright template rendering and metadata tag injection into destination files via the FileTypeHandler contract.

**`internal/xmp`** — Package xmp generates Adobe-compatible XMP sidecar files for media formats that cannot safely embed metadata. The sidecar follows the Adobe naming convention: <filename>.<ext>.xmp.

**`internal/ignore`** — Package ignore provides glob-based file ignore matching for Pixe discovery. It encapsulates both the hardcoded ledger-file exclusion and any user-configured patterns supplied via --ignore flags or .pixeignore files.

### User Interface

**`internal/progress`** — Package progress provides the pipeline event bus — a structured, typed channel that decouples the sort and verify pipelines from their output presentation. The pipeline emits Event values; consumers (CLI progress bars, the interactive TUI, or the plain-text writer) subscribe and render events in their own way.

**`internal/cli`** — Package cli provides the Bubble Tea progress bar model used by the `pixe sort --progress` and `pixe verify --progress` commands.

**`internal/tui`** — Package tui implements the interactive terminal UI for `pixe gui`.

### Other

**`cmd`** — Package cmd provides the Cobra CLI commands for Pixe.

**`internal/docgen`** — Package main implements the docgen tool that injects generated content into documentation files using marker-based replacement.

**`internal/fileutil`** — Package fileutil provides shared file-path utilities used across handler and discovery packages.

**`internal/handler/avif`** — Package avif implements the FileTypeHandler contract for AVIF images.

**`internal/handler/handlertest`** — Package handlertest provides a shared test suite for FileTypeHandler implementations that delegate to tiffraw.Base. Each handler test file calls RunSuite with handler-specific configuration to exercise the standard 10 behaviours without duplicating test logic.

**`internal/handler/orf`** — Package orf implements the FileTypeHandler contract for Olympus ORF RAW images.

**`internal/handler/png`** — Package png implements the FileTypeHandler contract for PNG images.

**`internal/handler/rw2`** — Package rw2 implements the FileTypeHandler contract for Panasonic RW2 RAW images.

**`internal/handler/tiff`** — Package tiff implements the FileTypeHandler contract for standalone TIFF images.
<!-- pixe:end:package-list -->
