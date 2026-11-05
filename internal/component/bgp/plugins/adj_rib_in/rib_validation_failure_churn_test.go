// Design: docs/architecture/plugin/rib-storage-design.md -- failed validation publication ownership.
// Related: rib_eligibility.go -- retained notification debt and receive admission.
package adj_rib_in

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestValidationPublisherFailureFencesDistinctReceiveChurn reaches the real
// registered plugin's receive callback and SDK publication path. A positively
// acknowledged, eligible route is withdrawn while the engine refuses its
// notification. Distinct announce/withdraw pairs must not keep accumulating
// absent-key debt in an otherwise empty store: the existing owner lifecycle
// must fence receive admission, report failure, and leave the gate closed.
func TestValidationPublisherFailureFencesDistinctReceiveChurn(t *testing.T) {
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	var refuse atomic.Bool
	var acknowledged atomic.Int64
	var refused atomic.Int64
	failureObserved := make(chan struct{}, 1)
	bridgeReady := make(chan *rpc.DirectBridge, 1)
	ownerExited := make(chan int, 1)
	publicationError := errors.New("engine refused validation withdrawal publication")
	engine := pluginengine.Start(t, pluginengine.TransportBridge, func(conn net.Conn) int {
		carrier, ok := conn.(rpc.Bridger)
		if !ok {
			return 2
		}
		bridge := carrier.Bridge()
		bridge.SetEmitEvent(func(namespace, eventType, _, _, event string) (int, error) {
			if namespace != ribevents.Namespace || eventType != ribevents.EventValidationChange {
				return 1, nil
			}
			var routes []ribevents.ValidationRoute
			if err := json.Unmarshal([]byte(event), &routes); err != nil {
				return 0, err
			}
			if refuse.Load() {
				refused.Add(1)
				select {
				case failureObserved <- struct{}{}:
				default:
				}
				return 0, publicationError
			}
			acknowledged.Add(int64(len(routes)))
			return 1, nil
		})
		bridgeReady <- bridge
		code := runAdjRIBInPlugin(conn)
		ownerExited <- code
		return code
	})
	bridge := <-bridgeReady
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, engine.Deliver(ctx, adjTransportStateUp(adjTransportSource)))
	_, err := engine.Execute(ctx, "request bgp adj-rib-in enable-validation", nil)
	require.NoError(t, err)
	_, err = engine.Execute(ctx, "request bgp adj-rib-in disable-validation", nil)
	require.NoError(t, err)

	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	deliver := func(id uint64, prefixIndex byte, withdraw bool) error {
		body := rfc4271Announce(1, 24, 10, prefixIndex, 0)
		if withdraw {
			body = []byte{0, 4, 24, 10, prefixIndex, 0, 0, 0}
		}
		update := wireu.NewWireUpdate(body, ctxID)
		attrs, err := update.Attrs()
		if err != nil {
			return err
		}
		return bridge.DeliverStructured([]any{&rpc.StructuredEvent{
			EventType: rpc.EventKindUpdate, PeerAddress: adjTransportSource,
			SourcePolicyEpoch: 5,
			RawMessage: &bgptypes.RawMessage{
				Type: msgtype.TypeUPDATE, MessageID: id, WireUpdate: update,
				AttrsWire: attrs, SourcePolicyEpoch: 5,
			},
		}})
	}
	key := ribevents.ValidationRoute{Peer: netip.MustParseAddr(adjTransportSource),
		Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.1.0.0/24")}
	require.NoError(t, deliver(100, 1, false))
	require.Equal(t, int64(1), acknowledged.Load(), "the real engine callback acknowledged the initial route")
	require.True(t, ribevents.RouteEligible(key, 100), "the positively published route is actually installed and eligible")

	refuse.Store(true)
	// The callback may itself return the publication error while closing, or
	// finish its admitted delivery before the owner fences the next one.
	_ = deliver(101, 1, true)
	select {
	case <-failureObserved:
	case <-ctx.Done():
		t.Fatal("the actual withdrawal never reached the refusing engine consumer: ", ctx.Err())
	}
	require.False(t, ribevents.RouteEligible(key, 0), "the failed notification owes an actual removal, not an unchanged installed route")

	acceptedChurn := 0
churn:
	for index := byte(2); index < 10; index++ {
		for _, withdraw := range []bool{false, true} {
			if err := deliver(100+uint64(index)*2+uint64(acceptedChurn), index, withdraw); err != nil {
				break churn
			}
			acceptedChurn++
		}
	}
	if acceptedChurn > 1 {
		t.Errorf("publisher accepted %d distinct announce/withdraw deliveries after failed withdrawal publication; terminal debt must fence further intake", acceptedChurn)
	}
	select {
	case code := <-ownerExited:
		if code != 1 {
			t.Errorf("publisher owner exited with status %d, want failure status 1", code)
		}
	case <-ctx.Done():
		t.Fatal("publisher owner remained live with terminal withdrawal debt after bounded distinct-key churn: ", ctx.Err())
	}
	if !errors.Is(bridge.DeliverStructured(nil), rpc.ErrBridgeClosed) {
		t.Error("terminated publisher still admits receive deliveries")
	}
	if ribevents.RouteEligible(key, 0) {
		t.Error("publisher teardown reopened eligibility for the withdrawn path before its obligation was retired")
	}
	if refused.Load() != 1 {
		t.Errorf("publisher retried failed delivery autonomously %d times, want exactly the original refusal", refused.Load())
	}
}
