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

// Package ignore provides glob-based file ignore matching for Pixe discovery.
// It encapsulates both the hardcoded ledger-file exclusion and any
// user-configured patterns supplied via --ignore flags or .pixeignore files.
//
// Pattern matching uses github.com/bmatcuk/doublestar/v4, which is a superset
// of filepath.Match and adds support for ** recursive globs, {alt1,alt2}
// alternatives, and character classes. All existing single-level patterns
// (*.txt, .DS_Store, subdir/*.tmp) continue to work identically.
//
// Patterns ending with "/" are directory-only patterns and are ignored by
// Match; they are handled exclusively by MatchDir.
//
// .pixeignore files are loaded lazily during discovery.Walk via PushScope and
// PopScope. Patterns in a .pixeignore are scoped to the directory containing
// the file and all its descendants. Multiple scopes may be active at once
// (one per nested .pixeignore encountered during a recursive walk).
//
// Thread safety: Matcher is used by a single goroutine (the Walk caller).
// The scope stack is not accessed concurrently; no mutex is needed.
package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// hardcoded filenames that are always ignored at any directory depth.
const (
	ledgerFilename     = ".pixe_ledger.json"
	pixeignoreFilename = ".pixeignore"
)

// patternScope holds the patterns loaded from a single .pixeignore file and
// the base path of the directory that contains it (relative to dirA).
type patternScope struct {
	basePath string   // relative path from dirA to the .pixeignore's directory
	patterns []string // deduplicated patterns from this .pixeignore
}

// Matcher holds compiled ignore patterns and provides Match and MatchDir
// methods. The zero value is valid and matches only the hardcoded filenames.
type Matcher struct {
	global []string       // deduplicated, trimmed user-configured glob patterns
	scopes []patternScope // stack of .pixeignore scopes (pushed/popped during walk)
}

// New creates a Matcher from user-configured glob patterns.
// Patterns are deduplicated and whitespace-trimmed; empty strings are dropped.
// The hardcoded ignores (.pixe_ledger.json, .pixeignore) are always active and
// do not need to be included in patterns.
func New(patterns []string) *Matcher {
	return &Matcher{global: dedup(patterns)}
}

// PushScope reads a .pixeignore file at pixeignorePath and pushes its patterns
// onto the scope stack. basePath is the path of the containing directory
// relative to dirA (use "." for the root dirA itself).
//
// Returns true if the file existed and was successfully loaded. Returns false
// (without error) if the file does not exist — this is the common case for
// directories that have no .pixeignore.
//
// Parse rules: blank lines and lines starting with "#" are ignored; leading
// and trailing whitespace is trimmed; duplicate patterns are collapsed.
func (m *Matcher) PushScope(basePath, pixeignorePath string) bool {
	f, err := os.Open(pixeignorePath)
	if err != nil {
		return false // file not found or unreadable — not an error
	}
	defer func() { _ = f.Close() }()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	m.scopes = append(m.scopes, patternScope{
		basePath: filepath.ToSlash(basePath),
		patterns: dedup(patterns),
	})
	return true
}

// PopScope removes the most recently pushed scope from the stack.
// It is a no-op if the stack is empty.
func (m *Matcher) PopScope() {
	if len(m.scopes) > 0 {
		m.scopes = m.scopes[:len(m.scopes)-1]
	}
}

// MatchDir reports whether a directory should be skipped entirely.
//
// dirname is the base name of the directory (e.g. "node_modules").
// relDirPath is the path relative to dirA (e.g. "src/node_modules").
//
// Two classes of patterns trigger a directory skip:
//
//  1. Trailing-slash patterns (e.g. "node_modules/", "**/cache/"): the slash is
//     stripped and the remainder is matched against both dirname and relDirPath.
//
//  2. Implicit directory patterns ending with "/**" (e.g. "backups/**"): the
//     "/**" suffix is stripped and the prefix is matched against relDirPath,
//     allowing the entire subtree to be skipped rather than descending and
//     ignoring each file individually.
//
// All other patterns are file-level and are ignored by MatchDir.
// Global patterns and all active .pixeignore scopes are checked.
func (m *Matcher) MatchDir(dirname, relDirPath string) bool {
	slashName := filepath.ToSlash(dirname)
	slashRel := filepath.ToSlash(relDirPath)

	// Check global patterns.
	for _, pattern := range m.global {
		if matchesDir(pattern, slashName, slashRel) {
			return true
		}
	}

	// Check active .pixeignore scopes.
	for i := range m.scopes {
		sc := &m.scopes[i]
		// Compute the directory path relative to this scope's base.
		scopeRel := scopeRelPath(sc.basePath, slashRel)
		scopeName := slashName // dirname is always just the base name
		for _, pattern := range sc.patterns {
			if matchesDir(pattern, scopeName, scopeRel) {
				return true
			}
		}
	}
	return false
}

// Match reports whether the file should be ignored.
//
// filename is the base name of the file (e.g. "IMG_0001.jpg").
// relPath is the path relative to dirA (e.g. "vacation/IMG_0001.jpg").
// For top-level files relPath == filename.
//
// The hardcoded ignores are checked first (by filename equality).
// Then global patterns and all active .pixeignore scopes are checked using
// doublestar semantics, enabling patterns such as "subdir/*.tmp" and
// "**/*.tmp" to match nested files.
//
// Patterns ending with "/" are directory-only and are skipped by Match;
// use MatchDir for directory-level ignore checks.
func (m *Matcher) Match(filename, relPath string) bool {
	// Hardcoded: always ignore these files at any depth.
	if filename == ledgerFilename || filename == pixeignoreFilename {
		return true
	}

	// Normalize to forward slashes for doublestar (which uses path semantics).
	slashName := filepath.ToSlash(filename)
	slashRel := filepath.ToSlash(relPath)

	// Check global patterns.
	for _, pattern := range m.global {
		if matchesFile(pattern, slashName, slashRel) {
			return true
		}
	}

	// Check active .pixeignore scopes.
	for i := range m.scopes {
		sc := &m.scopes[i]
		// Compute the file path relative to this scope's base directory.
		scopeRel := scopeRelPath(sc.basePath, slashRel)
		for _, pattern := range sc.patterns {
			if matchesFile(pattern, slashName, scopeRel) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// matchesFile reports whether a single pattern matches a file identified by
// its base name (slashName) and its path relative to some base (slashRel).
// Patterns ending with "/" are directory-only and are skipped.
func matchesFile(pattern, slashName, slashRel string) bool {
	if strings.HasSuffix(pattern, "/") {
		return false // directory-only pattern
	}
	if matched, _ := doublestar.Match(pattern, slashName); matched {
		return true
	}
	if slashRel != slashName {
		if matched, _ := doublestar.Match(pattern, slashRel); matched {
			return true
		}
	}
	return false
}

// matchesDir reports whether a single pattern should cause a directory to be
// skipped. Only trailing-slash and "/**"-suffix patterns are considered.
func matchesDir(pattern, slashName, slashRel string) bool {
	switch {
	case strings.HasSuffix(pattern, "/"):
		// Trailing-slash pattern: strip "/" and match the directory name/path.
		p := pattern[:len(pattern)-1]
		if matched, _ := doublestar.Match(p, slashName); matched {
			return true
		}
		if slashRel != slashName {
			if matched, _ := doublestar.Match(p, slashRel); matched {
				return true
			}
		}

	case strings.HasSuffix(pattern, "/**"):
		// Implicit directory pattern: strip "/**" and match the directory path.
		// e.g. "backups/**" → skip the "backups" directory itself.
		p := pattern[:len(pattern)-3]
		if matched, _ := doublestar.Match(p, slashName); matched {
			return true
		}
		if slashRel != slashName {
			if matched, _ := doublestar.Match(p, slashRel); matched {
				return true
			}
		}
	}
	return false
}

// scopeRelPath computes the path of a file/dir relative to a scope's basePath.
// Both basePath and fullRel are forward-slash paths.
//
// Examples:
//
//	scopeRelPath(".", "vacation/photo.jpg")    → "vacation/photo.jpg"
//	scopeRelPath("vacation", "vacation/photo.jpg") → "photo.jpg"
//	scopeRelPath("a/b", "a/b/c/photo.jpg")    → "c/photo.jpg"
func scopeRelPath(basePath, fullRel string) string {
	if basePath == "." || basePath == "" {
		return fullRel
	}
	prefix := basePath + "/"
	if strings.HasPrefix(fullRel, prefix) {
		return fullRel[len(prefix):]
	}
	// fullRel is not under this scope's base — return the full path so that
	// patterns won't accidentally match outside their scope.
	return fullRel
}

// dedup returns a deduplicated, whitespace-trimmed copy of patterns with empty
// strings removed.
func dedup(patterns []string) []string {
	seen := make(map[string]bool, len(patterns))
	var clean []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			clean = append(clean, p)
		}
	}
	return clean
}
