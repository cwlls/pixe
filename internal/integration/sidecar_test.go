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

package integration

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cwlls/pixe/internal/config"
	"github.com/cwlls/pixe/internal/discovery"
	"github.com/cwlls/pixe/internal/domain"
	arwhandler "github.com/cwlls/pixe/internal/handler/arw"
	jpeghandler "github.com/cwlls/pixe/internal/handler/jpeg"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/pipeline"
)

const (
	testCopyright   = "Copyright {year} Wells Family, all rights reserved"
	testCameraOwner = "Wells Family"
)

// buildOptsWithTags constructs SortOptions with copyright and camera-owner set.
func buildOptsWithTags(t *testing.T, dirA, dirB string, handlers ...domain.FileTypeHandler) pipeline.SortOptions {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	for _, handler := range handlers {
		reg.Register(handler)
	}
	ct, err := pathbuilder.ParseCopyrightTemplate(testCopyright)
	if err != nil {
		t.Fatalf("ParseCopyrightTemplate: %v", err)
	}
	return pipeline.SortOptions{
		Config: &config.AppConfig{
			Source:            dirA,
			Destination:       dirB,
			Algorithm:         "sha1",
			Copyright:         testCopyright,
			CopyrightTemplate: ct,
			CameraOwner:       testCameraOwner,
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &bytes.Buffer{},
		PixeVersion:  "test",
	}
}

// TestSidecar_RAW_getsSidecar_JPEG_getsSidecar verifies the end-to-end tagging
// routing: both RAW and JPEG files receive an XMP sidecar (no handler embeds
// metadata directly into the destination file).
func TestSidecar_RAW_getsSidecar_JPEG_getsSidecar(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Place a JPEG fixture and a synthetic ARW in dirA.
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	buildFakeARW(t, dirA, "RAW_0001.arw")

	opts := buildOptsWithTags(t, dirA, dirB,
		jpeghandler.New(),
		arwhandler.New(),
	)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}
	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}

	// --- Locate destination files ---
	var jpegDest, rawDest string
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg":
			jpegDest = path
		case ".arw":
			rawDest = path
		}
		return nil
	})

	if jpegDest == "" {
		t.Fatal("no JPEG destination file found in dirB")
	}
	if rawDest == "" {
		t.Fatal("no ARW destination file found in dirB")
	}

	// --- JPEG: metadata expressed via XMP sidecar (no EXIF embedding) ---
	t.Run("JPEG_has_XMP_sidecar", func(t *testing.T) {
		sidecarPath := jpegDest + ".xmp"
		data, err := os.ReadFile(sidecarPath)
		if err != nil {
			t.Fatalf("JPEG XMP sidecar not found at %q: %v", sidecarPath, err)
		}
		content := string(data)

		if !strings.Contains(content, "Wells Family") {
			t.Errorf("JPEG sidecar missing 'Wells Family'; content:\n%s", content)
		}
		if !strings.Contains(content, "dc:rights") {
			t.Error("JPEG sidecar missing dc:rights element")
		}
	})

	t.Run("JPEG_sidecar_follows_Adobe_naming", func(t *testing.T) {
		// Adobe convention: <filename>.<ext>.xmp
		sidecarPath := jpegDest + ".xmp"
		if !strings.HasSuffix(sidecarPath, ".jpg.xmp") {
			t.Errorf("sidecar path %q should end with .jpg.xmp (Adobe convention)", sidecarPath)
		}
	})

	// --- RAW (ARW): XMP sidecar must exist, source file must be unmodified ---
	t.Run("RAW_has_XMP_sidecar", func(t *testing.T) {
		sidecarPath := rawDest + ".xmp"
		data, err := os.ReadFile(sidecarPath)
		if err != nil {
			t.Fatalf("RAW XMP sidecar not found at %q: %v", sidecarPath, err)
		}
		content := string(data)

		if !strings.Contains(content, "Wells Family") {
			t.Errorf("RAW sidecar missing 'Wells Family'; content:\n%s", content)
		}
		if !strings.Contains(content, "dc:rights") {
			t.Error("RAW sidecar missing dc:rights element")
		}
		if !strings.Contains(content, "aux:OwnerName") {
			t.Error("RAW sidecar missing aux:OwnerName element")
		}
		if !strings.Contains(content, `<?xpacket begin=`) {
			t.Error("RAW sidecar missing xpacket header")
		}
		if !strings.Contains(content, `<?xpacket end="w"?>`) {
			t.Error("RAW sidecar missing xpacket footer")
		}
	})

	t.Run("RAW_sidecar_follows_Adobe_naming", func(t *testing.T) {
		// Adobe convention: <filename>.<ext>.xmp (not <filename>.xmp)
		sidecarPath := rawDest + ".xmp"
		if !strings.HasSuffix(sidecarPath, ".arw.xmp") {
			t.Errorf("sidecar path %q should end with .arw.xmp (Adobe convention)", sidecarPath)
		}
	})
}

// TestSidecar_noTagsConfigured_noSidecarWritten verifies that when no
// copyright or camera-owner is configured, no XMP sidecar is created.
func TestSidecar_noTagsConfigured_noSidecarWritten(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeARW(t, dirA, "RAW_0001.arw")

	// Build opts WITHOUT copyright/camera-owner.
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(arwhandler.New())
	opts := pipeline.SortOptions{
		Config: &config.AppConfig{
			Source:      dirA,
			Destination: dirB,
			Algorithm:   "sha1",
			// No Copyright or CameraOwner.
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &bytes.Buffer{},
		PixeVersion:  "test",
	}

	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Walk dirB — no .xmp files should exist.
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		if strings.HasSuffix(path, ".xmp") {
			t.Errorf("unexpected XMP sidecar created when no tags configured: %q", path)
		}
		return nil
	})
}
