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

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds all key bindings for the TUI application.
type KeyMap struct {
	// Global navigation.
	Tab      key.Binding
	ShiftTab key.Binding
	Tab1     key.Binding
	Tab2     key.Binding
	Tab3     key.Binding
	Quit     key.Binding
	Help     key.Binding

	// Sort tab actions.
	StartSort    key.Binding
	EditSettings key.Binding
	NewRun       key.Binding

	// Log navigation.
	ScrollUp   key.Binding
	ScrollDown key.Binding
	Top        key.Binding
	Bottom     key.Binding
	Filter     key.Binding
	Inspect    key.Binding

	// Overlay.
	Close key.Binding

	// Status tab.
	Category1 key.Binding
	Category2 key.Binding
	Category3 key.Binding
	Category4 key.Binding
	Category5 key.Binding
	Refresh   key.Binding

	// Verify tab.
	StartVerify key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "Sort tab"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "Verify tab"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "Status tab"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		StartSort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "start sort"),
		),
		EditSettings: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit settings"),
		),
		NewRun: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new run"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "scroll down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Inspect: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "inspect"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close"),
		),
		Category1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "sorted"),
		),
		Category2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "duplicates"),
		),
		Category3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "errored"),
		),
		Category4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "unsorted"),
		),
		Category5: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "unrecognised"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		StartVerify: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "start verify"),
		),
	}
}

// ShortHelp returns the short help bindings for the global keymap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Quit, k.Help}
}

// FullHelp returns the full help bindings for the global keymap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.ShiftTab, k.Tab1, k.Tab2, k.Tab3},
		{k.Quit, k.Help},
	}
}
