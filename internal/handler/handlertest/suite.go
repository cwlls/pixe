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

package handlertest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cwlls/pixe-go/internal/domain"
)

// SuiteConfig configures the shared handler test suite for a specific handler.
type SuiteConfig struct {
	// NewHandler returns a fresh handler instance for each subtest.
	NewHandler func() domain.FileTypeHandler

	// Extensions is the expected return value of handler.Extensions().
	Extensions []string

	// MagicSignatures is the expected return value of handler.MagicBytes().
	MagicSignatures []domain.MagicSignature

	// BuildFakeFile writes a minimal valid file for this format into dir
	// with the given name and returns the absolute path. It must call
	// t.Helper() as its first statement.
	BuildFakeFile func(t *testing.T, dir, name string) string

	// WrongExtension is a filename with an incorrect extension for this handler
	// (e.g., "test.jpg" for a DNG handler). Used in Detect/wrongExtension.
	WrongExtension string

	// MetadataCapability is the expected MetadataSupport() return value.
	MetadataCapability domain.MetadataCapability
}

// RunSuite runs the standard 10-subtest handler suite against the provided
// config. It is called from each handler's test file as the sole test body.
func RunSuite(t *testing.T, cfg SuiteConfig) {
	t.Helper()

	t.Run("Extensions", func(t *testing.T) {
		t.Parallel()
		h := cfg.NewHandler()
		got := h.Extensions()
		if len(got) != len(cfg.Extensions) {
			t.Fatalf("Extensions() len = %d, want %d; got %v", len(got), len(cfg.Extensions), got)
		}
		for i, want := range cfg.Extensions {
			if got[i] != want {
				t.Errorf("Extensions()[%d] = %q, want %q", i, got[i], want)
			}
		}
	})

	t.Run("MagicBytes", func(t *testing.T) {
		t.Parallel()
		h := cfg.NewHandler()
		got := h.MagicBytes()
		if len(got) != len(cfg.MagicSignatures) {
			t.Fatalf("MagicBytes() len = %d, want %d", len(got), len(cfg.MagicSignatures))
		}
		for i, want := range cfg.MagicSignatures {
			if got[i].Offset != want.Offset {
				t.Errorf("MagicBytes()[%d].Offset = %d, want %d", i, got[i].Offset, want.Offset)
			}
			if !bytes.Equal(got[i].Bytes, want.Bytes) {
				t.Errorf("MagicBytes()[%d].Bytes = %v, want %v", i, got[i].Bytes, want.Bytes)
			}
		}
	})

	t.Run("Detect/valid", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Use the first extension as the canonical extension for the fake file.
		name := "test" + cfg.Extensions[0]
		filePath := cfg.BuildFakeFile(t, dir, name)

		h := cfg.NewHandler()
		ok, err := h.Detect(filePath)
		if err != nil {
			t.Fatalf("Detect(%q): %v", filePath, err)
		}
		if !ok {
			t.Errorf("Detect returned false for valid %s file", cfg.Extensions[0])
		}
	})

	t.Run("Detect/wrongExtension", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		// Build a valid fake file but save it with the wrong extension.
		filePath := cfg.BuildFakeFile(t, dir, cfg.WrongExtension)

		h := cfg.NewHandler()
		ok, err := h.Detect(filePath)
		if err != nil {
			t.Fatalf("Detect(%q): %v", filePath, err)
		}
		if ok {
			t.Errorf("Detect returned true for wrong extension %q", cfg.WrongExtension)
		}
	})

	t.Run("Detect/wrongMagic", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		name := "test" + cfg.Extensions[0]
		path := filepath.Join(dir, name)
		// Write null bytes — will never match any TIFF magic signature.
		if err := os.WriteFile(path, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		h := cfg.NewHandler()
		ok, err := h.Detect(path)
		if err != nil {
			t.Fatalf("Detect(%q): %v", path, err)
		}
		if ok {
			t.Errorf("Detect returned true for file with wrong magic bytes")
		}
	})

	t.Run("ExtractDate/noEXIF_fallback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		name := "test" + cfg.Extensions[0]
		filePath := cfg.BuildFakeFile(t, dir, name)

		h := cfg.NewHandler()
		date, err := h.ExtractDate(filePath)
		if err != nil {
			t.Fatalf("ExtractDate(%q): %v", filePath, err)
		}

		// Minimal fake files have no EXIF — must fall back to Ansel Adams' birthday.
		want := time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)
		if !date.Equal(want) {
			t.Errorf("ExtractDate = %v, want Ansel Adams date %v", date, want)
		}
	})

	t.Run("HashableReader/returnsData", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		name := "test" + cfg.Extensions[0]
		filePath := cfg.BuildFakeFile(t, dir, name)

		h := cfg.NewHandler()
		rc, err := h.HashableReader(filePath)
		if err != nil {
			t.Fatalf("HashableReader(%q): %v", filePath, err)
		}
		defer func() { _ = rc.Close() }()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if len(data) == 0 {
			t.Error("HashableReader returned empty data")
		}
	})

	t.Run("HashableReader/deterministic", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		name := "test" + cfg.Extensions[0]
		filePath := cfg.BuildFakeFile(t, dir, name)

		h := cfg.NewHandler()

		read := func() []byte {
			rc, err := h.HashableReader(filePath)
			if err != nil {
				t.Fatalf("HashableReader(%q): %v", filePath, err)
			}
			defer func() { _ = rc.Close() }()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			return data
		}

		d1 := read()
		d2 := read()
		if !bytes.Equal(d1, d2) {
			t.Error("HashableReader returned different data on second call (not deterministic)")
		}
	})

	t.Run("MetadataSupport", func(t *testing.T) {
		t.Parallel()
		h := cfg.NewHandler()
		got := h.MetadataSupport()
		if got != cfg.MetadataCapability {
			t.Errorf("MetadataSupport() = %v, want %v", got, cfg.MetadataCapability)
		}
	})

	t.Run("WriteMetadataTags/noop", func(t *testing.T) {
		t.Parallel()
		h := cfg.NewHandler()
		tags := domain.MetadataTags{Copyright: "test", CameraOwner: "test"}
		// Use a dummy path — WriteMetadataTags must be a no-op for sidecar handlers.
		err := h.WriteMetadataTags("dummy"+cfg.Extensions[0], tags)
		if err != nil {
			t.Errorf("WriteMetadataTags should be a no-op, got error: %v", err)
		}
	})
}
