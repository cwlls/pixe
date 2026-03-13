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
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/config"
	"github.com/cwlls/pixe/internal/discovery"
	jpeghandler "github.com/cwlls/pixe/internal/handler/jpeg"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/pipeline"
)

// buildIgnoreOpts constructs a SortOptions with the given ignore patterns and
// recursive flag, wired to a real JPEG handler and SHA-1 hasher.
func buildIgnoreOpts(t *testing.T, dirA, dirB string, recursive bool, ignorePatterns []string) pipeline.SortOptions {
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
			Recursive:   recursive,
			Ignore:      ignorePatterns,
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &bytes.Buffer{},
		PixeVersion:  "test",
	}
}

// countFilesInDir returns the number of regular files under root.
func countFilesInDir(t *testing.T, root string) int {
	t.Helper()
	count := 0
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		count++
		return nil
	})
	return count
}

// TestSort_doublestarIgnore verifies that a "**/*.txt" pattern in --ignore
// excludes .txt files at all depths from the sort pipeline. The JPEG files
// at the same depths must still be processed.
func TestSort_doublestarIgnore(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Root-level JPEG and txt.
	copyFixture(t, dirA, fixtureExif1, "photo.jpg")
	if err := os.WriteFile(filepath.Join(dirA, "readme.txt"), []byte("root txt"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Nested JPEG and txt.
	subDir := filepath.Join(dirA, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, subDir, fixtureExif2, "nested.jpg")
	if err := os.WriteFile(filepath.Join(subDir, "notes.txt"), []byte("nested txt"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := buildIgnoreOpts(t, dirA, dirB, true, []string{"**/*.txt"})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Both JPEGs must be processed; both .txt files must be invisible.
	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2 (both JPEGs)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// No .txt files should appear anywhere in dirB.
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, _ error) error {
		if !d.IsDir() && filepath.Ext(d.Name()) == ".txt" {
			t.Errorf("unexpected .txt file in dirB: %q", path)
		}
		return nil
	})
}

// TestSort_directoryIgnore verifies that a "node_modules/" trailing-slash
// pattern causes the entire node_modules directory to be skipped. JPEGs
// outside that directory must still be processed.
func TestSort_directoryIgnore(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// A JPEG at the root — must be processed.
	copyFixture(t, dirA, fixtureExif1, "photo.jpg")

	// A JPEG inside node_modules/ — must be skipped (directory ignored).
	nmDir := filepath.Join(dirA, "node_modules")
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, nmDir, fixtureExif2, "icon.jpg")

	opts := buildIgnoreOpts(t, dirA, dirB, true, []string{"node_modules/"})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Only the root photo.jpg must be processed.
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1 (only root photo.jpg)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// Exactly 1 JPEG in dirB — photo.jpg with EXIF date 2021-12-25.
	jpegFiles := findFiles(t, filepath.Join(dirB, "2021", "12-Dec"), "20211225_062223-")
	if len(jpegFiles) != 1 {
		t.Errorf("expected 1 JPEG in 2021/12-Dec/, got %d", len(jpegFiles))
	}
}

// TestSort_pixeignoreFile verifies that a .pixeignore file placed in dirA is
// loaded automatically during the sort and its patterns are respected.
// Files matching the .pixeignore patterns must not appear in dirB.
func TestSort_pixeignoreFile(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Write a .pixeignore that excludes *.bak files.
	pixeignoreContent := "# Pixe ignore file\n*.bak\n"
	if err := os.WriteFile(filepath.Join(dirA, ".pixeignore"), []byte(pixeignoreContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// A JPEG — must be processed.
	copyFixture(t, dirA, fixtureExif1, "photo.jpg")

	// A .bak file — must be invisible (matched by .pixeignore).
	if err := os.WriteFile(filepath.Join(dirA, "photo.bak"), []byte("backup data"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := buildIgnoreOpts(t, dirA, dirB, true, nil) // no global patterns
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Only the JPEG must be processed.
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1 (only photo.jpg)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// No .bak files in dirB.
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, _ error) error {
		if !d.IsDir() && filepath.Ext(d.Name()) == ".bak" {
			t.Errorf("unexpected .bak file in dirB: %q", path)
		}
		return nil
	})

	// The .pixeignore file itself must not appear in dirB.
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, _ error) error {
		if !d.IsDir() && d.Name() == ".pixeignore" {
			t.Errorf(".pixeignore should not be copied to dirB: %q", path)
		}
		return nil
	})
}

// TestSort_nestedPixeignoreScoping verifies that a .pixeignore in a
// subdirectory only affects files within that subtree. A file at the root
// that matches the nested .pixeignore pattern must still be processed.
func TestSort_nestedPixeignoreScoping(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Root-level JPEG — must be processed (not affected by sub/.pixeignore).
	copyFixture(t, dirA, fixtureExif1, "root.jpg")

	// sub/ directory with its own .pixeignore that excludes *.jpg.
	subDir := filepath.Join(dirA, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, ".pixeignore"), []byte("*.jpg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// sub/excluded.jpg — must be invisible (matched by sub/.pixeignore).
	copyFixture(t, subDir, fixtureExif2, "excluded.jpg")

	// other/ directory with no .pixeignore — its JPEG must be processed.
	otherDir := filepath.Join(dirA, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, otherDir, fixtureExif2, "other.jpg")

	opts := buildIgnoreOpts(t, dirA, dirB, true, nil)
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// root.jpg and other/other.jpg must be processed; sub/excluded.jpg must be invisible.
	// Note: fixtureExif1 and fixtureExif2 have the same image payload, so one will
	// be a duplicate. Both are still "processed" (Processed counts all files handled).
	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2 (root.jpg + other/other.jpg)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// dirB must contain at least 1 file.
	if countFilesInDir(t, dirB) < 1 {
		t.Error("dirB is empty, expected at least 1 file")
	}
}
