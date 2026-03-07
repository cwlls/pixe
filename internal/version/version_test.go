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

package version

import (
	"strings"
	"testing"
)

func TestVersion_isSet(t *testing.T) {
	if Version == "" {
		t.Error("Version constant must not be empty")
	}
}

func TestVersion_semverFormat(t *testing.T) {
	parts := strings.Split(Version, ".")
	if len(parts) != 3 {
		t.Errorf("Version %q is not in MAJOR.MINOR.PATCH format", Version)
	}
	if strings.HasPrefix(Version, "v") {
		t.Errorf("Version %q should not have a 'v' prefix", Version)
	}
}

func TestFull_format(t *testing.T) {
	s := Full()
	if !strings.HasPrefix(s, "pixe v") {
		t.Errorf("Full() = %q, want prefix 'pixe v'", s)
	}
	if !strings.Contains(s, Version) {
		t.Errorf("Full() = %q, does not contain Version %q", s, Version)
	}
	if !strings.Contains(s, "commit:") {
		t.Errorf("Full() = %q, missing 'commit:' label", s)
	}
	if !strings.Contains(s, "built:") {
		t.Errorf("Full() = %q, missing 'built:' label", s)
	}
}

func TestFull_defaultValues(t *testing.T) {
	// Save and restore so we don't pollute other tests.
	origCommit, origBuildDate := Commit, BuildDate
	Commit, BuildDate = "unknown", "unknown"
	defer func() { Commit, BuildDate = origCommit, origBuildDate }()

	s := Full()
	if !strings.Contains(s, "unknown") {
		t.Errorf("Full() = %q, expected 'unknown' for default Commit/BuildDate", s)
	}
}
