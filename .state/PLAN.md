# Implementation Plan

## Feature: Version scheme migration — semver → `major.minor` with `v0.x` reset

Migrate from three-segment semver (`v2.7.3`) to two-segment `major.minor` (`v0.23`). Delete all 51 legacy tags, create 23 new `v0.x` tags at selected historical commits, update GoReleaser archive naming to use lowercase OS and consistent arch labels, and update all documentation that references old version numbers.

Reference: ARCHITECTURE.md §3 (Version Management)

## Task Summary

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Update `.goreleaser.yaml` archive `name_template` | high | @developer | [x] complete | — | Remove `title .Os`, `x86_64` alias; use raw `.Os`-`.Arch` with hyphens |
| 2 | Update `cmd/version.go` example comments | low | @developer | [x] complete | — | Change `v0.10.0` → `v0.23` in doc comments |
| 3 | Update `docs/commands.md` version example | low | @developer | [x] complete | — | Change `v2.0.0` → `v0.23` in `pixe version` output example |
| 4 | Update `docs/changelog.md` — add version mapping header | medium | @developer | [x] complete | — | Add a note at the top explaining the old→new version mapping; keep old entries as historical record |
| 5 | Update `CHANGELOG.md` — add version mapping header | medium | @developer | [x] complete | — | Mirror the same note added to `docs/changelog.md` |
| 6 | Run `make check` — verify all tests and lint pass | high | @tester | [x] complete | 1, 2, 3, 4, 5 | `make check` (fmt-check + vet + unit tests + docs-check) |
| 7 | Commit all file changes | medium | @committer | [x] complete | 6 | Commit: `dc8923d` (rebased from `a103ee4`) |
| 8 | Delete all local git tags | high | @developer | [x] complete | 7 | All 52 old local tags deleted |
| 9 | Create 23 new `v0.x` tags per mapping table | high | @developer | [x] complete | 8 | `v0.1`–`v0.23` created; `v0.23` → `dc8923d` |
| 10 | Delete all remote git tags | high | @developer | [x] complete | 9 | All 52 old remote tags deleted in one push |
| 11 | Push new tags to remote | high | @developer | [x] complete | 10 | All 23 new tags + 2 commits pushed; rebase resolved divergence |
| 12 | Verify tag state | medium | @tester | [x] complete | 11 | `git tag --sort=v:refname` shows `v0.1`–`v0.23`; `make build` produces correct version |
| 13 | *(Manual)* Delete GitHub Releases via web UI | high | @developer | [x] complete | 12 | Deleted manually by user via GitHub web UI. |

---

## Parallelization Strategy

**Wave 1** (parallel — no dependencies between them):
- Task 1: `.goreleaser.yaml` name_template update
- Task 2: `cmd/version.go` comment update
- Task 3: `docs/commands.md` version example update
- Task 4: `docs/changelog.md` mapping header
- Task 5: `CHANGELOG.md` mapping header

**Wave 2** (sequential gate):
- Task 6: `make check` (depends on all of Wave 1)
- Task 7: commit (depends on 6)

**Wave 3** (sequential — git tag surgery, strict ordering):
- Task 8: delete local tags (depends on 7)
- Task 9: create new tags (depends on 8)
- Task 10: delete remote tags (depends on 9)
- Task 11: push new tags (depends on 10)
- Task 12: verify (depends on 11)

**Wave 4** (manual):
- Task 13: user deletes GitHub Releases via web UI

---

## Completion Summary

**Status:** 13 of 13 tasks complete. ✅

**What was accomplished:**
- ✅ Migrated version scheme from three-segment semver (`v2.7.3`) to two-segment `major.minor` (`v0.23`)
- ✅ Updated GoReleaser archive naming to use lowercase OS and consistent architecture labels
- ✅ Updated all documentation (code comments, `docs/commands.md`, `docs/changelog.md`, `CHANGELOG.md`) to reflect the new scheme
- ✅ Deleted all 52 legacy git tags from local and remote repositories
- ✅ Created 23 new `v0.x` tags at selected historical commits (v0.1 through v0.23)
- ✅ Pushed all changes and new tags to remote
- ✅ Verified tag state and build output
- ✅ Deleted all legacy GitHub Releases via web UI

**Documentation:**
- ARCHITECTURE.md §3 (Version Management) is fully up to date and documents the new scheme.


