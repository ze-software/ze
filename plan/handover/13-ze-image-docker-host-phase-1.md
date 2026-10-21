# Handover 13: ze-image-docker-host, Phase 1 done, Phase 2 next

Spec: `plan/spec-ze-image-docker-host.md` (Status in-progress). Rewritten
2026-10-21 by the third Phase 1 agent. Phase 1 (AC-1) is proven and
committed; the next agent starts Phase 2.

## Phase 1 commits

| SHA | What |
|-----|------|
| 81fab69967 | Phase 1 code: docker-host gokrazy instance, `cmd/ze-docker-host`, `internal/appliance/dockerhost/`, kernel options |
| fcfb8cbd73 | `/etc` tmpfs over the read-only squashfs `/etc` (A-3 fallback), `/perm/home` 0755, systemctl `reload` (SIGHUP), busybox applet links, exit 125 on a half-applied boot, lint fixes |
| 72fc76c29b | `ze-docker-host: kernel <proc/version>` first console line; spec AC-1 evidence, A-7/A-8/A-9 outcomes, AC-1 text corrected |

AC-1 evidence is pasted in the spec under Implementation Steps, Phase 1.
Assumptions A-1, A-2, A-7, A-8, A-9 confirmed; A-3 broken with the fallback
taken. The Lima instance `zp` and `~/.cache/ze/lp-e1f0` are deleted.

## How the proof was run (reproducible on this Mac only)

Scripts live in the session scratch of e1f02c94 and are machine-local:
`rebuild-boot.sh` (`GOARCH=arm64 ./le build gokrazy --parent_dir gokrazy -i
docker-host overwrite --full <img> --target_storage_bytes 2147483648`, then
homebrew `mkfs.ext4 -F -E offset=1157627904 <img> 966639` for `/perm`, then
`limactl start --tty=false --name zp --timeout 8m <template>`),
`assemble-extrafiles.sh` (builds `gokrazy/docker-host/extrafiles_arm64.tar`:
busybox, sshd, sshd-session, ssh-keygen, sftp-server, sudo 04755 root),
`static-build/sudo.sh` (sudo needs `make LDFLAGS=-all-static`). Template:
`vmType: vz`, raw image under `images:`, `containerd: {system: false, user:
false}`, `mounts: []`. `LIMA_HOME` must be short (UNIX_PATH_MAX 104), so it
went under `~/.cache/ze/`.

## Open items for Phase 2 (and later), each owed

1. **Boot time: busybox `modprobe` costs 48 to 86 s.** Lima's `00-modprobe.sh`
   calls it 21 times; each call parses `modules.dep` and `modules.alias`
   (`modprobe tun`: 7.8 s, 6 s user CPU), and every one of those modules is in
   `modules.builtin`, so each answers "not found" after the work. The guest
   agent only starts after this, and R-1 gives Colima one minute. Fix in the
   image (for example a modprobe that answers a `modules.builtin` entry at
   once), with a test; never a script edit.
2. **`/perm` is unformatted on darwin.** gok does not run mkfs there; the proof
   ran it by hand. Phase 3: le's `docker-host` build formats it, reading the
   partition offset from the image.
3. **Lima resizes the raw disk** (2 to 4 GiB) and leaves the backup GPT header
   mid-disk; the kernel warns (`GPT:Alternate GPT header not at the end of the
   disk`) and boots. Phase 3 decides whether le sizes the image to the profile.
4. **gokrazy's web server listens on :80** in the guest and Lima tries to
   forward it (`failed to listen tcp 0.0.0.0:80` on the host). Phase 2 decides
   whether the docker-host instance turns it off.
5. `20-rootless-base.sh` fails (R-2): no `/etc/sysctl.d`, busybox `sysctl` has
   no `--system`, no logind. Accepted by AC-1; revisit only if rootless docker
   is ever wanted.
6. Phase 2 proper: pins file and source builds (`binaries.json`, record the
   OpenSSH 9.9p2 and sudo 1.9.16p2 sha256 and the Alpine builder digest),
   dockerd and containerd supervision, seeded containerd config, A-5 read of
   moby's nftables backend; AC-5, AC-11, AC-12. AC-4 amd64 kernel build is
   still unrun.

## Gates owed (not run by this agent: over five minutes)

`./le go lint run` (scoped golangci-lint over dockerhost, cmd/ze-docker-host,
le/build/gokrazy: 0 issues darwin and linux/arm64), `./le test unit all`,
`./le verify worktree`. Commits carried `structural-red-ok`: staticcheck parts
2 and 3 were red in the 12:27Z verify run, before these changes.
