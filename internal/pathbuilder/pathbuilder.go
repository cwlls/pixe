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
//	<dir-template>/YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>
//
// The directory structure is controlled by a Template (see template.go).
// The default template "{year}/{month}-{monthname}" produces:
//
//	<YYYY>/<MM>-<Mon>/YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>
//
// Duplicate files are routed under a timestamped subdirectory:
//
//	duplicates/<runTimestamp>/<dir-template>/YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>
//
// The month directory abbreviation is locale-aware. The file extension is
// always lowercased.
package pathbuilder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/language"
)

// defaultTmpl is the package-level default template, initialised once.
// It is used when Build is called with a nil template.
var defaultTmpl *Template

func init() {
	systemLocale = detectSystemLocale()
	var err error
	defaultTmpl, err = ParseTemplate(DefaultTemplate)
	if err != nil {
		// DefaultTemplate is a compile-time constant — this must never fail.
		panic("pathbuilder: failed to parse DefaultTemplate: " + err.Error())
	}
}

const duplicatesDir = "duplicates"

// systemLocale is the resolved locale tag, detected once at package init.
var systemLocale language.Tag

// detectSystemLocale reads LANGUAGE, LC_ALL, LC_TIME, or LANG from the
// environment and parses the first valid BCP 47 / POSIX locale tag.
// Falls back to language.English if nothing is set or parseable.
func detectSystemLocale() language.Tag {
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_TIME", "LANG"} {
		val := os.Getenv(key)
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}
		// POSIX locales use underscores (e.g. "fr_FR.UTF-8"); strip encoding suffix
		// and normalise to BCP 47 hyphen-separated form.
		val = strings.SplitN(val, ".", 2)[0]
		val = strings.ReplaceAll(val, "_", "-")
		tag, err := language.Parse(val)
		if err == nil {
			return tag
		}
	}
	return language.English
}

// SetLocaleForTesting overrides the detected system locale. This is intended
// for use in tests only — it is not safe for concurrent use.
func SetLocaleForTesting(tag language.Tag) {
	systemLocale = tag
}

// monthAbbreviations maps BCP 47 base language codes to their 12 abbreviated
// month names (title-cased). Sourced from Unicode CLDR.
// Add entries here to support additional locales; English is the fallback.
var monthAbbreviations = map[string][12]string{
	"en": {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
	"fr": {"Jan", "Fév", "Mar", "Avr", "Mai", "Jun", "Jul", "Aoû", "Sep", "Oct", "Nov", "Déc"},
	"de": {"Jan", "Feb", "Mär", "Apr", "Mai", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dez"},
	"es": {"Ene", "Feb", "Mar", "Abr", "May", "Jun", "Jul", "Ago", "Sep", "Oct", "Nov", "Dic"},
	"it": {"Gen", "Feb", "Mar", "Apr", "Mag", "Giu", "Lug", "Ago", "Set", "Ott", "Nov", "Dic"},
	"pt": {"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"},
	"nl": {"Jan", "Feb", "Mrt", "Apr", "Mei", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dec"},
	"ja": {"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
	"zh": {"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
	"ko": {"1월", "2월", "3월", "4월", "5월", "6월", "7월", "8월", "9월", "10월", "11월", "12월"},
	"ru": {"Янв", "Фев", "Мар", "Апр", "Май", "Июн", "Июл", "Авг", "Сен", "Окт", "Ноя", "Дек"},
}

// localizedMonthAbbr returns the title-cased abbreviated month name for the
// given month in the current system locale. Falls back to English when the
// locale is unsupported.
func localizedMonthAbbr(month time.Month) string {
	base, _ := systemLocale.Base()
	if table, ok := monthAbbreviations[base.String()]; ok {
		idx := int(month) - 1
		if idx >= 0 && idx < 12 {
			return table[idx]
		}
	}
	// Fallback: English via time.Month.String() truncated to 3 chars.
	s := month.String()
	if len(s) > 3 {
		s = s[:3]
	}
	return s
}

// MonthDir returns the locale-aware month directory name for the given month.
// Format: zero-padded two-digit number + hyphen + three-letter title-cased
// month abbreviation. Examples (English locale): "01-Jan", "02-Feb", "12-Dec".
//
// The abbreviation is derived from the system locale detected at init time.
// If the locale cannot be determined, English is used as the fallback.
func MonthDir(month time.Month) string {
	return fmt.Sprintf("%02d-%s", int(month), localizedMonthAbbr(month))
}

// Build returns the relative output path for a media file.
//
// Parameters:
//   - tmpl:         directory structure template. Pass nil to use the default
//     template ("{year}/{month}-{monthname}"), which reproduces the pre-template
//     hardcoded structure.
//   - date:         capture date/time extracted from the file's metadata.
//   - algoID:       numeric algorithm ID (see OVERVIEW.md Section 4.5.1).
//     0=md5, 1=sha1, 2=sha256, 3=blake3, 4=xxhash.
//   - checksum:     hex-encoded media payload hash.
//   - ext:          file extension including the leading dot (e.g. ".JPG").
//     It is lowercased automatically.
//   - isDuplicate:  when true the path is rooted under duplicates/<runTimestamp>/.
//   - runTimestamp: the sort run's start time formatted as "YYYYMMDD_HHMMSS",
//     used only when isDuplicate is true.
//
// Example outputs (default template):
//
//	Build(nil, t, 1, sha, ".jpg", false, "") → "2021/12-Dec/20211225_062223-1-<sha>.jpg"
//	Build(nil, t, 1, sha, ".JPG", true, "20260306_103000") → "duplicates/20260306_103000/2021/12-Dec/20211225_062223-1-<sha>.jpg"
func Build(tmpl *Template, date time.Time, algoID int, checksum string, ext string, isDuplicate bool, runTimestamp string) string {
	if tmpl == nil {
		tmpl = defaultTmpl
	}

	ext = strings.ToLower(ext)
	extNoDot := strings.TrimPrefix(ext, ".")

	// Format: YYYYMMDD_HHMMSS-<algoID>-<checksum>.<ext>
	// Underscores belong to the datetime portion; hyphens delimit the integrity segment.
	filename := fmt.Sprintf("%04d%02d%02d_%02d%02d%02d-%d-%s%s",
		date.Year(), int(date.Month()), date.Day(),
		date.Hour(), date.Minute(), date.Second(),
		algoID, checksum, ext,
	)

	dirPath := tmpl.Expand(date, extNoDot)
	relPath := filepath.Join(dirPath, filename)

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
