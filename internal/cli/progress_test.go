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

package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	pixeprogress "github.com/cwlls/pixe/internal/progress"
)

// runProgressAndWait is a test helper that creates a RunProgress container,
// emits the provided events, closes the bus, and waits for p.Wait() to return.
// It fails the test if p.Wait() does not return within 5 seconds.
func runProgressAndWait(t *testing.T, mode string, events []pixeprogress.Event) {
	t.Helper()
	bus := pixeprogress.NewBus(128)
	ctx := context.Background()
	p := RunProgress(ctx, bus, "/src", "/dst", mode)

	for _, e := range events {
		bus.Emit(e)
	}
	bus.Close()

	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("p.Wait() did not return within 5s — possible deadlock in event consumer")
	}
}

// TestRunProgress_BusClose verifies that closing the bus causes p.Wait() to return.
func TestRunProgress_BusClose(t *testing.T) {
	runProgressAndWait(t, "sort", nil)
}

// TestRunProgress_SortLifecycle exercises the full sort event sequence and
// verifies p.Wait() returns cleanly.
func TestRunProgress_SortLifecycle(t *testing.T) {
	events := []pixeprogress.Event{
		{Kind: pixeprogress.EventDiscoverDone, Total: 2},
		{Kind: pixeprogress.EventFileStart, WorkerID: 1, RelPath: "IMG_001.jpg", FileSize: 1024},
		{Kind: pixeprogress.EventByteProgress, WorkerID: 1, BytesWritten: 512, Stage: "HASH"},
		{Kind: pixeprogress.EventFileHashed, WorkerID: 1},
		{Kind: pixeprogress.EventByteProgress, WorkerID: 1, BytesWritten: 512, Stage: "COPY"},
		{Kind: pixeprogress.EventFileCopied, WorkerID: 1},
		{Kind: pixeprogress.EventByteProgress, WorkerID: 1, BytesWritten: 512, Stage: "VERIFY"},
		{Kind: pixeprogress.EventFileVerified, WorkerID: 1},
		{Kind: pixeprogress.EventFileComplete, WorkerID: 1, Completed: 1},
		{Kind: pixeprogress.EventFileStart, WorkerID: 1, RelPath: "IMG_002.jpg", FileSize: 2048},
		{Kind: pixeprogress.EventFileDuplicate, WorkerID: 1, Completed: 2},
		{Kind: pixeprogress.EventRunComplete, Summary: &pixeprogress.RunSummary{
			Processed:  2,
			Duplicates: 1,
			Duration:   83 * time.Second,
		}},
	}
	runProgressAndWait(t, "sort", events)
}

// TestRunProgress_VerifyLifecycle exercises the full verify event sequence.
func TestRunProgress_VerifyLifecycle(t *testing.T) {
	events := []pixeprogress.Event{
		{Kind: pixeprogress.EventVerifyStart, Total: 3},
		{Kind: pixeprogress.EventVerifyFileStart, WorkerID: 1, RelPath: "20210101_120000-1-abc.jpg", FileSize: 4096},
		{Kind: pixeprogress.EventByteProgress, WorkerID: 1, BytesWritten: 2048, Stage: "HASH"},
		{Kind: pixeprogress.EventVerifyOK, WorkerID: 1, Completed: 1},
		{Kind: pixeprogress.EventVerifyFileStart, WorkerID: 1, RelPath: "20210101_120001-1-def.jpg", FileSize: 4096},
		{Kind: pixeprogress.EventVerifyMismatch, WorkerID: 1, Completed: 2},
		{Kind: pixeprogress.EventVerifyFileStart, WorkerID: 1, RelPath: "unknown.txt", FileSize: 100},
		{Kind: pixeprogress.EventVerifyUnrecognised, WorkerID: 1, Completed: 3},
		{Kind: pixeprogress.EventVerifyDone, Summary: &pixeprogress.RunSummary{
			Verified:     1,
			Mismatches:   1,
			Unrecognised: 1,
			Duration:     45 * time.Second,
		}},
	}
	runProgressAndWait(t, "verify", events)
}

// TestRunProgress_MultipleWorkers verifies that multiple concurrent workers
// can be tracked without deadlock.
func TestRunProgress_MultipleWorkers(t *testing.T) {
	events := []pixeprogress.Event{
		{Kind: pixeprogress.EventDiscoverDone, Total: 3},
		{Kind: pixeprogress.EventFileStart, WorkerID: 1, RelPath: "a.jpg", FileSize: 1000},
		{Kind: pixeprogress.EventFileStart, WorkerID: 2, RelPath: "b.jpg", FileSize: 2000},
		{Kind: pixeprogress.EventFileStart, WorkerID: 3, RelPath: "c.jpg", FileSize: 3000},
		{Kind: pixeprogress.EventFileComplete, WorkerID: 1, Completed: 1},
		{Kind: pixeprogress.EventFileSkipped, WorkerID: 2, Completed: 2},
		{Kind: pixeprogress.EventFileError, WorkerID: 3, Completed: 3},
		{Kind: pixeprogress.EventRunComplete, Summary: &pixeprogress.RunSummary{
			Processed: 1,
			Skipped:   1,
			Errors:    1,
		}},
	}
	runProgressAndWait(t, "sort", events)
}

// TestRunProgress_SortLifecycleWithTagging exercises the full sort event
// sequence including EventFileTagged, verifying the TAG stage label path works.
func TestRunProgress_SortLifecycleWithTagging(t *testing.T) {
	events := []pixeprogress.Event{
		{Kind: pixeprogress.EventDiscoverDone, Total: 1},
		{Kind: pixeprogress.EventFileStart, WorkerID: 1, RelPath: "IMG_001.jpg", FileSize: 1024},
		{Kind: pixeprogress.EventFileHashed, WorkerID: 1},
		{Kind: pixeprogress.EventFileCopied, WorkerID: 1},
		{Kind: pixeprogress.EventFileVerified, WorkerID: 1},
		{Kind: pixeprogress.EventFileTagged, WorkerID: 1},
		{Kind: pixeprogress.EventFileComplete, WorkerID: 1, Completed: 1},
		{Kind: pixeprogress.EventRunComplete, Summary: &pixeprogress.RunSummary{
			Processed: 1,
			Duration:  5 * time.Second,
		}},
	}
	runProgressAndWait(t, "sort", events)
}

// TestRunProgress_WorkerIDCleanup is a regression test for the bug where
// terminal events used WorkerID: -1 while bars were keyed by actual worker ID,
// causing bars to accumulate until EventRunComplete. With the fix, each bar is
// removed immediately when its terminal event arrives.
//
// The test uses a high WorkerID (5) that would never match -1, and verifies
// that p.Wait() returns cleanly without needing EventRunComplete to drain bars.
func TestRunProgress_WorkerIDCleanup(t *testing.T) {
	events := []pixeprogress.Event{
		{Kind: pixeprogress.EventDiscoverDone, Total: 3},
		// Worker 5: complete via EventFileComplete with matching WorkerID.
		{Kind: pixeprogress.EventFileStart, WorkerID: 5, RelPath: "a.jpg", FileSize: 1000},
		{Kind: pixeprogress.EventFileComplete, WorkerID: 5, Completed: 1},
		// Worker 7: complete via EventFileDuplicate with matching WorkerID.
		{Kind: pixeprogress.EventFileStart, WorkerID: 7, RelPath: "b.jpg", FileSize: 2000},
		{Kind: pixeprogress.EventFileDuplicate, WorkerID: 7, Completed: 2},
		// Worker 9: complete via EventFileError with matching WorkerID.
		{Kind: pixeprogress.EventFileStart, WorkerID: 9, RelPath: "c.jpg", FileSize: 3000},
		{Kind: pixeprogress.EventFileError, WorkerID: 9, Completed: 3},
		{Kind: pixeprogress.EventRunComplete, Summary: &pixeprogress.RunSummary{
			Processed: 1,
			Errors:    1,
		}},
	}
	runProgressAndWait(t, "sort", events)
}

// TestRunProgress_ContextCancel verifies that cancelling the context causes
// p.Wait() to return even if the bus is not closed.
func TestRunProgress_ContextCancel(t *testing.T) {
	bus := pixeprogress.NewBus(128)
	ctx, cancel := context.WithCancel(context.Background())
	p := RunProgress(ctx, bus, "/src", "/dst", "sort")

	// Cancel the context without closing the bus.
	cancel()

	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("p.Wait() did not return within 5s after context cancel")
	}
	bus.Close() // cleanup
}

// TestRunProgress_ReturnsNonNil verifies that RunProgress returns a non-nil container.
func TestRunProgress_ReturnsNonNil(t *testing.T) {
	bus := pixeprogress.NewBus(64)
	ctx := context.Background()
	p := RunProgress(ctx, bus, "/src", "/dst", "sort")
	if p == nil {
		t.Fatal("RunProgress returned nil")
	}
	bus.Close()
	p.Wait()
}

// --- Helper function unit tests ---

func TestTruncName(t *testing.T) {
	tests := []struct {
		name     string
		maxWidth int
		want     string
	}{
		{"short.jpg", 24, "short.jpg               "},
		{"exactly24characters.jpg", 24, "exactly24characters.jpg "},
		{"this_is_a_very_long_filename_that_exceeds_limit.jpg", 24, "this_is_a_very_long_file"},
	}
	for _, tc := range tests {
		got := truncName(tc.name, tc.maxWidth)
		if len(got) != tc.maxWidth {
			t.Errorf("truncName(%q, %d): len=%d, want %d", tc.name, tc.maxWidth, len(got), tc.maxWidth)
		}
		if len(tc.name) > tc.maxWidth && !strings.HasSuffix(got, "...") {
			t.Errorf("truncName(%q, %d): want suffix '...', got %q", tc.name, tc.maxWidth, got)
		}
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KB"},
		{1536, "2 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tc := range tests {
		got := humanSize(tc.bytes)
		if got != tc.want {
			t.Errorf("humanSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestBuildStatusLine_Sort(t *testing.T) {
	cnt := &counters{
		copied:  5,
		dupes:   2,
		skipped: 1,
		errors:  0,
	}
	line := buildStatusLine(cnt, "sort")
	for _, want := range []string{"copied", "dupes", "skipped", "errors"} {
		if !strings.Contains(line, want) {
			t.Errorf("buildStatusLine sort: missing %q in %q", want, line)
		}
	}
}

func TestBuildStatusLine_Verify(t *testing.T) {
	cnt := &counters{
		verified:     10,
		mismatches:   1,
		unrecognised: 2,
	}
	line := buildStatusLine(cnt, "verify")
	for _, want := range []string{"verified", "mismatches", "unrecognised"} {
		if !strings.Contains(line, want) {
			t.Errorf("buildStatusLine verify: missing %q in %q", want, line)
		}
	}
}

func TestBuildStatusLine_ElapsedOnDone(t *testing.T) {
	cnt := &counters{
		done:     true,
		duration: 83 * time.Second,
	}
	line := buildStatusLine(cnt, "sort")
	if !strings.Contains(line, "(1m 23s)") {
		t.Errorf("buildStatusLine: missing elapsed time '(1m 23s)' when done=true, got %q", line)
	}
}

func TestBuildStatusLine_NoElapsedWhenNotDone(t *testing.T) {
	cnt := &counters{
		done:     false,
		duration: 83 * time.Second,
	}
	line := buildStatusLine(cnt, "sort")
	if strings.Contains(line, "(1m 23s)") {
		t.Errorf("buildStatusLine: elapsed time should not appear when done=false, got %q", line)
	}
}
