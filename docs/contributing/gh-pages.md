# GitHub Pages and Presentations

Website sources live under `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`./le site build` writes the publishable artifact to `../gh-pages` and removes
old source-only files there. It reuses matching demo artifacts. Run
`./le terminal-demo render-all` first to force new demo artifacts.
`./le terminal-demo render name <demo-id>` re-records one demo while you work on
its tape, and publishes it beside the artifacts it did not record. The ids are in
`demos/terminal/manifest.json`.

A recording runs in a container this repository builds and publishes to no
registry, so build it once per checkout with `./le terminal-demo image-build`.
It reads the image tag from `demos/terminal/manifest.json`, which is the tag the
recorder runs. A render refuses to start when that image is absent and names
this action. Rebuild after any change to `demos/terminal/Dockerfile`.

See `website/AI.md` for the full reference: structure, tools, and how to add a talk.


## Plugin catalog

The website plugin catalog at `../gh-pages/reference/plugins/` is
generated, not hand-authored. Its data source is each plugin's
`registry.Registration`: name, description, config roots, dependencies,
optional dependencies, and the YANG schema it registers.
<!-- source: internal/component/plugin/registry/registry.go -- Registration metadata -->

Two facts the catalog shows are DERIVED rather than declared, so do not look for
a field to fill in. The source directory is the package the plugin's engine
function was compiled in. The YANG file list is every `.yang` file in the
directory that holds the module the registration carries, and beside the
package when it carries none.
<!-- source: internal/le/inventory/plugins.go -- pluginPackageDir, pluginYANGFiles -->

A build reads the plugin registrations, writes
`../gh-pages/data/plugin-registry.json`, and renders the catalog plus one local
detail page per plugin. It reads the daemon's own configuration schema, writes
`../gh-pages/data/yang-config-tree.json`, and renders the configuration
reference from that tree and the same registrations. Do not add a parallel
hand-written plugin list.
<!-- source: internal/le/site/build.go -- refreshNativeSurfaces -->
<!-- source: internal/le/site/plugins.go -- renderPluginCatalog -->
<!-- source: internal/le/site/config.go -- renderConfiguration -->

A plugin the catalog no longer carries loses its page: a build removes every
detail directory whose plugin is not in the registry it just read.

A config root that names no node of the configuration schema STOPS the build.
The owning plugin's section would otherwise publish as core, with its owner and
its YANG source silently absent.

When adding or changing a plugin, update the registration metadata in the
plugin's `register.go`. If the website needs another fact, add a structured
field to `registry.Registration` first, then render from that data. Regenerate
the site with `./le site build`.

## Quality pages

`../gh-pages/quality/health/` and `../gh-pages/quality/rfc-compliance/` are
rendered from the tree being built, through the two packages that own those
numbers. Neither page computes a figure of its own.

The testing-health page reads `internal/le/testhealth.Render`, which answers the
metric record and the Markdown mirror in one pass. The mirror it publishes is
`docs/features/test-health.md`'s own bytes, so the site is never a second author
of that document.

The RFC compliance report reads `internal/le/rfc`: `Collect` for the
requirements and the test tags, `NewRenderInput` for the public ledger and the
recorded audit verdicts, and `Check` for the verdict and the open issues. It
writes `../gh-pages/data/rfc-compliance.json`, which is the same answer in
machine-readable form and is linked from the page.

Two figures the retired renderer published are gone rather than ported. It
counted a per-check error total, which the Go gate does not answer: it returns
one list of open issues, and the page publishes that list. It also grepped a
marker string out of a hook file, a Makefile target and a status script, all
three of which were deleted with the interpreter, to claim an agent guard was
ON. The page states the live verification wiring instead, read from the declared
pre-commit stage population.
<!-- source: internal/le/site/health.go -- renderHealth -->
<!-- source: internal/le/site/rfccompliance.go -- collectRFCCompliance -->
<!-- source: internal/le/verify/engine/stages.go -- fullStages -->
