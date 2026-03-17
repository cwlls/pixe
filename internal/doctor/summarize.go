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

package doctor

// Entry is the minimal input the diagnosis engine needs. It is populated by
// the cmd/ layer from either a ledger file or the archive database.
type Entry struct {
	// Path is the relative path of the file from the source directory.
	Path string
	// Status is the ledger status ("error", "skip", "duplicate", "copy") or
	// the DB status ("failed", "mismatch", "tag_failed", "skipped", "complete").
	Status string
	// Reason is the error message or skip reason. May be empty for duplicates.
	Reason string
}

// CategoryResult holds the diagnosis category and the files that matched it.
type CategoryResult struct {
	// Category is the matched diagnosis category.
	Category *Category
	// Count is the number of files in this category.
	Count int
	// Files contains the paths of files in this category.
	Files []string
}

// SectionReport summarises one section (errors, skipped, or duplicates).
type SectionReport struct {
	// Total is the total number of files in this section.
	Total int
	// Categories contains only categories with Count > 0, in definition order.
	Categories []CategoryResult
}

// Report is the full diagnosis output produced by Summarize.
type Report struct {
	Errors     SectionReport
	Skipped    SectionReport
	Duplicates SectionReport
}

// HasProblems returns true if any section has files.
func (r *Report) HasProblems() bool {
	return r.Errors.Total > 0 || r.Skipped.Total > 0 || r.Duplicates.Total > 0
}

// Summarize takes a list of pipeline entries and produces a categorized report.
// Entries with status "copy" or "complete" are ignored — they are not problems.
// Entries with status "duplicate" are counted in the Duplicates section without
// further categorization.
func Summarize(entries []Entry) *Report {
	// categoryIndex maps category name → index into the results slice we're building.
	errorCats := make(map[string]*CategoryResult)
	skipCats := make(map[string]*CategoryResult)

	var report Report

	for _, e := range entries {
		switch e.Status {
		case "copy", "complete":
			// Not a problem — ignore.
			continue
		case "duplicate":
			report.Duplicates.Total++
			continue
		}

		cat := Classify(e.Status, e.Reason)
		if cat == nil {
			// Status is not an error or skip (e.g. "pending", "hashed") — ignore.
			continue
		}

		switch cat.Section {
		case SectionErrors:
			cr := getOrCreate(errorCats, cat)
			cr.Count++
			cr.Files = append(cr.Files, e.Path)
			report.Errors.Total++
		case SectionSkipped:
			cr := getOrCreate(skipCats, cat)
			cr.Count++
			cr.Files = append(cr.Files, e.Path)
			report.Skipped.Total++
		}
	}

	// Build ordered category slices (definition order, zero-count omitted).
	for i := range Categories {
		cat := &Categories[i]
		switch cat.Section {
		case SectionErrors:
			if cr, ok := errorCats[cat.Name]; ok {
				report.Errors.Categories = append(report.Errors.Categories, *cr)
			}
		case SectionSkipped:
			if cr, ok := skipCats[cat.Name]; ok {
				report.Skipped.Categories = append(report.Skipped.Categories, *cr)
			}
		}
	}

	return &report
}

// getOrCreate returns the existing CategoryResult for the given category name,
// or creates and inserts a new one.
func getOrCreate(m map[string]*CategoryResult, cat *Category) *CategoryResult {
	if cr, ok := m[cat.Name]; ok {
		return cr
	}
	cr := &CategoryResult{Category: cat}
	m[cat.Name] = cr
	return cr
}
