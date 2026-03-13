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
	"strings"
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/hash"
)

// --- helpers ---

// writeSource creates a file with the given content and returns its path.
func writeSource(t *testing.T, dir, name string, content []byte) string {
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

// computeChecksum returns the SHA-1 hex digest of content.
func computeChecksum(t *testing.T, content []byte) string {
	t.Helper()
	h := sha1Hasher(t)
	sum, err := h.Sum(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("computeChecksum: %v", err)
	}
	return sum
}

// --- TempPath tests ---

func TestTempPath_format(t *testing.T) {
	cases := []struct {
		dest string
		want string
	}{
		{
			dest: "/archive/2021/12-Dec/20211225_062223_abc123.jpg",
			want: "/archive/2021/12-Dec/.20211225_062223_abc123.jpg.pixe-tmp",
		},
		{
			dest: "relative/path/photo.jpg",
			want: "relative/path/.photo.jpg.pixe-tmp",
		},
		{
			dest: "/flat/photo.cr3",
			want: "/flat/.photo.cr3.pixe-tmp",
		},
	}

	for _, tc := range cases {
		got := TempPath(tc.dest)
		if got != tc.want {
			t.Errorf("TempPath(%q) = %q, want %q", tc.dest, got, tc.want)
		}
	}
}

func TestTempPath_leadingDotAndSuffix(t *testing.T) {
	dest := "/some/dir/myfile.jpg"
	tmp := TempPath(dest)

	base := filepath.Base(tmp)
	if !strings.HasPrefix(base, ".") {
		t.Errorf("TempPath base %q should start with '.'", base)
	}
	if !strings.HasSuffix(base, ".pixe-tmp") {
		t.Errorf("TempPath base %q should end with '.pixe-tmp'", base)
	}
	if filepath.Dir(tmp) != filepath.Dir(dest) {
		t.Errorf("TempPath dir %q != dest dir %q — must be same dir for atomic rename",
			filepath.Dir(tmp), filepath.Dir(dest))
	}
}

// --- Execute tests ---

func TestExecute_writesToTempFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("pixe atomic copy test")
	srcFile := writeSource(t, src, "photo.jpg", content)
	destFile := filepath.Join(dst, "photo.jpg")

	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Temp file must exist at the returned path.
	if _, err := os.Stat(tmpPath); err != nil {
		t.Errorf("temp file not found at %q: %v", tmpPath, err)
	}

	// Temp file must be in the same directory as dest (required for atomic rename).
	if filepath.Dir(tmpPath) != filepath.Dir(destFile) {
		t.Errorf("temp file dir %q != dest dir %q — must be same dir for atomic rename",
			filepath.Dir(tmpPath), filepath.Dir(destFile))
	}

	// Temp file name must start with "." and contain ".pixe-tmp".
	base := filepath.Base(tmpPath)
	if !strings.HasPrefix(base, ".") {
		t.Errorf("temp file base %q should start with '.'", base)
	}
	if !strings.Contains(base, ".pixe-tmp") {
		t.Errorf("temp file base %q should contain '.pixe-tmp'", base)
	}

	// Final destination must NOT exist yet.
	if _, err := os.Stat(destFile); err == nil {
		t.Errorf("final destination %q should not exist before Promote", destFile)
	}
}

func TestExecute_tempFileContent(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("content integrity check")
	srcFile := writeSource(t, src, "photo.jpg", content)
	destFile := filepath.Join(dst, "photo.jpg")

	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("temp file content mismatch:\n  got  %q\n  want %q", got, content)
	}
}

func TestExecute_createsParentDirectories(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("nested dir test")
	srcFile := writeSource(t, src, "photo.jpg", content)
	// Destination is several levels deep — none of these directories exist yet.
	destFile := filepath.Join(dst, "2021", "12-Dec", "photo.jpg")

	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(tmpPath); err != nil {
		t.Errorf("temp file not found after Execute: %v", err)
	}
}

func TestExecute_setsPermissions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	srcFile := writeSource(t, src, "photo.jpg", []byte("perm test"))
	destFile := filepath.Join(dst, "photo.jpg")

	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		t.Fatalf("stat temp file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("temp file permissions = %o, want 0644", perm)
	}
}

func TestExecute_preservesMtime(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	srcFile := writeSource(t, src, "photo.jpg", []byte("mtime test"))
	knownTime := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	if err := os.Chtimes(srcFile, knownTime, knownTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	destFile := filepath.Join(dst, "photo.jpg")
	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		t.Fatalf("stat temp file: %v", err)
	}
	diff := info.ModTime().UTC().Sub(knownTime)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("temp file mtime = %v, want ~%v (diff %v)", info.ModTime().UTC(), knownTime, diff)
	}
}

func TestExecute_largeFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	content := bytes.Repeat([]byte("abcdefghij"), 10_000) // 100 KB > copyBufSize
	srcFile := writeSource(t, src, "large.jpg", content)
	destFile := filepath.Join(dst, "large.jpg")

	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute large file: %v", err)
	}

	got, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("read temp file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("large file content mismatch (lengths: got %d, want %d)", len(got), len(content))
	}
}

func TestExecute_missingSource(t *testing.T) {
	dst := t.TempDir()
	_, err := Execute("/nonexistent/path/photo.jpg", filepath.Join(dst, "photo.jpg"))
	if err == nil {
		t.Error("Execute with missing source should return error")
	}
}

// --- Verify tests ---

func TestVerify_success(t *testing.T) {
	dir := t.TempDir()
	content := []byte("verify success test")
	tmpFile := writeSource(t, dir, ".photo.jpg.pixe-tmp", content)

	h := sha1Hasher(t)
	expected := computeChecksum(t, content)

	result := Verify(tmpFile, expected, &stubHandler{}, h)
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

func TestVerify_mismatch(t *testing.T) {
	dir := t.TempDir()
	content := []byte("original content")
	tmpFile := writeSource(t, dir, ".photo.jpg.pixe-tmp", content)

	h := sha1Hasher(t)
	wrongChecksum := "0000000000000000000000000000000000000000"

	result := Verify(tmpFile, wrongChecksum, &stubHandler{}, h)
	if result.Success {
		t.Error("Verify should fail on checksum mismatch")
	}
	if result.Error == nil {
		t.Error("Verify should return error on mismatch")
	}
	// Actual checksum should be populated even on mismatch.
	if result.Checksum == "" {
		t.Error("Verify should populate Checksum even on mismatch")
	}
	if result.Checksum == wrongChecksum {
		t.Error("Verify Checksum should be the actual hash, not the expected one")
	}
	// Verify itself does NOT delete the temp file — that is CleanupTempFile's job.
	if _, err := os.Stat(tmpFile); err != nil {
		t.Errorf("Verify should not delete temp file on mismatch: %v", err)
	}
}

func TestVerify_missingFile(t *testing.T) {
	h := sha1Hasher(t)
	result := Verify("/nonexistent/.photo.jpg.pixe-tmp", "abc", &stubHandler{}, h)
	if result.Success {
		t.Error("Verify on missing file should not succeed")
	}
	if result.Error == nil {
		t.Error("Verify on missing file should return error")
	}
}

// --- Promote tests ---

func TestPromote_atomicRename(t *testing.T) {
	dir := t.TempDir()
	content := []byte("promote test")
	tmpFile := writeSource(t, dir, ".photo.jpg.pixe-tmp", content)
	destFile := filepath.Join(dir, "photo.jpg")

	if err := Promote(tmpFile, destFile); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	// Final destination must exist with correct content.
	got, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("read dest after Promote: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("dest content after Promote mismatch")
	}

	// Temp file must no longer exist.
	if _, err := os.Stat(tmpFile); err == nil {
		t.Errorf("temp file %q should not exist after Promote", tmpFile)
	}
}

func TestPromote_parentDirExists(t *testing.T) {
	// Simulate the real case: Execute created the parent dir, temp file is there,
	// Promote renames into the same dir.
	dst := t.TempDir()
	destDir := filepath.Join(dst, "2021", "12-Dec")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	destFile := filepath.Join(destDir, "20211225_062223_abc.jpg")
	tmpFile := TempPath(destFile)

	content := []byte("nested promote test")
	if err := os.WriteFile(tmpFile, content, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := Promote(tmpFile, destFile); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	if _, err := os.Stat(destFile); err != nil {
		t.Errorf("dest file not found after Promote: %v", err)
	}
}

func TestPromote_missingTempFile(t *testing.T) {
	dir := t.TempDir()
	err := Promote(filepath.Join(dir, ".missing.pixe-tmp"), filepath.Join(dir, "dest.jpg"))
	if err == nil {
		t.Error("Promote with missing temp file should return error")
	}
}

// --- CleanupTempFile tests ---

func TestCleanupTempFile_removesFile(t *testing.T) {
	dir := t.TempDir()
	tmpFile := writeSource(t, dir, ".photo.jpg.pixe-tmp", []byte("cleanup test"))

	CleanupTempFile(tmpFile)

	if _, err := os.Stat(tmpFile); err == nil {
		t.Errorf("temp file %q should have been removed by CleanupTempFile", tmpFile)
	}
}

func TestCleanupTempFile_missingFile(t *testing.T) {
	// CleanupTempFile must not panic or error on a missing file.
	// (It swallows the error intentionally.)
	CleanupTempFile("/nonexistent/.photo.jpg.pixe-tmp")
}

// --- Full atomic flow tests ---

func TestAtomicFlow_success(t *testing.T) {
	// Full flow: Execute → Verify (good checksum) → Promote → file at canonical path.
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("full atomic flow success")
	srcFile := writeSource(t, src, "photo.jpg", content)
	destFile := filepath.Join(dst, "2021", "12-Dec", "photo.jpg")
	expected := computeChecksum(t, content)

	// Step 1: Execute → temp file.
	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Step 2: Verify temp file.
	result := Verify(tmpPath, expected, &stubHandler{}, sha1Hasher(t))
	if !result.Success {
		t.Fatalf("Verify: %v", result.Error)
	}

	// Step 3: Promote to canonical path.
	if err := Promote(tmpPath, destFile); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	// Canonical path must exist with correct content.
	got, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("read canonical dest: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("canonical dest content mismatch")
	}

	// Temp file must be gone.
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf("temp file should not exist after Promote")
	}
}

func TestAtomicFlow_mismatch_tempDeleted(t *testing.T) {
	// Full flow: Execute → Verify (bad checksum) → CleanupTempFile → temp gone,
	// canonical path never created.
	src := t.TempDir()
	dst := t.TempDir()

	content := []byte("full atomic flow mismatch")
	srcFile := writeSource(t, src, "photo.jpg", content)
	destFile := filepath.Join(dst, "photo.jpg")
	wrongChecksum := "0000000000000000000000000000000000000000"

	// Step 1: Execute → temp file.
	tmpPath, err := Execute(srcFile, destFile)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Step 2: Verify with wrong checksum.
	result := Verify(tmpPath, wrongChecksum, &stubHandler{}, sha1Hasher(t))
	if result.Success {
		t.Fatal("Verify should fail with wrong checksum")
	}

	// Step 3: CleanupTempFile.
	CleanupTempFile(tmpPath)

	// Temp file must be gone.
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf("temp file should be deleted after CleanupTempFile")
	}

	// Canonical path must never have been created.
	if _, err := os.Stat(destFile); err == nil {
		t.Errorf("canonical dest %q should not exist after mismatch+cleanup", destFile)
	}
}
