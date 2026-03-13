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
//	The Copyright field uses {token} syntax, parsed by pathbuilder.ParseCopyrightTemplate.
//	Available tokens: {year}, {month}, {monthname}, {day}.
//	Example: "Copyright {year} My Family, all rights reserved"
//	→ "Copyright 2021 My Family, all rights reserved"
//
//	The template is validated at startup; unknown tokens produce a fatal error.
//
// Dispatch strategy:
//
//	Apply checks handler.MetadataSupport() and routes accordingly:
//	  MetadataEmbed   → handler.WriteMetadataTags (in-file EXIF/atoms)
//	  MetadataSidecar → xmp.WriteSidecar (XMP sidecar file)
//	  MetadataNone    → no-op
package tagging

import (
	"fmt"
	"time"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/xmp"
)

// RenderCopyright expands a parsed copyright template with values derived from
// date. Returns an empty string when tmpl is nil (no copyright configured).
func RenderCopyright(tmpl *pathbuilder.CopyrightTemplate, date time.Time) string {
	if tmpl == nil {
		return ""
	}
	return tmpl.Expand(date)
}

// Apply persists metadata tags for the file at destPath. The strategy
// depends on the handler's declared MetadataSupport capability:
//
//   - MetadataEmbed:   calls handler.WriteMetadataTags (in-file EXIF/atoms)
//   - MetadataSidecar: writes an XMP sidecar via xmp.WriteSidecar
//   - MetadataNone:    no-op, returns nil
//
// Returns nil immediately when tags.IsEmpty().
// Apply is a convenience wrapper around ApplyWithSidecars with no carried XMP.
func Apply(destPath string, handler domain.FileTypeHandler, tags domain.MetadataTags) error {
	return ApplyWithSidecars(destPath, handler, tags, "", false)
}

// ApplyWithSidecars persists metadata tags, accounting for a carried source
// .xmp sidecar. When carriedXMP is non-empty and the handler declares
// MetadataSidecar, Pixe merges tags into the existing carried sidecar instead
// of generating a new one from the template.
//
//   - MetadataEmbed:   calls handler.WriteMetadataTags (carriedXMP is ignored)
//   - MetadataSidecar: merges into carriedXMP if non-empty; otherwise generates
//     a new XMP sidecar via xmp.WriteSidecar
//   - MetadataNone:    no-op, returns nil
//
// overwrite controls the merge behaviour: when false (default), existing values
// in the source .xmp are preserved; when true, Pixe's values replace them.
//
// Returns nil immediately when tags.IsEmpty().
func ApplyWithSidecars(destPath string, handler domain.FileTypeHandler, tags domain.MetadataTags, carriedXMP string, overwrite bool) error {
	if tags.IsEmpty() {
		return nil
	}
	switch handler.MetadataSupport() {
	case domain.MetadataEmbed:
		if err := handler.WriteMetadataTags(destPath, tags); err != nil {
			return fmt.Errorf("tagging: embed metadata in %q: %w", destPath, err)
		}
	case domain.MetadataSidecar:
		if carriedXMP != "" {
			// Merge into the carried source .xmp sidecar.
			if err := xmp.MergeTags(carriedXMP, tags, overwrite); err != nil {
				return fmt.Errorf("tagging: merge tags into carried sidecar %q: %w", carriedXMP, err)
			}
		} else {
			// No carried sidecar — generate from template (existing behaviour).
			if err := xmp.WriteSidecar(destPath, tags); err != nil {
				return fmt.Errorf("tagging: write sidecar for %q: %w", destPath, err)
			}
		}
	case domain.MetadataNone:
		// No tagging for this format.
	}
	return nil
}
