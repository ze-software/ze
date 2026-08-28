# GitHub Pages and Presentations

Website sources live under `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`./le site build` writes the publishable artifact to `../gh-pages` and removes
old source-only files there. It reuses matching demo artifacts. Run
`./le terminal-demo render-all` first to force new demo artifacts.
`./le terminal-demo render name <demo-id>` re-records one demo while you work on
its tape, and publishes it beside the artifacts it did not record. The ids are in
`demos/terminal/manifest.json`.
See `website/AI.md` for the full reference: structure, tools, and how to add a talk.


## Plugin catalog

The website plugin catalog at `../gh-pages/reference/plugins/` is
generated, not hand-authored. Its data source is each plugin's
`registry.Registration`: name, description, config roots, dependencies,
optional dependencies, and YANG modules.
<!-- source: internal/component/plugin/registry/registry.go -- Registration metadata -->

The native site builder reads the plugin registrations and YANG modules, writes
`../gh-pages/data/plugin-registry.json`, and renders the catalog plus one local
detail page per plugin. Do not add a parallel hand-written plugin list.
<!-- source: internal/le/site/build.go -- Build -->

When adding or changing a plugin, update the registration metadata in the
plugin's `register.go`. If the website needs another fact, add a structured
field to the registry or extractor first, then render from that data.
Regenerate the site with `./le site build`.