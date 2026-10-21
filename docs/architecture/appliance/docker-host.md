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
generates one per disk on the first boot. The kernel command line names both
consoles, `hvc0` (vz, virtio console) and `ttyS0` (QEMU), so either boot logs.
The third-party binaries (busybox, a static musl OpenSSH built without OpenSSL,
a static musl sudo) reach the image as the extra files tar
`extrafiles_<arch>.tar`, under `/usr/lib/ze-docker-host/`. The tar is assembled
at build time and never committed.

<!-- source: cmd/ze-docker-host/main.go -- main -->

## What the program does at boot

`ze-docker-host` does cloud-init's part of the Lima guest contract in Go, then
runs Lima's own `boot.sh` unchanged under busybox `sh`:

1. remounts gokrazy's 1 MB tmpfs `/etc` at 64 MB, because Lima and Colima write
   configs and unit files there;
2. finds the disk whose ISO 9660 volume id is `cidata` (there is no udev, so no
   `/dev/disk/by-label`) and mounts it read-only at `/mnt/lima-cidata`, the
   path Lima's scripts read;
3. parses `lima.env`, `meta-data` and `user-data`, refusing a user, host name,
   home, key or mount point that would split a passwd or sudoers line or leave
   its directory;
4. keeps the machine id in `/perm/machine-id` and copies it to
   `/etc/machine-id`;
5. binds `/perm/home` on `/home`, and overlays a tmpfs on the read-only
   `/usr/local`, where Lima installs its guest agent;
6. writes `passwd`, `group` (a `docker` group, gid 1000, holding the Lima
   user), `shadow` (every account locked), `sudoers` and
   `sudoers.d/90-lima-users`, and the user's `authorized_keys`;
7. sets the host name and creates `/run/user/<uid>/systemd/private`, the file
   `boot.sh` waits for before it runs user provisioning scripts;
8. covers `/bin`, `/sbin`, `/usr/bin` and `/usr/sbin` with a tmpfs holding
   links to busybox's applets, sudo, sshd, ssh-keygen and the `systemctl`
   shim, plus `/bin/bash` as busybox `sh`;
9. writes `sshd_config` (public keys only, no root login, stream-local
   forwarding for the Docker socket) and a persistent ed25519 host key;
10. starts the supervisor, sshd, and Lima's `boot.sh`, which writes
    `/run/lima-boot-done` itself.

<!-- source: internal/appliance/dockerhost/boot.go -- Boot -->
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
`try-restart` and `is-active` with systemctl's exit statuses: 3 for a unit
that is not active, 4 when a caller other than root asks for a change (the
kernel's peer credentials decide, never the request), 5 for an unknown unit,
naming the units the guest knows. A service that exits is started again under
its `Restart=` policy, with a backoff from 1 to 30 seconds.

<!-- source: internal/appliance/dockerhost/systemctl.go -- verbs, Serve, RunShim -->
<!-- source: internal/appliance/dockerhost/supervise.go -- Supervisor, service.supervise -->
<!-- source: internal/appliance/dockerhost/unitfile.go -- readUnitFile, expandSpecifiers -->

## Kernel

The runtime profile carries what a Lima guest needs beyond the Docker set:
`VSOCKETS` and `VIRTIO_VSOCKETS` (guest agent), `INET_UDP_DIAG` (port
listing), `VIRTIO_FS` (Colima mounts), `VHOST_NET` (VPP, which needs
`MACVTAP=y` so that `TAP` does not cap it at `m`), `BINFMT_MISC` (Rosetta) and
`VIRTIO_CONSOLE` (the vz console). Each is in `gokrazy/kernel/runtime.config`
with its reason, and in `runtime.require`.
