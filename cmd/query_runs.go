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

// queryRunsCmd is the "pixe query runs" subcommand.
var queryRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List all sort runs recorded in the archive database",
	Long:  `Display a summary of every sort run recorded in the archive database, ordered by start time (most recent first).`,
	RunE:  runQueryRuns,
}

// runQueryRuns is the RunE handler for the "query runs" subcommand.
func runQueryRuns(_ *cobra.Command, _ []string) error {
	summaries, err := queryDB.ListRuns()
	if err != nil {
		return fmt.Errorf("list runs: %w", err)
	}

	if jsonOut {
		type runJSON struct {
			ID         string  `json:"id"`
			Version    string  `json:"version"`
			Source     string  `json:"source"`
			Started    string  `json:"started"`
			Status     string  `json:"status"`
			FileCount  int     `json:"file_count"`
			FinishedAt *string `json:"finished_at,omitempty"`
		}
		type summaryJSON struct {
			TotalRuns  int `json:"total_runs"`
			TotalFiles int `json:"total_files"`
		}

		results := make([]runJSON, 0, len(summaries))
		totalFiles := 0
		for _, s := range summaries {
			r := runJSON{
				ID:        s.ID,
				Version:   s.PixeVersion,
				Source:    s.Source,
				Started:   formatDateTime(s.StartedAt),
				Status:    s.Status,
				FileCount: s.FileCount,
			}
			if s.FinishedAt != nil {
				ts := formatDateTime(*s.FinishedAt)
				r.FinishedAt = &ts
			}
			results = append(results, r)
			totalFiles += s.FileCount
		}
		sum := summaryJSON{TotalRuns: len(summaries), TotalFiles: totalFiles}
		return printQueryJSON(os.Stdout, "runs", results, sum)
	}

	if len(summaries) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No runs found.")
		return nil
	}

	headers := []string{"RUN ID", "VERSION", "SOURCE", "STARTED", "STATUS", "FILES"}
	rows := make([][]string, 0, len(summaries))
	totalFiles := 0
	for _, s := range summaries {
		rows = append(rows, []string{
			truncID(s.ID),
			s.PixeVersion,
			s.Source,
			formatDateTime(s.StartedAt),
			s.Status,
			commaInt(s.FileCount),
		})
		totalFiles += s.FileCount
	}

	summary := fmt.Sprintf("%s runs | %s total files", commaInt(len(summaries)), commaInt(totalFiles))
	printQueryTable(os.Stdout, headers, rows, summary)
	return nil
}

func init() {
	queryCmd.AddCommand(queryRunsCmd)
}
