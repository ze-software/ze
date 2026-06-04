# 854 -- install-8-appliance-iso

## Context

Ze appliances are built by `ze appliance build`, which produces a full gokrazy disk image with ZeFS credentials already embedded in the `/perm` partition. Deploying to new hardware required either PXE network provisioning (which fetches a separate ZeFS database and needs an isolated provisioning network) or manually `dd`-ing the image file. Operators needed a bootable installer medium that preserves the exact image bytes and embedded credentials without PXE infrastructure.

## Decisions

- ISO is a transport envelope around the existing raw image, not a new image format. The installer writes the full disk image unchanged, preserving gokrazy's A/B partition layout, boot references, and embedded ZeFS. No partition reconstruction in the initrd.
- Extended the existing initrd with a `ze.source=iso` mode over creating a second init script, because disk safety, checksum validation, and success markers are shared concerns.
- ISO mode does not fetch or write a separate `database.zefs` (unlike PXE), because `ze appliance build` already injects ZeFS into the `/perm` partition of the image.
- Missing `.sha256` sidecar is a hard error (no `--allow-no-checksum` override), because ISOs are durable offline artifacts and checksum enforcement prevents distributing corrupt media.
- Refuse implicit target selection when multiple fixed disks exist (unlike PXE which picks the first), because ISO installs run on multi-disk machines where silent selection is dangerous.
- ISO source media identified by a 16-byte random `ze.media-id` marker file, so the initrd can reliably find and exclude the boot medium even across CD-ROM and USB block device names.
- Image is gzip-compressed into the ISO staging tree and decompressed during install, reducing ISO size without changing the installed bytes.
- UEFI boot via `grub-mkstandalone` and `xorriso` over custom EFI stub, because GRUB handles the firmware handoff and xorriso produces standards-compliant ISO 9660.
- `--target` flag embeds an explicit `ze.target=/dev/vda` in the GRUB kernel command line, so operators can lock the install target at ISO creation time for unattended deployment.

## Consequences

- `ze appliance iso <name>` is the complete ISO creation command; no manual staging, iPXE compilation, or GRUB scripting needed.
- The initrd has two source modes (`http` for PXE, `iso` for local media) with shared target-disk filtering and checksum validation.
- External dependencies (`grub-mkstandalone`/`grub2-mkstandalone`, `xorriso`) are validated at command startup with actionable error messages. No doctor check added because these are build-time-only tools.
- arm64 ISO uses `arm64-efi` GRUB target and `bootaa64.efi` boot file; arch mismatch between kernel and appliance config is rejected.
- `ze appliance iso` shares `resolveImagePath` and path-containment checks with `ze appliance push`; no duplicated validation logic.
- QEMU evidence (`scripts/evidence/effective-install-iso-qemu.py`) proves the full chain: ISO boot, image write, no PXE ZeFS injection, power-off (not reboot), and SSH login with embedded credentials on the installed disk.

## Gotchas

- The spec assumed code would live under `cmd/ze/install/appliance/` but the appliance commands had already moved to `internal/appliance/` (spec-cmd-to-plugin, learned 850). All file paths in the spec are stale.
- `grub-mkstandalone` is named `grub2-mkstandalone` on some distros (Fedora, RHEL). The builder resolution tries both names.
- The installer must power off (not reboot) after writing, because rebooting back into removable media would re-run the installer. The initrd uses `poweroff -f` in ISO mode.
- Image names embedded in the GRUB kernel command line must be safe for the shell and bootloader parser. A dedicated regex (`[a-zA-Z0-9._-]+`) rejects anything with spaces, quotes, or special characters.
- PXE and ISO share the same initrd binary but diverge at the `ZE_SOURCE` branch. Changes to shared functions (disk detection, checksum validation) must be tested in both modes.

## Files

- Created: `internal/appliance/cmd_iso.go` (ISO command, staging, builder invocation, gzip compression, FAT EFI image, GRUB config generation)
- Created: `internal/appliance/cmd_iso_test.go` (22 tests: dispatch, name validation, image selection, path escape, symlink escape, checksum, staging plan, builder args, cleanup, dependency missing, arch mismatch, arm64 boot artifacts, default paths, kernel cmdline safety)
- Created: `tools/installer-initrd/test/test-iso-media.sh` (11 tests: media discovery with mock block devices and mounts)
- Created: `scripts/evidence/effective-install-iso-qemu.py` (end-to-end QEMU proof: build, ISO creation, boot, write, partition check, SSH login)
- Created: `test/install/qemu-iso.ci` (functional test entry for ISO evidence)
- Modified: `internal/appliance/main.go` (added `iso` to dispatch table and help)
- Modified: `internal/appliance/register.go` (added `cmdIso` slot)
- Modified: `tools/installer-initrd/init` (added `ze.source` parsing, `ze.media-id` validation, `find_iso_media`, ISO source branch with local image streaming and gzip decompression, power-off instead of reboot)
- Modified: `tools/installer-initrd/Makefile` (added `/mnt/iso` dir, `test-iso-media.sh` target)
- Modified: `tools/installer-initrd/test/test-cmdline-parse.sh` (added ISO cmdline parse cases)
- Modified: `tools/installer-initrd/test/test-disk-detect.sh` (added ISO source exclusion, multi-target rejection, explicit target validation)
- Modified: `mk/test-integration.mk` (added `ze-install-iso-qemu-test` target)
- Modified: `docs/guide/appliance.md` (ISO creation workflow, command table, prerequisites)
- Modified: `docs/guide/ze-install.md` (ISO vs PXE distinction, target selection, boot flow)
- Modified: `docs/functional-tests.md` (ISO evidence prerequisites, skip behavior, make target)
