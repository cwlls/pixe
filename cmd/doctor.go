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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe/internal/archivedb"
	"github.com/cwlls/pixe/internal/dblocator"
	"github.com/cwlls/pixe/internal/doctor"
	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/manifest"
)

// doctorCmd is the "pixe doctor" subcommand.
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose problems from the last sort run",
	Long: `Summarize errors, skips, and duplicates from the most recent sort.

Run from a source directory to read the ledger (no --dest required):

  cd /path/to/photos
  pixe doctor

Add --advice for plain-language explanations and suggested actions:

  pixe doctor --advice

Use --dest to enable DB-backed mode for richer detail:

  pixe doctor --advice -d /Volumes/NAS/Photos`,
	RunE: runDoctor,
}

// runDoctor is the RunE handler for the doctor subcommand.
func runDoctor(cmd *cobra.Command, _ []string) error {
	advice, _ := cmd.Flags().GetBool("advice")
	jsonOut, _ := cmd.Flags().GetBool("json")
	runFilter, _ := cmd.Flags().GetString("run")
	destFlag := viper.GetString("doctor_dest")

	// --run requires --dest (it's a DB operation).
	if runFilter != "" && destFlag == "" {
		return fmt.Errorf("--run requires --dest (the archive directory containing the database)")
	}

	// Resolve source directory.
	sourceDir, _ := cmd.Flags().GetString("source")
	if sourceDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve current directory: %w", err)
		}
		sourceDir = cwd
	}

	// DB mode when --dest is provided.
	if destFlag != "" {
		dest, err := resolveDest("doctor_dest")
		if err != nil {
			return err
		}
		return runDoctorDB(cmd, sourceDir, dest, runFilter, advice, jsonOut)
	}

	// Ledger mode (default).
	return runDoctorLedger(sourceDir, advice, jsonOut)
}

// runDoctorLedger runs doctor in ledger-based mode (no --dest required).
func runDoctorLedger(sourceDir string, advice, jsonOut bool) error {
	lc, err := manifest.LoadLedger(sourceDir)
	if err != nil {
		return fmt.Errorf("read ledger: %w", err)
	}
	if lc == nil {
		_, _ = fmt.Fprintf(os.Stdout,
			"No ledger found in %s.\nRun pixe sort from this directory first, or use -s to specify a source directory.\n",
			sourceDir)
		return nil
	}

	// Convert ledger entries to doctor.Entry values.
	entries := ledgerEntriesToDoctorEntries(lc.Entries)
	report := doctor.Summarize(entries)

	// Render header.
	header := formatDoctorHeader(lc.Header.PixeRun, lc.Header.Destination)

	if jsonOut {
		return renderDoctorJSON(os.Stdout, report, header, lc.Header.Destination, "ledger")
	}

	_, _ = fmt.Fprintln(os.Stdout, header)

	if !report.HasProblems() {
		successful := countSuccessful(lc.Entries)
		_, _ = fmt.Fprintf(os.Stdout, "\n  No problems found. %d file(s) sorted successfully.\n", successful)
		return nil
	}

	if advice {
		renderDoctorAdvice(os.Stdout, report, lc.Header.Destination)
	} else {
		renderDoctorSummary(os.Stdout, report)
		_, _ = fmt.Fprintln(os.Stdout, "\nRun pixe doctor --advice for details and suggested actions.")
	}
	return nil
}

// runDoctorDB runs doctor in DB-backed mode (--dest provided).
func runDoctorDB(cmd *cobra.Command, sourceDir, dest, runFilter string, advice, jsonOut bool) error {
	// Validate destination directory.
	if info, err := os.Stat(dest); err != nil {
		return fmt.Errorf("destination directory %q: %w", dest, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dest)
	}

	dbPath := resolveDBPath("doctor_db_path")
	loc, err := dblocator.Resolve(dest, dbPath)
	if err != nil {
		return fmt.Errorf("resolve database location: %w", err)
	}
	if loc.Notice != "" {
		fmt.Fprintln(os.Stderr, loc.Notice)
	}

	db, err := archivedb.OpenReadOnly(loc.DBPath)
	if err != nil {
		return fmt.Errorf("open archive database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Resolve target run.
	var run *archivedb.Run
	if runFilter != "" {
		runs, err := db.GetRunByPrefix(runFilter)
		if err != nil {
			return fmt.Errorf("resolve run %q: %w", runFilter, err)
		}
		if len(runs) == 0 {
			return fmt.Errorf("no run matching %q", runFilter)
		}
		if len(runs) > 1 {
			return fmt.Errorf("ambiguous run prefix %q matches %d runs", runFilter, len(runs))
		}
		run = runs[0]
	} else {
		absSource, err := filepath.Abs(sourceDir)
		if err != nil {
			return fmt.Errorf("resolve source path: %w", err)
		}
		run, err = db.MostRecentRunBySource(absSource)
		if err != nil {
			return fmt.Errorf("find most recent run: %w", err)
		}
		if run == nil {
			// Fall back to most recent run overall.
			run, err = db.MostRecentRun()
			if err != nil {
				return fmt.Errorf("find most recent run: %w", err)
			}
		}
	}
	if run == nil {
		_, _ = fmt.Fprintln(os.Stdout, "No completed runs found in this archive.")
		return nil
	}

	// Query files for this run.
	errorFiles, err := db.FilesWithErrorsByRun(run.ID)
	if err != nil {
		return fmt.Errorf("query errors: %w", err)
	}
	skippedFiles, err := db.AllSkippedByRun(run.ID)
	if err != nil {
		return fmt.Errorf("query skipped: %w", err)
	}
	dupFiles, err := db.AllDuplicatesByRun(run.ID)
	if err != nil {
		return fmt.Errorf("query duplicates: %w", err)
	}

	// Convert to doctor.Entry values.
	var entries []doctor.Entry
	for _, f := range errorFiles {
		reason := ""
		if f.Error != nil {
			reason = *f.Error
		}
		entries = append(entries, doctor.Entry{
			Path:   f.SourcePath,
			Status: f.Status,
			Reason: reason,
		})
	}
	for _, f := range skippedFiles {
		reason := ""
		if f.SkipReason != nil {
			reason = *f.SkipReason
		}
		entries = append(entries, doctor.Entry{
			Path:   f.SourcePath,
			Status: "skip",
			Reason: reason,
		})
	}
	for _, f := range dupFiles {
		entries = append(entries, doctor.Entry{
			Path:   f.SourcePath,
			Status: "duplicate",
		})
	}

	report := doctor.Summarize(entries)
	header := formatDoctorHeader(run.StartedAt.Format(time.RFC3339), run.Destination)

	if jsonOut {
		return renderDoctorJSON(os.Stdout, report, header, run.Destination, "database")
	}

	_, _ = fmt.Fprintln(os.Stdout, header)

	if !report.HasProblems() {
		_, _ = fmt.Fprintln(os.Stdout, "\n  No problems found.")
		return nil
	}

	if advice {
		renderDoctorAdvice(os.Stdout, report, run.Destination)
	} else {
		renderDoctorSummary(os.Stdout, report)
		_, _ = fmt.Fprintln(os.Stdout, "\nRun pixe doctor --advice for details and suggested actions.")
	}
	return nil
}

// renderDoctorSummary renders the compact problem summary (default mode).
func renderDoctorSummary(w io.Writer, report *doctor.Report) {
	_, _ = fmt.Fprintln(w)
	if report.Errors.Total > 0 {
		cats := formatCategoryBreakdown(report.Errors.Categories)
		_, _ = fmt.Fprintf(w, "  %d error(s)    — %s\n", report.Errors.Total, cats)
	}
	if report.Skipped.Total > 0 {
		cats := formatCategoryBreakdown(report.Skipped.Categories)
		_, _ = fmt.Fprintf(w, "  %d skipped     — %s\n", report.Skipped.Total, cats)
	}
	if report.Duplicates.Total > 0 {
		_, _ = fmt.Fprintf(w, "  %d duplicate(s)\n", report.Duplicates.Total)
	}
}

// renderDoctorAdvice renders the full advice output (--advice mode).
func renderDoctorAdvice(w io.Writer, report *doctor.Report, destPath string) {
	destLabel := ""
	if destPath != "" {
		destLabel = " -d " + destPath
	}

	if report.Errors.Total > 0 {
		_, _ = fmt.Fprintf(w, "\nERRORS (%d file(s))\n", report.Errors.Total)
		for _, cr := range report.Errors.Categories {
			_, _ = fmt.Fprintf(w, "\n  %s (%d file(s))\n", cr.Category.Name, cr.Count)
			_, _ = fmt.Fprintln(w, wrapText(cr.Category.Description, 2, 76))
		}
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "  → Re-run sort to retry all %d errored file(s) (they are NOT marked as processed).\n", report.Errors.Total)
		if destLabel != "" {
			_, _ = fmt.Fprintf(w, "  → Or run: pixe retry%s\n", destLabel)
		}
	}

	if report.Skipped.Total > 0 {
		_, _ = fmt.Fprintf(w, "\nSKIPPED (%d file(s))\n", report.Skipped.Total)
		for _, cr := range report.Skipped.Categories {
			_, _ = fmt.Fprintf(w, "\n  %s (%d file(s))\n", cr.Category.Name, cr.Count)
			_, _ = fmt.Fprintln(w, wrapText(cr.Category.Description, 2, 76))
		}
	}

	if report.Duplicates.Total > 0 {
		_, _ = fmt.Fprintf(w, "\nDUPLICATES (%d file(s))\n", report.Duplicates.Total)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "  %d file(s) matched content already in your archive.\n", report.Duplicates.Total)
		if destLabel != "" {
			_, _ = fmt.Fprintf(w, "  → To review: pixe query duplicates%s --list\n", destLabel)
		}
	}
}

// renderDoctorJSON renders structured JSON output.
func renderDoctorJSON(w io.Writer, report *doctor.Report, header, destPath, source string) error {
	type catJSON struct {
		Name        string `json:"name"`
		Count       int    `json:"count"`
		Severity    string `json:"severity"`
		Description string `json:"description,omitempty"`
	}
	type sectionJSON struct {
		Total      int       `json:"total"`
		Categories []catJSON `json:"categories"`
	}
	type dupSectionJSON struct {
		Total int `json:"total"`
	}
	type outputJSON struct {
		Header      string         `json:"header"`
		Destination string         `json:"destination,omitempty"`
		Source      string         `json:"source"` // "ledger" or "database"
		Errors      sectionJSON    `json:"errors"`
		Skipped     sectionJSON    `json:"skipped"`
		Duplicates  dupSectionJSON `json:"duplicates"`
	}

	toSection := func(sr doctor.SectionReport) sectionJSON {
		cats := make([]catJSON, 0, len(sr.Categories))
		for _, cr := range sr.Categories {
			cats = append(cats, catJSON{
				Name:        cr.Category.Name,
				Count:       cr.Count,
				Severity:    string(cr.Category.Severity),
				Description: cr.Category.Description,
			})
		}
		return sectionJSON{Total: sr.Total, Categories: cats}
	}

	out := outputJSON{
		Header:      header,
		Destination: destPath,
		Source:      source,
		Errors:      toSection(report.Errors),
		Skipped:     toSection(report.Skipped),
		Duplicates:  dupSectionJSON{Total: report.Duplicates.Total},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(out)
}

// formatDoctorHeader formats the "Last sort: ..." header line.
func formatDoctorHeader(runTimestamp, destination string) string {
	dateStr := runTimestamp
	if t, err := time.Parse(time.RFC3339, runTimestamp); err == nil {
		dateStr = t.Local().Format("2006-01-02 15:04")
	}
	destLabel := destination
	if destination != "" {
		destLabel = "..." + filepath.Base(destination)
	}
	return fmt.Sprintf("Last sort: %s  →  %s", dateStr, destLabel)
}

// formatCategoryBreakdown formats the inline category breakdown for summary mode.
// e.g. "metadata (2), disk (1)"
func formatCategoryBreakdown(cats []doctor.CategoryResult) string {
	if len(cats) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cats))
	for _, cr := range cats {
		parts = append(parts, fmt.Sprintf("%s (%d)", cr.Category.Name, cr.Count))
	}
	return strings.Join(parts, ", ")
}

// ledgerEntriesToDoctorEntries converts ledger entries to doctor.Entry values.
// Entries with status "copy" are included but will be ignored by Summarize.
func ledgerEntriesToDoctorEntries(entries []domain.LedgerEntry) []doctor.Entry {
	result := make([]doctor.Entry, 0, len(entries))
	for _, e := range entries {
		result = append(result, doctor.Entry{
			Path:   e.Path,
			Status: e.Status,
			Reason: e.Reason,
		})
	}
	return result
}

// countSuccessful counts ledger entries with status "copy".
func countSuccessful(entries []domain.LedgerEntry) int {
	n := 0
	for _, e := range entries {
		if e.Status == domain.LedgerStatusCopy {
			n++
		}
	}
	return n
}

// wrapText wraps text to the given width with the given indent (spaces).
func wrapText(text string, indent, width int) string {
	prefix := strings.Repeat(" ", indent)
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var sb strings.Builder
	lineLen := indent
	sb.WriteString(prefix)

	for i, word := range words {
		if i == 0 {
			sb.WriteString(word)
			lineLen += len(word)
			continue
		}
		if lineLen+1+len(word) > width {
			sb.WriteString("\n")
			sb.WriteString(prefix)
			sb.WriteString(word)
			lineLen = indent + len(word)
		} else {
			sb.WriteString(" ")
			sb.WriteString(word)
			lineLen += 1 + len(word)
		}
	}
	return sb.String()
}

func init() {
	rootCmd.AddCommand(doctorCmd)

	doctorCmd.Flags().Bool("advice", false, "show plain-language explanations for each problem category")
	doctorCmd.Flags().StringP("source", "s", "", "source directory (default: current directory)")
	doctorCmd.Flags().StringP("dest", "d", "", "archive directory (enables DB-backed mode)")
	doctorCmd.Flags().String("run", "", "specific run ID, prefix match (requires --dest)")
	doctorCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")
	doctorCmd.Flags().Bool("json", false, "output as JSON")

	_ = viper.BindPFlag("doctor_dest", doctorCmd.Flags().Lookup("dest"))
	_ = viper.BindPFlag("doctor_db_path", doctorCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("doctor_json", doctorCmd.Flags().Lookup("json"))
}
