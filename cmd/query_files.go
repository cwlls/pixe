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

package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe/internal/archivedb"
)

// queryFilesCmd is the "pixe query files" subcommand.
var queryFilesCmd = &cobra.Command{
	Use:   "files",
	Short: "Search for files in the archive by date or source",
	Long: `Filter archive files by capture date range, import date range, or source directory.

Exactly one filter type must be specified:
  --from / --to          filter by capture date (YYYY-MM-DD)
  --imported-from / --imported-to  filter by import (verified) date (YYYY-MM-DD)
  --source               filter by source directory path

Date flags are inclusive. If only --from is set, --to defaults to today.
If only --to is set, --from defaults to 1900-01-01.`,
	RunE: runQueryFiles,
}

var (
	// filesFrom is the start of the capture date range filter.
	filesFrom string
	// filesTo is the end of the capture date range filter.
	filesTo string
	// filesImportedFrom is the start of the import date range filter.
	filesImportedFrom string
	// filesImportedTo is the end of the import date range filter.
	filesImportedTo string
	// filesSource filters results to files imported from this source directory.
	filesSource string
)

// runQueryFiles is the RunE handler for the "query files" subcommand.
func runQueryFiles(_ *cobra.Command, _ []string) error {
	// Determine which filter mode is active.
	hasCapture := filesFrom != "" || filesTo != ""
	hasImport := filesImportedFrom != "" || filesImportedTo != ""
	hasSource := filesSource != ""

	// Validate: at least one filter required.
	if !hasCapture && !hasImport && !hasSource {
		return fmt.Errorf("at least one filter flag is required (--from/--to, --imported-from/--imported-to, or --source)")
	}

	// Validate: mutually exclusive filter types.
	if hasCapture && hasImport {
		return fmt.Errorf("--from/--to and --imported-from/--imported-to are mutually exclusive")
	}
	if hasSource && (hasCapture || hasImport) {
		return fmt.Errorf("--source is mutually exclusive with date range flags")
	}

	var files []*archivedb.FileRecord
	var err error

	switch {
	case hasSource:
		files, err = queryDB.FilesBySource(filesSource)
		if err != nil {
			return fmt.Errorf("files by source: %w", err)
		}

	case hasCapture:
		start, end, parseErr := parseDateRange(filesFrom, filesTo)
		if parseErr != nil {
			return parseErr
		}
		files, err = queryDB.FilesByCaptureDateRange(start, end)
		if err != nil {
			return fmt.Errorf("files by capture date range: %w", err)
		}

	case hasImport:
		start, end, parseErr := parseDateRange(filesImportedFrom, filesImportedTo)
		if parseErr != nil {
			return parseErr
		}
		files, err = queryDB.FilesByImportDateRange(start, end)
		if err != nil {
			return fmt.Errorf("files by import date range: %w", err)
		}
	}

	if viper.GetBool("query_json") {
		return printFilesJSON(files)
	}
	return printFilesTable(files)
}

// parseDateRange parses from/to date strings (YYYY-MM-DD) and applies defaults.
// If from is empty, defaults to 1900-01-01. If to is empty, defaults to today.
func parseDateRange(from, to string) (time.Time, time.Time, error) {
	var start, end time.Time
	var err error

	if from != "" {
		start, err = time.Parse("2006-01-02", from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD format", from)
		}
	} else {
		start = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	if to != "" {
		end, err = time.Parse("2006-01-02", to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD format", to)
		}
		// Include the entire end day.
		end = end.Add(24*time.Hour - time.Second)
	} else {
		end = time.Now().UTC()
	}

	return start, end, nil
}

// printFilesTable writes file query results as a human-readable table.
func printFilesTable(files []*archivedb.FileRecord) error {
	if len(files) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No files found.")
		return nil
	}

	headers := []string{"SOURCE PATH", "DESTINATION", "CHECKSUM", "CAPTURE DATE", "STATUS"}
	rows := make([][]string, 0, len(files))

	var earliest, latest *time.Time
	for _, f := range files {
		dest := "—"
		if f.DestRel != nil {
			dest = *f.DestRel
		}
		checksum := "—"
		if f.Checksum != nil {
			checksum = truncChecksum(*f.Checksum)
		}
		rows = append(rows, []string{
			f.SourcePath,
			dest,
			checksum,
			formatDate(f.CaptureDate),
			f.Status,
		})

		if f.CaptureDate != nil {
			if earliest == nil || f.CaptureDate.Before(*earliest) {
				t := *f.CaptureDate
				earliest = &t
			}
			if latest == nil || f.CaptureDate.After(*latest) {
				t := *f.CaptureDate
				latest = &t
			}
		}
	}

	summary := fmt.Sprintf(
		"%s files | capture range: %s to %s",
		commaInt(len(files)),
		formatDate(earliest),
		formatDate(latest),
	)
	printQueryTable(os.Stdout, headers, rows, summary)
	return nil
}

// printFilesJSON writes file query results as a JSON object.
func printFilesJSON(files []*archivedb.FileRecord) error {
	type fileJSON struct {
		SourcePath  string  `json:"source_path"`
		Destination *string `json:"destination,omitempty"`
		Checksum    *string `json:"checksum,omitempty"`
		CaptureDate *string `json:"capture_date,omitempty"`
		Status      string  `json:"status"`
	}
	type summaryJSON struct {
		TotalFiles      int     `json:"total_files"`
		EarliestCapture *string `json:"earliest_capture,omitempty"`
		LatestCapture   *string `json:"latest_capture,omitempty"`
	}

	results := make([]fileJSON, 0, len(files))
	var earliest, latest *time.Time
	for _, f := range files {
		fj := fileJSON{
			SourcePath:  f.SourcePath,
			Destination: f.DestRel,
			Checksum:    f.Checksum,
			Status:      f.Status,
		}
		if f.CaptureDate != nil {
			s := formatDate(f.CaptureDate)
			fj.CaptureDate = &s
			if earliest == nil || f.CaptureDate.Before(*earliest) {
				t := *f.CaptureDate
				earliest = &t
			}
			if latest == nil || f.CaptureDate.After(*latest) {
				t := *f.CaptureDate
				latest = &t
			}
		}
		results = append(results, fj)
	}

	sum := summaryJSON{TotalFiles: len(files)}
	if earliest != nil {
		s := formatDate(earliest)
		sum.EarliestCapture = &s
	}
	if latest != nil {
		s := formatDate(latest)
		sum.LatestCapture = &s
	}
	return printQueryJSON(os.Stdout, "files", results, sum)
}

func init() {
	queryCmd.AddCommand(queryFilesCmd)

	queryFilesCmd.Flags().StringVar(&filesFrom, "from", "", "capture date range start (YYYY-MM-DD)")
	queryFilesCmd.Flags().StringVar(&filesTo, "to", "", "capture date range end (YYYY-MM-DD)")
	queryFilesCmd.Flags().StringVar(&filesImportedFrom, "imported-from", "", "import date range start (YYYY-MM-DD)")
	queryFilesCmd.Flags().StringVar(&filesImportedTo, "imported-to", "", "import date range end (YYYY-MM-DD)")
	queryFilesCmd.Flags().StringVar(&filesSource, "source", "", "filter by source directory path")
}
