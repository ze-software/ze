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

1. **Render the blog.** `tools/render-blog.py` -- regenerates the new post,
   the blog index, and the (calculated, not hardcoded) week count.
2. **Check Features for drift.** Did the week ship something with no card
   yet, or move a feature from Experimental to shipped? Cross-check
   `../main/docs/features.md` / `docs/features/*.md` maturity against
   `features/index.html`. Adding or removing a card changes the count in
   its intro paragraph -- update that number too.
3. **Regenerate Activity.** `tools/render-activity.py` -- pulls fresh commit
   and added-line data from `main` (never touches any `presentations/*/`
   snapshot, those are historic and frozen).
4. **Check Compare for drift.** Did the week close one of the
   "Where Ze is behind today" gaps, or change a Yes/No/Partial cell? Edit
   `../main/docs/comparison.md` first (the source of truth), then re-run
   `tools/render-doc.py compare/comparison.md compare/index.html --root ../
   --desc "..." --cat routing` to refresh the copy here. Bump the
   `Last updated:` date in main's file and the "as of" date in this file's
   disclaimer when real content changes -- never let this page carry content
   main doesn't also have.
5. **Check Labs for drift.** Did the week add new interop/QEMU evidence
   substantial enough for its own lab page (see `labs/bgp-interop/` as the
   template)?
6. **Check Performance for drift.** Did `../main/docs/performance.md` get a
   fresh benchmark run this week? If so, update the headline stat-row on
   `performance/index.html` (Convergence/Throughput/Withdrawal) to match.
7. **Re-render docs if any `../main/docs/*.md` changed.**
   `tools/render-docs.py` (batch) or `tools/render-doc.py` (single file) --
   see each script's own docstring.
8. **Link-check before calling it done.** Every `href`/`src` across the
   published site (excluding `presentations/`) should resolve to a real
   local file or an external URL. Write a quick script that walks all
   `*.html` files and resolves relative links via `pathlib`, or reuse one
   from a prior session if still on disk.
9. **Never edit `presentations/*/` content** as part of this checklist --
   those decks are historic snapshots frozen at the time they were given.
