# Implementation State

## Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Create `cmd/status.go` — Cobra command skeleton with flag binding | high | @developer | [ ] pending | — | Flags: `--source`/`-s`, `--recursive`/`-r`, `--ignore`, `--json` |
| 2  | Implement status classification logic in `runStatus` | high | @developer | [ ] pending | 1 | Walk → load ledger → classify into 5 categories |
| 3  | Implement human-readable output formatter | high | @developer | [ ] pending | 2 | Sectioned listing with summary line per §7.4.4 |
| 4  | Implement JSON output formatter | medium | @developer | [ ] pending | 2 | Single JSON object per §7.4.5 |
| 5  | Write unit tests for `cmd/status.go` | high | @developer | [ ] pending | 3, 4 | Cover: no ledger, all sorted, mixed, no files, recursive |
| 6  | Write integration test for `pixe status` | medium | @tester | [ ] pending | 5 | End-to-end: sort then status, verify output |
| 7  | Run `make check` — ensure lint, vet, and all tests pass | high | @developer | [ ] pending | 5 | Gate before commit |
| 8  | Commit all changes | medium | @committer | [ ] pending | 7 | Architecture + implementation |

---

# Task Descriptions

## Task 1 — Create `cmd/status.go` Cobra command skeleton

**File:** `cmd/status.go`

Create the `pixe status` Cobra command following the exact patterns established by `cmd/sort.go` and `cmd/query.go`.

### Requirements

1. **Copyright header** — Apache 2.0 block identical to all other `.go` files.

2. **Command definition:**
   ```go
   var statusCmd = &cobra.Command{
       Use:   "status",
       Short: "Show the sorting status of a source directory",
       Long:  `...`,  // describe ledger-based status check
       RunE:  runStatus,
   }
   ```

3. **Flags** (in `init()`, registered on `statusCmd`):
   - `--source` / `-s` (string, required) — source directory to inspect. Use `statusCmd.Flags().StringP("source", "s", ...)`.
   - `--recursive` / `-r` (bool, default false) — recursively inspect subdirectories. Use `statusCmd.Flags().BoolP("recursive", "r", ...)`.
   - `--ignore` (string array, repeatable) — glob patterns. Use `statusCmd.Flags().StringArray("ignore", ...)`.
   - `--json` (bool, default false) — JSON output mode. Use `statusCmd.Flags().Bool("json", ...)`.

4. **Viper binding** — Bind each flag to Viper using `status_`-prefixed keys to avoid collision with `sort`'s bindings (same pattern as `query.go` uses `query_dir`):
   - `status_source`, `status_recursive`, `status_ignore`, `status_json`

5. **Registration:** `rootCmd.AddCommand(statusCmd)` in `init()`.

6. **Mark required:** `_ = statusCmd.MarkFlagRequired("source")`

7. **Imports** — will need:
   - `"fmt"`, `"os"`, `"path/filepath"`
   - `"github.com/spf13/cobra"`, `"github.com/spf13/viper"`
   - `"github.com/cwlls/pixe-go/internal/discovery"`
   - `"github.com/cwlls/pixe-go/internal/ignore"`
   - `"github.com/cwlls/pixe-go/internal/manifest"`
   - All 9 handler packages (same import block as `sort.go` lines 32–39)

8. **`runStatus` stub** — initially return `nil`; Task 2 fills in the body.

---

## Task 2 — Implement status classification logic

**File:** `cmd/status.go` (inside `runStatus`)

### Algorithm (per §7.4.2)

```go
func runStatus(cmd *cobra.Command, args []string) error {
    // 1. Read flags from Viper.
    source    := viper.GetString("status_source")
    recursive := viper.GetBool("status_recursive")
    ignorePatterns := viper.GetStringSlice("status_ignore")
    jsonFlag  := viper.GetBool("status_json")

    // 2. Validate source directory exists and is a directory.
    abs, err := filepath.Abs(source)
    // ... os.Stat, IsDir checks (same pattern as sort.go lines 91-97)

    // 3. Build handler registry (identical to sort.go lines 120-129).
    reg := discovery.NewRegistry()
    reg.Register(jpeghandler.New())
    // ... all 9 handlers

    // 4. Walk dirA.
    walkOpts := discovery.WalkOptions{
        Recursive: recursive,
        Ignore:    ignore.New(ignorePatterns),
    }
    discovered, skipped, err := discovery.Walk(abs, reg, walkOpts)

    // 5. Load ledger.
    lc, err := manifest.LoadLedger(abs)
    // lc is *manifest.LedgerContents or nil (no ledger).

    // 6. Build ledger lookup map: relPath → domain.LedgerEntry
    ledgerMap := make(map[string]domain.LedgerEntry)
    if lc != nil {
        for _, entry := range lc.Entries {
            ledgerMap[entry.Path] = entry
        }
    }

    // 7. Classify discovered files into 4 buckets.
    //    Use a local struct for classified files:
    type statusFile struct {
        Path        string // relative path from dirA
        Destination string // ledger destination (sorted/duplicate)
        Matches     string // ledger matches field (duplicate)
        Reason      string // ledger reason (error) or skip reason (unrecognized)
    }

    var sorted, duplicates, errored, unsorted []statusFile

    for _, df := range discovered {
        entry, found := ledgerMap[df.RelPath]
        if !found {
            unsorted = append(unsorted, statusFile{Path: df.RelPath})
            continue
        }
        switch entry.Status {
        case domain.LedgerStatusCopy:
            sorted = append(sorted, statusFile{
                Path:        df.RelPath,
                Destination: entry.Destination,
            })
        case domain.LedgerStatusDuplicate:
            duplicates = append(duplicates, statusFile{
                Path:        df.RelPath,
                Destination: entry.Destination,
                Matches:     entry.Matches,
            })
        case domain.LedgerStatusError:
            errored = append(errored, statusFile{
                Path:   df.RelPath,
                Reason: entry.Reason,
            })
        default:
            // "skip" or any unknown status → treat as unsorted
            unsorted = append(unsorted, statusFile{Path: df.RelPath})
        }
    }

    // 8. Classify skipped files (from discovery walk) as unrecognized.
    var unrecognized []statusFile
    for _, sf := range skipped {
        unrecognized = append(unrecognized, statusFile{
            Path:   sf.Path,
            Reason: sf.Reason,
        })
    }

    // 9. Sort each bucket alphabetically by Path.
    //    Use slices.SortFunc or sort.Slice.

    // 10. Produce output (Task 3 or Task 4).
}
```

### Key types

Define `statusFile` as a package-level unexported struct in `cmd/status.go`:

```go
// statusFile holds the classification result for a single file.
type statusFile struct {
    Path        string `json:"path"`
    Destination string `json:"destination,omitempty"`
    Matches     string `json:"matches,omitempty"`
    Reason      string `json:"reason,omitempty"`
}
```

Define `statusResult` to hold the full classification:

```go
// statusResult holds the complete classification of a source directory.
type statusResult struct {
    Source       string                 // absolute path to dirA
    Ledger       *manifest.LedgerContents // nil if no ledger
    Sorted       []statusFile
    Duplicates   []statusFile
    Errored      []statusFile
    Unsorted     []statusFile
    Unrecognized []statusFile
}
```

---

## Task 3 — Implement human-readable output formatter

**File:** `cmd/status.go`

### Function signature

```go
func printStatusTable(w io.Writer, r *statusResult)
```

### Output structure (per §7.4.4)

1. **Header lines:**
   ```
   Source: /Users/wells/photos
   Ledger: run a1b2c3d4, 2026-03-06 10:30:00 UTC (recursive: no)
   ```
   If no ledger: `Ledger: none found — no prior sort runs recorded for this directory.`

   The run ID is truncated to 8 characters (same convention as `pixe query runs`).

2. **Sections** — each section is printed only if it has >0 files:
   - `SORTED (N files)` — each line: `  <relPath>  → <destination>`
   - `DUPLICATE (N files)` — each line: `  <relPath>  → matches <matches>`
   - `ERRORED (N files)` — each line: `  <relPath>  → <reason>`
   - `UNSORTED (N files)` — each line: `  <relPath>`
   - `UNRECOGNIZED (N files)` — each line: `  <relPath>  → <reason>`

   Use "file" (singular) when count == 1, "files" otherwise.

3. **Summary line:**
   ```
   265 total | 247 sorted | 3 duplicates | 1 errored | 12 unsorted | 2 unrecognized
   ```
   Omit categories with 0 count from the summary (except total, which is always shown). If all categories are 0: `0 total`.

4. **Arrow character:** Use `→` (Unicode U+2192) as the separator, matching the architecture spec examples.

---

## Task 4 — Implement JSON output formatter

**File:** `cmd/status.go`

### Function signature

```go
func printStatusJSON(w io.Writer, r *statusResult) error
```

### JSON structure (per §7.4.5)

```go
type statusJSON struct {
    Source       string          `json:"source"`
    Ledger       *ledgerInfo    `json:"ledger"`       // null when no ledger
    Sorted       []statusFile   `json:"sorted"`
    Duplicates   []statusFile   `json:"duplicates"`
    Errored      []statusFile   `json:"errored"`
    Unsorted     []statusFile   `json:"unsorted"`
    Unrecognized []statusFile   `json:"unrecognized"`
    Summary      statusSummary  `json:"summary"`
}

type ledgerInfo struct {
    RunID       string `json:"run_id"`
    PixeVersion string `json:"pixe_version"`
    Timestamp   string `json:"timestamp"`
    Recursive   bool   `json:"recursive"`
}

type statusSummary struct {
    Total        int `json:"total"`
    Sorted       int `json:"sorted"`
    Duplicates   int `json:"duplicates"`
    Errored      int `json:"errored"`
    Unsorted     int `json:"unsorted"`
    Unrecognized int `json:"unrecognized"`
}
```

- Use `json.NewEncoder(w)` with `SetEscapeHTML(false)` and `SetIndent("", "  ")`.
- Empty arrays are included (not omitted) — use `[]statusFile{}` not `nil` for empty slices.

---

## Task 5 — Write unit tests

**File:** `cmd/status_test.go`

Use `package cmd` (white-box testing, same as other test files in the project).

### Test cases

| Test Name | Setup | Assertion |
|---|---|---|
| `TestRunStatus_noLedger` | Create temp dir with JPEG fixture files, no ledger | All files appear as UNSORTED; output contains "none found" |
| `TestRunStatus_allSorted` | Create temp dir with JPEG fixtures + ledger with all `"copy"` entries | All files appear as SORTED with correct destinations |
| `TestRunStatus_mixedStatus` | Ledger with copy, duplicate, error entries + files not in ledger | Correct classification into all 5 categories |
| `TestRunStatus_unrecognizedFiles` | Dir with `.txt` and `.jpg` files, ledger for the `.jpg` | `.txt` appears as UNRECOGNIZED, `.jpg` as SORTED |
| `TestRunStatus_recursive` | Nested subdirectories with files at multiple levels | Files at all depths discovered and classified |
| `TestRunStatus_jsonOutput` | Same as mixedStatus but with `--json` | Valid JSON with correct structure and counts |
| `TestRunStatus_emptyDirectory` | Empty temp dir, no ledger | Summary shows 0 total |

### Test helpers

- `writeTestLedger(t *testing.T, dirA string, header domain.LedgerHeader, entries []domain.LedgerEntry)` — writes a valid JSONL ledger file to `dirA/.pixe_ledger.json`.
- Reuse existing JPEG fixture helpers from the test suite (check `internal/integration/` for patterns).
- Use `t.TempDir()` for all filesystem tests.

### Testing approach

Since `runStatus` writes to `os.Stdout`, refactor the output functions to accept an `io.Writer` parameter. The `runStatus` function passes `cmd.OutOrStdout()` (standard Cobra pattern). Tests capture output via `bytes.Buffer`.

Alternatively, test the classification logic and formatters as separate functions, calling them directly with constructed `statusResult` values.

---

## Task 6 — Write integration test

**File:** `internal/integration/status_test.go`

End-to-end test that:

1. Creates a temp source dir with JPEG fixture files.
2. Creates a temp dest dir.
3. Runs `pixe sort` (via `pipeline.Run`) to sort the files.
4. Verifies the ledger was created in the source dir.
5. Adds new JPEG files to the source dir (unsorted).
6. Calls the status classification logic and verifies:
   - Previously sorted files appear as SORTED with correct destinations.
   - New files appear as UNSORTED.
   - Summary counts are correct.

This test should be tagged/located in `internal/integration/` so it's excluded from `make test` but included in `make test-all`.

---

## Task 7 — Run `make check`

Run `make check` (which executes `fmt-check + vet + unit tests`) and fix any issues:
- Lint errors from `golangci-lint`
- Vet warnings
- Test failures
- Format issues (run `make fmt` if needed)

---

## Task 8 — Commit all changes

Commit the architecture update and implementation together. Files expected:
- `.state/ARCHITECTURE.md` (updated: §7.1 command list, §7.4 status command design)
- `.state/PLAN.md` (this plan)
- `cmd/status.go` (new)
- `cmd/status_test.go` (new)
- `internal/integration/status_test.go` (new)
