Dataplane lab

# VPP Dataplane Evidence

Ze programs FIB, traffic, and firewall state into a real VPP daemon over GoVPP -- the evidence behind the production numbers quoted in the VPP guide.

`Daemon`

Runs a real VPP (FD.io Vector Packet Processor) daemon in Docker and proves Ze's `fib-vpp` plugin programs routes from Ze's system RIB directly into VPP's FIB via the GoVPP binary API, that per-interface and system-wide stats come back through the VPP stats segment, and that firewall state applies correctly.

This is the evidence behind the numbers already published in the VPP guide: on the reference hardware IPng Networks runs in production on AS8298, VPP forwards at roughly 35 Mpps on a 4-core Xeon D-1518 -- line rate where the default kernel-FIB path would be dropping packets.

- **Proves:** Real FIB/traffic/firewall programming into VPP via GoVPP, not a mocked binary API
- **Peer:** Real VPP daemon in a Docker container
- **Requires:** Docker with a privileged container
- **Source:** [docs/guide/vpp.md](https://github.com/ze-software/ze/blob/main/docs/guide/vpp.md)

```
# real VPP daemon in Docker, privileged container
$ make ze-deployment-vpp-test
```

`Prerequisites`

Docker with a privileged container (VPP needs DPDK-class access to devices).

- [VPP deployment reference IPng Networks production numbers](https://github.com/ze-software/ze/blob/main/docs/research/vpp-deployment-reference.md)
