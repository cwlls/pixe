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

package benchmark

import (
	"fmt"
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/archivedb"
)

// BenchmarkDBInsert measures the latency of inserting a single file record
// into databases of varying pre-populated sizes. Run with:
//
//	go test -bench BenchmarkDBInsert -benchmem ./internal/benchmark/
func BenchmarkDBInsert(b *testing.B) {
	sizes := []int{0, 1000, 10000, 100000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			db := prepopulateDB(b, n)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				f := &archivedb.FileRecord{
					RunID:      "bench-run-001",
					SourcePath: fmt.Sprintf("/bench/source/insert_%d_%d.jpg", n, i),
				}
				if _, err := db.InsertFile(f); err != nil {
					b.Fatalf("InsertFile: %v", err)
				}
			}
		})
	}
}

// BenchmarkDBDedupCheck measures the latency of CheckDuplicate queries
// (indexed SELECT on checksum) for hit and miss cases across varying DB sizes.
func BenchmarkDBDedupCheck(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("rows=%d/hit", n), func(b *testing.B) {
			db := prepopulateDB(b, n)
			knownChecksum := getChecksum(n / 2) // middle of the table
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if _, err := db.CheckDuplicate(knownChecksum); err != nil {
					b.Fatalf("CheckDuplicate: %v", err)
				}
			}
		})

		b.Run(fmt.Sprintf("rows=%d/miss", n), func(b *testing.B) {
			db := prepopulateDB(b, n)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if _, err := db.CheckDuplicate("nonexistent_checksum_value_that_does_not_exist"); err != nil {
					b.Fatalf("CheckDuplicate miss: %v", err)
				}
			}
		})
	}
}

// BenchmarkDBSkipCheck measures the latency of CheckSourceProcessed queries
// (indexed SELECT on source_path) for hit and miss cases across varying DB sizes.
func BenchmarkDBSkipCheck(b *testing.B) {
	sizes := []int{1000, 10000, 100000}

	for _, n := range sizes {
		b.Run(fmt.Sprintf("rows=%d/hit", n), func(b *testing.B) {
			db := prepopulateDB(b, n)
			knownPath := getSourcePath(n / 2)
			knownChecksum := getChecksum(n / 2)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if _, err := db.CheckSourceProcessed(knownPath, knownChecksum); err != nil {
					b.Fatalf("CheckSourceProcessed: %v", err)
				}
			}
		})

		b.Run(fmt.Sprintf("rows=%d/miss", n), func(b *testing.B) {
			db := prepopulateDB(b, n)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				if _, err := db.CheckSourceProcessed("/bench/source/nonexistent_file.jpg", "nonexistent_checksum"); err != nil {
					b.Fatalf("CheckSourceProcessed miss: %v", err)
				}
			}
		})
	}
}

// BenchmarkDBInsertBatch measures the throughput of InsertFiles (batch insert)
// for varying batch sizes.
func BenchmarkDBInsertBatch(b *testing.B) {
	batchSizes := []int{10, 100, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch=%d", batchSize), func(b *testing.B) {
			db := openBenchDB(b)

			// Insert a run for the foreign key.
			run := &archivedb.Run{
				ID:          "bench-batch-run",
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
				b.Fatalf("InsertRun: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				files := make([]*archivedb.FileRecord, batchSize)
				for j := range files {
					files[j] = &archivedb.FileRecord{
						RunID:      "bench-batch-run",
						SourcePath: fmt.Sprintf("/bench/source/batch_%d_%d.jpg", i, j),
					}
				}
				if _, err := db.InsertFiles(files); err != nil {
					b.Fatalf("InsertFiles: %v", err)
				}
			}
		})
	}
}
