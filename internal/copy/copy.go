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

// Package copy provides the atomic file copy and post-copy verification
// engine for the Pixe sort pipeline.
//
// Safety model:
//   - Execute streams the source file to a temporary file (.<name>.pixe-tmp)
//     in the same directory as the intended destination, never loading the
//     full file into memory. The temp file is in the same directory so that
//     Promote can use an atomic os.Rename on the same filesystem.
//   - Verify re-reads the temp file through the filetype handler's
//     HashableReader and recomputes the checksum independently.
//   - On verification success, Promote atomically renames the temp file to
//     its canonical destination path. A file at its canonical path is always
//     verified.
//   - On verification failure, CleanupTempFile deletes the temp file. The
//     source in dirA is untouched and can be reprocessed.
//   - Parent directories are created automatically by Execute.
//   - The source file's modification time is preserved on the temp file via
//     os.Chtimes (informational only — Pixe never uses mtime for dating).
package copy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cwlls/pixe-go/internal/domain"
	"github.com/cwlls/pixe-go/internal/hash"
)

const copyBufSize = 32 * 1024 // 32 KB

// CopyResult holds the outcome of a Verify operation.
type CopyResult struct {
	// Success is true only when the post-copy verification checksum matched
	// the expected value.
	Success bool

	// Checksum is the hex-encoded media-payload hash computed during Verify.
	// It is populated even on mismatch so callers can log the actual value.
	Checksum string

	// Error carries the first error encountered, or nil on success.
	Error error
}

// TempPath returns the temporary file path for a given destination.
//
// The temp file is placed in the same directory as dest so that Promote can
// use an atomic os.Rename on the same filesystem. The name follows the
// pattern: <dir>/.<basename>.pixe-tmp
//
// TempPath is used by tests and by callers that need to predict the temp path
// before calling Execute. Execute itself uses os.CreateTemp to generate a
// unique name, so the actual temp path returned by Execute may differ from
// TempPath when multiple workers process files with the same destination.
func TempPath(dest string) string {
	dir := filepath.Dir(dest)
	base := filepath.Base(dest)
	return filepath.Join(dir, "."+base+".pixe-tmp")
}

// Execute streams src to a uniquely-named temporary file in the same directory
// as dest and returns the temp file path. The caller must call Verify on the
// temp file and then either Promote (on success) or CleanupTempFile (on failure).
//
// The temp file is placed in the same directory as dest so that Promote can
// use an atomic os.Rename on the same filesystem. A unique name is generated
// via os.CreateTemp to prevent concurrent workers from overwriting each other's
// temp files when processing files with the same destination path.
//
// Execute creates all parent directories of dest if they do not exist, sets
// temp file permissions to 0644, and preserves the source file's modification
// time on the temp file via os.Chtimes.
func Execute(src, dest string) (tmpPath string, err error) {
	// Stat the source so we can preserve its mtime.
	srcInfo, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("copy: stat source %q: %w", src, err)
	}

	// Create parent directories (for the final dest — temp file shares the dir).
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("copy: create parent directories for %q: %w", dest, err)
	}

	// Open source.
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("copy: open source %q: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	// Create a uniquely-named temp file in the destination directory.
	// Using os.CreateTemp prevents concurrent workers from overwriting each
	// other's temp files when they share the same canonical destination path
	// (e.g. two workers processing identical files in no-DB mode).
	base := filepath.Base(dest)
	out, err := os.CreateTemp(destDir, "."+base+".pixe-tmp-*")
	if err != nil {
		return "", fmt.Errorf("copy: create temp file in %q: %w", destDir, err)
	}
	tmpPath = out.Name()

	// os.CreateTemp creates files with 0600; set to 0644 for archive consistency.
	if err := out.Chmod(0o644); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("copy: chmod temp file %q: %w", tmpPath, err)
	}

	// Stream with a fixed-size buffer.
	buf := make([]byte, copyBufSize)
	if _, err := io.CopyBuffer(out, in, buf); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("copy: stream %q → %q: %w", src, tmpPath, err)
	}

	// Flush to stable storage before closing. Without Sync, a power failure
	// between Close and the kernel writing dirty pages could leave a partial
	// file on disk. The subsequent Verify step would catch this on the next
	// run, but Sync provides the durability guarantee up front — especially
	// important on network filesystems where Close does not imply fsync.
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("copy: sync temp file %q: %w", tmpPath, err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("copy: close temp file %q: %w", tmpPath, err)
	}

	// Preserve source mtime on the temp file (informational — not used for
	// date extraction). Non-fatal if it fails.
	_ = os.Chtimes(tmpPath, srcInfo.ModTime(), srcInfo.ModTime())

	return tmpPath, nil
}

// Verify re-reads tmpPath through handler.HashableReader, hashes the media
// payload with hasher, and compares the result against expectedChecksum.
//
// On mismatch the caller should call CleanupTempFile to remove the temp file.
// Verify itself does not delete anything.
func Verify(tmpPath, expectedChecksum string, handler domain.FileTypeHandler, hasher *hash.Hasher) CopyResult {
	rc, err := handler.HashableReader(tmpPath)
	if err != nil {
		return CopyResult{
			Error: fmt.Errorf("copy: open hashable reader for %q: %w", tmpPath, err),
		}
	}
	defer func() { _ = rc.Close() }()

	actual, err := hasher.Sum(rc)
	if err != nil {
		return CopyResult{
			Error: fmt.Errorf("copy: hash temp file %q: %w", tmpPath, err),
		}
	}

	if actual != expectedChecksum {
		return CopyResult{
			Checksum: actual,
			Error: fmt.Errorf(
				"copy: checksum mismatch for %q: expected %s, got %s",
				tmpPath, expectedChecksum, actual,
			),
		}
	}

	return CopyResult{
		Success:  true,
		Checksum: actual,
	}
}

// Promote atomically renames the verified temp file to its canonical
// destination path. Because TempPath places the temp file in the same
// directory as dest, os.Rename is always an atomic same-filesystem operation.
func Promote(tmpPath, dest string) error {
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("copy: promote %q → %q: %w", tmpPath, dest, err)
	}
	return nil
}

// CleanupTempFile removes an unverified temp file. It is called when Verify
// returns a mismatch or when the pipeline is unwinding after an error.
//
// Failure to remove the temp file is non-fatal — the source in dirA is
// untouched and the file can be reprocessed. Orphaned temp files are
// self-healing on the next pixe resume run.
func CleanupTempFile(tmpPath string) {
	_ = os.Remove(tmpPath)
}
