# Spec: Public Go API for BGP Primitives (pkg/ze/bgp)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/compatibility.md` - post-release contract rules
4. `pkg/ze/` - existing public surface
5. `internal/component/bgp/message/` - wire decoder/encoder
6. `internal/component/bgp/wire/` - WireUpdate and pool types

## Task

Ze carries a rich set of BGP primitives (wire decoder and encoder, prefix types,
attribute pool, WireUpdate, ContextID, PackContext, family registry). All of
them live under `internal/` today, which means any Go project outside the ze
daemon cannot reuse them. A side tool that wants to parse a BGP UPDATE has to
either depend on an unrelated third-party library or vendor chunks of ze's
internal packages.

This spec covers exposing a curated, stable subset of BGP primitives under
`pkg/ze/bgp/` so that external Go programs (test harnesses, analysis tooling,
third-party plugins, integration shims) can import the ze types directly.

The goal is **not** to expose the daemon. It is to expose:
- Wire decode / encode (one UPDATE in, structured view out, and the reverse)
- Prefix and path types (`Prefix`, `Path`, `ASPath`, attributes)
- Family identifiers
- Whatever is needed for those to be self-contained

The goal is also to **lock the contract** once published. The rule in
`.claude/rules/compatibility.md` is that everything under `internal/` may
change freely, but once a surface is exposed in `pkg/` the signatures become
frozen post-release.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] - checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` - overall layering
- [ ] `.claude/rules/compatibility.md` - what "public API" means for ze
- [ ] `.claude/rules/design-principles.md` - encapsulation onion, lazy over eager
  → Constraint: the public API must not force callers to hold onto parsed
  structs; it must expose the lazy iterators that `internal/` uses today.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - base BGP wire format
- [ ] `rfc/short/rfc4760.md` - multiprotocol NLRI / MP_REACH / MP_UNREACH
- [ ] `rfc/short/rfc7911.md` - ADD-PATH (must round-trip through the public API)

**Key insights:** (summary of all checkpoint lines)
- The public API is a re-export layer, not a re-implementation. Do not duplicate.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `pkg/ze/` - what is currently public (engine, config, plugin, subsystem; no BGP types)
- [ ] `internal/component/bgp/message/` - eager-style decoder and family constants
- [ ] `internal/component/bgp/wire/` - lazy WireUpdate, ContextID, PackContext
- [ ] `internal/component/bgp/nlri/` - NLRI iterator pattern
- [ ] `internal/component/bgp/attr/` - attribute pool, dedup, refcounting

**Behavior to preserve:**
- `internal/` code may continue to use its own direct imports; the public
  package is additive.
- Zero-copy, buffer-first encoding remains mandatory. The public API must not
  introduce a code path that allocates per UPDATE.
- Lazy iteration remains the default. The public API must not return slices of
  structs where `internal/` returns iterators.

**Behavior to change:**
- None. This spec only exposes what already exists.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- External Go program imports `pkg/ze/bgp`, calls decode on a byte slice.

### Transformation Path
1. External bytes -> `bgp.DecodeUpdate(buf)` -> thin wrapper -> existing
   `internal/component/bgp/wire` `WireUpdate`
2. Existing iterators (`NLRIs`, `Attributes`) exposed as public iterators
3. Encode path: caller provides pooled buffer (or caller-owned buffer) +
   `bgp.Encoder.WriteUpdate(buf, update)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| External -> internal | re-export in `pkg/ze/bgp` | [ ] |
| Public type -> internal impl | unexported field holding the internal value | [ ] |

### Integration Points
- `pkg/ze/` already exists with `engine`, `config`, `plugin`, `subsystem`. New
  subpackage `pkg/ze/bgp/` sits beside them.
- `internal/component/bgp/message` decoder stays the canonical implementation.

### Architectural Verification
- [ ] Public API adds no per-UPDATE allocations
- [ ] Public API calls internal code directly (no parallel impl)
- [ ] Public API surface is a strict subset of what internal exposes
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| External Go program importing `pkg/ze/bgp` | → | `pkg/ze/bgp.DecodeUpdate` | `TestPublicDecodeRoundTrip` |
| External Go program importing `pkg/ze/bgp` | → | `pkg/ze/bgp.EncodeUpdate` | `TestPublicEncodeIntoBuffer` |
| External Go program iterating NLRIs | → | `pkg/ze/bgp.Update.NLRIs` | `TestPublicNLRIIterator` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build ./pkg/ze/bgp/...` from a clean checkout | Builds with no dependency on `internal/` leaking into the exported signatures |
| AC-2 | A program outside the `ze` repo imports `pkg/ze/bgp` | Can decode a BGP UPDATE byte slice into a usable view |
| AC-3 | Same program iterates NLRIs and attributes | Uses the public iterator API with no reflection or internal type access |
| AC-4 | Same program encodes a synthesized UPDATE into a pool buffer | Bytes round-trip back through decode and match the original semantics |
| AC-5 | MP_REACH and MP_UNREACH with IPv6 unicast | Round-trip through decode/encode without data loss |
| AC-6 | ADD-PATH family | Path-ID is preserved through decode/encode |
| AC-7 | A signature exported from `pkg/ze/bgp` is changed | Go's API-compat checker (`gorelease` or equivalent) flags it |
| AC-8 | `go doc pkg/ze/bgp` | Every exported symbol has a doc comment |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPublicDecodeRoundTrip` | `pkg/ze/bgp/update_test.go` | Decode -> re-encode byte-identical for a representative corpus | |
| `TestPublicEncodeIntoBuffer` | `pkg/ze/bgp/encode_test.go` | Caller-owned buffer is the only allocation path | |
| `TestPublicNLRIIterator` | `pkg/ze/bgp/nlri_test.go` | Iterator yields the same prefixes as the internal iterator | |
| `TestPublicFamilyConstants` | `pkg/ze/bgp/family_test.go` | Public family constants match internal values | |
| `TestPublicAPIStability` | `pkg/ze/bgp/stability_test.go` | `go doc`-level signature list matches a golden file | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-pkg-ze-bgp-decode` | `test/pkg/*.ci` | External consumer decodes captured UPDATE bytes | |

### Future (if deferring any tests)
- Post-release: add `gorelease` to `make ze-verify` to catch accidental breakage.

## Files to Modify
- `pkg/ze/` - ensure the new subpackage fits the existing layout

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new API | Yes | `test/pkg/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - note public Go API |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | Yes | new page: `docs/guide/public-go-api.md` |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/public-go-api.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - add "public Go API" row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - public surface boundary |

## Files to Create
- `pkg/ze/bgp/doc.go` - package doc and stability statement
- `pkg/ze/bgp/update.go` - public `Update` type + `DecodeUpdate`
- `pkg/ze/bgp/encode.go` - `EncodeUpdate` into caller-owned buffer
- `pkg/ze/bgp/nlri.go` - public NLRI iterator
- `pkg/ze/bgp/attr.go` - public attribute accessor
- `pkg/ze/bgp/family.go` - public family constants
- `pkg/ze/bgp/stability_test.go` - API surface golden file
- `docs/guide/public-go-api.md` - user guide for external consumers
- `test/pkg/pkg-ze-bgp-decode.ci` - end-user round-trip test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist |
| 6. Fix issues | - |
| 7. Re-verify | Re-run stage 4 |
| 9. Deliverables review | Deliverables Checklist |
| 10. Security review | Security Review Checklist |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

1. **Phase: Scope curation** - inventory internal types and pick the smallest
   closed subset that supports decode + iterate + encode for IPv4/IPv6 unicast
   and ADD-PATH. Document what is deliberately left out.
2. **Phase: Re-export layer** - create `pkg/ze/bgp/*.go` wrapping the internal
   types. Unexported fields only. Constructors call internal functions.
3. **Phase: Stability test** - write `stability_test.go` that compares the
   current API surface (generated via `go doc`-style listing) to a checked-in
   golden file. Changes to the golden must be intentional.
4. **Phase: Functional test** - external-style test under `test/pkg/` that
   imports only `pkg/ze/bgp` and round-trips a captured UPDATE.
5. **Phase: Docs** - write `docs/guide/public-go-api.md` and update
   `docs/features.md`, `docs/comparison.md`, `docs/architecture/core-design.md`.
6. **Full verification** - `make ze-verify`.
7. **Complete spec** - audit, learned summary, spec removal.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation evidence |
| Correctness | Decode/encode round-trip is byte-identical where allowed |
| Naming | Public types follow `pkg/ze/` naming convention (no redundant `BGP` prefix inside `bgp` package) |
| Data flow | No parallel decoder: public API calls internal code |
| Rule: no-layering | Old `internal/` paths untouched; public package is additive |
| Rule: compatibility | Signatures reviewed as "post-release frozen" |
| Allocation | `go test -bench` shows zero per-UPDATE allocation for decode |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `pkg/ze/bgp/` package exists | `ls pkg/ze/bgp/` |
| Package compiles standalone | `go build ./pkg/ze/bgp/...` |
| Stability golden file exists | `ls pkg/ze/bgp/testdata/api-surface.golden` |
| External-style test passes | functional test log |
| User guide published | `ls docs/guide/public-go-api.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Decode of malformed bytes must not panic; fuzz corpus from existing `internal/component/bgp/message/fuzz_test.go` is reused |
| Resource exhaustion | Caller-controlled buffer size; reject lengths above RFC max |
| Error leakage | Decode errors name the offending field, not internal file paths |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Round-trip byte mismatch | Re-check internal encoder assumptions first |
| API surface changed without intent | Review golden diff; do not blindly update |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

## RFC Documentation

Add `// RFC NNNN Section X.Y: "..."` comments on public exports where the
behavior is directly tied to the RFC (family IDs, path-ID location, MP_REACH
layout).

## Implementation Summary

### What Was Implemented
- (fill during /implement)

### Bugs Found/Fixed
- (fill during /implement)

### Documentation Updates
- (fill during /implement)

### Deviations from Plan
- (fill during /implement)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] External-style test imports only `pkg/ze/bgp`
- [ ] API stability golden file in place
- [ ] Architecture docs updated

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional test for end-user behavior

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-pkg-ze-bgp.md`
- [ ] Summary included in commit
