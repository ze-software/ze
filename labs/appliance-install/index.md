Appliance lab

# Appliance Installer Evidence

The installer boots and completes for real in QEMU across HTTP/PXE, ISO, and Ventoy-on-FAT paths, plus failure-path and rescue scenarios.

`Appliance`

Four native QEMU actions cover the installer end to end: the PXE/HTTP chain boots the installer kernel and initrd, serves the image over HTTP, and asserts serial success markers before rebooting into the written disk and logging in over SSH. The ISO path wraps the same image in a bootable ISO and also verifies GPT layout and power-off-not-reboot behaviour. The Ventoy path proves the installer finds the appliance ISO as a *file* on a FAT/exFAT data partition rather than as burned boot media. The scenarios action covers what the other three do not: a forced mid-init panic recovering cleanly, MAC-based install pinning on multi-homed boxes, and rescue console access.

The current amd64 installation ISO is under 150 MB. The `prod` build with the hardware kernel profile on October 1, 2026 produced `ze-20261001-142644.iso`: 128,933,888 bytes (128.93 MB) for the complete ISO.

`ze appliance kernel` builds the installer kernel. Set `ZE_INSTALL_KERNEL` to its path for these QEMU tests.

- **Proves:** Installer boots and completes for real across HTTP/PXE, ISO, and Ventoy paths, plus fault, pinning, and rescue scenarios
- **Environment:** QEMU only, no Docker (the installer initrd is pure Go, no BusyBox)
- **Requires:** An operator-supplied installer kernel with IP_PNP_DHCP, VIRTIO_NET, VIRTIO_BLK, EXT4 built in
- **Source:** [docs/guide/ze-install.md](https://github.com/ze-software/ze/blob/main/docs/guide/ze-install.md)

```
# HTTP/PXE install chain
$ ./le test qemu install-test

# bootable ISO chain
$ ./le test qemu install-iso-test

# Ventoy ISO-on-FAT path
$ ./le test qemu install-ventoy-test

# fault/pin/rescue scenarios
$ ./le test qemu install-scenarios-test
```

`Prerequisites`

QEMU and an operator-supplied installer kernel (`ZE_INSTALL_KERNEL=/path/to/vmlinuz`); each native QEMU action reports a clear skip when none is usable.

- [internal/le/test/qemu native QEMU evidence](https://github.com/ze-software/ze/tree/main/internal/le/test/qemu)
