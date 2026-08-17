Appliance lab

# Appliance Installer Evidence

The installer boots and completes for real in QEMU across HTTP/PXE, ISO, and Ventoy-on-FAT paths, plus failure-path and rescue scenarios.

`Appliance`

Four QEMU evidence scripts cover the installer end to end: the PXE/HTTP chain boots the installer kernel and initrd, serves the image over HTTP, and asserts serial success markers before rebooting into the written disk and logging in over SSH. The ISO path wraps the same image in a bootable ISO and also verifies GPT layout and power-off-not-reboot behaviour. The Ventoy path proves the installer finds the appliance ISO as a *file* on a FAT/exFAT data partition rather than as burned boot media. The scenarios script covers what the other three do not: a forced mid-init panic recovering cleanly, MAC-based install pinning on multi-homed boxes, and rescue console access.

The installer kernel is operator-supplied by design; Ze neither ships nor builds it. Point any script at one with `ZE_INSTALL_KERNEL`.

- **Proves:** Installer boots and completes for real across HTTP/PXE, ISO, and Ventoy paths, plus fault, pinning, and rescue scenarios
- **Environment:** QEMU only, no Docker (the installer initrd is pure Go, no BusyBox)
- **Requires:** An operator-supplied installer kernel with IP_PNP_DHCP, VIRTIO_NET, VIRTIO_BLK, EXT4 built in
- **Source:** [docs/guide/ze-install.md](https://github.com/ze-software/ze/blob/main/docs/guide/ze-install.md)

```
# HTTP/PXE install chain
$ make ze-qemu-install-test

# bootable ISO chain
$ make ze-qemu-install-iso-test

# Ventoy ISO-on-FAT path
$ make ze-qemu-install-ventoy-test

# fault/pin/rescue scenarios
$ make ze-qemu-install-scenarios-test
```

`Prerequisites`

QEMU and an operator-supplied installer kernel (`ZE_INSTALL_KERNEL=/path/to/vmlinuz`); each script self-skips with a clear message if none is usable.

- [scripts/evidence/effective-install-*.py evidence scripts](https://github.com/ze-software/ze/tree/main/scripts/evidence)
