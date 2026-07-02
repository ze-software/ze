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
    audience.json                         -- the "Two ways to run Ze" and "Who should look now"
                                              cards on index.html
  tools/
    sitelib.py                            -- shared nav/head/foot chrome, imported by every
                                              render-*.py; also the navblock patcher for pages
                                              with no dedicated generator
    build.py                              -- regenerates the entire site in one command
    render-docs.py / render-doc.py        -- ../main/docs/*.md -> docs/**/index.html (also used
                                              directly for compare/comparison.md -> compare/index.html)
    render-blog.py                        -- blog/posts/*.md -> blog/**/index.html
    render-activity.py                    -- git history -> activity/index.html
    render-features.py                    -- data/features.json -> features/index.html
    render-index.py                       -- data/audience.json + template -> index.html
```

Pages with no dedicated generator (`zeledon/`, `labs/*/`, `talks/`,
`style-guide/`, `performance/`) are hand-authored HTML for their body content,
but their nav block is still patched from `data/nav.json` by `tools/build.py`
(the `nav` step) so they can never drift from the mega-menu on every other
page.

Run `tools/build.py` (or `tools/build.py --only <docs,blog,activity,compare,
features,index,nav>` for a subset) to regenerate everything. It warns on
stderr if `data/nav.json`'s Features-dropdown card count falls out of sync
with `data/features.json`'s actual card count -- the class of bug that
motivated data-driving this in the first place (a hand-typed "41 features"
count silently went stale when a card moved between sections).

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

Trigger: a new `blog/posts/<start-date>.md` lands (the week's Discord `ze-news`
update). Work through this before considering the update done.

1. **Check Features for drift.** Did the week ship something with no card
   yet, or move a feature from Experimental to shipped? Add/move/edit its
   entry in `data/features.json` -- the intro paragraph's count is computed
   from the data at render time, nothing to hand-update.
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
5. **Run `tools/build.py`.** One command regenerates everything: the new
   blog post and blog index, the activity heatmap from fresh git history,
   `compare/index.html` from `compare/comparison.md`, `features/index.html`
   from `data/features.json`, `index.html` from `data/audience.json`, every
   `../main/docs/*.md` -> `docs/**/index.html`, and the nav block on every
   hand-authored page (labs, talks, style guide, performance, zeledon).
   Watch stderr for the feature-count drift warning and any per-step
   failures.
6. **Link-check before calling it done.** Every `href`/`src` across the
   published site (excluding `presentations/`) should resolve to a real
   local file or an external URL. Write a quick script that walks all
   `*.html` files and resolves relative links via `pathlib`, or reuse one
   from a prior session if still on disk.
7. **Never edit `presentations/*/` content** as part of this checklist --
   those decks are historic snapshots frozen at the time they were given.
