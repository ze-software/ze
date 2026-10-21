# Handover 16: ze-image-docker-host, Phase 5 stopped (disk full)

Spec: `plan/spec-ze-image-docker-host.md` (in-progress). Written 2026-10-21
18:20 BST by the Phase 5 agent, which stopped because the Mac's data volume is
full (`df`: 927G, 2.7G free) and the AC-10 run could not build. Phase 5 code
is NOT committed. Supersedes handover 15 for Phase 5 items only.

## Why it stopped

The AC-10 run (`./le test qemu docker-lab lab "perf run dut ze"`, 18:14 BST)
died before boot: the host cross-build of the probe `ze` failed with
`link: mapping output file failed: no space left on device`, and the kernel
tree copy into `tmp/kernel/build` failed the same way. No docker-host VM was
left running. Another session's VM (`qemu-system-aarch64 ... -m 16384 -cdrom
~/.cache/ze/alpine-iso/...`, launched by session 2026-10-20-0b4a8e87) was
running at the time; it is not this agent's and was not touched. Nothing was
deleted. `./le scratch cache-clean` is the documented remedy, but freeing
space on the owner's machine is his call.

## Done in Phase 5 (uncommitted)

- AC-9 arm64 (HVF) passed 18:06-18:07 BST; evidence pasted in the spec under
  Implementation Steps, Phase 5 (verdict pass, check-exit 0, Ze kernel
  `7.2.9-ze` on the console, alpine-iso mtime unchanged, no guest le built).
- Decisions recorded in the spec: D-10 settled as Ze's own ISO 9660 cidata
  writer; le's own `boot.sh` (no Lima host agent under QEMU); forwarded socket
  at `<checkout>/tmp/qemu/docker-lab-<n>/docker.sock` (103-byte limit).

## Uncommitted files, Phase 5's

| File | State |
|------|-------|
| `internal/le/test/qemu/dockerlab.go` | rewritten (+710/-): boots the docker-host image, cidata, 9p shares at host paths, ssh socket forward, host le, Ctrl-C; Alpine path removed. Does not call `dockerhost.Environment` |
| `internal/le/test/qemu/cidata.go`, `cidata_test.go` | new: ISO 9660 cidata writer and its read-back tests |
| `internal/le/test/qemu/dockerlab_test.go`, `dockerlab_integration_test.go`, `dockerlab_leftover_test.go` | adapted to the rewrite |
| `internal/le/test/qemu/actions.go`, `run.go`, `cache_share_test.go`, `install_iso.go`, `install_test.go` | small adaptations (Alpine docker-lab path and guest le build removed) |
| `docs/architecture/testing/qemu-integration.md`, `ci-workflows.md`, `runner-architecture.md`, `.github/workflows/evidence-nightly.yml` | docs and nightly for the image-based docker-lab |
| `plan/spec-ze-image-docker-host.md` | Phase 5 decisions and AC-9 evidence (file also carries Phase 4 hunks) |

Not Phase 5's: `internal/le/test/qemu/install_build.go` and
`internal/le/dockerhost/` belong to Phase 4 (the two-value
`dockerhost.Environment` change). `internal/le/interoplab/*` and
`internal/le/setup/*` are Phase 4's too.

Last known unit state (from Phase 4's run, before this stop):
`TestDockerLabBuildsUnderTheVerbRoot` and
`TestDockerLabForwardsOneInterruptThenStopsTheVM` red with "ran go under a
canceled root: env GOFLAGS". Not re-run since; re-run
`./le job run go test ./internal/le/test/qemu/` once Phase 4's signature
change lands, and fix them (they are Phase 5's).

## Owed, in order

1. Free disk (owner), then re-run AC-10 on the arm64 Mac: the scratch script
   `p5-ac10b.sh` in the parent session's scratch does it (start the lab, wait
   for `verdict  pass`, 60 s, one SIGINT, record exit and leftover qemu). AC-10
   wants `left behind: none`, VM stopped, exit 1.
2. Fix the two red docker-lab tests above.
3. Remove `ZE_DOCKER_LAB_GUEST` (`interoplab.DockerLabGuestVariable`,
   `inDockerLabGuest`, `TestDockerKernelRouteInsideTheDockerLabGuest`) in
   `internal/le/interoplab/kernelcheck.go` once Phase 4 releases that file;
   replace with a refusal that names the runtime.config fix when the daemon's
   kernel release ends in `-ze`.
4. `Hugepages.applianceEnv` (`internal/le/test/qemu/hugepages.go`) runs
   `ze appliance build` with no endpoint selection (handover 15).
5. AC-9 amd64 under KVM is the nightly's run; not provable on this Mac.
6. Gates for the main thread: scoped golangci-lint over `internal/le/test/qemu`,
   `./le test unit all`, `./le verify worktree`, `./le doc check verify`
   (the `developer-setup.md` anchor on `dockerLabKernel` needs repointing).

## Final state, 2026-10-21 ~20:00 BST (Phase 6 agent)

Items 1 to 3 above are done: AC-10 passed and its evidence is committed
(63d1b8561d); the docker-lab tests went green in ca386b6ae3; the
`ZE_DOCKER_LAB_GUEST` removal is written and race-green but NOT committed (see
below). Phase 6 (AC-13, AC-14, AC-15) ran on the `ze` Colima profile; its
evidence is in the spec under Implementation Steps, Phase 6, and A-6 and A-13
are confirmed there.

| Commit | What |
|--------|------|
| 63d1b8561d | spec: AC-10 evidence |
| 7a98e14ab6 | vpp-iface-test: the uRPF and LCP fixtures put `address` under `unit` (the schema has it under `ipv4`), so uRPF failed on every run; and a Ctrl-C made `awaitFeature` poll on and print a FAIL. Fixed with `TestAnInterruptedScenarioEndsWithTheInterruption`; the full proof now exits 0 on the `ze` profile |
| (next) | spec Phase 6 evidence, the sibling-spec pointers, the M1 journal row for the remaining Alpine `Run` guests (`gate-verdict-depends-on-the-machine.md`) |

Still uncommitted, this spec's: `internal/le/interoplab/kernelcheck.go`,
`kernelcheck_test.go` and the matching `docs/architecture/testing/interop.md`
hunks (row ~733, anchor ~895). The same page carries the rrdep session's hunk
(~line 2237) describing `test/interop/scenarios/bgp-addpath-departure-reflector-frr`,
which is untracked and not in HEAD, so the page cannot be committed until that
session lands its scenario. Its `le-rrdep` process was still running at 19:50.
Commit the code and the page together once their hunk is in HEAD, never the
code alone.

Before the review gate: the gates in item 6 plus `./le test weakened check`;
AC-9 amd64 (nightly); handover item 4 (`Hugepages.applianceEnv`); no test parses
the vpp-iface scenario configs through the schema, which is how the `address`
fixture defect stayed hidden (owed: a parse test over every scenario config).
