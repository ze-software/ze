# Hardware kernel profiles

A kernel profile is a pair of files in an open registry: `<name>.config` holds
the fragment, `<name>.require` holds the manifest of symbols the build must
prove. Names accept safe tokens only. One optional `# ze-base: <profile>` line
gives a single level of layering, and nesting is rejected.

<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile, registeredKernelProfiles, validateKernelProfileName -->
<!-- source: internal/appliance/kernelreq.go -- requirement enforcement over the emitted config -->
<!-- source: internal/appliance/cmd_kernel.go -- fragment ordering, builder invocation, verification -->

## Go owns the guarantee

`ze appliance kernel` defaults to the installer target. For a local build, it
resolves the fragments, invokes Docker or QEMU with an explicit `--fragment`
order, and reads the emitted configuration. The installer target checks the profile's
manifest and the compiled floor of `IP_PNP_DHCP`, `EXT4_FS`, `BLK_DEV_INITRD`,
and `DEVTMPFS_MOUNT`. The runtime target checks its runtime floor instead.
These checks verify the artifact's configuration after the build.
<!-- source: internal/appliance/cmd_kernel.go -- kernelTargetFor, resolveInstallerKernel, resolveRuntimeKernel -->
<!-- source: internal/appliance/kernelreq.go -- universalKernelRequirements, runtimeKernelRequirements -->

The worker starts with Linux `defconfig`, merges the resolved fragments, and
runs `olddefconfig`. The installer `qemu` fragment requests virtio drivers;
drivers inherited from `defconfig` can also remain enabled. The emitted
configuration records the complete selection.
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

For a named appliance, `image.kernel-profile` selects the installer profile.
Disk images resolve the shared `runtime` profile for `image.arch` separately,
so the installer profile does not change the disk image's runtime drivers.
<!-- source: internal/appliance/cmd_kernel.go -- runKernel -->
<!-- source: internal/appliance/kernelargs.go -- resolveBuildParentDir -->
<!-- source: internal/appliance/runtimekernel.go -- RuntimeKernelTree -->

Library callers supply the source root. The resolver reads profiles, their
base/shared includes and requirement manifests under that root. It preserves
absolute manifest operands through post-build enforcement, while cache hashing
and the native builder receive repository-relative labels for the same inputs.
The native request also carries the root explicitly, rather than deriving it
from whichever directory the caller happens to occupy.

`internal/appliance/kernelbuilder` owns backend selection, downloads, verified
Alpine ISO caching, Docker and QEMU lifecycle, and the low-level kernel build.
The low-level Go worker invokes the kernel's `merge_config.sh`, `patch`, and
`tar`. The container and QEMU guest run the compiled `ze-kernel-builder`
command.

Each low-level worker creates its own `worker-*` directory beneath its build
directory (`/tmp/kbuild` by default). Extraction, configuration and compilation
stay inside that directory, so a second patched build cannot remove or reuse a
live worker's source tree. When `RunWorker` returns, after its commands finish,
it removes only its own directory; a cleanup failure is returned alongside any
build failure. Docker's shared build and work volumes remain mounted, and
downloaded source tarballs and installer build archives remain in the shared
work directory (`/build` by default).
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker, restoreOrExtractTree -->
<!-- test: internal/appliance/kernelbuilder/workspace_test.go TestRunWorkerConcurrentPatchedBuildsKeepLiveTree -->

Each Docker worker is created, without starting it, under a unique per-run name.
Creation has a detached 2-minute bound (`containerLifecycleTimeout`), so
canceling the build cannot kill the CLI before it returns the full container ID.
The bound is not the 15-second host-probe bound: creating the ~717 MB builder
container took 25 seconds on a loaded host whose daemon answered `docker info`
in 0.1 seconds. Ze starts and attaches to that ID, then uses the same detached
2-minute bound to force-remove that exact container after the CLI finishes, on
success, failure or cancellation. A create or removal that runs out of it is
reported as having had no answer within the 2-minute limit. A successful daemon
removal joins the worker before output ownership repair or disk reclaim. The
ownership-repair container follows the same create, attach and remove lifecycle.
Ze never finds workers by image ancestry and never prunes another build's work.
If creation or removal cannot be confirmed, the build refuses without ownership
repair or disk reclaim; its error names the exact container identity and the
inspection or removal command. Killing the Docker CLI alone is not evidence
that a daemon worker stopped.

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
| The kernel container builds the kernel | the build output above. Ze removes the exact owned container before repairing output ownership or reclaiming disk space. If removal fails or cannot be confirmed, every stop, including Ctrl-C, names that failure and gives `docker rm --force <full-container-id>`; repair and reclaim do not run |
| Go compiles the QEMU kernel worker | the same Go command, with its `GOOS` and `GOARCH`, to run from the repository root |
| The build VM boots, logs in, or sets up SSH | the QEMU command that boots the same VM to the same stage |
| The host downloads the kernel tarball for the build VM | the `curl` command that fetches it by hand to the path the next build reads it from, through a `.part` file, so a fetch cut short leaves no partial tarball |
| The build VM builds the kernel | the build output above, and `ssh -t <options> -p <port> root@localhost top` to watch the next build, with the options the build logs in with (`sshClientOptions`) and the port that run prints on its `>>> booting Alpine VM` line. The QEMU command alone boots a VM with no build in it, so it is not offered here |
| Any other step, and the choice of builder | `Build` names the stop itself, with what the step said as evidence and the build output above as the next step (`stopReport`). It does so for every backend error and for `SelectBuilder`'s, whose accelerator probe runs `sysctl` on a Mac, so no stop from either is reported as a library's error alone |

A download has no limit of its own, only the build's 2 hours: a stalled mirror
is reported when the build runs out of them, at the download step, with its URL.
The i915 firmware blobs of the `hardware-kms` profile are fetched at their
pinned commits (below) before the build starts, each within 10 minutes
(`downloadTimeout`). A firmware download
that fails or runs out of that time names its URL and gives the `curl` command
that fetches the blob by hand to where the next run looks for it. Every limit a
build names is worded by one formatter (`kernelbuilder.LimitWords`): "the
2-hour limit" for the kernel build, "the 10-minute limit" for a firmware
download and for the ISO tools, and "within 15s" for each question the build
asks the host first (`hostProbeTimeout`: QEMU's `-accel help` and
`docker info`). Creating and removing a build container are daemon work, not
questions, and have "the 2-minute limit" (`containerLifecycleTimeout`).

An arm64 kernel is built on the Mac and an amd64 kernel on the Linux host.

<!-- source: internal/appliance/cmd_kernel.go -- kernelBuildTimeout, defaultKernelBuild, downloadFirmwareBlob, firmwareDownloadFailed -->
<!-- source: internal/appliance/kernelbuilder/stopped.go -- WithBuildLimit, buildLimit, LimitWords, errBuildStopped, stopReport -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build, SelectBuilder, validateRequest, runDocker, runContainer, containerLifecycleTimeout, namedLifecycleOverrun, containerFailure, repairOwnership -->
<!-- source: internal/appliance/kernelbuilder/space.go -- dockerRootDir -->
<!-- source: internal/appliance/kernelbuilder/qemu.go -- hostProbeTimeout, hardwareAccelerator, probeKVM, probeHVF, availableAccelerators, errBootFailed, buildGuestWorker, qemuNextStep, guestBuildNextStep, sshClientOptions, ensureAlpineISO, downloadNextStep, fetchKernelTarball, tarballNextStep -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

`ze appliance kernel` selects the profile and architecture, validates the
result against the registry, and uses the shared builder in
`internal/appliance/kernelbuilder`.

The kernel source is pinned by exact release and digest.
`internal/appliance/kernel.version` names the release, and
`kernelSourceSHA256` in the worker holds the SHA-256 kernel.org publishes for
that tarball in `sha256sums.asc`. The worker refuses a version with no tracked
digest before downloading, and refuses a tarball, downloaded or found in the
work directory, whose digest differs, naming both digests;
`TestKernelVersionHasDigestPin` fails when the pinned release has no digest.
The patch series applies with `patch --fuzz=0`, so a bump the series no longer
fits fails the build instead of placing a hunk by guesswork; a bump refreshes
the series against the new release.
<!-- source: internal/appliance/kernelbuilder/worker.go -- kernelSourceSHA256, verifyKernelSource, applyPatches -->

The firmware the `hardware-kms` profile compiles in is pinned the same way.
`hardwareKMSFirmware` in the worker names each blob with the linux-firmware
commit it is fetched at and its SHA-256 at that commit, and the host fetches
`linux-firmware.git/plain/<path>?id=<commit>`, never the head of the branch.
`ensureFirmware` moves a download into the firmware cache
(`~/.cache/ze/firmware/`) only when its digest matches; a refused download
leaves nothing cached and names the URL, both digests and the
`curl -fL <url> | sha256sum` that compares by hand. Before each build it checks
a cached blob again and refuses one that differs, naming the file, both
digests and the `rm` after which the next build fetches the pinned blob. The
worker checks each blob once more before `CONFIG_EXTRA_FIRMWARE` names it, so a
`--firmware-dir` given to `tools/kernel-builder` directly is held to the same
pins, and the provenance record of a `hardware-kms` kernel carries one
`firmware-url` and one `firmware-sha256` line per blob. The pins sit in a
`BuildSources` file, so a re-pin moves the kernel cache key.
`TestHardwareKMSFirmwareIsPinned` fails on a blob with no commit or no digest.
A re-pin takes the last commit in the file's linux-firmware log and the digest
of the file fetched at it.
<!-- source: internal/appliance/kernelbuilder/worker.go -- hardwareKMSFirmware, FirmwareBlob, FileSHA256, embedFirmwareBlobs, verifyFirmwareBlob -->
<!-- source: internal/appliance/kernelbuilder/recipe.go -- writeProvenance -->
<!-- source: internal/appliance/cmd_kernel.go -- ensureFirmware, ensureFirmwareBlob, downloadFirmwareBlob -->
<!-- test: internal/appliance/firmware_test.go TestHardwareKMSFirmwareIsPinned, TestFirmwareDownloadRefusesWrongBytes, TestCachedFirmwareIsVerifiedBeforeUse, TestEnsureFirmwareRefusesCorruptedCache -->
<!-- test: internal/appliance/kernelbuilder/firmware_test.go TestEmbedFirmwareRefusesUnpinnedBlob, TestWriteProvenanceRecordsFirmware -->

A bump also owes a pass over the
[Linux limitations register](../linux-limitations.md): re-read each `kernel`
entry at the new release and update its `Checked` line.
`TestLinuxLimitsCheckedAtShippedVersions` reads the register through the shared
`linuxlimit.Parse` parser and fails while any entry names another release.
<!-- source: internal/appliance/linux_limits_test.go -- TestLinuxLimitsCheckedAtShippedVersions -->
<!-- source: internal/le/linuxlimit/parse.go -- Parse -->

## Built in or loadable

The runtime profile is one kernel for hardware, QEMU and Xen. Its rule:

| Kind | Value | Examples |
|------|-------|----------|
| A networking feature Ze configures | Built in (`=y`) | Routing, firewall, IPsec, WireGuard, MPLS, traffic control, PPP/L2TP, tunnels |
| A virtual or boot-critical device | Built in (`=y`) | virtio, the Xen guest frontends, NVMe, SATA/AHCI, SCSI disk, USB host and storage, HID core with generic and USB HID, the watchdogs, PWM fan control, KVM, the console |
| A driver for physical hardware | Loadable (`=m`) | Physical NICs and their PHYs, wireless, sound, i915 graphics, PC Card, vendor HID devices, board sensors, laptop platform drivers |
| A facility no Ze feature uses at boot, which a host may want | Loadable (`=m`) | The NFS client with its RPC and lock layers, software RAID, device mapper |

A guest then holds only the drivers of the devices it has. A networking
feature never depends on module loading. A hardware driver reaches its device
only when a loader inserts it: gokrazy's boot loader inserts only
`pwm-fan.ko`, which this profile builds in. Modules are not signed.
<!-- source: gokrazy/kernel/runtime.config -- loadable hardware drivers, CONFIG_MODULES -->

The last row is the same rule applied to software. Ze mounts no NFS export,
assembles no RAID array and maps no device, so the appliance never loads
these modules; a Docker or lab host that boots this kernel loads one when it
needs it. `CONFIG_NFS_FS`, `CONFIG_NFS_V3` and `CONFIG_NFS_V4` are requested
as modules, and the promptless RPC and lock layers follow them.
`CONFIG_ROOT_NFS` depends on `NFS_FS=y`, so it drops out and this kernel
cannot mount its root file system over NFS. `CONFIG_BLK_DEV_MD` and
`CONFIG_BLK_DEV_DM` are requested as modules, and their children follow.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_NFS_FS, CONFIG_BLK_DEV_MD, CONFIG_BLK_DEV_DM -->

The profile turns `CONFIG_KALLSYMS_ALL` off and keeps `CONFIG_KALLSYMS`. The
kernel's symbol table then holds function names and no data symbols, so the
backtrace in a captured oops or panic still names its functions.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_KALLSYMS_ALL -->
<!-- source: gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/modules.go -- loadModules -->

**Built in by decision.** Three facilities stay built in although no Ze
feature configures them, and the configuration base stays `defconfig`:

- **Kernel tracing.** The tracing support (`CONFIG_FTRACE`, with kprobe and
  uprobe events) and `CONFIG_KPROBES` stay built in: they are how a deployed
  box is diagnosed live. The fragment requests none of them. The amd64
  `defconfig` enables them, and the profile does not turn them off.
- **KVM host support.** `CONFIG_KVM` is requested built in, with its Intel and
  AMD parts on x86. Nested QEMU labs on the [docker-host image](docker-host.md)
  need it, and nothing in that image loads a module for a device or a CPU: it
  runs no udev and does not link Ze's coldplug loader, and its busybox
  `modprobe` loads only a module that a script names.
- **SELinux.** `CONFIG_SECURITY_SELINUX` stays built in, as the amd64
  `defconfig` sets it: a distribution host that boots this kernel
  (`le setup docker kernel install`) can run an SELinux userland.
- **The `defconfig` base.** The runtime kernel is not built up from an empty
  configuration. One kernel must also boot physical hardware, and `defconfig`
  supplies the generic platform support for it that the fragment does not
  list: on x86, ACPI, PCI, SMP and the serial console.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_KVM, CONFIG_KVM_INTEL, CONFIG_KVM_AMD; no tracing, SELinux, ACPI or PCI request -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->
<!-- source: cmd/ze-docker-host/main.go -- main -->
<!-- source: cmd/ze/setup_features_appliance.go -- the coldplug blank import, under ze_appliance -->
<!-- source: internal/appliance/dockerhost/boot.go -- writableOverlays -->
<!-- source: internal/appliance/dockerhost/guest_linux.go -- no udev in the guest -->

The `=m` lines name only drivers with a Kconfig prompt. Promptless helpers
(`LIBIE`, `PHY_PACKAGE`, `SND_PCM`, `DRM_TTM`, `INTEL_GTT`) follow the drivers
that select them, and an `=m` line cannot demote a symbol a built-in one
selects. `CONFIG_DRM` and `CONFIG_AGP` stay built in: the EFI console fragment
asks for `CONFIG_DRM_SIMPLEDRM=y`, which depends on `DRM`, and in Linux 7.3
`DRM` depends on `AGP || AGP=n`, so a modular `AGP` would make `DRM` and
simpledrm modular too. Only `CONFIG_DRM_I915` and the AGP chipset drivers are
loadable.
<!-- source: tools/kernel-builder/common/efi-console.config -- CONFIG_DRM_SIMPLEDRM -->
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_DRM_I915, CONFIG_AGP_INTEL -->

Xen support is guest side only: `CONFIG_XEN`, the PVH entry
(`CONFIG_XEN_PVH`) and the HVM guest (`CONFIG_XEN_PVHVM_GUEST`), the Xen
console (`CONFIG_HVC_XEN`) and the block and network frontends. Legacy PV,
dom0, every backend and the toolstack's userspace devices are turned off,
because `XEN_PV` is `default y` once `XEN` is set and dom0 and the backends
default from it. `runtime.require` names `XEN`, `HVC_XEN` and the two
frontends. It cannot name `XEN_PVH` or `XEN_PVHVM_GUEST`: they exist on x86
only, and one manifest applies to both architectures. No Xen boot of this
kernel has been proven.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_XEN, CONFIG_XEN_PV -->
<!-- source: gokrazy/kernel/runtime.require -- CONFIG_XEN, CONFIG_XEN_NETDEV_FRONTEND -->
<!-- source: internal/appliance/kernelconfig_pairing_test.go -- unverifiedRuntimeSymbols -->

## Runtime requirements

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
`SHA1`, `SHA256` and `SHA512`. Like every networking feature, these never
depend on module loading. The runtime fragment requests each one, and the runtime
manifest and compiled floor require each one. The transform-to-symbol map is
`xfrmTransformKernel`, and
`TestXfrmTransformsHaveRequiredKernelSymbol` walks the algorithm tables and
fails when a transform's symbol is not `=y` in the fragments or not in
`runtime.require`, so a cipher added to a table alone fails a test. The AES,
AUTHENC, ECHAINIV, SEQIV, HMAC and SHA256 templates the transforms also use are
selected by `INET_ESP` through `XFRM_ESP` in Linux 7.3.
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
not exclude gokrazy's direct module loader. With loadable hardware drivers the
guard holds only while the guest gets no device whose driver is loadable: its
NIC, disks and console are virtio devices, whose drivers are built in.
<!-- source: internal/appliance/kernelreq_test.go -- TestRuntimeFloorRequiresBuiltInPWMFan -->
<!-- source: internal/le/test/qemu/dockerlab.go -- dockerLabVM.argv -->
<!-- source: internal/le/test/qemu/vpnproof.go -- runVPNKernelHere -->
<!-- source: docs/architecture/testing/qemu-integration.md -- VPN facilities without module loading -->

TCP-MD5 sessions require built-in `CONFIG_TCP_MD5SIG`. The runtime fragment
requests it and the runtime manifest verifies the emitted value. Ze configures
the socket through `setTCPMD5Sig`; a kernel without this feature refuses that
socket option with `ENOPROTOOPT`, before a signed connection can start. Linux
7.3 selects its built-in `CRYPTO_LIB_MD5` and `CRYPTO_LIB_UTILS` dependencies
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
release is the release the version `internal/appliance/kernel.version` pins
builds, with that suffix (`<release>-ze`), and its `/boot` and `/lib/modules` entries never share a name with another kernel built from
the same upstream release. `le setup docker kernel install` reads the suffix
from the cache entry's emitted `config` and refuses a release that does not end
with it, because a reinstall removes the trees it replaces. The QEMU guest
release check accepts the suffixed release. The release and the pin differ for
a series' first release: kernel.org names that tarball with two components
(`linux-7.3.tar.xz`), while the kernel's Makefile always prints
`VERSION.PATCHLEVEL.SUBLEVEL`, so pin `7.3` builds release `7.3.0` and its
modules tree is `7.3.0-ze`. A pin with three components builds itself.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_LOCALVERSION -->
<!-- source: internal/le/test/deployment/gokrazykernel.go -- KernelRelease, versionProblems -->
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
  downloads the i915 firmware at its pinned commit, verifies its SHA-256 and
  passes it, then enforces that `CONFIG_EXTRA_FIRMWARE` is set in the emitted
  config.

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

## Loading driver modules on the device

The appliance runs no udev and no modprobe. gokrazy's own loader loads one
module it names in code, and nothing else, so a device whose driver the kernel
builds as a module has no driver until something loads it. Ze loads the modules
the devices present at boot need, at `ze start`, before anything lists the
NICs.
<!-- source: gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/modules.go -- loadModules, loadModule -->
<!-- source: internal/plugins/coldplug/coldplug.go -- coldplug -->

**When it runs.** `ze start` runs every step registered in `cmd/ze/startprep`
before it opens its store, so before `bootstrapFromDiscovery` and before the
iface component's apply in `hub.Run` enumerate interfaces. The start path names
no step. The appliance flavor links the loader by a blank import in
`cmd/ze/setup_features_appliance.go`, and the loader runs only when
`ze.gokrazy.enabled` is set, which `gokrazy/ze/config.json` does. A
`ze_appliance` binary started anywhere else loads nothing.
<!-- source: cmd/ze/startprep/startprep.go -- Register, Run -->
<!-- source: cmd/ze/ze_core_start.go -- cmdStart -->
<!-- source: internal/plugins/coldplug/register_linux.go -- coldplugOnAppliance -->

**What one pass does.** The pass reads the module tree of the running kernel,
`/lib/modules/<release>`, which the kernel build installs into the image.

| Input | How the pass reads it |
|-------|-----------------------|
| Every `modalias` file under `/sys/devices` | One alias per device. Links are not followed, because sysfs links each device back up the tree |
| `modules.alias` | Each pattern is a shell glob read as `fnmatch(3)` with no flags, as kmod reads it: `*`, `?`, `[...]` with ranges and `!` or `^`, and `\`. A `/` is an ordinary character. Every module whose pattern matches a device is loaded |
| `modules.dep` | A module's dependencies load before it, each after its own dependencies, whatever order the line lists them in |
| `modules.softdep` | `pre:` modules load before the module, `post:` modules after it. A soft dependency the index does not hold is skipped |
| `modules.builtin`, `/proc/modules` | A module built into the kernel or already loaded is skipped |

Each module loads with `finit_module(2)`, with no parameters and no flags. A
kernel answer of `EEXIST` counts as loaded.
<!-- source: internal/plugins/coldplug/sysfs.go -- walkDevices -->
<!-- source: internal/plugins/coldplug/glob.go -- aliasMatch -->
<!-- source: internal/plugins/coldplug/index.go -- readIndex -->
<!-- source: internal/plugins/coldplug/coldplug.go -- planner.add, runPlan -->
<!-- source: internal/plugins/coldplug/finit_linux.go -- finitModule -->

**Faults never stop the boot.** Each fault costs only the devices that need
the module. Every warning carries the doctor code `doctor-module-missing`, and
a warning about one module names it and the device alias that asked for it.

| Case | What the pass does |
|------|--------------------|
| No `/lib/modules/<release>`, or no `modules.alias` or `modules.dep` in it | Nothing, logged at debug: a kernel with every driver built in looks like this |
| An index file is present and cannot be read or parsed | Loads nothing, and warns with the file, the line and the fix (rebuild the image) |
| A module does not load | Warns with the file and the kernel's error, and names `show system kernel-log level warning` for the module's own reason. The pass goes on. A module that needs it is not tried, and is reported with the module that blocked it |
| A device alias names a module the index does not hold | Warns, and goes on |
| The index names a compressed module (`.ko.zst`, `.ko.xz`, `.ko.gz`) | Warns that Ze loads only uncompressed modules, and goes on |
<!-- source: internal/plugins/coldplug/coldplug.go -- coldplug, planner.add, runPlan -->

**A NIC that appears after the pass.** `finit_module` returns after the
module's init has run. A PCI NIC driver binds its device and registers its
interface from the probe that init starts, and the kernel probes synchronously
unless the driver or the `async_probe` module option asks otherwise, so the
interface exists when the pass ends. That reading of `do_init_module`
(`kernel/module/main.c`) and `driver_allows_async_probing`
(`drivers/base/dd.c`) is not verified against the 7.3 source in this
repository; the QEMU trial with an `e1000` module settles it. A driver that
defers its probe, waits for firmware, or sits on a bus that enumerates later
(USB) can register its interface after `ze start` lists the NICs. The netlink
monitor announces that interface, but the iface apply skips an `ethernet` entry
whose device is absent ("configured interface not present, skipping"), and only
the next commit applies it. The pass has no hotplug listener: a device that
appears after the pass gets no module from it.
<!-- source: internal/plugins/iface/netlink/monitor_linux.go -- monitor.handleLinkUpdate -->
<!-- source: internal/component/iface/config_apply.go -- applyConfigPass, the skip of an absent ethernet entry -->

**Why Ze loads the modules and not gokrazy's init.** The pass runs before
anything in Ze enumerates interfaces, which is the only ordering it needs, and
gokrazy starts no service on this image that uses a NIC. In Ze it is tested Go
against fake trees, and the gokrazy diff (`gokrazy-build-pins.md`) does not
grow.

**Not built.** A hotplug listener for uevents, compressed modules, module
signing, the `modprobe.d` blacklist and module options, and a `ze doctor` check
that reports a present device whose module exists but is not loaded.
