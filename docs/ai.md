---
layout: page
title: Built with AI collaboration
section_label: Transparency
permalink: /ai/
---

<div class="ai-card">
  <p>Pixe was designed and built with Claude as a continuous drafting and implementation partner. A significant portion of the code, tests, and this documentation was written by AI — not assembled from templates or lightly edited, but produced through an iterative back-and-forth where the AI drafted and the human directed, accepted, redirected, or discarded.</p>

  <p>That said, every architectural decision in this project came from a human. The choice to never modify source files, to require copy-then-verify before a file is considered complete, to build with no external binary dependencies, to use deterministic output naming, to track history in SQLite — these are design values, not technical defaults. AI can draft code and surface edge cases and maintain consistency across a large codebase. It cannot decide what matters, or why.</p>

  <p>This is the collaboration model I find honest: the developer decides what to build and why. AI helps build it. There is no pretense that AI is a passive autocomplete tool, and no pretense that it is doing the thinking. Both are true at different moments in the same session. The result is software I could not have shipped as quickly alone — and software whose values and guarantees I own entirely.</p>

  <div class="ai-ref">
    This approach to AI-augmented design is described more fully at <a href="https://daplin.org/ai-collaboration.html" target="_blank" rel="noopener">daplin.org/ai-collaboration.html</a>: "AI augments human capability. It does not replace human judgment."
  </div>
</div>
