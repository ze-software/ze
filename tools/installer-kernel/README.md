# ze installer kernel

The PXE installer boots an operator-supplied Linux kernel alongside the busybox
initrd in `../installer-initrd` (see `docs/guide/ze-install.md`). `ze` ships no
installer kernel, because the right kernel is site-specific. This directory
builds one suitable for the QEMU end-to-end test (`make ze-install-qemu-test`,
`test/install/qemu-full.ci`) and for real hardware PXE/ISO deployment.

## Why a purpose-built kernel

The initrd is a bare busybox cpio with **no kernel modules**. It relies on the
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

```sh
make                                  # qemu profile, arm64 (default)
make PROFILE=hardware                 # real hardware, headless, arm64
make PROFILE=hardware-kms ARCH=amd64  # real hardware + i915 KMS, x86_64
make PROFILE=hardware ARCH=amd64      # real hardware, headless, x86_64
make ARCH=amd64                       # qemu profile, x86_64
make LINUX_VERSION=6.12.9             # pin a different kernel
```

Output: `build/Image` (the kernel) and `build/config` (the resolved config).
If you want to keep both architectures side by side, copy or rename the kernel
after each build and pass it back to `ze appliance iso --kernel ...`.

## Use with the QEMU install test

```sh
ZE_INSTALL_KERNEL=$(pwd)/tools/installer-kernel/build/Image make ze-install-qemu-test
```

Without `ZE_INSTALL_KERNEL` the test self-skips: there is no safe default kernel.
