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
`DEVTMPFS_MOUNT`, `QUOTA` and `QFMT_V2`. The runtime target checks its runtime
floor instead, which also holds `QUOTA` and `QFMT_V2`.
These checks verify the artifact's configuration after the build.

Both quota options are built in for every kernel that mounts the persistent
partition read-write, the installer's and the runtime's: the partition carries
the ext4 quota feature with a project quota inode
(`internal/appliance/permformat`), and ext4 is recalled to refuse a read-write
mount of such a filesystem on a kernel without quota support. `QFMT_V2` is the
vfsv1 format of ext4's hidden quota inodes, and a module of it would never load.
`gokrazy/kernel/kernel.config` and `tools/installer-kernel/kernel.config`
request both, and their two `kernel.require` manifests require them.
<!-- source: gokrazy/kernel/kernel.config -- CONFIG_QUOTA, CONFIG_QFMT_V2 -->
<!-- source: tools/installer-kernel/kernel.config -- CONFIG_QUOTA, CONFIG_QFMT_V2 -->
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

## PPP original-resource ABI

`LL-PPP-ORIGINAL-RESOURCE` in the [Linux limitations register](../linux-limitations.md#ll-ppp-original-resource)
records why stock Linux 7.3 requires Ze's carried PPP original-resource patches.
Kernel configuration alone cannot provide this ABI. Admit the reviewed link
and address guard, route guard, family/alias correction, channel/unit guard and
atomic PPPoX capture together, in their registered series order; the capture
permission correction is part of that source set. A compiled artifact is not
native acceptance until the matching running image proves these operations.

A global compiled-capability check uses `PPPIOCGSUPPORT` on a held unbound
/dev/ppp file, without creating a unit, channel, transport or network link.
This ioctl is `_IOWR('t', 42, struct ppp_support)` (ARM64 `0xc010742a`).
The 16-byte, four-byte-aligned layout is u32 abi/features/reserved0/reserved1:
input ABI1 with every other field zero. Malformed input returns EINVAL,
a valid request on a bound file returns EBUSY, and the ordinary current network
namespace CAP_NET_ADMIN requirement is preserved. A successful query leaves
the file unbound and returns only the compiled feature declaration.
<!-- source: gokrazy/kernel/patches/0011b-ppp-global-support-query.patch -- ppp_support_ioctl, ppp_support -->

Its core mask is SETLINK, ALIAS, CONNECT and BRIDGE. ADDR4/ROUTE4 and
ADDR6/ROUTE6 are advertised only when the respective address family is built
in. CAPTURE is declared when PPPoX is compiled, including as a module; that
does not prove a module is loaded or that a transport exists. A full runtime
profile carrying both families and PPPoX declares bits0–10 (2047).
Bit10 is declared only if a supported address family is built in.
CONTROL6 bit9 requires built-in IPv6 and the complete original-unit receive,
fragment, transmit and multicast implementation. This is compiled support;
native acceptance and release acceptance require their own evidence.
<!-- source: gokrazy/kernel/patches/0013d-ppp-control-activation.patch -- PPP_OWNER_CONTROL6, ppp_support_ioctl -->

The global result is for capability checking only. It never authorizes a
mutation or replaces the exact live creator, channel or transport probes below.
Require the bits for each actual configured operation: SETLINK and ALIAS for
the session's link/metadata writers, CONNECT and CAPTURE for shared native
setup, BRIDGE and CAPTURE for the relay, and the corresponding ADDR/ROUTE pair
for each address family used. Pair deletion also requires EXACT_ADDR_PAIR
bit10 alongside that family's ADDR bit. A missing bit, ENOTTY or invalid ABI means
unsupported; it never enables an errno-based inference, numeric fallback or
unguarded write. Module loading and executed native acceptance are separate
from this declaration.
<!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- PPP_OWNER_EXACT_ADDR_PAIR, ppp_owner_ioctl -->

Every ABI structure uses fixed 32-bit fields and alignment four. ABI is one;
reserved fields and input feature fields are zero. `PPPIOCGOWNER` takes the
captured positive ifindex and nonnegative unit on the original creator file
and returns supported features. Its structure is 20 bytes. A separately opened
file attached to the same unit is refused; a true duplicate remains useful.
<!-- source: gokrazy/kernel/patches/0007-ppp-original-file-link-address-guard.patch -- ppp_owner, ppp_owner_ioctl -->

| Bit | Capability | Guarded operation |
|-----|------------|-------------------|
| 0 | SETLINK | MTU and IFF_UP through RTM_SETLINK |
| 1, 2 | ADDR4, ADDR6 | Native IPv4 and IPv6 address changes |
| 3, 4 | ROUTE4, ROUTE6 | Native IPv4 and IPv6 route add, replace and delete |
| 5 | ALIAS | Existing IFLA_IFALIAS session metadata writer |
| 6 | CONNECT | Exact channel/original-unit-file connect and disconnect |
| 7 | BRIDGE | Exact original channel-file pair bridge and unbridge |
| 8 | CAPTURE | Atomic original PPPoX channel capture into an unbound file |
| 9 | CONTROL6 | Native ICMPv6 raw and UDP6 sockets bound to the original PPP unit |
| 10 | EXACT_ADDR_PAIR | Exact retained LOCAL/ADDRESS/prefix deletion; preserve other IPv4 pairs |
<!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- PPP_OWNER_EXACT_ADDR_PAIR, original-resource capability bits -->
<!-- source: gokrazy/kernel/patches/0013d-ppp-control-activation.patch -- PPP_OWNER_CONTROL6 -->

Only built-in address families are advertised. The unit probe advertises bits
0–7 as implemented, bit9 with built-in IPv6 and bit10 when either family is
built in (full mask1791);
the live channel probe `PPPIOCGCHANOWNER` advertises bits
6–7.
<!-- source: gokrazy/kernel/patches/0013d-ppp-control-activation.patch -- ppp_owner_ioctl, PPP_OWNER_CONTROL6 -->

Live CAPTURE support is confirmed by the actual transport's non-mutating
`PPPIOCGTRANSPORTOWNER` probe, whose 20-byte layout is u32 abi/features,
s32 channel/unit, u32 reserved. All input fields except abi are zero; output
channel and unit (or -1 when unattached) are diagnostic labels only.
The transport probe does not perform the PPPoX handoff or drain queued packets.
<!-- source: gokrazy/kernel/patches/0011-pppox-original-channel-capture.patch -- ppp_channel_owner_info, pppox_ioctl -->

`PPPIOCATTCHANOWNER` is invoked on that original PPPoX socket with a
16-byte `ppp_fd_peer`: u32 abi, s32 fd, u32 flags/reserved. The fd names a
new unbound /dev/ppp file and may be zero. The kernel pins the socket's actual
native channel, checks its live state and socket namespace, preserves the
target namespace's CAP_NET_ADMIN permission, and transfers its held native
reference into the file before the usual queued-packet handoff. Unregister or
reconnect cannot rebind that file to a replacement channel.

The query can return `EBUSY` before filling its reply if the channel semaphore
is busy. Capture can return `EBUSY` before changing the file binding if the PPP
mutex is busy or the destination already has a binding. The kernel uses
trylocks under the transport socket lock to avoid waiting across the PPP to
RTNL lock order. Ze retries only on the retained original socket and its still
private, freshly opened destination file, which no other caller can bind.

The same peer layout supplies the original creator-unit file to
`PPPIOCCONNECTOWNER`/`PPPIOCDISCONNECTOWNER`, and the retained original
peer-channel file to `PPPIOCBRIDGEOWNER`/`PPPIOCUNBRIDGEOWNER`.
Cleanup validates the actual old relationship before list, count or reference
changes and joins native settlement. A retired original relationship may be
cleaned; a different current relationship is preserved. Cleanup uses its
retained initial capability receipt and files, rather than requiring a new
live-only probe after retirement.

Rtnetlink requests carry IFLA_OWNER_FD (70), IFA_OWNER_FD (13) or
RTA_OWNER_FD (33), a signed 32-bit original creator descriptor. The native
writer validates the exact file/device, socket namespace and captured index
under its actual writer locks. Route replacement retains replacement
semantics; every affected old and new leg is checked before effects.
Unsupported multipath/nexthop-ID forms and owner-tagged unsupported message
families are refused before dispatch or mutation. Unknown attributes on an
older kernel are insufficient: require a successful positive ABI probe before
any guarded write. ENOTTY, absent features or ABI rejection never permits a
legacy numeric or unguarded fallback.

Owner-tagged RTM_DELADDR requires both explicit IFA_LOCAL and IFA_ADDRESS
plus the exact retained prefix length. A local-only address uses identical
LOCAL and ADDRESS. Missing fields or an invalid prefix return EINVAL;
a nonmatching complete tuple returns EADDRNOTAVAIL; an ambiguous exact native
match returns EEXIST before effects. The original creator file/device,
namespace and index checks still apply under the actual writer locks.
<!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- inet_rtm_deladdr, inet6_rtm_deladdr -->

IPv4 compares the peer address itself, rather than merely its subnet, and
forces the native secondary-promotion path for this guarded deletion only.
Other pairs remain installed and native notifications/routes follow promotion;
the per-device, global and default configuration values are never toggled.
There is no name/procfs write or temporary restoration window. Untagged
deletion keeps its legacy subnet selection and configured cascade behavior.
<!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- __inet_del_ifa, inet_rtm_deladdr -->

IPv6 validates LOCAL, the stored effective peer and prefix under its native
locks before deletion. A local-only address's internal zero peer is normalized
to LOCAL for that comparison. Native deletion, notifications and associated
derived-temporary cleanup retain their existing semantics; unrelated ordinary
pairs are not selected. All native prefix lengths remain supported: IPv4
0–32 and IPv6 0–128. Provider cleanup must retain and send the actual complete
tuple and require its family bit plus bit10, without narrowing to host masks
or falling back to a weaker selector.
<!-- source: gokrazy/kernel/patches/0011c-ppp-exact-address-pair-delete.patch -- inet_rtm_deladdr, __inet_del_ifa, inet6_addr_del -->

Go producers must retain their strong original unit/channel owners through the
actual short syscall or netlink write, without holding a descriptor borrow over
an ACK wait. The shared setup supplier must replace PPPIOCGCHAN followed by
PPPIOCATTCHAN with atomic capture; the relay must retain both original channel
files through unbridge; the CPE CONNECT caller must supply the creator file.
The native session alias stamp is also a guarded writer. These requirements
do not claim that this ABI covers IPv6 carrier socket binding, multicast
membership or packet I/O; those actual consumers need their own authority
proof before shared PPP integration is accepted.

The complete IPv6 control constructor uses `PPPIOCSOCKETOWNER` at ioctl
number 43 with a 32-byte input: u32 abi/kind, s32 unit/ifindex, u32 flags and
three reserved u32 fields. Flags and reserved fields are zero. The constructor
supports native ICMPv6 raw (kind1) and UDP6 (kind2) sockets in the original
unit namespace, using ordinary userspace security checks. Success returns a new
nonblocking, close-on-exec socket descriptor; descriptor zero is valid.
The complete composed source dispatches this ioctl (`0x4020742b`) and advertises
CONTROL6 bit9 on global42 and live unit51 when IPv6 is built in. Modular or
unavailable IPv6 returns EAFNOSUPPORT before construction. The original creator,
namespace, permission, lifetime and IPv6-only checks remain mandatory.
<!-- source: gokrazy/kernel/patches/0012-ppp-control-owner-foundation.patch -- ppp_control_socket, ppp_control_socket_ioctl -->
<!-- source: gokrazy/kernel/patches/0013d-ppp-control-activation.patch -- PPPIOCSOCKETOWNER, ppp_control_socket_ioctl, ppp_support_ioctl, ppp_owner_ioctl -->

The constructor also requires the caller to be in the original unit's current
network namespace. A foreign caller receives EXDEV under the first RTNL lock,
before capability checks, owner acquisition or socket creation.
<!-- source: gokrazy/kernel/patches/0013da-ppp-control-current-namespace.patch -- ppp_control_socket_ioctl -->

The live control owner retains the logical PPP allocation without retaining
the creator file or a netdevice reference that delays unregister. Retirement
permanently removes device publication, waits a grace period, then releases
the logical PPP and provider references in core-owned work. Later socket or
fragment references retain only the small core wrapper; they cannot postpone
PPP teardown or form a packet-to-PPP-queue ownership cycle. Fragment accounting
covers that wrapper, not the transient retirement-work backlog. A permanent
unit latch records namespace departure and retirement from creation onward.
Packet receipts distinguish receive provenance from transmit restrictions;
their ownership survives extension sharing and copy-on-write. Socket lookup,
receive-copy, final output, multicast cleanup and all applicable fragment
queues must enforce that authority before the constructor can be enabled.

The receive preparation uses the existing original PPP unit's native IPv6
input after its final namespace scrub. That new physical input supersedes any
outer PPP transport provenance. Owned ICMPv6, DHCP server-port traffic and
extension or fragment candidates retain a receipt; ordinary direct TCP and
other UDP traffic remain untagged. The native IPv6 stack still validates and
reassembles packets. No second PPP reader or replacement wire parser is added.

Both local IPv6 and conntrack fragment queues retain the exact nullable PPP
owner and receive/transmit role, including global-address keys whose numeric
ingress index is zero. The existing XFRM gathering path uses those same
conntrack queues. Their native copy paths preserve receipts, and completion
checks the retained head's custody. Queue accounting includes retained owners
and extension storage alongside the existing XFRM scope and policy charges.

The control-socket contract keeps ICMPv6 and UDP sockets IPv6-only.
It supports ordinary IPv6 packet options, errors, path MTU, SO_REUSEADDR and
basic multicast join/leave on the original PPP unit. A join obtains the native
IPv6 device object from that retained unit while protected; leave and close
use that object too. Retirement cleanup frees local membership bookkeeping
without resolving the saved index against a replacement device.

Transmit captures the socket owner before LOCAL_OUT can orphan or reroute
the packet. A copied header bit keeps that requirement when an extension is
lost. Route output, postrouting completion, device handoff and redirect into
receive check the actual device. Native IPv6 fragments copy both the receipt
and the required bit. The original PPP writer checks again and consumes the
receipt before compression, queueing or lower transport, so ordinary forwarded
traffic and nested PPP transports keep their native behavior.

Owned socket PMTU, redirect, neighbor confirmation and route-cache writers
check their actual target against the original unit. Generic ICMP protocol
feedback before socket lookup remains native, including feedback for ordinary
forwarded WAN traffic. Receiving an ICMP packet on PPP does not give an old
socket authority over an independently selected route.

This constructor deliberately refuses IPv4 conversion, clearing V6ONLY,
HDRINCL, UDP encapsulation callbacks, SO_REUSEPORT and reuseport-BPF attachment,
SOCKMAP/SOCKHASH protocol replacement, source-specific multicast/filtering and
anycast options before they mutate state. Generic XDP redirect of a packet
still requiring original-PPP transmit custody also refuses before a non-skb
destination can discard that custody; XDP PASS and same-device TX remain native.
Plain sockets retain their existing options. These are the control constructor's
supported operations, not a promise of all Linux socket extensions.

The final activation patch is admitted only with the complete reviewed receive,
fragment, transmit and multicast source set. It enables matching-kernel native
proof, including useful original traffic and exact replacement discrimination.
Its compiled declaration does not prove those native results or release acceptance.
<!-- source: gokrazy/kernel/patches/0013d-ppp-control-activation.patch -- complete-source prerequisites, compiled declaration -->

The receive preparation checks an owned socket's immutable logical PPP binding
against the packet's retained RX receipt and actual ingress device before UDP6
hash selection, score comparison, early demux, GRO lookup, multicast cloning,
raw selection or queue admission. A prefetched, BPF-selected or reuseport-selected
socket that fails this check cannot consume the packet; ordinary eligible lookup
continues. Tuple-only lookups without a packet cannot select an owned socket.
Unowned sockets retain the native selection behavior.
<!-- source: gokrazy/kernel/patches/0013b-ppp-control-ipv6-receive.patch -- compute_score, udp6_lib_lookup4, udp6_lib_lookup_skb, __udp6_lib_lookup_owner, __udp6_lib_demux_lookup, udpv6_rcv, ipv6_raw_deliver -->

Receive-copy checks the retained receipt and current original owner again.
It does not read the queued packet's device field: UDP reuses that storage for
accounting scratch. Invalid peeked datagrams are removed through their native
queue-accounting path; a retired owned socket returns ESTALE before delivering
an old queue entry. The common UDP receive actor receives the same check.
UDP keeps the receipt only because its enqueue scrubs extensions through
`ppp_control_scrub` instead of resetting them all. A datagram that still carries
the receipt is then not marked stateless, so consuming it releases the
receipt's owner reference. Linux 7.3's own reset there erased the receipt, so
every datagram queued to an owned UDP6 socket was dropped at receive-copy.
A `MSG_PEEK` read leaves the receipt on the queued datagram, so the read that
follows it returns the same datagram and the queue keeps delivering.
The scrub runs for unowned UDP6 sockets too, so they keep the receipt as well.
A PPP unit with a control owner stamps a received IPv6 packet that the 0012
candidate filter matches: UDP to port 547, including DHCPv6 multicast to
`ff02::1:2`, which native delivery clones to every matching socket, and every
packet whose first next header is an IPv6 extension or fragment header. Such a
datagram queued to an unowned UDP6 socket holds the receipt, and with it the
owner reference that pins the logical PPP interface object and the provider
module, until that socket reads the datagram or closes.
<!-- source: gokrazy/kernel/patches/0013b-ppp-control-ipv6-receive.patch -- ppp_control_rx_match, udpv6_recvmsg, rawv6_recvmsg, udp_read_skb, udp_try_make_stateless -->
<!-- source: gokrazy/kernel/patches/0012-ppp-control-owner-foundation.patch -- ppp_control_rx_candidate, ppp_control_scrub -->
<!-- source: gokrazy/kernel/patches/0013a-ppp-control-rx-fragments.patch -- ppp_control_rx_stamp -->
<!-- test: internal/component/l2tp/ppp/kernel/created_unit_control_socket_receive_integration_linux_test.go TestCreatedUnitControlSocketNativeUDP6ReceivesPeerDatagram, TestCreatedUnitControlSocketNativeUDP6PeekKeepsReceipt -->

Received ICMP errors preserve their RX receipt through the native error clone.
Fresh local errors and path-MTU notifications receive a TX receipt from the
original live socket binding. Error-queue and path-MTU dequeue check that role
and owner before copying. The native checksum, extension, error reporting and
ancillary-message paths remain in place.
<!-- source: gokrazy/kernel/patches/0013b-ppp-control-ipv6-receive.patch -- ipv6_icmp_error, ipv6_local_error, ipv6_local_rxpmtu, ipv6_recv_error, ipv6_recv_rxpmtu -->

These receive guards form part of the complete control-socket contract with
transmit and multicast enforcement, owned route/cache writer guards for PMTU
and redirects, IPv6-only sockets and refusal of owned reuseport, socket-map
and family-escape operations before their native effects. The compiled
capability declaration does not establish native or release acceptance.
<!-- source: gokrazy/kernel/patches/0013b-ppp-control-ipv6-receive.patch -- private source preconditions, receive-only scope -->

`CreatedUnit.OpenIPv6ControlSocket` exposes this constructor to shared PPP
callers through the existing joined original-namespace helper. It validates the
kind and context, then runs creator-only query51 and constructor43 in one short
borrow of the same original file. It enters no namespace and duplicates no
creator file. A missing control capability refuses construction.
The caller MUST Close every nonnil returned control file, including descriptor
zero or a file accompanied by a late borrow or context error. The caller's
session/carrier adapter owns any `net.FilePacketConn` conversion and cleanup.
<!-- source: internal/component/l2tp/ppp/kernel/created_unit_control_socket.go -- CreatedUnit.OpenIPv6ControlSocket, IPv6ControlSocketKind -->
<!-- source: internal/component/l2tp/ppp/kernel/created_unit_control_socket_linux.go -- CreatedUnit.openIPv6ControlSocket, validateCreatedUnitControlOwner -->

The kernel session's IPv6 carrier is that adapter and the constructor's only
production caller. Inside one `WithOriginalNamespace` call it opens the ICMPv6
socket and the UDPv6 socket, sets `SO_REUSEADDR` and binds port 547 on the
UDPv6 descriptor, converts each with `net.FilePacketConn`, and closes every
returned file, on partial failure too. Its multicast joins pass interface
zero, which `ppp_control_mc_idev` resolves to the retained original device.
The carrier resolves no interface name and sets no `SO_BINDTODEVICE`: the
constructor's own device binding is the only device selection, and there is
no fallback to name-bound sockets.
<!-- source: internal/component/l2tp/ppp/kernel/ipv6_linux.go -- openIPv6Carrier, openOriginalIPv6Conn, prepareDHCPSocket -->
<!-- source: gokrazy/kernel/patches/0013c-ppp-control-ipv6-transmit.patch -- ppp_control_mc_idev, __ipv6_sock_mc_join -->

Constructor and option tests cover this file API and its refusal boundaries.
On the amd64 runtime kernel, the carrier owner roots prove all four traffic
lanes through a real PPPoE session, a rename before and after acquisition,
refusal of every stale read and write once the original retires and a
replacement takes its index and name, multicast cleanup, and socket
construction from the retained session rather than the caller's namespace.
The fragment roots prove useful fragmented UDP through the PPP owner, no
completion across two units sharing a tuple and fragment ID, no completion on
a replacement after retirement, one queue kept across a rename, and untouched
reassembly on ordinary Ethernet. Complete subscriber acceptance remains open.
<!-- test: internal/component/l2tp/pppoe/dataplane/kernel/native_ipv6_carrier_owner_integration_linux_test.go TestNativeIPv6CarrierOriginalTraffic, TestNativeIPv6CarrierRenameAfterAcquire, TestNativeIPv6CarrierAcquireAfterRename, TestNativeIPv6CarrierRetiredRS, TestNativeIPv6CarrierRetiredDHCP, TestNativeIPv6CarrierRetiredRAWrite, TestNativeIPv6CarrierRetiredDHCPWrite, TestNativeIPv6CarrierMulticastCleanup, TestNativeIPv6CarrierForeignCaller -->
<!-- test: internal/component/l2tp/pppoe/dataplane/kernel/native_ipv6_fragment_integration_linux_test.go TestNativeIPv6FragmentUseful, TestNativeIPv6FragmentDistinctOwners, TestNativeIPv6FragmentRetiredOwner, TestNativeIPv6FragmentRename, TestNativeIPv6FragmentOrdinaryEthernet -->

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
(the netlink API every SA install goes through), `CONFIG_XFRM_INTERFACE`,
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

Scoped IPsec SAs also require the carried state-scope, fragment-context and
fragment-guard patches. The runtime fragment and manifest require built-in
`CONFIG_XFRM_FRAGMENT_GUARD`; its Kconfig dependency requires built-in IPv6
and netfilter, and selects both family defrag implementations as built-ins.
The compiled runtime floor does not independently require this guard symbol.
Before every scoped SA installation, the dataplane reads the live namespace's
capability receipt and requires the exact ABI, 64-selector limit and feature
bits 0 through 3, bit 5, the ICMP reverse check that runs for error types
only and stops at an inbound discard entry the message matches as received
(`XFRM_ZE_CAP_ICMP_REVERSE`), bit 6, the forwarded ICMP error mapping that
tries the return SA when the out lookup of a forwarded error answers BLOCK
(`XFRM_ZE_CAP_ICMP_FORWARD_MAP`), and bit 7, the payload check of a forwarded
ICMP error that arrived on an SA and meets a flagged template-free forward allow
(an operator bypass's transit gate or forward decision) by its own headers
(`XFRM_ZE_CAP_ICMP_FORWARD_GATE`). A kernel with only part of that support cannot install a
scoped SA, even when its configuration passed the build's checks.
<!-- source: gokrazy/kernel/runtime.config -- CONFIG_XFRM_FRAGMENT_GUARD -->
<!-- source: gokrazy/kernel/runtime.require -- CONFIG_XFRM_FRAGMENT_GUARD -->
<!-- source: gokrazy/kernel/patches/series -- state-scope, fragment-context and fragment-guard patches -->
<!-- source: gokrazy/kernel/patches/0003-xfrm-state-scope.patch -- XFRM_ZE_CAP_ICMP_REVERSE, XFRM_ZE_CAP_ICMP_FORWARD_MAP, XFRM_ZE_CAP_ICMP_FORWARD_GATE -->
<!-- source: gokrazy/kernel/patches/0005-xfrm-fragment-guard.patch -- XFRM_FRAGMENT_GUARD -->
<!-- source: internal/appliance/kernelreq.go -- runtimeKernelRequirements -->
<!-- source: internal/component/ike/dataplane/xfrm_state_scope_linux.go -- checkXFRMStateScopeCapabilities -->

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
| An index file is present and cannot be read or parsed | Loads nothing, and warns with the file, the line and the fix (rebuild the image and push it, with the two `./ze-host appliance` commands) |
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
(USB) can register its interface after `ze start` lists the NICs. The
netlink monitor announces that interface, but the iface apply skips an `ethernet` entry
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
