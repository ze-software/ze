# Spec: dns-resolver-recursion

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Waits on Thomas's D-2 choice (the D-2 table under Task, carried from `spec-lan-dns-resolver`, closed 2026-10-28): whether the LAN resolver also recurses from the root. Recommended default: ship it, because recursion removes the dependency on an operator upstream. No RFC the fttx-edge profile lists requires either mode. |
| RFC gaps | - |
| Depends | - (spec-lan-dns-resolver closed 2026-10-28) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Add full recursion from the root to the forwarding LAN resolver of
`spec-lan-dns-resolver` (closed 2026-10-28): a `recursive` value of
the `dns-resolver mode` leaf, built-in IANA root hints replaceable by config,
the delegation walk, and glue hardening.

This spec was split out of `spec-lan-dns-resolver` on 2026-10-27 so the
forwarding resolver could be implemented while D-2 stays open. It holds what
the previous draft designed for recursion, moved verbatim, with the ids
AC-34 and AC-35 kept. It is a skeleton: it runs only once Thomas answers D-2
(a), and its design is re-verified at the producers then.

| Decision | Options | Recommended default |
|----------|---------|---------------------|
| D-2 | (a) ship recursion (this spec runs); (b) forward only (this spec is withdrawn) | (a). Evidence: neither RFC 1035, RFC 2181 nor the profile title ("local DNS for the LAN") requires either mode; Unbound recurses by default and forwards when a `forward-zone` is set; no rule in `ai/META-DECISIONS.md` decides it |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/dns/lan-resolver.md` (the design of `spec-lan-dns-resolver`, closed 2026-10-28) - the forwarding resolver, its cache, ranking, bailiwick check and per-resolution budget, which recursion reuses
  → Constraint: the per-resolution budget (32 upstream queries, 11 CNAME restarts, deadline) bounds the delegation walk too; no goroutine per exchange
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the recorded red

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2181.md` - RFC2181-5.4.1-3 (data ranking) applies most strongly to referrals and glue; the forwarding spec closes the row, and this spec adds the recursive-mode case to its tagged test
- [ ] `rfc/short/rfc1035.md` - resolver algorithm references; RFC 1034 Section 5.3.3 (resolver algorithm) has no summary, so its tests carry no RFC tag

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dnsresolver/yang/ze-dns-resolver-conf.yang` (from `spec-lan-dns-resolver`, closed 2026-10-28) - forward mode only; the `mode` leaf has the single value `forward`
- [ ] `vendor/github.com/miekg/dns/client.go` - the client; miekg has no iterative resolver

Carried from the previous draft and to be re-verified when this spec is designed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `service { dns-resolver { mode recursive; recursive { root-hint <name> { ... } } } }`.
- Wire: a recursive query whose answer is not cached.

### Transformation Path
1. Cache miss in recursive mode: walk delegations from the root hints, asking each zone's servers in turn.
2. Referrals: glue is used only to reach the delegated servers; out-of-zone glue is not trusted, and the server's address is resolved.
3. Ranking and bailiwick as in forward mode; the per-resolution budget bounds the walk.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to authorities | UDP and TCP DNS from an ephemeral, per-exchange source port with a random ID | No |

### Integration Points
- `internal/plugins/dnsresolver` - `iterate.go`, `roothints.go`; the `mode` enum gains `recursive`

## Config and CLI surface (from the previous draft)

| Path | Type | Default | Constraint | Purpose |
|------|------|---------|------------|---------|
| `dns-resolver mode` | enum gains `recursive` | stays `forward`, so existing configs keep their meaning | - | resolution mode |
| `dns-resolver recursive root-hint <name>` | list, leaves `ipv4`, `ipv6` | absent: built-in IANA root hints | 1..13 entries | override the root for a lab |

`show dns resolver` shows the root-hint source in recursive mode.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-R1 | The forwarding spec's budget and coalescing bound the delegation walk with no new limit | `spec-lan-dns-resolver` Key Design Decisions (Unbound `max-sent-count` 32) | NS loops amplify work | `TestDelegationWalk` with an NS loop | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-R1 | Work amplification from deep chains and NS loops | `upstream_queries_total` per client query | per-resolution budget, coalescing, outstanding bound |
| R-R2 | Cache poisoning through glue | answers that disagree with Unbound in recursive mode | glue hardening (AC-35), bailiwick check, RFC 2181 ranking |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| recursive mode with lab root hints | → | iterative resolution | interop `dns-resolver-recursive-knot` |

## Acceptance Criteria

Moved verbatim from `spec-lan-dns-resolver`, ids kept.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-34 | `mode recursive` with root hints pointing at a lab root | Resolution follows delegations from the root; glue is used only to reach the delegated servers |
| AC-35 | Referral whose glue names an address outside the delegated zone's servers' authority | The out-of-zone glue is not trusted (Unbound `harden-glue` default yes: "Will trust glue only if it is within the servers authority"); the server's address is resolved |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator runs full recursion with no upstream | root hints → iterative → reply | interop `dns-resolver-recursive-knot` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDelegationWalk`, `TestGlueHardening` | `internal/plugins/dnsresolver/iterate_test.go` | AC-34, AC-35 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `recursive root-hint` count | 1-13 | 13 | N/A (absent: built-in) | 14 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dns-resolver-recursive-knot` | `test/interop/scenarios/dns-resolver-recursive-knot/` | Knot DNS hierarchy (lab root, `test.`, delegated children, out-of-zone glue) | full recursion from lab root hints; answers cross-checked against Unbound in recursive mode with the same hints | |

## Files to Modify
- `internal/plugins/dnsresolver/` - `config.go` (`recursive` mode, root hints), `show.go` (root-hint source), `yang/ze-dns-resolver-conf.yang`
- `internal/le/interoplab/bgp/check_dns_resolver.go` - the recursive scenario's checker
- `docs/guide/dns-resolver.md`, `docs/architecture/dns/lan-resolver.md` - recursive mode

## Files to Create
- `internal/plugins/dnsresolver/iterate.go`, `roothints.go`, `iterate_test.go`
- `test/interop/scenarios/dns-resolver-recursive-knot/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `mode` enum value, `recursive root-hint` list |
| YANG validation constraints | Yes | `max-elements` 13 on root hints |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |

## Implementation Steps

1. **Phase: Recursion** -- root hints, delegation walk, glue hardening
   - Tests: AC-34, AC-35, `TestDelegationWalk`, `TestGlueHardening`
2. **Phase: Interop** -- `dns-resolver-recursive-knot` with a recorded red

## Design Insights

| Source | Sentence | Design consequence |
|--------|----------|-------------------|
| unbound.conf `harden-glue` | "Will trust glue only if it is within the servers authority." | out-of-zone glue not trusted (AC-35) |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Out-of-zone glue not trusted | trust all glue | unbound.conf `harden-glue`: "Will trust glue only if it is within the servers authority." Default yes |
| `mode` default stays `forward` | default `recursive`, as Unbound | an existing forwarding config keeps its meaning when recursion lands |

## Known Limitations
- Under DNSSEC validation, DS records must be asked of the parent's servers (V-13 of `plan/post-release/features/spec-dns-resolver-dnssec-validation.md`); that is the validation spec's work.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-34 and AC-35 demonstrated
- [ ] The targeted tests for what changed pass: the scoped package tests and `./le verify changed` over the closing commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only
