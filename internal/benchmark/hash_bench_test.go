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
	"testing"

	"github.com/cwlls/pixe-go/internal/hash"
)

// BenchmarkHash measures hashing throughput for all supported algorithms
// across a range of file sizes. Run with:
//
//	go test -bench BenchmarkHash -benchmem ./internal/benchmark/
func BenchmarkHash(b *testing.B) {
	algorithms := []string{"md5", "sha1", "sha256", "blake3", "xxhash"}
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"1MB", 1 << 20},
		{"10MB", 10 << 20},
		{"100MB", 100 << 20},
	}

	for _, alg := range algorithms {
		for _, sz := range sizes {
			b.Run(fmt.Sprintf("%s/%s", alg, sz.name), func(b *testing.B) {
				data := generateFixture(b, sz.size)
				b.SetBytes(int64(sz.size))
				b.ResetTimer()

				h, err := hash.NewHasher(alg)
				if err != nil {
					b.Fatalf("NewHasher(%q): %v", alg, err)
				}

				for i := 0; i < b.N; i++ {
					if _, err := h.Sum(bytes.NewReader(data)); err != nil {
						b.Fatalf("Sum: %v", err)
					}
				}
			})
		}
	}
}
