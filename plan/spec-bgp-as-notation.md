# Spec: bgp-as-notation

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
3. `internal/core/bgp/attribute/text_append.go` - AS-path/ASN text formatting
4. `internal/core/bgp/attribute/text.go` - AS-path text parsing
5. `internal/component/config/schema.go` - uint32 (ASN) config value parsing

## Task

Ze only understands AS numbers in "asplain" form: a plain decimal integer on
both config input and display output. Operators who think in "asdot" notation
(for a 4-byte ASN `X.Y`, where `ASN = X*65536 + Y`, e.g. `1.10` = 65546) cannot
type an ASN in dotted form, and Ze never renders ASNs in dotted form.

Add AS-notation support:
1. Accept an asdot `X.Y` token anywhere an ASN is configured, canonicalising it
   to the underlying uint32.
2. Add a BGP-global `as-notation` option (`asdot` | `asdot+`) that controls how
   ASNs and AS paths are rendered in CLI / looking-glass output. Default stays
   asplain (unchanged output) when the option is absent.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/performance.md` - the attribute text formatters are buffer-first and config-free.
  → Constraint: `core/bgp/attribute` is a leaf package and must NOT import config; the notation choice must be passed in as a parameter, not read from global state.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - the new BGP-global leaf.
  → Constraint: `as-notation` is a display preference, so it is a YANG leaf (not an env var), scoped to BGP global parameters.

**Key insights:**
- asplain↔asdot is a pure integer<->string transform; the stored/wire value is always uint32.
- Input acceptance (parse `X.Y`) and output rendering (format as `X.Y`) are independent; input is the cheap, high-value half.
- The formatters live in a leaf package with no config access, so display notation must thread through a format parameter/context, not a package global.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/attribute/text_append.go` - `(*ASPath).AppendText` (text_append.go) renders every ASN via `strconv.AppendUint(buf, uint64(asn), 10)` (:158 single, :170 in a list); aggregator ASN at :29. Always base-10 asplain, no notation parameter.
- [ ] `internal/core/bgp/attribute/text.go` - `ParseASPathText` parses each token with `strconv.ParseUint(tok, 10, 32)` (text.go); rejects any token containing a dot.
- [ ] `internal/component/config/schema.go` - config value parse for an ASN (typedef `asn` = uint32) uses `strconv.ParseUint(value, 10, 32)` (schema.go); no dotted form.

**Behavior to preserve:**
- Stored/wire ASN representation stays uint32 everywhere; no change to the on-wire encoding.
- Default output is byte-for-byte identical to today (asplain) when `as-notation` is unset.
- Existing asplain config input keeps working unchanged.

**Behavior to change:**
- ASN config parsing additionally accepts a dotted `X.Y` token and canonicalises to uint32.
- ASN/AS-path display renders in asdot/asdot+ when the BGP-global `as-notation` option selects it.

## Data Flow (MANDATORY)

### Entry Point
- Config input: any ASN leaf (`local-as`, `peer-as`, `session/asn`, aggregator, filter AS lists) typed as either a decimal or a dotted `X.Y` string.
- Config input: new BGP-global `as-notation` leaf (`asdot` | `asdot+`).
- Display output: CLI / looking-glass / RIB-replay rendering of ASNs and AS paths.

### Transformation Path
1. On config parse, an ASN token is normalised: if it contains `.`, split into `X` and `Y`, validate each in range, compute `X*65536 + Y`; else parse as decimal. Result is uint32 stored as today.
2. The `as-notation` leaf is read into BGP global settings and made available to the display layer as a notation mode value (asplain default / asdot / asdot+).
3. At render time, the ASN formatter receives the notation mode and emits either decimal (asplain) or `X.Y` (asdot: dotted only for ASN > 65535; asdot+: dotted for all).
4. AS-path rendering applies the same per-ASN formatter so paths render consistently.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config text ↔ uint32 ASN | asdot/asplain normaliser at ASN parse | [ ] |
| BGP config ↔ display layer | notation mode threaded as a format parameter | [ ] |
| ASN uint32 ↔ output text | notation-aware formatter (asplain/asdot/asdot+) | [ ] |

### Integration Points
- `internal/component/config/schema.go` (or the ASN typedef validation path) - accept dotted input.
- `internal/core/bgp/attribute/text.go` - accept dotted token in `ParseASPathText`.
- `internal/core/bgp/attribute/text_append.go` - notation-aware ASN/AS-path append.
- BGP-global YANG (`parameters`) - new `as-notation` leaf, read into settings and passed to the formatter.

### Architectural Verification
- [ ] No bypassed layers (notation is a parameter to the formatter, not a package global read inside the leaf package)
- [ ] No unintended coupling (`core/bgp/attribute` gains no config import)
- [ ] No duplicated functionality (one shared asdot normaliser and one shared formatter)
- [ ] Registration over hardcoding - the display option is a config leaf read into settings; no per-notation switch is hardcoded into a shared/core package beyond the single formatter helper.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All ASN parse paths funnel through the uint32 validator + `ParseASPathText` | schema.go, text.go | additional parse sites accept only decimal | grep every `ParseUint(.*32)` ASN site during audit | unvalidated |
| A-2 | The formatter can receive a notation mode without a config import | buffer-first leaf-package rule | display change is deeper than expected | thread a mode parameter/context in the audit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Display threading touches many call sites (looking glass, RIB replay, CLI) | formatter callers multiply | ship input-acceptance first (Phase 2); land display (Phase 3) behind the default-off option |
| R-2 | Ambiguous token `1.10` vs an IP-like value in some leaf | parser confusion | only ASN-typed leaves get the asdot normaliser; IP leaves are unaffected |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set ... local-as 1.10` | → | ASN normaliser yields 65546 | `test/ci/bgp-as-notation.ci` |
| `set protocols bgp parameters as-notation asdot` | → | notation-aware formatter renders `1.10` | `test/ci/bgp-as-notation.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | config ASN `1.10` | canonicalises to 65546 uint32 |
| AC-2 | config ASN `65546` | still parses (asplain preserved) |
| AC-3 | `as-notation asdot`, render ASN 65546 | outputs `1.10` |
| AC-4 | `as-notation asdot`, render ASN 100 | outputs `100` (2-byte stays plain) |
| AC-5 | `as-notation asdot+`, render ASN 100 | outputs `0.100` (dotted for all) |
| AC-6 | `as-notation` unset | output byte-for-byte identical to today (asplain) |
| AC-7 | invalid asdot `1.99999` (Y out of range) | rejected with a clear error |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a peer AS in asdot and sets asdot display | ASN normaliser → settings → notation-aware formatter | `test/ci/bgp-as-notation.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseASNAsdot` | `internal/component/config/schema_test.go` | `X.Y` → uint32, range-checked | |
| `TestParseASPathTextAsdot` | `internal/core/bgp/attribute/text_test.go` | dotted token accepted in AS path | |
| `TestASPathAppendTextNotation` | `internal/core/bgp/attribute/text_append_test.go` | asplain/asdot/asdot+ rendering | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| asdot X | 0..65535 | 65535 | - | 65536 |
| asdot Y | 0..65535 | 65535 | - | 65536 |
| ASN (combined) | 1..4294967295 | 4294967295 | 0 | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-as-notation` | `test/ci/bgp-as-notation.ci` | asdot input accepted; asdot display emitted | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| asdot is a local notation only; wire ASN is unchanged uint32 | - | - | no on-wire change; interop unaffected | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/config/schema.go` - accept dotted ASN input at the uint32/ASN parse path
- `internal/core/bgp/attribute/text.go` - accept dotted token in `ParseASPathText`
- `internal/core/bgp/attribute/text_append.go` - notation-aware ASN / AS-path formatter
- BGP-global YANG (`parameters` block) - add `as-notation` leaf; read into BGP settings and pass to the formatter

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaf) | [ ] yes | BGP `parameters` `as-notation`; `ai/rules/config.md` |
| Functional test | [ ] yes | `test/ci/bgp-as-notation.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/ci/bgp-as-notation.ci` - functional test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the `as-notation` leaf (parsed, unused) and a failing `test/ci/bgp-as-notation.ci`.
2. **Phase: Input acceptance** - shared asdot normaliser at the ASN parse sites (config + AS-path text).
   - Tests: `TestParseASNAsdot`, `TestParseASPathTextAsdot`
3. **Phase: Display notation** - thread notation mode into the ASN/AS-path formatter; render asdot/asdot+.
   - Tests: `TestASPathAppendTextNotation`
4. **Functional test** - asdot input + asdot display end to end.
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | uint32 storage unchanged; default output identical; asdot vs asdot+ boundary at 65535 |
| No leaf-package config import | `core/bgp/attribute` still imports no config |
| Registration over hardcoding | notation read from config into settings; single shared formatter, no scattered per-notation branches |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| asdot input | `go test ./internal/component/config -run Asdot` |
| notation render | `go test ./internal/core/bgp/attribute -run Notation` |
| functional | `test/ci/bgp-as-notation.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | reject out-of-range X/Y and malformed dotted tokens |

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
- [ ] AC-1..AC-7 all demonstrated
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
- [ ] Boundary tests for all numeric inputs (X, Y, combined ASN)
- [ ] Functional tests for end-to-end behavior
