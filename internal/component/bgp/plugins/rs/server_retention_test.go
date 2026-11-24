// Design: docs/guide/graceful-restart.md -- source-family retention on route-server DOWN.

package rs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
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
	v4a := withdrawalKey{owner: 1, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.0.0/24")}
	v4b := withdrawalKey{owner: 1, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.1.0/24")}
	v6 := withdrawalKey{owner: 1, fam: family.IPv6Unicast, prefix: netip.MustParsePrefix("2001:db8::/48")}
	opaque := withdrawalKey{owner: 1, fam: linkState, nlriStr: bgplsNLRIUnknown, wireForm: true}
	opaquePath := withdrawalKey{owner: 1, fam: linkState, nlriStr: "00000007" + bgplsNLRIUnknown, wireForm: true, addPath: true}
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
			rs.handleState(&Event{PeerAddr: source, PeerOwner: 1, ConnectionID: 101, DestinationSession: 201, SessionGeneration: 7, State: "up", Reason: rpc.ReasonAttached, RSClient: true})
			rs.handleState(&Event{PeerAddr: other, PeerOwner: 2, ConnectionID: 102, DestinationSession: 202, SessionGeneration: 7, State: "up", Reason: rpc.ReasonAttached, RSClient: true})
			if tc.provider {
				retainedOwner, retainedConnection := uint64(1), uint64(101)
				if tc.retainedPeer == other {
					retainedOwner = 2
					retainedConnection = 102
				}
				payload, err := retention.Encode(retainedOwner, retainedConnection, tc.retainedPeer, []family.Family{tc.retainedFam})
				require.NoError(t, err)
				rs.dispatchText(`{"namespace":"` + retention.Namespace + `","event":"` +
					retention.EventType + `","payload":` + payload + `}`)
			}
			// With no provider no verdict is delivered: bgp-gr holds no state.

			entries := map[withdrawalKey]withdrawalEntry{v4a: {}, v4b: {}, v6: {}, opaque: {}, opaquePath: {}}
			for native, entry := range entries {
				entry.owner = 1
				entry.connectionID = 101
				entry.sourceReceipt = 301
				entry.msgID = 1
				entries[native] = entry
			}
			require.NoError(t, rs.acquireSource(rsFixtureKey(source, 1, 101), 301))
			otherKey := v4a
			otherKey.owner = 2
			otherEntries := map[withdrawalKey]withdrawalEntry{otherKey: {owner: 2, connectionID: 102, sourceReceipt: 302, msgID: 2}}
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

			original := rs.peers[rsFixtureKey(source, 1, 101)]
			rs.handleState(&Event{PeerAddr: source, PeerOwner: 1, ConnectionID: 101, SessionGeneration: 7, State: "down"})
			require.NoError(t, rs.settlePendingDown(t.Context(), rsFixtureKey(source, 1, 101)))
			keptVerdict, verdictErr := rs.retained.Take(1, 101)
			require.NoError(t, verdictErr)
			require.Empty(t, keptVerdict, "the DOWN did not consume its verdict")
			require.Equal(t, otherEntries, rs.withdrawals[other])
			require.False(t, original.Up)
			require.True(t, original.StateSeen)
			var departed, retained []withdrawalKey
			for _, native := range []withdrawalKey{v4a, v4b, v6, opaque, opaquePath} {
				if entry, exists := rs.withdrawals[source][native]; exists {
					require.Equal(t, uint64(1), entry.owner)
					require.Equal(t, uint64(101), entry.connectionID)
					require.Equal(t, uint64(301), entry.sourceReceipt)
					require.True(t, entry.retained, "kept family lost its exact retained authority")
					retained = append(retained, native)
				} else {
					departed = append(departed, native)
				}
			}
			require.ElementsMatch(t, tc.wantEntries, departed, "every unretained native row must be absent from current membership")
			require.Len(t, retained, 5-len(tc.wantEntries), "retained family siblings must remain useful")

			var commands []string
			deadline := time.NewTimer(2 * time.Second)
			defer deadline.Stop()
			for range tc.wantCommands {
				select {
				case got := <-output:
					require.Equal(t, other, got.selector)
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
	for _, mode := range []string{"joined", "bounded-refusal", "owner-canceled"} {
		t.Run(mode, func(t *testing.T) {
			rs := newTestRouteServer(t)
			owner, stopOwner := context.WithCancel(context.Background())
			defer stopOwner()
			rs.downCtx = owner
			_, ownerDeadline := owner.Deadline()
			require.False(t, ownerDeadline, "the required original owner lifetime must outlive individual SDK request budgets")
			bridge := rpc.NewDirectBridge()
			pluginEnd, engineEnd := net.Pipe()
			t.Cleanup(func() { _ = engineEnd.Close() })
			plugin := sdk.NewWithConn("rs-recovery-lifetime-test", rpc.NewBridgedConn(pluginEnd, bridge))
			t.Cleanup(func() { _ = plugin.Close() })
			rs.plugin = plugin
			var calls, releases atomic.Int32
			bridge.SetDispatchNative(func(ctx context.Context, method string, value any) (any, bool, error) {
				if method == rpc.MethodSourceLease {
					input, ok := value.(rpc.SourceLeaseInput)
					if !ok {
						return nil, true, errors.New("wrong original lease envelope")
					}
					if input.PeerOwner != 1 || input.SourceReceipt != 301 {
						return nil, true, errors.New("native recovery changed its original source lease")
					}
					if input.Action == rpc.SourceLeaseRelease {
						releases.Add(1)
					}
				}
				return rsFixtureNativeOperation(ctx, method, value)
			})
			type originalCall struct {
				ctx   context.Context
				input rpc.UpdateRouteInput
			}
			entered := make(chan originalCall, 1)
			release := make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			bridge.SetDispatchRPC(func(call context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
				var input rpc.UpdateRouteInput
				if err := json.Unmarshal(raw, &input); err != nil {
					return nil, err
				}
				if method != rpc.MethodUpdateRoute || input.Meta["recovery-source"] != "192.0.2.1" || input.Meta["recovery-source-owner"] != "1" {
					return nil, errors.New("native recovery lost its original source scope")
				}
				if input.DestinationOwner != 2 || input.DestinationSession != 202 {
					return nil, errors.New("native recovery escaped its original destination UP")
				}
				if calls.Add(1) != 1 {
					return json.RawMessage(`{"announced":0,"withdrawn":1}`), nil
				}
				entered <- originalCall{ctx: call, input: input}
				select {
				case <-release:
					if mode == "bounded-refusal" {
						return nil, rpc.CachedWriteNotDispatched(errors.New("bounded original request refused before writer admission"))
					}
					return json.RawMessage(`{"announced":0,"withdrawn":1}`), nil
				case <-call.Done():
					return nil, call.Err()
				}
			})
			bridge.SetReady()
			source := rsFixtureKey("192.0.2.1", 1, 101)
			rs.handleState(&Event{PeerAddr: "192.0.2.1", PeerOwner: 1, ConnectionID: 101, DestinationSession: 201, SessionGeneration: 7, State: "up", Reason: rpc.ReasonAttached, RSClient: true})
			rs.handleState(&Event{PeerAddr: "192.0.2.2", PeerOwner: 2, ConnectionID: 102, DestinationSession: 202, SessionGeneration: 8, State: "up", Reason: rpc.ReasonAttached, RSClient: true})
			require.NoError(t, rs.acquireSource(source, 301))
			original := rs.peers[source]
			rs.withdrawals["192.0.2.1"] = map[withdrawalKey]withdrawalEntry{{owner: 1, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("192.0.2.0/24")}: {owner: 1, connectionID: 101, sourceReceipt: 301}}
			rs.handleState(&Event{PeerAddr: "192.0.2.1", PeerOwner: 1, ConnectionID: 101, SessionGeneration: 7, State: "down"})
			var call originalCall
			select {
			case call = <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("DOWN lifecycle never entered its actual bound SDK call")
			}
			deadline, bounded := call.ctx.Deadline()
			require.True(t, bounded, "each actual SDK request must retain its finite transport budget")
			require.Equal(t, deadline.UnixMilli(), call.input.DeadlineMS, "the wire request lost its actual SDK deadline")
			joined := make(chan struct{})
			go func() { rs.stopDepartures(); close(joined) }()
			select {
			case <-joined:
				t.Fatal("DOWN ownership released before its held destination operation")
			default:
			}
			if mode == "owner-canceled" {
				stopOwner()
			} else {
				releaseOnce.Do(func() { close(release) })
			}
			select {
			case <-joined:
			case <-time.After(3 * time.Second):
				t.Fatal("DOWN worker did not join after its call completed or its owner stopped")
			}
			if mode == "joined" {
				require.Equal(t, int32(1), releases.Load(), "joined native output did not release its original lease")
				require.False(t, original.sourceLease)
				return
			}
			require.Equal(t, int32(0), releases.Load(), "a failed bounded request declared source cleanup complete")
			require.True(t, original.sourceLease, "failed output lost its original source authority before reconciliation")
			require.NotNil(t, original.departure, "failed output erased its current native route/target obligation")
			require.Len(t, original.departure.routes, 1)
			require.Len(t, original.departure.targets, 1)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if mode == "owner-canceled" {
				require.ErrorIs(t, call.ctx.Err(), context.Canceled)
				err := rs.settleRetainedEvents(ctx)
				require.ErrorContains(t, err, "192.0.2.2 owner 2 session 202", "UNKNOWN remote completion must refuse the exact original scope")
				require.Equal(t, int32(1), calls.Load(), "UNKNOWN native output was blindly retried")
				require.Equal(t, int32(0), releases.Load(), "an ordinary owner cancellation was accepted as a final-writer join")
				return
			}
			require.NoError(t, owner.Err(), "an individual bounded refusal canceled the owner's longer operation lifetime")
			require.NoError(t, rs.settleRetainedEvents(ctx), "a positive non-dispatch witness and exact successful retry must reconcile current work")
			require.Equal(t, int32(2), calls.Load())
			require.Equal(t, int32(1), releases.Load())
			require.False(t, original.sourceLease)
		})
	}
}
