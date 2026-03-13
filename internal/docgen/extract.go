// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Task 3: Version extractor
// ---------------------------------------------------------------------------

// extractVersion returns the version string from `git describe --tags --abbrev=0`.
// Falls back to "dev" if no tags exist or the command fails.
func extractVersion() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	out, err := cmd.Output()
	if err != nil {
		return "dev", nil
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "dev", nil
	}
	return v, nil
}

// extractVersionReplacement returns a Replacement for the version section.
func extractVersionReplacement() func() (string, error) {
	return func() (string, error) {
		v, err := extractVersion()
		if err != nil {
			return "", err
		}
		return FormatYAMLValue("version", v), nil
	}
}

// ---------------------------------------------------------------------------
// Task 4: Interface extractor
// ---------------------------------------------------------------------------

// extractInterface parses internal/domain/handler.go and extracts the
// MetadataCapability type, its const block, and the FileTypeHandler interface.
// Returns the combined source as a fenced Go code block.
func extractInterface() (string, error) {
	path := filepath.Join("internal", "domain", "handler.go")
	src, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("extract interface: read %q: %w", path, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("extract interface: parse %q: %w", path, err)
	}

	var parts []string

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		switch genDecl.Tok {
		case token.TYPE:
			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch ts.Name.Name {
				case "MetadataCapability", "FileTypeHandler":
					// Extract the full declaration including doc comment.
					start := fset.Position(genDecl.Pos())
					end := fset.Position(genDecl.End())
					// Include doc comment if present.
					if genDecl.Doc != nil {
						start = fset.Position(genDecl.Doc.Pos())
					}
					parts = append(parts, strings.TrimRight(string(src[start.Offset:end.Offset]), "\n"))
				}
			}

		case token.CONST:
			// Check if this const block contains MetadataNone.
			for _, spec := range genDecl.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if name.Name == "MetadataNone" {
						start := fset.Position(genDecl.Pos())
						end := fset.Position(genDecl.End())
						if genDecl.Doc != nil {
							start = fset.Position(genDecl.Doc.Pos())
						}
						parts = append(parts, strings.TrimRight(string(src[start.Offset:end.Offset]), "\n"))
						goto nextDecl
					}
				}
			}
		}
	nextDecl:
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("extract interface: no matching declarations found in %q", path)
	}

	combined := strings.Join(parts, "\n\n")
	return FormatGoCodeBlock(combined), nil
}

// ---------------------------------------------------------------------------
// Task 5: CLI flag extractor
// ---------------------------------------------------------------------------

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
// If mergeRoot is true, also merges persistent flags from cmd/root.go.
func extractFlags(cmdFile string, format string, mergeRoot bool) func() (string, error) {
	return func() (string, error) {
		flags, err := parseFlagsFromFile(cmdFile)
		if err != nil {
			return "", err
		}

		if mergeRoot {
			rootFlags, err := parseFlagsFromFile(filepath.Join("cmd", "root.go"))
			if err != nil {
				return "", err
			}
			// Prepend root persistent flags (workers, algorithm).
			flags = append(rootFlags, flags...)
		}

		if len(flags) == 0 {
			return "", nil
		}

		headers := []string{"Flag", "Default", "Description"}
		rows := make([][]string, 0, len(flags))
		for _, f := range flags {
			flagCol := "--" + f.Long
			if f.Short != "" {
				flagCol = "-" + f.Short + ", --" + f.Long
			}
			rows = append(rows, []string{flagCol, f.Default, f.Description})
		}

		switch format {
		case "html":
			return FormatHTMLTable(headers, rows), nil
		default:
			return FormatMarkdownTable(headers, rows), nil
		}
	}
}

// parseFlagsFromFile parses a single Go source file and extracts all Cobra
// flag registrations from init() functions.
func parseFlagsFromFile(path string) ([]Flag, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("extract flags: read %q: %w", path, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("extract flags: parse %q: %w", path, err)
	}

	var flags []Flag

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name
		flag, ok := parseCobraFlagCall(methodName, call.Args)
		if !ok {
			return true
		}

		flags = append(flags, flag)
		return true
	})

	return flags, nil
}

// parseCobraFlagCall extracts a Flag from a Cobra flag registration call.
// Returns (flag, true) if the method name is a recognized Cobra flag method.
func parseCobraFlagCall(method string, args []ast.Expr) (Flag, bool) {
	// Methods with short form: StringP, BoolP, IntP, StringArrayP, etc.
	// args: name, shorthand, value, usage
	// Methods without short form: String, Bool, Int, StringArray, StringVar, BoolVar, etc.
	// args: name, value, usage

	// VarP variants: StringVarP, BoolVarP, IntVarP
	// args: ptr, name, shorthand, value, usage
	// Var variants: StringVar, BoolVar, IntVar
	// args: ptr, name, value, usage

	switch method {
	case "StringP", "BoolP", "IntP", "StringArrayP", "Float64P", "Int64P":
		if len(args) < 4 {
			return Flag{}, false
		}
		return Flag{
			Long:        stringLit(args[0]),
			Short:       stringLit(args[1]),
			Default:     exprToString(args[2]),
			Description: stringLit(args[3]),
		}, true

	case "String", "Bool", "Int", "StringArray", "Float64", "Int64":
		if len(args) < 3 {
			return Flag{}, false
		}
		return Flag{
			Long:        stringLit(args[0]),
			Default:     exprToString(args[1]),
			Description: stringLit(args[2]),
		}, true

	case "StringVarP", "BoolVarP", "IntVarP", "Float64VarP":
		if len(args) < 5 {
			return Flag{}, false
		}
		return Flag{
			Long:        stringLit(args[1]),
			Short:       stringLit(args[2]),
			Default:     exprToString(args[3]),
			Description: stringLit(args[4]),
		}, true

	case "StringVar", "BoolVar", "IntVar", "Float64Var":
		if len(args) < 4 {
			return Flag{}, false
		}
		return Flag{
			Long:        stringLit(args[1]),
			Default:     exprToString(args[2]),
			Description: stringLit(args[3]),
		}, true
	}

	return Flag{}, false
}

// stringLit extracts the string value from a *ast.BasicLit of kind STRING.
// Returns "" if the expression is not a string literal.
func stringLit(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	// Strip surrounding quotes.
	s := lit.Value
	if len(s) >= 2 && s[0] == '"' {
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '`' {
		return s[1 : len(s)-1]
	}
	return s
}

// exprToString converts an AST expression to a display string for the Default column.
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.STRING:
			return stringLit(expr)
		case token.INT, token.FLOAT:
			return e.Value
		}
	case *ast.Ident:
		switch e.Name {
		case "true":
			return "true"
		case "false":
			return "false"
		case "nil":
			return ""
		}
		return e.Name
	case *ast.UnaryExpr:
		if e.Op == token.SUB {
			return "-" + exprToString(e.X)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Task 6: Format table extractor
// ---------------------------------------------------------------------------

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
func extractFormats(format string) func() (string, error) {
	return func() (string, error) {
		handlerDir := filepath.Join("internal", "handler")
		entries, err := os.ReadDir(handlerDir)
		if err != nil {
			return "", fmt.Errorf("extract formats: read dir %q: %w", handlerDir, err)
		}

		var handlers []HandlerInfo
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "tiffraw" || name == "handlertest" {
				continue // shared base and test infra, not user-facing
			}

			info, err := parseHandlerPackage(filepath.Join(handlerDir, name), name)
			if err != nil {
				return "", fmt.Errorf("extract formats: parse handler %q: %w", name, err)
			}
			handlers = append(handlers, info)
		}

		// Sort alphabetically by package name for deterministic output.
		sort.Slice(handlers, func(i, j int) bool {
			return handlers[i].PackageName < handlers[j].PackageName
		})

		headers := []string{"Format", "Extensions", "Metadata"}
		rows := make([][]string, 0, len(handlers))
		for _, h := range handlers {
			rows = append(rows, []string{
				h.DisplayName,
				strings.Join(h.Extensions, ", "),
				h.Metadata,
			})
		}

		switch format {
		case "html":
			return FormatHTMLTable(headers, rows), nil
		default:
			return FormatMarkdownTable(headers, rows), nil
		}
	}
}

// parseHandlerPackage parses a handler package directory and extracts metadata.
func parseHandlerPackage(dir, pkgName string) (HandlerInfo, error) {
	primaryFile := filepath.Join(dir, pkgName+".go")
	src, err := os.ReadFile(primaryFile)
	if err != nil {
		return HandlerInfo{}, fmt.Errorf("read %q: %w", primaryFile, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, primaryFile, src, parser.ParseComments)
	if err != nil {
		return HandlerInfo{}, fmt.Errorf("parse %q: %w", primaryFile, err)
	}

	info := HandlerInfo{
		PackageName: pkgName,
		DisplayName: handlerDisplayName(pkgName),
	}

	// Extract package doc comment (first sentence).
	if f.Doc != nil {
		info.DocComment = firstSentence(f.Doc.Text())
	}

	// Walk AST to find Extensions() and MetadataSupport() methods.
	ast.Inspect(f, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		switch funcDecl.Name.Name {
		case "Extensions":
			info.Extensions = extractStringSliceReturn(funcDecl)
		case "MetadataSupport":
			info.Metadata = extractMetadataSupport(funcDecl)
		}
		return true
	})

	// If no MetadataSupport method was found in this file, check if the
	// handler embeds tiffraw.Base (detected by importing the tiffraw package).
	// tiffraw.Base.MetadataSupport() always returns domain.MetadataSidecar,
	// so any handler that imports tiffraw inherits "XMP sidecar" capability.
	if info.Metadata == "" {
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasSuffix(path, "/handler/tiffraw") {
				info.Metadata = "XMP sidecar"
				break
			}
		}
	}

	return info, nil
}

// handlerDisplayName converts a package name to a display name.
func handlerDisplayName(pkgName string) string {
	switch pkgName {
	case "mp4":
		return "MP4/MOV"
	case "jpeg":
		return "JPEG"
	case "heic":
		return "HEIC"
	case "dng":
		return "DNG"
	case "nef":
		return "NEF"
	case "cr2":
		return "CR2"
	case "cr3":
		return "CR3"
	case "pef":
		return "PEF"
	case "arw":
		return "ARW"
	}
	return strings.ToUpper(pkgName)
}

// extractStringSliceReturn extracts the string slice literal from a function
// that returns []string{...}.
func extractStringSliceReturn(funcDecl *ast.FuncDecl) []string {
	if funcDecl.Body == nil {
		return nil
	}
	var result []string
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		comp, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range comp.Elts {
			if s := stringLit(elt); s != "" {
				result = append(result, s)
			}
		}
		return false // stop after first composite literal
	})
	return result
}

// extractMetadataSupport extracts the MetadataCapability constant name from
// a MetadataSupport() method body.
func extractMetadataSupport(funcDecl *ast.FuncDecl) string {
	if funcDecl.Body == nil {
		return "None"
	}
	var result string
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		if len(ret.Results) == 0 {
			return true
		}
		// The return value is a selector expression like domain.MetadataEmbed.
		sel, ok := ret.Results[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "MetadataEmbed":
			result = "Embedded EXIF"
		case "MetadataSidecar":
			result = "XMP sidecar"
		case "MetadataNone":
			result = "None"
		default:
			result = sel.Sel.Name
		}
		return false
	})
	if result == "" {
		return "None"
	}
	return result
}

// firstSentence returns the first sentence of a doc comment text.
func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	// Find the first period followed by whitespace or end of string.
	for i, ch := range text {
		if ch == '.' {
			if i+1 >= len(text) || text[i+1] == ' ' || text[i+1] == '\n' {
				return text[:i+1]
			}
		}
	}
	// No period found — return first line.
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// ---------------------------------------------------------------------------
// Task 7: Package reference extractor
// ---------------------------------------------------------------------------

// PackageInfo represents one package's extracted documentation.
type PackageInfo struct {
	ImportPath string // e.g., "internal/pipeline"
	Name       string // e.g., "pipeline"
	DocComment string // full package doc comment text
}

// packageCategories defines the grouping for the package reference page.
var packageCategories = []struct {
	Title    string
	Packages []string
}{
	{
		Title: "Core Engine",
		Packages: []string{
			"internal/pipeline",
			"internal/discovery",
			"internal/copy",
			"internal/verify",
			"internal/hash",
			"internal/pathbuilder",
		},
	},
	{
		Title: "Data & Persistence",
		Packages: []string{
			"internal/archivedb",
			"internal/manifest",
			"internal/migrate",
			"internal/dblocator",
			"internal/domain",
			"internal/config",
		},
	},
	{
		Title: "File Type Handlers",
		Packages: []string{
			"internal/handler/jpeg",
			"internal/handler/heic",
			"internal/handler/mp4",
			"internal/handler/tiffraw",
			"internal/handler/dng",
			"internal/handler/nef",
			"internal/handler/cr2",
			"internal/handler/cr3",
			"internal/handler/pef",
			"internal/handler/arw",
		},
	},
	{
		Title: "Metadata",
		Packages: []string{
			"internal/tagging",
			"internal/xmp",
			"internal/ignore",
		},
	},
	{
		Title: "User Interface",
		Packages: []string{
			"internal/progress",
			"internal/cli",
		},
	},
}

// extractPackageReference scans all packages under internal/ and cmd/,
// extracts their package doc comments, groups them by category, and
// returns formatted Markdown.
func extractPackageReference() (string, error) {
	// Build a map of import path → PackageInfo.
	pkgMap := make(map[string]PackageInfo)

	// Walk internal/.
	if err := walkPackages("internal", pkgMap); err != nil {
		return "", fmt.Errorf("extract package reference: %w", err)
	}
	// Walk cmd/.
	if err := walkPackages("cmd", pkgMap); err != nil {
		return "", fmt.Errorf("extract package reference: %w", err)
	}

	var sb strings.Builder
	categorized := make(map[string]bool)

	for _, cat := range packageCategories {
		sb.WriteString("### " + cat.Title + "\n\n")
		for _, importPath := range cat.Packages {
			info, ok := pkgMap[importPath]
			if !ok {
				continue
			}
			categorized[importPath] = true
			doc := strings.TrimSpace(info.DocComment)
			// Use first paragraph only.
			if idx := strings.Index(doc, "\n\n"); idx >= 0 {
				doc = doc[:idx]
			}
			doc = strings.ReplaceAll(doc, "\n", " ")
			fmt.Fprintf(&sb, "**`%s`** — %s\n\n", importPath, doc)
		}
	}

	// Collect uncategorized packages under "Other".
	var other []PackageInfo
	for path, info := range pkgMap {
		if !categorized[path] {
			other = append(other, info)
		}
	}
	if len(other) > 0 {
		sort.Slice(other, func(i, j int) bool {
			return other[i].ImportPath < other[j].ImportPath
		})
		sb.WriteString("### Other\n\n")
		for _, info := range other {
			doc := strings.TrimSpace(info.DocComment)
			if idx := strings.Index(doc, "\n\n"); idx >= 0 {
				doc = doc[:idx]
			}
			doc = strings.ReplaceAll(doc, "\n", " ")
			fmt.Fprintf(&sb, "**`%s`** — %s\n\n", info.ImportPath, doc)
		}
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

// walkPackages walks a directory tree and extracts package doc comments.
func walkPackages(root string, pkgMap map[string]PackageInfo) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Skip hidden directories and test-only directories.
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") || base == "integration" {
			return filepath.SkipDir
		}

		// Check if directory contains .go files.
		goFiles, err := filepath.Glob(filepath.Join(path, "*.go"))
		if err != nil || len(goFiles) == 0 {
			return nil
		}

		// Parse the first .go file to get the package doc comment.
		info, err := extractPackageDoc(path, goFiles)
		if err != nil {
			return nil // skip packages that fail to parse
		}
		if info.DocComment != "" {
			pkgMap[path] = info
		}
		return nil
	})
}

// extractPackageDoc extracts the package doc comment from a directory.
func extractPackageDoc(dir string, goFiles []string) (PackageInfo, error) {
	// Try each file until we find one with a package doc comment.
	for _, file := range goFiles {
		// Skip test files.
		if strings.HasSuffix(file, "_test.go") {
			continue
		}

		src, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
		if err != nil {
			continue
		}

		if f.Doc != nil && f.Doc.Text() != "" {
			return PackageInfo{
				ImportPath: filepath.ToSlash(dir),
				Name:       f.Name.Name,
				DocComment: f.Doc.Text(),
			}, nil
		}
	}
	return PackageInfo{}, fmt.Errorf("no package doc comment found in %q", dir)
}

// ---------------------------------------------------------------------------
// Task 8: Query subcommand extractor
// ---------------------------------------------------------------------------

// QuerySubcommand represents one query subcommand extracted from source.
type QuerySubcommand struct {
	Name        string
	Description string
}

// extractQuerySubcommands returns a function that parses all cmd/query_*.go
// files and extracts the cobra.Command struct literal for each subcommand.
func extractQuerySubcommands(format string) func() (string, error) {
	return func() (string, error) {
		// Files to parse (excluding query.go parent and query_format.go helper).
		files := []string{
			filepath.Join("cmd", "query_runs.go"),
			filepath.Join("cmd", "query_run.go"),
			filepath.Join("cmd", "query_duplicates.go"),
			filepath.Join("cmd", "query_errors.go"),
			filepath.Join("cmd", "query_skipped.go"),
			filepath.Join("cmd", "query_files.go"),
			filepath.Join("cmd", "query_inventory.go"),
		}

		var subs []QuerySubcommand
		for _, file := range files {
			sub, err := parseQuerySubcommand(file)
			if err != nil {
				return "", fmt.Errorf("extract query subcommands: %w", err)
			}
			if sub.Name != "" {
				subs = append(subs, sub)
			}
		}

		headers := []string{"Subcommand", "Description"}
		rows := make([][]string, 0, len(subs))
		for _, s := range subs {
			rows = append(rows, []string{s.Name, s.Description})
		}

		switch format {
		case "html":
			return FormatHTMLTable(headers, rows), nil
		default:
			return FormatMarkdownTable(headers, rows), nil
		}
	}
}

// parseQuerySubcommand parses a single query_*.go file and extracts the
// cobra.Command Use and Short fields.
func parseQuerySubcommand(path string) (QuerySubcommand, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return QuerySubcommand{}, fmt.Errorf("read %q: %w", path, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return QuerySubcommand{}, fmt.Errorf("parse %q: %w", path, err)
	}

	var sub QuerySubcommand

	ast.Inspect(f, func(n ast.Node) bool {
		comp, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Look for cobra.Command composite literals.
		sel, ok := comp.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Command" {
			return true
		}

		for _, elt := range comp.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "Use":
				use := stringLit(kv.Value)
				// Extract just the subcommand name (first word).
				sub.Name = strings.Fields(use)[0]
			case "Short":
				sub.Description = stringLit(kv.Value)
			}
		}
		return false
	})

	return sub, nil
}
