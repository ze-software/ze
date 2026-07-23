# spec-fixit-sleeps-qemu-bulk

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fixit-migrate-sleeps-infra (QEMU-gated carve-out) |
| Phase | 0/N (research) |
| Updated | 2026-07-17 |

Update (2026-07-22 plan review; body corrected in-body 2026-07-22): the sleep
ratchet baseline is now **125** (`test/.ci-sleep-baseline`, composable-delta
format), not 132 -- every in-body baseline occurrence is annotated with the
current value (historical counts kept as history); and both sibling handoff
targets this spec defers work to have since CLOSED
(`spec-fixit-reject-fence-observability` -> learned 1232,
`spec-fixit-ddos-test-infra` -> learned 1186). Re-verify the group A/B
taxonomy and handoff ownership against the current tree before implementing.
The `record_parse.go`/`record.go` line drift is also corrected in-body:
`NeedsLinux=true` `:412` (was `:400`), `case "needs-linux"` `:394` (was
`:391`), the `ZE_QEMU_LINUX_ONLY` gate `:236-237` (was `:235-236`),
`record.go` `NeedsLinux` field doc `:203-207` (was `:179-183`). (Re-verified
2026-07-23 after the origin/main fast-forward to 822029463, which touched
both runner files again.)
| Scope | groups A + B (needs-linux + skip-os:darwin blind sleeps); group C ungated is OUT (2026-07-17 autonomous default, see Open Questions → Resolutions) |

## Task

The linux-gated `.ci` tests still hold blind `time.sleep()` calls that cannot be
converted to deterministic waits on the darwin dev host, because neither the
original nor the converted test can be RUN there to prove the conversion. Each
one needs the same treatment: confirm the ORIGINAL passes under QEMU, convert the
blind hold to a deterministic wait, then RE-VERIFY under QEMU. A darwin skip is
not evidence: per `ai/rules/qemu-testing.md` and the umbrella spec's R-2, no
linux-only conversion may be marked done on a darwin-skip alone. Converting each
sleep lowers `test/.ci-sleep-baseline` (cited 132; now 125, 2026-07-22), which must be
ratcheted down in the same change.

> **Scope correction (verified 2026-07-15, see Problem / Evidence).** The brief that
> spawned this spec described the scope as "needs-linux, QEMU-only" and gave
> per-directory raw sleep counts. Reading the files shows those counts are raw
> `time.sleep(` totals mixing three populations: blind holds (the real targets),
> already-deterministic bounded polls (must NOT be converted), and tests with no OS
> gate at all (host-runnable on darwin, so not QEMU-gated). Only **9** blind sleeps
> are reachable by `make ze-qemu-needs-linux-test`. Research must re-derive the target
> list from the gate, not from the directory totals.

## Origin

Carved out of `plan/spec-fixit-migrate-sleeps-infra.md` (the umbrella), whose Design
Insights name a "QEMU-gated needs-linux bulk (~150)" that the darwin dev host cannot
verify, and whose Implementation Summary states no further clean host-verifiable blind
sleeps remain. Skeleton written 2026-07-15 alongside `spec-fixit-sleeps-cli-harness`.

## Required Reading

<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->

### Source (read before designing)
- [ ] `internal/test/runner/record_parse.go:394-412` (was `:391-400`) - `option=needs-linux` parsing: the ONLY place `Record.NeedsLinux` is set.
  → Constraint: `option=skip-os:value=darwin` does NOT set `NeedsLinux`. The two gates are not interchangeable, and the difference decides which QEMU target reaches a test.
- [ ] `internal/test/runner/record_parse.go:236-237` (was `:235-236`) - the `ZE_QEMU_LINUX_ONLY=1` filter.
  → Constraint: every test NOT marked `option=needs-linux` gets `SkipReason = "ZE_QEMU_LINUX_ONLY (not option=needs-linux)"`. So `make ze-qemu-needs-linux-test` SKIPS the whole policy/firewall/ospf/pppoe group. Claiming those "QEMU-verified" via that target would be false.
- [ ] `internal/test/runner/record.go:203-207` (was `:179-183`) - `NeedsLinux` field doc.
- [ ] `mk/test-integration.mk:251-260` - the `ze-qemu-needs-linux-test` target: cross-compiles linux binaries, runs `qemu-all-tests.sh` with `ZE_QEMU_LINUX_ONLY=1` and `ZE_QEMU_SKIP_SUITES="web"`.
  → Constraint: one VM boot for all linux-only tests, not one VM per test. Per-test iteration uses `make ze-qemu-debug RUN=...`.
- [ ] `scripts/dev/verify_wiring_docs.py:196-256` (`check_ci_sleep_ratchet`), `:258-285` (`check_ci_sleep_justification`) - the two gates.
  → Constraint: the ratchet caps HOW MANY sleeps exist (against the baseline); the justification gate is scoped to CHANGED `.ci` files only, so touching a file makes this session responsible for every sleep in it.
- [ ] `test/.ci-sleep-baseline` - cited `132`; now `125` (composable-delta sum, 2026-07-22).
  → Constraint: the 2026-07-15 measurement was exactly 126 `time.sleep(` in `test/**/*.ci` (baseline tight, zero slack); `test/.ci-sleep-baseline` read 132 (verified 2026-07-16 by reading the file) after sibling specs landed, and now sums to 125 (2026-07-22, composable-delta format), so re-measure the tree at Phase 2. Any conversion must lower the baseline in the same change or the ratchet fails.

### Architecture Docs
- [ ] `ai/rules/qemu-testing.md` - QEMU integration is mandatory for linux-only code; never skip for "needs hardware".
  → Constraint: this rule is the spec's completion criterion. Conversion is not done; QEMU re-verification is done.
- [ ] `ai/rules/ci-sleep-justification.md` - the comment-on-every-sleep rule.
  → Constraint: a converted sleep removes its comment with it; a KEPT sleep must keep justifying itself or the gate fails on the changed file.
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` directive catalog (`option=needs-linux`, `option=skip-os`, `expect=`).

## Current Behavior (MANDATORY)

**Source files (cite file:line):**
- [ ] `internal/test/runner/record_parse.go` - `r.NeedsLinux = true` (:412, was :400), reached only under `case "needs-linux":` (:394, was :391). Sets `SkipReason` for any non-needs-linux test when `ZE_QEMU_LINUX_ONLY == "1"` (:236-237, was :235-236).
- [ ] `internal/test/runner/record.go` - the `NeedsLinux` field doc (:203-207, was :179-183): set by `option=needs-linux` so the `ZE_QEMU_LINUX_ONLY` filter can run ONLY those tests.
- [ ] `scripts/dev/verify_wiring_docs.py` - `check_ci_sleep_ratchet` (:196) reads `test/.ci-sleep-baseline`; `check_ci_sleep_justification` (:258) is scoped to CHANGED `.ci` files (:268).
- [ ] `test/.ci-sleep-baseline` - was `132` (:1, then a single integer); now sums to `125` (2026-07-22, composable-delta format: the ceiling is the sum of the signed-integer lines); up from the 126 measured 2026-07-15 as sibling specs landed (re-measure the tree count at Phase 2).
- [ ] `test/traffic/001-boot-apply.ci:21-25`, `test/traffic/011-vpp-reject-hfsc.ci:23`, `test/traffic/020-vpp-reject-dscp-filter.ci:24`, `test/traffic/022-boot-qdisc-tc.ci:21`, `test/traffic/024-vpp-reject-prio.ci:21` - the ZE_READY_FILE blind-hold shape, annotated "blind hold: a backgrounded ze gets no ZE_READY_FILE marker to poll; hold until OnConfigure emits the asserted log line, left un-converted (no readiness signal for a background daemon)".
- [ ] `test/traffic/012-vpp-not-connected.ci:2,14,22,26` and `test/traffic/026-vpp-accept-multiclass.ci:23,24` - the deliberate-timer shape: "blind hold: the internal 5s vpp WaitConnected timeout IS the behavior under test". 012's header states it VALIDATES that `WaitConnected` returns an error after the 5s timeout.
- [ ] `test/traffic/002-reload-apply.ci:39,44`, `test/traffic/023-reload-qdisc-tc.ci:38,46`, `test/policy/006-reload.ci:33,46` - the reload shape: a "blind settle" before SIGHUP ("let the initial apply finish before the reload; this standalone driver has no post-apply signal to poll") plus a "blind hold" after ("SIGHUP reload exposes no completion signal to this standalone driver; hold for the reactor to re-apply and emit the asserted log").

**Behavior to preserve:**
- Every converted test keeps its exact `expect=`/`reject=`/fatal assertions. Only the WAIT mechanism changes (umbrella "Behavior to preserve").
- The 51 sleeps annotated "bounded wait not a blind sleep" stay as-is. They are already deterministic (the enclosing loop breaks on a real signal; the sleep is only the poll interval). Converting them is out of scope and would be churn.
- Deliberate timers where the delay IS the behavior under test stay, documented (traffic/012, traffic/026: the 5s vpp `WaitConnected` timeout).
- Each kept sleep keeps a justifying comment so `check_ci_sleep_justification` stays green on the changed files.
- No production behavior change beyond additive test-support surfaces.

**Behavior to change:**
- Convert the QEMU-gated blind holds/settles to deterministic waits and lower the baseline.
- Exact per-test recipe: None yet, research first. The umbrella's Core Insight is that conversion is NOT mechanical: each sleep has a per-test reason that only surfaces on attempting and running it.

## Problem / Evidence

**CONFIRMED (measured 2026-07-15 by reading and grepping the tree):**

Total `time.sleep(` in `test/**/*.ci` was 126 as measured 2026-07-15, matching the then-baseline; `test/.ci-sleep-baseline` rose to 132 as sibling specs landed, and now sums to 125 (2026-07-22; re-measure the tree at Phase 2).
Of those, 51 carry the "bounded wait not a blind sleep" annotation (already deterministic
poll intervals), leaving 75 non-poll sleeps. 51 of the 75 are explicitly annotated
`# blind hold:` (38) or `# blind settle:` (13); the remaining 24 carry other
justifications (deliberate timers, raw-protocol pacing).

Gate taxonomy across ALL files containing sleeps (blind = raw minus bounded polls):

| Group | Gate | Reachable by | Raw | Blind |
|-------|------|--------------|-----|-------|
| A | `option=needs-linux` | `make ze-qemu-needs-linux-test` | 14 | **9** |
| B | `option=skip-os:value=darwin` only | NOT that target (filtered at `record_parse.go:235`); needs `ze-qemu-all-test` | 31 | **12** |
| C | no OS gate | the darwin host directly | 81 | **54** |

Group A files (raw/blind): `install/dhcp-zero-listener.ci` 1/1, `install/tftp-zero-listener.ci` 1/1,
`plugin/ddos-detect-characterize.ci` 3/1, `plugin/ddos-detect-external-warns.ci` 1/1,
`plugin/ddos-detect-mitigate.ci` 2/0, `plugin/flowexport-external-refuses.ci` 1/1,
`plugin/trafficusage-external-refuses.ci` 1/1, `traffic/022-boot-qdisc-tc.ci` 1/1,
`traffic/023-reload-qdisc-tc.ci` 3/2.

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
1. Only 9 blind sleeps are actually reachable by `make ze-qemu-needs-linux-test`. The brief's implied scope (~56 raw across those dirs) is roughly 6x the target-reachable blind population.
2. The policy/firewall/ospf/pppoe bulk (group B, 12 blind) is linux-only but NOT run by the needs-linux target. Verifying it needs `make ze-qemu-all-test`, or those tests need `option=needs-linux` added. Unresolved fork, see Open Questions.
3. flow-export (4 blind), reload (1 blind), `install/image-resolve-failure.ci` (1 blind) and the 12 ungated traffic vpp-stub sleeps have NO OS gate, so they are host-runnable on darwin and do not belong in a QEMU-gated spec by its own criterion. Either they are misfiled here, or they are missing a gate they should have.

→ AUTONOMOUS DEFAULT (2026-07-17) [scope]: This spec is scoped to **groups A + B** — the needs-linux + skip-os:darwin linux-only blind sleeps (A = 9 blind, reachable by `make ze-qemu-needs-linux-test`; B = 12 blind, reachable only by `make ze-qemu-all-test`; measured taxonomy verified 2026-07-17: group A raw 14, group B raw 31, tree total 132 -- baseline now 125, 2026-07-22; re-derive the taxonomy at Phase 2). **Group C** (54 blind, ungated, host-runnable on darwin) is OUT OF SCOPE and recorded as a noted follow-up (see Future + Open Questions → Resolutions). Rationale: smaller, self-contained scope per the readiness decision protocol — a QEMU-gated spec should own only the sleeps its QEMU criterion can verify; the ungated group C is host-verifiable and belongs to a host-runnable (or gate-audit) spec that decides per file whether each C test is misfiled or missing a gate. Thomas: override if wrong.
→ AUTONOMOUS DEFAULT (2026-07-17) [scope]: Group B verification fork (correction 2) resolves in favour of `make ze-qemu-all-test` as the DEFAULT branch (it already runs the skip-os:darwin group; zero committed change). Adding `option=needs-linux` to the 15 group B files — which would pull the 12 blind into the fast `ze-qemu-needs-linux-test` loop — is the more-reversible alternative, recorded as an OPTIONAL follow-up (a test-metadata change with its own review), adopted only if full-suite turnaround proves impractical during implement. Rationale: the smaller self-contained default mutates zero committed metadata; both branches remain valid "done" states under AC-8. Thomas: override if wrong.

**UNVERIFIED:**
- That the ORIGINAL (unconverted) tests currently pass under QEMU. Not run: this host does not run QEMU, and this skeleton ran no tests. The confirm-original step is AC-1 precisely because it is unverified.
- Whether the group B / group C classification reflects intent or drift (a test may be missing an `option=needs-linux` it deserves).
- The umbrella's "~150 QEMU-gated" figure. The measured linux-gated blind population (groups A+B) is 21, an order of magnitude smaller. The umbrella figure appears to count raw sleeps at an earlier baseline (246), before the conversions it records.

**Scope update (2026-07-16, sibling hand-offs):** R-5 hands the 3 external-warn group A files (`ddos-detect-external-warns.ci`, `flowexport-external-refuses.ci`, `trafficusage-external-refuses.ci`) to `plan/spec-fixit-reject-fence-observability.md`; R-8 hands the ddos-detect-mitigate files to `plan/spec-fixit-ddos-test-infra.md`, which now carry 0 blind annotations, so this spec has nothing to convert there. After both hand-offs the cleanly-owned convertible sleeps shrink to about 5: `traffic/022-boot-qdisc-tc.ci`, `traffic/023-reload-qdisc-tc.ci` (2 sleeps), `install/dhcp-zero-listener.ci`, `install/tftp-zero-listener.ci`. At Phase 2 the residual population must be re-measured to confirm a standalone spec is still justified, versus folding the QEMU-verification of these ~5 sleeps into the sibling specs.

## Data Flow (MANDATORY)

### Entry Point
- A `.ci` test's embedded driver or observer reaches a point where it must wait for a linux-only effect (a tc qdisc programmed, an nft table applied, a SIGHUP reload re-applied, a backgrounded ze finishing OnConfigure). Today it calls `time.sleep()` blindly.

### Transformation Path
1. The test driver waits: today a fixed blind duration, after this spec a bounded poll on a real signal.
2. The signal is produced by the linux-only effect itself (the asserted log line, a readback such as `ListQdiscs` or `nft list`, a readiness marker).
3. The driver's existing `expect=`/fatal assertion runs unchanged once the wait is satisfied.
4. The runner compares the driver's output against the unchanged `expect=` directives.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| runner ↔ backgrounded ze | ZE_READY_FILE marker (absent for background spawns today; the gap driving the blind-hold shape) | [ ] |
| driver ↔ daemon (reload) | SIGHUP, with no completion signal back to a standalone driver today | [ ] |
| driver ↔ linux kernel | tc / nft readback from the driver process | [ ] |
| host ↔ QEMU VM | `make ze-qemu-needs-linux-test` (one VM boot, `ZE_QEMU_LINUX_ONLY=1`) | [ ] |

### Integration Points
- `.ci` files across `test/traffic`, `test/policy`, `test/firewall`, `test/ospf`, `test/pppoe`, `test/install`, and the linux-gated slice of `test/plugin`.
- `test/.ci-sleep-baseline` (ratchet), lowered per conversion batch.
- Possibly `internal/test/runner/` if the backgrounded-ze readiness gap (A-2) is fixed once at the runner instead of per test.
- Possibly `docs/architecture/testing/ci-format.md` if a new wait surface or gate convention is introduced.

### Architectural Verification
- [ ] No bypassed layers (each wait polls the real effect it asserts).
- [ ] No unintended coupling (test-support surfaces stay additive).
- [ ] No duplicated functionality (reuse existing readbacks and the existing readiness barrier before adding any).
- [ ] Registration over hardcoding: any new wait surface or reflecting query registers and is core-discovered, never a per-test special case wired into a shared package.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| backgrounded ze reaches OnConfigure | -> | readiness signal pollable by the driver (runner-level or marker-based) | `test/traffic/022-boot-qdisc-tc.ci` (QEMU) |
| driver sends SIGHUP for reload | -> | reload-completion signal the driver can poll | `test/traffic/023-reload-qdisc-tc.ci`, `test/policy/006-reload.ci` (QEMU) |
| tc qdisc programmed by OnConfigure | -> | tc readback poll replacing the blind hold | `test/traffic/022-boot-qdisc-tc.ci` (QEMU) |
| dhcp/tftp zero-listener path | -> | deterministic wait on the asserted listener state | `test/install/dhcp-zero-listener.ci`, `test/install/tftp-zero-listener.ci` (QEMU) |
| ddos characterize pipeline | -> | deterministic wait on the characterization result | `test/plugin/ddos-detect-characterize.ci` (QEMU) |
| a sleep is removed from any `.ci` | -> | `check_ci_sleep_ratchet` (`scripts/dev/verify_wiring_docs.py:196`) | `test/.ci-sleep-baseline` lowered; `make ze-verify-changed` green |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Before converting any test | The ORIGINAL test is confirmed green under QEMU, so a post-conversion failure is unambiguously the conversion's fault |
| AC-2 | A blind hold on a background daemon's readiness (the ZE_READY_FILE shape) | Replaced by a deterministic wait on a real signal (the asserted OnConfigure log line, or a readiness marker made available to backgrounded ze), bounded, with a timeout that names what it waited for |
| AC-3 | A pre-SIGHUP blind settle / post-SIGHUP blind hold (traffic/002, traffic/023, policy/006) | Replaced by a wait on an observable reload-completion signal, or the test is recorded as infra-gated with the missing signal named |
| AC-4 | Each converted test, after conversion | RE-VERIFIED green under QEMU. Never marked done on a darwin skip alone (`ai/rules/qemu-testing.md`, umbrella R-2) |
| AC-5 | A sleep annotated "bounded wait not a blind sleep" | Left unchanged; it is already deterministic |
| AC-6 | A deliberate timer (traffic/012, traffic/026: the 5s vpp `WaitConnected` timeout IS the behavior under test) | Kept, justifying comment intact, `check_ci_sleep_justification` green |
| AC-7 | Any commit that removes sleeps | `test/.ci-sleep-baseline` lowered by exactly the number removed, in the SAME change (the baseline stood at 132 when written; now 125, 2026-07-22) |
| AC-8 | The group B (skip-os-darwin) tests in scope | Either verified via a target that actually runs them (`ze-qemu-all-test`), or given `option=needs-linux` so `ze-qemu-needs-linux-test` reaches them, with the choice recorded. Never claimed verified by a target that skipped them |
| AC-9 | Full suite after all conversions | `make ze-qemu-needs-linux-test` green, no test converted-but-unverified, no regression in the affected suites |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The originals currently pass under QEMU | they are committed and gated, not quarantined (no skip/xfail marker found) | AC-1 fails first and this becomes a test-repair spec, not a conversion spec | run `make ze-qemu-needs-linux-test` before touching anything | unvalidated |
| A-2 | A backgrounded ze can be given a pollable readiness signal | the annotation says the ZE_READY_FILE marker is absent for backgrounded ze, not that readiness is unobservable; the asserted OnConfigure log line already exists as a signal | AC-2 collapses to "wait on the log line" only, or needs a production readiness surface (scope grows) | read the ZE_READY_FILE producer and the runner's background-spawn path | unvalidated |
| A-3 | SIGHUP reload completion is observable without new production code | the tests already assert a post-reload log line | AC-3 needs a new reload-processed signal, overlapping `plan/spec-fixit-reject-fence-observability.md` | read the SIGHUP handler and its emitted events | unvalidated |
| A-4 | Group B tests are correctly gated with `skip-os` rather than `needs-linux` | they are linux-only in substance (nft/tc/kernel) | adding `option=needs-linux` is a cheap fix bringing 12 blind sleeps into the fast QEMU loop | compare a group A and a group B test's requirements | unvalidated |
| A-5 | Converting a blind hold to a log-line wait is not vacuous | the log line is already the asserted effect | the wait passes before the effect lands, giving a false green | assert the effect (kernel readback), not only the log line, where possible | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A linux-only conversion is claimed done on a darwin skip | the test "passes" on darwin instantly with a skip reason | MANDATORY QEMU run per test (`ai/rules/qemu-testing.md`); AC-4 makes re-verification the completion criterion, not conversion |
| R-2 | "Verified" via a target that silently skipped the test (group B under `ZE_QEMU_LINUX_ONLY=1`) | the QEMU run reports the test skipped, not passed, and the summary is not read closely | AC-8; always read the skip count in the QEMU summary, never just the exit code |
| R-3 | A converted test flakes under QEMU's slower timing | intermittent red | investigate the race at the source; NEVER re-add a sleep (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| R-4 | Converting a bounded poll or a deliberate timer for baseline credit | the diff touches sleeps whose comments say "bounded wait" or "IS the behavior under test" | AC-5/AC-6; the ratchet rewards removal, so guard against gaming it |
| R-5 | Overlap with `plan/spec-fixit-reject-fence-observability.md` | two specs edit the same external-warn tests | 3 group A files (`ddos-detect-external-warns`, `flowexport-external-refuses`, `trafficusage-external-refuses`) are named by the umbrella as infra-gated reject-fence cases; confirm ownership before touching them |
| R-8 | Overlap with `plan/spec-fixit-ddos-test-infra.md` (sibling skeleton, created 2026-07-15) | both specs edit `test/plugin/ddos-detect-mitigate.ci` | that spec's Problem A REWRITES that file onto the `ze_api` observer pattern, changing its sleep count. It has blind=0 today, so this spec has nothing to convert in it: leave it alone and let the ddos spec own it. Re-measure the baseline after that spec lands |
| R-6 | QEMU turnaround makes per-test iteration slow | the loop drags | use `make ze-qemu-debug RUN=...` for single-test iteration; batch the final verification |
| R-7 | Touching a `.ci` file makes the session own every sleep in it (justification gate is changed-file scoped) | the gate fails on sleeps the session did not add | expect it; justify or convert the neighbours in the same file |

## 🧪 TDD Test Plan

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
| `022-boot-qdisc-tc.ci` | `test/traffic/` | tc qdisc programmed at boot; blind hold -> deterministic wait | pending (QEMU) |
| `023-reload-qdisc-tc.ci` | `test/traffic/` | SIGHUP reload re-applies tc; settle + hold converted | pending (QEMU) |
| `006-reload.ci` | `test/policy/` | SIGHUP reload re-applies nft policy routes | pending (QEMU, group B) |
| `dhcp-zero-listener.ci`, `tftp-zero-listener.ci` | `test/install/` | zero-listener path | pending (QEMU) |
| `ddos-detect-characterize.ci` | `test/plugin/` | ddos characterization | pending (QEMU) |
| `012-vpp-not-connected.ci`, `026-vpp-accept-multiclass.ci` | `test/traffic/` | deliberate 5s `WaitConnected` timer: unchanged, kept as control that AC-6 was honored | keep |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Notes |
|----------|-------|
| N/A | test infrastructure only; no wire-protocol behavior changes |

### Future (if deferring any tests)
- Group B (12 blind) may split into its own change if the `option=needs-linux` regating (A-4) is approved separately.
- Group C (54 blind, ungated) is likely out of scope for this spec entirely; see Open Questions.
  → AUTONOMOUS DEFAULT (2026-07-17) [scope]: CONFIRMED out of scope. Group C is neither converted nor QEMU-verified by this spec. Follow-up: a separate host-runnable (or gate-audit) spec decides, per file, whether each ungated blind sleep is misfiled into a QEMU context or is missing an `option=needs-linux`/`skip-os` gate it deserves. Thomas: override if wrong.

## Files to Modify

- `test/.ci-sleep-baseline` - lowered per conversion batch (ratchet).
- `docs/architecture/testing/ci-format.md` - document any new wait surface or gate convention introduced by research (candidate, pending research).
- `internal/test/runner/record_parse.go` - only if the backgrounded-ze readiness gap (A-2) or a regating decision (A-4) lands at the runner (candidate, pending research; UNVERIFIED that a change is needed here).
- `ai/rules/qemu-testing.md` - only if the group A/B target-reachability distinction deserves recording as a rule (candidate).
- The converted `.ci` files: `test/traffic/022-boot-qdisc-tc.ci`, `test/traffic/023-reload-qdisc-tc.ci`, `test/install/dhcp-zero-listener.ci`, `test/install/tftp-zero-listener.ci`, `test/plugin/ddos-detect-characterize.ci`, plus the group B set (`test/policy/*.ci`, `test/firewall/*.ci`, `test/ospf/*.ci`, `test/pppoe/*.ci`) if AC-8 resolves in favour of including them.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Test infra docs | only if a new wait surface lands | `docs/architecture/testing/ci-format.md` |
| Discovery updates | only if a new primitive/gate lands | `ai/INDEX.md` per `ai/rules/discovery-updates.md` |
| QEMU verification | yes (the spec's whole point) | `make ze-qemu-needs-linux-test` |
| Ratchet | yes | `test/.ci-sleep-baseline` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File |
|---|----------|----------|------|
| 8 | Plugin SDK/test-support changed? | only if a new wait surface lands | `ai/rules/testing.md`, `docs/functional-tests.md` |
| 10 | Test infrastructure changed? | likely (readiness signal / regating) | `docs/architecture/testing/ci-format.md` |

## Files to Create
- None planned. Research may add a runner unit-test file if A-2 leads to a runner change.

## Implementation Steps

### /implement Stage Mapping
| Stage | Section |
|-------|---------|
| Audit | this spec's Problem / Evidence tables, re-measured against the tree (counts drift as siblings land) |
| Implement | the phases below, one shape at a time |
| Verify | `make ze-qemu-needs-linux-test` per batch; `make ze-qemu-debug RUN=...` per test while iterating |
| Close | ratchet lowered per batch; two-commit closure |

### Implementation Phases
1. **Baseline under QEMU (AC-1).** Run `make ze-qemu-needs-linux-test` untouched; record which tests pass, fail, and SKIP. Resolve the group B question (AC-8) from the observed skip list before any conversion.
2. **Re-measure the taxonomy.** Re-derive group A/B/C and the blind counts from the tree; sibling specs move these numbers.
3. **ZE_READY_FILE shape (A-2).** Investigate why a backgrounded ze gets no marker. If fixable once at the runner, that converts the whole shape in one move.
4. **tc/nft readback shape.** Convert the boot-apply holds to kernel readback polls; QEMU-verify each.
5. **Reload shape (A-3).** Resolve whether a reload-completion signal exists; convert or hand to `spec-fixit-reject-fence-observability`.
6. **Group B decision (AC-8).** Either regate to `option=needs-linux` or verify via `ze-qemu-all-test`; then convert.
7. **Final ratchet + full QEMU pass (AC-9).**

### Critical Review Checklist (/implement stage 6)
| Check | For this spec |
|-------|---------------|
| Assertions preserved | every converted test keeps its `expect=`/fatal checks |
| QEMU-verified | no linux-only conversion claimed done on darwin-skip alone (R-1); skip counts read, not just exit codes (R-2) |
| Non-vacuous | the wait polls the asserted effect, not a proxy that can precede it (A-5) |
| Bounded polls untouched | no churn on the 51 "bounded wait not a blind sleep" entries (AC-5) |
| Deliberate timers kept | traffic/012, traffic/026 unchanged and still justified (AC-6) |
| Registration over hardcoding | any new wait surface registers and is core-discovered, not hardcoded into a shared package |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification |
|-------------|--------------|
| Each converted test | sleep gone (`grep`); QEMU green before AND after |
| Ratchet lowered | `cat test/.ci-sleep-baseline`; `scripts/dev/verify_wiring_docs.py` green |
| No regressions | `make ze-qemu-needs-linux-test` + affected suites |

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
| A-3 false (no reload signal) | hand the reload shape to `plan/spec-fixit-reject-fence-observability.md` |
| Converted test flakes | investigate the race at the source; never re-add a sleep |
| 3 fix attempts fail | mark DEFER in `plan/deferrals.md`, move on, report |

## Open Questions (research before design)

- Do the originals pass under QEMU today (A-1)? This gates everything and must be the first action.
- Group B fork: run them via `make ze-qemu-all-test`, or add `option=needs-linux` to bring them into the fast loop? The second is a test-metadata change with its own review, but it makes 12 blind sleeps verifiable in the tight target. Which is intended?
- Group C (flow-export 4, reload 1, `install/image-resolve-failure` 1, traffic vpp-stub 12): misfiled into a QEMU spec, or missing an OS gate they should have? If genuinely host-runnable, do they belong here at all, given the umbrella says no clean host-verifiable blind sleeps remain?
- Why does a backgrounded ze get no ZE_READY_FILE marker (A-2)? Is that a runner limitation fixable once, converting the whole ZE_READY_FILE shape (5+ traffic tests) in one move rather than per test?
- Is there a reload-completion signal for the SIGHUP shape (A-3), or does it need the same production observability as `spec-fixit-reject-fence-observability`? If the latter, should the reload shape move to that spec?
- Do the 3 external-warn files in group A belong to this spec or to `spec-fixit-reject-fence-observability` (R-5)?
- Does the vpp `WaitConnected` deliberate-timer set (traffic/012, /026) deserve an injectable timeout so the test asserts the behavior without a 5-6s wall-clock wait, or is the real timeout part of what is validated?
- What did the umbrella's "~150 QEMU-gated" figure count? The measured linux-gated blind population is 21. Reconciling this may reveal work this spec has not scoped.

### Resolutions (2026-07-17, autonomous — APPEND-ONLY, Thomas override any if wrong)

Every open question above is resolved for readiness. Empirical confirmations that can only run under QEMU are deferred to implement-time as the named AC (they are design-settled with a stated fallback, not design-open).

| # | Question | Resolution | Stakes |
|---|----------|-----------|--------|
| 1 | Originals pass under QEMU today (A-1)? | Cannot run on this host; settled as AC-1, the first implement action. Failure routing already routes an AC-1 red to "fix/quarantine first, do not convert a red test". Not a readiness blocker — it is the first implement step, not a design gap. | low |
| 2 | Group B fork: `ze-qemu-all-test` vs add `option=needs-linux`? | Group B is IN SCOPE. Default branch = `make ze-qemu-all-test` (zero committed change). Regating to `option=needs-linux` is an optional follow-up with its own review. Both are valid AC-8 "done" states. | scope |
| 3 | Group C: misfiled or missing a gate? | OUT OF SCOPE for this spec (host-runnable, not QEMU-gated). The misfiled-vs-missing-gate call is handed to a follow-up host-runnable/gate-audit spec. | scope |
| 4 | Why no ZE_READY_FILE marker for backgrounded ze (A-2)? Runner-level one-move fix? | Implement-time research (Phase 3). Design-settled: if not fixable once at the runner, AC-2 falls back to a wait on the already-asserted OnConfigure log line (A-5 requires asserting the kernel readback too where possible). Fallback stated → not design-open. | arch |
| 5 | Reload-completion signal for SIGHUP (A-3)? | Implement-time research (Phase 5). Design-settled: if no signal exists, the reload shape hands to `plan/spec-fixit-reject-fence-observability.md` (Failure Routing) or is recorded infra-gated (AC-3). Fallback stated → not design-open. | arch |
| 6 | Do the 3 external-warn group A files belong here or to reject-fence (R-5)? | RESOLVED by the 2026-07-16 scope update: handed to `plan/spec-fixit-reject-fence-observability.md`; `ddos-detect-mitigate.ci` handed to `plan/spec-fixit-ddos-test-infra.md` (R-8). This spec does not touch them. Cleanly-owned group A converts: `traffic/022`, `traffic/023`, `install/dhcp-zero-listener`, `install/tftp-zero-listener`, `plugin/ddos-detect-characterize`. | scope |
| 7 | vpp `WaitConnected` (traffic/012, /026): injectable timeout, or is the real 5s wait validated? | The real timeout IS the behavior under test (012's header: WaitConnected returns an error after the 5s timeout). Kept unchanged as the AC-6 control. An injectable timeout is a separate optimisation, OUT OF SCOPE (ungated group C anyway). | low |
| 8 | What did the umbrella's "~150" count? | Reconciled: it counted RAW sleeps at an earlier baseline (~246), before the conversions it records. The linux-gated BLIND population (groups A+B) is 21. No hidden in-scope work is revealed; group C's ungated blind sleeps are the difference and are handled as a follow-up (question 3). | low |

## Checklist

### Goal Gates
- [ ] Tests written -- each converted `.ci` IS its own functional test and keeps its exact assertions; infra changes (if any) get red-first unit tests.
- [ ] Tests FAIL -- infra unit tests are red-first (TDD); a converted `.ci` that fails surfaces a real race, fixed at the source, never with a re-added sleep.
- [ ] Tests PASS -- each converted test green under QEMU, both before (AC-1) and after (AC-4) conversion.
- [ ] make ze-test / `make ze-qemu-needs-linux-test` -- affected suites green before each batch's commit; skip counts read, not just exit codes.

### Quality Gates
- [ ] `test/.ci-sleep-baseline` lowered by exactly the number of sleeps removed, same change.
- [ ] `make ze-verify-changed` green (ratchet + justification gates).
- [ ] `make ze-lint-changed` green.
- [ ] Every kept sleep still carries a justifying comment.
- [ ] No bounded poll and no deliberate timer converted for baseline credit.

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
