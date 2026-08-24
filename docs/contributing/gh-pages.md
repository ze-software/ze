# GitHub Pages and Presentations

Website sources live under `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`make ze-site-generate` writes the publishable artifact to `../gh-pages` and removes old source-only files there.
It reuses the existing demo artifacts when `assets/demos/manifest.json` matches the checked-in tape definitions.
Use `make ze-terminal-demo-release-render-all` to force new demo artifacts.
See `website/AI.md` for the full reference: structure, tools, and how to add a talk.


## Plugin catalog

The website plugin catalog at `../gh-pages/reference/plugins/` is
generated, not hand-authored. Its data source is each plugin's
`registry.Registration`: name, description, config roots, dependencies,
optional dependencies, and YANG modules.
<!-- source: internal/component/plugin/registry/registry.go -- Registration metadata -->

The website extractor reads `internal/**/register.go`, resolves YANG imports,
and writes `../gh-pages/data/plugin-registry.json`. The catalog
renderer derives groups from `ConfigRoots` and repository source paths, then
emits the catalog plus one local detail page per plugin. Do not add a parallel
hand-written plugin list in the website.
<!-- source: website/tools/extract-plugin-registry.py -- plugin registry extraction -->
<!-- source: website/tools/render-plugin-catalog.py -- plugin catalog renderer -->

When adding or changing a plugin, update the registration metadata in the
plugin's `register.go`. If the website needs another fact, add a structured
field to the registry or extractor first, then render from that data.
Regenerate the site with `make ze-site-generate`.