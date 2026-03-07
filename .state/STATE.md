# Pixe Implementation State

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Project Scaffold & Go Module Init | High | @developer | ✅ Complete | — | Go module, directory layout, Cobra/Viper bootstrap |
| 2 | Core Domain Types & Interfaces | High | @developer | ✅ Complete | 1 | FileTypeHandler contract, pipeline types, config structs |
| 3 | Hashing Engine | High | @developer | ✅ Complete | 2 | Configurable hash.Hash factory, streaming io.Reader consumer |
| 4 | Manifest & Ledger Persistence | High | @developer | ✅ Complete | 2 | JSON read/write, atomic saves, per-file state tracking |
| 5 | File Discovery & Handler Registry | High | @developer | ✅ Complete | 2 | Walk dirA, extension match, magic-byte verify, skip dotfiles |
| 6 | Path Builder (Naming & Dedup) | High | @developer | ✅ Complete | 2, 3 | Deterministic output paths, duplicate routing |
| 7 | JPEG Filetype Module | High | @developer | ✅ Complete | 2, 3 | First concrete handler — proves the contract |
| 8 | Copy & Verify Engine | High | @developer | ✅ Complete | 3, 4, 6 | Streamed copy, post-copy re-hash, manifest updates |
| 9 | Sort Pipeline Orchestrator | High | @developer | ✅ Complete | 5, 7, 8 | Single-threaded first: discover → extract → hash → copy → verify |
| 10 | CLI: `pixe sort` Command | High | @developer | ✅ Complete | 9 | Cobra command, Viper flag binding, dry-run mode |
| 11 | Worker Pool & Concurrency | Medium | @developer | ✅ Complete | 9 | Coordinator + N workers, configurable --workers |
| 12 | HEIC Filetype Module | Medium | @developer | ✅ Complete | 7 | Second handler — validates contract generality |
| 13 | MP4 Filetype Module | Medium | @developer | ✅ Complete | 7 | Third handler — video keyframe hashing |
| 14 | Metadata Tagging Engine | Medium | @developer | ✅ Complete | 7, 8 | Copyright template, CameraOwner injection post-verify |
| 15 | CLI: `pixe verify` Command | Medium | @developer | ✅ Complete | 3, 5, 10 | Walk dirB, parse filename checksum, report mismatches |
| 16 | CLI: `pixe resume` Command | Medium | @developer | ✅ Complete | 4, 9, 10 | Load manifest, skip completed, re-enter pipeline |
| 17 | Integration Tests & Safety Audit | High | @tester | ✅ Complete | 10, 15, 16 | End-to-end with fixture files, interrupt simulation |
| 18 | Makefile & Build Tooling | Medium | @developer | ✅ Complete | 1 | help, build, test, lint, check, install targets; ldflags version injection |
| 19 | Version Package — Single Source of Truth | High | @developer | ✅ Complete | — | `internal/version/version.go`: const, vars, `Full()` accessor |
| 20 | CLI: `pixe version` Command | High | @developer | ✅ Complete | 19 | Cobra subcommand in `cmd/version.go` |
| 21 | Domain Structs — Add `PixeVersion` Field | High | @developer | ✅ Complete | 19 | Add field to `Manifest` and `Ledger` in `internal/domain/pipeline.go` |
| 22 | Pipeline — Populate `PixeVersion` at Runtime | High | @developer | ✅ Complete | 19, 21 | Wire `version.Version` into manifest/ledger creation in pipeline + worker |
| 23 | Makefile — Retarget ldflags to `internal/version` | Medium | @developer | ✅ Complete | 19 | Update LDFLAGS paths, remove Version override |
| 24 | Tests & Verification | High | @tester | ✅ Complete | 19, 20, 21, 22, 23 | Unit tests for version pkg, manifest round-trip with new field, `go vet`, full test suite green |
| 25 | Lint Fixes — golangci-lint 0 issues | High | @developer | ✅ Complete | 1–24 | Fixed 30+ errcheck and unused lint violations across copy, discovery, heic, jpeg, mp4, verify, hash, manifest, pipeline packages; installed golangci-lint |

---

## Milestone: Tasks 1–18 Complete

All 18 original tasks have been completed. The pixe-go photo organization tool is fully functional with support for sorting, verifying, and resuming operations across JPEG, HEIC, and MP4 file types.

## Feature: Centralized Version Management (Tasks 19–24) — Complete

Adds a single-source-of-truth version package, a `pixe version` CLI command, and embeds the Pixe version into manifests and ledgers. See Architecture Section 3.

All 24 tasks complete. `pixe v0.9.0` ships with full version management.

---

## Task 1 — Project Scaffold & Go Module Init

**Goal:** Establish the Go module, directory layout, and a runnable `pixe` binary that prints help text.

**Acceptance Criteria:**
- `go build ./...` succeeds with zero errors.
- Running `./pixe` prints Cobra root help with `sort`, `verify`, and `resume` listed as subcommands (stubs only — no logic).
- `go test ./...` passes (even if no tests exist yet).

**Directory Layout:**
```
pixe-go/
├── go.mod                          # module github.com/wellsiau/pixe-go (or chosen path)
├── go.sum
├── main.go                         # func main() { cmd.Execute() }
├── cmd/
│   ├── root.go                     # Cobra root command, Viper config init
│   ├── sort.go                     # Stub: pixe sort
│   ├── verify.go                   # Stub: pixe verify
│   └── resume.go                   # Stub: pixe resume
├── internal/
│   ├── config/                     # Viper config loading, struct definitions
│   ├── domain/                     # Core types: FileTypeHandler, pipeline enums, MetadataTags
│   ├── hash/                       # Hashing engine (Task 3)
│   ├── manifest/                   # Manifest + Ledger persistence (Task 4)
│   ├── discovery/                  # File walker + handler registry (Task 5)
│   ├── pathbuilder/                # Output path construction (Task 6)
│   ├── pipeline/                   # Orchestrator + worker pool (Tasks 9, 11)
│   ├── copy/                       # Copy + verify engine (Task 8)
│   ├── tagging/                    # Metadata tag injection (Task 14)
│   └── handler/                    # Filetype modules
│       ├── jpeg/                   # (Task 7)
│       ├── heic/                   # (Task 12)
│       └── mp4/                    # (Task 13)
└── .state/
    ├── ARCHITECTURAL_OVERVIEW.md
    └── STATE.md
```

**Technical Notes:**
- `cmd/root.go`: Initialize Viper with config file search paths (`$HOME/.pixe.yaml`, `./.pixe.yaml`). Bind `--workers` and `--algorithm` as persistent flags on root so all subcommands inherit them.
- All subcommands return `fmt.Println("not implemented")` for now.
- Dependencies to `go get`: `github.com/spf13/cobra`, `github.com/spf13/viper`.

---

## Task 2 — Core Domain Types & Interfaces

**Goal:** Define the shared types that every other package imports. This is the contract layer — no implementations yet.

**Acceptance Criteria:**
- Package `internal/domain` compiles.
- The `FileTypeHandler` interface is defined exactly as specified in the Architecture (Section 5.1).
- All supporting types (`MetadataTags`, `MagicSignature`, `FileStatus`, `ManifestEntry`, `Manifest`, `LedgerEntry`, `Ledger`) are defined.
- Unit tests validate enum string conversions for `FileStatus`.

**File: `internal/domain/handler.go`**
```go
package domain

import (
    "io"
    "time"
)

// MagicSignature defines a byte pattern at a specific offset for file type detection.
type MagicSignature struct {
    Offset int
    Bytes  []byte
}

// MetadataTags holds optional tags to inject into destination files.
type MetadataTags struct {
    Copyright   string // Already template-rendered (e.g., "Copyright 2021 My Family...")
    CameraOwner string
}

// FileTypeHandler is the contract every filetype module must implement.
type FileTypeHandler interface {
    Detect(filePath string) (bool, error)
    ExtractDate(filePath string) (time.Time, error)
    HashableReader(filePath string) (io.Reader, error)
    WriteMetadataTags(filePath string, tags MetadataTags) error
    Extensions() []string
    MagicBytes() []MagicSignature
}
```

**File: `internal/domain/pipeline.go`**
```go
package domain

import "time"

// FileStatus represents the processing state of a single file.
type FileStatus string

const (
    StatusPending   FileStatus = "pending"
    StatusExtracted FileStatus = "extracted"
    StatusHashed    FileStatus = "hashed"
    StatusCopied    FileStatus = "copied"
    StatusVerified  FileStatus = "verified"
    StatusTagged    FileStatus = "tagged"
    StatusComplete  FileStatus = "complete"
    StatusFailed    FileStatus = "failed"
    StatusMismatch  FileStatus = "mismatch"
    StatusTagFailed FileStatus = "tag_failed"
)

// ManifestEntry tracks the state of a single file through the pipeline.
type ManifestEntry struct {
    Source      string     `json:"source"`
    Destination string     `json:"destination,omitempty"`
    Checksum    string     `json:"checksum,omitempty"`
    Status      FileStatus `json:"status"`
    ExtractedAt *time.Time `json:"extracted_at,omitempty"`
    CopiedAt    *time.Time `json:"copied_at,omitempty"`
    VerifiedAt  *time.Time `json:"verified_at,omitempty"`
    TaggedAt    *time.Time `json:"tagged_at,omitempty"`
    Error       string     `json:"error,omitempty"`
}

// Manifest is the top-level operational journal written to dirB/.pixe/manifest.json.
type Manifest struct {
    Version     int              `json:"version"`
    Source      string           `json:"source"`
    Destination string           `json:"destination"`
    Algorithm   string           `json:"algorithm"`
    StartedAt   time.Time        `json:"started_at"`
    Workers     int              `json:"workers"`
    Files       []*ManifestEntry `json:"files"`
}

// LedgerEntry is a minimal record of a successfully processed file.
type LedgerEntry struct {
    Path        string    `json:"path"`
    Checksum    string    `json:"checksum"`
    Destination string    `json:"destination"`
    VerifiedAt  time.Time `json:"verified_at"`
}

// Ledger is written to dirA/.pixe_ledger.json.
type Ledger struct {
    Version     int           `json:"version"`
    PixeRun     time.Time     `json:"pixe_run"`
    Algorithm   string        `json:"algorithm"`
    Destination string        `json:"destination"`
    Files       []LedgerEntry `json:"files"`
}
```

**File: `internal/config/config.go`**
```go
package config

// AppConfig holds the resolved runtime configuration.
type AppConfig struct {
    Source      string
    Destination string
    Workers     int
    Algorithm   string // "sha1", "sha256", etc.
    Copyright   string // Raw template string, e.g. "Copyright {{.Year}} ..."
    CameraOwner string
    DryRun      bool
}
```

---

## Task 3 — Hashing Engine

**Goal:** A package that accepts an `io.Reader` and a named algorithm, and returns the hex-encoded checksum. Streaming — never buffers the full file.

**Acceptance Criteria:**
- `hash.NewHasher("sha1")` and `hash.NewHasher("sha256")` return valid hashers.
- `hash.NewHasher("unsupported")` returns a descriptive error.
- `hasher.Sum(reader)` returns `(string, error)` — the hex-encoded digest.
- Unit tests hash a known byte string and assert the expected digest for both SHA-1 and SHA-256.

**File: `internal/hash/hasher.go`**
```go
package hash

import (
    "crypto/sha1"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "hash"
    "io"
)

// Hasher wraps a configurable hash algorithm.
type Hasher struct {
    newFunc func() hash.Hash
    name    string
}

// NewHasher returns a Hasher for the named algorithm.
// Supported: "sha1", "sha256".
func NewHasher(algorithm string) (*Hasher, error) { ... }

// Sum reads the entire stream from r and returns the hex-encoded digest.
// It reads in 32KB chunks to bound memory usage.
func (h *Hasher) Sum(r io.Reader) (string, error) { ... }

// Algorithm returns the name of the hash algorithm.
func (h *Hasher) Algorithm() string { return h.name }
```

---

## Task 4 — Manifest & Ledger Persistence

**Goal:** Read, write, and atomically update the manifest and ledger JSON files. Must be safe against partial writes (write to temp file, then rename).

**Acceptance Criteria:**
- `manifest.Save(m *domain.Manifest, dirB string)` writes to `dirB/.pixe/manifest.json` atomically (write tmp → rename).
- `manifest.Load(dirB string)` reads and deserializes the manifest, returning `(nil, nil)` if no manifest exists.
- `ledger.Save(l *domain.Ledger, dirA string)` writes to `dirA/.pixe_ledger.json` atomically.
- `ledger.Load(dirA string)` reads and deserializes the ledger.
- Unit tests: round-trip save/load, verify atomic write doesn't corrupt on simulated error.
- Creates `dirB/.pixe/` directory if it doesn't exist.

**Technical Notes:**
- Atomic write pattern: write to `manifest.json.tmp`, then `os.Rename` to `manifest.json`. This is safe on POSIX filesystems. On cross-filesystem scenarios, the manifest lives in `dirB` so rename is always same-filesystem.
- JSON is indented for human readability (`json.MarshalIndent`).

---

## Task 5 — File Discovery & Handler Registry

**Goal:** Walk `dirA`, classify each file using registered handlers, and return a list of discovered files with their assigned handler.

**Acceptance Criteria:**
- `registry.Register(handler domain.FileTypeHandler)` adds a handler.
- `registry.Detect(filePath string)` returns the matching handler or `nil` for unrecognized files.
- Detection order: match extension → verify magic bytes. If magic bytes fail, try all other handlers' magic bytes. If none match, return `nil`.
- `discovery.Walk(dirA string, registry *Registry)` returns `[]DiscoveredFile` and `[]SkippedFile`.
- Skips dotfiles and directories (e.g., `.pixe_ledger.json`, `.DS_Store`).
- Unit tests with mock handlers asserting extension match, magic byte verification, and reclassification.

**Key Types:**
```go
// DiscoveredFile pairs a source path with its resolved handler.
type DiscoveredFile struct {
    Path    string
    Handler domain.FileTypeHandler
}

// SkippedFile records a file that could not be classified.
type SkippedFile struct {
    Path   string
    Reason string // e.g., "unrecognized format", "magic byte mismatch"
}
```

---

## Task 6 — Path Builder (Naming & Dedup)

**Goal:** Given a date, checksum, extension, and dedup state, produce the deterministic output path.

**Acceptance Criteria:**
- `pathbuilder.Build(date, checksum, ext, isDuplicate, runTimestamp)` returns the relative path.
- Normal: `2021/12/20211225_062223_7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg`
- Duplicate: `duplicates/20260306_103000/2021/12/20211225_062223_7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg`
- Extension is always lowercased.
- Month is non-zero-padded (`2`, not `02`).
- Same-second collision (different checksum, same timestamp) is naturally handled because the checksum differs → different filename.
- Unit tests for normal path, duplicate path, default date (1902/2), extension normalization.

**Function Signature:**
```go
func Build(date time.Time, checksum string, ext string, isDuplicate bool, runTimestamp string) string
```

---

## Task 7 — JPEG Filetype Module

**Goal:** First concrete `FileTypeHandler` implementation. Proves the contract works end-to-end.

**Acceptance Criteria:**
- `Detect`: returns `true` for files with `.jpg`/`.jpeg` extension AND `FF D8 FF` magic bytes at offset 0.
- `ExtractDate`: reads EXIF `DateTimeOriginal`, falls back to `CreateDate`, falls back to `1902-02-20T00:00:00Z`.
- `HashableReader`: returns an `io.Reader` over the JPEG image data (SOS marker `FF DA` through end of image `FF D9`), excluding EXIF/APP markers.
- `WriteMetadataTags`: writes Copyright and CameraOwner EXIF tags into the destination JPEG. No-op if `MetadataTags` is zero-value.
- `Extensions`: returns `[]string{".jpg", ".jpeg"}`.
- `MagicBytes`: returns `[]MagicSignature{{Offset: 0, Bytes: []byte{0xFF, 0xD8, 0xFF}}}`.
- Unit tests with real JPEG fixture files (include 2–3 small test JPEGs in `testdata/`).

**Package Selection Note:** Evaluate `rwcarlsen/goexif` for EXIF read. For EXIF write (tagging), evaluate `dsoprea/go-exif`. Document chosen packages in a comment at the top of the module file.

---

## Task 8 — Copy & Verify Engine

**Goal:** Copy a file from source to destination, then independently re-hash the destination to confirm integrity.

**Acceptance Criteria:**
- `copy.Execute(src, dest string)` streams the file in 32KB chunks. Creates parent directories as needed.
- `copy.Verify(dest string, expectedChecksum string, handler domain.FileTypeHandler, hasher *hash.Hasher)` re-reads the destination via `handler.HashableReader`, hashes it, and compares.
- Returns structured result: `CopyResult{Success bool, Checksum string, Error error}`.
- On verify mismatch, the destination file is **not** deleted (preserved for debugging) but the error is clearly reported.
- Unit tests: successful copy+verify, simulated mismatch (corrupt destination), missing parent directory creation.

**Technical Notes:**
- Use `io.Copy` with a buffered writer for streaming.
- Set destination file permissions to `0644`.
- Preserve original file's modification time on the copy via `os.Chtimes` (informational only — not used for date extraction).

---

## Task 9 — Sort Pipeline Orchestrator (Single-Threaded)

**Goal:** Wire together discovery, extraction, hashing, path building, copy, verify, and manifest updates into a single sequential pipeline. This is the core `sort` logic before concurrency is added.

**Acceptance Criteria:**
- Given `dirA` and `dirB`, the orchestrator:
  1. Initializes or loads the manifest.
  2. Walks `dirA` via discovery.
  3. For each discovered file: extract date → hash → check dedup index → build path → copy → verify → (tag if configured) → update manifest → update dedup index.
  4. After all files: write ledger to `dirA`, finalize manifest.
  5. Print summary: N processed, N duplicates, N skipped, N errors.
- Dedup index: `map[string]string` (checksum → destination path). If checksum already exists, route to `duplicates/`.
- Manifest is saved after every file completes (not batched) for crash safety.
- Dry-run mode: runs through extract + hash + path build but skips copy/verify/tag. Prints what would happen.

**Key Type:**
```go
// SortOptions holds the resolved options for a sort run.
type SortOptions struct {
    Config       *config.AppConfig
    Hasher       *hash.Hasher
    Registry     *discovery.Registry
    RunTimestamp string // e.g., "20260306_103000"
}
```

---

## Task 10 — CLI: `pixe sort` Command

**Goal:** Wire the sort orchestrator into the Cobra `sort` subcommand with full Viper flag binding.

**Acceptance Criteria:**
- `pixe sort --source ./photos --dest ./archive` runs the sort pipeline.
- All flags from Architecture Section 6.1 are bound: `--source`, `--dest`, `--workers`, `--algorithm`, `--copyright`, `--camera-owner`, `--dry-run`.
- Viper merges: CLI flags > config file > defaults.
- Validates required flags (`--source`, `--dest`) and that directories exist (source must exist; dest is created if absent).
- Prints progress to stdout: one line per file processed.
- Exits with code 0 on success, 1 on any errors.
- Manual smoke test: sort a directory of 3 test JPEGs, verify output structure.

---

## Task 11 — Worker Pool & Concurrency

**Goal:** Replace the sequential loop in the orchestrator with a coordinator + worker pool pattern.

**Acceptance Criteria:**
- `--workers N` spawns N goroutines, each pulling files from a shared channel.
- Coordinator goroutine:
  - Feeds discovered files into the work channel.
  - Receives results from a results channel.
  - Updates the manifest (single-writer, no mutex needed on manifest).
  - Maintains the dedup index (single-writer — dedup decisions happen in the coordinator, not workers).
- Workers perform: extract → hash → report back to coordinator → coordinator decides path (dedup check) → worker copies → verifies → tags.
- Graceful shutdown on context cancellation (Ctrl+C).
- Unit test: process 20 files with 4 workers, assert all files appear in manifest.

**Concurrency Design:**
```
                    ┌─────────┐
  discovered ──────>│  work   │──────> worker 1 ──> result ch ──┐
  files       chan  │ channel │──────> worker 2 ──> result ch ──┤
                    │         │──────> worker N ──> result ch ──┤
                    └─────────┘                                 │
                                                                v
                                                         coordinator
                                                        (manifest, dedup)
```

---

## Task 12 — HEIC Filetype Module

**Goal:** Second `FileTypeHandler` — validates that the contract generalizes beyond JPEG.

**Acceptance Criteria:**
- `Detect`: `.heic`/`.heif` extension + `ftyp` box magic bytes (`00 00 00 ?? 66 74 79 70` at offset 0, where `??` is the box size byte).
- `ExtractDate`: HEIC EXIF extraction (HEIC embeds EXIF in an `Exif` item within the ISOBMFF container).
- `HashableReader`: returns reader over the primary image item data (the `mdat` payload for the primary item).
- `WriteMetadataTags`: EXIF Copyright and CameraOwner into the HEIC's EXIF block.
- Unit tests with HEIC fixture files.

**Package Selection Note:** Evaluate `niclas/go-heif` or similar. HEIC is ISOBMFF-based; may share parsing logic with MP4. If no pure-Go HEIC EXIF library exists, consider extracting the EXIF blob from the ISOBMFF container and parsing it with the same EXIF library used for JPEG.

---

## Task 13 — MP4 Filetype Module

**Goal:** Third `FileTypeHandler` — video support with keyframe-based hashing.

**Acceptance Criteria:**
- `Detect`: `.mp4`/`.mov` extension + `ftyp` atom magic bytes.
- `ExtractDate`: QuickTime `mvhd` atom creation date, or `©day` metadata atom.
- `HashableReader`: returns reader that yields the concatenated data of video keyframes (sync samples from `stss` + `stco`/`co64` + `stsz` atoms). This is a subset of `mdat` — not the full video stream.
- `WriteMetadataTags`: write `©cpy` (copyright) and `©own` (camera owner) into the `udta` metadata atom.
- Unit tests with small MP4 fixture files.

**Package Selection Note:** Evaluate `abema/go-mp4` for atom-level parsing. Keyframe extraction requires reading the `stbl` box hierarchy (`stss` for sync sample indices, `stco`/`co64` for chunk offsets, `stsz` for sample sizes).

---

## Task 14 — Metadata Tagging Engine

**Goal:** Template rendering for Copyright and dispatch to the filetype handler's `WriteMetadataTags`.

**Acceptance Criteria:**
- `tagging.RenderCopyright(template string, date time.Time)` returns the rendered string. Uses `text/template` from stdlib.
- Template context: `{{.Year}}` → 4-digit year from the file's capture date.
- `tagging.Apply(destPath string, handler domain.FileTypeHandler, tags domain.MetadataTags)` calls `handler.WriteMetadataTags`.
- If both Copyright and CameraOwner are empty, `Apply` is a no-op (returns nil immediately).
- Unit tests: template rendering with various years, empty template passthrough.

---

## Task 15 — CLI: `pixe verify` Command

**Goal:** Walk a sorted `dirB`, parse checksums from filenames, recompute hashes, report mismatches.

**Acceptance Criteria:**
- `pixe verify --dir ./archive` walks all files in `dirB` (including `duplicates/`).
- Parses filename: extracts checksum between the second `_` and the `.ext`.
- Uses the handler registry to detect each file's type, calls `HashableReader`, hashes, compares.
- Output: one line per file — `OK` or `MISMATCH` with expected vs. actual checksum.
- Summary: N verified, N mismatches, N unrecognized (skipped).
- Exit code: 0 if all OK, 1 if any mismatches.
- Supports `--workers` for concurrent verification.

---

## Task 16 — CLI: `pixe resume` Command

**Goal:** Resume an interrupted sort from the manifest.

**Acceptance Criteria:**
- `pixe resume --dir ./archive` loads `./archive/.pixe/manifest.json`.
- Errors if no manifest found.
- For each entry: if `status == "complete"`, skip. If `status == "copied"`, re-verify. If `status == "pending"` or `"extracted"` or `"hashed"`, re-enter the pipeline from the appropriate stage.
- Re-uses the same `source` directory from the manifest (validates it still exists).
- Updates the manifest in-place as files are re-processed.
- Prints summary on completion.

---

## Task 17 — Integration Tests & Safety Audit

**Goal:** End-to-end tests that exercise the full sort → verify → resume cycle with real fixture files.

**Acceptance Criteria:**
- Test fixture directory with 5+ JPEG files (varied EXIF dates, one with no EXIF, one duplicate).
- **Test: Full sort** — sort fixtures, verify output structure matches expected paths.
- **Test: Verify clean** — run `pixe verify` on sort output, assert 0 mismatches.
- **Test: Duplicate routing** — assert duplicate file lands in `duplicates/<timestamp>/` subtree.
- **Test: No-date fallback** — assert file with no EXIF gets `19020220_000000_` prefix.
- **Test: Resume after interrupt** — sort 5 files, kill after 2 (simulate by truncating manifest), resume, assert all 5 complete.
- **Test: Source untouched** — assert no files in `dirA` were modified (compare checksums of originals before and after sort). Only `.pixe_ledger.json` is new.
- **Test: Dry-run** — assert `--dry-run` creates no files in `dirB`.
- All tests use `t.TempDir()` for isolation.

---

## Task 18 — Makefile & Build Tooling

**Goal:** Provide a Makefile with standard development targets for building, testing, linting, and installing the pixe binary with version metadata injection.

**Acceptance Criteria:**
- `make help` displays all available targets with descriptions.
- `make build` compiles the pixe binary with embedded version, commit, and build date via ldflags.
- `make test` runs unit tests (excludes integration tests).
- `make test-integration` runs integration tests only.
- `make lint` runs golangci-lint.
- `make check` runs fmt-check + vet + unit tests (fast CI gate).
- `make install` builds and installs to `$GOPATH/bin`.
- Version injection: `cmd.Version`, `cmd.Commit`, `cmd.BuildDate` set via `-ldflags -X`.

**Targets Implemented:**
| Target | Description |
|--------|-------------|
| `help` | Show available targets with descriptions |
| `build` | Build pixe binary with ldflags |
| `build-debug` | Build without symbol stripping (for dlv) |
| `run` | Build and run with ARGS |
| `clean` | Remove build artifacts |
| `test` | Alias for test-unit |
| `test-unit` | Run unit tests (excludes integration) |
| `test-integration` | Run integration tests only |
| `test-all` | Run all tests including integration |
| `test-cover` | Run unit tests with coverage report |
| `test-cover-html` | Open HTML coverage report |
| `vet` | Run go vet |
| `fmt` | Format all Go source files |
| `fmt-check` | Check formatting without modifying |
| `lint` | Run golangci-lint |
| `check` | fmt-check + vet + unit tests |
| `tidy` | Run go mod tidy |
| `deps` | Download module dependencies |
| `install` | Build and install to $GOPATH/bin |
| `uninstall` | Remove from $GOPATH/bin |

**Design Decisions:**
- Default goal is `help` for discoverability.
- LDFLAGS inject version info from git tags (`git describe --tags`), commit hash, and build date.
- Test targets exclude integration tests by default using `grep -v '/integration'`.
- Uses `.PHONY` declarations for all targets to avoid filename conflicts.
- Coverage output uses atomic mode for accurate parallel test coverage.

---

## Task 19 — Version Package — Single Source of Truth

**Goal:** Create `internal/version/version.go` as the centralized, importable version package. This is the foundation that all other version-related tasks depend on.

**Architecture Reference:** Section 3 (Version Management)

**File to create: `internal/version/version.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...

// Package version provides the centralized version constant for Pixe.
// This is the single source of truth — update the Version constant here
// when cutting a new release.
package version

import "fmt"

// Version is the semantic version of Pixe (without the "v" prefix).
// Update this value when cutting a new release.
const Version = "0.9.0"

// Commit is the short git SHA, injected at build time via -ldflags.
// Example: go build -ldflags "-X 'github.com/cwlls/pixe-go/internal/version.Commit=abc1234'"
var Commit = "unknown"

// BuildDate is the UTC build timestamp, injected at build time via -ldflags.
// Example: go build -ldflags "-X 'github.com/cwlls/pixe-go/internal/version.BuildDate=2026-03-06T10:30:00Z'"
var BuildDate = "unknown"

// Full returns the human-readable version string.
// Example output: "pixe v0.9.0 (commit: abc1234, built: 2026-03-06T10:30:00Z)"
func Full() string {
    return fmt.Sprintf("pixe v%s (commit: %s, built: %s)", Version, Commit, BuildDate)
}
```

**Acceptance Criteria:**
- `internal/version` package compiles.
- `version.Version` is the string `"0.9.0"`.
- `version.Commit` defaults to `"unknown"` (overridable by ldflags).
- `version.BuildDate` defaults to `"unknown"` (overridable by ldflags).
- `version.Full()` returns `"pixe v0.9.0 (commit: unknown, built: unknown)"` when not built with ldflags.
- The package is importable by any other internal package (`cmd`, `pipeline`, `manifest`, etc.).
- Include the standard Apache 2.0 license header matching the project convention.

**Technical Notes:**
- `Version` is a `const` — it cannot be overridden by ldflags. This is intentional: the Go source file is the single source of truth for the version number.
- `Commit` and `BuildDate` are `var` — they *can* be overridden by ldflags at build time.
- No dependencies beyond `fmt`.

---

## Task 20 — CLI: `pixe version` Command

**Goal:** Add a `pixe version` Cobra subcommand that prints the human-readable version string and exits.

**Architecture Reference:** Section 7.1 (`pixe version`)

**Depends on:** Task 19 (the `internal/version` package must exist)

**File to create: `cmd/version.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
// ...

package cmd

import (
    "fmt"

    "github.com/cwlls/pixe-go/internal/version"
    "github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print the version of Pixe",
    Long:  "Print the version, git commit, and build date of the Pixe binary.",
    Args:  cobra.NoArgs,
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println(version.Full())
    },
}

func init() {
    rootCmd.AddCommand(versionCmd)
}
```

**Acceptance Criteria:**
- `pixe version` prints exactly one line to stdout in the format: `pixe v0.9.0 (commit: <hash>, built: <date>)`
- `pixe version` exits with code 0.
- `pixe version --help` shows the short/long description.
- `pixe version` accepts no arguments — `pixe version foo` returns an error.
- The `version` subcommand appears in `pixe --help` output.
- No flags on this command.
- Include the standard Apache 2.0 license header.

---

## Task 21 — Domain Structs — Add `PixeVersion` Field

**Goal:** Add a `PixeVersion` field to both `domain.Manifest` and `domain.Ledger` so that every manifest and ledger records which version of Pixe produced it.

**Architecture Reference:** Section 3.4 (Consumers), Section 8.1/8.2 (updated JSON schemas)

**Depends on:** Task 19 (the field's value will come from `version.Version`, but the struct change itself only needs to know the field type — `string`)

**File to modify: `internal/domain/pipeline.go`**

Add `PixeVersion string` field to both structs, positioned immediately after the `Version int` field for readability:

```go
// Manifest is the top-level operational journal written to
// dirB/.pixe/manifest.json.
type Manifest struct {
    Version     int              `json:"version"`
    PixeVersion string           `json:"pixe_version"`   // ← NEW
    Source      string           `json:"source"`
    Destination string           `json:"destination"`
    Algorithm   string           `json:"algorithm"`
    StartedAt   time.Time        `json:"started_at"`
    Workers     int              `json:"workers"`
    Files       []*ManifestEntry `json:"files"`
}

// Ledger is the source-side record written to dirA/.pixe_ledger.json.
type Ledger struct {
    Version     int           `json:"version"`
    PixeVersion string        `json:"pixe_version"`   // ← NEW
    PixeRun     time.Time     `json:"pixe_run"`
    Algorithm   string        `json:"algorithm"`
    Destination string        `json:"destination"`
    Files       []LedgerEntry `json:"files"`
}
```

**Acceptance Criteria:**
- `domain.Manifest` has a `PixeVersion string` field with JSON tag `"pixe_version"`.
- `domain.Ledger` has a `PixeVersion string` field with JSON tag `"pixe_version"`.
- `go build ./...` succeeds — no compilation errors from existing code that constructs these structs (struct literal fields are named, so adding a new field is backward-compatible).
- Existing tests in `internal/domain/pipeline_test.go` still pass (they don't reference the new field).
- JSON round-trip: a manifest serialized with `PixeVersion: "0.9.0"` deserializes back with the same value.

**Impact Analysis — Existing struct literals that must be updated (Task 22):**
The following locations construct `Manifest` or `Ledger` with named fields and will need `PixeVersion` added:
1. `internal/pipeline/pipeline.go` line ~88: `m = &domain.Manifest{Version: 1, ...}`
2. `internal/pipeline/pipeline.go` line ~143: `ledger := &domain.Ledger{Version: 1, ...}`
3. `internal/pipeline/worker.go` line ~127: `ledger := &domain.Ledger{Version: 1, ...}`
4. `internal/pipeline/worker.go` line ~384: `ledger := &domain.Ledger{Version: 1, ...}`
5. `internal/manifest/manifest_test.go` line ~31: `sampleManifest()` — `&domain.Manifest{Version: 1, ...}`
6. `internal/manifest/manifest_test.go` line ~53: `sampleLedger()` — `&domain.Ledger{Version: 1, ...}`
7. `internal/integration/integration_test.go` — does not construct Manifest/Ledger directly (uses `pipeline.Run`), so no change needed.

> **Note:** Because Go struct literals use named fields, adding a new field does NOT break compilation. However, the new field will be zero-value (`""`) until Task 22 populates it. Tests in Task 24 will verify the field is populated.

---

## Task 22 — Pipeline — Populate `PixeVersion` at Runtime

**Goal:** Wire `version.Version` into every location that constructs a `domain.Manifest` or `domain.Ledger`, so the version is recorded in the output JSON.

**Architecture Reference:** Section 3.4 (Consumers table)

**Depends on:** Task 19 (`internal/version` package), Task 21 (`PixeVersion` field exists on structs)

**Files to modify:**

### 1. `internal/pipeline/pipeline.go`

Add import:
```go
import "github.com/cwlls/pixe-go/internal/version"
```

At line ~88, where a new manifest is created:
```go
// BEFORE:
m = &domain.Manifest{
    Version:     1,
    Source:      dirA,
    ...
}

// AFTER:
m = &domain.Manifest{
    Version:     1,
    PixeVersion: version.Version,   // ← ADD
    Source:      dirA,
    ...
}
```

At line ~143, where the ledger is created:
```go
// BEFORE:
ledger := &domain.Ledger{
    Version:     1,
    PixeRun:     m.StartedAt,
    ...
}

// AFTER:
ledger := &domain.Ledger{
    Version:     1,
    PixeVersion: version.Version,   // ← ADD
    PixeRun:     m.StartedAt,
    ...
}
```

### 2. `internal/pipeline/worker.go`

Add import:
```go
import "github.com/cwlls/pixe-go/internal/version"
```

At line ~127 (`RunConcurrent` ledger creation):
```go
// BEFORE:
ledger := &domain.Ledger{
    Version:     1,
    PixeRun:     m.StartedAt,
    ...
}

// AFTER:
ledger := &domain.Ledger{
    Version:     1,
    PixeVersion: version.Version,   // ← ADD
    PixeRun:     m.StartedAt,
    ...
}
```

At line ~384 (`runSequential` ledger creation):
```go
// BEFORE:
ledger := &domain.Ledger{
    Version:     1,
    PixeRun:     m.StartedAt,
    ...
}

// AFTER:
ledger := &domain.Ledger{
    Version:     1,
    PixeVersion: version.Version,   // ← ADD
    PixeRun:     m.StartedAt,
    ...
}
```

**Acceptance Criteria:**
- After a `pixe sort` run, `dirB/.pixe/manifest.json` contains `"pixe_version": "0.9.0"`.
- After a `pixe sort` run, `dirA/.pixe_ledger.json` contains `"pixe_version": "0.9.0"`.
- The `pixe_version` field appears immediately after the `version` field in the JSON output (Go's `encoding/json` serializes struct fields in declaration order).
- All existing tests pass — the new field is additive and does not change any existing behavior.
- `go build ./...` succeeds.

---

## Task 23 — Makefile — Retarget ldflags to `internal/version`

**Goal:** Update the Makefile's LDFLAGS to inject `Commit` and `BuildDate` into `internal/version` instead of the non-existent `cmd` variables. Remove the `Version` ldflags override since the Go const is now authoritative.

**Architecture Reference:** Section 3.2 (Build-Time Metadata)

**Depends on:** Task 19 (`internal/version` package with `var Commit` and `var BuildDate`)

**File to modify: `Makefile`**

```makefile
# BEFORE (lines 17-20):
LDFLAGS     := -s -w \
               -X '$(MODULE)/cmd.Version=$(VERSION)' \
               -X '$(MODULE)/cmd.Commit=$(COMMIT)' \
               -X '$(MODULE)/cmd.BuildDate=$(BUILD_DATE)'

# AFTER:
LDFLAGS     := -s -w \
               -X '$(MODULE)/internal/version.Commit=$(COMMIT)' \
               -X '$(MODULE)/internal/version.BuildDate=$(BUILD_DATE)'
```

Also remove the `VERSION` variable (line 13) since it is no longer injected:

```makefile
# BEFORE (line 13):
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# AFTER: (remove this line entirely)
```

**Acceptance Criteria:**
- `make build` succeeds.
- `./pixe version` prints `pixe v0.9.0 (commit: <actual-short-sha>, built: <actual-utc-timestamp>)`.
- The `Commit` value matches `git rev-parse --short HEAD`.
- The `BuildDate` value is a valid UTC timestamp.
- `go build ./...` (without ldflags) still works — `./pixe version` prints `pixe v0.9.0 (commit: unknown, built: unknown)`.
- No references to `cmd.Version`, `cmd.Commit`, or `cmd.BuildDate` remain in the Makefile.

---

## Task 24 — Tests & Verification

**Goal:** Add unit tests for the new version package, update existing manifest tests to cover the `PixeVersion` field, and verify the entire test suite passes.

**Architecture Reference:** Section 3 (Version Management)

**Depends on:** Tasks 19, 20, 21, 22, 23 (all implementation tasks complete)

### 1. New file: `internal/version/version_test.go`

```go
package version

import (
    "strings"
    "testing"
)

func TestVersion_isSet(t *testing.T) {
    if Version == "" {
        t.Error("Version constant must not be empty")
    }
}

func TestVersion_semverFormat(t *testing.T) {
    // Must be MAJOR.MINOR.PATCH — no "v" prefix.
    parts := strings.Split(Version, ".")
    if len(parts) != 3 {
        t.Errorf("Version %q is not in MAJOR.MINOR.PATCH format", Version)
    }
    if strings.HasPrefix(Version, "v") {
        t.Errorf("Version %q should not have a 'v' prefix", Version)
    }
}

func TestFull_format(t *testing.T) {
    s := Full()
    // Must start with "pixe v"
    if !strings.HasPrefix(s, "pixe v") {
        t.Errorf("Full() = %q, want prefix 'pixe v'", s)
    }
    // Must contain the version constant
    if !strings.Contains(s, Version) {
        t.Errorf("Full() = %q, does not contain Version %q", s, Version)
    }
    // Must contain commit and built labels
    if !strings.Contains(s, "commit:") {
        t.Errorf("Full() = %q, missing 'commit:' label", s)
    }
    if !strings.Contains(s, "built:") {
        t.Errorf("Full() = %q, missing 'built:' label", s)
    }
}

func TestFull_defaultValues(t *testing.T) {
    // Without ldflags, Commit and BuildDate should be "unknown".
    s := Full()
    if !strings.Contains(s, "unknown") {
        t.Logf("Full() = %q — Commit=%q BuildDate=%q", s, Commit, BuildDate)
        // This is not a hard failure because ldflags may have been set.
        // But in a normal `go test` run, they should be "unknown".
    }
}
```

### 2. Update: `internal/manifest/manifest_test.go`

Update `sampleManifest()` and `sampleLedger()` to include the `PixeVersion` field, and add assertions:

In `sampleManifest()` (~line 31):
```go
// ADD after Version: 1,
PixeVersion: "0.9.0",
```

In `sampleLedger()` (~line 53):
```go
// ADD after Version: 1,
PixeVersion: "0.9.0",
```

Add a new test or extend `TestManifest_SaveLoad_roundtrip` to assert:
```go
if got.PixeVersion != m.PixeVersion {
    t.Errorf("PixeVersion: got %q, want %q", got.PixeVersion, m.PixeVersion)
}
```

Add a similar assertion to `TestLedger_SaveLoad_roundtrip`:
```go
if got.PixeVersion != l.PixeVersion {
    t.Errorf("PixeVersion: got %q, want %q", got.PixeVersion, l.PixeVersion)
}
```

### 3. Verification commands

After all implementation tasks are complete, run:

```bash
go vet ./...                                    # No warnings
go build ./...                                  # Compiles cleanly
go test -race -timeout 120s ./...               # All tests pass (unit + integration)
make build && ./pixe version                    # Prints version with real commit/date
```

**Acceptance Criteria:**
- `internal/version/version_test.go` exists and all tests pass.
- `TestVersion_isSet` confirms the constant is non-empty.
- `TestVersion_semverFormat` confirms `MAJOR.MINOR.PATCH` format without `v` prefix.
- `TestFull_format` confirms the output string structure.
- Manifest round-trip test asserts `PixeVersion` survives save/load.
- Ledger round-trip test asserts `PixeVersion` survives save/load.
- `go vet ./...` reports no issues.
- `go test -race -timeout 120s ./...` passes (all unit + integration tests).
- `make build && ./pixe version` prints the expected format with real git metadata.

---

## Task 25 — Lint Fixes

**Goal:** Resolve all golangci-lint violations so `make lint` exits 0.

**Changes made:**
- `internal/copy/copy.go`: wrapped `defer f/rc.Close()` with `_ =` and `out.Close()` error path
- `internal/discovery/registry.go`: wrapped `defer f.Close()`
- `internal/handler/heic/heic.go` + `heic_test.go`: wrapped all `defer f/rc.Close()`
- `internal/handler/jpeg/jpeg.go` + `jpeg_test.go`: wrapped all `defer f/rc.Close()`
- `internal/handler/mp4/mp4.go` + `mp4_test.go`: wrapped all `defer f/rc.Close()`
- `internal/verify/verify.go`: `_, _ = fmt.Fprintf(...)` for all output writes; `_ = rc.Close()`
- `internal/hash/hasher_test.go`: removed unused `knownDigests` var
- `internal/manifest/manifest.go`: removed unused `manifestVersion` and `ledgerVersion` consts
- `internal/pipeline/worker.go`: removed unused `workerContext` type; `_, _ = fmt.Fprintf(...)` for all output writes; `_ = rc.Close()`
- `internal/pipeline/pipeline.go`: `_, _ = fmt.Fprintf(...)` for all output writes; `_ = rc.Close()`
- `cmd/resume.go`: `_, _ = fmt.Fprintf(...)` for stdout write
- `internal/integration/integration_test.go`: wrapped `defer f.Close()`

**Result:** `make lint` → `0 issues.` | `go test ./internal/...` → all 13 packages pass.
