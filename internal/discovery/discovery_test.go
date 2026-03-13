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
	"runtime"
	"strings"
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

// ---------------------------------------------------------------------------
// Task 5 & 6: directory-ignore + .pixeignore walk tests
// ---------------------------------------------------------------------------

// TestWalk_directoryIgnoreTrailingSlash verifies that a trailing-slash pattern
// causes an entire directory to be skipped (files inside are invisible).
func TestWalk_directoryIgnoreTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "photo.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "node_modules", "file.js"), []byte("js"))

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	m := newIgnoreMatcher(t, []string{"node_modules/"})
	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true, Ignore: m})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (photo.jpg only)", len(discovered))
	}
	// node_modules/file.js must be completely invisible.
	for _, sf := range skipped {
		if sf.Path == filepath.Join("node_modules", "file.js") {
			t.Error("node_modules/file.js should be invisible (directory skipped), not in skipped")
		}
	}
}

// TestWalk_directoryIgnoreDoublestar verifies that a "**/cache/" pattern skips
// a deeply nested cache directory entirely.
func TestWalk_directoryIgnoreDoublestar(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, "photo.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "a", "b", "cache", "thumb.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	m := newIgnoreMatcher(t, []string{"**/cache/"})
	discovered, _, err := Walk(dir, reg, WalkOptions{Recursive: true, Ignore: m})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (photo.jpg only, cache/ skipped)", len(discovered))
	}
	for _, df := range discovered {
		if df.RelPath == filepath.Join("a", "b", "cache", "thumb.jpg") {
			t.Error("thumb.jpg inside cache/ should be invisible (directory skipped)")
		}
	}
}

// TestWalk_pixeignoreLoaded verifies that a .pixeignore file in dirA is loaded
// and its patterns applied during the walk.
func TestWalk_pixeignoreLoaded(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, ".pixeignore"), []byte("*.txt\n"))
	writeFile(t, filepath.Join(dir, "notes.txt"), []byte("hello"))
	writeFile(t, filepath.Join(dir, "photo.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	m := newIgnoreMatcher(t, nil) // no global patterns — only .pixeignore
	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true, Ignore: m})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (photo.jpg only)", len(discovered))
	}
	// notes.txt must be invisible (matched by .pixeignore pattern *.txt).
	for _, sf := range skipped {
		if sf.Path == "notes.txt" {
			t.Error("notes.txt should be invisible (matched .pixeignore pattern *.txt), not in skipped")
		}
	}
}

// TestWalk_nestedPixeignore verifies that a .pixeignore in a subdirectory only
// applies to files within that subtree, not to files at the root.
func TestWalk_nestedPixeignore(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	// Root-level app.log — should NOT be ignored (pattern is in sub/.pixeignore).
	writeFile(t, filepath.Join(dir, "app.log"), []byte("root log"))
	// sub/app.log — SHOULD be ignored by sub/.pixeignore.
	writeFile(t, filepath.Join(dir, "sub", "app.log"), []byte("sub log"))
	writeFile(t, filepath.Join(dir, "sub", ".pixeignore"), []byte("*.log\n"))
	writeFile(t, filepath.Join(dir, "photo.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	m := newIgnoreMatcher(t, nil)
	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true, Ignore: m})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	// photo.jpg discovered; app.log at root is skipped (unsupported); sub/app.log invisible.
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (photo.jpg)", len(discovered))
	}
	rootLogFound := false
	subLogFound := false
	for _, sf := range skipped {
		if sf.Path == "app.log" {
			rootLogFound = true
		}
		if sf.Path == filepath.Join("sub", "app.log") {
			subLogFound = true
		}
	}
	if !rootLogFound {
		t.Error("expected root app.log in skipped (unsupported format, not in scope)")
	}
	if subLogFound {
		t.Error("sub/app.log should be invisible (matched sub/.pixeignore pattern *.log)")
	}
}

// ---------------------------------------------------------------------------
// Uppercase extension tests
// ---------------------------------------------------------------------------

// TestRegistry_uppercaseExtension_detected verifies that the registry's
// fast-path lookup is case-insensitive: a file whose extension is uppercase
// (e.g. ".JPG") is matched to the same handler as its lowercase counterpart.
// This is the unit-level proof that strings.ToLower in Detect covers all
// registered formats.
func TestRegistry_uppercaseExtension_detected(t *testing.T) {
	// Each case pairs an uppercase extension with the magic bytes that the
	// corresponding handler expects, so the magic-byte verification also passes.
	cases := []struct {
		name      string
		upperExt  string
		magic     []domain.MagicSignature
		fileBytes []byte
	}{
		{
			name:      "JPG",
			upperExt:  ".JPG",
			magic:     jpegMagic,
			fileBytes: append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...),
		},
		{
			name:      "JPEG",
			upperExt:  ".JPEG",
			magic:     jpegMagic,
			fileBytes: append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...),
		},
		{
			// TIFF LE magic — used by DNG, NEF, PEF, ARW.
			name:     "DNG",
			upperExt: ".DNG",
			magic: []domain.MagicSignature{
				{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}},
			},
			fileBytes: []byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name:     "NEF",
			upperExt: ".NEF",
			magic: []domain.MagicSignature{
				{Offset: 0, Bytes: []byte{0x49, 0x49, 0x2A, 0x00}},
			},
			fileBytes: []byte{0x49, 0x49, 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			// MP4/MOV: ftyp box magic.
			name:     "MP4",
			upperExt: ".MP4",
			magic: []domain.MagicSignature{
				{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
			},
			fileBytes: []byte{0x00, 0x00, 0x00, 0x18,
				0x66, 0x74, 0x79, 0x70, // "ftyp"
				0x6D, 0x70, 0x34, 0x32, // brand "mp42"
				0x00, 0x00, 0x00, 0x00},
		},
		{
			name:     "MOV",
			upperExt: ".MOV",
			magic: []domain.MagicSignature{
				{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}},
			},
			fileBytes: []byte{0x00, 0x00, 0x00, 0x18,
				0x66, 0x74, 0x79, 0x70,
				0x71, 0x74, 0x20, 0x20, // brand "qt  "
				0x00, 0x00, 0x00, 0x00},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reg := NewRegistry()
			h := &mockHandler{
				exts:  []string{strings.ToLower(tc.upperExt)},
				magic: tc.magic,
				name:  tc.name,
			}
			reg.Register(h)

			dir := t.TempDir()
			f := filepath.Join(dir, "photo"+tc.upperExt)
			writeFile(t, f, tc.fileBytes)

			got, err := reg.Detect(f)
			if err != nil {
				t.Fatalf("Detect(%q): %v", tc.upperExt, err)
			}
			if got != h {
				t.Errorf("Detect(%q): got %v, want handler %q", tc.upperExt, got, tc.name)
			}
		})
	}
}

// TestWalk_uppercaseExtensionDiscovered verifies that Walk classifies a file
// whose extension is uppercase (e.g. "photo.JPG") as a DiscoveredFile, not a
// SkippedFile. This exercises the full extension-normalisation path through
// Walk → Registry.Detect.
func TestWalk_uppercaseExtensionDiscovered(t *testing.T) {
	dir := t.TempDir()

	// Write a file with uppercase .JPG extension and valid JPEG magic bytes.
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)
	writeFile(t, filepath.Join(dir, "photo.JPG"), jpegBytes)

	reg := NewRegistry()
	// Register the handler with the lowercase extension only — the registry
	// must normalise the file's extension before lookup.
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: false})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (photo.JPG should be classified)", len(discovered))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped: got %d, want 0 (photo.JPG must not be skipped)", len(skipped))
	}
	if len(discovered) == 1 && discovered[0].RelPath != "photo.JPG" {
		t.Errorf("RelPath = %q, want %q", discovered[0].RelPath, "photo.JPG")
	}
}

// TestWalk_mixedCaseExtensions verifies that a directory containing files with
// both lowercase and uppercase extensions of the same format are all discovered.
func TestWalk_mixedCaseExtensions(t *testing.T) {
	dir := t.TempDir()

	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)
	writeFile(t, filepath.Join(dir, "lower.jpg"), jpegBytes)
	writeFile(t, filepath.Join(dir, "upper.JPG"), jpegBytes)
	writeFile(t, filepath.Join(dir, "mixed.Jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: false})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(discovered) != 3 {
		t.Errorf("discovered: got %d, want 3 (lower.jpg, upper.JPG, mixed.Jpg)", len(discovered))
	}
	if len(skipped) != 0 {
		t.Errorf("skipped: got %d, want 0", len(skipped))
	}
}

// TestWalk_pixeignoreFileItself verifies that .pixeignore files themselves do
// not appear in either discovered or skipped slices.
func TestWalk_pixeignoreFileItself(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	writeFile(t, filepath.Join(dir, ".pixeignore"), []byte("*.txt\n"))
	writeFile(t, filepath.Join(dir, "sub", ".pixeignore"), []byte("*.log\n"))
	writeFile(t, filepath.Join(dir, "photo.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	m := newIgnoreMatcher(t, nil)
	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true, Ignore: m})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	for _, df := range discovered {
		if filepath.Base(df.RelPath) == ".pixeignore" {
			t.Errorf(".pixeignore should not appear in discovered: %q", df.RelPath)
		}
	}
	for _, sf := range skipped {
		if filepath.Base(sf.Path) == ".pixeignore" {
			t.Errorf(".pixeignore should not appear in skipped: %q", sf.Path)
		}
	}
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (photo.jpg only)", len(discovered))
	}
}

// ---------------------------------------------------------------------------
// Edge-case tests: §16.4.6 Discovery-level edge cases
// ---------------------------------------------------------------------------

// TestWalk_symlinkToFile verifies that a symlink to a media file is recorded
// in the skipped list with reason "symlink" (Walk does not follow symlinks).
func TestWalk_symlinkToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}

	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	// Create a real JPEG in a separate temp dir.
	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "real.jpg")
	writeFile(t, targetPath, jpegBytes)

	// Create a symlink inside dirA pointing to the real file.
	linkPath := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: false})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Symlinks are skipped, not discovered.
	if len(discovered) != 0 {
		t.Errorf("discovered: got %d, want 0 (symlinks are skipped)", len(discovered))
	}
	if len(skipped) != 1 {
		t.Errorf("skipped: got %d, want 1", len(skipped))
	} else if skipped[0].Reason != "symlink" {
		t.Errorf("skipped reason = %q, want %q", skipped[0].Reason, "symlink")
	}
}

// TestWalk_symlinkToDir verifies that a symlink to a directory is not
// descended into (filepath.WalkDir does not follow directory symlinks).
func TestWalk_symlinkToDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}

	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	// Create a real subdirectory with a JPEG.
	realSubDir := t.TempDir()
	writeFile(t, filepath.Join(realSubDir, "photo.jpg"), jpegBytes)

	// Create a symlink inside dirA pointing to the real subdirectory.
	linkPath := filepath.Join(dir, "sublink")
	if err := os.Symlink(realSubDir, linkPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	// In recursive mode, the symlinked directory is seen as a symlink entry
	// by WalkDir and is skipped (WalkDir does not follow directory symlinks).
	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// The symlink itself is recorded as skipped; the files inside are not seen.
	if len(discovered) != 0 {
		t.Errorf("discovered: got %d, want 0 (symlinked dir not followed)", len(discovered))
	}
	// The symlink directory entry appears in skipped with reason "symlink".
	foundSymlink := false
	for _, sf := range skipped {
		if sf.Reason == "symlink" {
			foundSymlink = true
		}
	}
	if !foundSymlink {
		t.Errorf("expected a skipped entry with reason 'symlink', got: %v", skipped)
	}
}

// TestWalk_unreadableFile verifies that a media file with no read permissions
// is skipped with a detection error, and the walk continues to other files.
func TestWalk_unreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission model differs on Windows")
	}

	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	// A readable JPEG.
	writeFile(t, filepath.Join(dir, "readable.jpg"), jpegBytes)

	// An unreadable JPEG (0000 permissions).
	unreadablePath := filepath.Join(dir, "unreadable.jpg")
	writeFile(t, unreadablePath, jpegBytes)
	if err := os.Chmod(unreadablePath, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadablePath, 0o644) })

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, skipped, err := Walk(dir, reg, WalkOptions{Recursive: false})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// The readable file is discovered; the unreadable one is skipped.
	if len(discovered) != 1 {
		t.Errorf("discovered: got %d, want 1 (readable.jpg)", len(discovered))
	}
	if len(skipped) != 1 {
		t.Errorf("skipped: got %d, want 1 (unreadable.jpg)", len(skipped))
	}
}

// TestWalk_unreadableDir verifies that a subdirectory with no read permissions
// causes Walk to return an error (filepath.WalkDir propagates the error).
func TestWalk_unreadableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission model differs on Windows")
	}

	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	// A readable file in the root.
	writeFile(t, filepath.Join(dir, "photo.jpg"), jpegBytes)

	// An unreadable subdirectory.
	subDir := filepath.Join(dir, "locked")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, filepath.Join(subDir, "hidden.jpg"), jpegBytes)
	if err := os.Chmod(subDir, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(subDir, 0o755) })

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	// Walk returns an error when it cannot read the locked directory.
	_, _, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err == nil {
		t.Error("Walk should return an error for an unreadable directory")
	}
}

// TestWalk_unicodeDirNames verifies that files inside directories with Unicode
// names are discovered with the correct RelPath.
func TestWalk_unicodeDirNames(t *testing.T) {
	dir := t.TempDir()
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 12)...)

	// Create a file inside a Unicode-named subdirectory.
	unicodeSubDir := filepath.Join(dir, "日本旅行")
	if err := os.MkdirAll(unicodeSubDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, filepath.Join(unicodeSubDir, "IMG_0001.jpg"), jpegBytes)

	reg := NewRegistry()
	reg.Register(&mockHandler{exts: []string{".jpg"}, magic: jpegMagic, name: "jpeg"})

	discovered, _, err := Walk(dir, reg, WalkOptions{Recursive: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(discovered) != 1 {
		t.Fatalf("discovered: got %d, want 1", len(discovered))
	}

	wantRel := filepath.Join("日本旅行", "IMG_0001.jpg")
	if discovered[0].RelPath != wantRel {
		t.Errorf("RelPath = %q, want %q", discovered[0].RelPath, wantRel)
	}
}
