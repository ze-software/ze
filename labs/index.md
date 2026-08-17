Testing labs

# Testing labs.

These aren't browser demos. Each lab runs Ze for real, on your own machine.

Labs run against real third-party daemons (xl2tpd, accel-ppp, strongSwan), or boot Ze in a VM using Docker or QEMU. Some need Docker with host-kernel features Docker Desktop on macOS doesn't provide; a QEMU path is noted where one exists. This is what "evidence over claims" means in practice. For speed rather than correctness, see [Performance](https://ze-software.net/performance/).

### [BGP Protocol Interop](https://ze-software.net/labs/bgp-interop/)

`Daemon` `Docker`

- Real **FRR, BIRD, and GoBGP** sessions, not mocks
- 68 scenarios across core protocol and **extensions**

### [L2TP PPP/NCP Interop](https://ze-software.net/labs/l2tp-interop/)

`Daemon` `Docker` `QEMU`

- Ze as LNS vs real **xl2tpd**/pppd LAC
- FRR proves **BGP redistribution** from a live PPP session

### [PPPoE Interop](https://ze-software.net/labs/pppoe-interop/)

`Daemon` `Docker` `QEMU`

- Ze's PPPoE client vs real **accel-ppp** access concentrator
- The dominant **open-source BRAS/AC**, not a stub

### [IPsec / IKEv2 Interop](https://ze-software.net/labs/ipsec-interop/)

`Daemon` `Docker`

- Ze as IKE initiator vs real **strongSwan**/charon
- FRR **redistribute** scenarios over the tunnel

### [VLAN QoS Wire-Level Proof](https://ze-software.net/labs/vlan-qos/)

`Daemon` `AF_PACKET`

- 802.1p **PCP tagging** actually on the wire
- Not just kernel-state **acceptance**

### [Looking Glass Graph Demo](https://ze-software.net/labs/looking-glass-graph/)

`Daemon` `Browsable`

- Realistic UK topology, real external **ASNs**
- The one lab that's actually **visual** today

### [Appliance Installer Evidence](https://ze-software.net/labs/appliance-install/)

`Appliance` `QEMU`

- Installer boots and completes for real: **HTTP/PXE, ISO, Ventoy**
- Plus failure-path, fault, pin, and **rescue** scenarios

### [VPP Dataplane Evidence](https://ze-software.net/labs/vpp-dataplane/)

`Daemon` `Docker`

- Ze programs **FIB, traffic, and firewall** into a real VPP daemon
- Backs the production numbers in the **VPP guide**
