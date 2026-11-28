# Spec: BGP one-off defects — 3: tests, interop, and RFC enrollment

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each item repairs a broken BGP functional `.ci` test, a broken Docker interop scenario/oracle, an unenrolled implemented draft, or missing RFC 9552 MUST coverage owned under `ai/doors/le-tooling.md` |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-bgp-one-off-defects.md`. This child owns the 15 open BGP functional test, interop scenario, and RFC-short journal rows under `ai/doors/le-tooling.md` (`test/`, `rfc/short/`, `docs/architecture/testing/interop.md`).

## Task

Repair the 15 remaining open BGP test, interop-scenario, and RFC-enrollment one-off rows after retiring already-fixed rows in the parent umbrella. Every test expectation or RFC-tag modification goes through `ze-test-judge` (`ai/META-DECISIONS.md`) so no test is weakened to clear a red. If diagnosing a broken `.ci` or interop scenario uncovers a real product defect in `internal/component/bgp/`, that product defect is routed to Child 1 (`spec-bgp-one-off-defects-1-bgp-engine.md`) or Child 2 (`spec-bgp-one-off-defects-2-bgp-plugins.md`) rather than hidden in the fixture.

Standing architectural rules applied here:
- **No test weakening (`ai/META-DECISIONS.md`)**: every change to an RFC-tagged test, fixture, or golden output is judged by `ze-test-judge`.
- **Functional test placement**: plugin and LG `.ci` tests live under `test/plugin/`, parse tests under `test/parse/`, encode tests under `test/encode/`, and Docker interop scenarios under `test/interop/scenarios/`.
- **Third-party interop patches**: any GoBGP patch for `bgp-sr-policy-gobgp` stays a diff against the upstream release applied at Docker image build (`ai/META-DECISIONS.md`), or the scenario documents the upstream API bug and uses a working route path.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, RFC compliance precedence (RFC > ExaBGP API > ExaBGP behavior)
  → Constraint: when a fixture disagrees with the RFC, fix the fixture, never the RFC-compliant product behavior.
- [ ] `ai/doors/le-tooling.md` - `.ci` functional runner, interop scenario harness, and vacuity rules
  → Constraint: every test must assert on state rather than time and fail when the behavior under test is reverted.
- [ ] `docs/architecture/testing/interop.md` - interop scenario discovery, peer oracle differences, and vacuity traps
  → Constraint: documented peer implementation quirks (such as FRR's Prefix-SID reset off labeled unicast) are recorded here rather than bending Ze away from the RFC.
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` directives and assertion semantics
  → Constraint: fixtures use valid registered family names and deterministic barriers.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8669.md` - Section 3.1 (Prefix-SID MUST be ignored on non-labeled-unicast AFI/SAFI)
- [ ] `rfc/short/rfc9552.md` - Section 5.1 (five MUST rows needing tests or recorded exclusions)
- [ ] `rfc/short/rfc2545.md` - Section 3 (RFC2545-3-2 negative proof owed outside `test/encode/new-v6.ci`)
- [ ] `rfc/drafts/draft-simpson-idr-flowspec-redirect-02.txt` - draft to enroll under `rfc/short/`

**Key insights:**
- Every file edited here lives under `test/`, `rfc/short/`, or `docs/architecture/testing/interop.md`, keeping this child strictly on `ai/doors/le-tooling.md`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `docs/architecture/testing/interop.md` - has no record of FRR's RFC 8669 §3.1 Prefix-SID reset difference (`bgp-prefix-sid-malformed-frr`).
- [ ] `rfc/short/draft-simpson-idr-flowspec-redirect-02.md` - absent (`exists: false`).
- [ ] `rfc/short/rfc9552.md` - lines 698–702 carry unchecked `[MUST]` rows (`RFC9552-5.1-1`, `RFC9552-5.1-2`, `RFC9552-5.1-3`, and siblings) without bound tests or recorded exclusions.
- [ ] `test/interop/scenarios/bgp-sr-policy-gobgp/` - affected by GoBGP `pkg/apiutil/attribute.go` field overload.
- [ ] `test/parse/redistribute-family-filter.ci`, `test/encode/new-v6.ci`, `test/plugin/bgp-gtsm-reject.ci`, `test/plugin/relay-withdraw-nexthop-self.ci`, `test/plugin/bgp-rs-fastpath.ci`, `test/plugin/rpki-decorator-merge.ci` - recorded open test/fixture defects awaiting red reproduction and repair.

**Behavior to preserve:**
- All existing passing `.ci` tests, interop scenarios, and `./le rfc check` gates stay green; no assertion is weakened.

**Behavior to change:**
- Only the 9 test/interop/RFC-enrollment ACs listed in `## Acceptance Criteria` below.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| RFC gate & summary enrollment | `./le rfc check`; `rfc/short/*.md`; `rfc/audit/*.json` | Verifies every implemented RFC/draft has requirement rows bound to positive/negative tests or recorded exclusions | Extends: AC-3 enrolls `draft-simpson-idr-flowspec-redirect-02.md`; AC-7 closes the 5 `rfc9552.md` MUST rows |
| Interop lab & peer difference record | `docs/architecture/testing/interop.md`; `test/interop/scenarios/` | Runs Ze against FRR, BIRD, and GoBGP in Docker and records peer deviations from RFCs | Extends: AC-1, AC-4, and AC-6 repair broken interop scenarios and record FRR's Prefix-SID reset |
| Functional `.ci` test runner | `internal/test/runner`; `test/{plugin,parse,encode}/*.ci` | Executes real `ze` daemon scenarios and checks state assertions | Extends: AC-2, AC-5, AC-6, and AC-8 fix broken fixtures and functional reds without weakening assertions |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order (`ai/rules/rfc-compliance.md`)" | agrees | Fixtures and interop scenarios enforce RFC compliance first (such as RFC 8669 Section 3.1 ignore behavior) and record peer quirks rather than changing Ze to match a non-compliant peer. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/encode/new-v6.ci`, `test/parse/redistribute-family-filter.ci`, `test/plugin/bgp-gtsm-reject.ci`, `test/plugin/relay-withdraw-nexthop-self.ci`, `test/plugin/bgp-rs-fastpath.ci`, `test/plugin/rpki-decorator-merge.ci` | "**A compiled Go fixture returns an error when an assertion fails** (`ai/patterns/functional-test.md`, `ai/rules/testing.md`)." | agrees | Every `.ci` and interop scenario fix asserts on observable protocol state and fails closed on unmet assertions without weakening coverage. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | a page this spec modifies | "A checker waits for readiness, asserts the protocol behaviour, verifies stability where the scenario needs it, and returns an error on failure." | agrees | AC-1, AC-4, and AC-7 keep strict checker assertions while recording FRR's RFC 8669 Section 3.1 Prefix-SID reset difference in `docs/architecture/testing/interop.md`. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Repaired BGP interop scenarios (`bgp-sr-policy-gobgp`, `bgp-prefix-sid-malformed-frr`, `bgp-nexthop-self-local-auto-frr`, `bgp-linklocal-only-multihop-withdraw-frr`, `rfc2545-joint-subnet-frr`) declare only the kernel capabilities they actually need. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test functional <suite>`, `./le test integration interop`, and `./le rfc check`.

### Transformation Path
1. Functional runner executes `.ci` scenarios against the real `ze` binary (`test/plugin/`, `test/parse/`, `test/encode/`).
2. Interop lab spins up Docker topologies (`test/interop/scenarios/`) pairing Ze with FRR or GoBGP.
3. `./le rfc check` validates `rfc/short/` requirement rows against tagged Go and `.ci` tests.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test harness ↔ `ze` binary / Docker peers | `.ci` assertions, interop topology scripts, and RFC tags | No |

### Changed-Wire Census

**Search evidence:** Searched `test/interop/scenarios/`, `test/{plugin,parse,encode}/`, `rfc/short/`, and `docs/architecture/testing/interop.md`. This child modifies only test fixtures, interop scenario definitions, RFC summary files, and `docs/architecture/testing/interop.md`; no Ze wire encoder or decoder changes here (any wire-visible bug discovered while fixing a red routes to Child 1 or Child 2).

### Integration Points
- `./le rfc check` - verifies `rfc/short/draft-simpson-idr-flowspec-redirect-02.md` and `rfc/short/rfc9552.md`.
- `ze-test-judge` - independent approval gate for every test or RFC-tag update.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | All edits stay under `test/`, `rfc/short/`, and `docs/architecture/testing/interop.md` |
| No unintended coupling (components stay isolated) | No | Single door (`ai/doors/le-tooling.md`); no product package imports touched |
| No duplicated functionality (extends existing, does not recreate) | No | Uses existing `.ci`, interop lab, and `rfc/short/` mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No hot-path runtime code touched |
| Registration over hardcoding, outbound | Yes | Interop scenarios and RFC summaries are discovered automatically |
| Registration over hardcoding, inbound | Yes | No central runner list hardcoded |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each listed `.ci` or interop failure is either a fixture/oracle defect or reproduces a live product bug | Recorded in `plan/journal/` rows owned here | Route any newly exposed product bug to Child 1 or Child 2 rather than masking it in the test | Scoped test run by main thread | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Updating a `.ci` fixture or RFC tag accidentally lowers what the test proves | `ze-test-judge` rejects the diff | Submit every test or RFC-tag edit to `ze-test-judge` and prove the test fails when the underlying behavior is reverted |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | False green or flaky functional/interop/RFC gates |
| How is it reverted? | Single commit revert of test/doc files |
| Who else touches this path? | Child 1 (`spec-bgp-one-off-defects-1-bgp-engine.md`) and Child 2 (`spec-bgp-one-off-defects-2-bgp-plugins.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` on `rfc/short/` | → | `draft-simpson-idr-flowspec-redirect-02.md` and `rfc9552.md` | `./le rfc check` passes with zero unenrolled or untested MUST rows |
| `./le test functional` on repaired `.ci` cases | → | `test/plugin/*.ci`, `test/parse/*.ci` | `bgp-gtsm-reject.ci`, `relay-withdraw-nexthop-self.ci`, `bgp-rs-fastpath.ci`, `rpki-decorator-merge.ci`, `redistribute-family-filter.ci` green |
| `./le test integration interop` on repaired scenarios | → | `test/interop/scenarios/` | `bgp-sr-policy-gobgp`, `bgp-linklocal-only-multihop-withdraw-frr`, `rfc2545-joint-subnet-frr`, `bgp-nexthop-self-local-auto-frr`, labeled-withdraw green |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `bgp-sr-policy-gobgp` interop scenario (`test/interop/scenarios/bgp-sr-policy-gobgp/`, old AC-11) | Scenario passes cleanly; any GoBGP fix is kept as a diff against the upstream release applied at image build, or the scenario avoids the broken GoBGP API with the reason recorded | Upstream GoBGP release remains unvendored in the Ze repository |
| AC-2 | `test/parse/redistribute-family-filter.ci` uses unregistered/impossible family spelling (old AC-12 `.ci` half) | Fixture encodes only registered family names allowed by the schema and RFC, and passes `./le test functional parse` | Valid family filters in the scenario remain tested end-to-end |
| AC-3 | Implemented FlowSpec redirect draft (`rfc/drafts/draft-simpson-idr-flowspec-redirect-02.txt`, old AC-20) | `rfc/short/draft-simpson-idr-flowspec-redirect-02.md` exists with requirement rows bound to existing tests and `./le rfc check` passes | Existing FlowSpec redirect tests remain intact and strengthened only where needed |
| AC-4 | FRR Prefix-SID interop difference (`bgp-prefix-sid-malformed-frr`, old AC-28) | Ze's RFC 8669 Section 3.1 ignore behavior off labeled unicast is preserved and FRR's session reset is documented as an interop difference in `docs/architecture/testing/interop.md` | Ze never bends away from RFC 8669 §3.1 MUST-ignore to imitate FRR |
| AC-5 | `test/encode/new-v6.ci` unprovable `RFC2545-3-2` negative tag (journal 2026-10-31) | The unprovable tag is removed from `new-v6.ci` (via `ze-test-judge`) and replaced by a real negative proof bound in `rfc/short/rfc2545.md` | Positive IPv6 encoding assertions in `new-v6.ci` remain intact |
| AC-6 | Broken BGP functional `.ci` tests (`bgp-gtsm-reject.ci`, `relay-withdraw-nexthop-self.ci`, draft `rib-rail-two-octet-source.ci`, `bgp-rib` as relay, old AC-42 `.ci` rows) | Each scenario runs green with its root cause named in the commit (and any product defect routed to Child 1 or Child 2) | Every test asserts on live protocol state without time-based sleeps or weakened assertions |
| AC-7 | Broken BGP Docker interop scenarios (`bgp-nexthop-self-local-auto-frr` AIGP, `bgp-linklocal-only-multihop-withdraw-frr` `File exists` route setup, `rfc2545-joint-subnet-frr` `frrvty` group, labeled-withdraw FRR OPENSENT FSM Error, old AC-42 interop + 2026-10-26 rows) | Each interop topology sets up cleanly and passes against FRR with root cause recorded | Peer oracle configs remain standard FRR behavior |
| AC-8 | `rfc/short/rfc9552.md` five MUST rows without tests (old AC-48) | Each of the five MUST rows carries a bound positive/negative test or a recorded exclusion, and `./le rfc check` passes | Existing passing RFC 9552 rows remain green |
| AC-9 | Plugin `.ci` export reds (`test/plugin/bgp-rs-fastpath.ci`, `test/plugin/rpki-decorator-merge.ci`, old AC-54) | Both tests pass cleanly in the main checkout with root cause named | All fast-path and RPKI decorator merge assertions remain at full strength |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `./le rfc check` to audit RFC coverage | `le` → `rfc/short/` → tagged tests | `./le rfc check` green for RFC 9552, RFC 2545, and `draft-simpson-idr-flowspec-redirect-02` |
| 2 | Runs the repaired BGP functional and interop scenarios | `le` → `test/plugin/`, `test/parse/`, `test/interop/scenarios/` | Named `.ci` and interop scenarios green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| RFC gate verification | `rfc/short/draft-simpson-idr-flowspec-redirect-02.md`, `rfc/short/rfc9552.md` | `./le rfc check` passes with zero missing MUST proofs | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (test/interop/RFC enrollment scope) | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-family-filter.ci` | `test/parse/redistribute-family-filter.ci` | Valid registered family filter parses cleanly | |
| `bgp-gtsm-reject.ci`, `relay-withdraw-nexthop-self.ci`, `bgp-rs-fastpath.ci`, `rpki-decorator-merge.ci` | `test/plugin/` | Functional plugin suites pass reliably | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-sr-policy-gobgp` | `test/interop/scenarios/bgp-sr-policy-gobgp/` | GoBGP | SR Policy exchange passes cleanly | |
| `bgp-prefix-sid-malformed-frr` | `test/interop/scenarios/bgp-prefix-sid-malformed-frr/` | FRR | Ze's RFC 8669 §3.1 ignore vs FRR reset documented | |
| `bgp-nexthop-self-local-auto-frr`, `bgp-linklocal-only-multihop-withdraw-frr`, `rfc2545-joint-subnet-frr` | `test/interop/scenarios/` | FRR | AIGP, link-local multihop withdraw, and RFC 2545 joint-subnet pass | |

## Files to Modify
- `docs/architecture/testing/interop.md` - record FRR RFC 8669 §3.1 Prefix-SID reset difference (`bgp-prefix-sid-malformed-frr`)
- `rfc/short/rfc9552.md` - bind tests or recorded exclusions for the five untested MUST rows
- `rfc/short/rfc2545.md` - update `RFC2545-3-2` negative proof binding
- `test/encode/new-v6.ci` - remove unprovable `RFC2545-3-2` negative tag (via `ze-test-judge`)
- `test/parse/redistribute-family-filter.ci` - replace impossible family name with registered family
- `test/plugin/bgp-gtsm-reject.ci` - repair broken test path
- `test/plugin/relay-withdraw-nexthop-self.ci` - repair broken test path
- `test/plugin/bgp-rs-fastpath.ci` - repair export red
- `test/plugin/rpki-decorator-merge.ci` - repair export red

## Files to Create
- `rfc/short/draft-simpson-idr-flowspec-redirect-02.md` - enroll FlowSpec redirect draft requirement rows

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test, interop, and RFC summary spec only |
| YANG validation constraints | N-A | No schema changes |
| YANG custom validators | N-A | No schema changes |
| CLI commands/flags | N-A | No CLI changes |
| CLI grammar (keyword before value) | N-A | No CLI changes |
| Editor autocomplete | N-A | No schema changes |
| Functional test for new RPC/API | Yes | Repairs existing `.ci` tests under `test/plugin/`, `test/parse/`, `test/encode/` |
| Pipe completeness | N-A | No command output changes |
| Env var registration | N-A | No env var changes |
| Doctor check for runtime dependencies | N-A | No runtime dependencies added |
| Doctor code per problem class | N-A | No new log or report-bus sites |
| Prometheus counters/metrics | N-A | No metric changes |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Test and RFC enrollment fixes only |
| 2 | Config syntax changed? | No | No config change |
| 3 | CLI command added/changed? | No | No CLI change |
| 4 | API/RPC added/changed? | No | No API change |
| 5 | Plugin added/changed? | No | No plugin change |
| 6 | Has a user guide page? | No | No guide change |
| 7 | Wire format changed? | No | No wire change |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-simpson-idr-flowspec-redirect-02.md`, `rfc/short/rfc9552.md`, `rfc/short/rfc2545.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (FRR Prefix-SID interop difference) |
| 11 | Affects daemon comparison? | No | No comparison change |
| 12 | Internal architecture changed? | No | No architecture change |
| 13 | Route metadata keys added/changed? | No | No metadata change |
| 14 | Prometheus counters added/changed? | No | No metric change |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration change |
| 16 | Any changed source file referenced by existing doc source anchors? | No | Verified via `./le spec citation anchors spec plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config/CLI/API examples affected |

## Design presentation

### What changes

This child spec repairs the 15 remaining open BGP functional test, Docker interop scenario, and RFC-enrollment defects under `ai/doors/le-tooling.md`: enrolling `rfc/short/draft-simpson-idr-flowspec-redirect-02.md`, binding tests or exclusions for the five untested RFC 9552 MUST rows, recording FRR's RFC 8669 §3.1 Prefix-SID reset difference in `docs/architecture/testing/interop.md`, replacing the unprovable `RFC2545-3-2` tag on `new-v6.ci`, and fixing the listed `.ci` and FRR/GoBGP interop scenarios without weakening assertions.

### Before and after

1. **FlowSpec redirect draft enrollment (AC-3):** Before, `draft-simpson-idr-flowspec-redirect-02` is implemented in Ze but absent from `rfc/short/`. After, `rfc/short/draft-simpson-idr-flowspec-redirect-02.md` binds every requirement row to its tests and `./le rfc check` passes.
2. **FRR Prefix-SID interop difference (AC-4):** Before, `bgp-prefix-sid-malformed-frr` failed because FRR resets the session where RFC 8669 §3.1 requires ignoring Prefix-SID off labeled unicast. After, Ze keeps RFC 8669 §3.1 compliance and `docs/architecture/testing/interop.md` documents FRR's behavior.
3. **Broken `.ci` and interop fixtures (AC-1, AC-2, AC-6, AC-7, AC-9):** Before, fixture setup issues (`File exists`, `frrvty` group, invalid family name, GoBGP API overload) left release scenarios red. After, each scenario sets up cleanly and proves real protocol state.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Change Ze to reset on Prefix-SID off labeled unicast to match FRR | Violates RFC 8669 Section 3.1 ("It MUST be ignored") and `ai/doors/system.md` RFC-first precedence |
| Delete or weaken failing `.ci` tests | Violates `ai/rules/testing.md` and `ai/META-DECISIONS.md` (`ze-test-judge` gate) |
| Vendor a forked GoBGP binary into the repository | Violates `ai/META-DECISIONS.md` (keep GoBGP fixes as an image-build diff against the upstream release) |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** — run each failing `.ci`, interop scenario, and `./le rfc check` via the main thread to confirm live failures at HEAD.
2. **Phase: RFC enrollment & interop documentation (AC-3, AC-4, AC-5, AC-8)** — create `rfc/short/draft-simpson-idr-flowspec-redirect-02.md`, close the 5 `rfc/short/rfc9552.md` MUST rows, replace the unprovable `RFC2545-3-2` tag (via `ze-test-judge`), and record FRR's Prefix-SID reset in `docs/architecture/testing/interop.md`.
3. **Phase: Functional `.ci` and interop scenario repairs (AC-1, AC-2, AC-6, AC-7, AC-9)** — repair each `.ci` and Docker interop scenario, routing any newly exposed product bug to Child 1 or Child 2, and verifying green through the main thread.

## Work Not Done

- BGP engine defects (`ai/doors/bgp-engine.md`) are owned by `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md`.
- BGP plugin defects (`ai/doors/bgp-plugins.md`) are owned by `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md`.
- Already-fixed defects (33 journal rows) are retired in `plan/release/fixes/spec-bgp-one-off-defects.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `reference-checked-claim-unchecked` | 2026-10-31 | new-v6.ci RFC2545-3-2 tag unprovable; proof owed in an attribute unit test |
| `test-against-broken-path` | 2026-10-26 | `bgp-linklocal-only-multihop-withdraw-frr` route setup `File exists` |
| `test-against-broken-path` | 2026-10-26 | FRR oracle fixture `rfc2545-joint-subnet-frr`: zebra's root not in vty group frrvty |
| `field-carries-two-meanings` | 2026-10-20 | GoBGP `pkg/apiutil/attribute.go` |
| `fixture-encodes-an-impossible-state` | 2026-09-06 | `test/parse/redistribute-family-filter.ci` |
| `implemented-draft-not-enrolled` | 2026-10-21 | `draft-simpson-idr-flowspec-redirect-02` |
| `requirement-applied-outside-the-case-it-names` | 2026-10-20 | `bgp-prefix-sid-malformed-frr` |
| `concurrent-rfc-gate-stale` | 2026-08-25 | `rfc/short/rfc9552.md` five MUST rows with no test |
| `gate-red-where-nothing-blocks-on-it` | 2026-10-10 | `bgp-rs-fastpath.ci`, `rpki-decorator-merge.ci` red in an export |
| `test-against-broken-path` | 2026-08-10 | `test/plugin/bgp-gtsm-reject.ci` times out |
| `test-against-broken-path` | 2026-08-15 | `test/plugin/relay-withdraw-nexthop-self.ci` |
| `test-against-broken-path` | 2026-09-09 | `bgp-rib` as a relay |
| `test-against-broken-path` | 2026-10-06 | `bgp-nexthop-self-local-auto-frr` AIGP |
| `test-against-broken-path` | 2026-10-08 | draft `rib-rail-two-octet-source.ci` |
| `session-never-establishes-with-the-peer` | 2026-10-26 | FRR answers FSM Error in OPENSENT, labeled-withdraw scenario |

## Review Gate

Pending implementation and independent review.
