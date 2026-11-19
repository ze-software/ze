// Design: docs/architecture/bgp/structural-forwarding.md -- final identity before trimming
// Related: ls71_pair_scope_test.go -- LS71 recording-writer fixture and byte assertions
// Related: rfc4271_scoped_next_hop_test.go -- scoped transport identity
// Related: link_scope_fixture_test.go -- shared scoped topology and session helpers
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The recipient first receives useful LS71 state, then an effective next hop
// naming itself must withdraw exactly that NLRI, and a corrected field must
// reannounce it. The independent IPv4 section remains useful in every phase.
// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
// advertised to a peer using an address of that peer as NEXT_HOP."
func TestLS71PairPeerOwnRecovery(t *testing.T) {
	for _, mode := range ls71PairModes() {
		for _, native := range []bool{false, true} {
			for _, onLink := range []bool{true, false} {
				topology := ls71PairTopologies()[0]
				if !onLink {
					topology = ls71PairTopologies()[1]
				}
				for _, policy := range []bool{false, true} {
					// Configured rewriting follows a raw policy base. Its harmful
					// final global is selected by its real mode, never by claiming
					// that the earlier raw policy can override that mode.
					if policy && !mode.preserving {
						continue
					}
					name := mode.name + "/" + nextHopAutoRailName(native) + "/" + topology.name
					switch {
					case policy:
						name += "/policy-final"
					case mode.preserving:
						name += "/received-final"
					default:
						name += "/selected-final"
					}
					t.Run(name, func(t *testing.T) {
						f := ls71PairFixture(t, mode, topology)
						seed := ls71PairBody(t, ls71PairReceived(topology), true)
						replacement := seed
						if policy {
							ls71PairChangingPolicy(t, f, &replacement)
						}
						ls71PairRelay(t, f, seed, native, false)
						ls71PairAssert(t, f, ls71PairWanted(mode, topology, true), false, true)

						f.conn.reset()
						bad := append(topology.recipient.AsSlice(), netip.MustParseAddr("fe80::9").AsSlice()...)
						body := seed
						if mode.preserving {
							if policy {
								replacement = ls71PairBody(t, bad, true)
							} else {
								body = ls71PairBody(t, bad, true)
							}
						} else {
							ls71PairSelectGlobal(f, mode, topology.recipient)
						}
						ls71PairRelay(t, f, body, native, native && policy)
						ls71PairAssert(t, f, nil, true, true)

						f.conn.reset()
						replacement = seed
						if !mode.preserving {
							global := topology.self
							if mode.mode == NextHopExplicit {
								global = topology.explicit
							}
							ls71PairSelectGlobal(f, mode, global)
						}
						ls71PairRelay(t, f, seed, native, false)
						ls71PairAssert(t, f, ls71PairWanted(mode, topology, true), false, true)
					})
				}
			}
		}
	}
}

// A scoped link-local recipient's wire address must be recognized in slot two
// before off-link trimming can conceal it. A global and an LL transport cannot
// share one IP prefix, so a legal other-router pair trims to its useful global.
// These are the three preserving modes: configured rewrites supersede the bad
// pair and are covered separately, rather than manufacturing an impossible
// global-plus-link-local-recipient joint subnet.
func TestLS71PairScopedPeerOwnRecovery(t *testing.T) {
	for _, mode := range ls71PairModes() {
		if !mode.preserving {
			continue
		}
		for _, native := range []bool{false, true} {
			for _, policy := range []bool{false, true} {
				for _, peerPrefix := range []bool{false, true} {
					name := mode.name + "/" + nextHopAutoRailName(native)
					if policy {
						name += "/policy-final"
					} else {
						name += "/received-final"
					}
					if peerPrefix {
						name += "/recipient-prefix-present"
					} else {
						name += "/recipient-prefix-absent"
					}
					t.Run(name, func(t *testing.T) {
						topology := ls71PairScopedTopology(peerPrefix)
						f := ls71PairFixture(t, mode, topology)
						seed := ls71PairBody(t, ls71PairReceived(topology), true)
						replacement := seed
						if policy {
							ls71PairChangingPolicy(t, f, &replacement)
						}
						ls71PairRelay(t, f, seed, native, false)
						ls71PairAssert(t, f, topology.received.AsSlice(), false, true)
						ls71PairAssertZone(t, f)

						f.conn.reset()
						bad := mustHex(t, "20010db8000100000000000000000009fe800000000000000000000000000002")
						body := seed
						if policy {
							replacement = ls71PairBody(t, bad, true)
						} else {
							body = ls71PairBody(t, bad, true)
						}
						ls71PairRelay(t, f, body, native, native && policy)
						ls71PairAssert(t, f, nil, true, true)
						ls71PairAssertZone(t, f)

						f.conn.reset()
						replacement = seed
						ls71PairRelay(t, f, seed, native, false)
						ls71PairAssert(t, f, topology.received.AsSlice(), false, true)
						ls71PairAssertZone(t, f)
					})
				}
			}
		}
	}
}

// Auto/Self/explicit must judge the selected field instead of stale received or
// policy-base identity. A scoped recipient also prevents pair selection: even
// setting the configured speaker LL to the recipient's wire address must leave
// the selected useful global16, because there is no joint global/LL subnet.
func TestLS71PairRewritesSupersededPeerOwn(t *testing.T) {
	for _, mode := range ls71PairModes() {
		if mode.preserving {
			continue
		}
		for _, native := range []bool{false, true} {
			for _, scoped := range []bool{false, true} {
				for _, policy := range []bool{false, true} {
					name := mode.name + "/" + nextHopAutoRailName(native)
					if scoped {
						name += "/scoped-second-slot"
					} else {
						name += "/global-first-slot"
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
						f := ls71PairFixture(t, mode, topology)
						bad := mustHex(t, "20010db8000100000000000000000002fe800000000000000000000000000009")
						if scoped {
							bad = mustHex(t, "20010db8000100000000000000000009fe800000000000000000000000000002")
							f.destination.settings.LinkLocal = netip.MustParseAddr("fe80::2")
							f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
						}
						body := ls71PairBody(t, bad, true)
						if policy {
							ls71PairPolicy(t, f, body)
							body = ls71PairBody(t, ls71PairReceived(topology), true)
						}
						rfc4271ProofRelay(t, f, body, native)
						ls71PairAssert(t, f, ls71PairWanted(mode, topology, true), false, true)
						if scoped {
							ls71PairAssertZone(t, f)
						}
					})
				}
			}
		}
	}
}

// Invalid address roles must withdraw the seeded LS71 route before a trim could
// hide the offending slot; an otherwise identical valid pair recovers it. Both
// receipt and real export-policy bases retain the useful mixed IPv4 sibling.
func TestLS71PairInvalidBeforeTrim(t *testing.T) {
	for _, mode := range ls71PairModes() {
		if !mode.preserving {
			continue
		}
		for _, native := range []bool{false, true} {
			for _, topology := range ls71PairTopologies()[:2] {
				for _, policy := range []bool{false, true} {
					for _, bad := range []struct{ name, hex string }{
						{"first-is-link-local", "fe800000000000000000000000000009fe800000000000000000000000000009"},
						{"second-is-global", "20010db800010000000000000000000920010db8000100000000000000000008"},
						{"first-is-unspecified", "00000000000000000000000000000000fe800000000000000000000000000009"},
					} {
						name := mode.name + "/" + nextHopAutoRailName(native) + "/" + topology.name + "/" + bad.name
						if policy {
							name += "/policy-final"
						} else {
							name += "/received-final"
						}
						t.Run(name, func(t *testing.T) {
							f := ls71PairFixture(t, mode, topology)
							seed := ls71PairBody(t, ls71PairReceived(topology), true)
							replacement := seed
							if policy {
								ls71PairChangingPolicy(t, f, &replacement)
							}
							ls71PairRelay(t, f, seed, native, false)
							ls71PairAssert(t, f, ls71PairWanted(mode, topology, true), false, true)
							f.conn.reset()
							body := ls71PairBody(t, mustHex(t, bad.hex), true)
							if policy {
								replacement, body = body, seed
							}
							ls71PairRelay(t, f, body, native, native && policy)
							ls71PairAssert(t, f, nil, true, true)
							f.conn.reset()
							replacement = seed
							ls71PairRelay(t, f, seed, native, false)
							ls71PairAssert(t, f, ls71PairWanted(mode, topology, true), false, true)
						})
					}
				}
			}
		}
	}
}

func ls71PairReceived(topology ls71PairTopology) []byte {
	return append(topology.received.AsSlice(), netip.MustParseAddr("fe80::9").AsSlice()...)
}

func ls71PairSelectGlobal(f *aigpReplayFixture, mode ls71PairMode, global netip.Addr) {
	if mode.mode == NextHopExplicit {
		f.destination.settings.NextHopAddress = global
	} else {
		f.destination.session.transport.Store(&sessionTransport{local: global, localString: global.String()})
	}
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
}

func ls71PairAssertZone(t *testing.T, f *aigpReplayFixture) {
	t.Helper()
	require.Equal(t, netip.MustParseAddr("fe80::2%if0"), f.destination.Settings().Address)
	require.Equal(t, "if0", f.destination.session.settings.Address.Zone())
	require.Equal(t, netip.MustParseAddr("fe80::2"), f.destination.Settings().Address.WithZone(""))
	conn, ok := f.destination.session.conn.(*ls71PairScopedConn)
	require.True(t, ok)
	require.Equal(t, "if0", conn.remote.Zone)
	require.Equal(t, f.destination.Settings().Address, conn.remote.AddrPort().Addr())
}

// The queue-drain fence before each replacement ensures no earlier filter call
// can still read the previous generation. The actual export callback must run.
func ls71PairChangingPolicy(t *testing.T, f *aigpReplayFixture, replacement *[]byte) {
	t.Helper()
	scope := f.destination.llScope.Load()
	exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
		return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: *replacement}
	}, f.destination)
	f.destination.llScope.Store(scope)
	f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
	transport, ok := f.r.filterTransportSeam.(*answerTransport)
	require.True(t, ok)
	t.Cleanup(func() { require.Positive(t, transport.calls(), "the real export policy must run") })
}
