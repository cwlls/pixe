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
	"path/filepath"
	"strings"
)

// SidecarFile represents a pre-existing sidecar file in dirA that is
// associated with a parent media file by stem matching.
type SidecarFile struct {
	Path    string // absolute path in dirA
	RelPath string // relative path from dirA
	Ext     string // normalized lowercase extension (e.g., ".aae", ".xmp")
}

// sidecarExtensions is the set of file extensions recognized as sidecars.
// This is a package-level constant — not user-configurable. Adding new
// extensions is a code change, not a config change.
var sidecarExtensions = map[string]bool{
	".aae": true,
	".xmp": true,
}

// associateSidecars performs a second pass over the skipped file list,
// matching recognized sidecar files to their parent DiscoveredFile entries
// by stem within the same directory.
//
// Matched sidecars are appended to the parent's Sidecars slice and removed
// from the skipped list. Unmatched sidecars have their reason updated to
// "orphan sidecar: no matching media file".
//
// The discovered slice is modified in-place (Sidecars field). The returned
// skipped slice is a new slice with matched sidecars removed.
//
// Note: SkippedFile.Path holds the relative path from dirA (same convention
// as DiscoveredFile.RelPath). The absolute path for each sidecar is derived
// from its parent's absolute path directory.
func associateSidecars(discovered []DiscoveredFile, skipped []SkippedFile) ([]DiscoveredFile, []SkippedFile) {
	if len(discovered) == 0 {
		// No media files — all sidecars are orphans; update their reasons.
		for i, sf := range skipped {
			ext := strings.ToLower(filepath.Ext(sf.Path))
			if sidecarExtensions[ext] {
				skipped[i].Reason = "orphan sidecar: no matching media file"
			}
		}
		return discovered, skipped
	}

	// Build two indexes keyed by (dir, lowercase_key) → index into discovered.
	//
	// stemIndex:     (dir, lowercase_stem)          e.g. "vacation\x00img_1234"
	// fullNameIndex: (dir, lowercase_full_filename)  e.g. "vacation\x00img_1234.heic"
	//
	// The full-name index supports the Adobe full-extension convention:
	//   IMG_1234.HEIC.xmp → parent is IMG_1234.HEIC
	stemIndex := make(map[string]int, len(discovered))
	fullNameIndex := make(map[string]int, len(discovered))

	for i, df := range discovered {
		dir := filepath.Dir(df.RelPath)
		base := filepath.Base(df.RelPath)
		stem := strings.TrimSuffix(base, filepath.Ext(base))

		stemKey := dir + "\x00" + strings.ToLower(stem)
		fullKey := dir + "\x00" + strings.ToLower(base)

		// First writer wins — preserves discovery order for ambiguous stems.
		if _, exists := stemIndex[stemKey]; !exists {
			stemIndex[stemKey] = i
		}
		if _, exists := fullNameIndex[fullKey]; !exists {
			fullNameIndex[fullKey] = i
		}
	}

	// Track which skipped entries were matched so we can remove them.
	matched := make([]bool, len(skipped))

	for si, sf := range skipped {
		// sf.Path is the relative path from dirA (SkippedFile convention).
		ext := strings.ToLower(filepath.Ext(sf.Path))
		if !sidecarExtensions[ext] {
			continue // not a sidecar extension
		}

		dir := filepath.Dir(sf.Path)
		base := filepath.Base(sf.Path)

		// Strip the sidecar extension to get the candidate stem.
		// For "IMG_1234.HEIC.xmp" → stemCandidate is "IMG_1234.HEIC".
		// For "IMG_1234.xmp"      → stemCandidate is "IMG_1234".
		stemCandidate := strings.TrimSuffix(base, ext)

		// 1. Try full-name index first (unambiguous: "IMG_1234.HEIC.xmp" → "IMG_1234.HEIC").
		fullKey := dir + "\x00" + strings.ToLower(stemCandidate)
		if idx, ok := fullNameIndex[fullKey]; ok {
			parentAbsDir := filepath.Dir(discovered[idx].Path)
			discovered[idx].Sidecars = append(discovered[idx].Sidecars, SidecarFile{
				Path:    filepath.Join(parentAbsDir, base),
				RelPath: sf.Path,
				Ext:     ext,
			})
			matched[si] = true
			continue
		}

		// 2. Fall back to stem index (plain stem: "IMG_1234.xmp" → stem "IMG_1234").
		stemKey := dir + "\x00" + strings.ToLower(stemCandidate)
		if idx, ok := stemIndex[stemKey]; ok {
			parentAbsDir := filepath.Dir(discovered[idx].Path)
			discovered[idx].Sidecars = append(discovered[idx].Sidecars, SidecarFile{
				Path:    filepath.Join(parentAbsDir, base),
				RelPath: sf.Path,
				Ext:     ext,
			})
			matched[si] = true
			continue
		}

		// No parent found — mark as orphan.
		skipped[si].Reason = "orphan sidecar: no matching media file"
	}

	// Build a new skipped slice without matched sidecars.
	filtered := make([]SkippedFile, 0, len(skipped))
	for i, sf := range skipped {
		if !matched[i] {
			filtered = append(filtered, sf)
		}
	}

	return discovered, filtered
}
