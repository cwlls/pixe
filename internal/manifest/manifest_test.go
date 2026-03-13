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

package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/domain"
)

// sampleManifest returns a populated Manifest for use in tests.
func sampleManifest(dirB string) *domain.Manifest {
	now := time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC)
	verified := now.Add(2 * time.Second)
	return &domain.Manifest{
		Version:     1,
		PixeVersion: "test",
		Source:      "/tmp/source",
		Destination: dirB,
		Algorithm:   "sha1",
		StartedAt:   now,
		Workers:     4,
		Files: []*domain.ManifestEntry{
			{
				Source:      "/tmp/source/IMG_0001.jpg",
				Destination: filepath.Join(dirB, "2021/12/20211225_062223_abc123.jpg"),
				Checksum:    "abc123",
				Status:      domain.StatusComplete,
				VerifiedAt:  &verified,
			},
		},
	}
}

// sampleLedgerHeader returns a LedgerHeader for use in tests.
func sampleLedgerHeader() domain.LedgerHeader {
	return domain.LedgerHeader{
		Version:     4,
		RunID:       "test-run-id",
		PixeVersion: "test",
		PixeRun:     "2026-03-06T10:30:00Z",
		Algorithm:   "sha1",
		Destination: "/tmp/dest",
		Recursive:   false,
	}
}

// writeSampleLedger writes a header + one copy entry to dirA and returns
// the header and entry for assertion.
func writeSampleLedger(t *testing.T, dirA string) (domain.LedgerHeader, domain.LedgerEntry) {
	t.Helper()
	now := time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC)
	header := sampleLedgerHeader()
	entry := domain.LedgerEntry{
		Path:        "IMG_0001.jpg",
		Status:      domain.LedgerStatusCopy,
		Checksum:    "abc123",
		Destination: "2021/12/20211225_062223_abc123.jpg",
		VerifiedAt:  &now,
	}
	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	if err := lw.WriteEntry(entry); err != nil {
		t.Fatalf("WriteEntry: %v", err)
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return header, entry
}

// --- Manifest tests ---

func TestManifest_SaveLoad_roundtrip(t *testing.T) {
	dirB := t.TempDir()
	m := sampleManifest(dirB)

	if err := Save(m, dirB); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify the .pixe directory was created.
	if _, err := os.Stat(filepath.Join(dirB, ".pixe")); err != nil {
		t.Fatalf(".pixe directory not created: %v", err)
	}

	// Verify the manifest file exists.
	if _, err := os.Stat(manifestPath(dirB)); err != nil {
		t.Fatalf("manifest file not created: %v", err)
	}

	got, err := Load(dirB)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("Load returned nil, expected manifest")
	}

	// Structural equality checks.
	if got.Version != m.Version {
		t.Errorf("Version: got %d, want %d", got.Version, m.Version)
	}
	if got.PixeVersion != m.PixeVersion {
		t.Errorf("PixeVersion: got %q, want %q", got.PixeVersion, m.PixeVersion)
	}
	if got.Source != m.Source {
		t.Errorf("Source: got %q, want %q", got.Source, m.Source)
	}
	if got.Algorithm != m.Algorithm {
		t.Errorf("Algorithm: got %q, want %q", got.Algorithm, m.Algorithm)
	}
	if got.Workers != m.Workers {
		t.Errorf("Workers: got %d, want %d", got.Workers, m.Workers)
	}
	if len(got.Files) != len(m.Files) {
		t.Fatalf("Files len: got %d, want %d", len(got.Files), len(m.Files))
	}
	if got.Files[0].Checksum != m.Files[0].Checksum {
		t.Errorf("Files[0].Checksum: got %q, want %q", got.Files[0].Checksum, m.Files[0].Checksum)
	}
	if got.Files[0].Status != m.Files[0].Status {
		t.Errorf("Files[0].Status: got %q, want %q", got.Files[0].Status, m.Files[0].Status)
	}
}

func TestManifest_Load_notExist(t *testing.T) {
	dirB := t.TempDir()
	got, err := Load(dirB)
	if err != nil {
		t.Fatalf("Load on missing manifest should return nil error, got: %v", err)
	}
	if got != nil {
		t.Errorf("Load on missing manifest should return nil, got: %+v", got)
	}
}

func TestManifest_Save_createsDir(t *testing.T) {
	// dirB itself exists but .pixe does not — Save must create it.
	dirB := t.TempDir()
	m := sampleManifest(dirB)
	if err := Save(m, dirB); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirB, ".pixe", "manifest.json")); err != nil {
		t.Errorf("manifest.json not found after Save: %v", err)
	}
}

func TestManifest_Save_atomic_noTmpLeftover(t *testing.T) {
	// After a successful Save, no .tmp file should remain.
	dirB := t.TempDir()
	m := sampleManifest(dirB)
	if err := Save(m, dirB); err != nil {
		t.Fatalf("Save: %v", err)
	}
	tmp := manifestPath(dirB) + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("temp file %q should not exist after successful Save", tmp)
	}
}

func TestManifest_Save_overwrite(t *testing.T) {
	// Saving twice should overwrite cleanly.
	dirB := t.TempDir()
	m := sampleManifest(dirB)
	if err := Save(m, dirB); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	m.Workers = 8
	if err := Save(m, dirB); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	got, err := Load(dirB)
	if err != nil {
		t.Fatalf("Load after overwrite: %v", err)
	}
	if got.Workers != 8 {
		t.Errorf("Workers after overwrite: got %d, want 8", got.Workers)
	}
}

// --- Ledger tests (v4 JSONL format) ---

func TestLedger_v4_roundtrip(t *testing.T) {
	dirA := t.TempDir()
	header, entry := writeSampleLedger(t, dirA)

	if _, err := os.Stat(ledgerPath(dirA)); err != nil {
		t.Fatalf("ledger file not created: %v", err)
	}

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if got == nil {
		t.Fatal("LoadLedger returned nil")
	}
	if got.Header.Version != header.Version {
		t.Errorf("Header.Version: got %d, want %d", got.Header.Version, header.Version)
	}
	if got.Header.PixeVersion != header.PixeVersion {
		t.Errorf("Header.PixeVersion: got %q, want %q", got.Header.PixeVersion, header.PixeVersion)
	}
	if got.Header.Algorithm != header.Algorithm {
		t.Errorf("Header.Algorithm: got %q, want %q", got.Header.Algorithm, header.Algorithm)
	}
	if len(got.Entries) != 1 {
		t.Fatalf("Entries len: got %d, want 1", len(got.Entries))
	}
	if got.Entries[0].Path != entry.Path {
		t.Errorf("Entries[0].Path: got %q, want %q", got.Entries[0].Path, entry.Path)
	}
}

func TestLedger_Load_notExist(t *testing.T) {
	dirA := t.TempDir()
	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger on missing file should return nil error, got: %v", err)
	}
	if got != nil {
		t.Errorf("LoadLedger on missing file should return nil, got: %+v", got)
	}
}

// --- Task 27: ledger v4 JSONL serialization tests ---

// writeLedgerV4Full writes a v4 JSONL ledger with all four entry types and
// returns the header and entries for assertion.
func writeLedgerV4Full(t *testing.T, dirA string) (domain.LedgerHeader, []domain.LedgerEntry) {
	t.Helper()
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(5 * time.Second)

	header := domain.LedgerHeader{
		Version:     4,
		RunID:       "run-uuid-abc123",
		PixeVersion: "1.2.3",
		PixeRun:     "2026-03-07T12:00:00Z",
		Algorithm:   "sha256",
		Destination: "/dst/archive",
		Recursive:   true,
	}
	entries := []domain.LedgerEntry{
		{
			Path:        "IMG_0001.jpg",
			Status:      domain.LedgerStatusCopy,
			Checksum:    "deadbeef01",
			Destination: "2026/03-Mar/20260307_120000_deadbeef01.jpg",
			VerifiedAt:  &verifiedAt,
		},
		{
			Path:   "notes.txt",
			Status: domain.LedgerStatusSkip,
			Reason: "unsupported format: .txt",
		},
		{
			Path:        "IMG_0002.jpg",
			Status:      domain.LedgerStatusDuplicate,
			Checksum:    "deadbeef01",
			Destination: "duplicates/20260307_120000_deadbeef01.jpg",
			Matches:     "2026/03-Mar/20260307_120000_deadbeef01.jpg",
		},
		{
			Path:   "corrupt.jpg",
			Status: domain.LedgerStatusError,
			Reason: "extract date: no EXIF data",
		},
	}

	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	for _, e := range entries {
		if err := lw.WriteEntry(e); err != nil {
			t.Fatalf("WriteEntry: %v", err)
		}
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return header, entries
}

// TestLedger_v4_fullRoundtrip verifies that a v4 ledger with all entry types
// round-trips through NewLedgerWriter/LoadLedger with all fields preserved.
func TestLedger_v4_fullRoundtrip(t *testing.T) {
	dirA := t.TempDir()
	header, entries := writeLedgerV4Full(t, dirA)

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if got == nil {
		t.Fatal("LoadLedger returned nil")
	}

	if got.Header.Version != 4 {
		t.Errorf("Header.Version = %d, want 4", got.Header.Version)
	}
	if got.Header.PixeVersion != header.PixeVersion {
		t.Errorf("Header.PixeVersion = %q, want %q", got.Header.PixeVersion, header.PixeVersion)
	}
	if got.Header.RunID != header.RunID {
		t.Errorf("Header.RunID = %q, want %q", got.Header.RunID, header.RunID)
	}
	if got.Header.PixeRun != header.PixeRun {
		t.Errorf("Header.PixeRun = %q, want %q", got.Header.PixeRun, header.PixeRun)
	}
	if got.Header.Algorithm != header.Algorithm {
		t.Errorf("Header.Algorithm = %q, want %q", got.Header.Algorithm, header.Algorithm)
	}
	if got.Header.Destination != header.Destination {
		t.Errorf("Header.Destination = %q, want %q", got.Header.Destination, header.Destination)
	}
	if !got.Header.Recursive {
		t.Error("Header.Recursive = false, want true")
	}
	if len(got.Entries) != len(entries) {
		t.Fatalf("Entries len = %d, want %d", len(got.Entries), len(entries))
	}
}

// TestLedger_v4_copyEntry verifies all fields of a "copy" ledger entry.
func TestLedger_v4_copyEntry(t *testing.T) {
	dirA := t.TempDir()
	_, entries := writeLedgerV4Full(t, dirA)

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	var copyEntry *domain.LedgerEntry
	for i := range got.Entries {
		if got.Entries[i].Status == domain.LedgerStatusCopy {
			copyEntry = &got.Entries[i]
			break
		}
	}
	if copyEntry == nil {
		t.Fatal("no copy entry found")
	}

	want := entries[0]
	if copyEntry.Path != want.Path {
		t.Errorf("Path = %q, want %q", copyEntry.Path, want.Path)
	}
	if copyEntry.Checksum != want.Checksum {
		t.Errorf("Checksum = %q, want %q", copyEntry.Checksum, want.Checksum)
	}
	if copyEntry.Destination != want.Destination {
		t.Errorf("Destination = %q, want %q", copyEntry.Destination, want.Destination)
	}
	if copyEntry.VerifiedAt == nil {
		t.Fatal("VerifiedAt is nil, want non-nil")
	}
	if !copyEntry.VerifiedAt.Equal(*want.VerifiedAt) {
		t.Errorf("VerifiedAt = %v, want %v", copyEntry.VerifiedAt, want.VerifiedAt)
	}
	if copyEntry.Reason != "" {
		t.Errorf("Reason = %q, want empty (omitempty)", copyEntry.Reason)
	}
	if copyEntry.Matches != "" {
		t.Errorf("Matches = %q, want empty (omitempty)", copyEntry.Matches)
	}
}

// TestLedger_v4_skipEntry verifies all fields of a "skip" ledger entry.
func TestLedger_v4_skipEntry(t *testing.T) {
	dirA := t.TempDir()
	_, entries := writeLedgerV4Full(t, dirA)

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	var skipEntry *domain.LedgerEntry
	for i := range got.Entries {
		if got.Entries[i].Status == domain.LedgerStatusSkip {
			skipEntry = &got.Entries[i]
			break
		}
	}
	if skipEntry == nil {
		t.Fatal("no skip entry found")
	}

	want := entries[1]
	if skipEntry.Path != want.Path {
		t.Errorf("Path = %q, want %q", skipEntry.Path, want.Path)
	}
	if skipEntry.Reason != want.Reason {
		t.Errorf("Reason = %q, want %q", skipEntry.Reason, want.Reason)
	}
	if skipEntry.Checksum != "" {
		t.Errorf("Checksum = %q, want empty (omitempty)", skipEntry.Checksum)
	}
	if skipEntry.VerifiedAt != nil {
		t.Errorf("VerifiedAt = %v, want nil (omitempty)", skipEntry.VerifiedAt)
	}
}

// TestLedger_v4_duplicateEntry verifies all fields of a "duplicate" ledger entry.
func TestLedger_v4_duplicateEntry(t *testing.T) {
	dirA := t.TempDir()
	_, entries := writeLedgerV4Full(t, dirA)

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	var dupEntry *domain.LedgerEntry
	for i := range got.Entries {
		if got.Entries[i].Status == domain.LedgerStatusDuplicate {
			dupEntry = &got.Entries[i]
			break
		}
	}
	if dupEntry == nil {
		t.Fatal("no duplicate entry found")
	}

	want := entries[2]
	if dupEntry.Path != want.Path {
		t.Errorf("Path = %q, want %q", dupEntry.Path, want.Path)
	}
	if dupEntry.Checksum != want.Checksum {
		t.Errorf("Checksum = %q, want %q", dupEntry.Checksum, want.Checksum)
	}
	if dupEntry.Destination != want.Destination {
		t.Errorf("Destination = %q, want %q", dupEntry.Destination, want.Destination)
	}
	if dupEntry.Matches != want.Matches {
		t.Errorf("Matches = %q, want %q", dupEntry.Matches, want.Matches)
	}
	if dupEntry.VerifiedAt != nil {
		t.Errorf("VerifiedAt = %v, want nil (omitempty for duplicate)", dupEntry.VerifiedAt)
	}
}

// TestLedger_v4_errorEntry verifies all fields of an "error" ledger entry.
func TestLedger_v4_errorEntry(t *testing.T) {
	dirA := t.TempDir()
	_, entries := writeLedgerV4Full(t, dirA)

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	var errEntry *domain.LedgerEntry
	for i := range got.Entries {
		if got.Entries[i].Status == domain.LedgerStatusError {
			errEntry = &got.Entries[i]
			break
		}
	}
	if errEntry == nil {
		t.Fatal("no error entry found")
	}

	want := entries[3]
	if errEntry.Path != want.Path {
		t.Errorf("Path = %q, want %q", errEntry.Path, want.Path)
	}
	if errEntry.Reason != want.Reason {
		t.Errorf("Reason = %q, want %q", errEntry.Reason, want.Reason)
	}
	if errEntry.Checksum != "" {
		t.Errorf("Checksum = %q, want empty (omitempty)", errEntry.Checksum)
	}
	if errEntry.VerifiedAt != nil {
		t.Errorf("VerifiedAt = %v, want nil (omitempty)", errEntry.VerifiedAt)
	}
}

// TestLedger_v4_recursive_false verifies that Recursive=false is preserved.
func TestLedger_v4_recursive_false(t *testing.T) {
	dirA := t.TempDir()
	header := domain.LedgerHeader{
		Version:     4,
		RunID:       "test-run",
		PixeVersion: "1.0.0",
		PixeRun:     "2026-03-07T12:00:00Z",
		Algorithm:   "sha1",
		Destination: "/dst",
		Recursive:   false,
	}
	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if got.Header.Recursive {
		t.Error("Header.Recursive = true, want false")
	}
}

// TestLedger_v4_omitempty_jsonl verifies that omitempty fields are absent from
// the JSONL output for a skip entry (which has no checksum, destination, etc.).
func TestLedger_v4_omitempty_jsonl(t *testing.T) {
	dirA := t.TempDir()
	header := domain.LedgerHeader{
		Version:     4,
		RunID:       "test-run",
		PixeVersion: "1.0.0",
		PixeRun:     "2026-03-07T12:00:00Z",
		Algorithm:   "sha1",
		Destination: "/dst",
	}
	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	_ = lw.WriteEntry(domain.LedgerEntry{
		Path:   "notes.txt",
		Status: domain.LedgerStatusSkip,
		Reason: "unsupported format: .txt",
		// Checksum, Destination, VerifiedAt, Matches intentionally zero.
	})
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(ledgerPath(dirA))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Split into lines; line 2 (index 1) is the entry.
	lines := splitLines(string(data))
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d; content:\n%s", len(lines), string(data))
	}
	entryLine := lines[1]

	// These keys should NOT appear in the entry line.
	absentInEntry := []string{`"checksum"`, `"verified_at"`, `"matches"`, `"destination"`}
	for _, key := range absentInEntry {
		if containsStr(entryLine, key) {
			t.Errorf("entry line contains %q but it should be omitted (omitempty); line:\n%s", key, entryLine)
		}
	}

	// These keys SHOULD appear in the entry line.
	presentKeys := []string{`"path"`, `"status"`, `"reason"`}
	for _, key := range presentKeys {
		if !containsStr(entryLine, key) {
			t.Errorf("entry line missing %q; line:\n%s", key, entryLine)
		}
	}
}

// splitLines splits s by newlines, discarding empty trailing lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		if tail := s[start:]; tail != "" {
			lines = append(lines, tail)
		}
	}
	return lines
}

// containsStr is a simple helper to avoid importing strings in the test file.
func containsStr(s, substr string) bool {
	return findStrIdx(s, substr) >= 0
}

// findStrIdx returns the index of the first occurrence of substr in s,
// or -1 if not present.
func findStrIdx(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// --- Task 26: LedgerWriter unit tests ---

// TestLedgerWriter_headerOnly opens a writer, writes only the header, closes it,
// and verifies the file has exactly 1 line that parses back as a valid LedgerHeader.
func TestLedgerWriter_headerOnly(t *testing.T) {
	dirA := t.TempDir()
	header := sampleLedgerHeader()

	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(ledgerPath(dirA))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := splitLines(string(data))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d; content:\n%s", len(lines), string(data))
	}

	var got domain.LedgerHeader
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if got.Version != header.Version {
		t.Errorf("Version = %d, want %d", got.Version, header.Version)
	}
	if got.RunID != header.RunID {
		t.Errorf("RunID = %q, want %q", got.RunID, header.RunID)
	}
	if got.PixeVersion != header.PixeVersion {
		t.Errorf("PixeVersion = %q, want %q", got.PixeVersion, header.PixeVersion)
	}
	if got.Algorithm != header.Algorithm {
		t.Errorf("Algorithm = %q, want %q", got.Algorithm, header.Algorithm)
	}
	if got.Destination != header.Destination {
		t.Errorf("Destination = %q, want %q", got.Destination, header.Destination)
	}
}

// TestLedgerWriter_headerAndEntries writes a header + 3 entries and verifies
// the file has 4 lines, each parseable, in the correct order.
func TestLedgerWriter_headerAndEntries(t *testing.T) {
	dirA := t.TempDir()
	header := sampleLedgerHeader()
	now := time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC)

	entries := []domain.LedgerEntry{
		{Path: "a.jpg", Status: domain.LedgerStatusCopy, Checksum: "aaa", Destination: "2026/a.jpg", VerifiedAt: &now},
		{Path: "b.txt", Status: domain.LedgerStatusSkip, Reason: "unsupported format: .txt"},
		{Path: "c.jpg", Status: domain.LedgerStatusError, Reason: "extract date: no EXIF"},
	}

	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	for _, e := range entries {
		if err := lw.WriteEntry(e); err != nil {
			t.Fatalf("WriteEntry: %v", err)
		}
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(ledgerPath(dirA))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := splitLines(string(data))
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (1 header + 3 entries), got %d; content:\n%s", len(lines), string(data))
	}

	// Verify each line is valid JSON.
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("line %d is not valid JSON: %s", i+1, line)
		}
	}

	// Verify entry order and status fields.
	wantStatuses := []string{domain.LedgerStatusCopy, domain.LedgerStatusSkip, domain.LedgerStatusError}
	for i, wantStatus := range wantStatuses {
		var e domain.LedgerEntry
		if err := json.Unmarshal([]byte(lines[i+1]), &e); err != nil {
			t.Fatalf("unmarshal entry %d: %v", i+1, err)
		}
		if e.Status != wantStatus {
			t.Errorf("entry %d Status = %q, want %q", i+1, e.Status, wantStatus)
		}
		if e.Path != entries[i].Path {
			t.Errorf("entry %d Path = %q, want %q", i+1, e.Path, entries[i].Path)
		}
	}
}

// TestLedgerWriter_omitempty verifies that zero-valued optional fields are absent
// from the raw JSON line for a skip entry.
func TestLedgerWriter_omitempty(t *testing.T) {
	dirA := t.TempDir()
	lw, err := NewLedgerWriter(dirA, sampleLedgerHeader())
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	_ = lw.WriteEntry(domain.LedgerEntry{
		Path:   "notes.txt",
		Status: domain.LedgerStatusSkip,
		Reason: "unsupported format: .txt",
		// Checksum, Destination, VerifiedAt, Matches intentionally zero.
	})
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(ledgerPath(dirA))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := splitLines(string(data))
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	entryLine := lines[1]

	absent := []string{`"checksum"`, `"destination"`, `"verified_at"`, `"matches"`}
	for _, key := range absent {
		if containsStr(entryLine, key) {
			t.Errorf("entry line contains %q but should be omitted (omitempty); line:\n%s", key, entryLine)
		}
	}
}

// TestLedgerWriter_compactJSON verifies that each line is compact JSON —
// no embedded newlines and no leading/trailing whitespace within the JSON value.
func TestLedgerWriter_compactJSON(t *testing.T) {
	dirA := t.TempDir()
	now := time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC)
	lw, err := NewLedgerWriter(dirA, sampleLedgerHeader())
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	_ = lw.WriteEntry(domain.LedgerEntry{
		Path: "img.jpg", Status: domain.LedgerStatusCopy,
		Checksum: "abc", Destination: "2026/img.jpg", VerifiedAt: &now,
	})
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(ledgerPath(dirA))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Each non-empty line must be valid JSON with no internal newlines.
	for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !json.Valid([]byte(line)) {
			t.Errorf("line %d is not valid JSON: %s", i+1, line)
		}
		// A compact JSON object has no embedded newlines.
		if strings.ContainsRune(line, '\n') {
			t.Errorf("line %d contains embedded newline (not compact): %q", i+1, line)
		}
	}
}

// TestLedgerWriter_nilSafe verifies that calling WriteEntry on a nil *LedgerWriter
// does not panic and returns nil.
func TestLedgerWriter_nilSafe(t *testing.T) {
	var lw *LedgerWriter
	err := lw.WriteEntry(domain.LedgerEntry{Path: "a.jpg", Status: domain.LedgerStatusCopy})
	if err != nil {
		t.Errorf("WriteEntry on nil LedgerWriter returned error: %v", err)
	}
}

// TestLedgerWriter_version4 verifies that the header line contains "version":4.
func TestLedgerWriter_version4(t *testing.T) {
	dirA := t.TempDir()
	header := sampleLedgerHeader() // Version: 4
	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatalf("NewLedgerWriter: %v", err)
	}
	if err := lw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(ledgerPath(dirA))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := splitLines(string(data))
	if len(lines) == 0 {
		t.Fatal("ledger file is empty")
	}
	if !containsStr(lines[0], `"version":4`) {
		t.Errorf("header line does not contain %q; line:\n%s", `"version":4`, lines[0])
	}
}

// --- Task 29: interrupted run produces partial valid JSONL ---

// TestLedgerWriter_partialWrite verifies that if Close is never called (simulating
// a crash), the file on disk contains valid JSONL up to the last complete entry.
func TestLedgerWriter_partialWrite(t *testing.T) {
	dirA := t.TempDir()
	now := time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC)

	header := domain.LedgerHeader{
		Version:     4,
		RunID:       "test-run",
		PixeVersion: "1.0.0",
		PixeRun:     "2026-03-06T10:30:00Z",
		Algorithm:   "sha1",
		Destination: "/dst",
	}
	lw, err := NewLedgerWriter(dirA, header)
	if err != nil {
		t.Fatal(err)
	}

	// Write 2 entries but do NOT call Close (simulating a crash).
	_ = lw.WriteEntry(domain.LedgerEntry{Path: "a.jpg", Status: domain.LedgerStatusCopy, Checksum: "aaa", Destination: "2026/a.jpg", VerifiedAt: &now})
	_ = lw.WriteEntry(domain.LedgerEntry{Path: "b.jpg", Status: domain.LedgerStatusSkip, Reason: "previously imported"})
	// Intentionally no lw.Close()

	// Read the file directly and verify it's valid JSONL.
	raw, err := os.ReadFile(filepath.Join(dirA, ".pixe_ledger.json"))
	if err != nil {
		t.Fatal(err)
	}

	lines := splitLines(string(raw))
	if len(lines) != 3 { // header + 2 entries
		t.Fatalf("expected 3 lines, got %d; content:\n%s", len(lines), string(raw))
	}

	// Each line must be valid JSON.
	for i, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("line %d is not valid JSON: %s", i+1, line)
		}
	}

	// Parse and verify the header.
	var h domain.LedgerHeader
	if err := json.Unmarshal([]byte(lines[0]), &h); err != nil {
		t.Fatalf("header parse: %v", err)
	}
	if h.Version != 4 {
		t.Errorf("header version: got %d, want 4", h.Version)
	}

	// Parse and verify the two entries.
	var e1, e2 domain.LedgerEntry
	if err := json.Unmarshal([]byte(lines[1]), &e1); err != nil {
		t.Fatalf("entry 1 parse: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[2]), &e2); err != nil {
		t.Fatalf("entry 2 parse: %v", err)
	}
	if e1.Path != "a.jpg" {
		t.Errorf("entry 1 path = %q, want %q", e1.Path, "a.jpg")
	}
	if e2.Path != "b.jpg" {
		t.Errorf("entry 2 path = %q, want %q", e2.Path, "b.jpg")
	}
}
