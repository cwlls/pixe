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
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/ignore"
)

// --- Mock handler ---

// mockHandler is a test double for domain.FileTypeHandler.
type mockHandler struct {
	exts  []string
	magic []domain.MagicSignature
	name  string
}

func (m *mockHandler) Extensions() []string                                  { return m.exts }
func (m *mockHandler) MagicBytes() []domain.MagicSignature                   { return m.magic }
func (m *mockHandler) Detect(filePath string) (bool, error)                  { return true, nil }
func (m *mockHandler) ExtractDate(filePath string) (time.Time, error)        { return time.Time{}, nil }
func (m *mockHandler) HashableReader(filePath string) (io.ReadCloser, error) { return nil, nil }
func (m *mockHandler) MetadataSupport() domain.MetadataCapability            { return domain.MetadataNone }
func (m *mockHandler) WriteMetadataTags(filePath string, tags domain.MetadataTags) error {
	return nil
}

// jpegMagic is the standard JPEG header.
var jpegMagic = []domain.MagicSignature{{Offset: 0, Bytes: []byte{0xFF, 0xD8, 0xFF}}}

// pngMagic is the standard PNG header.
var pngMagic = []domain.MagicSignature{{Offset: 0, Bytes: []byte{0x89, 0x50, 0x4E, 0x47}}}

// writeFile creates a file at path with the given content.
func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// --- Registry tests ---

func TestRegistry_extensionMatch_magicVerified(t *testing.T) {
	reg := NewRegistry()
	h := &mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"}
	reg.Register(h)

	dir := t.TempDir()
	f := filepath.Join(dir, "photo.jpg")
	writeFile(t, f, append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))

	got, err := reg.Detect(f)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got != h {
		t.Errorf("expected jpeg handler, got %v", got)
	}
}

func TestRegistry_extensionMatch_magicFails_reclassified(t *testing.T) {
	// File has .jpg extension but PNG magic bytes → should be reclassified to pngHandler.
	reg := NewRegistry()
	jpegH := &mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"}
	pngH := &mockHandler{exts: []string{".png"}, magic: pngMagic, name: "png"}
	reg.Register(jpegH)
	reg.Register(pngH)

	dir := t.TempDir()
	f := filepath.Join(dir, "actually_png.jpg")
	writeFile(t, f, append([]byte{0x89, 0x50, 0x4E, 0x47}, make([]byte, 12)...))

	got, err := reg.Detect(f)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got != pngH {
		t.Errorf("expected reclassification to png handler, got %v", got)
	}
}

func TestRegistry_noMatch_returnsNil(t *testing.T) {
	reg := NewRegistry()
	h := &mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"}
	reg.Register(h)

	dir := t.TempDir()
	f := filepath.Join(dir, "unknown.bin")
	writeFile(t, f, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F})

	got, err := reg.Detect(f)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil handler for unknown file, got %v", got)
	}
}

func TestRegistry_noExtensionMatch_magicFallback(t *testing.T) {
	// File has no known extension but magic bytes match a registered handler.
	reg := NewRegistry()
	h := &mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"}
	reg.Register(h)

	dir := t.TempDir()
	f := filepath.Join(dir, "photo.unknown")
	writeFile(t, f, append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))

	got, err := reg.Detect(f)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got != h {
		t.Errorf("expected jpeg handler via magic fallback, got %v", got)
	}
}

func TestRegistry_shortFile_noMatch(t *testing.T) {
	// File shorter than magic signature offset — should not panic, should return nil.
	reg := NewRegistry()
	h := &mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"}
	reg.Register(h)

	dir := t.TempDir()
	f := filepath.Join(dir, "tiny.jpg")
	writeFile(t, f, []byte{0xFF}) // only 1 byte

	got, err := reg.Detect(f)
	if err != nil {
		t.Fatalf("Detect on short file: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for file shorter than magic signature, got %v", got)
	}
}

// --- Walk tests ---

func TestWalk_classifiesKnownFiles(t *testing.T) {
	dir := t.TempDir()

	// Create two JPEG files and one unknown.
	writeFile(t, filepath.Join(dir, "a.jpg"), append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))
	writeFile(t, filepath.Join(dir, "b.jpg"), append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))
	writeFile(t, filepath.Join(dir, "c.bin"), []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F})

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 2 {
		t.Errorf("discovered: got %d, want 2", len(discovered))
	}
	if len(skipped) != 1 {
		t.Errorf("skipped: got %d, want 1 (c.bin)", len(skipped))
	}
}

func TestWalk_skipsDotfiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, ".DS_Store"), []byte("junk"))
	writeFile(t, filepath.Join(dir, ".pixe_ledger.json"), []byte("{}"))
	writeFile(t, filepath.Join(dir, "photo.jpg"), append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1", len(discovered))
	}
	// Both dotfiles should appear in skipped (no ignore matcher supplied).
	if len(skipped) != 2 {
		t.Errorf("skipped: got %d, want 2 (dotfiles)", len(skipped))
	}
}

func TestWalk_skipsDotDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a .pixe subdirectory with a file inside — should be skipped entirely.
	writeFile(t, filepath.Join(dir, ".pixe", "manifest.json"), []byte("{}"))
	writeFile(t, filepath.Join(dir, "photo.jpg"), append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, _, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (dot-directory contents must be skipped)", len(discovered))
	}
}

func TestWalk_recurseSubdirectories(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "2021", "12", "photo.jpg"),
		append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))
	writeFile(t, filepath.Join(dir, "2022", "1", "photo.jpg"),
		append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, _, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 2 {
		t.Errorf("discovered: got %d, want 2", len(discovered))
	}
}

func TestWalk_emptyDirectory(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry()

	discovered, skipped, err := Walk(dir, reg, WalkOptions{})
	if err != nil {
		t.Fatalf("Walk on empty dir: %v", err)
	}
	if len(discovered) != 0 || len(skipped) != 0 {
		t.Errorf("empty dir: got discovered=%d skipped=%d, want 0 0", len(discovered), len(skipped))
	}
}

// --- Recursive + ignore tests (Task 14) ---

// TestWalk_nonRecursiveSkipsSubdirs verifies that with Recursive=false only
// top-level files are returned; nested files are silently ignored.
func TestWalk_nonRecursiveSkipsSubdirs(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "a.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "sub", "b.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: false})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (only a.jpg)", len(discovered))
	}
	if discovered[0].RelPath != "a.jpg" {
		t.Errorf("discovered[0].RelPath = %q, want %q", discovered[0].RelPath, "a.jpg")
	}
	// b.jpg is in a subdir that was skipped entirely — it should not appear in skipped either.
	for _, sf := range skipped {
		if sf.Path == filepath.Join("sub", "b.jpg") {
			t.Errorf("b.jpg should not appear in skipped when its parent dir is skipped")
		}
	}
}

// TestWalk_recursiveFindsNestedFiles verifies that Recursive=true descends into
// subdirectories and discovers files at any depth.
func TestWalk_recursiveFindsNestedFiles(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "a.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "sub", "b.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "sub", "deep", "c.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, _, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 3 {
		t.Errorf("discovered: got %d, want 3", len(discovered))
	}
}

// TestWalk_ignorePatternExcludesFile verifies that a file matching an ignore
// pattern is completely invisible — not in discovered, not in skipped.
func TestWalk_ignorePatternExcludesFile(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "a.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "notes.txt"), []byte("hello"))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	// notes.txt would normally appear in skipped (unsupported format).
	// With the ignore pattern it should be completely invisible.
	ignoreMatcher := newIgnoreMatcher(t, []string{"*.txt"})
	discovered, skipped, err := Walk(dir, reg, WalkOptions{
		Recursive: false,
		Ignore:    ignoreMatcher,
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (a.jpg only)", len(discovered))
	}
	for _, sf := range skipped {
		if sf.Path == "notes.txt" {
			t.Errorf("notes.txt should be invisible (ignored), not in skipped")
		}
	}
}

// TestWalk_ledgerFileAlwaysIgnored verifies that .pixe_ledger.json is always
// invisible even without an explicit ignore pattern.
func TestWalk_ledgerFileAlwaysIgnored(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, ".pixe_ledger.json"), []byte("{}"))
	writeFile(t, filepath.Join(dir, "a.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	// Supply an ignore matcher with no patterns — the ledger hardcode still applies.
	ignoreMatcher := newIgnoreMatcher(t, nil)
	discovered, skipped, err := Walk(dir, reg, WalkOptions{
		Recursive: false,
		Ignore:    ignoreMatcher,
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (a.jpg only)", len(discovered))
	}
	for _, sf := range skipped {
		if sf.Path == ".pixe_ledger.json" {
			t.Errorf(".pixe_ledger.json should be invisible (hardcoded ignore), not in skipped")
		}
	}
}

// TestWalk_dotfilesStillSkipped verifies that dotfiles not covered by the
// ignore list (e.g. .DS_Store) appear in skipped with reason "dotfile".
func TestWalk_dotfilesStillSkipped(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, ".DS_Store"), []byte("junk"))
	writeFile(t, filepath.Join(dir, "a.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	// No ignore patterns — .DS_Store should land in skipped with reason "dotfile".
	ignoreMatcher := newIgnoreMatcher(t, nil)
	discovered, skipped, err := Walk(dir, reg, WalkOptions{
		Recursive: false,
		Ignore:    ignoreMatcher,
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1", len(discovered))
	}
	found := false
	for _, sf := range skipped {
		if sf.Path == ".DS_Store" && sf.Reason == "dotfile" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected .DS_Store in skipped with reason %q; skipped = %v", "dotfile", skipped)
	}
}

// TestWalk_relPathPopulatedCorrectly verifies that DiscoveredFile.RelPath is
// set to the path relative to dirA (not the absolute path).
func TestWalk_relPathPopulatedCorrectly(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "sub", "c.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, _, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("discovered: got %d, want 1", len(discovered))
	}
	wantRel := filepath.Join("sub", "c.jpg")
	if discovered[0].RelPath != wantRel {
		t.Errorf("RelPath = %q, want %q", discovered[0].RelPath, wantRel)
	}
	// AbsPath must be absolute and end with the relative path.
	if !filepath.IsAbs(discovered[0].Path) {
		t.Errorf("Path %q should be absolute", discovered[0].Path)
	}
}

// TestWalk_skippedFileRelPath verifies that SkippedFile.Path is relative to dirA.
func TestWalk_skippedFileRelPath(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "sub", "notes.txt"), []byte("hello"))

	reg := NewRegistry()
	// No handlers registered — notes.txt will be skipped as unsupported.

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 0 {
		t.Errorf("discovered: got %d, want 0", len(discovered))
	}
	if len(skipped) != 1 {
		t.Fatalf("skipped: got %d, want 1", len(skipped))
	}
	wantRel := filepath.Join("sub", "notes.txt")
	if skipped[0].Path != wantRel {
		t.Errorf("skipped[0].Path = %q, want %q", skipped[0].Path, wantRel)
	}
}

// TestWalk_ignorePatternInSubdir verifies that a path-qualified ignore pattern
// (e.g. "sub/*.txt") only ignores files in that subdirectory.
func TestWalk_ignorePatternInSubdir(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "a.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "sub", "notes.txt"), []byte("hello"))
	writeFile(t, filepath.Join(dir, "notes.txt"), []byte("top-level")) // should NOT be ignored

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	ignoreMatcher := newIgnoreMatcher(t, []string{"sub/*.txt"})
	discovered, skipped, err := Walk(dir, reg, WalkOptions{
		Recursive: true,
		Ignore:    ignoreMatcher,
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	// a.jpg discovered, top-level notes.txt in skipped (unsupported), sub/notes.txt invisible.
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1", len(discovered))
	}
	topLevelTxtFound := false
	subTxtFound := false
	for _, sf := range skipped {
		if sf.Path == "notes.txt" {
			topLevelTxtFound = true
		}
		if sf.Path == filepath.Join("sub", "notes.txt") {
			subTxtFound = true
		}
	}
	if !topLevelTxtFound {
		t.Error("expected top-level notes.txt in skipped (unsupported format)")
	}
	if subTxtFound {
		t.Error("sub/notes.txt should be invisible (matched ignore pattern sub/*.txt)")
	}
}

// newIgnoreMatcher is a test helper that constructs an *ignore.Matcher.
func newIgnoreMatcher(t *testing.T, patterns []string) *ignore.Matcher {
	t.Helper()
	return ignore.New(patterns)
}
