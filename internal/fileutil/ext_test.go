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

package fileutil

import "testing"

func TestExt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "standard jpg extension",
			path: "/photos/IMG_0001.jpg",
			want: ".jpg",
		},
		{
			name: "no extension",
			path: "/photos/IMG_0001",
			want: "",
		},
		{
			name: "dot in directory unix — the Windows bug",
			path: "/photos/dir.backup/IMG_0001",
			want: "",
		},
		{
			name: "windows path with dot in directory",
			path: `C:\photos\dir.backup\IMG_0001`,
			want: "",
		},
		{
			name: "multiple dots — last wins",
			path: "/photos/file.tar.gz",
			want: ".gz",
		},
		{
			name: "hidden file — dot at start",
			path: "/photos/.hidden",
			want: ".hidden",
		},
		{
			name: "uppercase extension",
			path: "/photos/IMG_0001.JPG",
			want: ".JPG",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
		{
			name: "just a dot",
			path: ".",
			want: "",
		},
		{
			name: "heic extension",
			path: "/photos/IMG_1234.heic",
			want: ".heic",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Ext(tc.path)
			if got != tc.want {
				t.Errorf("Ext(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
