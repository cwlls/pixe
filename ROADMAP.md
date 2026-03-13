# Pixe Roadmap

This document tracks planned features and improvements for Pixe. Items are grouped by theme and annotated with an effort estimate.

**Effort scale:**
- 🟢 Low — A few files, well-contained, days of work
- 🟡 Medium — Multi-package changes, new tests, a week or two
- 🔴 High — Significant new architecture or new dependencies, weeks of work

---

## B. Pipeline & Core Engine

### B4 — Configurable Destination Path Templates 🟡
Replace the hardcoded `YYYY/MM-Mon/YYYYMMDD_HHMMSS_CHECKSUM.ext` structure with a user-defined template (e.g., `--path-template "{{.Year}}/{{.Month}}-{{.MonthName}}/{{.Filename}}"`). Requires careful design to preserve determinism guarantees.

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

### E6 — Destination Aliases 🟢
`pixe sort --dest @nas` resolves to a path configured in `.pixe.yaml` under `aliases`. Saves typing long or environment-specific paths on every invocation.

---

## I. Data Safety & Integrity

### I3 — Checksum Manifest Export 🟢
`pixe verify --export checksums.sha256` writes a standard `sha256sum`-compatible checksum file. Allows users to verify archive integrity with standard Unix tools, independent of Pixe and its database.

---

## Prioritization Notes

The following items offer the best impact-to-effort ratio and are good candidates for the next development cycle:

| Priority | Item | Rationale |
|:--------:|------|-----------|
