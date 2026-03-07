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
	"path/filepath"
	"testing"
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
