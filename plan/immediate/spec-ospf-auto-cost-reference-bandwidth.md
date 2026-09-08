# Spec: ospf-auto-cost-reference-bandwidth

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | 1/3 |
| Handoff | - |
| Updated | 2026-09-06 |

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
- [ ] `internal/plugins/iface/netlink/show_linux.go` - the backend reads `/sys/class/net/<name>/speed`, and a non-positive value becomes 0

**Behavior to preserve:** (unless the user explicitly said to change it)
- An interface with an explicit `cost` advertises that cost, whatever the link speed.
- An interface whose link speed the kernel does not report advertises 1, which is what every unconfigured interface advertised before.
- `DefaultMetric` keeps its name and its floor at `MetricMin`.

**Behavior to change:** (only what the user asked for)
- An interface with no `cost` on a link the kernel prices is now costed `reference-bandwidth / speed`, clamped to 1..65535. Under the default 100000 a 10 Gbit/s link costs 10 rather than 1.
- `DefaultMetric` now takes both operands in Mbit/s (it took bits per second) and clamps at `MetricMax` instead of erroring above it. `types.DefaultReferenceBandwidth`, a bits-per-second constant with no non-test user, is deleted; `config.DefaultReferenceBandwidth` (Mbps) is the one declaration.
- `interfaceGlobalParamsChanged` takes the whole `interfaceConfig` rather than the Area ID alone, so a `reference-bandwidth` change restarts the interfaces it re-prices.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `ospf/reference-bandwidth`, a uint32 in Mbps, range 1..4294967, default 100000.
- Kernel: `/sys/class/net/<name>/speed`, read through the iface component.

### Transformation Path
1. `applyTree` (`internal/plugins/ospf/config.go`) writes the leaf into `cfg.ReferenceBandwidth`.
2. `interfaceCost` (`internal/plugins/ospf/interface_cost.go`) returns `ic.Cost` when `ic.HasCost`, otherwise calls `interfaceLinkSpeedMbps` and `types.DefaultMetric`.
3. `interfaceLinkSpeedMbps` calls `ifcomp.LinkSpeedDuplex`, which reaches the netlink backend's sysfs read.
4. `types.DefaultMetric` divides and clamps to `[MetricMin, MetricMax]`, and returns `ErrOutOfRange` for a zero operand.
5. Four consumers read the result: `lsdbTopology` (what the Router-LSA advertises), `interfaceRuntimeConfigLocked` (what `show ospf interface` reports), `updateLDPSyncMachines` (the restore value), and `applyTELinkAttributes` (the RFC 3630 TE metric fallback).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF plugin ↔ iface component | `ifcomp.LinkSpeedDuplex(name)`, the same import `interface_addr.go` and `origination_v6.go` already make on the same pass | Yes -- `TestInterfaceLinkSpeedMbpsUnknownName` calls the production reader |
| Ze ↔ an outside OSPF implementation | The derived cost is what FRR reads out of Ze's Router-LSA | Yes -- `ospf-auto-cost-frr` |

### Integration Points
- `types.DefaultMetric` - the derivation that existed and had no production caller; this spec gives it one.
- `interfaceGlobalParamsChanged` - already restarted an interface on a Router ID or area-type change; a re-pricing joins that set.

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
| R-4 | A reference-bandwidth change bounces an adjacency that did not need it | An explicitly-costed interface restarts | `interfaceGlobalParamsChanged` returns false when `ic.HasCost`; `TestInterfaceGlobalParamsChangedReferenceBandwidth` pins both arms |

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
| A config reload lowering the leaf | → | `reconcile` → `interfaceGlobalParamsChanged` | `TestInterfaceGlobalParamsChangedReferenceBandwidth` |
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
| AC-5 | An interface whose link speed the kernel does not report | Cost is 1 |
| AC-6 | A `reference-bandwidth` of 0 | Cost is 1 |
| AC-7 | The default config, no `reference-bandwidth` stated | 100000 applies, so a 1 Gbit/s link costs 100 |
| AC-8 | A reload lowering `reference-bandwidth` | An interface with no `cost` is restarted and re-priced; one with an explicit `cost` is left alone |
| AC-9 | The same interface read four ways | The Router-LSA metric, `show ospf interface`, the LDP-sync restore value and the TE metric fallback all carry the same number |
| AC-10 | An OSPF peer reads Ze's Router-LSA | The peer sees the derived cost, not 1 |

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
| `TestInterfaceGlobalParamsChangedReferenceBandwidth` | `internal/plugins/ospf/interface_cost_test.go` | AC-8, both arms | pass |
| `TestDefaultMetric` | `internal/plugins/ospf/types/metric_test.go` | the derivation and both clamps, in Mbit/s | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `reference-bandwidth` (Mbps) | 1..4294967 (YANG) | 4294967 | 0 is refused by YANG; a 0 reaching `interfaceCost` yields cost 1 (AC-6) | 4294968 refused by YANG |
| derived cost | 1..65535 | 65535 | a quotient of 0 clamps up to 1 (AC-2) | a quotient above 65535 clamps down to 65535 (AC-3) |
| link speed (Mbit/s) | 0 or positive | any positive value | 0 means unknown and yields cost 1 (AC-5) | N/A |

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
- `internal/plugins/ospf/instance.go` - both cost copies deleted, `interfaceGlobalParamsChanged` takes the interface
- `internal/plugins/ospf/ldp_sync.go` - third cost copy deleted
- `internal/plugins/ospf/te_originate.go` - fourth cost copy deleted, the reference bandwidth threaded to the TE metric fallback
- `internal/plugins/ospf/types/metric.go` - `DefaultMetric` takes Mbit/s and clamps at both ends; the unused bits-per-second constant deleted
- `internal/plugins/ospf/types/metric_test.go` - rewritten for the Mbit/s contract and both clamps
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - the `reference-bandwidth` `ze:help`, which said the leaf changes no advertised metric
- `internal/plugins/iface/netlink/show_linux.go` - the doc comment named flow-export as the only caller, and claimed no virtual device reports a speed. Both were wrong after this change, and the second was wrong before it
- `docs/guide/ospf.md` - a new "Interface cost" section
- `docs/architecture/ospf/ospf-4-component-config.md` - the one-function decision, the no-cache decision, and two traps
- `internal/le/interoplab/bgp/checkers.go`, `internal/le/interoplab/bgp/check_extras.go` - the scenario's assertions (NOT COMMITTED, see Known Limitations)

## Files to Create
- `internal/plugins/ospf/interface_cost.go` - `interfaceCost`, `interfaceLinkSpeedMbps`, `costLinkSpeedUnknown`
- `internal/plugins/ospf/interface_cost_test.go` - the derivation, the wiring and the reload behavior
- `test/interop/scenarios/ospf-auto-cost-frr/ze.conf`, `frr.conf` - the interop scenario (NOT COMMITTED, see Known Limitations)

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
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md`, new "Interface cost" section |
| 7 | Wire format changed? | No | The Router-LSA link metric field is unchanged; the value Ze puts in it changed |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 2328 Appendix C.3 defines no derivation from a link speed, so no requirement id is claimed. The RFC 3630 section 2.5.5 fallback still holds and its wording in `te_originate.go` was updated to say the fallback now carries the derived cost |
| 10 | Test infrastructure changed? | No | The scenario uses existing operation kinds |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not enumerate cost derivation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-4-component-config.md`, declared by `interface_cost.go`'s `// Design:` header |
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
3. **Phase: Reload** -- re-price on a reference-bandwidth change, and only where it applies
   - Tests: `TestInterfaceGlobalParamsChangedReferenceBandwidth`
   - Files: `instance.go`
   - Verify: an auto-cost interface restarts, an explicitly-costed one does not
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

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- **The interop scenario is written, run and proven, but NOT COMMITTED.** Its two
  registration files, `internal/le/interoplab/bgp/checkers.go` and
  `internal/le/interoplab/bgp/check_extras.go`, carry another session's
  uncommitted RFC 7854 `scenarioStatisticsPMACCT` work in the same hunks, and a
  commit naming either file would carry that work too. `interoplab.Discover`
  errors on a scenario directory with no checker, so committing
  `test/interop/scenarios/ospf-auto-cost-frr/` alone would break
  `./le integration interop` for every session. Both stay in the working tree
  until the BMP session commits, then land together.
- The derivation is per interface and has no per-area or per-address-family
  override. `reference-bandwidth` is one instance-wide number, which is what the
  leaf declares.
- A carrier flap re-prices on the next origination pass rather than immediately.
  Nothing schedules an origination for a speed change alone.

## RFC Documentation (Scope: protocol)

`internal/plugins/ospf/types/metric.go` carries the one requirement this spec
enforces, above `DefaultMetric`'s lower clamp: RFC 2328 Appendix C.3 states that
the interface output cost "must always be greater than 0". The upper clamp is a
wire-format bound rather than an RFC rule: the Router-LSA link metric field is
two octets wide.

RFC 2328 defines NO derivation of a cost from a link speed, so auto-cost is a
convention and no requirement id in `rfc/short/rfc2328.md` is claimed, changed or
newly proven by this spec.

RFC 3630 section 2.5.5 makes the TE metric default to the OSPF interface cost.
That fallback is unchanged and now carries the derived cost, which
`applyTELinkAttributes` states in its doc comment.

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
