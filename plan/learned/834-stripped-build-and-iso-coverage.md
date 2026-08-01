# 834 -- Stripped Build And ISO Coverage

## Context

The appliance installer work added a second Ze product shape (the minimal/stripped build, now the no-tag default) and a new offline ISO path. Two regressions followed quickly: the stripped backend reused validation helpers from the full self-update implementation, which pulled `selfupdate.go` and its heavy imports back into the appliance build, and the first test pass proved only unit behavior while missing the real no-flag ISO path and the shipped `ze-stripped` CLI surface.

## Decisions

- Split self-update shared types and config validation into stripped-safe files, and gate the runtime self-update implementation behind `ze_distro`.
- Keep the appliance ISO default kernel contract aligned with the installer-kernel tool output (`tools/installer-kernel/build/Image`) instead of inventing per-arch default filenames.
- Add functional `.ci` coverage for the default ISO artifact path, the arm64 ISO staging path, and the `ze-stripped` command surface instead of relying on unit tests or ad hoc shell checks.

## Consequences

- `go list .../config/system` (no tags) now excludes `selfupdate.go`, so appliance images no longer drag the full self-update implementation into the minimal binary.
- `ze install appliance iso` is exercised through both the default no-flag path and the arm64 staging path in the install suite.
- `ze-stripped` is now covered as a user-facing artifact in `test/ui`, proving command omission and runtime errors through the actual binary surface.

## Gotchas

- When a stripped build still needs validation, move only the validation into a shared file. Reusing helpers from a runtime-heavy file defeats the product split even if the runtime command handlers are tagged out.
- Functional coverage for new CLI behavior must follow the shipped entry point. A QEMU proof that always passes explicit flags does not cover the default path users will actually run.

## Files

- `internal/component/config/system/selfupdate_shared.go`
- `internal/component/config/system/selfupdate_validate.go`
- `internal/component/config/system/selfupdate.go`
- `internal/component/config/system/backend_ze_appliance.go`
- `internal/appliance/cmd_iso.go`
- `test/appliance/appliance-iso-default-paths.ci`
- `test/appliance/appliance-iso-arm64.ci`
- `test/ui/ze-stripped-surface.ci`
- `mk/test-functional.mk`
