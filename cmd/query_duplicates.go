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

	"github.com/spf13/cobra"
)

// queryDuplicatesCmd is the "pixe query duplicates" subcommand.
var queryDuplicatesCmd = &cobra.Command{
	Use:   "duplicates",
	Short: "List all duplicate files in the archive",
	Long: `Display all files that were detected as duplicates during sorting.

Use --pairs to show each duplicate alongside the original file it duplicates.`,
	RunE: runQueryDuplicates,
}

// duplicatePairs enables paired output showing each duplicate alongside its original.
var duplicatePairs bool

// runQueryDuplicates is the RunE handler for the "query duplicates" subcommand.
func runQueryDuplicates(_ *cobra.Command, _ []string) error {
	if duplicatePairs {
		return runQueryDuplicatePairs()
	}
	return runQueryDuplicateList()
}

// runQueryDuplicateList lists all duplicate files without pairing.
func runQueryDuplicateList() error {
	files, err := queryDB.AllDuplicates()
	if err != nil {
		return fmt.Errorf("list duplicates: %w", err)
	}

	if jsonOut {
		type dupJSON struct {
			SourcePath  string  `json:"source_path"`
			Destination *string `json:"destination,omitempty"`
			Checksum    *string `json:"checksum,omitempty"`
			CaptureDate *string `json:"capture_date,omitempty"`
		}
		type summaryJSON struct {
			TotalDuplicates int `json:"total_duplicates"`
		}

		results := make([]dupJSON, 0, len(files))
		for _, f := range files {
			d := dupJSON{
				SourcePath:  f.SourcePath,
				Destination: f.DestRel,
				Checksum:    f.Checksum,
			}
			if f.CaptureDate != nil {
				s := formatDate(f.CaptureDate)
				d.CaptureDate = &s
			}
			results = append(results, d)
		}
		return printQueryJSON(os.Stdout, "duplicates", results, summaryJSON{TotalDuplicates: len(files)})
	}

	if len(files) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No duplicates found.")
		return nil
	}

	headers := []string{"SOURCE PATH", "DESTINATION", "CHECKSUM", "CAPTURE DATE"}
	rows := make([][]string, 0, len(files))
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
		})
	}

	printQueryTable(os.Stdout, headers, rows, fmt.Sprintf("%s duplicates", commaInt(len(files))))
	return nil
}

// runQueryDuplicatePairs lists each duplicate alongside its original.
func runQueryDuplicatePairs() error {
	pairs, err := queryDB.DuplicatePairs()
	if err != nil {
		return fmt.Errorf("list duplicate pairs: %w", err)
	}

	if jsonOut {
		type pairJSON struct {
			DuplicateSource string `json:"duplicate_source"`
			DuplicateDest   string `json:"duplicate_dest"`
			OriginalDest    string `json:"original_dest"`
		}
		type summaryJSON struct {
			TotalPairs int `json:"total_pairs"`
		}

		results := make([]pairJSON, 0, len(pairs))
		for _, p := range pairs {
			results = append(results, pairJSON{
				DuplicateSource: p.DuplicateSource,
				DuplicateDest:   p.DuplicateDest,
				OriginalDest:    p.OriginalDest,
			})
		}
		return printQueryJSON(os.Stdout, "duplicate_pairs", results, summaryJSON{TotalPairs: len(pairs)})
	}

	if len(pairs) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No duplicates found.")
		return nil
	}

	headers := []string{"DUPLICATE SOURCE", "DUPLICATE DEST", "ORIGINAL"}
	rows := make([][]string, 0, len(pairs))
	for _, p := range pairs {
		rows = append(rows, []string{p.DuplicateSource, p.DuplicateDest, p.OriginalDest})
	}

	printQueryTable(os.Stdout, headers, rows, fmt.Sprintf("%s duplicate pairs", commaInt(len(pairs))))
	return nil
}

func init() {
	queryCmd.AddCommand(queryDuplicatesCmd)
	queryDuplicatesCmd.Flags().BoolVar(&duplicatePairs, "pairs", false, "show each duplicate paired with its original")
}
