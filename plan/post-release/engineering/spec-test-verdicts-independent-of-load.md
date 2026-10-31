# Spec: test-verdicts-independent-of-load

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Test tooling: a red that measures host load hides real reds and costs the release evidence its trust, but no shipped behaviour changes |
| Depends | `plan/post-release/engineering/spec-native-action-job-admission.md` (admission on every heavy action; this spec adds memory and disk to it) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Sibling: `plan/post-release/engineering/spec-test-sandbox.md` owns the state
half of root cause T1. This spec owns the timing half.

## Task

**Root cause.** Every session shares one host, by the owner rule in
`ai/META-DECISIONS.md` ("Sessions share the one `main` checkout"), and many run
at once. That rule is the cause of this family and this spec does not change it.
What it changes is that a test's verdict is decided by a wall clock (a fixed
deadline, a sleep, a wall-clock ratio, a single read at an instant) and by
resources nobody admits (memory, disk), so the verdict measures how busy the
host was. The same tree gives disjoint failure sets at load 3 and load 40.

**Why it matters.** 109 open journal rows. Most say "not reproduced" or "passes
alone", which is the signature: the test was never wrong about the code, it was
wrong about the clock. Each costs a session an investigation and teaches agents
to discount reds.

**Correction to the merged analysis.** The analysis proposed "load-scaled
budgets". `plan/release/validation/spec-a-test-passes-at-any-concurrency.md`
measured that shape on 2026-09-19: widening fixture waits by the contention
factor made the plugin suite WORSE (5, 3, 5 lost against 2, 2, 2), because a
longer wait holds daemons, peers and ports longer and feeds the contention. So
the fix is not a bigger clock. It is no clock where the protocol owes none.

**The fix in one sentence.** A test waits on the event it asserts (a barrier
the fixture SDK provides), a deadline exists only where a protocol timer owes
one, the runner kills on a stall (no progress) rather than on a wall total, and
admission holds a run until the memory and disk it needs are free.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/runner-architecture.md` - timeout resolution, contended-run labelling, inner gates
  -> Constraint: the page states inner gates derive from the resolved per-test budget with parallel headroom. That is the design this spec replaces; the page is edited with it.
- [ ] `docs/architecture/testing/test-health.md` - how flakes are recorded
- [ ] `plan/release/validation/spec-a-test-passes-at-any-concurrency.md` - the plugin suite at every concurrency
  -> Decision: that spec keeps its ACs for the plugin suite and its per-case protocol-boundary classification. This spec supplies the mechanism (barriers, stall kill, admission) and owns every other suite and the unit packages. Neither re-proposes widening waits or lowering concurrency.
- [ ] `plan/post-release/engineering/spec-fixit-sleeps-qemu-bulk.md` - blind holds in the Linux-gated tests
  -> Decision: its groups A and B become users of the barrier API this spec adds; that spec stays the owner of those fixtures.
- [ ] `plan/post-release/engineering/spec-native-action-job-admission.md` - admission rollout to every native action
- [ ] `plan/post-release/engineering/spec-le-build-cache-cap.md` - the Go build-cache size cap
  -> Decision: the cap bounds one writer; this spec adds the admission check that refuses to start a run into a full disk, for every writer.

## Current Behavior (MANDATORY)

Verified at the producer on 2026-10-23.

- [ ] `internal/test/runner/timing.go` - `Timings.SuggestedTimeout`: `min(fallback, max(5s, 5 x average))` from recorded samples. No term for the host's state; the budget is a wall total.
- [ ] `internal/test/runner/parallel.go` - `ParallelTimeoutHeadroom = 3` triples it whenever tests run concurrently.
- [ ] `internal/le/job/job.go` - admission slots are `runtime.NumCPU() / gotoolchain.CoresPerJob()`. CPU is the only resource counted.
- [ ] `internal/le/job/registry.go` - `(*Admission).take` writes the holder entry; nothing checks free memory or free disk. `breakStalled` sends SIGKILL 3 s after SIGTERM whatever the teardown is doing.
- [ ] `internal/le/interoplab/zebuild.go` - `labBuildTimeout` is a fixed 5 minutes for the lab cross-compile.
- [ ] `test/plugin/*.ci` - `option=timeout:value=<n>s` is an absolute wall deadline covering daemon start, session establishment and plugin startup.
- [ ] `internal/test/fixture/register_drain_signal.go` - `drainSSHRestart` polls 75 x 200 ms; the readiness waits in the fixtures are sleep loops with a count.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le test <suite>`, `./le test unit`, `./le test integration ...`, and every Go test that waits on a goroutine, a socket or a daemon.

### Transformation Path
1. Admission: the action declares its resource class (peak memory, disk written); admission waits until CPU slots, available memory and free disk on the cache and tmp volumes all cover it. A run never starts into a state the host cannot serve.
2. The runner starts the test with a stall detector instead of a wall total: the test is killed only when it shows no progress (no output, no observer event, no CPU time) for the stall window.
3. Inside the test, every wait is on an event: a log line, a state the daemon reports, an observer acknowledgement. The fixture SDK provides the barrier; a sleep loop with a count is refused.
4. A deadline remains only where an RFC timer is the subject (hold time, retransmission). Such a test names the timer, and a red under a host too loaded to serve it is reported as a capacity finding, never a code red.
5. A failure keeps its full output; the runner never elides the daemon stderr of a failed case.

### Boundaries Crossed
Runner to admission registry; runner to child (progress signal); fixture to daemon (event barrier).

### Integration Points
`internal/le/job` admission; `internal/test/fixture` helpers; `internal/test/runner` timeout resolution.

### Architectural Verification
- [ ] No fixture wait is a sleep with a count
- [ ] No runner kill is a wall total
- [ ] Admission counts memory and disk, not CPU alone

## Risks & Assumptions

### Assumptions
| Id | Assumption | Evidence | If wrong |
|----|------------|----------|----------|
| A-1 | every fixture wait has an observable event to wait on | the plugin rows name the missing event each time (End-of-RIB, SSH listening, observer line) | the daemon gains the report it lacks; that is a product change named in the row |
| A-2 | progress is observable for every test kind | the runner already reads child output and the observer stream | count CPU time of the process group as progress |

### Risks
- A stall window that is too long hides a hang for longer. The window is per suite and the runner prints the last progress it saw when it kills.

## Blast Radius
`internal/test/runner`, `internal/test/fixture`, `internal/le/job`, `internal/le/interoplab`, every `.ci` file carrying `option=timeout`, and the unit tests named in the owned rows.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `./le test plugin` under synthetic load | -> | event barriers, stall kill | the suite under 4 CPU burners plus one 192 MB `conv=fsync` writer (the load the rows measured) is green |
| `./le test unit` with free memory below the class | -> | admission | the run waits, then starts; it is never OOM-killed |
| any action with the cache volume below its disk class | -> | admission | refuses before starting, naming the volume, the free space, and `./le scratch trim` |

## Acceptance Criteria

| AC | Condition | Observable |
|----|-----------|------------|
| AC-1 | `./le test plugin`, `reload`, `ui`, `ipsec`, `exabgp-test`, `encode` under 4 CPU burners plus one 192 MB `conv=fsync` writer | green three runs in a row |
| AC-2 | a fixture waits | it waits on a named event; a lint over `internal/test/fixture` refuses a sleep loop with a count and a fixed readiness deadline |
| AC-3 | a `.ci` case | carries no absolute `option=timeout`; the runner kills only on a stall and prints the last progress it saw |
| AC-4 | a test asserts on an RFC timer | it names the timer; a red under host overload is reported `capacity` with the load figures, never `fail` |
| AC-5 | an action starts | admission has checked CPU, available memory and free disk on the cache and tmp volumes against its declared class |
| AC-6 | free disk falls below a running action's class | the action stops with a host-resource error naming the volume and the fix command, and the verdict is not a code red (`no space left on device` is classified, never reported as a test failure) |
| AC-7 | a unit test asserts a wall-clock ratio or ceiling | it is moved to `./le perf`, or rewritten to assert the property (ordering, count) without the clock |
| AC-8 | a test fails | the runner keeps its full output, the daemon stderr included |
| AC-9 | `breakStalled` kills a holder | it waits for the process group to exit before SIGKILL, bounded by the holder's own declared teardown |
| AC-10 | every row in "Owned journal rows" | re-run under the AC-1 load: green, or red with a cause that is not the clock or an unadmitted resource |
| AC-11 | `le go staticcheck check part 6 of 6` under the AC-1 load (its bound is `deadlinePerRow`, 90 s a row, `internal/le/go/staticcheck/judge.go`; on 2026-10-23 the part passed in 62 s a row on 2026-10-21 and missed the 9 min bound under load) | the run judges the matrix, or fails because Staticcheck made no progress for the stall window; total elapsed time alone never ends it |
| AC-12 | `./le go lint run scope <pkgs>` while other sessions edit unrelated files | the scoped run judges its packages; it does not refuse with "lint inputs changed during planning" for edits outside its scope |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Proves |
|------|------|--------|
| `TestStallKillIgnoresWallTotal` | `internal/test/runner/stall_test.go` | AC-3 |
| `TestBarrierWaitsOnTheEvent` | `internal/test/fixture/barrier_test.go` | AC-2 |
| `TestAdmissionWaitsForMemory` | `internal/le/job/admission_test.go` | AC-5 |
| `TestAdmissionRefusesAFullVolume` | `internal/le/job/admission_test.go` | AC-6 |
| `TestENOSPCIsAHostVerdict` | `internal/test/runner/failure_test.go` | AC-6 |

### Functional Tests
| Test | Proves |
|------|--------|
| `./le test plugin` under the AC-1 load | AC-1 |

## Files to Modify
- `internal/test/runner/timing.go`, `parallel.go`, `runner_exec.go`
- `internal/test/fixture/*` readiness waits
- `internal/le/job/job.go`, `registry.go`
- `internal/le/interoplab/zebuild.go`
- `test/**/*.ci` carrying `option=timeout`

## Files to Create
- `internal/test/runner/stall.go`, `internal/test/fixture/barrier.go`

### Integration Checklist
- [ ] Every heavy action declares a resource class

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/testing/runner-architecture.md` - stall kill replaces the wall budget
- [ ] `docs/architecture/testing/ci-format.md` - `option=timeout` retired, barrier steps documented
- [ ] `docs/contributing/running-commands.md` - admission by memory and disk
- [ ] `docs/architecture/testing/interop.md` - lab build bound and interop checkers wait on events (`labBuildTimeout`, `check_extras.go`)
- [ ] `docs/architecture/core-design.md` - read for the admission and job model it states; edited only if it describes CPU-only admission

## Implementation Steps
1. Add the barrier API to the fixture SDK and the lint that refuses counted sleeps.
2. Move fixtures onto barriers, suite by suite, starting with the rows below.
3. Replace `SuggestedTimeout` and `ParallelTimeoutHeadroom` with the stall detector; retire absolute `option=timeout`.
4. Add memory and disk to admission; classify ENOSPC as a host verdict.
5. Move wall-clock ratio assertions to `./le perf`.
6. Run AC-1 and re-check every owned row.

### Failure Routing
| Failure | Route |
|---------|-------|
| a case has no event to wait on | the daemon gains the report; record it in the row, never restore a sleep |

## Design Insights
- A fence widened is still a clock (from the concurrency spec). Waiting longer under contention holds resources and makes it worse.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| event barriers, no clock | load-scaled budgets | measured worse on 2026-09-19 |
| stall kill | wall total | a progressing test is never killed; a hung one still is |
| admission by memory and disk | load average | three OOM kills on 2026-09-06 started at low load average |

## Known Limitations
- Startup that blocks on fsync under disk contention (`storage.Create`, the hub's config source) is product behaviour; this spec makes the verdict honest about it, and any startup change is its own spec.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| gate-verdict-depends-on-the-machine.md | 2026-10-30 | TestRefreshPeerLinkScopesSkipsAnUnchangedTable reads the host neighbor table |
| field-carries-two-meanings.md | 2026-10-27 | .ci cmd timeout= replaces option=timeout as the test budget |
| gate-verdict-depends-on-the-machine.md | 2026-10-27 | verify changed load reds at e6474aa200 |
| gate-verdict-depends-on-the-machine.md | 2026-10-27 | verify changed load reds at 2f36373081 |
| gate-verdict-depends-on-the-machine.md | 2026-10-27 | functional ui, web, parse, plugin timeouts at 0723d7d26a |
| gate-verdict-depends-on-the-machine.md | 2026-10-27 | appliance QEMU guest SSH timeouts at 0723d7d26a |
| gate-verdict-depends-on-the-machine.md | 2026-10-27 | internal/component/cli package 20m timeout |
| gate-verdict-depends-on-the-machine.md | 2026-10-08 | ze init, ze start storage writes |
| gate-verdict-depends-on-the-machine.md | 2026-10-06 | bgp-srpolicy-1-encode, job-enum-srpolicy-owned-syscall-duration-f764e51c.lo |
| gate-verdict-depends-on-the-machine.md | 2026-10-05 | internal/le/repo/changed/selector.go, loadPackageGraph |
| gate-verdict-depends-on-the-machine.md | 2026-09-29 | test/plugin/role-strict-enforcement.ci in the clean run of ./le rfc discrim |
| gate-verdict-depends-on-the-machine.md | 2026-09-25 | ui send-raw-reaches-one-peer (226) and plugin concurrent-config-commit (188 |
| gate-verdict-depends-on-the-machine.md | 2026-09-24 | ./le functional plugin under -p 32 |
| gate-verdict-depends-on-the-machine.md | 2026-09-23 | IS-IS TestLSDBFullDropPath under -race -timeout=2m |
| gate-verdict-depends-on-the-machine.md | 2026-09-23 | Native sweep part 1 on b2197f3e6778 |
| gate-verdict-depends-on-the-machine.md | 2026-09-08 | test/encode/peer-open-four-octet-as.ci, run through ze-test bgp encode |
| gate-verdict-depends-on-the-machine.md | 2026-09-06 | test/exabgp-compat/api/api-reload.ci, run through ./le functional exabgp-te |
| gate-verdict-depends-on-the-machine.md | 2026-09-04 | ./le functional plugin, the whole 730-test suite |
| gate-verdict-depends-on-the-machine.md | 2026-09-04 | ./le integration interop, its ze-interop image build (test/interop/Dockerfi |
| gate-verdict-depends-on-the-machine.md | 2026-09-04 | managed/hub-ca-trust (internal/test/fixture/misc_fixture_managed_ca_trust.g |
| gate-verdict-depends-on-the-machine.md | 2026-09-04 | TestParallelPluginFanOut (internal/component/bgp/server/events_test.go) |
| gate-verdict-depends-on-the-machine.md | 2026-09-03 | ifaceLinkFlap08 (internal/test/fixture/plugin_fixture_08_flap.go) against t |
| gate-verdict-depends-on-the-machine.md | 2026-09-03 | test/plugin/ddos-detect-characterize.ci against the ddos classifier |
| gate-verdict-depends-on-the-machine.md | 2026-08-31 | TestActiveAnswerConsumerStreamsPastQueueDepth (pkg/plugin/rpc/mux_test.go) |
| gate-verdict-depends-on-the-machine.md | 2026-08-24 | test/plugin/plugin-reads-engine-answer.ci |
| gate-verdict-depends-on-the-machine.md | 2026-08-24 | cmd/ze/hub TestEphemeralDaemonStartsSSH |
| gate-verdict-depends-on-the-machine.md | 2026-08-23 | test/ipsec/ipsec-dpd-holds-tunnel.ci |
| gate-verdict-depends-on-the-machine.md | 2026-08-22 | test/plugin/remove-private-as-export.ci |
| gate-verdict-depends-on-the-machine.md | 2026-08-20 | plugin functional suite |
| gate-verdict-depends-on-the-machine.md | 2026-08-19 | test/plugin/remove-private-as-replace-peer.ci |
| gate-verdict-depends-on-the-machine.md | 2026-08-19 | the retired ze-unit-pkg-test PKG=./scripts/dev (current: go test -race ./sc |
| gate-verdict-depends-on-the-machine.md | 2026-08-17 | test/plugin/*.ci wall budget |
| gate-verdict-depends-on-the-machine.md | 2026-08-16 | test/web/chaos-stream-swaps.wb |
| gate-verdict-depends-on-the-machine.md | 2026-08-13 | plugin functional suite |
| gate-verdict-depends-on-the-machine.md | 2026-08-15 | wireu shape cache |
| gate-verdict-depends-on-the-machine.md | 2026-08-15 | plugin and reload functional suites |
| gate-verdict-depends-on-the-machine.md | 2026-08-15 | the retired ze-functional-plugin-test ZE_PLUGIN_PARALLEL=1 (current: ./le f |
| gate-verdict-depends-on-the-machine.md | 2026-08-15 | test/plugin/llgr-readvertise-multipeer.ci |
| gate-verdict-depends-on-the-machine.md | 2026-08-16 | the retired ze-functional-plugin-test (current: ./le functional plugin) und |
| gate-verdict-depends-on-the-machine.md | 2026-08-19 | test/plugin/logging-level-filter.ci wall budget |
| gate-verdict-depends-on-the-machine.md | 2026-08-19 | test/plugin/aaa-radius-admin.ci |
| gate-verdict-depends-on-the-machine.md | 2026-08-19 | the mcp tests' readiness wait |
| gate-verdict-depends-on-the-machine.md | 2026-08-21 | test/install/ze-kernel-no-modcache-mutation.ci, the retired ze-functional-i |
| gate-verdict-depends-on-the-machine.md | 2026-08-22 | test/plugin/filter-family-export-flowspec.ci, the observer's establishment |
| gate-verdict-depends-on-the-machine.md | 2026-08-22 | TestSendExecuteCommandReadsRecords/a long walk reads as a record sequence ( |
| gate-verdict-depends-on-the-machine.md | 2026-08-23 | the retired ze-unit-reactor-test-race (current: go test -race ./internal/co |
| gate-verdict-depends-on-the-machine.md | 2026-08-25 | test_a_stalled_holder_is_broken_and_the_kill_carries_its_evidence (the reti |
| gate-verdict-depends-on-the-machine.md | 2026-08-29 | ./le functional plugin, 703 fixtures, judged from three detached worktrees |
| gate-verdict-depends-on-the-machine.md | 2026-09-06 | ./le functional firewall, the whole suite |
| gate-verdict-depends-on-the-machine.md | 2026-09-06 | INTEROP_SCENARIO=as-path-prepend-two-octet-peer ./le integration interop |
| gate-verdict-depends-on-the-machine.md | 2026-09-06 | the same interop scenario, retried under controlled conditions |
| gate-verdict-depends-on-the-machine.md | 2026-09-15 | ./le integration gtsm (internal/le/integration/gates.go), ./le functional f |
| gate-verdict-depends-on-the-machine.md | 2026-09-15 | test/firewall/gtsm-related-icmp.ci, run as root through ze-test firewall -- |
| gate-verdict-depends-on-the-machine.md | 2026-09-17 | test/plugin/exec-answer-unconditional.ci (271) and test/plugin/explicit-fil |
| gate-verdict-depends-on-the-machine.md | 2026-09-20 | TestBFDClientReStampsTheEntryTimeOnlyOnAChange (internal/component/bgp/reac |
| gate-verdict-depends-on-the-machine.md | 2026-10-14 | test/plugin/session-editor-commit-confirmed-boundary.ci |
| gate-verdict-depends-on-the-machine.md | 2026-10-25 | internal/le/site under -race, TestBuildRendersEveryPublishedRoute |
| startup-wait-expires-under-load.md | 2026-10-09 | terminal-demo startDaemon against ze start in the render container |
| startup-wait-expires-under-load.md | 2026-10-17 | ze startup under fsync load (test/plugin/filter-irr.ci runs) |
| startup-wait-expires-under-load.md | 2026-10-17 | drainSSHRestart 15s SSH poll (fixture/register_drain_signal.go), test/plugi |
| startup-wait-expires-under-load.md | 2026-10-20 | storageCommand 67.5s deadline (fixture/storage_tree_fixture.go), test/plugi |
| startup-wait-expires-under-load.md | 2026-10-20 | TestBuildTag_SSH_AbsentRESTAuthenticatesConfigUser (cmd/ze/hub/build_tag_ss |
| startup-wait-expires-under-load.md | 2026-10-22 | FRR interop startup, BGP configure and SSH host-key creation |
| startup-wait-expires-under-load.md | 2026-10-22 | bgp-filter-irr-loaded-over-ssh, session-editor helper |
| grace-bound-hides-a-missing-signal.md | 2026-09-09 | internal/component/l2tp/ppp/ncp_test.go TestIPResponseConfiguresInterface |
| grace-bound-hides-a-missing-signal.md | 2026-08-31 | internal/component/l2tp/plugins/authradius/coa_test.go, rfc5176_coa_test.go |
| grace-bound-hides-a-missing-signal.md | 2026-08-20 | internal/component/plugin/server/startup_autoload_test.go |
| grace-bound-hides-a-missing-signal.md | 2026-08-21 | internal/component/plugin/server/startup_autoload_test.go |
| grace-bound-hides-a-missing-signal.md | 2026-08-21 | internal/component/plugin/server/startup_autoload_test.go |
| grace-bound-hides-a-missing-signal.md | 2026-09-15 | test/ui/display-fill-completion.ci, internal/test/fixture/ui_fixture_displa |
| grace-bound-hides-a-missing-signal.md | 2026-09-17 | test/ui/display-fill-completion.ci, internal/test/fixture/ui_fixture_displa |
| bound-too-small-for-its-own-burst.md | 2026-10-21 | killGrace (3 s) in (*Admission).breakStalled (internal/le/job/registry.go) |
| bound-too-small-for-its-own-burst.md | 2026-09-07 | Timings.SuggestedTimeout (internal/test/runner/timing.go) and ParallelTimeo |
| bound-too-small-for-its-own-burst.md | 2026-10-09 | labBuildTimeout (5 min, internal/le/interoplab/zebuild.go) and linuxBuildTi |
| full-disk-false-red.md | 2026-10-23 | Mac root, 34.1G free |
| full-disk-false-red.md | 2026-10-21 | Mac root /dev/disk3s1s1, 117M to 474M free of 927G from 18:14 |
| full-disk-false-red.md | 2026-10-21 | Mac root /dev/disk3s1s1, 391M free of 927G |
| full-disk-false-red.md | 2026-10-15 | TMPDIR (~/.cache/tmp on /dev/sdb) |
| full-disk-false-red.md | 2026-10-08 | Proof artifacts after owner-reported disk exhaustion |
| full-disk-false-red.md | 2026-09-21 | test/ui/le-rfc-answers.ci, extracting rfc/short into its own temporary tree |
| full-disk-false-red.md | 2026-09-19 | ./le commit debt-clear all of 6 run repeatedly without a clean between |
| full-disk-false-red.md | 2026-09-04 | cacheClean (internal/le/scratch/), and the Edit tool's atomic rename |
| full-disk-false-red.md | 2026-09-03 | three phase agents at once, and what a full device does to a DISCRIMINATION |
| full-disk-false-red.md | 2026-09-03 | the agent harness's own tool-result path under /private/tmp/claude-502/, an |
| full-disk-false-red.md | 2026-09-02 | ze appliance kernel --target runtime, kernelbuilder.dockerArgs (internal/ap |
| full-disk-false-red.md | 2026-09-01 | ./le verify worktree, twice |
| full-disk-false-red.md | 2026-08-18 | ./le repository tracked-build check |
| full-disk-false-red.md | 2026-09-09 | ./le verify lint run, the typecheck pass over cmd/ze and several unrelated |
| full-disk-false-red.md | 2026-09-11 | ./le --name blog-six-voice site build, command catalog compilation |
| full-disk-false-red.md | 2026-09-13 | go test over ./internal/le/... under ./le job run, closing the spec |
| full-disk-false-red.md | 2026-09-24 | ./le functional reload, building the isolated binaries into tmp/session/<id |
| package-red-not-reproduced.md | 2026-10-10 | cmd/ze/hub unit tests (go test -race, ze_core plus feature-gate tags) |
| package-red-not-reproduced.md | 2026-10-18 | test/plugin/storage-rir-restart.ci (plugin 871) |
| false-synchronization-claim.md | 2026-10-04 | peerCounter09 versus Peer.sendInitialRoutes, Linux raw-policy smoke |
| false-synchronization-claim.md | 2026-10-04 | bgp-labeled-withdraw-compatibility-frr/inject.msg, checkLabeledWithdrawComp |
| false-synchronization-claim.md | 2026-09-21 | test/plugin/attach-process-dynamic-group.ci, the DYN-GROUP: the dynamic mem |
| false-synchronization-claim.md | 2026-09-19 | the plugin functional suite as a POPULATION, not any one case |
| false-synchronization-claim.md | 2026-08-10 | plugin functional test |
| false-synchronization-claim.md | 2026-08-13 | plugin functional test |
| false-synchronization-claim.md | 2026-08-13 | plugin functional test |
| false-synchronization-claim.md | 2026-08-13 | interop scenario |
| false-synchronization-claim.md | 2026-08-13 | cli config editor functional test |
| false-synchronization-claim.md | 2026-09-05 | plugin functional test |
| false-synchronization-claim.md | 2026-09-17 | test/plugin/ipv6-announce-withdraw.ci, test/plugin/rfc6793-ingest-collapse. |
| assertion-reads-state-the-scheduler-owns.md | 2026-10-19 | TestDHCPv4LoopHoldsLeaseFiveLeasePeriods/renewed_every_T1 (internal/plugins |
| assertion-reads-state-the-scheduler-owns.md | 2026-10-19 | TestRunSmallBenchmark (internal/perf/perf_test.go, the d < 0 check over sen |
| assertion-reads-state-the-scheduler-owns.md | 2026-10-17 | TestRunnerHangStacksLieOutsideTempDir (internal/component/cli/testing/runne |
| assertion-reads-state-the-scheduler-owns.md | 2026-10-15 | test/plugin/drain-budget-over-ceiling-refused.ci, fixture drainBudgetOverCe |
| assertion-reads-state-the-scheduler-owns.md | 2026-10-02 | interop ospf-ext-prefix-link-frr (interoplab/bgp check_extras.go, first ext |
| assertion-reads-state-the-scheduler-owns.md | 2026-10-02 | interop ospf-opaque-frr, ospf-te-frr (interoplab/bgp check_extras.go) on ma |
| assertion-reads-state-the-scheduler-owns.md | 2026-09-04 | TestRFC2869InterimLoopKeepsOneGenerationOutstanding (internal/component/l2t |
| assertion-reads-state-the-scheduler-owns.md | 2026-08-31 | TestRFC2869InterimLoopKeepsOneGenerationOutstanding (internal/component/l2t |
| assertion-reads-state-the-scheduler-owns.md | 2026-09-11 | test/plugin/peer-selector-name-and-ip.ci, which asserts two plugin updates |
| assertion-reads-state-the-scheduler-owns.md | 2026-10-21 | TestReloadRemovalFailureRecoversCommittedService (internal/component/plugin |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-08-16 | test/plugin under high -p |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-08-15 | test/plugin under high -p |
| parallel-copies-collide-on-a-deterministic-port.md | 2026-08-17 | the ipsec functional suite under the retired scripts/evidence/qemu-all-test |
| reload-rolls-back-instead-of-applying.md | 2026-08-25 | test/traffic/traffic-reload-apply.ci fails one run in eight |
| test-against-broken-path.md | 2026-09-17 | audit-config-commit fixture exceeds its 5-second HTTP deadline |
| concurrent-rfc-gate-stale.md | 2026-08-15 | test/plugin/test-pipe-first-last.ci observer sees count 0 for its whole budget |
| gate-red-where-nothing-blocks-on-it.md | 2026-09-07 | static-interface-nexthop-no-backend passes or times out at its 15s budget |
| gate-red-where-nothing-blocks-on-it.md | 2026-10-12 | test/plugin/rpki-decorator-merge.ci fails 1 in 20 under stress-repro |
| gate-verdict-depends-on-the-machine.md | 2026-08-15 | test/plugin/llgr-readvertise-multipeer.ci failing on an idle machine (second row) |
| gate-verdict-depends-on-the-machine.md | 2026-09-04 | relay rail: route received on one peer never reaches the interop daemon |
| gate-verdict-depends-on-the-machine.md | 2026-10-07 | TestWriteAnswerReleasesBufferOnError counts 28 allocations under -race |

## Rows found already fixed

None among the rows this spec owns. Each was re-read against today's producer;
the timing mechanism each names is still present.

## Checklist

### Goal Gates (MUST pass)
- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after it
- [ ] `./le test unit` over `internal/test/runner`, `internal/test/fixture`, `internal/le/job`, under `./le job run`
- [ ] AC-1 load run of `./le test plugin`, by the main thread or a fresh agent on a host that may run full suites
