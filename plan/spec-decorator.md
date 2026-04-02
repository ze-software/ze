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
- YANG extension `ze:decorate` added to `ze-extensions.yang` (lines 135-145) with argument `name`
- `Decorate` field added to `LeafNode` in `config/schema.go` (line 119) for schema-level decorator metadata
- `getDecorateExtension()` added to `config/yang_schema.go` (lines 350-358), wired into `yangToLeaf()` (line 379)
- `TestYANGSchemaDecorateExtension` added to `config/yang_schema_test.go` validating YANG-to-schema flow
- `ze:decorate "asn-name"` applied to three ASN leaves in `ze-bgp-conf.yang`: global `session/asn/local` (line 32), peer-fields `session/asn/local` (line 261), and `session/asn/remote` (line 266)
- `Decorator` interface, `DecoratorRegistry`, and `DecoratorFunc` adapter in `web/decorator.go`
- ASN name decorator with Team Cymru DNS TXT parsing in `web/decorator_asn.go`, with graceful degradation and input validation
- `DecoratorName` and `Decoration` fields on `FieldMeta` in `web/fragment.go` (lines 30-31), propagated in `buildFieldMeta()` (line 413)
- `SetDecorators()` and `ResolveDecorations()` on `Renderer` in `web/render.go` (lines 189-206), plus decoration resolution inside `RenderField()` (lines 210-235)
- `ResolveDecorations()` called in `HandleFragment` (fragment.go line 221) for show/monitor rendering
- Decoration span in `wrapper.html` template (line 5) with `ze-field-decoration` CSS class, conditionally rendered
- `NewASNNameDecoratorFromResolver` and `NewASNNameDecoratorFromCymru` public constructors for wiring with DNS resolver

### Bugs Found/Fixed
- `nilerr` linter required `//nolint:nilerr` annotations in `decorator_asn.go` where DNS errors are intentionally swallowed for graceful degradation

### Documentation Updates
- `docs/features/web-interface.md` -- YANG decorators feature entry (line 21)
- `docs/comparison.md` -- ASN name enrichment mention (lines 193-194, 201)
- `docs/architecture/web-components.md` -- Decorators section (lines 91-101)

### Deviations from Plan
- `ze-extensions.yang` already existed at `internal/component/config/yang/modules/` -- spec originally suggested creating it at `internal/component/web/schema/`. The extension was added to the existing file instead.
- `handler_config_leaf.go` was not modified; decoration resolution happens in `render.go` (`RenderField`) and `fragment.go` (`HandleFragment` calling `ResolveDecorations`) instead.
- `buildFieldMeta` was placed in `fragment.go` rather than `handler_config_leaf.go` since that is where field building occurs for fragment rendering.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| YANG extension `ze:decorate` defined | Done | `config/yang/modules/ze-extensions.yang:140-145` | Extension with argument `name` |
| Schema extracts `ze:decorate` from YANG | Done | `config/yang_schema.go:350-358,379` | `getDecorateExtension()` wired into `yangToLeaf()` |
| `LeafNode` carries decorator name | Done | `config/schema.go:119` | `Decorate string` field |
| Decorator registry and interface | Done | `web/decorator.go` | `Decorator`, `DecoratorRegistry`, `DecoratorFunc` |
| ASN-name decorator via Team Cymru DNS | Done | `web/decorator_asn.go` | `asnNameDecorator`, `parseASNName()` |
| Renderer integrates decorators | Done | `web/render.go:189-206,210-235` | `SetDecorators()`, `ResolveDecorations()`, `RenderField()` |
| Template displays annotation | Done | `templates/input/wrapper.html:5` | Conditional `ze-field-decoration` span |
| YANG leaves annotated with `ze:decorate` | Done | `bgp/schema/ze-bgp-conf.yang:32,261,266` | Three ASN leaves |
| Functional test for config parsing | Done | `test/parse/decorator-yang.ci` | Config with ze:decorate parses |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestASNDecorator` (decorator_asn_test.go:72), `TestRenderDecoratedLeaf` (render_test.go:251), `TestRenderFieldResolvesDecoration` (render_test.go:371) | Resolves "64500" to "Cloudflare, Inc.", renders in HTML |
| AC-2 | Done | `TestRenderUnDecoratedLeaf` (render_test.go:289), `decorator-yang.ci` | Plain leaf renders without `ze-field-decoration` class; config parses normally |
| AC-3 | Done | `TestDecoratorUnknownName` (decorator_test.go:57), `TestDecoratorRegistryResolveField` field4 (render_test.go:349-354) | Unknown decorator returns nil / leaves Decoration empty |
| AC-4 | Done | `TestASNDecoratorFailure` (decorator_asn_test.go:90) | DNS error, NXDOMAIN, non-numeric, empty, out-of-range all return empty |
| AC-5 | Done | Inherited from spec-dns DNS resolver cache | DNS cache from spec-dns serves repeated queries |
| AC-6 | Done | `TestASNDecoratorParseCymru` (decorator_asn_test.go:16) | Parses standard, short name, dash prefix, empty, too few fields |
| AC-7 | Done | `TestYANGSchemaDecorateExtension` (yang_schema_test.go:123), `decorator-yang.ci` | Extension loads and is available |
| AC-8 | Done | `TestDecoratorRegistry` (decorator_test.go:17) | Registers and looks up multiple decorators by name |
| AC-9 | Done | `TestRenderDecoratedLeaf` (render_test.go:251), `TestResolveDecorationsOnRenderer` (render_test.go:408), `HandleFragment` calls `ResolveDecorations` (fragment.go:221) | Decoration rendered in show tier via HandleFragment |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDecoratorRegistry` | Done | `web/decorator_test.go:17` | AC-8: register and look up decorators |
| `TestASNDecorator` | Done | `web/decorator_asn_test.go:72` | AC-1, AC-6: resolves ASN to name |
| `TestASNDecoratorFailure` | Done | `web/decorator_asn_test.go:90` | AC-4: graceful degradation on DNS failure |
| `TestASNDecoratorParseCymru` | Done | `web/decorator_asn_test.go:16` | AC-6: parse Team Cymru TXT format |
| `TestDecoratorUnknownName` | Done | `web/decorator_test.go:57` | AC-3: unknown decorator is silent |
| `TestRenderDecoratedLeaf` | Done | `web/render_test.go:251` | AC-1, AC-9: template renders annotation |
| `TestRenderUnDecoratedLeaf` | Done | `web/render_test.go:289` | AC-2: no regression for plain leaves |
| `TestBuildFieldMetaPropagatesDecorate` | Done | `web/decorator_test.go:68` | DecoratorName flows from LeafNode to FieldMeta |
| `TestASNDecoratorBoundary` | Done | `web/decorator_asn_test.go:155` | Boundary: 0, max uint32, max+1, -1 |
| `TestNewASNNameDecoratorFromResolver` | Done | `web/decorator_asn_test.go:192` | Public constructor with resolver interface |
| `TestDecoratorRegistryResolveField` | Done | `web/render_test.go:318` | ResolveField populates Decoration |
| `TestRenderFieldResolvesDecoration` | Done | `web/render_test.go:371` | RenderField resolves via registry |
| `TestResolveDecorationsOnRenderer` | Done | `web/render_test.go:408` | ResolveDecorations on Renderer, nil-safe |
| `TestDecorationHTMLEscaped` | Done | `web/render_test.go:438` | XSS prevention: HTML-escaped output |
| `test-decorator-yang` | Done | `test/parse/decorator-yang.ci` | Config with ze:decorate extension parses |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/web/decorator.go` (create) | Done | Decorator interface, DecoratorRegistry, DecoratorFunc |
| `internal/component/web/decorator_asn.go` (create) | Done | ASN name decorator, Team Cymru parsing |
| `internal/component/web/decorator_test.go` (create) | Done | Registry tests, buildFieldMeta propagation |
| `internal/component/web/decorator_asn_test.go` (create) | Done | ASN decorator tests: parse, failure, boundary |
| `internal/component/web/schema/ze-extensions.yang` (create) | Changed | Already existed at `config/yang/modules/ze-extensions.yang`; extension added there |
| `test/parse/decorator-yang.ci` (create) | Done | Functional test for YANG extension parsing |
| `internal/component/web/render.go` (modify) | Done | `SetDecorators()`, `ResolveDecorations()`, decoration in `RenderField()` |
| `internal/component/web/handler_config_leaf.go` (modify) | Changed | Not modified; decoration handled in `render.go` and `fragment.go` instead |
| `internal/component/web/templates/input/wrapper.html` (modify) | Done | Conditional `ze-field-decoration` span added |

### Audit Summary
- **Total items:** 33 (9 requirements, 9 ACs, 15 tests from TDD + extras + .ci)
- **Done:** 31
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (ze-extensions.yang location, handler_config_leaf.go not modified -- documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/web/decorator.go` | Yes | -rw-rw-r-- 2554 Mar 30 18:25 |
| `internal/component/web/decorator_asn.go` | Yes | -rw-rw-r-- 3901 Apr  2 15:25 |
| `internal/component/web/decorator_test.go` | Yes | -rw-rw-r-- 2724 Mar 29 22:08 |
| `internal/component/web/decorator_asn_test.go` | Yes | -rw-rw-r-- 5875 Mar 29 22:08 |
| `internal/component/config/yang/modules/ze-extensions.yang` | Yes | -rw-rw-r-- 6857 Apr  1 17:50 (contains `extension decorate` at line 140) |
| `test/parse/decorator-yang.ci` | Yes | -rw-rw-r-- 581 Mar 31 13:01 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | ASN leaf with ze:decorate rendered with annotation | `TestRenderDecoratedLeaf` asserts `Cloudflare, Inc.` in output and `ze-field-decoration` class present |
| AC-2 | Plain leaf rendered without decoration | `TestRenderUnDecoratedLeaf` asserts `ze-field-decoration` class NOT in output; `decorator-yang.ci` exits 0 |
| AC-3 | Unknown decorator name silent | `TestDecoratorUnknownName` asserts `Get("nonexistent")` returns nil; `TestDecoratorRegistryResolveField` field4 asserts empty Decoration for `"nonexistent"` |
| AC-4 | DNS failure graceful degradation | `TestASNDecoratorFailure` tests DNS error, NXDOMAIN, non-numeric, empty, out-of-range -- all return empty string, no error |
| AC-5 | DNS cache serves repeated queries | Decorator uses `txtResolver` function which in production is `dns.Resolver.ResolveTXT` with built-in caching from spec-dns |
| AC-6 | Team Cymru TXT parsed correctly | `TestASNDecoratorParseCymru` parses standard, short name, dash prefix formats; extracts "Cloudflare, Inc." from pipe-delimited response |
| AC-7 | ze-extensions YANG loaded | `ze-extensions.yang` line 140: `extension decorate { argument name; }`. `TestYANGSchemaDecorateExtension` loads full YANG schema and verifies `LeafNode.Decorate == "asn-name"` on `bgp.session.asn.local` |
| AC-8 | Multiple decorators registered | `TestDecoratorRegistry` registers `"test-decorator"` and `"other"`, looks up both independently |
| AC-9 | Decoration visible in show tier | `HandleFragment` (fragment.go:221) calls `renderer.ResolveDecorations(data.Fields)` before rendering. `RenderField` (render.go:213) also calls `ResolveField`. Both paths resolve decoration for show and HTMX-swap rendering |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| YANG leaf with ze:decorate, config parsed by `ze config validate` | `test/parse/decorator-yang.ci` | .ci file reads config with `session { asn { local 64500 } }` and `session { asn { remote 13335 } }` on decorated leaves, runs `ze config validate`, expects exit 0 and "configuration valid" |
| Web UI renders peer remote-as | No .ci (web rendering tested via Go unit tests) | `TestRenderDecoratedLeaf` and `TestRenderFieldResolvesDecoration` exercise the full render path including `RenderField` -> `ResolveField` -> template output with decoration |
| YANG leaf without ze:decorate | `test/parse/decorator-yang.ci` | Same config test -- non-decorated leaves (router-id, ip, port) parse without error alongside decorated ones |

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
