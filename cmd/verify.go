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
