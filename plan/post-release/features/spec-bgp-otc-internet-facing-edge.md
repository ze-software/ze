# Spec: bgp-otc-internet-facing-edge

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | RFC 9234 is declared by no release profile (`rfc/profiles/` names it nowhere), so its absent requirement waits until after the first release |
| RFC gaps | RFC9234-5-9 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Provenance: AC-11 of `plan/release/validation/spec-rfc-verdict-test-fix-pass.md`
needs every `weak` audit verdict named in an owning spec's acceptance criteria.
`rfc/audit/rfc9234/RFC9234-5-9.json` is `weak` and no spec named it. The
closest existing spec, `plan/post-release/features/spec-bgp-deferred-confederation-otc.md`,
has the same boundary rule for a confederation (its AC-2, RFC9234-5-8), but the
edit gate refused it for its missing `## Existing Mechanisms` section, so this
item has its own spec.

The absent requirement, RFC 9234 Section 5, as row `RFC9234-5-9` keeps it (the
RFC breaks the line after "Internet-"):

"On egress from the Internet- facing AS, the OTC Attribute MUST NOT contain a value other than the Internet-facing ASN."

The sentences before it set the scenario: "The procedures specified in this
document in scenarios that use private AS numbers behind an Internet-facing ASN
(e.g., a data-center network [RFC7938] or stub customer) may be used, but any
details are outside the scope of this document."

What the producers do today (read 2026-11-14):

- `OTCEgressFilter` (`internal/component/bgp/plugins/role/otc.go`) adds an OTC
  only when the route has none, using `dest.LocalAS`. When an OTC is already
  present, it keeps the value.
- `otcAttrModHandler` (same file) also keeps an OTC that is already present,
  which is the RFC9234-5-6 rule: "Once the OTC Attribute has been set, it MUST
  be preserved unchanged (this also applies to an AS Confederation)."
- Ze has no concept of an Internet-facing AS. When an OTC is set to a private
  ASN inside a privately numbered domain, it leaves the Internet-facing edge
  unchanged.
- The only unit tagged for the row, `TestOTCEgressStampLocalASN`
  (`role/rfc9234_otc_test.go`), proves the absent-OTC case only. An independent
  judge refused a negative test offered for this row, because the test proves
  RFC9234-5-6 and not this row. It now carries the RFC9234-5-6 tag.

The open design question: how Ze knows that an egress is the Internet-facing
edge of a privately numbered domain, and how the rule for that edge fits with
the preserve rule of RFC9234-5-6. The design phase owes the FRR and BIRD source
reading that `ai/META-DECISIONS.md` requires before a protocol design is
approved, with the deciding function quoted.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-plugins.md`, `ai/doors/bgp-engine.md` - the doors for the role plugin and the filter API
  → Constraint: a procedure that a feature's config implies is a filter-list member, inserted by its plugin (`bgp-plugins.md`, Boundaries)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9234.md` - rows RFC9234-5-6 and RFC9234-5-9, and its "Coverage limits" section
- [ ] `rfc/full/rfc9234.txt` Section 5 - the private-AS paragraph and the preserve rule

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-14, skeleton survey only)
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCEgressFilter` adds an OTC when it is absent; when the source has an OTC, `otcAttrModHandler` calls `KeepAll` and refuses a suppression with a warning
- [ ] `internal/component/bgp/plugins/role/rfc9234_otc_test.go` - `TestOTCEgressStampLocalASN`, the only unit tagged for RFC9234-5-9
- [ ] `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as.go` - `evaluateRemovePrivateAS`, the operator-attached private-ASN AS_PATH rewrite

**Behavior to preserve:** RFC9234-5-6, keep an OTC that is already set, everywhere the RFC does not ask for this edge rule.

**Behavior to change:** the design phase decides this.

## Existing Mechanisms (MANDATORY)

Candidates only. The design phase confirms or rejects each one.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| OTC egress filter | `internal/component/bgp/plugins/role/otc.go` `OTCEgressFilter`, `otcAttrModHandler` | Adds an OTC when it is absent and keeps an OTC that is present | Candidate to extend: this is where the egress decision for the Internet-facing edge would go |
| Remove private ASNs filter | `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as.go` `evaluateRemovePrivateAS` | A named filter type, `remove-private-as`, that the operator attaches to a chain. When AS_PATH or AS4_PATH contains an RFC 6996 Private Use ASN, it returns a remove intent for `as-path`, and the engine does the rewrite. It does not touch OTC | Partial precedent only: where the operator attaches it already marks an edge where private ASNs must not leave. It is not an OTC rewrite and does not decide this design |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Per-destination forwarding through the role plugin's egress filter (`OTCEgressFilter`). The design phase confirms this.

### Transformation Path
1. The design phase fills this.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Role filter → wire encoding | `ModAccumulator` ops resolved by `otcAttrModHandler` | [ ] |

### Integration Points
- `internal/component/bgp/plugins/role/` - the OTC egress decision (candidate)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE with a private-ASN OTC forwarded out of the Internet-facing edge | → | egress decision for the edge (design phase names it) | design phase names it |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Row `RFC9234-5-9`, audit verdict `weak` (owned here for AC-11 of `plan/release/validation/spec-rfc-verdict-test-fix-pass.md`). A route carries an OTC set to a private ASN inside a domain numbered with private ASNs behind an Internet-facing ASN, and the route leaves through that domain's Internet-facing edge | No OTC with a value other than the Internet-facing ASN reaches the external peer. Proven when a negative unit sends a private-ASN OTC out of the Internet-facing edge and asserts that the value does not leave, a positive unit and that negative both carry the `RFC9234-5-9` tag, `./le rfc record` observes a recorded break of the producer turning the negative red, and an agent that wrote neither test re-judges the verdict `enforced` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (existing) `TestOTCEgressStampLocalASN` | `internal/component/bgp/plugins/role/rfc9234_otc_test.go` | RFC9234-5-9 positive, absent-OTC case | |
| negative unit, named at design | `internal/component/bgp/plugins/role/` | AC-1: a private-ASN OTC does not leave the Internet-facing edge | planned |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named at design | `test/plugin/` | An operator runs a privately numbered domain behind an Internet-facing ASN, and the external peer never receives a private-ASN OTC | planned |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| named at design | `test/interop/scenarios/` | FRR or BIRD, whichever the design-phase source reading shows can print a received OTC | AC-1 on the wire | planned |

## Files to Modify
- `internal/component/bgp/plugins/role/otc.go` - candidate, confirmed at design
- `rfc/short/rfc9234.md` - the "Coverage limits" section, once AC-1 is met

## Implementation Steps
1. Design phase: read FRR's and BIRD's OTC egress source, then answer the design question in the Task.
2. Write the red negative unit, then the fix, then record the break with `./le rfc record`.
3. Independent re-judge of `rfc/audit/rfc9234/RFC9234-5-9.json`.

## Known Limitations
- Skeleton: no design yet.

## RFC Documentation (Scope: protocol)
The enforcing code carries `// RFC 9234 Section 5: "..."` above the check, quoted byte-exact.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 demonstrated
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)

- [ ] `./le test unit pkg path ./internal/component/bgp/plugins/role/` passes
- [ ] `./le rfc check` clean for `rfc9234`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
