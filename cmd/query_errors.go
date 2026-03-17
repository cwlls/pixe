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
	"github.com/spf13/viper"
)

// queryErrorsCmd is the "pixe query errors" subcommand.
var queryErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "List all files that encountered errors during sorting",
	Long:  `Display all files in error states (failed, mismatch, tag_failed) across all runs.`,
	RunE:  runQueryErrors,
}

// runQueryErrors is the RunE handler for the "query errors" subcommand.
func runQueryErrors(cmd *cobra.Command, _ []string) error {
	listMode, _ := cmd.Flags().GetBool("list")
	if listMode && viper.GetBool("query_json") {
		return fmt.Errorf("--list and --json are mutually exclusive")
	}

	runID, err := resolveQueryRunFilter(cmd)
	if err != nil {
		return err
	}

	files, err := queryDB.FilesWithErrorsByRun(runID)
	if err != nil {
		return fmt.Errorf("list errors: %w", err)
	}

	if listMode {
		for _, f := range files {
			_, _ = fmt.Fprintln(os.Stdout, f.SourcePath)
		}
		return nil
	}

	if viper.GetBool("query_json") {
		type errJSON struct {
			SourcePath string  `json:"source_path"`
			Status     string  `json:"status"`
			Error      *string `json:"error,omitempty"`
			RunSource  string  `json:"run_source"`
		}
		type summaryJSON struct {
			TotalErrors int `json:"total_errors"`
			Failed      int `json:"failed"`
			Mismatch    int `json:"mismatch"`
			TagFailed   int `json:"tag_failed"`
		}

		results := make([]errJSON, 0, len(files))
		var failed, mismatch, tagFailed int
		for _, f := range files {
			results = append(results, errJSON{
				SourcePath: f.SourcePath,
				Status:     f.Status,
				Error:      f.Error,
				RunSource:  f.RunSource,
			})
			switch f.Status {
			case "failed":
				failed++
			case "mismatch":
				mismatch++
			case "tag_failed":
				tagFailed++
			}
		}
		sum := summaryJSON{
			TotalErrors: len(files),
			Failed:      failed,
			Mismatch:    mismatch,
			TagFailed:   tagFailed,
		}
		return printQueryJSON(os.Stdout, "errors", results, sum)
	}

	if len(files) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No errors found.")
		return nil
	}

	headers := []string{"SOURCE PATH", "STATUS", "ERROR", "RUN SOURCE"}
	rows := make([][]string, 0, len(files))
	var failed, mismatch, tagFailed int
	for _, f := range files {
		errMsg := "—"
		if f.Error != nil {
			errMsg = *f.Error
		}
		rows = append(rows, []string{
			f.SourcePath,
			f.Status,
			errMsg,
			f.RunSource,
		})
		switch f.Status {
		case "failed":
			failed++
		case "mismatch":
			mismatch++
		case "tag_failed":
			tagFailed++
		}
	}

	summary := fmt.Sprintf(
		"%s errors | %s failed | %s mismatch | %s tag_failed",
		commaInt(len(files)),
		commaInt(failed),
		commaInt(mismatch),
		commaInt(tagFailed),
	)
	if runID != "" {
		summary += " (run " + truncID(runID) + ")"
	}
	printQueryTable(os.Stdout, headers, rows, summary)
	return nil
}

func init() {
	queryCmd.AddCommand(queryErrorsCmd)
	queryErrorsCmd.Flags().String("run", "", "filter to a specific run (prefix match)")
	queryErrorsCmd.Flags().Bool("list", false, "output one source file path per line (mutually exclusive with --json)")
}
