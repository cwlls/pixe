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
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe/internal/archivedb"
	"github.com/cwlls/pixe/internal/dblocator"
)

// statsCmd is the "pixe stats" subcommand.
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show archive statistics and summary dashboard",
	Long: `Stats displays a summary dashboard for a destination archive, including
total files, total size, date range, format breakdown, error rate,
and last import date. All data is read from the archive database.`,
	RunE: runStats,
}

func runStats(cmd *cobra.Command, _ []string) error {
	dir := viper.GetString("stats_dir")
	if dir == "" {
		return fmt.Errorf("--dir is required")
	}

	dbPath := viper.GetString("stats_db_path")
	loc, err := dblocator.Resolve(dir, dbPath)
	if err != nil {
		return fmt.Errorf("resolve database location: %w", err)
	}

	db, err := archivedb.OpenReadOnly(loc.DBPath)
	if err != nil {
		return fmt.Errorf("open archive database: %w", err)
	}
	defer func() { _ = db.Close() }()

	stats, err := db.ArchiveStats()
	if err != nil {
		return fmt.Errorf("query archive stats: %w", err)
	}

	breakdown, err := db.FormatBreakdown()
	if err != nil {
		return fmt.Errorf("query format breakdown: %w", err)
	}

	lastRun, err := db.LastRunDate()
	if err != nil {
		return fmt.Errorf("query last run date: %w", err)
	}

	out := cmd.OutOrStdout()

	if viper.GetBool("stats_json") {
		return printStatsJSON(out, dir, stats, breakdown, lastRun)
	}

	return printStatsHuman(out, dir, stats, breakdown, lastRun)
}

func printStatsHuman(out io.Writer, dir string, stats *archivedb.ArchiveStats, breakdown []archivedb.FormatCount, lastRun *time.Time) error {
	_, _ = fmt.Fprintf(out, "Archive: %s\n\n", dir)

	_, _ = fmt.Fprintf(out, "Files:       %s (%s)\n", formatCount(stats.Complete), formatBytes(stats.TotalSize))
	_, _ = fmt.Fprintf(out, "Duplicates:  %s\n", formatCount(stats.Duplicates))

	errCount := stats.Failed + stats.Mismatches + stats.TagFailed
	if errCount > 0 {
		_, _ = fmt.Fprintf(out, "Errors:      %s (%d failed, %d mismatch, %d tag_failed)\n",
			formatCount(errCount), stats.Failed, stats.Mismatches, stats.TagFailed)
	} else {
		_, _ = fmt.Fprintf(out, "Errors:      0\n")
	}
	_, _ = fmt.Fprintf(out, "Skipped:     %s\n", formatCount(stats.Skipped))

	_, _ = fmt.Fprintln(out)

	if stats.EarliestCapture != nil && stats.LatestCapture != nil {
		_, _ = fmt.Fprintf(out, "Date Range:  %s to %s\n",
			stats.EarliestCapture.Format("2006-01-02"),
			stats.LatestCapture.Format("2006-01-02"))
	} else {
		_, _ = fmt.Fprintf(out, "Date Range:  —\n")
	}

	if lastRun != nil {
		_, _ = fmt.Fprintf(out, "Last Import: %s\n", lastRun.Format("2006-01-02 15:04:05 UTC"))
	} else {
		_, _ = fmt.Fprintf(out, "Last Import: —\n")
	}

	_, _ = fmt.Fprintf(out, "Total Runs:  %d\n", stats.RunCount)

	if len(breakdown) > 0 {
		_, _ = fmt.Fprintln(out)
		_, _ = fmt.Fprintln(out, "Format Breakdown:")
		total := 0
		for _, fc := range breakdown {
			total += fc.Count
		}
		for _, fc := range breakdown {
			pct := float64(fc.Count) / float64(total) * 100.0
			_, _ = fmt.Fprintf(out, "  %-8s %s (%.1f%%)\n", fc.Extension, formatCount(fc.Count), pct)
		}
	}

	return nil
}

func printStatsJSON(out io.Writer, dir string, stats *archivedb.ArchiveStats, breakdown []archivedb.FormatCount, lastRun *time.Time) error {
	type jsonOutput struct {
		Dir             string                  `json:"dir"`
		Complete        int                     `json:"complete"`
		Duplicates      int                     `json:"duplicates"`
		Failed          int                     `json:"failed"`
		Mismatches      int                     `json:"mismatches"`
		TagFailed       int                     `json:"tag_failed"`
		Skipped         int                     `json:"skipped"`
		TotalSize       int64                   `json:"total_size_bytes"`
		RunCount        int                     `json:"run_count"`
		EarliestCapture *string                 `json:"earliest_capture,omitempty"`
		LatestCapture   *string                 `json:"latest_capture,omitempty"`
		LastImport      *string                 `json:"last_import,omitempty"`
		Formats         []archivedb.FormatCount `json:"formats,omitempty"`
	}

	result := jsonOutput{
		Dir:        dir,
		Complete:   stats.Complete,
		Duplicates: stats.Duplicates,
		Failed:     stats.Failed,
		Mismatches: stats.Mismatches,
		TagFailed:  stats.TagFailed,
		Skipped:    stats.Skipped,
		TotalSize:  stats.TotalSize,
		RunCount:   stats.RunCount,
		Formats:    breakdown,
	}

	if stats.EarliestCapture != nil {
		s := stats.EarliestCapture.Format("2006-01-02")
		result.EarliestCapture = &s
	}
	if stats.LatestCapture != nil {
		s := stats.LatestCapture.Format("2006-01-02")
		result.LatestCapture = &s
	}
	if lastRun != nil {
		s := lastRun.Format(time.RFC3339)
		result.LastImport = &s
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// formatCount returns a comma-separated integer string (e.g. "8,421").
func formatCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%s,%03d", formatCount(n/1000), n%1000)
}

func init() {
	rootCmd.AddCommand(statsCmd)

	statsCmd.Flags().StringP("dir", "d", "", "destination directory containing the archive (required)")
	statsCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database")
	statsCmd.Flags().Bool("json", false, "emit output as JSON")

	_ = statsCmd.MarkFlagRequired("dir")

	_ = viper.BindPFlag("stats_dir", statsCmd.Flags().Lookup("dir"))
	_ = viper.BindPFlag("stats_db_path", statsCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("stats_json", statsCmd.Flags().Lookup("json"))
}
