# Spec: rib-arch-3 -- RFC 5549 Extended Next-Hop for Injected Routes

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `internal/component/bgp/plugins/rib/rib_commands.go` - `injectRoute`
5. `rfc/short/rfc5549.md`, `rfc/short/rfc8950.md`

## Task

`request bgp rib inject` (`injectRoute`, `internal/component/bgp/plugins/rib/rib_commands.go:225`)
inserts a route into adj-rib-in as if received from a peer, with no live BGP session.
It accepts a `nexthop` attribute and already validates IPv6 next-hops per rfc8950
(`rib_commands.go:4` RFC comment).

GAP: injected routes cannot carry an **RFC 5549 extended next-hop** -- an IPv4 NLRI
reachable via an IPv6 next-hop. The receive/parse side already supports RFC 5549/8950
(`internal/core/bgp/attribute/mpnlri.go`, tests `TestParseMPReachNLRI_ExtendedNextHop*`
in `mpnlri_test.go`); this item adds the inject/encode counterpart so injected routes can
exercise the extended-next-hop forwarding path end-to-end.

STALE ANCHOR (verified 2026-07-08): the 2026-07-06 triage cited `PackContext.ExtendedNextHop`
as the missing field; `PackContext` no longer exists anywhere in the tree. Re-verify the
current injection pack/encode path (starting at `injectRoute` and its `attribute.NewBuilder()`
usage, `rib_commands.go:255`) at design time and locate where an extended next-hop would be
set.

### Re-verification (2026-07-14): de-risked

- Assumption A-1 is now VALIDATED. A reusable extended-next-hop encoder already exists on
  the announce/forward path: `attribute.NewMPReachNLRI` (`internal/core/bgp/attribute/mpnlri.go:116`)
  + `MPReachNLRI.WriteTo` (`mpnlri.go:154`), and `ValidNextHopLens(AFIIPv4, SAFIUnicast)`
  returns `{4, 16}` (plain IPv4 or RFC 5549 IPv6) as the single source of truth for both
  encode and decode. The remaining work is confined to the inject/attribute-assembly
  layer (`attribute.NewBuilder()` has an IPv4-only `SetNextHop`; no MP_REACH setter).
- Inject already has a capability-aware `validateIPv6NextHop` (`rib_commands.go`, the
  helper called from the `nexthop` branch) that today validate-then-DROPS an IPv6
  next-hop for an IPv4 NLRI (never encodes it into MP_REACH). This post-dates the 2026-07
  triage framing; the gap (no extended next-hop ever emitted for an injected route)
  remains real.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `injectRoute` entry and attribute building
  → Constraint: injection currently restricts to simple prefix families (`rib_commands.go:244`); RFC 5549 must fit that or extend it explicitly.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5549.md` - advertising IPv4 NLRI with an IPv6 next-hop
  → Constraint: next-hop length encodes the address family; the injected next-hop must follow §3/§4.
- [ ] `rfc/short/rfc8950.md` - the updated (obsoleting) extended-next-hop encoding
  → Constraint: injection next-hop validation already cites rfc8950; keep consistent.

**Key insights:**
- Parse-side RFC 5549 exists; this is the symmetric inject/encode gap. The triage's `PackContext` anchor is gone -- re-locate the pack path.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `injectRoute` (:225): parses peer/family/prefix + optional `origin`/`nexthop`/`aspath`/`localpref`/`med`; restricts to simple prefix families (:244); builds attributes via `attribute.NewBuilder()` (:255)
- [ ] `internal/core/bgp/attribute/mpnlri.go` - parse-side RFC 5549/8950 extended next-hop (receive direction), covered by `mpnlri_test.go:311` `TestParseMPReachNLRI_ExtendedNextHop`

**Behavior to preserve:**
- Existing inject syntax and same-family next-hop validation; the simple-prefix-family restriction unless design deliberately widens it.

**Behavior to change:**
- Injected routes may specify an IPv6 next-hop for an IPv4 NLRI (and the encode path emits the RFC 5549/8950 extended next-hop).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `request bgp rib inject <peer> <family> <prefix> ... nexthop <ip>` CLI command

### Transformation Path
1. `injectRoute` parses args and validates family/next-hop (`rib_commands.go:225`)
2. Attributes built via `attribute.NewBuilder()` (`rib_commands.go:255`)
3. Route enters adj-rib-in as if received from the labelled peer
4. Proposed: when the next-hop family differs from the NLRI family, encode an RFC 5549/8950 extended next-hop instead of rejecting

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI → RIB | `injectRoute` builds attributes and inserts into adj-rib-in | [ ] |
| structured → wire | extended-next-hop encoding on the forward path | [ ] |

### Integration Points
- `injectRoute` (`rib_commands.go:225`) - the inject entry point
- `attribute.NewBuilder()` (`rib_commands.go:255`) - attribute assembly (next-hop set here)
- parse-side `mpnlri.go` - the symmetric decode already implemented

### Architectural Verification
- [ ] No bypassed layers (inject uses the normal attribute-build + adj-rib-in path)
- [ ] No unintended coupling (extended-next-hop logic reuses the existing encoder, not a new one)
- [ ] No duplicated functionality (reuse the RFC 5549/8950 encoding used on the forward path)
- [ ] Registration over hardcoding - families resolved via the family registry, not a new switch (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An extended-next-hop encoder exists on the forward path to reuse | parse side exists; forward side likely symmetric | Must build the encoder too; larger scope | grep for the MP_REACH next-hop encoder at design | **VALIDATED (2026-07-14)**: `NewMPReachNLRI` (`mpnlri.go:116`) + `WriteTo` (`:154`) already emit it; reuse, don't rebuild |
| A-2 | The simple-prefix-family inject restriction can host an IPv4-over-IPv6 case | `rib_commands.go:244` | May need to widen the restriction | read the restriction check at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Inject accepts a next-hop the forward encoder cannot emit (exact-or-reject violation) | mismatch between inject validation and encode | reject at inject time if encode unsupported (`ai/rules/exact-or-reject.md`) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `request bgp rib inject ... ipv4/unicast <prefix> nexthop <ipv6>` | → | RFC 5549 extended next-hop encoded on the injected route | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Inject IPv4 unicast prefix with an IPv6 next-hop | route accepted; MP_REACH carries the RFC 5549/8950 extended next-hop |
| AC-2 | Inject with a next-hop the encoder cannot emit | rejected with a clear error (exact-or-reject) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | `internal/component/bgp/plugins/rib/rib_commands_test.go` | inject accepts IPv6 next-hop for IPv4 NLRI; encodes extended next-hop | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rib-inject-rfc5549` (new) | `test/plugin/rib-inject-rfc5549.ci` | operator injects an IPv4 route with an IPv6 next-hop; it forwards with the extended next-hop | |

### Interop Tests (MANDATORY for protocol features)
- Design decides whether an interop scenario (FRR/BIRD accepting the injected RFC 5549 route) is warranted; the parse side is already interop-tested.

## Files to Modify

- `internal/component/bgp/plugins/rib/rib_commands.go` - `injectRoute` next-hop family handling
- the MP_REACH extended-next-hop encoder on the forward path (located at design)

## Implementation Steps

1. **Phase: design** - re-locate the pack/encode path (triage `PackContext` anchor is stale); confirm the encoder (A-1).
2. **Phase: wiring** - failing test injecting IPv4-over-IPv6.
3. **Phase: implement (TDD)** - accept the cross-family next-hop at inject; encode RFC 5549/8950.
4. **Functional + RFC comments** - `.ci` fixture; `// RFC 5549`/`// RFC 8950` on enforcing code.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Injected IPv4-over-IPv6 route encodes the extended next-hop
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## RFC Documentation

Add `// RFC 5549 Section X.Y` / `// RFC 8950 Section X.Y` comments above the injected
extended-next-hop encoding and validation code.

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
