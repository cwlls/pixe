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

package copy

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/hash"
)

// --- helpers ---

// writeSource creates a temp file with the given content and returns its path.
func writeSource(t *testing.T, dir string, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("writeSource: %v", err)
	}
	return p
}

// stubHandler is a test double whose HashableReader returns the full file
// contents (no metadata stripping needed for these unit tests).
type stubHandler struct{}

func (s *stubHandler) Extensions() []string                                { return nil }
func (s *stubHandler) MagicBytes() []domain.MagicSignature                 { return nil }
func (s *stubHandler) Detect(string) (bool, error)                         { return true, nil }
func (s *stubHandler) ExtractDate(string) (time.Time, error)               { return time.Time{}, nil }
func (s *stubHandler) MetadataSupport() domain.MetadataCapability          { return domain.MetadataNone }
func (s *stubHandler) WriteMetadataTags(string, domain.MetadataTags) error { return nil }
func (s *stubHandler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	return f, err
}

// sha1Hasher returns a SHA-1 Hasher for tests.
func sha1Hasher(t *testing.T) *hash.Hasher {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	return h
}

// --- Execute tests ---

func TestExecute_copiesContent(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("pixe copy engine test content")
	srcFile := writeSource(t, src, "photo.jpg", content)
	destFile := filepath.Join(dst, "photo.jpg")

	if err := Execute(srcFile, destFile); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("destination content mismatch:\n  got  %q\n  want %q", got, content)
	}
}

func TestExecute_createsParentDirectories(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("nested dir test")
	srcFile := writeSource(t, src, "photo.jpg", content)
	// Destination is several levels deep — none of these directories exist yet.
	destFile := filepath.Join(dst, "2021", "12", "photo.jpg")

	if err := Execute(srcFile, destFile); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(destFile); err != nil {
		t.Errorf("destination not found after Execute: %v", err)
	}
}

func TestExecute_setsPermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	srcFile := writeSource(t, src, "photo.jpg", []byte("perm test"))
	destFile := filepath.Join(dst, "photo.jpg")

	if err := Execute(srcFile, destFile); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	info, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	// Mask to permission bits only.
	perm := info.Mode().Perm()
	if perm != 0o644 {
		t.Errorf("destination permissions = %o, want 0644", perm)
	}
}

func TestExecute_preservesMtime(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	srcFile := writeSource(t, src, "photo.jpg", []byte("mtime test"))
	// Set a known mtime on the source.
	knownTime := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	if err := os.Chtimes(srcFile, knownTime, knownTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	destFile := filepath.Join(dst, "photo.jpg")
	if err := Execute(srcFile, destFile); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	info, err := os.Stat(destFile)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	// Allow 1-second tolerance for filesystem mtime precision.
	diff := info.ModTime().UTC().Sub(knownTime)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("destination mtime = %v, want ~%v (diff %v)", info.ModTime().UTC(), knownTime, diff)
	}
}

func TestExecute_largeFile(t *testing.T) {
	// Verify streaming works for a file larger than copyBufSize (32 KB).
	src := t.TempDir()
	dst := t.TempDir()

	content := bytes.Repeat([]byte("abcdefghij"), 10_000) // 100 KB
	srcFile := writeSource(t, src, "large.jpg", content)
	destFile := filepath.Join(dst, "large.jpg")

	if err := Execute(srcFile, destFile); err != nil {
		t.Fatalf("Execute large file: %v", err)
	}

	got, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("large file content mismatch (lengths: got %d, want %d)", len(got), len(content))
	}
}

func TestExecute_missingSource(t *testing.T) {
	dst := t.TempDir()
	err := Execute("/nonexistent/path/photo.jpg", filepath.Join(dst, "photo.jpg"))
	if err == nil {
		t.Error("Execute with missing source should return error")
	}
}

// --- Verify tests ---

func TestVerify_success(t *testing.T) {
	dir := t.TempDir()
	content := []byte("verify success test")
	destFile := writeSource(t, dir, "photo.jpg", content)

	h := sha1Hasher(t)
	handler := &stubHandler{}

	// Compute expected checksum.
	expected, err := h.Sum(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("compute expected: %v", err)
	}

	result := Verify(destFile, expected, handler, h)
	if !result.Success {
		t.Errorf("Verify should succeed: %v", result.Error)
	}
	if result.Checksum != expected {
		t.Errorf("Verify checksum = %q, want %q", result.Checksum, expected)
	}
	if result.Error != nil {
		t.Errorf("Verify error should be nil, got: %v", result.Error)
	}
}

func TestVerify_mismatch_filePreserved(t *testing.T) {
	dir := t.TempDir()
	content := []byte("original content")
	destFile := writeSource(t, dir, "photo.jpg", content)

	h := sha1Hasher(t)
	handler := &stubHandler{}

	// Pass a wrong expected checksum.
	wrongChecksum := "0000000000000000000000000000000000000000"

	result := Verify(destFile, wrongChecksum, handler, h)
	if result.Success {
		t.Error("Verify should fail on checksum mismatch")
	}
	if result.Error == nil {
		t.Error("Verify should return error on mismatch")
	}
	// Destination file must still exist (preserved for debugging).
	if _, err := os.Stat(destFile); err != nil {
		t.Errorf("destination file should be preserved on mismatch, got: %v", err)
	}
	// Actual checksum should be populated even on mismatch.
	if result.Checksum == "" {
		t.Error("Verify should populate Checksum even on mismatch")
	}
	if result.Checksum == wrongChecksum {
		t.Error("Verify Checksum should be the actual hash, not the expected one")
	}
}

func TestVerify_missingDestination(t *testing.T) {
	h := sha1Hasher(t)
	handler := &stubHandler{}

	result := Verify("/nonexistent/photo.jpg", "abc", handler, h)
	if result.Success {
		t.Error("Verify on missing file should not succeed")
	}
	if result.Error == nil {
		t.Error("Verify on missing file should return error")
	}
}

// TestExecuteAndVerify_roundtrip is an end-to-end test: copy a file then
// verify it, asserting the full pipeline succeeds.
func TestExecuteAndVerify_roundtrip(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("end-to-end copy+verify test payload")
	srcFile := writeSource(t, src, "photo.jpg", content)
	destFile := filepath.Join(dst, "2021", "12", "photo.jpg")

	// Step 1: copy.
	if err := Execute(srcFile, destFile); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Step 2: compute expected checksum from source.
	h := sha1Hasher(t)
	handler := &stubHandler{}
	expected, err := h.Sum(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("compute expected: %v", err)
	}

	// Step 3: verify.
	result := Verify(destFile, expected, handler, h)
	if !result.Success {
		t.Fatalf("Verify after Execute failed: %v", result.Error)
	}
}
