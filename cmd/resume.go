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
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/manifest"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume an interrupted sort operation using the saved manifest",
	Long: `Resume reads the manifest at <dir>/.pixe/manifest.json and continues
processing any files that did not reach the 'complete' state.

Files already marked 'complete' are skipped. Files in earlier states
re-enter the pipeline from the beginning.`,
	RunE: runResume,
}

func runResume(cmd *cobra.Command, args []string) error {
	dir := viper.GetString("resume_dir")
	if dir == "" {
		return fmt.Errorf("--dir is required")
	}

	// Validate destination directory exists.
	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf("destination directory %q: %w", dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	// Load the manifest — error if not found.
	m, err := manifest.Load(dir)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	if m == nil {
		return fmt.Errorf("no manifest found at %q — has pixe sort been run?", dir)
	}

	// Validate source directory still exists.
	if info, err := os.Stat(m.Source); err != nil {
		return fmt.Errorf("source directory from manifest %q: %w", m.Source, err)
	} else if !info.IsDir() {
		return fmt.Errorf("source %q from manifest is not a directory", m.Source)
	}

	// Resolve workers.
	workers := viper.GetInt("workers")
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	// Build hasher using the algorithm recorded in the manifest.
	h, err := hash.NewHasher(m.Algorithm)
	if err != nil {
		return fmt.Errorf("hash algorithm from manifest %q: %w", m.Algorithm, err)
	}

	// Build registry.
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())

	cfg := &config.AppConfig{
		Source:      m.Source,
		Destination: dir,
		Workers:     workers,
		Algorithm:   m.Algorithm,
	}

	opts := pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       os.Stdout,
	}

	fmt.Fprintf(os.Stdout, "Resuming sort: source=%s dest=%s\n", m.Source, dir)

	result, err := pipeline.Run(opts)
	if err != nil {
		return fmt.Errorf("resume failed: %w", err)
	}

	if result.Errors > 0 {
		return fmt.Errorf("%d file(s) failed to process — check output above", result.Errors)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(resumeCmd)

	resumeCmd.Flags().StringP("dir", "d", "", "destination directory containing .pixe/manifest.json (required)")
	_ = resumeCmd.MarkFlagRequired("dir")
	_ = viper.BindPFlag("resume_dir", resumeCmd.Flags().Lookup("dir"))
}
