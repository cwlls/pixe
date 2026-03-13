# Pixe Roadmap

This document tracks planned features and improvements for Pixe. Items are grouped by theme and annotated with an effort estimate.

**Effort scale:**
- 🟢 Low — A few files, well-contained, days of work
- 🟡 Medium — Multi-package changes, new tests, a week or two
- 🔴 High — Significant new architecture or new dependencies, weeks of work

---

## Completed Features

### ✅ B4 — Configurable Destination Path Templates
User-defined path templates via `--path-template` flag. Token-based syntax with `{year}`, `{month}`, `{monthname}`, `{day}`, `{hour}`, `{minute}`, `{second}`, `{ext}`. Default template `{year}/{month}-{monthname}` preserves pre-template behavior. Fully validated at startup.

### ✅ E6 — Destination Aliases
`pixe sort --dest @nas` resolves `@`-prefixed aliases to filesystem paths configured in `.pixe.yaml` under `aliases`. Supports tilde expansion and source-local alias augmentation.

---

## B. Pipeline & Core Engine

---

## C. Archive Management & Maintenance

### C8 — Database Merge 🟡
When sorting to a NAS destination, Pixe intentionally keeps the run's database local to the machine performing the sort to avoid SQLite reliability issues over remote filesystems. After the sort completes, the user manually copies that local `.pixe.db` to the NAS. This feature would add a `pixe db merge` command that runs on the NAS-side machine, reads the copied database file, and folds its records into the NAS's own local copy of the archive database — reconciling file entries, run history, and checksums without duplicating existing records.

---

## D. User Experience & Output

### D5 — Machine-Readable Output (`--json`) 🟡
Emit newline-delimited JSON to stdout instead of the human-readable COPY/SKIP/DUPE/ERR lines. Mirrors the ledger format and enables scripting and integration with external tools.

---

## E. Configuration & Workflow

---

## I. Data Safety & Integrity

### I3 — Checksum Manifest Export 🟢
`pixe verify --export checksums.sha256` writes a standard `sha256sum`-compatible checksum file. Allows users to verify archive integrity with standard Unix tools, independent of Pixe and its database.

---

## Prioritization Notes

The following items offer the best impact-to-effort ratio and are good candidates for the next development cycle:

| Priority | Item | Rationale |
|:--------:|------|-----------|
