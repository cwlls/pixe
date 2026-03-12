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

// Package manifest handles persistence of the Pixe manifest and ledger files.
//
// Manifest — written to <dirB>/.pixe/manifest.json — is the legacy
// operational journal for a sort run. It is retained for the migration path
// only; new runs use the SQLite archive database instead.
//
// Ledger — written to <dirA>/.pixe_ledger.json — is the source-side receipt
// of every file Pixe processed during a run. It uses the JSONL format (v4):
// line 1 is a header object containing run metadata; each subsequent line is
// an independent LedgerEntry JSON object appended as the coordinator
// finalises each file result. This streaming approach keeps memory usage O(1)
// regardless of file count and leaves a partial-but-valid receipt if the run
// is interrupted.
//
// The LedgerWriter type owns the file handle and json.Encoder. The pipeline
// coordinator is the sole caller — no mutex is needed.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwlls/pixe-go/internal/domain"
)

const (
	// manifestDir is the subdirectory within dirB that holds Pixe metadata.
	manifestDir = ".pixe"
	// manifestFile is the filename of the operational journal.
	manifestFile = "manifest.json"
	// ledgerFile is the filename of the source-side ledger.
	ledgerFile = ".pixe_ledger.json"
)

// manifestPath returns the absolute path to the manifest file.
func manifestPath(dirB string) string {
	return filepath.Join(dirB, manifestDir, manifestFile)
}

// ledgerPath returns the absolute path to the ledger file.
func ledgerPath(dirA string) string {
	return filepath.Join(dirA, ledgerFile)
}

// Save atomically writes m to <dirB>/.pixe/manifest.json.
// It creates the .pixe directory if it does not exist.
// The write is atomic: content is written to a .tmp file first, then
// renamed over the target so a crash mid-write leaves the previous
// version intact.
func Save(m *domain.Manifest, dirB string) error {
	dir := filepath.Join(dirB, manifestDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("manifest: create directory %q: %w", dir, err)
	}
	return atomicWriteJSON(m, manifestPath(dirB))
}

// Load reads and deserialises the manifest from <dirB>/.pixe/manifest.json.
// Returns (nil, nil) if the manifest file does not exist — callers treat
// this as "no prior run".
func Load(dirB string) (*domain.Manifest, error) {
	path := manifestPath(dirB)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("manifest: read %q: %w", path, err)
	}
	var m domain.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: parse %q: %w", path, err)
	}
	return &m, nil
}

// LedgerContents holds the parsed contents of a JSONL ledger file.
// Used by tests to verify ledger output after a sort run.
type LedgerContents struct {
	Header  domain.LedgerHeader  // Header is the run-level metadata from the first line of the JSONL ledger.
	Entries []domain.LedgerEntry // Entries is the list of per-file outcome records from the ledger.
}

// LoadLedger reads a JSONL ledger file and returns its parsed contents.
// Line 1 is decoded as the LedgerHeader; all subsequent lines are decoded
// as LedgerEntry objects.
// Returns (nil, nil) if the ledger file does not exist.
func LoadLedger(dirA string) (*LedgerContents, error) {
	path := ledgerPath(dirA)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("manifest: open ledger %q: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	dec := json.NewDecoder(f)

	var lc LedgerContents
	if err := dec.Decode(&lc.Header); err != nil {
		return nil, fmt.Errorf("manifest: decode ledger header %q: %w", path, err)
	}
	for dec.More() {
		var entry domain.LedgerEntry
		if err := dec.Decode(&entry); err != nil {
			return nil, fmt.Errorf("manifest: decode ledger entry %q: %w", path, err)
		}
		lc.Entries = append(lc.Entries, entry)
	}
	return &lc, nil
}

// LedgerWriter streams ledger entries to a JSONL file.
// The pipeline coordinator goroutine is the sole caller — no mutex is needed.
// Call NewLedgerWriter to open the file and write the header, then WriteEntry
// for each file result, and finally Close when the run completes.
type LedgerWriter struct {
	f   *os.File
	enc *json.Encoder
}

// NewLedgerWriter opens <dirA>/.pixe_ledger.json for writing (truncating any
// existing content), writes header as the first JSON line, and returns the
// writer. The caller must call Close when the run completes.
//
// Returns an error if the file cannot be created or the header cannot be
// written. In either case the file is closed before returning.
func NewLedgerWriter(dirA string, header domain.LedgerHeader) (*LedgerWriter, error) {
	f, err := os.Create(ledgerPath(dirA))
	if err != nil {
		return nil, fmt.Errorf("manifest: open ledger: %w", err)
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false) // keep file paths with & < > unescaped

	if err := enc.Encode(header); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("manifest: write ledger header: %w", err)
	}

	return &LedgerWriter{f: f, enc: enc}, nil
}

// WriteEntry appends entry as a single compact JSON line to the ledger.
// If lw is nil (dry-run mode) the call is a no-op and returns nil.
func (lw *LedgerWriter) WriteEntry(entry domain.LedgerEntry) error {
	if lw == nil {
		return nil
	}
	return lw.enc.Encode(entry)
}

// Close flushes and closes the underlying file.
// If lw is nil the call is a no-op and returns nil.
func (lw *LedgerWriter) Close() error {
	if lw == nil {
		return nil
	}
	return lw.f.Close()
}

// atomicWriteJSON marshals v to indented JSON and writes it to target
// atomically via a sibling .tmp file and os.Rename.
func atomicWriteJSON(v any, target string) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest: marshal JSON: %w", err)
	}
	// Write to a temp file in the same directory so rename is same-filesystem.
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("manifest: write temp file %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		// Best-effort cleanup of the orphaned temp file.
		_ = os.Remove(tmp)
		return fmt.Errorf("manifest: rename %q → %q: %w", tmp, target, err)
	}
	return nil
}
