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
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/text/language"
)

const testChecksum = "7d97e98f8af710c7e7fe703abc8f639e0ee507c4"

// TestMain pins the locale to English for all tests in this package so that
// assertions on month directory names are deterministic regardless of the
// developer's system locale.
func TestMain(m *testing.M) {
	SetLocaleForTesting(language.English)
	os.Exit(m.Run())
}

func date(year, month, day, hour, min, sec int) time.Time {
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
}

func TestBuild_normalPath(t *testing.T) {
	d := date(2021, 12, 25, 6, 22, 23)
	got := Build(d, testChecksum, ".jpg", false, "")
	want := filepath.Join("2021", "12-Dec", "20211225_062223_"+testChecksum+".jpg")
	if got != want {
		t.Errorf("Build normal:\n  got  %q\n  want %q", got, want)
	}
}

func TestBuild_duplicatePath(t *testing.T) {
	d := date(2021, 12, 25, 6, 22, 23)
	got := Build(d, testChecksum, ".jpg", true, "20260306_103000")
	want := filepath.Join("duplicates", "20260306_103000", "2021", "12-Dec", "20211225_062223_"+testChecksum+".jpg")
	if got != want {
		t.Errorf("Build duplicate:\n  got  %q\n  want %q", got, want)
	}
}

func TestBuild_defaultDate_anselsAdams(t *testing.T) {
	// Files with no EXIF date fall back to Ansel Adams' birthday: 1902-02-20.
	d := date(1902, 2, 20, 0, 0, 0)
	got := Build(d, testChecksum, ".jpg", false, "")
	want := filepath.Join("1902", "02-Feb", "19020220_000000_"+testChecksum+".jpg")
	if got != want {
		t.Errorf("Build Ansel Adams date:\n  got  %q\n  want %q", got, want)
	}
}

func TestBuild_extensionNormalization(t *testing.T) {
	d := date(2022, 6, 15, 12, 0, 0)
	cases := []struct {
		ext  string
		want string
	}{
		{".JPG", ".jpg"},
		{".JPEG", ".jpeg"},
		{".HEIC", ".heic"},
		{".MP4", ".mp4"},
		{".jpg", ".jpg"},
	}
	for _, tc := range cases {
		got := Build(d, testChecksum, tc.ext, false, "")
		// The extension in the filename should be lowercased.
		if filepath.Ext(got) != tc.want {
			t.Errorf("Build ext %q: got ext %q, want %q (full path: %q)",
				tc.ext, filepath.Ext(got), tc.want, got)
		}
	}
}

func TestBuild_monthDirectoryFormat(t *testing.T) {
	// Locale is already pinned to English by TestMain.
	cases := []struct {
		month          int
		wantDir        string // MM-Mon format
		wantInFilename string // zero-padded numeric
	}{
		{1, "01-Jan", "01"},
		{2, "02-Feb", "02"},
		{9, "09-Sep", "09"},
		{10, "10-Oct", "10"},
		{12, "12-Dec", "12"},
	}
	for _, tc := range cases {
		d := date(2022, tc.month, 5, 0, 0, 0)
		got := Build(d, testChecksum, ".jpg", false, "")
		parts := splitPath(got)
		if len(parts) < 3 {
			t.Fatalf("unexpected path structure: %q", got)
		}
		// parts[0]=year, parts[1]=month-dir, parts[2]=filename
		if parts[1] != tc.wantDir {
			t.Errorf("month %d: directory = %q, want %q", tc.month, parts[1], tc.wantDir)
		}
		// Month in filename is zero-padded numeric (part of YYYYMMDD).
		filename := parts[len(parts)-1]
		monthInFilename := filename[4:6]
		if monthInFilename != tc.wantInFilename {
			t.Errorf("month %d: filename month digits = %q, want %q", tc.month, monthInFilename, tc.wantInFilename)
		}
	}
}

func TestBuild_sameSecondDifferentChecksum(t *testing.T) {
	// Two files taken at the same second with different content → different paths.
	d := date(2022, 3, 1, 10, 0, 0)
	sha1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sha2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	p1 := Build(d, sha1, ".jpg", false, "")
	p2 := Build(d, sha2, ".jpg", false, "")
	if p1 == p2 {
		t.Errorf("same-second different checksums produced identical paths: %q", p1)
	}
}

func TestMonthDir(t *testing.T) {
	// Locale is already pinned to English by TestMain.
	cases := []struct {
		month time.Month
		want  string
	}{
		{time.January, "01-Jan"},
		{time.February, "02-Feb"},
		{time.March, "03-Mar"},
		{time.April, "04-Apr"},
		{time.May, "05-May"},
		{time.June, "06-Jun"},
		{time.July, "07-Jul"},
		{time.August, "08-Aug"},
		{time.September, "09-Sep"},
		{time.October, "10-Oct"},
		{time.November, "11-Nov"},
		{time.December, "12-Dec"},
	}
	for _, tc := range cases {
		got := MonthDir(tc.month)
		if got != tc.want {
			t.Errorf("MonthDir(%v) = %q, want %q", tc.month, got, tc.want)
		}
	}
}

func TestMonthDir_nonEnglishLocale(t *testing.T) {
	SetLocaleForTesting(language.French)
	defer SetLocaleForTesting(language.English) // restore for subsequent tests

	cases := []struct {
		month time.Month
		want  string
	}{
		{time.March, "03-Mar"},
		{time.February, "02-Fév"},
		{time.August, "08-Aoû"},
		{time.December, "12-Déc"},
	}
	for _, tc := range cases {
		got := MonthDir(tc.month)
		if got != tc.want {
			t.Errorf("MonthDir(%v) [fr] = %q, want %q", tc.month, got, tc.want)
		}
	}
}

func TestMonthDir_germanLocale(t *testing.T) {
	SetLocaleForTesting(language.German)
	defer SetLocaleForTesting(language.English)

	cases := []struct {
		month time.Month
		want  string
	}{
		{time.March, "03-Mär"},
		{time.October, "10-Okt"},
		{time.December, "12-Dez"},
	}
	for _, tc := range cases {
		got := MonthDir(tc.month)
		if got != tc.want {
			t.Errorf("MonthDir(%v) [de] = %q, want %q", tc.month, got, tc.want)
		}
	}
}

func TestMonthDir_unsupportedLocale_fallsBackToEnglish(t *testing.T) {
	// Swahili is not in the table — should fall back to English.
	sw, _ := language.Parse("sw")
	SetLocaleForTesting(sw)
	defer SetLocaleForTesting(language.English)

	got := MonthDir(time.January)
	want := "01-Jan"
	if got != want {
		t.Errorf("MonthDir(January) [sw fallback] = %q, want %q", got, want)
	}
}

func TestDetectSystemLocale_fallback(t *testing.T) {
	// When no locale env vars are set, detectSystemLocale should return English.
	for _, k := range []string{"LANGUAGE", "LC_ALL", "LC_TIME", "LANG"} {
		t.Setenv(k, "")
	}

	tag := detectSystemLocale()
	base, _ := tag.Base()
	if base.String() != "en" {
		t.Errorf("detectSystemLocale() with no env vars = %v, want English", tag)
	}
}

func TestDetectSystemLocale_posixLocale(t *testing.T) {
	// "C" and "POSIX" must be skipped; the function should fall back to English.
	for _, k := range []string{"LANGUAGE", "LC_ALL", "LC_TIME", "LANG"} {
		t.Setenv(k, "")
	}

	t.Setenv("LANG", "C")
	tag := detectSystemLocale()
	base, _ := tag.Base()
	if base.String() != "en" {
		t.Errorf("detectSystemLocale() with LANG=C = %v, want English", tag)
	}
}

func TestDetectSystemLocale_posixWithEncoding(t *testing.T) {
	// "fr_FR.UTF-8" should parse to French.
	for _, k := range []string{"LANGUAGE", "LC_ALL", "LC_TIME", "LANG"} {
		t.Setenv(k, "")
	}

	t.Setenv("LANG", "fr_FR.UTF-8")
	tag := detectSystemLocale()
	base, _ := tag.Base()
	if base.String() != "fr" {
		t.Errorf("detectSystemLocale() with LANG=fr_FR.UTF-8 = %v, want French", tag)
	}
}

func TestRunTimestamp(t *testing.T) {
	d := time.Date(2026, 3, 6, 10, 30, 0, 0, time.UTC)
	got := RunTimestamp(d)
	want := "20260306_103000"
	if got != want {
		t.Errorf("RunTimestamp = %q, want %q", got, want)
	}
}

// splitPath splits a filepath into its components.
func splitPath(p string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(p)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == p {
			break
		}
		p = filepath.Clean(dir)
	}
	return parts
}
