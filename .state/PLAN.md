# Implementation Plan

## Feature: Cross-Command Flag Resolution Convention

Establish `resolve*()` helpers for all cross-command flags that require defaulting, fallback, or transformation logic. Fixes bugs where config-file values for `workers` and `db_path` are silently ignored by some commands. Normalizes `--json` Viper binding on query.

Reference: `.state/OVERVIEW.md` §7.1a

## Task Summary

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Create `resolveWorkers()` helper in `cmd/helpers.go` | high | @developer | [x] complete | — | New function, no existing code changes |
| 2 | Create `resolveDBPath()` helper in `cmd/helpers.go` | high | @developer | [x] complete | — | New function, no existing code changes |
| 3 | Write tests for `resolveWorkers()` and `resolveDBPath()` in `cmd/helpers_test.go` | high | @developer | [x] complete | 1, 2 | Follow `TestResolveDest_*` pattern |
| 4 | Migrate `--workers` from root persistent flag to per-command local flags | high | @developer | [x] complete | 1 | Touches root.go, sort.go, verify.go, resume.go |
| 5 | Migrate `--db-path` to use `resolveDBPath()` across all commands | high | @developer | [x] complete | 2 | Touches sort.go, resume.go, clean.go, stats.go, query.go |
| 6 | Normalize query `--json` to Viper binding | medium | @developer | [x] complete | — | Touches query.go + 7 subcommand files |
| 7 | Update `resolveConfig()` to use new helpers | high | @developer | [x] complete | 1, 2, 4 | Remove inline workers/db_path logic from helpers.go |
| 8 | Run tests, lint, vet | high | @tester | [x] complete | 3, 4, 5, 6, 7 | `make check` must pass |
| 9 | Commit | low | @committer | [~] in-process | 8 | — |

---

## Parallelization Strategy

**Wave 1** (no dependencies — can run in parallel):
- Task 1: Create `resolveWorkers()`
- Task 2: Create `resolveDBPath()`
- Task 6: Normalize query `--json` to Viper

**Wave 2** (depends on Wave 1):
- Task 3: Tests for both new helpers
- Task 4: Migrate `--workers` flag registrations (depends on Task 1)
- Task 5: Migrate `--db-path` flag registrations (depends on Task 2)
- Task 7: Update `resolveConfig()` (depends on Tasks 1, 2, 4)

**Wave 3** (depends on Wave 2):
- Task 8: Run full test/lint/vet suite
- Task 9: Commit

---

## Task Descriptions

### Task 1: Create `resolveWorkers()` helper

**File:** `cmd/helpers.go`

**Add function** (near `resolveDest()`, around line 306):

```go
// resolveWorkers resolves the worker count from Viper configuration.
// It checks the command-specific prefixed key first (e.g., "verify_workers"),
// falls back to the global "workers" key (from config file / env var / PIXE_WORKERS),
// then defaults to runtime.NumCPU() if still zero or negative.
func resolveWorkers(prefixedKey string) int {
	w := viper.GetInt(prefixedKey)
	if w <= 0 {
		w = viper.GetInt("workers")
	}
	if w <= 0 {
		w = runtime.NumCPU()
	}
	return w
}
```

Ensure `"runtime"` is in the import block (it already is — used by `resolveConfig()`).

---

### Task 2: Create `resolveDBPath()` helper

**File:** `cmd/helpers.go`

**Add function** (adjacent to `resolveWorkers()`):

```go
// resolveDBPath resolves the database path from Viper configuration.
// It checks the command-specific prefixed key first (e.g., "clean_db_path"),
// falls back to the global "db_path" key (from config file / env var / PIXE_DB_PATH).
// An empty return value means "auto-resolve" (dblocator will determine the path).
func resolveDBPath(prefixedKey string) string {
	p := viper.GetString(prefixedKey)
	if p == "" {
		p = viper.GetString("db_path")
	}
	return p
}
```

---

### Task 3: Write tests for `resolveWorkers()` and `resolveDBPath()`

**File:** `cmd/helpers_test.go`

Follow the exact pattern established by `TestResolveDest_*` tests (lines 146–254):
- Each test calls `viper.Reset()` at start and `t.Cleanup(viper.Reset)`.
- Values injected via `viper.Set(...)`.
- Standard `if got != want { t.Errorf(...) }` assertions.
- NOT `t.Parallel()` (global Viper state).

**Tests for `resolveWorkers()`:**

| Test Name | Setup | Expected |
|-----------|-------|----------|
| `TestResolveWorkers_prefixedKeyTakesPrecedence` | Set `sort_workers=4`, `workers=8` | Returns `4` |
| `TestResolveWorkers_fallbackToGlobalWorkers` | Set `workers=8` only | Returns `8` |
| `TestResolveWorkers_defaultToNumCPU` | Set nothing (both zero) | Returns `runtime.NumCPU()` |
| `TestResolveWorkers_negativeDefaultsToNumCPU` | Set `sort_workers=-1` | Returns `runtime.NumCPU()` |
| `TestResolveWorkers_zeroGlobalDefaultsToNumCPU` | Set `workers=0` only | Returns `runtime.NumCPU()` |

**Tests for `resolveDBPath()`:**

| Test Name | Setup | Expected |
|-----------|-------|----------|
| `TestResolveDBPath_prefixedKeyTakesPrecedence` | Set `clean_db_path=/a`, `db_path=/b` | Returns `/a` |
| `TestResolveDBPath_fallbackToGlobalDBPath` | Set `db_path=/b` only | Returns `/b` |
| `TestResolveDBPath_emptyBothReturnsEmpty` | Set nothing | Returns `""` |

---

### Task 4: Migrate `--workers` from root persistent flag to per-command local flags

This is the most complex task. Three files gain a local `--workers` flag; one file loses a persistent flag.

#### 4a. `cmd/root.go` — Remove persistent `--workers`

**Remove** lines 68-69:
```go
rootCmd.PersistentFlags().IntP("workers", "w", 0,
    "number of concurrent workers (0 = auto: runtime.NumCPU())")
```

**Remove** line 81:
```go
_ = viper.BindPFlag("workers", rootCmd.PersistentFlags().Lookup("workers"))
```

The `--algorithm`, `--quiet`, `--verbose`, `--profile` persistent flags remain unchanged.

#### 4b. `cmd/sort.go` — Add local `--workers` flag

**Add** to the flag registration block (around line 263, with the other sort flags):
```go
sortCmd.Flags().IntP("workers", "w", 0, "number of concurrent workers (0 = auto: runtime.NumCPU())")
```

**Add** to the Viper binding block (around line 286):
```go
_ = viper.BindPFlag("sort_workers", sortCmd.Flags().Lookup("workers"))
```

**In `RunE`** (or wherever `resolveConfig()` result is used): workers will now be resolved by `resolveWorkers("sort_workers")` — see Task 7.

#### 4c. `cmd/verify.go` — No flag changes needed

Verify already has a local `--workers` flag (line 154) bound to `verify_workers` (line 161). The only change is in the `RunE` — replace the inline resolution:

**Replace** (lines 75-78):
```go
workers := viper.GetInt("verify_workers")
if workers <= 0 {
    workers = runtime.NumCPU()
}
```

**With:**
```go
workers := resolveWorkers("verify_workers")
```

#### 4d. `cmd/resume.go` — Add local `--workers` flag

**Add** to the flag registration block (around line 225, with the other resume flags):
```go
resumeCmd.Flags().IntP("workers", "w", 0, "number of concurrent workers (0 = auto: runtime.NumCPU())")
```

**Add** to the Viper binding block (around line 228):
```go
_ = viper.BindPFlag("resume_workers", resumeCmd.Flags().Lookup("workers"))
```

**Replace** (lines 115-118):
```go
workers := viper.GetInt("workers")
if workers <= 0 {
    workers = runtime.NumCPU()
}
```

**With:**
```go
workers := resolveWorkers("resume_workers")
```

---

### Task 5: Migrate `--db-path` to use `resolveDBPath()` across all commands

#### 5a. `cmd/sort.go`

**Change Viper binding** (line 291) from:
```go
_ = viper.BindPFlag("db_path", sortCmd.Flags().Lookup("db-path"))
```
To:
```go
_ = viper.BindPFlag("sort_db_path", sortCmd.Flags().Lookup("db-path"))
```

The read happens in `resolveConfig()` — see Task 7.

#### 5b. `cmd/resume.go`

**Change Viper binding** (line 229) from:
```go
_ = viper.BindPFlag("db_path", resumeCmd.Flags().Lookup("db-path"))
```
To:
```go
_ = viper.BindPFlag("resume_db_path", resumeCmd.Flags().Lookup("db-path"))
```

**Replace** the read (line 61):
```go
dbPath := viper.GetString("db_path")
```
With:
```go
dbPath := resolveDBPath("resume_db_path")
```

#### 5c. `cmd/clean.go`

Already bound to `clean_db_path` (line 380). **Replace** the read (line 73):
```go
dbPath := viper.GetString("clean_db_path")
```
With:
```go
dbPath := resolveDBPath("clean_db_path")
```

#### 5d. `cmd/stats.go`

Already bound to `stats_db_path` (line 195). **Replace** the read (line 46):
```go
dbPath := viper.GetString("stats_db_path")
```
With:
```go
dbPath := resolveDBPath("stats_db_path")
```

#### 5e. `cmd/query.go`

Already bound to `query_db_path` (line 119). **Replace** the read (line 79):
```go
dbPath := viper.GetString("query_db_path")
```
With:
```go
dbPath := resolveDBPath("query_db_path")
```

---

### Task 6: Normalize query `--json` to Viper binding

#### 6a. `cmd/query.go` — Change flag registration and remove `jsonOut` variable

**Remove** `jsonOut` from the `var` block (line 39):
```go
jsonOut bool
```

**Change** the flag registration (line 116) from:
```go
queryCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "emit JSON output instead of a table")
```
To:
```go
queryCmd.PersistentFlags().Bool("json", false, "emit JSON output instead of a table")
```

**Add** Viper binding (after line 119, with the other bindings):
```go
_ = viper.BindPFlag("query_json", queryCmd.PersistentFlags().Lookup("json"))
```

#### 6b. Replace all `jsonOut` reads with `viper.GetBool("query_json")`

8 references across 7 files. Each `if jsonOut {` becomes `if viper.GetBool("query_json") {`:

| File | Line | Change |
|------|------|--------|
| `cmd/query_files.go` | 107 | `if jsonOut {` → `if viper.GetBool("query_json") {` |
| `cmd/query_duplicates.go` | 52 | `if jsonOut {` → `if viper.GetBool("query_json") {` |
| `cmd/query_duplicates.go` | 114 | `if jsonOut {` → `if viper.GetBool("query_json") {` |
| `cmd/query_runs.go` | 39 | `if jsonOut {` → `if viper.GetBool("query_json") {` |
| `cmd/query_run.go` | 69 | `if jsonOut {` → `if viper.GetBool("query_json") {` |
| `cmd/query_inventory.go` | 40 | `if jsonOut {` → `if viper.GetBool("query_json") {` |
| `cmd/query_skipped.go` | 40 | `if jsonOut {` → `if viper.GetBool("query_json") {` |
| `cmd/query_errors.go` | 39 | `if jsonOut {` → `if viper.GetBool("query_json") {` |

Ensure each file has `"github.com/spf13/viper"` in its imports. Files that already import viper need no change; files that don't will need it added.

---

### Task 7: Update `resolveConfig()` to use new helpers

**File:** `cmd/helpers.go`

#### 7a. Workers

**Replace** (line 64):
```go
Workers:              viper.GetInt("workers"),
```
With:
```go
Workers:              resolveWorkers("sort_workers"),
```

**Remove** the defaulting block (lines 115-117):
```go
if cfg.Workers <= 0 {
    cfg.Workers = runtime.NumCPU()
}
```
(This logic is now inside `resolveWorkers()`.)

#### 7b. DBPath

**Replace** (line 69):
```go
DBPath:               viper.GetString("db_path"),
```
With:
```go
DBPath:               resolveDBPath("sort_db_path"),
```

---

### Task 8: Run tests, lint, vet

```bash
make check          # fmt-check + vet + unit tests
make test           # unit tests with -race
make lint           # golangci-lint
```

All must pass. Pay particular attention to:
- Tests that set `viper.Set("workers", ...)` or `viper.Set("db_path", ...)` — they may need updating to use the new prefixed keys.
- Integration tests (`make test-integration`) if they exercise CLI flag resolution.
- `make docs-check` — the docgen AST parser extracts flag registrations. Moving `--workers` from root persistent to per-command local flags may change the generated docs. If so, run `make docs` and include the regenerated files.

---

### Task 9: Commit

Single commit covering all changes. Suggested message:

```
refactor: unify cross-command flag resolution with resolve*() helpers

Add resolveWorkers() and resolveDBPath() helpers in cmd/helpers.go,
following the pattern established by resolveDest(). Fixes bugs where
config-file workers and db_path values were silently ignored by
verify, clean, stats, and query commands.

- Move --workers from root persistent flag to local flags on sort,
  verify, and resume
- Normalize --db-path Viper keys to prefixed pattern across all
  commands
- Migrate query --json from raw BoolVar to Viper binding
```
