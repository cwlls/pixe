---
title: Contributing
---

# Contributing

Pixe is open source under the Apache 2.0 license. Contributions are welcome — bug reports, new format handlers, feature ideas, and documentation improvements.

New file format support is particularly straightforward: each format is an isolated package that implements the `FileTypeHandler` interface. The core pipeline requires no changes. See the [Adding a new format](adding-formats.md) guide for a step-by-step walkthrough.

1. **Open an issue first** for anything beyond a small bug fix. Alignment before implementation prevents wasted effort on both sides. Describe what you want to build and why.

2. **Clone and build:**

   ```bash
   $ git clone https://github.com/cwlls/pixe.git
   $ cd pixe && make build
   ```

3. **Run the test suite** before and after your changes:

   ```bash
   $ make check        # fmt + vet + unit tests (fast gate)
   $ make test-all     # includes integration tests
   $ make lint         # golangci-lint
   ```

4. **Follow the conventions:** Apache 2.0 header on every `.go` file, stdlib-only test assertions (no testify), three import groups (stdlib / external / internal), `-race` always on. See `AGENTS.md` in the repo for the full style guide.

5. **Submit a pull request** on GitHub. CI runs formatting checks, vet, lint, and the full test suite on every PR.

→ [Open an issue on GitHub](https://github.com/cwlls/pixe/issues){:target="_blank" rel="noopener"}
