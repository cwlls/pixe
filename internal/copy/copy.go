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

// Package copy provides the streamed file copy and post-copy verification
// engine for the Pixe sort pipeline.
//
// Safety model:
//   - Execute streams the source file to the destination in 32 KB chunks,
//     never loading the full file into memory.
//   - Verify re-reads the destination through the filetype handler's
//     HashableReader and recomputes the checksum independently. A mismatch
//     means the copy is corrupt; the destination file is preserved (not
//     deleted) so the user can inspect it.
//   - Parent directories are created automatically by Execute.
//   - The source file's modification time is preserved on the copy via
//     os.Chtimes (informational only — Pixe never uses mtime for dating).
package copy

import (
	"fmt"
	"io"
	"os"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/hash"
)

const copyBufSize = 32 * 1024 // 32 KB

// CopyResult holds the outcome of a copy+verify operation.
type CopyResult struct {
	// Success is true only when the file was copied AND the post-copy
	// verification checksum matched the expected value.
	Success bool

	// Checksum is the hex-encoded media-payload hash computed during Verify.
	// It is populated even on mismatch so callers can log the actual value.
	Checksum string

	// Error carries the first error encountered, or nil on success.
	Error error
}

// Execute streams src to dest in copyBufSize chunks.
//
// It creates all parent directories of dest if they do not exist, sets
// destination permissions to 0644, and preserves the source file's
// modification time on the copy via os.Chtimes.
//
// Execute does NOT verify the copy — call Verify after Execute.
func Execute(src, dest string) error {
	// Stat the source so we can preserve its mtime.
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy: stat source %q: %w", src, err)
	}

	// Create parent directories.
	if err := os.MkdirAll(parentDir(dest), 0o755); err != nil {
		return fmt.Errorf("copy: create parent directories for %q: %w", dest, err)
	}

	// Open source.
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy: open source %q: %w", src, err)
	}
	defer in.Close()

	// Create destination.
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("copy: create destination %q: %w", dest, err)
	}

	// Stream with a fixed-size buffer.
	buf := make([]byte, copyBufSize)
	if _, err := io.CopyBuffer(out, in, buf); err != nil {
		out.Close()
		return fmt.Errorf("copy: stream %q → %q: %w", src, dest, err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("copy: close destination %q: %w", dest, err)
	}

	// Preserve source mtime (informational — not used for date extraction).
	if err := os.Chtimes(dest, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		// Non-fatal: log-worthy but not a reason to fail the copy.
		_ = err
	}

	return nil
}

// Verify re-reads dest through handler.HashableReader, hashes the media
// payload with hasher, and compares the result against expectedChecksum.
//
// On mismatch the destination file is intentionally NOT deleted — it is
// preserved for debugging. The returned CopyResult will have Success=false
// and a descriptive Error.
func Verify(dest, expectedChecksum string, handler domain.FileTypeHandler, hasher *hash.Hasher) CopyResult {
	rc, err := handler.HashableReader(dest)
	if err != nil {
		return CopyResult{
			Error: fmt.Errorf("copy: open hashable reader for %q: %w", dest, err),
		}
	}
	defer rc.Close()

	actual, err := hasher.Sum(rc)
	if err != nil {
		return CopyResult{
			Error: fmt.Errorf("copy: hash destination %q: %w", dest, err),
		}
	}

	if actual != expectedChecksum {
		return CopyResult{
			Checksum: actual,
			Error: fmt.Errorf(
				"copy: checksum mismatch for %q: expected %s, got %s (file preserved for inspection)",
				dest, expectedChecksum, actual,
			),
		}
	}

	return CopyResult{
		Success:  true,
		Checksum: actual,
	}
}

// parentDir returns the directory component of path.
// Uses a manual scan to avoid importing path/filepath in the hot path,
// though filepath.Dir would be equally correct here.
func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
