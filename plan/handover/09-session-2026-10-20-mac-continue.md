# Handover: Linux session d8e7dbbf to the Mac (2026-10-20)

The Linux session moves to the Mac. Everything it committed is on `origin/main`.
Detailed history of the lab-cleanup work: `plan/handover/06-lab-docker-resources-reaped.md`.

## Work for the Mac

`plan/spec-lab-docker-cleanup-on-macos.md` (status ready) exists for exactly this
machine. Its acceptance criteria need macOS, Colima, Docker Desktop and HVF:

| AC | What the Mac proves |
|----|---------------------|
| AC-1 | `kern.bootsessionuuid` and `IOPlatformUUID` readable without sudo; paste both values into the spec |
| AC-2 | A perf DUT's `ze.lab.*` labels carry the Mac's machine id, boot id, `darwin`, the live `le` pid, no pid-ns |
| AC-3 | After a reboot, `./le test integration cleanup dry` lists a killed run's resources as "would remove", and `cleanup` removes them |
| AC-4 | `TestKilledLabRunIsCleanedByNextStart` (`-tags docker`, `ZE_REQUIRE_DOCKER=1`) passes on Colima and on Docker Desktop |
| AC-5 | `./le test qemu docker-lab` with interop-ipsec under HVF: start sweep summary, labeled creates accepted by the guest dockerd |
| AC-6 | One Ctrl-C on `./le perf run dut ze` leaves no container, network or retained tag of that run |
| AC-7 | FreeRtr image build on both daemons carries `ze.lab.kind=image` and leaves no retained tag |
| AC-8 | One Ctrl-C on the `vpp-test` and `vpp-iface-test` deployment proofs: exit 1, no verdict line, nothing of the run left |

The exact commands and expected output are in the spec's AC table; run them as
written. The spec must be claimed on the Mac (`./le spec claim spec
spec-lab-docker-cleanup-on-macos.md`).

## Still open, not Mac work

| Item | Blocked on |
|------|-----------|
| `plan/spec-lab-docker-cleanup-and-go-test-containers.md` | A green full verify run, and the first nightly run carrying `docker-peers` / `live-rpki` (AC-25). Review gate is CLEAN at round 10 |
| `plan/spec-interop-image-copies-a-prebuilt-ze.md` AC-10 | `./le verify current mode full` passing |
| Verification debt of the specs this session closed (LDP keepalive expiry, BFD link-local, BGP prefix-SID, boot record after listen) | The full suites. The owner reserved them to another session; verify is STALE since the 2026-10-19T14:57Z failure |
| `plan/spec-lab-kernel-features-in-setup.md` (vhost-net and modprobes in the Linux setup) | Owner: bucket `plan/` or `plan/pre-release/`; Linux work |
| RFC 4761 Section 3.5 VPLS reclassification | Owner ruling |
| LDP spec AC-6 rewording (spec already closed) | Owner acceptance |
| 208 untested RFC rows | Absent features, other roles, or awaiting rulings; RFC scope is frozen |
| Round 10 NOTEs N2 and N6 (`setup docker-kernel install` stop wording and second Ctrl-C) | Rows in `plan/journal/stop-leaves-the-work-running.md`, not fixed |

## Done this session (for orientation)

- G-2: every lab verb stops its docker children on Ctrl-C/SIGTERM; 23 fix commits, review rounds 6 to 10, real Ctrl-C proofs recorded in handover 06.
- Specs closed: LDP keepalive expiry, BFD link-local peer zone, BGP prefix-SID, boot record after listen.
- RFC tests with both polarities and recorded mutant proofs: OSPF RFC 8362, RPKI 8210bis, LDP RFC 3031, EAP RFC 7627.

## Ctrl-C forwarding agent 2026-10-21

Status: round 1 is implemented and uncommitted. The review fixes are NOT started, because the first agent hit its 100-call budget. A fresh agent carries the same package.

Round 1 change (uncommitted, live tree):
- `internal/le/test/qemu/run_guestcommand.go` and `run_guestcommand_test.go` (both new)
- `run.go`: `commandSpec.OwnGroup`, `runOps.Interrupts`/`notifyInterrupts`, `Run.teardownGrace`
- `run_exec.go`: QEMU `Setpgid`, `sshSession`
- `dockerlab.go`: comment only
- `fakeqemu_test.go`: console mode
- `docs/architecture/testing/qemu-integration.md`: new section "Stopping a guest command"

Real run 5 on the Mac, 2026-10-21 03:13: one `kill -INT -<le pgid>` once the DUT and sender ran (run id QK45GSN2KZXGVVHAGW6AZ3EBAD).
- Guest output: `SIGINT sent to guest process group 1514`, then the guest le's `measure ze: ... canceled`.
- Host output: `it finished its own teardown and exited 1, and the VM was then stopped`, `QEMU VM: FAIL (exit code 1)`, host le exit 1.
- Not observed: an empty guest `docker ps` before the VM stopped, which is why the review asks for an AC-6 leftover line.

Review round 1 to-do. Tests first. Touch only `internal/le/test/qemu` and `qemu-integration.md`; ask the coordinator before touching anything else.

- **B1, a VM must never outlive its le.**
  - Add SIGHUP to the qemu roots: `guestContext` (guestlabs.go), the `signal.Notify` in `actions.go` runQEMUHere and runInstallerHere, and `notifyInterrupts`.
  - Linux: set `Pdeathsig` on QEMU and on the ssh clients. The field is linux-only, so it needs a `_linux.go`/`_other.go` helper pair that builds the `SysProcAttr`.
  - macOS: record `{qemu pid, qemu start time, le pid, le start time}` under `tmp/qemu/`. At the start of the next run, reap a QEMU only when the recorded le is gone (pid dead, or its start time differs) AND the QEMU pid's start time still matches the record. Remove the record when `stopVM` runs.
  - Tell the coordinator, without editing `internal/le/job`, that the 3 s stall-breaker grace in `internal/le/job/registry.go` is shorter than the guest teardown: 2 min of grace plus the 10 s `shutdownGrace`.
- **I2.** Replace `trap : INT` with `trap 'exit $?' INT`, set before the pid file is written.
  - Check in busybox ash that `a; b` stops after `a`.
  - The AC-6 leftover report below has to survive this exit, so emit it from an EXIT trap inside `dockerLabCommand`.
- **I3.** In actions.go runQEMUHere, print `runErr` (`leaction.ReportError`) before returning `signalExitCode`.
- **I4.** Add a test where `Options.Timeout` expires (for example 200ms, no cancel). Assert "command timeout" in the error and that the interrupt ssh ran.
- **N6.** In `runGuestCommand`, after `limit.Done()`, check `finished` without blocking before forwarding.
- **N7.** Write the pgid at startup (`cut -d' ' -f5 /proc/$$/stat > file`), and make the interrupt line `kill -INT -$(cat file)`.
- **N9.** Give `waitForSSH` `OwnGroup: true`, or fix the doc sentence.
- **N10.** In fakeqemu_test.go the new `const fakeQEMUConsole` sits between `fakeQEMUProcess`'s doc comment and the function. Move it above the comment.
- **N11.** A Ctrl-C during the dockerd wait must not print "dockerd did not answer". In `stopGuestCommand`, an ssh exit 255 after the forward must say "ssh failed", not "the guest command exited 255".
- **AC-6 evidence.**
  - Make `dockerLabCommand` print, after the lab ends (normally or interrupted) and before the VM stops, what the guest dockerd still holds by label `ze.lab`: containers, networks, and images. For images, mind AC-6: the kept DUT image under its cache tag is allowed, so report only `ze-interop-retained:` tags, or list kept images separately. End with a line such as `interop guest: left behind: none`, or the list.
  - Test it, then repeat the real run and show that line.

Real-run notes:
- Use `ZE_QEMU_SSH_PORT=24871`: 2222 and 2290 are taken by other sessions' QEMUs.
- Launch with `bash -c 'set -m; ./le --name fwd test qemu docker-lab lab "perf run dut ze" timeout 3600s > log 2>&1 & echo pgid=$!; wait'`.
- The snapshot helper is `scratch/snap.sh <label>` in the parent session's scratch.
- The hook blocks any command that names `/tmp` (even a guest path) or that contains `go build` in a grep pattern.
- Runs 1 to 3 failed on the cold guest go-cache: the 5-min lab build bound, then `linuxBuildTimeout` twice. The warmed export cache was deleted, so expect retries. The coordinator journals this.

Gates owed after the fixes:
- `./le go lint run` (qemu)
- `-race` unit run of `./internal/le/test/qemu/`
- `./le doc check verify`
- `./le test weakened check`
- `./le go vet-platforms darwin` (and linux, for the Pdeathsig files)

This file is uncommitted, and the coordinator commits it.
