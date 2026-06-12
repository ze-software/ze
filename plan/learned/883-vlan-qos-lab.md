# 883 -- VLAN QoS Lab

## Context

The spec-vlan-qos-map feature adds VLAN 802.1p QoS maps (PCP-to-priority and
priority-to-PCP) through Ze's config path. The kernel-state integration tests
(TestVLANQoSMapIntegration) proved the maps are accepted by the kernel, but not
that the PCP bits actually appear on the wire or that ingress classification
produces the expected skb->priority. This lab proves wire-level behavior using
AF_PACKET capture and nftables counters.

## Decisions

- AF_PACKET with manual TCI bit extraction over tcpdump text parsing: 4 lines of bit arithmetic, unit-testable on any host, no text format dependency
- Single netns with veth pair for QEMU tests over two-netns topology: Go test threading with LockOSThread makes multi-netns complex; static ARP neighbor avoids local routing ambiguity
- nftables `meta priority` counter for ingress verification over tc filter hit counters: more reliable on Alpine, already in the QEMU package list, avoids iproute2 `tc basic` ematch availability uncertainty
- Negative controls (AC-2, AC-4) as first-class test cases over positive-only: a capture that always reports PCP 6 due to a decode bug would pass positive tests; the negative control catches it
- TCI helpers in a separate file without build tag (`vlanqoslab_tci_test.go`) over everything in `integration && linux`: enables `go test` on macOS to verify the decode logic
- Manual lab selftest uses `ip link` commands over ze daemon: faster CI smoke; ze config path proven by `test/plugin/iface-vlan-qos.ci`

## Consequences

- veth pairs on Linux do NOT have hardware VLAN offload (ethtool shows `rx-vlan-offload: off [fixed]`), so disabling offloads (spec R-1) is unnecessary and the test works without it
- The nftables approach for ingress verification means the QEMU integration test needs the `nft` binary; the existing `--packages "nftables"` in mk/test-integration.mk already provides it
- Kernel ingress QoS map sets skb->priority in `vlan_do_receive()` before the IP stack, so nftables prerouting chain with `iifname "ze0.100"` and `meta priority 6` correctly observes the mapped priority
- `type route hook output priority mangle` is needed (not `type filter`) for the full-chain test because `meta priority set` requires a chain that can modify routing-adjacent metadata

## Gotchas

- AF_PACKET capture on a veth picks up non-VLAN frames too (IPv6 NDP, etc.); must filter for TPID 0x8100 AND EtherType 0x0800 in the capture loop
- When testing both mapped and unmapped priorities in the same test function, residual frames in the AF_PACKET buffer from the first send can confuse the second capture; drain the buffer between assertions
- The destination for egress UDP send must have a static ARP neighbor entry; without it, the kernel sends ARP requests instead of the UDP frame and the capture timeout fires
- `htons` is needed for AF_PACKET socket protocol and bind; Go's `unix.ETH_P_ALL` is in host byte order

## Files

- `internal/plugins/iface/netlink/vlanqoslab_tci_test.go` (new): decodeTCI, buildTaggedFrame, TestDecodeTCI, TestBuildTaggedFrame
- `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` (new): 3 QEMU scenarios + AF_PACKET/nftables helpers
- `test/vlan-qos-lab/run.sh` (new): manual lab runner with --selftest
- `test/vlan-qos-lab/ze-vlan-qos.conf` (new): reference Ze config
- `test/vlan-qos-lab/README.md` (new): topology, scenarios, limitations
- `docs/functional-tests.md` (modified): added dataplane integration row
