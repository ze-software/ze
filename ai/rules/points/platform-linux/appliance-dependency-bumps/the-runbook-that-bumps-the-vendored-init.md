---
kind: directive
level: MUST
stage:
---
1. **Find a fixed upstream version.** You MUST fetch the candidate `.mod` from the proxy (`https://proxy.golang.org/github.com/gokrazy/gokrazy/@v/<version>.mod` or `@latest`) and confirm it `require`s the fixed dependency version. Only then bump.
2. **You MUST bump the version string in the 7 builddir modules** under `gokrazy/ze/builddir/`: the `require` in `gokrazy` + `cmd/{dhcp,ntp,heartbeat,randomd}`, and the `replace` RHS in `serial-busybox` + `rtr7/kernel`. <!-- doc-links: ignore (cmd/{dhcp,ntp,heartbeat,randomd} are gokrazy submodules under gokrazy/ze/builddir/github.com/gokrazy/gokrazy/, not top-level cmd/) -->
3. **You MUST remove any now-false workaround pin/comment** (e.g. an explicit `x/net` pin added because "upstream pins the old version"). Verify it is safe: `go list -m <dep>` in each builddir MUST still resolve `>=` the fixed version via the new upstream `require`.
4. **You MUST regenerate the go.sums cleanly.** You MUST delete the affected builddir `go.sum` files (filesystem `rm`, never `git rm`), then run `make ze-gokrazy-deps` (runs `go mod download all` per builddir; the deleted sums regenerate from the new build list, pruning the old version string). You MUST NOT hand-edit hashes.
5. **Re-vendor + prune.** `ze-gokrazy-deps` extracts the new version's source under `gokrazy/modcache/github.com/gokrazy/gokrazy@<new>/` (auto-whitelisted by the `@*` glob). You MUST `rm -rf` the old `@<old>` directory. You MUST confirm the working tree: old tracked files deleted, new source untracked, nothing unexpected (no docs/website, no binaries).
6. **Refresh coupling.** You MUST run `git grep <old-version-string>`, and update any doc/spec that referenced the old modcache path (e.g. `plan/spec-kernel-lockdown-hardening.md`).
7. **Verify (BLOCKING).** You MUST confirm `grep -r <old-version>` is empty; the new committed `go.mod` names the fixed dependency version; `make ze-gokrazy` builds; and the appliance **boots in QEMU**. The image *build* alone is not sufficient: an init bump can regress boot.
