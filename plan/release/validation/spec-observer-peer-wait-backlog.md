# Spec: observer-peer-wait-backlog

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release validation: 240 functional fixtures whose observer can stop ze before its scripted peer has its messages, so their verdict depends on host load and a green run proves less than it claims |
| RFC gaps | - |
| Depends | - (the all-family guard `TestPluginObserversWaitForTheScriptedPeer`, `internal/test/fixture/plugin_fixture_peer_wait_test.go`, may still be uncommitted pending its `ze-test-judge` verdict; phase 1 starts only once it is committed, A-1) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

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
(`internal/test/fixture/plugin_fixture_peer_wait_test.go`) reads every
registration and its `.ci`, and refuses an observer that never waits, by name,
address or count, for a scripted peer that expects ze's wire output. Its
family 08 and 12 wrappers (`TestPluginObservers08WaitForTheScriptedPeer`,
`TestPluginObservers12WaitForTheScriptedPeer`) are green: those families were
fixed in dd11f5d194, 17d5fb358d and 7db9137e34. Over every family it reports
**281 missing waits across 240 fixtures** (list at HEAD on 2026-11-02, taken by
running the guard; regenerate it the same way):

| Family | Missing waits | Wait helper the family already has |
|--------|---------------|------------------------------------|
| 01 | 11 | `plugin01WaitCounter` |
| 02 | 30 | none |
| 03 | 15 | `waitPeerCounter03`, `waitPeersEOR03` |
| 04 | 14 | `waitPeerEOR04` |
| 05 | 20 | none |
| 06 | 10 | `fixture06WaitEOR` |
| 07 | 4 | `waitEOR07` |
| 09 | 27 | `waitL2TPBGPEOR09` |
| 10 | 12 | `fixture10WaitEOR` |
| 11 | 3 | none |
| 13 | 22 | `waitEORSent13` |
| 14 | 39 | none |
| 15 | 16 | `plugin15WaitEOR` |
| 16 | 14 | `plugin16WaitEOR` |
| misc | 5 | `waitPeerEORSent` |
| other (register_*.go and the rest) | 39 | none of its own |

The drivers carrying the most findings: `observer02` (35, families 02 and
other), `plugin14Observer` (29), `plugin15Observe` (14), `observe03` (14),
`plugin14EventObserver` (11), `routeServerReplay13` (11), `rsObserver04` (10).
Of the 240 fixtures, 227 are `test/plugin/`, 7 `test/static/`, 5
`test/reload/` and 1 `test/ui/`; 26 carry `option=needs-linux`.

The goal: every observer waits for each scripted peer its `.ci` expects
output on, the guard over all families reports zero findings, and every
changed fixture passes three runs in a row. This is the work the sleep rule
asks for (`ai/rules/points/testing/ci-sleep-justification/justify-every-sleep-in-a-ci-test.md`:
"A sleep MUST be converted to a deterministic wait whenever a condition exists
to wait on"), applied to the compiled observers.

### Open design point, decided here

Two drivers wait through an event stream the guard does not read.
`rsObserver04` (`plugin_fixture_04_route_server.go`) subscribes to sent UPDATE
events and loops until `expected` distinct peers have been sent an End-of-RIB
(an UPDATE with no NLRI) and each non-source peer the forwarded prefix.
`routeServerReplay13` (`plugin_fixture_13.go`) does the same for two distinct
peers, from the sent events and from `eor-sent` in `show bgp peer * detail`.

The contract's words are "the peer state or sent-payload counter its wire
assertions require". A sent event is emitted only after the message was
written and flushed to the socket (`internal/component/bgp/reactor/session_write.go`,
the `onMessageReceived(..., rpc.DirectionSent, ...)` call after the write), so
a sent End-of-RIB per distinct peer is the same fact `eor-sent >= 1` gives,
read from the event instead of the counter. **Decision: a count of distinct
peers that were sent their End-of-RIB is a wait, and the guard learns these two
drivers as count waits.** It counts them only where the count is real:

| Driver | Counts as a wait for | Does not count when |
|--------|----------------------|---------------------|
| `rsObserver04(expected, prefix, requireReplay, ...)` | `expected` distinct peers | `requireReplay` is not the literal `true`: the loop then sets its End-of-RIB need to zero and stops at the first forwarded event (`bgp-rs-reactor-fastpath-fallback`), so it waits on no peer; that fixture calls a named wait |
| `routeServerReplay13(name, prefix, seed)` | 2 distinct peers, a constant of the driver | the `.ci` scripts more than two peers, which the count rule already refuses (`waitedOnBy`: count >= scripted) |

The rejected alternative, making every event-stream observer also call a named
helper, adds a second wait that proves nothing the first did not, and leaves the
guard unable to read a valid wait shape the next observer will use again.

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
- The guard reads waits only from calls to helpers listed in `observerWaitFamilies`, with peer names as string literals or package constants; a helper called with a variable names nothing.
- Every family's helpers count for every registration, so a family without a helper calls an existing one rather than gaining a copy.
- The seven precedent commits fixed only the wait; none changed a `.ci`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/fixture/plugin_fixture_peer_wait_test.go` - the all-family guard: `observerWaitFamilies`, the wait kinds (selector, fixed, command, count), `scriptedPeers`, `waitedOnBy`
- [ ] `internal/test/fixture/plugin_fixture_04_route_server.go` - `rsObserver04`: event-stream count of distinct End-of-RIB peers, skipped when `requireReplay` is false
- [ ] `internal/test/fixture/plugin_fixture_13.go` - `routeServerReplay13`: two distinct End-of-RIB peers from events and `eor-sent`
- [ ] `internal/test/fixture/misc_fixture_observers.go` - `waitPeerEORSent` (shared selector wait on `eor-sent`), `quiesce`
- [ ] `internal/test/fixture/plugin_fixture_08.go` - `waitPeersEOR08`, `waitPeerCounter08` (precedent)
- [ ] `internal/test/fixture/plugin_fixture_12.go` - `p12WaitPeersEOR`, `p12WaitPeerCounter` (precedent, budget-scaled through `WaitAttempts`)
- [ ] `internal/component/bgp/reactor/session_write.go` - the sent event fires after a successful write and flush

**Behavior to preserve:**
- Every `.ci` file's expectations, byte for byte. A fix changes the observer, never what the peer expects.
- The family 08 and 12 wrappers and their fixtures stay green.
- An observer's own assertions and its error text.

**Behavior to change:**
- Each of the 281 findings: the observer waits for that scripted peer before it ends.
- The guard reads `rsObserver04` and `routeServerReplay13` as count waits, under the conditions above.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| All-family observer peer-wait guard | `internal/test/fixture/plugin_fixture_peer_wait_test.go` `TestPluginObserversWaitForTheScriptedPeer`, `observerWaitFamilies` | Refuses an observer that never waits for a scripted peer; 281 findings over all families | Extends: the table gains the two event-stream drivers as count waits; the guard is the acceptance check |
| Family wrappers | `plugin_fixture_08_peer_wait_test.go`, `plugin_fixture_12_peer_wait_test.go` | Run the same check over one family; green | Does not change: they keep guarding 08 and 12 |
| Per-peer wait helpers | `waitPeerEORSent` (`misc_fixture_observers.go`), `p12WaitPeersEOR`, `waitPeersEOR08`, and the family helpers in the Task table | Poll `eor-sent` (or a counter) for a named peer | Extends: callers. Families 02, 05, 11 and 14 call an existing helper, never a new per-family copy (`plan/journal/helper-bypassed-by-an-open-coded-copy.md`) |
| Event-stream replay observers | `rsObserver04`, `routeServerReplay13` | Count distinct peers sent an End-of-RIB from sent UPDATE events | Extends: the guard reads them as count waits; the `requireReplay=false` fixture gains a named wait |
| Quiesce barrier | `misc_fixture_observers.go` `quiesce`, `internal/component/plugin/server/quiesce.go` | Drains pending async work, settles a still-connecting peer | Does not apply: it does not cover a peer whose connect has not completed, which is the defect |
| Budget-scaled polling | `Poll`, `WaitAttempts`, `WaitBudget` (`internal/test/fixture`) | Bounded waits that scale with the run's headroom | Extends: new waits use these bounds, never a fixed `time.Sleep` |
| Sleep ratchet | `./le doc wiring`, `// sleep(<kind>):` markers (`docs/architecture/testing/ci-format.md`) | Counts and justifies sleeps | Applies: a sleep a wait replaces is removed; none is added |
| TACACS+ observer guard | `TestTacacsObserversWaitForWhatTheirCIExpects` (`plugin_shell_extra_4_tacacs_test.go`) | Checks mock records, not BGP peers | Does not apply: no scripted BGP peer |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A functional run: `./le test bgp plugin <name>` (or the `static`, `reload`, `ui` suites) reads `test/<suite>/<name>.ci`, starts ze, each `le test peer` stdin block and the compiled observer.
- The guard: a Go test that parses `internal/test/fixture` source and each registered `.ci`.

### Transformation Path
1. The observer registers, receives `OnAllPluginsReady`, runs its scenario.
2. The scenario's wait helper polls `show bgp peer <name> detail` for `eor-sent` (or reads sent events) until each scripted peer has its initial sync on the wire.
3. The scenario asserts, returns; `Observe` quiesces and requests shutdown; the peer sees its expected messages before Cease.
4. The guard maps each registration to its `.ci`, resolves the scripted peers, walks every function the registration reaches, and compares.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Observer ↔ engine | SDK `Dispatch` of `show bgp peer <name> detail`, sent UPDATE events | No |
| Runner ↔ peer | `le test peer` stdin script, `expect=bgp:` lines | No |

### Integration Points
- `observerWaitFamilies` - each helper and count driver is one entry, beside its family.
- `Observe` - unchanged; the wait sits in the scenario or its wrapper before it returns.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Waits read the daemon through SDK commands, as the precedents do |
| No unintended coupling (components stay isolated) | Yes | Test package only |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new per-family helper; existing helpers called |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Test code |
| Registration over hardcoding, outbound | Yes | A new wait shape is one table entry in `observerWaitFamilies`, which is the guard's only list |
| Registration over hardcoding, inbound | Yes | Searched: `observerWaitFamilies` is the one list the guard reads; no other list names wait helpers |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The all-family guard lands as written, with its 08 and 12 wrappers | Brief of 2026-11-02; file untracked at `internal/test/fixture/plugin_fixture_peer_wait_test.go` | The acceptance check differs; re-derive the population from the committed guard | `git log -- internal/test/fixture/plugin_fixture_peer_wait_test.go` before phase 1 | unvalidated |
| A-2 | A sent UPDATE event fires only after the bytes were written and flushed | `session_write.go`, the `DirectionSent` callback after the write | The count-wait decision is wrong; those drivers call a named wait | Read the second `DirectionSent` call site (the forward path, `session_write.go` ~924) too | unvalidated |
| A-3 | Initial-sync `eor-sent >= 1` is reached for every scripted peer the guard names | Precedent fixes (`waitPeersEOR08`, `p12WaitPeersEOR`) | A peer with no family negotiates no End-of-RIB (`plan/journal/silent-fall-through.md`, 2026-08-16); that observer waits on `updates-sent` or Established instead | Each changed fixture green 3/3 | unvalidated |
| A-4 | No finding is a guard false positive | 08 and 12 passes found none | A wrongly resolved peer would force a wait on a peer that never connects | Read each `.ci` before adding the wait; a false positive is fixed in the guard, with the case pinned by a test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A wait added before a scenario that must act before the peer is up (peer create, admin-down) | The fixture times out waiting | Put the wait after the action the `.ci` scripts; the guard reads it anywhere in the reached functions |
| R-2 | A scenario that emits and then returns still ends before the emitted UPDATE is on the wire | Intermittent red after the initial-sync wait | Wait on `updates-sent` stepping past a mark, the `p12EmitOnWire` shape of 7db9137e34 |
| R-3 | Host load makes a 3/3 run red for a reason outside the fixture | Timeouts across unrelated fixtures, load average far above core count | Name the load and rerun only that fixture (`ai/META-DECISIONS.md`: find the cause, no blind retry); `plan/journal/gate-verdict-depends-on-the-machine.md` |
| R-4 | Teaching the guard a count wait that is not conditioned lets `bgp-rs-reactor-fastpath-fallback` pass unwaited | Guard green while that fixture still has no wait | AC-4 mutation: the false branch must stay a finding until the fixture names its peer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: test fixtures only. A wrong wait makes a fixture time out |
| How is it reverted? | Single commit revert per family |
| Who else touches this path? | Every session adding a plugin fixture; the guard refuses a new unwaited observer once this lands |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `go test` of `internal/test/fixture` | → | `checkObserversWaitForTheScriptedPeer` over every family | `TestPluginObserversWaitForTheScriptedPeer` |
| `./le test bgp plugin bgp-rs-reactor-fastpath` | → | `rsObserver04` count wait | `test/plugin/bgp-rs-reactor-fastpath.ci` |
| `./le test bgp plugin prefixsid-malformed-receive` | → | `routeServerReplay13` count wait | `test/plugin/prefixsid-malformed-receive.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `TestPluginObserversWaitForTheScriptedPeer` run over the package | Passes with zero findings over every family |
| AC-2 | Every `.ci` whose observer changed | Passes 3 runs in a row; the 26 `needs-linux` ones pass 3 in a row inside a QEMU guest booting Ze's runtime kernel |
| AC-3 | One fixture of each family 02, 05, 11, 14 and other, with its new wait removed | The guard fails and names that fixture and peer |
| AC-4 | `bgp-rs-reactor-fastpath-fallback` with its named wait removed (`rsObserver04` called with `requireReplay` false) | The guard fails naming it; with `requireReplay` true the same driver satisfies the guard |
| AC-5 | `./le doc wiring` sleep count after the change | Not above its count before; each fixed sleep a wait replaced is gone |
| AC-6 | `git diff` over `test/**/*.ci` | No `.ci` expectation removed or weakened; any `.ci` edit carries a `ze-test-judge` approval |
| AC-7 | The 08 and 12 wrappers | Still pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginObserversWaitForTheScriptedPeer` | `internal/test/fixture/plugin_fixture_peer_wait_test.go` | AC-1, AC-3: red at start (281), green at the end | |
| `TestObserverCountWaitHonoursItsCondition` | `internal/test/fixture/plugin_fixture_peer_wait_test.go` | AC-4: a count entry with a condition argument counts only when that argument is the literal `true`; a fixed-count entry counts its constant | |
| `TestPluginObservers08WaitForTheScriptedPeer`, `TestPluginObservers12WaitForTheScriptedPeer` | existing | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| count wait vs scripted peers | count >= scripted | count == scripted | count == scripted - 1 (finding) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| The 240 fixtures the guard names | `test/plugin/`, `test/static/`, `test/reload/`, `test/ui/` | Each passes 3/3 with its observer waiting for each scripted peer | |

## Files to Modify
- `internal/test/fixture/plugin_fixture_peer_wait_test.go` - count-wait entries for `rsObserver04` (conditioned on its `requireReplay` argument) and `routeServerReplay13` (a fixed count of 2); entries for any helper a family starts calling
- `internal/test/fixture/plugin_fixture_01.go` - 7 registrations (family 01)
- `internal/test/fixture/plugin_fixture_02.go` - 25 (`observer02` and its scenarios)
- `internal/test/fixture/plugin_fixture_02_events.go` - 3 (`eventObserver02`)
- `internal/test/fixture/plugin_fixture_02_srv6_ineligible.go` - 1 (`routeServerReplay13` registration)
- `internal/test/fixture/plugin_fixture_03.go` - 15 (`observe03`)
- `internal/test/fixture/plugin_fixture_04.go` - 14 (`rsObserver04`, `observe04`)
- `internal/test/fixture/plugin_fixture_05.go` - 14 (`p05CommunityStrip` and others)
- `internal/test/fixture/plugin_fixture_05_ddos.go` - 4
- `internal/test/fixture/plugin_fixture_06.go` - 10
- `internal/test/fixture/plugin_fixture_07.go` - 4
- `internal/test/fixture/plugin_fixture_07_irr.go` - 2
- `internal/test/fixture/plugin_fixture_09.go` - 23 (`llgrReadvertise09`, `rsObserver09`, metrics, LG, L2TP show scenarios)
- `internal/test/fixture/plugin_fixture_10.go` - 11
- `internal/test/fixture/plugin_fixture_11.go` - 3 (`prefixCountBounded`, `prefixCountExact`)
- `internal/test/fixture/plugin_fixture_13.go` - 19
- `internal/test/fixture/plugin_fixture_13_rest.go` - 2
- `internal/test/fixture/plugin_fixture_14.go` - 31 (`plugin14Observer`, `plugin14EventObserver`, `plugin14ASPAStateObserver`)
- `internal/test/fixture/plugin_fixture_15.go` - 14 (`plugin15Observe`)
- `internal/test/fixture/plugin_fixture_16.go` - 9
- `internal/test/fixture/plugin_fixture_16_l2tp.go` - 1
- `internal/test/fixture/misc_fixture_observers.go` - 5 (`reloadRESTObserver`, `observeDriver`)
- `internal/test/fixture/plugin_claim_permission.go` - 1
- `internal/test/fixture/plugin_fixture_llgr_import.go` - 1 (`llgrImportNoLLGR`)
- `internal/test/fixture/bgpls_export.go` - 1 (`bgplsExportLifecycle`)
- `internal/test/fixture/register_blackhole_readvertise.go` - 1
- `internal/test/fixture/register_clear_caches.go` - 1
- `internal/test/fixture/register_connected_distance.go` - 2
- `internal/test/fixture/register_gr_sender_facts.go` - 3 (`grSenderFactsDriver`)
- `internal/test/fixture/register_modify_increment_med.go` - 2
- `internal/test/fixture/register_peer_create_delete_rib.go` - 1
- `internal/test/fixture/register_rfc8212_policy.go` - 1 (`rfc8212PolicyDriver`)
- `internal/test/fixture/register_rib_clear_advertised.go` - 1
- `internal/test/fixture/register_show_rib_best_addpath.go` - 1
- `internal/test/fixture/register_static_distance.go` - 7
- `internal/test/fixture/register_update_text_prefix_sid.go` - 1
- `internal/test/fixture/ui_fixture_web_commit_reject.go` - 1 (`uiDriver`)

The count is registrations per file. Where a wait belongs in a shared driver
(`observer02`, `plugin14Observer`, `plugin15Observe`, `observe03`) the edit
lands in the file that defines the driver, which may differ from the file that
registers the fixture.

## Files to Create
- None. No new helper file; families without a helper call `waitPeerEORSent` or `p12WaitPeersEOR`.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test fixtures only |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Prometheus counters/metrics | N-A | Reads existing peer counters only |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Test harness only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, "Waiting without sleep": name the all-family guard and the count-wait rule for event-stream observers |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs of the files above: `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/testing/ci-format.md`, `docs/guide/graceful-restart.md`, `docs/architecture/resolve.md`, `docs/architecture/core-design.md`, `docs/architecture/bgp/egress-attribute-rules.md`, `docs/architecture/api/commands.md`, `docs/functional-tests.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/static-routes.md`, `docs/architecture/api/update-syntax.md`. Each describes the fixture's scenario, not its synchronization, so none changes unless the edit changes what the scenario asserts; confirm with `./le spec citation anchors spec plan/release/validation/spec-observer-peer-wait-backlog.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- confirm A-1, run the all-family guard and save its findings as the starting population.
   - Tests: `TestPluginObserversWaitForTheScriptedPeer` (red, 281)
   - Files: none
   - Verify: the finding count and per-family split match the Task table, or the table is corrected first
2. **Phase: Guard count waits** -- add the conditioned count entry for `rsObserver04` and the fixed count for `routeServerReplay13`, with `TestObserverCountWaitHonoursItsCondition`.
   - Tests: `TestObserverCountWaitHonoursItsCondition` red, then green; the all-family finding count drops by the `requireReplay=true` and two-peer fixtures only
   - Files: `plugin_fixture_peer_wait_test.go`
3. **Phase: Families with a helper** (01, 03, 04, 06, 07, 09, 10, 13, 15, 16, misc) -- one commit per family: each scenario waits for each scripted peer by name before it ends, and on the post-emit counter where it emits (R-2). Run that family's changed `.ci` 3/3.
4. **Phase: Families without a helper** (02, 05, 11, 14, other) -- the same, calling `waitPeerEORSent` or `p12WaitPeersEOR` (or the shared driver's own wait where every scenario of the driver needs it at the same point). Remove each fixed sleep the wait replaces.
5. **Phase: needs-linux proof** -- run the 26 `needs-linux` fixtures 3/3 inside `./le test qemu run kernel <vmlinuz> command "<the ./le test command>"` on Ze's runtime kernel.
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
| Guard green over every family | `./le job run label unit-pkg command go test -tags <the tags ./le test unit uses> ./internal/test/fixture/ -run 'TestPluginObservers'` |
| Changed fixtures 3/3 | `./le test bgp plugin <name>` three times per changed fixture (`static`, `reload`, `ui` suites likewise), output kept |
| needs-linux fixtures 3/3 | `./le test qemu run kernel <vmlinuz> command "..."` log per fixture |
| Change set judged | `./le verify changed` after the closing commit |

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
| A fixture stays red after its wait with ze at fault | That is a product defect the wait exposed: fix it or journal it with an owner (`ai/rules/completion.md`) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `false-synchronization-claim.md` | 2026-11-02 | 281 observer waits missing across 240 fixtures, all-family peer-wait guard |

No observer or sleep row was added to `plan/journal/` by the rules pass of
2026-11-02 (d1c57af45c, a4d48332fe): that pass changed the sleep rule point to
name the guards and added no journal row for it. The row above records this
backlog.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A sent End-of-RIB counted per distinct peer is a wait; the guard learns `rsObserver04` and `routeServerReplay13` as count waits | Make each also call a named helper | The sent event fires after the write (A-2), so it is the contract's "peer state or sent-payload counter"; a second wait proves nothing more |
| `rsObserver04` counts only when `requireReplay` is the literal `true` | Count it unconditionally | Its false branch sets the End-of-RIB need to zero and waits on no peer |
| Families without a helper call `waitPeerEORSent` or `p12WaitPeersEOR` | A new helper per family, as 08 and 12 got | Every family's helpers count for every registration; a per-family copy is the open-coded-copy class |
| Waits sit where the scenario's assertions need them, in the scenario or in a shared driver | Always in the driver | Some scenarios act before their peer is up (R-1) |

## Known Limitations
- The guard proves a wait exists, not that it is placed after the scenario's last emit; R-2 is a review check.

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
- [ ] The targeted tests for what changed pass: the fixture package tests, each changed `.ci` through `./le test bgp plugin <name>` 3/3, the `needs-linux` ones in QEMU, and `./le verify changed` over the closing commits
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
