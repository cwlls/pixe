# Implementation State

## Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | `sort`: default `--source` to cwd | high | @developer | [ ] pending | — | Remove required constraint, add `os.Getwd()` fallback |
| 2  | `status`: default `--source` to cwd | high | @developer | [ ] pending | — | Remove required constraint, add `os.Getwd()` fallback |
| 3  | Update help text for both commands | medium | @developer | [ ] pending | 1, 2 | `Long` descriptions and flag usage strings |
| 4  | Add/update tests for cwd default | high | @tester | [ ] pending | 1, 2 | Verify cwd fallback, explicit override, and error on non-directory |
| 5  | Run `make check` and fix any issues | high | @developer | [ ] pending | 1–4 | `fmt-check + vet + unit tests` gate |

---

# Task Descriptions

## Task 1 — `sort`: default `--source` to cwd

**Files:** `cmd/sort.go`

### Changes to `init()` (flag registration)

1. **Remove** the `MarkFlagRequired("source")` call (line 210). The flag is no longer required — it has a default.
2. **Update** the flag usage string from `"source directory containing media files to sort (required)"` to `"source directory containing media files to sort (default: current directory)"`.

### Changes to `runSort()` (runtime logic)

3. **Replace** the `cfg.Source == ""` error block (lines 83–85) with a cwd fallback:

```go
if cfg.Source == "" {
    cwd, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("resolve current directory: %w", err)
    }
    cfg.Source = cwd
}
```

The existing `os.Stat` + `IsDir()` validation (lines 91–97) remains unchanged — it already validates whatever path ends up in `cfg.Source`, whether explicit or defaulted.

### Changes to `Long` description

4. **Update** the `Long` string on `sortCmd` to mention the cwd default. Replace:
   ```
   Sort discovers all supported media files in the source directory (--source),
   ```
   with:
   ```
   Sort discovers all supported media files in the source directory. When --source
   is omitted, the current working directory is used.
   ```

---

## Task 2 — `status`: default `--source` to cwd

**Files:** `cmd/status.go`

### Changes to `init()` (flag registration)

1. **Remove** the `MarkFlagRequired("source")` call (line 423). The flag is no longer required.
2. **Update** the flag usage string from `"source directory to inspect (required)"` to `"source directory to inspect (default: current directory)"`.

### Changes to `runStatus()` (runtime logic)

3. **Add** a cwd fallback after reading the `source` variable from Viper (after line 87):

```go
if source == "" {
    cwd, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("resolve current directory: %w", err)
    }
    source = cwd
}
```

The existing `filepath.Abs` + `os.Stat` + `IsDir()` validation (lines 95–105) remains unchanged.

### Changes to `Long` description

4. **Update** the `Long` string on `statusCmd` to mention the cwd default. Replace:
   ```
   Status inspects a source directory and reports which files have been sorted,
   ```
   with:
   ```
   Status inspects a source directory (defaulting to the current working directory)
   and reports which files have been sorted,
   ```

---

## Task 3 — Update help text for both commands

**Files:** `cmd/sort.go`, `cmd/status.go`

Covered by the `Long` description and flag usage string changes in Tasks 1 and 2. This task exists as a checkpoint to verify the help output reads naturally after the changes. Run `go run . sort --help` and `go run . status --help` and confirm:

- `--source` is listed with `(default: current directory)` not `(required)`.
- The `Long` description mentions the cwd default.
- `--dest` on `sort` still shows as required.

---

## Task 4 — Add/update tests for cwd default

**Files:** `cmd/status_test.go` (and optionally a new `cmd/sort_test.go` if sort-level unit tests exist)

### Status tests

The existing `status_test.go` tests use the `classify()` helper which bypasses Cobra flag parsing. The cwd-default logic lives in `runStatus()`, so a Cobra-level test is needed:

1. **Add `TestRunStatus_defaultsToCwd`** — Invoke `statusCmd` via Cobra without `--source`. Verify it uses the current directory. This can be done by:
   - Creating a temp dir with a JPEG fixture.
   - Changing the working directory to the temp dir (`os.Chdir`).
   - Executing `statusCmd` through Cobra's `Execute()` (or calling `runStatus` directly after resetting Viper).
   - Asserting the output references the temp dir path.
   - Restoring the original working directory in a `t.Cleanup`.

2. **Add `TestRunStatus_sourceOverridesCwd`** — Invoke `statusCmd` with an explicit `--source` pointing to a different directory than cwd. Verify the explicit path is used, not cwd.

### Sort tests

The sort command has heavier dependencies (DB, pipeline), so a lightweight test verifying the cwd-default resolution in isolation is sufficient:

3. **Add a test** that calls `runSort` (or the relevant config-resolution block) with an empty `source` Viper value and verifies `cfg.Source` is set to `os.Getwd()`. Alternatively, test at the Cobra flag level that `--source` is no longer marked required.

---

## Task 5 — Run `make check` and fix any issues

Run `make check` (`fmt-check + vet + unit tests`). All existing tests must continue to pass. The status tests that use `classify()` directly are unaffected since they don't go through Cobra flag parsing. The new tests from Task 4 must also pass.
