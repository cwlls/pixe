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

package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	copypkg "github.com/cwlls/pixe-go/internal/copy"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/manifest"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
)

// workItem is sent from the coordinator to a worker.
type workItem struct {
	df    discovery.DiscoveredFile
	entry *domain.ManifestEntry
}

// workResult is sent from a worker back to the coordinator after the
// extract+hash phase. The coordinator performs the dedup check (single-writer
// on the dedup index) and then sends the resolved destination back to the
// worker via the assignCh channel.
type workResult struct {
	df       discovery.DiscoveredFile
	entry    *domain.ManifestEntry
	checksum string
	date     time.Time
	ext      string
	err      error
}

// destAssignment is sent from the coordinator to the worker after the dedup
// decision has been made.
type destAssignment struct {
	absDest     string
	relDest     string
	isDuplicate bool
}

// RunConcurrent executes the sort pipeline with N concurrent workers.
// It is called by Run when opts.Config.Workers > 1.
//
// Architecture:
//
//	coordinator goroutine:
//	  - feeds workItems into workCh
//	  - receives workResults from resultCh (extract+hash done)
//	  - performs dedup check (single-writer on dedupIndex)
//	  - sends destAssignment back to the worker via per-worker assignCh
//	  - receives finalResults from doneCh (copy+verify+tag done)
//	  - updates manifest and dedup index
//
//	worker goroutines (N):
//	  - pull workItems from workCh
//	  - extract date + hash payload
//	  - send workResult to resultCh
//	  - wait for destAssignment on their assignCh
//	  - copy + verify + tag
//	  - send final result to doneCh
func RunConcurrent(ctx context.Context, opts SortOptions, discovered []discovery.DiscoveredFile,
	existing map[string]*domain.ManifestEntry, dedupIndex map[string]string,
	m *domain.Manifest, dirA, dirB string, out io.Writer) SortResult {

	n := opts.Config.Workers
	if n < 1 {
		n = 1
	}

	workCh := make(chan workItem, n*2)
	resultCh := make(chan workResult, n*2)
	doneCh := make(chan workerFinalResult, n*2)

	// Per-worker assignment channels (one per worker, buffered 1).
	assignChs := make([]chan destAssignment, n)
	for i := range assignChs {
		assignChs[i] = make(chan destAssignment, 1)
	}

	var wg sync.WaitGroup

	// Start N workers.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runWorker(ctx, id, workCh, resultCh, assignChs[id], doneCh, opts, dirB, out)
		}(i)
	}

	// Close doneCh when all workers finish.
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	var result SortResult
	result.Skipped = 0

	ledger := &domain.Ledger{
		Version:     1,
		PixeVersion: m.PixeVersion,
		PixeRun:     m.StartedAt,
		Algorithm:   opts.Hasher.Algorithm(),
		Destination: dirB,
	}

	// Feed work and coordinate in a single goroutine.
	go func() {
		for _, df := range discovered {
			entry := existing[df.Path]
			if entry.Status == domain.StatusComplete {
				continue
			}
			select {
			case <-ctx.Done():
				close(workCh)
				return
			case workCh <- workItem{df: df, entry: entry}:
			}
		}
		close(workCh)
	}()

	// Track which worker is waiting for which assignment.
	pendingAssign := make(map[string]int) // df.Path → worker id
	workerID := 0

	// Drain resultCh (extract+hash done) and doneCh (copy+verify done).
	pendingCount := 0
	for _, df := range discovered {
		if existing[df.Path].Status != domain.StatusComplete {
			pendingCount++
		}
	}

	completed := 0
	for completed < pendingCount {
		select {
		case <-ctx.Done():
			goto done

		case wr, ok := <-resultCh:
			if !ok {
				goto done
			}
			if wr.err != nil {
				wr.entry.Status = domain.StatusFailed
				wr.entry.Error = wr.err.Error()
				saveManifest(m, dirB, out)
				result.Errors++
				_, _ = fmt.Fprintf(out, "  ERROR  %s: %v\n", filepath.Base(wr.df.Path), wr.err)
				completed++
				continue
			}
			// Dedup check (single-writer).
			_, isDuplicate := dedupIndex[wr.checksum]
			relDest := pathbuilder.Build(wr.date, wr.checksum, wr.ext, isDuplicate, opts.RunTimestamp)
			absDest := filepath.Join(dirB, relDest)
			wr.entry.Destination = absDest

			wid := pendingAssign[wr.df.Path]
			assignChs[wid] <- destAssignment{
				absDest:     absDest,
				relDest:     relDest,
				isDuplicate: isDuplicate,
			}

		case fr, ok := <-doneCh:
			if !ok {
				goto done
			}
			_ = workerID
			if fr.err != nil {
				fr.entry.Status = domain.StatusFailed
				fr.entry.Error = fr.err.Error()
				result.Errors++
				_, _ = fmt.Fprintf(out, "  ERROR  %s: %v\n", filepath.Base(fr.df.Path), fr.err)
			} else {
				fr.entry.Status = domain.StatusComplete
				if fr.isDuplicate {
					result.Duplicates++
				}
				result.Processed++
				if _, alreadyIndexed := dedupIndex[fr.checksum]; !alreadyIndexed {
					dedupIndex[fr.checksum] = fr.relDest
				}
				ledger.Files = append(ledger.Files, domain.LedgerEntry{
					Path:        relPath(dirA, fr.df.Path),
					Checksum:    fr.checksum,
					Destination: fr.relDest,
					VerifiedAt:  time.Now().UTC(),
				})
			}
			saveManifest(m, dirB, out)
			completed++
		}
	}

done:
	// Write ledger and final manifest.
	if !opts.Config.DryRun {
		if err := manifest.SaveLedger(ledger, dirA); err != nil {
			_, _ = fmt.Fprintf(out, "WARNING: could not write ledger: %v\n", err)
		}
	}
	saveManifest(m, dirB, out)

	_ = pendingAssign
	return result
}

type workerFinalResult struct {
	df          discovery.DiscoveredFile
	entry       *domain.ManifestEntry
	checksum    string
	relDest     string
	isDuplicate bool
	err         error
}

func runWorker(ctx context.Context, id int,
	workCh <-chan workItem,
	resultCh chan<- workResult,
	assignCh <-chan destAssignment,
	doneCh chan<- workerFinalResult,
	opts SortOptions,
	dirB string,
	out io.Writer,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-workCh:
			if !ok {
				return
			}

			now := func() *time.Time { t := time.Now().UTC(); return &t }

			// --- Extract date ---
			captureDate, err := item.df.Handler.ExtractDate(item.df.Path)
			if err != nil {
				resultCh <- workResult{df: item.df, entry: item.entry, err: fmt.Errorf("extract date: %w", err)}
				doneCh <- workerFinalResult{df: item.df, entry: item.entry, err: fmt.Errorf("extract date: %w", err)}
				continue
			}
			item.entry.ExtractedAt = now()

			// --- Hash ---
			rc, err := item.df.Handler.HashableReader(item.df.Path)
			if err != nil {
				resultCh <- workResult{df: item.df, entry: item.entry, err: fmt.Errorf("hash reader: %w", err)}
				doneCh <- workerFinalResult{df: item.df, entry: item.entry, err: fmt.Errorf("hash reader: %w", err)}
				continue
			}
			checksum, err := opts.Hasher.Sum(rc)
			_ = rc.Close()
			if err != nil {
				resultCh <- workResult{df: item.df, entry: item.entry, err: fmt.Errorf("hash: %w", err)}
				doneCh <- workerFinalResult{df: item.df, entry: item.entry, err: fmt.Errorf("hash: %w", err)}
				continue
			}
			item.entry.Checksum = checksum
			item.entry.Status = domain.StatusHashed

			ext := filepath.Ext(item.df.Path)

			// Send extract+hash result to coordinator for dedup decision.
			resultCh <- workResult{
				df:       item.df,
				entry:    item.entry,
				checksum: checksum,
				date:     captureDate,
				ext:      ext,
			}

			// Wait for destination assignment from coordinator.
			var assign destAssignment
			select {
			case <-ctx.Done():
				return
			case assign = <-assignCh:
			}

			if opts.Config.DryRun {
				_, _ = fmt.Fprintf(out, "  DRY-RUN  %s → %s\n", filepath.Base(item.df.Path), assign.relDest)
				item.entry.Status = domain.StatusComplete
				doneCh <- workerFinalResult{
					df: item.df, entry: item.entry,
					checksum: checksum, relDest: assign.relDest,
					isDuplicate: assign.isDuplicate,
				}
				continue
			}

			// --- Copy ---
			_, _ = fmt.Fprintf(out, "  COPY     %s → %s\n", filepath.Base(item.df.Path), assign.relDest)
			if err := copypkg.Execute(item.df.Path, assign.absDest); err != nil {
				doneCh <- workerFinalResult{df: item.df, entry: item.entry, err: fmt.Errorf("copy: %w", err)}
				continue
			}
			item.entry.CopiedAt = now()
			item.entry.Status = domain.StatusCopied

			// --- Verify ---
			vr := copypkg.Verify(assign.absDest, checksum, item.df.Handler, opts.Hasher)
			if !vr.Success {
				item.entry.Status = domain.StatusMismatch
				doneCh <- workerFinalResult{df: item.df, entry: item.entry, err: vr.Error}
				continue
			}
			item.entry.VerifiedAt = now()
			item.entry.Status = domain.StatusVerified

			// --- Tag ---
			tags := resolveTags(opts.Config, captureDate)
			if !tags.IsEmpty() {
				if err := item.df.Handler.WriteMetadataTags(assign.absDest, tags); err != nil {
					item.entry.Status = domain.StatusTagFailed
					_, _ = fmt.Fprintf(out, "  WARNING  tag failed for %s: %v\n", filepath.Base(item.df.Path), err)
				} else {
					item.entry.TaggedAt = now()
					item.entry.Status = domain.StatusTagged
				}
			}

			item.entry.Status = domain.StatusComplete
			doneCh <- workerFinalResult{
				df: item.df, entry: item.entry,
				checksum: checksum, relDest: assign.relDest,
				isDuplicate: assign.isDuplicate,
			}
		}
	}
}

// RunWithWorkers is the entry point that selects sequential vs concurrent
// execution based on the worker count.
func RunWithWorkers(ctx context.Context, opts SortOptions, discovered []discovery.DiscoveredFile,
	existing map[string]*domain.ManifestEntry, dedupIndex map[string]string,
	m *domain.Manifest, dirA, dirB string, out io.Writer) SortResult {

	if opts.Config.Workers <= 1 {
		// Sequential path — reuse existing processFile loop.
		return runSequential(ctx, opts, discovered, existing, dedupIndex, m, dirA, dirB, out)
	}
	return RunConcurrent(ctx, opts, discovered, existing, dedupIndex, m, dirA, dirB, out)
}

// runSequential is the single-threaded path, extracted from Run for reuse.
func runSequential(_ context.Context, opts SortOptions, discovered []discovery.DiscoveredFile,
	existing map[string]*domain.ManifestEntry, dedupIndex map[string]string,
	m *domain.Manifest, dirA, dirB string, out io.Writer) SortResult {

	var result SortResult
	ledger := &domain.Ledger{
		Version:     1,
		PixeVersion: m.PixeVersion,
		PixeRun:     m.StartedAt,
		Algorithm:   opts.Hasher.Algorithm(),
		Destination: dirB,
	}

	for _, df := range discovered {
		entry := existing[df.Path]
		if entry.Status == domain.StatusComplete {
			result.Processed++
			if entry.VerifiedAt != nil {
				ledger.Files = append(ledger.Files, domain.LedgerEntry{
					Path:        relPath(dirA, df.Path),
					Checksum:    entry.Checksum,
					Destination: relPath(dirB, entry.Destination),
					VerifiedAt:  *entry.VerifiedAt,
				})
			}
			continue
		}

		if err := processFile(df, entry, opts, opts.Config, dirA, dirB, dedupIndex, m, ledger, out); err != nil {
			entry.Status = domain.StatusFailed
			entry.Error = err.Error()
			saveManifest(m, dirB, out)
			result.Errors++
			_, _ = fmt.Fprintf(out, "  ERROR  %s: %v\n", filepath.Base(df.Path), err)
			continue
		}
		if entry.Status == domain.StatusComplete {
			result.Processed++
			if _, isDup := dedupIndex[entry.Checksum]; isDup && containsStr(entry.Destination, "duplicates") {
				result.Duplicates++
			}
		}
	}

	if !opts.Config.DryRun {
		if err := manifest.SaveLedger(ledger, dirA); err != nil {
			_, _ = fmt.Fprintf(out, "WARNING: could not write ledger: %v\n", err)
		}
	}
	saveManifest(m, dirB, out)
	return result
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure os is imported for Stdout reference in worker.
var _ = os.Stdout
