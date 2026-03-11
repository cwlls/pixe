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
)

var queryInventoryCmd = &cobra.Command{
	Use:   "inventory",
	Short: "List all canonical files in the archive",
	Long:  `Display all completed, non-duplicate files — the canonical contents of the archive.`,
	RunE:  runQueryInventory,
}

func runQueryInventory(_ *cobra.Command, _ []string) error {
	entries, err := queryDB.ArchiveInventory()
	if err != nil {
		return fmt.Errorf("archive inventory: %w", err)
	}

	if jsonOut {
		type entryJSON struct {
			Destination string  `json:"destination"`
			Checksum    string  `json:"checksum"`
			CaptureDate *string `json:"capture_date,omitempty"`
		}
		type summaryJSON struct {
			TotalFiles      int     `json:"total_files"`
			EarliestCapture *string `json:"earliest_capture,omitempty"`
			LatestCapture   *string `json:"latest_capture,omitempty"`
		}

		results := make([]entryJSON, 0, len(entries))
		var earliest, latest *time.Time
		for _, e := range entries {
			ej := entryJSON{
				Destination: e.DestRel,
				Checksum:    e.Checksum,
			}
			if e.CaptureDate != nil {
				s := formatDate(e.CaptureDate)
				ej.CaptureDate = &s
				if earliest == nil || e.CaptureDate.Before(*earliest) {
					t := *e.CaptureDate
					earliest = &t
				}
				if latest == nil || e.CaptureDate.After(*latest) {
					t := *e.CaptureDate
					latest = &t
				}
			}
			results = append(results, ej)
		}

		sum := summaryJSON{TotalFiles: len(entries)}
		if earliest != nil {
			s := formatDate(earliest)
			sum.EarliestCapture = &s
		}
		if latest != nil {
			s := formatDate(latest)
			sum.LatestCapture = &s
		}
		return printQueryJSON(os.Stdout, "inventory", results, sum)
	}

	if len(entries) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No files in archive.")
		return nil
	}

	headers := []string{"DESTINATION", "CHECKSUM", "CAPTURE DATE"}
	rows := make([][]string, 0, len(entries))
	var earliest, latest *time.Time
	for _, e := range entries {
		rows = append(rows, []string{
			e.DestRel,
			truncChecksum(e.Checksum),
			formatDate(e.CaptureDate),
		})
		if e.CaptureDate != nil {
			if earliest == nil || e.CaptureDate.Before(*earliest) {
				t := *e.CaptureDate
				earliest = &t
			}
			if latest == nil || e.CaptureDate.After(*latest) {
				t := *e.CaptureDate
				latest = &t
			}
		}
	}

	summary := fmt.Sprintf(
		"%s files | capture range: %s to %s",
		commaInt(len(entries)),
		formatDate(earliest),
		formatDate(latest),
	)
	printQueryTable(os.Stdout, headers, rows, summary)
	return nil
}

func init() {
	queryCmd.AddCommand(queryInventoryCmd)
}
