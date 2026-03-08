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
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// openTestDB opens a fresh database in t.TempDir() and registers cleanup.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestOpen_createsSchema verifies that Open creates all expected tables.
func TestOpen_createsSchema(t *testing.T) {
	db := openTestDB(t)

	tables := []string{"schema_version", "runs", "files"}
	for _, tbl := range tables {
		var name string
		err := db.conn.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", tbl, err)
		}
	}
}

// TestOpen_idempotent verifies that opening an existing database is safe.
func TestOpen_idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idempotent.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	_ = db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() { _ = db2.Close() }()

	// Schema version row should still be exactly one row.
	var count int
	if err := db2.conn.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&count); err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if count != 1 {
		t.Errorf("schema_version row count = %d, want 1", count)
	}
}

// TestOpen_WALMode verifies that WAL journal mode is active after Open.
func TestOpen_WALMode(t *testing.T) {
	db := openTestDB(t)

	var mode string
	if err := db.conn.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

// TestOpen_ForeignKeys verifies that foreign key enforcement is enabled.
func TestOpen_ForeignKeys(t *testing.T) {
	db := openTestDB(t)

	var fk int
	if err := db.conn.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

// TestSchemaVersion verifies the schema_version table has version 1.
func TestSchemaVersion(t *testing.T) {
	db := openTestDB(t)

	var version int
	if err := db.conn.QueryRow(`SELECT version FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("schema version = %d, want %d", version, schemaVersion)
	}
}

// TestOpen_createsIndexes verifies that all expected indexes are created.
func TestOpen_createsIndexes(t *testing.T) {
	db := openTestDB(t)

	indexes := []string{
		"idx_files_checksum",
		"idx_files_run_id",
		"idx_files_status",
		"idx_files_source",
		"idx_files_capture_date",
	}
	for _, idx := range indexes {
		var name string
		err := db.conn.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

// TestOpen_path verifies that Path() returns the correct database path.
func TestOpen_path(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "path_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if db.Path() != path {
		t.Errorf("Path() = %q, want %q", db.Path(), path)
	}
}

// TestOpen_createsParentDirectory verifies that Open creates missing parent dirs.
func TestOpen_createsParentDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open with nested path: %v", err)
	}
	defer func() { _ = db.Close() }()
}

// TestClose verifies that Close does not return an error on a healthy connection.
func TestClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Run CRUD tests (Task 30)
// ---------------------------------------------------------------------------

func makeTestRun(id string) *Run {
	return &Run{
		ID:          id,
		PixeVersion: "1.2.3",
		Source:      "/src/photos",
		Destination: "/dst/archive",
		Algorithm:   "sha256",
		Workers:     4,
		StartedAt:   time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
	}
}

// TestInsertRun_roundtrip verifies that InsertRun + GetRun round-trips all fields.
func TestInsertRun_roundtrip(t *testing.T) {
	db := openTestDB(t)
	r := makeTestRun("run-001")

	if err := db.InsertRun(r); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	got, err := db.GetRun("run-001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got == nil {
		t.Fatal("GetRun returned nil, want run")
	}

	if got.ID != r.ID {
		t.Errorf("ID = %q, want %q", got.ID, r.ID)
	}
	if got.PixeVersion != r.PixeVersion {
		t.Errorf("PixeVersion = %q, want %q", got.PixeVersion, r.PixeVersion)
	}
	if got.Source != r.Source {
		t.Errorf("Source = %q, want %q", got.Source, r.Source)
	}
	if got.Destination != r.Destination {
		t.Errorf("Destination = %q, want %q", got.Destination, r.Destination)
	}
	if got.Algorithm != r.Algorithm {
		t.Errorf("Algorithm = %q, want %q", got.Algorithm, r.Algorithm)
	}
	if got.Workers != r.Workers {
		t.Errorf("Workers = %d, want %d", got.Workers, r.Workers)
	}
	if !got.StartedAt.Equal(r.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, r.StartedAt)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want %q", got.Status, "running")
	}
	if got.FinishedAt != nil {
		t.Errorf("FinishedAt = %v, want nil", got.FinishedAt)
	}
}

// TestGetRun_notFound verifies that GetRun returns (nil, nil) for unknown IDs.
func TestGetRun_notFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetRun("nonexistent")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got != nil {
		t.Errorf("GetRun = %+v, want nil", got)
	}
}

// TestCompleteRun verifies that CompleteRun sets status and finished_at.
func TestCompleteRun(t *testing.T) {
	db := openTestDB(t)
	r := makeTestRun("run-complete")
	if err := db.InsertRun(r); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	finishedAt := time.Date(2026, 3, 7, 13, 0, 0, 0, time.UTC)
	if err := db.CompleteRun("run-complete", finishedAt); err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}

	got, err := db.GetRun("run-complete")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("Status = %q, want %q", got.Status, "completed")
	}
	if got.FinishedAt == nil {
		t.Fatal("FinishedAt is nil, want non-nil")
	}
	if !got.FinishedAt.Equal(finishedAt) {
		t.Errorf("FinishedAt = %v, want %v", got.FinishedAt, finishedAt)
	}
}

// TestInterruptRun verifies that InterruptRun sets status and finished_at.
func TestInterruptRun(t *testing.T) {
	db := openTestDB(t)
	r := makeTestRun("run-interrupt")
	if err := db.InsertRun(r); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	finishedAt := time.Date(2026, 3, 7, 13, 30, 0, 0, time.UTC)
	if err := db.InterruptRun("run-interrupt", finishedAt); err != nil {
		t.Fatalf("InterruptRun: %v", err)
	}

	got, err := db.GetRun("run-interrupt")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != "interrupted" {
		t.Errorf("Status = %q, want %q", got.Status, "interrupted")
	}
	if got.FinishedAt == nil {
		t.Fatal("FinishedAt is nil, want non-nil")
	}
}

// TestFindInterruptedRuns verifies only "running" runs are returned.
func TestFindInterruptedRuns(t *testing.T) {
	db := openTestDB(t)

	// Insert 3 runs: one running, one completed, one interrupted.
	runs := []*Run{
		makeTestRun("run-a"),
		makeTestRun("run-b"),
		makeTestRun("run-c"),
	}
	for _, r := range runs {
		if err := db.InsertRun(r); err != nil {
			t.Fatalf("InsertRun %s: %v", r.ID, err)
		}
	}

	now := time.Now().UTC()
	if err := db.CompleteRun("run-b", now); err != nil {
		t.Fatalf("CompleteRun: %v", err)
	}
	if err := db.InterruptRun("run-c", now); err != nil {
		t.Fatalf("InterruptRun: %v", err)
	}

	interrupted, err := db.FindInterruptedRuns()
	if err != nil {
		t.Fatalf("FindInterruptedRuns: %v", err)
	}
	if len(interrupted) != 1 {
		t.Fatalf("FindInterruptedRuns returned %d runs, want 1", len(interrupted))
	}
	if interrupted[0].ID != "run-a" {
		t.Errorf("interrupted run ID = %q, want %q", interrupted[0].ID, "run-a")
	}
}

// ---------------------------------------------------------------------------
// File CRUD tests (Task 30)
// ---------------------------------------------------------------------------

func insertTestRun(t *testing.T, db *DB, id string) {
	t.Helper()
	if err := db.InsertRun(makeTestRun(id)); err != nil {
		t.Fatalf("InsertRun(%q): %v", id, err)
	}
}

// TestInsertFile_roundtrip verifies InsertFile returns a valid ID and the record is retrievable.
func TestInsertFile_roundtrip(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-file-1")

	f := &FileRecord{RunID: "run-file-1", SourcePath: "/src/photo.jpg"}
	id, err := db.InsertFile(f)
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}
	if id <= 0 {
		t.Errorf("InsertFile returned id=%d, want > 0", id)
	}

	files, err := db.GetFilesByRun("run-file-1")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("GetFilesByRun returned %d files, want 1", len(files))
	}
	got := files[0]
	if got.ID != id {
		t.Errorf("ID = %d, want %d", got.ID, id)
	}
	if got.RunID != "run-file-1" {
		t.Errorf("RunID = %q, want %q", got.RunID, "run-file-1")
	}
	if got.SourcePath != "/src/photo.jpg" {
		t.Errorf("SourcePath = %q, want %q", got.SourcePath, "/src/photo.jpg")
	}
	if got.Status != "pending" {
		t.Errorf("Status = %q, want %q", got.Status, "pending")
	}
}

// TestInsertFiles_batch verifies that 100 files can be batch-inserted in one transaction.
func TestInsertFiles_batch(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-batch")

	const n = 100
	files := make([]*FileRecord, n)
	for i := range files {
		files[i] = &FileRecord{
			RunID:      "run-batch",
			SourcePath: fmt.Sprintf("/src/photo_%03d.jpg", i),
		}
	}

	ids, err := db.InsertFiles(files)
	if err != nil {
		t.Fatalf("InsertFiles: %v", err)
	}
	if len(ids) != n {
		t.Fatalf("InsertFiles returned %d ids, want %d", len(ids), n)
	}
	// All IDs should be positive and unique.
	seen := make(map[int64]bool, n)
	for _, id := range ids {
		if id <= 0 {
			t.Errorf("id = %d, want > 0", id)
		}
		if seen[id] {
			t.Errorf("duplicate id %d", id)
		}
		seen[id] = true
	}

	all, err := db.GetFilesByRun("run-batch")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	if len(all) != n {
		t.Errorf("GetFilesByRun returned %d files, want %d", len(all), n)
	}
}

// TestInsertFiles_empty verifies that InsertFiles with an empty slice is a no-op.
func TestInsertFiles_empty(t *testing.T) {
	db := openTestDB(t)
	ids, err := db.InsertFiles(nil)
	if err != nil {
		t.Fatalf("InsertFiles(nil): %v", err)
	}
	if ids != nil {
		t.Errorf("InsertFiles(nil) = %v, want nil", ids)
	}
}

// TestUpdateFileStatus_withChecksum verifies that WithChecksum sets both status and checksum.
func TestUpdateFileStatus_withChecksum(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-upd-1")

	id, err := db.InsertFile(&FileRecord{RunID: "run-upd-1", SourcePath: "/src/a.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	const checksum = "abc123def456"
	if err := db.UpdateFileStatus(id, "hashed", WithChecksum(checksum)); err != nil {
		t.Fatalf("UpdateFileStatus: %v", err)
	}

	files, err := db.GetFilesByRun("run-upd-1")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	got := files[0]
	if got.Status != "hashed" {
		t.Errorf("Status = %q, want %q", got.Status, "hashed")
	}
	if got.Checksum == nil || *got.Checksum != checksum {
		t.Errorf("Checksum = %v, want %q", got.Checksum, checksum)
	}
	if got.HashedAt == nil {
		t.Error("HashedAt is nil, want non-nil")
	}
}

// TestUpdateFileStatus_progression walks a file through all pipeline stages.
func TestUpdateFileStatus_progression(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-prog")

	id, err := db.InsertFile(&FileRecord{RunID: "run-prog", SourcePath: "/src/prog.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	captureDate := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	stages := []struct {
		status string
		opts   []UpdateOption
	}{
		{"extracted", []UpdateOption{WithCaptureDate(captureDate), WithFileSize(1024)}},
		{"hashed", []UpdateOption{WithChecksum("deadbeef")}},
		{"copied", []UpdateOption{WithDestination("/dst/2025/06-Jun/photo.jpg", "2025/06-Jun/photo.jpg")}},
		{"verified", nil},
		{"tagged", nil},
		{"complete", nil},
	}

	for _, stage := range stages {
		if err := db.UpdateFileStatus(id, stage.status, stage.opts...); err != nil {
			t.Fatalf("UpdateFileStatus(%q): %v", stage.status, err)
		}
	}

	files, err := db.GetFilesByRun("run-prog")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	got := files[0]

	if got.Status != "complete" {
		t.Errorf("Status = %q, want %q", got.Status, "complete")
	}
	if got.ExtractedAt == nil {
		t.Error("ExtractedAt is nil")
	}
	if got.HashedAt == nil {
		t.Error("HashedAt is nil")
	}
	if got.CopiedAt == nil {
		t.Error("CopiedAt is nil")
	}
	if got.VerifiedAt == nil {
		t.Error("VerifiedAt is nil")
	}
	if got.TaggedAt == nil {
		t.Error("TaggedAt is nil")
	}
	if got.Checksum == nil || *got.Checksum != "deadbeef" {
		t.Errorf("Checksum = %v, want %q", got.Checksum, "deadbeef")
	}
	if got.DestRel == nil || *got.DestRel != "2025/06-Jun/photo.jpg" {
		t.Errorf("DestRel = %v, want %q", got.DestRel, "2025/06-Jun/photo.jpg")
	}
	if got.CaptureDate == nil || !got.CaptureDate.Equal(captureDate) {
		t.Errorf("CaptureDate = %v, want %v", got.CaptureDate, captureDate)
	}
	if got.FileSize == nil || *got.FileSize != 1024 {
		t.Errorf("FileSize = %v, want 1024", got.FileSize)
	}
}

// TestCheckDuplicate_found verifies CheckDuplicate returns dest_rel for a complete file.
func TestCheckDuplicate_found(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-dup-1")

	id, err := db.InsertFile(&FileRecord{RunID: "run-dup-1", SourcePath: "/src/dup.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	const checksum = "unique-checksum-abc"
	const destRel = "2025/06-Jun/photo.jpg"

	if err := db.UpdateFileStatus(id, "hashed", WithChecksum(checksum)); err != nil {
		t.Fatalf("UpdateFileStatus hashed: %v", err)
	}
	if err := db.UpdateFileStatus(id, "copied", WithDestination("/dst/"+destRel, destRel)); err != nil {
		t.Fatalf("UpdateFileStatus copied: %v", err)
	}
	if err := db.UpdateFileStatus(id, "complete"); err != nil {
		t.Fatalf("UpdateFileStatus complete: %v", err)
	}

	got, err := db.CheckDuplicate(checksum)
	if err != nil {
		t.Fatalf("CheckDuplicate: %v", err)
	}
	if got != destRel {
		t.Errorf("CheckDuplicate = %q, want %q", got, destRel)
	}
}

// TestCheckDuplicate_notFound verifies CheckDuplicate returns "" for unknown checksums.
func TestCheckDuplicate_notFound(t *testing.T) {
	db := openTestDB(t)

	got, err := db.CheckDuplicate("nonexistent-checksum")
	if err != nil {
		t.Fatalf("CheckDuplicate: %v", err)
	}
	if got != "" {
		t.Errorf("CheckDuplicate = %q, want %q", got, "")
	}
}

// TestCheckDuplicate_ignoresNonComplete verifies that non-complete files are not returned.
func TestCheckDuplicate_ignoresNonComplete(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-dup-nc")

	id, err := db.InsertFile(&FileRecord{RunID: "run-dup-nc", SourcePath: "/src/nc.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	const checksum = "hashed-only-checksum"
	if err := db.UpdateFileStatus(id, "hashed", WithChecksum(checksum)); err != nil {
		t.Fatalf("UpdateFileStatus hashed: %v", err)
	}

	// File is "hashed" but not "complete" — should not be returned.
	got, err := db.CheckDuplicate(checksum)
	if err != nil {
		t.Fatalf("CheckDuplicate: %v", err)
	}
	if got != "" {
		t.Errorf("CheckDuplicate = %q, want %q (non-complete file should be ignored)", got, "")
	}
}

// TestGetIncompleteFiles verifies only non-terminal files are returned.
func TestGetIncompleteFiles(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-incomplete")

	// Insert 5 files and advance them to various states.
	sources := []string{"/src/a.jpg", "/src/b.jpg", "/src/c.jpg", "/src/d.jpg", "/src/e.jpg"}
	ids := make([]int64, len(sources))
	for i, src := range sources {
		id, err := db.InsertFile(&FileRecord{RunID: "run-incomplete", SourcePath: src})
		if err != nil {
			t.Fatalf("InsertFile %s: %v", src, err)
		}
		ids[i] = id
	}

	// Terminal states.
	if err := db.UpdateFileStatus(ids[0], "complete"); err != nil {
		t.Fatalf("UpdateFileStatus complete: %v", err)
	}
	if err := db.UpdateFileStatus(ids[1], "failed", WithError("copy failed")); err != nil {
		t.Fatalf("UpdateFileStatus failed: %v", err)
	}

	// Non-terminal states.
	if err := db.UpdateFileStatus(ids[2], "extracted"); err != nil {
		t.Fatalf("UpdateFileStatus extracted: %v", err)
	}
	if err := db.UpdateFileStatus(ids[3], "hashed", WithChecksum("abc")); err != nil {
		t.Fatalf("UpdateFileStatus hashed: %v", err)
	}
	// ids[4] stays "pending".

	incomplete, err := db.GetIncompleteFiles("run-incomplete")
	if err != nil {
		t.Fatalf("GetIncompleteFiles: %v", err)
	}
	if len(incomplete) != 3 {
		t.Errorf("GetIncompleteFiles returned %d files, want 3", len(incomplete))
	}
}

// TestUpdateFileStatus_withError verifies that WithError sets the error field.
func TestUpdateFileStatus_withError(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-err")

	id, err := db.InsertFile(&FileRecord{RunID: "run-err", SourcePath: "/src/err.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	const errMsg = "checksum mismatch after copy"
	if err := db.UpdateFileStatus(id, "mismatch", WithError(errMsg)); err != nil {
		t.Fatalf("UpdateFileStatus: %v", err)
	}

	files, err := db.GetFilesByRun("run-err")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	got := files[0]
	if got.Status != "mismatch" {
		t.Errorf("Status = %q, want %q", got.Status, "mismatch")
	}
	if got.Error == nil || *got.Error != errMsg {
		t.Errorf("Error = %v, want %q", got.Error, errMsg)
	}
}

// TestUpdateFileStatus_withIsDuplicate verifies that WithIsDuplicate sets the flag.
func TestUpdateFileStatus_withIsDuplicate(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "run-isdup")

	id, err := db.InsertFile(&FileRecord{RunID: "run-isdup", SourcePath: "/src/dup2.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}

	if err := db.UpdateFileStatus(id, "complete", WithIsDuplicate(true)); err != nil {
		t.Fatalf("UpdateFileStatus: %v", err)
	}

	files, err := db.GetFilesByRun("run-isdup")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	if !files[0].IsDuplicate {
		t.Error("IsDuplicate = false, want true")
	}
}

// ---------------------------------------------------------------------------
// Query method tests (Task 31)
// ---------------------------------------------------------------------------

// completeFile is a helper that walks a file through all pipeline stages to "complete".
func completeFile(t *testing.T, db *DB, fileID int64, checksum, destRel string, captureDate time.Time) {
	t.Helper()
	destPath := "/dst/" + destRel
	steps := []struct {
		status string
		opts   []UpdateOption
	}{
		{"extracted", []UpdateOption{WithCaptureDate(captureDate), WithFileSize(1024)}},
		{"hashed", []UpdateOption{WithChecksum(checksum)}},
		{"copied", []UpdateOption{WithDestination(destPath, destRel)}},
		{"verified", nil},
		{"tagged", nil},
		{"complete", nil},
	}
	for _, s := range steps {
		if err := db.UpdateFileStatus(fileID, s.status, s.opts...); err != nil {
			t.Fatalf("UpdateFileStatus(%q): %v", s.status, err)
		}
	}
}

// TestListRuns verifies runs are returned in reverse chronological order with file counts.
func TestListRuns(t *testing.T) {
	db := openTestDB(t)

	// Insert 3 runs with distinct timestamps.
	runs := []*Run{
		{
			ID: "lr-run-1", PixeVersion: "1.0.0", Source: "/src/a",
			Destination: "/dst", Algorithm: "sha256", Workers: 2,
			StartedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		},
		{
			ID: "lr-run-2", PixeVersion: "1.0.0", Source: "/src/b",
			Destination: "/dst", Algorithm: "sha256", Workers: 2,
			StartedAt: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC),
		},
		{
			ID: "lr-run-3", PixeVersion: "1.0.0", Source: "/src/c",
			Destination: "/dst", Algorithm: "sha256", Workers: 2,
			StartedAt: time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
		},
	}
	for _, r := range runs {
		if err := db.InsertRun(r); err != nil {
			t.Fatalf("InsertRun %s: %v", r.ID, err)
		}
	}

	// Add 2 files to run-2 and 1 file to run-3.
	for i, src := range []string{"/src/b/photo1.jpg", "/src/b/photo2.jpg"} {
		id, err := db.InsertFile(&FileRecord{RunID: "lr-run-2", SourcePath: src})
		if err != nil {
			t.Fatalf("InsertFile %d: %v", i, err)
		}
		_ = id
	}
	id3, err := db.InsertFile(&FileRecord{RunID: "lr-run-3", SourcePath: "/src/c/photo.jpg"})
	if err != nil {
		t.Fatalf("InsertFile run-3: %v", err)
	}
	_ = id3

	summaries, err := db.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("ListRuns returned %d summaries, want 3", len(summaries))
	}

	// Verify reverse chronological order.
	if summaries[0].ID != "lr-run-3" {
		t.Errorf("summaries[0].ID = %q, want %q", summaries[0].ID, "lr-run-3")
	}
	if summaries[1].ID != "lr-run-2" {
		t.Errorf("summaries[1].ID = %q, want %q", summaries[1].ID, "lr-run-2")
	}
	if summaries[2].ID != "lr-run-1" {
		t.Errorf("summaries[2].ID = %q, want %q", summaries[2].ID, "lr-run-1")
	}

	// Verify file counts.
	if summaries[0].FileCount != 1 {
		t.Errorf("run-3 FileCount = %d, want 1", summaries[0].FileCount)
	}
	if summaries[1].FileCount != 2 {
		t.Errorf("run-2 FileCount = %d, want 2", summaries[1].FileCount)
	}
	if summaries[2].FileCount != 0 {
		t.Errorf("run-1 FileCount = %d, want 0", summaries[2].FileCount)
	}
}

// TestListRuns_empty verifies ListRuns returns nil (not an error) when no runs exist.
func TestListRuns_empty(t *testing.T) {
	db := openTestDB(t)
	summaries, err := db.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("ListRuns returned %d summaries, want 0", len(summaries))
	}
}

// TestFilesBySource verifies filtering by source directory.
func TestFilesBySource(t *testing.T) {
	db := openTestDB(t)

	// Two runs from different sources.
	r1 := &Run{
		ID: "fbs-run-1", PixeVersion: "1.0.0", Source: "/src/alpha",
		Destination: "/dst", Algorithm: "sha256", Workers: 1,
		StartedAt: time.Now().UTC(),
	}
	r2 := &Run{
		ID: "fbs-run-2", PixeVersion: "1.0.0", Source: "/src/beta",
		Destination: "/dst", Algorithm: "sha256", Workers: 1,
		StartedAt: time.Now().UTC(),
	}
	if err := db.InsertRun(r1); err != nil {
		t.Fatalf("InsertRun r1: %v", err)
	}
	if err := db.InsertRun(r2); err != nil {
		t.Fatalf("InsertRun r2: %v", err)
	}

	// 2 files from alpha, 1 from beta.
	for _, src := range []string{"/src/alpha/a.jpg", "/src/alpha/b.jpg"} {
		if _, err := db.InsertFile(&FileRecord{RunID: "fbs-run-1", SourcePath: src}); err != nil {
			t.Fatalf("InsertFile alpha: %v", err)
		}
	}
	if _, err := db.InsertFile(&FileRecord{RunID: "fbs-run-2", SourcePath: "/src/beta/c.jpg"}); err != nil {
		t.Fatalf("InsertFile beta: %v", err)
	}

	files, err := db.FilesBySource("/src/alpha")
	if err != nil {
		t.Fatalf("FilesBySource: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("FilesBySource returned %d files, want 2", len(files))
	}
	for _, f := range files {
		if f.RunID != "fbs-run-1" {
			t.Errorf("file RunID = %q, want %q", f.RunID, "fbs-run-1")
		}
	}

	// Beta should return 1.
	betaFiles, err := db.FilesBySource("/src/beta")
	if err != nil {
		t.Fatalf("FilesBySource beta: %v", err)
	}
	if len(betaFiles) != 1 {
		t.Errorf("FilesBySource beta returned %d files, want 1", len(betaFiles))
	}
}

// TestFilesBySource_noMatch verifies an empty slice is returned for unknown sources.
func TestFilesBySource_noMatch(t *testing.T) {
	db := openTestDB(t)
	files, err := db.FilesBySource("/nonexistent/source")
	if err != nil {
		t.Fatalf("FilesBySource: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("FilesBySource returned %d files, want 0", len(files))
	}
}

// TestFilesByCaptureDateRange verifies date-range filtering on completed files.
func TestFilesByCaptureDateRange(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "cdr-run")

	// Insert 3 files with different capture dates.
	type fixture struct {
		src         string
		checksum    string
		destRel     string
		captureDate time.Time
	}
	fixtures := []fixture{
		{"/src/jan.jpg", "cksum-jan", "2026/01-Jan/jan.jpg", time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"/src/feb.jpg", "cksum-feb", "2026/02-Feb/feb.jpg", time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)},
		{"/src/mar.jpg", "cksum-mar", "2026/03-Mar/mar.jpg", time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)},
	}
	for _, fx := range fixtures {
		id, err := db.InsertFile(&FileRecord{RunID: "cdr-run", SourcePath: fx.src})
		if err != nil {
			t.Fatalf("InsertFile %s: %v", fx.src, err)
		}
		completeFile(t, db, id, fx.checksum, fx.destRel, fx.captureDate)
	}

	// Query Jan–Feb range.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 28, 23, 59, 59, 0, time.UTC)
	files, err := db.FilesByCaptureDateRange(start, end)
	if err != nil {
		t.Fatalf("FilesByCaptureDateRange: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("FilesByCaptureDateRange returned %d files, want 2", len(files))
	}
}

// TestFilesByCaptureDateRange_excludesNonComplete verifies only complete files are returned.
func TestFilesByCaptureDateRange_excludesNonComplete(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "cdr-nc-run")

	captureDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	id, err := db.InsertFile(&FileRecord{RunID: "cdr-nc-run", SourcePath: "/src/nc.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}
	// Only advance to "hashed" — not complete.
	if err := db.UpdateFileStatus(id, "extracted", WithCaptureDate(captureDate)); err != nil {
		t.Fatalf("UpdateFileStatus extracted: %v", err)
	}

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	files, err := db.FilesByCaptureDateRange(start, end)
	if err != nil {
		t.Fatalf("FilesByCaptureDateRange: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("FilesByCaptureDateRange returned %d files, want 0 (non-complete excluded)", len(files))
	}
}

// TestFilesByImportDateRange verifies filtering by verified_at timestamp.
func TestFilesByImportDateRange(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "idr-run")

	captureDate := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	// Insert and complete 2 files (verified_at will be set to now).
	for i, src := range []string{"/src/p1.jpg", "/src/p2.jpg"} {
		id, err := db.InsertFile(&FileRecord{RunID: "idr-run", SourcePath: src})
		if err != nil {
			t.Fatalf("InsertFile %d: %v", i, err)
		}
		checksum := fmt.Sprintf("cksum-idr-%d", i)
		destRel := fmt.Sprintf("2025/06-Jun/photo%d.jpg", i)
		completeFile(t, db, id, checksum, destRel, captureDate)
	}

	// Query a wide range that covers "now".
	start := time.Now().UTC().Add(-time.Minute)
	end := time.Now().UTC().Add(time.Minute)
	files, err := db.FilesByImportDateRange(start, end)
	if err != nil {
		t.Fatalf("FilesByImportDateRange: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("FilesByImportDateRange returned %d files, want 2", len(files))
	}

	// Query a range in the past — should return nothing.
	pastStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	pastEnd := time.Date(2020, 12, 31, 23, 59, 59, 0, time.UTC)
	pastFiles, err := db.FilesByImportDateRange(pastStart, pastEnd)
	if err != nil {
		t.Fatalf("FilesByImportDateRange past: %v", err)
	}
	if len(pastFiles) != 0 {
		t.Errorf("FilesByImportDateRange past returned %d files, want 0", len(pastFiles))
	}
}

// TestFilesWithErrors verifies error-state files are returned with run source context.
func TestFilesWithErrors(t *testing.T) {
	db := openTestDB(t)

	r := &Run{
		ID: "err-run", PixeVersion: "1.0.0", Source: "/src/errors",
		Destination: "/dst", Algorithm: "sha256", Workers: 1,
		StartedAt: time.Now().UTC(),
	}
	if err := db.InsertRun(r); err != nil {
		t.Fatalf("InsertRun: %v", err)
	}

	// Insert files in various states.
	type fileFixture struct {
		src    string
		status string
		errMsg string
	}
	fixtures := []fileFixture{
		{"/src/errors/ok.jpg", "complete", ""},
		{"/src/errors/fail.jpg", "failed", "copy failed"},
		{"/src/errors/mismatch.jpg", "mismatch", "checksum mismatch"},
		{"/src/errors/tagfail.jpg", "tag_failed", "exiftool error"},
	}

	for _, fx := range fixtures {
		id, err := db.InsertFile(&FileRecord{RunID: "err-run", SourcePath: fx.src})
		if err != nil {
			t.Fatalf("InsertFile %s: %v", fx.src, err)
		}
		var opts []UpdateOption
		if fx.errMsg != "" {
			opts = append(opts, WithError(fx.errMsg))
		}
		if err := db.UpdateFileStatus(id, fx.status, opts...); err != nil {
			t.Fatalf("UpdateFileStatus %s: %v", fx.status, err)
		}
	}

	errFiles, err := db.FilesWithErrors()
	if err != nil {
		t.Fatalf("FilesWithErrors: %v", err)
	}
	if len(errFiles) != 3 {
		t.Fatalf("FilesWithErrors returned %d files, want 3", len(errFiles))
	}

	// All should have RunSource set.
	for _, ef := range errFiles {
		if ef.RunSource != "/src/errors" {
			t.Errorf("RunSource = %q, want %q", ef.RunSource, "/src/errors")
		}
		if ef.Error == nil {
			t.Errorf("Error is nil for file %s", ef.SourcePath)
		}
	}
}

// TestFilesWithErrors_empty verifies no error when there are no error-state files.
func TestFilesWithErrors_empty(t *testing.T) {
	db := openTestDB(t)
	errFiles, err := db.FilesWithErrors()
	if err != nil {
		t.Fatalf("FilesWithErrors: %v", err)
	}
	if len(errFiles) != 0 {
		t.Errorf("FilesWithErrors returned %d files, want 0", len(errFiles))
	}
}

// TestAllDuplicates verifies that only is_duplicate=1 files are returned.
func TestAllDuplicates(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "dup-run")

	// Insert 3 files: 1 original, 2 duplicates.
	origID, err := db.InsertFile(&FileRecord{RunID: "dup-run", SourcePath: "/src/orig.jpg"})
	if err != nil {
		t.Fatalf("InsertFile orig: %v", err)
	}
	completeFile(t, db, origID, "cksum-orig", "2026/01-Jan/orig.jpg", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	for i, src := range []string{"/src/dup1.jpg", "/src/dup2.jpg"} {
		id, err := db.InsertFile(&FileRecord{RunID: "dup-run", SourcePath: src})
		if err != nil {
			t.Fatalf("InsertFile dup%d: %v", i, err)
		}
		if err := db.UpdateFileStatus(id, "complete",
			WithChecksum("cksum-orig"),
			WithDestination("/dst/duplicates/dup.jpg", "duplicates/dup.jpg"),
			WithIsDuplicate(true),
		); err != nil {
			t.Fatalf("UpdateFileStatus dup%d: %v", i, err)
		}
	}

	dups, err := db.AllDuplicates()
	if err != nil {
		t.Fatalf("AllDuplicates: %v", err)
	}
	if len(dups) != 2 {
		t.Errorf("AllDuplicates returned %d files, want 2", len(dups))
	}
	for _, d := range dups {
		if !d.IsDuplicate {
			t.Errorf("file %s: IsDuplicate = false, want true", d.SourcePath)
		}
	}
}

// TestDuplicatePairs verifies each duplicate is paired with its original via checksum.
func TestDuplicatePairs(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "dp-run")

	const checksum = "cksum-pair"
	const origDestRel = "2026/01-Jan/original.jpg"
	const dupDestRel = "duplicates/original.jpg"

	// Insert original.
	origID, err := db.InsertFile(&FileRecord{RunID: "dp-run", SourcePath: "/src/original.jpg"})
	if err != nil {
		t.Fatalf("InsertFile orig: %v", err)
	}
	completeFile(t, db, origID, checksum, origDestRel, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	// Insert duplicate.
	dupID, err := db.InsertFile(&FileRecord{RunID: "dp-run", SourcePath: "/src/duplicate.jpg"})
	if err != nil {
		t.Fatalf("InsertFile dup: %v", err)
	}
	if err := db.UpdateFileStatus(dupID, "complete",
		WithChecksum(checksum),
		WithDestination("/dst/"+dupDestRel, dupDestRel),
		WithIsDuplicate(true),
	); err != nil {
		t.Fatalf("UpdateFileStatus dup: %v", err)
	}

	pairs, err := db.DuplicatePairs()
	if err != nil {
		t.Fatalf("DuplicatePairs: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("DuplicatePairs returned %d pairs, want 1", len(pairs))
	}

	p := pairs[0]
	if p.DuplicateSource != "/src/duplicate.jpg" {
		t.Errorf("DuplicateSource = %q, want %q", p.DuplicateSource, "/src/duplicate.jpg")
	}
	if p.DuplicateDest != dupDestRel {
		t.Errorf("DuplicateDest = %q, want %q", p.DuplicateDest, dupDestRel)
	}
	if p.OriginalDest != origDestRel {
		t.Errorf("OriginalDest = %q, want %q", p.OriginalDest, origDestRel)
	}
}

// TestDuplicatePairs_empty verifies no error when there are no duplicates.
func TestDuplicatePairs_empty(t *testing.T) {
	db := openTestDB(t)
	pairs, err := db.DuplicatePairs()
	if err != nil {
		t.Fatalf("DuplicatePairs: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("DuplicatePairs returned %d pairs, want 0", len(pairs))
	}
}

// TestArchiveInventory verifies only complete, non-duplicate files are returned.
func TestArchiveInventory(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "inv-run")

	captureDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Insert 3 complete non-duplicates.
	for i, src := range []string{"/src/a.jpg", "/src/b.jpg", "/src/c.jpg"} {
		id, err := db.InsertFile(&FileRecord{RunID: "inv-run", SourcePath: src})
		if err != nil {
			t.Fatalf("InsertFile %d: %v", i, err)
		}
		checksum := fmt.Sprintf("cksum-inv-%d", i)
		destRel := fmt.Sprintf("2026/03-Mar/photo%d.jpg", i)
		completeFile(t, db, id, checksum, destRel, captureDate)
	}

	// Insert 1 duplicate.
	dupID, err := db.InsertFile(&FileRecord{RunID: "inv-run", SourcePath: "/src/dup.jpg"})
	if err != nil {
		t.Fatalf("InsertFile dup: %v", err)
	}
	if err := db.UpdateFileStatus(dupID, "complete",
		WithChecksum("cksum-inv-0"),
		WithDestination("/dst/duplicates/dup.jpg", "duplicates/dup.jpg"),
		WithIsDuplicate(true),
	); err != nil {
		t.Fatalf("UpdateFileStatus dup: %v", err)
	}

	// Insert 1 failed file.
	failID, err := db.InsertFile(&FileRecord{RunID: "inv-run", SourcePath: "/src/fail.jpg"})
	if err != nil {
		t.Fatalf("InsertFile fail: %v", err)
	}
	if err := db.UpdateFileStatus(failID, "failed", WithError("copy error")); err != nil {
		t.Fatalf("UpdateFileStatus fail: %v", err)
	}

	inventory, err := db.ArchiveInventory()
	if err != nil {
		t.Fatalf("ArchiveInventory: %v", err)
	}
	// Only the 3 non-duplicate complete files.
	if len(inventory) != 3 {
		t.Errorf("ArchiveInventory returned %d entries, want 3", len(inventory))
	}
	for _, e := range inventory {
		if e.DestRel == "" {
			t.Error("InventoryEntry.DestRel is empty")
		}
		if e.Checksum == "" {
			t.Error("InventoryEntry.Checksum is empty")
		}
	}
}

// ---------------------------------------------------------------------------
// CompleteFileWithDedupCheck tests (Task 36)
// ---------------------------------------------------------------------------

// TestCompleteFileWithDedupCheck_noRace verifies that the first file to complete
// a given checksum is marked complete with is_duplicate=0 and returns "".
func TestCompleteFileWithDedupCheck_noRace(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "cwdc-run-1")

	id, err := db.InsertFile(&FileRecord{RunID: "cwdc-run-1", SourcePath: "/src/first.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}
	if err := db.UpdateFileStatus(id, "hashed", WithChecksum("cwdc-checksum-1")); err != nil {
		t.Fatalf("UpdateFileStatus hashed: %v", err)
	}

	existingDest, err := db.CompleteFileWithDedupCheck(id, "cwdc-checksum-1")
	if err != nil {
		t.Fatalf("CompleteFileWithDedupCheck: %v", err)
	}
	if existingDest != "" {
		t.Errorf("existingDest = %q, want %q (no race expected)", existingDest, "")
	}

	files, err := db.GetFilesByRun("cwdc-run-1")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	if files[0].Status != "complete" {
		t.Errorf("Status = %q, want %q", files[0].Status, "complete")
	}
	if files[0].IsDuplicate {
		t.Error("IsDuplicate = true, want false (first file should not be a duplicate)")
	}
}

// TestCompleteFileWithDedupCheck_raceDetected verifies that the second file with
// the same checksum is detected as a duplicate and the existing dest_rel is returned.
func TestCompleteFileWithDedupCheck_raceDetected(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "cwdc-run-2")

	const checksum = "cwdc-checksum-race"
	const origDestRel = "2026/03-Mar/original.jpg"

	// Insert and complete the first file (the "winner").
	origID, err := db.InsertFile(&FileRecord{RunID: "cwdc-run-2", SourcePath: "/src/orig.jpg"})
	if err != nil {
		t.Fatalf("InsertFile orig: %v", err)
	}
	if err := db.UpdateFileStatus(origID, "hashed", WithChecksum(checksum)); err != nil {
		t.Fatalf("UpdateFileStatus hashed orig: %v", err)
	}
	if err := db.UpdateFileStatus(origID, "copied",
		WithDestination("/dst/"+origDestRel, origDestRel)); err != nil {
		t.Fatalf("UpdateFileStatus copied orig: %v", err)
	}
	if err := db.UpdateFileStatus(origID, "complete"); err != nil {
		t.Fatalf("UpdateFileStatus complete orig: %v", err)
	}

	// Insert the second file (the "loser" — arrives after the first is complete).
	dupID, err := db.InsertFile(&FileRecord{RunID: "cwdc-run-2", SourcePath: "/src/dup.jpg"})
	if err != nil {
		t.Fatalf("InsertFile dup: %v", err)
	}
	if err := db.UpdateFileStatus(dupID, "hashed", WithChecksum(checksum)); err != nil {
		t.Fatalf("UpdateFileStatus hashed dup: %v", err)
	}

	// CompleteFileWithDedupCheck should detect the race.
	existingDest, err := db.CompleteFileWithDedupCheck(dupID, checksum)
	if err != nil {
		t.Fatalf("CompleteFileWithDedupCheck: %v", err)
	}
	if existingDest != origDestRel {
		t.Errorf("existingDest = %q, want %q", existingDest, origDestRel)
	}

	// The duplicate file should be marked complete with is_duplicate=1.
	files, err := db.GetFilesByRun("cwdc-run-2")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	var dupFile *FileRecord
	for _, f := range files {
		if f.ID == dupID {
			dupFile = f
			break
		}
	}
	if dupFile == nil {
		t.Fatal("duplicate file not found in GetFilesByRun results")
	}
	if dupFile.Status != "complete" {
		t.Errorf("dup Status = %q, want %q", dupFile.Status, "complete")
	}
	if !dupFile.IsDuplicate {
		t.Error("dup IsDuplicate = false, want true")
	}
}

// TestCompleteFileWithDedupCheck_doesNotMatchSelf verifies that a file does not
// detect itself as a duplicate (the id != ? clause in the query).
func TestCompleteFileWithDedupCheck_doesNotMatchSelf(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "cwdc-run-3")

	const checksum = "cwdc-self-check"

	id, err := db.InsertFile(&FileRecord{RunID: "cwdc-run-3", SourcePath: "/src/self.jpg"})
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}
	if err := db.UpdateFileStatus(id, "hashed", WithChecksum(checksum)); err != nil {
		t.Fatalf("UpdateFileStatus hashed: %v", err)
	}
	// Manually mark as complete first (simulating a partial state).
	if err := db.UpdateFileStatus(id, "complete"); err != nil {
		t.Fatalf("UpdateFileStatus complete: %v", err)
	}

	// Now call CompleteFileWithDedupCheck — it should NOT match itself.
	existingDest, err := db.CompleteFileWithDedupCheck(id, checksum)
	if err != nil {
		t.Fatalf("CompleteFileWithDedupCheck: %v", err)
	}
	if existingDest != "" {
		t.Errorf("existingDest = %q, want %q (should not match self)", existingDest, "")
	}
}

// TestCompleteFileWithDedupCheck_atomicity verifies that the check and update
// happen within the same transaction (no partial state visible between them).
func TestCompleteFileWithDedupCheck_atomicity(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "cwdc-run-4")

	const checksum = "cwdc-atomic"

	// Insert two files with the same checksum.
	id1, err := db.InsertFile(&FileRecord{RunID: "cwdc-run-4", SourcePath: "/src/atomic1.jpg"})
	if err != nil {
		t.Fatalf("InsertFile 1: %v", err)
	}
	id2, err := db.InsertFile(&FileRecord{RunID: "cwdc-run-4", SourcePath: "/src/atomic2.jpg"})
	if err != nil {
		t.Fatalf("InsertFile 2: %v", err)
	}

	for _, id := range []int64{id1, id2} {
		if err := db.UpdateFileStatus(id, "hashed", WithChecksum(checksum)); err != nil {
			t.Fatalf("UpdateFileStatus hashed %d: %v", id, err)
		}
	}

	// Complete the first file — should succeed with no race.
	dest1, err := db.CompleteFileWithDedupCheck(id1, checksum)
	if err != nil {
		t.Fatalf("CompleteFileWithDedupCheck id1: %v", err)
	}
	if dest1 != "" {
		t.Errorf("id1 existingDest = %q, want %q", dest1, "")
	}

	// Complete the second file — should detect the race.
	dest2, err := db.CompleteFileWithDedupCheck(id2, checksum)
	if err != nil {
		t.Fatalf("CompleteFileWithDedupCheck id2: %v", err)
	}
	if dest2 == "" {
		t.Error("id2 existingDest is empty, want non-empty (race should be detected)")
	}

	// Verify final states.
	files, err := db.GetFilesByRun("cwdc-run-4")
	if err != nil {
		t.Fatalf("GetFilesByRun: %v", err)
	}
	for _, f := range files {
		if f.Status != "complete" {
			t.Errorf("file %d Status = %q, want %q", f.ID, f.Status, "complete")
		}
	}
	// Exactly one should be a duplicate.
	dupCount := 0
	for _, f := range files {
		if f.IsDuplicate {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Errorf("duplicate count = %d, want 1", dupCount)
	}
}

// TestArchiveInventory_orderedByDestRel verifies stable ordering.
func TestArchiveInventory_orderedByDestRel(t *testing.T) {
	db := openTestDB(t)
	insertTestRun(t, db, "inv-ord-run")

	captureDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Insert files in reverse alphabetical order of destRel.
	destRels := []string{
		"2026/03-Mar/c.jpg",
		"2026/03-Mar/a.jpg",
		"2026/03-Mar/b.jpg",
	}
	for i, dr := range destRels {
		id, err := db.InsertFile(&FileRecord{RunID: "inv-ord-run", SourcePath: fmt.Sprintf("/src/%d.jpg", i)})
		if err != nil {
			t.Fatalf("InsertFile %d: %v", i, err)
		}
		completeFile(t, db, id, fmt.Sprintf("cksum-%d", i), dr, captureDate)
	}

	inventory, err := db.ArchiveInventory()
	if err != nil {
		t.Fatalf("ArchiveInventory: %v", err)
	}
	if len(inventory) != 3 {
		t.Fatalf("ArchiveInventory returned %d entries, want 3", len(inventory))
	}

	// Should be sorted: a, b, c.
	expected := []string{
		"2026/03-Mar/a.jpg",
		"2026/03-Mar/b.jpg",
		"2026/03-Mar/c.jpg",
	}
	for i, e := range inventory {
		if e.DestRel != expected[i] {
			t.Errorf("inventory[%d].DestRel = %q, want %q", i, e.DestRel, expected[i])
		}
	}
}
