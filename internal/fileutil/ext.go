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

// Package fileutil provides shared file-path utilities used across handler
// and discovery packages.
package fileutil

import "strings"

// Ext returns the file extension including the leading dot, or "" if none.
// Unlike filepath.Ext, this function correctly handles both Unix ("/") and
// Windows ("\") path separators regardless of the OS, and treats "." as a
// directory name (not an extension).
//
// This replaces the hand-rolled fileExt helper that was duplicated across
// handler packages and incorrectly handled Windows paths containing dots in
// directory names.
func Ext(path string) string {
	// Handle empty path and special case "."
	if path == "" || path == "." {
		return ""
	}

	// Normalize path separators: treat both "/" and "\" as separators
	// to handle cross-platform paths correctly.
	lastSlash := strings.LastIndexAny(path, "/\\")
	if lastSlash >= 0 {
		path = path[lastSlash+1:]
	}

	// Now path contains just the filename. Find the last dot.
	lastDot := strings.LastIndex(path, ".")
	if lastDot < 0 {
		// No dot found
		return ""
	}

	// If the dot is at position 0 (hidden file like ".hidden"), return the whole thing
	// This matches filepath.Ext behavior
	return path[lastDot:]
}
