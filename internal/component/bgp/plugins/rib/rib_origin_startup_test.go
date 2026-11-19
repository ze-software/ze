// Design: docs/architecture/plugin/rib-storage-design.md -- real origin provider startup.
package rib

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// originIntakeFixture preserves distinct configured owner, physical Session,
// native key and legal explicit path zero; no received-source lease is invented.
func originIntakeFixture(peer netip.Addr, owner, receipt uint64, last byte) rpc.OriginIntakeInput {
	return rpc.OriginIntakeInput{Operation: rpc.OriginIntakeAnnounce,
		Destination: peer.String(), PeerOwner: owner, OriginReceipt: receipt,
		DeadlineMS: time.Now().Add(20 * time.Second).UnixMilli(),
		Producer:   rpc.OriginProducer{Kind: rpc.OriginProducerProcess, Name: "local-producer"},
		Source:     rpc.OriginSource{Kind: rpc.OriginSourceLocalBGP},
		Route: rpc.OriginBuild{Family: "ipv4/unicast", KeyNLRI: []byte{24, 198, 18, last},
			SourceAddPath: true, SourcePathID: 0, AttributesExplicit: true,
			Attributes: []byte{0x40, 1, 1, 0}, NextHopPolicy: rpc.OriginNextHopSelf}}
}

// TestRIBOriginIntakeRealStartupScopesAndRetains drives the real RIB startup
// and registered callback over both transports. Useful common learned output
// precedes A-only local replacement, denied replacement and exact removal.
func TestRIBOriginIntakeRealStartupScopesAndRetains(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(fmt.Sprintf("native=%t", native), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			peers := [2]netip.Addr{netip.MustParseAddr("192.0.2.211"), netip.MustParseAddr("192.0.2.212")}
			var mu sync.Mutex
			var writes []rpc.SelectRoutesInput
			denied := false
			manager, r := startQuiesceRIB(t, ctx, native, peers, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				writes = append(writes, in)
				if denied && in.Destination == peers[0].String() {
					return selectedDispatchAnswer(in, rpc.SelectedSuppressed), nil
				}
				return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
			})
			learned := selectedDispatchRoute(211, 811)
			require.NoError(t, r.selectedDispatch.publish(&learned))
			for i, peer := range peers {
				r.handleStructuredState(&rpc.StructuredEvent{
					PeerAddress: peer.String(), PeerOwner: uint64(311 + i), DestinationSession: uint64(411 + i),
					SelectedDestination: true, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached})
			}
			require.NoError(t, r.selectedDispatch.barrier(ctx))
			mu.Lock()
			require.Len(t, writes, 2)
			writes = nil
			mu.Unlock()
			input := originIntakeFixture(peers[0], 311, 901, 211)
			output, err := manager.GetProcess("bgp-rib").Conn().SendOriginIntake(ctx, input)
			require.NoError(t, err, "real startup must register the optional origin provider")
			require.NoError(t, rpc.ValidateOriginIntakeOutput(input, output))
			require.Equal(t, uint32(1), output.Announced)
			require.Equal(t, rpc.RouteAdmissionSelected, output.Admissions[0].Disposition)
			mu.Lock()
			require.Len(t, writes, 1)
			require.Equal(t, peers[0].String(), writes[0].Destination)
			require.Equal(t, rpc.SelectedRouteOrigin, writes[0].Routes[0].Kind)
			require.Equal(t, uint64(901), writes[0].Routes[0].OriginReceipt)
			writes = nil
			denied = true
			mu.Unlock()
			input.OriginReceipt = 902
			output, err = manager.GetProcess("bgp-rib").Conn().SendOriginIntake(ctx, input)
			require.NoError(t, err)
			require.Zero(t, output.Announced)
			require.Equal(t, rpc.RouteAdmissionSuppressedExport, output.Admissions[0].Disposition)
			mu.Lock()
			require.Len(t, writes, 1)
			require.Equal(t, rpc.SelectedRouteOrigin, writes[0].Routes[0].Kind)
			writes = nil
			denied = false
			mu.Unlock()
			input.Operation = rpc.OriginIntakeWithdraw
			output, err = manager.GetProcess("bgp-rib").Conn().SendOriginIntake(ctx, input)
			require.NoError(t, err)
			require.Equal(t, uint32(1), output.Withdrawn)
			mu.Lock()
			require.Len(t, writes, 1)
			require.Equal(t, peers[0].String(), writes[0].Destination)
			require.Equal(t, rpc.SelectedRouteReceived, writes[0].Routes[0].Kind)
			require.Equal(t, uint64(811), writes[0].Routes[0].SourceReceipt)
			mu.Unlock()
		})
	}
}
