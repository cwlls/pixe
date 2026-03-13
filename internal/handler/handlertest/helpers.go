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

package handlertest

import (
	"os"
	"path/filepath"
	"testing"
)

// BuildEmptyFile creates a zero-byte file with the given extension in a
// temporary directory and returns its absolute path. The file has the correct
// extension but zero bytes — used to verify that handlers degrade gracefully
// on empty input.
func BuildEmptyFile(t *testing.T, ext string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty"+ext)
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("handlertest.BuildEmptyFile: %v", err)
	}
	return path
}

// BuildMagicOnly creates a file containing only the given magic bytes with the
// given extension. This exercises the edge case where a file has a valid magic
// signature but no further structure — parsers must not panic or infinite-loop.
func BuildMagicOnly(t *testing.T, magic []byte, ext string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "magic_only"+ext)
	if err := os.WriteFile(path, magic, 0o644); err != nil {
		t.Fatalf("handlertest.BuildMagicOnly: %v", err)
	}
	return path
}

// BuildTruncatedFile takes a full valid file's bytes and writes a truncated
// version (at truncateAt bytes) with the given extension. If truncateAt is
// greater than or equal to len(data), the full data is written. Used to verify
// that handlers return errors (not panics) on truncated input.
func BuildTruncatedFile(t *testing.T, data []byte, truncateAt int, ext string) string {
	t.Helper()
	if truncateAt > len(data) {
		truncateAt = len(data)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "truncated"+ext)
	if err := os.WriteFile(path, data[:truncateAt], 0o644); err != nil {
		t.Fatalf("handlertest.BuildTruncatedFile: %v", err)
	}
	return path
}

// BuildWithFilename creates a file from the given bytes with a specific
// filename (including extension) in a temporary directory. Used for Unicode
// filename testing and mismatched-extension testing.
func BuildWithFilename(t *testing.T, data []byte, filename string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("handlertest.BuildWithFilename: %v", err)
	}
	return path
}

// BuildSymlink creates a symbolic link named linkName pointing to targetPath,
// both within a temporary directory. Returns the absolute path of the symlink.
// The symlink is created in the same temp directory as the target.
func BuildSymlink(t *testing.T, targetPath, linkName string) string {
	t.Helper()
	dir := t.TempDir()
	linkPath := filepath.Join(dir, linkName)
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("handlertest.BuildSymlink: %v", err)
	}
	return linkPath
}
