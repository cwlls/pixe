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

// Package domain defines shared types used across internal packages,
// including the FileTypeHandler interface, pipeline status types,
// and ledger/manifest structures.
package domain

import (
	"io"
	"time"
)

// MagicSignature defines a byte pattern at a specific file offset used to
// verify a file's true type independent of its extension.
type MagicSignature struct {
	Offset int    // Offset is the byte position in the file where the signature starts.
	Bytes  []byte // Bytes is the expected byte sequence at the given offset.
}

// MetadataTags holds optional tags to be injected into destination files
// after copy and verification. The Copyright field is expected to be
// fully rendered (template variables already substituted) before being
// placed here.
type MetadataTags struct {
	Copyright   string // e.g. "Copyright 2021 My Family, all rights reserved"
	CameraOwner string // e.g. "Wells Family"
}

// IsEmpty reports whether no tags are set, allowing callers to skip the
// tagging stage entirely without opening the file.
func (t MetadataTags) IsEmpty() bool {
	return t.Copyright == "" && t.CameraOwner == ""
}

// MetadataCapability declares how a handler supports metadata tagging.
type MetadataCapability int

const (
	// MetadataNone indicates the format cannot receive metadata at all.
	// The pipeline skips tagging entirely for this handler.
	MetadataNone MetadataCapability = iota

	// MetadataEmbed indicates the format supports safe in-file metadata writing.
	// The pipeline calls WriteMetadataTags to inject tags directly into the file.
	MetadataEmbed

	// MetadataSidecar indicates the format cannot safely embed metadata.
	// The pipeline writes an XMP sidecar file alongside the destination copy.
	MetadataSidecar
)

// FileTypeHandler is the contract every filetype module must implement.
// The core engine is format-agnostic and interacts with all media files
// exclusively through this interface.
//
// Detection strategy:
//  1. The registry performs an initial fast-path match on file extension
//     using Extensions().
//  2. Magic bytes are then read from the file header and compared against
//     MagicBytes() to confirm the type. If they do not match, the file
//     may be reclassified or flagged as unrecognized.
//
// Hashable region:
//
//	All handlers hash the complete file contents. Destination files are
//	byte-identical copies of their source, and metadata is expressed
//	exclusively via XMP sidecars, so the full-file hash remains stable
//	regardless of tagging operations.
type FileTypeHandler interface {
	// Detect returns true if this handler can process the given file.
	// Implementations should verify magic bytes at the file header after
	// the registry has already performed an extension-based pre-filter.
	Detect(filePath string) (bool, error)

	// ExtractDate returns the capture date/time from the file's metadata.
	// Each implementation defines its own format-appropriate fallback chain.
	// The global policy is: DateTimeOriginal → CreateDate → 1902-02-20 00:00:00 UTC
	// (Ansel Adams' birthday), making undated files immediately identifiable.
	ExtractDate(filePath string) (time.Time, error)

	// HashableReader returns an io.ReadCloser over the complete file contents.
	// The core engine pipes this reader through the configured hash algorithm.
	// All current handlers return a reader over the full file — destination
	// files are byte-identical copies of their source. Callers are responsible
	// for closing the returned reader.
	HashableReader(filePath string) (io.ReadCloser, error)

	// MetadataSupport declares this handler's metadata tagging capability.
	// The pipeline uses this to decide between embedded writes, XMP sidecar
	// generation, or skipping tagging entirely.
	MetadataSupport() MetadataCapability

	// WriteMetadataTags injects metadata tags directly into the file.
	// Only called when MetadataSupport() returns MetadataEmbed.
	// Must be a no-op when tags.IsEmpty() is true.
	WriteMetadataTags(filePath string, tags MetadataTags) error

	// Extensions returns the lowercase file extensions this handler claims,
	// used for the initial fast-path detection before magic byte verification.
	// Example: []string{".jpg", ".jpeg"}
	Extensions() []string

	// MagicBytes returns the byte signatures used to confirm file type.
	// Multiple signatures may be returned for formats with variant headers.
	MagicBytes() []MagicSignature
}
