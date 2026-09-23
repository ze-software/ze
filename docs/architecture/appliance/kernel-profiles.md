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

`internal/appliance/kernelbuilder` owns backend selection, downloads, verified
Alpine ISO caching, Docker and QEMU lifecycle, and the low-level kernel build.
The low-level Go worker invokes the kernel's `merge_config.sh`, `patch`, and
`tar`. The container and QEMU guest run the compiled `ze-kernel-builder`
command.

<!-- source: internal/appliance/kernelbuilder/driver.go -- Build -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

`ze appliance kernel` selects the profile and architecture, validates the
result against the registry, and uses the shared builder in
`internal/appliance/kernelbuilder`.

The runtime profile requires `CONFIG_XFRM_MIGRATE` for atomic MOBIKE state
migration and `CONFIG_MPLS_IP_MTU` for native labeled-IP fragmentation and ICMP.
Both requirements are in the compiled runtime floor, so an edited manifest
cannot silently remove them. The MPLS capability comes from the registered
kernel patch series.

Flow export requires `CONFIG_NET_ACT_SAMPLE` for the ingress sampling action and
`CONFIG_PSAMPLE` for delivery to the userspace exporter over generic netlink.
Both are built-in requirements in the runtime fragment, manifest and compiled
floor. A kernel with only `CONFIG_NET_CLS_MATCHALL` can classify packets but
rejects installation of the missing sample action; adding a module loader cannot
repair a kernel built without the action or psample.
<!-- source: internal/plugins/flowexport/sampling/tc_linux.go -- SetupSampling, buildSampleFilter -->
<!-- source: internal/plugins/flowexport/sampling/psample_linux.go -- NewPsampleReader -->

Default nftables logging requires built-in `CONFIG_NF_LOG_SYSLOG` as well as
`CONFIG_NFT_LOG`. A modular logger leaves log-rule installation failing with
`ENOENT` in the runtime guest, where Alpine cannot load Ze's kernel modules.
The runtime manifest and compiled floor require both symbols.
<!-- source: internal/appliance/kernelreq.go -- runtimeKernelRequirements -->

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
- **Assert the absence of interpreter and shell drivers, not only the presence
  of the Go worker.** A Docker or QEMU argv test can pass while a stale launch
  path survives beside it.
- **A test profile goes in a scratch repository root, never in
  `tools/installer-kernel/`.** The registry is a directory scan, so a
  `<name>.config` plus `<name>.require` pair in the tracked directory IS a
  buildable profile for as long as it sits on disk, and an EXIT trap does not
  survive SIGKILL. Resolution follows the process working directory through
  the relative `kernelInstallerConfigDir`. Put the complete registry and native
  builder fixture beneath the scratch root.
- `./le repository check` flags an exported symbol in a changed Go file that has no
  cross-package caller, even when the symbol predates the change. Appliance-only
  helpers stay unexported for that reason.

<!-- source: internal/appliance/cmd_kernel.go -- kernelInstallerConfigDir, defaultKernelBuild -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Request.Root -->
<!-- source: internal/appliance/kernelreg_test.go -- TestRegisteredKernelProfilesShippedSet -->

## Related

- `build-artifacts.md` for how a built or downloaded kernel is resolved
