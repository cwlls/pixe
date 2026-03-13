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

// Package cmd provides the Cobra CLI commands for Pixe.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd is the base command. All subcommands are registered against it.
var rootCmd = &cobra.Command{
	Use:   "pixe",
	Short: "Pixe — a safe, deterministic photo and video sorting utility",
	Long: `Pixe organizes irreplaceable media files into a date-based directory
structure with embedded integrity checksums.

Source files are never modified. Every copy is verified before being
considered complete. Interrupted runs can always be resumed.

Documentation: https://github.com/cwlls/pixe-go`,
}

// Execute is the entry point called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Config file flag — local to root only.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)")

	// Persistent flags inherited by all subcommands.
	rootCmd.PersistentFlags().IntP("workers", "w", 0,
		"number of concurrent workers (0 = auto: runtime.NumCPU())")
	rootCmd.PersistentFlags().StringP("algorithm", "a", "sha1",
		"hash algorithm: md5, sha1 (default), sha256, blake3, xxhash")

	rootCmd.PersistentFlags().BoolP("quiet", "q", false,
		"suppress per-file output; show only the final summary")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false,
		"show per-stage timing and debug information")
	rootCmd.PersistentFlags().String("profile", "",
		"load a named config profile from ~/.pixe/profiles/<name>.yaml")

	// Bind persistent flags to Viper so config file values are also respected.
	_ = viper.BindPFlag("workers", rootCmd.PersistentFlags().Lookup("workers"))
	_ = viper.BindPFlag("algorithm", rootCmd.PersistentFlags().Lookup("algorithm"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("profile", rootCmd.PersistentFlags().Lookup("profile"))
}

// initConfig reads the config file and environment variables.
func initConfig() {
	if cfgFile != "" {
		// Use the file explicitly provided via --config.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search order: current directory, then $HOME.
		viper.SetConfigName(".pixe")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		if home, err := os.UserHomeDir(); err == nil {
			viper.AddConfigPath(home)
		}
		// Also check XDG config dir on Linux/macOS.
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			viper.AddConfigPath(filepath.Join(xdg, "pixe"))
		}
	}

	// Allow environment variables prefixed with PIXE_ to override config.
	// e.g., PIXE_WORKERS=8 overrides the workers setting.
	viper.SetEnvPrefix("PIXE")
	viper.AutomaticEnv()

	// Silently ignore "config file not found" — it's optional.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
