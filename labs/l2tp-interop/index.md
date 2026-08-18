Protocol lab

# L2TP PPP/NCP Interop

Ze as an LNS against a real `xl2tpd`/`pppd` LAC, with FRR proving that a subscriber route makes it all the way into BGP.

`Daemon`

A peer-isolated Docker lab runs Ze as an LNS, a real `xl2tpd`/`pppd` LAC, and FRR as a BGP peer in separate privileged containers on an isolated bridge network. It proves the complete path from L2TP control tunnel through PPP LCP/IPCP, kernel `pppN` interface creation, dataplane connectivity, and BGP route redistribution from a live PPP session.

Two scenarios: full PPP/IPv4 dataplane proof with cleanup verification, and a subscriber /32 BGP redistribution scenario -- advertised to FRR when the session comes up, withdrawn on teardown.

- **Proves:** Full L2TP control tunnel, PPP LCP/IPCP negotiation, kernel pppN creation, BGP redistribution
- **Peer:** Real xl2tpd/pppd (LAC), optionally FRR as BGP peer
- **Requires:** Docker and a host kernel with PPPoL2TP support (Docker Desktop on macOS typically lacks this -- use the QEMU path instead)
- **Source:** [docs/labs/l2tp-interop.md](https://github.com/ze-software/ze/blob/main/docs/labs/l2tp-interop.md)

```
# Docker lab, all scenarios
$ make ze-deployment-docker-l2tp-ppp-test

# single scenario, verbose
$ python3 test/interop-l2tp/run.py 01-ppp-ipv4

# QEMU path instead of Docker (needs tmp/kernel/vmlinuz)
$ make ze-qemu-l2tp-ppp-test
```

`Prerequisites`

Docker for the primary path. The QEMU path needs a staged kernel (`make ze-kernel-build GOKRAZY_ARCH=arm64`) and works where Docker Desktop's VM lacks the PPPoL2TP kernel module.

- [test/interop-l2tp/ lab source](https://github.com/ze-software/ze/tree/main/test/interop-l2tp)
