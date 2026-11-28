package locatororigin

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLocatorOriginIndexArms exercises every arm of the in-daemon locator-originator
// index and JSON snapshot contract.
//
// VALIDATES: empty index answers not-known with its named reason (never an empty
// address set treated as a mismatch); ApplySnapshot longest-match, monotonic
// generation filtering, Active: false deactivation, ClearSource disconnect cleanup
// accepting generation 1 from a replacement, conflicting multi-source originators
// answering anycast not-known, MatchAddress/MatchPrefix arms, and JSON round-trip
// of Snapshot.
// PREVENTS: stale or disconnected IGP snapshots surviving in the index, out-of-order
// generations overwriting fresher state, or an unresolvable locator withdrawing
// valid MUP routes.
func TestLocatorOriginIndexArms(t *testing.T) {
	sid48 := netip.MustParseAddr("2001:db8:1::100")
	sid64 := netip.MustParseAddr("2001:db8:1:2::100")
	sidUncovered := netip.MustParseAddr("2001:db8:ff::1")
	addrPE1 := netip.MustParseAddr("2001:db8::1")
	addrPE2 := netip.MustParseAddr("2001:db8::2")
	addrOther := netip.MustParseAddr("2001:db8::99")
	addrV4 := netip.MustParseAddr("192.0.2.1")

	t.Run("empty index answers not-known and never mismatches", func(t *testing.T) {
		idx := NewIndex()
		res := idx.Resolve(sid48)
		assert.False(t, res.Known)
		assert.Equal(t, ReasonNoActiveSource, res.ReasonClass)
		assert.NotEmpty(t, res.Reason)

		outcome, rc, reason := res.MatchAddress(addrPE1)
		assert.Equal(t, MatchUnknown, outcome)
		assert.Equal(t, ReasonNoActiveSource, rc)
		assert.NotEmpty(t, reason)

		v4Outcome, v4RC, _ := res.MatchAddress(addrV4)
		assert.Equal(t, MatchUnknown, v4Outcome)
		assert.Equal(t, ReasonNoActiveSource, v4RC)
	})

	t.Run("longest match, MatchAddress, MatchPrefix, and TLV 237 guard", func(t *testing.T) {
		idx := NewIndex()
		applied := idx.ApplySnapshot(&Snapshot{
			Source:     "isis",
			Generation: 1,
			Active:     true,
			Entries: []LocatorEntry{
				{
					Prefix:     netip.MustParsePrefix("2001:db8:1::/48"),
					Known:      true,
					Originator: "0000.0000.0001",
					Addresses:  []netip.Addr{addrPE1},
					Prefixes:   []netip.Prefix{netip.MustParsePrefix("2001:db8:10::/48")},
				},
				{
					Prefix:             netip.MustParsePrefix("2001:db8:1:2::/64"),
					Known:              true,
					Originator:         "0000.0000.0002",
					Addresses:          []netip.Addr{addrPE2},
					HasUndecodedTLV237: true,
				},
			},
		})
		require.True(t, applied)

		r48 := idx.Resolve(sid48)
		require.True(t, r48.Known)
		assert.Equal(t, "0000.0000.0001", r48.Originator)

		out, _, _ := r48.MatchAddress(addrPE1)
		assert.Equal(t, MatchKnown, out)
		out, _, _ = r48.MatchAddress(addrOther)
		assert.Equal(t, MatchMismatch, out)
		out, _, _ = r48.MatchAddress(addrV4)
		assert.Equal(t, MatchMismatch, out)

		out, _, _ = r48.MatchPrefix(netip.MustParsePrefix("2001:db8::1/128"))
		assert.Equal(t, MatchKnown, out)
		out, _, _ = r48.MatchPrefix(netip.MustParsePrefix("2001:db8:10::/48"))
		assert.Equal(t, MatchKnown, out)
		out, _, _ = r48.MatchPrefix(netip.MustParsePrefix("2001:db8:20::/48"))
		assert.Equal(t, MatchMismatch, out)
		out, _, _ = r48.MatchPrefix(netip.MustParsePrefix("10.0.0.0/24"))
		assert.Equal(t, MatchMismatch, out)

		// Longer /64 beats /48.
		r64 := idx.Resolve(sid64)
		require.True(t, r64.Known)
		assert.Equal(t, "0000.0000.0002", r64.Originator)
		out, _, _ = r64.MatchAddress(addrPE2)
		assert.Equal(t, MatchKnown, out)
		// Unmatched IPv6 address against originator with undecoded TLV 237 -> unknown, not mismatch.
		out, rc, _ := r64.MatchAddress(addrOther)
		assert.Equal(t, MatchUnknown, out)
		assert.Equal(t, ReasonUndecodedTLV237, rc)
		// IPv4 against known IPv6 originator still mismatches even with TLV 237.
		out, _, _ = r64.MatchAddress(addrV4)
		assert.Equal(t, MatchMismatch, out)

		// Uncovered SID -> ReasonNoCoveringLocator.
		rMiss := idx.Resolve(sidUncovered)
		assert.False(t, rMiss.Known)
		assert.Equal(t, ReasonNoCoveringLocator, rMiss.ReasonClass)

		// IPv4-mapped IPv6 SID (::ffff:x.x.x.x) is rejected as non-native IPv6.
		addr4In6 := netip.MustParseAddr("::ffff:192.0.2.1")
		r4In6 := idx.Resolve(addr4In6)
		assert.False(t, r4In6.Known)
		assert.Equal(t, ReasonNoCoveringLocator, r4In6.ReasonClass)

		// IPv4-mapped IPv6 next hop / prefix returns MatchMismatch even with HasUndecodedTLV237=true.
		out, _, _ = r48.MatchAddress(addr4In6)
		assert.Equal(t, MatchMismatch, out)
		out, _, _ = r64.MatchAddress(addr4In6)
		assert.Equal(t, MatchMismatch, out)
		out, _, _ = r64.MatchPrefix(netip.PrefixFrom(addr4In6, 128))
		assert.Equal(t, MatchMismatch, out)
		out, _, _ = r64.MatchPrefix(netip.MustParsePrefix("::ffff:192.0.2.0/120"))
		assert.Equal(t, MatchMismatch, out)
	})

	t.Run("IPv4 MatchPrefix against known originator addresses and prefixes", func(t *testing.T) {
		idx := NewIndex()
		v4Host := netip.MustParseAddr("192.0.2.10")
		v4Pfx := netip.MustParsePrefix("10.10.0.0/24")
		require.True(t, idx.ApplySnapshot(&Snapshot{
			Source:     "isis",
			Generation: 1,
			Active:     true,
			Entries: []LocatorEntry{{
				Prefix:             netip.MustParsePrefix("2001:db8:1::/48"),
				Known:              true,
				Originator:         "0000.0000.0001",
				Addresses:          []netip.Addr{addrPE1, v4Host},
				Prefixes:           []netip.Prefix{v4Pfx},
				HasUndecodedTLV237: true,
			}},
		}))

		r := idx.Resolve(sid48)
		require.True(t, r.Known)

		out, _, _ := r.MatchPrefix(netip.PrefixFrom(v4Host, 32))
		assert.Equal(t, MatchKnown, out)
		out, _, _ = r.MatchPrefix(v4Pfx)
		assert.Equal(t, MatchKnown, out)
		// Unadvertised IPv4 prefix mismatches even when HasUndecodedTLV237=true.
		out, _, _ = r.MatchPrefix(netip.MustParsePrefix("10.99.0.0/24"))
		assert.Equal(t, MatchMismatch, out)
		// DSD MatchAddress still treats IPv4 as mismatch against known IPv6 locator originator.
		out, _, _ = r.MatchAddress(v4Host)
		assert.Equal(t, MatchMismatch, out)
	})

	t.Run("monotonic generation filtering, Active false deactivation, and ClearSource replacement", func(t *testing.T) {
		idx := NewIndex()
		require.True(t, idx.ApplySnapshot(&Snapshot{
			Source:     "isis",
			Generation: 5,
			Active:     true,
			Entries: []LocatorEntry{{
				Prefix:     netip.MustParsePrefix("2001:db8:1::/48"),
				Known:      true,
				Originator: "0000.0000.0001",
				Addresses:  []netip.Addr{addrPE1},
			}},
		}))

		// Duplicate and older generations are rejected.
		assert.False(t, idx.ApplySnapshot(&Snapshot{Source: "isis", Generation: 5, Active: false}))
		assert.False(t, idx.ApplySnapshot(&Snapshot{Source: "isis", Generation: 4, Active: false}))
		assert.True(t, idx.Resolve(sid48).Known)

		// Higher generation with Active: false deactivates the source.
		require.True(t, idx.ApplySnapshot(&Snapshot{Source: "isis", Generation: 6, Active: false}))
		res := idx.Resolve(sid48)
		assert.False(t, res.Known)
		assert.Equal(t, ReasonNoActiveSource, res.ReasonClass)

		// Re-activate at generation 7, then simulate abrupt disconnect with ClearSource.
		require.True(t, idx.ApplySnapshot(&Snapshot{
			Source:     "isis",
			Generation: 7,
			Active:     true,
			Entries: []LocatorEntry{{
				Prefix:     netip.MustParsePrefix("2001:db8:1::/48"),
				Known:      true,
				Originator: "0000.0000.0001",
				Addresses:  []netip.Addr{addrPE1},
			}},
		}))
		require.True(t, idx.Resolve(sid48).Known)

		idx.ClearSource("isis")
		assert.Equal(t, ReasonNoActiveSource, idx.Resolve(sid48).ReasonClass)

		// Replacement incarnation starting at generation 1 is accepted.
		require.True(t, idx.ApplySnapshot(&Snapshot{
			Source:     "isis",
			Generation: 1,
			Active:     true,
			Entries: []LocatorEntry{{
				Prefix:     netip.MustParsePrefix("2001:db8:1::/48"),
				Known:      true,
				Originator: "0000.0000.0002",
				Addresses:  []netip.Addr{addrPE2},
			}},
		}))
		got := idx.Resolve(sid48)
		require.True(t, got.Known)
		assert.Equal(t, "0000.0000.0002", got.Originator)
	})

	t.Run("conflicting active sources answer anycast not-known", func(t *testing.T) {
		idx := NewIndex()
		require.True(t, idx.ApplySnapshot(&Snapshot{
			Source:     "isis",
			Generation: 1,
			Active:     true,
			Entries: []LocatorEntry{{
				Prefix:     netip.MustParsePrefix("2001:db8:1::/48"),
				Known:      true,
				Originator: "0000.0000.0001",
				Addresses:  []netip.Addr{addrPE1},
			}},
		}))
		require.True(t, idx.ApplySnapshot(&Snapshot{
			Source:     "isis-2",
			Generation: 1,
			Active:     true,
			Entries: []LocatorEntry{{
				Prefix:     netip.MustParsePrefix("2001:db8:1::/48"),
				Known:      true,
				Originator: "0000.0000.0002",
				Addresses:  []netip.Addr{addrPE2},
			}},
		}))
		res := idx.Resolve(sid48)
		assert.False(t, res.Known)
		assert.Equal(t, ReasonAnycast, res.ReasonClass)
	})

	t.Run("JSON round-trip preserves Snapshot and ReasonClass", func(t *testing.T) {
		want := Snapshot{
			Source:     "isis",
			Generation: 42,
			Active:     true,
			Entries: []LocatorEntry{
				{
					Prefix:          netip.MustParsePrefix("2001:db8:1::/48"),
					Known:           true,
					Originator:      "0000.0000.0001",
					Addresses:       []netip.Addr{addrPE1},
					Prefixes:        []netip.Prefix{netip.MustParsePrefix("2001:db8:10::/48")},
					SourceRouterIDs: []netip.Addr{addrPE1},
				},
				{
					Prefix:          netip.MustParsePrefix("2001:db8:2::/48"),
					Known:           false,
					Originator:      "0000.0000.0002",
					SourceRouterIDs: []netip.Addr{addrPE1},
					ReasonClass:     ReasonReadvertised,
					Reason:          "covering locator 2001:db8:2::/48 re-advertised by 0000.0000.0002 (R or X set)",
				},
			},
		}
		raw, err := json.Marshal(&want)
		require.NoError(t, err)

		var got Snapshot
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, want, got)
	})
}
