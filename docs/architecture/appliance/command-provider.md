# Appliance as a removable command provider

`internal/appliance/` owns the whole `ze appliance` command surface and
registers it through the offline command registry. Deleting the directory and
its blank import removes the surface, and nothing else needs editing.

<!-- source: internal/appliance/register.go -- root handler registration -->

## What this replaced

The appliance commands sat under `ze install appliance`, reached through a
static switch case in the install dispatcher, with a deprecated `ze appliance`
alias that warned and delegated. Deleting the directory left dangling references
in three places: the install dispatch, its usage text, and the alias. That is
the failure the ownership model exists to prevent.

## Decisions

- **`internal/appliance/`, not `internal/plugins/appliance/` or
  `internal/component/appliance/`.** The plugins tier is for SDK runtime plugins
  with a bus and an engine; the component tier implies a daemon role. Appliance
  has neither: it is build-host tooling.
- **`MustRegisterRootHandler`, not a `case "appliance"` in the central switch.**
  Dispatch goes through the registry lookup, so no appliance spelling remains in
  the central switch.
- **Clean break.** The old path and the alias were deleted rather than
  transitioned. Ze has never been released.
- The importable leaf packages `internal/core/helpfmt` and `internal/core/suggest`
  are used instead of their `cmd/ze/internal/` counterparts, which the
  no-cmd-import constraint forbids.

## Consequences

- Appliance is the second offline-only command provider to use root-handler
  registration, after the interface CLI. It validates the model for shell-only
  build-host tooling.
- `ze install` is reduced to `local` and `remote`. A new appliance feature
  registers in `internal/appliance/`.
- The blank import lives in `cmd/ze/setup_features_appliance.go` behind the
  `ze_appliance` build tag.
- `ze help --ai` derives from the registry listing, so a newly registered root
  appears with no help edit.

## Trap

A move between directory depths breaks every relative path in the package,
including the ones inside tests, and including command arrays in the evidence
scripts. Both spellings occur: a string literal, and a list of argv elements
such as `[str(ze), "install", "appliance", ...]`. A grep for the literal misses
the second.

## Related

- `builder.md` for the command surface itself
