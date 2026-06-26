# Spec: ospf-ext-11 -- OSPFv2 LDP-IGP Synchronization (RFC 5443, RFC 6138)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospf-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc5443.md` -- the base mechanism: cost-out an MPLS link to LSInfinity (`0xFFFF`) while LDP is not fully operational (§2); the three "fully operational" conditions (§2); the hold-down-timer estimation strategy for condition 3 (§2); raise the IP cost only, never the TE cost (§4); broadcast-link granularity is the whole link, not a peer (§3); SHOULD alert on a persistent (non-bring-up) cost-out (§3)
4. `rfc/short/rfc6138.md` -- the broadcast refinement: instead of max-metric on a LAN, WITHHOLD the Router-LSA "Link Type 2" (link to transit network) until LDP is operational with ALL peers on the subnet (§4); the single normative MUST NOT -- a cut-edge interface MUST be advertised immediately regardless of LDP state (§4); a scheduled-but-pending SPF MUST run before any cut-edge check (Appendix A); only the router whose link is coming up acts (§4)
5. `plan/spec-ospf-0-umbrella.md` -- delivered OSPFv2 umbrella; "Shared Contracts" (per-interface cost -> Router-LSA link metric; Router-LSA re-origination triggers; `ze_ospf_*` metric naming) and the note that LDP-IGP sync is a future extension on the stable base
6. `internal/plugins/ospf/lsdb/origination.go` -- `LSInfinity = 0xffff`, `routerLinks()` (the single place a per-link metric becomes a Router-LSA link, and the place that already substitutes `LSInfinity` when `in.MaxMetric`), `OriginateFromTopology`, `OriginateRouter`
7. `internal/plugins/ospf/instance.go` -- `lsdbTopology()` (builds `InterfaceInfo` with the per-interface `Cost`), `originateSelfLSAs()` (re-originates self-LSAs on change), `reconcile()`, `subscribeIfaceEvents()` (the existing EventBus subscription seam), `interfaceRuntimeConfigLocked()`
8. `internal/plugins/ldp/events.go` -- `SessionEvent{PeerAddress, TransportAddr, LDPIdentifier, SessionState, ...}`, `EventSessionUp`/`EventSessionDown`, `Namespace`; **note the event carries NO local interface name today**
9. `internal/plugins/ldp/register.go` -- `emitSessionEvent`, `startSessionForAdj` (where `SessionUp`/`SessionDown` are emitted), `discoverOnInterface(... ifName ...)` (the layer that DOES know the interface name), `runAdjacencyExpiry`
10. `internal/plugins/ospf/config.go` -- `interfaceConfig{Cost uint16; HasCost bool; ...}`, `maxMetricConfig` (RFC 6987 router-wide stub-router, NOT per-link; the precedent for a max-metric override but at a different scope), interface-list parsing
11. `internal/plugins/ospf/redistribute/consumer.go` -- the canonical pattern for OSPF subscribing to ANOTHER plugin's event stream via `ze.EventBus.Subscribe(namespace, eventType, handler)`

## Task

Add OSPFv2 LDP-IGP synchronization (RFC 5443, refined for broadcast links by
RFC 6138) to the native OSPFv2 plugin at `internal/plugins/ospf/`. On an
MPLS-enabled link, hold the OSPF interface output cost at maximum (LSInfinity,
`0xFFFF`) until the LDP plugin (`internal/plugins/ldp`) signals it is fully
synchronized on that link, so transit traffic is not black-holed before the LDP
label bindings exist. Restore the real (configured) cost once LDP is
synchronized. On broadcast links apply the RFC 6138 refinement: instead of
max-metric, withhold the Router-LSA transit (Link Type 2) advertisement for the
segment until LDP is operational with all peers, unless the interface is a
cut-edge (in which case it MUST be advertised immediately).

The umbrella (`plan/spec-ospf-0-umbrella.md`) delivered OSPFv2 with a per-interface
cost that flows `interfaceConfig.Cost` -> `InterfaceInfo.Cost` -> `routerLinks()`
metric -> Router-LSA, and an existing `LSInfinity` substitution path that
`OriginateFromTopology` already uses for the router-wide RFC 6987 max-metric
(`in.MaxMetric`). What is missing is the *per-link, LDP-driven* cost override: a
per-interface `ldp-sync` enable, an LDP-sync state machine per interface that
starts in "not synchronized" (cost = LSInfinity) when the link comes up, runs a
configurable hold-down timer after the LDP session is established, and restores
the configured cost on hold-down expiry; plus the wiring that subscribes the OSPF
engine to LDP `SessionUp`/`SessionDown` events and re-originates the Router-LSA on
every sync-state change.

This is purely a local mechanism: RFC 5443/6138 define NO wire format, message,
TLV, capability, or flag (RFC 5443 §2: "does not entail any protocol changes and
is a local implementation issue"). The only externally observable effect is the
metric value (or, for broadcast, the presence/absence of the transit link) in the
Router-LSA this router already originates. No new packet type, no codec change,
no neighbour negotiation.

The feature integrates with the existing LDP plugin's session state. Because the
LDP `SessionEvent` does not currently carry the local interface name (it carries
only `PeerAddress`/`TransportAddr`), part of this spec extends the LDP session
event with the discovering interface name (the `discoverOnInterface` layer
already knows it) so OSPF can map an LDP session to an OSPF interface; this is the
one cross-plugin change and it adds a field to an existing event, not a new
protocol surface.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Per-interface `ldp-sync` config | A new `ldp-sync` container under the OSPF interface list (`enable` boolean, `holddown` seconds); resolved into `interfaceConfig` |
| Per-interface LDP-sync state | A state machine per OSPF interface: NotSynchronized (cost forced to LSInfinity) -> HoldDown (LDP session up, awaiting binding-exchange estimate) -> Synchronized (configured cost restored); reverse on session loss (RFC 5443 §2 state model) |
| Cost override | While NotSynchronized/HoldDown, the effective Router-LSA link metric for the interface is LSInfinity (`0xFFFF`); applied at `lsdbTopology()`/`routerLinks()`, not by mutating the configured cost |
| Cost restoration | On HoldDown expiry (or, when available, End-of-LIB; not implemented here -- hold-down only), restore the configured cost and re-originate the Router-LSA (RFC 5443 §2) |
| Hold-down timer | Configurable per interface (`holddown` seconds); runs after LDP session establishment; on expiry the link is declared synchronized (RFC 5443 §2 estimation strategy; the RFC deliberately defines no default) |
| LDP event subscription | OSPF subscribes to LDP `EventSessionUp`/`EventSessionDown` via `ze.EventBus.Subscribe(ldp.Namespace, ...)`; maps the event to an OSPF interface by the new interface-name field |
| LDP event interface tag | Extend `ldp.SessionEvent` with the local interface name; emit it from `startSessionForAdj` (carry the `ifName` from `discoverOnInterface` through the adjacency) |
| Broadcast handling (RFC 6138) | On a broadcast interface, withhold the Router-LSA transit (Link Type 2) link for the segment until LDP is synchronized with all peers, UNLESS the interface is a cut-edge (then advertise immediately, MUST NOT delay -- §4); cut-edge computed from the existing SPF result (Appendix A); a pending SPF MUST run before the check |
| Re-origination trigger | Every sync-state change calls `originateSelfLSAs()` so the Router-LSA reflects the new metric / link presence immediately |
| Persistent-cost-out alert | When the interface stays NotSynchronized beyond a (configurable, default = hold-down) threshold after the LDP session was once up (a genuine fault, not bring-up), raise a network-management alert / log + metric (RFC 5443 §3 SHOULD) |
| CLI + metrics | `show ospf ldp-sync` (per-interface sync state); per-interface gauges/counters |

### Out of scope (noted so it is not silently assumed done)

| Item | Where / why |
|------|-------------|
| RSVP-TE IGP sync | Explicitly excluded by the task; only LDP sync here |
| TE link cost manipulation | RFC 5443 §4 forbids raising the TE cost; Ze originates no TE LSA today (that is ext-2), so there is nothing to suppress -- only the IP link cost is touched |
| LDP End-of-LIB (RFC 5919) | The deterministic condition-3 signal is not implemented in the LDP plugin; this spec uses the hold-down-timer estimation only (RFC 5443 §2 fallback). End-of-LIB is a future LDP enhancement |
| IPv6-family LDP-sync detail | The IPv6 address family (v6 code in `internal/plugins/ospf/v3/`, not a separate plugin) is not elaborated here; this spec targets the IPv4 address family. The state machine is AF-neutral, so the IPv6 half reuses it on the shared interface model |
| IS-IS LDP-sync (RFC 5443 §2 IS-IS half) | The `isis` plugin is a separate consumer of the same RFCs; not this spec |
| Targeted-LDP-over-TE-tunnel sync (RFC 5443 §4) | No TE tunnels in Ze; future work |
| Sync without support at the far end (RFC 6138 Appendix B) | "for further study"; not implemented |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` lines 1546-1549 ("LDP-IGP Synchronisation (RFC 5443, RFC 6138)") -- the FRR feature this mirrors: "Delays interface cost convergence until LDP has converged on the same link, avoiding transient black holes in MPLS networks. Defer unless ze has LDP."
  -> Decision: Ze HAS an LDP plugin (`internal/plugins/ldp`), so the FRR "defer unless ze has LDP" caveat is satisfied; build the feature against the LDP session-state events
  -> Constraint: the mechanism is "delay interface cost convergence" -- a local cost hold, not a protocol change; touch only Router-LSA origination, never the codec or neighbour negotiation
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts" (per-interface cost -> Router-LSA link metric; Router-LSA re-origination triggers; metric naming) -- the contracts this extends
  -> Constraint: the per-interface cost is a 16-bit field flowing `interfaceConfig.Cost` -> `InterfaceInfo.Cost` -> `routerLinks()`; the LDP-sync override substitutes LSInfinity at the SAME seam the RFC 6987 max-metric uses, so the LSDB key and Router-LSA layout are unchanged
  -> Decision: the override is computed at origination time (`lsdbTopology()`), never by writing into the stored configured cost; the configured cost must survive so it can be restored
- [ ] `ai/rules/plugin-self-containment.md` -- OSPF and LDP are independent plugins
  -> Constraint: OSPF must not import the LDP plugin's internals; the only coupling is the public `ze.EventBus` (`ldp.Namespace` + the session event types) and the new interface-name field on the public event payload. Removing the LDP plugin leaves OSPF compiling; with no LDP events the `ldp-sync` interface simply never leaves NotSynchronized (and, if `ldp-sync` is not enabled, the interface behaves exactly as today)
- [ ] `ai/rules/no-sprintf-alloc.md` -- no `fmt`/`+` on the hot path
  -> Constraint: any `show ospf ldp-sync` rendering uses `textbuf`/`AppendTo`; the per-interface state and timers do not allocate per tick
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` -- YANG vs env var, kebab-case
  -> Constraint: `ldp-sync` is operational config (per-interface, YANG), NOT an environment leaf; leaves use kebab-case (`ldp-sync`, `holddown`)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5443.md` -- the base LDP-IGP sync mechanism
  -> Constraint: §2 -- while LDP is not fully operational on a link, the IGP advertises that link with maximum cost; for OSPF the maximum cost is LSInfinity, the 16-bit value `0xFFFF` (as in RFC 3137/6987)
  -> Constraint: §2 -- LDP is "fully operational" only when all three hold: an LDP hello adjacency exists, a suitable associated LDP session is established to the peer at the other end of the link, AND all label bindings have been exchanged. Condition 3 "cannot generally be verified by a router"; use the hold-down-timer estimation (run after session establishment, declare operational on expiry)
  -> Constraint: §2 -- the hold-down default is deliberately undefined (LIB-size/equipment variation); make it configurable, do not hardcode a universal default
  -> Constraint: §4 -- apply the cost raise to the IP link cost ONLY, never the TE link cost (would force unnecessary TE reroutes). Ze originates no TE cost today, so only the IP metric is touched
  -> Constraint: §3 -- on a broadcast link the cost-out applies to the link as a whole, never a single peer; a persistent (non-bring-up) cost-out SHOULD raise a network-management alert
- [ ] `rfc/short/rfc6138.md` -- the broadcast refinement (updates RFC 5443)
  -> Constraint: §4 (the single normative MUST NOT) -- if the interface is a "cut-edge" (no alternate path to the directly connected network), updating the Router-LSA MUST NOT be delayed by LDP's operational state; advertise the link immediately or the network partitions
  -> Constraint: §4 -- if NOT a cut-edge, postpone updating the Router-LSA with the "Link Type 2" (link to transit network) for that subnet until LDP is operational with ALL neighbouring routers on the subnet; then update and flood
  -> Constraint: §4 -- the check is done just before the first broadcast adjacency is reflected in the LSA; only the router whose link is coming up acts (no negotiation, no wire change)
  -> Constraint: Appendix A -- a cut-edge needs no extra SPF run (derive from the last SPF); but if an SPF run is scheduled-but-pending it MUST be executed before any cut-edge check (no stale cut-edge across a topology change)

**Key insights:**
- RFC 5443/6138 add NO wire format. The whole feature is a *metric-origination* problem: substitute LSInfinity (P2P) or withhold the transit link (broadcast, non-cut-edge) at `routerLinks()`, driven by a per-interface LDP-sync state machine, and re-originate.
- Ze already substitutes `LSInfinity` at `routerLinks()` for the router-wide RFC 6987 max-metric (`in.MaxMetric`). The LDP-sync override is the SAME substitution at *per-interface* granularity, gated by per-interface sync state instead of a router-wide flag.
- The LDP plugin already emits `SessionUp`/`SessionDown` on the `ze.EventBus`, but the event lacks the local interface name. The smallest correct fix is to add that field (the `discoverOnInterface` layer knows it) rather than have OSPF reverse-map a transport address to an interface.
- Condition 3 ("all bindings exchanged") is not directly observable (no End-of-LIB in Ze's LDP); the hold-down timer is the RFC-sanctioned estimation. The state machine therefore has an explicit HoldDown sub-state between session-up and synchronized.
- Broadcast handling is genuinely different (RFC 6138): a single pseudonode cost cannot express "avoid peer B only", so withhold the whole transit link unless that would partition (cut-edge). Cut-edge reuses the existing SPF result.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/plugins/ospf/lsdb/origination.go` -- `LSInfinity types.Metric = 0xffff`; `routerLinks(in OriginInput)` builds the Router-LSA links: per interface it takes `metric := types.Metric(iface.Cost)` (defaulting 0 to 1) and, for P2P / broadcast-transit links, substitutes `LSInfinity` when `in.MaxMetric`; `OriginateFromTopology(router, maxMetric)` re-originates all self Router-LSAs; `OriginateRouter` installs one area's Router-LSA
  -> Constraint: `routerLinks()` is the SINGLE place a per-link cost becomes a Router-LSA link metric AND the place that already knows how to substitute LSInfinity; the per-interface override must land here (or in the `InterfaceInfo.Cost` feeding it), reusing the existing substitution pattern. The broadcast Link-Type-2 (transit) link is built in the `NetworkBroadcast` branch -- withholding it for RFC 6138 is a `continue`/skip in that same branch
  -> Constraint: `in.MaxMetric` is router-wide (RFC 6987); LDP-sync is per-interface, so the override cannot reuse `in.MaxMetric` -- it must arrive as a per-`InterfaceInfo` signal (an effective-cost or a per-interface max-metric flag)
- [ ] `internal/plugins/ospf/instance.go` -- `lsdbTopology()` builds `[]InterfaceInfo`, setting `Cost: ic.Cost` (default 1) per running interface; `originateSelfLSAs()` calls `OriginateFromTopology(cfg.RouterID, cfg.MaxMetric.RouterLSAAlways)`; `reconcile()` applies config; `subscribeIfaceEvents(eb)` is the existing EventBus subscription entry; `interfaceRuntimeConfigLocked(ic)` maps `interfaceConfig` to `ospfiface.Config`
  -> Constraint: `lsdbTopology()` is where the per-interface effective cost is chosen; the LDP-sync override is applied here (read the per-interface sync state, substitute LSInfinity / mark withhold) so every origination path inherits it. `originateSelfLSAs()` is the re-origination trigger every sync-state change must call
  -> Constraint: `subscribeIfaceEvents()` is the precedent for an engine-level EventBus subscription; the LDP-event subscription is a sibling (`subscribeLDPSyncEvents`) wired from the same `register.go` site
- [ ] `internal/plugins/ospf/config.go` -- `interfaceConfig{Cost uint16; HasCost bool; ...}`; the interface list is parsed from the YANG tree; `maxMetricConfig{RouterLSAAlways, OnStartupSec, OnShutdownSec}` is the router-wide RFC 6987 stub-router precedent
  -> Constraint: add `LDPSync` fields to `interfaceConfig` (enable + holddown seconds); parse from the new `ldp-sync` container; the configured `Cost` stays the restore value
- [ ] `internal/plugins/ldp/events.go` -- `SessionEvent{PeerAddress, TransportAddr, LDPIdentifier, SessionState, HoldTime, KeepaliveTime}`; `EventSessionUp`/`EventSessionDown`/`EventLabelBind`; `Namespace` and the `SessionUp`/`SessionDown`/`LabelBind` event handles
  -> Constraint: the event has NO local interface name -- this is the integration gap; add an `Interface string` field (and JSON tag) so OSPF can key sync state by OSPF interface name
- [ ] `internal/plugins/ldp/register.go` -- `startSessionForAdj` emits `SessionUp` (and `SessionDown` on session end) with `PeerAddress`/`TransportAddr`; `discoverOnInterface(ifctx, log, c, lsrID, ifName, adjTable, onAdj)` is the layer that DOES know `ifName`; `Adjacency{PeerLSRID, PeerLabelSpace, TransportAddr, HoldTime, Targeted, LastSeen}` (no interface field); `runAdjacencyExpiry` tears sessions
  -> Constraint: carry `ifName` from `discoverOnInterface` into the `Adjacency` (new field) and into `startSessionForAdj` so the emitted `SessionEvent.Interface` is populated; for a `SessionDown` from adjacency expiry the same interface is available
- [ ] `internal/plugins/ldp/session.go` -- `SessionState` (NonExistent/Initialized/OpenReceived/OpenSent/Operational); `ReadLoop(..., onOperational)`; `handleInit` drives the session to Operational; there is NO End-of-LIB handling
  -> Constraint: "LDP session established" for sync purposes = `StateOperational`; condition 3 (all bindings) is estimated by the hold-down timer since no End-of-LIB exists
- [ ] `internal/plugins/ospf/redistribute/consumer.go` -- `NewConsumer` + `ze.EventBus.Subscribe(namespace, eventType, handler)` is how OSPF already consumes another plugin's event stream
  -> Constraint: reuse this exact subscription shape for the LDP-sync subscriber; the handler is in-process, payload read-only (per `pkg/ze/eventbus.go`)
- [ ] `internal/plugins/ospf/iface/ism.go` + `iface.go` -- the OSPF interface state machine and `Config{Cost uint16, ...}`; interface up/down feeds `InterfaceInfo.State`
  -> Constraint: LDP-sync state is a SEPARATE per-interface machine layered beside the OSPF ISM; interface-down clears sync state (returns to NotSynchronized when it next comes up); the OSPF ISM is not modified
- [ ] `internal/plugins/ospf/spf/` (the SPF computer; `e.spf`) -- intra-area Dijkstra producing the route table
  -> Constraint: RFC 6138 cut-edge is derived from the last SPF result (Appendix A: "no alternate path for the directly connected network"); add a read-only cut-edge query, run a pending SPF first; do not add a second SPF pass

**Behavior to preserve:**
- An interface WITHOUT `ldp-sync` enabled behaves exactly as today: configured cost, transit link always advertised, no state machine, no LDP subscription effect.
- The per-interface cost -> Router-LSA metric flow, the LSDB key triple, the `LSInfinity` substitution for RFC 6987 router-wide max-metric, `OriginateFromTopology`/`OriginateRouter`/`routerLinks()` signatures.
- All existing OSPFv2 functional/interop tests; the LDP plugin's existing session/event behaviour (the only LDP change is an additive event field).
- `ze.EventBus.Subscribe` read-only-payload contract.

**Behavior to change:** (all RFC-5443/6138-required, gated behind `ldp-sync` enable)
- `interfaceConfig`: gains `LDPSyncEnabled bool` + `LDPSyncHoldDown` seconds.
- `ldp.SessionEvent`: gains `Interface string` (additive; default empty for any non-interface-scoped emitter).
- `lsdbTopology()`: for an `ldp-sync` interface that is NotSynchronized/HoldDown, the effective cost is LSInfinity (P2P) or the transit link is withheld (broadcast, non-cut-edge).
- `originateSelfLSAs()` is invoked on every LDP-sync state change.
- OSPF subscribes to `ldp` `SessionUp`/`SessionDown` events.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** the operator sets `ldp-sync { enable true; holddown N; }` under an OSPF interface -> `reconcile()` -> `interfaceConfig.LDPSync*` -> the per-interface sync machine is created in NotSynchronized.
- **LDP event:** the LDP plugin emits `SessionUp`/`SessionDown` (now tagged with `Interface`) on the `ze.EventBus` -> the OSPF LDP-sync subscriber -> the matching interface's sync machine transitions.
- **Timer:** the hold-down timer fires -> the machine declares Synchronized -> re-origination.
- **Interface down:** OSPF interface-down -> the machine resets (next bring-up starts NotSynchronized again).

### Transformation Path
1. **Config resolve (new):** `ldp-sync` container -> `interfaceConfig.LDPSyncEnabled`/`LDPSyncHoldDown`; the configured `Cost` is retained as the restore value.
2. **Sync-machine init (new):** when an `ldp-sync` interface comes up, the per-interface machine enters NotSynchronized; the effective cost for that interface becomes LSInfinity.
3. **LDP session up (new):** the subscriber receives `SessionUp{Interface=eth1}` -> the machine moves NotSynchronized -> HoldDown and arms the hold-down timer.
4. **Hold-down expiry (new):** the timer fires -> the machine moves HoldDown -> Synchronized (the §2 estimation that bindings are exchanged) -> the effective cost reverts to the configured cost -> `originateSelfLSAs()`.
5. **Origination (mostly existing):** `originateSelfLSAs()` -> `lsdbTopology()` reads each interface's effective cost (LSInfinity while not Synchronized; configured cost when Synchronized) -> `OriginateFromTopology` -> `routerLinks()` emits the link metric -> install + flood the Router-LSA (the existing §13 path).
6. **Broadcast refinement (new, RFC 6138):** for a broadcast `ldp-sync` interface that is not Synchronized, `lsdbTopology()`/`routerLinks()` consults the cut-edge query; if NOT a cut-edge the transit (Link Type 2) link is withheld from the Router-LSA; if it IS a cut-edge the link is advertised at normal cost immediately (MUST NOT delay). A pending SPF is flushed before the cut-edge query.
7. **LDP session down (new):** `SessionDown{Interface=eth1}` -> the machine returns to NotSynchronized -> cost forced to LSInfinity / transit withheld -> re-originate; if this happens after the link was once Synchronized and persists beyond the alert threshold, raise the §3 alert (log + metric).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> OSPF | new `ldp-sync` container -> `interfaceConfig` in `config.go` | [ ] |
| LDP <-> OSPF | `ze.EventBus` `ldp.Namespace` `SessionUp`/`SessionDown`, payload now carries `Interface` (additive); read-only | [ ] |
| LDP discovery <-> LDP event | `ifName` carried from `discoverOnInterface` -> `Adjacency` -> `startSessionForAdj` -> `SessionEvent.Interface` | [ ] |
| Sync machine <-> origination | per-interface effective cost / withhold flag read in `lsdbTopology()`; state change calls `originateSelfLSAs()` | [ ] |
| SPF <-> cut-edge | read-only cut-edge query over the last SPF result; pending SPF flushed first (RFC 6138 App A) | [ ] |
| Sync state <-> CLI/metrics | `show ospf ldp-sync`; `ze_ospf_ldp_sync_*` series | [ ] |

### Integration Points
- `internal/plugins/ospf/config.go` -- parse `ldp-sync`; `interfaceConfig` fields.
- `internal/plugins/ospf/instance.go` -- the per-interface sync machine map; `lsdbTopology()` effective-cost / withhold logic; `subscribeLDPSyncEvents()`; re-origination on state change.
- `internal/plugins/ospf/lsdb/origination.go` -- `routerLinks()` per-interface LSInfinity substitution + broadcast transit-link withhold (driven by a per-`InterfaceInfo` field).
- `internal/plugins/ospf/spf/` -- the read-only cut-edge query.
- `internal/plugins/ldp/events.go` + `internal/plugins/ldp/register.go` + `internal/plugins/ldp/discovery.go` -- the `Interface` field and its population.
- `internal/plugins/ospf/register.go` -- wire `subscribeLDPSyncEvents` (mirroring `subscribeIfaceEvents`).
- `internal/plugins/ospf/cmd_show.go` + `yang/ze-ospf-cmd.yang` -- `show ospf ldp-sync`.
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `ldp-sync` container.

### Architectural Verification
- [ ] No bypassed layers (the override flows config -> sync machine -> `lsdbTopology` -> `routerLinks` -> §13 flood, the same Router-LSA spine as a normal cost change)
- [ ] No unintended coupling (OSPF depends on the public `ze.EventBus` + `ldp.Namespace`/event types only; no LDP internal import; LDP does not know OSPF exists)
- [ ] No duplicated functionality (reuses `LSInfinity`, `routerLinks()`, `OriginateFromTopology`, `originateSelfLSAs`, the EventBus subscription pattern, and the last SPF result; adds only the per-interface machine, the override read, the broadcast withhold, and the cut-edge query)
- [ ] Zero-copy preserved (no per-tick allocation in the sync machine; the override is a metric substitution, not a new buffer)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The per-interface cost -> Router-LSA metric flows through `routerLinks()` and `lsdbTopology()` sets `InterfaceInfo.Cost`, so a per-interface effective-cost override there reaches the wire | `origination.go` `routerLinks()` `metric := types.Metric(iface.Cost)`; `instance.go` `lsdbTopology()` `Cost: ic.Cost` | the override would need a deeper Router-LSA rewrite; larger change | `TestLDPSyncForcesMaxMetric` asserts the originated Router-LSA link metric is `0xFFFF` while NotSynchronized | unvalidated |
| A-2 | `LSInfinity = 0xffff` is the correct OSPF max-cost and is already the value the codebase substitutes for max-metric | `origination.go` `const LSInfinity types.Metric = 0xffff`; RFC 5443 §2 (OSPF max = LSInfinity 0xFFFF) | wrong max value; black-hole not avoided / link withdrawn | `TestLDPSyncMaxMetricValue` pins `0xFFFF`; RFC §2 cross-check | unvalidated |
| A-3 | The LDP plugin emits `SessionUp`/`SessionDown` on the `ze.EventBus` and OSPF can subscribe with `ze.EventBus.Subscribe(ldp.Namespace, ...)` exactly as the redistribute consumer subscribes to other streams | `ldp/events.go` `EventSessionUp/Down`, `Namespace`; `ldp/register.go` `emitSessionEvent`; `ospf/redistribute/consumer.go` `Subscribe` | a different cross-plugin mechanism is needed; new plumbing | `TestLDPSyncSubscribesSessionEvents` (a published SessionUp drives the machine) | unvalidated |
| A-4 | The LDP `SessionEvent` does NOT carry the local interface name today, so adding an `Interface` field is required and the field is populated from the `discoverOnInterface` `ifName` reachable at `startSessionForAdj` | `ldp/events.go` (no interface field); `ldp/register.go` `discoverOnInterface(... ifName ...)`, `startSessionForAdj`; `ldp/discovery.go` `Adjacency` (no interface field) | OSPF must reverse-map a transport address to an interface (brittle); or the field cannot be populated | `TestLDPSessionEventCarriesInterface` (emit carries the discovering interface) | unvalidated |
| A-5 | "LDP session established" for sync = `SessionUp` (`StateOperational`); condition 3 (all bindings) is estimated by the hold-down timer because no End-of-LIB exists in the LDP plugin | `ldp/session.go` `StateOperational`, no End-of-LIB; RFC 5443 §2 estimation strategy | premature restore re-introduces the black hole, or sync never completes | `TestLDPSyncRestoresAfterHoldDown` (cost restored only on timer expiry, not on session-up) | unvalidated |
| A-6 | Cut-edge (RFC 6138) can be derived read-only from the last SPF result without a second Dijkstra pass | `ospf/spf/`; RFC 6138 App A ("should not require extra SPF runs") | extra SPF cost / complexity; possible stale result | `TestLDPSyncBroadcastCutEdgeAdvertised` + `TestLDPSyncCutEdgeUsesFreshSPF` | unvalidated |
| A-7 | Withholding the broadcast transit (Link Type 2) link is a clean skip in the `NetworkBroadcast` branch of `routerLinks()` and does not corrupt the rest of the Router-LSA (the stub link for the subnet is a separate decision) | `origination.go` `routerLinks()` `NetworkBroadcast` branch builds the transit link separately from the stub link | the whole interface's links vanish, or SPF mis-reads the LSA | `TestLDPSyncBroadcastWithholdsTransitLink` (only the transit link is absent; non-cut-edge) | unvalidated |
| A-8 | A per-interface LDP-sync state machine is independent of the OSPF ISM; interface-down simply resets it; no OSPF ISM change is needed | `ospf/iface/ism.go`; `instance.go` interface up/down hooks | the OSPF ISM must be modified; wider blast radius | package builds; `TestLDPSyncResetsOnInterfaceDown` | unvalidated |
| A-9 | The configured cost survives the override (the override is computed at origination, not stored), so restoration uses the original `interfaceConfig.Cost` | design decision; `lsdbTopology()` reads `ic.Cost` | restoration restores LSInfinity, leaving the link costed-out forever | `TestLDPSyncRestoresConfiguredCost` (after sync, metric == configured cost, not 1, not 0xFFFF) | unvalidated |
| A-10 | Removing the LDP plugin (build without it) leaves OSPF compiling and an `ldp-sync` interface simply stuck NotSynchronized; with `ldp-sync` disabled the interface is byte-for-byte today's behaviour | `plugin-self-containment.md`; OSPF imports only `ze.EventBus`/`ldp.Namespace` constants | a hard OSPF->LDP dependency violates plugin self-containment | `go build` without ldp; `TestLDPSyncDisabledIsNoOp` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Hold-down too short -> cost restored before label bindings exist -> the black hole the feature exists to prevent | traffic dropped right after sync declared; FRR shows a label gap | hold-down is configurable and never defaulted to 0; `TestLDPSyncRestoresAfterHoldDown` proves restore waits for the timer; document the trade-off (RFC 5443 §2) |
| R-2 | The override mutates the stored configured cost -> after one sync cycle the link is permanently LSInfinity | a synced link still shows metric 0xFFFF | compute the override at origination only; never write LSInfinity into `interfaceConfig.Cost`; `TestLDPSyncRestoresConfiguredCost` |
| R-3 | RFC 6138 cut-edge wrong -> a cut-edge link is withheld -> the broadcast network is partitioned (violates the one MUST NOT) | a directly connected LAN becomes unreachable when LDP is slow | cut-edge MUST advertise immediately; `TestLDPSyncBroadcastCutEdgeAdvertised`; default-safe: if cut-edge cannot be determined, advertise (do not withhold) |
| R-4 | LDP `SessionEvent.Interface` empty (emitter that does not know the interface) -> OSPF cannot match the event -> sync never completes | a configured `ldp-sync` interface stays NotSynchronized despite an operational LDP session | populate `Interface` at `startSessionForAdj`; OSPF logs + a metric on an unmatched event; `TestLDPSessionEventCarriesInterface` |
| R-5 | Re-origination storm: a flapping LDP session toggles the metric every event -> Router-LSA churn | high `ze_ospf_ldp_sync_transitions_total` rate; LSA flood rate spike | the hold-down timer naturally damps flap (a down->up cycle must re-serve hold-down); MinLSInterval throttles re-origination via the existing origination path |
| R-6 | A persistent NotSynchronized (genuine fault) goes unnoticed by the operator | MPLS traffic quietly avoids a link forever | the §3 alert: log + `ze_ospf_ldp_sync_holddown_expired_total` / a "stuck" gauge when NotSynchronized persists past the threshold after having been up |
| R-7 | The LDP-event subscription leaks (not unsubscribed on OSPF shutdown/reconcile) -> stale handler or double subscription | duplicate state transitions; goroutine/handler growth across reconciles | keep the `unsubscribe` func from `Subscribe`; call it on engine shutdown/reconcile, mirroring `subscribeIfaceEvents` lifecycle; `TestLDPSyncUnsubscribesOnShutdown` |
| R-8 | Pending SPF not flushed before the cut-edge check (RFC 6138 App A MUST) -> a stale cut-edge result across a topology change | a just-failed alternate path still treated as present; wrong withhold decision | flush a scheduled-but-pending SPF before the query; `TestLDPSyncCutEdgeUsesFreshSPF` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ldp-sync { enable true; holddown N }` under an OSPF interface in config | -> | `reconcile()` -> `interfaceConfig.LDPSync*` -> the per-interface sync machine is created NotSynchronized; the interface originates at LSInfinity | `TestLDPSyncConfigCreatesMachine` (unit) + `test/ospf/ospf-ldp-sync-config.ci` |
| The OSPF engine starts with LDP-sync enabled | -> | `subscribeLDPSyncEvents(eb)` subscribes to `ldp.Namespace` `SessionUp`/`SessionDown` | `TestLDPSyncSubscribesSessionEvents` (unit) |
| LDP publishes `SessionUp{Interface=eth1}` | -> | subscriber -> machine NotSynchronized -> HoldDown -> (timer) Synchronized -> `originateSelfLSAs()` | `test/ospf/ospf-ldp-sync-restore.ci` |
| LDP publishes `SessionDown{Interface=eth1}` after sync | -> | machine -> NotSynchronized -> cost forced LSInfinity -> re-originate | `test/ospf/ospf-ldp-sync-down.ci` |
| LDP starts a session on an interface | -> | `discoverOnInterface` `ifName` -> `Adjacency` -> `startSessionForAdj` -> `SessionEvent.Interface` populated | `TestLDPSessionEventCarriesInterface` (unit, ldp package) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An OSPF interface with `ldp-sync enable true` comes up; no LDP session yet | the interface's Router-LSA link metric is LSInfinity (`0xFFFF`); the sync state is NotSynchronized (RFC 5443 §2) |
| AC-2 | An LDP `SessionUp` arrives for that interface | the machine moves to HoldDown and arms the configured hold-down timer; the metric stays LSInfinity (bindings not yet estimated) (§2) |
| AC-3 | The hold-down timer expires | the machine moves to Synchronized; the Router-LSA link metric reverts to the CONFIGURED cost (not 1, not 0xFFFF) and the Router-LSA is re-originated (§2) |
| AC-4 | An LDP `SessionDown` arrives for a Synchronized interface | the machine returns to NotSynchronized; the metric is forced back to LSInfinity and the Router-LSA re-originated (§2 state model) |
| AC-5 | The configured cost is C; an interface goes NotSynchronized -> HoldDown -> Synchronized | after sync the metric is exactly C; the stored configured cost is never overwritten with LSInfinity (the override is origination-time only) |
| AC-6 | A broadcast `ldp-sync` interface that is NOT a cut-edge is NotSynchronized | the Router-LSA transit (Link Type 2) link for that subnet is WITHHELD until LDP is synchronized with all peers; it appears once Synchronized (RFC 6138 §4) |
| AC-7 | A broadcast `ldp-sync` interface that IS a cut-edge is NotSynchronized | the transit link is advertised IMMEDIATELY at normal cost regardless of LDP state (RFC 6138 §4 MUST NOT delay) |
| AC-8 | A cut-edge check runs while an SPF is scheduled-but-pending | the pending SPF is executed before the cut-edge result is read (RFC 6138 Appendix A MUST) |
| AC-9 | `ldp-sync` holddown is set to N seconds | the timer runs for N seconds after `SessionUp`; there is no hardcoded universal default (the operator must choose) (§2) |
| AC-10 | An OSPF interface WITHOUT `ldp-sync` (or with `enable false`) | byte-for-byte today's behaviour: configured cost, transit always advertised, no subscription effect, no state machine |
| AC-11 | The LDP plugin emits a session event for a discovered interface | `SessionEvent.Interface` is the discovering OSPF/LDP interface name (P2P link case) so OSPF can match it |
| AC-12 | An OSPF interface stays NotSynchronized past the alert threshold AFTER having been Synchronized (genuine fault) | a network-management alert is logged and `ze_ospf_ldp_sync_holddown_expired_total` / a "stuck" indicator records it (RFC 5443 §3 SHOULD) |
| AC-13 | `show ospf ldp-sync` | lists each `ldp-sync` interface with its state (not-synchronized / hold-down / synchronized), remaining hold-down, and effective metric |
| AC-14 | The OSPF engine reconciles or shuts down | the LDP-event subscription is unsubscribed (no stale handler, no double subscription on re-subscribe) |
| AC-15 | The TE link cost (if/when Ze originates a TE LSA) | is NEVER raised by this mechanism; only the IP link metric is touched (RFC 5443 §4). Today Ze originates no TE cost, so the IP-only guarantee holds trivially and is asserted by the absence of any TE-cost write |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `ldp-sync` on a P2P MPLS interface; the link comes up before LDP | config -> `reconcile` -> machine NotSynchronized -> `lsdbTopology` substitutes LSInfinity -> Router-LSA advertises `0xFFFF`; a peer's SPF avoids the link | `test/ospf/ospf-ldp-sync-config.ci` |
| 2 | LDP forms a session on the link; after the hold-down the cost is restored | LDP `SessionUp{Interface}` -> subscriber -> HoldDown -> timer -> Synchronized -> `originateSelfLSAs` -> Router-LSA advertises the configured cost; the peer's SPF prefers the link again | `test/ospf/ospf-ldp-sync-restore.ci` + `ospf-ldp-sync-frr` interop |
| 3 | LDP session drops on a synced link | LDP `SessionDown{Interface}` -> machine -> NotSynchronized -> metric LSInfinity -> re-originate; traffic diverts before the black hole | `test/ospf/ospf-ldp-sync-down.ci` |
| 4 | Configures `ldp-sync` on a broadcast (LAN) interface that has an alternate path | broadcast not-cut-edge -> transit Link-Type-2 withheld until LDP synced with all peers -> appears after sync | `test/ospf/ospf-ldp-sync-broadcast.ci` |
| 5 | Runs `show ospf ldp-sync` | CLI -> per-interface sync snapshot (state, remaining hold-down, effective metric) | `test/ospf/ospf-ldp-sync-show.ci` |
| 6 | Builds Ze without the LDP plugin, with an `ldp-sync` interface configured | OSPF compiles; the interface stays NotSynchronized (no LDP events); a non-`ldp-sync` interface is unaffected | `TestLDPSyncDisabledIsNoOp` + build-without-ldp check |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLDPSyncConfigCreatesMachine` | `internal/plugins/ospf/ldp_sync_test.go` | AC-1: `ldp-sync` config creates a NotSynchronized machine; effective cost = LSInfinity | |
| `TestLDPSyncForcesMaxMetric` | `internal/plugins/ospf/ldp_sync_test.go` | AC-1, A-1: originated Router-LSA link metric is `0xFFFF` while NotSynchronized | |
| `TestLDPSyncMaxMetricValue` | `internal/plugins/ospf/ldp_sync_test.go` | A-2: the substituted value is exactly `LSInfinity` (`0xFFFF`) | |
| `TestLDPSyncSubscribesSessionEvents` | `internal/plugins/ospf/ldp_sync_test.go` | AC-2, A-3: a published `SessionUp` drives the machine to HoldDown | |
| `TestLDPSyncRestoresAfterHoldDown` | `internal/plugins/ospf/ldp_sync_test.go` | AC-3, A-5, R-1: cost restored only on hold-down expiry, not on session-up | |
| `TestLDPSyncRestoresConfiguredCost` | `internal/plugins/ospf/ldp_sync_test.go` | AC-5, A-9, R-2: after sync the metric equals the configured cost; stored cost never overwritten | |
| `TestLDPSyncSessionDownForcesMaxMetric` | `internal/plugins/ospf/ldp_sync_test.go` | AC-4: `SessionDown` returns the machine to NotSynchronized and re-forces LSInfinity | |
| `TestLDPSyncHoldDownConfigurable` | `internal/plugins/ospf/ldp_sync_test.go` | AC-9: timer honours the configured `holddown`; no hardcoded default | |
| `TestLDPSyncResetsOnInterfaceDown` | `internal/plugins/ospf/ldp_sync_test.go` | A-8: interface-down resets the machine; next bring-up starts NotSynchronized | |
| `TestLDPSyncDisabledIsNoOp` | `internal/plugins/ospf/ldp_sync_test.go` | AC-10, A-10: no `ldp-sync` = today's behaviour; LDP events ignored | |
| `TestLDPSyncUnsubscribesOnShutdown` | `internal/plugins/ospf/ldp_sync_test.go` | AC-14, R-7: subscription removed on shutdown/reconcile | |
| `TestLDPSyncStuckRaisesAlert` | `internal/plugins/ospf/ldp_sync_test.go` | AC-12, R-6: persistent NotSynchronized after sync logs the alert + bumps the metric | |
| `TestLDPSyncBroadcastWithholdsTransitLink` | `internal/plugins/ospf/lsdb/ldp_sync_origination_test.go` | AC-6, A-7: non-cut-edge broadcast withholds only the transit Link-Type-2 link | |
| `TestLDPSyncBroadcastCutEdgeAdvertised` | `internal/plugins/ospf/lsdb/ldp_sync_origination_test.go` | AC-7, R-3: cut-edge broadcast advertises the transit link immediately at normal cost | |
| `TestLDPSyncCutEdgeUsesFreshSPF` | `internal/plugins/ospf/spf/ldp_sync_cutedge_test.go` | AC-8, A-6, R-8: a pending SPF is flushed before the cut-edge query | |
| `TestLDPSyncP2PMaxMetricNotWithheld` | `internal/plugins/ospf/lsdb/ldp_sync_origination_test.go` | A-7: P2P uses LSInfinity (not withhold); only broadcast uses withhold | |
| `TestLDPSessionEventCarriesInterface` | `internal/plugins/ldp/register_test.go` | AC-11, A-4, R-4: emitted `SessionEvent.Interface` is the discovering interface | |
| `TestLDPSyncTECostUntouched` | `internal/plugins/ospf/ldp_sync_test.go` | AC-15: the mechanism writes no TE cost (only the IP metric) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Effective link metric while not synchronized | LSInfinity | `0xFFFF` (65535) | N/A | N/A (16-bit max; never `>0xFFFF`) |
| Configured interface cost (restore value) | 1-65535 | 65535 | 0 (rejected by YANG `range 1..65535`) | N/A |
| `ldp-sync holddown` seconds | 0-65535 | 65535 | N/A (0 = no estimation wait; allowed but discouraged) | N/A |
| Restored metric after sync | configured cost | configured cost | N/A | must not be LSInfinity (the R-2 trap) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-ldp-sync-config` | `test/ospf/ospf-ldp-sync-config.ci` | an `ldp-sync` interface with no LDP session originates at metric 0xFFFF; `show ospf ldp-sync` shows not-synchronized | |
| `ospf-ldp-sync-restore` | `test/ospf/ospf-ldp-sync-restore.ci` | LDP session up + hold-down expiry restores the configured cost; state shows synchronized | |
| `ospf-ldp-sync-down` | `test/ospf/ospf-ldp-sync-down.ci` | LDP session down re-forces 0xFFFF on a synced interface | |
| `ospf-ldp-sync-broadcast` | `test/ospf/ospf-ldp-sync-broadcast.ci` | a non-cut-edge broadcast interface withholds the transit link until synced; a cut-edge advertises immediately | |
| `ospf-ldp-sync-show` | `test/ospf/ospf-ldp-sync-show.ci` | `show ospf ldp-sync` lists state, remaining hold-down, effective metric | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-ldp-sync-frr` | `test/interop/scenarios/ospf-ldp-sync-frr/` | FRR `ospfd` + FRR `ldpd` (`mpls ldp` with `ldp-sync` on the link) | FRR observes Ze advertise the MPLS link at LSInfinity while LDP is converging and at the configured cost after; Ze observes/handles the equivalent from FRR; the IP-only metric change interoperates with FRR's SPF (no wire change, so this validates the externally observable metric behaviour, not a negotiation) | |

> Interop is required because the externally observable Router-LSA metric (and the
> RFC 6138 transit-link presence) is what other routers act on. RFC 5443/6138 add
> no wire format, so the interop test validates that FRR's SPF reacts correctly to
> Ze's cost-out/restore (and the broadcast withhold), not a protocol negotiation.
> The raw-IP / multicast OSPF paths and the LDP TCP/UDP paths are Linux-only and
> run as QEMU integration tests (`ai/rules/qemu-testing.md`), consistent with the
> rest of the OSPF and LDP interop sets.

### Future (if deferring any tests)
- None. Every AC maps to a unit, functional, or interop test above. End-of-LIB
  (RFC 5919) deterministic sync is out of scope (no LDP support); when added it
  becomes an alternative condition-3 trigger tested in that future spec.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/plugins/ospf/config.go` -- `interfaceConfig` gains `LDPSyncEnabled bool` + `LDPSyncHoldDown` (seconds); parse the `ldp-sync` container in the interface-list parser; keep `Cost` as the restore value
- `internal/plugins/ospf/instance.go` -- the per-interface LDP-sync machine map + lifecycle; `lsdbTopology()` reads each interface's sync state and sets the effective `InterfaceInfo.Cost` (LSInfinity while not Synchronized) and a per-`InterfaceInfo` withhold flag for broadcast; `subscribeLDPSyncEvents(eb)`; call `originateSelfLSAs()` on every state change; reset on interface-down
- `internal/plugins/ospf/lsdb/origination.go` -- `routerLinks()` honours a per-`InterfaceInfo` "withhold transit link" flag (RFC 6138 broadcast) in the `NetworkBroadcast` branch; the per-interface LSInfinity substitution arrives via `InterfaceInfo.Cost` already (P2P)
- `internal/plugins/ospf/lsdb/origination.go` `InterfaceInfo` (struct) -- add `LDPSyncWithholdTransit bool` (broadcast withhold signal)
- `internal/plugins/ospf/spf/` (the computer) -- a read-only `IsCutEdge(interfaceName)` query over the last SPF result; flush a pending SPF first (RFC 6138 App A)
- `internal/plugins/ospf/register.go` -- wire `subscribeLDPSyncEvents(getEventBus())` next to `subscribeIfaceEvents`; keep + call the `unsubscribe` on shutdown/reconcile
- `internal/plugins/ospf/cmd_show.go` -- `show ospf ldp-sync` handler + snapshot
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `ldp-sync` container under `interfaces/interface`
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` -- the `show ospf ldp-sync` command
- `internal/plugins/ldp/events.go` -- `SessionEvent` gains `Interface string` (`json:"interface"`)
- `internal/plugins/ldp/discovery.go` -- `Adjacency` gains an interface-name field (carried from discovery)
- `internal/plugins/ldp/register.go` -- carry `ifName` from `discoverOnInterface` into `Adjacency` and `startSessionForAdj`; populate `SessionEvent.Interface` on `SessionUp`/`SessionDown`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- `ldp-sync` container (`enable` boolean, `holddown` seconds); read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `enable` native boolean; `holddown` `type uint16 { range "0..65535"; } units seconds` |
| YANG custom validators | [ ] no | native types suffice |
| CLI commands/flags | [ ] yes | `show ospf ldp-sync` in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ospf ldp-sync` |
| Editor autocomplete | [ ] yes | automatic for the YANG boolean/uint16 + the new show subcommand |
| Functional test for new RPC/API | [ ] yes | `test/ospf/ospf-ldp-sync-*.ci` |
| Pipe completeness | [ ] yes | `show ospf ldp-sync` routes through `ApplyPipes` like the other show outputs |
| Env var registration | [ ] no | `ldp-sync` is per-interface operational config, not an `environment/` leaf |
| Doctor check for runtime dependencies | [ ] no | no new socket/port/binary/cert; the LDP coupling is the existing in-process EventBus |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospf_ldp_sync_state` | gauge (0=not-sync,1=hold-down,2=sync) | `interface` |
| `ze_ospf_ldp_sync_transitions_total` | counter | `interface`, `to` |
| `ze_ospf_ldp_sync_holddown_expired_total` | counter | `interface` |
| `ze_ospf_ldp_sync_costout_seconds` | gauge | `interface` |

> These extend the umbrella's canonical OSPF metric set with the
> `ze_ospf_ldp_sync_*` prefix, registered by this spec's owner code. The umbrella
> "Metrics" table must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPF LDP-IGP synchronization |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` -- the `ldp-sync` container |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ospf ldp-sync` |
| 4 | API/RPC added/changed? | [ ] no | the show RPC lives in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an LDP-sync consumer; LDP session event gains an interface field |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` -- an LDP-IGP sync section |
| 7 | Wire format changed? | [ ] no | RFC 5443/6138 define no wire format; nothing to document under wire |
| 8 | Plugin SDK/protocol changed? | [ ] yes | the LDP `SessionEvent` gains `Interface`; note it for event consumers |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc5443.md` + `rfc/short/rfc6138.md` -- flip the compliance items this spec satisfies |
| 10 | Test infrastructure changed? | [ ] yes (interop scenario added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPF LDP-IGP sync parity with FRR |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc -- the per-interface sync machine + LDP event coupling |
| 13 | Route metadata keys added/changed? | [ ] no | LDP-sync changes a metric, installs no route metadata key |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPF telemetry doc -- the four `ze_ospf_ldp_sync_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table; the LDP `SessionEvent` field |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed OSPF/LDP files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPF interface config examples against the new `ldp-sync` container |

## Files to Create
- `internal/plugins/ospf/ldp_sync.go` -- the per-interface LDP-sync state machine (states, hold-down timer, transition logic, alert), the EventBus subscriber (`subscribeLDPSyncEvents`), the effective-cost / withhold query, and the `show ospf ldp-sync` snapshot
- `internal/plugins/ospf/ldp_sync_test.go` -- the unit tests for the machine, subscription, restore, no-op, alert, unsubscribe, TE-cost-untouched
- `internal/plugins/ospf/lsdb/ldp_sync_origination_test.go` -- broadcast withhold / cut-edge / P2P-LSInfinity origination tests
- `internal/plugins/ospf/spf/ldp_sync_cutedge_test.go` -- the cut-edge query + fresh-SPF tests
- `test/ospf/ospf-ldp-sync-config.ci`, `ospf-ldp-sync-restore.ci`, `ospf-ldp-sync-down.ci`, `ospf-ldp-sync-broadcast.ci`, `ospf-ldp-sync-show.ci`
- `test/interop/scenarios/ospf-ldp-sync-frr/` -- `ze.conf`, `frr.conf` (ospfd + ldpd), `check.py`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm `LSInfinity`, `routerLinks()`, `lsdbTopology()`, the LDP `SessionEvent`, and the `Subscribe` pattern exist |
| 3. Wiring phase | Wiring Test table -- config + subscriber + failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | from critical review |
| 9. Re-verify | re-run stage 6 |
| 10. Repeat 7-9 | until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | re-run stage 6 |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- config surface + the LDP event coupling + failing wiring tests
   - Tests: `TestLDPSyncConfigCreatesMachine`, `TestLDPSyncSubscribesSessionEvents`, `TestLDPSessionEventCarriesInterface`, `test/ospf/ospf-ldp-sync-config.ci`
   - Files: `yang/ze-ospf-conf.yang` (`ldp-sync` container), `config.go` (`interfaceConfig` fields + parse), `ldp/events.go` (`SessionEvent.Interface`), `ldp/discovery.go` + `ldp/register.go` (carry `ifName`), `ospf/ldp_sync.go` (machine skeleton + `subscribeLDPSyncEvents`), `ospf/register.go` (wire the subscription)
   - Verify: an `ldp-sync` interface creates a NotSynchronized machine; a published `SessionUp` reaches the subscriber; the LDP event carries the interface; restore/withhold logic still stubbed so deeper tests fail
2. **Phase: P2P cost override + restore** -- LSInfinity while not synchronized, configured cost after
   - Tests: `TestLDPSyncForcesMaxMetric`, `TestLDPSyncMaxMetricValue`, `TestLDPSyncRestoresAfterHoldDown`, `TestLDPSyncRestoresConfiguredCost`, `TestLDPSyncSessionDownForcesMaxMetric`, `TestLDPSyncHoldDownConfigurable`, `TestLDPSyncP2PMaxMetricNotWithheld`
   - Files: `instance.go` (`lsdbTopology()` effective cost from sync state; re-originate on transition), `ldp_sync.go` (hold-down timer, transitions)
   - Verify: NotSynchronized/HoldDown -> 0xFFFF; hold-down expiry -> configured cost; session-down -> 0xFFFF; stored cost never overwritten
3. **Phase: Broadcast handling (RFC 6138) + cut-edge** -- withhold the transit link unless cut-edge
   - Tests: `TestLDPSyncBroadcastWithholdsTransitLink`, `TestLDPSyncBroadcastCutEdgeAdvertised`, `TestLDPSyncCutEdgeUsesFreshSPF`, `ospf-ldp-sync-broadcast.ci`
   - Files: `lsdb/origination.go` (`InterfaceInfo.LDPSyncWithholdTransit` honoured in the `NetworkBroadcast` branch), `spf/` (`IsCutEdge` + pending-SPF flush), `instance.go` (set the withhold flag from sync state + cut-edge)
   - Verify: non-cut-edge broadcast withholds only the transit link; cut-edge advertises immediately; a pending SPF is flushed first
4. **Phase: Lifecycle, alert, no-op, unsubscribe** -- robustness
   - Tests: `TestLDPSyncResetsOnInterfaceDown`, `TestLDPSyncDisabledIsNoOp`, `TestLDPSyncUnsubscribesOnShutdown`, `TestLDPSyncStuckRaisesAlert`, `TestLDPSyncTECostUntouched`
   - Files: `instance.go` (interface-down reset; shutdown/reconcile unsubscribe), `ldp_sync.go` (the §3 alert + counter)
   - Verify: interface-down resets; no `ldp-sync` = no-op; subscription removed on shutdown; persistent stuck raises the alert; no TE-cost write
5. **Phase: CLI + metrics** -- user surface
   - Tests: `ospf-ldp-sync-show.ci`, the config/restore/down `.ci`
   - Files: `cmd_show.go`, `yang/ze-ospf-cmd.yang`, the four metric series registration
   - Verify: `show ospf ldp-sync` lists state + remaining hold-down + effective metric; the four series are registered
6. **Functional tests** -> the five `.ci` cover the user-visible behaviour
7. **RFC refs** -> add `// RFC 5443 Section 2/3/4` and `// RFC 6138 Section 4 / Appendix A` comments on the cost-out, restore, broadcast-withhold, cut-edge, and IP-only enforcement code
8. **Interop** -> `ospf-ldp-sync-frr` QEMU scenario (ospfd + ldpd)
9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has file:line implementation |
| Feature completeness | each user story has a working path; parity with FRR's RFC 5443/6138 LDP-sync (cost-out + restore + broadcast withhold), RSVP-TE excluded by design |
| Correctness | max value exactly `0xFFFF` (LSInfinity); restore uses the configured cost not 1/0xFFFF; hold-down honours config with no hardcoded default; cut-edge MUST-advertise; pending-SPF flush; TE cost never touched |
| Naming | `ze_ospf_ldp_sync_*` metrics; YANG `ldp-sync`/`holddown`/`enable` kebab-case; `SessionEvent.Interface` |
| Data flow | override computed at origination (`lsdbTopology`/`routerLinks`), never written into stored cost; OSPF<->LDP only via `ze.EventBus`; no LDP internal import |
| CLI grammar | `show ospf ldp-sync` action-before-identifier |
| Doctor checks | none added (no new runtime dependency) -- confirm |
| YANG validation | `enable` boolean; `holddown` uint16 range 0..65535 units seconds |
| Prometheus counters | the four series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | OSPF compiles without LDP; `ldp-sync` disabled is byte-for-byte today; LDP unaware of OSPF |
| Rule: no-workarounds | condition 3 estimated by the documented hold-down (RFC-sanctioned), not by faking End-of-LIB; the restore waits for the real timer |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Per-interface LDP-sync machine + override | `go test ./internal/plugins/ospf -run TestLDPSync` |
| LSInfinity while not synchronized | `go test ./internal/plugins/ospf -run TestLDPSyncForcesMaxMetric` |
| Configured-cost restore after hold-down | `go test ./internal/plugins/ospf -run TestLDPSyncRestoresConfiguredCost` |
| Broadcast withhold + cut-edge | `go test ./internal/plugins/ospf/lsdb -run TestLDPSyncBroadcast && go test ./internal/plugins/ospf/spf -run TestLDPSyncCutEdge` |
| LDP event carries the interface | `go test ./internal/plugins/ldp -run TestLDPSessionEventCarriesInterface` |
| Four metric series registered | `grep -rn 'ze_ospf_ldp_sync_' internal/plugins/ospf` |
| `show ospf ldp-sync` present | `grep -rn 'ldp-sync' internal/plugins/ospf/yang/ze-ospf-cmd.yang internal/plugins/ospf/cmd_show.go` |
| Functional tests present | `ls test/ospf/ospf-ldp-sync-*.ci` |
| Interop scenario present | `ls test/interop/scenarios/ospf-ldp-sync-frr/` |
| OSPF compiles without LDP | build the OSPF package without the LDP plugin import |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the LDP `SessionEvent.Interface` is matched against configured `ldp-sync` interfaces only; an unmatched/empty interface is logged + metered, never used to mutate an arbitrary interface |
| Resource exhaustion | a flapping LDP session cannot spin re-origination beyond MinLSInterval throttling; the hold-down timer is per interface and bounded; no unbounded timer/goroutine growth across reconciles |
| Availability (RFC 5443 §5) | cost-out is internal to the router; costing out many links degrades capacity -- the §3 alert + the `costout_seconds` gauge surface a stuck condition so an operational/implementation error is visible |
| Partition safety (RFC 6138 §4) | the cut-edge MUST-advertise rule is enforced; if cut-edge cannot be determined, default to advertise (never withhold) so a bug cannot partition the network |
| Subscription lifecycle | the EventBus `unsubscribe` is always called on shutdown/reconcile (no stale handler reading freed state) |
| Error leakage | sync-state and unmatched-event errors are logged/metered locally, never reflected to peers (no wire surface exists) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -->

## Core Insight
LDP-IGP sync is a *metric-origination* problem, not a protocol problem: RFC
5443/6138 add no wire format, so the entire feature is a per-interface effective
cost (LSInfinity while not synchronized, configured cost after) substituted at the
same `routerLinks()` seam the codebase already uses for the router-wide RFC 6987
max-metric, plus the RFC 6138 broadcast refinement (withhold the transit link
unless cut-edge). The only cross-plugin coupling is reading the LDP plugin's
`SessionUp`/`SessionDown` events off the public `ze.EventBus`, which first
requires tagging those events with the interface name the LDP discovery layer
already knows.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Override the effective cost at origination time, never the stored configured cost | mutate `interfaceConfig.Cost` to LSInfinity and back | mutating the stored cost risks the R-2 trap (permanent cost-out) and loses the restore value; origination-time substitution mirrors how `in.MaxMetric` already works |
| Per-interface sync machine, separate from the OSPF ISM | fold sync state into the OSPF interface state machine | LDP-sync is orthogonal to the OSPF ISM (an interface can be DR and not-synchronized); a separate machine keeps the ISM untouched and reusable for OSPFv3 later |
| Add `Interface` to the LDP `SessionEvent` | have OSPF reverse-map the transport address to an interface | the discovery layer already knows `ifName`; an additive event field is robust and avoids brittle address->interface resolution |
| Hold-down timer for condition 3 | wait for LDP End-of-LIB (RFC 5919) | the LDP plugin has no End-of-LIB; the hold-down timer is the RFC-5443-sanctioned estimation; End-of-LIB is a clean future swap |
| Reuse the last SPF result for cut-edge | run a dedicated cut-edge SPF | RFC 6138 Appendix A explicitly says no extra SPF; flush a pending SPF and read the existing result |
| Broadcast withhold uses LSInfinity ONLY for P2P, withhold for broadcast | max-metric the broadcast pseudonode cost | a broadcast pseudonode carries a single cost that cannot avoid one peer; RFC 6138 withholds the whole transit link instead |
| RSVP-TE sync out of scope | implement both LDP and RSVP-TE sync | the task scopes to LDP; Ze has an LDP plugin and no RSVP-TE label distribution to sync against |

## Known Limitations
- Condition 3 ("all label bindings exchanged") is estimated by the hold-down
  timer, not confirmed by End-of-LIB (RFC 5919 not implemented in the LDP
  plugin); a too-short hold-down can re-introduce a transient black hole.
- OSPFv3 LDP-sync is not implemented (OSPFv2 only); the state machine is written
  to be reusable by a later OSPFv3 spec.
- IS-IS LDP-sync (the IS-IS half of RFC 5443, max-metric `0xFFFFFE`) is a
  separate `isis`-plugin consumer, not this spec.
- RSVP-TE IGP sync and targeted-LDP-over-TE-tunnel sync (RFC 5443 §4) are out of
  scope (no TE tunnels in Ze).
- RFC 6138 Appendix B (sync where the far end supports no method) is "for further
  study" and not implemented.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code:
- RFC 5443 §2 -- "the IGP will advertise the link with maximum cost to avoid any transit traffic over it" (the LSInfinity substitution while not synchronized)
- RFC 5443 §2 -- the three "fully operational" conditions + "use a configurable hold-down timer to allow LDP session establishment" (the HoldDown sub-state)
- RFC 5443 §3 -- "an implementation should issue network management alerts" (the stuck-cost-out alert)
- RFC 5443 §4 -- "should only be applied to the IP link cost" (the TE-cost-untouched guarantee)
- RFC 6138 §4 -- "the Router-LSA is not updated with a 'Link Type 2' ... until LDP is operational with all neighboring routers" (the broadcast withhold)
- RFC 6138 §4 -- the MUST NOT: a cut-edge "MUST NOT be delayed by LDP's operational state" (the cut-edge immediate-advertise)
- RFC 6138 Appendix A -- "that SPF MUST be executed immediately before any procedure checks whether an interface is a 'cut-edge'" (the pending-SPF flush)

## Implementation Summary

### What Was Implemented
- [filled at implementation time]

### Bugs Found/Fixed
- [filled at implementation time]

### Documentation Updates
- [filled at implementation time]

### Deviations from Plan
- [filled at implementation time]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Hold link cost at max-metric until LDP synced | unit + functional | `TestLDPSyncForcesMaxMetric`, `ospf-ldp-sync-config.ci` |
| Restore real cost once synced | unit + functional + interop | `TestLDPSyncRestoresConfiguredCost`, `ospf-ldp-sync-restore.ci`, `ospf-ldp-sync-frr` |
| Re-force max-metric on LDP session loss | unit + functional | `TestLDPSyncSessionDownForcesMaxMetric`, `ospf-ldp-sync-down.ci` |
| Broadcast handling per RFC 6138 (withhold/cut-edge) | unit + functional | `TestLDPSyncBroadcastWithholdsTransitLink`, `TestLDPSyncBroadcastCutEdgeAdvertised`, `ospf-ldp-sync-broadcast.ci` |
| Integrate with the LDP plugin sync state | unit | `TestLDPSyncSubscribesSessionEvents`, `TestLDPSessionEventCarriesInterface` |
| Hold-down timer (configurable, no default) | unit | `TestLDPSyncHoldDownConfigurable`, `TestLDPSyncRestoresAfterHoldDown` |
| CLI + metrics | functional | `ospf-ldp-sync-show.ci`; `grep ze_ospf_ldp_sync_` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE |  | file:line |  |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`, `internal/plugins/ldp/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 5443 / RFC 6138 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the machine serves OSPFv2 now; OSPFv3 reuse is noted, not pre-built)
- [ ] No speculative features (no End-of-LIB, no RSVP-TE, no IS-IS)
- [ ] Single responsibility per component (the sync machine owns state; origination owns the metric)
- [ ] Explicit > implicit behavior (override computed at origination, not via hidden cost mutation)
- [ ] Minimal coupling (OSPF<->LDP only via the public EventBus)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-ldp-sync-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospf-ext-11-ldp-igp-sync.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-ext-11-ldp-igp-sync.md`
