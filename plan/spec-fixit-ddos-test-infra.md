# spec-fixit-ddos-test-infra

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | ~~0/N (research)~~ design complete; impl Phases 1-5 pending (2026-07-17) |
| Updated | 2026-07-17 |

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
  -> Constraint: the format has NO veth/multi-interface directive and needs none. A veth
     topology is built by a `tmpfs=setup.py` script run as `cmd=foreground:seq=1`, which the
     runner awaits to completion before the daemon starts (`runner_exec.go:797-808`). Precedent:
     `test/vrrp/vrrp-instance-up.ci:34-51,212`. A-4 CONFIRMED; no harness change.
  -> Constraint: `option=netns:veth=...` (used by `test/pppoe/*.ci`) is NOT a real directive.
     `parseOption` (`internal/test/runner/record_parse.go:287-430`) has no `netns` case and
     returns `unknown option type` for anything unrecognized (`:428-430`). Do NOT copy that
     syntax. See Design Insights (orphaned-suite finding, out of scope here).
- [ ] `ai/rules/qemu-testing.md` - QEMU integration tests are mandatory for linux-only code
  -> Constraint: both problems here are QEMU-only by nature
- [ ] `plan/learned/1110-ddos-direction-allowlist.md` - parent spec's learned summary
  -> Decision: AC-10 was left to a functional follow-up because a loopback victim is always
     RTN_LOCAL, so the harness of record could only ever exercise local -> INPUT. That reason
     still holds; a transit topology is genuinely required.

**Key insights:**
- Both premises of the two deferral rows were TRUE on 2026-07-12 and were made FALSE by two
  commits landed on 2026-07-13, one day later. The rows are stale, not wrong.
- The nft ruleset read is MANDATORY for this spec's assertions: no dispatch-command surface
  exposes the netfilter hook (`show ddos local` returns enabled/active/target only,
  `show.go:32-35`). This single fact drives the whole design and contradicts current AC-2.

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

**Source files read (2026-07-16, DESIGN phase -- producers, not callers):**

| # | Finding | Producing code (read) |
|---|---------|----------------------|
| F1 | The readiness handshake is ALIVE end to end. `zeReadyFileEnabled` returns true for a `ze` daemon in foreground OR background with a TmpfsTempDir; the runner arms `ZE_READY_FILE=<tmpfs>/daemon.ready`, waits for that file (when no ze-peer supplies BGP-level sync), then writes `daemon.pid`. | `runner_exec_util.go:125-130`; `runner_exec.go:702-705`, `:777-791` |
| F2 | The daemon really writes it. `ze -` creates the ready file after `WaitForStartupComplete` succeeds. A second producer covers the `hub` path, written after `dropPrivileges`. | `cmd/ze/hub/main.go:945` then `:957-961`; `:1111` then `:1121-1125` |
| F3 | The env key matches. `env.Get("ze.ready.file")` normalizes to `ze_ready_file`; `normalize` lowercases and maps `.`->`_`, so the runner's literal `ZE_READY_FILE` resolves to the same cache entry. The chain has no gap. | `internal/core/env/env.go:41-43`, `:71-99` |
| F4 | **The handshake support postdates the deferral by one day.** The deferral rows are dated 2026-07-12; background-ze readiness landed 2026-07-13. Its message states background ze "got neither ZE_READY_FILE nor a daemon.pid write ... so driver.py-style suites that poll daemon.pid/daemon.ready timed out by construction". | commit `dc082c288` (2026-07-13) |
| F5 | **A live, green test polls exactly this handshake today.** It waits on `daemon.pid` + `daemon.ready` via `ze_api.wait_until`, then asserts kernel state with `ip -j` readback. Its spec closed in `1b8e44053`..`fbc99f1d4` (VRRP vrrp-0..5). This is the pattern of record for kernel-state assertions, and it is NOT the in-daemon probe. | `test/vrrp/vrrp-instance-up.ci:123`, `:59-67`, `:79-171` |
| F6 | **nft is programmed WITHOUT a `firewall {}` block.** `ApplyAll` autoloads the OS-default backend on demand when no backend is active and there are tables to apply, precisely so plugin-owned tables (copp, policy-routes, ddos-local) reach the kernel with no operator firewall block. The default is `"nft"` on Linux. | `firewall/registry.go:79-111` (autoload at `:104-109`), `:53`; `default_linux.go:11` |
| F7 | Without a `firewall {}` section the firewall COMPONENT stays idle and loads no backend, so under autoload the ddos-local responder is the ONLY nft driver. The two-concurrent-driver combination behind the R-1 deadlock requires an explicit `firewall {}` block. | `firewall/engine.go:299-303` (idle branch) vs `:316-324` (LoadBackend + RegisterTables + ApplyAll) |
| F8 | **The autoload also postdates the deferral by one day** (2026-07-13), so the `test-relax` note in `ddos-policy.ci:174-180` ("no firewall {} backend block here ... adding one deadlocked dispatch") was written against a tree where a block was the only way to get a backend. | commit `c5273da42` (2026-07-13) |
| F9 | **No dispatch-command surface exposes the netfilter hook.** `show ddos local` returns `enabled`/`active`/`target` only. Hook selection (INPUT vs FORWARD) is therefore UNPROVABLE through the probe's command surface. | `internal/plugins/ddos/local/show.go:23-37` |
| F10 | The responder's success log DOES carry the hook name (`hook=ingress` or `hook=forward`); the not-ok path logs the flowspec deferral. These are assertable via `expect=stderr:contains=`, but prove responder INTENT, not kernel state. | `responder.go:152-153`, `:111-118`; `hookChainName` `:170-175` |
| F11 | **In the QEMU plugin suite the ze daemon runs as ROOT.** `ZE_TEST_NETNS` is never set by the needs-linux QEMU path, so `netnsModeActive()` is false, the `SysProcAttr.Credential` UID drop is skipped, and `dropPrivileges` is a no-op when `ze.user` is unset. Only the curated firewall subset sets netns. | `runner_exec_util.go:33-35`; `runner_exec.go:745-749`; `cmd/ze/hub/main_system.go:45-53`; `scripts/evidence/qemu-all-tests.sh:125`; `scripts/evidence/netns_qemu.py:107` |
| F12 | The runner's DESIGN INTENT reserves the foreground driver for kernel reads: "ze-peer / driver.py stay privileged (root under sudo) so they can read nft state and signal the daemon", while the ze daemon is the one dropped. | `runner_exec.go:740-744` |
| F13 | A veth topology needs no new directive: a foreground non-ze command that is not the last command is awaited to completion (`proc.Wait()`) before the next starts. The `cmdIdx < len(cmds)-1` guard is load-bearing: a setup script placed last would NOT be awaited. | `runner_exec.go:797-808` |
| F14 | No in-daemon `ze_api` probe test reads the nft ruleset. All three "working" ddos tests assert through dispatch-command only; their `nft` mentions are prose in comments. `ddos-policy.ci` proves the no-drop case via `show ddos local` `active`. | `ddos-policy.ci:69-73,174-180`; `ddos-direction.ci:56-65`; `ddos-bps-amplification.ci:209-256` |
| F15 | The `.ci` sleep ratchet is a COUNT baseline enforced by a make gate, so replacing hand-rolled `time.sleep` polls with `ze_api.wait_until` lowers the count and passes. `wait_until`'s internal sleep is ratchet-exempt. | `scripts/dev/verify_wiring_docs.py:192` (`test/.ci-sleep-baseline`); `test/scripts/ze_api.py:983-997`, `:1405-1414` |

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
- ~~The `ze_api` observer-probe pattern and its 5-stage handshake exactly as the three
  working tests use it; do not fork a variant.~~ **REVISED (D-1):** the rule survives, the
  target changes. Reuse `ze_api` as-is and fork no variant, but the piece this test needs is
  `ze_api.wait_until` (`ze_api.py:983-997`) in a root driver.py, not the 5-stage in-daemon
  handshake. Pattern of record for kernel-state assertions: `vrrp-instance-up.ci:123`.
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
- ~~In-daemon probe: `plugin { external <name> { run ./<name>.run } }`~~ superseded by D-1
- Topology setup: `tmpfs=setup.py` at `cmd=foreground:seq=1` (veth pair + forwarding), awaited
  before the daemon starts (F13)
- Flood traffic: `driver.py`'s UDP socket toward the victim address

### Transformation Path (per D-1, ~~pending approval~~ APPROVED by Thomas 2026-07-16)
1. Runner awaits `setup.py` (seq=1), which builds the veth topology; then starts `ze -` as
   `cmd=background:seq=2` with the `.ci` config, arming `ZE_READY_FILE` (F1)
2. The daemon reaches startup-complete and creates `daemon.ready` (F2); the runner sees it and
   publishes `daemon.pid` (F1). `driver.py` (seq=3, root) waits on both via `ze_api.wait_until`
3. driver.py floods the victim; the iface rate collector reports pps to ddos-detect
4. trafficusage (track-ip) resolves the dominant destination; the detector opens an
   incident and emits AttackDetected with a direction
5. ddos-local `hookForDirection` picks INPUT (local victim) or FORWARD (remote victim with
   forward-mitigation on) and registers the drop table; `ApplyAll` autoloads the nft backend on
   demand (F6) since no `firewall {}` block is declared, and programs the kernel
6. driver.py polls `nft list ruleset` via `wait_until` and asserts the table, victim and hook;
   the runner asserts the responder's `hook=` log through `expect=stderr:contains=` (D-4)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| runner -> daemon | `cmd=background:seq=2:exec=ze -` with `.ci` stdin config | [ ] confirmed at DESIGN: `runner_exec.go:702-705`, `:759-791` |
| runner <-> daemon readiness | `ZE_READY_FILE` -> `daemon.ready` -> `daemon.pid` | [ ] confirmed at DESIGN (F1/F2/F3); proven live by `vrrp-instance-up.ci:123` |
| driver.py -> daemon | `daemon.pid` for SIGTERM; `ze_api.wait_until` for pacing | [ ] |
| driver.py -> kernel | UDP flood to the victim; `nft list ruleset` readback (root, F11/F12) | [ ] |
| detector -> responder | `ddosevent.Detected` / `Characterized` on the event bus | [ ] |
| responder -> kernel | firewall RegisterTables + ApplyAll -> nft backend **autoloaded on demand** | [ ] confirmed at DESIGN: `registry.go:104-109` |

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

**RESOLVED 2026-07-16 (DESIGN): the deferral's premise is BROKEN, and the Open Question
"was the arming added after 2026-07-12?" is answered YES.**

The deferral row (`plan/deferrals.md:54`) states the handshake "is never satisfied". That was
TRUE when written (2026-07-12) and became FALSE one day later:

| Date | Event | Evidence |
|------|-------|----------|
| 2026-07-12 | Deferral rows authored. Background ze got neither `ZE_READY_FILE` nor a `daemon.pid` write, so the poll at `ddos-detect-mitigate.ci:39-47` timed out **by construction**. The row was ACCURATE. | commit message of `dc082c288` |
| 2026-07-13 | `dc082c288` "fix(test): runner emits daemon readiness for background ze" adds `zeReadyFileEnabled` and the `!hasPeer -> waitReady -> publish daemon.pid` path. Handshake ALIVE. | F1, F4 |
| 2026-07-13 | `c5273da42` makes `ApplyAll` autoload the nft backend for plugin-owned tables. ddos-local now programs the kernel with no `firewall {}` block. | F6, F8 |
| 2026-07-15 | `test/vrrp/vrrp-instance-up.ci` ships and closes green while polling `daemon.pid`+`daemon.ready`. Handshake PROVEN in production. | F5 |

-> Decision: the row is **stale, not wrong**. Nothing about it was a diagnostic error; the
tree moved underneath it. Record this in the Mistake Log as a stale-deferral class, not as a
bad diagnosis, and do NOT rewrite the test to escape a handshake that works.

-> Decision: **A-1 is BROKEN.** Both of its consequences follow: (a) the rework premise
collapses, and (b) the residual reason the test has never been green is NOT the handshake. The
remaining suspects are the header's three unconfirmed runtime behaviors (`:14-22`) plus one the
header did not know about: before `c5273da42` the test's `nft list ruleset` grep could not have
matched, because the config declares no `firewall {}` block and nothing else would have loaded a
backend (F6/F7). **The test was doubly dead on 2026-07-12 and is now, on the static evidence,
doubly unblocked.**

**STILL UNVERIFIED (needs the QEMU run; AC-1 keeps it BLOCKING):**
- Whether the test passes today when simply run. The three runtime behaviors at `:14-22` (`lo`
  RxPackets reaching the detector, trafficusage TCX attach on `lo`, sustaining >1000 pps across
  >=2 collect ticks) are unchanged and unproven. F5 proves the HARNESS works, not that the
  loopback flood trips the detector from a driver.py.
  Mitigating evidence: `ddos-direction.ci` and `ddos-policy.ci` DO trip the detector on a
  loopback flood under QEMU, so the trigger itself is sound; only the flooder's location
  (external driver vs in-daemon probe) differs.

**Correction retained:** the deferral's `:111-116` line range does not carry a "has NOT been
run" statement; at those lines the file has the config terminator and the `cmd=`/`expect=`
block. The wording is at `:11-14` and is scoped to the darwin dev host.

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

**RESOLVED 2026-07-16 (DESIGN):**
- **A-4 CONFIRMED: a veth transit topology needs NO new harness directive.** `.ci` builds it
  with a `tmpfs=setup.py` run as `cmd=foreground:seq=1`, which the runner awaits to completion
  before the daemon starts (F13). Working precedent: `test/vrrp/vrrp-instance-up.ci:34-51,212`
  creates a veth pair with `ip link add ... type veth`, addresses it and brings both ends up.
  -> Constraint: the setup script must NOT be the last command, or `runner_exec.go:797` does not
     match and it is not awaited (F13). Order: setup.py seq=1, `ze` seq=2, driver.py seq=3.
  -> Constraint: do NOT use `option=netns:veth=...`. It is invented syntax that `parseOption`
     rejects with `unknown option type` (`record_parse.go:287-430`); it survives only in the
     orphaned `test/pppoe/` suite, which nothing executes.
- **A-2 RESOLVED, and it inverts the spec's chosen pattern.** The FORWARD proof CANNOT be made
  through the probe's dispatch-command surface: `show ddos local` exposes `enabled`/`active`/
  `target` and no hook (F9). `active=true` proves *a* drop exists, never *which hook* it is on,
  so it cannot distinguish AC-4 from the local-victim case. **Reading the nft ruleset is
  mandatory.** The only non-nft signal is the responder's log, which does carry `hook=forward`
  (F10) but proves intent, not kernel state.

**R-1 DOWNGRADED (not eliminated) -- key sequencing answer:**
- The deadlock as recorded needs TWO concurrent nft drivers: the firewall component (driven by a
  `firewall {}` block) and the ddos-local responder. Without that block the firewall component
  is idle and loads no backend (F7), while `ApplyAll` autoloads nft for the responder alone (F6).
- -> Decision: **neither test needs a `firewall {}` block.** Both get a real nft backend via
  autoload, with ddos-local as the sole driver, so neither reproduces the two-driver combination
  that was observed to hang. On this evidence Problem B is **not blocked** and can proceed
  without waiting for `spec-fixit-firewall-concurrency-deadlock`.
- Honesty bound: the deadlock's ROOT CAUSE is recorded as UNVERIFIED (observed symptom only), so
  this reasoning rests on the *recorded* trigger, not a proven mechanism. R-1 stays on the risk
  table: if either test hangs, STOP and report (do not add a sleep, do not relax).
- Corollary worth flagging: since 2026-07-13 autoload means **any** ddos-local enforce test that
  actually installs a drop now loads nft implicitly. `ddos-direction.ci` runs `response-level
  enforce` on an unexempted local victim, so it now programs nft where it previously could not.
  That is a behavior change to an existing green test, and it is the concurrency spec's territory.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `daemon.pid`/`daemon.ready` handshake is genuinely dead for this test | `plan/deferrals.md:54` | The rework premise collapses; the real failure is elsewhere | run the test under QEMU and observe the actual failure | **BROKEN (2026-07-16)**. Handshake is armed (`runner_exec_util.go:125-130`, `runner_exec.go:702-705,777-791`), the daemon writes the file (`cmd/ze/hub/main.go:957-961`), the env key resolves (`env.go:41-43`), it landed in `dc082c288` **one day after** the deferral, and a green test polls it today (`test/vrrp/vrrp-instance-up.ci:123`). Mistake Log + Deviations recorded |
| A-2 | The `ze_api` observer-probe pattern can express what this test asserts (an nft ruleset check, not just an incident field) | `ddos-direction.ci` dispatches commands but greps no ruleset; `ddos-detect-mitigate.ci:69` shells out to `nft list ruleset` | the probe needs a ruleset-inspection path, or the assertion moves to a dispatch-command surface | prototype the probe; check what the probe process may exec under QEMU | **BROKEN as stated (2026-07-16)**. No dispatch surface carries the hook (`show.go:23-37`), so a ruleset read is mandatory and the "assertion moves to dispatch-command" escape does not exist. The probe *could* exec `nft` today only because the QEMU plugin suite leaves ze as root (F11) -- an accident of suite config, not a property of the pattern, and false under netns mode. Drives D-1 |
| A-3 | A loopback flood can drive the detector at all | `ddos-direction.ci` does exactly this and is the harness of record | Problem A's rework must switch to a dedicated veth, as the header anticipates (`:21-22`) | the three working tests already pass under QEMU | **CONFIRMED for an in-daemon flooder** (`ddos-direction.ci:96-106`, `ddos-policy.ci:96-111` both trip the detector on a `lo` flood under QEMU). NOT yet confirmed for an external driver.py flooder -- same kernel path, different process; AC-1's run settles it |
| A-4 | A veth transit topology is expressible in the current `.ci` format | none: not checked | new harness directives are needed; scope grows and the user must approve | read `docs/architecture/testing/ci-format.md` + existing multi-interface tests | **CONFIRMED (2026-07-16)**. `test/vrrp/vrrp-instance-up.ci:34-51,212` builds a veth pair from a `tmpfs=setup.py` at `cmd=foreground:seq=1`, awaited by `runner_exec.go:797-808`. No directive, no runner change. Constraint: setup must not be the last command |
| A-5 | The FORWARD proof needs the real nft backend loaded | an nft FORWARD-hook drop is the assertion | if the assertion can be made on registered-table state instead, the deadlock does not block it (but the proof is weaker and may not satisfy AC-10) | design review against the parent AC-10 wording | **CONFIRMED, and it is FREE**. `ApplyAll` autoloads nft on demand for plugin-owned tables (`registry.go:104-109`, `:53`, `default_linux.go:11`), so the backend loads with no `firewall {}` block and ddos-local is its only driver (`engine.go:299-303`). The registered-table fallback is rejected: it would not prove the kernel hook |
| A-6 | Problem A and Problem B belong in one spec | both are ddos test-infra from the same parent (`deferrals.md:54,57`) | split them; B may be blocked on the deadlock fix while A is not | first design review | **CONFIRMED (2026-07-16)**. The split rationale was "B is blocked, A is not"; R-1 is downgraded (neither test needs a `firewall {}` block, F6/F7) so both are unblocked. They also now share one harness (setup.py + driver.py + nft readback), making a split actively wasteful |
| A-7 | The in-daemon probe can exec `nft list ruleset` under QEMU | new at DESIGN | if false, the probe pattern cannot satisfy AC-3/AC-4 at all | read the QEMU suite's privilege wiring | **CONFIRMED-BUT-FRAGILE (2026-07-16)**. True only because `ZE_TEST_NETNS` is unset in the needs-linux QEMU path, so `netnsModeActive()` is false, the UID drop is skipped, and `dropPrivileges` no-ops without `ze.user` (F11). Under netns mode (already used by the firewall subset, `netns_qemu.py:107`) the daemon is dropped and its child probe loses nft. Drives D-1 |
| A-8 | The runner's design intends the foreground driver, not the daemon's child, to read kernel state | new at DESIGN | D-1's rationale weakens to preference | read the runner's privilege-drop comment | **CONFIRMED**. `runner_exec.go:740-744`: "driver.py stay privileged (root under sudo) so they can read nft state and signal the daemon", while ze is the process dropped |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Problem B is blocked by the firewall concurrency deadlock: the FORWARD proof needs nft + flood, the exact combination that hung dispatch (`ddos-direction.ci:11-16`) | the new transit test hangs like the original observation | **DOWNGRADED 2026-07-16, still live.** The recorded trigger needs two concurrent nft drivers (firewall component + responder); neither test declares a `firewall {}` block, so the component stays idle (`engine.go:299-303`) and autoload gives the responder a backend as sole driver (`registry.go:104-109`). B need not wait on the deadlock spec. But the deadlock's root cause is UNVERIFIED, so this rests on the recorded trigger: if either test hangs, STOP and report. Do not add a sleep, do not relax the assertion |
| R-2 | Rework proceeds on the wrong premise (A-1) and rewrites a test that only needed running | the reworked test fails the same way the original would have | **MATERIALIZED at DESIGN, caught before code.** A-1 is broken and A-2 is broken as stated; the skeleton's AC-2 (mandating the in-daemon probe) is built on both. Phase 1 stands: run the original under QEMU FIRST. D-1 must be settled by the user before any rewrite |
| R-7 | Autoload (`c5273da42`, 2026-07-13) silently changed EXISTING ddos tests: any `response-level enforce` test that installs a drop now programs nft where before 2026-07-13 nothing loaded a backend. `ddos-direction.ci` is in that set | an existing green ddos test starts hanging or flaking under the concurrency spec's work | Out of scope here (concurrency spec's territory) but flagged: do not assume the three "working" ddos tests are still green on the same mechanism they passed on. Confirm the baseline with a QEMU run at Phase 1 before attributing any failure to this spec's changes |
| R-8 | The two `.ci` files drift apart: Problem A's local-victim test and Problem B's transit test share a harness (setup.py + driver.py + nft readback) but are separate files | duplicated setup logic diverges | Build A first, then derive B from it. Keep the nft-readback helper shape identical; if a third copy appears, hoist to `test/scripts/` rather than fork |
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
| AC-1 | Research on Problem A complete | The real reason `ddos-detect-mitigate.ci` does not pass is established by running it under QEMU and observing the failure, not inferred. A-1 resolved either way. **Partially met at DESIGN:** A-1 is BROKEN on static evidence (F1-F5) and a second dead cause was found (no nft backend before `c5273da42`, F6/F8). The QEMU run stays BLOCKING because the header's three runtime behaviors (`:14-22`) are still unproven for an external flooder (A-3) |
| ~~AC-2~~ | ~~`test/plugin/ddos-detect-mitigate.ci` after rework~~ | ~~Runs green under QEMU and uses the in-daemon `ze_api` observer-probe pattern; no `daemon.pid` / `daemon.ready` polling remains~~ **STRUCK -- SUPERSEDED by AC-2a. -> Decision (user, 2026-07-16): D-1 approved, so this AC is dead and is NOT to be reinstated.** Reason: the AC mandates a pattern chosen to escape a handshake that is not dead (A-1 BROKEN), and the in-daemon probe cannot assert the hook through any dispatch surface (A-2 BROKEN, F9: `show.go:23-37` returns enabled/active/target only) while root-execing `nft` from a daemon child works only by suite accident (A-7, false under netns mode). Struck rather than deleted per `ai/rules/planning.md` (append-only) |
| AC-2a | `test/plugin/ddos-detect-mitigate.ci` after rework | **APPROVED 2026-07-16 (D-1); replaces AC-2.** Runs green under QEMU. **Keeps** the `daemon.pid`/`daemon.ready` handshake and the root `driver.py`, and every hand-rolled `time.sleep` poll loop (`:39-47`, `:74-75`) is replaced by `ze_api.wait_until`, matching `test/vrrp/vrrp-instance-up.ci:123`. Net `.ci` sleep count decreases (F15). The nft readback stays in `driver.py`, which the runner keeps privileged for exactly this (`runner_exec.go:740-744`) |
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
| `ddos-detect-mitigate.ci` | `test/plugin/` | ~~reworked onto the `ze_api` probe~~ (D-1) driver.py + `wait_until`; local victim flood -> INPUT drop installed, asserted by nft readback + `hook=ingress` log (needs-linux) | rework |
| `ddos-transit-forward-drop.ci` | `test/plugin/` | veth transit topology; remote victim flood + forward-mitigation on -> FORWARD drop; off -> no drop (needs-linux) | new |
| Baseline re-run (R-7) | `test/plugin/` | `ddos-direction.ci` / `ddos-policy.ci` / `ddos-bps-amplification.ci` still green now that autoload programs nft under enforce mode | verify at Phase 1 |

### QEMU Evidence
| Check | Command | Status |
|-------|---------|--------|
| Both tests green on the QEMU Alpine VM | `bin/ze-test plugin --pattern ddos` | |

## Files to Modify
- `test/plugin/ddos-detect-mitigate.ci` - ~~rework onto the in-daemon `ze_api` probe pattern~~
  **superseded by D-1:** keep the handshake, migrate the poll loops to `ze_api.wait_until`, add
  the veth flood device if AC-1's run shows the loopback trigger does not fire from an external
  driver (the header anticipates this at `:21-22`)
- `plan/deferrals.md` - resolve rows `:54` and `:57` at closure. **Record BOTH as stale-on-
  arrival, not as wrong diagnoses:** each was accurate on 2026-07-12 and was invalidated by a
  2026-07-13 commit (`dc082c288`, `c5273da42`). NOTE: a concurrent spec is reworking how this
  file works; coordinate on format before editing
- ~~`internal/test/runner/runner_exec.go`~~ - **DROPPED (D-3).** Research proves NO harness gap:
  the handshake is armed (F1), the daemon writes the file (F2), a foreground setup script is
  awaited (F13), and veth needs no directive (A-4 CONFIRMED). The spec's "only if research
  proves a gap" condition is not met, so the runner is NOT touched
- ~~`docs/architecture/testing/ci-format.md`~~ - **DROPPED:** no new directive is added (D-3)

## Files to Create
- `test/plugin/ddos-transit-forward-drop.ci` - the AC-10 transit proof
- ~~the probe `.run` scripts embedded in those `.ci` files (tmpfs blocks, per the pattern)~~
  **CORRECTED (2026-07-17, per D-1):** not in-daemon probe `.run` scripts. Under D-1 the tests
  keep the `driver.py` model, so the embedded scripts are `tmpfs=` blocks: `driver.py` (flood +
  root `nft list ruleset` readback) in both `.ci` files, plus `setup.py` (veth pair + forwarding,
  `cmd=foreground:seq=1`) in `ddos-transit-forward-drop.ci`. Pattern of record:
  `test/vrrp/vrrp-instance-up.ci` (`tmpfs=driver.py` + setup, not an external-plugin `.run`)

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

-> ~~Constraint: Phase 0 gates everything. D-1 contradicts AC-2 as written, and implementing
either shape before the user rules would burn the work.~~
**SUPERSEDED: Phase 0 is COMPLETE. -> Decision (user, 2026-07-16): D-1 approved** (keep
driver.py, migrate the sleep loops to `ze_api.wait_until`, do NOT port to the in-daemon
probe). AC-2 is struck, AC-2a governs, and the shape question is settled. Phase 1 (the
empirical QEMU run) is still BLOCKING and still runs first: it establishes WHY the test is
red, which the decision does not answer.

0. ~~**Phase: User decision on D-1 (BLOCKING).** Present the A-1/A-2 evidence and the
   driver.py-vs-probe choice. Until it is settled, write no test code. Phase 1 may run in
   parallel: it is read-only observation and its result informs D-1.~~
   **DONE 2026-07-16: D-1 approved by Thomas.** No longer gates anything.
1. **Phase: Resolve A-1 empirically (BLOCKING, do this first)** - run `ddos-detect-mitigate.ci`
   as-is under QEMU (`bin/ze-test plugin --pattern ddos-detect-mitigate`). Capture the actual
   failure. Static evidence says both historical blockers are gone (F1-F8), so the live
   outcomes are: (a) it passes -> Problem A collapses to the `wait_until` migration (AC-2a) and
   the header's STATUS block; (b) it fails on the loopback trigger -> switch the flood to the
   veth from Phase 3, as the header anticipates (`:21-22`); (c) it hangs -> R-1/R-7 apply, STOP
   and report. Also capture a baseline run of the three "working" ddos tests (R-7): confirm they
   are still green now that autoload programs nft under them.
   Do not rewrite the test before this.
2. **Phase: Problem A (shape decided by Phase 0 + Phase 1)** - migrate the poll loops to
   `ze_api.wait_until`; keep both original assertions (AC-3): populated victim DstPrefix AND a
   real nft drop. Refresh the header's STATUS block, which currently claims the runtime path is
   unconfirmed. Green under QEMU.
3. **Phase: Transit topology** - `setup.py` at `cmd=foreground:seq=1` builds the veth pair and
   enables forwarding (pattern: `vrrp-instance-up.ci:34-51`); the victim must resolve as
   RTN_UNICAST via the box, not RTN_LOCAL, or the direction classifies local and the test proves
   nothing. Prove remote-victim classification reaches the responder (`show ddos incidents`
   direction=remote) BEFORE asserting any hook.
4. **Phase: Problem B proof** - `forward-mitigation` on: nft drop for the victim on the FORWARD
   hook (chain name `forward`, `responder.go:170-175`), corroborated by the `hook=forward` log
   (D-4). Off: no on-host drop; assert the flowspec-deferral log (`responder.go:115-117`). If
   this hangs, STOP: R-1 applies, report to the user, do not work around it.
5. **Phase: Close** - resolve `plan/deferrals.md:54,57` as stale-on-arrival; both tests in the
   QEMU gate (AC-6).

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
| A-1: the `daemon.pid`/`daemon.ready` handshake is dead, so the test must be reworked onto the in-daemon probe | The handshake is fully alive: armed (`runner_exec_util.go:125-130`, `runner_exec.go:702-705,777-791`), written by the daemon (`cmd/ze/hub/main.go:957-961`), env key resolves (`env.go:41-43`). It landed in `dc082c288` on 2026-07-13, ONE DAY after the 2026-07-12 deferral, and `test/vrrp/vrrp-instance-up.ci:123` polls it green today | DESIGN 2026-07-16: traced the producer chain end to end, then `git log -S zeReadyFileEnabled` dated the fix against the deferral | The rework premise collapses. AC-2 struck, AC-2a proposed (D-1), pending user approval |
| The test's `nft list ruleset` grep only needed the daemon to start | It ALSO needed an nft backend, and the config declares no `firewall {}` block. Before `c5273da42` (2026-07-13) nothing would have loaded one, so the grep could never have matched. `ApplyAll` now autoloads the OS default for plugin-owned tables (`registry.go:104-109`) | DESIGN 2026-07-16: read `engine.go:299-303` (idle without a section) then `ApplyAll` | The test was dead for TWO independent reasons, both fixed on 2026-07-13. Neither the deferral nor the skeleton knew about the second |
| The `ze_api` probe pattern is the harness of record for these assertions | It is the harness of record for DISPATCH-COMMAND assertions. No probe test reads nft (F14), no dispatch surface carries the hook (F9), and the runner's design reserves the privileged foreground driver for kernel reads (F12). The pattern of record for kernel-state assertions is `vrrp-instance-up.ci`: setup.py + background ze + driver.py + `wait_until` | DESIGN 2026-07-16: read all three "working" ddos tests, `show.go`, and the runner's privilege comment | Drives D-1. "The pattern the other tests use" was true but load-bearingly incomplete: those tests never assert kernel state |

### Class: stale deferrals

Both rows in this spec's Origin were ACCURATE when written and falsified one day later by
unrelated commits. Neither was a diagnostic error. The generalizable lesson: **a deferral row
records the tree as it was on its date; re-verify its premise against the current tree before
acting on it, and date the evidence.** This spec's skeleton caught the first (A-1) by reading
the runner; it missed the second (the nft backend) because it never asked why a test with no
`firewall {}` block expected an nft rule. Worth a `RECURRING-PATTERNS.md` entry at closure.

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
  -> Decision (DESIGN): fallback only. A synthetic event bypasses detection and classification,
     leaving only `hookForDirection` under test, which `TestLocalHookByDirection` already covers
     exhaustively. Parent AC-10 asks for an end-to-end functional proof. If the real flood
     proves undrivable, raise the substitution with the user; do not swap silently.
- **The inversion the skeleton found is real but points the other way.** The skeleton read the
  process topology (probe in-daemon + foreground `ze` vs background `ze` + foreground driver.py)
  as "the substance of the rework". The privilege topology is what actually decides it: the
  runner DROPS the daemon and KEEPS the foreground driver privileged, precisely so the driver
  can read nft state (`runner_exec.go:740-744`). A test whose assertion is a kernel read
  therefore belongs in the driver, not in a daemon child. The in-daemon probe suits tests whose
  assertions come back over the engine socket, which is exactly what the three working ddos
  tests assert and why they never touch nft (F14).
- **`show ddos local` is one leaf short of making the probe viable.** If it reported the chain
  hook alongside `active`, the whole nft/root problem would dissolve and AC-4/AC-5 would be
  pure dispatch assertions. That is a genuinely attractive product change (it is also operator-
  useful: "which hook is my drop on?"), but it is a product change to make a test pass (R-4) and
  must stand on its own merits with user approval. Recorded as the rejected alternative in D-1.
- **`test/pppoe/` is orphaned dead code** (found while validating A-4, out of scope here). Its
  three `.ci` files use `option=netns:veth=...`, which `parseOption` rejects with `unknown option
  type` (`record_parse.go:287-430`), so they cannot even parse. Nothing runs them: `registerCIRoot`
  (`internal/test/cli/register.go:17-36`) lists 20 suites and pppoe is not among them, and
  `scripts/evidence/qemu-all-tests.sh:124-155` enumerates suites explicitly without it. The
  invalid syntax has never been caught because no gate reaches the files. Worth its own fixit
  spec (delete them, or wire the suite and fix the directive); flagged, not fixed here.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| ~~**D-1 (BLOCKED ON USER)**~~ **D-1 APPROVED -> Decision (user, 2026-07-16): keep the `driver.py` + `daemon.ready` pattern; do NOT port to the in-daemon `ze_api` probe.** Migrate its hand-rolled sleep loops (`ddos-detect-mitigate.ci:39-47,74-75`) to `ze_api.wait_until`, exactly as `test/vrrp/vrrp-instance-up.ci:123` does. No longer blocked; Phase 0 is satisfied. | (a) ~~Port to the in-daemon probe, as the deferral row and current AC-2 direct~~ **REJECTED.** (b) ~~Keep driver.py unchanged~~ **REJECTED** (leaves the ad-hoc poll loops). (c) ~~Add a `hook` field to `show ddos local` so the probe can assert it~~ **REJECTED -- see the rejected-alternative note below; it is a product change to make a test pass.** | The deferral chose the probe to escape a handshake it believed dead. It is not dead (A-1 BROKEN, F1-F5), so the reason is gone. Positively, and **re-verified at the producers on 2026-07-16**: the assertion REQUIRES reading the nft ruleset, because no dispatch surface carries the hook -- `handleShowDdosLocal` (`internal/plugins/ddos/local/show.go:23-37`) returns only `enabled`, `active` and (when active) `target`. Reading nft state needs root, and the runner **deliberately** drops the daemon's privileges while keeping `driver.py` privileged for exactly this purpose: `internal/test/runner/runner_exec.go:740-744` reads "drop the ze daemon to a normal user so its readiness handshake works ... **ze-peer / driver.py stay privileged (root under sudo) so they can read nft state and signal the daemon**", and the UID drop at `:745-749` is applied only when `binName == "ze"`. The in-daemon probe can exec `nft` today only because the QEMU plugin suite happens to leave ze root (A-7, F11) -- a **suite accident, not a property of the pattern, and false under netns mode**. So (a) buys nothing and couples the test to that accident. **AC-2 is struck and replaced by AC-2a.** |
| **D-2: no `firewall {}` block in either test.** Let `ApplyAll` autoload nft. | Declare `firewall { backend nft }` like the original observation did. | Autoload (F6) gives a real kernel backend with the responder as sole nft driver, avoiding the two-driver combination behind R-1 (F7). A block would add nothing and re-create the hang. |
| **D-3: no runner change.** | Add a veth/topology directive to the `.ci` format. | A-4 CONFIRMED: `setup.py` at `cmd=foreground:seq=1` is awaited (F13) and already builds veths in `vrrp-instance-up.ci`. The spec's conditional runner row is therefore dropped from Files to Modify. |
| **D-4: assert kernel state AND the responder log, not either alone.** nft readback for `ddos-local` + victim + the chain's hook; plus `expect=stderr:contains=ddos-local: drop rule installed` (which carries `hook=forward` / `hook=ingress`, F10). | nft only; log only. | AC-3 forbids weakening to a log-grep, so nft is primary. The log is a cheap corroborator that pins WHICH hook the responder chose, and localizes failures (intent vs kernel). |
| **D-5: keep both problems in one spec** (A-6 CONFIRMED). | Split A and B. | The split rationale was "B is blocked, A is not". R-1 is downgraded, so both are unblocked, and they now share one harness. Splitting would duplicate setup.py/driver.py scaffolding. |

### The rejected alternative (D-1 option c), recorded so it is not silently revived

-> Decision (user, 2026-07-16): **adding a `hook` field to `show ddos local` is REJECTED as a
means of satisfying this spec** -- it is a **product change made to make a test pass**
(R-4), and this spec is test-infrastructure only. The tail must not wag the dog: the test
adapts to the product's existing surface (a root `driver.py` nft readback), not the reverse.

-> Constraint: the rejection is **scoped to this spec's motivation, not to the idea itself.**
Thomas noted the field is **independently worth considering on operator-usefulness grounds**:
"which hook is my drop on?" is a reasonable thing for an operator to ask, and
`handleShowDdosLocal` (`show.go:23-37`) currently cannot answer it. If it is pursued, it must
stand on its own merits in its own spec, justified by operator need and approved on that
basis -- never as a test-enablement change. If such a spec ever lands, this spec's nft
readback stays: it proves KERNEL state, whereas a dispatch field would prove only what the
responder believes (the same intent-vs-kernel distinction D-4 rests on).

## Known Limitations
- ~~Problem B may be gated on `plan/spec-fixit-firewall-concurrency-deadlock.md` (R-1). If so,
  this spec cannot close until that one does, or the two problems split.~~
  **REVISED 2026-07-16:** on the recorded trigger (two concurrent nft drivers), neither test is
  gated: neither declares a `firewall {}` block, so the firewall component idles
  (`engine.go:299-303`) and autoload gives the responder a backend as sole driver
  (`registry.go:104-109`). The deadlock's root cause is UNVERIFIED, so this is a downgrade, not
  a clearance. R-1 stays live: if either test hangs, STOP and report.
- The FORWARD proof asserts the chain's HOOK, not that forwarded packets are actually dropped in
  flight. Proving the latter needs a receiver on the far veth end counting arrivals. Out of scope
  unless the user asks; the parent AC-10 wording asks for the drop to be *installed* on the
  FORWARD hook.
- The nft readback couples these two tests to the QEMU plugin suite running ze as root (F11). If
  that suite ever adopts netns mode (as the firewall subset has, `netns_qemu.py:107`), driver.py
  keeps working by design (F12) but the daemon-side assertions would need review. This is an
  argument for D-1, not a defect.

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
- [ ] D-1 approved by the user before any test code is written (AC-2 is struck pending it)
- [ ] AC-1, AC-2a, AC-3..AC-9 all demonstrated
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

**Answered during DESIGN (2026-07-16):**

| Question | Answer |
|----------|--------|
| Was the runner's background-mode readiness arming added after 2026-07-12? | **YES.** `dc082c288`, 2026-07-13, one day after the deferral. Its message states background ze "timed out by construction" before it. The deferral was accurate when written and is now stale (F4). |
| Can the `ze_api` probe inspect the nft ruleset under QEMU, or must the assertion go through a dispatch-command surface (A-2)? | **Neither escape works as the skeleton hoped.** There is NO dispatch surface for the hook (`show ddos local` = enabled/active/target, F9), so a ruleset read is mandatory. The probe *can* exec `nft` today, but only because the QEMU plugin suite leaves ze root (F11/A-7) -- untrue under netns mode. Drives D-1. |
| Does the `.ci` format support creating a veth pair before the daemon starts (A-4)? | **YES, with no new directive.** `tmpfs=setup.py` + `cmd=foreground:seq=1`, awaited by `runner_exec.go:797-808`; precedent `vrrp-instance-up.ci:34-51,212`. Do NOT use `option=netns:veth=` (invalid syntax, `record_parse.go:428-430`). |
| Is Problem B blocked on the firewall concurrency deadlock (R-1)? | **No, on the recorded trigger.** It needs two concurrent nft drivers; neither test declares a `firewall {}` block, so the component idles (`engine.go:299-303`) and autoload gives the responder a backend as sole driver (`registry.go:104-109`). Root cause is UNVERIFIED, so R-1 stays live as a stop-and-report risk. |
| Should the two problems stay in one spec (A-6)? | **One spec.** The split rationale ("B blocked, A not") is void, and both now share one harness. |

**Still open (need the QEMU run ~~or the user~~):**

-> READINESS RECONCILIATION (2026-07-17): the ONLY user item (D-1) is resolved. Everything left
   below is implementation-phase execution (the QEMU run) or already-defaulted, so NONE of it
   gates a fresh implementer picking this spec up. Item-by-item resolution follows each bullet.

- ~~**[USER, BLOCKING] D-1: driver.py or in-daemon probe?**~~ **RESOLVED -- D-1 APPROVED by
  Thomas 2026-07-16.** The evidence says keep driver.py and
  migrate its poll loops to `ze_api.wait_until`. This contradicts AC-2 as written, which is why
  AC-2 is struck and AC-2a is proposed. ~~Only the user can approve the scope change.~~
  -> The user approved: keep `driver.py`, migrate the poll loops to `ze_api.wait_until`, do NOT
     port to the in-daemon probe. AC-2a governs. No longer open.
- **Does `ddos-detect-mitigate.ci` actually fail today, and how?** Static evidence says both
  historical blockers are gone. Nobody has run it. AC-1 keeps this BLOCKING before any rewrite.
  -> READINESS NOTE (2026-07-17): NOT a design/readiness blocker. This is an empirical runtime
     fact resolvable only by executing the test under QEMU, which is exactly AC-1 and the FIRST
     implementation action (Phase 1, `bin/ze-test plugin --pattern ddos-detect-mitigate`). A
     fresh implementer opens the work with this run; it cannot and should not be resolved by
     reading source, so it does not gate `ready`.
- Can `internal/test/plugins/fakeddos/` drive a synthetic remote-victim AttackDetected, making
  the FORWARD proof deterministic without a real transit flood? Would that still satisfy the
  parent AC-10, which asks for a functional proof? **Design lean: no.** A synthetic event would
  bypass detection/classification and prove only `hookForDirection`, which
  `TestLocalHookByDirection` already covers exhaustively. AC-10 asks for the end-to-end proof.
  Keep fakeddos as a Phase 3 fallback only if the real transit flood proves undrivable, and
  raise it with the user rather than silently substituting a weaker proof.
  -> AUTONOMOUS DEFAULT (2026-07-17): adopt the design lean. Use the REAL transit flood for
     AC-4/AC-5; `fakeddos` is a Phase-3 fallback used only if the real flood proves undrivable,
     and only after raising the substitution with Thomas. Rationale: parent AC-10 asks for an
     end-to-end functional proof; a synthetic event proves only `hookForDirection`, already
     covered exhaustively by `TestLocalHookByDirection`, so it is the smaller-scope but weaker
     option and is not the default. Thomas: override if wrong.
- **[COORDINATION] R-7:** autoload now programs nft under existing enforce-mode ddos tests
  (`ddos-direction.ci`). Whether that destabilizes them belongs to
  `plan/spec-fixit-firewall-concurrency-deadlock.md`. Flagged, not resolved here.
  -> READINESS NOTE (2026-07-17): out of scope for this spec and correctly owned by
     `plan/spec-fixit-firewall-concurrency-deadlock.md`. Not a readiness blocker; it is captured
     here as R-7 and as the Phase-1 baseline re-run row in `### Functional Tests`. No open
     decision for this spec.

## Notes
- Authored 2026-07-15 as a skeleton from `plan/deferrals.md:54` and `:57`. Every `file:line`
  here was read at authoring time. Two claims from the deferral rows did not survive
  verification and are recorded in the Mistake Log; the most important is A-1, which
  inverts the starting move for Problem A from "rewrite it" to "run it and see".
