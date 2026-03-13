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
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/cli"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/progress"
	"github.com/cwlls/pixe-go/internal/verify"
)

// verifyCmd is the "pixe verify" subcommand.
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the integrity of a sorted archive by recomputing checksums",
	Long: `Verify walks a previously sorted destination directory (--dir), parses the
checksum embedded in each filename, recomputes the data-only hash of each file,
and reports any mismatches.

Exit code 0 means all files verified successfully.
Exit code 1 means one or more mismatches were detected.`,
	RunE: runVerify,
}

// runVerify is the RunE handler for the verify subcommand.
func runVerify(cmd *cobra.Command, args []string) error {
	dir := viper.GetString("verify_dir")
	algorithm := viper.GetString("algorithm")

	if dir == "" {
		return fmt.Errorf("--dir is required")
	}

	// Validate directory exists.
	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf("archive directory %q: %w", dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	h, err := hash.NewHasher(algorithm)
	if err != nil {
		return fmt.Errorf("hash algorithm: %w", err)
	}

	reg := buildRegistry()

	useProgress := viper.GetBool("verify_progress") && isatty.IsTerminal(os.Stdout.Fd())

	opts := verify.Options{
		Dir:      dir,
		Hasher:   h,
		Registry: reg,
		Output:   os.Stdout,
	}

	var result verify.Result
	if useProgress {
		bus := progress.NewBus(256)
		opts.EventBus = bus
		opts.Output = io.Discard

		model := cli.NewProgressModel(bus, dir, "", "verify")
		p := tea.NewProgram(model)

		var verifyErr error
		done := make(chan struct{})
		go func() {
			defer close(done)
			result, verifyErr = verify.Run(opts)
			bus.Close()
		}()

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("progress UI: %w", err)
		}
		<-done
		if verifyErr != nil {
			return fmt.Errorf("verify failed: %w", verifyErr)
		}
	} else {
		var err error
		result, err = verify.Run(opts)
		if err != nil {
			return fmt.Errorf("verify failed: %w", err)
		}
	}

	if result.Mismatches > 0 {
		return fmt.Errorf("%d mismatch(es) detected", result.Mismatches)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringP("dir", "d", "", "archive directory to verify (required)")
	verifyCmd.Flags().Bool("progress", false, "show a live progress bar instead of per-file text output (requires a TTY)")
	_ = verifyCmd.MarkFlagRequired("dir")
	// Note: --algorithm is inherited from the root command. For new-format files
	// (YYYYMMDD_HHMMSS-<ID>-<CHECKSUM>), the algorithm is auto-detected from the
	// embedded ID and this flag is ignored. For legacy files, it is used as a
	// fallback when the digest length is ambiguous.
	_ = viper.BindPFlag("verify_dir", verifyCmd.Flags().Lookup("dir"))
	_ = viper.BindPFlag("verify_progress", verifyCmd.Flags().Lookup("progress"))
}
