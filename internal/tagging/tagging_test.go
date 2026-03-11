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

	"github.com/cwlls/pixe-go/internal/domain"
)

// --- RenderCopyright tests ---

func TestRenderCopyright_yearSubstitution(t *testing.T) {
	cases := []struct {
		tmpl string
		year int
		want string
	}{
		{"Copyright {{.Year}} My Family", 2021, "Copyright 2021 My Family"},
		{"Copyright {{.Year}} My Family", 2026, "Copyright 2026 My Family"},
		{"Copyright {{.Year}} My Family, all rights reserved", 1902, "Copyright 1902 My Family, all rights reserved"},
	}
	for _, tc := range cases {
		date := time.Date(tc.year, 1, 1, 0, 0, 0, 0, time.UTC)
		got := RenderCopyright(tc.tmpl, date)
		if got != tc.want {
			t.Errorf("RenderCopyright(%q, %d) = %q, want %q", tc.tmpl, tc.year, got, tc.want)
		}
	}
}

func TestRenderCopyright_noTemplate(t *testing.T) {
	date := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright("No template here", date)
	if got != "No template here" {
		t.Errorf("RenderCopyright with no template = %q, want %q", got, "No template here")
	}
}

func TestRenderCopyright_emptyString(t *testing.T) {
	date := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright("", date)
	if got != "" {
		t.Errorf("RenderCopyright('') = %q, want empty string", got)
	}
}

func TestRenderCopyright_malformedTemplate_returnsRaw(t *testing.T) {
	// Unclosed action — should return the raw string, not panic.
	raw := "Copyright {{.Year My Family"
	date := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright(raw, date)
	if got != raw {
		t.Errorf("malformed template: got %q, want raw %q", got, raw)
	}
}

func TestRenderCopyright_multipleYearReferences(t *testing.T) {
	tmpl := "© {{.Year}}-{{.Year}} My Family"
	date := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	got := RenderCopyright(tmpl, date)
	want := "© 2024-2024 My Family"
	if got != want {
		t.Errorf("RenderCopyright multiple refs = %q, want %q", got, want)
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
