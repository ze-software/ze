# Handover: Ze's image as the lab Docker host (Colima `ze` profile and QEMU guest)

Written 2026-10-21 by session f2de520e on the owner's Mac, for a fresh session.
It opens a new spec. `plan/spec-lab-docker-cleanup-on-macos.md` stays
in-progress and waits on it.

## Owner decisions (all 2026-10-21)

- M10 in `ai/META-DECISIONS.md`: a lab's Docker daemon runs on Ze's kernel and
  Ze's system. Colima is fine as the macOS VM manager, but only booting Ze's
  image, never its stock Ubuntu VM or Alpine. Ubuntu's modules (`modprobe`)
  would supply L2TP and MPLS, "still we want our kernel".
- "Yes to 1, 2 and 3":
  1. A separate `ze` Colima profile boots Ze's image. The `default` profile,
     which other sessions use, is left untouched.
  2. One image serves both that Colima profile and the QEMU `docker-lab`
     guest. That guest drops Alpine.
  3. The Docker Desktop halves of AC-4 and AC-7 are dropped.

## Facts established

- Colima 0.10.3 (Lima 2.2.1, vz, aarch64) boots a custom image with
  `colima start --disk-image <file>`. `--force-disk-image` is needed for an
  image Colima does not recognize.
- What Colima needs inside the VM to start and reach it has not been checked
  (the Lima guest agent, how it reads its boot-time config, the docker socket
  forward). This is the first risk the spec proves.
- The QEMU `docker-lab` guest already boots Ze's runtime kernel 7.2.9. Its
  docker kernel check passes there. Alpine supplies only init, `apk` and
  dockerd 27.3.1 (`internal/le/test/qemu/dockerlab.go`,
  `run_iso.go`).
- On Colima's stock kernel 6.8.0-117, with `ze-kernel-probe` loaded by the
  owner, `./le setup docker-kernel check` passes AppArmor. It then refuses on
  five absent features: `ipsec-mobike` (no `XFRM_MSG_MIGRATE_STATE`, which
  needs Linux 7.2, so modules cannot fix it), `l2tp`, `l2tp-ppp`, `mpls` and
  `mpls-transit-mtu`.
- Alpine userland makes guest lab labels carry an empty machine id and boot
  id, and no `ze.lab.pid-ns` (journaled under M1). Ze's system has to give
  real ones.

## What closes the waiting spec once the image exists

Run these on the `ze` Colima profile:

- AC-2 and AC-6: `./le perf run dut ze`, then a Ctrl-C.
- AC-8: `vpp-test` and `vpp-iface-test`. The kernel needs `CONFIG_VHOST_NET`
  (`plan/spec-lab-kernel-features-in-setup.md`).

Then run the review gate and the two-commit close.

## Still owed by the waiting spec

- `./le go vet-platforms darwin` and `./le doc check verify` across this
  session's commits.
- The owner may delete `~/.config/docker/config.json.bak-ze-buildx-2026-10-21`.
