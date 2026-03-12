---
layout: page
title: Commands
section_label: Reference
permalink: /commands/
---

Click any command to expand its flags and details.

<!-- pixe sort -->
<div class="cmd-block open">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe sort</span>
    <span class="cmd-desc">Organize media into a date-based archive</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <p style="font-size:0.875rem;color:var(--text-dim);margin-bottom:0.75rem;">Primary operation. Discovers files in the source directory, processes them through the pipeline, and writes organized output to the destination. When <code>--source</code> is omitted, the current working directory is used.</p>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe sort --dest /path/to/archive [options]</span></pre>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Flag</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>-s, --source</td><td>Source directory (default: current working directory)</td></tr>
          <tr><td>-d, --dest</td><td>Destination archive directory <strong>(required)</strong></td></tr>
          <tr><td>-w, --workers</td><td>Concurrent workers (default: auto-detect from CPU count)</td></tr>
          <tr><td>-a, --algorithm</td><td>Hash algorithm: <code>sha1</code> or <code>sha256</code> (default: sha1)</td></tr>
          <tr><td>-r, --recursive</td><td>Recurse into subdirectories of source</td></tr>
          <tr><td>--ignore</td><td>Glob pattern to exclude (repeatable: <code>--ignore "*.txt"</code>). Supports <code>**</code> recursive globs and trailing <code>/</code> for directory-only matching.</td></tr>
          <tr><td>--skip-duplicates</td><td>Skip duplicates entirely instead of copying to <code>duplicates/</code></td></tr>
          <tr><td>--copyright</td><td>Copyright template: <code>"Copyright {{.Year}} My Family"</code></td></tr>
          <tr><td>--camera-owner</td><td>Camera owner string to inject into metadata</td></tr>
          <tr><td>--no-carry-sidecars</td><td>Disable carrying pre-existing <code>.aae</code> and <code>.xmp</code> sidecar files from source to destination (carry is enabled by default)</td></tr>
          <tr><td>--overwrite-sidecar-tags</td><td>When merging tags into a carried <code>.xmp</code> sidecar, replace existing values instead of preserving them</td></tr>
          <tr><td>--progress</td><td>Show live progress bar with file count, ETA, and status counters (only activates when stdout is a TTY)</td></tr>
          <tr><td>--dry-run</td><td>Preview operations without copying any files</td></tr>
          <tr><td>--db-path</td><td>Explicit path to the SQLite archive database</td></tr>
        </tbody>
      </table>
    </div>
    <h3>Examples</h3>
    <pre><span class="term-comment"># Sort from current directory</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe sort --dest ~/Archive</span>

<span class="term-comment"># Recursive sort with explicit source</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe sort --source ~/Photos --dest ~/Archive --recursive</span>

<span class="term-comment"># Dry run to preview without copying</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe sort --dest ~/Archive --dry-run</span>

<span class="term-comment"># With copyright tagging and duplicate skipping</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe sort --dest ~/Archive --copyright "Copyright {{.Year}} My Family" --skip-duplicates</span>

<span class="term-comment"># Ignore OS junk files</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe sort --dest ~/Archive --ignore ".DS_Store" --ignore "Thumbs.db"</span>

<span class="term-comment"># Sort without carrying sidecar files</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe sort --dest ~/Archive --no-carry-sidecars</span></pre>
  </div>
</div>

<!-- pixe status -->
<div class="cmd-block">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe status</span>
    <span class="cmd-desc">Report sort status of a source directory</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <p style="font-size:0.875rem;color:var(--text-dim);margin-bottom:0.75rem;">Read-only. Compares files on disk against the <code>.pixe_ledger.json</code> written by prior sort runs. No archive database or destination directory required — works entirely from the source directory. When <code>--source</code> is omitted, the current working directory is inspected.</p>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe status [options]</span></pre>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Flag</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>-s, --source</td><td>Source directory to inspect (default: current working directory)</td></tr>
          <tr><td>-r, --recursive</td><td>Recurse into subdirectories</td></tr>
          <tr><td>--ignore</td><td>Glob pattern to exclude (repeatable)</td></tr>
          <tr><td>--json</td><td>Emit JSON output instead of human-readable listing</td></tr>
        </tbody>
      </table>
    </div>
    <p style="font-size:0.8rem;color:var(--text-faint);margin-top:0.75rem;">Categories: <code>SORTED</code> · <code>DUPLICATE</code> · <code>ERRORED</code> · <code>UNSORTED</code> · <code>UNRECOGNIZED</code></p>
  </div>
</div>

<!-- pixe verify -->
<div class="cmd-block">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe verify</span>
    <span class="cmd-desc">Re-hash every file in the archive to confirm integrity</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <p style="font-size:0.875rem;color:var(--text-dim);margin-bottom:0.75rem;">Walks a previously sorted archive, parses checksums from filenames, recomputes data-only hashes, and reports mismatches. Use this to confirm your archive is intact after a disk migration or NAS transfer.</p>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe verify --dir /path/to/archive [options]</span></pre>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Flag</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>-d, --dir</td><td>Archive directory to verify <strong>(required)</strong></td></tr>
          <tr><td>-w, --workers</td><td>Concurrent workers (default: auto)</td></tr>
          <tr><td>-a, --algorithm</td><td>Must match the algorithm used during sort (default: sha1)</td></tr>
          <tr><td>--progress</td><td>Show live progress bar with file count, ETA, and status counters (only activates when stdout is a TTY)</td></tr>
        </tbody>
      </table>
    </div>
    <p style="font-size:0.8rem;color:var(--text-faint);margin-top:0.75rem;">Exit code <code>0</code> = all verified. Exit code <code>1</code> = one or more mismatches.</p>
  </div>
</div>

<!-- pixe resume -->
<div class="cmd-block">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe resume</span>
    <span class="cmd-desc">Resume an interrupted sort operation</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe resume --dir /path/to/archive</span></pre>
    <p style="font-size:0.875rem;color:var(--text-dim);margin-top:0.5rem;">Finds the most recent interrupted run in the archive database and re-sorts from the original source directory. Files already marked complete are skipped automatically.</p>
    <div class="flag-table-wrap mt1">
      <table class="flag-table">
        <thead><tr><th>Flag</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>-d, --dir</td><td>Destination containing the archive database <strong>(required)</strong></td></tr>
          <tr><td>--db-path</td><td>Explicit path to the SQLite archive database</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</div>

<!-- pixe query -->
<div class="cmd-block">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe query</span>
    <span class="cmd-desc">Read-only queries against the archive database</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <p style="font-size:0.875rem;color:var(--text-dim);margin-bottom:0.75rem;">Read-only interrogation of the archive SQLite database. No files are modified. All subcommands accept <code>--json</code> for machine-readable output.</p>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe query &lt;subcommand&gt; --dir /path/to/archive [--json]</span></pre>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Flag</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>--dir</td><td>Destination directory associated with the archive database <strong>(required)</strong></td></tr>
          <tr><td>--db-path</td><td>Explicit path to the SQLite archive database</td></tr>
          <tr><td>--json</td><td>Emit JSON output instead of human-readable table</td></tr>
        </tbody>
      </table>
    </div>
    <h3>Subcommands</h3>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Subcommand</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>runs</td><td>List all sort runs with file counts, ordered by start time</td></tr>
          <tr><td>run &lt;id&gt;</td><td>Show metadata and file list for one run. Supports short prefix matching — <code>pixe query run a1b2</code> works if the prefix is unambiguous.</td></tr>
          <tr><td>duplicates</td><td>List all duplicates. <code>--pairs</code> shows each duplicate alongside its original.</td></tr>
          <tr><td>errors</td><td>List all files in error states (<code>failed</code>, <code>mismatch</code>, <code>tag_failed</code>) across all runs</td></tr>
          <tr><td>skipped</td><td>List all skipped files with skip reasons</td></tr>
          <tr><td>files</td><td>Filter by <code>--from</code>/<code>--to</code> (capture date), <code>--imported-from</code>/<code>--imported-to</code> (import date), or <code>--source</code> (source directory)</td></tr>
          <tr><td>inventory</td><td>List all canonical archive files (complete, non-duplicate)</td></tr>
        </tbody>
      </table>
    </div>
    <h3>Examples</h3>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe query runs --dir ~/Archive</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe query run a1b2c3d4 --dir ~/Archive</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe query duplicates --dir ~/Archive --pairs</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe query files --dir ~/Archive --from 2024-01-01 --to 2024-12-31</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe query inventory --dir ~/Archive --json | jq '.results | length'</span></pre>
  </div>
</div>

<!-- pixe clean -->
<div class="cmd-block">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe clean</span>
    <span class="cmd-desc">Remove orphaned temp files and compact the archive database</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <p style="font-size:0.875rem;color:var(--text-dim);margin-bottom:0.75rem;">Maintenance command for a destination archive. Removes <code>.pixe-tmp</code> files left by interrupted runs, removes orphaned XMP sidecars, and optionally runs <code>VACUUM</code> on the SQLite database to reclaim space.</p>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe clean --dir /path/to/archive [options]</span></pre>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Flag</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>-d, --dir</td><td>Destination directory to clean <strong>(required)</strong></td></tr>
          <tr><td>--db-path</td><td>Explicit path to the SQLite archive database</td></tr>
          <tr><td>--dry-run</td><td>Preview what would be cleaned without deleting files or running VACUUM</td></tr>
          <tr><td>--temp-only</td><td>Only clean orphaned temp files and XMP sidecars. Skip database compaction.</td></tr>
          <tr><td>--vacuum-only</td><td>Only compact the database. Skip file scanning. Mutually exclusive with <code>--temp-only</code>.</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</div>

<!-- pixe gui -->
<div class="cmd-block">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe gui</span>
    <span class="cmd-desc">Launch the interactive terminal UI</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <p style="font-size:0.875rem;color:var(--text-dim);margin-bottom:0.75rem;">Interactive terminal UI with three tabs: Sort (configure and run with live progress bar, activity log, per-worker status), Verify (configure and run with live progress bar and activity log), and Status (background walk + ledger classification). Requires a TTY.</p>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe gui --dest /path/to/archive [options]</span></pre>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Flag</th><th>Description</th></tr></thead>
        <tbody>
          <tr><td>-s, --source</td><td>Source directory (default: current working directory)</td></tr>
          <tr><td>-d, --dest</td><td>Destination archive directory</td></tr>
          <tr><td>-w, --workers</td><td>Concurrent workers (default: auto-detect from CPU count)</td></tr>
          <tr><td>-a, --algorithm</td><td>Hash algorithm: <code>sha1</code> or <code>sha256</code> (default: sha1)</td></tr>
          <tr><td>-r, --recursive</td><td>Recurse into subdirectories of source</td></tr>
          <tr><td>--ignore</td><td>Glob pattern to exclude (repeatable)</td></tr>
          <tr><td>--skip-duplicates</td><td>Skip duplicates entirely instead of copying to <code>duplicates/</code></td></tr>
          <tr><td>--copyright</td><td>Copyright template: <code>"Copyright {{.Year}} My Family"</code></td></tr>
          <tr><td>--camera-owner</td><td>Camera owner string to inject into metadata</td></tr>
          <tr><td>--no-carry-sidecars</td><td>Disable carrying pre-existing <code>.aae</code> and <code>.xmp</code> sidecar files from source to destination (carry is enabled by default)</td></tr>
          <tr><td>--overwrite-sidecar-tags</td><td>When merging tags into a carried <code>.xmp</code> sidecar, replace existing values instead of preserving them</td></tr>
          <tr><td>--dry-run</td><td>Preview operations without copying any files</td></tr>
          <tr><td>--db-path</td><td>Explicit path to the SQLite archive database</td></tr>
        </tbody>
      </table>
    </div>
    <h3>Key bindings</h3>
    <div class="flag-table-wrap">
      <table class="flag-table">
        <thead><tr><th>Key</th><th>Action</th></tr></thead>
        <tbody>
          <tr><td>Tab / Shift+Tab</td><td>Cycle between tabs</td></tr>
          <tr><td>1 / 2 / 3</td><td>Jump to Sort / Verify / Status tab</td></tr>
          <tr><td>s</td><td>Start sort (Sort tab, configure state)</td></tr>
          <tr><td>v</td><td>Start verify (Verify tab, configure state)</td></tr>
          <tr><td>f</td><td>Cycle activity log filter (All → COPY → DUPE → ERR → SKIP → All)</td></tr>
          <tr><td>n</td><td>New run (complete state)</td></tr>
          <tr><td>e</td><td>Filter to errors (complete state)</td></tr>
          <tr><td>j / k / ↑ / ↓</td><td>Scroll activity log</td></tr>
          <tr><td>r</td><td>Refresh (Status tab)</td></tr>
          <tr><td>q / Ctrl+C</td><td>Quit</td></tr>
        </tbody>
      </table>
    </div>
    <h3>Examples</h3>
    <pre><span class="term-comment"># Launch GUI with destination archive</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe gui --dest ~/Archive</span>

<span class="term-comment"># GUI with explicit source and recursive</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe gui --source ~/Photos --dest ~/Archive --recursive</span>

<span class="term-comment"># GUI with copyright tagging</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe gui --dest ~/Archive --copyright "Copyright {{.Year}} My Family"</span></pre>
  </div>
</div>

<!-- pixe version -->
<div class="cmd-block">
  <div class="cmd-header" onclick="toggle(this)">
    <span class="cmd-name">pixe version</span>
    <span class="cmd-desc">Print version, commit, and build date</span>
    <span class="cmd-toggle">▼</span>
  </div>
  <div class="cmd-body">
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe version</span>
pixe v2.0.0 (commit: abc1234, built: 2026-03-11T10:30:00Z)</pre>
    <p style="font-size:0.875rem;color:var(--text-dim);margin-top:0.5rem;">No flags. Prints the version string and exits.</p>
  </div>
</div>

---

### Configuration file

Pixe reads `.pixe.yaml` from the current directory, home directory, or `$XDG_CONFIG_HOME/pixe`. CLI flags take precedence over config file values. Environment variables prefixed `PIXE_` also override config values (e.g., `PIXE_ALGORITHM=sha256`).

<div class="pre-label">.pixe.yaml</div>
```yaml
algorithm: sha1
workers: 8
recursive: false
skip_duplicates: false
copyright: "Copyright {{.Year}} My Family, all rights reserved"
camera_owner: "Wells Family"
ignore:
  - "*.txt"
  - ".DS_Store"
  - "Thumbs.db"
  - "*.aae"
  - "node_modules/"
```
