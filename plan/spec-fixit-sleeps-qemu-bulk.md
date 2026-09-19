# Spec: fixit-sleeps-qemu-bulk

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | groups A + B: Linux-gated readiness/reload waits; group C remains outside this spec |
| Depends | - |
| Phase | - |
| Updated | 2026-09-19 |

The July inventory below is historical. The parent closed on September 5 after
the embedded `.ci` sleep count reached zero, but a zero at that syntax does not
prove that the native replacement waits on an observable effect. Current
source still gives this spec an in-scope remainder, so it stays open for a
current inventory and design.

## Provenance

Reclassified as an improvement on 2026-08-14 at Thomas's instruction and moved
from `plan/` to `plan/future/`. Reason: these are blind sleeps in QEMU-gated
tests. A sleep is a wait, and the assertion after it still runs, so no green is
hidden by leaving them.

## Task

Replace the remaining blind readiness and reload holds in the Linux-gated
tests with waits on the effects those tests assert. The scope remains groups
A and B from the July inventory, including their native fixture replacements.
The current producers are in `internal/test/fixture`, rather than embedded
Python inside the `.ci` files.

Two original group-A consumers demonstrate the remainder:
`traffic-boot-qdisc-tc.ci` calls a fixture registered as
`sleepFixture(2500*time.Millisecond)`, and `traffic-reload-qdisc-tc.ci` calls
`trafficReloadQdisc`, which waits 500ms before the first kernel read and 2s after
SIGHUP before the second. `sleepContext` completes on a timer or cancellation;
it observes no qdisc state. These are source findings, with no new QEMU run.

Research must account for every originally in-scope test against its current
consumer and producer, separating completed conversions from retained blind
holds, bounded condition waits and deliberate protocol timers. A disappeared
Python call alone is insufficient. Each remaining conversion still owes a
passing original and a passing changed test under a Linux/QEMU route that
actually runs it, with all assertions preserved. No closed sibling is a current
handoff target, and no retired helper is to be restored.

## Origin

Carved out of `spec-fixit-migrate-sleeps-infra` (the umbrella, CLOSED and removed
from the tree on 2026-09-05 with the `test/**/*.ci` sleep count at 0), whose Design
Insights name a "QEMU-gated needs-linux bulk (~150)" that the darwin dev host cannot
verify, and whose Implementation Summary states no further clean host-verifiable blind
sleeps remain. Skeleton written 2026-07-15 alongside `spec-fixit-sleeps-cli-harness`.

## Required Reading

### Current documentation and producers
- `docs/architecture/testing/ci-format.md`: daemon readiness, `await=stderr`, and Linux gating.
- `docs/functional-tests.md`: the current Linux/QEMU execution routes.
- `ai/rules/platform-linux.md` and `ai/rules/testing.md`: proof on Linux and assertion preservation.
- `internal/test/fixture/netfilter_fixture.go`: registrations, `waitDaemon` and `sleepContext`.
- `internal/test/fixture/netfilter_fixture_traffic.go`: `sleepFixture` and `trafficReloadQdisc`.
- `internal/test/runner/await_stderr.go`: `awaitDaemonStderr`, already consumed by the listener-refusal tests.
- `test/.ci-sleep-baseline`: the historical embedded-Python ratchet. Its ceiling does not count Go fixture timers.

### Architecture Docs
- [ ] `ai/rules/platform-linux.md` - QEMU integration is mandatory for linux-only code; never skip for "needs hardware".
  → Constraint: this rule is the spec's completion criterion. Conversion is not done; QEMU re-verification is done.
- [ ] `ai/rules/testing.md` - the comment-on-every-sleep rule.
  → Constraint: a converted sleep removes its comment with it; a KEPT sleep must keep justifying itself or the gate fails on the changed file.
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` directive catalog (`option=needs-linux`, `option=skip-os`, `expect=`).

## Current Behavior (MANDATORY)

`test/traffic/traffic-boot-qdisc-tc.ci` starts a background daemon, runs
`ze-test fixture traffic/traffic-boot-qdisc-tc`, then reads `tc qdisc show`.
Its fixture is a 2.5s timer. `trafficReloadQdisc` reads the qdisc before and after
SIGHUP, but reaches each read after a fixed hold. Both remain in the original
Linux-gated population.

`test/install/dhcp-zero-listener.ci` instead uses
`await=stderr:contains=no interfaces bound`; `awaitDaemonStderr` waits for the
relayed output and fails on its context bound. This is a converted consumer,
not another missing readiness mechanism. `policy-reload.ci` now calls the
native `policy/policy-reload` fixture and is marked `needs-linux`; the July
group-B gate classification therefore cannot serve as today's run list.

A source search on September 20 found no `time.sleep(` in `test/**/*.ci`.
The baseline file still carries the composable deltas and historical conversion
notes, including waits moved into Go helpers. Neither observation is a current
QEMU pass or a proof that every remaining wait is deterministic.

**Behaviour to preserve:** every effect assertion, Linux execution proof,
already-bounded condition wait and deliberate timer covered by AC-1 through
AC-9. Group C remains outside this spec. No production behaviour changes
beyond test-support surfaces justified by the existing scope.

**Behaviour to change:** the in-scope native blind holds must complete on the
asserted effect. Research must name any missing observable completion fact
before design; it must not reintroduce the retired Python migration recipe.

## Historical Problem / Evidence (July 2026)

**CONFIRMED (measured 2026-07-15 by reading and grepping the tree):**

Total `time.sleep(` in `test/**/*.ci` was 126 as measured 2026-07-15, matching the then-baseline; `test/.ci-sleep-baseline` rose to 132 as sibling specs landed, and now sums to 125 (2026-07-22; re-measure the tree at Phase 2).
Of those, 51 carry the "bounded wait not a blind sleep" annotation (already deterministic
poll intervals), leaving 75 non-poll sleeps. 51 of the 75 are explicitly annotated
`# blind hold:` (38) or `# blind settle:` (13); the remaining 24 carry other
justifications (deliberate timers, raw-protocol pacing).

Gate taxonomy across ALL files containing sleeps (blind = raw minus bounded polls):

| Group | Gate | Reachable by | Raw | Blind |
|-------|------|--------------|-----|-------|
| A | `option=needs-linux` | `./le qemu run command "./le qemu all-tests"` | 14 | **9** |
| B | `option=skip-os:value=darwin` only | NOT that target (filtered at `record_parse.go`); needs `ze-qemu-test-all` | 31 | **12** |
| C | no OS gate | the darwin host directly | 81 | **54** |

Group A files (raw/blind): `install/dhcp-zero-listener.ci` 1/1, `install/tftp-zero-listener.ci` 1/1,
`plugin/ddos-detect-characterize.ci` 3/1, `plugin/ddos-detect-external-warns.ci` 1/1,
`plugin/ddos-detect-mitigate.ci` 2/0, `plugin/flowexport-external-refuses.ci` 1/1,
`plugin/trafficusage-external-refuses.ci` 1/1, `traffic/traffic-boot-qdisc-tc.ci` 1/1,
`traffic/traffic-reload-qdisc-tc.ci` 3/2.

Per-directory raw vs blind (the brief's numbers are the raw column):

| Dir | Raw | Blind | Note |
|-----|-----|-------|------|
| traffic | 16 | 14 | only 022 (1/1) and 023 (3/2) are `option=needs-linux`; the other 12 raw are UNGATED (vpp-stub tests, run on darwin) |
| policy | 13 | 7 | all `skip-os:value=darwin` (group B) |
| firewall | 8 | 2 | all group B; 6 of the 8 raw are bounded polls |
| flow-export | 6 | 4 | all UNGATED (group C) |
| ospf | 4 | 1 | all group B |
| pppoe | 3 | 1 | group B |
| install | 3 | 3 | 2 are `needs-linux`; `image-resolve-failure.ci` is ungated |
| reload | 2 | 1 | both UNGATED |
| ui | 1 | **0** | its single sleep is a bounded poll: nothing to convert |

**The three corrections that matter for scope:**
1. Only 9 blind sleeps are actually reachable by `./le qemu run command "./le qemu all-tests"`. The brief's implied scope (~56 raw across those dirs) is roughly 6x the target-reachable blind population.
2. The policy/firewall/ospf/pppoe bulk (group B, 12 blind) is linux-only but NOT run by the needs-linux target. Verifying it needs `./le qemu run command "./le qemu all-tests"`, or those tests need `option=needs-linux` added. Unresolved fork, see Open Questions.
3. flow-export (4 blind), reload (1 blind), `install/image-resolve-failure.ci` (1 blind) and the 12 ungated traffic vpp-stub sleeps have NO OS gate, so they are host-runnable on darwin and do not belong in a QEMU-gated spec by its own criterion. Either they are misfiled here, or they are missing a gate they should have.

→ AUTONOMOUS DEFAULT (2026-07-17) [scope]: This spec is scoped to **groups A + B** — the needs-linux + skip-os:darwin linux-only blind sleeps (A = 9 blind, reachable by the retired `ze-qemu-needs-linux-test` (current: `./le qemu run command "./le qemu all-tests"`); B = 12 blind, reachable only by the retired `ze-qemu-test-all` (current: `./le qemu run command "./le qemu all-tests"`); measured taxonomy verified 2026-07-17: group A raw 14, group B raw 31, tree total 132 -- baseline now 125, 2026-07-22; re-derive the taxonomy at Phase 2). **Group C** (54 blind, ungated, host-runnable on darwin) is OUT OF SCOPE and recorded as a noted follow-up (see Future + Open Questions → Resolutions). Rationale: smaller, self-contained scope per the readiness decision protocol — a QEMU-gated spec should own only the sleeps its QEMU criterion can verify; the ungated group C is host-verifiable and belongs to a host-runnable (or gate-audit) spec that decides per file whether each C test is misfiled or missing a gate. Thomas: override if wrong.
→ AUTONOMOUS DEFAULT (2026-07-17) [scope]: Group B verification fork (correction 2) resolves in favour of the retired `ze-qemu-test-all` (current: `./le qemu run command "./le qemu all-tests"`) as the DEFAULT branch (it already runs the skip-os:darwin group; zero committed change). Adding `option=needs-linux` to the 15 group B files — which would pull the 12 blind into the fast `ze-qemu-needs-linux-test` loop — is the more-reversible alternative, recorded as an OPTIONAL follow-up (a test-metadata change with its own review), adopted only if full-suite turnaround proves impractical during implement. Rationale: the smaller self-contained default mutates zero committed metadata; both branches remain valid "done" states under AC-8. Thomas: override if wrong.

**UNVERIFIED:**
- That the ORIGINAL (unconverted) tests currently pass under QEMU. Not run: this host does not run QEMU, and this skeleton ran no tests. The confirm-original step is AC-1 precisely because it is unverified.
- Whether the group B / group C classification reflects intent or drift (a test may be missing an `option=needs-linux` it deserves).
- The umbrella's "~150 QEMU-gated" figure. The measured linux-gated blind population (groups A+B) is 21, an order of magnitude smaller. The umbrella figure appears to count raw sleeps at an earlier baseline (246), before the conversions it records.

**Scope update (2026-07-16, sibling hand-offs):** R-5 hands the 3 external-warn group A files (`ddos-detect-external-warns.ci`, `flowexport-external-refuses.ci`, `trafficusage-external-refuses.ci`) to `plan/spec-fixit-reject-fence-observability.md`; R-8 hands the ddos-detect-mitigate files to `plan/spec-fixit-ddos-test-infra.md`, which now carry 0 blind annotations, so this spec has nothing to convert there. After both hand-offs the cleanly-owned convertible sleeps shrink to about 5: `traffic/traffic-boot-qdisc-tc.ci`, `traffic/traffic-reload-qdisc-tc.ci` (2 sleeps), `install/dhcp-zero-listener.ci`, `install/tftp-zero-listener.ci`. At Phase 2 the residual population must be re-measured to confirm a standalone spec is still justified, versus folding the QEMU-verification of these ~5 sleeps into the sibling specs. <!-- doc-links: ignore (spec closed and removed) -->

## Data Flow (MANDATORY)

### Entry Point
A Linux-gated `.ci` invokes its native fixture before reading the kernel state
or asserting a reload result.

### Transformation Path
1. Discover the current `.ci` consumer and the registered native fixture.
2. Trace the wait to the fact it reads, or record that it only reads a timer.
3. Replace a remaining blind hold with completion on the asserted effect.
4. Keep the existing assertions and prove the conversion on Linux/QEMU.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner to native fixture | `ze-test fixture` | source read only |
| fixture to daemon | readiness and reload completion | current inventory and runtime proof owed |
| fixture to Linux kernel | tc/nft readback | current inventory and runtime proof owed |
| host to Linux/QEMU | a route that includes each consumer without skipping it | runtime proof owed |

### Integration Points
- The original groups A and B, through their current `.ci` and native fixture paths.
- `test/.ci-sleep-baseline` only if a conversion removes counted embedded sleeps; moving a Go timer does not lower that count.
- Existing readiness and completion producers, reused before adding test-support machinery.

### Architectural Verification
- [ ] No bypassed layers (each wait polls the real effect it asserts).
- [ ] No unintended coupling (test-support surfaces stay additive).
- [ ] No duplicated functionality (reuse existing readbacks and the existing readiness barrier before adding any).
- [ ] Registration over hardcoding: any new wait surface or reflecting query registers and is core-discovered, never a per-test special case wired into a shared package.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| backgrounded ze reaches OnConfigure | -> | readiness signal pollable by the driver (runner-level or marker-based) | `test/traffic/traffic-boot-qdisc-tc.ci` (QEMU) |
| driver sends SIGHUP for reload | -> | reload-completion signal the driver can poll | `test/traffic/traffic-reload-qdisc-tc.ci`, `test/policy/policy-reload.ci` (QEMU) |
| tc qdisc programmed by OnConfigure | -> | tc readback poll replacing the blind hold | `test/traffic/traffic-boot-qdisc-tc.ci` (QEMU) |
| dhcp/tftp zero-listener path | -> | deterministic wait on the asserted listener state | `test/install/dhcp-zero-listener.ci`, `test/install/tftp-zero-listener.ci` (QEMU) |
| ddos characterize pipeline | -> | deterministic wait on the characterization result | `test/plugin/ddos-detect-characterize.ci` (QEMU) |
| a counted embedded sleep is removed from a `.ci` | -> | the existing sleep ratchet | lower `test/.ci-sleep-baseline` by the removed count; native timer removal alone earns no delta |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Before converting any test | The ORIGINAL test is confirmed green under QEMU, so a post-conversion failure is unambiguously the conversion's fault |
| AC-2 | A blind hold on a background daemon's readiness (the ZE_READY_FILE shape) | Replaced by a deterministic wait on a real signal (the asserted OnConfigure log line, or a readiness marker made available to backgrounded ze), bounded, with a timeout that names what it waited for |
| AC-3 | A pre-SIGHUP blind settle / post-SIGHUP blind hold, including its native replacement | Replace it with an observable reload-completion wait, or identify the missing completion fact as an unresolved in-scope requirement; an infra-gated item remains open here until a live owner is named |
| AC-4 | Each converted test, after conversion | RE-VERIFIED green under QEMU. Never marked done on a darwin skip alone (`ai/rules/platform-linux.md`, umbrella R-2) |
| AC-5 | A sleep annotated "bounded wait not a blind sleep" | Left unchanged; it is already deterministic |
| AC-6 | A deliberate timer (traffic/012, traffic/026: the 5s vpp `WaitConnected` timeout IS the behavior under test) | Kept, justifying comment intact, `check_ci_sleep_justification` green |
| AC-7 | A conversion removes sleeps counted by the embedded `.ci` ratchet | Lower `test/.ci-sleep-baseline` by exactly the number removed in the same change. Native fixture timers are outside that count and cannot justify a delta |
| AC-8 | A current Linux-gated test in scope, including a former group-B member | Verify through a current Linux/QEMU route that actually runs it, recording any gate change. A skipped case is never evidence |
| AC-9 | Full suite after all conversions | `./le qemu run command "./le qemu all-tests"` green, no test converted-but-unverified, no regression in the affected suites |

## Historical Risks & Assumptions (July 2026; re-derive during current inventory)

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The originals currently pass under QEMU | they are committed and gated, not quarantined (no skip/xfail marker found) | AC-1 fails first and this becomes a test-repair spec, not a conversion spec | run `./le qemu run command "./le qemu all-tests"` before touching anything | unvalidated |
| A-2 | A backgrounded ze can be given a pollable readiness signal | the annotation says the ZE_READY_FILE marker is absent for backgrounded ze, not that readiness is unobservable; the asserted OnConfigure log line already exists as a signal | AC-2 collapses to "wait on the log line" only, or needs a production readiness surface (scope grows) | read the ZE_READY_FILE producer and the runner's background-spawn path | unvalidated |
| A-3 | SIGHUP reload completion is observable without new production code | the tests already assert a post-reload log line | AC-3 needs a new reload-processed signal, overlapping `plan/spec-fixit-reject-fence-observability.md` | read the SIGHUP handler and its emitted events | unvalidated | <!-- doc-links: ignore (spec closed and removed) -->
| A-4 | Group B tests are correctly gated with `skip-os` rather than `needs-linux` | they are linux-only in substance (nft/tc/kernel) | adding `option=needs-linux` is a cheap fix bringing 12 blind sleeps into the fast QEMU loop | compare a group A and a group B test's requirements | unvalidated |
| A-5 | Converting a blind hold to a log-line wait is not vacuous | the log line is already the asserted effect | the wait passes before the effect lands, giving a false green | assert the effect (kernel readback), not only the log line, where possible | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A linux-only conversion is claimed done on a darwin skip | the test "passes" on darwin instantly with a skip reason | MANDATORY QEMU run per test (`ai/rules/platform-linux.md`); AC-4 makes re-verification the completion criterion, not conversion |
| R-2 | "Verified" via a target that silently skipped the test (group B under `ZE_QEMU_LINUX_ONLY=1`) | the QEMU run reports the test skipped, not passed, and the summary is not read closely | AC-8; always read the skip count in the QEMU summary, never just the exit code |
| R-3 | A converted test flakes under QEMU's slower timing | intermittent red | investigate the race at the source; NEVER re-add a sleep (`ai/rules/completion.md`) |
| R-4 | Converting a bounded poll or a deliberate timer for baseline credit | the diff touches sleeps whose comments say "bounded wait" or "IS the behavior under test" | AC-5/AC-6; the ratchet rewards removal, so guard against gaming it |
| R-5 | Overlap with `plan/spec-fixit-reject-fence-observability.md` | two specs edit the same external-warn tests | 3 group A files (`ddos-detect-external-warns`, `flowexport-external-refuses`, `trafficusage-external-refuses`) are named by the umbrella as infra-gated reject-fence cases; confirm ownership before touching them | <!-- doc-links: ignore (spec closed and removed) -->
| R-8 | Overlap with `plan/spec-fixit-ddos-test-infra.md` (sibling skeleton, created 2026-07-15) | both specs edit `test/plugin/ddos-detect-mitigate.ci` | that spec's Problem A REWRITES that file onto the `ze_api` observer pattern, changing its sleep count. It has blind=0 today, so this spec has nothing to convert in it: leave it alone and let the ddos spec own it. Re-measure the baseline after that spec lands | <!-- doc-links: ignore (spec closed and removed) -->
| R-6 | QEMU turnaround makes per-test iteration slow | the loop drags | use `./le qemu run command "..."` for single-test iteration; batch the final verification |
| R-7 | Touching a `.ci` file makes the session own every sleep in it (justification gate is changed-file scoped) | the gate fails on sleeps the session did not add | expect it; justify or convert the neighbours in the same file |

## Historical Test Design (July 2026; current cases must be rebound to native producers)

Migration-adapted: each converted `.ci` IS its own functional test and keeps its exact
assertions. Unit tests apply only if research adds runner/production infrastructure.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| needs-linux gate parsing | `internal/test/runner/record_parse_test.go` | `option=needs-linux` sets `NeedsLinux`; `skip-os:value=darwin` does NOT (the group A/B distinction this spec rests on) | pending |
| backgrounded-ze readiness signal (only if A-2 leads to a runner change) | `internal/test/runner/` (file per research) | a backgrounded daemon exposes a pollable readiness marker; bounded, times out naming the signal | pending |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Notes |
|-------|-------|-------|
| any new wait timeout | bounded, > 0 | must time out with a message naming the awaited signal; no unbounded loop |
| `test/.ci-sleep-baseline` | non-negative integer, monotonically decreasing | the ratchet rejects any increase |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `traffic-boot-qdisc-tc.ci` | `test/traffic/` | tc qdisc programmed at boot; blind hold -> deterministic wait | pending (QEMU) |
| `traffic-reload-qdisc-tc.ci` | `test/traffic/` | SIGHUP reload re-applies tc; settle + hold converted | pending (QEMU) |
| `policy-reload.ci` | `test/policy/` | SIGHUP reload re-applies nft policy routes | pending (QEMU, group B) |
| `dhcp-zero-listener.ci`, `tftp-zero-listener.ci` | `test/install/` | zero-listener path | pending (QEMU) |
| `ddos-detect-characterize.ci` | `test/plugin/` | ddos characterization | pending (QEMU) |
| `traffic-vpp-not-connected.ci`, `traffic-vpp-accept-multiclass.ci` | `test/traffic/` | deliberate 5s `WaitConnected` timer: unchanged, kept as control that AC-6 was honored | keep |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Notes |
|----------|-------|
| N/A | test infrastructure only; no wire-protocol behavior changes |

### Future (if deferring any tests)
- Group B (12 blind) may split into its own change if the `option=needs-linux` regating (A-4) is approved separately.
- Group C (54 blind, ungated) is likely out of scope for this spec entirely; see Open Questions.
  → AUTONOMOUS DEFAULT (2026-07-17) [scope]: CONFIRMED out of scope. Group C is neither converted nor QEMU-verified by this spec. Follow-up: a separate host-runnable (or gate-audit) spec decides, per file, whether each ungated blind sleep is misfiled into a QEMU context or is missing an `option=needs-linux`/`skip-os` gate it deserves. Thomas: override if wrong.

## Files to Modify

- `internal/test/fixture/netfilter_fixture.go` and `netfilter_fixture_traffic.go`: the confirmed native blind holds for the original Linux-gated traffic consumers.
- The current `.ci` consumers in groups A and B, only where completing the wait requires an invocation change; preserve all effect assertions.
- `docs/architecture/testing/ci-format.md` if the approved design changes a test-support contract.
- `test/.ci-sleep-baseline` only if a counted embedded sleep is removed.

The inventory must name any additional current producer before scheduling its
edit. It must not expand this spec into the separate load-independence or
default-peer-hold features.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Test infra docs | only if a new wait surface lands | `docs/architecture/testing/ci-format.md` |
| Discovery updates | only if a new primitive/gate lands | `ai/INDEX.md` per `ai/rules/repo-maintenance.md` |
| QEMU verification | yes (the spec's whole point) | `./le qemu run command "./le qemu all-tests"` |
| Ratchet | only for removal of counted embedded sleeps | `test/.ci-sleep-baseline` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File |
|---|----------|----------|------|
| 8 | Plugin SDK/test-support changed? | only if a new wait surface lands | `ai/rules/testing.md`, `docs/functional-tests.md` |
| 10 | Test infrastructure changed? | likely (readiness signal / regating) | `docs/architecture/testing/ci-format.md` |

## Files to Create
- None identified. The current inventory and design must establish any need.

## Implementation Steps

1. **Current inventory.** Rebind every original group-A/group-B consumer to its current producer and gate. Record converted, blind, condition-wait and deliberate-timer cases separately. Preserve the July tables as provenance, never as the execution population.
2. **Design the remainder.** For each native blind hold, identify the observable readiness/reload fact and the existing producer. A missing fact remains an open requirement here; closed reject-fence and ddos specs cannot accept a handoff.
3. **Linux baseline (AC-1, AC-8).** Before changing a remaining case, run its original through a current Linux/QEMU route and record pass, failure and skip counts. Do not convert a red or skipped case as though it supplied a baseline.
4. **Convert and prove (AC-2 through AC-7).** Preserve the asserted effects, replace only the blind wait, prove the failure path discriminates, and rerun each converted case under Linux/QEMU. Keep already-deterministic waits and deliberate timers.
5. **Population proof (AC-9).** Run the affected Linux suites and the full applicable QEMU population, accounting for every original requirement. No closure follows merely from the `.ci` sleep count being zero.

### Critical Review Checklist (/implement stage 6)
| Check | For this spec |
|-------|---------------|
| Assertions preserved | every converted test keeps its `expect=`/fatal checks |
| QEMU-verified | no linux-only conversion claimed done on darwin-skip alone (R-1); skip counts read, not just exit codes (R-2) |
| Non-vacuous | the wait reads the asserted effect, rather than a proxy that can precede it (A-5) |
| Bounded condition waits untouched | no conversion merely for ratchet credit; the July count of 51 is historical, and the current inventory must identify the surviving cases (AC-5) |
| Deliberate timers kept | traffic/012, traffic/026 unchanged and still justified (AC-6) |
| Registration over hardcoding | any new wait surface registers and is core-discovered, not hardcoded into a shared package |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification |
|-------------|--------------|
| Each converted test | its producer completes on the asserted effect; QEMU green before and after, with a discriminating failure case |
| Ratchet accounting | a delta only for counted embedded sleeps removed; no credit for moving or removing a Go timer |
| No regressions | `./le qemu run command "./le qemu all-tests"` + affected suites |

### Security Review Checklist (/implement stage 11)
| Check | Notes |
|-------|-------|
| Input validation | any new predicate/pattern bounded (timeout/attempts); test-only surface |
| Resource exhaustion | no unbounded wait loop; every poll bounded and named |

### Failure Routing
| Failure | Route To |
|---------|----------|
| AC-1 fails (original red under QEMU) | fix or quarantine the test FIRST; do not convert a red test |
| A-2 false (no backgrounded readiness possible) | fall back to a log-line wait; record the limitation |
| A-3 false (no reload signal) | retain the missing completion fact as an open requirement here until design resolves it or an owner-approved live spec takes it |
| Converted test flakes | investigate the race at the source; never re-add a sleep |
| 3 fix attempts fail | stop and return the failed approaches and the unresolved requirement to the owner; do not park it in a retired deferral file |

## Historical Open Questions (July 2026)

- Did the originals pass under QEMU at the time (A-1)? Current implementation still owes its own baseline after inventory.
- Group B fork: run them via `./le qemu run command "./le qemu all-tests"`, or add `option=needs-linux` to bring them into the fast loop? The second is a test-metadata change with its own review, but it makes 12 blind sleeps verifiable in the tight target. Which is intended?
- Group C (flow-export 4, reload 1, `install/image-resolve-failure` 1, traffic vpp-stub 12): misfiled into a QEMU spec, or missing an OS gate they should have? If genuinely host-runnable, do they belong here at all, given the umbrella says no clean host-verifiable blind sleeps remain?
- Why does a backgrounded ze get no ZE_READY_FILE marker (A-2)? Is that a runner limitation fixable once, converting the whole ZE_READY_FILE shape (5+ traffic tests) in one move rather than per test?
- Is there a reload-completion signal for the SIGHUP shape (A-3), or does it need the same production observability as `spec-fixit-reject-fence-observability`? If the latter, should the reload shape move to that spec?
- Do the 3 external-warn files in group A belong to this spec or to `spec-fixit-reject-fence-observability` (R-5)?
- Does the vpp `WaitConnected` deliberate-timer set (traffic/012, /026) deserve an injectable timeout so the test asserts the behavior without a 5-6s wall-clock wait, or is the real timeout part of what is validated?
- What did the umbrella's "~150 QEMU-gated" figure count? The measured linux-gated blind population is 21. Reconciling this may reveal work this spec has not scoped.

### Resolutions (2026-07-17, autonomous — APPEND-ONLY, Thomas override any if wrong)

These were the July readiness resolutions. The native migration supersedes that
readiness judgement and its closed-spec handoffs; the current inventory and
implementation steps above govern the remaining work.

| # | Question | Resolution | Stakes |
|---|----------|-----------|--------|
| 1 | Originals pass under QEMU today (A-1)? | Cannot run on this host; settled as AC-1, the first implement action. Failure routing already routes an AC-1 red to "fix/quarantine first, do not convert a red test". Not a readiness blocker — it is the first implement step, not a design gap. | low |
| 2 | Group B fork: `ze-qemu-test-all` vs add `option=needs-linux`? | Group B is IN SCOPE. Default branch = `./le qemu run command "./le qemu all-tests"` (zero committed change). Regating to `option=needs-linux` is an optional follow-up with its own review. Both are valid AC-8 "done" states. | scope |
| 3 | Group C: misfiled or missing a gate? | OUT OF SCOPE for this spec (host-runnable, not QEMU-gated). The misfiled-vs-missing-gate call is handed to a follow-up host-runnable/gate-audit spec. | scope |
| 4 | Why no ZE_READY_FILE marker for backgrounded ze (A-2)? Runner-level one-move fix? | Implement-time research (Phase 3). Design-settled: if not fixable once at the runner, AC-2 falls back to a wait on the already-asserted OnConfigure log line (A-5 requires asserting the kernel readback too where possible). Fallback stated → not design-open. | arch |
| 5 | Reload-completion signal for SIGHUP (A-3)? | Implement-time research (Phase 5). Design-settled: if no signal exists, the reload shape hands to `plan/spec-fixit-reject-fence-observability.md` (Failure Routing) or is recorded infra-gated (AC-3). Fallback stated → not design-open. | arch | <!-- doc-links: ignore (spec closed and removed) -->
| 6 | Do the 3 external-warn group A files belong here or to reject-fence (R-5)? | RESOLVED by the 2026-07-16 scope update: handed to `plan/spec-fixit-reject-fence-observability.md`; `ddos-detect-mitigate.ci` handed to `plan/spec-fixit-ddos-test-infra.md` (R-8). This spec does not touch them. Cleanly-owned group A converts: `traffic/022`, `traffic/023`, `install/dhcp-zero-listener`, `install/tftp-zero-listener`, `plugin/ddos-detect-characterize`. | scope | <!-- doc-links: ignore (spec closed and removed) -->
| 7 | vpp `WaitConnected` (traffic/012, /026): injectable timeout, or is the real 5s wait validated? | The real timeout IS the behavior under test (012's header: WaitConnected returns an error after the 5s timeout). Kept unchanged as the AC-6 control. An injectable timeout is a separate optimisation, OUT OF SCOPE (ungated group C anyway). | low |
| 8 | What did the umbrella's "~150" count? | Reconciled: it counted RAW sleeps at an earlier baseline (~246), before the conversions it records. The linux-gated BLIND population (groups A+B) is 21. No hidden in-scope work is revealed; group C's ungated blind sleeps are the difference and are handled as a follow-up (question 3). | low |

## Checklist

### Goal Gates
- [ ] Tests written -- each converted `.ci` IS its own functional test and keeps its exact assertions; infra changes (if any) get red-first unit tests.
- [ ] Tests FAIL -- infra unit tests are red-first (TDD); a converted `.ci` that fails surfaces a real race, fixed at the source, never with a re-added sleep.
- [ ] Tests PASS -- each converted test green under QEMU, both before (AC-1) and after (AC-4) conversion.
- [ ] `./le verify worktree` and `./le qemu run command "./le qemu all-tests"` -- affected suites green before each batch's commit; skip counts read, not just exit codes. `worktree` runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`).

### Quality Gates
- [ ] `test/.ci-sleep-baseline` lowered by exactly the counted embedded sleeps removed, if any, in the same change; native timers never earn a delta.
- [ ] `./le verify current mode changed` green (ratchet + justification gates).
- [ ] `./le changed scope` green.
- [ ] Every kept sleep still carries a justifying comment.
- [ ] No bounded poll and no deliberate timer converted for baseline credit.

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
