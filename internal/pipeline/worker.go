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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cwlls/pixe/internal/archivedb"
	copypkg "github.com/cwlls/pixe/internal/copy"
	"github.com/cwlls/pixe/internal/discovery"
	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/manifest"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/progress"
	"github.com/cwlls/pixe/internal/tagging"
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
	absDest               string
	relDest               string
	isDuplicate           bool
	existingDestForLedger string // non-empty when isDuplicate is true
	skipCopy              bool   // true when --skip-duplicates is set and file is a duplicate
}

// workerFinalResult is sent from a worker to the coordinator after
// copy+verify+tag completes (or after a skip-copy no-op).
type workerFinalResult struct {
	df                    discovery.DiscoveredFile
	fileID                int64
	checksum              string
	relDest               string
	isDuplicate           bool
	existingDestForLedger string
	verifiedAt            time.Time
	captureDate           time.Time // used by coordinator to resolve copyright year in sidecar lines
	skipCopy              bool      // true when the worker skipped I/O due to --skip-duplicates
	carriedSidecarRels    []string  // dest_rel paths of successfully carried sidecars
	err                   error
	dbErr                 error // non-nil when a terminal DB status update failed in the worker
}

// RunConcurrent executes the sort pipeline with N concurrent workers.
//
// Architecture:
//
//	coordinator goroutine:
//	  - emits SKIP lines for discovery-phase skips
//	  - checks previously-imported files and emits SKIP lines
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
	skipped []discovery.SkippedFile,
	fileIDs map[string]int64, dirA, dirB string, out io.Writer, lw *manifest.SafeLedgerWriter, fmtr *Formatter) SortResult {

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	destLabel := opts.DestLabel
	return runConcurrentCtx(ctx, opts, discovered, skipped, fileIDs, dirA, dirB, out, lw, fmtr, destLabel)
}

// runConcurrentCtx is the context-aware implementation, used by tests.
func runConcurrentCtx(ctx context.Context, opts SortOptions, discovered []discovery.DiscoveredFile,
	skipped []discovery.SkippedFile,
	fileIDs map[string]int64, dirA, dirB string, out io.Writer, lw *manifest.SafeLedgerWriter, fmtr *Formatter, destLabel string) SortResult {

	// Wrap out in a mutex so concurrent workers don't race on writes.
	sw := &syncWriter{w: out}
	out = sw

	n := opts.Config.Workers
	if n < 1 {
		n = 1
	}

	db := opts.DB
	var result SortResult

	// --- Emit SKIP lines for discovery-phase skips (unsupported, dotfiles, etc.) ---
	for _, sf := range skipped {
		if opts.Config.Verbosity >= 0 {
			_, _ = fmt.Fprint(out, fmtr.FormatOutput("SKIP", sf.Path, sf.Reason))
		}
		lw.WriteEntry(domain.LedgerEntry{
			Path:   sf.Path,
			Status: domain.LedgerStatusSkip,
			Reason: sf.Reason,
		})
		if db != nil {
			skipFileID, insertErr := db.InsertFile(&archivedb.FileRecord{
				RunID:      opts.RunID,
				SourcePath: filepath.Join(dirA, sf.Path),
			})
			if insertErr == nil {
				if dbErr := db.UpdateFileStatus(skipFileID, "skipped",
					archivedb.WithSkipReason(sf.Reason)); dbErr != nil {
					_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", sf.Path, dbErr)))
				}
			}
		}
		result.Skipped++
		emit(opts.EventBus, progress.Event{
			Kind:      progress.EventFileSkipped,
			RelPath:   sf.Path,
			Reason:    sf.Reason,
			WorkerID:  -1,
			Completed: result.Skipped + result.Errors,
		})
	}

	// --- Filter out previously-imported files before feeding workers ---
	// actualDiscovered holds files that need real processing.
	actualDiscovered := make([]discovery.DiscoveredFile, 0, len(discovered))
	for _, df := range discovered {
		if db != nil {
			processed, checkErr := db.CheckSourceProcessed(df.Path)
			if checkErr != nil {
				if opts.Config.Verbosity >= 0 {
					_, _ = fmt.Fprint(out, fmtr.FormatOutput("ERR ", df.RelPath, checkErr.Error()))
				}
				result.Errors++
				emit(opts.EventBus, progress.Event{
					Kind:      progress.EventFileError,
					RelPath:   df.RelPath,
					Reason:    checkErr.Error(),
					Err:       checkErr,
					WorkerID:  -1,
					Completed: result.Skipped + result.Processed + result.Errors,
				})
				continue
			}
			if processed {
				const reason = "previously imported"
				if opts.Config.Verbosity >= 0 {
					_, _ = fmt.Fprint(out, fmtr.FormatOutput("SKIP", df.RelPath, reason))
				}
				lw.WriteEntry(domain.LedgerEntry{
					Path:   df.RelPath,
					Status: domain.LedgerStatusSkip,
					Reason: reason,
				})
				fileID := fileIDs[df.Path]
				if dbErr := db.UpdateFileStatus(fileID, "skipped",
					archivedb.WithSkipReason(reason)); dbErr != nil {
					_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", df.RelPath, dbErr)))
				}
				result.Skipped++
				emit(opts.EventBus, progress.Event{
					Kind:      progress.EventFileSkipped,
					RelPath:   df.RelPath,
					Reason:    reason,
					WorkerID:  -1,
					Completed: result.Skipped + result.Processed + result.Errors,
				})
				continue
			}
		}
		actualDiscovered = append(actualDiscovered, df)
	}

	// Channel buffer sizing — deadlock safety analysis:
	//
	// The coordination protocol has four channel hops per file:
	//   workCh (coordinator→worker) → resultCh (worker→coordinator) →
	//   assignChs[i] (coordinator→worker, per-worker) → doneCh (worker→coordinator)
	//
	// Worst-case scenario: all N workers simultaneously complete extract+hash
	// and send to resultCh before the coordinator processes any of them.
	// resultCh is buffered at n*2, so all N sends succeed without blocking.
	// Workers then block on assignChs[i] (buffer 1). The coordinator's select
	// reads from resultCh (not doneCh), processes the dedup check, and sends
	// to assignChs[i]. Each worker unblocks, performs copy+verify+tag, and
	// sends to doneCh (buffer n*2). At most N items are ever pending in
	// resultCh simultaneously, and n*2 > N, so the buffer never fills.
	// Therefore no deadlock is possible under any load level.
	//
	// The ctx.Done() case in the coordinator's select ensures that a
	// cancellation signal does not leave workers blocked on assignChs[i]:
	// workers also select on ctx.Done() when waiting for their assignment.
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
			runWorker(ctx, id, workCh, resultCh, assignChs[id], doneCh, opts, dirB, out, fmtr)
		}(i)
	}
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// Feed work items.
	go func() {
		for _, df := range actualDiscovered {
			select {
			case <-ctx.Done():
				close(workCh)
				return
			case workCh <- workItem{df: df, fileID: fileIDs[df.Path]}:
			}
		}
		close(workCh)
	}()

	pendingCount := len(actualDiscovered)
	completed := 0
	// memSeen is an in-memory dedup fallback used when no DB is available (e.g. tests).
	// The coordinator is single-threaded, so no mutex is needed.
	memSeen := make(map[string]string)

	for completed < pendingCount {
		select {
		case <-ctx.Done():
			goto done

		case wr, ok := <-resultCh:
			if !ok {
				goto done
			}
			if wr.err != nil {
				var dfs *dateFilterSkip
				if errors.As(wr.err, &dfs) {
					// Date filter skip — not an error.
					if opts.Config.Verbosity >= 0 {
						_, _ = fmt.Fprint(out, fmtr.FormatOutput("SKIP", wr.df.RelPath, dfs.reason))
					}
					lw.WriteEntry(domain.LedgerEntry{
						Path:   wr.df.RelPath,
						Status: domain.LedgerStatusSkip,
						Reason: dfs.reason,
					})
					if db != nil {
						if dbErr := db.UpdateFileStatus(wr.fileID, "skipped",
							archivedb.WithSkipReason(dfs.reason),
							archivedb.WithCaptureDate(dfs.captureDate)); dbErr != nil {
							_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", wr.df.RelPath, dbErr)))
						}
					}
					result.Skipped++
					emit(opts.EventBus, progress.Event{
						Kind:      progress.EventFileSkipped,
						RelPath:   wr.df.RelPath,
						Reason:    dfs.reason,
						WorkerID:  wr.workerID,
						Completed: result.Skipped + result.Processed + result.Errors,
					})
					completed++
					continue
				}
				if db != nil {
					if dbErr := db.UpdateFileStatus(wr.fileID, "failed",
						archivedb.WithError(wr.err.Error())); dbErr != nil {
						_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", wr.df.RelPath, dbErr)))
					}
				}
				result.Errors++
				if opts.Config.Verbosity >= 0 {
					_, _ = fmt.Fprint(out, fmtr.FormatOutput("ERR ", wr.df.RelPath, wr.err.Error()))
				}
				lw.WriteEntry(domain.LedgerEntry{
					Path:   wr.df.RelPath,
					Status: domain.LedgerStatusError,
					Reason: wr.err.Error(),
				})
				emit(opts.EventBus, progress.Event{
					Kind:      progress.EventFileError,
					RelPath:   wr.df.RelPath,
					Reason:    wr.err.Error(),
					Err:       wr.err,
					WorkerID:  wr.workerID,
					Completed: result.Skipped + result.Processed + result.Errors,
				})
				completed++
				continue
			}

			// Dedup check — single-writer on the DB (or memSeen when DB is nil).
			var isDuplicate bool
			var existingDestForLedger string
			if db != nil {
				existingDest, err := db.CheckDuplicate(wr.checksum)
				if err != nil {
					if dbErr := db.UpdateFileStatus(wr.fileID, "failed", archivedb.WithError(err.Error())); dbErr != nil {
						_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", wr.df.RelPath, dbErr)))
					}
					result.Errors++
					if opts.Config.Verbosity >= 0 {
						_, _ = fmt.Fprint(out, fmtr.FormatOutput("ERR ", wr.df.RelPath, err.Error()))
					}
					lw.WriteEntry(domain.LedgerEntry{
						Path:   wr.df.RelPath,
						Status: domain.LedgerStatusError,
						Reason: err.Error(),
					})
					completed++
					continue
				}
				isDuplicate = existingDest != ""
				existingDestForLedger = existingDest
			} else if dest, seen := memSeen[wr.checksum]; seen {
				isDuplicate = true
				existingDestForLedger = dest
			}

			skipCopy := isDuplicate && opts.Config.SkipDuplicates
			relDest := pathbuilder.Build(opts.PathTemplate, wr.date, opts.Hasher.AlgorithmID(), wr.checksum, wr.ext, isDuplicate, opts.RunTimestamp)
			absDest := filepath.Join(dirB, relDest)

			// When no DB is available, record the assignment in memSeen immediately
			// so that subsequent workResults with the same checksum are routed as
			// duplicates before this worker completes. Without this, two workers
			// processing identical files concurrently would both receive
			// isDuplicate=false and race to write the same temp file path.
			if db == nil && !isDuplicate {
				memSeen[wr.checksum] = relDest
			}

			assignChs[wr.workerID] <- destAssignment{
				absDest:               absDest,
				relDest:               relDest,
				isDuplicate:           isDuplicate,
				existingDestForLedger: existingDestForLedger,
				skipCopy:              skipCopy,
			}

		case fr, ok := <-doneCh:
			if !ok {
				goto done
			}
			// Warn if a terminal DB status update failed inside the worker.
			if fr.dbErr != nil {
				_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", fr.df.RelPath, fr.dbErr)))
			}
			if fr.err != nil {
				result.Errors++
				if opts.Config.Verbosity >= 0 {
					_, _ = fmt.Fprint(out, fmtr.FormatOutput("ERR ", fr.df.RelPath, fr.err.Error()))
				}
				lw.WriteEntry(domain.LedgerEntry{
					Path:   fr.df.RelPath,
					Status: domain.LedgerStatusError,
					Reason: fr.err.Error(),
				})
				emit(opts.EventBus, progress.Event{
					Kind:      progress.EventFileError,
					RelPath:   fr.df.RelPath,
					Reason:    fr.err.Error(),
					Err:       fr.err,
					WorkerID:  -1,
					Completed: result.Skipped + result.Processed + result.Errors,
				})
			} else if fr.skipCopy {
				// --skip-duplicates: no file was written; update DB and emit output.
				if db != nil {
					if dbErr := db.UpdateFileStatus(fr.fileID, "complete",
						archivedb.WithChecksum(fr.checksum),
						archivedb.WithAlgorithm(opts.Hasher.Algorithm()),
						archivedb.WithIsDuplicate(true)); dbErr != nil {
						_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", fr.df.RelPath, dbErr)))
					}
				}
				result.Processed++
				result.Duplicates++
				matchDetail := fr.existingDestForLedger
				if opts.Config.Verbosity >= 0 {
					_, _ = fmt.Fprint(out, fmtr.FormatOutput("DUPE", fr.df.RelPath,
						fmt.Sprintf("matches %s", matchDetail)))
				}
				lw.WriteEntry(domain.LedgerEntry{
					Path:     fr.df.RelPath,
					Status:   domain.LedgerStatusDuplicate,
					Checksum: fr.checksum,
					Matches:  fr.existingDestForLedger,
					// Destination intentionally omitted — no file was written.
				})
				emit(opts.EventBus, progress.Event{
					Kind:        progress.EventFileDuplicate,
					RelPath:     fr.df.RelPath,
					IsDuplicate: true,
					MatchesDest: fr.existingDestForLedger,
					Checksum:    fr.checksum,
					WorkerID:    -1,
					Completed:   result.Skipped + result.Processed + result.Errors,
				})
			} else {
				finalRelDest := fr.relDest
				finalIsDup := fr.isDuplicate
				finalExistingDest := fr.existingDestForLedger
				finalSidecars := fr.carriedSidecarRels

				if db != nil {
					if finalIsDup {
						// Pre-copy dedup — just mark complete.
						if dbErr := db.UpdateFileStatus(fr.fileID, "complete",
							archivedb.WithIsDuplicate(true),
							archivedb.WithCarriedSidecars(finalSidecars)); dbErr != nil {
							_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", fr.df.RelPath, dbErr)))
						}
					} else {
						// Atomic post-copy dedup re-check to handle cross-process races.
						existingDest, dedupErr := db.CompleteFileWithDedupCheck(fr.fileID, fr.checksum)
						if dedupErr != nil {
							result.Errors++
							errMsg := fmt.Sprintf("dedup check: %v", dedupErr)
							if opts.Config.Verbosity >= 0 {
								_, _ = fmt.Fprint(out, fmtr.FormatOutput("ERR ", fr.df.RelPath, errMsg))
							}
							lw.WriteEntry(domain.LedgerEntry{
								Path:   fr.df.RelPath,
								Status: domain.LedgerStatusError,
								Reason: errMsg,
							})
							completed++
							continue
						}
						if existingDest != "" {
							// Race detected — relocate physical file to duplicates/.
							dupRelDest := pathbuilder.Build(opts.PathTemplate, fr.verifiedAt, opts.Hasher.AlgorithmID(), fr.checksum,
								filepath.Ext(fr.df.Path), true, opts.RunTimestamp)
							dupAbsDest := filepath.Join(dirB, dupRelDest)
							absDest := filepath.Join(dirB, fr.relDest)
							if renErr := os.Rename(absDest, dupAbsDest); renErr != nil {
								if dbErr := db.UpdateFileStatus(fr.fileID, "failed",
									archivedb.WithError(renErr.Error())); dbErr != nil {
									_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", fr.df.RelPath, dbErr)))
								}
								result.Errors++
								errMsg := fmt.Sprintf("relocate duplicate: %v", renErr)
								if opts.Config.Verbosity >= 0 {
									_, _ = fmt.Fprint(out, fmtr.FormatOutput("ERR ", fr.df.RelPath, errMsg))
								}
								lw.WriteEntry(domain.LedgerEntry{
									Path:   fr.df.RelPath,
									Status: domain.LedgerStatusError,
									Reason: errMsg,
								})
								completed++
								continue
							}
							if dbErr := db.UpdateFileStatus(fr.fileID, "complete",
								archivedb.WithDestination(dupAbsDest, dupRelDest),
								archivedb.WithIsDuplicate(true),
								archivedb.WithCarriedSidecars(finalSidecars)); dbErr != nil {
								_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", fr.df.RelPath, dbErr)))
							}
							finalRelDest = dupRelDest
							finalIsDup = true
							finalExistingDest = existingDest
						} else {
							// Non-race path: update carried_sidecars if any.
							if len(finalSidecars) > 0 {
								if dbErr := db.UpdateFileStatus(fr.fileID, "complete",
									archivedb.WithCarriedSidecars(finalSidecars)); dbErr != nil {
									_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("DB status update failed for %q: %v", fr.df.RelPath, dbErr)))
								}
							}
						}
						// If existingDest == "", CompleteFileWithDedupCheck already set status='complete'.
					}
				} else if !finalIsDup {
					// No DB — ensure memSeen is up to date. The coordinator already
					// wrote this entry at assignment time, but update here as a
					// safety net in case the relDest was adjusted (e.g. race rename).
					memSeen[fr.checksum] = finalRelDest
				}

				result.Processed++
				if finalIsDup {
					result.Duplicates++
					if opts.Config.Verbosity >= 0 {
						matchDetail := finalExistingDest
						if matchDetail == "" {
							matchDetail = finalRelDest
						}
						_, _ = fmt.Fprint(out, fmtr.FormatOutput("DUPE", fr.df.RelPath,
							fmt.Sprintf("matches %s", displayDest(destLabel, matchDetail))))
						emitSidecarLines(out, fr.df.Sidecars, finalRelDest, resolveTags(opts.Config, fr.captureDate), opts.Config, destLabel)
					}
					lw.WriteEntry(domain.LedgerEntry{
						Path:        fr.df.RelPath,
						Status:      domain.LedgerStatusDuplicate,
						Checksum:    fr.checksum,
						Destination: finalRelDest,
						Sidecars:    finalSidecars,
						Matches:     finalExistingDest,
					})
					emit(opts.EventBus, progress.Event{
						Kind:        progress.EventFileDuplicate,
						RelPath:     fr.df.RelPath,
						IsDuplicate: true,
						MatchesDest: finalExistingDest,
						Destination: finalRelDest,
						Checksum:    fr.checksum,
						WorkerID:    -1,
						Completed:   result.Skipped + result.Processed + result.Errors,
					})
				} else {
					if opts.Config.Verbosity >= 0 {
						_, _ = fmt.Fprint(out, fmtr.FormatOutput("COPY", fr.df.RelPath, displayDest(destLabel, finalRelDest)))
						emitSidecarLines(out, fr.df.Sidecars, finalRelDest, resolveTags(opts.Config, fr.captureDate), opts.Config, destLabel)
					}
					lw.WriteEntry(domain.LedgerEntry{
						Path:        fr.df.RelPath,
						Status:      domain.LedgerStatusCopy,
						Checksum:    fr.checksum,
						Destination: finalRelDest,
						VerifiedAt:  &fr.verifiedAt,
						Sidecars:    finalSidecars,
					})
					emit(opts.EventBus, progress.Event{
						Kind:        progress.EventFileComplete,
						RelPath:     fr.df.RelPath,
						Destination: finalRelDest,
						Checksum:    fr.checksum,
						WorkerID:    -1,
						Completed:   result.Skipped + result.Processed + result.Errors,
					})
				}
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
	fmtr *Formatter,
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

			var fileSize int64
			if fi, statErr := os.Stat(item.df.Path); statErr == nil {
				fileSize = fi.Size()
			}
			emit(opts.EventBus, progress.Event{
				Kind:     progress.EventFileStart,
				RelPath:  item.df.RelPath,
				WorkerID: id,
				FileSize: fileSize,
			})

			// --- Extract date ---
			captureDate, err := item.df.Handler.ExtractDate(item.df.Path)
			if err != nil {
				err = fmt.Errorf("extract date: %w", err)
				// Send error via resultCh; coordinator will count it and not expect a doneCh message.
				resultCh <- workResult{df: item.df, fileID: item.fileID, workerID: id, err: err}
				continue
			}
			// Best-effort intermediate status update. Workers write intermediate
			// states (extracted, hashed, copied, verified) directly to the DB;
			// the coordinator owns terminal states (complete, failed, skipped).
			// Errors are intentionally discarded — intermediate tracking is
			// observational and not required for pipeline correctness. The SQLite
			// busy timeout (5 s) handles contention from concurrent workers.
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "extracted",
					archivedb.WithCaptureDate(captureDate))
			}
			emit(opts.EventBus, progress.Event{
				Kind:        progress.EventFileExtracted,
				RelPath:     item.df.RelPath,
				CaptureDate: captureDate,
				WorkerID:    id,
			})

			// --- Date filter gate ---
			cfg := opts.Config
			if cfg.Since != nil && captureDate.Before(*cfg.Since) {
				resultCh <- workResult{
					df: item.df, fileID: item.fileID, workerID: id,
					err: &dateFilterSkip{
						reason:      "outside date range: before " + cfg.Since.Format("2006-01-02"),
						captureDate: captureDate,
					},
				}
				continue
			}
			if cfg.Before != nil && captureDate.After(*cfg.Before) {
				resultCh <- workResult{
					df: item.df, fileID: item.fileID, workerID: id,
					err: &dateFilterSkip{
						reason:      "outside date range: after " + cfg.Before.Truncate(24*time.Hour).Format("2006-01-02"),
						captureDate: captureDate,
					},
				}
				continue
			}

			// --- Hash ---
			rc, err := item.df.Handler.HashableReader(item.df.Path)
			if err != nil {
				err = fmt.Errorf("hash reader: %w", err)
				resultCh <- workResult{df: item.df, fileID: item.fileID, workerID: id, err: err}
				continue
			}
			// Wrap with ProgressReader for byte-level progress (no-op when bus is nil).
			hashReader := progress.NewProgressReader(rc, opts.EventBus, item.df.RelPath, id, "HASH", fileSize)
			checksum, err := opts.Hasher.Sum(hashReader)
			_ = rc.Close()
			if err != nil {
				err = fmt.Errorf("hash: %w", err)
				resultCh <- workResult{df: item.df, fileID: item.fileID, workerID: id, err: err}
				continue
			}
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "hashed",
					archivedb.WithChecksum(checksum),
					archivedb.WithAlgorithm(opts.Hasher.Algorithm()))
			}
			emit(opts.EventBus, progress.Event{
				Kind:     progress.EventFileHashed,
				RelPath:  item.df.RelPath,
				Checksum: checksum,
				WorkerID: id,
			})

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

			// --- Skip-duplicates: no I/O, coordinator handles DB + ledger ---
			if assign.skipCopy {
				doneCh <- workerFinalResult{
					df:                    item.df,
					fileID:                item.fileID,
					checksum:              checksum,
					relDest:               assign.relDest,
					isDuplicate:           true,
					existingDestForLedger: assign.existingDestForLedger,
					captureDate:           captureDate,
					skipCopy:              true,
				}
				continue
			}

			if opts.Config.DryRun {
				if opts.Config.Verbosity >= 0 {
					_, _ = fmt.Fprintf(out, "  DRY-RUN  %s -> %s\n", item.df.RelPath, displayDest(opts.DestLabel, assign.relDest))
					dryTags := resolveTags(opts.Config, captureDate)
					emitSidecarLines(out, item.df.Sidecars, assign.relDest, dryTags, opts.Config, opts.DestLabel)
				}
				var dryDBErr error
				if db != nil {
					dryDBErr = db.UpdateFileStatus(item.fileID, "complete",
						archivedb.WithDestination(assign.absDest, assign.relDest),
						archivedb.WithIsDuplicate(assign.isDuplicate))
				}
				doneCh <- workerFinalResult{
					df: item.df, fileID: item.fileID,
					checksum: checksum, relDest: assign.relDest,
					isDuplicate:           assign.isDuplicate,
					existingDestForLedger: assign.existingDestForLedger,
					captureDate:           captureDate,
					verifiedAt:            time.Now().UTC(),
					dbErr:                 dryDBErr,
				}
				continue
			}

			// --- Copy (atomic: write to temp file) ---
			// Wrap the destination writer with ProgressWriter for byte-level progress.
			var copyWriter *progress.ProgressWriter
			tmpPath, err := copypkg.ExecuteWithProgress(item.df.Path, assign.absDest, func(w io.Writer) io.Writer {
				copyWriter = progress.NewProgressWriter(w, opts.EventBus, item.df.RelPath, id, "COPY", fileSize)
				return copyWriter
			})
			if err != nil {
				ferr := fmt.Errorf("copy: %w", err)
				var copyDBErr error
				if db != nil {
					copyDBErr = db.UpdateFileStatus(item.fileID, "failed", archivedb.WithError(ferr.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: ferr, dbErr: copyDBErr}
				continue
			}
			// Emit a final 100% byte-progress event so the UI sees completion
			// before the stage-transition EventFileCopied arrives.
			if copyWriter != nil {
				copyWriter.EmitFinal()
			}
			if db != nil {
				// Record the intended final destination; the temp path is an
				// implementation detail not tracked in the DB.
				_ = db.UpdateFileStatus(item.fileID, "copied",
					archivedb.WithDestination(assign.absDest, assign.relDest))
			}
			emit(opts.EventBus, progress.Event{
				Kind:        progress.EventFileCopied,
				RelPath:     item.df.RelPath,
				Destination: assign.relDest,
				WorkerID:    id,
			})

			// --- Verify (hash the temp file) ---
			// Wrap the hashable reader with ProgressReader for byte-level progress.
			vr := copypkg.VerifyWithProgress(tmpPath, checksum, item.df.Handler, opts.Hasher, func(r io.Reader) io.Reader {
				return progress.NewProgressReader(r, opts.EventBus, item.df.RelPath, id, "VERIFY", fileSize)
			})
			if !vr.Success {
				// If the temp file no longer exists, another worker won the race
				// to the same destination (same checksum, no-DB mode). The file
				// is already at the canonical path — treat this as a duplicate.
				if errors.Is(vr.Error, os.ErrNotExist) {
					doneCh <- workerFinalResult{
						df:          item.df,
						fileID:      item.fileID,
						checksum:    checksum,
						relDest:     assign.relDest,
						isDuplicate: true,
					}
					continue
				}
				copypkg.CleanupTempFile(tmpPath)
				var mmDBErr error
				if db != nil {
					mmDBErr = db.UpdateFileStatus(item.fileID, "mismatch", archivedb.WithError(vr.Error.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: vr.Error, dbErr: mmDBErr}
				continue
			}

			// --- Promote (atomic rename temp → canonical path) ---
			if err := copypkg.Promote(tmpPath, assign.absDest); err != nil {
				copypkg.CleanupTempFile(tmpPath)
				// If the rename failed because the temp file no longer exists,
				// another worker won the race to the same destination (same
				// checksum, no-DB mode). Treat this as a duplicate rather than
				// an error — the file is already at the canonical path.
				if errors.Is(err, os.ErrNotExist) {
					doneCh <- workerFinalResult{
						df:          item.df,
						fileID:      item.fileID,
						checksum:    checksum,
						relDest:     assign.relDest,
						isDuplicate: true,
					}
					continue
				}
				ferr := fmt.Errorf("promote: %w", err)
				var promDBErr error
				if db != nil {
					promDBErr = db.UpdateFileStatus(item.fileID, "failed", archivedb.WithError(ferr.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: ferr, dbErr: promDBErr}
				continue
			}
			verifiedAt := time.Now().UTC()
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "verified")
			}
			emit(opts.EventBus, progress.Event{
				Kind:        progress.EventFileVerified,
				RelPath:     item.df.RelPath,
				Destination: assign.relDest,
				WorkerID:    id,
			})

			// --- Carry sidecars (after verify, before tag) ---
			var carriedSidecarRels []string
			var carriedXMPAbs string
			if opts.Config.CarrySidecars && len(item.df.Sidecars) > 0 {
				for _, sc := range item.df.Sidecars {
					sidecarDest := assign.absDest + sc.Ext
					sidecarRel := assign.relDest + sc.Ext
					if err := copypkg.CopySidecar(sc.Path, sidecarDest); err != nil {
						if opts.Config.Verbosity >= 0 {
							_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("sidecar carry failed for %s: %v", sc.RelPath, err)))
						}
						emit(opts.EventBus, progress.Event{
							Kind:           progress.EventSidecarFailed,
							RelPath:        item.df.RelPath,
							SidecarRelPath: sc.RelPath,
							SidecarExt:     sc.Ext,
							Reason:         err.Error(),
							Err:            err,
							WorkerID:       id,
						})
						continue
					}
					carriedSidecarRels = append(carriedSidecarRels, sidecarRel)
					emit(opts.EventBus, progress.Event{
						Kind:           progress.EventSidecarCarried,
						RelPath:        item.df.RelPath,
						SidecarRelPath: sc.RelPath,
						SidecarExt:     sc.Ext,
						Destination:    sidecarRel,
						WorkerID:       id,
					})
					if sc.Ext == ".xmp" {
						carriedXMPAbs = sidecarDest
					}
				}
			}

			// --- Tag ---
			var finalDBErr error
			tags := resolveTags(opts.Config, captureDate)
			if !tags.IsEmpty() {
				if err := tagging.ApplyWithSidecars(assign.absDest, item.df.Handler, tags, carriedXMPAbs, opts.Config.OverwriteSidecarTags); err != nil {
					if db != nil {
						finalDBErr = db.UpdateFileStatus(item.fileID, "tag_failed", archivedb.WithError(err.Error()))
					}
					if opts.Config.Verbosity >= 0 {
						_, _ = fmt.Fprint(out, fmtr.FormatWarning(fmt.Sprintf("tag failed for %s: %v", item.df.RelPath, err)))
					}
					// tag_failed is non-fatal — the file is copied and verified.
				} else {
					if db != nil {
						_ = db.UpdateFileStatus(item.fileID, "tagged")
					}
					emit(opts.EventBus, progress.Event{
						Kind:     progress.EventFileTagged,
						RelPath:  item.df.RelPath,
						WorkerID: id,
					})
				}
			}

			doneCh <- workerFinalResult{
				df:                    item.df,
				fileID:                item.fileID,
				checksum:              checksum,
				relDest:               assign.relDest,
				isDuplicate:           assign.isDuplicate,
				existingDestForLedger: assign.existingDestForLedger,
				captureDate:           captureDate,
				verifiedAt:            verifiedAt,
				carriedSidecarRels:    carriedSidecarRels,
				dbErr:                 finalDBErr,
			}
		}
	}
}
