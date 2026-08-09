# Bootable ISO installer

`ze appliance iso <name>` wraps an already-built appliance image in a bootable
ISO. The ISO is a transport envelope, not a new image format: the installer
writes the raw image bytes unchanged, so gokrazy's A/B partition layout, its
boot references, and the ZeFS credentials already injected into `/perm` all
survive. The initrd reconstructs no partitions.

<!-- source: internal/appliance/cmd_iso.go -- staging tree, GRUB config, EFI image, builder invocation -->
<!-- source: internal/install/disk/iso.go -- findISOMedia, tryVentoyISO, checkISOContent -->

## Decisions

- Extend the one initrd with an ISO source mode rather than ship a second init.
  Disk safety, checksum validation, and success marking are shared concerns.
- ISO mode does not fetch or write a separate `database.zefs`, unlike PXE.
  `ze appliance build` has already injected ZeFS into the image's `/perm`
  partition.
- A missing `.sha256` sidecar is a hard error, with no override flag. An ISO is
  a durable offline artifact, and enforcement is what stops corrupt media from
  being handed around.
- Implicit target selection is refused when the machine has more than one fixed
  disk. PXE picks the first disk; an ISO install runs on multi-disk machines,
  where a silent pick is dangerous.
- The source medium is identified by a 16-byte random `ze.media-id` marker file,
  so the initrd finds and excludes the boot medium across CD-ROM and USB device
  names alike.
- The image is gzip-compressed into the staging tree and decompressed during
  install. The installed bytes do not change.
- UEFI boot goes through `grub-mkstandalone` and `xorriso`, not a custom EFI
  stub. GRUB owns the firmware handoff and xorriso emits standards-compliant
  ISO 9660.
- `--target` embeds an explicit `ze.target=/dev/vda` in the GRUB kernel command
  line, which locks the install target at ISO creation time for unattended
  deployment.

## Traps

**The installer must power off, not reboot.** A reboot lands back in the
removable medium and runs the installer again.

<!-- source: internal/install/disk/system.go -- doPoweroff and doReboot -->

**`grub-mkstandalone` is `grub2-mkstandalone` on Fedora and RHEL.** The builder
resolution tries both names.

**Names embedded in the GRUB kernel command line must survive two parsers.** A
dedicated regex accepts `[a-zA-Z0-9._-]+` only, so spaces, quotes, and shell
metacharacters cannot reach the bootloader.

**PXE and ISO share one initrd binary and diverge at the source branch.** A
change to a shared function, disk detection or checksum validation among them,
must be tested in both modes.

**arm64 uses the `arm64-efi` GRUB target and `bootaa64.efi`.** An architecture
mismatch between the kernel and the appliance config is rejected at build time.

## Evidence

`scripts/evidence/effective-install-iso-qemu.py` proves the whole chain: ISO
boot, image write, no PXE ZeFS injection, power off rather than reboot, and an
SSH login on the installed disk with the embedded credentials.

<!-- source: scripts/evidence/effective-install-iso-qemu.py -- end-to-end ISO proof -->

## Related

- `installer-initrd.md` for the PID 1 binary and the host-versus-target rule
- `build-artifacts.md` for kernel and initrd resolution before an ISO build
