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

// Package xmp generates Adobe-compatible XMP sidecar files for media
// formats that cannot safely embed metadata. The sidecar follows the
// Adobe naming convention: <filename>.<ext>.xmp.
//
// The generated XMP packet is a standards-compliant XML document with
// the Adobe XMP packet wrapper. Only namespaces for fields that are
// actually present are included in the rdf:Description element.
//
// Atomic write:
//
//	The sidecar is written to a temporary file (<path>.tmp) and then
//	renamed to the final path. This prevents partial sidecar files
//	from being left on disk if the process is interrupted.
package xmp

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/cwlls/pixe/internal/domain"
)

// xmpTemplate is the XMP packet template. It conditionally includes
// namespace declarations and field elements based on which tags are set.
//
// The xpacket begin attribute contains the UTF-8 BOM (U+FEFF) as required
// by the Adobe XMP specification for the packet wrapper.
const xmpTemplate = `<?xpacket begin="` + "\ufeff" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""{{if .Copyright}}
      xmlns:dc="http://purl.org/dc/elements/1.1/"
      xmlns:xmpRights="http://ns.adobe.com/xap/1.0/rights/"{{end}}{{if .CameraOwner}}
      xmlns:aux="http://ns.adobe.com/exif/1.0/aux/"{{end}}>{{if .Copyright}}
      <dc:rights>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">{{.Copyright}}</rdf:li>
        </rdf:Alt>
      </dc:rights>
      <xmpRights:Marked>True</xmpRights:Marked>{{end}}{{if .CameraOwner}}
      <aux:OwnerName>{{.CameraOwner}}</aux:OwnerName>{{end}}
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>
`

// parsedTemplate is the compiled xmpTemplate, initialised once at package load.
var parsedTemplate = template.Must(template.New("xmp").Parse(xmpTemplate))

// SidecarPath returns the XMP sidecar path for the given media file.
// It appends ".xmp" to the full filename (Adobe convention).
//
// Example:
//
//	"/archive/2021/12-Dec/20211225_062223_abc123.arw"
//	→ "/archive/2021/12-Dec/20211225_062223_abc123.arw.xmp"
func SidecarPath(mediaPath string) string {
	return mediaPath + ".xmp"
}

// WriteSidecar generates and writes an XMP sidecar file alongside the
// media file at mediaPath. The sidecar contains the provided metadata
// tags in a standards-compliant XMP packet.
//
// Returns nil if tags.IsEmpty() — no sidecar is written.
// Returns an error if the file cannot be created or written.
//
// The write is atomic: the content is first written to a temporary file
// (<sidecarPath>.tmp) and then renamed to the final path.
func WriteSidecar(mediaPath string, tags domain.MetadataTags) error {
	if tags.IsEmpty() {
		return nil
	}

	sidecarPath := SidecarPath(mediaPath)
	tmpPath := sidecarPath + ".tmp"

	// XML-escape user-supplied values before injecting into the template.
	// text/template does not escape XML special characters, so a copyright
	// string containing &, <, >, " or ' would produce malformed XML.
	escapedTags := domain.MetadataTags{
		Copyright:   xmlEscape(tags.Copyright),
		CameraOwner: xmlEscape(tags.CameraOwner),
	}

	var buf bytes.Buffer
	if err := parsedTemplate.Execute(&buf, escapedTags); err != nil {
		return fmt.Errorf("xmp: write sidecar %q: render template: %w", sidecarPath, err)
	}

	if err := os.WriteFile(tmpPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("xmp: write sidecar %q: %w", sidecarPath, err)
	}

	if err := os.Rename(tmpPath, sidecarPath); err != nil {
		// Best-effort cleanup of the temp file.
		_ = os.Remove(tmpPath)
		return fmt.Errorf("xmp: write sidecar %q: %w", sidecarPath, err)
	}

	return nil
}
