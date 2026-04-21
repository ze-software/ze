# Design Document References Map

Authoritative mapping from source directory to the `// Design:` annotation
that files in that directory should carry. Enforced by
`ai/rules/design-doc-references.md` (BLOCKING for all `.go` source files
that are not test, generated, register, embed, or doc files).

This file replaces the `dirMapping` table that lived in the one-shot
`scripts/add-design-refs.go` script. The script has been deleted; this
document is now the source of truth.

Multi-line entries (Design + RFC) appear when a plugin implements a specific
RFC. Both lines belong at the top of every non-exempt file in the directory.

## How to use

When adding a `.go` file in any of these directories, copy the matching
annotation block to the top of the file (after the package clause and before
the imports), then write the file's specific topic in the annotation.

When adding a new directory of source files, add an entry here first, then
add the annotation to the new files.

## Mapping

### `cmd/`

| Directory | Annotation |
|-----------|------------|
| `cmd/ze` | `// Design: docs/architecture/system-architecture.md — ze main entry point` |
| `cmd/ze/bgp` | `// Design: docs/architecture/core-design.md — BGP CLI commands` |
| `cmd/ze/cli` | `// Design: docs/architecture/core-design.md — interactive CLI` |
| `cmd/ze/config` | `// Design: docs/architecture/config/syntax.md — config CLI commands` |
| `cmd/ze/exabgp` | `// Design: docs/architecture/core-design.md — external format bridge CLI` |
| `cmd/ze/hub` | `// Design: docs/architecture/hub-architecture.md — hub CLI entry point` |
| `cmd/ze/plugin` | `// Design: docs/architecture/api/process-protocol.md — plugin CLI dispatch` |
| `cmd/ze/schema` | `// Design: docs/architecture/config/yang-config-design.md — schema CLI` |
| `cmd/ze/signal` | `// Design: docs/architecture/behavior/signals.md — signal handling CLI` |
| `cmd/ze-chaos` | `// Design: docs/architecture/chaos-web-dashboard.md — chaos test orchestrator` |
| `cmd/ze-test` | `// Design: docs/architecture/testing/ci-format.md — test runner CLI` |

### `internal/chaos/`

| Directory | Annotation |
|-----------|------------|
| `internal/chaos/engine` | `// Design: docs/architecture/chaos-web-dashboard.md — chaos action scheduling` |
| `internal/chaos/guard` | `// Design: docs/architecture/chaos-web-dashboard.md — chaos action compatibility guard` |
| `internal/chaos/inprocess` | `// Design: docs/architecture/chaos-web-dashboard.md — in-process chaos runner` |
| `internal/chaos/mocknet` | `// Design: docs/architecture/chaos-web-dashboard.md — mock network for in-process chaos` |
| `internal/chaos/peer` | `// Design: docs/architecture/chaos-web-dashboard.md — BGP peer simulation` |
| `internal/chaos/replay` | `// Design: docs/architecture/chaos-web-dashboard.md — event replay and diff` |
| `internal/chaos/report` | `// Design: docs/architecture/chaos-web-dashboard.md — chaos reporting and metrics` |
| `internal/chaos/route` | `// Design: docs/architecture/chaos-web-dashboard.md — route action scheduling` |
| `internal/chaos/scenario` | `// Design: docs/architecture/chaos-web-dashboard.md — scenario generation` |
| `internal/chaos/shrink` | `// Design: docs/architecture/chaos-web-dashboard.md — test case shrinking` |
| `internal/chaos/validation` | `// Design: docs/architecture/chaos-web-dashboard.md — property-based validation` |
| `internal/chaos/web` | `// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI` |

### `internal/component/`

| Directory | Annotation |
|-----------|------------|
| `internal/component/bgp/attrpool` | `// Design: docs/architecture/pool-architecture.md — attribute and NLRI pools` |
| `internal/component/bgp/config` | `// Design: docs/architecture/config/syntax.md — BGP config extraction and loading` |
| `internal/component/bgp/plugins/cmd/peer` | `// Design: docs/architecture/api/commands.md — BGP peer command handlers` |
| `internal/component/bgp/plugins/cmd/raw` | `// Design: docs/architecture/api/commands.md — BGP raw message command handler` |
| `internal/component/bgp/plugins/cmd/update` | `// Design: docs/architecture/api/commands.md — BGP update command handlers` |
| `internal/component/bgp/store` | `// Design: docs/architecture/pool-architecture.md — attribute and NLRI storage` |
| `internal/component/cli` | `// Design: docs/architecture/config/yang-config-design.md — unified CLI model` |
| `internal/component/cli/testing` | `// Design: docs/architecture/config/yang-config-design.md — editor test infrastructure` |
| `internal/component/cmd/cache` | `// Design: docs/architecture/api/commands.md — BGP cache command handlers` |
| `internal/component/cmd/commit` | `// Design: docs/architecture/api/commands.md — BGP commit command handlers` |
| `internal/component/cmd/meta` | `// Design: docs/architecture/api/commands.md — BGP command discovery and plugin configuration` |
| `internal/component/cmd/subscribe` | `// Design: docs/architecture/api/commands.md — BGP event subscription handlers` |
| `internal/component/config` | `// Design: docs/architecture/config/syntax.md — config parsing and loading` |
| `internal/component/config/env` | `// Design: docs/architecture/config/environment.md — environment variable handling` |
| `internal/component/config/migration` | `// Design: docs/architecture/config/syntax.md — config migration` |
| `internal/component/config/yang` | `// Design: docs/architecture/config/yang-config-design.md — YANG schema handling` |

### `internal/core/`

| Directory | Annotation |
|-----------|------------|
| `internal/core/hub` | `// Design: docs/architecture/hub-architecture.md — hub coordination` |
| `internal/core/ipc` | `// Design: docs/architecture/api/ipc_protocol.md — IPC framing and dispatch` |
| `internal/core/syslog` | `// Design: docs/architecture/testing/ci-format.md — syslog test helpers` |

### `internal/exabgp/`

| Directory | Annotation |
|-----------|------------|
| `internal/exabgp/bridge` | `// Design: docs/architecture/core-design.md — ExaBGP plugin bridge` |
| `internal/exabgp/migration` | `// Design: docs/architecture/core-design.md — ExaBGP config migration` |

### `internal/parse`, `internal/pidfile`, `internal/plugin*`

| Directory | Annotation |
|-----------|------------|
| `internal/parse` | `// Design: docs/architecture/config/syntax.md — parsing helpers` |
| `internal/pidfile` | `// Design: docs/architecture/system-architecture.md — PID file management` |
| `internal/plugin` | `// Design: docs/architecture/api/process-protocol.md — plugin process management` |
| `internal/plugin/all` | `// Design: docs/architecture/api/architecture.md — plugin auto-registration` |
| `internal/plugin/cli` | `// Design: docs/architecture/cli/plugin-modes.md — plugin CLI framework` |
| `internal/plugin/registry` | `// Design: docs/architecture/api/architecture.md — plugin registry` |

### `internal/plugins/bgp/` (BGP engine internals)

| Directory | Annotation |
|-----------|------------|
| `internal/plugins/bgp/attribute` | `// Design: docs/architecture/wire/attributes.md — path attribute encoding` |
| `internal/plugins/bgp/capability` | `// Design: docs/architecture/wire/capabilities.md — capability negotiation` |
| `internal/plugins/bgp/commit` | `// Design: docs/architecture/update-building.md — commit management` |
| `internal/plugins/bgp/context` | `// Design: docs/architecture/encoding-context.md — encoding context` |
| `internal/plugins/bgp/filter` | `// Design: docs/architecture/core-design.md — route filtering` |
| `internal/plugins/bgp/format` | `// Design: docs/architecture/api/json-format.md — message formatting` |
| `internal/plugins/bgp/fsm` | `// Design: docs/architecture/behavior/fsm.md — BGP finite state machine` |
| `internal/plugins/bgp/message` | `// Design: docs/architecture/wire/messages.md — BGP message types` |
| `internal/plugins/bgp/nlri` | `// Design: docs/architecture/wire/nlri.md — NLRI encoding and decoding` |
| `internal/plugins/bgp/reactor` | `// Design: docs/architecture/core-design.md — BGP reactor event loop` |
| `internal/plugins/bgp/rib` | `// Design: docs/architecture/pool-architecture.md — RIB wire storage` |
| `internal/plugins/bgp/route` | `// Design: docs/architecture/route-types.md — route definitions` |
| `internal/plugins/bgp/server` | `// Design: docs/architecture/core-design.md — BGP server events and hooks` |
| `internal/plugins/bgp/types` | `// Design: docs/architecture/core-design.md — shared BGP types` |
| `internal/plugins/bgp/wire` | `// Design: docs/architecture/wire/buffer-writer.md — wire buffer utilities` |
| `internal/plugins/bgp/wireu` | `// Design: docs/architecture/wire/messages.md — wire UPDATE lazy parsing` |

### `internal/plugins/bgp-*` (named BGP plugins)

| Directory | Annotation(s) |
|-----------|---------------|
| `internal/plugins/bgp-gr` | `// Design: docs/architecture/core-design.md — graceful restart plugin`<br>`// RFC: rfc/short/rfc4724.md` |
| `internal/plugins/bgp-hostname` | `// Design: docs/architecture/core-design.md — hostname capability plugin` |
| `internal/plugins/bgp-llnh` | `// Design: docs/architecture/core-design.md — link-local next-hop plugin`<br>`// RFC: rfc/short/rfc5549.md` |
| `internal/plugins/bgp-rib` | `// Design: docs/architecture/plugin/rib-storage-design.md — RIB plugin` |
| `internal/plugins/bgp-rib/pool` | `// Design: docs/architecture/pool-architecture.md — per-attribute pool instances` |
| `internal/plugins/bgp-rib/storage` | `// Design: docs/architecture/plugin/rib-storage-design.md — RIB storage internals` |
| `internal/plugins/bgp-role` | `// Design: docs/architecture/core-design.md — BGP role plugin`<br>`// RFC: rfc/short/rfc9234.md` |
| `internal/plugins/bgp-rs` | `// Design: docs/architecture/core-design.md — route server plugin` |
| `internal/plugins/bgp-nlri-evpn` | `// Design: docs/architecture/wire/nlri-evpn.md — EVPN NLRI plugin`<br>`// RFC: rfc/short/rfc7432.md` |
| `internal/plugins/bgp-nlri-flowspec` | `// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec NLRI plugin`<br>`// RFC: rfc/short/rfc5575.md` |
| `internal/plugins/bgp-nlri-labeled` | `// Design: docs/architecture/wire/nlri.md — labeled unicast NLRI plugin`<br>`// RFC: rfc/short/rfc8277.md` |
| `internal/plugins/bgp-nlri-ls` | `// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS NLRI plugin`<br>`// RFC: rfc/short/rfc7752.md` |
| `internal/plugins/bgp-nlri-mup` | `// Design: docs/architecture/wire/nlri.md — MUP NLRI plugin`<br>`// RFC: rfc/short/draft-ietf-bess-mup-safi.md` |
| `internal/plugins/bgp-nlri-mvpn` | `// Design: docs/architecture/wire/nlri.md — MVPN NLRI plugin` |
| `internal/plugins/bgp-nlri-rtc` | `// Design: docs/architecture/wire/nlri.md — route target constraint plugin`<br>`// RFC: rfc/short/rfc4684.md` |
| `internal/plugins/bgp-nlri-vpls` | `// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin`<br>`// RFC: rfc/short/rfc4761.md` |
| `internal/plugins/bgp-nlri-vpn` | `// Design: docs/architecture/wire/nlri.md — VPN NLRI plugin`<br>`// RFC: rfc/short/rfc4364.md` |

### `internal/selector`, `sim`, `slogutil`, `source`

| Directory | Annotation |
|-----------|------------|
| `internal/selector` | `// Design: docs/architecture/core-design.md — peer selector` |
| `internal/sim` | `// Design: docs/architecture/chaos-web-dashboard.md — simulation infrastructure` |
| `internal/slogutil` | `// Design: docs/architecture/config/environment.md — structured logging utilities` |
| `internal/source` | `// Design: docs/architecture/core-design.md — source registry` |

### `internal/test/`

| Directory | Annotation |
|-----------|------------|
| `internal/test` | `// Design: docs/architecture/testing/ci-format.md — test infrastructure` |
| `internal/test/ci` | `// Design: docs/architecture/testing/ci-format.md — CI test format parsing` |
| `internal/test/decode` | `// Design: docs/architecture/testing/ci-format.md — decode test helpers` |
| `internal/test/peer` | `// Design: docs/architecture/testing/ci-format.md — test BGP peer` |
| `internal/test/runner` | `// Design: docs/architecture/testing/ci-format.md — test runner framework` |
| `internal/test/tmpfs` | `// Design: docs/architecture/system-architecture.md — temporary filesystem management` |

### `pkg/`

| Directory | Annotation |
|-----------|------------|
| `pkg/plugin` | `// Design: docs/architecture/api/process-protocol.md — plugin package` |
| `pkg/plugin/rpc` | `// Design: docs/architecture/api/ipc_protocol.md — plugin RPC types` |
| `pkg/plugin/sdk` | `// Design: docs/architecture/api/process-protocol.md — plugin SDK` |

### Other

| Directory | Annotation |
|-----------|------------|
| `research` | `// Design: (none — research tool)` |
| `scripts` | `// Design: (none — build tool)` |

## Notes

- The mapping uses longest-prefix match: a file in `internal/plugins/bgp-rib/storage` picks the storage entry, not the parent `bgp-rib` entry.
- Exempt files (per `ai/rules/design-doc-references.md`): `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`.
- When a directory is added to the source tree, add it here in the same commit, then add the annotation to the new files in the same commit.
- The `// Design: (none — ...)` form is used for directories where no architecture document applies; the parenthesised reason is required.
