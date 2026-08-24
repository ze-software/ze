# Spec: website-asciinema-terminal-demos

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling \| docs |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/website-asciinema-terminal-demos.md` (create on the first deferral) |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The published site carries every terminal demo as a VHS-rendered `.webm` with a
`.png` poster. Video is not diffable: a re-render replaces the whole blob, so
each render costs its full size again in git, forever.

Measured on the `gh-pages` branch: the demo binaries entered the branch on
2026-08-17 and added 18.6 MB over three publishes in five days. 18 distinct
videos already hold 37 stored blobs. The working tree carries 7.6 MB of `.webm`
and 1.8 MB of `.png` against 24 KB of transcripts.

Convert the 17 demos the manifest marks `kind: terminal` to asciicast v2
`.cast` files, played on the site by a self-hosted `asciinema-player`. A
`.cast` is text, so git deltas it instead of storing a whole new blob, and the
reader gets selectable, copyable terminal output.

VHS cannot emit a `.cast`, so the recorder changes with the format, and VHS
leaves the tree entirely. The demo DEFINITIONS do not change: `pty-session.py`
learns to read the existing `.tape` files.

### Decisions taken by the owner before design (2026-08-24)

| ID | Decision | Why it binds the design |
|----|----------|-------------------------|
| D-1 | Generated artifacts stay COMMITTED. Deploy-time generation is refused | It was tried and failed too often, in Ze's pipeline and in GitHub's. The owner must be able to inspect the exact bytes a publish will push |
| D-2 | The site is TOTALLY SELF-HOSTED, fonts included | The player's JS and CSS, and the Poppins and Lato faces, are served from the site. No CDN, no asciinema.org, no `fonts.googleapis.com` |
| D-3 | `web-config` stays a video | The manifest marks it `kind: browser` (Playwright). A browser recording has no terminal byte stream, so no `.cast` can represent it |
| D-4 | Scope is the FORMAT conversion, its recorder, and self-hosting | D-1 REFUSES a publishing-pipeline change rather than postponing one, so no such work is pending and nothing is deferred |
| D-5 | VHS is REMOVED, not left dormant | Owner instruction, 2026-08-24: a dependency that stops being used is deleted (`ai/rules/no-layering.md`) |
| D-6 | Approach B: `pty-session.py` reads the tapes; the tapes are not rewritten | Chosen over translating 17 tapes by hand. The definitions stay byte-identical, which is what holds A-4's drift risk down |
| D-7 | The drift risk of a second engine is ACCEPTED, with the transcript as its gate | Owner accepted it at the research gate |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/terminal-demonstrations.md` - the gallery's source page; its demo markers decide which demos the gallery embeds
  → Constraint: the gallery embeds 17 of the 18 demos, and that set is NOT the 17 `kind: terminal` demos. It includes `web-config` (`kind: browser`) and excludes `irr-filter`, which appears only on `guides/irr-filtering`. Any "17" in this spec must say which 17

**Key insights:** (minimal context to resume after compaction)
- The growth is caused by non-diffable binaries, not by publish frequency
- Text artifacts keep the owner's inspect-before-push workflow (D-1) intact
- VHS owns the PTY, so the recorder is a second driver, not a wrapper (C-1)
- `Hide`/`Show` is interleaved through every tape, so recording cannot simply start late (C-10)
- Transcripts are hand-authored, not recorded, so they are free to become the drift gate (C-3)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `demos/terminal/render.py` - renders each demo, writes the artifact manifest, verifies assets
  → Constraint: C-1. `_render_demo` carries the pipeline's ONLY `kind` branch: `render_source = render_tape(demo) if demo["kind"] == "terminal" else source_path`. Everything below it (`container_command`) is kind-blind, and the container-side switch is the `case "$1" in *.tape) vhs "$@"` in `container-entrypoint.sh`. A `.tape` is INTERPRETED by `vhs`, which owns the pseudo-terminal itself, so no recorder can wrap a tape
  → Constraint: C-2. `verify_assets` iterates a hardcoded `("video", "poster", "transcript")` tuple and checks, per asset, that the file exists and that both `st_size` and `sha256` match the manifest. The same triple is hardcoded in three more places: `terminal_demos.ASSET_EXTENSIONS`, `_render_html`/`_render_markdown`, and a hand-written hero `<video>` block in `render-index.py`. The triple moves in four places or not at all
  → Constraint: C-3. `_render_demo` does NOT record the transcript. It runs `shutil.copyfile(source_path.parent / "transcript.txt", ...)` over a hand-authored, git-tracked file. The transcript is therefore an authored CLAIM about the session, which is what makes it available as this spec's drift gate
  → Constraint: C-4. `stamp_definition_hashes` rewrites `definition_sha256` from the current definition and then self-verifies, so the `--check-definition` that follows it in `ze-site-generate` passes by construction. What actually triggers a re-render is `verify_assets` failing on a missing or changed FILE
  → Constraint: C-5. `source_digest` covers 8 shared paths plus every non-dot file under the demo directory; `definition_digest` covers only `common.tape` plus the demo's `source`, and additionally hashes `kind`. `--check` uses the first, `--check-definition` the second
  → Decision: C-11. The renderer image pin is written in two independent places with no cross-check: `mk/terminal-demo.mk` (`TERMINAL_DEMO_IMAGE`, used only by the `docker build -t` in `ze-terminal-demo-image-build`) and `demos/terminal/manifest.json` (`renderer.image`, the copy that reaches `docker run`). Both move when the base image changes
- [ ] `demos/terminal/pty-session.py` - a 329-line in-repo PTY driver with `@type`, `@key`, `@wait`, `@sleep` and `@escape` directives, written for full-screen programs that read single keystrokes
  → Decision: C-12. This is the only in-tree driver that owns a PTY the way a recorder needs. Used today only by four `validate.sh` scripts, never to produce a published artifact. Its `KEYS` map already carries `enter`, `up`, `down` and `left`
  → Constraint: C-13. Its `@wait` matches the PTY BYTE STREAM. VHS's `Wait` matches the RENDERED SCREEN, and the tapes use the screen form: `container-entrypoint.sh` exports `PS1='$ '` precisely so `Wait+Screen /\$ /` resolves, and says so in its own comment. For a full-screen program that repaints, the bytes arrive before the screen settles, so the two are not interchangeable (this is R-1)
- [ ] `demos/terminal/container-entrypoint.sh` - the container's PID 1 for a demo
  → Constraint: C-16. It sources `demo-lock.sh` FIRST because `HOME` is on the mounted repository and every demo container shares it. The lock is sourced rather than wrapped so the demo runs in this shell and keeps the exported `PS1`. A new driver inherits that environment unchanged and MUST NOT be introduced as a wrapper script
  → Constraint: C-17. Its dispatch is `case "${1:-}" in *.tape) vhs "$@" ;; *) "$@" ;; esac`. The `.tape` arm is the only VHS invocation in the container
- [ ] `demos/terminal/*/demo.tape`, `demos/terminal/common.tape` - the demo definitions
  → Constraint: C-14. The vocabulary is closed and small: 13 directives (`Type` 280, `Enter` 276, `Sleep` 159, `Wait` 133, `Show` 61, `Hide` 61, `Source` 17, `Screenshot` 17, `Output` 17, `Set` 16, `Escape` 10, `Left` 1, `Down` 1) and 10 `Set` keys (`FontFamily`, `FontSize`, `Framerate`, `Height`, `Padding`, `Shell`, `Theme`, `TypingSpeed`, `WaitTimeout`, `Width`). Only `Width`, `Height`, `Shell`, `TypingSpeed` and `WaitTimeout` carry meaning for a `.cast`; the other five are video concerns the player replaces with CSS
  → Constraint: C-10. `Hide`/`Show` is interleaved, not a preamble: every tape has about three pairs, mid-demo and near the end (`cli-dashboard` hides at 4, 17 and 51 of 58 lines). A recorder that only starts late cannot express it, so the writer must be able to suspend and resume mid-session
- [ ] `website/tools/terminal_demos.py` - the ONLY reader of the artifact manifest; `render-doc.py` is its one call site
  → Constraint: C-6. `_render_html` emits a `<video controls playsinline preload="metadata" poster=...>` with one `<source type="video/webm">` inside a CSS frame; `_render_markdown` emits the parallel Markdown, which ships as every page's `index.md` sibling AND is indexed into `data/search-index.json`. Both change
  → Constraint: C-7. `_asset_url` versions every demo asset as `?v=<sha256[:10]>`, unlike `sitelib.asset_url`, which deliberately emits no version for `site.css`/`site.js` and strips legacy ones
  → Constraint: C-18. Its module-level `GH_PAGES` is `Path(__file__).resolve().parent.parent`, which is the SITE BUILD ROOT the tools run from, not the `../gh-pages` repository the name suggests. `_assert_generated_assets_untracked` runs at import and refuses demo media tracked under that root or under `demos/terminal/artifacts`. It therefore never inspects the published repository, which is why D-1 does not collide with it. Do not "fix" the name into a real gh-pages path: that would turn the guard against the committed artifacts D-1 requires
- [ ] `website/tools/render-js.py` - publishes the site's JavaScript
  → Constraint: C-8. It reads EXACTLY ONE file, `assets/js/site.js`, and minifies it to `assets/site.js`. No import expansion, no concatenation. `sitepaths._SOURCE_ONLY_DIRS` prunes the whole of `assets/css` and `assets/js` from the published tree, while `build-site.stage_sources` copies every other git-listed file verbatim. So a vendored player belongs on a copied path, not under `assets/js/`
- [ ] `website/tools/sitelib.py` - page templates
  → Constraint: C-9. `FONT_CSS_URL` and `PAGE_HEAD` emit a `fonts.googleapis.com` stylesheet and preconnects to `fonts.googleapis.com` and `fonts.gstatic.com` on EVERY page. Verified in the published `index.html`. The site is not self-hosted today, so D-2 is a new property to establish
  → Decision: C-15. `page_head` already accepts an `extra_head` parameter, and `render-doc.py` calls it without one. The player's `<link>`/`<script>` reach only the pages that carry a demo through that existing parameter; no new template mechanism is needed
- [ ] `mk/terminal-demo.mk` - the render targets
  → Constraint: C-19. `ze-terminal-demo-tools-install` is a one-line target whose only body is `demos/terminal/install-vhs.sh`. It goes with the script (D-5). Every render target already routes through `python3 demos/terminal/render.py`, so no target changes shape

**Behavior to preserve:** (unless the user explicitly said to change it)
- The manifest's per-demo digest discipline: an artifact that does not match its recorded digest must still fail the check
- Every page that embeds a demo: 18 HTML pages, their 18 `index.md` siblings, and the transcript text in `data/search-index.json`
- `web-config` playback, unchanged (D-3). The site keeps a `<video>` path either way
- What each demo SHOWS. The tapes are not edited by this spec
- The demo frame chrome, and a reserved box of the right shape before playback
- The container environment `container-entrypoint.sh` builds, and the lock it takes first (C-16)
- The scope of `_assert_generated_assets_untracked`: build root and artifacts only, never the published repository (C-18)

**Behavior to change:** (only what the user asked for)
- The artifact a `kind: terminal` demo produces: `.cast` in place of `.webm` plus `.png`
- The engine that produces it: `pty-session.py` in place of `vhs`
- The markup and assets the site uses to play a terminal demo
- Font delivery: local `.woff2` in place of Google Fonts

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `demos/terminal/<id>/demo.tape`, plus the `common.tape` it `Source`s. Unchanged text, new interpreter
- Format at entry: VHS tape syntax, the closed vocabulary in C-14

### Transformation Path
1. `render.py` `_render_demo` selects the arm on `demo["kind"]` (the branch C-1 names, now choosing an engine rather than a tape rewrite)
2. `container_command` runs the pinned image; `container-entrypoint.sh` dispatches a `.tape` to `pty-session.py` instead of `vhs` (C-17)
3. `pty-session.py` parses the tape, drives the PTY, and appends `[time, "o", data]` events to an asciicast v2 file, suspending between `Hide` and `Show`
4. The host digests `artifacts/<id>.cast` and writes `assets.cast` into the artifact manifest
5. `render.py` compares the cast's visible text against the demo's `transcript.txt` and fails the render on mismatch
6. `build-site.stage_terminal_media` copies `assets/demos/` into the build tree; `publish_artifact` writes it to `../gh-pages`
7. `terminal_demos._render_html` emits a player mount for a `kind: terminal` demo and the existing `<video>` for `kind: browser`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host ↔ renderer container | bind mount at `ARTIFACT_ROOT`, `docker run` from `container_command` | No |
| Renderer ↔ site | the artifact manifest's per-demo `assets` map | No |
| Site ↔ browser | the vendored player reading a `.cast` over HTTP | No |

### Integration Points
- `pty-session.py` gains a tape front end and an asciicast writer; its existing `@`-directive path and its four `validate.sh` callers keep working unchanged
- `verify_assets` becomes kind-aware instead of iterating one hardcoded triple
- `page_head`'s existing `extra_head` parameter carries the player assets (C-15)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `.cast` is small enough that committing every re-render is cheap | 18 transcripts total 24 KB; a cast adds escape sequences and timings | The conversion buys far less than the estimate and the growth problem survives | Render ONE demo, measure the file against its roughly 400 KB webm, BEFORE converting the rest | unvalidated |
| A-2 | `asciinema-player` replays every TUI demo faithfully, cursor and redraws included | It is a terminal emulator in JS; the demos are terminal sessions | Some demos degrade and must stay video | Play the rendered cast for `cli-dashboard` in a browser | unvalidated |
| A-3 | The published site sets no CSP that would refuse the player's script | No CSP in any `sitelib` template, any `render-*.py`, or any published `.html`; no `_headers`, no `netlify.toml`. `sitelib.patch_theme_bootstrap` already injects an INLINE `<script id="theme-bootstrap">` into every page, which any `script-src` CSP without a nonce would have broken already | The player is silently refused and the demos show nothing | Confirmed by the inline theme-bootstrap script working in production | CONFIRMED |
| A-4 | Re-driving each demo under `pty-session.py` reproduces the session the tape produces under VHS | Unverified. C-13: the two engines match on different things | Demos drift from what they show today, silently, one demo at a time | The transcript gate in AC-5, per demo | unvalidated |
| A-5 | `Screenshot`, `Framerate`, `FontFamily`, `FontSize`, `Padding` and `Theme` can be ignored without changing what a demo SHOWS | C-14: they are video-rendering concerns, and a cast carries none of them | A demo's meaning depended on a visual setting, and the cast loses it | Read every tape's `Set` block during phase 2 and confirm each ignored key is presentational | unvalidated |
| A-6 | The renderer image can drop the VHS base and keep chromium for `web-config` | Unverified. The base is `ghcr.io/charmbracelet/vhs:v0.11.0` plus an apt layer that already installs chromium and `playwright-core@1.55.0` | The browser demo breaks while the terminal ones are converted | Rebuild the image and re-render `web-config` alone, digest unchanged | unvalidated |
| A-7 | `ttyd` leaves with VHS and nothing else needs it | Unverified. VHS drives a terminal through `ttyd`; `pty-session.py` opens its own PTY with the `pty` module | The image loses a binary something else used | Grep the tree and the Dockerfile for `ttyd` during phase 7 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `Wait` means "screen settled" in VHS and "bytes seen" in `pty-session.py` (C-13). A repainting program races ahead | The cast shows a command's output before the screen would have settled, or the transcript gate fails | Convert `cli-dashboard`, `launcher` and `config-graph` FIRST, not last: they are the redraw-heavy ones. If a byte-stream match proves insufficient, add a settle delay to the tape front end's `Wait`, never to the tapes |
| R-2 | Removing `<video>` removes the `aspect-ratio: 5/3` that reserves the demo box, so pages reflow as the player loads | The gallery jumps on load | Reserve the box on the player mount from the tape's own `Set Width`/`Set Height` |
| R-3 | The old `.webm` and `.png` stay in git history whatever this spec does | None: a property of git | Stated in Known Limitations so the size claim is not overstated |
| R-4 | `_render_markdown` output is indexed into `data/search-index.json`, so a markup change moves search results too | Search results lose demo pages, or link to a `.webm` that no longer exists | Cover the Markdown arm in the same test as the HTML arm |
| R-5 | Two engines exist mid-conversion, so a half-converted tree can publish a demo twice or neither | A demo page carries a player AND a video, or an empty frame | `verify_assets` is kind-aware and demands exactly the asset set the kind names; a demo carrying both fails the check |
| R-6 | The image pin lives in two files with no cross-check (C-11); one is updated and the other is not | The build target names one image and `docker run` pulls another | Change both in the same commit; the Deliverables table carries the grep that proves they agree |
| R-7 | `HOME` is shared by every demo container and the lock is taken before any setup (C-16). A driver introduced as a wrapper script would run outside that shell and lose the exported `PS1` | Demos time out waiting for a prompt that is never painted | Dispatch the driver in the entrypoint's existing `case`, in the same shell, exactly where `vhs` is invoked today |
| R-8 | `_assert_generated_assets_untracked` is renamed or "corrected" toward the real gh-pages path (C-18) | Every site render fails, because the published repository tracks 55 demo files by design | Leave the guard's scope alone; the spec's Critical Review carries the check |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The public demo gallery: demos that do not play, an empty player box, or a page that reflows on load |
| How is it reverted? | Single commit revert in each repo; the previous `.webm` assets remain in `gh-pages` history |
| Who else touches this path? | `plan/spec-ze-website-0-umbrella.md` (Status `design`) plans a live in-browser CLI that would partly supersede recorded terminal demos |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `demos/terminal/render.py --demo cli-dashboard` (a `kind: terminal` demo) | → | the tape front end in `pty-session.py` and its asciicast writer | `test_render_terminal_demo_produces_cast` |
| `demos/terminal/render.py --check` over a converted demo | → | the kind-aware `verify_assets` | `test_verify_assets_demands_cast_for_terminal_kind` |
| a published page carrying a demo | → | `terminal_demos._render_html` | `test_render_demo_page_embeds_player` |
| `demos/terminal/render.py --demo web-config` (the `kind: browser` demo) | → | the unchanged Playwright arm | `test_browser_demo_still_produces_video_and_poster` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `kind: terminal` demo is rendered | An `<id>.cast` exists; no `<id>.webm` and no `<id>.png` are produced for that demo |
| AC-2 | A tape suspends output with `Hide` and resumes with `Show` | No byte emitted between them appears in the cast |
| AC-3 | A tape hides a region lasting N seconds | Cast timestamps stay monotonic across the region and carry no N-second gap: the clock re-bases |
| AC-4 | A tape sets `Set Width` and `Set Height` | The cast header records those as its terminal size, and the page reserves a box of that shape before the player loads |
| AC-5 | The visible text of a rendered cast differs from the demo's `transcript.txt` | The render FAILS, names the demo and the first differing line, and publishes nothing |
| AC-6 | `--check` runs over a `kind: terminal` demo whose manifest entry names a `video` asset | The check fails: a terminal demo's asset set is cast plus transcript |
| AC-7 | `--check` runs over the `kind: browser` demo | It still demands video, poster and transcript, and passes unchanged |
| AC-8 | A published page carries a `kind: terminal` demo | The page emits a player mount bound to that demo's `.cast`, with the player's JS and CSS served from this site, and no `<video>` element for that demo |
| AC-9 | A published page's `index.md` sibling carries a `kind: terminal` demo | It links the cast and the transcript, and offers no WebM recording |
| AC-10 | Any published page is loaded | It requests nothing from `fonts.googleapis.com` or `fonts.gstatic.com`; Poppins and Lato are served from this site |
| AC-11 | The tree is searched for VHS | No `vhs` invocation, no `charmbracelet/vhs` image reference, and no `install-vhs.sh` remains. The `.tape` files stay |
| AC-12 | The published `assets/demos/` tree is listed after a full render | It carries exactly one `.webm` and one `.png`, both belonging to `web-config` |
| AC-13 | An unknown tape directive or `Set` key is met | The render fails naming the directive, rather than skipping it silently |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens a guide page and plays its terminal demo | published page → self-hosted player → `.cast` | `test_render_demo_page_embeds_player` |
| 2 | Copies a command out of a demo | player renders selectable text rather than video pixels | manual, recorded in Goal Validation |
| 3 | Opens the gallery on a slow link | reserved box, no reflow, no external font request | `test_render_demo_page_reserves_player_box` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_tape_vocabulary_is_covered` | `demos/terminal/test_render.py` | AC-13: every directive and `Set` key in C-14 is implemented or explicitly ignored; an unknown one raises | |
| `test_hide_suspends_recording` | `demos/terminal/test_render.py` | AC-2 | |
| `test_hidden_region_rebases_the_clock` | `demos/terminal/test_render.py` | AC-3 | |
| `test_cast_header_carries_tape_dimensions` | `demos/terminal/test_render.py` | AC-4 | |
| `test_transcript_mismatch_fails_the_render` | `demos/terminal/test_render.py` | AC-5 | |
| `test_verify_assets_demands_cast_for_terminal_kind` | `demos/terminal/test_render.py` | AC-6 | |
| `test_verify_assets_unchanged_for_browser_kind` | `demos/terminal/test_render.py` | AC-7 | |
| `test_render_demo_page_embeds_player` | `website/tools/test_render_demos.py` | AC-8 | |
| `test_render_demo_page_reserves_player_box` | `website/tools/test_render_demos.py` | AC-4, R-2 | |
| `test_markdown_sibling_links_the_cast` | `website/tools/test_render_demos.py` | AC-9, R-4 | |
| `test_no_external_font_reference` | `website/tools/test_render_demos.py` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| tape `Set Width` / `Set Height` (columns, rows) | as the tapes use them | the value in the tape | 0 is rejected | N/A: no upper bound is imposed today |
| cast event timestamp | monotonic, non-negative seconds | the last event's time | a negative or decreasing timestamp is rejected | N/A |

### Functional Tests
<!-- No daemon Go changes, so no `.ci` surface exists. The driving surfaces are
     the two Python tools this spec changes, tested the way
     `website/tools/test_render_blog.py` already tests its renderer. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test_render_terminal_demo_produces_cast` | `demos/terminal/test_render.py` | An operator renders a terminal demo and gets a `.cast`, with no `.webm` and no poster `.png` | |
| `test_browser_demo_still_produces_video_and_poster` | `demos/terminal/test_render.py` | The browser demo is untouched by the conversion | |
| `test_published_tree_has_one_video` | `website/tools/test_render_demos.py` | After a full render the published tree carries exactly one `.webm`, `web-config`'s | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A: no wire-visible behavior changes | N-A | N-A | N-A | N-A |

## Files to Modify
- `demos/terminal/pty-session.py` - tape front end plus asciicast v2 writer; the existing `@`-directive path is preserved for the four `validate.sh` callers
- `demos/terminal/render.py` - per-kind asset map in `_render_demo`, kind-aware `verify_assets`, the transcript gate, removal of `render_tape` and `accelerated_terminal_tape`
- `demos/terminal/container-entrypoint.sh` - dispatch a `.tape` to the new driver rather than `vhs`, in the same shell (C-16, R-7)
- `demos/terminal/Dockerfile` - rebase off the VHS image, keep chromium and Playwright
- `demos/terminal/manifest.json` - `renderer.image` and `renderer.version`
- `mk/terminal-demo.mk` - the image tag, and `ze-terminal-demo-tools-install` (C-19)
- `website/tools/terminal_demos.py` - per-kind `ASSET_EXTENSIONS`, `_render_html`, `_render_markdown`, `_publish_assets`
- `website/tools/render-doc.py` - pass the player assets through `page_head`'s existing `extra_head` (C-15)
- `website/tools/render-index.py` - the hand-written hero `<video>` block
- `website/tools/sitelib.py` - drop `FONT_CSS_URL` and both preconnects; serve local faces
- `website/assets/css/30-components.css` - player styling in place of the video frame rules
- `docs/guide/terminal-demonstrations.md` - wording that names WebM

## Files to Create
- `demos/terminal/test_render.py` - the render-side tests above
- `website/tools/test_render_demos.py` - the site-side tests above
- `website/assets/vendor/asciinema-player.min.js` and its `.css` - the vendored player, on a verbatim-copied path (C-8)
- `website/assets/vendor/fonts/` - the Poppins and Lato `.woff2` faces and a local `@font-face` stylesheet (D-2)

## Files to Delete
- `demos/terminal/install-vhs.sh` - D-5, with its `ze-terminal-demo-tools-install` target (C-19)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No daemon config surface: this is website and demo tooling |
| YANG validation constraints | N-A | As above |
| YANG custom validators | N-A | As above |
| CLI commands/flags | N-A | No `ze` command changes; `render.py`'s own flags are unchanged |
| CLI grammar (keyword before value) | N-A | As above |
| Editor autocomplete | N-A | No config leaves |
| Functional test for new RPC/API | N-A | No RPC or API |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | `ZE_DEMO_RELEASE` and `ZE_DEMO_SPEEDUP` already exist and are container-local, not `ze.*` daemon config |
| Doctor check for runtime dependencies | Yes | The renderer's external binaries change: `vhs` and probably `ttyd` leave (A-7). `mk/terminal-demo.mk`'s own `command -v docker` check is the place; the daemon gains no dependency, so `internal/core/diagnostic` is untouched |
| Prometheus counters/metrics | N-A | No runtime state |
| BGP family surface | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The demos already exist; their format changes |
| 2 | Config syntax changed? | No | No config surface |
| 3 | CLI command added/changed? | No | No `ze` command changes |
| 4 | API/RPC added/changed? | No | No RPC surface |
| 5 | Plugin added/changed? | No | No plugin surface |
| 6 | Has a user guide page? | Yes | `docs/guide/terminal-demonstrations.md` names WebM in its prose |
| 7 | Wire format changed? | No | No protocol surface |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol surface |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, if it names the demo renderer; confirmed in phase 1 |
| 11 | Affects daemon comparison? | No | No daemon behavior changes |
| 12 | Internal architecture changed? | Yes | The demo pipeline's own doc under `docs/architecture/`, if one exists; named in phase 1 |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry surface |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | Run `python3 scripts/dev/spec_doc_anchors.py plan/spec-website-asciinema-terminal-demos.md` in phase 1 and record the result here |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any doc showing a demo embed or a `.webm` link |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the new arm reachable and failing
   - Tests: the four Wiring Test rows, all red
   - Files: `demos/terminal/pty-session.py` (tape front end stub), `demos/terminal/render.py` (per-kind asset map), `demos/terminal/container-entrypoint.sh`, `demos/terminal/test_render.py`
   - Verify: `render.py --demo cli-dashboard` reaches the new driver and fails because it writes no cast. Also run `spec_doc_anchors.py` and fill documentation row 16
2. **Phase: The tape front end and the cast writer** -- AC-1 to AC-4, AC-13
   - Tests: `test_tape_vocabulary_is_covered`, `test_hide_suspends_recording`, `test_hidden_region_rebases_the_clock`, `test_cast_header_carries_tape_dimensions`
   - Files: `demos/terminal/pty-session.py`
   - Verify: an unknown directive raises rather than being skipped, and A-5 is answered by reading every tape's `Set` block
3. **Phase: The transcript gate** -- AC-5, mitigating A-4 and R-1
   - Tests: `test_transcript_mismatch_fails_the_render`
   - Files: `demos/terminal/render.py`
   - Verify: convert `cli-dashboard`, `launcher` and `config-graph` FIRST (R-1), then measure A-1 on the first cast before converting the remaining fourteen
4. **Phase: Kind-aware verification** -- AC-6, AC-7, R-5
   - Tests: `test_verify_assets_demands_cast_for_terminal_kind`, `test_verify_assets_unchanged_for_browser_kind`
   - Files: `demos/terminal/render.py`
5. **Phase: The site** -- AC-8, AC-9, R-2, R-4
   - Tests: the four `website/tools/test_render_demos.py` tests
   - Files: `website/tools/terminal_demos.py`, `render-doc.py`, `render-index.py`, `website/assets/css/30-components.css`, the vendored player
6. **Phase: Self-hosted fonts** -- AC-10, its own commit
   - Tests: `test_no_external_font_reference`
   - Files: `website/tools/sitelib.py`, `website/assets/vendor/fonts/`
7. **Phase: Remove VHS** -- AC-11, AC-12, D-5, its own commit
   - Files: `demos/terminal/Dockerfile`, `demos/terminal/manifest.json`, `mk/terminal-demo.mk`, delete `demos/terminal/install-vhs.sh`
   - Verify: rebuild the image, re-render `web-config` alone, and confirm its digest is unchanged (A-6). Answer A-7 with a `ttyd` grep

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | All 17 `kind: terminal` demos converted, not a subset; the gallery and all 18 pages still render |
| Correctness | A converted demo replays the same session the `.webm` showed, judged against `transcript.txt` |
| Naming | The asset key is `cast`, matching the file extension, as `video`/`poster`/`transcript` already do |
| Data flow | The manifest stays the single source of truth for a demo's assets; nothing infers an extension from the demo id |
| Rule: `ai/rules/no-layering.md` | VHS is DELETED, not left behind a flag or an unused branch |
| Rule: `ai/rules/evidence.md` | The size claim in the Task is measured on a rendered `.cast`, never estimated |
| Guard scope (C-18, R-8) | `_assert_generated_assets_untracked` still checks the build root and `demos/terminal/artifacts`, and still never inspects the published repository |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No VHS remains | `grep -rn "vhs\|charmbracelet" demos/ mk/ website/ Makefile` returns only the `.tape` files' own syntax |
| The image pin agrees in both places (R-6) | `grep -rn "ze-terminal-demo-render-all" mk/terminal-demo.mk demos/terminal/manifest.json` shows one tag |
| One video survives | `ls ../gh-pages/assets/demos/*.webm` lists exactly `web-config.webm` |
| No external font request | `grep -rn "fonts.googleapis\|fonts.gstatic" ../gh-pages/ website/tools/` returns nothing |
| A cast is smaller than the webm it replaces (A-1) | `ls -l` on both for one demo, the numbers recorded in Goal Validation |
| Every terminal demo converted | `ls ../gh-pages/assets/demos/*.cast` lists 17 files |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A `.cast` is replayed in the reader's browser. Confirm the player is the only thing interpreting it, and that no page code evaluates its contents |
| Supply chain | The vendored player is pinned by version, committed, and reviewed once, rather than fetched at build time. Record the version and the upstream digest |
| Untrusted input | The cast is produced by our own renderer from our own tapes, so it is not attacker-controlled. Record that reasoning rather than assuming it |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The pipeline had exactly one `kind` branch before this spec (C-1), which is why a terminal-only change is expressible at all. Keeping it to one branch is the design's main constraint.
- The hand-authored transcript looked like redundancy and turned out to be the only independent description of what a demo shows. It becomes the gate that makes an engine swap safe.
- `Hide`/`Show` is what forces the recorder to be ours rather than `asciinema rec`. A format decision was settled by a directive nobody would have looked at.
- `terminal_demos.GH_PAGES` names the build root, not the gh-pages repository (C-18). The guard reads correctly only once that is known, and it reads backwards until then.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Convert to asciicast v2 | Keep `.webm` and re-encode smaller; move media to Release assets; deploy-time generation | Deploy-time generation is refused (D-1). Of the rest, only a text format makes a committed re-render cheap AND keeps the inspect-before-push workflow |
| Teach `pty-session.py` to read the tapes (B) | Translate 17 tapes into `pty-session.py` directives (A) | The definitions stay byte-identical, so the drift A-4 warns about has one chance to appear rather than seventeen. The vocabulary is closed and small (C-14), which is what makes the parser cheap |
| Write the asciicast from the driver | Shell out to `asciinema rec` | `Hide`/`Show` is interleaved (C-10) and `asciinema rec` has no pause primitive, so wrapping it means recording everything and cutting regions afterwards: the same work plus a dependency. Writing it directly adds no recorder dependency at all |
| Gate the cast on the hand-authored transcript | Trust the engine swap; compare against the old video | A video cannot be diffed, which is the premise of this whole spec. The transcript is the only text description of the session that already exists |
| Remove VHS entirely | Leave it installed for other uses | Owner instruction (D-5) and `ai/rules/no-layering.md`. Nothing else in the tree invokes it |

## Known Limitations
- The `.webm` and `.png` already in `gh-pages` history stay there. This spec changes future growth only (R-3)
- `web-config` remains a video, so the site keeps one video path and one `.webm` (D-3)
- A cast records the terminal byte stream, so anything a demo conveyed through video-only settings (`Theme`, `FontFamily`, `Padding`) becomes the player's CSS rather than the recording's content (A-5)

## RFC Documentation (Scope: protocol)

N-A: no protocol surface.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
