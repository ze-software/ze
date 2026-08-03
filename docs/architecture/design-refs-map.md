# Design Document References Map

Authoritative mapping from source directory to the `// Design:` annotation
that files in that directory should carry. Enforced by
`ai/rules/go-standards.md` (BLOCKING for all `.go` source files
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
| `cmd/ze` | `// Design: docs/architecture/system-architecture.md - ze main entry point` |
| `cmd/ze/bgp` | `// Design: docs/architecture/core-design.md - BGP CLI commands` |
| `cmd/ze/cli` | `// Design: docs/architecture/core-design.md - interactive CLI` |
| `cmd/ze/config` | `// Design: docs/architecture/config/syntax.md - config CLI commands` |
| `cmd/ze/doctor` | `// Design: docs/architecture/system-architecture.md - readiness checks` |
| `cmd/ze/exabgp` | `// Design: docs/architecture/core-design.md - external format bridge CLI` |
| `cmd/ze/hub` | `// Design: docs/architecture/hub-architecture.md - hub startup wiring` |
| `cmd/ze/install` | `// Design: docs/architecture/system-architecture.md - installation and appliance tooling` |
| `cmd/ze/plugin` | `// Design: docs/architecture/api/process-protocol.md - plugin CLI dispatch` |
| `cmd/ze/schema` | `// Design: docs/architecture/config/yang-config-design.md - schema CLI` |
| `cmd/ze/service` | `// Design: docs/architecture/system-architecture.md - service management` |
| `cmd/ze/signal` | `// Design: docs/architecture/behavior/signals.md - signal handling CLI` |
| `cmd/ze/support` | `// Design: docs/architecture/system-architecture.md - support bundle generation` |
| `cmd/ze/ze_chaos_*.go` | `// Design: docs/architecture/chaos-web-dashboard.md - chaos test orchestrator` |
| `internal/test/cli/*.go` | `// Design: docs/architecture/testing/ci-format.md - test runner CLI` |

### `internal/component/`

| Directory | Annotation |
|-----------|------------|
| `internal/component/api` | `// Design: docs/architecture/api/commands.md - REST and gRPC command API` |
| `internal/component/bgp` | `// Design: docs/architecture/core-design.md - BGP subsystem` |
| `internal/core/bgp/attribute` | `// Design: docs/architecture/wire/attributes.md - path attribute encoding` |
| `internal/core/bgp/capability` | `// Design: docs/architecture/wire/capabilities.md - capability negotiation` |
| `internal/core/bgp/context` | `// Design: docs/architecture/encoding-context.md - encoding context` |
| `internal/component/bgp/fsm` | `// Design: docs/architecture/behavior/peer-lifecycle.md - BGP finite state machine` |
| `internal/component/bgp/message` | `// Design: docs/architecture/wire/messages.md - BGP message types` |
| `internal/component/bgp/plugins` | `// Design: docs/plugin-overview.md - BGP plugin implementations` |
| `internal/component/bgp/reactor` | `// Design: docs/architecture/core-design.md - BGP reactor event loop` |
| `internal/component/bgp/wireu` | `// Design: docs/architecture/wire/messages.md - wire UPDATE lazy parsing` |
| `internal/component/cli` | `// Design: docs/architecture/config/yang-config-design.md - unified CLI model` |
| `internal/component/cmd` | `// Design: docs/architecture/api/commands.md - online command handlers` |
| `internal/component/command` | `// Design: docs/architecture/api/commands.md - command dispatch and pipes` |
| `internal/component/config` | `// Design: docs/architecture/config/syntax.md - config parsing and loading` |
| `internal/component/config/yang` | `// Design: docs/architecture/config/yang-config-design.md - YANG schema handling` |
| `internal/component/engine` | `// Design: docs/architecture/system-architecture.md - engine supervisor` |
| `internal/component/firewall` | `// Design: docs/guide/firewall.md - firewall component` |
| `internal/plugins/flowexport` | `// Design: docs/guide/flow-export.md - flow export component` |
| `internal/component/gnmi` | `// Design: docs/guide/gnmi.md - gNMI service` |
| `internal/component/hub` | `// Design: docs/architecture/hub-architecture.md - hub coordination` |
| `internal/component/iface` | `// Design: docs/features/interfaces.md - interface component` |
| `internal/component/ike` | `// Design: docs/config-reference.md - IKEv2 engine` |
| `internal/component/ike/ipsec` | `// Design: docs/config-reference.md - IPsec data model` |
| `internal/component/l2tp` | `// Design: docs/guide/l2tp.md - L2TP subsystem` |
| `internal/plugins/ldp` | `// Design: docs/guide/mpls.md - LDP component` |
| `internal/component/mcp` | `// Design: docs/guide/mcp/overview.md - MCP server` |
| `internal/component/plugin` | `// Design: docs/architecture/api/process-protocol.md - plugin infrastructure` |
| `internal/component/plugin/all` | `// Design: docs/architecture/api/architecture.md - plugin auto-registration` |
| `internal/component/plugin/registry` | `// Design: docs/architecture/api/architecture.md - plugin registry` |
| `internal/plugins/rsvpte` | `// Design: docs/guide/mpls.md - RSVP-TE component` |
| `internal/component/telemetry` | `// Design: docs/guide/monitoring.md - telemetry service` |
| `internal/component/traffic` | `// Design: docs/guide/traffic-control.md - traffic control component` |
| `internal/component/vpp` | `// Design: docs/guide/vpp.md - VPP lifecycle` |
| `internal/component/web` | `// Design: docs/architecture/web-interface.md - web interface` |

### `internal/plugins/`

| Directory | Annotation |
|-----------|------------|
| `internal/component/bfd` | `// Design: docs/guide/bfd.md - BFD plugin` |
| `internal/plugins/connected` | `// Design: docs/guide/redistribution.md - connected route redistribution` |
| `internal/plugins/fib` | `// Design: docs/architecture/core-design.md - FIB plugins` |
| `internal/plugins/firewall` | `// Design: docs/guide/firewall.md - firewall backends` |
| `internal/plugins/iface` | `// Design: docs/features/interfaces.md - interface backends` |
| `internal/plugins/kernel` | `// Design: docs/guide/redistribution.md - kernel route redistribution` |
| `internal/plugins/policyroute` | `// Design: docs/guide/policy-routing.md - policy routing` |
| `internal/plugins/static` | `// Design: docs/guide/static-routes.md - static routes` |
| `internal/component/sysctl` | `// Design: docs/guide/environment-variables.md - kernel tunables` |
| `internal/component/sysrib` | `// Design: docs/architecture/core-design.md - system RIB` |
| `internal/plugins/traffic` | `// Design: docs/guide/traffic-control.md - traffic backends` |

### `internal/core/`

| Directory | Annotation |
|-----------|------------|
| `internal/core/env` | `// Design: docs/architecture/config/environment.md - environment registry` |
| `internal/core/family` | `// Design: docs/architecture/core-design.md - address family registry` |
| `internal/core/health` | `// Design: docs/guide/health-checks.md - health registry` |
| `internal/core/ipc` | `// Design: docs/architecture/api/ipc_protocol.md - IPC framing and dispatch` |
| `internal/core/metrics` | `// Design: docs/plugin-development/metrics.md - metrics registry` |
| `internal/core/report` | `// Design: docs/guide/operational-reports.md - report bus` |
| `internal/core/rib` | `// Design: docs/architecture/route-selection.md - shared RIB structures` |
| `internal/core/routewatch` | `// Design: docs/guide/redistribution.md - kernel route watcher` |
| `internal/core/slogutil` | `// Design: docs/architecture/config/environment.md - structured logging utilities` |
| `internal/core/source` | `// Design: docs/architecture/core-design.md - source registry` |

### Other

| Directory | Annotation |
|-----------|------------|
| `internal/chaos` | `// Design: docs/architecture/chaos-web-dashboard.md - chaos framework` |
| `internal/exabgp` | `// Design: docs/exabgp/exabgp-migration.md - ExaBGP migration and bridge` |
| `internal/test` | `// Design: docs/architecture/testing/ci-format.md - test infrastructure` |
| `pkg/plugin/rpc` | `// Design: docs/architecture/api/ipc_protocol.md - plugin RPC types` |
| `pkg/plugin/sdk` | `// Design: docs/architecture/api/process-protocol.md - plugin SDK` |
| `pkg/zefs` | `// Design: docs/architecture/zefs-format.md - ZeFS storage` |
| `scripts` | `// Design: (none - build tool)` |

## Notes

- The mapping uses longest-prefix match: a file under `internal/component/bgp/plugins/rib/` picks the BGP plugin entry, not the parent BGP subsystem entry.
- Exempt files (per `ai/rules/go-standards.md`): `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`.
- When a directory is added to the source tree, add it here in the same commit, then add the annotation to the new files in the same commit.
- The `// Design: (none - ...)` form is used for directories where no architecture document applies; the parenthesised reason is required.
