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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/manifest"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
	"github.com/cwlls/pixe-go/internal/verify"
)

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

	// File with EXIF date 2021-12-25 must land under 2021/12/.
	files2021 := findFiles(t, filepath.Join(dirB, "2021", "12"), "20211225_062223_")
	if len(files2021) != 1 {
		t.Errorf("expected 1 file in 2021/12/ with prefix 20211225_062223_, got %d", len(files2021))
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

	// The path must contain "1902" and "2" directory components.
	for _, f := range files {
		rel, _ := filepath.Rel(dirB, f)
		parts := strings.Split(rel, string(filepath.Separator))
		// parts[0] or parts[2] (under duplicates) should be "1902"
		found1902 := false
		for _, p := range parts {
			if p == "1902" {
				found1902 = true
				break
			}
		}
		if !found1902 {
			t.Errorf("path %q does not contain 1902 directory", rel)
		}
	}
}

// TestIntegration_ResumeAfterInterrupt simulates an interrupted sort by
// manually resetting manifest entries to pending, then resuming.
func TestIntegration_ResumeAfterInterrupt(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")
	copyFixture(t, dirA, fixtureNoExif, "IMG_0003.jpg")

	opts := buildOpts(t, dirA, dirB, false)

	// First run — complete all 3 files.
	result1, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if result1.Processed != 3 {
		t.Fatalf("first run: Processed = %d, want 3", result1.Processed)
	}

	// Simulate interrupt: reset 2 entries back to pending.
	m, err := manifest.Load(dirB)
	if err != nil || m == nil {
		t.Fatalf("Load manifest: %v", err)
	}
	resetCount := 0
	for _, e := range m.Files {
		if resetCount < 2 {
			e.Status = domain.StatusPending
			e.Checksum = ""
			e.Destination = ""
			e.CopiedAt = nil
			e.VerifiedAt = nil
			resetCount++
		}
	}
	if err := manifest.Save(m, dirB); err != nil {
		t.Fatalf("Save manifest after reset: %v", err)
	}

	// Resume run — should re-process the 2 reset entries.
	opts2 := buildOpts(t, dirA, dirB, false)
	result2, err := pipeline.Run(opts2)
	if err != nil {
		t.Fatalf("resume Run: %v", err)
	}
	if result2.Errors != 0 {
		t.Errorf("resume: Errors = %d, want 0", result2.Errors)
	}

	// All 3 entries must be complete in the final manifest.
	mFinal, err := manifest.Load(dirB)
	if err != nil || mFinal == nil {
		t.Fatalf("Load final manifest: %v", err)
	}
	for _, e := range mFinal.Files {
		if e.Status != domain.StatusComplete {
			t.Errorf("entry %q status = %q after resume, want complete", filepath.Base(e.Source), e.Status)
		}
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
