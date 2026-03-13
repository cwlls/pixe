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

package copy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopySidecar copies a sidecar file from src to dest. Unlike Execute, this is
// a simple copy without temp-file atomicity or hash verification. Sidecars are
// small metadata files; the source in dirA is always available for re-copy if
// needed.
//
// Parent directories of dest are created if they do not exist.
// The source file's modification time is preserved on the destination.
func CopySidecar(src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy: sidecar stat %q: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("copy: sidecar mkdir %q: %w", dest, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy: sidecar open %q: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("copy: sidecar create %q: %w", dest, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("copy: sidecar stream %q → %q: %w", src, dest, err)
	}

	// Flush to stable storage before closing. Sidecars are written directly
	// to their final path (no temp-file atomicity), so Sync is especially
	// important here — without it a power failure could leave a truncated
	// sidecar with no subsequent verification step to catch it.
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("copy: sidecar sync %q: %w", dest, err)
	}

	if err := out.Close(); err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("copy: sidecar close %q: %w", dest, err)
	}

	// Preserve source mtime (informational — not used for date extraction).
	_ = os.Chtimes(dest, srcInfo.ModTime(), srcInfo.ModTime())

	return nil
}
