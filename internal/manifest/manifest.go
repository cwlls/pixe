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

// Package manifest handles atomic persistence of the Pixe manifest and
// ledger JSON files.
//
// Manifest — written to <dirB>/.pixe/manifest.json — is the operational
// journal for a sort run. It tracks per-file pipeline state and enables
// interrupted runs to be resumed.
//
// Ledger — written to <dirA>/.pixe_ledger.json — is the minimal,
// source-side record of files that were successfully processed. It is the
// only file Pixe writes into the source directory.
//
// Both files are written atomically: the new content is first written to a
// temporary file in the same directory, then renamed over the target. On
// POSIX filesystems rename(2) is atomic within a single filesystem, so a
// crash between write and rename leaves the previous version intact.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwlls/pixe-go/internal/domain"
)

const (
	// manifestDir is the subdirectory within dirB that holds Pixe metadata.
	manifestDir = ".pixe"
	// manifestFile is the filename of the operational journal.
	manifestFile = "manifest.json"
	// ledgerFile is the filename of the source-side ledger.
	ledgerFile = ".pixe_ledger.json"

	manifestVersion = 1
	ledgerVersion   = 1
)

// manifestPath returns the absolute path to the manifest file.
func manifestPath(dirB string) string {
	return filepath.Join(dirB, manifestDir, manifestFile)
}

// ledgerPath returns the absolute path to the ledger file.
func ledgerPath(dirA string) string {
	return filepath.Join(dirA, ledgerFile)
}

// Save atomically writes m to <dirB>/.pixe/manifest.json.
// It creates the .pixe directory if it does not exist.
// The write is atomic: content is written to a .tmp file first, then
// renamed over the target so a crash mid-write leaves the previous
// version intact.
func Save(m *domain.Manifest, dirB string) error {
	dir := filepath.Join(dirB, manifestDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("manifest: create directory %q: %w", dir, err)
	}
	return atomicWriteJSON(m, manifestPath(dirB))
}

// Load reads and deserialises the manifest from <dirB>/.pixe/manifest.json.
// Returns (nil, nil) if the manifest file does not exist — callers treat
// this as "no prior run".
func Load(dirB string) (*domain.Manifest, error) {
	path := manifestPath(dirB)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("manifest: read %q: %w", path, err)
	}
	var m domain.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: parse %q: %w", path, err)
	}
	return &m, nil
}

// SaveLedger atomically writes l to <dirA>/.pixe_ledger.json.
func SaveLedger(l *domain.Ledger, dirA string) error {
	return atomicWriteJSON(l, ledgerPath(dirA))
}

// LoadLedger reads and deserialises the ledger from <dirA>/.pixe_ledger.json.
// Returns (nil, nil) if the ledger file does not exist.
func LoadLedger(dirA string) (*domain.Ledger, error) {
	path := ledgerPath(dirA)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("manifest: read ledger %q: %w", path, err)
	}
	var l domain.Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("manifest: parse ledger %q: %w", path, err)
	}
	return &l, nil
}

// atomicWriteJSON marshals v to indented JSON and writes it to target
// atomically via a sibling .tmp file and os.Rename.
func atomicWriteJSON(v any, target string) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest: marshal JSON: %w", err)
	}
	// Write to a temp file in the same directory so rename is same-filesystem.
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("manifest: write temp file %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		// Best-effort cleanup of the orphaned temp file.
		_ = os.Remove(tmp)
		return fmt.Errorf("manifest: rename %q → %q: %w", tmp, target, err)
	}
	return nil
}
