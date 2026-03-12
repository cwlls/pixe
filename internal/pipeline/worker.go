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

	"github.com/cwlls/pixe-go/internal/archivedb"
	copypkg "github.com/cwlls/pixe-go/internal/copy"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/manifest"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/tagging"
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
	skipCopy              bool     // true when the worker skipped I/O due to --skip-duplicates
	carriedSidecarRels    []string // dest_rel paths of successfully carried sidecars
	err                   error
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
	fileIDs map[string]int64, dirA, dirB string, out io.Writer, lw *manifest.LedgerWriter) SortResult {

	ctx := context.Background()
	return runConcurrentCtx(ctx, opts, discovered, skipped, fileIDs, dirA, dirB, out, lw)
}

// runConcurrentCtx is the context-aware implementation, used by tests.
func runConcurrentCtx(ctx context.Context, opts SortOptions, discovered []discovery.DiscoveredFile,
	skipped []discovery.SkippedFile,
	fileIDs map[string]int64, dirA, dirB string, out io.Writer, lw *manifest.LedgerWriter) SortResult {

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
		_, _ = fmt.Fprint(out, formatOutput("SKIP", sf.Path, sf.Reason))
		_ = lw.WriteEntry(domain.LedgerEntry{
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
				_ = db.UpdateFileStatus(skipFileID, "skipped",
					archivedb.WithSkipReason(sf.Reason))
			}
		}
		result.Skipped++
	}

	// --- Filter out previously-imported files before feeding workers ---
	// actualDiscovered holds files that need real processing.
	actualDiscovered := make([]discovery.DiscoveredFile, 0, len(discovered))
	for _, df := range discovered {
		if db != nil {
			processed, checkErr := db.CheckSourceProcessed(df.Path)
			if checkErr != nil {
				_, _ = fmt.Fprint(out, formatOutput("ERR ", df.RelPath, checkErr.Error()))
				result.Errors++
				continue
			}
			if processed {
				const reason = "previously imported"
				_, _ = fmt.Fprint(out, formatOutput("SKIP", df.RelPath, reason))
				_ = lw.WriteEntry(domain.LedgerEntry{
					Path:   df.RelPath,
					Status: domain.LedgerStatusSkip,
					Reason: reason,
				})
				fileID := fileIDs[df.Path]
				_ = db.UpdateFileStatus(fileID, "skipped",
					archivedb.WithSkipReason(reason))
				result.Skipped++
				continue
			}
		}
		actualDiscovered = append(actualDiscovered, df)
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
				if db != nil {
					_ = db.UpdateFileStatus(wr.fileID, "failed",
						archivedb.WithError(wr.err.Error()))
				}
				result.Errors++
				_, _ = fmt.Fprint(out, formatOutput("ERR ", wr.df.RelPath, wr.err.Error()))
				_ = lw.WriteEntry(domain.LedgerEntry{
					Path:   wr.df.RelPath,
					Status: domain.LedgerStatusError,
					Reason: wr.err.Error(),
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
					_ = db.UpdateFileStatus(wr.fileID, "failed", archivedb.WithError(err.Error()))
					result.Errors++
					_, _ = fmt.Fprint(out, formatOutput("ERR ", wr.df.RelPath, err.Error()))
					_ = lw.WriteEntry(domain.LedgerEntry{
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
			relDest := pathbuilder.Build(wr.date, wr.checksum, wr.ext, isDuplicate, opts.RunTimestamp)
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
			if fr.err != nil {
				result.Errors++
				_, _ = fmt.Fprint(out, formatOutput("ERR ", fr.df.RelPath, fr.err.Error()))
				_ = lw.WriteEntry(domain.LedgerEntry{
					Path:   fr.df.RelPath,
					Status: domain.LedgerStatusError,
					Reason: fr.err.Error(),
				})
			} else if fr.skipCopy {
				// --skip-duplicates: no file was written; update DB and emit output.
				if db != nil {
					_ = db.UpdateFileStatus(fr.fileID, "complete",
						archivedb.WithChecksum(fr.checksum),
						archivedb.WithIsDuplicate(true))
				}
				result.Processed++
				result.Duplicates++
				matchDetail := fr.existingDestForLedger
				_, _ = fmt.Fprint(out, formatOutput("DUPE", fr.df.RelPath,
					fmt.Sprintf("matches %s", matchDetail)))
				_ = lw.WriteEntry(domain.LedgerEntry{
					Path:     fr.df.RelPath,
					Status:   domain.LedgerStatusDuplicate,
					Checksum: fr.checksum,
					Matches:  fr.existingDestForLedger,
					// Destination intentionally omitted — no file was written.
				})
			} else {
				finalRelDest := fr.relDest
				finalIsDup := fr.isDuplicate
				finalExistingDest := fr.existingDestForLedger
				finalSidecars := fr.carriedSidecarRels

				if db != nil {
					if finalIsDup {
						// Pre-copy dedup — just mark complete.
						_ = db.UpdateFileStatus(fr.fileID, "complete",
							archivedb.WithIsDuplicate(true),
							archivedb.WithCarriedSidecars(finalSidecars))
					} else {
						// Atomic post-copy dedup re-check to handle cross-process races.
						existingDest, dedupErr := db.CompleteFileWithDedupCheck(fr.fileID, fr.checksum)
						if dedupErr != nil {
							result.Errors++
							errMsg := fmt.Sprintf("dedup check: %v", dedupErr)
							_, _ = fmt.Fprint(out, formatOutput("ERR ", fr.df.RelPath, errMsg))
							_ = lw.WriteEntry(domain.LedgerEntry{
								Path:   fr.df.RelPath,
								Status: domain.LedgerStatusError,
								Reason: errMsg,
							})
							completed++
							continue
						}
						if existingDest != "" {
							// Race detected — relocate physical file to duplicates/.
							dupRelDest := pathbuilder.Build(fr.verifiedAt, fr.checksum,
								filepath.Ext(fr.df.Path), true, opts.RunTimestamp)
							dupAbsDest := filepath.Join(dirB, dupRelDest)
							absDest := filepath.Join(dirB, fr.relDest)
							if renErr := os.Rename(absDest, dupAbsDest); renErr != nil {
								_ = db.UpdateFileStatus(fr.fileID, "failed",
									archivedb.WithError(renErr.Error()))
								result.Errors++
								errMsg := fmt.Sprintf("relocate duplicate: %v", renErr)
								_, _ = fmt.Fprint(out, formatOutput("ERR ", fr.df.RelPath, errMsg))
								_ = lw.WriteEntry(domain.LedgerEntry{
									Path:   fr.df.RelPath,
									Status: domain.LedgerStatusError,
									Reason: errMsg,
								})
								completed++
								continue
							}
							_ = db.UpdateFileStatus(fr.fileID, "complete",
								archivedb.WithDestination(dupAbsDest, dupRelDest),
								archivedb.WithIsDuplicate(true),
								archivedb.WithCarriedSidecars(finalSidecars))
							finalRelDest = dupRelDest
							finalIsDup = true
							finalExistingDest = existingDest
						} else {
							// Non-race path: update carried_sidecars if any.
							if len(finalSidecars) > 0 {
								_ = db.UpdateFileStatus(fr.fileID, "complete",
									archivedb.WithCarriedSidecars(finalSidecars))
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
					matchDetail := finalExistingDest
					if matchDetail == "" {
						matchDetail = finalRelDest
					}
					_, _ = fmt.Fprint(out, formatOutput("DUPE", fr.df.RelPath,
						fmt.Sprintf("matches %s", matchDetail)))
					emitSidecarLines(out, fr.df.Sidecars, finalRelDest, resolveTags(opts.Config, time.Time{}), opts.Config)
					_ = lw.WriteEntry(domain.LedgerEntry{
						Path:        fr.df.RelPath,
						Status:      domain.LedgerStatusDuplicate,
						Checksum:    fr.checksum,
						Destination: finalRelDest,
						Sidecars:    finalSidecars,
						Matches:     finalExistingDest,
					})
				} else {
					_, _ = fmt.Fprint(out, formatOutput("COPY", fr.df.RelPath, finalRelDest))
					emitSidecarLines(out, fr.df.Sidecars, finalRelDest, resolveTags(opts.Config, time.Time{}), opts.Config)
					_ = lw.WriteEntry(domain.LedgerEntry{
						Path:        fr.df.RelPath,
						Status:      domain.LedgerStatusCopy,
						Checksum:    fr.checksum,
						Destination: finalRelDest,
						VerifiedAt:  &fr.verifiedAt,
						Sidecars:    finalSidecars,
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

			// --- Skip-duplicates: no I/O, coordinator handles DB + ledger ---
			if assign.skipCopy {
				doneCh <- workerFinalResult{
					df:                    item.df,
					fileID:                item.fileID,
					checksum:              checksum,
					relDest:               assign.relDest,
					isDuplicate:           true,
					existingDestForLedger: assign.existingDestForLedger,
					skipCopy:              true,
				}
				continue
			}

			if opts.Config.DryRun {
				_, _ = fmt.Fprintf(out, "  DRY-RUN  %s -> %s\n", item.df.RelPath, assign.relDest)
				dryTags := resolveTags(opts.Config, captureDate)
				emitSidecarLines(out, item.df.Sidecars, assign.relDest, dryTags, opts.Config)
				if db != nil {
					_ = db.UpdateFileStatus(item.fileID, "complete",
						archivedb.WithDestination(assign.absDest, assign.relDest),
						archivedb.WithIsDuplicate(assign.isDuplicate))
				}
				doneCh <- workerFinalResult{
					df: item.df, fileID: item.fileID,
					checksum: checksum, relDest: assign.relDest,
					isDuplicate:           assign.isDuplicate,
					existingDestForLedger: assign.existingDestForLedger,
					verifiedAt:            time.Now().UTC(),
				}
				continue
			}

			// --- Copy (atomic: write to temp file) ---
			tmpPath, err := copypkg.Execute(item.df.Path, assign.absDest)
			if err != nil {
				ferr := fmt.Errorf("copy: %w", err)
				if db != nil {
					_ = db.UpdateFileStatus(item.fileID, "failed", archivedb.WithError(ferr.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: ferr}
				continue
			}
			if db != nil {
				// Record the intended final destination; the temp path is an
				// implementation detail not tracked in the DB.
				_ = db.UpdateFileStatus(item.fileID, "copied",
					archivedb.WithDestination(assign.absDest, assign.relDest))
			}

			// --- Verify (hash the temp file) ---
			vr := copypkg.Verify(tmpPath, checksum, item.df.Handler, opts.Hasher)
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
				if db != nil {
					_ = db.UpdateFileStatus(item.fileID, "mismatch", archivedb.WithError(vr.Error.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: vr.Error}
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
				if db != nil {
					_ = db.UpdateFileStatus(item.fileID, "failed", archivedb.WithError(ferr.Error()))
				}
				doneCh <- workerFinalResult{df: item.df, fileID: item.fileID, err: ferr}
				continue
			}
			verifiedAt := time.Now().UTC()
			if db != nil {
				_ = db.UpdateFileStatus(item.fileID, "verified")
			}

			// --- Carry sidecars (after verify, before tag) ---
			var carriedSidecarRels []string
			var carriedXMPAbs string
			if opts.Config.CarrySidecars && len(item.df.Sidecars) > 0 {
				for _, sc := range item.df.Sidecars {
					sidecarDest := assign.absDest + sc.Ext
					sidecarRel := assign.relDest + sc.Ext
					if err := copypkg.CopySidecar(sc.Path, sidecarDest); err != nil {
						_, _ = fmt.Fprintf(out, "  WARNING  sidecar carry failed for %s: %v\n",
							sc.RelPath, err)
						continue
					}
					carriedSidecarRels = append(carriedSidecarRels, sidecarRel)
					if sc.Ext == ".xmp" {
						carriedXMPAbs = sidecarDest
					}
				}
			}

			// --- Tag ---
			tags := resolveTags(opts.Config, captureDate)
			if !tags.IsEmpty() {
				if err := tagging.ApplyWithSidecars(assign.absDest, item.df.Handler, tags, carriedXMPAbs, opts.Config.OverwriteSidecarTags); err != nil {
					if db != nil {
						_ = db.UpdateFileStatus(item.fileID, "tag_failed", archivedb.WithError(err.Error()))
					}
					_, _ = fmt.Fprintf(out, "  WARNING  tag failed for %s: %v\n",
						item.df.RelPath, err)
				} else {
					if db != nil {
						_ = db.UpdateFileStatus(item.fileID, "tagged")
					}
				}
			}

			doneCh <- workerFinalResult{
				df:                    item.df,
				fileID:                item.fileID,
				checksum:              checksum,
				relDest:               assign.relDest,
				isDuplicate:           assign.isDuplicate,
				existingDestForLedger: assign.existingDestForLedger,
				verifiedAt:            verifiedAt,
				carriedSidecarRels:    carriedSidecarRels,
			}
		}
	}
}
