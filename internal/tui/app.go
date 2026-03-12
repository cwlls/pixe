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

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	"github.com/cwlls/pixe-go/internal/hash"
)

// tabActivatedMsg is sent to a sub-model when its tab becomes active.
type tabActivatedMsg struct{}

// AppOptions holds the dependencies passed from cmd/gui.go to the TUI.
type AppOptions struct {
	Config   *config.AppConfig
	Registry *discovery.Registry
	Hasher   *hash.Hasher
	Version  string
}

const (
	tabSort   = 0
	tabVerify = 1
	tabStatus = 2
)

// App is the root Bubble Tea model for `pixe gui`.
type App struct {
	activeTab int
	tabs      []string

	sort   SortModel
	verify VerifyModel
	status StatusModel

	width  int
	height int
	keymap KeyMap
	help   help.Model
}

// NewApp creates the root App model with all three sub-models initialized.
func NewApp(opts AppOptions) App {
	km := DefaultKeyMap()
	return App{
		activeTab: tabSort,
		tabs:      []string{"Sort", "Verify", "Status"},
		sort:      NewSortModel(opts),
		verify:    NewVerifyModel(opts),
		status:    NewStatusModel(opts),
		keymap:    km,
		help:      help.New(),
	}
}

// Init returns the initial commands for all sub-models.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.sort.Init(),
		a.verify.Init(),
		a.status.Init(),
	)
}

// Update handles messages and delegates to the active sub-model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		// Propagate to sub-models.
		contentHeight := a.height - headerHeight() - footerHeight()
		if contentHeight < 1 {
			contentHeight = 1
		}
		subMsg := tea.WindowSizeMsg{Width: a.width, Height: contentHeight}
		var cmds []tea.Cmd
		var cmd tea.Cmd
		var m tea.Model

		m, cmd = a.sort.Update(subMsg)
		a.sort = m.(SortModel)
		cmds = append(cmds, cmd)

		m, cmd = a.verify.Update(subMsg)
		a.verify = m.(VerifyModel)
		cmds = append(cmds, cmd)

		m, cmd = a.status.Update(subMsg)
		a.status = m.(StatusModel)
		cmds = append(cmds, cmd)

		return a, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch {
		case msg.String() == "q" || msg.String() == "ctrl+c":
			return a, tea.Quit

		case msg.String() == "tab":
			return a.switchTab((a.activeTab + 1) % len(a.tabs))

		case msg.String() == "shift+tab":
			prev := a.activeTab - 1
			if prev < 0 {
				prev = len(a.tabs) - 1
			}
			return a.switchTab(prev)

		case msg.String() == "1":
			return a.switchTab(tabSort)

		case msg.String() == "2":
			return a.switchTab(tabVerify)

		case msg.String() == "3":
			return a.switchTab(tabStatus)
		}

		// Delegate to active sub-model.
		return a.delegateToActive(msg)
	}

	// Delegate all other messages to the active sub-model.
	return a.delegateToActive(msg)
}

// switchTab changes the active tab and sends tabActivatedMsg to the new tab.
func (a App) switchTab(idx int) (tea.Model, tea.Cmd) {
	if idx == a.activeTab {
		return a, nil
	}
	a.activeTab = idx
	return a.delegateToActive(tabActivatedMsg{})
}

// delegateToActive forwards a message to the active sub-model.
func (a App) delegateToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var m tea.Model
	switch a.activeTab {
	case tabSort:
		m, cmd = a.sort.Update(msg)
		a.sort = m.(SortModel)
	case tabVerify:
		m, cmd = a.verify.Update(msg)
		a.verify = m.(VerifyModel)
	case tabStatus:
		m, cmd = a.status.Update(msg)
		a.status = m.(StatusModel)
	}
	return a, cmd
}

// View renders the full TUI: header, content area, footer.
func (a App) View() string {
	var sb strings.Builder

	// Header.
	sb.WriteString(a.renderHeader())
	sb.WriteString("\n")

	// Content area.
	contentHeight := a.height - headerHeight() - footerHeight()
	if contentHeight < 1 {
		contentHeight = 1
	}
	_ = contentHeight // sub-models manage their own height

	switch a.activeTab {
	case tabSort:
		sb.WriteString(a.sort.View())
	case tabVerify:
		sb.WriteString(a.verify.View())
	case tabStatus:
		sb.WriteString(a.status.View())
	}

	// Footer.
	sb.WriteString("\n")
	sb.WriteString(a.renderFooter())

	return sb.String()
}

// renderHeader renders the app name and tab bar.
func (a App) renderHeader() string {
	appName := headerStyle.Render("pixe gui")

	var tabParts []string
	for i, name := range a.tabs {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		if i == a.activeTab {
			tabParts = append(tabParts, tabActiveStyle.Render(label))
		} else {
			tabParts = append(tabParts, tabInactiveStyle.Render(label))
		}
	}
	tabBar := strings.Join(tabParts, " ")

	// Right-align tab bar.
	gap := a.width - lipgloss.Width(appName) - lipgloss.Width(tabBar) - 2
	if gap < 1 {
		gap = 1
	}
	return appName + strings.Repeat(" ", gap) + tabBar
}

// renderFooter renders the context-sensitive help line.
func (a App) renderFooter() string {
	helpStr := footerStyle.Render("tab: next  shift+tab: prev  1/2/3: jump  q: quit  ?: help")
	return helpStr
}

// headerHeight returns the number of lines used by the header.
func headerHeight() int { return 2 } // header line + blank line

// footerHeight returns the number of lines used by the footer.
func footerHeight() int { return 2 } // blank line + footer line
