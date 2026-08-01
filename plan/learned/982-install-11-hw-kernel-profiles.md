# 982 - install-11 hardware kernel profiles

## Context

Spec A replaced the shared kernel shell recipe with a Python recipe, moved profile resolution and requirement enforcement into Go, and kept the existing installer/runtime kernel profiles behavior-preserving.

## Decisions

- Kernel profiles are an open registry: `<name>.config` plus `<name>.require`, safe-token names only, optional one-level `# ze-base: <profile>` layering.
- Go owns profile resolution and the verified guarantee. `ze appliance kernel` resolves fragments, invokes Docker/QEMU with explicit `--fragment` order, reads emitted `build/config`, then enforces manifests plus the hardcoded universal floor (`IP_PNP_DHCP`, `EXT4_FS`, `BLK_DEV_INITRD`, `DEVTMPFS_MOUNT`).
- `tools/kernel-builder/build.py` is intentionally thin: stdlib download/extract/copy, subprocess only for `make`, kernel `merge_config.sh`, and `patch`. No `shell=True`.
- Raw `make -C tools/installer-kernel` and `make ze-kernel` remain Ze-binary-free and therefore unverified by Go; they still consume the same `.config`/`.require` registry and `build.py`.
- Cache variants include registry-derived hashes so profile/config/manifest/builder changes invalidate stale kernel artifacts.
- `hardware-kms` remains the one non-`=y` special case: Go downloads/passes i915 firmware and enforces `CONFIG_EXTRA_FIRMWARE` is set in the emitted config.

## Tests and gates

- Unit tests cover token validation, missing manifests, one-level base ordering, nested-base rejection, manifest parsing, universal floor, stripped `=y`, cache invalidation, Docker/QEMU argv, and firmware enforcement.
- Install functional tests cover Docker/QEMU kernel paths, auto builder selection, open-registry fixture profile, missing manifest failure, stripped-symbol failure, no project shell script in `tools/kernel-builder`, runtime dependency invalidation, and make wiring.
- `make ze-verify-changed` passed for the implementation. A full `make ze-verify` attempt was blocked by unrelated existing failures in `tmp/isis/genfix` and `internal/component/bgp/config`.

## Gotchas

- `ze-validate` flags any exported symbols in a changed Go file with no cross-package caller, even if the symbol pre-existed. Keeping appliance-only helpers unexported avoids false failures.
- Functional fake builders must emit all symbols required by the manifest and universal floor. Otherwise Go enforcement correctly fails after the fake build writes `build/config`.
- The Docker/QEMU fake tests should assert the absence of old `build.sh` and runtime-only flags, not only the presence of new `build.py` argv.

## Files

None recorded.
