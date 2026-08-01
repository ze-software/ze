# 1024 -- Pure-Go installer initrd: QEMU test infrastructure, closed WITHOUT QEMU verification

## ⚠️ Closure status (read first)

This spec was **closed by explicit user decision WITHOUT running the QEMU
functional gates green.** The dev machine has no operator-supplied installer
kernel (`ZE_INSTALL_KERNEL`) and lacks `grub-mkstandalone` / `xorriso` /
`mtools`, so none of the install QEMU gates can run here -- they all self-skip.
Everything in this entry is **implemented and statically verified (build, vet,
unit, lint, code review)** but the QEMU acceptance criteria (AC-2/3/4/5/7/7b/7c,
the R-6 fault gate, the both-arches goal gate) are **NOT green-proven.** The
Goal Validation table in the spec reflects this honestly. Do not treat the
installer as field-validated until the gates below pass on a kernel box.

## Context

The pure-Go PID-1 installer initrd (`cmd/ze-installer`, `internal/install/disk`)
was implemented and committed earlier (`ed5636f65`, `faabc1cbb`), replacing the
1176-line busybox `/bin/sh` initrd. What was missing was the QEMU evidence the
spec's "Test Infrastructure to Build" table demands: the happy-path HTTP/ISO
tests existed, but not the failure-path, multi-NIC pin, rescue-console, arm64
ISO, or Ventoy scenarios. This session built that infrastructure.

## What was built

- **R-6 goroutine-panic recovery**: `internal/install/disk/fault_linux.go` (build
  tag `ze_installer_fault`) injects a real runtime fault (nil-map write) in a
  goroutine guarded by `recover -> fatalInitrd`; `fault_stub_linux.go` is the
  no-op so the shipping initrd carries no fault code. `ZE_INITRD_FAULT=1` in
  `defaultInitrdMakeBuild` builds the variant through the production packer.
- **QEMU scenarios** (`scripts/evidence/effective-install-scenarios-qemu.py`):
  fault, `pin-ac4` (pin + foreign NIC never up), `pin-ac5` (flush + recover),
  `rescue-ac7/7b/7c` (a `select`+`os.read` serial expect driver). Plus the
  **Ventoy** harness (`effective-install-ventoy-qemu.py`) and **arm64** support
  in the ISO harness. Wired to `make ze-install-scenarios-qemu-test` /
  `ze-install-ventoy-qemu-test`.
- **AC-11 packer test**: extracted `writeInitrdPack` from `defaultInitrdMakeBuild`
  and added `internal/appliance/initrd_pack_test.go` (gunzip + newc round-trip).

## Reusable gotchas (the point of this entry)

- **A backgrounded `make ... > log; echo "exit=$?"` reports the ECHO's exit, not
  make's.** The `;`-chain returns the last command's status, so a failing `make`
  looked like "exit 0". This masked a red `ze-unit-test` for a full round.
  **Always grep the log for `FAIL`, or put `$?` inside the redirect / check make
  directly** -- never trust a trailing-echo exit code from a background job.
- **The initrd cache key ignores build tags** (`internal/appliance/cache.go:121`
  `initrdCacheVariant` keys on version+arch+a 4-file source hash only). A fault
  build and a normal build collide on the same cache path, so a fault initrd can
  be served a cached normal one. The harness isolates them with a per-variant
  `XDG_CACHE_HOME`. If you ever add a real build-tag dimension to the initrd,
  fold it into the cache key instead.
- **Multi-NIC pin determinism in QEMU slirp**: put `ze.server` on the *recovery*
  NIC's directly-connected subnet (10.0.2.0/24). The connected route to it always
  beats any default route the pinned/foreign NIC holds, so reachability does not
  depend on default-route metric ordering after `fallbackDHCP` re-leases every
  NIC. The unreachable NIC uses a different subnet + `restrict=on` (still gets a
  slirp lease + default route, so the flush path fires, but can never carry host
  traffic).
- **A bootstrap installer legitimately resolves interfaces directly.** The
  no-direct-resolution gate (`scripts/checks/iface_resolution.go`, run inside
  `ze-verify`) flagged the installer's `netlink.LinkByName`. The fix is an
  allowlist entry, not a resolver call: the initrd must NOT pull the `iface`
  component (it is a self-contained PID-1 binary), exactly like the existing
  `internal/plugins/provision/` PXE-time exemption. This gate had been red since
  `ed5636f65` because that commit added the installer without the exemption.
- **Validate arch-specific QEMU device wiring without a full boot**: `qemu-system-
  aarch64 ... -device virtio-scsi-pci -device scsi-cd,... <timeout>` reaching the
  UEFI firmware (exit 124 from `gtimeout`, no device error) proves the attachment
  parses on `virt`, even with no bootable media. arm64 `virt` has no IDE bus, so
  the ISO cdrom must be virtio-scsi, not `if=ide`.

## Mistakes / process notes

- Initially trusted a background job's "exit 0" summary and reported the unit
  suite green when `ze-unit-test` was actually red (see gotcha 1). Caught on the
  next round by grepping the log. Lesson reinforced: read the actual log content,
  not the wrapper's exit.
- The arm64 ISO harness was first made host-arch-default, which silently shifted
  the proven amd64 default on arm64 dev boxes. Reverted to amd64-default with
  arm64 opt-in (`ZE_INSTALL_ARCH=arm64`) after code review, since arm64 ISO is
  unproven.

## Remaining validation (MUST run on a kernel box before trusting the installer)

```
ZE_INSTALL_KERNEL=/path/to/vmlinuz make ze-install-scenarios-qemu-test
ZE_INSTALL_KERNEL=/path/to/vmlinuz make ze-install-ventoy-qemu-test
ZE_INSTALL_ARCH=arm64 ZE_INSTALL_KERNEL=/path/to/Image make ze-install-iso-qemu-test
```

First-run risk points, most to least likely: (1) `nclient4` DHCP behaviour in
slirp (pin scenarios; spec assumption A-3 is unvalidated); (2) the `mon:stdio`
keystroke path in the rescue driver (QEMU-version-sensitive); (3) `mtools`
`mformat -F` flags + `vdb` device ordering (Ventoy); (4) arm64 grub `arm64-efi`
availability + arm64 installer kernel (ISO).

## Files

None recorded.
