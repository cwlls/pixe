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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
	"github.com/cwlls/pixe-go/internal/progress"
)

// sortState represents the current state of the Sort tab.
type sortState int

const (
	sortStateConfigure sortState = iota
	sortStateRunning
	sortStateComplete
)

// sortEventMsg wraps a progress.Event for the Sort tab.
type sortEventMsg struct {
	event progress.Event
}

// sortBusClosedMsg signals the sort event bus was closed.
type sortBusClosedMsg struct{}

// sortResultMsg carries the final sort result.
type sortResultMsg struct {
	result pipeline.SortResult
	err    error
}

// SortModel is the Bubble Tea model for the Sort tab.
type SortModel struct {
	state    sortState
	config   *config.AppConfig
	registry *discovery.Registry
	hasher   *hash.Hasher
	version  string

	bus      *progress.Bus
	progress styledProgress
	log      activityLog
	workers  workerPane
	overlay  errorOverlay

	// Counters.
	total      int
	completed  int
	copied     int
	duplicates int
	skipped    int
	errors     int

	result *pipeline.SortResult
	filter string
	width  int
	height int
	keymap KeyMap
}

// NewSortModel creates a SortModel from AppOptions.
func NewSortModel(opts AppOptions) SortModel {
	return SortModel{
		state:    sortStateConfigure,
		config:   opts.Config,
		registry: opts.Registry,
		hasher:   opts.Hasher,
		version:  opts.Version,
		progress: newStyledProgress(80),
		log:      newActivityLog(80, 20),
		workers:  newWorkerPane(opts.Config.Workers),
		overlay:  newErrorOverlay(),
		keymap:   DefaultKeyMap(),
	}
}

// Init returns the initial command (none needed for configure state).
func (m SortModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the Sort tab.
func (m SortModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		// Overlay takes priority.
		if m.overlay.Visible() {
			if msg.String() == "esc" {
				m.overlay.Hide()
			}
			return m, nil
		}

		switch m.state {
		case sortStateConfigure:
			if msg.String() == "s" && m.config.Destination != "" {
				return m.startRun()
			}

		case sortStateRunning:
			switch msg.String() {
			case "f":
				m.cycleFilter()
			case "enter":
				// Inspect last error — no-op if no errors.
			case "k", "up":
				var cmd tea.Cmd
				m.log, cmd = m.log.Update(msg)
				return m, cmd
			case "j", "down":
				var cmd tea.Cmd
				m.log, cmd = m.log.Update(msg)
				return m, cmd
			}

		case sortStateComplete:
			switch msg.String() {
			case "n":
				m.state = sortStateConfigure
				m.resetCounters()
			case "e":
				m.log.SetFilter("ERR")
				m.filter = "ERR"
			case "k", "up", "j", "down":
				var cmd tea.Cmd
				m.log, cmd = m.log.Update(msg)
				return m, cmd
			}
		}

	case sortEventMsg:
		m.handleEvent(msg.event)
		if m.state == sortStateRunning {
			return m, listenSortEvents(m.bus)
		}
		return m, nil

	case sortBusClosedMsg:
		if m.state == sortStateRunning {
			m.state = sortStateComplete
		}
		return m, nil

	case sortResultMsg:
		if msg.err == nil {
			m.result = &msg.result
		}
		return m, nil
	}

	return m, nil
}

// startRun creates a bus, builds SortOptions, and launches the pipeline.
func (m SortModel) startRun() (tea.Model, tea.Cmd) {
	bus := progress.NewBus(256)
	m.bus = bus
	m.state = sortStateRunning
	m.resetCounters()
	m.log = newActivityLog(m.width, m.height-10)
	m.workers = newWorkerPane(m.config.Workers)

	opts := pipeline.SortOptions{
		Config:       m.config,
		Hasher:       m.hasher,
		Registry:     m.registry,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       io.Discard,
		PixeVersion:  m.version,
		EventBus:     bus,
		RunID:        uuid.New().String(),
	}

	return m, tea.Batch(
		listenSortEvents(bus),
		runSortPipeline(opts, bus),
	)
}

// runSortPipeline runs the sort pipeline in a goroutine and returns a tea.Cmd.
func runSortPipeline(opts pipeline.SortOptions, bus *progress.Bus) tea.Cmd {
	return func() tea.Msg {
		result, err := pipeline.Run(opts)
		bus.Close()
		return sortResultMsg{result: result, err: err}
	}
}

// listenSortEvents returns a tea.Cmd that reads the next event from the sort bus.
func listenSortEvents(bus *progress.Bus) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-bus.Events()
		if !ok {
			return sortBusClosedMsg{}
		}
		return sortEventMsg{event: e}
	}
}

// handleEvent updates the model based on a sort event.
func (m *SortModel) handleEvent(e progress.Event) {
	switch e.Kind {
	case progress.EventDiscoverDone:
		m.total = e.Total

	case progress.EventFileStart:
		if e.WorkerID >= 0 {
			m.workers.SetWorkerStatus(e.WorkerID, "start", e.RelPath)
		}

	case progress.EventFileExtracted:
		if e.WorkerID >= 0 {
			m.workers.SetWorkerStatus(e.WorkerID, "extract", e.RelPath)
		}

	case progress.EventFileHashed:
		if e.WorkerID >= 0 {
			m.workers.SetWorkerStatus(e.WorkerID, "hash", e.RelPath)
		}

	case progress.EventFileCopied:
		if e.WorkerID >= 0 {
			m.workers.SetWorkerStatus(e.WorkerID, "copy", e.RelPath)
		}

	case progress.EventFileVerified:
		if e.WorkerID >= 0 {
			m.workers.SetWorkerStatus(e.WorkerID, "verify", e.RelPath)
		}

	case progress.EventFileComplete:
		m.completed = e.Completed
		m.copied++
		if e.WorkerID >= 0 {
			m.workers.SetWorkerIdle(e.WorkerID)
		}
		line := logCopyStyle.Render("COPY") + "  " + e.RelPath
		if e.Destination != "" {
			line += " → " + e.Destination
		}
		m.log.Append(line)
		m.updateProgress()

	case progress.EventFileDuplicate:
		m.completed = e.Completed
		m.duplicates++
		if e.WorkerID >= 0 {
			m.workers.SetWorkerIdle(e.WorkerID)
		}
		line := logDupeStyle.Render("DUPE") + "  " + e.RelPath
		if e.MatchesDest != "" {
			line += " → matches " + e.MatchesDest
		}
		m.log.Append(line)
		m.updateProgress()

	case progress.EventFileSkipped:
		m.completed = e.Completed
		m.skipped++
		line := logSkipStyle.Render("SKIP") + "  " + e.RelPath
		if e.Reason != "" {
			line += " (" + e.Reason + ")"
		}
		m.log.Append(line)
		m.updateProgress()

	case progress.EventFileError:
		m.completed = e.Completed
		m.errors++
		if e.WorkerID >= 0 {
			m.workers.SetWorkerIdle(e.WorkerID)
		}
		line := logErrStyle.Render("ERR ") + "  " + e.RelPath
		if e.Reason != "" {
			line += ": " + e.Reason
		}
		m.log.Append(line)
		m.updateProgress()

	case progress.EventRunComplete:
		m.state = sortStateComplete
		if e.Summary != nil {
			m.copied = e.Summary.Processed - e.Summary.Duplicates
			m.duplicates = e.Summary.Duplicates
			m.skipped = e.Summary.Skipped
			m.errors = e.Summary.Errors
		}
		m.progress.SetPercent(1.0)
	}
}

// updateProgress recalculates the progress bar percentage.
func (m *SortModel) updateProgress() {
	if m.total > 0 {
		pct := float64(m.completed) / float64(m.total)
		m.progress.SetPercent(pct)
	}
}

// cycleFilter cycles through filter modes: All → COPY → DUPE → ERR → SKIP → All.
func (m *SortModel) cycleFilter() {
	filters := []string{"", "COPY", "DUPE", "ERR", "SKIP"}
	for i, f := range filters {
		if f == m.filter {
			m.filter = filters[(i+1)%len(filters)]
			m.log.SetFilter(m.filter)
			return
		}
	}
	m.filter = ""
	m.log.SetFilter("")
}

// resetCounters resets all counters for a new run.
func (m *SortModel) resetCounters() {
	m.total = 0
	m.completed = 0
	m.copied = 0
	m.duplicates = 0
	m.skipped = 0
	m.errors = 0
	m.filter = ""
	m.result = nil
}

// View renders the Sort tab.
func (m SortModel) View() string {
	switch m.state {
	case sortStateConfigure:
		return m.viewConfigure()
	case sortStateRunning:
		return m.viewRunning()
	case sortStateComplete:
		return m.viewComplete()
	}
	return ""
}

func (m SortModel) viewConfigure() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("  Sort Configuration"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "  Source:      %s\n", dimStyle.Render(m.config.Source))
	fmt.Fprintf(&sb, "  Destination: %s\n", dimStyle.Render(m.config.Destination))
	fmt.Fprintf(&sb, "  Workers:     %s\n", dimStyle.Render(fmt.Sprintf("%d", m.config.Workers)))
	fmt.Fprintf(&sb, "  Algorithm:   %s\n", dimStyle.Render(m.config.Algorithm))
	fmt.Fprintf(&sb, "  Recursive:   %s\n", dimStyle.Render(fmt.Sprintf("%v", m.config.Recursive)))
	if m.config.Copyright != "" {
		fmt.Fprintf(&sb, "  Copyright:   %s\n", dimStyle.Render(m.config.Copyright))
	}
	if m.config.CameraOwner != "" {
		fmt.Fprintf(&sb, "  Camera Owner:%s\n", dimStyle.Render(m.config.CameraOwner))
	}
	sb.WriteString("\n")
	if m.config.Destination == "" {
		sb.WriteString(logErrStyle.Render("  --dest is required to start a sort run"))
	} else {
		sb.WriteString(dimStyle.Render("  [s] Start Sort   [e] Edit Settings"))
	}
	sb.WriteString("\n")
	return sb.String()
}

func (m SortModel) viewRunning() string {
	var sb strings.Builder
	sb.WriteString("\n")

	// Progress bar.
	sb.WriteString("  ")
	sb.WriteString(m.progress.View())
	sb.WriteString("\n")

	// Counters.
	sb.WriteString("  ")
	fmt.Fprintf(&sb, "%d / %d  ", m.completed, m.total)
	sb.WriteString(logCopyStyle.Render(fmt.Sprintf("copied: %d", m.copied)))
	sb.WriteString("  │  ")
	sb.WriteString(logDupeStyle.Render(fmt.Sprintf("dupes: %d", m.duplicates)))
	sb.WriteString("  │  ")
	sb.WriteString(logSkipStyle.Render(fmt.Sprintf("skipped: %d", m.skipped)))
	sb.WriteString("  │  ")
	sb.WriteString(logErrStyle.Render(fmt.Sprintf("errors: %d", m.errors)))
	sb.WriteString("\n\n")

	// Filter indicator.
	if m.filter != "" {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  filter: %s  [f] cycle", m.filter)))
		sb.WriteString("\n")
	}

	// Activity log.
	sb.WriteString(m.log.View())
	sb.WriteString("\n")

	// Worker pane.
	workerView := m.workers.View(m.width, 6)
	if workerView != "" {
		sb.WriteString(borderStyle.Render(strings.Repeat("─", m.width-4)))
		sb.WriteString("\n")
		sb.WriteString(workerView)
		sb.WriteString("\n")
	}

	// Error overlay on top.
	if m.overlay.Visible() {
		return m.overlay.View(m.width, m.height)
	}

	return sb.String()
}

func (m SortModel) viewComplete() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(headerStyle.Render("  Sort Complete"))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "  Copied:    %s\n", logCopyStyle.Render(fmt.Sprintf("%d", m.copied)))
	fmt.Fprintf(&sb, "  Duplicates:%s\n", logDupeStyle.Render(fmt.Sprintf("%d", m.duplicates)))
	fmt.Fprintf(&sb, "  Skipped:   %s\n", logSkipStyle.Render(fmt.Sprintf("%d", m.skipped)))
	fmt.Fprintf(&sb, "  Errors:    %s\n", logErrStyle.Render(fmt.Sprintf("%d", m.errors)))
	sb.WriteString("\n")
	sb.WriteString(m.log.View())
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  [n] New Run   [e] Filter Errors   [j/k] Scroll"))
	sb.WriteString("\n")
	return sb.String()
}
