# bug-review-1-inventory-and-self-containment

## Summary

The inventory pass proved that review scope must be derived from compiled registration surfaces, not from directory names alone. Generated imports, registry calls, schema rows, RPC rows, event namespaces, and command-only roots each expose different user-visible surfaces.

## Key decisions

- Counted generated import groups in `internal/component/plugin/all/all.go`: SCHEMA, PLUG, EVT, and RPC.
- Reconciled `internal/plugins/*`, `internal/component/bgp/plugins/*`, NLRI family directories, and BGP command plugin directories.
- Recorded directory-only command providers that use `codegen:skip` and are wired by `cmd/ze`, not by `plugin/all`.
- Assigned every in-scope row to child 2, child 3, child 4, or child 5, with unassigned count zero.

## Results

- Created `plan/review-bug-review-inventory.md`.
- Recorded INV-OBS-1: `capa` is imported by generated plugin aggregator but omitted from generated architecture prose.
- Recorded INV-OBS-2: command roots skipped by codegen are expected and remain child 2 command-wiring scope.

## Gotchas

- Generated YANG glue is evidence, not the canonical owner. Review the owning YANG and package unless the generator itself is suspect.
- A package absent from `plugin/all` is not automatically missing. It may be a command root imported under `cmd/ze` build-tag composition.

## Verification

- Inventory report audit tests passed manually: all imports accounted, registries represented, no unassigned rows, exclusions have reasons.

## Files

None recorded.
