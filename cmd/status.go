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
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/ignore"
	"github.com/cwlls/pixe-go/internal/manifest"
)

// statusFile holds the classification result for a single file.
type statusFile struct {
	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
	Matches     string `json:"matches,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// statusResult holds the complete classification of a source directory.
type statusResult struct {
	Source       string
	Ledger       *manifest.LedgerContents // nil if no ledger exists
	Sorted       []statusFile
	Duplicates   []statusFile
	Errored      []statusFile
	Unsorted     []statusFile
	Unrecognized []statusFile
}

// total returns the sum of all file categories.
func (r *statusResult) total() int {
	return len(r.Sorted) + len(r.Duplicates) + len(r.Errored) +
		len(r.Unsorted) + len(r.Unrecognized)
}

// statusCmd is the "pixe status" subcommand.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the sorting status of a source directory",
	Long: `Status inspects a source directory (defaulting to the current working directory)
and reports which files have been sorted, which are duplicates, which encountered
errors, and which still need sorting.

It reads the ledger file (dirA/.pixe_ledger.json) left by the most recent
'pixe sort' run. No archive database or destination directory is required —
the command works entirely from the source directory.

If no ledger exists, all recognized files are reported as unsorted.`,
	RunE: runStatus,
}

// runStatus is the RunE handler for the status subcommand.
func runStatus(cmd *cobra.Command, _ []string) error {
	// ------------------------------------------------------------------
	// 1. Read flags from Viper.
	// ------------------------------------------------------------------
	source := viper.GetString("status_source")
	recursive := viper.GetBool("status_recursive")
	ignorePatterns := viper.GetStringSlice("status_ignore")
	jsonFlag := viper.GetBool("status_json")

	if source == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve current directory: %w", err)
		}
		source = cwd
	}

	// ------------------------------------------------------------------
	// 2. Validate source directory.
	// ------------------------------------------------------------------
	abs, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve source path %q: %w", source, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("source directory %q: %w", abs, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source %q is not a directory", abs)
	}

	// ------------------------------------------------------------------
	// 3. Build handler registry.
	// ------------------------------------------------------------------
	reg := buildRegistry()

	// ------------------------------------------------------------------
	// 4. Walk dirA.
	// ------------------------------------------------------------------
	walkOpts := discovery.WalkOptions{
		Recursive: recursive,
		Ignore:    ignore.New(ignorePatterns),
	}
	discovered, skipped, err := discovery.Walk(abs, reg, walkOpts)
	if err != nil {
		return fmt.Errorf("status: walk source: %w", err)
	}

	// ------------------------------------------------------------------
	// 5. Load ledger (nil if none exists — not an error).
	// ------------------------------------------------------------------
	lc, err := manifest.LoadLedger(abs)
	if err != nil {
		return fmt.Errorf("status: load ledger: %w", err)
	}

	// ------------------------------------------------------------------
	// 6. Build ledger lookup map: relPath → LedgerEntry.
	// ------------------------------------------------------------------
	var ledgerEntryCount int
	if lc != nil {
		ledgerEntryCount = len(lc.Entries)
	}
	ledgerMap := make(map[string]domain.LedgerEntry, ledgerEntryCount)
	if lc != nil {
		for _, entry := range lc.Entries {
			ledgerMap[entry.Path] = entry
		}
	}

	// ------------------------------------------------------------------
	// 7. Classify discovered files into four buckets.
	// ------------------------------------------------------------------
	var sorted, duplicates, errored, unsorted []statusFile

	for _, df := range discovered {
		entry, found := ledgerMap[df.RelPath]
		if !found {
			unsorted = append(unsorted, statusFile{Path: df.RelPath})
			continue
		}
		switch entry.Status {
		case domain.LedgerStatusCopy:
			sorted = append(sorted, statusFile{
				Path:        df.RelPath,
				Destination: entry.Destination,
			})
		case domain.LedgerStatusDuplicate:
			duplicates = append(duplicates, statusFile{
				Path:        df.RelPath,
				Destination: entry.Destination,
				Matches:     entry.Matches,
			})
		case domain.LedgerStatusError:
			errored = append(errored, statusFile{
				Path:   df.RelPath,
				Reason: entry.Reason,
			})
		default:
			// "skip" or any unknown status → treat as unsorted.
			unsorted = append(unsorted, statusFile{Path: df.RelPath})
		}
	}

	// ------------------------------------------------------------------
	// 8. Classify discovery-skipped files as unrecognized.
	// ------------------------------------------------------------------
	var unrecognized []statusFile
	for _, sf := range skipped {
		unrecognized = append(unrecognized, statusFile{
			Path:   sf.Path,
			Reason: sf.Reason,
		})
	}

	// ------------------------------------------------------------------
	// 9. Sort each bucket alphabetically by Path.
	// ------------------------------------------------------------------
	sortByPath := func(files []statusFile) {
		sort.Slice(files, func(i, j int) bool {
			return files[i].Path < files[j].Path
		})
	}
	sortByPath(sorted)
	sortByPath(duplicates)
	sortByPath(errored)
	sortByPath(unsorted)
	sortByPath(unrecognized)

	// ------------------------------------------------------------------
	// 10. Produce output.
	// ------------------------------------------------------------------
	r := &statusResult{
		Source:       abs,
		Ledger:       lc,
		Sorted:       sorted,
		Duplicates:   duplicates,
		Errored:      errored,
		Unsorted:     unsorted,
		Unrecognized: unrecognized,
	}

	out := cmd.OutOrStdout()
	if jsonFlag {
		return printStatusJSON(out, r)
	}
	printStatusTable(out, r)
	return nil
}

// ---------------------------------------------------------------------------
// Human-readable table formatter (§7.4.4)
// ---------------------------------------------------------------------------

// printStatusTable writes a human-readable status report to w.
func printStatusTable(w io.Writer, r *statusResult) {
	// Header.
	_, _ = fmt.Fprintf(w, "Source: %s\n", r.Source)
	if r.Ledger == nil {
		_, _ = fmt.Fprintln(w, "Ledger: none found — no prior sort runs recorded for this directory.")
	} else {
		h := r.Ledger.Header
		runID := truncID(h.RunID)
		recursive := "no"
		if h.Recursive {
			recursive = "yes"
		}
		_, _ = fmt.Fprintf(w, "Ledger: run %s, %s (recursive: %s)\n",
			runID, h.PixeRun, recursive)
	}

	// Sections — only printed when non-empty.
	printSection(w, "SORTED", r.Sorted, func(f statusFile) string {
		if f.Destination != "" {
			return "→ " + f.Destination
		}
		return ""
	})
	printSection(w, "DUPLICATE", r.Duplicates, func(f statusFile) string {
		if f.Matches != "" {
			return "→ matches " + f.Matches
		}
		return ""
	})
	printSection(w, "ERRORED", r.Errored, func(f statusFile) string {
		return "→ " + f.Reason
	})
	printSection(w, "UNSORTED", r.Unsorted, func(_ statusFile) string {
		return ""
	})
	printSection(w, "UNRECOGNIZED", r.Unrecognized, func(f statusFile) string {
		return "→ " + f.Reason
	})

	// Summary line.
	_, _ = fmt.Fprintln(w, buildSummaryLine(r))
}

// printSection writes one categorized section to w, skipping it entirely when
// files is empty. detail returns the right-hand annotation for each file
// (empty string means no annotation is printed for that file).
func printSection(w io.Writer, heading string, files []statusFile, detail func(statusFile) string) {
	if len(files) == 0 {
		return
	}
	noun := "files"
	if len(files) == 1 {
		noun = "file"
	}
	_, _ = fmt.Fprintf(w, "\n%s (%d %s)\n", heading, len(files), noun)

	// Compute left-column width for alignment.
	maxPath := 0
	for _, f := range files {
		if len(f.Path) > maxPath {
			maxPath = len(f.Path)
		}
	}

	for _, f := range files {
		ann := detail(f)
		if ann != "" {
			_, _ = fmt.Fprintf(w, "  %-*s  %s\n", maxPath, f.Path, ann)
		} else {
			_, _ = fmt.Fprintf(w, "  %s\n", f.Path)
		}
	}
}

// buildSummaryLine constructs the pipe-separated summary line.
// Zero-count categories (except total) are omitted.
func buildSummaryLine(r *statusResult) string {
	total := r.total()
	parts := []string{fmt.Sprintf("%d total", total)}

	add := func(n int, label string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, label))
		}
	}
	add(len(r.Sorted), "sorted")
	add(len(r.Duplicates), "duplicates")
	add(len(r.Errored), "errored")
	add(len(r.Unsorted), "unsorted")
	add(len(r.Unrecognized), "unrecognized")

	return strings.Join(parts, " | ")
}

// ---------------------------------------------------------------------------
// JSON formatter (§7.4.5)
// ---------------------------------------------------------------------------

// ledgerInfo is the JSON representation of ledger header metadata.
type ledgerInfo struct {
	RunID       string `json:"run_id"`
	PixeVersion string `json:"pixe_version"`
	Timestamp   string `json:"timestamp"`
	Recursive   bool   `json:"recursive"`
}

// statusSummary holds aggregate counts for JSON output.
type statusSummary struct {
	Total        int `json:"total"`
	Sorted       int `json:"sorted"`
	Duplicates   int `json:"duplicates"`
	Errored      int `json:"errored"`
	Unsorted     int `json:"unsorted"`
	Unrecognized int `json:"unrecognized"`
}

// statusJSON is the top-level JSON envelope for --json output.
type statusJSON struct {
	Source       string        `json:"source"`
	Ledger       *ledgerInfo   `json:"ledger"` // null when no ledger
	Sorted       []statusFile  `json:"sorted"`
	Duplicates   []statusFile  `json:"duplicates"`
	Errored      []statusFile  `json:"errored"`
	Unsorted     []statusFile  `json:"unsorted"`
	Unrecognized []statusFile  `json:"unrecognized"`
	Summary      statusSummary `json:"summary"`
}

// printStatusJSON writes the status report as an indented JSON object to w.
func printStatusJSON(w io.Writer, r *statusResult) error {
	// Ensure empty slices serialize as [] not null.
	coerce := func(s []statusFile) []statusFile {
		if s == nil {
			return []statusFile{}
		}
		return s
	}

	out := statusJSON{
		Source:       r.Source,
		Sorted:       coerce(r.Sorted),
		Duplicates:   coerce(r.Duplicates),
		Errored:      coerce(r.Errored),
		Unsorted:     coerce(r.Unsorted),
		Unrecognized: coerce(r.Unrecognized),
		Summary: statusSummary{
			Total:        r.total(),
			Sorted:       len(r.Sorted),
			Duplicates:   len(r.Duplicates),
			Errored:      len(r.Errored),
			Unsorted:     len(r.Unsorted),
			Unrecognized: len(r.Unrecognized),
		},
	}

	if r.Ledger != nil {
		h := r.Ledger.Header
		out.Ledger = &ledgerInfo{
			RunID:       h.RunID,
			PixeVersion: h.PixeVersion,
			Timestamp:   h.PixeRun,
			Recursive:   h.Recursive,
		}
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringP("source", "s", "", "source directory to inspect (default: current directory)")
	statusCmd.Flags().BoolP("recursive", "r", false, "recursively inspect subdirectories of --source")
	statusCmd.Flags().StringArray("ignore", nil, `glob pattern for files to ignore (repeatable, e.g. --ignore "*.txt")`)
	statusCmd.Flags().Bool("json", false, "emit JSON output instead of a human-readable listing")

	_ = viper.BindPFlag("status_source", statusCmd.Flags().Lookup("source"))
	_ = viper.BindPFlag("status_recursive", statusCmd.Flags().Lookup("recursive"))
	_ = viper.BindPFlag("status_ignore", statusCmd.Flags().Lookup("ignore"))
	_ = viper.BindPFlag("status_json", statusCmd.Flags().Lookup("json"))
}
