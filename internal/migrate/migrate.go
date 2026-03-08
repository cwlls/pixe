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

// Package migrate handles automatic migration from the legacy JSON manifest
// (dirB/.pixe/manifest.json) to the SQLite archive database.
//
// Migration is a one-time, idempotent operation: if manifest.json exists and
// manifest.json.migrated does not, the manifest is imported into the database
// as a synthetic completed run, then renamed to manifest.json.migrated.
// Subsequent calls are no-ops.
package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/cwlls/pixe-go/internal/archivedb"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/manifest"
)

const (
	pixeDir        = ".pixe"
	manifestFile   = "manifest.json"
	migratedSuffix = ".migrated"
)

// Result holds the outcome of a migration attempt.
type Result struct {
	// Migrated is true if a migration was actually performed.
	Migrated bool
	// FileCount is the number of file entries migrated.
	FileCount int
	// Notice is a user-facing message describing what happened.
	Notice string
}

// MigrateIfNeeded checks for a legacy manifest.json at dirB/.pixe/ and,
// if found (and no .migrated sentinel exists), migrates its contents into
// the provided database as a synthetic completed run.
//
// Steps:
//  1. Check for dirB/.pixe/manifest.json — if absent, return (not migrated).
//  2. Check for dirB/.pixe/manifest.json.migrated — if present, skip (already done).
//  3. Read and parse the JSON manifest via manifest.Load.
//  4. Create a synthetic run in the DB using manifest metadata.
//  5. Insert all file entries into the DB, mapping ManifestEntry → FileRecord.
//  6. Rename manifest.json → manifest.json.migrated.
//  7. Return Result with a user-facing notice.
func MigrateIfNeeded(db *archivedb.DB, dirB string) (*Result, error) {
	manifestPath := filepath.Join(dirB, pixeDir, manifestFile)
	migratedPath := manifestPath + migratedSuffix

	// Step 1: no manifest → nothing to do.
	if _, err := os.Stat(manifestPath); errors.Is(err, os.ErrNotExist) {
		return &Result{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("migrate: stat manifest: %w", err)
	}

	// Step 2: already migrated → idempotent no-op.
	if _, err := os.Stat(migratedPath); err == nil {
		return &Result{}, nil
	}

	// Step 3: load and parse the manifest.
	m, err := manifest.Load(dirB)
	if err != nil {
		return nil, fmt.Errorf("migrate: load manifest: %w", err)
	}
	if m == nil {
		// Shouldn't happen given the stat above, but be defensive.
		return &Result{}, nil
	}

	// Step 4: create a synthetic run representing the prior completed sort.
	runID := uuid.New().String()
	syntheticRun := &archivedb.Run{
		ID:          runID,
		PixeVersion: m.PixeVersion,
		Source:      m.Source,
		Destination: m.Destination,
		Algorithm:   m.Algorithm,
		Workers:     m.Workers,
		StartedAt:   m.StartedAt,
		FinishedAt:  &m.StartedAt, // best approximation — manifest doesn't record finished_at
	}
	if err := db.InsertRun(syntheticRun); err != nil {
		return nil, fmt.Errorf("migrate: insert synthetic run: %w", err)
	}
	// Mark it completed immediately (InsertRun sets status="running").
	if err := db.CompleteRun(runID, m.StartedAt); err != nil {
		return nil, fmt.Errorf("migrate: complete synthetic run: %w", err)
	}

	// Step 5: map ManifestEntry → FileRecord and batch-insert as "pending",
	// then apply per-file field updates.
	destPrefix := strings.TrimRight(m.Destination, "/") + "/"

	// Build the minimal records for batch insertion.
	pendingRecords := make([]*archivedb.FileRecord, len(m.Files))
	for i, entry := range m.Files {
		pendingRecords[i] = &archivedb.FileRecord{
			RunID:      runID,
			SourcePath: entry.Source,
		}
	}

	ids, err := db.InsertFiles(pendingRecords)
	if err != nil {
		return nil, fmt.Errorf("migrate: batch insert files: %w", err)
	}

	// Apply the full field set for each entry.
	for i, entry := range m.Files {
		if err := applyEntryUpdates(db, ids[i], entry, destPrefix); err != nil {
			return nil, fmt.Errorf("migrate: update file %d (%s): %w", i, entry.Source, err)
		}
	}

	// Step 6: rename manifest.json → manifest.json.migrated.
	if err := os.Rename(manifestPath, migratedPath); err != nil {
		return nil, fmt.Errorf("migrate: rename manifest: %w", err)
	}

	// Step 7: return result.
	notice := fmt.Sprintf("Migrated %d file(s) from manifest.json → pixe.db", len(m.Files))
	return &Result{
		Migrated:  true,
		FileCount: len(m.Files),
		Notice:    notice,
	}, nil
}

// applyEntryUpdates applies the full set of field updates for a migrated
// ManifestEntry to the already-inserted FileRecord row identified by fileID.
// Files that are still "pending" need no further update beyond the default.
func applyEntryUpdates(db *archivedb.DB, fileID int64, entry *domain.ManifestEntry, destPrefix string) error {
	status := string(entry.Status)
	if status == "" || status == "pending" {
		return nil
	}

	opts := []archivedb.UpdateOption{}

	// Checksum.
	if entry.Checksum != "" {
		opts = append(opts, archivedb.WithChecksum(entry.Checksum))
	}

	// Destination paths and duplicate inference.
	if entry.Destination != "" {
		destRel := strings.TrimPrefix(entry.Destination, destPrefix)
		opts = append(opts, archivedb.WithDestination(entry.Destination, destRel))
		if strings.Contains(destRel, "duplicates/") {
			opts = append(opts, archivedb.WithIsDuplicate(true))
		}
	}

	// Timestamps — set whichever are present; UpdateFileStatus will also set
	// the status-driven timestamp for the final status, but we need to
	// preserve intermediate timestamps too. We do this by walking the
	// progression in order and updating each stage that has a timestamp.
	if entry.ExtractedAt != nil {
		if err := db.UpdateFileStatus(fileID, "extracted"); err != nil {
			return err
		}
	}
	if entry.CopiedAt != nil {
		copiedOpts := []archivedb.UpdateOption{}
		if entry.Destination != "" {
			destRel := strings.TrimPrefix(entry.Destination, destPrefix)
			copiedOpts = append(copiedOpts, archivedb.WithDestination(entry.Destination, destRel))
		}
		if entry.Checksum != "" {
			copiedOpts = append(copiedOpts, archivedb.WithChecksum(entry.Checksum))
		}
		if err := db.UpdateFileStatus(fileID, "copied", copiedOpts...); err != nil {
			return err
		}
	}
	if entry.VerifiedAt != nil {
		if err := db.UpdateFileStatus(fileID, "verified"); err != nil {
			return err
		}
	}
	if entry.TaggedAt != nil {
		if err := db.UpdateFileStatus(fileID, "tagged"); err != nil {
			return err
		}
	}

	// Error message.
	if entry.Error != "" {
		opts = append(opts, archivedb.WithError(entry.Error))
	}

	// Apply the terminal/final status with all accumulated options.
	return db.UpdateFileStatus(fileID, status, opts...)
}
