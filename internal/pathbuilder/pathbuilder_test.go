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
	"testing"
	"time"
)

const testChecksum = "7d97e98f8af710c7e7fe703abc8f639e0ee507c4"

func date(year, month, day, hour, min, sec int) time.Time {
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
}

func TestBuild_normalPath(t *testing.T) {
	d := date(2021, 12, 25, 6, 22, 23)
	got := Build(d, testChecksum, ".jpg", false, "")
	want := filepath.Join("2021", "12", "20211225_062223_"+testChecksum+".jpg")
	if got != want {
		t.Errorf("Build normal:\n  got  %q\n  want %q", got, want)
	}
}

func TestBuild_duplicatePath(t *testing.T) {
	d := date(2021, 12, 25, 6, 22, 23)
	got := Build(d, testChecksum, ".jpg", true, "20260306_103000")
	want := filepath.Join("duplicates", "20260306_103000", "2021", "12", "20211225_062223_"+testChecksum+".jpg")
	if got != want {
		t.Errorf("Build duplicate:\n  got  %q\n  want %q", got, want)
	}
}

func TestBuild_defaultDate_anselsAdams(t *testing.T) {
	// Files with no EXIF date fall back to Ansel Adams' birthday: 1902-02-20.
	d := date(1902, 2, 20, 0, 0, 0)
	got := Build(d, testChecksum, ".jpg", false, "")
	want := filepath.Join("1902", "2", "19020220_000000_"+testChecksum+".jpg")
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

func TestBuild_monthNotZeroPadded(t *testing.T) {
	cases := []struct {
		month          int
		wantDir        string
		wantInFilename string
	}{
		{1, "1", "01"},
		{2, "2", "02"},
		{9, "9", "09"},
		{10, "10", "10"},
		{12, "12", "12"},
	}
	for _, tc := range cases {
		d := date(2022, tc.month, 5, 0, 0, 0)
		got := Build(d, testChecksum, ".jpg", false, "")
		// Directory component should be non-zero-padded.
		parts := splitPath(got)
		if len(parts) < 2 {
			t.Fatalf("unexpected path structure: %q", got)
		}
		if parts[1] != tc.wantDir {
			t.Errorf("month %d: directory = %q, want %q", tc.month, parts[1], tc.wantDir)
		}
		// Month in filename is zero-padded (part of YYYYMMDD).
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
