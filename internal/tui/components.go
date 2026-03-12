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
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// styledProgress — wraps bubbles/progress.Model with TUI adaptive styling.
// ---------------------------------------------------------------------------

// styledProgress wraps a Bubbles progress bar with the TUI's adaptive styling.
type styledProgress struct {
	bar   progress.Model
	width int
}

// newStyledProgress creates a styledProgress with the given width.
func newStyledProgress(width int) styledProgress {
	bar := progress.New(progress.WithDefaultGradient())
	bar.Width = width
	return styledProgress{bar: bar, width: width}
}

// SetPercent updates the progress bar percentage (0.0–1.0).
func (sp *styledProgress) SetPercent(pct float64) {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	_ = sp.bar.SetPercent(pct)
}

// View renders the progress bar.
func (sp styledProgress) View() string {
	return progressBarStyle.Render(sp.bar.View())
}

// ---------------------------------------------------------------------------
// activityLog — scrollable activity log backed by bubbles/viewport.
// ---------------------------------------------------------------------------

// activityLog is a scrollable log of activity lines backed by a viewport.
type activityLog struct {
	vp     viewport.Model
	lines  []string
	filter string
	follow bool
}

// newActivityLog creates an activityLog with the given dimensions.
func newActivityLog(width, height int) activityLog {
	vp := viewport.New(width, height)
	return activityLog{vp: vp, follow: true}
}

// Append adds a line to the log. If follow mode is on, auto-scrolls to bottom.
func (al *activityLog) Append(line string) {
	al.lines = append(al.lines, line)
	al.vp.SetContent(al.filteredContent())
	if al.follow {
		al.vp.GotoBottom()
	}
}

// SetFilter sets the verb filter (e.g. "COPY", "DUPE", "ERR", "SKIP", or "" for all).
func (al *activityLog) SetFilter(filter string) {
	al.filter = filter
	al.vp.SetContent(al.filteredContent())
}

// ToggleFollow toggles auto-scroll to bottom.
func (al *activityLog) ToggleFollow() {
	al.follow = !al.follow
}

// filteredContent returns the log content with the current filter applied.
func (al *activityLog) filteredContent() string {
	if al.filter == "" {
		return strings.Join(al.lines, "\n")
	}
	var filtered []string
	for _, line := range al.lines {
		if strings.Contains(line, al.filter) {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

// Update delegates to the viewport model.
func (al activityLog) Update(msg tea.Msg) (activityLog, tea.Cmd) {
	var cmd tea.Cmd
	al.vp, cmd = al.vp.Update(msg)
	return al, cmd
}

// View renders the activity log.
func (al activityLog) View() string {
	return al.vp.View()
}

// ---------------------------------------------------------------------------
// workerPane — displays per-worker status.
// ---------------------------------------------------------------------------

type workerStatus struct {
	stage    string
	filename string
	idle     bool
}

// workerPane displays the status of each concurrent worker.
type workerPane struct {
	workers []workerStatus
}

// newWorkerPane creates a workerPane for the given number of workers.
func newWorkerPane(numWorkers int) workerPane {
	workers := make([]workerStatus, numWorkers)
	for i := range workers {
		workers[i] = workerStatus{idle: true}
	}
	return workerPane{workers: workers}
}

// SetWorkerStatus updates the status of a worker.
func (wp *workerPane) SetWorkerStatus(workerID int, stage, filename string) {
	if workerID < 0 || workerID >= len(wp.workers) {
		return
	}
	wp.workers[workerID] = workerStatus{stage: stage, filename: filename, idle: false}
}

// SetWorkerIdle marks a worker as idle.
func (wp *workerPane) SetWorkerIdle(workerID int) {
	if workerID < 0 || workerID >= len(wp.workers) {
		return
	}
	wp.workers[workerID] = workerStatus{idle: true}
}

// View renders the worker pane. If maxHeight is too small, renders a summary.
func (wp workerPane) View(width, maxHeight int) string {
	if len(wp.workers) == 0 {
		return ""
	}

	lines := make([]string, 0, len(wp.workers))
	for i, w := range wp.workers {
		var line string
		if w.idle {
			line = workerIdleStyle.Render(fmt.Sprintf("  worker %d  idle", i))
		} else {
			filename := w.filename
			maxFilename := width - 20
			if maxFilename < 10 {
				maxFilename = 10
			}
			if len(filename) > maxFilename {
				filename = "..." + filename[len(filename)-maxFilename+3:]
			}
			line = workerActiveStyle.Render(fmt.Sprintf("  worker %d  %s  %s", i, w.stage, filename))
		}
		lines = append(lines, line)
	}

	if len(lines) > maxHeight {
		// Collapse to summary.
		active := 0
		for _, w := range wp.workers {
			if !w.idle {
				active++
			}
		}
		return dimStyle.Render(fmt.Sprintf("  %d workers (%d active)", len(wp.workers), active))
	}

	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// errorOverlay — modal overlay for error details.
// ---------------------------------------------------------------------------

// errorOverlay is a modal overlay that shows error details.
type errorOverlay struct {
	visible bool
	relPath string
	stage   string
	errMsg  string
}

// newErrorOverlay creates a hidden error overlay.
func newErrorOverlay() errorOverlay {
	return errorOverlay{}
}

// Show populates and makes the overlay visible.
func (eo *errorOverlay) Show(relPath, stage, errMsg string) {
	eo.relPath = relPath
	eo.stage = stage
	eo.errMsg = errMsg
	eo.visible = true
}

// Hide hides the overlay.
func (eo *errorOverlay) Hide() {
	eo.visible = false
}

// Visible returns whether the overlay is currently shown.
func (eo errorOverlay) Visible() bool {
	return eo.visible
}

// View renders the overlay centered in the given dimensions.
func (eo errorOverlay) View(width, height int) string {
	if !eo.visible {
		return ""
	}

	content := fmt.Sprintf("File:  %s\nStage: %s\nError: %s", eo.relPath, eo.stage, eo.errMsg)
	box := overlayStyle.Width(width / 2).Render(content)

	// Center the box.
	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)
	leftPad := (width - boxWidth) / 2
	topPad := (height - boxHeight) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	if topPad < 0 {
		topPad = 0
	}

	var sb strings.Builder
	for i := 0; i < topPad; i++ {
		sb.WriteString("\n")
	}
	for _, line := range strings.Split(box, "\n") {
		sb.WriteString(strings.Repeat(" ", leftPad))
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}
