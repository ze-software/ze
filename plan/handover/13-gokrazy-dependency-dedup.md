# Handover: deduplicate gokrazy dependency sources

Goal: make Ze's root Go module the single source for gokrazy builder and updater code now that Ze imports gokrazy directly, while preserving offline appliance builds and the existing Make and `ze install appliance` workflows.

## Current evidence

- Root `go.mod:41-44` already requires `github.com/gokrazy/gokapi`, `github.com/gokrazy/internal`, `github.com/gokrazy/tools`, and `github.com/gokrazy/updater`.
- Root `vendor/modules.txt:109-142` vendors the same gokrazy modules under `vendor/github.com/gokrazy/`.
- `cmd/ze/install/appliance/cmd_build.go:18` imports `github.com/gokrazy/tools/gok`, and `cmd_build.go:184-210` runs gok in process with `GOMODCACHE` pointed at `gokrazy/modcache`.
- `mk/gokrazy.mk:49-52`, `mk/gokrazy.mk:56`, `mk/gokrazy.mk:68`, and `mk/gokrazy.mk:104-106` still build and run `bin/gok` from the separate `gokrazy/tools` module.
- `gokrazy/tools/go.mod:5-12` requires the same gokrazy modules, and `gokrazy/tools/vendor/modules.txt:7-41` vendors them a second time.
- `gokrazy/tools/cmd/ze-gok/main.go:3-14` wraps gok only to force a repo-local module cache. That overlaps with `runGokInProcess` in `cmd_build.go:184-210`.
- `cmd/ze/install/appliance/updater/updater.go:6-7` says the local updater copy exists to avoid adding `github.com/gokrazy/updater` to Ze's main module. That rationale is stale because root `go.mod:44` already includes it.
- `cmd/ze/install/appliance/cmd_push.go:24` imports the local copied updater package, not `github.com/gokrazy/updater`.

## Read first

- `ai/rules/before-writing-code.md`
- `ai/rules/never-destroy-work.md`
- `ai/rules/no-sprintf-alloc.md`
- `ai/rules/wiring-completeness.md`
- `ai/rules/documentation.md`
- `ai/rules/lint-gate.md`
- `plan/learned/816-install-7b-vendor-builder.md`
- `plan/learned/817-install-7c-vendor-updater.md`
- `docs/guide/appliance.md`, especially the setup and repo layout sections

## Cleanup plan

1. Inventory current consumers before editing:
   - `github.com/gokrazy/tools/gok`
   - `github.com/gokrazy/tools/cmd/gok`
   - `github.com/gokrazy/updater`
   - `codeberg.org/thomas-mangin/ze/cmd/ze/install/appliance/updater`
   - `bin/gok`
   - `gokrazy/tools`

2. Remove the stale local updater copy only after preserving its local fixes:
   - Preferred route: bump or select a `github.com/gokrazy/updater` revision that has the local fixes from `cmd/ze/install/appliance/updater/updater.go`.
   - Required local fixes to check before switching imports:
     - response bodies are closed after `Do` calls (`cmd/ze/install/appliance/updater/updater.go:100`, `136`, `155`, `217`, `241`);
     - empty requests use `http.NoBody` (`cmd/ze/install/appliance/updater/updater.go:128`, `147`, `209`, `232`);
     - `Supports` avoids manual string loop churn (`cmd/ze/install/appliance/updater/updater.go:69-72`).
   - The currently vendored upstream updater does not have all of those fixes (`vendor/github.com/gokrazy/updater/updater.go:120-124`, `167`, `185`, `248`, `335`). Do not silently regress resource cleanup just to remove a duplicate.
   - Once the chosen upstream updater is acceptable, change `cmd/ze/install/appliance/cmd_push.go` to import `github.com/gokrazy/updater`, then delete `cmd/ze/install/appliance/updater/`.
   - Keep `authTransport`, `protocolError`, `mapUpdaterError`, `doPushFn`, `--testboot`, and `--no-reboot` behavior in `cmd_push.go`; only the updater package source should change.

3. Collapse the separate gok builder module:
   - Keep Ze root module and root `vendor/` as the source of gokrazy tooling.
   - Remove `gokrazy/tools/go.mod`, `gokrazy/tools/go.sum`, `gokrazy/tools/tools.go`, `gokrazy/tools/cmd/ze-gok/`, and `gokrazy/tools/vendor/` after replacement is working.
   - Preserve the public `make bin/gok`, `make ze-gokrazy-deps`, and `make ze-gokrazy` targets unless intentionally replacing the whole old Make appliance workflow.
   - Conservative implementation: make `bin/gok` build from the root module, for example by adding a root tools pin for `github.com/gokrazy/tools/cmd/gok`, running `go mod vendor`, and changing `mk/gokrazy.mk:49-52` to build `bin/gok` from root `vendor/` rather than `gokrazy/tools`.
   - Do not point Make at `ze install appliance build` as a shortcut unless you prove the old Make workflow remains behaviorally equivalent. The Make target also prepares the legacy ZeFS injection path.
   - Update stale comments in `cmd/ze/install/appliance/cmd_build.go:216-218` and `cmd/ze/install/appliance/cmd_build_test.go:35-36` that still describe `ze-gok` working-directory behavior.

4. Keep intentional appliance build artifacts separate from duplicate source code:
   - Keep `gokrazy/ze/config.json`; it defines the appliance package set.
   - Keep `gokrazy/ze/builddir/**/go.mod` and `go.sum`; these pin the appliance package graph for gokrazy init, kernel, serial busybox, and Ze.
   - Keep `gokrazy/modcache/.gitignore` and any intentionally tracked whitelisted source required for offline builds unless `make ze-gokrazy-deps` plus an offline build proves they are no longer needed.
   - Do not delete `gokrazy/modcache` wholesale as part of source deduplication. It is a build cache and offline input, not the same class as `gokrazy/tools/vendor/`.

5. Update documentation and comments after the code shape changes:
   - `docs/guide/appliance.md:38-43` should no longer say the gok build tool is vendored at `gokrazy/tools/vendor/` if that directory is removed.
   - `docs/guide/appliance.md:254-279` repo layout must describe root `vendor/` or the new root tools pin, not `gokrazy/tools/vendor/`.
   - `mk/gokrazy.mk:6-9` comments must match the final offline-build model.
   - Leave `plan/learned/816-*` and `plan/learned/817-*` as historical records unless the project's learned-summary rules require a new follow-up entry.

6. Refresh module metadata only after imports and tool pins are settled:
   - Run `go mod tidy`.
   - Run `go mod vendor` so `vendor/modules.txt` and `vendor/github.com/gokrazy/` match root `go.mod`.
   - Confirm `github.com/gokrazy/updater` is direct if `cmd_push.go` imports it directly.
   - Confirm `github.com/gokrazy/tools/cmd/gok` is present in root vendor if `make bin/gok` builds that command from vendor.

## Acceptance

- No Ze code imports `codeberg.org/thomas-mangin/ze/cmd/ze/install/appliance/updater`.
- `cmd/ze/install/appliance/updater/` is gone, or explicitly retained with a new comment documenting why upstream still cannot replace it. Do not leave the stale `avoid adding external dependencies to ze's main go.mod` rationale.
- `gokrazy/tools/vendor/` is gone.
- `gokrazy/tools/` no longer contains a separate Go module or `ze-gok` wrapper unless a documented, verified consumer still needs it.
- `make bin/gok` still works and builds from root-managed dependencies.
- `make ze-gokrazy-deps` still populates `gokrazy/modcache` for offline appliance system packages.
- `ze install appliance build` still uses in-process gok via `cmd_build.go` and still honors repo-local `gokrazy/modcache`.
- `ze install appliance push` still streams root image, switches or testboots, and reboots according to `--testboot` and `--no-reboot`.
- Appliance docs no longer mention removed paths or duplicate vendoring.

## Verification

Run these after the cleanup:

1. `go test ./cmd/ze/install/appliance`
2. `go test ./internal/component/gokrazy ./internal/core/gokrazyutil`
3. `make bin/gok`
4. `bin/ze-test install --all`
5. `make ze-lint-changed`
6. `make ze-doc-test` if `docs/guide/appliance.md` changed

If appliance build prerequisites are available, also run the smallest real image-build check that exercises the final `bin/gok` path. If they are not available, record the missing prerequisite and the exact command that remains unrun; do not claim full image-build coverage from unit tests alone.
