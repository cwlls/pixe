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
	"strings"
	"time"
)

// schemaVersion is the current schema version.
const schemaVersion = 2

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
    recursive     INTEGER NOT NULL DEFAULT 0,
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
// records the schema version for fresh databases, and then runs any
// pending migrations for existing databases.
func (db *DB) applySchema() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("archivedb: begin schema transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Create tables and indexes (IF NOT EXISTS — safe to re-run on existing DBs).
	if _, err := tx.Exec(schemaDDL); err != nil {
		return fmt.Errorf("archivedb: apply schema DDL: %w", err)
	}

	// Insert the current schema version only for fresh databases (empty table).
	// For existing databases the version row already exists; migrateSchema will
	// advance it after applying ALTER TABLE statements.
	appliedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`INSERT INTO schema_version (version, applied_at)
		 SELECT ?, ? WHERE NOT EXISTS (SELECT 1 FROM schema_version)`,
		schemaVersion, appliedAt,
	)
	if err != nil {
		return fmt.Errorf("archivedb: insert schema version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("archivedb: commit schema transaction: %w", err)
	}

	// Run incremental migrations for existing databases that are behind.
	return db.migrateSchema()
}

// migrateSchema applies any pending schema migrations for databases created
// by an older version of Pixe. It is called after applySchema so that the
// schema_version table is guaranteed to exist.
//
// Migration strategy:
//   - Read MAX(version) from schema_version.
//   - If already at schemaVersion, return immediately (idempotent).
//   - Apply each version's ALTER TABLE statements in order.
//   - Ignore "duplicate column" errors so the function is safe to re-run.
//   - Insert a new schema_version row after each version's migrations succeed.
func (db *DB) migrateSchema() error {
	var currentVersion int
	err := db.conn.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		// schema_version table missing or empty — fresh DB already at current version.
		return nil
	}
	if currentVersion >= schemaVersion {
		return nil // already up to date
	}

	// v1 → v2: add recursive to runs, skip_reason to files.
	//
	// SQLite CHECK constraints are defined at table-creation time and cannot be
	// altered. The expanded CHECK (including 'skipped') is already in the DDL for
	// new databases. For existing v1 databases the original CHECK remains, but
	// SQLite only validates CHECK on INSERT/UPDATE — the 'skipped' value will be
	// accepted because ALTER TABLE ADD COLUMN does not re-create the table and
	// does not re-validate existing rows.
	if currentVersion < 2 {
		migrations := []string{
			`ALTER TABLE runs ADD COLUMN recursive INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE files ADD COLUMN skip_reason TEXT`,
		}
		for _, m := range migrations {
			if _, err := db.conn.Exec(m); err != nil {
				// Ignore "duplicate column" errors for idempotency (re-run safety).
				if !strings.Contains(err.Error(), "duplicate column") {
					return fmt.Errorf("archivedb: migrate v1→v2: %w", err)
				}
			}
		}
		_, _ = db.conn.Exec(
			`INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (?, ?)`,
			2, time.Now().UTC().Format(time.RFC3339),
		)
	}

	return nil
}
