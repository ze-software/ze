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
<!-- source: internal/le/tier/tier.go -- the placement gate and the reverse-dependency report -->

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

`internal/le/tier/testdata/tier_non_engine_categories.txt` is the source of
truth for intentional non-engine placements outside `internal/core/`. It is
non-code data consumed by `./le tier check`, so an exception is never hidden in
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
gates as `<tag> <pkg>` rows. `./le feature-tags write` updates the static
consumers and `./le feature-tags check` refuses drift.

## What the gate enforces

`./le tier check` enforces engine placement, the non-engine manifest, core
import direction, disable-ability, and build-tag drift. Grandfathered core
import pairs are non-code data in
`internal/le/tier/testdata/core_import_baseline.txt`; a new pair and a stale
row both fail.

`internal/le/tier/testdata/tier_migration_baseline.txt` lists engines scheduled
to move. The gate fails on a new violation and on a stale entry, so the file
can only shrink. An empty baseline means zero exceptions.

The gate excludes nested sub-plugin namespaces, which it reads from `pluginDirs`
in `internal/le/plugin/imports/pluginimports.go`, so packages under
`internal/component/bgp/plugins/` are never flagged for being nested.

<!-- source: internal/le/plugin/imports/pluginimports.go -- pluginDirs -->

## Related documents

- `ai/rules/architecture.md` -- the placement obligations
- `ai/rules/plugins.md` -- the delete-the-folder invariant, registration patterns, the Proximity Principle
- `docs/architecture/core-design.md` -- component boundaries
