# Ze Documentation Index

## I Want To...

| Task | Read first | Then |
|------|-----------|------|
| Understand the modular core | `patterns/registration.md` | `docs/architecture/core-design.md` |
| Add a CLI command | `patterns/cli-command.md` | `rules/cli-patterns.md` |
| Add a web page/endpoint | `patterns/web-endpoint.md` | `docs/architecture/web-interface.md` |
| Create a plugin | `patterns/plugin.md` | `rules/plugin-design.md` |
| Add a config option | `patterns/config-option.md` | `rules/config-design.md` |
| Add a .ci functional test | `patterns/functional-test.md` | `docs/architecture/testing/ci-format.md` |
| Modify wire encoding | `rules/buffer-first.md` | `docs/architecture/buffer-architecture.md` |
| Add route processing | `rules/architecture-summary.md` | `docs/architecture/core-design.md` |
| Add NLRI family support | `patterns/plugin.md` (NLRI codec section) | `docs/architecture/wire/nlri.md` |
| Add an attribute | `rules/buffer-first.md` | `docs/architecture/wire/attributes.md` |
| Add a capability | `patterns/plugin.md` (capabilities section) | `docs/architecture/wire/capabilities.md` |
| Implement an RFC | `rules/rfc-compliance.md` | `docs/contributing/rfc-implementation-guide.md` |
| Write a spec | `rules/planning.md` | `plan/TEMPLATE.md` |

## Dev Tools

| Tool | Location | Purpose |
|------|----------|---------|
| `go_extract.go` | `scripts/dev/` | Move Go symbols between files |
| `replace.py` | `scripts/dev/` | Bulk find-and-replace with diff preview (run without `--apply` to review, then `--apply` to write). Supports `--regex` and `--all`. |

## Pattern Cookbooks

Mechanical recipes for creating common artifacts. Read before coding.

| Pattern | File | What it covers |
|---------|------|---------------|
| **Registration** | `patterns/registration.md` | **All registries, startup flow, modular core architecture** |
| CLI Command | `patterns/cli-command.md` | Offline/online dispatch, grammar, YANG tree, exit codes |
| Web Endpoint | `patterns/web-endpoint.md` | Handler sequence, templates, HTMX OOB, route registration |
| Plugin | `patterns/plugin.md` | register.go, logger, SDK protocol, filters, codecs |
| Config Option | `patterns/config-option.md` | YANG leaf, env var, validator, naming across layers |
| Functional Test | `patterns/functional-test.md` | .ci format, test directories, templates, expectations |

## Learned Summaries (Curated)

Structural decisions, patterns, and gotchas extracted from 500+ completed specs.
Full index: `.claude/LEARNED-INDEX.md`. All summaries: `plan/learned/`.

## Architecture Docs

| Area | Doc |
|------|-----|
| **Core Design** | `docs/architecture/core-design.md` **(START HERE)** |
| **System Architecture** | `docs/architecture/system-architecture.md` |
| **Overview** | `docs/architecture/overview.md` |
| **Hub Architecture** | `docs/architecture/hub-architecture.md` |
| Buffer-first | `docs/architecture/buffer-architecture.md` |
| Message buffers | `docs/architecture/message-buffer-design.md` |
| Wire formats | `docs/architecture/wire/messages.md` |
| NLRI types | `docs/architecture/wire/nlri.md` |
| NLRI BGP-LS | `docs/architecture/wire/nlri-bgpls.md` |
| NLRI EVPN | `docs/architecture/wire/nlri-evpn.md` |
| NLRI FlowSpec | `docs/architecture/wire/nlri-flowspec.md` |
| NLRI qualifiers | `docs/architecture/wire/qualifiers.md` |
| MP NLRI ordering | `docs/architecture/wire/mp-nlri-ordering.md` |
| UPDATE packing | `docs/architecture/wire/update-packing.md` |
| Buffer writer | `docs/architecture/wire/buffer-writer.md` |
| Attributes | `docs/architecture/wire/attributes.md` |
| BGP-LS attr naming | `docs/architecture/wire/bgpls-attribute-naming.md` |
| Capabilities | `docs/architecture/wire/capabilities.md` |
| UPDATE building | `docs/architecture/update-building.md` |
| UPDATE cache | `docs/architecture/update-cache.md` |
| UPDATE density | `docs/architecture/update-density-analysis.md` |
| Memory pools | `docs/architecture/pool-architecture.md` |
| Pool review | `docs/architecture/pool-architecture-review.md` |
| Zero-copy | `docs/architecture/encoding-context.md` |
| RIB transition | `docs/architecture/rib-transition.md` |
| RIB storage | `docs/architecture/plugin/rib-storage-design.md` |
| Route types | `docs/architecture/route-types.md` |
| Route selection | `docs/architecture/route-selection.md` |
| FSM | `docs/architecture/behavior/fsm.md` |
| Signals | `docs/architecture/behavior/signals.md` |
| API | `docs/architecture/api/architecture.md` |
| API Capabilities | `docs/architecture/api/capability-contract.md` |
| API Commands | `docs/architecture/api/commands.md` |
| API JSON format | `docs/architecture/api/json-format.md` |
| IPC protocol | `docs/architecture/api/ipc_protocol.md` |
| Process protocol | `docs/architecture/api/process-protocol.md` |
| MuxConn wire format | `docs/architecture/api/wire-format.md` |
| UPDATE syntax | `docs/architecture/api/update-syntax.md` |
| Text format | `docs/architecture/api/text-format.md` |
| Text parser | `docs/architecture/api/text-parser.md` |
| Text coverage | `docs/architecture/api/text-coverage.md` |
| Config syntax | `docs/architecture/config/syntax.md` |
| Config environment | `docs/architecture/config/environment.md` |
| Environment block | `docs/architecture/config/environment-block.md` |
| Config tokenizer | `docs/architecture/config/tokenizer.md` |
| YANG design | `docs/architecture/config/yang-config-design.md` |
| ExaBGP syntax | `docs/architecture/config/exabgp-syntax.md` |
| VyOS research | `docs/architecture/config/vyos-research.md` |
| Plugin modes | `docs/architecture/cli/plugin-modes.md` |
| Plugin testing | `docs/architecture/debugging/plugin-testing.md` |
| Edge: ASN4 | `docs/architecture/edge-cases/as4.md` |
| Edge: ADD-PATH | `docs/architecture/edge-cases/addpath.md` |
| Edge: Extended msg | `docs/architecture/edge-cases/extended-message.md` |
| Route metadata | `docs/architecture/meta/README.md` |
| Role metadata | `docs/architecture/meta/role.md` |
| Forward pool | `docs/architecture/forward-congestion-pool.md` |
| Congestion industry | `docs/architecture/congestion-industry.md` |
| Subsystem wiring | `docs/architecture/subsystem-wiring.md` |
| Plugin mgr wiring | `docs/architecture/plugin-manager-wiring.md` |
| Hub API commands | `docs/architecture/hub-api-commands.md` |
| RFC MAY decisions | `docs/architecture/rfc-may-decisions.md` |
| ZeFS format | `docs/architecture/zefs-format.md` |
| Fleet config | `docs/architecture/fleet-config.md` |
| Web interface | `docs/architecture/web-interface.md` |
| Web components | `docs/architecture/web-components.md` |
| Chaos dashboard | `docs/architecture/chaos-web-dashboard.md` |
| CI format | `docs/architecture/testing/ci-format.md` |
| Interop testing | `docs/architecture/testing/interop.md` |
| ExaBGP mapping | `docs/exabgp/exabgp-code-map.md` |
| ExaBGP compat | `docs/exabgp/exabgp-differences.md` |

## Keyword → Architecture Doc

| Keywords | Docs |
|----------|------|
| buffer, iterator, parse, wire | `core-design.md`, `buffer-architecture.md`, `rules/buffer-first.md` |
| encode, Pack, WriteTo, alloc | `rules/buffer-first.md`, `buffer-architecture.md` |
| UPDATE, message, build, route | `core-design.md`, `update-building.md`, `encoding-context.md` |
| attribute, AS_PATH, NEXT_HOP, MED | `core-design.md`, `wire/attributes.md`, `update-building.md` |
| community, ext community, large community | `wire/attributes.md` |
| NLRI, prefix, MP_REACH, MP_UNREACH | `core-design.md`, `wire/nlri.md` |
| multiprotocol, AFI, SAFI | `wire/nlri.md`, `wire/capabilities.md` |
| capability, OPEN, negotiate | `wire/capabilities.md` |
| pool, memory, dedup, zero-copy | `core-design.md`, `pool-architecture.md`, `encoding-context.md` |
| forward, reflect, wire cache | `core-design.md`, `encoding-context.md`, `update-building.md` |
| route, rib, storage | `core-design.md`, `route-types.md`, `rib-transition.md`, `plugin/rib-storage-design.md` |
| route selection, best path | `route-selection.md` |
| FSM, state, session, peer | `behavior/fsm.md` |
| signal, SIGHUP, SIGUSR | `behavior/signals.md` |
| API, command, announce, withdraw | `api/architecture.md`, `api/capability-contract.md`, `api/commands.md` |
| text format, IPC, formatter, parser | `api/text-format.md`, `api/text-parser.md`, `api/text-coverage.md` |
| IPC, wire format, muxconn | `api/ipc_protocol.md`, `api/wire-format.md`, `api/process-protocol.md` |
| JSON, event format | `api/json-format.md` |
| config, load | `config/syntax.md`, `config/tokenizer.md` |
| environment, env vars | `config/environment.md`, `config/environment-block.md` |
| web, dashboard, UI | `web-interface.md`, `web-components.md`, `chaos-web-dashboard.md` |
| subsystem, wiring, plugin manager | `subsystem-wiring.md`, `plugin-manager-wiring.md` |
| forward pool, congestion | `forward-congestion-pool.md`, `congestion-industry.md` |
| hub, API commands | `hub-architecture.md`, `hub-api-commands.md` |
| cache, update cache | `update-cache.md`, `update-density-analysis.md` |
| metadata, route meta | `meta/README.md` |
| interop, test infra | `testing/interop.md`, `testing/ci-format.md` |
| zefs, blob, netcapstring, storage | `zefs-format.md`, `fleet-config.md` |
| fleet, managed, server, backup, bootstrap | `fleet-config.md` |
| FlowSpec | `wire/nlri.md`, `wire/nlri-flowspec.md` |
| VPN, L3VPN, MPLS-VPN, 6PE | `wire/nlri.md` |
| EVPN, MAC-IP | `wire/nlri.md`, `wire/nlri-evpn.md` |
| BGP-LS, link-state | `wire/nlri-bgpls.md`, `wire/bgpls-attribute-naming.md` |
| ExaBGP | `exabgp/exabgp-code-map.md`, `exabgp/exabgp-differences.md` |
| ASN4, AS4 | `edge-cases/as4.md` |
| ADD-PATH | `edge-cases/addpath.md` |
| extended message | `edge-cases/extended-message.md` |
| test, functional, .ci | `docs/functional-tests.md` (top-level, not architecture/), `testing/ci-format.md` |

All architecture docs in `docs/architecture/` unless noted.

## Keyword → RFC

| Keywords | Primary RFC | Related |
|----------|-------------|---------|
| open, capability | `rfc5492` | `rfc9072` |
| update, nlri, prefix | `rfc4271` | `rfc4760` |
| multiprotocol, mp-bgp | `rfc4760` | |
| notification, error | `rfc4271` | `rfc7606`, `rfc9003` |
| route-refresh | `rfc2918` | `rfc7313` |
| community | `rfc1997` | |
| extended community, RT | `rfc4360` | `rfc5701` |
| large community | `rfc8092` | `rfc8195` |
| 4-byte AS, ASN4 | `rfc6793` | |
| add-path | `rfc7911` | |
| graceful restart | `rfc4724` | |
| extended message | `rfc8654` | |
| label, mpls | `rfc8277` | `rfc3032` |
| vpn, l3vpn, 6pe | `rfc4364` | `rfc4659`, `rfc4798` |
| flowspec | `rfc8955` | `rfc8956` |
| evpn | `rfc7432` | `rfc9136` |
| vpls | `rfc4761` | `rfc4762` |
| bgp-ls | `rfc7752` | `rfc9085`, `rfc9514` |
| role, otc | `rfc9234` | |
| ipv6 next hop | `rfc8950` | |
| shutdown | `rfc9003` | `rfc8203` |
| treat-as-withdraw | `rfc7606` | |

RFC summaries: `rfc/short/`. Full RFCs: `rfc/full/`.

## Session State

Per-session: `tmp/session/session-state-<spec-stem>-<SID>.md` (gitignored). Each session gets its own file.
Session markers: `tmp/session/.session-<ID>` map sessions to specs. See `hooks/lib/state-file.sh`.
On startup, `_find_latest_state_for_spec()` finds the most recent state file for a spec from any previous session.
