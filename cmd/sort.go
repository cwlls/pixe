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
	"errors"
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
	"github.com/cwlls/pixe-go/internal/migrate"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
)

var sortCmd = &cobra.Command{
	Use:   "sort",
	Short: "Sort and rename media files from a source directory into an organized archive",
	Long: `Sort discovers all supported media files in the source directory (--source),
extracts capture dates from metadata, computes data-only checksums, and copies
files into the destination directory (--dest) using the naming convention:

  YYYY/MM-Mon/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>

The source directory is never modified. Every copy is verified by re-hashing
the destination file. An archive database is written to <dest>/.pixe/pixe.db
and a ledger is written to <source>/.pixe_ledger.json.`,
	RunE: runSort,
}

func runSort(cmd *cobra.Command, args []string) error {
	// ------------------------------------------------------------------
	// 1. Resolve configuration from Viper (flags > config file > defaults).
	// ------------------------------------------------------------------
	cfg := &config.AppConfig{
		Source:      viper.GetString("source"),
		Destination: viper.GetString("dest"),
		Workers:     viper.GetInt("workers"),
		Algorithm:   viper.GetString("algorithm"),
		Copyright:   viper.GetString("copyright"),
		CameraOwner: viper.GetString("camera_owner"),
		DryRun:      viper.GetBool("dry_run"),
		DBPath:      viper.GetString("db_path"),
	}

	// ------------------------------------------------------------------
	// 2. Validate inputs.
	// ------------------------------------------------------------------
	if cfg.Source == "" {
		return errors.New("--source is required")
	}
	if cfg.Destination == "" {
		return errors.New("--dest is required")
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

	// Default workers to NumCPU when unset (0).
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
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
	// 5. Resolve and open the archive database.
	// ------------------------------------------------------------------
	loc, err := dblocator.Resolve(cfg.Destination, cfg.DBPath)
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

	// Write dbpath marker if needed (explicit path or network mount).
	if loc.MarkerNeeded {
		if err := dblocator.WriteMarker(cfg.Destination, loc.DBPath); err != nil {
			return fmt.Errorf("write dbpath marker: %w", err)
		}
	}

	// ------------------------------------------------------------------
	// 6. Auto-migrate from legacy JSON manifest if present.
	// ------------------------------------------------------------------
	migResult, err := migrate.MigrateIfNeeded(db, cfg.Destination)
	if err != nil {
		return fmt.Errorf("migrate manifest: %w", err)
	}
	if migResult.Migrated {
		_, _ = fmt.Fprintln(os.Stdout, migResult.Notice)
	}

	// ------------------------------------------------------------------
	// 7. Run the pipeline.
	// ------------------------------------------------------------------
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

	result, err := pipeline.Run(opts)
	if err != nil {
		return fmt.Errorf("sort failed: %w", err)
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
	sortCmd.Flags().StringP("source", "s", "", "source directory containing media files to sort (required)")
	sortCmd.Flags().StringP("dest", "d", "", "destination directory for the organized archive (required)")
	sortCmd.Flags().String("copyright", "", `copyright template injected into destination files, e.g. "Copyright {{.Year}} My Family"`)
	sortCmd.Flags().String("camera-owner", "", "camera owner string injected into destination files")
	sortCmd.Flags().Bool("dry-run", false, "preview operations without copying any files")
	sortCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")

	// Mark required flags.
	_ = sortCmd.MarkFlagRequired("source")
	_ = sortCmd.MarkFlagRequired("dest")

	// Bind sort-specific flags to Viper.
	_ = viper.BindPFlag("source", sortCmd.Flags().Lookup("source"))
	_ = viper.BindPFlag("dest", sortCmd.Flags().Lookup("dest"))
	_ = viper.BindPFlag("copyright", sortCmd.Flags().Lookup("copyright"))
	_ = viper.BindPFlag("camera_owner", sortCmd.Flags().Lookup("camera-owner"))
	_ = viper.BindPFlag("dry_run", sortCmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("db_path", sortCmd.Flags().Lookup("db-path"))
}
