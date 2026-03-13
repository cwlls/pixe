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
	"io"
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/discovery"
	"github.com/cwlls/pixe/internal/domain"
)

// benchmarkHandler is a minimal FileTypeHandler that matches .jpg files
// by magic bytes. Used for discovery walk benchmarks.
type benchmarkHandler struct{}

func (h *benchmarkHandler) Extensions() []string { return []string{".jpg"} }
func (h *benchmarkHandler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 0, Bytes: []byte{0xFF, 0xD8, 0xFF}},
	}
}
func (h *benchmarkHandler) Detect(_ string) (bool, error)           { return true, nil }
func (h *benchmarkHandler) ExtractDate(_ string) (time.Time, error) { return time.Time{}, nil }
func (h *benchmarkHandler) HashableReader(_ string) (io.ReadCloser, error) {
	return nil, nil
}
func (h *benchmarkHandler) MetadataSupport() domain.MetadataCapability { return domain.MetadataNone }
func (h *benchmarkHandler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// BenchmarkDiscoveryWalk measures the speed of discovery.Walk() over synthetic
// directory trees of varying sizes and structures. Run with:
//
//	go test -bench BenchmarkDiscoveryWalk -benchmem ./internal/benchmark/
func BenchmarkDiscoveryWalk(b *testing.B) {
	treeSizes := []struct {
		name  string
		count int
	}{
		{"100files", 100},
		{"1Kfiles", 1000},
		{"10Kfiles", 10000},
	}

	structures := []struct {
		name   string
		nested bool
	}{
		{"flat", false},
		{"nested", true},
	}

	reg := discovery.NewRegistry()
	reg.Register(&benchmarkHandler{})

	for _, sz := range treeSizes {
		for _, st := range structures {
			b.Run(fmt.Sprintf("%s/%s", sz.name, st.name), func(b *testing.B) {
				root := createFileTree(b, sz.count, st.nested)
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					discovered, _, err := discovery.Walk(root, reg, discovery.WalkOptions{
						Recursive: st.nested,
					})
					if err != nil {
						b.Fatalf("Walk: %v", err)
					}
					// Prevent the compiler from optimizing away the result.
					if len(discovered) == 0 && sz.count > 0 {
						b.Fatal("Walk returned no files")
					}
				}
			})
		}
	}
}
