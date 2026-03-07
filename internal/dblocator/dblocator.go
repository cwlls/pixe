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

// Package dblocator resolves the filesystem path for the Pixe archive database.
// It implements the priority chain: explicit --db-path → dbpath marker file →
// local default (dirB/.pixe/pixe.db), with automatic fallback to
// ~/.pixe/databases/<slug>.db when dirB is on a network filesystem.
package dblocator

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	pixeDir    = ".pixe"
	markerFile = "dbpath"
	dbFile     = "pixe.db"
)

// Location holds the resolved database path and metadata about the resolution.
type Location struct {
	// DBPath is the absolute path to the SQLite database file.
	DBPath string
	// IsRemote is true if dirB was detected as a network mount.
	IsRemote bool
	// MarkerNeeded is true if a dbpath marker should be written to dirB/.pixe/.
	MarkerNeeded bool
	// Notice is a user-facing message explaining the location choice.
	// Empty if the default local path was used.
	Notice string
}

// Resolve determines the database path for the given destination directory.
//
// Priority chain:
//  1. explicitPath (from --db-path flag) — used unconditionally if non-empty.
//  2. dirB/.pixe/dbpath marker file — if it exists, its contents are used.
//  3. dirB/.pixe/pixe.db — if dirB is on a local filesystem.
//  4. ~/.pixe/databases/<slug>.db — if dirB is on a network mount.
func Resolve(dirB string, explicitPath string) (*Location, error) {
	// Priority 1: explicit --db-path flag.
	if explicitPath != "" {
		abs, err := filepath.Abs(explicitPath)
		if err != nil {
			return nil, fmt.Errorf("dblocator: resolve explicit path: %w", err)
		}
		return &Location{
			DBPath:       abs,
			MarkerNeeded: true,
			Notice:       fmt.Sprintf("Using explicit database path: %s", abs),
		}, nil
	}

	// Priority 2: dbpath marker file.
	markerPath, err := ReadMarker(dirB)
	if err != nil {
		return nil, fmt.Errorf("dblocator: read marker: %w", err)
	}
	if markerPath != "" {
		return &Location{
			DBPath:       markerPath,
			MarkerNeeded: false,
		}, nil
	}

	// Priority 3 / 4: local vs. network mount.
	remote, err := isNetworkMount(dirB)
	if err != nil {
		// If detection fails (e.g. dirB doesn't exist yet), treat as local.
		remote = false
	}

	if remote {
		// Priority 4: network mount — store DB in ~/.pixe/databases/<slug>.db
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("dblocator: get home dir: %w", err)
		}
		s := slug(dirB)
		dbPath := filepath.Join(home, ".pixe", "databases", s+".db")
		return &Location{
			DBPath:       dbPath,
			IsRemote:     true,
			MarkerNeeded: true,
			Notice: fmt.Sprintf(
				"Destination is on a network filesystem. Storing database locally at: %s", dbPath,
			),
		}, nil
	}

	// Priority 3: local filesystem — dirB/.pixe/pixe.db
	dbPath := filepath.Join(dirB, pixeDir, dbFile)
	return &Location{
		DBPath:       dbPath,
		MarkerNeeded: false,
	}, nil
}

// WriteMarker writes the dbpath marker file at dirB/.pixe/dbpath
// containing the absolute path to the database. No trailing newline.
func WriteMarker(dirB string, dbPath string) error {
	dir := filepath.Join(dirB, pixeDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("dblocator: create .pixe dir: %w", err)
	}
	path := filepath.Join(dir, markerFile)
	if err := os.WriteFile(path, []byte(dbPath), 0644); err != nil {
		return fmt.Errorf("dblocator: write marker: %w", err)
	}
	return nil
}

// ReadMarker reads the dbpath marker file at dirB/.pixe/dbpath.
// Returns ("", nil) if the marker does not exist.
func ReadMarker(dirB string) (string, error) {
	path := filepath.Join(dirB, pixeDir, markerFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("dblocator: read marker: %w", err)
	}
	return strings.TrimRight(string(data), "\n\r"), nil
}

// sanitizeRe matches characters that are NOT alphanumeric or hyphens.
var sanitizeRe = regexp.MustCompile(`[^a-z0-9-]+`)

// slug generates a human-readable, collision-resistant identifier for a dirB path.
// Format: <last-path-component>-<8 hex chars of SHA-256>.
// Example: "/Volumes/NAS/Photos/archive" → "archive-a1b2c3d4"
func slug(dirB string) string {
	abs, _ := filepath.Abs(dirB)
	base := strings.ToLower(filepath.Base(abs))
	base = sanitizeRe.ReplaceAllString(base, "")
	if base == "" || base == "." {
		base = "pixe"
	}
	h := sha256.Sum256([]byte(abs))
	return fmt.Sprintf("%s-%x", base, h[:4])
}
