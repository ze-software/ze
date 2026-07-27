# 1276 -- fixit: the CI reds that survived the six root causes

## Context

Learned 1275 fixed six causes of a red `verify` and left twelve failures
unexplained, split by the maintainer into three guesses: five that "run on macOS
too and pass here", five Linux-only ones "never exercised locally", and two
suspected load casualties. The guess about WHICH tests grouped together was
right; the guess that each group shared a cause was wrong. Every remaining test
had its own defect, and the run at `ea980cf76` showed three more reds nobody had
listed -- including the entire 87-test web suite.

The through-line is not an environment difference. It is that **a test which has
never executed is not a test**. `skip-os:value=darwin` hid five of them from the
only machine anyone ran, CI had never been green, and so five separate broken
assertions sat unexamined: one named a config family that does not exist, one
asserted response fields the handler never emitted, one demanded an exit code
that contradicted its own next line, and two waited on an announce that nothing
ever triggered.

## Decisions

- **`skip-os:value=darwin` is not a capability declaration, and the distinction
  is the opposite of harmless.** It hides a test from macOS and therefore RUNS it,
  unprivileged, on the Linux runner -- exactly where it cannot pass.
  `resolve-ping` failed every CI run on `status=error` because `doPingCtx`
  (`internal/component/ping/cmd/ping.go`) opens `ip4:icmp`, which needs
  CAP_NET_RAW. Added a `net-raw` token to `capsRequired`
  (`internal/test/runner/caps.go`) so the test relocates to the QEMU runner that
  has the capability instead of failing where it does not. Learned 1275 added
  `net-admin` and `bpf` for the same reason; the vocabulary was simply missing an
  entry, and the missing entry read as "this test is macOS-specific".

- **A prerequisite a checkout does not carry gets a declared SKIP, not a silent
  pass.** `gokrazy/modcache/.gitignore` ignores everything but the vendored
  gokrazy init, so the pinned `rtr7/kernel` module and its 15 MB `vmlinuz` exist
  only after `make ze-gokrazy-deps`; `ze-kernel-overlay.ci` read that `vmlinuz`
  unguarded and died `shasum: ... No such file or directory` on every run. Added
  `option=needs-path:value=<repo-rel>[:hint=<cmd>]`, which skips VISIBLY naming
  the path and the command that materializes it. Chose that over an `exit 0`
  guard: a green bar over a test that ran nothing is worse than the red it
  replaces. Chose it over adding `ze-gokrazy-deps` to the merge gate, which would
  put a heavyweight network fetch in front of every push.

- **The device under test must be resolved, never spelled.** `cmd_web.go` built
  its own `<baseDir>/bin/ze` path while the functional flow builds an isolated set
  into `tmp/testbin-*/bin` and exports `ZE_BIN` (`mk/test-functional.mk`). Wrong
  in BOTH directions: on a fresh checkout all 87 `.wb` tests died in ~4ms on
  `fork/exec bin/ze: no such file or directory`, and on a developer host a
  leftover `bin/ze` hid that AND meant the suite tested a stale binary that was
  not the one under test -- passing for the wrong reason. `buildZe` already
  existed in the same package. Added a source-level guard
  (`dut_resolution_test.go`) that fails on any `filepath.Join(..., "bin", <ze
  binary>)` in the package, narrowed to ze binary names so the exabgp fixture
  wrappers (`test/exabgp/bin/exabgp`) do not trip it.

- **Two budgets for one wait, and the smaller implicit one wins silently.**
  `as112-external-refuses` declares a 15s command budget and waits on an
  `await=stderr` fence whose default was a hardcoded 10s, so under load the fence
  expired INSIDE a budget the test had explicitly asked for. The fence's own
  comment said it was "kept below the suite's per-test timeout (15s)" -- a real
  intent pinned to one specific number while the effective budget is computed at
  run time. Derived it instead (`defaultAwaitStderrTimeout`, 80% of the resolved
  budget, floored at the old 10s), which preserves the ordering at any budget.
  Chose deriving over raising the constant: a bigger fixed number is the same bug
  one size later.

- **Doctor was silent where it should have spoken, in two different ways.**
  `ze doctor` reported `"ready": true` and exit 0 for configs `ze config validate`
  rejects -- it never ran the engine's peer resolution. Added `checkBGPPeerConfig`
  through the `infra.ValidateBGPPeers` seam (config cannot import infra: cycle),
  and it immediately found a SECOND latent defect in the very config being fixed.
  Reported at ERROR (owner decision): `ready=false` and exit 1, because a
  readiness check that blesses a config the engine refuses is the trap the check
  exists to close. THREE shipped `test/ui` fixtures turned out to carry
  engine-invalid configs while asserting exit 0 (doctor-bgp-listen,
  doctor-bgp-md5, doctor-config-reference); all three were the defect and all
  three are fixed rather than accommodated. The tree is CLONED before the
  validator sees it -- PeersFromConfigTree calls PruneInactive, an in-place
  mutation, and doctor's tree is shared by ~30 later checks.
  Separately, `checkMPLSSupport` returned nil when the module list was unreadable,
  so a broken check and a healthy host produced byte-identical output -- now
  `doctor-mpls-unknown`.

- **The failure report must not overwrite a recorded error with a guess.**
  `likelyCauseTimeout` replaced `awaitDaemonStderr`'s precise "stderr never
  contained X within Ys" with "server likely failed to start or crashed", which
  for an await fence is simply false -- the daemon started fine. That guess cost
  several reproduction rounds. `rec.Error` now wins over every heuristic.

## Consequences

- The `.ci` prerequisite vocabulary is now three-way: `skip-os` (this OS cannot),
  `needs-linux:caps=` (this host lacks a capability), `needs-path` (this checkout
  lacks an artifact). Reaching for the first when you mean the second is what put
  five tests on a runner that could never pass them.
- `qemu-nightly`'s first execution measured what was previously assumed: KVM IS
  usable on a hosted runner after the udev rule, the guest sizing holds, and boot
  costs 9.8s. The one thing that did NOT hold was the inner budget -- 3269s of
  3600s, 91% -- raised to 5400s with the per-phase timings recorded in
  `mk/test-integration.mk`. The 300s TCG boot budget remains unexercised because
  KVM was available; it stays as insurance and the workflow says so.
- Two `.ci` mechanisms are now known to be inert decoration and must not be
  mistaken for behavior: `cmd=api:...:text=` is a REPORT LABEL consumed only by
  `report.go`, and it reads exactly like the `ze_api.API.send()` string it is not.

## Gotchas

- **A unit test that hand-builds a config tree can pin the bug.** `TestMPLSInUse`
  passed for as long as `containerPeersLabeled` was broken, because both built
  `family` as a CONTAINER -- while the parser produces a LIST keyed by the family
  name, which every other reader in the tree (`web/page_bgp_*.go`,
  `exabgp/migration`) already used correctly. The test and the code agreed on a
  shape no config ever has. Fixtures for tree-walking code should be PARSED, not
  assembled; the new `checks_mpls_linux_test.go` does that and needs a blank
  import of `internal/component/bgp/yang` to make `bgp {` a known keyword.
- **Two independent faults can produce one symptom, and fixing the first reveals
  nothing.** `mpls-doctor` named a family that does not exist AND read `family`
  as a container. Either alone produced the identical silent pass, so the first
  fix looked like it had failed.
- **Attribute the red before claiming it.** Four `doctor` listener unit tests and
  13 `bmp` lint issues appeared during this work. Removing every one of my
  changes left the four failing identically, and the lint was in files never
  touched -- a concurrent session in the same checkout. The isolation run is what
  made that a fact rather than a hope; the FIRST isolation attempt was wrong
  because it removed only the call site and not the new import, which can change
  init-time registration.
- **Fail-closed is not always right, and the wrong one regresses the other
  platform.** `ze-peer --ttl` returned the setsockopt error when the platform has
  no TTL options -- so the listener never bound on macOS and `bgp-gtsm`, the test
  the flag was added to FIX on Linux, started failing on darwin. Where the DUT
  cannot enforce GTSM either (`ttl_other.go` is a no-op stub), an unsupported
  option is not a failure. `network.IsIPTTLUnsupported` exists for exactly this.
- **A reproduction harness invoked wrongly fabricates a reproduction.**
  `stress-repro.py "plugin"` (instead of `"bgp plugin"`) dispatched `ze plugin
  -v`, whose usage error `--any-failure` faithfully reported as a reproduction on
  invocation 1.
- **Do not force-run a test outside the scope it is declared for.** Running
  `as112-external-refuses` in the QEMU debug VM failed it, which reads as a
  regression; it carries no `needs-linux`, so `ZE_QEMU_LINUX_ONLY=1`
  (`record_parse.go`) skips it in the real nightly, and it PASSES on the CI
  runner. The debug VM is a diagnosis tool, not a verdict.
- **An RFC claim needs the producer read, not a story.** A LOCAL_PREF-on-eBGP
  "violation" was drafted here on the reasoning that the attribute "can only be
  here because the caller put it there". False: the same builder carries relayed
  blocks (`buildWireModeUpdate`), which also raises RFC 7947 route-server
  transparency. Reverted rather than shipped.

## Files

| File | Change |
|------|--------|
| `internal/test/cli/cmd_web.go` | resolve the DUT via `buildZe`, not `<baseDir>/bin/ze` |
| `internal/test/cli/dut_resolution_test.go` | NEW: source guard against re-spelling a ze binary path |
| `internal/test/runner/needs_path.go` (+test) | NEW: `option=needs-path` prerequisite gate |
| `internal/test/runner/caps.go` | `net-raw` token (CAP_NET_RAW) |
| `internal/test/runner/await_stderr.go` (+test) | fence budget derived from the resolved test budget |
| `internal/test/runner/report.go` | a recorded error beats every likely-cause guess |
| `internal/test/peer/listen_ttl.go`, `peer.go`, `cmd_peer.go` | `ze-peer --ttl` (RFC 5082 GTSM peer) |
| `internal/component/doctor/checks_config.go` | NEW `checkBGPPeerConfig` via the infra seam |
| `internal/component/doctor/checks_linux.go` | `family` read as a LIST; `doctor-mpls-unknown` |
| `internal/component/doctor/checks_mpls_linux_test.go` | NEW: parsed-fixture tests for the MPLS check |
| `mk/test-integration.mk` | inner QEMU budget 3600s -> 5400s, measurements recorded |
| `.github/workflows/qemu-nightly.yml` | first-run measurements replace the assumptions |
| 10 `.ci` files | the individual broken assertions |
