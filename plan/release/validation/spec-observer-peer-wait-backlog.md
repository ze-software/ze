# Spec: observer-peer-wait-backlog

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release validation: 151 functional fixtures whose observer can stop ze before its scripted peer has its messages, so their verdict depends on host load and a green run proves less than it claims |
| RFC gaps | - |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-11-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The owner approved a one-off WIP-cap exception for this implementation session
(`ZE_SPEC_WIP_CAP=24` for the claim).

## Task

A compiled observer (`internal/test/fixture`) runs its scenario once the
plugins are ready, then `Observe` quiesces and asks the daemon to stop.
`request quiesce` reports a peer whose TCP connect has not completed as
settled (`docs/architecture/api/commands.md`, "What the barrier does NOT
cover"), and `docs/functional-tests.md` ("Waiting without sleep") puts the wait
on the observer: "Before requesting shutdown, an observer waits for the peer
state or sent-payload counter its wire assertions require." An observer that
ends without that wait makes ze send Cease to a `le test peer` that still
expects its messages. The test then passes or fails with the host's load.

The all-family guard `TestPluginObserversWaitForTheScriptedPeer`
(`internal/test/fixture/plugin_fixture_peer_wait_test.go`, committed in
7e68ebce69 with its false-pass cases in
`TestPluginObserverPeerWaitGuardRefusesFalsePasses`,
`plugin_fixture_peer_wait_guard_test.go`) reads every registration and its
`.ci`, and refuses an observer that never waits, by name, address or count, for
a scripted peer that expects ze's wire output. It sits beside the family 08 and
12 guards (`TestPluginObservers08WaitForTheScriptedPeer`,
`TestPluginObservers12WaitForTheScriptedPeer`), which stay as they are at HEAD
and are green: those families were fixed in dd11f5d194, 17d5fb358d and
7db9137e34. Over every family it reports **182 scripted peers across 151
fixtures** that the observer never waits for (list taken by running the guard
at HEAD on 2026-11-02, after 7e68ebce69; regenerate it the same way). The
first count this spec recorded, 281 across 240 (d96f79bf99), came from an
earlier form of the guard that did not yet read the event-stream and poll
waits below.

| Family | Missing waits | Wait helpers the guard reads for the family (`observerWaitFamilies`) |
|--------|---------------|----------------------------------------------------------------------|
| 01 | 11 | `plugin01WaitCounter` |
| 02 | 6 | `eorSent02`, `routeServerObserver02`, `waitUp02` |
| 03 | 5 | `waitPeerCounter03`, `waitPeersEOR03` |
| 04 | 5 | `waitPeerEOR04`, `rsObserver04` |
| 05 | 15 | none |
| 06 | 9 | `fixture06WaitEOR`, `fixture06DeadpeerHolddown` |
| 07 | 2 | `waitEOR07` |
| 09 | 16 | `waitL2TPBGPEOR09` |
| 10 | 10 | `fixture10WaitEOR`, `fixture10PeerCounter` |
| 11 | 3 | none |
| 13 | 10 | `waitEORSent13`, `routeServerReplay13` |
| 14 | 35 | `plugin14PollCommand`, `blackholeReadvertiseFence` |
| 15 | 16 | `plugin15WaitEOR` |
| 16 | 13 | `plugin16WaitEOR` |
| misc | 5 | `waitPeerEORSent` |
| other (register_*.go and the rest) | 21 | `Poll` (a poll whose predicate reads a peer), `awaitGRState` |

The drivers carrying the most findings: `plugin14Observer` (19),
`plugin15Observe` (14), `plugin14EventObserver` (11), then `observer02`,
`observe04` and `p05` scenarios with five or fewer each. Of the 151 fixtures,
145 are `test/plugin/`, 5 `test/reload/` and 1 `test/ui/`; 17 carry
`option=needs-linux`. Six findings are inline scenarios (registered as a
literal function): `bgpls-export-lifecycle`, `errors-config-abort-show`,
`rib-reconnect`, `rib-withdrawal`, `teardown-tunnel` and
`update-text-prefix-sid`.

The goal: every observer waits for each scripted peer its `.ci` expects
output on, the guard over all families reports zero findings, and every
changed fixture passes three runs in a row. This is the work the sleep rule
asks for (`ai/rules/points/testing/ci-sleep-justification/justify-every-sleep-in-a-ci-test.md`:
"A sleep MUST be converted to a deterministic wait whenever a condition exists
to wait on"), applied to the compiled observers.

### Event-stream and poll waits: already implemented in the guard

This spec first recorded, as an open design point, that drivers waiting
through an event stream should count as waits. The committed guard does that
already, so no guard change is owed for it:

| Wait shape | Where the guard reads it (`observerWaitFamilies`) | What it counts |
|------------|----------------------------------------------------|----------------|
| `routeServerObserver02` | family 02, count at argument 1 | Distinct peers sent their End-of-RIB, from sent UPDATE events |
| `waitUp02` | family 02, fixed `*` | A state event that a session came up; `*` names a peer only when the `.ci` scripts one |
| `rsObserver04` | family 04, count at argument 0, gated on argument 2 (`requireReplay`) being the literal `true` | With `requireReplay` false it waits on no peer, so `bgp-rs-reactor-fastpath-fallback` stays a finding |
| `routeServerReplay13` | family 13, fixed count 2 | Two distinct peers sent their End-of-RIB; a `.ci` scripting more stays a finding (`waitedOnBy`: count >= scripted) |
| `awaitGRState` | other, selector at argument 2 | The peer's gr-state row, which appears once its GR session went down |
| `Poll` with a poll-read helper (`fixture10PeerCounter`) | other and family 10 | The peer a `show bgp peer <selector>` command or poll-read selector in the predicate names |
| `passiveObserve09`, `passivePlugin13`, `p12WaitForDaemonDriver` | `exempt` of families 09, 13, 12 | Nothing: these drivers end no run. They stay alive until the daemon says goodbye, so they never ask for the shutdown the wait protects |

The initial-sync `eor-sent` counter advances after `sendUpdateHeld` writes and
flushes the marker (`Peer.sendInitialRoutes`, `session_write.go`
`Session.sendUpdateHeld`). A sent UPDATE event in `Session.writeUpdateBody`
can fire before the buffered bytes are flushed. An event-stream observer's
distinct-peer End-of-RIB count therefore establishes that those peers have live
sessions and have emitted their markers into the send path; `Observe` then
requests quiesce before shutdown to complete delivery. The event alone does
not prove a completed flush. The gated-count regression (AC-4) pins the
`requireReplay` condition so that removing the gate makes the test fail.

### Implementation audit, 2026-11-04

The main-checkout baseline already passes the all-family, 08 and 12 guards.
The named launcher ran `go test -race -v` over `internal/test/fixture`, with
the feature tags from `feature-gates.txt`, selecting
`TestPluginObservers(WaitForTheScriptedPeer|08WaitForTheScriptedPeer|12WaitForTheScriptedPeer)$`.
No observer waits were added by this wiring phase. Earlier implementation
commit `73978cd75e` and existing shared-checkout edits must be audited rather
than repeated. AC-2's consecutive fixture runs and AC-3/AC-4 discrimination
proofs were still owed at that baseline.

### Implementation evidence, 2026-11-05

All 208 affected CI files passed three consecutive runs: 191 native files
and all 17 Linux files, totaling 624 successful invocations with zero skips.
The result covers the retained guard baseline and every reached alias added
by the dependency audit. The final CI hashes match the reviewed population.
Earlier failed attempts and baseline runs are excluded from that count.

Native proof ran in the main checkout through each registered suite:
`le test bgp plugin <fixture>`, `le test bgp reload <fixture>` and
`le test ui <fixture>`, with frozen host-built binaries and explicit
`LE_TEST_NO_BUILD=1` and `ZE_BIN`. Linux proof used the own-system
`le test qemu docker-lab kernel <runtime-kernel> command <guest-script>`
carrier. Its root guest bound the same main checkout at `/workspace` and ran
`le test qemu all test <test-path>` for each CI. Kernel `7.3.0-ze`, PID 1
`/gokrazy/init` and the Ze root filesystem were checked; each invocation
reported one executed test and no skipped phase. The implementation remains
in-progress for the separate closure phase; this evidence is not a Review Gate.

AC-1, AC-3, AC-4 and AC-7 are complete. The feature-tagged, race-enabled
`go test` runs of `internal/test/fixture` passed the all-family guard, its
false-pass cases, and the unchanged 08 and 12 guards. Removing the selected
waits produced eight fixture/peer findings covering families 05, 11, 14,
other and an inline scenario. Exact restoration returned the guard to zero
findings. Removing the replay gate made the disabled-count case fail while
the enabled control passed. The restored false-pass test passed.

The final fixture-package run also passed the real TLS quiesce regressions
and all three `awaitObserverResult` controls named in the TDD Test Plan.
`observeConfigured` and `runPlugin13` now require a completed final drain
before publishing successful shutdown. The latter drains only when it owns
shutdown, preserving the externally stopped scenarios. Custom owners report
their errors while the daemon can still relay them.

Fresh binaries were built from the main checkout and frozen as binary
artifacts. `testfunctional.PrepareGuest`, reached through
`./le test qemu guest-binaries`, produced `ze`, `ze-stripped`,
`ze-stripped-flow` and `ze-fakefilter`. The fresh Linux/amd64 harness was
pinned for both native and guest use. That build read main directly and
retained binary artifacts. Fixture-package evidence uses admitted
`go test -race` runs with the feature tags from `feature-gates.txt`.
Functional evidence uses each file's
registered suite, including `le test bgp plugin <fixture>`, with
`LE_TEST_NO_BUILD=1` and explicit `ZE_BIN`.

All 191 native CI files have now passed three consecutive runs each. The
first batch supplied 128 triples. After the startup repair, the resumed
plugin batch supplied 58 triples, including the seven additional aliases.
That private script then selected the obsolete reload command namespace and
exited before running a reload CI. The five reload files subsequently passed
three times each through the documented `le test bgp reload <fixture>` route.
The verified union of those results exactly matches the 191-file native
manifest. The completed Linux proof below closes AC-2's remaining population.

The startup diagnosis has an observed RED in both runner paths.
`TestPeerStartupUsesCaseBudget`, `TestPeerStartupReportsExit`,
`TestPeerStartupHonorsDeadline` and `TestPeerStartupHonorsCancellation`
in `internal/test/runner/peer_startup_test.go` failed their eight subcases.
The tests expose the five-second cap, lost early-exit attribution, and lost
deadline or cancellation cause. Both production call sites now use
`awaitPeerListening` in `internal/test/runner/peer_startup.go` with their
existing bounded case context and owned peer completion. All four tests
passed after the repair, and the full runner package passed with race enabled.

`TestDockerLabCommandRunsInZeSystem` in
`internal/le/test/qemu/dockerlab_command_integration_test.go` passed an
actual own-system command run, including kernel, PID 1, root filesystem and
private scratch writeback checks. The earlier Linux attempt could not start
its first selected CI because the image's BusyBox `timeout` rejects `-k`.
The native deadline regression reproduced that failure through the suite
consumer before the production repair.

`allTestsRun.suiteChild` in `internal/le/test/qemu/alltests_deadline.go` now
owns that deadline through `ServerProcess.StopGroup`. The real consumer
tests `TestQEMUAllSuiteDeadlineUsesNativeOwnership` and
`TestQEMUAllSuiteDeadlineRefusesUnboundedBudgets` passed after the repair.
They prove normal exit statuses and tally, group TERM, the 15-second grace
before forced group KILL, ordinary expiry 124, forced KILL 137, and refusal
of invalid budgets before child startup. The scoped QEMU package run and
full fixture package also passed with race enabled. This deadline repair
landed in `12afa215f6`.

With the fresh harness, `test/plugin/cos-dynamic-coa.ci` and
`test/plugin/cos-dynamic-session.ci`, followed by
`test/plugin/ddos-bps-amplification.ci`, `test/plugin/ddos-direction.ci` and
`test/plugin/ddos-flow-recent.ci`, each passed three consecutive runs through
`le test qemu all test <test-path>` in Ze's own system. Every invocation
reported one executed test and zero skipped phases. These supplied the first
five Linux triples; the subsequent proof below covers additional fixtures.

The first `test/plugin/ddos-incident-confidence.ci` attempt then failed its
unchanged confidence assertion. The run opened an incident and had nonzero
eBPF traffic counters, but `show flow recent` remained empty. The frozen
non-appliance DUT's `ensureConntrackTracking` deliberately leaves tracking
setup to its host. The older `Run.setupCommand` in
`internal/le/test/qemu/run.go` enabled conntrack accounting and installed a
tracking hook. The own-system `qemu all` routes omitted that preparation.

`TestQEMUAllPreparesConntrackBeforeSuites` and
`TestQEMUAllRefusesConntrackPreparationFailure` in
`internal/le/test/qemu/alltests_conntrack_test.go` reproduced the missing
preparation and missing refusal through both whole-run and one-CI entry
points. All four subcases were observed RED. Both paths now call
`allTestsRun.prepareConntrack` before a suite. It checks root and Ze-system
identity, enables and reads back accounting, and atomically replaces only its
reserved tracking chain. Its native process owner bounds setup to 30 seconds
plus the existing 15-second cleanup grace. All four subcases passed after the
repair; the scoped QEMU package passed with race enabled. Five existing
test-count adjustments received independent approval before they changed.
The preparation repair landed in `3265a72946`, carrying all five approvals as
Git trailers.

The repaired guest started with accounting at zero. Preparation read back
accounting at one and exactly one tracking rule. The unchanged confidence CI,
`ddos-policy`, `ddos-show` and `flowspec-fw-legacy-table-removed` then each
passed three consecutive runs. Every invocation executed one test without
skips and preserved the unrelated sentinel nftables table. Together with the
prior five triples, these establish nine of the 17 Linux fixtures.
The next `flowspec-fw-withdraw-removes-table` attempt failed with
`no peer process to stop after the observed withdrawal`; its peer had
completed the wire exchange and was lingering. `peerPIDs07` still looked for
the old `peer` argument at the first position rather than the actual
`test peer` command. This is a separate fixture failure, not missing
conntrack data.

`TestFlowSpecPeerLookupUsesCaseProcess` in
`internal/test/fixture/plugin_fixture_07_peer_lookup_linux_test.go` reproduced
that lookup failure with five live owned children: a canonical peer and four
excluded legacy, command, case-directory and executable controls. The old
lookup selected the legacy child and missed the canonical peer. After that
RED, `peerPIDs07` was changed to require `test peer`, the current case's
working-directory inode and the observer harness's executable inode.
The test passed after repair, together with the all-family zero-findings
guard, unchanged 08 and 12 guards, false-pass controls, strict TLS consumer
regressions and `awaitObserverResult` controls. The scoped fixture package
passed with race enabled, and the spec citation-anchor command exited zero.
A fresh harness carried this repair; the DUT binaries remained unchanged.
`flowspec-fw-withdraw-removes-table` then passed three consecutive runs,
including the peer-loss and final table-removal assertions. All seven ASPA
fixtures also passed three times each. Those final eight triples, together
with the earlier five and four, exactly cover all 17 Linux files. Repeated
conntrack preparation retained one checked rule and preserved the unrelated
sentinel table throughout the repaired runs. The earlier five triples alone
did not prove the new conntrack preparation or lookup repair; their actual
consumer regressions and the later guest runs supply that proof.

The policy recovery repair landed in `4007003754` after independent approval
of the paired `internal/test/fixture/policy_final.go` and
`test/plugin/policy-final-disposition.ci` changes. The named draft passed
three times through `le test bgp plugin --draft --pattern policy-final-disposition`.
The promoted live CI also passed three times through its registered plugin
suite. Those passes are included in the completed native population.

Independent full-CI placement reviews covered 201 paths in two batches of
113 and 88. The parent bound those reviews to the frozen fixture sources
with zero source drift at that check. A later dependency audit read seven
additional live aliases and their reached source. The current planned
population is therefore 191 native and 17 Linux CI files, 208 in total.
Source review is separate from execution. AC-5's sleep criterion passed:
`./le doc wiring` reported `ci-sleep ratchet OK (0 <= ceiling 0)`, retaining
the zero baseline. Every path reference resolved. The overall documentation
command still exited 1, reporting 143 drift findings and three inventory
anchor issues across the shared unpushed change set. This is not a passing
global documentation gate. The two owned claim findings are handled by the
netlink lifecycle clarification and the Stop reviewed-claim row for
`12afa215f6`.
Implementation stopped in-progress at Phase 6/6. The separate `/ze-close`
phase records its audit and Review Gate below.

The retained pre-implementation guard output from 2026-11-02 contains 185
findings over 153 fixtures, including 17 `needs-linux` fixtures. This is a
larger retained baseline than the task's 182 findings over 151 fixtures.
Preserve both counts. Do not infer the identities of the two historical
extra fixtures from the current tree without evidence. Every current guard
binding is covered by the source audit, and the expanded live population
retains the larger baseline rather than reducing it.

The seven additional native consumers are included in the completed 191-file
native proof:

- `test/plugin/clear-irr-cache-external.ci`
- `test/plugin/llgr-import-no-llgr-forked.ci`
- `test/plugin/role-learned-cleared-on-peer-delete-forked.ci`
- `test/plugin/rfc8212-ibgp-no-policy-unchanged.ci`
- `test/plugin/llgr-rib-stale.ci`
- `test/plugin/llgr-readvertise.ci`
- `test/plugin/llgr-readvertise-forked.ci`

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - "Waiting without sleep (the quiesce barrier)" and "Route Delivery Synchronization"
  → Constraint: plugin registration readiness is not BGP establishment; before requesting shutdown an observer waits for the peer state or sent-payload counter its wire assertions require
  → Constraint: the check peer must linger while observer work remains
- [ ] `docs/architecture/api/commands.md` - "Quiesce Barrier", "What the barrier does NOT cover"
  → Constraint: quiesce does not wait for a peer still connecting, so it never replaces the per-peer wait
- [ ] `docs/architecture/testing/ci-format.md` - sleep markers and the ratchet
  → Constraint: a sleep that stays carries `// sleep(<kind>): <reason>`; the ratchet caps how many exist, so a removed sleep is not re-added elsewhere
- [ ] `ai/patterns/functional-test.md` - shape of a compiled fixture
  → Constraint: a compiled Go fixture returns an error when an assertion fails (le-tooling door)
- [ ] `ai/doors/le-tooling.md` - the door for `internal/test` and `test/`
  → Constraint: a bare `go test` omits feature tags; run the package through `./le job run label unit-pkg command go test -tags ...`
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux` and runs in the QEMU VM

### Rules
- [ ] `ai/rules/points/testing/ci-sleep-justification/justify-every-sleep-in-a-ci-test.md`
  → Constraint: names the observer peer-wait guards as the check for compiled observers
- [ ] `ai/rules/testing.md` and `ai/META-DECISIONS.md`, "Test changes"
  → Constraint: a change that could lower what a test proves is judged by an independent `ze-test-judge`; no `.ci` expectation is weakened to turn a red green

**Key insights:**
- The guard reads waits only from calls to helpers listed in `observerWaitFamilies`, with peer names as string literals, package constants or parameters bound to them; a helper called with an unbound variable names nothing.
- A registration's families decide which helpers count for it, and the `other` family's `Poll` and `awaitGRState` count where a file falls in it; a family without a helper calls an existing helper rather than gaining a copy, and that helper is added to the family's entry.
- The seven precedent commits fixed only the wait; none changed a `.ci`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/fixture/plugin_fixture_peer_wait_test.go` - the all-family guard: `observerWaitFamilies`, the wait kinds (selector, fixed, command, count, poll, poll-read), the gated count, `exempt`, `scriptedPeers`, `waitedOnBy`
- [ ] `internal/test/fixture/plugin_fixture_peer_wait_guard_test.go` - `TestPluginObserverPeerWaitGuardRefusesFalsePasses`: red cases and controls for each false pass the judges found, including the gated-count case
- [ ] `internal/test/fixture/plugin_fixture_04_route_server.go` - `rsObserver04`: event-stream count of distinct End-of-RIB peers, skipped when `requireReplay` is false
- [ ] `internal/test/fixture/plugin_fixture_13.go` - `routeServerReplay13`: two distinct End-of-RIB peers from events and `eor-sent`
- [ ] `internal/test/fixture/misc_fixture_observers.go` - `waitPeerEORSent` (shared selector wait on `eor-sent`), `quiesce`
- [ ] `internal/test/fixture/plugin_fixture_08.go` - `waitPeersEOR08`, `waitPeerCounter08` (precedent)
- [ ] `internal/test/fixture/plugin_fixture_12.go` - `p12WaitPeersEOR`, `p12WaitPeerCounter` (precedent, budget-scaled through `WaitAttempts`)
- [ ] `internal/component/bgp/reactor/session_write.go` - `Session.writeUpdateBody` emits the sent event after a buffered write, before flush. `Session.sendUpdateHeld` flushes before the initial-sync `eor-sent` counter advances.

**Behavior to preserve:**
- Every existing wire and scenario assertion. Any necessary `.ci` expectation edit requires an independent `ze-test-judge` approval before commit; the paired policy recovery repair is recorded above.
- The family 08 and 12 guards and their fixtures stay green, unchanged.
- An observer's own assertions and its error text.
- The guard's event-stream, poll and exemption readings above.

**Behavior to change:**
- Each of the 182 findings: the observer waits for that scripted peer before it ends.
- The guard gains a pinned false-pass case for the gated count (AC-4).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| All-family observer peer-wait guard | `internal/test/fixture/plugin_fixture_peer_wait_test.go` `TestPluginObserversWaitForTheScriptedPeer`, `observerWaitFamilies` (committed 7e68ebce69) | Refuses an observer that never waits for a scripted peer; reads event-stream counts, gated counts, polls and exemptions; the task recorded 182 historical findings | Applies as the acceptance check; the restored current source reports zero findings |
| False-pass cases of the guard | `plugin_fixture_peer_wait_guard_test.go` `TestPluginObserverPeerWaitGuardRefusesFalsePasses` | Red cases with controls for each way a registration could look as if it waits | Extends: one gated-count case and its control (AC-4) |
| Family 08 and 12 guards | `plugin_fixture_08_peer_wait_test.go`, `plugin_fixture_12_peer_wait_test.go` | Run the same check over one family; green | Does not change: they keep guarding 08 and 12 as they are at HEAD |
| Per-peer wait helpers | `waitPeerEORSent` (`misc_fixture_observers.go`), `p12WaitPeersEOR`, `waitPeersEOR08`, and the family helpers in the Task table | Poll `eor-sent` (or a counter) for a named peer | Extends: callers. Families 05 and 11, and the inline scenarios, call an existing helper, never a new per-family copy (`plan/journal/helper-bypassed-by-an-open-coded-copy.md`) |
| Event-stream replay observers | `routeServerObserver02`, `rsObserver04`, `routeServerReplay13` | Count distinct peers sent an End-of-RIB from sent UPDATE events; the guard already reads them as count waits | Retains the count semantics and requires a checked final drain for shutdown owners. The `requireReplay=false` fixture gains a named source wait and retains its forwarding assertion. |
| Quiesce barrier | `misc_fixture_observers.go` `quiesce`, `internal/component/plugin/server/quiesce.go` | Drains pending async work but treats a still-connecting peer as settled | Complements each peer wait after the last emit. `observeConfigured` and custom shutdown owners now reject a failed, refused or incomplete drain before reporting success. |
| Budget-scaled polling | `Poll`, `WaitAttempts`, `WaitBudget` (`internal/test/fixture`) | Bounded waits that scale with the run's headroom | Extends: new waits use these bounds, never a fixed `time.Sleep` |
| Sleep ratchet | `./le doc wiring`, `// sleep(<kind>):` markers (`docs/architecture/testing/ci-format.md`) | Counts and justifies sleeps | Applies: a sleep a wait replaces is removed; none is added |
| TACACS+ observer guard | `TestTacacsObserversWaitForWhatTheirCIExpects` (`plugin_shell_extra_4_tacacs_test.go`) | Checks mock records, not BGP peers | Does not apply: no scripted BGP peer |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A functional run: `./le test bgp plugin <name>`, `./le test bgp reload <name>` or `./le test ui <name>` reads `test/<suite>/<name>.ci`, starts ze, each `le test peer` stdin block and the compiled observer.
- A kernel-dependent run uses the own-system command carrier, then `le test qemu all test <test-path>` inside its prepared Ze guest. The selected route prepares conntrack and the declared namespace resources before starting the suite.
- The guard: a Go test that parses `internal/test/fixture` source and each registered `.ci`.

### Transformation Path
1. The observer registers, receives `OnAllPluginsReady`, runs its scenario.
2. The scenario's wait helper polls `show bgp peer <name> detail` for the state or counter its assertions require. Initial-sync `eor-sent` follows flush; a sent event alone establishes a marker in the send path.
3. The scenario asserts and returns. A successful shutdown owner requires a completed final quiesce, reports any error while the daemon can relay it, publishes its verdict, then requests shutdown.
4. The guard maps each registration to its `.ci`, resolves the scripted peers, walks every function the registration reaches, and compares.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Observer ↔ engine | SDK `Dispatch` of `show bgp peer <name> detail`, sent UPDATE events and checked quiesce | Yes: all 191 native and 17 Linux triples; real authenticated TLS regressions also exercise successful, refused and lost drain replies |
| Runner ↔ peer | `le test peer` stdin script, `expect=bgp:` lines | Yes: all 208 affected CI files passed 3/3 without skips through their registered suites |

### Integration Points
- `observerWaitFamilies` - each helper and count driver is one entry, beside its family.
- `Observe` - the peer wait sits in the scenario or its wrapper. `observeConfigured` now propagates quiesce transport errors, and custom owners retain that final barrier and failure-report ordering.
- Runner and QEMU ownership - readiness uses the existing case context; native deadline and setup children use the sole-reaper process owner.
- Stage 1 filter declarations - `Server.declaredFilters` requires `Registration.Done`, so allocated but unfinished registration cannot reject a valid startup reference as an authoritative empty declaration.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Waits read the daemon through SDK commands, as the precedents do |
| No unintended coupling (components stay isolated) | Yes | Fixtures use the existing SDK; QEMU uses the existing runner lifecycle owner. The plugin declaration repair stays in the server's existing registration resolver. |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new per-family helper; existing helpers called |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Test code |
| Registration over hardcoding, outbound | Yes | A new wait shape is one table entry in `observerWaitFamilies`, which is the guard's only list |
| Registration over hardcoding, inbound | Yes | Searched: `observerWaitFamilies` is the one list the guard reads; no other list names wait helpers |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The all-family guard is committed, beside the unchanged 08 and 12 guards | `git log -- internal/test/fixture/plugin_fixture_peer_wait_test.go` names 7e68ebce69 | - | That commit, read 2026-11-02 | confirmed |
| A-2 | An initial-sync `eor-sent` counter follows a successful flush; a sent End-of-RIB event establishes a live session whose buffered output `Observe` then quiesces before shutdown | `peer_initial_sync.go` `Peer.sendInitialRoutes`, `session_write.go` `Session.sendUpdateHeld` and `Session.writeUpdateBody`, `fixture.go` `observeConfigured` | An event-only caller would need a delivery barrier before shutdown | Read the sending and observer producers in the main checkout, 2026-11-04 | confirmed |
| A-3 | Each scripted peer can reach the state or marker its observer needs; initial-sync waits using `eor-sent` reach at least one completed marker | Precedent fixes (`waitPeersEOR08`, `p12WaitPeersEOR`) and the reviewed scenario-specific waits | A peer with no family negotiates no End-of-RIB (`plan/journal/silent-fall-through.md`, 2026-08-16); its lifecycle observer must use the appropriate sent-payload counter or Established state instead | All 208 affected CI files passed three consecutive runs without skips, including all 17 Linux files on Ze's system. Initial-sync marker waits and post-emit final drains are distinguished by A-2. | confirmed |
| A-4 | No finding is a guard false positive | Historical guard findings included existing `p05PeerCounter` poll waits that the guard did not recognize | A guard finding can misclassify an existing runtime wait | Commit `73978cd75e` replaces named `peer1` polls in `p05ConfigAdjRIB`, `p05ConfigGroupUpdates` and `p05CursorReplay` with the canonical helper. Current guard bindings are covered by the source audit. | broken |

### Wrong assumptions found during implementation

| Assumption | Evidence | Correction |
|------------|----------|------------|
| Every sent UPDATE event follows a completed flush | `Session.writeUpdateBody` calls the sent callback after `bufWriter.Write`, before its caller flushes | Distinguish post-flush initial-sync counters from sent events, and retain `Observe`'s subsequent quiesce barrier |
| Every historical finding names a missing runtime wait | Before `73978cd75e`, `p05ConfigAdjRIB`, `p05ConfigGroupUpdates` and `p05CursorReplay` already polled named `peer1` `eor-sent` through `p05PeerCounter`. The guard did not recognize that poll-read shape. | Record these as historical guard false positives. The canonical helper rewrite preserves their waits, and the current guard has no finding for them. Keep recognition rewrites separate from new runtime waits when reporting the historical population. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A wait added before a scenario that must act before the peer is up (peer create, admin-down) | The fixture times out waiting | Put the wait after the action the `.ci` scripts; the guard reads it anywhere in the reached functions |
| R-2 | A scenario that emits and then returns still ends before the emitted UPDATE is on the wire | Intermittent red after the initial-sync wait | Wait for the needed post-emit counter or event, then require a completed checked quiesce after that final gate and before shutdown. `p12EmitOnWire` identifies counter progression; the counter alone can precede flush, so it does not replace the final drain. |
| R-3 | Host load makes a 3/3 run red for a reason outside the fixture | Timeouts across unrelated fixtures, load average far above core count | Name the load and rerun only that fixture (`ai/META-DECISIONS.md`: find the cause, no blind retry); `plan/journal/gate-verdict-depends-on-the-machine.md` |
| R-4 | The gated count loses its condition in a later guard edit and `bgp-rs-reactor-fastpath-fallback` passes unwaited | Guard green while that fixture still has no wait | AC-4 pinned case: the false branch is a red case in the false-pass test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong wait or drain can race shutdown or time out a fixture. Runner ownership and guest preparation can prevent tests from executing. The startup declaration correction also affects live filter-reference validation while a plugin is completing Stage 1. |
| How is it reverted? | A reviewed follow-up `./le commit` can reverse a family or harness repair. The destructive Git commands banned by META are not an implementation step. |
| Who else touches this path? | Every session adding a plugin fixture; the guard refuses a new unwaited observer once this lands |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go test` of `internal/test/fixture` | → | `observerPeerWaitFindings` over every family | `TestPluginObserversWaitForTheScriptedPeer` |
| `./le test bgp plugin bgp-rs-reactor-fastpath` | → | `rsObserver04` count wait | `test/plugin/bgp-rs-reactor-fastpath.ci` |
| `./le test bgp plugin prefixsid-malformed-receive` | → | `routeServerReplay13` count wait | `test/plugin/prefixsid-malformed-receive.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `TestPluginObserversWaitForTheScriptedPeer` run over the package | Passes with zero findings over every family |
| AC-2 | Every `.ci` whose observer changed | Passes 3 runs in a row; the 17 `needs-linux` ones pass 3 in a row inside a QEMU guest booting Ze's runtime kernel |
| AC-3 | One fixture of each family 05, 11, 14 and other, and one inline scenario, with its new wait removed | The guard fails and names that fixture and peer |
| AC-4 | A gated-count case: a driver entry gated on an argument, called with `false` | The false-pass test reports a finding; its control, called with `true`, reports none |
| AC-5 | `./le doc wiring` sleep count after the change | Not above its count before; each fixed sleep a wait replaced is gone |
| AC-6 | `git diff` over `test/**/*.ci` | No `.ci` expectation removed or weakened; any `.ci` edit carries a `ze-test-judge` approval |
| AC-7 | The 08 and 12 guards | Still pass, unchanged |

### Current acceptance audit

| Criterion | Status | Evidence |
|-----------|--------|-----------------------------|
| AC-1 | PASS | `TestPluginObserversWaitForTheScriptedPeer` reports zero findings across all families. |
| AC-2 | PASS | The exact expanded population of 191 native and 17 Linux CI files passed three consecutive runs each: 624 successful invocations, zero skips. The Linux union is exactly the retained 17-file population. |
| AC-3 | PASS | Cutting waits from families 05, 11, 14, other and an inline scenario produced eight named fixture/peer findings. Exact restoration returned the guard to zero. |
| AC-4 | PASS | Removing the replay gate made the disabled-count case fail while its enabled control passed. The restored `TestPluginObserverPeerWaitGuardRefusesFalsePasses` passed. |
| AC-5 | PASS | `./le doc wiring` reported `ci-sleep ratchet OK (0 <= ceiling 0)`. Its broader shared-tree documentation failures are recorded separately above. |
| AC-6 | PASS | The policy-final CI repair received independent approval of the paired producer and expectations before `4007003754`. Its original policy checks remain, and replay recovery adds checks. Other wait changes retain the CI assertions. |
| AC-7 | PASS | `TestPluginObservers08WaitForTheScriptedPeer` and `TestPluginObservers12WaitForTheScriptedPeer` pass; this implementation does not alter those guards. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginObserversWaitForTheScriptedPeer` | `internal/test/fixture/plugin_fixture_peer_wait_test.go` | AC-1, AC-3: zero findings, selected wait cuts fail | PASS, including restored-source run |
| `TestPluginObserverPeerWaitGuardRefusesFalsePasses`, new gated-count case | `internal/test/fixture/plugin_fixture_peer_wait_guard_test.go` | AC-4: a gated count counts only when its gate argument is the literal `true` | PASS, with gate-removal discrimination |
| `TestPluginObservers08WaitForTheScriptedPeer`, `TestPluginObservers12WaitForTheScriptedPeer` | existing | AC-7 | PASS, unchanged |
| `TestObserveConfiguredRequiresCompletedQuiesce`, `TestP05ActiveObserversRequireCompletedQuiesce` | `internal/test/fixture/fixture_quiesce_test.go` | Completed drain control, transport loss, RPC refusal and warning through real observer consumers | PASS |
| `TestRunPlugin13RequiresCompletedQuiesceBeforeShutdown`, `TestPlugin15StartedReportsBarrierFailureBeforeShutdown` | `internal/test/fixture/plugin_fixture_custom_quiesce_test.go` | The owner checks its drain and publishes failure before shutdown | PASS |
| `TestAwaitObserverResultDoesNotLoseALateScenarioFailure`, `TestAwaitObserverResultAcceptsTransportStopAfterScenarioSuccess`, `TestAwaitObserverResultReturnsTransportErrorBeforeScenarioStarts` | `internal/test/fixture/fixture_test.go` | Started error, completed success and pre-start transport failure retain their verdicts | PASS |
| `TestPeerStartupUsesCaseBudget`, `TestPeerStartupReportsExit`, `TestPeerStartupHonorsDeadline`, `TestPeerStartupHonorsCancellation` | `internal/test/runner/peer_startup_test.go` | Both readiness paths obey the case budget, attribute early exit and retain context causes | RED then GREEN in all eight subcases. Full runner package PASS with race. |
| `TestQEMUAllSuiteDeadlineUsesNativeOwnership`, `TestQEMUAllSuiteDeadlineRefusesUnboundedBudgets` | `internal/le/test/qemu/alltests_timeout_linux_test.go` | Bounded native suite ownership, process-group termination, exit status and tally | Consumer RED then GREEN. Scoped QEMU package PASS with race. |
| `TestQEMUAllPreparesConntrackBeforeSuites`, `TestQEMUAllRefusesConntrackPreparationFailure` | `internal/le/test/qemu/alltests_conntrack_test.go` | Conntrack setup precedes every suite, and a setup failure prevents suite execution on both guest entry paths | RED then GREEN in all four subcases. Scoped QEMU package PASS with race. Actual accounting, hook readback, idempotence and preserved foreign table observed; confidence CI 3/3 PASS. |
| `TestDeclaredFiltersWaitsForStageOne` | `internal/component/plugin/server/server_declared_filters_test.go` | Incomplete registration is not an authoritative empty filter declaration | RED then GREEN; before-registration, completed-empty and completed-filter cases PASS, with aliased-reference controls. Repair committed in `f4dca4ecab`. |
| `TestDockerLabCommandRunsInZeSystem` | `internal/le/test/qemu/dockerlab_command_integration_test.go` | The carrier boots Ze's kernel and system, reaches the command, and writes back only its private scratch | PASS through the actual own-system carrier; fixture execution is recorded separately. |
| `TestFlowSpecPeerLookupUsesCaseProcess` | `internal/test/fixture/plugin_fixture_07_peer_lookup_linux_test.go` | Canonical peer lookup selects only this case's executable and working directory, excluding four live controls | Consumer RED then GREEN; scoped fixture run also passes all-family, 08/12, false-pass, TLS drain and result-ordering controls with race enabled. The unchanged live withdrawal CI passed 3/3. |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| count wait vs scripted peers | count >= scripted | count == scripted | count == scripted - 1 (finding) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Native task population, retained guard baseline and current reached aliases | `test/plugin/`, `test/reload/`, `test/ui/` | Every current affected native CI passes 3/3 | All 191 native files passed three consecutive runs. Exact manifest union verified. |
| The 17 `needs-linux` fixtures | The Phase 5 one-CI own-system route | Each passes 3/3 without skips on Ze's system | All 17 passed three consecutive runs each: 51 successful invocations with zero skips, checked Ze kernel/system identity and exact population coverage. |
| `policy-final-disposition` | `test/plugin/policy-final-disposition.ci` and its draft counterpart | Policy rejection is checked before same-session Route Refresh recovery | Draft 3/3 and promoted live 3/3 PASS. Paired repair committed in `4007003754`. |

## Files to Modify
- `internal/test/fixture/plugin_fixture_peer_wait_guard_test.go` - the gated-count case and its control (AC-4)
- `internal/test/fixture/plugin_fixture_peer_wait_test.go` - a family entry for any existing helper a family starts calling
- The files defining the drivers the findings name (findings per file; a wait in a shared driver may serve several fixtures):

| File | Findings | Drivers |
|------|----------|---------|
| `internal/test/fixture/plugin_fixture_01.go`, `_01_adj_rib.go`, `_01_api.go`, `_01_save.go` | 11 | `plugin01AddRemove`, `plugin01AdjRIBInQuery`, `plugin01ReplayAddPath`, `plugin01ReplayOnPeerUp`, `plugin01ReplayUnownedPeer`, `plugin01APIPeerPrefixUpdate`, `plugin01APIPeerSave` |
| `internal/test/fixture/plugin_fixture_02.go`, `_02_events.go` | 9 | `observer02`, `pathASNExportDriver02`, `eventObserver02` (three findings are family "other" registrations of these drivers) |
| `internal/test/fixture/plugin_fixture_03.go` | 5 | `observe03`, `observePort03` |
| `internal/test/fixture/plugin_fixture_04.go`, `_04_route_server.go` | 6 | `observe04`, `rsObserver04` (`bgp-rs-reactor-fastpath-fallback`) |
| `internal/test/fixture/plugin_fixture_05.go`, `_05_ddos.go` | 15 | the `p05*` scenarios |
| `internal/test/fixture/plugin_fixture_06.go`, `_06_decoder.go` | 9 | the `fixture06*` scenarios and one inline scenario |
| `internal/test/fixture/plugin_fixture_07_flowspec.go` | 2 | `flowSpecLegacyTable07`, `flowSpecWithdrawTable07` |
| `internal/test/fixture/plugin_fixture_09.go`, `_09_bgp.go`, `_09_l2tp.go` | 16 | `llgrReadvertise09`, `localPrefStrip09`, `medRawDrop09` and the rest |
| `internal/test/fixture/plugin_fixture_10.go` | 10 | the `fixture10*` scenarios |
| `internal/test/fixture/plugin_fixture_11.go` | 3 | `prefixCountBounded`, `prefixCountExact` |
| `internal/test/fixture/plugin_fixture_13.go`, `_13_peer_lifecycle.go`, `_13_rest.go` | 12 | `forwardedPrivateAS13`, `opaqueWithdraw13`, `originatedPrivateAS13`, `reloadListenerRejected13`, `reloadSharedSecret13`, `peerCreateDeleteRIB13`, `restAPICommands13`, `restExecute13` |
| `internal/test/fixture/plugin_fixture_14.go` | 35 | `plugin14Observer`, `plugin14EventObserver`, `plugin14ASPAStateObserver`, two inline scenarios |
| `internal/test/fixture/plugin_fixture_15.go`, `_15_standalone.go` | 16 | `plugin15Observe`, `plugin15ObserveStarted`, `plugin15SummaryFormat` |
| `internal/test/fixture/plugin_fixture_16.go`, `_16_l2tp.go` | 13 | the `plugin16*` scenarios, one inline scenario |
| `internal/test/fixture/misc_fixture_observers.go` | 5 | `reloadRESTObserver`, `observeDriver` |
| `internal/test/fixture/plugin_fixture_gr_sender_facts.go`, `plugin_fixture_llgr_import.go`, `plugin_fixture_llgr_lifecycle.go` | 5 | `grSenderFactsDriver`, `llgrImportNoLLGR`, `llgrLifecycle` |
| `internal/test/fixture/policy_final.go`, `rfc8212_policy.go`, `register_rib_clear_advertised.go` | 7 | `policyFinalDriver`, `rfc8212PolicyDriver`, `ribClearAdvertised` |
| `internal/test/fixture/bgpls_export.go`, `register_update_text_prefix_sid.go`, `ui_fixture_web_commit_reject.go` | 3 | `bgplsExportLifecycle`, `updateTextPrefixSID`, `runWebCommitReject` |

## Files to Create

No new per-family peer-wait helper is needed: families without one call
`waitPeerEORSent` or `p12WaitPeersEOR`. Runtime proof exposed additional
infrastructure defects, whose repairs added these files:

| Area | Created files | Purpose |
|------|---------------|---------|
| Completed observer drain | `internal/test/fixture/fixture_quiesce_test.go`, `internal/test/fixture/plugin_fixture_custom_quiesce_test.go` | Actual TLS consumer regressions for the shared observer, active p05 scenarios and custom shutdown owners |
| Stage 1 declarations | `internal/component/plugin/server/server_declared_filters_test.go` | Unfinished registration differs from an authoritative completed empty declaration |
| Peer readiness | `internal/test/runner/peer_startup.go`, `internal/test/runner/peer_startup_test.go` | Existing case deadline, early exit attribution and cancellation in both startup paths |
| Native suite deadline | `internal/le/test/qemu/alltests_deadline.go`, `internal/le/test/qemu/alltests_timeout_linux_test.go` | Sole-reaper deadline consumer and process-group lifecycle regressions |
| Conntrack preparation | `internal/le/test/qemu/alltests_conntrack.go`, `internal/le/test/qemu/alltests_conntrack_test.go` | Checked native guest preparation before either suite entry path |
| FlowSpec peer ownership | `internal/test/fixture/plugin_fixture_07_peer_lookup_linux_test.go` | Actual process lookup excludes legacy argv, another command, a sibling case and a different executable |
| Own-system carrier | `internal/le/test/qemu/dockerlab_command.go`, `internal/le/test/qemu/dockerlab_command_test.go`, `internal/le/test/qemu/dockerlab_command_integration_test.go`, `internal/le/test/qemu/dockerlab_identity_test.go`, `internal/le/test/qemu/alltests_one_netns_test.go` | Command execution in Ze's system and canonical one-CI namespace selection |
| Guest tools and workspace | `internal/appliance/dockerhost/guest_tools_linux_test.go`, `internal/le/setup/docker/host/extrafiles_workspace_test.go` | Image tool installation and the shared main-checkout mountpoint |
| Unchanged documented behavior | `plan/doc-reviewed/e23fbcef.md` | Records why `ServerProcess.Stop` retains the terminal-demo cleanup claim after delegation to its shared helper |

The corresponding existing producers changed in
`internal/component/plugin/server/server.go`,
`internal/test/runner/runner_exec.go`,
`internal/test/runner/runner_exec_util.go`,
`internal/test/runner/server_process.go`, `internal/le/test/qemu/alltests.go`,
the own-system image builder and command carrier. These are validation
blocker repairs, not new peer-wait implementations.

Additional named-wait consumers include
`internal/test/fixture/plugin_fixture_llgr_received.go`
(`llgrStaleReceived02`),
`internal/test/fixture/plugin_claim_permission.go`
(`observeReplayClaimPermissions`),
`internal/test/fixture/clear_caches_fixture.go` (`clearIRRCache04`) and
`internal/test/fixture/role_learned_cleared_fixture.go`
(`roleLearnedClearedOnPeerDelete`). Their reached aliases are included in
the 191-file native proof. The Linux withdrawal diagnosis also reaches
`internal/test/fixture/plugin_fixture_07_flowspec.go` (`peerPIDs07`), whose
canonical command and case/executable checks pass the new regression and the
unchanged live withdrawal/table-removal fixture's three consecutive runs.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test fixtures only |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | Existing `test qemu docker-lab` gains its own-system command branch; selected `test qemu all` retains its path syntax and now checks guest preparation |
| CLI grammar (keyword before value) | Yes | The command carrier uses the existing keyword grammar: `kernel <path> command <text> timeout <duration>`; selected CI remains `all test <path>` |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A for shipped code | The new dependencies belong to the isolated test guest. Native preparation checks actual UID, PID 1, Ze kernel, accounting and nftables readback before a suite; the guest image supplies the required tools. |
| Prometheus counters/metrics | N-A | Reads existing peer counters only |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Test harness only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/architecture/testing/qemu-integration.md`: own-system command branch, selected CI and root/preparation requirements |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/ci-format.md`: named waits, checked drain and failure ordering; `docs/architecture/testing/runner-architecture.md`: readiness and process ownership; `docs/architecture/testing/qemu-integration.md`: own-system setup and native deadlines; `docs/architecture/appliance/docker-host.md`: guest tools/workspace |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | Existing runner process ownership is reused by suite deadlines and guest preparation; the runner and QEMU architecture pages describe these consumers |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Updated the observer claim in `docs/architecture/iface/netlink-monitor.md`; `plan/doc-reviewed/e23fbcef.md` retains the unchanged terminal-demo `ServerProcess.Stop` claim. `./le spec citation anchors spec plan/release/validation/spec-observer-peer-wait-backlog.md` exited zero after the peer lookup repair. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Replaced the QEMU page's Alpine/manual-shim selected-CI recipe with the prepared Ze-system `qemu all test` route |

## Implementation Steps

1. **Phase: Wiring** -- run the all-family guard and save its findings as the starting population.
   - Tests: `TestPluginObserversWaitForTheScriptedPeer` (red, 182)
   - Files: none
   - Verify: the finding count and per-family split match the Task table, or the table is corrected first
2. **Phase: Gated-count case** -- add the AC-4 case and control to `TestPluginObserverPeerWaitGuardRefusesFalsePasses`.
   - Tests: the new case red with the gate removed from the guard's reading, green with it
   - Files: `plugin_fixture_peer_wait_guard_test.go`
3. **Phase: Families with a helper** (01, 02, 03, 04, 06, 07, 09, 10, 13, 15, 16, misc) -- one commit per family: each scenario waits for each scripted peer by name before it ends, and on the post-emit counter where it emits (R-2). Run that family's changed `.ci` 3/3.
4. **Phase: Families without a helper** (05, 11, 14's driver gaps, other, the inline scenarios) -- the same, calling `waitPeerEORSent` or `p12WaitPeersEOR` (or the shared driver's own wait where every scenario of the driver needs it at the same point), adding the helper to the family's entry. Remove each fixed sleep the wait replaces.
5. **Phase: needs-linux proof** -- run the 17 `needs-linux` fixtures 3/3 through `./le test qemu docker-lab kernel <runtime-tree>/vmlinuz command <guest-script>`. Boot Ze's own image and matching runtime kernel. The guest script binds the shared main checkout at `/workspace`, then runs a fresh host-built guest `le test qemu all test <test-path>` as root. Carry the guest's LABEL-only `ZE_RUN_JOB` stand-in, and keep the host session identity on the host. Confirm each selected fixture ran without a skip. The own-system command branch permits TCG for this correctness proof.
6. **Phase: Docs** -- `docs/functional-tests.md` row 10 of the checklist.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every finding in the phase 1 population is gone, each by a wait at file:line |
| Correctness | Each wait names the peer the `.ci` scripts (name or the address the guard resolves), not `*` where several peers are scripted |
| Correctness | No wait placed before an action the scenario must take while the peer is down (R-1) |
| Rule: justify-every-sleep | No `time.Sleep` added; each removed sleep's condition is now a named wait |
| Rule: test changes | No `.ci` expectation changed without a judge |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Guard green over every family | `./le job run label unit-pkg command go test -tags <the tags ./le test unit uses> ./internal/test/fixture/ -run 'TestPluginObserver'` |
| Changed fixtures 3/3 | `./le test bgp plugin <name>` three times per changed fixture (`reload`, `ui` suites likewise), output kept |
| needs-linux fixtures 3/3 | The own-system docker-lab carrier in Phase 5, with one-CI `le test qemu all test <test-path>` runs and no skips |
| Change set judged | Scoped fixture-package tests and the changed fixtures' consecutive 3/3 runs in the main checkout; no `./le verify` command |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: test code reading its own fixtures |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A fixture stays red after its wait with ze at fault | Diagnose and fix the owning producer, then rerun the affected proof. A journal entry does not discharge a blocker (`ai/rules/completion.md`). |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `false-synchronization-claim.md` | 2026-11-02 | observer waits missing across the fixtures, all-family peer-wait guard |

No observer or sleep row was added to `plan/journal/` by the rules pass of
2026-11-02 (d1c57af45c, a4d48332fe): that pass changed the sleep rule point to
name the guards and added no journal row for it. The row above records this
backlog; its text carries the first count (281 across 240), and the current
population is the Task table.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A sent End-of-RIB counted per distinct peer is a wait; the guard reads `routeServerObserver02`, `rsObserver04` and `routeServerReplay13` as count waits (done in 7e68ebce69) | Make each also call a named helper | The event establishes a live session and a marker in its send path. `Observe` then quiesces buffered output before shutdown (A-2). Initial-sync `eor-sent` counters instead advance after flush. |
| `rsObserver04` counts only when `requireReplay` is the literal `true` (done in 7e68ebce69) | Count it unconditionally | Its false branch sets the End-of-RIB need to zero and waits on no peer |
| Families without a helper call `waitPeerEORSent` or `p12WaitPeersEOR` | A new helper per family, as 08 and 12 got | A per-family copy is the open-coded-copy class |
| Waits sit where the scenario's assertions need them, in the scenario or in a shared driver | Always in the driver | Some scenarios act before their peer is up (R-1) |

## Known Limitations
- The guard proves a wait exists, not that it is placed after the scenario's last emit; R-2 is a review check.
- The guard's reading is static: a wait in a branch that never runs counts (its own LIMIT comment).

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the fixture package tests, each changed `.ci` through its registered suite 3/3, and the `needs-linux` ones through `./le test qemu all` in QEMU on Ze's runtime kernel; no `./le verify` command
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
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
- [ ] **Commit B:** `remove plan/release/validation/spec-observer-peer-wait-backlog.md` only, in the same `./le commit create` script (commit A preserves the spec in history)


## Implementation Summary

### What Was Implemented

Observers wait for the named session state or sent-payload marker their CI
requires, then complete a checked final drain before requesting shutdown.
Existing family helpers supply the waits. The disabled replay-count control
pins the guard's literal-true gate. All 208 reached CI files passed three
consecutive runs each: 191 native and 17 on Ze's system, 624 successful
invocations and zero skips.

### Bugs Found/Fixed

| Defect | Producing source | Regression |
|--------|------------------|------------|
| Allocated registration treated as a completed empty declaration | `internal/component/plugin/server/server.go` `Server.declaredFilters` | `TestDeclaredFiltersWaitsForStageOne` |
| Peer readiness had a separate five-second cap and lost exit/cancellation causes | `internal/test/runner/peer_startup.go` `awaitPeerListening` | Four `TestPeerStartup*` controls, both consumers |
| Minimal guest timeout rejected the suite before execution | `internal/le/test/qemu/alltests_deadline.go` `allTestsRun.suiteChild` | `TestQEMUAllSuiteDeadlineUsesNativeOwnership`, invalid-budget controls |
| Guest omitted conntrack accounting and tracking hook | `internal/le/test/qemu/alltests_conntrack.go` `allTestsRun.prepareConntrack` | Both entry paths' setup/refusal tests and unchanged confidence CI |
| FlowSpec lookup selected the legacy command instead of its owned peer | `internal/test/fixture/plugin_fixture_07_flowspec.go` `peerPIDs07` | Five-live-child consumer test and unchanged withdrawal CI |
| Lost or incomplete drain could report success | `internal/test/fixture/fixture.go` `observeConfigured`; custom owners `rsObserver04`, `medRawDrop09`, `accumulatorIsolation09`, `runPlugin13` | Real TLS completion, transport-loss, refusal and warning controls |
| Policy recovery crossed epochs before checking rejected imports | `internal/test/fixture/policy_final.go` `policyFinalCase`, `policyFinalRestore` | Independently approved paired CI; draft and live 3/3 |

### Documentation Updates

| Page | Committed update and source |
|------|-----------------------------|
| `docs/functional-tests.md` | Count gate (`1e410324b2`), policy restoration (`4007003754`), owned FlowSpec lookup (`d52985d520`), final drains (`c994dace87`); anchors name the guard, fixture producers and tests |
| `docs/architecture/testing/ci-format.md` | `c994dace87`: checked quiesce and failure-before-shutdown; `observeConfigured`, `awaitObserverResult`, custom owners |
| `docs/architecture/testing/runner-architecture.md` | `12afa215f6`, `e0e95030e8`: sole reaper, leader/group stop and case-budget readiness |
| `docs/architecture/testing/qemu-integration.md` | `934e7d8c03`, `12afa215f6`, `3265a72946`: actual own-system carrier, selected namespace, native deadline and conntrack setup |
| `docs/architecture/appliance/docker-host.md` | `1dd25fba1d`: static guest tools, executable aliases and read-only-root workspace mountpoint |
| `docs/architecture/iface/netlink-monitor.md` | `c994dace87`: Observe requires completed quiesce |
| `plan/doc-reviewed/e23fbcef.md` | `3265a72946`, `f799f67abd`: unchanged Stop and VPN claims validated against their producers |

The implementation's `./le doc wiring` run included `./le doc check all`.
The sleep ratchet passed at zero and every path reference resolved. The overall
command remained red for shared unpushed documentation drift and three foreign
inventory anchors. Scoped reviewed-claim validation exited zero. Closure
changes no documentation page, so it reuses these observed checks.

### Deviations from Plan

The main checkout already held most waits in `73978cd75e`; they were audited
and proved rather than written twice. Runtime proof required the seven
producer repairs above, the own-system command carrier and static guest tools.
Those changes preserve the full validation population. The source audit
expanded the live population to 208 CI files; it did not discard the retained
historical baseline. The original sent-event flush assumption was wrong, so
final checked drains complete the post-emit delivery contract. A-4 was broken:
some historical findings were unrecognized existing polls, now expressed with
the canonical helper. The policy expectation repair received an independent
judge's approval before it changed. This completed incremental implementation
entered closure already committed; commit A preserves its final audit and
marks the existing owned journal row fixed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Original A-2 credited sent events as completed flushes | `Session.writeUpdateBody` can report before flush | Read the send and quiesce producers | Require a checked final drain after the last payload gate |
| assumption | A-4 assumed every historical finding lacked a runtime wait | Three p05 scenarios already polled peer1 through an unrecognized shape | Read `73978cd75e` and the previous poll bodies | Preserve those waits through canonical helpers; report the false positives |
| approach | Guest startup and suite wrappers assumed distribution tools/setup | Ze's minimal system rejected external timeout and lacked the host tracking preparation | Actual consumer REDs and own-system failures | Native deadline owner, checked conntrack setup and owned peer lookup |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every affected observer waits for each scripted peer | Done | Family scenario functions in the committed path audit below; `p12WaitPeersEOR`, `plugin01WaitCounter` and existing helpers | Placement preserves pre-establishment actions and follows the last required emit |
| All-family guard zero | Done | `plugin_fixture_peer_wait_test.go` `TestPluginObserversWaitForTheScriptedPeer` | Actual PASS; AC-3 wait cuts produce eight named findings |
| Changed fixtures each pass three consecutive runs | Done | Registered plugin/reload/UI suites; `allTestsRun.executeOne` for Linux | Exact 191 native + 17 Linux union, 624 successful invocations, zero skips |
| Preserve wire assertions and 08/12 protection | Done | Existing 08/12 tests; `policy_final.go` and paired CI | Only owned CI change independently approved; all eight original policy epochs retained |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | All-family guard PASS with zero findings | Final race-enabled fixture check |
| AC-2 | Done | 208 exact CI hashes bound to three consecutive passes per file | 191 native; all 17 Linux on 7.3.0-ze with gokrazy PID 1 |
| AC-3 | Done | Five family/inline wait cuts produced eight fixture/peer findings; exact restore PASS | Real all-family consumer |
| AC-4 | Done | Disabled gate red, enabled control green; restored false-pass suite PASS | `observerWalk.readWait` and `scriptedPeer.waitedOnBy` |
| AC-5 | Done | `ci-sleep ratchet OK (0 <= ceiling 0)` | Wider foreign doc failures are disclosed |
| AC-6 | Done | `4007003754` carries independent paired-policy approval | Other owned CI expectations unchanged |
| AC-7 | Done | Unchanged family 08 and 12 guards PASS | No edits to either family guard |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All-family, false-pass and family 08/12 guards | Done | `internal/test/fixture/plugin_fixture*_peer_wait*_test.go` | Zero current findings; AC-3/4 observed RED controls |
| Observe, active p05 and custom shutdown TLS cases | Done | `fixture_quiesce_test.go`, `plugin_fixture_custom_quiesce_test.go` | Completed positive control and three rejected-drain cases |
| Three await-result controls | Done | `internal/test/fixture/fixture_test.go` | Late scenario error, success after stop and pre-start transport failure |
| Four peer startup tests, both paths | Done | `internal/test/runner/peer_startup_test.go` | All eight subcases RED then GREEN; full runner race PASS |
| Deadline status, group cleanup and invalid budgets | Done | `internal/le/test/qemu/alltests_timeout_linux_test.go` | Consumer RED then GREEN; scoped QEMU race PASS |
| Conntrack setup and refusal, both entry paths | Done | `internal/le/test/qemu/alltests_conntrack_test.go` | Four subcases RED then GREEN; actual guest strict readback/idempotence |
| Stage 1 allocated/empty/nonempty declarations | Done | `server_declared_filters_test.go` | RED then GREEN; aliases retain their assertions |
| Actual own-system command | Done | `dockerlab_command_integration_test.go` | Registered action reaches Ze guest and writes exact private bytes |
| Actual process lookup with four live exclusions | Done | `plugin_fixture_07_peer_lookup_linux_test.go` | RED then GREEN; final withdrawal CI 3/3 |
| Every native and Linux functional test planned | Done | 208-file bound population | All three-consecutive runs completed, zero skips |
| Numeric count boundary | Done | `TestPluginObserverPeerWaitGuardRefusesFalsePasses` | Count equal/short, replay false/true controls |
| Protocol interoperability | Done | No protocol behavior added or changed | This is harness synchronization; existing wire expectations retained |

All 22 exact test names in the TDD table were checked against their current
declarations and found. No named test is missing.

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `docs/architecture/appliance/docker-host.md` | Done | Present; exact owned commit reviewed |
| `docs/architecture/iface/netlink-monitor.md` | Done | Present; exact owned commit reviewed |
| `docs/architecture/testing/ci-format.md` | Done | Present; exact owned commit reviewed |
| `docs/architecture/testing/qemu-integration.md` | Done | Present; exact owned commit reviewed |
| `docs/architecture/testing/runner-architecture.md` | Done | Present; exact owned commit reviewed |
| `docs/functional-tests.md` | Done | Present; exact owned commit reviewed |
| `gokrazy/docker-host/binaries.json` | Done | Present; exact owned commit reviewed |
| `internal/appliance/dockerhost/guest_linux.go` | Done | Present; exact owned commit reviewed |
| `internal/appliance/dockerhost/guest_tools_linux_test.go` | Done | Present; exact owned commit reviewed |
| `internal/component/plugin/server/filter_reference_alias_test.go` | Done | Present; exact owned commit reviewed |
| `internal/component/plugin/server/server.go` | Done | Present; exact owned commit reviewed |
| `internal/component/plugin/server/server_declared_filters_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/setup/docker/host/build_action_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/setup/docker/host/extrafiles.go` | Done | Present; exact owned commit reviewed |
| `internal/le/setup/docker/host/extrafiles_workspace_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/setup/docker/host/static.go` | Done | Present; exact owned commit reviewed |
| `internal/le/setup/docker/host/static_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/actions.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_conntrack.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_conntrack_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_deadline.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_one_netns_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_one_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_tally.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/alltests_timeout_linux_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/dockerlab.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/dockerlab_command.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/dockerlab_command_integration_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/dockerlab_command_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/dockerlab_identity_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/dockerlab_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/netns.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/netns_alltests_test.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/run_exec.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/run_guestcommand.go` | Done | Present; exact owned commit reviewed |
| `internal/le/test/qemu/run_report.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/bgpls_export.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/clear_caches_fixture.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/fixture.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/fixture_quiesce_test.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/misc_fixture_observers.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_claim_permission.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_01.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_01_adj_rib.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_01_api.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_01_save.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_02.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_03_bfd.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_03_fastpath_replay.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_03_mod_copy.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_03_route_server.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_04_cli.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_04_route_server.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_05.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_05_ddos.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_06.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_06_decoder.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_06_events.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_07_flowspec.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_07_peer_lookup_linux_test.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_09.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_09_bgp.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_09_l2tp.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_10.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_11.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_12_filters.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_13.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_13_peer_lifecycle.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_13_rest.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_14.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_15.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_15_rpki.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_15_show.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_15_standalone.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_16.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_16_l2tp.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_custom_quiesce_test.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_gr_sender_facts.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_llgr_import.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_llgr_lifecycle.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_llgr_received.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/plugin_fixture_peer_wait_guard_test.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/policy_final.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/register_rib_clear_advertised.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/register_update_text_prefix_sid.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/rfc8212_policy.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/role_learned_cleared_fixture.go` | Done | Present; exact owned commit reviewed |
| `internal/test/fixture/ui_fixture_web_commit_reject.go` | Done | Present; exact owned commit reviewed |
| `internal/test/runner/peer_startup.go` | Done | Present; exact owned commit reviewed |
| `internal/test/runner/peer_startup_test.go` | Done | Present; exact owned commit reviewed |
| `internal/test/runner/runner_exec.go` | Done | Present; exact owned commit reviewed |
| `internal/test/runner/runner_exec_util.go` | Done | Present; exact owned commit reviewed |
| `internal/test/runner/server_process.go` | Done | Present; exact owned commit reviewed |
| `plan/doc-reviewed/e23fbcef.md` | Done | Present; exact owned commit reviewed |
| `plan/release/validation/spec-observer-peer-wait-backlog.md` | Done | Present; exact owned commit reviewed |
| `test/plugin/policy-final-disposition.ci` | Done | Present; exact owned commit reviewed |
| Existing family drivers not edited by this work | Done | `plugin_fixture_peer_wait_test.go`, `plugin_fixture_02_events.go`, `plugin_fixture_03.go` and `plugin_fixture_04.go` retain their existing guard/helper paths; no duplicate edit needed |

### Audit Summary

Four task obligations, seven ACs, all 22 named tests and all planned file
groups are satisfied. No Partial or Skipped item remains. Changes from the
initial approach are explained under Deviations from Plan. The path audit
covers 98 committed files, including 18 created files; every created file exists.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Each observer waits for each scripted peer's required state or output | Functional and discrimination | Named waits and checked final drains in the actual observer consumers; 208 affected CI files pass 3/3 with unchanged wire assertions; five wait cuts make the guard name eight missing fixture/peer pairs |
| Guard over all families reports zero | Unit and negative control | `TestPluginObserversWaitForTheScriptedPeer` PASS; disabled replay gate red, enabled control green; exact restoration PASS; unchanged 08/12 controls PASS |
| Every changed fixture passes three consecutive runs | Functional | Exact hash-bound population of 191 native and 17 own-system Linux files, 624 successful invocations, zero skips; every Linux invocation reports one test and checked Ze-system identity |

## Work Not Done

None. All agreed implementation and runtime proof is complete. No in-scope
item needs another spec. Foreign working changes and broad foreign gate
findings are outside this closure's agreed change set.

## Review Gate

Round 1 scope: the complete owned implementation in commits `73978cd75e`,
`1e410324b2`, `f4dca4ecab`, `1dd25fba1d`, `19d3857b12`, `f4fb05eebd`,
`82aa86e9a7`, `934e7d8c03`, `4007003754`, `12afa215f6`, `880f14f641`,
`e0e95030e8`, `3265a72946`, `d52985d520`, `c994dace87` and `f799f67abd`.
The independent closure owner read each immutable patch and the reached
producers inline. This is one pass with three lenses, including the Go style
pass over every changed Go file. No new product fix was needed.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/observer-peer-wait-backlog-b4d8e48a-cb87-4d81-8524-568c8d63b384.md`; 309 immutable committed blobs and modes |
| `./le spec review check` | Exit 0: 295 unchanged checkout code files, clean hashes. Foreign working edits excluded. |
| Full committed-tree check | `./le commit check` exit 0, clean=true over all 300 reviewable code files at tree `c722a1976df62f5a0b7d84bd57923b72be035c6c` |
| Rounds | 1 |
| Reviewer lenses used | Logic and wiring; security and edge cases; guard and test integrity; explicit Go style pass |
| Final verdict | CLEAN: 0 BLOCKER, 0 ISSUE |

### Findings fixed

No BLOCKER or ISSUE was found in the owned change set. Record-only NOTES were
resolved in this audit: incremental committed entry, corrected assumptions,
historical/current population distinction and the owned journal's fixed state.
They do not require another review round.

### Automated pre-checks and limits

`./le repo check` exited 1 with nine unrelated exported-symbol findings.
`./le test weakened check base origin/main` exited 1 with ten unrelated units
under schema, coupling, site and OSPF tests. None is an owned review path.
They are not a passing global gate. The five count-adjustment approvals in
`3265a72946` are parsed Git trailers. The two approval lines in `934e7d8c03`
lack a trailer separator; they are approval prose, not parsed trailers.
Prior independent approval and subsequent exact test-count judgments remain
recorded; closure does not invent an approval.

`./le rfc check` exited 1 with 131 shared-tree violations, including 26 stale
and six shifted audit entries. None names an owned path. This implementation
adds no RFC requirement tag, changes no protocol behavior and owns no RFC
summary or discrimination record; no such record was rewritten at closure.

The security review traced the explicit local command boundary, checked kernel
identity and image command-line bounds, resource lifetimes, immutable input
pins and filename quoting. The sole product change retains completed-empty
Stage 1 refusal and changes only unfinished declarations. No protocol or SDK
wire contract changes. No peer-reachable panic was introduced. The new loops
and workers are bounded by case/command deadlines or owned process completion.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Every file in Files to Create | Yes | All 18 added paths from the 16 immutable implementation commits resolve as regular files |
| All reached functional CI files | Yes | All 208 paths read for fresh SHA256 comparison; zero drift |
| Wiring carriers | Yes | `test/plugin/bgp-rs-reactor-fastpath.ci` and `test/plugin/prefixsid-malformed-receive.ci` read in full |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Guard actually ran and passed | Final log names `TestPluginObserversWaitForTheScriptedPeer` PASS; `observerPeerWaitFindings` traverses all registrations |
| AC-2 | Full exact population completed | Fresh comparison of all 208 current CI SHA256 values with completed proof: zero mismatches; 191 native + 17 Linux, each three consecutive |
| AC-3 | Removing waits is rejected | Read actual eight named findings in `TestPluginObserversWaitForTheScriptedPeer` RED; restored run PASS |
| AC-4 | Replay false is not credited | `observerWalk.readWait` requires literal true at the gate; observed disabled/control RED/GREEN proof |
| AC-5 | Sleep count does not increase | Read final doc-wiring result: `ci-sleep ratchet OK (0 <= ceiling 0)` |
| AC-6 | Original expectations retained or independently judged | Read paired `4007003754` patch and approval; only owned CI change; five exact count trailers parsed in `3265a72946` |
| AC-7 | Existing 08/12 guards preserved | Final log names both PASS; no owned patch edits either guard |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Fixture package guard | All reached plugin/reload/UI carriers | `observerPeerWaitFindings` resolves registrations and scripted peers; negative cuts exercise that entry |
| `le test bgp plugin bgp-rs-reactor-fastpath` | `test/plugin/bgp-rs-reactor-fastpath.ci` | Actual external observer command, two scripted connections and forward expectation; `rsObserver04` counts distinct peers and drains last |
| `le test bgp plugin prefixsid-malformed-receive` | `test/plugin/prefixsid-malformed-receive.ci` | Actual external observer command, eight route/withdraw assertions and two peer connections; `routeServerReplay13` then owner drain |
| Own-system one-CI carrier | All 17 Linux selections | Registered command boot verifies matching kernel/PID1/root; `executeOne` parses real namespace metadata, prepares conntrack and reports one executed CI |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Existing guard and family 08/12 declarations present; final race log names all PASS |
| A-2 | confirmed after correction | Original flush assumption was broken; `Session.writeUpdateBody` reports pre-flush, `Peer.sendInitialRoutes` increments initial EOR after send/flush, and `observeConfigured` requires the final done barrier |
| A-3 | confirmed | Each of 208 actual CI consumers completed 3/3; lifecycle cases use the state/payload marker their scenario requires |
| A-4 | broken | Existing p05 polls predated helper rewrites in `73978cd75e`; source recognition was the issue for those historical entries |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Named waits, count gate and completed final drain | `observerWalk.readWait`, `scriptedPeer.waitedOnBy`, `observeConfigured`, custom shutdown producers | Yes; committed docs carry anchors |
| FlowSpec process ownership | `peerPIDs07` compares canonical argv, case inode and executable inode | Yes; `d52985d520` |
| Runner readiness and group lifecycle | `awaitPeerListening`, `ServerProcess.StopGroup`, `allTestsRun.suiteChild` | Yes; `12afa215f6`, `e0e95030e8` |
| Own-system carrier and conntrack preparation | `dockerLabMode`, `Run.dockerLabCommand`, `allTestsRun.executeOne`, `guestConntrackSetup` | Yes; `934e7d8c03`, `3265a72946` |
| Image tool payload and mountpoint | `shippedLinks`, `staticRecipe`, `extraFileDirs`; archive and guest-alias regressions | Yes; `1dd25fba1d` |
| Existing Stop and VPN claims | `ServerProcess.Stop`, `runVPNKernelHere`, `dockerLabGuest.cidataFiles` | Yes; native scoped reviewed-claim validation exit 0 |
| Feature declaration, config, API/RPC, plugin SDK, wire, RFC, comparison, route metadata, metrics | Owned patch census contains no schema/RPC/wire/metric producer changes; `docs/` anchor census found the harness pages above and unchanged-claim ledger | No update required; Stage 1 completion repair preserves the declared protocol |
| Documentation check limit | Implementation's `doc wiring` includes `doc check all`; sleep/path checks pass but foreign drift and three inventory anchors fail | Recorded honestly; no global green claim |

## Core Insight

Peer readiness and completed delivery are separate facts. A named state or
counter gate prevents shutdown before connection progress; a checked final
quiesce completes buffered delivery after the last relevant emit.
