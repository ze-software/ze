# Vendored Web Assets

Third-party web assets used by Ze's web interfaces.
Source of truth: files in this directory. Consumer copies are synced by `internal/le/vendorweb`.

## Assets

| Asset | Version | Source | Vendored |
|-------|---------|--------|----------|
| htmx.min.js | 4.0.0-beta6 | https://unpkg.com/htmx.org@4.0.0-beta6/dist/htmx.min.js | 2026-08-15 |
| hx-sse.min.js | 4.0.0-beta6 (htmx ext) | https://unpkg.com/htmx.org@4.0.0-beta6/dist/ext/hx-sse.min.js | 2026-08-15 |
| ze.svg | - | `docs/logo/ze.svg` (project logo with Exa gradient) | 2026-03-31 |
| swagger-ui/swagger-ui.css | 5.32.2 | https://unpkg.com/swagger-ui-dist@5.32.2/swagger-ui.css | 2026-04-11 |
| swagger-ui/swagger-ui-bundle.js | 5.32.2 | https://unpkg.com/swagger-ui-dist@5.32.2/swagger-ui-bundle.js | 2026-04-11 |
| uplot/uPlot.min.js | 1.6.32 | https://unpkg.com/uplot@1.6.32/dist/uPlot.iife.min.js | 2026-04-22 |
| uplot/uPlot.min.css | 1.6.32 | https://unpkg.com/uplot@1.6.32/dist/uPlot.min.css | 2026-04-22 |

Both htmx files come from one npm package, `htmx.org@4.0.0-beta6`: htmx 4 holds
its extensions in the core package, where htmx 2 published `htmx-ext-sse`
separately. The core keeps the name `htmx.min.js` that every page has always
loaded, so the cutover changed the bytes behind the name rather than the name.
The extension is named as htmx 4 publishes it, `hx-sse.min.js`.

htmx 2 is gone from this tree. Its core and its `sse.js` were deleted in the
same change that served htmx 4 (`ai/rules/no-layering.md`): two versions in the
tree is the state where a page silently loads the wrong one. A page loads the
extension only when it streams, and the per-page sets are derived
(`internal/le/webassets`).

## Vendor directories

A directory under `third_party/web/` is the unit a consumer subscribes to. A
consumer that holds one file of a directory MUST hold every file of that
directory, and `./le vendor-web check` reads the two trees that way. An
asset for one consumer only gets its own directory, as `swagger-ui/` does. A
directory that reaches no consumer is a problem too: it says the sync was never
told to copy it.

## Upgrade scanner provenance

Ze's native Go upgrade scanner in `internal/le/htmxupgrade/` transcribes the
rules and behavior of htmx's 4.0.0-beta6 upgrade checker:

- Upstream source: https://unpkg.com/htmx.org@4.0.0-beta6/dist/scripts/upgrade-check.py
- Upstream source SHA-256: `9633ce96b7d16d8ef2c11a6da91a6f0adcea891bec663e005249aea39df7a58b`
- Native rule-contract SHA-256: `889b22c7c227548392f8567e65a7472beb9243516a97c399a5e35c5b6402fcf8`

The upstream source is provenance only. Ze neither vendors nor executes it.
The compiled tables, DOM inheritance fixtures, parser boundaries, issue order,
and exact report rows are checked by `internal/le/htmxupgrade/htmxupgrade_test.go`,
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
./le vendor-web sync           # copy from third_party/web/ to every consumer
./le repository generate       # runs the same sync, with the other generators
./le vendor-web check          # gate: each consumer copy matches its source
./le vendor-web update-report  # ask the npm registry for newer versions
./le htmx-upgrade check        # gate: no unexplained htmx 4 upgrade issue
./le htmx-upgrade report       # print every htmx 4 upgrade issue
```

The consumer copies are generated files and they stay tracked in git. `//go:embed`
reads them at compile time, and `./le repository-tracked-build check` compiles what git
holds, so a build that runs no generator must find them.

`./le vendor-web check` is a stage of `./le verify current mode full` and a
prerequisite of `./le repository generated-check`. It exits non-zero when a copy
differs or is absent. It queries no registry, so it runs with no network.
