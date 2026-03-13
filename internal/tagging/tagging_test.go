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

package tagging

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/pathbuilder"
)

// --- RenderCopyright tests ---

// mustParseCopyrightTemplate is a test helper that parses a copyright template
// and fatals if parsing fails.
func mustParseCopyrightTemplate(t *testing.T, raw string) *pathbuilder.CopyrightTemplate {
	t.Helper()
	tmpl, err := pathbuilder.ParseCopyrightTemplate(raw)
	if err != nil {
		t.Fatalf("ParseCopyrightTemplate(%q): %v", raw, err)
	}
	return tmpl
}

func TestRenderCopyright_yearSubstitution(t *testing.T) {
	cases := []struct {
		tmpl string
		year int
		want string
	}{
		{"Copyright {year} My Family", 2021, "Copyright 2021 My Family"},
		{"Copyright {year} My Family", 2026, "Copyright 2026 My Family"},
		{"Copyright {year} My Family, all rights reserved", 1902, "Copyright 1902 My Family, all rights reserved"},
	}
	for _, tc := range cases {
		ct := mustParseCopyrightTemplate(t, tc.tmpl)
		date := time.Date(tc.year, 1, 1, 0, 0, 0, 0, time.UTC)
		got := RenderCopyright(ct, date)
		if got != tc.want {
			t.Errorf("RenderCopyright(%q, %d) = %q, want %q", tc.tmpl, tc.year, got, tc.want)
		}
	}
}

func TestRenderCopyright_noTokens(t *testing.T) {
	ct := mustParseCopyrightTemplate(t, "No tokens here")
	date := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright(ct, date)
	if got != "No tokens here" {
		t.Errorf("RenderCopyright with no tokens = %q, want %q", got, "No tokens here")
	}
}

func TestRenderCopyright_nilTemplate(t *testing.T) {
	date := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright(nil, date)
	if got != "" {
		t.Errorf("RenderCopyright(nil) = %q, want empty string", got)
	}
}

func TestRenderCopyright_malformedTemplate_parseError(t *testing.T) {
	// Unclosed brace — ParseCopyrightTemplate must return an error.
	_, err := pathbuilder.ParseCopyrightTemplate("Copyright {year My Family")
	if err == nil {
		t.Fatal("expected error for unclosed brace in copyright template")
	}
}

func TestRenderCopyright_multipleYearReferences(t *testing.T) {
	ct := mustParseCopyrightTemplate(t, "© {year}-{year} My Family")
	date := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright(ct, date)
	want := "© 2024-2024 My Family"
	if got != want {
		t.Errorf("RenderCopyright multiple refs = %q, want %q", got, want)
	}
}

func TestRenderCopyright_multipleTokens(t *testing.T) {
	ct := mustParseCopyrightTemplate(t, "© {year}-{month} My Family")
	date := time.Date(2021, 12, 25, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright(ct, date)
	want := "© 2021-12 My Family"
	if got != want {
		t.Errorf("RenderCopyright multiple tokens = %q, want %q", got, want)
	}
}

// --- Apply tests ---

// mockHandler records calls to WriteMetadataTags and returns a configurable
// MetadataCapability so all three dispatch branches can be exercised.
type mockHandler struct {
	writeCalled     bool
	writeTags       domain.MetadataTags
	writeErr        error
	metadataSupport domain.MetadataCapability
}

func (m *mockHandler) Extensions() []string                         { return nil }
func (m *mockHandler) MagicBytes() []domain.MagicSignature          { return nil }
func (m *mockHandler) Detect(string) (bool, error)                  { return true, nil }
func (m *mockHandler) ExtractDate(string) (time.Time, error)        { return time.Time{}, nil }
func (m *mockHandler) HashableReader(string) (io.ReadCloser, error) { return nil, nil }
func (m *mockHandler) MetadataSupport() domain.MetadataCapability   { return m.metadataSupport }
func (m *mockHandler) WriteMetadataTags(path string, tags domain.MetadataTags) error {
	m.writeCalled = true
	m.writeTags = tags
	return m.writeErr
}

func TestApply_noop_whenEmpty(t *testing.T) {
	h := &mockHandler{metadataSupport: domain.MetadataEmbed}
	err := Apply("/some/file.jpg", h, domain.MetadataTags{})
	if err != nil {
		t.Errorf("Apply with empty tags should return nil, got: %v", err)
	}
	if h.writeCalled {
		t.Error("Apply should not call WriteMetadataTags when tags are empty")
	}
}

func TestApply_embed_callsWriteMetadataTags(t *testing.T) {
	h := &mockHandler{metadataSupport: domain.MetadataEmbed}
	tags := domain.MetadataTags{Copyright: "Copyright 2021 Test", CameraOwner: "Test Owner"}
	err := Apply("/some/file.jpg", h, tags)
	if err != nil {
		t.Errorf("Apply: unexpected error: %v", err)
	}
	if !h.writeCalled {
		t.Error("Apply should call WriteMetadataTags for MetadataEmbed handler")
	}
	if h.writeTags.Copyright != tags.Copyright {
		t.Errorf("Copyright passed = %q, want %q", h.writeTags.Copyright, tags.Copyright)
	}
	if h.writeTags.CameraOwner != tags.CameraOwner {
		t.Errorf("CameraOwner passed = %q, want %q", h.writeTags.CameraOwner, tags.CameraOwner)
	}
}

func TestApply_embed_propagatesError(t *testing.T) {
	h := &mockHandler{metadataSupport: domain.MetadataEmbed, writeErr: errors.New("write failed")}
	tags := domain.MetadataTags{Copyright: "Copyright 2021"}
	err := Apply("/some/file.jpg", h, tags)
	if err == nil {
		t.Error("Apply should propagate WriteMetadataTags error")
	}
	if !strings.Contains(err.Error(), "tagging: embed metadata") {
		t.Errorf("error should contain 'tagging: embed metadata', got: %v", err)
	}
}

func TestApply_sidecar_writesXMPFile(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	tags := domain.MetadataTags{Copyright: "Copyright 2021 Test", CameraOwner: "Test Owner"}
	if err := Apply(mediaPath, h, tags); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	sidecarPath := mediaPath + ".xmp"
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("sidecar not created at %q: %v", sidecarPath, err)
	}
	content := string(data)
	if !strings.Contains(content, "Copyright 2021 Test") {
		t.Errorf("sidecar missing copyright text; content:\n%s", content)
	}
	if !strings.Contains(content, "Test Owner") {
		t.Errorf("sidecar missing camera owner; content:\n%s", content)
	}
	if h.writeCalled {
		t.Error("Apply should NOT call WriteMetadataTags for MetadataSidecar handler")
	}
}

func TestApply_sidecar_copyrightOnly(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	tags := domain.MetadataTags{Copyright: "Copyright 2021 Test"}
	if err := Apply(mediaPath, h, tags); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(mediaPath + ".xmp")
	if err != nil {
		t.Fatalf("sidecar not created: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "aux:OwnerName") {
		t.Error("sidecar should not contain aux:OwnerName when CameraOwner is empty")
	}
	if !strings.Contains(content, "dc:rights") {
		t.Error("sidecar should contain dc:rights when Copyright is set")
	}
}

func TestApply_sidecar_cameraOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	tags := domain.MetadataTags{CameraOwner: "Wells Family"}
	if err := Apply(mediaPath, h, tags); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(mediaPath + ".xmp")
	if err != nil {
		t.Fatalf("sidecar not created: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "dc:rights") {
		t.Error("sidecar should not contain dc:rights when Copyright is empty")
	}
	if strings.Contains(content, "xmpRights:Marked") {
		t.Error("sidecar should not contain xmpRights:Marked when Copyright is empty")
	}
	if !strings.Contains(content, "aux:OwnerName") {
		t.Error("sidecar should contain aux:OwnerName when CameraOwner is set")
	}
}

func TestApply_none_skipsEverything(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.raw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataNone}
	tags := domain.MetadataTags{Copyright: "Copyright 2021", CameraOwner: "Owner"}
	if err := Apply(mediaPath, h, tags); err != nil {
		t.Fatalf("Apply: unexpected error: %v", err)
	}
	if h.writeCalled {
		t.Error("Apply should not call WriteMetadataTags for MetadataNone handler")
	}
	if _, err := os.Stat(mediaPath + ".xmp"); !os.IsNotExist(err) {
		t.Error("Apply should not create XMP sidecar for MetadataNone handler")
	}
}

func TestApply_sidecar_errorPropagation(t *testing.T) {
	// Use a path in a non-existent directory to force a write error.
	mediaPath := filepath.Join(t.TempDir(), "nonexistent", "photo.arw")

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	tags := domain.MetadataTags{Copyright: "Copyright 2021"}
	err := Apply(mediaPath, h, tags)
	if err == nil {
		t.Fatal("Apply should return error when sidecar cannot be written")
	}
	if !strings.Contains(err.Error(), "tagging: write sidecar") {
		t.Errorf("error should contain 'tagging: write sidecar', got: %v", err)
	}
}

func TestApply_onlyCameraOwner(t *testing.T) {
	h := &mockHandler{metadataSupport: domain.MetadataEmbed}
	tags := domain.MetadataTags{CameraOwner: "Wells Family"}
	err := Apply("/some/file.jpg", h, tags)
	if err != nil {
		t.Errorf("Apply with CameraOwner only: %v", err)
	}
	if !h.writeCalled {
		t.Error("Apply should call WriteMetadataTags when CameraOwner is set")
	}
}

// --- ApplyWithSidecars tests ---

// minimalXMPForTagging is a bare-bones XMP sidecar used in tagging tests.
const minimalXMPForTagging = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="">
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// xmpWithExistingCopyrightForTagging has an existing dc:rights value.
const xmpWithExistingCopyrightForTagging = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
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

// TestApplyWithSidecars_noCarriedXMP_sidecarHandler verifies that when no
// carried XMP is provided and the handler is MetadataSidecar, a new XMP
// sidecar is generated (existing behaviour).
func TestApplyWithSidecars_noCarriedXMP_sidecarHandler(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	tags := domain.MetadataTags{Copyright: "Copyright 2026 Test"}

	if err := ApplyWithSidecars(mediaPath, h, tags, "", false); err != nil {
		t.Fatalf("ApplyWithSidecars: %v", err)
	}

	// A new XMP sidecar must be created at mediaPath + ".xmp".
	sidecarPath := mediaPath + ".xmp"
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("sidecar not created: %v", err)
	}
	if !strings.Contains(string(data), "Copyright 2026 Test") {
		t.Error("copyright not found in generated sidecar")
	}
	if h.writeCalled {
		t.Error("WriteMetadataTags should not be called for MetadataSidecar handler")
	}
}

// TestApplyWithSidecars_carriedXMP_sidecarHandler verifies that when a carried
// XMP path is provided and the handler is MetadataSidecar, MergeTags is called
// on the carried sidecar instead of generating a new one.
func TestApplyWithSidecars_carriedXMP_sidecarHandler(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	// Write a minimal XMP sidecar as the "carried" file.
	carriedXMPPath := filepath.Join(dir, "photo.arw.xmp")
	if err := os.WriteFile(carriedXMPPath, []byte(minimalXMPForTagging), 0o644); err != nil {
		t.Fatalf("write carried XMP: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	tags := domain.MetadataTags{Copyright: "Copyright 2026 Merged"}

	if err := ApplyWithSidecars(mediaPath, h, tags, carriedXMPPath, false); err != nil {
		t.Fatalf("ApplyWithSidecars: %v", err)
	}

	// The carried XMP must be updated in-place (merged).
	data, err := os.ReadFile(carriedXMPPath)
	if err != nil {
		t.Fatalf("read carried XMP: %v", err)
	}
	if !strings.Contains(string(data), "Copyright 2026 Merged") {
		t.Error("copyright not merged into carried XMP")
	}
	if h.writeCalled {
		t.Error("WriteMetadataTags should not be called for MetadataSidecar handler")
	}
}

// TestApplyWithSidecars_carriedXMP_embedHandler verifies that when the handler
// is MetadataEmbed, the carried XMP is ignored and WriteMetadataTags is called.
func TestApplyWithSidecars_carriedXMP_embedHandler(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(mediaPath, []byte("fake jpeg"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	// A carried XMP path (would exist in a real scenario).
	carriedXMPPath := filepath.Join(dir, "photo.jpg.xmp")

	h := &mockHandler{metadataSupport: domain.MetadataEmbed}
	tags := domain.MetadataTags{Copyright: "Copyright 2026"}

	if err := ApplyWithSidecars(mediaPath, h, tags, carriedXMPPath, false); err != nil {
		t.Fatalf("ApplyWithSidecars: %v", err)
	}

	// WriteMetadataTags must be called (embed path).
	if !h.writeCalled {
		t.Error("WriteMetadataTags should be called for MetadataEmbed handler")
	}
	// The carried XMP must NOT be created/modified (embed handler ignores it).
	if _, err := os.Stat(carriedXMPPath); err == nil {
		t.Error("carried XMP should not be created/modified for MetadataEmbed handler")
	}
}

// TestApplyWithSidecars_emptyTags_noOp verifies that empty tags result in no
// action even when a carried XMP path is provided.
func TestApplyWithSidecars_emptyTags_noOp(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	carriedXMPPath := filepath.Join(dir, "photo.arw.xmp")
	if err := os.WriteFile(carriedXMPPath, []byte(minimalXMPForTagging), 0o644); err != nil {
		t.Fatalf("write carried XMP: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	before, _ := os.Stat(carriedXMPPath)

	if err := ApplyWithSidecars(mediaPath, h, domain.MetadataTags{}, carriedXMPPath, false); err != nil {
		t.Fatalf("ApplyWithSidecars: %v", err)
	}

	after, _ := os.Stat(carriedXMPPath)
	if before.ModTime() != after.ModTime() {
		t.Error("carried XMP was modified despite empty tags")
	}
}

// TestApplyWithSidecars_overwriteFlagPropagated verifies that the overwrite
// flag is correctly passed through to MergeTags. When overwrite=true, an
// existing copyright in the carried XMP is replaced.
func TestApplyWithSidecars_overwriteFlagPropagated(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	carriedXMPPath := filepath.Join(dir, "photo.arw.xmp")
	if err := os.WriteFile(carriedXMPPath, []byte(xmpWithExistingCopyrightForTagging), 0o644); err != nil {
		t.Fatalf("write carried XMP: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataSidecar}
	tags := domain.MetadataTags{Copyright: "New Copyright 2026"}

	// overwrite=true → existing copyright should be replaced.
	if err := ApplyWithSidecars(mediaPath, h, tags, carriedXMPPath, true); err != nil {
		t.Fatalf("ApplyWithSidecars: %v", err)
	}

	data, err := os.ReadFile(carriedXMPPath)
	if err != nil {
		t.Fatalf("read carried XMP: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "Existing Copyright 2020") {
		t.Error("existing copyright was not replaced (overwrite=true)")
	}
	if !strings.Contains(content, "New Copyright 2026") {
		t.Error("new copyright not found after overwrite")
	}
}

// TestApplyWithSidecars_none_skipsEverything verifies that MetadataNone
// handlers result in no action regardless of carried XMP.
func TestApplyWithSidecars_none_skipsEverything(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.raw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	carriedXMPPath := filepath.Join(dir, "photo.raw.xmp")
	if err := os.WriteFile(carriedXMPPath, []byte(minimalXMPForTagging), 0o644); err != nil {
		t.Fatalf("write carried XMP: %v", err)
	}

	h := &mockHandler{metadataSupport: domain.MetadataNone}
	tags := domain.MetadataTags{Copyright: "Copyright 2026"}
	before, _ := os.Stat(carriedXMPPath)

	if err := ApplyWithSidecars(mediaPath, h, tags, carriedXMPPath, false); err != nil {
		t.Fatalf("ApplyWithSidecars: %v", err)
	}

	if h.writeCalled {
		t.Error("WriteMetadataTags should not be called for MetadataNone handler")
	}
	after, _ := os.Stat(carriedXMPPath)
	if before.ModTime() != after.ModTime() {
		t.Error("carried XMP was modified for MetadataNone handler")
	}
}
