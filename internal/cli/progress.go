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
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	pixeprogress "github.com/cwlls/pixe/internal/progress"
)

// spinnerFrames are the braille spinner characters used during the discovery phase.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// tickInterval is the Bubble Tea tick interval for ETA and spinner updates.
const tickInterval = 150 * time.Millisecond

// eventMsg wraps a progress.Event for delivery to the Bubble Tea model.
type eventMsg struct {
	event pixeprogress.Event
}

// busClosedMsg signals that the event bus has been closed.
type busClosedMsg struct{}

// tickMsg triggers a re-render for ETA and spinner updates.
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

// tickCmd returns a tea.Cmd that fires a tickMsg after tickInterval.
func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// WorkerState tracks the current processing state of a single worker.
type WorkerState struct {
	// WorkerID is the unique identifier for this worker (1-indexed for sort/verify workers).
	WorkerID int
	// RelPath is the basename of the file being processed.
	RelPath string
	// Stage is the current pipeline stage label: "HASH", "COPY", "VERIFY", or "TAG".
	Stage string
	// FileSize is the total file size in bytes.
	FileSize int64
	// BytesWritten is the number of bytes processed in the current I/O stage.
	BytesWritten int64
	// StageStart is the time when the current stage began, used to compute per-file ETA.
	StageStart time.Time
}

// ProgressModel is a Bubble Tea model that renders a live multi-line progress
// display for `pixe sort --progress` and `pixe verify --progress`.
//
// The model tracks per-worker state in a map keyed by WorkerID, aggregate
// counters (copied, duplicates, skipped, errors for sort; verified, mismatches,
// unrecognised for verify), and discovery phase state. The View() method renders
// a header line, overall progress bar (or discovery spinner), per-worker status
// lines, and status counters.
type ProgressModel struct {
	bus  *pixeprogress.Bus
	bar  progress.Model
	mode string // "sort" or "verify"

	source string
	dest   string

	// Aggregate counters.
	total      int
	completed  int
	copied     int
	duplicates int
	skipped    int
	errors     int
	verified   int
	mismatches int

	// Per-worker state (keyed by WorkerID).
	workers map[int]*WorkerState

	// Discovery phase: true until EventDiscoverDone / EventVerifyStart.
	discovering bool
	spinner     int // animation frame index

	startedAt time.Time
	width     int
	done      bool
}

// NewProgressModel creates a ProgressModel for the given bus, source/dest
// paths, and mode ("sort" or "verify").
func NewProgressModel(bus *pixeprogress.Bus, source, dest, mode string) ProgressModel {
	bar := progress.New(progress.WithDefaultGradient())
	return ProgressModel{
		bus:         bus,
		bar:         bar,
		mode:        mode,
		source:      source,
		dest:        dest,
		workers:     make(map[int]*WorkerState),
		discovering: true,
		startedAt:   time.Now(),
		width:       80,
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
		m.spinner = (m.spinner + 1) % len(spinnerFrames)
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

// handleEvent updates model state based on the event kind.
func (m *ProgressModel) handleEvent(e pixeprogress.Event) {
	switch e.Kind {
	case pixeprogress.EventDiscoverDone, pixeprogress.EventVerifyStart:
		m.total = e.Total
		m.discovering = false

	case pixeprogress.EventFileStart, pixeprogress.EventVerifyFileStart:
		m.workers[e.WorkerID] = &WorkerState{
			WorkerID:   e.WorkerID,
			RelPath:    filepath.Base(e.RelPath),
			Stage:      "HASH",
			FileSize:   e.FileSize,
			StageStart: time.Now(),
		}

	case pixeprogress.EventByteProgress:
		if ws, ok := m.workers[e.WorkerID]; ok {
			ws.BytesWritten = e.BytesWritten
			if e.Stage != "" {
				ws.Stage = e.Stage
			}
		}

	case pixeprogress.EventFileHashed:
		if ws, ok := m.workers[e.WorkerID]; ok {
			ws.Stage = "COPY"
			ws.BytesWritten = 0
			ws.StageStart = time.Now()
		}

	case pixeprogress.EventFileCopied:
		if ws, ok := m.workers[e.WorkerID]; ok {
			ws.Stage = "VERIFY"
			ws.BytesWritten = 0
			ws.StageStart = time.Now()
		}

	case pixeprogress.EventFileVerified:
		if ws, ok := m.workers[e.WorkerID]; ok {
			ws.Stage = "TAG"
			ws.BytesWritten = 0
			ws.StageStart = time.Now()
		}

	case pixeprogress.EventFileComplete:
		m.completed = e.Completed
		m.copied++
		delete(m.workers, e.WorkerID)

	case pixeprogress.EventFileDuplicate:
		m.completed = e.Completed
		m.duplicates++
		delete(m.workers, e.WorkerID)

	case pixeprogress.EventFileSkipped:
		m.completed = e.Completed
		m.skipped++
		delete(m.workers, e.WorkerID)

	case pixeprogress.EventFileError:
		m.completed = e.Completed
		m.errors++
		delete(m.workers, e.WorkerID)

	case pixeprogress.EventVerifyOK:
		m.completed = e.Completed
		m.verified++
		delete(m.workers, e.WorkerID)

	case pixeprogress.EventVerifyMismatch:
		m.completed = e.Completed
		m.mismatches++
		delete(m.workers, e.WorkerID)

	case pixeprogress.EventVerifyUnrecognised:
		m.completed = e.Completed
		m.skipped++
		delete(m.workers, e.WorkerID)

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

// View renders the multi-line progress display.
func (m ProgressModel) View() string {
	var sb strings.Builder

	// Line 1: header.
	header := fmt.Sprintf("pixe %s  %s → %s", m.mode, m.source, m.dest)
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n\n")

	// Line 2: overall progress bar or discovery spinner.
	if m.discovering {
		frame := spinnerFrames[m.spinner%len(spinnerFrames)]
		sb.WriteString(discoveryStyle.Render(frame + "  Discovering files..."))
		sb.WriteString("\n")
	} else {
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
	}

	// Worker lines — one per active worker, sorted by WorkerID for stability.
	if len(m.workers) > 0 {
		sb.WriteString("\n")

		// Collect and sort worker IDs for stable rendering.
		ids := make([]int, 0, len(m.workers))
		for id := range m.workers {
			ids = append(ids, id)
		}
		sort.Ints(ids)

		for _, id := range ids {
			ws := m.workers[id]
			sb.WriteString(m.renderWorkerLine(ws))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	// Status counters.
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

// renderWorkerLine renders a single worker status line with stage label,
// filename, per-file progress bar, percentage, file size, and ETA estimate.
//
// Example output:
//
//	HASH    filename.jpg  ████████░░  78%   12.4 MB   ~2s
func (m ProgressModel) renderWorkerLine(ws *WorkerState) string {
	const stageWidth = 8
	const barWidth = 10
	const filenameWidth = 24

	// Stage label (fixed width).
	stageLabel := fmt.Sprintf("%-*s", stageWidth, ws.Stage)

	// Filename (truncated).
	name := ws.RelPath
	if len(name) > filenameWidth {
		name = name[:filenameWidth-3] + "..."
	}
	namePadded := fmt.Sprintf("%-*s", filenameWidth, name)

	// Per-file progress bar and percentage.
	var barStr, pctStr string
	hasProgress := ws.Stage == "HASH" || ws.Stage == "COPY" || ws.Stage == "VERIFY"
	if hasProgress && ws.FileSize > 0 {
		pct := float64(ws.BytesWritten) / float64(ws.FileSize)
		if pct > 1.0 {
			pct = 1.0
		}
		barStr = miniBar(pct, barWidth)
		pctStr = fmt.Sprintf("%3.0f%%", pct*100)
	} else {
		barStr = strings.Repeat("░", barWidth)
		pctStr = "    "
	}

	// File size.
	sizeStr := humanSize(ws.FileSize)

	// Per-file ETA.
	etaStr := workerETA(ws)

	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(stageStyle.Render(stageLabel))
	sb.WriteString("  ")
	sb.WriteString(workerFileStyle.Render(namePadded))
	sb.WriteString("  ")
	sb.WriteString(barStr)
	sb.WriteString("  ")
	sb.WriteString(counterStyle.Render(pctStr))
	sb.WriteString("  ")
	sb.WriteString(fileSizeStyle.Render(fmt.Sprintf("%8s", sizeStr)))
	if etaStr != "" {
		sb.WriteString("  ")
		sb.WriteString(workerETAStyle.Render(etaStr))
	}
	return sb.String()
}

// miniBar renders a small progress bar using Unicode block characters (█ for filled, ░ for empty).
// width is the total number of characters. pct is clamped to [0, 1].
func miniBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

// humanSize returns a human-readable file size string (e.g., "12.4 MB", "856 KB").
func humanSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// workerETA returns a per-file ETA string based on byte throughput in the current stage.
// Returns "" if the ETA cannot be calculated (e.g., file size unknown or no progress yet).
// Format: "~2s", "~1m30s", etc.
func workerETA(ws *WorkerState) string {
	if ws.FileSize <= 0 || ws.BytesWritten <= 0 {
		return ""
	}
	elapsed := time.Since(ws.StageStart)
	if elapsed < time.Millisecond {
		return ""
	}
	rate := float64(ws.BytesWritten) / elapsed.Seconds() // bytes/sec
	remaining := float64(ws.FileSize-ws.BytesWritten) / rate
	if remaining < 0 {
		remaining = 0
	}
	d := time.Duration(remaining * float64(time.Second))
	if d < time.Second {
		return "~0s"
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	if mins > 0 {
		return fmt.Sprintf("~%dm%ds", mins, secs)
	}
	return fmt.Sprintf("~%ds", secs)
}

// etaString returns a human-readable overall ETA string based on file-count throughput.
// Returns "" if the ETA cannot be calculated (e.g., no files completed yet).
// Format: "< 1s", "1m 30s", "45s", etc.
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
