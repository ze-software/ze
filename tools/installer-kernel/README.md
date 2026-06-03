# ze installer kernel

The PXE installer boots an operator-supplied Linux kernel alongside the busybox
initrd in `../installer-initrd` (see `docs/guide/ze-install.md`). `ze` ships no
installer kernel, because the right kernel is site-specific. This directory
builds one suitable for the QEMU end-to-end test (`make ze-install-qemu-test`,
`test/install/qemu-full.ci`) and as a reference for a real deployment.

## Why a purpose-built kernel

The initrd is a bare busybox cpio with **no kernel modules**. It relies on the
kernel having configured the network from `ip=dhcp` before `/init` runs, and it
mounts an ext4 `/perm` partition off a virtio disk. So virtio-net, virtio-blk,
ext4, initramfs, devtmpfs and IP autoconfiguration must all be **built in**
(`=y`). Stock distro/cloud kernels usually ship these as modules, which is why
they cannot boot this initrd. `kernel.config` is the fragment that forces the
required options on; `build.sh` verifies they resolved to `=y` before building.

## Build

```sh
make                      # build/Image for arm64
make ARCH=amd64           # build/Image for x86_64
make LINUX_VERSION=6.12.9 # pin a different kernel
```

Output: `build/Image` (the kernel) and `build/config` (the resolved config).
If you want to keep both architectures side by side, copy or rename the kernel
after each build and pass it back to `ze appliance iso --kernel ...`.

## Use with the QEMU install test

```sh
ZE_INSTALL_KERNEL=$(pwd)/tools/installer-kernel/build/Image make ze-install-qemu-test
```

Without `ZE_INSTALL_KERNEL` the test self-skips: there is no safe default kernel.
