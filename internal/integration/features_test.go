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

// features_test.go contains integration tests for the features added in v2.3.0:
//   - B5: --since / --before date filters
//   - D3: --quiet / --verbose verbosity levels
//   - D4: colorized output (Formatter)
//   - A3: PNG handler
//   - A6: ORF (Olympus) and RW2 (Panasonic) RAW handlers

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/config"
	"github.com/cwlls/pixe-go/internal/discovery"
	jpeghandler "github.com/cwlls/pixe-go/internal/handler/jpeg"
	orfhandler "github.com/cwlls/pixe-go/internal/handler/orf"
	pnghandler "github.com/cwlls/pixe-go/internal/handler/png"
	rw2handler "github.com/cwlls/pixe-go/internal/handler/rw2"
	"github.com/cwlls/pixe-go/internal/hash"
	"github.com/cwlls/pixe-go/internal/pathbuilder"
	"github.com/cwlls/pixe-go/internal/pipeline"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// buildOptsWithConfig is a flexible helper that accepts a pre-built AppConfig.
func buildOptsWithConfig(t *testing.T, cfg *config.AppConfig, out *bytes.Buffer) pipeline.SortOptions {
	t.Helper()
	h, err := hash.NewHasher("sha1")
	if err != nil {
		t.Fatalf("NewHasher: %v", err)
	}
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	reg.Register(pnghandler.New())
	reg.Register(orfhandler.New())
	reg.Register(rw2handler.New())
	return pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       out,
		PixeVersion:  "test",
	}
}

// buildFakePNG writes a minimal valid PNG file (signature + IHDR + IEND).
func buildFakePNG(t *testing.T, dir, name string) string {
	t.Helper()
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	buf := new(bytes.Buffer)
	buf.Write(pngMagic)

	// IHDR chunk: length=13, type="IHDR", 13 bytes data, 4 bytes CRC placeholder.
	ihdrData := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdrData[0:4], 1) // width=1
	binary.BigEndian.PutUint32(ihdrData[4:8], 1) // height=1
	ihdrData[8] = 8                              // bit depth
	ihdrData[9] = 2                              // color type RGB
	_ = binary.Write(buf, binary.BigEndian, uint32(13))
	buf.WriteString("IHDR")
	buf.Write(ihdrData)
	buf.Write(make([]byte, 4)) // CRC placeholder

	// IEND chunk: length=0, type="IEND", 4 bytes CRC placeholder.
	_ = binary.Write(buf, binary.BigEndian, uint32(0))
	buf.WriteString("IEND")
	buf.Write(make([]byte, 4)) // CRC placeholder

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFakeORF writes a minimal Olympus ORF file (IIRO header).
func buildFakeORF(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)
	// Olympus ORF "IIRO" header.
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	buf.WriteByte(0x52)
	buf.WriteByte(0x4F)
	// IFD0 offset (8).
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))
	// IFD0: 0 entries.
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	// Next IFD offset (0 = end).
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// buildFakeRW2 writes a minimal Panasonic RW2 file.
func buildFakeRW2(t *testing.T, dir, name string) string {
	t.Helper()
	buf := new(bytes.Buffer)
	// Panasonic RW2 header: 49 49 55 00.
	buf.WriteByte(0x49)
	buf.WriteByte(0x49)
	buf.WriteByte(0x55)
	buf.WriteByte(0x00)
	// IFD0 offset (8).
	_ = binary.Write(buf, binary.LittleEndian, uint32(8))
	// IFD0: 0 entries.
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	// Next IFD offset (0 = end).
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ─── B5: Date filter tests ───────────────────────────────────────────────────

// TestDateFilter_Since verifies that --since skips files captured before the
// given date. fixtureExif1 has date 2021-12-25; fixtureExif2 has 2022-02-02.
// With --since 2022-01-01, only fixtureExif2 should be processed.
func TestDateFilter_Since(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_2021.jpg") // 2021-12-25
	copyFixture(t, dirA, fixtureExif2, "IMG_2022.jpg") // 2022-02-02

	since := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
		Since:       &since,
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Only the 2022 file should be processed; the 2021 file should be skipped.
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1 (only 2022 file)", result.Processed)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (2021 file filtered by --since)", result.Skipped)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// The SKIP line must mention "outside date range".
	output := out.String()
	if !strings.Contains(output, "outside date range") {
		t.Errorf("expected 'outside date range' in output, got:\n%s", output)
	}

	// The 2022 file must exist in dirB; the 2021 file must not.
	files2022 := findFiles(t, filepath.Join(dirB, "2022"), "20220202_")
	if len(files2022) != 1 {
		t.Errorf("expected 1 file in 2022/ with prefix 20220202_, got %d", len(files2022))
	}
	files2021 := findFiles(t, dirB, "20211225_")
	if len(files2021) != 0 {
		t.Errorf("expected 0 files with 2021 prefix (filtered by --since), got %d", len(files2021))
	}
}

// TestDateFilter_Before verifies that --before skips files captured after the
// given date. With --before 2021-12-31, only fixtureExif1 (2021-12-25) should
// be processed; fixtureExif2 (2022-02-02) should be skipped.
func TestDateFilter_Before(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_2021.jpg") // 2021-12-25
	copyFixture(t, dirA, fixtureExif2, "IMG_2022.jpg") // 2022-02-02

	// --before is end-of-day inclusive; parse as start-of-day then add 24h-1ns.
	beforeDay := time.Date(2021, 12, 31, 0, 0, 0, 0, time.UTC)
	before := beforeDay.Add(24*time.Hour - time.Nanosecond)
	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
		Before:      &before,
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1 (only 2021 file)", result.Processed)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (2022 file filtered by --before)", result.Skipped)
	}

	output := out.String()
	if !strings.Contains(output, "outside date range") {
		t.Errorf("expected 'outside date range' in output, got:\n%s", output)
	}
}

// TestDateFilter_Range verifies that combining --since and --before produces a
// closed date window. Files outside either bound are skipped.
func TestDateFilter_Range(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_2021.jpg")  // 2021-12-25 — before window
	copyFixture(t, dirA, fixtureExif2, "IMG_2022.jpg")  // 2022-02-02 — inside window
	copyFixture(t, dirA, fixtureNoExif, "IMG_1902.jpg") // 1902-02-20 — before window

	since := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	beforeDay := time.Date(2022, 12, 31, 0, 0, 0, 0, time.UTC)
	before := beforeDay.Add(24*time.Hour - time.Nanosecond)

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
		Since:       &since,
		Before:      &before,
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Only the 2022 file is inside the window.
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1", result.Processed)
	}
	if result.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", result.Skipped)
	}
}

// ─── D3: Verbosity tests ─────────────────────────────────────────────────────

// TestVerbosity_Quiet verifies that --quiet suppresses per-file lines but
// still emits the final summary.
func TestVerbosity_Quiet(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")
	copyFixture(t, dirA, fixtureExif2, "IMG_0002.jpg")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
		Verbosity:   -1, // quiet
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}

	output := out.String()

	// Per-file COPY lines must NOT appear.
	if strings.Contains(output, "COPY") {
		t.Errorf("quiet mode: unexpected COPY line in output:\n%s", output)
	}
	// Summary line MUST appear.
	if !strings.Contains(output, "Done.") {
		t.Errorf("quiet mode: expected 'Done.' summary line, got:\n%s", output)
	}
}

// TestVerbosity_Normal verifies that normal mode (Verbosity=0) emits COPY lines.
func TestVerbosity_Normal(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
		Verbosity:   0, // normal
	}
	opts := buildOptsWithConfig(t, cfg, out)

	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "COPY") {
		t.Errorf("normal mode: expected COPY line in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Done.") {
		t.Errorf("normal mode: expected 'Done.' summary line, got:\n%s", output)
	}
}

// TestVerbosity_Verbose verifies that verbose mode (Verbosity=1) emits COPY
// lines plus timing information.
func TestVerbosity_Verbose(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
		Verbosity:   1, // verbose
	}
	opts := buildOptsWithConfig(t, cfg, out)

	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := out.String()
	// Verbose mode must still emit COPY and Done.
	if !strings.Contains(output, "COPY") {
		t.Errorf("verbose mode: expected COPY line, got:\n%s", output)
	}
	if !strings.Contains(output, "Done.") {
		t.Errorf("verbose mode: expected 'Done.' summary line, got:\n%s", output)
	}
	// Verbose mode must emit timing info (parenthesized duration).
	// The duration unit varies by system speed: fast systems may emit "(0s)"
	// while slower ones emit "(Xms)". We only verify the parenthesized form
	// is present, not the specific unit.
	if !strings.Contains(output, "(") || !strings.Contains(output, ")") {
		t.Errorf("verbose mode: expected timing info like '(Xms)' or '(0s)', got:\n%s", output)
	}
}

// ─── D4: Colorized output tests ──────────────────────────────────────────────

// TestColorFormatter_NoColor verifies that with ColorOutput=false the output
// contains no ANSI escape sequences.
func TestColorFormatter_NoColor(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	h, _ := hash.NewHasher("sha1")
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	opts := pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       out,
		PixeVersion:  "test",
		ColorOutput:  false, // explicitly disabled
	}

	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "\x1b[") {
		t.Errorf("no-color mode: unexpected ANSI escape in output:\n%s", output)
	}
	if !strings.Contains(output, "COPY") {
		t.Errorf("no-color mode: expected plain COPY verb, got:\n%s", output)
	}
}

// TestColorFormatter_WithColor verifies that with ColorOutput=true the output
// contains ANSI escape sequences wrapping the status verbs.
func TestColorFormatter_WithColor(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "IMG_0001.jpg")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	h, _ := hash.NewHasher("sha1")
	reg := discovery.NewRegistry()
	reg.Register(jpeghandler.New())
	opts := pipeline.SortOptions{
		Config:       cfg,
		Hasher:       h,
		Registry:     reg,
		RunTimestamp: pathbuilder.RunTimestamp(time.Now()),
		Output:       out,
		PixeVersion:  "test",
		ColorOutput:  true, // explicitly enabled
	}

	if _, err := pipeline.Run(opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := out.String()
	// ANSI escape sequences must be present when color is enabled.
	if !strings.Contains(output, "\x1b[") {
		t.Errorf("color mode: expected ANSI escape sequences in output, got:\n%s", output)
	}
}

// ─── A3: PNG handler tests ───────────────────────────────────────────────────

// TestPNG_Discovery verifies that .png files are discovered and processed.
func TestPNG_Discovery(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakePNG(t, dirA, "screenshot.png")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1 (PNG file must be discovered)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// PNG with no EXIF → Ansel Adams date → 1902/02-Feb/.
	pngFiles := findFiles(t, dirB, "19020220_000000-")
	found := false
	for _, f := range pngFiles {
		if strings.HasSuffix(f, ".png") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a .png file with Ansel Adams prefix in dirB, got files: %v", pngFiles)
	}
}

// TestPNG_ExtensionPreserved verifies that the destination file has a lowercase
// .png extension.
func TestPNG_ExtensionPreserved(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakePNG(t, dirA, "SCREENSHOT.PNG") // uppercase source extension

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1", result.Processed)
	}

	// Destination must have lowercase .png extension.
	pngFiles := findFiles(t, dirB, "19020220_000000-")
	for _, f := range pngFiles {
		if strings.HasSuffix(f, ".PNG") {
			t.Errorf("destination has uppercase .PNG extension: %q", f)
		}
	}
}

// TestPNG_MixedWithJPEG verifies that a directory with both JPEG and PNG files
// is sorted correctly.
func TestPNG_MixedWithJPEG(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "photo.jpg") // 2021-12-25
	buildFakePNG(t, dirA, "screenshot.png")         // no EXIF → 1902-02-20

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 2 {
		t.Errorf("Processed = %d, want 2", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// JPEG must land in 2021/12-Dec/.
	jpegFiles := findFiles(t, filepath.Join(dirB, "2021", "12-Dec"), "20211225_")
	if len(jpegFiles) != 1 {
		t.Errorf("expected 1 JPEG in 2021/12-Dec/, got %d", len(jpegFiles))
	}

	// PNG must land in 1902/02-Feb/ with .png extension.
	pngFiles := findFiles(t, dirB, "19020220_000000-")
	foundPNG := false
	for _, f := range pngFiles {
		if strings.HasSuffix(f, ".png") {
			foundPNG = true
			break
		}
	}
	if !foundPNG {
		t.Errorf("expected .png file with Ansel Adams prefix in dirB")
	}
}

// ─── A6: ORF and RW2 handler tests ──────────────────────────────────────────

// TestORF_Discovery verifies that .orf files are discovered and processed.
func TestORF_Discovery(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeORF(t, dirA, "olympus.orf")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1 (ORF file must be discovered)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// ORF with no EXIF → Ansel Adams date.
	orfFiles := findFiles(t, dirB, "19020220_000000-")
	found := false
	for _, f := range orfFiles {
		if strings.HasSuffix(f, ".orf") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a .orf file with Ansel Adams prefix in dirB, got: %v", orfFiles)
	}
}

// TestRW2_Discovery verifies that .rw2 files are discovered and processed.
func TestRW2_Discovery(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	buildFakeRW2(t, dirA, "panasonic.rw2")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 1 {
		t.Errorf("Processed = %d, want 1 (RW2 file must be discovered)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}

	// RW2 with no EXIF → Ansel Adams date.
	rw2Files := findFiles(t, dirB, "19020220_000000-")
	found := false
	for _, f := range rw2Files {
		if strings.HasSuffix(f, ".rw2") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a .rw2 file with Ansel Adams prefix in dirB, got: %v", rw2Files)
	}
}

// TestORF_RW2_MixedBatch verifies that a directory with ORF, RW2, JPEG, and
// PNG files is sorted correctly with all formats processed.
func TestORF_RW2_MixedBatch(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	copyFixture(t, dirA, fixtureExif1, "photo.jpg") // 2021-12-25
	buildFakePNG(t, dirA, "screenshot.png")         // no EXIF
	buildFakeORF(t, dirA, "olympus.orf")            // no EXIF
	buildFakeRW2(t, dirA, "panasonic.rw2")          // no EXIF

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Processed != 4 {
		t.Errorf("Processed = %d, want 4 (jpg + png + orf + rw2)", result.Processed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0\nOutput:\n%s", result.Errors, out.String())
	}

	// JPEG must land in 2021/12-Dec/.
	jpegFiles := findFiles(t, filepath.Join(dirB, "2021", "12-Dec"), "20211225_")
	if len(jpegFiles) != 1 {
		t.Errorf("expected 1 JPEG in 2021/12-Dec/, got %d", len(jpegFiles))
	}

	// PNG, ORF, RW2 must all land in 1902/02-Feb/ with correct extensions.
	anselFiles := findFiles(t, dirB, "19020220_000000-")
	extsSeen := map[string]bool{}
	for _, f := range anselFiles {
		extsSeen[filepath.Ext(f)] = true
	}
	for _, ext := range []string{".png", ".orf", ".rw2"} {
		if !extsSeen[ext] {
			t.Errorf("expected a %s file with Ansel Adams prefix in dirB; seen: %v", ext, extsSeen)
		}
	}
}

// TestORF_RW2_DuplicateDetection verifies that identical ORF/RW2 files are
// routed to the duplicates/ directory.
func TestORF_RW2_DuplicateDetection(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Two identical ORF files.
	buildFakeORF(t, dirA, "olympus_a.orf")
	buildFakeORF(t, dirA, "olympus_b.orf")

	out := &bytes.Buffer{}
	cfg := &config.AppConfig{
		Source:      dirA,
		Destination: dirB,
		Algorithm:   "sha1",
	}
	opts := buildOptsWithConfig(t, cfg, out)

	result, err := pipeline.Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}

	dupDir := filepath.Join(dirB, "duplicates")
	if _, err := os.Stat(dupDir); err != nil {
		t.Errorf("duplicates/ directory not created: %v", err)
	}
}
