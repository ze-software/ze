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
