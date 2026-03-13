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
	"io"
	"time"
)

// progressInterval is the minimum time between EventByteProgress emissions.
// A final event is always emitted at EOF regardless of this interval.
const progressInterval = 100 * time.Millisecond

// ProgressReader wraps an io.Reader and emits EventByteProgress events to the
// bus at time-throttled intervals. It is used to provide byte-level progress
// for hash, copy, and verify I/O operations.
//
// When bus is nil, Read delegates directly to the underlying reader with zero
// overhead — no allocations, no time checks.
type ProgressReader struct {
	r        io.Reader
	bus      *Bus
	relPath  string
	workerID int
	stage    string
	total    int64 // BytesTotal (file size); 0 if unknown.
	written  int64 // bytes read so far.
	lastEmit time.Time
}

// NewProgressReader creates a ProgressReader. If bus is nil, the returned
// reader passes through to r with no overhead.
func NewProgressReader(r io.Reader, bus *Bus, relPath string, workerID int, stage string, total int64) *ProgressReader {
	return &ProgressReader{
		r:        r,
		bus:      bus,
		relPath:  relPath,
		workerID: workerID,
		stage:    stage,
		total:    total,
	}
}

// Read implements io.Reader. After each underlying Read, it checks whether the
// throttle interval has elapsed and emits an EventByteProgress if so. A final
// event is always emitted when the underlying reader returns io.EOF.
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.r.Read(p)
	if pr.bus == nil {
		return n, err
	}

	pr.written += int64(n)

	atEOF := err == io.EOF
	now := time.Now()
	if atEOF || now.Sub(pr.lastEmit) >= progressInterval {
		total := pr.total
		if atEOF && total == 0 {
			total = pr.written
		}
		pr.bus.Emit(Event{
			Kind:         EventByteProgress,
			RelPath:      pr.relPath,
			WorkerID:     pr.workerID,
			Stage:        pr.stage,
			BytesWritten: pr.written,
			BytesTotal:   total,
		})
		pr.lastEmit = now
	}

	return n, err
}

// ProgressWriter wraps an io.Writer and emits EventByteProgress events to the
// bus at time-throttled intervals. It is used to provide byte-level progress
// for copy I/O operations where the destination writer is wrapped rather than
// the source reader.
//
// When bus is nil, Write delegates directly to the underlying writer with zero
// overhead — no allocations, no time checks.
type ProgressWriter struct {
	w        io.Writer
	bus      *Bus
	relPath  string
	workerID int
	stage    string
	total    int64 // BytesTotal (file size); 0 if unknown.
	written  int64 // bytes written so far.
	lastEmit time.Time
}

// NewProgressWriter creates a ProgressWriter. If bus is nil, the returned
// writer passes through to w with no overhead.
func NewProgressWriter(w io.Writer, bus *Bus, relPath string, workerID int, stage string, total int64) *ProgressWriter {
	return &ProgressWriter{
		w:        w,
		bus:      bus,
		relPath:  relPath,
		workerID: workerID,
		stage:    stage,
		total:    total,
	}
}

// Write implements io.Writer. After each underlying Write, it checks whether
// the throttle interval has elapsed and emits an EventByteProgress if so.
func (pw *ProgressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.w.Write(p)
	if pw.bus == nil {
		return n, err
	}

	pw.written += int64(n)

	now := time.Now()
	if now.Sub(pw.lastEmit) >= progressInterval {
		pw.bus.Emit(Event{
			Kind:         EventByteProgress,
			RelPath:      pw.relPath,
			WorkerID:     pw.workerID,
			Stage:        pw.stage,
			BytesWritten: pw.written,
			BytesTotal:   pw.total,
		})
		pw.lastEmit = now
	}

	return n, err
}

// EmitFinal emits a final EventByteProgress with BytesWritten == BytesTotal,
// signalling 100% completion for the current stage. Called by the pipeline
// after copy.Execute completes to ensure the UI sees 100% before the
// stage-transition event arrives.
func (pw *ProgressWriter) EmitFinal() {
	if pw.bus == nil {
		return
	}
	total := pw.total
	if total == 0 {
		total = pw.written
	}
	pw.bus.Emit(Event{
		Kind:         EventByteProgress,
		RelPath:      pw.relPath,
		WorkerID:     pw.workerID,
		Stage:        pw.stage,
		BytesWritten: total,
		BytesTotal:   total,
	})
}
