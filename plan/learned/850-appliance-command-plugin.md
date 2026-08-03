# 850 -- Appliance Command Plugin

Supersedes: `plan/learned/815-install-7a-namespace.md` (the move from `ze appliance` to `ze install appliance` is reversed).

## Context

The appliance command surface lived in `cmd/ze/install/appliance/`, nested under `ze install` as a subcommand. It was reached through a static switch case in `cmd/ze/install/main.go`, with a deprecated alias (`ze appliance`) in `cmd/ze/main.go` that warned then delegated. This placement violated Ze's command-surface-ownership model: deleting the appliance directory left dangling references in install's dispatch, usage, and the deprecated alias. The goal was to make appliance a self-contained, removable in-process command provider that owns its entire surface and registers through the importable offline command registry.

## Decisions

- Chose `internal/appliance/` as the provider location over `internal/plugins/appliance/` (reserved for SDK runtime plugins with bus/engine) and `internal/component/appliance/` (implies daemon component presence). Appliance has no daemon role.
- Chose `MustRegisterRootHandler` (the ownership model from command-surface-ownership) over a static `case "appliance"` in `main.go`. The handler is dispatched through `dispatchRegisteredRoot` which calls `LookupRoot`, so no appliance spelling remains in the central switch.
- Chose clean break (delete old path and alias) over a deprecation transition, per `ai/rules/go-standards.md`: Ze has never been released, no users, no compat shims.
- Kept the `dispatchTable()` built-at-call-time pattern from the original package. The cmd*.go files install handlers via package-level var assignment in `init()`, and the map must be built after all `init()` runs.
- Used `internal/core/helpfmt` and `internal/core/suggest` (importable leaf packages) instead of `cmd/ze/internal/helpfmt` and `cmd/ze/internal/suggest` (which would violate the no-cmd-import constraint).

## Consequences

- Appliance is now the second offline-only command provider after iface/cli to use `RegisterRootHandler`. It validates the ownership model for shell-only build-host tooling.
- `ze install` is reduced to `local` and `remote`. Any future appliance features register in `internal/appliance/`, not `cmd/ze/install/`.
- The blank import lives in `cmd/ze/setup_features_appliance.go` (gated `ze_appliance`). Removing that file plus `internal/appliance/` removes the entire surface.
- Future specs referencing `ze install appliance` (e.g. spec-install-8, spec-install-9) need updating if they are implemented; their current text references the old path.

## Gotchas

- The `cmd_build_test.go` had a relative path `../../../../gokrazy/ze/config.json` that was correct from `cmd/ze/install/appliance/` (4 levels deep) but broke from `internal/appliance/` (2 levels deep). Adjusted to `../../gokrazy/ze/config.json`.
- All `ze install appliance` references also exist in the evidence scripts (`scripts/evidence/effective-install-iso-qemu.py`, `effective-install-qemu.py`) as both string literals and command arrays like `[str(ze), "install", "appliance", ...]`. Both forms needed updating.
- The help display (`ze help --ai`) already derives from `cmdregistry.ListRoot()`, so the appliance root appeared automatically after registration. The summary-only `ze help --ai` (no `--cli` flag) shows only counts, not individual commands.

## Files

- Created: `internal/appliance/` (41 .go files + `updater/` subpackage, moved from `internal/appliance/`)
- Created: `internal/appliance/register.go` (root handler registration)
- Created: `internal/appliance/register_test.go`
- Created: `cmd/ze/setup_features_appliance.go` (blank import, `ze_appliance`; was `cmd/ze/appliance_import.go`)
- Created: `test/appliance/` (7 `.ci` files: help, list, build-no-gok, build-arm64-goarch, iso-arm64, iso-default-paths, push-image-escape, no-install-appliance)
- Modified: `cmd/ze/main.go` (removed deprecated `case "appliance"`)
- Modified: `cmd/ze/setup_features_full.go` (removed `runDeprecatedAppliance`)
- Modified: `cmd/ze/setup_features_stripped.go` (removed `runDeprecatedAppliance`)
- Modified: `cmd/ze/install/main.go` (removed appliance case and import)
- Modified: `cmd/ze/install/register.go` (removed appliance from Subs)
- Modified: `scripts/evidence/effective-install-iso-qemu.py`, `effective-install-qemu.py` (command paths)
- Modified: `docs/guide/appliance.md`, `docs/guide/ze-install.md` (command paths)
- Modified: `docs/functional-tests.md`, `tools/installer-kernel/README.md`, `ai/INDEX.md`
- Modified: `cmd/ze-gok/main.go` (comment)
