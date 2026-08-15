# Vendored Web Assets

Third-party web assets used by Ze's web interfaces.
Source of truth: files in this directory. Consumer copies are synced via `scripts/vendor/sync_web.go`.

## Assets

| Asset | Version | Source | Vendored |
|-------|---------|--------|----------|
| htmx.min.js | 2.0.4 | https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js | 2026-03-27 |
| sse.js | 2.0.4 (htmx ext) | https://unpkg.com/htmx-ext-sse@2.0.4/sse.js | 2026-03-27 |
| htmx4.min.js | 4.0.0-beta5 | https://unpkg.com/htmx.org@4.0.0-beta5/dist/htmx.min.js | 2026-08-15 |
| hx-sse.min.js | 4.0.0-beta5 (htmx ext) | https://unpkg.com/htmx.org@4.0.0-beta5/dist/ext/hx-sse.min.js | 2026-08-15 |
| ze.svg | - | `docs/logo/ze.svg` (project logo with Exa gradient) | 2026-03-31 |
| swagger-ui/swagger-ui.css | 5.32.2 | https://unpkg.com/swagger-ui-dist@5.32.2/swagger-ui.css | 2026-04-11 |
| swagger-ui/swagger-ui-bundle.js | 5.32.2 | https://unpkg.com/swagger-ui-dist@5.32.2/swagger-ui-bundle.js | 2026-04-11 |
| uplot/uPlot.min.js | 1.6.32 | https://unpkg.com/uplot@1.6.32/dist/uPlot.iife.min.js | 2026-04-22 |
| uplot/uPlot.min.css | 1.6.32 | https://unpkg.com/uplot@1.6.32/dist/uPlot.min.css | 2026-04-22 |

`htmx4.min.js` and `hx-sse.min.js` are vendored and synced, and no page loads
them. Every page serves `htmx.min.js` at 2.0.4. The cutover spec
(`spec-web-htmx4-cutover`) makes htmx 4 live. Both files come from one npm
package, `htmx.org@4.0.0-beta5`: htmx 4 holds its extensions in the core
package, where htmx 2 published `htmx-ext-sse` separately. The local name of
the core file is `htmx4.min.js` because the 2.0.4 file keeps the name
`htmx.min.js` until the cutover.

## Vendor directories

A directory under `third_party/web/` is the unit a consumer subscribes to. A
consumer that holds one file of a directory MUST hold every file of that
directory, and `scripts/vendor/check_web.go` reads the two trees that way. An
asset for one consumer only gets its own directory, as `swagger-ui/` does. A
directory that reaches no consumer is a problem too: it says the sync was never
told to copy it.

## Consumers

| Consumer | Path | Embed |
|----------|------|-------|
| chaos web | `internal/chaos/web/assets/` | `go:embed` in `internal/chaos/web/handlers.go` |
| looking glass | `internal/component/lg/assets/` | `go:embed` in `internal/component/lg/embed.go` |
| component web | `internal/component/web/assets/` | `go:embed` in `internal/component/web/render.go` |
| REST API docs | `internal/component/api/rest/assets/` | `go:embed` in `internal/component/api/rest/embed.go` |

## Sync

```bash
make ze-sync-vendor-web            # copy from third_party/web/ to every consumer
make generate                      # runs the same sync, with the other generators
make ze-check-vendor-web           # gate: each consumer copy matches its source
make ze-check-vendor-web-updates   # ask the npm registry for newer versions
```

The consumer copies are generated files and they stay tracked in git. `//go:embed`
reads them at compile time, and `make ze-tracked-build-check` compiles what git
holds, so a build with no `make` run must find them.

`make ze-check-vendor-web` is a stage of `make ze-verify` and a prerequisite of
`ze-regen-check-readonly`. It exits non-zero over a copy that differs or is
absent. It queries no registry, so it runs with no network.
