# Spec: traceroute-source-af

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/traceroute/cmd/traceroute.go` - traceroute engine + arg parsing
4. `internal/component/traceroute/cmd/resolve.go` - target/source resolution
5. `internal/core/probe/icmp.go` - `ResolveTarget`

## Task

When an operator runs a traceroute toward a hostname and specifies a source-address
of a particular family (e.g. an IPv6 source), Ze resolves the destination hostname
without regard to that family and picks the first answer. If the first answer is an
A (IPv4) record, Ze opens an IPv4 socket and tries to bind the IPv6 source, which
fails. The source-address family must constrain destination resolution: a v6 source
forces v6 resolution, a v4 source forces v4.

Make the source-address's family drive the address family used to resolve the
destination hostname (and, consequently, the socket family).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - probe/traceroute component placement.
  → Constraint: traceroute uses Ze's own ICMP engine (raw socket), not a shell-out; AF selection is internal.

**Key insights:**
- The destination AF is currently derived from the resolved destination address, and the source-address is parsed afterwards and only used as the bind address.
- The fix is ordering + intent: read the source-address family first, then resolve the destination in that family.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traceroute/cmd/traceroute.go` - `doTracerouteCtx` derives the socket family from the destination: `isV6 := dest.Is6()` (traceroute.go:192); the `source` is used only for `bindAddr` (traceroute.go:202-205, 208). `parseTracerouteArgs` (traceroute.go:60-115) resolves the destination first; source has no bearing on it.
- [ ] `internal/component/traceroute/cmd/resolve.go` - the destination is resolved AF-agnostically before the source arg is read; `validateSourceIP` (resolve.go:157-164) checks only that the source is a valid IP, not its family vs the destination.
- [ ] `internal/core/probe/icmp.go` - `ResolveTarget` calls `net.DefaultResolver.LookupNetIP(ctx, "ip", s)` and returns `ips[0]` (icmp.go:54-66); network `"ip"` means either family, first answer wins.

**Behavior to preserve:**
- With no source-address, resolution stays AF-agnostic (current behaviour, first answer wins).
- Traceroute continues to use the internal ICMP engine (no shell-out).
- A literal IP destination is unaffected (no resolution needed).

**Behavior to change:**
- When a source-address is given, its family constrains destination hostname resolution.

## Data Flow (MANDATORY)

### Entry Point
- CLI: `resolve traceroute` / `show traceroute` with a `source-address <ip>` option and a hostname target, parsed in `parseTracerouteArgs` (traceroute.go:60-115).

### Transformation Path
1. Parse args; extract the source-address (if any) before resolving the destination.
2. Derive an address-family hint from the source-address family (v4 → `"ip4"`, v6 → `"ip6"`, none → `"ip"`).
3. Resolve the destination hostname with that family hint (`ResolveTarget` gains a family parameter, or a family-aware variant is used).
4. Open the ICMP socket for the resolved family and bind the source-address.
5. On family mismatch that cannot be satisfied (e.g. v6 source, hostname has no AAAA), return a clear error rather than a bind failure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ traceroute engine | source-address family passed into resolution | [ ] |
| Engine ↔ resolver | family hint → `LookupNetIP(ctx, "ip4"|"ip6"|"ip", …)` | [ ] |
| Engine ↔ socket | resolved family selects v4/v6 socket | [ ] |

### Integration Points
- `ResolveTarget` (`probe/icmp.go:54-66`) - accept a family hint.
- `parseTracerouteArgs` / `doTracerouteCtx` (`traceroute.go`) - order source parse before destination resolve; pass the hint.
- `validateSourceIP` (`resolve.go:157-164`) - optionally assert source family matches the resolved destination.

### Architectural Verification
- [ ] No bypassed layers (resolution still through the probe resolver)
- [ ] No unintended coupling (family hint is a parameter, not global state)
- [ ] No duplicated functionality (reuse `ResolveTarget`, extend it)
- [ ] Registration over hardcoding — traceroute command remains registry-registered; no central AF switch.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `LookupNetIP` supports `"ip4"`/`"ip6"` networks for family-constrained resolution | Go stdlib net semantics | need manual filtering of results | unit test with a dual-stack name | unvalidated |
| A-2 | Both `resolve traceroute` and `show traceroute` share the same parse path | traceroute.go:60-115 | must fix both call sites | grep both entry points during audit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | v6 source + hostname with only A records → no resolvable target | traceroute errors | return a clear "no AAAA for source family" message, not a bind failure |
| R-2 | Regression for the no-source case | v4-only hosts change behaviour | family hint defaults to `"ip"` when no source given (unchanged path) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show traceroute <dualstack-host> source-address <v6>` | → | destination resolved as v6, v6 socket | `test/plugin/traceroute-source-af.ci` |
| `... source-address <v4>` | → | destination resolved as v4 | `test/plugin/traceroute-source-af.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | dual-stack hostname + IPv6 source-address | destination resolved to its AAAA; v6 socket bound to the v6 source |
| AC-2 | dual-stack hostname + IPv4 source-address | destination resolved to its A record; v4 socket |
| AC-3 | hostname with only A records + IPv6 source | clear error (no target in source family), not a bind failure |
| AC-4 | no source-address | AF-agnostic resolution unchanged (first answer wins) |
| AC-5 | literal IP destination | unaffected |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | traceroutes a dual-stack host from a v6 source | source parsed first → v6 resolve → v6 socket | `test/plugin/traceroute-source-af.ci` |
| 2 | uses a v6 source toward a v4-only name | clear error message | `test/plugin/traceroute-source-af.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveTargetFamilyHint` | `internal/core/probe/icmp_test.go` | family hint constrains resolution | |
| `TestTracerouteSourceForcesV6` | `internal/component/traceroute/cmd/traceroute_test.go` | v6 source → v6 destination/socket | |
| `TestTracerouteSourceFamilyMismatch` | `internal/component/traceroute/cmd/traceroute_test.go` | v6 source + A-only name → error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `traceroute-source-af` | `test/plugin/traceroute-source-af.ci` | source family drives destination resolution | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - operational tool, no peer protocol | - | - | validated by functional tests | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/traceroute/cmd/traceroute.go` - parse source before resolving destination; pass family hint
- `internal/component/traceroute/cmd/resolve.go` - family-aware resolution; optional source/dest family assertion
- `internal/core/probe/icmp.go` - `ResolveTarget` accepts a family hint

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI grammar | [ ] no (existing options) | `ai/rules/cli-grammar.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/traceroute-source-af.ci` |
| Pipe completeness | [ ] yes | traceroute output already routes through pipes; keep it |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | [ ] maybe | `docs/guide/command-reference.md` (behaviour clarification) |
| 12 | Internal architecture changed? | [ ] maybe | probe/traceroute doc if resolution semantics documented |

## Files to Create
- `test/plugin/traceroute-source-af.ci` - functional test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add a family-hint parameter to `ResolveTarget` (default `"ip"`, no behaviour change); failing `test/plugin/traceroute-source-af.ci`.
2. **Phase: Source-first parse + hint** — parse source-address before resolving destination; derive and pass the family hint in both `resolve traceroute` and `show traceroute`.
   - Tests: `TestResolveTargetFamilyHint`, `TestTracerouteSourceForcesV6`
3. **Phase: Mismatch error** — clear error when the source family has no resolvable target.
   - Tests: `TestTracerouteSourceFamilyMismatch`
4. **Functional test**
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | no-source path byte-for-byte unchanged; both entry points fixed |
| Data flow | family hint threaded as a parameter |
| Registration over hardcoding | no central AF switch introduced |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| family-aware resolution | `go test ./internal/core/probe -run FamilyHint` |
| both entry points | grep shows source parsed before resolve in resolve+show paths |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | source-address parsed as a valid IP before use |
| Error leakage | resolution errors do not leak internal resolver detail |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
