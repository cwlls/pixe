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

package pathbuilder

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// DefaultTemplate is the template string that reproduces the pre-template
// hardcoded directory structure. It is the default value when no
// --path-template flag or path_template config key is provided.
const DefaultTemplate = "{year}/{month}-{monthname}"

// knownTokens is the complete set of valid token names for path templates.
var knownTokens = map[string]bool{
	"year":      true,
	"month":     true,
	"monthname": true,
	"day":       true,
	"hour":      true,
	"minute":    true,
	"second":    true,
	"ext":       true,
}

// invalidPathChars lists characters that are not valid in directory names
// on any supported platform.
const invalidPathChars = `:*?"<>|`

// segment is a single parsed unit of a template: either a literal string
// or a token placeholder.
type segment struct {
	literal string // non-empty for literal text segments
	token   string // non-empty for token placeholder segments (e.g. "year")
}

// Template is a parsed, validated path template for the directory structure
// component of a destination path. It is immutable after construction via
// ParseTemplate.
//
// Only the directory structure is templated — the filename
// (YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>) is always fixed.
type Template struct {
	raw      string    // original template string as provided by the user
	segments []segment // parsed representation
}

// ParseTemplate parses and validates a path template string.
//
// Valid templates use {token} placeholders from the known set:
// {year}, {month}, {monthname}, {day}, {hour}, {minute}, {second}, {ext}.
// Literal text between tokens is preserved verbatim.
//
// Validation rules:
//  1. Template must not be empty.
//  2. All {token} names must be from the known set.
//  3. Braces must be balanced — no unclosed { or stray }.
//  4. Template must not start with /.
//  5. Template must not contain characters invalid in directory names: :*?"<>|
//     or a null byte.
//  6. No path component may be . or .. (checked after dummy expansion).
func ParseTemplate(raw string) (*Template, error) {
	if raw == "" {
		return nil, fmt.Errorf("pathbuilder: path template must not be empty")
	}

	// Rule 4: must not start with /.
	if strings.HasPrefix(raw, "/") {
		return nil, fmt.Errorf("pathbuilder: path template must not start with '/' (it is always relative to the destination directory): %q", raw)
	}

	// Rule 5: invalid characters (excluding / which is the path separator).
	if strings.ContainsAny(raw, invalidPathChars) || strings.ContainsRune(raw, 0) {
		return nil, fmt.Errorf("pathbuilder: path template contains invalid directory characters (%s or null byte): %q", invalidPathChars, raw)
	}

	// Parse into segments, validating brace balance and token names.
	segments, err := parseSegments(raw)
	if err != nil {
		return nil, err
	}

	tmpl := &Template{raw: raw, segments: segments}

	// Rule 6: no path component may be . or .. after dummy expansion.
	dummy := tmpl.Expand(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), "jpg")
	for _, component := range strings.Split(dummy, "/") {
		if component == "." || component == ".." {
			return nil, fmt.Errorf("pathbuilder: path template produces path-traversal component %q: %q", component, raw)
		}
	}

	return tmpl, nil
}

// parseSegments splits raw into a slice of literal and token segments,
// validating token names against knownTokens (the path-template token set).
// Returns an error if braces are unbalanced or a token name is unknown.
func parseSegments(raw string) ([]segment, error) {
	return parseSegmentsAllowed(raw, knownTokens)
}

// parseSegmentsAllowed is the shared brace-parser used by both path templates
// and copyright templates. allowed is the set of valid token names for the
// caller's context; an unknown token produces a fatal error listing the
// allowed set.
func parseSegmentsAllowed(raw string, allowed map[string]bool) ([]segment, error) {
	var segments []segment
	rest := raw

	for len(rest) > 0 {
		openIdx := strings.IndexByte(rest, '{')
		closeIdx := strings.IndexByte(rest, '}')

		// Stray closing brace before any opening brace.
		if closeIdx >= 0 && (openIdx < 0 || closeIdx < openIdx) {
			return nil, fmt.Errorf("pathbuilder: template has unexpected '}' at position %d: %q", len(raw)-len(rest)+closeIdx, raw)
		}

		if openIdx < 0 {
			// No more tokens — remainder is all literal.
			segments = append(segments, segment{literal: rest})
			break
		}

		// Literal text before the opening brace.
		if openIdx > 0 {
			segments = append(segments, segment{literal: rest[:openIdx]})
		}

		// Find the matching closing brace.
		rest = rest[openIdx+1:]
		closeIdx = strings.IndexByte(rest, '}')
		if closeIdx < 0 {
			return nil, fmt.Errorf("pathbuilder: template has unclosed '{': %q", raw)
		}

		// Nested opening brace inside a token.
		if nested := strings.IndexByte(rest[:closeIdx], '{'); nested >= 0 {
			return nil, fmt.Errorf("pathbuilder: template has nested '{' inside a token: %q", raw)
		}

		tokenName := rest[:closeIdx]
		if tokenName == "" {
			return nil, fmt.Errorf("pathbuilder: template has empty token '{}': %q", raw)
		}

		if !allowed[tokenName] {
			valid := sortedAllowedTokenNames(allowed)
			return nil, fmt.Errorf("pathbuilder: template has unknown token {%s} — valid tokens are: %s", tokenName, strings.Join(valid, ", "))
		}

		segments = append(segments, segment{token: tokenName})
		rest = rest[closeIdx+1:]
	}

	return segments, nil
}

// sortedAllowedTokenNames returns the token names from the given allowed set
// in sorted order, formatted as {name}, for use in error messages.
func sortedAllowedTokenNames(allowed map[string]bool) []string {
	names := make([]string, 0, len(allowed))
	for k := range allowed {
		names = append(names, "{"+k+"}")
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// Copyright template
// ---------------------------------------------------------------------------

// copyrightTokens is the set of valid token names for copyright templates.
// A subset of knownTokens — excludes {hour}, {minute}, {second}, {ext} which
// have no practical use in a copyright string.
var copyrightTokens = map[string]bool{
	"year":      true,
	"month":     true,
	"monthname": true,
	"day":       true,
}

// CopyrightTemplate is a parsed, validated copyright template string.
// Unlike Template, it has no path-specific validation rules (no leading-slash
// check, no invalid-path-char check, no traversal check).
// It is immutable after construction via ParseCopyrightTemplate.
type CopyrightTemplate struct {
	raw      string    // original template string as provided by the user
	segments []segment // parsed representation
}

// ParseCopyrightTemplate parses and validates a copyright template string.
//
// Uses the same {token} syntax as path templates but only allows:
// {year}, {month}, {monthname}, {day}.
//
// Validation rules:
//  1. Template must not be empty.
//  2. All {token} names must be from the copyright token set.
//  3. Braces must be balanced — no unclosed { or stray }.
//
// No path-specific rules (leading /, invalid chars, traversal) are applied.
func ParseCopyrightTemplate(raw string) (*CopyrightTemplate, error) {
	if raw == "" {
		return nil, fmt.Errorf("pathbuilder: copyright template must not be empty")
	}
	segments, err := parseSegmentsAllowed(raw, copyrightTokens)
	if err != nil {
		return nil, err
	}
	return &CopyrightTemplate{raw: raw, segments: segments}, nil
}

// Expand substitutes tokens in the copyright template with values derived
// from date, returning the rendered string.
func (ct *CopyrightTemplate) Expand(date time.Time) string {
	var sb strings.Builder
	for _, seg := range ct.segments {
		if seg.literal != "" {
			sb.WriteString(seg.literal)
			continue
		}
		switch seg.token {
		case "year":
			fmt.Fprintf(&sb, "%04d", date.Year())
		case "month":
			fmt.Fprintf(&sb, "%02d", int(date.Month()))
		case "monthname":
			sb.WriteString(localizedMonthAbbr(date.Month()))
		case "day":
			fmt.Fprintf(&sb, "%02d", date.Day())
		}
	}
	return sb.String()
}

// String returns the original template string as provided to ParseCopyrightTemplate.
func (ct *CopyrightTemplate) String() string {
	return ct.raw
}

// Expand applies the template to the given date and file extension, returning
// the directory path without a trailing separator.
//
// The ext parameter must be the lowercase extension without the leading dot
// (e.g. "jpg", not ".jpg"). The caller is responsible for lowercasing.
func (t *Template) Expand(date time.Time, ext string) string {
	var sb strings.Builder
	for _, seg := range t.segments {
		if seg.literal != "" {
			sb.WriteString(seg.literal)
			continue
		}
		switch seg.token {
		case "year":
			fmt.Fprintf(&sb, "%04d", date.Year())
		case "month":
			fmt.Fprintf(&sb, "%02d", int(date.Month()))
		case "monthname":
			sb.WriteString(localizedMonthAbbr(date.Month()))
		case "day":
			fmt.Fprintf(&sb, "%02d", date.Day())
		case "hour":
			fmt.Fprintf(&sb, "%02d", date.Hour())
		case "minute":
			fmt.Fprintf(&sb, "%02d", date.Minute())
		case "second":
			fmt.Fprintf(&sb, "%02d", date.Second())
		case "ext":
			sb.WriteString(ext)
		}
	}
	return sb.String()
}

// String returns the original template string as provided to ParseTemplate.
func (t *Template) String() string {
	return t.raw
}
