# Appliance Dependency Bumps

**When:** a Dependabot alert fires on a `go.mod` under `gokrazy/modcache/`, or you must bump the vendored gokrazy init.
**Severity:** advisory

## Directives

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
   One of the eight is **untracked**: `gokrazy/ze/builddir/codeberg.org/thomas-mangin/ze/go.sum`
   is gitignored (see `.gitignore`), because that module is only
   `replace ze => <repo root>` and every line of its sum is already in the root
   `go.sum`. Regenerate it like the rest; expect no diff. The other seven are
   tracked locks and DO show a diff.
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

## Module cache hygiene: what may accumulate, and what must never

`gokrazy/modcache/` is a real Go module cache and Go never garbage-collects it.
Two kinds of growth are expected, one is a defect.

**Expected.** Superseded versions after a pin bump (runbook step 5 tells you to
`rm -rf` the old dir; do it, or every bump leaves 15-50 MB behind), and the breadth
of `go mod download all` (`mk/gokrazy.mk`), which is the whole module graph
including test-only deps and their fixtures: `pierrec/lz4` is 75 MB of `testdata/`,
`klauspost/compress` 46 MB. A second Go toolchain also lands here
(`golang.org/toolchain@...`, ~310 MB with its zip) whenever a builddir `go`
directive is newer than the host toolchain and `GOTOOLCHAIN=auto`.

**A defect.** Either of these means a build resolved over the network instead of
through the pins, and the version it built is not the version this repo chose:

| What you find | What it means |
|---------------|---------------|
| `codeberg.org/thomas-mangin/ze@v0.0.0-<date>-<hash>` | ze was fetched from the proxy. The builddir replaces ze with the working tree, so a build that reaches the proxy for ze did not read the builddir, and it compiled a *pushed commit* rather than your tree |
| A version of a builddir-pinned module that is not the pinned one | `gok` fell back to `go get` and took whatever upstream had. For `github.com/rtr7/kernel` that is the appliance's **kernel** |

Both were live between 2026-07-18 and 2026-07-22: the derived (hugepage) parent
handed `gok` an instance with no `builddir`, so every pin was discarded
(`internal/appliance/kernelargs.go` `materializeDerivedParent`, against
`vendor/github.com/gokrazy/tools/packer/gotool.go` `getPkg`/`getIncomplete`). That
route is closed, and `TestMaterializeDerivedParent` gates it. A reappearance is a
regression in whatever new path prepares an instance: find that path, do not just
delete the directory.

**Never `rm -rf gokrazy/modcache`.** 60 tracked files live inside it (the gokrazy
init source, whitelisted by `gokrazy/modcache/.gitignore`). Delete named `@version`
directories plus their `cache/download/<module>/@v/<version>.*` files, and confirm
with `git status --porcelain gokrazy/` that nothing tracked moved.

## Do not just dismiss

Dismissing the alert leaves the stale manifest; a future advisory below the pin will
re-fire on the same file. Bumping removes the manifest at the source.

## Proactive review cadence (builddir pins)

The appliance builddir modules (`gokrazy/ze/builddir/`) and the checked-in module
cache (`gokrazy/modcache/`) are **excluded from Dependabot** (`.github/dependabot.yml`)
on purpose: an automated PR would fight the hand-pin (the MVS `max` is chosen
deliberately, and a bot bump reopens the stale-manifest churn described above).
Dependabot stays off; a **proactive review** replaces it: *review*, never an
automated bump.

**Cadence:** review the builddir pins **once per release cycle, and at minimum
quarterly**, whichever comes first. Each review:

1. For the vendored gokrazy init and `rtr7/kernel`, fetch the latest upstream
   `.mod` from the proxy (as in "The fix" above) and note whether a newer commit
   carries security-relevant fixes.
2. If a fix applies, run the bump runbook above. If not, record the review date so
   the next reviewer knows the pins were checked, not forgotten.
3. Re-confirm the GPLv2 source-offer sign-off below is still current.

The pins move only through the runbook, never through a bot PR.

## GPLv2 source-offer sign-off (rtr7/kernel): UNRESOLVED, flag only

The appliance image ships a GPLv2 Linux kernel: `github.com/rtr7/kernel`
(`gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod:5`, pinned as an indirect
pseudo-version). Distributing a GPLv2 binary obliges the distributor to make the
**corresponding source** available (typically a written offer accompanying the image).

**Status: UNRESOLVED.** No source-offer compliance sign-off is recorded. This note
**flags** the obligation; it does not adjudicate it. That is a licensing/legal call,
out of scope here. Before the image is distributed to third parties, a source-offer
sign-off must be produced and recorded. Re-confirm each review cycle above.

## Root-module pseudo-version pins (no upstream tags)

Separate from the builddir concern: six **root** `go.mod` direct dependencies are
pinned to pseudo-versions (`v0.0.0-<date>-<hash>`) rather than semver tags. This is
**not a defect**. It was verified (2026-07-21, `spec-fixit-supply-chain-hardening`
AC-4) that **none of these upstreams publish any semver tag**: `go list -m -versions`
and `proxy.golang.org/<mod>/@v/list` return an empty version list for every one, and
`@latest` resolves to a pseudo-version. There is nothing to move the pin to.

| Root dep (`go.mod` line) | Pin form | Upstream semver tag? |
|--------------------------|----------|----------------------|
| `github.com/charmbracelet/ssh` (:12) | pseudo-version | none published |
| `github.com/insomniacslk/dhcp` (:15) | pseudo-version | none published |
| `github.com/packetcap/go-pcap` (:19) | pseudo-version | none published |
| `golang.zx2c4.com/wireguard/wgctrl` (:30) | pseudo-version | none published |
| `github.com/gokrazy/tools` (:38) | pseudo-version | none published |
| `github.com/gokrazy/updater` (:39) | pseudo-version | none (hard fork; see `scripts/dev/reapply-updater-fixes.py`) |

Keep the pseudo-versions. Re-check for a first tag when bumping any of these, and
move the pin to a tag the day upstream cuts one. Until then a pseudo-version is the
only available form and is legal. The note exists so a future reviewer does not
"fix" a non-problem or assume the pins were never examined.
