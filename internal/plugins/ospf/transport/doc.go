// Package transport is the OSPFv2 raw IPv4 transport byte pipe.
//
// It owns socket lifecycle, multicast group membership, and payload delivery for
// IP protocol 89. It deliberately does not parse OSPF packet types, verify areas,
// checksums, or authentication. Those policies live in the OSPF instance/runtime
// children above the packet codec.
//
// The structure mirrors internal/plugins/isis/transport: a platform Backend opens
// per-interface handles, Transport tracks enabled interfaces, link events open and
// close handles, Receive delivers `(ifindex, src, payload)`, and SendPacket sends
// final OSPF bytes without altering them.
package transport
