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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeFile writes content to a file in dir, returning the full path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return path
}

// repoRoot returns the path to the repository root by walking up from the
// test binary's working directory until go.mod is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("repoRoot: getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repoRoot: go.mod not found")
		}
		dir = parent
	}
}

// ---------------------------------------------------------------------------
// Task 10: Injection engine tests
// ---------------------------------------------------------------------------

func TestInjectFile_basicReplacement(t *testing.T) {
	dir := t.TempDir()
	content := "before\n<!-- pixe:begin:foo -->\nold content\n<!-- pixe:end:foo -->\nafter\n"
	path := writeFile(t, dir, "test.md", content)

	result, err := InjectFile(path, []Replacement{{Name: "foo", Content: "new content"}})
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}

	want := "before\n<!-- pixe:begin:foo -->\n\nnew content\n\n<!-- pixe:end:foo -->\nafter\n"
	if result != want {
		t.Errorf("got:\n%q\nwant:\n%q", result, want)
	}
}

func TestInjectFile_multipleMarkers(t *testing.T) {
	dir := t.TempDir()
	content := "<!-- pixe:begin:a -->\nA\n<!-- pixe:end:a -->\nmiddle\n<!-- pixe:begin:b -->\nB\n<!-- pixe:end:b -->\n"
	path := writeFile(t, dir, "test.md", content)

	result, err := InjectFile(path, []Replacement{
		{Name: "a", Content: "new-a"},
		{Name: "b", Content: "new-b"},
	})
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}

	if !strings.Contains(result, "new-a") {
		t.Errorf("result missing new-a: %q", result)
	}
	if !strings.Contains(result, "new-b") {
		t.Errorf("result missing new-b: %q", result)
	}
	if !strings.Contains(result, "middle") {
		t.Errorf("result missing middle: %q", result)
	}
}

func TestInjectFile_preservesSurroundingContent(t *testing.T) {
	dir := t.TempDir()
	content := "# Title\n\nSome prose.\n\n<!-- pixe:begin:section -->\nold\n<!-- pixe:end:section -->\n\nMore prose.\n"
	path := writeFile(t, dir, "test.md", content)

	result, err := InjectFile(path, []Replacement{{Name: "section", Content: "new"}})
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}

	if !strings.Contains(result, "# Title") {
		t.Errorf("title missing from result")
	}
	if !strings.Contains(result, "Some prose.") {
		t.Errorf("prose before missing from result")
	}
	if !strings.Contains(result, "More prose.") {
		t.Errorf("prose after missing from result")
	}
	if strings.Contains(result, "old") {
		t.Errorf("old content should be replaced")
	}
}

func TestInjectFile_idempotent(t *testing.T) {
	dir := t.TempDir()
	content := "<!-- pixe:begin:x -->\noriginal\n<!-- pixe:end:x -->\n"
	path := writeFile(t, dir, "test.md", content)

	rep := []Replacement{{Name: "x", Content: "injected"}}

	first, err := InjectFile(path, rep)
	if err != nil {
		t.Fatalf("first InjectFile: %v", err)
	}

	// Write the result back and inject again.
	if err := os.WriteFile(path, []byte(first), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	second, err := InjectFile(path, rep)
	if err != nil {
		t.Fatalf("second InjectFile: %v", err)
	}

	if first != second {
		t.Errorf("not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestInjectFile_yamlCommentMarkers(t *testing.T) {
	dir := t.TempDir()
	content := "key: old\n# <!-- pixe:begin:version -->\nversion: \"v1.0.0\"\n# <!-- pixe:end:version -->\nother: value\n"
	path := writeFile(t, dir, "_config.yml", content)

	result, err := InjectFile(path, []Replacement{{Name: "version", Content: "version: \"v2.0.0\""}})
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}

	if !strings.Contains(result, `version: "v2.0.0"`) {
		t.Errorf("new version not found in result: %q", result)
	}
	if strings.Contains(result, `version: "v1.0.0"`) {
		t.Errorf("old version should be replaced: %q", result)
	}
	if !strings.Contains(result, "other: value") {
		t.Errorf("surrounding content missing: %q", result)
	}
}

func TestInjectFile_unpairedBegin(t *testing.T) {
	dir := t.TempDir()
	content := "<!-- pixe:begin:foo -->\nno end marker\n"
	path := writeFile(t, dir, "test.md", content)

	_, err := InjectFile(path, nil)
	if err == nil {
		t.Fatal("expected error for unpaired begin marker")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error should mention section name: %v", err)
	}
}

func TestInjectFile_unpairedEnd(t *testing.T) {
	dir := t.TempDir()
	content := "no begin marker\n<!-- pixe:end:foo -->\n"
	path := writeFile(t, dir, "test.md", content)

	_, err := InjectFile(path, nil)
	if err == nil {
		t.Fatal("expected error for unpaired end marker")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error should mention section name: %v", err)
	}
}

func TestInjectFile_nestedMarkers(t *testing.T) {
	dir := t.TempDir()
	content := "<!-- pixe:begin:outer -->\n<!-- pixe:begin:inner -->\ncontent\n<!-- pixe:end:inner -->\n<!-- pixe:end:outer -->\n"
	path := writeFile(t, dir, "test.md", content)

	_, err := InjectFile(path, nil)
	if err == nil {
		t.Fatal("expected error for nested markers")
	}
}

func TestInjectFile_unknownSection(t *testing.T) {
	dir := t.TempDir()
	content := "<!-- pixe:begin:known -->\nold\n<!-- pixe:end:known -->\n"
	path := writeFile(t, dir, "test.md", content)

	// Providing a replacement for "unknown" (not in file) should be a no-op.
	result, err := InjectFile(path, []Replacement{
		{Name: "known", Content: "new"},
		{Name: "unknown", Content: "ignored"},
	})
	if err != nil {
		t.Fatalf("InjectFile: %v", err)
	}
	if !strings.Contains(result, "new") {
		t.Errorf("known section not replaced: %q", result)
	}
}

func TestWriteIfChanged_writesWhenDifferent(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "file.txt", "original")

	changed, err := WriteIfChanged(path, "modified")
	if err != nil {
		t.Fatalf("WriteIfChanged: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}

	got, _ := os.ReadFile(path)
	if string(got) != "modified" {
		t.Errorf("file content: got %q, want %q", got, "modified")
	}
}

func TestWriteIfChanged_skipsWhenIdentical(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "file.txt", "same content")

	changed, err := WriteIfChanged(path, "same content")
	if err != nil {
		t.Fatalf("WriteIfChanged: %v", err)
	}
	if changed {
		t.Error("expected changed=false when content is identical")
	}
}

// ---------------------------------------------------------------------------
// Task 11: Extractor tests
// ---------------------------------------------------------------------------

func TestExtractVersion_returnsGitTag(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	v, err := extractVersion()
	if err != nil {
		t.Fatalf("extractVersion: %v", err)
	}
	// Should be either a tag (starts with v) or "dev".
	if v != "dev" && !strings.HasPrefix(v, "v") {
		t.Errorf("unexpected version %q: want vX.Y.Z or dev", v)
	}
}

func TestExtractInterface_containsFileTypeHandler(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	result, err := extractInterface()
	if err != nil {
		t.Fatalf("extractInterface: %v", err)
	}

	methods := []string{
		"Detect(",
		"ExtractDate(",
		"HashableReader(",
		"MetadataSupport(",
		"WriteMetadataTags(",
		"Extensions(",
		"MagicBytes(",
	}
	for _, m := range methods {
		if !strings.Contains(result, m) {
			t.Errorf("interface missing method %q", m)
		}
	}
	if !strings.Contains(result, "FileTypeHandler") {
		t.Errorf("result missing FileTypeHandler interface name")
	}
}

func TestExtractInterface_containsMetadataCapability(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	result, err := extractInterface()
	if err != nil {
		t.Fatalf("extractInterface: %v", err)
	}

	constants := []string{"MetadataNone", "MetadataEmbed", "MetadataSidecar"}
	for _, c := range constants {
		if !strings.Contains(result, c) {
			t.Errorf("result missing constant %q", c)
		}
	}
}

func TestExtractFlags_sortCommand(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractFlags(filepath.Join("cmd", "sort.go"), "markdown", true)
	result, err := fn()
	if err != nil {
		t.Fatalf("extractFlags: %v", err)
	}

	expectedFlags := []string{"--source", "--dest", "--recursive", "--dry-run", "--progress"}
	for _, f := range expectedFlags {
		if !strings.Contains(result, f) {
			t.Errorf("sort flags missing %q", f)
		}
	}
}

func TestExtractFlags_includesShortForms(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractFlags(filepath.Join("cmd", "sort.go"), "markdown", true)
	result, err := fn()
	if err != nil {
		t.Fatalf("extractFlags: %v", err)
	}

	shortForms := []string{"-s,", "-d,", "-r,"}
	for _, s := range shortForms {
		if !strings.Contains(result, s) {
			t.Errorf("sort flags missing short form %q", s)
		}
	}
}

func TestExtractFormats_allHandlers(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractFormats("markdown")
	result, err := fn()
	if err != nil {
		t.Fatalf("extractFormats: %v", err)
	}

	handlers := []string{"JPEG", "HEIC", "MP4/MOV", "DNG", "NEF", "CR2", "CR3", "PEF", "ARW"}
	for _, h := range handlers {
		if !strings.Contains(result, h) {
			t.Errorf("format table missing handler %q", h)
		}
	}
}

func TestExtractFormats_excludesTiffraw(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractFormats("markdown")
	result, err := fn()
	if err != nil {
		t.Fatalf("extractFormats: %v", err)
	}

	if strings.Contains(strings.ToLower(result), "tiffraw") {
		t.Errorf("format table should not contain tiffraw (shared base)")
	}
}

func TestExtractFormats_excludesHandlertest(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractFormats("markdown")
	result, err := fn()
	if err != nil {
		t.Fatalf("extractFormats: %v", err)
	}

	if strings.Contains(strings.ToLower(result), "handlertest") {
		t.Errorf("format table should not contain handlertest (test infrastructure)")
	}
}

func TestExtractFormats_tiffrawHandlersShowSidecar(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractFormats("markdown")
	result, err := fn()
	if err != nil {
		t.Fatalf("extractFormats: %v", err)
	}

	// All TIFF-based handlers inherit MetadataSidecar from tiffraw.Base.
	// The docgen extractor must detect this via the tiffraw import, not a
	// direct MetadataSupport() func decl (which doesn't exist in their files).
	tiffHandlers := []string{"ARW", "CR2", "DNG", "NEF", "PEF"}
	lines := strings.Split(result, "\n")
	for _, handler := range tiffHandlers {
		found := false
		for _, line := range lines {
			if strings.Contains(line, handler) {
				found = true
				if !strings.Contains(line, "XMP sidecar") {
					t.Errorf("handler %s row missing 'XMP sidecar': %q", handler, line)
				}
				break
			}
		}
		if !found {
			t.Errorf("handler %s not found in format table", handler)
		}
	}
}

func TestExtractFormats_deterministic(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractFormats("markdown")

	first, err := fn()
	if err != nil {
		t.Fatalf("first extractFormats: %v", err)
	}
	second, err := fn()
	if err != nil {
		t.Fatalf("second extractFormats: %v", err)
	}

	if first != second {
		t.Errorf("extractFormats is not deterministic:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestExtractPackageReference_allGroups(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	result, err := extractPackageReference()
	if err != nil {
		t.Fatalf("extractPackageReference: %v", err)
	}

	groups := []string{
		"### Core Engine",
		"### Data & Persistence",
		"### File Type Handlers",
		"### Metadata",
		"### User Interface",
	}
	for _, g := range groups {
		if !strings.Contains(result, g) {
			t.Errorf("package reference missing group %q", g)
		}
	}
}

func TestExtractPackageReference_containsDocComments(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	result, err := extractPackageReference()
	if err != nil {
		t.Fatalf("extractPackageReference: %v", err)
	}

	// Verify at least one known package doc comment appears.
	if !strings.Contains(result, "internal/pipeline") {
		t.Errorf("package reference missing internal/pipeline")
	}
	if !strings.Contains(result, "internal/archivedb") {
		t.Errorf("package reference missing internal/archivedb")
	}
}

func TestExtractQuerySubcommands_allSubcommands(t *testing.T) {
	root := repoRoot(t)
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(orig) }()

	fn := extractQuerySubcommands("markdown")
	result, err := fn()
	if err != nil {
		t.Fatalf("extractQuerySubcommands: %v", err)
	}

	subs := []string{"runs", "run", "duplicates", "errors", "skipped", "files", "inventory"}
	for _, s := range subs {
		if !strings.Contains(result, s) {
			t.Errorf("query subcommands missing %q", s)
		}
	}
}

// ---------------------------------------------------------------------------
// Format function tests
// ---------------------------------------------------------------------------

func TestFormatMarkdownTable_basic(t *testing.T) {
	result := FormatMarkdownTable(
		[]string{"Flag", "Description"},
		[][]string{
			{"--foo", "does foo"},
			{"--bar", "does bar"},
		},
	)

	if !strings.Contains(result, "| Flag") {
		t.Errorf("missing header: %q", result)
	}
	if !strings.Contains(result, "| --foo") {
		t.Errorf("missing row: %q", result)
	}
	if !strings.Contains(result, "---") {
		t.Errorf("missing separator: %q", result)
	}
}

func TestFormatHTMLTable_basic(t *testing.T) {
	result := FormatHTMLTable(
		[]string{"Flag", "Description"},
		[][]string{
			{"--foo", "does foo"},
		},
	)

	if !strings.Contains(result, `<table class="flag-table">`) {
		t.Errorf("missing table tag: %q", result)
	}
	if !strings.Contains(result, "<th>Flag</th>") {
		t.Errorf("missing header: %q", result)
	}
	if !strings.Contains(result, "<td>--foo</td>") {
		t.Errorf("missing cell: %q", result)
	}
}

func TestFormatGoCodeBlock_basic(t *testing.T) {
	result := FormatGoCodeBlock("type Foo interface{}")
	if !strings.HasPrefix(result, "```go\n") {
		t.Errorf("missing go fence: %q", result)
	}
	if !strings.HasSuffix(result, "\n```") {
		t.Errorf("missing closing fence: %q", result)
	}
}

func TestFormatYAMLValue_basic(t *testing.T) {
	result := FormatYAMLValue("version", "v2.0.0")
	want := `version: "v2.0.0"`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}
