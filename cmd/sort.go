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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
)

var sortCmd = &cobra.Command{
	Use:   "sort",
	Short: "Sort and rename media files from a source directory into an organized archive",
	Long: `Sort discovers all supported media files in the source directory (--source),
extracts capture dates from metadata, computes data-only checksums, and copies
files into the destination directory (--dest) using the naming convention:

  YYYY/M/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>

The source directory is never modified. Every copy is verified by re-hashing
the destination file. A manifest is written to <dest>/.pixe/manifest.json and
a ledger is written to <source>/.pixe_ledger.json.`,
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
	// HEIC and MP4 handlers registered here once Tasks 12 & 13 are complete.

	// ------------------------------------------------------------------
	// 5. Run the pipeline.
	// ------------------------------------------------------------------
	opts := pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       os.Stdout,
		PixeVersion:  Version(),
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

	// Mark required flags.
	_ = sortCmd.MarkFlagRequired("source")
	_ = sortCmd.MarkFlagRequired("dest")

	// Bind sort-specific flags to Viper.
	_ = viper.BindPFlag("source", sortCmd.Flags().Lookup("source"))
	_ = viper.BindPFlag("dest", sortCmd.Flags().Lookup("dest"))
	_ = viper.BindPFlag("copyright", sortCmd.Flags().Lookup("copyright"))
	_ = viper.BindPFlag("camera_owner", sortCmd.Flags().Lookup("camera-owner"))
	_ = viper.BindPFlag("dry_run", sortCmd.Flags().Lookup("dry-run"))
}
