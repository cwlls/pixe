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
	"path/filepath"

	"github.com/cwlls/pixe/internal/discovery"
)

// ANSI 256-color escape codes for verb coloring.
// These mid-range palette values are readable on both light and dark terminals.
const (
	ansiReset   = "\033[0m"
	ansiGreen   = "\033[38;5;114m"   // COPY — green
	ansiYellow  = "\033[38;5;179m"   // DUPE, WARNING — amber/yellow
	ansiBoldRed = "\033[1;38;5;204m" // ERR — bold red
	ansiGray    = "\033[38;5;102m"   // SKIP — gray
)

// ansiWrap wraps text in an ANSI color escape sequence.
// Returns text unchanged when color is false.
func ansiWrap(text, code string, color bool) string {
	if !color {
		return text
	}
	return code + text + ansiReset
}

// colorForVerb returns the ANSI color code for a given pipeline verb.
func colorForVerb(verb string) string {
	switch verb {
	case "COPY":
		return ansiGreen
	case "DUPE":
		return ansiYellow
	case "ERR ", "ERR":
		return ansiBoldRed
	case "SKIP":
		return ansiGray
	default:
		return ""
	}
}

// Formatter produces status lines with optional color.
// When color is true, ANSI 256-color escape codes are used for verb labels.
// The color bool is set by the caller based on TTY detection and NO_COLOR.
type Formatter struct {
	color bool
}

// NewFormatter creates a Formatter. When color is true, status verbs are
// rendered with ANSI 256-color codes.
func NewFormatter(color bool) *Formatter {
	return &Formatter{color: color}
}

// FormatOutput returns a single stdout line for a file outcome.
// verb is one of "COPY", "SKIP", "DUPE", "ERR ".
func (f *Formatter) FormatOutput(verb, source, detail string) string {
	v := ansiWrap(verb, colorForVerb(verb), f.color)
	return fmt.Sprintf("%s %s -> %s\n", v, source, detail)
}

// FormatOutputWithAnnotation returns a single stdout line for a file outcome
// with an optional sidecar annotation appended inline (e.g. " [+xmp]").
// verb is one of "COPY", "SKIP", "DUPE", "ERR ".
// annotation is the result of formatSidecarAnnotation; pass "" for no annotation.
func (f *Formatter) FormatOutputWithAnnotation(verb, source, detail, annotation string) string {
	v := ansiWrap(verb, colorForVerb(verb), f.color)
	return fmt.Sprintf("%s %s -> %s%s\n", v, source, detail, annotation)
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
	prefix := "  " + ansiWrap("WARNING", ansiYellow, f.color) + "  "
	return prefix + msg + "\n"
}
