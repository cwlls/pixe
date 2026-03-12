# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Marker injection engine (`internal/docgen/inject.go`) | high | @developer | [ ] pending | — | Core infrastructure: parse markers, replace content, write-if-changed, --check mode |
| 2  | Output formatters (`internal/docgen/formats.go`) | high | @developer | [ ] pending | — | Markdown table, HTML table, fenced code block, YAML value formatters |
| 3  | Version extractor (`internal/docgen/extract.go`) | high | @developer | [ ] pending | 1, 2 | `git describe --tags --abbrev=0` → YAML value; also recognizes `# <!-- pixe:begin:... -->` YAML comment markers |
| 4  | Interface extractor (`internal/docgen/extract.go`) | high | @developer | [ ] pending | 1, 2 | Parse `internal/domain/handler.go` AST for `FileTypeHandler` interface + `MetadataCapability` const block → fenced Go code block |
| 5  | CLI flag extractor (`internal/docgen/extract.go`) | high | @developer | [ ] pending | 1, 2 | Parse `cmd/*.go` AST for Cobra `Flags().StringVarP()` / `BoolVarP()` / `IntVarP()` / `PersistentFlags().*` calls → flag table (name, short, default, description) |
| 6  | Format table extractor (`internal/docgen/extract.go`) | high | @developer | [ ] pending | 1, 2 | Parse `internal/handler/*/` AST for `Extensions()` return slice, `MetadataSupport()` return constant, package doc comment → format table rows |
| 7  | Package reference extractor (`internal/docgen/extract.go`) | medium | @developer | [ ] pending | 1, 2 | Parse all `internal/` and `cmd/` packages for `// Package` doc comments → grouped listing by category |
| 8  | Query subcommand extractor (`internal/docgen/extract.go`) | medium | @developer | [ ] pending | 5 | Parse `cmd/query_*.go` for `cobra.Command` struct literals → subcommand name, description, flags |
| 9  | Target manifest and main entry point (`internal/docgen/main.go`) | high | @developer | [ ] pending | 1–8 | Hardcoded `[]Target` mapping files to section→extractor; `--check` flag; orchestration loop |
| 10 | Tests for injection engine | high | @developer | [ ] pending | 1 | Marker parsing, replacement, idempotency, error cases (unpaired markers, unknown sections) |
| 11 | Tests for extractors | high | @developer | [ ] pending | 3–8 | Test each extractor against real source files; verify deterministic output |
| 12 | Add markers to `docs/_config.yml` | medium | @developer | [ ] pending | 3 | Wrap `version:` line with `# <!-- pixe:begin:version -->` / `# <!-- pixe:end:version -->` YAML comment markers |
| 13 | Add markers to `docs/adding-formats.md` | medium | @developer | [ ] pending | 4 | Wrap the `FileTypeHandler` interface code block with `<!-- pixe:begin:interface -->` / `<!-- pixe:end:interface -->` |
| 14 | Add markers to `docs/commands.md` | medium | @developer | [ ] pending | 5, 8 | Wrap each command's flag table with `<!-- pixe:begin:<cmd>-flags -->` markers; wrap query subcommand table with `<!-- pixe:begin:query-subs -->` |
| 15 | Add markers to `docs/how-it-works.md` | medium | @developer | [ ] pending | 6 | Wrap the supported file types section with `<!-- pixe:begin:format-table -->` |
| 16 | Add markers to `README.md` | medium | @developer | [ ] pending | 5, 6, 8 | Wrap flag tables (sort, verify, resume, status, clean, gui, query), query subcommands table, and format table with markers |
| 17 | Create `docs/packages.md` | medium | @developer | [ ] pending | 7 | New page with front matter, hand-authored intro, and `<!-- pixe:begin:package-list -->` marker block |
| 18 | Update `docs/_data/navigation.yml` | low | @developer | [ ] pending | 17 | Reorder nav: end-user pages first, developer pages grouped, add Packages entry |
| 19 | Add Makefile targets (`docs`, `docs-check`) | medium | @developer | [ ] pending | 9 | `make docs` → `go run ./internal/docgen`; `make docs-check` → `go run ./internal/docgen --check`; update `check` target to include `docs-check` |
| 20 | Update CI workflow | low | @developer | [ ] pending | 19 | Add `docs-check` step to `.github/workflows/ci.yml`; remove stale `.github/workflows/pages.yml` |
| 21 | Run `make docs` and verify end-to-end | high | @tester | [ ] pending | 9–19 | Run the tool, verify all markers are populated, verify `--check` passes, verify idempotency |

---

## Task Descriptions

### Task 1: Marker injection engine (`internal/docgen/inject.go`)

Build the core file-processing engine that reads a file, finds `<!-- pixe:begin:NAME -->` / `<!-- pixe:end:NAME -->` marker pairs, and replaces the content between them with generated output.

**Key types and functions:**

```go
// Replacement holds the generated content for a single marker section.
type Replacement struct {
    Name    string // marker section name (e.g., "sort-flags")
    Content string // generated content to inject (excluding markers themselves)
}

// InjectFile reads the file at path, replaces all marker sections with the
// provided replacements, and returns the resulting content. It does not write
// the file — the caller decides whether to write (normal mode) or compare
// (--check mode). Returns an error if a marker pair is malformed (begin
// without end, or end without begin).
func InjectFile(path string, replacements []Replacement) (string, error)

// WriteIfChanged writes content to path only if it differs from the current
// file contents. Returns true if the file was written, false if unchanged.
// Preserves the file's original permissions.
func WriteIfChanged(path string, content string) (bool, error)
```

**Marker regex:** `<!-- pixe:begin:([a-z0-9-]+)(?:\s+[^>]*)? -->` and `<!-- pixe:end:([a-z0-9-]+) -->`. Also recognize the YAML-comment variant: `# <!-- pixe:begin:... -->`.

**Error cases to handle:**
- Begin marker without matching end → fatal error with line number
- End marker without matching begin → fatal error with line number
- Nested markers (begin inside begin) → fatal error
- Replacement provided for a section name not found in the file → warning (non-fatal)

---

### Task 2: Output formatters (`internal/docgen/formats.go`)

Implement output formatting functions that convert extracted data into the appropriate string representation for injection.

**Functions:**

```go
// FormatMarkdownTable renders rows as a GitHub-Flavored Markdown table.
// headers is the first row; rows are subsequent data rows.
func FormatMarkdownTable(headers []string, rows [][]string) string

// FormatHTMLTable renders rows as an HTML <table> with class "flag-table".
// Used for docs/commands.md where the Jekyll theme styles HTML tables.
func FormatHTMLTable(headers []string, rows [][]string) string

// FormatGoCodeBlock wraps Go source text in a fenced ```go code block.
func FormatGoCodeBlock(source string) string

// FormatYAMLValue renders a single key: "value" YAML line.
func FormatYAMLValue(key string, value string) string
```

All formatters must produce deterministic output — no trailing whitespace, consistent newlines, no optional formatting variations.

---

### Task 3: Version extractor

Extract the current version from the latest git tag.

```go
// extractVersion returns a Replacement containing the version string
// from `git describe --tags --abbrev=0`. Falls back to "dev" if no
// tags exist. Output format for YAML: `version: "v2.0.0"`
func extractVersion() (string, error)
```

**Implementation:** Use `os/exec` to run `git describe --tags --abbrev=0`. This is the one place `os/exec` is acceptable — it's a dev-time tool, not the shipped binary. Trim whitespace from output. If the command fails (no tags), return `"dev"`.

**YAML marker note:** The `_config.yml` file uses `# <!-- pixe:begin:version -->` (YAML comment prefix). The injection engine (Task 1) must handle this variant — the `#` prefix is stripped when scanning for markers and re-emitted when writing output.

---

### Task 4: Interface extractor

Extract the `FileTypeHandler` interface and `MetadataCapability` type from `internal/domain/handler.go`.

```go
// extractInterface parses internal/domain/handler.go and extracts:
// 1. The MetadataCapability type declaration and its const block
//    (MetadataNone, MetadataEmbed, MetadataSidecar) with doc comments.
// 2. The FileTypeHandler interface type spec with all method signatures
//    and their doc comments.
// Returns the combined source as a fenced Go code block.
func extractInterface() (string, error)
```

**Implementation:** Use `go/parser.ParseFile` with `parser.ParseComments` to parse `handler.go`. Walk the AST to find:
- The `MetadataCapability` type spec (`ast.GenDecl` with `token.TYPE`)
- The const block containing `MetadataNone`, `MetadataEmbed`, `MetadataSidecar` (`ast.GenDecl` with `token.CONST`)
- The `FileTypeHandler` interface type spec

For each, extract the source text from the original file bytes using the node's `Pos()` and `End()` positions. Include associated doc comments. Combine into a single string and wrap with `FormatGoCodeBlock()`.

---

### Task 5: CLI flag extractor

Extract Cobra flag registrations from `cmd/*.go` files.

```go
// Flag represents a single CLI flag extracted from Cobra registration.
type Flag struct {
    Long        string // e.g., "source"
    Short       string // e.g., "s" (empty if none)
    Default     string // e.g., "" or "sha1" or "false"
    Description string // usage string from the registration call
}

// extractFlags returns a function that parses the given cmd file and
// extracts all Cobra flag registrations. The format parameter controls
// output: "markdown" for GFM tables, "html" for styled HTML tables.
func extractFlags(cmdFile string, format string) func() (string, error)
```

**AST patterns to match:**

Cobra flags are registered via method chains on `cmd.Flags()` or `cmd.PersistentFlags()`. The patterns are:

```go
// StringVarP(&var, "long", "s", "default", "description")
// BoolVarP(&var, "long", "b", false, "description")
// IntVarP(&var, "long", "i", 0, "description")
// StringVar(&var, "long", "default", "description")  // no short form
// BoolVar(&var, "long", false, "description")
```

Parse the file, find all `CallExpr` nodes where the function name matches `StringVarP`, `BoolVarP`, `IntVarP`, `StringVar`, `BoolVar`, `IntVar`. Extract the string literal arguments by position:
- `*VarP`: args[1]=long, args[2]=short, args[3]=default, args[4]=description
- `*Var`: args[1]=long, args[2]=default, args[3]=description (no short)

Also extract the `cobra.Command` struct literal's `Use`, `Short`, and `Long` fields for command description.

**Output:** Format the extracted flags into a table using `FormatMarkdownTable` or `FormatHTMLTable` based on the `format` parameter. Columns: `Flag` (formatted as `-s, --long` or `--long`), `Description`.

---

### Task 6: Format table extractor

Extract supported file format information from handler packages.

```go
// HandlerInfo represents one file type handler's extracted metadata.
type HandlerInfo struct {
    PackageName string   // e.g., "jpeg"
    DisplayName string   // e.g., "JPEG" (derived from package name, uppercased)
    Extensions  []string // e.g., [".jpg", ".jpeg"]
    Metadata    string   // e.g., "Embedded EXIF" or "XMP sidecar"
    DocComment  string   // package-level doc comment (first sentence)
}

// extractFormats returns a function that scans all handler packages under
// internal/handler/ and extracts format metadata. The format parameter
// controls output: "markdown" or "html".
func extractFormats(format string) func() (string, error)
```

**Implementation:**
1. List directories under `internal/handler/` (skip `tiffraw` — it's a shared base, not a user-facing format).
2. For each handler package, parse the primary `.go` file (same name as directory).
3. Find the `Extensions()` method and extract the returned `[]string` literal.
4. Find the `MetadataSupport()` method and extract the returned constant name. Map `MetadataEmbed` → `"Embedded EXIF"`, `MetadataSidecar` → `"XMP sidecar"`, `MetadataNone` → `"None"`.
5. Extract the package doc comment's first sentence.
6. Sort handlers alphabetically by package name.
7. Format as a table with columns: Format, Extensions, Metadata.

**Special case:** MP4 handler covers both `.mp4` and `.mov` — display name should be `"MP4/MOV"`.

---

### Task 7: Package reference extractor

Extract package-level doc comments from all internal packages and group them.

```go
// PackageInfo represents one package's extracted documentation.
type PackageInfo struct {
    ImportPath string // e.g., "internal/pipeline"
    Name       string // e.g., "pipeline"
    DocComment string // full package doc comment text
}

// extractPackageReference scans all packages under internal/ and cmd/,
// extracts their package doc comments, groups them by category, and
// returns formatted Markdown.
func extractPackageReference() (string, error)
```

**Implementation:**
1. Walk `internal/` and `cmd/` directories.
2. For each directory containing `.go` files, parse one file to extract the package doc comment (use `go/parser.ParseDir` or parse individual files with `parser.ParseComments`).
3. Group packages according to the hardcoded category map (from Architecture Section 15.7):
   - **Core Engine:** pipeline, discovery, copy, verify, hash, pathbuilder
   - **Data & Persistence:** archivedb, manifest, migrate, dblocator, domain, config
   - **File Type Handlers:** handler/jpeg, handler/heic, handler/mp4, handler/tiffraw, handler/dng, handler/nef, handler/cr2, handler/cr3, handler/pef, handler/arw
   - **Metadata:** tagging, xmp, ignore
   - **User Interface:** progress, cli, tui
4. Emit as Markdown with `### Group Name` headers and `**\`import/path\`** — first paragraph of doc comment` entries.
5. Packages not in any group are listed under an "Other" section (future-proofing).

---

### Task 8: Query subcommand extractor

Extract query subcommand definitions from `cmd/query_*.go` files.

```go
// extractQuerySubcommands returns a function that parses all cmd/query_*.go
// files and extracts the cobra.Command struct literal for each subcommand.
// Returns a table of subcommand name, description, and any subcommand-specific flags.
func extractQuerySubcommands(format string) func() (string, error)
```

**Implementation:** Reuse the AST patterns from Task 5. For each `cmd/query_*.go` file (excluding `cmd/query.go` which is the parent command and `cmd/query_format.go` which is a helper):
1. Find the `cobra.Command` struct literal.
2. Extract `Use` (parse the subcommand name from the `Use` string, e.g., `"runs"` from `"runs [flags]"`).
3. Extract `Short` for the description.
4. Extract any subcommand-specific flags.
5. Format as a table with columns: Subcommand, Description.

---

### Task 9: Target manifest and main entry point (`internal/docgen/main.go`)

Wire everything together in the `main.go` entry point.

```go
package main

// Target defines a documentation file and its injectable sections.
type Target struct {
    File     string                        // relative path from repo root
    Sections map[string]func() (string, error) // section name → extractor
}

// main parses --check flag, builds the target manifest, runs all
// extractors, injects into target files, and either writes (default)
// or compares (--check mode).
func main()
```

**Behavior:**
- **Default mode:** For each target, run extractors, inject, write if changed. Print summary of updated/unchanged files to stderr.
- **`--check` mode:** For each target, run extractors, inject into memory, compare against file on disk. If any differ, print stale files to stderr and exit with code 1.
- **Exit codes:** 0 = success (all up to date or all written), 1 = stale files found (--check mode) or fatal error.

The target manifest is the hardcoded `[]Target` slice from Architecture Section 15.5.3. The `Extractor` type is `func() (string, error)` — each extractor returns the content to inject (without markers).

---

### Task 10: Tests for injection engine

Test the marker injection engine in `internal/docgen/docgen_test.go` (or `inject_test.go`).

**Test cases:**
- `TestInjectFile_basicReplacement` — single marker pair, content replaced
- `TestInjectFile_multipleMarkers` — multiple marker pairs in one file
- `TestInjectFile_preservesSurroundingContent` — content outside markers is untouched
- `TestInjectFile_idempotent` — injecting the same content twice produces identical output
- `TestInjectFile_yamlCommentMarkers` — `# <!-- pixe:begin:... -->` variant works
- `TestInjectFile_unpairedBegin` — begin without end returns error
- `TestInjectFile_unpairedEnd` — end without begin returns error
- `TestInjectFile_nestedMarkers` — begin inside begin returns error
- `TestInjectFile_unknownSection` — replacement for non-existent section is a no-op (warning)
- `TestWriteIfChanged_writesWhenDifferent` — file is written when content differs
- `TestWriteIfChanged_skipsWhenIdentical` — file is not written when content matches

Use `t.TempDir()` for filesystem tests. Follow project conventions: stdlib `testing` only, `t.Helper()` on helpers, `TestTypeName_behavior` naming.

---

### Task 11: Tests for extractors

Test each extractor against the real codebase source files.

**Test cases:**
- `TestExtractVersion_returnsGitTag` — verify output matches `git describe` (or "dev" if no tags)
- `TestExtractInterface_containsFileTypeHandler` — verify output contains the interface name and all 7 method signatures
- `TestExtractInterface_containsMetadataCapability` — verify output contains the 3 capability constants
- `TestExtractFlags_sortCommand` — verify `cmd/sort.go` produces a table with `--source`, `--dest`, `--workers`, `--algorithm`, `--recursive`, etc.
- `TestExtractFlags_includesShortForms` — verify `-s`, `-d`, `-w`, `-r` appear in output
- `TestExtractFormats_allHandlers` — verify all 9 handler packages produce table rows (JPEG, HEIC, MP4, DNG, NEF, CR2, CR3, PEF, ARW)
- `TestExtractFormats_excludesTiffraw` — verify `tiffraw` (shared base) is not in the output
- `TestExtractFormats_deterministic` — run twice, verify identical output
- `TestExtractPackageReference_allGroups` — verify all 5 groups appear in output
- `TestExtractPackageReference_containsDocComments` — verify at least one package's doc comment text appears
- `TestExtractQuerySubcommands_allSubcommands` — verify `runs`, `run`, `duplicates`, `errors`, `skipped`, `files`, `inventory` all appear

---

### Task 12: Add markers to `docs/_config.yml`

Wrap the `version:` line in `docs/_config.yml` with YAML-comment-style markers:

**Before:**
```yaml
version: "v1.8.0"
```

**After:**
```yaml
# <!-- pixe:begin:version -->
version: "v2.0.0"
# <!-- pixe:end:version -->
```

The version value will be populated by `make docs` from the latest git tag.

---

### Task 13: Add markers to `docs/adding-formats.md`

Locate the section that reproduces the `FileTypeHandler` interface definition and wrap it with markers. The hand-authored narrative prose before and after the code block is preserved.

---

### Task 14: Add markers to `docs/commands.md`

For each command section in `commands.md`, wrap the flag table with the appropriate marker:
- `<!-- pixe:begin:sort-flags -->` / `<!-- pixe:end:sort-flags -->`
- `<!-- pixe:begin:verify-flags -->` / `<!-- pixe:end:verify-flags -->`
- `<!-- pixe:begin:resume-flags -->` / `<!-- pixe:end:resume-flags -->`
- `<!-- pixe:begin:status-flags -->` / `<!-- pixe:end:status-flags -->`
- `<!-- pixe:begin:clean-flags -->` / `<!-- pixe:end:clean-flags -->`
- `<!-- pixe:begin:gui-flags -->` / `<!-- pixe:end:gui-flags -->`
- `<!-- pixe:begin:query-flags -->` / `<!-- pixe:end:query-flags -->`
- `<!-- pixe:begin:query-subs -->` / `<!-- pixe:end:query-subs -->`

Use `format=html` markers since `commands.md` uses HTML tables for the accordion styling.

---

### Task 15: Add markers to `docs/how-it-works.md`

Wrap the supported file types section with `<!-- pixe:begin:format-table -->` / `<!-- pixe:end:format-table -->`. Use `format=html` since this is a docs page.

---

### Task 16: Add markers to `README.md`

Wrap the following sections with markers (all using `format=markdown` for GFM tables):
- Each command's flag table: `sort-flags`, `verify-flags`, `resume-flags`, `status-flags`, `clean-flags`, `gui-flags`, `query-flags`
- Query subcommands table: `query-subs`
- Supported File Types table: `format-table`

Preserve all hand-authored narrative prose outside the markers.

---

### Task 17: Create `docs/packages.md`

Create the new developer-facing Package Reference page with:
- Jekyll front matter: `layout: page`, `title: Package Reference`, `section_label: Developer Guide`, `permalink: /packages/`
- Hand-authored introduction paragraph (2-3 sentences explaining the page is generated from godoc comments)
- Single `<!-- pixe:begin:package-list -->` / `<!-- pixe:end:package-list -->` marker block

---

### Task 18: Update `docs/_data/navigation.yml`

Reorder navigation to reflect audience delineation per Architecture Section 15.12:
- End-user: Install, Commands, How It Works, Technical
- Developer: Adding Formats, Packages, Contributing
- Project: Changelog

---

### Task 19: Add Makefile targets

Add to `Makefile`:

```makefile
# ---------- documentation -----------------------------------
docs: ## Regenerate documentation from source code
	go run ./internal/docgen

docs-check: ## Check that generated docs are up to date (CI gate)
	@go run ./internal/docgen --check
	@echo "Documentation is up to date."
```

Update the `check` target:
```makefile
check: fmt-check vet test-unit docs-check ## Run fmt-check + vet + unit tests + docs-check (fast CI gate)
```

Update the `.PHONY` declaration to include `docs` and `docs-check`.

---

### Task 20: Update CI workflow

**`.github/workflows/ci.yml`:** Add a step after tests:
```yaml
- name: Check generated docs are up to date
  run: go run ./internal/docgen --check
```

**`.github/workflows/pages.yml`:** Remove this file — it is stale cruft superseded by GitHub's automatic Pages deployment configured via repository settings.

---

### Task 21: Run `make docs` and verify end-to-end

End-to-end verification:
1. Run `make docs` — all markers should be populated with extracted content.
2. Run `make docs-check` — should exit 0 (no stale files).
3. Run `make docs` again — no files should be written (idempotency).
4. Manually modify a generated section — `make docs-check` should exit 1.
5. Run `make docs` — the modification should be overwritten.
6. Verify `make check` passes (includes `docs-check`).
7. Verify the generated content is correct: flag tables match Cobra definitions, format table matches handler packages, interface matches `handler.go`, version matches git tag, package reference includes all packages.
