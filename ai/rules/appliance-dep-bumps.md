# Appliance Dependency Bumps

**When:** a Dependabot alert fires on a `go.mod` under `gokrazy/modcache/`, or you must
bump the vendored gokrazy init.

The alert is almost always a stale *vendored upstream manifest*, not your real
dependency graph. Follow this runbook.

## Why this happens

The appliance is built by `gok` (`cmd/ze-gok`, wrapping `github.com/gokrazy/tools`).
`gok` compiles every appliance package with `go build -mod=mod` and fetches with
`go get` (`vendor/github.com/gokrazy/tools/packer/gotool.go`). It has **zero vendor
support** — a `vendor/` tree in a builddir is ignored. So the build resolves through a
**checked-in module cache** (`gokrazy/modcache/`, `GOMODCACHE` set by `cmd/ze-gok/main.go`).

`gokrazy/modcache/.gitignore` ignores everything except the gokrazy init source
(`github.com/gokrazy/gokrazy@*/**`). That committed source includes upstream's own
`go.mod`, and GitHub's dependency graph scans **every** `go.mod` in the repo as a
manifest. When upstream's `go.mod` names a version with a later advisory, the alert
fires on that file even though the image never builds the vulnerable version (the
builddir modules pin the fix and MVS takes the max).

Do NOT try to convert to `go mod vendor` — `gok` can't consume it. Do NOT hand-edit
modcache go.sum hashes.

## The fix: bump the vendored init to an upstream commit that carries the fix

1. **Find a fixed upstream version.** Fetch the candidate `.mod` from the proxy
   (`https://proxy.golang.org/github.com/gokrazy/gokrazy/@v/<version>.mod` or `@latest`)
   and confirm it `require`s the fixed dependency version. Only then bump.
2. **Bump the version string in the 7 builddir modules** under
   `gokrazy/ze/builddir/`: the `require` in `gokrazy` + `cmd/{dhcp,ntp,heartbeat,randomd}`, <!-- doc-links: ignore (cmd/{dhcp,ntp,heartbeat,randomd} are gokrazy submodules under gokrazy/ze/builddir/github.com/gokrazy/gokrazy/, not top-level cmd/) -->
   and the `replace` RHS in `serial-busybox` + `rtr7/kernel`.
3. **Remove any now-false workaround pin/comment** (e.g. an explicit `x/net` pin added
   because "upstream pins the old version"). Verify it is safe: `go list -m <dep>` in each
   builddir must still resolve `>=` the fixed version via the new upstream `require`.
4. **Regenerate the go.sums cleanly.** Delete the affected builddir `go.sum` files
   (filesystem `rm`, never `git rm`), then `make ze-gokrazy-deps` (runs
   `go mod download all` per builddir; the deleted sums regenerate from the new build
   list, pruning the old version string). Do NOT hand-edit hashes.
5. **Re-vendor + prune.** `ze-gokrazy-deps` extracts the new version's source under
   `gokrazy/modcache/github.com/gokrazy/gokrazy@<new>/` (auto-whitelisted by the `@*`
   glob). `rm -rf` the old `@<old>` directory. Confirm the working tree: old tracked
   files deleted, new source untracked, nothing unexpected (no docs/website, no binaries).
6. **Refresh coupling.** `git grep <old-version-string>` — update any doc/spec that
   referenced the old modcache path (e.g. `plan/spec-kernel-lockdown-hardening.md`).
7. **Verify (BLOCKING).** `grep -r <old-version>` is empty; the new committed
   `go.mod` names the fixed dependency version; `make ze-gokrazy` builds; and the
   appliance boots + supervises in QEMU (`test/appliance/serial-login.ci` and, for the
   L2TP path, `ze-deployment-gokrazy-l2tp-ppp-test`). The image *build* alone is not
   sufficient — an init bump can regress boot; run the QEMU appliance proof.

## Git safety

The re-vendor deletes ~60 tracked files and adds ~60 new ones. Never use bare
`git rm`/`git add` — stage the whole change through the commit-helper script at
closure so the deletion and addition land in one commit.

## Cache permissions

Anything that downloads into `gokrazy/modcache/` MUST carry `-modcacherw`
(`GOFLAGS=-modcacherw`): go's default read-only cache permissions (dirs `r-x`) make git
unable to delete or overwrite modcache files on later checkouts and rebases (a
`git pull --rebase` across the 2026-07 init bump wedged exactly this way).
`make ze-gokrazy-deps` (`mk/gokrazy.mk`), `ze appliance build`
(`ensureModcacheRW`, `internal/appliance/cmd_build.go`), and `ze-gok`
(`cmd/ze-gok/main.go`) all set it; keep the flag when running `go mod download` by
hand. A cache written before the flag existed needs a one-time
`chmod -R u+w gokrazy/modcache`.

## Do not just dismiss

Dismissing the alert leaves the stale manifest; a future advisory below the pin will
re-fire on the same file. Bumping removes the manifest at the source.
