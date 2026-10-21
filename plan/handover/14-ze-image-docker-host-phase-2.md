# Handover 14: ze-image-docker-host, Phase 2 code and guest proof done

Spec: `plan/spec-ze-image-docker-host.md` (Status in-progress). Written
2026-10-21 by the Phase 2 agent. Supersedes handover 13 for open items.
Every outcome below is also recorded in the spec under Implementation Steps,
Phase 2, with its evidence.

## Phase 2 commits

| SHA | What |
|-----|------|
| a61dc558a8 | `/lib/modules` tmpfs overlay so busybox modprobe writes its index (R-1): guest agent 101 s to 47 s, then 7 to 10 s |
| 78c7ebcca3 | `Update.NoPassword`: gokrazy's web server on its unix socket only, guest listens on :22 only |
| (this commit's parent) | pins file, `internal/le/setup/dockerhost` (LoadPins, Fetcher, ImageKey), containerd and dockerd built-in units, seeded containerd config, docker and iptables links, page and spec |

## Proven on the arm64 Mac (Lima 2.2.1, vz, throwaway LIMA_HOME, now deleted)

- AC-11: every verb and exit status (0, `active`, `inactive` 3, nosuch 5).
- AC-12 (first half): `/etc/containerd/config.toml` seeded on a fresh boot.
- Through `ssh -L` of `/var/run/docker.sock`: `docker info` kernel `7.2.9-ze`,
  driver `overlayfs`, cgroup 2, root `/perm/var/lib/docker`; `hello-world`
  ran; a busybox container reached example.com (iptables NAT).
- A-5 broken, fallback taken (static legacy iptables 1.8.13). A-4 holds in
  substance; AC-2's "overlay2" must become "overlayfs" (Docker 29 containerd
  image store), Phase 3 corrects the AC text.

## Open items, each owed

1. Phase 3: le assembles `extrafiles_<arch>.tar` from the pins (busybox from
   ze-serial-shell, Docker tarball unpacked to `usr/lib/ze-docker-host/docker/`,
   `xtables-legacy-multi`, OpenSSH, sudo) and runs the builder-container
   source builds itself. The proof used the scratch scripts
   `assemble-extrafiles.sh`, `static-build/{build,sudo,iptables}.sh` in session
   e1f02c94 (machine-local); their recipes are summarized in the spec
   (A-7, A-9, A-5 rows).
2. Phase 3: AC-5's three-run proof through `./le setup docker-host build`
   (area registration is Phase 3), `/perm` formatting on darwin, image sizing,
   AC-12's second half (Colima's backup copy), AC-2 text fix above.
3. amd64: the iptables static build and the image were built for arm64 only.
4. Docker's static tarball sums are trust-on-first-download (2026-10-21):
   download.docker.com publishes none.

## Gates owed (not run: over five minutes)

`./le go lint run` (scoped to `internal/appliance/dockerhost`,
`cmd/ze-docker-host`, `internal/le/setup/dockerhost`), `./le test unit all`,
`./le verify worktree`. Scoped `go test -race` of both packages is green and
`GOOS=linux GOARCH=arm64 go vet` is clean. Commits carry `structural-red-ok`
for staticcheck parts 2 and 3, red in the 12:27Z verify before these changes.
