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

// Package discovery walks a source directory and classifies files using a
// registry of FileTypeHandler implementations.
//
// Detection is a two-phase process:
//  1. Extension-based fast path — the file extension is matched against
//     handlers that claim it via Extensions().
//  2. Magic-byte verification — the file header is read and compared against
//     the candidate handler's MagicBytes(). If the bytes do not match, all
//     other registered handlers are tried in registration order. If none
//     match, the file is recorded as a SkippedFile.
package discovery

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cwlls/pixe-go/internal/domain"
)

const magicReadSize = 16 // bytes — enough for all supported format signatures

// Registry holds the set of registered FileTypeHandler implementations and
// provides file-type detection.
type Registry struct {
	handlers []domain.FileTypeHandler
	// extIndex maps lowercase extension → handler for O(1) fast-path lookup.
	extIndex map[string]domain.FileTypeHandler
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		extIndex: make(map[string]domain.FileTypeHandler),
	}
}

// Register adds h to the registry. If two handlers claim the same extension
// the later registration wins the fast-path slot (but both are tried during
// magic-byte fallback).
func (r *Registry) Register(h domain.FileTypeHandler) {
	r.handlers = append(r.handlers, h)
	for _, ext := range h.Extensions() {
		r.extIndex[strings.ToLower(ext)] = h
	}
}

// Detect returns the FileTypeHandler that can process filePath, or nil if no
// registered handler recognises the file.
//
// Detection order:
//  1. Look up the file extension in the index for a candidate handler.
//  2. Read the first magicReadSize bytes of the file.
//  3. If the candidate's magic bytes match → return it.
//  4. Otherwise try every other registered handler's magic bytes in order.
//  5. If nothing matches → return nil.
func (r *Registry) Detect(filePath string) (domain.FileTypeHandler, error) {
	ext := strings.ToLower(fileExt(filePath))

	header, err := readHeader(filePath)
	if err != nil {
		return nil, fmt.Errorf("discovery: read header %q: %w", filePath, err)
	}

	// Phase 1: try the extension-matched candidate first.
	if candidate, ok := r.extIndex[ext]; ok {
		if matchesMagic(header, candidate.MagicBytes()) {
			return candidate, nil
		}
	}

	// Phase 2: try all handlers in registration order (reclassification).
	for _, h := range r.handlers {
		if matchesMagic(header, h.MagicBytes()) {
			return h, nil
		}
	}

	return nil, nil
}

// fileExt returns the file extension including the leading dot, or "" if none.
func fileExt(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[i:]
		}
	}
	return ""
}

// readHeader reads up to magicReadSize bytes from the start of the file.
func readHeader(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, magicReadSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		// io.ErrUnexpectedEOF just means the file is shorter than magicReadSize — fine.
		return nil, err
	}
	return buf[:n], nil
}

// matchesMagic reports whether header satisfies at least one of the provided
// magic signatures. An empty signatures slice never matches.
func matchesMagic(header []byte, sigs []domain.MagicSignature) bool {
	for _, sig := range sigs {
		if sig.Offset+len(sig.Bytes) > len(header) {
			continue
		}
		match := true
		for i, b := range sig.Bytes {
			if header[sig.Offset+i] != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
