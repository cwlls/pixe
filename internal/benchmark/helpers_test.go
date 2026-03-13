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

// Package benchmark provides centralized performance benchmarks for the Pixe
// sort pipeline. All benchmarks live here so they can be run with a single
// invocation: go test -bench . ./internal/benchmark/
//
// Benchmarks are excluded from make test and make test-all. Run them with:
//
//	make bench
package benchmark

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/archivedb"
)

// generateFixture creates a deterministic byte slice of the given size.
// Uses a fixed seed for reproducibility across benchmark runs.
func generateFixture(b *testing.B, size int) []byte {
	b.Helper()
	//nolint:gosec // fixed seed is intentional for reproducibility
	r := rand.New(rand.NewSource(42))
	data := make([]byte, size)
	_, err := r.Read(data)
	if err != nil {
		b.Fatalf("generateFixture: %v", err)
	}
	return data
}

// openBenchDB opens a fresh SQLite database in b.TempDir() for benchmarking.
func openBenchDB(b *testing.B) *archivedb.DB {
	b.Helper()
	path := filepath.Join(b.TempDir(), "bench.db")
	db, err := archivedb.Open(path)
	if err != nil {
		b.Fatalf("openBenchDB: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })
	return db
}

// prepopulateDB creates a database with n file records in status "complete",
// each with a unique checksum and source path. Returns the populated DB.
func prepopulateDB(b *testing.B, n int) *archivedb.DB {
	b.Helper()
	db := openBenchDB(b)

	// Insert a run to satisfy the foreign key constraint.
	run := &archivedb.Run{
		ID:          "bench-run-001",
		PixeVersion: "0.0.0-bench",
		Source:      "/bench/source",
		Destination: "/bench/dest",
		Algorithm:   "sha1",
		Workers:     1,
		Recursive:   false,
		StartedAt:   time.Now(),
		Status:      "running",
	}
	if err := db.InsertRun(run); err != nil {
		b.Fatalf("prepopulateDB InsertRun: %v", err)
	}

	// Batch-insert n file records.
	files := make([]*archivedb.FileRecord, n)
	for i := range files {
		files[i] = &archivedb.FileRecord{
			RunID:      "bench-run-001",
			SourcePath: fmt.Sprintf("/bench/source/file_%06d.jpg", i),
		}
	}
	ids, err := db.InsertFiles(files)
	if err != nil {
		b.Fatalf("prepopulateDB InsertFiles: %v", err)
	}

	// Update each file to "complete" with a unique checksum.
	for i, id := range ids {
		checksum := fmt.Sprintf("%040x", i) // 40-char hex string (SHA-1 length)
		destRel := fmt.Sprintf("2024/01-Jan/20240101_120000-1-%s.jpg", checksum)
		if err := db.UpdateFileStatus(id, "complete",
			archivedb.WithChecksum(checksum),
			archivedb.WithAlgorithm("sha1"),
			archivedb.WithDestination("/bench/dest/"+destRel, destRel),
		); err != nil {
			b.Fatalf("prepopulateDB UpdateFileStatus %d: %v", i, err)
		}
	}

	return db
}

// getChecksum returns the checksum of the file at index i in a prepopulated DB.
// The checksum format matches what prepopulateDB inserts.
func getChecksum(i int) string {
	return fmt.Sprintf("%040x", i)
}

// getSourcePath returns the source path of the file at index i.
func getSourcePath(i int) string {
	return fmt.Sprintf("/bench/source/file_%06d.jpg", i)
}

// createFileTree creates a temporary directory with n synthetic JPEG files
// distributed across a year/month directory structure. Returns the root path.
// Files are zero-byte placeholders — suitable for walk-speed benchmarks.
func createFileTree(b *testing.B, n int, nested bool) string {
	b.Helper()
	root := b.TempDir()

	jpegMagic := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}

	months := []string{
		"01-Jan", "02-Feb", "03-Mar", "04-Apr",
		"05-May", "06-Jun", "07-Jul", "08-Aug",
		"09-Sep", "10-Oct", "11-Nov", "12-Dec",
	}

	for i := 0; i < n; i++ {
		var dir string
		if nested {
			year := 2020 + (i / (n/4 + 1))
			month := months[i%12]
			dir = filepath.Join(root, fmt.Sprintf("%d", year), month)
		} else {
			dir = root
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatalf("createFileTree MkdirAll: %v", err)
		}
		path := filepath.Join(dir, fmt.Sprintf("IMG_%06d.jpg", i))
		if err := os.WriteFile(path, jpegMagic, 0o644); err != nil {
			b.Fatalf("createFileTree WriteFile: %v", err)
		}
	}

	return root
}
