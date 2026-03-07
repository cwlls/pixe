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

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume an interrupted sort operation using the saved manifest",
	Long: `Resume reads the manifest at <dir>/.pixe/manifest.json and continues
processing any files that did not reach the 'complete' state.

Files already marked 'complete' are skipped. Files in the 'copied' state are
re-verified. Files in earlier states re-enter the pipeline from the beginning.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("pixe resume: not implemented")
		fmt.Printf("  dir: %s\n", viper.GetString("resume_dir"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resumeCmd)

	resumeCmd.Flags().StringP("dir", "d", "", "destination directory containing .pixe/manifest.json (required)")
	_ = resumeCmd.MarkFlagRequired("dir")
	_ = viper.BindPFlag("resume_dir", resumeCmd.Flags().Lookup("dir"))
}
