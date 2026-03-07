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

package domain

import "testing"

func TestFileStatusString(t *testing.T) {
	cases := []struct {
		status FileStatus
		want   string
	}{
		{StatusPending, "pending"},
		{StatusExtracted, "extracted"},
		{StatusHashed, "hashed"},
		{StatusCopied, "copied"},
		{StatusVerified, "verified"},
		{StatusTagged, "tagged"},
		{StatusComplete, "complete"},
		{StatusFailed, "failed"},
		{StatusMismatch, "mismatch"},
		{StatusTagFailed, "tag_failed"},
	}
	for _, tc := range cases {
		if got := tc.status.String(); got != tc.want {
			t.Errorf("FileStatus(%q).String() = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestFileStatusIsTerminal(t *testing.T) {
	terminal := []FileStatus{StatusComplete, StatusFailed, StatusMismatch, StatusTagFailed}
	nonTerminal := []FileStatus{StatusPending, StatusExtracted, StatusHashed, StatusCopied, StatusVerified, StatusTagged}

	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("expected %q to be non-terminal", s)
		}
	}
}

func TestFileStatusIsError(t *testing.T) {
	errStatuses := []FileStatus{StatusFailed, StatusMismatch, StatusTagFailed}
	okStatuses := []FileStatus{StatusPending, StatusExtracted, StatusHashed, StatusCopied, StatusVerified, StatusTagged, StatusComplete}

	for _, s := range errStatuses {
		if !s.IsError() {
			t.Errorf("expected %q to be an error status", s)
		}
	}
	for _, s := range okStatuses {
		if s.IsError() {
			t.Errorf("expected %q to not be an error status", s)
		}
	}
}

func TestMetadataTagsIsEmpty(t *testing.T) {
	if !(MetadataTags{}).IsEmpty() {
		t.Error("zero-value MetadataTags should be empty")
	}
	if (MetadataTags{Copyright: "c"}).IsEmpty() {
		t.Error("MetadataTags with Copyright set should not be empty")
	}
	if (MetadataTags{CameraOwner: "o"}).IsEmpty() {
		t.Error("MetadataTags with CameraOwner set should not be empty")
	}
	if (MetadataTags{Copyright: "c", CameraOwner: "o"}).IsEmpty() {
		t.Error("MetadataTags with both fields set should not be empty")
	}
}
