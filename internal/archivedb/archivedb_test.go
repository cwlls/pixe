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
