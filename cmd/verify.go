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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/discovery"
	arwhandler "github.com/cwlls/pixe-go/internal/handler/arw"
	cr2handler "github.com/cwlls/pixe-go/internal/handler/cr2"
	cr3handler "github.com/cwlls/pixe-go/internal/handler/cr3"
	dnghandler "github.com/cwlls/pixe-go/internal/handler/dng"
	heichandler "github.com/cwlls/pixe-go/internal/handler/heic"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	mp4handler "github.com/cwlls/pixe-go/internal/handler/mp4"
	nefhandler "github.com/cwlls/pixe-go/internal/handler/nef"
	pefhandler "github.com/cwlls/pixe-go/internal/handler/pef"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/verify"
)

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

	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	reg.Register(heichandler.New())
	reg.Register(mp4handler.New())
	reg.Register(dnghandler.New())
	reg.Register(nefhandler.New())
	reg.Register(cr2handler.New())
	reg.Register(cr3handler.New())
	reg.Register(pefhandler.New())
	reg.Register(arwhandler.New())

	result, err := verify.Run(verify.Options{
		Dir:      dir,
		Hasher:   h,
		Registry: reg,
		Output:   os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("verify failed: %w", err)
	}

	if result.Mismatches > 0 {
		return fmt.Errorf("%d mismatch(es) detected", result.Mismatches)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringP("dir", "d", "", "archive directory to verify (required)")
	_ = verifyCmd.MarkFlagRequired("dir")
	_ = viper.BindPFlag("verify_dir", verifyCmd.Flags().Lookup("dir"))
}
