# Spec: netlink-ci-harness

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | Fix C + Fix A + Fix B DONE — netns launch mode validated host-safe in QEMU; nft-rendering brittleness fixed; pre-existing non-netns reds logged for triage |
| Updated | 2026-07-13 |

## Progress

- **Fix C (ze_api EOF busy-spin): DONE.** `test/scripts/ze_api.py` `_read_tls_line`/`_read_line`
  now set `self._shutdown = True` on EOF/closed-connection (recv/read → `b""` or OSError/SSLError).
  TDD: `test/plugin/plugin-eof-no-spin.ci` (socketpair FD-mode, peer closed) — RED before
  (`iterations=101, elapsed=0.00s` busy-spin), GREEN after. Regression: `bin/ze-test bgp reload
  --all` = 33/36; the 3 fails are pre-existing (`commit-transactional`, identical
  `meta/config/rollback` error in the original run) or need CAP_NET_ADMIN (iface tunnel/wireguard),
  none caused by Fix C. Runs on all platforms (no caps needed) → guards the regression in `ze-verify`.
- **Fix A (background-ze readiness): DONE.** Runner now arms the readiness
  handshake for `cmd=background` ze daemons, not just foreground.
  → Decision: new pure helper `zeReadyFileEnabled(mode, binName, tmpfsTempDir)`
    (`runner_exec_util.go`) gates BOTH the `ZE_READY_FILE` env (`runner_exec.go`
    ~:628) and the `daemon.pid` write; env-arming and pid-write stay in lockstep.
  → Decision: the `cmd=="background"` case gained a `case zeReadyFileEnabled(...)`
    branch mirroring the foreground `default:` path — `!hasPeer` → `waitReady` on
    `daemon.ready`, then write `daemon.pid`. ze-peer / helper-script background
    procs keep their existing behavior (peer WaitFor / 100ms startup sleep).
  → Constraint (sub-fix required for AC-1): the foreground `default:` branch used
    to write `daemon.pid` for ANY foreground last-command, so a `driver.py`
    (foreground, last) clobbered `daemon.pid` with its OWN pid and killed itself
    (mysterious exit -1). Guarded that write on `zeReadyFileEnabled` (ze-only) too.
    This latent bug never surfaced before because no driver.py test ever got past
    the previously-missing readiness gate. Not scope creep: AC-1 ("driver.py-style
    tests proceed") cannot pass without the correct daemon pid.
  TDD: RED→GREEN via `test/reload/background-ze-readiness.ci` (background ze +
  driver.py polling daemon.pid/daemon.ready, no caps/skip-os — guards Fix A on
  every platform) + unit `TestBackgroundZeGetsReadinessEnv` (7 subcases).
  Regression (rebuilt bin/ze-test): reload 25/25 (0 fail, 12 cap/os skips),
  managed 13/13, l2tp 17/17, ipsec 6/6. `make ze-lint-changed` = 0 issues.
  Doc: `docs/architecture/testing/ci-format.md` gained a "Daemon readiness"
  note (foreground+background, ze-only).
- **Newly found (separate, pre-existing):** `test/reload/commit-transactional.ci`
  was reported failing on a missing `meta/config/rollback` file in the 2026-07-12
  release run; in the 2026-07-13 full `bgp reload --all -p 1` run it PASSED on this
  host. Left as an observation for triage (env-dependent), not in this spec's scope.
- **Newly found (separate, pre-existing):** `test/static/001-boot-apply.ci` (no
  skip-os, runs on macOS) is RED for an INDEPENDENT reason: `parse config: line 4:
  unknown field in route: next-hop` (stale static route config syntax) — the daemon
  dies before readiness, so Fix A alone will not green it. Log for triage.
- **Fix B (per-test netns + setcap): DONE, validated host-safe in QEMU.**
  Opt-in `ZE_TEST_NETNS` launch mode: the runner locks the per-test goroutine's
  thread into a fresh named netns before spawning, and execs `ze` as a normal
  user (`SysProcAttr.Credential` from `ZE_TEST_UID`) off a setcap'd binary, so the
  daemon programs nft inside a throwaway netns as a non-root process with ambient
  caps. `ze`/`ze-peer`/driver.py fork-inherit the netns (A-5).
  → Files: `internal/test/runner/netns_linux.go` (`enterTestNetns`, `testNetnsName`,
    `bringLoopbackUp`) + `netns_other.go` stub; `runner_exec.go` `runOrchestrated`
    (netns entry + `chownTree` of the tmpfs/config dir + `SysProcAttr` credential +
    `ZE_TEST_NETNS_HOST` env); `runner_exec_util.go` (`netnsModeActive`,
    `netnsChildIDs`, `chownTree`; `startWithETXTBSYRetry` preserves `SysProcAttr`).
  → R-2 gate: `internal/plugins/firewall/nft/host_netns_guard_linux.go`
    (`refuseHostNetnsFirewall`) called first in `(*backend).Apply` — refuses nft if
    `ZE_TEST_NETNS_HOST` is set but the process is in the host netns (fails closed).
    Env is test-only → production unaffected.
  → Constraint (sub-fix for AC-2): the tmpfs-less config path (`ze-config-*`,
    0700-root) must be chowned to the dropped uid too, else offline `ze config
    validate -` gets EACCES. Fixed (006-dscp-ipv6-rejected went red→green).
  → Make targets: `mk/test-integration.mk` `ze-netns-test` (real Linux host: setcap
    + `sudo ZE_TEST_NETNS=1` per suite, host-nft-unchanged assertion) and
    `ze-netns-qemu-test` (AC-7, macOS via QEMU). Docs: `docs/functional-tests.md`
    "Netns launch mode for netlink .ci suites".
  TDD: `TestNetnsLaunchChildInheritsNamespace` (A-5, netns_linux_test.go) and
  `TestRefuseHostNetnsFirewall` (R-2, host_netns_guard_linux_test.go) — both GREEN
  in QEMU. E2e (`make ze-netns-qemu-test`): host-safe firewall subset **15/15 PASS**
  (001,002,003,005,006,007,008,010-016,command-owner), host `nft list tables`
  byte-identical before/after. `make ze-lint-changed` = 0 issues.
  → A-4 re-confirmed (A-5 already): a `ze` run as ROOT reaches the firewall Apply but
    the readiness file (written after dropPrivileges) never appears → dropping to a
    normal user is required. → A-5 validated: child `/proc/self/ns/net` differs from
    host (`net:[4026531840]` vs `net:[4026532218]`).
- **nft-rendering brittleness FIXED (per user scope decision).** The `.ci` expect
  strings were authored against an older nft; Alpine nft 1.1.1 renders the same rules
  differently. Made version-robust with `expect=stdout:pattern=` alternations
  (confirmed 4/4 green): `001` `(tcp|th) dport 22 accept`, `005`
  `(ip saddr|@nh,96,32) @blocked drop`, `007` `(tcp|th) dport 5060` +
  `(ip dscp set|@nh,8,8 set)`, `012` `icmp type (8|echo-request)` /
  `icmpv6 type (128|echo-request)`, `copp-bgp` `(tcp|th) dport 179`.
- **Newly found — pre-existing, NOT Fix B (orthogonal; logged for triage):**
  These fail the SAME with `ze` as root in the host netns (no netns/uid-drop), so the
  netns launch mode does not cause them:
  - `test/firewall/004-cli-show.ci`: `ze cli` opens a `zefs` database, but the test
    daemon runs `ze.storage.blob=false` → no `database.zefs`. cli↔storage interaction.
  - `test/firewall/009-set-element-timeout.ci`: crashes the Alpine QEMU kernel on nft
    set-element-timeout (already skipped by `ZE_QEMU_SKIP_SUITES`).
  - `test/firewall/copp-{bgp,trusted,withdraw}.ci`: `copp apply: firewall backend not
    loaded` — the copp config has no `firewall { backend nft }` block, so the nft
    backend is never loaded (plugin-wiring, confirmed by reading the config).
  - `test/policy/*.ci`: `firewallnft: flush: netlink receive: operation not supported`
    — Alpine QEMU kernel lacks an nft feature `policy-routes` programs. Reproduces
    identically with `ze` as root, no netns (attribution run B/C). A full-kernel Linux
    host is expected to pass. `ospf --all` (97 tests) is mostly offline config/doctor
    tests and is the wrong granularity for suite-wide netns mode.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/test/runner/runner_exec.go` (esp. `:576` exec sites, `:628-631` ZE_READY_FILE, `:673-727` mode switch, `:714-726` daemon.pid)
3. `test/scripts/ze_api.py` (`:200-224` `_read_tls_line`, `:264-303` `_read_line`, `:1010-1069` `read_line`, `:389` `_shutdown`)
4. `test/firewall/001-boot-apply.ci` (background-ze + driver.py readiness pattern)
5. Integration netns pattern: `internal/component/iface/integration_helpers_linux_test.go:36-69` (`withNetNS`), vendored `github.com/vishvananda/netns`
6. Evidence + digests: `tmp/rel-evidence/` and `tmp/session/session-state-4113.md`

## Task

Make the Linux-only netlink `.ci` functional suites (firewall/policy/ospf/ospfv3, and any
suite whose `ze` daemon needs `CAP_NET_ADMIN` for nft/FIB/iface) actually runnable and
passable on a Linux host, host-safely. These suites carry `option=skip-os:value=darwin`,
were therefore never exercised on macOS, and have never run on Linux, so three latent
defects surface the first time the release-evidence matrix runs on Linux:

- **Fix A (readiness gap):** 18/20 firewall tests (and policy/ospf peers) launch `ze` as
  `cmd=background` and poll `daemon.pid`+`daemon.ready`, but the runner only produces those
  for `cmd=foreground` `ze`. So they time out by construction.
- **Fix B (isolation + capabilities):** these suites need `CAP_NET_ADMIN`; run unprivileged
  the nft firewall plugin hard-fails and takes the daemon down; run with caps in the host
  netns they program the real host firewall (this locked the operator out mid-session).
  They must run with ambient caps AND per-test network-namespace isolation.
- **Fix C (hang landmine):** the python plugin SDK busy-spins a core on connection EOF and
  can deadlock `ze-verify` for hours when a daemon dies without a graceful `bye`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/qemu-integration.md` — how linux-only netlink code is
  isolated/tested today (QEMU + integration-tagged Go tests; `option=needs-linux`)
  → Constraint: linux-only paths must have QEMU coverage (`ai/rules/qemu-testing.md`); the
    new runner netns path is linux-only and must be exercised under QEMU too.
- [ ] `docs/functional-tests.md` — `.ci` runner behavior and options; this is test infra and
  must be updated (Documentation Update Checklist #10).
  → Constraint: document the new privileged/netns run mode and the make target.

**Key insights:**
- The runner's per-test unit is a goroutine (20-way pool, `parallel.go:191,221`); `ze` and
  `ze-peer` are separate child processes (`exec.CommandContext` at `runner_exec.go:126/210/576`).
  Both children of one test must land in the SAME netns (they talk over `127.0.0.1:rec.Port`).
- Go `exec` has no setns hook. The integration tests enter a netns by `runtime.LockOSThread`
  + `netns.Set` on the test goroutine; children fork-inherit the thread's netns. That model
  is reusable here: lock the test goroutine's thread, enter a fresh per-test netns, exec both
  children (they inherit it), restore + delete on cleanup.
- Creating a netns needs `CAP_SYS_ADMIN`; running nft needs `CAP_NET_ADMIN`; the readiness
  handshake breaks when `ze` runs as root. Resolution: the runner runs privileged (sudo /
  cap_sys_admin), creates the per-test netns, and execs `ze` as a NORMAL user (SysProcAttr
  Credential) using a `setcap cap_net_admin,cap_net_raw,cap_net_bind_service+ep` binary so
  the daemon has ambient caps without being root.
- `Record.RunCommands []RunCommand` (`record.go:157`); `RunCommand.Mode` is "background"/
  "foreground" (`record.go:194-200`). `Record.TmpfsTempDir` (`:142`), `Port` (`:89`),
  `EnvVars` (`:125`). `NeedsLinux`/`option=needs-linux` (`:179-183`) already marks QEMU-only tests.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec.go` — orchestrated multi-process launch. `ZE_READY_FILE`
  set ONLY for `modeForeground && binName=="ze"` (`:628-631`). `daemon.pid` written ONLY in the
  `default:` (foreground daemon) case (`:714-726`). Background `ze` (`:674-691`) writes neither.
  No `SysProcAttr` anywhere in this file.
- [ ] `internal/test/runner/record.go` — `Record` + `RunCommand` structs (fields above).
- [ ] `cmd/ze/hub/main.go` — `ze` writes the readiness file (`os.Create(readyFile)`) from the
  `ze.ready.file` env only, after startup completes (`:930-934`) / after `dropPrivileges` in the
  orchestrator path (`:1094-1098`). ze does not otherwise emit daemon.pid.
- [ ] `test/scripts/ze_api.py` — `_read_line`/`_read_tls_line` return `None` on EOF exactly as on
  timeout (`:222-223`, `:300-301`) and never set `_shutdown` (only `bye` does, `:389`,`:1065`).
- [ ] `internal/component/iface/integration_helpers_linux_test.go:36-69` — `withNetNS` pattern
  (LockOSThread → NewNamed → Set → Cleanup Set(orig)+DeleteNamed+UnlockOSThread; Skip if no caps).
- [ ] `internal/core/privilege/{check_linux.go:17-64, drop_unix.go:38-43}` — cap bit table and
  `Setuid/Setgid`.

**Behavior to preserve:**
- Foreground-`ze` readiness path unchanged (existing passing tests: `reload-add-peer.ci:83`,
  `signal-stop.ci:64`, all `test/managed/*`, `test/reload/*` that use foreground ze).
- Non-netlink suites keep running in the host netns, unprivileged, exactly as today (no netns,
  no setcap) — the new path is opt-in per suite/run, never forced onto suites that pass today.
- `.ci` file format and existing `cmd=background`/`cmd=foreground` semantics for ze-peer and
  helper scripts unchanged.

**Behavior to change:**
- Runner emits `daemon.pid`+`daemon.ready` for background `ze` daemons too (Fix A).
- Runner gains an opt-in per-test network-namespace + normal-user-with-ambient-caps launch mode
  for netlink suites (Fix B).
- `ze_api.read_line` detects EOF, sets `_shutdown`, returns without spinning (Fix C).

## Data Flow (MANDATORY)

### Entry Point
- `make ze-<netlink-suite>-test` (new/updated target) invokes `bin/ze-test <suite> -a` under
  the privileged+netns run mode (env flag, e.g. `ZE_TEST_NETNS=1`), with `ze`/`ze-stripped`
  setcap'd. The whole invocation runs under `sudo` (established pattern, `mk/test-integration.mk:49`).

### Transformation Path
1. Runner reads `ZE_TEST_NETNS`; if set and on Linux, each per-test goroutine, before spawning
   children, `runtime.LockOSThread()` + creates/enters a fresh netns (`netns.New`/`NewNamed`),
   brings `lo` up in it.
2. Each child `exec.Cmd` gets `SysProcAttr{Credential: {Uid: NORMAL_UID, Gid: NORMAL_GID}}` so
   the setcap'd `ze` runs as a normal user (readiness handshake works) with ambient caps (nft works).
3. `ze` and `ze-peer` fork-inherit the goroutine-thread's netns → same namespace → they reach
   each other over `127.0.0.1:rec.Port`. nft tables the firewall programs live in the throwaway
   netns → host firewall untouched.
4. On test end: restore original netns, delete the per-test netns, UnlockOSThread.
5. Fix A: for background `ze`, the runner sets `ZE_READY_FILE` and, after `waitReady`, writes
   `daemon.pid` into `rec.TmpfsTempDir` (mirrors the foreground branch).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner goroutine ↔ per-test netns | LockOSThread + netns.Set, children fork-inherit | [x] (A-5 QEMU: child netns ≠ host) |
| Runner ↔ child `ze` privilege | SysProcAttr.Credential drops uid; setcap grants caps | [x] (QEMU: ze uid 1000 programs nft; firewall subset 15/15) |
| Host netns ↔ test netns | new netns per test; nft is netns-scoped | [x] (QEMU e2e: host nft byte-identical before/after) |

### Integration Points
- Reuse vendored `github.com/vishvananda/netns` (already used by integration tests).
- Reuse `internal/core/privilege` cap table for the setcap requirement doctor/preflight.

### Architectural Verification
- [ ] No bypassed layers (readiness fix mirrors existing foreground path; netns is additive)
- [ ] No unintended coupling (netns mode is opt-in via env; default path unchanged)
- [ ] No duplicated functionality (reuse vishvananda/netns, not a new impl)
- [x] Registration over hardcoding — N/A (no new registry surface; test-runner infra)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | With CAP_NET_ADMIN the nft firewall plugin applies cleanly | ran `sudo unshare -n bin/ze-test firewall 1` → `firewall config applied tables=1` | Fix B insufficient; deeper firewall issue | prototype run | confirmed |
| A-2 | Per-test `unshare -n` keeps host nft untouched | prototype: host `nft list tables` byte-identical before/after | host-safety broken; cannot run on shared box | prototype run | confirmed |
| A-3 | Background `ze` gets no daemon.pid/ready today | only writers are `runner_exec.go:628` (foreground) + `:724` (foreground); passing daemon.pid tests all use `cmd=foreground:exec=ze` | Fix A misplaced | grep + reading | confirmed |
| A-4 | Running `ze` as a normal user (not root) is required for the readiness handshake | root run reaches firewall Apply but the readiness file (written after dropPrivileges) never appears | may run ze as root in netns (simpler) | isolate root-vs-normaluser once Fix A lands | **confirmed** (QEMU: root ze never writes daemon.ready) |
| A-5 | Fork-inherited netns covers exec'd children when the goroutine thread is locked + Set | integration `withNetNS` relies on this for in-thread ops; child inheritance is standard clone() semantics | need explicit setns wrapper / nsenter | unit test spawning a child that reads /proc/self/ns/net | **confirmed** (`TestNetnsLaunchChildInheritsNamespace`: child `net:[…218]` ≠ host `net:[…840]`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | 20-way goroutine pool + per-thread netns interact badly (thread reuse leaks netns) | tests flake / see wrong netns | Force `LockOSThread` for the whole test lifetime; never `Set` back on a thread we don't `Unlock`; consider `-p 1` fallback for netns suites |
| R-2 | setcap binary accidentally run in host netns → repeat lockout | host nft gains `ze_*` table | Gate netns mode so a setcap'd run without a netns REFUSES to start; keep setcap off by default |
| R-3 | Fix A changes behavior for an existing background-ze test that does not want readiness files | an existing passing test breaks | Only add files (never remove); run full functional suite before/after |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ZE_TEST_NETNS=1 bin/ze-test firewall 1` (under sudo, setcap ze) | → | runner netns launch path | `test/firewall/001-boot-apply.ci` passes green + host nft unchanged |
| background `ze` in a `.ci` | → | runner Fix A readiness branch | a new non-netlink `.ci` asserting daemon.ready/daemon.pid appear for background ze |
| daemon killed without `bye` | → | `ze_api.read_line` EOF path | a `.ci` that SIGKILLs the daemon and asserts the python plugin exits <1s (no spin) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Background `ze` in a `.ci` with `TmpfsTempDir` set | runner writes `daemon.pid` and sets `ZE_READY_FILE`; `daemon.ready` appears; driver.py-style tests proceed |
| AC-2 | `ZE_TEST_NETNS=1`, setcap'd ze, `bin/ze-test firewall -a` under sudo | every firewall test runs in its own netns; suite passes; host `nft list tables` identical before/after |
| AC-3 | Same as AC-2 but for `policy`, `ospf`, `ospfv3` suites | suites pass in isolated netns, host untouched |
| AC-4 | Netlink suite run WITHOUT `ZE_TEST_NETNS` on a host without caps | tests skip or fail loudly with a clear "needs CAP_NET_ADMIN + netns mode" message (never program the host firewall) |
| AC-5 | A `ze` daemon hosting a python `ze_api` plugin is SIGKILLed | the plugin's `read_line` returns, `_shutdown` becomes true, the plugin process exits within ~1s (CPU stays near 0, no busy-spin) |
| AC-6 | Non-netlink suites (`encode`, `reload`, `managed`, ...) with default env | run exactly as before: host netns, unprivileged, no setcap, all still pass |
| AC-7 | QEMU integration run | the netns launch path is exercised under QEMU (linux-only coverage) |

## End-to-End User Stories (MANDATORY)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `make ze-firewall-test` on Linux | sudo → ze-test (netns mode) → per-test netns → setcap ze as normal user → nft in netns → green | `test/firewall/*.ci` all pass; host nft unchanged |
| 2 | a plugin daemon crashes mid-test | ze dies → socket EOF → `read_line` sets `_shutdown` → plugin exits → runner sees pipe EOF → gate does not hang | SIGKILL `.ci` + `ze-verify` completes |
| 3 | runs a non-netlink suite unprivileged | default path, no netns/caps | `make ze-reload-test` etc still green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBackgroundZeGetsReadinessEnv` | `internal/test/runner/runner_exec_test.go` | Fix A: background ze command gets `ZE_READY_FILE` + daemon.pid write path | |
| `TestNetnsLaunchChildInheritsNamespace` | `internal/test/runner/netns_linux_test.go` | Fix B: a child exec'd under the netns mode reports a different `/proc/self/ns/net` than the host | |
| `TestReadLineEOFSetsShutdown` (python) | `test/scripts/ze_api_selftest.py` or a `.ci` | Fix C: read_line on a closed fd returns None AND sets `_shutdown` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no new numeric config) | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `test/firewall/*.ci` (18) | `test/firewall/` | firewall suites pass under netns mode | |
| new `test/plugin/plugin-eof-no-spin.ci` | `test/plugin/` | SIGKILL daemon → python plugin exits promptly | |
| new `test/reload/background-ze-readiness.ci` | `test/reload/` | background ze produces daemon.ready/daemon.pid | |

### Interop Tests
N/A — no wire-protocol change. This is test-harness infrastructure.

## Files to Modify
- `internal/test/runner/runner_exec.go` — Fix A (background-ze readiness); Fix B (SysProcAttr netns + Credential at exec sites)
- `test/scripts/ze_api.py` — Fix C (EOF detection in `_read_line`/`_read_tls_line`; set `_shutdown`)
- `mk/test-functional.mk` / `mk/test-integration.mk` — netns-mode make target(s) for netlink suites; wire into release evidence
- `docs/functional-tests.md` — document the netns/privileged run mode
- `internal/core/diagnostic/codes.go` + owning doctor — optional: preflight that netns suites have setcap + caps

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` — new netns run mode + make target |
| 8 | Plugin SDK/protocol changed? | Yes | `test/scripts/ze_api.py` behavior note (EOF → shutdown); `docs/architecture/api/process-protocol.md` if it documents plugin shutdown |
| 12 | Internal architecture changed? | Yes | test-runner section of the relevant architecture/testing doc |
| others | — | No (verify with grep for source anchors on changed files) | — |

## Files to Create
- `internal/test/runner/netns_linux.go` (+ `netns_other.go` stub) — per-test netns enter/exit helper reusing vishvananda/netns
- `internal/test/runner/netns_linux_test.go` — child-inherits-netns unit test
- `test/plugin/plugin-eof-no-spin.ci` — Fix C functional test
- `test/reload/background-ze-readiness.ci` — Fix A functional test

## Implementation Steps

### Implementation Phases
1. **Phase: Fix C (ze_api EOF)** — smallest, self-contained, unblocks hang landmine.
   - Test: `test/plugin/plugin-eof-no-spin.ci` (SIGKILL daemon → plugin exits <1s). Write failing first.
   - Files: `test/scripts/ze_api.py` — distinguish EOF (`b""`) from timeout in `_read_line`/`_read_tls_line`; on EOF set `self._shutdown = True` and return None.
   - Verify: plugin exits promptly; no core spin.
2. **Phase: Fix A (background-ze readiness)** — runner change, testable without caps.
   - Test: `test/reload/background-ze-readiness.ci` + `runner_exec_test.go` unit. Write failing first.
   - Files: `runner_exec.go` — set `ZE_READY_FILE` and write `daemon.pid` for background ze (mirror foreground branch; guard on TmpfsTempDir; no-peer waitReady).
   - Verify: daemon.ready/daemon.pid appear for background ze; existing tests unaffected.
3. **Phase: Fix B netns skeleton (wiring)** — opt-in env flag, LockOSThread + enter/exit netns helper, SysProcAttr wiring at the 3 exec sites, lo up.
   - Test: `netns_linux_test.go` child-inherits-netns. Write failing first.
   - Files: `netns_linux.go`, `runner_exec.go`.
   - Verify: child sees a distinct netns; host untouched.
4. **Phase: Fix B end-to-end** — setcap requirement, Credential drop to normal user, make target, gate (R-2 refuse-in-host-netns).
   - Test: `test/firewall/*.ci` green under `ZE_TEST_NETNS=1` + host-nft-unchanged assertion.
   - Files: `mk/*.mk`, `runner_exec.go`.
5. **Phase: QEMU coverage** — exercise the netns path under `ze-qemu-integration-test`.
6. **Phase: Docs + discovery** — `docs/functional-tests.md`, rule/index updates, doctor preflight.
7. **Full verification + close** — `make ze-verify`; learned summary; two-commit closure.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC-N implemented with file:line |
| Correctness | netns entered/exited on the SAME locked thread; no netns leak across pool reuse; setcap binary refuses host-netns run |
| Host-safety | host `nft list tables` identical before/after every netns suite (assert in the harness) |
| No-regression | full non-netlink functional suite green before/after Fix A |
| Rule: qemu-testing | linux-only netns path has QEMU coverage |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Fix C | `test/plugin/plugin-eof-no-spin.ci` passes; manual: SIGKILL daemon, `ps` shows plugin gone, no 100% CPU |
| Fix A | `test/reload/background-ze-readiness.ci` passes; grep runner for background-ze ZE_READY_FILE |
| Fix B | `ZE_TEST_NETNS=1 sudo make ze-firewall-test` green; before/after `nft list tables` diff empty |
| Make target | `grep netns mk/*.mk`; `make -n <target>` shows sudo + env + setcap |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Privilege | Credential drop to a NON-root uid; setcap grants only cap_net_admin,cap_net_raw,cap_net_bind_service (not cap_sys_admin) to `ze` |
| Host isolation | netns mode must be provably active before any nft Apply; refuse setcap+host-netns |
| Input | test ids / suite names are trusted (local dev); no new untrusted input surface |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Integrate netns into the Go runner (per user) | minimal external `unshare -n` wrapper (prototyped, host-safe) | user chose deeper integration for a durable, first-class harness |
| Runner emits readiness for background ze (per user) | rewrite 18+ tests to foreground ze | smaller, localized; keeps tests as-written |
| ze runs as normal-user + setcap, runner privileged | run ze as root in netns | root breaks the readiness handshake (observed) |

## Known Limitations
- Requires a privileged runner (sudo / cap_sys_admin) to create netns; matches the existing
  integration-test model. Not runnable in an unprivileged CI without that.
- macOS still skips these suites (no netns/nft); parity is Linux-only.

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| netlink `.ci` suites pass on Linux host-safely | functional test | firewall/policy/ospf suites green under netns mode; host nft unchanged |
| no hang landmine | functional test | SIGKILL `.ci` → gate completes, plugin CPU ~0 |

## Review Gate

### Run 1 (2026-07-13, /ze-review on commit 650e5f242)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | dropped-ze config read depends on umask (root-owned config; 027/077 → 0640/0600 → EACCES) | `runner_exec.go` config-write paths | FIXED: `os.Chown` the config file to the target uid; AC-7 driver runs under umask 077 as regression (b5cbc5b39) |
| 2 | ISSUE | `ze-netns-test` ran the suite in parallel; netns mode only validated with `-p 1` (R-1) | `mk/test-integration.mk` | FIXED: force `-p 1` (b5cbc5b39) |
| 3 | NOTE | leaked netns → `EEXIST` on a port-recycled name | `netns_linux.go` `enterTestNetns` | FIXED: best-effort `DeleteNamed` before `NewNamed` (b5cbc5b39) |
| 4 | NOTE | `ZE_TEST_UID` root/missing silently ran ze as root | `runner_exec.go` / `netnsChildIDs` | FIXED: fail loudly (`errNetnsNeedsUID`); `TestNetnsChildIDs` boundary test (b5cbc5b39) |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

> Outcome: `/ze-review` found 0 BLOCKER, 2 ISSUE, 2 NOTE; all four resolved and
> re-validated (`make ze-netns-qemu-test` 15/15 under umask 077, host nft
> unchanged) in commit `b5cbc5b39`. No open findings.
>
> Scope note (user-approved): the netns/setcap harness + Fix A/C + nft-rendering
> fixes are delivered and validated. Four pre-existing, non-netns failures the
> Linux run surfaced are triaged separately (they reproduce with `ze` as root, no
> netns): policy Alpine-kernel nft-feature gap, copp `firewall backend not loaded`
> (config lacks a `firewall { backend nft }` block), `004-cli-show` cli-vs-blob
> storage, `009` Alpine-kernel crasher. Recorded in `plan/learned/1112-netlink-ci-harness.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
