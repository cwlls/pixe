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

// Package cli provides the mpb-based progress bar display used by the
// `pixe sort --progress` and `pixe verify --progress` commands.
//
// RunProgress creates an mpb progress container and starts an event consumer
// goroutine that reads from the pipeline event bus and updates bars directly.
// The caller launches the pipeline in a separate goroutine, then calls
// p.Wait() to block until all bars complete (triggered by bus.Close()).
//
// This package has no Charm/Bubble Tea dependencies. All styling is done with
// raw ANSI 256-color escape codes.
package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	pixeprogress "github.com/cwlls/pixe/internal/progress"
)

// ANSI 256-color escape codes for the progress display.
// Mid-range palette values chosen for readability on both light and dark terminals.
const (
	ansiReset    = "\033[0m"
	ansiBoldBlue = "\033[1;38;5;75m"  // stage labels: HASH, COPY, VERIFY, TAG
	ansiDimGray  = "\033[38;5;242m"   // header, file size, elapsed
	ansiBoldRed  = "\033[1;38;5;204m" // error count
	ansiWhite    = "\033[38;5;252m"   // filenames, counters
	ansiItalic   = "\033[3;38;5;242m" // discovery text
)

func ansiWrap(text, code string) string {
	return code + text + ansiReset
}

// WorkerState tracks the current processing state of a single worker.
// The Bar field is the mpb bar for this worker; it is created on EventFileStart
// and removed on terminal events.
type WorkerState struct {
	WorkerID     int
	RelPath      string // basename of the file being processed
	Stage        string // current pipeline stage label
	FileSize     int64  // total file size in bytes
	BytesWritten int64  // bytes processed so far (for delta tracking)
	Bar          *mpb.Bar
	mu           sync.Mutex // protects Stage and RelPath for decor.Any reads
}

func (ws *WorkerState) setStage(s string) {
	ws.mu.Lock()
	ws.Stage = s
	ws.mu.Unlock()
}

func (ws *WorkerState) getStage() string {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.Stage
}

func (ws *WorkerState) getRelPath() string {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.RelPath
}

// stageLabel returns an ANSI-colored, fixed-width stage label string.
func stageLabel(stage string) string {
	const width = 8
	padded := fmt.Sprintf("%-*s", width, stage)
	return ansiWrap(padded, ansiBoldBlue)
}

// truncName truncates a filename to maxWidth characters, appending "..." if needed.
func truncName(name string, maxWidth int) string {
	if len(name) <= maxWidth {
		return fmt.Sprintf("%-*s", maxWidth, name)
	}
	return name[:maxWidth-3] + "..."
}

// humanSize returns a human-readable file size string.
func humanSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// counters holds the aggregate run counters updated by the event consumer.
// All fields are written only by the consumer goroutine; decor.Any closures
// read them on the mpb render goroutine. The mutex protects concurrent access.
type counters struct {
	mu           sync.Mutex
	copied       int
	dupes        int
	skipped      int
	errors       int
	verified     int
	mismatches   int
	unrecognised int
	done         bool
	duration     time.Duration
}

// RunProgress creates an mpb progress container, starts an event consumer
// goroutine that reads from bus and updates bars, and returns the container.
//
// The caller is responsible for:
//  1. Launching the pipeline in a separate goroutine.
//  2. Calling bus.Close() when the pipeline finishes.
//  3. Calling p.Wait() to block until all bars complete.
//
// mode must be "sort" or "verify".
func RunProgress(ctx context.Context, bus *pixeprogress.Bus, source, dest, mode string) *mpb.Progress {
	p := mpb.NewWithContext(ctx,
		mpb.WithRefreshRate(150*time.Millisecond),
		mpb.PopCompletedMode(),
	)

	cnt := &counters{}

	// Header bar — static NopStyle line at the top.
	header := fmt.Sprintf("pixe %s  %s", mode, source)
	if dest != "" {
		header += " → " + dest
	}
	headerBar := p.New(0, mpb.NopStyle(),
		mpb.BarPriority(0),
		mpb.BarFillerTrim(),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				return ansiWrap(header, ansiDimGray)
			}),
		),
	)

	// Discovery spinner — shown until EventDiscoverDone / EventVerifyStart.
	spinnerBar := p.AddSpinner(0,
		mpb.BarPriority(1),
		mpb.BarFillerTrim(),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				return ansiWrap("  Discovering files...", ansiItalic)
			}),
		),
	)

	// Overall progress bar — total set when discovery completes.
	totalBar := p.AddBar(0,
		mpb.BarPriority(2),
		mpb.PrependDecorators(
			decor.CountersNoUnit(" %d / %d ", decor.WCSyncSpace),
		),
		mpb.AppendDecorators(
			decor.NewPercentage(" %d%% "),
			decor.AverageETA(decor.ET_STYLE_GO,
				decor.WC{W: 10},
			),
		),
	)

	// Status counter bar — NopStyle line at the bottom.
	statusBar := p.New(0, mpb.NopStyle(),
		mpb.BarPriority(1000),
		mpb.BarFillerTrim(),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				return buildStatusLine(cnt, mode)
			}),
		),
	)

	// Event consumer goroutine.
	go func() {
		workers := make(map[int]*WorkerState)

		for e := range bus.Events() {
			switch e.Kind {

			case pixeprogress.EventDiscoverDone, pixeprogress.EventVerifyStart:
				spinnerBar.Abort(true)
				totalBar.SetTotal(int64(e.Total), false)

			case pixeprogress.EventFileStart, pixeprogress.EventVerifyFileStart:
				// For sort: total = fileSize * 3 (hash + copy + verify stages).
				// For verify: total = fileSize * 1 (hash only).
				fileTotal := e.FileSize
				if mode == "sort" {
					fileTotal = e.FileSize * 3
				}
				if fileTotal <= 0 {
					fileTotal = 1 // avoid zero-total bars
				}

				ws := &WorkerState{
					WorkerID: e.WorkerID,
					RelPath:  filepath.Base(e.RelPath),
					Stage:    "HASH",
					FileSize: e.FileSize,
				}

				bar := p.AddBar(fileTotal,
					mpb.BarRemoveOnComplete(),
					mpb.BarPriority(100+e.WorkerID),
					mpb.PrependDecorators(
						decor.Any(func(s decor.Statistics) string {
							return "  " + stageLabel(ws.getStage())
						}, decor.WC{W: 12}),
						decor.Any(func(s decor.Statistics) string {
							return "  " + ansiWrap(truncName(ws.getRelPath(), 24), ansiWhite)
						}, decor.WC{W: 28}),
					),
					mpb.AppendDecorators(
						decor.NewPercentage(" %d%%", decor.WC{W: 5}),
						decor.Any(func(s decor.Statistics) string {
							return "  " + ansiWrap(fmt.Sprintf("%8s", humanSize(ws.FileSize)), ansiDimGray)
						}, decor.WC{W: 12}),
						decor.EwmaETA(decor.ET_STYLE_GO, 30, decor.WC{W: 8}),
					),
				)
				ws.Bar = bar
				workers[e.WorkerID] = ws

			case pixeprogress.EventByteProgress:
				if ws, ok := workers[e.WorkerID]; ok {
					delta := e.BytesWritten - ws.BytesWritten
					if delta > 0 {
						ws.Bar.EwmaIncrInt64(delta, time.Since(e.Timestamp))
						ws.BytesWritten = e.BytesWritten
					}
					if e.Stage != "" {
						ws.setStage(e.Stage)
					}
				}

			case pixeprogress.EventFileHashed:
				if ws, ok := workers[e.WorkerID]; ok {
					ws.setStage("COPY")
					ws.BytesWritten = 0
				}

			case pixeprogress.EventFileCopied:
				if ws, ok := workers[e.WorkerID]; ok {
					ws.setStage("VERIFY")
					ws.BytesWritten = 0
				}

			case pixeprogress.EventFileVerified:
				if ws, ok := workers[e.WorkerID]; ok {
					ws.setStage("TAG")
					ws.BytesWritten = 0
				}

			case pixeprogress.EventFileComplete:
				cnt.mu.Lock()
				cnt.copied++
				cnt.mu.Unlock()
				finishWorker(workers, e.WorkerID)
				totalBar.Increment()

			case pixeprogress.EventFileDuplicate:
				cnt.mu.Lock()
				cnt.dupes++
				cnt.mu.Unlock()
				finishWorker(workers, e.WorkerID)
				totalBar.Increment()

			case pixeprogress.EventFileSkipped:
				cnt.mu.Lock()
				cnt.skipped++
				cnt.mu.Unlock()
				finishWorker(workers, e.WorkerID)
				totalBar.Increment()

			case pixeprogress.EventFileError:
				cnt.mu.Lock()
				cnt.errors++
				cnt.mu.Unlock()
				finishWorker(workers, e.WorkerID)
				totalBar.Increment()

			case pixeprogress.EventVerifyOK:
				cnt.mu.Lock()
				cnt.verified++
				cnt.mu.Unlock()
				finishWorker(workers, e.WorkerID)
				totalBar.Increment()

			case pixeprogress.EventVerifyMismatch:
				cnt.mu.Lock()
				cnt.mismatches++
				cnt.mu.Unlock()
				finishWorker(workers, e.WorkerID)
				totalBar.Increment()

			case pixeprogress.EventVerifyUnrecognised:
				cnt.mu.Lock()
				cnt.unrecognised++
				cnt.mu.Unlock()
				finishWorker(workers, e.WorkerID)
				totalBar.Increment()

			case pixeprogress.EventRunComplete, pixeprogress.EventVerifyDone:
				if e.Summary != nil {
					cnt.mu.Lock()
					cnt.duration = e.Summary.Duration
					cnt.done = true
					if mode == "sort" {
						cnt.copied = e.Summary.Processed - e.Summary.Duplicates
						cnt.dupes = e.Summary.Duplicates
						cnt.skipped = e.Summary.Skipped
						cnt.errors = e.Summary.Errors
					} else {
						cnt.verified = e.Summary.Verified
						cnt.mismatches = e.Summary.Mismatches
						cnt.unrecognised = e.Summary.Unrecognised
					}
					cnt.mu.Unlock()
				}
				// Drain any remaining worker bars.
				for id := range workers {
					finishWorker(workers, id)
				}
				// Complete the overall progress bar; abort display-only bars.
				cur := totalBar.Current()
				totalBar.SetTotal(cur, true)
				headerBar.Abort(false)
				statusBar.Abort(false)
				spinnerBar.Abort(true)
			}
		}

		// Bus closed without EventRunComplete (e.g. interrupted).
		// Abort all bars so p.Wait() returns.
		for id := range workers {
			finishWorker(workers, id)
		}
		cur := totalBar.Current()
		totalBar.SetTotal(cur, true)
		headerBar.Abort(false)
		statusBar.Abort(false)
		spinnerBar.Abort(true)
	}()

	return p
}

// finishWorker aborts the worker's bar (removing it from the display) and
// deletes it from the workers map.
func finishWorker(workers map[int]*WorkerState, workerID int) {
	if ws, ok := workers[workerID]; ok {
		ws.Bar.Abort(true)
		delete(workers, workerID)
	}
}

// buildStatusLine renders the status counter line for the given mode.
func buildStatusLine(cnt *counters, mode string) string {
	cnt.mu.Lock()
	defer cnt.mu.Unlock()

	var line string
	if mode == "sort" {
		errStr := fmt.Sprintf("%d", cnt.errors)
		if cnt.errors > 0 {
			errStr = ansiWrap(errStr, ansiBoldRed)
		}
		line = fmt.Sprintf(" copied: %d  │  dupes: %d  │  skipped: %d  │  errors: %s",
			cnt.copied, cnt.dupes, cnt.skipped, errStr)
	} else {
		mmStr := fmt.Sprintf("%d", cnt.mismatches)
		if cnt.mismatches > 0 {
			mmStr = ansiWrap(mmStr, ansiBoldRed)
		}
		line = fmt.Sprintf(" verified: %d  │  mismatches: %s  │  unrecognised: %d",
			cnt.verified, mmStr, cnt.unrecognised)
	}

	if cnt.done && cnt.duration > 0 {
		line += "  " + ansiWrap("("+pixeprogress.FormatElapsedDuration(cnt.duration)+")", ansiDimGray)
	}
	return line
}
