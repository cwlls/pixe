# Implementation

## Task Summary

| #  | Task | Priority | Agent | Status | Depends On | Notes |
|:---|:-----|:---------|:------|:-------|:-----------|:------|
| 1  | Implement `findSensorDataIFD` in `tiffraw` package | high | @developer | [x] complete | — | New IFD selection logic: sensor data vs JPEG preview |
| 2  | Rewrite `tiffraw.Base.HashableReader` to use sensor data | high | @developer | [x] complete | 1 | Replace `findLargestJPEGPreview` call with `findSensorDataIFD` |
| 3  | Implement `findCR3SensorData` in `cr3` package | high | @developer | [x] complete | — | ISOBMFF box navigation for raw sensor data track |
| 4  | Rewrite `cr3.Handler.HashableReader` to use sensor data | high | @developer | [x] complete | 3 | Replace `findCR3JpegPreview` call with `findCR3SensorData` |
| 5  | Update `tiffraw` package doc comment | medium | @developer | [x] complete | 2 | Change "Hashable region" section from JPEG preview to sensor data |
| 6  | Update `cr3` package doc comment | medium | @developer | [x] complete | 4 | Change "Hashable region" section from JPEG preview to sensor data |
| 7  | Update `tiffraw_test.go` — sensor data extraction tests | high | @developer | [x] complete | 2 | New test fixtures with sensor data IFDs; update existing tests |
| 8  | Update `cr3_test.go` — sensor data extraction tests | high | @developer | [x] complete | 4 | New test fixtures with sensor data in mdat; update existing tests |
| 9  | Run full test suite and fix regressions | high | @tester | [x] complete | 7, 8 | `make test-all` with `-race`; verify all handler tests pass |
| 10 | Run lint and format checks | medium | @developer | [x] complete | 9 | `make check` (fmt-check + vet + unit tests) |

---

## Task Descriptions

### Task 1: Implement `findSensorDataIFD` in `tiffraw` package

**File:** `internal/handler/tiffraw/tiffraw.go`

**What:** Add a new function `findSensorData(r io.ReadSeeker) (*sensorRegion, error)` that navigates the TIFF IFD chain and locates the raw sensor data payload — the primary image data stored with a non-JPEG compression scheme.

**Type to add:**

```go
// sensorRegion holds the offset(s) and size(s) of the raw sensor data strips/tiles.
type sensorRegion struct {
    offsets    []int64  // file offsets of each strip/tile
    byteCounts []int64  // byte count of each strip/tile
    totalSize  int64    // sum of all byteCounts
}
```

**IFD selection logic — distinguishing sensor data from JPEG preview:**

The function walks the TIFF IFD chain (IFD0 → IFD1 → ... plus SubIFD pointers via tag `0x014A`) and collects candidate IFDs. For each IFD, read:

- `Compression` (tag `0x0103`) — sensor data uses non-JPEG values: `1` (uncompressed), `7` (lossless JPEG — used by NEF, CR2), `34713` (Nikon NEF compressed), `34892` (lossy JPEG DNG), or other vendor-specific values. JPEG preview IFDs use compression `6`.
- `ImageWidth` (tag `0x0100`) and `ImageLength` (tag `0x0101`) — sensor data has the largest dimensions.
- `NewSubfileType` (tag `0x00FE`) — value `0` = full-resolution primary image; value `1` = reduced-resolution preview.
- `StripOffsets` (tag `0x0111`) / `StripByteCounts` (tag `0x0117`) — or `TileOffsets` (tag `0x0144`) / `TileByteCounts` (tag `0x0145`) for tiled formats.

**Selection algorithm:**

1. Collect all IFDs that have strip/tile data (offsets + byte counts present).
2. Exclude IFDs where `Compression == 6` (standard JPEG) — these are preview IFDs.
3. Among remaining candidates, prefer the IFD with `NewSubfileType == 0` if present.
4. If ambiguity remains, select the IFD with the largest total data payload (sum of `StripByteCounts` or `TileByteCounts`).
5. If no non-JPEG IFD is found, return `nil` (caller falls back to full-file hash).

**New tag constants to add:**

```go
const (
    tagImageWidth      = 0x0100
    tagImageLength     = 0x0101
    tagTileOffsets     = 0x0144
    tagTileByteCounts  = 0x0145
)
```

**Note on `Compression == 7`:** Lossless JPEG (compression `7`) is used by sensor data in NEF and CR2 files. This is distinct from standard JPEG compression (`6`) used for preview images. The selection logic must treat `7` as a sensor data compression type, not a preview type.

**Reuse existing infrastructure:** The `parseIFD` function and `readUint32Array` helper already handle IFD traversal and tag reading. Extend `ifdValues` to capture the new tags (`ImageWidth`, `ImageLength`, `TileOffsets`, `TileByteCounts`), and add a new function that uses the extended `parseIFD` to select the sensor data IFD.

---

### Task 2: Rewrite `tiffraw.Base.HashableReader` to use sensor data

**File:** `internal/handler/tiffraw/tiffraw.go`

**What:** Replace the current `HashableReader` implementation that calls `findLargestJPEGPreview` with one that calls the new `findSensorData` function from Task 1.

**New `HashableReader` logic:**

```go
func (b *Base) HashableReader(filePath string) (io.ReadCloser, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("tiffraw: open %q: %w", filePath, err)
    }

    sensor, err := findSensorData(f)
    if err != nil || sensor == nil {
        // Fallback: hash the full file.
        if _, err := f.Seek(0, io.SeekStart); err != nil {
            _ = f.Close()
            return nil, fmt.Errorf("tiffraw: seek %q: %w", filePath, err)
        }
        return f, nil
    }

    // Return a reader that streams all sensor data strips/tiles in order.
    return newMultiSectionReader(f, sensor.offsets, sensor.byteCounts), nil
}
```

**Multi-section reader:** The sensor data may span multiple strips or tiles at non-contiguous file offsets. Implement a `multiSectionReader` that wraps the file handle and reads each strip/tile in sequence, presenting them as a single contiguous stream to the hasher. This is analogous to `io.MultiReader` but backed by `io.SectionReader` segments over the same file.

```go
type multiSectionReader struct {
    file    *os.File
    readers []io.Reader
    multi   io.Reader
}

func newMultiSectionReader(f *os.File, offsets, byteCounts []int64) *multiSectionReader {
    readers := make([]io.Reader, len(offsets))
    for i := range offsets {
        readers[i] = io.NewSectionReader(f, offsets[i], byteCounts[i])
    }
    return &multiSectionReader{
        file:    f,
        readers: readers,
        multi:   io.MultiReader(readers...),
    }
}

func (m *multiSectionReader) Read(p []byte) (int, error) { return m.multi.Read(p) }
func (m *multiSectionReader) Close() error               { return m.file.Close() }
```

**Cleanup:** The `findLargestJPEGPreview` function, `jpegPreview` type, and `sectionReadCloser` type can be removed if no other code references them. The `sectionReadCloser` may still be useful — evaluate whether `multiSectionReader` fully replaces it.

**Fallback behavior preserved:** If `findSensorData` returns `nil` (no sensor data IFD found), the handler falls back to hashing the full file, exactly as the current JPEG preview fallback works.

---

### Task 3: Implement `findCR3SensorData` in `cr3` package

**File:** `internal/handler/cr3/cr3.go`

**What:** Add a new function `findCR3SensorData(r io.ReadSeeker) (*sensorRegion, error)` that navigates the ISOBMFF box structure to locate the raw sensor data within the `mdat` box.

**CR3 sensor data location strategy:**

CR3 files store raw sensor data in the `mdat` box, referenced by track metadata in the `moov` box. The approach:

1. Walk top-level boxes to find `moov` and `mdat`.
2. Within `moov`, walk child boxes to find `trak` boxes.
3. Within each `trak`, look for `mdia` → `minf` → `stbl` → `stsz` (sample sizes) and `stco`/`co64` (chunk offsets) to determine the byte ranges of the raw image data within `mdat`.
4. The primary image track (largest total sample size) contains the sensor data.
5. Return the offset and size of the sensor data region within `mdat`.

**Fallback:** If the track metadata cannot be parsed to isolate the sensor data, fall back to returning the entire `mdat` box contents (excluding the box header). This is a safe fallback — the `mdat` box is predominantly sensor data, with only minor overhead from other tracks.

**Type to add (local to `cr3` package):**

```go
type sensorRegion struct {
    offset int64
    size   int64
}
```

**Reuse existing infrastructure:** The `readBox`, `walkBoxes`, and `isobmffBox` types already handle ISOBMFF box parsing. Extend with helper functions to navigate the `moov` → `trak` → `mdia` → `minf` → `stbl` path.

---

### Task 4: Rewrite `cr3.Handler.HashableReader` to use sensor data

**File:** `internal/handler/cr3/cr3.go`

**What:** Replace the current `HashableReader` implementation that calls `findCR3JpegPreview` with one that calls the new `findCR3SensorData` function from Task 3.

**New `HashableReader` logic:**

```go
func (h *Handler) HashableReader(filePath string) (io.ReadCloser, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("cr3: open %q: %w", filePath, err)
    }

    sensor, err := findCR3SensorData(f)
    if err != nil || sensor == nil {
        // Fallback: hash the full file.
        if _, err := f.Seek(0, io.SeekStart); err != nil {
            _ = f.Close()
            return nil, fmt.Errorf("cr3: seek %q: %w", filePath, err)
        }
        return f, nil
    }

    sr := io.NewSectionReader(f, sensor.offset, sensor.size)
    return &sectionReadCloser{Reader: sr, Closer: f}, nil
}
```

**Cleanup:** The `findCR3JpegPreview`, `findLargestJPEGInData`, and `findJPEGEnd` functions can be removed if no other code references them.

**Fallback behavior preserved:** If `findCR3SensorData` returns `nil`, the handler falls back to hashing the full file.

---

### Task 5: Update `tiffraw` package doc comment

**File:** `internal/handler/tiffraw/tiffraw.go`

**What:** Update the package-level doc comment (lines 15-44) to reflect the new hashable region strategy. Replace the "Hashable region" section:

**From:**
> Extracts the embedded full-resolution JPEG preview image. TIFF-based RAW files store this in a secondary IFD...

**To:**
> Extracts the raw sensor data payload. TIFF-based RAW files store sensor data in a primary IFD with a non-JPEG compression scheme (uncompressed, lossless JPEG, or vendor-specific). The handler navigates the IFD chain, distinguishes sensor data IFDs from JPEG preview IFDs by compression type and image dimensions, and returns a reader over the sensor data strips/tiles. Falls back to full-file hash if the sensor data cannot be extracted.

---

### Task 6: Update `cr3` package doc comment

**File:** `internal/handler/cr3/cr3.go`

**What:** Update the package-level doc comment (lines 15-42) to reflect the new hashable region strategy. Replace the "Hashable region" section:

**From:**
> The embedded full-resolution JPEG preview is extracted from the ISOBMFF container by scanning for JPEG SOI markers...

**To:**
> The raw sensor data is extracted from the ISOBMFF container. CR3 stores sensor data in the mdat box, referenced by track metadata in the moov box. The handler navigates the box structure to locate the primary image track and returns a reader over the sensor data region. Falls back to full-file hash if extraction fails.

---

### Task 7: Update `tiffraw_test.go` — sensor data extraction tests

**File:** `internal/handler/tiffraw/tiffraw_test.go`

**What:** Update existing tests and add new tests for the sensor data extraction logic.

**Tests to update:**

- `TestBase_HashableReader_withJPEGPreview` → Rename to `TestBase_HashableReader_withSensorData`. Build a test TIFF fixture that contains a sensor data IFD (with `Compression != 6`, `StripOffsets`, `StripByteCounts`) and verify that `HashableReader` returns the sensor data bytes (not the JPEG preview bytes).
- `TestBase_HashableReader_fullFileFallback` → Keep as-is. A minimal TIFF with no sensor data IFD should still fall back to full-file hash.

**New tests to add:**

- `TestBase_HashableReader_prefersNonJPEGCompression` — Build a TIFF with both a JPEG preview IFD (`Compression=6`) and a sensor data IFD (`Compression=7` or `Compression=1`). Verify `HashableReader` returns the sensor data, not the JPEG preview.
- `TestBase_HashableReader_tiledSensorData` — Build a TIFF with `TileOffsets`/`TileByteCounts` instead of strips. Verify the reader streams all tiles.
- `TestBase_HashableReader_multipleStrips` — Build a TIFF with multiple strip offsets/byte counts. Verify the reader concatenates all strips in order.
- `TestFindSensorData_noSensorIFD` — Verify `findSensorData` returns `nil` for a TIFF with only JPEG preview IFDs.

**Test fixture helpers:** Add `buildTIFFWithSensorData(t, dir, name)` that constructs a minimal TIFF with:
- IFD0 containing a sensor data entry: `Compression=7`, `StripOffsets`, `StripByteCounts` pointing to known bytes.
- IFD1 containing a JPEG preview: `Compression=6`, `JPEGInterchangeFormat`, `JPEGInterchangeFormatLength`.
- Known sensor data bytes at the strip offset (e.g., `0xDE, 0xAD, 0xBE, 0xEF` repeated) so the test can verify exact content.

---

### Task 8: Update `cr3_test.go` — sensor data extraction tests

**File:** `internal/handler/cr3/cr3_test.go`

**What:** Update existing tests and add new tests for the sensor data extraction logic.

**Tests to update:**

- `TestHandler_HashableReader_returnsData` → Verify that the returned data corresponds to the `mdat` sensor data region, not a JPEG blob.

**New tests to add:**

- `TestHandler_HashableReader_returnsSensorData` — Build a CR3 fixture with a `moov` box containing track metadata and an `mdat` box with known sensor data bytes. Verify `HashableReader` returns the sensor data region.
- `TestFindCR3SensorData_noMdat` — Verify `findCR3SensorData` returns `nil` for a CR3 with no `mdat` box.
- `TestFindCR3SensorData_fallbackFullMdat` — Verify that when track metadata cannot be parsed, the function falls back to the full `mdat` contents.

**Test fixture helpers:** Update `buildFakeCR3(t, dir, name)` to include known sensor data bytes in the `mdat` box (not JPEG SOI markers) so the test can verify the correct region is returned.

---

### Task 9: Run full test suite and fix regressions

**Agent:** @tester

**What:** After Tasks 1-8 are complete, run the full test suite to verify no regressions:

```bash
make test-all    # all tests including integration, with -race
make lint        # golangci-lint
```

**Expected areas of impact:**

- `internal/handler/tiffraw/` — direct changes
- `internal/handler/cr3/` — direct changes
- `internal/handler/dng/`, `nef/`, `cr2/`, `pef/`, `arw/` — these embed `tiffraw.Base`, so their `HashableReader` tests will exercise the new code path. Their test fixtures use `buildMinimalTIFF` from their own test files (which delegate to `tiffraw` test helpers or build their own). Verify these still pass.
- `internal/pipeline/` — pipeline tests that process RAW files through the full pipeline. The hash values will change (sensor data vs JPEG preview), so any hardcoded expected checksums in pipeline tests need updating.
- `internal/copy/` — uses `HashableReader` for verify. Should work unchanged since the interface is the same.
- `internal/verify/` — walks `dirB` and recomputes hashes via `HashableReader`. Same interface, should work unchanged.
- `internal/integration/` — end-to-end tests. If any use RAW fixtures with expected checksums, those will need updating.

---

### Task 10: Run lint and format checks

**File:** All modified files

**What:** Run the pre-commit gate to ensure code style compliance:

```bash
make check    # fmt-check + vet + unit tests
```

Ensure all modified files have the Apache 2.0 copyright header, correct import grouping (stdlib / external / internal), and pass `golangci-lint`.
