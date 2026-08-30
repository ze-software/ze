---
kind: directive
level: MUST
stage:
---
**An initrd operation MUST use its in-process Go replacement; it MUST NOT shell out to an external tool.** That covers bringing a link up, applying an address and a route, the DHCP lease, the image download, and mount, umount, loop, block-device ioctls, reboot and poweroff. The replacement for each operation, and the procfs or sysfs source for each state read, is `docs/architecture/appliance/installer-initrd.md`.
