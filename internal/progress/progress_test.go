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
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Bus tests
// ---------------------------------------------------------------------------

func TestBus_EmitAndReceive(t *testing.T) {
	t.Helper()
	bus := NewBus(16)

	events := []Event{
		{Kind: EventFileStart, RelPath: "a.jpg"},
		{Kind: EventFileHashed, RelPath: "a.jpg", Checksum: "abc123"},
		{Kind: EventFileCopied, RelPath: "a.jpg"},
		{Kind: EventFileVerified, RelPath: "a.jpg"},
		{Kind: EventFileComplete, RelPath: "a.jpg", Destination: "2026/01-Jan/a.jpg"},
	}

	for _, e := range events {
		bus.Emit(e)
	}
	bus.Close()

	var received []Event
	for e := range bus.Events() {
		received = append(received, e)
	}

	if len(received) != len(events) {
		t.Fatalf("got %d events, want %d", len(received), len(events))
	}
	for i, e := range received {
		if e.Kind != events[i].Kind {
			t.Errorf("event[%d].Kind = %v, want %v", i, e.Kind, events[i].Kind)
		}
		if e.RelPath != events[i].RelPath {
			t.Errorf("event[%d].RelPath = %q, want %q", i, e.RelPath, events[i].RelPath)
		}
	}
}

func TestBus_NonBlockingEmit(t *testing.T) {
	// Buffer size 1 — most emits will be dropped, but none should block.
	bus := NewBus(1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			bus.Emit(Event{Kind: EventFileStart, RelPath: "x.jpg"})
		}
	}()

	select {
	case <-done:
		// Good — completed without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("Emit blocked for more than 2 seconds — non-blocking contract violated")
	}
	bus.Close()
}

func TestBus_Close(t *testing.T) {
	bus := NewBus(8)
	bus.Emit(Event{Kind: EventFileStart})
	bus.Emit(Event{Kind: EventFileComplete})
	bus.Close()

	// Range should exit after close.
	count := 0
	for range bus.Events() {
		count++
	}
	if count != 2 {
		t.Errorf("got %d events after close, want 2", count)
	}
}

func TestBus_CloseIdempotent(t *testing.T) {
	bus := NewBus(8)
	// Should not panic.
	bus.Close()
	bus.Close()
	bus.Close()
}

func TestBus_EmitAfterClose(t *testing.T) {
	bus := NewBus(8)
	bus.Close()
	// Should not panic.
	bus.Emit(Event{Kind: EventFileStart, RelPath: "x.jpg"})
}

func TestBus_TimestampSet(t *testing.T) {
	bus := NewBus(8)
	before := time.Now()
	bus.Emit(Event{Kind: EventFileStart})
	bus.Close()

	e := <-bus.Events()
	if e.Timestamp.IsZero() {
		t.Fatal("Timestamp was not set by Emit")
	}
	if e.Timestamp.Before(before) {
		t.Errorf("Timestamp %v is before emit time %v", e.Timestamp, before)
	}
}

// ---------------------------------------------------------------------------
// PlainWriter tests
// ---------------------------------------------------------------------------

func TestPlainWriter_SortOutput(t *testing.T) {
	bus := NewBus(32)
	var buf bytes.Buffer
	pw := NewPlainWriter(&buf, "...Photos")

	// Emit a sequence of sort events.
	bus.Emit(Event{Kind: EventFileSkipped, RelPath: "skip.jpg", Reason: "unsupported"})
	bus.Emit(Event{Kind: EventFileComplete, RelPath: "copy.jpg", Destination: "2026/01-Jan/copy.jpg"})
	bus.Emit(Event{Kind: EventFileDuplicate, RelPath: "dupe.jpg", MatchesDest: "2026/01-Jan/orig.jpg"})
	bus.Emit(Event{Kind: EventFileError, RelPath: "err.jpg", Reason: "hash failed"})
	bus.Emit(Event{Kind: EventRunComplete, Summary: &RunSummary{
		Processed:  2,
		Duplicates: 1,
		Skipped:    1,
		Errors:     1,
	}})
	bus.Close()

	pw.Run(bus.Events())

	out := buf.String()
	wantLines := []string{
		"SKIP skip.jpg -> unsupported",
		"COPY copy.jpg -> ...Photos/2026/01-Jan/copy.jpg",
		"DUPE dupe.jpg -> matches ...Photos/2026/01-Jan/orig.jpg",
		"ERR  err.jpg -> hash failed",
		"Done. processed=2 duplicates=1 skipped=1 errors=1",
	}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestPlainWriter_VerifyOutput(t *testing.T) {
	bus := NewBus(32)
	var buf bytes.Buffer
	pw := NewPlainWriter(&buf, "")

	bus.Emit(Event{Kind: EventVerifyOK, RelPath: "ok.jpg"})
	bus.Emit(Event{
		Kind:             EventVerifyMismatch,
		RelPath:          "bad.jpg",
		ExpectedChecksum: "aaa",
		ActualChecksum:   "bbb",
	})
	bus.Emit(Event{Kind: EventVerifyUnrecognised, RelPath: "unknown.txt"})
	bus.Emit(Event{Kind: EventVerifyMismatch, RelPath: "err.jpg", Err: errors.New("read error")})
	bus.Emit(Event{Kind: EventVerifyDone, Summary: &RunSummary{
		Verified:     1,
		Mismatches:   2,
		Unrecognised: 1,
	}})
	bus.Close()

	pw.Run(bus.Events())

	out := buf.String()
	wantLines := []string{
		"OK            ok.jpg",
		"MISMATCH      bad.jpg",
		"expected: aaa",
		"actual:   bbb",
		"UNRECOGNISED  unknown.txt",
		"ERROR         err.jpg",
		"Done. verified=1 mismatches=2 unrecognised=1",
	}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}
