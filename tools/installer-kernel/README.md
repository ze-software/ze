# ze installer kernel

The PXE installer boots an operator-supplied Linux kernel alongside the pure-Go
installer initrd (a single static `cmd/ze-installer` binary, built by
`ze appliance initrd`; see `docs/guide/ze-install.md`). `ze` ships no
installer kernel, because the right kernel is site-specific. This directory
builds one suitable for the QEMU end-to-end test (`make ze-qemu-install-test`,
`test/install/qemu-full.ci`) and for real hardware PXE/ISO deployment.

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

| Backend | Variable | Prerequisites | Best for |
|---------|----------|---------------|----------|
| `qemu` (default) | `BUILDER=qemu` | qemu, python3, curl | Same-arch builds (near-native via HVF/KVM) |
| `docker` | `BUILDER=docker` | docker | Cross-arch builds (Rosetta 2 is much faster than TCG) |

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
make                                  # qemu builder, qemu profile, arm64
make BUILDER=docker                   # docker builder (faster cross-arch)
make PROFILE=hardware                 # real hardware, headless, arm64
make PROFILE=hardware-kms ARCH=amd64  # real hardware + i915 KMS, x86_64
make BUILDER=docker PROFILE=hardware-kms ARCH=amd64  # docker + hardware-kms
make PROFILE=hardware ARCH=amd64      # real hardware, headless, x86_64
make ARCH=amd64                       # qemu profile, x86_64
```

The kernel version is single-sourced in `internal/appliance/kernel.version` and
read by the driver; the Makefile no longer takes a version variable.

Output: `build/Image` (the kernel), `build/config` (the resolved config), and
`build/kernel.version` (a provenance sidecar recording what was built). If you
want to keep both architectures side by side, copy or rename the kernel after
each build and pass it back to `ze appliance iso --kernel ...`.

The Makefile calls the single shared driver `../kernel-builder/run.py` (the same
driver the runtime gokrazy kernel build uses), which selects Docker or QEMU and
invokes `build.py`. `ze appliance kernel` resolves the profile registry
(`<name>.config` + `<name>.require`, with optional one-level `# ze-base:`
layering and `# ze-include:` shared fragments), passes the resolved fragments to
the builder, then enforces the required symbols from Go. See `PROFILES.md` for
profile authoring.
The recipe can also be used directly inside any Linux environment (VM, container, bare metal):

```sh
python3 ../kernel-builder/build.py --src-dir /path/to/tools/installer-kernel --out-dir /path/to/output --fragment /path/to/tools/installer-kernel/kernel.config --fragment /path/to/tools/installer-kernel/qemu.config
```
<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile -->
<!-- source: internal/appliance/kernelreq.go -- enforceKernelRequirements -->
<!-- source: tools/kernel-builder/build.py -- main -->

## Use with the QEMU install test

```sh
ZE_INSTALL_KERNEL=$(pwd)/build/kernel/Image make ze-qemu-install-test
```

Without `ZE_INSTALL_KERNEL` the test self-skips: there is no safe default kernel.
