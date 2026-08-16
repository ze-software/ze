# Spec: firewall-prefix-normalize

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
3. `internal/component/firewall/config.go` - `parseAddressMatch`
4. `internal/component/firewall/validate.go` - `validateMatch` (and the acknowledged gap comment)
5. `internal/component/firewall/model.go` - `MatchSourceAddress` / `MatchDestinationAddress`

## Task

A firewall rule whose source/destination address has host bits set (e.g.
`10.10.10.1/30`) is stored verbatim in the config model. There is no
canonicalization to the network address (`10.10.10.0/30`) and no validation
rejecting the non-canonical form. The dataplane backends mask defensively so the
*installed rule* is correct, but the config model round-trips a non-canonical value.
That produces idempotency/diff noise, gives the operator no feedback that their
input was imprecise, and leaves a latent trap for any future consumer that reads the
stored prefix without masking.

Canonicalize (or reject) non-canonical IPv4/IPv6 prefixes in firewall rule
source/destination address at parse/verify time, so the stored model is always the
network address.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config idempotency expectations.
  → Constraint: the resolved config model should be canonical so re-reading it produces no spurious diff.
- [ ] `ai/rules/config.md` - firewall address matches are operator config.
  → Constraint: decide canonicalize-silently vs reject-with-error (design decision; canonicalize is the gentler default).

**Key insights:**
- Both dataplane backends already mask (nft `maskedAddr`/`prefixMask`; vpp `.Masked()`), so this is about the *model*, not the installed rule.
- `netip.Prefix.Masked()` gives the canonical network prefix directly.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/config.go` - `parseAddressMatch` calls `netip.ParsePrefix(v)` and stores the result verbatim in `MatchSourceAddress{Prefix}` / `MatchDestinationAddress{Prefix}` (config.go); no `.Masked()`, no host-bits check.
- [ ] `internal/component/firewall/validate.go` - `validateMatch` (validate.go) has no case for `MatchSourceAddress`/`MatchDestinationAddress`; the code comments the gap explicitly: "Parallel gap for literal MatchSourceAddress/DestinationAddress is tracked separately (pre-existing, not introduced here)" (validate.go).
- [ ] `internal/component/firewall/model.go` - `MatchSourceAddress struct{ Prefix netip.Prefix }` / `MatchDestinationAddress` (model.go,284).
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - backend masks defensively (`lowerAddrMatch` at :370-403, `prefixMask`/`maskedAddr` at :982-1001), so the kernel rule is correct despite the stored host bits.

**Behavior to preserve:**
- The installed dataplane rule stays correct (backends keep masking; no regression there).
- Address ranges (e.g. `10.10.13.1-10.10.13.16`) and single addresses are handled as today.
- Negation (`!prefix`) semantics preserved.

**Behavior to change:**
- The stored model prefix becomes canonical (network address), applied at parse and/or asserted at verify.

## Data Flow (MANDATORY)

### Entry Point
- Config: firewall rule `source address` / `destination address` values, parsed by `parseAddressMatch` (config.go).

### Transformation Path
1. Operator sets a rule address (possibly with host bits).
2. `parseAddressMatch` parses it via `netip.ParsePrefix`.
3. NEW: canonicalize with `.Masked()` before storing (or, if design chooses reject, compare `p != p.Masked()` and error at verify).
4. Stored `MatchSourceAddress`/`MatchDestinationAddress` now holds the network prefix.
5. Backends lower the (now already-canonical) prefix unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ firewall model | `parseAddressMatch` canonicalizes prefix | [ ] |
| Model ↔ verify | `validateMatch` gains address-match handling (if reject path) | [ ] |
| Model ↔ dataplane | backends receive an already-canonical prefix | [ ] |

### Integration Points
- `parseAddressMatch` (`config.go`) - canonicalize here.
- `validateMatch` (`validate.go`) - add the address-match case (closes the acknowledged gap).

### Architectural Verification
- [ ] No bypassed layers (canonicalization at the single parse point)
- [ ] No unintended coupling (backends unchanged; they still mask harmlessly)
- [ ] No duplicated functionality (one `.Masked()` at parse, not per-backend)
- [ ] Registration over hardcoding — firewall match parsing/validation stays in the firewall component; no central change.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Canonicalizing silently is acceptable operator UX | common firewall behaviour | operator prefers rejection | design confirmation with user | unvalidated |
| A-2 | `parseAddressMatch` is the only entry that builds these matches | config.go | other builders bypass it | grep constructors of MatchSource/DestinationAddress | unvalidated |
| A-3 | No existing config relies on host bits being preserved | backends already mask | behaviour change surprises someone | scan tests/config fixtures | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Canonicalization changes a stored value operators typed | diff on first re-commit | document; canonical form matches the effective rule anyway |
| R-2 | Ranges/single-addresses accidentally masked | range endpoints altered | only apply `.Masked()` to the prefix (CIDR) case, not ranges |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| rule `source address 10.10.10.1/30` | → | `parseAddressMatch` stores `10.10.10.0/30` | `test/plugin/firewall-prefix-normalize.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `source address 10.10.10.1/30` | stored/model value is `10.10.10.0/30` |
| AC-2 | `destination address 2001:db8:10:10::1/126` | stored value is `2001:db8:10:10::/126` |
| AC-3 | already-canonical `10.0.0.0/24` | unchanged |
| AC-4 | single address `10.10.14.1` | unchanged (no prefix) |
| AC-5 | range `10.10.13.1-10.10.13.16` | unchanged (not masked) |
| AC-6 | negated `!10.10.15.1/30` | stored `!10.10.15.0/30`, negation preserved |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enters a rule with host bits set | parse canonicalizes → model holds network prefix → no re-commit diff | `test/plugin/firewall-prefix-normalize.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAddressMatchCanonicalizesV4` | `internal/component/firewall/config_test.go` | host bits cleared for v4 CIDR | |
| `TestParseAddressMatchCanonicalizesV6` | `internal/component/firewall/config_test.go` | host bits cleared for v6 CIDR | |
| `TestParseAddressMatchPreservesRange` | `internal/component/firewall/config_test.go` | ranges/singles unchanged | |
| `TestParseAddressMatchNegation` | `internal/component/firewall/config_test.go` | negation preserved after canonicalization | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length v4 | 0-32 | 32 | N/A | 33 |
| prefix length v6 | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-prefix-normalize` | `test/plugin/firewall-prefix-normalize.ci` | non-canonical prefix stored canonical, no re-commit diff | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - config normalization, no wire protocol | - | - | validated by functional + dataplane tests | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/firewall/config.go` - canonicalize in `parseAddressMatch`
- `internal/component/firewall/validate.go` - add address-match case (closes the acknowledged gap comment)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Functional test for new behaviour | [ ] yes | `test/plugin/firewall-prefix-normalize.ci` |
| YANG validation constraints | [ ] maybe | if reject-path chosen, a validator on the address leaf |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` (canonicalization note) |
| 17 | Existing docs show examples for this area? | [ ] yes | verify firewall address examples |

## Files to Create
- `test/plugin/firewall-prefix-normalize.ci` - functional test
- (unit tests extend `config_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add a failing `test/plugin/firewall-prefix-normalize.ci` asserting the stored prefix is canonical.
2. **Phase: Canonicalize** — apply `.Masked()` to the CIDR case in `parseAddressMatch`; leave ranges/singles untouched.
   - Tests: `TestParseAddressMatchCanonicalizesV4/V6`, `TestParseAddressMatchPreservesRange`, `TestParseAddressMatchNegation`
3. **Phase: Verify case** — add the address-match handling in `validateMatch`, removing the acknowledged-gap comment.
4. **Functional test**
5. **Full verification** → `make ze-precommit-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | only CIDR masked; ranges/singles/negation intact; both v4 and v6 |
| Data flow | canonicalization at the single parse point |
| Rule: no-layering | acknowledged-gap comment at validate.go removed once closed |
| Registration over hardcoding | change confined to firewall component |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| canonicalization | `go test ./internal/component/firewall -run AddressMatch` |
| no re-commit diff | functional test round-trips config |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | malformed prefixes still rejected by `ParsePrefix` |
| Correctness | canonicalization never widens the match (network address is never broader than the input) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
