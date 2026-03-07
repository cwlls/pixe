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

package dblocator

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Resolve tests
// ---------------------------------------------------------------------------

// TestResolve_explicitPath verifies that an explicit path always wins and
// sets MarkerNeeded=true.
func TestResolve_explicitPath(t *testing.T) {
	dirB := t.TempDir()
	explicit := "/tmp/custom/archive.db"

	loc, err := Resolve(dirB, explicit)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// DBPath should be the absolute form of the explicit path.
	if !strings.HasSuffix(loc.DBPath, "archive.db") {
		t.Errorf("DBPath = %q, want suffix %q", loc.DBPath, "archive.db")
	}
	if !loc.MarkerNeeded {
		t.Error("MarkerNeeded = false, want true for explicit path")
	}
	if loc.Notice == "" {
		t.Error("Notice is empty, want non-empty for explicit path")
	}
}

// TestResolve_markerFile verifies that a written marker is used when no
// explicit path is provided.
func TestResolve_markerFile(t *testing.T) {
	dirB := t.TempDir()
	markerDB := "/stored/in/marker.db"

	if err := WriteMarker(dirB, markerDB); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	loc, err := Resolve(dirB, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if loc.DBPath != markerDB {
		t.Errorf("DBPath = %q, want %q", loc.DBPath, markerDB)
	}
	if loc.MarkerNeeded {
		t.Error("MarkerNeeded = true, want false (marker already exists)")
	}
}

// TestResolve_localDefault verifies that on a local filesystem with no marker,
// the default dirB/.pixe/pixe.db path is returned.
func TestResolve_localDefault(t *testing.T) {
	dirB := t.TempDir()

	loc, err := Resolve(dirB, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := filepath.Join(dirB, ".pixe", "pixe.db")
	if loc.DBPath != want {
		t.Errorf("DBPath = %q, want %q", loc.DBPath, want)
	}
	if loc.MarkerNeeded {
		t.Error("MarkerNeeded = true, want false for local default")
	}
	if loc.IsRemote {
		t.Error("IsRemote = true, want false for local filesystem")
	}
}

// TestResolve_priorityOrder verifies explicit > marker > default.
func TestResolve_priorityOrder(t *testing.T) {
	dirB := t.TempDir()

	// Write a marker — it should be overridden by the explicit path.
	if err := WriteMarker(dirB, "/marker/path.db"); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	explicit := "/explicit/wins.db"
	loc, err := Resolve(dirB, explicit)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if !strings.HasSuffix(loc.DBPath, "wins.db") {
		t.Errorf("DBPath = %q, want explicit path to win", loc.DBPath)
	}
}

// ---------------------------------------------------------------------------
// WriteMarker / ReadMarker tests
// ---------------------------------------------------------------------------

// TestWriteMarker_ReadMarker_roundtrip verifies that WriteMarker + ReadMarker
// round-trips the database path exactly.
func TestWriteMarker_ReadMarker_roundtrip(t *testing.T) {
	dirB := t.TempDir()
	const dbPath = "/some/absolute/path/pixe.db"

	if err := WriteMarker(dirB, dbPath); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	got, err := ReadMarker(dirB)
	if err != nil {
		t.Fatalf("ReadMarker: %v", err)
	}
	if got != dbPath {
		t.Errorf("ReadMarker = %q, want %q", got, dbPath)
	}
}

// TestReadMarker_notExists verifies that ReadMarker returns ("", nil) when no
// marker file is present.
func TestReadMarker_notExists(t *testing.T) {
	dirB := t.TempDir()

	got, err := ReadMarker(dirB)
	if err != nil {
		t.Fatalf("ReadMarker: %v", err)
	}
	if got != "" {
		t.Errorf("ReadMarker = %q, want %q", got, "")
	}
}

// TestWriteMarker_createsDirectory verifies that WriteMarker creates the
// .pixe directory if it does not exist.
func TestWriteMarker_createsDirectory(t *testing.T) {
	dirB := t.TempDir()
	const dbPath = "/db/path.db"

	if err := WriteMarker(dirB, dbPath); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}

	// Marker file should exist.
	got, err := ReadMarker(dirB)
	if err != nil {
		t.Fatalf("ReadMarker: %v", err)
	}
	if got != dbPath {
		t.Errorf("ReadMarker = %q, want %q", got, dbPath)
	}
}

// ---------------------------------------------------------------------------
// slug tests
// ---------------------------------------------------------------------------

// slugHexRe matches the expected slug format: <base>-<8 hex chars>
var slugHexRe = regexp.MustCompile(`^[a-z0-9-]+-[0-9a-f]{8}$`)

// TestSlug_normalPath verifies the format: <base>-<8hex>.
func TestSlug_normalPath(t *testing.T) {
	s := slug("/Volumes/NAS/Photos/archive")
	if !slugHexRe.MatchString(s) {
		t.Errorf("slug = %q, want format <base>-<8hex>", s)
	}
	if !strings.HasPrefix(s, "archive-") {
		t.Errorf("slug = %q, want prefix %q", s, "archive-")
	}
}

// TestSlug_rootPath verifies the edge case: "/" → "pixe-<8hex>".
func TestSlug_rootPath(t *testing.T) {
	s := slug("/")
	if !slugHexRe.MatchString(s) {
		t.Errorf("slug('/') = %q, want format <base>-<8hex>", s)
	}
	if !strings.HasPrefix(s, "pixe-") {
		t.Errorf("slug('/') = %q, want prefix %q", s, "pixe-")
	}
}

// TestSlug_deterministic verifies that the same input always produces the same slug.
func TestSlug_deterministic(t *testing.T) {
	const path = "/Volumes/NAS/Photos/archive"
	s1 := slug(path)
	s2 := slug(path)
	if s1 != s2 {
		t.Errorf("slug not deterministic: %q != %q", s1, s2)
	}
}

// TestSlug_differentPaths verifies that different inputs produce different slugs.
func TestSlug_differentPaths(t *testing.T) {
	s1 := slug("/Volumes/NAS/Photos/archive")
	s2 := slug("/Volumes/NAS/Photos/backup")
	if s1 == s2 {
		t.Errorf("slug collision: %q == %q for different paths", s1, s2)
	}
}

// TestSlug_sanitizesSpecialChars verifies that special characters in the base
// component are stripped, leaving only alphanumeric and hyphens.
func TestSlug_sanitizesSpecialChars(t *testing.T) {
	s := slug("/mnt/my archive (2025)!")
	if !slugHexRe.MatchString(s) {
		t.Errorf("slug = %q, want format <base>-<8hex>", s)
	}
	// The base should not contain spaces, parens, or exclamation marks.
	base := strings.SplitN(s, "-", 2)[0]
	if strings.ContainsAny(base, " ()!") {
		t.Errorf("slug base %q contains special characters", base)
	}
}

// TestSlug_emptyBase verifies that a path whose base sanitizes to empty
// falls back to "pixe".
func TestSlug_emptyBase(t *testing.T) {
	// A path whose base is all special characters.
	s := slug("/mnt/!!!")
	if !strings.HasPrefix(s, "pixe-") {
		t.Errorf("slug = %q, want prefix %q for empty-base path", s, "pixe-")
	}
}
