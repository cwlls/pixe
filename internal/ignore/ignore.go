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
// user-configured patterns supplied via --ignore flags.
package ignore

import (
	"path/filepath"
	"strings"
)

// ledgerFilename is the hardcoded filename that is always ignored, at any
// directory depth. It is the only file Pixe writes into the source directory.
const ledgerFilename = ".pixe_ledger.json"

// Matcher holds compiled ignore patterns and provides a Match method.
// The zero value is valid and matches only the hardcoded ledger file.
type Matcher struct {
	patterns []string // deduplicated, trimmed user-configured glob patterns
}

// New creates a Matcher from user-configured glob patterns.
// Patterns are deduplicated and whitespace-trimmed; empty strings are dropped.
// The hardcoded ledger ignore (.pixe_ledger.json) is always active and does
// not need to be included in patterns.
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
	return &Matcher{patterns: clean}
}

// Match reports whether the file should be ignored.
//
// filename is the base name of the file (e.g. "IMG_0001.jpg").
// relPath is the path relative to dirA (e.g. "vacation/IMG_0001.jpg").
// For top-level files relPath == filename.
//
// The hardcoded ledger ignore is checked first (by filename equality).
// Then each user pattern is matched against both the filename and the
// relPath using filepath.Match semantics, enabling patterns such as
// "subdir/*.tmp" to match nested files.
func (m *Matcher) Match(filename, relPath string) bool {
	// Hardcoded: always ignore the ledger file at any depth.
	if filename == ledgerFilename {
		return true
	}

	for _, pattern := range m.patterns {
		// Match against the base filename.
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true
		}
		// Match against the relative path (enables "subdir/*.tmp" patterns).
		if relPath != filename {
			if matched, _ := filepath.Match(pattern, relPath); matched {
				return true
			}
		}
	}
	return false
}
