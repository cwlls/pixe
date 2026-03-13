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
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/cwlls/pixe-go/internal/pathbuilder"
)

const benchChecksum = "7d97e98f8af710c7e7fe703abc8f639e0ee507c4"

// BenchmarkPathBuilder measures the time per pathbuilder.Build() call for
// various date inputs. Locale is pinned to English for reproducibility.
// Run with:
//
//	go test -bench BenchmarkPathBuilder -benchmem ./internal/benchmark/
func BenchmarkPathBuilder(b *testing.B) {
	pathbuilder.SetLocaleForTesting(language.English)

	cases := []struct {
		name string
		date time.Time
	}{
		{
			name: "recent",
			date: time.Date(2024, 7, 15, 14, 30, 22, 0, time.UTC),
		},
		{
			name: "old",
			date: time.Date(1985, 3, 1, 8, 0, 0, 0, time.UTC),
		},
		{
			name: "ansel_adams_fallback",
			date: time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "year_boundary",
			date: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = pathbuilder.Build(nil, tc.date, 1, benchChecksum, ".jpg", false, "")
			}
		})
	}
}

// BenchmarkPathBuilderAllAlgorithms measures Build() throughput across all
// supported algorithm IDs (0–4).
func BenchmarkPathBuilderAllAlgorithms(b *testing.B) {
	pathbuilder.SetLocaleForTesting(language.English)

	date := time.Date(2024, 7, 15, 14, 30, 22, 0, time.UTC)
	algoNames := []string{"md5", "sha1", "sha256", "blake3", "xxhash"}

	for id, name := range algoNames {
		id, name := id, name
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = pathbuilder.Build(nil, date, id, benchChecksum, ".jpg", false, "")
			}
		})
	}
}
