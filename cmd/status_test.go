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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	"github.com/cwlls/pixe-go/internal/ignore"
	"github.com/cwlls/pixe-go/internal/manifest"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fixturesDir returns the path to the JPEG test fixtures.
func statusFixturesDir() string {
	return filepath.Join("..", "internal", "handler", "jpeg", "testdata")
}

// copyJPEG copies a JPEG fixture into dir with the given destination name.
func copyJPEG(t *testing.T, dir, srcName, dstName string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(statusFixturesDir(), srcName))
	if err != nil {
		t.Fatalf("copyJPEG: read %q: %v", srcName, err)
	}
	if err := os.WriteFile(filepath.Join(dir, dstName), data, 0o644); err != nil {
		t.Fatalf("copyJPEG: write %q: %v", dstName, err)
	}
}

// writeTxtFile writes a plain text file (unrecognized by any handler).
func writeTxtFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("writeTxtFile %q: %v", name, err)
	}
}

// writeLedger writes a JSONL ledger to dirA/.pixe_ledger.json.
func writeLedger(t *testing.T, dirA string, header domain.LedgerHeader, entries []domain.LedgerEntry) {
	t.Helper()
	lw, err := manifest.NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("writeLedger: open: %v", err)
	}
	for _, e := range entries {
		if err := lw.WriteEntry(e); err != nil {
			t.Fatalf("writeLedger: write entry: %v", err)
		}
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("writeLedger: close: %v", err)
	}
}

// defaultHeader returns a minimal valid LedgerHeader for tests.
func defaultHeader() domain.LedgerHeader {
	return domain.LedgerHeader{
		Version:     4,
		RunID:       "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		PixeVersion: "test",
		PixeRun:     "2026-03-06T10:30:00Z",
		Algorithm:   "sha1",
		Destination: "/tmp/archive",
		Recursive:   false,
	}
}

// classify runs the core classification logic and returns a statusResult.
// It mirrors the logic in runStatus without going through Cobra.
func classify(t *testing.T, dirA string, recursive bool) *statusResult {
	t.Helper()

	reg := buildStatusRegistry()

	walkOpts := discovery.WalkOptions{
		Recursive: recursive,
		Ignore:    ignore.New(nil), // activates the hardcoded ledger-file exclusion
	}
	discovered, skipped, err := discovery.Walk(dirA, reg, walkOpts)
	if err != nil {
		t.Fatalf("classify: walk: %v", err)
	}

	lc, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("classify: load ledger: %v", err)
	}

	var ledgerEntryCount int
	if lc != nil {
		ledgerEntryCount = len(lc.Entries)
	}
	ledgerMap := make(map[string]domain.LedgerEntry, ledgerEntryCount)
	if lc != nil {
		for _, e := range lc.Entries {
			ledgerMap[e.Path] = e
		}
	}

	var sorted, duplicates, errored, unsorted []statusFile
	for _, df := range discovered {
		entry, found := ledgerMap[df.RelPath]
		if !found {
			unsorted = append(unsorted, statusFile{Path: df.RelPath})
			continue
		}
		switch entry.Status {
		case domain.LedgerStatusCopy:
			sorted = append(sorted, statusFile{Path: df.RelPath, Destination: entry.Destination})
		case domain.LedgerStatusDuplicate:
			duplicates = append(duplicates, statusFile{Path: df.RelPath, Destination: entry.Destination, Matches: entry.Matches})
		case domain.LedgerStatusError:
			errored = append(errored, statusFile{Path: df.RelPath, Reason: entry.Reason})
		default:
			unsorted = append(unsorted, statusFile{Path: df.RelPath})
		}
	}

	var unrecognized []statusFile
	for _, sf := range skipped {
		unrecognized = append(unrecognized, statusFile{Path: sf.Path, Reason: sf.Reason})
	}

	return &statusResult{
		Source:       dirA,
		Ledger:       lc,
		Sorted:       sorted,
		Duplicates:   duplicates,
		Errored:      errored,
		Unsorted:     unsorted,
		Unrecognized: unrecognized,
	}
}

// buildStatusRegistry returns a registry with only the JPEG handler — sufficient
// for unit tests that use JPEG fixtures and .txt files.
func buildStatusRegistry() *discovery.Registry {
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	return reg
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestRunStatus_emptyDirectory verifies that an empty source dir produces a
// zero-total summary with no sections.
func TestRunStatus_emptyDirectory(t *testing.T) {
	dir := t.TempDir()
	r := classify(t, dir, false)

	if r.total() != 0 {
		t.Errorf("total = %d, want 0", r.total())
	}
	if len(r.Unsorted) != 0 {
		t.Errorf("unsorted = %d, want 0", len(r.Unsorted))
	}

	var buf bytes.Buffer
	printStatusTable(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "none found") {
		t.Errorf("expected 'none found' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "0 total") {
		t.Errorf("expected '0 total' in summary, got:\n%s", out)
	}
}

// TestRunStatus_noLedger verifies that recognized files with no ledger are
// all classified as unsorted.
func TestRunStatus_noLedger(t *testing.T) {
	dir := t.TempDir()
	copyJPEG(t, dir, "with_exif_date.jpg", "IMG_0001.jpg")
	copyJPEG(t, dir, "with_exif_date2.jpg", "IMG_0002.jpg")

	r := classify(t, dir, false)

	if len(r.Unsorted) != 2 {
		t.Errorf("unsorted = %d, want 2", len(r.Unsorted))
	}
	if len(r.Sorted) != 0 {
		t.Errorf("sorted = %d, want 0", len(r.Sorted))
	}
	if r.Ledger != nil {
		t.Error("Ledger should be nil when no ledger file exists")
	}

	var buf bytes.Buffer
	printStatusTable(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "none found") {
		t.Errorf("expected 'none found' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "UNSORTED") {
		t.Errorf("expected UNSORTED section, got:\n%s", out)
	}
	if !strings.Contains(out, "2 unsorted") {
		t.Errorf("expected '2 unsorted' in summary, got:\n%s", out)
	}
}

// TestRunStatus_allSorted verifies that files with "copy" ledger entries are
// classified as sorted and show their destination.
func TestRunStatus_allSorted(t *testing.T) {
	dir := t.TempDir()
	copyJPEG(t, dir, "with_exif_date.jpg", "IMG_0001.jpg")
	copyJPEG(t, dir, "with_exif_date2.jpg", "IMG_0002.jpg")

	writeLedger(t, dir, defaultHeader(), []domain.LedgerEntry{
		{Path: "IMG_0001.jpg", Status: domain.LedgerStatusCopy, Destination: "2021/12-Dec/20211225_062223_abc.jpg"},
		{Path: "IMG_0002.jpg", Status: domain.LedgerStatusCopy, Destination: "2022/02-Feb/20220202_123101_def.jpg"},
	})

	r := classify(t, dir, false)

	if len(r.Sorted) != 2 {
		t.Errorf("sorted = %d, want 2", len(r.Sorted))
	}
	if len(r.Unsorted) != 0 {
		t.Errorf("unsorted = %d, want 0", len(r.Unsorted))
	}

	// Check destinations are preserved.
	if r.Sorted[0].Destination != "2021/12-Dec/20211225_062223_abc.jpg" {
		t.Errorf("sorted[0].Destination = %q, want 2021/12-Dec/...", r.Sorted[0].Destination)
	}

	var buf bytes.Buffer
	printStatusTable(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "SORTED") {
		t.Errorf("expected SORTED section, got:\n%s", out)
	}
	if !strings.Contains(out, "2 sorted") {
		t.Errorf("expected '2 sorted' in summary, got:\n%s", out)
	}
	if strings.Contains(out, "UNSORTED") {
		t.Errorf("unexpected UNSORTED section in output:\n%s", out)
	}
	if !strings.Contains(out, "→ 2021/12-Dec/20211225_062223_abc.jpg") {
		t.Errorf("expected destination arrow in output, got:\n%s", out)
	}
}

// TestRunStatus_mixedStatus verifies correct classification across all five
// categories when the ledger contains a mix of outcomes.
func TestRunStatus_mixedStatus(t *testing.T) {
	dir := t.TempDir()
	// sorted
	copyJPEG(t, dir, "with_exif_date.jpg", "sorted.jpg")
	// duplicate
	copyJPEG(t, dir, "with_exif_date2.jpg", "dupe.jpg")
	// errored
	copyJPEG(t, dir, "no_exif.jpg", "errored.jpg")
	// unsorted (no ledger entry)
	copyJPEG(t, dir, "with_exif_date.jpg", "unsorted.jpg")
	// unrecognized
	writeTxtFile(t, dir, "notes.txt")

	writeLedger(t, dir, defaultHeader(), []domain.LedgerEntry{
		{Path: "sorted.jpg", Status: domain.LedgerStatusCopy, Destination: "2021/12-Dec/sorted.jpg"},
		{Path: "dupe.jpg", Status: domain.LedgerStatusDuplicate, Matches: "2022/02-Feb/original.jpg"},
		{Path: "errored.jpg", Status: domain.LedgerStatusError, Reason: "EXIF parse failed"},
	})

	r := classify(t, dir, false)

	if len(r.Sorted) != 1 {
		t.Errorf("sorted = %d, want 1", len(r.Sorted))
	}
	if len(r.Duplicates) != 1 {
		t.Errorf("duplicates = %d, want 1", len(r.Duplicates))
	}
	if len(r.Errored) != 1 {
		t.Errorf("errored = %d, want 1", len(r.Errored))
	}
	if len(r.Unsorted) != 1 {
		t.Errorf("unsorted = %d, want 1", len(r.Unsorted))
	}
	if len(r.Unrecognized) != 1 {
		t.Errorf("unrecognized = %d, want 1", len(r.Unrecognized))
	}
	if r.total() != 5 {
		t.Errorf("total = %d, want 5", r.total())
	}

	// Check specific fields.
	if r.Duplicates[0].Matches != "2022/02-Feb/original.jpg" {
		t.Errorf("duplicate Matches = %q, want 2022/02-Feb/original.jpg", r.Duplicates[0].Matches)
	}
	if r.Errored[0].Reason != "EXIF parse failed" {
		t.Errorf("errored Reason = %q, want 'EXIF parse failed'", r.Errored[0].Reason)
	}

	var buf bytes.Buffer
	printStatusTable(&buf, r)
	out := buf.String()

	for _, section := range []string{"SORTED", "DUPLICATE", "ERRORED", "UNSORTED", "UNRECOGNIZED"} {
		if !strings.Contains(out, section) {
			t.Errorf("expected %s section in output, got:\n%s", section, out)
		}
	}
	if !strings.Contains(out, "5 total") {
		t.Errorf("expected '5 total' in summary, got:\n%s", out)
	}
}

// TestRunStatus_unrecognizedFiles verifies that .txt files are classified as
// unrecognized while .jpg files are classified normally.
func TestRunStatus_unrecognizedFiles(t *testing.T) {
	dir := t.TempDir()
	copyJPEG(t, dir, "with_exif_date.jpg", "photo.jpg")
	writeTxtFile(t, dir, "readme.txt")
	writeTxtFile(t, dir, "notes.txt")

	writeLedger(t, dir, defaultHeader(), []domain.LedgerEntry{
		{Path: "photo.jpg", Status: domain.LedgerStatusCopy, Destination: "2021/12-Dec/photo.jpg"},
	})

	r := classify(t, dir, false)

	if len(r.Sorted) != 1 {
		t.Errorf("sorted = %d, want 1", len(r.Sorted))
	}
	if len(r.Unrecognized) != 2 {
		t.Errorf("unrecognized = %d, want 2", len(r.Unrecognized))
	}
	if len(r.Unsorted) != 0 {
		t.Errorf("unsorted = %d, want 0", len(r.Unsorted))
	}

	// Verify unrecognized reason contains the extension.
	for _, f := range r.Unrecognized {
		if !strings.Contains(f.Reason, ".txt") {
			t.Errorf("unrecognized reason %q should mention .txt", f.Reason)
		}
	}
}

// TestRunStatus_recursive verifies that files in subdirectories are discovered
// and classified when recursive=true.
func TestRunStatus_recursive(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "vacation")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	copyJPEG(t, dir, "with_exif_date.jpg", "top.jpg")
	copyJPEG(t, subdir, "with_exif_date2.jpg", "sub.jpg")

	writeLedger(t, dir, domain.LedgerHeader{
		Version:     4,
		RunID:       "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		PixeVersion: "test",
		PixeRun:     "2026-03-06T10:30:00Z",
		Algorithm:   "sha1",
		Destination: "/tmp/archive",
		Recursive:   true,
	}, []domain.LedgerEntry{
		{Path: "top.jpg", Status: domain.LedgerStatusCopy, Destination: "2021/12-Dec/top.jpg"},
		{Path: "vacation/sub.jpg", Status: domain.LedgerStatusCopy, Destination: "2022/02-Feb/sub.jpg"},
	})

	r := classify(t, dir, true /* recursive */)

	if len(r.Sorted) != 2 {
		t.Errorf("sorted = %d, want 2 (recursive)", len(r.Sorted))
	}
	if len(r.Unsorted) != 0 {
		t.Errorf("unsorted = %d, want 0", len(r.Unsorted))
	}

	// Verify the subdirectory file has the correct relative path.
	found := false
	for _, f := range r.Sorted {
		if f.Path == "vacation/sub.jpg" {
			found = true
		}
	}
	if !found {
		t.Error("expected vacation/sub.jpg in sorted files")
	}
}

// TestRunStatus_recursiveNonRecursive verifies that subdirectory files are NOT
// discovered when recursive=false.
func TestRunStatus_recursiveNonRecursive(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "vacation")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	copyJPEG(t, dir, "with_exif_date.jpg", "top.jpg")
	copyJPEG(t, subdir, "with_exif_date2.jpg", "sub.jpg")

	r := classify(t, dir, false /* non-recursive */)

	// Only top.jpg should be discovered.
	if r.total() != 1 {
		t.Errorf("total = %d, want 1 (non-recursive should skip subdirs)", r.total())
	}
}

// TestRunStatus_skipStatusTreatedAsUnsorted verifies that ledger entries with
// status "skip" are treated as unsorted (not sorted).
func TestRunStatus_skipStatusTreatedAsUnsorted(t *testing.T) {
	dir := t.TempDir()
	copyJPEG(t, dir, "with_exif_date.jpg", "photo.jpg")

	writeLedger(t, dir, defaultHeader(), []domain.LedgerEntry{
		{Path: "photo.jpg", Status: domain.LedgerStatusSkip, Reason: "previously imported"},
	})

	r := classify(t, dir, false)

	if len(r.Unsorted) != 1 {
		t.Errorf("unsorted = %d, want 1 (skip status should be treated as unsorted)", len(r.Unsorted))
	}
	if len(r.Sorted) != 0 {
		t.Errorf("sorted = %d, want 0", len(r.Sorted))
	}
}

// TestPrintStatusTable_summaryLineFormat verifies the summary line format and
// that zero-count categories are omitted.
func TestPrintStatusTable_summaryLineFormat(t *testing.T) {
	r := &statusResult{
		Source: "/photos",
		Sorted: []statusFile{
			{Path: "a.jpg", Destination: "2021/12-Dec/a.jpg"},
		},
		Duplicates:   nil,
		Errored:      nil,
		Unsorted:     []statusFile{{Path: "b.jpg"}, {Path: "c.jpg"}},
		Unrecognized: nil,
	}

	line := buildSummaryLine(r)

	if !strings.Contains(line, "3 total") {
		t.Errorf("summary should contain '3 total', got: %q", line)
	}
	if !strings.Contains(line, "1 sorted") {
		t.Errorf("summary should contain '1 sorted', got: %q", line)
	}
	if !strings.Contains(line, "2 unsorted") {
		t.Errorf("summary should contain '2 unsorted', got: %q", line)
	}
	// Zero-count categories should be absent.
	if strings.Contains(line, "duplicates") {
		t.Errorf("summary should not contain 'duplicates' when count is 0, got: %q", line)
	}
	if strings.Contains(line, "errored") {
		t.Errorf("summary should not contain 'errored' when count is 0, got: %q", line)
	}
	if strings.Contains(line, "unrecognized") {
		t.Errorf("summary should not contain 'unrecognized' when count is 0, got: %q", line)
	}
}

// TestPrintStatusTable_singularPlural verifies "file" vs "files" noun.
func TestPrintStatusTable_singularPlural(t *testing.T) {
	r := &statusResult{
		Source:   "/photos",
		Unsorted: []statusFile{{Path: "a.jpg"}},
	}
	var buf bytes.Buffer
	printStatusTable(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "UNSORTED (1 file)") {
		t.Errorf("expected singular 'file', got:\n%s", out)
	}

	r2 := &statusResult{
		Source:   "/photos",
		Unsorted: []statusFile{{Path: "a.jpg"}, {Path: "b.jpg"}},
	}
	var buf2 bytes.Buffer
	printStatusTable(&buf2, r2)
	out2 := buf2.String()

	if !strings.Contains(out2, "UNSORTED (2 files)") {
		t.Errorf("expected plural 'files', got:\n%s", out2)
	}
}

// TestPrintStatusJSON_structure verifies the JSON output structure and that
// empty arrays serialize as [] not null.
func TestPrintStatusJSON_structure(t *testing.T) {
	r := &statusResult{
		Source: "/photos",
		Ledger: &manifest.LedgerContents{
			Header: domain.LedgerHeader{
				RunID:       "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				PixeVersion: "0.10.0",
				PixeRun:     "2026-03-06T10:30:00Z",
				Recursive:   false,
			},
		},
		Sorted:       []statusFile{{Path: "a.jpg", Destination: "2021/12-Dec/a.jpg"}},
		Duplicates:   nil, // should serialize as []
		Errored:      nil, // should serialize as []
		Unsorted:     []statusFile{{Path: "b.jpg"}},
		Unrecognized: nil, // should serialize as []
	}

	var buf bytes.Buffer
	if err := printStatusJSON(&buf, r); err != nil {
		t.Fatalf("printStatusJSON: %v", err)
	}

	// Unmarshal and verify structure.
	var out statusJSON
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	if out.Source != "/photos" {
		t.Errorf("source = %q, want /photos", out.Source)
	}
	if out.Ledger == nil {
		t.Error("ledger should not be null when ledger exists")
	}
	if out.Ledger.RunID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("ledger.run_id = %q", out.Ledger.RunID)
	}
	if len(out.Sorted) != 1 {
		t.Errorf("sorted len = %d, want 1", len(out.Sorted))
	}
	if out.Sorted[0].Destination != "2021/12-Dec/a.jpg" {
		t.Errorf("sorted[0].destination = %q", out.Sorted[0].Destination)
	}
	if len(out.Unsorted) != 1 {
		t.Errorf("unsorted len = %d, want 1", len(out.Unsorted))
	}
	if out.Summary.Total != 2 {
		t.Errorf("summary.total = %d, want 2", out.Summary.Total)
	}
	if out.Summary.Sorted != 1 {
		t.Errorf("summary.sorted = %d, want 1", out.Summary.Sorted)
	}
	if out.Summary.Unsorted != 1 {
		t.Errorf("summary.unsorted = %d, want 1", out.Summary.Unsorted)
	}

	// Verify empty arrays are [] not null in the raw JSON.
	raw := buf.String()
	if strings.Contains(raw, `"duplicates": null`) {
		t.Error("duplicates should serialize as [] not null")
	}
	if strings.Contains(raw, `"errored": null`) {
		t.Error("errored should serialize as [] not null")
	}
	if strings.Contains(raw, `"unrecognized": null`) {
		t.Error("unrecognized should serialize as [] not null")
	}
}

// TestRunStatus_defaultsToCwd verifies that omitting --source causes runStatus
// to inspect the current working directory.
func TestRunStatus_defaultsToCwd(t *testing.T) {
	dir := t.TempDir()
	copyJPEG(t, dir, "with_exif_date.jpg", "IMG_0001.jpg")

	// Change working directory to the temp dir; restore on cleanup.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}

	// Call runStatus with source unset (empty string from Viper default).
	// statusCmd.OutOrStdout() needs a writer; set it on the command.
	var buf bytes.Buffer
	statusCmd.SetOut(&buf)
	defer statusCmd.SetOut(nil)

	// Invoke runStatus directly — source is "" so cwd fallback fires.
	if err := runStatus(statusCmd, nil); err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	out := buf.String()
	// The output header must reference the temp dir (the cwd).
	if !strings.Contains(out, dir) {
		t.Errorf("expected output to reference cwd %q, got:\n%s", dir, out)
	}
	// The JPEG should appear as unsorted (no ledger).
	if !strings.Contains(out, "UNSORTED") {
		t.Errorf("expected UNSORTED section, got:\n%s", out)
	}
}

// TestRunStatus_sourceOverridesCwd verifies that an explicit --source flag
// takes precedence over the current working directory.
func TestRunStatus_sourceOverridesCwd(t *testing.T) {
	// Two distinct temp dirs: cwd and the explicit source.
	cwdDir := t.TempDir()
	srcDir := t.TempDir()
	copyJPEG(t, srcDir, "with_exif_date.jpg", "IMG_explicit.jpg")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatalf("chdir %q: %v", cwdDir, err)
	}

	// Simulate --source being set to srcDir via Viper.
	viper.Set("status_source", srcDir)
	defer viper.Set("status_source", "")

	var buf bytes.Buffer
	statusCmd.SetOut(&buf)
	defer statusCmd.SetOut(nil)

	if err := runStatus(statusCmd, nil); err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	out := buf.String()
	// Output must reference srcDir, not cwdDir.
	if !strings.Contains(out, srcDir) {
		t.Errorf("expected output to reference explicit source %q, got:\n%s", srcDir, out)
	}
	if strings.Contains(out, cwdDir) {
		t.Errorf("output should not reference cwd %q when --source is explicit, got:\n%s", cwdDir, out)
	}
}

// TestPrintStatusJSON_noLedger verifies that the ledger field is null when no
// ledger exists.
func TestPrintStatusJSON_noLedger(t *testing.T) {
	r := &statusResult{
		Source:   "/photos",
		Ledger:   nil,
		Unsorted: []statusFile{{Path: "a.jpg"}},
	}

	var buf bytes.Buffer
	if err := printStatusJSON(&buf, r); err != nil {
		t.Fatalf("printStatusJSON: %v", err)
	}

	var out statusJSON
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	if out.Ledger != nil {
		t.Error("ledger should be null when no ledger exists")
	}
}

// TestPrintStatusTable_ledgerHeader verifies the ledger header line format.
func TestPrintStatusTable_ledgerHeader(t *testing.T) {
	r := &statusResult{
		Source: "/photos",
		Ledger: &manifest.LedgerContents{
			Header: domain.LedgerHeader{
				RunID:     "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				PixeRun:   "2026-03-06T10:30:00Z",
				Recursive: true,
			},
		},
	}

	var buf bytes.Buffer
	printStatusTable(&buf, r)
	out := buf.String()

	// Run ID should be truncated to 8 chars.
	if !strings.Contains(out, "run a1b2c3d4") {
		t.Errorf("expected truncated run ID 'a1b2c3d4', got:\n%s", out)
	}
	if !strings.Contains(out, "recursive: yes") {
		t.Errorf("expected 'recursive: yes', got:\n%s", out)
	}
}
