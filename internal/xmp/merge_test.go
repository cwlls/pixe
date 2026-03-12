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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwlls/pixe-go/internal/domain"
)

// minimalXMP is a bare-bones XMP sidecar with an empty rdf:Description.
const minimalXMP = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="">
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// xmpWithCopyright is an XMP sidecar that already has dc:rights and xmpRights:Marked.
const xmpWithCopyright = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:dc="http://purl.org/dc/elements/1.1/"
      xmlns:xmpRights="http://ns.adobe.com/xap/1.0/rights/">
      <dc:rights>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">Existing Copyright 2020</rdf:li>
        </rdf:Alt>
      </dc:rights>
      <xmpRights:Marked>True</xmpRights:Marked>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// xmpWithOwner is an XMP sidecar that already has aux:OwnerName.
const xmpWithOwner = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:aux="http://ns.adobe.com/exif/1.0/aux/">
      <aux:OwnerName>Existing Owner</aux:OwnerName>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// xmpWithLightroomSettings simulates an XMP with Lightroom develop settings
// and no copyright/owner fields.
const xmpWithLightroomSettings = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/">
      <crs:Exposure2012>+0.50</crs:Exposure2012>
      <crs:Contrast2012>+10</crs:Contrast2012>
      <crs:Highlights2012>-20</crs:Highlights2012>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// writeSidecarFile writes content to a temp file and returns its path.
func writeSidecarFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sidecar %q: %v", path, err)
	}
	return path
}

// readSidecarFile reads and returns the content of a sidecar file.
func readSidecarFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sidecar %q: %v", path, err)
	}
	return string(data)
}

// TestMergeTags_emptyTags verifies that MergeTags is a no-op when tags are empty.
func TestMergeTags_emptyTags(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", minimalXMP)

	before, _ := os.Stat(path)

	if err := MergeTags(path, domain.MetadataTags{}, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	after, _ := os.Stat(path)
	if before.ModTime() != after.ModTime() {
		t.Error("file was modified despite empty tags")
	}
}

// TestMergeTags_injectIntoEmptyDescription verifies that fields are added
// when the rdf:Description has no copyright or owner fields.
func TestMergeTags_injectIntoEmptyDescription(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", minimalXMP)

	tags := domain.MetadataTags{
		Copyright:   "Copyright 2026 Test",
		CameraOwner: "Test Owner",
	}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if !strings.Contains(content, "Copyright 2026 Test") {
		t.Error("copyright not found in merged XMP")
	}
	if !strings.Contains(content, "<xmpRights:Marked>True</xmpRights:Marked>") {
		t.Error("xmpRights:Marked not found in merged XMP")
	}
	if !strings.Contains(content, "Test Owner") {
		t.Error("camera owner not found in merged XMP")
	}
	// Namespace declarations must be present.
	if !strings.Contains(content, `xmlns:dc="http://purl.org/dc/elements/1.1/"`) {
		t.Error("xmlns:dc namespace not added")
	}
	if !strings.Contains(content, `xmlns:xmpRights=`) {
		t.Error("xmlns:xmpRights namespace not added")
	}
	if !strings.Contains(content, `xmlns:aux=`) {
		t.Error("xmlns:aux namespace not added")
	}
	// xpacket wrapper must be preserved.
	if !strings.Contains(content, "<?xpacket") {
		t.Error("<?xpacket?> wrapper was lost")
	}
}

// TestMergeTags_preserveExistingCopyright verifies that when overwrite=false,
// an existing dc:rights value is preserved and Pixe's value is NOT injected.
func TestMergeTags_preserveExistingCopyright(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", xmpWithCopyright)

	tags := domain.MetadataTags{Copyright: "New Copyright 2026"}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if !strings.Contains(content, "Existing Copyright 2020") {
		t.Error("existing copyright was removed (should be preserved)")
	}
	if strings.Contains(content, "New Copyright 2026") {
		t.Error("new copyright was injected despite overwrite=false")
	}
}

// TestMergeTags_overwriteExistingCopyright verifies that when overwrite=true,
// an existing dc:rights value is replaced with Pixe's value.
func TestMergeTags_overwriteExistingCopyright(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", xmpWithCopyright)

	tags := domain.MetadataTags{Copyright: "New Copyright 2026"}
	if err := MergeTags(path, tags, true); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if strings.Contains(content, "Existing Copyright 2020") {
		t.Error("existing copyright was not replaced (overwrite=true)")
	}
	if !strings.Contains(content, "New Copyright 2026") {
		t.Error("new copyright not found after overwrite")
	}
}

// TestMergeTags_preserveExistingOwner verifies that when overwrite=false,
// an existing aux:OwnerName is preserved.
func TestMergeTags_preserveExistingOwner(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", xmpWithOwner)

	tags := domain.MetadataTags{CameraOwner: "New Owner"}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if !strings.Contains(content, "Existing Owner") {
		t.Error("existing owner was removed (should be preserved)")
	}
	if strings.Contains(content, "New Owner") {
		t.Error("new owner was injected despite overwrite=false")
	}
}

// TestMergeTags_overwriteExistingOwner verifies that when overwrite=true,
// an existing aux:OwnerName is replaced.
func TestMergeTags_overwriteExistingOwner(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", xmpWithOwner)

	tags := domain.MetadataTags{CameraOwner: "New Owner"}
	if err := MergeTags(path, tags, true); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if strings.Contains(content, "Existing Owner") {
		t.Error("existing owner was not replaced (overwrite=true)")
	}
	if !strings.Contains(content, "New Owner") {
		t.Error("new owner not found after overwrite")
	}
}

// TestMergeTags_partialOverlap verifies that when overwrite=false and the
// source XMP has dc:rights but not aux:OwnerName, the owner is injected
// while the existing copyright is preserved.
func TestMergeTags_partialOverlap(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", xmpWithCopyright)

	tags := domain.MetadataTags{
		Copyright:   "New Copyright 2026",
		CameraOwner: "New Owner",
	}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	// Copyright preserved (not overwritten).
	if !strings.Contains(content, "Existing Copyright 2020") {
		t.Error("existing copyright was removed")
	}
	if strings.Contains(content, "New Copyright 2026") {
		t.Error("new copyright was injected despite overwrite=false")
	}
	// Owner injected (was missing).
	if !strings.Contains(content, "New Owner") {
		t.Error("new owner was not injected")
	}
}

// TestMergeTags_preservesUnrelatedFields verifies that Lightroom develop
// settings and other unrelated XMP fields are preserved after merge.
func TestMergeTags_preservesUnrelatedFields(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", xmpWithLightroomSettings)

	tags := domain.MetadataTags{Copyright: "Copyright 2026 Test"}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	// Lightroom settings must survive.
	if !strings.Contains(content, "<crs:Exposure2012>+0.50</crs:Exposure2012>") {
		t.Error("Lightroom Exposure2012 setting was lost")
	}
	if !strings.Contains(content, "<crs:Contrast2012>+10</crs:Contrast2012>") {
		t.Error("Lightroom Contrast2012 setting was lost")
	}
	// Copyright injected.
	if !strings.Contains(content, "Copyright 2026 Test") {
		t.Error("copyright not injected")
	}
}

// TestMergeTags_namespaceAddedWhenMissing verifies that a missing namespace
// declaration is added to rdf:Description when a field is injected.
func TestMergeTags_namespaceAddedWhenMissing(t *testing.T) {
	dir := t.TempDir()
	// Source has xmlns:dc but not xmlns:aux.
	src := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:dc="http://purl.org/dc/elements/1.1/"
      xmlns:xmpRights="http://ns.adobe.com/xap/1.0/rights/">
      <dc:rights>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">Existing</rdf:li>
        </rdf:Alt>
      </dc:rights>
      <xmpRights:Marked>True</xmpRights:Marked>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

	path := writeSidecarFile(t, dir, "photo.xmp", src)

	tags := domain.MetadataTags{CameraOwner: "Test Owner"}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if !strings.Contains(content, `xmlns:aux=`) {
		t.Error("xmlns:aux namespace was not added")
	}
	if !strings.Contains(content, "Test Owner") {
		t.Error("owner not injected")
	}
}

// TestMergeTags_atomicWrite verifies that the write is atomic: no .tmp file
// is left behind after a successful merge.
func TestMergeTags_atomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", minimalXMP)

	tags := domain.MetadataTags{Copyright: "Copyright 2026"}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	// No .tmp file should remain.
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf("temp file %q was not cleaned up", tmpPath)
	}
	// The final file must exist.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("final file %q missing after merge: %v", path, err)
	}
}

// TestMergeTags_fileNotFound verifies that MergeTags returns an error when
// the sidecar file does not exist.
func TestMergeTags_fileNotFound(t *testing.T) {
	tags := domain.MetadataTags{Copyright: "Copyright 2026"}
	err := MergeTags("/nonexistent/path/photo.xmp", tags, false)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

// TestMergeTags_xmlEscaping verifies that special XML characters in tag values
// are properly escaped in the output.
func TestMergeTags_xmlEscaping(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", minimalXMP)

	tags := domain.MetadataTags{
		Copyright:   `Copyright 2026 "Wells" & <Family>`,
		CameraOwner: `O'Brien`,
	}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	// Verify XML-escaped values are present.
	if !strings.Contains(content, "Copyright 2026 &quot;Wells&quot; &amp; &lt;Family&gt;") {
		t.Errorf("copyright not properly XML-escaped; content:\n%s", content)
	}
	if !strings.Contains(content, "O&apos;Brien") {
		t.Errorf("owner not properly XML-escaped; content:\n%s", content)
	}
}

// TestMergeTags_onlyCopyright verifies that only copyright fields are added
// when CameraOwner is empty.
func TestMergeTags_onlyCopyright(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", minimalXMP)

	tags := domain.MetadataTags{Copyright: "Copyright 2026"}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if !strings.Contains(content, "Copyright 2026") {
		t.Error("copyright not injected")
	}
	if strings.Contains(content, "aux:OwnerName") {
		t.Error("aux:OwnerName injected despite empty CameraOwner")
	}
}

// TestMergeTags_onlyOwner verifies that only aux:OwnerName is added when
// Copyright is empty.
func TestMergeTags_onlyOwner(t *testing.T) {
	dir := t.TempDir()
	path := writeSidecarFile(t, dir, "photo.xmp", minimalXMP)

	tags := domain.MetadataTags{CameraOwner: "Test Owner"}
	if err := MergeTags(path, tags, false); err != nil {
		t.Fatalf("MergeTags: %v", err)
	}

	content := readSidecarFile(t, path)

	if !strings.Contains(content, "Test Owner") {
		t.Error("owner not injected")
	}
	if strings.Contains(content, "dc:rights") {
		t.Error("dc:rights injected despite empty Copyright")
	}
}
