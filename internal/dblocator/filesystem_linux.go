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

//go:build linux

package dblocator

import "syscall"

// Linux filesystem magic numbers for network mounts.
const (
	nfsMagic  = 0x6969
	smbMagic  = 0x517B
	smb2Magic = 0xFE534D42
	cifsMagic = 0xFF534D42
)

// isNetworkMount returns true if the given path resides on a network
// filesystem (NFS, SMB/CIFS) on Linux.
func isNetworkMount(path string) (bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return false, err
	}
	switch stat.Type {
	case nfsMagic, smbMagic, smb2Magic, cifsMagic:
		return true, nil
	}
	return false, nil
}
