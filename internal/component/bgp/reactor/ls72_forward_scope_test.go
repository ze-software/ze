// Design: docs/architecture/bgp/structural-forwarding.md -- LS joint-subnet admission
// Related: docs/architecture/wire/nlri-bgpls.md -- LS72 24/48-octet fields
// Related: ls72_forward_framing_test.go -- complete received UPDATE and writer oracles
// RFC 9552 Section 5.5 -- see rfc/short/rfc9552.md
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// LS72 must grow24-to48 and trim48-to24 under the joint-subnet rule after
// mode and policy selection. The split-prefix case keeps the recipient on a
// speaker subnet while the effective global is elsewhere: the VPN128 peer-only
// exception must not leak into this RD-framed family. Every positive observes
// useful exact NLRI output; the mixed policy case also requires its IPv4 sibling.
func TestLSVPNForwardScope(t *testing.T) {
	for _, mode := range ls71PairModes() {
		for _, native := range []bool{false, true} {
			for _, topology := range ls71PairTopologies() {
				for _, pair := range []bool{false, true} {
					for _, policy := range []bool{false, true} {
						name := mode.name + "/" + nextHopAutoRailName(native) + "/" + topology.name
						name += "/received24"
						if pair {
							name = name[:len(name)-len("received24")] + "received48"
						}
						if policy {
							name += "/policy-base-mixed"
						}
						t.Run(name, func(t *testing.T) {
							f := ls72Fixture(t, mode, topology)
							body := ls72Body(t, ls72Received(topology, pair), policy)
							if policy {
								ls71PairPolicy(t, f, body)
								// Policy selects a complete base before configured
								// rewriting; this distinct received global cannot
								// accidentally satisfy the policy-final oracle.
								field := mustHex(t, "000000000000000020010db80001000000000000000000070000000000000000fe800000000000000000000000000007")
								body = ls72Body(t, field, true)
							}
							original := bytes.Clone(body)
							ls71PairRelay(t, f, body, native, false)
							ls72Assert(t, f, ls72Wanted(mode, topology, pair), false, policy)
							require.Equal(t, original, body)
						})
					}
				}
			}
		}
	}
}

// A framed standalone link-local is not the capability-77 plain16 form. Seed
// useful LS state, then require the exact LS withdrawal with an independently
// useful sibling on receipt and policy rails, with capability77 on and off.
// Restoring the valid field must recover without resetting the session.
func TestLSVPNStandaloneLinkLocalRefused(t *testing.T) {
	for _, mode := range ls71PairModes() {
		if !mode.preserving {
			continue
		}
		for _, native := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, policy := range []bool{false, true} {
					for _, direct := range []bool{false, true} {
						name := mode.name + "/" + nextHopAutoRailName(native) + "/cap77-off"
						if enabled {
							name = name[:len(name)-len("off")] + "on"
						}
						if policy {
							name += "/policy-final"
						} else {
							name += "/received-final"
						}
						name += "/multihop"
						if direct {
							name = name[:len(name)-len("multihop")] + "direct"
						}
						t.Run(name, func(t *testing.T) {
							topology := ls72SingleTopology(true, direct)
							f := ls72Fixture(t, mode, topology)
							ls72Capability77(f, enabled)
							seed := ls72Body(t, ls72Received(topology, false), true)
							replacement := seed
							if policy {
								ls71PairChangingPolicy(t, f, &replacement)
							}
							ls71PairRelay(t, f, seed, native, false)
							ls72Assert(t, f, ls72Received(topology, false), false, true)
							f.conn.reset()
							bad := ls72Body(t, mustHex(t, "0000000000000000fe800000000000000000000000000009"), true)
							body := bad
							if policy {
								replacement = bad
								body = seed
							}
							ls71PairRelay(t, f, body, native, native && policy)
							ls72Assert(t, f, nil, true, true)
							f.conn.reset()
							replacement = seed
							ls71PairRelay(t, f, seed, native, false)
							ls72Assert(t, f, ls72Received(topology, false), false, true)
						})
					}
				}
			}
		}
	}
}

// Use the real negotiation, including both family and ASN4 declarations. No
// Extended Next Hop capability is offered by either endpoint in either arm.
func ls72Capability77(f *aigpReplayFixture, enabled bool) {
	for _, peer := range []*Peer{f.source, f.destination} {
		caps := []capability.Capability{
			&capability.ASN4{ASN: peer.settings.LocalAS},
			&capability.Multiprotocol{AFI: ls.BGPLSVPNFamily.AFI, SAFI: ls.BGPLSVPNFamily.SAFI},
			&capability.Multiprotocol{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
		}
		if enabled {
			caps = append(caps, &capability.LinkLocalNextHop{})
		}
		neg := capability.Negotiate(caps, caps, capability.PeerIdentity{
			LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
			Internal: peer.settings.IsIBGP(),
		})
		peer.session.negotiated = neg
		peer.negotiated.Store(NewNegotiatedCapabilities(neg))
	}
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
}

// A valid replacement supersedes an obsolete recipient-owned base before
// identity admission. Rewriting modes choose their actual connected/configured
// global; they do not pretend that export policy overrides configured selection.
func TestLSVPNRewriteSupersedesPeerOwn(t *testing.T) {
	for _, mode := range ls71PairModes() {
		if mode.preserving {
			continue
		}
		for _, native := range []bool{false, true} {
			for _, scoped := range []bool{false, true} {
				for _, policy := range []bool{false, true} {
					name := mode.name + "/" + nextHopAutoRailName(native) + "/global-first-slot"
					if scoped {
						name = mode.name + "/" + nextHopAutoRailName(native) + "/scoped-second-slot"
					}
					if policy {
						name += "/policy-base"
					} else {
						name += "/received-base"
					}
					t.Run(name, func(t *testing.T) {
						topology := ls71PairTopologies()[0]
						if scoped {
							topology = ls71PairScopedTopology(true)
						}
						f := ls72Fixture(t, mode, topology)
						bad := ls72Framed(topology.recipient.WithZone("").AsSlice(), netip.MustParseAddr("fe80::9").AsSlice())
						if scoped {
							bad = ls72Framed(topology.received.AsSlice(), topology.recipient.WithZone("").AsSlice())
							f.destination.settings.LinkLocal = netip.MustParseAddr("fe80::2")
							f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
						}
						body := ls72Body(t, bad, true)
						if policy {
							ls71PairPolicy(t, f, body)
							body = ls72Body(t, ls72Received(topology, true), true)
						}
						ls71PairRelay(t, f, body, native, false)
						ls72Assert(t, f, ls72Wanted(mode, topology, true), false, true)
						if scoped {
							ls71PairAssertZone(t, f)
						}
					})
				}
			}
		}
	}
}

// RD framing does not import VPN128's unspecified-global exception. Even on
// real scoped link-local transport endpoints, with a connected recipient and
// capability77, LS72 must refuse the unspecified-global plus LL48 field.
func TestLSVPNUnspecifiedGlobalLinkLocalTransportRefused(t *testing.T) {
	for _, mode := range ls71PairModes() {
		if !mode.preserving {
			continue
		}
		for _, native := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, policy := range []bool{false, true} {
					name := mode.name + "/" + nextHopAutoRailName(native) + "/cap77-off"
					if enabled {
						name = name[:len(name)-len("off")] + "on"
					}
					if policy {
						name += "/policy-final"
					} else {
						name += "/received-final"
					}
					t.Run(name, func(t *testing.T) {
						topology := ls71PairScopedTopology(true)
						f := ls72Fixture(t, mode, topology)
						ls72Capability77(f, enabled)
						ls72LinkLocalEndpoint(t, f)
						seed := ls72Body(t, ls72Received(topology, false), true)
						replacement := seed
						if policy {
							ls71PairChangingPolicy(t, f, &replacement)
						}
						ls71PairRelay(t, f, seed, native, false)
						ls72Assert(t, f, ls72Received(topology, false), false, true)
						ls71PairAssertZone(t, f)
						f.conn.reset()
						bad := mustHex(t, "0000000000000000000000000000000000000000000000000000000000000000fe800000000000000000000000000009")
						body := ls72Body(t, bad, true)
						if policy {
							replacement = body
							body = seed
						}
						ls71PairRelay(t, f, body, native, native && policy)
						ls72Assert(t, f, nil, true, true)
						ls71PairAssertZone(t, f)
						f.conn.reset()
						replacement = seed
						ls71PairRelay(t, f, seed, native, false)
						ls72Assert(t, f, ls72Received(topology, false), false, true)
					})
				}
			}
		}
	}
}

// Auto and Self resolve the actual endpoint. When it becomes link-local, the
// selected standalone LS72 form is still unusable with capability77: the plain
// capability77 form is not broadened into permission for a framed24 field.
func TestLSVPNSelfLinkLocalRefused(t *testing.T) {
	for _, mode := range ls71PairModes()[:2] {
		for _, native := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				name := mode.name + "/" + nextHopAutoRailName(native) + "/cap77-off"
				if enabled {
					name = name[:len(name)-len("off")] + "on"
				}
				t.Run(name, func(t *testing.T) {
					topology := ls71PairScopedTopology(true)
					f := ls72Fixture(t, mode, topology)
					conn, ok := f.destination.session.conn.(*ls71PairScopedConn)
					require.True(t, ok)
					originalLocal := bytes.Clone(conn.local.IP)
					ls72Capability77(f, enabled)
					seed := ls72Body(t, ls72Received(topology, false), true)
					ls71PairRelay(t, f, seed, native, false)
					ls72Assert(t, f, ls72Wanted(mode, topology, false), false, true)
					f.conn.reset()
					ls72LinkLocalEndpoint(t, f)
					transport := f.destination.session.transport.Load()
					require.NotNil(t, transport)
					actual, actualString := connectedLocalEndpoint(f.destination, transport.local, transport.localString)
					require.Equal(t, transport.local, actual)
					require.Equal(t, transport.localString, actualString)
					ls71PairRelay(t, f, seed, native, false)
					ls72Assert(t, f, nil, true, true)
					ls71PairAssertZone(t, f)
					f.conn.reset()
					conn.local.IP = originalLocal
					ls71PairSelectGlobal(f, mode, topology.self)
					transport = f.destination.session.transport.Load()
					require.NotNil(t, transport)
					actual, actualString = connectedLocalEndpoint(f.destination, transport.local, transport.localString)
					require.Equal(t, transport.local, actual)
					require.Equal(t, transport.localString, actualString)
					ls71PairRelay(t, f, seed, native, false)
					ls72Assert(t, f, ls72Wanted(mode, topology, false), false, true)
				})
			}
		}
	}
}

func ls72LinkLocalEndpoint(t *testing.T, f *aigpReplayFixture) {
	t.Helper()
	local := netip.MustParseAddr("fe80::1")
	f.destination.session.transport.Store(&sessionTransport{local: local, localString: local.String()})
	conn, ok := f.destination.session.conn.(*ls71PairScopedConn)
	require.True(t, ok)
	conn.local.IP = local.AsSlice()
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	require.True(t, destOnLink(f.destination))
}
