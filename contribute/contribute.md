# Contributing to Ze

Ze takes code, documentation, bug reports, and real-world interop reports. The repositories are public, the issue tracker is public, and development happens in the open.

<div class="contribute-intro">
  <article class="contribute-route cat-operate">
    <span class="contribute-route-kicker">Code</span>
    <h2>Fix the thing you can reproduce</h2>
    <p>Small, well-proven changes are welcome: parser fixes, command output fixes, protocol edge cases, docs corrections, and test coverage that catches real behavior.</p>
  </article>
  <article class="contribute-route cat-observe">
    <span class="contribute-route-kicker">Evidence</span>
    <h2>Bring a trace, lab, or transcript</h2>
    <p>Interop reports, QEMU runs, failing <code>.ci</code> transcripts, and browser captures are useful because they show exactly what broke and what should be checked next.</p>
  </article>
  <article class="contribute-route cat-secure">
    <span class="contribute-route-kicker">Care</span>
    <h2>Handle security privately</h2>
    <p>If an unauthenticated peer can trigger it, or if it can escalate access, use the <a href="../security/">security policy</a> instead of a public issue.</p>
  </article>
</div>

## The contribution contract

<div class="contribute-contract">
  <div>
    <span class="contribute-label">Hard requirement</span>
    <h3>No signed CLA, no merge.</h3>
    <p>Contributions require signing off commits with <code>git commit -s</code>, which signifies agreement to Ze's <a href="https://github.com/ze-software/ze/blob/main/CLA.md">Contributor License Agreement</a>.</p>
    <p>You keep your copyright. What you grant is a broad, non-exclusive license for the Maintainer to use your contribution, including the right to relicense it, alone or combined with the rest of the project, under different license terms.</p>
  </div>
  <aside>
    <strong>Why this exists</strong>
    <p>It keeps the project legally movable. Ze itself stays AGPLv3 for everyone, and the CLA avoids tracking down every past contributor if a commercial license ever helps the project reach more people.</p>
  </aside>
</div>

## How Ze is funded

<div class="contribute-funding cat-services">
  <div>
    <span class="contribute-label">Stewardship</span>
    <h3>Backed work, public project</h3>
    <p>Ze is currently developed by Thomas Mangin, with his time on the project supported by <a href="https://exa.net.uk">Exa Networks</a>. There is no subscription tier, no paid support contract, and no separate commercial entity behind it today.</p>
    <p><a href="https://exa.net.uk">Exa Networks</a> has been backing this work since 2009, when it started with <a href="https://github.com/Exa-Networks/exabgp">ExaBGP</a>, Ze's predecessor.</p>
  </div>
  <div class="contribute-funding-mark" aria-hidden="true">2009</div>
</div>

## Where to start

<div class="contribute-start">
  <a class="contribute-action cat-routing" href="https://github.com/ze-software/ze/issues">
    <span>Issues</span>
    <strong>Browse or file a bug</strong>
    <small>Use this for reproducible problems, missing docs, and clear feature gaps.</small>
  </a>
  <a class="contribute-action cat-platform" href="https://github.com/ze-software/ze">
    <span>Repository</span>
    <strong>GitHub is the source repository</strong>
    <small>Development moved off Codeberg in July 2026. Clone, issues and pull requests all live on GitHub.</small>
  </a>
  <a class="contribute-action cat-automate" href="https://discord.gg/T8s7CjPDne">
    <span>Discussion</span>
    <strong>Ask before guessing</strong>
    <small>There is no formal good-first-issue program yet. Ask where help is most useful right now.</small>
  </a>
</div>
