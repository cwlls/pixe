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

// Package config provides the resolved runtime configuration for Pixe,
// populated from CLI flags, config file, and environment variables via Viper.
package config

import "time"

// AppConfig holds the fully resolved configuration for a Pixe run.
// It is constructed in the CLI layer (cmd/) from Viper's merged values
// and passed down into the pipeline — no package below cmd/ reads Viper directly.
type AppConfig struct {
	// Source is the read-only directory containing media files to sort.
	Source string

	// Destination is the directory where the organized archive will be written.
	Destination string

	// Workers is the number of concurrent pipeline workers.
	// 0 means auto-detect based on runtime.NumCPU().
	Workers int

	// Algorithm is the name of the hash algorithm to use.
	// Supported values: "md5", "sha1" (default), "sha256", "blake3", "xxhash".
	Algorithm string

	// Copyright is the raw template string for the Copyright metadata tag.
	// Supports {{.Year}} which expands to the file's 4-digit capture year.
	// Empty string means no Copyright tag is written.
	Copyright string

	// CameraOwner is the freetext string for the CameraOwner metadata tag.
	// Empty string means no CameraOwner tag is written.
	CameraOwner string

	// DryRun, when true, causes the pipeline to extract and hash files but
	// skip all copy, verify, and tag operations. Output is printed to stdout.
	DryRun bool

	// DBPath is an explicit path to the SQLite archive database.
	// If empty, the database location is auto-resolved (see dblocator package).
	DBPath string

	// Recursive, when true, causes discovery to descend into subdirectories
	// of Source. Default is false (top-level only).
	Recursive bool

	// SkipDuplicates, when true, causes the pipeline to skip copying files
	// whose checksum matches an already-archived file. No file is written to
	// the duplicates/ directory; the ledger entry records the match but omits
	// the destination field. Default is false (duplicates are copied to
	// duplicates/<run_timestamp>/).
	SkipDuplicates bool

	// Ignore is a list of glob patterns for files to exclude from processing.
	// Patterns are matched against the filename (and relative path in recursive
	// mode) using filepath.Match semantics. The ledger file (.pixe_ledger.json)
	// is always ignored regardless of this list — that is handled in the
	// ignore package, not here.
	Ignore []string

	// CarrySidecars controls whether pre-existing sidecar files (.aae, .xmp)
	// in dirA are carried alongside their parent media file to dirB.
	// Default is true (enabled). Set to false via --no-carry-sidecars.
	CarrySidecars bool

	// OverwriteSidecarTags controls the merge behaviour when Pixe injects
	// metadata tags into a carried .xmp sidecar that already contains those
	// fields. When false (default), existing values in the source .xmp are
	// preserved (source is authoritative). When true, Pixe's configured
	// --copyright and --camera-owner values replace existing values.
	OverwriteSidecarTags bool

	// Since, when non-nil, causes the pipeline to skip files with a capture
	// date before this time. Format: YYYY-MM-DD parsed to start-of-day UTC.
	Since *time.Time

	// Before, when non-nil, causes the pipeline to skip files with a capture
	// date after this time. Format: YYYY-MM-DD parsed to end-of-day UTC (23:59:59.999999999).
	Before *time.Time

	// Verbosity controls output detail level.
	// -1 = quiet (summary only), 0 = normal (default), 1 = verbose (timing info).
	Verbosity int

	// PathTemplate is the token-based template for the directory structure
	// leading to the filename. Uses {token} syntax with known tokens:
	// {year}, {month}, {monthname}, {day}, {hour}, {minute}, {second}, {ext}.
	// Default: "{year}/{month}-{monthname}" (matches pre-template behavior).
	// The filename itself (YYYYMMDD_HHMMSS-<ALGO_ID>-<CHECKSUM>.<ext>) is not
	// configurable — only the directory path is templated.
	PathTemplate string

	// Aliases maps short names to filesystem paths for destination resolution.
	// Populated from the "aliases" key in .pixe.yaml. Used by the cmd/ layer
	// to resolve @-prefixed --dest values before populating Destination.
	// Example: {"nas": "/Volumes/NAS/Photos", "backup": "/Volumes/Backup/Archive"}
	Aliases map[string]string
}
