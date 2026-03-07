# Changelog: Pixe

*This file contains the high-level progress of the project for the user. Contents appear with the newest changes at the top.*

---

## [v0.10.0] - 2026-03-07
- **Features**:
  - Locale-aware month directory names (MM-Mon format) for better internationalization.
  - Centralized version management to ensure consistent versioning across all components.

- **Improvements**:
  - Fixed issues with template versioning and ldflags path in goreleaser configuration.
  - Improved JPEG entropy data parsing by correctly identifying the EOI marker.
  - Enhanced Go linting workflows by removing deprecated configurations and updating to the latest golangci-lint version.

- **Bug Fixes**:
  - Resolved all golangci-lint violations in the codebase.
  - Fixed formatting issues in developer documentation.
  - Corrected release permissions and version bumping logic.

- **Other**:
  - Updated release configuration (`release.yml`) and added comprehensive linting and testing workflows for GitHub Actions.

## [v0.9.6] - 2026-03-06
- **Features**:
  - Implemented core domain types and interfaces for a robust foundation.
  - Added support for HEIC and MP4 file types through new handlers and processing pipelines.
  - Introduced a worker pool for efficient parallel processing of file operations.
  - Added the `pixe sort` CLI command to enable sorting of files by metadata.

- **Engine Implementations**:
  - Built the Sort Pipeline Orchestrator to manage the sorting workflow.
  - Developed the Copy & Verify Engine to ensure data integrity during operations.
  - Implemented the Path Builder to construct file paths dynamically.
  - Added a hashing engine with persistent manifest storage for file discovery and verification.

- **Other**:
  - Marked all related tasks (11-16) as complete in the project state.
  - Added a Makefile with common development targets to streamline local development.
  - Conducted integration tests and a safety audit to validate system reliability.

## [v0.9.5] - 2026-03-06
- **Refactor**:
  - Renamed the module to `github.com/cwlls/pixe-go` for better clarity and consistency.

- **Documentation**:
  - Added a project README to document the project's purpose and setup.
  - Updated the architectural overview to include version management details.

## [v0.9.4] - 2026-03-06
- **Chore**:
  - Removed a duplicate LICENSE file and added Apache-2.0 license headers to all source files.

## [v0.9.3] - 2026-03-06
- **Initial Commit**:
  - Project scaffold and Go module initialized.

- **Foundation**:
  - Established the core domain structure and interfaces.

## [v0.9.2] - 2026-03-06
- **Initial Commit**:
  - Project scaffold and Go module initialized.

## [v0.9.1] - 2026-03-06
- **Initial Commit**:
  - Project scaffold and Go module initialized.

## [v0.9.0] - 2026-03-06
- **Initial Commit**:
  - Project scaffold and Go module initialized.

> All changes are tracked in the git history. For detailed commit logs, see the full git log.

*Note: Version numbers are derived directly from git tags. Semantic versioning is followed with major, minor, and patch updates reflecting feature additions, improvements, and bug fixes.*
