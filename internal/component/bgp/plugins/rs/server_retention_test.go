// Design: docs/guide/graceful-restart.md -- source-family retention on route-server DOWN.

package rs

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestPeerDownRetainedFamilies checks the real DOWN handler's outgoing commands
// and detached inventory. The hook observes updateRouteSel before its SDK call;
// newTestRouteServer supplies a closed SDK connection, not a replacement sender.
// Multiple routes and wire forms share families, and bgp-gr's verdict reaches
// bgp-rs as the bgp-gr/retention event through the same event handler the
// DOWN arrives on, before it. The DOWN consumes the verdict.
func TestPeerDownRetainedFamilies(t *testing.T) {
	const source = "192.0.2.1"
	const other = "192.0.2.2"
	linkState := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	v4a := withdrawalKey{fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	v4b := withdrawalKey{fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")}
	v6 := withdrawalKey{fam: family.IPv6Unicast, prefix: netip.MustParsePrefix("2001:db8::/48")}
	opaque := withdrawalKey{fam: linkState, nlriStr: bgplsNLRIUnknown, wireForm: true}
	opaquePath := withdrawalKey{fam: linkState, nlriStr: "00000007" + bgplsNLRIUnknown, wireForm: true, addPath: true}
	v4Command := "update text nlri del prefix 10.0.0.0/24 del prefix 10.0.1.0/24"
	v6Command := "update text mp ipv6/unicast del prefix 2001:db8::/48"
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
			wantEntries:  []withdrawalKey{v6, opaque, opaquePath},
			wantCommands: []string{v6Command, opaqueCommand, opaquePathCommand},
		},
		{
			name: "retained opaque family covers both wire forms", provider: true,
			retainedPeer: source, retainedFam: linkState,
			wantEntries:  []withdrawalKey{v4a, v4b, v6},
			wantCommands: []string{v4Command, v6Command},
		},
		{
			name: "another peer retention does not suppress withdrawals", provider: true,
			retainedPeer: other, retainedFam: family.IPv4Unicast,
			wantEntries:  []withdrawalKey{v4a, v4b, v6, opaque, opaquePath},
			wantCommands: []string{v4Command, v6Command, opaqueCommand, opaquePathCommand},
		},
		{
			name:         "no verdict preserves ordinary withdrawals",
			wantEntries:  []withdrawalKey{v4a, v4b, v6, opaque, opaquePath},
			wantCommands: []string{v4Command, v6Command, opaqueCommand, opaquePathCommand},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs := newTestRouteServer(t)
			rs.handleState(&Event{PeerAddr: source, State: "up", Reason: rpc.ReasonAttached})
			rs.handleState(&Event{PeerAddr: other, State: "up", Reason: rpc.ReasonAttached})
			if tc.provider {
				payload, err := retention.Encode(tc.retainedPeer, []family.Family{tc.retainedFam})
				require.NoError(t, err)
				rs.dispatchText(`{"namespace":"` + retention.Namespace + `","event":"` +
					retention.EventType + `","payload":` + payload + `}`)
			}
			// With no provider no verdict is delivered: bgp-gr holds no state.

			entries := map[withdrawalKey]withdrawalEntry{v4a: {}, v4b: {}, v6: {}, opaque: {}, opaquePath: {}}
			otherEntries := map[withdrawalKey]withdrawalEntry{v4a: {}}
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
			require.Empty(t, rs.retained.Take(source), "the DOWN did not consume its verdict")
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

// TestPeerDownRecoveryUsesJoinedLifecycle holds the actual SDK recovery call
// across an arbitrary wait. Only successful completion or cancellation of the
// RS owner releases its joined DOWN work; ordinary command expiry owns neither.
func TestPeerDownRecoveryUsesJoinedLifecycle(t *testing.T) {
	for _, cancelOwner := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "shutdown"}[cancelOwner], func(t *testing.T) {
			rs := newTestRouteServer(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			rs.downCtx = ctx
			bridge := rpc.NewDirectBridge()
			pluginEnd, engineEnd := net.Pipe()
			t.Cleanup(func() { _ = engineEnd.Close() })
			p := sdk.NewWithConn("rs-recovery-lifetime-test", rpc.NewBridgedConn(pluginEnd, bridge))
			t.Cleanup(func() { _ = p.Close() })
			rs.plugin = p
			entered := make(chan context.Context, 1)
			release := make(chan struct{})
			bridge.SetDispatchRPC(func(call context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
				var input rpc.UpdateRouteInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, err
				}
				if method != rpc.MethodUpdateRoute || input.Meta["recovery-source"] != "192.0.2.1" {
					t.Errorf("wrong recovery request: %s %+v", method, input)
				}
				entered <- call
				select {
				case <-release:
					return json.RawMessage(`{"announced":0,"withdrawn":1}`), nil
				case <-call.Done():
					return nil, call.Err()
				}
			})
			bridge.SetReady()
			rs.withdrawals["192.0.2.1"] = map[withdrawalKey]withdrawalEntry{
				{fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("192.0.2.0/24")}: {},
			}
			rs.handleStateDown("192.0.2.1", 7)
			var call context.Context
			select {
			case call = <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("DOWN lifecycle never entered the SDK")
			}
			_, deadline := call.Deadline()
			require.False(t, deadline, "required DOWN recovery must not inherit an invented RPC expiry")
			joined := make(chan struct{})
			go func() {
				rs.stopDepartures()
				close(joined)
			}()
			select {
			case <-joined:
				t.Fatal("DOWN ownership released before its destination operation")
			default:
			}
			if cancelOwner {
				stop()
			} else {
				close(release)
			}
			select {
			case <-joined:
			case <-time.After(3 * time.Second):
				t.Fatal("DOWN lifecycle did not join after completion or shutdown")
			}
			if cancelOwner {
				require.ErrorIs(t, call.Err(), context.Canceled)
			} else {
				require.NoError(t, call.Err())
			}
		})
	}
}
