# Vendored Web Assets

Third-party web assets used by Ze's web interfaces.
Source of truth: files in this directory. Consumer copies are synced by `internal/le/web/vendor`.

## Assets

| Asset | Version | Licence | Source | Vendored |
|-------|---------|---------|--------|----------|
| htmx.min.js | 4.0.0 | Zero-Clause BSD (`htmx/htmx.LICENSE`) | https://unpkg.com/htmx.org@4.0.0/dist/htmx.min.js | 2026-09-02 |
| hx-sse.min.js | 4.0.0 (htmx ext) | Zero-Clause BSD (`htmx/htmx.LICENSE`) | https://unpkg.com/htmx.org@4.0.0/dist/ext/hx-sse.min.js | 2026-09-02 |
| ze.svg | - | Ze's own, no third-party licence | `docs/logo/ze.svg` (project logo with Exa gradient) | 2026-03-31 |
| swagger-ui/swagger-ui.css | 5.32.2 | Apache License (Version 2.0, January 2004) (`swagger-ui/swagger-ui.LICENSE`, `swagger-ui/swagger-ui.NOTICE`); embeds normalize.css v7.0.0, whose banner reads "MIT License" | https://unpkg.com/swagger-ui-dist@5.32.2/swagger-ui.css | 2026-04-11 |
| swagger-ui/swagger-ui-bundle.js | 5.32.2 | Apache License (Version 2.0, January 2004) (`swagger-ui/swagger-ui.LICENSE`, `swagger-ui/swagger-ui.NOTICE`); the libraries it bundles are in `swagger-ui/swagger-ui-bundle.js.LICENSE.txt` | https://unpkg.com/swagger-ui-dist@5.32.2/swagger-ui-bundle.js | 2026-04-11 |
| uplot/uPlot.min.js | 1.6.32 | The MIT License (MIT) (`uplot/uPlot.LICENSE`) | https://unpkg.com/uplot@1.6.32/dist/uPlot.iife.min.js | 2026-04-22 |
| uplot/uPlot.min.css | 1.6.32 | The MIT License (MIT) (`uplot/uPlot.LICENSE`) | https://unpkg.com/uplot@1.6.32/dist/uPlot.min.css | 2026-04-22 |

Each licence names the licence as the licence file's own title states it.

## Licence files

Each third-party directory carries its package's licence as published at the
vendored version, byte for byte. The name carries the asset's name, because
one file name cannot be vendored in two directories (`./le web vendor check`
refuses it).

| File | Read from | Fetched |
|------|-----------|---------|
| htmx/htmx.LICENSE | https://unpkg.com/htmx.org@4.0.0/LICENSE | 2026-11-20 |
| uplot/uPlot.LICENSE | https://unpkg.com/uplot@1.6.32/LICENSE | 2026-11-20 |
| swagger-ui/swagger-ui.LICENSE | https://unpkg.com/swagger-ui-dist@5.32.2/LICENSE | 2026-11-20 |
| swagger-ui/swagger-ui.NOTICE | https://unpkg.com/swagger-ui-dist@5.32.2/NOTICE | 2026-11-20 |
| swagger-ui/swagger-ui-bundle.js.LICENSE.txt | https://unpkg.com/swagger-ui-dist@5.32.2/swagger-ui-bundle.js.LICENSE.txt | 2026-11-20 |

The bundle's companion file keeps its upstream name: the bundle's first line
reads "For license information please see swagger-ui-bundle.js.LICENSE.txt".
A licence file is recognised by its name ending in `.LICENSE` or
`.LICENSE.txt` (`internal/le/web/vendor/vendorweb.go`, `licenceSuffixes`).
<!-- source: internal/le/web/vendor/vendorweb.go -- licenceSuffixes, licenceExempt -->
<!-- source: internal/le/web/vendor/check.go -- licenceCheck -->

When a package is upgraded, its licence files are fetched again from the new
version and this table is updated.

Both htmx files come from one npm package, `htmx.org@4.0.0`: htmx 4 holds
its extensions in the core package, where htmx 2 published `htmx-ext-sse`
separately. The core keeps the name `htmx.min.js` that every page has always
loaded, so the cutover changed the bytes behind the name rather than the name.
The extension is named as htmx 4 publishes it, `hx-sse.min.js`.

htmx 2 is gone from this tree. Its core and its `sse.js` were deleted in the
same change that served htmx 4 (`ai/rules/no-layering.md`): two versions in the
tree is the state where a page silently loads the wrong one. A page loads the
extension only when it streams, and the per-page sets are derived
(`internal/le/web/assets`).

## Vendor directories

A directory under `third_party/web/` is the unit a consumer subscribes to. A
consumer that holds one file of a directory MUST hold every file of that
directory, and `./le web vendor check` reads the two trees that way. An
asset for one consumer only gets its own directory, as `swagger-ui/` does. A
directory that reaches no consumer is a problem too: it says the sync was never
told to copy it.

A directory also holds its package's licence file, so every consumer that
serves the package serves its licence beside it. `./le web vendor check`
refuses a directory with no licence file, naming it (`UNLICENSED`).
`ze/` is the one exception, declared in the code with its reason: it holds
Ze's own logo, which carries no third-party licence.

## Upgrade scanner provenance

Ze's native Go upgrade scanner in `internal/le/web/htmx/` transcribes the
rules and behavior of htmx's 4.0.0 upgrade checker:

- Upstream source: https://unpkg.com/htmx.org@4.0.0/dist/scripts/upgrade-check.py
- Upstream source SHA-256: `abcf7cc3ce3162911a1352ed7ad21aa32da09c8f9a92725ba32d7e29d3ca480b`
- Native rule-contract SHA-256: `3e4faee901b9b437d2220b7402689f8f2af4d298dd97a6ff4dacd9c496db9a4b`

The upstream source is provenance only. Ze neither vendors nor executes it.
The compiled tables, DOM inheritance fixtures, parser boundaries, issue order,
and exact report rows are checked by `internal/le/web/htmx/htmxupgrade_test.go`,
so both upgrade actions remain offline and deterministic.

## Consumers

| Consumer | Path | Embed |
|----------|------|-------|
| chaos web | `internal/chaos/web/assets/` | `go:embed` in `internal/chaos/web/handlers.go` |
| looking glass | `internal/component/lg/assets/` | `go:embed` in `internal/component/lg/embed.go` |
| component web | `internal/component/web/assets/` | `go:embed` in `internal/component/web/render.go` |
| REST API docs | `internal/component/api/rest/assets/` | `go:embed` in `internal/component/api/rest/embed.go` |

## Sync

```bash
./le web vendor update         # copy from third_party/web/ to every consumer
./le repo update       # runs the same sync, with the other generators
./le web vendor check          # gate: each consumer copy matches its source
./le web vendor report         # ask the npm registry for newer versions
./le web htmx check        # gate: no unexplained htmx 4 upgrade issue
./le web htmx report       # print every htmx 4 upgrade issue
```

The consumer copies are generated files and they stay tracked in git. `//go:embed`
reads them at compile time, and `./le repo compiles check` compiles what git
holds, so a build that runs no generator must find them.

`./le web vendor check` is a stage of `./le verify current` and a
prerequisite of `./le repo check generated`. It exits non-zero when a copy
differs or is absent, and when a vendored directory holds no licence file. It queries no registry, so it runs with no network.

`./le web vendor report` reads EVERY npm dist-tag and answers the newest
release among them, prereleases excluded. htmx is why: 4.0.0 shipped under the
`next` tag and `latest` stayed on 2.0.10, so a query for `latest` alone reports
the release this tree already holds as a downgrade.
