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
	arwhandler "github.com/cwlls/pixe/internal/handler/arw"
	jpeghandler "github.com/cwlls/pixe/internal/handler/jpeg"
	"github.com/cwlls/pixe/internal/hash"
	"github.com/cwlls/pixe/internal/pathbuilder"
	"github.com/cwlls/pixe/internal/pipeline"
)

// --- helpers for sidecar carry tests ---

// minimalAAE is a minimal Apple Adjustment Envelope XML file.
const minimalAAE = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>adjustmentFormatVersion</key>
  <integer>1</integer>
</dict>
</plist>`

// minimalXMPSidecar is a bare-bones XMP sidecar with no copyright/owner fields.
const minimalXMPSidecar = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="">
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// xmpWithExistingCopyright is an XMP sidecar with a pre-existing dc:rights value.
const xmpWithExistingCopyright = `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
      xmlns:dc="http://purl.org/dc/elements/1.1/"
      xmlns:xmpRights="http://ns.adobe.com/xap/1.0/rights/"
      xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/">
      <dc:rights>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">Existing Copyright 2020</rdf:li>
        </rdf:Alt>
      </dc:rights>
      <xmpRights:Marked>True</xmpRights:Marked>
      <crs:Exposure2012>+0.50</crs:Exposure2012>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// buildCarrySidecarOpts constructs SortOptions with CarrySidecars enabled.
func buildCarrySidecarOpts(t *testing.T, dirA, dirB string, cfg *config.AppConfig) pipeline.SortOptions {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	reg.Register(arwhandler.New())

	cfg.Source = dirA
	cfg.Destination = dirB
	cfg.Algorithm = "sha1"
	cfg.CarrySidecars = true

	if cfg.Copyright != "" {
		ct, err := pathbuilder.ParseCopyrightTemplate(cfg.Copyright)
		if err != nil {
			t.Fatalf("ParseCopyrightTemplate: %v", err)
		}
		cfg.CopyrightTemplate = ct
	}

	return pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &bytes.Buffer{},
		PixeVersion:  "test",
	}
}

// writeSidecar writes content to a file at dir/name.
func writeSidecar(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sidecar %q: %v", path, err)
	}
	return path
}

// findFilesByExt returns all files under root with the given extension (lowercase).
func findFilesByExt(t *testing.T, root, ext string) []string {
	t.Helper()
	var found []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.ToLower(filepath.Ext(path)) == ext {
			found = append(found, path)
		}
		return nil
	})
	return found
}

// --- Integration tests ---

// TestCarrySidecars_basicAAE verifies that a .aae sidecar is carried alongside
// its parent JPEG to dirB, renamed to match the destination filename.
func TestCarrySidecars_basicAAE(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	writeSidecar(t, dirA, "IMG_0001.aae", minimalAAE)

	opts := buildCarrySidecarOpts(t, dirA, dirB, &config.AppConfig{})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1", result.Processed)
	}

	// Find the destination JPEG.
	jpegs := findFilesByExt(t, dirB, ".jpg")
	if len(jpegs) != 1 {
		t.Fatalf("expected 1 JPEG in dirB, got %d", len(jpegs))
	}
	jpegDest := jpegs[0]

	// The .aae sidecar must exist at <jpegDest>.aae.
	aaeDest := jpegDest + ".aae"
	if _, err := os.Stat(aaeDest); err != nil {
		t.Errorf(".aae sidecar not found at %q: %v", aaeDest, err)
	}
}

// TestCarrySidecars_xmpCarriedVerbatim verifies that when no tags are
// configured, a .xmp sidecar is carried verbatim (content unchanged).
func TestCarrySidecars_xmpCarriedVerbatim(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeARW(t, dirA, "RAW_0001.arw")
	writeSidecar(t, dirA, "RAW_0001.xmp", minimalXMPSidecar)

	// No copyright/camera-owner configured.
	opts := buildCarrySidecarOpts(t, dirA, dirB, &config.AppConfig{})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}

	// Find the destination ARW.
	arws := findFilesByExt(t, dirB, ".arw")
	if len(arws) != 1 {
		t.Fatalf("expected 1 ARW in dirB, got %d", len(arws))
	}
	xmpDest := arws[0] + ".xmp"
	data, err := os.ReadFile(xmpDest)
	if err != nil {
		t.Fatalf(".xmp sidecar not found at %q: %v", xmpDest, err)
	}
	// Content must be the original (no tags injected).
	if !strings.Contains(string(data), "<?xpacket") {
		t.Error("carried XMP missing xpacket header")
	}
	// No copyright should have been injected.
	if strings.Contains(string(data), "dc:rights") {
		t.Error("dc:rights was injected despite no copyright configured")
	}
}

// TestCarrySidecars_xmpWithTagMerge verifies that when copyright is configured
// and a .xmp sidecar is carried, the tags are merged into the carried sidecar.
func TestCarrySidecars_xmpWithTagMerge(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeARW(t, dirA, "RAW_0001.arw")
	writeSidecar(t, dirA, "RAW_0001.xmp", minimalXMPSidecar)

	opts := buildCarrySidecarOpts(t, dirA, dirB, &config.AppConfig{
		Copyright:   "Copyright 2026 Test",
		CameraOwner: "Test Owner",
	})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}

	arws := findFilesByExt(t, dirB, ".arw")
	if len(arws) != 1 {
		t.Fatalf("expected 1 ARW in dirB, got %d", len(arws))
	}
	xmpDest := arws[0] + ".xmp"
	data, err := os.ReadFile(xmpDest)
	if err != nil {
		t.Fatalf(".xmp sidecar not found at %q: %v", xmpDest, err)
	}
	content := string(data)

	// Tags must be merged into the carried sidecar.
	if !strings.Contains(content, "Copyright 2026 Test") {
		t.Error("copyright not merged into carried XMP")
	}
	if !strings.Contains(content, "Test Owner") {
		t.Error("camera owner not merged into carried XMP")
	}
	// xpacket wrapper must be preserved.
	if !strings.Contains(content, "<?xpacket") {
		t.Error("xpacket header lost after merge")
	}
}

// TestCarrySidecars_preserveExistingCopyright verifies that when overwrite is
// false (default), an existing copyright in the carried .xmp is preserved.
func TestCarrySidecars_preserveExistingCopyright(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeARW(t, dirA, "RAW_0001.arw")
	writeSidecar(t, dirA, "RAW_0001.xmp", xmpWithExistingCopyright)

	opts := buildCarrySidecarOpts(t, dirA, dirB, &config.AppConfig{
		Copyright:            "New Copyright 2026",
		OverwriteSidecarTags: false, // default: preserve existing
	})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}

	arws := findFilesByExt(t, dirB, ".arw")
	if len(arws) != 1 {
		t.Fatalf("expected 1 ARW in dirB, got %d", len(arws))
	}
	data, err := os.ReadFile(arws[0] + ".xmp")
	if err != nil {
		t.Fatalf(".xmp sidecar not found: %v", err)
	}
	content := string(data)

	// Existing copyright must be preserved.
	if !strings.Contains(content, "Existing Copyright 2020") {
		t.Error("existing copyright was removed (should be preserved)")
	}
	if strings.Contains(content, "New Copyright 2026") {
		t.Error("new copyright was injected despite overwrite=false")
	}
	// Lightroom settings must survive.
	if !strings.Contains(content, "crs:Exposure2012") {
		t.Error("Lightroom settings were lost")
	}
}

// TestCarrySidecars_overwriteExistingCopyright verifies that when
// OverwriteSidecarTags is true, the existing copyright is replaced.
func TestCarrySidecars_overwriteExistingCopyright(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeARW(t, dirA, "RAW_0001.arw")
	writeSidecar(t, dirA, "RAW_0001.xmp", xmpWithExistingCopyright)

	opts := buildCarrySidecarOpts(t, dirA, dirB, &config.AppConfig{
		Copyright:            "New Copyright 2026",
		OverwriteSidecarTags: true,
	})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}

	arws := findFilesByExt(t, dirB, ".arw")
	if len(arws) != 1 {
		t.Fatalf("expected 1 ARW in dirB, got %d", len(arws))
	}
	data, err := os.ReadFile(arws[0] + ".xmp")
	if err != nil {
		t.Fatalf(".xmp sidecar not found: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "Existing Copyright 2020") {
		t.Error("existing copyright was not replaced (overwrite=true)")
	}
	if !strings.Contains(content, "New Copyright 2026") {
		t.Error("new copyright not found after overwrite")
	}
}

// TestCarrySidecars_disabled verifies that when CarrySidecars is false,
// sidecars are not carried to dirB.
func TestCarrySidecars_disabled(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	writeSidecar(t, dirA, "IMG_0001.aae", minimalAAE)
	writeSidecar(t, dirA, "IMG_0001.xmp", minimalXMPSidecar)

	// CarrySidecars = false (default).
	opts := buildOpts(t, dirA, dirB, false)
	// buildOpts doesn't set CarrySidecars — it defaults to false.

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}

	// No .aae or .xmp files should appear in dirB.
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".aae" || ext == ".xmp" {
			t.Errorf("unexpected sidecar in dirB (carry disabled): %q", path)
		}
		return nil
	})
}

// TestCarrySidecars_dryRun verifies that in dry-run mode, no sidecar files
// are copied to dirB (but the pipeline runs without error).
func TestCarrySidecars_dryRun(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	writeSidecar(t, dirA, "IMG_0001.aae", minimalAAE)

	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())

	var buf bytes.Buffer
	opts := pipeline.SortOptions{
		Config: &config.AppConfig{
			Source:        dirA,
			Destination:   dirB,
			Algorithm:     "sha1",
			DryRun:        true,
			CarrySidecars: true,
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &buf,
		PixeVersion:  "test",
	}

	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// No files should be in dirB (dry-run).
	_ = filepath.WalkDir(dirB, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		t.Errorf("unexpected file in dirB during dry-run: %q", path)
		return nil
	})

	// Output must contain a +sidecar line.
	output := buf.String()
	if !strings.Contains(output, "+sidecar") {
		t.Errorf("dry-run output missing +sidecar line; output:\n%s", output)
	}
}

// TestCarrySidecars_multipleSidecars verifies that both .aae and .xmp sidecars
// are carried when both are present.
func TestCarrySidecars_multipleSidecars(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeARW(t, dirA, "RAW_0001.arw")
	writeSidecar(t, dirA, "RAW_0001.aae", minimalAAE)
	writeSidecar(t, dirA, "RAW_0001.xmp", minimalXMPSidecar)

	opts := buildCarrySidecarOpts(t, dirA, dirB, &config.AppConfig{})
	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}

	arws := findFilesByExt(t, dirB, ".arw")
	if len(arws) != 1 {
		t.Fatalf("expected 1 ARW in dirB, got %d", len(arws))
	}
	arwDest := arws[0]

	if _, err := os.Stat(arwDest + ".aae"); err != nil {
		t.Errorf(".aae sidecar not found at %q: %v", arwDest+".aae", err)
	}
	if _, err := os.Stat(arwDest + ".xmp"); err != nil {
		t.Errorf(".xmp sidecar not found at %q: %v", arwDest+".xmp", err)
	}
}

// TestCarrySidecars_orphanSidecarSkipped verifies that a sidecar with no
// matching media file is reported as skipped (not carried).
func TestCarrySidecars_orphanSidecarSkipped(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Only a JPEG — no matching parent for ORPHAN.xmp.
	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	writeSidecar(t, dirA, "ORPHAN.xmp", minimalXMPSidecar)

	var buf bytes.Buffer
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())

	opts := pipeline.SortOptions{
		Config: &config.AppConfig{
			Source:        dirA,
			Destination:   dirB,
			Algorithm:     "sha1",
			CarrySidecars: true,
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &buf,
		PixeVersion:  "test",
	}

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Errors != 0 {
		t.Errorf("pipeline Errors = %d, want 0", result.Errors)
	}
	// The orphan sidecar must be reported as skipped.
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (orphan sidecar)", result.Skipped)
	}

	// ORPHAN.xmp must NOT appear in dirB.
	xmps := findFilesByExt(t, dirB, ".xmp")
	for _, p := range xmps {
		if strings.Contains(p, "ORPHAN") {
			t.Errorf("orphan sidecar was carried to dirB: %q", p)
		}
	}

	// Output must contain SKIP for the orphan.
	if !strings.Contains(buf.String(), "SKIP") {
		t.Errorf("output missing SKIP line for orphan sidecar; output:\n%s", buf.String())
	}
}

// TestCarrySidecars_outputContainsSidecarLine verifies that the +sidecar
// output line is emitted after the parent COPY line.
func TestCarrySidecars_outputContainsSidecarLine(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	writeSidecar(t, dirA, "IMG_0001.aae", minimalAAE)

	var buf bytes.Buffer
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())

	opts := pipeline.SortOptions{
		Config: &config.AppConfig{
			Source:        dirA,
			Destination:   dirB,
			Algorithm:     "sha1",
			CarrySidecars: true,
		},
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       &buf,
		PixeVersion:  "test",
	}

	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "COPY") {
		t.Errorf("output missing COPY line; output:\n%s", output)
	}
	if !strings.Contains(output, "+sidecar") {
		t.Errorf("output missing +sidecar line; output:\n%s", output)
	}
	if !strings.Contains(output, ".aae") {
		t.Errorf("output missing .aae reference; output:\n%s", output)
	}
}
