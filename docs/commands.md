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
<!-- pixe:begin:sort-flags -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Flag</th>
      <th>Default</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>--config</td>
      <td></td>
      <td>config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)</td>
    </tr>
    <tr>
      <td>-w, --workers</td>
      <td>0</td>
      <td>number of concurrent workers (0 = auto: runtime.NumCPU())</td>
    </tr>
    <tr>
      <td>-a, --algorithm</td>
      <td>sha1</td>
      <td>hash algorithm: md5, sha1 (default), sha256, blake3, xxhash</td>
    </tr>
    <tr>
      <td>-q, --quiet</td>
      <td>false</td>
      <td>suppress per-file output; show only the final summary</td>
    </tr>
    <tr>
      <td>-v, --verbose</td>
      <td>false</td>
      <td>show per-stage timing and debug information</td>
    </tr>
    <tr>
      <td>--profile</td>
      <td></td>
      <td>load a named config profile from ~/.pixe/profiles/<name>.yaml</td>
    </tr>
    <tr>
      <td>-s, --source</td>
      <td></td>
      <td>source directory containing media files to sort (default: current directory)</td>
    </tr>
    <tr>
      <td>-d, --dest</td>
      <td></td>
      <td>destination directory for the organized archive (required)</td>
    </tr>
    <tr>
      <td>--copyright</td>
      <td></td>
      <td>copyright template injected into destination files, e.g. "Copyright {{.Year}} My Family"</td>
    </tr>
    <tr>
      <td>--camera-owner</td>
      <td></td>
      <td>camera owner string injected into destination files</td>
    </tr>
    <tr>
      <td>--dry-run</td>
      <td>false</td>
      <td>preview operations without copying any files</td>
    </tr>
    <tr>
      <td>--db-path</td>
      <td></td>
      <td>explicit path to the SQLite archive database (overrides auto-resolution)</td>
    </tr>
    <tr>
      <td>-r, --recursive</td>
      <td>false</td>
      <td>recursively process subdirectories of --source</td>
    </tr>
    <tr>
      <td>--skip-duplicates</td>
      <td>false</td>
      <td>skip copying duplicate files instead of copying to duplicates/ directory</td>
    </tr>
    <tr>
      <td>--ignore</td>
      <td></td>
      <td>glob pattern for files to ignore (repeatable, e.g. --ignore "*.txt" --ignore ".DS_Store")</td>
    </tr>
    <tr>
      <td>--no-carry-sidecars</td>
      <td>false</td>
      <td>disable carrying pre-existing .aae and .xmp sidecar files from source to destination</td>
    </tr>
    <tr>
      <td>--overwrite-sidecar-tags</td>
      <td>false</td>
      <td>when merging tags into a carried .xmp sidecar, overwrite existing values instead of preserving them</td>
    </tr>
    <tr>
      <td>--progress</td>
      <td>false</td>
      <td>show a live progress bar instead of per-file text output (requires a TTY)</td>
    </tr>
    <tr>
      <td>--since</td>
      <td></td>
      <td>only process files with capture date on or after this date (format: YYYY-MM-DD)</td>
    </tr>
    <tr>
      <td>--before</td>
      <td></td>
      <td>only process files with capture date on or before this date (format: YYYY-MM-DD)</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:sort-flags -->
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
<!-- pixe:begin:status-flags -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Flag</th>
      <th>Default</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>--config</td>
      <td></td>
      <td>config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)</td>
    </tr>
    <tr>
      <td>-w, --workers</td>
      <td>0</td>
      <td>number of concurrent workers (0 = auto: runtime.NumCPU())</td>
    </tr>
    <tr>
      <td>-a, --algorithm</td>
      <td>sha1</td>
      <td>hash algorithm: md5, sha1 (default), sha256, blake3, xxhash</td>
    </tr>
    <tr>
      <td>-q, --quiet</td>
      <td>false</td>
      <td>suppress per-file output; show only the final summary</td>
    </tr>
    <tr>
      <td>-v, --verbose</td>
      <td>false</td>
      <td>show per-stage timing and debug information</td>
    </tr>
    <tr>
      <td>--profile</td>
      <td></td>
      <td>load a named config profile from ~/.pixe/profiles/<name>.yaml</td>
    </tr>
    <tr>
      <td>-s, --source</td>
      <td></td>
      <td>source directory to inspect (default: current directory)</td>
    </tr>
    <tr>
      <td>-r, --recursive</td>
      <td>false</td>
      <td>recursively inspect subdirectories of --source</td>
    </tr>
    <tr>
      <td>--ignore</td>
      <td></td>
      <td>glob pattern for files to ignore (repeatable, e.g. --ignore "*.txt")</td>
    </tr>
    <tr>
      <td>--json</td>
      <td>false</td>
      <td>emit JSON output instead of a human-readable listing</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:status-flags -->
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
<!-- pixe:begin:verify-flags -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Flag</th>
      <th>Default</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>--config</td>
      <td></td>
      <td>config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)</td>
    </tr>
    <tr>
      <td>-w, --workers</td>
      <td>0</td>
      <td>number of concurrent workers (0 = auto: runtime.NumCPU())</td>
    </tr>
    <tr>
      <td>-a, --algorithm</td>
      <td>sha1</td>
      <td>hash algorithm: md5, sha1 (default), sha256, blake3, xxhash</td>
    </tr>
    <tr>
      <td>-q, --quiet</td>
      <td>false</td>
      <td>suppress per-file output; show only the final summary</td>
    </tr>
    <tr>
      <td>-v, --verbose</td>
      <td>false</td>
      <td>show per-stage timing and debug information</td>
    </tr>
    <tr>
      <td>--profile</td>
      <td></td>
      <td>load a named config profile from ~/.pixe/profiles/<name>.yaml</td>
    </tr>
    <tr>
      <td>-d, --dir</td>
      <td></td>
      <td>archive directory to verify (required)</td>
    </tr>
    <tr>
      <td>--progress</td>
      <td>false</td>
      <td>show a live progress bar instead of per-file text output (requires a TTY)</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:verify-flags -->
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
<!-- pixe:begin:resume-flags -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Flag</th>
      <th>Default</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>--config</td>
      <td></td>
      <td>config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)</td>
    </tr>
    <tr>
      <td>-w, --workers</td>
      <td>0</td>
      <td>number of concurrent workers (0 = auto: runtime.NumCPU())</td>
    </tr>
    <tr>
      <td>-a, --algorithm</td>
      <td>sha1</td>
      <td>hash algorithm: md5, sha1 (default), sha256, blake3, xxhash</td>
    </tr>
    <tr>
      <td>-q, --quiet</td>
      <td>false</td>
      <td>suppress per-file output; show only the final summary</td>
    </tr>
    <tr>
      <td>-v, --verbose</td>
      <td>false</td>
      <td>show per-stage timing and debug information</td>
    </tr>
    <tr>
      <td>--profile</td>
      <td></td>
      <td>load a named config profile from ~/.pixe/profiles/<name>.yaml</td>
    </tr>
    <tr>
      <td>-d, --dir</td>
      <td></td>
      <td>destination directory containing the archive database (required)</td>
    </tr>
    <tr>
      <td>--db-path</td>
      <td></td>
      <td>explicit path to the SQLite archive database (overrides auto-resolution)</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:resume-flags -->
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
<!-- pixe:begin:query-flags -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Flag</th>
      <th>Default</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>-d, --dir</td>
      <td></td>
      <td>archive directory containing the database (required)</td>
    </tr>
    <tr>
      <td>--db-path</td>
      <td></td>
      <td>explicit path to the SQLite archive database (overrides auto-resolution)</td>
    </tr>
    <tr>
      <td>--json</td>
      <td>false</td>
      <td>emit JSON output instead of a table</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:query-flags -->
    </div>
    <h3>Subcommands</h3>
    <div class="flag-table-wrap">
<!-- pixe:begin:query-subs -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Subcommand</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>runs</td>
      <td>List all sort runs recorded in the archive database</td>
    </tr>
    <tr>
      <td>run</td>
      <td>Show details for a specific sort run</td>
    </tr>
    <tr>
      <td>duplicates</td>
      <td>List all duplicate files in the archive</td>
    </tr>
    <tr>
      <td>errors</td>
      <td>List all files that encountered errors during sorting</td>
    </tr>
    <tr>
      <td>skipped</td>
      <td>List all files that were skipped during sorting</td>
    </tr>
    <tr>
      <td>files</td>
      <td>Search for files in the archive by date or source</td>
    </tr>
    <tr>
      <td>inventory</td>
      <td>List all canonical files in the archive</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:query-subs -->
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
<!-- pixe:begin:clean-flags -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Flag</th>
      <th>Default</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>-d, --dir</td>
      <td></td>
      <td>destination directory (dirB) to clean (required)</td>
    </tr>
    <tr>
      <td>--db-path</td>
      <td></td>
      <td>explicit path to the SQLite archive database</td>
    </tr>
    <tr>
      <td>--dry-run</td>
      <td>false</td>
      <td>preview what would be cleaned without modifying anything</td>
    </tr>
    <tr>
      <td>--temp-only</td>
      <td>false</td>
      <td>only clean orphaned files, skip database compaction</td>
    </tr>
    <tr>
      <td>--vacuum-only</td>
      <td>false</td>
      <td>only compact the database, skip file scanning</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:clean-flags -->
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
<!-- pixe:begin:gui-flags -->
<table class="flag-table">
  <thead>
    <tr>
      <th>Flag</th>
      <th>Default</th>
      <th>Description</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>--config</td>
      <td></td>
      <td>config file (default: $HOME/.pixe.yaml or ./.pixe.yaml)</td>
    </tr>
    <tr>
      <td>-w, --workers</td>
      <td>0</td>
      <td>number of concurrent workers (0 = auto: runtime.NumCPU())</td>
    </tr>
    <tr>
      <td>-a, --algorithm</td>
      <td>sha1</td>
      <td>hash algorithm: md5, sha1 (default), sha256, blake3, xxhash</td>
    </tr>
    <tr>
      <td>-q, --quiet</td>
      <td>false</td>
      <td>suppress per-file output; show only the final summary</td>
    </tr>
    <tr>
      <td>-v, --verbose</td>
      <td>false</td>
      <td>show per-stage timing and debug information</td>
    </tr>
    <tr>
      <td>--profile</td>
      <td></td>
      <td>load a named config profile from ~/.pixe/profiles/<name>.yaml</td>
    </tr>
    <tr>
      <td>-s, --source</td>
      <td></td>
      <td>source directory containing media files (default: current directory)</td>
    </tr>
    <tr>
      <td>-d, --dest</td>
      <td></td>
      <td>destination directory for the organized archive</td>
    </tr>
    <tr>
      <td>--copyright</td>
      <td></td>
      <td>copyright template injected into destination files, e.g. "Copyright {{.Year}} My Family"</td>
    </tr>
    <tr>
      <td>--camera-owner</td>
      <td></td>
      <td>camera owner string injected into destination files</td>
    </tr>
    <tr>
      <td>--dry-run</td>
      <td>false</td>
      <td>preview operations without copying any files</td>
    </tr>
    <tr>
      <td>--db-path</td>
      <td></td>
      <td>explicit path to the SQLite archive database (overrides auto-resolution)</td>
    </tr>
    <tr>
      <td>-r, --recursive</td>
      <td>false</td>
      <td>recursively process subdirectories of --source</td>
    </tr>
    <tr>
      <td>--skip-duplicates</td>
      <td>false</td>
      <td>skip copying duplicate files instead of copying to duplicates/ directory</td>
    </tr>
    <tr>
      <td>--ignore</td>
      <td></td>
      <td>glob pattern for files to ignore (repeatable)</td>
    </tr>
    <tr>
      <td>--no-carry-sidecars</td>
      <td>false</td>
      <td>disable carrying pre-existing .aae and .xmp sidecar files</td>
    </tr>
    <tr>
      <td>--overwrite-sidecar-tags</td>
      <td>false</td>
      <td>overwrite existing sidecar tag values instead of preserving them</td>
    </tr>
  </tbody>
</table>
<!-- pixe:end:gui-flags -->
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
