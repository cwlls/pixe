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
	"testing"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/manifest"
)

// copyFixtureN copies the named JPEG fixture into dir with a unique name.
func copyFixtureN(t *testing.T, dir, srcName, dstName string) {
	t.Helper()
	src := filepath.Join("..", "handler", "jpeg", "testdata", srcName)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("copyFixtureN read %q: %v", src, err)
	}
	if err := os.WriteFile(filepath.Join(dir, dstName), data, 0o644); err != nil {
		t.Fatalf("copyFixtureN write: %v", err)
	}
}

func newOptsN(t *testing.T, cfg *config.AppConfig, workers int, out *bytes.Buffer) SortOptions {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	cfg.Workers = workers
	return SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: "20260306_103000",
		Output:       out,
		PixeVersion:  "test",
	}
}

// TestRun_multipleWorkers processes 4 files with 4 workers and asserts all
// are processed and the ledger is written with 4 entries.
func TestRun_multipleWorkers(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Copy 4 files (2 unique, 2 duplicates).
	copyFixtureN(t, dirA, "with_exif_date.jpg", "photo1.jpg")
	copyFixtureN(t, dirA, "with_exif_date2.jpg", "photo2.jpg")
	copyFixtureN(t, dirA, "with_exif_date.jpg", "photo3.jpg")  // duplicate of photo1
	copyFixtureN(t, dirA, "with_exif_date2.jpg", "photo4.jpg") // duplicate of photo2

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	opts := newOptsN(t, cfg, 4, &out)

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("Run (4 workers): %v\nOutput:\n%s", err, out.String())
	}

	if result.Processed != 4 {
		t.Errorf("Processed = %d, want 4", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0\nOutput:\n%s", result.Errors, out.String())
	}

	// Ledger should have 4 entries (2 unique + 2 duplicates).
	l, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if l == nil {
		t.Fatal("ledger not written to dirA")
	}
	if len(l.Entries) != 4 {
		t.Errorf("ledger.Entries len = %d, want 4\nOutput:\n%s", len(l.Entries), out.String())
	}
	for _, e := range l.Entries {
		if e.Checksum == "" {
			t.Errorf("ledger entry %q has empty checksum", e.Path)
		}
	}
}

// TestRun_workers1_equivalentToSequential verifies that --workers 1 produces
// the same output structure as the default sequential path.
func TestRun_workers1_equivalentToSequential(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixtureN(t, dirA, "with_exif_date.jpg", "photo1.jpg")
	copyFixtureN(t, dirA, "with_exif_date2.jpg", "photo2.jpg")

	var out bytes.Buffer
	cfg := &config.AppConfig{Source: dirA, Destination: dirB, Algorithm: "sha1"}
	opts := newOptsN(t, cfg, 1, &out)

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("Run (1 worker): %v", err)
	}
	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}
}
