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
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/archivedb"
	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/dblocator"
	"github.com/cwlls/pixe-go/internal/discovery"
	arwhandler "github.com/cwlls/pixe-go/internal/handler/arw"
	cr2handler "github.com/cwlls/pixe-go/internal/handler/cr2"
	cr3handler "github.com/cwlls/pixe-go/internal/handler/cr3"
	dnghandler "github.com/cwlls/pixe-go/internal/handler/dng"
	heichandler "github.com/cwlls/pixe-go/internal/handler/heic"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	mp4handler "github.com/cwlls/pixe-go/internal/handler/mp4"
	nefhandler "github.com/cwlls/pixe-go/internal/handler/nef"
	pefhandler "github.com/cwlls/pixe-go/internal/handler/pef"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
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
	dir := viper.GetString("resume_dir")
	if dir == "" {
		return fmt.Errorf("--dir is required")
	}
	dbPath := viper.GetString("db_path")

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
	workers := viper.GetInt("workers")
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

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
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	reg.Register(heichandler.New())
	reg.Register(mp4handler.New())
	reg.Register(dnghandler.New())
	reg.Register(nefhandler.New())
	reg.Register(cr2handler.New())
	reg.Register(cr3handler.New())
	reg.Register(pefhandler.New())
	reg.Register(arwhandler.New())

	// ------------------------------------------------------------------
	// 7. Build config and pipeline options, then run.
	// ------------------------------------------------------------------
	cfg := &config.AppConfig{
		Source:      run.Source,
		Destination: dir,
		Workers:     workers,
		Algorithm:   run.Algorithm,
		DBPath:      dbPath,
	}

	// Generate a fresh RunID for this resume attempt. The pipeline will
	// insert a new run record; the interrupted run remains in the DB as
	// historical context.
	runID := uuid.New().String()

	opts := pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       os.Stdout,
		PixeVersion:  Version(),
		DB:           db,
		RunID:        runID,
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

	resumeCmd.Flags().StringP("dir", "d", "", "destination directory containing the archive database (required)")
	resumeCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")

	_ = resumeCmd.MarkFlagRequired("dir")

	_ = viper.BindPFlag("resume_dir", resumeCmd.Flags().Lookup("dir"))
	_ = viper.BindPFlag("db_path", resumeCmd.Flags().Lookup("db-path"))
}
