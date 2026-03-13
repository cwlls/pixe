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

package pathbuilder

import (
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"
	"time"
	"unicode/utf8"
)

// Property 1: Determinism — same inputs always produce the same output.
func TestBuild_prop_deterministic(t *testing.T) {
	// Locale is pinned to English by TestMain in pathbuilder_test.go.

	f := func(year int16, month uint8, day uint8, hour uint8, min uint8, sec uint8, checksum string, algoID uint8, ext string) bool {
		y := int(year)%300 + 1900 // 1900–2199
		mo := time.Month(month%12 + 1)
		d := int(day%28 + 1) // 1–28 (always valid for any month)
		h := int(hour % 24)
		mi := int(min % 60)
		s := int(sec % 60)
		aid := int(algoID % 5) // 0–4 (valid algorithm IDs)

		if len(checksum) == 0 || len(ext) == 0 {
			return true // skip degenerate inputs
		}
		if !utf8.ValidString(checksum) || !utf8.ValidString(ext) {
			return true
		}

		captureDate := time.Date(y, mo, d, h, mi, s, 0, time.UTC)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}

		result1 := Build(nil, captureDate, aid, checksum, ext, false, "")
		result2 := Build(nil, captureDate, aid, checksum, ext, false, "")

		return result1 == result2
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 10000}); err != nil {
		t.Error(err)
	}
}

// Property 2: Valid path characters — output never contains filesystem-illegal characters.
// This property applies to valid (hex) checksums only, since real Pixe checksums are
// always hex-encoded digests. Arbitrary Unicode strings are not valid checksums.
func TestBuild_prop_validPathCharacters(t *testing.T) {
	invalidChars := []rune{':', '*', '?', '"', '<', '>', '|', 0}

	f := func(year int16, month uint8, day uint8, checksumIdx uint8, algoID uint8) bool {
		y := int(year)%300 + 1900
		mo := time.Month(month%12 + 1)
		d := int(day%28 + 1)
		aid := int(algoID % 5)

		// Use a fixed set of realistic hex checksums to avoid arbitrary Unicode.
		hexChecksums := []string{
			"7d97e98f8af710c7e7fe703abc8f639e0ee507c4",
			"da39a3ee5e6b4b0d3255bfef95601890afd80709",
			"aabbccddeeff00112233445566778899aabbccdd",
			"0000000000000000000000000000000000000000",
			"ffffffffffffffffffffffffffffffffffffffff",
		}
		checksum := hexChecksums[int(checksumIdx)%len(hexChecksums)]

		captureDate := time.Date(y, mo, d, 12, 0, 0, 0, time.UTC)
		result := Build(nil, captureDate, aid, checksum, ".jpg", false, "")

		for _, c := range invalidChars {
			if strings.ContainsRune(result, c) {
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 10000}); err != nil {
		t.Error(err)
	}
}

// Property 3: Correct structure — output is always YYYY/MM-Mon/filename.ext
// with exactly 2 directory levels.
func TestBuild_prop_correctStructure(t *testing.T) {
	f := func(year int16, month uint8, day uint8, hour uint8, min uint8, sec uint8, algoID uint8) bool {
		y := int(year)%300 + 1900
		mo := time.Month(month%12 + 1)
		d := int(day%28 + 1)
		h := int(hour % 24)
		mi := int(min % 60)
		s := int(sec % 60)
		aid := int(algoID % 5)

		captureDate := time.Date(y, mo, d, h, mi, s, 0, time.UTC)
		result := Build(nil, captureDate, aid, "abcdef1234567890", ".jpg", false, "")

		// Must have exactly 2 path separators (YYYY/MM-Mon/filename).
		parts := strings.Split(filepath.ToSlash(result), "/")
		if len(parts) != 3 {
			return false
		}

		// Year directory is 4 digits.
		if len(parts[0]) != 4 {
			return false
		}

		// Month directory matches MM-Mon pattern (2 digits, hyphen, 3+ chars).
		if len(parts[1]) < 6 || parts[1][2] != '-' {
			return false
		}

		// Filename contains the extension.
		filename := parts[2]
		return strings.HasSuffix(filename, ".jpg")
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 10000}); err != nil {
		t.Error(err)
	}
}

// Property 4: Extension preservation — output extension always matches input extension.
func TestBuild_prop_extensionPreserved(t *testing.T) {
	extensions := []string{
		".jpg", ".jpeg", ".heic", ".png", ".mp4", ".mov",
		".dng", ".nef", ".cr2", ".cr3", ".pef", ".arw", ".orf", ".rw2",
		".tif", ".tiff", ".avif",
	}

	f := func(year int16, month uint8, day uint8, extIdx uint8) bool {
		ext := extensions[int(extIdx)%len(extensions)]
		y := int(year)%300 + 1900
		mo := time.Month(month%12 + 1)
		d := int(day%28 + 1)

		captureDate := time.Date(y, mo, d, 12, 0, 0, 0, time.UTC)
		result := Build(nil, captureDate, 1, "abcdef1234567890", ext, false, "")

		return strings.HasSuffix(result, ext)
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Error(err)
	}
}

// Property 5: Date encoding — YYYYMMDD_HHMMSS prefix in the filename matches
// the input date exactly.
func TestBuild_prop_dateEncoding(t *testing.T) {
	f := func(year int16, month uint8, day uint8, hour uint8, min uint8, sec uint8) bool {
		y := int(year)%300 + 1900
		mo := time.Month(month%12 + 1)
		d := int(day%28 + 1)
		h := int(hour % 24)
		mi := int(min % 60)
		s := int(sec % 60)

		captureDate := time.Date(y, mo, d, h, mi, s, 0, time.UTC)
		result := Build(nil, captureDate, 1, "abcdef1234567890", ".jpg", false, "")

		// Extract the YYYYMMDD_HHMMSS prefix from the filename.
		parts := strings.Split(filepath.ToSlash(result), "/")
		if len(parts) < 1 {
			return false
		}
		filename := parts[len(parts)-1]
		if len(filename) < 15 {
			return false
		}
		dateStr := filename[:15] // YYYYMMDD_HHMMSS

		expected := captureDate.Format("20060102_150405")
		return dateStr == expected
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 10000}); err != nil {
		t.Error(err)
	}
}

// Property 6: Algorithm ID presence — filename contains the correct algorithm
// ID at the correct position (after the datetime, before the checksum).
func TestBuild_prop_algorithmIDPresent(t *testing.T) {
	f := func(year int16, algoID uint8) bool {
		aid := int(algoID % 5)
		y := int(year)%300 + 1900
		captureDate := time.Date(y, time.June, 15, 12, 0, 0, 0, time.UTC)
		result := Build(nil, captureDate, aid, "abcdef1234567890", ".jpg", false, "")

		parts := strings.Split(filepath.ToSlash(result), "/")
		if len(parts) < 1 {
			return false
		}
		filename := parts[len(parts)-1]

		// Filename format: YYYYMMDD_HHMMSS-<algoID>-<checksum>.ext
		// Position 15 = '-', position 16 = algoID digit, position 17 = '-'.
		if len(filename) < 18 {
			return false
		}
		if filename[15] != '-' {
			return false
		}
		expectedDigit := byte('0' + aid)
		if filename[16] != expectedDigit {
			return false
		}
		if filename[17] != '-' {
			return false
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 5000}); err != nil {
		t.Error(err)
	}
}
