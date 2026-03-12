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
	"testing"
)

// --- associateSidecars unit tests ---

// makeDiscovered builds a minimal DiscoveredFile for testing.
// absDir is the absolute directory; relPath is relative to dirA.
func makeDiscovered(absDir, relPath string) DiscoveredFile {
	return DiscoveredFile{
		Path:    filepath.Join(absDir, relPath),
		RelPath: relPath,
	}
}

// makeSkipped builds a minimal SkippedFile for testing.
func makeSkipped(relPath, reason string) SkippedFile {
	return SkippedFile{Path: relPath, Reason: reason}
}

// TestAssociateSidecars_stemMatch verifies that a plain-stem sidecar
// (e.g. IMG_1234.xmp) is associated with its parent (IMG_1234.HEIC).
func TestAssociateSidecars_stemMatch(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "IMG_1234.HEIC"),
	}
	skipped := []SkippedFile{
		makeSkipped("IMG_1234.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	if len(disc[0].Sidecars) != 1 {
		t.Fatalf("Sidecars: got %d, want 1", len(disc[0].Sidecars))
	}
	sc := disc[0].Sidecars[0]
	if sc.Ext != ".xmp" {
		t.Errorf("Ext = %q, want %q", sc.Ext, ".xmp")
	}
	if sc.RelPath != "IMG_1234.xmp" {
		t.Errorf("RelPath = %q, want %q", sc.RelPath, "IMG_1234.xmp")
	}
	wantAbsPath := filepath.Join(absDir, "IMG_1234.xmp")
	if sc.Path != wantAbsPath {
		t.Errorf("Path = %q, want %q", sc.Path, wantAbsPath)
	}
	// Matched sidecar must be removed from skipped.
	if len(skip) != 0 {
		t.Errorf("skipped: got %d, want 0", len(skip))
	}
}

// TestAssociateSidecars_fullExtensionMatch verifies that the full-extension
// convention (IMG_1234.HEIC.xmp) associates with IMG_1234.HEIC specifically.
func TestAssociateSidecars_fullExtensionMatch(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "IMG_1234.HEIC"),
		makeDiscovered(absDir, "IMG_1234.JPG"),
	}
	skipped := []SkippedFile{
		makeSkipped("IMG_1234.HEIC.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	// The .xmp must go to HEIC, not JPG.
	if len(disc[0].Sidecars) != 1 {
		t.Fatalf("HEIC Sidecars: got %d, want 1", len(disc[0].Sidecars))
	}
	if disc[0].Sidecars[0].RelPath != "IMG_1234.HEIC.xmp" {
		t.Errorf("HEIC sidecar RelPath = %q, want %q", disc[0].Sidecars[0].RelPath, "IMG_1234.HEIC.xmp")
	}
	if len(disc[1].Sidecars) != 0 {
		t.Errorf("JPG Sidecars: got %d, want 0", len(disc[1].Sidecars))
	}
	if len(skip) != 0 {
		t.Errorf("skipped: got %d, want 0", len(skip))
	}
}

// TestAssociateSidecars_caseInsensitive verifies that stem matching is
// case-insensitive (img_1234.xmp matches IMG_1234.HEIC).
func TestAssociateSidecars_caseInsensitive(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "IMG_1234.HEIC"),
	}
	skipped := []SkippedFile{
		makeSkipped("img_1234.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	if len(disc[0].Sidecars) != 1 {
		t.Fatalf("Sidecars: got %d, want 1", len(disc[0].Sidecars))
	}
	if len(skip) != 0 {
		t.Errorf("skipped: got %d, want 0", len(skip))
	}
}

// TestAssociateSidecars_multipleSidecars verifies that both .aae and .xmp
// sidecars are associated with the same parent.
func TestAssociateSidecars_multipleSidecars(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "IMG_1234.HEIC"),
	}
	skipped := []SkippedFile{
		makeSkipped("IMG_1234.aae", "unsupported format: .aae"),
		makeSkipped("IMG_1234.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	if len(disc[0].Sidecars) != 2 {
		t.Fatalf("Sidecars: got %d, want 2", len(disc[0].Sidecars))
	}
	exts := map[string]bool{}
	for _, sc := range disc[0].Sidecars {
		exts[sc.Ext] = true
	}
	if !exts[".aae"] || !exts[".xmp"] {
		t.Errorf("expected both .aae and .xmp sidecars; got %v", exts)
	}
	if len(skip) != 0 {
		t.Errorf("skipped: got %d, want 0", len(skip))
	}
}

// TestAssociateSidecars_orphan verifies that a sidecar with no matching
// parent remains in skipped with the orphan reason.
func TestAssociateSidecars_orphan(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "OTHER.HEIC"),
	}
	skipped := []SkippedFile{
		makeSkipped("ORPHAN.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	if len(disc[0].Sidecars) != 0 {
		t.Errorf("OTHER.HEIC Sidecars: got %d, want 0", len(disc[0].Sidecars))
	}
	if len(skip) != 1 {
		t.Fatalf("skipped: got %d, want 1", len(skip))
	}
	const wantReason = "orphan sidecar: no matching media file"
	if skip[0].Reason != wantReason {
		t.Errorf("orphan reason = %q, want %q", skip[0].Reason, wantReason)
	}
}

// TestAssociateSidecars_noMediaFiles verifies that all sidecars become orphans
// when there are no discovered media files.
func TestAssociateSidecars_noMediaFiles(t *testing.T) {
	skipped := []SkippedFile{
		makeSkipped("IMG_1234.xmp", "unsupported format: .xmp"),
		makeSkipped("IMG_5678.aae", "unsupported format: .aae"),
	}

	disc, skip := associateSidecars(nil, skipped)

	if len(disc) != 0 {
		t.Errorf("discovered: got %d, want 0", len(disc))
	}
	if len(skip) != 2 {
		t.Fatalf("skipped: got %d, want 2", len(skip))
	}
	for _, sf := range skip {
		const wantReason = "orphan sidecar: no matching media file"
		if sf.Reason != wantReason {
			t.Errorf("reason for %q = %q, want %q", sf.Path, sf.Reason, wantReason)
		}
	}
}

// TestAssociateSidecars_ambiguousStem verifies that when two media files share
// a stem, the sidecar associates with the first one (discovery order).
func TestAssociateSidecars_ambiguousStem(t *testing.T) {
	absDir := "/tmp/source"
	// HEIC is first in discovery order.
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "IMG_1234.HEIC"),
		makeDiscovered(absDir, "IMG_1234.JPG"),
	}
	skipped := []SkippedFile{
		makeSkipped("IMG_1234.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	// Sidecar goes to the first file (HEIC).
	if len(disc[0].Sidecars) != 1 {
		t.Errorf("HEIC Sidecars: got %d, want 1", len(disc[0].Sidecars))
	}
	if len(disc[1].Sidecars) != 0 {
		t.Errorf("JPG Sidecars: got %d, want 0", len(disc[1].Sidecars))
	}
	if len(skip) != 0 {
		t.Errorf("skipped: got %d, want 0", len(skip))
	}
}

// TestAssociateSidecars_fullExtDisambiguates verifies that the full-extension
// convention takes priority over stem matching when both media files exist.
func TestAssociateSidecars_fullExtDisambiguates(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "IMG_1234.HEIC"),
		makeDiscovered(absDir, "IMG_1234.JPG"),
	}
	skipped := []SkippedFile{
		// Full-extension sidecar for JPG specifically.
		makeSkipped("IMG_1234.JPG.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	// Must go to JPG, not HEIC.
	if len(disc[0].Sidecars) != 0 {
		t.Errorf("HEIC Sidecars: got %d, want 0", len(disc[0].Sidecars))
	}
	if len(disc[1].Sidecars) != 1 {
		t.Fatalf("JPG Sidecars: got %d, want 1", len(disc[1].Sidecars))
	}
	if disc[1].Sidecars[0].RelPath != "IMG_1234.JPG.xmp" {
		t.Errorf("RelPath = %q, want %q", disc[1].Sidecars[0].RelPath, "IMG_1234.JPG.xmp")
	}
	if len(skip) != 0 {
		t.Errorf("skipped: got %d, want 0", len(skip))
	}
}

// TestAssociateSidecars_nonSidecarSkippedPreserved verifies that non-sidecar
// skipped entries (e.g. dotfiles, unsupported formats) are left unchanged.
func TestAssociateSidecars_nonSidecarSkippedPreserved(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		makeDiscovered(absDir, "IMG_1234.HEIC"),
	}
	skipped := []SkippedFile{
		makeSkipped(".DS_Store", "dotfile"),
		makeSkipped("notes.txt", "unsupported format: .txt"),
		makeSkipped("IMG_1234.xmp", "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	// .xmp matched; .DS_Store and notes.txt remain.
	if len(disc[0].Sidecars) != 1 {
		t.Errorf("Sidecars: got %d, want 1", len(disc[0].Sidecars))
	}
	if len(skip) != 2 {
		t.Errorf("skipped: got %d, want 2; got %v", len(skip), skip)
	}
}

// TestAssociateSidecars_differentDirectory verifies that a sidecar in a
// subdirectory does NOT match a parent in a different directory.
func TestAssociateSidecars_differentDirectory(t *testing.T) {
	absDir := "/tmp/source"
	discovered := []DiscoveredFile{
		// Parent is in root.
		makeDiscovered(absDir, "IMG_1234.HEIC"),
	}
	skipped := []SkippedFile{
		// Sidecar is in a subdirectory — different dir, should not match.
		makeSkipped(filepath.Join("sub", "IMG_1234.xmp"), "unsupported format: .xmp"),
	}

	disc, skip := associateSidecars(discovered, skipped)

	if len(disc[0].Sidecars) != 0 {
		t.Errorf("Sidecars: got %d, want 0 (cross-directory match must not occur)", len(disc[0].Sidecars))
	}
	// The sidecar should remain in skipped as an orphan.
	if len(skip) != 1 {
		t.Fatalf("skipped: got %d, want 1", len(skip))
	}
	const wantReason = "orphan sidecar: no matching media file"
	if skip[0].Reason != wantReason {
		t.Errorf("reason = %q, want %q", skip[0].Reason, wantReason)
	}
}

// --- Walk integration tests for CarrySidecars option ---

// TestWalk_carrySidecars_disabled verifies that when CarrySidecars is false,
// sidecar files remain in the skipped list as "unsupported format".
func TestWalk_carrySidecars_disabled(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "IMG_1234.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "IMG_1234.xmp"), []byte(`<?xpacket?><x:xmpmeta/>`))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{CarrySidecars: false})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1", len(discovered))
	}
	if len(discovered[0].Sidecars) != 0 {
		t.Errorf("Sidecars: got %d, want 0 (carry disabled)", len(discovered[0].Sidecars))
	}
	// .xmp must remain in skipped.
	found := false
	for _, sf := range skipped {
		if sf.Path == "IMG_1234.xmp" {
			found = true
		}
	}
	if !found {
		t.Errorf("IMG_1234.xmp not found in skipped; skipped = %v", skipped)
	}
}

// TestWalk_carrySidecars_enabled verifies that when CarrySidecars is true,
// sidecar files are associated with their parent and removed from skipped.
func TestWalk_carrySidecars_enabled(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "IMG_1234.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "IMG_1234.xmp"), []byte(`<?xpacket?><x:xmpmeta/>`))
	writeFile(t, filepath.Join(dir, "IMG_1234.aae"), []byte(`<?xml version="1.0"?>`))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{CarrySidecars: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(discovered) != 1 {
		t.Fatalf("discovered: got %d, want 1", len(discovered))
	}
	if len(discovered[0].Sidecars) != 2 {
		t.Errorf("Sidecars: got %d, want 2", len(discovered[0].Sidecars))
	}
	// Neither .xmp nor .aae should remain in skipped.
	for _, sf := range skipped {
		if sf.Path == "IMG_1234.xmp" || sf.Path == "IMG_1234.aae" {
			t.Errorf("unexpected sidecar in skipped: %v", sf)
		}
	}
}

// TestWalk_carrySidecars_orphanInSkipped verifies that an orphan sidecar
// (no matching media file) remains in skipped with the orphan reason.
func TestWalk_carrySidecars_orphanInSkipped(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "OTHER.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "ORPHAN.xmp"), []byte(`<?xpacket?><x:xmpmeta/>`))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	_, skipped, err := Walk(dir, reg, WalkOptions{CarrySidecars: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	found := false
	for _, sf := range skipped {
		if sf.Path == "ORPHAN.xmp" {
			found = true
			const wantReason = "orphan sidecar: no matching media file"
			if sf.Reason != wantReason {
				t.Errorf("reason = %q, want %q", sf.Reason, wantReason)
			}
		}
	}
	if !found {
		t.Errorf("ORPHAN.xmp not found in skipped; skipped = %v", skipped)
	}
}

// TestWalk_carrySidecars_recursive verifies that sidecar association works
// correctly in recursive mode, matching only within the same directory.
func TestWalk_carrySidecars_recursive(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	// Root: IMG_1234.jpg + IMG_1234.xmp (should match)
	writeFile(t, filepath.Join(dir, "IMG_1234.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "IMG_1234.xmp"), []byte(`<?xpacket?><x:xmpmeta/>`))

	// Sub: IMG_5678.jpg + IMG_5678.xmp (should match within sub/)
	writeFile(t, filepath.Join(dir, "sub", "IMG_5678.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "sub", "IMG_5678.xmp"), []byte(`<?xpacket?><x:xmpmeta/>`))

	// Cross-dir: sidecar in sub/ for parent in root (should NOT match)
	writeFile(t, filepath.Join(dir, "sub", "IMG_1234.xmp"), []byte(`<?xpacket?><x:xmpmeta/>`))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true, CarrySidecars: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(discovered) != 2 {
		t.Fatalf("discovered: got %d, want 2", len(discovered))
	}

	// Find each discovered file and check its sidecars.
	for _, df := range discovered {
		switch df.RelPath {
		case "IMG_1234.jpg":
			if len(df.Sidecars) != 1 {
				t.Errorf("IMG_1234.jpg Sidecars: got %d, want 1", len(df.Sidecars))
			} else if df.Sidecars[0].RelPath != "IMG_1234.xmp" {
				t.Errorf("IMG_1234.jpg sidecar = %q, want %q", df.Sidecars[0].RelPath, "IMG_1234.xmp")
			}
		case filepath.Join("sub", "IMG_5678.jpg"):
			if len(df.Sidecars) != 1 {
				t.Errorf("sub/IMG_5678.jpg Sidecars: got %d, want 1", len(df.Sidecars))
			}
		default:
			t.Errorf("unexpected discovered file: %q", df.RelPath)
		}
	}

	// sub/IMG_1234.xmp is an orphan (no parent in sub/).
	found := false
	for _, sf := range skipped {
		if sf.Path == filepath.Join("sub", "IMG_1234.xmp") {
			found = true
			const wantReason = "orphan sidecar: no matching media file"
			if sf.Reason != wantReason {
				t.Errorf("orphan reason = %q, want %q", sf.Reason, wantReason)
			}
		}
	}
	if !found {
		t.Errorf("sub/IMG_1234.xmp orphan not found in skipped; skipped = %v", skipped)
	}
}
