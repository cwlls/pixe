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
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe/internal/archivedb"
	"github.com/cwlls/pixe/internal/config"
	"github.com/cwlls/pixe/internal/dblocator"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/pipeline"
)

// resumeCmd is the "pixe resume" subcommand.
var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume an interrupted sort operation using the archive database",
	Long: `Resume discovers the archive database for the given destination directory
and continues processing any files that did not reach the 'complete' state.

The database location is resolved via the same priority chain as 'pixe sort':
  1. --db-path flag (explicit override)
  2. <dir>/.pixe/dbpath marker file
  3. <dir>/.pixe/pixe.db (local filesystem default)

Files already marked 'complete' are skipped. Files in earlier states
re-enter the pipeline from the beginning.`,
	RunE: runResume,
}

// runResume is the RunE handler for the resume subcommand.
func runResume(cmd *cobra.Command, args []string) error {
	dir, err := resolveDest("resume_dest")
	if err != nil {
		return err
	}
	dbPath := resolveDBPath("resume_db_path")

	// Validate destination directory exists.
	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf("destination directory %q: %w", dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	// ------------------------------------------------------------------
	// 1. Resolve and open the archive database.
	// ------------------------------------------------------------------
	loc, err := dblocator.Resolve(dir, dbPath)
	if err != nil {
		return fmt.Errorf("resolve database location: %w", err)
	}
	if loc.Notice != "" {
		fmt.Fprintln(os.Stderr, loc.Notice)
	}

	db, err := archivedb.Open(loc.DBPath)
	if err != nil {
		return fmt.Errorf("open archive database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// ------------------------------------------------------------------
	// 2. Find interrupted runs (status = "running").
	// ------------------------------------------------------------------
	interrupted, err := db.FindInterruptedRuns()
	if err != nil {
		return fmt.Errorf("find interrupted runs: %w", err)
	}
	if len(interrupted) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No interrupted runs found.")
		return nil
	}

	// Resume the most recent interrupted run (FindInterruptedRuns returns
	// runs ordered by started_at DESC, so index 0 is the most recent).
	run := interrupted[0]

	// ------------------------------------------------------------------
	// 3. Validate the source directory still exists.
	// ------------------------------------------------------------------
	if info, err := os.Stat(run.Source); err != nil {
		return fmt.Errorf("source directory from interrupted run %q: %w", run.Source, err)
	} else if !info.IsDir() {
		return fmt.Errorf("source %q from interrupted run is not a directory", run.Source)
	}

	// ------------------------------------------------------------------
	// 4. Resolve workers.
	// ------------------------------------------------------------------
	workers := resolveWorkers("resume_workers")

	// ------------------------------------------------------------------
	// 5. Build hasher using the algorithm recorded in the interrupted run.
	// ------------------------------------------------------------------
	h, err := hash.NewHasher(run.Algorithm)
	if err != nil {
		return fmt.Errorf("hash algorithm from interrupted run %q: %w", run.Algorithm, err)
	}

	// ------------------------------------------------------------------
	// 6. Build handler registry.
	// ------------------------------------------------------------------
	reg := buildRegistry()

	// ------------------------------------------------------------------
	// 7. Build config and pipeline options, then run.
	// ------------------------------------------------------------------
	// Parse the path template (use default if not configured).
	pathTemplateStr := viper.GetString("path_template")
	if pathTemplateStr == "" {
		pathTemplateStr = pathbuilder.DefaultTemplate
	}
	tmpl, err := pathbuilder.ParseTemplate(pathTemplateStr)
	if err != nil {
		return err
	}

	// Populate all config fields from Viper so the resumed sort behaves
	// consistently with the original — carry sidecars, copyright, ignore
	// patterns, etc. are read from the current config (CLI flags, .pixe.yaml,
	// env) since they are not persisted in the DB's runs table.
	quiet := viper.GetBool("quiet")
	verbose := viper.GetBool("verbose")
	verbosity := 0
	if quiet {
		verbosity = -1
	} else if verbose {
		verbosity = 1
	}

	cfg := &config.AppConfig{
		Source:               run.Source,
		Destination:          dir,
		Workers:              workers,
		Algorithm:            run.Algorithm,
		DBPath:               dbPath,
		PathTemplate:         pathTemplateStr,
		CarrySidecars:        !viper.GetBool("no_carry_sidecars"),
		Copyright:            viper.GetString("copyright"),
		CameraOwner:          viper.GetString("camera_owner"),
		SkipDuplicates:       viper.GetBool("skip_duplicates"),
		Recursive:            run.Recursive,
		OverwriteSidecarTags: viper.GetBool("overwrite_sidecar_tags"),
		Ignore:               viper.GetStringSlice("ignore"),
		Verbosity:            verbosity,
	}

	// Parse copyright template if configured.
	if cfg.Copyright != "" {
		ct, err := pathbuilder.ParseCopyrightTemplate(cfg.Copyright)
		if err != nil {
			return err
		}
		cfg.CopyrightTemplate = ct
	}

	// Generate a fresh RunID for this resume attempt. The pipeline will
	// insert a new run record; the interrupted run remains in the DB as
	// historical context.
	runID := uuid.New().String()

	// Wire SIGINT/SIGTERM to a cancellable context so the pipeline can drain
	// gracefully on interruption.
	ctx, stopSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	opts := pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		PathTemplate: tmpl,
		Output:       os.Stdout,
		PixeVersion:  Version(),
		DB:           db,
		RunID:        runID,
		Context:      ctx,
		DestLabel:    "..." + filepath.Base(dir),
	}

	_, _ = fmt.Fprintf(os.Stdout, "Resuming sort: source=%s dest=%s\n", run.Source, dir)

	result, err := pipeline.Run(opts)
	if err != nil {
		return fmt.Errorf("resume failed: %w", err)
	}

	if result.Errors > 0 {
		return fmt.Errorf("%d file(s) failed to process — check output above", result.Errors)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(resumeCmd)

	resumeCmd.Flags().StringP("dest", "d", "", "destination directory containing the archive database (required)")
	resumeCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")
	resumeCmd.Flags().IntP("workers", "w", 0, "number of concurrent workers (0 = auto: runtime.NumCPU())")

	_ = viper.BindPFlag("resume_dest", resumeCmd.Flags().Lookup("dest"))
	_ = viper.BindPFlag("resume_db_path", resumeCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("resume_workers", resumeCmd.Flags().Lookup("workers"))
}
