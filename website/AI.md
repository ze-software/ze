# Website Sources

Published site: landing page + talks. `make ze-site-generate` builds the Pages artifact into `../gh-pages`.

## Layout

Website sources live in `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`make ze-site-generate` replaces that worktree with the publishable artifact and keeps `.git`.
It reuses existing VHS media unless the checked-in tape definitions changed.
Use `make ze-terminal-demo-release-render-all` to force new demo media.
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
  presentations/tools/                   -- shared presentation tooling
  tools/                                 -- site generators, checks, and tests
  talks/
    linx-2026-06/
      index.html                         -- authored slide renderer
      slides.md                          -- authored slide content
      screenshots/                       -- frozen editorial assets
    netmcr-2026-04/
      index.html                         -- authored presentation
      screenshots/                       -- frozen editorial assets
  update-website.sh
```

`make ze-site-generate` builds in a temporary artifact directory, publishes only after a successful build, removes source-only files from `../gh-pages`, and validates the artifact.

## Site build tooling

The site shell and every page outside `talks/<slug>/` are generated from data
and Markdown, not hand-edited HTML. Structure:

```
website/
  data/
    nav.json                              -- single source for the mega-menu; the nav build step
                                              renders assets/header.html, which every page loads
    features.json                         -- every card on features/index.html: section, category,
                                              status, chips, bullets
    milestones.json                       -- every node on milestones/index.html: date, title,
                                              category, blurb, and the blog week it links to
    topics.json                           -- controlled tag vocabulary for the Changes chips: every
                                              allowed tag -> one of eight categories (seven site
                                              colors + neutral `meta`). render-changes.py reads each
                                              weekly post's `tags:` front matter against this and the
                                              build fails on any tag not listed. Keep tags atomic and
                                              in canonical casing (BGP, IS-IS, Flow Export, ...)
    audience.json                         -- the "Two ways to run Ze" and "Who should look now"
                                              cards on index.html
    whats-new.json                        -- the freeform note in the "What's new in Ze" band on
                                              index.html (the other two slots are generated: newest
                                              blog article, newest weekly update). Keep the note
                                              short: the band sits above the proof strip and the KPI
                                              cards have to stay on screen on a laptop viewport
    dependencies.json                     -- every direct Go dependency's "why", grouped by
                                              category, keyed to ../go.mod
    command-equivalents.json              -- curated vendor equivalents keyed by Ze CLI paths;
                                              render-command-equivalents.py joins it to the generated
                                              CLI catalog and emits unmapped rows
    page-links.json                       -- right-rail page navigation, external project quick links,
                                              and reader-intent related links used by sitelib.py
  tools/
    render-css.py                         -- expands assets/css/site.css imports into
                                              the published assets/site.css bundle, minified
    render-js.py                          -- minifies assets/js/site.js into the published
                                              assets/site.js
    sitelib.py                            -- shared head/sidebar/footer chrome and the renderer for
                                              assets/header.html; also migrates embedded headers to
                                              stable fragment mounts and owns Markdown mirrors
                                              (see "Markdown mirrors" below)
    page_registry.py                      -- central registry for docs manifest, generated Markdown
                                              page sets, hand-authored nav patch targets, and
                                              destination-derived site roots
    build.py                              -- regenerates the entire site in one command
    check-page-links.py                   -- validates page-links.json external URLs, duplicate
                                              external pages, generated external link targets, and
                                              optional network reachability
    render-docs.py / render-doc.py        -- ../docs/*.md -> docs/**/index.html (also used
                                              directly for compare/comparison.md -> compare/index.html
                                              and contribute/contribute.md -> contribute/index.html)
    render-blog.py                        -- blog/posts/*.md (editorial articles) -> blog/**/index.html
                                              (empty until articles are added; the weekly changelog
                                              is the Changes section below, not the blog)
    render-changes.py                     -- changes/posts/*.md (weekly updates) -> changes/<week>/
                                              index.html full write-up + changes/index.html terse
                                              index + changes/feed.xml RSS. Topic chips come from
                                              each post's `tags:` front matter (comma-separated),
                                              colored via data/topics.json; a post with no `tags:`
                                              falls back to its section headers and warns
    render-activity.py                    -- git history -> activity/index.html
    render-features.py                    -- data/features.json -> features/index.html
    render-timeline.py                    -- data/milestones.json -> milestones/index.html, the
                                              landmark-features timeline, oldest first, grouped by
                                              quarter and color-coded by category
    render-command-equivalents.py         -- data/command-equivalents.json + live Ze CLI catalog ->
                                              reference/command-equivalents/index.html and index.md
    render-cli-catalog.py                 -- `ze help command --json` -> reference/cli/index.html,
                                              with a live search box that jumps to a matching command's
                                              anchor (id="cmd-<slug>") in its group
    render-dependencies.py                -- ../go.mod + data/dependencies.json -> reference/dependencies/index.html
    extract-plugin-registry.py            -- ../internal/**/register.go + YANG imports ->
                                              data/plugin-registry.json
    render-plugin-catalog.py             -- data/plugin-registry.json -> reference/plugins/
                                              catalog plus one local detail page per plugin;
                                              grouping is derived from ConfigRoots and source
                                              paths, not a hand-written plugin taxonomy
    render-config-reference.py            -- data/plugin-registry.json ->
                                              reference/configuration/index.html, every plugin
                                              (not just BGP) grouped by config root
    render-index.py                       -- data/audience.json + data/whats-new.json + template
                                              -> index.html
    render-llms-txt.py                    -- data/nav.json + page_registry.py + Markdown + live counts -> llms.txt
  update-website.sh                       -- builds the complete `../gh-pages` artifact through
                                              `tools/build-site.py`; forwards generator arguments, <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
                                              e.g. `./update-website.sh --only cli`
```

Pages with no dedicated generator (`zeledon/`, `labs/*/`, `talks/`,
`style-guide/`, `performance/`) are hand-authored HTML for their body content.
Like generated pages, they contain only a stable shared-header mount. The `nav`
step renders `assets/header.html` from `data/nav.json`, so menu changes update
one fragment instead of rewriting every page.

Run `make ze-site-generate` from the repository root. Do not run `tools/build.py` <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
from `website/`: that lower-level generator intentionally writes into its
own checkout and is executed only inside the staged artifact. Pass `--only`
with comma-separated step names from `tools/build.py` (for example <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
`config,plugins,search,seo`) to rebuild an existing artifact subset. A full
build recreates `../gh-pages` first. The `links`, `linkcheck`, search, SEO, and
`llms.txt` guardrails run after selected steps. Watch stderr for drift warnings
and per-step failures.

### Website architecture

- **Data sources.** Published pages come from structured data and Markdown:
  `data/nav.json` owns top navigation and the curated `llms.txt` page order,
  `tools/page_registry.py` owns the complete published docs and use-case page map, <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  `data/features.json`, `data/audience.json`, `data/whats-new.json`,
  `data/milestones.json`,
  `data/dependencies.json`, and `data/command-equivalents.json` own their
  matching generated pages, and `data/plugin-registry.json` is generated from
  `../internal/**/register.go` plus local `PLUGIN.md` metadata. Markdown
  sources live either in `website/` (`use-cases/`, `compare/`, `quality/`,
  `contribute/`, `faq/`, `roadmap/`, `license/`, `docs/docs.md`) or in <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  `../docs/` for product documentation and lab architecture detail.
  Top navigation dropdown entries in `data/nav.json` must use an emoji glyph
  for `icon`, not a text abbreviation or label.
- **Page registry.** `tools/page_registry.py` centralizes the small lists that <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  decide which Markdown files are rendered by generic document renderers:
  the main docs manifest, use-case pages, lab detail pages, compare pages,
  quality pages, and hand-authored pages whose nav, footer, asset versions,
  sidebar, and Markdown sibling are patched by the `nav` step. It maps source
  files to public information-architecture destinations such as `guides/`,
  `features/`, `reference/`, and `developers/`, rather than exposing repository
  directories. Builders call `page_registry.page_root_for_dest(dest)` so a
  moved page gets the correct `../` depth from its destination path.
- **Renderers.** `tools/build.py` is the orchestrator. It loads hyphenated <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  renderer scripts by file path, preserves the documented `--only` step names,
  and collects failures instead of stopping at the first broken page. Generic
  Markdown pages go through `tools/render-doc.py`; the main repo docs batch <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  goes through `tools/render-docs.py` using `page_registry.DOCS_MANIFEST` and a <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  temporary cross-doc link manifest. Specialized renderers own pages whose
  content is computed from data, live Ze output, git history, Go module data,
  or extracted plugin and YANG facts.
- **Generated assets.** Renderers write `index.html` plus an AI-readable
  `index.md` sibling for every generated page outside `talks/<slug>/`.
  `tools/render-search-index.py` writes search data, `tools/render-seo.py` <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  writes SEO artifacts, `tools/render-llms-txt.py` writes `llms.txt`, and <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  feed or catalog renderers write their derived XML, JSON, or detail pages.
  Do not edit generated HTML directly when a Markdown source, JSON data file,
  extractor, or renderer produced it.
- **Validation gates.** `tools/build.py` runs drift warning hooks before page <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  generation, records per-step failures, always patches generated external
  links, then always runs `tools/check-page-links.py --skip-network`. <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  That gate validates `data/page-links.json`, duplicate external page usage,
  and generated external anchor policy without touching the network. Network
  reachability stays opt-in with
  `tools/check-page-links.py --check-network` or <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  `tools/check-page-links.py --check-network --all-html`. <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
- **CSS source layers.** `assets/site.css` is generated (and minified) from
  `assets/css/site.css` by `tools/render-css.py`. The source manifest imports <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  `10-base.css`, the legacy bulk stylesheet, followed by smaller extracted
  files for tokens, shared components, and responsive fixes. Keep new CSS in
  the smallest source file that matches the concern, then run the `css` build
  step. Edit only the source files under `assets/css/`, never the minified
  `assets/site.css`.
- **JS behavior model.** The editable script is `assets/js/site.js`; the `js`
  build step (`tools/render-js.py`) minifies it into the published <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  `assets/site.js`. It loads `assets/header.html` into each page's stable
  mount, then provides navigation interactions and progressive enhancement for
  reveal effects, search-like controls, and generated page explorers.
  `data/nav.json` remains the navigation source of truth. Page body content and
  evidence must remain meaningful without JavaScript. Edit only
  `assets/js/site.js`, never the minified `assets/site.js`.
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
- **Prose number tokens.** Website-owned page sources (e.g. `compare/*.md`) may
  embed `{{ze:<name>}}` tokens (`unit-tests`, `e2e-tests`, `fuzz-targets`,
  `interop-targets`, `interop-scenarios`, `cli-commands`, `config-sections`,
  `dependencies`, `features`, `changes`) that `tools/render-doc.py` resolves <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  from `data/site-facts.json` at render time, so counts can never silently
  drift from the live facts. Use tokens only where the source path differs from
  its published path (compare pages qualify; `use-cases/*/index.md`, whose
  source is its own publish target, and imported `../docs/*.md`, which
  render raw on the code host, must use literal numbers).
- **Verification commands.** Use targeted commands, not a project-wide build,
  while editing architecture scripts:
  `python3 -m py_compile tools/page_registry.py tools/build.py tools/render-docs.py tools/check-page-links.py`,
  `python3 tools/check-page-links.py --skip-network`, and, when you need to
  prove the build wiring rather than regenerate every page,
  `tools/build.py --only links`. Use `tools/check-page-links.py --check-network` <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  only when external reachability is the thing being verified.

### Markdown to HTML contract

Ordinary Markdown pages use `tools/render-doc.py`. Do not add a renderer for a <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
page whose body is already Markdown. A dedicated renderer is justified only
when the HTML is computed from structured data, live command output, source
extraction, or another input that needs its own transformation.

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
| `category` | Optional | Heading colour: `operate`, `routing`, `automate`, `observe`, `secure`, `services`, or `platform`. |
| `journey` | Optional | Short label shown in the page hero. The renderer derives it from the destination when omitted. |
| `table-columns` | Optional, defaults to `true` | Enables shared show/hide controls for tables on the page. |

Front matter is deliberately limited to top-level `key: value` scalars. This
keeps the website build independent from a YAML package. Explicit arguments
from an existing batch builder take precedence, so old manifests can move to
front matter one page at a time.

With a `destination`, one command is enough:

```sh
tools/render-doc.py path/to/page.md
```

The renderer strips front matter, converts Markdown with `tables`,
`fenced_code`, `sane_lists`, and `toc`, applies the shared evidence-cell and
link passes, builds the hero and table of contents, then wraps the result with
`sitelib.page_head()` and `sitelib.page_foot()`. It writes the HTML destination
and its `index.md` mirror. The browser loads `assets/site.js` from the shared
page shell, so per-page JavaScript is not part of this pipeline.

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

Every published page -- generated or hand-authored -- gets an `index.md`
sibling next to its `index.html`, reachable at the same URL with `.md` for
`/`. This is what `llms.txt` (below) links to: an LLM fetching the site
gets plain Markdown, not HTML it has to parse. Three tiers, by how the
page is already built:

- **Real Markdown source already exists** (`docs/*`, `compare/`,
  `contribute/`): `render-doc.py` publishes that source itself, with
  internal `[text](other.md)` links rewritten to the sibling `.md` path
  when the target is also published (GitHub blob link otherwise -- same
  rule its HTML-link rewriter already used, just emitting Markdown link
  syntax).
- **Built from JSON/data** (`features/`, `reference/cli/`, `dependencies/`,
  `reference/configuration/`, `activity/`, blog posts): each `render-*.py` has a
  `render_markdown()` next to its `render()`, both reading the same data,
  so the two can't disagree.
- **Hand-authored HTML, no source of either kind** (`labs/*/`, `talks/`,
  `style-guide/`, `performance/`, `zeledon/`): `tools/build.py`'s `nav` <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
  step extracts the `<main id="top">...</main>` content
  (`sitelib.extract_main`) and converts it with `sitelib.html_to_markdown`
  -- a small HTML->Markdown converter written against exactly the tags and
  component classes this site emits (status-row facts, stat tiles,
  terminal panels, chips/tags, tables), not a general-purpose library.
  Re-derived from the HTML on every build, so it can't drift from the page
  the way a hand-maintained companion file would.

`tools/build.py` runs the page renderers before `llms`, so every registered <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
documentation page already has its `index.md` on disk. `llms.txt` combines
the curated `data/nav.json` order with the complete docs and use-case lists from
`tools/page_registry.py`. <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->


### Site design and content rules

Before adding or restyling a page or component, read `style-guide/index.html`
or the generated `style-guide/index.md`. Reuse the vivid candy claymation
language already in `assets/site.css`: seven category hues, clay card depth,
2px white "sugar coat" borders, masked grids, soft candy washes, and thin
concentric ring ornaments. Do not add one-off shadows, filled decorative balls,
opaque blobs, unrelated palettes, or custom components without updating the
style guide in the same change.

Navigation is part of the design system. `data/nav.json` owns the top
mega-menu and curated `llms.txt` sections; `tools/page_registry.py` supplies <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
the complete published docs index. The `nav` step publishes the menu once as
`assets/header.html`. The footer is the
license line and the publication stamp from `sitelib.footer_html`, not a
sitemap or second call-to-action block. Group the top menu by reader job: Start, Evaluate, Docs,
Examples, Reference, Project. Every multi-column dropdown must show a label at
the top of each column. Use short labels and one-line descriptions. Do not let
dropdown panels clip outside the viewport.

Use the right page menu for easy local navigation. Add related choices, nearby
evidence, and next steps to `data/page-links.json`; `sitelib.patch_page_sidebar`
injects `.page-sidebar` and the responsive layout. Do not hand-code duplicate
right-menu link lists inside page bodies.

Every factual claim must have evidence. A claim about a feature, protocol, lab,
benchmark, command, dependency, plugin, quality gate, or comparison must trace
to a source file, Markdown source, JSON data file, script, generated binary
output, or external reference. If a claim cannot be traced, do not publish it.

Prefer generated data over hand-maintained prose or tables. Lists and facts
should come from Markdown or structured data: `data/*.json`, `../docs/*.md`,
`go.mod`, registry extraction, YANG extraction, git history, or session `ze`
output. Extend a renderer or extractor before you hardcode a catalog in HTML.

Every published page needs an AI-readable `index.md` sibling. Generate it from
the same Markdown or structured data as the HTML; only hand-authored HTML pages
may rely on the `nav` step's HTML-to-Markdown extraction. `llms.txt` must remain
generated from `data/nav.json`, `tools/page_registry.py`, page Markdown, and live <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
counts. It must link each page's `index.md` first and include the human web URL
as the secondary link.

To add, remove, or re-categorize a feature card: edit `data/features.json`,
then run `tools/build.py --only features` (or the full build). Same for <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
`data/audience.json` and `data/whats-new.json` with `--only index`. For
navigation, edit `data/nav.json`
and run `tools/build.py --only nav`; only `assets/header.html` should change <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
after the one-time migration to shared mounts.

Command equivalence maintenance: Ze command paths come from the live CLI
catalog, not the JSON mapping. Add vendor equivalents to
`data/command-equivalents.json`, keep every `ze` path exact, build the session
`ze` binary with `make ze-build` (not a `zetest` binary), then run
`tools/build.py --only cli,command-equivalents,search,seo,llms` after command-tree <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
changes so `data/cli-commands.json`, the page, search index, metadata, and
`llms.txt` stay aligned. For mapping-only edits where the CLI catalog is already
fresh, `tools/build.py --only command-equivalents,search,seo,llms` is enough. <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->

### Plugin catalog

`reference/plugins/` is generated from `data/plugin-registry.json`, which
is generated from `../internal/**/register.go` plus optional local
`PLUGIN.md` front matter beside a plugin's `register.go`. Do not hand-edit
plugin cards, dependency lists, source paths, or detail pages. Add or fix
machine facts in `registry.Registration`; add local prose or display metadata
in that plugin's `PLUGIN.md`; then run `tools/build.py --only
plugins,search,seo,llms` or the full build.

The catalog renderer creates `reference/plugins/index.html`, its `index.md`
mirror, and one local `reference/plugins/<plugin>/` detail page
per registry entry. Card clicks must stay on the site. If the page needs a new
machine fact, extend the extractor or registry data instead of adding a
hardcoded list to the renderer.

## Presentation tooling

Tools live at `presentations/tools/`; published decks live at `talks/<slug>/`.

| Tool | Purpose |
|------|---------|
| `bundle-html.py` | Inlines local images, `slides.md`, and embeds into `<name>-inlined.html`. |
| `project-root.sh` | Resolves the main Ze source worktree through Git, independent of the caller's directory. |
| `update-stats.sh` | Reads current project statistics from the resolved source worktree. |
| `presentation-screenshots.sh` | Starts Ze with a demo config and writes captures under the source worktree's `tmp/`. |
| `linx-screenshots.sh` | Captures screenshots into `talks/linx-2026-06/screenshots/`. |
| `loc_activity.py` | Generates the activity heatmap from the main source worktree. |

## Adding a new presentation

1. Create a new directory under `talks/`
2. Add `index.html` (self-contained or Markdown renderer + `slides.md`)
3. Add `screenshots/` with extracted images (not base64-embedded)
4. Add a record to `data/talks.json`
5. Add a path-independent `update.sh` beside the deck to regenerate its inlined file.

## Updating presentations

The per-deck update scripts resolve their tools and source worktree from their
own location, so they can be called from any directory:

```sh
website/talks/linx-2026-06/update.sh
website/talks/linx-2026-06/update.sh --bundle-only
website/talks/netmcr-2026-04/update.sh
```

The first LINX command refreshes its project statistics and activity page. The
other two commands only regenerate the standalone HTML deck.

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
   never a tag. `tools/build.py` fails on any tag not in `data/topics.json`; if <!-- doc-links: ignore (path is relative to website/ in this architecture note) -->
   the week did something genuinely new, add the tag to the vocabulary (with
   its category) rather than forcing a near-miss.

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
5. **Run `make ze-site-generate`.** One command regenerates everything: the new
   weekly page and the terse Changes index (plus `changes/feed.xml`), the
   editorial blog index, the activity heatmap from fresh git history,
   `compare/index.html` from `compare/comparison.md`, `features/index.html`
   from `data/features.json`, `index.html` from `data/audience.json`, every
   imported `../docs/*.md` at their registered public destinations, the
   nav block on every hand-authored page (labs, talks index, style guide,
   performance, Zeledon), and every applicable page's `index.md` sibling. The
   `nav` step re-derives Markdown mirrors for hand-authored shell pages from
   their HTML. Watch stderr for the
   feature-count drift warning, the missing-`index.md` warning, and any
   per-step failures.
6. **Link-check before calling it done.** Every `href`/`src` across the
   published site outside the frozen `talks/<slug>/` decks, and every link
   inside every generated `index.md`, should resolve to a real local file or an
   external URL. `./update-website.sh` includes the local page-link check and
   leaves the validated result in `../gh-pages`.
7. **Never edit `talks/<slug>/` content** as part of this checklist. Those decks
   are historic snapshots frozen at the time they were given.
