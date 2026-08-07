---
kind: note
level: MUST
stage:
---
A functional `.ci` test that boots a daemon (or runs `ze`) which
exercises a real Linux kernel feature -- netlink interface/VLAN/veth creation,
nftables, kernel sockets, L2TP/PPPoE kernel modules -- MUST be marked
`option=needs-linux`. Such a test cannot pass natively on darwin and must be
validated automatically inside the QEMU Alpine VM instead.
