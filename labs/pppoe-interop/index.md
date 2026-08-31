Protocol lab

# PPPoE Interop

Ze's PPPoE client against real `accel-ppp`, the dominant open-source BRAS/access concentrator.

`Daemon`

An L2TP interop test against accel-ppp isn't buildable: an L2TP tunnel needs exactly one LAC (initiator) and one LNS (responder), and both Ze and accel-ppp are LNS-only. PPPoE is where the two have complementary roles instead -- accel-ppp's first-class role is the PPPoE server (access concentrator), and Ze has a full RFC 2516 PPPoE client. This lab exercises exactly that pairing.

A peer-isolated Docker lab proves Ze's PPPoE client discovers, negotiates PPP, and establishes a session against a real accel-ppp AC.

- **Proves:** RFC 2516 PPPoE discovery + PPP negotiation against a real access concentrator, not a stub
- **Peer:** Real accel-ppp access concentrator
- **Requires:** Docker, or Linux root with accel-ppp and PPPoE kernel support
- **Source:** [docs/labs/pppoe-interop.md](https://github.com/ze-software/ze/blob/main/docs/labs/pppoe-interop.md)

```
# Docker lab
$ ./le deployment docker-pppoe-accel-test

# native Linux network-namespace path
$ ./le qemu pppoe-accel-test
```

`Prerequisites`

Docker for the peer-isolated lab. The native Linux action uses paired network namespaces and needs `ip`, `ping`, `accel-pppd`, and PPPoE kernel support.

- [test/interop-pppoe/ lab source](https://github.com/ze-software/ze/tree/main/test/interop-pppoe)
