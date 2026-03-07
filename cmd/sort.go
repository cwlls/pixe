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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("pixe sort: not implemented")
		fmt.Printf("  source:       %s\n", viper.GetString("source"))
		fmt.Printf("  dest:         %s\n", viper.GetString("dest"))
		fmt.Printf("  workers:      %d\n", viper.GetInt("workers"))
		fmt.Printf("  algorithm:    %s\n", viper.GetString("algorithm"))
		fmt.Printf("  copyright:    %s\n", viper.GetString("copyright"))
		fmt.Printf("  camera-owner: %s\n", viper.GetString("camera_owner"))
		fmt.Printf("  dry-run:      %v\n", viper.GetBool("dry_run"))
		return nil
	},
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
