Wire lab

# VLAN QoS Wire-Level Proof

Proves that VLAN 802.1p QoS maps work on the wire, not just in kernel state.

`Daemon`

Two network namespaces joined by a veth pair. One side has a VLAN sub-interface with egress/ingress QoS maps; the other is a raw capture/inject endpoint. Egress: send UDP with a given `SO_PRIORITY`, the egress map stamps the matching PCP bit in the 802.1Q header, and a capture on the peer side confirms it. Ingress: a raw 802.1Q frame with a given PCP arrives, the 8021q module strips the tag, and the ingress map translates PCP back to `skb->priority`, verified with an nftables counter.

**Limitations, stated by the lab itself:** proves behavior on veth pairs (software), not physical NICs with hardware VLAN offload. Single-tag 802.1Q only -- QinQ is out of scope. No throughput or latency assertions; this proves marking correctness, not QoS scheduling behavior.

- **Proves:** 802.1p PCP tagging/classification actually on the wire (AF_PACKET capture + nftables counters)
- **Scenarios:** Egress (PCP on the wire), Ingress (PCP classification), Full chain (DSCP to PCP)
- **Requires:** Linux with network namespace support, root (CAP_NET_ADMIN), iproute2, nftables
- **Source:** [internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go)

```
# run the wire-level Go integration proof (requires Linux root)
$ sudo -E env "PATH=$PATH" go test -tags integration -count=1 ./internal/plugins/iface/netlink -run TestVLANQoS
```

`Prerequisites`

Linux with network namespace support and root (CAP_NET_ADMIN). No Docker, no QEMU needed.

- [VLAN QoS integration proof Go producer and assertions](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go)
