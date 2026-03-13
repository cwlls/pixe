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

package xmp

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/cwlls/pixe/internal/domain"
)

// MergeTags reads the existing XMP sidecar at sidecarPath, injects the
// provided metadata tags, and writes the result back atomically.
//
// When overwrite is false (default), existing values for dc:rights,
// xmpRights:Marked, and aux:OwnerName are preserved — Pixe only fills
// in fields that are missing from the source XMP.
//
// When overwrite is true, Pixe's configured values replace any existing
// values for those fields.
//
// Missing namespace declarations (xmlns:dc, xmlns:xmpRights, xmlns:aux)
// are added to the rdf:Description element as needed.
//
// Returns nil if tags.IsEmpty() — no modification is made.
// Returns an error if the file cannot be read, parsed, or written.
//
// The write is atomic: the modified content is first written to a temporary
// file (<sidecarPath>.tmp) and then renamed to the final path.
func MergeTags(sidecarPath string, tags domain.MetadataTags, overwrite bool) error {
	if tags.IsEmpty() {
		return nil
	}

	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return fmt.Errorf("xmp: merge tags %q: read: %w", sidecarPath, err)
	}

	content := string(data)

	// Apply Copyright merge (dc:rights + xmpRights:Marked).
	if tags.Copyright != "" {
		content, err = mergeCopyright(content, tags.Copyright, overwrite)
		if err != nil {
			return fmt.Errorf("xmp: merge tags %q: copyright: %w", sidecarPath, err)
		}
	}

	// Apply CameraOwner merge (aux:OwnerName).
	if tags.CameraOwner != "" {
		content, err = mergeOwnerName(content, tags.CameraOwner, overwrite)
		if err != nil {
			return fmt.Errorf("xmp: merge tags %q: camera owner: %w", sidecarPath, err)
		}
	}

	// Atomic write.
	tmpPath := sidecarPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("xmp: merge tags %q: write tmp: %w", sidecarPath, err)
	}
	if err := os.Rename(tmpPath, sidecarPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("xmp: merge tags %q: rename: %w", sidecarPath, err)
	}
	return nil
}

// rdfDescriptionRE matches the opening rdf:Description tag (possibly multi-line)
// so we can inject namespace declarations into it.
var rdfDescriptionRE = regexp.MustCompile(`(?s)(<rdf:Description\b[^>]*?)(\s*>)`)

// dcRightsRE matches an existing dc:rights block (the full element).
var dcRightsRE = regexp.MustCompile(`(?s)<dc:rights>.*?</dc:rights>`)

// xmpRightsMarkedRE matches an existing xmpRights:Marked element.
var xmpRightsMarkedRE = regexp.MustCompile(`<xmpRights:Marked>[^<]*</xmpRights:Marked>`)

// auxOwnerNameRE matches an existing aux:OwnerName element.
var auxOwnerNameRE = regexp.MustCompile(`<aux:OwnerName>[^<]*</aux:OwnerName>`)

// mergeCopyright injects or replaces the dc:rights and xmpRights:Marked fields.
func mergeCopyright(content, copyright string, overwrite bool) (string, error) {
	hasDCRights := dcRightsRE.MatchString(content)
	hasXMPRightsMarked := xmpRightsMarkedRE.MatchString(content)

	newDCRights := buildDCRights(copyright)
	newXMPRightsMarked := "<xmpRights:Marked>True</xmpRights:Marked>"

	if hasDCRights {
		if overwrite {
			content = dcRightsRE.ReplaceAllString(content, newDCRights)
		}
		// overwrite=false: leave existing dc:rights intact
	} else {
		// Field missing — inject it. We insert before </rdf:Description>.
		content = injectBeforeDescriptionClose(content, newDCRights)
		// Also ensure namespace declarations are present.
		content = ensureNamespace(content, "dc", "http://purl.org/dc/elements/1.1/")
		content = ensureNamespace(content, "xmpRights", "http://ns.adobe.com/xap/1.0/rights/")
	}

	if hasXMPRightsMarked {
		if overwrite {
			content = xmpRightsMarkedRE.ReplaceAllString(content, newXMPRightsMarked)
		}
	} else {
		content = injectBeforeDescriptionClose(content, newXMPRightsMarked)
		content = ensureNamespace(content, "xmpRights", "http://ns.adobe.com/xap/1.0/rights/")
	}

	return content, nil
}

// mergeOwnerName injects or replaces the aux:OwnerName field.
func mergeOwnerName(content, ownerName string, overwrite bool) (string, error) {
	newOwner := "<aux:OwnerName>" + xmlEscape(ownerName) + "</aux:OwnerName>"

	if auxOwnerNameRE.MatchString(content) {
		if overwrite {
			content = auxOwnerNameRE.ReplaceAllString(content, newOwner)
		}
		// overwrite=false: leave existing value intact
	} else {
		content = injectBeforeDescriptionClose(content, newOwner)
		content = ensureNamespace(content, "aux", "http://ns.adobe.com/exif/1.0/aux/")
	}

	return content, nil
}

// buildDCRights constructs the dc:rights XML block for the given copyright string.
func buildDCRights(copyright string) string {
	return "<dc:rights>\n        <rdf:Alt>\n          <rdf:li xml:lang=\"x-default\">" +
		xmlEscape(copyright) +
		"</rdf:li>\n        </rdf:Alt>\n      </dc:rights>"
}

// injectBeforeDescriptionClose inserts element XML just before the closing
// </rdf:Description> tag. If no closing tag is found, the content is returned
// unchanged (malformed XMP — caller should not fail silently but we avoid panic).
func injectBeforeDescriptionClose(content, element string) string {
	const closeTag = "</rdf:Description>"
	idx := strings.LastIndex(content, closeTag)
	if idx < 0 {
		return content // malformed XMP — leave unchanged
	}
	return content[:idx] + "      " + element + "\n    " + content[idx:]
}

// ensureNamespace adds xmlns:<prefix>="<uri>" to the rdf:Description opening
// tag if it is not already present.
func ensureNamespace(content, prefix, uri string) string {
	decl := `xmlns:` + prefix + `="`
	if strings.Contains(content, decl) {
		return content // already declared
	}
	// Inject into the rdf:Description opening tag, before the closing >.
	return rdfDescriptionRE.ReplaceAllStringFunc(content, func(match string) string {
		// match is the full rdf:Description opening tag.
		// Insert the namespace declaration before the final >.
		sub := rdfDescriptionRE.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		return sub[1] + "\n      " + decl + uri + `"` + sub[2]
	})
}

// xmlEscape escapes the five XML special characters in a string value.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
