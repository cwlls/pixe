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

//go:build darwin

package dblocator

import "syscall"

// isNetworkMount returns true if the given path resides on a network
// filesystem (NFS, SMB/CIFS, AFP, WebDAV) on macOS.
func isNetworkMount(path string) (bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return false, err
	}
	fstype := fstypeName(stat.Fstypename[:])
	switch fstype {
	case "nfs", "smbfs", "afpfs", "webdav":
		return true, nil
	}
	return false, nil
}

// fstypeName converts a null-terminated [16]int8 filesystem type name to a string.
func fstypeName(b []int8) string {
	buf := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}
	return string(buf)
}
