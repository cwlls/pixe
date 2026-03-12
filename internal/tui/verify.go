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
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/progress"
	"github.com/cwlls/pixe-go/internal/verify"
)

// verifyState represents the current state of the Verify tab.
type verifyState int

const (
	verifyStateConfigure verifyState = iota
	verifyStateRunning
	verifyStateComplete
)

// verifyEventMsg wraps a progress.Event for the Verify tab.
type verifyEventMsg struct {
	event progress.Event
}

// verifyBusClosedMsg signals the verify event bus was closed.
type verifyBusClosedMsg struct{}

// verifyResultMsg carries the final verify result.
type verifyResultMsg struct {
	result verify.Result
	err    error
}

// VerifyModel is the Bubble Tea model for the Verify tab.
type VerifyModel struct {
	state    verifyState
	dir      string
	registry *discovery.Registry
	hasher   *hash.Hasher

	bus      *progress.Bus
	progress styledProgress
	log      activityLog
	overlay  errorOverlay

	// Counters.
	total        int
	completed    int
	verified     int
	mismatches   int
	unrecognised int

	result *verify.Result
	width  int
	height int
	keymap KeyMap
}

// NewVerifyModel creates a VerifyModel from AppOptions.
func NewVerifyModel(opts AppOptions) VerifyModel {
	dir := ""
	if opts.Config != nil {
		dir = opts.Config.Destination
	}
	return VerifyModel{
		state:    verifyStateConfigure,
		dir:      dir,
		registry: opts.Registry,
		hasher:   opts.Hasher,
		progress: newStyledProgress(80),
		log:      newActivityLog(80, 20),
		overlay:  newErrorOverlay(),
		keymap:   DefaultKeyMap(),
	}
}

// Init returns the initial command (none needed for configure state).
func (m VerifyModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the Verify tab.
func (m VerifyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress = newStyledProgress(m.width - 4)
		logHeight := m.height - 10
		if logHeight < 5 {
			logHeight = 5
		}
		m.log = newActivityLog(m.width, logHeight)
		return m, nil

	case tabActivatedMsg:
		return m, nil

	case tea.KeyMsg:
		if m.overlay.Visible() {
			if msg.String() == "esc" {
				m.overlay.Hide()
			}
			return m, nil
		}

		switch m.state {
		case verifyStateConfigure:
			if msg.String() == "v" && m.dir != "" {
				return m.startRun()
			}

		case verifyStateRunning:
			switch msg.String() {
			case "k", "up", "j", "down":
				var cmd tea.Cmd
				m.log, cmd = m.log.Update(msg)
				return m, cmd
			}

		case verifyStateComplete:
			switch msg.String() {
			case "n":
				m.state = verifyStateConfigure
				m.resetCounters()
			case "e":
				m.log.SetFilter("MISMATCH")
			case "k", "up", "j", "down":
				var cmd tea.Cmd
				m.log, cmd = m.log.Update(msg)
				return m, cmd
			}
		}

	case verifyEventMsg:
		m.handleEvent(msg.event)
		if m.state == verifyStateRunning {
			return m, listenVerifyEvents(m.bus)
		}
		return m, nil

	case verifyBusClosedMsg:
		if m.state == verifyStateRunning {
			m.state = verifyStateComplete
		}
		return m, nil

	case verifyResultMsg:
		if msg.err == nil {
			m.result = &msg.result
		}
		return m, nil
	}

	return m, nil
}

// startRun creates a bus and launches the verify pipeline.
func (m VerifyModel) startRun() (tea.Model, tea.Cmd) {
	bus := progress.NewBus(256)
	m.bus = bus
	m.state = verifyStateRunning
	m.resetCounters()
	m.log = newActivityLog(m.width, m.height-10)

	opts := verify.Options{
		Dir:      m.dir,
		Hasher:   m.hasher,
		Registry: m.registry,
		Output:   io.Discard,
		EventBus: bus,
	}

	return m, tea.Batch(
		listenVerifyEvents(bus),
		runVerifyPipeline(opts, bus),
	)
}

// runVerifyPipeline runs the verify pipeline in a goroutine.
func runVerifyPipeline(opts verify.Options, bus *progress.Bus) tea.Cmd {
	return func() tea.Msg {
		result, err := verify.Run(opts)
		bus.Close()
		return verifyResultMsg{result: result, err: err}
	}
}

// listenVerifyEvents returns a tea.Cmd that reads the next event from the verify bus.
func listenVerifyEvents(bus *progress.Bus) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-bus.Events()
		if !ok {
			return verifyBusClosedMsg{}
		}
		return verifyEventMsg{event: e}
	}
}

// handleEvent updates the model based on a verify event.
func (m *VerifyModel) handleEvent(e progress.Event) {
	switch e.Kind {
	case progress.EventVerifyStart:
		m.total = e.Total

	case progress.EventVerifyOK:
		m.completed = e.Completed
		m.verified++
		line := logCopyStyle.Render("OK  ") + "  " + e.RelPath
		m.log.Append(line)
		m.updateProgress()

	case progress.EventVerifyMismatch:
		m.completed = e.Completed
		m.mismatches++
		line := logErrStyle.Render("MISMATCH") + "  " + e.RelPath
		if e.ExpectedChecksum != "" && e.ActualChecksum != "" {
			line += fmt.Sprintf("\n    expected: %s\n    actual:   %s", e.ExpectedChecksum, e.ActualChecksum)
		} else if e.Reason != "" {
			line += ": " + e.Reason
		}
		m.log.Append(line)
		m.updateProgress()

	case progress.EventVerifyUnrecognised:
		m.completed = e.Completed
		m.unrecognised++
		line := logSkipStyle.Render("UNRECOGNISED") + "  " + e.RelPath
		m.log.Append(line)
		m.updateProgress()

	case progress.EventVerifyDone:
		m.state = verifyStateComplete
		if e.Summary != nil {
			m.verified = e.Summary.Verified
			m.mismatches = e.Summary.Mismatches
			m.unrecognised = e.Summary.Unrecognised
		}
		m.progress.SetPercent(1.0)
	}
}

// updateProgress recalculates the progress bar percentage.
func (m *VerifyModel) updateProgress() {
	if m.total > 0 {
		pct := float64(m.completed) / float64(m.total)
		m.progress.SetPercent(pct)
	}
}

// resetCounters resets all counters for a new run.
func (m *VerifyModel) resetCounters() {
	m.total = 0
	m.completed = 0
	m.verified = 0
	m.mismatches = 0
	m.unrecognised = 0
	m.result = nil
}

// View renders the Verify tab.
func (m VerifyModel) View() string {
	switch m.state {
	case verifyStateConfigure:
		return m.viewConfigure()
	case verifyStateRunning:
		return m.viewRunning()
	case verifyStateComplete:
		return m.viewComplete()
	}
	return ""
}

func (m VerifyModel) viewConfigure() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("  Verify Configuration"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "  Directory: %s\n", dimStyle.Render(m.dir))
	sb.WriteString("\n")
	if m.dir == "" {
		sb.WriteString(logErrStyle.Render("  --dest is required to start a verify run"))
	} else {
		sb.WriteString(dimStyle.Render("  [v] Start Verify"))
	}
	sb.WriteString("\n")
	return sb.String()
}

func (m VerifyModel) viewRunning() string {
	var sb strings.Builder
	sb.WriteString("\n")

	sb.WriteString("  ")
	sb.WriteString(m.progress.View())
	sb.WriteString("\n")

	sb.WriteString("  ")
	fmt.Fprintf(&sb, "%d / %d  ", m.completed, m.total)
	sb.WriteString(logCopyStyle.Render(fmt.Sprintf("verified: %d", m.verified)))
	sb.WriteString("  │  ")
	sb.WriteString(logErrStyle.Render(fmt.Sprintf("mismatches: %d", m.mismatches)))
	sb.WriteString("  │  ")
	sb.WriteString(logSkipStyle.Render(fmt.Sprintf("unrecognised: %d", m.unrecognised)))
	sb.WriteString("\n\n")

	sb.WriteString(m.log.View())
	sb.WriteString("\n")

	if m.overlay.Visible() {
		return m.overlay.View(m.width, m.height)
	}

	return sb.String()
}

func (m VerifyModel) viewComplete() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("  Verify Complete"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "  Verified:     %s\n", logCopyStyle.Render(fmt.Sprintf("%d", m.verified)))
	fmt.Fprintf(&sb, "  Mismatches:   %s\n", logErrStyle.Render(fmt.Sprintf("%d", m.mismatches)))
	fmt.Fprintf(&sb, "  Unrecognised: %s\n", logSkipStyle.Render(fmt.Sprintf("%d", m.unrecognised)))
	sb.WriteString("\n")
	sb.WriteString(m.log.View())
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  [n] New Verify   [e] Filter Mismatches   [j/k] Scroll"))
	sb.WriteString("\n")
	return sb.String()
}
