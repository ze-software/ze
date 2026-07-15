# spec-fixit-ddos-test-infra

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-15 |

## Task

Two ddos test-infrastructure gaps left by `spec-ddos-direction-allowlist`, grouped because
both are test-infra follow-ups from the same parent spec and both touch the same `.ci`
harness under QEMU.

**Problem A:** `test/plugin/ddos-detect-mitigate.ci` is built on a `daemon.pid` /
`daemon.ready` file handshake and has never been run green. Its driver polls for those
files (`:39-47`) and the test was authored for a QEMU run that its own header says never
happened. Rework it onto the in-daemon `ze_api` observer-probe pattern used by
`ddos-policy.ci` / `ddos-direction.ci` / `ddos-bps-amplification.ci`.

**Problem B:** `spec-ddos-direction-allowlist` AC-10 is unproven end to end: no QEMU
functional proof that a transit victim gets an nft FORWARD-hook drop when
`ddos { local { forward-mitigation } }` is on. It needs a 2-interface transit topology
(veth + forwarding); the loopback harness only exercises local -> INPUT because loopback
victims are always box-owned (RTN_LOCAL). Hook selection IS unit-tested at
`internal/plugins/ddos/local/responder_test.go:179` (`TestLocalHookByDirection`), so this
is a missing functional proof, not missing behavior.

Goal: both gaps closed. Rework the dead test; add the transit-topology QEMU proof.

**IMPORTANT (found at authoring, needs research first):** the deferral row's premise for
Problem A, that the handshake "is never satisfied", appears CONTRADICTED by current runner
code. See Problem / Evidence. Establish whether the handshake is genuinely dead before
deciding the rework shape.

Skeleton = captured intent with verified `file:line` evidence. Research happens via
`/ze-spec` when this is picked up; the spec moves to `design` then.

## Origin

Two `plan/deferrals.md` rows dated 2026-07-12, both from `spec-ddos-direction-allowlist`:
- `plan/deferrals.md:54` "(test infra)": rework `ddos-detect-mitigate.ci` off the dead
  `daemon.pid` / `daemon.ready` handshake onto the `ze_api` observer-probe pattern.
- `plan/deferrals.md:57` "(AC-10)": QEMU functional proof of the remote -> FORWARD-hook
  drop; needs a 2-interface transit topology.

Both recorded as "none yet (future spec-followup-ddos)". Grouped here because both are
ddos test-infrastructure follow-ups from the same parent spec.

### Scope

- IN: `test/plugin/ddos-detect-mitigate.ci` reworked and actually green under QEMU.
- IN: a transit-topology `.ci` proving remote -> FORWARD drop (parent AC-10).
- OUT: the firewall concurrency deadlock itself (`plan/spec-fixit-firewall-concurrency-deadlock.md`).
  Problem B may be blocked by it: a FORWARD drop proof needs the nft backend loaded, which
  is exactly the combination that hung dispatch. See R-1.
- OUT: per-source drop-term narrowing and the flowspec withdraw NOTE (`plan/deferrals.md:56,58`).

## Required Reading

### Source (read before designing)
<!-- NEVER tick [ ] to [x]. Capture insights as -> Decision: / -> Constraint:. -->
- [ ] `test/plugin/ddos-detect-mitigate.ci` (:1-119, header :11-22, handshake poll :39-47,
      cmd block :113-118) - the test to rework
- [ ] `test/plugin/ddos-direction.ci` (:33-120 probe, :153-158 plugin block, :198-200 cmd) -
      the target pattern: in-daemon external plugin + `ze_api` 5-stage handshake
- [ ] `test/plugin/ddos-bps-amplification.ci` - same pattern, named in both the deferral
      row and `ddos-direction.ci:19` as the harness of record
- [ ] `test/plugin/ddos-policy.ci` - same pattern; uses victim 127.0.0.2
- [ ] `test/scripts/ze_api.py` - the probe API (`API`, `result_text_data`, `runtime_fail`,
      the 5-stage handshake)
- [ ] `internal/test/runner/runner_exec.go` (:697-705 ZE_READY_FILE arming, :779-800
      daemon.ready / daemon.pid writing) - decides whether the Problem A premise holds
- [ ] `internal/test/runner/runner_exec_util.go` (:118-130 zeReadyFileEnabled) - gates the
      handshake on binary `ze` + non-empty TmpfsTempDir + foreground or background mode
- [ ] `internal/plugins/ddos/local/responder.go` (:156-175 hookForDirection) - maps victim
      direction to the netfilter hook; FORWARD only when forward-mitigation is on
- [ ] `internal/plugins/ddos/local/responder_test.go` (:179-183 TestLocalHookByDirection) -
      existing unit coverage of AC-9/AC-10/AC-11 hook selection
- [ ] `internal/plugins/ddos/local/config.go` (:25-31 ForwardMitigation, :71-73 parse) -
      the `forward-mitigation` leaf Problem B must switch on

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` directives, options, handshakes
  -> Constraint: (fill during research) what the format supports for multi-interface setup
- [ ] `ai/rules/qemu-testing.md` - QEMU integration tests are mandatory for linux-only code
  -> Constraint: both problems here are QEMU-only by nature
- [ ] `plan/learned/1110-ddos-direction-allowlist.md` - parent spec's learned summary
  -> Decision: (fill during research) why AC-10 was left to a functional follow-up

**Key insights:** (fill during research)

## Current Behavior (MANDATORY)

**Source files read (2026-07-15, spec author):**
- [ ] `internal/test/runner/runner_exec_util.go` (:125-130 zeReadyFileEnabled) - returns
      true for binary `ze` with a non-empty TmpfsTempDir in foreground OR background mode,
      so a background `ze` IS armed for the readiness handshake
- [ ] `internal/test/runner/runner_exec.go` (:702-705) - when armed, sets
      `ZE_READY_FILE=<TmpfsTempDir>/daemon.ready`; (:786-789) waits for the readiness file
      then writes `daemon.pid`
- [ ] `internal/plugins/ddos/local/responder.go` (:160 hookForDirection) - returns the hook
      and an ok flag; a remote victim with forward-mitigation off returns not-ok and the
      responder logs "deferring to flowspec" (:113-117) instead of installing a drop
- [ ] `internal/plugins/ddos/local/config.go` (:31 ForwardMitigation) - `forward-mitigation`
      JSON leaf, parsed at :71-73
- [ ] `internal/plugins/ddos/local/responder_test.go` (:179 TestLocalHookByDirection) -
      unit test whose comment states it validates AC-9/AC-10/AC-11: INPUT for local,
      FORWARD for remote when forward-mitigation is on, no drop for remote when off

**Non-Go files read (same session):**
- `test/plugin/ddos-detect-mitigate.ci` (119 lines): header `:11-22`; driver polls
  `daemon.pid` + `daemon.ready` for up to 400 iterations then exits 1 (`:39-47`); reads the
  pid (`:49-50`); floods 127.0.0.2 and greps `nft list ruleset` for `ddos-local` (`:62-75`);
  SIGTERMs the daemon by pid (`:78`); launches `ze -` as `cmd=background:seq=1` with
  `driver.py` as `cmd=foreground:seq=2` (`:113-114`).
- `test/plugin/ddos-direction.ci` (201 lines): the target pattern. In-daemon probe declared
  via `plugin { external ddos-direction-probe { run ./ddos-direction-probe.run } }`
  (`:153-158`), `.run` script imports `ze_api` (`:42`), 5-stage handshake (`:80-86`),
  dispatches `show ddos incidents` through `ze-plugin-engine:dispatch-command` (`:49-53`),
  and the daemon runs as `cmd=foreground:seq=2` (`:199`).

**Behavior to preserve:**
- What `ddos-detect-mitigate.ci` intends to validate: cp-survival-5-detect-5 AC-1 (the
  detector fills the victim DstPrefix and emits a populated AttackDetected) and AC-2
  (ddos-local in enforce mode installs an nft drop for the attacked destination), per its
  header `:3-9`. The rework must keep proving both, not weaken to a log-grep.
- The `ze_api` observer-probe pattern and its 5-stage handshake exactly as the three
  working tests use it; do not fork a variant.
- Victim-address separation across tests so parallel runs do not collide
  (`ddos-direction.ci:44` notes 127.0.0.3 vs `ddos-policy.ci`'s 127.0.0.2).
- `TestLocalHookByDirection` stays the exhaustive hook-selection unit test; the new `.ci`
  complements it, does not replace it.
- The sink-socket trick that stops ICMP port-unreachable backscatter from mis-resolving
  the victim (`ddos-direction.ci:89-93`, learned/1109).

**Behavior to change:**
- None in product code (test infrastructure only), unless research proves the transit
  proof needs a product-side seam. If it does, that is a scope change: raise it with the
  user (`ai/rules/no-partial-completion.md`).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Test harness: `bin/ze-test plugin --pattern ddos-detect-mitigate` (and the new transit
  test) under QEMU (`option=needs-linux`)
- In-daemon probe: `plugin { external <name> { run ./<name>.run } }` in the `.ci` config
- Flood traffic: the probe's UDP socket toward the victim address

### Transformation Path
1. Runner starts `ze -` with the `.ci` config; the external probe plugin is spawned
2. Probe runs the `ze_api` 5-stage handshake (declare_done, wait_for_config,
   capability_done, wait_for_registry, ready, wait_for_post_startup)
3. Probe floods the victim; the iface rate collector reports pps to ddos-detect
4. trafficusage (track-ip) resolves the dominant destination; the detector opens an
   incident and emits AttackDetected with a direction
5. ddos-local `hookForDirection` picks INPUT (local victim) or FORWARD (remote victim with
   forward-mitigation on) and registers the drop table
6. Probe dispatches `show ddos incidents` (and, for the drop proof, inspects the ruleset)
   through `ze-plugin-engine:dispatch-command` and asserts

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| runner -> daemon | `cmd=foreground:exec=ze -` with `.ci` stdin config | [ ] |
| probe -> daemon | external plugin over the `ze_api` handshake + dispatch-command | [ ] |
| probe -> kernel | UDP flood to the victim; veth topology for the transit case | [ ] |
| detector -> responder | `ddosevent.Detected` / `Characterized` on the event bus | [ ] |
| responder -> kernel | firewall RegisterTables + ApplyAll -> nft backend | [ ] |

### Integration Points
- `test/plugin/` the `.ci` suite and its QEMU runner
- `test/scripts/ze_api.py` the probe API
- `internal/test/runner/` harness: readiness handshake, tmpfs, privilege drop
- `internal/plugins/ddos/local/` the responder under test
- `internal/test/plugins/fakeddos/` an existing in-daemon test plugin that emits ddos
  events; may be a shortcut for driving the transit case deterministically

### Architectural Verification
- [ ] No bypassed layers (the test drives the real daemon path, not a stub)
- [ ] No unintended coupling (no product change to make a test pass)
- [ ] No duplicated functionality (reuse `ze_api`, do not fork a probe variant)
- [ ] Registration over hardcoding - the probe registers as an external plugin like the
      working tests do

## Problem / Evidence

### Problem A: `ddos-detect-mitigate.ci` never ran green

**CONFIRMED (read 2026-07-15):**
- The test polls for `daemon.pid` and `daemon.ready` and hard-fails if they never appear
  (`ddos-detect-mitigate.ci:39-47`).
- Its header `:11-14` says: "authored for the Linux/QEMU integration run
  (option=needs-linux); it is NOT executable on the darwin dev host and has not been run
  there". The header then lists three runtime behaviors that "must be confirmed under
  QEMU" (`:14-22`). So the file itself records that its runtime path is unconfirmed.
- The three working ddos tests (`ddos-policy.ci`, `ddos-direction.ci`,
  `ddos-bps-amplification.ci`) all use `ze_api` instead of the file handshake.
- `ddos-detect-mitigate.ci` runs the daemon as `cmd=background:seq=1` with a foreground
  `driver.py` (`:113-114`); `ddos-direction.ci` inverts this: probe in-daemon, `ze -` as
  `cmd=foreground:seq=2` (`:199`).

**CONTRADICTED / needs re-verification (the reason this is research-first):**
- The deferral row (`plan/deferrals.md:54`) states the handshake "is never satisfied". Current
  runner code appears to satisfy it: `zeReadyFileEnabled` (`runner_exec_util.go:125-130`)
  returns true for `ze` in **background** mode with a TmpfsTempDir, `runner_exec.go:702-705`
  arms `ZE_READY_FILE`, and `:786-789` writes `daemon.pid` after the ready file appears.
  The comment at `runner_exec.go:699-701` explicitly says arming covers "background ze
  (driver.py-style suites poll daemon.pid/daemon.ready)". `runner_exec.go:740-743` records a
  prior fix ("Fix B") to make that handshake work under privilege drop.
- Therefore: either the handshake support postdates the 2026-07-12 deferral, or the row's
  diagnosis was wrong and the test fails for a different reason (the header's three
  unconfirmed runtime behaviors are the obvious suspects: `lo` RxPackets reaching the
  detector, trafficusage TCX on `lo`, sustaining >1000 pps across two collect ticks).
  **Establish which before choosing the rework shape.** The right fix might be smaller
  (run it and fix what actually breaks) or different (the trigger does not fire on
  loopback, as the header itself anticipates at `:21-22`).

**UNVERIFIED:**
- Whether the test passes today if simply run under QEMU. Nobody has reported running it.
- The exact `:111-116` line range in the deferral row: at those lines the file has the
  config terminator and the `cmd=` / `expect=` block, not a "has NOT been run" statement.
  The "has not been run" wording is at `:11-14`, and it is scoped to the darwin dev host.

### Problem B: AC-10 transit -> FORWARD drop unproven

**CONFIRMED (read 2026-07-15):**
- Hook selection is unit-tested: `responder_test.go:179` `TestLocalHookByDirection`, whose
  comment (`:180-182`) states it validates AC-9/AC-10/AC-11 (INPUT local, FORWARD remote
  with forward-mitigation on, no drop remote with it off).
- The product path exists: `hookForDirection` (`responder.go:160`) and the
  `forward-mitigation` leaf (`config.go:31`, parsed `:71-73`).
- No `.ci` covers it. `ddos-direction.ci` asserts classification only and carries a
  `test-relax` note (`:11-16`) saying the on-host drop assertion was dropped because
  loading the nft backend under flood deadlocked command dispatch.

**UNVERIFIED:**
- Whether a veth transit topology can be built inside the current `.ci` format without new
  harness directives.
- Whether the FORWARD proof is blocked by the firewall concurrency deadlock (it needs the
  same nft backend + flood combination that hung dispatch). This is the key sequencing
  question: see R-1.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `daemon.pid`/`daemon.ready` handshake is genuinely dead for this test | `plan/deferrals.md:54` | Contradicted by `runner_exec_util.go:125-130` + `runner_exec.go:702-705,786-789`: the rework premise collapses and the real failure is elsewhere (likely the header's unconfirmed runtime behaviors) | run the test under QEMU and observe the actual failure | **likely broken, verify first** |
| A-2 | The `ze_api` observer-probe pattern can express what this test asserts (an nft ruleset check, not just an incident field) | `ddos-direction.ci` dispatches commands but greps no ruleset; `ddos-detect-mitigate.ci:69` shells out to `nft list ruleset` | the probe needs a ruleset-inspection path, or the assertion moves to a dispatch-command surface | prototype the probe; check what the probe process may exec under QEMU | unvalidated |
| A-3 | A loopback flood can drive the detector at all | `ddos-direction.ci` does exactly this and is the harness of record | Problem A's rework must switch to a dedicated veth, as the header anticipates (`:21-22`) | the three working tests already pass under QEMU | likely true |
| A-4 | A veth transit topology is expressible in the current `.ci` format | none: not checked | new harness directives are needed; scope grows and the user must approve | read `docs/architecture/testing/ci-format.md` + existing multi-interface tests | unvalidated |
| A-5 | The FORWARD proof needs the real nft backend loaded | an nft FORWARD-hook drop is the assertion | if the assertion can be made on registered-table state instead, the deadlock does not block it (but the proof is weaker and may not satisfy AC-10) | design review against the parent AC-10 wording | unvalidated |
| A-6 | Problem A and Problem B belong in one spec | both are ddos test-infra from the same parent (`deferrals.md:54,57`) | split them; B may be blocked on the deadlock fix while A is not | first design review | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Problem B is blocked by the firewall concurrency deadlock: the FORWARD proof needs nft + flood, the exact combination that hung dispatch (`ddos-direction.ci:11-16`) | the new transit test hangs like the original observation | sequence B after `spec-fixit-firewall-concurrency-deadlock`; if B is attempted first and hangs, STOP and report: do not add a sleep or relax the assertion |
| R-2 | Rework proceeds on the wrong premise (A-1) and rewrites a test that only needed running | the reworked test fails the same way the original would have | run the original under QEMU FIRST; capture the real failure before rewriting |
| R-3 | The transit test is flaky (timing, veth setup, forwarding sysctls) | intermittent red in CI | build on the working pattern's pacing (`api.read_line` poll, no blind sleeps); every `.ci` sleep needs a comment per the repo gate |
| R-4 | Fixing the test tempts a product change to make it pass | a diff touching `internal/plugins/ddos/` appears | test-infra only; a product-side seam is a scope change requiring user approval |
| R-5 | Two problems in one spec: one lands, one stalls, spec closes "partially" | B blocked at review time | Not allowed to close partially (`ai/rules/no-partial-completion.md`). If B is blocked, keep the spec open or split with user approval |
| R-6 | Parallel-run victim-address collision with existing ddos tests | intermittent cross-test failures | pick fresh victim addresses; follow `ddos-direction.ci:44` |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Flood a box-owned victim under QEMU | -> | detector fills DstPrefix; ddos-local installs the INPUT drop | `test/plugin/ddos-detect-mitigate.ci` (reworked) |
| Flood a transit victim with `forward-mitigation` on | -> | `hookForDirection` selects FORWARD; nft FORWARD-hook drop installed | `test/plugin/ddos-transit-forward-drop.ci` (new) |
| Flood a transit victim with `forward-mitigation` off | -> | no on-host drop; responder defers to flowspec (`responder.go:113-117`) | `test/plugin/ddos-transit-forward-drop.ci` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Research on Problem A complete | The real reason `ddos-detect-mitigate.ci` does not pass is established by running it under QEMU and observing the failure, not inferred. A-1 resolved either way |
| AC-2 | `test/plugin/ddos-detect-mitigate.ci` after rework | Runs green under QEMU and uses the in-daemon `ze_api` observer-probe pattern; no `daemon.pid` / `daemon.ready` polling remains |
| AC-3 | The reworked test's assertions | Still prove the original intent (header `:3-9`): the detector emits a populated victim DstPrefix AND ddos-local installs an nft drop for it. Not weakened to a log-grep or an incident-field check alone |
| AC-4 | A 2-interface transit topology (veth + forwarding) under QEMU, victim reachable through the box, `ddos { local { forward-mitigation } }` on | An nft drop for the victim is installed on the FORWARD hook (parent AC-10 proven end to end) |
| AC-5 | Same topology, `forward-mitigation` off | No on-host drop is installed; the responder logs deferral to flowspec (`responder.go:115-117`) |
| AC-6 | Both new/reworked tests | Registered in the `.ci` suite and actually executed by the QEMU run (not skipped, not `needs-linux`-excluded from the gate) |
| AC-7 | `TestLocalHookByDirection` | Still passes and remains the exhaustive hook-selection unit test |
| AC-8 | The parent deferral rows | `plan/deferrals.md:54` and `:57` resolved or updated with the outcome |
| AC-9 | Any `.ci` sleep introduced | Carries a comment justifying it, per the repo gate on `.ci` `time.sleep` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator's box is flooded on its own address; ddos-local drops it on INPUT | flood -> detect -> characterize -> INPUT drop | `test/plugin/ddos-detect-mitigate.ci` |
| 2 | Operator transits traffic to a downstream victim and enables forward-mitigation | flood -> detect (remote) -> FORWARD drop | `test/plugin/ddos-transit-forward-drop.ci` |
| 3 | Same, forward-mitigation left off | flood -> detect (remote) -> no on-host drop, flowspec owns it | `test/plugin/ddos-transit-forward-drop.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLocalHookByDirection` | `internal/plugins/ddos/local/responder_test.go` (:179, exists) | AC-7 regression guard: hook selection stays exhaustively covered | exists |

Note: this spec is test infrastructure. No new product unit tests are expected. If
research shows a product seam is required, that is a scope change (R-4) and the user
approves it before any product code moves.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| flood rate vs `absolute-floor` (pps) | must exceed 1000 across >=2 collect ticks | - | - | - |
| test timeout (seconds) | working pattern uses 90s (`ddos-direction.ci:24`) | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-detect-mitigate.ci` | `test/plugin/` | reworked onto the `ze_api` probe; local victim flood -> INPUT drop installed (needs-linux) | rework |
| `ddos-transit-forward-drop.ci` | `test/plugin/` | veth transit topology; remote victim flood + forward-mitigation on -> FORWARD drop; off -> no drop (needs-linux) | new |

### QEMU Evidence
| Check | Command | Status |
|-------|---------|--------|
| Both tests green on the QEMU Alpine VM | `bin/ze-test plugin --pattern ddos` | |

## Files to Modify
- `test/plugin/ddos-detect-mitigate.ci` - rework onto the in-daemon `ze_api` probe pattern
- `plan/deferrals.md` - resolve rows `:54` and `:57` at closure
- `internal/test/runner/runner_exec.go` - ONLY if research proves a harness gap blocks the
  transit topology (veth setup / forwarding); user approval first (R-4)
- `docs/architecture/testing/ci-format.md` - only if a new harness directive is added

## Files to Create
- `test/plugin/ddos-transit-forward-drop.ci` - the AC-10 transit proof
- the probe `.run` scripts embedded in those `.ci` files (tmpfs blocks, per the pattern)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | N/A | `forward-mitigation` already exists (`config.go:31`) |
| CLI commands/flags | N/A | no new command |
| Functional test for the gap | Yes | both `.ci` files above |
| Doctor check for runtime dependencies | N/A | no new runtime dependency |
| Prometheus counters/metrics | N/A | test infrastructure only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | No (test infrastructure); confirm at design |
| 9 | Test infrastructure / harness changed? | [ ] | `docs/architecture/testing/ci-format.md` if a directive is added |
| 12 | Internal architecture changed? | [ ] | `docs/functional-tests.md` if the suite listing changes |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, Risks & Assumptions (resolve A-1 FIRST) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases
1. **Phase: Resolve A-1 (BLOCKING, do this first)** - run `ddos-detect-mitigate.ci` as-is
   under QEMU. Capture the actual failure. This decides whether Problem A is a rework, a
   small fix, or a different problem entirely. Do not rewrite the test before this.
2. **Phase: Rework Problem A** - port to the in-daemon `ze_api` probe following
   `ddos-direction.ci`; keep both original assertions (AC-3); green under QEMU.
3. **Phase: Transit topology** - build the veth + forwarding harness; prove remote victim
   classification reaches the responder.
4. **Phase: Problem B proof** - FORWARD drop with forward-mitigation on; no drop with it
   off. If this hangs on the firewall deadlock, STOP: R-1 applies, report to the user, do
   not work around it.
5. **Phase: Close** - resolve `plan/deferrals.md:54,57`; both tests in the QEMU gate.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | The tests assert real on-host behavior, not proxies for it |
| No workaround | No sleep-to-pass, no relaxed assertion, no product change to make a test green (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| Pattern reuse | The probes use `ze_api` as-is; no forked variant |
| Completeness | Both problems closed, or the spec stays open (R-5) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Reworked local-victim test green | `bin/ze-test plugin --pattern ddos-detect-mitigate` |
| Transit FORWARD drop proven | `bin/ze-test plugin --pattern ddos-transit-forward-drop` |
| Hook unit coverage intact | `go test -run TestLocalHookByDirection ./internal/plugins/ddos/local/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Test isolation | The veth topology and flood stay inside the QEMU VM; no traffic escapes to the host |
| Address collisions | Fresh victim addresses; no interference with parallel ddos tests |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The original test passes when run (A-1 broken) | STOP the rework; report to user; re-scope Problem A |
| Transit test hangs on the nft/flood combination | R-1: sequence behind the deadlock fixit; report, do not work around |
| `.ci` format cannot express the veth topology | A-4 broken: harness work needed; user approves the scope change |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The `daemon.pid`/`daemon.ready` handshake "is never satisfied" (`plan/deferrals.md:54`) | Current runner code arms it for background `ze`: `zeReadyFileEnabled` accepts `modeBackground` (`runner_exec_util.go:125-130`), `runner_exec.go:702-705` sets ZE_READY_FILE, `:786-789` writes daemon.pid | Spec authoring, 2026-07-15: read the runner while verifying the deferral's claim | Recorded as A-1; premise must be re-verified before any rework |
| The header says the test "has NOT been run" under QEMU | The header (`:11-14`) says it is not executable on the darwin dev host and "has not been run there" (darwin), and lists runtime behavior to confirm under QEMU. The stronger claim is not what the file says | Read `ddos-detect-mitigate.ci:11-22` | Wording corrected here; the conclusion (never proven green) still holds |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- The working pattern inverts the process topology: the probe lives IN the daemon as an
  external plugin and the daemon runs foreground (`ddos-direction.ci:199`), whereas the
  broken test backgrounds the daemon and drives it from a foreground `driver.py`
  (`ddos-detect-mitigate.ci:113-114`). That inversion, not just the handshake, is the
  substance of the rework: it removes the need for a pid/ready file at all.
- `internal/test/plugins/fakeddos/` is an existing in-daemon plugin that emits ddos events
  and its own comments reference the `daemon.pid` / `daemon.ready` directory convention. It
  may offer a deterministic way to drive the transit case without a real flood: worth
  evaluating in Phase 3 before building a veth flood.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Problem B may be gated on `plan/spec-fixit-firewall-concurrency-deadlock.md` (R-1). If so,
  this spec cannot close until that one does, or the two problems split.

## Implementation Summary
### What Was Implemented
- (fill at completion)
### Bugs Found/Fixed
- (fill at completion)
### Documentation Updates
- (fill at completion)
### Deviations from Plan
- (fill at completion)

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Dead test reworked and actually green | functional (QEMU) | `ddos-detect-mitigate.ci` passing output pasted at closure |
| Parent AC-10 proven end to end | functional (QEMU) | `ddos-transit-forward-drop.ci` showing an nft FORWARD-hook drop |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Both `.ci` tests actually run in the QEMU gate (AC-6), not silently skipped
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); A-1 resolved before any rework
- [ ] `plan/deferrals.md:54` and `:57` resolved or updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] QEMU evidence pasted (both tests green)
- [ ] Goal Validation table filled with concrete evidence

## Open Questions (research before design)

- **Does `ddos-detect-mitigate.ci` actually fail today, and how?** Run it under QEMU before
  touching it. The deferral's stated cause looks wrong (A-1). Everything else about
  Problem A depends on this answer.
- Was the runner's background-mode readiness arming (`runner_exec.go:699-705`) added after
  2026-07-12? If yes, the deferral was accurate when written and is simply stale.
- Can the `ze_api` probe inspect the nft ruleset (exec `nft list ruleset`) under QEMU, or
  must the drop assertion go through a dispatch-command surface (A-2)?
- Does the `.ci` format support creating a veth pair and enabling forwarding before the
  daemon starts, or is a new harness directive needed (A-4)?
- Can `internal/test/plugins/fakeddos/` drive a synthetic remote-victim AttackDetected,
  making the FORWARD proof deterministic without a real transit flood? Would that still
  satisfy the parent AC-10, which asks for a functional proof?
- Is Problem B blocked on the firewall concurrency deadlock (R-1)? Decide the sequencing
  with the user before starting Phase 4.
- Should the two problems stay in one spec, or split now that B looks blocked and A does
  not (A-6)?

## Notes
- Authored 2026-07-15 as a skeleton from `plan/deferrals.md:54` and `:57`. Every `file:line`
  here was read at authoring time. Two claims from the deferral rows did not survive
  verification and are recorded in the Mistake Log; the most important is A-1, which
  inverts the starting move for Problem A from "rewrite it" to "run it and see".
