# Handover: ze-image-docker-host, continue on the Linux host

Written 2026-10-22 by session e1f02c94 on the owner's Mac, for a fresh session
on the Linux amd64 host. It supersedes handovers 13 to 16 of this spec, which
stay as history. Spec: `plan/spec-ze-image-docker-host.md` (in-progress). The
claim is released; the next session claims it with
`./le spec claim spec plan/spec-ze-image-docker-host.md`.

## State in one paragraph

One gokrazy image on Ze's runtime kernel (`7.2.9-ze`) carries dockerd,
containerd and the Colima/Lima guest contract. On the Mac it boots as the `ze`
Colima profile (left Running there, `default` untouched), labs on macOS target
only its socket, and `./le setup docker-host build|start|stop|check` builds and
wires it. `./le test qemu docker-lab` boots the same image under QEMU and drives
its dockerd from the host le; the Alpine docker-lab path is deleted. Every AC is
proven on arm64 except AC-9's amd64 half. Review round 1 (7 ISSUE) is fixed;
round 2 found 3 ISSUE and 5 NOTE, not yet fixed.

## Owner decisions taken in this session

| Decision | Where recorded |
|----------|----------------|
| Colima's guest contract is met with static third-party binaries (busybox, OpenSSH, sudo, docker CLI) and a Go init plus `systemctl` shim | spec Task, `-> Decision` |
| OpenSSH, sudo and iptables are built as static musl binaries in a digest-pinned Alpine container that is only a build tool (D-13) | spec Task, A-12 |
| M13: kernel work uses the machine's own architecture; the other arch's proof comes from the host that runs it natively | `ai/META-DECISIONS.md` (3b20d9b04d) |
| The waiting `plan/spec-lab-docker-cleanup-on-macos.md` keeps AC-5 as written (exit 0) and stays open until `ike-padded-probe-strongswan` and `mobike-initiator` pass | that spec, deb4a223e7 |

## What the Linux session does, in order

| # | Item | Detail |
|---|------|--------|
| 1 | Fix review round 2 findings | R2-1: restore the docker CLI preflight before `RunOneShot` in `runBuilderContainer` (`internal/le/setup/dockerhost/static.go`) with the install fix in the message, test with an empty PATH. R2-2: extract the restart-refusal marker check in `RunGuest` (`internal/appliance/dockerhost/guest_linux.go`) into a function taking the marker path, unit-test both branches. R2-3: a fake docker-lab run without the `/tmp` root expecting the share refusal before QEMU starts. NOTEs R2-4 to R2-8 (test half that proves nothing in `supervise_test.go`, socket path text in `actions.go` help, `dockerhost.go` package doc and `developer-setup.md`, the spec's builder-container annotation, `homeRoot` used for the bind in `boot.go`, the restart hint naming both recoveries). Findings in full: the spec's Review Gate section |
| 2 | Review round 3 | Fresh context, scope: the fixes for R2-1 to R2-3 only (write the scope into the spec first). Five rounds are the session's; round 4 and 5 remain |
| 3 | Fix the free-port check | `freeRunSSHPort` (`internal/le/test/qemu/run.go`) probes with a wildcard bind, which succeeds while another VM holds `127.0.0.1:<port>`; the run then dies with "sshd did not answer" and the QEMU error file is deleted with the run directory. Probe the address QEMU binds, keep `qemu-errors.log` on failure. Journal row: `plan/journal/parallel-copies-collide-on-a-deterministic-port.md` (rewritten 6910274273) |
| 4 | AC-9 amd64, natively (M13) | `./ze-host appliance kernel --target runtime --arch amd64`, then `./le setup docker-host build arch amd64` (builds the static OpenSSH, sudo and iptables for x86_64; pin any amd64 artifact missing from `gokrazy/docker-host/binaries.json`), then `./le test qemu docker-lab` under KVM: the kernel check must pass against the forwarded socket. Then one lab, e.g. `./le test qemu docker-lab lab "test integration interop"`. Paste the evidence into the spec's AC-9 row. The amd64 config already resolves every AC-4 symbol to `=y` (7ed46cac0d) |
| 5 | Full gates | `./le verify current` and `./le test unit all`; they need about 35 GB free. Classify each red as caused by this spec's commits (list below) or another session's; fix ours at the cause |
| 6 | Close | `/ze-close` in a fresh context (review artifact via `./le spec review record`, two commits from one script). The nightly's amd64 run after the owner's next push is the recurring proof; it is not a precondition once step 4 has run on the Linux host |

## Not for the Linux host

- The `ze` Colima profile lives on the Mac. Nothing on Linux touches it.
- `plan/spec-lab-docker-cleanup-on-macos.md` AC-5 runs "on the Mac with HVF"; it
  waits on the two IPsec bugs, whose fixes can be made anywhere.

## Evidence the next machine can reach

- Spec commit 38622cb4f1; Phase 1 81fab69967, fcfb8cbd73, 72fc76c29b; Phase 2
  a61dc558a8, 78c7ebcca3, 612b4a832e; Phase 3 22e1b9e8e7, 7aef71a19e,
  54fb7e9a1f, 39a2c5caeb, df9757f09c; Phase 4 af4c0708ed; Phase 5 08f66e5450,
  ca386b6ae3, 63d1b8561d; Phase 6 7a98e14ab6, efe381cedc; amd64 config
  e6fc0d382e, 7ed46cac0d; f5ea135dd3 (`ZE_DOCKER_LAB_GUEST` removed); review
  round 1 fixes a02b362081, 2b98d538dc, 02fa09c15f, c0b69d7a53, b5079f335a,
  fa2b697807, 2cb1eb5f1c.
- AC evidence lines are pasted in the spec under each phase.

## Known blockers on the way

- Every commit since the afternoon needed `structural-red-ok`: staticcheck parts
  2 and 3 are red on other sessions' code (an ExaBGP bridge test missing its
  build tag, journaled in `plan/journal/feature-test-missing-build-tag.md`; a
  syntax error in another session's uncommitted RIB test). Debt rows sit in
  `plan/verification-debt/`.
- Commit scripts left a stale `.git/index.lock` four times on 2026-10-21/22,
  each time leaving the shared index on pre-commit copies. Recovery is the
  `git ls-tree HEAD -- <paths> | git update-index --index-info` line the script
  prints, run by the owner. Class already journaled
  (`plan/journal/concurrent-session-corruption.md`).
- The Mac's disk repeatedly filled to under 2 GB from builds, VM images and
  caches; the full verify alone needs about 25 GB.
