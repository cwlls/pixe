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

// Package main implements the docgen tool that injects generated content
// into documentation files using marker-based replacement.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Replacement holds the generated content for a single marker section.
type Replacement struct {
	Name    string // marker section name (e.g., "sort-flags")
	Content string // generated content to inject (excluding markers themselves)
}

// markerBegin matches <!-- pixe:begin:NAME --> and # <!-- pixe:begin:NAME -->
var markerBegin = regexp.MustCompile(`^(\s*#\s*)?<!-- pixe:begin:([a-z0-9-]+)(?:\s+[^>]*)? -->`)

// markerEnd matches <!-- pixe:end:NAME --> and # <!-- pixe:end:NAME -->
var markerEnd = regexp.MustCompile(`^(\s*#\s*)?<!-- pixe:end:([a-z0-9-]+) -->`)

// InjectFile reads the file at path, replaces all marker sections with the
// provided replacements, and returns the resulting content. It does not write
// the file — the caller decides whether to write (normal mode) or compare
// (--check mode). Returns an error if a marker pair is malformed (begin
// without end, or end without begin).
func InjectFile(path string, replacements []Replacement) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("inject: read %q: %w", path, err)
	}
	return injectContent(string(data), replacements)
}

// injectContent performs marker replacement on the given content string.
// Exported for testing without filesystem access.
func injectContent(content string, replacements []Replacement) (string, error) {
	// Build a lookup map for fast access.
	repMap := make(map[string]string, len(replacements))
	for _, r := range replacements {
		repMap[r.Name] = r.Content
	}

	lines := strings.Split(content, "\n")
	var out []string
	var currentSection string
	var currentBeginLine int
	var inSection bool

	for i, line := range lines {
		lineNum := i + 1

		// Check for begin marker.
		if m := markerBegin.FindStringSubmatch(line); m != nil {
			if inSection {
				return "", fmt.Errorf("inject: nested begin marker at line %d (already inside section %q)", lineNum, currentSection)
			}
			yamlPrefix := m[1]
			name := m[2]
			inSection = true
			currentSection = name
			currentBeginLine = lineNum

			// Emit the begin marker line as-is.
			out = append(out, line)

			// Emit the replacement content if we have one.
			if newContent, ok := repMap[name]; ok {
				if newContent != "" {
					// Blank line after begin marker so Markdown renderers
					// (GitHub Pages / kramdown) recognise tables and fenced
					// blocks that start immediately after the marker.
					trimmed := strings.TrimRight(newContent, "\n")
					// Re-apply yaml prefix to content lines if needed.
					_ = yamlPrefix // yaml prefix only applies to markers, not content
					out = append(out, "", trimmed, "")
				}
			}
			continue
		}

		// Check for end marker.
		if m := markerEnd.FindStringSubmatch(line); m != nil {
			name := m[2]
			if !inSection {
				return "", fmt.Errorf("inject: end marker for %q at line %d without matching begin", name, lineNum)
			}
			if name != currentSection {
				return "", fmt.Errorf("inject: end marker for %q at line %d does not match open begin for %q (started at line %d)", name, lineNum, currentSection, currentBeginLine)
			}
			inSection = false
			currentSection = ""
			// Emit the end marker line.
			out = append(out, line)
			continue
		}

		// If inside a section, skip existing content (it will be replaced).
		if inSection {
			continue
		}

		// Outside any section — emit as-is.
		out = append(out, line)
	}

	if inSection {
		return "", fmt.Errorf("inject: begin marker for %q at line %d has no matching end marker", currentSection, currentBeginLine)
	}

	return strings.Join(out, "\n"), nil
}

// WriteIfChanged writes content to path only if it differs from the current
// file contents. Returns true if the file was written, false if unchanged.
// Preserves the file's original permissions.
func WriteIfChanged(path string, content string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("inject: read existing %q: %w", path, err)
	}

	if string(existing) == content {
		return false, nil
	}

	// Determine permissions from existing file, or use default.
	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return false, fmt.Errorf("inject: write %q: %w", path, err)
	}
	return true, nil
}
