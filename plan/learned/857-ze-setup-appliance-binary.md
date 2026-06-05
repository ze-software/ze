# 857 -- ze-setup appliance binary

## Context

Ze builds four binary variants from the same `cmd/ze/` entry point using positive build tags: `ze` (ze_distro, on-device), `ze-appliance` (ze_appliance, originally for appliance build tooling), `ze-setup` (ze_setup, provisioning), and a stripped no-tag binary for gokrazy. Appliance build commands (`ze appliance init/build/iso/kernel/initrd`) and PXE provisioning (`ze install remote`) were split across two separate tags, even though an operator building an ISO typically also needs PXE provisioning.

## Decisions

- Moved `internal/appliance` import exclusively to `ze_setup`. The `ze-setup` binary is the single "build and deploy" tool: it has all appliance commands (init, build, iso, kernel, initrd) and PXE provisioning (install remote, DHCP, TFTP, image server) in one binary.
- Removed `internal/appliance` from `ze_appliance`. The `ze_appliance` tag is reserved for on-device appliance runtime features (health revert, pushed config), not build-host tooling.
- The `ze_distro` (on-device Linux daemon) binary has never had appliance commands and still does not.
- Documentation uses `bin/ze-setup` in all examples, not bare `ze`. The appliance commands do not exist in the `ze` binary, so examples must be honest about which binary runs them.

## Consequences

- Operators use one binary (`bin/ze-setup`) for the entire pipeline: create appliance, build image, prepare kernel/initrd, build ISO, serve over PXE.
- The on-device `ze` binary stays lean. No build tooling, no Docker invocations, no HTTP download code.
- `make ze-setup` is the build target operators need. `make build` produces all variants.
- The `ze_appliance` tag currently imports nothing (the file has no import block). It exists as a slot for future on-device appliance runtime features.

## Gotchas

- The appliance commands were originally in `ze_appliance` because that tag was created when the appliance builder was first written (learned 675). Moving them to `ze_setup` required updating the build tag test assertions: `build_tag_appliance_test.go` now asserts appliance is absent, `build_tag_setup_test.go` asserts it is present.
- The `build_tag_full_test.go` (all three tags combined) still expects `appliance` present, which works because `ze_setup` brings it in.

## Files

- Modified: `cmd/ze/setup_features_appliance.go` (removed `internal/appliance` import)
- Modified: `cmd/ze/build_tag_appliance_test.go` (assert appliance absent in ze_appliance-only build)
- Modified: `docs/guide/appliance.md` (all examples use `bin/ze-setup`, added end-to-end pipeline walkthrough)
- Modified: `docs/guide/ze-install.md` (ISO section references `ze appliance kernel/initrd`)
