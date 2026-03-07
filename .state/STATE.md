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
| 8 | Copy & Verify Engine | High | @developer | ⬜ Pending | 3, 4, 6 | Streamed copy, post-copy re-hash, manifest updates |
| 9 | Sort Pipeline Orchestrator | High | @developer | ⬜ Pending | 5, 7, 8 | Single-threaded first: discover → extract → hash → copy → verify |
| 10 | CLI: `pixe sort` Command | High | @developer | ⬜ Pending | 9 | Cobra command, Viper flag binding, dry-run mode |
| 11 | Worker Pool & Concurrency | Medium | @developer | ⬜ Pending | 9 | Coordinator + N workers, configurable --workers |
| 12 | HEIC Filetype Module | Medium | @developer | ⬜ Pending | 7 | Second handler — validates contract generality |
| 13 | MP4 Filetype Module | Medium | @developer | ⬜ Pending | 7 | Third handler — video keyframe hashing |
| 14 | Metadata Tagging Engine | Medium | @developer | ⬜ Pending | 7, 8 | Copyright template, CameraOwner injection post-verify |
| 15 | CLI: `pixe verify` Command | Medium | @developer | ⬜ Pending | 3, 5, 10 | Walk dirB, parse filename checksum, report mismatches |
| 16 | CLI: `pixe resume` Command | Medium | @developer | ⬜ Pending | 4, 9, 10 | Load manifest, skip completed, re-enter pipeline |
| 17 | Integration Tests & Safety Audit | High | @tester | ⬜ Pending | 10, 15, 16 | End-to-end with fixture files, interrupt simulation |

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
