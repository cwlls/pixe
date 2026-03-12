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

package png

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/handler/handlertest"
)

func TestHandler(t *testing.T) {
	handlertest.RunSuite(t, handlertest.SuiteConfig{
		NewHandler: func() domain.FileTypeHandler { return New() },
		Extensions: []string{".png"},
		MagicSignatures: []domain.MagicSignature{
			{Offset: 0, Bytes: pngMagic},
		},
		BuildFakeFile:      buildFakePNG,
		WrongExtension:     "test.jpg",
		MetadataCapability: domain.MetadataSidecar,
	})
}

// buildFakePNG creates a minimal valid PNG file at the given path.
// It contains: PNG signature + IHDR chunk + IEND chunk.
func buildFakePNG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	// PNG signature.
	_, _ = f.Write(pngMagic)

	// IHDR chunk: length=13, type="IHDR", data (13 bytes), CRC (4 bytes).
	ihdrData := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdrData[0:4], 1) // width=1
	binary.BigEndian.PutUint32(ihdrData[4:8], 1) // height=1
	ihdrData[8] = 8                              // bit depth
	ihdrData[9] = 2                              // color type (RGB)
	ihdrData[10] = 0                             // compression
	ihdrData[11] = 0                             // filter
	ihdrData[12] = 0                             // interlace

	writeChunk(f, "IHDR", ihdrData)

	// IEND chunk: length=0, type="IEND", CRC.
	writeChunk(f, "IEND", nil)

	return path
}

// writeChunk writes a PNG chunk (length + type + data + CRC placeholder).
func writeChunk(f *os.File, chunkType string, data []byte) {
	_ = binary.Write(f, binary.BigEndian, uint32(len(data)))
	_, _ = f.Write([]byte(chunkType))
	if len(data) > 0 {
		_, _ = f.Write(data)
	}
	// CRC placeholder (4 bytes of zeros — not a valid CRC but sufficient for testing).
	_, _ = f.Write(make([]byte, 4))
}
