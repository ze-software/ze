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
