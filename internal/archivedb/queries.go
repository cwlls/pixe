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

package archivedb

import (
	"database/sql"
	"fmt"
	"time"
)

// RunSummary is a lightweight view of a run for listing purposes.
type RunSummary struct {
	ID          string
	PixeVersion string
	Source      string
	StartedAt   time.Time
	FinishedAt  *time.Time
	Status      string
	FileCount   int
}

// FileWithSource wraps a FileRecord with the source directory of its run.
// Used by FilesWithErrors to provide context about which run produced the error.
type FileWithSource struct {
	FileRecord
	RunSource string
}

// DuplicatePair pairs a duplicate file with the original it duplicates.
type DuplicatePair struct {
	DuplicateSource string
	DuplicateDest   string
	OriginalDest    string
}

// InventoryEntry represents a single canonical file in the archive.
type InventoryEntry struct {
	DestRel     string
	Checksum    string
	CaptureDate *time.Time
}

// FormatCount holds a file extension and its count in the archive.
type FormatCount struct {
	Extension string `json:"extension"`
	Count     int    `json:"count"`
}

// ArchiveStats holds aggregate statistics for the entire archive database.
// Used to populate summary lines in pixe query output.
type ArchiveStats struct {
	TotalFiles      int
	Complete        int        // complete AND is_duplicate = 0
	Duplicates      int        // is_duplicate = 1
	Failed          int        // status = 'failed'
	Mismatches      int        // status = 'mismatch'
	TagFailed       int        // status = 'tag_failed'
	Skipped         int        // status = 'skipped'
	TotalSize       int64      // SUM(file_size) across all files
	RunCount        int        // total rows in runs table
	EarliestCapture *time.Time // MIN(capture_date) across all files
	LatestCapture   *time.Time // MAX(capture_date) across all files
}

// ListRuns returns all runs ordered by started_at descending, with file counts.
func (db *DB) ListRuns() ([]*RunSummary, error) {
	const q = `
		SELECT r.id, r.pixe_version, r.source, r.started_at, r.finished_at, r.status,
		       COUNT(f.id) AS file_count
		FROM runs r
		LEFT JOIN files f ON f.run_id = r.id
		GROUP BY r.id
		ORDER BY r.started_at DESC`

	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("archivedb: list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []*RunSummary
	for rows.Next() {
		s, err := scanRunSummary(rows)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archivedb: list runs iterate: %w", err)
	}
	return summaries, nil
}

// FilesBySource returns all files imported from a given source directory.
// The sourceDir is matched against the run's source field.
func (db *DB) FilesBySource(sourceDir string) ([]*FileRecord, error) {
	const q = `
		SELECT f.id, f.run_id, f.source_path, f.dest_path, f.dest_rel, f.checksum,
		       f.status, f.is_duplicate, f.capture_date, f.file_size,
		       f.extracted_at, f.hashed_at, f.copied_at, f.verified_at, f.tagged_at, f.error,
		       f.skip_reason, f.carried_sidecars
		FROM files f
		JOIN runs r ON r.id = f.run_id
		WHERE r.source = ?
		ORDER BY f.id`

	rows, err := db.conn.Query(q, sourceDir)
	if err != nil {
		return nil, fmt.Errorf("archivedb: files by source: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFileRows(rows)
}

// FilesByCaptureDateRange returns completed files with capture dates in [start, end].
// Only files with status "complete" and a non-NULL capture_date are returned.
func (db *DB) FilesByCaptureDateRange(start, end time.Time) ([]*FileRecord, error) {
	const q = `
		SELECT id, run_id, source_path, dest_path, dest_rel, checksum,
		       status, is_duplicate, capture_date, file_size,
		       extracted_at, hashed_at, copied_at, verified_at, tagged_at, error,
		       skip_reason, carried_sidecars
		FROM files
		WHERE status = 'complete'
		  AND capture_date IS NOT NULL
		  AND capture_date >= ?
		  AND capture_date <= ?
		ORDER BY capture_date, id`

	startStr := start.UTC().Format(time.RFC3339)
	endStr := end.UTC().Format(time.RFC3339)

	rows, err := db.conn.Query(q, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("archivedb: files by capture date range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFileRows(rows)
}

// FilesByImportDateRange returns files verified within [start, end].
// "Import date" is defined as verified_at — the timestamp when the copy was confirmed.
func (db *DB) FilesByImportDateRange(start, end time.Time) ([]*FileRecord, error) {
	const q = `
		SELECT id, run_id, source_path, dest_path, dest_rel, checksum,
		       status, is_duplicate, capture_date, file_size,
		       extracted_at, hashed_at, copied_at, verified_at, tagged_at, error,
		       skip_reason, carried_sidecars
		FROM files
		WHERE verified_at IS NOT NULL
		  AND verified_at >= ?
		  AND verified_at <= ?
		ORDER BY verified_at, id`

	startStr := start.UTC().Format(time.RFC3339)
	endStr := end.UTC().Format(time.RFC3339)

	rows, err := db.conn.Query(q, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("archivedb: files by import date range: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFileRows(rows)
}

// FilesWithErrors returns all files in error states across all runs,
// joined with their run's source directory for context.
// Error states: "failed", "mismatch", "tag_failed".
func (db *DB) FilesWithErrors() ([]*FileWithSource, error) {
	const q = `
		SELECT f.id, f.run_id, f.source_path, f.dest_path, f.dest_rel, f.checksum,
		       f.status, f.is_duplicate, f.capture_date, f.file_size,
		       f.extracted_at, f.hashed_at, f.copied_at, f.verified_at, f.tagged_at, f.error,
		       f.skip_reason, f.carried_sidecars,
		       r.source AS run_source
		FROM files f
		JOIN runs r ON r.id = f.run_id
		WHERE f.status IN ('failed', 'mismatch', 'tag_failed')
		ORDER BY f.id`

	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("archivedb: files with errors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*FileWithSource
	for rows.Next() {
		fws, err := scanFileWithSource(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, fws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archivedb: files with errors iterate: %w", err)
	}
	return results, nil
}

// AllDuplicates returns all files marked as duplicates (is_duplicate = 1).
func (db *DB) AllDuplicates() ([]*FileRecord, error) {
	const q = `
		SELECT id, run_id, source_path, dest_path, dest_rel, checksum,
		       status, is_duplicate, capture_date, file_size,
		       extracted_at, hashed_at, copied_at, verified_at, tagged_at, error,
		       skip_reason, carried_sidecars
		FROM files
		WHERE is_duplicate = 1
		ORDER BY id`

	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("archivedb: all duplicates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFileRows(rows)
}

// DuplicatePairs returns each duplicate alongside the original it duplicates.
// The join is performed on checksum: for each duplicate file, the query finds
// the earliest non-duplicate complete file with the same checksum.
func (db *DB) DuplicatePairs() ([]*DuplicatePair, error) {
	const q = `
		SELECT dup.source_path, dup.dest_rel, orig.dest_rel
		FROM files dup
		JOIN files orig
		  ON orig.checksum = dup.checksum
		 AND orig.is_duplicate = 0
		 AND orig.status = 'complete'
		WHERE dup.is_duplicate = 1
		  AND dup.checksum IS NOT NULL
		ORDER BY dup.id`

	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("archivedb: duplicate pairs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pairs []*DuplicatePair
	for rows.Next() {
		var p DuplicatePair
		var dupDest, origDest sql.NullString
		if err := rows.Scan(&p.DuplicateSource, &dupDest, &origDest); err != nil {
			return nil, fmt.Errorf("archivedb: scan duplicate pair: %w", err)
		}
		p.DuplicateDest = dupDest.String
		p.OriginalDest = origDest.String
		pairs = append(pairs, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archivedb: duplicate pairs iterate: %w", err)
	}
	return pairs, nil
}

// ArchiveInventory returns all completed, non-duplicate files — the canonical
// archive contents. Results are ordered by dest_rel for stable output.
func (db *DB) ArchiveInventory() ([]*InventoryEntry, error) {
	const q = `
		SELECT dest_rel, checksum, capture_date
		FROM files
		WHERE status = 'complete'
		  AND is_duplicate = 0
		ORDER BY dest_rel`

	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("archivedb: archive inventory: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []*InventoryEntry
	for rows.Next() {
		e, err := scanInventoryEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archivedb: archive inventory iterate: %w", err)
	}
	return entries, nil
}

// AllSkipped returns all files with status "skipped" across all runs.
// Results are ordered by insertion order (id ASC).
func (db *DB) AllSkipped() ([]*FileRecord, error) {
	const q = `
		SELECT id, run_id, source_path, dest_path, dest_rel, checksum,
		       status, is_duplicate, capture_date, file_size,
		       extracted_at, hashed_at, copied_at, verified_at, tagged_at, error,
		       skip_reason, carried_sidecars
		FROM files
		WHERE status = 'skipped'
		ORDER BY id`

	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("archivedb: all skipped: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFileRows(rows)
}

// GetRunByPrefix returns all runs whose ID starts with the given prefix,
// ordered by started_at descending. The caller is responsible for handling
// the ambiguous-prefix case (len > 1) and the not-found case (len == 0).
func (db *DB) GetRunByPrefix(prefix string) ([]*Run, error) {
	const q = `
		SELECT id, pixe_version, source, destination, algorithm, workers,
		       recursive, started_at, finished_at, status
		FROM runs
		WHERE id LIKE ?
		ORDER BY started_at DESC`

	rows, err := db.conn.Query(q, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("archivedb: get run by prefix: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []*Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		if r != nil {
			runs = append(runs, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archivedb: get run by prefix iterate: %w", err)
	}
	return runs, nil
}

// ArchiveStats returns aggregate statistics for the entire archive database.
// It executes two queries: one for file-level aggregates and one for the run count.
func (db *DB) ArchiveStats() (*ArchiveStats, error) {
	const fileQ = `
		SELECT
		    COUNT(*),
		    COALESCE(SUM(CASE WHEN status = 'complete' AND is_duplicate = 0 THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN is_duplicate = 1 THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN status = 'mismatch' THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN status = 'tag_failed' THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END), 0),
		    COALESCE(SUM(file_size), 0),
		    MIN(capture_date),
		    MAX(capture_date)
		FROM files`

	var s ArchiveStats
	var earliestStr, latestStr sql.NullString

	if err := db.conn.QueryRow(fileQ).Scan(
		&s.TotalFiles,
		&s.Complete,
		&s.Duplicates,
		&s.Failed,
		&s.Mismatches,
		&s.TagFailed,
		&s.Skipped,
		&s.TotalSize,
		&earliestStr,
		&latestStr,
	); err != nil {
		return nil, fmt.Errorf("archivedb: archive stats file query: %w", err)
	}

	if earliestStr.Valid && earliestStr.String != "" {
		t, err := time.Parse(time.RFC3339, earliestStr.String)
		if err != nil {
			return nil, fmt.Errorf("archivedb: archive stats parse earliest_capture: %w", err)
		}
		s.EarliestCapture = &t
	}
	if latestStr.Valid && latestStr.String != "" {
		t, err := time.Parse(time.RFC3339, latestStr.String)
		if err != nil {
			return nil, fmt.Errorf("archivedb: archive stats parse latest_capture: %w", err)
		}
		s.LatestCapture = &t
	}

	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&s.RunCount); err != nil {
		return nil, fmt.Errorf("archivedb: archive stats run count: %w", err)
	}

	return &s, nil
}

// FormatBreakdown returns file counts grouped by lowercase extension for
// complete, non-duplicate files. The result maps extension (e.g., ".jpg")
// to count, ordered by count descending.
func (db *DB) FormatBreakdown() ([]FormatCount, error) {
	// INSTR finds the first dot in dest_rel. Pathbuilder-generated filenames
	// have the form YYYY/MM-Mon/YYYYMMDD_HHMMSS_<hex>.<ext> — exactly one dot
	// per filename — so INSTR gives the correct extension in all real cases.
	const q = `
		SELECT LOWER(SUBSTR(dest_rel, INSTR(dest_rel, '.'))) AS ext,
		       COUNT(*) AS cnt
		FROM files
		WHERE status = 'complete' AND is_duplicate = 0
		  AND dest_rel IS NOT NULL AND INSTR(dest_rel, '.') > 0
		GROUP BY ext
		ORDER BY cnt DESC`

	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, fmt.Errorf("archivedb: format breakdown: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []FormatCount
	for rows.Next() {
		var fc FormatCount
		if err := rows.Scan(&fc.Extension, &fc.Count); err != nil {
			return nil, fmt.Errorf("archivedb: format breakdown scan: %w", err)
		}
		result = append(result, fc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archivedb: format breakdown iterate: %w", err)
	}
	return result, nil
}

// LastRunDate returns the finish time of the most recently completed run,
// or nil if no completed runs exist.
func (db *DB) LastRunDate() (*time.Time, error) {
	const q = `SELECT MAX(finished_at) FROM runs WHERE status = 'completed'`
	var s sql.NullString
	if err := db.conn.QueryRow(q).Scan(&s); err != nil {
		return nil, fmt.Errorf("archivedb: last run date: %w", err)
	}
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s.String)
	if err != nil {
		return nil, fmt.Errorf("archivedb: last run date parse: %w", err)
	}
	return &t, nil
}

// CheckSourceProcessed returns true if a file with the given absolute source
// path has already been successfully processed (status 'complete' or
// 'duplicate') in any prior run. Used by the pipeline to decide whether to
// emit a SKIP line instead of re-processing the file.
//
// Note: 'skipped' and 'failed' statuses are intentionally excluded — a
// previously-skipped file (e.g. unsupported format) should be re-evaluated
// in case a new handler has been registered, and a previously-failed file
// should be retried.
func (db *DB) CheckSourceProcessed(sourcePath string) (bool, error) {
	const q = `
		SELECT 1 FROM files
		WHERE source_path = ?
		  AND status IN ('complete', 'duplicate')
		LIMIT 1`

	var exists int
	err := db.conn.QueryRow(q, sourcePath).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("archivedb: check source processed: %w", err)
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Internal scan helpers
// ---------------------------------------------------------------------------

// scanRunSummary scans a single RunSummary from a row scanner.
func scanRunSummary(rows *sql.Rows) (*RunSummary, error) {
	var s RunSummary
	var startedAtStr string
	var finishedAtStr sql.NullString

	if err := rows.Scan(
		&s.ID,
		&s.PixeVersion,
		&s.Source,
		&startedAtStr,
		&finishedAtStr,
		&s.Status,
		&s.FileCount,
	); err != nil {
		return nil, fmt.Errorf("archivedb: scan run summary: %w", err)
	}

	var err error
	s.StartedAt, err = time.Parse(time.RFC3339, startedAtStr)
	if err != nil {
		return nil, fmt.Errorf("archivedb: parse run summary started_at: %w", err)
	}

	if finishedAtStr.Valid {
		t, err := time.Parse(time.RFC3339, finishedAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("archivedb: parse run summary finished_at: %w", err)
		}
		s.FinishedAt = &t
	}

	return &s, nil
}

// extraScanner wraps *sql.Rows and appends extra destination pointers to every
// Scan call. This lets scanFileRow (which scans the 18 standard file columns)
// be reused when the query returns additional columns after the standard ones.
type extraScanner struct {
	rows  *sql.Rows
	extra []any
}

func (es *extraScanner) Scan(dest ...any) error {
	return es.rows.Scan(append(dest, es.extra...)...)
}

// scanFileWithSource scans a FileRecord plus the run_source column.
// It delegates the 18 standard file columns to scanFileRow via extraScanner,
// eliminating the ~90 lines of duplicated scan logic.
func scanFileWithSource(rows *sql.Rows) (*FileWithSource, error) {
	var fws FileWithSource
	es := &extraScanner{rows: rows, extra: []any{&fws.RunSource}}
	fr, err := scanFileRow(es)
	if err != nil {
		return nil, fmt.Errorf("archivedb: scan file with source: %w", err)
	}
	fws.FileRecord = *fr
	return &fws, nil
}

// scanInventoryEntry scans a single InventoryEntry from a row scanner.
func scanInventoryEntry(rows *sql.Rows) (*InventoryEntry, error) {
	var e InventoryEntry
	var destRel, checksum sql.NullString
	var captureDate sql.NullString

	if err := rows.Scan(&destRel, &checksum, &captureDate); err != nil {
		return nil, fmt.Errorf("archivedb: scan inventory entry: %w", err)
	}

	e.DestRel = destRel.String
	e.Checksum = checksum.String

	if captureDate.Valid {
		t, err := time.Parse(time.RFC3339, captureDate.String)
		if err != nil {
			return nil, fmt.Errorf("archivedb: parse inventory capture_date: %w", err)
		}
		e.CaptureDate = &t
	}

	return &e, nil
}

// Vacuum rebuilds the database file, reclaiming space from deleted rows
// and reducing fragmentation. Requires exclusive access — no concurrent
// readers or writers should be active.
func (db *DB) Vacuum() error {
	_, err := db.conn.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("archivedb: vacuum: %w", err)
	}
	return nil
}

// HasActiveRuns returns true if any run has status = 'running'.
// Used by pixe clean to guard against vacuuming while a sort is in progress.
func (db *DB) HasActiveRuns() (bool, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM runs WHERE status = 'running'`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("archivedb: check active runs: %w", err)
	}
	return count > 0, nil
}
