# Interop Testing and Goal Validation

**BLOCKING.** Protocol features MUST have interop tests. All features MUST have
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
`/ze-implement` step 11 (Deliverables review) and verified during `/ze-review`.

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
