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

// Package cli provides the Bubble Tea progress bar model used by the
// `pixe sort --progress` and `pixe verify --progress` commands.
//
// All Lip Gloss styles use adaptive colors only — no hardcoded hex values —
// so the UI looks correct on both light and dark terminals.
package cli

import "github.com/charmbracelet/lipgloss"

// headerStyle renders the command header line (source → dest).
var headerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"})

// counterStyle renders status counters (copied, duplicates, skipped).
var counterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#cccccc"})

// errorCountStyle renders the error counter with slight emphasis.
var errorCountStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#cc0000", Dark: "#ff6666"}).
	Bold(true)

// currentFileStyle renders the current file being processed (dim, truncated).
var currentFileStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"})

// etaStyle renders the ETA estimate (dim, right-aligned).
var etaStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#888888"})

// labelStyle renders counter labels.
var labelStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#aaaaaa"})
