// Design: docs/architecture/bgp/structural-forwarding.md -- final identity before trimming
// Related: ls72_forward_framing_test.go -- actual receive/cache-to-Session wire fixtures
// Related: ls71_pair_relay_test.go -- exact native policy-refusal handback and continuation
// RFC 4271 Section 5.1.3 -- see rfc/short/rfc4271.md
package reactor

import (
	"net/netip"
	"testing"
)

// Seed each single-address route, then make the effective received, policy or
// configured next hop name the recipient. Its exact NLRI must be withdrawn and
// corrected state must reannounce on both rails and both transport distances.
func TestNextHopLSVPNPeerOwnRecovery(t *testing.T) {
	for _, ipv6 := range []bool{false, true} {
		for _, direct := range []bool{false, true} {
			for _, mode := range ls71PairModes() {
				for _, native := range []bool{false, true} {
					for _, policy := range []bool{false, true} {
						if policy && !mode.preserving {
							continue
						}
						name := "ipv4/" + mode.name + "/" + nextHopAutoRailName(native)
						if ipv6 {
							name = "ipv6/" + mode.name + "/" + nextHopAutoRailName(native)
						}
						if direct {
							name += "/direct"
						} else {
							name += "/multihop"
						}
						switch {
						case policy:
							name += "/policy-final"
						case mode.preserving:
							name += "/received-final"
						default:
							name += "/selected-final"
						}
						t.Run(name, func(t *testing.T) {
							topology := ls72SingleTopology(ipv6, direct)
							f := ls72Fixture(t, mode, topology)
							f.destination.settings.OutTTL = 1
							if !direct {
								f.destination.settings.OutTTL = 3
							}
							f.destination.fwdFacts.Store(f.destination.buildForwardFacts())
							seed := ls72Body(t, ls72Received(topology, false), false)
							replacement := seed
							if policy {
								ls71PairChangingPolicy(t, f, &replacement)
							}
							want := topology.self
							if mode.mode == NextHopExplicit {
								want = topology.explicit
							}
							if mode.preserving {
								want = topology.received
							}
							ls71PairRelay(t, f, seed, native, false)
							ls72Assert(t, f, ls72Framed(want.AsSlice(), nil), false, false)

							f.conn.reset()
							body := seed
							if mode.preserving {
								bad := ls72Body(t, ls72Framed(topology.recipient.AsSlice(), nil), false)
								if policy {
									replacement = bad
								} else {
									body = bad
								}
							} else {
								ls71PairSelectGlobal(f, mode, topology.recipient)
							}
							ls71PairRelay(t, f, body, native, false)
							ls72Assert(t, f, nil, true, false)

							f.conn.reset()
							replacement = seed
							if !mode.preserving {
								ls71PairSelectGlobal(f, mode, want)
							}
							ls71PairRelay(t, f, seed, native, false)
							ls72Assert(t, f, ls72Framed(want.AsSlice(), nil), false, false)
						})
					}
				}
			}
		}
	}
}

// A recipient-owned first slot must be judged before48-to24 trimming. A
// genuine mixed sibling remains announced during exact LS withdrawal and
// recovery. Only the predeclared policy-mixed refusal uses native handback.
func TestLSVPNPairPeerOwnRecovery(t *testing.T) {
	for _, mode := range ls71PairModes() {
		for _, native := range []bool{false, true} {
			for _, topology := range ls71PairTopologies()[:2] {
				for _, policy := range []bool{false, true} {
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
						f := ls72Fixture(t, mode, topology)
						seed := ls72Body(t, ls72Received(topology, true), true)
						replacement := seed
						if policy {
							ls71PairChangingPolicy(t, f, &replacement)
						}
						ls71PairRelay(t, f, seed, native, false)
						ls72Assert(t, f, ls72Wanted(mode, topology, true), false, true)

						f.conn.reset()
						body := seed
						if mode.preserving {
							bad := ls72Framed(topology.recipient.AsSlice(), netip.MustParseAddr("fe80::9").AsSlice())
							if policy {
								replacement = ls72Body(t, bad, true)
							} else {
								body = ls72Body(t, bad, true)
							}
						} else {
							ls71PairSelectGlobal(f, mode, topology.recipient)
						}
						ls71PairRelay(t, f, body, native, native && policy)
						ls72Assert(t, f, nil, true, true)

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
						ls72Assert(t, f, ls72Wanted(mode, topology, true), false, true)
					})
				}
			}
		}
	}
}

// A scoped LL recipient can own the second framed address even though its
// transport zone never appears in the field. The valid other-router pair trims
// to a useful24, while the otherwise identical peer-owned pair withdraws.
func TestLSVPNPairScopedPeerOwnRecovery(t *testing.T) {
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
						f := ls72Fixture(t, mode, topology)
						seed := ls72Body(t, ls72Received(topology, true), true)
						replacement := seed
						if policy {
							ls71PairChangingPolicy(t, f, &replacement)
						}
						ls71PairRelay(t, f, seed, native, false)
						ls72Assert(t, f, ls72Received(topology, false), false, true)
						ls71PairAssertZone(t, f)

						f.conn.reset()
						bad := mustHex(t, "000000000000000020010db80001000000000000000000090000000000000000fe800000000000000000000000000002")
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
						ls71PairAssertZone(t, f)
					})
				}
			}
		}
	}
}

// Both slots retain their roles until final admission. Mapped/unspecified/LL
// first slots and a global second slot cannot become a valid24 by trimming an
// invalid48. Useful state and the IPv4 sibling bracket every refusal.
func TestLSVPNPairInvalidBeforeTrim(t *testing.T) {
	for _, mode := range ls71PairModes() {
		if !mode.preserving {
			continue
		}
		for _, native := range []bool{false, true} {
			for _, topology := range ls71PairTopologies()[:2] {
				for _, policy := range []bool{false, true} {
					for _, bad := range []struct{ name, hex string }{
						{"first-is-mapped", "000000000000000000000000000000000000ffffc00002090000000000000000fe800000000000000000000000000009"},
						{"first-is-unspecified", "0000000000000000000000000000000000000000000000000000000000000000fe800000000000000000000000000009"},
						{"first-is-link-local", "0000000000000000fe8000000000000000000000000000090000000000000000fe800000000000000000000000000009"},
						{"second-is-global", "000000000000000020010db8000100000000000000000009000000000000000020010db8000100000000000000000008"},
					} {
						name := mode.name + "/" + nextHopAutoRailName(native) + "/" + topology.name + "/" + bad.name
						if policy {
							name += "/policy-final"
						} else {
							name += "/received-final"
						}
						t.Run(name, func(t *testing.T) {
							f := ls72Fixture(t, mode, topology)
							seed := ls72Body(t, ls72Received(topology, true), true)
							replacement := seed
							if policy {
								ls71PairChangingPolicy(t, f, &replacement)
							}
							ls71PairRelay(t, f, seed, native, false)
							ls72Assert(t, f, ls72Wanted(mode, topology, true), false, true)
							f.conn.reset()
							body := ls72Body(t, mustHex(t, bad.hex), true)
							if policy {
								replacement = body
								body = seed
							}
							ls71PairRelay(t, f, body, native, native && policy)
							ls72Assert(t, f, nil, true, true)
							f.conn.reset()
							replacement = seed
							ls71PairRelay(t, f, seed, native, false)
							ls72Assert(t, f, ls72Wanted(mode, topology, true), false, true)
						})
					}
				}
			}
		}
	}
}
