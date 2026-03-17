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
// of every file Pixe processed. It uses a cumulative JSONL format (v6):
// each run appends a header line followed by per-file LedgerEntry lines.
// Multiple runs therefore produce a single growing file with interleaved
// headers and entries. LoadLedger parses all runs and returns them grouped
// by run. This streaming approach keeps memory usage O(1) regardless of file
// count and leaves a partial-but-valid receipt if a run is interrupted.
//
// The LedgerWriter type owns the file handle and json.Encoder. The pipeline
// coordinator is the sole caller — no mutex is needed.
package manifest

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/cwlls/pixe/internal/domain"
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

// LedgerRun groups a single run's header with its per-file entries.
type LedgerRun struct {
	Header  domain.LedgerHeader  // Header is the run-level metadata written at the start of the run.
	Entries []domain.LedgerEntry // Entries is the list of per-file outcome records for this run.
}

// LedgerContents holds all runs parsed from a cumulative JSONL ledger file.
// Since v6 the ledger appends across runs, so a single file may contain
// multiple runs. Pre-v6 (single-run) ledgers are returned as one LedgerRun.
type LedgerContents struct {
	Runs []LedgerRun
}

// LatestRun returns the most recent run (last in file order), or nil if the
// ledger is empty.
func (lc *LedgerContents) LatestRun() *LedgerRun {
	if lc == nil || len(lc.Runs) == 0 {
		return nil
	}
	return &lc.Runs[len(lc.Runs)-1]
}

// LatestHeader returns the header of the most recent run, or a zero-value
// header if the ledger is empty.
func (lc *LedgerContents) LatestHeader() domain.LedgerHeader {
	if r := lc.LatestRun(); r != nil {
		return r.Header
	}
	return domain.LedgerHeader{}
}

// AllEntries returns all entries across all runs in file order (oldest first).
// A file that appears in multiple runs will have multiple entries — callers
// that want the most recent outcome per path should iterate in order and let
// later entries overwrite earlier ones in a map.
func (lc *LedgerContents) AllEntries() []domain.LedgerEntry {
	if lc == nil {
		return nil
	}
	var total int
	for i := range lc.Runs {
		total += len(lc.Runs[i].Entries)
	}
	out := make([]domain.LedgerEntry, 0, total)
	for i := range lc.Runs {
		out = append(out, lc.Runs[i].Entries...)
	}
	return out
}

// LoadLedger reads a cumulative JSONL ledger file and returns all runs it
// contains. Each run starts with a header line (identified by the presence
// of the "version" key) followed by zero or more entry lines.
//
// Malformed lines (e.g. from an interrupted write) are silently skipped so
// that a partial run does not prevent reading subsequent complete runs.
//
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

	lc := &LedgerContents{}
	scanner := bufio.NewScanner(f)
	// Increase the scanner buffer for very long lines (large sidecar lists etc.).
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Probe for the "version" key to distinguish headers from entries.
		// We use a minimal struct rather than json.RawMessage + map to avoid
		// allocating a full map for every line.
		var probe struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			// Malformed JSON — skip (e.g. truncated line from interrupted run).
			continue
		}

		if probe.Version > 0 {
			// Header line — start a new run.
			var h domain.LedgerHeader
			if err := json.Unmarshal(line, &h); err != nil {
				// Malformed header — skip.
				continue
			}
			lc.Runs = append(lc.Runs, LedgerRun{Header: h})
		} else {
			// Entry line — append to the current run.
			var e domain.LedgerEntry
			if err := json.Unmarshal(line, &e); err != nil {
				// Malformed entry — skip.
				continue
			}
			if len(lc.Runs) == 0 {
				// Entry before any header (should not happen in well-formed
				// files, but handle gracefully by creating an implicit run).
				lc.Runs = append(lc.Runs, LedgerRun{})
			}
			lc.Runs[len(lc.Runs)-1].Entries = append(lc.Runs[len(lc.Runs)-1].Entries, e)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("manifest: read ledger %q: %w", path, err)
	}
	return lc, nil
}

// LedgerWriter streams ledger entries to a JSONL file.
// The pipeline coordinator goroutine is the sole caller — no mutex is needed.
// Call NewLedgerWriter to open the file and write the header, then WriteEntry
// for each file result, and finally Close when the run completes.
type LedgerWriter struct {
	f   *os.File
	enc *json.Encoder
}

// NewLedgerWriter opens <dirA>/.pixe_ledger.json for appending (creating it
// if it does not exist), writes header as a JSON separator line, and returns
// the writer. The caller must call Close when the run completes.
//
// Append semantics mean each run adds its header and entries after the
// previous run's content, producing a cumulative multi-run ledger.
//
// Returns an error if the file cannot be opened or the header cannot be
// written. In either case the file is closed before returning.
func NewLedgerWriter(dirA string, header domain.LedgerHeader) (*LedgerWriter, error) {
	f, err := os.OpenFile(ledgerPath(dirA), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
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

// Close syncs and closes the underlying file.
// Sync is called before Close to ensure all buffered writes are flushed to
// stable storage — without it a power failure between the last WriteEntry
// and process exit could lose recent ledger entries.
// If lw is nil the call is a no-op and returns nil.
func (lw *LedgerWriter) Close() error {
	if lw == nil {
		return nil
	}
	if err := lw.f.Sync(); err != nil {
		_ = lw.f.Close()
		return fmt.Errorf("manifest: sync ledger: %w", err)
	}
	return lw.f.Close()
}

// SafeLedgerWriter wraps a LedgerWriter and tracks write health.
// On the first write error it logs a single warning to the provided io.Writer
// and suppresses subsequent per-entry warnings. Ledger failure is non-fatal —
// the archive database is the primary record; the ledger is the user-facing
// audit trail.
//
// SafeLedgerWriter is safe for concurrent use from multiple goroutines.
type SafeLedgerWriter struct {
	lw     *LedgerWriter
	out    io.Writer
	mu     sync.Mutex
	failed bool
}

// NewSafeLedgerWriter wraps lw with first-failure error tracking. Warnings
// are written to out. If lw is nil (dry-run mode) all writes are no-ops.
func NewSafeLedgerWriter(lw *LedgerWriter, out io.Writer) *SafeLedgerWriter {
	return &SafeLedgerWriter{lw: lw, out: out}
}

// WriteEntry delegates to the underlying LedgerWriter. On the first error,
// a warning is printed to out. Subsequent errors are silently absorbed.
// The full operation is serialized under mu so concurrent callers cannot
// interleave writes into the underlying json.Encoder.
func (sw *SafeLedgerWriter) WriteEntry(entry domain.LedgerEntry) {
	if sw == nil || sw.lw == nil {
		return
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if err := sw.lw.WriteEntry(entry); err != nil {
		if !sw.failed {
			sw.failed = true
			_, _ = fmt.Fprintf(sw.out,
				"WARNING: ledger write failed (further warnings suppressed): %v\n", err)
		}
	}
}

// Close flushes and closes the underlying LedgerWriter.
// If sw is nil the call is a no-op and returns nil.
// mu is held to prevent a race between a final WriteEntry and Close.
func (sw *SafeLedgerWriter) Close() error {
	if sw == nil || sw.lw == nil {
		return nil
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.lw.Close()
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
