# Spec: ze-image-docker-host

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/6 |
| Handoff | - |
| Updated | 2026-10-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Opened by `plan/handover/11-ze-image-docker-host.md`. One image built from Ze's kernel and Ze's system, with dockerd, boots as a separate `ze` Colima profile on the Mac and replaces Alpine in the QEMU `docker-lab` guest. `./le setup` builds the image and wires the profile, and leaves the `default` Colima profile untouched. Owner decisions: M1 and M10 in `ai/META-DECISIONS.md`.

-> Decision (owner, 2026-10-21, design gate, D-13): OpenSSH and sudo are compiled in a pinned Alpine container (image pinned by digest) whose only role is a build tool. Alpine is chosen for musl: a fully static musl binary reads `/etc/passwd` and `/etc/group` directly, while a static glibc binary still loads NSS libraries at runtime for `getpwnam` and `initgroups`, which Ze's image does not carry, so sshd and sudo would fail at the uid-501 login Lima performs. Nothing from that container runs in any VM, guest or Docker host except the two static binaries it outputs; it is not a lab guest and M1/M10 do not apply to it. The Docker host itself stays Ze's kernel and Ze's system, with no Alpine userland, `apk` or Alpine package in the image. Rejected: Debian with glibc (NSS failure above); Debian with `musl-tools` (every OpenSSH dependency would need its own pinned musl build, for no gain).

-> Decision (owner, 2026-10-21, research gate): Colima's guest contract is met by shipping static third-party binaries in Ze's image (busybox as shell and coreutils, OpenSSH, sudo, docker CLI) as per-arch extra files, with the Lima boot steps and a `systemctl` shim in Go. Rejected: Ze's own SSH server grown into a guest shell; Colima `--runtime none`.

### Why

M10 makes every lab Docker daemon run on Ze's kernel and Ze's system. Today the Mac has no such daemon: Colima's `default` profile runs Ubuntu 6.8 (refused by `./le setup docker-kernel check` on `ipsec-mobike`, `l2tp`, `l2tp-ppp`, `mpls`, `mpls-transit-mtu`), and the QEMU `docker-lab` guest runs Ze's kernel 7.2.9 under Alpine's userland and dockerd (M1 rejects it). `plan/spec-lab-docker-cleanup-on-macos.md` cannot close its AC-2, AC-6 and AC-8 until such a daemon exists.

### Goals

| ID | Goal |
|----|------|
| G-1 | One image, built from `gokrazy/kernel` runtime kernel and a gokrazy instance, runs dockerd and meets the Colima 0.10.3 / Lima 2.2.1 guest contract |
| G-2 | `./le setup` builds that image for the host architecture and runs it as the Colima profile `ze`, never touching `default` |
| G-3 | Docker labs on darwin target the `ze` profile's socket and refuse, naming the setup command, when it is absent |
| G-4 | `./le test qemu docker-lab` boots the same image (arm64 HVF, amd64 KVM), drives its dockerd from the host le, and no longer uses Alpine, apk, the in-guest Go toolchain or the guest le |
| G-5 | The waiting spec's AC-2, AC-6 and AC-8 pass on the `ze` profile (AC-13 to AC-15 here) |

## Required Reading

### Architecture Docs
- [ ] `ai/META-DECISIONS.md` M1, M10 - owner scope
  → Constraint: a guest that is a Docker host runs Ze's kernel and Ze's system; Colima may only boot Ze's image; no Alpine or Ubuntu fallback, no test-side workaround for an Alpine-only problem.
- [ ] `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` D-5, D-6, AC-8, AC-10, AC-14
  → Decision: D-5 holds: one kernel for every Docker host, the `runtime` profile; every option this spec needs goes into `gokrazy/kernel/runtime.config` and `runtime.require` with a reason, no `docker-host` kernel profile.
  → Constraint: D-6's "colima is never the lab host" (and AC-8's wording) is superseded for the Mac by M10: Colima booting Ze's image is a lab host. D-6's QEMU guest stays for the nightly and as the Mac's second route. AC-8 and AC-10 name an Alpine guest; after this spec their guest is Ze's image (pointer appended at implementation, see Files to Modify).
- [ ] `plan/spec-lab-docker-cleanup-on-macos.md` AC-2, AC-6, AC-8, R-6
  → Decision: copied here as AC-13, AC-14, AC-15, run on the `ze` profile. Their labels contract (AC-2: `ze.lab.machine-id` is the Mac's IOPlatformUUID, `ze.lab.os` is `darwin`) holds because le runs on the Mac and only dockerd runs in the VM.
  → Constraint: R-6: the VPP image is `linux/amd64` only, so AC-15 on an arm64 Mac needs amd64 emulation in the `ze` profile (Rosetta through Colima `--vz-rosetta`, which needs `CONFIG_BINFMT_MISC` and `CONFIG_VIRTIO_FS`), plus `/dev/vhost-net`.
- [ ] `plan/spec-lab-kernel-features-in-setup.md` D-1, AC-4
  → Constraint: `CONFIG_VHOST_NET` is that spec's AC-4. This spec needs it for AC-15; if that spec has not landed it when this one implements, this spec adds it with the same reason line and that spec's AC-4 is then met by this commit (pointer appended to that spec).
- [ ] `docs/architecture/testing/qemu-integration.md` (docker-lab section ~430, guest section ~801, kernel pairing ~820)
  → Constraint: the page describes the docker-lab guest as Alpine with OpenRC dockerd and guest le; every such paragraph is rewritten in the phase that changes `internal/le/test/qemu/dockerlab.go`. The Alpine sections about `./le test qemu run`, mobike and stress stay (those guests are not Docker hosts).
- [ ] `docs/guide/developer-setup.md` "Docker labs" (~231-250)
  → Constraint: line ~238 says labs run "never in colima"; it becomes the `ze` profile route with `./le setup docker-host start`, and the QEMU route is named second.
- [ ] `docs/architecture/testing/interop.md` (~637-673, ~811)
  → Constraint: the AppArmor-on-Colima refusal text and the docker-lab route text describe the `default` profile and the in-guest le; both change with AC-8 and AC-9.
- [ ] `ai/rules/platform-linux.md`
  → Constraint: host binaries are never cross-compiled for the appliance; the docker-host guest binary is a target binary (built for the image's GOARCH by gok), `le` stays a host binary.
- [ ] `ai/rules/repo-maintenance.md` (runtime dependency, discovery)
  → Constraint: new external binaries (colima, limactl for AC-1 only, qemu) and new cache paths are runtime dependencies of le; they are reported by `./le setup check`, not by `ze doctor` (development host, not the product).

**Key insights:**
- Colima cannot direct-boot a kernel: vz boots EFI from the disk (`\EFI\BOOT\BOOTAA64.EFI`), so the image's own boot partition must work under Apple's EFI (lima `pkg/driver/vz/vm_darwin.go` bootLoader, per the research digest).
- A fresh gokrazy image is GPT plus a hybrid MBR (partition 1 FAT, partition 2 type 0xEE) and carries systemd-boot as `BOOTX64.EFI` and `BOOTAA64.EFI` (`vendor/github.com/gokrazy/tools/internal/packer/packerprepare.go` sets `UseGPT` for a new installation; `vendor/github.com/gokrazy/tools/packer/packer.go` `writePartitionTable`, `Partition`; `write.go` lines 389-402). Whether Apple's EFI accepts the hybrid MBR is A-1.
- Lima's `boot.sh` writes `/run/lima-boot-done` on its last line whatever its scripts returned; a failing boot script only sets `CODE=1` (`cidata.TEMPLATE.d/boot.sh`). The fatal points are the guest agent within one minute, `boot-done`, and Colima's own provisioning commands.
- Several Lima scripts take a systemd branch when `command -v systemctl` succeeds (`00-check-rtc-and-wait-ntp.sh`, `20-rootless-base.sh`, `40-install-containerd.sh`) and `boot.sh` waits forever for `/run/user/<uid>/systemd/private` before `provision.user` scripts unless `/sbin/openrc-run` exists. The shim's presence changes which branches run (R-2).
- `25-guestagent-base.sh` installs the agent to `${LIMA_CIDATA_GUEST_INSTALL_PREFIX}/bin` (default `/usr/local`, `lima/pkg/limayaml/defaults.go` defaultGuestInstallPrefix) and then runs `lima-guestagent install-systemd`, which writes `/etc/systemd/system/lima-guestagent.service` and runs `systemctl --system daemon-reload`, `enable`, `try-restart`, `start` (`cmd/lima-guestagent/install_systemd_linux.go`). The unit's `ExecStart` carries `%t/%N`.
- Colima writes `/etc/systemd/system/docker.service.d/docker.conf` with `ExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock --host-gateway-ip=<ip>` (colima `environment/container/docker/daemon.go`), so socket activation is assumed; the supervisor does not do `fd://` (Decision D-7).
- Docker's nftables firewall backend exists from Docker 29.0.0 and is experimental; it does not enable IP forwarding itself (docs.docker.com/engine/network/firewall-nftables, fetched 2026-10-21).
- The busybox Ze already ships (`cmd/ze-serial-shell/_gokrazy/extrafiles_{amd64,arm64}.tar`, 1.2 MB each, deployed as `/usr/local/bin/ze-recovery-shell`) is BusyBox v1.34.1 and carries `sh ash awk stat sha256sum install seq id ip udhcpc chpasswd adduser addgroup mke2fs fdisk od paste sed timeout tee date mount df nsenter`; it lacks `sfdisk`, `findmnt`, `getent` and `sudo` (applet names read from the arm64 binary's strings, 2026-10-21; a name match, not an execution).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/ze/config.json` - the only gokrazy instance: packages `cmd/ze-serial-shell` and `cmd/ze`; GokrazyPackages `randomd`, `heartbeat` (so no gokrazy `dhcp`); a fixed `/etc/machine-id` `8c41122c...` baked into every image through randomd's ExtraFileContents; `SerialConsole ttyS0,115200`; `KernelPackage ze.invalid/kernel`.
- [ ] `internal/appliance/instance/prepare.go` - `Prepare(srcParent, Options)` copies `<srcParent>/ze` (the `Name` constant is `ze`), its `builddir`, patches `KernelExtraArgs`, assembles the kernel package from `Options.KernelTree`; refuses without a kernel tree.
  → Constraint: the instance name is a constant; a second instance needs it as an `Options` field, every caller passing it (no default, no fallback).
- [ ] `internal/appliance/cmd_build.go` - `buildOne(name)` builds a named appliance from an `applianceConfig` (identity, credentials, ssh, web, tls, device, image, qemu) and injects Ze's config database (`injectZeFS`); `runGokBuild` runs gok in-process.
  → Constraint: the appliance config is shaped for a Ze NOS appliance; the docker-host image reuses `instance.Prepare` and `runGokBuild`, not `buildOne`'s config and database injection.
- [ ] `internal/le/build/gosum/gokrazygosum.go` - `builddirPrefix = "gokrazy/ze/builddir/"`.
  → Constraint: a second instance's `builddir` go.sum files are outside this check unless the prefix becomes every `gokrazy/*/builddir/`.
- [ ] `gokrazy/kernel/runtime.config`, `runtime.require`, `kernel.config` and the arm64 cache entry `~/.cache/ze/runtime-kernel/7.2.9-runtime-arm64-runtime-docker-fc13b746-34cba457/config` (2026-10-17) - present: `INET_DIAG`, `INET_TCP_DIAG` (default with INET_DIAG), `EFI`, `EFI_STUB`, `EFI_PARTITION`, `MSDOS_PARTITION`, `VIRTIO_CONSOLE`, `VIRTIO_BLK`, `VIRTIO_NET`, `VIRTIO_MMIO`, `ISO9660_FS`, `FUSE_FS`, `EXT4_FS`, `VFAT_FS`, `OVERLAY_FS`, `NF_TABLES` family, `IP_NF_IPTABLES_LEGACY`, the D-5 Docker set. Absent: `VSOCKETS`, `VIRTIO_VSOCKETS`, `INET_UDP_DIAG`, `VIRTIO_FS`, `VHOST_NET`, `BINFMT_MISC`, `CFS_BANDWIDTH`. `VIRTIO_CONSOLE` comes from arm64 `defconfig`, not from `runtime.config`.
- [ ] gokrazy init `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/mount.go` - tmpfs `/etc` with `size=1M`, `/etc/resolv.conf` symlinked to `/proc/net/pnp`, `/etc/hosts` written, tmpfs `/tmp`, `/dev/shm`, `/run`; ext4 `/perm`; `/perm/var` with `/perm/var/run` symlinked to `/run`; cgroup2 on `/sys/fs/cgroup`. `machineid.go` `MachineID` reads `/perm/machine-id`, then `/etc/machine-id`, then `/etc/hostname`.
- [ ] `internal/le/test/qemu/dockerlab.go` - `runDockerLab`: resolves the runtime kernel cache entry (`dockerLabKernel`), builds the guest le (`buildGuestLe`) and the linux ze (`BuildLinuxZe`), runs `NewRun(...).Execute` with `dockerLabPackages` and `dockerLabCommand`: exports `ZE_DOCKER_LAB_GUEST=1`, `rc-service docker start`, waits for `docker info`, runs the guest `le setup docker-kernel check ze <guest ze>`, then the lab words under `dockerLabLeftoverTrap` (lists `ze.lab` containers, networks, `ze-interop-retained` images).
- [ ] `internal/le/test/qemu/run.go` - `Run.Plan`, `qemuArgs` (direct `-kernel` with Alpine initramfs and ISO), `setupCommand` (apk repositories and packages, `modprobe` ppp/l2tp/nft_chain_nat/nf_conntrack lines, `nf_conntrack_acct`, the `ztrack` nft table, 9p `/workspace` and scratch shares, the Go toolchain download into `/usr/local/go`, the patched GOROOT, a tmpfs on `/workspace/tmp/evidence`), `acceleration`, `arm64BIOS`, `virtfsArgs`.
  → Constraint: `NewRun` is also called by `actions.go` (`./le test qemu run`), `stress.go` and `mobike.go`; those guests are not Docker hosts and keep `Run` unchanged here.
- [ ] `internal/le/test/qemu/guestle.go` - `buildGuestLe`, `GuestLeRel`: cross-builds le for the guest. Used only by the docker-lab path and stress.
- [ ] `internal/le/interoplab/kernelcheck.go` - `DockerLabGuestVariable` (`ZE_DOCKER_LAB_GUEST`), `inDockerLabGuest`, and the darwin route text "On darwin the labs run inside the Ze-kernel QEMU guest ... ./le test qemu docker-lab lab ..."; the probe container mounts the host path of the linux ze (`-v zePath:...:ro`, line ~247), so the daemon host must see the client's path.
- [ ] `internal/le/dockerhost/dockerhost.go` - `Environment`: on darwin, with DOCKER_HOST empty and `/var/run/docker.sock` absent, selects `~/.colima/default/docker.sock`; callers `dockerhost.Inherited` in `interoplab/docker.go`, `setup/dockerkernel/dockerkernel_apparmor_colima.go`, `site/terminaldemo/render.go`, and `Environment` in `test/qemu/install_build.go`.
- [ ] `internal/le/setup/dockerkernel/register.go` - `leroot.Register(dockerKernelArea, ...)`, `RegisterActions`, `RegisterShape`; actions `check` (keyword `ze`), `install` (`builder`, `confirm`), `apparmor` (`confirm`).
- [ ] Colima `environment/vm/lima/yaml.go`, `disk.go`, `disk.sh`, `environment/container/docker/{docker,containerd,daemon}.go`, `cmd/start.go`, `cmd/delete.go`; Lima `pkg/cidata/cidata.TEMPLATE.d/` (`boot.sh`, `boot.Linux/*`, `boot.essential.Linux/*`), `cmd/lima-guestagent/install_systemd_linux.go`; alpine-lima `lima-init.sh` (sources in the session scratch, Colima 0.10.3, Lima 2.2.1).
  → Constraint: Colima adds a `dependency` provision script `groupadd -f docker && usermod -aG docker <user>`, `system` scripts `sysctl -w fs.inotify.max_user_watches=1048576`, the `/etc/hosts` hostname line and `hostnamectl set-hostname <profile>`; a runtime data disk script (`disk.sh`: `df`, `findmnt`, `sfdisk`, `mkfs.ext4`, `e2label`, `mount`) and `mount --bind` lines for the data dirs. `colima delete` takes `--force`; `colima start` takes `--disk-image`, `--force-disk-image`, `--vm-type`, `--mount-type`, `--vz-rosetta`, `--arch`, `--runtime`.

**Behavior to preserve:**
- The `ze` appliance instance, its image and `./le build gokrazy` output are unchanged except that `instance.Prepare` takes the instance name.
- `./le setup docker-kernel check` stays the gate every Docker lab runs before any container; its rows and refusal format are unchanged, only the darwin route text changes.
- `./le test qemu run`, `stress`, `mobike`, `all-tests` keep the Alpine `Run` (out of scope here; journal row, see Known Limitations).
- The `docker-lab` verb surface: `lab "<le words>"`, `env "<NAME=value ...>"`, `timeout`; the nightly workflow lines in `.github/workflows/evidence-nightly.yml` keep their text.
- The owner's 2026-10-21 "yes forward" decision: a Ctrl-C reaches the le that runs the lab, whose own teardown runs before the VM stops.

**Behavior to change:**
- darwin's default Docker endpoint for labs becomes `~/.colima/ze/docker.sock`; `~/.colima/default/docker.sock` is no longer selected (no fallback, `ai/rules/no-layering.md`).
- `docker-lab` boots Ze's docker-host image and runs the lab from the host le; the Alpine ISO, initramfs, apk, the guest le, the in-guest Go toolchain and `ZE_DOCKER_LAB_GUEST` leave the docker-lab path.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le setup docker-host build | start | check | stop` (new area, host le).
- `./le test qemu docker-lab [lab "<words>"] [env "..."] [timeout <d>]` (existing verb).
- Any Docker lab on darwin (`./le test integration ...`, `./le perf run ...`, `./le test deployment ...`) through `dockerhost.Inherited`.

### Transformation Path
1. Inputs: the runtime kernel cache entry for the arch (`testdeployment.RuntimeKernelCacheDir`), the pinned binary set (`gokrazy/docker-host/binaries.json`: name, version, URL, sha256, per arch), the `gokrazy/docker-host/` instance (config.json, builddir), `cmd/ze-docker-host` source.
2. Fetch and build: Docker static tarballs downloaded and checked against the pin; OpenSSH, sudo (and iptables if A-5 fails) built from pinned source tarballs as static musl binaries in the pinned Alpine builder container (build tool only, D-13); busybox from the committed ze-serial-shell tar. Results cached under `~/.cache/ze/docker-host/<arch>/<input-key>/`.
3. Image: `instance.Prepare(gokrazy, {Instance: "docker-host", KernelTree, Arch})`, extra files assembled per arch, `runGokBuild` writes a raw image at a fixed sparse size to the cache.
4. Colima (darwin): `colima start --profile ze --vm-type vz --arch <host> --runtime docker --disk-image <img> --force-disk-image --mount-type virtiofs --vz-rosetta` (Rosetta only on arm64); when the recorded image key differs, `colima delete --profile ze --force` first.
5. Guest boot: Apple EFI or QEMU firmware, systemd-boot, Ze's kernel, gokrazy init, `ze-docker-host` (cidata, users, sshd, supervisor, Lima `boot.sh`), Lima guest agent, then Colima's provisioning over SSH (containerd restart, daemon.json, `systemctl start docker.service`), socket forward to `~/.colima/ze/docker.sock`.
6. Labs: host le with `DOCKER_HOST=unix://~/.colima/ze/docker.sock` (darwin default) or the QEMU forwarded socket; `setup docker-kernel check` first.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| host le ↔ Colima/Lima | `colima` CLI argv; Colima config under `~/.colima/ze/` | No (AC-2, AC-7) |
| Lima host agent ↔ guest | cidata ISO (`lima.env`, `param.env`, `user-data`, `meta-data`, `boot.sh`, `lima-guestagent`), SSH on 22, vsock guest agent | No (AC-1) |
| QEMU le ↔ guest | cidata disk written by le with the same file set, SSH via `hostfwd`, unix-socket forward `ssh -L <local>:/var/run/docker.sock`, 9p shares mounted at the host's absolute paths | No (AC-9) |
| guest scripts ↔ supervisor | `systemctl` shim: argv, exit status, a control socket under `/run/ze-docker-host/` | No (AC-11) |
| host le ↔ dockerd | Docker API over the forwarded unix socket | No (AC-3, AC-9) |

### Integration Points
- `instance.Prepare` - gains an instance-name field; `ze` callers pass `instance.Name`.
- `runGokBuild` / gok in-process - reused for the second instance without `injectZeFS`.
- `testdeployment.RuntimeKernelCacheDir`, `dockerLabKernel` - the kernel tree for both the image and its key.
- `Run.acceleration`, `Run.arm64BIOS`, `vmrecord` (QEMU reaping) - reused by the new docker-lab boot.
- `interoplab` kernel check (`dockerKernelCheck`) - unchanged gate; route text changes.
- `dockerhost.Environment` - darwin candidate changes to the `ze` profile.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | labs still go through `dockerhost.Inherited` and the kernel check; the image goes through `instance.Prepare` and gok |
| No unintended coupling (components stay isolated) | Yes (design) | the guest program depends on nothing in `internal/component`; le's setup area depends on `internal/appliance/instance` and `internal/le/dockerhost` only |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | reuses Prepare, gok, the kernel cache, `acceleration`, `arm64BIOS`, `vmrecord`; runs Lima's own `boot.sh` instead of re-implementing it |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, no wire path |
| Registration over hardcoding, outbound | Yes (design) | the `docker-host` setup area registers through `leroot.Register`/`RegisterActions` in its own `register.go`; no edit to `internal/le/setup/actions.go` |
| Registration over hardcoding, inbound | To verify at implementation | lists searched for names this spec introduces: `instance.Name` users, `builddirPrefix` in `gosum` (becomes every instance's builddir), `dockerhost.Environment` candidate, the kernelcheck darwin route string, `.github/workflows/evidence-nightly.yml` docker-lab lines, `ai/INDEX.md` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Apple's vz EFI boots a gokrazy image: GPT with a hybrid MBR, FAT partition 1 holding `\EFI\BOOT\BOOTAA64.EFI` (systemd-boot) | gokrazy `packer.go` writePartitionTable, `write.go` 389-402; lima vz bootLoader is EFI from disk | the image never reaches the kernel under Colima; the builder writes a protective-MBR-only GPT (gokrazy `mbrOnlyWithoutGpt` is the inverse knob; a GPT-only option needs a packer change) | AC-1 first step: `limactl start` with a throwaway `LIMA_HOME` under the session scratch, console log read for systemd-boot and the kernel banner | confirmed 2026-10-21: `limactl start` (vz, Lima 2.2.1) booted the gokrazy image from its own EFI partition; `serialv.log` shows kernel `7.2.9-ze`, `VFS: Mounted root (squashfs filesystem) readonly`, `Run /gokrazy/init`, and the image's `cmdline.txt` (`root=PARTUUID=60c24cc1-.../PARTNROFF=1`). systemd-boot prints to the EFI console, not hvc0, so its lines cannot appear in the serial log. Lima growing the raw disk to 4 GiB leaves the backup GPT header mid-disk (kernel warns, boots) |
| A-2 | The console is `hvc0` under vz and `ttyS0` under QEMU amd64; the instance's `SerialConsole`/`KernelExtraArgs` list both consoles so either boot logs | arm64 cache config has `VIRTIO_CONSOLE=y`; gokrazy `SerialConsole` field | no boot log under vz, failures are blind; `VIRTIO_CONSOLE` added to `runtime.config` for amd64 | AC-1 (vz serial log in the Lima instance dir), AC-9 (QEMU) | confirmed for vz 2026-10-21: with `console=hvc0` last on the cmdline, the kernel log, gokrazy init's lines and ze-docker-host's console lines all reach `serialv.log`; `ttyS0` under QEMU is AC-9 |
| A-3 | gokrazy init's 1 MB tmpfs `/etc` can be remounted larger by the docker-host program and stays writable for passwd, group, shadow, sudoers, ssh, docker, containerd, systemd unit files | gokrazy `mount.go` line 65 mounts it with `size=1M` and nothing else holds it | Colima's writes fail on ENOSPC; the program mounts a fresh tmpfs over `/etc` after copying gokrazy's entries | AC-11 unit test over a temp root; AC-2 | broken 2026-10-21: the remount answered `invalid argument` on the first vz boot. On a squashfs root `/etc` is part of the read-only root; gokrazy mounts its 1 MB tmpfs `/etc` only in `mountCompat`, for old FAT roots. Fallback taken: `mountWritableEtc` (`etc.go`) snapshots the squashfs `/etc`, mounts a 64 MB tmpfs over it and writes the entries back; `TestDockerHostBootSequenceOverATempRoot` covers it (red before the code) |
| A-4 | overlay2 works with its upper dir on `/perm` (ext4), and `/var/lib/docker` resolves through `/var` → `/perm/var` | gokrazy `mount.go` 158-170; `OVERLAY_FS=y` | dockerd falls back to vfs or fails; the program bind-mounts a dedicated ext4 directory | AC-2 (`docker info` Storage Driver `overlay2`) | unvalidated |
| A-5 | The pinned Docker static release (29.x) runs with `--firewall-backend=nftables` on Ze's kernel without an `iptables` binary, given `ip_forward` is set by the program | Docker docs (29.0.0, experimental, no forwarding by itself); kernel has `NF_TABLES` set; whether moby's nftables backend shells out to `nft` is unverified | ship static `iptables` (legacy, `IP_NF_IPTABLES_LEGACY=y` is present) or `nft` from pinned source and use that backend; decided in Phase 2 by reading moby's `libnetwork` nftables code at the pinned tag | Phase 2 read of moby source at the pin, then AC-3 and AC-13 (published ports and container NAT work) | unvalidated |
| A-6 | Rosetta amd64 emulation in the `ze` profile works with `BINFMT_MISC` and `VIRTIO_FS` in Ze's kernel and Lima's own rosetta registration in `05-lima-mounts.sh` under busybox | colima `yaml.go` 36-43 sets `Rosetta.Enabled` and `BinFmt`; `05-lima-mounts.sh` handles a `rosetta` tag | AC-15 cannot run the VPP image on arm64; the run records the refusal per the waiting spec's R-6 and the owner is told | AC-15 | unvalidated |
| A-7 | Static OpenSSH built `--without-openssl` covers Lima's needs: ed25519 user and host keys, ControlMaster, `-L` to a unix socket (direct-streamlocal), exec | Lima generates ed25519 keys; OpenSSH's internal crypto has ed25519, chacha20-poly1305, aes-ctr | Lima cannot connect; build with a pinned static OpenSSL | AC-1 (`limactl shell` and the socket forward) | unvalidated |
| A-8 | Busybox v1.34.1 from ze-serial-shell runs every command `boot.sh`, the `boot.Linux` scripts that do not exit early, Colima's provisioning and `disk.sh` need, as `/bin/sh` and `/bin/bash` | applet names in the binary (Key insights); `boot.sh` uses `sed -E`, `od`, `paste`, `date -Iseconds`, `[[ ]]` in `05-lima-mounts.sh` | a script fails; the fix is a newer busybox built from pinned source (Phase 2 decides by running the scripts under the image), never a script edit | AC-1 boot log has no `WARNING: Failed to execute` for a script the contract needs | unvalidated |
| A-9 | gokrazy extra files can place executables at `/bin`, `/usr/bin`, `/usr/sbin` in the squashfs root with mode and setuid preserved (sudo needs setuid root) | ze-serial-shell precedent places `usr/local/bin/ze-recovery-shell`; setuid not yet seen | sudo cannot elevate; the program copies sudo to a tmpfs with setuid at boot | AC-11, AC-2 (`sudo docker info` as the Lima user) | unvalidated |
| A-10 | Lima keeps a raw `--disk-image` sparse when it copies it into `~/.colima/_lima/colima-ze/`, and the image's `/perm` is sized for lab images | `lima` disk conversion (not read) | the copy takes the full size on the Mac; the image size is lowered and `/perm` grown at first boot instead | AC-2 (`du` of the Lima disk against its apparent size) | unvalidated |
| A-11 | Colima's runtime data disk (`disk.sh`) can be left failing: `30-install-packages.sh` exits with the dependency scripts' code, and boot still reaches `boot-done`; docker data then lives on `/perm` | `boot.sh` writes boot-done unconditionally; colima `disk.go` `useRuntimeDisk` only adds the disk when `limautil.HasDisk()` | if Colima mounts the data disk over `/var/lib/docker` later, data moves; the program then implements the data disk (format by label, mount) in Go | AC-2 and a second `colima start` after `colima stop` keeps pulled images | unvalidated |
| A-12 | A `docker-host` builder container for OpenSSH and sudo is a build tool, not a VM or test guest, so M1 and M10 do not govern its base image | M1 names "VMs and test guests", M10 the lab Docker host | the owner rules otherwise | owner ruling at the design gate | confirmed 2026-10-21: owner chose Alpine for this role (D-13) |
| A-13 | Lima's DNS for the guest works without `iptables` in the guest (`09-host-dns-setup.sh` skips when `iptables` is absent) because the DHCP lease names a usable resolver | `09-host-dns-setup.sh` `command -v iptables`; Colima `HostResolver.Enabled` when no DNS is configured | image pulls fail on name resolution; the program writes `/etc/resolv.conf` from `lima.env`'s DNS address | AC-13 (the lab pulls its images) | unvalidated |
| A-14 | `CONFIG_VHOST_NET=y` in `runtime.config` resolves to `=y` | AC-4 lists it as a plain request | the build refuses it | Phase 1 arm64 kernel build | broken 2026-10-21: refused "CONFIG_VHOST_NET did not resolve to =y"; Kconfig caps it at TAP, which arm64 defconfig sets to m through MACVTAP=m. Fixed by `CONFIG_MACVTAP=y` (runtime.config, runtime.require); rebuild owed (`plan/handover/13-ze-image-docker-host-phase-1.md`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Guest agent not alive within one minute: Colima reports Degraded and `colima start` fails | `limactl start` prints `guest agent does not seem to be running`; AC-1 fails | the agent is started by Lima's own `25-guestagent-base.sh` through the shim; the shim supports `daemon-reload`, `enable`, `try-restart`, `start` for a unit Lima wrote and expands `%t`, `%N`; AC-11 unit test covers that exact argv sequence |
| R-2 | The shim makes systemd-only branches run: `00-check-rtc-and-wait-ntp.sh` (when no `/dev/rtc0`) calls `timedatectl`, `20-rootless-base.sh` edits dotfiles and runs `daemon-reload`, and `boot.sh` blocks forever before `provision.user` scripts | boot log stalls before `boot-done`; `WARNING: Failed to execute 00-check-rtc...` | the program creates `/run/user/<uid>/systemd/private` (an empty socket file) after the user exists so the wait ends; the shim answers `daemon-reload` and unknown verbs with systemd's exit codes; a `timedatectl` is not shipped, so that script fails fast (`set -eu`) as a warning. Each branch is listed in AC-1's boot log check |
| R-3 | `/usr/local` is read-only squashfs, so the agent install fails | `install: can't create '/usr/local/bin/lima-guestagent'` | the program mounts an overlay on `/usr/local` (lower squashfs, upper tmpfs) before `boot.sh` |
| R-4 | Colima's `docker.conf` drop-in assumes `-H fd://`; obeying it verbatim leaves dockerd without a socket | `docker.service` never active; Colima retries 60 times | the supervisor runs a fixed dockerd argv (unix socket `/var/run/docker.sock`, group `docker`, containerd socket, nftables backend) and takes only `--host-gateway-ip` from the drop-in (D-7) |
| R-5 | A rebuilt image is ignored because Colima keeps the existing disk | `uname -r` or the image key file in the guest differs from the cache key | setup records the started image key and recreates the profile on change (AC-7) |
| R-6 | Setup touches `default` by accident (a missing `--profile`) | the recorder test sees an argv without `--profile ze` | every colima argv is built by one function that always sets `--profile ze`; AC-7's test fails on any other argv |
| R-7 | The host-driven docker-lab breaks bind mounts: the kernel probe mounts the client's absolute path of the linux ze | probe container fails with `no such file` | the QEMU guest mounts the 9p checkout and scratch shares at the host's absolute paths (from `user-data` mounts the program applies); Colima mounts `$HOME` by virtiofs at the same path |
| R-8 | Upstream Docker static tarball removed or changed | sha256 mismatch on fetch | the fetch refuses naming file, expected and got sums and the pin file; the cache keeps the last good copy |
| R-9 | Nightly amd64 KVM boots differ (BIOS vs EFI) | AC-9 amd64 run hangs at firmware | amd64 uses SeaBIOS with gokrazy's MBR boot code; if that fails, OVMF EFI as on arm64 |
| R-10 | Two sessions run `./le setup docker-host start` at once | one `colima delete` under the other's start | setup takes an exclusive file lock under `~/.cache/ze/docker-host/` for the duration of `start`/`stop` |
| R-11 | Removing `ZE_DOCKER_LAB_GUEST` changes the refusal text a lab prints inside the guest; nothing runs le in the guest any more | grep finds a remaining reader | the variable, `inDockerLabGuest` and their tests are deleted in the same phase (no layering) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Docker labs on the Mac and the nightly docker-lab jobs cannot reach a daemon; nothing in the shipped `ze` binary or the appliance image changes |
| How is it reverted? | single commit revert per phase; the `ze` Colima profile is deleted with `colima delete --profile ze --force` |
| Who else touches this path? | `plan/spec-lab-docker-cleanup-on-macos.md` (waits on this), `plan/spec-lab-kernel-features-in-setup.md` (`VHOST_NET`), `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` (docker-lab, D-5), `plan/pre-release/spec-lab-containers-least-privilege.md` (names the Alpine guest), `plan/spec-lab-docker-cleanup-and-go-test-containers.md` (throwaway guest assumption) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le setup docker-host build` | → | docker-host area `build` action → image builder (Prepare + gok) | `TestDockerHostBuildActionBuildsTheInstance` (`internal/le/setup/dockerhostimage/build_test.go`, injected fetcher and gok runner) |
| `./le setup docker-host start` | → | colima argv builder, image-key compare, delete-then-start | `TestDockerHostStartRecreatesProfileOnNewImage` (`internal/le/setup/dockerhostimage/colima_test.go`, recorder colima) |
| `./le setup docker-host start` | → | same | `TestDockerHostNeverNamesTheDefaultProfile` (same file) |
| any darwin Docker lab | → | `dockerhost.Environment` darwin candidate | `TestEnvironmentSelectsTheZeProfileSocket` (`internal/le/dockerhost/dockerhost_test.go`) |
| darwin Docker lab with no socket | → | kernel check route text | `TestKernelCheckRouteNamesDockerHostSetup` (`internal/le/interoplab/kernelcheck_test.go`) |
| `./le test qemu docker-lab` | → | image boot, cidata writer, socket forward, host le run | `TestDockerLabBootsTheDockerHostImage` (`internal/le/test/qemu/dockerlab_test.go`, fake qemu and ssh) |
| gokrazy init starts `/user/ze-docker-host` | → | cidata mount, users, supervisor, `boot.sh` | `TestDockerHostBootSequenceOverATempRoot` (`internal/appliance/dockerhost/boot_test.go`) |
| `sudo systemctl ...` in the guest | → | shim → supervisor control socket | `TestSystemctlShimLimaAndColimaVerbs` (`internal/appliance/dockerhost/systemctl_test.go`) |
| the real boot | → | all of the above | AC-1, AC-2, AC-9 runs, output pasted |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | On the arm64 Mac, with `LIMA_HOME` set to a new directory under the session scratch (never `~/.colima`), `limactl start` on a Lima template naming the built image as a raw `images:` entry, `vmType: vz`, `containerd: {system: false, user: false}` | `limactl list` shows the instance `Running` (not `Degraded`) within the start timeout; `limactl shell <inst> cat /run/lima-boot-done` prints the instance id from `meta-data`; `limactl shell <inst> uname -r` ends in `-ze`; the boot log (`serial*.log` in the instance dir) shows systemd-boot, Ze's kernel banner and `LIMA ... Exiting with code`; every `WARNING: Failed to execute` line in it names a script listed in the spec's R-2 or A-11 rows, and no other; `limactl delete --force` then removes it |
| AC-2 | `./le setup docker-host start` on the arm64 Mac with the image built | exits 0; `colima list` shows profile `ze` `Running` with runtime docker; `~/.colima/ze/docker.sock` is a socket; `DOCKER_HOST=unix://$HOME/.colima/ze/docker.sock docker info` reports a Kernel Version ending in `-ze`, Storage Driver `overlay2`, Cgroup Version 2; `colima ssh --profile ze -- docker info` succeeds as the Lima user without sudo; `colima ssh --profile ze -- sudo docker info` succeeds; `colima list` output for `default` (status, arch, runtime, disk) is identical before and after the run |
| AC-3 | `./le setup docker-kernel check` with DOCKER_HOST pointing at the `ze` profile | every enrolled feature row is present, including `ipsec-mobike`, `l2tp`, `l2tp-ppp`, `mpls`, `mpls-transit-mtu`; exit 0 |
| AC-4 | `gokrazy/kernel/runtime.config` and `runtime.require` | each carries `CONFIG_VSOCKETS`, `CONFIG_VIRTIO_VSOCKETS`, `CONFIG_INET_UDP_DIAG`, `CONFIG_VIRTIO_FS`, `CONFIG_VHOST_NET`, `CONFIG_BINFMT_MISC`, `CONFIG_VIRTIO_CONSOLE`, each once with a comment naming the consumer (Lima guest agent vsock, guest agent port listing over sock_diag, Colima virtiofs mounts, VPP `/dev/vhost-net`, Rosetta binfmt, vz console); the runtime kernel build for arm64 and amd64 exits 0 and its emitted config shows each `=y`; `TestHostDependencyOptionsAreInTheRuntimeProfile` (or its successor) fails when one is removed from the fragment |
| AC-5 | `./le setup docker-host build` with inputs cached, then again with nothing changed, then with one pinned sha256 altered in a temp copy of the pin file passed to the builder | first run writes the image under `~/.cache/ze/docker-host/<arch>/<key>/` and prints its path; second run prints the same path and builds nothing (no gok run); the third exits 1 before any image is written, naming the file, the expected and actual sha256, and the pin file to fix; the image's root contains no `/etc/machine-id` |
| AC-6 | The `ze` profile stopped and started twice, plus the AC-1 Lima instance | `/etc/machine-id` in the `ze` guest is 32 lowercase hex, identical across the two starts and equal to `/perm/machine-id`; it differs from the AC-1 instance's; `/proc/sys/kernel/random/boot_id` differs between the two starts |
| AC-7 | `./le setup docker-host start` run once with image key K1 recorded, then after an input change producing K2 | the second run prints and runs `colima stop --profile ze`, `colima delete --profile ze --force`, then `colima start --profile ze ...` with the new image, and records K2; every colima argv in both runs carries `--profile ze`; with K2 unchanged a third run starts nothing when the profile is running. Proved by the recorder tests in Wiring and by the real run output pasted |
| AC-8 | A darwin Docker lab (`./le test integration interop-ipsec`) with DOCKER_HOST unset, `/var/run/docker.sock` absent: (a) `~/.colima/ze/docker.sock` present; (b) absent while `~/.colima/default/docker.sock` is present | (a) the lab's Docker calls carry `DOCKER_HOST=unix://$HOME/.colima/ze/docker.sock`; (b) exits 1 before any image build or container, naming the missing socket, that labs on macOS run on the `ze` Colima profile, and `./le setup docker-host start` on a line of its own; the `default` socket is never used |
| AC-9 | `./le test qemu docker-lab` (no `lab`) on the arm64 Mac under HVF, and the nightly's amd64 run under KVM | the guest boots the docker-host image for that arch; the host le runs `setup docker-kernel check` against the forwarded socket and it passes; no Alpine ISO or initramfs is read or fetched (the `alpine-iso` cache dir mtime unchanged), no guest le and no Go toolchain is built for the guest; exit 0; the image file's sha256 is unchanged after the run |
| AC-10 | `./le test qemu docker-lab lab "perf run dut ze"`, Ctrl-C once while the DUT runs | the host le running the lab receives the interrupt and runs its own teardown; then le lists the lab's `ze.lab` containers, networks and `ze-interop-retained:` images on the guest daemon and prints `left behind: none`; the VM is stopped after that; exit 1 |
| AC-11 | The shim inside a booted guest (AC-1 instance or `ze` profile): `sudo systemctl daemon-reload`, `sudo systemctl restart containerd.service`, `sudo systemctl start docker.service`, `systemctl is-active docker.service`, `sudo systemctl stop docker.service`, `sudo systemctl start nosuch.service` | the first four exit 0 and `is-active` prints `active`; after stop, `is-active` prints `inactive` and exits 3; the last exits 5 with a message naming the unit and the units the shim knows; `lima-guestagent.service` started through `enable`/`try-restart`/`start` runs the `ExecStart` Lima wrote, with `%t` and `%N` expanded |
| AC-12 | `/etc/containerd/config.toml` on a fresh boot, before Colima provisions | exists (seeded by the program), so Colima's `sudo cp` backup succeeds; after `colima start`, `/etc/containerd/config.colima.bak.toml` exists and containerd runs with Colima's config |
| AC-13 | Waiting spec AC-2, on the `ze` profile: `./le perf run dut ze`; while the DUT runs, `docker ps --filter label=ze.lab --format '{{.ID}}'`, then `docker inspect --format '{{json .Config.Labels}}'` on the DUT container | `ze.lab.machine-id` equals the lowercase IOPlatformUUID; `ze.lab.boot-id` equals the lowercase `kern.bootsessionuuid`; `ze.lab.os` is `darwin`; `ze.lab.pid` is the pid of the running `le`; `ze.lab.pid-ns` is absent |
| AC-14 | Waiting spec AC-6, on the `ze` profile: `./le perf run dut ze`; once `docker ps --filter label=ze.lab` lists the DUT and the sender, press Ctrl-C once; then, with the run id from the DUT's `ze.lab.run` label, `docker ps -a --filter label=ze.lab.run=<id>`, `docker network ls --filter label=ze.lab.run=<id>` and `docker image ls --filter label=ze.lab.run=<id> --format '{{.Repository}}:{{.Tag}}'` | the run exits 1 after its removals; the first two commands print no resource; the third prints no `ze-interop-retained:` tag (the kept DUT image under its cache tag is allowed); `./le test integration cleanup dry` names no resource of that run |
| AC-15 | Waiting spec AC-8, on the `ze` profile: for `vpp-test` and then `vpp-iface-test`, start `./le test deployment <proof>` from a terminal; once `docker ps --filter label=ze.lab` lists its container, press Ctrl-C once; then the same three `docker ... --filter label=ze.lab.run=<id>` commands | each run exits 1 and prints no verdict line, only the interruption; the first two commands print no resource; the third prints no `ze-interop-retained:` tag (the pulled VPP image is allowed); `./le test integration cleanup dry` names no resource of that run |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrepareTakesTheInstanceName` | `internal/appliance/instance/prepare_test.go` | Prepare copies the named instance; empty name refused | |
| `TestDockerHostBuildActionBuildsTheInstance` | `internal/le/setup/dockerhostimage/build_test.go` | Prepare called with `docker-host`, extra files per arch, no `injectZeFS` | |
| `TestDockerHostBuildRefusesAChecksumMismatch` | same | AC-5 refusal text and that no image is written | |
| `TestDockerHostBuildKeyIsStable` | same | same inputs, same key; any pin, kernel key or source change, new key | |
| `TestDockerHostStartRecreatesProfileOnNewImage` | `internal/le/setup/dockerhostimage/colima_test.go` | AC-7 argv sequence | |
| `TestDockerHostNeverNamesTheDefaultProfile` | same | every argv carries `--profile ze` | |
| `TestDockerHostStartTakesTheLock` | same | R-10 second concurrent start waits or refuses naming the holder pid | |
| `TestEnvironmentSelectsTheZeProfileSocket` | `internal/le/dockerhost/dockerhost_test.go` | AC-8 (a), and that `default` is never chosen | |
| `TestKernelCheckRouteNamesDockerHostSetup` | `internal/le/interoplab/kernelcheck_test.go` | AC-8 (b) text on darwin; Linux text unchanged | |
| `TestCidataParsesLimaFiles` | `internal/appliance/dockerhost/cidata_test.go` | `lima.env`, `param.env`, `meta-data` instance id, `user-data` users, keys, mounts | |
| `TestDockerHostBootSequenceOverATempRoot` | `internal/appliance/dockerhost/boot_test.go` | passwd, group (`docker` gid matching Colima's containerd `gid = 1000`), sudoers, authorized_keys, hostname, `/usr/local` overlay request, machine id persisted to `/perm`, `/run/user/<uid>/systemd/private` created, `boot.sh` invoked with `LIMA_CIDATA_MNT` | |
| `TestSystemctlShimLimaAndColimaVerbs` | `internal/appliance/dockerhost/systemctl_test.go` | AC-11 argv set and exit codes 0, 3, 5; `%t`/`%N` expansion | |
| `TestSupervisorDockerdArgv` | `internal/appliance/dockerhost/supervise_test.go` | fixed argv, `--host-gateway-ip` taken from Colima's drop-in, nftables backend, restart on exit | |
| `TestDockerLabBootsTheDockerHostImage` | `internal/le/test/qemu/dockerlab_test.go` | qemu argv has the image (`-snapshot`), cidata drive, firmware per arch, 9p shares at host paths; no `-kernel`/`-initrd`; host le command carries the forwarded DOCKER_HOST | |
| `TestDockerLabWritesCidata` | `internal/le/test/qemu/cidata_test.go` | the cidata volume has label `cidata` and the file set the program reads | |
| `TestHostDependencyOptionsAreInTheRuntimeProfile` (extended) | `internal/le/setup/dockerkernel/hostdeps_test.go` | AC-4 symbols | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `LIMA_CIDATA_UID` | 1-65534 | 65534 | 0 (root refused) | 65535 |
| `LIMA_CIDATA_VSOCK_PORT` | 0-4294967295 (0 means none) | 4294967295 | N/A | 4294967296 |
| image size (GiB) | 2-1024 | 1024 | 1 | 1025 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestDockerHostAreaIsRegistered` | `internal/le/setup/dockerhostimage/register_test.go` (le has no `.ci` harness; this runs the registered area through `leroot`'s dispatcher as `./le setup docker-host` does) | `./le setup docker-host` lists build, start, stop, check with their keywords | |
| AC-1, AC-2, AC-9 real runs | the owner's Mac and the nightly, output pasted in the spec | boot under limactl, Colima `ze`, QEMU docker-lab | |

### Interop Tests (Scope: protocol)
N-A: tooling. The labs these daemons serve are the interop suites themselves; AC-3 and AC-13 to AC-15 run them on the new host.

## Files to Modify
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - AC-4 symbols with reasons.
- `internal/appliance/instance/prepare.go` - instance name in `Options`; `ze` callers updated (`internal/appliance/cmd_build.go`, `internal/le/build/gokrazy`).
- `internal/le/build/gosum/gokrazygosum.go` - every `gokrazy/*/builddir/`.
- `internal/le/dockerhost/dockerhost.go` - darwin candidate `~/.colima/ze/docker.sock`.
- `internal/le/interoplab/kernelcheck.go` - darwin route text; `DockerLabGuestVariable` and `inDockerLabGuest` deleted.
- `internal/le/test/qemu/dockerlab.go` - image boot, cidata, socket forward, host le; `dockerLabPackages`, `dockerLabCommand`, guest le build removed.
- `internal/le/test/qemu/guestle.go` - keeps only what `stress.go` uses.
- `internal/le/setup/dockerkernel/dockerkernel_apparmor_colima.go` - targets the `ze` profile where it names `default` (AppArmor is not loaded on Ze's kernel; its route text names `docker-host`).
- `docs/guide/developer-setup.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/testing/interop.md`, `docs/architecture/testing/ci-workflows.md`, `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md`, `docs/architecture/testing/runner-architecture.md`, `docs/architecture/appliance/build-artifacts.md`, `docs/architecture/appliance/gokrazy-build-pins.md`, `docs/architecture/appliance/README.md`, `docs/guide/appliance.md`, `ai/INDEX.md` (keyword row) - per the Documentation checklist.
- `ai/rules/platform-linux.md` and its point file `ai/rules/points/platform-linux/interop-labs-and-docker-based-tests-need-a-qemu-runner-too/ship-a-qemu-path-beside-every-docker-interop-lab.md` - the QEMU route now boots Ze's docker-host image (rule text regenerated by `./le ai rules condensed-update`).
- Append-only pointers: `plan/spec-lab-docker-cleanup-on-macos.md` (AC-2, AC-6, AC-8 run here as AC-13 to AC-15), `plan/pre-release/spec-docker-hosts-run-the-ze-kernel.md` (D-6 Mac route, AC-8, AC-10 guest is this image), `plan/spec-lab-kernel-features-in-setup.md` (`VHOST_NET` lands here if not already), `plan/pre-release/spec-lab-containers-least-privilege.md` and `plan/spec-lab-docker-cleanup-and-go-test-containers.md` (their Alpine guest is this image).

## Files to Create
- `gokrazy/docker-host/config.json`, `gokrazy/docker-host/builddir/` - the second instance: packages `cmd/ze-docker-host`; GokrazyPackages `randomd`, `dhcp` (no `heartbeat`, no baked machine id); no `cmd/ze`.
- `gokrazy/docker-host/binaries.json` - the pin file (D-4).
- `cmd/ze-docker-host/main.go` - gokrazy package; dispatches on `argv[0]` (`systemctl` runs the shim).
- `internal/appliance/dockerhost/` - `cidata.go`, `users.go`, `mounts.go`, `supervise.go`, `systemctl.go`, `boot.go` with tests.
- `internal/le/setup/dockerhostimage/` - `register.go` (area `docker-host`), `build.go`, `fetch.go`, `colima.go`, `limacheck.go` (AC-1 helper template writer), tests.
- `internal/le/test/qemu/cidata.go` - cidata volume writer for the QEMU route.
- No `.ci` file: le actions have no functional `.ci` harness (no `test/le/` directory exists); the dispatcher test above stands in.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling; no ze config or RPC |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `internal/le/setup/dockerhostimage/register.go` (le area `setup docker-host`: `build`, `start`, `stop`, `check`); `docker-lab` keywords unchanged |
| CLI grammar (keyword before value) | Yes | `start arch <a>` style keywords through `leaction` (`ai/rules/cli.md`) |
| Editor autocomplete | N-A | le actions complete from the registered action list |
| Functional test for new RPC/API | N-A | no RPC; the le area is proved by `TestDockerHostAreaIsRegistered` and the AC-2, AC-7 real runs |
| Pipe completeness | N-A | le action output, not a ze CLI command |
| Env var registration | N-A | no `ze.*` variable; `ZE_DOCKER_LAB_GUEST` is deleted |
| Doctor check for runtime dependencies | No | development host only; `./le setup docker-host check` reports colima, limactl, qemu, the image and the socket, the way `./le setup check` reports tools. `ze doctor` is for the product |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | development tooling; no `features/<id>.md` covers lab hosts |
| 2 | Config syntax changed? | N-A | no ze config |
| 3 | CLI command added/changed? | No | `docs/guide/command-reference.md` covers `ze`; the le area is documented in `docs/guide/developer-setup.md` (row 6) |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` "Docker labs": macOS bullet rewritten for the `ze` profile and `./le setup docker-host start`, QEMU route second |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC; the labs' RFC evidence keeps its tags |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` (docker-lab section, guest section, kernel pairing, and the "QEMU VM has no Docker" paragraph ~845), `docs/architecture/testing/interop.md` (~637-673 route text, ~811), `docs/architecture/testing/ci-workflows.md` (docker-lab jobs), `docs/architecture/testing/runner-architecture.md` (~142: docker-lab's "two admitted cross-compiles" become the linux ze compile plus the image build, both under the verb root; the guest le compile goes), `docs/labs/l2tp-interop.md`, `docs/labs/pppoe-interop.md` |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/appliance/build-artifacts.md` (second gokrazy instance, pin file, cache path); `docs/architecture/appliance/gokrazy-build-pins.md` (named prepared instance, every instance's builddir pinned); `docs/guide/appliance.md` (~408: `Prepare` copies the named instance); a new section in `docs/architecture/appliance/README.md` for the docker-host instance; `docs/architecture/appliance/docker-host.md` (new in Phase 1: the instance, the guest boot, the systemctl shim) |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | le area only |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation with `./le spec citation anchors spec plan/spec-ze-image-docker-host.md`; known now: `qemu-integration.md` and `interop.md` anchor `dockerlab.go` and `kernelcheck.go`. Declared by `// Design:` headers: `docs/architecture/appliance/gokrazy-build-pins.md` (declared by `prepare.go` and `gokrazygosum.go`) is affected and updated in Phase 1: a prepared instance is named, and every `gokrazy/*/builddir` carries pins checked by the gosum gate; `docs/architecture/core-design.md` (declared by `dockerhost.go`, "le's composition, one import per tool") is unaffected: the endpoint candidate changes, le's composition and imports do not |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `developer-setup.md` and `qemu-integration.md` show `./le test qemu docker-lab lab ...`; the commands stay, the described guest changes |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md`: a new keyword row "docker-host image, ze Colima profile, lab Docker host on macOS" → `docs/guide/developer-setup.md`, `docs/architecture/appliance/build-artifacts.md`, `./le setup docker-host` |
| What rule prevents regression? | M10 row; `ai/rules/platform-linux.md` QEMU-path point updated to name the docker-host image |
| What registry prevents drift? | `leroot.Register` for the area; `runtime.require` for the kernel symbols; `gokrazy/docker-host/binaries.json` is the only home of binary versions and sums |
| What verification proves it? | `./le setup docker-host check`; `./le setup docker-kernel check`; the nightly docker-lab jobs |

## Implementation Steps

1. **Phase: Boot contract proof (MANDATORY FIRST, AC-1)** - the smallest image that meets the Lima contract, proven under `limactl` before any setup or QEMU work.
   - Tests: `TestPrepareTakesTheInstanceName`, `TestCidataParsesLimaFiles`, `TestDockerHostBootSequenceOverATempRoot`, `TestSystemctlShimLimaAndColimaVerbs`
   - Files: `prepare.go`, `gokrazy/docker-host/`, `cmd/ze-docker-host`, `internal/appliance/dockerhost/` (no dockerd yet), kernel options of AC-4, busybox, sshd, sudo
   - Verify: AC-1 with a throwaway `LIMA_HOME`; settle A-1, A-2, A-3, A-7, A-8, A-9 and record each result in the Assumptions table. If A-1 fails, stop and report: the fix is a packer change and the design gate is re-opened
2. **Phase: Docker and pins (AC-5, AC-11, AC-12)** - pin file, fetch and source builds, dockerd, containerd supervision, seeded containerd config; read moby's nftables backend at the pinned tag and settle A-5.
   - Tests: `TestDockerHostBuildRefusesAChecksumMismatch`, `TestDockerHostBuildKeyIsStable`, `TestSupervisorDockerdArgv`
   - Verify: AC-11 and AC-12 on the AC-1 instance
3. **Phase: `./le setup docker-host` and Colima (AC-2, AC-3, AC-6, AC-7)** - area registration, build, start, stop, check, lock.
   - Tests: wiring rows 1-3, `TestDockerHostStartTakesTheLock`
   - Verify: AC-2, AC-3, AC-6, AC-7 real runs, output pasted; settle A-4, A-10, A-11
4. **Phase: darwin endpoint (AC-8)** - `dockerhost.Environment`, kernelcheck route, AppArmor-colima text; docs rows 6 and 10 for these lines in the same phase.
5. **Phase: QEMU docker-lab (AC-9, AC-10)** - image boot, cidata writer, 9p shares at host paths, socket forward, host le, Ctrl-C; delete the Alpine docker-lab path, `ZE_DOCKER_LAB_GUEST`, the guest le build for docker-lab; `qemu-integration.md` and `ci-workflows.md` in the same phase.
6. **Phase: Closing runs (AC-13, AC-14, AC-15)** - on the `ze` profile with Rosetta; settle A-6, A-13; append the pointers in other specs.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line and pasted output for the real runs |
| Correctness | Lima's own `boot.sh` writes `boot-done` (not the program); the program never writes `/run/lima-boot-done` |
| Correctness | `default` profile untouched: every colima argv built by one function with `--profile ze` |
| Naming | area `docker-host`, profile `ze`, instance `docker-host`, binary `ze-docker-host`; error text names the command that fixes it (`ai/rules/user-facing-errors.md`) |
| Data flow | binary versions live only in `binaries.json`; the kernel options only in `runtime.config`/`runtime.require` |
| Rule: no-layering | no Alpine fallback in docker-lab, no `default` socket fallback in `dockerhost.Environment` |
| Rule: M1 | the remaining Alpine `Run` guests (`run`, `stress`, `mobike`, `all-tests`) get one journal row, nothing else |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| docker-host image for arm64 and amd64 | `./le setup docker-host build arch arm64` and `arch amd64` print a path; `ls -l` the files |
| `ze` Colima profile running Ze's kernel | `colima list`; `docker info` Kernel Version |
| docker-lab without Alpine | `grep -n "alpine\|apk\|rc-service" internal/le/test/qemu/dockerlab.go` prints nothing |
| No `default` fallback | `grep -n "colima\", \"default\"" internal/le/dockerhost/dockerhost.go` prints nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | cidata values (user name, uid, home, keys, hostname) are validated before they reach passwd, sudoers or a path: user and hostname against a strict pattern, uid as in the boundary table, no `/` or `..` in names |
| Privilege | sudoers grants NOPASSWD to the Lima user only (matching Lima's `90-lima-users`); sshd allows public-key only, no root login, no passwords |
| Supply chain | every fetched or built binary checked against `binaries.json` sha256 before it enters an image; source tarballs pinned by sha256 too |
| Exposure | dockerd listens only on the unix socket; the QEMU forward binds a socket under the session's scratch with mode 0600 |
| Resource | the supervisor restarts a dying daemon with backoff, never in a tight loop |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-1 fails (vz will not boot the image) | Stop, report, re-open the design gate |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Lima contract is mostly Lima's own shell code; meeting it with Lima's `boot.sh` under busybox keeps Ze's Go to what cloud-init does (cidata, users, keys, hostname, mounts) plus a supervisor, and lets a Lima upgrade bring its own script changes.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 (owner) Static third-party binaries per arch; Lima boot steps and a `systemctl` shim in Go | Ze's SSH server grown into a guest shell; Colima `--runtime none` | owner decision, 2026-10-21 (Task) |
| D-2 A second gokrazy instance `gokrazy/docker-host` on the same runtime kernel, built through `instance.Prepare` and gok | a `docker-host` kernel profile; reusing the `ze` instance with dockerd added | D-5: one kernel; the NOS appliance image must not carry dockerd |
| D-3 Init: a Go program does cloud-init's part (mount cidata by label `cidata`, iso9660 or vfat; user, `docker` group, sudoers, `authorized_keys`, hostname, `user-data` mounts), starts sshd, persists the machine id on `/perm`, mounts the `/usr/local` overlay, then runs Lima's own `boot.sh` under busybox sh | running alpine-lima's `lima-init.sh` under busybox | `lima-init.sh` needs `useradd`/`adduser` semantics, `awk` YAML scraping of `user-data`, and OpenRC (`rc-update`, `rc-service sshd`), and `boot.essential.Linux` scripts gate on `/etc/alpine-release`; in Go the parsing is testable over a temp root, no Alpine marker is faked, and `boot.sh` (which Lima ships with the cidata) still runs unchanged |
| D-4 Binary sourcing: Docker's official static tarballs pinned by version and sha256; OpenSSH (`--without-openssl`, A-7) and sudo (`--without-pam`) built from pinned source tarballs as fully static musl binaries in the pinned Alpine builder container on the Docker backend (D-13), cached under `~/.cache/ze/docker-host/`; busybox reused from `cmd/ze-serial-shell/_gokrazy/` (A-8); no new large binary committed | commit the binaries; fetch distribution packages | the committed busybox tar is the only binary in git and is already there; pins in one file are the single source (principles) |
| D-5 dockerd with `--firewall-backend=nftables` and `ip_forward` set by the program, subject to A-5; otherwise static legacy `iptables` from pinned source | iptables backend with `iptables-nft` | Ze's kernel carries both; the nftables route ships one binary fewer; A-5 settles it in Phase 2 |
| D-6 QEMU docker-lab is host-driven: the guest runs dockerd only; le on the host reaches it through `ssh -L` to `/var/run/docker.sock` with `DOCKER_HOST` | keep the in-guest le (built for the guest, run over ssh) | the guest le needs a Go toolchain, the patched GOROOT and 9p workspace in the guest, a second identity (the empty machine id journaled under M1) and a Ctrl-C forwarding layer; host-driven gives one le, the host's identity (as the Colima route does) and its own Ctrl-C teardown. Cost: bind-mounted host paths must exist in the guest, met by 9p shares at the same absolute paths (R-7) |
| D-7 The supervisor runs a fixed dockerd argv and reads only `--host-gateway-ip` from Colima's drop-in | honour `ExecStart` verbatim; implement systemd socket activation (`LISTEN_FDS`) | `-H fd://` needs socket activation, which is a second init to write for one flag; Colima's other daemon settings travel in `daemon.json`, which dockerd reads itself |
| D-8 New le area `setup docker-host` | new actions in `setup docker-kernel` | `docker-kernel`'s area is the kernel check and the Linux kernel install, and its `install` changes a host's boot kernel; the image build and Colima lifecycle are a separate lifecycle with their own lock and Mac-only steps. Each area registers itself (`leroot.Register`), so the split costs no central edit |
| D-9 darwin's lab endpoint is `~/.colima/ze/docker.sock` with no fallback to `default` | keep `default` as a fallback | no-layering; `default` runs Ubuntu, which M10 rules out |
| D-10 QEMU cidata: le writes a vfat or iso9660 volume labelled `cidata` with the Lima file set; library choice (`github.com/diskfs/go-diskfs`, which Lima uses, or a host tool) settled in Phase 5 | QEMU vvfat (`fat:` drive) | gokrazy's FAT writer is internal to its module and cannot be imported; vvfat's label handling is unverified |
| D-11 The guest agent is started by Lima's `25-guestagent-base.sh` through the shim, not by the program directly | the program starts the agent from cidata itself | a second starter races Lima's `install-systemd` (which runs `try-restart` and `start`); one path, the one Lima maintains |
| D-12 Kernel additions: `VSOCKETS`, `VIRTIO_VSOCKETS`, `INET_UDP_DIAG`, `VIRTIO_FS`, `VHOST_NET`, `BINFMT_MISC`, `VIRTIO_CONSOLE`; not `CFS_BANDWIDTH` | add `CFS_BANDWIDTH` too | no lab found that sets a CPU quota (`--cpus`); added only if a lab run refuses for it |

## Known Limitations

- `./le test qemu run`, `stress`, `mobike` and `all-tests` still boot Alpine userland. They are not Docker hosts, so M10 does not reach them through this spec, but M1 does: one row in `plan/journal/` (class of the Alpine-guest rows) at implementation, per `ai/rules/rule-precedence.md`. The kernel builder's QEMU backend (`plan/pre-release/spec-appliance-ships-ze-kernel.md`) is the same case.
- Docker Desktop is not a lab host (handover decision 3).

## RFC Documentation (Scope: protocol)

N-A: tooling.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-ze-image-docker-host.md` only, in the same `./le commit create` script

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | |
| `./le spec review check` | not run |
| Rounds | |
| Reviewer lenses used | |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
