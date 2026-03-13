---
title: Installation
---

# Installation

## Install via Go

The fastest way to install Pixe. Requires Go 1.21 or later.

```bash
go install github.com/cwlls/pixe-go@latest
```

## Build from source

```bash
git clone https://github.com/cwlls/pixe-go.git
cd pixe-go
make build
```

The `make build` command uses GoReleaser to produce a production binary at `./pixe`. Run `make build-debug` to build with debug symbols for use with `dlv`.

## Quick start

Sort photos from the current directory:

```bash
pixe sort --dest ~/Archive
```

Specify a source and recurse into subdirectories:

```bash
pixe sort --source ~/Downloads/Photos --dest ~/Archive --recursive
```

Check the sort status of files in the current directory:

```bash
pixe status
```

Verify archive integrity (re-hashes every file):

```bash
pixe verify --dir ~/Archive
```

> **Tip:** Run `pixe sort --dry-run` first to preview exactly what would happen — no files are copied.

---

→ [See the full command reference](commands.md)
