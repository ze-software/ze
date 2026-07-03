# gh-pages Branch

Published site: landing page + presentations. Deployed via GitHub Pages (branch mode, root `/`).

## Worktree

Checked out as a worktree at `../gh-pages` (sibling of `main/`).
Edit published content here, commit and push from this worktree.

## Structure

```
gh-pages/
  .nojekyll
  index.html                              -- project landing page
  assets/                                 -- shared assets (logos)
  presentations/
    tools/                                -- shared presentation tooling
    linx-2026-06/                         -- one directory per talk
      index.html                          -- slide renderer (loads slides.md)
      slides.md                           -- slide content (markdown)
      zeledon.svg                         -- logo
      screenshots/                        -- presentation screenshots (PNGs)
    netmcr-2026-04/
      index.html                          -- self-contained presentation
      screenshots/                        -- extracted images
```

Each presentation is self-contained: its `index.html` loads assets from its own directory.

## Site build tooling

The published site (everything except `presentations/`) is generated from
data and Markdown, not hand-edited HTML. Structure:

```
gh-pages/
  data/
    nav.json                              -- single source for the mega-menu (Project/Labs/Docs
                                              dropdowns, top links, badges) -- every page reads
                                              this, generated or hand-authored
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
    dependencies.json                     -- every direct Go dependency's "why", grouped by
                                              category, keyed to ../main/go.mod
    plugin-registry.json                  -- every plugin's Registration{} fields + resolved YANG
                                              module paths, extracted fresh from ../main/internal/
  tools/
    sitelib.py                            -- shared nav/head/foot chrome, imported by every
                                              render-*.py; also the navblock patcher for pages
                                              with no dedicated generator, and the Markdown-mirror
                                              machinery (see "Markdown mirrors" below)
    build.py                              -- regenerates the entire site in one command
    render-docs.py / render-doc.py        -- ../main/docs/*.md -> docs/**/index.html (also used
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
    render-cli-catalog.py                 -- `ze help command --json` -> cli/index.html, with a
                                              live search box that jumps to a matching command's
                                              anchor (id="cmd-<slug>") in its group
    render-dependencies.py                -- ../main/go.mod + data/dependencies.json -> dependencies/index.html
    extract-plugin-registry.py            -- ../main/internal/**/register.go + YANG imports ->
                                              data/plugin-registry.json
    render-config-reference.py            -- data/plugin-registry.json -> config-reference/index.html,
                                              every plugin (not just BGP) grouped by config root
    render-index.py                       -- data/audience.json + template -> index.html
    render-llms-txt.py                    -- data/nav.json + live counts -> llms.txt
  update-website.sh                       -- thin wrapper at the repo root: `./update-website.sh`
                                              regenerates everything, same as `tools/build.py`.
                                              Forwards args, e.g. `./update-website.sh --only cli`
```

Pages with no dedicated generator (`zeledon/`, `labs/*/`, `talks/`,
`style-guide/`, `performance/`) are hand-authored HTML for their body content,
but their nav block is still patched from `data/nav.json` by `tools/build.py`
(the `nav` step) so they can never drift from the mega-menu on every other
page.

Run `./update-website.sh` (repo root) or `tools/build.py` directly -- same
thing, the script is just a short, obvious name to reach for. Pass `--only
<docs,blog,activity,compare,features,cli,deps,config,contribute,index,timeline,nav,llms>`
to regenerate a subset. It warns on stderr if `data/nav.json`'s Features-dropdown
card count falls out of sync with `data/features.json`'s actual card count,
its CLI Reference command count with `data/cli-commands.json`, or its
Configuration Reference plugin count with `data/plugin-registry.json` -- the
class of bug that motivated data-driving this in the first place (a
hand-typed "41 features" count silently went stale when a card moved between
sections).

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
- **Built from JSON/data** (`features/`, `cli/`, `dependencies/`,
  `config-reference/`, `activity/`, blog posts): each `render-*.py` has a
  `render_markdown()` next to its `render()`, both reading the same data,
  so the two can't disagree.
- **Hand-authored HTML, no source of either kind** (`labs/*/`, `talks/`,
  `style-guide/`, `performance/`, `zeledon/`): `tools/build.py`'s `nav`
  step extracts the `<main id="top">...</main>` content
  (`sitelib.extract_main`) and converts it with `sitelib.html_to_markdown`
  -- a small HTML->Markdown converter written against exactly the tags and
  component classes this site emits (status-row facts, stat tiles,
  terminal panels, chips/tags, tables), not a general-purpose library.
  Re-derived from the HTML on every build, so it can't drift from the page
  the way a hand-maintained companion file would.

`tools/build.py` runs `nav` before `llms` so every page `llms.txt` links to
already has its `index.md` on disk, and warns on stderr
(`check_llms_md_siblings`) if a `data/nav.json` entry's `index.md` is
missing.

To add, remove, or re-categorize a feature card: edit `data/features.json`,
then run `tools/build.py --only features` (or the full build). Same for
`data/audience.json` and `--only index`, or `data/nav.json` and `--only nav`
(plus regenerate anything else the nav change should also touch).

## Presentation tooling

Tools live at `presentations/tools/`.

| Tool | Purpose |
|------|---------|
| `bundle-html.py` | Inlines local images, slides.md, and embeds into a self-contained HTML file. Output: `<name>-inlined.html`. Accepts multiple files. |
| `presentation-screenshots.sh` | Starts ze with a demo config, captures CLI and browser screenshots. Uses project root `tmp/`. |
| `linx-screenshots.sh` | Captures screenshots specific to the LINX presentation. |
| `loc_activity.py` | GitHub-style activity heatmap from git history. `python3 loc_activity.py [--serve] [--output <path>] [--days N]` |

Screenshot capture scripts also exist per-deck (e.g., `linx-2026-06/update.sh`).

## Adding a new presentation

1. Create a new directory under `presentations/`
2. Add `index.html` (self-contained or markdown-renderer + `slides.md`)
3. Add `screenshots/` with extracted images (not base64-embedded)
4. Add a card linking to it in `index.html` (Talks section)
5. Generate inlined version: `python3 presentations/tools/bundle-html.py <path>/index.html`

## Updating presentations

Edit here, commit and push from this worktree.
Do not edit presentation content on main.

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
   never a tag. `tools/build.py` fails on any tag not in `data/topics.json`; if
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
   `../main/docs/comparison.md` first (the source of truth), then copy the
   change into `compare/comparison.md` here (or re-run whatever produced
   that mirror). Bump the `Last updated:` date in main's file and the "as
   of" date in this file's disclaimer when real content changes -- never let
   this page carry content main doesn't also have.
3. **Check Labs for drift.** Did the week add new interop/QEMU evidence
   substantial enough for its own lab page (see `labs/bgp-interop/` as the
   template)? A new lab also needs an entry in `data/nav.json`'s Labs
   dropdown.
4. **Check Performance for drift.** Did `../main/docs/performance.md` get a
   fresh benchmark run this week? If so, update the headline stat-row on
   `performance/index.html` (Convergence/Throughput/Withdrawal) to match.
5. **Run `./update-website.sh`.** One command regenerates everything: the new
   weekly page and the terse Changes index (plus `changes/feed.xml`), the
   editorial blog index, the activity heatmap from fresh git history,
   `compare/index.html` from `compare/comparison.md`, `features/index.html`
   from `data/features.json`, `index.html` from `data/audience.json`, every
   `../main/docs/*.md` -> `docs/**/index.html`, the nav block on every
   hand-authored page (labs, talks, style guide, performance, zeledon), and
   every page's `index.md` sibling (see "Markdown mirrors" above) --
   including the hand-authored pages', which need no manual step since the
   `nav` step re-derives them from the HTML every run. Watch stderr for the
   feature-count drift warning, the missing-`index.md` warning, and any
   per-step failures.
6. **Link-check before calling it done.** Every `href`/`src` across the
   published site (excluding `presentations/`), and every link inside every
   `index.md`, should resolve to a real local file or an external URL.
   Write a quick script that walks all `*.html`/`*.md` files and resolves
   relative links via `pathlib`, or reuse one from a prior session if still
   on disk.
7. **Never edit `presentations/*/` content** as part of this checklist --
   those decks are historic snapshots frozen at the time they were given.
