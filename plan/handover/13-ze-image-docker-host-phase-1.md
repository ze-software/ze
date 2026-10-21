# Handover 13: ze-image-docker-host, Phase 1 continuation

Spec: `plan/spec-ze-image-docker-host.md` (Status in-progress, Phase 1/6).
Written 2026-10-21 by the Phase 1 agent at its 100-call budget. Phase 1 is NOT
finished: the code and its unit tests are committed, the AC-1 proof is not.

## Committed in this chunk

| File | What it holds |
|------|---------------|
| `gokrazy/kernel/runtime.config`, `runtime.require` | AC-4 options with reasons: VSOCKETS, VIRTIO_VSOCKETS, INET_UDP_DIAG, VIRTIO_FS, VHOST_NET, BINFMT_MISC, VIRTIO_CONSOLE, plus MACVTAP=y (see Finding 1) |
| `internal/appliance/instance/prepare.go` | `Options.Instance` (required, one directory name, `checkInstanceName`); `Name` stays the appliance's instance |
| `internal/appliance/kernelargs.go` | appliance build passes `instance.Name` |
| `internal/le/build/gokrazy/gokrazy.go` | `instanceOf` takes gok's `-i`/`--instance`; refuses a parent dir with no instance |
| `internal/le/build/gosum/gokrazygosum.go` | `inInstanceBuilddir`: every `gokrazy/<instance>/builddir/` is judged |
| `gokrazy/docker-host/config.json`, `builddir/` | second instance: `cmd/ze-docker-host`, randomd, dhcp; no baked machine id; `console=hvc0`; `ExtraFilePaths {"/": "extrafiles"}` (resolved as `extrafiles_<arch>.tar` in the instance dir, gitignored) |
| `cmd/ze-docker-host/main.go` | argv[0] `systemctl` runs the shim, else `RunGuest` |
| `internal/appliance/dockerhost/` | `cidata.go` (ParseCidata + validation), `boot.go` (Boot over a root, System interface, BootScript), `accounts.go`, `supervise.go`, `unitfile.go`, `systemctl.go` (verbs table, Serve, RunShim), `peeruid_{linux,darwin}.go`, `guest_linux.go` (RunGuest, bin tmpfs + busybox links, sshd config and host key, Linux System, FindCidata by ISO 9660 volume id) |
| `docs/architecture/appliance/docker-host.md` (new), `gokrazy-build-pins.md`, `README.md` | pages for the above |

Tests green (`go test -race`): `TestPrepareTakesTheInstanceName`,
`TestCidataParsesLimaFiles`, `TestCidataRefusesUnsafeValues` (boundaries),
`TestDockerHostBootSequenceOverATempRoot`, `TestSystemctlShimLimaAndColimaVerbs`,
`TestSystemctlRefusesAChangeFromANonAdmin`, `TestEveryInstanceBuilddirIsJudged`,
plus the existing `internal/appliance/instance`, `internal/le/build/gokrazy`,
`internal/le/build/gosum` packages. Red was a build failure (symbols undefined)
before the code. `GOOS=linux GOARCH=arm64 go vet` clean on the guest packages.

## Left for the continuation (Phase 1 is not done until all of these are)

1. arm64 runtime kernel build with the new options:
   `./ze-host appliance kernel --target runtime --arch arm64` (docker builder).
   First run refused `CONFIG_VHOST_NET did not resolve to =y`; the MACVTAP fix
   was in flight when this was written. Confirm the emitted config shows every
   AC-4 symbol `=y`; amd64 build too (AC-4).
2. Static OpenSSH 9.9p2 (`--without-openssl --without-zlib`,
   `--libexecdir=/usr/lib/ze-docker-host`, privsep `/var/empty` user `sshd`,
   `LDFLAGS=-static`; targets sshd sshd-session ssh-keygen sftp-server) and sudo
   1.9.16p2 (`--without-pam --disable-shared --enable-static-sudoers
   --with-noexec=no --disable-zlib`) built in `alpine:3.22` pinned by digest;
   Result of that run: sshd, sshd-session (and ssh-keygen, sftp-server) came
   out static aarch64; **sudo came out a dynamically linked PIE against
   `/lib/ld-musl-aarch64.so.1`** although `LDFLAGS=-static` was given (libtool
   drops it). Rebuild sudo fully static (for example `CC="gcc -static"
   --disable-pie`, then `file` must say "statically linked") before it enters
   the image. Record the source sha256 and builder digest for Phase 2's
   `binaries.json`.
3. Assemble `gokrazy/docker-host/extrafiles_arm64.tar`:
   `usr/lib/ze-docker-host/{busybox,sshd,sshd-session,ssh-keygen,sftp-server,sudo}`
   (sudo mode 04755, owner 0), busybox from
   `cmd/ze-serial-shell/_gokrazy/extrafiles_arm64.tar`, and directory entries
   `bin/ sbin/ usr/bin/ usr/sbin/ home/ mnt/lima-cidata/ usr/local/bin/`
   (the program mounts tmpfs or overlays on them; they must exist in squashfs).
4. Build the image: `./le build gokrazy --parent_dir gokrazy -i docker-host
   overwrite --full <scratch>/docker-host-arm64.img` (gok `--target_storage_bytes`
   as needed).
5. AC-1: `LIMA_HOME=<scratch>/lima limactl start` on a template with the raw
   image under `images:`, `vmType: vz`, `containerd: {system: false, user:
   false}`. Paste: `limactl list` Running (not Degraded), `limactl shell <i> cat
   /run/lima-boot-done` == meta-data instance id, `uname -r` ending `-ze`, the
   serial log lines (systemd-boot, kernel banner, `LIMA ... Exiting with code`,
   every `WARNING: Failed to execute` checked against R-2 and A-11). Then
   `limactl delete --force`. Never touch `~/.colima/default`.
6. Write A-1, A-2, A-3, A-7, A-8, A-9 outcomes into the spec's Assumptions
   table. A-1 failing stops the spec (design gate re-opens).
7. Gates owed, not run: `./le go lint run` (the write hook reported lint issues
   on `guest_linux.go`, `systemctl.go`, `supervise.go`; read them),
   `./le test unit all`, `./le build gosum`.

## Findings

1. `CONFIG_VHOST_NET` cannot be `=y` while `TAP=m`: Kconfig "depends on TAP ||
   !TAP", and arm64 defconfig sets `MACVTAP=m`, which selects `TAP=m`.
   `CONFIG_MACVTAP=y` was added with that reason. This also affects
   `plan/spec-lab-kernel-features-in-setup.md` AC-4.
2. gokrazy's extra files tar keeps the tar header mode as the squashfs inode
   mode (`packer.go` FileInfo.Mode from `header.Mode`; squashfs `uint16(mode)`),
   so a 04755 sudo in the tar should stay setuid: A-9 is likely to hold, unproven
   until the guest shows `ls -l /usr/lib/ze-docker-host/sudo`.

## Do not assume

- A restarted `ze-docker-host` (gokrazy restarts an exited program) skips the
  boot sequence and boot.sh and serves the supervisor only; services the first
  instance started are orphaned and reported inactive. Acceptable for AC-1; a
  Phase 2 decision if dockerd supervision needs it.
- `TestSystemctlShimLimaAndColimaVerbs` covers Lima's verbs and the AC-11 exit
  codes on the guest-agent unit; containerd and docker units are Phase 2.
- The docker gid is 1000; a Lima uid of 1000 makes docker the user's primary
  group (`groupLines`).
