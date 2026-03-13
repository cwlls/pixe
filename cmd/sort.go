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
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe/internal/cli"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/pipeline"
	"github.com/cwlls/pixe/internal/progress"
)

// sortCmd is the "pixe sort" subcommand.
var sortCmd = &cobra.Command{
	Use:   "sort",
	Short: "Sort and rename media files from a source directory into an organized archive",
	Long: `Sort discovers all supported media files in the source directory. When --source
is omitted, the current working directory is used. Extracts capture dates from
metadata, computes data-only checksums, and copies files into the destination
directory (--dest) using the naming convention:

  YYYY/MM-Mon/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>

The source directory is never modified. Every copy is verified by re-hashing
the destination file. An archive database is written to <dest>/.pixe/pixe.db
and a ledger is written to <source>/.pixe_ledger.json.`,
	RunE: runSort,
}

// runSort is the RunE handler for the sort subcommand.
func runSort(cmd *cobra.Command, args []string) error {
	// ------------------------------------------------------------------
	// 1. Resolve configuration from Viper (flags > config file > defaults).
	// ------------------------------------------------------------------
	cfg, err := resolveConfig()
	if err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// 1b. Check for source-local config (.pixe.yaml in dirA).
	// ------------------------------------------------------------------
	sourceConfig := filepath.Join(cfg.Source, ".pixe.yaml")
	if _, statErr := os.Stat(sourceConfig); statErr == nil {
		localViper := viper.New()
		localViper.SetConfigFile(sourceConfig)
		if readErr := localViper.ReadInConfig(); readErr == nil {
			fmt.Fprintln(os.Stderr, "Using source config:", sourceConfig)
			mergeSourceConfig(localViper, cmd)
			// Re-resolve to pick up merged values.
			cfg, err = resolveConfig()
			if err != nil {
				return err
			}
		}
	}

	// ------------------------------------------------------------------
	// 1c. Load named profile (--profile), if specified.
	// Profile priority: CLI flags > source config > profile > global config.
	// ------------------------------------------------------------------
	if profileName := viper.GetString("profile"); profileName != "" {
		if err := loadProfile(profileName, cmd); err != nil {
			return err
		}
		// Re-resolve to pick up profile values.
		cfg, err = resolveConfig()
		if err != nil {
			return err
		}
	}

	// ------------------------------------------------------------------
	// 1d. Resolve destination alias (@name → filesystem path).
	// ------------------------------------------------------------------
	resolvedDest, err := resolveAlias(cfg.Destination, cfg.Aliases)
	if err != nil {
		return err
	}
	cfg.Destination = resolvedDest

	// ------------------------------------------------------------------
	// 1e. Parse and validate the path template.
	// ------------------------------------------------------------------
	tmpl, err := pathbuilder.ParseTemplate(cfg.PathTemplate)
	if err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// 2. Validate inputs.
	// ------------------------------------------------------------------
	if cfg.Destination == "" {
		return fmt.Errorf("--dest is required")
	}

	// Source must exist and be a directory.
	srcInfo, err := os.Stat(cfg.Source)
	if err != nil {
		return fmt.Errorf("source directory %q: %w", cfg.Source, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %q is not a directory", cfg.Source)
	}

	// Destination is created if absent.
	if err := os.MkdirAll(cfg.Destination, 0o755); err != nil {
		return fmt.Errorf("create destination directory %q: %w", cfg.Destination, err)
	}

	// ------------------------------------------------------------------
	// 3. Build the hasher.
	// ------------------------------------------------------------------
	h, err := hash.NewHasher(cfg.Algorithm)
	if err != nil {
		return fmt.Errorf("hash algorithm: %w", err)
	}

	// ------------------------------------------------------------------
	// 4. Build the handler registry.
	// ------------------------------------------------------------------
	reg := buildRegistry()

	// ------------------------------------------------------------------
	// 5. Resolve and open the archive database.
	// ------------------------------------------------------------------
	db, cleanup, err := openArchiveDB(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	// ------------------------------------------------------------------
	// 6. Run the pipeline.
	// ------------------------------------------------------------------
	runID := uuid.New().String()

	// Detect TTY and color support.
	isTTY := isatty.IsTerminal(os.Stdout.Fd())
	_, noColor := os.LookupEnv("NO_COLOR")
	useProgress := viper.GetBool("progress") && isTTY

	// Wire SIGINT/SIGTERM to a cancellable context so the pipeline can drain
	// gracefully. signal.NotifyContext restores default signal behaviour on the
	// second signal, allowing a hard exit if the user insists.
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
		ColorOutput:  isTTY && !noColor && cfg.Verbosity >= 0,
		Context:      ctx,
	}

	var result pipeline.SortResult
	if useProgress {
		bus := progress.NewBus(256)
		opts.EventBus = bus
		opts.Output = io.Discard

		model := cli.NewProgressModel(bus, cfg.Source, cfg.Destination, "sort")
		p := tea.NewProgram(model)

		// Run pipeline in background; close bus when done.
		var pipelineErr error
		done := make(chan struct{})
		go func() {
			defer close(done)
			result, pipelineErr = pipeline.Run(opts)
			bus.Close()
		}()

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("progress UI: %w", err)
		}
		<-done
		if pipelineErr != nil {
			return fmt.Errorf("sort failed: %w", pipelineErr)
		}
	} else {
		var err error
		result, err = pipeline.Run(opts)
		if err != nil {
			return fmt.Errorf("sort failed: %w", err)
		}
	}

	// Non-zero errors → exit code 1 (Cobra propagates the returned error).
	if result.Errors > 0 {
		return fmt.Errorf("%d file(s) failed to process — check output above", result.Errors)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(sortCmd)

	// Sort-specific flags.
	sortCmd.Flags().StringP("source", "s", "", "source directory containing media files to sort (default: current directory)")
	sortCmd.Flags().StringP("dest", "d", "", "destination directory for the organized archive (required)")
	sortCmd.Flags().String("copyright", "", `copyright template injected into destination files, e.g. "Copyright {{.Year}} My Family"`)
	sortCmd.Flags().String("camera-owner", "", "camera owner string injected into destination files")
	sortCmd.Flags().Bool("dry-run", false, "preview operations without copying any files")
	sortCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")
	sortCmd.Flags().BoolP("recursive", "r", false, "recursively process subdirectories of --source")
	sortCmd.Flags().Bool("skip-duplicates", false, "skip copying duplicate files instead of copying to duplicates/ directory")
	sortCmd.Flags().StringArray("ignore", nil, `glob pattern for files to ignore (repeatable, e.g. --ignore "*.txt" --ignore ".DS_Store")`)
	sortCmd.Flags().Bool("no-carry-sidecars", false, "disable carrying pre-existing .aae and .xmp sidecar files from source to destination")
	sortCmd.Flags().Bool("overwrite-sidecar-tags", false, "when merging tags into a carried .xmp sidecar, overwrite existing values instead of preserving them")
	sortCmd.Flags().Bool("progress", false, "show a live progress bar instead of per-file text output (requires a TTY)")
	sortCmd.Flags().String("since", "", `only process files with capture date on or after this date (format: YYYY-MM-DD)`)
	sortCmd.Flags().String("before", "", `only process files with capture date on or before this date (format: YYYY-MM-DD)`)
	sortCmd.Flags().String("path-template", "", `token-based template for destination directory structure (default: "{year}/{month}-{monthname}")`)

	// Mark required flags (--source defaults to cwd; --dest has no default).
	_ = sortCmd.MarkFlagRequired("dest")

	// Bind sort-specific flags to Viper.
	_ = viper.BindPFlag("source", sortCmd.Flags().Lookup("source"))
	_ = viper.BindPFlag("dest", sortCmd.Flags().Lookup("dest"))
	_ = viper.BindPFlag("copyright", sortCmd.Flags().Lookup("copyright"))
	_ = viper.BindPFlag("camera_owner", sortCmd.Flags().Lookup("camera-owner"))
	_ = viper.BindPFlag("dry_run", sortCmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("db_path", sortCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("recursive", sortCmd.Flags().Lookup("recursive"))
	_ = viper.BindPFlag("skip_duplicates", sortCmd.Flags().Lookup("skip-duplicates"))
	_ = viper.BindPFlag("ignore", sortCmd.Flags().Lookup("ignore"))
	_ = viper.BindPFlag("no_carry_sidecars", sortCmd.Flags().Lookup("no-carry-sidecars"))
	_ = viper.BindPFlag("overwrite_sidecar_tags", sortCmd.Flags().Lookup("overwrite-sidecar-tags"))
	_ = viper.BindPFlag("progress", sortCmd.Flags().Lookup("progress"))
	_ = viper.BindPFlag("since", sortCmd.Flags().Lookup("since"))
	_ = viper.BindPFlag("before", sortCmd.Flags().Lookup("before"))
	_ = viper.BindPFlag("path_template", sortCmd.Flags().Lookup("path-template"))
}
