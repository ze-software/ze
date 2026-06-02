# Ze Documentation Index

## I Want To...

| Task | Read first | Then |
|------|-----------|------|
| Understand the modular core | `patterns/registration.md` | `docs/architecture/core-design.md` |
| Add a CLI command | `patterns/cli-command.md` | `rules/cli-patterns.md` |
| Add a web page/endpoint | `patterns/web-endpoint.md` | `docs/architecture/web-interface.md` |
| Create a plugin | `patterns/plugin.md` | `rules/plugin-design.md` |
| Keep a plugin self-contained (removal test) | `rules/plugin-self-containment.md` | Remove the plugin and ALL its features vanish; other plugins and core keep working |
| Add a config option | `patterns/config-option.md` | `rules/config-design.md` |
| Add a .ci functional test | `patterns/functional-test.md` | `docs/architecture/testing/ci-format.md` |
| Test linux-only code (QEMU) | `rules/qemu-testing.md` | `rules/testing.md` (Linux-Only Tests section) |
| Modify wire encoding | `rules/buffer-first.md` | `docs/architecture/buffer-architecture.md` |
| Add route processing | `rules/architecture-summary.md` | `docs/architecture/core-design.md` |
| Add NLRI family support | `patterns/plugin.md` (NLRI codec section) | `docs/architecture/wire/nlri.md` |
| Add an attribute | `rules/buffer-first.md` | `docs/architecture/wire/attributes.md` |
| Add a capability | `patterns/plugin.md` (capabilities section) | `docs/architecture/wire/capabilities.md` |
| Implement an RFC | `rules/rfc-compliance.md` | `docs/contributing/rfc-implementation-guide.md` |
| Write a spec | `rules/planning.md` | `plan/TEMPLATE.md` |
| Add a feature, tool, self-check, verification gate, or test infrastructure | `rules/discovery-updates.md` | Update docs, rules, indexes, and verification paths in the same change |
| Reorganize YANG tree | `scripts/dev/yang_move.py --help` | Preview diff, then `--apply` |
| Find context for an unfamiliar area | `ai/NAVIGATION.md` | Task-to-context decision tree |
| Understand Ze vs standard Go | `ai/rules/ze-divergences.md` | Buffer-first, registration, YANG, etc. |
| Know which hooks will check my code | `ai/rules/hook-mapping.md` | Pre-flight compliance checklist |
| Edit the website or presentations | `docs/contributing/gh-pages.md` then `../gh-pages/AI.md` | Worktree layout, tooling, adding a talk |

## Dev Tools

| Tool | Location | Purpose |
|------|----------|---------|
| `commit_helper.py` | `scripts/dev/` | Generate commit message files and executable user-run commit scripts. Reuses `tmp/commit-session-id`, rejects ignored/generated paths, uses `git commit -F`, and requires a learned summary or explicit no-lesson reason for workflow/tooling/rule changes. |
| `go_extract.go` | `scripts/dev/` | Move Go symbols between files |
| `replace.py` | `scripts/dev/` | Bulk find-and-replace with diff preview (run without `--apply` to review, then `--apply` to write). Supports `--regex` and `--all`. |
| `yang_move.py` | `scripts/dev/` | Format-aware YANG path refactoring. When YANG nodes move, updates slash paths, set commands, brace blocks, and GetContainer chains across the codebase. `remove <seg> --under <path>`, `rename <old> <new> --under <path>`, `move <src> <dst>`. Preview by default, `--apply` to write. Run `--test` for self-tests. |
| `bundle-html.py` | `gh-pages: presentations/tools/` | Inline local images, slides.md, and embeds into HTML as a self-contained file. Output: `<name>-inlined.html`. Accepts multiple files. |
| `make ze-verify-wiring-docs` | `mk/inventory.mk` | Changed-file-aware wiring, documentation, command, and inventory gate used by `make ze-verify`. |
| `go run ./scripts/status/verify_run.go ze-verify` | `scripts/status/verify_run.go` | Verify protocol runner used by `make ze-verify`. Writes `tmp/ze-verify.log`, per-stage logs, compact failure indexes, and `tmp/ze-verify.status`. |
| `verify-status.sh` | `scripts/dev/` | Checks whether the current tree is byte-identical to the last passing `ze-verify` run. Commit preparation must treat FRESH as authoritative and skip rerunning verify. |
| `make ze-doc-test` | `mk/inventory.mk` | Documentation drift, stale source anchors, and YANG command handler contract checks. |
| `make ze-inventory` / `make ze-inventory-json` | `mk/inventory.mk` | Registry-backed plugin, command, YANG, and test inventory. |
| `make ze-command-list` / `make ze-command-list-json` | `mk/inventory.mk` | Live command inventory generated from registered handlers and schemas. |
| `make ze-doc-index` | `mk/inventory.mk` | Regenerate `ai/CODE-TO-DOCS.md`, the source-to-document reverse index. |
| `make ze-spec-status` / `make ze-spec-status-json` | `mk/inventory.mk` | Spec progress overview for active planning and handoff. |

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
Full index: `ai/LEARNED-INDEX.md`. All summaries: `plan/learned/`.

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
| pool, memory, dedup, zero-copy, lifecycle | `rules/memory-architecture.md`, `core-design.md`, `pool-architecture.md`, `encoding-context.md` |
| textbuf, string building, AppendTo, alloc-free | `rules/no-sprintf-alloc.md`, `rules/memory-architecture.md`, `internal/core/textbuf/` |
| error message, actionable error, corrective action, remediation, fail closed | `rules/error-messages.md`, `rules/exact-or-reject.md`, `rules/derive-not-hardcode.md` |
| sync.Pool, buffer pool, ring buffer, peerPool | `rules/memory-architecture.md`, `forward-congestion-pool.md` |
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
| bridge, direct call, request/response, sync handler | `core-design.md` (section 9), `rules/plugin-design.md` (DirectBridge), `plan/learned/294-inprocess-direct-transport.md` |
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
| test, functional, .ci, verify failures | `docs/functional-tests.md` (top-level, not architecture/), `testing/ci-format.md` |
| netdata, telemetry, prometheus, metrics, monitoring, collector | `docs/guide/monitoring.md`, `docs/features.md`, `plan/learned/653-netdata-os-collectors.md` |
| DHCP, dhcp-server, lease, pool | `internal/plugins/dhcpserver/` (plugin), `ze-dhcp-server-conf.yang` |
| NTP, time sync | `internal/plugins/ntp/` (plugin), `ze-ntp-conf.yang` |
| sysctl, kernel tuning, profile | `internal/plugins/sysctl/` (plugin), `ze-sysctl-conf.yang` |
| firewall, nftables, NAT, masquerade | `internal/component/firewall/` (component), `ze-firewall-conf.yang` |
| PPPoE, pppoe-client, access concentrator | `internal/component/pppoe/` (AC), `internal/component/iface/` (client), `ze-pppoe-conf.yang` |
| wireguard, WireGuard, wg | `internal/component/iface/wireguard.go`, `ze-iface-conf.yang` |
| static route, default route | `internal/plugins/static/` (plugin), `ze-static-conf.yang` |
| conntrack, connection tracking | `internal/component/config/system/conntrack.go`, `ze-system-conf.yang` |
| archive, config backup, revision | `internal/component/config/archive/`, `ze-system-conf.yang` |
| SSH, authentication, user, public-key | `internal/component/ssh/`, `ze-ssh-conf.yang` |
| IPsec, IKE, IKEv2, SA, child SA | `plan/learned/734` (data model), `plan/learned/739` (crypto), `plan/learned/740` (engine), `plan/learned/742` (child SA) |
| EAP, NAT-T, MOBIKE | `plan/learned/744` (EAP/NAT-T), `plan/learned/737` (EAP extension) |
| XFRM, xfrm interface, VTI | `plan/learned/735` (XFRM interfaces) |
| subscriber, session, PPPoE, L2TP | `plan/learned/760-subscriber-session-model.md`, `internal/component/pppoe/` |
| editor, TUI, completion, headless | `internal/component/cli/`, `test/editor/`, `rules/testing.md` (Editor Tests section) |
| diagnostic, doctor, health, readiness | `plan/learned/755-ze-doctor.md`, `rules/doctor-checks.md`, `plan/learned/727-diag-core.md` |
| EventBus, event, pub/sub, subscribe, emit | `pkg/ze/eventbus.go`, `rules/plugin-design.md` (EventBus Typed Payloads), `internal/component/plugin/events.go` |
| DirectBridge, bridge, direct call, typed handler | `pkg/plugin/rpc/bridge.go`, `rules/plugin-design.md` (DirectBridge), `plan/learned/294-inprocess-direct-transport.md` |
| BFD, bidirectional forwarding | `docs/architecture/bfd.md` |
| resolve, origin, pipe, pipe operator | `docs/architecture/resolve.md`, `ai/rules/pipe-completeness.md` |
| MCP, model context protocol | `docs/architecture/mcp/`, `internal/component/mcp/` |
| self-update, manifest, auto-update | `plan/learned/748-self-update.md` |
| ASPA, path verification, RTR | `plan/learned/721-bgp-2-aspa.md`, `plan/learned/722-spec-bgp-4-aspa-policy.md` |
| BMP, monitoring protocol | `plan/learned/574-bgp-4-bmp.md`, `plan/learned/647-bmp-5-sender-compliance.md` |
| docker, container, scratch | `plan/learned/753-docker-go126.md`, `docs/guide/docker.md` |
| chaos, fault injection, scheduler | `plan/learned/723-chaos-actions-v2.md`, `docs/architecture/chaos-web-dashboard.md` |
| commit, commit script, commit message, lesson learned, verified commit, verify freshness | `scripts/dev/commit_helper.py`, `scripts/dev/verify-status.sh`, `ai/rules/git-safety.md`, `ai/skills/ze-commit.md`, `ai/skills/ze-commit-check.md` |
| self-improvement, discoverability, discovery, new tool, self-check, verification gate | `ai/rules/discovery-updates.md`, `ai/rules/hook-mapping.md`, `docs/contributing/documentation-testing.md` |
| inventory, command-list, doc drift, source anchor, doc index | `ai/rules/discovery-updates.md`, `ai/rules/documentation.md`, `docs/contributing/documentation-testing.md`, `mk/inventory.mk` |
| command grammar, verb-first, command alias, deprecated alias | `ai/rules/cli-grammar.md`, `plan/learned/829-command-verb-first.md` |
| DispatchCommandArgs, typed inter-plugin dispatch, tokenizer bypass | `plan/learned/830-typed-inter-plugin-dispatch.md`, `ai/rules/plugin-design.md` |
| RawMessage, double marshal, callback passthrough, SDK callback | `plan/learned/826-ipc-dispatch-data-raw.md`, `plan/learned/827-dispatch-response-passthrough.md`, `plan/learned/828-codec-callback-passthrough.md` |
| pipe first, pipe last, pipe metadata | `ai/rules/pipe-completeness.md`, `plan/learned/822-pipe-first-last.md` |
| RIB dump, bounded dump, replay batching, update cursor | `plan/learned/823-rib-show-bounded-dump.md`, `plan/learned/824-rib-feed-replay-batch.md` |
| plugin internal keyword, in-process plugin config | `plan/learned/821-plugin-internal-keyword.md`, `ai/patterns/plugin.md` |
| appliance auth, local admin, bootstrap auth, RBAC | `plan/learned/831-appliance-auth-hardening.md`, `internal/component/auth/`, `internal/component/aaa/` |
| install appliance iso, installer iso, appliance iso, install qemu | `docs/guide/appliance.md`, `docs/guide/ze-install.md`, `scripts/evidence/effective-install-iso-qemu.py`, `mk/test-integration.mk` |
| code-to-docs, reverse index, which docs | `ai/CODE-TO-DOCS.md` (generated, `make ze-doc-index`) |

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
