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

// Package archivedb provides SQLite-backed persistence for the Pixe archive
// database. It replaces the earlier JSON manifest with a cumulative registry
// that tracks all files ever sorted into a destination archive across all runs.
//
// The database uses WAL mode for concurrent-process safety and commits each
// file completion individually for crash recovery.
package archivedb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // CGo-free SQLite driver
)

// DB wraps a SQLite database connection for the Pixe archive.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens (or creates) the archive database at the given path.
// It applies the schema if the database is new, configures WAL mode,
// busy timeout, and enables foreign keys.
func Open(path string) (*DB, error) {
	// Ensure the parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("archivedb: create parent directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("archivedb: open database: %w", err)
	}

	// SQLite performs best with a single writer connection; WAL mode handles
	// concurrent readers without blocking. Limit to one open connection to
	// avoid "database is locked" errors from the same process.
	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn, path: path}

	// Apply PRAGMAs on every open.
	if err := db.applyPragmas(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	// Apply schema (idempotent — uses IF NOT EXISTS).
	if err := db.applySchema(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return db, nil
}

// OpenReadOnly opens an existing archive database at the given path in
// read-only mode. It does not create the file, create parent directories,
// or apply schema migrations — the database must already exist.
//
// Use OpenReadOnly for commands that only query the database (e.g. pixe query).
// The ?mode=ro DSN parameter enforces read-only access at the driver level so
// that a bug in a query command cannot accidentally mutate the archive.
func OpenReadOnly(path string) (*DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("archivedb: database not found: %s", path)
	}

	conn, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("archivedb: open database read-only: %w", err)
	}

	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn, path: path}

	if err := db.applyPragmas(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Path returns the filesystem path to the database file.
func (db *DB) Path() string { return db.path }

// applyPragmas configures the database connection for WAL mode, busy timeout,
// and foreign key enforcement. Called on every Open.
func (db *DB) applyPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.conn.Exec(p); err != nil {
			return fmt.Errorf("archivedb: pragma %q: %w", p, err)
		}
	}
	return nil
}
