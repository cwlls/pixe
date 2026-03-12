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
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/ignore"
	"github.com/cwlls/pixe-go/internal/manifest"
)

// statusTabState represents the loading/ready state of the Status tab.
type statusTabState int

const (
	statusTabLoading statusTabState = iota
	statusTabReady
)

// statusFileEntry holds the classification result for a single file.
type statusFileEntry struct {
	relPath string
	detail  string // destination, match, reason, or empty
}

// categoryData holds a named category with its file list.
type categoryData struct {
	name  string
	files []statusFileEntry
}

// statusResultMsg carries the result of a background status walk.
type statusResultMsg struct {
	categories []categoryData
	ledgerInfo string
	err        error
}

// StatusModel is the Bubble Tea model for the Status tab.
type StatusModel struct {
	state          statusTabState
	source         string
	config         *config.AppConfig
	registry       *discovery.Registry
	categories     []categoryData
	activeCategory int
	viewport       viewport.Model
	ledgerInfo     string
	width          int
	height         int
	keymap         KeyMap
}

// NewStatusModel creates a StatusModel from AppOptions.
func NewStatusModel(opts AppOptions) StatusModel {
	source := ""
	if opts.Config != nil {
		source = opts.Config.Source
	}
	return StatusModel{
		state:    statusTabLoading,
		source:   source,
		config:   opts.Config,
		registry: opts.Registry,
		viewport: viewport.New(80, 20),
		keymap:   DefaultKeyMap(),
	}
}

// Init triggers the initial status walk.
func (m StatusModel) Init() tea.Cmd {
	return m.loadStatus()
}

// Update handles messages for the Status tab.
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - 10
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport = viewport.New(m.width, vpHeight)
		m.refreshViewport()
		return m, nil

	case tabActivatedMsg:
		// Refresh on tab activation.
		m.state = statusTabLoading
		return m, m.loadStatus()

	case statusResultMsg:
		if msg.err != nil {
			m.state = statusTabReady
			m.ledgerInfo = fmt.Sprintf("Error: %v", msg.err)
			return m, nil
		}
		m.categories = msg.categories
		m.ledgerInfo = msg.ledgerInfo
		m.activeCategory = 0
		m.state = statusTabReady
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		if m.state == statusTabLoading {
			return m, nil
		}
		switch msg.String() {
		case "r":
			m.state = statusTabLoading
			return m, m.loadStatus()
		case "1":
			m.setCategory(0)
		case "2":
			m.setCategory(1)
		case "3":
			m.setCategory(2)
		case "4":
			m.setCategory(3)
		case "5":
			m.setCategory(4)
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// setCategory switches the active category and refreshes the viewport.
func (m *StatusModel) setCategory(idx int) {
	if idx < 0 || idx >= len(m.categories) {
		return
	}
	m.activeCategory = idx
	m.refreshViewport()
}

// refreshViewport repopulates the viewport with the active category's files.
func (m *StatusModel) refreshViewport() {
	if len(m.categories) == 0 {
		m.viewport.SetContent("  (no data)")
		return
	}
	if m.activeCategory >= len(m.categories) {
		m.activeCategory = 0
	}
	cat := m.categories[m.activeCategory]
	if len(cat.files) == 0 {
		m.viewport.SetContent("  (none)")
		return
	}
	var sb strings.Builder
	for _, f := range cat.files {
		if f.detail != "" {
			fmt.Fprintf(&sb, "  %s  %s\n", f.relPath, dimStyle.Render(f.detail))
		} else {
			fmt.Fprintf(&sb, "  %s\n", f.relPath)
		}
	}
	m.viewport.SetContent(sb.String())
	m.viewport.GotoTop()
}

// loadStatus returns a tea.Cmd that walks the source directory in the background.
func (m StatusModel) loadStatus() tea.Cmd {
	source := m.source
	registry := m.registry
	cfg := m.config

	return func() tea.Msg {
		recursive := false
		var ignorePatterns []string
		if cfg != nil {
			recursive = cfg.Recursive
			ignorePatterns = cfg.Ignore
		}

		walkOpts := discovery.WalkOptions{
			Recursive: recursive,
			Ignore:    ignore.New(ignorePatterns),
		}
		discovered, skipped, err := discovery.Walk(source, registry, walkOpts)
		if err != nil {
			return statusResultMsg{err: fmt.Errorf("walk source: %w", err)}
		}

		lc, err := manifest.LoadLedger(source)
		if err != nil {
			return statusResultMsg{err: fmt.Errorf("load ledger: %w", err)}
		}

		// Build ledger lookup map.
		ledgerMap := make(map[string]domain.LedgerEntry)
		if lc != nil {
			for _, entry := range lc.Entries {
				ledgerMap[entry.Path] = entry
			}
		}

		// Classify files.
		var sorted, duplicates, errored, unsorted []statusFileEntry
		for _, df := range discovered {
			entry, found := ledgerMap[df.RelPath]
			if !found {
				unsorted = append(unsorted, statusFileEntry{relPath: df.RelPath})
				continue
			}
			switch entry.Status {
			case domain.LedgerStatusCopy:
				sorted = append(sorted, statusFileEntry{
					relPath: df.RelPath,
					detail:  "→ " + entry.Destination,
				})
			case domain.LedgerStatusDuplicate:
				detail := ""
				if entry.Matches != "" {
					detail = "→ matches " + entry.Matches
				}
				duplicates = append(duplicates, statusFileEntry{
					relPath: df.RelPath,
					detail:  detail,
				})
			case domain.LedgerStatusError:
				errored = append(errored, statusFileEntry{
					relPath: df.RelPath,
					detail:  "→ " + entry.Reason,
				})
			default:
				unsorted = append(unsorted, statusFileEntry{relPath: df.RelPath})
			}
		}

		var unrecognised []statusFileEntry
		for _, sf := range skipped {
			unrecognised = append(unrecognised, statusFileEntry{
				relPath: sf.Path,
				detail:  sf.Reason,
			})
		}

		// Sort each bucket alphabetically.
		sortEntries := func(files []statusFileEntry) {
			sort.Slice(files, func(i, j int) bool {
				return files[i].relPath < files[j].relPath
			})
		}
		sortEntries(sorted)
		sortEntries(duplicates)
		sortEntries(errored)
		sortEntries(unsorted)
		sortEntries(unrecognised)

		categories := []categoryData{
			{name: "Sorted", files: sorted},
			{name: "Duplicates", files: duplicates},
			{name: "Errored", files: errored},
			{name: "Unsorted", files: unsorted},
			{name: "Unrecognised", files: unrecognised},
		}

		ledgerInfo := "none"
		if lc != nil {
			h := lc.Header
			ledgerInfo = fmt.Sprintf("run %s  %s", truncID(h.RunID), h.PixeRun)
		}

		return statusResultMsg{
			categories: categories,
			ledgerInfo: ledgerInfo,
		}
	}
}

// truncID truncates a UUID to 8 characters for display.
func truncID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// View renders the Status tab.
func (m StatusModel) View() string {
	var sb strings.Builder
	sb.WriteString("\n")

	if m.state == statusTabLoading {
		sb.WriteString(dimStyle.Render("  Loading..."))
		sb.WriteString("\n")
		return sb.String()
	}

	// Source and ledger info.
	fmt.Fprintf(&sb, "  Source: %s\n", dimStyle.Render(m.source))
	fmt.Fprintf(&sb, "  Ledger: %s\n", dimStyle.Render(m.ledgerInfo))
	sb.WriteString("\n")

	// Summary counters.
	if len(m.categories) > 0 {
		var parts []string
		for _, cat := range m.categories {
			parts = append(parts, fmt.Sprintf("%s: %d", cat.name, len(cat.files)))
		}
		sb.WriteString("  ")
		sb.WriteString(dimStyle.Render(strings.Join(parts, "  │  ")))
		sb.WriteString("\n\n")
	}

	// Category selector.
	sb.WriteString("  ")
	for i, cat := range m.categories {
		label := fmt.Sprintf("[%d] %s (%d)", i+1, cat.name, len(cat.files))
		if i == m.activeCategory {
			sb.WriteString(categoryActiveStyle.Render(label))
		} else {
			sb.WriteString(categoryInactiveStyle.Render(label))
		}
		sb.WriteString(" ")
	}
	sb.WriteString("\n\n")

	// File list viewport.
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n")

	// Footer hints.
	sb.WriteString(dimStyle.Render("  1-5: category  r: refresh  j/k: scroll"))
	sb.WriteString("\n")

	return sb.String()
}
