# 1275 -- fixit: the six independent reasons GitHub Actions was red

## Context

Every `verify` run on `main` had been failing, and `Deploy website` failed on most
docs pushes. "CI is red" read like one problem and was six, each with a different
root cause and three of them invisible on the maintainer's macOS host. The
investigation ran off two complete job logs (runs 30219943935 and 30209773072),
whose stage results were identical, so nothing here rests on a single sample.

The through-line: **the merge gate runs on a machine unlike any developer's**.
Small (4 vCPU), unprivileged, and shallow-cloned. Three of the six reds are that
difference expressed in different subsystems, and each had been invisible locally
for exactly that reason.

## Decisions

- **`GOOS` is part of the lint contract.** `translate_linux.go:82`'s `unparam`
  finding (`makeHandle - major always receives 1`) is real and deterministic, and
  `make ze-lint` on darwin cannot see it: the file is `//go:build linux` and
  `Makefile:276` runs golangci-lint with no GOOS. Chose to fix the finding (drop
  the always-1 parameter, name `rootQdiscMajor`) over a `//nolint`. Reproduce any
  "CI-only" lint red with `GOOS=linux golangci-lint run ./path/...`. The helper
  is now `tcHandle`, not `makeHandle`: at one argument it would have shadowed
  nothing but read identically to the vendored `netlink.MakeHandle(major, minor)`
  used in the same files, so collapsing `netlink.MakeHandle(1, 0)` into
  `makeHandle(1)` would compile and silently mean 1:1. `classHandle` was taken --
  it is a parameter name throughout the package.

- **A checkout depth is a test input.** `collect_adoption`
  (`scripts/dev/testing_health.py:727-751`) deliberately answers `unknown` in a
  shallow clone, `do_check` compares every metric status, and `actions/checkout`
  defaults to `fetch-depth: 1` -- so `ze-regen-check-readonly` was red on EVERY
  run with a diagnostic ("regenerate and commit") that could not possibly fix it,
  because regenerating in a full clone reproduces the committed `ok`. Chose
  `fetch-depth: 0` over relaxing the check: the metric is right, CI was starving
  it.

- **"Leave 3 cores free" degenerates.** `GO_TEST_PROCS = nproc - 3` was sized for
  a workstation and gave GOMAXPROCS=1 on a 4-vCPU runner. That is two failures:
  `go test -p` defaults to GOMAXPROCS so ~450 packages ran one at a time, and the
  `.et` editor suite blew go's implicit 10-minute package default. Measured it:
  `internal/component/cli/testing` takes 169s at GOMAXPROCS=13 and 357s at 2 on a
  16-core host. Chose a floor of half the cores (n=4 -> 2, n=16 -> 13 unchanged)
  plus an explicit `GO_TEST_TIMEOUT=20m`, because an inherited default is not a
  chosen one.

- **"Unset" and "all at once" must not be the same value.** Every suite in
  `internal/test/cli/register.go` declared parallel `0`, and `Runner.Run`
  (`runner.go:417-421`) turns a non-positive `Parallel` into `len(selected)`. So
  `ze-test ospf --all` launched 97 ze daemons simultaneously and the GitHub runner
  agent itself was killed (exit 143, "the runner has received a shutdown signal").
  Chose to resolve `0` at registration to `DefaultSuiteConcurrency()` (2x CPUs,
  floor 8 -- the value `ZE_PLUGIN_PARALLEL` already survives on a 4-vCPU runner)
  rather than sprinkle `-p` across twenty make lines. `-p 0` still means all, so
  the explicit request survives.

- **`skip-os:value=darwin` is not a capability declaration.** 24 tests hid from
  macOS with `skip-os` (or carried a bare `needs-linux`) and therefore RAN
  unprivileged on the Linux runner, where they failed or hung to their timeout on
  `operation not permitted`. Classification was done from the job log, and the
  first pass was WRONG: every Linux daemon prints a "running without root; missing
  capabilities: ... CAP_NET_ADMIN" banner, which matched 29 tests. Only a real
  EPERM from a netlink/nft/bpf call is evidence, which narrowed it to 16 --
  and the log-based method then MISSED three more (`ddos-transit-forward-drop`
  creates a veth; `ddos-policy` and `ddos-incident-confidence` carry the same
  eBPF-dependent traffic-usage block as their converted siblings) because the
  runner's death had cancelled them before they could produce an error. A CI log
  proves a test needs a capability; only the test's own config proves it does not.

- **Marking tests must relocate coverage, not delete it.** A `caps=` option makes
  a test skip wherever the capability is absent, including the verify runner, so
  marking alone would have turned red into green by removing the tests. Added
  `.github/workflows/qemu-nightly.yml` (scheduled, advisory) running
  `ze-qemu-needs-linux-test`, and `TestCapabilityGatedTestsHaveAQemuHome` fails
  if the redirection is broken from either end: no workflow running that target
  BY NAME, or a gated test carrying a `skip-env` that excludes it from that run.
  Both halves were needed -- see the Gotchas.

- **`caps=` is a LIST, because declaring one of two needed capabilities fails
  OPEN.** The three eBPF-dependent ddos tests were first marked `caps=net-admin`
  on the reasoning that root-in-QEMU has everything anyway. That is assuming the
  conclusion: the probe tests bit 12 and nothing else, so a host with
  `--cap-add=NET_ADMIN` and no CAP_BPF passes a gate it cannot satisfy and the
  test fails exactly as it did unmarked. The runner now accepts
  `caps=net-admin,bpf`. `bpf` maps to CAP_BPF ALONE: the CI message read "need
  CAP_BPF/CAP_SYS_RESOURCE", which invites requiring both, but that is a CASCADE
  -- without CAP_BPF the memcg probe (BPF_MAP_CREATE) fails, so
  `rlimit.RemoveMemlock` falls back to `prlimit(2)` and is denied there too. The
  fallback exists for kernels older than 5.11
  (`vendor/github.com/cilium/ebpf/rlimit/rlimit_linux.go:108`) and every kernel
  this gate runs on is far past it -- ze's appliance builds 7.1.4, CI is 6.x.
  Requiring the second bit would be over-strict, and an over-strict gate SKIPS a
  test the host could run, which is the deletion the mechanism exists to stop.

- **One website pipeline, not two.** The site is built locally
  (`gh-pages/update-website.sh` -> `tools/build.py`) and its HTML committed to
  `gh-pages`, whose own workflow deploys it. main's `.github/workflows/pages.yml`
  was a second, divergent pipeline that rebuilt and deployed on docs pushes; it
  had drifted (it ran `uv run --with markdown`, while `tools/build.py` declares
  `--with markdown --with rcssmin --with rjsmin`) and it failed. Owner decision:
  delete it.

- **The rpki demo raced a ten-minute timer.** `run.sh` backgrounded the RTR cache
  mock and `ze` with nothing sequencing them. ze dials the cache ONCE and then
  waits its RFC 8210 Retry Interval -- 600s by default
  (`rpki/rtr_session.go:81`, which is the RFC's own default and correct). A cache
  milliseconds late to listen therefore costs ten minutes, not a retry: the
  session sits `state: idle`, every prefix validates NotFound, `not-found accept`
  admits the RPKI-invalid `10.43.0.0/24`, and the demo teaches the opposite of its
  point. Fixed by waiting on the fixtures' listeners before starting ze and by
  gating readiness on the VRP count. Verified by injecting a 6s-late cache: it
  failed unrecoverably before the fix and passes after.

## Consequences

- The merge gate's environment is now part of the contract in three places that
  previously assumed a workstation: clone depth, core count, and privilege. A
  future "works locally, red in CI" should check those three first.
- The Linux-only functional surface has an automated home for the first time.
  It is advisory and may run under TCG emulation, so it reports rather than
  blocks; `ai/rules/qemu-testing.md`'s "What actually RUNS these suites" table
  records the split between `ze-qemu-needs-linux-test` (now scheduled) and
  `ze-qemu-integration-test` (still nothing).
- Suites now run bounded by default. On a 16-core host the ospf suite goes from
  97-at-once to 32-at-once; wall clock is comparable because the 97-way run was
  thrashing, but a machine-specific timing baseline may shift once.
- `plan/deferrals/fixit-ci-schedule-evidence.md` (written by a concurrent session
  the same day) says "today NOTHING automated executes the QEMU suites". That is
  now half true: its row is about `ze-qemu-integration-test`, which is still
  unautomated, but the functional QEMU surface is scheduled. The row was left
  untouched -- it belongs to the session that wrote it.

## Gotchas

- **A log banner is not an error.** The capability classifier matched
  "CAP_NET_ADMIN" and produced ten false positives, including four pure-BGP tests
  that would have been wrongly removed from macOS coverage. Grep for the failure,
  not for the topic.
- **An under-provisioned container invents its own failures, and I fell for it
  TWICE.** A first reproduction had all four daemon tests failing on `plugin ...
  TLS connect-back: context canceled`, which looked like a real defect; the
  observers are `#!/usr/bin/env python3` and the Alpine image had no python3.
  Having caught that, the next image lacked `go` -- which several ospf tests exec
  as a helper peer -- and 65 tests died in about a second, which I briefly
  reported as a finding. It was not: `go` is present wherever Go is tested. The
  damage was not the wrong claim but the wrong CONCLUSION drawn from it -- with
  two thirds of the suite dead on arrival the run generated almost no load, so
  "the six pass under load" was meaningless until the image could actually run
  the suite. Provision the reproduction to match the environment before reading
  anything into its failures.
- **A `.ci` option name can appear in prose.** A scripted `option=needs-linux` ->
  `option=needs-linux:caps=net-admin` replacement spliced a comment block into
  the middle of three files' STATUS prose, because `replace(..., 1)` hit a
  documentation mention first. Anchor whole-line edits to column 0.
- **The ospf mass-timeout was a symptom, not a cause.** 40+ ospf tests reported
  TIME in the failing run; they were victims of the runner dying under the 97-way
  fan-out, not 40 broken tests. Read the failure INDEX, not the last screen.
- **The independent review found what self-review could not, and the pattern is
  worth naming: every miss was a place I had reasoned instead of read.** The
  `caps=net-admin`-is-close-enough call for the ddos tests, `show-policy-routes`
  running in NO environment (a caps gate plus a pre-existing
  `skip-env:var=ZE_QEMU`, the only one of 68 gated tests with that combination),
  three more tests needing the marker that the CI log could not reveal because
  the runner's death had cancelled them, a single-test `ze-test <suite> <id>`
  silently getting 3x timeout headroom because the suite default stopped being
  1, and a demo readiness wait that could outlast the 30s VHS `Wait` and abort
  the entire website build. None was visible from the diff alone.
- **`sessions: 1` means "one cache is configured".** It is true while the session
  has never connected, so the demo's readiness loop broke on its first iteration
  and then asserted on data that had not arrived. The VRP count is the first
  observable that means synced.
- **The six ospf failures were load casualties, and the concurrency bound cures
  them. Proven, not inferred.** They could not be attributed at first: the job
  was killed mid-suite so the ospf failure index never printed. Reproduced
  afterwards in a container (cross-compiled linux binaries, 2 CPUs, go + python3
  + iproute2 present) -- same suite, same binaries, concurrency the only
  variable:

  | run | result | wall clock |
  |-----|--------|-----------|
  | `-p 0` (the old default: all 89 at once) | 4 pass, 36 fail, 49 timeout | 251s |
  | bounded `DefaultSuiteConcurrency` | **89 pass** | 68.7s |

  All six (3, 30, 31, 33, 66, 88) are in the unbounded run's failure sets and
  all six pass bounded. The bounded run is also 3.7x FASTER, which is what
  proves the unbounded one was thrash rather than work. Declining to mark them
  `caps=` was right: two of them are pure `ze config validate -` with no daemon
  at all.

- Two plugin tests (`as112-external-refuses`, `cos-external-warns`) failed on
  this macOS host when run in isolation and passed in the full verify run, so
  they are load-sensitive locally, not deterministic. They passed on the Linux
  runner. Unrelated to this work.

## Files

| File | Change |
|------|--------|
| `internal/plugins/traffic/netlink/translate_linux.go` | `rootQdiscMajor` const; `makeHandle(minor)` |
| `.github/workflows/verify.yml` | `fetch-depth: 0` |
| `.github/workflows/qemu-nightly.yml` | NEW: scheduled `ze-qemu-needs-linux-test`, KVM udev best-effort, TCG fallback |
| `.github/workflows/pages.yml` | removed in this commit (gh-pages owns the deploy) |
| `.github/workflows/evidence-nightly.yml` | reconciled the "no QEMU in CI" rationale |
| `Makefile` | `GO_TEST_PROCS` floor; explicit `GO_TEST_TIMEOUT` |
| `internal/test/runner/parallel.go` | `DefaultSuiteConcurrency`, `SuiteConcurrencyFloor` |
| `internal/test/cli/dispatch.go` | `registerCIRoot` resolves 0 to the bounded default |
| `scripts/dev/github_workflows_test.go` | pins qemu-nightly's shape; links markers to a QEMU home |
| 24 `.ci` files | `option=needs-linux:caps=net-admin`, or `caps=net-admin,bpf` for the six eBPF-dependent ddos tests |
| `demos/terminal/rpki/{run,validate}.sh` | wait on listeners and on the VRP count |
| `ai/rules/qemu-testing.md` | the marker relocates coverage; CI table updated |
