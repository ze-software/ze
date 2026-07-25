### `ze-test bgp reload` -- fixed startup deadlines fail under CPU oversubscription

**Status 2026-07-23: the suite is GREEN unloaded (31/31, 7 honest skips). What
remains is one coherent issue, stated below, not a set of flaky tests.**

The original entry claimed four "load-sensitive" tests sharing a root cause: a
plugin connection closing between SIGHUP and the verify dispatch. **That was a
hypothesis and it was wrong.** The first real `stress-repro.py` run disproved it:
`connection closed`, `crashed?`, `verify failed` and `plugin ike` appear nowhere
in the capture. Two of the four had real, separate, now-fixed root causes, and a
third had nothing to do with load at all:

| Was | Actually | Where |
|-----|----------|-------|
| test 2 `commit-transactional` | a successful reload emitted NO completion signal, so the test raced its own teardown | fixed; see `RESOLVED.md` 2026-07-23 |
| test 34 `test-tx-ipsec-eap-tls-requires-ca` | peer script sent SIGHUP and SIGTERM back-to-back | fixed; `await=stderr` fence |
| test 18 + six `test-tx-iface-*` | needed CAP_NET_ADMIN; HUNG instead of skipping | fixed; `option=needs-linux:caps=net-admin` |

### What is actually left

Under deliberate CPU starvation (`stress-repro.py` at 16-64 burners on 32 cores)
several reload tests fail on **fixed startup deadlines**, not on any reload race:

- `peer did not start listening within 5s` (`internal/test/runner/peer_contract.go:169`,
  `runner_exec.go:147`) -- a hardcoded 5s bind deadline for `ze-peer`.
- `expected messages: 1, received messages: 0` -- the BGP session not established
  within the per-test timeout.

Both are the harness giving a process a fixed wall-clock budget on a machine that
is 50-200% oversubscribed. They are NOT reload defects: every one of these tests
passes unloaded, and the two genuine races found in this area have been fixed at
source rather than waited out.

**Why this is not fixed here.** `internal/test/runner/peer_contract.go` and
`runner_exec.go` -- which own the 5s deadline -- are being actively edited by a
concurrent session (uncommitted at the time of writing). Editing them would
cross-commit that session's work, which `CLAUDE.md` forbids. Owner has confirmed
the sleep-under-starvation class is already planned work. Reproduce with:

```
python3 scripts/dev/stress-repro.py "bgp reload" --any-failure --timeout 600
```

Note the suite name: there is no `reload` suite: the reload tests live under
`bgp`, and a sub-suite is passed as ONE argument.

---

## Found by independent review 2026-07-23, owned elsewhere

Three findings from the review of the fixes above. None is parked: each names
where it is owned and what the evidence is.

### 1. `await=stderr` + a check-mode peer never `Wait()`s the peer

`internal/test/runner/runner_exec.go`: the `awaitStderrSW != nil` branch returns
without reaching the `default:` branch, which is the ONLY place peer processes
are `Wait()`ed; the teardown loop skips peers explicitly. `proc.Stdout` is a
`syncWriter`, so `exec.Cmd` interposes a pipe plus a copier goroutine, and
`Wait()` is what joins it. Reading `rec.PeerOutput` without it is a lost-tail
race.

Every pre-existing `await=` test is peer-less; `commit-transactional.ci` and
`test-tx-ipsec-eap-tls-requires-ca.ci` are the FIRST to combine the fence with a
check peer. It fails closed (spurious red, not a false green) and does not
reproduce today: 10/10 and 20/20 respectively, plus 40/40 for the ipsec test
under the 64-burner profile. **Not fixed here because `runner_exec.go` is being
edited by a concurrent session.** One-line fix: wait for peers after the fence
returns as well as instead of.

### 2. Eight reload tests are vacuous: their peer completes before the SIGHUP

`test-tx-iface-apply`, `-bgp-chain`, `-tunnel-create`, `-wireguard-apply` (still
`skip-os:darwin`), and `-tunnel-modify-key`, `-tunnel-remove`,
`-wireguard-modify`, `-wireguard-remove` (now `caps=net-admin`).

All eight declare only the two pre-reload expectations at `conn=1:seq=1` and
drive the reload from a `trigger.sh` that sleeps 2s after `daemon.ready`. The
check peer's `Checker.Completed()` goes true on the second message, it prints
`successful` and exits, and the runner tears the daemon down roughly 1.7s BEFORE
the SIGHUP. Measured: `received SIGHUP` appears 0 times in the daemon stderr of
all four unskipped ones, which nonetheless PASS in ~2.2s.

So four of the "31/31 green" above are green because they assert nothing, and the
four gated ones would pass in QEMU without reaching the reload either. This is the
sleep-in-tests class the owner has confirmed is already planned work; recorded
here with the measurement so that work does not have to rediscover it. The fix
shape is known and already exercised by a sibling:
`test-config-apply-ordering-create.ci` has no sleep in its trigger and its peer
expectations can only be met post-reload, so it genuinely fences.

### 3. The daemon hangs, rather than failing, when it cannot apply interface config

Underneath finding 2: an unprivileged `ze` whose config contains an
`interface { ... }` block dies in the interface plugin's stage-3 handshake
(`iface: set up "...": operation not permitted`) and the daemon then sits there.
That is a product-side `ai/rules/fail-closed-guards.md` violation -- it neither
starts nor says why -- and it is what the `caps=net-admin` marker steps around
rather than fixes. Operator-facing, unrelated to tests: run `ze <conf>` as a
non-root user with any interface block and it never returns.


---

## Update 2026-07-25: findings 1 and 2 FIXED; finding 2 uncovered a real reload defect

### Finding 1 (`await=stderr` never `Wait()`s the peer) -- already fixed

`runner_exec.go` now calls `drainPeers(peerOutputs, peerDrainGrace)`
(`internal/test/runner/runner_exec_util.go:413`) on EVERY arm before anything
reads `rec.PeerOutput`, and it `Wait()`s each peer not already waited. The
lost-tail race this entry described is closed.

### Finding 2 (eight vacuous reload tests) -- FIXED, and what they test is BROKEN

The measurement was correct: all eight declared their BGP peer in the INITIAL
config and drove the reload from a trigger sleeping 2s after `daemon.ready`, so
the check peer completed on the second PRE-reload message and the runner tore
the daemon down before the SIGHUP. Fixed by moving `peer peer1` into the
reloaded config (peer expectations reachable only post-reload), deleting the two
`sleep 2` lines and the trailing `kill -TERM`, and adding an `expect=stderr` on
the interface plugin's apply log.

They now genuinely reach the reload (`received SIGHUP` appears, the peer
completes after it) and they FAIL, because interface reload is broken. Two
distinct defects, both found this way:

1. **FIXED -- the monitor dropped every address event for pre-existing links.**
   `netlink.LinkSubscribe` delivers only CHANGES, so the monitor's index->name
   cache started empty and `handleAddrUpdate` returned early for any interface
   predating the monitor -- which is every interface ze creates during boot
   `applyConfig`, since the monitor starts after it. The config-transaction
   settlement waiter for an address add blocks on that `interface/addr-added`
   event (`internal/component/iface/operation.go:57-65`, 5s), so EVERY reload
   changing an interface address timed out and rolled back. Reproduced in QEMU:

       config apply partial failure: operation interface-add-address-zdiag0-10.77.0.2_24
       settlement timeout waiting for interface/addr-added 10.77.0.2 after 5s

   Fixed by seeding the cache from `netlink.LinkList()` at monitor start
   (`internal/plugins/iface/netlink/monitor_linux.go`, `seedLinkNames`). The
   transaction then completes in ~5ms instead of timing out.

2. **OPEN -- the transaction now reports success but leaves the wrong state.**
   With (1) fixed, a SIGHUP changing a dummy's address from 10.77.0.1/24 to
   10.77.0.2/24 logs `interface config operation journal committed` /
   `config reload completed` / `sighup reload complete`, and the interface ends
   with NO address: the old one removed, the new one absent. Reproduced in QEMU
   with `ze.storage.blob=false` -- WITHOUT that, the blob store serves a stale
   config and the reload reports "no changes", which will mislead anyone
   re-running this. Ruled out: the commit path is correct
   (`OnConfigOperationCommit` calls `j.Discard()`, not `Rollback()`,
   `internal/component/iface/register.go:693-706`), and the add DID settle, so
   the address reached the kernel and the monitor observed it. NOT traced past
   that point -- do not attribute it without reading the producer.

   Deterministic, so it does not belong in this directory; recorded here only
   because it is what these eight tests now expose. It needs a spec. The eight
   tests are correct as written and must NOT be weakened to go green
   (`ai/rules/no-parking.md`); they are QEMU-only (`skip-os:darwin` /
   `caps=net-admin`) so native `make ze-verify` stays green.

   Likely the same family as `iface-vlan-unit-address-reconcile.md` in this
   directory ("VLAN unit address change never applied on reload").

### Finding 3 (daemon hangs when it cannot apply interface config) -- DISPROVEN 2026-07-25

**The daemon does NOT hang. The hang claim was wrong.** Reproduced in the QEMU
Alpine VM (`scripts/evidence/qemu-run.py`, aarch64, kernel 6.12.13-0-virt) with
an unprivileged user (`adduser -D zetest`, uid 1000, no CAP_NET_ADMIN) and a
config carrying only `interface { dummy zdiag0 {} }`:

    start   2026-07-25T19:37:07Z
    exited  2026-07-25T19:37:07Z
    VERDICT[unpriv-stdin]: EXITED rc=1 elapsed=0s => NO HANG

Identical for both daemon entry points (`ze -` with the config on stdin, and
`ze start <file>`). The control case -- the same binary and config as root --
started normally and created `zdiag0`, so the only variable was privilege.

Note `ze <config>` as a bare positional no longer exists: it was removed by
spec-fixit-config-file-positional-grammar (`cmd/ze/ze_core_dispatch.go:402-404`)
and now prints "unknown command". The original finding's repro line is stale.

The failure is at **stage 2 (configure)**, not stage 3: `deliverConfigRPC failed`
-> `plugin interface failed during startup at stage Config`. The plugin-side
"stage 3 (declare-capabilities)" error is a consequence (the pipe is already
closed), not the cause. Phase 1 of `runPluginStartup`
(`internal/component/plugin/server/startup.go:101-108`) sets `startupErr`, and
`cmd/ze/hub/main.go:1063` exits 1 -- the guard already fails CLOSED.

**What was really wrong: the error dropped its cause.** `runPluginPhase`
synthesized the phase error from `proc.Stage()` alone, so the operator got

    error: config-path plugin startup failed: plugin interface failed during startup at stage Config

naming no offending object, no reason and no corrective action, while the real
cause was logged at Debug level and discarded. Fixed 2026-07-25:
`Process.SetStartupError`/`StartupError` record the handshake cause,
`startupFailureError` (`internal/component/plugin/server/startup_failure.go`)
wraps it with `%w`, and `joinApplyErrors` (`internal/component/iface/register.go`)
appends the CAP_NET_ADMIN remediation when the kernel refused for want of
privilege. The same run now ends:

    error: config-path plugin startup failed: plugin interface failed during startup at stage Config: rpc error: interface config: dummy zdiag0 create: iface: create dummy "zdiag0": operation not permitted (interface configuration needs CAP_NET_ADMIN: run ze as root, or grant the binary the capability with `setcap cap_net_admin+ep <path-to-ze>`)

`ze doctor` already covered this before startup and still does:
`[doctor-iface-macvlan] cannot create a probe macvlan device (requires
CAP_NET_ADMIN)`, so no new doctor check was needed.

This does not change the `caps=net-admin` markers: the TEST still hangs to the
suite timeout, because its check peer waits for a BGP session the exited daemon
will never open. The rule text in `ai/rules/qemu-testing.md` says exactly that
and is correct; only its "stage-3 handshake" wording is off by one stage.
