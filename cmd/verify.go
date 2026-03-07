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

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the integrity of a sorted archive by recomputing checksums",
	Long: `Verify walks a previously sorted destination directory (--dir), parses the
checksum embedded in each filename, recomputes the data-only hash of each file,
and reports any mismatches.

Exit code 0 means all files verified successfully.
Exit code 1 means one or more mismatches were detected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("pixe verify: not implemented")
		fmt.Printf("  dir:       %s\n", viper.GetString("verify_dir"))
		fmt.Printf("  workers:   %d\n", viper.GetInt("workers"))
		fmt.Printf("  algorithm: %s\n", viper.GetString("algorithm"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringP("dir", "d", "", "archive directory to verify (required)")
	_ = verifyCmd.MarkFlagRequired("dir")
	_ = viper.BindPFlag("verify_dir", verifyCmd.Flags().Lookup("dir"))
}
