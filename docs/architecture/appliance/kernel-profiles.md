# Hardware kernel profiles

A kernel profile is a pair of files in an open registry: `<name>.config` holds
the fragment, `<name>.require` holds the manifest of symbols the build must
prove. Names accept safe tokens only. One optional `# ze-base: <profile>` line
gives a single level of layering, and nesting is rejected.

<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile, registeredKernelProfiles, validateKernelProfileName -->
<!-- source: internal/appliance/kernelreq.go -- requirement enforcement over the emitted config -->
<!-- source: internal/appliance/cmd_kernel.go -- fragment ordering, builder invocation, verification -->

## Go owns the guarantee

`ze appliance kernel` resolves the fragments, invokes Docker or QEMU with an
explicit `--fragment` order, reads the emitted `build/config`, and only then
enforces two things: the profile's own manifest, and a hardcoded universal floor
of `IP_PNP_DHCP`, `EXT4_FS`, `BLK_DEV_INITRD`, and `DEVTMPFS_MOUNT`. The
guarantee is read back from the artifact, not assumed from the inputs.

`tools/kernel-builder/build.py` stays thin on purpose: stdlib download, extract,
and copy, with subprocess used only for `make`, the kernel's own
`merge_config.sh`, and `patch`. It never uses `shell=True`.

<!-- source: tools/kernel-builder/build.py -- the single builder driver -->

Raw `make -C tools/installer-kernel` and `make ze-kernel` stay free of any Ze
binary, so nothing verifies their output. They consume the same registry and the
same builder.

## Decisions

- The registry is open. Adding a profile is adding two files, not editing a
  list.
- Cache variants fold in registry-derived hashes, so a profile, config,
  manifest, or builder change invalidates stale kernel artifacts.
- `hardware-kms` is the one profile whose requirement is not a plain `=y`. Go
  downloads and passes the i915 firmware, then enforces that
  `CONFIG_EXTRA_FIRMWARE` is set in the emitted config.

## Traps

- **A fake builder in a test must emit every symbol the manifest and the
  universal floor name.** Otherwise Go enforcement correctly fails after the
  fake build writes `build/config`, and the failure reads as a bug in the
  enforcement.
- **Assert the absence of the old shell driver, not only the presence of the new
  one.** A Docker or QEMU argv test that checks for `build.py` still passes when
  a stale `build.sh` path survives beside it.
- **A test profile goes in a scratch repository root, never in
  `tools/installer-kernel/`.** The registry is a directory scan, so a
  `<name>.config` plus `<name>.require` pair in the tracked directory IS a
  buildable profile for as long as it sits on disk, and an EXIT trap does not
  survive SIGKILL. Both resolvers follow a root the test controls:
  `kernelInstallerConfigDir` is the relative `tools/installer-kernel`, so Go
  follows the process working directory, and `repo_root` in
  `tools/kernel-builder/run.py` walks up from `run.py` to the first `go.mod`.
  Copy the builder directory into the scratch root. A symlink resolves back to
  the real repository through `Path(__file__).resolve()` and defeats the
  isolation.
- `ze-validate` flags an exported symbol in a changed Go file that has no
  cross-package caller, even when the symbol predates the change. Appliance-only
  helpers stay unexported for that reason.

<!-- source: internal/appliance/cmd_kernel.go -- kernelInstallerConfigDir -->
<!-- source: tools/kernel-builder/run.py -- repo_root -->
<!-- source: internal/appliance/kernelreg_test.go -- TestRegisteredKernelProfilesShippedSet -->

## Related

- `build-artifacts.md` for how a built or downloaded kernel is resolved
