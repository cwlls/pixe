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

Documentation: https://github.com/wellsiau/pixe-go`,
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
		"hash algorithm to use: sha1, sha256")

	// Bind persistent flags to Viper so config file values are also respected.
	_ = viper.BindPFlag("workers", rootCmd.PersistentFlags().Lookup("workers"))
	_ = viper.BindPFlag("algorithm", rootCmd.PersistentFlags().Lookup("algorithm"))
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
