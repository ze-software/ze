# Spec: Control-Plane Survivability Under DDoS (Umbrella)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-22 |

Awaiting closure verification (recorded 2026-07-22 during plan review): no
child spec remains in `plan/` and every child has a learned summary (1004
gtsm, 1005 copp-port179, 1007 egress-cs6-sched, 1008 on-demand-origination
DESIGN, 1011 + 1015 detect); the gating deployment doc exists
(`docs/guide/ddos-mitigation.md`) and the `docs/features.md` rows are present.
The D2 caveat is now RESOLVED (verified 2026-07-22): **D2 as specified did NOT
ship.** The named "firewall->FlowSpec reverse bridge (new plugin)" does not
exist (`internal/plugins/firewall/` has no flowspec origination;
`internal/plugins/flowspec-firewall/` is the INBOUND direction), and learned
1008 itself labels it "future D2 (flowspec-egress bridge)". The functional
goal (attack -> outbound FlowSpec/RTBH announcement) shipped via a different
trigger: the ddos responder (`internal/plugins/ddos/flowspec/responder.go:32,
60,151-188`, children 5, learned 1011/1015) originates from ddos-detect
characterization events, not firewall config rules. At closure, record D2 as
a homed deferral row (no spec owns "originate FlowSpec from firewall config")
or as an explicit user-approved supersession by the ddos responder.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. The four child specs: `spec-cp-survival-1-bgp-gtsm.md`, `spec-cp-survival-2-copp-port179.md`, `spec-cp-survival-3-egress-cs6-sched.md`, `spec-cp-survival-4-flowspec-origination.md`
4. `internal/component/bgp/reactor/session_connection.go` (existing CS6 marking)

## Task

When a network is under volumetric DDoS and wants to signal an upstream provider (or scrubbing
service) to filter the attack via BGP FlowSpec or RTBH, three independent failure modes can stop
the mitigation message from getting out:

1. **Link saturation** — the egress link to the upstream is full; BGP keepalives and the FlowSpec
   UPDATE compete with attack traffic for bandwidth.
2. **Control-plane / CPU saturation** — traffic aimed at the router's own address (or collateral
   punt load) starves the process that runs BGP.
3. **No way to originate the mitigation** — the session is healthy but nothing turns "under attack"
   into an outbound FlowSpec/RTBH announcement quickly.

This umbrella frames the initiative and coordinates four child specs that each close one gap. It
owns the shared research (current state + the gap analysis) and the cross-cutting design decisions;
the child specs own implementation. The umbrella itself ships **no production code** beyond an
operator-facing deployment-guidance doc.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component/plugin registration model these features plug into
  → Constraint: features register via `init()` in `register.go`; core discovers via registries, never imports plugins directly. Each child must self-contain (`ai/rules/plugin-self-containment.md`).
- [ ] `ai/rules/buffer-first.md` - wire encoding discipline
  → Constraint: the FlowSpec/RTBH UPDATE must build via `WriteTo(buf, off) int` with pooled buffers so origination works under memory pressure (the whole point of gap D under DDoS).
- [ ] `ai/rules/config-surface.md` - YANG-vs-env-var decision for new config
  → Decision: GTSM (gap A) and CoPP (gap B) toggles are per-peer/operational policy → YANG leaves, not env vars.

### RFC Summaries
- [ ] `rfc/short/rfc5082.md` - GTSM (gap A). Verify summary exists; create via `/ze-rfc` if missing.
  → Constraint: GTSM sets outgoing TTL=255 and drops incoming packets with TTL < (255 − hops + 1).
- [ ] `rfc/short/rfc7999.md` - BLACKHOLE community 0xFFFF029A (gap D / RTBH). Verify summary exists.
  → Constraint: BLACKHOLE community signals null-routing; rides on ordinary unicast NLRI, no new SAFI.
- [ ] `rfc/short/rfc8955.md` - FlowSpec v4 (gap D). Already implemented (SAFI 133); confirm summary exists.
  → Constraint: origination reuses the existing flowspec NLRI codec; no new BGP family work.

**Key insights:**
- The single highest-leverage protection is operational, not code: run the mitigation-signaling
  session **out-of-band** (separate path / management connectivity to the upstream's controller or a
  scrubber) so it never crosses the attacked link. Captured in the deployment doc (AC-5). The code
  gaps below matter for the case where the signaling session *must* cross the attacked path.
- Ze already does the most important in-code thing: it marks BGP egress DSCP CS6
  (`session_connection.go:251-253`). The gaps are about (A) keeping spoofed traffic off the control
  path, (B) keeping the CPU alive, (C) making Ze's *own* egress honor that CS6 mark, and (D) being
  able to originate the signal on demand.

## Current Behavior (MANDATORY)

**Source files read:** (shared grounding; per-gap detail lives in the child specs)
- [ ] `internal/component/bgp/reactor/session_connection.go` - `connectionEstablished()` sets TCP_NODELAY, IP_TOS/IPV6_TCLASS=0xC0 (CS6), SO_RCVBUF/SNDBUF on every BGP socket (dial + accept).
  → Constraint: this is the existing socket-tuning seam; gap A adds TTL setsockopt in the same Control callback.
- [ ] `internal/core/network/network.go` - `RealDialer.DialContext` Control callback applies setsockopt (MD5) before connect; `RealListenerFactory.Listen` does the same per-peer for inbound.
  → Constraint: outbound socket options must be applied here (pre-connect), not only post-accept.
- [ ] `internal/core/bgp/attribute/community.go:99` - `CommunityBlackhole = 0xFFFF029A` defined and named "blackhole".
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:28` - `AnnounceNLRIBatch(sel, batch)` already exists; runtime announce works today via the text-protocol RPC.

**Existing protections (do NOT regress):**
- DSCP CS6 marking on BGP sockets (RFC 4271 §5.1).
- TCP MD5 (RFC 2385) on dial + listen.
- Zero-alloc / buffer-first UPDATE encoding.
- Static + text-protocol FlowSpec origination; BLACKHOLE community.

**Behavior to change:** None directly in the umbrella. Each child adds new, opt-in behavior.

## Data Flow (MANDATORY)

### Entry Point
- Operator/automation intent ("we are under attack, signal upstream") OR pre-staged policy.

### Transformation Path (target end-state across the four children)
1. **Survive (A, B):** spoofed/flood traffic to the control path is dropped cheaply — GTSM at the
   socket (A) and a CoPP rate-limit chain on the BGP listen port (B).
2. **Prioritise (C):** Ze's own egress qdisc places CS6-marked control packets in a strict-priority
   band so keepalives/UPDATEs win the link.
3. **Originate (D):** operator verb (or firewall-derived bridge) calls `AnnounceNLRIBatch` to emit
   the FlowSpec/RTBH UPDATE upstream.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ BGP reactor (A) | YANG `connection > ttl` → PeerSettings → setsockopt | [ ] |
| Config tree ↔ firewall (B) | YANG firewall input chain → nft backend | [ ] |
| Config tree ↔ traffic (C) | YANG traffic-control → netlink tc u32 selector | [ ] |
| Operator/firewall ↔ reactor (D) | CLI/RPC or bridge → `AnnounceNLRIBatch` → wire | [ ] |

### Integration Points
- `internal/component/bgp/reactor/session_connection.go` `connectionEstablished()` - gap A hooks here
- `internal/component/firewall` input-hook chains - gap B hooks here
- `internal/component/traffic` netlink backend - gap C hooks here
- `internal/component/bgp/reactor/reactor_api_batch.go` `AnnounceNLRIBatch` - gap D hooks here

### Architectural Verification
- [ ] No bypassed layers (each child uses its subsystem's existing config→backend path)
- [ ] No unintended coupling (children are independent; no child imports another)
- [ ] No duplicated functionality (B reuses firewall Limit; D reuses AnnounceNLRIBatch)
- [ ] Zero-copy preserved (D origination uses buffer-first encoding)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The four gaps are genuinely independent and can ship/commit separately | subsystem analysis (BGP socket / firewall / traffic / origination) | umbrella sequencing wrong; cross-spec coupling | each child compiles + tests green alone | unvalidated |
| A-2 | Out-of-band signaling is a deployment pattern Ze already supports (multiple peers/sessions), needing only docs not code | Ze supports many peers per reactor | gap-0 grows a code AC | confirm a second peer over a distinct path configures today | unvalidated |
| A-3 | CS6 marking already present means upstream-honored prioritisation is the common case; gap C only matters when Ze is the bottleneck | `session_connection.go:251-253` | C is higher priority than ranked | operator confirmation of topology | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | CoPP (B) misconfigured locks the operator out of their own router (drops BGP/SSH) | session/SSH drops on apply | B ships a conservative default (accept established, limit new), and a commit-confirm/rollback note in the deployment doc |
| R-2 | GTSM (A) breaks a legitimately multihop eBGP session whose TTL is < 255 | session won't establish after enabling | A is opt-in per peer; `max`/`min` leaves let operator widen the window |
| R-3 | On-demand origination (D) leaves a stale FlowSpec/RTBH rule announced after the attack ends | upstream keeps filtering | D includes explicit withdraw + a documented TTL/timeout pattern |
| R-4 | Doing all four at once dilutes review focus | review churn | implement in the ranked order; each child has its own `/ze-review` gate |

## Wiring Test (MANDATORY)

The umbrella has no executable feature of its own; its "wiring" is that each child's wiring test
passes and the deployment doc exists.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Each child spec closed | → | child wiring tests | child `/ze-review` gates + functional tests (see children) |
| `docs/guide/ddos-mitigation.md` exists & is linked | → | deployment guidance | `make ze-doc-test` (doc lint) + grep for the source anchor |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Child spec 1 (GTSM) complete | `spec-cp-survival-1-bgp-gtsm.md` closed: GTSM configurable per peer, enforced at socket, tested |
| AC-2 | Child spec 2 (CoPP) complete | `spec-cp-survival-2-copp-port179.md` closed: a control-plane-protection construct for port 179 exists and is tested |
| AC-3 | Child spec 3 (egress CS6) complete | `spec-cp-survival-3-egress-cs6-sched.md` closed: DSCP filter classification works in tc and a CS6 priority class is configurable |
| AC-4 | Child spec 4 (origination) complete | `spec-cp-survival-4-flowspec-origination.md` closed: operator can originate FlowSpec/RTBH on demand |
| AC-5 | Operator reads docs | `docs/guide/ddos-mitigation.md` documents the out-of-band signaling pattern, the layered defenses, and the CoPP lock-out caveat (R-1), with source anchors |
| AC-6 | Child spec 5 (auto-detection) complete | `spec-cp-survival-5-detect-0-umbrella.md` closed: automatic detector + responders drive Gap D / the firewall; decides *when* to mitigate and when the attack has ended |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables GTSM + CoPP, marks BGP CS6 priority, then announces a FlowSpec rule mid-attack | A (socket) + B (nft) + C (tc) + D (announce → AnnounceNLRIBatch → wire) | each child's functional test; this story is the integration narrative in the deployment doc |
| 2 | runs the mitigation session out-of-band | second peer over management path | deployment-doc worked example (AC-5) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none — umbrella owns no executable code) | n/a | unit coverage lives in the four child specs | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (per child) | (per child) | see child specs | |
| `ddos-doc` | `make ze-doc-test` | deployment guide builds and source anchors resolve | |

### Interop Tests
Owned by the children that touch the wire (gap D). N/A for the umbrella itself.

## Files to Modify
- `docs/features.md` - add "DDoS control-plane survivability" feature row (source-anchored)
- `docs/guide/production-diagnostics.md` - cross-link the new mitigation guide

## Files to Create
- `docs/guide/ddos-mitigation.md` - operator guidance: out-of-band signaling, layered defenses, lock-out caveat

## Implementation Steps

The umbrella is closed by closing its children and writing the deployment doc. Recommended order
(smallest/independent first, largest last):

1. **spec-cp-survival-1-bgp-gtsm** — smallest; YANG already modeled, mirrors BFD. Highest value/effort ratio.
2. **spec-cp-survival-2-copp-port179** — independent; reuses firewall Limit + input hook.
3. **spec-cp-survival-4-flowspec-origination (phase D1 only)** — ergonomic announce verb over existing `AnnounceNLRIBatch`.
4. **spec-cp-survival-3-egress-cs6-sched** — fixes the u32-DSCP bug + adds a CS6 priority class.
5. **spec-cp-survival-4-flowspec-origination (phase D2)** — firewall→FlowSpec reverse bridge (new plugin).
6. **Deployment doc** (AC-5) — write after at least A+B+D1 land so examples are real.

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the relevant child |
| 2. Audit | Per-child Files/Tests |
| 14. Present summary | Roll up child summaries into one umbrella learned summary (`plan/learned/NNN-cp-survival-0-umbrella.md`) per the `820-flow-export-0-umbrella` precedent |

## Known Limitations
- The umbrella's gaps A-D do not themselves attempt automatic attack *detection*. Gap D provides the
  lever (originate on demand / from a firewall rule); the automatic decision layer — deciding *when* to
  pull it and when the attack has ended — is now child spec 5, `spec-cp-survival-5-detect-0-umbrella.md`.
- Egress prioritisation (C) only helps when Ze is the congested node; the upstream honoring CS6 is
  the common path and needs no Ze code.

## Design Insights
- The cheapest, most robust mitigation is one requiring zero computation under load: pre-staged
  FlowSpec/RTBH plus an out-of-band path. The code gaps harden the in-band fallback.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit
(Filled at closure — roll-up of child audits.)

## Review Gate
### Final status
- [ ] All four child specs closed
- [ ] Deployment doc passes `make ze-doc-test`

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated (each child closed + deployment doc)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] All four child specs closed
- [ ] Implementation Summary filled (roll-up of child summaries)
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-0-umbrella.md`
