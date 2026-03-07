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
)

// String implements fmt.Stringer.
func (s FileStatus) String() string { return string(s) }

// IsTerminal reports whether the status represents a final state —
// one from which the pipeline will not advance further.
func (s FileStatus) IsTerminal() bool {
	switch s {
	case StatusComplete, StatusFailed, StatusMismatch, StatusTagFailed:
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
	Source      string     `json:"source"`
	Destination string     `json:"destination,omitempty"`
	Checksum    string     `json:"checksum,omitempty"`
	Status      FileStatus `json:"status"`
	ExtractedAt *time.Time `json:"extracted_at,omitempty"`
	CopiedAt    *time.Time `json:"copied_at,omitempty"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`
	TaggedAt    *time.Time `json:"tagged_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// Manifest is the top-level operational journal written to
// dirB/.pixe/manifest.json. It is created at the start of a sort run
// and updated after each file completes a pipeline stage.
type Manifest struct {
	Version     int              `json:"version"`
	Source      string           `json:"source"`
	Destination string           `json:"destination"`
	Algorithm   string           `json:"algorithm"`
	StartedAt   time.Time        `json:"started_at"`
	Workers     int              `json:"workers"`
	Files       []*ManifestEntry `json:"files"`
}

// LedgerEntry is a minimal, immutable record of a successfully processed
// file. Written to dirA/.pixe_ledger.json after verification completes.
type LedgerEntry struct {
	Path        string    `json:"path"`        // relative path within dirA
	Checksum    string    `json:"checksum"`    // hex-encoded media payload hash
	Destination string    `json:"destination"` // relative path within dirB
	VerifiedAt  time.Time `json:"verified_at"`
}

// Ledger is the source-side record written to dirA/.pixe_ledger.json.
// It is the only file Pixe writes into the source directory.
type Ledger struct {
	Version     int           `json:"version"`
	PixeRun     time.Time     `json:"pixe_run"`
	Algorithm   string        `json:"algorithm"`
	Destination string        `json:"destination"`
	Files       []LedgerEntry `json:"files"`
}
