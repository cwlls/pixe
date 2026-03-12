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

	pixeprogress "github.com/cwlls/pixe-go/internal/progress"
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

func TestProgressModel_CurrentFile(t *testing.T) {
	m, bus := newTestModel("sort")
	defer bus.Close()

	m = sendEvent(t, m, pixeprogress.Event{Kind: pixeprogress.EventFileStart, RelPath: "IMG_001.jpg"})
	if m.currentFile != "IMG_001.jpg" {
		t.Errorf("currentFile = %q, want %q", m.currentFile, "IMG_001.jpg")
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
