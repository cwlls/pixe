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

package progress

import (
	"fmt"
	"io"
)

// PlainWriter consumes events from a Bus and writes the traditional
// plain-text output (COPY, SKIP, DUPE, ERR lines and a summary) to an
// io.Writer. It is the reference implementation that proves the event bus
// carries enough structured data to reproduce the existing CLI output exactly.
//
// Usage:
//
//	pw := progress.NewPlainWriter(os.Stdout, "...Photos", 0)
//	go pw.Run(bus.Events())
type PlainWriter struct {
	w         io.Writer
	verbosity int
	destLabel string
}

// displayDest returns the destination path formatted for human output.
// When destLabel is non-empty, it prepends destLabel+"/" to dest.
func displayDest(destLabel, dest string) string {
	if destLabel == "" {
		return dest
	}
	return destLabel + "/" + dest
}

// NewPlainWriter creates a PlainWriter that writes to w.
// destLabel is the display prefix for destination paths (e.g. "...Photos").
// verbosity controls output detail: -1 = quiet (summary only),
// 0 = normal (default), 1 = verbose.
func NewPlainWriter(w io.Writer, destLabel string, verbosity ...int) *PlainWriter {
	v := 0
	if len(verbosity) > 0 {
		v = verbosity[0]
	}
	return &PlainWriter{w: w, destLabel: destLabel, verbosity: v}
}

// Run reads events from the channel and writes formatted output until the
// channel is closed. Intended to be called in a goroutine.
func (pw *PlainWriter) Run(events <-chan Event) {
	for e := range events {
		switch e.Kind {
		case EventFileComplete:
			if pw.verbosity >= 0 {
				_, _ = fmt.Fprintf(pw.w, "COPY %s -> %s\n", e.RelPath, displayDest(pw.destLabel, e.Destination))
			}

		case EventFileDuplicate:
			if pw.verbosity >= 0 {
				if e.MatchesDest != "" {
					_, _ = fmt.Fprintf(pw.w, "DUPE %s -> matches %s\n", e.RelPath, displayDest(pw.destLabel, e.MatchesDest))
				} else {
					_, _ = fmt.Fprintf(pw.w, "DUPE %s -> %s\n", e.RelPath, displayDest(pw.destLabel, e.Destination))
				}
			}

		case EventFileSkipped:
			if pw.verbosity >= 0 {
				_, _ = fmt.Fprintf(pw.w, "SKIP %s -> %s\n", e.RelPath, e.Reason)
			}

		case EventFileError:
			if pw.verbosity >= 0 {
				msg := e.Reason
				if msg == "" && e.Err != nil {
					msg = e.Err.Error()
				}
				_, _ = fmt.Fprintf(pw.w, "ERR  %s -> %s\n", e.RelPath, msg)
			}

		case EventSidecarCarried:
			if pw.verbosity >= 0 {
				_, _ = fmt.Fprintf(pw.w, "     +sidecar %s -> %s\n", e.SidecarRelPath, displayDest(pw.destLabel, e.Destination))
			}

		case EventSidecarFailed:
			if pw.verbosity >= 0 {
				_, _ = fmt.Fprintf(pw.w, "  WARNING  sidecar carry failed for %s: %v\n", e.SidecarRelPath, e.Err)
			}

		case EventRunComplete:
			if s := e.Summary; s != nil {
				_, _ = fmt.Fprintf(pw.w, "\nDone. processed=%d duplicates=%d skipped=%d errors=%d\n",
					s.Processed, s.Duplicates, s.Skipped, s.Errors)
			}

		// Verify events.
		case EventVerifyOK:
			if pw.verbosity >= 0 {
				_, _ = fmt.Fprintf(pw.w, "  OK            %s\n", e.RelPath)
			}

		case EventVerifyMismatch:
			if pw.verbosity >= 0 {
				if e.Err != nil {
					_, _ = fmt.Fprintf(pw.w, "  ERROR         %s: %v\n", e.RelPath, e.Err)
				} else {
					_, _ = fmt.Fprintf(pw.w, "  MISMATCH      %s\n    expected: %s\n    actual:   %s\n",
						e.RelPath, e.ExpectedChecksum, e.ActualChecksum)
				}
			}

		case EventVerifyUnrecognised:
			if pw.verbosity >= 0 {
				_, _ = fmt.Fprintf(pw.w, "  UNRECOGNISED  %s\n", e.RelPath)
			}

		case EventVerifyDone:
			if s := e.Summary; s != nil {
				_, _ = fmt.Fprintf(pw.w, "\nDone. verified=%d mismatches=%d unrecognised=%d\n",
					s.Verified, s.Mismatches, s.Unrecognised)
			}
		}
	}
}
