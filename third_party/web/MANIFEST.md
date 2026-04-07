# Vendored Web Assets

Third-party web assets used by Ze's web interfaces.
Source of truth: files in this directory. Consumer copies are synced via `scripts/vendor/sync_web.go`.

## Assets

| Asset | Version | Source | Vendored |
|-------|---------|--------|----------|
| htmx.min.js | 2.0.4 | https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js | 2026-03-27 |
| sse.js | 2.0.4 (htmx ext) | https://unpkg.com/htmx-ext-sse@2.0.4/sse.js | 2026-03-27 |
| ze.svg | - | `docs/logo/ze.svg` (project logo with Exa gradient) | 2026-03-31 |

## Consumers

| Consumer | Path | Embed |
|----------|------|-------|
| chaos web | `internal/chaos/web/assets/` | `go:embed` in `internal/chaos/web/handlers.go` |
| looking glass | `internal/component/lg/assets/` | `go:embed` in `internal/component/lg/embed.go` |
| component web | `internal/component/web/assets/` | `go:embed` in `internal/component/web/render.go` |

## Sync

```bash
scripts/vendor/sync_web.go         # copy from third_party/web/ to all consumer directories
scripts/vendor/check_web.go        # check npm registry for newer versions
make ze-sync-vendor-web            # make target for sync
```
