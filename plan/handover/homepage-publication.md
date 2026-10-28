# Handoff: homepage publication

**Spec:** `spec-homepage-first-interaction.md`, removed at closure by `e6e6089851`; its implementation and acceptance evidence are preserved in `b1dd567e07`.
**Branch:** `main`; canonical website checkout uses `gh-pages`.
**Goal:** Improve the homepage without losing information, repair the reported hero regressions, and commit and push the repository changes requested by the owner.

## Status

- Done: original homepage improvements in `b1dd567e07`, spec closure in `e6e6089851`, and the subsequent hero correction in `c881a217ed`.
- Done: canonical publication `594ebfe2144f64151d00766f7777d6c134183c37` pushed to `origin refs/heads/gh-pages`. The public response from `https://ze-software.net/` subsequently contained the corrected hero markup.
- Done: the repository snapshot captured at 2026-10-28T23:18:39Z was committed through immutable per-file hunk selections. The final groups are `a0589dd1d1` (104 protocol/source-policy files), `d8427e4380` (76 namespace-carrier files), and `5d752735a0` (the pre-existing deletion of `todo`). Earlier logical preservation commits precede them. These commits preserve other sessions' unfinished work and do not certify those implementations.
- In progress: other sessions continued editing after that snapshot. Their later edits remain in the shared working tree; do not discard them or treat them as an incomplete homepage change.
- Remaining: the main-branch push. The native push command refused because `./le verify changed` has not passed in this checkout and no commit is covered by its scoped record. No main push script was produced or run.
- Not done here: the ongoing BGP policy work belongs to `plan/release/fixes/spec-bgp-rfc8212-ebgp-default-reject.md`; the kernel-carrier conversion and its outstanding runtime/discrimination work belong to `plan/release/validation/spec-privileged-tests-run-in-a-user-namespace.md`. Neither spec was closed by these preservation commits.

## Why verification needs the owner

The observed `./le repo changed packages` result selected `./...` because `go.sum` changed. This makes the required run whole-tree, which the agent is not permitted to start on this Linux host. The existing verification record failed; preservation commit bodies name the known failures rather than claiming a new passing run. No gate was bypassed and no assertion was removed to make a commit pass.

## Homepage correction and evidence

The dark-theme `.hero-start-panel` rule painted another gradient and shadow over the shared outer hero. Removing that obsolete inner paint restored both edges. The capability groups retain all 17 destinations inside the left copy column; the two desktop columns are centered, and the supporting action/release row immediately follows them.

The final canonical site build and `site check` passed. Actual Chromium verification covered light and dark themes at widths 1440, 1320, 1081, 1080, 850, 560, 390 and 320: no horizontal overflow, no inner background or shadow, desktop column center differences below one pixel, and correctly contained and aligned release text. Desktop/mobile screenshots were inspected. The fresh browser error log was empty. Independent review was clean.

Original interaction proof also covered quickstart navigation, every keyboard-operated preview, no-JavaScript content, the genuine idle frame, replay from 00:00, the management chapter from 00:36, and the repaired seek/frame-step behavior. The focused homepage/media contract tests passed. The separately restored migration explanation assertion passed its targeted race-enabled test; no wider protocol verification was claimed.

Evidence is under `tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/`:

- `homepage-before.json` and `homepage-browser-proof.json`: original inventory and interaction proof.
- `hero-repair-proof.json`, `hero-repair.diff`, and `hero-repair-light-desktop.webp`: final layout observations and visual evidence; the JSON names the other captures.
- `final-snapshot-protocol.txt`, `final-snapshot-userns.txt`, their `.diff` files and corresponding `-patches/` directories: exact additional commit populations and immutable selected content.

The original independent review is `tmp/review/homepage-first-interaction-644a4e78-0122-4eeb-8642-eb50638a1641.md`. The later independent hero review found no issue. Independent test judges approved preservation of the captured protocol tests and all 43 changed RFC-tagged namespace units. The native commit trailers carry the approvals used by the gates, including the stronger `skipNoRaw` to `requireRaw` prerequisite replacement.

## Files already handled

- `internal/le/site/homebody.go`: canonical hero content and information preservation.
- `website/assets/css/30-components.css` and `website/assets/css/60-responsive.css`: shared gradient, column alignment and responsive layout.
- `website/assets/js/site.js`: genuine poster and unchanged replay/chapter behavior.
- `website/assets/vendor/README.md`, the player bundle, and `website/tools/asciinema-player-poster.patch`: reproducible underlying player correction and modification notice.
- `website/AI.md`, `website/style-guide/index.html`, and `docs/contributing/gh-pages.md`: current presentation and publication contracts.
- `docs/contributing/committing.md`: immutable hunk selections, native approval trailers and push procedure.

## Edits

No homepage code edit remains. Do not reopen the completed layout work merely because the repository-wide push is blocked. Resolve any verification failures through their owning work before publishing `main`; the preserved unfinished work is not a passing-test claim.

## Then

When the owner has a suitable quiet verification window, run from the main checkout:

```sh
./le verify changed
```

Let the native runner manage its stage deadlines; do not impose the short interactive tool timeout used for command preparation. Do not substitute `verify current`, `verify worktree`, a narrower test selection, an attestation, or a disabled gate.

Once the required verification passes, prepare the authorized push:

```sh
./le commit push "User requested: When you finish commit all the changes in the repo and then push it."
```

Read the resulting script and run `bash` with the exact `script=` path it prints. Never construct that path. The canonical website push already succeeded and need not be repeated.

## Retained artifacts and cleanup

The two owned preview servers were stopped and the owned browser tabs closed. The disposable minifier `node_modules` directory was removed, recovering about 4.8 MiB. Keep the approximately 2.1 MiB pristine player/minifier comparison and the evidence above until the owner accepts the repair and the main publication is settled.

`bin/le-homepage-ergonomics/` is the 160 MiB successful native site-build binary retained to reproduce the verified publication while other sessions' Go edits continue. Remove it after main publication is settled and a fresh native build is available. The broader native scratch directory also contains other work streams' appliance images, test runs and evidence; this homepage session did not create or remove those artifacts. Never delete that shared directory as homepage cleanup.
