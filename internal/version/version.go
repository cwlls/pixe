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

// Package version provides the centralized version constant for Pixe.
// This is the single source of truth — update the Version constant here
// when cutting a new release.
//
// Version follows Semantic Versioning (MAJOR.MINOR.PATCH) without a "v"
// prefix in the constant; the "v" is prepended at display time by Full().
//
// Commit and BuildDate are package-level variables (not constants) so that
// the build system can inject real values at link time via -ldflags -X:
//
//	go build -ldflags "\
//	  -X 'github.com/cwlls/pixe-go/internal/version.Commit=$(git rev-parse --short HEAD)' \
//	  -X 'github.com/cwlls/pixe-go/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
//
// When built without ldflags (e.g. plain `go build` or `go test`), both
// variables retain their default value of "unknown".
package version

import "fmt"

// Version is the semantic version of Pixe (without the "v" prefix).
// Update this constant when cutting a new release; no other file needs
// to change for a version bump.
const Version = "0.9.6"

// Commit is the short git SHA of the build, injected at link time via
// -ldflags. Defaults to "unknown" when not set.
var Commit = "unknown"

// BuildDate is the UTC timestamp of the build, injected at link time via
// -ldflags. Defaults to "unknown" when not set.
var BuildDate = "unknown"

// Full returns the canonical human-readable version string used by the
// `pixe version` CLI command and any other consumer that needs a single
// formatted line.
//
// Example output:
//
//	pixe v0.9.0 (commit: abc1234, built: 2026-03-06T10:30:00Z)
func Full() string {
	return fmt.Sprintf("pixe v%s (commit: %s, built: %s)", Version, Commit, BuildDate)
}
