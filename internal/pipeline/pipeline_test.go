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

package pipeline

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/manifest"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
)

// TestMain pins the locale to English so that month directory assertions are
// deterministic regardless of the developer's system locale.
func TestMain(m *testing.M) {
	pathbuilder.SetLocaleForTesting(language.English)
	os.Exit(m.Run())
}

// --- helpers ---

// newOpts builds a SortOptions wired to a real JPEG handler and SHA-1 hasher.
func newOpts(t *testing.T, cfg *config.AppConfig, out *bytes.Buffer) SortOptions {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())

	return SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: "20260306_103000",
		Output:       out,
		PixeVersion:  "test",
	}
}

// copyFixture copies a file from the jpeg testdata directory into dir.
func copyFixture(t *testing.T, dir, name string) string {
	t.Helper()
	src := filepath.Join("..", "handler", "jpeg", "testdata", name)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("copyFixture read %q: %v", src, err)
	}
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("copyFixture write %q: %v", dst, err)
	}
	return dst
}

// --- Tests ---

func TestRun_basicSort(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "with_exif_date.jpg")
	copyFixture(t, dirA, "with_exif_date2.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	result, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v\nOutput:\n%s", err, out.String())
	}

	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0\nOutput:\n%s", result.Errors, out.String())
	}
}

func TestRun_outputDirectoryStructure(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "with_exif_date.jpg") // date: 2021-12-25

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	_, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Expect a file under dirB/2021/12-Dec/
	yearDir := filepath.Join(dirB, "2021")
	if _, err := os.Stat(yearDir); err != nil {
		t.Errorf("year directory %q not created: %v", yearDir, err)
	}
	monthDir := filepath.Join(dirB, "2021", "12-Dec")
	if _, err := os.Stat(monthDir); err != nil {
		t.Errorf("month directory %q not created: %v", monthDir, err)
	}

	// Find the file in the month directory.
	entries, err := os.ReadDir(monthDir)
	if err != nil {
		t.Fatalf("ReadDir %q: %v", monthDir, err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file in month dir, got %d", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, "20211225_062223_") {
		t.Errorf("filename %q does not start with expected date prefix 20211225_062223_", name)
	}
	if !strings.HasSuffix(name, ".jpg") {
		t.Errorf("filename %q does not end with .jpg", name)
	}
}

func TestRun_noExifFallbackDate(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "no_exif.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	result, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1", result.Processed)
	}

	// File should land under 1902/02-Feb/ (Ansel Adams fallback).
	monthDir := filepath.Join(dirB, "1902", "02-Feb")
	entries, err := os.ReadDir(monthDir)
	if err != nil {
		t.Fatalf("ReadDir %q: %v", monthDir, err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file in 1902/02-Feb/, got %d", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "19020220_000000_") {
		t.Errorf("filename %q should start with Ansel Adams prefix 19020220_000000_", entries[0].Name())
	}
}

func TestRun_duplicateRouting(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Copy the same file twice under different names — same content → same checksum.
	copyFixture(t, dirA, "with_exif_date.jpg")
	src := filepath.Join("..", "handler", "jpeg", "testdata", "with_exif_date.jpg")
	data, _ := os.ReadFile(src)
	if err := os.WriteFile(filepath.Join(dirA, "duplicate.jpg"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	result, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1\nOutput:\n%s", result.Duplicates, out.String())
	}

	// Duplicate must be under dirB/duplicates/
	dupDir := filepath.Join(dirB, "duplicates")
	if _, err := os.Stat(dupDir); err != nil {
		t.Errorf("duplicates directory not created: %v", err)
	}
}

func TestRun_ledgerWritten_withEntry(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "with_exif_date.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	_, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	l, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if l == nil {
		t.Fatal("ledger not written to dirA")
	}
	if len(l.Files) != 1 {
		t.Errorf("ledger.Files len = %d, want 1", len(l.Files))
	}
	if l.Files[0].Checksum == "" {
		t.Error("ledger entry checksum should not be empty")
	}
	if l.Files[0].Destination == "" {
		t.Error("ledger entry destination should not be empty")
	}
}

func TestRun_ledgerWritten(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "with_exif_date.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	_, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	l, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if l == nil {
		t.Fatal("ledger not written to dirA")
	}
	if len(l.Files) != 1 {
		t.Errorf("ledger.Files len = %d, want 1", len(l.Files))
	}
	if l.Files[0].Checksum == "" {
		t.Error("ledger entry checksum should not be empty")
	}
}

func TestRun_ledgerVersion3WithRunID(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "with_exif_date.jpg")

	const wantRunID = "test-run-id-12345"
	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	opts := newOpts(t, cfg, &out)
	opts.RunID = wantRunID

	_, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	l, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if l == nil {
		t.Fatal("ledger not written to dirA")
	}
	if l.Version != 3 {
		t.Errorf("ledger.Version = %d, want 3", l.Version)
	}
	if l.RunID != wantRunID {
		t.Errorf("ledger.RunID = %q, want %q", l.RunID, wantRunID)
	}
}

func TestRun_sourceUntouched(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	srcFile := copyFixture(t, dirA, "with_exif_date.jpg")

	// Record source checksum before sort.
	h, _ := hash.NewHasher("sha1")
	srcData, _ := os.ReadFile(srcFile)
	beforeChecksum, _ := h.Sum(bytes.NewReader(srcData))

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	_, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Source file must be byte-for-byte identical after sort.
	srcDataAfter, _ := os.ReadFile(srcFile)
	afterChecksum, _ := h.Sum(bytes.NewReader(srcDataAfter))
	if beforeChecksum != afterChecksum {
		t.Error("source file was modified during sort — safety violation")
	}

	// Only .pixe_ledger.json should be new in dirA.
	entries, _ := os.ReadDir(dirA)
	for _, e := range entries {
		if e.Name() != "with_exif_date.jpg" && e.Name() != ".pixe_ledger.json" {
			t.Errorf("unexpected file in dirA after sort: %q", e.Name())
		}
	}
}

func TestRun_dryRun_noFilesCreated(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "with_exif_date.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1", DryRun: true}
	result, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}

	// No media files should be created in dirB.
	_ = filepath.Walk(dirB, func(path string, info os.FileInfo, _ error) error {
		if !info.IsDir() && !strings.Contains(path, ".pixe") {
			t.Errorf("dry-run created unexpected file in dirB: %q", path)
		}
		return nil
	})

	// Output should mention DRY-RUN.
	if !strings.Contains(out.String(), "DRY-RUN") {
		t.Error("dry-run output should contain 'DRY-RUN'")
	}
	_ = result
}

func TestRun_resume_skipsCompleted(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, "with_exif_date.jpg")
	copyFixture(t, dirA, "with_exif_date2.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	opts := newOpts(t, cfg, &out)

	// First run — processes both files.
	result1, err := Run(opts)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if result1.Processed != 2 {
		t.Fatalf("first run: Processed = %d, want 2", result1.Processed)
	}

	// Second run — manifest already has both files as complete; should skip.
	out.Reset()
	result2, err := Run(opts)
	if err != nil {
		t.Fatalf("second Run (resume): %v", err)
	}
	if result2.Processed != 2 {
		t.Errorf("resume: Processed = %d, want 2 (skipped from manifest)", result2.Processed)
	}
	if result2.Errors != 0 {
		t.Errorf("resume: Errors = %d, want 0", result2.Errors)
	}
}

func TestRun_skippedFilesReported(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Write a non-JPEG file — should be skipped.
	if err := os.WriteFile(filepath.Join(dirA, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, dirA, "with_exif_date.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	result, err := Run(newOpts(t, cfg, &out))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (readme.txt)", result.Skipped)
	}
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1", result.Processed)
	}
}

func TestRenderCopyright(t *testing.T) {
	cases := []struct {
		tmpl string
		year int
		want string
	}{
		{"Copyright {{.Year}} My Family", 2021, "Copyright 2021 My Family"},
		{"Copyright {{.Year}} My Family", 2026, "Copyright 2026 My Family"},
		{"No template here", 2021, "No template here"},
		{"", 2021, ""},
	}
	for _, tc := range cases {
		date := time.Date(tc.year, 1, 1, 0, 0, 0, 0, time.UTC)
		got := renderCopyright(tc.tmpl, date)
		if got != tc.want {
			t.Errorf("renderCopyright(%q, %d) = %q, want %q", tc.tmpl, tc.year, got, tc.want)
		}
	}
}
