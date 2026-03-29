# Spec: YANG Decorator Framework

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-dns |
| Phase | 5/5 |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/web/render.go` - existing template rendering
4. `internal/component/web/handler_config_leaf.go` - leaf type dispatch
5. `internal/component/web/schema/ze-web-conf.yang` - YANG schema pattern
6. `internal/component/dns/resolver.go` - DNS resolver (from spec-dns)

## Task

Implement a YANG decorator framework that enriches leaf values at display time with
additional information resolved from external sources. The framework introduces a custom
YANG extension (`ze:decorate`) that can be attached to any leaf. When the web UI renders
a decorated leaf, it calls the registered decorator function to obtain supplementary
display text.

The first decorator is `asn-name`: when a leaf carrying `ze:decorate "asn-name"` displays
an AS number, the decorator resolves the AS name via Team Cymru DNS (`TXT ASxxxx.asn.cymru.com`)
using the DNS resolver component (`spec-dns.md`) and shows the name alongside the number
(e.g., `64500 (Cloudflare)`).

This is a general-purpose mechanism. Future decorators could include reverse DNS for IP
addresses, RPKI validation status, community name descriptions, or any other enrichment
that resolves external data for display.

### Why YANG Extension

The decorator is declared in the schema, making it explicit and auditable. The renderer
does not need to guess which fields to decorate -- the YANG extension is the single source
of truth. Adding a new decorator to any leaf requires only a schema change, no Go code
changes in the renderer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/config/syntax.md` - YANG schema conventions
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A (not protocol work)

**Key insights:**
- To be filled during implementation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/render.go` - template loading, RenderFragment, fieldFor dispatch
- [ ] `internal/component/web/handler_config_leaf.go` - leaf field type mapping
- [ ] `internal/component/web/templates/input/wrapper.html` - field label + tooltip wrapper
- [ ] `internal/component/web/templates/input/text.html` - text input template
- [ ] `internal/component/web/templates/input/number.html` - number input template
- [ ] `internal/component/config/` - YANG schema loading, extension handling

**Behavior to preserve:**
- Existing leaf rendering (text, number, bool, enum) unchanged for non-decorated leaves
- YANG schema loading and validation
- Template dispatch via fieldFor()
- Input templates structure and HTMX attributes

**Behavior to change:**
- Renderer gains awareness of `ze:decorate` extension on leaves
- Decorated leaves render with additional display text (annotation)
- New YANG extension definition needed

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Web UI renders a YANG leaf that has `ze:decorate "asn-name"` extension
- The leaf value is an AS number (e.g., `64500`)

### Transformation Path
1. Renderer calls `fieldFor()` to determine input type for the leaf
2. Renderer checks if leaf has `ze:decorate` extension
3. If decorated: look up registered decorator by name (`asn-name`)
4. Call decorator with the leaf value (`64500`)
5. Decorator queries DNS resolver: TXT query for `AS64500.asn.cymru.com`
6. Team Cymru returns: `"64500 | US | arin | 2005-06-01 | CLOUDFLARE - Cloudflare, Inc., US"`
7. Decorator extracts AS name: `Cloudflare, Inc.`
8. Renderer passes both value and decoration to template
9. Template displays: `64500` with annotation `Cloudflare, Inc.`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> Renderer | Extension metadata on leaf node | [ ] |
| Renderer -> Decorator | Go function call with leaf value | [ ] |
| Decorator -> DNS resolver | Go function call (ResolveTXT) | [ ] |
| DNS resolver -> Team Cymru | DNS TXT query | [ ] |

### Integration Points
- YANG extension definition in a shared YANG module
- Decorator registry (map of name -> decorator function)
- Web renderer (fieldFor / template dispatch)
- DNS resolver component (from spec-dns)

### Architectural Verification
- [ ] No bypassed layers (decorators go through registry, not hardcoded)
- [ ] No unintended coupling (renderer knows decorator interface, not implementations)
- [ ] No duplicated functionality (extends existing renderer, does not replace)
- [ ] Zero-copy preserved where applicable (decorator returns string, no buffer concerns)

## YANG Extension Design

The framework defines a custom YANG extension in a shared module:

| YANG element | Purpose |
|-------------|---------|
| Module: `ze-extensions` | Shared YANG module defining Ze-specific extensions |
| Extension: `ze:decorate` | Takes a string argument naming the decorator (e.g., `"asn-name"`) |
| Usage | Any leaf can carry `ze:decorate "decorator-name"` |

The `ze-extensions` module is imported by any YANG module that uses decorators. The
extension argument is a free-form string matching a registered decorator name.

### YANG Usage Pattern

A YANG module that wants to decorate a leaf imports `ze-extensions` and annotates:

| Leaf | Extension | Effect |
|------|-----------|--------|
| `remote-as` | `ze:decorate "asn-name"` | AS number displayed with org name |
| `local-as` | `ze:decorate "asn-name"` | AS number displayed with org name |
| Any future leaf | `ze:decorate "reverse-dns"` | IP displayed with hostname |

### Decorator Registry

| Name | Input | Output | Source | Depends on |
|------|-------|--------|--------|-----------|
| `asn-name` | AS number (uint32) | Organization name (string) | Team Cymru DNS TXT | spec-dns |

Future decorators (not in scope for this spec):

| Name | Input | Output | Source |
|------|-------|--------|--------|
| `reverse-dns` | IP address | Hostname | DNS PTR |
| `community-name` | Community string | Description | Local config or IANA |
| `rpki-status` | Prefix + origin AS | Valid/Invalid/Unknown | RPKI validator |

## Decorator Interface

| Method | Parameters | Returns | Description |
|--------|-----------|---------|-------------|
| Decorate | value (string) | annotation (string), error | Resolve display annotation for a value |
| Name | none | string | Decorator name (matches YANG extension argument) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG leaf with ze:decorate | -> | Renderer calls decorator, shows annotation | TBD |
| Web UI renders peer remote-as | -> | AS name shown next to number | TBD |
| YANG leaf without ze:decorate | -> | Rendered as before, no decorator call | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG leaf has `ze:decorate "asn-name"` and value is a valid ASN | Rendered with AS organization name annotation |
| AC-2 | YANG leaf has no `ze:decorate` extension | Rendered exactly as before (no regression) |
| AC-3 | Decorator name in YANG does not match any registered decorator | Leaf rendered without annotation (silent, no error) |
| AC-4 | ASN resolution fails (DNS timeout, NXDOMAIN) | Leaf rendered with value only, no annotation (graceful degradation) |
| AC-5 | Same ASN queried multiple times | DNS cache serves repeated queries (from spec-dns cache) |
| AC-6 | Team Cymru response parsed correctly | Organization name extracted from pipe-delimited TXT record |
| AC-7 | YANG module `ze-extensions` loaded | Extension `ze:decorate` available for all YANG modules |
| AC-8 | Multiple decorators registered | Each leaf uses its own decorator based on extension argument |
| AC-9 | Decoration visible in both show and looking-glass tiers | Annotation appears wherever the decorated leaf is rendered |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecoratorRegistry` | `internal/component/web/decorator_test.go` | AC-8: register and look up decorators | |
| `TestASNDecorator` | `internal/component/web/decorator_asn_test.go` | AC-1, AC-6: resolves ASN to name | |
| `TestASNDecoratorFailure` | `internal/component/web/decorator_asn_test.go` | AC-4: graceful degradation on DNS failure | |
| `TestASNDecoratorParseCymru` | `internal/component/web/decorator_asn_test.go` | AC-6: parse Team Cymru TXT format | |
| `TestDecoratorUnknownName` | `internal/component/web/decorator_test.go` | AC-3: unknown decorator is silent | |
| `TestRenderDecoratedLeaf` | `internal/component/web/render_test.go` | AC-1, AC-9: template renders annotation | |
| `TestRenderUnDecoratedLeaf` | `internal/component/web/render_test.go` | AC-2: no regression for plain leaves | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN input | 0-4294967295 | 4294967295 | N/A (0 is valid) | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-decorator-yang` | `test/parse/decorator-yang.ci` | Config with ze:decorate extension parses | |

### Future (if deferring any tests)
- Reverse DNS decorator
- Community name decorator
- RPKI status decorator

## Files to Modify
- `internal/component/web/render.go` - add decorator awareness to template rendering
- `internal/component/web/handler_config_leaf.go` - pass decoration data to templates
- `internal/component/web/templates/input/wrapper.html` - display annotation text

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (extension module) | [x] | `internal/component/web/schema/ze-extensions.yang` |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A (decorator is display-only) |
| Functional test | [x] | `test/parse/decorator-yang.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - YANG decorator framework |
| 2 | Config syntax changed? | [ ] | N/A (extension is in YANG schema, not user config) |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - automatic ASN name resolution |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - decorator framework |

## Files to Create
- `internal/component/web/decorator.go` - decorator registry and interface
- `internal/component/web/decorator_asn.go` - ASN-name decorator (Team Cymru DNS)
- `internal/component/web/decorator_test.go` - registry unit tests
- `internal/component/web/decorator_asn_test.go` - ASN decorator unit tests
- `internal/component/web/schema/ze-extensions.yang` - YANG extension module
- `test/parse/decorator-yang.ci` - functional test for YANG extension parsing

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG extension module** -- define ze-extensions YANG, register module
   - Tests: `schema_test.go` (extension loads)
   - Files: `schema/ze-extensions.yang`, `schema/embed.go`, `schema/register.go`
   - Verify: YANG module loads and extension is recognized
2. **Phase: Decorator registry** -- interface definition, registration, lookup
   - Tests: `TestDecoratorRegistry`, `TestDecoratorUnknownName`
   - Files: `decorator.go`, `decorator_test.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: ASN-name decorator** -- Team Cymru DNS resolution, response parsing
   - Tests: `TestASNDecorator`, `TestASNDecoratorFailure`, `TestASNDecoratorParseCymru`
   - Files: `decorator_asn.go`, `decorator_asn_test.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Renderer integration** -- wire decorators into template rendering
   - Tests: `TestRenderDecoratedLeaf`, `TestRenderUnDecoratedLeaf`
   - Files: `render.go`, `handler_config_leaf.go`, `templates/input/wrapper.html`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Functional tests** -- YANG parsing test
   - Tests: `test/parse/decorator-yang.ci`
   - Verify: `make ze-functional-test` passes
6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Team Cymru TXT parsing handles all observed response formats |
| Naming | YANG extension uses `ze:decorate`, decorator names are kebab-case |
| Data flow | Decorator calls go through registry, not hardcoded in renderer |
| Rule: design-principles | Decorator interface is minimal; no premature abstraction |
| Rule: no-layering | No duplicate rendering paths for decorated vs undecorated |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG extension module exists | `ls internal/component/web/schema/ze-extensions.yang` |
| Decorator registry works | `go test -run TestDecoratorRegistry ./internal/component/web/...` |
| ASN decorator resolves names | `go test -run TestASNDecorator ./internal/component/web/...` |
| Rendered output shows annotation | `go test -run TestRenderDecoratedLeaf ./internal/component/web/...` |
| No regression on plain leaves | `go test -run TestRenderUnDecoratedLeaf ./internal/component/web/...` |
| Config parses | functional test `decorator-yang.ci` passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | ASN value validated as numeric before DNS query construction |
| Injection | Decorator does not pass unsanitized values into DNS query strings |
| Resource exhaustion | DNS cache (from spec-dns) bounds memory; decorator does not cache separately |
| Error leakage | DNS errors not exposed in rendered HTML |
| XSS | Decorator output HTML-escaped before rendering in template |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A -- not protocol work.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
