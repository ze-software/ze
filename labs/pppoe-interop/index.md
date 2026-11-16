Protocol lab

# PPPoE Interop

This lab connects Ze's PPPoE client to the `accel-ppp` access concentrator.

`Daemon`

In this pairing, accel-ppp is the PPPoE server (access concentrator) and Ze is the client. Their L2TP implementations both take the LNS responder role, so they cannot form an L2TP tunnel with each other. PPPoE gives them complementary roles for testing discovery and PPP negotiation.

A peer-isolated Docker lab tests Ze's PPPoE client discovers, negotiates PPP, and establishes a session against a real accel-ppp AC.

- **What this lab tests:** RFC 2516 PPPoE discovery and PPP negotiation with accel-ppp
- **Peer:** Real accel-ppp access concentrator
- **Requires:** Docker, or Linux root with accel-ppp and PPPoE kernel support
- **Source:** [docs/labs/pppoe-interop.md](https://github.com/ze-software/ze/blob/main/docs/labs/pppoe-interop.md)

```
# Docker lab
$ ./le deployment docker-pppoe-accel

# native Linux network-namespace path
$ ./le qemu pppoe-accel
```

`Prerequisites`

Docker for the peer-isolated lab. The native Linux action uses paired network namespaces and needs `ip`, `ping`, `accel-pppd`, and PPPoE kernel support.

- [test/interop-pppoe/ lab source](https://github.com/ze-software/ze/tree/main/test/interop-pppoe)
