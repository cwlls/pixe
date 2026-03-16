# Implementation Plan

## Feature: Uniform `--dest` resolution and alias support across all commands

Two bugs affect every command except `sort`: (1) `dest:` in the config file is ignored because each command binds `--dest` to a prefixed Viper key (e.g., `"verify_dest"`) that doesn't match the config file key `"dest"`, and (2) `+alias` syntax is never resolved because only `sort` calls `resolveAlias()`. The fix introduces a shared `resolveDest()` helper and migrates all six commands — including `sort` — to a uniform pattern.

## Task Summary

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add `resolveDest()` helper to `cmd/helpers.go` | high | @developer | [x] complete | — | New shared function; foundation for all other tasks |
| 2 | Add tests for `resolveDest()` in `cmd/helpers_test.go` | high | @developer | [x] complete | 1 | Unit tests covering fallback, alias, empty, precedence |
| 3 | Migrate `sort` to `resolveDest("sort_dest")` | high | @developer | [x] complete | 1 | Change Viper binding from `"dest"` to `"sort_dest"`, replace inline `resolveAlias` call |
| 4 | Migrate `verify` to `resolveDest("verify_dest")` | high | @developer | [x] complete | 1 | Replace inline dest reading + validation |
| 5 | Migrate `resume` to `resolveDest("resume_dest")` | high | @developer | [x] complete | 1 | Replace inline dest reading + validation |
| 6 | Migrate `stats` to `resolveDest("stats_dest")` | high | @developer | [x] complete | 1 | Replace inline dest reading + validation |
| 7 | Migrate `query` to `resolveDest("query_dest")` | high | @developer | [x] complete | 1 | Replace in `PersistentPreRunE` |
| 8 | Migrate `clean` to `resolveDest("clean_dest")` | high | @developer | [x] complete | 1 | Replace inline dest reading + validation |
| 9 | Update `resolveConfig()` to use `resolveDest()` | high | @developer | [x] complete | 1, 3 | `resolveConfig()` populates `cfg.Destination` — must use new helper |
| 10 | Run full test suite and lint | high | @tester | [x] complete | 1–9 | `make check` + `make test-all` + `make lint` |
| 11 | Commit | medium | @committer | [x] complete | 10 | Single commit for the full change |

---

## Parallelization Strategy

**Wave 1** (foundation): Tasks 1 + 2 — create the helper and its tests. Must complete before anything else.

**Wave 2** (migrations — all independent, can run in parallel): Tasks 3, 4, 5, 6, 7, 8, 9 — each command migration is independent. `sort` (task 3) and `resolveConfig` (task 9) are coupled (both touch `helpers.go` and `sort.go`) so should be done by the same developer in sequence, but the other five commands (tasks 4–8) can be done in parallel with each other.

**Wave 3** (validation): Task 10 — run after all code changes.

**Wave 4** (commit): Task 11 — after tests pass.

---

## Task Descriptions

### Task 1: Add `resolveDest()` helper to `cmd/helpers.go`

**File:** `cmd/helpers.go`

**What:** Add a new exported function `resolveDest` with the following signature and behavior:

```go
// resolveDest resolves the destination directory from Viper configuration.
// It checks the command-specific prefixed key first (e.g., "verify_dest"),
// falls back to the global "dest" key (from config file / env var), then
// resolves any +alias prefix via resolveAlias().
func resolveDest(prefixedKey string) (string, error) {
    dir := viper.GetString(prefixedKey)
    if dir == "" {
        dir = viper.GetString("dest")
    }
    if dir == "" {
        return "", fmt.Errorf("--dest is required")
    }

    aliases := viper.GetStringMapString("aliases")
    resolved, err := resolveAlias(dir, aliases)
    if err != nil {
        return "", err
    }
    return resolved, nil
}
```

**Location:** Insert after the existing `resolveAlias()` function (after line ~283). This keeps the two alias-related functions adjacent.

**Notes:**
- The function is unexported (`resolveDest`, not `ResolveDest`) — it's only used within the `cmd` package.
- It does NOT validate that the directory exists or is a directory — that responsibility stays with each command (some commands create the directory, others require it to exist, `stats` delegates to `dblocator`).
- It does NOT call `filepath.Abs()` — commands that need absolute paths do that themselves.
- The error message `"--dest is required"` matches the existing wording used by all commands.
- Copyright header is already at the top of `helpers.go` — no change needed.

---

### Task 2: Add tests for `resolveDest()` in `cmd/helpers_test.go`

**File:** `cmd/helpers_test.go`

**What:** Add test functions covering `resolveDest()`. Each test must reset Viper state to avoid cross-test contamination. Use `viper.Reset()` in setup or `t.Cleanup`.

**Test cases:**

1. **`TestResolveDest_prefixedKeyTakesPrecedence`** — Set both `viper.Set("sort_dest", "/from/flag")` and `viper.Set("dest", "/from/config")`. Call `resolveDest("sort_dest")`. Expect `"/from/flag"`.

2. **`TestResolveDest_fallbackToGlobalDest`** — Set only `viper.Set("dest", "/from/config")`. Call `resolveDest("sort_dest")`. Expect `"/from/config"`.

3. **`TestResolveDest_emptyBothReturnsError`** — Neither key set. Call `resolveDest("sort_dest")`. Expect error containing `"--dest is required"`.

4. **`TestResolveDest_aliasResolved`** — Set `viper.Set("dest", "+nas")` and `viper.Set("aliases", map[string]string{"nas": "/Volumes/NAS/Photos"})`. Call `resolveDest("sort_dest")`. Expect `"/Volumes/NAS/Photos"`.

5. **`TestResolveDest_aliasFromPrefixedKey`** — Set `viper.Set("verify_dest", "+backup")` and `viper.Set("aliases", map[string]string{"backup": "/mnt/backup"})`. Call `resolveDest("verify_dest")`. Expect `"/mnt/backup"`.

6. **`TestResolveDest_unknownAliasReturnsError`** — Set `viper.Set("dest", "+unknown")` with aliases map that doesn't contain `"unknown"`. Expect error from `resolveAlias`.

7. **`TestResolveDest_literalPathPassthrough`** — Set `viper.Set("dest", "/literal/path")`. Call `resolveDest("sort_dest")`. Expect `"/literal/path"` unchanged.

8. **`TestResolveDest_tildeExpandedInAlias`** — Set `viper.Set("dest", "+home")` and `viper.Set("aliases", map[string]string{"home": "~/Photos"})`. Call `resolveDest("sort_dest")`. Expect the tilde expanded to `os.UserHomeDir()` + `/Photos`.

**Pattern:** Each test should call `viper.Reset()` at the start (or use `t.Cleanup(viper.Reset)`). The existing `resolveAlias` tests in this file already follow a similar pattern — match their style.

---

### Task 3: Migrate `sort` to `resolveDest("sort_dest")`

**File:** `cmd/sort.go`

**What:** Three changes:

1. **Change Viper binding** (line 290): Change from `viper.BindPFlag("dest", sortCmd.Flags().Lookup("dest"))` to `viper.BindPFlag("sort_dest", sortCmd.Flags().Lookup("dest"))`.

2. **Remove inline `resolveAlias` call** (lines 103-112 in RunE): The `resolveAlias()` call and the `cfg.Destination = resolvedDest` assignment are no longer needed here — `resolveConfig()` will handle this via `resolveDest()` (see Task 9).

3. **Remove the empty-dest check** (lines 131-133): The `if cfg.Destination == "" { return fmt.Errorf("--dest is required") }` check is no longer needed — `resolveDest()` returns this error, and `resolveConfig()` propagates it.

**Important:** The `mergeSourceConfig()` function (helpers.go lines 201, 212-219) currently checks if the `--dest` flag was changed via `sortCmd.Flags().Lookup("dest").Changed`. This logic compares the flag state to decide whether to merge the source-local `dest` value. This still works correctly because `Lookup("dest").Changed` reflects whether the CLI flag was passed — it's independent of the Viper key name. No change needed in `mergeSourceConfig()`.

**Also important:** The `os.MkdirAll` call for the destination directory (line 145-147) must remain — `sort` is the only command that creates the destination if it doesn't exist.

---

### Task 4: Migrate `verify` to `resolveDest("verify_dest")`

**File:** `cmd/verify.go`

**What:** In the `RunE` function, replace the dest-reading block (lines 52-64):

```go
// BEFORE (lines 52-64):
dir := viper.GetString("verify_dest")
// ...
if dir == "" {
    return fmt.Errorf("--dest is required")
}
// ...
info, err := os.Stat(dir)
// ... error handling ...
```

With:

```go
// AFTER:
dir, err := resolveDest("verify_dest")
if err != nil {
    return err
}

dir, err = filepath.Abs(dir)
if err != nil {
    return fmt.Errorf("resolving absolute path: %w", err)
}

info, err := os.Stat(dir)
// ... existing error handling for stat/isdir stays the same ...
```

**Notes:**
- The Viper binding at line 160 (`viper.BindPFlag("verify_dest", ...)`) stays unchanged — the key name is already correct.
- The `filepath.Abs` call should be added if not already present (verify currently doesn't call it, but `query` and `clean` do — adding it here is defensive and consistent).
- The `os.Stat` + `IsDir` validation stays — verify requires the directory to already exist.

---

### Task 5: Migrate `resume` to `resolveDest("resume_dest")`

**File:** `cmd/resume.go`

**What:** In the `RunE` function, replace the dest-reading block (lines 57-68):

```go
// BEFORE:
dir := viper.GetString("resume_dest")
if dir == "" {
    return fmt.Errorf("--dest is required")
}
// ...
info, err := os.Stat(dir)
// ... error handling ...
```

With:

```go
// AFTER:
dir, err := resolveDest("resume_dest")
if err != nil {
    return err
}

dir, err = filepath.Abs(dir)
if err != nil {
    return fmt.Errorf("resolving absolute path: %w", err)
}

info, err := os.Stat(dir)
// ... existing error handling stays ...
```

**Notes:**
- The Viper binding at line 228 stays unchanged.
- The `os.Stat` + `IsDir` validation stays — resume requires the directory to already exist.

---

### Task 6: Migrate `stats` to `resolveDest("stats_dest")`

**File:** `cmd/stats.go`

**What:** In the `RunE` function, replace the dest-reading block (lines 41-44):

```go
// BEFORE:
dir := viper.GetString("stats_dest")
if dir == "" {
    return fmt.Errorf("--dest is required")
}
```

With:

```go
// AFTER:
dir, err := resolveDest("stats_dest")
if err != nil {
    return err
}
```

**Notes:**
- The Viper binding at line 194 stays unchanged.
- `stats` does NOT validate directory existence itself — it delegates to `dblocator.Resolve` which handles that. So no `os.Stat` call needed here.
- Check whether `err` is already declared in scope (it may need `:=` vs `=` depending on context).

---

### Task 7: Migrate `query` to `resolveDest("query_dest")`

**File:** `cmd/query.go`

**What:** In the `PersistentPreRunE` function (`openQueryDB`), replace the dest-reading block (lines 60-77):

```go
// BEFORE:
dir := viper.GetString("query_dest")
if dir == "" {
    return fmt.Errorf("--dest is required")
}
// ...
dir, err := filepath.Abs(dir)
// ... stat + isdir checks ...
```

With:

```go
// AFTER:
dir, err := resolveDest("query_dest")
if err != nil {
    return err
}

dir, err = filepath.Abs(dir)
if err != nil {
    return fmt.Errorf("resolving absolute path: %w", err)
}

info, err := os.Stat(dir)
// ... existing isdir check stays ...
```

**Notes:**
- The Viper binding at line 118 stays unchanged.
- `query` uses `PersistentFlags` (not `Flags`) because it has subcommands — this is unaffected by the change.
- The `filepath.Abs` + `os.Stat` + `IsDir` validation stays.

---

### Task 8: Migrate `clean` to `resolveDest("clean_dest")`

**File:** `cmd/clean.go`

**What:** In the `RunE` function, replace the dest-reading block (lines 69-94):

```go
// BEFORE:
dir := viper.GetString("clean_dest")
if dir == "" {
    return fmt.Errorf("--dest is required")
}
// ...
dir, err := filepath.Abs(dir)
// ... stat + isdir checks ...
```

With:

```go
// AFTER:
dir, err := resolveDest("clean_dest")
if err != nil {
    return err
}

dir, err = filepath.Abs(dir)
if err != nil {
    return fmt.Errorf("resolving absolute path: %w", err)
}

info, err := os.Stat(dir)
// ... existing isdir check stays ...
```

**Notes:**
- The Viper binding at line 379 stays unchanged.
- The `filepath.Abs` + `os.Stat` + `IsDir` validation stays.

---

### Task 9: Update `resolveConfig()` to use `resolveDest()`

**File:** `cmd/helpers.go`

**What:** In `resolveConfig()` (line 64), replace the direct Viper read:

```go
// BEFORE (line 64):
Destination: viper.GetString("dest"),
```

With a call to `resolveDest()` that happens before the struct literal, and populates `Destination` with the resolved value:

```go
// AFTER:
resolvedDest, err := resolveDest("sort_dest")
if err != nil && err.Error() != "--dest is required" {
    // Alias resolution error — fail immediately.
    return nil, err
}
// If err is "--dest is required", resolvedDest is "" and that's fine —
// the caller (sort.go RunE) will check cfg.Destination later.
```

**Wait — reconsider.** `resolveConfig()` is called by `sort` at multiple points (initial, after source config merge, after profile load). The dest value may be empty on the first call and populated after source config merge. The cleanest approach:

**Option A (recommended):** Do NOT call `resolveDest()` inside `resolveConfig()`. Instead, have `sort.go`'s RunE call `resolveDest("sort_dest")` directly after all config merging is complete (after the final `resolveConfig()` call), and assign the result to `cfg.Destination`. This matches how the other commands work — they call `resolveDest()` at the top of their RunE.

**Implementation for Option A:**
1. In `resolveConfig()` (line 64): keep `Destination: viper.GetString("dest"),` as-is — but wait, `sort` now binds to `"sort_dest"`, not `"dest"`. So this line would no longer pick up the CLI flag value. Change it to:
   ```go
   Destination: viper.GetString("sort_dest"),
   ```
   But this breaks the config-file fallback inside `resolveConfig()`. The simplest fix: just leave `Destination` empty in `resolveConfig()` and let `sort.go` call `resolveDest("sort_dest")` after all config resolution is done.

2. In `resolveConfig()`, change line 64 to not set `Destination` at all (or set it to `""`). Actually, `resolveConfig()` builds the full `AppConfig` struct — just remove the `Destination` field from the struct literal and let it zero-value to `""`.

3. In `sort.go` RunE, after the final `resolveConfig()` call and after `mergeSourceConfig()` and `loadProfile()`, add:
   ```go
   resolvedDest, err := resolveDest("sort_dest")
   if err != nil {
       return err
   }
   cfg.Destination = resolvedDest
   ```

4. Remove the now-redundant empty check and `resolveAlias` call that were previously in sort.go (already covered by Task 3).

**Also update `mergeSourceConfig()`:** The `mergeSourceConfig()` function at lines 212-219 merges the source-local `dest` value into Viper's `"dest"` key. This still works because `resolveDest("sort_dest")` falls back to `viper.GetString("dest")`. The source-local merge writes to the `"dest"` key (not `"sort_dest"`), so the fallback picks it up. No change needed in `mergeSourceConfig()`.

---

### Task 10: Run full test suite and lint

**Commands:**
```bash
make check          # fmt-check + vet + unit tests
make test-all       # all tests including integration
make lint           # golangci-lint
```

**What to verify:**
- All existing tests pass (especially `cmd/` package tests that exercise sort, verify, etc.)
- The new `resolveDest` tests pass
- No lint warnings
- No formatting issues

**If tests fail:** The most likely failures are in integration tests that may set up Viper state expecting the old `"dest"` key for sort. Check `internal/integration/` for any `viper.Set("dest", ...)` calls that need updating.

---

### Task 11: Commit

**Commit message:**
```
fix: uniform --dest resolution and alias support across all commands

Add resolveDest() helper that resolves --dest from command-specific
Viper key with fallback to global "dest" config key and +alias
expansion. Migrate all six --dest-accepting commands (sort, verify,
resume, stats, query, clean) to use it uniformly.

Previously only sort read from the "dest" config key and resolved
aliases. All other commands bound to prefixed keys (e.g. "verify_dest")
that didn't match the config file, and never called resolveAlias().
```
