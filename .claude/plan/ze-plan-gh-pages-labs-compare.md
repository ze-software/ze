# gh-pages: dedicated pages for Comparison + QEMU/interop Labs

## Context

Across this conversation we analysed plakar.io against Ze's current landing
page (`../gh-pages/index.html`, 2483 lines, single self-contained page) and
found:

- A mislabeled link (`index.html:2345-2351`): text "Looking Glass" / "peer and
  route viewer" points at `docs/comparison.md`, not
  `docs/features/looking-glass.md`. Both files exist; this is the only
  existing link to the comparison doc anywhere on the site.
- `docs/comparison.md` (401 lines) is a rigorous, mostly-honest 11-implementation
  BGP comparison (Ze, rustbgpd, BIRD 2/3, bio-rd, RustyBGP, FRR, GoBGP, ExaBGP,
  OpenBGPd, freeRtr) but has no real entry point from the site, and its
  "Positioning" paragraph for Ze itself is pure marketing copy next to frank,
  caveated paragraphs for every competitor.
- The repo has real, currently-invisible interop/QEMU labs proving Ze against
  **real third-party software** (xl2tpd, accel-ppp, strongSwan) plus wire-level
  and workflow proof (VLAN QoS, Looking Glass topology, appliance installer,
  VPP dataplane) -- none of it surfaced on the site.

The user wants: (1) the comparison surfaced honestly, including an explicit
"where Ze is behind" section, and (2) a dedicated page per QEMU/interop lab so
a visitor can see how to actually run and use the software -- not just read
prose. Content depth agreed: **descriptive first pass** (accurate text, exact
commands, links to real source docs/scripts -- no fabricated output), then
check back in before attempting to actually run labs and capture real
terminal output/screenshots as a phase 2. Styling agreed: **shared
stylesheet**, not self-contained pages (unlike `presentations/`, which stays
self-contained per its own convention in `AI.md`).

This plan is scoped to structure + content sourcing for the new pages only.
It does **not** include the earlier-discussed hero rewrite, card-grid
regrouping into pillars, or accent-color reduction on the existing
`index.html` sections -- those were offered as separate recommendations and
are out of scope here unless folded in later.

## Site map

```
gh-pages/
  index.html                       -- existing page: nav gets 2 new entries
                                       (Compare, Labs), the mislabeled
                                       link-list entry is fixed to point at
                                       the new Compare page, and a compact
                                       "Labs" teaser section (3-4 cards + a
                                       "see all labs" link) is added
  assets/
    site.css                       -- NEW: index.html's <style> block
                                       extracted verbatim (no redesign),
                                       index.html switches to <link
                                       rel="stylesheet" href="assets/site.css">
                                       so every new page shares one source
  compare/
    index.html                     -- NEW: the comparison page
  labs/
    index.html                     -- NEW: labs overview/catalog (what "labs"
                                       means, the honesty note on
                                       prerequisites, cards linking to each)
    l2tp-interop/index.html        -- NEW
    pppoe-interop/index.html       -- NEW
    ipsec-interop/index.html       -- NEW
    vlan-qos/index.html            -- NEW
    looking-glass-graph/index.html -- NEW
    appliance-install/index.html   -- NEW (covers HTTP/PXE, ISO, Ventoy,
                                       failure-path scenarios together --
                                       one capability, four QEMU scripts)
    vpp-dataplane/index.html       -- NEW
```

9 new HTML files + 1 new CSS file. Every new page reuses the same
header/nav/footer markup as `index.html` (brand, nav links, footer license +
links), linking `assets/site.css`, so navigating between pages feels like one
site, not a bolt-on.

## Shared stylesheet

Extract the full `<style>` block from `index.html` (currently inline, lines
~27-998) into `assets/site.css` unchanged, then replace it with a single
`<link rel="stylesheet" href="assets/site.css">`. Pure extraction, no visual
change to the existing page. New pages link the same file and reuse the same
class names (`.card`, `.section-head`, `.status-panel`, `.link-list`, etc.)
so lab/compare pages look native rather than inventing new components.

## Per-page content sourcing (verified this conversation, no fabrication)

Each lab page: what's proven, prerequisites (stated plainly, including known
macOS/Docker-Desktop limitations where the source docs already say so), the
exact command, and a link to the real source for anyone who wants the detail.

| Page | Proves | Source (cited) | Command |
|------|--------|-----------------|---------|
| `labs/l2tp-interop/` | Ze as LNS against real `xl2tpd`/`pppd` LAC; FRR proves BGP redistribution of a subscriber /32 from a live PPP session | `docs/architecture/testing/l2tp-interop.md`, `test/interop-l2tp/lab.py` | `make ze-deployment-docker-l2tp-ppp-test` (Docker); `make ze-qemu-l2tp-ppp-test` (QEMU, needs `tmp/kernel/vmlinuz`, `mk/test-integration.mk`) |
| `labs/pppoe-interop/` | Ze's PPPoE client against real `accel-ppp`, "the dominant open-source BRAS/AC"; explains why L2TP-vs-accel-ppp isn't buildable (both LNS-only) | `docs/architecture/testing/pppoe-interop.md` | `make ze-deployment-docker-pppoe-accel-test`; `make ze-qemu-pppoe-accel-test` (`mk/test-integration.mk`) |
| `labs/ipsec-interop/` | Ze as IKE initiator against real strongSwan/charon, FRR redistribute scenarios | `test/interop-ipsec/lab.py` docstring, `Makefile:467` (no long-form design doc exists for this one -- page stays modest, doesn't invent rationale beyond what's written) | `make ze-interop-ipsec-test` (`IPSEC_INTEROP_SCENARIO=name` optional) |
| `labs/vlan-qos/` | 802.1p PCP tagging/classification actually on the wire (AF_PACKET capture + nftables counters), not just kernel-state acceptance. Existing README already has an honest "Limitations" section (veth/software only, single-tag only, no throughput assertions) -- reuse verbatim | `test/vlan-qos-lab/README.md` | `sudo ./test/vlan-qos-lab/run.sh` (`--selftest` for CI-style smoke) |
| `labs/looking-glass-graph/` | A realistic UK topology (AS65000 ring + edges, real external ASNs NTT/Cogent/Cloudflare/Akamai) populating the Looking Glass topology graph -- the one lab that's actually browsable/visual today | `test/plugin/lg-graph-lab/run.sh`, `lg-lab.py` | `./test/plugin/lg-graph-lab/run.sh [lg-port]`, then browse the printed URL |
| `labs/appliance-install/` | The installer boots and completes for real in QEMU across HTTP/PXE, ISO, and Ventoy-on-FAT paths, plus failure-path/fault/pin/rescue scenarios | Docstrings of `scripts/evidence/effective-install-qemu.py`, `-iso-qemu.py`, `-ventoy-qemu.py`, `-scenarios-qemu.py`; `docs/guide/ze-install.md` for the operator-supplied-kernel requirement | `make ze-qemu-install-test`, `ze-qemu-install-iso-test`, `ze-qemu-install-ventoy-test`, `ze-qemu-install-scenarios-test` (`mk/test-integration.mk`) |
| `labs/vpp-dataplane/` | Ze programs FIB, traffic, and firewall into a real VPP daemon via GoVPP; backs the production performance numbers already quoted in `docs/guide/vpp.md` (IPng Networks AS8298) | `scripts/evidence/effective-vpp.py` docstring, `docs/guide/vpp.md` | `make ze-deployment-vpp-test` (`mk/test-integration.mk`) |

## Comparison page (`compare/index.html`)

- Keep the existing disclaimer near the top (AI-assisted generation,
  informational only, corrections welcome via issue tracker) -- already
  honest, don't drop it.
- **New, prominent "Where Ze is behind today" section** placed before the
  detail tables (not buried in them), built directly from cells already in
  `docs/comparison.md`: no Confederation (RFC 5065) vs. BIRD 3/FRR/GoBGP/BIRD
  2/freeRtr; no dynamic/passive neighbors vs. five other implementations; no
  privilege separation (OpenBGPd's signature feature); BFD marked "Partial"
  vs. full support elsewhere; no embeddable library mode (bio-rd, GoBGP have
  one); no custom filter language (BIRD, OpenBGPd have one); no SR Policy;
  pre-release maturity / first release 2026 next to competitors with years to
  decades of production hardening; performance "not yet benchmarked at scale"
  (already-disclosed line in the doc).
- Port the 11 comparison tables (Overview, Address Families, Core Protocol,
  Cross-Protocol Redistribute, Policy & Route Manipulation, Security,
  Monitoring & Observability, API & Programmability, Operations, Best-Path
  Selection, BNG Capabilities) to styled HTML tables using the site's
  existing table/card CSS conventions.
- Rewrite Ze's "Positioning" paragraph to match the frank tone already used
  for every competitor (e.g. GoBGP's "higher memory and CPU usage... best as
  an SDN controller rather than a high-performance router") -- fold in the
  maturity gap and the unbenchmarked-performance caveat as first-class
  sentences, not an aside.
- Note at the top: this page mirrors `docs/comparison.md` as of its
  `Last updated` date, with a link to the source file on Codeberg/GitHub, so
  drift is visible rather than silent if only one copy gets updated later.

## index.html changes

1. Fix the mislabeled link-list entry: instead of pointing
   "Looking Glass" at `docs/comparison.md`, repoint that entry to
   `compare/index.html` with matching label/description ("How Ze
   compares -- and where it's still behind"). Looking Glass already has a
   correct link elsewhere in the Core cards grid, so nothing
   is lost.
2. Add "Labs" and "Compare" to the top nav (`.nav-links`, currently Status /
   Core / Experimental / Try / Talks / GitHub).
3. **Revised (user steer: the site should lean less single-page, not more)**:
   skip the card-grid "Labs" teaser section entirely. Instead add one more
   `.link-list` entry to the existing "Try" section ("Labs -- real interop
   proof you can run yourself" -> `labs/`), matching the low weight of the
   existing Compare entry. Discovery for both Compare and Labs is nav link +
   this one link-list row -- nothing heavier added to `index.html`. Applies
   more broadly too: prefer moving content to its own page over growing
   `index.html` for anything added from here on.

## Additions from VyOS research

VyOS (vyos.io/vyos.net), a much closer analog than Plakar (it's also an
open-source network OS), offered three transferable patterns -- approved by
the user with one constraint: **do not name VyOS or any other network OS by
name anywhere in Ze's copy.** (`docs/comparison.md`'s existing BGP-daemon
comparison table is a different, already-established category and is
unaffected -- this constraint is about full network-OS competitors.)

1. **Compare page gets an FAQ section**, in addition to the "Where Ze is
   behind today" callout already planned. VyOS's FAQ answers its community's
   actual skeptical question head-on ("does an EOL Debian base mean unpatched
   vulnerabilities?") instead of avoiding it. Ze's FAQ should do the same for
   its own known gaps, sourced from `docs/comparison.md`, e.g. "Ze is
   pre-release -- why should I trust it yet?" (point to the proof-strip: unit
   tests, e2e tests, fuzz targets, interop) and "Why no BGP confederations
   yet?" (state it plainly, no invented roadmap date). No competitor naming
   needed here since the FAQ is about Ze itself.
2. **`labs/index.html` adopts a clean link-card "hub" grid** (structural
   pattern only, not attributed to any source in the copy) rather than a
   denser feature-card layout.
3. **A plain, unattributed positioning line** (hero or Compare intro): Ze has
   no paywall, no subscription tier, no feature gating -- stated as a fact
   about Ze, not as a comparison to any named competitor.

## Palette redesign (added mid-implementation)

User direction: restyle to "deep pastel candy claymation" colors, applied to
the now-shared `assets/site.css` so every page (existing + new) inherits it.
Implemented as: deepened/saturated `:root` custom properties (richer versions
of the same semantic slots -- purple/cyan/teal/coral/lavender/sky/lime/pink/
gold/blue/green/amber), warmed backgrounds (cream/lavender tint instead of
cool blue-white), and a shared `--clay` box-shadow (inset highlight + soft
outer shadow) applied to card-like components for a molded look. Terminal
panel colors (`--term-*`) intentionally left dark/unchanged -- a code panel,
not a candy surface.

## Style Guide page (added mid-implementation)

Add a Style Guide page to the site documenting the palette (with swatches),
the claymation shadow/bevel treatment, typography, component patterns (cards,
buttons, badges, tables), and the content rules agreed in this conversation
(honesty about shortcomings, no fabrication, no naming other network OSes,
citation discipline for lab pages). Lives at `style-guide/index.html`, linked
from the footer. Reuses `assets/site.css` like every other page.

## Daemon vs. Appliance: first-class distinction (added mid-implementation)

User decision: Ze runs two ways -- as a **daemon on any existing Linux
distro** (systemd-managed, easier on-ramp for people fitting it into
existing infra) or as a **dedicated appliance** (gokrazy-built bootable
image, purpose-built hardware, read-only rootfs, no shell). This must be a
first-class distinction on the site, not buried. The current hero sentence
already gestures at it ("Ze creates appliances or makes Linux speak BGP...")
but doesn't make it a structured choice.

Applied as: (1) a small two-path callout on `index.html` (reuse existing
card patterns, stay lean per the earlier "don't grow index.html" steer --
two cards, not a section essay), each with its own quick-start command and
link (`docs/guide/docker.md`-style daemon install vs. `docs/guide/
appliance.md` gokrazy image); (2) a `Daemon` / `Appliance` / `Both`
card-label tag (reusing the existing `.card-label` pill styling) applied to
every Features card and every Labs card going forward. Verified from this
conversation's research: `labs/appliance-install/` is Appliance-only (it's
literally proving the installer/image chain); `labs/l2tp-interop/`,
`pppoe-interop/`, `ipsec-interop/`, `vlan-qos/`, `vpp-dataplane/`, and
`looking-glass-graph/` all run the `ze` binary directly in Docker/netns/QEMU,
not the gokrazy appliance image, so they tag `Daemon`.

## General-purpose doc-to-HTML renderer (added mid-implementation)

User steer: generalize the markdown-rendering approach beyond a one-off for
Compare -- build it like the existing presentation tooling
(`presentations/tools/bundle-html.py`), as a reusable script, so any doc
content can be published to the site and refreshed with one command when the
source `.md` changes in the main repo.

Design chosen over the earlier "fetch client-side at runtime" idea: a Python
script `tools/render-doc.py` (new `gh-pages/tools/`, sibling to
`presentations/tools/`) that reads a source markdown file (from the main
worktree, e.g. `../main/docs/comparison.md`), converts it (GFM tables
required) to HTML, wraps it in the site's shared page shell (header/nav/
footer, `assets/site.css`), and writes the output page. This is preferred
over a live cross-origin fetch (e.g. raw.githubusercontent.com) because it
matches the existing repo convention (Python scripts the maintainer runs
manually, committed output, same workflow as `bundle-html.py`) and avoids a
runtime dependency on GitHub's raw-content CORS/availability. "Keep it up to
date" = re-run the script after editing the source `.md`, then commit --
same discipline as regenerating an inlined presentation.

First use: `compare/index.html`, generated from `compare/comparison.md` (the
gh-pages-local copy seeded from `docs/comparison.md`'s tables plus the new
"Where Ze is behind," rewritten Positioning, and FAQ sections agreed above).

Natural follow-up (not in this pass unless requested): the same script could
replace the raw-GitHub-blob links scattered across the Features cards with
site-native rendered pages -- flagged as a known opportunity, not undertaken
now given the size of the current queue.

## Comparison page: markdown-sourced, HTML-rendered (revised)

Supersedes the earlier "hand-port the tables to static HTML" approach. The
whole Compare page's content -- disclaimer, "Where Ze is behind today"
callout, all 11 tables, the Positioning section (Ze's rewritten to match
competitor frankness), and the new FAQ -- lives in one markdown file,
`compare/comparison.md`, seeded from `docs/comparison.md`'s tables plus the
new sections agreed above. `compare/index.html` is a thin shell (site
header/nav/footer) that fetches `comparison.md` client-side and renders it
with a small pinned markdown library (GFM tables required), styled via a
`.md-content table` ruleset added to `site.css` to match the site's card/
claymation look. Updating the comparison going forward means editing one
`.md` file -- no HTML table editing, no regeneration step. The page still
notes it mirrors `docs/comparison.md`'s data as of a stated date with a link
to the canonical source, since the two files are not automatically synced.

## Homepage de-densification (added mid-implementation, confirmed via AskUserQuestion)

User steer, repeated across several messages: the site should lean away from
being a single page, and this applies to *existing* `index.html` content, not
just new additions. Confirmed split:

- **`features/index.html` (NEW)**: the Core (15 cards) and Experimental (9
  cards) feature-grid sections move here verbatim, as two labeled sections
  keeping the maturity distinction. This was by far the largest chunk of
  `index.html` (~900 of its original ~2483 lines).
- **`talks/index.html` (NEW)**: the Talks/presentations section moves here
  verbatim (still linking out to `presentations/*` exactly as before).
- **`index.html` keeps**: Hero, proof-strip, Status panel, a short feature
  summary paragraph + "See all features" link (replacing the full grid), Try
  (quickstart terminal + link-list, now including Compare and Labs entries),
  Audience ("Who should look now"), Closing, footer.
- Nav becomes: Status, Features, Experimental (anchor inside `features/` or
  drop if redundant with Features), Try, Labs, Compare, Talks, GitHub -- adjust
  anchors since `#core`/`#experimental`/`#talks` no longer exist on
  `index.html` once their sections move.

General principle going forward: prefer a new dedicated page over growing
`index.html` for anything from here on, existing or new.

## Daemon/Appliance gap closed (post-audit)

A completeness audit (user-requested) found two items recorded as decided but
never executed: Features cards had no mode tags, and the homepage had no
two-path callout. Both closed:

- All 39 Features cards tagged, verified per-card against
  `docs/features.md` (self-update backend differs but is supported on
  both) rather than assumed: 38 `Both`, 1 `Daemon` (Docker Support -- a
  packaging path distinct from the gokrazy appliance image). Labs' mode-tag
  colors normalized to match (`Daemon` = blue, `Appliance` = coral,
  consistent everywhere instead of varying per card).
- Homepage gets a lean "Two ways to run Ze" two-card callout between Status
  and the Features summary, each linking to its own quickstart/install doc.
- **Honest finding**: tagging revealed the daemon/appliance split barely
  fragments the feature set -- almost everything is `Both`, which is the
  correct reflection of "same binary, same config" (`docs/guide/
  appliance.md`), not a sign the tagging is low-value busywork.

## What's explicitly out of scope for this pass

- Actually running any lab/QEMU script to capture real terminal output or
  screenshots (agreed as phase 2 -- check in before starting).
- Hero rewrite, Core/Experimental card regrouping into named pillars, and
  accent-color palette reduction on the existing sections of `index.html`.

## Verification

- Every new page loads and links resolve (no 404s) when served locally, e.g.
  `python3 -m http.server` from `gh-pages/` and click through nav, teaser
  cards, and footer.
- `index.html` renders identically before/after the CSS extraction (visual
  diff / manual check) -- the extraction must be behavior-preserving.
- Every command shown on a lab page is copy-pasted verbatim from the
  `Makefile`/`mk/*.mk` targets cited above, not retyped from memory.
- Spot-check that no page states a capability, prerequisite, or command that
  isn't backed by the citations in the table above.
