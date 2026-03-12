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
	"path/filepath"
	"runtime"
	"time"

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
	orfhandler "github.com/cwlls/pixe-go/internal/handler/orf"
	pefhandler "github.com/cwlls/pixe-go/internal/handler/pef"
	pnghandler "github.com/cwlls/pixe-go/internal/handler/png"
	rw2handler "github.com/cwlls/pixe-go/internal/handler/rw2"
	"github.com/cwlls/pixe-go/internal/migrate"
)

// resolveConfig reads Viper values and returns a populated *config.AppConfig.
// Source defaults to the current working directory when not set.
// Workers defaults to runtime.NumCPU() when <= 0.
// Used by runSort and runGUI.
func resolveConfig() (*config.AppConfig, error) {
	cfg := &config.AppConfig{
		Source:               viper.GetString("source"),
		Destination:          viper.GetString("dest"),
		Workers:              viper.GetInt("workers"),
		Algorithm:            viper.GetString("algorithm"),
		Copyright:            viper.GetString("copyright"),
		CameraOwner:          viper.GetString("camera_owner"),
		DryRun:               viper.GetBool("dry_run"),
		DBPath:               viper.GetString("db_path"),
		Recursive:            viper.GetBool("recursive"),
		SkipDuplicates:       viper.GetBool("skip_duplicates"),
		Ignore:               viper.GetStringSlice("ignore"),
		CarrySidecars:        !viper.GetBool("no_carry_sidecars"),
		OverwriteSidecarTags: viper.GetBool("overwrite_sidecar_tags"),
	}

	if s := viper.GetString("since"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil, fmt.Errorf("invalid --since date %q: expected YYYY-MM-DD: %w", s, err)
		}
		cfg.Since = &t
	}
	if s := viper.GetString("before"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return nil, fmt.Errorf("invalid --before date %q: expected YYYY-MM-DD: %w", s, err)
		}
		// End of day — inclusive.
		eod := t.Add(24*time.Hour - time.Nanosecond)
		cfg.Before = &eod
	}

	quiet := viper.GetBool("quiet")
	verbose := viper.GetBool("verbose")
	if quiet && verbose {
		return nil, fmt.Errorf("--quiet and --verbose are mutually exclusive")
	}
	if quiet {
		cfg.Verbosity = -1
	} else if verbose {
		cfg.Verbosity = 1
	}

	if cfg.Source == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve current directory: %w", err)
		}
		cfg.Source = cwd
	}

	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}

	return cfg, nil
}

// buildRegistry creates and populates a discovery.Registry with all
// supported file type handlers. Used by sort, verify, status, and gui.
func buildRegistry() *discovery.Registry {
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	reg.Register(heichandler.New())
	reg.Register(mp4handler.New())
	reg.Register(pnghandler.New())
	reg.Register(dnghandler.New())
	reg.Register(nefhandler.New())
	reg.Register(cr2handler.New())
	reg.Register(cr3handler.New())
	reg.Register(pefhandler.New())
	reg.Register(arwhandler.New())
	reg.Register(orfhandler.New())
	reg.Register(rw2handler.New())
	return reg
}

// openArchiveDB resolves the database location for the given destination
// directory, opens the SQLite archive database, runs any pending migrations,
// and returns the DB handle plus a cleanup function (which closes the DB).
//
// The caller must invoke the cleanup function (typically via defer) to close
// the database. If an error is returned, the cleanup function is a no-op.
//
// Used by runSort and the TUI Sort tab.
func openArchiveDB(cfg *config.AppConfig) (*archivedb.DB, func(), error) {
	loc, err := dblocator.Resolve(cfg.Destination, cfg.DBPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("resolve database location: %w", err)
	}
	if loc.Notice != "" {
		fmt.Fprintln(os.Stderr, loc.Notice)
	}

	db, err := archivedb.Open(loc.DBPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open archive database: %w", err)
	}

	cleanup := func() { _ = db.Close() }

	// Write dbpath marker if needed (explicit path or network mount).
	if loc.MarkerNeeded {
		if err := dblocator.WriteMarker(cfg.Destination, loc.DBPath); err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("write dbpath marker: %w", err)
		}
	}

	// Auto-migrate from legacy JSON manifest if present.
	migResult, err := migrate.MigrateIfNeeded(db, cfg.Destination)
	if err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("migrate manifest: %w", err)
	}
	if migResult.Migrated {
		_, _ = fmt.Fprintln(os.Stdout, migResult.Notice)
	}

	return db, cleanup, nil
}

// mergeSourceConfig merges values from a secondary Viper instance into the
// global Viper store. Only keys not explicitly set via CLI flags are merged.
// This preserves the priority chain: CLI flags > source config > global config.
//
// The ignore key is special: patterns from the source config are appended to
// the existing ignore list (additive merge) rather than replacing it.
func mergeSourceConfig(local *viper.Viper, cmd *cobra.Command) {
	keys := []struct {
		viperKey string
		flagName string
	}{
		{"dest", "dest"},
		{"copyright", "copyright"},
		{"camera_owner", "camera-owner"},
		{"algorithm", "algorithm"},
		{"recursive", "recursive"},
		{"skip_duplicates", "skip-duplicates"},
		{"no_carry_sidecars", "no-carry-sidecars"},
		{"overwrite_sidecar_tags", "overwrite-sidecar-tags"},
	}

	for _, k := range keys {
		// Only merge if the CLI flag was not explicitly provided.
		if cmd.Flags().Changed(k.flagName) {
			continue
		}
		if local.IsSet(k.viperKey) {
			viper.Set(k.viperKey, local.Get(k.viperKey))
		}
	}

	// Ignore patterns are merged additively.
	if local.IsSet("ignore") {
		existing := viper.GetStringSlice("ignore")
		additional := local.GetStringSlice("ignore")
		viper.Set("ignore", append(existing, additional...))
	}
}

// loadProfile loads a named config profile and merges it into the global
// Viper store. Profiles are YAML files stored in ~/.pixe/profiles/<name>.yaml
// or $XDG_CONFIG_HOME/pixe/profiles/<name>.yaml.
//
// Priority chain after loading: CLI flags > source config > profile > global config.
// Only keys not already set via CLI flags are merged (same rule as mergeSourceConfig).
func loadProfile(name string, cmd *cobra.Command) error {
	var profilePaths []string
	if home, err := os.UserHomeDir(); err == nil {
		profilePaths = append(profilePaths, filepath.Join(home, ".pixe", "profiles", name+".yaml"))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		profilePaths = append(profilePaths, filepath.Join(xdg, "pixe", "profiles", name+".yaml"))
	}

	for _, p := range profilePaths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		profileViper := viper.New()
		profileViper.SetConfigFile(p)
		if err := profileViper.ReadInConfig(); err != nil {
			return fmt.Errorf("load profile %q: %w", name, err)
		}
		fmt.Fprintln(os.Stderr, "Using profile:", p)
		mergeSourceConfig(profileViper, cmd)
		return nil
	}
	return fmt.Errorf("profile %q not found (searched: %v)", name, profilePaths)
}
