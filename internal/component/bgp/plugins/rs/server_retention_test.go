// Design: docs/guide/graceful-restart.md -- source-family retention on route-server DOWN.

package rs

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestPeerDownRetainedFamilies checks the real DOWN handler's outgoing commands
// and detached inventory. The hook observes updateRouteSel before its SDK call;
// newTestRouteServer supplies a closed SDK connection, not a replacement sender.
// Multiple routes and wire forms share families to require one synchronous
// retention query per family, including negative decisions.
func TestPeerDownRetainedFamilies(t *testing.T) {
	const source = "192.0.2.1"
	const other = "192.0.2.2"
	linkState := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	v4a := withdrawalKey{fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	v4b := withdrawalKey{fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")}
	v6 := withdrawalKey{fam: family.IPv6Unicast, prefix: netip.MustParsePrefix("2001:db8::/48")}
	opaque := withdrawalKey{fam: linkState, nlriStr: bgplsNLRIUnknown, wireForm: true}
	opaquePath := withdrawalKey{fam: linkState, nlriStr: "00000007" + bgplsNLRIUnknown, wireForm: true, addPath: true}
	v4Command := "update text nlri ipv4/unicast del prefix 10.0.0.0/24 del prefix 10.0.1.0/24"
	v6Command := "update text nlri ipv6/unicast del prefix 2001:db8::/48"
	opaqueCommand := "update hex nlri bgp-ls/bgp-ls del " + bgplsNLRIUnknown
	opaquePathCommand := "update hex nlri bgp-ls/bgp-ls addpath del 00000007" + bgplsNLRIUnknown

	for _, tc := range []struct {
		name         string
		provider     bool
		retainedPeer string
		retainedFam  family.Family
		wantEntries  []withdrawalKey
		wantCommands []string
	}{
		{
			name: "retained IPv4 with unretained IPv6 and opaque families", provider: true,
			retainedPeer: source, retainedFam: family.IPv4Unicast,
			wantEntries: []withdrawalKey{v6, opaque, opaquePath},
			wantCommands: []string{v6Command, opaqueCommand, opaquePathCommand},
		},
		{
			name: "retained opaque family covers both wire forms", provider: true,
			retainedPeer: source, retainedFam: linkState,
			wantEntries: []withdrawalKey{v4a, v4b, v6},
			wantCommands: []string{v4Command, v6Command},
		},
		{
			name: "another peer retention does not suppress withdrawals", provider: true,
			retainedPeer: other, retainedFam: family.IPv4Unicast,
			wantEntries: []withdrawalKey{v4a, v4b, v6, opaque, opaquePath},
			wantCommands: []string{v4Command, v6Command, opaqueCommand, opaquePathCommand},
		},
		{
			name: "no in-process provider preserves ordinary withdrawals",
			wantEntries: []withdrawalKey{v4a, v4b, v6, opaque, opaquePath},
			wantCommands: []string{v4Command, v6Command, opaqueCommand, opaquePathCommand},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs := newTestRouteServer(t)
			queries := make(map[family.Family]int)
			var queriedPeers []string
			if tc.provider {
				owner := retention.Publish(func(peer string, fam family.Family) bool {
					queriedPeers = append(queriedPeers, peer)
					queries[fam]++
					return peer == tc.retainedPeer && fam == tc.retainedFam
				})
				t.Cleanup(owner.Close)
			} else {
				// Close the publication before DOWN to exercise an absent owner,
				// rather than a provider that merely returns false.
				owner := retention.Publish(nil)
				owner.Close()
			}

			entries := map[withdrawalKey]struct{}{v4a: {}, v4b: {}, v6: {}, opaque: {}, opaquePath: {}}
			otherEntries := map[withdrawalKey]struct{}{v4a: {}}
			rs.withdrawals[source] = entries
			rs.withdrawals[other] = otherEntries
			type withdrawal struct {
				selector string
				command  string
			}
			output := make(chan withdrawal, len(entries))
			rs.updateRouteHook = func(peer, command string) {
				output <- withdrawal{selector: peer, command: command}
			}

			rs.handleState(&Event{PeerAddr: source, State: "down"})

			// These assertions run immediately, without awaiting the sender:
			// retention belongs to the DOWN event, not its asynchronous RPCs.
			if tc.provider {
				require.Equal(t, map[family.Family]int{
					family.IPv4Unicast: 1, family.IPv6Unicast: 1, linkState: 1,
				}, queries)
				require.Equal(t, []string{source, source, source}, queriedPeers)
			}
			require.NotContains(t, rs.withdrawals, source)
			require.Equal(t, otherEntries, rs.withdrawals[other])
			require.False(t, rs.peers[source].Up)
			require.True(t, rs.peers[source].StateSeen)
			var remaining []withdrawalKey
			for key := range entries {
				remaining = append(remaining, key)
			}
			require.ElementsMatch(t, tc.wantEntries, remaining,
				"retained entries must not reach the sender or become a second stale inventory")

			var commands []string
			deadline := time.NewTimer(2 * time.Second)
			defer deadline.Stop()
			for range tc.wantCommands {
				select {
				case got := <-output:
					require.Equal(t, selector.ExcludeAddr(netip.MustParseAddr(source)).String(), got.selector)
					commands = append(commands, got.command)
				case <-deadline.C:
					t.Fatal("timeout waiting for peer-down withdrawals")
				}
			}
			require.ElementsMatch(t, tc.wantCommands, commands)
		})
	}
}
