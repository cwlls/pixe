// Package config provides the resolved runtime configuration for Pixe,
// populated from CLI flags, config file, and environment variables via Viper.
package config

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
	// Supported values: "sha1" (default), "sha256".
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
}
