# 804 -- gNMI YANG Config, CLI, Metrics, Docs

## Context

The gNMI component (learned/803) was implemented with env-var-only configuration, no CLI
visibility, no Prometheus counters, and no documentation. Operators could enable gNMI
but had no YANG config schema, no `show gnmi` command, no metrics to monitor it, and
external config commits (web/CLI) did not notify gNMI Subscribe STREAM subscribers.

## Decisions

- Chose separate YANG module `ze-gnmi-conf` under `internal/component/gnmi/schema/` over
  augmenting the hub or api-server schema, because gNMI is its own component with
  independent lifecycle, following the web/mcp/lg pattern.
- Chose `show gnmi` over `show service gnmi` because no `show service` container exists
  in the CLI YANG and components use `show <component>` (show bgp, show vpn, show bmp).
- Chose wrapping `reloadAfterCommit` to chain `NotifyConfigReload()` over adding a second
  hook slot to ConfigSessionManager, because the closure captures the `gnmiNotifier` variable
  by reference and requires no API changes.
- Chose generic "config-reload" notification for external commits over per-path diffs,
  because fine-grained diffs would require config tree diffing infrastructure that does not exist.
- Added TLS cert/key as file path env vars (`ze.gnmi.tls.cert`, `ze.gnmi.tls.key`) with
  file read at startup, matching the MCP TLS pattern.

## Consequences

- gNMI is now fully YANG-modeled: `environment { gnmi { enabled; token; tls; server } }`.
- `show gnmi` provides runtime status (enabled, address, auth, TLS, subscriber count).
- Three Prometheus counters enable monitoring: `ze_gnmi_requests_total{rpc}`,
  `ze_gnmi_subscribe_active`, `ze_gnmi_errors_total{rpc,code}`.
- External config commits from any surface (web, CLI, managed) now notify STREAM subscribers.
- TLS cert/key loaded at startup only; cert rotation requires restart (same as web/mcp).

## Gotchas

- goyang `uses` groupings are not expanded in the raw Module AST (`GetModule`). The
  resolved entry tree (`GetEntry`) with `Dir` map must be used to verify listener leaves.
- `plugin.ResponseData` interface requires embedding `plugin.DataMarker` in any struct
  returned via `plugin.Response.Data`.
- gocritic `typeDefFirst` requires type definitions to appear before their methods in the
  same file; adding methods on Server before the struct definition triggers lint failure.
- The gnmi schema package must be blank-imported in `plugin/all/all.go` and config test
  files for the `init()` registration to run in the binary.

## Files

- `internal/component/gnmi/schema/` -- YANG module, embed, register, tests
- `internal/component/gnmi/metrics.go` -- gnmiMetrics struct and init
- `internal/component/gnmi/server.go` -- ServerStatus, Status(), SetMetricsRegistry, RegisterGlobal, LookupServer, recordError
- `internal/component/gnmi/subscribe.go` -- NotifyConfigReload, subscribe gauge instrumentation
- `internal/component/gnmi/capabilities.go`, `get.go`, `set.go` -- request/error counter instrumentation
- `internal/component/cmd/show/gnmi.go` -- show gnmi handler
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- show gnmi YANG entry
- `internal/component/config/environment.go` -- ze.gnmi.tls.cert/key env vars
- `cmd/ze/hub/main.go` -- TLS file loading, gnmiNotifier commit hook wiring, RegisterGlobal
- `docs/guide/gnmi.md`, `docs/features.md`, `docs/comparison.md`, `docs/architecture/api/architecture.md`
