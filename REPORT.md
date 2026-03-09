# Comprehensive Project Review: Pixe

This report builds upon the initial assessment of `ARCHITECTURAL_OVERVIEW.md` by integrating a detailed review of all Go source files across four key areas: adherence to design principles, security, maintainability, and speed & efficiency.

## 1. Adherence to Design Principles

The codebase consistently reflects the project's stated North Star Principles and Global Constraints.

*   **Safety above all else:**
    *   **Read-only `dirA` enforcement:** The `pipeline.Run` function and associated `discovery.Walk` strictly handle `dirA` as read-only. The only write operation permitted is for the `.pixe_ledger.json` (as seen in `pipeline.Run` and `manifest.SaveLedger`), which is explicitly part of the safety design. Source files are opened with `os.Open` (read-only) in handlers (`jpeg.go`, `heic.go`, `mp4.go`, `tiffraw.go`).
    *   **Copy-then-verify:** The `copy.Execute` and `copy.Verify` functions (`copy/copy.go`) directly implement this, ensuring data integrity post-transfer. `pipeline.Run` and `runWorker` orchestrate this sequence.
    *   **Database-backed resumability:** The `archivedb` package (`internal/archivedb/`) and `cmd/resume.go` fully implement the SQLite-based persistence model, with `DB.InsertRun`, `DB.CompleteRun`, `DB.UpdateFileStatus`, and `DB.GetIncompleteFiles` (in `archivedb/runs.go` and `archivedb/files.go`) ensuring crash safety and resumability. Each file's progress is tracked and committed individually.

*   **Native Go execution:**
    *   **Pure Go parsers:** The `handler` packages (`jpeg`, `heic`, `mp4`, `tiffraw`, `dng`, `nef`, `cr2`, `cr3`, `pef`, `arw`) utilize pure Go libraries (e.g., `github.com/rwcarlsen/goexif`, `github.com/dsoprea/go-exif/v3`, `github.com/abema/go-mp4`) for metadata extraction and hashable region identification. There are no `os/exec` calls for core media processing.
    *   **CGo-free SQLite:** `modernc.org/sqlite` is explicitly imported in `archivedb/archivedb.go`, confirming the CGo-free approach.

*   **Deterministic output:**
    *   **Consistent naming:** `pathbuilder.Build` (`internal/pathbuilder/pathbuilder.go`) meticulously constructs file and directory names based on extracted date, checksum, and file extension, adhering to the `YYYY/MM-Mon/YYYYMMDD_HHMMSS_<CHECKSUM>.<ext>` convention.
    *   **Locale-aware month names:** `pathbuilder.MonthDir` correctly uses `golang.org/x/text/language` and a `monthAbbreviations` map to provide locale-sensitive month directory names (e.g., "02-Feb", "02-Fév").
    *   **Ansel Adams fallback date:** Implemented in `jpeg.anselsAdams`, `heic.anselsAdams`, `mp4.anselsAdams`, and `tiffraw.anselsAdams`, ensuring consistent dating for files lacking metadata.

*   **Modular by design:**
    *   **`domain.FileTypeHandler` interface:** Central to the modularity, defined in `internal/domain/handler.go`. All specific media format handlers (JPEG, HEIC, MP4, RAWs) implement this interface, enabling the `discovery.Registry` to treat them uniformly.
    *   **`tiffraw.Base` for RAWs:** The `internal/handler/tiffraw/tiffraw.go` package provides a shared base struct that is embedded by TIFF-based RAW handlers (DNG, NEF, CR2, PEF, ARW), demonstrating effective code reuse and minimizing duplication as outlined in the architectural overview.

*   **Operational Safety:**
    *   **Comprehensive logging:** The `pipeline.Run` and `runWorker` functions write clear `COPY`, `SKIP`, `DUPE`, `ERROR` messages to `os.Stdout` (or a provided `io.Writer`), ensuring an auditable record.
    *   **SQLite WAL mode and busy-retry:** Explicitly configured in `archivedb.applyPragmas` (`PRAGMA journal_mode=WAL`, `PRAGMA busy_timeout=5000`), allowing safe concurrent read/write access to the database.
    *   **Atomic deduplication check:** `archivedb.CompleteFileWithDedupCheck` uses a transaction to prevent race conditions during deduplication in multi-process scenarios, ensuring only one file is the "original" and subsequent identical files are marked as duplicates.

*   **Scalability:**
    *   **Worker pool:** `pipeline.RunConcurrent` and `runWorker` implement a goroutine worker pool (`n` workers, configurable via `--workers` flag, defaulting to `runtime.NumCPU()`) for parallel file processing, maximizing throughput on multi-core systems.
    *   **Database-backed deduplication:** `archivedb.CheckDuplicate` leverages a `idx_files_checksum` index, which is partial (`WHERE status = 'complete'`) to efficiently query for duplicates without loading all checksums into memory, supporting large archives.
    *   **Streaming I/O:** `hash.Hasher.Sum` and `copy.Execute` use a `copyBufSize` (32 KB) buffer to process files in chunks, preventing large files from consuming excessive memory.
    *   **Optimized RAW hashing:** `tiffraw.HashableReader` and `cr3.HashableReader` prioritize extracting embedded JPEG previews for hashing over processing full RAW data, significantly improving performance for these large files.

## 2. Security

The project's design prioritizes security, but some minor refinements could further enhance it.

*   **Strengths:**
    *   **Read-only source (dirA):** Reinforced by `os.Open` usage for source files.
    *   **No external binaries:** Confirmed by direct usage of pure Go libraries for media processing.
    *   **Data Integrity:** The `hash` package provides SHA-1 and SHA-256 options. `copy.Verify` performs a critical post-copy hash check.
    *   **CGo-free SQLite:** `modernc.org/sqlite` is a good choice for avoiding CGo-related attack vectors.
    *   **Input validation:** CLI flags are parsed by `spf13/cobra` and `spf13/viper`, providing some level of input sanitization. Directory paths are validated for existence and type in `cmd/sort.go` and `cmd/resume.go`.

*   **Recommendation:**
    *   **Hash Algorithm Default:** The `hash/hasher.go` explicitly marks SHA-1 usage with `//nolint:gosec // SHA-1 is used for filename checksums, not security`. While acknowledged, SHA-1 is cryptographically weaker for collision resistance. As recommended in the initial report, changing the default to SHA-256 (e.g., in `cmd/root.go` and `hash/hasher.go` `NewHasher`) would improve overall cryptographic assurance, even if it results in slightly longer filenames. This would make the system more robust against potential collision attacks, even if currently only used for unique identification.

## 3. Maintainability

Maintainability is a significant strength, supported by clear code structure, effective use of Go features, and comprehensive testing.

*   **Strengths:**
    *   **Modularity and SRP:** Packages have well-defined responsibilities (e.g., `discovery` for file walking/classification, `archivedb` for DB interactions, `pipeline` for orchestration). This clear separation of concerns (Single Responsibility Principle) makes the codebase easier to understand, test, and extend.
    *   **Go Interfaces for Extensibility:** The `domain.FileTypeHandler` interface allows for easy addition of new media types without modifying the core pipeline logic. New handlers only need to implement the contract.
    *   **Composition over Inheritance:** The `tiffraw.Base` embedding in RAW handlers (e.g., `dng/dng.go`) demonstrates idiomatic Go composition, reducing boilerplate code and improving maintainability.
    *   **Consistent Error Handling:** The `fmt.Errorf("package: function: %w", err)` pattern for wrapping errors is consistently applied throughout the codebase (e.g., `archivedb/archivedb.go`, `pipeline/pipeline.go`), facilitating easy error tracing and debugging.
    *   **Comprehensive Testing:** The presence of extensive unit tests (`_test.go` files in nearly every package) and `internal/integration` tests demonstrates a strong commitment to quality and makes refactoring safer. Test helpers like `openTestDB`, `copyFixture`, `buildFakeDNG` (in `archivedb_test.go`, `integration_test.go`) promote consistency in testing.
    *   **Clear `config.AppConfig` usage:** The `cmd` package reads `viper` configuration and populates a single `config.AppConfig` struct (`internal/config/config.go`) that is passed to lower layers. This ensures that no lower-level packages have direct `viper` dependencies, promoting clean dependency management.
    *   **SQL Management:** SQL queries are defined as `const` strings (e.g., in `archivedb/schema.go`, `archivedb/runs.go`, `archivedb/files.go`, `archivedb/queries.go`), improving readability and making them easy to review. The `schemaDDL` is idempotent, simplifying database initialization and upgrades.
    *   **Detailed Doc Comments:** Extensive package-level and function-level doc comments (e.g., `tiffraw/tiffraw.go`, `pipeline/pipeline.go`) explain the *why* and *how* of the code, greatly aiding new developers.

*   **Recommendation:**
    *   **Centralized Error Definition for Skipped Reasons:** Currently, `discovery.SkippedFile` uses a `Reason` string (`walk.go`). For consistency and easier programmatic handling in future, consider defining specific `SkippedReason` constants (similar to `FileStatus`) or an enum. This would make it easier to categorize and respond to different skip reasons without relying on string matching.
    *   **`filepath.Match` limitation for ignores:** As noted in the architectural overview, the current ignore mechanism relies on `filepath.Match` which doesn't support recursive glob patterns like `**`. If users frequently need to ignore files across arbitrary subdirectories (e.g., `**/Thumbs.db`), integrating a library like `doublestar` could provide this functionality and might be a small maintainability cost for a significant feature gain.

## 4. Speed and Efficiency

The code demonstrates a deep understanding of performance considerations in Go, particularly for I/O-bound and CPU-bound tasks.

*   **Strengths:**
    *   **Concurrency (`pipeline/worker.go`):** The `RunConcurrent` function, utilizing goroutines, `workCh`, `resultCh`, and `assignChs` channels, is a well-engineered producer-consumer pattern for parallel file processing. The `syncWriter` ensures thread-safe output.
    *   **Optimized I/O (Streaming):** `hash/hasher.go` uses `io.CopyBuffer` with a `copyBufSize` (32 KB) to efficiently hash large files without excessive memory allocations. Similarly, `copy/copy.go` uses buffered streaming for file transfers.
    *   **Database Query Optimization:** The `archivedb` package features several indexes (e.g., `idx_files_checksum`, `idx_files_source`, `idx_files_capture_date`) to ensure fast lookups for deduplication, run history, and file queries.
    *   **Specialized RAW Handlers for Hashing:**
        *   `tiffraw.HashableReader` attempts to extract the embedded full-resolution JPEG preview for hashing, avoiding the computationally expensive processing of the entire RAW sensor data. It includes a fallback to full-file hashing if the JPEG cannot be extracted.
        *   `cr3.HashableReader` similarly focuses on extracting the embedded JPEG from the ISOBMFF container. `mp4.extractKeyframePayload` specifically extracts keyframe data for hashing, optimizing for video content.
    *   **Atomic Deduplication for Performance:** `archivedb.CompleteFileWithDedupCheck` performs the duplicate check and status update within a single transaction, minimizing locking overhead and improving performance under concurrent writes by avoiding multiple database round-trips for critical logic.
    *   **Efficient Date Extraction:** Handlers prioritize `DateTimeOriginal` then `DateTime` for EXIF dates, falling back to a fixed "Ansel Adams" date, preventing expensive filesystem lookups or complex heuristics for undated files.

*   **Recommendation:**
    *   **Pre-allocation for slices:** In several places, slices are appended to in loops (e.g., `discovery.Walk`, `archivedb.scanFileRows`). While Go's slice growth is efficient, pre-allocating with `make([]T, 0, capacity)` when the approximate or maximum size is known (e.g., `len(discovered)` in `pipeline.Run` when creating `records`) can slightly reduce allocations and improve performance, especially for large numbers of files. This is a micro-optimization but good practice.
    *   **Metadata Write for HEIC/MP4/RAWs:** The `WriteMetadataTags` for HEIC (`heic/heic.go`), MP4 (`mp4/mp4.go`), and many RAW formats (`tiffraw/tiffraw.go`, `cr3/cr3.go`) are currently no-ops. While the architectural overview provides a rationale (risk of corruption, lack of pure Go libraries), if metadata tagging is a desired feature for these formats, investigating suitable pure-Go libraries (or contributing to existing ones) could be a future performance optimization. Currently, this doesn't hinder speed as it's skipped, but if implemented, care would be needed to ensure it remains efficient.

---
This comprehensive report concludes the review of the Pixe project, incorporating insights from both the architectural overview and the detailed code review.
