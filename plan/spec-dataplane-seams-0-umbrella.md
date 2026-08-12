# Spec: dataplane-seams-0 -- Backend Neutrality at the Control-Plane / Dataplane Boundary (Umbrella)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/dataplane-seams.md` (create on the first deferral) |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A review on 2026-08-07 of every boundary where ze's control plane meets a
dataplane found five separable items: two concrete changes (children 1 and 2),
one anticipated extension whose harder half is a new design question (child 5),
one that needs research before it is known to be a change at all (child 3), and
one that is purely a design decision (child 4). They are grouped here because
they share one cause.

**The cause.** Ze has two kinds of dataplane seam, and they aged differently.

| Seam kind | Example | State today |
|-----------|---------|-------------|
| Serialized across a process boundary | `(system-rib, best-change)` carrying `BestChangeBatch` | Backend-neutral. Carries `netip.Prefix`, `netip.Addr`, label slices and typed actions. No backend handle appears in it |
| In-process Go interface | `iface.Backend` | Carries both Linux-only and VPP-only operations, and each backend rejects the other's |

The serialized seam has an external consumer: `BestChangeBatch.MarshalJSON`
exists because external FIB plugin processes decode the payload. A backend
handle cannot pass through JSON to another process without the author noticing.
The in-process seams were never asked that question.

**The goal.** Hold the in-process seams to the same standard as the serialized
one, remove the one leak that did reach the serialized seam, and decide whether
a shared control-packet receive path should exist.

This is an umbrella. Each child below is independently pickable and
independently closable. Child 4 is a design question and MUST NOT block the
other four.

### Findings (verified 2026-08-07, each against the producing function)

| # | Finding | Producer | Child |
|---|---------|----------|-------|
| F-1 | `sysribevents.RouteType` uses values 1/6/7/8, documented "Values match Linux RTN_ constants for direct mapping in the kernel backend". Linux numbering inside a payload that external plugin processes decode | `RouteType` in `internal/component/sysrib/events/events.go` | 1 |
| F-2 | `iface.Binding.Ifindex` is an `int` documented "the kernel interface index of the resolved OS device". The VPP iface backend fills the same `InterfaceInfo.Index` with its `sw_if_index`. One field, two namespaces, and the guard against confusing them is a function each caller must remember to call | `Binding` in `internal/component/iface/iface.go`; `int(d.SwIfIndex)` in `internal/plugins/iface/vpp/query.go`; `ActiveBackendName` in `internal/component/iface/backend.go` | 2 |
| F-3 | `BestChangeEntry` and `ECMPPath` have no egress-interface field. Routes cross that seam as next-hop IP only. Static routes CAN name an interface, but they reach the dataplane by a different path: `internal/plugins/static` declares its own `routeBackend` with netlink and VPP implementations, imports no sysrib, and separately emits `redistevents`. Whether the missing field is a gap or a deliberate boundary is an open question child 3 answers first, and cancelling that child is a legitimate outcome | `BestChangeEntry` in `internal/component/sysrib/events/events.go`; `routeBackend` in `internal/plugins/static/backend.go`; `emitRouteChange` in `internal/plugins/static/inject.go` | 3 |
| F-4 | Ten protocols each open their own receive socket, with four unrelated per-packet metadata structs and no shared type. No receive path anywhere carries a VLAN tag: `PACKET_AUXDATA`, `TP_STATUS_VLAN` and `VlanTCI` do not appear in the tree. DHCP learns neither its source nor its ingress interface | `serveMulti` in `internal/plugins/dhcpserver/register.go`; `transport.Inbound` (bfd), `wire.RawPacket` (ospf), `transport.RawFrame` (isis), `packet.RxMeta` (vrrp) | 4 |
| F-5 | `copp` polices TCP only. Its translation hardcodes a TCP protocol match in both the trusted-source term and the rate-limit term, so DHCP, ND, OSPF, IS-IS, BFD and PPPoE are unpoliced. This is the extension the copp design anticipated, not a defect. What that design did NOT anticipate: its `FamilyInet` choice means an nft `inet` table cannot reach ARP, IS-IS or PPPoE at all, since none is IP. That half is a new design question | `translatePolicy` in `internal/plugins/copp/translate.go`; `docs/architecture/traffic/cp-survival-2-copp-port179.md` | 5 |

### Children

| # | Spec | Size | Coordinates with |
|---|------|------|------------------|
| 1 | `plan/spec-dataplane-seams-1-route-type-numbering.md` | small | - |
| 2 | `plan/spec-dataplane-seams-2-backend-typed-index.md` | medium | the static-interface-nexthops ruling, `docs/architecture/iface/logical-name-resolution.md` |
| 3 | `plan/spec-dataplane-seams-3-route-egress-interface.md` | research first, may cancel | `plan/spec-fib-depth.md` (in-progress) |
| 4 | `plan/spec-dataplane-seams-4-control-packet-rx.md` | design only | `spec-cp-survival-0-umbrella` (closed) |
| 5 | `plan/spec-dataplane-seams-5-copp-non-tcp.md` | medium | `spec-cp-survival-0-umbrella` (closed; owned copp) |

### Decisions taken at the scope gate (2026-08-07, user)

| Decision | Chosen | Rejected |
|----------|--------|----------|
| D-1 Spec shape | Umbrella plus five independently closable children | One flat spec; two specs split by size; mechanical items only |
| D-2 Child 2 direction | Keep one resolver and keep `ActiveBackendName`. Make the raw index reachable only through an accessor that names the target dataplane and errors on mismatch, so a caller cannot skip the check by forgetting it | Reopening the one-resolver design; leaving it as an accepted risk |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - how components and plugins are meant to be isolated
- [ ] `docs/features/interfaces.md` - named by the `// Design:` annotation on `internal/component/iface/backend.go`
- [ ] `ai/rules/architecture.md` - tier rules and where a config-driven engine belongs
- [ ] `ai/rules/plugins.md` - registration over hardcoding; no plugin spelling in generic packages

### Learned Summaries
- [ ] The static-interface-nexthops ruling (record retired with the learned corpus) - the ruling that produced F-2
  → Decision: one resolver serves both dataplanes on purpose. The VPP iface backend publishes its `sw_if_index` through `iface.InterfaceInfo.Index` into `Binding.Ifindex`, and no second resolution path may be introduced.
  → Decision: `ActiveBackendName()` was chosen over a config-verify pairing check specifically because `LoadBackend` swaps the backend live at runtime. A config-time check therefore cannot replace it.
  → Constraint: a zero or invalid resolved index must be rejected, never emitted. Index 0 is VPP `local0`.
  → Constraint: the summary's Consequences section already instructs future authors to gate on `ActiveBackendName()`. Child 2 changes the enforcement, not this decision.
- [ ] `docs/architecture/iface/logical-name-resolution.md` - `Binding` is a pure value type; no second resolver

### Related Specs
- [ ] `plan/spec-fib-depth.md` - in-progress, owns FIB programming depth; child 3 lands in its territory
- [ ] `spec-cp-survival-0-umbrella` - closed 2026-08-12; it owned copp, and children 4 and 5 must not contradict its closure record (read it from git history)
- [ ] `plan/spec-lifecycle-invariants.md` - ready, written by the same cross-cutting review method

**Key insights:** (minimal context to resume after compaction)
- The route seam is an event, not an interface, and it is serialized for external plugin processes. That is why it stayed neutral, and it is why changing it (child 3) is a wire-contract change rather than a refactor.
- F-2 is not an oversight. It is a recorded decision (learned 1185) whose enforcement child 2 strengthens.
- Children 1, 2 and 5 touch no external contract. Child 3 does. Child 4 decides whether a contract should exist.

## Current Behavior (MANDATORY)

**Source files read:** (2026-08-07, verified against the producing function)
- [ ] `internal/component/sysrib/events/events.go` - defines `RouteType`, `ECMPPath`, `BestChangeEntry`, `BestChangeBatch` and its `MarshalJSON`. `BestChangeEntry` fields are action, prefix, next-hop, protocol, labels, route type, metric, table id, SRv6 SID, ECMP paths, backup paths
- [ ] `internal/component/iface/backend.go` - defines `Backend` (about 40 methods), `VLANSpec` (single tag: parent plus one VLAN id), `RegisterBackend`, `LoadBackend`, `ActiveBackendName`, `GetBackend`, `CloseBackend`. Exactly one backend is active at a time
- [ ] `internal/component/iface/iface.go` - defines `Binding` with `Ifindex int`
- [ ] `internal/plugins/iface/vpp/query.go` - fills `InterfaceInfo.Index` from the VPP `sw_if_index`
- [ ] `internal/plugins/copp/translate.go` - `translatePolicy` builds the nft `ze_copp` inet table with an input base chain
- [ ] `internal/plugins/dhcpserver/register.go` - `serveMulti` reads with `ReadFromUDP`, discards the returned address, then tries each handler in order until one produces a reply

**Behavior to preserve:**
- One resolver for both dataplanes (`iface.Resolve`), per learned 1185 and 950. No second resolution path.
- The JSON shape external FIB plugin processes already decode, unless a child explicitly versions it.
- `LoadBackend` swapping the active backend at runtime.
- copp's existing BGP-port protection and its `ze_copp` table ownership prefix.

**Behavior to change:** per child. The umbrella changes nothing on its own.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Routes: protocol plugins (bgp, static, connected, isis, ospf, ldp) publish into `sysrib`.
- Control packets: raw and UDP sockets opened independently by each protocol's own transport package.
- Interface state: config, then `iface.LoadBackend`, then the active backend.

### Transformation Path
1. `sysrib` merges per-source routes and selects a best path.
2. `sysrib` emits `(system-rib, best-change)` carrying `BestChangeBatch`, per family.
3. `fibkernel`, `fibvpp` and `fibp4` consume the event. Each declares its own package-private backend interface; the three share no type.
4. Interface work goes the other way: callers reach `iface.Resolve` or `iface.GetBackend()` and the single active backend performs it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| sysrib ↔ FIB plugin (in-process) | Go event bus, typed payload | No |
| sysrib ↔ FIB plugin (external process) | JSON over the plugin process protocol, via `BestChangeBatch.MarshalJSON` | No |
| iface component ↔ iface backend | `iface.Backend`, one active implementation at a time | No |
| protocol plugin ↔ kernel | each protocol's own socket, no shared type | No |

### Integration Points
- `internal/component/sysrib/events` - the route payload every FIB backend decodes
- `internal/component/iface` - the registry every interface consumer reaches through
- `internal/component/firewall` - what copp compiles into
- each protocol's `transport` package - the ten independent receive paths

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | External FIB plugin processes exist that decode `BestChangeBatch` JSON, so its shape is a real contract and not only an internal convenience | `MarshalJSON` doc comment in `internal/component/sysrib/events/events.go` says external FIB plugin processes decode it | Children 1 and 3 become plain refactors with no compatibility work | Search the plugin SDK and `docs/architecture/api/process-protocol.md` for a documented consumer | unvalidated |
| A-2 | No consumer outside `fibkernel` relies on `RouteType` values being RTN_ numbers | `RouteType` comment names only the kernel backend | Child 1 breaks a consumer | Find every reader of the field before changing the numbering | unvalidated |
| A-3 | Every current reader of `Binding.Ifindex` can be migrated to an accessor without changing its resolution semantics | Learned 1185 says `iface.Resolve` already returns the correct per-dataplane index | Child 2 grows into the design reopen that D-2 rejected | Enumerate every reader and classify each as kernel-target or vpp-target | unvalidated |
| A-4 | A shared control-packet receive path is wanted at all | Not stated by the user; child 4 exists to answer it | Child 4 closes as "no change", which is a valid outcome | Child 4's own design gate | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Child 3 changes a payload external plugin processes decode, and an unversioned change breaks them silently | An external FIB plugin stops programming routes with no error | Treat the field as additive and optional, and decide the compatibility story before implementing |
| R-2 | Child 2 touches every reader of `Binding.Ifindex` and grows beyond its stated size | The migration list exceeds a handful of call sites | Land the accessor first with the bare field retained, then migrate readers one at a time |
| R-3 | Child 5 overlaps `spec-cp-survival-0-umbrella`, which is in-progress and awaiting closure verification | Both specs claim the same copp behavior | Read that umbrella's closure record before starting child 5, and record the split there |
| R-4 | Child 4 expands to cover subscriber work it was not scoped for | The design discusses session state rather than packet delivery | Child 4's deliverable is a decision plus a written design, not an implementation |
| R-5 | Children land piecemeal and the shared cause is lost, so a later author re-derives the same five findings | A new spec repeats a finding in the table above | The findings table is the record; cite this umbrella from each child |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Child 3 can break external FIB plugin processes and mis-program routes. Child 2 can mis-program an interface index into the wrong dataplane, which is the failure it exists to prevent. Children 1 and 5 are contained. Child 4 changes nothing by itself |
| How is it reverted? | Children 1, 2 and 5 are single-commit reverts. Child 3 is not, once an external plugin has seen the new field |
| Who else touches this path? | `spec-fib-depth` (in-progress) on the route seam, `spec-cp-survival-0-umbrella` on copp, `spec-finish-vpp-stub` on VPP test coverage |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Umbrella carries no code of its own | → | see each child's own Wiring Test table | N/A - umbrella |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | All five children reach a terminal state (closed, or cancelled with a recorded reason) | The umbrella closes. No finding in the findings table is left without a destination |
| AC-2 | A child is cancelled rather than implemented | Its row in the findings table names the reason and the decision's author, so the finding is not re-derived later |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | (umbrella carries no user-facing operation of its own) | see each child | N/A |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none - umbrella) | | Each child owns its own tests | |

### Functional Tests
<!-- The umbrella carries no code, but each child changes daemon behavior and
     must prove it from the entry point. The existing `.ci` files named here are
     the ones each child extends or must not regress. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `002-fib-route` (existing, must not regress) | `test/vpp/002-fib-route.ci` | Routes still program on VPP after child 1 renumbers `RouteType` and child 3 adds a field | |
| `007-fib-route-lookup` (existing, must not regress) | `test/vpp/007-fib-route-lookup.ci` | Route lookup still resolves on VPP | |
| `005-table-interface`, `006-interface-nexthop-no-backend` (existing, must not regress) | `test/static/005-table-interface.ci`, `test/static/006-interface-nexthop-no-backend.ci` | An interface-named next-hop still programs on both dataplanes, and still fails with an actionable error when no backend is loaded, after child 2 moves the guard into an accessor | |
| `isis-route-install`, `ospf-route-install` (existing, must not regress) | `test/isis/isis-route-install.ci`, `test/ospf/ospf-route-install.ci` | IGP routes still reach the FIB after the route payload changes | |
| new: route type over the plugin process protocol | `test/plugin/*.ci` (child 1) | An external FIB plugin process receives the documented route type value, not a Linux constant | |
| new: interface-scoped route | `test/static/*.ci` (child 3) | A route that names an egress interface reaches the FIB with that interface attached | |
| new: control-plane policing beyond TCP | `test/firewall/*.ci` (child 5) | A flood of non-TCP control traffic is policed, and BGP protection still holds | |

## Files to Modify
- (none directly) - the umbrella coordinates; each child names its own files

## Files to Create
- `plan/spec-dataplane-seams-1-route-type-numbering.md` - child 1
- `plan/spec-dataplane-seams-2-backend-typed-index.md` - child 2
- `plan/spec-dataplane-seams-3-route-egress-interface.md` - child 3
- `plan/spec-dataplane-seams-4-control-packet-rx.md` - child 4
- `plan/spec-dataplane-seams-5-copp-non-tcp.md` - child 5
- `plan/deferrals/dataplane-seams.md` - on the first deferral

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Umbrella adds no config. Children 4 and 5 answer this themselves |
| YANG validation constraints | N-A | As above |
| YANG custom validators | N-A | As above |
| CLI commands/flags | N-A | Umbrella adds no command |
| CLI grammar (keyword before value) | N-A | Umbrella adds no command |
| Editor autocomplete | N-A | Umbrella adds no config leaf |
| Functional test for new RPC/API | N-A | Umbrella adds no RPC. Child 3 changes an existing payload and answers this |
| Pipe completeness | N-A | Umbrella produces no CLI output |
| Env var registration | N-A | Umbrella adds no env var |
| Doctor check for runtime dependencies | N-A | Umbrella adds no runtime dependency. Child 4 will if it opens a socket |
| Prometheus counters/metrics | N-A | Umbrella adds no observable state. Child 5 should consider a policed-drop counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | Umbrella ships no feature |
| 2 | Config syntax changed? | N-A | Children answer this |
| 3 | CLI command added/changed? | N-A | No command |
| 4 | API/RPC added/changed? | N-A | Child 3 answers this |
| 5 | Plugin added/changed? | N-A | No plugin added |
| 6 | Has a user guide page? | N-A | Not user-facing |
| 7 | Wire format changed? | N-A | Child 3 answers this |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` - child 3 changes a payload external plugin processes decode |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC obligation is touched by the umbrella |
| 10 | Test infrastructure changed? | N-A | Children answer this |
| 11 | Affects daemon comparison? | N-A | No externally comparable behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - the seam distinction in Task belongs there once a child lands |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md` - child 3 adds a field to the route payload |
| 14 | Prometheus counters added/changed? | N-A | Children answer this |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on `sysrib/events/events.go`, `iface/backend.go`, `iface/iface.go`, `copp/translate.go` before each child lands |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check `docs/` route-payload and interface-backend examples before children 2 and 3 land |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- not applicable to the umbrella. Each child performs its own wiring phase.
2. **Phase: order the children** -- take 1, then 2, then 5. Each is contained and none blocks another. Do 3 only after reading `spec-fib-depth`'s current state, because it owns that surface. Do 4 whenever the design question becomes worth answering.
3. **Phase: close** -- when all five reach a terminal state, close the umbrella per `ai/rules/planning.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every finding F-1..F-5 has a child, and every child has a terminal state |
| Correctness | No child contradicts learned 1185 or learned 950 |
| Data flow | No child introduces a second interface-resolution path |
| Rule: `ai/rules/planning.md` | Each child closed with its own learned summary; the umbrella closes last |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Five child spec files exist | `ls plan/spec-dataplane-seams-*.md` |
| Each child names its parent | `grep -l dataplane-seams-0-umbrella plan/spec-dataplane-seams-*.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed guards | Child 2's accessor must error rather than return a zero index. Index 0 is VPP `local0`, so a zero return is a valid-looking wrong answer (`ai/rules/evidence.md`) |
| Resource exhaustion | Child 5 changes what is policed on the control plane. A widened match must not remove the existing BGP protection |

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

- The one property that kept the route seam neutral was that it had to be serialized for a consumer in another process. That is a cheap, mechanical discipline and it is worth applying deliberately to any future seam intended to outlive its first backend.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Umbrella plus five independently closable children | One flat spec; two specs split by size; mechanical items only | The control-packet receive path is an open design question. Inside a flat spec it would block four small changes that are ready to make |
| Child 2 keeps the learned-1185 design and changes only its enforcement | Reopening "one resolver, two dataplanes"; accepting the risk | The decision is sound and the runtime swap that motivated it still happens. What is weak is that the guard must be remembered by each caller |

## Known Limitations
- The umbrella does not decide whether a shared control-packet receive path should exist. That is child 4's deliverable, and "no change" is a legitimate outcome.
- No child covers the width of `iface.Backend` itself (about 40 methods, holding both Linux-only and VPP-only operations). That is a real observation from the same review, but it has no concrete defect attached and no test would prove it fixed. Recorded here rather than specced.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
