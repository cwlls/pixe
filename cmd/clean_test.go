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

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/cwlls/pixe/internal/archivedb"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeFile creates a file with the given content at the specified path,
// creating parent directories as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// fileExists returns true if the path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// createTestDB creates a valid archive database at dirB/.pixe/pixe.db with
// a completed run. Returns the DB path.
func createTestDB(t *testing.T, dirB string) string {
	t.Helper()
	dbPath := filepath.Join(dirB, ".pixe", "pixe.db")
	db, err := archivedb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open test DB: %v", err)
	}
	r := &archivedb.Run{
		ID:          "clean-test-run",
		PixeVersion: "test",
		Source:      "/src",
		Destination: dirB,
		Algorithm:   "sha1",
		Workers:     1,
		StartedAt:   time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
	}
	if err := db.InsertRun(r); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}
	if err := db.CompleteRun("clean-test-run", time.Date(2026, 3, 1, 10, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}
	_ = db.Close()
	return dbPath
}

// createRunningDB creates a database with a run in status='running'.
func createRunningDB(t *testing.T, dirB string) string {
	t.Helper()
	dbPath := filepath.Join(dirB, ".pixe", "pixe.db")
	db, err := archivedb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open test DB: %v", err)
	}
	r := &archivedb.Run{
		ID:          "running-test-run",
		PixeVersion: "test",
		Source:      "/src",
		Destination: dirB,
		Algorithm:   "sha1",
		Workers:     1,
		StartedAt:   time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
	}
	if err := db.InsertRun(r); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}
	// Do NOT complete the run — leave it as 'running'.
	_ = db.Close()
	return dbPath
}

// resetCleanViper clears all clean-related viper keys.
func resetCleanViper(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		viper.Set("clean_dir", "")
		viper.Set("clean_db_path", "")
		viper.Set("clean_dry_run", false)
		viper.Set("clean_temp_only", false)
		viper.Set("clean_vacuum_only", false)
	})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestCleanCmd_flagValidation verifies --temp-only and --vacuum-only are
// mutually exclusive.
func TestCleanCmd_flagValidation(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	viper.Set("clean_dir", dir)
	viper.Set("clean_temp_only", true)
	viper.Set("clean_vacuum_only", true)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	err := runClean(cleanCmd, nil)
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want it to contain 'mutually exclusive'", err.Error())
	}
}

// TestCleanCmd_tempFileDetection verifies that .pixe-tmp files are detected
// and removed, while normal files are untouched.
func TestCleanCmd_tempFileDetection(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	// Create directory structure.
	monthDir := filepath.Join(dir, "2021", "12-Dec")

	// Normal file — should NOT be removed.
	normalFile := filepath.Join(monthDir, "20211225_062223_abc123def456.jpg")
	writeFile(t, normalFile, "jpeg data")

	// Temp file with random suffix — should be removed.
	tempFile1 := filepath.Join(monthDir, ".20211225_062223_abc123def456.jpg.pixe-tmp-xyz789")
	writeFile(t, tempFile1, "temp data")

	// Temp file without random suffix — should be removed.
	anotherMonth := filepath.Join(dir, "2022", "01-Jan")
	tempFile2 := filepath.Join(anotherMonth, ".file.jpg.pixe-tmp")
	writeFile(t, tempFile2, "temp data 2")

	viper.Set("clean_dir", dir)
	viper.Set("clean_temp_only", true)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	if err := runClean(cleanCmd, nil); err != nil {
		t.Fatalf("runClean: %v", err)
	}

	// Normal file should still exist.
	if !fileExists(normalFile) {
		t.Error("normal file was removed, should be untouched")
	}

	// Temp files should be removed.
	if fileExists(tempFile1) {
		t.Error("temp file 1 should have been removed")
	}
	if fileExists(tempFile2) {
		t.Error("temp file 2 should have been removed")
	}

	out := buf.String()
	if !strings.Contains(out, "REMOVE") {
		t.Errorf("expected REMOVE in output, got:\n%s", out)
	}
	if !strings.Contains(out, ".pixe-tmp") {
		t.Errorf("expected .pixe-tmp in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2 temp files") {
		t.Errorf("expected '2 temp files' in summary, got:\n%s", out)
	}
}

// TestCleanCmd_orphanedSidecarDetection verifies that orphaned XMP sidecars
// are detected and removed, while sidecars with existing media files and
// non-Pixe XMP files are left alone.
func TestCleanCmd_orphanedSidecarDetection(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	monthDir := filepath.Join(dir, "2021", "12-Dec")

	// Media file + sidecar (both present) — sidecar should NOT be removed.
	mediaFile := filepath.Join(monthDir, "20211225_062223_abc123def456789012345678901234567890.arw")
	sidecarOK := filepath.Join(monthDir, "20211225_062223_abc123def456789012345678901234567890.arw.xmp")
	writeFile(t, mediaFile, "raw data")
	writeFile(t, sidecarOK, "xmp data")

	// Orphaned sidecar (no corresponding media file) — should be removed.
	orphanedSidecar := filepath.Join(monthDir, "20211225_071500_a3b4c5d6e7f8901234567890abcdef1234567890.arw.xmp")
	writeFile(t, orphanedSidecar, "orphaned xmp data")

	// Non-Pixe XMP file — should NOT be removed.
	nonPixeXMP := filepath.Join(monthDir, "notes.xmp")
	writeFile(t, nonPixeXMP, "user xmp data")

	viper.Set("clean_dir", dir)
	viper.Set("clean_temp_only", true)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	if err := runClean(cleanCmd, nil); err != nil {
		t.Fatalf("runClean: %v", err)
	}

	// OK sidecar should still exist.
	if !fileExists(sidecarOK) {
		t.Error("sidecar with existing media file was removed, should be untouched")
	}

	// Non-Pixe XMP should still exist.
	if !fileExists(nonPixeXMP) {
		t.Error("non-Pixe XMP file was removed, should be untouched")
	}

	// Orphaned sidecar should be removed.
	if fileExists(orphanedSidecar) {
		t.Error("orphaned sidecar should have been removed")
	}

	out := buf.String()
	if !strings.Contains(out, "orphaned sidecar") {
		t.Errorf("expected 'orphaned sidecar' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1 orphaned sidecar") {
		t.Errorf("expected '1 orphaned sidecar' in summary, got:\n%s", out)
	}
}

// TestCleanCmd_dryRunNoModification verifies that --dry-run lists orphaned
// files without actually deleting them.
func TestCleanCmd_dryRunNoModification(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	monthDir := filepath.Join(dir, "2021", "12-Dec")

	tempFile := filepath.Join(monthDir, ".20211225_062223_abc123def456.jpg.pixe-tmp-abc")
	writeFile(t, tempFile, "temp data")

	orphanedSidecar := filepath.Join(monthDir, "20211225_071500_a3b4c5d6e7f8901234567890abcdef1234567890.arw.xmp")
	writeFile(t, orphanedSidecar, "orphaned xmp")

	viper.Set("clean_dir", dir)
	viper.Set("clean_temp_only", true)
	viper.Set("clean_dry_run", true)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	if err := runClean(cleanCmd, nil); err != nil {
		t.Fatalf("runClean: %v", err)
	}

	// Both files should still exist.
	if !fileExists(tempFile) {
		t.Error("temp file was removed during dry-run, should be untouched")
	}
	if !fileExists(orphanedSidecar) {
		t.Error("orphaned sidecar was removed during dry-run, should be untouched")
	}

	out := buf.String()
	if !strings.Contains(out, "WOULD REMOVE") {
		t.Errorf("expected 'WOULD REMOVE' in dry-run output, got:\n%s", out)
	}
	if strings.Contains(out, "\n  REMOVE ") {
		t.Errorf("unexpected 'REMOVE' (without WOULD) in dry-run output, got:\n%s", out)
	}
}

// TestCleanCmd_vacuumActiveRunGuard verifies that VACUUM is refused when an
// active sort run is detected.
func TestCleanCmd_vacuumActiveRunGuard(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	createRunningDB(t, dir)

	viper.Set("clean_dir", dir)
	viper.Set("clean_vacuum_only", true)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	err := runClean(cleanCmd, nil)
	if err == nil {
		t.Fatal("expected error for active run, got nil")
	}
	if !strings.Contains(err.Error(), "cannot vacuum") {
		t.Errorf("error = %q, want it to contain 'cannot vacuum'", err.Error())
	}
	if !strings.Contains(err.Error(), "active sort run") {
		t.Errorf("error = %q, want it to contain 'active sort run'", err.Error())
	}
}

// TestCleanCmd_vacuumSuccess verifies that VACUUM succeeds on a database
// with no active runs.
func TestCleanCmd_vacuumSuccess(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	createTestDB(t, dir)

	viper.Set("clean_dir", dir)
	viper.Set("clean_vacuum_only", true)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	if err := runClean(cleanCmd, nil); err != nil {
		t.Fatalf("runClean: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Size before:") {
		t.Errorf("expected 'Size before:' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Size after:") {
		t.Errorf("expected 'Size after:' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Reclaimed") {
		t.Errorf("expected 'Reclaimed' in output, got:\n%s", out)
	}
}

// TestCleanCmd_noDatabaseSkipsCompaction verifies that when no database exists,
// the compaction step is skipped with a notice (not an error).
func TestCleanCmd_noDatabaseSkipsCompaction(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	// Plant a temp file but no database.
	tempFile := filepath.Join(dir, ".something.pixe-tmp")
	writeFile(t, tempFile, "orphan")

	viper.Set("clean_dir", dir)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	if err := runClean(cleanCmd, nil); err != nil {
		t.Fatalf("runClean: %v", err)
	}

	// Temp file should be removed.
	if fileExists(tempFile) {
		t.Error("temp file should have been removed")
	}

	out := buf.String()
	if !strings.Contains(out, "skipping compaction") {
		t.Errorf("expected 'skipping compaction' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "REMOVE") {
		t.Errorf("expected REMOVE for temp file in output, got:\n%s", out)
	}
}

// TestCleanCmd_noOrphansNoDatabase verifies clean output when there is nothing
// to clean and no database.
func TestCleanCmd_noOrphansNoDatabase(t *testing.T) {
	dir := t.TempDir()
	resetCleanViper(t)

	// Create a normal file only.
	writeFile(t, filepath.Join(dir, "photo.jpg"), "jpeg")

	viper.Set("clean_dir", dir)

	var buf bytes.Buffer
	cleanCmd.SetOut(&buf)
	defer cleanCmd.SetOut(nil)

	if err := runClean(cleanCmd, nil); err != nil {
		t.Fatalf("runClean: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No orphaned files found") {
		t.Errorf("expected 'No orphaned files found' in output, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

// TestIsTempFile verifies the temp file detection logic.
func TestIsTempFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".20211225_062223_abc.jpg.pixe-tmp-xyz", true},
		{".file.jpg.pixe-tmp", true},
		{".something.pixe-tmp-123456", true},
		{"20211225_062223_abc.jpg", false},
		{"photo.jpg", false},
		{"notes.xmp", false},
		{"pixe-tmp", false}, // no leading dot or .pixe-tmp substring
	}
	for _, tt := range tests {
		got := isTempFile(tt.name)
		if got != tt.want {
			t.Errorf("isTempFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestPixeXMPPattern verifies the regex matches Pixe-generated XMP filenames
// and rejects non-Pixe XMP files.
func TestPixeXMPPattern(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"20211225_062223_abc123def456.arw.xmp", true},
		{"20220202_123101_447d3060abc12345.jpg.xmp", true},
		{"20211225_071500_a3b4c5d6e7f890.dng.xmp", true},
		{"notes.xmp", false},                        // no timestamp/checksum
		{"photo.xmp", false},                        // no timestamp/checksum
		{"IMG_0001.jpg.xmp", false},                 // no Pixe naming convention
		{"20211225_062223.jpg.xmp", false},          // no checksum
		{"20211225_062223_GHIJ.arw.xmp", false},     // uppercase non-hex
		{"20211225_062223_abc123def456.arw", false}, // not .xmp
	}
	for _, tt := range tests {
		got := pixeXMPPattern.MatchString(tt.name)
		if got != tt.want {
			t.Errorf("pixeXMPPattern.MatchString(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestFormatBytes verifies human-readable byte formatting.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{999, "999 B"},
		{1000, "1.0 KB"},
		{1500, "1.5 KB"},
		{1000000, "1.0 MB"},
		{12400000, "12.4 MB"},
		{1000000000, "1.0 GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestPluralize verifies singular/plural word formatting.
func TestPluralize(t *testing.T) {
	tests := []struct {
		word string
		n    int
		want string
	}{
		{"file", 0, "files"},
		{"file", 1, "file"},
		{"file", 2, "files"},
		{"sidecar", 1, "sidecar"},
		{"sidecar", 3, "sidecars"},
	}
	for _, tt := range tests {
		got := pluralize(tt.word, tt.n)
		if got != tt.want {
			t.Errorf("pluralize(%q, %d) = %q, want %q", tt.word, tt.n, got, tt.want)
		}
	}
}

// TestTruncateID verifies ID truncation.
func TestTruncateID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"a1b2c3d4-e5f6-7890-abcd-ef1234567890", "a1b2c3d4"},
		{"short", "short"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
	}
	for _, tt := range tests {
		got := truncateID(tt.input)
		if got != tt.want {
			t.Errorf("truncateID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
