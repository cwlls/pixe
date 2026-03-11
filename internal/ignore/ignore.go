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
package ignore

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// hardcoded filenames that are always ignored at any directory depth.
const (
	ledgerFilename     = ".pixe_ledger.json"
	pixeignoreFilename = ".pixeignore"
)

// Matcher holds compiled ignore patterns and provides Match and MatchDir
// methods. The zero value is valid and matches only the hardcoded filenames.
type Matcher struct {
	global []string // deduplicated, trimmed user-configured glob patterns
}

// New creates a Matcher from user-configured glob patterns.
// Patterns are deduplicated and whitespace-trimmed; empty strings are dropped.
// The hardcoded ignores (.pixe_ledger.json, .pixeignore) are always active and
// do not need to be included in patterns.
func New(patterns []string) *Matcher {
	seen := make(map[string]bool, len(patterns))
	var clean []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			clean = append(clean, p)
		}
	}
	return &Matcher{global: clean}
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
func (m *Matcher) MatchDir(dirname, relDirPath string) bool {
	slashName := filepath.ToSlash(dirname)
	slashRel := filepath.ToSlash(relDirPath)

	for _, pattern := range m.global {
		if matchesDir(pattern, slashName, slashRel) {
			return true
		}
	}
	return false
}

// matchesDir is the per-pattern directory-match logic shared by MatchDir and
// (in a later task) the scope-aware variant.
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

// Match reports whether the file should be ignored.
//
// filename is the base name of the file (e.g. "IMG_0001.jpg").
// relPath is the path relative to dirA (e.g. "vacation/IMG_0001.jpg").
// For top-level files relPath == filename.
//
// The hardcoded ignores are checked first (by filename equality).
// Then each user pattern is matched against both the filename and the
// relPath using doublestar semantics, enabling patterns such as
// "subdir/*.tmp" and "**/*.tmp" to match nested files.
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

	for _, pattern := range m.global {
		// Skip directory-only patterns — those are handled by MatchDir.
		if strings.HasSuffix(pattern, "/") {
			continue
		}
		// Match against the base filename.
		if matched, _ := doublestar.Match(pattern, slashName); matched {
			return true
		}
		// Match against the relative path (enables "subdir/*.tmp" and "**/*.tmp").
		if slashRel != slashName {
			if matched, _ := doublestar.Match(pattern, slashRel); matched {
				return true
			}
		}
	}
	return false
}
