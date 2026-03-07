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
)

// DiscoveredFile pairs a source path with its resolved FileTypeHandler.
type DiscoveredFile struct {
	Path    string
	Handler domain.FileTypeHandler
}

// SkippedFile records a file that could not be classified or was intentionally
// excluded from processing.
type SkippedFile struct {
	Path   string
	Reason string
}

// Walk recursively walks dirA, classifies each regular file using the
// provided registry, and returns two slices:
//   - discovered: files with a matched handler, ready for the pipeline.
//   - skipped: files that were excluded (dotfiles, unrecognised formats).
//
// Walk never modifies any file. Directories are traversed but not returned.
// Dotfiles and dot-directories (names starting with ".") are skipped entirely.
func Walk(dirA string, reg *Registry) (discovered []DiscoveredFile, skipped []SkippedFile, err error) {
	err = filepath.WalkDir(dirA, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Surface walk errors (e.g. permission denied on a subdirectory).
			return fmt.Errorf("discovery: walk %q: %w", path, walkErr)
		}

		name := d.Name()

		// Skip dot-directories entirely (don't descend into them).
		if d.IsDir() {
			if strings.HasPrefix(name, ".") && path != dirA {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip dotfiles (e.g. .pixe_ledger.json, .DS_Store).
		if strings.HasPrefix(name, ".") {
			skipped = append(skipped, SkippedFile{
				Path:   path,
				Reason: "dotfile skipped",
			})
			return nil
		}

		// Attempt to classify the file.
		handler, detErr := reg.Detect(path)
		if detErr != nil {
			skipped = append(skipped, SkippedFile{
				Path:   path,
				Reason: fmt.Sprintf("detection error: %v", detErr),
			})
			return nil
		}
		if handler == nil {
			skipped = append(skipped, SkippedFile{
				Path:   path,
				Reason: "unrecognised format",
			})
			return nil
		}

		discovered = append(discovered, DiscoveredFile{
			Path:    path,
			Handler: handler,
		})
		return nil
	})
	return discovered, skipped, err
}
