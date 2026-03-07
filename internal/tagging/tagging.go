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

// Package tagging handles Copyright template rendering and metadata tag
// injection into destination files via the FileTypeHandler contract.
//
// Template support:
//
//	The Copyright field supports Go text/template syntax. The only variable
//	currently exposed is {{.Year}}, which expands to the 4-digit capture year.
//	Example: "Copyright {{.Year}} My Family, all rights reserved"
//	→ "Copyright 2021 My Family, all rights reserved"
package tagging

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
)

// copyrightData is the template execution context.
type copyrightData struct {
	Year int
}

// RenderCopyright executes tmplStr as a Go text/template with the capture
// year derived from date. Returns the rendered string, or tmplStr unchanged
// if it contains no template directives or if parsing/execution fails.
func RenderCopyright(tmplStr string, date time.Time) string {
	if tmplStr == "" {
		return ""
	}
	tmpl, err := template.New("copyright").Parse(tmplStr)
	if err != nil {
		// Malformed template — return raw string rather than failing.
		return tmplStr
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, copyrightData{Year: date.Year()}); err != nil {
		return tmplStr
	}
	return buf.String()
}

// Apply injects tags into the file at destPath via handler.WriteMetadataTags.
// It is a no-op when tags.IsEmpty() is true, returning nil immediately without
// opening the file.
func Apply(destPath string, handler domain.FileTypeHandler, tags domain.MetadataTags) error {
	if tags.IsEmpty() {
		return nil
	}
	if err := handler.WriteMetadataTags(destPath, tags); err != nil {
		return fmt.Errorf("tagging: write metadata tags to %q: %w", destPath, err)
	}
	return nil
}
