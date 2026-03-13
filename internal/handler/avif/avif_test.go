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

package avif

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cwlls/pixe/internal/domain"
	"github.com/cwlls/pixe/internal/handler/handlertest"
)

func TestHandler(t *testing.T) {
	handlertest.RunSuite(t, handlertest.SuiteConfig{
		NewHandler: func() domain.FileTypeHandler { return New() },
		Extensions: []string{".avif"},
		MagicSignatures: []domain.MagicSignature{
			{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
		},
		BuildFakeFile:      buildFakeAVIF,
		WrongExtension:     "test.jpg",
		MetadataCapability: domain.MetadataSidecar,
	})
}

// buildFakeAVIF writes a minimal valid AVIF file at the given path.
// The file contains a single 24-byte ISOBMFF ftyp box with the "avif"
// major brand. This is sufficient for Detect() and MagicBytes() tests.
// ExtractDate() will fall back to the Ansel Adams date (no EXIF present),
// which is the expected behaviour for the handlertest no-EXIF subtest.
func buildFakeAVIF(t *testing.T, dir, name string) string {
	t.Helper()
	// ftyp box: size(4) + "ftyp"(4) + brand "avif"(4) + minor(4) +
	//           compat "avif"(4) + compat "mif1"(4) = 24 bytes total.
	data := []byte{
		0x00, 0x00, 0x00, 0x18, // box size = 24
		0x66, 0x74, 0x79, 0x70, // "ftyp"
		0x61, 0x76, 0x69, 0x66, // major brand "avif"
		0x00, 0x00, 0x00, 0x00, // minor version
		0x61, 0x76, 0x69, 0x66, // compat brand "avif"
		0x6D, 0x69, 0x66, 0x31, // compat brand "mif1"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("buildFakeAVIF: %v", err)
	}
	return path
}
