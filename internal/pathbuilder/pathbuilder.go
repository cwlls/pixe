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

// Package pathbuilder constructs deterministic output paths for sorted media
// files using the Pixe naming convention:
//
//	<YYYY>/<M>/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
//
// Duplicate files are routed under a timestamped subdirectory:
//
//	duplicates/<runTimestamp>/<YYYY>/<M>/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>
//
// Month is intentionally non-zero-padded (e.g. "2" not "02") to keep
// directory names concise. The file extension is always lowercased.
package pathbuilder

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const duplicatesDir = "duplicates"

// Build returns the relative output path for a media file.
//
// Parameters:
//   - date:         capture date/time extracted from the file's metadata.
//   - checksum:     hex-encoded media payload hash (e.g. 40-char SHA-1).
//   - ext:          file extension including the leading dot (e.g. ".JPG").
//     It is lowercased automatically.
//   - isDuplicate:  when true the path is rooted under duplicates/<runTimestamp>/.
//   - runTimestamp: the sort run's start time formatted as "YYYYMMDD_HHMMSS",
//     used only when isDuplicate is true.
//
// Example outputs:
//
//	Build(t, sha, ".jpg", false, "") → "2021/12/20211225_062223_<sha>.jpg"
//	Build(t, sha, ".JPG", true, "20260306_103000") → "duplicates/20260306_103000/2021/12/20211225_062223_<sha>.jpg"
func Build(date time.Time, checksum string, ext string, isDuplicate bool, runTimestamp string) string {
	ext = strings.ToLower(ext)

	// Non-zero-padded month per spec.
	year := date.Year()
	month := int(date.Month())

	filename := fmt.Sprintf("%04d%02d%02d_%02d%02d%02d_%s%s",
		year, month, date.Day(),
		date.Hour(), date.Minute(), date.Second(),
		checksum, ext,
	)

	relPath := filepath.Join(fmt.Sprintf("%d", year), fmt.Sprintf("%d", month), filename)

	if isDuplicate {
		relPath = filepath.Join(duplicatesDir, runTimestamp, relPath)
	}

	return relPath
}

// RunTimestamp formats t as the canonical run-timestamp string used in
// duplicate directory names: "YYYYMMDD_HHMMSS".
func RunTimestamp(t time.Time) string {
	return fmt.Sprintf("%04d%02d%02d_%02d%02d%02d",
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second(),
	)
}
