# Spec: restore-bespoke-interop-assertions

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Fifteen interop scenarios run, establish sessions, and assert nothing.

`specialCheckers` (`internal/le/interoplab/bgp/check_special.go`) registers each of
them as a BESPOKE checker, which is correct: their control flow cannot be written
as an ordered operation list. What was never written is the bespoke Go body.
Thirteen of the fifteen have a body of one line that calls `checkScenario` with
their own scenario name, and `checkScenario` (`check_engine.go`) answers
`scenario %s has no typed assertions` because `scenarioOperations` does not hold
them. Two more carry a complete implementation behind that same call, used as a
guard that always fires, so every assertion below it is unreachable.

Adding `scenarioOperations` keys is not the repair, and a test already forbids it:
`TestCheckerPopulationMatchesProducer` (`bgp_test.go`) fails with
`bespoke checker %s still has a generic fallback` when a `specialCheckers` name
also appears in that table.

This is a RESTORE, not new design. At `a374622db~1` every one of the fifteen
carried a Python checker with real assertions. The le-personality migration
(`a374622db`, `eae282592`, 2026-08-27/28) ported 116 scenarios into typed
operations and left these fifteen behind, because their logic did not fit the
operation vocabulary: their deleted Python has a median of 141 lines against 59
for the ported set. Every one is recoverable verbatim with
`git show a374622db~1:test/interop/scenarios/<name>/check.py`, and every scenario
directory still holds its `.conf` inputs.

The cost of leaving them is not hypothetical. `rfc/requirements/rfc2545.md` binds
two SHALL requirements to `checkRFC2545NextHops` by FUNCTION NAME as
interop/nightly evidence, and that function has never executed a line of its own
assertions. `rfc/enrolled.txt` cites three of the scenarios in its enrolment
prose, `docs/features/rfc-status.md` cites three, and two architecture pages cite
four more.

The work is: express what each deleted checker asserted as a pure predicate over
daemon output plus the lab I/O that feeds it, in the shape the ported bespoke
checkers already use. It is not a transcription of Python.

Scope is settled by the owner (2026-08-31): all fifteen, no trimming. Nothing in
this spec re-opens it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - the suite this work repairs, its
  vacuity traps, and its rule for querying Ze
  → Decision: assertions never live in the scenario directory; they are typed Go
  checkers under `internal/le/interoplab/`, and a scenario with a nil checker is
  an error rather than a skip.
  → Constraint: a scenario is evidence only when it goes RED with the behavior
  broken. An absent value never proves a negative assertion by itself: the
  operation must also name positive evidence that the query mechanism ran.
  → Constraint: let the harness build. Never build the image by hand and then run
  with `NO_BUILD=1`; the tag is shared across sessions on the host.
- [ ] `docs/contributing/ze-go-style.md` - the standard for every restored body
  → Constraint: guard, handle, return. Split a compound condition. State the
  invariant positively. A helper takes the name of its caller.
  → Constraint: one concern in one file, and 1000 lines is where you look for a
  second concern. `check_rfc.go` is 415 lines today and the restored bodies add
  several hundred, so the split is planned here rather than discovered later.
- [ ] `ai/rules/interop-and-goal-validation.md` - what an interop test owes
  → Constraint: a test added to already-working code never had a red phase, so
  its discrimination is unproven until one is forced.
- [ ] `ai/rules/evidence.md` - fail closed or say something
  → Constraint: a helper never converts a failed query into a plausible value.
  A failed query and an empty protocol state are different answers.
- [ ] `ai/rules/rfc-compliance.md` - what the restored bodies are evidence FOR
  → Constraint: an `RFC requirement:` tag is a claim about what the body asserts.
  A tag left above a body that does not assert it is a false citation.

### RFC Summaries (Scope: protocol)

The scope of this spec is tooling, but each restored body is the cited evidence
for a requirement, so the summary is read before the body is written.

- [ ] `rfc/short/rfc2545.md` - RFC2545-3-2, RFC2545-3-3, bound to
  `checkRFC2545NextHops` by function name
  → Constraint: the link-local address is included only when the speaker shares a
  subnet with BOTH the entity named by the global next hop and the peer.
- [ ] `rfc/short/rfc4724.md` - RFC4724-4-1, bound to `checkNoFamilyEndOfRIB`
  → Constraint: the End-of-RIB marker is owed on a session where neither speaker
  advertised a Multiprotocol capability.
- [ ] `rfc/short/rfc7999.md` - RFC7999-3.3-1 and -3.3-2, two bullet conditions of
  one MUST sentence, each with its own negative
- [ ] `rfc/short/rfc1997.md` - RFC1997-Well-1, the NO_EXPORT boundary and its scope
- [ ] `rfc/short/rfc5301.md` - RFC5301-3-4 and -3-6, TLV type 137 and its value length
- [ ] `rfc/short/rfc3101.md` - RFC3101-2.4-5, the NSSA default without a config gate
- [ ] `rfc/short/rfc4271.md` - 5.1.2-3, 5.1.3-1, 5.1.4-1, 5.1.4-4, 5.1.5-2
- [ ] `rfc/short/rfc7606.md` - RFC7606-5.1-3 and RFC7606-5.4-1
- [ ] `rfc/short/rfc7911.md` - RFC7911-2-2, the re-advertised Path Identifier
- [ ] `rfc/short/rfc7947.md` - RFC7947-x-1, route server AS-path transparency

**Key insights:** (minimal context to resume after compaction)
- The fifteen are all in one file, `internal/le/interoplab/bgp/check_rfc.go`.
- Two shapes: 13 one-line stubs, and 2 full implementations behind a guard that
  always fires. The 2 are the shortest path to a genuine red-to-green.
- A bespoke checker is split in two: a pure predicate over text, and a body that
  does the lab I/O. `TestBespokeCheckerBranches` exercises the predicate in both
  polarities with no container anywhere.
- Proof has two tiers. The predicate tier runs in seconds; only the end-to-end
  tier needs Docker and the nightly budget.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/interoplab/bgp/check_rfc.go` - holds all fifteen checkers, their
  `RFC requirement:` tags, and the helpers the two guarded bodies already use
  (`queryFRREstablishedEpoch`, `waitAddPathState`, `waitFRRNewEpoch`,
  `samePathIdentifiers`, `queryFRRNextHops`, `parseFRRNextHops`,
  `requireNextHopShape`, `parseAddPathState`, `parseOTCValue`)
- [ ] `internal/le/interoplab/bgp/check_special.go` - `specialCheckers`, the bespoke
  registry, and the ported bodies that show the target shape
  (`checkBFDFailover`, `checkHoldtimeDeadPeer`, `checkReflectorWithdrawal`,
  `checkMaxPrefixPerFamily`, `checkAddPathRailAgreement`)
- [ ] `internal/le/interoplab/bgp/check_engine.go` - `checkScenario` returns
  `scenario %s has no typed assertions`; `checkerFailure` wraps a cause as
  `scenario %s assertion %d: %w` and appends peer logs
- [ ] `internal/le/interoplab/bgp/checkers.go` - `scenarioOperations` and
  `scenarioExtras`, the table the fifteen must NOT enter
- [ ] `internal/le/interoplab/bgp/bgp_test.go` - the four tests that constrain this
  work: `TestCheckerPopulationMatchesProducer`,
  `TestEveryCheckerFailsClosedWithoutPeerEvidence`,
  `TestRFCInteropCheckerBindings`, `TestBespokeCheckerBranches`
- [ ] `docs/architecture/testing/interop.md` - the page that describes the suite
- [ ] `rfc/requirements/rfc2545.md` - binds RFC2545-3-2 and RFC2545-3-3 to
  `internal/le/interoplab/bgp/check_rfc.go` `checkRFC2545NextHops` (interop/nightly)
- [ ] `rfc/enrolled.txt` - cites `bgp-rfc7999-blackhole-frr` (rfc7999),
  `bgp-wellknown-noexport-frr` (rfc1997), `isis-p2p-frr` (rfc5301)

**The fifteen, as they stand:**

| Scenario | Checker | Body today | Deleted checker lines | Peer daemons and sidecars |
|----------|---------|-----------|----------------------|---------------------------|
| `bgp-addpath-readvertise-collision-frr` | `checkAddPathReadvertiseCollision` | full body behind an always-failing guard | 221 | FRR, BIRD, GoBGP |
| `bgp-rfc2545-linklocal-nexthop-frr` | `checkRFC2545NextHops` | full body behind an always-failing guard | 201 | FRR |
| `no-family-peer-eor-frr` | `checkNoFamilyEndOfRIB` | stub | 117 | FRR |
| `isis-p2p-frr` | `checkISISDynamicHostname` | stub | 86 | FRR |
| `ospf-stub-nssa-frr` | `checkNSSADefault` | stub | 54 | FRR |
| `bgp-relay-withdraw-shape-frr` | `checkRelayWithdrawalShape` | stub | 160 | FRR, raw injector |
| `bgp-rfc7606-relay-shape-frr` | `checkRFC7606MixedUpdate` | stub | 122 | FRR, raw injector |
| `bgp-self-nexthop-withheld-frr` | `checkSelfNextHopWithheld` | stub | 142 | FRR, raw injector |
| `bgp-rfc7999-blackhole-frr` | `checkRFC7999Blackhole` | stub | 140 | FRR, BIRD |
| `bgp-route-server-frr` | `checkRouteServerASPath` | stub | 52 | FRR, BIRD |
| `bgp-wellknown-noexport-frr` | `checkNoExportBoundary` | stub | 55 | FRR, BIRD, raw injector |
| `bgp-local-pref-strip-gobgp` | `checkLocalPrefStrip` | stub | 178 | GoBGP, FRR, raw injector |
| `bgp-med-across-as-gobgp` | `checkMEDAcrossAS` | stub | 206 | GoBGP, FRR, raw injector |
| `bgp-med-remove-configured-gobgp` | `checkMEDRemovalConfiguration` | stub | 179 | GoBGP, FRR, raw injector |
| `bgp-rfc7606-typed-nlri-discard` | `checkRFC7606TypedNLRIDiscard` | stub | 103 | raw injector, strict speaker |

**Behavior to preserve:** (unless the user explicitly said to change it)
- The registry population. The same fifteen names stay in `specialCheckers`, and
  none of them gains a `scenarioOperations` or `scenarioExtras` key.
- The failure text contract. `TestRFCInteropCheckerBindings` drives each checker
  with a lab that fails every observation and requires the returned error to
  CONTAIN the scenario name. Today `checkScenario` supplies that name. Once it is
  gone, the restored body owes the same, which `checkerFailure` provides.
- Fail-closed behavior. `TestEveryCheckerFailsClosedWithoutPeerEvidence` requires
  an error when every observation fails. The fifteen pass it today for the wrong
  reason: they always error. After the repair they must still fail closed.
- Every `RFC requirement:` tag comment above a touched checker, with its polarity
  claim still true of the restored body.
- `checkOSPFLFA` is three lines and is CORRECT: it delegates to
  `checkOSPFFastReroute`, a real implementation. It is not in scope.

**Behavior to change:**
- `checkAddPathReadvertiseCollision` and `checkRFC2545NextHops` stop returning
  early and run their own assertions.
- The thirteen stubs stop delegating and assert against their peers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator or the nightly job runs `./le integration interop`, optionally
  narrowed with `INTEROP_SCENARIO=<name>`.
- Format at entry: a scenario directory name, matched exactly by
  `interoplab.Discover` against the subdirectories of `test/interop/scenarios/`.

### Transformation Path
1. `Discover` (`internal/le/interoplab/discover.go`) joins each scenario
   directory with the BGP checker registry; a missing or nil checker is an error.
2. The suite (`internal/le/interoplab/lab.go`) creates the network, starts Ze and
   the peer daemons the scenario's config files declare, and waits for readiness.
3. The registered `interoplab.Checker` runs with a `CheckContext` carrying the
   selected `Network` and the `CheckerLab`.
4. The checker body queries the peers through `Lab.Query`, `Lab.Logs` and
   `Lab.Exec`, which run a command inside a named container.
5. The daemon's own CLI or log text is handed to a pure predicate, which returns
   the decision. The body wraps a failure with `checkerFailure`, which names the
   scenario and the assertion index and appends peer logs.
6. Teardown removes every container and network, including after a failure.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Checker ↔ peer daemon | `Lab.Query` runs the daemon's own CLI, or `ip route show`, inside the peer container and returns its text | No |
| Checker ↔ Ze | `Ze.cli` only, with the user and format flags; the harness appends the SSH listener to the rendered `ze.conf` | No |
| Checker ↔ strict speaker | container logs carrying a structured verdict, read with `Lab.Logs` | No |
| Lab I/O ↔ predicate | plain text or decoded JSON passed as a function argument, no lab handle | No |

### Integration Points
- `specialCheckers` (`check_special.go`) - the fifteen entries already exist and
  are unchanged; only the functions they name gain bodies.
- `checkerFailure` (`check_engine.go`) - the failure wrapper every restored body
  uses, so the scenario name survives into the error.
- `TestBespokeCheckerBranches` (`bgp_test.go`) - the unit tier every restored
  predicate is proven in.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Restored bodies query peers through `CheckerLab` only; no direct container exec and no file read from the scenario directory |
| No unintended coupling (components stay isolated) | No | The work stays inside `internal/le/interoplab/bgp`; no product code under `internal/component/` is touched |
| No duplicated functionality (extends existing, does not recreate) | No | Reuses `waitContains`, `requireAbsentWithProof`, `queryFRRNextHops`, `parseOTCValue` and the other helpers already in the package rather than writing new ones per scenario |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Test tooling over daemon text output, not a wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | `specialCheckers` is the registration point and its population does not change; `Discover` still derives the scenario set from the directory listing |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The deleted checker source is recoverable verbatim for all fifteen | `git show a374622db~1:test/interop/scenarios/<name>/check.py` returned 53 to 222 lines for each of the fifteen (run 2026-08-31) | The assertion has to be re-derived from the scenario config and the RFC text, at a much higher cost | Re-running `git show` per scenario at the start of each group | unvalidated |
| A-2 | Each scenario's `.conf` inputs are unchanged since its checker was deleted, so its assertions still describe the topology | All fifteen directories exist with their config files; only the checker file was removed by `a374622db` | A restored assertion names an address, prefix or AS that the config no longer produces | `git diff a374622db~1 -- test/interop/scenarios/<name>` per scenario, before writing the body | unvalidated |
| A-3 | The two guarded bodies were correct when they were written, so deleting the guard exposes working assertions rather than broken ones | Both carry complete implementations with helpers and unit-tested parsers (`TestRFCInteropStructuredPeerEvidence`) | AC-1 goes red and the cause is either a stale assertion or a Ze defect; both are in scope | The AC-1 nightly run | unvalidated |
| A-4 | `TestRFCInteropCheckerBindings` matches the scenario name that `checkerFailure` writes, so wrapping with it preserves the binding | `check_engine.go` formats `scenario %s assertion %d: %w`; the test requires the returned error text to contain the scenario name | Every restored body fails that test and the binding has to be produced another way | Running that test alone over the package | unvalidated |
| A-5 | Each `RFC requirement:` tag describes an assertion the deleted checker actually made | The `no-family-peer-eor-frr` checker carries the `RFC4724-4-1 positive` tag and its docstring names both assertions | A tag is a false citation and must be corrected, which is an owner question when it lowers a claim (`ai/rules/rfc-compliance.md`) | Comparing each tag against the restored body, per checker | unvalidated |
| A-6 | The predicate plus lab-I/O split is the house shape for a bespoke checker | `bfdSessionDown`, `holdNotificationSeen`, `frrReceivedHoldExpiry`, `speakerRouteUpdate`, `maxPrefixWarnOnlyDecision` and `requireBFDTeardownBudget` are all pure and all covered by `TestBespokeCheckerBranches` | The unit tier is unavailable and every proof falls to Docker | Reading those subtests before writing the first predicate | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Removing an always-failing guard makes real assertions run against a live peer for the FIRST time, so they can fail for their own reasons | AC-1's first live run is red | This is expected work, not a surprise. Root-cause it: a stale assertion is fixed here; a Ze protocol defect the assertion exposes blocks the goal this work exists to achieve and is fixed here too (`ai/rules/completion.md`). Never weaken the assertion to reach green |
| R-2 | The deleted checkers matched the output of FRR 10.3.1, GoBGP 3.31.0 and BIRD 2.x as of 2026-08, and the images may have moved | A predicate finds no match on a live run while the daemon plainly holds the state | Prefer the daemon's JSON field over its rendered text where one exists; keep the case-insensitivity the deleted checker chose deliberately (the `no-family-peer-eor-frr` family match is case-insensitive because the spelling moves between FRR releases); pin the image with `FRR_IMAGE=` when isolating |
| R-3 | The end-to-end tier needs Docker, is nightly, and cannot run inside a short foreground budget | A group's work stalls waiting on a suite run | Two tiers, not one. The predicate tier proves the assertion logic in seconds and lands first. Batch the Docker runs per AC group with `INTEROP_SCENARIO=<name>`, and let the harness build |
| R-4 | A restored predicate asserts an ABSENCE that would hold with the mechanism removed (vacuity trap 2 in `docs/architecture/testing/interop.md`) | The negative subtest still passes when the predicate is given empty input | Every absence assertion goes through `requireAbsentWithProof`, which needs positive evidence that the query mechanism ran; the negative subtest feeds it empty text and requires an error |
| R-5 | The package is too large for one agent: fifteen bodies with a median of 141 deleted checker lines each | One session finishes fewer than its assigned AC groups | Report the size to the main thread and let it re-cut the packages by AC group (`ai/rules/planning.md`). Never trim an AC to fit |
| R-6 | `check_rfc.go` passes the 1000-line threshold as the bodies land | The file grows past 1000 lines mid-phase | The split is planned in Files to Create: predicates and daemon-output parsers move to their own file, and the tagged bodies stay together |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | No product code changes, so no live session, route or config risk. A wrong restoration leaves an interop gate that still proves nothing, which is the current failure with a green bar, or a nightly that goes red for a test-side reason and costs the owner a triage |
| How is it reverted? | Single-commit revert of `internal/le/interoplab/bgp/`. No migration, no state |
| Who else touches this path? | Any session adding an interop scenario edits `checkers.go` and `check_special.go` in the same package; `rfc/requirements/` rows cite `check_rfc.go` by function name |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `INTEROP_SCENARIO=bgp-rfc2545-linklocal-nexthop-frr ./le integration interop` | → | `checkRFC2545NextHops` FRR next-hop assertions | `TestRFCInteropCheckerBindings/bgp-rfc2545-linklocal-nexthop-frr` |
| `INTEROP_SCENARIO=bgp-addpath-readvertise-collision-frr ./le integration interop` | → | `checkAddPathReadvertiseCollision` epoch and Path Identifier comparison | `TestRFCInteropCheckerBindings/bgp-addpath-readvertise-collision-frr` |
| `INTEROP_SCENARIO=no-family-peer-eor-frr ./le integration interop` | → | `checkNoFamilyEndOfRIB` FRR log match | `TestBespokeCheckerBranches/no-family-peer-eor-frr` |
| `INTEROP_SCENARIO=bgp-rfc7999-blackhole-frr ./le integration interop` | → | `checkRFC7999Blackhole` kernel FIB assertions | `TestBespokeCheckerBranches/bgp-rfc7999-blackhole-frr` |
| `INTEROP_SCENARIO=bgp-med-across-as-gobgp ./le integration interop` | → | `checkMEDAcrossAS` GoBGP RIB assertions | `TestBespokeCheckerBranches/bgp-med-across-as-gobgp` |
| `INTEROP_SCENARIO=bgp-rfc7606-typed-nlri-discard ./le integration interop` | → | `checkRFC7606TypedNLRIDiscard` speaker verdict | `TestBespokeCheckerBranches/bgp-rfc7606-typed-nlri-discard` |
| `./le integration interop` (whole suite) | → | `specialCheckers` registry population | `TestCheckerPopulationMatchesProducer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `INTEROP_SCENARIO=bgp-addpath-readvertise-collision-frr` and `INTEROP_SCENARIO=bgp-rfc2545-linklocal-nexthop-frr` each run against their live labs | Neither run reports `has no typed assertions`. Each executes its own assertions: the first compares Path Identifiers across an FRR-forced re-establishment, the second compares the on-link and off-link next-hop shapes FRR decoded. A run whose peer state is wrong fails naming the assertion |
| AC-2 | `no-family-peer-eor-frr`, `isis-p2p-frr` and `ospf-stub-nssa-frr` run against FRR with no injector | Each asserts the state its RFC tag claims, read from FRR: the End-of-RIB marker for IPv4 unicast on a session where neither side advertised a Multiprotocol capability, Ze's dynamic hostname rendered from TLV 137, and the NSSA default originated without a config gate |
| AC-3 | `bgp-relay-withdraw-shape-frr`, `bgp-rfc7606-relay-shape-frr` and `bgp-self-nexthop-withheld-frr` run with the raw injector driving Ze | Each asserts FRR's decode of what Ze relayed: the AS_PATH on the announcement and the clean acceptance of the withdrawal, the acceptance and installation of one UPDATE mixing withdrawn routes with NLRI, and the absence of a route whose NEXT_HOP is FRR's own address beside the presence of the third-party-next-hop route |
| AC-4 | `bgp-rfc7999-blackhole-frr`, `bgp-route-server-frr` and `bgp-wellknown-noexport-frr` run with FRR and BIRD as two independent observers | Each asserts both polarities across the two observers: a discard route in the Ze container's FIB for the covered prefix on the agreeing session and an ordinary route for the two negatives, an AS path with no route-server ASN prepended, and NO_EXPORT withheld from the external observer while the internal one learns the same prefix |
| AC-5 | `bgp-local-pref-strip-gobgp`, `bgp-med-across-as-gobgp` and `bgp-med-remove-configured-gobgp` run with GoBGP | Each asserts GoBGP's decode of Ze's egress attributes: no LOCAL_PREF on an external session, no received MED propagated to another neighboring AS while Ze's own metric survives, and the configured removal applied to the matched prefix while the control prefix keeps MED 100 |
| AC-6 | `bgp-rfc7606-typed-nlri-discard` runs with the raw injector and the compiled strict speaker | The checker reads the speaker's structured verdict and asserts the assigned EVPN route type reached it while the unassigned type from the same attribute did not |
| AC-7 | Every one of the fifteen checkers is driven by a lab whose observations all fail | Each returns an error, and that error names its own scenario. `TestEveryCheckerFailsClosedWithoutPeerEvidence` and `TestRFCInteropCheckerBindings` both pass with no change to either test's expectations |
| AC-8 | The bespoke-branch unit test runs with no Docker available | Every restored checker has a named subtest that exercises its predicate in BOTH polarities: the true case, and a false case written against a specific wrong reading (tokens matched across two log lines, a peer-originated event passing as a received one, an absence with no proof the query ran) |
| AC-9 | The checker-population unit test runs | It passes, and no name among the fifteen appears in `scenarioOperations` or `scenarioExtras`. The fifteen stay bespoke |
| AC-10 | Each `RFC requirement:` tag above a restored checker is read against its body | The tag's polarity claim is asserted by the body: a `positive` tag has an assertion that the state IS present, a `negative` tag one that the opposite state is present with proof the query ran. No tag survives above a body that does not assert it |

## Goal Validation

| Goal | Evidence that proves it | How it is forced RED |
|------|------------------------|----------------------|
| The fifteen scenarios assert what their names claim | Each AC group's `INTEROP_SCENARIO=<name> ./le integration interop` run passes, quoting the image ID the harness read from its own build | Per group, revert the Ze behavior under test and rebuild through the harness: drop the per-side implicit family in `capability.Negotiate` (AC-2, the mutation this scenario was written against, measured 2026-08-17), stop narrowing BLACKHOLE announcements to the sessions that agreed the community (AC-4), stop stripping the received MED on egress to another AS (AC-5), emit LOCAL_PREF to an external peer (AC-5), prepend the local AS on a route-server relay (AC-4), relay a route whose NEXT_HOP is the receiving peer's own address (AC-3). Restore, confirm GREEN, record the RED output |
| Each restored checker DISCRIMINATES rather than merely passing | A negative subtest in `TestBespokeCheckerBranches` for every restored predicate, failing against a stated wrong reading | The negative subtest IS the forced red, and it runs in seconds with no container. A predicate that asserted only the true case would pass against a stub, which is the exact failure this spec repairs |
| The RFC citations that name these checkers become true | `rfc/requirements/rfc2545.md` rows RFC2545-3-2 and RFC2545-3-3 name `checkRFC2545NextHops`, and that function now executes both next-hop shape assertions; the three `rfc/enrolled.txt` scenario citations and the three `docs/features/rfc-status.md` rows are each re-read against the restored body | AC-10: a tag whose claim the body does not assert is a finding, not a pass |
| The repair does not weaken the gate it repairs | `TestCheckerPopulationMatchesProducer`, `TestEveryCheckerFailsClosedWithoutPeerEvidence` and `TestRFCInteropCheckerBindings` pass with no edit to their expectations | Each of the three already fails on the failure mode it guards: a generic fallback, a checker that passes with no peer evidence, an error that does not name its scenario |

## 🧪 TDD Test Plan

The proof has TWO tiers, and the expensive one is not the only one.

Tier 1 is the predicate: a pure function over daemon text or decoded JSON,
exercised in both polarities by a named subtest of `TestBespokeCheckerBranches`,
with no container anywhere. It runs in the foreground in seconds.

Tier 2 is the end-to-end path: the checker body's lab I/O feeding that predicate
against a live peer, proven by a Docker run at nightly tier.

Every restored checker owes both. `recorderFor` and `contradictoryRecorderFor`
serve `scenarioOperations` rows and are NOT the route for this work.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBespokeCheckerBranches/bgp-addpath-readvertise-collision-frr` | `internal/le/interoplab/bgp/bgp_test.go` | Path Identifier sets that agree and sets that differ across the replay | |
| `TestBespokeCheckerBranches/bgp-rfc2545-linklocal-nexthop-frr` | `internal/le/interoplab/bgp/bgp_test.go` | On-link shape accepted; a link-local entry on the off-link route rejected (a covering case exists under `TestRFCInteropStructuredPeerEvidence`, so extend rather than duplicate) | |
| `TestBespokeCheckerBranches/no-family-peer-eor-frr` | `internal/le/interoplab/bgp/bgp_test.go` | The End-of-RIB line matched case-insensitively on the family and exactly on the peer; a marker naming a different peer rejected | |
| `TestBespokeCheckerBranches/isis-p2p-frr` | `internal/le/interoplab/bgp/bgp_test.go` | The configured hostname read out of FRR's IS-IS database; a raw system ID rejected | |
| `TestBespokeCheckerBranches/ospf-stub-nssa-frr` | `internal/le/interoplab/bgp/bgp_test.go` | The NSSA default present in FRR's OSPF database; an ordinary external LSA rejected | |
| `TestBespokeCheckerBranches/bgp-relay-withdraw-shape-frr` | `internal/le/interoplab/bgp/bgp_test.go` | The relayed AS_PATH accepted; a withdrawal that drew an attribute error rejected | |
| `TestBespokeCheckerBranches/bgp-rfc7606-relay-shape-frr` | `internal/le/interoplab/bgp/bgp_test.go` | The mixed UPDATE accepted and installed; a treat-as-withdraw outcome rejected | |
| `TestBespokeCheckerBranches/bgp-self-nexthop-withheld-frr` | `internal/le/interoplab/bgp/bgp_test.go` | The absence of the self-next-hop route with positive proof the per-UPDATE log was read; the same absence with no proof rejected | |
| `TestBespokeCheckerBranches/bgp-rfc7999-blackhole-frr` | `internal/le/interoplab/bgp/bgp_test.go` | A discard route for the covered prefix accepted; an ordinary forwarding route for it rejected; a discard route for the uncovered prefix rejected | |
| `TestBespokeCheckerBranches/bgp-route-server-frr` | `internal/le/interoplab/bgp/bgp_test.go` | An AS path without the route server's own ASN accepted; one carrying it rejected | |
| `TestBespokeCheckerBranches/bgp-wellknown-noexport-frr` | `internal/le/interoplab/bgp/bgp_test.go` | The external observer's absence with proof it learned the control prefix; the internal observer's presence required | |
| `TestBespokeCheckerBranches/bgp-local-pref-strip-gobgp` | `internal/le/interoplab/bgp/bgp_test.go` | GoBGP output with no LOCAL_PREF accepted; output carrying one rejected | |
| `TestBespokeCheckerBranches/bgp-med-across-as-gobgp` | `internal/le/interoplab/bgp/bgp_test.go` | The received metric absent at the third AS while Ze's own metric is present; either half missing rejected | |
| `TestBespokeCheckerBranches/bgp-med-remove-configured-gobgp` | `internal/le/interoplab/bgp/bgp_test.go` | The matched prefix without MED and the control prefix with MED 100; an unconditional strip rejected | |
| `TestBespokeCheckerBranches/bgp-rfc7606-typed-nlri-discard` | `internal/le/interoplab/bgp/bgp_test.go` | A speaker verdict naming the assigned route type accepted; one naming the unassigned type rejected | |
| `TestRFCInteropCheckerBindings` | `internal/le/interoplab/bgp/bgp_test.go` | Every restored body still returns an error naming its scenario when the lab fails | |
| `TestEveryCheckerFailsClosedWithoutPeerEvidence` | `internal/le/interoplab/bgp/bgp_test.go` | Every restored body still fails closed with no peer evidence | |
| `TestCheckerPopulationMatchesProducer` | `internal/le/interoplab/bgp/bgp_test.go` | The fifteen stay bespoke, with no generic fallback and no generic extra operations | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Next-hop entries on the on-link IPv6 route | 1-2 | 2 (global plus link-local) | 1 (global alone, which is the off-link shape) | 3 (a second link-local entry) |
| Next-hop entries on the off-link IPv6 route | 1-1 | 1 (global alone) | 0 (no next hop decoded) | 2 (a link-local entry that Section 3 forbids here) |
| Distinct Path Identifiers on the re-advertised prefix | 2-2 | 2 | 1 (the collision this scenario exists to catch) | N/A (the scenario advertises two paths) |
| MED on the control prefix in `bgp-med-remove-configured-gobgp` | 100-100 | 100 | absent (an unconditional strip) | any other value (a rewritten metric) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | The user-facing surface of this work is `./le integration interop`, whose tier is the interop suite in the table below. A `.ci` case drives a Ze daemon and cannot observe FRR, BIRD, GoBGP or the strict speaker, which is what all fifteen assertions exist to do | N-A |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-addpath-readvertise-collision-frr` | `test/interop/scenarios/` | FRR, BIRD, GoBGP | A re-advertising speaker generates its own Path Identifiers, and the replay after a peer reset repeats them (RFC7911-2-2) | |
| `bgp-rfc2545-linklocal-nexthop-frr` | `test/interop/scenarios/` | FRR | The next-hop length octet and the link-local inclusion condition (RFC2545-3-2, RFC2545-3-3) | |
| `no-family-peer-eor-frr` | `test/interop/scenarios/` | FRR | End-of-RIB for IPv4 unicast with no Multiprotocol capability on either side (RFC4724-4-1) | |
| `isis-p2p-frr` | `test/interop/scenarios/` | FRR | Dynamic hostname TLV 137 and its value length (RFC5301-3-4, RFC5301-3-6) | |
| `ospf-stub-nssa-frr` | `test/interop/scenarios/` | FRR | The NSSA default originated without a config gate (RFC3101-2.4-5) | |
| `bgp-relay-withdraw-shape-frr` | `test/interop/scenarios/` | FRR, raw injector | AS_PATH prepended on advertisement, absent and unfaulted on withdrawal (RFC4271-5.1.2-3) | |
| `bgp-rfc7606-relay-shape-frr` | `test/interop/scenarios/` | FRR, raw injector | One UPDATE mixing withdrawn routes with NLRI is accepted, relayed and installed (RFC7606-5.1-3) | |
| `bgp-self-nexthop-withheld-frr` | `test/interop/scenarios/` | FRR, raw injector | A route whose NEXT_HOP is the peer's own address is withheld; a third-party next hop is not (RFC4271-5.1.3-1) | |
| `bgp-rfc7999-blackhole-frr` | `test/interop/scenarios/` | FRR, BIRD | Both bullet conditions of the BLACKHOLE MUST, each with its negative (RFC7999-3.3-1, RFC7999-3.3-2) | |
| `bgp-route-server-frr` | `test/interop/scenarios/` | FRR, BIRD | A route server does not prepend its own AS (RFC7947-x-1) | |
| `bgp-wellknown-noexport-frr` | `test/interop/scenarios/` | FRR, BIRD, raw injector | NO_EXPORT withheld outside the boundary and honored inside it (RFC1997-Well-1) | |
| `bgp-local-pref-strip-gobgp` | `test/interop/scenarios/` | GoBGP, FRR, raw injector | LOCAL_PREF not sent to an external peer (RFC4271-5.1.5-2) | |
| `bgp-med-across-as-gobgp` | `test/interop/scenarios/` | GoBGP, FRR, raw injector | A received MED is not propagated to another neighboring AS, while Ze's own metric is (RFC4271-5.1.4-1) | |
| `bgp-med-remove-configured-gobgp` | `test/interop/scenarios/` | GoBGP, FRR, raw injector | The configured MED removal applies to the matched prefix only (RFC4271-5.1.4-4) | |
| `bgp-rfc7606-typed-nlri-discard` | `test/interop/scenarios/` | raw injector, strict speaker | The assigned EVPN route type reaches the peer and the unassigned one does not (RFC7606-5.4-1) | |

## Files to Modify
- `internal/le/interoplab/bgp/check_rfc.go` - delete the two always-failing guard
  preambles, replace the thirteen delegating bodies with lab I/O that feeds a
  predicate, keep every `RFC requirement:` tag and keep each tag true
- `internal/le/interoplab/bgp/bgp_test.go` - a both-polarity subtest per restored
  predicate in `TestBespokeCheckerBranches`
- `docs/architecture/testing/interop.md` - the page `check_rfc.go`,
  `check_special.go`, `check_engine.go` and `bgp_test.go` declare in their
  `// Design:` headers. Its "Scenario Structure" section says `Audit()` retains
  the digest and obligation mapping of the removed checker revision, and no
  `Audit` symbol exists anywhere under `internal/le/interoplab/`; the same
  sentence is why "Writing a New Scenario" step 4 asks for a "source-contract
  audit row". Correct both here, since this work already edits the page
- `rfc/requirements/rfc2545.md` - re-read the RFC2545-3-2 and RFC2545-3-3 rows
  against the restored `checkRFC2545NextHops`; correct only what the restored body
  does not assert
- `docs/features/rfc-status.md` - re-read the RFC 4724, RFC 7999 and OSPF
  stub/NSSA rows against the restored bodies

## Files to Create
- `internal/le/interoplab/bgp/check_rfc_predicate.go` - the pure predicates and
  daemon-output parsers the restored bodies call. `check_rfc.go` is 415 lines and
  the bodies add several hundred, so the second concern is separated when it
  appears rather than after the file passes 1000 lines

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface changes; the work is inside the interop lab |
| YANG validation constraints | N-A | No leaf is added |
| YANG custom validators | N-A | No leaf is added |
| CLI commands/flags | N-A | `./le integration interop` and its scenario selector already exist and are unchanged |
| CLI grammar (keyword before value) | N-A | No command is added |
| Editor autocomplete | N-A | No config leaf is added |
| Functional test for new RPC/API | N-A | No RPC or API is added; the tier for this work is the interop suite |
| Pipe completeness | N-A | No command output is produced |
| Env var registration | N-A | `INTEROP_SCENARIO`, `SESSION_TIMEOUT`, `NO_BUILD`, `VERBOSE` and `FRR_IMAGE` are existing harness variables, not `ze.*` YANG-backed leaves |
| Doctor check for runtime dependencies | N-A | Docker is a prerequisite of the interop action, already documented in `docs/architecture/testing/interop.md`, and no new runtime dependency is introduced |
| Prometheus counters/metrics | N-A | Test tooling, no observable daemon state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute is added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Restored test assertions; no product feature reaches a user |
| 2 | Config syntax changed? | No | No YANG leaf, no parser change |
| 3 | CLI command added/changed? | No | The interop action is unchanged |
| 4 | API/RPC added/changed? | No | No handler is touched |
| 5 | Plugin added/changed? | No | No plugin is touched |
| 6 | Has a user guide page? | No | The audience is the developer running the suite, served by `docs/architecture/testing/interop.md` |
| 7 | Wire format changed? | No | No encoder or decoder is touched |
| 8 | Plugin SDK/protocol changed? | No | The compiled process helpers are used as they stand |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/requirements/rfc2545.md` (RFC2545-3-2, RFC2545-3-3 become executed evidence), `docs/features/rfc-status.md` (RFC 4724, RFC 7999, OSPF stub/NSSA rows) and the `rfc/enrolled.txt` prose citing `bgp-rfc7999-blackhole-frr`, `bgp-wellknown-noexport-frr` and `isis-p2p-frr`. Each row is re-read against the restored body; a row the body does not support is an owner question, never a silent edit |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` where it describes the bespoke checker route; `docs/architecture/testing/interop.md` for the stale `Audit()` sentence and the "source-contract audit row" step it produced |
| 11 | Affects daemon comparison? | No | No Ze capability changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/interop.md` gains the predicate plus lab-I/O shape as the documented way to write a bespoke checker |
| 13 | Route metadata keys added/changed? | No | No metadata key is touched |
| 14 | Prometheus counters added/changed? | No | No counter is added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | `specialCheckers` keeps exactly the same fifteen names; only the functions gain bodies |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, run `./le spec citation anchors spec plan/spec-restore-bespoke-interop-assertions.md` before closure. `check_rfc.go`, `check_special.go`, `check_engine.go` and `bgp_test.go` each declare `docs/architecture/testing/interop.md` in a `// Design:` header, and that page is named in Files to Modify. `ai/CODE-TO-DOCS.md` maps this package to `docs/architecture/testing/interop.md`, `docs/features/interoperability-testing.md`, `docs/functional-tests.md` and `docs/guide/vrrp.md`; `docs/guide/vrrp.md` is unaffected because no VRRP scenario is touched, and `docs/features/interoperability-testing.md` is re-read for any claim about what these fifteen prove |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/testing/interop.md` "Running" shows scenario-selector invocations; confirm every scenario name it prints still exists and that the "Writing a New Scenario" steps match the restored shape |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the two dead-code bodies reachable
   - Tests: `TestRFCInteropCheckerBindings`, `TestEveryCheckerFailsClosedWithoutPeerEvidence`
   - Files: `internal/le/interoplab/bgp/check_rfc.go`
   - Verify: delete the three-line `checkScenario` preamble from
     `checkAddPathReadvertiseCollision` and `checkRFC2545NextHops`, then wrap each
     body's failures so the error still names the scenario. Run the two tests
     before the wrapping lands and expect the binding test RED: that red proves
     the preamble was what supplied the scenario name. Then run the two scenarios
     end to end and record the result with the image ID the harness built
2. **Phase: AC-2, FRR-only observers** -- `no-family-peer-eor-frr`, `isis-p2p-frr`, `ospf-stub-nssa-frr`
   - Tests: the three `TestBespokeCheckerBranches` subtests, both polarities
   - Files: `check_rfc.go`, `check_rfc_predicate.go`, `bgp_test.go`
   - Verify: recover each deleted checker with `git show`, diff its scenario
     directory against `a374622db~1` (A-2), extract the assertion, write the
     predicate and its negative subtest, then the lab I/O. Tier 1 green, then one
     end-to-end run
3. **Phase: AC-3, injector-driven FRR relay shape** -- `bgp-relay-withdraw-shape-frr`, `bgp-rfc7606-relay-shape-frr`, `bgp-self-nexthop-withheld-frr`
   - Tests: the three subtests; the self-next-hop absence goes through `requireAbsentWithProof`
   - Files: `check_rfc.go`, `check_rfc_predicate.go`, `bgp_test.go`
   - Verify: as phase 2, plus the R-4 check that each absence assertion carries positive proof
4. **Phase: AC-4, two-observer FRR and BIRD** -- `bgp-rfc7999-blackhole-frr`, `bgp-route-server-frr`, `bgp-wellknown-noexport-frr`
   - Tests: the three subtests, each covering both observers
   - Files: `check_rfc.go`, `check_rfc_predicate.go`, `bgp_test.go`
   - Verify: as phase 2. The blackhole assertions read the Ze container's FIB, not a Ze table
5. **Phase: AC-5, GoBGP attribute policy** -- `bgp-local-pref-strip-gobgp`, `bgp-med-across-as-gobgp`, `bgp-med-remove-configured-gobgp`
   - Tests: the three subtests, each with its control-prefix half
   - Files: `check_rfc.go`, `check_rfc_predicate.go`, `bgp_test.go`
   - Verify: as phase 2, against GoBGP 3.31.0
6. **Phase: AC-6, strict speaker** -- `bgp-rfc7606-typed-nlri-discard`
   - Tests: the subtest over the speaker verdict text
   - Files: `check_rfc.go`, `check_rfc_predicate.go`, `bgp_test.go`
   - Verify: as phase 2, reading the speaker's structured verdict from container logs
7. **Phase: tags, citations and pages** -- AC-10 and the documentation rows
   - Tests: the whole package's unit tier, then `./le verify worktree`
   - Files: `check_rfc.go`, `docs/architecture/testing/interop.md`, `rfc/requirements/rfc2545.md`, `docs/features/rfc-status.md`, `docs/functional-tests.md`
   - Verify: every tag is read against its body; every citation named in Task is
     re-read; the stale `Audit()` sentence is corrected
8. **Phase: forced red** -- the Goal Validation mutations
   - Tests: the per-group mutation runs in the Goal Validation table
   - Files: none permanently; each mutation is reverted after its run
   - Verify: each group's mutation produces a RED scenario run, restoring produces
     GREEN, and both results are recorded with the image ID the harness built

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the fifteen has a body, a predicate, and a both-polarity subtest at file:line |
| Feature completeness | No checker reaches `checkScenario` any more: a grep for that call over `check_rfc.go` finds nothing |
| Correctness | Each restored assertion reads the FOREIGN daemon's decode, never Ze's own view of what it sent |
| Correctness | Every absence assertion carries positive proof that its query mechanism ran |
| Naming | A predicate is named for the decision it makes, not for its input type; a helper carries its caller's name |
| Data flow | The predicate takes text or decoded JSON and holds no lab handle, so its subtest needs no container |
| Rule: `ai/rules/rfc-compliance.md` | Every `RFC requirement:` tag is true of the body under it, and no tag's level is lowered without the owner |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each of the four vacuity traps is checked by its tell before a scenario is called evidence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No checker delegates to `checkScenario` | `grep -n 'checkScenario(ctx, check, "' internal/le/interoplab/bgp/check_rfc.go` returns nothing |
| The fifteen stay bespoke | Run `TestCheckerPopulationMatchesProducer` over the package |
| Every restored predicate is covered in both polarities | Run `TestBespokeCheckerBranches` verbosely: it lists one subtest per scenario |
| Every checker fails closed | Run `TestEveryCheckerFailsClosedWithoutPeerEvidence` and `TestRFCInteropCheckerBindings` |
| Each scenario passes end to end | `INTEROP_SCENARIO=<name> ./le integration interop` per scenario, with the image ID quoted |
| Each group discriminates | The recorded RED output of its Goal Validation mutation, and the GREEN after restoring |
| The gate is green | `./le verify worktree` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A predicate parses text a container produced. Malformed or empty text must return an error, never a plausible verdict |
| Error leakage | `checkerFailure` appends up to 80 lines of peer logs; confirm no scenario config carries a credential that would land in CI output |
| Resource exhaustion | Every wait uses an explicit bound; no new unbounded poll is added |
| Authorization that could fail open | The fail-closed tests are the guard, and neither may be weakened to accommodate a restored body |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A restored assertion goes red against a live peer | Root-cause it. A stale assertion is fixed here; a Ze protocol defect it exposes blocks this work's goal and is fixed here (`ai/rules/completion.md`, `ai/rules/rfc-compliance.md`) |
| The daemon's output no longer matches the deleted checker | Prefer its JSON field; if none exists, pin the image and record the version the predicate reads |
| A work package is too large for one session | Report the size to the main thread and let it re-cut by AC group. Never trim an AC |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The two shapes have very different costs. Deleting a three-line preamble is
  minutes of work and buys the first genuine red-to-green in this spec; the
  thirteen bodies are the bulk.
- `checkScenario` used as a guard is a fail-closed idiom that inverted: the
  function whose job is to answer "no typed assertions" was called by the very
  checkers that were supposed to carry them, and its always-true error read as an
  ordinary early return.
- The scenario name inside a failure is load-bearing beyond diagnostics: a test
  asserts on it, so deleting the delegation removes a contract as well as a stub.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Restore as predicate plus lab I/O | One monolithic checker body doing query and decision together | The predicate is testable in both polarities with no container. A monolith puts every proof behind Docker, which is where these fifteen went unproven |
| Keep the tagged bodies in `check_rfc.go` and move predicates and parsers to `check_rfc_predicate.go` | Split by peer daemon into one file per daemon | The RFC tag catalogue is one concern and reads best in one file. The second concern is the pure decision layer, not the peer. A peer split would put one RFC's positive and negative halves in different files whenever two daemons observe one requirement, which AC-4 does three times |
| Recover the assertion from the deleted checker, not its shape | Rewrite each assertion from the RFC text alone | The deleted checkers encode measured daemon behavior: which spelling moves between FRR releases, which field is stable. Re-deriving that from the RFC would repeat the measurement at container cost |
| Two proof tiers, unit first | Prove everything with the Docker suite | The unit tier runs in seconds in the foreground and holds the discrimination proof. Docker proves only that the lab I/O reaches the predicate |

## Known Limitations
- The end-to-end tier is nightly and needs Docker, so the full fifteen-scenario
  evidence set cannot be produced in one short foreground session. The unit tier
  can, and it is the tier that proves discrimination.
- This spec restores the fifteen and adds no new scenario. Coverage gaps named in
  `docs/architecture/testing/interop.md` (Long-Lived Graceful Restart with live
  peers) stay open.
- The stale `Audit()` sentence in `docs/architecture/testing/interop.md` is
  corrected here because this work already edits that page. No wider sweep of the
  page's claims is in scope.

## RFC Documentation (Scope: protocol)

The scope is tooling and no protocol code is written, so no new
`// RFC NNNN Section X.Y:` comment lands in product code. The existing
`RFC requirement:` tags above the fifteen checkers ARE the RFC documentation of
this work: each names a requirement id, a polarity, and the assertion that proves
it. AC-10 holds every one of them to its restored body, and lowering a level or
adding a `{gap}` is an owner question (`ai/rules/rfc-compliance.md`).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/le/interoplab/bgp/`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] Goal Validation table complete, with a recorded RED for every group

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (N-A, with the reason in the Functional Tests table)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Seven commits, `fae0f1830` through `dc21e2a5a`, all on `main`.
- `fae0f1830` (AC-1): deleted the always-firing `checkScenario` preamble from
  `checkAddPathReadvertiseCollision` and `checkRFC2545NextHops`, whose complete
  bodies had never executed a line, and wrapped each failure so the error still
  names its scenario.
- `c5b61cf47` (AC-2), `125ab2202` (AC-3), `18017069a` (AC-4), `cb882fd58`
  (AC-5/AC-6): the thirteen delegating bodies, written as lab I/O over pure
  predicates. `internal/le/interoplab/bgp/check_rfc_predicate.go` was created at
  AC-2 and holds every predicate and daemon-output parser (858 lines).
- Zero `checkScenario` delegations remain:
  `grep -n 'checkScenario(ctx, check, "' internal/le/interoplab/bgp/check_rfc.go`
  returns nothing.
- `docs/architecture/testing/interop.md`: the RIB-attach and container-recreation
  subsections written while root-causing `d26692442`, plus the three stale-claim
  repairs this closure found (below).

### Bugs Found/Fixed
- `d26692442`: five scenarios declared `bgp-rs`, `bgp-rib` or `bgp-adj-rib-in`
  and attached none of them to a peer. `Server.PeerScopedProcs`
  (`internal/component/plugin/server/delivery_graph.go`) delivers an event only
  where the plugin's declaration and the peer's `attach process` block agree, so
  the plugin saw no peer and ze forwarded nothing. Every restored assertion in
  those five reads a relay that could not have run. `ef320b724` fixed the same
  defect in two other scenarios in August and the rest were never swept.
- `dc21e2a5a`: `frrDecodeFields` demanded the neighbor address as a whole field,
  and FRR 10.3.1 writes it as `172.30.0.2(Unknown)`, so no decode assertion could
  match a real FRR log. `frrNamesPeer` (`check_rfc_predicate.go`) now accepts the
  address or the address followed by `(`, which keeps 172.30.0.22 from passing as
  172.30.0.2.
- Four `RFC requirement:` tags of roughly twenty examined claimed an assertion no
  body made: `RFC2545-3-3`, `RFC4724-4-1`, `RFC7606-5.1-3`, and the structural
  `gobgp global rib add` omission. Each was repaired by restoring the assertion,
  never by softening the tag.

### Documentation Updates
- `docs/architecture/testing/interop.md`, three repairs made in this closure.
  The page said "`Audit()` retains the exact digest and obligation mapping of the
  removed checker revision", and no `Audit` symbol exists anywhere under
  `internal/le/interoplab/` (`grep -rn 'Audit' internal/le/interoplab/` returns
  nothing). The same sentence produced "their audit rows name each branch and
  mutation separately" and step 4 of "Writing a New Scenario". All three now
  state the shape the code has: a body in `check_rfc.go` that does the lab I/O
  and a pure predicate in `check_rfc_predicate.go`, with a both-polarity subtest
  in `TestBespokeCheckerBranches`.
- `docs/architecture/testing/interop.md`, two subsections written during
  `d26692442`: a scenario that reads the RIB attaches the RIB plugin, and editing
  a scenario config means recreating the container rather than restarting it.
- `docs/functional-tests.md`: no update owed. `grep -n -i 'bespoke\|checkScenario'`
  returns nothing, so the page does not describe the bespoke checker route.
- `rfc/requirements/rfc2545.md`: no update owed. Rows RFC2545-3-2 and RFC2545-3-3
  both cite `check_rfc.go` `checkRFC2545NextHops` as interop/nightly evidence, and
  the restored body now asserts both, so the rows became TRUE without an edit.
- `./le doc check verify`: FAILED on two findings, neither of them this work's.
  `docs/guide/operational-reports.md` names `RaisePrefixStale` and
  `ClearPrefixStale` in a file that declares neither, and `ai/RFC-REQUIREMENTS.md`
  is stale against its sources. No CLAIM names any file this spec changed.

### Deviations from Plan
- The spec planned a `check_rfc.go` under 1000 lines after the predicate split.
  The split happened (`check_rfc_predicate.go`, 858 lines) and `check_rfc.go`
  still measures 1367. What remains is one concern, the tagged checker bodies,
  and `ai/rules/go-standards.md` treats 1000 as the point to LOOK for a second
  concern rather than a limit. No further split is owed.
- `plan/learned/NNN-<name>.md` in the spec's own Closure checklist is superseded
  by the journal row of `/ze-close` step 6a. One row is written, to
  `plan/journal/gate-excludes-part-of-its-population.md`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed each scenario's inputs still described its topology, so a restored assertion would read a working lab | Five scenarios had never forwarded anything: they declared a relay plugin and attached it to no peer, so the assertion had no relay to read | The AC-3 and AC-5 end-to-end runs went red at the route wait, and `show bgp rib status` reported `"peers": 0` beside two Established sessions | Fixed at the source in `d26692442`, and the tell is now written into `docs/architecture/testing/interop.md` |
| assumption | A-5 assumed each `RFC requirement:` tag described an assertion the deleted checker made | Four tags of roughly twenty claimed an assertion no body made | AC-10, reading each tag against its restored body | Each was repaired by restoring the assertion. No tag level was lowered, so no owner question was owed |
| approach | The implementation phase treated `docs/architecture/testing/interop.md` as edited once the RIB-attach subsections landed | The three stale `Audit()` claims the spec named in Files to Modify were still in the page | This closure's documentation review, step 4 | Repaired here. `ai/rules/documentation.md` puts the page edit in the work that broke it, so a doc row named in Files to Modify is checked against the page before the phase closes, not at closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every one of the fifteen carries a bespoke Go body that asserts | Done | `internal/le/interoplab/bgp/check_rfc.go` | `grep -n 'checkScenario(ctx, check, "' check_rfc.go` returns nothing |
| The two guarded bodies execute | Done | `checkAddPathReadvertiseCollision` and `checkRFC2545NextHops` (`check_rfc.go`) | The three-line preamble is gone from both; the failure wrapper keeps the scenario name |
| Each assertion is expressed as a pure predicate over daemon output plus lab I/O | Done | `check_rfc_predicate.go` (858 lines) | Every function there takes text or decoded JSON and holds no lab handle |
| Scope is all fifteen, no trimming | Done | `TestBespokeCheckerBranches` (`bgp_test.go`) | Fifteen scenario subtests, one per AC-2..AC-6 name plus the two AC-1 names |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `fae0f1830`; scenarios `bgp-addpath-readvertise-collision-frr` and `bgp-rfc2545-linklocal-nexthop-frr` pass end to end | Neither reports `has no typed assertions` |
| AC-2 | Done | `c5b61cf47`; `no-family-peer-eor-frr`, `isis-p2p-frr`, `ospf-stub-nssa-frr` | `requireMultiprotocolAdvertisedOnly`, `isisDatabaseNamesZe`, `requireNoExternalLSA` |
| AC-3 | Done | `125ab2202`; `bgp-relay-withdraw-shape-frr`, `bgp-rfc7606-relay-shape-frr`, `bgp-self-nexthop-withheld-frr` | The self-next-hop absence is read out of the SAME log text the control decode was found in |
| AC-4 | Done | `18017069a`; `bgp-rfc7999-blackhole-frr`, `bgp-route-server-frr`, `bgp-wellknown-noexport-frr` | `kernelRouteFor` returns a typed verdict, so an absent route never reads as forwarded |
| AC-5 | Done | `cb882fd58`; `bgp-local-pref-strip-gobgp`, `bgp-med-across-as-gobgp`, `bgp-med-remove-configured-gobgp` | `requireGoBGPAttributeAbsent` separates a stripped attribute from one carrying zero |
| AC-6 | Done | `cb882fd58`; `bgp-rfc7606-typed-nlri-discard` | `requireSpeakerEVPNDiscard` requires the oracle name, Established, one route-bearing UPDATE and one decoded EVPN NLRI before it reads PASS |
| AC-7 | Done | `TestEveryCheckerFailsClosedWithoutPeerEvidence`, `TestRFCInteropCheckerBindings` | Neither test's expectations were edited: the diff touches `TestBespokeCheckerBranches` alone in `bgp_test.go` |
| AC-8 | Done | `TestBespokeCheckerBranches`, fifteen scenario subtests | Each drives its predicate in both polarities |
| AC-9 | Done | `TestCheckerPopulationMatchesProducer` | Unedited, and green in every batch |
| AC-10 | Done | Every `RFC requirement:` tag in `check_rfc.go` read against its body in this closure | Four false tags were found during implementation and repaired by restoring the assertion |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBespokeCheckerBranches`, both polarities per predicate | Done | `internal/le/interoplab/bgp/bgp_test.go` | Fifteen subtests added by this work, named for their scenario |
| `TestCheckerPopulationMatchesProducer` | Done | `bgp_test.go` | Unedited |
| `TestEveryCheckerFailsClosedWithoutPeerEvidence` | Done | `bgp_test.go` | Unedited |
| `TestRFCInteropCheckerBindings` | Done | `bgp_test.go` | Unedited |
| Per-scenario `INTEROP_SCENARIO=<name> ./le integration interop` | Done | all fifteen | 21 to 150 s each once the harness image is built, 353 s on the image-building run |
| Forced-mutation discrimination | Done | AC-3, AC-4, AC-5/AC-6 | AC-3 forced 9+, AC-4 forced 11, AC-5/AC-6 forced 23, zero survivors |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/interoplab/bgp/check_rfc.go` | Done | 1367 lines |
| `internal/le/interoplab/bgp/bgp_test.go` | Done | 1550 lines |
| `internal/le/interoplab/bgp/check_rfc_predicate.go` | Done | Created, 858 lines |
| `docs/architecture/testing/interop.md` | Done | Five edits: two from `d26692442`, three stale-claim repairs here |
| `rfc/requirements/rfc2545.md` | Changed | No edit owed: both rows already cite `checkRFC2545NextHops`, and the restored body made them true |
| `docs/features/rfc-status.md` | Changed | No edit owed: the RFC 4724, RFC 7999 and OSPF rows already describe what the restored bodies now assert |

### Audit Summary
- **Total items:** 26
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (both recorded in Deviations and in Files from Plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The fifteen scenarios assert what their names claim | interop | All fifteen pass end to end against live FRR 10.3.1, GoBGP 3.31.0 and BIRD 2.x, run per AC group with `INTEROP_SCENARIO=<name> ./le integration interop`. Category 3 (ze non-conformance): none found |
| Each restored checker DISCRIMINATES rather than merely passing | forced mutation | One mutation at a time, each read and each file restored `cmp`-identical: AC-3 forced 9+ reds, AC-4 forced 11, AC-5/AC-6 forced 23, with 0 survivors. AC-5/AC-6 also found that AC-3's and AC-4's own base-network guards had never been mutation-tested, and added `checkerGuardFailure` to drive all five guarded bodies through their real entry point |
| The RFC citations that name these checkers become true | interop + read | `rfc/requirements/rfc2545.md` rows RFC2545-3-2 and RFC2545-3-3 name `check_rfc.go` `checkRFC2545NextHops` as interop/nightly evidence, and that function now executes assertions 4, 5 and 6. The `rfc1997` and `rfc7999` rows of `rfc/enrolled.txt` are now true, and so are the RFC 7999 row of `docs/features/rfc-status.md` and the three claims in `docs/architecture/bgp/egress-attribute-rules.md`, each of which was false before |
| The repair does not weaken the gate it repairs | unit | `TestCheckerPopulationMatchesProducer`, `TestEveryCheckerFailsClosedWithoutPeerEvidence` and `TestRFCInteropCheckerBindings` pass with no edit to their expectations. The `bgp_test.go` diff carries two hunks and both fall inside `TestBespokeCheckerBranches` |
| The gate that governs a weakened RFC-tagged test agrees | audit | `./le commit audit base fae0f1830~1` reports 23 findings and names no file under `internal/le/interoplab/`. Every RFC-tagged change this work made carries its owner-approval row in `test/rfc-changed.md` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard exists | done | The spec's metadata declares `Deferral shard: -`, and `ls plan/deferrals/ \| grep restore-bespoke` returns nothing. No `remove` is owed, and no foreign shard was emptied by this closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/restore-bespoke-interop-assertions-5286b836-8702-47bc-a370-6034e573c3d9.md`, written by `./le spec session review record` over 10 files (3 of them code) |
| `./le spec session review check` | `review_gate: OK (3 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 2. Round 1 read the complete seven-commit diff and found one ISSUE. Round 2 read the fix and found nothing |
| Reviewer lenses used | wiring and functional coverage, documentation drift, removed-behavior audit, logic and edge cases, security, `docs/contributing/ze-go-style.md` style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The page claims `Audit()` retains the digest and obligation mapping of the removed checker revision, and asks for a "source-contract audit row" as step 4 of writing a scenario. No `Audit` symbol is declared anywhere under `internal/le/interoplab/`, so both claims send a reader to a symbol that does not exist and neither describes the shape the fifteen restored checkers have | `docs/architecture/testing/interop.md`, its "Scenario Structure", "Querying Ze" and "Writing a New Scenario" sections | Three edits in this closure, replacing both claims with the body-plus-predicate shape and the `TestBespokeCheckerBranches` both-polarity obligation, each carrying a source anchor `./le doc check verify` accepted |

### Notes recorded, not blocking
| # | Finding | Location |
|---|---------|----------|
| 1 | `requireRouteInstalledVia` matches the prefix and the next hop with `strings.Contains` on one line, while `frrDecodeFields` in the same file matches whole fields and its comment says why. The substring form is required for the next hop, which FRR renders with a trailing comma (`via 172.30.0.9, eth0`). A false pass needs a route whose text carries the wanted prefix as a substring, and the three call sites pass constants (`2001:db8:5601::/48`, `203.0.113.0/24`, `0.0.0.0/0`) into labs that hold no such route | `requireRouteInstalledVia` (`check_rfc_predicate.go`) |
| 2 | `seenGlobal = address == global` assigns rather than accumulates, so a second global-scope entry would overwrite the first. The assignment can only turn the flag FALSE, and the entry-count guard above rejects the shape that would reach it, so it fails closed | `requireNextHopShape` (`check_rfc_predicate.go`) |
| 3 | `check_rfc.go` measures 1367 lines, past the 1000-line mark. The second concern was already separated into `check_rfc_predicate.go`; what remains is the tagged checker bodies, which are one concern | `check_rfc.go` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/interoplab/bgp/check_rfc_predicate.go` | yes | `ls -la` reports 34691 bytes; `wc -l` reports 858 |
| `internal/le/interoplab/bgp/check_rfc.go` | yes | `wc -l` reports 1367 |
| `internal/le/interoplab/bgp/bgp_test.go` | yes | `wc -l` reports 1550 |
| `docs/architecture/testing/interop.md` | yes | Read in full over its "Scenario Structure", "Querying Ze" and "Writing a New Scenario" sections while making the three repairs |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | No body reaches `checkScenario` | `grep -n 'checkScenario(ctx, check, "' internal/le/interoplab/bgp/check_rfc.go` exits 1 with no output, run in this closure |
| AC-1 | Both formerly guarded bodies execute their own assertions | Read in this closure: `checkAddPathReadvertiseCollision` runs assertions 1 to 12 and `checkRFC2545NextHops` runs 1 to 7, neither behind a preamble |
| AC-2 to AC-6 | Each of the fifteen has a body that asserts | `check_rfc.go` read in full in this closure: each body numbers its assertions through a `fail(assertion int, cause error)` closure over `checkerFailure` |
| AC-7 | The three fail-closed and binding tests are unedited | `git diff fae0f1830~1 dc21e2a5a -- internal/le/interoplab/bgp/bgp_test.go` carries two hunks, both inside `TestBespokeCheckerBranches` |
| AC-8 | Fifteen both-polarity subtests exist | `grep -n 't.Run('` over the body of `TestBespokeCheckerBranches` lists all fifteen scenario names |
| AC-9 | No bespoke name has a generic fallback | Same grep as AC-1, plus `TestCheckerPopulationMatchesProducer` unedited |
| AC-10 | Every tag is true of the body under it | Every `RFC requirement:` tag in `check_rfc.go` read against its assertions in this closure: RFC7911-2-2, RFC4271-5.1.5-2, RFC4271-5.1.4-1 (both), RFC4271-5.1.4-4 (both), RFC4271-5.1.2-3 (both), RFC2545-3-2 and 3-3 (three), RFC7606-5.1-3, RFC7606-5.4-1, RFC7999-3.3-1 and 3.3-2 (four), RFC9234-5-4 (both), RFC7947-x-1, RFC4271-5.1.3-1 (both), RFC1997-Well-1 (both), RFC5301-3-4 and 3-6, RFC4724-4-1, RFC3101-2.4-5 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `INTEROP_SCENARIO=bgp-addpath-readvertise-collision-frr ./le integration interop` | `TestRFCInteropCheckerBindings`, `TestBespokeCheckerBranches/bgp-addpath-readvertise-collision-frr` | yes: subtest and checker `checkAddPathReadvertiseCollision` both read; scenario passes end to end |
| `INTEROP_SCENARIO=bgp-rfc2545-linklocal-nexthop-frr ./le integration interop` | `TestBespokeCheckerBranches/bgp-rfc2545-linklocal-nexthop-frr` | yes: `checkRFC2545NextHops` read; scenario passes end to end |
| `INTEROP_SCENARIO=no-family-peer-eor-frr ./le integration interop` | `TestBespokeCheckerBranches/no-family-peer-eor-frr` | yes: `checkNoFamilyEndOfRIB` read |
| `INTEROP_SCENARIO=bgp-rfc7999-blackhole-frr ./le integration interop` | `TestBespokeCheckerBranches/bgp-rfc7999-blackhole-frr` | yes: `checkRFC7999Blackhole` asserts the Linux FIB in the ze container |
| `INTEROP_SCENARIO=bgp-med-across-as-gobgp ./le integration interop` | `TestBespokeCheckerBranches/bgp-med-across-as-gobgp` | yes: `checkMEDAcrossAS` read |
| `INTEROP_SCENARIO=bgp-rfc7606-typed-nlri-discard ./le integration interop` | `TestBespokeCheckerBranches/bgp-rfc7606-typed-nlri-discard` | yes: `checkRFC7606TypedNLRIDiscard` read |
| `./le integration interop` (whole suite) | `TestCheckerPopulationMatchesProducer` | yes: unedited, and no bespoke name appears in `scenarioOperations` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `git show a374622db~1:test/interop/scenarios/<name>/check.py` returned a checker for each of the fifteen, and each restored body carries the assertion it encoded |
| A-2 | broken | Five scenarios declared a relay plugin and attached it to no peer, so their inputs did NOT describe a working topology. Fixed at the source in `d26692442`; Mistake Log row 1 |
| A-3 | confirmed | Deleting the guard exposed working assertions in both bodies. Both scenarios pass end to end |
| A-4 | confirmed | `TestRFCInteropCheckerBindings` passes unedited over every restored body, so wrapping with `checkerFailure` preserved the scenario name in the error text |
| A-5 | broken | Four tags of roughly twenty claimed an assertion no body made. Each repaired by restoring the assertion, never by lowering a level. Mistake Log row 2 |
| A-6 | confirmed | Every function in `check_rfc_predicate.go` takes text or decoded JSON and holds no lab handle, so `TestBespokeCheckerBranches` runs all fifteen with no container |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #9 RFC behavior newly proven | `rfc/requirements/rfc2545.md` rows RFC2545-3-2 and RFC2545-3-3 already cite `checkRFC2545NextHops`; the restored body asserts both. The `rfc1997` and `rfc7999` rows of `rfc/enrolled.txt` describe what `checkNoExportBoundary` and `checkRFC7999Blackhole` now assert | yes, no edit owed |
| #10 Test infrastructure changed | `grep -n -i 'bespoke\|checkScenario' docs/functional-tests.md` returns nothing, so that page does not carry the route. `docs/architecture/testing/interop.md` carried the stale `Audit()` claim and its two descendants, repaired here | yes, repaired |
| #12 Internal architecture changed | `docs/architecture/testing/interop.md` now documents the body-plus-predicate shape as the way a bespoke checker is written, with anchors to `check_rfc.go`, `check_rfc_predicate.go` and `bgp_test.go` | yes |
| #15 Registered inventory changed | `specialCheckers` keeps the same fifteen names; `check_special.go` is untouched by this work | yes, no edit owed |
| #16 Source anchors on changed files | `./le doc check verify` raises no CLAIM against any file this spec changed. Its two failures name `docs/guide/operational-reports.md` and a stale `ai/RFC-REQUIREMENTS.md`, both other sessions' | yes |
| #17 Existing examples for this area | The "Running" and "Writing a New Scenario" sections were read while making the repairs; every scenario name they print still exists | yes |
| Doctor checks | No runtime dependency is added. Docker is an existing prerequisite of `./le integration interop` | yes, no check owed |

### Verification Not Run (owner instruction, 2026-08-31)
Thomas directed this closure to run WITHOUT verification. `./le verify worktree`
and `./le verify lint run` were NOT run, and no check whose result was already
recorded was re-run. The evidence above is attributed to where it came from: the
implementation phase's own runs for the unit tier and the interop tier, and this
closure for every grep, read and audit it names. A peer's
`internal/component/l2tp/plugins/authradius/acct.go` does not compile in this
working tree (`unknown field interval in struct literal of type acctSession`),
which is another session's work and would have reddened a lint run regardless.
The closure commits therefore carry a verification-debt row each, in
`plan/verification-debt/`, and no push is ordered.

## Core Insight

An RFC tag can promise more than its body asserts, and no gate in the repository
can see the gap. `TestCheckerPopulationMatchesProducer` exists to refuse a
bespoke checker that also holds a generic fallback, and it reads membership of
`scenarioOperations`. The fallback here lived INSIDE the checker body, which is
outside the population that gate reads, so fifteen checkers carrying twenty-odd
RFC requirement tags sat behind a call that could only ever return an error. The
tag is a claim by its author on the day it was written; only the body is
evidence.
