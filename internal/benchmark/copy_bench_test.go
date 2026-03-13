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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	copypkg "github.com/cwlls/pixe-go/internal/copy"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/hash"
)

// fullFileHandler is a minimal FileTypeHandler that returns the full file
// content from HashableReader. Used for copy+verify benchmarks where we
// want to measure raw I/O throughput without format-specific parsing.
type fullFileHandler struct{}

func (h *fullFileHandler) Extensions() []string                    { return []string{".bin"} }
func (h *fullFileHandler) MagicBytes() []domain.MagicSignature     { return nil }
func (h *fullFileHandler) Detect(_ string) (bool, error)           { return true, nil }
func (h *fullFileHandler) ExtractDate(_ string) (time.Time, error) { return time.Time{}, nil }
func (h *fullFileHandler) HashableReader(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
func (h *fullFileHandler) MetadataSupport() domain.MetadataCapability { return domain.MetadataNone }
func (h *fullFileHandler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// BenchmarkCopyVerify measures end-to-end copy+verify throughput for the
// atomic copy-then-verify flow. Run with:
//
//	go test -bench BenchmarkCopyVerify -benchmem ./internal/benchmark/
func BenchmarkCopyVerify(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1MB", 1 << 20},
		{"10MB", 10 << 20},
		{"100MB", 100 << 20},
	}

	handler := &fullFileHandler{}
	hasher, err := hash.NewHasher("sha1")
	if err != nil {
		b.Fatalf("NewHasher: %v", err)
	}

	for _, sz := range sizes {
		b.Run(sz.name, func(b *testing.B) {
			data := generateFixture(b, sz.size)

			// Pre-compute the expected checksum once.
			expectedChecksum, err := hasher.Sum(bytes.NewReader(data))
			if err != nil {
				b.Fatalf("pre-compute checksum: %v", err)
			}

			b.SetBytes(int64(sz.size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				b.StopTimer()

				// Write source file to a fresh temp dir each iteration.
				srcDir := b.TempDir()
				srcPath := filepath.Join(srcDir, fmt.Sprintf("src_%d.bin", i))
				if err := os.WriteFile(srcPath, data, 0o644); err != nil {
					b.Fatalf("WriteFile: %v", err)
				}
				destDir := b.TempDir()
				destPath := filepath.Join(destDir, fmt.Sprintf("dst_%d.bin", i))

				b.StartTimer()

				// Execute: stream src → temp file.
				tmpPath, err := copypkg.Execute(srcPath, destPath)
				if err != nil {
					b.Fatalf("Execute: %v", err)
				}

				// Verify: re-hash the temp file.
				result := copypkg.Verify(tmpPath, expectedChecksum, handler, hasher)
				if !result.Success {
					b.Fatalf("Verify failed: %v", result.Error)
				}

				// Promote: atomic rename to final destination.
				if err := copypkg.Promote(tmpPath, destPath); err != nil {
					b.Fatalf("Promote: %v", err)
				}
			}
		})
	}
}
