# Implementation Plan

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Jekyll scaffolding: `_config.yml`, `_data/navigation.yml`, `Gemfile` | high | @developer | [ ] pending | — | Foundation — everything else depends on this |
| 2  | SCSS theme: `_sass/` partials + `assets/css/main.scss` entry point | high | @developer | [ ] pending | 1 | Extract all CSS from original `index.html` into 12 SCSS partials |
| 3  | Layouts: `default.html`, `landing.html`, `page.html` | high | @developer | [ ] pending | 1 | Three layouts per Section 10.6 |
| 4  | Includes: `head.html`, `nav.html`, `footer.html` | high | @developer | [ ] pending | 1, 2 | Shared structural includes used by all pages |
| 5  | Includes: `hero.html`, `pipeline.html`, `format-grid.html` | high | @developer | [ ] pending | 2 | Reusable content components extracted from original HTML |
| 6  | Homepage: `index.md` | high | @developer | [ ] pending | 3, 4, 5 | Landing layout with hero, condensed "why", pipeline, quick start |
| 7  | Install page: `install.md` | medium | @developer | [ ] pending | 3, 4 | Installation methods + quick start examples |
| 8  | Commands page: `commands.md` | medium | @developer | [ ] pending | 3, 4 | Full command reference with accordion components and flag tables |
| 9  | How It Works page: `how-it-works.md` | medium | @developer | [ ] pending | 3, 4, 5 | Pipeline, output format, naming, date fallback, file types grid |
| 10 | Technical Benefits page: `technical.md` | medium | @developer | [ ] pending | 3, 4 | New content: engineering principles and design values |
| 11 | Contributing page: `contributing.md` | medium | @developer | [ ] pending | 3, 4 | Contributing guide with 5-step flow |
| 12 | Adding Formats page: `adding-formats.md` | medium | @developer | [ ] pending | 3, 4 | New content: developer guide for FileTypeHandler |
| 13 | Changelog page: `changelog.md` | low | @developer | [ ] pending | 3, 4 | Mirror of root CHANGELOG.md |
| 14 | AI Statement page: `ai.md` | low | @developer | [ ] pending | 3, 4 | AI collaboration statement with `.ai-card` component |
| 15 | Remove original `docs/index.html` | low | @developer | [ ] pending | 6 | Delete after homepage is in place |
| 16 | Local build verification | high | @tester | [ ] pending | 1–15 | `bundle exec jekyll build` succeeds with no errors |

---

## Task Descriptions

### Task 1 — Jekyll scaffolding: `_config.yml`, `_data/navigation.yml`, `Gemfile`

**Agent:** @developer  
**Priority:** High  
**Depends on:** —

Create the foundational Jekyll configuration files that every other task depends on.

**Files to create:**

1. **`docs/_config.yml`** — Per Section 10.3.2:
   ```yaml
   title: Pixe
   description: Safe, deterministic photo and video sorting
   url: "https://cwlls.github.io"
   baseurl: "/pixe-go"
   version: "v1.8.0"
   tagline: "Your originals are never touched. Every copy is verified before it counts. Interrupted runs resume exactly where they left off."
   markdown: kramdown
   kramdown:
     input: GFM
     syntax_highlighter: rouge
     syntax_highlighter_opts:
       default_lang: bash
   sass:
     sass_dir: _sass
     style: compressed
   exclude:
     - README.md
     - Gemfile
     - Gemfile.lock
   ```
   Note: `version` and `tagline` are custom site variables consumed by `hero.html` (Task 5).

2. **`docs/_data/navigation.yml`** — Per Section 10.3.3:
   ```yaml
   - title: Install
     url: /install/
   - title: Commands
     url: /commands/
   - title: How It Works
     url: /how-it-works/
   - title: Technical
     url: /technical/
   - title: Contributing
     url: /contributing/
   - title: Changelog
     url: /changelog/
   ```

3. **`docs/Gemfile`** — For local development:
   ```ruby
   source "https://rubygems.org"
   gem "jekyll", "~> 4.3"
   gem "kramdown-parser-gfm"
   ```

Also create the empty directory structure needed by subsequent tasks:
- `docs/_includes/`
- `docs/_layouts/`
- `docs/_sass/`
- `docs/assets/css/`

Use `.gitkeep` files if needed to ensure empty directories are tracked, but they will be populated by Tasks 2–5.

---

### Task 2 — SCSS theme: `_sass/` partials + `assets/css/main.scss`

**Agent:** @developer  
**Priority:** High  
**Depends on:** Task 1

Extract all inline CSS from the original `docs/index.html` (lines 7–612) into 12 SCSS partials under `docs/_sass/`. This is a direct decomposition — every CSS rule from the original must appear in exactly one partial. No rules are dropped or modified (except converting to SCSS nesting where natural).

**Files to create:**

1. **`docs/_sass/_variables.scss`** — The `:root` block with all CSS custom properties (lines 11–29 of original). Tokens listed in Section 10.4.1.

2. **`docs/_sass/_reset.scss`** — Box-sizing reset (`*, *::before, *::after`), `html` base, `body` base, `a` base (lines 9, 31–42).

3. **`docs/_sass/_typography.scss`** — `h2`, `h3`, `p`, inline `code`, `pre`, `pre code` styles (lines 110–136, 298–338). Also Markdown-specific rendering styles from Section 10.4.3: blockquotes (`> blockquote` → left border `2px solid var(--accent)`, padding-left `1rem`, color `--text-dim`), list items (color `--text-dim`, `0.5rem` spacing), bold (color `--text`), horizontal rules (`1px solid --border`, `3rem` vertical margin). Also Markdown table styling matching `.flag-table` pattern (monospace first column in `--accent`, `--border` row separators).

4. **`docs/_sass/_nav.scss`** — `nav`, `.nav-brand`, `.nav-links`, `.nav-spacer`, `.nav-gh` (lines 44–95).

5. **`docs/_sass/_layout.scss`** — `.container`, `section` spacing (lines 97–108). Responsive breakpoint at `640px` (lines 589–595).

6. **`docs/_sass/_hero.scss`** — `#hero`, `.hero-tag`, `.hero-title`, `.hero-sub`, `.hero-promise`, `.hero-actions`, `.btn`, `.btn-primary`, `.btn-ghost` (lines 137–206).

7. **`docs/_sass/_cards.scss`** — `.problem-grid`, `.problem-card`, `.problem-q`, `.problem-a`, `.tag-ok`, `.intro-text` (lines 208–258). `.format-grid`, `.format-card`, `.format-name`, `.format-ext` (lines 349–377). `.ai-card`, `.ai-ref` and `#ai` background (lines 511–548).

8. **`docs/_sass/_code.scss`** — `pre`, `code`, `pre code` (if not already in typography), `.pre-label`, terminal color classes (`.term-prompt`, `.term-cmd`, `.term-copy`, `.term-skip`, `.term-dupe`, `.term-err`, `.term-done`, `.term-comment`) (lines 297–348). Also Rouge syntax highlighting theme per Section 10.4.4: `.highlight .c` → `#555`, `.highlight .s` → `#4a9e6e`, `.highlight .k` → `#b5a642`, `.highlight .n`/`.nf` → `#e8e8e8`, `.highlight .err` → `#c0554a`, `.highlight` background → `#0a0a0a`.

9. **`docs/_sass/_tables.scss`** — `.flag-table-wrap`, `.flag-table`, `th`, `td` styles (lines 379–419).

10. **`docs/_sass/_components.scss`** — `.pipeline`, `.pipeline-step`, `.pipeline-arrow` (lines 260–295). `.cmd-block`, `.cmd-header`, `.cmd-name`, `.cmd-desc`, `.cmd-toggle`, `.cmd-body` (lines 421–469). `.contribute-steps`, `.step`, `.step-num`, `.step-text` (lines 471–509). `.callout` (lines 602–612).

11. **`docs/_sass/_footer.scss`** — `footer`, `.footer-inner`, `.footer-brand`, `.footer-links`, `.footer-spacer`, `.footer-copy` (lines 550–579).

12. **`docs/_sass/_utilities.scss`** — `.mt1`, `.mt2`, `.dim`, `.section-label` (lines 597–600, 125–133).

13. **`docs/assets/css/main.scss`** — SCSS entry point with front matter dashes and `@import` for all 12 partials:
    ```scss
    ---
    ---
    @import "variables";
    @import "reset";
    @import "typography";
    @import "nav";
    @import "layout";
    @import "hero";
    @import "cards";
    @import "code";
    @import "tables";
    @import "components";
    @import "footer";
    @import "utilities";
    ```

**Verification:** Every CSS class and rule from the original `<style>` block (lines 7–612) must be accounted for in exactly one partial. No orphaned styles.

---

### Task 3 — Layouts: `default.html`, `landing.html`, `page.html`

**Agent:** @developer  
**Priority:** High  
**Depends on:** Task 1

Create the three Jekyll layouts per Section 10.6.

**Files to create:**

1. **`docs/_layouts/default.html`** — Base layout. Structure:
   ```html
   <!DOCTYPE html>
   <html lang="en">
   <head>{% include head.html %}</head>
   <body>
     {% include nav.html %}
     {{ content }}
     {% include footer.html %}
     <script>
       function toggle(header) {
         const block = header.closest('.cmd-block');
         block.classList.toggle('open');
       }
     </script>
   </body>
   </html>
   ```
   The accordion toggle script is included globally (it's harmless on pages without `.cmd-block` elements).

2. **`docs/_layouts/landing.html`** — Extends `default`. Used only by `index.md`:
   ```html
   ---
   layout: default
   ---
   {% include hero.html %}
   {{ content }}
   ```
   Note: The `{{ content }}` from `index.md` provides the "Why Pixe" section and pipeline. The landing layout does NOT wrap content in a `<section>` — the Markdown content in `index.md` includes its own section wrappers via HTML blocks.

3. **`docs/_layouts/page.html`** — Extends `default`. Used by all inner pages:
   ```html
   ---
   layout: default
   ---
   <section>
     <div class="container">
       {% if page.section_label %}
         <div class="section-label">{{ page.section_label }}</div>
       {% endif %}
       <h2>{{ page.title }}</h2>
       {{ content }}
     </div>
   </section>
   ```
   Pages set `section_label` and `title` in their YAML front matter.

---

### Task 4 — Includes: `head.html`, `nav.html`, `footer.html`

**Agent:** @developer  
**Priority:** High  
**Depends on:** Tasks 1, 2

Create the three structural includes that appear on every page.

**Files to create:**

1. **`docs/_includes/head.html`** — The `<head>` block:
   ```html
   <meta charset="UTF-8" />
   <meta name="viewport" content="width=device-width, initial-scale=1.0" />
   <title>{% if page.title %}{{ page.title }} — {{ site.title }}{% else %}{{ site.title }} — {{ site.description }}{% endif %}</title>
   <link rel="stylesheet" href="{{ '/assets/css/main.css' | relative_url }}">
   ```

2. **`docs/_includes/nav.html`** — Sticky top nav bar. Extracted from original lines 617–629. Must iterate `site.data.navigation` for links and highlight the current page:
   ```html
   <nav>
     <a class="nav-brand" href="{{ '/' | relative_url }}">pixe</a>
     <ul class="nav-links">
       {% for item in site.data.navigation %}
         <li>
           <a href="{{ item.url | relative_url }}"{% if page.url == item.url %} style="color: var(--text)"{% endif %}>
             {{ item.title }}
           </a>
         </li>
       {% endfor %}
     </ul>
     <div class="nav-spacer"></div>
     <a class="nav-gh" href="https://github.com/cwlls/pixe-go" target="_blank" rel="noopener">GitHub ↗</a>
   </nav>
   ```
   The brand link points to site root. The GitHub button is hardcoded (not data-driven). Active page highlighting uses `page.url` comparison.

3. **`docs/_includes/footer.html`** — Extracted from original lines 1022–1036:
   ```html
   <footer>
     <div class="container">
       <div class="footer-inner">
         <span class="footer-brand">pixe</span>
         <ul class="footer-links">
           <li><a href="https://github.com/cwlls/pixe-go" target="_blank" rel="noopener">GitHub</a></li>
           <li><a href="https://github.com/cwlls/pixe-go/issues" target="_blank" rel="noopener">Issues</a></li>
           <li><a href="https://github.com/cwlls/pixe-go/blob/main/LICENSE" target="_blank" rel="noopener">Apache 2.0</a></li>
           <li><a href="{{ '/ai/' | relative_url }}">AI collaboration</a></li>
         </ul>
         <div class="footer-spacer"></div>
         <span class="footer-copy">© 2026 Chris Wells</span>
       </div>
     </div>
   </footer>
   ```
   Note: The "AI collaboration" link uses `relative_url` to point to the AI page within the Jekyll site.

---

### Task 5 — Includes: `hero.html`, `pipeline.html`, `format-grid.html`

**Agent:** @developer  
**Priority:** High  
**Depends on:** Task 2

Create the three reusable content component includes.

**Files to create:**

1. **`docs/_includes/hero.html`** — Extracted from original lines 632–643. Uses `site.version`, `site.description`, and `site.tagline` from `_config.yml`:
   ```html
   <section id="hero">
     <div class="container">
       <div class="hero-tag">{{ site.version }} · Apache 2.0</div>
       <h1 class="hero-title">pixe</h1>
       <p class="hero-sub">{{ site.description | capitalize }}.</p>
       <p class="hero-promise">{{ site.tagline }}</p>
       <div class="hero-actions">
         <a class="btn btn-primary" href="{{ '/install/' | relative_url }}">Get Started</a>
         <a class="btn btn-ghost" href="https://github.com/cwlls/pixe-go" target="_blank" rel="noopener">View on GitHub</a>
       </div>
     </div>
   </section>
   ```
   The "Get Started" button links to the Install page (not an anchor on the same page).

2. **`docs/_includes/pipeline.html`** — Extracted from original lines 694–718. The pipeline visualization:
   ```html
   <div class="pipeline">
     <span class="pipeline-step active">discover</span>
     <span class="pipeline-arrow">→</span>
     <span class="pipeline-step">extract</span>
     <span class="pipeline-arrow">→</span>
     <span class="pipeline-step">hash</span>
     <span class="pipeline-arrow">→</span>
     <span class="pipeline-step">copy</span>
     <span class="pipeline-arrow">→</span>
     <span class="pipeline-step">verify</span>
     <span class="pipeline-arrow">→</span>
     <span class="pipeline-step">tag</span>
     <span class="pipeline-arrow">→</span>
     <span class="pipeline-step">complete</span>
   </div>
   ```

3. **`docs/_includes/format-grid.html`** — Extracted from original lines 740–777. The supported file types grid with all 9 formats:
   ```html
   <div class="format-grid">
     <div class="format-card">
       <div class="format-name">JPEG</div>
       <div class="format-ext">.jpg &nbsp;.jpeg</div>
     </div>
     <!-- ... HEIC, Video, DNG, Nikon RAW, Canon RAW 2, Canon RAW 3, Pentax RAW, Sony RAW ... -->
   </div>
   ```
   All 9 format cards from the original, verbatim.

---

### Task 6 — Homepage: `index.md`

**Agent:** @developer  
**Priority:** High  
**Depends on:** Tasks 3, 4, 5

Create the homepage content file. This uses the `landing` layout, which provides the hero section via include. The Markdown content provides the "Why Pixe" section and a condensed pipeline + quick start.

**File to create:** `docs/index.md`

**Front matter:**
```yaml
---
layout: landing
title: Pixe — Safe, Deterministic Photo Sorting
---
```

**Content structure:**

1. **Why Pixe section** — The six problem/answer cards from the original (lines 646–684). Since these use custom HTML classes (`.problem-grid`, `.problem-card`, etc.), they are written as raw HTML blocks within the Markdown file. Include the `<section id="why">` wrapper with `.container`, the `.section-label` ("Purpose"), the `<h2>`, the `.intro-text` paragraph, and all six `.problem-card` divs.

2. **Pipeline section** — A brief "How it works" teaser. Use `{% include pipeline.html %}` to render the pipeline visualization. Include the one-line stage descriptions from the original (lines 710–718). Link to the full How It Works page.

3. **Quick start snippet** — A condensed 3-4 line terminal example:
   ```
   $ pixe sort --dest ~/Archive
   $ pixe sort --source ~/Photos --dest ~/Archive --recursive
   ```
   With a link: "See all commands →" pointing to `/commands/`.

---

### Task 7 — Install page: `install.md`

**Agent:** @developer  
**Priority:** Medium  
**Depends on:** Tasks 3, 4

Create the installation page. Content carried from the original Install section (lines 782–812).

**File to create:** `docs/install.md`

**Front matter:**
```yaml
---
layout: page
title: Get started in minutes
section_label: Installation
permalink: /install/
---
```

**Content sections:**
- **Install via Go** — `go install github.com/cwlls/pixe-go@latest` (requires Go 1.21+)
- **Build from source** — `git clone`, `cd`, `make build`
- **Quick start** — The four terminal examples from the original: basic sort, explicit source + recursive, status check, verify
- **Dry-run tip** — The `.callout` block: "Run `pixe sort --dry-run` first to preview..."
- **Link to Commands** — "See the full command reference →"

---

### Task 8 — Commands page: `commands.md`

**Agent:** @developer  
**Priority:** Medium  
**Depends on:** Tasks 3, 4

Create the comprehensive command reference page. This is the largest content page. Content sourced from the original Usage section (lines 816–957) and expanded with all commands from ARCHITECTURE.md Section 7.1.

**File to create:** `docs/commands.md`

**Front matter:**
```yaml
---
layout: page
title: Commands
section_label: Reference
permalink: /commands/
---
```

**Content structure:**

Each command uses the `.cmd-block` accordion component (raw HTML within Markdown). The `pixe sort` block has class `open` by default; all others are collapsed.

Commands to document (in order), each with signature, description, flag table (`.flag-table-wrap` > `.flag-table`), and usage examples:

1. **`pixe sort`** — Flags: `-s/--source`, `-d/--dest` (required), `-w/--workers`, `-a/--algorithm`, `-r/--recursive`, `--ignore`, `--skip-duplicates`, `--copyright`, `--camera-owner`, `--dry-run`, `--db-path`. Source from ARCHITECTURE.md Section 7.1 `pixe sort` table.
2. **`pixe status`** — Flags: `-s/--source`, `-r/--recursive`, `--ignore`, `--json`. Categories: SORTED, DUPLICATE, ERRORED, UNSORTED, UNRECOGNIZED.
3. **`pixe verify`** — Flags: `-d/--dir` (required), `-w/--workers`, `-a/--algorithm`. Exit codes.
4. **`pixe resume`** — Flags: `-d/--dir` (required), `--db-path`.
5. **`pixe query`** — Parent flags: `--dir`, `--db-path`, `--json`. Then each subcommand as a nested section: `runs`, `run <id>` (with prefix matching note), `duplicates` (with `--pairs`), `errors`, `skipped`, `files` (with `--from`/`--to`/`--imported-from`/`--imported-to`/`--source`), `inventory`.
6. **`pixe clean`** — Flags: `-d/--dir` (required), `--db-path`, `--dry-run`, `--temp-only`, `--vacuum-only`. Note mutual exclusivity.
7. **`pixe version`** — No flags. Output format example.

**Configuration file section** at the end — `.pixe.yaml` example with all supported keys (`algorithm`, `workers`, `copyright`, `camera_owner`, `recursive`, `skip_duplicates`, `ignore`). Note on config file locations (cwd, home, `$XDG_CONFIG_HOME/pixe`). Note on env var prefix `PIXE_`.

---

### Task 9 — How It Works page: `how-it-works.md`

**Agent:** @developer  
**Priority:** Medium  
**Depends on:** Tasks 3, 4, 5

Create the internals page. Content expanded from the original "How It Works" section (lines 688–778) with additional detail from ARCHITECTURE.md Sections 4.2–4.8.

**File to create:** `docs/how-it-works.md`

**Front matter:**
```yaml
---
layout: page
title: How it works
section_label: Internals
permalink: /how-it-works/
---
```

**Content sections:**

1. **Pipeline stages** — Opening paragraph about the linear pipeline. `{% include pipeline.html %}`. Then detailed descriptions of each stage: discover, extract, hash, copy, verify, tag, complete. Include the error states (failed, mismatch, tag_failed). Source: ARCHITECTURE.md Section 4.2.

2. **Output format** — The four outcome verbs with terminal-styled examples (use raw HTML `<pre>` blocks with `.term-*` classes for colored output). Source: original lines 721–728.

3. **Output naming** — The `YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` convention. Directory structure `<YYYY>/<MM-Mon>/`. Example path. Locale-aware month names note. Source: original lines 730–734, ARCHITECTURE.md Section 4.5.

4. **Date fallback chain** — EXIF DateTimeOriginal → CreateDate → February 20, 1902 (Ansel Adams). Source: original line 734, ARCHITECTURE.md Section 4.7.

5. **Duplicate handling** — Default behavior (copy to `duplicates/<run_timestamp>/...`) vs `--skip-duplicates`. Source: ARCHITECTURE.md Section 4.6.

6. **Archive database** — SQLite at `<dest>/.pixe/<slug>.db`. What it tracks. WAL mode. Source: original lines 736–737, ARCHITECTURE.md Section 8.1.

7. **Supported file types** — `{% include format-grid.html %}`. All 9 formats with extensions.

---

### Task 10 — Technical Benefits page: `technical.md`

**Agent:** @developer  
**Priority:** Medium  
**Depends on:** Tasks 3, 4

Create the technical design values page. This is **new content** — not carried from the original site. It is a prose-focused page explaining engineering principles. Source material: ARCHITECTURE.md Sections 4.10, 5.1–5.4.

**File to create:** `docs/technical.md`

**Front matter:**
```yaml
---
layout: page
title: Why Pixe is built this way
section_label: Design
permalink: /technical/
---
```

**Content sections (each 2-3 short paragraphs, no code):**

1. **Source files are never touched** — `dirA` is read-only. Only `.pixe_ledger.json` is written there. Why: irreplaceable family photos cannot tolerate any risk of modification. Even a metadata write to the wrong offset could corrupt a file.

2. **Copy-then-verify** — Every file is written to a temp file, independently re-hashed, and only renamed to its canonical path when hashes match. Why: USB drives drop bytes, NAS connections hiccup, disks develop bad sectors. A checksum on both sides catches what the filesystem silently accepts.

3. **Deterministic output** — Same input files + same config = same archive structure, always. Why: you can re-run Pixe on the same source and get the same result. You can merge archives from different sources with confidence. You can verify an archive years later.

4. **No external dependencies** — Single binary. No exiftool, no ffmpeg, no ImageMagick. All EXIF parsing, RAW decoding, and video metadata extraction is pure Go. Why: the tool should work in 10 years without a dependency chain. Install it, run it, done.

5. **Crash-safe by design** — Each file completion is committed individually to SQLite. The JSONL ledger is streamed (each line flushed immediately). Temp files are atomically renamed. An interrupted run loses at most one in-flight file and resumes cleanly. Why: sorting 50,000 photos takes hours. Power failures happen. The tool must survive them.

6. **Content-based deduplication** — Checksums are computed over the media payload (pixel data, sensor data), not filenames or metadata. `IMG_0001.jpg` from your phone and `IMG_0001.jpg` from your partner's phone are correctly identified as different files. The same photo renamed to `vacation_sunset.jpg` is correctly identified as a duplicate. Why: filenames are meaningless for identity. Content is truth.

---

### Task 11 — Contributing page: `contributing.md`

**Agent:** @developer  
**Priority:** Medium  
**Depends on:** Tasks 3, 4

Create the contributing guide. Content carried from the original Contributing section (lines 960–998).

**File to create:** `docs/contributing.md`

**Front matter:**
```yaml
---
layout: page
title: Contributing to Pixe
section_label: Open Source
permalink: /contributing/
---
```

**Content:**
- Opening paragraphs about Apache 2.0 license, welcome contributions (bug reports, format handlers, features, docs)
- Note about `FileTypeHandler` interface making new format support straightforward
- The 5-step contributing flow using raw HTML with `.contribute-steps` / `.step` / `.step-num` / `.step-text` classes (carried from original lines 967–995). Steps: (01) Open an issue first, (02) Clone and build, (03) Run test suite, (04) Follow conventions, (05) Submit PR
- Link to GitHub Issues

---

### Task 12 — Adding Formats page: `adding-formats.md`

**Agent:** @developer  
**Priority:** Medium  
**Depends on:** Tasks 3, 4

Create the developer guide for implementing a new `FileTypeHandler`. This is **new content**. Source material: ARCHITECTURE.md Sections 6.1–6.4.

**File to create:** `docs/adding-formats.md`

**Front matter:**
```yaml
---
layout: page
title: Adding a new file format
section_label: Developer Guide
permalink: /adding-formats/
---
```

**Content sections:**

1. **Overview** — Pixe's format support is modular. Each format is an isolated package under `internal/handler/`. The core pipeline is format-agnostic — it processes files through the `FileTypeHandler` interface without knowing anything about JPEG, HEIC, or RAW internals.

2. **The `FileTypeHandler` interface** — Full Go interface definition in a code block (from ARCHITECTURE.md Section 6.1). Then a plain-language explanation of each method:
   - `Detect(filePath)` — Magic-byte verification after extension-based assumption
   - `ExtractDate(filePath)` — Capture date from metadata, with fallback chain
   - `HashableReader(filePath)` — Reader over the media payload only (not metadata)
   - `MetadataSupport()` — Declares embed/sidecar/none capability
   - `WriteMetadataTags(filePath, tags)` — In-file metadata injection (embed only)
   - `Extensions()` — Lowercase extensions this handler claims
   - `MagicBytes()` — Byte signatures for file type verification

3. **Step-by-step walkthrough** — Using WEBP as a hypothetical example:
   - Create `internal/handler/webp/webp.go`
   - Define `Handler` struct, `New()` constructor
   - Implement `Extensions()` → `[]string{".webp"}`
   - Implement `MagicBytes()` → RIFF header + WEBP signature
   - Implement `Detect()` — check extension, then verify magic bytes
   - Implement `ExtractDate()` — parse EXIF from WEBP container, apply fallback chain
   - Implement `HashableReader()` — return reader over image data (VP8/VP8L payload)
   - Implement `MetadataSupport()` → choose `MetadataSidecar` (safest default for new formats)
   - Implement `WriteMetadataTags()` → no-op stub (sidecar handler)
   - Register in `cmd/sort.go`, `cmd/verify.go`, `cmd/resume.go`, `cmd/status.go`

4. **TIFF-based RAW shortcut** — If the format is TIFF-based, embed `tiffraw.Base` and only implement `Extensions()`, `MagicBytes()`, `Detect()`. Reference DNG/NEF/CR2/PEF/ARW as templates. Show the minimal handler code (~30 lines).

5. **Testing conventions** — stdlib `testing` only (no testify), `t.TempDir()` for filesystem tests, `t.Helper()` on helpers, `-race` always, test names `TestTypeName_behavior`, fixture files in `testdata/`.

---

### Task 13 — Changelog page: `changelog.md`

**Agent:** @developer  
**Priority:** Low  
**Depends on:** Tasks 3, 4

Create the changelog page. Per Section 10.5.8, Option A: the content is manually kept in sync with the root `CHANGELOG.md`.

**File to create:** `docs/changelog.md`

**Front matter:**
```yaml
---
layout: page
title: Changelog
section_label: History
permalink: /changelog/
---
```

**Content:** Copy the full content of the root `CHANGELOG.md` (minus the `# Changelog: Pixe` H1 heading, since the layout provides the title). The content starts from the description line and includes all version entries.

---

### Task 14 — AI Statement page: `ai.md`

**Agent:** @developer  
**Priority:** Low  
**Depends on:** Tasks 3, 4

Create the AI collaboration statement page. Content carried from the original AI section (lines 1002–1018).

**File to create:** `docs/ai.md`

**Front matter:**
```yaml
---
layout: page
title: Built with AI collaboration
section_label: Transparency
permalink: /ai/
---
```

**Content:** The three paragraphs from the original AI section, wrapped in the `.ai-card` component (raw HTML div with class `ai-card`). Include the `.ai-ref` div with the link to `daplin.org/ai-collaboration.html`.

---

### Task 15 — Remove original `docs/index.html`

**Agent:** @developer  
**Priority:** Low  
**Depends on:** Task 6

Delete the original `docs/index.html` file. All of its content has been decomposed into the Jekyll theme (SCSS partials, layouts, includes) and Markdown content pages. The file is no longer needed and would conflict with Jekyll's `index.md`.

**Action:** `git rm docs/index.html`

---

### Task 16 — Local build verification

**Agent:** @tester  
**Priority:** High  
**Depends on:** Tasks 1–15

Verify the Jekyll site builds successfully with no errors.

**Steps:**
1. `cd docs && bundle install`
2. `bundle exec jekyll build`
3. Verify exit code 0 and no error output
4. Verify `docs/_site/` contains: `index.html`, `install/index.html`, `commands/index.html`, `how-it-works/index.html`, `technical/index.html`, `contributing/index.html`, `adding-formats/index.html`, `changelog/index.html`, `ai/index.html`
5. Verify `docs/_site/assets/css/main.css` exists and is non-empty
6. Spot-check that the generated HTML contains expected elements (nav links, hero section on index, section labels on inner pages)

If `bundle` is not available, verify with `jekyll build` directly or confirm the file structure is correct and all includes/layouts reference existing files.
