# Feature-Gate Registration (compile-out-able features)

**When:** How to add or change a **compile-out-able feature**
**Severity:** advisory

## Directives

How to add or change a **compile-out-able feature**: a subsystem that can be
dropped from the `ze` binary at build time via a `//go:build ze_<feature>` tag,
for a smaller binary and a smaller attack surface (looking-glass `ze_lg`, ssh
`ze_ssh`, web `ze_web`, gNMI `ze_gnmi`, MCP `ze_mcp`, REST API `ze_rest`, gRPC
API `ze_grpc`, Prometheus exporter `ze_telemetry`, routing protocols `ze_isis` /
`ze_ldp` / `ze_ospf` / `ze_rsvpte`, first-hop redundancy `ze_vrrp`, BGP `ze_bgp`,
...).

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

| Consumer | Role | Mechanism |
|----------|------|-----------|
| `Makefile` `ZE_FEATURES` | default-on tags for `ze` / `ze-appliance` | derives: `$(shell awk ...)` |
| `internal/test/runner` `TestBuildTags` | tags for the functional-test `ze` | derives: reads the file |
| `scripts/codegen/plugin_imports.go` `featureTags` | gates `<pkg>` and `<pkg>/yang` into `all_<tag>.go` | derives: `loadFeatureTags` |
| `scripts/dev/dep_audit.py` `DISABLEABLE` | no always-on import of `<pkg>` | derives: `load_feature_gates` |
| `scripts/dev/stress-repro.py` `race_tags` | full-feature race build | derives: `_feature_gate_tags()` |
| `.golangci.yml` `build-tags` | lint the feature-on build | **generated** by `feature_tags.go` |
| `gokrazy/ze/config.json` `GoBuildTags` | appliance image build tags | **generated** by `feature_tags.go` |
| `docs/guide/quickstart.md` `go install` cmd | install without cloning the repo | **generated** by `feature_tags.go` |

**No consumer is hand-maintained.** The three static files that cannot read the
manifest at runtime (`.golangci.yml` `build-tags`, `gokrazy/ze/config.json`
`GoBuildTags`, `docs/guide/quickstart.md`'s `go install -tags '...'` command) are
GENERATED from it by `scripts/codegen/feature_tags.go` (run by `make generate`,
surgical byte-stable edits). Do NOT hand-edit their tag lists -- add the gate to
`feature-gates.txt` and run `make generate`. Three gates catch drift: the
`scripts/codegen` unit test `feature_tags.go --check`, `dep_audit.py --check`
(golangci), and `internal/appliance` `TestGokrazyConfigMatchesApplianceBuildTags`
(gokrazy).

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
5. `make generate`. This emits `all_ze_<x>.go` (plugin_imports) AND regenerates the
   three static tag lists from the manifest (`feature_tags.go`: `.golangci.yml`
   `build-tags`, `gokrazy/ze/config.json` `GoBuildTags`, `docs/guide/quickstart.md`).
   Do NOT hand-edit those files' tag lists. Then `make ze-verify-changed`.
6. Write present/absent build-tag tests:
   `cmd/ze/hub/build_tag_<x>_present_test.go` (`//go:build ze_<x>`) and
   `_absent_test.go` (`//go:build !ze_<x>`); an absent test asserts via
   `go tool nm` that zero feature symbols are linked.

That is the whole list. Step 3 (edit `feature-gates.txt`) is the ONLY manifest
declaration point. Everything else follows: the Makefile, the runner, the generators,
dep_audit, and stress-repro all derive from it, and `feature_tags.go` (via
`make generate`) regenerates the three static tag lists. There is nothing to hand-sync.

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

**Plugin compile-out (routing protocols).** When the feature is already a
self-registering plugin discovered by the generator (`register.go` -> `plugin/all`),
there is NO new `register_<x>.go` or seam: gating is purely *blank-import
partitioning*. List each owned dir as its own `feature-gates.txt` line under the
shared tag -- a protocol spans several discovered dirs (engine + `transport` +
`cli` + the `*-cmd` command schema), e.g.:

```
ze_ospf  internal/plugins/ospf
ze_ospf  internal/plugins/ospf/cli
ze_ospf  internal/plugins/ospf/transport
ze_ospf  internal/plugins/ospf/v3/transport
```

The plugin's `.go` files are NOT source-tagged; `make generate` moves their blank
imports into `all_<tag>.go` and dead-code elimination drops the unreferenced
packages when the tag is off (A-1: nothing always-on imports a protocol). Mind the
**two composition roots**: the generated `all.go` AND the hand-written
`cmd/ze/ze_core_dispatch.go` (CLI). Protocols with a programmatic `cli` package
(isis, ospf) move their dispatch-root CLI blank import into a per-protocol gated
companion `cmd/ze/dispatch_<proto>.go` (`//go:build ze_core && ze_<proto>`); miss
that root and the package stays linked. A plugin that registers its CLI through the
plugin registry's `CLIHandler` (not a programmatic `cli` package) has only the ONE
root -- the generated `all.go` -- and needs NO dispatch companion. The shape is not
protocol-specific: `ze_vrrp` (first-hop redundancy; the `vrrp` plugin + its `transport`
sidecar, two manifest lines, no `cli` package) is that single-plugin case, gated purely
by blank-import partitioning like ldp/rsvpte. Routing protocols are also the first gated
packages that are `sdk.NewWithConn` *engines* and multi-package features whose
sub-packages import each other, so `dep_audit.py` (a) counts the generated
`all_<tag>.go` as a registration importer (an engine's blank import there is not a
"feature depends on it" tier violation) and (b) skips same-tag importers in the
disableable check (the engine importing its own `transport` sub-package is
intra-feature -- dropped together, not an always-on pin).

**Extract-then-gate at subsystem scale (`ze_bgp`).** Gating the BGP subsystem
(~59 manifest lines: the whole `internal/component/bgp` tree plus
`internal/plugins/flowspec-firewall`) is the same blank-import partitioning, but
the one invariant does NOT hold going in -- 27 always-on files imported a bgp
package. Three techniques clear them, in this order of preference. The goal is
the FEWEST source-tagged files, not the fewest edits:

1. **Transitive package drop** (no tag). A manifest line moves the package's
   blank imports into `all_<tag>.go`; dead-code elimination does the rest. Whole
   plugins qualify: `flowspec-firewall` needed one line and no source change.
2. **Core-leaf move** (no tag). A contract always-on consumers share with the
   feature moves to an always-on `internal/core/*` leaf; consumers change an
   import path only. `ze_bgp` needed three: `internal/core/bgp/routeaction` (the
   route-action/verb vocabulary sysrib and every FIB backend use),
   `internal/core/bgp/msgtype` (the message-type codes MRT classifies by), and
   `internal/core/bgp/ribevents` (the best-change contract sysrib and flow-export
   subscribe to). Move the LEAF, not the package: `bgp/message` imports
   `plugin/registry`, so relocating it wholesale would be a core-tier violation.
3. **Inversion-of-control seam** (no tag on the always-on side). Where always-on
   code reaches INTO the feature, invert it: the always-on side exposes a
   nil-able hook and the gated code self-registers from its own `init()`. A nil
   seam needs a CORRECT no-feature behavior, not just a nil check. `ze_bgp`
   inverted five: `ze config dump|diff|validate` tree resolution and peer
   validation plus the graceful-restart marker writer
   (`internal/component/config/infra`), the MRT RIB-dump provider and the web
   hex-packet decoder (`internal/component/plugin/registry`), and the IGP
   next-hop cost sysrib used to push into BGP best-path
   (`internal/core/rib/igpcost`).

Two traps that only appear at this scale:

- **A feature-gated file is still an always-on pin for a DIFFERENT gate.**
  `cmd/ze/hub/service_ssh.go` (`//go:build ze_ssh`) imported `bgp/config`.
  `dep_audit.file_requires_tag` is per-tag, so it flags that file -- correctly: a
  `ze_ssh`-on / `ze_bgp`-off build genuinely fails to compile. Clear gated files
  too, not just untagged ones.
- **Removing an always-on import can unlink an `init()` nobody else pulls in.**
  `bgp/config` registers the reactor factory, and it was linked ONLY because the
  hub imported it. Blank-importing it from `bgp/plugin` is the natural fix but
  cycles in test (`bgp/config`'s own tests import `plugin/all`, which imports
  `bgp/plugin`). It is linked from `cmd/ze/dispatch_bgp.go` instead: a `package
  main` root can never be imported back, so the edge is always safe there. After
  deleting an always-on import, ask what that package's `init()` was providing.

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
