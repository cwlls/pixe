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

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/progress"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func testAppOptions() AppOptions {
	h, _ := hash.NewHasher("sha1")
	return AppOptions{
		Config: &config.AppConfig{
			Source:      "/tmp/src",
			Destination: "/tmp/dst",
			Workers:     2,
			Algorithm:   "sha1",
		},
		Registry: discovery.NewRegistry(),
		Hasher:   h,
		Version:  "test",
	}
}

func pressKey(t *testing.T, m tea.Model, key string) (tea.Model, tea.Cmd) {
	t.Helper()
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func pressSpecialKey(t *testing.T, m tea.Model, keyType tea.KeyType) (tea.Model, tea.Cmd) {
	t.Helper()
	return m.Update(tea.KeyMsg{Type: keyType})
}

// ---------------------------------------------------------------------------
// App tests
// ---------------------------------------------------------------------------

func TestApp_TabSwitching(t *testing.T) {
	app := NewApp(testAppOptions())

	if app.activeTab != tabSort {
		t.Fatalf("initial activeTab = %d, want %d (tabSort)", app.activeTab, tabSort)
	}

	// Tab key cycles forward.
	m, _ := pressSpecialKey(t, app, tea.KeyTab)
	app = m.(App)
	if app.activeTab != tabVerify {
		t.Errorf("after Tab: activeTab = %d, want %d (tabVerify)", app.activeTab, tabVerify)
	}

	m, _ = pressSpecialKey(t, app, tea.KeyTab)
	app = m.(App)
	if app.activeTab != tabStatus {
		t.Errorf("after Tab: activeTab = %d, want %d (tabStatus)", app.activeTab, tabStatus)
	}

	m, _ = pressSpecialKey(t, app, tea.KeyTab)
	app = m.(App)
	if app.activeTab != tabSort {
		t.Errorf("after Tab wrap: activeTab = %d, want %d (tabSort)", app.activeTab, tabSort)
	}

	// ShiftTab cycles backward.
	m, _ = pressSpecialKey(t, app, tea.KeyShiftTab)
	app = m.(App)
	if app.activeTab != tabStatus {
		t.Errorf("after ShiftTab: activeTab = %d, want %d (tabStatus)", app.activeTab, tabStatus)
	}

	// Direct jump with number keys.
	m, _ = pressKey(t, app, "1")
	app = m.(App)
	if app.activeTab != tabSort {
		t.Errorf("after '1': activeTab = %d, want %d (tabSort)", app.activeTab, tabSort)
	}

	m, _ = pressKey(t, app, "2")
	app = m.(App)
	if app.activeTab != tabVerify {
		t.Errorf("after '2': activeTab = %d, want %d (tabVerify)", app.activeTab, tabVerify)
	}

	m, _ = pressKey(t, app, "3")
	app = m.(App)
	if app.activeTab != tabStatus {
		t.Errorf("after '3': activeTab = %d, want %d (tabStatus)", app.activeTab, tabStatus)
	}
}

func TestApp_WindowSize(t *testing.T) {
	app := NewApp(testAppOptions())

	m, _ := app.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	app = m.(App)

	if app.width != 200 {
		t.Errorf("width = %d, want 200", app.width)
	}
	if app.height != 50 {
		t.Errorf("height = %d, want 50", app.height)
	}
}

func TestApp_QuitKeys(t *testing.T) {
	app := NewApp(testAppOptions())

	_, cmd := pressKey(t, app, "q")
	if cmd == nil {
		t.Error("'q' key should return tea.Quit cmd, got nil")
	}

	_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c should return tea.Quit cmd, got nil")
	}
}

// ---------------------------------------------------------------------------
// SortModel tests
// ---------------------------------------------------------------------------

func TestSortModel_StateTransitions(t *testing.T) {
	opts := testAppOptions()
	m := NewSortModel(opts)

	if m.state != sortStateConfigure {
		t.Fatalf("initial state = %v, want sortStateConfigure", m.state)
	}

	// Simulate EventRunComplete to transition to complete.
	m.state = sortStateRunning
	m.handleEvent(progress.Event{
		Kind:    progress.EventRunComplete,
		Summary: &progress.RunSummary{Processed: 3},
	})
	if m.state != sortStateComplete {
		t.Errorf("after EventRunComplete: state = %v, want sortStateComplete", m.state)
	}

	// 'n' key resets to configure.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(SortModel)
	if m.state != sortStateConfigure {
		t.Errorf("after 'n': state = %v, want sortStateConfigure", m.state)
	}
}

func TestSortModel_EventCounters(t *testing.T) {
	opts := testAppOptions()
	m := NewSortModel(opts)
	m.state = sortStateRunning

	m.handleEvent(progress.Event{Kind: progress.EventDiscoverDone, Total: 10})
	m.handleEvent(progress.Event{Kind: progress.EventFileComplete, Completed: 1})
	m.handleEvent(progress.Event{Kind: progress.EventFileComplete, Completed: 2})
	m.handleEvent(progress.Event{Kind: progress.EventFileDuplicate, Completed: 3})
	m.handleEvent(progress.Event{Kind: progress.EventFileSkipped, Completed: 4})
	m.handleEvent(progress.Event{Kind: progress.EventFileError, Completed: 5})

	if m.total != 10 {
		t.Errorf("total = %d, want 10", m.total)
	}
	if m.copied != 2 {
		t.Errorf("copied = %d, want 2", m.copied)
	}
	if m.duplicates != 1 {
		t.Errorf("duplicates = %d, want 1", m.duplicates)
	}
	if m.skipped != 1 {
		t.Errorf("skipped = %d, want 1", m.skipped)
	}
	if m.errors != 1 {
		t.Errorf("errors = %d, want 1", m.errors)
	}
}

func TestSortModel_ActivityLogAppend(t *testing.T) {
	opts := testAppOptions()
	m := NewSortModel(opts)
	m.state = sortStateRunning
	m.log = newActivityLog(80, 20)

	m.handleEvent(progress.Event{
		Kind:        progress.EventFileComplete,
		RelPath:     "photo.jpg",
		Destination: "2026/01-Jan/photo.jpg",
		Completed:   1,
	})
	m.handleEvent(progress.Event{
		Kind:      progress.EventFileError,
		RelPath:   "bad.jpg",
		Reason:    "hash failed",
		Completed: 2,
	})

	content := m.log.filteredContent()
	if !strings.Contains(content, "photo.jpg") {
		t.Errorf("activity log missing 'photo.jpg'\ngot:\n%s", content)
	}
	if !strings.Contains(content, "bad.jpg") {
		t.Errorf("activity log missing 'bad.jpg'\ngot:\n%s", content)
	}
}

func TestSortModel_FilterCycle(t *testing.T) {
	opts := testAppOptions()
	m := NewSortModel(opts)
	m.state = sortStateRunning

	// Initial filter is "".
	if m.filter != "" {
		t.Fatalf("initial filter = %q, want empty", m.filter)
	}

	// Cycle through filters.
	expected := []string{"COPY", "DUPE", "ERR", "SKIP", ""}
	for _, want := range expected {
		m.cycleFilter()
		if m.filter != want {
			t.Errorf("after cycleFilter: filter = %q, want %q", m.filter, want)
		}
	}
}

// ---------------------------------------------------------------------------
// VerifyModel tests
// ---------------------------------------------------------------------------

func TestVerifyModel_StateTransitions(t *testing.T) {
	opts := testAppOptions()
	m := NewVerifyModel(opts)

	if m.state != verifyStateConfigure {
		t.Fatalf("initial state = %v, want verifyStateConfigure", m.state)
	}

	// Simulate EventVerifyDone to transition to complete.
	m.state = verifyStateRunning
	m.handleEvent(progress.Event{
		Kind:    progress.EventVerifyDone,
		Summary: &progress.RunSummary{Verified: 5},
	})
	if m.state != verifyStateComplete {
		t.Errorf("after EventVerifyDone: state = %v, want verifyStateComplete", m.state)
	}

	// 'n' key resets to configure.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(VerifyModel)
	if m.state != verifyStateConfigure {
		t.Errorf("after 'n': state = %v, want verifyStateConfigure", m.state)
	}
}

// ---------------------------------------------------------------------------
// StatusModel tests
// ---------------------------------------------------------------------------

func TestStatusModel_CategorySwitching(t *testing.T) {
	opts := testAppOptions()
	m := NewStatusModel(opts)

	// Inject pre-built categories to avoid filesystem walk.
	m.state = statusTabReady
	m.categories = []categoryData{
		{name: "Sorted", files: []statusFileEntry{{relPath: "a.jpg"}}},
		{name: "Duplicates", files: []statusFileEntry{{relPath: "b.jpg"}}},
		{name: "Errored", files: nil},
		{name: "Unsorted", files: []statusFileEntry{{relPath: "c.jpg"}}},
		{name: "Unrecognised", files: nil},
	}
	m.refreshViewport()

	if m.activeCategory != 0 {
		t.Fatalf("initial activeCategory = %d, want 0", m.activeCategory)
	}

	// Switch to category 2 (Duplicates, index 1).
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = updated.(StatusModel)
	if m.activeCategory != 1 {
		t.Errorf("after '2': activeCategory = %d, want 1", m.activeCategory)
	}

	// Switch to category 4 (Unsorted, index 3).
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m = updated.(StatusModel)
	if m.activeCategory != 3 {
		t.Errorf("after '4': activeCategory = %d, want 3", m.activeCategory)
	}
}

func TestStatusModel_Refresh(t *testing.T) {
	opts := testAppOptions()
	m := NewStatusModel(opts)
	m.state = statusTabReady

	// 'r' key should transition to loading and return a cmd.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(StatusModel)

	if m.state != statusTabLoading {
		t.Errorf("after 'r': state = %v, want statusTabLoading", m.state)
	}
	if cmd == nil {
		t.Error("'r' key should return a load cmd, got nil")
	}
}
