# Spec: onie-0 -- ONIE installer for Ze on whitebox switches (Umbrella)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P3 |
| Release reason | Installing Ze from ONIE on a switch is broader platform support, not a missing first-release installer artifact: the ISO and PXE installers already exist. |
| RFC gaps | - |
| Depends | spec-appliance-perm-grows-at-install (in progress: it wires the growth steps this work shares and must not be redesigned in parallel) |
| Phase | DESIGN (direction approved 2026-11-28; the three children were judged once, refused, and reworked for every finding. The judge could not record from the Mac: its identity and read records are local to the machine it runs on, so each child owes a fresh judgment by a judge running in this checkout, then Thomas's terminal approval) |
| Handoff | - |
| Design tier | strongest tier |
| Updated | 2026-11-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Give Ze an installer that ONIE runs, the way the bootable ISO is the installer a
PC firmware runs. An operator at the ONIE prompt of a supported switch types
`onie-nos-install` with the URL of one file built by the appliance builder, and
the switch reboots into Ze. ONIE itself, and any vendor diagnostics partition,
stay on the disk, and a Ze command boots the switch back into ONIE.

The first two supported switches are the Dell S4048-ON and the Dell S5232F-ON.

The builder takes the hardware model as an input. The model selects the
per-hardware facts (architecture, kernel profile, console, firmware mode, the
ONIE platform strings the installer accepts) so that an operator names the
box and does not spell those facts out. The same input covers the Intel N100
mini PC the appliance already targets.

The reference studied is NOSaic (`Salvaged-silicon/nosaic-switch`,
`internal/boot/onie.go`, Apache-2.0). Only its envelope carries over (one file:
a program followed by an appended payload). Its installer is a shell script
that overwrites the whole disk on U-Boot boards (PowerPC, 32-bit ARM) whose
ONIE lives in flash. On both Dell switches ONIE lives on the same disk as the
NOS, and Ze builds neither of those architectures.

### Owner decisions (2026-11-28, voice-transcribed, recorded as understood)

| # | Decision | Reading taken |
|---|----------|---------------|
| D-1 | Target hardware: Dell S4048-ON and Dell S5232F-ON | amd64 only. No arm64 or U-Boot ONIE machine is in scope |
| D-2 | "We should be able to pass the model to the builder (same for N100)" | The appliance builder gains a hardware-model input. Models register themselves and are discovered, as kernel profiles are |
| D-3 | "Our solution should be pure Go" | No shell script and no external tool, on the build host or on the switch. The file ONIE runs is a static Go program with the image appended |
| D-4 | ONIE must survive a Ze install | Ze installs beside ONIE and the diagnostics partition and can boot back into ONIE. The whole-disk overwrite NOSaic uses is rejected |
| D-5 | The work is saved on the Linux server, in its shared checkout | One-off authorisation for this spec, given to a session on the Mac |

### Children (cut approved by Thomas on 2026-11-28)

| Child | Owns | Why it is separate |
|-------|------|--------------------|
| spec-onie-1-hardware-model | The model input of the builder, the model registry, the first models (generic, n100, dell-s4048-on, dell-s5232f-on), and the kernel drivers a model requires | Useful without ONIE (the N100), and every later child reads a model |
| spec-onie-2-relative-partitions | Ze and the pinned gokrazy runtime find Ze's four partitions relative to its boot partition, not at GPT entries 1 to 4. Boot, OTA push, MBR boot code and growth all keep working on a disk that holds other partitions first | It changes behaviour an image in the field depends on, and is provable on today's installers before any ONIE code exists |
| spec-onie-3-installer | `ze appliance onie`, the self-contained installer program and its third source mode, ONIE's boot environment and EFI boot entries, the command that returns to ONIE, and the proof against ONIE's virtual machine | The feature itself, and the only child that needs an ONIE environment |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`
  → Constraint: a feature registers from its own `init()` and is discovered; no switch, table or help string elsewhere learns its name.
- [ ] `ai/doors/appliance.md`
  → Constraint: an image already in the field is the contract; a change to the push protocol or a `/perm/ze/` path must still work against the device running the previous image.
  → Constraint: the initrd is one static Go binary with zero external binaries; a host binary is never cross-compiled, a target binary is.
- [ ] `docs/architecture/appliance/iso-installer.md`
  → Decision: "Extend the one initrd with an ISO source mode rather than ship a second init. Disk safety, checksum validation, and success marking are shared concerns." The ONIE installer is a third source mode of the same installer, not a second program.
  → Constraint: a missing checksum sidecar is a hard error with no override; the image is gzip-compressed for transport and the installed bytes do not change.
  → Constraint: the ISO installer powers off, never reboots. Under ONIE the opposite holds: exit status 0 and a reboot are what ONIE expects.
- [ ] `docs/architecture/appliance/installer-initrd.md`
  → Constraint: mount, block-device ioctls, reboot and network are done in process; `internal/install/disk` and `cmd/ze-installer` hold no `exec.Command` of an external binary.
  → Constraint: PID 1 routes a fatal error to a rescue console. Under ONIE the installer is not PID 1, so its fatal path must exit non-zero instead.
- [ ] `docs/architecture/appliance/on-device-installer.md`
  → Constraint: `internal/core/gptdisk` is the one reader of a GPT; `Grow` extends partition 4 and refuses a disk where partition 4 is not the last.
  → Constraint: the growth steps are logged under `doctor-storage-not-grown` and do not yet do their work; `spec-appliance-perm-grows-at-install` owns that.
- [ ] `docs/architecture/appliance/ota-push.md`
  → Constraint: a push writes only the first 440 bytes of the MBR (`pushMBRCodeBytes`), and that boot code must match the FAT layout of the pushed image.
- [ ] `docs/architecture/appliance/kernel-profiles.md`, `builder.md`
  → Decision: a kernel profile is a pair of files found by directory scan (`registeredKernelProfiles`); `image.kernel-profile` selects the installer kernel only, and the runtime kernel is one shared profile.
- [ ] `ai/META-DECISIONS-SITUATIONAL.md`, "Test systems and labs"
  → Decision: an ONIE virtual machine image may boot in the QEMU harness as the third party under test, pinned by checksum (owner rule added 2026-11-28); after the install the guest runs Ze's own kernel and system.

**Key insights:**
- ONIE runs the downloaded file directly after `chmod +x`, with no shell and no content check except refusing a file that carries its updater marker string. A static Go program is a valid installer.
- The appliance image owns the whole disk: four partitions fixed at GPT entries 1 to 4 from sector 8192, addressed by number in boot, update and growth. ONIE already holds the first entries and those sectors.
- Under legacy BIOS the image boots through 440 bytes of MBR code that read the kernel from absolute sectors fixed at build time, so an image placed at another offset needs that code recomputed, at install and at every push.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/modcache/github.com/gokrazy/internal@v0.0.0-20260625065634-6994f9152c44/rootdev/rootdev.go` - `findGPTPartUUID` compares the root PARTUUID with the first GPT entry of each disk only ("Check only the first partition id"); `Partition(n)` is the device path plus n; `ActiveRootPartition` panics unless the root is partition 2 or 3.
- [ ] `vendor/github.com/gokrazy/tools/packer/packer.go` - writes a hybrid MBR and a GPT with entries 1 (FAT boot, 100 MiB, from sector 8192), 2 and 3 (root A and B, 500 MiB each) and 4 (persistent, the rest). Reported by the research agent, not re-read by the main session.
- [ ] `internal/core/gptdisk/gptdisk.go`, `grow.go` - `PersistentPartition` is 4; `planGrowth` refuses any entry lying after partition 4.
- [ ] `internal/install/disk/run.go` - two source modes (HTTP and ISO) chosen by a switch on the source; inputs come only from the kernel command line; `partitionPath(disk, 4)` is used for the persistent partition; HTTP ends in a reboot, ISO in a poweroff.
- [ ] `internal/appliance/cmd_iso.go` - `ze appliance iso`: name validation, required checksum sidecar, gzip of the image, a staging tree, then two external tools (`grub-mkstandalone`, `xorriso`). The ISO is UEFI only.
- [ ] `internal/appliance/push_image.go`, `cmd_push.go` - a push streams root, boot and the first 440 MBR bytes.
- [ ] `internal/appliance/kernelreg.go`, `config.go` - profiles qemu, hardware, hardware-kms; no build-time model; the console is fixed per image; no fragment builds `ixgbe`.
- [ ] ONIE `rootconf/default/bin/exec_installer` (upstream master) - `chmod +x` then direct execution of the installer; exit 0 leads to ONIE calling `reboot`; a non-zero exit leads to a retry every 20 seconds.
- [ ] NOSaic `internal/boot/onie.go` - shell installer with an appended tar; whole-disk overwrite; U-Boot environment handling.

**Behavior to preserve:**
- An image built without a model, and the ISO and PXE installers, behave exactly as today.
- A device already in the field, installed on a whole disk, still boots, takes a push and grows its persistent partition after this work.
- The installer stays one program with no external binary.

**Behavior to change:**
- The builder accepts a hardware model.
- Ze's partitions may sit after other partitions on a disk.
- A new builder command produces an ONIE installer file, and the installer gains a source mode that reads the image appended to itself.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The installer and its source modes | `internal/install/disk/run.go` `Run`, `RunInitrd`; `iso-installer.md` "Decisions" | Writes an image from an HTTP server or from ISO media, sharing target checks, checksum and write | extends: a third source mode reads the image appended to the running program |
| The ISO builder command | `internal/appliance/cmd_iso.go` `runIso`, `resolveISOInput`, `compressOpenFileGzip` | Validates the appliance, requires the sidecar, compresses the image, wraps it for a firmware | extends: `ze appliance onie` reuses the validation, sidecar and compression; the GRUB and xorriso half does not apply |
| The installer program | `cmd/ze-installer`, `installer-initrd.md` | One static Go binary, PID 1 of the initrd | extends: the same program is the file ONIE runs, entered as an ordinary process |
| The GPT reader and grower | `internal/core/gptdisk` `Parse`, `Grow`, `PersistentPartition` | Reads a table, extends partition 4 when it is last | extends: it learns to add entries after existing ones and to address the persistent partition relative to Ze's boot partition |
| Kernel-profile registry | `internal/appliance/kernelreg.go` `registeredKernelProfiles`; `kernel-profiles.md` | Discovers profile file pairs by directory scan | extends: a model is discovered the same way and names the profile it needs |
| Appliance image options | `internal/appliance/config.go` `ImageConfig` | Holds arch, kernel profile, hugepages, isolated CPUs, memory | extends: gains the model; the model supplies defaults for the other fields |
| gokrazy runtime patch route | `gokrazy/patches/reboot-ack.patch`, `gokrazy-build-pins.md` | Carries Ze's diff against the pinned gokrazy runtime | extends: a second diff makes `rootdev` and the update handlers relative to the boot partition |
| OTA push | `internal/appliance/push_image.go`; `ota-push.md` | Streams root, boot and 440 bytes of MBR code | extends: the device recomputes the MBR code for its own boot-partition offset |
| Hardware facts at runtime | `internal/component/host/dmi_linux.go`; `plan/post-release/features/spec-hardware-capacity-profile.md` (skeleton, part P5) | Reads DMI; plans a platform-matched capacity database | does not apply at build time; the model names of this spec and that database must agree, so the child spec cites it |
| Installer QEMU proofs | `internal/le/test/qemu/install.go`, `install_iso.go` | Boot an installer in a VM, install, boot the result, log in | extends: a new proof boots ONIE's virtual machine and runs the installer under it |

## Research findings (2026-11-28)

### ONIE's contract for a NOS installer on x86 (upstream `opencomputeproject/onie`)

| Subject | Finding | Source |
|---------|---------|--------|
| Fetch | The file is downloaded to `/var/tmp/installer`, a tmpfs with no size option, so half of RAM bounds its size | `exec_installer`, `init.d/makedev.sh` |
| Execution | `chmod +x` then direct execution; arguments after the URL are passed through | `exec_installer` `run_installer` |
| Only content check | A file holding ONIE's updater marker string in its first hundred lines is refused in install mode | `exec_installer` `get_image_type` |
| Success | Exit status 0, then ONIE calls `reboot` by bare name; a non-zero status is retried every 20 seconds | `exec_installer` `finish_nos_install`, `discover` |
| Machine facts | `/etc/machine.conf` holds platform, firmware and partition type; they are not reliably in the environment | `init.d/gen-config.sh`, `lib/onie/functions` |
| Disk layout, BIOS | GPT; entry 1 GRUB-BOOT (2 MiB), entry 2 ONIE-BOOT (128 MiB, type GUID 7412F7D5-A156-4B13-81DC-867174929325) | `installer/grub-arch/install-arch`, `onie-blkdev-common` |
| Disk layout, UEFI | GPT; entry 1 EFI system partition (256 MiB), entry 2 ONIE-BOOT | same |
| Partitions a NOS must leave alone | ONIE-BOOT, GRUB-BOOT or the ESP, and any partition whose label ends in `-DIAG` | `discover-arch`, design page `x86_nos_interface` |
| Leaving install mode | The NOS sets `onie_nos_mode=yes` and clears `onie_mode` in `/mnt/onie-boot/grub/grubenv`, a 1024-byte GRUB environment block | `nos-mode-arch`, `onie-blkdev-common` |
| Install mode under BIOS | Until that is set, ONIE reinstalls its own boot code in the MBR on every boot | `onie-blkdev-common` `bios_boot_onie_install` |
| Returning to ONIE | BIOS: the NOS boot loader chainloads ONIE-BOOT. UEFI: the NOS sets `BootNext` to ONIE's entry | `grub.d/50_onie_grub`, design page `x86_uefi` |
| Virtual machine | `machine/kvm_x86_64` builds a recovery ISO usable under BIOS and UEFI; a copy is published by the SONiC project (34.8 MB, answered HTTP 200 on 2026-11-28) | `machine/kvm_x86_64/INSTALL`; `sonic-buildimage` `platform/vs/onie.mk` |

### The two switches

| Fact | S4048-ON | S5232F-ON |
|------|----------|-----------|
| ONIE machine | `dell_s4000_c2338` upstream (the directory does not name the S4048-ON; unverified on the unit) | none upstream; SONiC names it `x86_64-dellemc_s5232f_c3538-r0` |
| CPU | Intel Atom C2338 | Intel Atom C3000 family (C3538 inferred from the SONiC name) |
| Firmware as ONIE uses it | legacy BIOS mode (UEFI firmware with CSM), ONIE kernel 3.2.69 with no EFI support | UEFI (unverified) |
| Disk | SATA CFast, appears as a SCSI disk; size unverified | SSD; type and size unverified |
| Console | first serial port at 115200 | first serial port (SONiC platform file) |
| Management NIC | unverified | two 10G ports reported; chip unverified |
| RAM | unverified | 16 GB reported |
| Switch ASIC | Broadcom | Broadcom Trident 3 |

## Risks & Assumptions

### Assumptions

| # | Assumption | Basis | If wrong | Validation |
|---|------------|-------|----------|------------|
| A-1 | The factory ONIE on each switch behaves as upstream master does | Only upstream source was read | The installer may be refused or misread the machine facts | The commands of "Commands for the switches" below, run by Thomas |
| A-2 | A Go 1.27 static program runs on the S4048-ON's ONIE kernel | Go 1.24 notes require Linux 3.2; the 1.25 to 1.27 notes name no higher floor; upstream ONIE for this machine uses 3.2.69 | Pure Go cannot run under that ONIE at all, and D-3 needs a different route on this switch | `uname -r` on the switch, then running a trivial static Go program there |
| A-3 | ONIE's partitions keep working when later entries hold Ze's partitions | The reference installer adds partitions after ONIE's | ONIE may fail to update or uninstall | The ONIE virtual machine proof: install Ze, boot back into ONIE, run its uninstall |
| A-4 | The compressed installer fits in half the S4048-ON's RAM | The default image is 2 GiB and mostly empty; no measured compressed size exists | The download fails with a short write | Measure the compressed size at build; refuse at build above the model's stated limit |
| A-5 | The S5232F-ON runs without Secure Boot enforcing signatures | Unverified | Unsigned boot code does not start | Secure Boot state read on the switch |
| A-6 | Both switches have a disk large enough for ONIE plus a 2 GiB image | Unverified | The installer must refuse, naming the space it needs | Partition table read on the switch |
| A-7 | The management NIC of each switch has a driver in Ze's kernels | No fragment builds `ixgbe`; `igb` is a module | The installed switch has no management port | NIC identity read on the switch; the model names the driver it requires |

### Risks

| # | Risk | Early signal | Mitigation |
|---|------|--------------|------------|
| R-1 | A failed install leaves neither ONIE nor Ze bootable | Any write before the last step fails | Order the writes so ONIE's boot path is the last thing changed; every earlier step is repeatable, because ONIE re-runs the installer |
| R-2 | A push to a switch installed beside ONIE writes boot code computed for a whole-disk layout and the switch no longer boots | The pushed MBR code names sectors inside ONIE's partition | The device recomputes the code for its own offset; an acceptance criterion pushes to such a device and boots it |
| R-3 | The installer program happens to contain ONIE's updater marker string and is refused | ONIE reports an update image in install mode | The builder refuses to emit a file that contains the string |
| R-4 | The front-panel ports do not forward | Only the management port carries traffic after install | Outside this set: it needs a datapath for the Broadcom ASIC. The feature status of each model says management only |
| R-5 | The proof VM differs from the S4048-ON (newer kernel, Secure Boot build) | A pass in the VM and a failure on the unit | Run the VM in BIOS mode for the S4048-ON model, and record a pass on the real unit separately |

## Commands for the switches (for Thomas, at the ONIE prompt of each)

These settle A-1, A-2, A-5, A-6 and A-7. An agent never runs a command on a
network device.

| Command | What it settles |
|---------|-----------------|
| `cat /etc/machine.conf` | platform string, firmware mode, partition type, ONIE version |
| `uname -r` | the kernel against Go's 3.2 floor |
| `ls /sys/firmware/efi/efivars` | whether ONIE booted under UEFI |
| `blkid` | partition labels, and any diagnostics partition |
| `sgdisk -p /dev/sda` | the partition table and the free space |
| `free -m` | RAM, which bounds the installer's size |
| `mount` | the tmpfs options and where ONIE-BOOT is mounted |
| `lspci -nn` | the management NIC and disk controller |
| `efibootmgr -v` | on the S5232F-ON, the boot entries; absent on a BIOS build |
| `mokutil --sb-state` | on the S5232F-ON, Secure Boot state, if the tool exists |

## Open design questions (to settle at the design gate)

| # | Question |
|---|----------|
| C-1 | DECIDED 2026-11-28: the QEMU harness may boot ONIE's own virtual machine image, pinned by checksum, as the third party under test. Recorded in `ai/META-DECISIONS-SITUATIONAL.md`, "Test systems and labs" |
| C-2 | Design taken in `spec-onie-3-installer.md` after the first design judgment: Ze starts ONIE by loading ONIE's own kernel from ONIE-BOOT and jumping to it, writing nothing to disk, so a reboot from ONIE's rescue mode returns to Ze. ONIE's boot code is still saved in a checksummed record at install, as the manual fallback |
| C-3 | DECIDED 2026-11-28: three children, as listed under "Children" |
| C-4 | Raised by Thomas: "Can we update the kernel when we do an onie install?" The installed switch always runs Ze's kernel. Having the installer itself jump to Ze's installer kernel is recorded as an alternative set aside in `spec-onie-3-installer.md`, with the condition that reopens it |
## Design judgment, round 2 (2026-11-28): all three children refused

Recorded by three independent judges with `./le spec design judge` in the Mac
checkout, on copies identical to commit 9c4b846e8f. Each row is owed before the
child is judged again.

### spec-onie-1-hardware-model (digest b726b9ec8e97)

| # | Finding | Fix owed |
|---|---------|----------|
| 1 | The `build-artifacts.md` row says the model check reads "the cached kernel's emitted config"; the rest of the spec says fragment files only | Reword the row: fragment files only |
| 2 | The `vpp-host-tuning.md` decision row speaks of a build-time refusal for a missing driver, which the design no longer has | Reword: the refusals are AC-3, AC-4 and AC-12 |
| 3 | The `builder.md` row is marked agrees, but the page says three config fields reach the kernel command line and the console becomes a fourth | Make it a `departs` row with before and after |
| 4 | AC-4 says `Validate` refuses "at every command"; it is called from init, build, iso and kernel only | Scope the AC to those, or name the commands that gain the call |
| 5 | Census lacks two writers of the `image` object: `cmd_clone.go` and `editImageConfig` in `internal/le/test/qemu/hugepages.go` | Add both rows |
| 6 | Existing Mechanisms lacks `RuntimeKernelLayers`, `applianceDoctorChecks` and `applianceCommands` | Add three rows |
| 7 | The Task says a model's driver "is carried by" the runtime kernel; AC-6 proves only that a fragment requests it | Reword the Task, or prove carriage inside the one existing kernel check |
| 8 | Minor: five `.ci` tests are listed where rows say four; AC-12 says `generic` lists both firmware modes while AC-2 says it states nothing; A-4 still says "driver refusal"; "as a module" is wrong for a boot-critical device | Correct each |

### spec-onie-2-relative-partitions (digest 9007085d5579)

| # | Finding | Fix owed |
|---|---------|----------|
| 1 | Existing Mechanisms has no row for the installer proof kind dispatch that AC-12 replaces (`InstallKind` and its three switches in `install.go`, `runInstallerHere` in `actions.go`) | Add the row, marked replaces |
| 2 | AC-8 removes an earlier Ze run and its data, and R-7 says another gokrazy system would be replaced too; neither the Task nor the Design presentation says so | State it in both, or put replace versus refuse to Thomas as a choice |
| 3 | Census lacks the install-time writer of re-targeted boot code (`internal/core/bootcode`), the installer's whole-image write as a GPT and MBR writer, and `internal/le/setup/docker/host/perm.go`, which reads `gptdisk.PersistentPartition` | Add the rows and the file |
| 4 | Not a ground: one patch file spans two modules, while the guard takes one patch per module; R-5's test is in `grow_integration_linux_test.go` | Split the patch, name the test |

Every Design documents row of this child held, including both `departs` rows.

### spec-onie-3-installer (digest ea23848c4b46)

| # | Finding | Fix owed |
|---|---------|----------|
| 1 | The `system-plugins.md` row still says the plugin compiles out under `ze_onie`, which the spec now rejects | Settle the gate (finding 2), then make the row match |
| 2 | Linking the plugin by a hand import behind `ze_appliance` departs from the system door: generated files own the blank imports, and a gated feature is declared in `feature-gates.txt`. `./le repo update` would link the plugin into every flavor. `coldplug` is no precedent: it is a start step, not a registry plugin | Redesign: a real gate in `feature-gates.txt` is the likely answer, which was the first design |
| 3 | `test/plugin/onie-show-absent.ci` runs a `ze` built without `ze_appliance`, where the commands would be absent | Follows from finding 2 |
| 4 | The reboot after graceful shutdown is done by the hub (`cmd/ze/hub/main.go`, `internal/core/reboot`), after every plugin has stopped; a plugin cannot jump to ONIE's kernel after that | Design the seam: the hub or `core/reboot` takes a registered end-of-shutdown action |
| 5 | Census lacks the wires the start-ONIE design added: ONIE's kernel, ramdisk and boot files (reader: the plugin), and the kernel command line with the boot reason (writer: the plugin, reader: ONIE) | Add the rows |
| 6 | AC-20 requires a rescue round trip on a real S4048-ON, which AC-22 and C-5 say the loader refuses | Decide C-5 first, then make AC-20 follow it |
| 7 | The Task carries neither the reinstall's first write nor the 3.8 kernel limit | Add both to the Task |
