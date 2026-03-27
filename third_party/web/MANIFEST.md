# Vendored Web Assets

Third-party web assets used by Ze's web interfaces.
Source of truth: files in this directory. Consumer copies are synced via `scripts/sync-vendor-web.sh`.

## Assets

| Asset | Version | Source | Vendored |
|-------|---------|--------|----------|
| htmx.min.js | 2.0.4 | https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js | 2026-03-27 |
| sse.js | 2.0.4 (htmx ext) | https://unpkg.com/htmx-ext-sse@2.0.4/sse.js | 2026-03-27 |

## Consumers

| Consumer | Path | Embed |
|----------|------|-------|
| chaos web | `internal/chaos/web/assets/` | `go:embed` in `internal/chaos/web/server.go` |
| component web | `internal/component/web/assets/` | `go:embed` in `internal/component/web/render.go` |

## Sync

```bash
scripts/sync-vendor-web.sh        # copy from third_party/web/ to all consumer directories
scripts/check-vendor-web.sh       # check npm registry for newer versions
make ze-sync-vendor-web            # make target for sync
```
