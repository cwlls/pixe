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
)

// version fields are injected at build time via -ldflags -X.
// When built without ldflags (e.g. plain `go build` or `go test`), these
// retain their dev defaults.
//
// GoReleaser injects all three from the git tag, commit SHA, and build
// timestamp. The Makefile (via goreleaser build --snapshot) injects commit
// and buildDate but leaves version as "dev"; init() enriches that to
// "dev-<commit>" for traceability.
//
// ldflags targets:
//
//	-X github.com/cwlls/pixe/cmd.version={{.Version}}
//	-X github.com/cwlls/pixe/cmd.commit={{.Commit}}
//	-X github.com/cwlls/pixe/cmd.buildDate={{.Date}}
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func init() {
	// Enrich dev builds with the commit hash for traceability.
	// A goreleaser --snapshot build injects commit but not version,
	// producing e.g. "dev-2159446" so local binaries are traceable.
	if version == "dev" && commit != "unknown" {
		version = "dev-" + commit
	}

	rootCmd.AddCommand(versionCmd)
}

// Version returns the current version string for use by packages that cannot
// import cmd directly (e.g. pipeline, which cmd already imports).
// Callers in the cmd package should read the version var directly.
func Version() string { return version }

// fullVersion returns the canonical human-readable version string used by
// the `pixe version` CLI command.
//
// Examples:
//
//	Release build:  "pixe v0.23 (commit: abc1234, built: 2026-03-16T12:00:00Z)"
//	Snapshot build: "pixe vdev-2159446 (commit: 2159446, built: 2026-03-16T12:00:00Z)"
//	Bare go build:  "pixe vdev (commit: unknown, built: unknown)"
func fullVersion() string {
	return fmt.Sprintf("pixe v%s (commit: %s, built: %s)", version, commit, buildDate)
}

// versionCmd is the "pixe version" subcommand.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Pixe",
	Long:  "Print the version, git commit, and build date of the Pixe binary.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(fullVersion())
	},
}
