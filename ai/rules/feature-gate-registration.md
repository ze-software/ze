# Feature-Gate Registration (compile-out-able features)

How to add or change a **compile-out-able feature**: a subsystem that can be
dropped from the `ze` binary at build time via a `//go:build ze_<feature>` tag,
for a smaller binary and a smaller attack surface (looking-glass `ze_lg`, ssh
`ze_ssh`, web `ze_web`, gNMI `ze_gnmi`, MCP `ze_mcp`, REST API `ze_rest`, gRPC
API `ze_grpc`, Prometheus exporter `ze_telemetry`, ...).

Read this before touching `feature-gates.txt`, `cmd/ze/hub/service_registry.go`,
a `register_<x>.go` / `service_<x>.go` file, an `*_infra.go` seam, the
`ZE_FEATURES` Makefile var, `.golangci.yml` build-tags, `TestBuildTags`,
`scripts/codegen/plugin_imports.go` `featureTags`, or `scripts/dev/dep_audit.py`
`DISABLEABLE`.

Companion rules: `ai/rules/module-tiers.md` (disable-ability + the gate),
`ai/rules/plugin-self-containment.md` (the delete-the-folder invariant),
`ai/rules/registration-dispatch.md` (command dispatch, not feature gating).

## The one invariant

A feature is compile-out-able **only when nothing always-on (untagged, non-test)
imports its package** for ANY reason: lifecycle OR a borrowed helper. Always-on
code reaches it ONLY through build-tag-gated registration. A single direct
`import` from untagged code pins the package into every binary and defeats the
compile-out. `scripts/dev/dep_audit.py --check` (run by `make ze-verify`, target
`ze-tier-check`) fails on any such importer.

If always-on code needs a non-lifecycle helper the feature happens to export
(e.g. web exported cert generation to the installer), **extract that helper to an
always-on home FIRST** (`internal/core/*` leaf), then gate the feature. This is
"extract-then-gate"; the registry-ize is the easy half.

## `feature-gates.txt` is the single source of truth

The repo-root file `feature-gates.txt` declares every gated package. Most
features need one line; features with owned sidecar packages use the same tag on
additional lines:

```
ze_gnmi  internal/component/gnmi
ze_gnmi  internal/plugins/gnmi-cmd
```

`<build-tag>` then `<gated-package>`. Tags are `ze_<feature>` by convention.
The generator gates both `<pkg>` and `<pkg>/yang` when those packages are
discovered. The direct package covers RPC/registration side effects, and the
YANG package covers config or command schema. **Every other consumer
DERIVES from this file**. Do NOT hand-edit a parallel list:

| Consumer | Derives | Mechanism |
|----------|---------|-----------|
| `Makefile` `ZE_FEATURES` | default-on tags for `ze` / `ze-appliance` | `$(shell awk ...)` |
| `internal/test/runner` `TestBuildTags` | tags for the functional-test `ze` | reads the file |
| `scripts/codegen/plugin_imports.go` `featureTags` | gates `<pkg>` and `<pkg>/yang` into `all_<tag>.go` | `loadFeatureTags` |
| `scripts/dev/dep_audit.py` `DISABLEABLE` | no always-on import of `<pkg>` | `load_feature_gates` |

The ONE consumer that cannot self-derive is `.golangci.yml` `build-tags` (static
YAML). It must equal `ze_core` + every gate tag; `dep_audit.py --check` fails on
drift and tells you exactly which tag to add.

## Procedure: add a feature gate

1. **Extract first.** Search for always-on importers of the feature package
   (`<module>/internal/component/<x>`). Move every non-lifecycle helper they use
   to an always-on `internal/core/*` leaf. Re-check until only gated construction
   remains.
2. **Pick the shape** (see below): construction registry, or a seam.
3. **Add lines** to `feature-gates.txt` for every owned package that must vanish:
   the main package (`ze_<x> internal/component/<x>`) plus sidecars such as
   command-schema packages under `internal/plugins/<x>-cmd`.
4. **Create the gated files** for your shape (`service_<x>.go` + `register_<x>.go`,
   or an `*_infra.go` seam + gated registration). All carry `//go:build ze_<x>`.
   Feature-only helpers live INSIDE a gated file, or a no-feature build flags them
   U1000-unused.
5. **Add the tag to `.golangci.yml` `build-tags`** (the one non-deriving consumer).
6. `make generate` (emits `all_ze_<x>.go`), then `make ze-verify-changed`.
7. Write present/absent build-tag tests:
   `cmd/ze/hub/build_tag_<x>_present_test.go` (`//go:build ze_<x>`) and
   `_absent_test.go` (`//go:build !ze_<x>`); an absent test asserts via
   `go tool nm` that zero feature symbols are linked.

That is the whole list. Step 3 is the only manifest declaration point; step 5 is
the only static non-deriving consumer. The Makefile, the runner, the generator,
and dep_audit all follow from step 3.

## Two registration shapes

**Listener service (default — looking-glass, web).** The feature plugs into the
construction registry (`cmd/ze/hub/service_registry.go`). A gated `service_<x>.go`
builds a `Service` (the `Reconfigurable` listener-migration contract + `Name` +
`Shutdown`); `register_<x>.go`'s `init()` calls
`registerService("<x>", build<X>Service, wireMigrator)`. The hub iterates the
registry in `buildServices(deps)` and routes the built service via
`registerBuiltService`. Generic inputs cross the boundary as plain values in
`ServiceDeps`; **no `internal/component/<x>` type may appear in `ServiceDeps` or
any always-on signature** — widen always-on handles to `Reconfigurable` (as
`SetWeb`/`SetLG` do). A second construction path (e.g. a `ze start --web`
standalone mode) goes through a nil-able seam var set from the gated
registration, never a direct always-on import.

**Seam (ssh, gNMI).** Use a seam when the listener registry genuinely cannot
express the construction shape. ssh is built inside shared daemon startup,
interleaved with always-on AAA/authz/accounting, and owns an interactive session,
so it uses `ssh_infra.go` (`sshBuild` / `sshWirePostStart` /
`sshBuildStandalone`). gNMI has richer constructor dependencies, a reload
notification hook, and no listener live-migration contract, so it uses
`gnmi_infra.go` (`gnmiBuild` / `gnmiReloadNotify`). Always-on code calls the seam
if non-nil; with the tag off the vars stay nil and the feature is skipped. Use a
seam ONLY when the registry genuinely does not fit; prefer the registry.

**Core-level seam (telemetry).** When more than one start site in *different
components* must reach a gated feature, the seam var cannot live in the hub --
put it in the always-on leaf both sites already import. The Prometheus exporter
(`ze_telemetry`) is started from the hub standalone path *and* the bgp reactor
path (`internal/component/bgp/config`), so its hook `metrics.StartExporter` lives
in `internal/core/metrics`; the hub's gated `register_telemetry.go` wires the
gated `internal/component/telemetry/exporter` (and its `collector` sidecar) into
it. The metric COLLECTION API (registry + the `NopRegistry` dummy) stays in that
same always-on leaf so dependents keep working when the exporter is gated --
gate only the part nothing always-on imports (the HTTP exporter), never the
collection API. A core leaf may hold a nil-able hook var set by a gated
component init; `make ze-tier-check` stays green (a value, not an import).

## Banned

- A hand-maintained second list of gate tags or gated packages anywhere. Declare
  the gate ONCE in `feature-gates.txt`; derive the rest.
- An always-on (untagged, non-test) `import` of a gated feature package. Route
  through the registry or a seam. `dep_audit.py --check` enforces this.
- A feature type in an always-on signature (`*zeweb.WebServer`, etc.). Use
  `Reconfigurable` or another always-on interface.
- Leaving a feature's borrowed helper in the feature package when always-on code
  needs it. Extract to `internal/core/*` first.
- Adding a gate without present/absent build-tag tests and an `nm` symbol check.
