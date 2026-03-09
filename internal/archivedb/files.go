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
	"strings"
	"time"
)

// FileRecord represents a row in the files table.
type FileRecord struct {
	ID          int64
	RunID       string
	SourcePath  string
	DestPath    *string // nil until copied
	DestRel     *string // nil until copied
	Checksum    *string // nil until hashed
	Status      string
	IsDuplicate bool
	CaptureDate *time.Time
	FileSize    *int64
	ExtractedAt *time.Time
	HashedAt    *time.Time
	CopiedAt    *time.Time
	VerifiedAt  *time.Time
	TaggedAt    *time.Time
	Error       *string
	SkipReason  *string // non-nil when status = "skipped"
}

// updateParams holds the optional fields that can be set during a status update.
type updateParams struct {
	checksum    *string
	destPath    *string
	destRel     *string
	captureDate *time.Time
	fileSize    *int64
	errMsg      *string
	isDuplicate *bool
	skipReason  *string
}

// UpdateOption configures optional fields on a file status update.
type UpdateOption func(*updateParams)

// WithChecksum sets the checksum field on a file status update.
func WithChecksum(checksum string) UpdateOption {
	return func(p *updateParams) { p.checksum = &checksum }
}

// WithDestination sets the dest_path and dest_rel fields on a file status update.
func WithDestination(destPath, destRel string) UpdateOption {
	return func(p *updateParams) {
		p.destPath = &destPath
		p.destRel = &destRel
	}
}

// WithCaptureDate sets the capture_date field on a file status update.
func WithCaptureDate(t time.Time) UpdateOption {
	return func(p *updateParams) { p.captureDate = &t }
}

// WithFileSize sets the file_size field on a file status update.
func WithFileSize(size int64) UpdateOption {
	return func(p *updateParams) { p.fileSize = &size }
}

// WithError sets the error field on a file status update.
func WithError(msg string) UpdateOption {
	return func(p *updateParams) { p.errMsg = &msg }
}

// WithIsDuplicate sets the is_duplicate field on a file status update.
func WithIsDuplicate(dup bool) UpdateOption {
	return func(p *updateParams) { p.isDuplicate = &dup }
}

// WithSkipReason sets the skip_reason field on a file status update.
func WithSkipReason(reason string) UpdateOption {
	return func(p *updateParams) { p.skipReason = &reason }
}

// InsertFile creates a new file record with status "pending".
// Returns the auto-generated ID.
func (db *DB) InsertFile(f *FileRecord) (int64, error) {
	const q = `
		INSERT INTO files (run_id, source_path, status)
		VALUES (?, ?, 'pending')`

	res, err := db.conn.Exec(q, f.RunID, f.SourcePath)
	if err != nil {
		return 0, fmt.Errorf("archivedb: insert file: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("archivedb: insert file last id: %w", err)
	}
	return id, nil
}

// InsertFiles batch-inserts multiple file records within a single transaction.
// Returns the IDs of the inserted records in the same order as the input slice.
func (db *DB) InsertFiles(files []*FileRecord) ([]int64, error) {
	if len(files) == 0 {
		return nil, nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("archivedb: begin batch insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`INSERT INTO files (run_id, source_path, status) VALUES (?, ?, 'pending')`)
	if err != nil {
		return nil, fmt.Errorf("archivedb: prepare batch insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	ids := make([]int64, len(files))
	for i, f := range files {
		res, err := stmt.Exec(f.RunID, f.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("archivedb: batch insert file %d: %w", i, err)
		}
		ids[i], err = res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("archivedb: batch insert last id %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("archivedb: commit batch insert: %w", err)
	}
	return ids, nil
}

// UpdateFileStatus updates a file's status and the corresponding timestamp.
// The timestamp column set is determined by the status value:
//   - "extracted"  → extracted_at
//   - "hashed"     → hashed_at  (also sets checksum via WithChecksum)
//   - "copied"     → copied_at  (also sets dest_path/dest_rel via WithDestination)
//   - "verified"   → verified_at
//   - "tagged"     → tagged_at
//   - "complete"   → (no additional timestamp)
//   - "failed" / "mismatch" / "tag_failed" / "duplicate" → sets error via WithError
//
// Each call is wrapped in its own implicit transaction (single-statement autocommit).
func (db *DB) UpdateFileStatus(fileID int64, status string, opts ...UpdateOption) error {
	p := &updateParams{}
	for _, o := range opts {
		o(p)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Build the SET clause dynamically.
	setClauses := []string{"status = ?"}
	args := []any{status}

	// Status-driven timestamp column.
	switch status {
	case "extracted":
		setClauses = append(setClauses, "extracted_at = ?")
		args = append(args, now)
	case "hashed":
		setClauses = append(setClauses, "hashed_at = ?")
		args = append(args, now)
	case "copied":
		setClauses = append(setClauses, "copied_at = ?")
		args = append(args, now)
	case "verified":
		setClauses = append(setClauses, "verified_at = ?")
		args = append(args, now)
	case "tagged":
		setClauses = append(setClauses, "tagged_at = ?")
		args = append(args, now)
	}

	// Optional field updates.
	if p.checksum != nil {
		setClauses = append(setClauses, "checksum = ?")
		args = append(args, *p.checksum)
	}
	if p.destPath != nil {
		setClauses = append(setClauses, "dest_path = ?")
		args = append(args, *p.destPath)
	}
	if p.destRel != nil {
		setClauses = append(setClauses, "dest_rel = ?")
		args = append(args, *p.destRel)
	}
	if p.captureDate != nil {
		setClauses = append(setClauses, "capture_date = ?")
		args = append(args, p.captureDate.UTC().Format(time.RFC3339))
	}
	if p.fileSize != nil {
		setClauses = append(setClauses, "file_size = ?")
		args = append(args, *p.fileSize)
	}
	if p.errMsg != nil {
		setClauses = append(setClauses, "error = ?")
		args = append(args, *p.errMsg)
	}
	if p.isDuplicate != nil {
		val := 0
		if *p.isDuplicate {
			val = 1
		}
		setClauses = append(setClauses, "is_duplicate = ?")
		args = append(args, val)
	}
	if p.skipReason != nil {
		setClauses = append(setClauses, "skip_reason = ?")
		args = append(args, *p.skipReason)
	}

	args = append(args, fileID)
	q := fmt.Sprintf("UPDATE files SET %s WHERE id = ?", strings.Join(setClauses, ", "))

	res, err := db.conn.Exec(q, args...)
	if err != nil {
		return fmt.Errorf("archivedb: update file status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("archivedb: update file rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("archivedb: update file status: file %d not found", fileID)
	}
	return nil
}

// CompleteFileWithDedupCheck atomically checks for an existing completed file
// with the same checksum and marks this file as complete within a single
// transaction. This prevents the TOCTOU race where two concurrent processes
// both believe they are the first to complete a given checksum.
//
// Returns:
//   - existingDest: the dest_rel of the already-completed file if a duplicate
//     was detected; empty string if this file is the first.
//   - err: any database error.
//
// When existingDest is non-empty the caller must physically move the copied
// file to the duplicates directory and update the DB record accordingly.
func (db *DB) CompleteFileWithDedupCheck(fileID int64, checksum string) (existingDest string, err error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return "", fmt.Errorf("archivedb: complete with dedup check begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Check for an existing completed file with the same checksum (excluding self).
	const selectQ = `
		SELECT dest_rel FROM files
		WHERE checksum = ? AND status = 'complete' AND id != ?
		LIMIT 1`

	var destRel sql.NullString
	scanErr := tx.QueryRow(selectQ, checksum, fileID).Scan(&destRel)
	if scanErr != nil && scanErr != sql.ErrNoRows {
		return "", fmt.Errorf("archivedb: complete with dedup check query: %w", scanErr)
	}

	if scanErr == nil {
		// A completed file with the same checksum exists (duplicate detected).
		// Mark this file as complete with is_duplicate=1.
		// The caller is responsible for relocating the physical file.
		const updateDupQ = `UPDATE files SET status = 'complete', is_duplicate = 1 WHERE id = ?`
		if _, err := tx.Exec(updateDupQ, fileID); err != nil {
			return "", fmt.Errorf("archivedb: complete duplicate file: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("archivedb: complete duplicate commit: %w", err)
		}
		// Return dest_rel if available; otherwise return a non-empty sentinel so
		// the caller knows a duplicate was detected even when dest_rel is NULL.
		if destRel.Valid && destRel.String != "" {
			return destRel.String, nil
		}
		return "<duplicate>", nil
	}

	// No duplicate — mark as complete (non-duplicate).
	const updateQ = `UPDATE files SET status = 'complete' WHERE id = ?`
	if _, err := tx.Exec(updateQ, fileID); err != nil {
		return "", fmt.Errorf("archivedb: complete file: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("archivedb: complete commit: %w", err)
	}
	return "", nil
}

// CheckDuplicate queries whether a file with the given checksum exists with
// status "complete". Returns the dest_rel path if found, empty string if not.
// This is the hot-path dedup query — served by idx_files_checksum.
func (db *DB) CheckDuplicate(checksum string) (string, error) {
	const q = `
		SELECT dest_rel FROM files
		WHERE checksum = ? AND status = 'complete'
		LIMIT 1`

	var destRel sql.NullString
	err := db.conn.QueryRow(q, checksum).Scan(&destRel)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("archivedb: check duplicate: %w", err)
	}
	return destRel.String, nil
}

// GetFilesByRun returns all file records for a given run ID.
func (db *DB) GetFilesByRun(runID string) ([]*FileRecord, error) {
	const q = `
		SELECT id, run_id, source_path, dest_path, dest_rel, checksum,
		       status, is_duplicate, capture_date, file_size,
		       extracted_at, hashed_at, copied_at, verified_at, tagged_at, error,
		       skip_reason
		FROM files WHERE run_id = ?
		ORDER BY id`

	rows, err := db.conn.Query(q, runID)
	if err != nil {
		return nil, fmt.Errorf("archivedb: get files by run: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFileRows(rows)
}

// GetIncompleteFiles returns all files for a run that are not in a terminal state.
// Terminal states are: "complete", "failed", "mismatch", "tag_failed", "duplicate", "skipped".
// Used by resume to find files that need reprocessing.
func (db *DB) GetIncompleteFiles(runID string) ([]*FileRecord, error) {
	const q = `
		SELECT id, run_id, source_path, dest_path, dest_rel, checksum,
		       status, is_duplicate, capture_date, file_size,
		       extracted_at, hashed_at, copied_at, verified_at, tagged_at, error,
		       skip_reason
		FROM files
		WHERE run_id = ?
		  AND status NOT IN ('complete', 'failed', 'mismatch', 'tag_failed', 'duplicate', 'skipped')
		ORDER BY id`

	rows, err := db.conn.Query(q, runID)
	if err != nil {
		return nil, fmt.Errorf("archivedb: get incomplete files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanFileRows(rows)
}

// scanFileRows scans all rows from a files query into a []*FileRecord slice.
func scanFileRows(rows *sql.Rows) ([]*FileRecord, error) {
	var records []*FileRecord
	for rows.Next() {
		f, err := scanFileRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("archivedb: scan file rows: %w", err)
	}
	return records, nil
}

// scanFileRow scans a single FileRecord from a row scanner.
func scanFileRow(s scanner) (*FileRecord, error) {
	var f FileRecord
	var destPath, destRel, checksum, captureDate sql.NullString
	var fileSize sql.NullInt64
	var extractedAt, hashedAt, copiedAt, verifiedAt, taggedAt sql.NullString
	var errMsg, skipReason sql.NullString
	var isDupInt int

	err := s.Scan(
		&f.ID,
		&f.RunID,
		&f.SourcePath,
		&destPath,
		&destRel,
		&checksum,
		&f.Status,
		&isDupInt,
		&captureDate,
		&fileSize,
		&extractedAt,
		&hashedAt,
		&copiedAt,
		&verifiedAt,
		&taggedAt,
		&errMsg,
		&skipReason,
	)
	if err != nil {
		return nil, fmt.Errorf("archivedb: scan file row: %w", err)
	}

	f.IsDuplicate = isDupInt != 0

	if destPath.Valid {
		f.DestPath = &destPath.String
	}
	if destRel.Valid {
		f.DestRel = &destRel.String
	}
	if checksum.Valid {
		f.Checksum = &checksum.String
	}
	if fileSize.Valid {
		f.FileSize = &fileSize.Int64
	}
	if errMsg.Valid {
		f.Error = &errMsg.String
	}
	if skipReason.Valid {
		f.SkipReason = &skipReason.String
	}

	parseOptTime := func(ns sql.NullString, fieldName string) (*time.Time, error) {
		if !ns.Valid {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339, ns.String)
		if err != nil {
			return nil, fmt.Errorf("archivedb: parse %s: %w", fieldName, err)
		}
		return &t, nil
	}

	if f.CaptureDate, err = parseOptTime(captureDate, "capture_date"); err != nil {
		return nil, err
	}
	if f.ExtractedAt, err = parseOptTime(extractedAt, "extracted_at"); err != nil {
		return nil, err
	}
	if f.HashedAt, err = parseOptTime(hashedAt, "hashed_at"); err != nil {
		return nil, err
	}
	if f.CopiedAt, err = parseOptTime(copiedAt, "copied_at"); err != nil {
		return nil, err
	}
	if f.VerifiedAt, err = parseOptTime(verifiedAt, "verified_at"); err != nil {
		return nil, err
	}
	if f.TaggedAt, err = parseOptTime(taggedAt, "tagged_at"); err != nil {
		return nil, err
	}

	return &f, nil
}
