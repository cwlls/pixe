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
	"io"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ProgressReader tests
// ---------------------------------------------------------------------------

// TestProgressReader_NilBus verifies that when bus is nil, Read delegates
// directly to the underlying reader with zero overhead.
func TestProgressReader_NilBus(t *testing.T) {
	t.Parallel()
	data := []byte("hello world")
	underlying := bytes.NewReader(data)

	pr := NewProgressReader(underlying, nil, "test.jpg", 1, "HASH", int64(len(data)))

	// Read should delegate directly to underlying reader.
	buf := make([]byte, 5)
	n, err := pr.Read(buf)
	if n != 5 {
		t.Errorf("Read returned n=%d, want 5", n)
	}
	if err != nil {
		t.Errorf("Read returned err=%v, want nil", err)
	}
	if !bytes.Equal(buf, []byte("hello")) {
		t.Errorf("Read returned %q, want %q", buf, "hello")
	}

	// Read again to get the rest.
	buf2 := make([]byte, 10)
	n, _ = pr.Read(buf2)
	if n != 6 {
		t.Errorf("second Read returned n=%d, want 6", n)
	}
	// bytes.Reader returns io.EOF on the next read after exhausting data.
	// Read the third time to get EOF.
	buf3 := make([]byte, 1)
	_, err = pr.Read(buf3)
	if err != io.EOF {
		t.Errorf("third Read returned err=%v, want io.EOF", err)
	}
}

// TestProgressReader_EmitsOnThrottle verifies that events are emitted at
// throttled intervals, not on every Read call.
func TestProgressReader_EmitsOnThrottle(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	// Create a reader that returns 1 byte at a time.
	data := []byte("0123456789")
	underlying := bytes.NewReader(data)

	pr := NewProgressReader(underlying, bus, "test.jpg", 1, "HASH", int64(len(data)))

	// Read 10 times (1 byte each).
	for i := 0; i < 10; i++ {
		buf := make([]byte, 1)
		_, _ = pr.Read(buf)
	}

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	// With throttling at 100ms and rapid reads, we expect fewer events than reads.
	// The exact count depends on timing, but it should be significantly less than 10.
	if len(events) >= 10 {
		t.Errorf("got %d events for 10 reads, want fewer due to throttling", len(events))
	}
	if len(events) == 0 {
		t.Error("got 0 events, want at least 1 (final EOF event)")
	}
}

// TestProgressReader_EOFFinalEvent verifies that a final event is emitted
// when EOF is reached, with BytesWritten == BytesTotal.
func TestProgressReader_EOFFinalEvent(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	data := []byte("test data")
	underlying := bytes.NewReader(data)

	pr := NewProgressReader(underlying, bus, "test.jpg", 1, "HASH", int64(len(data)))

	// Read all data in one call.
	buf := make([]byte, 100)
	n, _ := pr.Read(buf)
	if n != len(data) {
		t.Fatalf("Read returned n=%d, want %d", n, len(data))
	}
	// First read returns the data without EOF.
	// Read again to get EOF.
	buf2 := make([]byte, 1)
	_, err := pr.Read(buf2)
	if err != io.EOF {
		t.Fatalf("second Read returned err=%v, want io.EOF", err)
	}

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	if len(events) == 0 {
		t.Fatal("got 0 events, want at least 1 (final EOF event)")
	}

	// Last event should have BytesWritten == BytesTotal.
	lastEvent := events[len(events)-1]
	if lastEvent.BytesWritten != lastEvent.BytesTotal {
		t.Errorf("final event: BytesWritten=%d, BytesTotal=%d, want equal",
			lastEvent.BytesWritten, lastEvent.BytesTotal)
	}
	if lastEvent.BytesTotal != int64(len(data)) {
		t.Errorf("final event: BytesTotal=%d, want %d", lastEvent.BytesTotal, len(data))
	}
}

// TestProgressReader_UnknownSize verifies that when total is 0 (unknown size),
// the final EOF event sets BytesTotal to the actual bytes read.
func TestProgressReader_UnknownSize(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	data := []byte("test data")
	underlying := bytes.NewReader(data)

	// Create reader with total=0 (unknown size).
	pr := NewProgressReader(underlying, bus, "test.jpg", 1, "HASH", 0)

	// Read all data in one call.
	buf := make([]byte, 100)
	n, _ := pr.Read(buf)
	if n != len(data) {
		t.Fatalf("Read returned n=%d, want %d", n, len(data))
	}
	// Read again to get EOF.
	buf2 := make([]byte, 1)
	_, err := pr.Read(buf2)
	if err != io.EOF {
		t.Fatalf("second Read returned err=%v, want io.EOF", err)
	}

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	if len(events) == 0 {
		t.Fatal("got 0 events, want at least 1 (final EOF event)")
	}

	// Final event should have BytesTotal set to actual bytes read.
	lastEvent := events[len(events)-1]
	if lastEvent.BytesTotal != int64(len(data)) {
		t.Errorf("final event: BytesTotal=%d, want %d (actual bytes read)", lastEvent.BytesTotal, len(data))
	}
}

// TestProgressReader_EventFields verifies that emitted events have correct fields.
func TestProgressReader_EventFields(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	data := []byte("test")
	underlying := bytes.NewReader(data)

	pr := NewProgressReader(underlying, bus, "photo.jpg", 2, "HASH", int64(len(data)))

	// Read all data.
	buf := make([]byte, 100)
	_, _ = pr.Read(buf)

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	if len(events) == 0 {
		t.Fatal("got 0 events")
	}

	// Check fields in the final event.
	lastEvent := events[len(events)-1]
	if lastEvent.Kind != EventByteProgress {
		t.Errorf("event.Kind = %v, want EventByteProgress", lastEvent.Kind)
	}
	if lastEvent.RelPath != "photo.jpg" {
		t.Errorf("event.RelPath = %q, want %q", lastEvent.RelPath, "photo.jpg")
	}
	if lastEvent.WorkerID != 2 {
		t.Errorf("event.WorkerID = %d, want 2", lastEvent.WorkerID)
	}
	if lastEvent.Stage != "HASH" {
		t.Errorf("event.Stage = %q, want %q", lastEvent.Stage, "HASH")
	}
}

// ---------------------------------------------------------------------------
// ProgressWriter tests
// ---------------------------------------------------------------------------

// TestProgressWriter_NilBus verifies that when bus is nil, Write delegates
// directly to the underlying writer with zero overhead.
func TestProgressWriter_NilBus(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, nil, "test.jpg", 1, "COPY", 100)

	data := []byte("hello world")
	n, err := pw.Write(data)
	if n != len(data) {
		t.Errorf("Write returned n=%d, want %d", n, len(data))
	}
	if err != nil {
		t.Errorf("Write returned err=%v, want nil", err)
	}
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("buffer contains %q, want %q", buf.Bytes(), data)
	}
}

// TestProgressWriter_EmitsOnThrottle verifies that events are emitted at
// throttled intervals, not on every Write call.
func TestProgressWriter_EmitsOnThrottle(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, bus, "test.jpg", 1, "COPY", 100)

	// Write 10 times (1 byte each).
	for i := 0; i < 10; i++ {
		_, _ = pw.Write([]byte("x"))
	}

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	// With throttling at 100ms and rapid writes, we expect fewer events than writes.
	if len(events) >= 10 {
		t.Errorf("got %d events for 10 writes, want fewer due to throttling", len(events))
	}
}

// TestProgressWriter_EmitFinal verifies that EmitFinal() emits an event with
// BytesWritten == BytesTotal.
func TestProgressWriter_EmitFinal(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, bus, "test.jpg", 1, "COPY", 100)

	// Write some data.
	_, _ = pw.Write([]byte("hello"))

	// Emit final event.
	pw.EmitFinal()

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	if len(events) == 0 {
		t.Fatal("got 0 events, want at least 1 (final event)")
	}

	// Last event should have BytesWritten == BytesTotal.
	lastEvent := events[len(events)-1]
	if lastEvent.BytesWritten != lastEvent.BytesTotal {
		t.Errorf("final event: BytesWritten=%d, BytesTotal=%d, want equal",
			lastEvent.BytesWritten, lastEvent.BytesTotal)
	}
	if lastEvent.BytesTotal != 100 {
		t.Errorf("final event: BytesTotal=%d, want 100", lastEvent.BytesTotal)
	}
}

// TestProgressWriter_EmitFinalUnknownSize verifies that EmitFinal() with
// unknown total (0) sets BytesTotal to actual bytes written.
func TestProgressWriter_EmitFinalUnknownSize(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, bus, "test.jpg", 1, "COPY", 0)

	// Write some data.
	_, _ = pw.Write([]byte("hello"))

	// Emit final event.
	pw.EmitFinal()

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	if len(events) == 0 {
		t.Fatal("got 0 events")
	}

	// Final event should have BytesTotal set to actual bytes written.
	lastEvent := events[len(events)-1]
	if lastEvent.BytesTotal != 5 {
		t.Errorf("final event: BytesTotal=%d, want 5 (actual bytes written)", lastEvent.BytesTotal)
	}
}

// TestProgressWriter_EmitFinalNilBus verifies that EmitFinal() is a no-op
// when bus is nil.
func TestProgressWriter_EmitFinalNilBus(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, nil, "test.jpg", 1, "COPY", 100)

	// Should not panic.
	pw.EmitFinal()
}

// TestProgressWriter_EventFields verifies that emitted events have correct fields.
func TestProgressWriter_EventFields(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, bus, "photo.jpg", 3, "COPY", 1000)

	// Write some data.
	_, _ = pw.Write([]byte("test"))

	// Emit final event.
	pw.EmitFinal()

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	if len(events) == 0 {
		t.Fatal("got 0 events")
	}

	// Check fields in the final event.
	lastEvent := events[len(events)-1]
	if lastEvent.Kind != EventByteProgress {
		t.Errorf("event.Kind = %v, want EventByteProgress", lastEvent.Kind)
	}
	if lastEvent.RelPath != "photo.jpg" {
		t.Errorf("event.RelPath = %q, want %q", lastEvent.RelPath, "photo.jpg")
	}
	if lastEvent.WorkerID != 3 {
		t.Errorf("event.WorkerID = %d, want 3", lastEvent.WorkerID)
	}
	if lastEvent.Stage != "COPY" {
		t.Errorf("event.Stage = %q, want %q", lastEvent.Stage, "COPY")
	}
}

// TestProgressWriter_MultipleWrites verifies that BytesWritten accumulates
// across multiple Write calls.
func TestProgressWriter_MultipleWrites(t *testing.T) {
	t.Parallel()
	bus := NewBus(32)
	defer bus.Close()

	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, bus, "test.jpg", 1, "COPY", 100)

	// Write in chunks.
	_, _ = pw.Write([]byte("hello"))
	time.Sleep(150 * time.Millisecond) // Wait for throttle interval.
	_, _ = pw.Write([]byte("world"))

	bus.Close()

	// Collect events.
	var events []Event
	for e := range bus.Events() {
		if e.Kind == EventByteProgress {
			events = append(events, e)
		}
	}

	if len(events) < 2 {
		t.Errorf("got %d events, want at least 2 (one per throttle interval)", len(events))
	}

	// Check that BytesWritten increases.
	if len(events) >= 2 {
		if events[0].BytesWritten >= events[1].BytesWritten {
			t.Errorf("BytesWritten did not increase: %d -> %d", events[0].BytesWritten, events[1].BytesWritten)
		}
	}
}
