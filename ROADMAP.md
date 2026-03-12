# Pixe Roadmap

This document tracks planned features and improvements for Pixe. Items are grouped by theme and annotated with an effort estimate.

**Effort scale:**
- 🟢 Low — A few files, well-contained, days of work
- 🟡 Medium — Multi-package changes, new tests, a week or two
- 🔴 High — Significant new architecture or new dependencies, weeks of work

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

### A2 — AVIF Handler 🟡
Support for AVIF (AV1 Image File Format), the HEIC successor used by iPhone 16+ and modern Android devices. The ISOBMFF container parsing can share logic with the existing HEIC handler. Requires evaluating a pure-Go AVIF library.

### A3 — PNG Handler 🟢
Support for `.png` files — common for screenshots and edited photos. The stdlib `image/png` package handles decoding. EXIF lives in the `eXIf` chunk (PNG 1.5+) or `tEXt`/`iTXt` chunks; date extraction may be unreliable for PNGs that lack EXIF.

### A4 — Standalone TIFF Handler 🟢
A thin handler for `.tif`/`.tiff` files produced by scanners and professional workflows. The existing `tiffraw.Base` already handles the heavy lifting — this is primarily extension registration and magic-byte detection.

### A5 — RAF Handler (Fujifilm RAW) 🟡
Support for Fujifilm's proprietary RAF format. Unlike other RAW formats, RAF is not TIFF-based — it uses a custom container with an embedded JPEG preview and raw sensor data. Requires a new parser. Niche but Fujifilm has a devoted user base.

### A6 — ORF / RW2 Handlers (Olympus / Panasonic RAW) 🟢
Both formats are TIFF-based and can use `tiffraw.Base`. Adding them completes coverage of the major mirrorless camera RAW ecosystem with minimal new code.

---

## B. Pipeline & Core Engine

### B4 — Configurable Destination Path Templates 🟡
Replace the hardcoded `YYYY/MM-Mon/YYYYMMDD_HHMMSS_CHECKSUM.ext` structure with a user-defined template (e.g., `--path-template "{{.Year}}/{{.Month}}-{{.MonthName}}/{{.Filename}}"`). Requires careful design to preserve determinism guarantees.

### B5 — Date Filters on `sort` 🟢
Add `--since` and `--before` flags to `pixe sort` so only files with capture dates in a given range are processed. The capture date is already extracted — this is a simple filter gate in the pipeline. Highly useful for re-importing a specific trip or time period.

---

## C. Archive Management & Maintenance

### C4 — `pixe stats` Archive Dashboard 🟢
A quick summary command: total files, total size, date range, format breakdown, error rate, and last run date. Most of this data is already queryable via `ArchiveStats()` — this is primarily a presentation layer.

### C7 — Database Backup / Export 🟢
`pixe query export --dir ./archive --format csv` dumps the archive database to CSV or JSON for external analysis, spreadsheets, or backup. Enables users to work with their archive metadata independently of Pixe.

### C8 — Database Merge 🟡
When sorting to a NAS destination, Pixe intentionally keeps the run's database local to the machine performing the sort to avoid SQLite reliability issues over remote filesystems. After the sort completes, the user manually copies that local `.pixe.db` to the NAS. This feature would add a `pixe db merge` command that runs on the NAS-side machine, reads the copied database file, and folds its records into the NAS's own local copy of the archive database — reconciling file entries, run history, and checksums without duplicating existing records.

---

## D. User Experience & Output

### D3 — `--quiet` / `--verbose` Log Levels 🟢
`--quiet` suppresses per-file output and shows only the final summary. `--verbose` adds per-stage timing, worker assignments, and debug information. Currently there is only one output level.

### D4 — Colorized Terminal Output 🟢
Color-code status lines: COPY in green, DUPE in yellow, ERR in red, SKIP in dim. Lip Gloss is already a dependency. Auto-detect TTY and disable colors when stdout is piped.

### D5 — Machine-Readable Output (`--json`) 🟡
Emit newline-delimited JSON to stdout instead of the human-readable COPY/SKIP/DUPE/ERR lines. Mirrors the ledger format and enables scripting and integration with external tools.

---

## E. Configuration & Workflow

### E1 — Config Auto-Discovery in `dirA` 🟢
Look for a `.pixe.yaml` in the source directory before falling back to the global config location. This lets users place per-project configs alongside their photos — for example, a `.pixe.yaml` at the root of an SD card that sets copyright and destination automatically.

### E2 — Config Profiles 🟡
`pixe sort --profile family` loads `~/.pixe/profiles/family.yaml`. Different cameras or sources can have different copyright strings, hash algorithms, or destinations without requiring flags on every invocation.

### E3 — `pixe init` Interactive Setup Wizard 🟡
A guided setup command that asks questions and generates a `.pixe.yaml`. Lowers the barrier to entry for new users who are unfamiliar with the available configuration options.

### E6 — Destination Aliases 🟢
`pixe sort --dest @nas` resolves to a path configured in `.pixe.yaml` under `aliases`. Saves typing long or environment-specific paths on every invocation.

---

## G. Distribution & Platform

### G5 — Linux ARM Builds (Raspberry Pi / NAS) 🟢
Add `linux/arm64` and `linux/arm` targets to the GoReleaser build matrix. GoReleaser already supports cross-compilation — this is a configuration change. Enables deployment on Raspberry Pi, Synology, Unraid, and similar NAS platforms.

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

### I2 — xxHash / BLAKE3 Hash Algorithm Option 🟡
Add BLAKE3 (and optionally xxHash) as selectable hash algorithms for users who prioritize throughput over cryptographic strength. BLAKE3 is approximately 3× faster than SHA-256 on modern CPUs. Requires a new dependency and a hash-algorithm option in config.

### I3 — Checksum Manifest Export 🟢
`pixe verify --export checksums.sha256` writes a standard `sha256sum`-compatible checksum file. Allows users to verify archive integrity with standard Unix tools, independent of Pixe and its database.

---

## Prioritization Notes

The following items offer the best impact-to-effort ratio and are good candidates for the next development cycle:

| Priority | Item | Rationale |
|:--------:|------|-----------|
| 1 | **B5** Date filters | Trivial effort, immediate workflow value |
| 2 | **D3** Quiet/verbose levels | Trivial effort, immediate UX improvement |
| 3 | **D4** Colorized output | Trivial effort, Lip Gloss already available |
| 4 | **E1** Config auto-discovery | Low effort, significant workflow improvement |
| 5 | **C4** `pixe stats` | Low effort, highly requested capability |
| 6 | **A3** PNG handler | Low effort, very common format |
| 7 | **A6** ORF/RW2 handlers | Low effort, completes RAW ecosystem coverage |
| 8 | **E2** Config profiles | Medium effort, high value for multi-camera households |
