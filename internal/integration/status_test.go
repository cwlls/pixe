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

package integration

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/cwlls/pixe/internal/discovery"
	"github.com/cwlls/pixe/internal/domain"
	jpeghandler "github.com/cwlls/pixe/internal/handler/jpeg"
	"github.com/cwlls/pixe/internal/ignore"
	"github.com/cwlls/pixe/internal/manifest"
	"github.com/cwlls/pixe/internal/pipeline"
)

// classifySource runs the pixe status classification logic against dirA and
// returns the five categorised slices. It mirrors the logic in cmd/status.go
// without importing the cmd package (which would create a cycle).
func classifySource(t *testing.T, dirA string, recursive bool) (
	sorted, duplicates, errored, unsorted, unrecognized []string,
) {
	t.Helper()

	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())

	walkOpts := discovery.WalkOptions{
		Recursive: recursive,
		Ignore:    ignore.New(nil),
	}
	discovered, skipped, err := discovery.Walk(dirA, reg, walkOpts)
	if err != nil {
		t.Fatalf("classifySource: walk: %v", err)
	}

	lc, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("classifySource: load ledger: %v", err)
	}

	ledgerMap := make(map[string]domain.LedgerEntry)
	if lc != nil {
		for _, e := range lc.Entries {
			ledgerMap[e.Path] = e
		}
	}

	for _, df := range discovered {
		entry, found := ledgerMap[df.RelPath]
		if !found {
			unsorted = append(unsorted, df.RelPath)
			continue
		}
		switch entry.Status {
		case domain.LedgerStatusCopy:
			sorted = append(sorted, df.RelPath)
		case domain.LedgerStatusDuplicate:
			duplicates = append(duplicates, df.RelPath)
		case domain.LedgerStatusError:
			errored = append(errored, df.RelPath)
		default:
			unsorted = append(unsorted, df.RelPath)
		}
	}
	for _, sf := range skipped {
		unrecognized = append(unrecognized, sf.Path)
	}

	// Stable sort for deterministic assertions.
	sort.Strings(sorted)
	sort.Strings(duplicates)
	sort.Strings(errored)
	sort.Strings(unsorted)
	sort.Strings(unrecognized)

	return sorted, duplicates, errored, unsorted, unrecognized
}

// TestStatus_SortThenStatus is the primary end-to-end test:
//  1. Sort one JPEG file from dirA → dirB (writes the ledger).
//  2. Add a second JPEG to dirA (not yet sorted).
//  3. Run status classification and verify:
//     - The sorted file appears as SORTED.
//     - The new file appears as UNSORTED.
//     - No duplicates, errors, or unrecognized files.
//
// Note: all three test fixtures share the same pixel payload, so sorting more
// than one would produce duplicates. We sort exactly one file.
func TestStatus_SortThenStatus(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Seed one file and sort it.
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")

	result, err := pipeline.Run(buildOpts(t, dirA, dirB, false))
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}
	if result.Processed != 1 {
		t.Fatalf("expected 1 processed, got %d", result.Processed)
	}

	// Verify the ledger was written.
	lc, err := manifest.LoadLedger(dirA)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if lc == nil {
		t.Fatal("expected ledger to exist after sort run")
	}
	if len(lc.Entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(lc.Entries))
	}

	// Add a second file that has NOT been sorted yet.
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	// Run status classification.
	sorted, duplicates, errored, unsorted, unrecognized := classifySource(t, dirA, false)

	if len(sorted) != 1 {
		t.Errorf("sorted = %d, want 1; got %v", len(sorted), sorted)
	}
	if len(unsorted) != 1 {
		t.Errorf("unsorted = %d, want 1; got %v", len(unsorted), unsorted)
	}
	if len(unsorted) == 1 && unsorted[0] != "IMG_0002.jpg" {
		t.Errorf("unsorted[0] = %q, want IMG_0002.jpg", unsorted[0])
	}
	if len(duplicates) != 0 {
		t.Errorf("duplicates = %d, want 0", len(duplicates))
	}
	if len(errored) != 0 {
		t.Errorf("errored = %d, want 0", len(errored))
	}
	if len(unrecognized) != 0 {
		t.Errorf("unrecognized = %d, want 0", len(unrecognized))
	}
}

// TestStatus_DuplicateAppearsAsDuplicate verifies that a file whose content
// was already archived appears as DUPLICATE in the status output.
func TestStatus_DuplicateAppearsAsDuplicate(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Sort the original and a content-identical copy.
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif1, "IMG_0001_copy.jpg") // same content → duplicate

	if _, err := pipeline.Run(buildOpts(t, dirA, dirB, false)); err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}

	sorted, duplicates, errored, unsorted, unrecognized := classifySource(t, dirA, false)

	if len(sorted) != 1 {
		t.Errorf("sorted = %d, want 1; got %v", len(sorted), sorted)
	}
	if len(duplicates) != 1 {
		t.Errorf("duplicates = %d, want 1; got %v", len(duplicates), duplicates)
	}
	if len(errored)+len(unsorted)+len(unrecognized) != 0 {
		t.Errorf("expected no errored/unsorted/unrecognized, got errored=%v unsorted=%v unrecognized=%v",
			errored, unsorted, unrecognized)
	}
}

// TestStatus_AllUnsortedWhenNoLedger verifies that when no ledger exists,
// all recognized files are reported as unsorted.
func TestStatus_AllUnsortedWhenNoLedger(t *testing.T) {
	dirA := t.TempDir()

	// Place files without running sort (no ledger).
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	sorted, duplicates, errored, unsorted, unrecognized := classifySource(t, dirA, false)

	if len(unsorted) != 2 {
		t.Errorf("unsorted = %d, want 2; got %v", len(unsorted), unsorted)
	}
	if len(sorted)+len(duplicates)+len(errored)+len(unrecognized) != 0 {
		t.Errorf("expected only unsorted files, got sorted=%v duplicates=%v errored=%v unrecognized=%v",
			sorted, duplicates, errored, unrecognized)
	}
}

// TestStatus_RecursiveSort verifies that files sorted with --recursive are
// correctly reflected in a recursive status check.
//
// Note: all test fixtures share the same pixel payload, so sorting two files
// produces one sorted and one duplicate. The test verifies that both files are
// accounted for (sorted + duplicates == 2) and that the subdirectory file
// appears in one of those categories.
func TestStatus_RecursiveSort(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dirA, "vacation"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixture(t, dirA, fixtureExif1, "top.jpg")
	copyFixture(t, filepath.Join(dirA, "vacation"), fixtureExif2, "sub.jpg")

	// Sort recursively.
	opts := buildOpts(t, dirA, dirB, false)
	opts.Config.Recursive = true
	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}

	// Status check — also recursive.
	sorted, duplicates, _, unsorted, _ := classifySource(t, dirA, true)

	// Both files should be accounted for (one sorted, one duplicate — same payload).
	if len(sorted)+len(duplicates) != 2 {
		t.Errorf("sorted+duplicates = %d, want 2 (recursive); sorted=%v duplicates=%v",
			len(sorted)+len(duplicates), sorted, duplicates)
	}
	if len(unsorted) != 0 {
		t.Errorf("unsorted = %d, want 0; got %v", len(unsorted), unsorted)
	}

	// Verify the subdirectory file has the correct relative path in either bucket.
	allAccounted := append(sorted, duplicates...)
	found := false
	for _, p := range allAccounted {
		if p == "vacation/sub.jpg" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected vacation/sub.jpg in sorted or duplicates, got sorted=%v duplicates=%v",
			sorted, duplicates)
	}
}
