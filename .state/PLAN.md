# Pixe Implementation State

**Status:** `pixe query` command — expose archive database queries to end users (Architecture §7.3).

| # | Task | Priority | Agent | Status | Depends On | Notes |
|:--|:-----|:---------|:------|:-------|:-----------|:------|
| 1 | Add new database methods (`OpenReadOnly`, `AllSkipped`, `GetRunByPrefix`, `ArchiveStats`) | high | @developer | [x] complete | — | Three new query methods + a read-only constructor in `internal/archivedb/` |
| 2 | Create parent command and shared formatting (`cmd/query.go`, `cmd/query_format.go`) | high | @developer | [x] complete | 1 | Cobra parent command with `PersistentPreRunE` for DB setup; shared table/JSON output helpers |
| 3 | Implement query subcommands (`runs`, `run`, `duplicates`, `errors`, `skipped`, `files`, `inventory`) | high | @developer | [x] complete | 2 | Seven subcommand files in `cmd/`, one per query type |
| 4 | Tests and verification | high | @tester | [x] complete | 3 | Unit tests for new DB methods, integration tests for CLI subcommands, `make check && make lint` |
| 5 | Commit `pixe query` feature | low | @committer | [~] in-process | 4 | `feat: add pixe query command for archive database interrogation` |


