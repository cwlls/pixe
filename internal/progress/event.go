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

// Package progress provides the pipeline event bus — a structured, typed
// channel that decouples the sort and verify pipelines from their output
// presentation. The pipeline emits Event values; consumers (the CLI progress
// bar in internal/cli, or the plain-text writer) subscribe and render events
// in their own way.
//
// This package has zero external dependencies — it is pure Go stdlib.
// Charm/Bubble Tea dependencies are confined to the internal/cli package
// that consumes events for the --progress bar.
package progress

import "time"

// EventKind identifies the type of pipeline event.
type EventKind int

const (
	// Discovery phase events.
	EventDiscoverStart EventKind = iota // Walk began.
	EventDiscoverDone                   // Walk complete; Total and Skipped fields set.

	// Per-file lifecycle events.
	EventFileStart     // File processing began.
	EventFileExtracted // Date extracted from metadata.
	EventFileHashed    // Checksum computed over media payload.
	EventFileCopied    // Temp file written to destination.
	EventFileVerified  // Hash verified; temp file promoted to canonical path.
	EventFileTagged    // Metadata written (embedded or sidecar).
	EventFileComplete  // Terminal success state.
	EventFileDuplicate // File identified as a content duplicate.
	EventFileSkipped   // File skipped (previously imported or unsupported format).
	EventFileError     // File failed at some pipeline stage.

	// Sidecar events.
	EventSidecarCarried // Sidecar file copied alongside its parent.
	EventSidecarFailed  // Sidecar carry failed (non-fatal).

	// Run-level events.
	EventRunComplete // All files processed; Summary field populated.

	// Verify-specific events.
	EventVerifyStart        // Verify walk began; Total field set.
	EventVerifyOK           // File checksum matches filename-embedded checksum.
	EventVerifyMismatch     // File checksum does not match, or read error.
	EventVerifyUnrecognised // File not parseable by any registered handler.
	EventVerifyDone         // Verify walk complete; Summary field populated.
)

// Event is the universal pipeline event. Not all fields are populated for
// every EventKind — consumers should check Kind and read the relevant fields.
type Event struct {
	Kind      EventKind
	Timestamp time.Time

	// File identity.
	RelPath  string // relative path from dirA (sort) or dirB (verify).
	AbsPath  string // absolute path, for consumers that need filesystem access.
	WorkerID int    // which worker is handling this file; -1 for the coordinator.

	// Pipeline progress counters (updated on terminal events).
	Total     int // total files discovered (set on EventDiscoverDone / EventVerifyStart).
	Completed int // files in terminal states so far.
	Skipped   int // files skipped during discovery (set on EventDiscoverDone).

	// File outcome data (populated progressively through the pipeline).
	Checksum    string
	Destination string // relative path within dirB.
	CaptureDate time.Time
	FileSize    int64

	// Duplicate info (EventFileDuplicate).
	IsDuplicate bool
	MatchesDest string // dest_rel of the existing file this one matches.

	// Skip / error info.
	Reason string // human-readable skip reason or error message.
	Err    error  // underlying error (EventFileError, EventVerifyMismatch).

	// Verify-specific fields (EventVerifyMismatch).
	ExpectedChecksum string
	ActualChecksum   string

	// Sidecar info (EventSidecarCarried, EventSidecarFailed).
	SidecarRelPath string
	SidecarExt     string

	// Summary (EventRunComplete, EventVerifyDone).
	Summary *RunSummary
}

// RunSummary aggregates the final counts for a completed run.
// It is attached to EventRunComplete and EventVerifyDone events.
type RunSummary struct {
	// Sort-specific.
	Processed  int
	Duplicates int
	Skipped    int
	Errors     int
	Duration   time.Duration

	// Verify-specific.
	Verified     int
	Mismatches   int
	Unrecognised int
}
