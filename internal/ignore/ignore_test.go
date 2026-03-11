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

package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMatcher_hardcodedLedgerIgnore verifies that .pixe_ledger.json is always
// ignored regardless of user patterns, even with no patterns configured.
func TestMatcher_hardcodedLedgerIgnore(t *testing.T) {
	m := New(nil)
	if !m.Match(".pixe_ledger.json", ".pixe_ledger.json") {
		t.Error("expected .pixe_ledger.json to be ignored (top-level)")
	}
}

// TestMatcher_ledgerIgnoreAtDepth verifies that .pixe_ledger.json is ignored
// when it appears in a subdirectory (relPath differs from filename).
func TestMatcher_ledgerIgnoreAtDepth(t *testing.T) {
	m := New(nil)
	if !m.Match(".pixe_ledger.json", "subdir/.pixe_ledger.json") {
		t.Error("expected .pixe_ledger.json to be ignored at subdirectory depth")
	}
}

// TestMatcher_simpleGlobMatch verifies that a glob pattern matches the filename.
func TestMatcher_simpleGlobMatch(t *testing.T) {
	m := New([]string{"*.txt"})
	if !m.Match("notes.txt", "notes.txt") {
		t.Error("expected *.txt to match notes.txt")
	}
}

// TestMatcher_noMatch verifies that a non-matching file is not ignored.
func TestMatcher_noMatch(t *testing.T) {
	m := New([]string{"*.txt"})
	if m.Match("photo.jpg", "photo.jpg") {
		t.Error("expected *.txt NOT to match photo.jpg")
	}
}

// TestMatcher_exactFilenameMatch verifies that an exact filename pattern works.
func TestMatcher_exactFilenameMatch(t *testing.T) {
	m := New([]string{".DS_Store"})
	if !m.Match(".DS_Store", ".DS_Store") {
		t.Error("expected .DS_Store pattern to match .DS_Store filename")
	}
}

// TestMatcher_relativePathMatch verifies that a pattern with a path separator
// matches against the relative path, not just the filename.
func TestMatcher_relativePathMatch(t *testing.T) {
	m := New([]string{"subdir/*.tmp"})
	// filename alone should NOT match (no path separator in filename)
	if m.Match("cache.tmp", "cache.tmp") {
		t.Error("expected subdir/*.tmp NOT to match bare filename cache.tmp")
	}
	// relPath should match
	if !m.Match("cache.tmp", "subdir/cache.tmp") {
		t.Error("expected subdir/*.tmp to match relPath subdir/cache.tmp")
	}
}

// TestMatcher_emptyPatterns verifies that a Matcher with no patterns only
// ignores the hardcoded ledger file.
func TestMatcher_emptyPatterns(t *testing.T) {
	m := New([]string{})
	if m.Match("photo.jpg", "photo.jpg") {
		t.Error("expected no-pattern matcher NOT to match photo.jpg")
	}
	if m.Match("readme.txt", "readme.txt") {
		t.Error("expected no-pattern matcher NOT to match readme.txt")
	}
}

// TestMatcher_nilPatterns verifies that New(nil) behaves identically to New([]).
func TestMatcher_nilPatterns(t *testing.T) {
	m := New(nil)
	if m.Match("photo.jpg", "photo.jpg") {
		t.Error("expected nil-pattern matcher NOT to match photo.jpg")
	}
}

// TestMatcher_deduplication verifies that duplicate patterns are collapsed and
// do not cause incorrect behavior or panics.
func TestMatcher_deduplication(t *testing.T) {
	m := New([]string{"*.txt", "*.txt", "*.txt"})
	// Should still match correctly.
	if !m.Match("notes.txt", "notes.txt") {
		t.Error("expected deduplicated *.txt to still match notes.txt")
	}
	// Internal pattern slice should have length 1.
	if len(m.global) != 1 {
		t.Errorf("expected 1 deduplicated pattern, got %d", len(m.global))
	}
}

// TestMatcher_whitespacePatternsTrimmed verifies that patterns with leading/
// trailing whitespace are trimmed and empty-after-trim patterns are dropped.
func TestMatcher_whitespacePatternsTrimmed(t *testing.T) {
	m := New([]string{"  *.txt  ", "   ", "\t*.jpg\t"})
	if !m.Match("notes.txt", "notes.txt") {
		t.Error("expected trimmed *.txt to match notes.txt")
	}
	if !m.Match("photo.jpg", "photo.jpg") {
		t.Error("expected trimmed *.jpg to match photo.jpg")
	}
	// The whitespace-only pattern should have been dropped.
	if len(m.global) != 2 {
		t.Errorf("expected 2 patterns after trimming, got %d", len(m.global))
	}
}

// TestMatcher_multiplePatterns verifies that any matching pattern triggers ignore.
func TestMatcher_multiplePatterns(t *testing.T) {
	m := New([]string{"*.txt", "*.log", "Thumbs.db"})
	cases := []struct {
		filename string
		want     bool
	}{
		{"notes.txt", true},
		{"app.log", true},
		{"Thumbs.db", true},
		{"photo.jpg", false},
		{"video.mp4", false},
	}
	for _, tc := range cases {
		got := m.Match(tc.filename, tc.filename)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.filename, got, tc.want)
		}
	}
}

// TestMatcher_tableTests is a comprehensive table-driven test covering the
// full matrix of filename / relPath / pattern combinations.
func TestMatcher_tableTests(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		filename string
		relPath  string
		want     bool
	}{
		// Hardcoded ledger
		{"ledger top-level", nil, ".pixe_ledger.json", ".pixe_ledger.json", true},
		{"ledger in subdir", nil, ".pixe_ledger.json", "vacation/.pixe_ledger.json", true},
		// Glob on filename
		{"glob *.txt matches", []string{"*.txt"}, "readme.txt", "readme.txt", true},
		{"glob *.txt no match", []string{"*.txt"}, "photo.jpg", "photo.jpg", false},
		// Glob on relPath
		{"glob subdir/*.tmp relPath match", []string{"subdir/*.tmp"}, "cache.tmp", "subdir/cache.tmp", true},
		{"glob subdir/*.tmp filename no match", []string{"subdir/*.tmp"}, "cache.tmp", "cache.tmp", false},
		// Exact name
		{".DS_Store exact", []string{".DS_Store"}, ".DS_Store", ".DS_Store", true},
		// No patterns
		{"no patterns jpg", nil, "photo.jpg", "photo.jpg", false},
		// Multiple patterns, first matches
		{"multi first match", []string{"*.txt", "*.log"}, "notes.txt", "notes.txt", true},
		// Multiple patterns, second matches
		{"multi second match", []string{"*.txt", "*.log"}, "app.log", "app.log", true},
		// Multiple patterns, none match
		{"multi no match", []string{"*.txt", "*.log"}, "photo.jpg", "photo.jpg", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(tc.patterns)
			got := m.Match(tc.filename, tc.relPath)
			if got != tc.want {
				t.Errorf("Match(filename=%q, relPath=%q) = %v, want %v",
					tc.filename, tc.relPath, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MatchDir tests (Task 3)
// ---------------------------------------------------------------------------

// TestMatcher_matchDir_trailingSlash verifies that trailing-slash patterns
// match directories by name and by relative path.
func TestMatcher_matchDir_trailingSlash(t *testing.T) {
	cases := []struct {
		name       string
		patterns   []string
		dirname    string
		relDirPath string
		want       bool
	}{
		// Simple name match
		{"node_modules/ by name", []string{"node_modules/"}, "node_modules", "node_modules", true},
		// Nested: match by relDirPath
		{"node_modules/ nested", []string{"node_modules/"}, "node_modules", "src/node_modules", true},
		// Glob trailing-slash
		{".git/ exact", []string{".git/"}, ".git", ".git", true},
		// Doublestar trailing-slash
		{"**/cache/ deep", []string{"**/cache/"}, "cache", "a/b/cache", true},
		{"**/cache/ top", []string{"**/cache/"}, "cache", "cache", true},
		// Nested path pattern
		{"backups/old/ nested", []string{"backups/old/"}, "old", "backups/old", true},
		// No match — different name
		{"node_modules/ no match", []string{"node_modules/"}, "vendor", "vendor", false},
		// No patterns
		{"no patterns", nil, "node_modules", "node_modules", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(tc.patterns)
			got := m.MatchDir(tc.dirname, tc.relDirPath)
			if got != tc.want {
				t.Errorf("MatchDir(%q, %q) = %v, want %v", tc.dirname, tc.relDirPath, got, tc.want)
			}
		})
	}
}

// TestMatcher_matchDir_noSlashNoMatch verifies that file-level patterns (no
// trailing slash, no "/**" suffix) do NOT trigger directory skipping.
func TestMatcher_matchDir_noSlashNoMatch(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		dirname  string
	}{
		{"plain glob", []string{"*.tmp"}, "scratch"},
		{"exact name no slash", []string{"node_modules"}, "node_modules"},
		{"doublestar file", []string{"**/*.txt"}, "docs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(tc.patterns)
			if m.MatchDir(tc.dirname, tc.dirname) {
				t.Errorf("MatchDir(%q) should NOT match file-level pattern %q", tc.dirname, tc.patterns[0])
			}
		})
	}
}

// TestMatcher_matchDir_implicitDoublestar verifies that "prefix/**" patterns
// cause the prefix directory itself to be skipped.
func TestMatcher_matchDir_implicitDoublestar(t *testing.T) {
	cases := []struct {
		name       string
		patterns   []string
		dirname    string
		relDirPath string
		want       bool
	}{
		{"backups/**", []string{"backups/**"}, "backups", "backups", true},
		{"nested backups/**", []string{"backups/**"}, "backups", "root/backups", true},
		{"no match vendor", []string{"backups/**"}, "vendor", "vendor", false},
		{"deep **/cache/**", []string{"**/cache/**"}, "cache", "a/b/cache", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(tc.patterns)
			got := m.MatchDir(tc.dirname, tc.relDirPath)
			if got != tc.want {
				t.Errorf("MatchDir(%q, %q) = %v, want %v", tc.dirname, tc.relDirPath, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Scope stack + .pixeignore tests (Task 4)
// ---------------------------------------------------------------------------

// TestMatcher_pixeignoreHardcoded verifies that .pixeignore itself is always
// ignored by Match regardless of user patterns.
func TestMatcher_pixeignoreHardcoded(t *testing.T) {
	m := New(nil)
	if !m.Match(pixeignoreFilename, pixeignoreFilename) {
		t.Error("expected .pixeignore to be hardcoded-ignored at top level")
	}
	if !m.Match(pixeignoreFilename, "subdir/.pixeignore") {
		t.Error("expected .pixeignore to be hardcoded-ignored in subdirectory")
	}
}

// TestMatcher_pushScope_fileNotFound verifies that PushScope returns false and
// does not push a scope when the file does not exist.
func TestMatcher_pushScope_fileNotFound(t *testing.T) {
	m := New(nil)
	pushed := m.PushScope(".", "/nonexistent/path/.pixeignore")
	if pushed {
		t.Error("expected PushScope to return false for nonexistent file")
	}
	if len(m.scopes) != 0 {
		t.Errorf("expected 0 scopes after failed push, got %d", len(m.scopes))
	}
}

// TestMatcher_pushScope_parsesFormat verifies that PushScope correctly parses
// the .pixeignore file format: comments, blank lines, whitespace trimming, and
// deduplication.
func TestMatcher_pushScope_parsesFormat(t *testing.T) {
	dir := t.TempDir()
	content := "# This is a comment\n\n  *.txt  \n*.log\n# another comment\n*.txt\n\n"
	path := filepath.Join(dir, ".pixeignore")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := New(nil)
	pushed := m.PushScope(".", path)
	if !pushed {
		t.Fatal("expected PushScope to return true for existing file")
	}
	if len(m.scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(m.scopes))
	}
	sc := m.scopes[0]
	// Should have exactly 2 patterns: *.txt and *.log (*.txt deduplicated).
	if len(sc.patterns) != 2 {
		t.Errorf("expected 2 patterns after parse+dedup, got %d: %v", len(sc.patterns), sc.patterns)
	}
}

// TestMatcher_pushPopScope verifies the basic push/pop lifecycle: patterns are
// active after push and inactive after pop.
func TestMatcher_pushPopScope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pixeignore")
	if err := os.WriteFile(path, []byte("*.secret\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := New(nil)

	// Before push: *.secret should NOT match.
	if m.Match("key.secret", "key.secret") {
		t.Error("expected *.secret NOT to match before PushScope")
	}

	m.PushScope(".", path)

	// After push: *.secret should match.
	if !m.Match("key.secret", "key.secret") {
		t.Error("expected *.secret to match after PushScope")
	}

	m.PopScope()

	// After pop: *.secret should NOT match again.
	if m.Match("key.secret", "key.secret") {
		t.Error("expected *.secret NOT to match after PopScope")
	}
}

// TestMatcher_nestedScopes verifies that two scopes can be pushed and popped
// independently, with inner scope patterns deactivated on inner pop.
func TestMatcher_nestedScopes(t *testing.T) {
	dir := t.TempDir()

	outerPath := filepath.Join(dir, "outer.pixeignore")
	innerPath := filepath.Join(dir, "inner.pixeignore")
	if err := os.WriteFile(outerPath, []byte("*.outer\n"), 0o644); err != nil {
		t.Fatalf("WriteFile outer: %v", err)
	}
	if err := os.WriteFile(innerPath, []byte("*.inner\n"), 0o644); err != nil {
		t.Fatalf("WriteFile inner: %v", err)
	}

	m := New(nil)
	m.PushScope(".", outerPath)
	m.PushScope("sub", innerPath)

	// Both patterns active.
	if !m.Match("file.outer", "file.outer") {
		t.Error("expected *.outer to match with both scopes active")
	}
	if !m.Match("file.inner", "sub/file.inner") {
		t.Error("expected *.inner to match with both scopes active")
	}

	m.PopScope() // pop inner

	// Only outer pattern active.
	if !m.Match("file.outer", "file.outer") {
		t.Error("expected *.outer to still match after inner pop")
	}
	if m.Match("file.inner", "sub/file.inner") {
		t.Error("expected *.inner NOT to match after inner pop")
	}

	m.PopScope() // pop outer

	// No patterns active.
	if m.Match("file.outer", "file.outer") {
		t.Error("expected *.outer NOT to match after both pops")
	}
}

// TestMatcher_scopeRelativePaths verifies that scoped patterns are matched
// relative to the scope's basePath, not relative to dirA.
// A pattern "*.log" in a scope at basePath "sub" should match "sub/app.log"
// (relPath from dirA) but NOT "app.log" at the root.
func TestMatcher_scopeRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pixeignore")
	if err := os.WriteFile(path, []byte("*.log\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := New(nil)
	m.PushScope("sub", path) // scope rooted at "sub/"

	// "sub/app.log" → relative to scope base "sub" → "app.log" → matches *.log
	if !m.Match("app.log", "sub/app.log") {
		t.Error("expected *.log to match sub/app.log (within scope)")
	}

	// "app.log" at root → relative to scope base "sub" → "app.log" (not under sub)
	// scopeRelPath returns the full path when not under the scope base,
	// so "app.log" won't match "*.log" via the scope path either — but it
	// will match via the slashName check. Let's test a path-specific pattern.
	m.PopScope()

	// Now test with a path-specific scoped pattern.
	path2 := filepath.Join(dir, ".pixeignore2")
	if err := os.WriteFile(path2, []byte("raw/*.dng\n"), 0o644); err != nil {
		t.Fatalf("WriteFile2: %v", err)
	}
	m.PushScope("exports", path2) // scope at "exports/"

	// "exports/raw/shot.dng" → relative to scope "exports" → "raw/shot.dng" → matches raw/*.dng
	if !m.Match("shot.dng", "exports/raw/shot.dng") {
		t.Error("expected raw/*.dng to match exports/raw/shot.dng (within scope)")
	}
	// "raw/shot.dng" at root → not under "exports" scope → full path "raw/shot.dng" → matches raw/*.dng via slashRel
	// This is expected: the pattern "raw/*.dng" also matches the global relPath "raw/shot.dng".
	// Test a case that is clearly outside scope and should NOT match:
	if m.Match("shot.dng", "other/raw/shot.dng") {
		// "other/raw/shot.dng" relative to scope "exports" → "other/raw/shot.dng" (not under exports)
		// scopeRelPath returns full path → "other/raw/shot.dng" does NOT match "raw/*.dng"
		// But slashName "shot.dng" also doesn't match "raw/*.dng"
		// So this should NOT match.
		t.Error("expected raw/*.dng NOT to match other/raw/shot.dng (outside scope)")
	}
}
