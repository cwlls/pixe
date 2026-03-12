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

// Package tui implements the interactive terminal UI for `pixe gui`.
//
// All Lip Gloss styles use adaptive colors only — no hardcoded hex values —
// so the UI looks correct on both light and dark terminals.
package tui

import "github.com/charmbracelet/lipgloss"

// Tab bar styles.
var (
	tabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"}).
			Background(lipgloss.AdaptiveColor{Light: "#dddddd", Dark: "#444444"}).
			Padding(0, 2).
			Bold(true)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"}).
				Padding(0, 2)
)

// Header / footer styles.
var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"}).
			Bold(true)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"})
)

// Border / divider style.
var borderStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#cccccc", Dark: "#444444"})

// Progress bar wrapper style.
var progressBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

// Activity log verb styles.
var (
	logCopyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#006600", Dark: "#66cc66"})

	logDupeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#886600", Dark: "#ccaa44"})

	logSkipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"})

	logErrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#ff6666"})
)

// Worker status pane styles.
var (
	workerActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#006600", Dark: "#66cc66"})

	workerIdleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#aaaaaa", Dark: "#666666"})
)

// Error overlay style.
var overlayStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#ff6666"}).
	Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"}).
	Padding(1, 2)

// Dim / secondary text style.
var dimStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})

// Status tab category selector styles.
var (
	categoryActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"}).
				Background(lipgloss.AdaptiveColor{Light: "#dddddd", Dark: "#444444"}).
				Padding(0, 1).
				Bold(true)

	categoryInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"}).
				Padding(0, 1)
)
