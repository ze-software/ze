# Website Sources

Published site: landing page + talks. `./le site build` builds the Pages artifact into `../gh-pages`.

## Layout

Website sources live in `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`./le site build` replaces that worktree with the publishable artifact and keeps `.git`.
It reuses the existing demo artifacts unless the checked-in tape definitions changed.
Use `./le terminal-demo render-all` to force new demo artifacts.
A recording runs in a container this repository builds, so run
`./le terminal-demo image-build` once per checkout first. A render refuses to
start without it and names the action.
Commit `../gh-pages` without source code.

## Structure

```
website/
  .nojekyll                              -- copied to the artifact root
  CNAME                                  -- copied to the artifact root
  assets/
    css/                                 -- stylesheet sources
    js/                                  -- JavaScript sources
    *.svg, social-card.png               -- static public assets
  data/                                  -- curated JSON inputs only
  presentations/                         -- authored decks and slide content
  tools/                                 -- reserved source-only build boundary
  talks/
    linx-2026-06/
      index.html                         -- authored slide renderer
      slides.md                          -- authored slide content
      screenshots/                       -- frozen editorial assets
    netmcr-2026-04/
      index.html                         -- authored presentation
      screenshots/                       -- frozen editorial assets
```

`./le site build` stages sources into the artifact, refreshes native generated surfaces, removes source-only files from `../gh-pages`, and validates the deployment boundary.

## Site build tooling

The site shell and every page outside `talks/<slug>/` are generated from data
and Markdown, not hand-edited HTML. Structure:

```
website/
  data/
    nav.json                              -- single source for the mega-menu; the header producer
                                              renders assets/header.html, which every page loads
    features.json                         -- every card on features/index.html: section, category,
                                              status, chips, bullets
    milestones.json                       -- every node on milestones/index.html: date, title,
                                              category, blurb, and the blog week it links to
    topics.json                           -- controlled tag vocabulary for Changes chips
    audience.json                         -- homepage audience cards
    whats-new.json                        -- homepage editorial note
    dependencies.json                     -- direct Go dependency explanations
    command-equivalents.json              -- curated vendor equivalents keyed by Ze CLI paths
    page-links.json                       -- page navigation and external project links
    repo-facts.json                       -- the six committed counts about this repository
                                              (`./le site facts update` writes it; the build
                                              reads it rather than walking the tree)
```

The native implementation lives in `internal/le/site`. `./le site build`
stages tracked and untracked website inputs into the Pages artifact, refreshes
the command surfaces and asset bundles, and removes every source-only path.
`./le site check` checks the deployment boundary. The same package bundles
presentation decks with `./le site bundle input <deck.html>` and renders the
repository activity page with `./le site activity`.

The live CLI catalog is rendered in-process by `internal/le/docvalid`, which
also regenerates the wiki catalogue through `internal/le/wikicatalog`. The
documentation drift gate therefore reads the same typed command contract as
the site build and starts no interpreter.

Hand-authored pages (`zeledon/`, `labs/*/`, `style-guide/`, `performance/`)
hold their body content as HTML. The `authored` producer reads each one,
publishes its body through the shared page shell, and writes the `index.md`
mirror beside it, so an authored page carries the same head, canonical link,
header mount, page sidebar and stamped footer as a generated one. The sidebar
comes from `data/page-links.json`, so an authored copy in the source is
replaced rather than published. Talk decks under `talks/<slug>/` are frozen:
the same producer publishes each deck exactly as its author wrote it, and
writes no mirror, because a deck is its own document.

The `header` producer renders `assets/header.html` from `data/nav.json` and the
counts in `data/site-facts.json`. Menu changes update one fragment instead of
every page. The fragment spells the site root as the `__ZE_SITE_ROOT__` placeholder,
which `assets/site.js` substitutes for the mounting page's own root.

The repository command is `./le site build`. `./le site build partial` keeps an
existing full artifact and refreshes the staged sources, while `./le site build
output <directory>` names another artifact root. A full build recreates the
artifact boundary and can seed it from the current complete Pages checkout.

### Website architecture

- **Data sources.** Published pages come from structured data and Markdown.
  `data/nav.json` owns top navigation, and the other files in `data/` own their
  matching catalogues and generated pages. Markdown sources live in `website/`
  and `../docs/`.
- **Paths and deployment.** `resolvePaths` in `internal/le/site/paths.go`
  derives the checkout, source and artifact roots. `isSourceOnly` is the common
  deployment boundary used by the builder and the checker. Both are unexported:
  every caller of either one is inside the package.
- **The producer registry.** Every published route is written by exactly ONE
  named producer. A producer is a `Name` and a `Render` function, registered
  from the `init()` of the file that owns it, and `Render` ANSWERS the route of
  every page it wrote rather than declaring the routes it owns. A declaration is
  a second statement of one fact, and a producer that stops writing a page keeps
  declaring it; an answer derived from the writing cannot drift from the writing.
  `checkProducer` panics at init on an empty name, a nil `Render`, or a
  duplicate name. `writeNamedArtifact` writes a published file that is NOT a
  route -- a feed, a data file, a machine-reader answer -- and `namedArtifacts`
  is the list `checkNamedArtifacts` holds it against.
- **`Coverage`, and what `./le site check` refuses.** `Coverage` compares the
  routes the producers answered against the routes the artifact publishes.
  `Unclaimed` is a published route no producer wrote, which survives from the
  incremental seed with frozen content and a fresh timestamp. `Doubled` is a
  route two producers wrote, where registration order alone decides which
  content a reader sees. `./le site check` exits 1 on FOUR conditions, not one:
  a published source-only path, a public route with no Markdown mirror, a named
  artifact that is absent or empty, and a red `Coverage`. It takes the same
  `output <directory>` keyword the build takes, so a session can check the
  artifact it just built rather than only the published tree.
- **Command surfaces.** Three producers in `internal/le/site` publish them from
  the live JSON catalog. `commands.go` writes the CLI reference and the
  operator guide, `equivalentdetail.go` the command-equivalent detail pages, and
  `derived.go` the `llms.txt` command lines. `internal/le/docvalid` renders no
  published page. Its unexported `renderCommandSurfaces` emits the contract
  fixture, and the documentation drift gate compares each published page against
  it.
- **Presentation tools.** `internal/le/site.BundlePresentation` embeds
  images, fonts, stylesheets, slide sources and screenshots in one HTML file.
  `RenderActivity` derives the activity table from Git history.
- **CSS source layers.** `assets/site.css` is generated from
  `assets/css/site.css` by `internal/le/site.RenderCSS`. The source
  manifest imports `10-base.css` and the smaller token, component and
  responsive files. New CSS belongs in the smallest matching source file.
- **JS behaviour model.** The editable script is `assets/js/site.js`;
  `internal/le/site.RenderJS` writes the published `assets/site.js`. It
  loads `assets/header.html` into each page and provides navigation and search
  interactions. Page content remains useful without JavaScript.
- **Source-evidence links.** `initSourceLinks` turns a `<code>` span that looks
  like a repository path into a link to a forge, from the `sourceLinks` rules in
  `data/frontend-vocab.json`. A rule that points at another project MUST carry a
  `scope` list of path fragments (for example `["/compare/"]`), because its paths
  are ordinary words on every other page: VyOS owns `data/` and `Makefile`,
  freeRtr owns `cfg/` and `misc/`. An unscoped foreign rule sent the blog
  sentence about `data/` to the VyOS repository. Only the Ze rule applies
  site-wide.
- **Asset URLs and cache-busting.** Pages link `assets/site.css` and
  `assets/site.js` by a stable URL with no `?v=` query, so a stylesheet or
  script edit touches only the asset file, not every page. Freshness is left to
  HTTP cache validation (GitHub Pages serves an ETag with a short max-age).
  Never reintroduce a per-page version query: it rewrites every page on any
  asset change.
- **Prose number tokens.** Website-owned Markdown may embed `{{ze:<name>}}`
  tokens whose values come from `data/site-facts.json`. HTML output carries a
  `data-ze-stat` marker, while Markdown mirrors receive plain text.
- **The facts snapshot.** `data/site-facts.json` is written by the build, not by
  hand, and it is written BEFORE any page producer runs: the prose tokens, the
  homepage proof strip and `llms.txt` all read it back. Each number is derived
  from one input and `_sources` names that input. Two of them are not derived
  from this tree: the star count reaches api.github.com and keeps the previously
  published number when it cannot, saying so in `_sources`, and the command and
  configuration counts come from the binary this build compiled.
- **The RFC requirement ledger.** `data/rfc-requirements.json` is derived once
  per build by `publishRFCLedger` (`internal/le/site/rfcledger.go`), from one
  reading of the checkout through `rfc.Collect` and `rfc.NewRenderInput`, and
  never through `rfc.Check`. The `rfc-compliance` producer reads it back and
  writes the aggregate page at `/quality/rfc-compliance/` plus one detail page
  per summary stem at `/quality/rfc-compliance/<stem>/`. Every cell of a
  requirement row is `rfc.RequirementRows`, which is also what
  `rfc/requirements/<stem>.md` is rendered from, so the site and the repository
  cannot state different things about one requirement.
- **The RFC family renders once for two outputs.** `rfcdetail.go` declares each
  section of a detail page as a heading and a PAIR of functions, one for the
  markup and one for the Markdown mirror, so a section cannot reach the page and
  miss the mirror. `rfcmarkup.go` holds the primitives both call: the requirement
  anchor, the linked mention, the scrolling table container and the escaped cell.
  `rfcevidence.go` holds the evidence half of the page -- the gaps, the proof
  state of every tagged unit, the extraction sign-off and the superseded rows.
  The headline cards are `rfcCardsHTML` and `rfcCardsMirror` in
  `rfccompliance.go`, and the index page and every detail page render through
  them, so the family has one card, and each card declares the rule behind its
  own tone so `rfcToneLegendHTML` can publish it. `repositoryBlobURL` and
  `repositoryLineURL` in `rfcmarkup.go` are the only spelling of the code host:
  `doctransform.go` resolves a documentation link through them, and a proof-state
  row resolves a test tag's line through them. `rfcBindingOf` and
  `rfcLedgerCoverage.Binding` answer the population every published ratio is
  taken over: the gated requirements less the `{not-applicable}` set, which is
  scope rather than coverage. `rfcSatisfaction` declares which bucket binds and
  `rfcStanding` groups the binding buckets into the ratio cards, so the shares
  partition their denominator and the two pages never disagree about it.
  `rfcLedgerCoverage.Bucket` is the one translation between the index's bucket
  keys and a stem's own counters. A card's movement in the grid is derived from
  its tone (`rfcCardsIn`), the mechanism sits outside the grid in
  `rfcMechanismHTML`, and `rfcCheckHTML` renders the gate's findings from
  `rfc.Finding`, the parts each check held before it formatted its line.
- **Verification commands.** `go test ./internal/le/site` exercises the
  build boundary, source digest, asset expansion and deck bundling.
  `go test ./internal/le/docvalid -run CommandSurface` exercises native command
  rendering and drift detection.

### Markdown to HTML contract

Ordinary Markdown pages use the shared site renderer. A dedicated renderer is
justified when the HTML is computed from structured data, live command output,
source extraction or another input with its own transformation.

New standalone Markdown sources should carry scalar front matter:

```yaml
---
description: Configure and operate the Ze DNS resolver.
destination: docs/features/dns-resolver/
category: services
journey: Feature
table-columns: true
---
```

The supported fields are:

| Field | Requirement | Meaning |
| --- | --- | --- |
| `description` | Required for new pages | SEO and social-card description. |
| `destination` | Required when the command does not pass an output path | Site-relative directory or a path ending in `index.html`. The renderer rejects paths outside the site root. |
| `title` | Optional | Browser and social title. The first level-one heading is used when omitted. The body should still have one level-one heading. |
| `category` | Optional | Heading color: `operate`, `routing`, `automate`, `observe`, `secure`, `services`, or `platform`. |
| `journey` | Optional | Short label shown in the page hero. The renderer derives it from the destination when omitted. |
| `table-columns` | Optional, defaults to `true` | Enables shared show/hide controls for tables on the page. |

Front matter is deliberately limited to top-level `key: value` scalars. This
keeps the website build independent from a YAML package. Explicit arguments
from an existing batch builder take precedence, so old manifests can move to
front matter one page at a time.

With a `destination`, `./le site build` publishes the source as part of the
artifact. The renderer strips front matter, converts the supported Markdown,
applies the shared evidence and link passes, and writes the HTML destination
beside its `index.md` mirror.

Column controls are progressive enhancement. A table remains complete when
JavaScript is unavailable. With JavaScript, every table with at least two
columns gets a checkbox per heading, plus `All` and `Default` actions. At least
one column remains visible.

Place this marker immediately before a Markdown table when column controls are
not useful:

```markdown
<!-- table-columns: off -->
| Key | Value |
| --- | --- |
```

For a raw HTML table, set `<table data-column-controls="off">`. Set
`table-columns: false` in front matter only when every table on the page should
remain fixed.

### Markdown mirrors

Every published page gets an `index.md` sibling beside `index.html`. A reader
can request the same URL with `.md` and receive the source form used by
`llms.txt`.

Pages backed by Markdown publish that source with internal links rewritten to
their public siblings. Pages built from JSON produce HTML and Markdown from the
same typed input. Hand-authored HTML pages derive their mirror from the
published main content, and the homepage is one of them: its copy lives in the
build's own template, so a hand-written mirror would state that copy twice.


### Site design and content rules

Before adding or restyling a page or component, read `style-guide/index.html`
or the generated `style-guide/index.md`. Reuse the vivid candy claymation
language already in `assets/site.css`: seven category hues, clay card depth,
2px white "sugar coat" borders, masked grids, soft candy washes, and thin
concentric ring ornaments. Do not add one-off shadows, filled decorative balls,
opaque blobs, unrelated palettes, or custom components without updating the
style guide in the same change.

Navigation is part of the design system. `data/nav.json` owns the top
mega-menu and curated `llms.txt` sections. The build publishes the menu as
`assets/header.html`, and the footer keeps the page publication stamp. Asset
URLs carry no version query, so an asset edit does not rewrite every page.

The right page menu comes from `data/page-links.json`; page bodies do not carry
duplicate link lists.

Every factual claim must have evidence. A claim about a feature, protocol, lab,
benchmark, command, dependency, plugin, quality gate, or comparison must trace
to a source file, Markdown source, JSON data file, script, generated binary
output, or external reference. If a claim cannot be traced, do not publish it.

Prefer generated data over hand-maintained prose or tables. Lists and facts
should come from Markdown or structured data: `data/*.json`, `../docs/*.md`,
`go.mod`, registry extraction, YANG extraction, git history, or session `ze`
output. Extend a renderer or extractor before you hardcode a catalog in HTML.

Every published page needs an AI-readable `index.md` sibling generated from
the same Markdown or structured data as the HTML. `llms.txt` links the Markdown
page first and includes the human web URL.

`llms-full.txt` is the same curation with the bodies inlined: every published
page's Markdown mirror, each preceded by its title and its canonical URL. The
frozen talk decks are left out, because a deck publishes no mirror.

Every section opens with `## Section: <name>` and every page with
`### Page: <title>` followed by its URL. A page body carries headings of its
own, and those two prefixes are what tells the file's structure apart from the
pages it holds.

Its ORDER is a contract stated in `internal/le/site/llmsfull.go`, in
`llmsFullReadingOrder`. What Ze is and why it is worth evaluating comes first,
how to use it comes second. `data/nav.json` supplies each section's membership
and the order WITHIN a section, and supplies neither the section order nor what
a section is for: a menu is ordered for a menu. The build refuses four things by
name -- a page in no section, a page in two, a declared section no page fills,
and a menu whose dropdown order runs a usage section before an evaluation one.

The last section of `llms-full.txt` REFERENCES the Ze wiki: one title, one
public URL and one summary for each page, and no page body. The wiki stays its
own source of truth (`plan/spec-website-wiki-content-migration.md`,
2026-07-22). It is read from the committed `data/wiki.json`, so the build never
opens a wiki checkout and a machine without the sibling directory writes the
same artifact. `./le site wiki update` refreshes that file from a checkout and
`./le site wiki check` reports it stale. The order and the grouping are the
wiki's own `_Sidebar.md`, and a wiki page the sidebar does not list is refused
by name unless `accountedUnlisted` in `internal/le/site/wiki/index.go` already
states why it is out.

Changes to feature cards, audience data, navigation or command equivalents are
made in `data/*.json`, followed by `./le site build`. Command paths still come
from the live CLI catalogue, while vendor mappings remain curated in
`data/command-equivalents.json`.

### Plugin catalog

`reference/plugins/` is generated from registry data and optional local
`PLUGIN.md` metadata. Plugin cards, dependencies and detail pages remain
generated outputs. Changes to registry facts or plugin metadata are published
with `./le site build`.

The catalog renderer creates `reference/plugins/index.html`, its `index.md`
mirror, and one local `reference/plugins/<plugin>/` detail page
per registry entry. Card clicks must stay on the site. If the page needs a new
machine fact, extend the extractor or registry data instead of adding a
hardcoded list to the renderer.

## Presentation tooling

Published decks live at `talks/<slug>/`. `./le site update-talk talk <slug>`
refreshes a deck's statistics and activity page, then writes
`<deck>-inlined.html`. Use `./le site update-talk talk <slug> bundle-only` when
only the standalone bundle should change. The lower-level
`./le site bundle input <deck.html>` and `./le site activity output <file> days
<count>` actions remain available for one-off artifacts.

The activity page is one standalone document that carries its own stylesheet
inside it, because a deck embeds it as an iframe where no link to the site's
assets resolves. It draws the same measurement as `/project/activity/` and
presents it differently: the published page is light, full size, and opens on a
hero, while the deck embed is dark and sized in viewport units so the whole
widget fits one slide. The two stylesheets are separate files, and neither
rendering reads the other's rules. `today <date>` pins the window, so a
published deck keeps the year it was presented with.

## Adding a new presentation

1. Create a new directory under `talks/`.
2. Add `index.html` and, when used by the deck, `slides.md`.
3. Add frozen images under `screenshots/`.
4. Run `./le site update-talk talk <slug>`.

## Updating presentations

`./le site update-talk talk <slug>` replaces each former per-talk update
script. It refreshes live slide statistics and embedded activity when the deck
uses them, then regenerates the standalone bundle.

## Weekly Update Checklist

Trigger: a new `changes/posts/<start-date>.md` lands (the week's Discord
`ze-news` update -- this is the weekly changelog source; the `blog/` section
is now for occasional editorial articles, not the weekly update). Work through
this before considering the update done.

0. **Tag the week.** Give the new post a `tags:` front-matter line (comma-
   separated) drawn from `data/topics.json`: one atomic tag per protocol or
   subsystem the week actually touched (`BGP`, `IS-IS`, `L2TP`, `Web UI`,
   ...), plus `Under the Hood` / `Quality Improvement` for refactors and
   fixes, and the `meta` family (`Presentation: <venue>`, `IETF Draft: <name>`,
   `Interop`) for non-subsystem work. The category is the broad area (it picks
   the chip color), so the tag is always the specific thing -- `Routing` is
   never a tag. `./le site build` fails on a tag absent from `data/topics.json`; if
   the week did something genuinely new, add the tag to the vocabulary (with
   its category) rather than forcing a near-miss.
   Weekly RFC counts are historical at publication. Keep
   `ze-stat-snapshot: true` in front matter, never in a body HTML comment.

1. **Check Features for drift.** Did the week ship something with no card
   yet, or move a feature from Experimental to shipped? Add/move/edit its
   entry in `data/features.json` -- the intro paragraph's count is computed
   from the data at render time, nothing to hand-update. If the week is a
   genuine landmark (a whole protocol or subsystem's first appearance, not a
   routine improvement), also add one node to `data/milestones.json` so the
   Milestones timeline stays current -- keep it coarse, one node per
   capability class, dated to the week's `covers` start so it links to the
   right weekly page under `changes/`.
2. **Check Compare for drift.** Did the week close one of the
   "Where Ze is behind today" gaps, or change a Yes/No/Partial cell? Edit
   `../docs/comparison.md` first (the source of truth), then copy the
   change into `compare/comparison.md` here (or re-run whatever produced
   that mirror). Bump the `Last updated:` date in main's file and the "as
   of" date in this file's disclaimer when real content changes -- never let
   this page carry content main doesn't also have.
3. **Check Labs for drift.** Did the week add new interop/QEMU evidence
   substantial enough for its own lab page (see `labs/bgp-interop/` as the
   template)? A new lab also needs an entry in `data/nav.json`'s Labs
   dropdown.
4. **Check Performance for drift.** Did `../docs/performance.md` get a
   fresh benchmark run this week? If so, update the headline stat-row on
   `performance/index.html` (Convergence/Throughput/Withdrawal) to match.
5. **Run `./le site build`.** One command regenerates the website artifact,
   including native page surfaces, assets, presentation activity, and
   standalone talk bundles. It stages the result in `../gh-pages` and removes
   source-only inputs before returning.
6. **Check the artifact before calling it done.** Run `./le site check` to
   verify the deployment boundary and Markdown mirrors in `../gh-pages`.
7. **Never edit `talks/<slug>/` content** as part of this checklist. Those decks
   are historic snapshots frozen at the time they were given.
