# 1021 -- dev-bootstrap

## Context

Ze's dev setup was split across three Makefile targets (`ze-setup-build`, `ze-setup-lint`, `ze-setup`) using ifeq/else platform branching, and only covered build+lint tools (protobuf, jq, golangci-lint, ruff). Appliance and evidence tools (qemu, e2fsprogs, xorriso, grub, uv) were discovered only by failure when a gate script died with "X not found". This work replaced the entire chain with a unified Python script that handles all dependencies.

## Decisions

- Replaced entire Makefile `ze-setup` chain with a single Python script (`scripts/dev/dev-setup.py`) over adding a sub-target `ze-setup-tools`, because consolidating platform detection in testable Python eliminates the only non-Python dev script.
- Used check-name drift test (matching `applianceDoctorChecks()` names against a Python dict) over recording-mock drift test, because `checkE2fsprogs` uses file stat (`e2fsDir`) not `doctorLookPathFn`, so a recording mock misses it.
- Set `uv` `apt=None` instead of `apt="uv"`, because uv is not in standard Debian/Ubuntu repos.
- Grub on macOS is skipped (brew=None) over attempting a tap install, because there is no first-party Homebrew formula.

## Consequences

- `make ze-setup` is now a single entry point for all dev tool installation.
- The `APPLIANCE_CHECKS` dict in `dev-setup.py` is the contract surface for the drift test. Renaming it or changing its format breaks `TestDevSetupMatchesDoctor`.
- Adding a new appliance doctor check in `doctor_checks.go` will fail the drift test unless a matching entry is added to `APPLIANCE_CHECKS`.
- Kernel and initrd checks are excluded from the drift test via a `buildArtifactChecks` allowlist. Adding a new non-installable check type requires updating this allowlist.

## Gotchas

- `checkE2fsprogs` does NOT use `doctorLookPathFn`. It checks `e2fsDir` which is resolved at package init time via `resolveE2FSDir()` (file stat, not LookPath). Any drift test strategy that only mocks `doctorLookPathFn` will silently miss e2fsprogs.
- e2fsprogs on macOS is keg-only. `brew install e2fsprogs` does NOT symlink binaries to `/opt/homebrew/sbin`. The Go code handles this via a Cellar glob at `cmd_build.go`. Dev-setup needs to install the package but does NOT need to modify PATH.
- `uv` is not in Debian/Ubuntu apt repos despite being a common Python tool. The script must print the curl installer URL instead.

## Files

- Created: `scripts/dev/dev-setup.py`, `internal/appliance/dev_setup_drift_test.go`, `scripts/dev/dev_setup_test.py`, `docs/guide/developer-setup.md`
- Modified: `Makefile`, `docs/contributing/testing.md`, `ai/INDEX.md`
