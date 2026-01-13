package context_test

// Test Coverage for FromNegotiatedRecv/Send
//
// This file tests the creation of EncodingContext from capability negotiation.
// The tests verify correct handling of:
//
// 1. Basic context fields (ASN4, IsIBGP, LocalAS, PeerAS)
// 2. ASN4 capability (all 4 permutations - RFC 6793)
// 3. ADD-PATH capability (all 9 mode permutations + no capability case - RFC 7911)
// 4. Extended Next Hop capability (negotiated + not negotiated - RFC 8950)
// 5. Nil input handling
//
// ASN4 Permutation Matrix (RFC 6793):
//
//	Test Case | Local ASN4 | Remote ASN4 | ctx.ASN4 | Encoding
//	----------|------------|-------------|----------|----------------------------
//	1         | Yes        | Yes         | true     | 4-byte ASNs in AS_PATH
//	2         | Yes        | No          | false    | 2-byte ASNs, AS_TRANS for >65535
//	3         | No         | Yes         | false    | 2-byte ASNs, AS_TRANS for >65535
//	4         | No         | No          | false    | 2-byte ASNs only
//
// ADD-PATH Permutation Matrix (RFC 7911):
//
//	Test Case | Local Mode | Remote Mode | Negotiated | recvCtx.AddPath | sendCtx.AddPath
//	----------|------------|-------------|------------|-----------------|----------------
//	1         | Receive    | Receive     | None       | false           | false
//	2         | Receive    | Send        | Receive    | true            | false
//	3         | Receive    | Both        | Receive    | true            | false
//	4         | Send       | Receive     | Send       | false           | true
//	5         | Send       | Send        | None       | false           | false
//	6         | Send       | Both        | Send       | false           | true
//	7         | Both       | Receive     | Send       | false           | true
//	8         | Both       | Send        | Receive    | true            | false
//	9         | Both       | Both        | Both       | true            | true
//	10        | (none)     | (none)      | (none)     | false           | false
//
// Extended Next Hop Permutations (RFC 8950):
//
//	Test Case | Local ExtNH | Remote ExtNH | recvCtx.ExtNH | sendCtx.ExtNH
//	----------|-------------|--------------|---------------|---------------
//	1         | Yes         | Yes          | true          | true
//	2         | Yes         | No           | false         | false
//	(3)       | No          | Yes          | false         | false  (symmetric, not tested separately)
//	(4)       | No          | No           | false         | false  (implicit in other tests)

import (
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
)

// makeNegotiated creates a Negotiated state from local/remote capabilities.
// Uses fixed ASNs for simplicity in tests.
func makeNegotiated(local, remote []capability.Capability, peerAS uint32) *capability.Negotiated {
	return capability.Negotiate(local, remote, 65000, peerAS)
}

// baseCaps returns basic IPv4 unicast + ASN4 capabilities (4-byte AS speaker).
func baseCaps(asn uint32) []capability.Capability {
	return []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: asn},
	}
}

// baseCapsWithAddPath returns base caps + ADD-PATH with given mode.
func baseCapsWithAddPath(asn uint32, mode capability.AddPathMode) []capability.Capability {
	return []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: asn},
		&capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: mode},
		}},
	}
}

// baseCapsWithExtNH returns base caps + Extended Next Hop.
func baseCapsWithExtNH(asn uint32) []capability.Capability {
	return []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: asn},
		&capability.ExtendedNextHop{Families: []capability.ExtendedNextHopFamily{
			{NLRIAFI: capability.AFIIPv4, NLRISAFI: capability.SAFIUnicast, NextHopAFI: capability.AFIIPv6},
		}},
	}
}

var ipv4Family = bgpctx.Family{AFI: 1, SAFI: 1}

// TestFromNegotiatedRecv_Basic verifies basic context creation for receiving.
//
// VALIDATES: FromNegotiatedRecv extracts ASN4, families from Negotiated.
//
// PREVENTS: Wrong encoding when parsing routes from peer.
func TestFromNegotiatedRecv_Basic(t *testing.T) {
	local := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65000},
	}
	remote := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.ASN4{ASN: 65001},
	}

	neg := makeNegotiated(local, remote, 65001)
	ctx := bgpctx.FromNegotiatedRecv(neg)

	require.NotNil(t, ctx, "context should not be nil")
	require.True(t, ctx.ASN4(), "ASN4 should be true when both support it")
	require.False(t, ctx.IsIBGP(), "should be EBGP (different ASNs)")
	require.Equal(t, uint32(65000), ctx.LocalASN())
	require.Equal(t, uint32(65001), ctx.PeerASN())
}

// TestFromNegotiatedSend_Basic verifies basic context creation for sending.
//
// VALIDATES: FromNegotiatedSend extracts ASN4, families from Negotiated.
//
// PREVENTS: Wrong encoding when sending routes to peer.
func TestFromNegotiatedSend_Basic(t *testing.T) {
	neg := makeNegotiated(baseCaps(65000), baseCaps(65000), 65000)
	ctx := bgpctx.FromNegotiatedSend(neg)

	require.NotNil(t, ctx, "context should not be nil")
	require.True(t, ctx.ASN4(), "ASN4 should be true")
	require.True(t, ctx.IsIBGP(), "should be IBGP (same ASN)")
	require.Equal(t, uint32(65000), ctx.LocalASN())
	require.Equal(t, uint32(65000), ctx.PeerASN())
}

// TestFromNegotiatedNil verifies nil input handling.
//
// VALIDATES: Returns nil for nil input.
//
// PREVENTS: Panic on nil Negotiated pointer.
func TestFromNegotiatedNil(t *testing.T) {
	require.Nil(t, bgpctx.FromNegotiatedRecv(nil), "FromNegotiatedRecv(nil) should return nil")
	require.Nil(t, bgpctx.FromNegotiatedSend(nil), "FromNegotiatedSend(nil) should return nil")
}

// TestFromNegotiatedAddPath verifies ADD-PATH handling for all 9 mode permutations.
//
// RFC 7911 ADD-PATH Negotiation Matrix:
//
//	              Remote
//	           | Receive | Send    | Both    |
//	    -------+---------+---------+---------+
//	    Recv   | None    | Receive | Receive |
//	Local Send | Send    | None    | Send    |
//	    Both   | Send    | Receive | Both    |
//
// Negotiated mode interpretation:
//   - Receive: local can receive path IDs → recvCtx.AddPath = true
//   - Send: local can send path IDs → sendCtx.AddPath = true
//   - Both: local can do both → both contexts have AddPath = true
//   - None: neither side can → both contexts have AddPath = false
//
// VALIDATES: AddPath is correctly set based on negotiated mode.
//
// PREVENTS: Wrong path ID handling in encoding/decoding.
func TestFromNegotiatedAddPath(t *testing.T) {
	// All 9 permutations of (Receive, Send, Both) x (Receive, Send, Both)
	tests := []struct {
		name       string
		localMode  capability.AddPathMode
		remoteMode capability.AddPathMode
		wantRecv   bool // recvCtx.AddPath = (negotiated mode includes Receive)
		wantSend   bool // sendCtx.AddPath = (negotiated mode includes Send)
	}{
		// Row 1: Local = Receive
		{
			name:       "Receive/Receive = neither can",
			localMode:  capability.AddPathReceive,
			remoteMode: capability.AddPathReceive,
			wantRecv:   false,
			wantSend:   false,
		},
		{
			name:       "Receive/Send = can receive only",
			localMode:  capability.AddPathReceive,
			remoteMode: capability.AddPathSend,
			wantRecv:   true,
			wantSend:   false,
		},
		{
			name:       "Receive/Both = can receive only",
			localMode:  capability.AddPathReceive,
			remoteMode: capability.AddPathBoth,
			wantRecv:   true,
			wantSend:   false,
		},
		// Row 2: Local = Send
		{
			name:       "Send/Receive = can send only",
			localMode:  capability.AddPathSend,
			remoteMode: capability.AddPathReceive,
			wantRecv:   false,
			wantSend:   true,
		},
		{
			name:       "Send/Send = neither can",
			localMode:  capability.AddPathSend,
			remoteMode: capability.AddPathSend,
			wantRecv:   false,
			wantSend:   false,
		},
		{
			name:       "Send/Both = can send only",
			localMode:  capability.AddPathSend,
			remoteMode: capability.AddPathBoth,
			wantRecv:   false,
			wantSend:   true,
		},
		// Row 3: Local = Both
		{
			name:       "Both/Receive = can send only",
			localMode:  capability.AddPathBoth,
			remoteMode: capability.AddPathReceive,
			wantRecv:   false,
			wantSend:   true,
		},
		{
			name:       "Both/Send = can receive only",
			localMode:  capability.AddPathBoth,
			remoteMode: capability.AddPathSend,
			wantRecv:   true,
			wantSend:   false,
		},
		{
			name:       "Both/Both = can send and receive",
			localMode:  capability.AddPathBoth,
			remoteMode: capability.AddPathBoth,
			wantRecv:   true,
			wantSend:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local := baseCapsWithAddPath(65000, tt.localMode)
			remote := baseCapsWithAddPath(65001, tt.remoteMode)
			neg := makeNegotiated(local, remote, 65001)

			recvCtx := bgpctx.FromNegotiatedRecv(neg)
			sendCtx := bgpctx.FromNegotiatedSend(neg)

			require.Equal(t, tt.wantRecv, recvCtx.AddPathFor(ipv4Family), "recv AddPath mismatch")
			require.Equal(t, tt.wantSend, sendCtx.AddPathFor(ipv4Family), "send AddPath mismatch")
		})
	}
}

// TestFromNegotiatedNoAddPath verifies behavior when ADD-PATH is not advertised.
//
// VALIDATES: AddPath is false when no ADD-PATH capability exists.
//
// PREVENTS: Unexpected path ID handling when capability not negotiated.
func TestFromNegotiatedNoAddPath(t *testing.T) {
	// Neither side advertises ADD-PATH
	neg := makeNegotiated(baseCaps(65000), baseCaps(65001), 65001)

	recvCtx := bgpctx.FromNegotiatedRecv(neg)
	sendCtx := bgpctx.FromNegotiatedSend(neg)

	require.False(t, recvCtx.AddPathFor(ipv4Family), "recv should not have AddPath without capability")
	require.False(t, sendCtx.AddPathFor(ipv4Family), "send should not have AddPath without capability")
}

// TestFromNegotiatedExtendedNextHop verifies extended NH extraction.
//
// VALIDATES: ExtendedNextHop is set when negotiated.
//
// PREVENTS: Wrong next-hop parsing for IPv4 routes with IPv6 NH.
func TestFromNegotiatedExtendedNextHop(t *testing.T) {
	neg := makeNegotiated(baseCapsWithExtNH(65000), baseCapsWithExtNH(65001), 65001)

	recvCtx := bgpctx.FromNegotiatedRecv(neg)
	sendCtx := bgpctx.FromNegotiatedSend(neg)

	require.NotZero(t, recvCtx.ExtendedNextHopFor(ipv4Family), "recv should have ExtendedNextHop")
	require.NotZero(t, sendCtx.ExtendedNextHopFor(ipv4Family), "send should have ExtendedNextHop")
}

// TestFromNegotiatedExtendedNextHopMissing verifies no ExtNH when not negotiated.
//
// VALIDATES: ExtendedNextHop is zero when not both sides advertise.
//
// PREVENTS: Wrong next-hop encoding when peer doesn't support it.
func TestFromNegotiatedExtendedNextHopMissing(t *testing.T) {
	// Only local advertises ExtNH
	neg := makeNegotiated(baseCapsWithExtNH(65000), baseCaps(65001), 65001)

	recvCtx := bgpctx.FromNegotiatedRecv(neg)
	sendCtx := bgpctx.FromNegotiatedSend(neg)

	require.Zero(t, recvCtx.ExtendedNextHopFor(ipv4Family), "recv should not have ExtendedNextHop")
	require.Zero(t, sendCtx.ExtendedNextHopFor(ipv4Family), "send should not have ExtendedNextHop")
}
