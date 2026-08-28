---
kind: directive
level: MUST
stage:
---
1. **Find a fixed upstream version.** You MUST fetch the candidate `.mod` from the proxy (`https://proxy.golang.org/github.com/gokrazy/gokrazy/@v/<version>.mod` or `@latest`) and confirm it `require`s the fixed dependency version. Only then bump.
2. **You MUST bump the version string in the 7 builddir modules** under `gokrazy/ze/builddir/`: the `require` in `gokrazy` + `cmd/{dhcp,ntp,heartbeat,randomd}`, and the `replace` RHS in `serial-busybox` + `rtr7/kernel`. <!-- doc-links: ignore (cmd/{dhcp,ntp,heartbeat,randomd} are gokrazy submodules under gokrazy/ze/builddir/github.com/gokrazy/gokrazy/, not top-level cmd/) -->
3. **You MUST remove any now-false workaround pin/comment** (e.g. an explicit `x/net` pin added because "upstream pins the old version"). Verify it is safe: `go list -m <dep>` in each builddir MUST still resolve `>=` the fixed version via the new upstream `require`.
4. **You MUST regenerate the go.sums cleanly.** Delete the affected builddir `go.sum` files (filesystem removal, never `git rm`), then run `go mod download all` in each affected builddir. The sums regenerate from the new build list and prune the old version string. You MUST NOT hand-edit hashes.
5. **Re-vendor and prune.** The module download extracts the new version under `gokrazy/modcache/github.com/gokrazy/gokrazy@<new>/`. Remove the old `@<old>` directory. Confirm the working tree holds only the expected old-file deletions and new source.
6. **Refresh coupling.** Search for the old version string and update every document or spec that names the old module-cache path.
7. **Verify (BLOCKING).** Confirm the old version string is absent, the new committed `go.mod` names the fixed dependency, `ze appliance build` succeeds, and `./le deployment gokrazy-l2tp-ppp-test` boots the appliance. An image build alone is insufficient.
