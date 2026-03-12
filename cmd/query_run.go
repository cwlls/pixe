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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cwlls/pixe-go/internal/archivedb"
)

// queryRunCmd is the "pixe query run <id>" subcommand.
var queryRunCmd = &cobra.Command{
	Use:   "run <id>",
	Short: "Show details for a specific sort run",
	Long: `Display metadata and file list for a single sort run.

The <id> argument may be a full UUID or a unique prefix (at least 4 characters).
If the prefix matches more than one run, an error is returned.`,
	Args: cobra.ExactArgs(1),
	RunE: runQueryRun,
}

// runQueryRun is the RunE handler for the "query run <id>" subcommand.
func runQueryRun(_ *cobra.Command, args []string) error {
	prefix := args[0]

	matches, err := queryDB.GetRunByPrefix(prefix)
	if err != nil {
		return fmt.Errorf("get run by prefix: %w", err)
	}

	switch len(matches) {
	case 0:
		return fmt.Errorf("no run found matching prefix %q", prefix)
	case 1:
		// proceed
	default:
		ids := make([]string, len(matches))
		for i, r := range matches {
			ids[i] = r.ID
		}
		return fmt.Errorf("ambiguous prefix %q matches %d runs: %s", prefix, len(matches), strings.Join(ids, ", "))
	}

	run := matches[0]

	files, err := queryDB.GetFilesByRun(run.ID)
	if err != nil {
		return fmt.Errorf("get files for run: %w", err)
	}

	if jsonOut {
		return printRunJSON(run, files)
	}

	return printRunTable(run, files)
}

// printRunTable writes run detail results as a human-readable table.
func printRunTable(run *archivedb.Run, files []*archivedb.FileRecord) error {
	w := os.Stdout

	// Header block — key/value pairs.
	_, _ = fmt.Fprintf(w, "Run:         %s\n", run.ID)
	_, _ = fmt.Fprintf(w, "Version:     %s\n", run.PixeVersion)
	_, _ = fmt.Fprintf(w, "Source:      %s\n", run.Source)
	_, _ = fmt.Fprintf(w, "Destination: %s\n", run.Destination)
	_, _ = fmt.Fprintf(w, "Algorithm:   %s\n", run.Algorithm)
	_, _ = fmt.Fprintf(w, "Workers:     %d\n", run.Workers)
	_, _ = fmt.Fprintf(w, "Started:     %s\n", formatDateTime(run.StartedAt))
	if run.FinishedAt != nil {
		_, _ = fmt.Fprintf(w, "Finished:    %s\n", formatDateTime(*run.FinishedAt))
	}
	_, _ = fmt.Fprintf(w, "Status:      %s\n", run.Status)
	_, _ = fmt.Fprintln(w)

	if len(files) == 0 {
		_, _ = fmt.Fprintln(w, "No files recorded for this run.")
		return nil
	}

	// File table.
	headers := []string{"SOURCE FILE", "STATUS", "DESTINATION", "CHECKSUM", "CAPTURE DATE"}
	rows := make([][]string, 0, len(files))

	var complete, duplicates, skipped, errors int
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
			filepath.Base(f.SourcePath),
			f.Status,
			dest,
			checksum,
			formatDate(f.CaptureDate),
		})

		switch f.Status {
		case "complete":
			if f.IsDuplicate {
				duplicates++
			} else {
				complete++
			}
		case "skipped":
			skipped++
		case "failed", "mismatch", "tag_failed":
			errors++
		}
	}

	summary := fmt.Sprintf(
		"%s files | %s complete | %s duplicates | %s skipped | %s errors",
		commaInt(len(files)),
		commaInt(complete),
		commaInt(duplicates),
		commaInt(skipped),
		commaInt(errors),
	)
	printQueryTable(w, headers, rows, summary)
	return nil
}

// printRunJSON writes run detail results as a JSON object.
func printRunJSON(run *archivedb.Run, files []*archivedb.FileRecord) error {
	type runJSON struct {
		ID          string  `json:"id"`
		Version     string  `json:"version"`
		Source      string  `json:"source"`
		Destination string  `json:"destination"`
		Algorithm   string  `json:"algorithm"`
		Workers     int     `json:"workers"`
		Started     string  `json:"started"`
		Finished    *string `json:"finished,omitempty"`
		Status      string  `json:"status"`
	}
	type fileJSON struct {
		SourcePath  string  `json:"source_path"`
		Status      string  `json:"status"`
		Destination *string `json:"destination,omitempty"`
		Checksum    *string `json:"checksum,omitempty"`
		CaptureDate *string `json:"capture_date,omitempty"`
		IsDuplicate bool    `json:"is_duplicate"`
	}
	type summaryJSON struct {
		TotalFiles int `json:"total_files"`
		Complete   int `json:"complete"`
		Duplicates int `json:"duplicates"`
		Skipped    int `json:"skipped"`
		Errors     int `json:"errors"`
	}

	rj := runJSON{
		ID:          run.ID,
		Version:     run.PixeVersion,
		Source:      run.Source,
		Destination: run.Destination,
		Algorithm:   run.Algorithm,
		Workers:     run.Workers,
		Started:     formatDateTime(run.StartedAt),
		Status:      run.Status,
	}
	if run.FinishedAt != nil {
		ts := formatDateTime(*run.FinishedAt)
		rj.Finished = &ts
	}

	fjs := make([]fileJSON, 0, len(files))
	var complete, duplicates, skipped, errors int
	for _, f := range files {
		fj := fileJSON{
			SourcePath:  f.SourcePath,
			Status:      f.Status,
			Destination: f.DestRel,
			Checksum:    f.Checksum,
			IsDuplicate: f.IsDuplicate,
		}
		if f.CaptureDate != nil {
			s := formatDate(f.CaptureDate)
			fj.CaptureDate = &s
		}
		fjs = append(fjs, fj)

		switch f.Status {
		case "complete":
			if f.IsDuplicate {
				duplicates++
			} else {
				complete++
			}
		case "skipped":
			skipped++
		case "failed", "mismatch", "tag_failed":
			errors++
		}
	}

	type resultsJSON struct {
		Run   runJSON    `json:"run"`
		Files []fileJSON `json:"files"`
	}
	results := resultsJSON{Run: rj, Files: fjs}
	sum := summaryJSON{
		TotalFiles: len(files),
		Complete:   complete,
		Duplicates: duplicates,
		Skipped:    skipped,
		Errors:     errors,
	}
	return printQueryJSON(os.Stdout, "run", results, sum)
}

func init() {
	queryCmd.AddCommand(queryRunCmd)
}
