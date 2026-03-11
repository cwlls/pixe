---
layout: page
title: Contributing to Pixe
section_label: Open Source
permalink: /contributing/
---

Pixe is open source under the Apache 2.0 license. Contributions are welcome — bug reports, new format handlers, feature ideas, and documentation improvements.

New file format support is particularly straightforward: each format is an isolated package that implements the `FileTypeHandler` interface. The core pipeline requires no changes. See the [Adding a new format](/adding-formats/) guide for a step-by-step walkthrough.

<div class="contribute-steps">
  <div class="step">
    <span class="step-num">01</span>
    <div class="step-text"><strong>Open an issue first</strong> for anything beyond a small bug fix. Alignment before implementation prevents wasted effort on both sides. Describe what you want to build and why.</div>
  </div>
  <div class="step">
    <span class="step-num">02</span>
    <div class="step-text"><strong>Clone and build:</strong>
      <pre style="margin-top:0.5rem;"><span class="term-prompt">$</span> <span class="term-cmd">git clone https://github.com/cwlls/pixe-go.git</span>
<span class="term-prompt">$</span> <span class="term-cmd">cd pixe-go && make build</span></pre>
    </div>
  </div>
  <div class="step">
    <span class="step-num">03</span>
    <div class="step-text"><strong>Run the test suite</strong> before and after your changes:
      <pre style="margin-top:0.5rem;"><span class="term-prompt">$</span> <span class="term-cmd">make check</span>        <span class="term-comment"># fmt + vet + unit tests (fast gate)</span>
<span class="term-prompt">$</span> <span class="term-cmd">make test-all</span>     <span class="term-comment"># includes integration tests</span>
<span class="term-prompt">$</span> <span class="term-cmd">make lint</span>         <span class="term-comment"># golangci-lint</span></pre>
    </div>
  </div>
  <div class="step">
    <span class="step-num">04</span>
    <div class="step-text"><strong>Follow the conventions:</strong> Apache 2.0 header on every <code>.go</code> file, stdlib-only test assertions (no testify), three import groups (stdlib / external / internal), <code>-race</code> always on. See <code>AGENTS.md</code> in the repo for the full style guide.</div>
  </div>
  <div class="step">
    <span class="step-num">05</span>
    <div class="step-text"><strong>Submit a pull request</strong> on GitHub. CI runs formatting checks, vet, lint, and the full test suite on every PR.</div>
  </div>
</div>

→ [Open an issue on GitHub](https://github.com/cwlls/pixe-go/issues){:target="_blank" rel="noopener"}
