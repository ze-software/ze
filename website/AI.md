# Website Sources

Published site: landing page + talks. `./le site build` builds the Pages artifact into `../gh-pages`.

## Layout

Website sources live in `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`./le site build` replaces that worktree with the publishable artifact and keeps `.git`.
It writes the artifact's own root `.gitignore` (`tmp/` and `.DS_Store`), which keeps
the commit scripts and logs sessions leave in that checkout off the published branch.
`website/.gitignore` is source and never ships: the build deletes any staged copy
before it writes the artifact's list (`internal/le/site/artifactignore.go`).
<!-- source: internal/le/site/artifactignore.go -- artifactIgnoreContent, writeArtifactIgnore -->
<!-- source: internal/le/site/build.go -- Build -->

It reuses the existing demo artifacts unless the checked-in tape definitions changed.
Use `./le site terminal-demo render all` to force new demo artifacts.
A recording runs in a container this repository builds, so run
`./le site terminal-demo build image` once per checkout first. A render refuses to
start without it and names the action.
Commit `../gh-pages` without source code.

The media lives in the artifact tree at `../gh-pages/assets/demos`. A render
writes it there, and a build reads it from there.

Private output is optional for previews and intermediate checks. To complete
approved website changes, run `./le site build` and `./le site check` against
the canonical `../gh-pages` checkout beside main. Use the native builder to
preserve `.git` and generated media. Local regeneration does not authorize a
push or deployment. Never push without owner instruction.

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
    features.json                         -- feature cards: category, the features/*.md ids
                                              they describe, chips, bullets; no status, the
                                              build derives shipped or experimental from the ids
    milestones.json                       -- landmarks grouped into monthly cards: date, title,
                                              category, blurb, and the source report
    topics.json                           -- controlled tag vocabulary for Changes chips
    audience.json                         -- homepage audience cards
    whats-new.json                        -- homepage editorial note
    dependencies.json                     -- direct Go dependency explanations
    command-equivalents.json              -- curated vendor equivalents keyed by Ze CLI paths
    page-links.json                       -- page navigation and external project links
    repo-facts.json                       -- committed repository counts, including Go source and test lines
                                              (`./le site facts update` writes it; the build
                                              reads it rather than walking the tree)
    github-releases.json                  -- the published releases the forge states
                                              (`./le site release update` writes it; the build
                                              reads it and never reaches the network for it)
```

The native implementation lives in `internal/le/site`. `./le site build`
stages tracked and untracked website inputs into the Pages artifact, refreshes
the command surfaces and asset bundles, and removes every source-only path.
`./le site check` checks the deployment boundary. The same package bundles
presentation decks with `./le site bundle input <deck.html>` and renders the
repository activity page with `./le site activity`.

The site build writes the live CLI catalog to its output directory.
The separate `./le cli catalog update file ../wiki/command-catalog.md` action
updates the wiki catalog. Both use the typed command registrations.

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
values in `data/site-facts.json`. A menu description names a count as a
`{{ze:<dotted.key>}}` token (`{{ze:cli_commands}} commands, ...`), which the
header resolves to its marked span and `llms.txt` to the plain value; the star
badge shows `github_stars` marked, and its aria-label names the badge without
the count. Menu changes update one fragment instead of every page. The fragment spells the site root as the `__ZE_SITE_ROOT__` placeholder,
which `assets/site.js` substitutes for the mounting page's own root.

The repository command is `./le site build`. `./le site build partial` keeps an
existing full artifact and refreshes the staged sources, while `./le site build
output <directory>` names another artifact root. A full build recreates the
artifact boundary and can seed it from the current complete Pages checkout.
Before it builds, it warns on stderr about every supported feature published on
a stale recorded run; `./le site build refresh` re-records those first
(`docs/contributing/feature-maturity.md`, "Recorded runs").

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
  artifact that is absent or empty, and a red `Coverage`. The root `.gitignore`
  is the one source-only name the artifact may carry, and only while it holds
  exactly the artifact's own list; any other content there is the source file
  leaking, and `publishedSourceOnly` reports it. It takes the same
  `output <directory>` keyword the build takes, so a session can check the
  artifact it just built rather than only the published tree.
- **Command surfaces.** Three producers in `internal/le/site` publish them from
  the live JSON catalog. `commands.go` writes the CLI reference and the
  operator guide, `equivalentdetail.go` the command-equivalent detail pages, and
  `derived.go` the `llms.txt` command lines. `internal/le/doc/yangcontract` renders no
  published page. Its unexported `renderCommandSurfaces` emits the contract
  fixture, and the documentation drift gate compares each published page against
  it.
- **Presentation tools.** `internal/le/site.BundlePresentation` embeds
  images, fonts, stylesheets, slide sources and screenshots in one HTML file.
  `RenderActivity` derives the activity table from Git history.
- **CSS source layers.** `assets/site.css` is generated from
  `assets/css/site.css` by `internal/le/site/assets.go`'s `renderCSS`. The source
  manifest imports `10-base.css` and the smaller token, component and
  responsive files. New CSS belongs in the smallest matching source file.
- **JS behaviour model.** The editable script is `assets/js/site.js`;
  `internal/le/site/assets.go`'s `renderJS` writes the published `assets/site.js`. It
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
- **Prose number tokens.** Website-owned Markdown and hand-written HTML pages
  may embed `{{ze:<dotted.key>}}` tokens, where the key is a dotted path of
  `data/site-facts.json`, such as `tests.unit_display` or
  `interop.scenarios_display`. The token names are the snapshot's own JSON
  keys, so a value the snapshot gains is a token with no edit anywhere. In HTML
  the value lands in a `data-ze-stat` span (`markedFact`,
  `internal/le/site/factmark.go`); in a hand-written page a token inside an
  attribute, inside `<head>` or inside a script, style or textarea takes the
  plain value instead. Markdown mirrors receive plain text. A key the snapshot
  does not name stops the build, naming the token, a close key and the `jq`
  command that lists every key. A first build into an empty tree has no
  snapshot yet and leaves every token alone. A hand-written page may also mark
  a value itself, `<span data-ze-stat="<key>">fallback</span>`: the build
  writes the snapshot's value into it when the key holds one and keeps the
  fallback while the key's section is `null`, as `site.js` does
  (`markedValue`, `internal/le/site/factfallback.go`). The element holds text
  only: markup or a token inside it stops the build even before the key holds
  a value. The appliance lab's ISO size is one: "under 150 MB" until a release
  carries an ISO.
- **The plain-text guard.** `./le site check` refuses a page that shows a
  snapshot value outside a marker, `data-ze-stat="<key>"` or
  `data-ze-rfc="<stem>:<field>"`, and names the page, the figure and the key
  (`checkPlainFigures`, `internal/le/site/plaintext.go`). It matches a display
  string carrying a thousands separator or a "+", and a whole number of 100 or
  more beside its unit word (the last word of the key's name). It reads text
  in `<head>` and the title, alt, aria-label and meta content attributes, and
  skips code, pre, script and style. The RFC pages whose figures are still
  server-written are listed, each with a reason, in
  `website/data/plain-figure-ratchet.json` (source-only, never deployed): a
  listed page is not refused, a listed page that shows no plain figure fails
  the check as `stale-ratchet` so the entry is removed, and only a page under
  `quality/rfc-compliance/` may be listed (`loadPlainFigureRatchet`). Never add
  a page to it: mark the figure instead.
- **The facts snapshot.** `data/site-facts.json` is written by the build, not by
  hand, and it is written BEFORE any page producer runs: the prose tokens, the
  shared header, the homepage proof strip and category buttons, the CLI
  reference (its group counts included), the command-equivalents index (its
  per-vendor coverage cards and per-verb counts included), the testing-health
  page (every figure its cards, tables and trends print, under `health.<metric
  key>` and `health_trends.<series>`) and `llms.txt` all read it back, and none of them counts a population the snapshot counts (a producer that writes a
  value itself reads it through `factLookup`, `internal/le/site/factmark.go`), and `site.js` fills
  every `data-ze-stat` element from it at page load. Each number is derived
  from one input and `_sources` names that input. Two of them are not derived
  from this tree: the star count reaches api.github.com and keeps the previously
  published number when it cannot, saying so in `_sources`, and the command and
  configuration counts come from the binary this build compiled. The release
  section (`release.*`) is a network fact too, but committed: the build reads
  it only from `data/github-releases.json`, and it is `null` while that file
  states no release (`factsFromReleases`, `internal/le/site/releasefacts.go`).
  When every
  number equals the previous artifact's, the build keeps that file whole, its
  `generated_at` and `published_at` included, so a build over an unchanged tree
  rewrites no byte of it (`previousFactsAt` in `internal/le/site/facts.go`).
- **The browser fill.** `site.js` fetches `data/site-facts.json` once per page
  load (`loadSiteFacts` keeps the one promise) and `fillRepoStats` writes each
  `[data-ze-stat]` element's value for its dotted key as `textContent`, never
  markup, and sets the width of each `[data-ze-stat-width]` bar (the health
  meter) from its key's percentage, so a bar never disagrees with the figure
  beside it. It fills the page body at load and the shared header after
  `loadSharedHeader` inserts it, because the header arrives after the body was
  filled. When the snapshot cannot be read, every element keeps the value the
  last build wrote into it. `TestTheBrowserFillsEveryMarkedValueFromTheSnapshot`
  (`internal/le/site/browsertest/fill_test.go`) proves it in a real browser: it
  serves a fixture site carrying the real script, loads a root page and a
  nested one in headless Chrome through chromedp, swaps the snapshot and
  reloads, and checks the header and body figures, a bar's width and the one
  fetch per load.
  It needs Google Chrome or Chromium on the host and fails, never skips, when
  neither is found (macOS `brew install --cask google-chrome`, Debian or Ubuntu
  `apt install chromium`). It sits in its own package so the `internal/le/site`
  tests still run on a host with no browser.
- **The release roadmap.** The `roadmap` producer collects committed `HEAD`
  once through `internal/le/spec/roadmap.Collect` on every full or partial build.
  Shared Markdown rendering publishes `/project/roadmap/` and its `index.md`;
  `data/release-roadmap.json` carries the same inventory with schema version,
  full revision, and its own input digest. The website source digest covers
  only website inputs. The build never reads the generated `plan/release/roadmap.md`
  or falls back to working-tree specs. Pending edits appear after commit and
  regeneration. Classification and priority describe planning intent, not proof
  of correctness, readiness or delivery.
  Feature cards describe shipped or experimental capabilities and link to the
  roadmap for remaining release work items.
- **The RFC requirement ledger.** `data/rfc-requirements.json` is derived once
  per build by `publishRFCLedger` (`internal/le/site/rfcledger.go`), from one
  reading of the checkout through `rfc.Collect` and `rfc.NewRenderInput`, and
  never through `rfc.Check`. The `rfc-compliance` producer reads it back and
  writes the aggregate page at `/quality/rfc-compliance/` plus one detail page
  per summary stem at `/quality/rfc-compliance/<stem>/`. Every cell of a
  requirement row is `rfc.RequirementRows`, which is also what
  `rfc/requirements/<stem>.md` is rendered from, so the site and the repository
  cannot state different things about one requirement. A `{gap}` row that a
  test demonstrates through `rfcgap.Demonstrate` carries that test's unit
  (`demonstrated-by`, from `rfc.RenderInput.Demonstrated`), the gaps table names
  it in place of "no test", and each stem counts `demonstrated-gaps` beside its
  declared gaps. The producer retires a
  page AFTER it has written this run's, and it deletes only a directory carrying
  a page this family wrote: on a real build the output is the published
  checkout, so a removal keyed on "a name that is not a live stem" would take
  any directory another producer or an author put under that prefix.
  A partial annotation carries typed `tested`, `gap` and `producer` fields
  beside its complete reason and unchanged Text. Its gated count is the
  `partial` subset of Annotated; `partial-rows` includes advisory rows too.
  The separate `Partial proof; remaining gap` bucket is not whole satisfaction
  or NoTest. Gap detail and record lists disclose both scopes without adding
  the row to ordinary Gaps, GatedGaps or DemonstratedGaps. Verified records are
  tag-claim evidence only. Render-only builds suppress whole credit even for
  an invalid stored enforced verdict on a partial row.
  A malformed checklist prevents publication through the shared render-input
  constructor, even when this path does not run the gate. A whitespace-padded
  malformed partial marker cannot become an ordinary whole-proof row. The
  partial tape segment uses the unmet-obligation color, never the proven color.
- **The RFC family renders once for two outputs.** `rfcdetail.go` declares each
  section of a detail page as a heading and a PAIR of functions, one for the
  markup and one for the Markdown mirror, so a section cannot reach the page and
  miss the mirror. `rfcmarkup.go` holds the primitives both call: the requirement
  anchor, the linked mention, the scrolling table container and the escaped cell.
  `rfcevidence.go` holds the evidence half of the page -- the gaps, the proof
  state of every tagged unit, the extraction sign-off and the superseded rows.
  Both halves publish a requirement's sentence through
  `rfcLedgerRequirement.Subject`, and the Coverage and Remaining cells through
  `rfcAuthoringNotesRemoved` (`rfcprose.go`), so an HTML comment a summary author
  left for the next author never reaches a reader; `data/rfc-requirements.json`
  keeps every row as authored. `rfcProseHTML` renders the author's closed
  `**emphasis**` as emphasis, and leaves emphasis that is unclosed, or that would
  cut a code span, as the asterisks the author typed.
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
  `go test ./internal/le/doc/yangcontract -run CommandSurface` exercises native command
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
| `category` | Optional | Subject category: `operate`, `routing`, `automate`, `observe`, `secure`, `services`, or `platform`. The shared card palette also has `meta` for cross-topic content. |
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

### Project overview

The `project` producer publishes `/project/` from `project/index.md`, filling
its two page-local slots before the shared Markdown renderer writes the HTML
and Markdown mirror. The latest update uses the same canonical report loader,
coverage label, authored summary and clipping rule as the homepage. The three
recent landmarks use `data/milestones.json` and the timeline's existing date and
category validation and stable newest-first order. Short histories show every
available landmark; an empty one is stated plainly. Missing or unknown template
slots stop publication rather than leaving an unfinished overview.
<!-- source: internal/le/site/project.go -- renderProject, projectOverviewContent, projectLatestUpdate, projectLandmarkList -->

The overview preserves the existing roadmap, milestones, updates, activity,
blog, talks, history, Why Ze, AI development and contribution destinations. Its
demonstration section links to the existing homepage player and continuous
recording with their transcript; the preview is an authentic static capture,
not a live hosted router. Release work remains a planning inventory and is
never presented as a completion percentage. Layout styles live in
`assets/css/35-project.css` and use the shared theme tokens.
<!-- source: website/project/index.md -- project overview sections -->
<!-- source: website/assets/css/35-project.css -- project overview layout -->

### Markdown mirrors

Every published page gets an `index.md` sibling beside `index.html`. A reader
can request the same URL with `.md` and receive the source form used by
`llms.txt`.

The search index is a plain-text consumer of those mirrors. It renders each
page's title and body with the shared Markdown engine, then extracts visible
HTML text once, preserving inline punctuation, image labels and separation
between blocks and table cells. Literal entity spellings inside code remain
literal; prose entities become the characters readers see. Script and style
contents do not enter the search index. The configuration index keeps its
existing structured-data path. `llms-full.txt` continues to copy Markdown
bodies, while `llms.txt` reads structured metadata and links to the mirrors.
<!-- source: internal/le/site/search.go -- pageSearchRecords, searchMirrorText, configSearchRecords -->
<!-- source: internal/le/site/llmsfull.go -- writeLLMSFullSection -->
<!-- source: internal/le/site/derived.go -- renderLLMS -->

When HTML is converted to Markdown, literal tag characters and entity-looking
text remain prose after the mirror is rendered again. Inline code,
preformatted blocks and code-producing chips, tags and card labels keep their
literal text; they do not decode entity-looking content a second time.
<!-- source: internal/le/site/mirror.go -- mirrorConverter.data, mirrorConverter.renderNode, renderComponent, mirrorCodeComponent -->

Pages backed by Markdown publish that source with internal links rewritten to
their public siblings. Pages built from JSON produce HTML and Markdown from the
same typed input. Hand-authored HTML pages derive their mirror from the
published main content, and the homepage is one of them: its copy lives in the
build's own template, so a hand-written mirror would state that copy twice.
The homepage demonstration replays the recording `terminaldemo.HeroDemo` names,
and its browser showcase link opens the one `homeTourDemo` names
(`internal/le/site/home.go`). The filename in the recording's frame, its
caption, its player label and both links come from those ids' entries in
`demos/terminal/manifest.json`: the caption is the entry's `title`, and each
link is the entry's `page` at its `anchor`. The template names no recording.
When selecting another replay, also select a useful preview time in that
recording for `homeTemplate`'s `data-cast-poster`.
<!-- source: internal/le/site/home.go -- homeHeroSlots -->
<!-- source: internal/le/site/homebody.go -- homeTemplate -->


The hero's project-size band replaces its repeated navigation tiles. It shows
lines of Go source code, lines of Go test code, lines of third-party Go code,
plugins and CLI commands through marked `data/site-facts.json` values. The line
figures include comments and blank lines. Source and test lines exclude
dependencies; the separate third-party figure counts tracked vendored Go files.
Source includes development tooling.
They come from the recorded repository facts, which count the tracked Go files
of the checkout. The activity page states its sizes from the committed size
history instead, so the two can describe different commits. Each line figure
links to activity, while plugins and commands link
to their reference pages. Why Ze exists remains an introductory text link, and
Discord is linked in a sentence. Search and demonstrations retain their existing
header and player paths. The release notice and testing proof section remain.
<!-- source: internal/le/site/homebody.go -- homeTemplate -->
<!-- source: internal/le/site/facts.go -- factsFromRepositoryFile -->

Thomas requested this correction on 11 November 2026:

> after 1.84 M can you add the 3rd party library count too as a new extra counter.
>
> Run BGP, OSPF and IS-IS on Linux. Manage interfaces and services through one configuration over SSH, web or APIs.
> You removed all the links to the page we have in the current live site at ze-project.net
> You also removed information when we have space for it, please rectify
>
> Current first-party Go inventory. Includes comments and blank lines; excludes third-party dependencies. Development activity →
> The comment is not useful with the extra number and the details are on the page so remove

The opening paragraph combines routing, interfaces, tunnels, forwarding and
services with their documentation links. Detailed configuration information
sits beside the demonstration on desktop and below the player on smaller screens. The metric
band has five figures and no inventory note; their definitions remain on the
activity page and in this build reference.
<!-- source: internal/le/site/homebody.go -- homeTemplate -->
<!-- source: website/assets/css/30-components.css -- .site-home .hero-start-intro.hero-grid -->
<!-- source: website/assets/css/30-components.css -- .site-home .hero-capabilities -->
<!-- source: website/assets/css/60-responsive.css -- .site-home .hero-start-intro.hero-grid -->

Thomas requested this layout refinement on 2026-11-12:

> can you bring the content of:
> Route and connect
> Speak BGP, ISIS and OSPF. Manage interfaces and tunnels with IPsec VPN and WireGuard. Set the forwarding table (FIB).
>
> Into the previous:
> Run BGP, OSPF and IS-IS on Linux. Manage interfaces and services through one configuration over SSH, web or APIs.
>
> The cards with the Million the number line must not wrap, they do not have to have the exact same size
>
> The zeledon card you can change it to have "Planned release" as title and then "Initial version" "Q4 2026" as two lines
> with the bird before the two lines

Each metric keeps its number and written unit on one line. The metric cards
take their width from their content and wrap as whole cards when space runs out.
The release group comes first, with no leading divider. Its larger bird fills
the height of a counter card and sits beside the title and two release lines,
with no divider between bird and text. The desktop metric group fits its content,
with the same gap after the release group as between metrics.
The introduction presents IPsec VPN or WireGuard as tunnel options and FIB or
VPP as forwarding options. The management clause describes interfaces and
services.
Links in the hero are never underlined, in any state. One rule covers every
link the hero holds, so a link added later is not underlined either. A text
link, one inside a paragraph or a caption, is a brighter blue than the muted
text around it, so a reader can tell it apart by color alone, and turns white
on hover; keyboard focus remains outlined. This is an owner ruling
("Owner rulings" below).
<!-- source: website/assets/css/30-components.css -- .site-home .hero a -->
<!-- source: internal/le/site/owner_rulings_test.go -- TestHeroLinksAreNeverUnderlined -->
The release text has a Planned release title above two lines: Initial version
and Q4 2026. The separator between release and counters is vertical on desktop
and horizontal when the counters stack below release at tablet widths and below.
<!-- source: internal/le/site/homebody.go -- homeTemplate -->
<!-- source: website/assets/css/30-components.css -- .site-home .hero-metrics-grid -->
<!-- source: website/assets/css/30-components.css -- .site-home .hero-project-metric strong -->
<!-- source: website/assets/css/30-components.css -- .site-home .hero-release-details -->
<!-- source: website/assets/css/60-responsive.css -- .site-home .hero-release-callout -->

The line labels read as a phrase after the quantity. Thomas requested this
wording improvement on 2026-11-11:

> "Go source lines" does not read well or naturally after the number same for the other string, "lines of go application code" may be better ???

Use "source code" because this count includes the development tools as well as
the application.
<!-- source: internal/le/site/homebody.go -- homeTemplate -->

The release date appears on its own line, with larger release text. Thomas
requested these changes on 2026-11-11:

> can you add a new line after expected release: so the date is on its own line
>
> and increase the font size
<!-- source: internal/le/site/homebody.go -- homeTemplate -->
<!-- source: website/assets/css/30-components.css -- .site-home .hero-release-callout p -->

Keep the RFC card's bullets compact while retaining the reviewed population,
exclusions and historical counting note. Thomas requested this on 2026-11-11:

> Under the first section with 45.3% has very long descriptions in the three bullet points pushing the page down. Can we be more compact in our descriptions ?
<!-- source: internal/le/site/homebody.go -- homeTemplate -->

### Site design and content rules

Before adding or restyling a page or component, read the
[design principles and review requirements](style-guide/index.html#sg-design-principles).
That section is the canonical record of the owner's approved website design
rules. Apply it to proposals and implementation.

#### Owner rulings

An owner ruling is a change Thomas told an agent to make to how the site looks
or reads. It stays until he changes it. Read this section before your first
website edit.

1. Apply a ruling at the scope he stated (the hero, every card, the whole
   site) in one rule that covers that scope. Never apply it one element at a
   time: an element added or renamed later then falls outside it, and the old
   look comes back.
2. Record the ruling in the table below and pin it with a test that goes red
   when it is undone, in the same work. A ruling with no test is not finished.
3. Never change what a ruling fixes while doing other work: a restyle, a new
   section, a stylesheet tidy-up, a regeneration. When the work in hand seems
   to need it, stop and ask Thomas, quoting the ruling.
4. When a pinned test goes red after your edit, repair your edit. Only Thomas
   changes a ruling, and then the style, the row and the test change together.
5. When Thomas has to repeat a ruling, the earlier fix was too narrow or was
   never pinned. Find the route by which it was undone and close it before
   you report the work.

| Ruling | Scope | Rule that applies it | Test that pins it |
|--------|-------|----------------------|-------------------|
| Links are never underlined, in any state. A text link stands out from the text around it by color and turns white on hover. Keyboard focus keeps its outline | Homepage hero | `.site-home .hero a` and `.site-home .hero :where(p, figcaption) a` in `website/assets/css/30-components.css` | `TestHeroLinksAreNeverUnderlined` |
<!-- source: internal/le/site/owner_rulings_test.go -- TestHeroLinksAreNeverUnderlined -->

The approved design uses logo blue for Ze's identity and a distinct soft
background behind each card's small category badge. Follow the canonical
[card color rules and live palette](style-guide/index.html#sg-category), including the neutral case and
independent status colors. "Decision" is the colored badge; "Reasons to consider
Ze" is a plain heading. Badge text, headings and card bodies use neutral colors.
Do not put a colored background behind the heading. Category selection controls
use the matching tint and neutral text across both themes. The homepage's
deployment cards (netlab/containerlab, native Linux, Docker, and VM/gokrazy)
share the existing daemon blue as an approved section exception; their category
metadata stays unchanged. Card borders, body surfaces and body text stay neutral.
Explicit compact software labels use `concept-software` (blue); evidence,
benchmark and test-layer labels use `concept-evidence` (teal). Category and
concept badges use `--badge-accent-share`: 24% accent in light mode and 18%
in dark mode, mixed into `--panel`, with neutral `--text` lettering.
Apply these classes to the small chip, badge or inline name itself, never to a
heading or card. Software examples include FRR, BIRD, GoBGP, Docker and QEMU;
protocols, deployment modes and commands remain neutral. Evidence color identifies
the type of information, not a successful result. Do not recolor prose by word
matching or assign hues by position, hash or presentation tone.

Generated compact chips and sidebar labels use the complete-name vocabulary in
`internal/le/site/concepts.go` (`compactConcepts` and `compactLabelClass`),
matched without case. This assigns presentation roles only, not support or
evidence claims. Typed dependency and vendor labels emit their roles directly;
authored HTML uses explicit concept classes.

Three section surfaces give the page color without recoloring individual cards.
Blue identifies management and run choices through `--brand-section-gradient`.
Teal identifies testing through `--evidence-section-gradient` on the homepage
proof section and quality overview. Lavender identifies references and reading
through `--reference-section-gradient` on the homepage documentation section,
documentation route grid and article takeaways. Light pages use a neutral
foundation and white cards; dark pages retain their navy foundations and cards.
Supporting notes and navigation selection use `--brand-surface`. Key reading
and action links use accessible `--blue`. Choose surfaces by their reader
purpose, never alternating arbitrary sections. Preserve the hero gradient,
neutral card borders, plain headings and terminal imagery. These colors retain
the approved layout. Poppins headings
and Lato body text share the same hierarchy across the homepage, guides,
catalogs and articles. The root element's `data-theme` attribute and `ze-theme`
storage key remain the theme contract; the prepaint bootstrap and the shared
script must agree about both. A new design treatment needs owner approval,
followed by the matching style-guide update.

Card producers use `cardCategory` in `internal/le/site/docpage.go` for optional
categories. It shares `pageCategories` and `validateCategory` with page metadata,
returns `meta` when the category is absent, and rejects unknown values. Emit the
result as `cat-<category>` and keep functional filter metadata separate. Take
subject assignments from the existing feature and topic records, not a new
page-specific color map. The design guide owns the palette and assignment rules.
Use `categoryLabel` for category display names: `platform` reads as
"Setup" while its identifier, anchors, classes and color stay
`platform`. Escape the label when rendering HTML.
<!-- source: internal/le/site/docpage.go -- categoryLabel -->

The homepage preserves its useful content and data slots. Its opening keeps
the network OS and immutable-services identity, followed by a short explanation
of Linux routing and shared configuration through SSH, web and APIs.
"Try the BGP quickstart" is the first, primary action and links directly to
`guides/quickstart/`; the demo gallery is secondary. The copy, actions and
pre-release notice sit beside the genuine terminal recording on desktop.
Keep these actions and the recording controls within the initial laptop
viewport. On phones, compact the copy and place the recording before the
interface selector so its preview and Play action are visible before scrolling.
The project metrics, release callout and product map follow the opening row.
Two groups below that map retain every routing, interface, tunnel and
management destination, followed by the engine, plugin and deployment
explanation. No capability content is collapsed or hidden.
The outer hero alone paints the shared navy gradient. Its inner panel and
showcase chrome remain transparent in both themes, without an inner shadow
that creates seams at the left and right edges.
<!-- source: internal/le/site/homebody.go -- homeTemplate -->

Regression prevention follows Latest news. Its cards describe the behavior checked, with short action
sentences and links to the test methods. Counts represent Go test functions,
functional scenarios, fuzz targets and interop images, not a passing-run total.
The RFC card separately shows the tested share of tracked MUST requirements.
The management panel's left column starts with Why Ze,
then explains how to manage Ze directly through its own SSH service. It has no
self-contained management badge. The architectural argument cards remain below
the panel. Product screenshots
are individually linked to their full-size files and captioned with their
capture provenance; every panel remains available without JavaScript. The
selector reserves one shared media frame and the tallest caption, so switching
interfaces never moves the hero. Inactive panels are invisible and inert, while
all panels remain visible without JavaScript. The Web configuration tab shows
the verified WebP screenshot of the committed hostname, with its
manifest-derived full-size and transcript links. The CLI editor and CLI
dashboard remain genuine static captures with no player controls burned in.
The full manifest-derived terminal replay precedes these previews in the opening
hero, with its transcript link and `recording-title` anchor. `HeroDemo` in
`internal/le/site/terminaldemo/types.go` selects the recording for both
the homepage and the README screenshot link. The selector never replaces
that replay or transcript. Screenshots use WebP; terminal playback uses the
captured `.cast` events. Encoded video and animated README carriers are not used.
The opening replay shows a genuine BGP dashboard frame at 43 seconds before
Play, using `data-cast-poster` and the player's recorded-time poster.
The initial preview enlarges the peer table within the existing media frame.
The caption identifies this crop and points readers to full-screen playback.
Playing, seeking or stepping through frames removes the enlargement.
Full screen also shows the whole terminal. The preview does not set the
playback start and never autoplays.
The management chapter's `data-cast-start` still sets both its preview and
playback start, taking precedence over a surrounding preview.
Recheck the preview time when the selected recording changes. Keep the
recording's original provenance; selecting a frame is not a new recording.
<!-- source: website/assets/js/site.js -- initTerminalDemoPlayers -->
<!-- source: internal/le/site/homebody.go -- homeTemplate -->
The replay heading uses solid hero text rather than the page-title gradient,
which is too dark against the blue hero in light mode.
The category overview derives every
feature link from `features.json`, alongside the existing category counts.
The documentation section explains how YANG and command definitions keep the
reference aligned with the documented build. Its introduction sits to the right
of its heading on desktop. Direct configuration, command and plugin reference
cards precede the feature directory. Both groups share the run cards' neutral
surfaces and spacing, with a category-colored background behind the compact
name only. Feature links appear one per line, without removing any destinations.
Its management explanation states that Ze runs its own SSH service and CLI,
without relying on a host SSH service or shell.
The management section keeps that explanation, its compact interface map and
both documentation links on the left. The right column reuses the verified
recording selected by `homeManagementDemo`, with its full-session and transcript
links. `homeManagementStart` opens playback at the SSH introduction; check that
position against the cast when changing or re-recording the demonstration.
The shared player reads the nearest `data-cast-start` value as its native
`startAt` option and uses that recorded frame as its poster. It retains all
controls and never autoplays. The columns stack at 960px.
<!-- source: internal/le/site/homebody.go -- homeTemplate -->
<!-- source: internal/le/site/home.go -- homeBody -->
<!-- source: website/assets/js/site.js -- initTerminalDemoPlayers -->

Update reports use the inclusive calendar dates in `covers:` as their
reporting period. Pages, index cards, homepage teasers, feeds and Markdown
mirrors show both dates and the duration: seven days is `1 week`, fourteen
is `2 weeks`, and other spans use days. `data/changes.json` carries
`coverage_start`, `coverage_end`, `duration_days` and `duration` beside the
unchanged start-date `slug`. A missing `covers:` uses the filename; a lone
date covers one day. Ranges require two valid dates in order. The historical
`YYYY-MM-DD HH:MM` form is validated and counted by calendar date.
<!-- source: internal/le/site/changes.go -- changeCoverage, changeDuration, readChangeWeek -->

The homepage's update report cards use the post's optional `home-summary:` front
matter. Write one short summary of at most 108 characters. Without it,
the card uses the introduction's first paragraph. The renderer collapses
whitespace and clips long summaries at a word boundary, with an ellipsis.
The full update page, feed and changelog data keep the complete introduction.
<!-- source: internal/le/site/home.go -- blogTeaserCards, clipSummary -->
<!-- source: internal/le/site/changes.go -- readChangeWeek, changeWeek -->

Guides retain their source heading IDs, prose, examples and demonstrations.
`insertDocTOC` groups the complete body beside its outline after the generated
title block. It verifies that the body starts with that block before splitting;
untitled authored HTML stays whole. The generated title block stays flat: its
first closing `div` marks where the reading layout begins. Articles group the byline with the title, retain their
themed illustration, and place the section outline and related navigation
beside the complete article and key points. Narrow screens return these parts
to document order. Both outlines use native disclosures, open without scripts;
the shared script initially collapses them on narrow screens and then respects
the reader's choice. Neither layout removes content for readers without scripts.
<!-- source: internal/le/site/markdown.go -- insertDocTOC -->
<!-- source: internal/le/site/doctransform.go -- pageHero -->
<!-- source: internal/le/site/blog.go -- blogArticleBody -->

Navigation is part of the design system. `data/nav.json` owns the top
mega-menu and curated `llms.txt` sections. The build publishes the menu as
`assets/header.html`. The footer carries the copyright line, the license and
legal notice links, and the page publication stamp. Asset
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
own source of truth (`spec-website-wiki-content-migration`,
2026-07-22). It is read from the committed `data/wiki.json`, so the build never
opens a wiki checkout and a machine without the sibling directory writes the
same artifact. `./le site wiki update` refreshes that file from a checkout and
`./le site wiki check` reports it stale. Both read the checkout's committed
`HEAD` through git, never its working directory, so an untracked or uncommitted
wiki file cannot change the committed index. The order and the grouping are the
wiki's own `_Sidebar.md`, and a wiki page the sidebar does not list is refused
by name unless `accountedUnlisted` in `internal/le/site/wiki/index.go` already
states why it is out.

Changes to feature cards, audience data, navigation or command equivalents are
made in `data/*.json`, followed by `./le site build`. Command paths still come
from the live CLI catalogue, while vendor mappings remain curated in
`data/command-equivalents.json`.

### Plugin catalog

`reference/plugins/` is generated from plugin registrations. Plugin cards,
dependencies and detail pages remain generated outputs. Changes to registry
facts are published with `./le site build`.

The catalog renderer creates `reference/plugins/index.html`, its `index.md`
mirror, and one local `reference/plugins/<plugin>/` detail page
per registry entry. Card clicks must stay on the site. If the page needs a new
machine fact, extend the extractor or registry data instead of adding a
hardcoded list to the renderer.

## Presentation tooling

Published decks live at `talks/<slug>/`. `./le site talk talk <slug>`
refreshes a deck's statistics and activity page, then writes
`<deck>-inlined.html`. Use `./le site talk talk <slug> bundle-only` when
only the standalone bundle should change. The lower-level
`./le site bundle input <deck.html>` and `./le site activity output <file> days
<count>` actions remain available for one-off artifacts.

The activity page is one standalone document that carries its own stylesheet
inside it, because a deck embeds it as an iframe where no link to the site's
assets resolves. It shares the commit calendar's drawing with
`/project/activity/`: the same grid, the same cards and the same Added
Lines/Commits switch, with the working tree's Go composition beside it. Its
measurement is its own (`MeasureActivity`): code files by extension, vendor
directories left out by glob, git's rename pairing, and the full year back from
its pinned day. Its cells and tooltip carry no deleted lines, so a deck renders
the bytes it always rendered. Only the published page has the
repository size view, which it reads from the committed size history. The two
present the calendar differently: the published page follows the site's theme, is full size, and opens on a
hero, while the deck embed is dark and sized in viewport units so the whole
widget fits one slide. The two stylesheets are separate files, and neither
rendering reads the other's rules. `today <date>` pins the window, so a
published deck keeps the year it was presented with.

The published calendar uses a sequential teal-green scale in each theme.
Zero activity uses `--line`; borders use `--line-strong`. Its panels and text
use the shared theme surfaces so low activity remains visible in dark mode.
<!-- source: internal/le/site/activitystyle.go -- activityStyle -->

The published page has two views, behind the Repository size and Commit
calendar tabs above them. It opens on the size view. Without JavaScript the tabs
stay hidden and both views show, the size view first.

The size view reads `data/repo-history.json`, the committed size history. Its
headline is the newest recorded day's total lines and that day's date, followed
by one figure for each section the file lists and a sentence on what the
figures count. One chart draws each
section's lines over every recorded day, in a color taken from the section's
place in the file and named in a legend whose buttons add or remove the section.
The same chart draws a dashed total line, each day's sum of the sections the
legend shows, and its scale covers that sum. The legend names the line "Sum of
the sections shown" without making it a button. The readout's "All sections"
row stays the sum of every section, so the line and the row agree whenever
every section is shown.
A second chart draws the lines the selected sections gained, above the axis, and
lost, below it, on each day. Both scales come from every recorded day, so an
earlier peak stays inside the chart. Both charts plot their days over the same
span, and the slider under them is exactly as long as that time line: its track
starts at the first recorded day and ends at the last, so its handle always
stands on the charts' cursor line. A day is chosen with the pointer, the arrow,
Home and End keys, or the slider, and the readout lists each section's lines,
gained and lost lines for that day. The readout's table keeps one size: every
section of the file has a row whichever sections the legend draws, and its
columns are fixed, so neither the day nor the selection moves it. It carries no
Columns control. When the file cannot be read, the readout says so and the
figures of the last build stand.

The calendar view opens with its own "Commit calendar" heading, so a reader
without JavaScript, who sees both views one after the other, finds the second
titled. It keeps the four summary cards and the Added Lines/Commits
switch, which show only with it, and opens on Commits.

The page's calendar measures a different population from the deck's
(`MeasureCommittedActivity`). It counts every committed file outside the
third-party trees `ThirdPartyPath` names (`vendor/`, `third_party/`,
`website/assets/vendor/`): code, tests, documentation, plans, the RFC corpus
and the rest, which are the size view's sections other than its two third-party
ones. It reads both numstat columns of one `git log` pass, so added and deleted
lines follow the same rules, and counts the third-party trees' lines in the same
pass apart from them, for the cumulative curve's all-files line. Lines include comments and blank lines. A change
in which either version of a file is binary counts no lines. A rename counts as
the old path's lines deleted and the new path's added (`--no-renames`). A merge
commit counts as a commit, and the lines it changes itself are not counted.
Each commit falls on the UTC day of its author time. The window opens on the
first day of a month: the first month start on or after the day a year back,
or the first day of the first commit's month when that is later. "Days shown"
counts the days from there to the build day.

The first card counts the lines added in the window. The note under the cards
states that population, then the lines the window's commits added and deleted
and their net change. When the window reaches back to the first commit, the
note adds that the net change is the number of lines these files hold at the
newest commit the calendar reads. That is the size view's figure for the same
files on the same commit, apart from the lines of a file that turned between
text and binary and any lines a merge changed itself. Each square's color still follows the lines that day added, so a day that rewrote as
much as it removed does not read as idle. Each square's tooltip and accessible
name state the lines added and deleted. The date range sits to the left of the
calendar's Less/More legend. After the calendar comes the Go composition of the newest recorded day
in the size history: the lines of all Go, third-party included, summed over
every section the file splits into Go lines, then first-party Go, production,
tests and third-party Go, each with files, lines, code, blank and comment
lines, and the module count. It does not read the working tree. Under it, the
cumulative curve follows the switch. Under Added Lines it sums the net lines
the calendar cells carry, added minus deleted, so it falls on a day whose
commits deleted more lines than they added and can fall below zero. A second,
dashed line draws the same net over every committed file, the third-party trees
included, from the third-party added and deleted lines each cell also carries
(`data-third-party-added`, `data-third-party-deleted`, counted in the same
numstat pass). A legend shown only under Added Lines names the two lines,
"Without third-party" and "All files", and the scale covers both. Its readout
and slider then show the net beside the added and deleted lines it is made of
and the all-files figure. The cards, each square's color and its tooltip still
count first-party lines only. Over the whole history the all-files line ends
near the size view's total for the same commit, apart from the lines of a file
that turned between text and binary and commits newer than the size history's
last recorded day.
Under Commits it sums their commits and draws one line. Both count from the first day of the
window. It has no heading: its sentence, readout and slider say which of the
two it shows, and the selected date stays when the switch changes. Its slider
spans the curve's time line the way the size view's does.

The three charts share one vertical axis rule (`activityAxisTicks`). The axis
helps the eye judge size in units of ten, and the readouts carry the exact
figures. Its step is the power of ten below the range. The step is doubled when
that would draw more than eleven lines and halved when it would draw fewer than
three, but it never goes below one, so commit counts stay whole. The lines run
from the step at or below the smallest value to the step at or above the
largest, and zero is always one of them. When an axis has values on both sides
of zero, the zero line is drawn as the baseline. An axis of five lines or fewer,
with a step of ten or more, gets an unlabeled dashed line halfway between each
pair. Labels are compact (`activityAxisLabel`): every tick on an axis takes the
unit its largest tick sets, M from a million and K from ten thousand, and the
figure is written in full below that. No data value is printed on the axis.

Every size figure on the page, the Go composition's included, is a
`data-ze-history` marker that the build fills and the page script refills from
the published file. The deck embed retains its calendar.
<!-- source: internal/le/site/activitygrowth.go -- activitySizeHTML, activityHistoryGoPanelHTML, activityCumulativeCurveHTML, activityGrowthScript -->
<!-- source: internal/le/site/activitygrowthstyle.go -- activityGrowthStyle -->
<!-- source: internal/le/site/activity.go -- activityPageBody, activityCalendarViewHTML, activityCalendarNoteHTML, activityScriptHTML -->
<!-- source: internal/le/repo/rewrite/activitymeasure.go -- MeasureCommittedActivity, activityWindowStart, measureCells -->
<!-- source: internal/le/repo/rewrite/activity.go -- collectActivity, isActivitySourcePath -->
<!-- source: internal/le/repo/rewrite/activitynumstat.go -- numstatLines -->
<!-- source: internal/le/site/history/measure.go -- countBlob, dayRecord -->

## Adding a new presentation

1. Create a new directory under `talks/`.
2. Add `index.html` and, when used by the deck, `slides.md`.
3. Add frozen images under `screenshots/`.
4. Run `./le site talk talk <slug>`.

## Updating presentations

`./le site talk talk <slug>` replaces each former per-talk update
script. It refreshes live slide statistics and embedded activity when the deck
uses them, then regenerates the standalone bundle.

## Update Report Checklist

Trigger: a new `changes/posts/<start-date>.md` lands (the Discord
`ze-news` update -- this is the development changelog source; the `blog/` section
is now for occasional editorial articles, not the update report). Work through
this before considering the update done. Complete the Milestones review during
research, before presenting the post for approval. A draft-only request includes
proposed milestone entries and leaves site data and generation unchanged.

The starting material is `changes/fragments/`: one file per change, written by
the agent that closed a spec or fixed a defect operators see
(`changes/fragments/README.md`). A fragment is a claim, never evidence, so
every claim used in the post is verified at its producer first
(`ai/skills/ze-weekly-update.md`, Phase 1). The site does not publish
fragments: the changes pages are built from `changes/posts/` alone, and the
build trims `changes/fragments/` from the artifact it deploys.
<!-- source: internal/le/site/changes.go -- changesSourceDirectory -->
<!-- source: internal/le/site/paths.go -- sourceOnlyDirectories -->

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
   Update RFC counts are historical at publication. Keep
   `ze-stat-snapshot: true` in front matter, never in a body HTML comment.

   **Summarize for the homepage.** Add `home-summary:` using the
   [homepage card contract](#site-design-and-content-rules). Keep the full introduction
   and its qualifications in the body.

   **Pin release evidence before drafting.** Resolve `HEAD` once to a full
   commit ID and use its first-parent history for both reporting boundaries.
   Use the prior post's `release-to` as the next start when available. Otherwise
   resolve the commit at the start of `covers:` in UTC. Resolve the end at the
   end of the final covered day on that same pinned history. A missing boundary
   is an error. Obtain the missing history rather than substitute zero or HEAD.
   An owner-authorized in-progress update states its cutoff explicitly.
   Run `./le spec roadmap compare from <full-sha> to <full-sha>` and save both
   full commit IDs as `release-from:` and `release-to:` scalar front matter.
   Read both required subgroups and the nice-to-have group.

   **Keep progress claims bounded.** The report compares endpoints, so it
   cannot show items added and removed wholly between them. Explain additions,
   removals, bucket moves, and status changes separately. Verify implementation
   at its source before calling any removed item delivered. Use a short
   release-progress paragraph and the [public roadmap](https://ze-software.net/project/roadmap/)
   link, with the phrase `release work items`. Keep planned capabilities and
   workflow jargon out of the Discord body. Until owner
   classification is resolved, describe the report as an inventory preview.

   **Approve fixed prose.** Save the facts in this post's body before the
   existing weekly dry run and show its exact messages for approval. Website
   news and Discord consume that same body. Do not add live count tokens,
   recollect on send, change transport, or send without the existing approval.
   Preserve the RFC section, message budget, archive, and resume behavior.

1. **Check Features for drift.** Did the week ship something with no card
   yet? Add or edit its entry in `data/features.json`, naming the
   `features/*.md` declarations it describes. A card moves from Experimental
   to shipped only when every declaration it names is complete and
   supported, so a level change is made in the declaration, never in the
   card (`docs/contributing/feature-maturity.md`); `./le feature report`
   lists each declaration's level and what the next level lacks. The intro paragraph's
   count is computed from the data at render time, nothing to hand-update.
2. **Review Milestones independently.** Read `data/milestones.json`, the report
   linked by its newest entry, and every later report through this update.
   Review each report's full `covers:` range and pinned revisions, including
   ranges that cross months. Feature cards needing no changes does not skip
   this review.

   Identify each whole protocol or subsystem's first appearance and verify it
   at the producing source in the report's revision range. Keep one entry per
   capability class in the data. Routine fixes and improvements to existing
   capabilities do not earn another entry, and a month with no landmark needs
   no new milestone data.
   During the authorized site update, add each missing landmark to
   `data/milestones.json`. Use the source report's `covers:` start for `date`
   and `blog`, so the entry links to its report under `changes/`.

   Present one main card per month, newest months first. Use a compact
   Milestones header and a two-column desktop grid that becomes one column
   on mobile. Put related navigation below the timeline. Each card's short
   Key Points use the milestone titles. Put the month's report links at the
   bottom of that card. Derive those links from `changes/posts/`, including
   every report whose `covers:` start falls in the month. Show each report
   link once and keep this list generated.

   Report the added titles and linked report dates outside the public post.
   If none qualifies, state that result and its reason. After generation,
   inspect `../gh-pages/project/milestones/index.html` and `index.md` for the
   monthly grouping, newest-first order, Key Points, categories and complete
   report links. Also inspect the HTML's compact header, responsive grid and
   related navigation below the timeline. A build or site check without this
   review leaves website work pending.
3. **Check Compare for drift.** Did the week close one of the
   "Where Ze is behind today" gaps, or change a Yes/No/Partial cell? Edit
   `../docs/comparison.md` first (the source of truth), then copy the
   change into `compare/bgp.md` here (`compare/comparison.md` is a dispatch
   page between the BGP and NOS comparisons, not a mirror). Bump the
   `Last updated:` date in main's file when real content changes -- never
   let the mirror carry content main doesn't also have.
4. **Check Labs for drift.** Did the week add new interop/QEMU evidence
   substantial enough for its own lab page (see `labs/bgp-interop/` as the
   template)? A new lab also needs an entry in `data/nav.json`'s Labs
   dropdown.
5. **Check Performance for drift.** Did `../docs/performance.md` get a
   fresh benchmark run this week? If so, update the headline stat-row on
   `performance/index.html` (Convergence/Throughput/Withdrawal) to match.
6. **Run `./le site build`.** One command regenerates the website artifact,
   including native page surfaces, assets, presentation activity, and
   standalone talk bundles. It stages the result in `../gh-pages` and removes
   source-only inputs before returning.
7. **Check the artifact before calling it done.** Run `./le site check` to
   verify the deployment boundary and Markdown mirrors in `../gh-pages`.
8. **Never edit `talks/<slug>/` content** as part of this checklist. Those decks
   are historic snapshots frozen at the time they were given.
