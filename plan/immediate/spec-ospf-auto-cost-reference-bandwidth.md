# Spec: ospf-auto-cost-reference-bandwidth

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-09-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** The `reference-bandwidth` leaf of
`internal/plugins/ospf/yang/ze-ospf-conf.yang` carries the description
"Auto-cost reference bandwidth in Mbps.", the unit Mbps, and the default 100000.
A reader takes that as the numerator of the auto-cost convention: Ze divides this
bandwidth by the speed of each link and advertises the quotient as the cost of
that link. Under that reading a 10 Gbit/s interface costs less than a 1 Gbit/s
interface with no per-interface configuration. Raising the leaf then spreads the
costs of a fast network apart.

**What Ze does instead.** `applyTree` (`internal/plugins/ospf/config.go`) writes
the value into `cfg.ReferenceBandwidth`. That field has no reader. Every other
mention of it in the tree is in `internal/plugins/ospf/config_test.go`. The
function the leaf exists for is `types.DefaultMetric`
(`internal/plugins/ospf/types/metric.go`), which divides a reference bandwidth by
an interface bandwidth and floors the result at 1. Its only callers are in
`internal/plugins/ospf/types/metric_test.go`, so no production path reaches it.
The cost Ze advertises comes from `interfaceRuntimeConfigLocked`
(`internal/plugins/ospf/instance.go`), which takes `ic.Cost` and substitutes 1
when `ic.HasCost` is false. Ze reads the speed of no link, and an interface with
no explicit `cost` advertises 1 at every speed. RFC 2328 Appendix C.3 permits
that outcome, because it states only that "The interface output cost must always
be greater than 0" and defines no derivation from a link speed. The `ze:help`
beside the leaf already discloses the gap, in the sentence "This leaf changes no
advertised metric." The `description` still names auto-cost.

**What closing it means.** The implementer either builds auto-cost or refuses the
leaf at commit, the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses the `vrf` leaf. Neither is
obviously right, and the two differ in cost by a wide margin. Building auto-cost
needs a link speed inside the OSPF plugin. The one producer of a link speed in
the tree is `LinkSpeedDuplex` (`internal/plugins/iface/netlink/show_linux.go`),
which reads `/sys/class/net/<name>/speed`, answers in Mbit/s, and answers 0 for a
virtual device or a down link. That value crosses a plugin boundary the OSPF
plugin does not cross today. The design must also decide what a 0 speed derives,
and when a derived cost is recomputed, because a link speed changes while OSPF
runs. Refusing costs one validator and one registration, and it removes the false
promise at once. Auto-cost is a feature an operator expects from an OSPF
implementation, so a refusal is the placeholder rather than the answer, and this
spec then stays open. `plan/spec-vrf.md` records that arrangement for the `vrf`
leaf.

## SETTLED by the owner: a `reference-bandwidth` commit costs no adjacency (2026-09-09)

**Thomas decided on 2026-09-09: re-price in place. A `reference-bandwidth`
commit MUST NOT tear down an established adjacency.** This closes round 2's
ISSUE 1 and replaces the open question that stood here. The behavior is
implemented, and no later reader re-opens it.

| | What Ze did until 2026-09-09 | What Ze does now |
|---|------------------------------|------------------|
| At the commit | Every interface whose derived cost changed was restarted, which dropped its neighbors. A router with 40 auto-costed 10 Gbit/s links dropped all 40, each re-forming over a dead interval plus a database exchange | `reconcile` pushes the new cost into the running interface through `repriceInterfaceLocked` and `(*Interface).SetCost`. No neighbor moves |
| What reaches the wire | The re-priced metric, on the origination pass that followed the restart | `reconcile` requests origination before it returns. The maintenance timer retries publication after MinLSInterval when the immediate attempt is rate-limited |

The evidence that the restart never published the cost is one line, and it was
already in this spec: a carrier flap re-prices a link with no restart at all,
because `lsdbTopology` derives `InterfaceInfo.Cost` from
`e.cfg.ReferenceBandwidth` and the live link speed on every origination pass,
and `reconcile` replaces `e.cfg` before it touches an interface. The restart
refreshed one reader only, the `Interface.cfg.Cost` copy that `snapshotLocked`
and `DetailSnapshot` hand to `show ospf interface`. `SetCost` refreshes that
copy without stopping anything.

**Which parameters still force a restart, established by reading the code.**
`Cost` is the ONLY field of `ospfiface.Config` that no running behavior reads:
its two readers in `internal/plugins/ospf/iface/` are `snapshotLocked` and
`DetailSnapshot`. Every other field is stamped into the ISM at `Start`
(`Passive`, `NetworkType`, `Priority`, `HelloInterval`, `DeadInterval`), into
each Hello (`RouterID`, `AreaID`, `NetworkMask`, `InterfaceAddress`,
`InstanceID`, `InterfaceID`, `IsV6`, `NBMANeighbors`, `PollInterval`), or into
the neighbor table's interface record through `neighborInterfaceConfig`
(`AreaType`, `InterfaceMTU`, `MTUIgnore`, `RetransmitInterval`, and the four
BFD fields). So `interfaceGlobalParamsChanged` keeps both of its remaining
arms, the Router ID and the area type, and `interfaceParamsEqual` is untouched.

**One sibling stays as it was, and it is the owner's to decide.** A change to an
interface's own `cost` leaf still restarts that interface, because
`interfaceParamsEqual` compares `Cost` and `HasCost`. It is the same question
Thomas answered for the router-wide leaf, asked of the per-interface one, and
this session did not extend his answer to it.

## Progress (2026-09-06)

Auto-cost is BUILT, tested, proven against FRR, and documented. The package
compiles and the OSPF unit suite and `./le functional ospf` are green. The
earlier "DOES NOT COMPILE" note above was written from editor diagnostics and was
already stale: `interfaceCost` had been removed from `te_originate.go`, so
nothing was redeclared.

Two claims in the Task section above turned out to be WRONG, and both were load
bearing:

- "That value crosses a plugin boundary the OSPF plugin does not cross today" is
  false. `internal/plugins/ospf/interface_addr.go` and `origination_v6.go`
  already import `internal/component/iface`, so the boundary is crossed on the
  same origination pass.
- The design's first draft asserted that no synthetic device reports a link
  speed. A veth that is UP reports **10000** Mbit/s, because the veth driver
  declares 10 Gbit/s full duplex. That is what makes auto-cost observable in a
  Docker container, and it is why an interop scenario can prove this feature at
  all. A loopback and a dummy report nothing; a bridge reports -1.

**Not committed with this spec:** the interop scenario's registration. See
Known Limitations.

## Progress (2026-09-08, design re-verification)

Every claim in this spec was re-read at its producer. Three corrections landed in
the sections below, and none of them changes the design.

- The "NOT COMMITTED" note is stale. `1753e1f4c9` landed the derivation and
  `178f73002c` landed both scenario directories, after the RFC 7854 session
  committed the two checker maps. `internal/le/interoplab/bgp/checkers.go` and
  `check_extras.go` both carry an `ospf-auto-cost-frr` entry at HEAD.
- The unknown-speed fallback is reached by two whole dataplanes, not only by a
  loopback or a tunnel. `stubBackend.LinkSpeedDuplex`
  (`internal/plugins/iface/netlink/backend_other.go`) and
  `vppBackendImpl.LinkSpeedDuplex` (`internal/plugins/iface/vpp/query.go`) each
  answer 0, and `iface.LinkSpeedDuplex` (`internal/component/iface/dispatch.go`)
  answers 0 when no backend is loaded. Auto-cost therefore prices nothing on a
  non-Linux host and nothing on a VPP dataplane. Known Limitations records it and
  AC-5 is widened to name it.
- A `reference-bandwidth` change drops the adjacencies on each interface it
  re-prices. `reconcile` calls `startInterfaceLocked`
  (`internal/plugins/ospf/instance.go`), which calls `Stop` on the interface
  runtime it replaces, and `iface.(*Interface).Stop`
  (`internal/plugins/ospf/iface/iface.go`) empties the neighbor map, clears the
  DR and the BDR, and calls `neighborSink.InterfaceDown`. AC-8 and the decision
  table now state that consequence.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/ospf/ospf-4-component-config.md` - declared by `interface_cost.go`'s `// Design:` header
  → Decision: one function answers what an interface costs, and every consumer calls it; four copies of "cost, or 1 when unset" existed before
  → Constraint: the link speed is read on each origination pass rather than cached, so a renegotiating link re-prices with nothing to invalidate
- [ ] `docs/guide/ospf.md` - the operator-facing description of interface cost
  → Constraint: the guide states the clamp, the unknown-speed fallback and the reload behavior, so each is a published promise
- [ ] `docs/architecture/ospf/ospf-5-interface-ism.md` - declared by `internal/plugins/ospf/iface/iface.go`, the package this spec's AC-8b proof drives
  → Constraint: "Constraints on callers" distinguishes Router ID and area-type changes from changes inside an interface block. A reference-bandwidth-only change preserves the runtime; the interface's own cost leaf still restarts it.
- [ ] `docs/architecture/ospf/ospf-1-types.md` - declared by `types/metric.go` and `types/metric_test.go`
  → Constraint: "Constraints on callers" gives the leaf package ownership of metrics and forbids a higher OSPF package from redeclaring them, which is the rule the four deleted cost copies broke. "The package has no runtime dependency" keeps `DefaultMetric` a pure arithmetic function, so `interfaceLinkSpeedMbps` reads the link outside `types` and passes the speed in.
- [ ] `docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md` - declared by `ldp_sync.go`, whose cost copy this spec deletes
  → Constraint: "Restore recomputes the configured cost at origination time. The stored cost is never overwritten." Deleting the copy must keep that property: restore calls the one cost function and re-derives, so a link that renegotiated its speed while held out is priced at its new speed. "Point-to-point cost-out must NOT override the interface cost" keeps max-metric a per-interface FLAG on the transit link, not a written cost.
- [ ] `docs/architecture/ospf/ospf-ext-2-traffic-engineering.md` - declared by `te_originate.go`, where the TE metric fallback is threaded
  → Constraint: "Origination is pull-model through the carrier. A withdraw diff on unchanged config floods nothing." The TE metric fallback therefore reads the derived cost at origination rather than snapshotting it, which matches the no-cache decision above, and a cost that moves with link speed produces a real diff and does flood.
- [ ] `docs/architecture/testing/interop.md` - declared by `internal/le/interoplab/bgp/checkers.go` and `check_extras.go`, both of which this spec edits
  → Constraint: "Typed checker operations" makes `checkers.go` the complete scenario catalogue, and an absent value never proves a negative assertion by itself: the operation must also name positive evidence that the query mechanism ran. The `ospf-auto-cost-frr` scenario asserts a cost VALUE FRR reports, which is a positive, so the trap it must avoid is a scenario that would pass at any speed.
- [ ] `docs/features/interfaces.md` - declared by `internal/plugins/iface/netlink/show_linux.go`
  → Constraint: the Physical Layer row records "Speed / duplex / autoneg" as `missing`, and that row is about the OPERATOR surface (it sits beside ethtool integration and ring buffer sizing). `LinkSpeedDuplex` is an internal sysfs read whose doc comment already names OSPF auto-cost as a consumer, so this spec adds a consumer and no operator-facing speed surface. The row is NOT flipped by this work.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2328.md` - Appendix C.3 defines the interface output cost
  → Constraint: "The interface output cost must always be greater than 0". RFC 2328 defines NO derivation from a link speed, so auto-cost is a convention rather than a requirement, and no RFC requirement id is claimed by this spec.
- [ ] `rfc/short/rfc3630.md` - section 2.5.5, the TE metric defaults to the OSPF interface cost
  → Constraint: an interface with no `te-metric` now falls back to the DERIVED cost, which is the same number the Router-LSA carries

**Key insights:** (minimal context to resume after compaction)
- A veth that is UP reports 10000 Mbit/s in sysfs. A Docker container's eth0 is a veth, so an interop scenario can observe the derivation.
- `defaultOSPFConfig()` sets `ReferenceBandwidth` to 100000, so auto-cost is ON by default and a 10 Gbit/s link costs 10 rather than 1.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ospf/config.go` - `applyTree` writes `reference-bandwidth` into `cfg.ReferenceBandwidth`; `defaultOSPFConfig` seeds it with 100000. Nothing read the field
- [ ] `internal/plugins/ospf/instance.go` - `interfaceRuntimeConfigLocked` and `lsdbTopology` each held their own "cost, or 1 when unset" copy
- [ ] `internal/plugins/ospf/te_originate.go` - held a third copy as an `interfaceCost` helper for the RFC 3630 TE metric fallback
- [ ] `internal/plugins/ospf/ldp_sync.go` - held a fourth copy for the LDP-sync restore value
- [ ] `internal/plugins/ospf/types/metric.go` - `DefaultMetric` divides and floors at 1, and had no non-test caller
- [ ] `internal/component/iface/dispatch.go` - `LinkSpeedDuplex` answers Mbit/s and returns 0 when unknown or no backend is loaded
- [ ] `internal/plugins/iface/netlink/show_linux.go` - the backend reads `/sys/class/net/<name>/speed`, and a non-positive value becomes 0. The file is `//go:build linux`
- [ ] `internal/plugins/iface/netlink/backend_other.go` - `stubBackend.LinkSpeedDuplex` answers 0 on every other platform
- [ ] `internal/plugins/iface/vpp/query.go` - `vppBackendImpl.LinkSpeedDuplex` answers 0, because a VPP interface has no `/sys/class/net` entry
- [ ] `internal/plugins/ospf/iface/iface.go` - `(*Interface).Stop` empties the neighbor map, clears the DR and the BDR, and calls `neighborSink.InterfaceDown`, so restarting an interface drops its adjacencies
  → Constraint: `Cost` is the ONLY `Config` field no running behavior reads. Its two readers in this package are `snapshotLocked` and `DetailSnapshot`; every other field is stamped into the ISM at `Start`, into each Hello, or into the neighbor table through `neighborInterfaceConfig`. That is what makes `SetCost` safe and a general config setter unsafe

**Behavior to preserve:** (unless the user explicitly said to change it)
- An interface with an explicit `cost` advertises that cost, whatever the link speed.
- An interface whose link speed the kernel does not report advertises 1, which is what every unconfigured interface advertised before.
- `DefaultMetric` keeps its name and its floor at `MetricMin`.

**Behavior to change:** (only what the user asked for)
- An interface with no `cost` on a link the kernel prices is now costed `reference-bandwidth / speed`, clamped to 1..65535. Under the default 100000 a 10 Gbit/s link costs 10 rather than 1.
- `DefaultMetric` now takes both operands in Mbit/s (it took bits per second) and clamps at `MetricMax` instead of erroring above it. `types.DefaultReferenceBandwidth`, a bits-per-second constant with no non-test user, is deleted; `config.DefaultReferenceBandwidth` (Mbps) is the one declaration.
- A `reference-bandwidth` change re-prices interfaces through `repriceInterfaceLocked` and `(*ospfiface.Interface).SetCost` without restarting them (Thomas, 2026-09-09). `reconcile` requests self-LSA origination before it returns. The maintenance worker retries rate-limited publication, including with only passive or loopback interfaces.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `ospf/reference-bandwidth`, a uint32 in Mbps, range 1..4294967, default 100000.
- Kernel: `/sys/class/net/<name>/speed`, read through the iface component.

### Transformation Path
1. `applyTree` (`internal/plugins/ospf/config.go`) writes the leaf into `cfg.ReferenceBandwidth`.
2. `interfaceCost` (`internal/plugins/ospf/interface_cost.go`) returns `ic.Cost` when `ic.HasCost`, otherwise calls `interfaceLinkSpeedMbps` and `types.DefaultMetric`.
3. `interfaceLinkSpeedMbps` calls `ifcomp.LinkSpeedDuplex`, which reaches the netlink backend's sysfs read.
4. `types.DefaultMetric` divides and clamps to `[MetricMin, MetricMax]`, and returns `ErrOutOfRange` for a zero operand.
5. Five consumers read the result: `lsdbTopology` (what the Router-LSA advertises), `interfaceRuntimeConfigLocked` (what `show ospf interface` reports on a runtime being created), `repriceInterfaceLocked` (the same reader on a runtime already running), `updateLDPSyncMachines` (the restore value), and `applyTELinkAttributes` (the RFC 3630 TE metric fallback).
6. On a reload, `reconcile` re-prices every interface it does not recreate and then calls `originateSelfLSAs`, so the commit publishes the new metric.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF plugin ↔ iface component | `ifcomp.LinkSpeedDuplex(name)`, the same import `interface_addr.go` and `origination_v6.go` already make on the same pass | Yes -- `TestInterfaceLinkSpeedMbpsUnknownName` calls the production reader |
| Ze ↔ an outside OSPF implementation | The derived cost is what FRR reads out of Ze's Router-LSA | Yes -- `ospf-auto-cost-frr` |

### Integration Points
- `types.DefaultMetric` - the derivation that existed and had no production caller; this spec gives it one.
- `interfaceGlobalParamsChanged` - restarts an interface on a Router ID or area-type change. A re-pricing does NOT join that set (owner decision, 2026-09-09): `reconcile` calls `repriceInterfaceLocked` for it instead, and originates before it returns.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every consumer calls `interfaceCost`; no site reads `ic.Cost` for an advertised metric any more |
| No unintended coupling (components stay isolated) | Yes | `internal/component/iface` was already imported by two files in this plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | The change DELETES four copies of the cost rule and reuses `types.DefaultMetric` rather than writing a fifth |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Integer arithmetic, no buffers |
| Registration over hardcoding | N-A | No new command, view, family or handler |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The OSPF plugin may read a link speed from the iface component | `interface_addr.go` and `origination_v6.go` already import it | The design would need a new event or a cached field | Import read; `./le tier check` inside `./le verify` | confirmed -- the Task section's claim that the boundary is not crossed today is wrong |
| A-2 | A synthetic device reports no link speed, so auto-cost cannot be observed in a container | The first draft of `interface_cost.go` said so | An interop scenario is possible after all | Measured: a container reading `/sys/class/net/eth0/speed` answers 10000 | BROKEN. A veth reports 10000 Mbit/s. The claim was corrected in the code, the test helper, the guide and the iface backend's doc comment, and the interop scenario exists because of it |
| A-3 | The YANG default reaches the parser, so auto-cost is on by default | `defaultOSPFConfig()` sets `ReferenceBandwidth: DefaultReferenceBandwidth` (100000) | Auto-cost would be off unless configured and the leaf would still change nothing | Producer read at `internal/plugins/ospf/config.go:563`, and `TestOSPFTopologyCostFollowsReferenceBandwidth` derives 100 from a config that names the leaf | confirmed |
| A-4 | No existing interop scenario asserts a cost that auto-cost changes | 4 of the 30 OSPF scenarios set no explicit `cost`, and none asserts a metric | Existing scenarios would go red | Search over `test/interop/scenarios/ospf*/`, and `ospf-auth-frr` re-run green | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Every unconfigured interface changes cost on upgrade, so paths move | Route selection changes with no config change | This is the feature the leaf promised, and it is what every other OSPF implementation does. The guide states it and `cost` pins any interface an operator wants unchanged |
| R-2 | A link that renegotiates re-prices mid-session and churns LSAs | Router-LSA re-origination on a carrier event | The speed is read on the origination pass, so a re-price rides the LSA Ze was already going to send; nothing schedules an extra one |
| R-3 | Two sysfs reads per interface per origination pass cost time | Origination latency on a router with many interfaces | Measured against what the same pass already does: `lsdbTopology` performs five netlink address dumps per interface beside these two file reads |
| R-4 | A reference-bandwidth change bounces an adjacency that did not need it | An interface restarts on a numerator change | CLOSED. No reference-bandwidth change bounces any adjacency: `interfaceGlobalParamsChanged` no longer reads the numerator at all, and `reconcile` re-prices in place (owner decision, 2026-09-09). `TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth` pins both directions: the numerator never restarts, the Router ID and the area type always do |
| R-5 | An operator raises `reference-bandwidth` network-wide and every auto-costed adjacency in the OSPF domain re-forms at once | A burst of interface-down events at the moment of the commit | CLOSED by the owner's decision of 2026-09-09. No interface is restarted for a numerator change, so no adjacency re-forms and there is no burst. `TestReferenceBandwidthReloadKeepsNeighborAndReprices` holds a 2-Way neighbor across the reload and reads the re-priced metric off the Router-LSA |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every OSPF path decision. A wrong cost moves traffic, and a cost of 0 would be a malformed Router-LSA link |
| How is it reverted? | Single commit revert. No config migration: the leaf already parsed, and reverting returns every unconfigured interface to cost 1 |
| Who else touches this path? | `plan/immediate/spec-ospf-accept-lifetime-receive-window.md` ran in the same package at the same time and touches `internal/plugins/ospf/yang/ze-ospf-conf.yang` and `docs/guide/ospf.md`, in different hunks |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `reference-bandwidth 100000` in the config file | → | `applyTree` → `interfaceCost` → `lsdbTopology` | `TestOSPFTopologyCostFollowsReferenceBandwidth` |
| The same leaf reaching `show ospf interface` | → | `interfaceRuntimeConfigLocked` → `iface.Snapshot.Cost` | `TestOSPFTopologyCostFollowsReferenceBandwidth`, snapshot assertion |
| A config reload lowering the leaf | → | `reconcile` → `repriceInterfaceLocked` → `originateSelfLSAs` | `TestReferenceBandwidthReloadKeepsNeighborAndReprices` |
| An OSPF peer reading Ze's Router-LSA | → | the derived metric on the wire | `ospf-auto-cost-frr` interop scenario |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An interface with no `cost` on a link the kernel prices | Cost is `reference-bandwidth / speed`, truncated |
| AC-2 | A link at or above the reference bandwidth | Cost is 1, never 0 |
| AC-3 | A link slow enough that the quotient exceeds 65535 | Cost is 65535 |
| AC-4 | An interface with an explicit `cost` | That cost, whatever the link speed and whether the speed is known |
| AC-5 | An interface whose link speed the backend does not report: a loopback, a dummy, a bridge, a tunnel, a link with no carrier, any interface on a VPP dataplane, and every interface when no iface backend is loaded | Cost is 1 |
| AC-6 | A `reference-bandwidth` of 0 | Cost is 1 |
| AC-7 | The default config, no `reference-bandwidth` stated | 100000 applies, so a 1 Gbit/s link costs 100 |
| AC-8 | A reload lowering `reference-bandwidth` | An interface whose derived cost changes is re-priced in place and keeps its adjacency. An explicit `cost` remains unchanged. `reconcile` requests origination before it returns. If MinLSInterval prevents publication, the maintenance timer publishes the latest cost after the interval, including with only passive or loopback interfaces |
| AC-8b | The same reload, on an interface that already holds a 2-Way neighbor and sets no `cost` | The neighbor stays 2-Way, the interface runtime is the same object, and both the Router-LSA link metric and the cost `show ospf interface` reports carry the new number. This is the owner's decision of 2026-09-09: a `reference-bandwidth` commit tears down no adjacency. The neighbor the unit test drives really reaches 2-Way, because its Hello lists this router and that is what `receiveHello` reads for `TwoWay`; the test asserts the state out of the neighbor table on both sides of the reload |
| AC-8c | A reload that changes the Router ID, or the type of the area an interface sits in | That interface IS recreated and its neighbor drops to Down. Both are stamped into the packets the runtime sends, so a stale runtime would advertise a stale identity or a stale E-bit. These two are the whole set OUTSIDE the interface's own block, which is what `interfaceGlobalParamsChanged` decides; `reconcile` ORs that predicate with `interfaceParamsEqual`, so a change to a field of the interface's own block, `cost` among them, still restarts that interface |
| AC-9 | The same interface read four ways | The Router-LSA metric, `show ospf interface`, the LDP-sync restore value and the TE metric fallback all carry the same number |
| AC-10 | An OSPF peer reads Ze's Router-LSA | The peer sees the derived cost, not 1 |
| AC-11 | The same link configured under `ospf` and under `address-family { ipv6 }`, with a non-default `reference-bandwidth` | Both families advertise the same derived cost. Every RFC 5838 address family carries the router-wide numerator |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs OSPF on a mixed-speed network with no per-interface cost and gets the fast path | config -> interfaceCost -> lsdbTopology -> Router-LSA | `ospf-auto-cost-frr` |
| 2 | Raises `reference-bandwidth` to spread a fast network's costs apart | config reload -> reconcile -> interfaceGlobalParamsChanged -> re-origination | `TestOSPFTopologyCostFollowsReferenceBandwidth` |
| 3 | Pins one interface with `cost` and leaves the rest derived | config -> interfaceCost, HasCost branch | `TestInterfaceCostAutoDerivation`, "explicit cost wins" |
| 4 | Reads the cost Ze decided | `show ospf interface` rendered as JSON | `ospf-auto-cost-frr`, the `"cost": 47` assertion |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInterfaceCostAutoDerivation` | `internal/plugins/ospf/interface_cost_test.go` | AC-1 to AC-6, eleven cases over a stubbed speed table | pass |
| `TestInterfaceLinkSpeedMbpsUnknownName` | `internal/plugins/ospf/interface_cost_test.go` | the production reader answers 0 for a name the kernel does not know | pass |
| `TestOSPFTopologyCostFollowsReferenceBandwidth` | `internal/plugins/ospf/interface_cost_test.go` | AC-7, AC-8, AC-9: the wiring test, from the real config parser to the origination topology and the snapshot | pass |
| `TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth` | `internal/plugins/ospf/interface_cost_test.go` | AC-8 and AC-8c: a reference-bandwidth change never restarts an interface, and a Router ID change and an area-type change both do | pass |
| `TestReferenceBandwidthReloadRepricesEveryAddressFamily` | `internal/plugins/ospf/interface_cost_test.go` | AC-8b and AC-11 on the reload path: an OSPFv3 address-family engine reconciled with the `v6Families` entry keeps eth0's runtime, and both its origination topology and its interface snapshot carry the re-priced 23 | pass |
| `TestReferenceBandwidthReachesEveryAddressFamily` | `internal/plugins/ospf/interface_cost_test.go` | AC-11: one link, `reference-bandwidth 470000` and a 10 Gbit/s speed, read out of the OSPFv2 origination topology and out of the OSPFv3 engine's, plus the numerator every entry of `v6Families` carries | pass |
| `TestReferenceBandwidthReloadKeepsNeighborAndReprices` | `internal/plugins/ospf/interface_cost_test.go` | AC-8b and AC-8c through the production path: `parseOSPFConfig`, `reconcile`, `interfaceGlobalParamsChanged`, `repriceInterfaceLocked`, `originateSelfLSAs`. The interface holds a 2-Way neighbor, the reload keeps it and keeps the runtime, the Router-LSA link metric and the `show ospf interface` cost both move, and a second reload changing the Router ID does restart and does drop the neighbor | pass |
| `TestDefaultMetric` | `internal/plugins/ospf/types/metric_test.go` | the derivation and both clamps, in Mbit/s | pass |
| `TestInterfaceStopClearsNeighbors` | `internal/plugins/ospf/iface/iface_test.go` | AC-8c: `(*Interface).Stop` empties the neighbor map, clears the DR and the BDR, and calls `neighborSink.InterfaceDown`, which is what a Router ID or area-type reload costs an interface, and what a re-pricing reload no longer costs one | pass. `TestOSPFStopLeavesAllDRouters` asserts the multicast leave alone, so this test carries the neighbor consequence |
| `TestDerivedCostReachesLDPSyncAndTEMetric` | `internal/plugins/ospf/interface_cost_test.go` | AC-9, one interface read four ways: eth0's Router-LSA metric, its `show ospf interface` cost, its LDP-sync restore value and its TE metric are all the derived 100. eth1 keeps the point-to-point TE arm beside it | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `reference-bandwidth` (Mbps) | 1..4294967 (YANG) | 4294967 | 0 is refused by YANG; a 0 reaching `interfaceCost` yields cost 1 (AC-6) | 4294968 refused by YANG |
| derived cost | 1..65535 | 65535 | a quotient of 0 clamps up to 1 (AC-2) | a quotient above 65535 clamps down to 65535 (AC-3) |
| link speed (Mbit/s) | 0 or positive | any positive value | 0 means unknown and yields cost 1 (AC-5). `parseLinkSpeedDuplex` maps a negative sysfs value, an unparseable one and an absent file all to 0, so a bridge reporting -1 arrives as unknown rather than as a negative divisor | N/A. The sysfs value is an `int`, and any positive value divides |
| reference bandwidth over link speed | the quotient before the clamp | 4294967 over 1 is 4294967, which clamps to 65535 | 1 over 4294967 truncates to 0, which clamps to 1 | no quotient overflows: both operands are widened to `uint64` before the division |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-config` | `test/ospf/ospf-config.ci` | `reference-bandwidth 100000` validates at commit (pre-existing, unchanged) | pass |
| `ospf-auto-cost-frr`, `show ospf interface` assertion | `test/interop/scenarios/ospf-auto-cost-frr/` | The operator reads the derived cost out of the real CLI on a real daemon. This is the operator-path proof a `.ci` would give, over a link that reports a speed -- which no `.ci` host provides, because the `.ci` suites configure no interface with a kernel-priced link | pass |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-auto-cost-frr` | `test/interop/scenarios/ospf-auto-cost-frr/` | FRR 10.3.1 | Ze derives 47 from `reference-bandwidth 470000` over a 10 Gbit/s veth, and FRR reads that 47 out of Ze's Router-LSA. Ze's own `show ospf interface` reports the same 47 | pass; RED observed with `interfaceCost` cut to the pre-auto-cost behavior: "assertion 2: wait for frr output timed out before the peer became ready", with assertion 1 (adjacency Full) still passing |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/ospf/config.go` - `parseOSPFConfig` inherits `ReferenceBandwidth` into `cfg.V6` and every `cfg.V6Extra` entry, beside the four inheritances already there (review BLOCKER)
- `internal/plugins/ospf/instance.go` - both cost copies deleted; `interfaceGlobalParamsChanged` reduced to its Router ID and area-type arms; `reconcile` gains a default arm that calls the new `repriceInterfaceLocked`, and originates the self-LSAs before it returns (owner decision, 2026-09-09)
- `internal/plugins/ospf/iface/iface.go` - `(*Interface).SetCost`, the in-place re-price. `Cost` is the one `Config` field no running behavior reads, which is what makes writing it safe and every other field unsafe
- `docs/architecture/ospf/ospf-5-interface-ism.md` - "Constraints on callers" records the in-place reference-bandwidth re-price and preserves both restart predicates.
- `internal/plugins/ospf/ldp_sync.go` - third cost copy deleted
- `internal/plugins/ospf/te_originate.go` - fourth cost copy deleted, the reference bandwidth threaded to the TE metric fallback
- `internal/plugins/ospf/types/metric.go` - `DefaultMetric` takes Mbit/s and clamps at both ends; the unused bits-per-second constant deleted
- `internal/plugins/ospf/types/metric_test.go` - rewritten for the Mbit/s contract and both clamps
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - the `reference-bandwidth` `ze:help`, which said the leaf changes no advertised metric
- `internal/plugins/iface/netlink/show_linux.go` - the doc comment named flow-export as the only caller, and claimed no virtual device reports a speed. Both were wrong after this change, and the second was wrong before it
- `docs/guide/ospf.md` - a new "Interface cost" section, plus a paragraph naming the three paths that report no speed for any interface: a VPP dataplane, a host that is not Linux, and no interface backend loaded
- `internal/plugins/ospf/iface/iface_test.go` - `TestInterfaceStopClearsNeighbors`, the AC-8b proof
- `docs/architecture/ospf/ospf-4-component-config.md` - the one-function decision, the no-cache decision, the router-wide numerator decision, and two traps. The trap "No synthetic device reports a link speed" was FALSE and is corrected: a veth reports 10000, which is what makes the interop scenario possible (review ISSUE 4)
- `internal/le/interoplab/bgp/checkers.go`, `internal/le/interoplab/bgp/check_extras.go` - the scenario's assertions: the adjacency wait and the two cost assertions. Both were committed by the RFC 7854 session that shared their hunks

## Files to Create
- `internal/plugins/ospf/interface_cost.go` - `interfaceCost`, `interfaceLinkSpeedMbps`, `costLinkSpeedUnknown`. `interfaceCostAtSpeed` existed for the one caller that priced a link twice from a single speed sample, `interfaceGlobalParamsChanged`; that caller is gone with the numerator arm, so the helper folded back into `interfaceCost` rather than staying as a hop with a comment about a caller that no longer exists
- `internal/plugins/ospf/interface_cost_test.go` - the derivation, the wiring and the reload behavior
- `test/interop/scenarios/ospf-auto-cost-frr/ze.conf`, `frr.conf` - the interop scenario, landed by `178f73002c`

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `reference-bandwidth` already existed with its type, range, units and default; only the `ze:help` changed |
| YANG validation constraints | No | `uint32 { range "1..4294967"; }` and `default 100000` already present |
| YANG custom validators | No | The native range is sufficient |
| CLI commands/flags | No | No command added or changed |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | A numeric leaf; the YANG range already drives it |
| Functional test for new RPC/API | N-A | No new RPC or API. The operator path is proven through the real CLI in the interop scenario |
| Pipe completeness | N-A | `show ospf interface` already routes through the pipes; only a field's value changed |
| Env var registration | N-A | Not under `environment/` |
| Doctor check for runtime dependencies | No | The sysfs speed file is read best-effort and its absence is a documented, tested outcome (cost 1) rather than a failure to report |
| Prometheus counters/metrics | No | No counter added |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | Covered in `docs/guide/ospf.md`, "Interface cost". `docs/features.md` lists OSPF as a whole and needs no row for one leaf becoming live |
| 2 | Config syntax changed? | No | No new leaf. `docs/guide/configuration.md` already shows `reference-bandwidth 100000` and it is still correct |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | No | -- |
| 5 | Plugin added/changed? | Yes | `docs/guide/ospf.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md`, new "Interface cost" section. The unknown-speed paragraph is owed a clause naming the VPP dataplane and a non-Linux host, both of which take the same fallback |
| 7 | Wire format changed? | No | The Router-LSA link metric field is unchanged; the value Ze puts in it changed |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 2328 Appendix C.3 defines no derivation from a link speed, so no requirement id is claimed. The RFC 3630 section 2.5.5 fallback still holds and its wording in `te_originate.go` was updated to say the fallback now carries the derived cost |
| 10 | Test infrastructure changed? | No | The scenario uses existing operation kinds |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not enumerate cost derivation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-4-component-config.md`, declared by `interface_cost.go`'s `// Design:` header, and `docs/architecture/ospf/ospf-5-interface-ism.md`, declared by `iface/iface.go` |
| 13 | Route metadata keys added/changed? | N-A | -- |
| 14 | Prometheus counters added/changed? | No | -- |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | -- |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/ospf.md` anchors the new file and the three changed producers; `internal/plugins/iface/netlink/show_linux.go` carries no `// Design:` header, and its comment correction is named here |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md:123` shows the leaf and was checked against the YANG; the guide's TE example already sets `cost 10` and is unaffected |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the leaf reaches the advertised metric
   - Tests: `TestOSPFTopologyCostFollowsReferenceBandwidth`
   - Files: `internal/plugins/ospf/interface_cost.go`, `instance.go`
   - Verify: the test failed because `lsdbTopology` substituted 1 and never read `cfg.ReferenceBandwidth`
2. **Phase: One cost rule** -- delete the four copies and give `DefaultMetric` a caller
   - Tests: `TestInterfaceCostAutoDerivation`, `TestDefaultMetric`
   - Files: `interface_cost.go`, `instance.go`, `ldp_sync.go`, `te_originate.go`, `types/metric.go`
   - Verify: every consumer reads the same number
3. **Phase: Reload** -- re-price on a reference-bandwidth change without dropping an adjacency
   - Tests: `TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth`, `TestReferenceBandwidthReloadKeepsNeighborAndReprices`
   - Files: `instance.go`, `iface/iface.go`
   - Verify: a re-priced interface keeps its neighbor and its runtime while the Router-LSA metric moves; a Router ID or area-type reload still restarts it
4. **Phase: Prove it against a peer** -- the interop scenario and its red
   - Tests: `ospf-auto-cost-frr`
   - Files: `test/interop/scenarios/ospf-auto-cost-frr/`, the two checker maps
   - Verify: FRR reads 47 with the derivation, and times out waiting for it without

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A cost of 0 is unreachable: `DefaultMetric` clamps up to `MetricMin`, and the two operands that cannot divide return an error the caller turns into 1 |
| Correctness | Both `DefaultMetric` operands are Mbit/s. The old signature took bits per second, so a stale call site would be wrong by a factor of a million and still compile |
| Naming | `costLinkSpeedUnknown` names the fallback rather than spelling 1 at four sites |
| Data flow | No consumer reads `ic.Cost` directly for an advertised metric; `interfaceCost` is the only source |
| Rule: `ai/rules/principles.md` | The unknown-speed zero is a GUARD, not a silently-wrong value: it is named, documented in the YANG help and the guide, and tested in both `interfaceCost` and `DefaultMetric` |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `types.DefaultMetric` has a non-test caller | Search `internal/plugins/ospf/` for `DefaultMetric` outside `_test.go`: `interface_cost.go` is the caller |
| No consumer keeps its own cost rule | Search `internal/plugins/ospf/` for `HasCost` outside `_test.go`: only `interface_cost.go` reads it |
| The leaf changes the advertised metric | `INTEROP_SCENARIO=ospf-auto-cost-frr ./le integration interop` |
| The YANG help no longer denies the feature | Search `internal/plugins/ospf/yang/ze-ospf-conf.yang` for "changes no advertised metric": no match |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | `reference-bandwidth` is bounded by YANG at 1..4294967; the link speed comes from the kernel and any non-positive value becomes 0 in `parseLinkSpeedDuplex` |
| Resource exhaustion | Two sysfs reads per interface per origination pass, bounded by the interface count, beside five netlink dumps the same pass already performs |
| Fail closed | An unpriceable link takes the lowest legal cost rather than 0. A 0 would be a malformed Router-LSA link metric, which is the failure this clamp exists to prevent |
| Untrusted input | None: the speed is the kernel's answer and the bandwidth is operator config. No wire value reaches the derivation |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- **A veth that is UP reports 10000 Mbit/s.** The veth driver declares 10 Gbit/s full duplex, and a Docker container's `eth0` is a veth. That single fact decided whether this feature could be proven against a peer at all. A loopback and a dummy report nothing and a bridge reports -1, which is what made "no synthetic device reports a speed" a believable and wrong generalization.
- Four copies of "cost, or 1 when unset" existed in four files. Each was three lines and each was correct. Adding two more cases to the rule is what made the duplication expensive: five copies would have been five chances to price a link differently from the LSA that advertises it.
- The Task section's architectural objection ("that value crosses a plugin boundary the OSPF plugin does not cross today") was written from the leaf and not from the package. Two files in the same package already made the import. A spec's stated obstacle is a claim like any other and gets read at the producer.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build auto-cost | Refuse the leaf at commit, the way `unimplementedVRFValidator` refuses `vrf` | Every OSPF implementation offers it, the derivation already existed in `types.DefaultMetric`, and the link speed was one already-made import away. A refusal is a placeholder that keeps the spec open |
| Read the link speed on each origination pass | Cache it and invalidate on a carrier event | A cache needs an invalidation path from the iface component into this plugin that nothing else here needs. Two sysfs reads ride a pass that already performs five netlink dumps per interface |
| An unknown speed costs 1 | Cost 65535 (unusable), or refuse to originate the link | 1 is what Ze advertised for every unconfigured interface before, so a tunnel or a bridge behaves exactly as it did. Making an unpriceable link unusable would change behavior nobody asked to change |
| `interfaceCost` takes the whole `interfaceConfig` | Pass the name, the cost and the flag separately | The function needs three fields of it, and a signature that takes three primitives invites a caller to pass them in the wrong order |
| Clamp at `MetricMax` rather than erroring | Keep the old `ErrOutOfRange` above 65535 | The old contract made a slow link under a large reference bandwidth an ERROR, and the caller then had to invent a cost. 65535 is the honest answer: the most expensive link the wire field can describe |
| Re-price the running interface in place | Restart the interface, which is what Ze did until 2026-09-09 | Owner decision, 2026-09-09. The restart never published the cost: `lsdbTopology` derives it from `e.cfg.ReferenceBandwidth` on every origination pass and `reconcile` replaces `e.cfg` first, so a carrier flap already re-prices a link with no restart at all. What the restart refreshed was the `Interface.cfg.Cost` copy `show ospf interface` reads, and `SetCost` refreshes that without stopping the state machine. The machinery is one setter and one engine helper, and it buys back every adjacency on the router |
| `SetCost` writes one field rather than taking a whole new `Config` | A general `UpdateConfig` on `Interface` | `Cost` is the ONLY `Config` field no running behavior reads, so a general setter would silently accept a `HelloInterval` or a `NetworkType` that the running timers and the ISM would ignore. Naming the one field makes the guarantee checkable: `ai/rules/simplicity.md` puts the burden of proof on the wider surface |
| `reconcile` requests origination before it returns | Leave publication entirely to the maintenance tick | The immediate attempt avoids an unnecessary wait. MinLSInterval can defer it, so every enrolled interface starts the existing maintenance worker, including passive and loopback interfaces. The tick retries from current topology and unchanged bodies do not flood |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- **RESOLVED 2026-09-08.** The scenario was held out of the implementation commit
  because its two registration files, `internal/le/interoplab/bgp/checkers.go`
  and `internal/le/interoplab/bgp/check_extras.go`, carried another session's
  uncommitted RFC 7854 `scenarioStatisticsPMACCT` work in the same hunks, and
  `interoplab.Discover` errors on a scenario directory with no checker. That
  session has since committed, and `178f73002c` landed both scenario
  directories. Both checker maps carry an `ospf-auto-cost-frr` entry at HEAD, so
  nothing about this spec is outstanding in the working tree.
- **Auto-cost prices nothing on a VPP dataplane or on a non-Linux host.**
  `vppBackendImpl.LinkSpeedDuplex` answers 0 because a VPP interface has no
  `/sys/class/net` entry, `stubBackend.LinkSpeedDuplex` answers 0 on every
  platform that is not Linux, and `iface.LinkSpeedDuplex` answers 0 when no
  backend is registered. Every interface with no `cost` therefore takes
  `costLinkSpeedUnknown` on those dataplanes, which is what Ze advertised before
  auto-cost existed. The operator's route is the `cost` leaf. Giving VPP a speed
  answer is a separate change to the VPP backend, and it is not this spec's
  scope. `docs/guide/ospf.md` now names all three paths and anchors each one on
  its producer.
- The derivation is per interface and has no per-area or per-address-family
  override. `reference-bandwidth` is one instance-wide number, which is what the
  leaf declares, and `parseOSPFConfig` inherits it into every RFC 5838 address
  family so the two families never price one link differently.
- **No test observes a re-pricing reload against a live peer.**
  `TestReferenceBandwidthReloadKeepsNeighborAndReprices` drives the reload, holds
  the neighbor at 2-Way across it, and reads the re-priced metric off the
  OSPFv2 Router-LSA the LSDB installed. The neighbor it drives reaches 2-Way and
  no further, because a Full adjacency needs a database exchange with a peer, and
  the `ospf-auto-cost-frr` scenario performs no reload: it starts Ze at
  `reference-bandwidth 470000` and asserts the 47 FRR reads. A reload step in
  that scenario, showing FRR's adjacency stay Full while the metric moves, is the
  proof this leaves open, and AC-8b claims only what the test checks.
- **The OSPFv3 re-price is proven at the origination topology and the interface
  snapshot, not at a v3 Router-LSA body.** An OSPFv3 Router-LSA describes
  adjacencies rather than stub networks, so an interface with no Full neighbor
  contributes no link to read a metric off, and a Full neighbor needs a peer.
  `v6RouterLSABody` takes the metric from the same `InterfaceInfo.Cost` field
  `routerLinks` reads, and
  `TestReferenceBandwidthReloadRepricesEveryAddressFamily` asserts that field and
  the snapshot at 23 after the reload.
- **A change to an interface's own `cost` leaf still restarts that interface.**
  `interfaceParamsEqual` compares `Cost` and `HasCost`, so the per-interface leaf
  keeps the behavior the router-wide leaf just lost. It is the same question
  Thomas answered on 2026-09-09 asked of the other leaf, and this session did not
  extend his answer to it. Extending it is not a one-line change:
  `lsdbTopology` prices from `e.running`, and `reconcile`'s `default` arm never
  writes `e.running[name] = want`, so a cost-only re-price must also update that
  map and split the restart branch into a cost-only arm and the rest (round 3
  ISSUE 1).
- A carrier flap re-prices on the next origination pass rather than immediately.
  Nothing schedules an origination for a speed change alone.

## RFC Documentation (Scope: protocol)

`internal/plugins/ospf/types/metric.go` carries the one requirement this spec
enforces, above `DefaultMetric`'s lower clamp. Read in `rfc/full/rfc2328.txt`,
Appendix C.3 "Router interface parameters", the "Interface output cost" entry:

```
        Interface output cost
            The cost of sending a packet on the interface, expressed in
            the link state metric.  This is advertised as the link cost
            for this interface in the router's router-LSA. The interface
            output cost must always be greater than 0.
```

`DefaultMetric` clamps up to `MetricMin`, and `interfaceCost` answers
`costLinkSpeedUnknown` for the two operands the division cannot take, so no path
reaches 0. The upper clamp is a wire-format bound rather than an RFC rule: the
Router-LSA link metric field is two octets wide.

RFC 2328 defines NO derivation of a cost from a link speed, so auto-cost is a
convention and no requirement id in `rfc/short/rfc2328.md` is claimed, changed or
newly proven by this spec.

RFC 3630 section 2.5.5 makes the TE metric default to the OSPF interface cost.
That fallback is unchanged and now carries the derived cost, which
`applyTELinkAttributes` states in its doc comment.

## Implementation Summary

### What Was Implemented
- The derivation and its consumers landed in `1753e1f4c9`: `internal/plugins/ospf/interface_cost.go` (`interfaceCost`, `interfaceLinkSpeedMbps`, `costLinkSpeedUnknown`), the four deleted cost copies in `instance.go`, `ldp_sync.go` and `te_originate.go`, `types.DefaultMetric` taking both operands in Mbit/s and clamping at both ends, and `interfaceGlobalParamsChanged` taking the whole `interfaceConfig`.
- The interop scenario `test/interop/scenarios/ospf-auto-cost-frr/` and its two checker-map entries landed in `178f73002c`.
- This phase wrote no product behavior. It added the two tests the design phase recorded as owed, and the guide clause it recorded as missing.
- The review-fix phase (2026-09-09) landed two product changes, both defects this spec introduced: the address-family inheritance in `parseOSPFConfig` and the cost comparison in `interfaceGlobalParamsChanged`.
- The re-price phase (2026-09-09) implemented the owner's answer to round 2's ISSUE 1: `(*ospfiface.Interface).SetCost`, `(*engine).repriceInterfaceLocked`, a `default` arm in `reconcile` that calls it, an `originateSelfLSAs` call at the end of `reconcile`, and `interfaceGlobalParamsChanged` reduced to its Router ID and area-type arms. `interfaceCostAtSpeed` folded back into `interfaceCost` with its last caller. A `reference-bandwidth` commit now tears down no adjacency.
- The resumed round 4 repair (2026-09-10) starts maintenance for passive-only and loopback-only enrollment at startup and reload, replaces per-callback goroutines with one coalesced notification, and orders worker registration against shutdown cancellation under `spawnMu`. `shutdown` joins admitted work and late callbacks are rejected.

### Bugs Found/Fixed
- **Every OSPFv3 address family was pinned to reference bandwidth 100000** (review BLOCKER). `applyAddressFamilies` seeds each RFC 5838 child from `defaultOSPFConfig`, and the `ospf-af-topology` grouping declares no `reference-bandwidth` leaf, so no operator value ever reached the child. One physical link configured in both families advertised 47 in the OSPFv2 Router-LSA and 10 in the OSPFv3 one under `reference-bandwidth 470000`, with no configuration that made them agree. Fixed by inheritance in `parseOSPFConfig`, beside the Router ID, Router Information, Graceful Restart and Fast Reroute inheritances, and unconditional because no sub-config can state a value of its own.
- **A reference-bandwidth change bounced every auto-costed adjacency even when no cost changed** (review ISSUE 2). `interfaceGlobalParamsChanged` compared the numerator, so 100000 to 105000 restarted every 10 Gbit/s interface at the same cost 10, and on a VPP dataplane or a non-Linux host it restarted every interface on the router while pricing none of them. It now compares `interfaceCost` on each side, which also subsumes the `ic.HasCost` arm.

### Documentation Updates
- `docs/guide/ospf.md`, "Interface cost": a paragraph naming the three paths that report no speed for ANY interface, rather than for one device. A VPP interface has no `/sys/class/net` entry, a host that is not Linux runs the stub backend, and Ze answers 0 when no interface backend is loaded. Verified at `vppBackendImpl.LinkSpeedDuplex` (`internal/plugins/iface/vpp/query.go`), `stubBackend.LinkSpeedDuplex` (`internal/plugins/iface/netlink/backend_other.go`, `//go:build !linux`) and `LinkSpeedDuplex` (`internal/component/iface/dispatch.go`), each of which returns `0, ""`.
- Three source anchors carry those producers. `./le docs-to-code index-check` accepts all three; its four failures are in `docs/architecture/api/`, `docs/architecture/exabgp-bridge.md` and `docs/architecture/firewall/`, none of which this spec touches.
- The review-fix phase (2026-09-09) carried four page edits with its two product changes.
  `docs/architecture/ospf/ospf-4-component-config.md` gains the router-wide numerator
  decision, states that the restart decision compares the derived cost, and REPLACES the
  false trap "No synthetic device reports a link speed" with what the kernel actually
  reports (a veth 10000, a loopback and a dummy nothing, a bridge -1 which
  `parseLinkSpeedDuplex` maps to 0). `docs/architecture/ospf/ospf-5-interface-ism.md`
  adds the re-pricing reload to the set of reloads that recreate an interface runtime,
  and says which one does not. `docs/guide/ospf.md` states that the leaf is router-wide
  across the address families and that a change restarts only the interfaces it
  re-prices. The `reference-bandwidth` `ze:help` carries both sentences for the operator
  who reads the schema instead of the guide.

### TDD Evidence

The product code landed before these two tests, so each red was forced by reverting the
behavior the test asserts, and each was observed rather than predicted.

RED, `(*Interface).Stop` with its clearing block and its `neighborSink.InterfaceDown` call
removed:

```
=== RUN   TestInterfaceStopClearsNeighbors
    iface_test.go:255: neighbors after Stop = 1, want 0: a restarted interface keeps no adjacency
    iface_test.go:258: DR after Stop = 10.0.0.2, want cleared
    iface_test.go:261: BDR after Stop = 10.0.0.1, want cleared
    iface_test.go:264: InterfaceDown calls = [], want one for "eth0"
--- FAIL: TestInterfaceStopClearsNeighbors (0.00s)
FAIL	github.com/ze-software/ze/internal/plugins/ospf/iface	0.468s
```

RED, `interfaceCost` cut to the pre-auto-cost behavior (the `cost` leaf, or 1):

```
=== RUN   TestDerivedCostReachesLDPSyncAndTEMetric
    interface_cost_test.go:189: ldp-sync restore value = 1, want the derived 100: LDP-sync must restore the auto-cost, not 1
    interface_cost_test.go:208: TE metric = 1, want the derived 10: the RFC 3630 fallback must carry the cost the Router-LSA advertises
--- FAIL: TestDerivedCostReachesLDPSyncAndTEMetric (0.00s)
FAIL	github.com/ze-software/ze/internal/plugins/ospf	0.517s
```

GREEN, both producers restored:

```
ok  	github.com/ze-software/ze/internal/plugins/ospf/iface	0.374s
ok  	github.com/ze-software/ze/internal/plugins/ospf	0.448s
```

GREEN, the whole OSPF package tree (17 packages, `go test ./internal/plugins/ospf/...`):

```
ok  	github.com/ze-software/ze/internal/plugins/ospf	0.479s
ok  	github.com/ze-software/ze/internal/plugins/ospf/iface	0.997s
ok  	github.com/ze-software/ze/internal/plugins/ospf/spf	2.707s
ok  	github.com/ze-software/ze/internal/plugins/ospf/types	2.953s
```

#### Review-fix phase, 2026-09-09

RED, the address-family test against the code as the review found it. The OSPFv2 family
reads the operator's 470000 and the OSPFv3 family reads the seeded default:

```
--- FAIL: TestReferenceBandwidthReachesEveryAddressFamily (0.00s)
    interface_cost_test.go:181: OSPFv3 advertised cost = 10, want 47: both families price one link alike
    interface_cost_test.go:188: address family ipv6-unicast reference bandwidth = 100000, want the router-wide 470000
    interface_cost_test.go:188: address family ipv4-multicast reference bandwidth = 100000, want the router-wide 470000
```

RED, the two new arms of the restart decision against the numerator comparison:

```
--- FAIL: TestInterfaceGlobalParamsChangedReferenceBandwidth (0.00s)
    interface_cost_test.go:152: a reference-bandwidth change that leaves the cost at 10 restarted the interface
    interface_cost_test.go:158: a reference-bandwidth change restarted an interface whose link speed the kernel does not report
```

RED, AC-8b forced twice, because the product code preceded the test. With
`interfaceGlobalParamsChanged` returning false, nothing restarts:

```
--- FAIL: TestReferenceBandwidthReloadDropsAdjacencyAndReprices (0.00s)
    interface_cost_test.go:261: reconcile journal = {opened:[] closed:[] changed:map[]}, want eth0 restarted: 10000 over a 1G link prices it 10, not 100
```

With `startInterfaceLocked` no longer stopping the runtime it replaces, the restart
happens and the adjacency survives it:

```
--- FAIL: TestReferenceBandwidthReloadDropsAdjacencyAndReprices (0.00s)
    interface_cost_test.go:264: the replaced runtime kept 1 neighbors, want 0: the re-price drops the adjacency
```

RED, AC-9's four ways, with `interfaceCost` cut to the pre-auto-cost behavior. All four
readings of eth0 fall to 1:

```
--- FAIL: TestDerivedCostReachesLDPSyncAndTEMetric (0.00s)
    interface_cost_test.go:329: ldp-sync restore value = 1, want the derived 100: LDP-sync must restore the auto-cost, not 1
    interface_cost_test.go:348: TE metric = 1, want the derived 10: the RFC 3630 fallback must carry the cost the Router-LSA advertises
    interface_cost_test.go:355: eth0 Router-LSA cost = 1, want the derived 100
    interface_cost_test.go:358: eth0 `show ospf interface` cost = 1, want the derived 100
    interface_cost_test.go:386: eth0 TE metric = 1, want the derived 100: one interface reads the same cost four ways
```

GREEN, every producer restored and the whole OSPF package tree
(`go test ./internal/plugins/ospf/... -count=1`):

```
ok  	github.com/ze-software/ze/internal/plugins/ospf	0.684s
ok  	github.com/ze-software/ze/internal/plugins/ospf/cli	3.462s
ok  	github.com/ze-software/ze/internal/plugins/ospf/iface	3.174s
ok  	github.com/ze-software/ze/internal/plugins/ospf/lsdb	4.812s
ok  	github.com/ze-software/ze/internal/plugins/ospf/types	1.146s
ok  	github.com/ze-software/ze/internal/plugins/ospf/v3/transport	2.137s
ok  	github.com/ze-software/ze/internal/plugins/ospf/yang	3.768s
```

#### Round 2 fix phase, 2026-09-09

RED, `TestReferenceBandwidthReloadDropsAdjacencyAndReprices` after the Hello gained
`Neighbors: []types.RouterID{cfg.RouterID}`, with `receiveHello` never setting `TwoWay`.
The new assertion is what proves the neighbor really reaches 2-Way:

```
--- FAIL: TestReferenceBandwidthReloadDropsAdjacencyAndReprices (0.00s)
    interface_cost_test.go: neighbor state before the reload = "init", want 2-way: the Hello names this router
```

RED, the same test with `interfaceGlobalParamsChanged` forced false, so nothing restarts:

```
--- FAIL: TestReferenceBandwidthReloadDropsAdjacencyAndReprices (0.00s)
    interface_cost_test.go: reconcile journal = {opened:[] closed:[] changed:map[]}, want eth0 restarted: 10000 over a 1G link prices it 10, not 100
```

RED, the same test with `startInterfaceLocked` no longer stopping the runtime it
replaces. The neighbor survives the restart, and the table still reads 2-way:

```
--- FAIL: TestReferenceBandwidthReloadDropsAdjacencyAndReprices (0.00s)
    interface_cost_test.go: the replaced runtime kept 1 neighbors, want 0: the re-price drops the adjacency
    interface_cost_test.go: eth0 neighbor after the re-pricing restart = "2-way" (present true), want down
```

RED, the single link-speed sample, with `interfaceGlobalParamsChanged` reverted to a
read for each side:

```
--- FAIL: TestInterfaceGlobalParamsChangedReferenceBandwidth (0.00s)
    interface_cost_test.go: a link that renegotiated between two speed reads restarted an interface at an unchanged reference bandwidth
    interface_cost_test.go: interfaceGlobalParamsChanged read the link speed 2 times, want 1
```

RED, the OSPFv3 reload, with the address-family numerator pinned at
`DefaultReferenceBandwidth`:

```
--- FAIL: TestReferenceBandwidthReloadRepricesEveryAddressFamily (0.00s)
    interface_cost_test.go: OSPFv3 cost before the reload = 10, want 47 from reference-bandwidth 470000 over a 10G link
```

RED, the same test with the restart disabled, which is the reload arm itself:

```
--- FAIL: TestReferenceBandwidthReloadRepricesEveryAddressFamily (0.00s)
    interface_cost_test.go: OSPFv3 reconcile journal = {opened:[] closed:[] changed:map[]}, want eth0 restarted: 235000 over a 10G link prices it 23, not 47
```

RED, `TestDerivedCostReachesLDPSyncAndTEMetric` after its TE arms began selecting the
Link LSA by local interface address, with `interfaceCost` cut to the pre-auto-cost
behavior. All five readings still discriminate, so the new selector lost nothing:

```
--- FAIL: TestDerivedCostReachesLDPSyncAndTEMetric (0.00s)
    interface_cost_test.go: ldp-sync restore value = 1, want the derived 100: LDP-sync must restore the auto-cost, not 1
    interface_cost_test.go: TE metric = 1, want the derived 10: the RFC 3630 fallback must carry the cost the Router-LSA advertises
    interface_cost_test.go: eth0 Router-LSA cost = 1, want the derived 100
    interface_cost_test.go: eth0 `show ospf interface` cost = 1, want the derived 100
    interface_cost_test.go: eth0 TE metric = 1, want the derived 100: one interface reads the same cost four ways
```

GREEN, every producer restored and the whole OSPF package tree
(`go test ./internal/plugins/ospf/... -count=1`, 17 packages, 16 with tests):

```
ok  	github.com/ze-software/ze/internal/plugins/ospf	2.287s
ok  	github.com/ze-software/ze/internal/plugins/ospf/iface	3.880s
ok  	github.com/ze-software/ze/internal/plugins/ospf/lsdb	3.538s
ok  	github.com/ze-software/ze/internal/plugins/ospf/neighbor	5.077s
ok  	github.com/ze-software/ze/internal/plugins/ospf/v3/transport	4.489s
```

#### Re-price phase, 2026-09-09 (owner decision: a `reference-bandwidth` commit costs no adjacency)

The product change is three edits: `(*Interface).SetCost`
(`internal/plugins/ospf/iface/iface.go`), `(*engine).repriceInterfaceLocked` plus a
`default` arm in `reconcile` and an `originateSelfLSAs` call before it returns, and
`interfaceGlobalParamsChanged` cut back to its Router ID and area-type arms (both
`internal/plugins/ospf/instance.go`). `interfaceCostAtSpeed` folded back into
`interfaceCost`: its only reason to exist was the two-price comparison in
`interfaceGlobalParamsChanged`, which is gone.

RED, before the product change, all four proofs (`./le job run label ospf-cost-red command
go test ./internal/plugins/ospf/ -run '...' -count=1`):

```
--- FAIL: TestOSPFTopologyCostFollowsReferenceBandwidth (0.00s)
    interface_cost_test.go:117: reconcile journal = {opened:[] closed:[] changed:map[eth0:true]}, want no interface restarted: a re-pricing publishes through the next origination pass
--- FAIL: TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth (0.00s)
    interface_cost_test.go:139: a reference-bandwidth change restarted an auto-cost interface: the Router-LSA carries the new cost without one
--- FAIL: TestReferenceBandwidthReloadRepricesEveryAddressFamily (0.00s)
    interface_cost_test.go:251: OSPFv3 reconcile journal = {opened:[] closed:[] changed:map[eth0:true]}, want eth0 untouched: a re-pricing must not restart the interface
    interface_cost_test.go:254: the OSPFv3 reconcile replaced the interface runtime it re-priced, so it dropped the adjacency
--- FAIL: TestReferenceBandwidthReloadKeepsNeighborAndReprices (0.00s)
    interface_cost_test.go:324: reconcile journal = {opened:[] closed:[] changed:map[eth0:true]}, want eth0 untouched: a re-pricing must not restart the interface
    interface_cost_test.go:327: reconcile replaced the interface runtime it re-priced, so it dropped the adjacency
```

RED for proof 3 (`show ospf interface` reports the new cost), forced by removing the
`e.repriceInterfaceLocked(want)` call from `reconcile`'s default arm:

```
--- FAIL: TestOSPFTopologyCostFollowsReferenceBandwidth (0.00s)
    interface_cost_test.go:114: eth0 runtime cost = 100 after the reload, want 10: the snapshot must not keep the old cost
--- FAIL: TestReferenceBandwidthReloadRepricesEveryAddressFamily (0.00s)
    interface_cost_test.go:262: OSPFv3 `show ospf interface` cost after the reload = 47, want the re-priced 23
--- FAIL: TestReferenceBandwidthReloadKeepsNeighborAndReprices (0.00s)
    interface_cost_test.go:339: eth0 `show ospf interface` cost = 100 after the reload, want the re-priced 10
```

RED for proof 4 (a change that DOES need a restart still gets one), forced by pinning
`interfaceGlobalParamsChanged` to false:

```
--- FAIL: TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth (0.00s)
    interface_cost_test.go:150: a Router ID change did not restart the interface: its Hellos would carry the old identity
    interface_cost_test.go:155: an area-type change did not restart the interface: its Hellos would carry the old E-bit
--- FAIL: TestReferenceBandwidthReloadKeepsNeighborAndReprices (0.00s)
    interface_cost_test.go:350: reconcile journal = {opened:[] closed:[] changed:map[]}, want eth0 restarted by a Router ID change
```

RED for proof 2 (the Router-LSA the next origination pass produces carries the new cost, in
both families), forced by withholding the reloaded config from `e.cfg` in `reconcile`. Each
family's BEFORE reading still passes, so the red is about the reload rather than about the
derivation:

```
--- FAIL: TestOSPFTopologyCostFollowsReferenceBandwidth (0.00s)
    interface_cost_test.go:111: eth0 advertised cost = 100 after lowering the reference bandwidth to 10000, want 10
--- FAIL: TestReferenceBandwidthReloadRepricesEveryAddressFamily (0.00s)
    interface_cost_test.go:259: OSPFv3 cost after the reload = 47, want the re-priced 23
    interface_cost_test.go:262: OSPFv3 `show ospf interface` cost after the reload = 47, want the re-priced 23
--- FAIL: TestReferenceBandwidthReloadKeepsNeighborAndReprices (0.00s)
    interface_cost_test.go:336: eth0 advertised cost = 100 after the reload, want the re-priced 10
    interface_cost_test.go:339: eth0 `show ospf interface` cost = 100 after the reload, want the re-priced 10
```

RED for the `reconcile` origination itself, forced by deleting the `e.originateSelfLSAs()`
call. The topology and the snapshot both read 10, and the Router-LSA an OSPFv2 peer would
read does not:

```
--- FAIL: TestReferenceBandwidthReloadKeepsNeighborAndReprices (0.00s)
    interface_cost_test.go:358: Router-LSA link metric = 100 after the reload, want the re-priced 10
```

GREEN, the whole OSPF tree (`./le job run label ospf-green-all command go test
./internal/plugins/ospf/... -count=1`):

```
ok  	github.com/ze-software/ze/internal/plugins/ospf	3.136s
ok  	github.com/ze-software/ze/internal/plugins/ospf/cli	4.178s
ok  	github.com/ze-software/ze/internal/plugins/ospf/iface	4.865s
ok  	github.com/ze-software/ze/internal/plugins/ospf/lsdb	3.656s
ok  	github.com/ze-software/ze/internal/plugins/ospf/neighbor	0.617s
ok  	github.com/ze-software/ze/internal/plugins/ospf/packet	2.710s
ok  	github.com/ze-software/ze/internal/plugins/ospf/redistribute	0.895s
ok  	github.com/ze-software/ze/internal/plugins/ospf/redistribute/events	1.444s
ok  	github.com/ze-software/ze/internal/plugins/ospf/spf	2.421s
ok  	github.com/ze-software/ze/internal/plugins/ospf/sr	0.292s
ok  	github.com/ze-software/ze/internal/plugins/ospf/transport	2.054s
ok  	github.com/ze-software/ze/internal/plugins/ospf/types	1.148s
ok  	github.com/ze-software/ze/internal/plugins/ospf/v3/packet	1.703s
ok  	github.com/ze-software/ze/internal/plugins/ospf/v3/transport	4.521s
ok  	github.com/ze-software/ze/internal/plugins/ospf/v3/types	3.905s
?   	github.com/ze-software/ze/internal/plugins/ospf/wire	[no test files]
ok  	github.com/ze-software/ze/internal/plugins/ospf/yang	3.297s
```

`./le verify lint run` reports no finding in `internal/plugins/ospf` or
`internal/plugins/ospf/iface`. Its 114 findings across the tree name no file this spec
touches, and they are pre-existing.

Two facts the reload test had to state to observe the Router-LSA at all, and both are the
test's environment rather than the product's:

- `eth0` does not exist on the test host, so `interfaceIPv4Address` reads no address for it
  and `routerLinks` emits no link. `addressedTopology` wires the engine's OWN origination
  topology into the LSDB with an address substituted, so the cost the assertion reads is
  still the one `lsdbTopology` derived.
- RFC 2328 Appendix B sets MinLSInterval to 5 seconds, which defers a second origination of
  one LSA whatever produced it, so the default would hide the reconcile pass behind the rate
  limit. The test's config sets `min-ls-interval-ms 1` and asserts the parse took it.

- The re-price phase (2026-09-09) carried four page edits with its product change.
  `docs/guide/ospf.md` drops the sentence saying a `reference-bandwidth` change restarts an
  interface and states the new promise, that the commit costs no adjacency, with the two
  reloads that do restart one. `docs/architecture/ospf/ospf-4-component-config.md` replaces
  the restart trap with the owner's decision and its evidence, and adds the reconcile
  origination. `docs/architecture/ospf/ospf-5-interface-ism.md` removes
  `reference-bandwidth` from the set of reloads that recreate a runtime and states why
  `Cost` is the one field that can be written into a running interface. The
  `reference-bandwidth` `ze:help` carries the same promise for the operator who reads the
  schema instead of the guide. Two new source anchors point at
  `(*Interface).SetCost` and `repriceInterfaceLocked`.

### Deviations from Plan
- The Review Gate section was written by the review into the session scratch and appended
  here by the fix phase. The `pretool-writeedit` gate refuses a line-number citation in
  prose, so each `file.go:NNN` in it was replaced by the symbol at that line. Nothing else
  in the section was edited.
- A-2 was broken: a Docker veth reports 10000 Mbit/s, so the implementation added a live FRR cost proof instead of treating container interop as impossible.
- Round 5 used the available independent `openai-codex/gpt-6-astra` reviewer under Thomas's explicit model override because Opus 5 was unavailable. The closure context ran no new test, gate, formatter, linter or build; it reused the observed resumed-repair evidence under the pre-release rule.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `reference-bandwidth` divided by the link speed is the cost Ze advertises | interop | `ospf-auto-cost-frr`: FRR 10.3.1 reads 47 out of Ze's Router-LSA under `reference-bandwidth 470000` over a 10 Gbit/s veth. The implementation phase recorded its RED with `interfaceCost` cut to the pre-auto-cost behavior: "assertion 2: wait for frr output timed out before the peer became ready". This phase did not re-run the scenario |
| A fast link costs less than a slow one with no per-interface configuration | unit | `TestInterfaceCostAutoDerivation`, eleven cases over a stubbed speed table: 1 Gbit/s costs 100, 10 Gbit/s costs 10, 100 Gbit/s costs 1 |
| Raising or lowering the leaf re-prices the network | unit | `TestOSPFTopologyCostFollowsReferenceBandwidth`: the real config parser to the origination topology, 100 before the reload and 10 after it |
| An explicit `cost` is never overridden | unit | `TestInterfaceCostAutoDerivation` "explicit cost wins" |
| Every consumer of the cost carries the same number (AC-9) | unit | `TestDerivedCostReachesLDPSyncAndTEMetric`: eth0 is read FOUR ways and each reading is the derived 100 -- its Router-LSA metric, its `show ospf interface` cost, its `show ospf ldp-sync` restore value and its RFC 3630 TE metric. eth1 keeps the point-to-point TE arm at 10 beside it. RED observed with `interfaceCost` cut to the pre-auto-cost behavior: all five assertions read 1 |
| A re-pricing reload keeps the adjacency and still publishes the new cost (AC-8b, owner decision 2026-09-09) | unit | `TestReferenceBandwidthReloadKeepsNeighborAndReprices` drives the production path: `parseOSPFConfig`, `reconcile`, `interfaceGlobalParamsChanged`, `repriceInterfaceLocked`, `originateSelfLSAs`. The neighbor is 2-Way on both sides of the reload, the runtime is the same object, and the OSPFv2 Router-LSA link metric moves from 100 to 10. Four forced REDs recorded in the TDD Evidence: the restart still firing, the re-price call removed, the reconcile origination removed, and the reloaded numerator withheld from `e.cfg`. UNPROVEN: no test observes a live peer holding its adjacency across the reload -- see Known Limitations |
| A change that DOES need a restart still gets one (AC-8c) | unit | `TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth` asserts the Router ID and area-type arms return true, and `TestReferenceBandwidthReloadKeepsNeighborAndReprices` drives a Router ID reload through `reconcile`: the runtime is replaced and the neighbor drops to Down. RED observed with `interfaceGlobalParamsChanged` pinned to false: all three assertions fail |
| One reference bandwidth prices a link the same way in both families ACROSS A RELOAD (AC-8b, AC-11) | unit | `TestReferenceBandwidthReloadRepricesEveryAddressFamily`: an OSPFv3 engine started at `reference-bandwidth 470000` reads cost 47, and reconciling it with the `v6Families` entry of a 235000 config keeps eth0's runtime while its origination topology and its interface snapshot both read 23. No default produces 23, so a family that failed to inherit the reload's numerator cannot pass by accident. RED observed twice in the re-price phase: with the re-price call removed (the snapshot stayed 47) and with the reloaded numerator withheld from `e.cfg` (both readings stayed 47) |
| One reference bandwidth prices a link the same way in both address families (AC-11) | unit | `TestReferenceBandwidthReachesEveryAddressFamily`: `reference-bandwidth 470000` over a 10 Gbit/s link is cost 47 in the OSPFv2 origination topology and 47 in the OSPFv3 engine's, and every `v6Families` entry carries 470000. RED observed against the code the review found: the OSPFv3 family read 10 |
| The operator reads the cost Ze decided | interop | `ospf-auto-cost-frr`, the `show ospf interface` assertion: Ze's own CLI reports the same 47. Recorded by the implementation phase; not re-run here |
| Passive-only and loopback-only reloads eventually publish after MinLSInterval | regression | `TestReferenceBandwidthReloadPublishesAfterMinLSInterval` reads installed Router-LSAs: metric 100 before the interval and 10 after it, for both initial and reload-added enrollment. All four cases fail against the saved inherited source and pass in `job-ospf-fixed-tree-3c4f78c3.log` |
| Deferred origination cannot outlive engine shutdown | regression | `TestDeferredOriginationJoinsTheEngine` blocks a topology read, observes shutdown waiting, releases it and observes the completed metric 10. A later callback performs no topology read. Independent join-removal and late-admission overlays fail; the fixed race suite passes |
| Existing live-peer auto-cost behavior survives the lifecycle repair | interop regression | Main ran `CGO_ENABLED=0 INTEROP_SCENARIO=ospf-auto-cost-frr ./le --name continue-ospf integration interop`: exit 0, `integration: 1 action(s) passed.` The renderer emitted no detailed JSON or image identifiers. This scenario starts at the configured cost and performs no reload |

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Review Gate

Independent review, 2026-09-08, Opus 5. Reviewed the WHOLE feature (`1753e1f4c9`,
`178f73002c`, `313f71195`) against the spec, not only this session's diff. Every
claim below was read at its producer. The review wrote this section into the session
scratch; the fix phase appended it here, and the only edits are the line-number
citations the `pretool-writeedit` gate refuses, each replaced by its symbol.

**Counts: 1 BLOCKER, 3 ISSUE, 3 NOTE.**

### BLOCKER

1. **Every OSPFv3 address family is pinned to a reference bandwidth of 100000 and
   the operator cannot change it.** `applyAddressFamilies`
   (`internal/plugins/ospf/config.go`) builds each RFC 5838 child with
   `child := defaultOSPFConfig()`, which seeds `ReferenceBandwidth` to
   `DefaultReferenceBandwidth` (100000), then calls `applyTree(&child, sub)` over the
   address-family subtree. The `ospf-af-topology` grouping
   (`internal/plugins/ospf/yang/ze-ospf-conf.yang`) contains `areas` and `interfaces`
   only, so `applyTree` never finds a `reference-bandwidth` key for the child and the
   seeded 100000 survives. `parseOSPFConfig` then inherits `RouterID`,
   `RouterInformation`, `GracefulRestart` and `FastReroute` into `cfg.V6` and
   `cfg.V6Extra`, and inherits `ReferenceBandwidth` into neither. `v6Families` hands
   that child to `register_multiaf.go`, whose `e.setConfig(families[i].cfg)` gives the
   v6 engine its own `e.cfg.ReferenceBandwidth`, which `lsdbTopology`
   (`internal/plugins/ospf/instance.go`) reads.

   Failure scenario. Config: `reference-bandwidth 470000`, `interface eth0 { area 0.0.0.0 }`
   under `ospf`, and the same `eth0` under `address-family { ipv6 { ... } }`. eth0 is a
   10 Gbit/s link (10000 Mbit/s). The OSPFv2 Router-LSA advertises cost 47
   (470000/10000). The OSPFv3 Router-LSA for the SAME physical link advertises cost 10
   (100000/10000, the seeded default). The operator's leaf silently applies to one
   family, the two families disagree about the same link by a factor of 4.7, and there
   is no configuration that makes them agree: neither raising the leaf (v6 ignores it)
   nor an `address-family` leaf (none exists in the YANG).

   This spec introduces the divergence. Before auto-cost both families advertised 1 for
   every interface with no `cost`, so there was nothing to disagree about. Fix is one of:
   inherit `ReferenceBandwidth` into `cfg.V6` and each `cfg.V6Extra[i].cfg` in
   `parseOSPFConfig` beside the four inheritances already there (matching the
   "router-wide policy" reasoning those comments give), or add the leaf to
   `ospf-af-topology` with an inheritance fallback. The first is the simpler and matches
   what the leaf declares: one instance-wide number.

### ISSUE

2. **A `reference-bandwidth` change bounces every auto-costed adjacency even when no
   derived cost changes.** `interfaceGlobalParamsChanged`
   (`internal/plugins/ospf/instance.go`) returns
   `!ic.HasCost && oldCfg.ReferenceBandwidth != newCfg.ReferenceBandwidth`. It compares
   the NUMERATOR, never the quotient. `reconcile` then takes the restart branch and
   `startInterfaceLocked` calls `old.Stop()`, which drops the adjacency.

   Failure scenario A: `reference-bandwidth 100000` raised to `105000` on a router whose
   interfaces are 10 Gbit/s. Old cost 10, new cost 10 (truncated). Every auto-costed
   interface restarts, every adjacency re-forms, traffic re-routes for the dead interval,
   and the advertised metric is byte-identical before and after.

   Failure scenario B, worse and documented as supported: on a VPP dataplane or a
   non-Linux host every interface takes `costLinkSpeedUnknown` (finding 5 below verifies
   the three producers). Auto-cost prices nothing there, so ANY `reference-bandwidth`
   change bounces EVERY OSPF adjacency on the router and changes no cost at all. The
   guide the session wrote states auto-cost prices nothing there; it does not warn that
   the leaf still bounces the router.

   The fix is available in the function's own package and is smaller than the current
   condition's blast radius: compare `interfaceCost(ic, oldCfg.ReferenceBandwidth) !=
   interfaceCost(ic, newCfg.ReferenceBandwidth)`. That also subsumes the `ic.HasCost`
   arm, since `interfaceCost` returns `ic.Cost` on both sides when the leaf is set.
   R-5 in Risks anticipates the domain-wide burst but assumes the re-price is real;
   neither R-4 nor R-5 covers a bounce with no cost change.

3. **`TestInterfaceStopClearsNeighbors` does not prove AC-8b; it proves the second half
   of a chain whose two halves never meet in one test.** The test
   (`internal/plugins/ospf/iface/iface_test.go`) constructs an `Interface` directly with
   `New`, drives one Hello, and calls `Stop`. It never touches `reconcile`,
   `interfaceGlobalParamsChanged`, `startInterfaceLocked` or a `reference-bandwidth`
   change. It is a correct and non-vacuous unit test of `(*Interface).Stop` (its recorded
   RED confirms discrimination), but as an AC-8b proof it is exactly the "proves `Stop`
   does what `Stop` does" shape.

   The first half is proven separately by `TestOSPFTopologyCostFollowsReferenceBandwidth`,
   whose `res.changed["eth0"]` assertion pins the reconcile branch that calls
   `startInterfaceLocked`. So the chain IS covered, by two tests that never meet, and
   nothing observes an adjacency being dropped by a reference-bandwidth change.

   Two narrower gaps sit inside this one. AC-8b says "already holds a **Full** adjacency";
   `ReceiveHello(peer, helloFor(cfg, cfg.RouterID), ...)` reaches 2-Way, and the test
   asserts `NeighborCount`, not the state. And AC-8b's second half, "it re-forms at the
   new cost", is proven NOWHERE: the Goal Validation row routes it to
   `ospf-auto-cost-frr`, and that scenario performs no reload. Its two operations
   (`internal/le/interoplab/bgp/checkers.go`, `check_extras.go`) are a wait for
   Full and two cost assertions. Either add a reload step to the scenario, or narrow the
   AC-8b row and the Goal Validation row to what is actually proven.

4. **`docs/architecture/ospf/ospf-4-component-config.md` still carries the false claim
   this spec recorded as BROKEN.** Its Traps section reads "**No synthetic device reports
   a link speed.** A veth, a dummy and a bond each expose no `speed` file in sysfs".
   Assumption A-2 in this spec records that as BROKEN, `interface_cost.go`'s header says
   "A veth is NOT in that set: the kernel prices one at 10 Gbit/s",
   `docs/guide/ospf.md` says "a veth reports 10000", and the whole
   `ospf-auto-cost-frr` scenario exists because a Docker veth reports 10000. The page and
   the code disagree, and the page is the wrong side. It is the exact claim that would
   tell the next reader an interop scenario for this feature is impossible. This page is
   named in the spec's own Files to Modify, so the correction belongs in this work
   (`ai/rules/documentation.md`, rung 2).

### NOTE

5. **The unknown-speed fallback claim is accurate at all three producers, and the guide
   clause is correct.** Verified: `vppBackendImpl.LinkSpeedDuplex`
   (`internal/plugins/iface/vpp/query.go`) returns `0, ""`;
   `stubBackend.LinkSpeedDuplex` (`internal/plugins/iface/netlink/backend_other.go`,
   `//go:build !linux`) returns `0, ""`; `LinkSpeedDuplex`
   (`internal/component/iface/dispatch.go`) returns `0, ""` when `GetBackend()` is
   nil. `parseLinkSpeedDuplex` (`internal/plugins/iface/netlink/show_linux.go`) maps
   a negative, unparseable or absent value to 0, so a bridge reporting -1 arrives as
   unknown rather than as a negative divisor. The guide clause names all three, anchors
   each on its producer, states the consequence (every interface with no `cost` costs 1)
   and gives the remedy (set `cost`). It is accurate and sufficient for what an operator
   must DO. What it does not say is that the leaf still bounces the router under those
   three conditions -- that belongs with finding 2 and disappears if 2 is fixed.

6. **`TestDerivedCostReachesLDPSyncAndTEMetric` observes the consumers; it does not
   re-compute the derivation beside them.** The LDP-sync arm reads
   `row.EffectiveMetric` out of `eng.ldpSyncSnapshot()`, and for a broadcast interface
   `ldpSyncManager.snapshot` (`internal/plugins/ospf/ldp_sync.go`) sets
   `metric := int(mc.cost)`, the stored restore value that `updateLDPSyncMachines`
   filled from `interfaceCost`. The TE arm decodes the originated Link LSA and
   reads `lsa.Link.TEMetric`, which `applyTELinkAttributes`
   (`internal/plugins/ospf/te_originate.go`) fills from `interfaceCost`. Both
   expected values (100, 10) are stated as literals and both are 1 without the
   derivation, which the recorded RED confirms. One caveat: the test prices eth0 for the
   LDP arm and eth1 for the TE arm, so it proves each consumer carries ITS derived cost.
   AC-9's stronger wording, "the same interface read four ways", is not what is asserted.

7. **`interfaceLinkSpeedMbps` is a package-level mutable `var` used as a test seam**
   (`internal/plugins/ospf/interface_cost.go`). `stubLinkSpeed` swaps it and restores it
   via `t.Cleanup`. It is documented and correct today, and it makes any test in this
   package that calls `t.Parallel()` race against it. No test in the package does, so
   this is an observation rather than a defect.

### Explicitly not found

- No wiring gap. `interfaceCost` has four non-test consumers
  (`instance.go` twice, `ldp_sync.go`, `te_originate.go`) and
  `types.DefaultMetric` now has a non-test caller. No consumer reads `ic.Cost` directly
  for an advertised metric: the only non-test `HasCost` read in the plugin's own package
  is in `interface_cost.go`.
- No arithmetic defect in the derivation. `DefaultMetric`
  (`internal/plugins/ospf/types/metric.go`) widens both operands to `uint64` before
  dividing, so the leaf's maximum (4294967) over the minimum divisor (1) cannot
  overflow, and `min(max(q, MetricMin), MetricMax)` makes cost 0 unreachable. RFC 2328
  Appendix C.3 is quoted above the lower clamp.
- No RFC violation. RFC 2328 Appendix C.3 requires only that the cost exceed 0 and
  defines no derivation from a link speed; the upper clamp is the two-octet wire field.
  RFC 3630 section 2.5.5's fallback still holds and now carries the derived cost.
- No fail-open guard, no unbounded allocation, no hot-path allocation, and no
  peer-reachable `panic()` in the changed code. The derivation is integer arithmetic on
  a control-plane origination pass.
- No test-rewrite coverage regression. `TestOSPFStopLeavesAllDRouters` is untouched and
  still asserts the multicast leave; the new test adds the neighbor consequence beside it.
- The interop scenario exists, is registered in both checker maps, and WOULD fail if the
  derivation broke: it asserts `Metric: 47` in FRR's `show ip ospf database router`
  alongside `zeLabAddress`, and `"cost": 47` in Ze's own `show ospf interface`. 47 is
  470000/10000 and no default produces it; the pre-auto-cost value is 1.

### Verdict

**Do not close.** Finding 1 is a correctness defect this spec introduced, in the feature's
own derivation, reachable from a documented configuration, with no operator workaround.
Finding 2 causes an avoidable adjacency outage for a config change that alters no metric.
Findings 3 and 4 are proof and documentation debt on claims the spec makes in its own
Goal Validation and Files to Modify. Fix 1 and 2 in product code, correct 4, then decide
between adding the reload step or narrowing AC-8b for 3, and re-run this gate over the
fixes only.

### Resolution (fix phase, 2026-09-09)

Every finding was re-read at the producer the review named before it was acted on.

| # | Verdict | Resolution |
|---|---------|------------|
| 1 BLOCKER | reproduced | `parseOSPFConfig` (`internal/plugins/ospf/config.go`) now inherits `ReferenceBandwidth` into `cfg.V6` and every `cfg.V6Extra` entry, unconditionally, because the `ospf-af-topology` grouping declares no leaf a sub-config could set. `TestReferenceBandwidthReachesEveryAddressFamily` reads cost 47 out of both families' origination topologies for one link under `reference-bandwidth 470000`; its RED against the old code read 10 for OSPFv3 |
| 2 ISSUE | reproduced | `interfaceGlobalParamsChanged` (`internal/plugins/ospf/instance.go`) compares `interfaceCost` on each side rather than the numerator, which subsumes the `ic.HasCost` arm as the review said. `TestInterfaceGlobalParamsChangedReferenceBandwidth` gains the truncating-quotient arm and the unpriced-link arm, both RED first. R-4 is marked MATERIALIZED |
| 3 ISSUE | reproduced | AC-8b now has `TestReferenceBandwidthReloadDropsAdjacencyAndReprices`, which drives `reconcile` over an interface holding a neighbor and observes the drop and the new cost. Two forced REDs recorded. The AC and the Goal Validation row are narrowed to what the test checks, and Known Limitations names what stays unproven: the re-form to Full, which needs a peer and a reload step the interop scenario does not perform |
| 4 ISSUE | reproduced | The false trap in `docs/architecture/ospf/ospf-4-component-config.md` is replaced by what the kernel reports, with `parseLinkSpeedDuplex` as a second source anchor. The same page gains the router-wide-numerator decision and the cost-comparison rule; `ospf-5-interface-ism.md`, `docs/guide/ospf.md` and the `reference-bandwidth` `ze:help` carry the two behavior changes |
| 5 NOTE | confirmed, no action | The three producers and the guide clause are accurate. Its one gap, that the leaf bounced the router where auto-cost prices nothing, disappeared with finding 2, and the guide now states the quotient rule |
| 6 NOTE | acted on | `TestDerivedCostReachesLDPSyncAndTEMetric` now reads ONE interface four ways. eth0 gains `traffic-engineering`, and a broadcast topology entry whose DR is this router gives its TE Link TLV the RFC 3630 section 2.5.2 multi-access Link ID. AC-9's wording is now what the test asserts |
| 7 NOTE | recorded, no action | `interfaceLinkSpeedMbps` stays a package-level test seam. No test in the package calls `t.Parallel()`, and the alternative (threading a speed reader through `engine` and every `interfaceCost` caller) adds a parameter to reach one test, which `ai/rules/simplicity.md` refuses. `stubLinkSpeed` restores it through `t.Cleanup`, and the file header states the reason it is a variable |

### Round 2, 2026-09-09, Opus 5

Independent review of the fixes in `aca27077f2` only, and of what they newly touched.
Every claim below was read at its producer. Round 1's other findings were re-checked.

**Counts: 0 BLOCKER, 2 ISSUE, 5 NOTE.**

#### BLOCKER

None. The round 1 BLOCKER is fixed on every path that builds an OSPFv3 config, not
only the one the new test drives. `parseOSPFConfig` (`internal/plugins/ospf/config.go`)
is the ONLY production producer of an `ospfConfig`: `register.go` calls it at the RPC
handler, at `OnConfigVerify` and at `OnConfigure`, and `doctor.go` and
`doctor_ipsec.go` call it for their checks. Every RFC 5838 child is built inside
`applyAddressFamilies`, which is reached only from `applyTree`, which is reached only
from `parseOSPFConfig`, so a child cannot be constructed after the inheritance runs.
The child then reaches an engine one way: `cfg.v6Families()` to
`v6EngineSet.configure`, `apply` or `start` (`internal/plugins/ospf/register_multiaf.go`),
each of which calls `(*engine).setConfig` or `(*engine).reconcile`, and `setConfig`
(`internal/plugins/ospf/instance.go`) stores `e.cfg = cfg` verbatim. So a reload, a
later reconcile and an address family added at reload all inherit: `apply` takes
`fam.cfg` out of the same freshly parsed `v6Families()`. `forInstance` copies by value,
so an OSPFv2 non-zero Instance ID inherits too. The inheritance is also safe
unconditionally: `defaultOSPFConfig` seeds 100000 and `applyTree` writes the leaf only
when `v > 0`, so the parent value is never 0.

#### ISSUE

1. **The interface restart is not what publishes the new cost, and it is now the only
   thing that refreshes a duplicate of it.** `interfaceGlobalParamsChanged`
   (`internal/plugins/ospf/instance.go`) returns true for a cost change, and `reconcile`
   answers that with `startInterfaceLocked`, which calls `old.Stop()` and drops the
   neighbors, the DR and the BDR of the interface. The Router-LSA does not need that.
   `lsdbTopology` derives `InterfaceInfo.Cost` from `e.cfg.ReferenceBandwidth` on EVERY
   origination pass, `routerLinks` (`internal/plugins/ospf/lsdb/origination.go`) and
   `v6RouterLSABody` (`internal/plugins/ospf/origination_v6.go`) take the link metric
   from that field, and `reconcile` replaces `e.cfg` before it touches an interface.
   This spec's own Known Limitations states the same thing from the other side: a
   carrier flap re-prices "on the next origination pass" with no restart at all.
   `updateLDPSyncMachines` (`internal/plugins/ospf/ldp_sync.go`) already re-prices its
   consumer in place from `e.cfg` at every reconcile.

   What the restart does refresh is `Interface.cfg.Cost`, a stored copy that
   `snapshotLocked` and `DetailSnapshot` (`internal/plugins/ospf/iface/iface.go`) hand
   to `interfaceSnapshot` (`internal/plugins/ospf/instance_snapshots.go`) for
   `show ospf interface`.

   Failure scenario. A router with 40 auto-costed 10 Gbit/s links. The operator lowers
   `reference-bandwidth` to spread the costs apart, which is User Story 2. All 40
   adjacencies are torn down at the commit, traffic re-routes for the dead interval plus
   a database exchange on each link, and the metric that reaches the wire is the one the
   next origination pass would have carried without the bounce. The same number changing
   because a link renegotiated 1G to 10G costs nothing.

   The Key Design Decisions row "Re-price through the existing interface restart"
   rejected an in-place re-price because it "needs a config-update method on
   `Interface`, which no parameter has today". There is a smaller shape than that: derive
   the cost in `Snapshot`/`DetailSnapshot` the way `lsdbTopology` already derives it,
   so the fact is declared once (`docs/contributing/ze-go-style.md`, "State that goes
   stale": do not copy a variable), drop the cost arm of
   `interfaceGlobalParamsChanged`, and have `reconcile` call `originateSelfLSAs`. That
   last part is load-bearing and is missing today: `reconcile` re-originates only
   indirectly, through the neighbor churn the restart causes, so removing the restart
   without adding the call would leave a re-priced interface with no adjacency waiting
   for the refresh timer.

   The spec says this decision is "Open for the owner to overrule". It has not been put
   to him. The round 1 fix narrowed the trigger and left the mechanism, so the question
   is now sharper, not answered.

   **RESOLVED 2026-09-09 by the owner. Thomas decided: re-price in place, a
   `reference-bandwidth` commit must not tear down an established adjacency.** The
   finding was correct on every point and is implemented, not merely recorded.
   `interfaceGlobalParamsChanged` no longer reads the numerator, `reconcile` re-prices
   the running interface through `repriceInterfaceLocked` and `(*Interface).SetCost`,
   and `reconcile` calls `originateSelfLSAs` before it returns so the commit publishes
   the metric. The review's suggested shape (derive the cost inside `Snapshot` /
   `DetailSnapshot`) was NOT taken: `Interface` holds no reference bandwidth and no link
   name resolution, so deriving there would import the engine's config into the ISM
   package. Writing the one field the snapshots read is the smaller change and keeps the
   derivation in `interfaceCost`, which is still the only place the rule lives. See
   "SETTLED by the owner" near the top of this spec.

2. **The reload test drives a one-way neighbor, and three places say 2-Way.**
   `receiveHello` (`internal/plugins/ospf/iface/iface.go`) sets
   `TwoWay: helloHasNeighbor(h, i.cfg.RouterID)`.
   `TestReferenceBandwidthReloadDropsAdjacencyAndReprices`
   (`internal/plugins/ospf/interface_cost_test.go`) builds its `types.Hello` with
   `HelloInterval`, `DeadInterval`, `Options` and `Priority` and NO `Neighbors` field,
   so `helloHasNeighbor` is false and the neighbor stays one-way (Init). The test passes
   anyway because `NeighborCount` is `len(i.neighbors)`, which counts a one-way neighbor.

   Three claims are therefore false: the test's own header comment ("The neighbor
   reaches 2-Way rather than Full"), AC-8b ("What the unit test drives is a 2-Way
   neighbor"), and Known Limitations ("The neighbor it drives reaches 2-Way"). AC-8b's
   "an interface that already holds an adjacency" over-states it further: an OSPF
   adjacency is ExStart or later, and this is neither an adjacency nor 2-Way.

   This is round 1 finding 3 landing one state too high. The fix is one field:
   `Neighbors: []types.RouterID{cfg.RouterID}` in the Hello, which is what
   `helloFor(cfg, cfg.RouterID)` (`internal/plugins/ospf/iface/iface_test.go`) does for
   `TestInterfaceStopClearsNeighbors`. That makes the three sentences true rather than
   editing three sentences to match a weaker test.

#### NOTE

3. **`interfaceGlobalParamsChanged` reads the link speed twice.** Each
   `interfaceCost` call re-enters `interfaceLinkSpeedMbps`
   (`internal/plugins/ospf/interface_cost.go`), which is two sysfs reads, so the
   predicate performs four per interface per reconcile and compares two independently
   sampled speeds. A renegotiation between the two reads reports a cost change that no
   config change produced, which is the class of needless bounce the fix removed. Cold
   path; reading the speed once and comparing two quotients over it closes it.

4. **No test drives an OSPFv3 reconcile after a `reference-bandwidth` change.** The
   inheritance makes `v6EngineSet.apply` pass a new numerator to
   `(*engine).reconcile` for each address family, so an OSPFv3 interface is now
   restarted by a re-price where it never was before.
   `TestReferenceBandwidthReachesEveryAddressFamily` covers the initial config only, and
   `TestReferenceBandwidthReloadDropsAdjacencyAndReprices` drives the v4 engine.

5. **The corrected trap in `docs/architecture/ospf/ospf-4-component-config.md` has
   in-repo evidence for one of its four devices.** `parseLinkSpeedDuplex`
   (`internal/plugins/iface/netlink/show_linux.go`) proves only that a negative,
   unparseable or absent value becomes 0, and `ospf-auto-cost-frr` proves the veth's
   10000. "a loopback and a dummy report nothing and a bridge reports -1" is kernel
   knowledge no test or command in this tree checks. It does not over-state in the old
   direction (the old claim denied the veth, which the scenario disproves) and all four
   devices land on cost 1 either way, so this is an observation rather than a defect.

6. **`TestDerivedCostReachesLDPSyncAndTEMetric`'s first TE arm now depends on eth0
   emitting no TE Link LSA.** eth0 gained `traffic-engineering`, and the loop over
   `eng.teOriginateType1` assigns `teMetric` on every matching LSA rather than selecting
   the interface it asserts about, so the arm that wants eth1's 10 is decided by
   iteration order the moment eth0 also emits one. It passes today because a broadcast
   interface with no DR emits none, which is why the second arm has to stub the
   topology. Selecting by `lsa.Link` interface identity would remove the dependency.

7. **One Goal Validation row is stale.** "An explicit `cost` is never overridden" still
   says `TestInterfaceGlobalParamsChangedReferenceBandwidth` "pins both arms". The test
   now has four, and R-4 says four.

#### Round 1 findings, re-checked

| # | Round 1 | Round 2 verdict |
|---|---------|-----------------|
| 1 BLOCKER | v6 pinned at 100000 | FIXED at the producer, on every path (see above) |
| 2 ISSUE | numerator comparison bounces at an unchanged cost | FIXED. `interfaceCost(ic, old) != interfaceCost(ic, new)` subsumes the `HasCost` arm exactly as claimed: `interfaceCost` returns `ic.Cost` on both sides when the leaf is set. The new predicate is a strict subset of the old one, so nothing that should bounce stopped bouncing; a change to the interface's own `cost` or `HasCost` is still caught by `interfaceParamsEqual`, which `reconcile` ORs with this call |
| 3 ISSUE | AC-8b proven by two tests that never meet | PARTLY. The new test does drive `parseOSPFConfig`, `reconcile`, `interfaceGlobalParamsChanged` and `startInterfaceLocked` in one path, and the AC and Goal Validation rows are narrowed. The neighbor state claim is wrong: ISSUE 2 above |
| 4 ISSUE | false trap in `ospf-4-component-config.md` | FIXED. The false sentence is gone and the replacement is anchored on `parseLinkSpeedDuplex`. See NOTE 5 |
| 5 NOTE | unknown-speed producers accurate | Still accurate; its one gap closed with finding 2 |
| 6 NOTE | AC-9 read two interfaces, not one | FIXED. eth0 is now read four ways. The fourth reading feeds a hand-built topology entry, but the metric comes from `applyTELinkAttributes` calling `interfaceCost` with the engine's real config (`internal/plugins/ospf/te_originate.go`), so the derivation is not stubbed |
| 7 NOTE | `interfaceLinkSpeedMbps` test seam | Unchanged, still no `t.Parallel()` in the package |

#### Explicitly not found

- No wiring gap. Nothing new is exported; the two changed functions have production
  callers (`register.go` for `parseOSPFConfig`, `reconcile` for
  `interfaceGlobalParamsChanged`).
- No weakened or deleted test in this commit. `./le commit audit base aca27077f~1`
  reports 21 findings and not one is under `internal/plugins/ospf/`; every changed test
  kept its old assertions and added to them.
- No stale copy of either corrected sentence anywhere else in the tree. The YANG
  description and both page paragraphs are the only carriers.
- No RFC violation introduced. The change moves a metric between two families and
  changes when an interface restarts; RFC 2328 Appendix C.3 constrains only the value.
- No hot-path allocation, no fail-open guard, no unbounded allocation. The predicate
  runs on the reconcile path with `e.mu` released.

#### Verdict

**Do not close yet, on ISSUE 2 alone.** It is one field in a test plus three sentences,
and leaving it publishes an AC and a Goal Validation row that claim more than the test
observes. ISSUE 1 was the owner's call: he answered it on 2026-09-09 (re-price
in place), and the answer is implemented. Everything else is a NOTE.

### Resolution (round 2 fix phase, 2026-09-09)

Every finding was re-read at the producer the review named before it was acted on.

| # | Verdict | Resolution |
|---|---------|------------|
| 1 ISSUE | reproduced, ANSWERED by the owner on 2026-09-09, and FIXED | `lsdbTopology` derives the cost from `e.cfg.ReferenceBandwidth` on every origination pass, so the restart is not what publishes the metric. Thomas decided on 2026-09-09 that a `reference-bandwidth` commit must tear down no adjacency, and that decision is implemented: `interfaceGlobalParamsChanged` keeps only its Router ID and area-type arms, `reconcile` re-prices in place through `repriceInterfaceLocked` and `(*Interface).SetCost`, and `reconcile` originates before it returns. Four proofs, each with a recorded RED, are in the TDD Evidence for the 2026-09-09 re-price phase. "SETTLED by the owner" near the top of this spec carries the decision |
| 2 ISSUE | reproduced, fixed | `receiveHello` was read at the producer first: `TwoWay: helloHasNeighbor(h, i.cfg.RouterID)`, and `helloHasNeighbor` is `slices.Contains(h.Neighbors, id)`. The test's Hello now carries `Neighbors: []types.RouterID{cfg.RouterID}`, and the claim is now OBSERVED rather than constructed: the test reads the state out of `eng.neighbors.Lookup` and asserts 2-Way before the reload and Down after it. Three forced REDs recorded. `Table.InterfaceDown` keeps the row and drops it to Down (RFC 2328 sec 10.2 KillNbr), so the assertion is on the state rather than on the row's absence. AC-8b, the test header and Known Limitations now say what the test checks; AC-8b's "adjacency" became "2-Way neighbor", which is the RFC 2328 section 10 term, and the behavior the row promises did not change |
| 3 NOTE | acted on | `interfaceGlobalParamsChanged` reads the link speed ONCE and prices both sides from that sample, through the new `interfaceCostAtSpeed` (`internal/plugins/ospf/interface_cost.go`). An explicit `cost` returns false before the read. A fifth test arm pins it with a reader that renegotiates between calls, and counts the reads. RED recorded |
| 4 NOTE | acted on | `TestReferenceBandwidthReloadRepricesEveryAddressFamily` drives an OSPFv3 engine through `reconcile` with the `v6Families` entry `v6EngineSet.apply` hands it. Two forced REDs recorded |
| 5 NOTE | acted on | `docs/architecture/ospf/ospf-4-component-config.md` now separates what the tree proves (the veth's 10000, through `ospf-auto-cost-frr`, and the absent-to-0 mapping, through `parseLinkSpeedDuplex`) from the kernel behavior it does not read (a loopback, a dummy and a bridge). The same bullet gains the single-sample rule from NOTE 3 |
| 6 NOTE | acted on | Both TE arms select the Link LSA by its RFC 3630 section 2.5.3 Local Interface IP Address, through the new `teMetricForLocalAddress` helper, so neither depends on the order `teOriginateType1` emits. RED re-observed with `interfaceCost` cut to the pre-auto-cost behavior: all five assertions still fail, so the selector cost the test no discrimination |
| 7 NOTE | acted on | The Goal Validation row now says five arms and names each one |

### Round 3, 2026-09-09, Opus 5

Independent review of `db3f18b0b` only, the re-price the owner ordered on 2026-09-09,
and of what it newly touched. Every claim below was read at its producer.

**Counts: 0 BLOCKER, 1 ISSUE, 4 NOTE.**

#### BLOCKER

None. The three product edits do what they say, and the ruling is implemented rather
than recorded.

`reconcile`'s new `default` arm is reached exactly when the interface already exists,
`interfaceParamsEqual(have, want)` holds and `interfaceGlobalParamsChanged` is false, so
the set of restarts shrank by exactly the numerator-driven ones and by nothing else.
`interfaceParamsEqual` compares `Cost` and `HasCost`, so pricing `want` in the default arm
gives the same number as pricing the `have` the engine keeps in `e.running`, which is what
`lsdbTopology` iterates: the runtime copy and the advertised metric cannot disagree.

`SetCost`'s safety claim holds at the readers, not by assumption. The only reads of
`i.cfg.Cost` in `internal/plugins/ospf/iface/` are `snapshotLocked` and `DetailSnapshot`,
both under `i.mu`, and `SetCost` writes under the same mutex. `neighborInterfaceConfig`
(`internal/plugins/ospf/instance.go`) carries no cost field, so the neighbor table holds no
copy to go stale. The sinks in `iface.go` are all invoked after `i.mu` is released, so the
new `e.mu` to `i.mu` order that `repriceInterfaceLocked` takes is the order
`interfaceSnapshot` already took and no reverse order exists.

Nothing else needed the restart. The three engine consumers of the cost are live-derived or
refreshed in place: `lsdbTopology` calls `interfaceCost` on every origination pass,
`applyTELinkAttributes` does the same for the RFC 3630 fallback, and
`updateLDPSyncMachines` recomputes `Cost` from `e.cfg.ReferenceBandwidth` and
`ldpSyncManager.reconcileTo` overwrites `mc.cost` on a machine that already exists, so the
LDP-sync restore value is not the stale duplicate it would have to be for the restart to
have been load-bearing.

The origination is correct for OSPF and not merely sufficient for the test.
`OriginateRouter` (`internal/plugins/ospf/lsdb/origination.go`) compares the encoded body
against the installed instance through `existingSelfBodyUnchanged` before it asks for a
sequence, so a reload that changes nothing bumps no sequence and floods nothing; a changed
body takes `rec.sequence.Next()` and installs, and a neighbor treats that as an ordinary
newer instance (RFC 2328 section 13), which changes no adjacency state. RFC 2328 section
12.4: "two instances of the same LSA may not be originated within the time period
MinLSInterval. This may require that the generation of the next instance be delayed by up
to MinLSInterval." `nextOwnSequenceForce` enforces that window and DROPS the attempt rather
than scheduling it, but the one-second loop in `startNeighborRetransmitLoop` calls
`originateSelfLSAs` on every tick, so a rate-limited re-price is delayed rather than lost.
`originateSelfLSAs` also returns early under `e.gr.suppressOrigination()`, so the new call
cannot originate during a graceful restart (RFC 3623 section 2).

#### ISSUE

1. **Two pages now tell the reader that a Router ID and an area type are the whole restart
   set, and `reconcile` restarts on thirteen more fields, `cost` among them.**
   `reconcile` (`internal/plugins/ospf/instance.go`) takes its restart branch on
   `!interfaceParamsEqual(have, want) || interfaceGlobalParamsChanged(oldCfg, newCfg, want)`.
   The diff narrowed the second half and says so accurately everywhere it names that
   function, but three sentences state the claim about the RELOAD rather than about the
   predicate:

   - `docs/guide/ospf.md`: "Two reloads do restart an interface, because both change what it
     stamps into the Hellos it sends: a new router id, and an area type."
   - `docs/architecture/ospf/ospf-5-interface-ism.md`: "A config reload that changes the
     router id or an area type recreates the runtimes, and those two are the whole set."
   - This spec's AC-8c row: "These two are the whole set."

   `interfaceParamsEqual` compares `Enabled`, `Passive`, `AreaID`, `NetworkType`, `Cost`,
   `HasCost`, `HelloInterval`, `DeadInterval`, `Priority`, `MTUIgnore`,
   `RetransmitInterval`, `TransmitDelay`, `Authentication`, `IPsec`, `LDPSyncEnabled` and
   `LDPSyncHoldDown`, and a change to any of them recreates the runtime and drops the
   neighbors of that interface.

   Failure scenario. An operator reads the guide paragraph this diff added, learns that
   re-pricing costs no adjacency, and sets `cost 5` on one interface to pin a path. That
   commit drops the adjacency on that interface, re-forms it over a dead interval and a
   database exchange, and nothing the operator read said it would. The asymmetry itself is
   defensible: Thomas answered for the router-wide leaf, the per-interface leaf bounces one
   interface rather than forty, and the spec's Known Limitations records it. Publishing
   "those two are the whole set" is what is not defensible, because it is the sentence a
   future implementer would rely on when deciding whether some other field needs a restart.

   The fix is prose, in this work: name `interfaceParamsEqual` as the other half in the ISM
   page, and give the guide one sentence saying a change to an interface's own `cost` still
   restarts that interface.

   For whoever extends the ruling to the sibling leaf: it is not a one-line change.
   `lsdbTopology` prices from `e.running`, and the default arm never writes
   `e.running[name] = want`, so a cost-only re-price would also have to update that map and
   split the restart branch into a cost-only arm and the rest.

#### NOTE

2. **The stated reason for the new `originateSelfLSAs` call is wider than the producer
   supports.** The `reconcile` comment ("would otherwise wait for an unrelated pass"),
   `docs/architecture/ospf/ospf-4-component-config.md` ("would otherwise carry the old
   metric until an unrelated event") and this spec's Key Design Decisions row ("reaches the
   wire at an unrelated later event") all read as an unbounded wait. On any router with one
   active interface it is at most one second: `startNeighborRetransmitLoop` starts when
   `activeInterfaces()` is non-empty and calls `e.originateSelfLSAs()` on every tick. The
   call is still right, and on a passive-only or loopback-only router it is the ONLY
   publisher, because `activeInterfaces()` skips both and the loop never starts. Reword to
   that, rather than dropping the call.

3. **No recorded red isolates the `originateSelfLSAs` call.** The four recorded reds break
   `interfaceGlobalParamsChanged`, `repriceInterfaceLocked` and `e.cfg`; proof 2's red is
   forced by withholding the reloaded config, which breaks the derivation everywhere and
   says nothing about the origination. The assertion does discriminate the line, since the
   LSDB otherwise holds the metric 100 the test originated before the reload, so this is a
   recording gap rather than a coverage gap. Two things make it worth a line anyway: the
   test's engine has that one-second loop running (`openInterfaces` started it), and the
   test sets `min-ls-interval-ms 1`, so on a host slow enough to put a second between the
   pre-reload origination and the post-reload assertions the loop publishes 10 by itself and
   the forced red would not reproduce.

4. **What the restart also refreshed, and nothing now does.**
   `interfaceRuntimeConfigLocked` re-reads `interfaceIPv4Address`, `interfaceNetworkMask`,
   `interfaceMTU` and `interfaceIndex` from the OS whenever it builds a runtime, so until
   this change a `reference-bandwidth` commit refreshed those four by accident. It does not
   reach the wire: `lsdbTopology` re-reads the address and the mask live on every
   origination pass, so only the runtime's own Hello fields are affected, and an address
   change with no link flap already left them stale before this diff. The accidental
   refresh is gone rather than a regression, and the underlying gap is pre-existing.

5. **`repriceInterfaceLocked` reads the link speed under `e.mu`.** `interfaceCost` calls
   `ifcomp.LinkSpeedDuplex`, which resolves the device and then asks the backend, a VPP
   binary-API round trip on a VPP dataplane. `interfaceGlobalParamsChanged` did that read
   with `e.mu` released. It is the same class as `interfaceRuntimeConfigLocked`, which
   already performs four OS reads under `e.mu`, and reconcile is a config-commit path, so
   this is an observation and not a defect.

#### The four weakened rows

All four are true, including both counts, which were checked mechanically rather than
taken. `TestReferenceBandwidthReloadRepricesEveryAddressFamily`: the row's "7 to 6" is the
detector's `fatalPattern` count (`t.Fatal*` 7 to 6, the one downgrade the row describes),
while its assertion count went 8 to 10. `TestOSPFTopologyCostFollowsReferenceBandwidth`:
assertion count 10 to 9 as stated. Nothing left the suite: no `Test` function was deleted,
both renamed tests kept their setup and their preconditions, `./le commit audit base
db3f18b0b~1` reports no finding under `internal/plugins/ospf/`, and no file in the tree
still references either old test name or `interfaceCostAtSpeed`. The read-count arm that
went with row 1 pinned a single speed sample inside a predicate that now reads no speed at
all, so it had nothing left to assert.

#### The four proofs

Three observe what they claim. `selfRouterLSAMetric` looks the installed Type-1 LSA up in
the LSDB and decodes it, so the metric it reads is the one `routerLinks` encoded;
`addressedTopology` substitutes only an address and a mask, which is what makes a stub link
exist at all, and leaves the derived cost untouched. The `show ospf interface` read is
`eng.interfaceSnapshot()`, which is the exact expression the `show ospf interface` case in
`register.go` returns, and it reaches `i.cfg.Cost` through `Interface.Snapshot`, the field
`SetCost` writes. The neighbor proof reads the state out of `eng.neighbors.Lookup` on both
sides of the reload.

The fourth is narrower than the commit message says. "The Router-LSA carries the new cost in
both families" is proven for OSPFv2 only:
`TestReferenceBandwidthReloadRepricesEveryAddressFamily` asserts the origination topology
and the interface snapshot at 23, never a decoded OSPFv3 Router-LSA. The spec's Known
Limitations states this correctly, so the over-claim is in the commit body alone.

#### Verdict

**Close it, after the prose fix in ISSUE 1.** The product change is correct at every
producer read: the arm fires where intended, the one field it writes has no live reader, the
consumers that were refreshed by the restart are refreshed without it, and the origination is
conformant. What remains is a sentence on an operator page and one in an architecture page
that promise more than `reconcile` delivers, plus four observations that change no behavior.

### Round 4 resumed, 2026-09-10

Scope: the complete feature's AC producers and the resumed repair in
`instance.go`, `virtual_link.go`, and `origination_lifecycle_test.go`, including
their shutdown and self-LSA publication paths. Thomas authorized the available
independent reviewer for this continuation, with no authorization for a sixth
round. The inherited lifecycle test was moved out of `instance_test.go`; its
existing RFC6549 test remains unchanged.

#### Findings fixed

| Severity | Producer | Failure | Source repair |
|----------|----------|---------|---------------|
| ISSUE | `openInterfaces`, `openConfiguredInterface`, `LSDB.nextOwnSequenceForce` | Passive-only and loopback-only engines never retried a rate-limited re-price | Both enrollment paths start the existing maintenance worker |
| ISSUE | `nsmAdapter.InterfaceDown`, `originateSelfLSAsDeferred` | Each callback started a separate unjoined goroutine | One pending slot is consumed by the engine-owned worker; `spawnMu` orders registration and cancellation, and `shutdown` joins it |
| ISSUE | `TestDeferredOriginationJoinsTheEngine` | Removing the sole interface left no area to originate, so the late-callback assertion could not detect illegal work | The test observes topology entry, blocks admitted work while shutdown joins it, and counts reads after teardown |

RFC 2328 Section 12.4 states: "When whatever is being described by an LSA
changes, a new LSA is originated." The following sentence forbids two instances
within MinLSInterval. The repair preserves that limit and supplies its missing
retry source. Bypassing the limit would publish sooner but violate the protocol.

### Round 5, 2026-09-10: CLEAN

Independent closure context: `openai-codex/gpt-6-astra`, with Thomas's model
override. The native recorder could not identify the running model and reported
that boundary as UNCHECKED; the reviewer identity and authorization are recorded
here and in its findings file. No sixth round ran.

| Field | Value |
|-------|-------|
| Final result | 0 BLOCKER, 0 ISSUE |
| Artifact | `tmp/review/ospf-auto-cost-reference-bandwidth-5bc855cc-5761-4012-8d90-7a9823029ab3.md` |
| `spec session review check` | OK: 3 closure code files, clean, hashes match; native model-detection note retained |
| Rounds | 5 |
| Reviewer lenses used | Logic and wiring; security and bounds; lifecycle, publication, tests, RFC behavior, style and documentation |
| Findings file | `tmp/session/2026-09-09-5bc855cc-5761-4012-8d90-7a9823029ab3/scratch/ospf-round5-review.txt` |

`openInterfaces` and `openConfiguredInterface` cover both enrollment paths.
`startNeighborRetransmitLoop` retains the one-second retry and owns one pending
notification; `originateSelfLSAsDeferred` never originates under the caller's
engine lock. `shutdown` cancels under the registration mutex and waits outside
it, and `originateSelfLSAs` rejects canceled work before and after acquiring
`mu`. Physical and virtual interfaces both use that path. The new channel has
capacity one, the worker has an explicit cancellation lifetime, and the change
adds no peer-controlled allocation or panic.

`interfaceCost`, `DefaultMetric`, `parseOSPFConfig`, `repriceInterfaceLocked`,
`updateLDPSyncMachines` and `applyTELinkAttributes` still implement the cost
contract. `nextOwnSequenceForce` retains MinLSInterval and `OriginateRouter`
retains unchanged-body suppression. The style pass examined all changed Go,
including paired lifecycle obligations, bounds and return widths. The guide
and both changed architecture pages describe those producers accurately.

#### Resumed proof

All log names below are under
`tmp/session/2026-09-09-5bc855cc-5761-4012-8d90-7a9823029ab3/scratch/`.
The overlays preserve the inherited and mutated sources without reverting the
shared checkout.

| Evidence | Observed result |
|----------|-----------------|
| `job-ospf-publication-inherited-d0d646e3.log` | RED: all four passive/loopback initial/reload-added cases retain metric 100 instead of 10 against the saved inherited source |
| `job-ospf-unjoined-red-3587ae2e.log` | RED: shutdown returns while origination holds a topology read |
| `job-ospf-late-red-020c7074.log` | RED: late callback reads topology 3 times instead of the 2 recorded at shutdown |
| `job-ospf-fixed-tree-3c4f78c3.log` | GREEN: complete OSPF subtree under the race detector, including all new tests and the existing feature tests |
| `ospf-fixed.cover` | Statement coverage only; the root OSPF package reports 80.7%, with no claim of full coverage |
| Native selected FRR interop action | Exit 0, `integration: 1 action(s) passed.` Startup cost regression evidence; no live-peer reload proof |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 excluded container interop | An up veth reports 10000 Mbit/s | Recorded kernel measurement and FRR cost assertion | Added the discriminating live-peer scenario |
| approach | An immediate reconcile origination was treated as publication | MinLSInterval can reject the attempt and passive-only engines had no retry worker | Four inherited-source RED cases | Start maintenance for every enrolled interface |
| approach | Deferred callbacks used independent goroutines | Shutdown could return while topology was still read | Join-removal and late-admission RED overlays | Coalesce callbacks into the joined worker |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Derive interface cost from bandwidth | Done | `interface_cost.go` `interfaceCost`; `types/metric.go` `DefaultMetric` | Explicit cost wins; unknown speed falls back to 1; quotient clamps |
| Preserve adjacency on reference-bandwidth reload | Done | `instance.go` `reconcile`, `repriceInterfaceLocked`; `iface/iface.go` `SetCost` | Router ID, area type and interface-block restart rules remain |
| Publish the latest cost after rate limiting | Done | `instance.go` `startNeighborRetransmitLoop` | Passive and loopback enrollment is covered at startup and reload |
| Bound and join deferred origination | Done | `instance.go` `originateSelfLSAsDeferred`, `shutdown`; `virtual_link.go` `startVirtualInterface` | One pending notification and cancellation-ordered registration |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestInterfaceCostAutoDerivation` | Truncating quotient |
| AC-2 | Done | `TestDefaultMetric` | Floor at 1 |
| AC-3 | Done | `TestInterfaceCostAutoDerivation`, `TestDefaultMetric` | Ceiling at 65535 |
| AC-4 | Done | `TestInterfaceCostAutoDerivation` | Explicit cost with known and unknown speed |
| AC-5 | Done | `TestInterfaceLinkSpeedMbpsUnknownName`, `TestInterfaceCostAutoDerivation` | Documented backend fallback |
| AC-6 | Done | `TestInterfaceCostAutoDerivation`, `TestDefaultMetric` | Zero numerator takes the fallback; native YANG rejects zero |
| AC-7 | Done | `defaultOSPFConfig`, `TestInterfaceCostAutoDerivation` | Default 100000 prices 1 Gbit/s at 100 |
| AC-8 | Done | `TestOSPFTopologyCostFollowsReferenceBandwidth`, `TestReferenceBandwidthReloadPublishesAfterMinLSInterval` | Every enrollment mode eventually publishes |
| AC-8b | Done | `TestReferenceBandwidthReloadKeepsNeighborAndReprices` | Runtime retained, 2-Way neighbor retained, v2 installed metric and snapshot move |
| AC-8c | Done | `TestInterfaceGlobalParamsChangedIgnoresReferenceBandwidth`, `TestReferenceBandwidthReloadKeepsNeighborAndReprices` | Identity/area predicate retained; identity restart exercised |
| AC-9 | Done | `TestDerivedCostReachesLDPSyncAndTEMetric` | Same interface read through four consumers |
| AC-10 | Done | `ospf-auto-cost-frr` | FRR reads 47; inherited discrimination recorded above |
| AC-11 | Done | `TestReferenceBandwidthReachesEveryAddressFamily`, `TestReferenceBandwidthReloadRepricesEveryAddressFamily` | v3 evidence is topology and snapshot, with no decoded v3 reload LSA claim |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All named cost, family, reload and consumer tests | Done | `internal/plugins/ospf/interface_cost_test.go` | PASS in fixed race-suite log |
| Arithmetic boundaries | Done | `internal/plugins/ospf/types/metric_test.go` | PASS in fixed race-suite log |
| Interface stop consequence | Done | `internal/plugins/ospf/iface/iface_test.go` | Existing multicast test retained beside neighbor consequence |
| Rate-limited publication and shutdown lifecycle | Done | `internal/plugins/ospf/origination_lifecycle_test.go` | Discriminating RED overlays and fixed GREEN |
| Native config and peer behavior | Done | `test/ospf/ospf-config.ci`, `test/interop/scenarios/ospf-auto-cost-frr/` | Earlier functional evidence retained; fresh interop regression passed |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Cost implementation and consumer files | Done | `interface_cost.go`, `config.go`, `instance.go`, `iface/iface.go`, `ldp_sync.go`, `te_originate.go`, `types/metric.go` are present; earlier implementation commits preserved |
| Cost and interface tests | Done | `interface_cost_test.go`, `types/metric_test.go`, `iface/iface_test.go` present |
| Schema and backend comment | Done | `yang/ze-ospf-conf.yang`, `internal/plugins/iface/netlink/show_linux.go` present |
| Interop registration and configs | Done | Both checker maps and both `ospf-auto-cost-frr` configs are committed |
| Operator and architecture pages | Done | Guide, component-config and ISM documentation present; resumed repair also updates LSDB/flooding architecture |
| Resumed repair additions | Done | `origination_lifecycle_test.go` and `virtual_link.go` callback wiring included in closure commit A |

### Audit Summary

All 13 acceptance rows are implemented. No in-scope product requirement is
partial or skipped. The earlier implementation commits remain part of the
feature history; commit A preserves this final audit and the resumed repair.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None within the settled acceptance criteria | Every AC producer and the resumed repair are complete | Not applicable |

The evidence limits remain explicit: no test holds a live peer Full across a
reference-bandwidth reload, and the v3 reload proof reads topology and snapshot
rather than a decoded v3 Router-LSA. The fresh FRR run performs no reload.
Those limits do not become stronger proof at closure. Per-interface `cost`
still restarts its interface; Thomas reserved that separate decision until
after this spec closes. VPP/non-Linux speed fallback remains the documented
boundary, with no backend-speed feature added here.

## Pre-Commit Verification

Observed product checks were reused under `ai/rules/pre-release.md`; this
independent closure context did not rerun them solely for a commit.

### Files Exist

| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/ospf/interface_cost.go`, `interface_cost_test.go`, `origination_lifecycle_test.go` | Yes | Source bodies read during round 5 |
| `test/ospf/ospf-config.ci` and both `test/interop/scenarios/ospf-auto-cost-frr/` configs | Yes | Full config and scenario files read during round 5 |

### AC Verified

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 through AC-7 | Arithmetic, precedence and fallback | Producers read in round 5; named tests PASS in fixed race log |
| AC-8, AC-8b, AC-8c | Re-price without adjacency teardown, retain real restarts and retry publication | Runtime/LSDB assertions and all four rate-limited cases PASS in fixed race log; inherited-source RED recorded |
| AC-9 | Cost consumers agree | `TestDerivedCostReachesLDPSyncAndTEMetric` PASS |
| AC-10 | Live peer sees the derived cost | Selected FRR native action exit 0; startup-only regression |
| AC-11 | Numerator inheritance and reload | Both named address-family tests PASS |

### Wiring Verified

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config validation | `test/ospf/ospf-config.ci` | `ze config validate` covers the existing reference-bandwidth leaf |
| Runtime config/reload and show | `test/interop/scenarios/ospf-auto-cost-frr/` plus engine regression tests | `runOSPFEngine` parses config, routes apply to instance/family reconcile, and returns `interfaceSnapshot` for show; the live scenario checks cost 47 |
| Physical/virtual interface down | Engine lifecycle regression | Both production sink constructors supply `originateSelfLSAsDeferred`; the admitted-work and late-callback assertions discriminate |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Existing iface-component imports and production `interfaceLinkSpeedMbps` reader |
| A-2 | broken | Recorded up-veth measurement 10000 and peer cost 47; Mistake Log and Deviations record the change |
| A-3 | confirmed | `defaultOSPFConfig` seeds 100000; parser retains it when absent |
| A-4 | confirmed | Earlier scenario search and recorded `ospf-auth-frr` regression; the new selected interop action also passes |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Cost derivation, fallback and family inheritance | `interfaceCost`, `DefaultMetric`, `parseOSPFConfig` | Guide and component-config architecture read against producers |
| Reload and deferred publication | `reconcile`, `openConfiguredInterface`, `startNeighborRetransmitLoop` | Guide and component-config/LSDB-flooding pages carry updated source anchors |
| Joined callback lifecycle | `originateSelfLSAsDeferred`, `shutdown`, `startVirtualInterface` | LSDB/flooding constraints describe lock order, coalescing and join |
| CLI/API/schema/inventory | Existing show handler and existing YANG leaf | No new command, RPC, wire field, plugin, runtime dependency or inventory item |
| Native documentation gate | Main's `doc check verify`, exit 1 | 3759 drift findings, eight summary-rule failures and unrelated anchors remain red; no repair loop |
| Other native checks | Main's repository check and commit audit | 24 BGP wiring findings and 16 concurrent changed-test findings remain red, outside this closure |
| Lint | Main's native lint run remains in flight at closure preparation | No clean result claimed; no rerun started by the closure context |
