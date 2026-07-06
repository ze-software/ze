# GitHub Pages and Presentations

The `gh-pages` branch is checked out as a worktree at `../gh-pages` (sibling of `main/`).

All presentation content, tooling, and structure documentation lives there.
See `../gh-pages/AI.md` for the full reference: structure, tools, and how to add a talk.


## Plugin catalog

The website plugin catalog at `../gh-pages/docs/features/plugins/` is
generated, not hand-authored. Its data source is each plugin's
`registry.Registration`: name, description, config roots, dependencies,
optional dependencies, and YANG modules.
<!-- source: internal/component/plugin/registry/registry.go -- Registration metadata -->

The gh-pages extractor reads `../main/internal/**/register.go`, resolves YANG
imports, and writes `../gh-pages/data/plugin-registry.json`. The catalog
renderer derives groups from `ConfigRoots` and repository source paths, then
emits the catalog plus one local detail page per plugin. Do not add a parallel
hand-written plugin list in the website.
<!-- source: ../gh-pages/tools/extract-plugin-registry.py -- plugin registry extraction -->
<!-- source: ../gh-pages/tools/render-plugin-catalog.py -- plugin catalog renderer -->

When adding or changing a plugin, update the registration metadata in the
plugin's `register.go`. If the website needs another fact, add a structured
field to the registry or extractor first, then render from that data. Regenerate
the site from `../gh-pages` with `tools/build.py --only config,plugins,search,seo`
or run the full `tools/build.py`.