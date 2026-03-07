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
		PixeVersion: "0.9.0",
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
		Version:     1,
		PixeVersion: "0.9.0",
		PixeRun:     now,
		Algorithm:   "sha1",
		Destination: "/tmp/dest",
		Files: []domain.LedgerEntry{
			{
				Path:        "IMG_0001.jpg",
				Checksum:    "abc123",
				Destination: "2021/12/20211225_062223_abc123.jpg",
				VerifiedAt:  now,
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
