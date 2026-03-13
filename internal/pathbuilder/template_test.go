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
	"strings"
	"testing"
	"time"

	"golang.org/x/text/language"
)

// ---------------------------------------------------------------------------
// ParseTemplate — valid inputs
// ---------------------------------------------------------------------------

func TestParseTemplate_default(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseTemplate(DefaultTemplate)
	if err != nil {
		t.Fatalf("ParseTemplate(DefaultTemplate) error: %v", err)
	}
	if tmpl == nil {
		t.Fatal("ParseTemplate returned nil template without error")
	}
	if tmpl.String() != DefaultTemplate {
		t.Errorf("String() = %q, want %q", tmpl.String(), DefaultTemplate)
	}
}

func TestParseTemplate_allTokens(t *testing.T) {
	t.Parallel()
	raw := "{year}/{month}/{monthname}/{day}/{hour}/{minute}/{second}/{ext}"
	tmpl, err := ParseTemplate(raw)
	if err != nil {
		t.Fatalf("ParseTemplate(all tokens) error: %v", err)
	}
	if tmpl == nil {
		t.Fatal("ParseTemplate returned nil template without error")
	}
}

func TestParseTemplate_literalOnly(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseTemplate("archive")
	if err != nil {
		t.Fatalf("ParseTemplate(literal) error: %v", err)
	}
	d := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	got := tmpl.Expand(d, "jpg")
	if got != "archive" {
		t.Errorf("Expand literal-only = %q, want %q", got, "archive")
	}
}

func TestParseTemplate_mixedLiteralAndToken(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseTemplate("photos/{year}/raw")
	if err != nil {
		t.Fatalf("ParseTemplate error: %v", err)
	}
	d := time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)
	got := tmpl.Expand(d, "jpg")
	if got != "photos/2021/raw" {
		t.Errorf("Expand = %q, want %q", got, "photos/2021/raw")
	}
}

// ---------------------------------------------------------------------------
// ParseTemplate — invalid inputs
// ---------------------------------------------------------------------------

func TestParseTemplate_empty(t *testing.T) {
	t.Parallel()
	_, err := ParseTemplate("")
	if err == nil {
		t.Fatal("ParseTemplate(\"\") expected error, got nil")
	}
}

func TestParseTemplate_unknownToken(t *testing.T) {
	t.Parallel()
	_, err := ParseTemplate("{year}/{foo}")
	if err == nil {
		t.Fatal("ParseTemplate with unknown token expected error, got nil")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error message should mention the unknown token name; got: %v", err)
	}
}

func TestParseTemplate_unclosedBrace(t *testing.T) {
	t.Parallel()
	_, err := ParseTemplate("{year/{month}")
	if err == nil {
		t.Fatal("ParseTemplate with unclosed brace expected error, got nil")
	}
}

func TestParseTemplate_strayClosingBrace(t *testing.T) {
	t.Parallel()
	_, err := ParseTemplate("{year}}")
	if err == nil {
		t.Fatal("ParseTemplate with stray } expected error, got nil")
	}
}

func TestParseTemplate_emptyToken(t *testing.T) {
	t.Parallel()
	_, err := ParseTemplate("{year}/{}")
	if err == nil {
		t.Fatal("ParseTemplate with empty {} expected error, got nil")
	}
}

func TestParseTemplate_pathTraversal(t *testing.T) {
	t.Parallel()
	cases := []string{
		"{year}/../{month}",
		"../backup",
		"{year}/./sub",
	}
	for _, raw := range cases {
		_, err := ParseTemplate(raw)
		if err == nil {
			t.Errorf("ParseTemplate(%q) expected error for path traversal, got nil", raw)
		}
	}
}

func TestParseTemplate_absolutePath(t *testing.T) {
	t.Parallel()
	_, err := ParseTemplate("/{year}")
	if err == nil {
		t.Fatal("ParseTemplate with absolute path expected error, got nil")
	}
}

func TestParseTemplate_invalidChars(t *testing.T) {
	t.Parallel()
	cases := []string{
		"{year}:backup",
		"{year}*{month}",
		`{year}?{month}`,
		`{year}"name"`,
		"{year}<{month}",
		"{year}>{month}",
		"{year}|{month}",
	}
	for _, raw := range cases {
		_, err := ParseTemplate(raw)
		if err == nil {
			t.Errorf("ParseTemplate(%q) expected error for invalid char, got nil", raw)
		}
	}
}

func TestParseTemplate_nullByte(t *testing.T) {
	t.Parallel()
	_, err := ParseTemplate("{year}\x00{month}")
	if err == nil {
		t.Fatal("ParseTemplate with null byte expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func TestExpand_default(t *testing.T) {
	// Locale pinned to English by TestMain.
	t.Parallel()
	tmpl, err := ParseTemplate(DefaultTemplate)
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	d := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	got := tmpl.Expand(d, "jpg")
	want := "2021/12-Dec"
	if got != want {
		t.Errorf("Expand default = %q, want %q", got, want)
	}
}

func TestExpand_dayGranularity(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseTemplate("{year}/{month}/{day}")
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	d := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	got := tmpl.Expand(d, "jpg")
	want := "2021/12/25"
	if got != want {
		t.Errorf("Expand day granularity = %q, want %q", got, want)
	}
}

func TestExpand_withExt(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseTemplate("{year}/{ext}")
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	d := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	got := tmpl.Expand(d, "jpg")
	want := "2021/jpg"
	if got != want {
		t.Errorf("Expand with ext = %q, want %q", got, want)
	}
}

func TestExpand_allDateComponents(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseTemplate("{year}-{month}-{day}T{hour}{minute}{second}")
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	d := time.Date(2021, 3, 5, 9, 7, 4, 0, time.UTC)
	got := tmpl.Expand(d, "jpg")
	want := "2021-03-05T090704"
	if got != want {
		t.Errorf("Expand all date components = %q, want %q", got, want)
	}
}

func TestExpand_localeAware(t *testing.T) {
	// Not parallel — modifies package-level locale.
	SetLocaleForTesting(language.French)
	defer SetLocaleForTesting(language.English)

	tmpl, err := ParseTemplate("{year}/{month}-{monthname}")
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	d := time.Date(2021, 12, 25, 0, 0, 0, 0, time.UTC)
	got := tmpl.Expand(d, "jpg")
	want := "2021/12-Déc"
	if got != want {
		t.Errorf("Expand French locale = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// String
// ---------------------------------------------------------------------------

func TestTemplate_String(t *testing.T) {
	t.Parallel()
	cases := []string{
		DefaultTemplate,
		"{year}/{month}/{day}",
		"{year}",
		"archive/{year}/{ext}",
	}
	for _, raw := range cases {
		tmpl, err := ParseTemplate(raw)
		if err != nil {
			t.Fatalf("ParseTemplate(%q): %v", raw, err)
		}
		if tmpl.String() != raw {
			t.Errorf("String() = %q, want %q", tmpl.String(), raw)
		}
	}
}

// ---------------------------------------------------------------------------
// ParseCopyrightTemplate — valid inputs
// ---------------------------------------------------------------------------

func TestParseCopyrightTemplate_valid(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseCopyrightTemplate("Copyright {year} My Family")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl == nil {
		t.Fatal("expected non-nil template")
	}
}

func TestParseCopyrightTemplate_allTokens(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("{year}-{month}-{monthname}-{day}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCopyrightTemplate_literalOnly(t *testing.T) {
	t.Parallel()
	tmpl, err := ParseCopyrightTemplate("All Rights Reserved")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	date := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	if got := tmpl.Expand(date); got != "All Rights Reserved" {
		t.Errorf("Expand() = %q, want %q", got, "All Rights Reserved")
	}
}

// ---------------------------------------------------------------------------
// ParseCopyrightTemplate — invalid inputs
// ---------------------------------------------------------------------------

func TestParseCopyrightTemplate_emptyString(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("")
	if err == nil {
		t.Fatal("expected error for empty template")
	}
}

func TestParseCopyrightTemplate_unknownToken_hour(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("Copyright {hour} Family")
	if err == nil {
		t.Fatal("expected error for {hour} token (not in copyright set)")
	}
	if !strings.Contains(err.Error(), "{hour}") {
		t.Errorf("error should mention the bad token, got: %v", err)
	}
}

func TestParseCopyrightTemplate_unknownToken_ext(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("{ext}")
	if err == nil {
		t.Fatal("expected error for {ext} token (not in copyright set)")
	}
}

func TestParseCopyrightTemplate_unknownToken_second(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("{second}")
	if err == nil {
		t.Fatal("expected error for {second} token (not in copyright set)")
	}
}

func TestParseCopyrightTemplate_unclosedBrace(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("Copyright {year Family")
	if err == nil {
		t.Fatal("expected error for unclosed brace")
	}
}

func TestParseCopyrightTemplate_strayCloseBrace(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("Copyright year} Family")
	if err == nil {
		t.Fatal("expected error for stray closing brace")
	}
}

func TestParseCopyrightTemplate_emptyToken(t *testing.T) {
	t.Parallel()
	_, err := ParseCopyrightTemplate("Copyright {} Family")
	if err == nil {
		t.Fatal("expected error for empty token {}")
	}
}

// ---------------------------------------------------------------------------
// CopyrightTemplate.Expand
// ---------------------------------------------------------------------------

func TestCopyrightTemplate_Expand_year(t *testing.T) {
	// Not parallel — calls SetLocaleForTesting which modifies package-level locale.
	SetLocaleForTesting(language.English)
	tmpl, err := ParseCopyrightTemplate("Copyright {year} My Family")
	if err != nil {
		t.Fatalf("ParseCopyrightTemplate: %v", err)
	}
	date := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	got := tmpl.Expand(date)
	want := "Copyright 2021 My Family"
	if got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
}

func TestCopyrightTemplate_Expand_allTokens(t *testing.T) {
	// Not parallel — calls SetLocaleForTesting which modifies package-level locale.
	SetLocaleForTesting(language.English)
	tmpl, err := ParseCopyrightTemplate("{year}/{month}/{monthname}/{day}")
	if err != nil {
		t.Fatalf("ParseCopyrightTemplate: %v", err)
	}
	date := time.Date(2021, 12, 25, 6, 22, 23, 0, time.UTC)
	got := tmpl.Expand(date)
	want := "2021/12/Dec/25"
	if got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
}

func TestCopyrightTemplate_Expand_multipleYearRefs(t *testing.T) {
	// Not parallel — calls SetLocaleForTesting which modifies package-level locale.
	SetLocaleForTesting(language.English)
	tmpl, err := ParseCopyrightTemplate("© {year}-{year} Family")
	if err != nil {
		t.Fatalf("ParseCopyrightTemplate: %v", err)
	}
	date := time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)
	got := tmpl.Expand(date)
	want := "© 2021-2021 Family"
	if got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
}

func TestCopyrightTemplate_String(t *testing.T) {
	t.Parallel()
	raw := "Copyright {year} Family"
	tmpl, err := ParseCopyrightTemplate(raw)
	if err != nil {
		t.Fatalf("ParseCopyrightTemplate: %v", err)
	}
	if tmpl.String() != raw {
		t.Errorf("String() = %q, want %q", tmpl.String(), raw)
	}
}
