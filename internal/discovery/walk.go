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
	Path     string                 // absolute path for file I/O
	RelPath  string                 // relative path from dirA for display and ledger
	Handler  domain.FileTypeHandler // resolved handler for this file type
	Sidecars []SidecarFile          // pre-existing sidecars from dirA (may be empty)
}

// SkippedFile records a file that could not be classified or was intentionally
// excluded from processing.
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

	// CarrySidecars, when true, causes Walk to perform a second pass after
	// classification, associating recognized sidecar files (.aae, .xmp) with
	// their parent media files by stem matching. Matched sidecars are removed
	// from the skipped list and attached to their parent DiscoveredFile.
	// When false, sidecars remain in the skipped list as "unsupported format".
	CarrySidecars bool
}

// Walk discovers files in dirA, classifies each regular file using the
// provided registry, and returns two slices:
//   - discovered: files with a matched handler, ready for the pipeline.
//   - skipped: files that were excluded with a human-readable reason.
//
// Files matching the ignore list (including the hardcoded .pixe_ledger.json
// and .pixeignore) are completely invisible — they appear in neither slice.
//
// Dot-directories (names starting with ".") are never descended into.
// When opts.Recursive is false, all subdirectories are skipped.
//
// When opts.Recursive is true and opts.Ignore is non-nil, Walk also:
//   - Calls opts.Ignore.MatchDir to skip directories matching user patterns.
//   - Loads .pixeignore files found in each directory via opts.Ignore.PushScope,
//     and pops those scopes when the walk exits the directory.
func Walk(dirA string, reg *Registry, opts WalkOptions) (discovered []DiscoveredFile, skipped []SkippedFile, err error) {
	// scopeStack tracks absolute paths of directories for which PushScope was
	// called, so we can call PopScope when the walk moves past them.
	// filepath.WalkDir visits entries in lexical order; a directory is "exited"
	// when the current path is no longer a descendant of the top of the stack.
	var scopeStack []string

	err = filepath.WalkDir(dirA, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("discovery: walk %q: %w", path, walkErr)
		}

		// --- Pop scopes for directories we have left ---
		// We detect a directory exit when the current path is no longer under
		// the top of the scope stack.
		if opts.Ignore != nil {
			for len(scopeStack) > 0 {
				top := scopeStack[len(scopeStack)-1]
				if path == top || strings.HasPrefix(path, top+string(filepath.Separator)) {
					break // still inside this scope's directory
				}
				opts.Ignore.PopScope()
				scopeStack = scopeStack[:len(scopeStack)-1]
			}
		}

		name := d.Name()

		// --- Directory handling ---
		if d.IsDir() {
			if path == dirA {
				// Root directory: always enter. Load root .pixeignore if present.
				if opts.Ignore != nil {
					pixeignorePath := filepath.Join(dirA, ".pixeignore")
					if opts.Ignore.PushScope(".", pixeignorePath) {
						scopeStack = append(scopeStack, dirA)
					}
				}
				return nil
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir // always skip dot-directories
			}
			if !opts.Recursive {
				return filepath.SkipDir // non-recursive: skip all subdirs
			}
			// Check user-configured directory ignore patterns (trailing-slash
			// and "/**"-suffix patterns via MatchDir).
			if opts.Ignore != nil {
				relDir, _ := filepath.Rel(dirA, path)
				if opts.Ignore.MatchDir(name, relDir) {
					return filepath.SkipDir
				}
				// Load .pixeignore in this subdirectory (if present).
				relDir = filepath.ToSlash(relDir)
				pixeignorePath := filepath.Join(path, ".pixeignore")
				if opts.Ignore.PushScope(relDir, pixeignorePath) {
					scopeStack = append(scopeStack, path)
				}
			}
			return nil // recursive: descend
		}

		// --- Compute relative path from dirA ---
		relPath, _ := filepath.Rel(dirA, path)

		// --- Apply ignore matcher (includes hardcoded ledger + .pixeignore ignore) ---
		if opts.Ignore != nil && opts.Ignore.Match(name, relPath) {
			return nil // completely invisible
		}

		// --- Dotfile policy (hardcoded, not configurable) ---
		// .pixe_ledger.json and .pixeignore are caught above by the ignore matcher.
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

	// Final cleanup: pop any scopes that were never exited (e.g. if the walk
	// ended inside a scoped directory).
	if opts.Ignore != nil {
		for range scopeStack {
			opts.Ignore.PopScope()
		}
	}

	// Second pass: associate sidecar files with their parent media files.
	if err == nil && opts.CarrySidecars {
		discovered, skipped = associateSidecars(discovered, skipped)
	}

	return discovered, skipped, err
}
