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
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	pixeprogress "github.com/cwlls/pixe/internal/progress"
)

// newTestModel creates a ProgressModel with a real bus for testing.
func newTestModel(mode string) (ProgressModel, *pixeprogress.Bus) {
	bus := pixeprogress.NewBus(64)
	m := NewProgressModel(bus, "/src", "/dst", mode)
	return m, bus
}

// sendEvent sends an eventMsg to the model and returns the updated model.
func sendEvent(t *testing.T, m ProgressModel, e pixeprogress.Event) ProgressModel {
	t.Helper()
	updated, _ := m.Update(eventMsg{event: e})
	return updated.(ProgressModel)
}

func TestProgressModel_Init(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd, want non-nil")
	}
}

func TestProgressModel_CounterUpdates(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileComplete, Completed: 1})
	if m.copied != 1 {
		t.Errorf("copied = %d, want 1", m.copied)
	}

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileDuplicate, Completed: 2})
	if m.duplicates != 1 {
		t.Errorf("duplicates = %d, want 1", m.duplicates)
	}

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileSkipped, Completed: 3})
	if m.skipped != 1 {
		t.Errorf("skipped = %d, want 1", m.skipped)
	}

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileError, Completed: 4})
	if m.errors != 1 {
		t.Errorf("errors = %d, want 1", m.errors)
	}
}

func TestProgressModel_DiscoverDone(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventDiscoverDone, Total: 100})
	if m.total != 100 {
		t.Errorf("total = %d, want 100", m.total)
	}
}

func TestProgressModel_WorkerTracking(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	// EventFileStart should create a WorkerState entry.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileStart,
		RelPath:  "IMG_001.jpg",
		WorkerID: 1,
		FileSize: 1024,
	})
	ws, ok := m.workers[1]
	if !ok {
		t.Fatal("workers[1] not found after EventFileStart")
	}
	if ws.RelPath != "IMG_001.jpg" {
		t.Errorf("workers[1].RelPath = %q, want %q", ws.RelPath, "IMG_001.jpg")
	}
	if ws.Stage != "HASH" {
		t.Errorf("workers[1].Stage = %q, want %q", ws.Stage, "HASH")
	}
	if ws.FileSize != 1024 {
		t.Errorf("workers[1].FileSize = %d, want 1024", ws.FileSize)
	}

	// EventByteProgress should update BytesWritten.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:         pixeprogress.EventByteProgress,
		RelPath:      "IMG_001.jpg",
		WorkerID:     1,
		Stage:        "HASH",
		BytesWritten: 512,
		BytesTotal:   1024,
	})
	if m.workers[1].BytesWritten != 512 {
		t.Errorf("workers[1].BytesWritten = %d, want 512", m.workers[1].BytesWritten)
	}

	// EventFileHashed should advance stage to COPY and reset BytesWritten.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileHashed,
		RelPath:  "IMG_001.jpg",
		WorkerID: 1,
	})
	if m.workers[1].Stage != "COPY" {
		t.Errorf("workers[1].Stage = %q, want %q", m.workers[1].Stage, "COPY")
	}
	if m.workers[1].BytesWritten != 0 {
		t.Errorf("workers[1].BytesWritten = %d, want 0 after stage reset", m.workers[1].BytesWritten)
	}

	// EventFileComplete should remove the worker entry.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:      pixeprogress.EventFileComplete,
		RelPath:   "IMG_001.jpg",
		WorkerID:  1,
		Completed: 1,
	})
	if _, ok := m.workers[1]; ok {
		t.Error("workers[1] still present after EventFileComplete, want removed")
	}
}

func TestProgressModel_DoneOnRunComplete(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	updated, cmd := m.Update(eventMsg{event: pixeprogress.Event{
		Kind:    pixeprogress.EventRunComplete,
		Summary: &pixeprogress.RunSummary{Processed: 5},
	}})
	m = updated.(ProgressModel)

	if !m.done {
		t.Error("done = false, want true after EventRunComplete")
	}
	if cmd == nil {
		t.Error("cmd should be tea.Quit, got nil")
	}
}

func TestProgressModel_DoneOnBusClosed(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	updated, cmd := m.Update(busClosedMsg{})
	m = updated.(ProgressModel)

	if !m.done {
		t.Error("done = false, want true after busClosedMsg")
	}
	if cmd == nil {
		t.Error("cmd should be tea.Quit, got nil")
	}
}

func TestProgressModel_WindowResize(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(ProgressModel)

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
}

func TestProgressModel_ViewContainsCounters(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventDiscoverDone, Total: 10})
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileComplete, Completed: 1})
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileDuplicate, Completed: 2})
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileSkipped, Completed: 3})
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileError, Completed: 4})

	view := m.View()
	wantStrings := []string{"copied", "dupes", "skipped", "errors"}
	for _, want := range wantStrings {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q\ngot:\n%s", want, view)
		}
	}
}

func TestProgressModel_VerifyMode(t *testing.T) {
	m, bus := newTestModel("verify")
	defer bus.Close()

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventVerifyStart, Total: 20})
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventVerifyOK, Completed: 1})
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventVerifyMismatch, Completed: 2})

	if m.verified != 1 {
		t.Errorf("verified = %d, want 1", m.verified)
	}
	if m.mismatches != 1 {
		t.Errorf("mismatches = %d, want 1", m.mismatches)
	}

	view := m.View()
	if !strings.Contains(view, "verified") {
		t.Errorf("View() missing 'verified' in verify mode\ngot:\n%s", view)
	}
	if !strings.Contains(view, "mismatches") {
		t.Errorf("View() missing 'mismatches' in verify mode\ngot:\n%s", view)
	}
}

// TestProgressModel_DiscoveringState verifies that discovering is true initially
// and becomes false after EventDiscoverDone.
func TestProgressModel_DiscoveringState(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	// Initially discovering should be true.
	if !m.discovering {
		t.Error("discovering = false initially, want true")
	}

	// After EventDiscoverDone, discovering should be false.
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventDiscoverDone, Total: 10})
	if m.discovering {
		t.Error("discovering = true after EventDiscoverDone, want false")
	}
}

// TestProgressModel_MultipleWorkers verifies that multiple workers are tracked
// independently.
func TestProgressModel_MultipleWorkers(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	// Send EventFileStart for workers 1, 2, 3 simultaneously.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileStart,
		RelPath:  "IMG_001.jpg",
		WorkerID: 1,
		FileSize: 1024,
	})
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileStart,
		RelPath:  "IMG_002.jpg",
		WorkerID: 2,
		FileSize: 2048,
	})
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileStart,
		RelPath:  "IMG_003.jpg",
		WorkerID: 3,
		FileSize: 4096,
	})

	// All three workers should be tracked.
	if len(m.workers) != 3 {
		t.Errorf("len(workers) = %d, want 3", len(m.workers))
	}

	// Each should have correct RelPath and FileSize.
	if m.workers[1].RelPath != "IMG_001.jpg" || m.workers[1].FileSize != 1024 {
		t.Errorf("worker 1: RelPath=%q FileSize=%d, want IMG_001.jpg 1024", m.workers[1].RelPath, m.workers[1].FileSize)
	}
	if m.workers[2].RelPath != "IMG_002.jpg" || m.workers[2].FileSize != 2048 {
		t.Errorf("worker 2: RelPath=%q FileSize=%d, want IMG_002.jpg 2048", m.workers[2].RelPath, m.workers[2].FileSize)
	}
	if m.workers[3].RelPath != "IMG_003.jpg" || m.workers[3].FileSize != 4096 {
		t.Errorf("worker 3: RelPath=%q FileSize=%d, want IMG_003.jpg 4096", m.workers[3].RelPath, m.workers[3].FileSize)
	}
}

// TestProgressModel_StageTransitions verifies the full lifecycle of a single
// worker through HASH → COPY → VERIFY → TAG → complete.
func TestProgressModel_StageTransitions(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	workerID := 1

	// EventFileStart: stage should be HASH.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileStart,
		RelPath:  "photo.jpg",
		WorkerID: workerID,
		FileSize: 1000,
	})
	if m.workers[workerID].Stage != "HASH" {
		t.Errorf("after EventFileStart: Stage=%q, want HASH", m.workers[workerID].Stage)
	}

	// EventFileHashed: stage should advance to COPY.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileHashed,
		RelPath:  "photo.jpg",
		WorkerID: workerID,
	})
	if m.workers[workerID].Stage != "COPY" {
		t.Errorf("after EventFileHashed: Stage=%q, want COPY", m.workers[workerID].Stage)
	}

	// EventFileCopied: stage should advance to VERIFY.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileCopied,
		RelPath:  "photo.jpg",
		WorkerID: workerID,
	})
	if m.workers[workerID].Stage != "VERIFY" {
		t.Errorf("after EventFileCopied: Stage=%q, want VERIFY", m.workers[workerID].Stage)
	}

	// EventFileVerified: stage should advance to TAG.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileVerified,
		RelPath:  "photo.jpg",
		WorkerID: workerID,
	})
	if m.workers[workerID].Stage != "TAG" {
		t.Errorf("after EventFileVerified: Stage=%q, want TAG", m.workers[workerID].Stage)
	}

	// EventFileComplete: worker should be removed.
	m = sendEvent(t, m, pixeprogress.Event{
		Kind:      pixeprogress.EventFileComplete,
		RelPath:   "photo.jpg",
		WorkerID:  workerID,
		Completed: 1,
	})
	if _, ok := m.workers[workerID]; ok {
		t.Error("after EventFileComplete: worker still present, want removed")
	}
}

// TestProgressModel_ViewContainsWorkerLine verifies that after EventFileStart,
// View() contains the stage label and filename.
func TestProgressModel_ViewContainsWorkerLine(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	m = sendEvent(t, m, pixeprogress.Event{
		Kind:     pixeprogress.EventFileStart,
		RelPath:  "vacation_photo.jpg",
		WorkerID: 1,
		FileSize: 2048,
	})

	view := m.View()
	if !strings.Contains(view, "vacation_photo.jpg") {
		t.Errorf("View() missing filename 'vacation_photo.jpg'\ngot:\n%s", view)
	}
	if !strings.Contains(view, "HASH") {
		t.Errorf("View() missing stage label 'HASH'\ngot:\n%s", view)
	}
}

// TestProgressModel_ViewDiscoverySpinner verifies that before EventDiscoverDone,
// View() contains "Discovering" or similar discovery indicator.
func TestProgressModel_ViewDiscoverySpinner(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	// Before EventDiscoverDone, discovering is true.
	view := m.View()
	if !strings.Contains(view, "Discovering") {
		t.Errorf("View() missing 'Discovering' before EventDiscoverDone\ngot:\n%s", view)
	}

	// After EventDiscoverDone, "Discovering" should no longer appear.
	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventDiscoverDone, Total: 10})
	view = m.View()
	if strings.Contains(view, "Discovering") {
		t.Errorf("View() still contains 'Discovering' after EventDiscoverDone\ngot:\n%s", view)
	}
}
