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

package main

import (
	"fmt"
	"strings"
)

// FormatMarkdownTable renders rows as a GitHub-Flavored Markdown table.
// headers is the first row; rows are subsequent data rows.
// Output is deterministic: no trailing whitespace, consistent newlines.
func FormatMarkdownTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Compute column widths (minimum = header width).
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder

	// Header row.
	sb.WriteString("|")
	for i, h := range headers {
		sb.WriteString(fmt.Sprintf(" %-*s |", widths[i], h))
	}
	sb.WriteString("\n")

	// Separator row.
	sb.WriteString("|")
	for _, w := range widths {
		sb.WriteString(" " + strings.Repeat("-", w) + " |")
	}
	sb.WriteString("\n")

	// Data rows.
	for _, row := range rows {
		sb.WriteString("|")
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			sb.WriteString(fmt.Sprintf(" %-*s |", widths[i], cell))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatHTMLTable renders rows as an HTML <table> with class "flag-table".
// Used for docs/commands.md where the Jekyll theme styles HTML tables.
// Output is deterministic: no trailing whitespace, consistent newlines.
func FormatHTMLTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("<table class=\"flag-table\">\n")
	sb.WriteString("  <thead>\n")
	sb.WriteString("    <tr>\n")
	for _, h := range headers {
		sb.WriteString(fmt.Sprintf("      <th>%s</th>\n", h))
	}
	sb.WriteString("    </tr>\n")
	sb.WriteString("  </thead>\n")
	sb.WriteString("  <tbody>\n")
	for _, row := range rows {
		sb.WriteString("    <tr>\n")
		for i := range headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			sb.WriteString(fmt.Sprintf("      <td>%s</td>\n", cell))
		}
		sb.WriteString("    </tr>\n")
	}
	sb.WriteString("  </tbody>\n")
	sb.WriteString("</table>")

	return sb.String()
}

// FormatGoCodeBlock wraps Go source text in a fenced ```go code block.
func FormatGoCodeBlock(source string) string {
	trimmed := strings.TrimRight(source, "\n")
	return "```go\n" + trimmed + "\n```"
}

// FormatYAMLValue renders a single key: "value" YAML line.
func FormatYAMLValue(key string, value string) string {
	return fmt.Sprintf("%s: %q", key, value)
}
