// Design: docs/guide/route-reflection.md -- departure withdrawal under ze's Path Identifier
// RFC: rfc/short/rfc7911.md -- Section 2, a re-advertised route carries the speaker's own Path Identifier

package rr

import (
	"context"
	"encoding/hex"
	"maps"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// peerDownRequest is one request the route reflector sent on a peer down, as
// the engine's DirectBridge received it.
type peerDownRequest struct {
	selector string
	command  string
	meta     map[string]any
}

// newPeerDownReflector returns a route reflector wired to a DirectBridge that
// records every UpdateRouteSel request it is sent.
func newPeerDownReflector(t *testing.T) (*routeReflector, chan peerDownRequest) {
	t.Helper()
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	p := sdk.NewWithConn("rr-down-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close() })
	requests := make(chan peerDownRequest, 4)
	bridge.SetUpdateRouteSel(func(_ context.Context, sel *selector.Selector, command string, meta map[string]any) (uint32, uint32, error) {
		requests <- peerDownRequest{selector: sel.String(), command: command, meta: meta}
		return 0, 2, nil
	})
	bridge.SetReady()
	return &routeReflector{plugin: p, withdrawals: make(map[string]map[string]withdrawalInfo)}, requests
}

// TestPeerDownIsSourceOwnedRecovery takes down an ADD-PATH source that holds
// two paths of one prefix, in unicast and in VPNv4.
//
// VALIDATES: the departure goes to the engine as a source-DOWN recovery: the
// request carries meta recovery-source naming the departed peer, excludes that
// peer, names both paths under the source's own identifiers, and the down's
// departure worker sent it before it stopped (stopDepartures).
// PREVENTS: a plain withdrawal, which the engine writes with the source's
// Path Identifier as given. ze re-advertised each path to an ADD-PATH
// destination under its own identifier, so that destination kept both paths.
// RFC 7911 Section 2: "A BGP speaker that re-advertises a route MUST generate
// its own Path Identifier to be associated with the re-advertised route."
// Only the engine's recovery maps the source's identifier to the destination's.
func TestPeerDownIsSourceOwnedRecovery(t *testing.T) {
	const source = "192.0.2.1"
	path11 := []byte{0, 0, 0, 11, 8, 10}
	path12 := []byte{0, 0, 0, 12, 8, 10}
	vpn11 := rrVPNRoute(vpn.IPv4VPN, true, 11, 1, 10, 0x000641)
	vpn12 := rrVPNRoute(vpn.IPv4VPN, true, 12, 1, 10, 0x000641)
	tests := []struct {
		name     string
		announce func(t *testing.T, rr *routeReflector)
		command  string
	}{
		{
			name: "ipv4/unicast",
			announce: func(t *testing.T, rr *routeReflector) {
				t.Helper()
				origin := []byte{0x40, 1, 1, 0}
				rr.updateWithdrawalMapWire(source, rrAddPathUpdate(t, nil, origin, append(append([]byte{}, path11...), path12...)))
			},
			command: "update hex nlri ipv4/unicast addpath del " + hex.EncodeToString(path11) + " del " + hex.EncodeToString(path12),
		},
		{
			name: "ipv4/mpls-vpn",
			announce: func(t *testing.T, rr *routeReflector) {
				t.Helper()
				rrApplyVPN(t, rr, vpn.IPv4VPN, true, 14, append(append([]byte{}, vpn11...), vpn12...))
			},
			command: "update hex nlri " + vpn.IPv4VPN.String() + " addpath del " + hex.EncodeToString(vpn11) + " del " + hex.EncodeToString(vpn12),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr, requests := newPeerDownReflector(t)
			tt.announce(t, rr)
			rr.handleStateDown(source, 7)
			var got peerDownRequest
			select {
			case got = <-requests:
			case <-time.After(5 * time.Second):
				t.Fatal("peer-down sent no request")
			}
			rr.stopDepartures()
			if got.command != tt.command {
				t.Fatalf("peer-down command = %q, want %q", got.command, tt.command)
			}
			if want := "!" + source; got.selector != want {
				t.Fatalf("peer-down selector = %q, want %q", got.selector, want)
			}
			if got.meta["recovery-source"] != source {
				t.Fatalf("peer-down meta = %v, want recovery-source %s: a plain withdrawal names the source's Path Identifiers to every destination", got.meta, source)
			}
			if got.meta["recovery-generation"] != "7" {
				t.Fatalf("peer-down meta = %v, want recovery-generation 7: the departure names the session its down ended", got.meta)
			}
			select {
			case extra := <-requests:
				t.Fatalf("peer-down sent a second request %+v, want one", extra)
			default:
			}
		})
	}
}

// TestPeerDownSkipsRetainedFamilies takes down an ADD-PATH source holding two
// unicast paths and two VPNv4 paths while an in-process bgp-gr owner retains
// one of the two families for that source.
//
// VALIDATES: the retained family's routes reach the engine as no departure at
// all, the other family's routes still do, retention is asked once per family
// and synchronously inside handleStateDown, and a retention decision for a
// different peer suppresses nothing.
// PREVENTS: a GR or LLGR source's routes withdrawn from every destination at
// the DOWN, during the restart window. RFC 4724 Section 4.2: "it MUST
// retain the routes received from the peer for all the address families that
// were previously received in the Graceful Restart Capability".
// The engine's recovery withdraws each destination's own copy, so a retained
// route sent as a departure is really withdrawn at an ADD-PATH destination.
func TestPeerDownSkipsRetainedFamilies(t *testing.T) {
	const source = "192.0.2.1"
	const other = "192.0.2.2"
	path11 := []byte{0, 0, 0, 11, 8, 10}
	path12 := []byte{0, 0, 0, 12, 8, 10}
	vpn11 := rrVPNRoute(vpn.IPv4VPN, true, 11, 1, 10, 0x000641)
	vpn12 := rrVPNRoute(vpn.IPv4VPN, true, 12, 1, 10, 0x000641)
	unicastCommand := "update hex nlri ipv4/unicast addpath del " + hex.EncodeToString(path11) + " del " + hex.EncodeToString(path12)
	vpnCommand := "update hex nlri " + vpn.IPv4VPN.String() + " addpath del " + hex.EncodeToString(vpn11) + " del " + hex.EncodeToString(vpn12)
	tests := []struct {
		name         string
		retainedPeer string
		retainedFam  family.Family
		wantCommands []string
	}{
		{name: "retained unicast", retainedPeer: source, retainedFam: family.IPv4Unicast, wantCommands: []string{vpnCommand}},
		{name: "retained vpn", retainedPeer: source, retainedFam: vpn.IPv4VPN, wantCommands: []string{unicastCommand}},
		{name: "another peer retained", retainedPeer: other, retainedFam: family.IPv4Unicast, wantCommands: []string{unicastCommand, vpnCommand}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := make(map[family.Family]int)
			owner := retention.Publish(func(peer string, fam family.Family) bool {
				if peer != source {
					t.Errorf("retention asked about peer %s, want %s", peer, source)
				}
				queries[fam]++
				return peer == tt.retainedPeer && fam == tt.retainedFam
			})
			t.Cleanup(owner.Close)

			rr, requests := newPeerDownReflector(t)
			origin := []byte{0x40, 1, 1, 0}
			rr.updateWithdrawalMapWire(source, rrAddPathUpdate(t, nil, origin, append(append([]byte{}, path11...), path12...)))
			rrApplyVPN(t, rr, vpn.IPv4VPN, true, 14, append(append([]byte{}, vpn11...), vpn12...))
			rr.handleStateDown(source, 7)

			// Read before the departure worker is stopped: retention belongs
			// to the DOWN event, not to its asynchronous requests.
			want := map[family.Family]int{family.IPv4Unicast: 1, vpn.IPv4VPN: 1}
			if !maps.Equal(queries, want) {
				t.Fatalf("retention queries = %v, want one per family %v", queries, want)
			}
			rr.stopDepartures()
			close(requests)
			var got []string
			for request := range requests {
				if request.meta["recovery-source"] != source {
					t.Fatalf("peer-down meta = %v, want recovery-source %s", request.meta, source)
				}
				got = append(got, request.command)
			}
			slices.Sort(got)
			wantCommands := slices.Clone(tt.wantCommands)
			slices.Sort(wantCommands)
			if !slices.Equal(got, wantCommands) {
				t.Fatalf("peer-down commands = %q, want %q", got, wantCommands)
			}
		})
	}
}
