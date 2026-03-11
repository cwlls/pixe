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

package integration

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cwlls/pixe-go/internal/archivedb"
	arwhandler "github.com/cwlls/pixe-go/internal/handler/arw"
	"github.com/cwlls/pixe-go/internal/pipeline"
)

// pixeXMPPattern matches Pixe-generated XMP sidecar filenames — duplicated
// from cmd/clean.go so the integration package doesn't import cmd/.
var pixeXMPPattern = regexp.MustCompile(`^\d{8}_\d{6}_[0-9a-f]+\..+\.xmp$`)

// --- helpers ---

// plantTempFile creates a fake orphaned .pixe-tmp file in dir.
func plantTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for temp file: %v", err)
	}
	if err := os.WriteFile(path, []byte("orphaned temp data"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// plantSidecar creates an XMP sidecar file at the given path.
func plantSidecar(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for sidecar: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
}

// isTempFile mirrors cmd/clean.go logic.
func isTempFile(name string) bool {
	return strings.Contains(name, ".pixe-tmp")
}

// isOrphanedSidecar mirrors cmd/clean.go logic.
func isOrphanedSidecar(path, name string) bool {
	if !pixeXMPPattern.MatchString(name) {
		return false
	}
	mediaPath := strings.TrimSuffix(path, ".xmp")
	_, err := os.Stat(mediaPath)
	return os.IsNotExist(err)
}

// --- Tests ---

// TestClean_SortThenClean is the primary end-to-end test:
//  1. Sort JPEG fixtures from dirA → dirB (creates archive structure + DB).
//  2. Plant orphaned .pixe-tmp files and orphaned XMP sidecars in dirB.
//  3. Simulate clean by walking dirB and removing orphans.
//  4. Verify orphans are removed, legitimate files + sidecars are untouched,
//     and the database is still valid.
//
// Note: this test exercises the clean logic at the function level rather than
// calling cmd.runClean (which would create an import cycle). The integration
// value is verifying that orphan detection works correctly in a real archive
// tree produced by the sort pipeline.
func TestClean_SortThenClean(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// --- Step 1: Sort files to create a real archive structure. ---
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")

	opts, db := buildOptsWithDB(t, dirA, dirB, false)
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}
	if result.Processed != 1 {
		t.Fatalf("Processed = %d, want 1", result.Processed)
	}
	if result.Errors != 0 {
		t.Fatalf("Errors = %d, want 0", result.Errors)
	}

	// --- Step 2: Locate the sorted file and plant orphans. ---
	var sortedFile string
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		if strings.HasSuffix(path, ".jpg") && !strings.Contains(d.Name(), ".pixe-tmp") {
			sortedFile = path
		}
		return nil
	})
	if sortedFile == "" {
		t.Fatal("no sorted JPEG found in dirB")
	}
	sortedDir := filepath.Dir(sortedFile)
	sortedBase := filepath.Base(sortedFile)

	// Orphaned temp file in the same directory as the sorted file.
	tempFile1 := plantTempFile(t, sortedDir, "."+sortedBase+".pixe-tmp-abc123")

	// Orphaned temp file in a different subdirectory.
	tempFile2 := plantTempFile(t, filepath.Join(dirB, "2022", "01-Jan"),
		".20220101_120000_deadbeef.jpg.pixe-tmp")

	// Orphaned XMP sidecar (no corresponding media file).
	orphanedXMP := filepath.Join(sortedDir, "20211225_071500_a3b4c5d6e7f89012.arw.xmp")
	plantSidecar(t, orphanedXMP, "orphaned xmp content")

	// Valid XMP sidecar for the sorted file — should NOT be removed.
	// (Pixe names sidecars as <mediafile>.xmp, e.g. foo.arw.xmp)
	// Since the sorted file is a .jpg, create a matching sidecar to verify it's kept.
	matchingSidecar := sortedFile + ".xmp"
	plantSidecar(t, matchingSidecar, "valid xmp content")

	// Non-Pixe XMP file — should NOT be removed.
	nonPixeXMP := filepath.Join(sortedDir, "notes.xmp")
	plantSidecar(t, nonPixeXMP, "user notes")

	// Record legitimate file paths before clean.
	legitimateFiles := map[string]bool{
		sortedFile:      true,
		matchingSidecar: true,
		nonPixeXMP:      true,
	}

	// --- Step 3: Simulate clean — walk and remove orphans. ---
	var removedTemp, removedSidecar int
	err = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		name := d.Name()

		if isTempFile(name) {
			if err := os.Remove(path); err != nil {
				t.Errorf("failed to remove temp file %q: %v", path, err)
			}
			removedTemp++
			return nil
		}

		if isOrphanedSidecar(path, name) {
			if err := os.Remove(path); err != nil {
				t.Errorf("failed to remove orphaned sidecar %q: %v", path, err)
			}
			removedSidecar++
			return nil
		}

		return nil
	})
	if err != nil {
		t.Fatalf("clean walk: %v", err)
	}

	// --- Step 4: Verify results. ---
	if removedTemp != 2 {
		t.Errorf("removed %d temp files, want 2", removedTemp)
	}
	if removedSidecar != 1 {
		t.Errorf("removed %d orphaned sidecars, want 1", removedSidecar)
	}

	// Orphaned files should be gone.
	for _, path := range []string{tempFile1, tempFile2, orphanedXMP} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("orphaned file still exists: %q", path)
		}
	}

	// Legitimate files should still exist.
	for path := range legitimateFiles {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("legitimate file missing after clean: %q: %v", path, err)
		}
	}

	// Database should still be valid.
	runs, err := db.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns after clean: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("ListRuns = %d, want 1", len(runs))
	}
	if runs[0].Status != "completed" {
		t.Errorf("run status = %q, want %q", runs[0].Status, "completed")
	}
}

// TestClean_DryRunPreservesAll verifies that a simulated dry-run walk
// identifies orphans but removes nothing.
func TestClean_DryRunPreservesAll(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Sort one file to create archive structure.
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	opts, _ := buildOptsWithDB(t, dirA, dirB, false)
	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}

	// Plant orphans.
	tempFile := plantTempFile(t, dirB, ".orphan.pixe-tmp")
	orphanedXMP := filepath.Join(dirB, "20211225_071500_deadbeef.arw.xmp")
	plantSidecar(t, orphanedXMP, "orphaned")

	// Dry-run: identify but don't remove.
	var buf bytes.Buffer
	var wouldRemove int
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		name := d.Name()
		if isTempFile(name) || isOrphanedSidecar(path, name) {
			wouldRemove++
			rel, _ := filepath.Rel(dirB, path)
			buf.WriteString("WOULD REMOVE " + rel + "\n")
		}
		return nil
	})

	if wouldRemove != 2 {
		t.Errorf("dry-run identified %d orphans, want 2", wouldRemove)
	}

	// Both files should still exist.
	if _, err := os.Stat(tempFile); err != nil {
		t.Errorf("temp file removed during dry-run: %v", err)
	}
	if _, err := os.Stat(orphanedXMP); err != nil {
		t.Errorf("orphaned XMP removed during dry-run: %v", err)
	}
}

// TestClean_VacuumIntegration verifies that VACUUM succeeds on a real
// archive database after sorting, and that the DB is still usable afterward.
func TestClean_VacuumIntegration(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Sort files to populate the database.
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	opts, db := buildOptsWithDB(t, dirA, dirB, false)
	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}

	// Verify no active runs (prerequisite for VACUUM).
	active, err := db.HasActiveRuns()
	if err != nil {
		t.Fatalf("HasActiveRuns: %v", err)
	}
	if active {
		t.Fatal("HasActiveRuns = true after completed sort, want false")
	}

	// Get size before VACUUM.
	dbPath := db.Path()
	beforeInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat DB before vacuum: %v", err)
	}
	sizeBefore := beforeInfo.Size()

	// Run VACUUM.
	if err := db.Vacuum(); err != nil {
		t.Fatalf("Vacuum: %v", err)
	}

	// Close and re-stat to get accurate post-VACUUM size.
	_ = db.Close()
	afterInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat DB after vacuum: %v", err)
	}
	sizeAfter := afterInfo.Size()

	// On small databases, VACUUM may increase size (WAL journal is folded in
	// and the DB is rebuilt from scratch). The important thing is that VACUUM
	// completed without error and the DB is still usable.
	t.Logf("DB size: before=%d, after=%d", sizeBefore, sizeAfter)
	if sizeAfter <= 0 {
		t.Errorf("DB has zero size after VACUUM")
	}

	// Re-open and verify DB is still usable.
	db2, err := archivedb.Open(dbPath)
	if err != nil {
		t.Fatalf("re-open DB after vacuum: %v", err)
	}
	defer func() { _ = db2.Close() }()

	runs, err := db2.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns after vacuum: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("ListRuns = %d after vacuum, want 1", len(runs))
	}
	if runs[0].Status != "completed" {
		t.Errorf("run status = %q after vacuum, want %q", runs[0].Status, "completed")
	}
}

// TestClean_RAWSidecarsPreserved verifies that XMP sidecars for RAW files
// (which are legitimate, not orphaned) survive the clean process.
func TestClean_RAWSidecarsPreserved(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Sort an ARW file with tags to generate a real XMP sidecar.
	buildFakeARW(t, dirA, "RAW_0001.arw")
	opts := buildOptsWithTags(t, dirA, dirB, arwhandler.New())
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}
	if result.Processed != 1 {
		t.Fatalf("Processed = %d, want 1", result.Processed)
	}

	// Find the sorted ARW and its sidecar.
	var arwDest, sidecarDest string
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		if strings.HasSuffix(path, ".arw") {
			arwDest = path
		}
		if strings.HasSuffix(path, ".arw.xmp") {
			sidecarDest = path
		}
		return nil
	})
	if arwDest == "" {
		t.Fatal("no ARW file found in dirB")
	}
	if sidecarDest == "" {
		t.Fatal("no ARW sidecar found in dirB")
	}

	// Verify the sidecar is NOT identified as orphaned (media file exists).
	sidecarName := filepath.Base(sidecarDest)
	if isOrphanedSidecar(sidecarDest, sidecarName) {
		t.Errorf("legitimate sidecar %q was incorrectly identified as orphaned", sidecarName)
	}

	// Plant an orphaned sidecar for a different (non-existent) file.
	orphanedXMP := filepath.Join(filepath.Dir(arwDest), "20211225_999999_deadbeef.arw.xmp")
	plantSidecar(t, orphanedXMP, "orphaned")

	// Simulate clean: walk and remove only orphans.
	var removedOrphans int
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		name := d.Name()
		if isTempFile(name) || isOrphanedSidecar(path, name) {
			removedOrphans++
			_ = os.Remove(path)
		}
		return nil
	})

	if removedOrphans != 1 {
		t.Errorf("removed %d orphans, want 1", removedOrphans)
	}

	// Legitimate ARW + sidecar should still exist.
	if _, err := os.Stat(arwDest); err != nil {
		t.Errorf("ARW file missing after clean: %v", err)
	}
	if _, err := os.Stat(sidecarDest); err != nil {
		t.Errorf("ARW sidecar missing after clean: %v", err)
	}

	// Orphaned sidecar should be gone.
	if _, err := os.Stat(orphanedXMP); !os.IsNotExist(err) {
		t.Errorf("orphaned XMP sidecar still exists after clean")
	}
}
