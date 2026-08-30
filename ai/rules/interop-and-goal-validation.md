# Interop Testing and Goal Validation

**When:** implementing or changing protocol behavior, and when validating that a spec's stated goals are met
**Severity:** blocking

## Directives

**A protocol feature MUST have an interop test. Every feature MUST have goal validation proving it achieves its intended purpose, not merely that the code runs without error.**

## Interop Testing (protocol features)

**When a spec implements or changes protocol behavior (BGP, IPsec, L2TP, PPPoE, or any wire protocol), an interop test MUST prove Ze works correctly with at least one other implementation.** The suites, their scenario directories, their native actions, and how a scenario is discovered and checked are in `docs/architecture/testing/interop.md`.

**Each feature type MUST prove the interop assertion its row names:**

| Feature type | Interop assertion |
|-------------|-------------------|
| New address family or NLRI | Routes exchanged and installed by the peer daemon |
| New capability | Capability negotiated, verified in the peer's neighbor output |
| Session behavior (GR, route refresh) | Session survives the event, and the peer confirms the expected behavior |
| Policy (community, filter, role) | Peer receives or rejects routes per the policy |
| Wire format change | Peer accepts the message, no NOTIFICATION |
| Authentication (MD5, EAP, PSK) | Session authenticates, handshake completes |

**An interop test MAY be omitted only in these three conditions:**

| Condition | Why |
|-----------|-----|
| A pure internal refactor with no wire-visible change | The existing interop tests cover the path |
| A config-only feature with no protocol impact | CLI and config tests suffice |
| Tooling (`ze-analyse`, `ze-perf`) | No protocol peer is involved |

**An interop scenario directory MUST be NAMED and MUST NOT carry a numeric prefix, and a spec planning a future scenario MUST name it too.** The directory name is the scenario's identity: `interoplab.Discover` matches it exactly, the native `./le integration` action takes it as a scenario selector, and specs, journal rows and code comments cite it.
**A number goes stale in two ways a name cannot.** A deleted scenario leaves a hole no reader can tell from a reservation, and a planned number is a reservation a second spec can take, which nothing detects because neither directory exists yet.

## Prove the test discriminates (BLOCKING)

**A passing interop or functional test is evidence only if it would FAIL when the behavior under test is broken. A test that passes whether or not the fix is present MUST NOT be presented as evidence.**
**A test added to ALREADY-WORKING code never had a red phase, so its discrimination is unproven until you force one.** This is not TDD's red-then-green: a regression test and an interop scenario for existing behavior both start green.

**Before claiming an interop/functional test validates a change, MUST revert the
change and confirm the test goes RED.** MUST rebuild the artifact the test drives
(the container image, the daemon binary) so the revert actually takes effect,
then restore the fix and confirm GREEN again. MUST record the RED result.

**Each trap below MUST be checked for by its tell before a test is called evidence:**

| Vacuity trap | Why it passes anyway | The tell |
|--------------|----------------------|----------|
| An interop test for a sender-side wire change whose receiver is obliged to accept any form (RFC 7606 Section 5.1: receivers accept any field combination) | A conforming peer accepts the old and new wire equally | Reverting the sender change leaves the peer's routing table identical |
| A test asserting the ABSENCE of something (no log line, no allocation, no route) | Deleting the mechanism leaves the same absence | Ask "what would still be absent if the code were removed?" |
| A test whose fixture is at an extreme (all-fields-set, max value) | An off-by-one or partial break still handles the extreme | Boundary the fixture: test one below and one above |
| A functional test whose data reaches the peer by a DIFFERENT path than the one changed | The unchanged path still delivers | Trace which code path actually produces the asserted bytes |

**When a change genuinely cannot be discriminated by the peer, because the receiver is required to accept both forms, you MUST say so explicitly in Goal Validation and move the discrimination to unit or mutation tests that CAN fail.** An interop test in that case proves ACCEPTANCE rather than correctness of the specific form, and you MUST state which.

## Goal Validation (all features)

**Before claiming a feature is done, MUST answer for each spec goal: "What concrete evidence proves this goal is achieved, beyond individual test assertions?"**

**Each goal type MUST carry the evidence its row names. "Tests pass" is not that evidence: goal validation is the bridge between the individual acceptance criteria and the feature's purpose:**

| Goal type | Required evidence |
|-----------|-------------------|
| Protocol interop ("ze speaks X with Y") | Interop test passes with the named peer daemon |
| Performance ("handles N updates/sec") | `ze-perf` benchmark result pasted |
| User workflow ("user can do X via CLI") | Functional `.ci` test exercising the full workflow, or an `.et` test for editor workflows |
| Data correctness ("routes installed correctly") | Functional test with explicit data assertions (hex match, JSON field match), never just exit code 0 |
| Resilience ("survives X failure") | Chaos test or fault-injection scenario |
| Security ("rejects unauthorized X") | Negative test: the unauthorized attempt fails with the expected error |

**These MUST NOT be offered as goal validation:**

| Not this | This instead |
|----------|-------------|
| "All tests pass" | "Here is the specific evidence for each goal" |
| "AC-1 through AC-5 implemented" | "AC-1 through AC-5 implemented, and together they prove [goal]" |
| "I tested it manually" | An automated test that can be re-run |
| "The code looks correct" | Observable behavior matching the spec's stated purpose |

**The spec's Goal Validation section MUST map each stated goal to the evidence that proves it.** It is filled during `/ze-close` step 1 and verified during `/ze-review`.

## Mechanical Check

**For every protocol feature in the spec, you MUST confirm a matching interop scenario exists under the suite's scenario directory. When none matches, one MUST be created before you claim done.**

**The spec's Goal Validation table MUST carry:**
- One row per stated goal, taken from the Task section
- An Evidence column filled with a concrete reference: a test name, a file path, or command output
- No empty evidence cells

## Relationship to Other Rules

**This rule adds to its neighbors and MUST NOT be read as replacing any of them:**
- `ai/rules/testing.md` requires functional tests per feature type, and owns the test infrastructure and workflow. This rule adds interop on top for protocol features, and says when each test type is mandatory
- `ai/rules/completion.md` requires every acceptance criterion tested. This rule requires the AGGREGATE goal proven
- `ai/rules/rfc-compliance.md` requires RFC conformance in code. This rule requires that conformance proven against another implementation
