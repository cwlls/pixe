# Pixe Roadmap

This document tracks planned features and improvements for Pixe. Items are grouped by theme and annotated with an effort estimate.

**Effort scale:**
- 🟢 Low — A few files, well-contained, days of work
- 🟡 Medium — Multi-package changes, new tests, a week or two
- 🔴 High — Significant new architecture or new dependencies, weeks of work

---

## v2.4.0 (2026-03-12)

**Implemented features:**

- **A2** — AVIF handler (AV1 Image File Format with custom ISOBMFF parser for EXIF extraction)
- **A4** — Standalone TIFF handler (`.tif`/`.tiff` files with embedded `tiffraw.Base`)

---

## v2.3.0 (2026-03-12)

**Implemented features:**

- **B5** — Date filter flags (`--since`, `--before`) on `pixe sort`
- **D3** — Verbosity levels (`--quiet`, `--verbose`)
- **D4** — Colorized terminal output with TTY auto-detection
- **E1** — Config auto-discovery in source directory (`.pixe.yaml`)
- **C4** — `pixe stats` archive dashboard command
- **A3** — PNG file format handler
- **A6** — ORF (Olympus RAW) and RW2 (Panasonic RAW) handlers
- **E2** — Config profiles (`--profile` flag)

---

## A. New File Format Support

### A5 — RAF Handler (Fujifilm RAW) 🟡
Support for Fujifilm's proprietary RAF format. Unlike other RAW formats, RAF is not TIFF-based — it uses a custom container with an embedded JPEG preview and raw sensor data. Requires a new parser. Niche but Fujifilm has a devoted user base.

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

## H. Testing & Quality

### H1 — Fuzz Testing for Handlers 🟡
Apply Go's built-in `testing.F` fuzzer to the EXIF, TIFF, and ISOBMFF parsers. Catches crashes on malformed or adversarial input — important for a tool that processes untrusted media files from arbitrary sources.

### H2 — Benchmark Suite 🟡
Add `go test -bench` benchmarks for hashing throughput, copy throughput, DB query latency, and discovery walk speed. Enables detection of performance regressions across releases.

### H3 — Fixture Corpus Expansion 🟢
Add test fixtures covering edge cases: zero-byte files, files with no EXIF, corrupt headers, extremely large metadata blocks, Unicode filenames, and symlink farms.

### H4 — Property-Based Testing for Path Builder 🟢
Use `testing/quick` or `pgregory.net/rapid` to verify that `pathbuilder` always produces valid, deterministic paths for any valid input date. Complements the existing table-driven tests with exhaustive random input coverage.

---

## I. Data Safety & Integrity

### I2 — Extended Hash Algorithm Support + Algorithm-Tagged Filenames 🟡
**Status: Architected** (see `.state/ARCHITECTURE.md` Section 4.5.1, 4.5.2)

Add MD5, BLAKE3, and xxHash-64 alongside existing SHA-1 (default) and SHA-256. Each algorithm is assigned a stable numeric ID embedded in the destination filename: `YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>.<ext>`. This makes the hash algorithm identifiable from the filename alone, enables `pixe verify` to auto-detect the algorithm, and supports mixed-algorithm archives. Legacy filenames (pre-I2, no algorithm ID) are recognized and handled via digest-length inference. Requires two new dependencies (`github.com/zeebo/blake3`, `github.com/cespare/xxhash/v2`), a schema migration (new `algorithm` column on `files` table), and a ledger version bump (v4 → v5).

### I3 — Checksum Manifest Export 🟢
`pixe verify --export checksums.sha256` writes a standard `sha256sum`-compatible checksum file. Allows users to verify archive integrity with standard Unix tools, independent of Pixe and its database.

---

## Prioritization Notes

The following items offer the best impact-to-effort ratio and are good candidates for the next development cycle:

| Priority | Item | Rationale |
|:--------:|------|-----------|

