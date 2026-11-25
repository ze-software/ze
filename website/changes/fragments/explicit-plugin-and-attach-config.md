---
spec: explicit-plugin-and-attach-config
topic: Plugins
---

## Before

- Operators declared plugins in a top-level `plugin { internal ...; external ...; hub ...; }` block and wired BGP peers to plugins one peer at a time using `attach process <name> { receive [ ... ]; send [ ... ]; }`, with silence as the default until every peer explicitly attached every plugin.
- ExaBGP scripts ran through a separate `exabgp { bridge { process ...; } }` configuration root and an in-process bridge runner with per-peer `feed` blocks; a script exit relied on a bridge-owned ledger rather than per-participant route ownership, and two scripts declaring the same capability conflicted at startup.
- Any plugin process — including a forked third-party external program — dispatched CLI commands under a reserved internal identity prefix that bypassed authorization profiles, and could invoke `route-install`, `route-remove`, `inject-wire-route`, or `emit-event` without declaring permission.
- Built-in defaults and derived peer/plugin bindings were hidden from `show config running`, and `ze config validate` with no daemon running refused configuration sections defined by external plugins without asking the plugin binary for its declaration.
- Renaming a compiled-in plugin block caused state-store key grants, `show doctor` snapshots, and `ze data rm` live-owner checks to miss the plugin because they keyed on the configured alias instead of the compiled registration identity.

## After

- One `bus { endpoint <name> { ... }; participant <path> { ... }; hub { ... }; }` block replaces `plugin { }`, `attach process`, and `exabgp { bridge }` across Ze. The managed hub listener service is now `bus-hub`.
- Every bus participant (BGP peers, groups, plugins, and external programs) registers the messages it sends and receives; with nothing written, those registered defaults run, and operators override them near the emitter or centrally via direction-grouped `bus { send { allow; to; message <name> { to; tag; } }; receive { allow; from; message <name> { from; tag; } }; }` blocks using short slash paths (`/bgp/peer/<name>`, `/bgp/group/<group>`, `/bgp/ip/<address>`, `/bus/endpoint/<name>`) and explicit `none` for empty lists.
- Route servers (`bgp-rs`) limit forwarding to `session rs-client` peers and route reflectors (`bgp-rr`) limit reflection targets to internal peers, allowing both to run on one router with nothing written and no double replay.
- Every engine RPC declares the bus message it carries (or an explicit exemption), and every plugin identity is authorized against a compiled command-prefix profile in `authz` rather than a prefix bypass; undeclared or disallowed calls are refused, counted under one bus refusal counter, and reported via `show doctor`.
- `show config running | implied`, `show config source <file> | implied`, `show config dump <file> | implied`, editor `show | implied`, and the Web UI display every implied endpoint, effective bus filter, replaced default, and auto-inserted filter-chain member alongside written configuration, while `show event delivery` shows edge origins (`written`, `inherited`, `registered`, `required`, `dormant`).
- Offline `ze config validate` and `show config source <file> | implied` query declared external programs in declare mode (`ZE_PLUGIN_MODE=declare`) without starting their run loops, failing closed if a program cannot be queried.
- ExaBGP scripts run as ordinary external bus endpoints via `ze exabgp plugin` (with `ze exabgp migrate` emitting `bus { endpoint ... }` blocks and virtual-session capability flags); the engine withdraws a departed participant's originated routes by default unless `external { on-exit keep; }` is configured, and identical virtual-session capabilities declared by multiple endpoints deduplicate cleanly on the wire.
- Compiled-in plugins keep their compiled registration identity under any endpoint alias and under `process separate`, fixing state-store access, `show doctor` reporting, and `ze data rm` protection.

<!-- source: internal/component/plugin/yang/ze-plugin-conf.yang -- container bus, grouping participant-bus -->
<!-- source: internal/component/plugin/types.go -- PluginConfig.CompiledIdentity, PluginConfig.RunsSeparateProcess -->
<!-- source: internal/component/plugin/server/bus_filter.go -- CompileBus, CompiledBus.ResolveAuthenticatedSender -->
<!-- source: internal/component/plugin/server/bus_op_gate.go -- checkEngineOpBusSend, checkEngineOpBusSendCLI -->
<!-- source: internal/component/authz/authz.go -- Store.authorize, authorizePluginIdentity -->
<!-- source: internal/component/plugin/server/dispatch_state.go -- stateInput -->
<!-- source: internal/component/config/declared_schemas.go -- PublishDeclaredSchemas -->
<!-- source: internal/component/bgp/reactor/origin_runtime.go -- EndOriginOwner -->

## Corrects

none

## Exposure

Deployments running external plugins (`plugin { external ... }`) were exposed to privilege escalation: an external plugin could dispatch arbitrary CLI commands bypassing authorization profiles via the reserved internal prefix and invoke system-RIB route installation or raw BGP wire injection without declaring those messages. Deployments with no external plugins were not exposed to third-party command escalation.

## Upstream

none

## Asked by

none
