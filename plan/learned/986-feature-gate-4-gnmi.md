# 986 - feature-gate child 4: gnmi compile-out (dedicated seam, three owned imports)

## Context

Child 4 of the feature-gate umbrella (`plan/spec-feature-gate-0-umbrella.md`): make
the gNMI service compile-out-able via `ze_gnmi` for a smaller binary and attack
surface. gNMI mirrors the ssh dedicated-seam pattern (981), not the lg listener
registry (980), because three couplings the registry's `Service`/`ServiceDeps`
contract cannot express: (1) its constructor needs richer deps (config-tree
accessor, `*api.ConfigSessionManager`, YANG loader, a `*ChangeNotifier`); (2) the
always-on config-reload closure called `gnmiNotifier.NotifyConfigReload()`, a direct
dependency on a gNMI type that had to be severed; (3) gNMI binds once and never
live-migrates, so it has no `Reconfigure` and is not a `ListenerMigrator`.

## Decisions

- **Dedicated `ze_gnmi` seam.** `cmd/ze/hub/gnmi_infra.go` (always-on) declares an
  opaque `gnmiServer` handle (`Stop()` only), a generic `gnmiBuildInputs` struct
  (config tree, store, config path, reload hook; never a `zegnmi` type), nil-able
  `gnmiBuild`/`gnmiReloadNotify` vars, and `setGNMIInfra`. `service_gnmi.go`
  (`//go:build ze_gnmi`) holds the impl + the moved `serveGNMI`/`waitForGNMIBind`;
  `register_gnmi.go` init() installs the hooks. Tag off => hooks nil, gNMI skipped.
- **Reload coupling via a nil-able hook.** The reload closure now calls
  `if gnmiReloadNotify != nil { gnmiReloadNotify() }`; the `*zegnmi.ChangeNotifier`
  lives in a package var inside the gated impl. No always-on file names a gNMI type.

## Core insight (the reusable lesson)

**Count every blank import a feature owns before gating it.** gNMI owns THREE
generated imports, not the usual one or two: the component package (handler/RPC
registration), its config schema (`gnmi/yang`), AND a command-schema sidecar
(`internal/plugins/gnmi-cmd/yang`, the `show gnmi` schema). Gating only the config
schema left `ze-show:gnmi` registered with its handler compiled out -- caught by
`make ze-doc-test` ("schema with no handler"). Two consequences:

- **Manifest is multi-row per feature.** `feature-gates.txt` now allows a feature to
  reuse one tag across owned sidecar packages (two `ze_gnmi` rows:
  `internal/component/gnmi`, `internal/plugins/gnmi-cmd`).
- **Generator gates the direct package too.** `plugin_imports.go` `loadFeatureTags`
  was extended to map `<pkg> -> tag` in addition to `<pkg>/yang -> tag`. The direct
  entry gates RPC/registration side-effect imports; the `/yang` entry gates config or
  command schema. Result: `all_ze_gnmi.go` blank-imports all three under `ze_gnmi`.

## Wiring is single-source (do not hand-edit derived lists)

`feature-gates.txt` is the only manifest edit. `ZE_FEATURES` (Makefile awk),
`TestBuildTags()` (`internal/test/runner`), `featureTags` (generator), and
`DISABLEABLE` (`dep_audit.py load_feature_gates`) all DERIVE from it. Only
`.golangci.yml` build-tags is hand-edited (static YAML); `dep_audit.py --check`
flags its drift. The spec's original "four-place hand-wiring" text was stale and was
corrected.

## Review fixes folded in (critical-review pass)

- **Shutdown parity:** the `WaitForStartupComplete` early-error path (`return 1`) now
  calls `gnmiSrv.Stop()`; the old code relied on deferred `gnmiCancel`/`RegisterGlobal(nil)`
  that the seam removed. `zegnmi.Server.Stop()` is idempotent (guards on `s.stopped`).
- **`buildGNMISessionManager` moved** from always-on `api.go` into gated
  `service_gnmi.go` (gNMI is its only caller); removes dead code in stripped builds.
- **Index guard:** `endpointsToAddrs(servers)[0]` is now length-checked, falling back
  to the `0.0.0.0:9339` default instead of panicking on an empty server list.
- **`testing.Short()` skip** on the binary-build/nm symbol test (it builds and spawns
  `ze`); the Makefile suite passes no `-short`, so full coverage is retained.

## Verification

`make ze-lint-changed` (0 issues); present/absent build-tag tests + `TestGNMIReloadNotify`
under `ze_gnmi` and bare `ze_core`; generator `--check`; `dep_audit.py --check` +
`--selftest`. `ze-stripped` (`ze_core ze_ssh`) links 0 gNMI symbols; `ze`/`ze-appliance`
keep gNMI.

## Files

None recorded.
