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

// Package integration contains end-to-end tests that exercise the full
// sort → verify → resume cycle using real fixture files and real packages.
// All tests use t.TempDir() for isolation.
package integration

import (
	"bytes"
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/language"

	"github.com/cwlls/pixe-go/internal/archivedb"
	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	arwhandler "github.com/cwlls/pixe-go/internal/handler/arw"
	cr2handler "github.com/cwlls/pixe-go/internal/handler/cr2"
	cr3handler "github.com/cwlls/pixe-go/internal/handler/cr3"
	dnghandler "github.com/cwlls/pixe-go/internal/handler/dng"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	nefhandler "github.com/cwlls/pixe-go/internal/handler/nef"
	pefhandler "github.com/cwlls/pixe-go/internal/handler/pef"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/manifest"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
	"github.com/cwlls/pixe-go/internal/verify"
)

// TestMain pins the locale to English so that month directory assertions are
// deterministic regardless of the developer's system locale.
func TestMain(m *testing.M) {
	pathbuilder.SetLocaleForTesting(language.English)
	os.Exit(m.Run())
}

// --- helpers ---

const (
	fixtureExif1  = "with_exif_date.jpg"  // date: 2021-12-25 06:22:23
	fixtureExif2  = "with_exif_date2.jpg" // date: 2022-02-02 12:31:01
	fixtureNoExif = "no_exif.jpg"         // no EXIF → 1902-02-20 fallback
)

func fixturesDir() string {
	return filepath.Join("..", "handler", "jpeg", "testdata")
}

// copyFixture copies a named fixture into dir, optionally renaming it.
func copyFixture(t *testing.T, dir, srcName, dstName string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir(), srcName))
	if err != nil {
		t.Fatalf("copyFixture: read %q: %v", srcName, err)
	}
	dst := filepath.Join(dir, dstName)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("copyFixture: write %q: %v", dst, err)
	}
	return dst
}

// buildOpts constructs a SortOptions wired to a real JPEG handler and SHA-1 hasher.
func buildOpts(t *testing.T, dirA, dirB string, dryRun bool) pipeline.SortOptions {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	return pipeline.SortOptions{
		Config: &config.AppConfig{
			Source:      dirA,
			Destination: dirB,
			Algorithm:   "sha1",
			DryRun:      dryRun,
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &bytes.Buffer{},
		PixeVersion:  "test",
	}
}

// buildOptsWithRAW constructs a SortOptions with all handlers (JPEG + 6 RAW formats).
func buildOptsWithRAW(t *testing.T, dirA, dirB string, dryRun bool) pipeline.SortOptions {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	reg.Register(dnghandler.New())
	reg.Register(nefhandler.New())
	reg.Register(cr2handler.New())
	reg.Register(cr3handler.New())
	reg.Register(pefhandler.New())
	reg.Register(arwhandler.New())
	return pipeline.SortOptions{
		Config: &config.AppConfig{
			Source:      dirA,
			Destination: dirB,
			Algorithm:   "sha1",
			DryRun:      dryRun,
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &bytes.Buffer{},
		PixeVersion:  "test",
	}
}

// findFiles returns all regular files under root whose names match prefix.
func findFiles(t *testing.T, root, prefix string) []string {
	t.Helper()
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasPrefix(d.Name(), prefix) {
			found = append(found, path)
		}
		return nil
	})
	return found
}

// sha1File returns the SHA-1 hex digest of the full file at path.
func sha1File(t *testing.T, path string) string {
	t.Helper()
	h, _ := hash.NewHasher("sha1")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("sha1File open %q: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	sum, err := h.Sum(f)
	if err != nil {
		t.Fatalf("sha1File sum %q: %v", path, err)
	}
	return sum
}

// --- RAW Fixture Builders ---

// buildFakeDNG writes a minimal valid TIFF LE file with .dng extension.
func buildFakeDNG(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// IFD0 offset (8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 0 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFakeNEF writes a minimal valid TIFF LE file with .nef extension.
func buildFakeNEF(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// IFD0 offset (8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 0 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFakeCR2 writes a CR2 file with TIFF LE header + "CR" at offset 8.
func buildFakeCR2(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// IFD0 offset (10)
	_ = binary.Write(buf, binary.LittleEndian, uint32(10))

	// "CR" signature at offset 8
	buf.WriteByte(0x43)
	buf.WriteByte(0x52)

	// IFD0: 0 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFakeCR3 writes a minimal CR3 file with ftyp box and "crx " brand.
func buildFakeCR3(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// ftyp box: size = 20
	_ = binary.Write(buf, binary.BigEndian, uint32(20))
	buf.WriteString("ftyp")
	buf.WriteString("crx ")
	_ = binary.Write(buf, binary.BigEndian, uint32(1)) // minor version
	buf.WriteString("crx ")                            // compat

	// mdat box: size = 16
	_ = binary.Write(buf, binary.BigEndian, uint32(16))
	buf.WriteString("mdat")
	_ = binary.Write(buf, binary.BigEndian, uint64(0)) // dummy data

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFakePEF writes a minimal valid TIFF LE file with .pef extension.
func buildFakePEF(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// IFD0 offset (8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 0 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFakeARW writes a minimal valid TIFF LE file with .arw extension.
func buildFakeARW(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)

	// Byte order marker (LE)
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)

	// TIFF magic (42 in LE)
	_ = binary.Write(buf, binary.LittleEndian, uint16(42))

	// IFD0 offset (8)
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))

	// IFD0: 0 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))

	// Next IFD offset (0 = end)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- Tests ---

// TestIntegration_FullSort verifies the complete sort pipeline produces the
// correct directory structure and file naming for 3 JPEG fixtures.
func TestIntegration_FullSort(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")
	copyFixture(t, dirA, fixtureNoExif, "IMG_0003.jpg")

	result, err := pipeline.Run(buildOpts(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 3 {
		t.Errorf("Processed = %d, want 3", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// File with EXIF date 2021-12-25 must land under 2021/12-Dec/.
	files2021 := findFiles(t, filepath.Join(dirB, "2021", "12-Dec"), "20211225_062223_")
	if len(files2021) != 1 {
		t.Errorf("expected 1 file in 2021/12-Dec/ with prefix 20211225_062223_, got %d", len(files2021))
	}

	// File with no EXIF must land under 1902/2/ (or duplicates/.../1902/2/).
	noExifFiles := findFiles(t, dirB, "19020220_000000_")
	if len(noExifFiles) == 0 {
		t.Error("expected at least 1 file with Ansel Adams prefix 19020220_000000_")
	}
}

// TestIntegration_VerifyClean sorts files then verifies the archive reports
// zero mismatches.
func TestIntegration_VerifyClean(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	if _, err := pipeline.Run(buildOpts(t, dirA, dirB, false)); err != nil {
		t.Fatalf("sort Run: %v", err)
	}

	h, _ := hash.NewHasher("sha1")
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())

	vResult, err := verify.Run(verify.Options{
		Dir:      dirB,
		Hasher:   h,
		Registry: reg,
		Output:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("verify Run: %v", err)
	}

	if vResult.Mismatches != 0 {
		t.Errorf("Mismatches = %d, want 0", vResult.Mismatches)
	}
	if vResult.Verified < 1 {
		t.Errorf("Verified = %d, want >= 1", vResult.Verified)
	}
}

// TestIntegration_DuplicateRouting copies the same file twice and asserts the
// second copy is routed to the duplicates/ subtree.
func TestIntegration_DuplicateRouting(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif1, "IMG_0001_copy.jpg") // identical content

	result, err := pipeline.Run(buildOpts(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}

	dupDir := filepath.Join(dirB, "duplicates")
	if _, err := os.Stat(dupDir); err != nil {
		t.Errorf("duplicates/ directory not created: %v", err)
	}
}

// TestIntegration_NoDateFallback asserts that a file with no EXIF data is
// placed under 1902/2/ with the Ansel Adams prefix.
func TestIntegration_NoDateFallback(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureNoExif, "no_date.jpg")

	if _, err := pipeline.Run(buildOpts(t, dirA, dirB, false)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Walk all of dirB looking for the Ansel Adams prefix.
	files := findFiles(t, dirB, "19020220_000000_")
	if len(files) == 0 {
		t.Error("no file with Ansel Adams prefix 19020220_000000_ found in dirB")
	}

	// The path must contain "1902" and "02-Feb" directory components.
	for _, f := range files {
		rel, _ := filepath.Rel(dirB, f)
		parts := strings.Split(rel, string(filepath.Separator))
		found1902 := false
		found02Feb := false
		for _, p := range parts {
			if p == "1902" {
				found1902 = true
			}
			if p == "02-Feb" {
				found02Feb = true
			}
		}
		if !found1902 {
			t.Errorf("path %q does not contain 1902 directory", rel)
		}
		if !found02Feb {
			t.Errorf("path %q does not contain 02-Feb directory", rel)
		}
	}
}

// TestIntegration_SecondRunDeduplicates verifies that running the pipeline
// twice against the same source/dest produces consistent results.
// The 3 fixture files all share the same image payload checksum (EXIF is
// stripped before hashing), so only the first file processed is "original"
// and the remaining 2 are always duplicates — on both runs.
func TestIntegration_SecondRunDeduplicates(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")
	copyFixture(t, dirA, fixtureNoExif, "IMG_0003.jpg")

	// First run — all 3 files processed; 2 are duplicates (same image payload).
	result1, err := pipeline.Run(buildOpts(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if result1.Processed != 3 {
		t.Fatalf("first run: Processed = %d, want 3", result1.Processed)
	}
	if result1.Errors != 0 {
		t.Fatalf("first run: Errors = %d, want 0", result1.Errors)
	}
	if result1.Duplicates != 2 {
		t.Errorf("first run: Duplicates = %d, want 2", result1.Duplicates)
	}

	// Second run — same source files, same dedup behaviour (no DB, so in-memory dedup).
	result2, err := pipeline.Run(buildOpts(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if result2.Errors != 0 {
		t.Errorf("second run: Errors = %d, want 0", result2.Errors)
	}
	if result2.Processed != 3 {
		t.Errorf("second run: Processed = %d, want 3", result2.Processed)
	}
	if result2.Duplicates != 2 {
		t.Errorf("second run: Duplicates = %d, want 2", result2.Duplicates)
	}
}

// TestIntegration_SourceUntouched asserts that source files are byte-for-byte
// identical after a sort, and that only .pixe_ledger.json is added to dirA.
func TestIntegration_SourceUntouched(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	src1 := copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	src2 := copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	// Record checksums before sort.
	before := map[string]string{
		src1: sha1File(t, src1),
		src2: sha1File(t, src2),
	}

	if _, err := pipeline.Run(buildOpts(t, dirA, dirB, false)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify source files are unchanged.
	for path, wantSum := range before {
		gotSum := sha1File(t, path)
		if gotSum != wantSum {
			t.Errorf("source file %q was modified during sort (checksum changed)", filepath.Base(path))
		}
	}

	// Only .pixe_ledger.json should be new in dirA.
	entries, err := os.ReadDir(dirA)
	if err != nil {
		t.Fatalf("ReadDir dirA: %v", err)
	}
	allowed := map[string]bool{
		"IMG_0001.jpg":      true,
		"IMG_0002.jpg":      true,
		".pixe_ledger.json": true,
	}
	for _, e := range entries {
		if !allowed[e.Name()] {
			t.Errorf("unexpected file in dirA after sort: %q", e.Name())
		}
	}
}

// TestIntegration_DryRun asserts that --dry-run creates no media files in dirB.
func TestIntegration_DryRun(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	result, err := pipeline.Run(buildOpts(t, dirA, dirB, true))
	if err != nil {
		t.Fatalf("dry-run Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("dry-run Errors = %d, want 0", result.Errors)
	}

	// Walk dirB — no media files should exist (only .pixe/ manifest is allowed).
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel, _ := filepath.Rel(dirB, path)
		// Allow manifest files under .pixe/
		if strings.HasPrefix(rel, ".pixe"+string(filepath.Separator)) {
			return nil
		}
		t.Errorf("dry-run created unexpected file in dirB: %q", rel)
		return nil
	})

	// Ledger must NOT be written to dirA in dry-run mode.
	ledgerPath := filepath.Join(dirA, ".pixe_ledger.json")
	if _, err := os.Stat(ledgerPath); err == nil {
		t.Error("dry-run should not write .pixe_ledger.json to dirA")
	}
}

// --- RAW Handler Integration Tests (Task 59) ---

// TestIntegration_RAW_Discovery verifies that all 6 RAW formats are discovered
// and processed correctly.
func TestIntegration_RAW_Discovery(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Create one file of each RAW format
	buildFakeDNG(t, dirA, "test.dng")
	buildFakeNEF(t, dirA, "test.nef")
	buildFakeCR2(t, dirA, "test.cr2")
	buildFakeCR3(t, dirA, "test.cr3")
	buildFakePEF(t, dirA, "test.pef")
	buildFakeARW(t, dirA, "test.arw")

	result, err := pipeline.Run(buildOptsWithRAW(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 6 {
		t.Errorf("Processed = %d, want 6", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// Verify each file was discovered and output files exist with correct extensions
	extensions := []string{".dng", ".nef", ".cr2", ".cr3", ".pef", ".arw"}
	for _, ext := range extensions {
		files := findFiles(t, dirB, "19020220_000000_")
		found := false
		for _, f := range files {
			if strings.HasSuffix(f, ext) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no output file found with extension %q", ext)
		}
	}
}

// TestIntegration_RAW_FullSort verifies RAW files are sorted with correct naming
// and extensions are preserved lowercase.
func TestIntegration_RAW_FullSort(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Create a mix of JPEG and RAW files
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	buildFakeDNG(t, dirA, "RAW_0001.dng")
	buildFakeCR2(t, dirA, "RAW_0002.cr2")

	result, err := pipeline.Run(buildOptsWithRAW(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 3 {
		t.Errorf("Processed = %d, want 3", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// Verify JPEG lands in 2021/12-Dec/
	jpegFiles := findFiles(t, filepath.Join(dirB, "2021", "12-Dec"), "20211225_062223_")
	if len(jpegFiles) != 1 {
		t.Errorf("expected 1 JPEG file in 2021/12-Dec/, got %d", len(jpegFiles))
	}

	// Verify RAW files land in 1902/02-Feb/ with lowercase extensions
	rawFiles := findFiles(t, dirB, "19020220_000000_")
	if len(rawFiles) < 2 {
		t.Errorf("expected at least 2 RAW files with Ansel Adams prefix, got %d", len(rawFiles))
	}

	// Check that extensions are lowercase
	for _, f := range rawFiles {
		if strings.HasSuffix(f, ".DNG") || strings.HasSuffix(f, ".CR2") {
			t.Errorf("RAW file has uppercase extension: %q", filepath.Base(f))
		}
	}
}

// TestIntegration_RAW_DuplicateDetection verifies that identical RAW files
// are routed to duplicates/.
func TestIntegration_RAW_DuplicateDetection(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Create the same RAW file twice with different names
	buildFakeDNG(t, dirA, "RAW_0001.dng")
	buildFakeDNG(t, dirA, "RAW_0001_copy.dng")

	result, err := pipeline.Run(buildOptsWithRAW(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}

	// Verify duplicates/ directory exists
	dupDir := filepath.Join(dirB, "duplicates")
	if _, err := os.Stat(dupDir); err != nil {
		t.Errorf("duplicates/ directory not created: %v", err)
	}
}

// TestIntegration_RAW_MixedWithJPEG verifies sorting a directory with both
// JPEG and RAW files produces correct date-based organization.
// Note: fixtureExif1 and fixtureExif2 have the same image payload (EXIF stripped),
// so fixtureExif2 will be detected as a duplicate. We only verify fixtureExif1.
func TestIntegration_RAW_MixedWithJPEG(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// JPEG with EXIF date 2021-12-25
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	// RAW files (no EXIF, will get Ansel Adams date 1902-02-20)
	buildFakeDNG(t, dirA, "RAW_0001.dng")
	buildFakeCR2(t, dirA, "RAW_0002.cr2")

	result, err := pipeline.Run(buildOptsWithRAW(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 3 {
		t.Errorf("Processed = %d, want 3", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// Verify JPEG file is in 2021/12-Dec/
	files2021 := findFiles(t, filepath.Join(dirB, "2021", "12-Dec"), "20211225_062223_")
	if len(files2021) != 1 {
		t.Errorf("expected 1 file in 2021/12-Dec/, got %d", len(files2021))
	}

	// Verify RAW files are in 1902/02-Feb/
	rawFiles := findFiles(t, dirB, "19020220_000000_")
	if len(rawFiles) < 2 {
		t.Errorf("expected at least 2 RAW files with Ansel Adams prefix, got %d", len(rawFiles))
	}
}

// TestIntegration_RAW_Verify verifies that checksums for RAW files match
// after sorting and verification.
func TestIntegration_RAW_Verify(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Create RAW files
	buildFakeDNG(t, dirA, "RAW_0001.dng")
	buildFakeCR2(t, dirA, "RAW_0002.cr2")

	// Sort
	_, err := pipeline.Run(buildOptsWithRAW(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("sort Run: %v", err)
	}

	// Verify
	h, _ := hash.NewHasher("sha1")
	reg := discovery.NewRegistry()
	reg.Register(dnghandler.New())
	reg.Register(cr2handler.New())

	vResult, err := verify.Run(verify.Options{
		Dir:      dirB,
		Hasher:   h,
		Registry: reg,
		Output:   &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("verify Run: %v", err)
	}

	if vResult.Mismatches != 0 {
		t.Errorf("Mismatches = %d, want 0", vResult.Mismatches)
	}
	if vResult.Verified < 1 {
		t.Errorf("Verified = %d, want >= 1", vResult.Verified)
	}
}

// --- SQLite Integration Tests (Task 42) ---

// buildOptsWithDB constructs SortOptions wired to a real archivedb.DB.
func buildOptsWithDB(t *testing.T, dirA, dirB string, dryRun bool) (pipeline.SortOptions, *archivedb.DB) {
	t.Helper()
	dbPath := filepath.Join(dirB, ".pixe", "pixe.db")
	db, err := archivedb.Open(dbPath)
	if err != nil {
		t.Fatalf("archivedb.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	runID := uuid.New().String()
	opts := buildOpts(t, dirA, dirB, dryRun)
	opts.DB = db
	opts.RunID = runID
	return opts, db
}

// loadLedger loads the ledger from dirA/.pixe_ledger.json.
func loadLedger(t *testing.T, dirA string) *domain.Ledger {
	t.Helper()
	l, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if l == nil {
		t.Fatal("ledger not written to dirA")
	}
	return l
}

// TestIntegration_SQLite_FullSort verifies a complete sort with DB persistence.
func TestIntegration_SQLite_FullSort(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	opts, db := buildOptsWithDB(t, dirA, dirB, false)
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// Verify DB file exists.
	dbPath := filepath.Join(dirB, ".pixe", "pixe.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("DB file not created at %q: %v", dbPath, err)
	}

	// Verify run record exists with status "completed".
	runs, err := db.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("ListRuns returned %d runs, want 1", len(runs))
	}
	if runs[0].Status != "completed" {
		t.Errorf("run status = %q, want %q", runs[0].Status, "completed")
	}
	if runs[0].FileCount != 2 {
		t.Errorf("run FileCount = %d, want 2", runs[0].FileCount)
	}

	// Verify files in DB have status "complete".
	files, err := db.GetFilesByRun(opts.RunID)
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("GetFilesByRun returned %d files, want 2", len(files))
	}
	for _, f := range files {
		if f.Status != "complete" {
			t.Errorf("file status = %q, want %q", f.Status, "complete")
		}
	}

	// Verify ledger has version 2 and run_id.
	ledger := loadLedger(t, dirA)
	if ledger.Version != 2 {
		t.Errorf("ledger Version = %d, want 2", ledger.Version)
	}
	if ledger.RunID != opts.RunID {
		t.Errorf("ledger RunID = %q, want %q", ledger.RunID, opts.RunID)
	}
}

// TestIntegration_SQLite_DuplicateRouting verifies duplicate detection with DB.
func TestIntegration_SQLite_DuplicateRouting(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif1, "IMG_0001_copy.jpg") // identical content

	opts, db := buildOptsWithDB(t, dirA, dirB, false)
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}

	// Verify DB has 1 duplicate.
	dups, err := db.AllDuplicates()
	if err != nil {
		t.Fatalf("AllDuplicates: %v", err)
	}
	if len(dups) != 1 {
		t.Errorf("AllDuplicates returned %d files, want 1", len(dups))
	}
	if !dups[0].IsDuplicate {
		t.Error("duplicate file IsDuplicate = false, want true")
	}

	// Verify CheckDuplicate returns the original destination.
	if dups[0].Checksum == nil {
		t.Fatal("duplicate file Checksum is nil")
	}
	destRel, err := db.CheckDuplicate(*dups[0].Checksum)
	if err != nil {
		t.Fatalf("CheckDuplicate: %v", err)
	}
	if destRel == "" {
		t.Error("CheckDuplicate returned empty string, want non-empty")
	}
}

// TestIntegration_SQLite_MultiSource verifies multiple runs into the same DB.
func TestIntegration_SQLite_MultiSource(t *testing.T) {
	dirA1 := t.TempDir()
	dirA2 := t.TempDir()
	dirB := t.TempDir()

	// First run: 1 file from dirA1.
	copyFixture(t, dirA1, fixtureExif1, "IMG_0001.jpg")
	opts1, db := buildOptsWithDB(t, dirA1, dirB, false)
	result1, err := pipeline.Run(opts1)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if result1.Processed != 1 {
		t.Errorf("first run Processed = %d, want 1", result1.Processed)
	}

	// Second run: 1 file from dirA2 into the same dirB.
	copyFixture(t, dirA2, fixtureExif2, "IMG_0002.jpg")
	opts2, _ := buildOptsWithDB(t, dirA2, dirB, false)
	result2, err := pipeline.Run(opts2)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if result2.Processed != 1 {
		t.Errorf("second run Processed = %d, want 1", result2.Processed)
	}

	// Verify DB has 2 runs.
	runs, err := db.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("ListRuns returned %d runs, want 2", len(runs))
	}

	// Verify total file count across both runs.
	totalFiles := 0
	for _, run := range runs {
		totalFiles += run.FileCount
	}
	if totalFiles != 2 {
		t.Errorf("total file count = %d, want 2", totalFiles)
	}
}

// TestIntegration_SQLite_Resume verifies resuming an interrupted run.
func TestIntegration_SQLite_Resume(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	opts, db := buildOptsWithDB(t, dirA, dirB, false)
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("initial Run: %v", err)
	}
	if result.Processed != 2 {
		t.Errorf("initial run Processed = %d, want 2", result.Processed)
	}

	// Simulate an interrupted run: insert a new run with status "running"
	// and mark one file as "pending".
	interruptedRunID := uuid.New().String()
	interruptedRun := &archivedb.Run{
		ID:          interruptedRunID,
		PixeVersion: "test",
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
		Workers:     1,
		StartedAt:   time.Now().UTC(),
		Status:      "running",
	}
	if err := db.InsertRun(interruptedRun); err != nil {
		t.Fatalf("InsertRun interrupted: %v", err)
	}

	// Insert a file for the interrupted run.
	fileRec := &archivedb.FileRecord{
		RunID:      interruptedRunID,
		SourcePath: "/src/test.jpg",
		Status:     "pending",
	}
	if _, err := db.InsertFile(fileRec); err != nil {
		t.Fatalf("InsertFile interrupted: %v", err)
	}

	// Find interrupted runs.
	interrupted, err := db.FindInterruptedRuns()
	if err != nil {
		t.Fatalf("FindInterruptedRuns: %v", err)
	}
	if len(interrupted) != 1 {
		t.Errorf("FindInterruptedRuns returned %d runs, want 1", len(interrupted))
	}
	if interrupted[0].ID != interruptedRunID {
		t.Errorf("interrupted run ID = %q, want %q", interrupted[0].ID, interruptedRunID)
	}
}

// TestIntegration_SQLite_DryRun verifies dry-run with DB persistence.
func TestIntegration_SQLite_DryRun(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	opts, db := buildOptsWithDB(t, dirA, dirB, true)
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("dry-run Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("dry-run Errors = %d, want 0", result.Errors)
	}

	// Verify no media files in dirB (only .pixe/ directory).
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel, _ := filepath.Rel(dirB, path)
		// Allow files under .pixe/
		if strings.HasPrefix(rel, ".pixe"+string(filepath.Separator)) {
			return nil
		}
		t.Errorf("dry-run created unexpected file in dirB: %q", rel)
		return nil
	})

	// Verify DB run record exists with status "completed".
	runs, err := db.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("ListRuns returned %d runs, want 1", len(runs))
	}
	if runs[0].Status != "completed" {
		t.Errorf("dry-run run status = %q, want %q", runs[0].Status, "completed")
	}

	// Verify files in DB have status "complete" (dry-run still records to DB).
	files, err := db.GetFilesByRun(opts.RunID)
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("GetFilesByRun returned %d files, want 2", len(files))
	}
	for _, f := range files {
		if f.Status != "complete" {
			t.Errorf("file status = %q, want %q", f.Status, "complete")
		}
	}
}
