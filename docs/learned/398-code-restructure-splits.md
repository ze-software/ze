# 398 -- Code Restructure Splits

## Objective

Split 6 large Go files (reactor.go 5390L, decode.go 1928L, server.go 1425L, loader.go 1689L, peer.go 2679L, ze-chaos main.go 1296L) into smaller single-responsibility files within their packages.

## Decisions

- Splits were executed as part of a broader path restructuring (`internal/plugins/bgp/` to `internal/component/bgp/`, `internal/plugin/` to `internal/component/plugin/`) rather than as isolated file splits
- server.go was promoted to its own subdirectory (`internal/component/plugin/server/`) with 20+ focused files (command.go, dispatch.go, startup.go, subscribe.go, etc.)
- reactor.go was split into reactor_api.go, reactor_api_batch.go, reactor_api_forward.go, reactor_wire.go, reactor_peers.go, reactor_notify.go, reactor_connection.go, reactor_metrics.go
- peer.go was split into peer.go + peer_send.go, peer_stats.go, peer_connection.go, peer_static_routes.go, peer_rib_routes.go, peer_initial_sync.go
- loader.go content moved to `internal/component/bgp/config/` with routeattr.go, routeattr_community.go, flowspec.go, mup.go splits
- decode.go shrunk to 232L with decode_plugin.go (433L) alongside
- ze-chaos main.go reduced to 596L (under 600L threshold)

## Patterns

- Large file splits are more effective when combined with path restructuring since both address structural concerns
- Promoting a file to a subdirectory (server.go to server/) is appropriate when the split would produce many files with a shared prefix
- Incremental splits across multiple specs (visible in learned 221, 244, 247, 351, 375) can achieve the same goal as a dedicated split spec

## Gotchas

- The spec was never formally executed as written because the broader restructuring (`internal/plugins/` to `internal/component/`) accomplished the same splits with different target filenames
- reactor.go remains at 945L (above the 600L target) but the concern separation is done -- the remaining code is the core reactor lifecycle which is a single concern
- The spec referenced a companion `code-restructure.md` report with detailed declaration inventories that became stale once the path restructuring happened

## Files

- `internal/component/bgp/reactor/reactor*.go` -- split reactor (was `internal/plugins/bgp/reactor/reactor.go`)
- `internal/component/bgp/reactor/peer*.go` -- split peer (was `internal/plugins/bgp/reactor/peer.go`)
- `internal/component/plugin/server/` -- split server (was `internal/plugin/server.go`)
- `internal/component/bgp/config/routeattr*.go` -- split loader routes (was `internal/config/loader.go`)
- `cmd/ze/bgp/decode.go`, `decode_plugin.go` -- split decode
- `cmd/ze-chaos/main.go` -- reduced to 596L
