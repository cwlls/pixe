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

// mockHandler records calls to WriteMetadataTags.
type mockHandler struct {
	writeCalled bool
	writeTags   domain.MetadataTags
	writeErr    error
}

func (m *mockHandler) Extensions() []string                         { return nil }
func (m *mockHandler) MagicBytes() []domain.MagicSignature          { return nil }
func (m *mockHandler) Detect(string) (bool, error)                  { return true, nil }
func (m *mockHandler) ExtractDate(string) (time.Time, error)        { return time.Time{}, nil }
func (m *mockHandler) HashableReader(string) (io.ReadCloser, error) { return nil, nil }
func (m *mockHandler) WriteMetadataTags(path string, tags domain.MetadataTags) error {
	m.writeCalled = true
	m.writeTags = tags
	return m.writeErr
}

func TestApply_noop_whenEmpty(t *testing.T) {
	h := &mockHandler{}
	err := Apply("/some/file.jpg", h, domain.MetadataTags{})
	if err != nil {
		t.Errorf("Apply with empty tags should return nil, got: %v", err)
	}
	if h.writeCalled {
		t.Error("Apply should not call WriteMetadataTags when tags are empty")
	}
}

func TestApply_callsHandler(t *testing.T) {
	h := &mockHandler{}
	tags := domain.MetadataTags{Copyright: "Copyright 2021 Test", CameraOwner: "Test Owner"}
	err := Apply("/some/file.jpg", h, tags)
	if err != nil {
		t.Errorf("Apply: unexpected error: %v", err)
	}
	if !h.writeCalled {
		t.Error("Apply should call WriteMetadataTags when tags are non-empty")
	}
	if h.writeTags.Copyright != tags.Copyright {
		t.Errorf("Copyright passed = %q, want %q", h.writeTags.Copyright, tags.Copyright)
	}
	if h.writeTags.CameraOwner != tags.CameraOwner {
		t.Errorf("CameraOwner passed = %q, want %q", h.writeTags.CameraOwner, tags.CameraOwner)
	}
}

func TestApply_propagatesError(t *testing.T) {
	h := &mockHandler{writeErr: errors.New("write failed")}
	tags := domain.MetadataTags{Copyright: "Copyright 2021"}
	err := Apply("/some/file.jpg", h, tags)
	if err == nil {
		t.Error("Apply should propagate WriteMetadataTags error")
	}
}

func TestApply_onlyCameraOwner(t *testing.T) {
	h := &mockHandler{}
	tags := domain.MetadataTags{CameraOwner: "Wells Family"}
	err := Apply("/some/file.jpg", h, tags)
	if err != nil {
		t.Errorf("Apply with CameraOwner only: %v", err)
	}
	if !h.writeCalled {
		t.Error("Apply should call WriteMetadataTags when CameraOwner is set")
	}
}
