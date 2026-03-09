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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
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

// sampleLedger returns a populated Ledger for use in tests.
func sampleLedger() *domain.Ledger {
	now := time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC)
	return &domain.Ledger{
		Version:     3,
		PixeVersion: "test",
		PixeRun:     now,
		Algorithm:   "sha1",
		Destination: "/tmp/dest",
		Files: []domain.LedgerEntry{
			{
				Path:        "IMG_0001.jpg",
				Status:      domain.LedgerStatusCopy,
				Checksum:    "abc123",
				Destination: "2021/12/20211225_062223_abc123.jpg",
				VerifiedAt:  &now,
			},
		},
	}
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

// --- Ledger tests ---

func TestLedger_SaveLoad_roundtrip(t *testing.T) {
	dirA := t.TempDir()
	l := sampleLedger()

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}

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
	if got.Version != l.Version {
		t.Errorf("Version: got %d, want %d", got.Version, l.Version)
	}
	if got.PixeVersion != l.PixeVersion {
		t.Errorf("PixeVersion: got %q, want %q", got.PixeVersion, l.PixeVersion)
	}
	if got.Algorithm != l.Algorithm {
		t.Errorf("Algorithm: got %q, want %q", got.Algorithm, l.Algorithm)
	}
	if len(got.Files) != 1 {
		t.Fatalf("Files len: got %d, want 1", len(got.Files))
	}
	if got.Files[0].Path != "IMG_0001.jpg" {
		t.Errorf("Files[0].Path: got %q, want %q", got.Files[0].Path, "IMG_0001.jpg")
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

func TestLedger_Save_atomic_noTmpLeftover(t *testing.T) {
	dirA := t.TempDir()
	l := sampleLedger()
	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}
	tmp := ledgerPath(dirA) + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("temp file %q should not exist after successful SaveLedger", tmp)
	}
}

// --- Task 16: ledger v3 serialization tests ---

// sampleLedgerV3Full returns a v3 Ledger with all four entry types:
// copy, skip, duplicate, and error.
func sampleLedgerV3Full() *domain.Ledger {
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(5 * time.Second)
	return &domain.Ledger{
		Version:     3,
		PixeVersion: "1.2.3",
		RunID:       "run-uuid-abc123",
		PixeRun:     now,
		Algorithm:   "sha256",
		Destination: "/dst/archive",
		Recursive:   true,
		Files: []domain.LedgerEntry{
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
		},
	}
}

// TestLedger_v3_roundtrip verifies that a v3 ledger with all entry types
// round-trips through SaveLedger/LoadLedger with all fields preserved.
func TestLedger_v3_roundtrip(t *testing.T) {
	dirA := t.TempDir()
	l := sampleLedgerV3Full()

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}

	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if got == nil {
		t.Fatal("LoadLedger returned nil")
	}

	// Top-level fields.
	if got.Version != 3 {
		t.Errorf("Version = %d, want 3", got.Version)
	}
	if got.PixeVersion != l.PixeVersion {
		t.Errorf("PixeVersion = %q, want %q", got.PixeVersion, l.PixeVersion)
	}
	if got.RunID != l.RunID {
		t.Errorf("RunID = %q, want %q", got.RunID, l.RunID)
	}
	if !got.PixeRun.Equal(l.PixeRun) {
		t.Errorf("PixeRun = %v, want %v", got.PixeRun, l.PixeRun)
	}
	if got.Algorithm != l.Algorithm {
		t.Errorf("Algorithm = %q, want %q", got.Algorithm, l.Algorithm)
	}
	if got.Destination != l.Destination {
		t.Errorf("Destination = %q, want %q", got.Destination, l.Destination)
	}
	if !got.Recursive {
		t.Error("Recursive = false, want true")
	}
	if len(got.Files) != 4 {
		t.Fatalf("Files len = %d, want 4", len(got.Files))
	}
}

// TestLedger_v3_copyEntry verifies all fields of a "copy" ledger entry.
func TestLedger_v3_copyEntry(t *testing.T) {
	dirA := t.TempDir()
	l := sampleLedgerV3Full()

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}
	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	// Find the copy entry.
	var copyEntry *domain.LedgerEntry
	for i := range got.Files {
		if got.Files[i].Status == domain.LedgerStatusCopy {
			copyEntry = &got.Files[i]
			break
		}
	}
	if copyEntry == nil {
		t.Fatal("no copy entry found")
	}

	want := l.Files[0]
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
	// omitempty fields that should be absent.
	if copyEntry.Reason != "" {
		t.Errorf("Reason = %q, want empty (omitempty)", copyEntry.Reason)
	}
	if copyEntry.Matches != "" {
		t.Errorf("Matches = %q, want empty (omitempty)", copyEntry.Matches)
	}
}

// TestLedger_v3_skipEntry verifies all fields of a "skip" ledger entry.
func TestLedger_v3_skipEntry(t *testing.T) {
	dirA := t.TempDir()
	l := sampleLedgerV3Full()

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}
	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	var skipEntry *domain.LedgerEntry
	for i := range got.Files {
		if got.Files[i].Status == domain.LedgerStatusSkip {
			skipEntry = &got.Files[i]
			break
		}
	}
	if skipEntry == nil {
		t.Fatal("no skip entry found")
	}

	want := l.Files[1]
	if skipEntry.Path != want.Path {
		t.Errorf("Path = %q, want %q", skipEntry.Path, want.Path)
	}
	if skipEntry.Reason != want.Reason {
		t.Errorf("Reason = %q, want %q", skipEntry.Reason, want.Reason)
	}
	// omitempty fields that should be absent.
	if skipEntry.Checksum != "" {
		t.Errorf("Checksum = %q, want empty (omitempty)", skipEntry.Checksum)
	}
	if skipEntry.VerifiedAt != nil {
		t.Errorf("VerifiedAt = %v, want nil (omitempty)", skipEntry.VerifiedAt)
	}
}

// TestLedger_v3_duplicateEntry verifies all fields of a "duplicate" ledger entry.
func TestLedger_v3_duplicateEntry(t *testing.T) {
	dirA := t.TempDir()
	l := sampleLedgerV3Full()

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}
	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	var dupEntry *domain.LedgerEntry
	for i := range got.Files {
		if got.Files[i].Status == domain.LedgerStatusDuplicate {
			dupEntry = &got.Files[i]
			break
		}
	}
	if dupEntry == nil {
		t.Fatal("no duplicate entry found")
	}

	want := l.Files[2]
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
	// omitempty fields that should be absent.
	if dupEntry.VerifiedAt != nil {
		t.Errorf("VerifiedAt = %v, want nil (omitempty for duplicate)", dupEntry.VerifiedAt)
	}
}

// TestLedger_v3_errorEntry verifies all fields of an "error" ledger entry.
func TestLedger_v3_errorEntry(t *testing.T) {
	dirA := t.TempDir()
	l := sampleLedgerV3Full()

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}
	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}

	var errEntry *domain.LedgerEntry
	for i := range got.Files {
		if got.Files[i].Status == domain.LedgerStatusError {
			errEntry = &got.Files[i]
			break
		}
	}
	if errEntry == nil {
		t.Fatal("no error entry found")
	}

	want := l.Files[3]
	if errEntry.Path != want.Path {
		t.Errorf("Path = %q, want %q", errEntry.Path, want.Path)
	}
	if errEntry.Reason != want.Reason {
		t.Errorf("Reason = %q, want %q", errEntry.Reason, want.Reason)
	}
	// omitempty fields that should be absent.
	if errEntry.Checksum != "" {
		t.Errorf("Checksum = %q, want empty (omitempty)", errEntry.Checksum)
	}
	if errEntry.VerifiedAt != nil {
		t.Errorf("VerifiedAt = %v, want nil (omitempty)", errEntry.VerifiedAt)
	}
}

// TestLedger_v3_recursive_false verifies that Recursive=false is preserved.
func TestLedger_v3_recursive_false(t *testing.T) {
	dirA := t.TempDir()
	l := &domain.Ledger{
		Version:     3,
		PixeVersion: "1.0.0",
		PixeRun:     time.Now().UTC(),
		Algorithm:   "sha1",
		Destination: "/dst",
		Recursive:   false,
		Files:       []domain.LedgerEntry{},
	}

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}
	got, err := LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if got.Recursive {
		t.Error("Recursive = true, want false")
	}
}

// TestLedger_v3_omitempty_json verifies that omitempty fields are absent from
// the JSON output for a skip entry (which has no checksum, destination, etc.).
func TestLedger_v3_omitempty_json(t *testing.T) {
	dirA := t.TempDir()
	l := &domain.Ledger{
		Version:     3,
		PixeVersion: "1.0.0",
		PixeRun:     time.Now().UTC(),
		Algorithm:   "sha1",
		Destination: "/dst",
		Files: []domain.LedgerEntry{
			{
				Path:   "notes.txt",
				Status: domain.LedgerStatusSkip,
				Reason: "unsupported format: .txt",
				// Checksum, Destination, VerifiedAt, Matches intentionally zero.
			},
		},
	}

	if err := SaveLedger(l, dirA); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}

	// Read raw JSON and extract just the files array section to check entry-level omitempty.
	// The top-level Ledger has a "destination" field which is expected; we only
	// want to verify that the LedgerEntry omits its optional fields.
	data, err := os.ReadFile(ledgerPath(dirA))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	jsonStr := string(data)

	// Find the "files" array section — everything after `"files": [`.
	filesMarker := `"files": [`
	filesIdx := findStrIdx(jsonStr, filesMarker)
	if filesIdx < 0 {
		t.Fatalf("JSON missing 'files' array; JSON:\n%s", jsonStr)
	}
	filesSection := jsonStr[filesIdx+len(filesMarker):]

	// These keys should NOT appear inside the file entry.
	absentInEntry := []string{`"checksum"`, `"verified_at"`, `"matches"`}
	for _, key := range absentInEntry {
		if containsStr(filesSection, key) {
			t.Errorf("file entry JSON contains %q but it should be omitted (omitempty); files section:\n%s", key, filesSection)
		}
	}
	// "destination" should NOT appear inside the entry (it's only at the top level).
	// We check the files section specifically.
	if containsStr(filesSection, `"destination"`) {
		t.Errorf("file entry JSON contains \"destination\" but it should be omitted (omitempty); files section:\n%s", filesSection)
	}

	// These keys SHOULD appear in the files section.
	presentKeys := []string{`"path"`, `"status"`, `"reason"`}
	for _, key := range presentKeys {
		if !containsStr(filesSection, key) {
			t.Errorf("file entry JSON missing %q; files section:\n%s", key, filesSection)
		}
	}
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
