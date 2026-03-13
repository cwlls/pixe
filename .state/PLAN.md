# Implementation Plan

## Feature: Documentation Overhaul — Config Page, Theme Migration, Pre-Commit Hook

## Task Summary

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Fix stale docs — run `make docs` and commit regenerated files | high | @developer | [ ] pending | — | Unblocks CI; must land before other doc changes |
| 2 | Create `scripts/pre-commit` hook script | high | @developer | [ ] pending | — | Shell script per §12.2 |
| 3 | Add `install-hooks` / `uninstall-hooks` Makefile targets | high | @developer | [ ] pending | 2 | Two new phony targets + update .PHONY list |
| 4 | Migrate Jekyll theme from Slate to Just the Docs | high | @developer | [ ] pending | 1 | `_config.yml`, `Gemfile`, front matter on all 11 pages |
| 5 | Create `docs/configuration.md` | high | @developer | [ ] pending | 4 | New page per §11.5; hand-authored, no docgen markers |
| 6 | Update `docs/commands.md` — remove config section, add cross-reference | high | @developer | [ ] pending | 5 | Remove lines 285–302, add link to configuration.md |
| 7 | Update `docs/index.md` — remove manual nav list | medium | @developer | [ ] pending | 4 | Sidebar replaces inline links; keep all other content |
| 8 | Update `docs/contributing.md` — document `make install-hooks` | medium | @developer | [ ] pending | 3 | Add hook installation to contributor setup steps |
| 9 | Regenerate docs after all changes — `make docs` | high | @developer | [ ] pending | 4,5,6,7 | Ensures docgen markers in new front-matter context still pass |
| 10 | Verify `make docs-check` passes | high | @tester | [ ] pending | 9 | Run `go run ./internal/docgen --check`; exit 0 = pass |
| 11 | Verify Jekyll site builds locally (optional) | low | @tester | [ ] pending | 9 | `bundle exec jekyll build` in docs/; confirms no Liquid errors |

---

## Parallelization Strategy

**Wave 1** (no dependencies — can run in parallel):
- Task 1: Fix stale docs (`make docs` + commit regenerated files)
- Task 2: Create `scripts/pre-commit` hook script
  
**Wave 2** (depends on Wave 1):
- Task 3: Makefile targets for hooks (depends on Task 2)
- Task 4: Theme migration — Slate → Just the Docs (depends on Task 1)

**Wave 3** (depends on Wave 2):
- Task 5: Create `docs/configuration.md` (depends on Task 4 for front matter conventions)
- Task 7: Update `docs/index.md` (depends on Task 4)
- Task 8: Update `docs/contributing.md` (depends on Task 3)

**Wave 4** (depends on Wave 3):
- Task 6: Update `docs/commands.md` (depends on Task 5)

**Wave 5** (final validation):
- Task 9: Regenerate docs (`make docs`)
- Task 10: Verify `make docs-check` passes
- Task 11: Verify Jekyll build (optional)

---

## Task Descriptions

### Task 1: Fix stale docs — run `make docs` and commit regenerated files

**Goal:** Unblock CI by regenerating all marker-injected documentation content.

**Steps:**
1. Run `make docs` (equivalent to `go run ./internal/docgen`).
2. Review the diff — expect changes in:
   - `docs/commands.md` — sort flags table will gain `--yes` / `-y` and `--no-ledger` rows (if the AST extractor picks them up from `cmd/sort.go`).
   - `README.md` — same sort flags table update.
   - `docs/changelog.md` — synced from root `CHANGELOG.md`.
3. If `--yes` / `--no-ledger` are NOT appearing in the generated table, inspect `cmd/sort.go` to see how they're registered (they may use `BoolP` vs `BoolVarP` or a different pattern that the AST extractor doesn't handle). If so, note this for Task 6 but do NOT fix the extractor in this task — just regenerate what currently works.
4. Stage and commit the regenerated files.
5. Verify: `go run ./internal/docgen --check` exits 0.

**Files modified:** `docs/commands.md`, `README.md`, `docs/changelog.md` (possibly `docs/packages.md`, `docs/how-it-works.md`, `docs/adding-formats.md`, `docs/_config.yml` if other content drifted).

---

### Task 2: Create `scripts/pre-commit` hook script

**Goal:** Create the pre-commit hook shell script per ARCHITECTURE.md §12.2.

**Steps:**
1. Create directory `scripts/` if it doesn't exist.
2. Create `scripts/pre-commit` with the exact content from §12.2:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Only check docs if any relevant source files are staged.
# Relevant files: cmd/*.go, internal/domain/handler.go, internal/handler/**/*.go,
# internal/docgen/*.go, CHANGELOG.md, docs/*.md, docs/_config.yml
RELEVANT_PATTERNS="cmd/.*\.go|internal/domain/handler\.go|internal/handler/.*\.go|internal/docgen/.*\.go|CHANGELOG\.md|docs/.*\.md|docs/_config\.yml"

STAGED=$(git diff --cached --name-only --diff-filter=ACMR)
if ! echo "$STAGED" | grep -qE "$RELEVANT_PATTERNS"; then
    exit 0  # No relevant files staged — skip docs check
fi

echo "pre-commit: checking generated documentation..."
if ! go run ./internal/docgen --check 2>&1; then
    echo ""
    echo "Run 'make docs' to regenerate, then 'git add' the updated files."
    exit 1
fi
```

3. Set executable bit: `chmod +x scripts/pre-commit`.

**Files created:** `scripts/pre-commit`

---

### Task 3: Add `install-hooks` / `uninstall-hooks` Makefile targets

**Goal:** Add opt-in hook installation to the Makefile.

**Steps:**
1. Add to the `.PHONY` list: `install-hooks uninstall-hooks`.
2. Add two new targets after the existing `install` / `uninstall` section:

```makefile
# ---------- git hooks ---------------------------------------
install-hooks: ## Install git pre-commit hook for docs freshness check
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
	@echo "Pre-commit hook installed."

uninstall-hooks: ## Remove git pre-commit hook
	rm -f .git/hooks/pre-commit
	@echo "Pre-commit hook removed."
```

**Files modified:** `Makefile`

---

### Task 4: Migrate Jekyll theme from Slate to Just the Docs

**Goal:** Replace `jekyll-theme-slate` with `just-the-docs` and add front matter navigation to all pages.

**Steps:**

1. **Update `docs/_config.yml`** — Replace the entire file with the configuration from ARCHITECTURE.md §11.3. Key changes:
   - Remove `theme: jekyll-theme-slate`.
   - Add `remote_theme: just-the-docs/just-the-docs`.
   - Add `plugins: [jekyll-remote-theme]`.
   - Add Just the Docs keys: `search_enabled`, `aux_links`, `heading_anchors`, `color_scheme`, `gh_edit_link`.
   - Preserve the `# <!-- pixe:begin:version -->` / `# <!-- pixe:end:version -->` markers and their content.
   - Preserve `url`, `baseurl`, `exclude`.

2. **Update `docs/Gemfile`** — Replace contents with:
   ```ruby
   source "https://rubygems.org"

   gem "jekyll", "~> 4.3"
   gem "just-the-docs"
   gem "jekyll-remote-theme"
   ```

3. **Update front matter on every `.md` file in `docs/`** — Add `nav_order` per the table in §11.4. Each file already has `title:` in front matter. Add `nav_order: N` on the line after `title:`.

   | File | Add to front matter |
   |---|---|
   | `index.md` | `nav_order: 1` and `permalink: /` |
   | `install.md` | `nav_order: 2` |
   | `commands.md` | `nav_order: 3` |
   | `how-it-works.md` | `nav_order: 5` |
   | `technical.md` | `nav_order: 6` |
   | `adding-formats.md` | `nav_order: 7` |
   | `packages.md` | `nav_order: 8` |
   | `contributing.md` | `nav_order: 9` |
   | `changelog.md` | `nav_order: 10` |
   | `ai.md` | `nav_order: 11` |

   Note: `nav_order: 4` is reserved for the new `configuration.md` (Task 5).

4. **Remove `Gemfile.lock`** if present — it will be regenerated with the new gems.

5. **Do NOT add** any `_layouts/`, `_includes/`, `_sass/`, or `assets/` directories. Just the Docs is used purely as a remote theme.

**Files modified:** `docs/_config.yml`, `docs/Gemfile`, `docs/index.md`, `docs/install.md`, `docs/commands.md`, `docs/how-it-works.md`, `docs/technical.md`, `docs/adding-formats.md`, `docs/packages.md`, `docs/contributing.md`, `docs/changelog.md`, `docs/ai.md`
**Files deleted:** `docs/Gemfile.lock` (if present)

---

### Task 5: Create `docs/configuration.md`

**Goal:** Create a comprehensive configuration reference page per ARCHITECTURE.md §11.5.

**Steps:**
1. Create `docs/configuration.md` with front matter:
   ```yaml
   ---
   title: Configuration
   nav_order: 4
   ---
   ```

2. Write the page content following the 8-section structure defined in §11.5:

   **Section 1: Precedence order.** Lead with a clear numbered table showing the 6-level resolution chain (CLI flags → env vars → source-local → profile → global config → defaults). Explain each level in a paragraph below the table.

   **Section 2: Global config file.** Document the three search paths (cwd, home, XDG). Show the YAML format.

   **Section 3: Settings reference table.** A markdown table with columns: Config Key, CLI Flag, Type, Default, Description. Include all 16 config-file keys from §11.5. Follow with a second table for CLI-only flags (`--config`, `--profile`, `--quiet`, `--verbose`, `--progress`, `--yes`, `--no-ledger`).

   **Section 4: Source-local config.** Explain auto-detection of `.pixe.yaml` in `--source` directory. List the 9 mergeable keys. Note additive merge for `ignore` and `aliases`.

   **Section 5: Named profiles.** Explain `--profile <name>`, search paths (`~/.pixe/profiles/<name>.yaml`, `$XDG_CONFIG_HOME/pixe/profiles/<name>.yaml`). Same merge rules as source-local.

   **Section 6: Destination aliases.** Explain the `aliases` map, `@name` syntax, resolution rules, layering. Include a YAML example and a CLI usage example.

   **Section 7: Environment variables.** Document `PIXE_` prefix, `AutomaticEnv()` behavior, bool value formats.

   **Section 8: Full annotated example.** A complete `.pixe.yaml` with every key and inline `#` comments.

3. **Source of truth for content:** Read `cmd/root.go` (config loading, env var setup), `cmd/sort.go` (flag definitions, Viper bindings), `cmd/helpers.go` (`resolveConfig()`, `mergeSourceConfig()`, `resolveAlias()`, `loadProfile()`), and `internal/config/config.go` (AppConfig struct) to ensure accuracy. Cross-reference with the tables in ARCHITECTURE.md §11.5.

**Files created:** `docs/configuration.md`

---

### Task 6: Update `docs/commands.md` — remove config section, add cross-reference

**Goal:** Move config documentation out of commands.md and into the new configuration.md page.

**Steps:**
1. Remove the "Configuration file" section at the bottom of `docs/commands.md` (currently lines 284–302, starting with `---` separator and `## Configuration file` heading through end of file).
2. Replace with a brief cross-reference:
   ```markdown
   ---

   ## Configuration

   For configuration file documentation, precedence rules, profiles, and aliases, see [Configuration](configuration.md).
   ```
3. Verify that the `--yes` / `-y` and `--no-ledger` flags appear in the sort flags table (the `<!-- pixe:begin:sort-flags -->` section). If they were added by `make docs` in Task 1, they should already be present. If not, check whether the docgen AST extractor handles their registration pattern in `cmd/sort.go`. If the extractor misses them, add them manually to the narrative section below the sort flags table with a note about their purpose (ledger write failure prompt behavior per §4.12).

**Files modified:** `docs/commands.md`

---

### Task 7: Update `docs/index.md` — remove manual nav list

**Goal:** Remove the manual navigation link list that the sidebar now replaces.

**Steps:**
1. Remove the "Documentation" section (lines 16–27 in current `index.md`):
   ```markdown
   ## Documentation

   - [Installation](install.md)
   - [Commands](commands.md)
   ...
   - [AI Collaboration](ai.md)
   ```
2. Keep the `---` separator and all content below (the FAQ section, quick start, etc.).
3. Keep the "Get started" and "View on GitHub" links at the top — these are call-to-action links, not navigation.

**Files modified:** `docs/index.md`

---

### Task 8: Update `docs/contributing.md` — document `make install-hooks`

**Goal:** Add hook installation to the contributor setup instructions.

**Steps:**
1. Read `docs/contributing.md` to find the setup/getting-started section.
2. Add a step (after clone/build instructions) recommending `make install-hooks`:
   ```markdown
   ### Git hooks

   Install the pre-commit hook to catch stale documentation before pushing:

   ```bash
   make install-hooks
   ```

   The hook runs `go run ./internal/docgen --check` when you commit changes to CLI flags, handlers, or documentation source files. If docs are stale, it blocks the commit and tells you to run `make docs`.
   ```
3. Keep the addition brief — 3–5 lines.

**Files modified:** `docs/contributing.md`

---

### Task 9: Regenerate docs after all changes — `make docs`

**Goal:** Ensure all docgen-managed content is fresh after the theme migration and page changes.

**Steps:**
1. Run `make docs`.
2. Review the diff. Front matter changes (adding `nav_order`) should not affect marker injection — markers are inside the content body, not front matter.
3. Stage any changes.

**Files potentially modified:** Any file with docgen markers (`docs/commands.md`, `docs/how-it-works.md`, `docs/adding-formats.md`, `docs/packages.md`, `docs/changelog.md`, `docs/_config.yml`, `README.md`).

---

### Task 10: Verify `make docs-check` passes

**Goal:** Confirm CI gate will pass.

**Steps:**
1. Run `go run ./internal/docgen --check`.
2. Expected output: `docgen: all documentation is up to date` with exit code 0.
3. If it fails, identify which files are stale and re-run `make docs`.

---

### Task 11: Verify Jekyll site builds locally (optional)

**Goal:** Confirm the Just the Docs theme renders correctly.

**Steps:**
1. In `docs/`, run `bundle install` then `bundle exec jekyll build`.
2. Check for Liquid errors or missing layout warnings.
3. Optionally run `bundle exec jekyll serve` and verify sidebar navigation, search, and page rendering in a browser.

**Note:** This requires Ruby and Bundler installed locally. If not available, skip — GitHub Pages will build on push and any issues will be visible immediately.
