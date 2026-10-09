# Module Tiers: core, component, plugin

Where a Go package lives under `internal/` is decided by dependency direction,
not by size or age. This page is the reference for the three tiers, the two
mechanical axes that place a package, the non-engine category manifest, and
compile-out. The obligations are in `ai/rules/architecture.md`; the mechanics
are here.

This generalizes the "delete the folder" test in `ai/rules/plugins.md` into a
placement rule that a gate can audit.

## The three tiers

| Tier | Home | What it is | Examples |
|------|------|------------|----------|
| core / infra | `internal/core/` | A library you cannot run as a plugin. Foundational, with no config-driven lifecycle | family, events, metrics, diagnostic, textbuf |
| component | `internal/component/` | A platform plugin: other plugins or components depend on it, or plug into it | bgp, iface, firewall, traffic, vpp |
| plugin (edge) | `internal/plugins/` | An edge plugin: a config-driven engine that nothing else depends on | ntp, static, dhcpserver, l2tp-auth-* |

## The two axes

| Axis | Mechanical test |
|------|-----------------|
| A. Is it a config-driven engine? | Does it call `sdk.NewWithConn(`? |
| B. Does a feature depend on it? | Does any `.go` file under `internal/component/` or `internal/plugins/` import it, excluding its own subtree, the generated composition root, `cmd/ze` dispatch, `internal/core`, `internal/chaos`, `internal/test`, and `_test.go`? |

<!-- source: pkg/plugin/sdk/sdk.go -- NewWithConn -->
<!-- source: internal/le/arch/tier/tier.go -- the placement gate and the reverse-dependency report -->

The normative rule follows from the two axes. A config-driven engine at a
top-level subsystem belongs in `internal/component/` when a feature depends on
it, and in `internal/plugins/` otherwise. A non-engine package outside
`internal/core/` is either classified by the existing registration mechanics or
carries a row in the non-engine manifest.

The "wired as a plugin" signal is mechanical. The gate reads the composition
roots (the generated `all.go`, the gated `all_<tag>.go` files, the `cmd/ze`
dispatch companions, and `cmd/ze/setup_features_*.go`) to tell a registered
package from a genuine core candidate. It catches every registration shape:
`registry.Register`, `RegisterRPCs`, `RegisterBackend`, doctor checks, `*-cmd`
verb providers, and setup-feature commands. There is no permanent allowlist.

## The non-engine category manifest

`internal/le/arch/tier/testdata/tier_non_engine_categories.txt` is the source of
truth for intentional non-engine placements outside `internal/core/`. It is
non-code data consumed by `./le arch tier check`, so an exception is never hidden in
Go code.

Each row is:

```text
<repo-relative package dir> <category> <rationale>
```

| Category | Meaning | Allowed home |
|----------|---------|--------------|
| `framework` | Wiring substrate or setup feature that exists to register, configure, command, audit or orchestrate other packages | `internal/component/`, or a setup package under `internal/plugins/` |
| `host-service` | Listener, appliance, host API or platform service pinned to composition by startup or by doctor and platform registration | `internal/component/` |
| `domain-library` | Non-engine package belonging to a real domain cluster. Today that means BNG and VPN only | `internal/component/` |
| `planned-violation` | A known placement scheduled to move or disappear. A new row needs a spec reference in its rationale | `internal/component/` or `internal/plugins/` |

The manifest classifies; it does not allow. A row may not point at an engine,
must use the correct home for its category, and may not go stale.

## Compile-out

Axis B also decides whether a feature can be compiled out of the binary. A
feature is compile-out-able exactly when nothing always-compiled depends on it:
it is reached only through build-tag-gated registration. A direct functional
import from always-on, untagged code pins the package into every binary and
defeats the compile-out. Only a blank or gated registration import can be
dropped by a build tag.

Two construction shapes keep a compile-out feature out of always-on code.
Listener services such as looking-glass, web and MCP register factories in
`cmd/ze/hub/service_registry.go`. Dedicated seams such as
`cmd/ze/hub/ssh_infra.go`, `gnmi_infra.go`, `api_infra.go`, and the core
metrics hook carry inputs that do not fit that registry. Each gated service
keeps its direct package and YANG imports behind the matching `ze_<feature>`
build tag.

`feature-gates.txt` at the repository root is the source of truth, declaring
gates as `<tag> <pkg>` rows. `./le repo feature-tags write` updates the static
consumers and `./le repo feature-tags check` refuses drift.

## What the gate enforces

`./le arch tier check` enforces engine placement, the non-engine manifest, core
import direction, disable-ability, build-tag drift, and plugin import ownership.
Grandfathered core import pairs are non-code data in
`internal/le/arch/tier/testdata/core_import_baseline.txt`; a new pair and a stale
row both fail. Plugin ownership has **no baseline**.

`internal/le/arch/tier/testdata/tier_migration_baseline.txt` lists engines scheduled
to move. The gate fails on a new violation and on a stale entry, so the file
can only shrink. An empty baseline means zero exceptions.

The **placement** check excludes nested sub-plugin namespaces, which it reads
from `PluginSearchRoots` in `internal/le/plugin/imports/pluginimports.go`.
The **ownership** check includes them. Both use the composition generator's
policy roots rather than maintaining another list.

### Plugin import ownership

The ownership check rejects production imports into another plugin's
implementation. It reads actual Go import declarations, including aliases, dot
imports and blank imports, without filtering platform files or build tags.
Unreadable or malformed source fails the check; comments and string literals
are not import edges. Dependency direction is checked throughout production
source, so routing an import through a shared helper still fails at the helper's
edge into the plugin.

Ownership follows the existing registration layout:

- Under a `plugins` search root, the first non-schema `register.go` on a package's
  ancestry defines its owner. This distinguishes grouped plugins such as
  `bgp/plugins/nlri/ls` and `bgp/plugins/nlri/flowspec`, while keeping `rib/pool`
  and a registered owner's command packages inside their owner. Without a
  registration, the immediate child of the namespace owns its subtree.
- A nested standalone search root, such as `bgp/reactor/filter`, owns its subtree.
  A more deeply declared search root starts a separate ownership boundary.
- Top-level component search roots, such as BFD and iface, remain host
  infrastructure, not edge-plugin owners. Their shared APIs remain valid
  dependencies; a registered descendant still owns its implementation.

Within-owner imports and shared contracts outside an owned subtree remain
allowed. A package called `api` inside a plugin does not become shared by name.
Registration is allowed only for an actual blank-import edge from `cmd/ze`'s
main package or the import-only `internal/component/plugin/all` composition
files. A `dispatch` or `all.go` filename alone grants no exemption. Named and dot
imports from those roots still fail, and an arbitrary production package's
blank import still pins the plugin and fails. One more blank edge is allowed: a
schema package (a `yang` directory) importing another schema package, which is
how a module's YANG `import` of a module another plugin registers is mirrored
in Go (`command-ownership.md`, "YANG as Data, Not Code"). A schema package holds
modules and no implementation, and the owner approved schema packages
importing each other on 2026-10-09.

Test files, test/chaos/performance harnesses, `internal/le` and `bin` tools are not
production importers. The shared scanner excludes fixture `testdata` directories,
vendored dependencies, scratch trees and the existing module-cache/worktree
exclusions. It parses the remaining Go files in full.

`./le arch tier check` already runs in regular worktree verification.
`./le arch tier selftest` includes clean ownership and forbidden platform-helper
fixtures; package tests also dispatch the registered command and distinguish
registration from functional imports. Ownership failures name each importing
file and imported package in deterministic order.

This graph check does **not** prove full feature ownership or runtime
removeability. In particular, copied role policy such as reactor logic reading
`RSClient` can violate the plugin rule without importing the route-server plugin.
The process-local-call heuristic and generated-import freshness check enforce
different properties; neither substitutes for ownership checks.

<!-- source: internal/le/plugin/imports/pluginimports.go -- PluginSearchRoots -->
<!-- source: internal/le/arch/tier/ownership.go -- pluginOwner, pluginOwnershipGate, compositionRegistration, schemaDependency -->

## Related documents

- `ai/rules/architecture.md` -- the placement obligations
- `ai/rules/plugins.md` -- the delete-the-folder invariant, registration patterns, the Proximity Principle
- `docs/architecture/core-design.md` -- component boundaries
