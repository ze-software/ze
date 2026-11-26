# Spec: bgp-rib-state-subtree-has-no-producer

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P1 |
| Release reason | Declared operational RIB state under `bgp/rib/{adj-rib-in,adj-rib-out}` must have a live producer answering gNMI operational-state reads with accurate YANG descriptions. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-improve-2-gnmi-state.md` (operational-state provider registry and gNMI `Get` `DataType` dispatch). Sibling `software-version/mode` half of the 2026-09-04 `unwired-feature.md` journal row is owned by `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`. |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| `bgp/rib/{adj-rib-in,adj-rib-out}` (`config false`) | Keep the declared state subtree and wire a real operational-state producer once `plan/release/fixes/spec-improve-2-gnmi-state.md` lands. |

## Task

**What the operator is promised.** The RIB plugin augments `/bgp:bgp` with a
`config false` container named `rib`
(`internal/component/bgp/plugins/rib/yang/ze-rib.yang:215-285`). It holds two
operational containers: `adj-rib-in` ("Routes received from peers") and
`adj-rib-out` ("Routes sent to peers"). Each holds a `list peer` keyed by
`address` whose entries expose `leaf address` (`string`) and `leaf route-count`
(`uint32`, covering all address families for that peer). A northbound operator
or controller reading a declared `config false` subtree over gNMI `Get`
(`STATE`, `OPERATIONAL`, or `ALL`) expects live per-peer Adj-RIB-In and
Adj-RIB-Out route counts matching the router's actual RIB.

**What Ze does instead.** No operational-state producer fills `/bgp/rib`.
`(*Server).Get` (`internal/component/gnmi/get.go:21-90`) walks only the running
config tree (`s.tree()`) and ignores `req.GetType()`, so every read under
`/bgp/rib/...` returns `NotFound`. Meanwhile, the per-peer counts already exist
inside `bgp-rib`: `(*RIBManager).status`
(`internal/component/bgp/plugins/rib/rib_commands.go:1039-1161`) computes
all-family per-peer `in` and `out` counts under `route-counts` for
`show bgp rib status`, and `updateMetrics`
(`internal/component/bgp/plugins/rib/rib.go:597-670`) publishes them to
Prometheus gauges, but neither feeds the declared YANG state tree. The YANG
`description` text on `rib`, `adj-rib-in`, and `adj-rib-out` still carries the
stopgap warning "No code answers a read of it today" / "No code answers a read
of this subtree", which contradicts the node `ze:help` text and leaks into the
published site reference snapshots.

**What closing it means.** Per the 2026-11-24 owner decision (and T-4 on
`plan/release/fixes/spec-yang-single-source-of-truth.md`), keep the declared
`/bgp/rib/{adj-rib-in,adj-rib-out}` subtree and wire `bgp-rib` as its real
operational-state producer on top of the operational-state provider mechanism
delivered by `plan/release/fixes/spec-improve-2-gnmi-state.md` (never inventing
a parallel mechanism beside it, per `ai/rules/no-layering.md`):

1. Extract one shared, lock-held per-peer route-count helper
   (`perPeerRouteCountsLocked`) in `internal/component/bgp/plugins/rib/rib_commands.go`
   used by both `(*RIBManager).status` and the new operational-state producer,
   guaranteeing exact mathematical agreement between `show bgp rib status` and
   `/bgp/rib/{adj-rib-in,adj-rib-out}`.
2. Implement the `/bgp/rib` operational-state snapshot producer and path
   resolver in `internal/component/bgp/plugins/rib/rib_state.go`, serving
   `/bgp/rib`, `/bgp/rib/adj-rib-in`, `/bgp/rib/adj-rib-out`, `peer` lists,
   keyed `peer[address=<ip>]` entries, and `address` / `route-count` (`uint32`)
   leaves with deterministic address ordering, omitting zero-count peer entries
   while keeping empty containers reachable, including manually injected BGP
   routes (`ribInPool[bgpProtocolID]`), aggregating multiple owners sharing one
   peer address, and excluding non-BGP redistribution sources.
3. Register `bgp-rib` as the `/bgp/rib` operational-state provider on both
   `registry.Registration` (`internal/component/bgp/plugins/rib/register.go`)
   and the live `sdk.Registration` / SDK state-read callback in `runRIBPlugin`
   (`internal/component/bgp/plugins/rib/rib.go`) using the exact contract
   landed by `spec-improve-2-gnmi-state.md`, so internal (`DirectBridge`) and
   forked external (`TLS RPC`) `bgp-rib` runs answer identically across the
   process boundary.
4. Bring `internal/component/bgp/plugins/rib/yang/ze-rib.yang` `description`
   and `ze:help` strings into full agreement (removing "No code answers a read"
   and adding a `revision` entry), update
   `docs/architecture/plugin/rib-storage-design.md`, and refresh the published
   site reference snapshots (`internal/le/site/testdata/published-configuration.md`,
   `internal/le/site/testdata/published-yang-config-tree.json`).

This spec is the sole owner of the `bgp/rib/adj-rib-in` / `adj-rib-out` half of
the 2026-09-04 `plan/journal/unwired-feature.md` row.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - Platform contracts, process boundary, registration over hardcoding
  → Decision: Every plugin runs internal or forked; code that only works in-process (such as reading a package-level singleton pointer from another component) is broken
  → Constraint: `bgp-rib` must publish `/bgp/rib` through the process-boundary-safe operational-state provider contract delivered by `spec-improve-2-gnmi-state.md`, never by exposing `*RIBManager` to `gnmi`
- [ ] `ai/doors/bgp-plugins.md` - BGP edge plugin placement, process-boundary parity, `ze_bgp` compile-out gate
  → Decision: `bgp-rib` is a `ze_bgp`-gated edge plugin; no always-on component imports it
  → Constraint: Stage 1 `sdk.Registration` and compile-time `registry.Registration` declarations must stay in agreement, and forked `bgp-rib` must answer state reads identically to internal `bgp-rib`
- [ ] `ai/doors/api-surfaces.md` and `docs/architecture/api/architecture.md` ("gNMI Transport") - gNMI Get RPC, encoding rules, auth and TLS, compile-out behind `ze_gnmi`
  → Decision: gNMI serves Ze YANG paths in `JSON` and `JSON_IETF` encodings and discovers state providers through the registry without importing any protocol or plugin package
  → Constraint: This spec adds zero `bgp-rib`-specific code to `internal/component/gnmi/`; `gnmi` reaches `bgp-rib` only through the generic provider registry from `spec-improve-2-gnmi-state.md`
- [ ] `docs/architecture/plugin/rib-storage-design.md` - `RIBManager` locking (`peerMu`), `bgpPeers` (owner-keyed), `ribInPool` (protocol-keyed), `ribOut` (owner-keyed), and `status` snapshot semantics
  → Decision: `peerMu.RLock` protects `bgpPeers`, `ribInPool`, `ribOut`, and `peerMeta`; `bgpPeers` and `ribOut` are keyed by `uint64` configured peer owner and map to IP addresses via `peerAddressLocked`
  → Constraint: Snapshot building must hold `peerMu.RLock` only long enough to copy per-peer integer counts, never holding it across RPC I/O or leaking internal `storage.PeerRIB` pointers
- [ ] `docs/architecture/plugin/plugin-system.md` and `docs/architecture/api/process-protocol.md` - 5-stage startup, query-mode purity, cross-boundary value types
  → Decision: Cross-boundary answers are self-contained values; `ze.plugin.mode=declare` starts nothing
  → Constraint: State-provider registration metadata in `register.go` is pure declarative data; live state queries return self-contained value trees / JSON maps

### RFC Summaries (Scope: plugin)
- [ ] `rfc/short/rfc4271.md` - Section 3.2 definitions of Adj-RIBs-In and Adj-RIBs-Out
  → Constraint: `adj-rib-in` counts routes advertised by BGP peers (plus explicit manual BGP injections in `ribInPool[bgpProtocolID]`), and `adj-rib-out` counts routes advertised to BGP peers; non-BGP redistribution tables are excluded

**Key insights:**
- `(*RIBManager).status` (`internal/component/bgp/plugins/rib/rib_commands.go:1039-1161`) already computes per-peer `in` and `out` counts across `bgpPeers`, `ribInPool[bgpProtocolID]`, and `ribOut` using `peerAddressLocked`, aggregating owners that share an address.
- `YANGSchema()` (`internal/component/config/yang_schema.go`) already includes the augmented `/bgp/rib/{adj-rib-in,adj-rib-out}/peer[address]/{address,route-count}` schema nodes, so `gnmi/path.go`'s `pathToSegments` and `segmentsToPath` already understand `peer[address=<ip>]` as a single-key list.
- Updating `ze-rib.yang`'s `description` changes rendered reference goldens in `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json`, which `TestTheConfigurationMirrorReadsAsThePublishedMirror` (`internal/le/site/config_test.go:152`) checks.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/yang/ze-rib.yang` - lines 215-285 augment `/bgp:bgp` with `config false` `container rib`, `container adj-rib-in`, `container adj-rib-out`, and keyed `list peer` (`address`, `route-count` `uint32`); descriptions explicitly say "No code answers a read of it today" / "No code answers a read of this subtree"
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - lines 1039-1161 implement `(*RIBManager).status(famFilter string)`, acquiring `r.peerMu.RLock()`, summing `bgpPeers`, `ribInPool[bgpProtocolID]`, and `ribOut` into `route-counts` keyed by peer IP address (`{"in": int, "out": int}`)
- [ ] `internal/component/bgp/plugins/rib/rib.go` - lines 379-396 define `ribInPool`, `bgpPeers`, `ribOut`; lines 597-670 implement `updateMetrics`; lines 798-965 implement `runRIBPlugin`, binding SDK handlers and `sdk.Registration` (no operational-state provider callback registered)
- [ ] `internal/component/bgp/plugins/rib/register.go` - registers `bgp-rib` with `registry.Register` (YANG, commands, sends/receives, dissemination, metrics, event bus; no operational-state provider declaration)
- [ ] `internal/component/gnmi/get.go` - `(*Server).Get` walks only `s.tree()` via `walkTree`, ignores `req.GetType()`, and returns `NotFound` for any `config false` path such as `/bgp/rib`
- [ ] `internal/component/gnmi/path.go` - `pathToSegments` and `segmentsToPath` convert gNMI paths to/from segment slices using `zeconfig.Schema`, preserving `peer[address=<ip>]` key translation
- [ ] `internal/component/config/yang_schema.go` - `schemaFromResolved` walks `-conf` modules including augments from `ze-rib.yang`, placing `rib` / `adj-rib-in` / `adj-rib-out` / `peer` in `YANGSchema()`
- [ ] `internal/le/site/config_test.go` - `TestTheConfigurationMirrorReadsAsThePublishedMirror` asserts rendered config documentation matches `internal/le/site/testdata/published-configuration.md`

**Behavior to preserve:**
- `show bgp rib status` (with and without an optional `<afi/safi>` family filter) keeps its exact JSON envelope, keys (`running`, `peers`, `routes-in`, `routes-out`, `stale-routes`, `route-counts`, optional `selected-output`, optional `gr-state`), and per-peer `{in, out}` values.
- `ze_rib_routes_in`, `ze_rib_routes_out`, `ze_rib_routes_in_total`, and `ze_rib_routes_out_total` Prometheus metrics keep their exact names, labels, and values.
- `/bgp/rib` remains `config false`: CLI `set bgp rib ...`, config file parsing, and gNMI `Set` on any path under `/bgp/rib` continue to be refused as read-only.
- gNMI `Get` with `DataType == CONFIG` on `/bgp/rib/...` continues to return `NotFound` (config tree never contains `config false` operational data).
- `ze start` startup behavior is unchanged (zero new config refusals; conforms to the 2026-11-26 "Ze always starts" ruling).
- `bgp-rib` EventBus `Sends` and `Receives` declarations and `bgp-rib` compilation behind `ze_bgp` remain unchanged.

**Behavior to change:**
- `bgp-rib` registers as the operational-state producer for YANG path `/bgp/rib` via the state-provider contract landed by `spec-improve-2-gnmi-state.md`, working identically whether `bgp-rib` runs in-process or as a forked external subprocess.
- gNMI `Get` with `DataType == STATE`, `OPERATIONAL`, or `ALL` on `/bgp/rib`, `/bgp/rib/adj-rib-in`, `/bgp/rib/adj-rib-out`, their `peer` lists, keyed `peer[address=<ip>]` entries, or `address` / `route-count` leaves returns live per-peer route counts matching `show bgp rib status`.
- `ze-rib.yang` `description` statements on `rib`, `adj-rib-in`, and `adj-rib-out` no longer claim "No code answers a read of this subtree"; they document the live operational-state producer alongside `show bgp rib status`, and the published site reference goldens are updated to match.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| RIB per-peer status counter | `internal/component/bgp/plugins/rib/rib_commands.go` `(*RIBManager).status` | Acquires `r.peerMu.RLock()`, aggregates all-family (or single-family) route counts across `bgpPeers`, `ribInPool[bgpProtocolID]`, and `ribOut` keyed by peer IP address | Extends: extracts `perPeerRouteCountsLocked` so `status` and the new `/bgp/rib` operational-state snapshot share one counting implementation with zero drift |
| Operational-state provider registry and gNMI `Get` `DataType` dispatch | `plan/release/fixes/spec-improve-2-gnmi-state.md` (prerequisite in `internal/component/plugin/registry/`, `pkg/plugin/sdk/`, `internal/component/gnmi/get.go`) | Provides YANG-path-keyed state-provider registration across the plugin boundary and routes gNMI `Get` (`STATE`/`OPERATIONAL`/`ALL`) to registered providers | Extends: `bgp-rib` registers `/bgp/rib` into this exact mechanism once it lands rather than inventing a second state-fetch path |
| Declared `config false` RIB YANG subtree | `internal/component/bgp/plugins/rib/yang/ze-rib.yang:215-285` (`augment "/bgp:bgp" { container rib { config false; ... } }`) | Declares `adj-rib-in/peer[address]/{address,route-count}` and `adj-rib-out/peer[address]/{address,route-count}`, already loaded into `config.YANGSchema()` | Extends: keeps the declared YANG schema path and node types intact, updates stale descriptions, and wires the live producer behind it |
| gNMI path-to-segment codec | `internal/component/gnmi/path.go` `pathToSegments`, `segmentsToPath` | Translates gNMI `PathElem` keys (such as `peer[address=192.0.2.1]`) against `zeconfig.Schema` | Does not apply (reused unchanged): already parses and formats `/bgp/rib/adj-rib-in/peer[address=<ip>]/route-count` from the registered `ze-rib.yang` schema |
| RIB plugin registration and SDK lifecycle | `internal/component/bgp/plugins/rib/register.go` `init`; `internal/component/bgp/plugins/rib/rib.go` `runRIBPlugin` | Registers compile-time plugin metadata (`registry.Register`) and runs the 5-stage SDK loop (`p.Run`) on both DirectBridge and forked TLS RPC | Extends: adds the `/bgp/rib` operational-state provider declaration and callback binding in both places for full internal/external parity |
| Site configuration golden tests | `internal/le/site/config_test.go` `TestTheConfigurationMirrorReadsAsThePublishedMirror` | Verifies published configuration markdown and YANG tree JSON match embedded YANG module descriptions | Extends: refreshes the two testdata snapshots so the updated `ze-rib.yang` descriptions stay in sync |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | `bgp-rib` registers its `/bgp/rib` operational-state callback through `spec-improve-2-gnmi-state.md`'s SDK / process-boundary provider mechanism in `runRIBPlugin` rather than exposing an in-process pointer to `gnmi` |
| `ai/doors/bgp-plugins.md` | Where it sits | the door covering `internal/component/bgp/plugins/rib/` | "The in-tree plugins hold no privilege over a third-party one" | agrees | `bgp-rib` registers and serves `/bgp/rib` operational state through the same generic operational-state provider registration and SDK callback available to every third-party plugin, working identically internal and forked |
| `docs/architecture/plugin/rib-storage-design.md` | Received source-policy provenance | the `// Design:` page of `rib_commands.go` and `rib.go`; a page this spec modifies | "The containing received store is keyed by the nonzero configured Peer owner. Its metadata holds the observed address and current Session token." | agrees | `perPeerRouteCountsLocked` and `rib_state.go` read `bgpPeers` and `ribOut` by configured peer owner, resolve peer addresses through `peerAddressLocked`, include `ribInPool[bgpProtocolID]`, and add a new section to this page documenting the `/bgp/rib` operational-state producer |
| `docs/architecture/api/commands.md` | Overview | anchored to `internal/component/bgp/plugins/rib/rib_commands.go` | - | silent | Covers CLI/API command dispatch and pipe rendering; `show bgp rib status` keeps its exact registration and JSON shape, and no CLI command is added or changed |
| `docs/architecture/route-selection.md` | Overview | anchored to `internal/component/bgp/plugins/rib/rib_commands.go` and `rib.go` | - | silent | Covers RFC 4271 best-path validation and selection; this spec reads route counts only and touches no selection logic |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/plugins/rib/rib_commands.go` | - | silent | Covers BGP UPDATE wire construction; this spec changes no UPDATE encoding or wire path |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `internal/component/bgp/plugins/rib/register.go` and `rib.go` | - | silent | Covers `ze doctor` checks and codes; this spec adds no doctor check, diagnostic code, or report-bus event |
| `docs/architecture/plugin/plugin-relationships.md` | 2. BGP Sub-Plugins | anchored to `internal/component/bgp/plugins/rib/register.go` | - | silent | Covers plugin dependency graph, config ownership, and bus topics; `bgp-rib` keeps its existing dependencies, config roots, and bus `Sends`/`Receives` unchanged |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers the 5-stage plugin wire protocol; this spec consumes the operational-state provider contract delivered by `spec-improve-2-gnmi-state.md` without modifying the protocol |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers BGP FSM Established-state transitions; read-only RIB operational-state queries do not affect FSM state |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers Prometheus pull-model metrics; existing `ze_rib_routes_*` gauges stay untouched |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers the RIB storage transition overview; `bgp-rib` storage layout and pool architecture are unchanged |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `internal/component/bgp/plugins/rib/rib.go` | - | silent | Covers zero-copy forward-handle fast path; this spec only reads integer route counts under `peerMu.RLock` |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/site/testdata/published-yang-config-tree.json` and `test/plugin/gnmi-bgp-rib-state.ci` | "Run one package through `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which takes the tags from the toolchain." | agrees | Deliverables and Goal Gates use `./le test unit pkg path` and `./le test functional plugin` (scoped tests only, no whole-tree verification command) and place the functional test in `test/plugin/gnmi-bgp-rib-state.ci` |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Does not add or alter any CLI command path or pipe operation |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not alter any command pipe overrides or answer shapes |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not alter pipe operator behavior |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Does not modify FlowSpec next-hop emission or BGP UPDATE building |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Does not add any undeclared engine operation; uses the declared state-provider contract from `spec-improve-2-gnmi-state.md` and leaves `bgp-rib` bus `Sends`/`Receives` unchanged |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not modify event delivery queues or overflow handling |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A gNMI client sends a `GetRequest` with `type` = `STATE`, `OPERATIONAL`, or `ALL` and a path at or below `/bgp/rib` (for example `/bgp/rib`, `/bgp/rib/adj-rib-in`, `/bgp/rib/adj-rib-in/peer[address=192.0.2.1]/route-count`, or `/bgp/rib/adj-rib-out/peer[address=192.0.2.1]/route-count`).
- The `bgp-rib` plugin maintains live `bgpPeers`, `ribInPool[bgpProtocolID]`, and `ribOut` tables in memory under `r.peerMu`.

### Transformation Path
1. `(*Server).Get` (`internal/component/gnmi/get.go`, after `spec-improve-2-gnmi-state.md`) authenticates the request, validates encoding (`JSON` or `JSON_IETF`), converts each `gpb.Path` to segments via `pathToSegments`, and inspects `req.GetType()`.
2. For `CONFIG`, `Get` walks only the running config tree (`s.tree()`), so `/bgp/rib/...` returns `NotFound`.
3. For `STATE`, `OPERATIONAL`, or `ALL`, the operational-state provider registry matches prefix `/bgp/rib` to the `bgp-rib` provider and dispatches the read across the plugin boundary (DirectBridge when internal, TLS RPC when forked) under the per-provider deadline owned by `spec-improve-2-gnmi-state.md`.
4. Inside `bgp-rib`, `(*RIBManager).handleOperationalState` (`internal/component/bgp/plugins/rib/rib_state.go`) acquires `r.peerMu.RLock()`, invokes the shared `r.perPeerRouteCountsLocked("")` helper (`rib_commands.go`) to compute all-family per-peer `(in, out)` totals across `bgpPeers`, `ribInPool[bgpProtocolID]`, and `ribOut`, and releases `r.peerMu.RUnlock()` immediately.
5. `rib_state.go` projects the copied counts into the `ze-rib.yang` subtree shape:
   - `adj-rib-in.peer`: deterministic address-sorted entries for peers with `in > 0`, each holding `address` (`string`) and `route-count` (`uint32`, clamped to `math.MaxUint32`).
   - `adj-rib-out.peer`: deterministic address-sorted entries for peers with `out > 0`, each holding `address` (`string`) and `route-count` (`uint32`, clamped to `math.MaxUint32`).
6. `rib_state.go` resolves the requested sub-path relative to `/bgp/rib` (container, `peer` list, keyed `peer[address=<addr>]`, or leaf `address` / `route-count`), returning `NotFound` for absent peer keys or unknown child segments, and returning empty containers for `/bgp/rib`, `/bgp/rib/adj-rib-in`, and `/bgp/rib/adj-rib-out` when no routes exist.
7. `gnmi/get.go` encodes the returned self-contained operational value into the requested `TypedValue` (`JSON`/`JSON_IETF` for containers/lists, scalar `TypedValue` for leaves) and merges with config state when `DataType == ALL`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gNMI Server ↔ State Provider Registry | YANG path prefix `/bgp/rib` lookup via `spec-improve-2-gnmi-state.md` registry; `gnmi` holds zero `bgp-rib` imports | No |
| Plugin Server ↔ `bgp-rib` Plugin | SDK operational-state read callback (DirectBridge in-process, TLS RPC when forked) returning self-contained value data | No |
| `RIBManager` Lock Boundary | `r.peerMu.RLock()` held only while copying integer per-peer counts in `perPeerRouteCountsLocked`; released before building response structures or crossing IPC | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `test/`, and `docs/` for `adj-rib-in`, `adj-rib-out`, `ze-rib.yang`, and `(*RIBManager).status` (`rib_commands.go:1039-1161` positive control: `show bgp rib status` and `summary.go` consumer). No wire format changes in this spec: BGP wire messages (`internal/component/bgp/message/`, `wireu/`) are untouched; `show bgp rib status` JSON output shape (`route-counts` map of `in`/`out`) is preserved byte-for-byte; the generic plugin-to-gNMI state-provider RPC/DirectBridge wire format is owned by prerequisite `plan/release/fixes/spec-improve-2-gnmi-state.md`; and the `/bgp/rib/{adj-rib-in,adj-rib-out}` gNMI operational payload strictly conforms to the existing `ze-rib.yang:215-285` schema, which previously had zero writers or readers.

### Integration Points
- `internal/component/bgp/plugins/rib/rib_commands.go` `(*RIBManager).status` - shares `perPeerRouteCountsLocked` with `rib_state.go` so CLI status and gNMI operational state never diverge.
- `internal/component/bgp/plugins/rib/register.go` `init` - declares `/bgp/rib` in `registry.Registration` using the state-provider registration field added by `spec-improve-2-gnmi-state.md`.
- `internal/component/bgp/plugins/rib/rib.go` `runRIBPlugin` - registers `/bgp/rib` in `sdk.Registration` and binds `r.handleOperationalState` to the SDK state-provider callback added by `spec-improve-2-gnmi-state.md`.
- `internal/component/bgp/plugins/rib/yang/ze-rib.yang` - updates descriptions on `rib`, `adj-rib-in`, `adj-rib-out` and adds a `revision` block; compiled in via existing `yang/embed.go`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Requests flow `gNMI Get -> provider registry -> plugin boundary (DirectBridge/TLS RPC) -> bgp-rib handleOperationalState -> perPeerRouteCountsLocked`; no component reads `activeManager` directly |
| No unintended coupling (components stay isolated) | Yes | `internal/component/gnmi` adds zero imports of `bgp` or `rib`; `bgp-rib` stays an edge plugin under `ze_bgp` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `spec-improve-2-gnmi-state.md`'s provider registry and shares `perPeerRouteCountsLocked` with `(*RIBManager).status` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Reads only integer lengths (`peerRIB.Len()`, `len(familyRoutes)`) under `peerMu.RLock`; never copies route attributes, NLRIs, or pooled buffers |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `bgp-rib` registers `/bgp/rib` via `registry.Registration` and `sdk.Registration`; zero `rib`-specific cases are added to `gnmi` or `plugin/server` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/gnmi/`, `internal/component/config/yang_schema.go`, and `internal/component/plugin/registry/registry.go`: `/bgp/rib` is already discovered dynamically from `ze-rib.yang` by `schemaFromResolved`, and state dispatch discovers `/bgp/rib` dynamically from the state-provider registry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `plan/release/fixes/spec-improve-2-gnmi-state.md` lands first and provides a process-boundary-safe operational-state provider registration contract (working for both internal DirectBridge and forked external plugins) keyed by YANG path prefix | Owner decision (2026-11-24) and `spec-improve-2-gnmi-state.md` Task/Data Flow; `ai/doors/system.md` process-boundary rule | Implementation of this spec is blocked until `spec-improve-2-gnmi-state.md` lands; if its contract is in-process only, `spec-improve-2-gnmi-state.md` must be fixed before wiring `bgp-rib` | Inspecting merged `spec-improve-2-gnmi-state.md` interfaces before Phase 1 coding | unvalidated |
| A-2 | `gnmi/path.go` (`pathToSegments` / `segmentsToPath`) already resolves `/bgp/rib/adj-rib-in/peer[address=<ip>]/route-count` against `config.YANGSchema()` because `schemaFromResolved` includes `ze-rib.yang` augments | `internal/component/config/yang_schema.go:353-397` and `internal/component/gnmi/path.go:27-125` | Path translation for `config false` list keys would fail with `InvalidArgument` | Unit test driving `pathToSegments` and `segmentsToPath` on `/bgp/rib/adj-rib-in/peer[address=192.0.2.1]/route-count` under `-tags ze_bgp` | unvalidated |
| A-3 | Canonical `netip.Addr.String()` normalization for requested `peer[address=<addr>]` keys matches the stored peer address keys (including IPv6 compressed form such as `2001:db8::1`) | `rib_commands.go:1069` (`addr.String()`) and `ze-rib.yang:240` | A client querying an unnormalized IPv6 literal (`2001:0db8::1`) would get a spurious `NotFound` | Unit test querying both IPv4 and unnormalized/canonical IPv6 peer addresses | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Holding `r.peerMu.RLock()` while building response trees or writing over IPC stalls BGP UPDATE processing under route churn | `-race` concurrency test or latency spikes in RIB benchmarks | Copy only per-peer `(in, out)` integers inside `perPeerRouteCountsLocked`, release `peerMu.RUnlock()` immediately, and construct all JSON/tree outputs lock-free |
| R-2 | Drift between `show bgp rib status` (`route-counts`) and `/bgp/rib/{adj-rib-in,adj-rib-out}` (e.g., manual injected routes, multi-owner same-address peers, or non-BGP redistribution sources) | Disagreement in unit or `.ci` comparison assertions | Both paths call the exact same `perPeerRouteCountsLocked` helper; unit and `.ci` tests assert exact equality between gNMI Get and `show bgp rib status` |
| R-3 | `bgp-rib` running in a separate/forked process (`external` / `process separate`) fails to answer `/bgp/rib` because of an accidental in-process singleton read | `./le plugin boundary check` or external-plugin functional test failure | Register and dispatch exclusively through `spec-improve-2-gnmi-state.md`'s SDK boundary; include a forked-transport test in `rib_state_test.go` and verify with `./le plugin boundary check` |
| R-4 | Updating `ze-rib.yang` descriptions breaks published site reference golden tests | `TestTheConfigurationMirrorReadsAsThePublishedMirror` failure in `internal/le/site/` | Refresh `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json` in the same commit |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | gNMI `Get` on `/bgp/rib/...` returns wrong counts, stale peer entries, or errors; live BGP sessions and forwarding are unaffected unless `peerMu` locking is mishandled |
| How is it reverted? | Single commit revert removes the `bgp-rib` operational-state provider registration, `rib_state.go`, and YANG description updates; zero config migration or wire compatibility impact |
| Who else touches this path? | `plan/release/fixes/spec-improve-2-gnmi-state.md` (prerequisite provider registry in `gnmi` and `plugin`); `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md` (sibling half of the 2026-09-04 `unwired-feature.md` row); `plan/release/fixes/spec-yang-single-source-of-truth.md` (allowlist referencing this spec) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| gNMI `Get` (`DataType=STATE`, `OPERATIONAL`, `ALL`) on `/bgp/rib/...` via state-provider registry | → | `bgp-rib` registered provider (`register.go`, `rib.go`) → `(*RIBManager).handleOperationalState` (`rib_state.go`) → `perPeerRouteCountsLocked` (`rib_commands.go`) | `TestGNMIGetBGPRIBOperationalStateWiring` (`internal/component/bgp/plugins/rib/rib_state_test.go`) |
| External/forked `bgp-rib` plugin answering `/bgp/rib/...` state read over SDK RPC transport | → | `runRIBPlugin` SDK state-read callback → `(*RIBManager).handleOperationalState` | `TestBGPRIBStateProducerWorksOverForkedTransport` (`internal/component/bgp/plugins/rib/rib_state_test.go`) |
| Real `ze` daemon with BGP peer + gNMI listener answering gNMI `Get` on `/bgp/rib/adj-rib-in` and `/bgp/rib/adj-rib-out` | → | Full daemon gNMI → state-provider dispatch → live `bgp-rib` plugin | `test/plugin/gnmi-bgp-rib-state.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Live `bgp-rib` holds Adj-RIB-In routes from peer A (across IPv4 and IPv6 families) and Adj-RIB-Out routes to peer B; gNMI `Get` (`STATE` or `OPERATIONAL`) requests `/bgp/rib`, `/bgp/rib/adj-rib-in`, `/bgp/rib/adj-rib-out`, `/bgp/rib/adj-rib-in/peer[address=A]/route-count`, and `/bgp/rib/adj-rib-out/peer[address=B]/route-count` | Every path returns the exact all-family per-peer route counts matching `show bgp rib status` (`route-counts[A].in` and `route-counts[B].out`), encoded as `uint32` / JSON per `ze-rib.yang`, with `address` equal to the canonical peer IP string | None: every declared node under `/bgp/rib/{adj-rib-in,adj-rib-out}` must return its live value matching `show bgp rib status` |
| AC-2 | Manually injected BGP routes (`request bgp rib inject`) and multiple configured peer owners sharing one remote IP address exist alongside non-BGP redistribution routes (`ribInPool[proto != bgpProtocolID]`) | `adj-rib-in/peer[address=<ip>]/route-count` includes manual BGP injections and sums same-address peer owners, while excluding non-BGP redistribution sources — matching `(*RIBManager).status("")` exactly | Non-BGP redistribution tables remain excluded from `bgp/rib/adj-rib-in/peer` because `ze-rib.yang` defines `peer` as BGP peers keyed by peer IP address (matching `status("")`) |
| AC-3 | A peer holds routes in `adj-rib-in` only (0 in `adj-rib-out`), or its last route is withdrawn/cleared (`clear bgp rib in` or `request bgp rib withdraw`), or the RIB holds zero routes overall | Container reads (`/bgp/rib`, `/bgp/rib/adj-rib-in`, `/bgp/rib/adj-rib-out`) succeed with empty `peer` entries when no peer holds routes in that direction; a peer with count 0 is omitted from that direction's `peer` list and direct reads of `peer[address=<ip>]` return `NotFound` | Empty containers `/bgp/rib`, `/bgp/rib/adj-rib-in`, `/bgp/rib/adj-rib-out` remain present (not `NotFound`) when `bgp-rib` is running so controllers can poll an idle router without treating zero routes as an error |
| AC-4 | gNMI `Get` requests `/bgp/rib` with `DataType=ALL` vs `DataType=CONFIG`, or requests an unknown child segment or unknown peer address under `/bgp/rib/...`, or `bgp-rib` is not running | `DataType=ALL` returns live `/bgp/rib` operational state (merged cleanly with `/bgp` config when `/bgp` is queried); `DataType=CONFIG` on `/bgp/rib` returns `NotFound`; unknown child segments or absent peer keys return `NotFound`; when `bgp-rib` is not running, `/bgp/rib` returns `NotFound` without panicking | `DataType=CONFIG` continues to serve only writable config nodes per gNMI Section 3.3.1 |
| AC-5 | `bgp-rib` runs as a forked external subprocess (over TLS RPC, not DirectBridge) and receives a gNMI `Get` (`STATE`) for `/bgp/rib` | Returns the identical live per-peer Adj-RIB-In and Adj-RIB-Out counts as an in-process `bgp-rib`, and `./le plugin boundary check` reports zero violations | None: process-boundary parity is mandatory for every Ze plugin (`ai/doors/system.md`) |
| AC-6 | Operator or schema reader inspects `ze-rib.yang` (`container rib`, `adj-rib-in`, `adj-rib-out`) and the published site configuration snapshots | `description` and `ze:help` agree on every node under `bgp/rib`, contain no "No code answers a read" contradiction, carry a YANG `revision` entry for this change, and `TestTheConfigurationMirrorReadsAsThePublishedMirror` passes | None: stale "No code answers" prose must not survive in `ze-rib.yang` or its published mirrors |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Polls per-peer Adj-RIB-In and Adj-RIB-Out counts via gNMI `Get` (`STATE` / `OPERATIONAL`) on a router with live BGP sessions | gNMI `Get` → state-provider registry → `bgp-rib` `handleOperationalState` → `perPeerRouteCountsLocked` → `TypedValue` response | `test/plugin/gnmi-bgp-rib-state.ci` |
| 2 | Fetches `/bgp` with gNMI `Get` (`ALL`) to get BGP config alongside live `rib` operational counts | gNMI `Get` (`ALL`) → running config tree + `/bgp/rib` state provider merge → `TypedValue` response | `test/plugin/gnmi-bgp-rib-state.ci` |
| 3 | Runs `bgp-rib` in a separate process and queries `/bgp/rib` over gNMI `Get` (`STATE`) | gNMI `Get` → plugin server TLS RPC → forked `bgp-rib` SDK callback → `handleOperationalState` → `TypedValue` response | `TestBGPRIBStateProducerWorksOverForkedTransport` |
| 4 | Withdraws or clears a peer's routes and re-polls `/bgp/rib/adj-rib-in/peer[address=<ip>]/route-count` | BGP withdrawal / `clear bgp rib in` → `bgpPeers` updated → gNMI `Get` returns `NotFound` for absent peer and empty `adj-rib-in` | `test/plugin/gnmi-bgp-rib-state.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPerPeerRouteCountsMatchesStatus` | `internal/component/bgp/plugins/rib/rib_state_test.go` | AC-1, AC-2: `perPeerRouteCountsLocked` matches `(*RIBManager).status("")` across multi-family `bgpPeers`, manual `ribInPool[bgpProtocolID]`, same-address multi-owner peers, `ribOut`, and excludes non-BGP `ribInPool` protocols | |
| `TestRIBStateSnapshotPathsAndValues` | `internal/component/bgp/plugins/rib/rib_state_test.go` | AC-1, AC-3, AC-4: resolves `/bgp/rib`, `/bgp/rib/adj-rib-in`, `/bgp/rib/adj-rib-out`, `peer`, `peer[address=<ip>]` (IPv4 + canonical/unnormalized IPv6), `address`, `route-count`; omits zero-count peers; returns empty containers when idle; returns `NotFound` for absent peers and unknown segments | |
| `TestRIBStateUint32ClampBoundary` | `internal/component/bgp/plugins/rib/rib_state_test.go` | Boundary conversion from `int` count to `uint32` (0, 1, normal, negative guard, `math.MaxUint32` saturation) | |
| `TestRIBStateConcurrentUpdatesNoRace` | `internal/component/bgp/plugins/rib/rib_state_test.go` | R-1: concurrent `handleOperationalState` reads during route inserts/withdrawals under `-race` | |
| `TestGNMIGetBGPRIBOperationalStateWiring` | `internal/component/bgp/plugins/rib/rib_state_test.go` | AC-1, AC-4: end-to-end wiring from `gnmi.Server.Get` (`STATE`, `OPERATIONAL`, `ALL`, `CONFIG`) through state-provider registry into `bgp-rib` | |
| `TestBGPRIBStateProducerWorksOverForkedTransport` | `internal/component/bgp/plugins/rib/rib_state_test.go` | AC-5: `bgp-rib` answers `/bgp/rib` state reads identically over non-DirectBridge RPC transport | |
| `TestZeRibYANGDescriptionsAgree` | `internal/component/bgp/plugins/rib/rib_state_test.go` | AC-6: `ze-rib.yang` `rib`/`adj-rib-in`/`adj-rib-out` nodes exist in `config.YANGSchema()`, remain `config false`, carry no "No code answers" text, and parse via `gnmi` path translation | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `route-count` (`uint32` conversion helper) | `0..4294967295` (`0..math.MaxUint32`; emitted entries hold `>= 1`) | `4294967295` (`math.MaxUint32`) | `< 0` (clamped to `0` / omitted) | `> math.MaxUint32` on 64-bit (saturated to `math.MaxUint32`, never wraps) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gnmi-bgp-rib-state` | `test/plugin/gnmi-bgp-rib-state.ci` | Real `ze` daemon + `le test peer` + gNMI listener: verifies gNMI `Get` (`STATE`, `OPERATIONAL`, `ALL`, `CONFIG`) on `/bgp/rib`, `/bgp/rib/adj-rib-in/...`, `/bgp/rib/adj-rib-out/...` matches `show bgp rib status` across route receipt, advertisement, and withdrawal | |

### Interop Tests (Scope: plugin)
- N-A: Zero BGP wire-protocol changes (no OPEN, UPDATE, NOTIFICATION, or KEEPALIVE bytes change). gNMI client interaction is verified in `test/plugin/gnmi-bgp-rib-state.ci`.

## Files to Modify
- `internal/component/bgp/plugins/rib/yang/ze-rib.yang` - update `description` on `rib`, `adj-rib-in`, `adj-rib-out` (remove "No code answers a read" contradictions) and add a `revision` entry
- `internal/component/bgp/plugins/rib/rib_commands.go` - extract `perPeerRouteCountsLocked` shared by `(*RIBManager).status` and `rib_state.go`
- `internal/component/bgp/plugins/rib/register.go` - register `/bgp/rib` operational-state provider metadata on `registry.Registration` (per `spec-improve-2-gnmi-state.md`)
- `internal/component/bgp/plugins/rib/rib.go` - declare `/bgp/rib` on `sdk.Registration` and bind `r.handleOperationalState` in `runRIBPlugin` (per `spec-improve-2-gnmi-state.md`)
- `docs/architecture/plugin/rib-storage-design.md` - document the `/bgp/rib/{adj-rib-in,adj-rib-out}` operational-state producer, shared counting helper, and process-boundary registration with source anchors
- `internal/le/site/testdata/published-configuration.md` - refresh published configuration mirror golden to reflect updated `ze-rib.yang` descriptions
- `internal/le/site/testdata/published-yang-config-tree.json` - refresh published YANG config tree golden to reflect updated `ze-rib.yang` descriptions

## Files to Create
- `internal/component/bgp/plugins/rib/rib_state.go` - `bgp-rib` operational-state snapshot builder and `/bgp/rib` path resolver (`handleOperationalState`, `uint32` count conversion)
- `internal/component/bgp/plugins/rib/rib_state_test.go` - unit, boundary, concurrency, wiring, and forked-transport parity tests for `/bgp/rib` operational state
- `test/plugin/gnmi-bgp-rib-state.ci` - end-to-end functional test querying `/bgp/rib/{adj-rib-in,adj-rib-out}` over gNMI against a live BGP peer and `show bgp rib status`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/rib/yang/ze-rib.yang` (existing `config false` `rib` subtree kept; descriptions and `revision` updated; zero new config leaves or RPCs) |
| YANG validation constraints | N-A | Existing `config false` `address` (`string`) and `route-count` (`uint32`) leaves are read-only operational state; no writable config constraints added |
| YANG custom validators | N-A | No new writable config leaves |
| CLI commands/flags | N-A | `show bgp rib status` is preserved unchanged; no CLI commands or flags added |
| CLI grammar (keyword before value) | N-A | No CLI commands or keywords added |
| Editor autocomplete | N-A | No writable config nodes added (`bgp/rib` remains `config false`) |
| Functional test for new RPC/API | Yes | `test/plugin/gnmi-bgp-rib-state.ci` exercises gNMI `Get` (`STATE`/`OPERATIONAL`/`ALL`/`CONFIG`) against live `bgp-rib` |
| Pipe completeness | N-A | gNMI `Get` serves YANG operational state, not CLI commands; `show bgp rib status` pipe handling is unchanged |
| Env var registration | N-A | No new environment variables |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, kernel module, listen port, procfs/sysctl, binary, or certificate |
| Doctor code per problem class | N-A | Read-only operational query introduces no refusal, `Warn`/`Error` log class, or report-bus event; absent peers and unknown paths return standard gNMI `NotFound` |
| Prometheus counters/metrics | N-A | `ze_rib_routes_in`, `ze_rib_routes_out`, `ze_rib_routes_in_total`, `ze_rib_routes_out_total` already exist in `rib.go` and stay unchanged; gNMI request/error counters already exist in `gnmi/metrics.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No address family, capability, or path attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Completes an already-declared `config false` YANG subtree; `docs/features.md` tracks the parent gNMI state capability in `spec-improve-2-gnmi-state.md` |
| 2 | Config syntax changed? | No | `bgp/rib` is `config false`; writable config syntax is unchanged |
| 3 | CLI command added/changed? | No | `show bgp rib status` is unchanged |
| 4 | API/RPC added/changed? | No | Generic gNMI `Get` `DataType` support is documented by `spec-improve-2-gnmi-state.md`; this spec adds one provider under the declared `/bgp/rib` path |
| 5 | Plugin added/changed? | No | Existing `bgp-rib` plugin gains an operational-state producer without changing its operator configuration surface in `docs/guide/plugins.md` |
| 6 | Has a user guide page? | No | No separate guide page describes `/bgp/rib` as unimplemented (verified by grep of `docs/`) |
| 7 | Wire format changed? | No | Zero wire-format changes |
| 8 | Plugin SDK/protocol changed? | No | Operational-state provider SDK/protocol docs are updated by `spec-improve-2-gnmi-state.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4271 Section 3.2 Adj-RIB-In/Out storage is already implemented; this exposes existing counts over gNMI |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner and gNMI test harness |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` anchors `ze-rib.yang` only for `blackhole-honor-fields` (line 300), unaffected |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` updated with the `/bgp/rib` operational-state producer section and source anchors |
| 13 | Route metadata keys added/changed? | No | No route metadata keys added or changed |
| 14 | Prometheus counters added/changed? | No | Existing RIB gauges unchanged |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Plugin name, commands, sends, receives, capabilities, and inventory unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/plugin/rib-storage-design.md` (updated). Advisory mentions unaffected (no change to anchored behavior): `docs/DESIGN.md` (overall architecture), `docs/architecture/api/commands.md` (`show bgp rib status` unchanged), `docs/architecture/behavior/fsm-established.md` (FSM unchanged), `docs/architecture/decisions/001-pull-model-metrics.md` (Prometheus metrics unchanged), `docs/architecture/doctor-and-health-checks.md` (doctor checks unchanged), `docs/architecture/plugin/plugin-relationships.md` (dependencies/bus topics unchanged), `docs/architecture/rib-transition.md` (RIB storage transition unchanged), `docs/architecture/rib/forward-handle.md` (fast path unchanged), `docs/architecture/route-selection.md` (best-path unchanged), `docs/architecture/update-building.md` (UPDATE building unchanged), `docs/config-reference.md` (anchors `blackhole-honor-fields` only), `docs/features/api-commands.md` (CLI commands unchanged), `docs/features/plugins.md` (plugin inventory unchanged), `docs/guide/bgp-resilience.md` (GR/resilience unchanged), `docs/guide/bmp.md` (BMP unchanged), `docs/guide/configuration.md` (anchors `blackhole-honor-fields` only), `docs/guide/graceful-restart.md` (GR unchanged), `docs/guide/looking-glass.md` (LG unchanged), `docs/guide/monitoring.md` (Prometheus gauges unchanged), `docs/guide/route-injection.md` (route injection unchanged), `docs/plugin-development/metrics.md` (metrics registration unchanged), `docs/plugin-development/schema.md` (YANG registration unchanged), `docs/plugin-overview.md` (plugin overview unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `internal/le/site/testdata/published-configuration.md` and `internal/le/site/testdata/published-yang-config-tree.json` render `ze-rib.yang` descriptions verbatim and are refreshed in the same commit |

## Design presentation

### What changes

`bgp-rib` now produces live operational state for its declared `config false`
YANG subtree `/bgp/rib` (`adj-rib-in/peer[address]/route-count` and
`adj-rib-out/peer[address]/route-count`) through the operational-state provider
mechanism delivered by `plan/release/fixes/spec-improve-2-gnmi-state.md`. Both
`show bgp rib status` and gNMI `Get` (`STATE`, `OPERATIONAL`, `ALL`) read from
one shared lock-held counter helper (`perPeerRouteCountsLocked`), so CLI and
gNMI counts always agree, whether `bgp-rib` runs in-process or forked. The
contradictory "No code answers a read of this subtree" notes in `ze-rib.yang`
and the published site reference mirrors are updated to describe the live
operational-state producer.

### Before and after

1. **gNMI Get `STATE` on `/bgp/rib/adj-rib-in/peer[address=192.0.2.1]/route-count`** (with 5 received routes from `192.0.2.1`):
   - **Before:** returns gRPC `NotFound` (`path not found: /bgp/rib/adj-rib-in/peer/192.0.2.1/route-count`) because gNMI walked only the running config tree and `bgp-rib` registered no operational-state producer.
   - **After:** returns `5` (`uint32` / JSON value), matching `show bgp rib status` `route-counts["192.0.2.1"].in`.
2. **gNMI Get `STATE` on `/bgp/rib`** (with 5 received from `192.0.2.1` and 2 sent to `192.0.2.2`):
   - **Before:** returns gRPC `NotFound`.
   - **After:** returns JSON containing `adj-rib-in.peer["192.0.2.1"]` (`address: "192.0.2.1"`, `route-count: 5`) and `adj-rib-out.peer["192.0.2.2"]` (`address: "192.0.2.2"`, `route-count: 2`), omitting peers with zero routes in that direction while returning empty containers when idle.
3. **YANG description in `ze-rib.yang` and published configuration reference:**
   - **Before:** `container rib` and `adj-rib-in` / `adj-rib-out` state "No code answers a read of it today" / "No code answers a read of this subtree."
   - **After:** `description` and `ze:help` agree that `bgp-rib` answers operational-state reads (`STATE`, `OPERATIONAL`, `ALL`) over gNMI alongside `show bgp rib status`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Delete `/bgp/rib/{adj-rib-in,adj-rib-out}` from `ze-rib.yang` and rely only on `show bgp rib status` | Rejected by owner decision (2026-11-24): northbound gNMI controllers need a YANG operational-state path for per-peer RIB counts |
| Expose `bgp-rib`'s `activeManager` singleton directly to `internal/component/gnmi` | Violates `ai/doors/system.md` (process-boundary parity: breaks when `bgp-rib` runs forked), `ai/doors/api-surfaces.md` (`gnmi` must not import protocol/plugin packages), and `feature-gates.txt` (`ze_gnmi` vs `ze_bgp` compile-out isolation) |
| Re-implement per-peer counting inside `rib_state.go` or shell out to `show bgp rib status` text parsing | Re-implementing risks drift from `status("")` on manual injections, multi-owner addresses, or non-BGP pools; sharing `perPeerRouteCountsLocked` keeps one source of truth with zero extra serialization |
| Invent a RIB-only gNMI hook before `spec-improve-2-gnmi-state.md` lands | Violates `ai/rules/no-layering.md` and the 2026-11-24 owner decision; `spec-improve-2-gnmi-state.md` owns the generic state-provider registry |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** -- register `/bgp/rib` operational-state provider metadata in `register.go` and bind a stub `handleOperationalState` callback in `runRIBPlugin` (`rib.go`) using the contract landed by `spec-improve-2-gnmi-state.md`; write failing wiring tests.
   - Tests: `TestGNMIGetBGPRIBOperationalStateWiring`, `TestBGPRIBStateProducerWorksOverForkedTransport` (`internal/component/bgp/plugins/rib/rib_state_test.go`)
   - Files: `internal/component/bgp/plugins/rib/register.go`, `internal/component/bgp/plugins/rib/rib.go`, `internal/component/bgp/plugins/rib/rib_state.go`, `internal/component/bgp/plugins/rib/rib_state_test.go`
   - Verify: gNMI `Get` (`STATE`) routes `/bgp/rib` through the state-provider registry into `bgp-rib` (internal and forked) and fails because the stub returns no counts yet.
2. **Phase 2: Shared counter helper and operational-state snapshot producer** -- extract `perPeerRouteCountsLocked` from `(*RIBManager).status` (`rib_commands.go`) and implement `handleOperationalState`, path resolution, IPv4/IPv6 key canonicalization, zero-count omission, and `uint32` clamping in `rib_state.go`.
   - Tests: `TestPerPeerRouteCountsMatchesStatus`, `TestRIBStateSnapshotPathsAndValues`, `TestRIBStateUint32ClampBoundary`, `TestRIBStateConcurrentUpdatesNoRace`, plus Phase 1 wiring tests now passing
   - Files: `internal/component/bgp/plugins/rib/rib_commands.go`, `internal/component/bgp/plugins/rib/rib_state.go`, `internal/component/bgp/plugins/rib/rib_state_test.go`
   - Verify: tests fail before implementation, pass after under `-race`, and `show bgp rib status` existing tests remain green unchanged.
3. **Phase 3: YANG descriptions, docs, site goldens, and functional test** -- update `ze-rib.yang` descriptions + revision, update `docs/architecture/plugin/rib-storage-design.md`, refresh `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json`, and add `test/plugin/gnmi-bgp-rib-state.ci`.
   - Tests: `TestZeRibYANGDescriptionsAgree`, `TestTheConfigurationMirrorReadsAsThePublishedMirror`, `test/plugin/gnmi-bgp-rib-state.ci`, `./le plugin boundary check`, `./le arch tier check`, `./le yang glue check`, `./le doc check all`
   - Files: `internal/component/bgp/plugins/rib/yang/ze-rib.yang`, `docs/architecture/plugin/rib-storage-design.md`, `internal/le/site/testdata/published-configuration.md`, `internal/le/site/testdata/published-yang-config-tree.json`, `test/plugin/gnmi-bgp-rib-state.ci`
   - Verify: functional test passes against a real `ze` daemon and peer; goldens and doc checks pass cleanly.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has an implementation at file:line and a passing test |
| Feature completeness | Every user story (1..4) has a connected path from gNMI `Get` to `bgp-rib` |
| Correctness | `perPeerRouteCountsLocked` is the single source of truth for both `status("")` and `/bgp/rib`; multi-family sums, manual BGP injections, same-address multi-owner peers, non-BGP pool exclusion, IPv6 key canonicalization, and zero-count peer omission all match |
| Naming | YANG node names (`rib`, `adj-rib-in`, `adj-rib-out`, `peer`, `address`, `route-count`) match `ze-rib.yang` verbatim |
| Data flow | Zero imports of `bgp`/`rib` in `internal/component/gnmi`; zero `activeManager` cross-package reads; `peerMu.RLock` released before IPC/encoding |
| Operator errors and doctor codes | Standard gNMI `NotFound` for absent peer keys / unknown sub-paths / `CONFIG` queries; no new config refusal at `ze start` (2026-11-26 ruling) |
| Rule: Process-boundary parity (`ai/doors/system.md`) | Works identically on DirectBridge and forked TLS RPC; `./le plugin boundary check` passes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Shared `perPeerRouteCountsLocked` + `rib_state.go` producer wired in `register.go` and `rib.go` | `./le test unit pkg path ./internal/component/bgp/plugins/rib/ run 'Test(PerPeerRouteCounts\|RIBState\|GNMIGetBGPRIB\|BGPRIBState\|ZeRibYANG)'` |
| Process-boundary and tier isolation preserved | `./le plugin boundary check` and `./le arch tier check` |
| Updated `ze-rib.yang`, `rib-storage-design.md`, and site reference goldens | `./le test unit pkg path ./internal/le/site/ run TestTheConfigurationMirrorReadsAsThePublishedMirror` and `./le doc check all` |
| End-to-end gNMI + live BGP peer functional proof | `./le test functional plugin` (including `test/plugin/gnmi-bgp-rib-state.ci`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Requested `peer[address=<addr>]` keys are parsed with `netip.ParseAddr` (invalid strings cleanly return `NotFound` without panicking or allocating unbounded state); path depth stays bounded by `gnmi/path.go`'s `maxPathDepth` |
| Resource exhaustion | `handleOperationalState` holds `peerMu.RLock` only for an `O(peers)` integer copy (never walking route tables or copying prefixes/attributes), bounded by `spec-improve-2-gnmi-state.md`'s per-provider deadline |
| Information exposure | Exposes only per-peer route counts already exposed by `show bgp rib status` and Prometheus gauges, gated behind existing gNMI bearer-token and TLS auth (`gnmi/server.go`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior and `spec-improve-2-gnmi-state.md`. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

None in the table (zero duplicate ownership). The shared 2026-09-04 open row in
`plan/journal/unwired-feature.md` (`session/capability/software-version/mode` and
`bgp/rib/adj-rib-in`, `adj-rib-out`) is already claimed in the owned-rows table
of `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`.
This spec is the sole owner of the `bgp/rib/adj-rib-in` and `adj-rib-out` half
per T-4 (2026-11-24). When this spec closes, if the `software-version/mode` half
is already fixed, flip the 2026-09-04 row to `fixed:`; otherwise note that the
`adj-rib-*` half is fixed while leaving the row open for `software-version/mode`.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Wire `bgp-rib` onto `spec-improve-2-gnmi-state.md`'s generic operational-state provider contract | Build a RIB-specific gNMI hook; delete `bgp/rib` from `ze-rib.yang` | Honors the 2026-11-24 owner decision, enforces registration over hardcoding (`ai/rules/plugins.md`), preserves `ze_bgp`/`ze_gnmi` compile-out isolation, and guarantees internal/forked process-boundary parity |
| Share `perPeerRouteCountsLocked` between `(*RIBManager).status` and `rib_state.go` | Duplicate map walks in `rib_state.go`; invoke `show bgp rib status` internally | Single source of truth prevents drift on multi-family totals, manual BGP injections, same-address multi-owner aggregation, and non-BGP pool exclusion |
| Omit zero-count peers from `adj-rib-in/peer` and `adj-rib-out/peer` while keeping empty containers reachable | Emit `{route-count: 0}` for peers with routes only in the opposite direction; return `NotFound` for empty containers | Matches `ze-rib.yang`'s "One entry for each peer that holds a route" while letting controllers poll `/bgp/rib`, `/bgp/rib/adj-rib-in`, and `/bgp/rib/adj-rib-out` cleanly on an idle router |

## Known Limitations
- Exposes only the `route-count` operational leaves declared in `ze-rib.yang` (`adj-rib-in/peer[address]/route-count` and `adj-rib-out/peer[address]/route-count`). Per-prefix route dumps over gNMI and gNMI `Subscribe` streaming for operational RIB state are outside this spec's scope (owned by `spec-improve-2-gnmi-state.md` / future telemetry specs).
- The `session/capability/software-version/mode` half of the 2026-09-04 `unwired-feature.md` journal row is owned by `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | - |
| `./le spec review check` | not run |
| Rounds | 0 |
| Reviewer lenses used | - |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/bgp/plugins/rib/`, `./le test unit pkg path ./internal/le/site/ run TestTheConfigurationMirrorReadsAsThePublishedMirror`, `./le test functional plugin` (including `test/plugin/gnmi-bgp-rib-state.ci`), `./le plugin boundary check`, `./le arch tier check`, `./le doc check all`, and the suites `./le repo changed scope print both` reaches (scoped tests only; GitHub CI runs whole-tree verification on push)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for `route-count` `uint32` conversion helper
- [ ] Functional `.ci` test (`test/plugin/gnmi-bgp-rib-state.ci`) for end-to-end behavior
- [ ] Interop tests N-A (zero BGP wire-protocol changes)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + journal row update (`plan/journal/unwired-feature.md`)
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-rib-state-subtree-has-no-producer.md` only
