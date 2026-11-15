Appliance lab

# Appliance Installer Evidence

Run the installer in QEMU through HTTP/PXE, ISO, or Ventoy-on-FAT paths, with separate failure-path and rescue scenarios. A successful action verifies its selected path; a skipped action supplies no boot evidence.

`Appliance`

Four native QEMU actions exercise the installer end to end: the PXE/HTTP chain boots the installer kernel and initrd, serves the image over HTTP, and asserts serial success markers before rebooting into the written disk and logging in over SSH. The ISO path wraps the same image in a bootable ISO and also verifies GPT layout and power-off-not-reboot behaviour. The Ventoy path proves the installer finds the appliance ISO as a *file* on a FAT/exFAT data partition rather than as burned boot media. The scenarios action covers what the other three do not: a forced mid-init panic recovering cleanly, MAC-based install pinning on multi-homed boxes, and rescue console access.

Until a release publishes an appliance image, build one with the appliance guide. The current amd64 installation ISO is under 150 MB. The `prod` build with the hardware kernel profile on October 1, 2026 produced `ze-20261001-142644.iso`: 128,933,888 bytes (128.93 MB) for the complete ISO.

From the source checkout, prepare the build-host command with `./le build host-driver`, then use `./ze-host appliance kernel` to build the installer kernel. Set `ZE_INSTALL_KERNEL` to its printed path for these QEMU tests. The appliance guide covers the image and native-architecture build prerequisites.

- **Proves:** A passing action verifies its selected install, fault, pinning, or rescue path in QEMU
- **Environment:** The boot checks use QEMU; building their kernel or image also needs a supported build backend. The installer initrd is pure Go.
- **Requires:** A matching native-architecture installer kernel with the selected media, network, and disk drivers built in; see the installer profiles in the guide
- **Source:** [docs/guide/ze-install.md](https://github.com/ze-software/ze/blob/main/docs/guide/ze-install.md)

```
# HTTP/PXE install chain
$ ./le test qemu installer

# bootable ISO chain
$ ./le test qemu installer-iso

# Ventoy ISO-on-FAT path
$ ./le test qemu installer-ventoy

# fault/pin/rescue scenarios
$ ./le test qemu installer-scenarios
```

`Prerequisites`

QEMU and an operator-supplied installer kernel (`ZE_INSTALL_KERNEL=/path/to/vmlinuz`); each native QEMU action reports a clear skip when none is usable.

- [internal/le/test/qemu native QEMU evidence](https://github.com/ze-software/ze/tree/main/internal/le/test/qemu)
