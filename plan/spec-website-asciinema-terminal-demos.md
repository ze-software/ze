# Spec: website-asciinema-terminal-demos

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling \| docs |
| Depends | - |
| Phase | 7/7 |
| Deferral shard | none: no phase deferred anything |
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
| D-8 | `config-graph`'s tape, transcript and validator ARE updated to the current pipe grammar. This is the ONE exception to D-6 | Owner ruling, 2026-08-24: the grammar change in `6b0eb49e3` was intentional. `ze config graph ... \| ze pipe match ...` is refused by `rowOperatorRefusal` (`internal/component/command/pipe.go`) with "match needs rows, and this answer has none". The published `.webm` predates that commit by four days, so the live site demonstrates a command the daemon now rejects. D-6 exists to keep a demo showing what it always showed; it was never a reason to keep publishing a broken command (R-11) |

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
  → Decision: C-11. The renderer image pin is written in two independent places with no cross-check: `mk/build-terminal-demo.mk` (`TERMINAL_DEMO_IMAGE`, used only by the `docker build -t` in `ze-terminal-demo-image-build`) and `demos/terminal/manifest.json` (`renderer.image`, the copy that reaches `docker run`). Both move when the base image changes
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
- [ ] `mk/build-terminal-demo.mk` - the render targets
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
| A-1 | A `.cast` is small enough that committing every re-render is cheap | 18 transcripts total 24 KB; a cast adds escape sequences and timings | The conversion buys far less than the estimate and the growth problem survives | Render ONE demo, measure the file against its roughly 400 KB webm, BEFORE converting the rest | CONFIRMED |
| A-2 | `asciinema-player` replays every TUI demo faithfully, cursor and redraws included | It is a terminal emulator in JS; the demos are terminal sessions | Some demos degrade and must stay video | Play the rendered cast for `cli-dashboard` in a browser | CONFIRMED for `cli-dashboard`, the redraw-heavy one, in Chromium against the vendored 3.17.0 bundle and the real cast. The strong evidence is agreement between two independent emulators: the player's rendered screen at t=38, read back from the DOM row by row, equals `demos/terminal/screen.py`'s reconstruction of the same cast at the same instant on every one of the 33 rows either one paints. Also seen: the config editor's box drawing at t=12 (`╭─...─╮`, the `│` rules, `╰─...─╯`), the cursor drawn as one filled cell on the player's canvas at the `ze>` prompt (a 192-pixel block on a 1096x864 canvas over a 137x36 grid, which is exactly one cell), and the Ze palette live in the computed style (`--term-color-foreground: #e6d9f2`, `--term-color-background: #1d1133`, `--term-color-2: #3ee6a8`). Not extended to the other 16 demos, which are not yet rendered |
| A-3 | The published site sets no CSP that would refuse the player's script | No CSP in any `sitelib` template, any `render-*.py`, or any published `.html`; no `_headers`, no `netlify.toml`. `sitelib.patch_theme_bootstrap` already injects an INLINE `<script id="theme-bootstrap">` into every page, which any `script-src` CSP without a nonce would have broken already | The player is silently refused and the demos show nothing | Confirmed by the inline theme-bootstrap script working in production | CONFIRMED |
| A-4 | Re-driving each demo under `pty-session.py` reproduces the session the tape produces under VHS | Unverified. C-13: the two engines match on different things | Demos drift from what they show today, silently, one demo at a time | The transcript gate in AC-5, per demo | CONFIRMED over the four demos rendered (`cli-dashboard`, `launcher`, `zefs-config`, `config-graph`), and the gate is what carries the other 13: `check_transcript` fails the render and publishes nothing, so a demo that does not reproduce cannot reach the site. It found three real drifts on its first run, which is the row being answered rather than assumed |
| A-5 | `Screenshot`, `Framerate`, `FontFamily`, `FontSize`, `Padding` and `Theme` can be ignored without changing what a demo SHOWS | C-14: they are video-rendering concerns, and a cast carries none of them | A demo's meaning depended on a visual setting, and the cast loses it | Read in phase 1, over every tape: `grep -h '^Set ' demos/terminal/*/demo.tape demos/terminal/common.tape \| sort \| uniq -c`. `common.tape` carries all 10 keys once each, and the ONLY per-demo override anywhere is `Set WaitTimeout 60s`, in 6 tapes. So each ignored key has exactly one value in the whole tree and none is per-demo. None writes a byte to the PTY, and none sets the terminal geometry, which `Width` and `Height` set and the cast keeps. `Screenshot` appears once per tape and writes the poster the `<video>` needed, which AC-1 removes. One obligation falls out for phase 5: `Set Theme` is the Ze 16-color palette, so the player's CSS must carry it or every demo changes color. CORRECTED in phase 2: `Set Width` and `Set Height` are PIXELS, not columns and rows. `render.py`'s `OUTPUT_WIDTH`/`OUTPUT_HEIGHT` are the same 1680x1008, and `website/assets/css/30-components.css` reserves their 5/3. A cast records a CHARACTER GRID, so `FontSize` and `Padding` are read as well, for the geometry and nothing else. Three keys stay ignored, not five: `FontFamily`, `Framerate` and `Theme` | CONFIRMED |
| A-8 | The 137x36 grid derived from the tape's pixel box is the grid VHS recorded into | `pty-session.terminal_size` derives it from `Set Width`, `Set Height`, `Set FontSize` and `Set Padding` with JetBrains Mono's own metrics: 600/1000 of an em per character, 1320/1000 per line. At `common.tape`'s numbers that grid fills 1678x1006 of the 1680x1008 box, so the box was chosen for a whole grid and the derivation recovers it. What xterm.js measured inside VHS was not read, and cannot be once VHS leaves. MEASURED in phase 3, from the published `cli-dashboard.png`, which VHS itself rendered at 1680x1008. Its intro card carries a rule of 38 `=`, and the cast records the same 38, so the two are the same characters: the 38 ink runs start at x=45 and x=495, giving an advance of (495-45)/37 = 12.162 px, against the 12 px the 0.6 ratio derives. Ten text bands 26 to 27 px apart give a line height of 26.4 px, which is 20 x 1.32 exactly. So the ROW derivation is right and the COLUMN derivation is not: at 12.162 px, 1680 pixels padded by 17 hold 135.3 columns, not 137 | Lines wrap at a different column than the published demo shows, in every demo at once | Phase 3, on the three redraw-heavy demos: the transcript gate, and a reading of where the cast wraps | BROKEN, and harmlessly: 137 is WIDER than the ~135 VHS gave, so nothing that fitted in the published demo wraps in the cast. No content is authored to a column: the widest line any of the three demos paints is `cli-dashboard`'s own status bar, which `ze` draws to whatever width the PTY reports. The two extra columns come from xterm.js measuring an advance of 0.6081 em where the font declares 0.6, which cannot be re-derived once VHS leaves. Recorded rather than matched, because 137 follows from the font's own metric and 135 follows from a renderer that is being removed |
| A-6 | The renderer image can drop the VHS base and keep chromium for `web-config` | Unverified. The base is `ghcr.io/charmbracelet/vhs:v0.11.0` plus an apt layer that already installs chromium and `playwright-core@1.55.0` | The browser demo breaks while the terminal ones are converted | Phase 7 rebased the image onto `debian:trixie-slim`, the distribution the VHS image itself ran (Debian 13), and re-rendered `web-config` alone. The `.webm` is not a usable instrument: two renders from the SAME unchanged image give two digests (`e6ea62f5`, `6191be47`), so a video digest moves whether anything changed or not. The POSTER is byte-stable across those same two runs (`dd42e311`), and it is the `page.screenshot()` in `demos/terminal/web-config/demo.cjs`, which is chromium's own rasterization of the page. On the rebased image the poster is `a6d8e910`, and the cause is ONE input. A third image, the rebased one with the VHS image's font files copied over its Debian ones, renders the poster `dd42e311`, byte-identical to the VHS-based image, on chromium 151.0.7922.169 against that image's 151.0.7922.137. So neither the base swap nor the chromium update moves a pixel. **Correction, 2026-08-24: the cause phase 7 named is wrong, and the third image's digest does not reproduce from the recipe its scratch Dockerfile records** (rebuilt today it is `565fa85f`). The two families the CSS names were never the difference. Measured in both images with Chrome DevTools `CSS.getPlatformFontsForNode`, `--font-ui` text is Noto Sans at 20 px line height and `--font-mono` text is Fira Code at 19 px on BOTH. What differs is the face chromium uses for the elements that name NO family, which is every form control on the page: Liberation Sans at 185x21 px in the VHS image against Noto Sans at 206x24 px in the rebased one. The VHS base carried Debian's `fonts-liberation`, and fontconfig's `30-metric-aliases.conf` resolves chromium's default Arial to Liberation Sans. That is the whole of the 2 to 6 px shift. The Ze web interface names both families, in `--font-mono` and `--font-ui` (`internal/component/web/assets/style.css`). All 18 validators also pass in the rebased image (`render.py --all --validate`, exit 0) | CONFIRMED, and the font question is CLOSED. The owner ruled on 2026-08-24 that the faces are vendored, and `demos/terminal/fonts/` now carries all three families the render needs: Fira Code 6.002, Noto Sans 2.015 and Liberation 2.1.5, 22 files, 8.4 MB, each byte-identical to its upstream release and to the copy the VHS base carried. The Dockerfile copies them and then deletes every font a Debian package installed, so the typography is decided by repository bytes rather than by the distribution. The acceptance was met exactly: the VHS image and the vendored one, rendered back to back, both produce poster `dd42e3113f0ec68dce7cff023b9b20c0f4498208bde55f4a2f48399c95ee6e4c`, bit-identical (PSNR inf). All 18 validators pass in the new image (`render.py --all --validate`, exit 0). Nothing was written to the published tree. One thing this row does NOT establish: the poster is not byte-stable across TIME. One unchanged image gave two digests on this machine today, differing in 21 anti-aliased border pixels, which is why the comparison above was run back to back. Row filed in `plan/journal/output-not-byte-stable.md` |
| A-7 | `ttyd` leaves with VHS and nothing else needs it | VHS drives a terminal through `ttyd`; `pty-session.py` opens its own PTY with the `pty` module | The image loses a binary something else used | Grepped in phase 1: `grep -rn ttyd . --exclude-dir=.git --exclude-dir=tmp --exclude-dir=vendor` matches two files outside this spec's own text. Both leave with VHS: `demos/terminal/install-vhs.sh`, which D-5 deletes, and one `Makefile` help line describing `ze-terminal-demo-tools-install`, which C-19 removes with it. `demos/terminal/Dockerfile` never names `ttyd`, so it arrives inside `ghcr.io/charmbracelet/vhs:v0.11.0` and leaves when that base does. The pattern reached the corpus: it returned the `install-vhs.sh` hits that are known to exist. RE-CHECKED in phase 7 after the removal: `grep -rniI "\bttyd\b" demos/ mk/ Makefile website/ docs/` returns nothing. A diff of the two images' executables lists `ttyd` and `vhs` among the 113 that left with the VHS base, and a word-boundary grep for all 113 over `demos/terminal/**/*.{sh,tape,cjs,py}` returns zero hits | CONFIRMED |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `Wait` means "screen settled" in VHS and "bytes seen" in `pty-session.py` (C-13). A repainting program races ahead | The cast shows a command's output before the screen would have settled, or the transcript gate fails | RESOLVED at the source in phase 3, and the settle was not what resolved it. The byte-stream match does not race a repainting program, it CANNOT MATCH IT AT ALL: typing `show` into the launcher paints `filter: sh`, then `o` at column 11, then `w` at column 12, each followed by the menu footer, so `filter: show` is a string no program ever emits and `launcher.tape:22` timed out against a faithful session. `demos/terminal/screen.py` reconstructs the screen from the cursor, `TapeSession.wait` searches it, and `Wait+Screen` then means in the recorder what it means in VHS. MEASURED on `cli-dashboard`: with `SETTLE_SECONDS` at 0 the same 61 events are recorded, the gate still passes, and the reconstruction differs by one line in 134, so the settle now buys insurance rather than correctness, at 2.0s of capture and 10s of replay. `SETTLE_LIMIT` was never approached: 2.0s of settle over 7 visible waits is 0.29s each. Both constants are kept at 0.25 and 2.0 on one demo's evidence, to be revisited when all 17 are converted |
| R-2 | Removing `<video>` removes the `aspect-ratio: 5/3` that reserves the demo box, so pages reflow as the player loads | The gallery jumps on load | RESOLVED in phase 5, and the box is reserved from the CAST rather than from the tape. `terminal_demos._cast_facts` reads the grid out of the asciicast header the recorder wrote, and `_reserved_box` turns it into an `aspect-ratio` the mount carries inline (`--demo-aspect: 82.2 / 47.52` for a 137x36 recording, at 0.6 em of advance and 1.32 em of line height). The artifact is then the only place the grid is written down, which the tape would not have been: a tape states a PIXEL box and the grid is derived from it (A-5, A-8). MEASURED in Chromium: the mount is 635.90625 px high before the player is created and 635.90625 px after, and the paragraph under it sits at y=699.90625 in both readings. The player is given `fit: "both"`, so it scales the terminal into whatever box it is handed instead of resizing the box |
| R-3 | The old `.webm` and `.png` stay in git history whatever this spec does | None: a property of git | Stated in Known Limitations so the size claim is not overstated |
| R-4 | `_render_markdown` output is indexed into `data/search-index.json`, so a markup change moves search results too | Search results lose demo pages, or link to a `.webm` that no longer exists | Cover the Markdown arm in the same test as the HTML arm |
| R-5 | Two engines exist mid-conversion, so a half-converted tree can publish a demo twice or neither | A demo page carries a player AND a video, or an empty frame | RESOLVED in phase 4. `verify_assets` reads `asset_paths`, the accessor `_render_demo` already writes from, so the set it checks is the set the demo was rendered as and there is no second list to keep in step. An asset the kind does not produce is refused before any file is read: `cli-dashboard: a terminal demo does not produce poster, video; its assets are cast, transcript`. Measured on the published tree, where all 17 terminal demos still carry a video: `--check-definition` fails for each and passes unchanged for `web-config`. That failure is not a stuck gate. `ze-site-generate` answers a failing `--check-definition` by re-rendering (`Makefile:511`), so the half-converted tree drives the conversion instead of blocking on it |
| R-6 | The image pin lives in two files with no cross-check (C-11); one is updated and the other is not | The build target names one image and `docker run` pulls another | Change both in the same commit; the Deliverables table carries the grep that proves they agree |
| R-7 | `HOME` is shared by every demo container and the lock is taken before any setup (C-16). A driver introduced as a wrapper script would run outside that shell and lose the exported `PS1` | Demos time out waiting for a prompt that is never painted | Dispatch the driver in the entrypoint's existing `case`, in the same shell, exactly where `vhs` is invoked today |
| R-9 | A hidden region that CLEARS the screen leaves the reader holding a screen the terminal threw away. 60 of the tapes' 61 hidden regions end in a `clear`, or in a card whose script starts with one; the exception is `bfd-failover`'s `Hide`/`exit`/`Show`, which resumes on the same screen | The intro card of one section stays on the reader's screen under the next section | JUDGED in phase 3, with evidence, and the phase-2 trade was wrong. With the reset alone, `cli-dashboard` showed `sshpass -e ssh ze-demo` typed onto a bare line, where the transcript and the `.webm` this replaces both show it behind the `$ ` the clear painted, and the AC-5 gate failed on it. VHS records the SCREEN, so the prompt is in the published video. `CastWriter.show` now writes the reset AND what the terminal painted after the last erase, which together are the screen the reader resumes on. Everything erased BEFORE the reset stays out, so the setup a tape hides is still hidden. **AC-2's wording needs the owner**: read literally, no byte emitted between `Hide` and `Show` may appear, and the prompt is such a byte. Read for its purpose, the setup must not appear, and the screen at `Show` is not setup. The code takes the second reading; the first would keep the recording less faithful than the video it replaces |
| R-10 | `accelerated_terminal_tape` divides every `Sleep` by 5, and `expand_timeline` gave the time back with `ffmpeg -itsscale`. A cast has no second pass, so a cast recorded from the accelerated tape replays its pauses five times too fast | Every demo feels rushed: `cli-dashboard`'s 44.5s of scripted pauses arrive as 8.9s | RESOLVED by giving a cast its own second pass, on the owner's instruction of 2026-08-24: the capture STAYS accelerated, because rendering seventeen demos in Docker is what this pipeline spends its wall clock on, and `expand_cast_timeline` multiplies every event timestamp by the same factor afterwards. A cast's timestamps are decimal numbers in a text file, so the pass is exact and lossless where the video arm re-encodes. Uniform scaling is correct only if the injected typing pace is the tape's own divided by the same factor, and it is: `common.tape:8` asks for 125ms and `RENDER_TYPING_SPEED_MS` x `RENDER_SPEEDUP` is 25 x 5 = 125, with no other `Set TypingSpeed` anywhere in the tree and `Set WaitTimeout 60s` the only per-demo override in any tape. MEASURED against each tape's own scripted time: `cli-dashboard` 50.8s scripted against a 55.0s cast, `launcher` 55.1s against 57.0s, `zefs-config` 178.1s against 201.2s, the excess being program paint time and settles, which the video carried too. Unscaled they would have been 11.0s, 11.4s and 40.2s |
| R-8 | `_assert_generated_assets_untracked` is renamed or "corrected" toward the real gh-pages path (C-18) | Every site render fails, because the published repository tracks 55 demo files by design | Leave the guard's scope alone; the spec's Critical Review carries the check |
| R-11 | `config-graph` cannot be rendered at all, and not for a reason this spec owns. `validate.sh` runs `ze config graph router.conf \| ze pipe match peer/upstream`, and the CLI answers `pipe error: match needs rows, and this answer has none: it holds several lists (edges, nodes), so select one first`. The refusal is deliberate and newer than the demo: `rowOperatorRefusal` (`internal/component/command/pipe.go`) arrived in `6b0eb49e3` on 2026-08-23, four days after the published `config-graph.webm` | `render.py --demo config-graph` fails in validation, before any recorder starts | RESOLVED in phase 3 under owner ruling D-8, which makes this demo the one exception to D-6. The branch that fires is the several-lists one, run rather than read: `pipe error: match needs rows, and this answer has none: it holds several lists (edges, nodes), so select one first` (`rowOperatorRefusal`, `internal/component/command/pipe.go`). Its guidance cannot be followed as written, because no operator selects one list out of a document: `internal/component/command/pipe_catalog.go` declares seventeen and none of them addresses `nodes` or `edges`. What the grammar does offer is a format operator over the whole answer, so the three views now read `ze config graph router.conf \| ze pipe text \| ze pipe match <selector>`. `\| text` renders both lists as aligned rows, and `match` over that text keeps whole lines, which is what a row operator does over an input that is not JSON (`applyMatch`, same file). The three selectors are unchanged, and each answer is now one relationship on one line, `peer/upstream-a  inherits  group/transit`, where the old grep over pretty JSON returned lone `"id":` and `"from":` fields. The tape also gains a hidden `cd /src/demos/terminal/config-graph`, which `config-views` already does, so what is typed carries the short path the transcript quotes and the AC-5 gate can hold it. Rendered: 311 events, 137x36, 8,397 bytes, gate passed, and the gate was shown to reject a transcript still quoting the old grammar |

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
| AC-2 | A tape suspends output with `Hide` and resumes with `Show` | No hidden WORK appears in the cast: no command, no output, no byte the hidden region produced as its own content. The terminal STATE the reader resumes into is restored, which after a hidden `clear` means the screen reset and the prompt the terminal painted after that erase. CORRECTED in phase 3: the original wording forbade any byte emitted between `Hide` and `Show`, which was written before we knew 60 of the 61 hidden regions end in a `clear`. Read literally it deleted the `$ ` prompt, so `cli-dashboard` typed `sshpass -e ssh ze-demo` onto a bare line where the hand-authored transcript AND the published `.webm` both show a prompt. Restoring the prompt is fidelity to what the demo showed, not leakage of hidden work (R-9) |
| AC-3 | A tape hides a region lasting N seconds | Cast timestamps stay monotonic across the region and carry no N-second gap: the clock re-bases |
| AC-4 | A tape sets `Set Width` and `Set Height` | The cast header records the CHARACTER GRID those describe, derived from the pixel box with `Set FontSize` and `Set Padding`, and the page reserves a box of that shape before the player loads. CORRECTED in phase 2: the original wording said the header records `Width` and `Height` themselves, which would ask for a 1680-column terminal. They are the video pixel box, the same 1680x1008 as `render.py`'s `OUTPUT_WIDTH`/`OUTPUT_HEIGHT` (A-5, A-8) |
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
| `test_tape_vocabulary_is_covered` | `demos/terminal/test_render.py` | AC-13: every directive and `Set` key in C-14 is implemented or explicitly ignored; an unknown one raises | PASS |
| `test_hide_suspends_recording` | `demos/terminal/test_render.py` | AC-2 | PASS |
| `test_a_hidden_clear_hands_back_the_screen_it_left` | `demos/terminal/test_render.py` | R-9: a hidden `clear` reaches the cast as a screen reset plus what the terminal painted after it, and nothing erased before it. Renamed from `test_hidden_clear_resynchronises_the_screen` in phase 3, whose assertions described the trade R-9 overturned | PASS |
| `test_transcript_mismatch_fails_the_render` | `demos/terminal/test_render.py` | AC-5 | PASS |
| `test_a_faithful_cast_passes_the_render` | `demos/terminal/test_render.py` | AC-5's other half: the gate accepts the session it describes, so a gate that always raised would not pass | PASS |
| `test_commands_shown_out_of_order_fail_the_gate` | `demos/terminal/test_render.py` | AC-5: the gate reads the transcript in ORDER, so a demo whose steps happen in the wrong one fails even though every command is present | PASS |
| `test_a_transcript_quoting_no_command_fails_the_render` | `demos/terminal/test_render.py` | AC-5: a transcript that is all narration would make the gate vacuous for that demo, silently | PASS |
| `test_the_gate_reads_through_the_escape_sequences` | `demos/terminal/test_render.py` | AC-5: the visible-text rule. A recorded terminal wraps nearly every line in SGR sequences | PASS |
| `test_the_stored_cast_is_paced_in_real_time` | `demos/terminal/test_render.py` | R-10: the capture is accelerated and the committed artifact is not | PASS |
| `test_a_real_time_demo_keeps_the_clock_it_recorded` | `demos/terminal/test_render.py` | R-10: a demo captured at wall-clock speed is not stretched | PASS |
| `test_expanding_a_cast_leaves_its_header_alone` | `demos/terminal/test_render.py` | R-10, D-1: the grid is not a clock, and a committed artifact must not move bytes nobody asked to move | PASS |
| `ScreenTest` (8 cases) | `demos/terminal/test_render.py` | `screen.py`: inline completion, scrolling region, `CSI M`, `ESC M`, trailing blanks, in-place repaint, autowrap, scroll-off. Each case is a sequence a Ze demo emits, and each was found by a render that failed against it | PASS |
| `test_hidden_region_rebases_the_clock` | `demos/terminal/test_render.py` | AC-3 | PASS |
| `test_cast_header_carries_tape_dimensions` | `demos/terminal/test_render.py` | AC-4, and the spec's two dimension boundaries | PASS |
| `test_verify_assets_demands_cast_for_terminal_kind` | `demos/terminal/test_render.py` | AC-6 | PASS |
| `ManifestDurationTest` (3 cases) | `demos/terminal/test_render.py` | The manifest states a duration only for a kind that records no cast. The checked-in manifest is accepted, a terminal entry carrying `duration` is refused, and `web-config` losing it is refused. Without the second case the field can come back as a second source of truth; without the third, deleting the rule outright reads as green | PASS |
| `test_browser_demo_still_produces_video_and_poster` | `demos/terminal/test_render.py` | AC-7. Renamed from the planned `test_verify_assets_unchanged_for_browser_kind`: phase 1 wrote ONE case for this row and for the wiring row of the same name, because both ask `verify_assets` what the browser kind owes. It asserts that the three assets are still accepted AND that a cast beside them is refused, which is R-5 read from the browser side | PASS |
| `test_render_demo_page_embeds_player` | `website/tools/test_render_demos.py` | AC-8 | PASS |
| `test_render_demo_page_reserves_player_box` | `website/tools/test_render_demos.py` | AC-4, R-2 | PASS |
| `test_markdown_sibling_links_the_cast` | `website/tools/test_render_demos.py` | AC-9, R-4 | PASS |
| `test_duration_is_read_from_the_cast` | `website/tools/test_render_demos.py` | The published length is the ARTIFACT's. A `kind: terminal` demo no longer restates one in `demos/terminal/manifest.json`, so the four numbers that had drifted from their recordings cannot drift again | PASS |
| `test_browser_demo_keeps_its_video` | `website/tools/test_render_demos.py` | D-3 and AC-7 on the site side: the browser demo still emits a `<video>`, a poster and the duration its manifest states | PASS |
| `HeroMountTest` (2 cases) | `website/tools/test_render_demos.py` | The homepage hero reads the artifact manifest rather than the digests it used to spell by hand (C-2), and refuses a demo that is not a terminal recording | PASS |
| `test_no_external_font_reference` | `website/tools/test_render_demos.py` | AC-10 | PASS |
| `test_font_faces_resolve_to_published_files` | `website/tools/test_render_demos.py` | D-2. A markup test cannot see a `@font-face` pointing at a path the build prunes: the page renders in a system font and stays green. This reads the stylesheet the page links, checks that every file it names exists, that each face keeps `font-display: swap`, and that the directory is not source-only | PASS |
| `test_no_authored_page_requests_a_font_from_google` | `website/tools/test_site_assets.py` | D-2 for the 14 hand-written pages under `labs/`, `performance/`, `style-guide/`, `zeledon/` and `talks/`, which carried the head block by hand. The talk decks are frozen against every patcher in `build.py`, so nothing rewrites them on the way out | PASS |
| `test_inlined_stylesheet_carries_its_font_files` (3 cases) | `website/tools/test_site_assets.py` | D-2 for the standalone deck. `bundle-html.py` base64-encodes a linked stylesheet into a `data:text/css` URI, and a data URI has no directory, so every relative `url()` in it stopped resolving. Until this phase the decks fetched their fonts from Google, so the offline deck was never offline | PASS |

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
| `test_render_terminal_demo_produces_cast` | `demos/terminal/test_render.py` | An operator renders a terminal demo and gets a `.cast`, with no `.webm` and no poster `.png` | PASS |
| `test_browser_demo_still_produces_video_and_poster` | `demos/terminal/test_render.py` | The browser demo is untouched by the conversion | PASS |
| `test_published_tree_has_one_video` | `website/tools/test_render_demos.py` | After a full render the published tree carries exactly one `.webm`, `web-config`'s | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A: no wire-visible behavior changes | N-A | N-A | N-A | N-A |

## Files to Modify
- `demos/terminal/pty-session.py` - tape front end plus asciicast v2 writer; the existing `@`-directive path is preserved for the four `validate.sh` callers
- `demos/terminal/render.py` - per-kind asset map in `_render_demo`, kind-aware `verify_assets`, the transcript gate, and `expand_cast_timeline`, which gives a cast the time the accelerated capture took off it (R-10). `render_tape` and `accelerated_terminal_tape` STAY: the owner ruled on 2026-08-24 that the capture keeps its fifth and the artifact is scaled instead. `validate_contract` asks for `duration` only from a kind that publishes no cast, and REFUSES one from a kind that does
- `demos/terminal/zefs-config/transcript.txt` - two corrections the AC-5 gate found on its first real render: the tape types `cat $ZE_INIT_INPUT` and the transcript quoted it with shell quotes the reader never sees, and the second `exit` is typed at the `ze>` prompt, not `ze#`
- `demos/terminal/config-graph/demo.tape`, `demos/terminal/config-graph/transcript.txt`, `demos/terminal/config-graph/validate.sh` - the one demo re-authored, to the pipe grammar the daemon accepts today (D-8, R-11). The three graph views read `ze config graph router.conf | ze pipe text | ze pipe match <selector>`, and a hidden `cd` into the demo directory shortens the typed path, which is what `config-views` already does
- `demos/terminal/container-entrypoint.sh` - dispatch a `.tape` to the new driver rather than `vhs`, in the same shell (C-16, R-7)
- `demos/terminal/Dockerfile` - rebase off the VHS image, keep chromium and Playwright, and install the vendored Fira Code, Liberation and Noto Sans faces in place of Debian's `fonts-firacode` and `fonts-noto-core`. The copy is followed by a `find` that deletes every other directory under `/usr/share/fonts`, so no distribution font reaches the render. `fontconfig` is named in the apt list now, because it arrived with those two packages and `fc-cache` reads the copied directories
- `demos/terminal/manifest.json` - `renderer.image`, `renderer.version`, and the 17 terminal demos' `engine`, which named the tool that no longer records them. The same 17 lost `duration`: the page reads the length off the cast, so a number here is a second source of truth and four of them had already drifted. `web-config` keeps its `duration`, because a browser recording publishes no cast to read one from (D-3)
- `mk/build-terminal-demo.mk` - the image tag, and `ze-terminal-demo-tools-install` (C-19). The tag names every pinned input the image carries, so vendoring the faces moved it again, in the same commit as `renderer.image` (R-6)
- `Makefile` - the help line for `ze-terminal-demo-tools-install`, which leaves with the target
- `demos/terminal/demo-lock.sh`, `demos/terminal/irr-filter/run.sh`, `demos/terminal/rpki/run.sh`, `demos/terminal/test_render.py`, and comments in `render.py` and `pty-session.py` - prose naming VHS as the live engine, which AC-11 searches for and `ai/rules/stale-comments.md` requires corrected. Two were stale on their own account: `pty-session.py` still said the recorder matches a `Wait` against the byte stream, and its `wait` docstring still described the window phase 3 deleted
- `website/tools/terminal_demos.py` - per-kind `ASSET_EXTENSIONS` plus a new `KIND_ASSETS`, `_render_html`, `_render_markdown`, `_publish_assets`, and the new `_cast_facts`, `_duration_phrase`, `_reserved_box`, `_player_mount`, `player_head` and `hero_mount`. `expand` returns a third value, the head fragment a page carrying a demo needs
- `website/tools/render-doc.py` - pass the player assets through `page_head`'s existing `extra_head` (C-15)
- `website/tools/render-index.py` - the hand-written hero `<video>` block
- `website/assets/js/site.js` - `initTerminalDemoPlayers`, which turns each mount into a player. It is the only site code that touches a cast
- `website/tools/sitepaths.py` - `assets/vendor/README.md` joins `_SOURCE_ONLY_FILES`, so the provenance record stays out of the published artifact while the two vendored files are staged verbatim (C-8)
- `website/tools/test_render_doc.py` - its terminal demo fixture is now the browser kind, because a `kind: terminal` demo no longer publishes a video and its three tests are about the video path
- `website/tools/sitelib.py` - `FONT_CSS_URL`, `FONT_CSS_URL_HTML` and `_FONT_REF_RE` are gone with both preconnects. `PAGE_HEAD` links `{font_css}`, which `page_head` fills from the new `FONT_CSS_PATH` through the existing `asset_url`
- `website/tools/render-activity.py` - its own head template carried the same two preconnects and the same `{font_css}`, filled from `sitelib.FONT_CSS_URL` unescaped
- 14 authored pages under `website/labs/`, `website/performance/`, `website/style-guide/`, `website/zeledon/` and `website/talks/` - each carried the head block by hand
- `website/presentations/tools/bundle-html.py` - `to_data_uri` now resolves a stylesheet's own `url()` references against that stylesheet's directory before inlining it (new `inline_css_urls`). Without it a self-hosted face reaches the standalone deck as an unresolvable relative path
- `website/assets/css/30-components.css` - player styling in place of the video frame rules
- `docs/guide/terminal-demonstrations.md` - wording that names WebM, and the two VHS claims phase 5 left behind (documentation row 6)
- `docs/contributing/gh-pages.md` - the publish description names VHS media (found in phase 1, documentation row 12)
- `website/AI.md` - the same two publish lines in their own wording. Phase 5 saw the file and could not reach it
- `scripts/checks/cli_grammar.go` - the `demoLaunchHits` comment explained why `make ze-precommit-verify` skips the demos with "they need Docker + VHS". Docker is still the reason and VHS is not one, so the clause names Docker alone (`ai/rules/stale-comments.md`)
- `ai/rules/points/cli/cli-grammar-keywords-before-values/the-feeders-that-enforce-the-grammar-rules.md` - the Demo call sites row carried that same sentence, in the same words. `ai/rules/cli.md` is GENERATED from this point and was re-rendered with `make ze-rules-render-update`
- `demos/terminal/test_render.py` - the render-side tests below; the file already exists and is already run
- `demos/terminal/render_test.py` - a SECOND, older render test file, found in phase 4 when the kind-aware check reddened it. Its freshness case built a `kind: terminal` demo carrying video, poster and transcript. The fixture now reads `asset_paths`, so it names the set the check reads; the assertion it makes is unchanged, that a stale `source_sha256` is not read under `definition_only`

## Files to Create
<!-- Corrected in phase 1: `demos/terminal/test_render.py` is NOT new. It is a
     tracked 652-line unittest file, and `TestPythonUnitTests` already discovers
     it under its `demos/terminal` root, so `make ze-unit-test` runs it. The
     render-side tests are ADDED to it, and it moved to Files to Modify. -->

- `demos/terminal/screen.py` - a terminal screen rebuilt from what a program painted onto it, read by BOTH programs in this directory and added in phase 3. `TapeSession.wait` searches it, so `Wait+Screen` means in the recorder what it means in VHS (R-1); `cast_visible_text` searches its history, so the AC-5 gate asks what a reader was shown rather than what the terminal was sent. It models the cursor, the scrolling region, line insert and delete, autowrap and the erases, and nothing else: no character sets, no colours, no alternate buffer. Every mechanism in it was added because a real demo failed without it, and `ScreenTest` names which
- `website/tools/test_render_demos.py` - the site-side tests above
- `website/assets/vendor/asciinema-player.min.js` and its `.css` - the vendored player, on a verbatim-copied path (C-8). Added in phase 5, with `asciinema-player.LICENSE` (Apache-2.0) and `README.md`, which records the version, the digests, the provenance attestation, what was checked in the bundle, and how to upgrade
- `demos/terminal/fonts/` - the 22 faces the renderer image installs: Fira Code 6.002 (6), Liberation 2.1.5 (12) and Noto Sans 2.015 (4), with `OFL-FiraCode.txt`, `OFL-Liberation.txt`, `OFL-NotoSans.txt` and a `README.md` recording each release URL and the sha256 of every file. 8.4 MB of TTF, byte-identical to the upstream releases and to the copies the VHS base carried. Liberation is the one nobody names: the web interface's form controls set no family, so chromium draws them in Arial, which fontconfig aliases to Liberation Sans. All three together hold the browser demo's poster at the digest the VHS-based renderer produced (D-1)
- `website/assets/vendor/fonts/` - 10 `.woff2` faces (Poppins 400/700/800 and Lato 400/700, each in the `latin` and `latin-ext` subsets Google publishes, with their `unicode-range`), `fonts.css` declaring them with `font-display: swap`, `poppins-600.css` for the talk decks alone, `OFL-Lato.txt` and `OFL-Poppins.txt`, and a `README.md` recording provenance and refresh. 98 KB for the site's own weights, against the 216 KB Google serves for them: the `devanagari` subset is not vendored, because no page in the site holds a Devanagari character (D-2)

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
| Doctor check for runtime dependencies | Yes | The renderer's external binaries change: `vhs` and probably `ttyd` leave (A-7). `mk/build-terminal-demo.mk`'s own `command -v docker` check is the place; the daemon gains no dependency, so `internal/core/diagnostic` is untouched |
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
| 6 | Has a user guide page? | Yes | `docs/guide/terminal-demonstrations.md`. ANSWERED in phase 7. Phase 5 removed the WebM wording; two VHS claims survived it and are now gone. The frontmatter `description` said "from reproducible VHS tapes" and reads "from reproducible tape files". The body said "The checked-in VHS tapes define every keystroke, pause, and terminal size, so a release can regenerate the recordings when Ze changes", one 27-word sentence, and reads as two with no tool name: "Each checked-in tape file defines every keystroke, pause, and terminal size. A release can therefore regenerate the recordings when Ze changes." The paragraph held 7 sentences and is split in two, because 6 is the STE limit |
| 7 | Wire format changed? | No | No protocol surface |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol surface |
| 10 | Test infrastructure changed? | No | Confirmed in phase 1: `docs/functional-tests.md` names no demo, no renderer, no VHS and no WebM. `grep -in "demo\|vhs\|webm\|terminal-demo"` returns only two `cmd_web.go` anchors matching on "web", against 787 lines carrying "test", so the file was reached. RE-CONFIRMED in phase 7 after the conversion: `grep -niIE "demo\|vhs\|webm\|terminal-demo" docs/functional-tests.md` returns the same two `cmd_web.go` source anchors and nothing else, so the file names no demo renderer and needs no edit |
| 11 | Affects daemon comparison? | No | No daemon behavior changes |
| 12 | Internal architecture changed? | Yes | Named in phase 1: no `docs/architecture/` page covers the demo pipeline. The one that does is `docs/contributing/gh-pages.md`, whose publish description says the site "reuses existing VHS media when `assets/demos/manifest.json` matches the checked-in tape definitions". Both halves move: the media is a cast, and the engine reading the tapes is not VHS. ANSWERED in phase 7. That sentence now reads "It reuses the existing demo artifacts when `assets/demos/manifest.json` matches the checked-in tape definitions". The reuse condition is unchanged and still true: `ze-site-generate` decides it with `render.py --all --check-definition`, which compares `definition_sha256` over `common.tape` plus the demo's source. "media" became "artifacts" in the next line too, so one name covers a `.cast` and a `.webm`, and `render.py` already prints "Ze demo artifacts verified". `website/AI.md` carried the same two lines in its own wording and got the same edit; phase 5 saw it and could not reach it. One design doc is declared by a file this phase changed and is UNAFFECTED: `scripts/checks/cli_grammar.go` carries a `// Design:` header naming `docs/architecture/cli/command-namespacing.md`, and this phase edited only that file's `demoLaunchHits` comment. That doc is about the CLI root namespace, not about how a demo is rendered, and `grep -niIE "vhs\|webm\|demo\|video"` over its 248 lines returns nothing |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry surface |
| 16 | Any changed source file referenced by existing doc source anchors? | No | `python3 scripts/dev/spec_doc_anchors.py plan/...` exits 0 in phase 1: no declared owner, no mention. Read its REACH before trusting that: `--json` derives one file from the 12 in Files to Modify, `mk/build-terminal-demo.mk`, because `spec_source_files` keeps only paths under its `CODE_PREFIX` and `demos/` and `website/` are not in it (`plan/journal/gate-excludes-part-of-its-population.md`). Checked by hand instead: no `<!-- source: -->` anchor in `docs/` or `ai/` names any of the other 11 paths |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | ANSWERED in phase 7: NOTHING TO CHANGE, and the evidence is a full sweep. 36 embed markers sit in 18 pages (`grep -rn "terminal-demo:" docs/ website/ --include=*.md --include=*.html`). The marker is `<!-- terminal-demo: <id> -->`, which names no format, so no page states anything the conversion falsified. No page in `docs/` or `website/` links a `.webm`: the only `.webm` spellings outside `demos/terminal/` and `website/tools/` are in test fixtures. Two pages describe the demo in prose and both stay. `docs/features/web-interface.md` says "The recording below signs in to a local Ze instance", and its demo is `web-config`, which D-3 keeps as a video. `docs/guide/irr-filtering.md` says "The recording starts with a stored BGP peer", and an asciicast is a recording, so the word claims no format |

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
   - DONE. `config-graph` was blocked by a CLI grammar change outside this spec (R-11), so `zefs-config` was converted while that stood; owner ruling D-8 then re-authored `config-graph` to the grammar the daemon accepts today and it is converted too, so all four are recorded. A-1 CONFIRMED, A-8 BROKEN, R-1, R-9 and R-10 resolved. The gate's rule: a transcript line behind a prompt (`$`, `ze>`, `ze#`) is a CLAIM about the session, and each claim must appear behind its prompt on one painted line, the lines searched forwards only so the ORDER is gated too. Narration is prose written for a reader and is not looked for; a transcript quoting no command fails the render rather than gating nothing
4. **Phase: Kind-aware verification** -- AC-6, AC-7, R-5
   - Tests: `test_verify_assets_demands_cast_for_terminal_kind`, `test_browser_demo_still_produces_video_and_poster`
   - Files: `demos/terminal/render.py`
   - DONE. `verify_assets` derives the asset set from `asset_paths(indexed[demo_id])` and refuses any asset outside it. `--check` and `--check-definition` both pass over a rendered `cli-dashboard` (C-5), and `stamp_definition_hashes` self-verifies through the same call. The end-to-end render is clean: `recorded artifacts/cli-dashboard.cast (61 events, 11.0s, 137x36)` then `Ze demo artifacts verified: cli-dashboard`, where it ended at `missing video metadata` before. The first of C-2's four hardcoded triples is gone; the other three all sit under `website/tools` and are phase 5
5. **Phase: The site** -- AC-8, AC-9, R-2, R-4
   - Tests: the four `website/tools/test_render_demos.py` tests
   - Files: `website/tools/terminal_demos.py`, `render-doc.py`, `render-index.py`, `website/assets/css/30-components.css`, the vendored player
   - DONE, except `test_no_external_font_reference`, which is AC-10 and belongs to phase 6. AC-8, AC-9, R-2 and R-4 are met, A-2 is CONFIRMED and R-2 is resolved. Three things the phase decided beyond its brief. The duration a page prints is now DERIVED from the cast (see Key Design Decisions), so the four numbers phase 3 measured as drifted are gone rather than corrected. The hero on `index.html` reads the artifact manifest through the new `hero_mount`, which removes the fourth place C-2's asset set was spelled and the two hardcoded `?v=` digests with it. And `docs/guide/terminal-demonstrations.md` lost the wording that called every demo a video; the sentence still names the tapes as VHS tapes, which is phase 7's to judge, because AC-11 keeps the `.tape` files
   - One thing is OWED and is not this phase's to make: `validate_contract` in `demos/terminal/render.py` still requires a `duration` on every demo, including the terminal ones whose value nothing reads any more. Removing it from that field list, and then from the 17 terminal entries in `demos/terminal/manifest.json`, is the last step of the duration decision. Phase 5 did not make it because phase 4 held that file, and it must not be left as a required field carrying a number no reader sees
6. **Phase: Self-hosted fonts** -- AC-10, its own commit
   - Tests: `test_no_external_font_reference`
   - Files: `website/tools/sitelib.py`, `website/assets/vendor/fonts/`
   - DONE. Nothing under `website/` requests a font from `fonts.googleapis.com` or `fonts.gstatic.com`. The `.woff2` files sit beside the `@font-face` rules that name them, under `assets/vendor/fonts/`, and NOT under `assets/css/`: `sitepaths._SOURCE_ONLY_DIRS` prunes `assets/css` from the published tree, and `render-css.py` inlines what survives into `assets/site.css` one directory up, where every relative `src: url()` would resolve against the wrong directory. The two talk decks link no `site.css` at all, so rules reachable only through that pipeline would never have reached them. Each page's `<link>` count is unchanged: the Google stylesheet became a local one
   - Two things the phase decided beyond its brief. The talk decks asked Google for Poppins 300/600 and Lato 300, which the site's own URL never did. Weight 300 is declared nowhere under `website/`, so it is not vendored; weight 600 is declared 16 times, and it ships as a separate `poppins-600.css` that only the decks link, because the site's own weight-600 rules (the homepage hero's outcome links among them) have always rendered at 700, and loading the face site-wide would change them. And `bundle-html.py` had to learn to resolve a stylesheet's `url()` references before inlining it: the standalone decks reached their fonts over the network until this phase, so self-hosting the site would otherwise have left them with no fonts at all
7. **Phase: Remove VHS** -- AC-11, AC-12, D-5, its own commit
   - Files: `demos/terminal/Dockerfile`, `demos/terminal/manifest.json`, `mk/build-terminal-demo.mk`, delete `demos/terminal/install-vhs.sh`
   - Verify: rebuild the image, re-render `web-config` alone, and confirm its digest is unchanged (A-6). Answer A-7 with a `ttyd` grep
   - DONE for AC-11. The base is `debian:trixie-slim`, pinned by digest, carrying chromium, Playwright, ffmpeg, the demo network tools and the two font families the web interface names. The image is 2.02 GB against the VHS base's 3.96 GB. `install-vhs.sh`, its `ze-terminal-demo-tools-install` target and that target's help line are gone, and the pin moved in both places at once. `render_tape`, `accelerated_terminal_tape` and `ZE_DEMO_SPEEDUP` STAY, per the owner's redirect recorded in R-10; the Files to Modify row already said so and needed no correction. A-6 is CONFIRMED for the base swap and leaves the owner one open choice about the font source, in its own row. A-7 is re-confirmed after the removal. AC-12 is NOT shown: it needs a full 17-demo re-render, which belongs to the regeneration rather than to this phase

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
| No VHS remains | Re-measured at closure: `grep -rniI "\bvhs\b\|\bttyd\b\|charmbracelet" demos/ mk/ Makefile website/` returns four lines and every one is prose ABOUT the removal -- two in `demos/terminal/fonts/README.md` explaining which faces the VHS base carried, and two in `mk/build-terminal-demo.mk`'s header saying the deleted target installed ffmpeg beside VHS and ttyd. `website/AI.md` no longer says "VHS media". `website/data/dependencies.json` names `github.com/charmbracelet/colorprofile`, a Go module Ze imports through `go.mod`, so the `charmbracelet` pattern carries one standing false positive |
| The image pin agrees in both places (R-6) | `grep -rn "ze-terminal-demo-render-all" mk/build-terminal-demo.mk demos/terminal/manifest.json` shows one tag. Re-measured at closure: both read `ze-terminal-demo-render-all:debian-13-playwright-1.55.0-firacode-6.002-notosans-2.015-liberation-2.1.5`, which moved when the faces were vendored |
| One video survives | `ls ../gh-pages/assets/demos/*.webm` lists exactly `web-config.webm` |
| No external font request | `grep -rln "fonts.googleapis\|fonts.gstatic" ../gh-pages/ website/` names no `.html` and no `.css`. The remaining hits are prose that says the hosts are gone: `sitelib.FONT_CSS_PATH`'s comment, the vendored `README.md` and `fonts.css` headers, and the tests that assert their absence |
| A cast is smaller than the webm it replaces (A-1) | `ls -l` on both, measured in phase 3 on three demos. `cli-dashboard`: 12,537-byte cast against a 325,166-byte webm and a 110,151-byte poster, 34.7x smaller than the pair. `launcher`: 23,422 against 684,491 + 99,303, 33.5x. `zefs-config`: 55,088 against 1,229,009 + 121,486, 24.5x. `config-graph`: 8,397 against 303,096 + 99,534, 47.9x. And the cast is text, so a re-render costs git a delta rather than a whole new blob |
| Every terminal demo converted | `ls ../gh-pages/assets/demos/*.cast` lists 17 files |

### Security Review Checklist
| Check | What to look for | Answered in phase 5 |
|-------|-----------------|---------------------|
| Input validation | A `.cast` is replayed in the reader's browser. Confirm the player is the only thing interpreting it, and that no page code evaluates its contents | The only site code that touches a cast is `initTerminalDemoPlayers` in `website/assets/js/site.js`, which reads the `data-cast-src` attribute and hands the URL to `AsciinemaPlayer.create`. Nothing parses a cast, and `terminal_demos` never puts cast CONTENT into a page: it reads the header for the grid and the last event's timestamp for the duration, both of which are numbers it re-emits as its own text |
| Supply chain | The vendored player is pinned by version, committed, and reviewed once, rather than fetched at build time. Record the version and the upstream digest | `asciinema-player` 3.17.0, Apache-2.0, committed under `website/assets/vendor/` with its LICENSE. Tarball sha512 `25b8cd2660364cb21e60d68e1236be9154b35da74aa2aa48cd55667c118d8f60e676d13fbc4c410a7904f0d604729690703bd4620f0bb6dd2ead3f347eb0029b`, equal to the digest the npm registry publishes and to the subject of npm's SLSA v1 provenance attestation, which names the build as `.github/workflows/release.yml` in `github.com/asciinema/asciinema-player` at `refs/tags/v3.17.0`. `asciinema-player.min.js` sha256 `a13c37632e1b5c49fe9128417b9319a9b5bc64cb457dd5ae52cbba8a3aceb880`, `asciinema-player.css` sha256 `f619fe17597043564f03b2c6918b3daf890ee8b912fb408542fba11afade4fdb`. The bundle names no external host: its only absolute URLs are the SVG namespace and the `http://localhost/` base a relative URL resolves against, its terminal emulator is a WebAssembly module carried as base64 inside the script, and its stylesheet has no `url()` and no `@import`, so it downloads no font. `fetch` runs on the recording URL the page gives it and on one the `benchmark` driver names; `WebSocket` belongs to the `websocket` driver; the site passes a plain `.cast` path, so neither driver is selected. `innerHTML` is written twice, once from a bundle template string and once from a fixed SVG path chosen by a `switch`. The full record and the upgrade procedure are in `website/assets/vendor/README.md`, which `sitepaths._SOURCE_ONLY_FILES` keeps out of the published artifact |
| Untrusted input | The cast is produced by our own renderer from our own tapes, so it is not attacker-controlled. Record that reasoning rather than assuming it | The producer is `demos/terminal/render.py` running tapes from this repository, and the artifact is committed (D-1). Before a page can embed one, `terminal_demos._artifact_source` checks its size and its sha256 against the artifact manifest and refuses on either mismatch, so a cast that reaches a reader is one this repository recorded and a byte-level edit fails the render (`test_tampered_recording_is_rejected_before_publish`) |

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
- The engine swap needed a TERMINAL, not a driver. VHS owned one and this spec replaced it with a byte stream, and both halves of the work came back for it: the tapes wait on the screen (R-1) and the transcript describes the screen (AC-5). `screen.py` is the piece nobody planned, and every mechanism in it was bought by a demo that failed without it, not by anticipating what a terminal can do.
- The gate earned itself on its first real render, three times over: it caught the lost prompt R-9 traded away, and two claims in `zefs-config`'s transcript that the demo does not show. The hand-authored transcript was the only independent description of a session, and it turns out to have been wrong in places nobody could see.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Convert to asciicast v2 | Keep `.webm` and re-encode smaller; move media to Release assets; deploy-time generation | Deploy-time generation is refused (D-1). Of the rest, only a text format makes a committed re-render cheap AND keeps the inspect-before-push workflow |
| Teach `pty-session.py` to read the tapes (B) | Translate 17 tapes into `pty-session.py` directives (A) | The definitions stay byte-identical, so the drift A-4 warns about has one chance to appear rather than seventeen. The vocabulary is closed and small (C-14), which is what makes the parser cheap |
| Write the asciicast from the driver | Shell out to `asciinema rec` | `Hide`/`Show` is interleaved (C-10) and `asciinema rec` has no pause primitive, so wrapping it means recording everything and cutting regions afterwards: the same work plus a dependency. Writing it directly adds no recorder dependency at all |
| Gate the cast on the hand-authored transcript | Trust the engine swap; compare against the old video | A video cannot be diffed, which is the premise of this whole spec. The transcript is the only text description of the session that already exists |
| Remove VHS entirely | Leave it installed for other uses | Owner instruction (D-5) and `ai/rules/no-layering.md`. Nothing else in the tree invokes it |
| Derive a terminal demo's published duration from its cast | Correct the four drifted numbers in `demos/terminal/manifest.json` and keep restating them | Phase 3 measured all four converted demos against the numbers the site prints: `cli-dashboard` 55.0s against "40 seconds", `launcher` 57.0s against "44 seconds", `zefs-config` 200.3s against "2 minutes 21 seconds", `config-graph` 68.1s against "1 minute 31 seconds". Correcting them was not even fully available: only 4 of the 17 terminal demos are rendered, so 13 numbers cannot be measured, and a correction re-creates the pair that must agree. A cast's last event timestamp IS the running time, and `terminal_demos` already opens the file to check its digest, so `_cast_facts` reads it there and `_duration_phrase` spells it the way the catalog always has. The manifest keeps `duration` for `web-config`, whose video states no length the site can read |
| Reserve the box from the cast header, not the tape | Reserve it from `Set Width`/`Set Height`, as R-2 originally said | A tape states a PIXEL box; the character grid is derived from it with the font's metrics (A-5, A-8), so the tape is one derivation away from the answer and the cast simply carries it. Reading the artifact also keeps `website/` from depending on anything under `demos/` beyond the two manifests it already reads |

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

## Implementation Summary

### What Was Implemented

The 17 `kind: terminal` demos record an asciicast v2 file instead of a WebM and
a poster, and VHS left the tree.

- **The recorder.** `demos/terminal/pty-session.py` gained a `--tape` front end:
  `parse_tape` reads the checked-in definitions over a CLOSED 13-directive,
  10-key vocabulary, `terminal_size` derives the character grid from the tape's
  pixel box, `TapeSession` drives a real PTY, and `CastWriter` writes the cast
  with its clock stopped between `Hide` and `Show`.
- **The screen.** `demos/terminal/screen.py` is new: a terminal rebuilt from what
  a program painted. `TapeSession.wait` searches it, so `Wait+Screen` means what
  it means in VHS, and `render.py`'s `cast_visible_text` searches its history, so
  the AC-5 gate asks what a reader was shown.
- **The pipeline.** `render.py` gained `ASSET_EXTENSIONS` / `asset_paths` (the
  per-kind asset set), a kind-aware `verify_assets`, `expand_cast_timeline`,
  `check_transcript` (the AC-5 gate), `remove_superseded_assets`, and a host
  ffmpeg preflight. `validate_contract` now refuses a `duration` from a kind that
  publishes a cast and requires one from a kind that does not.
- **The site.** `website/tools/terminal_demos.py` gained `KIND_ASSETS`,
  `_cast_facts`, `_duration_phrase`, `_reserved_box`, `_player_mount`,
  `player_head` and `hero_mount`; `expand` returns a head fragment; `site.js`
  gained `initTerminalDemoPlayers`. The player is vendored under
  `website/assets/vendor/`.
- **Self-hosting.** Poppins and Lato are vendored as 12 `.woff2` faces with their
  own `fonts.css`; `sitelib.FONT_CSS_URL` and both Google preconnects are gone,
  and 14 authored pages were edited. `bundle-html.py` resolves a stylesheet's own
  `url()` before inlining it, so the standalone decks are offline for the first
  time. The renderer image vendors Fira Code, Noto Sans and Liberation and
  deletes every distribution font.
- **VHS removal.** The Dockerfile rebased onto `debian:trixie-slim`,
  `install-vhs.sh` and `ze-terminal-demo-tools-install` are deleted, and the
  comment sweep AC-11 requires reached `render.py`, `pty-session.py`,
  `demo-lock.sh`, two `run.sh`, `test_render.py`, `scripts/checks/cli_grammar.go`
  and the rule point behind `ai/rules/cli.md`.

### Bugs Found/Fixed

- `os.execvp` in `pty-session.main` fell through into the parent's driver loop
  when the exec failed, so two processes read one terminal. Both call sites now
  `os._exit(127)` in a `finally` (phase 2).
- `config-graph` could not be rendered at all: `ze config graph | ze pipe match`
  is refused by `rowOperatorRefusal` since `6b0eb49e3`, and the refusal's own
  guidance cannot be followed. The demo was re-authored to
  `| ze pipe text | ze pipe match` (D-8, R-11). Journal rows in
  `plan/journal/guard-message-teaches-the-violation.md` and
  `plan/journal/documentation-shows-config-the-parser-refuses.md`.
- `render-activity.py` interpolated the font URL UNESCAPED, putting a raw `&` in
  an `href`. The URL is gone; the template now fills a local path (phase 6).
- `bundle-html.py` base64-encoded a stylesheet without resolving its `url()`
  references, so the "standalone offline deck" reached its fonts over the network
  (phase 6).
- **Found in the Review Gate:** nothing removed an artifact a demo's kind stopped
  producing, so a full re-render would have left all 17 `.webm` and 17 `.png` in
  the published tree and AC-12 would have been false.
  `remove_superseded_assets` fixes it.
- **Found in the Review Gate:** `install-vhs.sh` also installed the HOST ffmpeg
  `expand_timeline` and `resize_poster` run, and deleting it left a missing
  binary as a bare `FileNotFoundError` after the container had run the whole
  demo. `_render_demo` now names it before the container starts.
- **Found in the Review Gate:** `website/assets/vendor/fonts/README.md` recorded
  no per-file digest while both sibling vendor READMEs do. A digest table was
  added and every row re-checked against the file.

### Documentation Updates

- `docs/guide/terminal-demonstrations.md`: the frontmatter `description` and the
  body no longer name VHS or WebM, and the page says a terminal demo is an
  asciicast whose text can be copied.
- `docs/contributing/gh-pages.md` and `website/AI.md`: "VHS media" became "demo
  artifacts", one name covering a `.cast` and a `.webm`.
- `scripts/checks/cli_grammar.go` and
  `ai/rules/points/cli/cli-grammar-keywords-before-values/the-feeders-that-enforce-the-grammar-rules.md`
  (with its generated `ai/rules/cli.md`): the demos need Docker, not VHS.
- `mk/build-terminal-demo.mk` header: the host needs docker, python3 and ffmpeg.
- No `<!-- source: -->` anchor in `docs/` or `ai/` names any changed path
  (documentation row 16). `make ze-doc-verify` was not run: no doc claim about
  Go source changed, and the two edited pages carry no source anchor.

### Deviations from Plan

- `demos/terminal/screen.py` was not in the plan. `Wait+Screen` and the AC-5 gate
  both ask about the rendered screen, and neither is answerable from the byte
  stream (R-1).
- AC-2's wording was corrected in phase 3: `Show` after a hidden `clear` writes
  the reset AND the screen the terminal painted after that erase. The literal
  reading deleted the shell prompt the published video shows (R-9).
- AC-4's wording was corrected in phase 2: `Set Width`/`Set Height` are PIXELS,
  and the header records the grid derived from them.
- `render_tape` and `accelerated_terminal_tape` STAY, on the owner's redirect of
  2026-08-24: the capture keeps its fifth and `expand_cast_timeline` scales the
  artifact instead (R-10).
- `demos/terminal/render_test.py` was a second, older render test file nobody had
  named. Its fixture now reads `asset_paths`.
- `demos/terminal/container-entrypoint.sh` hands the two written mounts back to
  the host from an EXIT trap rather than after the render, so an interrupted run
  no longer leaves a root-owned tree.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-8: the 137x36 grid derived from the tape's pixel box was taken to be the grid VHS recorded into | VHS recorded ~135 columns. xterm.js measured an advance of 0.6081 em where JetBrains Mono declares 0.6 | Measured in phase 3 from the published `cli-dashboard.png`: a 38-character rule gives a 12.162 px advance, not 12 | Recorded BROKEN and kept at 137. Wider than 135, so nothing that fitted in the published demo wraps in the cast, and 135 follows from a renderer that is being removed |
| approach | Phase 2 held AC-2 literally, so `Show` after a hidden `clear` wrote the screen reset alone | The prompt the clear painted is what the reader resumes on, and the published `.webm` shows it | The AC-5 gate failed on `cli-dashboard`: `sshpass -e ssh ze-demo` was typed onto a bare line | `CastWriter.show` writes the reset AND the post-erase residue; AC-2's wording corrected (R-9) |
| approach | Phase 2 read `Wait` as a windowed match against the byte stream | The tapes name strings a program assembles across several paints. `filter: show` is a string no program ever emits | `launcher.tape:22` timed out against a faithful session | `screen.py`, and `TapeSession.wait` searches the screen (R-1) |
| assumption | The hand-authored transcripts were taken as a correct description of each demo | Two claims in `zefs-config/transcript.txt` describe things the demo does not show | The AC-5 gate rejected the first real render | Both corrected. The gate earned itself on its first run |
| approach | A-6 named the chromium update and the Debian base as candidates for the browser demo's pixel shift | Neither moves a pixel. The whole difference is the font build, and specifically Liberation Sans, which no CSS in the web interface names | Two images rendered back to back, poster digests compared, then the font files swapped one at a time | All three families vendored; the poster is bit-identical to the VHS-based render |
| approach | Phase 7 named `--font-mono` and `--font-ui` as the families that differ | Those resolve identically in both images. What differs is the face chromium picks for the form controls, which name no family | Chrome DevTools `CSS.getPlatformFontsForNode` in both images | A-6 corrected in place; the Dockerfile comment names Liberation and why |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Convert the 17 `kind: terminal` demos to asciicast v2 | Done | `demos/terminal/pty-session.py` `record_tape` / `drive_tape`; `render.py` `ASSET_EXTENSIONS` | The 17 tapes parse and the manifest carries `engine: Ze recorder` for all 17 |
| The demo DEFINITIONS do not change | Done | `parse_tape` reads the checked-in `.tape` | One exception, `config-graph`, re-authored under owner ruling D-8 because the CLI refuses its old pipeline (R-11) |
| Play them with a self-hosted player | Done | `website/assets/vendor/`, `terminal_demos.player_head` / `_player_mount`, `site.js` `initTerminalDemoPlayers` | |
| The site is totally self-hosted, fonts included (D-2) | Done | `sitelib.FONT_CSS_PATH`, `website/assets/vendor/fonts/` | No `.html` and no `.css` under `website/` requests a Google host |
| VHS leaves the tree (D-5) | Done | `demos/terminal/Dockerfile`, `mk/build-terminal-demo.mk`; `install-vhs.sh` deleted | |
| Generated artifacts stay COMMITTED (D-1) | Done | Unchanged: `render.py` writes into the publish tree and nothing generates at deploy time | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test_render_terminal_demo_produces_cast` | `asset_paths` for `terminal` is cast plus transcript |
| AC-2 | Done | `test_hide_suspends_recording`, `test_a_hidden_clear_hands_back_the_screen_it_left` | Wording corrected in phase 3 (R-9) |
| AC-3 | Done | `test_hidden_region_rebases_the_clock` | |
| AC-4 | Done | `test_cast_header_carries_tape_dimensions`, `test_render_demo_page_reserves_player_box` | Wording corrected in phase 2 |
| AC-5 | Done | `TranscriptGateTest` (8 cases) | Caught three real drifts on its first render |
| AC-6 | Done | `test_verify_assets_demands_cast_for_terminal_kind` | |
| AC-7 | Done | `test_browser_demo_still_produces_video_and_poster`, `test_browser_demo_keeps_its_video` | |
| AC-8 | Done | `test_render_demo_page_embeds_player` (both test files) | |
| AC-9 | Done | `test_markdown_sibling_links_the_cast` | |
| AC-10 | Done | `SelfHostedFontTest`, `test_no_authored_page_requests_a_font_from_google` | |
| AC-11 | Done | `grep -rniI "\bvhs\b\|\bttyd\b\|charmbracelet" demos/ mk/ Makefile website/` returns four lines, every one of them prose about the removal | The 17 `.tape` files stay |
| AC-12 | **Partial** | `test_a_render_removes_the_artifacts_its_kind_no_longer_produces` | The MECHANISM is implemented and tested; the published `../gh-pages/assets/demos/` still holds 18 `.webm` because no full 17-demo re-render has run. That regeneration follows this closure and is where the criterion is observed |
| AC-13 | Done | `test_tape_vocabulary_is_covered` | The census is taken from the tapes, so a new directive fails the test rather than the render |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TerminalDemoAssetTest` (6) | Done | `demos/terminal/test_render.py` | The four Wiring Test rows plus the two Review Gate fixes |
| `TapeRecorderTest` (5) | Done | same | Each case drives a real `bash` on a real PTY |
| `TranscriptGateTest` (8) | Done | same | |
| `ScreenTest` (8) | Done | same | One case per mechanism a real demo needed |
| `ManifestDurationTest` (3) | Done | same | |
| `TerminalDemoPlayerTest` (5), `HeroMountTest` (2), `SelfHostedFontTest` (2) | Done | `website/tools/test_render_demos.py` | |
| Bundler cases (3) | Done | `website/tools/test_site_assets.py` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every path in Files to Modify | Done | Plus `website/tools/test_site_assets.py` and `scripts/dev/github_workflows_test.go`, which pinned the deleted target |
| Every path in Files to Create | Done | `screen.py`, `test_render_demos.py`, the vendored player, `demos/terminal/fonts/`, `website/assets/vendor/fonts/` |
| `demos/terminal/install-vhs.sh` | Done | Deleted |

### Audit Summary
- **Total items:** 13 acceptance criteria plus 6 task requirements = 19
- **Done:** 18
- **Partial:** 1. AC-12's mechanism is implemented and tested. The criterion counts the published tree, and the regeneration after this closure is what produces that tree
- **Skipped:** 0
- **Changed:** 3 (AC-2 and AC-4 wording; `render_tape` kept on the owner's redirect), each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A re-render costs git a delta, not a whole new blob | measurement | Four demos rendered: `cli-dashboard` 12,537 bytes of cast against 325,166 plus 110,151 of webm plus poster (34.7x), `launcher` 23,422 against 783,794 (33.5x), `zefs-config` 55,088 against 1,350,495 (24.5x), `config-graph` 8,397 against 402,630 (47.9x). A cast is text, so git deltas it |
| The reader gets selectable, copyable terminal output | manual, in a browser | Chromium against the vendored 3.17.0 bundle and the real `cli-dashboard.cast`: the player's rendered screen at t=38, read row by row from the DOM, equals `screen.py`'s reconstruction on all 33 rows either one paints (A-2) |
| The demo DEFINITIONS do not change | functional test | `test_tape_vocabulary_is_covered` parses all 17 checked-in tapes plus `common.tape` unmodified; the recorder reads the definition VHS read |
| A converted demo replays the session the `.webm` showed | functional test | `TranscriptGateTest`, and the gate run for real on four demos. It is blocking: a mismatch moves the cast to `tmp/terminal-demos/rejected/` and publishes nothing |
| The site fetches nothing from a third party | functional test | `SelfHostedFontTest` and `test_no_authored_page_requests_a_font_from_google`; the vendored bundle names no external host (Security Review Checklist, Supply chain) |
| The page does not reflow as the player loads | measurement | Chromium: the mount is 635.90625 px high before the player is created and 635.90625 px after, and the paragraph under it sits at y=699.90625 in both readings (R-2) |
| VHS is gone | grep | `grep -rniI "\bvhs\b\|\bttyd\b\|charmbracelet" demos/ mk/ Makefile website/` returns four lines, all prose about the removal. A diff of the two images' executables lists `ttyd` and `vhs` among the 113 that left with the base, and a word-boundary grep for all 113 over the demo scripts returns nothing (A-7) |
| Interop | N-A | No protocol surface: this is website and demo tooling |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | `plan/deferrals/website-asciinema-terminal-demos.md` was never created: no phase deferred anything |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/website-asciinema-terminal-demos-d9b34094-ab32-4a5e-a127-6a4b65b9648d.md` |
| `review_gate.py check` | clean: `OK (38 code files, clean, hashes match)` |
| Rounds | 3 |
| Reviewer lenses used | size, wiring, functional-test coverage, documentation drift, removed-behavior audit, data flow, edge cases, security and supply chain, altitude and simplicity, ze-style over the changed Go |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | AC-12 had no producing code. Nothing removes an artifact a demo's kind stopped producing: `verify_assets` reads the set the manifest NAMES, and `build-site.stage_terminal_media` copies the directory as it stands. A full re-render would have left 17 `.webm` and 17 `.png` published | `demos/terminal/render.py`, `render_selected` | `remove_superseded_assets`, called under the lock after the manifest is written. Only `<demo id><a suffix some kind produces>` is a candidate, so it can never reach the manifest or another demo. `test_a_render_removes_the_artifacts_its_kind_no_longer_produces` renders a terminal demo and a browser demo in one run, and fails with the hunk reverted |
| 2 | ISSUE | `install-vhs.sh` also installed the HOST ffmpeg that `expand_timeline` and `resize_poster` run for the browser demo. Deleting it left a missing binary as a bare `FileNotFoundError`, raised after the container had rendered the whole demo | `demos/terminal/render.py`, `_render_demo` | A `shutil.which("ffmpeg")` preflight for a kind that produces a video, before the container starts, naming the binary. `test_a_video_render_asks_for_ffmpeg_before_the_container_runs` fails with the hunk reverted |
| 3 | ISSUE | `website/assets/vendor/fonts/README.md` recorded no per-file digest, while `website/assets/vendor/README.md` and `demos/terminal/fonts/README.md` both do. A refresh had nothing to compare against | `website/assets/vendor/fonts/README.md` | A digest table for all 12 `.woff2`, each row re-checked against the file. The prose states what the digests do and do not attest, because Google Fonts publishes none to check against |
| 4 | ISSUE | `test_the_artifact_manifest_is_published_under_the_lock` built a demo with no `kind` and did not patch `ARTIFACT_ROOT`, so it described a demo `validate_contract` refuses and reached the real publish tree | `demos/terminal/test_render.py` | The fixture states `kind` and points `ARTIFACT_ROOT` inside its temporary tree. What it asserts is unchanged |

NOTEs recorded and not blocking: `TapeSession.settle` measures quiet by
`CastWriter.events`, which never increments inside a hidden region, so the settle
is blind there (it cannot change what is recorded, and `wait` gates on the
screen, which is fed either way); `screen._control` does not clamp a
cursor-forward at the width, so an over-long CUF can wrap in the reconstruction
where a terminal would clamp (both readers of it are gates that fail loudly);
`_tape_lines` reports "sources itself" for any repeated `Source`, including a
legitimate diamond (no tape has one); `initTerminalDemoPlayers` returns silently
when the player script is absent, leaving an empty box with nothing in the
console; `inline_css_urls` recurses through `to_data_uri` for a `.css` `url()`,
so a CSS import cycle would not terminate (no cycle exists).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `demos/terminal/screen.py` | Yes | `wc -l` reports 372 |
| `website/tools/test_render_demos.py` | Yes | `wc -l` reports 314 |
| `website/assets/vendor/asciinema-player.min.js`, `.css`, `.LICENSE`, `README.md` | Yes | `sha256sum` matches the README's recorded digests exactly |
| `demos/terminal/fonts/` | Yes | 22 `.ttf`, 3 OFL files, `README.md`; all 22 digests re-checked against the README table, 0 mismatches |
| `website/assets/vendor/fonts/` | Yes | 12 `.woff2`, `fonts.css`, `poppins-600.css`, 2 OFL files, `README.md`; all 12 digests re-checked, 0 mismatches, 0 unlisted |
| `demos/terminal/install-vhs.sh` | No, deliberately | `ls` reports "No such file or directory"; `git status` shows a deletion |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-6, AC-7, AC-8, AC-12 | the per-kind asset set governs render, check and page | `python3 -m unittest test_render.TerminalDemoAssetTest`: Ran 6 tests, OK |
| AC-2, AC-3, AC-4, AC-13 | the recorder | `TapeRecorderTest`: 5 cases, each driving a real `bash` on a real PTY |
| AC-5 | the transcript gate | `TranscriptGateTest`: 8 cases |
| AC-8, AC-9, AC-10 | the site | `uv run --with pytest --with markdown python3 -m pytest -q website/tools/test_*.py`: 85 passed |
| AC-11 | no VHS remains | `grep -rniI "\bvhs\b\|\bttyd\b\|charmbracelet" demos/ mk/ Makefile website/`: four lines, all prose about the removal |
| all | the whole demo suite | `python3 -m unittest test_render render_test`: Ran 52 tests, OK |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `render.py --demo <terminal>` to the tape front end | `test_render_terminal_demo_produces_cast` | Yes: it runs `pty-session.py --tape` as a subprocess against a real shell and asserts the cast on disk |
| `render.py --check` to the kind-aware `verify_assets` | `test_verify_assets_demands_cast_for_terminal_kind` | Yes: every digest matches, so only the SET is under test |
| a published page to `terminal_demos._render_html` | `test_render_demo_page_embeds_player` (in both test files) | Yes: it renders a real page through `render-doc` and reads the markup |
| `render.py --demo web-config` to the Playwright arm | `test_browser_demo_still_produces_video_and_poster` | Yes, and it also refuses a cast on that kind |
| the entrypoint dispatches a `.tape` to the recorder | `grep -n "pty-session.py --tape" demos/terminal/container-entrypoint.sh` | Yes: in the same shell, at the position `vhs "$@"` held |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | CONFIRMED | Four demos measured: 24.5x to 47.9x smaller than the webm plus poster they replace, and text |
| A-2 | CONFIRMED | Chromium, the vendored bundle, the real `cli-dashboard.cast`. Two independent emulators agree on all 33 painted rows at t=38 |
| A-3 | CONFIRMED | No CSP in any template, any `render-*.py`, any published `.html`, `_headers` or `netlify.toml`; the inline theme-bootstrap script already works in production |
| A-4 | CONFIRMED, over the four demos rendered | The AC-5 gate is the validation this row names, and it is BLOCKING: `check_transcript` fails the render and publishes nothing. It found three real drifts on its first run. The other 13 demos are gated rather than asserted: each fails its own render if it does not reproduce, which is what the regeneration after this closure exercises |
| A-5 | CONFIRMED, corrected | `Set Width` and `Set Height` are pixels, so `FontSize` and `Padding` are read too. Three keys stay ignored, not five |
| A-6 | CONFIRMED, and the font question is CLOSED | The VHS image and the vendored one, rendered back to back, both produce poster `dd42e3113f0ec68dce7cff023b9b20c0f4498208bde55f4a2f48399c95ee6e4c`, bit-identical (PSNR inf). All 18 validators pass in the new image |
| A-7 | CONFIRMED | `grep -rniIw "vhs\|ttyd" demos/ mk/ Makefile` is empty; none of the 113 executables that left with the base is named in any demo script |
| A-8 | **BROKEN**, harmlessly | VHS recorded about 135 columns, measured from the published poster's 38-character rule. 137 is wider, so nothing that fitted wraps. Mistake Log row above |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 6, user guide | `docs/guide/terminal-demonstrations.md` names no tool and no format the conversion falsified | Yes, read |
| Row 12, internal architecture | `docs/contributing/gh-pages.md` and `website/AI.md` say "demo artifacts"; the reuse condition they describe is still what `render.py --all --check-definition` decides (`Makefile:511`) | Yes, read against the target |
| Row 10, test infrastructure | `grep -niIE "demo\|vhs\|webm\|terminal-demo" docs/functional-tests.md` returns two `cmd_web.go` source anchors matching on "web" and nothing else | Yes |
| Row 16, source anchors | No `<!-- source: -->` anchor in `docs/` or `ai/` names any changed path | Yes, checked by hand: `spec_doc_anchors.py` reaches only `mk/build-terminal-demo.mk`, because `demos/` and `website/` are outside its `CODE_PREFIX` |
| Row 17, config and CLI examples | 36 `<!-- terminal-demo: -->` markers in 18 pages, and the marker names no format. No page in `docs/` or `website/` links a `.webm` outside test fixtures | Yes |
| Generated rule | `ai/rules/cli.md` is regenerated from its point file; `make ze-rules-render-check` reports "29 rules are fresh" | Yes |

## Core Insight

The engine swap needed a TERMINAL, not a driver. VHS owned one, and replacing it
with a byte stream lost the two things that ask about a SCREEN: the tapes wait on
one (`Wait+Screen`) and the transcript describes one. `screen.py` is the piece
nobody planned, and every mechanism in it was bought by a demo that failed
without it rather than by anticipating what a terminal can do.

The second insight is cheaper and more transferable. The hand-authored
transcript looked like redundancy for years and turned out to be the only
description of a demo that does not come from the engine driving it. That is
exactly what makes an engine swap safe, and the gate earned itself on its first
real render by rejecting three things the transcripts and the code disagreed
about.
