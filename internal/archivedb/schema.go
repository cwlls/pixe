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
	"time"
)

// schemaVersion is the current schema version.
const schemaVersion = 1

// schemaDDL contains all CREATE TABLE and CREATE INDEX statements.
// Every statement uses IF NOT EXISTS so the function is idempotent.
const schemaDDL = `
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id            TEXT PRIMARY KEY,
    pixe_version  TEXT NOT NULL,
    source        TEXT NOT NULL,
    destination   TEXT NOT NULL,
    algorithm     TEXT NOT NULL,
    workers       INTEGER NOT NULL,
    started_at    TEXT NOT NULL,
    finished_at   TEXT,
    status        TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'interrupted'))
);

CREATE TABLE IF NOT EXISTS files (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id        TEXT NOT NULL REFERENCES runs(id),
    source_path   TEXT NOT NULL,
    dest_path     TEXT,
    dest_rel      TEXT,
    checksum      TEXT,
    skip_reason   TEXT,
    status        TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN (
            'pending', 'extracted', 'hashed', 'copied',
            'verified', 'tagged', 'complete',
            'failed', 'mismatch', 'tag_failed', 'duplicate',
            'skipped'
        )),
    is_duplicate  INTEGER NOT NULL DEFAULT 0,
    capture_date  TEXT,
    file_size     INTEGER,
    extracted_at  TEXT,
    hashed_at     TEXT,
    copied_at     TEXT,
    verified_at   TEXT,
    tagged_at     TEXT,
    error         TEXT
);

CREATE INDEX IF NOT EXISTS idx_files_checksum ON files(checksum) WHERE status = 'complete';
CREATE INDEX IF NOT EXISTS idx_files_run_id ON files(run_id);
CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
CREATE INDEX IF NOT EXISTS idx_files_source ON files(source_path);
CREATE INDEX IF NOT EXISTS idx_files_capture_date ON files(capture_date);
`

// applySchema creates all tables and indexes if they do not exist,
// and records the schema version.
func (db *DB) applySchema() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("archivedb: begin schema transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Create tables and indexes.
	if _, err := tx.Exec(schemaDDL); err != nil {
		return fmt.Errorf("archivedb: apply schema DDL: %w", err)
	}

	// Insert schema version row if not already present.
	appliedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (?, ?)`,
		schemaVersion, appliedAt,
	)
	if err != nil {
		return fmt.Errorf("archivedb: insert schema version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("archivedb: commit schema transaction: %w", err)
	}
	return nil
}
