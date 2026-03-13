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

package cmd

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// formatDuration tests
// ---------------------------------------------------------------------------

func TestFormatDuration_nilFinished(t *testing.T) {
	t.Parallel()
	started := time.Now()
	got := formatDuration(started, nil)
	if got != "—" {
		t.Errorf("formatDuration(nil) = %q, want %q", got, "—")
	}
}

func TestFormatDuration_subSecond(t *testing.T) {
	t.Parallel()
	started := time.Now()
	finished := started.Add(800 * time.Millisecond)
	got := formatDuration(started, &finished)
	// Should be "0.8s"
	if got != "0.8s" {
		t.Errorf("formatDuration(0.8s) = %q, want %q", got, "0.8s")
	}
}

func TestFormatDuration_seconds(t *testing.T) {
	t.Parallel()
	started := time.Now()
	finished := started.Add(23 * time.Second)
	got := formatDuration(started, &finished)
	if got != "23s" {
		t.Errorf("formatDuration(23s) = %q, want %q", got, "23s")
	}
}

func TestFormatDuration_minutes(t *testing.T) {
	t.Parallel()
	started := time.Now()
	finished := started.Add(1*time.Minute + 23*time.Second)
	got := formatDuration(started, &finished)
	if got != "1m 23s" {
		t.Errorf("formatDuration(1m23s) = %q, want %q", got, "1m 23s")
	}
}

func TestFormatDuration_hours(t *testing.T) {
	t.Parallel()
	started := time.Now()
	finished := started.Add(1*time.Hour + 5*time.Minute + 12*time.Second)
	got := formatDuration(started, &finished)
	if got != "1h 5m 12s" {
		t.Errorf("formatDuration(1h5m12s) = %q, want %q", got, "1h 5m 12s")
	}
}

func TestFormatDuration_exactMinute(t *testing.T) {
	t.Parallel()
	started := time.Now()
	finished := started.Add(2 * time.Minute)
	got := formatDuration(started, &finished)
	if got != "2m 0s" {
		t.Errorf("formatDuration(2m0s) = %q, want %q", got, "2m 0s")
	}
}

func TestFormatDurationSeconds_nil(t *testing.T) {
	t.Parallel()
	started := time.Now()
	got := formatDurationSeconds(started, nil)
	if got != nil {
		t.Errorf("formatDurationSeconds(nil) = %v, want nil", got)
	}
}

func TestFormatDurationSeconds_value(t *testing.T) {
	t.Parallel()
	started := time.Now()
	finished := started.Add(83 * time.Second)
	got := formatDurationSeconds(started, &finished)
	if got == nil {
		t.Fatal("formatDurationSeconds returned nil, want non-nil")
	}
	if *got < 82.9 || *got > 83.1 {
		t.Errorf("formatDurationSeconds = %v, want ~83.0", *got)
	}
}

// ---------------------------------------------------------------------------
// truncChecksum / truncID ellipsis tests
// ---------------------------------------------------------------------------

func TestTruncChecksum_short(t *testing.T) {
	t.Parallel()
	// Values <= 8 chars should not get ellipsis.
	cases := []struct{ in, want string }{
		{"", "—"},
		{"abc", "abc"},
		{"12345678", "12345678"},
	}
	for _, tc := range cases {
		got := truncChecksum(tc.in)
		if got != tc.want {
			t.Errorf("truncChecksum(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncChecksum_long(t *testing.T) {
	t.Parallel()
	// Values > 8 chars should be truncated with ellipsis.
	got := truncChecksum("7d97e98f1234567890abcdef")
	want := "7d97e98f…"
	if got != want {
		t.Errorf("truncChecksum(long) = %q, want %q", got, want)
	}
}

func TestTruncID_short(t *testing.T) {
	t.Parallel()
	got := truncID("abcd1234")
	if got != "abcd1234" {
		t.Errorf("truncID(8 chars) = %q, want %q", got, "abcd1234")
	}
}

func TestTruncID_long(t *testing.T) {
	t.Parallel()
	got := truncID("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	want := "a1b2c3d4…"
	if got != want {
		t.Errorf("truncID(UUID) = %q, want %q", got, want)
	}
}
