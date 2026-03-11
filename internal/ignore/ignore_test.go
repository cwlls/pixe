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

import "testing"

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
