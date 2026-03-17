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
	"github.com/cwlls/pixe/internal/discovery"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/pipeline"
)

// retryCmd is the "pixe retry" subcommand.
var retryCmd = &cobra.Command{
	Use:   "retry",
	Short: "Retry errored files from a previous sort run",
	Long: `Re-process only the files that failed, had integrity mismatches, or had
tag failures in a specific run. A new run is created for auditability —
the original error records are preserved unchanged.

By default, retries errors from the most recent run whose source directory
matches --source (or the current directory). Use --run to target a specific run.`,
	RunE: runRetry,
}

// runRetry is the RunE handler for the retry subcommand.
func runRetry(cmd *cobra.Command, _ []string) error {
	dest, err := resolveDest("retry_dest")
	if err != nil {
		return err
	}
	dbPath := resolveDBPath("retry_db_path")

	// Validate destination directory.
	if info, err := os.Stat(dest); err != nil {
		return fmt.Errorf("destination directory %q: %w", dest, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dest)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// ------------------------------------------------------------------
	// 1. Open the archive database (read-write — retry creates a new run).
	// ------------------------------------------------------------------
	loc, err := dblocator.Resolve(dest, dbPath)
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
	// 2. Resolve target run.
	// ------------------------------------------------------------------
	runFilter, _ := cmd.Flags().GetString("run")
	sourceFlag, _ := cmd.Flags().GetString("source")

	var targetRun *archivedb.Run
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
		targetRun = runs[0]
	} else {
		sourceDir := sourceFlag
		if sourceDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve current directory: %w", err)
			}
			sourceDir = cwd
		}
		absSource, err := filepath.Abs(sourceDir)
		if err != nil {
			return fmt.Errorf("resolve source path: %w", err)
		}
		targetRun, err = db.MostRecentRunBySource(absSource)
		if err != nil {
			return fmt.Errorf("find most recent run: %w", err)
		}
		if targetRun == nil {
			targetRun, err = db.MostRecentRun()
			if err != nil {
				return fmt.Errorf("find most recent run: %w", err)
			}
		}
	}
	if targetRun == nil {
		_, _ = fmt.Fprintln(os.Stdout, "No completed runs found.")
		return nil
	}

	// ------------------------------------------------------------------
	// 3. Query errored files from the target run.
	// ------------------------------------------------------------------
	errorFiles, err := db.FilesWithErrorsByRun(targetRun.ID)
	if err != nil {
		return fmt.Errorf("query errored files: %w", err)
	}
	if len(errorFiles) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No files to retry.")
		return nil
	}

	// ------------------------------------------------------------------
	// 4. Build handler registry and validate source files.
	// ------------------------------------------------------------------
	reg := buildRegistry()

	var retryFiles []discovery.DiscoveredFile
	missing := 0
	for _, f := range errorFiles {
		if _, statErr := os.Stat(f.SourcePath); os.IsNotExist(statErr) {
			_, _ = fmt.Fprintf(os.Stdout, "SKIP %s → source file no longer exists\n",
				filepath.Base(f.SourcePath))
			missing++
			continue
		}
		handler, detectErr := reg.Detect(f.SourcePath)
		if detectErr != nil || handler == nil {
			_, _ = fmt.Fprintf(os.Stdout, "SKIP %s → no handler found\n",
				filepath.Base(f.SourcePath))
			missing++
			continue
		}
		relPath, _ := filepath.Rel(targetRun.Source, f.SourcePath)
		if relPath == "" {
			relPath = filepath.Base(f.SourcePath)
		}
		retryFiles = append(retryFiles, discovery.DiscoveredFile{
			Path:    f.SourcePath,
			RelPath: relPath,
			Handler: handler,
			// Sidecars are not re-discovered in retry mode (non-fatal).
		})
	}

	if len(retryFiles) == 0 {
		_, _ = fmt.Fprintf(os.Stdout, "No files to retry (%d source file(s) no longer accessible).\n", missing)
		return nil
	}

	// ------------------------------------------------------------------
	// 5. Dry-run: list files and exit.
	// ------------------------------------------------------------------
	if dryRun {
		_, _ = fmt.Fprintf(os.Stdout, "Would retry %d file(s) from run %s:\n", len(retryFiles), truncID(targetRun.ID))
		for _, df := range retryFiles {
			_, _ = fmt.Fprintf(os.Stdout, "  %s\n", df.RelPath)
		}
		if missing > 0 {
			_, _ = fmt.Fprintf(os.Stdout, "  (%d file(s) skipped — source no longer accessible)\n", missing)
		}
		return nil
	}

	// ------------------------------------------------------------------
	// 6. Build config and pipeline options.
	// ------------------------------------------------------------------
	h, err := hash.NewHasher(targetRun.Algorithm)
	if err != nil {
		return fmt.Errorf("hash algorithm from run %q: %w", targetRun.Algorithm, err)
	}

	pathTemplateStr := viper.GetString("path_template")
	if pathTemplateStr == "" {
		pathTemplateStr = pathbuilder.DefaultTemplate
	}
	tmpl, err := pathbuilder.ParseTemplate(pathTemplateStr)
	if err != nil {
		return err
	}

	quiet := viper.GetBool("quiet")
	verbose := viper.GetBool("verbose")
	verbosity := 0
	if quiet {
		verbosity = -1
	} else if verbose {
		verbosity = 1
	}

	cfg := &config.AppConfig{
		Source:               targetRun.Source,
		Destination:          dest,
		Workers:              resolveWorkers("retry_workers"),
		Algorithm:            targetRun.Algorithm,
		DBPath:               dbPath,
		PathTemplate:         pathTemplateStr,
		CarrySidecars:        !viper.GetBool("no_carry_sidecars"),
		Copyright:            viper.GetString("copyright"),
		CameraOwner:          viper.GetString("camera_owner"),
		SkipDuplicates:       viper.GetBool("skip_duplicates"),
		Recursive:            targetRun.Recursive,
		OverwriteSidecarTags: viper.GetBool("overwrite_sidecar_tags"),
		Ignore:               viper.GetStringSlice("ignore"),
		Verbosity:            verbosity,
	}

	if cfg.Copyright != "" {
		ct, err := pathbuilder.ParseCopyrightTemplate(cfg.Copyright)
		if err != nil {
			return err
		}
		cfg.CopyrightTemplate = ct
	}

	runID := uuid.New().String()

	ctx, stopSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	_, _ = fmt.Fprintf(os.Stdout, "Retrying %d file(s) from run %s (source: %s)\n",
		len(retryFiles), truncID(targetRun.ID), targetRun.Source)

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
		DestLabel:    "..." + filepath.Base(dest),
		RetryFiles:   retryFiles,
		NoLedger:     true, // retry runs don't write a new ledger to the source dir
	}

	result, err := pipeline.Run(opts)
	if err != nil {
		return fmt.Errorf("retry failed: %w", err)
	}

	if result.Errors > 0 {
		return fmt.Errorf("%d file(s) failed to process — check output above", result.Errors)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(retryCmd)

	retryCmd.Flags().StringP("dest", "d", "", "archive directory (required)")
	retryCmd.Flags().StringP("source", "s", "", "source directory (for scoping to most recent run; default: current directory)")
	retryCmd.Flags().String("run", "", "specific run ID (prefix match)")
	retryCmd.Flags().Bool("dry-run", false, "preview what would be retried without processing")
	retryCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")
	retryCmd.Flags().IntP("workers", "w", 0, "number of concurrent workers (0 = auto: runtime.NumCPU())")

	_ = viper.BindPFlag("retry_dest", retryCmd.Flags().Lookup("dest"))
	_ = viper.BindPFlag("retry_db_path", retryCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("retry_workers", retryCmd.Flags().Lookup("workers"))
}
