# Spec: appliance-perm-grows-at-install

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P3 |
| Release reason | It exists for the container work, which is a post-release feature. Owner ruling of 2026-11-16: "The appliance grows its persistent partition to the disk at install. This is appliance work of its own and comes before the image store." No shipped behavior is wrong without it. |
| RFC gaps | - |
| Depends | - |
| Phase | 3/8 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The appliance's persistent partition is the size the image was built with: about 944 MB on the default 2 GiB image, on a disk of any size. The installer writes the image raw and changes nothing after it. The container image store (`spec-container-2-image-store.md`) stores images, writable layers and volumes on that partition, and needs the kernel to refuse a write past a size cap on a folder.

The goal has two parts.

- **Growth.** A device installed by the PXE or ISO installer uses its whole disk: partition 4 ends at the last usable sector and its filesystem fills the partition. A device that was not installed that way reports the difference in `ze doctor` and grows on one command.
- **The size-cap facility.** The persistent partition is formatted so the kernel enforces an ext4 project quota on it, the kernels that mount it carry quota support, and Go code can give a folder a project id, set its limit and read its usage with no external tool. This spec delivers the facility. The image store child uses it.

Ze is pre-release and no device is in the field. The new format is the format: no migration of an installed device is designed. The repository carries one other user of the format, the docker-host lab image that is also the own-system guest Ze's tests run in, and this spec converts it in the same work (D-4).

### Owner decisions

| Decision | Ruling |
|----------|--------|
| Growth at install | "The appliance grows its persistent partition to the disk at install. This is appliance work of its own and comes before the image store." (2026-11-16) |
| The size cap | A size cap the kernel enforces. A host whose data directory cannot enforce one refuses a container config (umbrella spec, "Where containers run") |

### Decisions taken (main thread, 2026-11-16)

Each of the three takes its recommendation, unless the owner objects at the final review, and the design below is written under them. A device not grown at install grows only on a command, and doctor reports it until then (D-1). The command is `request system storage grow` (D-2). A growth failure at install finishes the install and is reported by doctor on the booted device (D-3).

| # | Question | Recommendation | What the operator sees under each answer |
|---|----------|----------------|-------------------------------------------|
| D-1 | A device that was not grown at install (a raw image written by hand, a virtual disk enlarged later): does it grow by itself at boot, or only on a command | Only on a command. `ze doctor` reports it | By itself: the next boot grows the partition and logs one line; no doctor finding stays. On a command: `ze doctor` shows a warning with the two sizes until the operator runs `request system storage grow`, and the boot path never writes a partition table |
| D-2 | The command word | `request system storage grow` | `request` is the existing verb for an operational action on the system. The state is read with the existing `show host storage` |
| D-3 | Growth fails at install after the image is written (the kernel refuses the resize, the disk has an unsupported sector size) | Finish the install, log the failure as an error on the console, and let `ze doctor` report it on the booted device | Finish: the device boots and is reachable with the image-sized partition, and doctor names the fix. Stop: the installer enters its recovery console and the machine does not boot until someone attends it, although the disk holds a complete system |

A fourth decision is a ruling of the main thread and has no alternative left open.

| # | Ruling | Why | What follows in this spec |
|---|--------|-----|---------------------------|
| D-4 | The guest that Ze's own tests run in (the docker-host lab image, which is also the own-system QEMU guest) gets the same persistent-partition format as the appliance, from one shared definition of that format | That guest is formatted by `formatPerm`, a plain format with no feature list, so it cannot enforce a size cap and the container tests would be refused there. A change of format converts every user of it the repository carries (`ai/META-DECISIONS.md`, "Ze is pre-release") | `formatPerm` derives its format from the definition the appliance build uses, its skip of an already formatted partition is deleted, and AC-14 proves the guest's partition. The runtime, image store and network children of the container work run their guest tests on AC-14 |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/appliance.md` - the device, the installer, the build host
  → Constraint: the initrd is one static Go binary with no external binary. Growth at install is Go: a partition-table rewrite and a kernel ioctl, never a resize tool
  → Constraint: an image in the field is the contract for a path under the persistent partition, a zefs key and the push protocol. This spec adds no path and no key, and leaves the push protocol alone
  → Constraint: the initrd cache key hashes the files of the installer command folder and the installer package only (`initrdSourceDirs`). A package the installer newly imports is added there, or a cached initrd without the growth is served
- [ ] `ai/doors/system.md` - the shared design
  → Constraint: a command is declared once with a YANG registration and reaches every surface. The grow command is a plugin command, not a local-only verb
  → Constraint: a readiness check is a doctor check registered by the package that owns the dependency
- [ ] `ai/doors/system-plugins.md` - the command providers
  → Decision: a command-only plugin owns the command's YANG and handler and delegates the work to a component or a core package. The grow command joins the host command plugin and delegates to the leaf packages
  → Constraint: every plugin can run in another process. The handler's work is system calls made by its own process, with no call into another component's state
- [ ] `ai/doors/hub-engine.md` - daemon start
  → Constraint: a hook wired late in startup is missing for a plugin that acts as soon as it handshakes. If enforcement ever needs a step at daemon start (route E2 below), that step runs before the plugin tiers start
- [ ] `ai/doors/core.md` - shared libraries
  → Decision: the partition-table code, the filesystem resize and the quota calls are core leaf packages: libraries with no config lifecycle and no registry, imported by the installer, the build host and the daemon
- [ ] `ai/doors/le-tooling.md` - tests and development tooling
  → Constraint: a test that needs a kernel feature runs in the QEMU guest on Ze's kernel. A new QEMU action needs a real caller in the same change
- [ ] `docs/architecture/appliance/on-device-installer.md` - the installer and build-side verification
  → Constraint: "gokrazy does not reformat `/perm`. It mounts or skips." Nothing at boot repairs a filesystem the build got wrong
  → Decision: a pure-Go ext4 writer was studied and rejected (about 1500 lines, forked from a Windows container runtime). Creating the filesystem fresh at install is therefore not the design
  → Constraint: `debugfs -R` exits 0 when its subcommand fails. Every new build step that uses an e2fsprogs tool is verified by reading the result, never by the exit code alone
- [ ] `docs/architecture/appliance/iso-installer.md` - the ISO transport
  → Constraint: the page states "The initrd reconstructs no partitions" and that the image bytes are written unchanged. This spec changes that sentence: the bytes are written unchanged and the table is then extended
  → Constraint: PXE and ISO share one initrd and diverge at the source branch. A change to a shared function is tested in both modes
- [ ] `docs/architecture/appliance/installer-initrd.md` - PID 1 rules
  → Constraint: a syscall is isolated in a named Linux-only helper behind a function variable, so it is testable with a fake
  → Constraint: PID 1 cannot exit. A growth failure routes through the existing fatal path or is logged and survived (D-3), never a panic
- [ ] `docs/architecture/appliance/ota-push.md` - the update
  → Constraint: the push writes the inactive root, the boot filesystem and the first 440 bytes of the boot sector. "the target disk signature, partition table, GPT and persistent partition must remain intact". A grown table therefore survives every update, and an update never brings a new filesystem format to an installed device
- [ ] `docs/architecture/appliance/kernel-profiles.md`, `docs/architecture/appliance/build-artifacts.md` - kernels
  → Constraint: a kernel option is requested in the fragment, required in the manifest, and the build refuses when the emitted config does not hold it as built in. The installer floor and the runtime floor are compiled lists an edited manifest cannot drop
  → Constraint: a downloaded installer kernel has no staleness check. An installer kernel older than this change may lack quota support (R-4)
- [ ] `docs/architecture/appliance/docker-host.md` - the lab image, which is the own-system guest (the design page of `internal/le/setup/docker/host/perm.go`)
  → Constraint: step 5 of the image build says the partition "is formatted ext4 when it carries no superblock: gok formats it only where `mkfs.ext4` is on its PATH". No packer in the tree formats it, so the sentence is wrong today and is replaced with the new format
  → Constraint: the image is cached under a key over its inputs, and a build with the same inputs reuses the cached image. A change of format reaches a machine only through a changed key
- [ ] `docs/architecture/testing/qemu-integration.md` - QEMU proofs
  → Decision: the host-side `installer`, `installer-iso` and `appliance-ota` actions already boot a real install and read the stopped disk. They gain a target disk larger than the image
  → Constraint: `assertInstallGPT` requires the installed disk's first four entries to equal the image's. Entry 4 now differs by design, so that check changes with the feature
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor checks (the design page of the storage check's test, and the page `ai/CODE-TO-DOCS.md` names for `internal/component/doctor/checks_storage.go`)
  → Constraint: a check registers at the phase whose process holds the state it reads. The storage check reads the mount table and the disk, which read the same from the operator's process and the daemon, so it takes a config phase, before config
  → Constraint: a finding names the command that clears it in its `Fix` field, and every `Warn` or `Error` line names a code a registered check declares. The installer's growth-failure line needs the check that declares `doctor-storage-not-grown`
  → Decision: a dependency with no owner that can register its check is registered by `internal/component/doctor`, with a comment that makes the absent owner explicit. The host component owns the state and cannot import the check registry
- [ ] `ai/rules/platform-linux.md`, `ai/rules/go-standards.md` - Linux-only code
  → Constraint: shipped code runs no external program. The e2fsprogs tools stay on the build host only
  → Constraint: a probe that cannot run answers unknown with its reason. A storage state that cannot be read is never reported as grown

### Kernel interfaces (recalled, not read)

No kernel source and no kernel documentation is on this machine: the system source folder holds header packages only, and they carry neither the ext4 code nor the documentation tree. Every kernel behavior this design uses is recalled from the kernel's ext4 and quota code. Each is an assumption below with the test that settles it.

- [ ] ext4 online resize, the mount rules of the quota feature, the block-device partition ioctls
  → Constraint: nothing in this design is reported as confirmed until its QEMU test has run on Ze's runtime kernel

**Key insights:** (minimal context to resume after compaction)
- No partition-table code exists in the installer package. Four GPT readers exist elsewhere, and the only writer is the gokrazy packer on the build host.
- The image carries its backup GPT at the end of the image, not of the disk. On a larger disk the primary header's backup pointer and last usable sector are the image's.
- The packer's protective boot-sector entry covers only the sectors before partition 1, so it does not record the disk size and needs no rewrite.
- gokrazy init mounts the persistent partition as ext4 with no flag and no option string.
- The build formats with the host's `mkfs.ext4` defaults, minus metadata checksums. Which features the filesystem has depends on the build host's configuration file.
- The vendored system-call package has the block-device size and partition-resize ioctls and the quota system call number. It has no ext4 resize ioctl, no project-attribute ioctl and no quota structures: the leaf packages define them.
- The docker-host lab image, which is also the own-system QEMU guest Ze's tests run in, is formatted by a second formatter in development tooling: a plain format with no feature list. This spec converts it to the appliance's format, from one shared definition (D-4, AC-14).
- No packer in the tree formats partition 4 of an image file: the vendored packer and both module-cache copies only print the `mkfs.ext4` command as a hint. The lab formatter's skip of an already formatted partition therefore never fires on the fresh file its one caller passes.
- The HTTP install proof creates its target disk at exactly the image's size, so no proof installs on a larger disk today.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/install/disk/run.go` - `runHTTP` streams the image to the disk, syncs, asks the kernel to reread the table (a failure is a warning), waits for the node of partition 4, and calls `mountInjectDB`, then reboots. `runISO` writes the image from the medium, calls `finishISOWrite` (sync, reread, a failure is an error), unmounts the medium and powers off. It never mounts partition 4. Neither changes the table or the filesystem size
- [ ] `internal/install/disk/system.go` - `mountInjectDB` mounts partition 4 as ext4 read-write, keeps an existing live store or baked seed, and downloads the bootstrap database only for a seedless image. The function variables at the top are the syscall seams
- [ ] `internal/install/disk/blockdev_linux.go` - `sysBlkRereadPart` is the only block ioctl: the partition-table reread
- [ ] `internal/install/disk/mount_linux.go` - mount with the read-only flag or no flag, and an empty option string
- [ ] `internal/install/disk/detect.go` - `partitionPath` names a partition node for each disk naming scheme; `findTargetDisk` refuses more than one candidate
- [ ] `internal/install/disk/initrd_linux.go`, `internal/install/disk/bootstrap_linux.go`, `internal/install/disk/register.go`, `cmd/ze-installer/main.go` - one entry for PID 1 and one for `ze install disk`; both reach `runHTTP` and `runISO`
- [ ] `internal/appliance/cmd_build.go` - `runGokBuild` passes `image.size-bytes` as the target storage size. `injectZeFS` finds the last partition (`findLastPartition`, which reads entries with no checksum check), formats it with `mkfs.ext4` (quiet, forced, metadata checksums off, 4096-byte blocks, at the partition's offset), copies the partition out, writes the seed database and the build manifest into the `ze` folder with `debugfs`, sets the folder to root 0700 and the seed to root 0600, verifies, and writes the partition back. No quota or project feature is asked for
- [ ] `internal/appliance/diskverify.go` - `verifyInject` runs three checks: the seed bytes are in the image, `e2fsck` in forced read-only mode exits clean (skipped with a warning when the tool is absent), and a read-only loop mount on a Linux root build host
- [ ] `internal/appliance/config.go` - the default image is 2 GiB; accepted sizes are 512 MiB to 64 GiB
- [ ] `internal/appliance/push_image.go` - `readPushPartitions` validates the image file's boot sector, primary header, both checksums and the four entries before a push. It reads the image, never the device
- [ ] `internal/appliance/kernelreq.go` - `universalKernelRequirements` (installer floor) and `runtimeKernelRequirements` (runtime floor). Neither names a quota option
- [ ] `internal/appliance/cache.go` - `initrdSourceDirs` lists the two folders whose files key the initrd cache
- [ ] `internal/appliance/doctor_checks.go` - `checkE2fsprogs` reports a build host without `mkfs.ext4` and `debugfs`
- [ ] `vendor/github.com/gokrazy/tools/packer/packer.go` - `Partition` writes the boot sector, the primary header and entries, and the backup entries and header in the last 33 sectors of the target size. `writeGPT` lays out four partitions: 100 MB boot, two 500 MB roots, and the persistent partition from 1100 MB to 33 sectors before the end. `permSize` computes in 32-bit sector counts. `writePartitionTable` writes a protective entry that covers only the sectors before partition 1
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/mount.go` - `mountfs` mounts partition 4 on the persistent mount point, trying ext4, then vfat, then bcachefs, each with flags 0 and an empty option string. A failure is logged and the next type is tried
- [ ] `gokrazy/modcache/github.com/gokrazy/internal@v0.0.0-20260625065634-6994f9152c44/rootdev/rootdev.go` - the boot disk is found by the GUID of its first partition entry, and partition 4 by number. No checksum, size or backup header is read
- [ ] `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/kernel.require`, `gokrazy/kernel/runtime.require`, `tools/installer-kernel/kernel.config`, `tools/installer-kernel/kernel.require` - ext4 is requested and required in both bases. No quota option is in any of the six
- [ ] `internal/le/test/qemu/install_iso.go`, `internal/le/test/qemu/install_storage.go` - `installGPTEntries` reads the primary entries of a stopped disk; `assertInstallGPT` requires the first four to equal the image's; `installPerm` copies partition 4 out by its entry
- [ ] `internal/le/test/qemu/install_boot.go` - the HTTP proof truncates its target disk to the image's exact size before it boots the installer
- [ ] `internal/le/test/qemu/install_ota.go` - the update proof requires the persistent partition's identity to be the same before the push, after it and after recovery
- [ ] `internal/le/test/qemu/actions.go`, `internal/le/test/qemu/alltests.go` - the registered install actions, and `integrationPackages`, the closed list of packages the guest run tests
- [ ] `internal/le/setup/docker/host/perm.go` - `formatPerm` formats the docker-host image's persistent partition, found through its own GPT reader `permPartition`. The format is `mkfs.ext4` with the force flag, the partition's offset and its size in KiB: no feature list, no inode size, no journal size, so every property is the build host's default. `hasExt4` reads the two magic bytes of the superblock, and `formatPerm` returns without formatting when they are there. `findMkfs` resolves the tool through `appliance.ResolveE2FSTool`
- [ ] `internal/le/setup/docker/host/build.go` - `builder.build` is the one caller of `formatPerm`. It passes the partial file `gok` has just written, under a name that carries the process id, and renames it into the cache afterwards. An image already in the cache under its key is returned before `gok` runs and is never formatted again. `digestRoots` lists the folders whose files enter the image key: the lab tooling's own folder is one, `internal/appliance` is not
- [ ] `internal/le/setup/docker/host/actions.go` - the `check` action reports `mkfs.ext4` among the tools the image needs
- [ ] `internal/le/setup/tools.go`, `internal/le/setup/probes.go` - the development setup probes `mkfs.ext4` and `debugfs` as the e2fsprogs pair
- [ ] `internal/component/host/inventory.go`, `internal/component/host/storage_linux.go`, `internal/component/host/storage_other.go` - `show host storage` lists whole block devices with their sizes and skips partitions; it says nothing about the persistent partition
- [ ] `internal/plugins/host-cmd/cmd/register.go` - the host command plugin registers its show, kernel-log and file-descriptor commands from `init`
- [ ] `internal/component/doctor/checks_storage.go` - `checkDiskSpace` warns on a config partition with under 5% free
- [ ] `internal/core/diagnostic/codes.go` - the doctor codes, `doctor-disk-space` among them

**Behavior to preserve:**
- The image bytes reach the disk unchanged and are checked against their SHA-256 as today.
- Partitions 1 to 3, every partition GUID, the disk GUID, and the type and name of partition 4. gokrazy finds its disk and its root by them, and the OTA proof requires the persistent partition's identity to survive.
- The seed rules of `mountInjectDB`: an existing live store and a baked seed are kept.
- The ISO installer powers off and the HTTP installer reboots.
- The contents and modes the build puts on the persistent partition: the `ze` folder as root 0700, the seed database as root 0600, the build manifest.
- The push protocol, byte for byte.
- An image already in the docker-host cache is returned as it is. The cache key decides whether an image is current, never a look at its partition.

**Behavior to change:**
- The docker-host lab image's persistent partition gets the appliance's format, from the shared definition, and is formatted on every build: the skip of a partition that already holds an ext4 filesystem is deleted (D-4).
- After the image is written, the installer extends the partition table to the disk and grows the filesystem of partition 4.
- The ISO installer mounts partition 4 to grow it, then unmounts it.
- The build names every filesystem property itself, adds the project and quota features, and checks them.
- The runtime kernel and the installer kernels require quota support.
- `show host storage` reports the persistent storage, `ze doctor` reports storage smaller than its disk, and a command grows it.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Image-file GPT validation | `internal/appliance/push_image.go` `readPushPartitions` | Validates the boot-sector signature, the primary header, its checksum, the entry-array checksum and the bounds of four entries, on an image file | Replaces its parsing: the header and entry checks move to a new core leaf for partition tables, which also reads a block device. `readPushPartitions` keeps its push-only rules (four partitions, in order) over the shared parse |
| Last-partition finder | `internal/appliance/cmd_build.go` `findLastPartition` | Reads entries at a fixed offset with no checksum check and answers the offset and size of the highest one | Replaces: it calls the shared parse, so the build reads the table the way the installer does |
| Test-side GPT reader | `internal/le/test/qemu/install_iso.go` `installGPTEntries`, `assertInstallGPT`; `internal/le/test/qemu/install_storage.go` `installPerm` | Reads the primary entries of a stopped disk and compares four with the image | Extends: it uses the shared parse, and `assertInstallGPT` gains the grown expectation for entry 4 and the backup header |
| Docker-host GPT reader and formatter | `internal/le/setup/docker/host/perm.go` `permPartition`, `formatPerm` | Formats the persistent partition of the lab Docker host image, which is also the own-system QEMU guest: a plain `mkfs.ext4` with no feature list, skipped when the partition already holds an ext4 filesystem | Extends (D-4): `formatPerm` takes its format from the shared definition the appliance build uses, runs the same quota step and the same Go property check, and formats every image it is given, so the skip and `hasExt4` are deleted. `permPartition` calls the shared partition-table parse and keeps its own rule that entry 4 is a Linux filesystem partition. The image is still never installed by Ze's installer and is not grown |
| Partition-table writer | `vendor/github.com/gokrazy/tools/packer/packer.go` `Pack.Partition`, `writeGPT` | Writes the whole table for a target size on the build host, in 32-bit sector arithmetic | Does not apply as code: it is vendored, writes a whole new table, and cannot extend one. The grown table keeps what it wrote and changes four fields and the backup copy |
| Installer block-device seam | `internal/install/disk/system.go` function variables; `internal/install/disk/blockdev_linux.go` `sysBlkRereadPart` | One ioctl behind a function variable that tests replace | Extends: disk size, sector size and the growth call join the same seam pattern. The reread moves after the table rewrite |
| Installer mount of partition 4 | `internal/install/disk/system.go` `mountInjectDB` | Mounts partition 4 read-write for the seed, in HTTP mode only | Extends: the mount is where the filesystem is grown, and ISO mode gains a mount for it |
| Persistent partition formatter | `internal/appliance/cmd_build.go` `injectZeFS`; `InjectDatabase` for `internal/le/test/deployment/gokrazyimage.go` | The one formatter of an appliance image, through host e2fsprogs | Extends: the argument list names every property, and a step after the seed adds the quota feature. Both come from the shared format definition, which the lab formatter reads too |
| Build-side verification | `internal/appliance/diskverify.go` `verifyInject`, `verifyE2fsck` | Read-back of the seed, a structural check, a loop mount | Extends: a fourth check reads the superblock in Go and refuses a missing property. The structural check stays strict and unchanged |
| e2fsprogs tool resolution and its doctor check | `internal/appliance/cmd_build.go` `ResolveE2FSTool`; `internal/appliance/doctor_checks.go` `checkE2fsprogs`; `internal/le/setup/tools.go` | Finds `mkfs.ext4`, `debugfs` and `e2fsck` one by one and reports a missing one | Extends: `tune2fs` is resolved, required and reported the same way |
| Kernel requirement files and floors | `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `tools/installer-kernel/kernel.config`, `tools/installer-kernel/kernel.require`; `internal/appliance/kernelreq.go` | A build refuses a kernel whose emitted config lacks a required option | Extends: two quota options in both bases, both manifests and both floors |
| Kernel capability enrolment | `internal/component/kernelcap/kernelcap.go` `MustRegister` | One verdict for doctor, start, reload and validate when a config uses a feature | Does not apply here: no config uses this facility. The image store enrols its own capability and this spec pins the option that enrolment names |
| Host inventory | `internal/component/host/storage_linux.go` `DetectStorage`; `show host storage` | Lists block devices from sysfs | Extends: a `persistent` object with the partition, disk and filesystem sizes and the quota state |
| Host command plugin | `internal/plugins/host-cmd/cmd/register.go` (`set system file-descriptors`, `show host`) | Owns host commands with their YANG and delegates to the host component | Extends: one `request` command |
| Storage doctor check | `internal/component/doctor/checks_storage.go` `checkDiskSpace` | Warns when the config partition is nearly full | Extends beside it: one check for storage smaller than its disk. The free-space check is unchanged |
| First-boot hook on the appliance | `cmd/ze/ze_core_autoinit.go` `gokrazyAutoInit` | Imports the seed when `ze.gokrazy.enabled` is set | Does not apply under the recommendation for D-1. If the owner chooses growth at boot, the call sits beside it |
| Free-space reading | `internal/core/diskspace/diskspace.go` | Reads a filesystem's total and free blocks | Extends by use: the filesystem size in the storage report comes from it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The build host: `ze appliance build <name>` formats the persistent partition inside the image file.
- The installer: the kernel command line selects HTTP or ISO; the image arrives as bytes over HTTP or from the medium.
- The device: `request system storage grow` on any command surface, and `ze doctor`.
- A consumer: Go calls into the quota leaf from a process that runs as root.

### Transformation Path

At build:
1. `injectZeFS` formats partition 4 with the properties in "Design" below, including the project feature.
2. It writes the seed and the manifest with `debugfs`, as today.
3. It adds the quota feature for the project type and stores the project-quota mount option in the superblock, with `tune2fs`, after the files are written.
4. `verifyInject` reads the seed back, reads the superblock properties in Go, and runs the structural check, which also proves the quota usage record matches the files.

At the lab image build (`./le setup docker host build`, and every action that ensures the image):
1. `builder.build` runs `gok` into a partial file, as today.
2. `formatPerm` finds partition 4 through the shared parse and makes the filesystem with the format step and the quota step of the shared definition. No file is written between the two, because the lab image carries no seed.
3. It reads the superblock in Go with the check the appliance build uses, and refuses an image whose partition lacks a property, naming it.
4. The image is renamed into the cache under a key that covers the shared definition.

At install:
1. The image is written and synced, as today.
2. The installer reads the disk's size and sector size, parses and validates the primary table, and computes the grown table.
3. It writes the backup entries and header at the end of the disk, syncs, then writes the primary entries and header, syncs, and clears the stale backup copy the image carried.
4. It asks the kernel to reread the table, and waits for the node of partition 4.
5. It mounts partition 4. In HTTP mode this is the existing mount; the seed step runs first.
6. It asks the kernel to resize the mounted filesystem to the partition, syncs and unmounts.
7. It reboots or powers off, as today.

On a device, on the command:
1. The plugin finds the device mounted on the persistent mount point, its partition number and its disk from the mount table and sysfs.
2. It computes and writes the grown table as the installer does.
3. It asks the kernel to resize partition 4 in place, because the disk has mounted partitions and a full reread is refused.
4. It asks the kernel to resize the filesystem.
5. It answers the sizes before and after.

A consumer, per folder:
1. It marks an empty folder with a project id and the inherit flag.
2. It sets a hard block limit for that id.
3. It reads the usage for that id.

### Design

**The grown partition table.** All arithmetic is 64-bit.

| Field | Image value | Grown value |
|-------|-------------|-------------|
| Primary header: backup pointer | The image's last sector | The disk's last sector |
| Primary header: last usable sector | 33 sectors before the image's end | 33 sectors before the disk's end |
| Entry 4: last sector | The image's last usable sector | The disk's last usable sector, lowered so the partition is a whole number of 4096-byte blocks |
| Entry-array checksum, header checksum | Over the image values | Recomputed |
| Backup entries and header | In the image's last 33 sectors | In the disk's last 33 sectors, with the header's own and backup pointers swapped; the old copy is zeroed |
| Everything else (disk GUID, partition GUIDs, types, names, entries 1 to 3, first usable sector, the boot sector) | - | Byte for byte unchanged |

| Rule | Reason |
|------|--------|
| The backup copy is written and synced before the primary | At every instant one valid table describes a partition 4 that holds the whole filesystem: the old primary, or the new backup at the disk's last sector when the primary's checksum fails mid-write, or the new primary. UEFI firmware and GPT tools fall back to that backup. Whether the Linux kernel does without the `gpt` boot option is A-15: recalled as no, in which case a stop inside the two-sector primary write leaves a disk the kernel reads with no partition until the firmware or a tool restores the primary from the backup |
| A table whose entry 4 already ends at the last usable sector is left alone | The step is safe to repeat, and a disk the size of the image gets no write |
| Growth is refused, with the reason, when the logical sector size is not 512, the primary table fails a checksum, the fourth entry is not the highest, or the disk is smaller than the table says | Each is a disk this code must not guess about |
| The table never shrinks | A smaller disk already fails at the image write |

**Growing the filesystem: the approaches.**

| # | Approach | External tool | What it must carry | Verdict |
|---|----------|---------------|--------------------|---------|
| A | The kernel's online resize of the mounted ext4, called by the installer after it mounts partition 4, and by the grow command on a device | None | One ioctl on the mount point with the new block count | Recommended |
| B | The same resize, only at first boot by `ze` | None | The same ioctl, plus a partition resize under mounted partitions on every device | Set aside as the only route: the owner ruled growth at install, and the boot path would write a partition table. Its code is what the grow command runs |
| C | Create the filesystem fresh at install, at the disk's size | None, so a Go ext4 writer | A journal, flexible groups, a quota inode, and everything the build injected: the `ze` folder with its mode, the seed with its mode, the build manifest | Rejected: no ext4 writer is in the tree (the build's formatter is host `mkfs.ext4`, resolved by `ResolveE2FSTool`), and `docs/architecture/appliance/on-device-installer.md` records a pure-Go writer as studied and rejected |
| D | Build the image at the size of the disk | None | An image per disk size, capped at 64 GiB by `maxImageSize` | Rejected: one image no longer fits every device |

Under A, the filesystem keeps what the build chose, for life. The build therefore fixes these properties and no longer inherits the build host's defaults.

| Property | Today | New | Why |
|----------|-------|-----|-----|
| Feature list | The host's `mkfs.ext4` defaults, metadata checksums off | Named in full in the argument list: journal, extents, huge files, flexible groups, 64-bit, directory link count, extra inode size, resize inode, sparse superblocks, large files, file types, directory index, extended attributes, project. Metadata checksums stay off | A build on another host must give the same filesystem. Online resize needs flexible groups or the resize inode. 64-bit block numbers lift the 16 TiB bound |
| Inode size | The host default | 256 bytes | The project id lives in the extended part of the inode |
| Bytes per inode | The host default for the size class | 16384 | The inodes per group are fixed at format and carry to every group the resize adds |
| Reserved group-descriptor area | The default: room to grow about 1024 times | The largest the block size allows, about 8 TiB | Growth inside the reserved area needs no format conversion by the kernel (A-5) |
| Journal | Sized for the image partition, about 16 MB | 64 MiB | The journal cannot grow with the filesystem, and layer extraction is metadata heavy |
| Quota feature | Absent | Present, project type only, added after the seed is written | The usage record is then computed from the files that are there (A-7) |
| Stored mount option | None | Project quota | gokrazy mounts with no option; the kernel applies the stored one (A-3) |

**One definition of the format, two formatters.** The table above is declared once, in a new package `internal/appliance/permformat`: the format step's arguments, the quota step's arguments and the list of properties the Go check requires. `injectZeFS` and `formatPerm` both derive from it, and neither names a property of its own.

| Question | Appliance build (`injectZeFS`) | Lab image build (`formatPerm`) |
|----------|--------------------------------|--------------------------------|
| Format step | The definition's arguments, at the partition's offset in the image | The same arguments |
| Files written before the quota step | The seed and the manifest | None |
| Quota step | The definition's `tune2fs` arguments, on the copy of the partition the build already makes | The same arguments, on a filesystem file (A-12) |
| Check | The Go property check | The same check |
| Size of the partition | Under 1 GiB on the default image | About 63 GiB on the default lab image, in a sparse file |

`tune2fs` takes a device or a file that is the filesystem and has no offset argument, so it cannot work on a partition inside an image. The appliance build copies the partition out, in memory, and writes it back. The lab partition is too large for that, so the lab formatter makes the filesystem in a sparse file of the partition's size and copies it into the image at the partition's offset without filling the holes (A-12).

**The skip of an already formatted partition.** `formatPerm` skips a partition that holds an ext4 magic. Two ways to stop an old format surviving through it were weighed.

| # | Way | Verdict |
|---|-----|---------|
| S1 | Always format: delete the skip and `hasExt4` | Chosen. The one caller passes the partial file `gok` has just written, and no packer in the tree formats partition 4: the vendored `packerwrite.go` and both module-cache copies only log the `mkfs.ext4` command as a hint. The skip therefore guards a case that does not occur, the format already runs forced, and one path has nothing to disagree with |
| S2 | Reformat only when the superblock lacks a required property | Set aside. It keeps two outcomes for one input and makes the property check a decision instead of a refusal. A filesystem that passes the check and still differs in a property the check does not read would survive |

A lab image built before this work stays in the cache under its old key. The image key must change with the format: the folder of the shared definition joins `digestRoots`, so the next build makes a new image (R-10).

**Enforcement without a mount option.** gokrazy's init passes no option, and Ze does not patch it for this. The lab image is a gokrazy image too, so the same route gives the own-system guest its enforcement.

| # | Route | Step at run time | Failure mode | Verdict |
|---|-------|------------------|--------------|---------|
| E1 | The project-quota option is stored in the superblock at build; the kernel applies it at every mount | None | A kernel that refuses the stored string fails the mount. The first QEMU boot shows it | Recommended |
| E2 | `ze` turns enforcement on at start with the quota-on call, on the appliance | One call before any consumer, at every boot | A failure leaves the partition mounted and usable, with a doctor finding | The named fallback if A-3 is false |
| E3 | A diff to gokrazy init's mount | None | A filesystem without the feature fails the mount and init falls through to other types | Rejected: it couples the mount to the format of every image ever booted |

**What each way of getting a system does to the persistent partition.**

| Path | Today | After this spec |
|------|-------|-----------------|
| PXE (HTTP) installer | Image written raw; partition 4 mounted for the seed; size is the image's | Table extended, filesystem grown while mounted for the seed |
| ISO installer, Ventoy included | Image written raw; partition 4 never mounted | Table extended; partition 4 mounted, grown, unmounted |
| `ze install disk` | The same code as the two rows above | The same growth |
| A raw image written by hand, `ze appliance run`, a cloud disk | Size is the image's | Not grown at install. `ze doctor` reports it, `show host storage` shows it, and `request system storage grow` grows it online with no reboot (D-1) |
| OTA push | Replaces the inactive root, the boot filesystem and the boot code; never the table or partition 4 | Unchanged. A grown device stays grown. A device formatted before this work keeps its old format |
| The docker-host lab image, booted as the Colima profile or as the own-system QEMU guest | A plain format with the build host's defaults; the partition is the image's size, 64 GiB by default | The appliance's format, with project quota enforced (AC-14). Not grown: the image is booted as built and never installed |

**The facility a consumer gets.** Three new core leaf packages, each with its syscalls in Linux-only files: `gptdisk`, `ext4fs` and `projquota`.

| Package | Operation | Mechanism |
|---------|-----------|-----------|
| `gptdisk` | Parse and validate a table from an image file or a disk; compute and write the grown table | Pure Go |
| `ext4fs` | Read the superblock properties; resize a mounted filesystem to a block count | Pure Go read; the ext4 resize ioctl on the mount point |
| `projquota` | Answer whether project quota is enforced on a path's filesystem: enforced, not enforced, no feature, or unknown with the reason | The quota state call for the project type |
| `projquota` | Give a folder a project id that new entries inherit | The project-attribute ioctl pair on the folder, with the inherit flag |
| `projquota` | Set a hard block limit for a project id; read its usage and limit | The quota set and get calls for the project type, on the block device |

**The operator's view.** `show host storage | json` gains one object.

| Field | Type | Description |
|-------|------|-------------|
| `persistent.device` | string | The partition node mounted on the persistent mount point |
| `persistent.disk-bytes` | integer | The size of the disk that holds it |
| `persistent.partition-bytes` | integer | The size of the partition |
| `persistent.filesystem-bytes` | integer | The size of the filesystem |
| `persistent.grown` | boolean | The partition reaches the last usable sector and the filesystem fills the partition |
| `persistent.project-quota` | string | `enforced`, `not-enforced`, `no-feature` or `unknown` |

Doctor, on a device whose storage is smaller than its disk:

| Severity | Code | Message (sample) | Fix |
|----------|------|------------------|-----|
| warning | `doctor-storage-not-grown` | persistent storage /dev/sda4 uses 944 MiB of a 238 GiB disk, because it was not grown to the disk when this system was installed | `request system storage grow` |
| warning | `doctor-storage-not-grown` | cannot read the size of the persistent storage, so it may be smaller than its disk: (the cause) | `show host storage` |

The cause in the first message covers both ways a system keeps the size of its image: it was not installed by the installer, or the growth failed at install (D-3). The size in use is the filesystem's, so a partition that was grown around a filesystem that was not still reads as the small size.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build host to device | The image file: its partition table and the superblock of partition 4 | No |
| Installer to disk | Raw writes of the table through the disk node; block ioctls | No |
| Installer or daemon to kernel | The reread and partition-resize ioctls, the ext4 resize ioctl, mount | No |
| Consumer to kernel | The quota calls and the project-attribute ioctls | No |
| Operator to plugin | The command registry, on every surface | No |

### Changed-Wire Census

**Search evidence:** searched the first-party Go trees, the vendored gokrazy trees, the gokrazy module cache and the workflow, shell and Python files for the GPT signature string, `gptEntryLBA`, `findLastPartition`, `PartitionUUIDs`, `installGPTEntries`, the reread ioctl name and `readPushPartitions`; and the same trees, with the functional test files and Dockerfiles, for `mkfs.ext4`, `mke2fs`, `debugfs`, `e2fsck`, `tune2fs`, `resize2fs`, `InjectDatabase` and `injectZeFS`. The positive controls are `readPushPartitions` for the table and `injectZeFS` for the filesystem; both searches find them. The table search found one reader this spec did not start from (`permPartition` in the docker-host tooling), and the tool search found the second formatter beside it. No shell or Python carrier of either wire was found. The kernel and the firmware are readers outside the repository.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Partition table (GPT) | writer | `vendor/github.com/gokrazy/tools/packer/packer.go` `Pack.Partition`, `writeGPT` | Writes the whole table for the image size at build | Preserved. The image's table is unchanged |
| Partition table (GPT) | writer | `internal/core/fetch/fetch.go` `ToDisk`; `internal/install/disk/iso.go` `localImageToDisk` | Copy the image's bytes, table included, to the disk | Preserved. The copy stays raw |
| Partition table (GPT) | writer | The new partition-table leaf, called from `internal/install/disk/run.go` and the grow command | No writer of a disk's table exists on the device today | New. Changes four header and entry fields and moves the backup copy |
| Partition table (GPT) | reader | The Linux kernel's partition scan | Outside the repository | Preserved: the grown table is a stricter-valid GPT than the image's on a larger disk, whose backup is not at the end |
| Partition table (GPT) | reader | UEFI firmware | Outside the repository | Preserved: partition 1 and its GUID are unchanged (A-11) |
| Partition table (GPT) | reader | gokrazy `findGPTPartUUID` in `gokrazy/modcache/github.com/gokrazy/internal@v0.0.0-20260625065634-6994f9152c44/rootdev/rootdev.go` and in `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/mount.go` | Read the entry GUIDs only | Preserved: no GUID changes |
| Partition table (GPT) | reader | `internal/appliance/push_image.go` `readPushPartitions` | Validates the image file before a push; never reads the device | Preserved in behavior; its parse moves to the shared leaf |
| Partition table (GPT) | reader | `internal/appliance/cmd_build.go` `findLastPartition` | Finds partition 4 of the image for the formatter | Preserved in behavior; its parse moves to the shared leaf |
| Partition table (GPT) | reader | `internal/le/test/qemu/install_iso.go` `installGPTEntries`, `assertInstallGPT`; `internal/le/test/qemu/install_storage.go` `installPerm` | `assertInstallGPT` requires four entries equal to the image's | Changed: entry 4 is expected grown, entries 1 to 3 equal. `installPerm` reads the entry bounds and is preserved |
| Partition table (GPT) | reader | `internal/le/setup/docker/host/perm.go` `permPartition` | Its own reader, over the docker-host image, which is never installed | Preserved in behavior; its parse moves to the shared leaf, and it keeps its rule that entry 4 is a Linux filesystem partition |
| Persistent-partition ext4 superblock | writer | `internal/appliance/cmd_build.go` `injectZeFS`, reached also through `InjectDatabase` from `internal/le/test/deployment/gokrazyimage.go` | The one appliance formatter | Changed: named properties, the project and quota features, the stored mount option |
| Persistent-partition ext4 superblock | writer | The kernel's online resize, called from the new filesystem leaf | No caller exists today | New: block count, group count and free counts grow |
| Persistent-partition ext4 superblock | writer | `internal/le/setup/docker/host/perm.go` `formatPerm` | A plain format for the lab Docker host image, which is the own-system guest | Changed (D-4): the same named properties, project and quota features and stored mount option as the appliance formatter, from the shared definition. Formats every image it is given |
| Persistent-partition ext4 superblock | reader | gokrazy `mountfs` in `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/mount.go`, through the kernel's ext4 mount | Flags 0, no option, on the appliance and in the lab image | Preserved: mounts as before, and the kernel applies the stored option (A-3). A kernel without quota support refuses the mount read-write (A-2), so both kernels gain the option |
| Persistent-partition ext4 superblock | reader | `internal/install/disk/system.go` `mountInjectDB`, through the installer kernel's mount | Mounts read-write | Preserved, with the installer kernel's quota support required |
| Persistent-partition ext4 superblock | reader | `internal/appliance/diskverify.go` `verifyE2fsck`, `tryLoopbackVerify` | Host `e2fsck` and a read-only host mount | Preserved: the structural check stays strict and must be clean with the new features |
| Persistent-partition ext4 superblock | reader | `internal/le/test/qemu/install_storage.go` (host `debugfs` on a copy of the partition) | Reads and writes files in the installed partition | Preserved: host `debugfs` reads the new features (A-8) |
| Persistent-partition ext4 superblock | reader | The new filesystem leaf's superblock read; the image store's probe (`spec-container-2-image-store.md`) | No Go reader of these fields exists; gokrazy's `probeExt4` reads the UUID only | New |

### Integration Points
- `internal/install/disk/run.go` `runHTTP`, `runISO`, `finishISOWrite` - the growth step sits between the image write and the reread, and the resize sits inside the mount.
- `internal/appliance/cmd_build.go` `injectZeFS` and `internal/appliance/diskverify.go` `verifyInject` - the format and its check.
- `internal/le/setup/docker/host/perm.go` `formatPerm` and `internal/le/setup/docker/host/build.go` `digestRoots` - the same format for the lab image, and the image key that covers it.
- `spec-container-1-runtime.md`, `spec-container-2-image-store.md` and `spec-container-3-network.md` - their guest tests run in the own-system guest on its own persistent partition and rest on AC-14 of this spec. Without AC-14 that guest reports `no-feature` and a container config is refused there.
- `internal/appliance/kernelreq.go` - both floors.
- `internal/appliance/cache.go` `initrdSourceDirs` - the leaf packages the installer imports.
- `internal/component/host/storage_linux.go` `DetectStorage` and `internal/plugins/host-cmd/cmd/register.go` - the report and the command.
- `internal/core/diagnostic/codes.go` and the doctor registry - one code.
- `spec-container-2-image-store.md` - calls the `projquota` leaf for every quota operation (probe the state, give a folder a project id, set a limit, read usage), has no quota code of its own, never turns enforcement on, and makes no filesystem. It reads the leaf's four states as its own capability: `enforced` is present, `not-enforced` and `no-feature` are absent, `unknown` is cannot-determine. Its containers MUST NOT hold `CAP_SYS_RESOURCE`, which passes every hard limit (R-12). This spec owns the partition's format, the kernel quota pins and that one shared leaf.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel resizes a mounted ext4 filesystem to a larger block count through the ext4 resize ioctl on the mount point, for a root caller, when the filesystem has flexible groups and no cluster allocation | Recalled from the kernel's ext4 resize code. Not read on this machine | Growth needs a Go ext4 writer (approach C), which is a design change put to the owner | `TestOnlineResizeGrowsMountedFilesystem` in the QEMU guest; `./le test qemu installer` | holds: `TestOnlineResizeGrowsMountedFilesystem` passed in the guest, run as root on Ze's 7.3 runtime kernel (2026-11-16): the fixture grew while mounted, its data was intact after a remount, and a shrink was refused. `./le test qemu installer` has not run on this work yet |
| A-2 | A kernel built without quota support refuses to mount, read-write, a filesystem that has the quota or project feature | Recalled from the ext4 mount code. Not read | The quota options in the installer kernel are a safety pin rather than a requirement; nothing else changes | The kernel floors refuse a kernel without the options in every case, so the question does not reach a device. Recorded as recalled | unvalidated |
| A-3 | The kernel applies a project-quota option stored in the superblock at every mount, and enforcement is then on with no caller step | Recalled: stored mount options are parsed at mount. Whether a quota option is accepted there was not confirmed | The design moves to E2: `ze` turns enforcement on at start on the appliance | `./le test qemu appliance-storage` reads `project-quota` as `enforced` on a booted appliance; `TestGuestPersistentPartitionEnforcesProjectQuota` reads it on the own-system guest (AC-14) | holds on the fixture: `TestProjectQuotaStoredOptionEnforcesAtMount` passed in the guest on 7.3 (2026-11-16), the format fixture mounted with no option string reading `enforced`. Still owed on a booted appliance (`./le test qemu appliance-storage`) and on the own-system guest's own partition (AC-14) |
| A-4 | With the quota feature and the project inode present, the kernel accounts usage from mount, and the quota-on call turns enforcement on for a mounted filesystem | Recalled from the generic quota code. Not read | E2 is not available as the fallback, and E3 or a consumer-side remount is put to the owner | `TestProjectQuotaEnableOnMountedFilesystem` in the QEMU guest | holds: `TestProjectQuotaEnableOnMountedFilesystem` passed in the guest on 7.3 (2026-11-16): usage was accounted with enforcement off, and the quota-on call turned enforcement on for the mounted fixture. The E2 fallback stays available; A-3 holding on the fixture means it is not needed |
| A-5 | Growth inside the reserved group-descriptor area needs no conversion. Beyond it the kernel converts the filesystem to the meta-group layout and continues | Recalled. The second sentence is the weaker one | A disk above about 8 TiB grows only to the bound. Doctor reports the rest as not grown, with that reason | `TestOnlineResizeGrowsMountedFilesystem` at a size inside the bound. The case beyond the bound has no test: see Known Limitations | holds inside the bound: `TestOnlineResizeGrowsMountedFilesystem` passed in the guest on 7.3 (2026-11-16), growing the fixture from 256 MiB to 1 GiB. The second sentence (beyond the bound) stays unvalidated |
| A-6 | A partition on a disk with mounted partitions is resized in the kernel by the partition-resize ioctl, and a full reread is refused there | Recalled. The operation and its argument types are in the vendored system-call package; the ioctl number `BLKPG` is read from `/usr/include/linux/fs.h` (`_IO(0x12,105)`, 0x1269) and the structures from `/usr/include/linux/blkpg.h` | The grow command needs a reboot to take effect, and says so | `TestGrowLivePartitionWhileMounted` in the QEMU guest (early, on a loop disk: the reread refused with EBUSY, `BLKPG_RESIZE_PARTITION` taken, the mounted ext4 resized into it); `./le test qemu appliance-storage`, which grows a booted device | unvalidated: the guest test is written (phase 3) and has not run |
| A-15 | The Linux kernel's GPT reader reads the backup copy only through the primary header's alternate pointer, unless the `gpt` boot option forces it, so a primary that fails its checksum gives a disk with no partition | Recalled from `block/partitions/efi.c` `find_valid_gpt`. Not read: no kernel source is on this machine | If wrong (the kernel falls back), the write-order rule holds for the kernel as written. If right, a stop inside the two-sector primary write (entries sector, then header sector) leaves an installed disk the kernel reads unpartitioned until UEFI firmware or a GPT tool restores the primary from the valid backup. Fallback F-1, put to the main thread: the appliance's kernel command line gains `gpt`, so the kernel reads the backup at the last sector itself | `TestKernelReadsATableAtEveryStopOfTheRewrite` in the QEMU guest: red at the stops inside the primary write means the recall holds | unvalidated: the guest test is written (phase 3) and has not run |
| A-16 | The kernel lets the table be written through the whole-disk node while partition 4 is mounted (`CONFIG_BLK_DEV_WRITE_MOUNTED` is set, and the restriction binds the mounted partition's node only) | The host's own 7.2.9-ze kernel config reads `CONFIG_BLK_DEV_WRITE_MOUNTED=y`; the 7.3 runtime config was not read, and the binding is recalled | The grow command cannot write the table of a live disk; growth on a device needs a reboot into a step before the mount | `TestGrowLivePartitionWhileMounted` in the QEMU guest (its Grow call writes through `/dev/loopN` with `loopNp4` mounted) | unvalidated |
| A-17 | A loop device configured with `LO_FLAGS_PARTSCAN` scans its table inside `LOOP_CONFIGURE`, the kernel's GPT reader accepts the packer's hybrid MBR (a 0xEE record from sector 1 beside the boot partition's record), and it names partition N by entry N | Recalled. The flag is in the vendored system-call package | The guest tests read no partition and fail at attach; the hybrid record is what gokrazy has always booted with, so only the test's timing assumption is at risk | Both gptdisk guest tests fail at their first partition read if any part is wrong | unvalidated |
| A-7 | `debugfs` writes leave the quota usage record behind the files, and `tune2fs`, adding the quota feature afterwards, computes usage from the inodes | Recalled from e2fsprogs. The image store research met the first half as a concern | If `tune2fs` does not compute it, the build's structural check fails and the format needs another order | `TestInjectedImagePassesStructuralCheck` with host e2fsprogs; every QEMU image build | unvalidated |
| A-8 | The host e2fsprogs every build host carries (Debian, Alpine, Homebrew) knows the project feature and the stored mount option | The feature is years old. Versions were not read | The build refuses on that host with the tool and version named | The image builds of the QEMU actions on Linux, and one build on the owner's Mac | unvalidated |
| A-9 | arm64 kernels resolve the two quota options as built in once requested | The amd64 kernel this host runs, built from Ze's runtime profile, has both by upstream default. No arm64 config was read | The arm64 kernel build refuses, by design, until the fragment is right | An arm64 runtime and installer kernel build on the Mac | unvalidated |
| A-10 | The resize of a filesystem without group-descriptor checksums finishes in an acceptable time on a large disk | Not known. Metadata checksums are off in the build today, and the commit that carries the flag (`e6069ffd19`) only moved the code and gives no reason | Install time grows with the disk. The format then gains group-descriptor checksums, after the reason they are off is found | The duration `./le test qemu installer` records for the resize on its larger disk, and one run on a 1 TiB sparse disk | measured on the fixture: `TestOnlineResizeGrowsMountedFilesystem` logged "A-10: the online resize from 65536 to 262144 blocks took 271.457365ms" in the guest on 7.3 (2026-11-16), 256 MiB to 1 GiB. The installer's larger disk and the 1 TiB run are still owed |
| A-11 | UEFI firmware and the boot code find the system on a disk whose table was extended | The first partition, its GUID and the boot sector are unchanged | The installed device does not boot | `./le test qemu installer-iso`, which boots the installed disk under UEFI | unvalidated |
| A-12 | `tune2fs` has no way to reach a filesystem at an offset inside an image file, and `mkfs.ext4` cannot store the mount option, so the lab formatter needs the filesystem as a file of its own. A sparse file of the lab partition's size is formatted, given the quota step and copied into the image without filling its holes, in a time and a disk use close to today's build, on Linux and on the Mac | The manual pages on this host: `mke2fs` lists the offset and the quota type among its extended options and no stored mount option; `tune2fs` lists the stored mount option and the quota switch and no offset. The copy's cost was not measured | If the copy is slow or fills the file, the lab image build gets slower or larger, and the route changes to a loop device on Linux, which needs root. The format itself is not affected | `TestFormatPermGivesTheApplianceFormat` with host e2fsprogs, which also checks the image's allocated size; one lab image build on Linux and one on the Mac | unvalidated |
| A-13 | On a filesystem without the quota feature, the quota state query (`Q_XGETQSTATV`) answers with every state flag 0, so `Probe` reads `no-feature` from the flags | Recalled from the quota code. Not read | `Probe` cannot tell a filesystem without the feature from one it failed to query, and reads it from elsewhere | `TestProjectQuotaStateWithoutFeature` in the QEMU guest | corrected: WRONG. On 7.3 the state query is refused with ENOSYS on such a filesystem (`TestProjectQuotaStateWithoutFeature` failed on it in the guest, 2026-11-16). `Probe` now reads the feature bits from the superblock when the query is refused (`probeRefusedState`, `internal/core/projquota/quota_linux.go`). The fixed test has not been rerun yet at the time of writing |
| A-14 | `s_feature_ro_compat` is at offset 0x64 of the ext4 superblock, with the quota feature at bit 0x0100 and the project feature at bit 0x2000 | Recalled from the ext4 on-disk layout. Not read | `ext4fs.Superblock.QuotaFeature` and `ProjectFeature` read the wrong bits, and `Probe` reads a filesystem without the feature as `unknown` or one with it as `no-feature` | `TestProjectQuotaStateWithoutFeature` in the QEMU guest, which requires both bits set on the format fixture and only the project bit on the format-step fixture, so an offset or bit read from the wrong place fails there | unvalidated: the test that settles it has not run since the bits were added |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A defect in the table rewrite destroys a freshly installed disk | An install that does not boot | The write order keeps one valid table at every instant, the rewrite changes four fields, and a unit test stops the writer at each write and re-parses |
| R-2 | The grow command damages a running device | The persistent partition unreadable after the command | The same code and order as the installer; the partition only grows; the QEMU action checks the data after growth and after a reboot |
| R-3 | A new image meets a kernel without quota support and its persistent partition does not mount | The HTTP install fails at the seed step, or a booted device has no persistent storage | Both kernel floors require the options, so no kernel Ze builds lacks them |
| R-4 | An installer kernel or initrd from before this change is served from a cache or a download | Growth is absent after an install with new code in the tree | The initrd cache key gains the new packages. A downloaded installer kernel has no staleness check: the failure is named at the seed mount, and the fix is a rebuilt installer kernel |
| R-5 | The build's format differs between build hosts | Two builds give different superblocks | Every property is named in the argument list, and the Go superblock check refuses a difference |
| R-6 | A disk with 4096-byte logical sectors | The grow step refuses | The refusal names the sector size. The image's table is in 512-byte units (`gptSectorSize`, the packer's `writeGPT`), so such a disk is outside what the image supports today |
| R-7 | The image store lands first and finds no enforced quota | Its capability reads absent everywhere | It declares this spec as a dependency |
| R-8 | `request system storage grow` runs while a consumer writes heavily | The command takes long | The kernel's resize is online by design; the command holds no Ze lock and reports its duration |
| R-9 | The 64 MiB journal and the reserved area take about a tenth of a 944 MB partition before growth | A seed or a first-boot import that no longer fits on an ungrown device | The build fails if the seed does not fit, and the storage report shows the free space |
| R-10 | A lab image built before this work, or before a later change of the format, is served from the docker-host cache | The own-system guest reports `no-feature`, and AC-14 is red on a machine that has an old image | The folder of the shared definition is in `digestRoots`, so the image key changes with the format, and `TestImageKeyCoversThePermFormat` holds it there |
| R-12 | A writer holding `CAP_SYS_RESOURCE` passes every hard quota limit (recalled from the kernel's `fs/quota/dquot.c` `ignore_hardlimit`; the phase 2 quota tests write from a thread that dropped it, because the test runs as root) | A container fills its folder past its cap, and the cap reads as enforced | A constraint on the image store child (`spec-container-2-image-store.md`): a container whose size cap is to hold MUST NOT hold `CAP_SYS_RESOURCE`. The runtime's default set (`defaultCapabilities`, `internal/plugins/container/describe_linux.go`) is thirteen capabilities and does not include it, so any change that adds it, by default or on operator request, breaks the cap and must be refused or reported for a capped container |
| R-11 | The two formatters drift apart again | A property of the appliance differs in the guest, and a container test passes in the guest on a format no device has | Neither formatter names a property: both read the shared definition, and both run the same Go check on their result |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An install that does not boot, or a device whose persistent partition does not mount. Both show in the first QEMU install. Routing code is not touched |
| How is it reverted? | The commits revert. A device installed in between keeps a grown partition and the new format, which older code mounts as long as its kernel has quota support |
| Who else touches this path? | `spec-container-2-image-store.md` (the consumer), `spec-container-1-runtime.md` and `spec-container-3-network.md` (their guest tests rest on AC-14), every session that boots the own-system guest or the Colima profile (the next lab image build makes a new image under a new key), `plan/release/fixes/spec-crash-capture.md` (writes to the persistent partition; its Required Reading says the partition is "grown to fill the disk at build", which is the image, not the disk), `plan/post-release/features/spec-install-9-cloud-init.md` (a raw image booted on a larger disk) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The installer booted with the HTTP source | → | `runHTTP`, then the table growth and the filesystem resize | `TestRunHTTPGrowsPersistentPartition`; `./le test qemu installer` |
| The installer booted from the ISO | → | `runISO`, then the same growth with its own mount | `TestRunISOGrowsPersistentPartition`; `./le test qemu installer-iso` |
| `request system storage grow` | → | The host command handler, then growth on a mounted device | `test/appliance/storage-grow-qemu.ci` |
| `ze doctor` on an ungrown device | → | The storage check | `test/appliance/storage-grow-qemu.ci`; `TestStorageDoctorReportsStorageSmallerThanDisk` |
| `show host storage` | → | `DetectStorage` persistent object | `TestDetectStoragePersistentObject`; `test/appliance/storage-grow-qemu.ci` |
| `ze appliance build <name>` | → | `injectZeFS` format and `verifyInject` | `TestInjectFormatsPersistentPartition`; `TestInjectedImagePassesStructuralCheck` |
| A consumer's call into the quota leaf | → | The quota leaf on the runtime kernel | `TestProjectQuotaRefusesWritePastLimit` in `./le test qemu all` |
| A kernel build | → | `enforceKernelRequirements` with the two floors | `TestKernelFloorsRequireQuota` |
| `./le setup docker host build`, and the boot of the own-system guest that ensures the image | → | `builder.build`, then `formatPerm` with the shared definition | `TestFormatPermGivesTheApplianceFormat`; `TestGuestPersistentPartitionEnforcesProjectQuota` in `./le test qemu all` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The HTTP installer installs the default image on a disk larger than the image | The installed disk has a valid primary and backup table, the backup header is in the disk's last sector, entry 4 ends at the last usable sector, and the filesystem's size is the partition's. The device boots, and `show host storage` reports `grown` true with the three sizes | The last 33 sectors and a remainder under 4096 bytes stay outside the partition, because the table and the block size need them. Entries 1 to 3 equal the image's |
| AC-2 | The ISO installer does the same | The same table and sizes as AC-1, the device boots under UEFI, and the installer still powers off | An untargeted second disk is byte for byte unchanged, as the ISO proof already requires |
| AC-3 | Either installer installs on a disk exactly the size of the image | The install completes, the primary table is byte for byte the image's, and the console says the storage already uses the whole disk | The table gets no write, because there is nothing to extend |
| AC-4 | A raw image is copied to a larger disk and booted | `ze doctor` reports `doctor-storage-not-grown` as a warning with both sizes and the command that fixes it, and `show host storage` reports `grown` false | Every other function works, because the image-sized partition is a complete system |
| AC-5 | The operator runs `request system storage grow` on that device | The answer carries the sizes before and after. Partition and filesystem are grown with no reboot, the config and the login survive, the state survives a reboot, and doctor no longer reports the finding. A second run answers that the storage already uses the whole disk and writes nothing | A device on a disk the size of its image gets the second answer at once |
| AC-6 | The command runs where the persistent mount point is not an ext4 filesystem on the fourth GPT partition of a disk with 512-byte sectors, or on a host that is not the appliance | It refuses with the reason and what the operator can do, and every disk keeps its bytes | `show host storage` still answers, with `persistent` absent on a host that has no persistent partition |
| AC-7 | An appliance image is built | The persistent partition's superblock holds every property of the format table, the seed and manifest are present with their modes, and the structural check is clean. A build whose result lacks one property fails and names it | A build on a host whose e2fsprogs cannot make the format fails with the tool and its version named, because a silent other format is worse |
| AC-8 | An appliance built from this work boots | `show host storage` reports `project-quota` as `enforced`, with no operator step | An appliance formatted before this work reports `no-feature` and keeps every other function |
| AC-9 | On the runtime kernel, on a filesystem made by the build's formatter, a folder gets a project id and a 1 MiB limit, and a program writes 2 MiB into it | The write is refused with the quota error once the limit is reached, and the usage read back is within one block of the limit | A second folder with no limit takes the 2 MiB, which proves the refusal is the limit's and not the filesystem's |
| AC-10 | A runtime or installer kernel is built whose emitted config lacks either quota option as built in | The build refuses and names the option | A build with both options succeeds |
| AC-11 | An image is pushed to a grown device and the device reboots into it | The partition bounds, the partition identity, the filesystem size and the stored config are what they were before the push | The root slot and the boot filesystem change, because that is the update |
| AC-12 | The table rewrite is stopped after any one of its writes | A parse of the disk finds a valid table whose partition 4 holds the whole filesystem | Either the image's bounds or the grown bounds are found, because both are correct tables for that disk |
| AC-13 | Growth fails at install after the image is written | The installer logs the failure as an error with its cause, completes the install, and the booted device reports AC-4 (D-3) | The device runs on the image-sized partition, because the written system is complete |
| AC-14 | The own-system guest boots from a docker-host lab image built from this work, and a test in it gives a folder on the guest's own persistent partition a project id and a 1 MiB limit, then writes 2 MiB into it | The superblock of the guest's persistent partition holds every property of the format table, the project-quota state of that partition reads `enforced` with no step taken in the guest, and the write is refused with the quota error once the limit is reached | A second folder with no limit takes the 2 MiB, which proves the refusal is the limit's. The partition keeps the image's size, because this image is never installed and is not grown. A guest booted from an image built before this work is not covered: the image key changes with the format, so the build makes a new one |

The runtime, image store and network children of the container work (`spec-container-1-runtime.md`, `spec-container-2-image-store.md`, `spec-container-3-network.md`) run their guest tests on AC-14: it is what makes the own-system guest a host whose data folder can enforce a size cap.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Installs an appliance from the network on a 256 GiB disk and finds the space | Installer, table rewrite, reread, mount, resize, reboot, `show host storage` | `./le test qemu installer` |
| 2 | Installs from the ISO | The same, with the ISO mount and a poweroff | `./le test qemu installer-iso` |
| 3 | Writes an image to a disk by hand, boots, reads doctor, and grows the storage | Doctor check, `request system storage grow`, partition resize, filesystem resize | `test/appliance/storage-grow-qemu.ci` |
| 4 | Pushes a new image to a grown device | The updater writes root, boot and boot code; the table and partition 4 are untouched | `./le test qemu appliance-ota` |
| 5 | A consumer caps a folder | The quota leaf: assign, limit, write refused, usage read | `TestProjectQuotaRefusesWritePastLimit` |
| 6 | A developer runs a container test in Ze's own-system guest | The lab image build, `formatPerm` with the shared definition, the guest boot, gokrazy's mount, the quota leaf on the guest's own partition | `TestGuestPersistentPartitionEnforcesProjectQuota` in `./le test qemu all` |

## 🧪 TDD Test Plan

The tests that need the kernel run in the QEMU guest on Ze's runtime kernel. The guest's own persistent partition is formatted by the docker-host tooling, which this spec gives the appliance's format (D-4). One guest test, `TestGuestPersistentPartitionEnforcesProjectQuota`, runs on that partition and proves AC-14. The other guest tests need a filesystem they can enlarge, one without the stored option and one without the feature, and none of the three may be the partition the guest lives on. They use a fixture: a small filesystem image the host makes with the build's formatter before the guest boots, listed with the other things a guest needs, and attached in the guest through a loop device. A guest run that finds no fixture, or no persistent partition, fails; a native run skips with the reason.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseValidatesHeaderAndEntries` | `internal/core/gptdisk/gptdisk_test.go` | A table the packer's layout produces parses; a wrong signature, header checksum, entry checksum or reversed entry is refused with its reason | |
| `TestGrowExtendsFourthPartition` | `internal/core/gptdisk/grow_test.go` | The grown table of AC-1: both headers valid, the backup at the end, entry 4 at the last usable sector, the rest unchanged | |
| `TestGrowIsIdempotent` | `internal/core/gptdisk/grow_test.go` | A grown table and an image-sized disk get no write (AC-3) | |
| `TestGrowLeavesValidTableAtEveryWrite` | `internal/core/gptdisk/grow_test.go` | The writer stopped after each write leaves a table that parses and holds the filesystem (AC-12) | |
| `TestGrowClearsStaleBackup` | `internal/core/gptdisk/grow_test.go` | The image's backup copy is zeroed after the new table is in place | |
| `TestGrowRefusesUnsupportedDisk` | `internal/core/gptdisk/grow_test.go` | A 4096-byte sector size, a failed checksum, a fourth entry that is not the highest and a smaller disk are each refused, with no write | |
| `TestGrowBeyondThirtyTwoBitSectors` | `internal/core/gptdisk/grow_test.go` | A 3 TiB sparse disk grows with no wrapped sector number | |
| `TestGrowLivePartitionWhileMounted` | `internal/core/gptdisk/grow_integration_linux_test.go` | In the guest, on a loop disk with the format fixture as partition 4, mounted: Grow writes the table through the whole-disk node, the reread is refused (EBUSY), `ResizeInKernel` is taken, the ext4 grows into the partition, and after an unmount the kernel's own scan reads the grown bounds (A-6, A-16, A-17) | written in phase 3, not run |
| `TestKernelReadsATableAtEveryStopOfTheRewrite` | `internal/core/gptdisk/grow_integration_linux_test.go` | In the guest: after the rewrite is stopped at each sector it writes, the kernel's partition scan finds partition 4 at its old or new bounds (AC-12 for the kernel's reader, A-15). Predicted red at the stops inside the primary write, by the recall of A-15 | written in phase 3, not run |
| `TestReadSuperblockProperties` | `internal/core/ext4fs/superblock_test.go` | The features, inode size, reserved area, journal size, quota inode and stored option are read from a superblock | |
| `TestInjectFormatsPersistentPartition` | `internal/appliance/cmd_build_test.go` | The formatter's argument list names every property, and the quota step runs after the seed writes | |
| `TestVerifyRefusesMissingProperty` | `internal/appliance/diskverify_test.go` | A superblock without one property fails the build and names it (AC-7) | |
| `TestInjectedImagePassesStructuralCheck` | `internal/appliance/cmd_build_test.go` | With host e2fsprogs, a built partition passes the strict structural check and holds the seed (A-7) | |
| `TestKernelFloorsRequireQuota` | `internal/appliance/kernelreq_test.go` | Both floors, both fragments and both manifests hold the two options (AC-10) | |
| `TestInitrdCacheKeyCoversGrowthPackages` | `internal/appliance/cache_test.go` | An edit to a leaf package the installer imports changes the initrd cache variant | |
| `TestRunHTTPGrowsPersistentPartition` | `internal/install/disk/run_test.go` | Order through the seams: write, grow table, reread, mount, seed, resize, unmount, reboot | |
| `TestRunISOGrowsPersistentPartition` | `internal/install/disk/run_test.go` | The ISO order, with the mount and the poweroff | |
| `TestGrowthFailureCompletesInstall` | `internal/install/disk/run_test.go` | A failed growth is logged as an error and the install completes (AC-13) | |
| `TestDetectStoragePersistentObject` | `internal/component/host/storage_linux_test.go` | The object's fields from a fake sysfs and mount table, and its absence with no persistent partition | |
| `TestStorageDoctorReportsStorageSmallerThanDisk` | `internal/component/doctor/checks_storage_grown_test.go` | The finding, its text and its fix; no finding when grown or when no persistent storage is mounted; a warning that says unknown when the state cannot be read | |
| `TestDoctorStorageGrownCheckFunctional` | `internal/component/doctor/checks_storage_grown_test.go` | `ze doctor --json` prints the code on the appliance platform and not on a plain Linux host, so the registered check is what the runner reaches | |
| `TestStorageGrownCheckRunsOnTheApplianceOnly` | `internal/component/doctor/checks_storage_grown_test.go` | The check is registered for the appliance platform and for no other | |
| `TestInstallerSpellsTheStorageDoctorCode` | `internal/component/doctor/checks_storage_grown_test.go` | The code and the fix the installer spells are the ones the check uses | |
| `TestBinarySize` | `internal/component/doctor/checks_storage_grown_test.go` | The unit boundaries of the sizes the finding prints | |
| `TestDetectPersistentStorageReadsTheMountTable` | `internal/component/host/storage_persistent_linux_test.go` | Nothing on the persistent mount point answers no persistent storage; a mounted partition names its device; a missing or malformed mount table is an error and never reads as absent | |
| `TestStorageGrowRefusals` | `internal/plugins/host-cmd/cmd/storage_grow_test.go` | Each refusal of AC-6 with its text | |
| `TestOnlineResizeGrowsMountedFilesystem` | `internal/core/ext4fs/resize_integration_linux_test.go` | In the guest: the fixture is enlarged, resized while mounted, and its size and files are checked (A-1, A-5) | passed in the guest on 7.3 (2026-11-16), 271 ms for 256 MiB to 1 GiB (A-10) |
| `TestProjectQuotaRefusesWritePastLimit` | `internal/core/projquota/quota_integration_linux_test.go` | In the guest: AC-9 with its control folder | passed in the guest on 7.3 (2026-11-16): AC-9 observed, the capped folder refused the write with EDQUOT at the limit and the control folder took it |
| `TestProjectQuotaEnableOnMountedFilesystem` | `internal/core/projquota/quota_integration_linux_test.go` | In the guest: the state reads not enforced on a fixture without the stored option, and enforced after the quota-on call (A-4) | passed in the guest on 7.3 (2026-11-16) |
| `TestProjectQuotaStateWithoutFeature` | `internal/core/projquota/quota_integration_linux_test.go` | In the guest: a plain filesystem reads `no-feature`, never `not-enforced` | failed in the guest on 7.3 (2026-11-16): the state query answered ENOSYS (A-13). Probe and the test fixed (superblock feature bits, A-14); not rerun yet |
| `TestProjectQuotaStoredOptionEnforcesAtMount` | `internal/core/projquota/quota_integration_linux_test.go` | In the guest: the format fixture, mounted with no option string, reads `enforced` with no caller step. An early reading of A-3 on a fixture; phases 6 and 7 still read it on a booted appliance and on the guest's own partition | passed in the guest on 7.3 (2026-11-16) |
| `TestResizeRefusesZeroBlocks` | `internal/core/ext4fs/resize_integration_linux_test.go` | A block count of zero is refused before any kernel call | |
| `TestProjectQuotaRefusesBoundaries` | `internal/core/projquota/quota_integration_linux_test.go` | Project id 0, a limit under 4096 bytes, a limit that is no whole number of 1024-byte quota blocks, and a folder that is not empty are refused before any kernel call | |
| `projquota.Enable` (exercised by `TestProjectQuotaEnableOnMountedFilesystem` and `TestProjectQuotaStateWithoutFeature`) | `internal/core/projquota/quota_linux.go` | The quota-on call for the project type on a mounted filesystem, answering nil only when a probe after it reads `enforced`, and `ErrNoFeature` on a filesystem without accounting. It is the call route E2 needs if A-3 is false | |
| `TestGuestPersistentPartitionEnforcesProjectQuota` | `internal/core/projquota/quota_integration_linux_test.go` | In the guest, on the guest's own persistent partition: every property of the format table is in its superblock, the state reads `enforced`, a write past a 1 MiB limit is refused, and the control folder takes the write (AC-14) | |
| `TestPermFormatDefinitionNamesEveryProperty` | `internal/appliance/permformat/permformat_test.go` | The format step's arguments, the quota step's arguments and the checked property list each cover every row of the format table, so a property added to one is missed in none | |
| `TestFormatPermGivesTheApplianceFormat` | `internal/le/setup/docker/host/perm_test.go` | With host e2fsprogs, a sparse image with gokrazy's four partitions comes back with a partition 4 that passes the Go property check, and the image's allocated size stays near the filesystem's metadata (D-4, A-12) | |
| `TestFormatPermFormatsAnAlreadyFormattedPartition` | `internal/le/setup/docker/host/perm_test.go` | A partition 4 that already holds a plain ext4 filesystem is formatted again and passes the property check: the old format does not survive | |
| `TestFormatPermRefusesAResultWithoutAProperty` | `internal/le/setup/docker/host/perm_test.go` | With a formatter that leaves one property out, the build of the image fails and names the property | |
| `TestImageKeyCoversThePermFormat` | `internal/le/setup/docker/host/build_digest_test.go` | The folder of the shared definition is a digest root, so an edit to the definition gives a new image key (R-10) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Disk size against the table | The table's own extent and up | The image size (no change) | One sector smaller: refused | N/A: 64-bit sectors |
| Sectors gained before a rewrite | 0 and up | 0 (no write) | N/A | N/A |
| Logical sector size | 512 only | 512 | N/A | 4096: refused |
| New filesystem block count | The current count and up | The partition's whole blocks | Below the current count: refused, the filesystem never shrinks | Above the partition: refused |
| Project id | 1 to 4294967295 | 4294967295 | 0: refused, it is the default project | N/A |
| Hard limit in bytes | 4096 and up | 4096 | 0: refused as a limit, it means none | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `storage-grow-qemu` | `test/appliance/storage-grow-qemu.ci` | Wraps `./le test qemu appliance-storage`: an image copied to a larger disk boots, doctor reports it, the command grows it, the state survives a reboot, and the quota state reads enforced (AC-4, AC-5, AC-8) | |
| `host-storage-grow-refused` | `test/plugin/host-storage-grow-refused.ci` | On a host that is not the appliance the command refuses with its reason (AC-6) | |
| `./le test qemu installer` | `internal/le/test/qemu/install_boot.go` | The HTTP install on a target disk larger than the image, with the table and sizes of AC-1 read from the stopped disk and from the booted device | |
| `./le test qemu installer-iso` | `internal/le/test/qemu/install_iso.go` | The ISO install with the same checks (AC-2), and `assertInstallGPT` on the grown expectation | |
| `./le test qemu appliance-ota` | `internal/le/test/qemu/install_ota.go` | The sizes before and after a push are equal on a grown device (AC-11) | |

## Files to Modify
- `internal/install/disk/run.go` - the growth step in `runHTTP` and `runISO`; the reread after the rewrite
- `internal/install/disk/system.go` - the new seams; the resize inside the mount of partition 4
- `internal/install/disk/blockdev_linux.go` - disk size and sector size
- `internal/appliance/cmd_build.go` - `injectZeFS` format arguments, the quota step, `tune2fs` resolution; `findLastPartition` over the shared parse
- `internal/appliance/diskverify.go` - the superblock property check
- `internal/appliance/push_image.go` - `readPushPartitions` over the shared parse
- `internal/appliance/kernelreq.go` - both floors
- `internal/appliance/cache.go` - `initrdSourceDirs`
- `internal/appliance/doctor_checks.go`, `internal/le/setup/tools.go`, `internal/le/setup/probes.go` - `tune2fs` beside the other e2fsprogs tools
- `gokrazy/kernel/kernel.config`, `gokrazy/kernel/kernel.require`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `tools/installer-kernel/kernel.config`, `tools/installer-kernel/kernel.require` - the two quota options
- `internal/component/host/inventory.go`, `internal/component/host/storage_linux.go`, `internal/component/host/storage_other.go` - the persistent object
- `internal/plugins/host-cmd/cmd/register.go` - the command's registration
- `internal/core/diagnostic/codes.go` - `doctor-storage-not-grown`
- `internal/component/doctor/checks_storage.go`, `internal/component/doctor/doctor_checks.go`, `internal/component/doctor/doctor_codes_test.go` - the doctor check beside the free-space check, its registration for the appliance platform, and its code in the registered-codes test
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - the wire method of the grow command
- `internal/le/test/qemu/install_boot.go`, `internal/le/test/qemu/install_iso.go`, `internal/le/test/qemu/install_storage.go`, `internal/le/test/qemu/install_ota.go`, `internal/le/test/qemu/actions.go`, `internal/le/test/qemu/alltests.go` - the larger target disk, the grown checks, the new action, the fixture and the two integration packages
- `internal/le/setup/docker/host/perm.go` - `formatPerm` over the shared format definition, the quota step and the property check; the skip and `hasExt4` deleted; `permPartition` over the shared parse; `findMkfs` widened to the tools the two steps run
- `internal/le/setup/docker/host/build.go` - the folder of the shared definition in `digestRoots`
- `internal/le/setup/docker/host/actions.go` - the `check` action reports `tune2fs` beside `mkfs.ext4`
- `internal/appliance/cmd_build_test.go`, `internal/appliance/push_image_test.go`, `internal/le/test/qemu/install_test.go`, `internal/le/test/qemu/dockerlab_command_test.go` - test images rebuilt as valid GPTs for the shared parse (phase 3); `TestFindLastPartitionRejectsCorruptGPT` now also requires the reversed entry to be the reason
- `internal/test/ext4fixture/fixture_linux.go` - the guest checks, the loop attach and the sparse copy shared with `disk_linux.go` (phase 3)
- `internal/install/disk/grow_linux.go`, `internal/plugins/host-cmd/cmd/storage_grow_linux.go` - their own not-wired sentinels, since `gptdisk.ErrNotImplemented` is gone (phase 3)
- `internal/le/setup/docker/host/perm_test.go`, `internal/le/setup/docker/host/build_digest_test.go` - the lab formatter's tests and the image-key test. The two `hasExt4` assertions of `TestPermPartitionIsReadFromTheGPT` go with the function they call, and `TestFormatPermFormatsAnAlreadyFormattedPartition` proves what replaces it
- `docs/architecture/appliance/docker-host.md`, `docs/architecture/appliance/on-device-installer.md`, `docs/architecture/appliance/iso-installer.md`, `docs/architecture/appliance/installer-initrd.md`, `docs/architecture/appliance/builder.md`, `docs/architecture/appliance/kernel-profiles.md`, `docs/architecture/appliance/ota-push.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/host/inventory.md`, `docs/guide/appliance.md`, `docs/guide/command-reference.md`, `docs/guide/health-checks.md`, `docs/architecture/doctor-and-health-checks.md` - the pages the documentation checklist names

## Files to Create
- `internal/core/gptdisk/gptdisk.go`, `internal/core/gptdisk/grow.go`, `internal/core/gptdisk/gptdisk_test.go`, `internal/core/gptdisk/grow_test.go` - parse, validate, grow
- `internal/core/gptdisk/kernel_linux.go`, `internal/core/gptdisk/kernel_other.go`, `internal/core/gptdisk/grow_integration_linux_test.go` - the partition resize on a disk in use, and its guest tests
- `internal/test/gptfixture/gptfixture.go` - the packer's table layout at any size, for every test that needs a valid GPT
- `internal/test/ext4fixture/disk_linux.go` - a fixture as partition 4 of a loop disk that scans its table
- `internal/core/ext4fs/superblock.go`, `internal/core/ext4fs/resize_linux.go`, `internal/core/ext4fs/resize_other.go`, `internal/core/ext4fs/superblock_test.go`, `internal/core/ext4fs/resize_integration_linux_test.go` - superblock read and online resize
- `internal/core/projquota/quota.go`, `internal/core/projquota/quota_linux.go`, `internal/core/projquota/quota_other.go`, `internal/core/projquota/quota_integration_linux_test.go` - state, enable, assign, limit, usage
- `internal/appliance/permformat/permformat.go`, `internal/appliance/permformat/permformat_test.go` - the one definition of the persistent partition's format: the format step's arguments, the quota step's arguments, the checked property list
- `internal/install/disk/grow_linux.go` - the installer's call into the leaves
- `internal/component/doctor/checks_storage_grown_test.go`, `internal/component/host/storage_persistent_linux_test.go` - the tests of the doctor check and of the host read it calls. The check itself is in `internal/component/doctor/checks_storage.go` (Files to Modify): the host component cannot register it, because `internal/core/diagnostic` imports that component and the import back is a cycle
- `internal/plugins/host-cmd/cmd/storage_grow_linux.go`, `internal/plugins/host-cmd/cmd/storage_grow_other.go`, `internal/plugins/host-cmd/cmd/storage_grow_test.go`, `internal/plugins/host-cmd/yang/ze-host-request-cmd.yang` - the command
- `internal/le/test/qemu/install_storage_grow.go` - the `appliance-storage` action
- `test/appliance/storage-grow-qemu.ci`, `test/plugin/host-storage-grow-refused.ci` - functional tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/host-cmd/yang/ze-host-request-cmd.yang`: one command. No config leaf |
| YANG validation constraints | N-A | The command takes no argument |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `request system storage grow`, through the host command plugin (D-2) |
| CLI grammar (keyword before value) | Yes | Verb `request`, subject `system storage`, action word last; checked by `./le cli grammar` |
| Editor autocomplete | N-A | No config leaf |
| Functional test for new RPC/API | Yes | `test/plugin/host-storage-grow-refused.ci`, `test/appliance/storage-grow-qemu.ci` |
| Pipe completeness | Yes | The command's answer and the `show host storage` object go through the shared renderer, so the JSON pipe applies |
| Env var registration | N-A | No setting. The appliance is recognised by the existing `ze.gokrazy.enabled` |
| Doctor check for runtime dependencies | Yes | `internal/component/doctor/checks_storage.go`, registered in `internal/component/doctor/doctor_checks.go` and reading `host.DetectPersistentStorage`; code in `internal/core/diagnostic/codes.go`, unit and functional test |
| Doctor code per problem class | Yes | `doctor-storage-not-grown` (warning, in the phase before config: it reads the disk, not the config). The quota-absent class belongs to the image store's capability |
| Prometheus counters/metrics | No | Sizes are inventory, read on demand; nothing changes at run time |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The installation and host-inventory feature declarations under the features folder: the installed system uses the whole disk, and the inventory has the persistent object |
| 2 | Config syntax changed? | No | No config leaf is added |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `request system storage grow`, and the new object of `show host storage` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` where it lists host commands; the command reaches every surface through the registry |
| 5 | Plugin added/changed? | No | The host command plugin gains a command; no plugin is added |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md`: what the installer does to the disk, the command, the runtime kernel requirement rows; `docs/guide/health-checks.md`: the doctor code; `docs/architecture/doctor-and-health-checks.md`: the storage check, where it is registered and why |
| 7 | Wire format changed? | No | No protocol wire. The on-disk formats are in the appliance pages |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md`: the `appliance-storage` action, the larger target disk, the guest fixture, the changed layout check, and the own-system guest's persistent partition as a filesystem that enforces project quota |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/on-device-installer.md` (growth, the format, the property check), `docs/architecture/appliance/iso-installer.md` (the "reconstructs no partitions" sentence), `docs/architecture/appliance/installer-initrd.md` (the replacement table), `docs/architecture/appliance/builder.md` (the injection steps), `docs/architecture/appliance/kernel-profiles.md` (the quota options), `docs/architecture/appliance/ota-push.md` (the device's table differs from the image's), `docs/architecture/appliance/docker-host.md` (step 5 of the image build: the partition gets the appliance's format on every build, the sentence that gok formats it where `mkfs.ext4` is on its PATH is removed because no packer in the tree does, the shared definition is a path that shapes the image key, and `check` reports `tune2fs`) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The command inventory pages `./le doc check all` derives; `docs/guide/status.md` if it lists host commands |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/post-release/features/spec-appliance-perm-grows-at-install.md`, run at the start of implementation. The pages in rows 6, 10 and 12 are the ones the design headers of the changed files declare; `docs/architecture/appliance/docker-host.md` in row 12 is the page the headers of `internal/le/setup/docker/host/perm.go`, `build.go` and `actions.go` name. Two more are declared: `docs/architecture/host/inventory.md`, by the host component and host command files, which gains the persistent object and the grow command; and `docs/features/ai-first.md`, by `internal/core/diagnostic/codes.go`, which is unaffected, because it describes how diagnostic codes reach machine readers and one more code changes nothing it says |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/appliance.md` install walk-through and `docs/guide/command-reference.md` `show host storage` sample |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the leaf packages as stubs, the installer seams, the command registration, the doctor check registration, the QEMU action registered with its caller
   - Tests: `TestRunHTTPGrowsPersistentPartition`, `TestRunISOGrowsPersistentPartition`, `test/plugin/host-storage-grow-refused.ci`
   - Files: `internal/install/disk/system.go`, `internal/install/disk/run.go`, the three leaf packages, the host command files, `internal/le/test/qemu/actions.go`
   - Verify: each entry point reaches a stub and the wiring tests fail on the stub
2. **Phase: Kernel facts** -- the guest fixture and the guest tests, before anything depends on a recalled behavior
   - Tests: `TestOnlineResizeGrowsMountedFilesystem`, `TestProjectQuotaRefusesWritePastLimit`, `TestProjectQuotaEnableOnMountedFilesystem`, `TestProjectQuotaStateWithoutFeature`
   - Files: the filesystem and quota leaves, `internal/le/test/qemu/alltests.go`, the two kernel bases and manifests, `internal/appliance/kernelreq.go`
   - Verify: A-1 and A-4 are confirmed or broken in the guest. A broken one stops the work and goes to the main thread
   - Done in this phase beyond the list (2026-11-16): the shared format definition `internal/appliance/permformat` was created here, partly: the format step's arguments and the quota step's arguments only, because the guest fixtures are made "with the build's formatter". Phase 4 still adds the checked property list and `TestPermFormatDefinitionNamesEveryProperty`, and moves `injectZeFS` onto it; phase 7 moves `formatPerm`. The fixtures are made by `internal/test/ext4fixture` from `./le test qemu guest-binaries`, and `test qemu all` names them in `ZE_EXT4_FIXTURES`
   - The two kernel options are `CONFIG_QUOTA` and `CONFIG_QFMT_V2`, requested in `gokrazy/kernel/kernel.config` (the base the runtime profile layers first) and `tools/installer-kernel/kernel.config`, required in both `kernel.require` manifests and in both compiled floors. Evidence: `fs/quota/Kconfig` (read in `/usr/src/linux-headers-6.8.0-149`) declares `config QUOTA` as a bool that does `select QUOTACTL`, and `config QFMT_V2` as a tristate that `depends on QUOTA` and does `select QUOTA_TREE`, so `QUOTACTL` follows from `QUOTA` and `QFMT_V2` alone can be demoted to a module, which no kernel of Ze loads. That ext4's hidden quota inodes use the vfsv1 format `QFMT_V2` implements is recalled, not read. Both cached 7.3 runtime kernel configs already resolve `QUOTA=y`, `QFMT_V2=y`, `QUOTACTL=y` and `BLK_DEV_LOOP=y`
3. **Phase: Partition table** -- parse, validate, grow; the four existing readers move to the shared parse
   - Tests: the partition-table unit tests; the existing push, build and lab-image partition tests stay green
   - Files: the partition-table leaf, `internal/appliance/push_image.go`, `internal/appliance/cmd_build.go`, `internal/le/test/qemu/install_iso.go`, `internal/le/setup/docker/host/perm.go`
   - Verify: tests fail, implement, tests pass
   - Done in this phase (2026-11-16, nothing run yet): `gptdisk.Parse` (protective 0xEE record, both checksums, the header's own sector, every bound before a read, entries inside the usable sectors and not overlapping), `gptdisk.Grow` (backup, sync, primary, sync, stale backup zeroed except where the new backup overlaps it; no write on a described disk; the partition lowered to whole 4096-byte blocks, never shrunk), and, ahead of phase 6, `gptdisk.ResizeInKernel` (`BLKPG_RESIZE_PARTITION`) with its guest test, so A-6 is settled before the command depends on it. The test fixture `internal/test/gptfixture` writes the packer's layout at any size; `internal/test/ext4fixture` gained `CopyInto`, `AttachDisk` (partition scanning) and `KernelPartition`. The four readers parse through the leaf; their own parsing is deleted, and their test images that were not valid GPTs (entries with no header, headers with no checksum, a missing protective record) were rebuilt as valid ones. `gptdisk.ErrNotImplemented` is deleted; the installer and the grow command, still stubs until phases 5 and 6, wrap sentinels of their own
4. **Phase: Build format** -- the shared format definition, the named properties, the quota step, the property check, `tune2fs`
   - Tests: `TestPermFormatDefinitionNamesEveryProperty`, `TestInjectFormatsPersistentPartition`, `TestVerifyRefusesMissingProperty`, `TestInjectedImagePassesStructuralCheck`, `TestKernelFloorsRequireQuota`
   - Files: `internal/appliance/permformat/permformat.go`, `internal/appliance/cmd_build.go`, `internal/appliance/diskverify.go`, `internal/appliance/doctor_checks.go`, `internal/le/setup/tools.go`, `internal/le/setup/probes.go`
   - Verify: A-7 and A-8 are confirmed or broken on the build host
5. **Phase: Installer growth** -- both modes, the cache key
   - Tests: `TestGrowthFailureCompletesInstall`, `TestInitrdCacheKeyCoversGrowthPackages`, `./le test qemu installer`, `./le test qemu installer-iso`
   - Files: `internal/install/disk/run.go`, `internal/install/disk/grow_linux.go`, `internal/install/disk/blockdev_linux.go`, `internal/appliance/cache.go`, `internal/le/test/qemu/install_boot.go`
   - Verify: AC-1, AC-2, AC-3, AC-13
6. **Phase: Device surface** -- the report, the doctor check, the command, the appliance action
   - Tests: `TestDetectStoragePersistentObject`, `TestStorageDoctorReportsStorageSmallerThanDisk`, `TestStorageGrowRefusals`, `test/appliance/storage-grow-qemu.ci`, `./le test qemu appliance-ota`
   - Files: the host component and host command files, `internal/core/diagnostic/codes.go`, `internal/le/test/qemu/install_storage_grow.go`, `internal/le/test/qemu/install_ota.go`
   - Verify: AC-4, AC-5, AC-6, AC-8, AC-11, and A-3 confirmed or the design moved to E2
7. **Phase: Own-system guest format** -- the lab formatter over the shared definition, the deleted skip, the image key, then a new lab image and the guest test on its own partition. It follows phase 4, which makes the definition, and may run before phases 5 and 6
   - Tests: `TestFormatPermGivesTheApplianceFormat`, `TestFormatPermFormatsAnAlreadyFormattedPartition`, `TestFormatPermRefusesAResultWithoutAProperty`, `TestImageKeyCoversThePermFormat`, `TestGuestPersistentPartitionEnforcesProjectQuota` in `./le test qemu all`
   - Files: `internal/le/setup/docker/host/perm.go`, `internal/le/setup/docker/host/build.go`, `internal/le/setup/docker/host/actions.go`, `internal/core/projquota/quota_integration_linux_test.go`
   - Verify: AC-14, and A-12 confirmed or broken on a Linux and a Mac build host. A-3 is first seen here or in phase 6, whichever runs first
8. **Phase: Pages** -- every page of the documentation checklist, in the phase that makes it wrong where that is earlier

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The write order of the table is backup first; every sector number is 64-bit; the filesystem block count comes from the partition entry, never from the disk size; a failed checksum stops before any write |
| Naming | JSON keys of the persistent object are kebab-case; the doctor code is `doctor-storage-not-grown`; the leaf packages hold no gokrazy name |
| Data flow | The installer and the daemon call the same leaf functions; no second copy of the table arithmetic exists; the installer imports no component |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/go-standards.md` | No external program is run from the installer package, the leaf packages or the host command. A state that cannot be read answers unknown with its reason, never grown and never enforced |
| Rule: `ai/rules/platform-linux.md` | Every syscall is in a Linux-only file behind a seam; the guest tests carry the integration and Linux tags; the two packages are in `integrationPackages`; the new action has a caller |
| Rule: `ai/rules/no-layering.md` | The own parsing of `findLastPartition`, `readPushPartitions` and `permPartition` is deleted when the shared parse replaces it. `hasExt4` and the skip in `formatPerm` are deleted, not kept beside the new format |
| One definition of the format | No property of the format table is written in `injectZeFS` or in `formatPerm`: a search for a feature name, the inode size or the journal size finds each in the shared definition only |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| An installed device uses its disk | `./le test qemu installer`, `./le test qemu installer-iso` |
| A device grows on the command | `./le test qemu appliance-storage` |
| The quota is enforced by the kernel | `./le test qemu all` reaching `TestProjectQuotaRefusesWritePastLimit`, and the fixture check that refuses a skipped run |
| The build's format is fixed and checked | `./le test unit pkg path ./internal/appliance/` |
| The own-system guest has the appliance's format and enforces a size cap | `./le test unit pkg path ./internal/le/setup/docker/host/`, and `./le test qemu all` reaching `TestGuestPersistentPartitionEnforcesProjectQuota` on a lab image built from this work |
| The installer runs no external program | A search for the exec call in the installer package and the three leaf packages answers nothing outside tests |
| Both kernels require quota support | A search for the two option names in the three manifests finds each in each |
| The pages are right | `./le doc check all` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The table read from a disk is untrusted: every offset, count and size is bounded before it is used, with 64-bit arithmetic, as `readPushPartitions` does today |
| Destructive write to the wrong disk | The installer grows only the disk it just wrote. The command grows only the disk that holds the mounted persistent partition, found from the mount table, never from an argument. The lab formatter, which now formats without looking, is given only the partial file its own build has just written, never an image in the cache or a device |
| Authorization | `request system storage grow` is a write command and follows the command authorization of the surface it arrives on. An observer login is refused |
| Resource exhaustion | The command is not repeatable into harm: a second run writes nothing. The quota leaf refuses a zero limit and project id 0 |
| Error leakage | Messages name device nodes and sizes only |
| Fail-open state | A quota state that cannot be read is `unknown`, and a consumer treats it as not enforced |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A recalled kernel behavior is false in the guest (A-1, A-3, A-4, A-6) | STOP at phase 2 or 6. Report the observed behavior. The fallback each assumption names is a design change the main thread presents |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The installer already mounts partition 4 read-write in HTTP mode, so the installer kernel already carries a writable ext4. Growth adds one ioctl to an existing mount, and one mount to ISO mode.
- The image's table is valid on a larger disk only because the kernel tolerates a backup header that is not at the end. Extending the table makes the installed disk a stricter GPT than it is today.
- The quota features bind the kernel that mounts the filesystem, not only the consumer. The installer kernel is a reader of the superblock.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Grow the filesystem with the kernel's online resize, at install | A fresh format at install by a Go ext4 writer; growth only at first boot; an image per disk size | No ext4 writer exists in the tree and one was already rejected; the resize is one ioctl on a mount the installer already makes; the owner ruled growth at install |
| One growth code path for the installer and the device | Separate code for each | The table arithmetic and the write order are the dangerous part and are written and tested once. Only the way the kernel is told differs: a reread with nothing mounted, a partition resize on a live disk |
| Extend the table, keep every identity | Write a whole new table as the packer does | gokrazy finds its disk and root by GUID, and the update proof requires the persistent partition's identity to survive |
| Backup copy first | Primary first | With primary first, a stop between the entries and the header leaves a primary that fails its checksum and a backup pointer to the image's stale copy |
| The build names every filesystem property | Keep the host's defaults and add two features | The filesystem keeps its build-time properties for life after an online resize, so a host-dependent format becomes a per-device difference nobody can see |
| The quota feature is added after the seed, by `tune2fs` | Ask for it at format and trust `debugfs`; repair with a fixing `e2fsck` pass | A fixing pass would also repair a real structural fault and hide it from the strict check that exists to catch one |
| Enforcement by a mount option stored in the superblock (E1) | `ze` turns it on at start (E2); a diff to gokrazy's mount (E3) | No run-time step, no ordering against consumers, and the same answer for every process. E2 is the fallback when A-3 is false |
| Project quota type only | User and group types as well | Nothing uses them, and a changed owner in the seed step would then also need its usage recomputed |
| Not automatic at boot (D-1, recommended) | Grow at every boot when the disk is larger | The owner ruled install. A boot path that never writes a partition table cannot damage one |
| The docker-host lab image takes the appliance's format, from one shared definition read by two formatters (D-4) | The lab image keeps its plain format and the guest tests use only a fixture; one formatter function for both images | The lab image is the own-system guest the container tests run in, and a guest that cannot enforce a size cap refuses a container config. The two formatters stay separate because the appliance writes a seed between its two steps and works on an in-memory copy of a small partition, and the lab partition is about 63 GiB with no seed. What they share is the format, so that is what is declared once |
| The storage doctor check is registered by the doctor component and reads the state through `host.DetectPersistentStorage` | Register it from the host component, as the first file list of this spec said; register it from the host command plugin | `internal/core/diagnostic` imports the host component for the platform type (`doctor_registry.go`, the `Platform` field of `DoctorCheckContext`), so the host component cannot import the check registry. The doctor component is the documented home of a check with no owner that can register it, and already holds the free-space check. The host command plugin can run in another process, and a config-phase check must answer in whichever process answers doctor |
| The lab formatter always formats | Reformat only when the superblock lacks a required property | Its one caller passes a file `gok` has just written, and no packer in the tree formats partition 4, so the skip guards a case that does not occur. One path cannot keep an old format |

## Known Limitations
- A disk above about 8 TiB is not covered by a test. Growth beyond the reserved area depends on A-5, and proving it needs a filesystem larger than a test guest can initialise in reasonable time. Inside the bound the design does not depend on A-5.
- A disk with 4096-byte logical sectors is refused by the growth step. The image's table is written in 512-byte units, so this is the image's existing boundary, reported instead of guessed.
- A device formatted before this work keeps a filesystem without the quota feature. It is reinstalled to gain it: Ze is pre-release and no migration is designed.
- The journal stays at its build-time size on every device.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and the QEMU actions this spec names. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol peer)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
