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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/tui"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the interactive terminal UI",
	Long: `GUI opens an interactive terminal UI with three tabs:

  Sort    — configure and run a sort operation with a live progress view
  Verify  — verify an archive's integrity with a live progress view
  Status  — inspect the sorting status of a source directory

Key bindings:
  Tab / Shift+Tab  — cycle through tabs
  1 / 2 / 3        — jump to Sort / Verify / Status tab
  q / Ctrl+C       — quit`,
	RunE: runGUI,
}

func runGUI(cmd *cobra.Command, args []string) error {
	cfg, err := resolveGUIConfig(cmd)
	if err != nil {
		return err
	}

	reg := buildRegistry()

	h, err := hash.NewHasher(cfg.Algorithm)
	if err != nil {
		return fmt.Errorf("hash algorithm: %w", err)
	}

	opts := tui.AppOptions{
		Config:   cfg,
		Registry: reg,
		Hasher:   h,
		Version:  Version(),
	}

	app := tui.NewApp(opts)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("gui: %w", err)
	}
	return nil
}

// resolveGUIConfig reads flag values directly from the guiCmd cobra.Command,
// bypassing the global Viper store to avoid pflag-binding collisions with
// sortCmd (both commands register flags under the same Viper key names).
func resolveGUIConfig(cmd *cobra.Command) (*config.AppConfig, error) {
	flags := cmd.Flags()

	source, _ := flags.GetString("source")
	dest, _ := flags.GetString("dest")
	workers, _ := flags.GetInt("workers")
	algorithm, _ := flags.GetString("algorithm")
	copyright, _ := flags.GetString("copyright")
	cameraOwner, _ := flags.GetString("camera-owner")
	dryRun, _ := flags.GetBool("dry-run")
	dbPath, _ := flags.GetString("db-path")
	recursive, _ := flags.GetBool("recursive")
	skipDuplicates, _ := flags.GetBool("skip-duplicates")
	ignore, _ := flags.GetStringArray("ignore")
	noCarrySidecars, _ := flags.GetBool("no-carry-sidecars")
	overwriteSidecarTags, _ := flags.GetBool("overwrite-sidecar-tags")

	// Inherit persistent flags (workers, algorithm) from root if not overridden.
	if !flags.Changed("workers") {
		if pf := cmd.Root().PersistentFlags().Lookup("workers"); pf != nil {
			workers, _ = cmd.Root().PersistentFlags().GetInt("workers")
		}
	}
	if !flags.Changed("algorithm") {
		if pf := cmd.Root().PersistentFlags().Lookup("algorithm"); pf != nil {
			algorithm, _ = cmd.Root().PersistentFlags().GetString("algorithm")
		}
	}

	if source == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve current directory: %w", err)
		}
		source = cwd
	}

	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	if algorithm == "" {
		algorithm = "sha1"
	}

	return &config.AppConfig{
		Source:               source,
		Destination:          dest,
		Workers:              workers,
		Algorithm:            algorithm,
		Copyright:            copyright,
		CameraOwner:          cameraOwner,
		DryRun:               dryRun,
		DBPath:               dbPath,
		Recursive:            recursive,
		SkipDuplicates:       skipDuplicates,
		Ignore:               ignore,
		CarrySidecars:        !noCarrySidecars,
		OverwriteSidecarTags: overwriteSidecarTags,
	}, nil
}

func init() {
	rootCmd.AddCommand(guiCmd)

	// Register the same flags as sortCmd so the GUI can be pre-configured.
	guiCmd.Flags().StringP("source", "s", "", "source directory containing media files (default: current directory)")
	guiCmd.Flags().StringP("dest", "d", "", "destination directory for the organized archive")
	guiCmd.Flags().String("copyright", "", `copyright template injected into destination files, e.g. "Copyright {{.Year}} My Family"`)
	guiCmd.Flags().String("camera-owner", "", "camera owner string injected into destination files")
	guiCmd.Flags().Bool("dry-run", false, "preview operations without copying any files")
	guiCmd.Flags().String("db-path", "", "explicit path to the SQLite archive database (overrides auto-resolution)")
	guiCmd.Flags().BoolP("recursive", "r", false, "recursively process subdirectories of --source")
	guiCmd.Flags().Bool("skip-duplicates", false, "skip copying duplicate files instead of copying to duplicates/ directory")
	guiCmd.Flags().StringArray("ignore", nil, `glob pattern for files to ignore (repeatable)`)
	guiCmd.Flags().Bool("no-carry-sidecars", false, "disable carrying pre-existing .aae and .xmp sidecar files")
	guiCmd.Flags().Bool("overwrite-sidecar-tags", false, "overwrite existing sidecar tag values instead of preserving them")

}
