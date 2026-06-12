# Adding validated fields to registry.Registration across import cycles

## Context

Internal plugins needed to declare doctor checks via `registry.Registration`
instead of calling `diagnostic.RegisterDoctorCheck` as a side-channel. The
cycle `registry` -> `diagnostic` -> `plugin` -> `registry` blocked importing
diagnostic types directly.

## Key decisions

1. **Types in `registry` using `any` for cross-boundary values.** `DoctorCheckContext`
   types Tree as `any` (is `*config.Tree`) and Platform as `any` (is `*host.PlatformInfo`).
   This matches `ConfigureMetrics func(reg any)` and `ConfigureEventBus func(eventBus any)`
   already in Registration. Plugin authors type-assert in their check functions.

2. **Reuse `rpc` types for return values.** `rpc.DoctorCheckDiagnostic` (Code, Severity,
   Message as strings) is what external plugins already return over RPC. Internal plugins
   now return the same type. The bridge in `doctor/checks_plugin_registry.go` converts
   to `diagnostic.Diagnostic`.

3. **Runtime bridge, not init-time.** Plugin registrations and the bridge both run in
   `init()`. Go init ordering depends on the import graph, and `plugin/doctor` does not
   import `plugin/all`, so it cannot guarantee all plugins have registered. The bridge
   runs at doctor execution time inside `runDoctorChecks()` by querying
   `registry.PluginDoctorChecks()`. This avoids init ordering problems entirely.

4. **Provider pattern for validation sets.** `registry` validates platform strings but
   cannot import `host` for the valid set. Solution: `host/register.go` calls
   `registry.RegisterDoctorPlatforms()` at init with all `platformTypeNames` values.
   Only `"any"` is pre-seeded. This avoids hardcoding platform strings in the leaf
   package and keeps validation current when platforms are added.

## Recurring pattern

When `registry.Registration` needs a new validated field whose valid values come from
a package that `registry` cannot import:

1. Define a `Register<Thing>` setter in `registry` (protected by `mu`)
2. Call it from the owning package's `init()` (or `register.go`)
3. Pre-seed only the universal default (e.g., `"any"`) in the `var` initializer
4. Validation reads the registered set without locking (init-time writes, runtime reads)

## Files

- `internal/component/plugin/registry/doctor.go` -- types, validation, query
- `internal/component/doctor/checks_plugin_registry.go` -- runtime bridge
- `internal/component/host/register.go` -- platform name registration
- `internal/plugins/l2tpauthradius/register.go` -- reference migration
