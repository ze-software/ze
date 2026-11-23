Testing labs

# Testing labs.

Each lab runs Ze on your own machine so you can examine its behavior and repeat the checks.

Labs run against real third-party daemons (xl2tpd, accel-ppp, strongSwan), or boot Ze in a VM using Docker or QEMU. Some need Docker with host-kernel features Docker Desktop on macOS doesn't provide; a QEMU path is noted where one exists. The lab pages describe what each run checks and its limitations. Measurements of speed are covered under [Performance](https://ze-software.net/performance/).

## Available labs

### [BGP Protocol Interop](https://ze-software.net/labs/bgp-interop/)

`Daemon` `Docker`

- BGP sessions with **FRR, BIRD, and GoBGP**
- 423 scenarios across core protocol and **extensions**

### [netlab Topology Lab](https://ze-software.net/labs/netlab/)

`Daemon` `Docker` `QEMU`

- Three Ze nodes under **containerlab**, started by netlab
- Live check of **BGP, OSPF, IS-IS and BFD**, kernel routes and traffic

### [L2TP PPP/NCP Interop](https://ze-software.net/labs/l2tp-interop/)

`Daemon` `Docker` `QEMU`

- Ze as LNS vs real **xl2tpd**/pppd LAC
- FRR tests **BGP redistribution** from a live PPP session

### [PPPoE Interop](https://ze-software.net/labs/pppoe-interop/)

`Daemon` `Docker` `QEMU`

- Ze's PPPoE client vs real **accel-ppp** access concentrator
- **Discovery and PPP negotiation** checked against the access concentrator

### [IPsec / IKEv2 Interop](https://ze-software.net/labs/ipsec-interop/)

`Daemon` `Docker`

- Ze as IKE initiator vs real **strongSwan**/charon
- FRR **redistribute** scenarios over the tunnel

### [VLAN QoS Packet Tests](https://ze-software.net/labs/vlan-qos/)

`Daemon` `AF_PACKET`

- Packet captures check 802.1p **PCP tags**
- nftables counters check ingress **priority classification**

### [Looking Glass Graph Demo](https://ze-software.net/labs/looking-glass-graph/)

`Daemon` `Browsable`

- Realistic UK topology, real external **ASNs**
- A **browser graph** for inspecting AS paths

### [Appliance Installer Evidence](https://ze-software.net/labs/appliance-install/)

`Appliance` `QEMU`

- Installer boots and completes for real: **HTTP/PXE, ISO, Ventoy**
- Plus failure-path, fault, pin, and **rescue** scenarios

### [VPP Dataplane Evidence](https://ze-software.net/labs/vpp-dataplane/)

`Daemon` `Docker`

- Ze programs **FIB, traffic, and firewall** into a real VPP daemon
- Backs the production numbers in the **VPP guide**
