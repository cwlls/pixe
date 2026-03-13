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

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/cwlls/pixe/internal/discovery"
)

// Formatter produces status lines with optional color.
// When color is true a dedicated Lip Gloss renderer with TrueColor profile is
// used so that ANSI escape codes are emitted even when the output writer is not
// a TTY (e.g. bytes.Buffer in tests, or a pipe where the caller has already
// confirmed the terminal supports color).
type Formatter struct {
	color    bool
	renderer *lipgloss.Renderer
}

// NewFormatter creates a Formatter. When color is true, status verbs are
// rendered with Lip Gloss adaptive colors using a forced TrueColor profile.
// os.Stdout is passed to the renderer so termenv can probe the terminal's
// dark/light background preference for AdaptiveColor selection.
func NewFormatter(color bool) *Formatter {
	f := &Formatter{color: color}
	if color {
		r := lipgloss.NewRenderer(os.Stdout)
		r.SetColorProfile(termenv.TrueColor)
		f.renderer = r
	}
	return f
}

// style returns a Lip Gloss style bound to the formatter's renderer.
func (f *Formatter) style() lipgloss.Style {
	if f.renderer != nil {
		return f.renderer.NewStyle()
	}
	return lipgloss.NewStyle()
}

// verbStyle returns the colored style for a given verb.
func (f *Formatter) verbStyle(verb string) lipgloss.Style {
	switch verb {
	case "COPY":
		return f.style().Foreground(lipgloss.AdaptiveColor{Light: "#22863a", Dark: "#85e89d"})
	case "DUPE":
		return f.style().Foreground(lipgloss.AdaptiveColor{Light: "#b08800", Dark: "#ffdf5d"})
	case "ERR ", "ERR":
		return f.style().Foreground(lipgloss.AdaptiveColor{Light: "#cb2431", Dark: "#f97583"}).Bold(true)
	case "SKIP":
		return f.style().Foreground(lipgloss.AdaptiveColor{Light: "#959da5", Dark: "#6a737d"})
	default:
		return f.style()
	}
}

// FormatOutput returns a single stdout line for a file outcome.
// verb is one of "COPY", "SKIP", "DUPE", "ERR ".
func (f *Formatter) FormatOutput(verb, source, detail string) string {
	if f.color {
		verb = f.verbStyle(verb).Render(verb)
	}
	return fmt.Sprintf("%s %s -> %s\n", verb, source, detail)
}

// FormatOutput returns a single stdout line for a file outcome with an optional
// sidecar annotation appended inline (e.g. " [+xmp]").
// verb is one of "COPY", "SKIP", "DUPE", "ERR ".
// annotation is the result of formatSidecarAnnotation; pass "" for no annotation.
func (f *Formatter) FormatOutputWithAnnotation(verb, source, detail, annotation string) string {
	if f.color {
		verb = f.verbStyle(verb).Render(verb)
	}
	return fmt.Sprintf("%s %s -> %s%s\n", verb, source, detail, annotation)
}

// formatSidecarAnnotation builds the inline sidecar annotation string from a
// list of sidecar extensions (e.g. []string{".xmp", ".aae"} → " [+xmp +aae]").
// Returns "" when exts is empty.
func formatSidecarAnnotation(exts []string) string {
	if len(exts) == 0 {
		return ""
	}
	var parts []string
	for _, ext := range exts {
		// Strip leading dot for display: ".xmp" → "+xmp"
		if len(ext) > 1 && ext[0] == '.' {
			parts = append(parts, "+"+ext[1:])
		} else {
			parts = append(parts, "+"+ext)
		}
	}
	result := " ["
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	result += "]"
	return result
}

// sidecarExts returns the unique extensions of successfully carried sidecars
// in a stable order (.xmp before .aae).
func sidecarExts(sidecars []discovery.SidecarFile, carried []string) []string {
	if len(carried) == 0 {
		return nil
	}
	// Build a set of carried extensions from the carried rel paths.
	carriedSet := make(map[string]struct{}, len(carried))
	for _, rel := range carried {
		ext := filepath.Ext(rel)
		carriedSet[ext] = struct{}{}
	}
	// Return in canonical order.
	var exts []string
	for _, order := range []string{".xmp", ".aae"} {
		if _, ok := carriedSet[order]; ok {
			exts = append(exts, order)
		}
	}
	// Any other extensions not in the canonical list.
	for ext := range carriedSet {
		if ext != ".xmp" && ext != ".aae" {
			exts = append(exts, ext)
		}
	}
	_ = sidecars // parameter kept for future use
	return exts
}

// FormatWarning returns a warning line (sidecar carry failure, tag failure, etc.).
func (f *Formatter) FormatWarning(msg string) string {
	prefix := "  WARNING  "
	if f.color {
		warnStyle := f.style().Foreground(lipgloss.AdaptiveColor{Light: "#b08800", Dark: "#ffdf5d"})
		prefix = "  " + warnStyle.Render("WARNING") + "  "
	}
	return prefix + msg + "\n"
}
