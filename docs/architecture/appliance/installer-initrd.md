# Installer initrd: a busybox-free PID 1

The installer initrd is one static Go binary. `cmd/ze-installer` builds it, the
packer writes it into the cpio archive as `/init`, and the kernel runs it as
PID 1. There is no busybox, no `/bin/sh`, and no shell script. It replaced a
1176-line busybox `/bin/sh` initrd.

<!-- source: cmd/ze-installer/main.go -- installer initrd entry point, build tag ze_installer -->
<!-- source: internal/install/disk/initrd_linux.go -- RunInitrd, the PID-1 entry point -->
<!-- source: internal/appliance/cmd_initrd.go -- defaultInitrdMakeBuild and writeInitrdPack, the cpio packer -->

Everything a shell would have given you must therefore exist in Go: mount,
console fan-out, block-device and loop ioctls, DHCP, and netlink. Each one has
its own file under `internal/install/disk`.

<!-- source: internal/install/disk/mount_linux.go -- mount and umount through unix syscalls -->
<!-- source: internal/install/disk/console_linux.go -- fan-out writer across every console -->
<!-- source: internal/install/disk/blockdev_linux.go -- block device ioctls -->
<!-- source: internal/install/disk/loop_linux.go -- loop device attach and detach -->
<!-- source: internal/install/disk/dhcp_linux.go -- single-shot DHCPv4 through nclient4 -->
<!-- source: internal/install/disk/netlink_linux.go -- link, address, and route control -->

## Detect state through procfs and sysfs

The initrd reads kernel state out of `/proc` and `/sys` and programs it through
netlink. It runs no external tool, and `internal/install/disk` and
`cmd/ze-installer` contain no `exec.Command` of an external binary.

| Need | Source | Producer |
|------|--------|----------|
| List the interfaces | `os.ReadDir("/sys/class/net")` | `fallbackDHCP`, `bringUpAllNICs`, `internal/install/disk/network.go` |
| Wait for carrier or link | `/sys/class/net/<if>/carrier` | `waitForCarrier`, `internal/install/disk/network.go` |
| Report operstate and MAC in the rescue shell | `/sys/class/net/<if>/operstate`, `/address` | `internal/install/disk/rescue_linux.go` |
| Decide whether the image server is reachable | An HTTP probe of the image URL, not a route table read | `probeServer`, `waitForServer`, `internal/install/disk/network.go` |

The connectivity decision is an HTTP probe on purpose. A default route says a
route exists; it does not say the image server answers, and the server answering
is what the install needs.

## In-process replacements, one per shell tool

| Operation | Use | Never |
|-----------|-----|-------|
| Bring a link up, apply an address and a route | `github.com/vishvananda/netlink` behind `netlinkOps` (`internal/install/disk/netlink_linux.go`) | `ip link set`, `ip addr`, `ip route` |
| DHCP lease | In-process `nclient4` (`internal/install/disk/dhcp_linux.go`) | `udhcpc` plus a lease script |
| HTTP image or database download | `net/http` (`internal/install/disk/download.go`) | `wget`, `curl` |
| mount, umount, loop, block-device ioctls, reboot, poweroff | `golang.org/x/sys/unix` syscalls and ioctls, each isolated in a named `_linux.go` helper (`mount_linux.go`, `loop_linux.go`, `blockdev_linux.go`, `unix.Reboot` in `rescue_linux.go`) | `mount`, `losetup`, `reboot` |

The installer uses the upstream `vishvananda/netlink` package directly. It does
not import `internal/plugins/iface/netlink`: the initrd is a separate binary
with its own build tag, and the plugin tree is not in it.

`./le qemu install-test` proves the initrd boots and installs cleanly.

<!-- source: internal/install/disk/network.go -- waitForCarrier, fallbackDHCP, probeServer, waitForServer -->
<!-- source: internal/install/disk/netlink_linux.go -- netlinkOps, realNetlinkOps -->
<!-- source: internal/install/disk/rescue_linux.go -- the sysfs interface report and unix.Reboot -->

## Host binaries and target binaries

The initrd binary is a TARGET binary. It is cross-compiled
`GOOS=linux GOARCH=<arch> CGO_ENABLED=0` and it runs on the appliance, never on
the build host. `CLAUDE.md`, "Binary naming convention", holds the full rule.

**Never cross-compile a host binary.** A native build or test action that must RUN
`ze appliance ...` on the build host compiles `cmd/ze` for the host and names
the result `ze-host`. Give that build a target `GOARCH` and it cannot exec:
the failure is "exec format error", and it looks like a broken build rather
than a wrong tag. Apply `GOARCH=<target>` only to the build of a target binary,
or to the `ze appliance initrd` invocation that cross-compiles one internally.

## PID 1 cannot exit

PID 1 exiting panics the kernel, so a fatal error routes to a recovery console
instead. `fatalInitrd` picks one of three branches: a token-gated rescue shell,
an ungated one, or a reboot. `selectFatalBranch` decides from the kernel
cmdline, and the gated branch reads its credential through `rescueauth`.

<!-- source: internal/install/disk/rescue_linux.go -- fatalInitrd, selectFatalBranch, rescueOnConsoles -->
<!-- source: internal/core/rescueauth/rescueauth.go -- rescue-shell credential encoding -->

A goroutine panic kills PID 1 the same way, so every goroutine the initrd
starts is guarded by `recover` into `fatalInitrd`. The evidence for that guard
is a real injected runtime fault, not a test double: a nil-map write behind the
`ze_installer_fault` build tag. The shipping initrd compiles the stub instead,
so it carries no fault code.

<!-- source: internal/install/disk/fault_linux.go -- maybeInjectFault, build tag ze_installer_fault -->
<!-- source: internal/install/disk/fault_stub_linux.go -- the no-op the shipping initrd compiles -->

## Traps

**The initrd cache key ignores build tags.** `initrdCacheVariant` keys on
version, architecture, and a hash of a fixed source-file list. A fault build and
a normal build therefore collide on one cache path, and a fault build can be
served a cached normal initrd. The QEMU harness isolates them with a per-variant
`XDG_CACHE_HOME`. If you ever add a real build-tag dimension to the initrd, fold
it into the cache key rather than working around it again.

<!-- source: internal/appliance/cache.go -- initrdCacheVariant, initrdSourceFiles -->

**A bootstrap installer resolves interfaces directly, and that is correct.** The
no-direct-resolution gate flags `netlink.LinkByName`, and the answer is an
allowlist entry, not a resolver call. The initrd must not pull the `iface`
component: it is a self-contained PID 1 binary. `internal/plugins/provision/`
carries the same exemption for the same reason.

<!-- source: internal/le/ifaceresolution/ifaceresolution.go -- Answer -->

**arm64 `virt` has no IDE bus.** The ISO cdrom must attach as virtio-scsi, not
`if=ide`. You can prove the attachment parses without a bootable image: run
`qemu-system-aarch64` with the devices under a timeout and watch it reach UEFI
firmware. A timeout kill with no device error means the wiring is valid.

**Multi-NIC pin tests need a connected route, not a metric.** Put `ze.server` on
the recovery NIC's directly connected subnet. The connected route always beats
any default route the pinned or foreign NIC holds, so reachability does not
depend on default-route ordering after the DHCP fallback re-leases every NIC.

<!-- source: internal/le/qemu/actions.go -- Answer -->


## What is not proven here

The QEMU install gates self-skip without an operator-supplied installer kernel
(`ZE_INSTALL_KERNEL`) and without `grub-mkstandalone`, `xorriso`, and `mtools`.
The initrd is statically verified: build, vet, unit tests, and lint. Treat the
QEMU acceptance path as unproven until these run green on a machine that has a
kernel and those tools:

```
ZE_INSTALL_KERNEL=/path/to/vmlinuz ./le qemu install-scenarios-test
ZE_INSTALL_KERNEL=/path/to/vmlinuz ./le qemu install-ventoy-test
ZE_INSTALL_ARCH=arm64 ZE_INSTALL_KERNEL=/path/to/Image ./le qemu install-iso-test
```

The first-run risks, most likely first: `nclient4` DHCP behavior under QEMU
slirp, the `mon:stdio` keystroke path in the rescue driver, `mformat` flags and
`vdb` device ordering for Ventoy, then arm64 GRUB `arm64-efi` availability.

## Related

- `on-device-installer.md` for `ze install disk` and the boot-time repair path
- `iso-installer.md` for the ISO transport that carries this initrd
- `build-artifacts.md` for how the kernel and the initrd are resolved
