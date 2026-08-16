---
kind: table
level:
stage:
---
| The `.ci` test ... | Use |
|--------------------|-----|
| Only validates config (`ze config validate -`), parses, or runs offline `ze show`/`ze env` | nothing (runs natively on every OS) |
| Boots a daemon that **applies** Linux-only config (interface/VLAN, firewall, L2TP kernel) | `option=needs-linux` |
| Same, AND needs privileged network configuration (creates interfaces, brings links up, netlink) | `option=needs-linux:caps=net-admin` |
| Same, AND opens a raw/packet socket (`resolve ping`, traceroute: `net.ListenPacket("ip4:icmp", ...)`) | `option=needs-linux:caps=net-raw` |
| Same, AND loads eBPF | `option=needs-linux:caps=bpf` |
| Needs to skip only on a specific non-Linux OS for an unrelated reason | `option=skip-os:value=darwin` |
| Needs an OPTIONAL heavyweight artifact a checkout does not carry (the appliance module cache: `make ze-gokrazy-deps-download`) | `option=needs-path:value=<repo-rel>:hint=<cmd>` |
