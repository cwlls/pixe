---
layout: landing
title: Pixe — Safe, Deterministic Photo Sorting
---

<section id="why">
  <div class="container">
    <div class="section-label">Purpose</div>
    <h2>The problem with photo libraries</h2>
    <p class="intro-text">You've got thousands of photos on your phone, a camera SD card, an old hard drive, maybe a USB stick from a vacation years ago. They pile up in folders with names like <code>IMG_4821.jpg</code>, duplicated across devices, with no consistent organization. Every tool that promises to fix this either modifies your originals, locks you into a subscription, or leaves you wondering whether it actually worked.</p>

    <div class="problem-grid">
      <div class="problem-card">
        <div class="problem-q">What if the sort crashes halfway through?</div>
        <span class="tag-ok">Covered</span>
        <p class="problem-a">Pixe tracks every file in a SQLite database as it goes. Interrupted runs resume automatically — already-processed files are skipped.</p>
      </div>
      <div class="problem-card">
        <div class="problem-q">How do I know the copy actually worked?</div>
        <span class="tag-ok">Covered</span>
        <p class="problem-a">Every file is hashed before and after copying. Only when the hashes match does Pixe consider the copy complete. Mismatches are flagged, never silently ignored.</p>
      </div>
      <div class="problem-card">
        <div class="problem-q">What if the same photo appears twice?</div>
        <span class="tag-ok">Covered</span>
        <p class="problem-a">Duplicate detection is checksum-based, not name-based. The same image under different filenames is still caught. Duplicates go to a separate folder, or can be skipped entirely.</p>
      </div>
      <div class="problem-card">
        <div class="problem-q">Will Pixe touch my original files?</div>
        <span class="tag-ok">Never</span>
        <p class="problem-a">Source files are strictly read-only. Pixe copies to a destination directory — it never modifies, moves, or renames anything in your source folder.</p>
      </div>
      <div class="problem-card">
        <div class="problem-q">Will my archive look the same next year?</div>
        <span class="tag-ok">Yes</span>
        <p class="problem-a">Output is deterministic. The same photo always produces the same filename and directory path. Re-running Pixe on the same source produces the same archive.</p>
      </div>
      <div class="problem-card">
         <div class="problem-q">Do I need exiftool or ffmpeg installed?</div>
         <span class="tag-ok">No</span>
         <p class="problem-a">Pixe is a single binary with no runtime dependencies. All EXIF parsing, RAW decoding, and metadata handling is pure Go — nothing to install separately.</p>
       </div>
       <div class="problem-card">
         <div class="problem-q">Can I see what's happening in real time?</div>
         <span class="tag-ok">Yes</span>
         <p class="problem-a">Use <code>pixe gui</code> for a full interactive TUI with live progress bars, activity logs, and per-worker status. Or add <code>--progress</code> to any sort or verify command for a lightweight progress bar.</p>
       </div>
     </div>
  </div>
</section>

<section id="how">
  <div class="container">
    <div class="section-label">Internals</div>
    <h2>How it works</h2>
    <p class="dim">Every file passes through a linear pipeline. If any stage fails, the file is flagged and the pipeline continues with the next file — nothing is silently skipped.</p>

    {% include pipeline.html %}

    <p class="dim" style="font-size:0.85rem;">
      <strong style="color:var(--text)">Discover</strong> — walk source, classify by type &nbsp;·&nbsp;
      <strong style="color:var(--text)">Extract</strong> — read capture date from metadata &nbsp;·&nbsp;
      <strong style="color:var(--text)">Hash</strong> — checksum the media payload &nbsp;·&nbsp;
      <strong style="color:var(--text)">Copy</strong> — write to temp file in destination &nbsp;·&nbsp;
      <strong style="color:var(--text)">Verify</strong> — re-hash destination, confirm match &nbsp;·&nbsp;
      <strong style="color:var(--text)">Tag</strong> — optionally inject copyright metadata &nbsp;·&nbsp;
      <strong style="color:var(--text)">Complete</strong> — atomically rename temp → canonical path
    </p>

    <p class="dim mt2" style="font-size:0.875rem;">→ <a href="{{ '/how-it-works/' | relative_url }}">Read the full internals breakdown</a></p>
  </div>
</section>

<section id="quickstart">
  <div class="container">
    <div class="section-label">Quick Start</div>
    <h2>Up and running in minutes</h2>

    <div class="pre-label">Install</div>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">go install github.com/cwlls/pixe-go@latest</span></pre>

    <div class="pre-label">Sort your photos</div>
    <pre><span class="term-prompt">$</span> <span class="term-cmd">pixe sort --dest ~/Archive</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe sort --source ~/Photos --dest ~/Archive --recursive</span>
<span class="term-prompt">$</span> <span class="term-cmd">pixe gui --dest ~/Archive</span></pre>

    <div class="pre-label">Example output</div>
    <pre><span class="term-copy">COPY</span> IMG_0001.jpg -&gt; 2021/12-Dec/20211225_062223_abc123ef.jpg
<span class="term-skip">SKIP</span> IMG_0002.jpg -&gt; previously imported
<span class="term-dupe">DUPE</span> IMG_0003.jpg -&gt; matches 2021/12-Dec/20211225_062223_abc123ef.jpg
<span class="term-err">ERR </span> corrupt.jpg  -&gt; extract date: no EXIF data

<span class="term-done">Done. processed=4 duplicates=1 skipped=1 errors=1</span></pre>

    <div class="hero-actions mt2">
      <a class="btn btn-primary" href="{{ '/install/' | relative_url }}">Installation guide</a>
      <a class="btn btn-ghost" href="{{ '/commands/' | relative_url }}">All commands →</a>
    </div>
  </div>
</section>
