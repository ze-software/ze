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
- Optical drives are searched as source media. They remain excluded from disk
  targets; media discovery does not reuse that target-only optical exclusion.
  <!-- source: internal/install/disk/iso.go -- findISOMedia, isISOMediaDisk -->
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

ISO write completion is reported after synchronization and a successful kernel
partition-table reread. A reread failure keeps the installer in its recovery
path instead of claiming that finalization succeeded. The ISO QEMU proof waits
for a clean VM exit before it counts the poweroff; completion text by itself
does not satisfy that check.
<!-- source: internal/install/disk/run.go -- runISO, finishISOWrite -->
<!-- source: internal/le/test/qemu/install_boot.go -- runCaptureUntilExit -->

**`grub-mkstandalone` is `grub2-mkstandalone` on Fedora and RHEL.** The builder
resolution tries both names.

Checks and builds share GRUB executable admission. A custom `--builder` path
must exist and be executable, even when its matching EFI modules are installed.
An invalid builder reports the complete-builder repair with the selected
appliance name and directory. The check still reports xorriso after rejecting
a builder.
<!-- source: internal/appliance/cmd_iso.go -- resolveISOGRUBBuilder, checkISOPrerequisites, resolveISOBuilder -->

The standalone executable and its selected EFI module set are separate
prerequisites. `ze appliance iso --check <name>` checks the named appliance's
architecture; without a name it checks the host architecture. Building checks
the same module set before staging the image. On Debian and Ubuntu the module
packages are `grub-efi-amd64-bin` and `grub-efi-arm64-bin`, respectively. Other
distributions use their matching GRUB EFI module package. The existing
`doctor-appliance-grub` check also reports missing host-architecture modules.
The check prefers modules beside the selected builder, including its resolved
symlink target, before standard installation prefixes. The admitted directory
is retained through ISO assembly and passed to GRUB with `--directory`, so a
custom builder cannot silently use a different compiled-in module location.
<!-- source: internal/appliance/iso_modules.go -- checkISOGRUBModules -->
<!-- source: internal/appliance/cmd_iso.go -- checkISOPrerequisites, resolveISOInput -->
<!-- source: internal/appliance/doctor_checks.go -- checkGrubBinary -->

Missing modules produce a typed operator error with a repair command. On a
Debian or Ubuntu build host whose CPU architecture matches the selected target,
it names the `grub-efi-amd64-bin` or `grub-efi-arm64-bin` apt package. These
are native architecture packages, so a supplied foreign image instead gets
the complete-builder override below. Ze does not change the host's multiarch
configuration. On Fedora and Rocky
Linux, it names `grub2-tools-extra` and the matching
`grub2-efi-x64-modules` or `grub2-efi-aa64-modules` dnf package; those module
packages are noarch. The host is
identified from `/etc/os-release`; Ze does not guess a package manager on an
unrecognized host. Doctor retains the same repair and underlying failure.
The package names are published by [Fedora](https://packages.fedoraproject.org/pkgs/grub2/)
and [Rocky Linux](https://download.rockylinux.org/vault/rocky/10.1/BaseOS/x86_64/os/Packages/g/).
Debian lists the package architectures for
[amd64 modules](https://packages.debian.org/stable/grub-efi-amd64-bin) and
[arm64 modules](https://packages.debian.org/trixie/grub-efi-arm64-bin).
<!-- source: internal/appliance/iso_grub_repair.go -- isoGRUBInstallCommand, isoGRUBHostInstallCommand, isoGRUBModuleError -->

On another build host, supply a complete GRUB installation, including readable
modules for the selected EFI target beside the executable. The printed override
requires `GRUB_MKSTANDALONE` to contain its absolute executable path; this is a
shell variable for the command, not a Ze environment setting. For example:
<!-- source: internal/appliance/iso_grub_repair.go -- isoGRUBOverride, isoGRUBSelectionError -->

```bash
GRUB_MKSTANDALONE=/opt/grub/bin/grub-mkstandalone
./ze-host appliance iso --check --builder "$GRUB_MKSTANDALONE" lab
./ze-host appliance iso --builder "$GRUB_MKSTANDALONE" lab
```
<!-- source: internal/appliance/cmd_iso.go -- runIso, resolveISOInput -->

The override selects a different complete builder and module installation. It
does not install missing modules. A host without such an installation needs
the supported Linux build-host path described above. A builder that already
returns a typed operator error keeps its own repair command when ISO assembly
reports the failure. Invalid names in both prerequisite checks and ISO builds
report appliance selection in the same quoted directory. An extra name reports
help. Unreadable named configuration also reports the selected appliance
directory, so a repair does not silently switch to the default directory.
Named complete-builder fallback repairs preserve the appliance name and its
quoted `--dir`, including when the image architecture differs from the host.
Doctor's unnamed host-architecture check remains unnamed.
<!-- source: internal/appliance/iso_grub_repair.go -- isoGRUBSelectionError -->
<!-- source: internal/appliance/cmd_iso.go -- runIso, isoNameError, resolveISOInput, checkISOPrerequisites -->

The ISO QEMU proof discovers QEMU's bundled edk2 firmware (Homebrew's
`share/qemu`) and Debian's split OVMF 4M and AAVMF firmware. Split CODE/VARS
images use two pflash drives: code is read-only and variables use a private
snapshot, so a proof cannot change the host's firmware files. The variable
image is the one QEMU's own descriptor in the `firmware/` directory beside the
code image names (`edk2-aarch64-code.fd` pairs with `edk2-arm-vars.fd`,
`edk2-x86_64-code.fd` with `edk2-i386-vars.fd`), then the OVMF/AAVMF rename
(`_CODE` to `_VARS`, `-code` to `-vars`). A split image with no variable image
present is refused. A custom complete firmware image retains the `-bios` route.
<!-- source: internal/le/test/qemu/install_iso.go -- findUEFIFirmware, isoFirmwareArgs, isoFirmwareVarsCandidates -->

**The two ISO tools share a 10-minute limit (`isoBuildTimeout`).** A tool that
fails or runs out of it is named with its exact command line, the tool path
and every argument quoted for a shell, so a path with a space runs as written.
The limit is worded as every build limit is (`kernelbuilder.LimitWords`). The
command reads the staging directory, which the build removes, so to run it by hand,
run `ze appliance iso --keep-staging` first.

<!-- source: internal/appliance/cmd_iso.go -- isoBuildTimeout, runISOBuilder, isoToolFailed -->

**Names embedded in the GRUB kernel command line must survive two parsers.** A
dedicated regex accepts `[a-zA-Z0-9._-]+` only, so spaces, quotes, and shell
metacharacters cannot reach the bootloader.

**PXE and ISO share one initrd binary and diverge at the source branch.** A
change to a shared function, disk detection or checksum validation among them,
must be tested in both modes.

**arm64 uses the `arm64-efi` GRUB target and `bootaa64.efi`.** An architecture
mismatch between the kernel and the appliance config is rejected at build time.

## Evidence

`internal/le/test/qemu.Answer` proves the whole chain: ISO
boot, image write, no PXE ZeFS injection, power off rather than reboot, and an
SSH login on the installed disk with the embedded credentials. The installed
disk boots with the same UEFI firmware, answers an authenticated `show version`
command, and the extra untargeted disk is checked byte-for-byte by its hash.

<!-- source: internal/le/test/qemu/actions.go -- Answer -->
<!-- source: internal/le/test/qemu/install_iso.go -- executeISO -->
<!-- source: internal/le/test/qemu/install_boot.go -- bootTargetSSH, sshLoginOK -->

## Related

- `installer-initrd.md` for the PID 1 binary and the host-versus-target rule
- `build-artifacts.md` for kernel and initrd resolution before an ISO build
