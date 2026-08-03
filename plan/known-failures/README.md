# Known Failures

This directory tracks pre-existing, non-deterministic (flaky/environmental)
test failures, **one file per live failure**, so concurrent sessions never
cross-commit each other's rows. It replaces the single tracked file
`plan/known-failures.md`, sharded here by
`spec-fixit-shared-plan-file-contention` (phase 3).

## How this directory is organized

- **One live failure = one shard file** named
  `plan/known-failures/<make-target>-<test-name>.md`. To log a new red,
  create a new shard; `git add` then stages only your file, never another
  session's pending rows.
- **`RESOLVED.md`** is the verbatim archive of every resolved and
  struck-through entry. When you clear a red, append its entry to
  `RESOLVED.md` and delete its live shard.
- **`README.md`** (this file) holds the logging instructions below. Neither
  `README.md` nor `RESOLVED.md` counts as a live failure.
- The live count is folded on read, never stored: `collect_known_failures`
  in `scripts/dev/testing_health.py` counts the shard files (excluding
  `README.md` and `RESOLVED.md`) as live and the `### ` entries in
  `RESOLVED.md` as resolved.

Pre-existing test failures tracked here per `ai/rules/git-safety.md` ("Before Any
Commit" -> pre-existing failures >10 min): logged, not blocking unrelated commits.

**Scope: TEST reds whose MECHANISM you could not determine.** Deterministic
structural gates (`ze-lint`, `ze-lint-changed`, `ze-tier-check`, `ze-vet-evidence`,
`ze-plugin-boundary-check`, `ze-iface-resolution-check`, `ze-regen-check-readonly`,
`ze-verify-wiring-docs`) are NEVER logged here -- a red means the tree is
structurally broken; fix it at the source. `scripts/dev/commit_helper.py` enforces
this by refusing `--unverified` while a structural gate is red (see
`ai/rules/git-safety.md` "Structural Gates Are Never Known-Red").

**Host load is a mechanism, so it does not belong here either**
(`ai/rules/completion.md`, owner directive 2026-07-26). Once you can say
"it fails when the machine is busy",
you have the diagnosis: the test asserts on elapsed time instead of on state. Fix
it to wait on the condition. "Load-sensitive", "passes in isolation", "the failing
set rotates" and "not reproducible on a quiet host" are all that same diagnosis
restated, and none of them opens a shard. Raising a timeout is not a fix either --
it only moves the load level at which the test lies.

## BEFORE LOGGING ANYTHING HERE: reproduce with the Makefile's build tags

Bare `go test ./...` is **NOT** equivalent to `make ze-unit-test` in this repo.
Features compile out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_web`,
`ze_ssh`, ...), and the Makefile always supplies them (`Makefile:51` `ZE_FEATURES`
read from `feature-gates.txt`, `Makefile:65` `GO_TEST_TAGS`). A bare run silently
drops those plugins, so their registrations, validators and listeners never exist
and unrelated tests fail with phantom reds.

```
go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" ./path/...
```

This is not hypothetical: on 2026-07-15 two of the four entries below (7 tests)
were disproven as pure tags artifacts. Both had been logged with a confident but
wrong root cause (a "macOS socket-stack quirk", a "broken listener-conflict
validator"), and one had been "re-confirmed" six days later by repeating the same
flawed invocation. Reproducing a symptom is not attributing a cause.

**Status 2026-07-25: 13 live shards swept, 11 cleared, 2 left open.** Resolved and archived to
`RESOLVED.md`: `static-show-obsolete-next-hop-syntax` (fixed, and four more
defects in the same suite with it), `ci-suites-quick-exit-ze-unasserted` (149
commands armed across 52 files), `syncpool-capacity-identity-flakes` (both tests
asserted a guarantee `sync.Pool` does not make), plus two that were simply STALE
-- `install-kernel-tests-isolated-binary-layout` (fixed by `ebf0dfbad` two days
after the shard was written) and `bgp-plugin-show-l2tp-tunnel-detail` (passes).
Two stale shards out of thirteen is the reason to re-run a shard's own
reproduction before believing it.

Four more cleared once their fixes landed: `l2tp-session-stopccn-cascade` (its
receive-window diagnosis had no producer; the real cause is the missing
CAP_NET_ADMIN its own marker already declared),
`iface-vlan-unit-address-reconcile` (a Linux same-subnet secondary flush, checked
against the kernel directly rather than inferred),
`bgp-plugin-role-otc-export-unknown` (the test had let its own source peer
disconnect, and its gate `if total < 0` could never fire), and `rsvpte-lsp-setup`
(stale, closed on a third independent sweep).

A second pass the same day cleared two more.
`bgp-plugin-dest-peer-teardown-cluster` closed once its last two members had
producer-level reasons rather than green runs: 85 was STALE (its fail-open
observer had been replaced by the event-driven `run_rs_observer` in `4b52d74a6`
two days after the shard was written), and 97 `bmp-locrib` had a genuine
fail-open -- a 15s marker deadline that fell through to `request shutdown` whether
or not the marker appeared, plus a 0.3s sleep standing in for a barrier -- now
replaced with the established+`quiesce()` and `wait_until`+`runtime_fail` fences
and mutation-verified. `reload-transaction-tests-load-sensitive` closed because
its headline issue (fixed startup deadlines under CPU starvation) was already
fixed at source by `withParallelHeadroom`
(`internal/test/runner/runner_exec_util.go`), and because its finding 3 --
"the daemon hangs when it cannot apply interface config" -- was DISPROVEN in QEMU:
the daemon exits rc=1 in under a second as an unprivileged user. The real defect
there was an error that dropped its cause, now fixed.

Two remain open, for different reasons:

- `bgp-plugin-rs-forward-duplicate-and-order` -- the startup-ordering race it
  described IS fixed (the ownership decision is now declarative, delivered on
  Stage-2 configure; the margin went from 1-2 ms to 430-849 ms, mutation-verified).
  **Both symptoms nevertheless still reproduce**, and the capture disproves the
  entry's own causal claim for test 254: self-replay was off 849 ms before the
  first session existed and the duplicate still occurred. Remaining work is owned
  by `plan/spec-fixit-stored-route-relay-hardening.md`. A separate defect surfaced
  in the same capture -- a receiver session established and closed 1 ms apart under
  load -- and is recorded in the shard.
- `reload-config-apply-ordering-rotation` -- still never reproduced. Two fail-open
  defects in the mapped-batch path it exercises were fixed (a stuck OPEN handshake
  wedged the whole batch until the outer timeout with no diagnosis; a batch
  handshake failure did not name which connection failed), but neither explains
  the recorded `mismatch` symptom and the shard says so.

The sweep also found bugs that were NOT tracked here, which is the other reason
to run the suite rather than read about it: `make ze-unit-test` was red on every
darwin host (`ze-installer-unit-test` cross-compiled a linux test binary and then
tried to exec it); the test runner put the wrong `ze` on a child's PATH (under a
session the binary is `ze-<id>`, so a bare `ze` lookup found whatever stale
`bin/ze` existed -- a DARWIN binary when driving QEMU); and interface address
changes could not be applied by reload at all (see the "fixed startup deadlines"
entry in `RESOLVED.md`).

**Status 2026-07-15: 1 open entry (`rsvpte-lsp-setup`, genuinely
non-deterministic). Of the other three: 2 disproven as tags artifacts (7 tests,
never broken) and 1 fixed (`TestCmdMethods`, a real stale literal).**

**Status 2026-07-04 (historical): three open entries (below).**
`TestBuildCommandTreeEnsureExists` (config/yang) is now resolved (stale test
retargeted to the typed name selector -- see Resolved). A new open entry, the
`rsvpte-lsp-setup` load-only panic, is added. The OSPF build break and
`plugin/all` golden-snapshot failures from 2026-07-02 are resolved (the
concurrent OSPF session's `multi_instance.go` work landed and snapshots are
current). Every previously tracked entry from 2026-07-01 and earlier remains
resolved (see below).
