# Implementation Plan: Enhanced Ignore System (`.gitignore`-Style)

> **Architecture reference:** Section 4.11 of `.state/ARCHITECTURE.md`
> **Scope:** Consolidates former Future Considerations #11, #12, #13 into a single feature.

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Add `doublestar` dependency | High | Code | [x] complete | — | `go get github.com/bmatcuk/doublestar/v4` |
| 2  | Refactor `Matcher` to use `doublestar.Match` | High | Code | Pending | 1 | Drop-in replacement for `filepath.Match`; add slash normalization |
| 3  | Add `MatchDir` method | High | Code | Pending | 2 | Trailing-slash semantics for directory-level skipping |
| 4  | Add `.pixeignore` loading + scope stack | High | Code | Pending | 3 | `PushScope` / `PopScope`; parse file format; hardcode `.pixeignore` ignore |
| 5  | Update `discovery.Walk` for directory ignoring | High | Code | Pending | 3 | Call `MatchDir` in the directory handling block |
| 6  | Update `discovery.Walk` for `.pixeignore` scoping | High | Code | Pending | 4, 5 | `PushScope` on dir entry, `PopScope` on dir exit |
| 7  | Unit tests for `internal/ignore` | High | Code | Pending | 4 | `**` globs, trailing-slash, scopes, `.pixeignore` parsing |
| 8  | Unit tests for `discovery.Walk` changes | High | Code | Pending | 6 | Directory skipping, `.pixeignore` loading during walk |
| 9  | Integration tests | Medium | Code | Pending | 6 | End-to-end: nested `.pixeignore` + `**` + directory-skip |
| 10 | Update ARCHITECTURE.md Section 2 table | Low | Code | Pending | 1 | Add `doublestar` row to Technical Stack |
| 11 | Docs: CHANGELOG + README | Low | Scribe | Pending | 9 | Document new ignore capabilities |

---

## Task Descriptions

### Task 1 — Add `doublestar` dependency

**File:** `go.mod`, `go.sum`

```bash
go get github.com/bmatcuk/doublestar/v4
```

Verify the module resolves and `go mod tidy` passes.

---

### Task 2 — Refactor `Matcher` to use `doublestar.Match`

**File:** `internal/ignore/ignore.go`

Replace `filepath.Match` calls with `doublestar.Match`. Key changes:

1. Add import: `"github.com/bmatcuk/doublestar/v4"`.
2. In `Match()`, replace both `filepath.Match(pattern, filename)` and `filepath.Match(pattern, relPath)` with `doublestar.Match(pattern, filepath.ToSlash(name))` and `doublestar.Match(pattern, filepath.ToSlash(relPath))`.
3. File-level `Match()` must skip patterns that end with `/` — those are directory-only patterns handled by `MatchDir`.
4. Update the package doc comment to mention `doublestar` and `**` support.

**Backward compatibility:** All existing patterns (`*.txt`, `.DS_Store`, `subdir/*.tmp`) continue to work — `doublestar.Match` is a superset of `filepath.Match`.

**Existing tests must still pass** after this change — run `go test -race ./internal/ignore/...` before proceeding.

---

### Task 3 — Add `MatchDir` method

**File:** `internal/ignore/ignore.go`

Add the `MatchDir(dirname, relDirPath string) bool` method to `Matcher`:

1. Iterate over all patterns (global + active scopes, once scopes exist in Task 4).
2. **Trailing-slash patterns:** For patterns ending with `/`, strip the trailing `/` and match the remaining pattern against both `dirname` and `relDirPath` using `doublestar.Match`.
3. **Implicit directory patterns:** For patterns like `backups/**` (contains `**` at the end), check if `relDirPath` matches the prefix before `/**`. This allows the walk to skip the entire `backups/` directory rather than descending into it and ignoring each file individually.
4. Non-slash patterns are ignored by `MatchDir` (they apply to files, not directories).

---

### Task 4 — Add `.pixeignore` loading + scope stack

**File:** `internal/ignore/ignore.go`

Extend `Matcher` with the scope stack per Section 4.11.3–4.11.4:

1. **New types:**
   ```go
   type patternScope struct {
       basePath string   // relative path from dirA to .pixeignore's directory
       patterns []string // parsed patterns from that .pixeignore
   }
   ```

2. **Rename existing `patterns` field to `global`** in the `Matcher` struct. Add `scopes []patternScope`.

3. **`PushScope(basePath, pixeignorePath string) bool`:** Read and parse the `.pixeignore` file at `pixeignorePath`. Parse rules: skip blank lines, skip `#` comment lines, trim whitespace, deduplicate. Push a `patternScope` onto the stack. Return `true` if the file existed and was loaded.

4. **`PopScope()`:** Pop the last scope from the stack.

5. **Update `Match()` and `MatchDir()`** to also check patterns in all active scopes. For scoped patterns, compute the file's path relative to the scope's `basePath` before matching.

6. **Hardcoded ignores:** Add `.pixeignore` alongside `.pixe_ledger.json` as a hardcoded filename ignore:
   ```go
   const pixeignoreFilename = ".pixeignore"
   ```
   Both constants are checked by filename equality in `Match()`.

---

### Task 5 — Update `discovery.Walk` for directory ignoring

**File:** `internal/discovery/walk.go`

Update the directory handling block (currently lines 72–84) to call `MatchDir` after the existing hardcoded checks:

```go
if d.IsDir() {
    if path == dirA {
        return nil
    }
    if strings.HasPrefix(name, ".") {
        return filepath.SkipDir
    }
    if !opts.Recursive {
        return filepath.SkipDir
    }
    // NEW: user-configured directory ignore patterns
    if opts.Ignore != nil {
        relDir, _ := filepath.Rel(dirA, path)
        if opts.Ignore.MatchDir(name, relDir) {
            return filepath.SkipDir
        }
    }
    return nil
}
```

This is a small, isolated change. The `MatchDir` method does the heavy lifting.

---

### Task 6 — Update `discovery.Walk` for `.pixeignore` scoping

**File:** `internal/discovery/walk.go`

This is the most structurally complex change. `filepath.WalkDir` does not have a "leaving directory" callback, so we need to track scope depth ourselves.

**Approach:** Replace the simple `filepath.WalkDir` callback with one that tracks directory entry/exit via a stack of entered directories. When entering a directory, check for `.pixeignore` and call `PushScope`. When the walk moves to a path outside the current scope's directory, call `PopScope`.

**Implementation sketch:**

```go
// Track scope directories for PopScope on exit.
var scopeStack []string // absolute paths of directories where PushScope was called

err = filepath.WalkDir(dirA, func(path string, d fs.DirEntry, walkErr error) error {
    // Pop scopes for directories we've left.
    // filepath.WalkDir visits in lexical order; when we see a path that is
    // no longer under the top of scopeStack, we've exited that directory.
    for len(scopeStack) > 0 {
        top := scopeStack[len(scopeStack)-1]
        if strings.HasPrefix(path, top+string(filepath.Separator)) || path == top {
            break
        }
        opts.Ignore.PopScope()
        scopeStack = scopeStack[:len(scopeStack)-1]
    }

    // ... existing walkErr, name, directory handling ...

    // After directory handling (for directories we descend into):
    if d.IsDir() && path != dirA {
        relDir, _ := filepath.Rel(dirA, path)
        pixeignorePath := filepath.Join(path, ".pixeignore")
        if opts.Ignore != nil && opts.Ignore.PushScope(relDir, pixeignorePath) {
            scopeStack = append(scopeStack, path)
        }
        return nil
    }

    // ... rest of file handling ...
})

// Final cleanup: pop any remaining scopes.
for range scopeStack {
    opts.Ignore.PopScope()
}
```

**Key concern:** The `PushScope` call must happen *after* the `MatchDir` check (Task 5) but *before* returning `nil` to descend. This ensures a directory can be skipped by its parent's patterns before its own `.pixeignore` is read.

**Also handle `.pixeignore` in `dirA` root:** The root directory is entered unconditionally (`path == dirA` returns `nil`). After that early return, we should check for a root `.pixeignore`:

```go
if path == dirA {
    if opts.Ignore != nil {
        pixeignorePath := filepath.Join(dirA, ".pixeignore")
        if opts.Ignore.PushScope(".", pixeignorePath) {
            scopeStack = append(scopeStack, dirA)
        }
    }
    return nil
}
```

---

### Task 7 — Unit tests for `internal/ignore`

**File:** `internal/ignore/ignore_test.go`

Extend the existing test file with new test functions and table cases:

1. **`TestMatcher_doublestarGlob`** — Verify `**/*.tmp`, `**/Thumbs.db`, `backups/**`, `a/**/b` patterns work with `Match()`.
2. **`TestMatcher_alternativesGlob`** — Verify `*.{txt,log}` and `[Tt]humbs.db` patterns.
3. **`TestMatcher_trailingSlashIgnoredByMatch`** — Verify that `Match()` does NOT match files against patterns ending with `/`.
4. **`TestMatcher_matchDir_trailingSlash`** — Verify `MatchDir()` matches directories against `node_modules/`, `.git/`, `**/cache/`, `backups/old/`.
5. **`TestMatcher_matchDir_noSlashNoMatch`** — Verify `MatchDir()` does NOT match against patterns without trailing `/` (those are file patterns).
6. **`TestMatcher_matchDir_implicitDoublestar`** — Verify `MatchDir()` matches `backups` directory when pattern is `backups/**`.
7. **`TestMatcher_pixeignoreHardcoded`** — Verify `.pixeignore` file itself is always ignored by `Match()`.
8. **`TestMatcher_pushPopScope`** — Create a Matcher, push a scope with patterns, verify matching within scope, pop scope, verify patterns no longer active.
9. **`TestMatcher_nestedScopes`** — Push two scopes, verify inner scope patterns active, pop inner, verify only outer remains.
10. **`TestMatcher_scopeRelativePaths`** — Verify that scoped patterns are matched relative to the scope's `basePath`, not relative to `dirA`.
11. **`TestMatcher_pushScope_fileNotFound`** — Verify `PushScope` returns `false` and does not push when the file doesn't exist.
12. **`TestMatcher_pushScope_parsesFormat`** — Write a `.pixeignore` with comments, blank lines, whitespace — verify only valid patterns are loaded.

---

### Task 8 — Unit tests for `discovery.Walk` changes

**File:** `internal/discovery/discovery_test.go`

Add tests using `t.TempDir()` fixture trees:

1. **`TestWalk_directoryIgnoreTrailingSlash`** — Create `dirA/node_modules/file.js` and `dirA/photo.jpg`. Configure `Ignore: ignore.New([]string{"node_modules/"})`. Verify `node_modules/file.js` is not discovered.
2. **`TestWalk_directoryIgnoreDoublestar`** — Create `dirA/a/b/cache/file.tmp` with pattern `**/cache/`. Verify entire `cache/` tree is skipped.
3. **`TestWalk_pixeignoreLoaded`** — Create `dirA/.pixeignore` with `*.txt`. Create `dirA/notes.txt` and `dirA/photo.jpg`. Verify `notes.txt` is ignored, `photo.jpg` is discovered.
4. **`TestWalk_nestedPixeignore`** — Create `dirA/sub/.pixeignore` with `*.log`. Create `dirA/app.log` (should NOT be ignored — pattern is scoped to `sub/`) and `dirA/sub/app.log` (should be ignored). Verify correct scoping.
5. **`TestWalk_pixeignoreFileItself`** — Verify `.pixeignore` files themselves don't appear in discovered or skipped slices.

---

### Task 9 — Integration tests

**File:** `internal/integration/ignore_test.go`

End-to-end tests that exercise the full sort pipeline with enhanced ignore patterns:

1. **`TestSort_doublestarIgnore`** — `pixe sort` with `--ignore "**/*.txt"` on a nested directory tree. Verify `.txt` files at all depths are excluded from the destination.
2. **`TestSort_directoryIgnore`** — `pixe sort` with `--ignore "node_modules/"` on a tree containing `node_modules/`. Verify the directory is skipped entirely.
3. **`TestSort_pixeignoreFile`** — Place a `.pixeignore` in the source directory. Run `pixe sort`. Verify patterns from the file are respected.
4. **`TestSort_nestedPixeignoreScoping`** — Place `.pixeignore` files at two levels. Verify scoping: patterns in a subdirectory's `.pixeignore` only affect that subtree.

---

### Task 10 — Update ARCHITECTURE.md Section 2 table

**File:** `.state/ARCHITECTURE.md`

Add a row to the Technical Stack table (Section 2):

```
| **Glob Matching** | `bmatcuk/doublestar/v4` | `**` recursive globs, `{alt}` alternatives; superset of `filepath.Match` |
```

Also update the cross-reference note in Section 4.4 to point to Section 4.11 for the full specification.

---

### Task 11 — Docs: CHANGELOG + README

**Files:** `CHANGELOG.md`, `README.md`

**CHANGELOG:** Add an `## [Unreleased]` or version entry documenting:
- `**` recursive glob support in `--ignore` patterns
- Directory-level ignore patterns with trailing `/`
- `.pixeignore` file support (format, scoping, nesting)

**README:** Update the ignore/exclusion section to show new pattern syntax and `.pixeignore` usage example.

---

## Implementation Order

The tasks have a clear dependency chain:

```
1 (dependency) → 2 (doublestar in Matcher) → 3 (MatchDir) → 4 (scopes)
                                                  ↓               ↓
                                            5 (Walk dirs)   6 (Walk scopes)
                                                  ↓               ↓
                                            7 (ignore tests) ← ← ←
                                                  ↓
                                            8 (walk tests)
                                                  ↓
                                            9 (integration)
                                                  ↓
                                          10 (arch update) + 11 (docs)
```

**Recommended execution:** Tasks 1–6 are the core implementation and should be done sequentially. Tasks 7–8 can be written alongside their corresponding implementation tasks. Task 9 validates the full stack. Tasks 10–11 are polish.
