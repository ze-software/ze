# The docker-host image

Ze's lab Docker host is a gokrazy image built from Ze's runtime kernel and a
second instance, `gokrazy/docker-host`. It boots as a Lima/Colima guest on
macOS (the `ze` Colima profile) and as the QEMU `docker-lab` guest, so every lab
Docker daemon runs on Ze's kernel and Ze's system (owner decisions M1 and M10 in
`ai/META-DECISIONS.md`). The design and its acceptance criteria are in
`plan/spec-ze-image-docker-host.md`; this page describes what is built.

## The instance

`gokrazy/docker-host/config.json` packs one Ze program, `cmd/ze-docker-host`,
plus gokrazy's `randomd` and `dhcp`. It bakes no `/etc/machine-id`: the program
generates one per disk on the first boot. `Update.NoPassword` is set, so
gokrazy's web server listens only on its unix socket
(`/run/gokrazy-http.sock`, gokrazy `listeners.go` `updateListeners`) and never
on TCP port 80: Lima forwards every TCP port a guest listens on to the Mac,
and the guest exposes nothing there but sshd (Lima's own channel) and,
through it, the Docker socket. The kernel command line names both
consoles, `hvc0` (vz, virtio console) and `ttyS0` (QEMU), so either boot logs.
The third-party binaries (busybox, a static musl OpenSSH built without OpenSSL,
a static musl sudo, a static legacy iptables, Docker's static release) reach
the image as the extra files tar
`extrafiles_<arch>.tar`, under `/usr/lib/ze-docker-host/`. The tar is assembled
at build time and never committed.

<!-- source: cmd/ze-docker-host/main.go -- main -->

## What the program does at boot

`ze-docker-host` does cloud-init's part of the Lima guest contract in Go, then
runs Lima's own `boot.sh` unchanged under busybox `sh`. Its first console line
is `ze-docker-host: kernel ` followed by `/proc/version`, because under vz the
serial log (`serialv.log` in the Lima instance directory) starts about one
second into the boot, after the kernel printed its own banner. Then it:

1. mounts a 64 MB tmpfs over `/etc` and copies back what the squashfs held
   there (`hosts`, the `resolv.conf` and `localtime` symlinks, `ssl`): on a
   squashfs root gokrazy's `/etc` is read-only (its tmpfs `/etc` exists only
   for old FAT roots, `mount.go` `mountCompat`), and Lima and Colima write
   accounts, configs and unit files there;
2. covers `/mnt` and `/Users` with an empty tmpfs, because both are read-only
   squashfs directories and Lima's mount points live under them (`/mnt` holds
   the cidata and Rosetta mounts, `/Users` a macOS host's home; a Linux host's
   home lands under `/home`), mounts `binfmt_misc` on
   `/proc/sys/fs/binfmt_misc` (systemd does that on a distribution, nothing
   does on gokrazy, and Lima's `05-lima-mounts.sh` registers Rosetta through
   its `register` file: without it Colima warns "unable to enable rosetta"),
   then finds the disk whose ISO 9660 volume id is
   `cidata` (there is no udev, so no `/dev/disk/by-label`) and mounts it
   read-only at `/mnt/lima-cidata`, the path Lima's scripts read;
3. parses `lima.env`, `meta-data` and `user-data`, refusing a user, host name,
   home, key or mount point that would split a passwd or sudoers line or leave
   its directory;
4. keeps the machine id in `/perm/machine-id` and copies it to
   `/etc/machine-id`;
5. sets `/perm/home` to mode 0755 (gokrazy init creates it 0700, which kept
   sshd from reading `authorized_keys`), binds it on `/home`, and overlays a
   tmpfs on two read-only trees: `/usr/local`, where Lima installs its guest
   agent, and `/lib/modules`, where busybox `modprobe` writes its
   `modules.dep.bb` index on its first call. Without that index busybox's
   modprobe (modprobe-small) reads every module in the tree on each call, and
   Lima's `00-modprobe.sh` makes 21 calls: the boot took 101 s to the guest
   agent, 64 to 89 s of it in that script. With the index the first call
   reads the tree once (about 23 s on a cold page cache) and the other 20
   answer at once: 47 s to the guest agent. Busybox `ash` runs its own
   `modprobe` applet in-process whatever `/sbin` holds, so making the index
   writable is the fix rather than a different `modprobe` binary;
6. writes `passwd`, `group` (a `docker` group, gid 1000, holding the Lima
   user), `shadow` (every account locked), `sudoers` and
   `sudoers.d/90-lima-users`, and the user's `authorized_keys`;
7. sets the host name, mounts each `mounts` entry of `user-data` at its own
   path (the host's home over virtiofs, so a bind mount of a host path such
   as `./le setup docker-kernel check` makes resolves in the guest, and
   Rosetta's share), and creates `/run/user/<uid>/systemd/private`, the file
   `boot.sh` waits for before it runs user provisioning scripts. The mount
   options cloud-init would hand to mount(8) are translated: `defaults`,
   `rw`, `auto`, `_netdev`, `comment=` and `x-` words are dropped, `ro` makes
   a read-only mount, and a `nofail` share that fails is reported on the
   console while the boot goes on; any other share failure stops the boot.
   The `/mnt` and `/Users` tmpfs are then remounted read-only. That seal is a
   guard: Colima's provisioning bind-mounts `/mnt/lima-colima-<profile>/docker`
   (and `containerd`, `cni`, `rancher`, `ramalama`) over `/var/lib` when that
   directory exists, and Lima's `05-lima-disks.sh` creates it before the data
   disk mount that fails on this image (no `sfdisk`, no `mkfs.ext4`). On a
   writable `/mnt` Docker's data then lands on the tmpfs and every pulled
   image is gone after `stop`; sealed, it stays on `/perm`;
8. covers `/bin`, `/sbin`, `/usr/bin` and `/usr/sbin` with a tmpfs holding
   links to busybox's applets in those directories (an applet listed
   elsewhere, such as `/linuxrc`, is skipped), sudo, sshd, ssh-keygen and
   the `systemctl` shim, plus `/bin/bash` as busybox `sh`;
9. writes `sshd_config` (public keys only, no root login, stream-local
   forwarding for the Docker socket) and a persistent ed25519 host key;
10. starts the supervisor, sshd, and Lima's `boot.sh`, which writes
    `/run/lima-boot-done` itself.

A failure in steps 1 to 9 is printed on the console and the program exits
with status 125, which gokrazy's supervisor reads as "do not restart": the
sequence is half applied by then, and a restart would fail on an earlier step
(the cidata volume is already mounted) and bury the first error. A program
restarted after step 9 completed serves the supervisor only.

<!-- source: internal/appliance/dockerhost/boot.go -- Boot, mountShareRoots, mountBinfmtMisc, mountShares, sealShareRoots, shareRequest -->
<!-- source: internal/appliance/dockerhost/etc.go -- mountWritableEtc -->
<!-- source: internal/appliance/dockerhost/applets.go -- appletLinks -->
<!-- source: internal/appliance/dockerhost/cidata.go -- ParseCidata -->
<!-- source: internal/appliance/dockerhost/accounts.go -- writeAccounts -->
<!-- source: internal/appliance/dockerhost/guest_linux.go -- RunGuest, installBinaries, prepareSSHD, FindCidata -->

## The systemctl shim

Lima's `lima-guestagent install-systemd` writes
`/etc/systemd/system/lima-guestagent.service` and runs `systemctl --system`
`daemon-reload`, `enable`, `try-restart` and `start`. `/usr/bin/systemctl` is a
link to `ze-docker-host`, which, started under that name, sends its argv to the
supervisor over `/run/ze-docker-host/systemctl.sock`. The supervisor reads a
unit's `ExecStart` and `Restart` when it starts it, expands `%t`, `%N`, `%n`
and `%%`, and refuses any other specifier or `ExecStart` prefix by name. It
answers `daemon-reload`, `enable`, `is-enabled`, `start`, `stop`, `restart`,
`try-restart`, `is-active` and `reload` with systemctl's exit statuses: 3 for a unit
that is not active, 4 when a caller other than root asks for a change (the
kernel's peer credentials decide, never the request), 5 for an unknown unit,
naming the units the guest knows. `reload` sends SIGHUP to the unit's main
process (Lima's `11-colorterm-environment.sh` reloads sshd after adding
`AcceptEnv`); a unit with no running process refuses with exit status 1. A service that exits is started again under
its `Restart=` policy, with a backoff from 1 to 30 seconds.

<!-- source: internal/appliance/dockerhost/systemctl.go -- verbs, Serve, RunShim -->
<!-- source: internal/appliance/dockerhost/supervise.go -- Supervisor, service.supervise -->
<!-- source: internal/appliance/dockerhost/unitfile.go -- readUnitFile, expandSpecifiers -->

## Docker under the supervisor

The image carries Docker's official static release, unpacked under
`/usr/lib/ze-docker-host/docker/` and linked into `/usr/bin`, and a static
legacy `iptables` (`xtables-legacy-multi`, linked as `iptables`,
`ip6tables` and their `-save` and `-restore` forms). The supervisor knows two
units besides `sshd.service`, so Colima's `systemctl start docker.service`
and `restart containerd.service` work with no unit file:

| Unit | Command |
|------|---------|
| `containerd.service` | `containerd --config /etc/containerd/config.toml` |
| `docker.service` | `dockerd --host=unix:///var/run/docker.sock --group=docker --containerd=/run/containerd/containerd.sock`, plus `--host-gateway-ip=<ip>` when Colima's drop-in names one |

dockerd's argv is fixed. Colima's drop-in
`/etc/systemd/system/docker.service.d/docker.conf` says `-H fd://`, which
assumes systemd socket activation; the supervisor reads only
`--host-gateway-ip` from it, when the unit starts, and refuses a value that
is not an IP address. Every other daemon setting reaches dockerd through
`/etc/docker/daemon.json`, which dockerd reads itself. Both units restart
whenever they exit.

The firewall backend is dockerd's default, iptables. Docker's static build
links the `nft`-executing variant of its nftables backend (moby
`daemon/libnetwork/internal/nftables/nft_exec_linux.go`, build tag
`static_build`, which runs `exec.LookPath("nft")`), so the nftables backend
would need an `nft` binary and its libraries; legacy `iptables` builds with
none, and the runtime kernel carries the legacy tables Docker uses.

The boot writes `/etc/containerd/config.toml` (`version = 2`, containerd's
defaults) when it is absent. Colima backs that file up with `sudo cp` before
it writes its own, and the copy needs a file to copy.

<!-- source: internal/appliance/dockerhost/docker.go -- DockerUnits, resolveDockerd, seedContainerdConfig -->

## The pin file

`gokrazy/docker-host/binaries.json` is the only home of the third-party
versions and sums: the builder container image (by digest), Docker's static
release per arch, and the OpenSSH, sudo and iptables source tarballs. le's
`setup docker-host` area (`internal/le/setup/dockerhost`) loads it with
`LoadPins`, which refuses an unknown key, a sum that is not 64 lowercase hex
digits, a download for an arch Ze does not build, and a builder not pinned by
digest. `Fetcher.Fetch` downloads into `~/.cache/ze/docker-host/downloads/`
and checks the sha256 on the download and again on every reuse of the cached
copy; a mismatch refuses naming the file, both sums and the pin file, and
leaves nothing in the cache. `ImageKey` digests the builder, the arch's Docker
pin, every source pin, the runtime kernel key and the guest source digest, so
the same inputs always give the same key and a change to any gives a new one.

<!-- source: internal/le/setup/dockerhost/pins.go -- LoadPins, ImageKey -->
<!-- source: internal/le/setup/dockerhost/fetch.go -- Fetcher.Fetch -->

## Building the image: `./le setup docker-host build`

`./le setup docker-host build [arch <amd64|arm64>] [size <GiB>] [pins <file>]`
writes the raw image to
`~/.cache/ze/docker-host/<arch>/<key>/docker-host.img` and prints its path.
The key is `ImageKey` over the pins, the runtime kernel cache entry's name and
a source digest of `cmd/ze-docker-host`, `internal/appliance/dockerhost`,
`gokrazy/docker-host` (the extra files tar excepted), the arch's busybox tar,
the builder recipe and the size. When the entry holds the image, the build
answers it and runs nothing. Before anything else it resolves the arch's
runtime kernel (`dockerHostRuntimeKernel`, through
`appliance.ImageRuntimeKernelTree`): from the cache, else a native build on a
host of that arch. On a host of the other arch with no cache entry, it refuses
and names this command on a host of that arch, or the same command for this
host's arch, never a bare kernel build, which would fill the other host's
cache and leave this one refused. Otherwise, in this order:
<!-- source: internal/le/setup/dockerhost/build.go -- dockerHostRuntimeKernel -->
<!-- source: internal/appliance/runtimekernel.go -- ImageRuntimeKernelTree -->

1. every pinned input is fetched and checked against its sha256, so a
   mismatch refuses before anything is written;
2. OpenSSH, sudo and legacy iptables are built as static musl binaries in the
   pinned Alpine builder container (`staticRecipe`), cached under
   `<arch>/static-<key>/`. The container runs on the daemon the `docker` CLI
   reaches by its own environment (`DOCKER_HOST` or the current context): it
   is a build tool, not a lab, so the lab endpoint rules do not choose it. It
   is named `ze-docker-host-build-<pid>` and removed by name afterwards;
3. `extrafiles_<arch>.tar` is written into the instance directory: busybox
   from ze-serial-shell's tar, the static binaries (sudo `04755`) and
   Docker's binaries under `/usr/lib/ze-docker-host/`, every member owned by
   root;
4. gok builds the image through `le build gokrazy`'s path (prepared instance,
   the arch's runtime kernel, `--target_storage_bytes` of the size, 16 GiB by
   default, 2 to 1024 accepted);
5. `/perm`, partition 4 of the image's GPT, is formatted ext4 when it carries
   no superblock: gok formats it only where `mkfs.ext4` is on its PATH, which
   a Mac does not have. The offset comes from the partition table.

`build`, `start` and `stop` take an exclusive lock on
`~/.cache/ze/docker-host/setup.lock`; a second run refuses at once naming the
holder's pid.

<!-- source: internal/le/setup/dockerhost/build.go -- builder.build, sourceDigest -->
<!-- source: internal/le/setup/dockerhost/static.go -- staticRecipe, runBuilderContainer -->
<!-- source: internal/le/setup/dockerhost/extrafiles.go -- writeExtraFiles -->
<!-- source: internal/le/setup/dockerhost/perm.go -- formatPerm, permPartition -->
<!-- source: internal/le/setup/dockerhost/lock.go -- takeLock -->

## The `ze` Colima profile: `start`, `stop`, `check`

On macOS, `./le setup docker-host start` builds the host arch's image when
needed and boots it as the Colima profile `ze`:
`colima start --profile ze --vm-type vz --arch <aarch64|x86_64> --runtime docker
--disk-image <image> --force-disk-image --mount-type virtiofs --cpus 4
--memory 8 --activate=false`, plus `--vz-rosetta` on arm64. `--activate=false`
leaves the operator's current Docker context alone. The Docker socket is
`~/.colima/ze/docker.sock`. The image key and path the profile was started
with are recorded in `~/.cache/ze/docker-host/colima-ze.started`. With the
same key and the profile running, `start` does nothing; with a new key it runs
`colima stop --profile ze`, `colima delete --profile ze --force`, then the
start above, because Colima keeps an existing disk and would ignore a new
image. Every colima argv is built by `colimaArgv`, which always passes
`--profile ze`: setup never names Colima's `default` profile.

`stop` stops the profile when it runs. `check` reports colima, limactl, docker
and `mkfs.ext4`, the recorded image, whether the profile runs and whether its
socket exists, and exits 1 naming the fix for each failure.

<!-- source: internal/le/setup/dockerhost/colima.go -- colimaArgv, colimaStartArgv, colimaHost.start -->
<!-- source: internal/le/setup/dockerhost/actions.go -- checkRows -->

## Kernel

The runtime profile carries what a Lima guest needs beyond the Docker set:
`VSOCKETS` and `VIRTIO_VSOCKETS` (guest agent), `INET_UDP_DIAG` (port
listing), `VIRTIO_FS` (Colima mounts), `VHOST_NET` (VPP, which needs
`MACVTAP=y` so that `TAP` does not cap it at `m`), `BINFMT_MISC` (Rosetta) and
`VIRTIO_CONSOLE` (the vz console). Each is in `gokrazy/kernel/runtime.config`
with its reason, and in `runtime.require`.
