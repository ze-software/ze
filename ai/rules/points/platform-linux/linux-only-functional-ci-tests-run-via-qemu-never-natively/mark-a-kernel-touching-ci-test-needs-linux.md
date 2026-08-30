---
kind: directive
level: MUST
stage:
---
**A functional `.ci` test that boots a daemon, or runs `ze`, against a real Linux kernel feature MUST carry `option=needs-linux`.** Netlink interface, VLAN and veth creation, nftables, kernel sockets and the L2TP or PPPoE kernel paths are all such features. The test cannot pass natively on darwin, and the marker is what routes it to the QEMU Alpine VM instead. Which marker each test needs, and how `caps=` narrows it, is `docs/architecture/testing/ci-format.md`.
