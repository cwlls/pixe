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

// Package hash provides a configurable, streaming file hashing engine.
// It wraps Go's stdlib crypto primitives and third-party hash libraries
// behind a uniform interface so the rest of the pipeline never imports
// crypto packages directly.
//
// Supported algorithms and their numeric IDs (embedded in filenames):
//
//	0 — md5      (32 hex chars)
//	1 — sha1     (40 hex chars) — default
//	2 — sha256   (64 hex chars)
//	3 — blake3   (64 hex chars)
//	4 — xxhash   (16 hex chars)
//
// See OVERVIEW.md Section 4.5.1 for the full algorithm registry.
package hash

import (
	"crypto/md5"  //nolint:gosec // MD5 is used for content identification, not security
	"crypto/sha1" //nolint:gosec // SHA-1 is used for filename checksums, not security
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	gohash "hash"
	"io"

	"github.com/cespare/xxhash/v2"
	"github.com/zeebo/blake3"
)

const copyBufSize = 32 * 1024 // 32 KB — bounds per-file memory usage during hashing

// Hasher wraps a configurable hash algorithm and exposes a single Sum method
// that consumes an io.Reader in a streaming fashion.
type Hasher struct {
	newFunc func() gohash.Hash
	name    string
	id      int // numeric algorithm ID for filename embedding (Section 4.5.1)
}

// NewHasher returns a Hasher for the named algorithm.
// Supported values: "md5", "sha1" (default), "sha256", "blake3", "xxhash".
// Returns an error for any unrecognised algorithm name.
func NewHasher(algorithm string) (*Hasher, error) {
	switch algorithm {
	case "md5":
		return &Hasher{newFunc: md5.New, name: "md5", id: 0}, nil //nolint:gosec
	case "sha1":
		return &Hasher{newFunc: sha1.New, name: "sha1", id: 1}, nil //nolint:gosec
	case "sha256":
		return &Hasher{newFunc: sha256.New, name: "sha256", id: 2}, nil
	case "blake3":
		return &Hasher{newFunc: func() gohash.Hash { return blake3.New() }, name: "blake3", id: 3}, nil
	case "xxhash":
		return &Hasher{newFunc: func() gohash.Hash { return xxhash.New() }, name: "xxhash", id: 4}, nil
	default:
		return nil, fmt.Errorf("hash: unsupported algorithm %q (supported: md5, sha1, sha256, blake3, xxhash)", algorithm)
	}
}

// Sum reads the full contents of r and returns the lowercase hex-encoded
// digest. It copies in copyBufSize chunks so that arbitrarily large files
// are processed without loading them entirely into memory.
func (h *Hasher) Sum(r io.Reader) (string, error) {
	hw := h.newFunc()
	buf := make([]byte, copyBufSize)
	if _, err := io.CopyBuffer(hw, r, buf); err != nil {
		return "", fmt.Errorf("hash: reading stream: %w", err)
	}
	return hex.EncodeToString(hw.Sum(nil)), nil
}

// Algorithm returns the canonical name of the hash algorithm (e.g. "sha1").
func (h *Hasher) Algorithm() string { return h.name }

// AlgorithmID returns the numeric algorithm identifier embedded in filenames.
// See OVERVIEW.md Section 4.5.1 for the registry.
func (h *Hasher) AlgorithmID() int { return h.id }

// AlgorithmNameByID returns the algorithm name for a numeric ID, or "" if unknown.
// Used by verify to auto-detect the algorithm from a filename's embedded ID.
func AlgorithmNameByID(id int) string {
	switch id {
	case 0:
		return "md5"
	case 1:
		return "sha1"
	case 2:
		return "sha256"
	case 3:
		return "blake3"
	case 4:
		return "xxhash"
	default:
		return ""
	}
}
