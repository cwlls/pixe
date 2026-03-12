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
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwlls/pixe-go/internal/domain"
)

// --- SidecarPath tests ---

func TestSidecarPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mediaPath string
		want      string
	}{
		{"/archive/2021/12-Dec/20211225_062223_abc123.arw", "/archive/2021/12-Dec/20211225_062223_abc123.arw.xmp"},
		{"/archive/2021/12-Dec/20211225_062223_abc123.dng", "/archive/2021/12-Dec/20211225_062223_abc123.dng.xmp"},
		{"/archive/2022/01-Jan/20220101_120000_def456.mp4", "/archive/2022/01-Jan/20220101_120000_def456.mp4.xmp"},
		{"/archive/2022/06-Jun/20220615_090000_ghi789.heic", "/archive/2022/06-Jun/20220615_090000_ghi789.heic.xmp"},
		{"photo.jpg", "photo.jpg.xmp"},
	}
	for _, tc := range cases {
		got := SidecarPath(tc.mediaPath)
		if got != tc.want {
			t.Errorf("SidecarPath(%q) = %q, want %q", tc.mediaPath, got, tc.want)
		}
	}
}

// --- WriteSidecar tests ---

func TestWriteSidecar_bothFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	tags := domain.MetadataTags{
		Copyright:   "Copyright 2021 Wells Family, all rights reserved",
		CameraOwner: "Wells Family",
	}
	if err := WriteSidecar(mediaPath, tags); err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}

	sidecarPath := mediaPath + ".xmp"
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("sidecar not created at %q: %v", sidecarPath, err)
	}
	content := string(data)

	// Packet wrapper.
	if !strings.Contains(content, `<?xpacket begin=`) {
		t.Error("missing xpacket begin header")
	}
	if !strings.Contains(content, `<?xpacket end="w"?>`) {
		t.Error("missing xpacket end footer")
	}

	// Copyright fields.
	if !strings.Contains(content, "Copyright 2021 Wells Family, all rights reserved") {
		t.Errorf("missing copyright text; content:\n%s", content)
	}
	if !strings.Contains(content, "dc:rights") {
		t.Error("missing dc:rights element")
	}
	if !strings.Contains(content, "xmpRights:Marked") {
		t.Error("missing xmpRights:Marked element")
	}
	if !strings.Contains(content, "True") {
		t.Error("xmpRights:Marked should be True")
	}

	// CameraOwner field.
	if !strings.Contains(content, "aux:OwnerName") {
		t.Error("missing aux:OwnerName element")
	}
	if !strings.Contains(content, "Wells Family") {
		t.Errorf("missing camera owner text; content:\n%s", content)
	}

	// Valid XML (strip the xpacket PIs which are not valid XML on their own).
	xmlBody := extractRDFBody(content)
	if err := xml.Unmarshal([]byte(xmlBody), new(interface{})); err != nil {
		t.Errorf("XMP body is not valid XML: %v\nbody:\n%s", err, xmlBody)
	}
}

func TestWriteSidecar_copyrightOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.dng")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	tags := domain.MetadataTags{Copyright: "Copyright 2022 Test"}
	if err := WriteSidecar(mediaPath, tags); err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}

	data, err := os.ReadFile(mediaPath + ".xmp")
	if err != nil {
		t.Fatalf("sidecar not created: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "dc:rights") {
		t.Error("sidecar should contain dc:rights when Copyright is set")
	}
	if !strings.Contains(content, "xmpRights:Marked") {
		t.Error("sidecar should contain xmpRights:Marked when Copyright is set")
	}
	if strings.Contains(content, "aux:OwnerName") {
		t.Error("sidecar should NOT contain aux:OwnerName when CameraOwner is empty")
	}
	// aux namespace should be absent.
	if strings.Contains(content, `xmlns:aux=`) {
		t.Error("sidecar should NOT declare aux namespace when CameraOwner is empty")
	}
}

func TestWriteSidecar_cameraOwnerOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.heic")
	if err := os.WriteFile(mediaPath, []byte("fake heic"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	tags := domain.MetadataTags{CameraOwner: "My Camera"}
	if err := WriteSidecar(mediaPath, tags); err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}

	data, err := os.ReadFile(mediaPath + ".xmp")
	if err != nil {
		t.Fatalf("sidecar not created: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "dc:rights") {
		t.Error("sidecar should NOT contain dc:rights when Copyright is empty")
	}
	if strings.Contains(content, "xmpRights:Marked") {
		t.Error("sidecar should NOT contain xmpRights:Marked when Copyright is empty")
	}
	if !strings.Contains(content, "aux:OwnerName") {
		t.Error("sidecar should contain aux:OwnerName when CameraOwner is set")
	}
	if !strings.Contains(content, "My Camera") {
		t.Errorf("sidecar missing camera owner text; content:\n%s", content)
	}
	// dc/xmpRights namespaces should be absent.
	if strings.Contains(content, `xmlns:dc=`) {
		t.Error("sidecar should NOT declare dc namespace when Copyright is empty")
	}
}

func TestWriteSidecar_emptyTags_noFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.mp4")
	if err := os.WriteFile(mediaPath, []byte("fake mp4"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	tags := domain.MetadataTags{} // both empty
	if err := WriteSidecar(mediaPath, tags); err != nil {
		t.Fatalf("WriteSidecar with empty tags: %v", err)
	}

	sidecarPath := mediaPath + ".xmp"
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Errorf("WriteSidecar should not create a file when tags are empty, but %q exists", sidecarPath)
	}
}

func TestWriteSidecar_atomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "photo.arw")
	if err := os.WriteFile(mediaPath, []byte("fake raw"), 0o644); err != nil {
		t.Fatalf("create media file: %v", err)
	}

	tags := domain.MetadataTags{Copyright: "Copyright 2021 Test"}
	if err := WriteSidecar(mediaPath, tags); err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}

	// No .tmp file should remain after a successful write.
	tmpPath := mediaPath + ".xmp.tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file %q should not exist after successful write", tmpPath)
	}

	// The final sidecar must exist.
	if _, err := os.Stat(mediaPath + ".xmp"); err != nil {
		t.Errorf("sidecar file should exist after write: %v", err)
	}
}

func TestWriteSidecar_errorOnBadPath(t *testing.T) {
	t.Parallel()
	// Use a path in a non-existent directory to force a write error.
	mediaPath := filepath.Join(t.TempDir(), "nonexistent", "photo.arw")

	tags := domain.MetadataTags{Copyright: "Copyright 2021 Test"}
	err := WriteSidecar(mediaPath, tags)
	if err == nil {
		t.Fatal("WriteSidecar should return error when directory does not exist")
	}
	if !strings.HasPrefix(err.Error(), "xmp:") {
		t.Errorf("error should have 'xmp:' prefix, got: %v", err)
	}
}

// extractRDFBody strips the xpacket processing instructions and returns
// the inner XML body (x:xmpmeta element) for XML validation.
func extractRDFBody(content string) string {
	// Find the start of <x:xmpmeta and the end of </x:xmpmeta>.
	start := strings.Index(content, "<x:xmpmeta")
	end := strings.Index(content, "</x:xmpmeta>")
	if start < 0 || end < 0 {
		return content
	}
	return content[start : end+len("</x:xmpmeta>")]
}
