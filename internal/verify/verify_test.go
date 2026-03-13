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

	"github.com/cwlls/pixe/internal/discovery"
	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/progress"
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

// pixeFilename builds a legacy Pixe-format filename: YYYYMMDD_HHMMSS_<checksum>.<ext>.
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
// yield the correct checksum and algorithm.
func TestParseChecksum_validFormats(t *testing.T) {
	t.Parallel()
	sha1Checksum := "7d97e98f8af710c7e7fe703abc8f639e0ee507c4"
	sha256Checksum := strings.Repeat("a", 64)
	blake3Checksum := strings.Repeat("b", 64)
	xxhashChecksum := "a1b2c3d4e5f6a7b8"
	md5Checksum := "d41d8cd98f00b204e9800998ecf8427e"

	cases := []struct {
		name          string
		filename      string
		wantChecksum  string
		wantAlgorithm string
	}{
		// --- New format (YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>.<ext>) ---
		{
			name:          "new format SHA-1 (ID=1)",
			filename:      "20211225_062223-1-" + sha1Checksum + ".jpg",
			wantChecksum:  sha1Checksum,
			wantAlgorithm: "sha1",
		},
		{
			name:          "new format MD5 (ID=0)",
			filename:      "20211225_062223-0-" + md5Checksum + ".jpg",
			wantChecksum:  md5Checksum,
			wantAlgorithm: "md5",
		},
		{
			name:          "new format SHA-256 (ID=2)",
			filename:      "20211225_062223-2-" + sha256Checksum + ".jpg",
			wantChecksum:  sha256Checksum,
			wantAlgorithm: "sha256",
		},
		{
			name:          "new format BLAKE3 (ID=3)",
			filename:      "20211225_062223-3-" + blake3Checksum + ".jpg",
			wantChecksum:  blake3Checksum,
			wantAlgorithm: "blake3",
		},
		{
			name:          "new format xxHash (ID=4)",
			filename:      "20211225_062223-4-" + xxhashChecksum + ".jpg",
			wantChecksum:  xxhashChecksum,
			wantAlgorithm: "xxhash",
		},
		// --- Legacy format (YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>) ---
		{
			name:          "legacy SHA-1 (40 chars → sha1)",
			filename:      "20211225_062223_" + sha1Checksum + ".jpg",
			wantChecksum:  sha1Checksum,
			wantAlgorithm: "sha1",
		},
		{
			name:          "legacy SHA-256 (64 chars → sha256)",
			filename:      "20211225_062223_" + sha256Checksum + ".jpg",
			wantChecksum:  sha256Checksum,
			wantAlgorithm: "sha256",
		},
		{
			name:          "legacy ambiguous length (16 chars → empty algorithm)",
			filename:      "20220202_123101_abcdef0123456789.heic",
			wantChecksum:  "abcdef0123456789",
			wantAlgorithm: "",
		},
		{
			name:          "legacy exactly 8 chars (minimum, ambiguous)",
			filename:      "19020220_000000_12345678.dng",
			wantChecksum:  "12345678",
			wantAlgorithm: "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotChecksum, gotAlgo, ok := parseChecksum(tc.filename)
			if !ok {
				t.Fatalf("parseChecksum(%q) = (_, _, false), want true", tc.filename)
			}
			if gotChecksum != tc.wantChecksum {
				t.Errorf("parseChecksum(%q) checksum = %q, want %q", tc.filename, gotChecksum, tc.wantChecksum)
			}
			if gotAlgo != tc.wantAlgorithm {
				t.Errorf("parseChecksum(%q) algorithm = %q, want %q", tc.filename, gotAlgo, tc.wantAlgorithm)
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
			got, _, ok := parseChecksum(tc.filename)
			if ok {
				t.Errorf("parseChecksum(%q) = (%q, _, true), want (_, _, false)", tc.filename, got)
			}
			if got != "" {
				t.Errorf("parseChecksum(%q) returned non-empty checksum %q on failure", tc.filename, got)
			}
		})
	}
}

// TestParseChecksum_unknownNewFormatID verifies that a new-format filename with
// an unrecognised algorithm ID returns (_, _, false).
func TestParseChecksum_unknownNewFormatID(t *testing.T) {
	t.Parallel()
	// ID=9 is not in the registry.
	got, _, ok := parseChecksum("20211225_062223-9-abcdef0123456789.jpg")
	if ok {
		t.Errorf("parseChecksum with unknown ID = (%q, _, true), want (_, _, false)", got)
	}
}

// TestParseChecksum_nonHexAccepted documents the known behaviour that
// parseChecksum does not validate hex encoding — non-hex characters in the
// checksum position are accepted. This is intentional: the checksum is
// compared against the recomputed hash, so invalid hex will simply never match.
func TestParseChecksum_nonHexAccepted(t *testing.T) {
	t.Parallel()
	// "ghijklmn" is 8 chars but not valid hex — parseChecksum accepts it (legacy format).
	got, _, ok := parseChecksum("20211225_062223_ghijklmn.jpg")
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
	got, _, ok := parseChecksum("20211225_062223_\x00checksum.jpg")
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
	got, _, ok := parseChecksum(filename)
	if !ok {
		t.Fatal("parseChecksum should accept long checksum")
	}
	if got != longChecksum {
		t.Errorf("parseChecksum returned %q, want %q", got, longChecksum)
	}
}

// ---------------------------------------------------------------------------
// Concurrent verify tests
// ---------------------------------------------------------------------------

// TestRun_Concurrent_Correctness verifies that concurrent verification with
// Workers:4 produces correct results for known Pixe-named files.
func TestRun_Concurrent_Correctness(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// Create 5 files with correct checksums in their names.
	contents := [][]byte{
		[]byte("file one content"),
		[]byte("file two content"),
		[]byte("file three content"),
		[]byte("file four content"),
		[]byte("file five content"),
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
		Workers:  4,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Verified != 5 {
		t.Errorf("Verified = %d, want 5", result.Verified)
	}
	if result.Mismatches != 0 {
		t.Errorf("Mismatches = %d, want 0", result.Mismatches)
	}
	if result.Unrecognised != 0 {
		t.Errorf("Unrecognised = %d, want 0", result.Unrecognised)
	}
}

// TestRun_Concurrent_MatchesSequential verifies that concurrent and sequential
// verification produce identical results for the same files.
func TestRun_Concurrent_MatchesSequential(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// Create a mix of valid, mismatched, and unrecognised files.
	// Valid file.
	content1 := []byte("valid content")
	jpegContent1 := buildJPEGContent(content1)
	checksum1 := sha1Hex(jpegContent1)
	writeTestFile(t, dir, pixeFilename(checksum1, ".jpg"), jpegContent1)

	// Mismatched file.
	fakeChecksum := strings.Repeat("a", 40)
	jpegContent2 := buildJPEGContent([]byte("different content"))
	writeTestFile(t, dir, pixeFilename(fakeChecksum, ".jpg"), jpegContent2)

	// Unrecognised file (wrong extension).
	writeTestFile(t, dir, pixeFilename(strings.Repeat("b", 40), ".xyz"), []byte("some content"))

	// Run sequential.
	var outSeq bytes.Buffer
	resultSeq, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &outSeq,
		Workers:  1,
	})
	if err != nil {
		t.Fatalf("sequential Run: %v", err)
	}

	// Run concurrent.
	var outConc bytes.Buffer
	resultConc, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &outConc,
		Workers:  4,
	})
	if err != nil {
		t.Fatalf("concurrent Run: %v", err)
	}

	// Results should match.
	if resultSeq.Verified != resultConc.Verified {
		t.Errorf("Verified: sequential=%d, concurrent=%d, want equal", resultSeq.Verified, resultConc.Verified)
	}
	if resultSeq.Mismatches != resultConc.Mismatches {
		t.Errorf("Mismatches: sequential=%d, concurrent=%d, want equal", resultSeq.Mismatches, resultConc.Mismatches)
	}
	if resultSeq.Unrecognised != resultConc.Unrecognised {
		t.Errorf("Unrecognised: sequential=%d, concurrent=%d, want equal", resultSeq.Unrecognised, resultConc.Unrecognised)
	}
}

// TestRun_Concurrent_Race runs concurrent verification with -race flag.
// The test runner will detect any data races.
func TestRun_Concurrent_Race(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	// Create several files to exercise concurrent access.
	for i := 0; i < 10; i++ {
		content := []byte("file " + string(rune('0'+i)))
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
		Workers:  4,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Verified != 10 {
		t.Errorf("Verified = %d, want 10", result.Verified)
	}
}

// TestRun_Concurrent_EventEmission verifies that concurrent verification
// emits EventVerifyFileStart and terminal events with correct WorkerIDs.
func TestRun_Concurrent_EventEmission(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)
	bus := progress.NewBus(64)

	// Create 3 files.
	for i := 0; i < 3; i++ {
		content := []byte("file " + string(rune('0'+i)))
		jpegContent := buildJPEGContent(content)
		checksum := sha1Hex(jpegContent)
		name := pixeFilename(checksum, ".jpg")
		writeTestFile(t, dir, name, jpegContent)
	}

	// Run in a goroutine so we can collect events.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Run(Options{
			Dir:      dir,
			Hasher:   hasher,
			Registry: reg,
			Output:   io.Discard,
			EventBus: bus,
			Workers:  2,
		})
		bus.Close()
	}()

	// Collect events.
	var fileStartEvents []progress.Event
	var terminalEvents []progress.Event
	for e := range bus.Events() {
		if e.Kind == progress.EventVerifyFileStart {
			fileStartEvents = append(fileStartEvents, e)
		}
		if e.Kind == progress.EventVerifyOK || e.Kind == progress.EventVerifyMismatch || e.Kind == progress.EventVerifyUnrecognised {
			terminalEvents = append(terminalEvents, e)
		}
	}

	<-done

	// Should have 3 file-start events.
	if len(fileStartEvents) != 3 {
		t.Errorf("got %d EventVerifyFileStart events, want 3", len(fileStartEvents))
	}

	// All file-start events should have WorkerID > 0 (workers are 1, 2, ...).
	for _, e := range fileStartEvents {
		if e.WorkerID <= 0 {
			t.Errorf("EventVerifyFileStart has WorkerID=%d, want > 0", e.WorkerID)
		}
	}

	// Should have 3 terminal events.
	if len(terminalEvents) != 3 {
		t.Errorf("got %d terminal events, want 3", len(terminalEvents))
	}

	// All terminal events should have WorkerID > 0.
	for _, e := range terminalEvents {
		if e.WorkerID <= 0 {
			t.Errorf("terminal event has WorkerID=%d, want > 0", e.WorkerID)
		}
	}
}

// ---------------------------------------------------------------------------
// Sidecar awareness tests
// ---------------------------------------------------------------------------

// TestRun_sidecarFilesNotCountedAsUnrecognised verifies that .xmp and .aae
// sidecar files are not reported as UNRECOGNISED when they have a matching
// parent media file.
func TestRun_sidecarFilesNotCountedAsUnrecognised(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	content := buildJPEGContent([]byte("media content"))
	checksum := sha1Hex(content)
	mediaName := pixeFilename(checksum, ".jpg")
	writeTestFile(t, dir, mediaName, content)

	// Write a .xmp sidecar alongside the media file.
	writeTestFile(t, dir, mediaName+".xmp", []byte("<xmp/>"))

	var buf bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &buf,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Unrecognised != 0 {
		t.Errorf("Unrecognised = %d, want 0 (sidecar should not be counted)\noutput:\n%s", result.Unrecognised, buf.String())
	}
	if result.Verified != 1 {
		t.Errorf("Verified = %d, want 1\noutput:\n%s", result.Verified, buf.String())
	}
}

// TestRun_sidecarAnnotationOnParentLine verifies that when a media file has an
// associated sidecar, the verify output line includes the inline annotation.
func TestRun_sidecarAnnotationOnParentLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	content := buildJPEGContent([]byte("media content"))
	checksum := sha1Hex(content)
	mediaName := pixeFilename(checksum, ".jpg")
	writeTestFile(t, dir, mediaName, content)
	writeTestFile(t, dir, mediaName+".xmp", []byte("<xmp/>"))

	var buf bytes.Buffer
	_, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &buf,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[+xmp]") {
		t.Errorf("output missing [+xmp] annotation; got:\n%s", output)
	}
}

// TestRun_orphanedSidecarCountedAsUnrecognised verifies that a sidecar file
// with no matching parent media file is reported as UNRECOGNISED.
func TestRun_orphanedSidecarCountedAsUnrecognised(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	reg := newTestRegistry()
	hasher := newTestHasher(t)

	content := buildJPEGContent([]byte("media content"))
	checksum := sha1Hex(content)
	mediaName := pixeFilename(checksum, ".jpg")
	writeTestFile(t, dir, mediaName, content)

	// Write an orphaned .xmp sidecar (no matching parent).
	writeTestFile(t, dir, "orphan.xmp", []byte("<xmp/>"))

	var buf bytes.Buffer
	result, err := Run(Options{
		Dir:      dir,
		Hasher:   hasher,
		Registry: reg,
		Output:   &buf,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Unrecognised != 1 {
		t.Errorf("Unrecognised = %d, want 1 (orphaned sidecar should be counted)\noutput:\n%s", result.Unrecognised, buf.String())
	}
	if result.Verified != 1 {
		t.Errorf("Verified = %d, want 1\noutput:\n%s", result.Verified, buf.String())
	}
}

// TestAssociateSidecars verifies the sidecar association logic.
func TestAssociateSidecars(t *testing.T) {
	t.Parallel()

	mediaNames := []string{
		"20211225_062223-1-abc123.arw",
		"20211225_062223-1-abc123.jpg",
	}
	sidecarNames := []string{
		"20211225_062223-1-abc123.arw.xmp", // matches arw
		"20211225_062223-1-abc123.jpg.aae", // matches jpg
		"orphan.xmp",                       // no match
	}

	parentMap, orphans := associateSidecars(mediaNames, sidecarNames)

	// arw should have .xmp
	if exts := parentMap["20211225_062223-1-abc123.arw"]; len(exts) != 1 || exts[0] != ".xmp" {
		t.Errorf("arw sidecars = %v, want [.xmp]", exts)
	}
	// jpg should have .aae
	if exts := parentMap["20211225_062223-1-abc123.jpg"]; len(exts) != 1 || exts[0] != ".aae" {
		t.Errorf("jpg sidecars = %v, want [.aae]", exts)
	}
	// orphan.xmp should be in orphans
	if len(orphans) != 1 || orphans[0] != "orphan.xmp" {
		t.Errorf("orphans = %v, want [orphan.xmp]", orphans)
	}
}

// TestIsSidecarFile verifies the sidecar extension detection.
func TestIsSidecarFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want bool
	}{
		{"photo.xmp", true},
		{"photo.XMP", true},
		{"photo.aae", true},
		{"photo.AAE", true},
		{"photo.jpg", false},
		{"photo.arw", false},
		{"photo.arw.xmp", true},
	}
	for _, tc := range cases {
		got := isSidecarFile(tc.name)
		if got != tc.want {
			t.Errorf("isSidecarFile(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
