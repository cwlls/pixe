---
layout: page
title: Get started in minutes
section_label: Installation
permalink: /install/
---

### Install via Go

The fastest way to install Pixe. Requires Go 1.21 or later.

<div class="pre-label">Requires Go 1.21+</div>
```bash
go install github.com/cwlls/pixe-go@latest
```

### Build from source

<div class="pre-label">Clone and build</div>
```bash
git clone https://github.com/cwlls/pixe-go.git
cd pixe-go
make build
```

The `make build` command uses GoReleaser to produce a production binary at `./pixe`. Run `make build-debug` to build with debug symbols for use with `dlv`.

### Quick start

<div class="pre-label">Sort photos from the current directory</div>
```bash
pixe sort --dest ~/Archive
```

<div class="pre-label">Specify a source and recurse into subdirectories</div>
```bash
pixe sort --source ~/Downloads/Photos --dest ~/Archive --recursive
```

<div class="pre-label">Check the sort status of files in the current directory</div>
```bash
pixe status
```

<div class="pre-label">Verify archive integrity (re-hashes every file)</div>
```bash
pixe verify --dir ~/Archive
```

<div class="callout">
  <strong>Tip:</strong> Run <code>pixe sort --dry-run</code> first to preview exactly what would happen — no files are copied.
</div>

---

→ [See the full command reference](/commands/)
