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
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	pixeprogress "github.com/cwlls/pixe/internal/progress"
)

// eventMsg wraps a progress.Event for delivery to the Bubble Tea model.
type eventMsg struct {
	event pixeprogress.Event
}

// busClosedMsg signals that the event bus has been closed.
type busClosedMsg struct{}

// tickMsg triggers a re-render for ETA updates.
type tickMsg struct{}

// listenForEvents returns a tea.Cmd that reads the next event from the bus
// and delivers it as an eventMsg. When the channel is closed it delivers
// busClosedMsg.
func listenForEvents(bus *pixeprogress.Bus) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-bus.Events()
		if !ok {
			return busClosedMsg{}
		}
		return eventMsg{event: e}
	}
}

// tickCmd returns a tea.Cmd that fires a tickMsg after one second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// ProgressModel is a Bubble Tea model that renders a live progress bar for
// `pixe sort --progress` and `pixe verify --progress`.
type ProgressModel struct {
	bus    *pixeprogress.Bus
	bar    progress.Model
	mode   string // "sort" or "verify"
	source string
	dest   string

	// Counters.
	total      int
	completed  int
	copied     int
	duplicates int
	skipped    int
	errors     int
	verified   int
	mismatches int

	currentFile string
	startedAt   time.Time
	width       int
	done        bool
}

// NewProgressModel creates a ProgressModel for the given bus, source/dest
// paths, and mode ("sort" or "verify").
func NewProgressModel(bus *pixeprogress.Bus, source, dest, mode string) ProgressModel {
	bar := progress.New(progress.WithDefaultGradient())
	return ProgressModel{
		bus:       bus,
		bar:       bar,
		mode:      mode,
		source:    source,
		dest:      dest,
		startedAt: time.Now(),
		width:     80,
	}
}

// Init returns the initial command: start listening for events and a tick.
func (m ProgressModel) Init() tea.Cmd {
	return tea.Batch(listenForEvents(m.bus), tickCmd())
}

// Update handles incoming messages and updates the model state.
func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.bar.Width = msg.Width - 20
		if m.bar.Width < 10 {
			m.bar.Width = 10
		}
		return m, nil

	case tickMsg:
		if m.done {
			return m, nil
		}
		return m, tickCmd()

	case busClosedMsg:
		m.done = true
		return m, tea.Quit

	case eventMsg:
		m.handleEvent(msg.event)
		if m.done {
			return m, tea.Quit
		}
		return m, listenForEvents(m.bus)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

// handleEvent updates model counters based on the event kind.
func (m *ProgressModel) handleEvent(e pixeprogress.Event) {
	switch e.Kind {
	case pixeprogress.EventDiscoverDone, pixeprogress.EventVerifyStart:
		m.total = e.Total

	case pixeprogress.EventFileStart:
		m.currentFile = e.RelPath

	case pixeprogress.EventFileComplete:
		m.completed = e.Completed
		m.copied++

	case pixeprogress.EventFileDuplicate:
		m.completed = e.Completed
		m.duplicates++

	case pixeprogress.EventFileSkipped:
		m.completed = e.Completed
		m.skipped++

	case pixeprogress.EventFileError:
		m.completed = e.Completed
		m.errors++

	case pixeprogress.EventVerifyOK:
		m.completed = e.Completed
		m.verified++

	case pixeprogress.EventVerifyMismatch:
		m.completed = e.Completed
		m.mismatches++

	case pixeprogress.EventVerifyUnrecognised:
		m.completed = e.Completed
		m.skipped++

	case pixeprogress.EventRunComplete, pixeprogress.EventVerifyDone:
		m.done = true
		if e.Summary != nil {
			if m.mode == "sort" {
				m.copied = e.Summary.Processed - e.Summary.Duplicates
				m.duplicates = e.Summary.Duplicates
				m.skipped = e.Summary.Skipped
				m.errors = e.Summary.Errors
			} else {
				m.verified = e.Summary.Verified
				m.mismatches = e.Summary.Mismatches
				m.skipped = e.Summary.Unrecognised
			}
		}
	}
}

// View renders the progress bar UI.
func (m ProgressModel) View() string {
	var sb strings.Builder

	// Line 1: header.
	header := fmt.Sprintf("pixe %s  %s → %s", m.mode, m.source, m.dest)
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n\n")

	// Line 3: progress bar with count and ETA.
	pct := 0.0
	if m.total > 0 {
		pct = float64(m.completed) / float64(m.total)
		if pct > 1.0 {
			pct = 1.0
		}
	}

	barWidth := m.width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	m.bar.Width = barWidth

	eta := m.etaString()
	countStr := fmt.Sprintf("%d / %d", m.completed, m.total)
	pctStr := fmt.Sprintf("(%.0f%%)", pct*100)

	sb.WriteString(m.bar.ViewAs(pct))
	sb.WriteString("  ")
	sb.WriteString(counterStyle.Render(countStr))
	sb.WriteString("  ")
	sb.WriteString(counterStyle.Render(pctStr))
	if eta != "" {
		sb.WriteString("  ")
		sb.WriteString(etaStyle.Render("ETA " + eta))
	}
	sb.WriteString("\n")

	// Line 4: current file (sort mode only).
	if m.mode == "sort" && m.currentFile != "" {
		line := "Current  " + m.currentFile
		if len(line) > m.width {
			line = line[:m.width-3] + "..."
		}
		sb.WriteString(currentFileStyle.Render(line))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Line 6: status counters.
	if m.mode == "sort" {
		sb.WriteString(labelStyle.Render("copied: "))
		sb.WriteString(counterStyle.Render(fmt.Sprintf("%d", m.copied)))
		sb.WriteString("  │  ")
		sb.WriteString(labelStyle.Render("dupes: "))
		sb.WriteString(counterStyle.Render(fmt.Sprintf("%d", m.duplicates)))
		sb.WriteString("  │  ")
		sb.WriteString(labelStyle.Render("skipped: "))
		sb.WriteString(counterStyle.Render(fmt.Sprintf("%d", m.skipped)))
		sb.WriteString("  │  ")
		sb.WriteString(labelStyle.Render("errors: "))
		sb.WriteString(errorCountStyle.Render(fmt.Sprintf("%d", m.errors)))
	} else {
		sb.WriteString(labelStyle.Render("verified: "))
		sb.WriteString(counterStyle.Render(fmt.Sprintf("%d", m.verified)))
		sb.WriteString("  │  ")
		sb.WriteString(labelStyle.Render("mismatches: "))
		sb.WriteString(errorCountStyle.Render(fmt.Sprintf("%d", m.mismatches)))
		sb.WriteString("  │  ")
		sb.WriteString(labelStyle.Render("unrecognised: "))
		sb.WriteString(counterStyle.Render(fmt.Sprintf("%d", m.skipped)))
	}
	sb.WriteString("\n")

	return sb.String()
}

// etaString returns a human-readable ETA string, or "" if not calculable.
func (m ProgressModel) etaString() string {
	if m.total <= 0 || m.completed <= 0 {
		return ""
	}
	elapsed := time.Since(m.startedAt)
	rate := float64(elapsed) / float64(m.completed)
	remaining := time.Duration(rate * float64(m.total-m.completed))
	if remaining < time.Second {
		return "< 1s"
	}
	mins := int(remaining.Minutes())
	secs := int(remaining.Seconds()) % 60
	if mins > 0 {
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	return fmt.Sprintf("%ds", secs)
}
