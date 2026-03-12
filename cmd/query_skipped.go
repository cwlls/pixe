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
	"strings"

	"github.com/spf13/cobra"
)

// querySkippedCmd is the "pixe query skipped" subcommand.
var querySkippedCmd = &cobra.Command{
	Use:   "skipped",
	Short: "List all files that were skipped during sorting",
	Long:  `Display all files with status "skipped" across all runs, along with the skip reason.`,
	RunE:  runQuerySkipped,
}

// runQuerySkipped is the RunE handler for the "query skipped" subcommand.
func runQuerySkipped(_ *cobra.Command, _ []string) error {
	files, err := queryDB.AllSkipped()
	if err != nil {
		return fmt.Errorf("list skipped: %w", err)
	}

	if jsonOut {
		type skippedJSON struct {
			SourcePath string  `json:"source_path"`
			Reason     *string `json:"reason,omitempty"`
		}
		type summaryJSON struct {
			TotalSkipped       int `json:"total_skipped"`
			UnsupportedFormat  int `json:"unsupported_format"`
			PreviouslyImported int `json:"previously_imported"`
		}

		results := make([]skippedJSON, 0, len(files))
		var unsupported, previously int
		for _, f := range files {
			results = append(results, skippedJSON{
				SourcePath: f.SourcePath,
				Reason:     f.SkipReason,
			})
			if f.SkipReason != nil {
				switch {
				case strings.HasPrefix(*f.SkipReason, "unsupported"):
					unsupported++
				case strings.HasPrefix(*f.SkipReason, "previously"):
					previously++
				}
			}
		}
		sum := summaryJSON{
			TotalSkipped:       len(files),
			UnsupportedFormat:  unsupported,
			PreviouslyImported: previously,
		}
		return printQueryJSON(os.Stdout, "skipped", results, sum)
	}

	if len(files) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No skipped files found.")
		return nil
	}

	headers := []string{"SOURCE PATH", "REASON"}
	rows := make([][]string, 0, len(files))
	var unsupported, previously int
	for _, f := range files {
		reason := "—"
		if f.SkipReason != nil {
			reason = *f.SkipReason
			switch {
			case strings.HasPrefix(reason, "unsupported"):
				unsupported++
			case strings.HasPrefix(reason, "previously"):
				previously++
			}
		}
		rows = append(rows, []string{f.SourcePath, reason})
	}

	summary := fmt.Sprintf(
		"%s skipped files | %s unsupported format | %s previously imported",
		commaInt(len(files)),
		commaInt(unsupported),
		commaInt(previously),
	)
	printQueryTable(os.Stdout, headers, rows, summary)
	return nil
}

func init() {
	queryCmd.AddCommand(querySkippedCmd)
}
