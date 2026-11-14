# Spec: bgp-mp-attribute-first-wire-proof -- the daemon and interop proof that the MP attribute leaves first

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P2 |
| Release reason | RFC 7606 Section 5.1 is met and recorded at unit level; the proof through the daemon and against another implementation is still owed. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of spec-bgp-unowned-rfc-rows on 2026-11-14 (owner: "split
bgp-unowned-rfc-rows"). Its row 10, `RFC7606-5.1-1`, is implemented: every
builder writes MP_REACH_NLRI or MP_UNREACH_NLRI as the first path attribute.
The row in `rfc/short/rfc7606.md` carries no annotation, nine tagged units
prove it in both polarities, and
`rfc/discrimination/rfc7606/RFC7606-5.1-1.json` records their breaks.

What that spec promised for the row and never built is the proof outside unit
tests. A wire-visible protocol change owes an interop test
(`ai/rules/interop-and-goal-validation.md`), and work is done only when a test
reaches the behavior through the real entry point (`ai/rules/principles.md`).
This spec owns those proofs. It changes no product code unless a proof goes red.

| Owed | Why it is owed |
|------|----------------|
| Interop scenario `bgp-mp-reach-first-frr` | No scenario under `test/interop/scenarios/` asserts the position of the MP attribute in an UPDATE Ze sends. FRR accepts the attribute in any position, so "routes accepted" in the existing IPv6 scenarios does not discriminate |
| Interop scenario `bgp-mp-reach-first-bird` | The same against BIRD |
| `test/plugin/rfc7606-mp-reach-first.ci` | The wiring row "any MP announcement from config" named this test and it does not exist. Many exact-hex `.ci` files changed with the fix, but none was shown to be the proof of this row |
| One test feeding the same route through every writer and comparing bytes (`TestEveryBuilderSameBytesMPFirst`) | Risk R-2 of the source spec: the builders desynchronise again. No landed test of that name exists; the audit names the landed test that proves it, or it is written |

### The RFC text, quoted from the source

| Quote | Read at |
|-------|---------|
| "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if present) SHALL be encoded as the very first path attribute in an UPDATE message." | `rfc/full/rfc7606.txt` §5.1 |

### How FRR and BIRD handle it

| FRR | BIRD |
|-----|------|
| `bgp_packet_attribute` (`bgpd/bgp_attr.c` line 4950) writes MP_REACH (`bgp_packet_mpattr_start`, line 4976) before ORIGIN (line 4988) | `bgp_create_mp_reach` (`proto/bgp/packets.c` line 2895) writes MP_REACH first, then copies the other attributes after it |

Both line references were read on 2026-10-11 and are re-read at the version the
lab runs before a check is written.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/mp-nlri-ordering.md` - the page that states Ze's MP attribute order
  → Constraint: the page quotes RFC 7606 Section 5.1 and states MP first; the proofs assert what the page says
- [ ] `docs/architecture/testing/interop.md` - scenario naming, native packet observation, the revert-and-red walk
  → Constraint: every new scenario is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt
- [ ] `docs/architecture/wire/attributes.md` - declared by `internal/core/bgp/attribute/origin.go`
  → Constraint: unaffected by this spec, which adds proofs only

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7606.md` - row `RFC7606-5.1-1`
  → Constraint: an interop checker that carries an `RFC requirement: RFC7606-5.1-1` tag is a tagged unit and owes a discrimination record in the same change

**Key insights:**
- The code is in place and proven at unit level. This spec adds the daemon and interop proofs.
- The interop assertion is on a capture of what Ze sent, never on the peer's table.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/bgp/attribute/origin.go` - `OrderAttributes` returns MP_UNREACH_NLRI, then MP_REACH_NLRI, then every other attribute by ascending type code; its comment quotes RFC 7606 Section 5.1 and names the writers that reach the order
- [ ] `internal/le/interoplab/bgp/check_special.go` - `specialCheckers` binds a scenario name to its checker
- [ ] `internal/le/interoplab/bgp/check_packet_capture.go` - `packetCapture` runs a bounded tcpdump inside the lab-owned Ze container, started before Ze through `prestart`

**Behavior to preserve:**
- Everything: this spec adds tests.

**Behavior to change:** None - preserve all existing behavior.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Bounded native packet capture in an interop lab | `internal/le/interoplab/bgp/check_packet_capture.go`, `packetCapture` | Captures what the Ze container sends and receives, with an explicit time and packet bound | Extends by use: the two scenarios capture TCP port 179 and the checker reads the first path attribute of each UPDATE Ze sent |
| Scenario-to-checker binding | `internal/le/interoplab/bgp/check_special.go`, `specialCheckers` | Maps a scenario directory name to the Go checker that judges it | Extends: two entries, no second map |
| Exact-hex functional test | `test/plugin/*.ci` with a ze-peer expectation (`ai/patterns/functional-test.md`) | Asserts the bytes Ze sends to a test peer through the real daemon | Extends by use: `rfc7606-mp-reach-first.ci` is one more such test |
| Tagged units and their discrimination record | `rfc/discrimination/rfc7606/RFC7606-5.1-1.json` | Records the break under which each tagged unit goes red | Extends: a tagged interop checker joins the record through `./le rfc record` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: an IPv6 unicast route announced by Ze to a peer.

### Transformation Path
1. Config route to the UPDATE builders, `attribute.OrderAttributes` and `WriteAttributesOrdered`.
2. The UPDATE on the wire to FRR, BIRD or the test peer.
3. The capture, read by the checker; the `.ci` expectation, compared by the test peer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to a peer daemon | BGP UPDATE on TCP 179 | No |

### Integration Points
- `specialCheckers` - the two scenario bindings.
- `packetCapture` - the capture the checker reads.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | tests only |
| No duplicated functionality (extends existing, does not recreate) | No | the capture helper and the checker map are reused |
| Zero-copy preserved where applicable (refs, not copies) | No | not applicable, no product code |
| Registration over hardcoding, outbound | No | a scenario is discovered by its directory name |
| Registration over hardcoding, inbound | No | to be shown at implementation |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-3 | Ze never emits MP_REACH and MP_UNREACH in one UPDATE | RFC 7606 §5.1 second bullet forbids it | "Very first" has two candidates and a second §5.1 defect exists, which gets a journal row | grep the builders, a unit test over a mixed batch | holds by source reading (2026-10-11): each send path emits at most one MP attribute per UPDATE |
| A-8 | The lab's Ze container can capture its own BGP traffic with `packetCapture` in an FRR and a BIRD scenario | `check_gobgp_reload_wire.go` and `check_delayopen.go` use it with filter "ip and tcp port 179" | The checker reads the peer's debug log of the received UPDATE instead, after its print function is read in the peer's source | Run the scenario once | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-2 | The builders desynchronise again | two builders emitting different bytes for one route | One test feeds the same route through every builder and compares bytes |
| R-9 | A proof goes red because a writer still places the MP attribute late | the `.ci` or the capture shows ORIGIN first | It is a product defect in the row this spec proves: fix the writer here, with its unit test and record |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: a missing proof, or a vacuous one |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md` originates BGP-LS UPDATEs whose byte order follows the same rule |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Any MP announcement from config | → | `OrderAttributes` and every builder | `test/plugin/rfc7606-mp-reach-first.ci` |

## Acceptance Criteria

AC-7 is the number this criterion had in spec-bgp-unowned-rfc-rows.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-7 (row 10) | Ze sends any UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI, from any writer: `OrderAttributes`, `packAttributesOrderedInto` and the family builders on it, `buildRIBRouteUpdate`, `buildWireModeUpdate`, the splitter, the forward merge-insert, and a forward of a received UPDATE whose MP attribute was not first | That attribute is the first path attribute on the wire; the same route through every builder yields identical bytes; `OrderAttributes` given a list holding both MP attributes still puts one first; a forward of a source that already complies stays byte-identical. Landed and recorded at unit level; owed here: the `.ci` through the daemon, the two interop scenarios, and the every-builder byte comparison |
| AC-10 | The two interop scenarios, with the ordering fix reverted and the image rebuilt | Each goes red on the capture assertion, and the red is recorded; restored, each is green |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 3 | Sends IPv6 routes to FRR | config -> builder -> wire with MP_REACH first | `bgp-mp-reach-first-frr` |
| 4 | Sends IPv6 routes to BIRD | the same | `bgp-mp-reach-first-bird` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7606Section51MPAttributeEncodedFirst` | `internal/component/bgp/message/rfc7606_mp_first_test.go` | AC-7: BuildUnicast (IPv6), BuildVPN and BuildLabeledUnicast | landed, recorded |
| `TestOrderAttributesMPPlacement` (the plan named `TestOrderAttributesBothMPAttributes`) | `internal/core/bgp/attribute/attribute_test.go` | AC-7: a list holding both MP attributes | landed; the audit confirms it asserts one MP attribute first |
| `TestSplitMP_PreservesAscendingAttributeOrder`, `TestBuildUpdatePayloadMPAttributeFirst` (the plan named `TestSplitterEmitsMPFirst`) | `internal/component/bgp/message/rfc4271_update_split_attr_order_test.go`, `internal/component/bgp/wireu/rfc7606_split_mp_first_test.go` | AC-7 splitter | landed, recorded |
| `TestForwardMovesTrailingMPAttributeFirstSameContext`, `TestForwardMovesTrailingMPAttributeFirstAcrossContexts`, `TestForwardRebuildMovesSourcedMPAttributeFirst`, `TestForwardLeadingMPAttributeKeepsZeroCopy` (the plan named `TestForwardMovesLateMPFirst`) | `internal/component/bgp/reactor/rfc7606_forward_mp_first_test.go` | AC-7: a source with MP late is moved; a compliant source stays byte-identical | landed, recorded |
| `TestEgressOverrideMovesTrailingMPAttributeFirst`, `TestRelayPayloadLeadsWithMPReach` | `internal/component/bgp/reactor/rfc7606_egress_override_shape_test.go`, `rfc7606_relay_payload_test.go` | AC-7: the export-filter override and the stored-route relay | landed, recorded |
| `TestEveryBuilderSameBytesMPFirst` | `internal/component/bgp/reactor/` | AC-7, R-2, over every writer AC-7 lists | owed, unless the audit names the landed test that proves it |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-mp-reach-first` | `test/plugin/` | AC-7: a configured IPv6 route leaves with MP_REACH_NLRI as the first path attribute, asserted on the bytes | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-mp-reach-first-frr` | `test/interop/scenarios/` | FRR | FRR receives Ze's IPv6 UPDATE with MP_REACH as the first attribute (capture asserted), routes accepted | |
| `bgp-mp-reach-first-bird` | same | BIRD | the same against BIRD | |

Every scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify
- `internal/le/interoplab/bgp/check_special.go` - the `specialCheckers` entries binding `bgp-mp-reach-first-frr` and `bgp-mp-reach-first-bird` to their checker
- `rfc/discrimination/rfc7606/RFC7606-5.1-1.json` - through `./le rfc record`, when the checker carries the tag

## Files to Create
- `test/plugin/rfc7606-mp-reach-first.ci`
- `test/interop/scenarios/bgp-mp-reach-first-frr/`, `test/interop/scenarios/bgp-mp-reach-first-bird/`
- one checker file and its `_test.go` under `internal/le/interoplab/bgp/` (the capture asserts the first path attribute is type 14 or 15)
- the every-builder byte comparison test, if the audit finds none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tests only |
| YANG validation constraints | N-A | tests only |
| YANG custom validators | N-A | tests only |
| CLI commands/flags | N-A | tests only |
| CLI grammar (keyword before value) | N-A | tests only |
| Editor autocomplete | N-A | tests only |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Doctor code per problem class | N-A | no new problem class |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the `features/<id>.md` declaration whose evidence lists the RFC 7606 proofs, if `./le feature report` shows the interop scenario moves its level |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | `docs/architecture/wire/mp-nlri-ordering.md` already states MP first |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | the discrimination record, and the generated `docs/features/rfc-status.md` (`./le rfc update`) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, if the checker adds a capture reader other scenarios can use |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/interop.md` is declared by `check_special.go` and `check_packet_capture.go`: affected only by item 10. `docs/architecture/wire/attributes.md` and `rfc/short/rfc4271.md` are declared by `origin.go`, which this spec reads and does not change: unaffected. Re-derive with `./le spec citation anchors spec plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none |

## Implementation Steps

1. **Phase: Wiring** -- `rfc7606-mp-reach-first.ci` written; it is expected green, so its red is forced by reverting the order in `OrderAttributes` and recorded.
2. **Phase: audit** -- name the landed test that proves the every-builder byte comparison, or write it.
3. **Phase: interop** -- the two scenarios and their checker, each with the revert, rebuild, red, restore, green walk; the tag and its record.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-7's four owed proofs each exist and each has a recorded red |
| Correctness | The checker reads UPDATEs Ze sent, not ones it received, and fails when it read none |
| Rule: interop-and-goal-validation | No assertion rests on the peer accepting the route |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The `.ci` exists and asserts bytes | `ls test/plugin/rfc7606-mp-reach-first.ci`, then read it |
| Both scenarios exist and are bound | `ls test/interop/scenarios/ \| grep mp-reach-first`, `grep -n mp-reach-first internal/le/interoplab/bgp/check_special.go` |
| Ledger clean | `./le rfc check` exit status for `rfc7606` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The capture reader bounds every length it reads from the pcap |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The interop assertion is on a capture of what Ze sent | Assert that the peer accepted the routes | FRR and BIRD accept the MP attribute in any position (RFC 7606 §5.1 tells receivers to), so acceptance cannot go red when the order is reverted |
| The proofs are a spec of their own in `release/validation/` | Close the row with unit proof alone | A wire-visible change owes an interop test, and the row's own plan named these tests; the owner ordered the split so the proven work could close without them being dropped |

## Known Limitations
- None: every owed proof is in scope.

## RFC Documentation (Scope: protocol)

The checker quotes RFC 7606 Section 5.1 above the assertion and carries the
`RFC requirement: RFC7606-5.1-1` tag with the claim its body checks, no wider.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-7 and AC-10 demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run in the main checkout by the main thread: `./le test functional plugin` for `rfc7606-mp-reach-first`, `./le test unit pkg path ./internal/le/interoplab/bgp/`, and `./le test integration interop` for the two scenarios
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-bgp-mp-attribute-first-wire-proof.md` only, in the same `./le commit create` script
