# Implementation

## v2.4.0 — I2: Extended Hash Algorithms + Algorithm-Tagged Filenames

### Completed (pre-I2, included in v2.4.0)

| #   | Task                                                          | Priority | Agent      | Status | Depends On | Notes                                    |
|:----|:--------------------------------------------------------------|:---------|:-----------|:-------|:-----------|:-----------------------------------------|
| 13  | A4 — Implement standalone TIFF handler                        | 1        | @developer | [x]    | —          | Wave 1                                   |
| 14  | A2 — AVIF EXIF extraction spike                               | 2        | @developer | [x]    | —          | Wave 1 — custom parser needed            |
| 15  | A2 — Implement AVIF handler                                   | 3        | @developer | [x]    | 14         | Wave 2                                   |
| 16  | Consolidate handler registration (`resume.go`, `status.go`)   | 4        | @developer | [x]    | 13, 15     | Wave 3                                   |
| 17  | Register TIFF + AVIF handlers in `buildRegistry()`            | 5        | @developer | [x]    | 13, 15, 16 | Wave 3                                   |
| 18  | Verify all tests pass (`make check`)                          | 6        | @tester    | [x]    | 16, 17     | Wave 4 — complete                        |
| 19  | Commit all changes                                            | 7        | @committer | [x]    | 18         | Wave 5 — complete                        |

---

### I2 Task Summary

| #   | Task                                                          | Priority | Agent      | Status | Depends On | Notes                                    |
|:----|:--------------------------------------------------------------|:---------|:-----------|:-------|:-----------|:-----------------------------------------|
| 20  | Add new dependencies (`blake3`, `xxhash`)                     | 1        | @developer | [x]    | —          | Wave 1                                   |
| 21  | Extend `internal/hash` with MD5, BLAKE3, xxHash               | 2        | @developer | [x]    | 20         | Wave 2                                   |
| 22  | Add `AlgorithmID()` to `Hasher` + algorithm registry          | 3        | @developer | [x]    | 21         | Wave 2                                   |
| 23  | Update `pathbuilder.Build()` for new filename format          | 4        | @developer | [x]    | 22         | Wave 3                                   |
| 24  | Update `verify.parseChecksum()` for dual-format detection     | 5        | @developer | [x]    | 22         | Wave 3                                   |
| 25  | Add `algorithm` column to `files` table (schema v3 → v4)     | 6        | @developer | [x]    | —          | Wave 1                                   |
| 26  | Wire `algorithm` through DB writes (`files.go`)               | 7        | @developer | [x]    | 22, 25     | Wave 3                                   |
| 27  | Update pipeline to pass algorithm ID to `pathbuilder`         | 8        | @developer | [x]    | 23, 26     | Wave 4                                   |
| 28  | Update `pixe verify` for auto-detection                       | 9        | @developer | [x]    | 24         | Wave 4                                   |
| 29  | Update CLI flag help text and doc comments                    | 10       | @developer | [x]    | 21         | Wave 2                                   |
| 30  | Bump ledger version to v5                                     | 11       | @developer | [x]    | 22         | Wave 3                                   |
| 31  | Tests: hash, pathbuilder, verify, schema migration            | 12       | @tester    | [x]    | 21–30      | Wave 5                                   |
| 32  | Run `make check` — full validation                            | 13       | @tester    | [x]    | 31         | Wave 6                                   |
| 33  | Commit all I2 changes                                         | 14       | @committer | [~]    | 32         | Wave 7                                   |

---

### Parallelization Strategy

#### Wave 1 — Independent foundation (tasks 20, 25)
No file overlap. Task 20 adds Go module dependencies. Task 25 adds the `algorithm` column to the DB schema. Both are independent.

#### Wave 2 — Hash engine + doc updates (tasks 21, 22, 29)
Task 21 extends `NewHasher()` with three new algorithms. Task 22 adds the `AlgorithmID()` method and the algorithm-to-ID registry. Task 29 updates CLI help text and doc comments. All three touch different files and can run in parallel once Wave 1 completes.

#### Wave 3 — Format changes (tasks 23, 24, 26, 30)
Task 23 changes `pathbuilder.Build()` to accept an algorithm ID. Task 24 updates `verify.parseChecksum()` for dual-format detection. Task 26 wires the algorithm through DB writes. Task 30 bumps the ledger version. These touch different packages and can run in parallel once Wave 2 completes.

#### Wave 4 — Integration wiring (tasks 27, 28)
Task 27 updates the pipeline callers of `pathbuilder.Build()` to pass the algorithm ID. Task 28 updates `pixe verify` to use auto-detection. Both depend on Wave 3 but are independent of each other.

#### Wave 5 — Tests (task 31)
Comprehensive test updates across all changed packages.

#### Wave 6 — Validation (task 32)
`make check` — fmt, vet, all tests with `-race`.

#### Wave 7 — Commit (task 33)

---

### Task Descriptions

#### Task 20 — Add new dependencies (`blake3`, `xxhash`)

**Commands:**
```bash
go get github.com/zeebo/blake3
go get github.com/cespare/xxhash/v2
go get crypto/md5  # stdlib, no go get needed
go mod tidy
```

**Acceptance criteria:**
- `go.mod` and `go.sum` updated with `github.com/zeebo/blake3` and `github.com/cespare/xxhash/v2`
- `go mod tidy` clean

---

#### Task 21 — Extend `internal/hash` with MD5, BLAKE3, xxHash

**File:** `internal/hash/hasher.go`

**Changes:**

1. Add imports:
   ```go
   "crypto/md5"    //nolint:gosec // MD5 is used for content identification, not security
   "crypto/sha1"   //nolint:gosec // SHA-1 is used for filename checksums, not security
   "crypto/sha256"
   "encoding/hex"
   "fmt"
   gohash "hash"
   "io"

   "github.com/cespare/xxhash/v2"
   "github.com/zeebo/blake3"
   ```

2. Update `NewHasher()` switch to add three new cases:
   ```go
   func NewHasher(algorithm string) (*Hasher, error) {
       switch algorithm {
       case "md5":
           return &Hasher{newFunc: md5.New, name: "md5"}, nil //nolint:gosec
       case "sha1":
           return &Hasher{newFunc: sha1.New, name: "sha1"}, nil //nolint:gosec
       case "sha256":
           return &Hasher{newFunc: sha256.New, name: "sha256"}, nil
       case "blake3":
           return &Hasher{newFunc: func() gohash.Hash { return blake3.New() }, name: "blake3"}, nil
       case "xxhash":
           return &Hasher{newFunc: func() gohash.Hash { return xxhash.New() }, name: "xxhash"}, nil
       default:
           return nil, fmt.Errorf("hash: unsupported algorithm %q (supported: md5, sha1, sha256, blake3, xxhash)", algorithm)
       }
   }
   ```

   **Note on `blake3.New()` and `xxhash.New()`:** Both return types that implement `hash.Hash`. `zeebo/blake3` returns `*blake3.Hasher` which implements `hash.Hash`. `cespare/xxhash/v2` returns `*xxhash.Digest` which implements `hash.Hash`. Both need wrapper closures because their `New()` functions return concrete types, not `hash.Hash`.

3. Update the doc comment on `NewHasher` to list all five algorithms.

**Acceptance criteria:**
- `go build ./internal/hash/...` compiles
- `go vet ./internal/hash/...` clean

---

#### Task 22 — Add `AlgorithmID()` to `Hasher` + algorithm registry

**File:** `internal/hash/hasher.go`

**Changes:**

1. Add an `id` field to the `Hasher` struct:
   ```go
   type Hasher struct {
       newFunc func() gohash.Hash
       name    string
       id      int // numeric algorithm ID for filename embedding (Section 4.5.1)
   }
   ```

2. Update each `NewHasher` case to set the `id`:
   ```go
   case "md5":
       return &Hasher{newFunc: md5.New, name: "md5", id: 0}, nil
   case "sha1":
       return &Hasher{newFunc: sha1.New, name: "sha1", id: 1}, nil
   case "sha256":
       return &Hasher{newFunc: sha256.New, name: "sha256", id: 2}, nil
   case "blake3":
       return &Hasher{newFunc: func() gohash.Hash { return blake3.New() }, name: "blake3", id: 3}, nil
   case "xxhash":
       return &Hasher{newFunc: func() gohash.Hash { return xxhash.New() }, name: "xxhash", id: 4}, nil
   ```

3. Add `AlgorithmID()` method:
   ```go
   // AlgorithmID returns the numeric algorithm identifier embedded in filenames.
   // See ARCHITECTURE.md Section 4.5.1 for the registry.
   func (h *Hasher) AlgorithmID() int { return h.id }
   ```

4. Add a package-level lookup function for verify auto-detection:
   ```go
   // AlgorithmNameByID returns the algorithm name for a numeric ID, or "" if unknown.
   // Used by verify to auto-detect the algorithm from a filename's embedded ID.
   func AlgorithmNameByID(id int) string {
       switch id {
       case 0:
           return "md5"
       case 1:
           return "sha1"
       case 2:
           return "sha256"
       case 3:
           return "blake3"
       case 4:
           return "xxhash"
       default:
           return ""
       }
   }
   ```

**Acceptance criteria:**
- `h.AlgorithmID()` returns the correct numeric ID for each algorithm
- `AlgorithmNameByID(id)` round-trips correctly for all five algorithms
- `AlgorithmNameByID(99)` returns `""`

---

#### Task 23 — Update `pathbuilder.Build()` for new filename format

**File:** `internal/pathbuilder/pathbuilder.go`

**Changes:**

1. Update `Build()` signature to accept an algorithm ID:
   ```go
   func Build(date time.Time, algoID int, checksum string, ext string, isDuplicate bool, runTimestamp string) string
   ```

2. Update the `fmt.Sprintf` format string to use the new delimiter convention:
   ```go
   filename := fmt.Sprintf("%04d%02d%02d_%02d%02d%02d-%d-%s%s",
       year, int(date.Month()), date.Day(),
       date.Hour(), date.Minute(), date.Second(),
       algoID, checksum, ext,
   )
   ```

   This produces: `20211225_062223-1-7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg`

3. Update the doc comment and examples:
   ```go
   // Build(t, 1, sha, ".jpg", false, "") → "2021/12-Dec/20211225_062223-1-<sha>.jpg"
   // Build(t, 1, sha, ".JPG", true, "20260306_103000") → "duplicates/20260306_103000/2021/12-Dec/20211225_062223-1-<sha>.jpg"
   ```

**Acceptance criteria:**
- `Build()` produces filenames in the new `YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>.<ext>` format
- Existing tests updated to match new signature and expected output
- `go test -race -timeout 120s ./internal/pathbuilder/...` passes

---

#### Task 24 — Update `verify.parseChecksum()` for dual-format detection

**File:** `internal/verify/verify.go`

**Changes:**

1. Rewrite `parseChecksum()` to return the algorithm name alongside the checksum:
   ```go
   // parseChecksum extracts the checksum and algorithm from a Pixe filename.
   //
   // New format: YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>.<ext>
   //   Detected by '-' at position 15 of the stem. Returns the algorithm name
   //   resolved from the numeric ID via hash.AlgorithmNameByID().
   //
   // Legacy format: YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
   //   Detected by '_' at position 15 of the stem. Algorithm inferred from
   //   digest length: 40 = "sha1", 64 = "sha256". Returns "" if ambiguous.
   func parseChecksum(filename string) (checksum string, algorithm string, ok bool) {
       ext := filepath.Ext(filename)
       base := strings.TrimSuffix(filename, ext)

       if len(base) < 16 {
           return "", "", false
       }

       switch base[15] {
       case '-':
           // New format: YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>
           rest := base[16:] // after the first '-'
           dashIdx := strings.IndexByte(rest, '-')
           if dashIdx < 1 {
               return "", "", false
           }
           idStr := rest[:dashIdx]
           checksum = rest[dashIdx+1:]
           if len(checksum) < 8 {
               return "", "", false
           }
           // Parse numeric ID.
           id := 0
           for _, c := range idStr {
               if c < '0' || c > '9' {
                   return "", "", false
               }
               id = id*10 + int(c-'0')
           }
           algo := hash.AlgorithmNameByID(id)
           if algo == "" {
               return "", "", false
           }
           return checksum, algo, true

       case '_':
           // Legacy format: YYYYMMDD_HHMMSS_<CHECKSUM>
           checksum = base[16:]
           if len(checksum) < 8 {
               return "", "", false
           }
           switch len(checksum) {
           case 40:
               return checksum, "sha1", true
           case 64:
               return checksum, "sha256", true
           default:
               return checksum, "", true // ambiguous — caller must use --algorithm
           }

       default:
           return "", "", false
       }
   }
   ```

2. Update all callers of `parseChecksum()` within `verify.go` to handle the new return signature (3 values instead of 2).

3. Add `"github.com/cwlls/pixe-go/internal/hash"` to the imports.

**Acceptance criteria:**
- New-format filenames: algorithm auto-detected from ID
- Legacy filenames: algorithm inferred from length (40=sha1, 64=sha256)
- Unknown legacy lengths: returns empty algorithm string
- `go test -race -timeout 120s ./internal/verify/...` passes

---

#### Task 25 — Add `algorithm` column to `files` table (schema v3 → v4)

**File:** `internal/archivedb/schema.go`

**Changes:**

1. Bump `schemaVersion` from `3` to `4`.

2. Add `algorithm TEXT` column to the `files` DDL (after `checksum`):
   ```sql
   checksum      TEXT,
   algorithm     TEXT,               -- hash algorithm name, e.g. "sha1", "blake3" (NULL until hashed)
   ```

3. Add v3 → v4 migration block in `migrateSchema()`:
   ```go
   // v3 → v4: add algorithm to files, backfill from parent run.
   if currentVersion < 4 {
       migrations := []string{
           `ALTER TABLE files ADD COLUMN algorithm TEXT`,
       }
       for _, m := range migrations {
           if _, err := db.conn.Exec(m); err != nil {
               if !strings.Contains(err.Error(), "duplicate column") {
                   return fmt.Errorf("archivedb: migrate v3→v4: %w", err)
               }
           }
       }
       // Backfill: set algorithm from the parent run for all existing rows.
       if _, err := db.conn.Exec(
           `UPDATE files SET algorithm = (SELECT algorithm FROM runs WHERE runs.id = files.run_id) WHERE files.algorithm IS NULL`,
       ); err != nil {
           return fmt.Errorf("archivedb: migrate v3→v4 backfill: %w", err)
       }
       _, _ = db.conn.Exec(
           `INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (?, ?)`,
           4, time.Now().UTC().Format(time.RFC3339),
       )
   }
   ```

**File:** `internal/archivedb/files.go`

4. Add `Algorithm *string` field to `FileRecord` struct (after `Checksum`).

5. Update `scanFileRecord()` to scan the new column.

**Acceptance criteria:**
- Fresh databases created at schema v4 with `algorithm` column
- Existing v3 databases migrated: column added, backfilled from `runs.algorithm`
- `go test -race -timeout 120s ./internal/archivedb/...` passes

---

#### Task 26 — Wire `algorithm` through DB writes (`files.go`)

**File:** `internal/archivedb/files.go`

**Changes:**

1. Add `WithAlgorithm(algorithm string) UpdateOption`:
   ```go
   // WithAlgorithm sets the algorithm field on a file status update.
   func WithAlgorithm(algorithm string) UpdateOption {
       return func(p *updateParams) { p.algorithm = &algorithm }
   }
   ```

2. Add `algorithm *string` to `updateParams` struct.

3. Add the `algorithm` SET clause in `UpdateFileStatus()`:
   ```go
   if p.algorithm != nil {
       setClauses = append(setClauses, "algorithm = ?")
       args = append(args, *p.algorithm)
   }
   ```

**Acceptance criteria:**
- `WithAlgorithm("blake3")` correctly sets the column on update
- `go test -race -timeout 120s ./internal/archivedb/...` passes

---

#### Task 27 — Update pipeline to pass algorithm ID to `pathbuilder`

**Files:** `internal/pipeline/pipeline.go`, `internal/pipeline/worker.go`

**Changes:**

1. Update all four `pathbuilder.Build()` call sites to pass `opts.Hasher.AlgorithmID()` as the second argument:

   **`pipeline.go` line 545:**
   ```go
   relDest := pathbuilder.Build(captureDate, opts.Hasher.AlgorithmID(), checksum, ext, isDuplicate, opts.RunTimestamp)
   ```

   **`pipeline.go` line 698:**
   ```go
   dupRelDest := pathbuilder.Build(captureDate, opts.Hasher.AlgorithmID(), checksum, ext, true, opts.RunTimestamp)
   ```

   **`worker.go` line 350:**
   ```go
   relDest := pathbuilder.Build(wr.date, opts.Hasher.AlgorithmID(), wr.checksum, wr.ext, isDuplicate, opts.RunTimestamp)
   ```

   **`worker.go` line 453:**
   ```go
   dupRelDest := pathbuilder.Build(fr.verifiedAt, opts.Hasher.AlgorithmID(), fr.checksum, ...)
   ```

2. At each call site where `WithChecksum(checksum)` is passed to `UpdateFileStatus()`, also pass `WithAlgorithm(opts.Hasher.Algorithm())`. Search for all `WithChecksum` usages in `pipeline.go` and `worker.go` and add the corresponding `WithAlgorithm`.

**Acceptance criteria:**
- All `pathbuilder.Build()` calls pass the algorithm ID
- All `UpdateFileStatus()` calls that set a checksum also set the algorithm
- `go build ./internal/pipeline/...` compiles

---

#### Task 28 — Update `pixe verify` for auto-detection

**File:** `cmd/verify.go`

**Changes:**

1. Update the verify command to use the algorithm returned by `parseChecksum()` when available, falling back to the `--algorithm` flag:

   The `Run()` function in `internal/verify/verify.go` currently takes a single `Hasher` in `Options`. Update it to support per-file algorithm selection:

   **Option A (simpler):** Keep the existing `Hasher` in `Options` as the default/fallback. In the per-file verification loop, if `parseChecksum()` returns a non-empty algorithm that differs from the current hasher, create a new hasher for that file. Cache hashers by algorithm name to avoid repeated allocation.

   **Implementation in `verify.go`:**
   ```go
   // Add a hasher cache to Run():
   hashers := map[string]*hash.Hasher{
       opts.Hasher.Algorithm(): opts.Hasher,
   }

   // In the per-file loop, after parseChecksum():
   checksum, algo, ok := parseChecksum(filename)
   if !ok { ... }
   h := opts.Hasher // default
   if algo != "" && algo != opts.Hasher.Algorithm() {
       if cached, exists := hashers[algo]; exists {
           h = cached
       } else {
           newH, err := hash.NewHasher(algo)
           if err != nil { ... }
           hashers[algo] = newH
           h = newH
       }
   }
   actual, err := h.Sum(rc)
   ```

2. Update `cmd/verify.go` to change the `--algorithm` default behavior. The flag still exists as a fallback but is no longer required. Update the flag description.

**Acceptance criteria:**
- New-format files verified without `--algorithm` flag
- Legacy files verified via length inference
- Mixed-algorithm archives verified correctly in a single pass
- `go test -race -timeout 120s ./internal/verify/...` passes

---

#### Task 29 — Update CLI flag help text and doc comments

**Files to update (doc comments and help strings only):**

1. **`cmd/root.go`** — Update `--algorithm` flag help text:
   ```go
   rootCmd.PersistentFlags().StringP("algorithm", "a", "sha1",
       "hash algorithm: md5, sha1 (default), sha256, blake3, xxhash")
   ```

2. **`internal/config/config.go`** — Update `Algorithm` field doc comment:
   ```go
   // Algorithm is the name of the hash algorithm to use.
   // Supported values: "md5", "sha1" (default), "sha256", "blake3", "xxhash".
   Algorithm string
   ```

3. **`internal/domain/pipeline.go`** — Update `Algorithm` field doc comments on `LedgerHeader` and `Manifest` structs to list all five algorithms.

**Acceptance criteria:**
- All doc comments and help strings list the five supported algorithms
- `go vet ./...` clean

---

#### Task 30 — Bump ledger version to v5

**File:** `internal/manifest/manifest.go` (or wherever `LedgerVersion` is defined)

**Changes:**

1. Find the ledger version constant and bump from `4` to `5`.
2. Update any doc comments referencing the version number.

**Note:** The ledger format itself doesn't change structurally — the version bump signals that destination filenames in the ledger now use the new `YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>` format. Consumers parsing the ledger should be aware that `destination` field values have a different filename structure starting at v5.

**Acceptance criteria:**
- Ledger header emits `"version": 5`
- `go build ./internal/manifest/...` compiles

---

#### Task 31 — Tests: hash, pathbuilder, verify, schema migration

**Files:**

1. **`internal/hash/hasher_test.go`:**
   - Add `"md5"`, `"blake3"`, `"xxhash"` to `TestNewHasher_supported`
   - Remove `"md5"` from `TestNewHasher_unsupported` (it's now supported)
   - Add `TestHasher_Sum_md5` with known digest of empty string (`d41d8cd98f00b204e9800998ecf8427e`)
   - Add `TestHasher_Sum_blake3` with known digest of empty string
   - Add `TestHasher_Sum_xxhash` with known digest of empty string
   - Add digest length entries to `TestHasher_Sum_outputFormat`: `md5`=32, `blake3`=64, `xxhash`=16
   - Add `TestHasher_AlgorithmID` — verify each algorithm returns the correct numeric ID
   - Add `TestAlgorithmNameByID` — verify round-trip for all five IDs + unknown ID returns `""`

2. **`internal/pathbuilder/pathbuilder_test.go`:**
   - Update existing tests to pass the algorithm ID parameter (use `1` for SHA-1 to match existing expected outputs, but update expected filenames to new format)
   - Add test cases for different algorithm IDs (0, 2, 3, 4) to verify the ID appears correctly in the filename

3. **`internal/verify/verify_test.go`:**
   - Update `TestParseChecksum_validFormats` with new-format test cases:
     - `20211225_062223-1-7d97e98f8af710c7e7fe703abc8f639e0ee507c4.jpg` → checksum + algo `"sha1"`
     - `20211225_062223-0-d41d8cd98f00b204e9800998ecf8427e.jpg` → checksum + algo `"md5"`
     - `20211225_062223-3-<64chars>.jpg` → checksum + algo `"blake3"`
     - `20211225_062223-4-a1b2c3d4e5f6a7b8.jpg` → checksum + algo `"xxhash"`
   - Add legacy-format test cases:
     - `20211225_062223_<40chars>.jpg` → checksum + algo `"sha1"` (length inference)
     - `20211225_062223_<64chars>.jpg` → checksum + algo `"sha256"` (length inference)
   - Add edge cases: unknown ID (`-9-`), malformed formats
   - Update existing tests for the new 3-return-value signature

4. **`internal/archivedb/schema_test.go`** (or equivalent):
   - Add test for v3 → v4 migration: create a v3 database, run migration, verify `algorithm` column exists and is backfilled from `runs.algorithm`

**Acceptance criteria:**
- All new and updated tests pass with `-race`
- Coverage of all five algorithms in hash tests
- Coverage of both filename formats in verify tests
- Coverage of schema migration path

---

#### Task 32 — Run `make check` — full validation

```bash
make check    # fmt-check + vet + unit tests (with -race)
```

If any failures:
- Fix formatting issues (`make fmt`)
- Fix vet warnings
- Fix test failures
- Re-run until clean

Also run targeted tests:
```bash
go test -race -timeout 120s ./internal/hash/... -v
go test -race -timeout 120s ./internal/pathbuilder/... -v
go test -race -timeout 120s ./internal/verify/... -v
go test -race -timeout 120s ./internal/archivedb/... -v
go test -race -timeout 120s ./internal/pipeline/... -v
```

**Acceptance criteria:**
- `make check` exits 0
- All targeted tests pass with `-race`
- No regressions in existing tests

---

#### Task 33 — Commit all I2 changes

Commit the I2 feature as a single conventional commit.

**Suggested message:**
```
feat: add MD5, BLAKE3, xxHash algorithms with algorithm-tagged filenames (I2)
```

**Acceptance criteria:**
- Clean `git status` after commit
- Commit message follows project conventions
