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

// Package pipeline implements the single-threaded sort orchestrator that
// wires together discovery, extraction, hashing, path building, copy,
// verify, tagging, and manifest/ledger persistence.
//
// Concurrency (worker pool) is added in Task 11. This package provides the
// sequential baseline that is correct by construction and also serves as the
// --workers=1 code path.
package pipeline

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/cwlls/pixe-go/internal/config"
	copypkg "github.com/cwlls/pixe-go/internal/copy"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/manifest"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
)

// SortOptions holds the resolved runtime options for a sort run.
// It is constructed by the CLI layer and passed into Run.
type SortOptions struct {
	Config       *config.AppConfig
	Hasher       *hash.Hasher
	Registry     *discovery.Registry
	RunTimestamp string // e.g. "20260306_103000"
	// Output is where progress lines are written. Defaults to os.Stdout.
	Output io.Writer
}

// SortResult summarises the outcome of a completed sort run.
type SortResult struct {
	Processed  int
	Duplicates int
	Skipped    int
	Errors     int
}

// Run executes the full sort pipeline sequentially.
//
// Pipeline per file:
//
//	discover → extract date → hash payload → dedup check → build path
//	→ copy → verify → tag (optional) → update manifest → update dedup index
//
// After all files: write ledger to dirA, finalize manifest.
// In dry-run mode the copy/verify/tag steps are skipped; everything else runs.
func Run(opts SortOptions) (SortResult, error) {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	cfg := opts.Config
	dirA := cfg.Source
	dirB := cfg.Destination

	// ------------------------------------------------------------------
	// 1. Initialise or resume the manifest.
	// ------------------------------------------------------------------
	m, err := manifest.Load(dirB)
	if err != nil {
		return SortResult{}, fmt.Errorf("pipeline: load manifest: %w", err)
	}
	if m == nil {
		m = &domain.Manifest{
			Version:     1,
			Source:      dirA,
			Destination: dirB,
			Algorithm:   opts.Hasher.Algorithm(),
			StartedAt:   time.Now().UTC(),
			Workers:     1,
		}
	}

	// Build a fast lookup: source path → existing manifest entry (for resume).
	existing := make(map[string]*domain.ManifestEntry, len(m.Files))
	for _, e := range m.Files {
		existing[e.Source] = e
	}

	// ------------------------------------------------------------------
	// 2. Build the dedup index from already-completed entries.
	// ------------------------------------------------------------------
	dedupIndex := make(map[string]string) // checksum → relative dest path
	for _, e := range m.Files {
		if e.Status == domain.StatusComplete && e.Checksum != "" {
			dedupIndex[e.Checksum] = e.Destination
		}
	}

	// ------------------------------------------------------------------
	// 3. Walk dirA.
	// ------------------------------------------------------------------
	discovered, skipped, err := discovery.Walk(dirA, opts.Registry)
	if err != nil {
		return SortResult{}, fmt.Errorf("pipeline: walk source: %w", err)
	}

	// Add newly-discovered files to the manifest as pending (skip already known).
	for _, df := range discovered {
		if _, known := existing[df.Path]; !known {
			entry := &domain.ManifestEntry{
				Source: df.Path,
				Status: domain.StatusPending,
			}
			m.Files = append(m.Files, entry)
			existing[df.Path] = entry
		}
	}
	if err := manifest.Save(m, dirB); err != nil {
		return SortResult{}, fmt.Errorf("pipeline: save initial manifest: %w", err)
	}

	// ------------------------------------------------------------------
	// 4. Process each discovered file.
	// ------------------------------------------------------------------
	var result SortResult
	result.Skipped = len(skipped)

	ledger := &domain.Ledger{
		Version:     1,
		PixeRun:     m.StartedAt,
		Algorithm:   opts.Hasher.Algorithm(),
		Destination: dirB,
	}

	for _, df := range discovered {
		entry := existing[df.Path]

		// Skip files already completed in a previous (resumed) run.
		if entry.Status == domain.StatusComplete {
			result.Processed++
			ledger.Files = append(ledger.Files, domain.LedgerEntry{
				Path:        relPath(dirA, df.Path),
				Checksum:    entry.Checksum,
				Destination: relPath(dirB, entry.Destination),
				VerifiedAt:  *entry.VerifiedAt,
			})
			continue
		}

		if err := processFile(df, entry, opts, cfg, dirA, dirB, dedupIndex, m, ledger, out); err != nil {
			entry.Status = domain.StatusFailed
			entry.Error = err.Error()
			saveManifest(m, dirB, out)
			result.Errors++
			fmt.Fprintf(out, "  ERROR  %s: %v\n", filepath.Base(df.Path), err)
			continue
		}

		if entry.Status == domain.StatusComplete {
			result.Processed++
			if _, isDup := dedupIndex[entry.Checksum]; isDup && strings.Contains(entry.Destination, "duplicates") {
				result.Duplicates++
			}
		}
	}

	// ------------------------------------------------------------------
	// 5. Finalise: write ledger to dirA, save final manifest.
	// ------------------------------------------------------------------
	if !cfg.DryRun {
		if err := manifest.SaveLedger(ledger, dirA); err != nil {
			fmt.Fprintf(out, "WARNING: could not write ledger to %s: %v\n", dirA, err)
		}
	}
	saveManifest(m, dirB, out)

	fmt.Fprintf(out, "\nDone. processed=%d duplicates=%d skipped=%d errors=%d\n",
		result.Processed, result.Duplicates, result.Skipped, result.Errors)

	return result, nil
}

// processFile runs the full pipeline for a single file, mutating entry and
// dedupIndex as it progresses.
func processFile(
	df discovery.DiscoveredFile,
	entry *domain.ManifestEntry,
	opts SortOptions,
	cfg *config.AppConfig,
	dirA, dirB string,
	dedupIndex map[string]string,
	m *domain.Manifest,
	ledger *domain.Ledger,
	out io.Writer,
) error {
	now := func() *time.Time { t := time.Now().UTC(); return &t }

	// --- Extract date ---
	captureDate, err := df.Handler.ExtractDate(df.Path)
	if err != nil {
		return fmt.Errorf("extract date: %w", err)
	}
	entry.Status = domain.StatusExtracted
	entry.ExtractedAt = now()

	// --- Hash media payload ---
	rc, err := df.Handler.HashableReader(df.Path)
	if err != nil {
		return fmt.Errorf("open hashable reader: %w", err)
	}
	checksum, err := opts.Hasher.Sum(rc)
	rc.Close()
	if err != nil {
		return fmt.Errorf("hash payload: %w", err)
	}
	entry.Checksum = checksum
	entry.Status = domain.StatusHashed

	// --- Dedup check ---
	_, isDuplicate := dedupIndex[checksum]

	// --- Build destination path ---
	ext := filepath.Ext(df.Path)
	relDest := pathbuilder.Build(captureDate, checksum, ext, isDuplicate, opts.RunTimestamp)
	absDest := filepath.Join(dirB, relDest)
	entry.Destination = absDest

	if cfg.DryRun {
		fmt.Fprintf(out, "  DRY-RUN  %s → %s\n", filepath.Base(df.Path), relDest)
		entry.Status = domain.StatusComplete
		return nil
	}

	// --- Copy ---
	fmt.Fprintf(out, "  COPY     %s → %s\n", filepath.Base(df.Path), relDest)
	if err := copypkg.Execute(df.Path, absDest); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	entry.Status = domain.StatusCopied
	entry.CopiedAt = now()

	// --- Verify ---
	vr := copypkg.Verify(absDest, checksum, df.Handler, opts.Hasher)
	if !vr.Success {
		entry.Status = domain.StatusMismatch
		return fmt.Errorf("verify: %w", vr.Error)
	}
	entry.Status = domain.StatusVerified
	entry.VerifiedAt = now()

	// --- Tag (optional) ---
	tags := resolveTags(cfg, captureDate)
	if !tags.IsEmpty() {
		if err := df.Handler.WriteMetadataTags(absDest, tags); err != nil {
			entry.Status = domain.StatusTagFailed
			entry.Error = err.Error()
			// Tag failure is non-fatal: file is copied and verified; we warn and continue.
			fmt.Fprintf(out, "  WARNING  tag failed for %s: %v\n", filepath.Base(df.Path), err)
		} else {
			entry.Status = domain.StatusTagged
			entry.TaggedAt = now()
		}
	}

	// --- Complete ---
	entry.Status = domain.StatusComplete

	// Update dedup index so subsequent files with the same checksum are routed correctly.
	if !isDuplicate {
		dedupIndex[checksum] = relDest
	}

	// Append to ledger.
	ledger.Files = append(ledger.Files, domain.LedgerEntry{
		Path:        relPath(dirA, df.Path),
		Checksum:    checksum,
		Destination: relDest,
		VerifiedAt:  *entry.VerifiedAt,
	})

	// Persist manifest after every file for crash safety.
	saveManifest(m, dirB, out)

	return nil
}

// resolveTags renders the Copyright template and returns a MetadataTags value.
func resolveTags(cfg *config.AppConfig, captureDate time.Time) domain.MetadataTags {
	tags := domain.MetadataTags{
		CameraOwner: cfg.CameraOwner,
	}
	if cfg.Copyright != "" {
		tags.Copyright = renderCopyright(cfg.Copyright, captureDate)
	}
	return tags
}

// copyrightData is the template context for Copyright rendering.
type copyrightData struct {
	Year int
}

// renderCopyright executes the copyright template string with the capture year.
// On template error the raw string is returned unchanged.
func renderCopyright(tmplStr string, date time.Time) string {
	tmpl, err := template.New("copyright").Parse(tmplStr)
	if err != nil {
		return tmplStr
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, copyrightData{Year: date.Year()}); err != nil {
		return tmplStr
	}
	return buf.String()
}

// saveManifest persists the manifest, printing a warning on failure.
func saveManifest(m *domain.Manifest, dirB string, out io.Writer) {
	if err := manifest.Save(m, dirB); err != nil {
		fmt.Fprintf(out, "WARNING: could not save manifest: %v\n", err)
	}
}

// relPath returns the path of target relative to base.
// Falls back to target if Rel fails.
func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}
