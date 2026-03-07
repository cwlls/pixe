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
// It wraps Go's stdlib crypto primitives behind a uniform interface so
// the rest of the pipeline never imports crypto packages directly.
package hash

import (
	"crypto/sha1" //nolint:gosec // SHA-1 is used for filename checksums, not security
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	gohash "hash"
	"io"
)

const copyBufSize = 32 * 1024 // 32 KB — bounds per-file memory usage during hashing

// Hasher wraps a configurable hash algorithm and exposes a single Sum method
// that consumes an io.Reader in a streaming fashion.
type Hasher struct {
	newFunc func() gohash.Hash
	name    string
}

// NewHasher returns a Hasher for the named algorithm.
// Supported values: "sha1" (default), "sha256".
// Returns an error for any unrecognised algorithm name.
func NewHasher(algorithm string) (*Hasher, error) {
	switch algorithm {
	case "sha1":
		return &Hasher{newFunc: sha1.New, name: "sha1"}, nil //nolint:gosec
	case "sha256":
		return &Hasher{newFunc: sha256.New, name: "sha256"}, nil
	default:
		return nil, fmt.Errorf("hash: unsupported algorithm %q (supported: sha1, sha256)", algorithm)
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
