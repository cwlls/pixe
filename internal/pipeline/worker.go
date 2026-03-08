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
	"path/filepath"
	"sync"
	"time"

	"github.com/cwlls/pixe-go/internal/archivedb"
	copypkg "github.com/cwlls/pixe-go/internal/copy"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
)

// syncWriter wraps an io.Writer with a mutex so multiple goroutines can
// safely write progress lines without interleaving.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (sw *syncWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

// workItem is sent from the coordinator to a worker.
type workItem struct {
	df     discovery.DiscoveredFile
	fileID int64 // archive DB row ID
}

// workResult is sent from a worker back to the coordinator after the
// extract+hash phase. The coordinator performs the dedup check (single-writer
// on the DB) and sends the resolved destination back via the assignCh channel.
type workResult struct {
	df       discovery.DiscoveredFile
	fileID   int64
	workerID int // which worker sent this result
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

// workerFinalResult is sent from a worker to the coordinator after
// copy+verify+tag completes.
type workerFinalResult struct {
	df          discovery.DiscoveredFile
	fileID      int64
	checksum    string
	relDest     string
	isDuplicate bool
	verifiedAt  time.Time
	err         error
}

// RunConcurrent executes the sort pipeline with N concurrent workers.
//
// Architecture:
//
//	coordinator goroutine:
//	  - feeds workItems into workCh
//	  - receives workResults from resultCh (extract+hash done)
//	  - performs dedup check via db.CheckDuplicate (single-writer)
//	  - sends destAssignment back to the worker via per-worker assignCh
//	  - receives finalResults from doneCh (copy+verify+tag done)
//	  - calls db.UpdateFileStatus("complete") per file
//
//	worker goroutines (N):
//	  - pull workItems from workCh
//	  - extract date + hash payload → db.UpdateFileStatus("hashed")
//	  - send workResult to resultCh
//	  - wait for destAssignment on their assignCh
//	  - copy + verify + tag → db.UpdateFileStatus per stage
//	  - send final result to doneCh
func RunConcurrent(opts SortOptions, discovered []discovery.DiscoveredFile,
	fileIDs map[string]int64, dirA, dirB string, out io.Writer, ledger *domain.Ledger) SortResult {

	ctx := context.Background()
	return runConcurrentCtx(ctx, opts, discovered, fileIDs, dirA, dirB, out, ledger)
}

// runConcurrentCtx is the context-aware implementation, used by tests.
func runConcurrentCtx(ctx context.Context, opts SortOptions, discovered []discovery.DiscoveredFile,
	fileIDs map[string]int64, dirA, dirB string, out io.Writer, ledger *domain.Ledger) SortResult {

	// Wrap out in a mutex so concurrent workers don't race on writes.
	sw := &syncWriter{w: out}
	out = sw

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
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runWorker(ctx, id, workCh, resultCh, assignChs[id], doneCh, opts, dirB, out)
		}(i)
	}
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// Feed work items.
	go func() {
		for _, df := range discovered {
			select {
			case <-ctx.Done():
				close(workCh)
				return
			case workCh <- workItem{df: df, fileID: fileIDs[df.Path]}:
			}
		}
		close(workCh)
	}()

	pendingCount := len(discovered)
	completed := 0
	var result SortResult

	for completed < pendingCount {
		select {
		case <-ctx.Done():
			goto done

		case wr, ok := <-resultCh:
			if !ok {
				goto done
			}
			if wr.err != nil {
				if opts.DB != nil {
					_ = opts.DB.UpdateFileStatus(wr.fileID, "failed",
						archivedb.WithError(wr.err.Error()))
				}
				result.Errors++
				_, _ = fmt.Fprintf(out, "  ERROR  %s: %v\n", filepath.Base(wr.df.Path), wr.err)
				completed++
				continue
			}

			// Dedup check — single-writer on the DB.
			var isDuplicate bool
			if opts.DB != nil {
				existingDest, err := opts.DB.CheckDuplicate(wr.checksum)
				if err != nil {
					_ = opts.DB.UpdateFileStatus(wr.fileID, "failed", archivedb.WithError(err.Error()))
					result.Errors++
					completed++
					continue
				}
				isDuplicate = existingDest != ""
			}

			relDest := pathbuilder.Build(wr.date, wr.checksum, wr.ext, isDuplicate, opts.RunTimestamp)
			absDest := filepath.Join(dirB, relDest)

			assignChs[wr.workerID] <- destAssignment{
				absDest:     absDest,
				relDest:     relDest,
				isDuplicate: isDuplicate,
			}

		case fr, ok := <-doneCh:
			if !ok {
				goto done
			}
			if fr.err != nil {
				result.Errors++
				_, _ = fmt.Fprintf(out, "  ERROR  %s: %v\n", filepath.Base(fr.df.Path), fr.err)
			} else {
				if opts.DB != nil {
					_ = opts.DB.UpdateFileStatus(fr.fileID, "complete",
						archivedb.WithIsDuplicate(fr.isDuplicate))
				}
				result.Processed++
				if fr.isDuplicate {
					result.Duplicates++
				}
				ledger.Files = append(ledger.Files, domain.LedgerEntry{
					Path:        relPath(dirA, fr.df.Path),
					Checksum:    fr.checksum,
					Destination: fr.relDest,
					VerifiedAt:  fr.verifiedAt,
				})
			}
			completed++
		}
	}

done:
	return result
}

// runWorker is the per-worker goroutine: extract+hash, wait for dest, copy+verify+tag.
func runWorker(ctx context.Context, id int,
	workCh <-chan workItem,
	resultCh chan<- workResult,
	assignCh <-chan destAssignment,
	doneCh chan<- workerFinalResult,
	opts SortOptions,
	dirB string,
	out io.Writer,
) {
	db := opts.DB

	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-workCh:
			if !ok {
				return
			}

			// --- Extract date ---
			captureDate, err := item.df.Handler.ExtractDate(item.df.Path)
			if err != nil {
				err = fmt.Errorf("extract date: %w", err)
				// Send error via resultCh; coordinator will count it and not expect a doneCh message.
				resultCh <- workResult{df: item.df, fileID: item.fileID, workerID: id, err: err}
				continue
			}
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "extracted",
					archivedb.WithCaptureDate(captureDate))
			}

			// --- Hash ---
			rc, err := item.df.Handler.HashableReader(item.df.Path)
			if err != nil {
				err = fmt.Errorf("hash reader: %w", err)
				resultCh <- workResult{df: item.df, fileID: item.fileID, workerID: id, err: err}
				continue
			}
			checksum, err := opts.Hasher.Sum(rc)
			_ = rc.Close()
			if err != nil {
				err = fmt.Errorf("hash: %w", err)
				resultCh <- workResult{df: item.df, fileID: item.fileID, workerID: id, err: err}
				continue
			}
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "hashed", archivedb.WithChecksum(checksum))
			}

			ext := filepath.Ext(item.df.Path)

			// Send extract+hash result to coordinator for dedup decision.
			resultCh <- workResult{
				df:       item.df,
				fileID:   item.fileID,
				workerID: id,
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
				if db != nil {
					_ = db.UpdateFileStatus(item.fileID, "complete",
						archivedb.WithDestination(assign.absDest, assign.relDest),
						archivedb.WithIsDuplicate(assign.isDuplicate))
				}
				doneCh <- workerFinalResult{
					df: item.df, fileID: item.fileID,
					checksum: checksum, relDest: assign.relDest,
					isDuplicate: assign.isDuplicate,
					verifiedAt:  time.Now().UTC(),
				}
				continue
			}

			// --- Copy ---
			_, _ = fmt.Fprintf(out, "  COPY     %s → %s\n", filepath.Base(item.df.Path), assign.relDest)
			if err := copypkg.Execute(item.df.Path, assign.absDest); err != nil {
				ferr := fmt.Errorf("copy: %w", err)
				if db != nil {
					_ = db.UpdateFileStatus(item.fileID, "failed", archivedb.WithError(ferr.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: ferr}
				continue
			}
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "copied",
					archivedb.WithDestination(assign.absDest, assign.relDest))
			}

			// --- Verify ---
			vr := copypkg.Verify(assign.absDest, checksum, item.df.Handler, opts.Hasher)
			if !vr.Success {
				if db != nil {
					_ = db.UpdateFileStatus(item.fileID, "mismatch", archivedb.WithError(vr.Error.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: vr.Error}
				continue
			}
			verifiedAt := time.Now().UTC()
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "verified")
			}

			// --- Tag ---
			tags := resolveTags(opts.Config, captureDate)
			if !tags.IsEmpty() {
				if err := item.df.Handler.WriteMetadataTags(assign.absDest, tags); err != nil {
					if db != nil {
						_ = db.UpdateFileStatus(item.fileID, "tag_failed", archivedb.WithError(err.Error()))
					}
					_, _ = fmt.Fprintf(out, "  WARNING  tag failed for %s: %v\n",
						filepath.Base(item.df.Path), err)
				} else {
					if db != nil {
						_ = db.UpdateFileStatus(item.fileID, "tagged")
					}
				}
			}

			doneCh <- workerFinalResult{
				df:          item.df,
				fileID:      item.fileID,
				checksum:    checksum,
				relDest:     assign.relDest,
				isDuplicate: assign.isDuplicate,
				verifiedAt:  verifiedAt,
			}
		}
	}
}
