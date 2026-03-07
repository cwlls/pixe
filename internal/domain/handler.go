package domain

import (
	"io"
	"time"
)

// MagicSignature defines a byte pattern at a specific file offset used to
// verify a file's true type independent of its extension.
type MagicSignature struct {
	Offset int
	Bytes  []byte
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
//	Each handler defines what constitutes the "media payload" for its
//	format — the bytes that are hashed and embedded in the output filename.
//	This region excludes metadata so that metadata edits (e.g. tagging)
//	do not invalidate the checksum.
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

	// HashableReader returns an io.Reader scoped to the media payload only,
	// excluding all metadata. The core engine pipes this reader through the
	// configured hash algorithm. Callers are responsible for closing any
	// underlying file handles; implementations should return a reader that
	// holds an open file and document that the caller must close it.
	HashableReader(filePath string) (io.ReadCloser, error)

	// WriteMetadataTags injects optional metadata tags (Copyright,
	// CameraOwner) into the destination file. Called only after copy and
	// verify succeed. Must be a no-op when tags.IsEmpty() is true.
	WriteMetadataTags(filePath string, tags MetadataTags) error

	// Extensions returns the lowercase file extensions this handler claims,
	// used for the initial fast-path detection before magic byte verification.
	// Example: []string{".jpg", ".jpeg"}
	Extensions() []string

	// MagicBytes returns the byte signatures used to confirm file type.
	// Multiple signatures may be returned for formats with variant headers.
	MagicBytes() []MagicSignature
}
