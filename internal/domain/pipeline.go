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

package domain

import "time"

// FileStatus represents the processing state of a single file as it moves
// through the sort pipeline. States are persisted in the manifest so that
// interrupted runs can resume from the correct stage.
type FileStatus string

const (
	// StatusPending means the file has been discovered but not yet processed.
	StatusPending FileStatus = "pending"

	// StatusExtracted means the filetype module has read the capture date
	// and identified the hashable data region.
	StatusExtracted FileStatus = "extracted"

	// StatusHashed means the checksum has been computed over the media payload.
	StatusHashed FileStatus = "hashed"

	// StatusCopied means the file has been written to its destination path
	// in dirB but has not yet been verified.
	StatusCopied FileStatus = "copied"

	// StatusVerified means the destination file was re-read and re-hashed,
	// confirming the copy is intact.
	StatusVerified FileStatus = "verified"

	// StatusTagged means optional metadata tags (Copyright, CameraOwner)
	// have been injected into the destination file.
	StatusTagged FileStatus = "tagged"

	// StatusComplete means all pipeline stages succeeded. The file is
	// recorded in the ledger.
	StatusComplete FileStatus = "complete"

	// StatusFailed means a non-recoverable error occurred during processing.
	// The file is flagged for user attention.
	StatusFailed FileStatus = "failed"

	// StatusMismatch means the post-copy verification hash did not match
	// the source hash. The destination file is preserved for debugging.
	StatusMismatch FileStatus = "mismatch"

	// StatusTagFailed means copy and verify succeeded but metadata tag
	// injection failed. The file is otherwise intact.
	StatusTagFailed FileStatus = "tag_failed"

	// StatusSkipped means the file was intentionally not processed.
	// The skip_reason field records why (e.g., "previously imported",
	// "unsupported format: .txt").
	StatusSkipped FileStatus = "skipped"
)

// String implements fmt.Stringer.
func (s FileStatus) String() string { return string(s) }

// IsTerminal reports whether the status represents a final state —
// one from which the pipeline will not advance further.
func (s FileStatus) IsTerminal() bool {
	switch s {
	case StatusComplete, StatusFailed, StatusMismatch, StatusTagFailed, StatusSkipped:
		return true
	}
	return false
}

// IsError reports whether the status represents an error condition.
func (s FileStatus) IsError() bool {
	switch s {
	case StatusFailed, StatusMismatch, StatusTagFailed:
		return true
	}
	return false
}

// ManifestEntry tracks the full lifecycle of a single file through the
// sort pipeline. It is serialized into dirB/.pixe/manifest.json.
type ManifestEntry struct {
	Source      string     `json:"source"`                 // Source is the absolute path to the file in dirA.
	Destination string     `json:"destination,omitempty"`  // Destination is the relative path within dirB where the file was copied.
	Checksum    string     `json:"checksum,omitempty"`     // Checksum is the hex-encoded hash of the media payload.
	Status      FileStatus `json:"status"`                 // Status is the terminal pipeline state (e.g., "complete", "failed").
	ExtractedAt *time.Time `json:"extracted_at,omitempty"` // ExtractedAt is when metadata extraction completed.
	CopiedAt    *time.Time `json:"copied_at,omitempty"`    // CopiedAt is when the file copy completed.
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`  // VerifiedAt is when post-copy hash verification completed.
	TaggedAt    *time.Time `json:"tagged_at,omitempty"`    // TaggedAt is when metadata tagging completed.
	Error       string     `json:"error,omitempty"`        // Error is the error message if the file failed at any stage.
}

// Manifest is the top-level operational journal written to
// dirB/.pixe/manifest.json. It is created at the start of a sort run
// and updated after each file completes a pipeline stage.
type Manifest struct {
	Version     int              `json:"version"`      // Version is the manifest schema version.
	PixeVersion string           `json:"pixe_version"` // PixeVersion is the Pixe binary version that produced this manifest.
	Source      string           `json:"source"`       // Source is the absolute path to dirA.
	Destination string           `json:"destination"`  // Destination is the absolute path to dirB.
	Algorithm   string           `json:"algorithm"`    // Algorithm is the hash algorithm used (e.g., "sha1", "sha256", "blake3", "xxhash").
	StartedAt   time.Time        `json:"started_at"`   // StartedAt is the ISO 8601 UTC timestamp of when the sort run started.
	Workers     int              `json:"workers"`      // Workers is the number of concurrent workers used for this run.
	Files       []*ManifestEntry `json:"files"`        // Files is the list of all files processed in this run.
}

// Ledger status constants identify the outcome of a single file in the ledger.
// These are distinct from FileStatus (which tracks pipeline stages) — ledger
// statuses are the four user-visible outcomes written to .pixe_ledger.json.
const (
	// LedgerStatusCopy indicates the file was successfully copied to the archive.
	LedgerStatusCopy = "copy"

	// LedgerStatusSkip indicates the file was skipped (previously imported or unsupported format).
	LedgerStatusSkip = "skip"

	// LedgerStatusDuplicate indicates the file is a content duplicate of an already-archived file.
	LedgerStatusDuplicate = "duplicate"

	// LedgerStatusError indicates the file failed at some pipeline stage.
	LedgerStatusError = "error"
)

// LedgerEntry records the outcome of a single file discovered in dirA.
// Every discovered file (except ignored files) gets one entry.
// RunID links the entry to its sort run (matches LedgerHeader.RunID for that run).
// The omitempty tag on RunID ensures pre-v6 ledgers without the field round-trip
// cleanly — missing RunID deserialises as "" and is not re-serialised as "run_id":"".
type LedgerEntry struct {
	RunID       string     `json:"run_id,omitempty"`      // UUID of the run that produced this entry (v6+)
	Path        string     `json:"path"`                  // relative path from dirA
	Status      string     `json:"status"`                // "copy", "skip", "duplicate", "error"
	Checksum    string     `json:"checksum,omitempty"`    // hex hash (copy, duplicate, skip-previously-imported)
	Destination string     `json:"destination,omitempty"` // relative path in dirB (copy, duplicate)
	VerifiedAt  *time.Time `json:"verified_at,omitempty"` // ISO 8601 UTC (copy only)
	Sidecars    []string   `json:"sidecars,omitempty"`    // carried sidecar dest_rel paths (copy only)
	Matches     string     `json:"matches,omitempty"`     // existing file path (duplicate only)
	Reason      string     `json:"reason,omitempty"`      // explanation (skip, error)
}

// LedgerHeader is written as a separator line at the start of each run in the
// cumulative JSONL ledger file. Since v6 the ledger appends across runs, so
// multiple headers may appear in a single file — one per run.
// Consumers distinguish headers from entries by the presence of the "version" key.
type LedgerHeader struct {
	Version     int    `json:"version"`      // 5 for legacy single-run ledgers; 6 for cumulative append ledgers
	RunID       string `json:"run_id"`       // UUID linking to archive DB runs table
	PixeVersion string `json:"pixe_version"` // Pixe binary version that produced this ledger
	PixeRun     string `json:"pixe_run"`     // ISO 8601 UTC timestamp of run start
	Algorithm   string `json:"algorithm"`    // hash algorithm used ("md5", "sha1", "sha256", "blake3", "xxhash")
	Destination string `json:"destination"`  // absolute path to dirB
	Recursive   bool   `json:"recursive"`    // whether --recursive was active
}
