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
| 19 | Version Package — Single Source of Truth | High | @developer | ⬜ Superseded | — | Superseded by Tasks 44–49 (idiomatic ldflags approach) |
| 20 | CLI: `pixe version` Command | High | @developer | ⬜ Superseded | 19 | Superseded by Task 44 (version vars + command collapsed into `cmd`) |
| 21 | Domain Structs — Add `PixeVersion` Field | High | @developer | ✅ Complete | 19 | Add field to `Manifest` and `Ledger` in `internal/domain/pipeline.go` |
| 22 | Pipeline — Populate `PixeVersion` at Runtime | High | @developer | ⬜ Superseded | 19, 21 | Superseded by Task 46 (pipeline reads `cmd.Version()` instead of `version.Version`) |
| 23 | Makefile — Retarget ldflags to `internal/version` | Medium | @developer | ⬜ Superseded | 19 | Superseded by Task 47 (Makefile delegates to GoReleaser) |
| 24 | Tests & Verification | High | @tester | ⬜ Superseded | 19, 20, 21, 22, 23 | Superseded by Task 49 (version tests removed; verification via build smoke test) |
| 25 | Lint Fixes — golangci-lint 0 issues | High | @developer | ✅ Complete | 1–24 | Fixed 30+ errcheck and unused lint violations across copy, discovery, heic, jpeg, mp4, verify, hash, manifest, pipeline packages; installed golangci-lint |
| 26 | Locale-Aware Month Directory — `pathbuilder` rewrite | High | @developer |  ✅ Complete | 6 | Change month dir from `2` to `02-Feb` (locale-aware); add `MonthDir()` helper |
| 27 | Update Tests — Month Directory Format | High | @developer |  ✅ Complete | 26 | Rewrite pathbuilder, pipeline, and integration tests for `MM-Mon` format |
| 28 | Tests & Verification — Full Suite Green | High | @tester |  ✅ Complete | 26, 27 | `go vet`, `go test -race ./...`, `make lint` all pass |
| 29 | Archive DB — `internal/archivedb` package & schema | High | @developer | 🔲 Pending | 2 | SQLite database layer: Open, Close, schema creation, WAL mode, busy timeout |
| 30 | Archive DB — Run & File CRUD operations | High | @developer | 🔲 Pending | 29 | InsertRun, UpdateRun, InsertFile, UpdateFile, dedup query, batch insert |
| 31 | Archive DB — Query methods | Medium | @developer | 🔲 Pending | 30 | Query families: by source, date range, run, status, checksum, duplicates |
| 32 | DB Location Resolver — `internal/dblocator` package | High | @developer | 🔲 Pending | 29 | Priority chain: --db-path → dbpath marker → local default; network mount detection; slug generation |
| 33 | Domain Types — SQLite-era updates | High | @developer | 🔲 Pending | 2, 29 | Add `RunID` to Ledger, bump ledger version to 2, add `DBPath` to AppConfig |
| 34 | JSON Manifest Migration — `internal/migrate` package | High | @developer | 🔲 Pending | 29, 30 | Auto-detect manifest.json, create synthetic run, import entries, rename to .migrated |
| 35 | Pipeline Refactor — Replace JSON manifest with archive DB | High | @developer | 🔲 Pending | 29, 30, 32, 33 | Rewrite pipeline.go and worker.go to use archivedb instead of manifest.Save/Load |
| 36 | Pipeline — Cross-process dedup race handling | Medium | @developer | 🔲 Pending | 35 | Post-commit dedup re-check, retroactive duplicate routing |
| 37 | CLI Updates — `--db-path` flag & resume rewrite | High | @developer | 🔲 Pending | 32, 35 | Add --db-path to sort/resume, update resume to use DB discovery chain |
| 38 | Ledger Update — Add `run_id` field | Medium | @developer | 🔲 Pending | 33, 35 | Wire run UUID into ledger creation, bump version to 2 |
| 39 | Archive DB — Unit tests | High | @tester | 🔲 Pending | 29, 30, 31 | Schema creation, CRUD, queries, WAL concurrency, busy retry |
| 40 | DB Locator — Unit tests | High | @tester | 🔲 Pending | 32 | Local/network detection, slug generation, dbpath marker read/write |
| 41 | Migration — Unit tests | High | @tester | 🔲 Pending | 34 | JSON→SQLite migration, idempotency, synthetic run correctness |
| 42 | Integration Tests — SQLite pipeline end-to-end | High | @tester | 🔲 Pending | 35, 36, 37, 38 | Full sort→verify→resume cycle using DB, concurrent run simulation |
| 43 | Tests & Verification — Full Suite Green | High | @tester | 🔲 Pending | 39, 40, 41, 42 | `go vet`, `go test -race ./...`, `make lint` all pass |
| 44 | Version Vars & Command — Collapse into `cmd` | High | @developer | ✅ Complete | — | Move version vars + `fullVersion()` + `Version()` getter + `init()` into `cmd/version.go`; rewrite `pixe version` command |
| 45 | Delete `internal/version` Package | High | @developer | ✅ Complete | 44, 46 | Remove `internal/version/version.go` and `version_test.go`; remove stale import from any file |
| 46 | Pipeline — Switch to `cmd.Version()` | High | @developer | ✅ Complete | 44 | Replace `version.Version` with `cmd.Version()` in `pipeline.go` and `worker.go` |
| 47 | Makefile — Delegate to GoReleaser | High | @developer | ✅ Complete | 44 | Rewrite `build`/`install` targets to use `goreleaser build --single-target --snapshot`; keep `build-debug` as raw `go build` |
| 48 | GoReleaser — Fix ldflags Target | High | @developer | ✅ Complete | 44 | Retarget ldflags from `internal/version.*` to `cmd.version`, `cmd.commit`, `cmd.buildDate` |
| 49 | Tests & Verification — Version Refactor | High | @tester | ✅ Complete | 44, 45, 46, 47, 48 | Delete version_test.go; update manifest test fixtures; `go vet`, `go test -race ./...`, `make build && ./pixe version` |

---

## Milestone: Tasks 1–18 Complete

All 18 original tasks have been completed. The pixe-go photo organization tool is fully functional with support for sorting, verifying, and resuming operations across JPEG, HEIC, and MP4 file types.

## Feature: Centralized Version Management (Tasks 19–24) — Superseded

~~Adds a single-source-of-truth version package, a `pixe version` CLI command, and embeds the Pixe version into manifests and ledgers.~~

Tasks 19–24 are superseded by Tasks 44–49. The `internal/version` package is being replaced with idiomatic Go build-time injection via ldflags, with version variables collapsed into the `cmd` package. See Architecture Section 3 for the updated design.

**What is preserved from Tasks 19–24:**
- Task 21 (`PixeVersion` field on `Manifest` and `Ledger`) — remains as-is; the field still exists and is populated.
- The `pixe version` CLI command — rewritten in-place in Task 44.
- Manifest/ledger round-trip tests for `PixeVersion` — test fixtures updated to use `"dev"` instead of `"0.9.0"`.

## Feature: Idiomatic Version Management (Tasks 44–49) — Complete

Replaces the `internal/version` package with the standard Go pattern of build-time ldflags injection. The git tag becomes the sole source of truth for the version string. Version variables are collapsed into the `cmd` package. The Makefile delegates to GoReleaser for builds, eliminating ldflags drift. See Architecture Section 3.

All 6 tasks complete. The `internal/version` package has been deleted and replaced with idiomatic Go build-time ldflags injection. Version variables (`version`, `commit`, `buildDate`) live in `cmd/version.go` as unexported vars with an exported `Version()` getter. The Makefile delegates all binary compilation to GoReleaser. The goreleaser ldflags bug (injecting into a `const`) is fixed. Dev builds produce `pixe vdev-<commit>` when built via the Makefile. All 13 test packages pass.

**Design decisions:**
1. **Git tag is the source of truth** — no version literal in Go source code.
2. **Dev builds show `dev-<commit>`** — `init()` enriches the version string with the commit hash when available.
3. **`internal/version` package is deleted** — version vars live in `cmd/version.go` (unexported) with an exported `Version()` getter.
4. **Makefile delegates to GoReleaser** — `make build` runs `goreleaser build --single-target --snapshot`.
5. **Version tests are removed** — no source literal to test; correctness is verified by build smoke test.
6. **`PixeVersion` field on domain structs is unchanged** — pipeline reads `cmd.Version()` instead of `version.Version`.

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

> **⚠️ Superseded by Task 26.** The month directory format changed from bare integer (`2`) to locale-aware `MM-Mon` (`02-Feb`). See Task 26 for the updated spec.

**Goal:** Given a date, checksum, extension, and dedup state, produce the deterministic output path.

**Acceptance Criteria (original — see Task 26 for current):**
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

---

## Feature: Locale-Aware Month Directories (Tasks 26–28)

Changes the month subdirectory format from a bare non-zero-padded integer (e.g. `2`) to a zero-padded number + hyphen + locale-aware three-letter title-cased month abbreviation (e.g. `02-Feb`). See Architecture Section 4.3.

**Design decisions captured from user:**
1. Month abbreviation is always **title-cased** (e.g. `Jan`, `Feb`, `Mar`).
2. Separator is a **hyphen** (`03-Mar`).
3. Duplicate paths use the **same** `MM-Mon` format.
4. **Filename is unchanged** — `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` retains its zero-padded numeric month.
5. This is a **go-forward** change — no migration of existing archives.
6. Month abbreviation uses the **user's system locale** (not hardcoded English).
7. Year directory is **unchanged** (plain 4-digit number).

---

## Task 26 — Locale-Aware Month Directory — `pathbuilder` Rewrite

**Goal:** Change the `pathbuilder.Build()` function so the month directory component uses the format `MM-Mon` (zero-padded month number, hyphen, locale-aware three-letter title-cased month abbreviation) instead of the current bare integer.

**Architecture Reference:** Section 4.3 (Output Naming Convention), Section 4.4 (Duplicate Handling)

**Depends on:** Task 6 (existing pathbuilder)

### Files to modify

#### 1. `internal/pathbuilder/pathbuilder.go` — Core logic change

**Package doc comment** — update the path pattern and description:

```go
// Package pathbuilder constructs deterministic output paths for sorted media
// files using the Pixe naming convention:
//
//	<YYYY>/<MM>-<Mon>/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
//
// Duplicate files are routed under a timestamped subdirectory:
//
//	duplicates/<runTimestamp>/<YYYY>/<MM>-<Mon>/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
//
// The month directory is a zero-padded two-digit number, a hyphen, and the
// locale-aware three-letter title-cased month abbreviation (e.g. "03-Mar").
// The abbreviation is derived from the user's system locale. The file
// extension is always lowercased.
package pathbuilder
```

**New import:** `golang.org/x/text/language` (already an indirect dependency — promote to direct).

**New helper function — `MonthDir`:**

```go
// MonthDir returns the locale-aware month directory name for the given month.
// Format: zero-padded two-digit number + hyphen + three-letter title-cased
// month abbreviation. Examples (English locale): "01-Jan", "02-Feb", "12-Dec".
//
// The abbreviation is derived from the system locale detected at init time.
// If the locale cannot be determined, English is used as the fallback.
func MonthDir(month time.Month) string {
    abbr := localizedMonthAbbr(month)
    return fmt.Sprintf("%02d-%s", int(month), abbr)
}
```

**Locale detection strategy:**

The package needs a module-level variable holding the resolved `language.Tag` for the user's system locale. Detect at package init time by reading environment variables in standard precedence order:

```go
import (
    "os"
    "strings"

    "golang.org/x/text/language"
)

// systemLocale is the resolved locale tag, detected once at package init.
var systemLocale language.Tag

func init() {
    systemLocale = detectSystemLocale()
}

// detectSystemLocale reads LANGUAGE, LC_ALL, LC_TIME, or LANG from the
// environment and parses the first valid BCP 47 / POSIX locale tag.
// Falls back to language.English if nothing is set or parseable.
func detectSystemLocale() language.Tag {
    for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_TIME", "LANG"} {
        val := os.Getenv(key)
        if val == "" || val == "C" || val == "POSIX" {
            continue
        }
        // POSIX locales use underscores (e.g. "fr_FR.UTF-8"); strip encoding suffix.
        val = strings.SplitN(val, ".", 2)[0]
        val = strings.ReplaceAll(val, "_", "-")
        tag, err := language.Parse(val)
        if err == nil {
            return tag
        }
    }
    return language.English
}
```

**Localized month abbreviation:**

Use Go's CLDR-based `golang.org/x/text` packages to get the abbreviated month name for the detected locale. The `golang.org/x/text/date` package is experimental, so the recommended approach is to use a lookup table seeded from CLDR data, or use the `time` package's `Month.String()` and truncate to 3 characters as a baseline, then layer locale support on top.

**Practical approach — use `golang.org/x/text/language` + `golang.org/x/text/message` for locale detection, and a CLDR-derived month table:**

Since `golang.org/x/text` does not expose a simple "give me abbreviated month names for locale X" API, the cleanest approach is:

```go
// localizedMonthAbbr returns the three-letter title-cased abbreviated month
// name for the given month in the system locale.
func localizedMonthAbbr(month time.Month) string {
    // Use the system locale's abbreviated month names if available.
    // The golang.org/x/text ecosystem does not provide a direct API for
    // locale-aware month abbreviations, so we use time.Month.String()
    // truncated to 3 characters. Go's time package always returns English
    // month names, but this gives us the correct title-cased 3-letter form
    // for English. For non-English locales, a lookup table can be added.
    //
    // For now, detect the base language and use a built-in CLDR-derived
    // table for supported languages. Fall back to English for unsupported.
    base, _ := systemLocale.Base()
    if table, ok := monthAbbreviations[base.String()]; ok {
        if int(month) >= 1 && int(month) <= 12 {
            return table[month-1]
        }
    }
    // Fallback: English via time.Month.String() truncated to 3 chars.
    s := month.String()
    if len(s) > 3 {
        s = s[:3]
    }
    return s
}

// monthAbbreviations maps BCP 47 base language codes to their 12 abbreviated
// month names (title-cased, 3 letters). Sourced from Unicode CLDR.
// Add entries here to support additional locales.
var monthAbbreviations = map[string][12]string{
    "en": {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
    "fr": {"Jan", "Fév", "Mar", "Avr", "Mai", "Jun", "Jul", "Aoû", "Sep", "Oct", "Nov", "Déc"},
    "de": {"Jan", "Feb", "Mär", "Apr", "Mai", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dez"},
    "es": {"Ene", "Feb", "Mar", "Abr", "May", "Jun", "Jul", "Ago", "Sep", "Oct", "Nov", "Dic"},
    "it": {"Gen", "Feb", "Mar", "Apr", "Mag", "Giu", "Lug", "Ago", "Set", "Ott", "Nov", "Dic"},
    "pt": {"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"},
    "nl": {"Jan", "Feb", "Mrt", "Apr", "Mei", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dec"},
    "ja": {"1月",  "2月",  "3月",  "4月",  "5月",  "6月",  "7月",  "8月",  "9月",  "10月", "11月", "12月"},
    "zh": {"1月",  "2月",  "3月",  "4月",  "5月",  "6月",  "7月",  "8月",  "9月",  "10月", "11月", "12月"},
    "ko": {"1월",  "2월",  "3월",  "4월",  "5월",  "6월",  "7월",  "8월",  "9월",  "10월", "11월", "12월"},
    "ru": {"Янв", "Фев", "Мар", "Апр", "Май", "Июн", "Июл", "Авг", "Сен", "Окт", "Ноя", "Дек"},
}
```

> **Developer note:** The table above is a starting set. The key design point is that the table is extensible — adding a new locale is a one-line addition. For locales not in the table, English is the fallback. The abbreviations must be title-cased per the user's requirement.

**Update `Build()` function:**

Replace the month directory formatting on line 65:

```go
// BEFORE:
relPath := filepath.Join(fmt.Sprintf("%d", year), fmt.Sprintf("%d", month), filename)

// AFTER:
relPath := filepath.Join(fmt.Sprintf("%d", year), MonthDir(date.Month()), filename)
```

Also update the `Build()` doc comment examples:

```go
// Example outputs:
//
//	Build(t, sha, ".jpg", false, "") → "2021/12-Dec/20211225_062223_<sha>.jpg"
//	Build(t, sha, ".JPG", true, "20260306_103000") → "duplicates/20260306_103000/2021/12-Dec/20211225_062223_<sha>.jpg"
```

Remove the old comment `// Non-zero-padded month per spec.` and replace with `// Locale-aware month directory per spec (Section 4.3).`

**Exported `SetLocale` for testing:**

To make tests deterministic regardless of the host's actual locale, expose a setter:

```go
// SetLocaleForTesting overrides the detected system locale. This is intended
// for use in tests only — it is not safe for concurrent use.
func SetLocaleForTesting(tag language.Tag) {
    systemLocale = tag
}
```

### Dependency change

Promote `golang.org/x/text` from indirect to direct in `go.mod`:

```bash
go get golang.org/x/text/language
```

This should move the `// indirect` comment. No new dependency is introduced — `golang.org/x/text` is already in `go.sum`.

### Acceptance Criteria

- `pathbuilder.Build(date(2021,12,25,...), sha, ".jpg", false, "")` returns `"2021/12-Dec/20211225_062223_<sha>.jpg"` (on English locale).
- `pathbuilder.Build(date(1902,2,20,...), sha, ".jpg", false, "")` returns `"1902/02-Feb/19020220_000000_<sha>.jpg"` (on English locale).
- `pathbuilder.Build(date(2022,3,1,...), sha, ".jpg", true, "20260306_103000")` returns `"duplicates/20260306_103000/2022/03-Mar/20220301_100000_<sha>.jpg"`.
- `pathbuilder.MonthDir(time.January)` returns `"01-Jan"` on English locale.
- `pathbuilder.MonthDir(time.December)` returns `"12-Dec"` on English locale.
- Month in the **filename** is still zero-padded numeric (`02`, not `Feb`).
- Year directory is unchanged (`"2021"`, not `"2021-Twenty-One"`).
- `RunTimestamp()` is unchanged.
- `go build ./...` succeeds.
- The `Build()` function signature is unchanged — no callers need modification.

---

## Task 27 — Update Tests — Month Directory Format

**Goal:** Rewrite all tests that assert month directory paths to use the new `MM-Mon` format. No test should reference the old bare-integer month directory.

**Architecture Reference:** Section 4.3

**Depends on:** Task 26

### Files to modify

#### 1. `internal/pathbuilder/pathbuilder_test.go` — Unit tests

**Rewrite `TestBuild_normalPath`:**

```go
func TestBuild_normalPath(t *testing.T) {
    d := date(2021, 12, 25, 6, 22, 23)
    got := Build(d, testChecksum, ".jpg", false, "")
    want := filepath.Join("2021", "12-Dec", "20211225_062223_"+testChecksum+".jpg")
    if got != want {
        t.Errorf("Build normal:\n  got  %q\n  want %q", got, want)
    }
}
```

**Rewrite `TestBuild_duplicatePath`:**

```go
func TestBuild_duplicatePath(t *testing.T) {
    d := date(2021, 12, 25, 6, 22, 23)
    got := Build(d, testChecksum, ".jpg", true, "20260306_103000")
    want := filepath.Join("duplicates", "20260306_103000", "2021", "12-Dec", "20211225_062223_"+testChecksum+".jpg")
    if got != want {
        t.Errorf("Build duplicate:\n  got  %q\n  want %q", got, want)
    }
}
```

**Rewrite `TestBuild_defaultDate_anselsAdams`:**

```go
func TestBuild_defaultDate_anselsAdams(t *testing.T) {
    d := date(1902, 2, 20, 0, 0, 0)
    got := Build(d, testChecksum, ".jpg", false, "")
    want := filepath.Join("1902", "02-Feb", "19020220_000000_"+testChecksum+".jpg")
    if got != want {
        t.Errorf("Build Ansel Adams date:\n  got  %q\n  want %q", got, want)
    }
}
```

**Rename and rewrite `TestBuild_monthNotZeroPadded` → `TestBuild_monthDirectoryFormat`:**

This test now validates the `MM-Mon` format for the directory and confirms the filename still uses zero-padded numeric months:

```go
func TestBuild_monthDirectoryFormat(t *testing.T) {
    // Ensure English locale for deterministic test output.
    SetLocaleForTesting(language.English)

    cases := []struct {
        month          int
        wantDir        string // MM-Mon format
        wantInFilename string // zero-padded numeric
    }{
        {1, "01-Jan", "01"},
        {2, "02-Feb", "02"},
        {9, "09-Sep", "09"},
        {10, "10-Oct", "10"},
        {12, "12-Dec", "12"},
    }
    for _, tc := range cases {
        d := date(2022, tc.month, 5, 0, 0, 0)
        got := Build(d, testChecksum, ".jpg", false, "")
        parts := splitPath(got)
        if len(parts) < 2 {
            t.Fatalf("unexpected path structure: %q", got)
        }
        // Directory component should be MM-Mon.
        if parts[1] != tc.wantDir {
            t.Errorf("month %d: directory = %q, want %q", tc.month, parts[1], tc.wantDir)
        }
        // Month in filename is zero-padded numeric (part of YYYYMMDD).
        filename := parts[len(parts)-1]
        monthInFilename := filename[4:6]
        if monthInFilename != tc.wantInFilename {
            t.Errorf("month %d: filename month digits = %q, want %q", tc.month, monthInFilename, tc.wantInFilename)
        }
    }
}
```

**Add new test — `TestMonthDir`:**

```go
func TestMonthDir(t *testing.T) {
    SetLocaleForTesting(language.English)

    cases := []struct {
        month time.Month
        want  string
    }{
        {time.January, "01-Jan"},
        {time.February, "02-Feb"},
        {time.March, "03-Mar"},
        {time.September, "09-Sep"},
        {time.October, "10-Oct"},
        {time.December, "12-Dec"},
    }
    for _, tc := range cases {
        got := MonthDir(tc.month)
        if got != tc.want {
            t.Errorf("MonthDir(%v) = %q, want %q", tc.month, got, tc.want)
        }
    }
}
```

**Add new test — `TestMonthDir_nonEnglishLocale`:**

```go
func TestMonthDir_nonEnglishLocale(t *testing.T) {
    SetLocaleForTesting(language.French)

    // French abbreviated months from CLDR.
    got := MonthDir(time.March)
    if got != "03-Mar" {
        t.Errorf("MonthDir(March) with French locale = %q, want %q", got, "03-Mar")
    }

    got = MonthDir(time.February)
    if got != "02-Fév" {
        t.Errorf("MonthDir(February) with French locale = %q, want %q", got, "02-Fév")
    }

    // Restore English for other tests.
    SetLocaleForTesting(language.English)
}
```

**Add new test — `TestDetectSystemLocale_fallback`:**

```go
func TestDetectSystemLocale_fallback(t *testing.T) {
    // When no locale env vars are set, should fall back to English.
    // Save and clear env vars.
    keys := []string{"LANGUAGE", "LC_ALL", "LC_TIME", "LANG"}
    saved := make(map[string]string)
    for _, k := range keys {
        saved[k] = os.Getenv(k)
        os.Unsetenv(k)
    }
    defer func() {
        for k, v := range saved {
            if v != "" {
                os.Setenv(k, v)
            }
        }
    }()

    tag := detectSystemLocale()
    base, _ := tag.Base()
    if base.String() != "en" {
        t.Errorf("detectSystemLocale() with no env = %v, want English", tag)
    }
}
```

**Update `TestBuild_sameSecondDifferentChecksum`:**

The month directory for March changes from `"3"` to `"03-Mar"`. The test logic (asserting p1 != p2) is still valid — no structural change needed, but verify it still passes.

**Add import for `golang.org/x/text/language`** to the test file (needed for `SetLocaleForTesting` and `language.English`/`language.French`).

#### 2. `internal/pipeline/pipeline_test.go` — Pipeline tests

**`TestRun_outputDirectoryStructure` (line 106–131):**

Change the month directory assertion:

```go
// BEFORE:
monthDir := filepath.Join(dirB, "2021", "12")

// AFTER:
monthDir := filepath.Join(dirB, "2021", "12-Dec")
```

**`TestRun_noExifFallbackDate` (line 149–161):**

Change the month directory assertion:

```go
// BEFORE:
monthDir := filepath.Join(dirB, "1902", "2")

// AFTER:
monthDir := filepath.Join(dirB, "1902", "02-Feb")
```

Also update the error message string from `"1902/2/"` to `"1902/02-Feb/"`.

#### 3. `internal/integration/integration_test.go` — Integration tests

**`TestIntegration_FullSort` (line 146):**

```go
// BEFORE:
files2021 := findFiles(t, filepath.Join(dirB, "2021", "12"), "20211225_062223_")

// AFTER:
files2021 := findFiles(t, filepath.Join(dirB, "2021", "12-Dec"), "20211225_062223_")
```

Update the error message on line 148 from `"2021/12/"` to `"2021/12-Dec/"`.

**`TestIntegration_NoDateFallback` (line 235–249):**

The test currently checks for `"1902"` as a path component. It should also check for `"02-Feb"`:

```go
// BEFORE (line 241):
if p == "1902" {

// AFTER — also verify the month directory:
// Add a second check for the month component:
found02Feb := false
for _, p := range parts {
    if p == "02-Feb" {
        found02Feb = true
        break
    }
}
if !found02Feb {
    t.Errorf("path %q does not contain 02-Feb directory", rel)
}
```

#### 4. Add `init()` or `TestMain` to set English locale in test files

To ensure tests are deterministic regardless of the developer's system locale, add at the top of each test file that asserts specific month names:

**`internal/pathbuilder/pathbuilder_test.go`:**
```go
func TestMain(m *testing.M) {
    SetLocaleForTesting(language.English)
    os.Exit(m.Run())
}
```

**`internal/pipeline/pipeline_test.go`** and **`internal/integration/integration_test.go`:**
```go
import "github.com/cwlls/pixe-go/internal/pathbuilder"

func TestMain(m *testing.M) {
    pathbuilder.SetLocaleForTesting(language.English)
    os.Exit(m.Run())
}
```

### Acceptance Criteria

- All pathbuilder unit tests pass with the new `MM-Mon` format.
- `TestBuild_monthDirectoryFormat` validates months 1, 2, 9, 10, 12 produce `01-Jan`, `02-Feb`, `09-Sep`, `10-Oct`, `12-Dec` directories.
- `TestMonthDir` validates the exported helper directly.
- `TestMonthDir_nonEnglishLocale` proves French locale produces French abbreviations.
- `TestDetectSystemLocale_fallback` proves English fallback when no env vars are set.
- Pipeline tests (`TestRun_outputDirectoryStructure`, `TestRun_noExifFallbackDate`) pass with updated directory assertions.
- Integration tests (`TestIntegration_FullSort`, `TestIntegration_NoDateFallback`) pass with updated directory assertions.
- No test references the old bare-integer month directory format.
- All tests are locale-deterministic via `SetLocaleForTesting(language.English)` in `TestMain`.

---

## Task 28 — Tests & Verification — Full Suite Green

**Goal:** Verify the entire codebase compiles, passes all tests, and passes lint after the month directory format change.

**Depends on:** Tasks 26, 27

### Verification commands

```bash
go vet ./...                                    # No warnings
go build ./...                                  # Compiles cleanly
go test -race -timeout 120s ./...               # All tests pass (unit + integration)
make lint                                       # 0 issues
```

### Specific checks

1. **No stale references:** Grep the entire codebase for the old month format patterns:
   ```bash
   # Should return zero matches in .go files (excluding STATE.md):
   rg 'Sprintf\("%d", month\)' --include '*.go'
   rg 'non-zero-padded' --include '*.go'
   rg '"1902", "2"' --include '*.go'
   rg '"2021", "12"' --include '*.go'  # should only appear as "2021", "12-Dec"
   ```

2. **Dependency audit:** `go mod tidy` should not add or remove any modules (only promote `golang.org/x/text` from indirect to direct).

3. **Build smoke test:**
   ```bash
   make build
   ./pixe sort --source /tmp/test-photos --dest /tmp/test-archive --dry-run
   # Verify dry-run output shows paths like 2021/12-Dec/...
   ```

### Acceptance Criteria

- `go vet ./...` — zero warnings.
- `go build ./...` — compiles cleanly.
- `go test -race -timeout 120s ./...` — all tests pass.
- `make lint` — 0 issues.
- `go mod tidy` produces no diff.
- No `.go` file references the old `fmt.Sprintf("%d", month)` pattern for directory names.
- No `.go` file contains the phrase "non-zero-padded" (old spec language).

---

## Feature: Archive Database — JSON Manifest → SQLite (Tasks 29–43)

Replaces the JSON manifest (`dirB/.pixe/manifest.json`) with a SQLite database (`pixe.db`) that serves as a cumulative registry of all files ever sorted into a destination archive. Enables indexed queries, concurrent-process safety, persistent deduplication, and run history tracking. See Architecture Section 8.

**Design decisions captured from user:**
1. **Cumulative registry** — every run enriches the DB; permanent history across all sources.
2. **Concurrent access** — WAL mode + busy retry for simultaneous runs from different sources.
3. **DB location** — local `dirB` → `dirB/.pixe/pixe.db`; network mount → `~/.pixe/databases/<slug>.db` with user notice; `--db-path` override always wins.
4. **Discoverability** — `dirB/.pixe/dbpath` marker file when DB is stored outside `dirB`.
5. **Ledger** — kept as JSON in `dirA`, enriched with `run_id` linking back to the DB.
6. **Migration** — auto-migrate JSON → SQLite on first encounter, preserve original as `.migrated`.
7. **Write granularity** — commit per file for crash safety.
8. **SQLite driver** — CGo-free `modernc.org/sqlite` for single-binary distribution.

---

## Task 29 — Archive DB — `internal/archivedb` Package & Schema

**Goal:** Create the foundational SQLite database package that manages connection lifecycle, schema creation, and database configuration (WAL mode, busy timeout, foreign keys).

**Architecture Reference:** Section 8.3 (Schema Design), Section 8.5 (Concurrency & Integrity)

**Depends on:** Task 2 (domain types for `FileStatus` constants)

**New dependency:** `modernc.org/sqlite` — a CGo-free SQLite implementation for Go. Add via:
```bash
go get modernc.org/sqlite
```

**File to create: `internal/archivedb/archivedb.go`**

```go
// Package archivedb provides SQLite-backed persistence for the Pixe archive
// database. It replaces the earlier JSON manifest with a cumulative registry
// that tracks all files ever sorted into a destination archive across all runs.
//
// The database uses WAL mode for concurrent-process safety and commits each
// file completion individually for crash recovery.
package archivedb

import (
    "database/sql"
    "fmt"
    "time"

    _ "modernc.org/sqlite" // CGo-free SQLite driver
)

// DB wraps a SQLite database connection for the Pixe archive.
type DB struct {
    conn *sql.DB
    path string
}

// Open opens (or creates) the archive database at the given path.
// It applies the schema if the database is new, configures WAL mode,
// busy timeout, and enables foreign keys.
func Open(path string) (*DB, error) { ... }

// Close closes the database connection.
func (db *DB) Close() error { ... }

// Path returns the filesystem path to the database file.
func (db *DB) Path() string { return db.path }
```

**File to create: `internal/archivedb/schema.go`**

```go
package archivedb

// schemaVersion is the current schema version.
const schemaVersion = 1

// applySchema creates all tables and indexes if they do not exist,
// and records the schema version.
func (db *DB) applySchema() error { ... }
```

The schema DDL (executed within a single transaction):

```sql
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER NOT NULL,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id            TEXT PRIMARY KEY,
    pixe_version  TEXT NOT NULL,
    source        TEXT NOT NULL,
    destination   TEXT NOT NULL,
    algorithm     TEXT NOT NULL,
    workers       INTEGER NOT NULL,
    started_at    TEXT NOT NULL,
    finished_at   TEXT,
    status        TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'interrupted'))
);

CREATE TABLE IF NOT EXISTS files (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    source_path   TEXT NOT NULL,
    dest_path     TEXT,
    dest_rel      TEXT,
    checksum      TEXT,
    status        TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN (
            'pending', 'extracted', 'hashed', 'copied',
            'verified', 'tagged', 'complete',
            'failed', 'mismatch', 'tag_failed', 'duplicate'
        )),
    is_duplicate  INTEGER NOT NULL DEFAULT 0,
    capture_date  TEXT,
    file_size     INTEGER,
    extracted_at  TEXT,
    hashed_at     TEXT,
    copied_at     TEXT,
    verified_at   TEXT,
    tagged_at     TEXT,
    error         TEXT
);

CREATE INDEX IF NOT EXISTS idx_files_checksum ON files(checksum) WHERE status = 'complete';
CREATE INDEX IF NOT EXISTS idx_files_run_id ON files(run_id);
CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
CREATE INDEX IF NOT EXISTS idx_files_source ON files(source_path);
CREATE INDEX IF NOT EXISTS idx_files_capture_date ON files(capture_date);
```

After creating tables, insert the schema version row if not present:
```sql
INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (1, ?);
```

**Database configuration PRAGMAs** (applied on every `Open`):
```sql
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
```

**Acceptance Criteria:**
- `archivedb.Open("/tmp/test.db")` creates a new database with all tables and indexes.
- Opening an existing database does not re-create tables (uses `IF NOT EXISTS`).
- `PRAGMA journal_mode` returns `wal` after open.
- `PRAGMA foreign_keys` returns `1` after open.
- `db.Close()` cleanly closes the connection.
- `schema_version` table contains exactly one row with `version=1`.
- `go build ./...` succeeds with the new `modernc.org/sqlite` dependency.

---

## Task 30 — Archive DB — Run & File CRUD Operations

**Goal:** Add methods to the `archivedb.DB` type for creating/updating runs and files — the core write path used by the pipeline.

**Architecture Reference:** Section 8.3 (Schema), Section 8.5 (Transaction Granularity)

**Depends on:** Task 29

**File to create: `internal/archivedb/runs.go`**

```go
package archivedb

import "time"

// Run represents a row in the runs table.
type Run struct {
    ID           string
    PixeVersion  string
    Source       string
    Destination  string
    Algorithm    string
    Workers      int
    StartedAt    time.Time
    FinishedAt   *time.Time
    Status       string // "running", "completed", "interrupted"
}

// InsertRun creates a new run record with status "running".
// The ID should be a UUID v4 generated by the caller.
func (db *DB) InsertRun(r *Run) error { ... }

// CompleteRun sets the run's status to "completed" and records finished_at.
func (db *DB) CompleteRun(runID string, finishedAt time.Time) error { ... }

// InterruptRun sets the run's status to "interrupted" and records finished_at.
func (db *DB) InterruptRun(runID string, finishedAt time.Time) error { ... }

// GetRun retrieves a run by ID. Returns (nil, nil) if not found.
func (db *DB) GetRun(runID string) (*Run, error) { ... }

// FindInterruptedRuns returns all runs with status "running" (i.e., interrupted).
func (db *DB) FindInterruptedRuns() ([]*Run, error) { ... }
```

**File to create: `internal/archivedb/files.go`**

```go
package archivedb

import "time"

// FileRecord represents a row in the files table.
type FileRecord struct {
    ID           int64
    RunID        string
    SourcePath   string
    DestPath     *string  // nil until copied
    DestRel      *string  // nil until copied
    Checksum     *string  // nil until hashed
    Status       string
    IsDuplicate  bool
    CaptureDate  *time.Time
    FileSize     *int64
    ExtractedAt  *time.Time
    HashedAt     *time.Time
    CopiedAt     *time.Time
    VerifiedAt   *time.Time
    TaggedAt     *time.Time
    Error        *string
}

// InsertFile creates a new file record with status "pending".
// Returns the auto-generated ID.
func (db *DB) InsertFile(f *FileRecord) (int64, error) { ... }

// InsertFiles batch-inserts multiple file records within a single transaction.
// Returns the IDs of the inserted records.
func (db *DB) InsertFiles(files []*FileRecord) ([]int64, error) { ... }

// UpdateFileStatus updates a file's status and the corresponding timestamp.
// The timestamp field is determined by the status:
//   - "extracted" → extracted_at
//   - "hashed"    → hashed_at, also sets checksum
//   - "copied"    → copied_at, also sets dest_path, dest_rel
//   - "verified"  → verified_at
//   - "tagged"    → tagged_at
//   - "complete"  → (no additional timestamp)
//   - "failed"/"mismatch"/"tag_failed" → sets error field
func (db *DB) UpdateFileStatus(fileID int64, status string, opts ...UpdateOption) error { ... }

// UpdateOption configures optional fields on a file status update.
type UpdateOption func(*updateParams)

func WithChecksum(checksum string) UpdateOption { ... }
func WithDestination(destPath, destRel string) UpdateOption { ... }
func WithCaptureDate(t time.Time) UpdateOption { ... }
func WithFileSize(size int64) UpdateOption { ... }
func WithError(msg string) UpdateOption { ... }
func WithIsDuplicate(dup bool) UpdateOption { ... }

// CheckDuplicate queries whether a file with the given checksum exists
// with status "complete". Returns the dest_rel path if found, empty string if not.
// This is the hot-path dedup query — served by idx_files_checksum.
func (db *DB) CheckDuplicate(checksum string) (string, error) { ... }

// GetFilesByRun returns all file records for a given run ID.
func (db *DB) GetFilesByRun(runID string) ([]*FileRecord, error) { ... }

// GetIncompleteFiles returns all files for a run that are not in a terminal state.
// Used by resume to find files that need reprocessing.
func (db *DB) GetIncompleteFiles(runID string) ([]*FileRecord, error) { ... }
```

**Technical Notes:**
- `UpdateFileStatus` uses a single UPDATE statement with conditional SET clauses based on the options provided. Each call is wrapped in its own transaction (commit-per-file).
- `InsertFiles` uses a single transaction with a prepared INSERT statement for batch efficiency during the discovery phase.
- `CheckDuplicate` is a simple SELECT that hits the partial index `idx_files_checksum`.
- All timestamp fields are stored as ISO 8601 UTC strings (`time.Time.UTC().Format(time.RFC3339)`).

**Acceptance Criteria:**
- `InsertRun` + `GetRun` round-trips all fields correctly.
- `CompleteRun` sets `finished_at` and `status = "completed"`.
- `InsertFile` returns a valid auto-increment ID.
- `InsertFiles` batch-inserts 100 files in a single transaction.
- `UpdateFileStatus` with `WithChecksum` sets both `status` and `checksum`.
- `CheckDuplicate` returns the `dest_rel` for a known checksum, empty string for unknown.
- `GetIncompleteFiles` returns only non-terminal files for a given run.
- All operations work correctly with WAL mode enabled.

---

## Task 31 — Archive DB — Query Methods

**Goal:** Add read-only query methods to `archivedb.DB` that expose the query patterns defined in Architecture Section 8.4. These are used by future CLI commands (`pixe query`) and by the pipeline for operational queries.

**Architecture Reference:** Section 8.4 (Query Patterns)

**Depends on:** Task 30

**File to create: `internal/archivedb/queries.go`**

```go
package archivedb

import "time"

// RunSummary is a lightweight view of a run for listing purposes.
type RunSummary struct {
    ID          string
    PixeVersion string
    Source      string
    StartedAt   time.Time
    FinishedAt  *time.Time
    Status      string
    FileCount   int
}

// ListRuns returns all runs ordered by started_at descending.
func (db *DB) ListRuns() ([]*RunSummary, error) { ... }

// FilesBySource returns all files imported from a given source directory.
func (db *DB) FilesBySource(sourceDir string) ([]*FileRecord, error) { ... }

// FilesByCaptureDateRange returns completed files with capture dates in [start, end].
func (db *DB) FilesByCaptureDateRange(start, end time.Time) ([]*FileRecord, error) { ... }

// FilesByImportDateRange returns files verified within [start, end].
func (db *DB) FilesByImportDateRange(start, end time.Time) ([]*FileRecord, error) { ... }

// FilesWithErrors returns all files in error states across all runs,
// joined with their run's source directory for context.
type FileWithSource struct {
    FileRecord
    RunSource string
}
func (db *DB) FilesWithErrors() ([]*FileWithSource, error) { ... }

// AllDuplicates returns all files marked as duplicates.
func (db *DB) AllDuplicates() ([]*FileRecord, error) { ... }

// DuplicatePairs returns each duplicate alongside the original it duplicates.
type DuplicatePair struct {
    DuplicateSource string
    DuplicateDest   string
    OriginalDest    string
}
func (db *DB) DuplicatePairs() ([]*DuplicatePair, error) { ... }

// ArchiveInventory returns all completed, non-duplicate files (the canonical archive contents).
type InventoryEntry struct {
    DestRel     string
    Checksum    string
    CaptureDate *time.Time
}
func (db *DB) ArchiveInventory() ([]*InventoryEntry, error) { ... }
```

**Acceptance Criteria:**
- `ListRuns` returns runs in reverse chronological order with file counts.
- `FilesBySource` correctly filters by source directory path.
- `FilesByCaptureDateRange` returns only completed files within the date range.
- `FilesWithErrors` joins files with their run source and returns only error-state files.
- `DuplicatePairs` correctly pairs each duplicate with its original via checksum join.
- `ArchiveInventory` excludes duplicates and non-complete files.
- All queries use the defined indexes (verify via `EXPLAIN QUERY PLAN` in tests).

---

## Task 32 — DB Location Resolver — `internal/dblocator` Package

**Goal:** Implement the database location resolution logic: `--db-path` override → `dbpath` marker → local default, with network mount detection and slug generation for the fallback path.

**Architecture Reference:** Section 8.2 (Database Location)

**Depends on:** Task 29

**File to create: `internal/dblocator/dblocator.go`**

```go
// Package dblocator resolves the filesystem path for the Pixe archive database.
// It implements the priority chain: explicit --db-path → dbpath marker file →
// local default (dirB/.pixe/pixe.db), with automatic fallback to
// ~/.pixe/databases/<slug>.db when dirB is on a network filesystem.
package dblocator

// Location holds the resolved database path and metadata about the resolution.
type Location struct {
    // DBPath is the absolute path to the SQLite database file.
    DBPath string
    // IsRemote is true if dirB was detected as a network mount.
    IsRemote bool
    // MarkerNeeded is true if a dbpath marker should be written to dirB/.pixe/.
    MarkerNeeded bool
    // Notice is a user-facing message explaining the location choice.
    // Empty if the default local path was used.
    Notice string
}

// Resolve determines the database path for the given destination directory.
//
// Priority chain:
//  1. explicitPath (from --db-path flag) — used unconditionally if non-empty.
//  2. dirB/.pixe/dbpath marker file — if it exists, its contents are used.
//  3. dirB/.pixe/pixe.db — if dirB is on a local filesystem.
//  4. ~/.pixe/databases/<slug>.db — if dirB is on a network mount.
func Resolve(dirB string, explicitPath string) (*Location, error) { ... }

// WriteMarker writes the dbpath marker file at dirB/.pixe/dbpath
// containing the absolute path to the database.
func WriteMarker(dirB string, dbPath string) error { ... }

// ReadMarker reads the dbpath marker file at dirB/.pixe/dbpath.
// Returns ("", nil) if the marker does not exist.
func ReadMarker(dirB string) (string, error) { ... }
```

**File to create: `internal/dblocator/filesystem.go`**

```go
package dblocator

// isNetworkMount returns true if the given path resides on a network
// filesystem (NFS, SMB/CIFS, AFP). Uses OS-level filesystem type inspection.
func isNetworkMount(path string) (bool, error) { ... }
```

**Platform-specific implementation:**

**File: `internal/dblocator/filesystem_darwin.go`**
```go
//go:build darwin

package dblocator

import "syscall"

func isNetworkMount(path string) (bool, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(path, &stat); err != nil {
        return false, err
    }
    // Convert Fstypename [16]int8 to string.
    fstype := fstypeName(stat.Fstypename[:])
    // Network filesystem types on macOS.
    switch fstype {
    case "nfs", "smbfs", "afpfs", "webdav":
        return true, nil
    }
    return false, nil
}
```

**File: `internal/dblocator/filesystem_linux.go`**
```go
//go:build linux

package dblocator

import "syscall"

// Linux filesystem magic numbers for network mounts.
const (
    nfsMagic  = 0x6969
    smbMagic  = 0x517B
    smb2Magic = 0xFE534D42
    cifsMagic = 0xFF534D42
)

func isNetworkMount(path string) (bool, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(path, &stat); err != nil {
        return false, err
    }
    switch stat.Type {
    case nfsMagic, smbMagic, smb2Magic, cifsMagic:
        return true, nil
    }
    return false, nil
}
```

**Slug generation:**

```go
// slug generates a human-readable identifier for a dirB path.
// Format: <last-path-component>-<truncated-hash>.
// Example: "/Volumes/NAS/Photos/archive" → "archive-a1b2c3d4"
func slug(dirB string) string {
    abs, _ := filepath.Abs(dirB)
    base := strings.ToLower(filepath.Base(abs))
    // Sanitize: keep only alphanumeric and hyphens.
    base = sanitize(base)
    if base == "" {
        base = "pixe"
    }
    h := sha256.Sum256([]byte(abs))
    return fmt.Sprintf("%s-%x", base, h[:4])
}
```

**Marker file format:** Plain text, single line, the absolute path to the database file. No trailing newline.

**Acceptance Criteria:**
- `Resolve(dirB, "/explicit/path.db")` returns the explicit path with `MarkerNeeded=true`.
- `Resolve(dirB, "")` on a local filesystem returns `dirB/.pixe/pixe.db` with `MarkerNeeded=false`.
- `Resolve(dirB, "")` on a network mount returns `~/.pixe/databases/<slug>.db` with `MarkerNeeded=true` and a non-empty `Notice`.
- `WriteMarker` + `ReadMarker` round-trips the database path.
- `ReadMarker` returns `("", nil)` when no marker exists.
- `slug("/Volumes/NAS/Photos/archive")` returns `"archive-<8hex>"`.
- `slug("/")` returns `"pixe-<8hex>"` (edge case).
- Network mount detection works on macOS (darwin build tag).

---

## Task 33 — Domain Types — SQLite-Era Updates

**Goal:** Update the domain types and config struct to support the SQLite database: add `RunID` to the ledger, bump ledger version, and add `DBPath` to `AppConfig`.

**Architecture Reference:** Section 8.8 (Ledger v2), Section 9.1 (New Flag)

**Depends on:** Task 2, Task 29

### Files to modify

#### 1. `internal/config/config.go` — Add `DBPath` field

```go
type AppConfig struct {
    // ... existing fields ...

    // DBPath is an explicit path to the SQLite archive database.
    // If empty, the database location is auto-resolved (see dblocator package).
    DBPath string
}
```

#### 2. `internal/domain/pipeline.go` — Update Ledger struct

```go
// Ledger is the source-side record written to dirA/.pixe_ledger.json.
type Ledger struct {
    Version     int           `json:"version"`
    PixeVersion string        `json:"pixe_version"`
    RunID       string        `json:"run_id"`          // ← NEW: UUID linking to archive DB
    PixeRun     time.Time     `json:"pixe_run"`
    Algorithm   string        `json:"algorithm"`
    Destination string        `json:"destination"`
    Files       []LedgerEntry `json:"files"`
}
```

The `Version` field will be set to `2` when the ledger is created with a `RunID`. Existing code that creates ledgers with `Version: 1` will be updated in Task 38.

**Acceptance Criteria:**
- `AppConfig.DBPath` field exists and is a `string`.
- `Ledger.RunID` field exists with JSON tag `"run_id"`.
- `go build ./...` succeeds — the new fields are additive and don't break existing struct literals (Go named-field initialization is forward-compatible).
- Existing tests pass unchanged.

---

## Task 34 — JSON Manifest Migration — `internal/migrate` Package

**Goal:** Implement automatic migration from the JSON manifest to the SQLite database. When Pixe encounters `dirB/.pixe/manifest.json` but no database, it migrates all data into a new database, preserves the original file, and notifies the user.

**Architecture Reference:** Section 8.7 (Migration from JSON Manifest)

**Depends on:** Task 29, Task 30

**File to create: `internal/migrate/migrate.go`**

```go
// Package migrate handles automatic migration from the legacy JSON manifest
// (dirB/.pixe/manifest.json) to the SQLite archive database.
package migrate

import (
    "github.com/cwlls/pixe-go/internal/archivedb"
    "github.com/cwlls/pixe-go/internal/domain"
)

// Result holds the outcome of a migration attempt.
type Result struct {
    // Migrated is true if a migration was performed.
    Migrated bool
    // FileCount is the number of file entries migrated.
    FileCount int
    // Notice is a user-facing message describing what happened.
    Notice string
}

// MigrateIfNeeded checks for a legacy manifest.json at dirB/.pixe/ and,
// if found (and no .migrated version exists), migrates its contents into
// the provided database.
//
// Steps:
//  1. Check for dirB/.pixe/manifest.json — if absent, return (not migrated).
//  2. Check for dirB/.pixe/manifest.json.migrated — if present, skip (already done).
//  3. Read and parse the JSON manifest.
//  4. Create a synthetic run in the DB using manifest metadata.
//  5. Insert all file entries into the DB, mapping ManifestEntry fields to FileRecord.
//  6. Rename manifest.json → manifest.json.migrated.
//  7. Return the result with a user-facing notice.
func MigrateIfNeeded(db *archivedb.DB, dirB string) (*Result, error) { ... }
```

**Field mapping from `ManifestEntry` → `FileRecord`:**

| ManifestEntry field | FileRecord field | Notes |
|---|---|---|
| `Source` | `SourcePath` | Direct copy |
| `Destination` | `DestPath` | Direct copy (absolute) |
| — | `DestRel` | Computed: `strings.TrimPrefix(entry.Destination, manifest.Destination + "/")` |
| `Checksum` | `Checksum` | Direct copy |
| `Status` | `Status` | Direct copy (same enum values) |
| — | `IsDuplicate` | Inferred: `strings.Contains(destRel, "duplicates/")` |
| `ExtractedAt` | `ExtractedAt` | Direct copy |
| `CopiedAt` | `CopiedAt` | Direct copy |
| `VerifiedAt` | `VerifiedAt` | Direct copy |
| `TaggedAt` | `TaggedAt` | Direct copy |
| `Error` | `Error` | Direct copy |

**Synthetic run creation:**

```go
syntheticRun := &archivedb.Run{
    ID:          uuid.New().String(),  // or a deterministic UUID from manifest hash
    PixeVersion: manifest.PixeVersion,
    Source:      manifest.Source,
    Destination: manifest.Destination,
    Algorithm:   manifest.Algorithm,
    Workers:     manifest.Workers,
    StartedAt:   manifest.StartedAt,
    FinishedAt:  &manifest.StartedAt,  // best approximation
    Status:      "completed",          // the prior run is assumed complete
}
```

**UUID dependency:** Add `github.com/google/uuid` for UUID v4 generation:
```bash
go get github.com/google/uuid
```

**Acceptance Criteria:**
- Given a `dirB` with `manifest.json` containing 5 entries, `MigrateIfNeeded` creates a DB with 1 run and 5 files.
- The original `manifest.json` is renamed to `manifest.json.migrated`.
- Calling `MigrateIfNeeded` again (with `.migrated` present) returns `Migrated: false` — idempotent.
- Calling `MigrateIfNeeded` on a `dirB` with no manifest returns `Migrated: false`.
- The synthetic run has `status = "completed"`.
- File entries preserve all timestamps, checksums, and statuses.
- `IsDuplicate` is correctly inferred from the destination path.
- The `Result.Notice` contains the file count (e.g., `"Migrated 5 files from manifest.json → pixe.db"`).

---

## Task 35 — Pipeline Refactor — Replace JSON Manifest with Archive DB

**Goal:** Rewrite the pipeline orchestrator (`pipeline.go` and `worker.go`) to use `archivedb.DB` instead of `manifest.Save`/`manifest.Load`. This is the largest single task — it touches the core data flow.

**Architecture Reference:** Section 8.5 (Transaction Granularity), Section 8.6 (Database Lifecycle)

**Depends on:** Task 29, Task 30, Task 32, Task 33

### High-level changes

#### 1. `SortOptions` — Add DB reference

```go
type SortOptions struct {
    Config       *config.AppConfig
    Hasher       *hash.Hasher
    Registry     *discovery.Registry
    RunTimestamp string
    Output       io.Writer
    DB           *archivedb.DB   // ← NEW: archive database
    RunID        string          // ← NEW: UUID for this run
}
```

#### 2. `pipeline.Run()` — Rewrite flow

**Before (JSON):**
1. `manifest.Load(dirB)` → create or load manifest
2. Build dedup index from manifest entries (`map[checksum]destRel`)
3. Walk dirA, add new entries to manifest, `manifest.Save()`
4. Process each file, mutate `ManifestEntry`, `manifest.Save()` after each
5. Write ledger, final `manifest.Save()`

**After (SQLite):**
1. DB is already opened and passed in via `SortOptions.DB`
2. `db.InsertRun()` with `status = "running"`
3. Walk dirA, `db.InsertFiles()` batch-insert as `"pending"`
4. Dedup check: `db.CheckDuplicate(checksum)` — no in-memory map needed
5. Process each file, `db.UpdateFileStatus()` after each stage — commit per file
6. `db.CompleteRun()` at end
7. Write ledger with `RunID`

**Key difference:** The in-memory `dedupIndex map[string]string` is replaced by `db.CheckDuplicate(checksum)`. This is a SELECT query hitting the partial index — fast and memory-bounded.

#### 3. `worker.go` — Rewrite coordinator loop

The coordinator currently:
- Maintains `dedupIndex` in memory
- Calls `saveManifest()` after each file

**After:**
- Calls `db.CheckDuplicate()` for dedup decisions
- Calls `db.UpdateFileStatus()` after each file completes (commit per file)
- No more `saveManifest()` calls

Workers continue to operate the same way — they extract, hash, copy, verify, tag. The only change is that the coordinator writes to the DB instead of the JSON manifest.

#### 4. Remove `manifest.Save`/`manifest.Load` from pipeline

The `internal/manifest` package is **not deleted** — it's still needed for:
- `manifest.Load()` — used by the migration path (Task 34)
- `manifest.SaveLedger()` / `manifest.LoadLedger()` — ledger persistence is unchanged

But `manifest.Save()` is no longer called from the pipeline.

#### 5. `SortResult` — unchanged

The `SortResult` struct returned by `Run()` is unchanged. The summary statistics are computed the same way.

### Files to modify

- `internal/pipeline/pipeline.go` — major rewrite of `Run()` and `processFile()`
- `internal/pipeline/worker.go` — major rewrite of `RunConcurrent()` coordinator loop

### Files NOT modified

- `internal/manifest/manifest.go` — kept for migration and ledger
- `internal/copy/copy.go` — unchanged
- `internal/pathbuilder/pathbuilder.go` — unchanged
- `internal/discovery/` — unchanged

**Acceptance Criteria:**
- `pipeline.Run()` creates a run record in the DB with `status = "running"`.
- Each discovered file is inserted as `"pending"` via batch insert.
- Each file completion commits a status update to the DB.
- Dedup checks use `db.CheckDuplicate()` — no in-memory map.
- On successful completion, the run is marked `"completed"`.
- On context cancellation (Ctrl+C), the run is marked `"interrupted"`.
- The ledger is still written to `dirA` via `manifest.SaveLedger()`.
- `manifest.Save()` is no longer called anywhere in the pipeline.
- `go build ./...` succeeds.
- Existing pipeline tests are updated to provide a DB in `SortOptions`.

---

## Task 36 — Pipeline — Cross-Process Dedup Race Handling

**Goal:** Handle the race condition where two simultaneous `pixe sort` processes discover the same file (identical checksum) from different sources. The second process to commit should detect the conflict and retroactively route its copy to `duplicates/`.

**Architecture Reference:** Section 8.5 (Cross-Process Dedup Race Condition)

**Depends on:** Task 35

### Implementation

After a file is copied and verified, but before marking it `"complete"`, the coordinator performs a **post-commit dedup re-check**:

```go
// In the coordinator, after copy+verify succeeds:
existingDest, err := db.CheckDuplicate(checksum)
if err != nil {
    // handle error
}
if existingDest != "" {
    // Another process completed this checksum while we were copying.
    // Our copy is now a duplicate. Move it to the duplicates directory.
    dupDest := pathbuilder.Build(captureDate, checksum, ext, true, runTimestamp)
    if err := os.Rename(destPath, filepath.Join(dirB, dupDest)); err != nil {
        // handle error — file is still at destPath, mark as failed
    }
    // Update the file record with the new duplicate destination.
    db.UpdateFileStatus(fileID, "complete",
        WithDestination(filepath.Join(dirB, dupDest), dupDest),
        WithIsDuplicate(true),
    )
} else {
    // We're the first — mark complete at the original destination.
    db.UpdateFileStatus(fileID, "complete")
}
```

**Key insight:** The dedup check and the status update must happen within the same transaction to prevent a TOCTOU race between two processes both thinking they're first. Add a method:

```go
// CompleteFileWithDedupCheck atomically checks for an existing completed file
// with the same checksum and marks this file as complete. If a duplicate is
// detected, it returns the existing destination path so the caller can
// relocate the physical file.
func (db *DB) CompleteFileWithDedupCheck(fileID int64, checksum string) (existingDest string, err error) { ... }
```

This method runs within a single transaction:
1. `SELECT dest_rel FROM files WHERE checksum = ? AND status = 'complete' AND id != ? LIMIT 1`
2. If found: update file with `is_duplicate = 1`, return the existing dest
3. If not found: update file with `status = 'complete'`, return empty string

**Acceptance Criteria:**
- When two files with the same checksum are processed, the second one is correctly routed to `duplicates/`.
- The physical file is moved (renamed) to the duplicates directory.
- The DB record reflects `is_duplicate = 1` and the updated destination path.
- The operation is atomic — no window where both files appear as non-duplicates.

---

## Task 37 — CLI Updates — `--db-path` Flag & Resume Rewrite

**Goal:** Add the `--db-path` flag to `pixe sort` and `pixe resume`, and rewrite the resume command to use the database discovery chain instead of loading a JSON manifest.

**Architecture Reference:** Section 9.1 (New Flag), Section 9.2 (Updated `pixe resume`)

**Depends on:** Task 32, Task 35

### Files to modify

#### 1. `cmd/sort.go` — Add `--db-path` flag and DB lifecycle

Add the flag:
```go
sortCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database")
_ = viper.BindPFlag("db_path", sortCmd.Flags().Lookup("db-path"))
```

In `runSort()`, after resolving config:
```go
cfg.DBPath = viper.GetString("db_path")

// Resolve database location.
loc, err := dblocator.Resolve(cfg.Destination, cfg.DBPath)
if err != nil {
    return fmt.Errorf("resolve database location: %w", err)
}
if loc.Notice != "" {
    fmt.Fprintln(os.Stderr, loc.Notice)
}

// Open the database.
db, err := archivedb.Open(loc.DBPath)
if err != nil {
    return fmt.Errorf("open archive database: %w", err)
}
defer db.Close()

// Write dbpath marker if needed.
if loc.MarkerNeeded {
    if err := dblocator.WriteMarker(cfg.Destination, loc.DBPath); err != nil {
        return fmt.Errorf("write dbpath marker: %w", err)
    }
}

// Auto-migrate from JSON manifest if needed.
migResult, err := migrate.MigrateIfNeeded(db, cfg.Destination)
if err != nil {
    return fmt.Errorf("migrate manifest: %w", err)
}
if migResult.Migrated {
    fmt.Fprintln(os.Stdout, migResult.Notice)
}

// Generate run ID.
runID := uuid.New().String()

opts := pipeline.SortOptions{
    Config:       cfg,
    Hasher:       h,
    Registry:     reg,
    RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
    Output:       os.Stdout,
    DB:           db,
    RunID:        runID,
}
```

#### 2. `cmd/resume.go` — Rewrite to use DB

Replace the current manifest-based resume with database-based resume:

```go
func runResume(cmd *cobra.Command, args []string) error {
    dir := viper.GetString("resume_dir")
    dbPath := viper.GetString("db_path")

    // Resolve database location.
    loc, err := dblocator.Resolve(dir, dbPath)
    if err != nil {
        return fmt.Errorf("resolve database location: %w", err)
    }

    // Open the database.
    db, err := archivedb.Open(loc.DBPath)
    if err != nil {
        return fmt.Errorf("open archive database: %w", err)
    }
    defer db.Close()

    // Find interrupted runs.
    interrupted, err := db.FindInterruptedRuns()
    if err != nil {
        return fmt.Errorf("find interrupted runs: %w", err)
    }
    if len(interrupted) == 0 {
        fmt.Println("No interrupted runs found.")
        return nil
    }

    // Resume the most recent interrupted run.
    run := interrupted[0]
    // ... validate source still exists, build pipeline opts, call pipeline.Run() ...
}
```

Add `--db-path` flag to resume:
```go
resumeCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database")
_ = viper.BindPFlag("db_path", resumeCmd.Flags().Lookup("db-path"))
```

**Acceptance Criteria:**
- `pixe sort --db-path /tmp/custom.db --source ... --dest ...` uses the specified DB path.
- `pixe sort` without `--db-path` auto-resolves the DB location.
- `pixe resume --dir <dirB>` discovers the DB via the priority chain.
- `pixe resume --dir <dirB> --db-path /tmp/custom.db` uses the explicit path.
- The `--db-path` flag is bindable via config file (`db_path`) and env var (`PIXE_DB_PATH`).

---

## Task 38 — Ledger Update — Add `run_id` Field

**Goal:** Wire the run UUID into ledger creation and bump the ledger version to 2.

**Architecture Reference:** Section 8.8 (Ledger v2)

**Depends on:** Task 33, Task 35

### Files to modify

#### 1. `internal/pipeline/pipeline.go` — Ledger creation

```go
// BEFORE:
ledger := &domain.Ledger{
    Version:     1,
    PixeVersion: version.Version,
    PixeRun:     startedAt,
    Algorithm:   cfg.Algorithm,
    Destination: cfg.Destination,
}

// AFTER:
ledger := &domain.Ledger{
    Version:     2,
    PixeVersion: version.Version,
    RunID:       opts.RunID,
    PixeRun:     startedAt,
    Algorithm:   cfg.Algorithm,
    Destination: cfg.Destination,
}
```

#### 2. `internal/pipeline/worker.go` — Same change in all ledger creation sites

Update all 2-3 locations where `domain.Ledger` is constructed to include `RunID: opts.RunID` and `Version: 2`.

**Acceptance Criteria:**
- After a `pixe sort` run, `dirA/.pixe_ledger.json` contains `"version": 2` and `"run_id": "<uuid>"`.
- The `run_id` in the ledger matches the run ID in the archive database.
- `SELECT * FROM files WHERE run_id = '<ledger_run_id>'` returns the same files listed in the ledger.
- Existing ledger loading (`manifest.LoadLedger`) still works with v1 ledgers (the `RunID` field is simply empty).

---

## Task 39 — Archive DB — Unit Tests

**Goal:** Comprehensive unit tests for the `internal/archivedb` package covering schema creation, CRUD operations, query methods, WAL concurrency, and busy retry behavior.

**Architecture Reference:** Section 8.3, 8.4, 8.5

**Depends on:** Tasks 29, 30, 31

**File to create: `internal/archivedb/archivedb_test.go`**

### Test cases

1. **`TestOpen_createsSchema`** — Open a new DB, verify all tables exist via `sqlite_master`.
2. **`TestOpen_idempotent`** — Open an existing DB, verify no errors and schema is intact.
3. **`TestOpen_WALMode`** — Verify `PRAGMA journal_mode` returns `wal`.
4. **`TestSchemaVersion`** — Verify `schema_version` table has version 1.
5. **`TestInsertRun_roundtrip`** — Insert a run, retrieve it, verify all fields.
6. **`TestCompleteRun`** — Insert a run, complete it, verify `finished_at` and `status`.
7. **`TestInterruptRun`** — Insert a run, interrupt it, verify status.
8. **`TestFindInterruptedRuns`** — Insert 3 runs (1 running, 1 completed, 1 interrupted), verify only the running one is returned.
9. **`TestInsertFile_roundtrip`** — Insert a file, retrieve by run, verify all fields.
10. **`TestInsertFiles_batch`** — Batch-insert 100 files, verify count.
11. **`TestUpdateFileStatus_progression`** — Walk a file through all pipeline stages, verify timestamps are set.
12. **`TestCheckDuplicate_found`** — Insert a completed file, check its checksum, verify dest_rel returned.
13. **`TestCheckDuplicate_notFound`** — Check a checksum that doesn't exist, verify empty string.
14. **`TestCheckDuplicate_ignoresNonComplete`** — Insert a file with status "hashed" (not complete), verify CheckDuplicate returns empty.
15. **`TestGetIncompleteFiles`** — Insert files in various states, verify only non-terminal ones returned.
16. **`TestListRuns`** — Insert 3 runs, verify returned in reverse chronological order with file counts.
17. **`TestFilesWithErrors`** — Insert files with error states, verify join with run source.
18. **`TestDuplicatePairs`** — Insert an original and a duplicate with same checksum, verify pairing.
19. **`TestConcurrentReaders`** — Open two connections to the same DB, read simultaneously, verify no errors.
20. **`TestBusyRetry`** — Simulate write contention between two connections, verify the second writer succeeds after retry.

All tests use `t.TempDir()` for database file isolation.

**Acceptance Criteria:**
- All 20 test cases pass.
- Tests run with `-race` flag without data race warnings.
- Tests complete in under 5 seconds.

---

## Task 40 — DB Locator — Unit Tests

**Goal:** Unit tests for the `internal/dblocator` package covering the resolution priority chain, slug generation, and marker file operations.

**Depends on:** Task 32

**File to create: `internal/dblocator/dblocator_test.go`**

### Test cases

1. **`TestResolve_explicitPath`** — Explicit path always wins, `MarkerNeeded=true`.
2. **`TestResolve_markerFile`** — Write a marker, resolve without explicit path, verify marker contents used.
3. **`TestResolve_localDefault`** — No marker, local filesystem, verify `dirB/.pixe/pixe.db`.
4. **`TestResolve_priorityOrder`** — Explicit > marker > default.
5. **`TestWriteMarker_ReadMarker_roundtrip`** — Write and read back.
6. **`TestReadMarker_notExists`** — Returns empty string, no error.
7. **`TestSlug_normalPath`** — Verify format: `<base>-<8hex>`.
8. **`TestSlug_rootPath`** — Edge case: `/` → `"pixe-<8hex>"`.
9. **`TestSlug_deterministic`** — Same input always produces same slug.
10. **`TestSlug_differentPaths`** — Different inputs produce different slugs.

**Note:** Network mount detection (`isNetworkMount`) is difficult to unit test without actual network mounts. Test it with a mock/stub or skip on CI with a build tag.

**Acceptance Criteria:**
- All test cases pass.
- Slug generation is deterministic and collision-resistant.
- Marker file round-trip works correctly.

---

## Task 41 — Migration — Unit Tests

**Goal:** Unit tests for the `internal/migrate` package covering JSON→SQLite migration, idempotency, and edge cases.

**Depends on:** Task 34

**File to create: `internal/migrate/migrate_test.go`**

### Test cases

1. **`TestMigrateIfNeeded_noManifest`** — No manifest.json → `Migrated: false`.
2. **`TestMigrateIfNeeded_alreadyMigrated`** — `.migrated` exists → `Migrated: false`.
3. **`TestMigrateIfNeeded_success`** — Manifest with 5 entries → DB has 1 run + 5 files, manifest renamed.
4. **`TestMigrateIfNeeded_preservesTimestamps`** — Verify all timestamp fields survive migration.
5. **`TestMigrateIfNeeded_preservesStatuses`** — Verify all status values map correctly.
6. **`TestMigrateIfNeeded_infersDuplicates`** — Entry with `duplicates/` in dest path → `is_duplicate = 1`.
7. **`TestMigrateIfNeeded_syntheticRunMetadata`** — Verify the synthetic run has correct pixe_version, source, algorithm, etc.
8. **`TestMigrateIfNeeded_idempotent`** — Call twice, second call is a no-op.

All tests create a real `manifest.json` file in `t.TempDir()` and a real SQLite database.

**Acceptance Criteria:**
- All 8 test cases pass.
- Migration is lossless — all data from the JSON manifest is present in the DB.
- The original `manifest.json` is preserved as `manifest.json.migrated`.

---

## Task 42 — Integration Tests — SQLite Pipeline End-to-End

**Goal:** End-to-end integration tests that exercise the full sort → verify → resume cycle using the SQLite database, including concurrent run simulation.

**Depends on:** Tasks 35, 36, 37, 38

**File to modify: `internal/integration/integration_test.go`**

### New test cases (add to existing integration test file)

1. **`TestIntegration_SQLite_FullSort`** — Sort fixture files, verify:
   - Database exists at `dirB/.pixe/pixe.db`.
   - `runs` table has 1 row with `status = "completed"`.
   - `files` table has correct count with all `status = "complete"`.
   - Dedup check returns correct results.
   - Ledger has `version: 2` and `run_id` matching the DB.

2. **`TestIntegration_SQLite_Resume`** — Sort 5 files, simulate interrupt (mark run as "running", reset 2 files to "pending"), resume, verify all 5 complete.

3. **`TestIntegration_SQLite_MultiSource`** — Sort from source A, then sort from source B into the same `dirB`. Verify:
   - 2 runs in the `runs` table.
   - Files from both sources in the `files` table.
   - Dedup works across runs (if source B has a file identical to source A, it's routed to duplicates).

4. **`TestIntegration_SQLite_Migration`** — Create a `dirB` with a legacy `manifest.json`, run `pixe sort` against it, verify:
   - Auto-migration occurred.
   - `manifest.json.migrated` exists.
   - DB contains the migrated entries plus the new sort's entries.

5. **`TestIntegration_SQLite_DryRun`** — Dry-run creates a run record but no file copies. Verify DB state.

6. **`TestIntegration_SQLite_NoDBPathMarker_LocalFS`** — On local filesystem, verify no `dbpath` marker is created.

### Updated existing tests

All existing integration tests that reference `manifest.json` or `manifest.Load()` must be updated to use the database. The `TestIntegration_FullSort`, `TestIntegration_Resume`, etc. should be updated to verify DB state instead of (or in addition to) manifest state.

**Acceptance Criteria:**
- All new integration tests pass.
- All updated existing integration tests pass.
- Tests run with `-race` flag without data race warnings.
- Multi-source test demonstrates cumulative registry behavior.
- Migration test demonstrates seamless JSON→SQLite transition.

---

## Task 43 — Tests & Verification — Full Suite Green

**Goal:** Verify the entire codebase compiles, passes all tests, and passes lint after the SQLite migration.

**Depends on:** Tasks 39, 40, 41, 42

### Verification commands

```bash
go vet ./...                                    # No warnings
go build ./...                                  # Compiles cleanly
go test -race -timeout 120s ./...               # All tests pass
make lint                                       # 0 issues
go mod tidy                                     # No diff
```

### Specific checks

1. **No stale JSON manifest references in pipeline:**
   ```bash
   # Should return zero matches in pipeline files:
   rg 'manifest\.Save\(' internal/pipeline/
   rg 'manifest\.Load\(' internal/pipeline/
   # manifest.SaveLedger is still valid — ledger is unchanged
   ```

2. **No in-memory dedup index in pipeline:**
   ```bash
   # The old dedupIndex map should be gone:
   rg 'dedupIndex' internal/pipeline/
   ```

3. **Dependency audit:**
   ```bash
   go mod tidy
   # New dependencies: modernc.org/sqlite, github.com/google/uuid
   # Verify no unexpected additions
   ```

4. **Build smoke test:**
   ```bash
   make build
   ./pixe sort --source /tmp/test-photos --dest /tmp/test-archive --dry-run
   # Verify DB is created at /tmp/test-archive/.pixe/pixe.db
   # Verify output shows normal sort behavior
   ```

### Acceptance Criteria

- `go vet ./...` — zero warnings.
- `go build ./...` — compiles cleanly.
- `go test -race -timeout 120s ./...` — all tests pass (unit + integration).
- `make lint` — 0 issues.
- `go mod tidy` produces no diff.
- No pipeline code references `manifest.Save()` or `manifest.Load()`.
- No pipeline code uses an in-memory `dedupIndex` map.
- The `internal/manifest` package is retained for ledger persistence and migration support only.

---

## Task 44 — Version Vars & Command — Collapse into `cmd`

**Goal:** Rewrite `cmd/version.go` to contain the version variables, the `init()` dev-enrichment logic, the `fullVersion()` formatter, an exported `Version()` getter, and the `pixe version` Cobra command. This replaces the `internal/version` package as the home for all version concerns.

**Architecture Reference:** Section 3.1 (Source of Truth: Git Tags), Section 3.3 (Accessor Function), Section 3.4 (Consumers)

**Depends on:** Nothing (this is the foundation task)

**File to rewrite: `cmd/version.go`**

```go
// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version fields — injected at build time via -ldflags -X.
// When built without ldflags (e.g. plain `go build` or `go test`), these
// retain their dev defaults.
//
// GoReleaser injects all three; the Makefile (via goreleaser build --snapshot)
// injects commit and buildDate but leaves version as "dev".
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func init() {
	// Enrich dev builds with the commit hash for traceability.
	// A Makefile build injects commit but not version, producing "dev-abc1234".
	if version == "dev" && commit != "unknown" {
		version = "dev-" + commit
	}

	rootCmd.AddCommand(versionCmd)
}

// Version returns the current version string for use by internal packages
// (e.g., pipeline stamping into manifests and ledgers).
func Version() string { return version }

// fullVersion returns the human-readable version string.
//
// Examples:
//
//	Release:  "pixe v0.10.0 (commit: abc1234, built: 2026-03-06T10:30:00Z)"
//	Dev:      "pixe vdev-2159446 (commit: 2159446, built: unknown)"
//	Bare:     "pixe vdev (commit: unknown, built: unknown)"
func fullVersion() string {
	return fmt.Sprintf("pixe v%s (commit: %s, built: %s)", version, commit, buildDate)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Pixe",
	Long:  "Print the version, git commit, and build date of the Pixe binary.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(fullVersion())
	},
}
```

**Key design points:**

- `version`, `commit`, `buildDate` are **unexported** package-level `var`s — settable by ldflags (`-X github.com/cwlls/pixe-go/cmd.version=...`), invisible outside `cmd`.
- `Version()` is the **exported getter** — the only way for internal packages (like `pipeline`) to read the version string.
- `fullVersion()` is **unexported** — only used by the `pixe version` command within `cmd`.
- The `init()` function handles the dev-enrichment: when `version == "dev"` and `commit != "unknown"` (i.e., a Makefile build injected the commit but not the version), it produces `"dev-abc1234"`.
- The `init()` function also registers the `versionCmd` with `rootCmd` (moved from the old standalone `init()` block).

**Acceptance Criteria:**
- `cmd/version.go` compiles with the new content.
- `go build ./...` succeeds (the old `internal/version` package still exists at this point — it will be removed in Task 45).
- `Version()` returns `"dev"` when built without ldflags.
- `fullVersion()` returns `"pixe vdev (commit: unknown, built: unknown)"` when built without ldflags.
- The `pixe version` command prints the output of `fullVersion()`.

---

## Task 45 — Delete `internal/version` Package

**Goal:** Remove the `internal/version` package entirely. This is safe only after Tasks 44 and 46 are complete (all consumers have been migrated to `cmd.Version()`).

**Architecture Reference:** Section 3.1 (version vars now live in `cmd`)

**Depends on:** Task 44 (new version vars exist in `cmd`), Task 46 (pipeline no longer imports `internal/version`)

**Files to delete:**
- `internal/version/version.go`
- `internal/version/version_test.go`

**After deletion, verify:**
```bash
# No Go file should import the old package:
rg 'internal/version' --include '*.go'
# Should return zero matches.
```

**Acceptance Criteria:**
- `internal/version/` directory no longer exists.
- No `.go` file imports `github.com/cwlls/pixe-go/internal/version`.
- `go build ./...` succeeds.
- `go vet ./...` reports no issues.

---

## Task 46 — Pipeline — Switch to `cmd.Version()`

**Goal:** Replace all references to `version.Version` in the pipeline package with `cmd.Version()`. This is the critical wiring change that connects the pipeline to the new version source.

**Architecture Reference:** Section 3.4 (Consumers table)

**Depends on:** Task 44 (`cmd.Version()` getter exists)

**Import cycle concern:** `cmd` imports `pipeline`, so `pipeline` **cannot** import `cmd`. The solution is to pass the version string into the pipeline via `SortOptions`, rather than having the pipeline call `cmd.Version()` directly.

### Files to modify

#### 1. `internal/pipeline/pipeline.go`

**Add `PixeVersion` field to `SortOptions`:**

```go
// SortOptions holds the resolved runtime options for a sort run.
type SortOptions struct {
	Config       *config.AppConfig
	Hasher       *hash.Hasher
	Registry     *discovery.Registry
	RunTimestamp string
	Output       io.Writer
	PixeVersion  string   // ← NEW: version string for manifest/ledger stamping
}
```

**Remove the `internal/version` import** from `pipeline.go`.

**Replace `version.Version` with `opts.PixeVersion`** in the `Run()` function:

```go
// Line ~91 — manifest creation:
// BEFORE:
PixeVersion: version.Version,
// AFTER:
PixeVersion: opts.PixeVersion,

// Line ~147 — ledger creation:
// BEFORE:
PixeVersion: version.Version,
// AFTER:
PixeVersion: opts.PixeVersion,
```

#### 2. `internal/pipeline/worker.go`

**Remove the `internal/version` import** from `worker.go`.

**Replace `version.Version` with the version from the options/manifest** in all ledger creation sites:

```go
// Line ~122 — RunConcurrent ledger creation:
// BEFORE:
PixeVersion: version.Version,
// AFTER:
PixeVersion: m.PixeVersion,
// (The manifest was already populated with the version in Run(), so
//  the concurrent path can read it from there.)

// Line ~380 — runSequential ledger creation:
// BEFORE:
PixeVersion: version.Version,
// AFTER:
PixeVersion: m.PixeVersion,
```

#### 3. `cmd/sort.go`

**Wire `cmd.Version()` into `SortOptions`:**

```go
opts := pipeline.SortOptions{
	Config:       cfg,
	Hasher:       h,
	Registry:     reg,
	RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
	Output:       os.Stdout,
	PixeVersion:  Version(),   // ← NEW: pass version into pipeline
}
```

Since `cmd/sort.go` is in the `cmd` package, it can call `Version()` directly (same package).

**Acceptance Criteria:**
- No file in `internal/pipeline/` imports `github.com/cwlls/pixe-go/internal/version`.
- `pipeline.SortOptions` has a `PixeVersion string` field.
- `cmd/sort.go` passes `Version()` into `SortOptions.PixeVersion`.
- After a `pixe sort` run (dev build), `manifest.json` contains `"pixe_version": "dev"` and `ledger.json` contains `"pixe_version": "dev"`.
- `go build ./...` succeeds.
- All existing pipeline tests pass (they will need to set `PixeVersion` in their `SortOptions` — use `"test"` or `"dev"`).

---

## Task 47 — Makefile — Delegate to GoReleaser

**Goal:** Rewrite the Makefile so that `build` and `install` targets delegate to `goreleaser build` instead of running `go build` with hand-crafted ldflags. This ensures the Makefile and GoReleaser use identical build logic.

**Architecture Reference:** Section 3.2 (Build Tooling — Makefile)

**Depends on:** Task 44 (version vars exist in `cmd` at the ldflags target paths)

**File to rewrite: `Makefile`**

Replace the variables and build targets. The full updated Makefile:

```makefile
# ============================================================
# Makefile — pixe-go
# module: github.com/cwlls/pixe-go
# ============================================================

# ---------- variables ---------------------------------------
BINARY      := pixe
BUILD_DIR   := .

# Test flags
TEST_FLAGS  := -race -timeout 120s
COVER_OUT   := coverage.out
COVER_HTML  := coverage.html

# Tools
GOLANGCI    := golangci-lint

# ---------- default target ----------------------------------
.DEFAULT_GOAL := help

# ---------- phony targets -----------------------------------
.PHONY: help build build-debug run clean test test-unit test-integration test-all \
        test-cover test-cover-html lint vet fmt fmt-check tidy deps check install uninstall

# ---------- help --------------------------------------------
help: ## Show this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---------- build -------------------------------------------
build: ## Build pixe for the current platform via GoReleaser
	goreleaser build --single-target --snapshot --clean -o $(BUILD_DIR)/$(BINARY)

build-debug: ## Build without stripping symbols (for dlv) — bypasses GoReleaser
	go build -gcflags "all=-N -l" -o $(BUILD_DIR)/$(BINARY) .

# ---------- run ---------------------------------------------
run: build ## Build then run pixe with ARGS (e.g. make run ARGS="sort --help")
	./$(BINARY) $(ARGS)

# ---------- clean -------------------------------------------
clean: ## Remove build artifacts and coverage files
	rm -f $(BUILD_DIR)/$(BINARY)
	rm -rf dist/
	rm -f $(COVER_OUT) $(COVER_HTML)

# ---------- test --------------------------------------------
test: test-unit ## Alias for test-unit

test-unit: ## Run unit tests (excludes integration)
	go test $(TEST_FLAGS) $(shell go list ./... | grep -v '/integration')

test-integration: ## Run integration tests only (requires build)
	go test $(TEST_FLAGS) -v ./internal/integration/...

test-all: ## Run all tests including integration
	go test $(TEST_FLAGS) ./...

test-cover: ## Run unit tests with coverage report
	go test $(TEST_FLAGS) -coverprofile=$(COVER_OUT) -covermode=atomic \
	    $(shell go list ./... | grep -v '/integration')
	go tool cover -func=$(COVER_OUT)

test-cover-html: test-cover ## Open HTML coverage report in browser
	go tool cover -html=$(COVER_OUT) -o $(COVER_HTML)
	open $(COVER_HTML)

# ---------- code quality ------------------------------------
vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go source files
	gofmt -w -s .

fmt-check: ## Check formatting without modifying files (CI-safe)
	@out=$$(gofmt -l -s .); \
	if [ -n "$$out" ]; then \
	    echo "The following files need formatting:"; \
	    echo "$$out"; \
	    exit 1; \
	fi

lint: ## Run golangci-lint (install: brew install golangci-lint)
	$(GOLANGCI) run ./...

check: fmt-check vet test-unit ## Run fmt-check + vet + unit tests (fast CI gate)

# ---------- dependencies ------------------------------------
tidy: ## Run go mod tidy
	go mod tidy

deps: ## Download all module dependencies
	go mod download

# ---------- install / uninstall -----------------------------
install: build ## Install pixe to $GOPATH/bin
	cp $(BUILD_DIR)/$(BINARY) $(shell go env GOPATH)/bin/$(BINARY)

uninstall: ## Remove pixe from $GOPATH/bin
	rm -f $(shell go env GOPATH)/bin/$(BINARY)
```

**Key changes from the current Makefile:**

| Aspect | Before | After |
|---|---|---|
| `MODULE` variable | Used for ldflags path | Removed — GoReleaser owns ldflags |
| `COMMIT` variable | `git rev-parse --short HEAD` | Removed — GoReleaser computes this |
| `BUILD_DATE` variable | `date -u +%Y-%m-%dT%H:%M:%SZ` | Removed — GoReleaser computes this |
| `LDFLAGS` variable | Hand-crafted `-X` flags | Removed — defined in `.goreleaser.yaml` |
| `build` target | `go build -ldflags "$(LDFLAGS)"` | `goreleaser build --single-target --snapshot --clean -o ./pixe` |
| `build-debug` target | `go build -gcflags "all=-N -l"` | Unchanged — still raw `go build` for debugger |
| `install` target | `go install -ldflags "$(LDFLAGS)"` | `cp` after `goreleaser build` (GoReleaser doesn't support `go install`) |
| `clean` target | Removes binary + coverage | Also removes `dist/` (GoReleaser's output directory) |

**Acceptance Criteria:**
- `make build` invokes `goreleaser build --single-target --snapshot --clean -o ./pixe` and produces a working binary.
- `./pixe version` after `make build` prints a version string with a real commit hash (not `"unknown"`).
- `make build-debug` still produces an unstripped binary via raw `go build`.
- `make install` copies the binary to `$GOPATH/bin`.
- `make clean` removes the binary, `dist/`, and coverage files.
- `make help` displays all targets correctly.
- No `LDFLAGS`, `MODULE`, `COMMIT`, `BUILD_DATE`, or `VERSION` variables remain in the Makefile.

---

## Task 48 — GoReleaser — Fix ldflags Target

**Goal:** Update `.goreleaser.yaml` to inject version variables into `cmd.version`, `cmd.commit`, and `cmd.buildDate` (the new unexported vars in the `cmd` package) instead of the old `internal/version.Version`, `internal/version.Commit`, and `internal/version.BuildDate`.

**Architecture Reference:** Section 3.2 (GoReleaser configuration)

**Depends on:** Task 44 (the target vars exist in `cmd`)

**File to modify: `.goreleaser.yaml`**

```yaml
# BEFORE (line 25):
    ldflags:
      - -s -w -X github.com/cwlls/pixe-go/internal/version.Version={{.Version}} -X github.com/cwlls/pixe-go/internal/version.Commit={{.Commit}} -X github.com/cwlls/pixe-go/internal/version.BuildDate={{.Date}}

# AFTER:
    ldflags:
      - >-
        -s -w
        -X github.com/cwlls/pixe-go/cmd.version={{.Version}}
        -X github.com/cwlls/pixe-go/cmd.commit={{.Commit}}
        -X github.com/cwlls/pixe-go/cmd.buildDate={{.Date}}
```

**This also fixes the existing bug:** The old config tried to inject into `version.Version` which was a `const` — ldflags `-X` can only set `var`s, so the injection was silently ignored. The new targets are all `var`s.

**Additional cleanup:** Reformat the ldflags as a YAML block scalar (`>-`) for readability, matching the style shown in the Architecture doc.

**Acceptance Criteria:**
- `.goreleaser.yaml` ldflags target `cmd.version`, `cmd.commit`, and `cmd.buildDate`.
- No reference to `internal/version` remains in `.goreleaser.yaml`.
- `goreleaser check` passes (validates the config).
- `goreleaser build --single-target --snapshot` produces a binary where `./pixe version` shows a snapshot version string with a real commit hash and build date.

---

## Task 49 — Tests & Verification — Version Refactor

**Goal:** Verify the entire version refactor is correct: delete stale version tests, update test fixtures that reference version strings, and run the full test suite.

**Architecture Reference:** Section 3 (Version Management)

**Depends on:** Tasks 44, 45, 46, 47, 48

### 1. Deleted files (already done in Task 45)

- `internal/version/version_test.go` — no longer exists.
- `internal/version/version.go` — no longer exists.

### 2. Update manifest test fixtures

**File: `internal/manifest/manifest_test.go`**

The `sampleManifest()` and `sampleLedger()` helpers currently set `PixeVersion: "0.9.0"`. Update to `"test"`:

```go
// In sampleManifest() (~line 32):
// BEFORE:
PixeVersion: "0.9.0",
// AFTER:
PixeVersion: "test",

// In sampleLedger() (~line 55):
// BEFORE:
PixeVersion: "0.9.0",
// AFTER:
PixeVersion: "test",
```

The round-trip assertions (`got.PixeVersion != m.PixeVersion`) remain valid — they test serialization, not the specific version value.

### 3. Update pipeline test fixtures

Any pipeline test that constructs `SortOptions` must now include the `PixeVersion` field:

```go
opts := pipeline.SortOptions{
	Config:       cfg,
	Hasher:       h,
	Registry:     reg,
	RunTimestamp: "20260306_103000",
	Output:       &buf,
	PixeVersion:  "test",   // ← ADD
}
```

Search for all test files that construct `SortOptions`:
```bash
rg 'SortOptions{' --include '*_test.go'
```

### 4. Verification commands

```bash
# No stale imports:
rg 'internal/version' --include '*.go'
# Should return zero matches.

# No stale ldflags references:
rg 'internal/version' Makefile .goreleaser.yaml
# Should return zero matches.

# Full build + test:
go vet ./...
go build ./...
go test -race -timeout 120s ./...
make lint

# Build smoke test:
make build
./pixe version
# Should print: pixe v<snapshot-version> (commit: <hash>, built: <date>)

# Debug build smoke test:
make build-debug
./pixe version
# Should print: pixe vdev (commit: unknown, built: unknown)

# GoReleaser validation:
goreleaser check
```

### Acceptance Criteria

- `internal/version/` directory does not exist.
- No `.go` file imports `github.com/cwlls/pixe-go/internal/version`.
- No reference to `internal/version` in `Makefile` or `.goreleaser.yaml`.
- `go vet ./...` — zero warnings.
- `go build ./...` — compiles cleanly.
- `go test -race -timeout 120s ./...` — all tests pass.
- `make lint` — 0 issues.
- `make build && ./pixe version` — prints version with real commit hash.
- `make build-debug && ./pixe version` — prints `pixe vdev (commit: unknown, built: unknown)`.
- `goreleaser check` — passes.
- Manifest round-trip tests still assert `PixeVersion` survives serialization.
- Pipeline tests pass with `PixeVersion: "test"` in `SortOptions`.
