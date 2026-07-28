# Interop Testing and Goal Validation

**When:** implementing or changing protocol behavior, and when validating that a spec's stated goals are met
**Severity:** blocking

## Directives

Protocol features MUST have interop tests. All features MUST have
goal validation proving the feature achieves its intended purpose, not just that
the code runs without error.

## Interop Testing (protocol features)

When a spec implements or modifies protocol behavior (BGP, IPsec, L2TP, or any
wire protocol), an interop test MUST prove ze works correctly with at least one
other implementation.

### Required interop test by protocol

| Protocol area | Test infrastructure | Directory | Make target |
|---------------|--------------------|-----------|----|
| BGP (session, capability, NLRI, community, policy) | Docker: FRR, BIRD, GoBGP | `test/interop/scenarios/` | `make ze-interop-test` |
| IPsec (IKEv2, EAP, MOBIKE) | Docker: strongSwan | `test/ipsec-interop/` | `make ze-ipsec-interop-test` |
| L2TP | Docker | `test/l2tp-interop/` | (L2TP runner) |
| PPPoE (Ze as client) | Docker: accel-ppp | `test/pppoe-interop/` | `make ze-deployment-pppoe-accel-docker-test` |

### What must be tested

| Feature type | Interop assertion |
|-------------|-------------------|
| New address family / NLRI | Routes exchanged and installed by peer daemon |
| New capability | Capability negotiated, verified in peer's neighbor output |
| Session behavior (GR, route refresh) | Session survives the event, peer confirms expected behavior |
| Policy (community, filter, role) | Peer receives/rejects routes per the policy |
| Wire format change | Peer accepts the message, no NOTIFICATION |
| Authentication (MD5, EAP, PSK) | Session authenticates, handshake completes |

### When interop tests are NOT required

| Condition | Why |
|-----------|-----|
| Pure internal refactor, no wire-visible change | Existing interop tests cover the path |
| Config-only feature (no protocol impact) | CLI/config tests suffice |
| Tooling (ze-analyse, ze-perf) | No protocol peer involved |

### Interop scenario structure

Each scenario in `test/interop/scenarios/` follows the established pattern:
- `ze.conf`: ze configuration for the scenario
- `<peer>.conf`: peer daemon configuration (frr.conf, bird.conf, etc.)
- `check.py`: Python script with a `check()` function that asserts the expected behavior

The `check.py` MUST:
1. Wait for session establishment (`wait_session`)
2. Assert the specific protocol behavior being tested (route presence, capability negotiation, etc.)
3. Verify session stability after the exchange (`session_established`)
4. Use `log_pass`/`log_fail` for clear output
5. Raise on failure (AssertionError or RuntimeError)

## Prove the test discriminates (BLOCKING)

A passing interop or functional test is evidence only if it would FAIL when the
behaviour under test is broken. A test that passes whether or not the fix is
present proves nothing and must never be presented as evidence.

**Before claiming an interop/functional test validates a change, revert the
change and confirm the test goes RED.** Rebuild the artifact the test drives
(the container image, the daemon binary) so the revert actually takes effect,
then restore the fix and confirm GREEN again. Record the RED result.

This is not the same as TDD's red-then-green: a test added to ALREADY-WORKING
code (a regression test, an interop scenario for existing behaviour) never had a
red phase, so its discrimination is unproven until you force one.

| Vacuity trap | Why it passes anyway | The tell |
|--------------|----------------------|----------|
| An interop test for a sender-side wire change whose receiver must accept any form (e.g. RFC 7606 Section 5.1: receivers accept any field combination) | a conforming peer accepts the old and new wire equally | reverting the sender change leaves the peer's routing table identical |
| A test asserting the ABSENCE of something (no log line, no allocation, no route) | deleting the mechanism leaves the same absence | ask "what would still be absent if the code were removed?" |
| A test whose fixture is at an extreme (all-fields-set, max value) | an off-by-one or partial break still handles the extreme | boundary the fixture, test one-below and one-above |
| A functional test whose data reaches the peer by a DIFFERENT path than the one changed | the unchanged path still delivers | trace which code path actually produces the asserted bytes |

When a change genuinely cannot be discriminated by the peer (the receiver is
required to accept both forms), say so explicitly in Goal Validation and move the
discrimination to unit/mutation tests that CAN fail. An interop test in that case
proves ACCEPTANCE, not correctness of the specific form — state which.

## Goal Validation (all features)

Every spec has acceptance criteria (AC-1..AC-N). Tests prove each AC passes. But
"tests pass" does not mean "the feature achieves its goal." Goal validation is the
bridge between individual AC assertions and the feature's intended purpose.

### The rule

Before claiming a feature is done, answer for each spec goal:

**"What concrete evidence proves this goal is achieved, beyond individual test assertions?"**

| Goal type | Required evidence |
|-----------|-------------------|
| Protocol interop ("ze speaks X with Y") | Interop test passes with the named peer daemon |
| Performance ("handles N updates/sec") | `ze-perf` benchmark result pasted |
| User workflow ("user can do X via CLI") | Functional `.ci` test exercising the full workflow, or `.et` test for editor workflows |
| Data correctness ("routes installed correctly") | Functional test with explicit data assertions (hex match, JSON field match), not just exit code 0 |
| Resilience ("survives X failure") | Chaos test or fault-injection scenario |
| Security ("rejects unauthorized X") | Negative test: unauthorized attempt fails with expected error |

### What goal validation is NOT

| Not this | This instead |
|----------|-------------|
| "All tests pass" | "Here is the specific evidence for each goal" |
| "AC-1 through AC-5 implemented" | "AC-1 through AC-5 implemented, and together they prove [goal]" |
| "I tested it manually" | Automated test that can be re-run |
| "The code looks correct" | Observable behavior matches the spec's stated purpose |

### Where goal validation goes in the spec

The spec template has a **Goal Validation** section (added below the Implementation Audit).
Each row maps a stated goal to the evidence that proves it. This section is filled during
`/ze-close` step 1 (Deliverables review) and verified during `/ze-review`.

## Mechanical Check

### Interop check (protocol specs)

```
# For each protocol feature in the spec, verify an interop scenario exists:
ls test/interop/scenarios/*<feature-keyword>*/ 2>/dev/null
ls test/ipsec-interop/scenarios/*<feature-keyword>*/ 2>/dev/null
```

If no matching scenario exists, one must be created before claiming done.

### Goal validation check (all specs)

The spec's Goal Validation table must have:
- One row per stated goal (from the Task section)
- Evidence column filled with a concrete reference (test name, file path, command output)
- No empty evidence cells

## Relationship to Other Rules

- `functional-test-gate.md`: requires functional tests per feature type; this rule adds interop on top for protocol features
- `no-partial-completion.md`: requires every AC tested; this rule requires the *aggregate* goal proven
- `rfc-compliance.md`: requires RFC conformance in code; this rule requires conformance proven against other implementations
- `testing.md`: test infrastructure and workflow; this rule specifies when each test type is mandatory
