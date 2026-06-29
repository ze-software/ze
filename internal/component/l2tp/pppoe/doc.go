// Package pppoe implements the PPPoE (RFC 2516) access concentrator
// for subscriber session management over Ethernet. PPPoE is the
// direct-attach model: subscriber CPEs connect to the BNG without
// an intermediate L2TP tunnel.
//
// PPPoE has two phases:
//   - Discovery (ethertype 0x8863): PADI/PADO/PADR/PADS/PADT frames
//     on a raw AF_PACKET socket. Establishes a session ID.
//   - Session (ethertype 0x8864): PPP frames encapsulated in PPPoE
//     session headers. Handled by the kernel via AF_PPPOX/PX_PROTO_OE.
//
// The package is a peer of internal/component/l2tp and MUST NOT import
// it. Both transports feed sessions into the shared PPP driver
// (internal/component/l2tp/ppp) via ppp.StartSession.
//
// Architecture follows accel-ppp's proven model:
//   - Single AF_PACKET/SOCK_RAW socket per network namespace (shared
//     across all access interfaces)
//   - Per-interface server state (session table, SID bitmap, cookie
//     secret, rate limiter)
//   - Userspace dispatch by ifindex from recvfrom's sockaddr_ll
//
// Reference: RFC 2516 (PPPoE), RFC 1661 (PPP/LCP).
package pppoe
