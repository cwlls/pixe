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

// Severity indicates whether a category requires user action.
type Severity string

const (
	// SeverityActionable means the user should do something about these files.
	SeverityActionable Severity = "actionable"
	// SeverityInformational means these files were intentionally skipped; no action needed.
	SeverityInformational Severity = "informational"
)

// Section identifies which part of the doctor output a category belongs to.
type Section string

const (
	SectionErrors     Section = "errors"
	SectionSkipped    Section = "skipped"
	SectionDuplicates Section = "duplicates"
)

// Category describes a class of pipeline outcome with a human-readable
// explanation and suggested action.
type Category struct {
	// Name is the short display title, e.g. "Corrupted metadata".
	Name string
	// Section is which output section this category belongs to.
	Section Section
	// Severity indicates whether user action is needed.
	Severity Severity
	// Description is the 2-3 sentence plain-language explanation shown in
	// --advice mode.
	Description string

	// statuses are the ledger or DB status strings this category matches.
	// Ledger statuses: "error", "skip", "duplicate".
	// DB statuses: "failed", "mismatch", "tag_failed", "skipped".
	statuses []string
	// patterns are substrings matched (case-insensitively) against the
	// reason/error field. All patterns are OR'd — any match wins.
	// An empty patterns slice means "match any reason" (catch-all).
	patterns []string
}

// Categories is the ordered list of all diagnosis categories. Classify
// iterates this slice in order and returns the first match, so more specific
// categories must appear before broader ones. The Uncategorized catch-all is
// always last.
var Categories = []Category{
	// -----------------------------------------------------------------------
	// Error categories (ledger status "error"; DB statuses "failed",
	// "mismatch", "tag_failed")
	// -----------------------------------------------------------------------
	{
		Name:     "Corrupted metadata",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "These files have damaged or unreadable date information. " +
			"Pixe couldn't determine when the photo was taken. " +
			"You can re-sort them — they'll be filed under the fallback date (Feb 20, 1902) — " +
			"or fix the metadata first with a tool like ExifTool.",
		statuses: []string{"error", "failed"},
		patterns: []string{"extract date:"},
	},
	{
		Name:     "Corrupt file structure",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "These files appear to be damaged or truncated. " +
			"Pixe couldn't read their contents to compute a checksum. " +
			"Try opening the files in another application to confirm they're intact, " +
			"then re-sort to retry.",
		statuses: []string{"error", "failed"},
		patterns: []string{"open hashable reader:", "hash payload:"},
	},
	{
		Name:     "Disk or permission error",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "Pixe couldn't read the source file or write to the destination. " +
			"Check that the source drive is still connected, the destination has free space, " +
			"and that you have permission to read these files.",
		statuses: []string{"error", "failed"},
		patterns: []string{
			"permission denied",
			"input/output error",
			"no such file",
			"no space left",
			"read ",
			"write ",
		},
	},
	{
		Name:     "Copy failure",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "The file copy didn't complete successfully. " +
			"This can happen if the destination drive disconnected mid-copy or ran out of space. " +
			"Re-sort to retry — Pixe will attempt the copy again from scratch.",
		statuses: []string{"error", "failed"},
		patterns: []string{"copy:"},
	},
	{
		Name:     "Integrity mismatch",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "The file's checksum after copying didn't match what was computed before. " +
			"This usually means the destination drive has a hardware problem or the file changed " +
			"during the copy. The destination copy has been preserved for inspection. " +
			"Re-sort to retry — Pixe will re-copy from the source.",
		statuses: []string{"mismatch", "error"},
		patterns: []string{"verify:"},
	},
	{
		Name:     "Tag injection failed",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "The file was copied and verified successfully, but writing the metadata " +
			"sidecar (XMP) failed. The file itself is safe in your archive. " +
			"Re-sort to retry the tagging step.",
		statuses: []string{"tag_failed", "error"},
		patterns: []string{"tag "},
	},
	{
		Name:     "Database error",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "An internal database or filesystem operation failed. " +
			"This is uncommon and may indicate a problem with the archive database file. " +
			"Try running 'pixe clean' to repair the database, then re-sort to retry.",
		statuses: []string{"error", "failed"},
		patterns: []string{"dedup check:", "complete with dedup check:", "promote:", "relocate"},
	},

	// -----------------------------------------------------------------------
	// Skip categories (ledger status "skip"; DB status "skipped")
	// -----------------------------------------------------------------------
	{
		Name:     "Unsupported format",
		Section:  SectionSkipped,
		Severity: SeverityInformational,
		Description: "These file types aren't supported by Pixe. " +
			"If they're system junk files (.DS_Store, Thumbs.db, .txt), you can safely ignore them. " +
			"If they're media files you expected to be sorted, check the supported formats list " +
			"at https://github.com/cwlls/pixe.",
		statuses: []string{"skip", "skipped"},
		patterns: []string{"unsupported format:"},
	},
	{
		Name:     "Previously imported",
		Section:  SectionSkipped,
		Severity: SeverityInformational,
		Description: "These files were already successfully sorted in a prior run. " +
			"No action needed — they're already in your archive.",
		statuses: []string{"skip", "skipped"},
		patterns: []string{"previously imported"},
	},
	{
		Name:     "Outside date range",
		Section:  SectionSkipped,
		Severity: SeverityInformational,
		Description: "These files were filtered out by your --since or --before date range. " +
			"If you want to include them, re-sort without the date filter.",
		statuses: []string{"skip", "skipped"},
		patterns: []string{"outside date range:"},
	},
	{
		Name:     "Symbolic link",
		Section:  SectionSkipped,
		Severity: SeverityInformational,
		Description: "Pixe skips symbolic links for safety — following symlinks could " +
			"cause files to be processed multiple times or lead outside the source directory. " +
			"No action needed.",
		statuses: []string{"skip", "skipped"},
		patterns: []string{"symlink"},
	},
	{
		Name:     "Hidden file",
		Section:  SectionSkipped,
		Severity: SeverityInformational,
		Description: "Files starting with a dot (.) are skipped — these are typically " +
			"system or application metadata files (.DS_Store, .Spotlight-V100, etc.). " +
			"No action needed.",
		statuses: []string{"skip", "skipped"},
		patterns: []string{"dotfile"},
	},
	{
		Name:     "Detection error",
		Section:  SectionSkipped,
		Severity: SeverityActionable,
		Description: "The file looked like a supported format based on its extension, " +
			"but Pixe couldn't confirm it by reading the file's contents. " +
			"The file may be corrupted or have a mismatched extension. " +
			"Try opening it in another application to check.",
		statuses: []string{"skip", "skipped"},
		patterns: []string{"detection error:"},
	},

	// -----------------------------------------------------------------------
	// Catch-all — must be last. Matches any unrecognized error or skip.
	// -----------------------------------------------------------------------
	{
		Name:     "Unexpected error",
		Section:  SectionErrors,
		Severity: SeverityActionable,
		Description: "An unexpected error occurred that Pixe doesn't have specific advice for. " +
			"Re-sort to retry. If the problem persists, check the full error message with " +
			"'pixe query errors' and consider filing a bug report.",
		statuses: []string{"error", "failed", "mismatch", "tag_failed"},
		patterns: []string{}, // empty = catch-all for error statuses
	},
}
