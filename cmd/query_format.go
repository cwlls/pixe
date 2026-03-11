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

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// queryResult is the top-level JSON envelope for --json output.
type queryResult struct {
	Query   string `json:"query"`
	Dir     string `json:"dir"`
	Results any    `json:"results"`
	Summary any    `json:"summary"`
}

// printQueryJSON marshals a queryResult to w as indented JSON.
func printQueryJSON(w io.Writer, query string, results any, summary any) error {
	qr := queryResult{
		Query:   query,
		Dir:     queryDir,
		Results: results,
		Summary: summary,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(qr)
}

// printQueryTable prints a fixed-width columnar table followed by a summary line.
//
// headers contains the column names (displayed in uppercase).
// rows contains one []string per data row; each element corresponds to a column.
// summary is printed after a blank separator line; pass "" to suppress it.
//
// Column widths are computed as the maximum of the header width and the widest
// value in that column across all rows.
func printQueryTable(w io.Writer, headers []string, rows [][]string, summary string) {
	if len(rows) == 0 && summary == "" {
		return
	}

	// Compute column widths.
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

	// Print header row.
	printRow(w, headers, widths)

	// Print separator.
	parts := make([]string, len(headers))
	for i, width := range widths {
		parts[i] = strings.Repeat("-", width)
	}
	_, _ = fmt.Fprintln(w, strings.Join(parts, "  "))

	// Print data rows.
	for _, row := range rows {
		printRow(w, row, widths)
	}

	// Print summary.
	if summary != "" {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, summary)
	}
}

// printRow writes a single padded row to w.
func printRow(w io.Writer, cells []string, widths []int) {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		if i < len(widths) {
			parts[i] = fmt.Sprintf("%-*s", widths[i], cell)
		} else {
			parts[i] = cell
		}
	}
	// Trim trailing whitespace from the last column.
	if len(parts) > 0 {
		parts[len(parts)-1] = strings.TrimRight(parts[len(parts)-1], " ")
	}
	_, _ = fmt.Fprintln(w, strings.Join(parts, "  "))
}

// truncChecksum returns the first 8 characters of a checksum for table display.
// Returns "—" if the input is empty.
func truncChecksum(s string) string {
	if s == "" {
		return "—"
	}
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// truncID returns the first 8 characters of a UUID for table display.
func truncID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

// formatDate formats a *time.Time as "YYYY-MM-DD" for table display.
// Returns "—" if t is nil.
func formatDate(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("2006-01-02")
}

// formatDateTime formats a time.Time as "YYYY-MM-DD HH:MM:SS" for table display.
func formatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// commaInt formats an integer with comma separators (e.g., 1247 → "1,247").
func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	// Insert commas every 3 digits from the right.
	var b strings.Builder
	rem := len(s) % 3
	for i, ch := range s {
		if i > 0 && (i-rem)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	if n < 0 {
		return "-" + b.String()
	}
	return b.String()
}
