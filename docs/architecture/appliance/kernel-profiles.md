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

The builder runs only natively. `validateRequest` refuses a kernel whose
architecture is not the host's (`hostGOARCH`), so Docker's `--platform` always
names the host's own platform, and `hardwareAccelerator` refuses a QEMU that
offers neither `hvf` nor `kvm` instead of emulating under `tcg`. A QEMU that
cannot answer `-accel help` at all, missing or crashing, is refused as that
failure with its exit status and stderr, never reported as a host without an
accelerator. `-accel help` lists the accelerators compiled into QEMU, not the
ones the host can use, and a distro QEMU on Linux always lists `kvm`, so a
listed accelerator counts only when its probe passes: `/dev/kvm` opens
read-write for `kvm`, and `sysctl -n kern.hv_support` prints `1` for `hvf`.
The check runs when the builder is chosen, before the Alpine ISO download, and
`ze doctor` asks the same question. The probes and `docker info` are bounded
by `hostProbeTimeout`. A permission error on `/dev/kvm` gets the kvm-group fix;
any other open error points at the kernel log. A QEMU that still exits before
the guest boots is reported with what it wrote to stderr, read after the
process has ended so the last line is not lost. A QEMU that says nothing is
reported as an exit with its status when it exited on its own, or as a guest
that never reached its login prompt within `bootTimeout` when it kept running,
and either way with its exact command line, to run by hand. A failure after
login (SSH never answers, the build fails) carries QEMU's stderr when it wrote
any; SSH that never answers ends with the QEMU command that boots the same VM,
to log in on its console and run `sshd -d`. A probe stopped because the build
was canceled says so, rather than reporting a hung QEMU or Docker daemon.

`ze appliance kernel` gives one kernel build 2 hours (`kernelBuildTimeout`,
owner decision 2026-10-11). That is the only limit on a build, and the command
hands it to the builder as Ze's own (`kernelbuilder.WithBuildLimit`). A build
stopped part way names the step it was at and why it stopped. It is never
reported as a failed step or as a kernel that failed to compile:

| Why it stopped | What the error says |
|----------------|---------------------|
| Canceled (Ctrl-C) | the step, and that the build was canceled. No next step: the operator stopped it. A step that can leave work running still says how to clear it |
| Ze's limit | the step, that the build did not finish within the 2-hour limit Ze gives one kernel build, then the step's next step |
| A deadline that a caller of the package set | the step, that the build ran out of the time its caller gave it, then the step's next step |

| Step | Next step when it runs out of time |
|------|------------------------------------|
| The QEMU backend downloads the Alpine ISO's checksum, or the Alpine ISO | `curl -fL -o /dev/null <url>`, to check that the host reaches the mirror and how fast |
| Docker builds the builder image | the same `docker build` command, to run by hand |
| The kernel container builds the kernel | the build output above. On every stop, a Ctrl-C included, it then gives `docker ps --filter ancestor=<image>`: the stop kills the Docker CLI, not the container, which can still be compiling and still holds the build volumes the next build mounts |
| Go compiles the QEMU kernel worker | the same Go command, with its `GOOS` and `GOARCH`, to run from the repository root |
| The build VM boots, logs in, or sets up SSH | the QEMU command that boots the same VM to the same stage |
| The host downloads the kernel tarball for the build VM | the `curl` command that fetches it by hand to the path the next build reads it from, through a `.part` file, so a fetch cut short leaves no partial tarball |
| The build VM builds the kernel | the build output above, and `ssh -t <options> -p <port> root@localhost top` to watch the next build, with the options the build logs in with (`sshClientOptions`) and the port that run prints on its `>>> booting Alpine VM` line. The QEMU command alone boots a VM with no build in it, so it is not offered here |
| Any other step, and the choice of builder | `Build` names the stop itself, with what the step said as evidence and the build output above as the next step (`stopReport`). It does so for every backend error and for `SelectBuilder`'s, whose accelerator probe runs `sysctl` on a Mac, so no stop from either is reported as a library's error alone |

A download has no limit of its own, only the build's 2 hours: a stalled mirror
is reported when the build runs out of them, at the download step, with its URL.
The i915 firmware blobs of the `hardware-kms` profile are fetched before the
build starts, each within 10 minutes (`downloadTimeout`). A firmware download
that fails or runs out of that time names its URL and gives the `curl` command
that fetches the blob by hand to where the next run looks for it. Every limit a
build names is worded by one formatter (`kernelbuilder.LimitWords`): "the
2-hour limit" for the kernel build, "the 10-minute limit" for a firmware
download and for the ISO tools, and "within 15s" for each question the build
asks the host first (`hostProbeTimeout`: QEMU's `-accel help` and
`docker info`).

An arm64 kernel is built on the Mac and an amd64 kernel on the Linux host.

<!-- source: internal/appliance/cmd_kernel.go -- kernelBuildTimeout, defaultKernelBuild, downloadFirmwareBlob, firmwareDownloadFailed -->
<!-- source: internal/appliance/kernelbuilder/stopped.go -- WithBuildLimit, buildLimit, LimitWords, errBuildStopped, errBuildStoppedLeaving, stopReport -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build, SelectBuilder, validateRequest, runDocker, containerFailure -->
<!-- source: internal/appliance/kernelbuilder/space.go -- dockerRootDir -->
<!-- source: internal/appliance/kernelbuilder/qemu.go -- hostProbeTimeout, hardwareAccelerator, probeKVM, probeHVF, availableAccelerators, errBootFailed, buildGuestWorker, qemuNextStep, guestBuildNextStep, sshClientOptions, ensureAlpineISO, downloadNextStep, fetchKernelTarball, tarballNextStep -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

`ze appliance kernel` selects the profile and architecture, validates the
result against the registry, and uses the shared builder in
`internal/appliance/kernelbuilder`.

The kernel source is pinned by exact release and digest.
`internal/appliance/kernel.version` names the release (7.2.9), and
`kernelSourceSHA256` in the worker holds the SHA-256 kernel.org publishes for
that tarball in `sha256sums.asc`. The worker refuses a version with no tracked
digest before downloading, and refuses a tarball, downloaded or found in the
work directory, whose digest differs, naming both digests;
`TestKernelVersionHasDigestPin` fails when the pinned release has no digest.
The patch series applies with `patch --fuzz=0`, so a bump the series no longer
fits fails the build instead of placing a hunk by guesswork; a bump refreshes
the series against the new release.
<!-- source: internal/appliance/kernelbuilder/worker.go -- kernelSourceSHA256, verifyKernelSource, applyPatches -->

A bump also owes a pass over the
[Linux limitations register](../linux-limitations.md): re-read each `kernel`
entry at the new release and update its `Checked` line.
`TestLinuxLimitsCheckedAtShippedVersions` reads the register through the shared
`linuxlimit.Parse` parser and fails while any entry names another release.
<!-- source: internal/appliance/linux_limits_test.go -- TestLinuxLimitsCheckedAtShippedVersions -->
<!-- source: internal/le/linuxlimit/parse.go -- Parse -->

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

Subscriber upload policing requires built-in `CONFIG_NET_ACT_POLICE`.
`shaper.applyTC` hands the upload rate to the traffic backend, whose
`ingressPolicerFilter` installs a police action on the shared ingress hook.
The matchall classifier alone cannot enforce that rate. The runtime fragment,
manifest and compiled floor require the action, so a modular or missing action
is refused even if an edited manifest omits it.
<!-- source: internal/component/l2tp/plugins/shaper/shaper.go -- applyTC -->
<!-- source: internal/plugins/traffic/netlink/policer_linux.go -- ingressPolicerFilter -->

The ingress policer coexistence proof also requires built-in
`CONFIG_NET_ACT_GACT`: its foreign priority-1 filter carries a generic `PIPE`
action, not a mirred action. The runtime fragment, manifest and compiled floor
include that test prerequisite so the guest can exercise coexistence without
a module loader. This adds no traffic-control configuration surface.
<!-- source: internal/plugins/traffic/netlink/integration_linux_test.go -- TestNetlinkIntegration_IngressPolicerLeavesOtherHookOwnersAlone -->
<!-- source: internal/appliance/kernelreq_test.go -- TestRuntimeFloorRequiresBuiltInGenericAction -->

Default nftables logging requires built-in `CONFIG_NF_LOG_SYSLOG` as well as
`CONFIG_NFT_LOG`. A modular logger leaves log-rule installation failing with
`ENOENT` in the runtime guest, where Alpine cannot load Ze's kernel modules.
The runtime manifest and compiled floor require both symbols.
<!-- source: internal/appliance/kernelreq.go -- runtimeKernelRequirements -->

IPsec and WireGuard require their kernel support built in: `CONFIG_XFRM_USER`
(the netlink interface every SA install goes through), `CONFIG_XFRM_INTERFACE`,
`CONFIG_INET_ESP`, `CONFIG_INET6_ESP`, `CONFIG_INET_AH`, `CONFIG_INET6_AH`,
`CONFIG_WIREGUARD`, and one CRYPTO symbol for each kernel transform Ze's XFRM
algorithm tables can install: `CBC`, `DES`, `NULL`, `GCM`, `CHACHA20POLY1305`,
`SHA1`, `SHA256` and `SHA512`. These facilities must not depend on runtime
module loading. The runtime fragment requests each one, and the runtime
manifest and compiled floor require each one. The transform-to-symbol map is
`xfrmTransformKernel`, and
`TestXfrmTransformsHaveRequiredKernelSymbol` walks the algorithm tables and
fails when a transform's symbol is not `=y` in the fragments or not in
`runtime.require`, so a cipher added to a table alone fails a test. The AES,
AUTHENC, ECHAINIV, SEQIV, HMAC and SHA256 templates the transforms also use are
selected by `INET_ESP` through `XFRM_ESP` in Linux 7.2.
<!-- source: internal/component/ike/dataplane/kernelcap_linux.go -- xfrmTransformKernel -->
<!-- source: internal/appliance/kernelreq.go -- runtimeKernelRequirements -->

PWM fan control requires built-in `CONFIG_SENSORS_PWM_FAN`, with `CONFIG_HWMON`
and `CONFIG_PWM` built in. The runtime fragment requests all three, and the
manifest checks their emitted values. The compiled floor also requires the fan
driver, even if an edited manifest omits it. This preserves fan control without
loading a module: gokrazy's `Boot` calls `loadModules` before services, and that
loader attempts to insert `pwm-fan.ko`.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_HWMON, CONFIG_PWM, CONFIG_SENSORS_PWM_FAN -->
<!-- source: gokrazy/kernel/runtime.require -- CONFIG_HWMON, CONFIG_PWM, CONFIG_SENSORS_PWM_FAN -->
<!-- source: internal/appliance/kernelreq.go -- runtimeKernelRequirements -->
<!-- source: gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/gokrazy.go -- Boot -->
<!-- source: gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/modules.go -- loadModules, loadModule -->

The native proof checks the emitted config and refuses preloaded modules
before VPN traffic. Source checks alone cannot prove that Kconfig kept the
symbols built in or that the guest reaches traffic with an empty loaded-module
list. Keep the strict no-modules guard; an absent `modprobe` helper alone does
not exclude gokrazy's direct module loader.
<!-- source: internal/appliance/kernelreq_test.go -- TestRuntimeFloorRequiresBuiltInPWMFan -->
<!-- source: docs/architecture/testing/qemu-integration.md -- VPN facilities without module loading -->

TCP-MD5 sessions require built-in `CONFIG_TCP_MD5SIG`. The runtime fragment
requests it and the runtime manifest verifies the emitted value. Ze configures
the socket through `setTCPMD5Sig`; a kernel without this feature refuses that
socket option with `ENOPROTOOPT`, before a signed connection can start. Linux
7.2 selects its built-in `CRYPTO_LIB_MD5` and `CRYPTO_LIB_UTILS` dependencies
from `TCP_MD5SIG`; enabling the separate `CRYPTO_MD5` module is not the fix.
<!-- source: internal/core/network/md5_linux.go -- setTCPMD5Sig -->
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_TCP_MD5SIG -->
<!-- source: gokrazy/kernel/runtime.require -- CONFIG_TCP_MD5SIG -->

A Linux Docker host boots the runtime kernel in place of its distro kernel
(`le setup docker kernel install`), so the profile also carries what such a
host relies on to come up. `CONFIG_HOTPLUG_PCI`, `CONFIG_HOTPLUG_PCI_ACPI` and
`CONFIG_HOTPLUG_PCI_PCIE` publish `/sys/bus/pci/slots`, from which udev names
an interface by its slot (`ens18` on QEMU and Proxmox). Without them udev names
it by path (`enp0s18`) and the host's network config, written for `ens18`,
matches nothing. Ubuntu's stock UFW rule set (`/usr/share/ufw/iptables`)
loads `limit`, `recent`, `hl`, `rt`, `LOG` and `REJECT` (IPv4 and IPv6)
beside the `multiport`, `comment`, `addrtype` and `conntrack` matches Docker
already needs; without them `ufw.service` fails with "Extension limit revision
0 not supported". `ipv6header` and `state`, which hand-written rule sets load,
were set only by the defconfig and are now requested too. Each is built in and
declared in `runtime.require`. `le setup docker kernel install` refuses a
cached kernel that lacks an option the host relies on, and
`TestHostDependencyOptionsAreInTheRuntimeProfile` fails when its tables name an
option this fragment does not set (`docs/architecture/testing/interop.md`).
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_HOTPLUG_PCI_ACPI, CONFIG_NETFILTER_XT_MATCH_LIMIT -->
<!-- source: gokrazy/kernel/runtime.require -- CONFIG_HOTPLUG_PCI_ACPI, CONFIG_NETFILTER_XT_MATCH_LIMIT -->
<!-- source: internal/le/setup/docker/kernel/hostdeps.go -- netfilterModuleOptions, ufwModules, slotNameOptions -->

The runtime fragment sets `CONFIG_LOCALVERSION="-ze"`, so the runtime kernel's
release is the pinned version with that suffix (`7.2.9-ze`) and its `/boot`
and `/lib/modules` entries never share a name with another kernel built from
the same upstream release. `le setup docker kernel install` reads the suffix
from the cache entry's emitted `config` and refuses a release that does not end
with it, because a reinstall removes the trees it replaces. The QEMU guest
release check accepts the suffixed release.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_LOCALVERSION -->
<!-- source: internal/le/setup/docker/kernel/dockerkernel.go -- cachedKernelRelease, kernelLocalVersion -->
<!-- source: internal/le/test/qemu/run_exec.go -- Run.assertRuntimeKernel -->


## Decisions

- The runtime profile's kernel is the only kernel an appliance image boots. The
  image build resolves it through the same path as `ze appliance kernel
  --target runtime`, so it passes the same `enforceKernelRequirements` check;
  there is no second check and no other kernel to check.
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
- `./le repo check` flags an exported symbol in a changed Go file that has no
  cross-package caller, even when the symbol predates the change. Appliance-only
  helpers stay unexported for that reason.

<!-- source: internal/appliance/cmd_kernel.go -- kernelInstallerConfigDir, defaultKernelBuild -->
<!-- source: internal/appliance/kernelbuilder/recipe.go -- Request.Root -->
<!-- source: internal/appliance/kernelreg_test.go -- TestRegisteredKernelProfilesShippedSet -->

## Related

- `build-artifacts.md` for how a built or downloaded kernel is resolved
