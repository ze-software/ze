# Handover 15: ze-image-docker-host, Phase 3 done

Spec: `plan/spec-ze-image-docker-host.md` (Status in-progress). Written
2026-10-21 by the Phase 3 continuation agent. Supersedes handover 14 for open
items. Every outcome below is recorded in the spec under Implementation Steps,
Phase 3, with its pasted evidence.

## Phase 3 commits

| SHA | What |
|-----|------|
| 22e1b9e8e7 | `./le setup docker-host build`, `start`, `stop`, `check`; tests; docker-host.md and developer-setup.md sections; AC-5 evidence |
| 7aef71a19e | Lima's user-data `mounts` mounted at boot (`/mnt`, `/Users` tmpfs first); `Users` in the image's extra dirs |
| d77624fe86 | `ExecuteGok` restores the working directory gok leaves in a removed instance dir |
| 54fb7e9a1f | `binfmt_misc` mounted at boot, so Colima registers Rosetta (amd64 `hello-world` runs) |
| 39a2c5caeb | `/mnt` and `/Users` remounted read-only after the shares, so Colima's data-disk binds never put Docker on a tmpfs |
| (this commit) | spec evidence for AC-2, AC-3, AC-5 (machine id), AC-6, AC-7, A-11; this handover |

## Proven on the arm64 Mac (Colima, vz, profile `ze`, image key 6039173623982fd0)

- AC-7: K1 to K2 to K3 to K4 each printed and ran `colima stop`, `colima delete
  --force`, `colima start` with `--profile ze`, action `recreated`; an
  unchanged key with the profile running answered `action none`.
- AC-2 re-checked on K4: `7.2.9-ze`, `overlayfs`, cgroup 2, root
  `/perm/var/lib/docker`; user and sudo `docker info` in the guest.
- AC-3: `docker-kernel check` passes on the `ze` socket; the probe's 20 rows
  are all `present`, `ipsec-mobike`, `l2tp`, `l2tp-ppp`, `mpls`,
  `mpls-transit-mtu` included.
- AC-6: machine id equals `/perm/machine-id`, survives stop and start, boot id
  changes; four recreated instances, four ids.
- AC-5: the image's root squashfs has no `etc/machine-id`.
- A-11 confirmed: a pulled image survives stop and start.
- `default` profile line identical before and after every run.

The `ze` profile is left Running on image 6039173623982fd0.

## Open items, each owed

1. Phase 4 (AC-8): on darwin, labs target `~/.colima/ze/docker.sock` through
   `dockerhost.Environment`, with no fallback to `default` or to the current
   Docker context; the kernelcheck route and the AppArmor-colima text; spec
   docs rows 6 and 10 in the same phase.
2. Phase 6 (A-6, AC-15): Rosetta now registers; run the amd64 VPP image there.
   Colima still warns `unable to enable qemu aarch64 emulation: binfmt not
   found` (QEMU user emulation, which Rosetta replaces here).
3. `./le setup docker-kernel check` prints no rows on a pass, so AC-3's rows
   came from running its probe container by hand.
4. `start` prints `recorded-key` as the key recorded before the run, beside
   `key`; a reader can take it for the record after the run.
5. Colima's 100 GiB data disk stays unused (no `sfdisk`/`mkfs.ext4` in the
   guest); Docker's data lives on `/perm` (about 15 GiB in a 16 GiB image).
6. amd64 image never built (handover 14 item 3).

## Gates owed (not run here: over five minutes, or refused)

`./le go lint run` (the scoped linux/arm64 golangci-lint over
`internal/appliance/dockerhost` after 39a2c5caeb was refused twice by another
session's parallel golangci-lint; every earlier scoped run reported 0 issues),
`./le test unit all`, `./le verify worktree`. Commits carried
`structural-red-ok`: staticcheck parts 2 and 3 have been red since the 12:27Z
verify, before this phase.
