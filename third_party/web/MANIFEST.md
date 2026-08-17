# Vendored Web Assets

Third-party web assets used by Ze's web interfaces.
Source of truth: files in this directory. Consumer copies are synced via `scripts/vendor/sync_web.go`.

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
(`scripts/codegen/web_assets.go`).

## Vendor directories

A directory under `third_party/web/` is the unit a consumer subscribes to. A
consumer that holds one file of a directory MUST hold every file of that
directory, and `scripts/vendor/check_web.go` reads the two trees that way. An
asset for one consumer only gets its own directory, as `swagger-ui/` does. A
directory that reaches no consumer is a problem too: it says the sync was never
told to copy it.

## Vendored tools (not served, not synced)

A FILE at the top level of `third_party/web/` is a third-party tool, not an
asset. `scripts/vendor/check_web.go` reads directories only, so a top-level file
subscribes no consumer and drifts nowhere.

| Tool | Version | Source | Vendored |
|------|---------|--------|----------|
| htmx-upgrade-check.py | 4.0.0-beta6 | https://unpkg.com/htmx.org@4.0.0-beta6/dist/scripts/upgrade-check.py | 2026-08-15 |

`htmx-upgrade-check.py` is htmx's own htmx 2 to htmx 4 scanner, byte-identical
to the file in the npm package. It builds a DOM, so it reports the inheritance
carriers a text search cannot see. `make ze-htmx-upgrade-check` runs it through
`scripts/dev/htmx_upgrade_check.py`, which derives the packages to scan from the
Consumers table below: every `assets/` directory holding an htmx core file.

It is vendored rather than fetched at gate time for the reason every other file
here is: a gate that downloads its own judge cannot run offline, and cannot be
reproduced from what git holds. The beta5 and beta6 scanners differ, so the
version above is the version the cutover ships.

## Consumers

| Consumer | Path | Embed |
|----------|------|-------|
| chaos web | `internal/chaos/web/assets/` | `go:embed` in `internal/chaos/web/handlers.go` |
| looking glass | `internal/component/lg/assets/` | `go:embed` in `internal/component/lg/embed.go` |
| component web | `internal/component/web/assets/` | `go:embed` in `internal/component/web/render.go` |
| REST API docs | `internal/component/api/rest/assets/` | `go:embed` in `internal/component/api/rest/embed.go` |

## Sync

```bash
make ze-vendor-web-sync            # copy from third_party/web/ to every consumer
make generate                      # runs the same sync, with the other generators
make ze-vendor-web-check           # gate: each consumer copy matches its source
make ze-vendor-web-update-report   # ask the npm registry for newer versions
make ze-htmx-upgrade-check         # gate: no unexplained htmx 4 upgrade issue
make ze-htmx-upgrade-report        # print every htmx 4 upgrade issue
```

The consumer copies are generated files and they stay tracked in git. `//go:embed`
reads them at compile time, and `make ze-repository-tracked-build-check` compiles what git
holds, so a build with no `make` run must find them.

`make ze-vendor-web-check` is a stage of `make ze-precommit-verify` and a
prerequisite of `ze-generated-files-check`. It exits non-zero when a copy
differs or is absent. It queries no registry, so it runs with no network.
