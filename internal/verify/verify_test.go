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

package verify

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // SHA-1 used for filename checksums, not security
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/hash"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// writeTestFile creates a file at dir/relPath with the given content.
// relPath may include subdirectories (e.g., "2024/07-Jul/filename.jpg").
// Returns the absolute path of the created file.
func writeTestFile(t *testing.T, dir, relPath string, content []byte) string {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("writeTestFile MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		t.Fatalf("writeTestFile WriteFile: %v", err)
	}
	return full
}

// sha1Hex returns the lowercase hex SHA-1 of data.
func sha1Hex(data []byte) string {
	h := sha1.New() //nolint:gosec
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// pixeFilename builds a Pixe-format filename: YYYYMMDD_HHMMSS_<checksum>.<ext>.
func pixeFilename(checksum, ext string) string {
	return "20211225_062223_" + checksum + ext
}

// buildJPEGContent creates a minimal JPEG file with the given data appended.
// Returns the full JPEG content with magic bytes prepended.
func buildJPEGContent(data []byte) []byte {
	// JPEG magic bytes: SOI (0xFF 0xD8) + APP0 marker (0xFF 0xE0) + length + JFIF identifier
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	return append(jpegHeader, data...)
}

// stubHandler is a minimal FileTypeHandler that claims a configurable set of
// extensions and returns the full file content from HashableReader.
// It is used in verify tests to avoid depending on real format parsers.
type stubHandler struct {
	exts []string
}

func (s *stubHandler) Extensions() []string { return s.exts }
func (s *stubHandler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{{Offset: 0, Bytes: []byte{0xFF, 0xD8, 0xFF}}}
}
func (s *stubHandler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, e := range s.exts {
		if ext == e {
			return true, nil
		}
	}
	return false, nil
}
func (s *stubHandler) ExtractDate(_ string) (time.Time, error) {
	return time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC), nil
}
func (s *stubHandler) HashableReader(filePath string) (io.ReadCloser, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (s *stubHandler) MetadataSupport() domain.MetadataCapability              { return domain.MetadataNone }
func (s *stubHandler) WriteMetadataTags(_ string, _ domain.MetadataTags) error { return nil }

// newTestRegistry returns a Registry with a stub handler registered for .jpg.
func newTestRegistry() *discovery.Registry {
	reg := discovery.NewRegistry()
	reg.Register(&stubHandler{exts: []string{".jpg"}})
	return reg
}

// newTestHasher returns a SHA-1 hasher (same algorithm used to build test filenames).
func newTestHasher(t *testing.T) *hash.Hasher {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	return h
}

// ---------------------------------------------------------------------------
// Run() tests
// ---------------------------------------------------------------------------

// TestRun_allFilesVerified verifies the happy path: all files have correct
// checksums embedded in their filenames.
func TestRun_allFilesVerified(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// Create 3 files with correct checksums in their names.
	contents := [][]byte{
		[]byte("file one content"),
		[]byte("file two content"),
		[]byte("file three content"),
	}
	for _, content := range contents {
		jpegContent := buildJPEGContent(content)
		checksum := sha1Hex(jpegContent)
		name := pixeFilename(checksum, ".jpg")
		writeTestFile(t, dir, name, jpegContent)
	}

	var out bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Verified != 3 {
		t.Errorf("Verified = %d, want 3", result.Verified)
	}
	if result.Mismatches != 0 {
		t.Errorf("Mismatches = %d, want 0", result.Mismatches)
	}
	if result.Unrecognised != 0 {
		t.Errorf("Unrecognised = %d, want 0", result.Unrecognised)
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("output should contain OK; got:\n%s", out.String())
	}
}

// TestRun_mismatchDetected verifies that a file whose content doesn't match
// the checksum in its filename is reported as a mismatch.
func TestRun_mismatchDetected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// Embed a checksum that does NOT match the actual file content.
	fakeChecksum := strings.Repeat("a", 40) // 40 hex chars (SHA-1 length)
	name := pixeFilename(fakeChecksum, ".jpg")
	jpegContent := buildJPEGContent([]byte("different content"))
	writeTestFile(t, dir, name, jpegContent)

	var out bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Mismatches != 1 {
		t.Errorf("Mismatches = %d, want 1", result.Mismatches)
	}
	if result.Verified != 0 {
		t.Errorf("Verified = %d, want 0", result.Verified)
	}
	if !strings.Contains(out.String(), "MISMATCH") {
		t.Errorf("output should contain MISMATCH; got:\n%s", out.String())
	}
}

// TestRun_unrecognisedFile verifies that a file with a valid Pixe filename but
// an extension not claimed by any handler is reported as unrecognised.
func TestRun_unrecognisedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry() // only claims .jpg
	hasher := newTestHasher(t)

	// .xyz is not registered in the registry.
	checksum := strings.Repeat("b", 40)
	name := pixeFilename(checksum, ".xyz")
	writeTestFile(t, dir, name, []byte("some content"))

	var out bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Unrecognised != 1 {
		t.Errorf("Unrecognised = %d, want 1", result.Unrecognised)
	}
	if result.Verified != 0 {
		t.Errorf("Verified = %d, want 0", result.Verified)
	}
	if !strings.Contains(out.String(), "UNRECOGNISED") {
		t.Errorf("output should contain UNRECOGNISED; got:\n%s", out.String())
	}
}

// TestRun_unparsableFilename verifies that a file whose name doesn't match the
// Pixe format (YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>) is reported as unrecognised.
func TestRun_unparsableFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// "random_photo.jpg" has only 2 underscore-separated parts — parseChecksum returns false.
	writeTestFile(t, dir, "random_photo.jpg", []byte("some content"))

	var out bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Unrecognised != 1 {
		t.Errorf("Unrecognised = %d, want 1", result.Unrecognised)
	}
	if !strings.Contains(out.String(), "UNRECOGNISED") {
		t.Errorf("output should contain UNRECOGNISED; got:\n%s", out.String())
	}
}

// TestRun_dotfilesSkipped verifies that dotfiles and dot-directories are
// excluded from verification and do not appear in the result counts.
func TestRun_dotfilesSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// Dotfile at root level.
	writeTestFile(t, dir, ".DS_Store", []byte("mac junk"))

	// Dot-directory with a file inside.
	writeTestFile(t, dir, filepath.Join(".pixe", "manifest.json"), []byte("{}"))

	// One real file with a correct checksum — should be the only one counted.
	content := []byte("real file")
	jpegContent := buildJPEGContent(content)
	checksum := sha1Hex(jpegContent)
	name := pixeFilename(checksum, ".jpg")
	writeTestFile(t, dir, name, jpegContent)

	var out bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Verified != 1 {
		t.Errorf("Verified = %d, want 1 (dotfiles must be skipped)", result.Verified)
	}
	if result.Mismatches != 0 || result.Unrecognised != 0 {
		t.Errorf("unexpected counts: mismatches=%d unrecognised=%d", result.Mismatches, result.Unrecognised)
	}
	// Dotfile names must not appear in output.
	if strings.Contains(out.String(), ".DS_Store") {
		t.Error("output should not mention .DS_Store")
	}
}

// TestRun_emptyDirectory verifies that an empty directory produces zero counts
// and no error.
func TestRun_emptyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	var out bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result != (Result{}) {
		t.Errorf("empty dir: result = %+v, want zero Result", result)
	}
}

// TestRun_subdirectoryWalked verifies that files in subdirectories are
// included in the verification walk.
func TestRun_subdirectoryWalked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// File in a subdirectory with correct checksum.
	content := []byte("nested file content")
	jpegContent := buildJPEGContent(content)
	checksum := sha1Hex(jpegContent)
	name := pixeFilename(checksum, ".jpg")
	writeTestFile(t, dir, filepath.Join("2024", "07-Jul", name), jpegContent)

	var out bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Verified != 1 {
		t.Errorf("Verified = %d, want 1", result.Verified)
	}
}

// ---------------------------------------------------------------------------
// parseChecksum() tests
// ---------------------------------------------------------------------------

// TestParseChecksum_validFormats verifies that well-formed Pixe filenames
// yield the correct checksum string.
func TestParseChecksum_validFormats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "SHA-1 40-char checksum",
			filename: "20211225_062223_7d97e98f1234567890abcdef12345678abcdef12.jpg",
			want:     "7d97e98f1234567890abcdef12345678abcdef12",
		},
		{
			name:     "16-char checksum above minimum",
			filename: "20220202_123101_abcdef0123456789.heic",
			want:     "abcdef0123456789",
		},
		{
			name:     "exactly 8 chars (minimum)",
			filename: "19020220_000000_12345678.dng",
			want:     "12345678",
		},
		{
			name:     "SHA-256 64-char checksum",
			filename: "20260312_120000_" + strings.Repeat("a", 64) + ".arw",
			want:     strings.Repeat("a", 64),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseChecksum(tc.filename)
			if !ok {
				t.Fatalf("parseChecksum(%q) = (_, false), want true", tc.filename)
			}
			if got != tc.want {
				t.Errorf("parseChecksum(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

// TestParseChecksum_invalidFormats verifies that malformed filenames return
// ("", false).
func TestParseChecksum_invalidFormats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		filename string
	}{
		{
			name:     "only 2 underscore parts",
			filename: "random_photo.jpg",
		},
		{
			name:     "checksum too short (< 8 chars)",
			filename: "20211225_062223_short.jpg",
		},
		{
			name:     "no extension",
			filename: "noextension",
		},
		{
			name:     "only 2 parts after split",
			filename: "20211225_062223.jpg",
		},
		{
			name:     "empty string",
			filename: "",
		},
		{
			name:     "path traversal attempt",
			filename: "../../etc/passwd",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseChecksum(tc.filename)
			if ok {
				t.Errorf("parseChecksum(%q) = (%q, true), want (_, false)", tc.filename, got)
			}
			if got != "" {
				t.Errorf("parseChecksum(%q) returned non-empty checksum %q on failure", tc.filename, got)
			}
		})
	}
}

// TestParseChecksum_nonHexAccepted documents the known behaviour that
// parseChecksum does not validate hex encoding — non-hex characters in the
// checksum position are accepted. This is intentional: the checksum is
// compared against the recomputed hash, so invalid hex will simply never match.
func TestParseChecksum_nonHexAccepted(t *testing.T) {
	t.Parallel()
	// "ghijklmn" is 8 chars but not valid hex — parseChecksum accepts it.
	got, ok := parseChecksum("20211225_062223_ghijklmn.jpg")
	if !ok {
		t.Fatal("parseChecksum should accept non-hex checksum (length check only)")
	}
	if got != "ghijklmn" {
		t.Errorf("parseChecksum = %q, want %q", got, "ghijklmn")
	}
}

// TestParseChecksum_nullByteInChecksum documents that a null byte embedded in
// the checksum position is accepted by parseChecksum (length check only).
// The checksum will never match a recomputed hash, so it is harmless.
func TestParseChecksum_nullByteInChecksum(t *testing.T) {
	t.Parallel()
	// "\x00checksum" is 9 chars — passes the ≥8 minimum.
	got, ok := parseChecksum("20211225_062223_\x00checksum.jpg")
	if !ok {
		t.Fatal("parseChecksum should accept null-byte checksum (length check only)")
	}
	if got != "\x00checksum" {
		t.Errorf("parseChecksum = %q, want %q", got, "\x00checksum")
	}
}

// TestParseChecksum_longChecksumAccepted documents that very long checksums
// are accepted — the length check has no upper bound.
func TestParseChecksum_longChecksumAccepted(t *testing.T) {
	t.Parallel()
	longChecksum := strings.Repeat("a", 1000)
	filename := "20211225_062223_" + longChecksum + ".jpg"
	got, ok := parseChecksum(filename)
	if !ok {
		t.Fatal("parseChecksum should accept long checksum")
	}
	if got != longChecksum {
		t.Errorf("parseChecksum returned %q, want %q", got, longChecksum)
	}
}
