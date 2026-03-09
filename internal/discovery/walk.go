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

package discovery

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/ignore"
)

// DiscoveredFile pairs a source path with its resolved FileTypeHandler.
type DiscoveredFile struct {
	Path    string // absolute path for file I/O
	RelPath string // relative path from dirA for display and ledger
	Handler domain.FileTypeHandler
}

// SkippedFile records a file that could not be classified or was intentionally
// excluded from processing. Path is relative to dirA.
type SkippedFile struct {
	Path   string // relative path from dirA
	Reason string // human-readable reason
}

// WalkOptions configures the discovery walk.
type WalkOptions struct {
	// Recursive, when true, causes Walk to descend into subdirectories of dirA.
	// When false (the default) only the top-level files in dirA are processed.
	Recursive bool

	// Ignore is an optional matcher for files to exclude completely.
	// Ignored files do not appear in either the discovered or skipped slices.
	// If nil, only the hardcoded ledger-file exclusion applies (handled inside
	// the ignore package's zero-value Matcher).
	Ignore *ignore.Matcher
}

// Walk discovers files in dirA, classifies each regular file using the
// provided registry, and returns two slices:
//   - discovered: files with a matched handler, ready for the pipeline.
//   - skipped: files that were excluded with a human-readable reason.
//
// Files matching the ignore list (including the hardcoded .pixe_ledger.json)
// are completely invisible — they appear in neither slice.
//
// Dot-directories (names starting with ".") are never descended into.
// When opts.Recursive is false, all subdirectories are skipped.
func Walk(dirA string, reg *Registry, opts WalkOptions) (discovered []DiscoveredFile, skipped []SkippedFile, err error) {
	err = filepath.WalkDir(dirA, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("discovery: walk %q: %w", path, walkErr)
		}

		name := d.Name()

		// --- Directory handling ---
		if d.IsDir() {
			if path == dirA {
				return nil // always enter the root
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir // always skip dot-directories
			}
			if !opts.Recursive {
				return filepath.SkipDir // non-recursive: skip all subdirs
			}
			return nil // recursive: descend
		}

		// --- Compute relative path from dirA ---
		relPath, _ := filepath.Rel(dirA, path)

		// --- Apply ignore matcher (includes hardcoded ledger ignore) ---
		if opts.Ignore != nil && opts.Ignore.Match(name, relPath) {
			return nil // completely invisible
		}

		// --- Dotfile policy (hardcoded, not configurable) ---
		// .pixe_ledger.json is caught above by the ignore matcher.
		// Other dotfiles (e.g. .DS_Store) are skipped with a reason.
		if strings.HasPrefix(name, ".") {
			skipped = append(skipped, SkippedFile{
				Path:   relPath,
				Reason: "dotfile",
			})
			return nil
		}

		// --- Classify via registry ---
		handler, detErr := reg.Detect(path)
		if detErr != nil {
			skipped = append(skipped, SkippedFile{
				Path:   relPath,
				Reason: fmt.Sprintf("detection error: %v", detErr),
			})
			return nil
		}
		if handler == nil {
			ext := filepath.Ext(name)
			skipped = append(skipped, SkippedFile{
				Path:   relPath,
				Reason: fmt.Sprintf("unsupported format: %s", ext),
			})
			return nil
		}

		discovered = append(discovered, DiscoveredFile{
			Path:    path,    // absolute, for file I/O
			RelPath: relPath, // relative, for display and ledger
			Handler: handler,
		})
		return nil
	})
	return discovered, skipped, err
}
