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
