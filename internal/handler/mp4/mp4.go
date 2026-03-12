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

// Package mp4 implements the FileTypeHandler contract for MP4/MOV video files.
//
// Package selection:
//   - Atom parsing: github.com/abema/go-mp4 — pure Go, atom-level access to
//     mvhd (creation date), stss (sync samples), stco/co64 (chunk offsets),
//     stsz (sample sizes).
//   - Metadata write: udta/©cpy and udta/©own atoms for Copyright and
//     CameraOwner. Written via go-mp4's box builder.
//
// Date extraction:
//
//	QuickTime/MP4 stores creation time as seconds since 1904-01-01 00:00:00 UTC
//	in the mvhd box. We convert this to a time.Time and apply the fallback chain.
//
// Hashable region:
//
//	The hashable region is the concatenated raw bytes of all video keyframes
//	(sync samples). Keyframe locations are derived from:
//	  stss → sync sample indices
//	  stco/co64 → chunk byte offsets in the file
//	  stsc → samples-per-chunk mapping
//	  stsz → individual sample sizes
//	This excludes audio, metadata atoms, and non-keyframe video frames, so
//	the checksum is stable across metadata edits.
//
// Magic bytes:
//
//	MP4/MOV files are ISOBMFF containers. The ftyp box starts at offset 0:
//	  bytes 0-3: box size (big-endian uint32)
//	  bytes 4-7: box type "ftyp" (0x66 0x74 0x79 0x70)
//	We match on "ftyp" at offset 4, same as HEIC.
//	MOV files may use "wide" or "mdat" as the first box — we also accept
//	"moov" at offset 4 as a secondary signature.
package mp4

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	mp4lib "github.com/abema/go-mp4"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/fileutil"
)

// Compile-time interface check.
var _ domain.FileTypeHandler = (*Handler)(nil)

// mp4Epoch is the QuickTime/MP4 epoch: 1904-01-01 00:00:00 UTC.
var mp4Epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

// anselsAdams is the fallback date when no creation time can be extracted.
var anselsAdams = time.Date(1902, 2, 20, 0, 0, 0, 0, time.UTC)

// Handler implements domain.FileTypeHandler for MP4/MOV video files.
type Handler struct{}

// New returns a new MP4 Handler.
func New() *Handler { return &Handler{} }

// Extensions returns the lowercase file extensions this handler claims.
func (h *Handler) Extensions() []string {
	return []string{".mp4", ".mov"}
}

// MagicBytes returns the ISOBMFF ftyp box signature at offset 4.
// Also accepts "moov" at offset 4 for bare MOV files without ftyp.
func (h *Handler) MagicBytes() []domain.MagicSignature {
	return []domain.MagicSignature{
		{Offset: 4, Bytes: []byte{0x66, 0x74, 0x79, 0x70}}, // "ftyp"
		{Offset: 4, Bytes: []byte{0x6d, 0x6f, 0x6f, 0x76}}, // "moov"
		{Offset: 4, Bytes: []byte{0x77, 0x69, 0x64, 0x65}}, // "wide"
		{Offset: 4, Bytes: []byte{0x6d, 0x64, 0x61, 0x74}}, // "mdat"
	}
}

// Detect returns true if the file has a .mp4/.mov extension AND contains
// a recognised ISOBMFF box type at offset 4.
func (h *Handler) Detect(filePath string) (bool, error) {
	ext := strings.ToLower(fileutil.Ext(filePath))
	if ext != ".mp4" && ext != ".mov" {
		return false, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("mp4: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		return false, nil
	}

	boxType := string(header[4:8])
	switch boxType {
	case "ftyp", "moov", "wide", "mdat":
		return true, nil
	}
	return false, nil
}

// ExtractDate reads the creation time from the mvhd atom.
// Fallback chain: mvhd CreationTime → anselsAdams.
func (h *Handler) ExtractDate(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return anselsAdams, fmt.Errorf("mp4: open %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	boxes, err := mp4lib.ExtractBoxWithPayload(f, nil, mp4lib.BoxPath{
		mp4lib.BoxTypeMoov(),
		mp4lib.BoxTypeMvhd(),
	})
	if err != nil || len(boxes) == 0 {
		return anselsAdams, nil
	}

	mvhd, ok := boxes[0].Payload.(*mp4lib.Mvhd)
	if !ok {
		return anselsAdams, nil
	}

	secs := mvhd.GetCreationTime()
	if secs == 0 {
		return anselsAdams, nil
	}

	t := mp4Epoch.Add(time.Duration(secs) * time.Second)
	return t.UTC(), nil
}

// HashableReader returns an io.ReadCloser that yields the concatenated raw
// bytes of all video keyframes (sync samples) in presentation order.
//
// The implementation uses io.SectionReader + io.MultiReader to stream
// keyframe data directly from the file without loading it into memory.
// This bounds peak memory usage to O(1) regardless of video size or
// keyframe count, which is important when many workers run concurrently.
//
// If keyframe extraction fails (e.g. the file has no video track or no stss
// box), the full file is returned as a fallback so the file is always hashable.
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("mp4: open %q: %w", filePath, err)
	}

	r, err := extractKeyframePayload(f)
	if err != nil {
		// Fallback: hash the full file. Seek back to start.
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			_ = f.Close()
			return nil, fmt.Errorf("mp4: seek %q: %w", filePath, seekErr)
		}
		return f, nil
	}

	return &mp4ReadCloser{Reader: r, file: f}, nil
}

// mp4ReadCloser wraps an io.Reader with the file it reads from.
type mp4ReadCloser struct {
	io.Reader
	file *os.File
}

func (rc *mp4ReadCloser) Close() error {
	return rc.file.Close()
}

// MetadataSupport declares that MP4/MOV uses XMP sidecar files.
// Embedded udta atom writing may be added in a future enhancement.
func (h *Handler) MetadataSupport() domain.MetadataCapability {
	return domain.MetadataSidecar
}

// WriteMetadataTags is a no-op retained for interface compliance.
// The pipeline checks MetadataSupport() and routes to XMP sidecar
// generation instead of calling this method.
func (h *Handler) WriteMetadataTags(_ string, _ domain.MetadataTags) error {
	return nil
}

// extractKeyframePayload parses the stss, stco/co64, stsc, and stsz boxes to
// locate video keyframes and returns a streaming io.Reader over their byte
// ranges. It uses io.SectionReader + io.MultiReader so no keyframe data is
// loaded into memory — each SectionReader reads directly from f on demand.
//
// The caller must keep f open for the lifetime of the returned reader.
func extractKeyframePayload(f *os.File) (io.Reader, error) {
	// Extract stss (sync sample table) — lists keyframe sample numbers.
	stssBoxes, err := mp4lib.ExtractBoxWithPayload(f, nil, mp4lib.BoxPath{
		mp4lib.BoxTypeMoov(),
		mp4lib.BoxTypeTrak(),
		mp4lib.BoxTypeMdia(),
		mp4lib.BoxTypeMinf(),
		mp4lib.BoxTypeStbl(),
		mp4lib.BoxTypeStss(),
	})
	if err != nil || len(stssBoxes) == 0 {
		return nil, fmt.Errorf("no stss box")
	}
	stss, ok := stssBoxes[0].Payload.(*mp4lib.Stss)
	if !ok || len(stss.SampleNumber) == 0 {
		return nil, fmt.Errorf("empty stss")
	}

	// Extract stsz (sample sizes).
	stszBoxes, err := mp4lib.ExtractBoxWithPayload(f, nil, mp4lib.BoxPath{
		mp4lib.BoxTypeMoov(),
		mp4lib.BoxTypeTrak(),
		mp4lib.BoxTypeMdia(),
		mp4lib.BoxTypeMinf(),
		mp4lib.BoxTypeStbl(),
		mp4lib.BoxTypeStsz(),
	})
	if err != nil || len(stszBoxes) == 0 {
		return nil, fmt.Errorf("no stsz box")
	}
	stsz, ok := stszBoxes[0].Payload.(*mp4lib.Stsz)
	if !ok {
		return nil, fmt.Errorf("invalid stsz")
	}

	// Extract stco (chunk offsets) — try co64 first for large files.
	chunkOffsets, err := extractChunkOffsets(f)
	if err != nil || len(chunkOffsets) == 0 {
		return nil, fmt.Errorf("no chunk offsets")
	}

	// Extract stsc (sample-to-chunk mapping).
	stscBoxes, err := mp4lib.ExtractBoxWithPayload(f, nil, mp4lib.BoxPath{
		mp4lib.BoxTypeMoov(),
		mp4lib.BoxTypeTrak(),
		mp4lib.BoxTypeMdia(),
		mp4lib.BoxTypeMinf(),
		mp4lib.BoxTypeStbl(),
		mp4lib.BoxTypeStsc(),
	})
	if err != nil || len(stscBoxes) == 0 {
		return nil, fmt.Errorf("no stsc box")
	}
	stsc, ok := stscBoxes[0].Payload.(*mp4lib.Stsc)
	if !ok {
		return nil, fmt.Errorf("invalid stsc")
	}

	// Build sample → file offset map.
	sampleOffsets := buildSampleOffsets(stsc, chunkOffsets, stsz)

	// Build a slice of SectionReaders — one per keyframe — and combine with
	// MultiReader. No keyframe data is loaded into memory; each SectionReader
	// reads directly from f when the caller reads from the returned io.Reader.
	var readers []io.Reader
	for _, sampleNum := range stss.SampleNumber {
		idx := int(sampleNum) - 1 // 1-based → 0-based
		if idx < 0 || idx >= len(sampleOffsets) {
			continue
		}
		so := sampleOffsets[idx]
		if so.size == 0 {
			continue
		}
		readers = append(readers, io.NewSectionReader(f, int64(so.offset), int64(so.size)))
	}
	if len(readers) == 0 {
		return nil, fmt.Errorf("no keyframes found")
	}
	return io.MultiReader(readers...), nil
}

// sampleLocation identifies a byte range within the mdat box for a single sample.
type sampleLocation struct {
	offset uint64
	size   uint32
}

// buildSampleOffsets maps each sample index to its file offset and size.
func buildSampleOffsets(stsc *mp4lib.Stsc, chunkOffsets []uint64, stsz *mp4lib.Stsz) []sampleLocation {
	if len(stsc.Entries) == 0 || len(chunkOffsets) == 0 {
		return nil
	}

	totalSamples := len(stsz.EntrySize)
	if stsz.SampleSize != 0 {
		// Fixed sample size — compute total from stsz.SampleCount.
		totalSamples = int(stsz.SampleCount)
	}

	locs := make([]sampleLocation, 0, totalSamples)
	sampleIdx := 0

	for chunkIdx := 0; chunkIdx < len(chunkOffsets); chunkIdx++ {
		samplesInChunk := samplesPerChunk(stsc, chunkIdx+1) // 1-based chunk number
		chunkOffset := chunkOffsets[chunkIdx]

		offset := chunkOffset
		for s := 0; s < int(samplesInChunk); s++ {
			var size uint32
			if stsz.SampleSize != 0 {
				size = stsz.SampleSize
			} else if sampleIdx < len(stsz.EntrySize) {
				size = stsz.EntrySize[sampleIdx]
			}
			locs = append(locs, sampleLocation{offset: offset, size: size})
			offset += uint64(size)
			sampleIdx++
		}
	}
	return locs
}

// samplesPerChunk returns the number of samples in the given 1-based chunk
// number by walking the stsc entries.
func samplesPerChunk(stsc *mp4lib.Stsc, chunkNum int) uint32 {
	var result uint32 = 1
	for _, e := range stsc.Entries {
		if int(e.FirstChunk) <= chunkNum {
			result = e.SamplesPerChunk
		} else {
			break
		}
	}
	return result
}

// extractChunkOffsets returns chunk offsets from co64 (preferred) or stco.
func extractChunkOffsets(f *os.File) ([]uint64, error) {
	basePath := mp4lib.BoxPath{
		mp4lib.BoxTypeMoov(),
		mp4lib.BoxTypeTrak(),
		mp4lib.BoxTypeMdia(),
		mp4lib.BoxTypeMinf(),
		mp4lib.BoxTypeStbl(),
	}

	// Try co64 first (large file support).
	co64Boxes, err := mp4lib.ExtractBoxWithPayload(f, nil, append(basePath, mp4lib.BoxTypeCo64()))
	if err == nil && len(co64Boxes) > 0 {
		if co64, ok := co64Boxes[0].Payload.(*mp4lib.Co64); ok {
			offsets := make([]uint64, len(co64.ChunkOffset))
			copy(offsets, co64.ChunkOffset)
			return offsets, nil
		}
	}

	// Fall back to stco.
	stcoBoxes, err := mp4lib.ExtractBoxWithPayload(f, nil, append(basePath, mp4lib.BoxTypeStco()))
	if err != nil || len(stcoBoxes) == 0 {
		return nil, fmt.Errorf("no stco/co64 box")
	}
	stco, ok := stcoBoxes[0].Payload.(*mp4lib.Stco)
	if !ok {
		return nil, fmt.Errorf("invalid stco")
	}
	offsets := make([]uint64, len(stco.ChunkOffset))
	for i, o := range stco.ChunkOffset {
		offsets[i] = uint64(o)
	}
	return offsets, nil
}
