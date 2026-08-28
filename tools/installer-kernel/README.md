# ze installer kernel

The PXE installer boots an operator-supplied Linux kernel alongside the pure-Go
installer initrd (a single static `cmd/ze-installer` binary, built by
`ze appliance initrd`; see `docs/guide/ze-install.md`). `ze` ships no
installer kernel, because the right kernel is site-specific. This directory
contains the profiles used by `ze appliance kernel` for QEMU tests and real
hardware PXE/ISO deployment.

## Why a purpose-built kernel

The initrd is a single static Go binary packed into a cpio with **no kernel
modules**. It relies on the
kernel having configured the network from `ip=dhcp` before `/init` runs, and it
mounts an ext4 `/perm` partition off a target disk. So NIC drivers, disk
drivers, ext4, initramfs, devtmpfs and IP autoconfiguration must all be
**built in** (`=y`). Stock distro/cloud kernels usually ship these as modules,
which is why they cannot boot this initrd.

## Profiles

The kernel config is split into a base (`kernel.config`) plus a profile:

| Profile | File | Drivers | Use case |
|---------|------|---------|----------|
| `qemu` (default) | `qemu.config` | virtio NIC + block | QEMU tests, fast build |
| `hardware` | `hardware.config` | virtio + EFI + Intel/Realtek/Broadcom/Mellanox NICs + AHCI + NVMe + framebuffer | Bare metal PXE/ISO install (headless / serial console) |
| `hardware-kms` | `hardware-kms.config` | hardware + Intel i915 KMS | Bare metal where a working monitor is needed |

### Monitor output on Intel Alder Lake-N and newer

On Intel Alder Lake-N / Twin Lake iGPUs (N100, N150, N200, N305), the firmware
drops display output at EFI ExitBootServices() handover. The `hardware` profile
includes `simpledrm` and `efifb`, which inherit the firmware's GOP framebuffer
but cannot reprogram the display controller. This matches normal Linux behaviour
without a full KMS driver: boot succeeds on serial, monitor stays black.

Operators who need monitor output at the rack should select `hardware-kms`, which
adds `CONFIG_DRM_I915=y` at the cost of ~50-80 MB kernel size and ~1-3 s extra
boot time.

## Build

Two build backends are available:

| Backend | Option | Prerequisites | Best for |
|---------|--------|---------------|----------|
| `qemu` | `--builder qemu` | qemu, go | Same-arch builds (near-native via HVF/KVM) |
| `docker` | `--builder docker` | docker | Cross-arch builds (Rosetta 2 is much faster than TCG) |

When the host architecture matches the target, QEMU uses HVF (macOS) or KVM
(Linux) for near-native speed. Cross-arch builds under QEMU use TCG (full
software emulation), which is slow. Docker Desktop on macOS uses Rosetta 2 for
cross-arch emulation, which is significantly faster.

The `ze appliance kernel` command auto-selects Docker if available, falling
back to QEMU. Override with `--builder docker` or `--builder qemu`.

Compilation results are cached via ccache in `tmp/qemu/ccache/` (QEMU backend).
The first build populates the cache; subsequent builds with the same kernel
version skip unchanged translation units.

```sh
ze appliance kernel --arch arm64 --profile qemu
ze appliance kernel --arch arm64 --profile hardware
ze appliance kernel --arch amd64 --profile hardware
ze appliance kernel --arch amd64 --profile hardware-kms --builder docker
```

The kernel version is single-sourced in `internal/appliance/kernel.version`.

Output: `build/kernel/Image` (the kernel), `build/kernel/config` (the resolved
config), and `build/kernel/kernel.version` (a provenance sidecar recording what
was built). The command also populates the target- and profile-specific XDG
cache used by later appliance operations.

`ze appliance kernel` resolves the profile registry (`<name>.config` +
`<name>.require`, with optional one-level `# ze-base:` layering and
`# ze-include:` shared fragments), passes the resolved fragments to the compiled
shared builder, then enforces the required symbols. See `PROFILES.md` for
profile authoring.
<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile -->
<!-- source: internal/appliance/kernelreq.go -- enforceKernelRequirements -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

## Use with the QEMU install test

```sh
ZE_INSTALL_KERNEL=$(pwd)/build/kernel/Image ze-test install --pattern qemu-full -a
```

Without `ZE_INSTALL_KERNEL` the test self-skips: there is no safe default kernel.
