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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/archivedb"
	"github.com/cwlls/pixe-go/internal/dblocator"
)

// pixeXMPPattern matches Pixe-generated XMP sidecar filenames:
//
//	YYYYMMDD_HHMMSS_<hex_checksum>.<media_ext>.xmp
var pixeXMPPattern = regexp.MustCompile(`^\d{8}_\d{6}_[0-9a-f]+\..+\.xmp$`)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove orphaned temp files and compact the archive database",
	Long: `Clean performs maintenance on a destination archive (dirB).

It has two responsibilities:

  1. Orphaned file cleanup — scans dirB for .pixe-tmp temp files and orphaned
     XMP sidecars left behind by interrupted sort runs and removes them.

  2. Database compaction — runs VACUUM on the archive SQLite database to
     reclaim space from long-lived archives with many runs.

By default both operations are performed. Use --temp-only or --vacuum-only
to run a single operation.`,
	RunE: runClean,
}

// cleanResult tracks the outcome of a clean operation for output formatting.
type cleanResult struct {
	TempFiles        int
	OrphanedSidecars int
	RemoveErrors     int
	SizeBefore       int64
	SizeAfter        int64
	VacuumSkipped    bool
	VacuumReason     string
}

func runClean(cmd *cobra.Command, _ []string) error {
	dir := viper.GetString("clean_dir")
	dbPath := viper.GetString("clean_db_path")
	dryRun := viper.GetBool("clean_dry_run")
	tempOnly := viper.GetBool("clean_temp_only")
	vacuumOnly := viper.GetBool("clean_vacuum_only")

	// Validate mutually exclusive flags.
	if tempOnly && vacuumOnly {
		return fmt.Errorf("--temp-only and --vacuum-only are mutually exclusive")
	}

	// Resolve and validate directory.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve path %q: %w", dir, err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("destination directory %q: %w", absDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", absDir)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Cleaning %s\n", absDir)

	result := &cleanResult{}

	// ------------------------------------------------------------------
	// 1. Orphaned file cleanup (unless --vacuum-only).
	// ------------------------------------------------------------------
	if !vacuumOnly {
		_, _ = fmt.Fprintf(out, "\nOrphaned files:\n")
		if err := cleanOrphanedFiles(out, absDir, dryRun, result); err != nil {
			return fmt.Errorf("orphaned file cleanup: %w", err)
		}
		if result.TempFiles == 0 && result.OrphanedSidecars == 0 {
			_, _ = fmt.Fprintf(out, "  No orphaned files found.\n")
		}
	}

	// ------------------------------------------------------------------
	// 2. Database compaction (unless --temp-only).
	// ------------------------------------------------------------------
	if !tempOnly {
		_, _ = fmt.Fprintf(out, "\nDatabase compaction:\n")
		if err := compactDatabase(out, absDir, dbPath, dryRun, result); err != nil {
			return err
		}
	}

	// ------------------------------------------------------------------
	// 3. Summary line.
	// ------------------------------------------------------------------
	_, _ = fmt.Fprintf(out, "\n%s\n", buildCleanSummary(result, tempOnly, vacuumOnly))

	return nil
}

// cleanOrphanedFiles walks dirB and removes orphaned .pixe-tmp files and
// XMP sidecars whose corresponding media file does not exist.
func cleanOrphanedFiles(out io.Writer, dir string, dryRun bool, result *cleanResult) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			return nil
		}

		name := info.Name()
		relDir, _ := filepath.Rel(dir, filepath.Dir(path))
		if relDir == "." {
			relDir = ""
		}

		// Check for orphaned temp files.
		if isTempFile(name) {
			result.TempFiles++
			printCleanLine(out, name, relDir, "temp file", dryRun)
			if !dryRun {
				if err := os.Remove(path); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "  WARN: could not remove %s: %v\n", path, err)
					result.RemoveErrors++
				}
			}
			return nil
		}

		// Check for orphaned XMP sidecars.
		if isOrphanedSidecar(path, name) {
			result.OrphanedSidecars++
			printCleanLine(out, name, relDir, "orphaned sidecar", dryRun)
			if !dryRun {
				if err := os.Remove(path); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "  WARN: could not remove %s: %v\n", path, err)
					result.RemoveErrors++
				}
			}
			return nil
		}

		return nil
	})
}

// compactDatabase resolves the archive database, checks for active runs,
// and runs VACUUM. Reports size before/after.
func compactDatabase(out io.Writer, dir, dbPath string, dryRun bool, result *cleanResult) error {
	loc, err := dblocator.Resolve(dir, dbPath)
	if err != nil {
		// If resolution fails, the DB likely doesn't exist.
		result.VacuumSkipped = true
		result.VacuumReason = "No archive database found"
		_, _ = fmt.Fprintf(out, "  %s — skipping compaction.\n", result.VacuumReason)
		return nil
	}

	// Check if the DB file actually exists.
	if _, err := os.Stat(loc.DBPath); os.IsNotExist(err) {
		result.VacuumSkipped = true
		result.VacuumReason = "No archive database found"
		_, _ = fmt.Fprintf(out, "  %s — skipping compaction.\n", result.VacuumReason)
		return nil
	}

	// Get size before.
	beforeInfo, err := os.Stat(loc.DBPath)
	if err != nil {
		return fmt.Errorf("stat database: %w", err)
	}
	result.SizeBefore = beforeInfo.Size()

	// Open database in read-write mode.
	db, err := archivedb.Open(loc.DBPath)
	if err != nil {
		return fmt.Errorf("open archive database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Check for active runs.
	active, err := db.HasActiveRuns()
	if err != nil {
		return fmt.Errorf("check active runs: %w", err)
	}
	if active {
		// Retrieve run details for the error message.
		runs, findErr := db.FindInterruptedRuns()
		if findErr != nil || len(runs) == 0 {
			return fmt.Errorf("cannot vacuum — active sort run detected.\nComplete or interrupt the active run before running 'pixe clean'")
		}
		r := runs[0]
		return fmt.Errorf("cannot vacuum — active sort run detected (run %s, started %s).\nComplete or interrupt the active run before running 'pixe clean'",
			truncateID(r.ID), r.StartedAt.Format("2006-01-02 15:04:05 UTC"))
	}

	_, _ = fmt.Fprintf(out, "  Database: %s\n", loc.DBPath)

	if dryRun {
		_, _ = fmt.Fprintf(out, "  Current size: %s\n", formatBytes(result.SizeBefore))
		_, _ = fmt.Fprintf(out, "  (dry-run: VACUUM not executed)\n")
		result.SizeAfter = result.SizeBefore
		return nil
	}

	// Run VACUUM.
	_, _ = fmt.Fprintf(out, "  Size before: %s\n", formatBytes(result.SizeBefore))
	if err := db.Vacuum(); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	// Close and re-stat to get accurate post-VACUUM size.
	_ = db.Close()
	afterInfo, err := os.Stat(loc.DBPath)
	if err != nil {
		return fmt.Errorf("stat database after vacuum: %w", err)
	}
	result.SizeAfter = afterInfo.Size()

	reclaimed := result.SizeBefore - result.SizeAfter
	var pct float64
	if result.SizeBefore > 0 {
		pct = float64(reclaimed) / float64(result.SizeBefore) * 100
	}

	_, _ = fmt.Fprintf(out, "  Size after:  %s\n", formatBytes(result.SizeAfter))
	_, _ = fmt.Fprintf(out, "  Reclaimed:   %s (%.1f%%)\n", formatBytes(reclaimed), pct)

	return nil
}

// isTempFile returns true if the filename contains the .pixe-tmp marker.
func isTempFile(name string) bool {
	return strings.Contains(name, ".pixe-tmp")
}

// isOrphanedSidecar returns true if the file is a Pixe-generated .xmp sidecar
// whose corresponding media file does not exist.
func isOrphanedSidecar(path, name string) bool {
	if !pixeXMPPattern.MatchString(name) {
		return false
	}
	// Strip the trailing ".xmp" to get the expected media file path.
	mediaPath := strings.TrimSuffix(path, ".xmp")
	_, err := os.Stat(mediaPath)
	return os.IsNotExist(err)
}

// printCleanLine prints a single REMOVE / WOULD REMOVE output line.
func printCleanLine(out io.Writer, name, relDir, kind string, dryRun bool) {
	verb := "REMOVE"
	if dryRun {
		verb = "WOULD REMOVE"
	}

	location := ""
	if relDir != "" {
		location = fmt.Sprintf("  (%s/)", relDir)
	}

	suffix := ""
	if kind == "orphaned sidecar" {
		suffix = "  orphaned sidecar"
	}

	_, _ = fmt.Fprintf(out, "  %s %s%s%s\n", verb, name, location, suffix)
}

// truncateID returns the first 8 characters of an ID string.
func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// formatBytes returns a human-readable byte size string using decimal (SI) units.
func formatBytes(b int64) string {
	const (
		kb = 1000
		mb = 1000 * kb
		gb = 1000 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// buildCleanSummary constructs the final summary line.
func buildCleanSummary(r *cleanResult, tempOnly, vacuumOnly bool) string {
	var parts []string

	if !vacuumOnly {
		if r.TempFiles == 0 && r.OrphanedSidecars == 0 {
			parts = append(parts, "No orphaned files found")
		} else {
			fileParts := []string{}
			if r.TempFiles > 0 {
				fileParts = append(fileParts, fmt.Sprintf("%d temp %s", r.TempFiles, pluralize("file", r.TempFiles)))
			}
			if r.OrphanedSidecars > 0 {
				fileParts = append(fileParts, fmt.Sprintf("%d orphaned %s", r.OrphanedSidecars, pluralize("sidecar", r.OrphanedSidecars)))
			}
			parts = append(parts, "Cleaned "+strings.Join(fileParts, ", "))
		}
		if r.RemoveErrors > 0 {
			parts = append(parts, fmt.Sprintf("%d removal %s", r.RemoveErrors, pluralize("error", r.RemoveErrors)))
		}
	}

	if !tempOnly {
		if r.VacuumSkipped {
			parts = append(parts, r.VacuumReason)
		} else {
			reclaimed := r.SizeBefore - r.SizeAfter
			parts = append(parts, fmt.Sprintf("Reclaimed %s from database", formatBytes(reclaimed)))
		}
	}

	return strings.Join(parts, " | ")
}

// pluralize returns the singular form if n==1, otherwise appends "s".
func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().StringP("dir", "d", "", "destination directory (dirB) to clean (required)")
	cleanCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database")
	cleanCmd.Flags().Bool("dry-run", false, "preview what would be cleaned without modifying anything")
	cleanCmd.Flags().Bool("temp-only", false, "only clean orphaned files, skip database compaction")
	cleanCmd.Flags().Bool("vacuum-only", false, "only compact the database, skip file scanning")

	_ = cleanCmd.MarkFlagRequired("dir")

	_ = viper.BindPFlag("clean_dir", cleanCmd.Flags().Lookup("dir"))
	_ = viper.BindPFlag("clean_db_path", cleanCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("clean_dry_run", cleanCmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("clean_temp_only", cleanCmd.Flags().Lookup("temp-only"))
	_ = viper.BindPFlag("clean_vacuum_only", cleanCmd.Flags().Lookup("vacuum-only"))
}
