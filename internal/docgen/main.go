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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Target defines a documentation file and its injectable sections.
type Target struct {
	File     string                            // relative path from repo root
	Sections map[string]func() (string, error) // section name → extractor
}

func main() {
	checkMode := flag.Bool("check", false, "check mode: exit 1 if any docs are stale")
	flag.Parse()

	// Change to repo root (two levels up from internal/docgen/).
	// When invoked via `go run ./internal/docgen`, the working directory
	// is already the repo root. We verify by checking for go.mod.
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		// Try to find repo root by walking up.
		dir, _ := os.Getwd()
		for {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				if err := os.Chdir(dir); err != nil {
					fmt.Fprintf(os.Stderr, "docgen: chdir to repo root: %v\n", err)
					os.Exit(1)
				}
				break
			}
		}
	}

	targets := buildTargets()

	var stale []string
	var errors []string

	for _, target := range targets {
		// Build replacements by running all extractors.
		var replacements []Replacement
		for name, extractor := range target.Sections {
			content, err := extractor()
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s[%s]: %v", target.File, name, err))
				continue
			}
			replacements = append(replacements, Replacement{
				Name:    name,
				Content: content,
			})
		}

		if len(errors) > 0 {
			continue
		}

		// Inject into file content.
		result, err := InjectFile(target.File, replacements)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", target.File, err))
			continue
		}

		if *checkMode {
			// Compare against current file content.
			existing, err := os.ReadFile(target.File)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: read: %v", target.File, err))
				continue
			}
			if string(existing) != result {
				stale = append(stale, target.File)
			}
		} else {
			// Write if changed.
			changed, err := WriteIfChanged(target.File, result)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: write: %v", target.File, err))
				continue
			}
			if changed {
				fmt.Fprintf(os.Stderr, "docgen: updated %s\n", target.File)
			} else {
				fmt.Fprintf(os.Stderr, "docgen: unchanged %s\n", target.File)
			}
		}
	}

	if len(errors) > 0 {
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "docgen: error: %s\n", e)
		}
		os.Exit(1)
	}

	if *checkMode {
		if len(stale) > 0 {
			fmt.Fprintf(os.Stderr, "docgen: stale documentation files (run `make docs` to update):\n")
			for _, f := range stale {
				fmt.Fprintf(os.Stderr, "  %s\n", f)
			}
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "docgen: all documentation is up to date\n")
	}
}

// buildTargets returns the hardcoded target manifest mapping files to sections.
func buildTargets() []Target {
	return []Target{
		{
			File: filepath.Join("docs", "_config.yml"),
			Sections: map[string]func() (string, error){
				"version": extractVersionReplacement(),
			},
		},
		{
			File: filepath.Join("docs", "adding-formats.md"),
			Sections: map[string]func() (string, error){
				"interface": extractInterface,
			},
		},
		{
			File: filepath.Join("docs", "commands.md"),
			Sections: map[string]func() (string, error){
				"sort-flags":   extractFlags(filepath.Join("cmd", "sort.go"), "markdown", true),
				"verify-flags": extractFlags(filepath.Join("cmd", "verify.go"), "markdown", true),
				"resume-flags": extractFlags(filepath.Join("cmd", "resume.go"), "markdown", true),
				"status-flags": extractFlags(filepath.Join("cmd", "status.go"), "markdown", true),
				"stats-flags":  extractFlags(filepath.Join("cmd", "stats.go"), "markdown", false),
				"clean-flags":  extractFlags(filepath.Join("cmd", "clean.go"), "markdown", false),
				"query-flags":  extractFlags(filepath.Join("cmd", "query.go"), "markdown", false),
				"query-subs":   extractQuerySubcommands("markdown"),
			},
		},
		{
			File: filepath.Join("docs", "how-it-works.md"),
			Sections: map[string]func() (string, error){
				"format-table": extractFormats("markdown"),
			},
		},
		{
			File: "README.md",
			Sections: map[string]func() (string, error){
				"sort-flags":   extractFlags(filepath.Join("cmd", "sort.go"), "markdown", true),
				"verify-flags": extractFlags(filepath.Join("cmd", "verify.go"), "markdown", true),
				"resume-flags": extractFlags(filepath.Join("cmd", "resume.go"), "markdown", true),
				"status-flags": extractFlags(filepath.Join("cmd", "status.go"), "markdown", true),
				"clean-flags":  extractFlags(filepath.Join("cmd", "clean.go"), "markdown", false),
				"query-flags":  extractFlags(filepath.Join("cmd", "query.go"), "markdown", false),
				"query-subs":   extractQuerySubcommands("markdown"),
				"format-table": extractFormats("markdown"),
			},
		},
		{
			File: filepath.Join("docs", "packages.md"),
			Sections: map[string]func() (string, error){
				"package-list": extractPackageReference,
			},
		},
		{
			File: filepath.Join("docs", "changelog.md"),
			Sections: map[string]func() (string, error){
				"changelog": extractChangelog,
			},
		},
	}
}
