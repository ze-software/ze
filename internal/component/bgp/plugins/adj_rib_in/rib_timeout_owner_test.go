// Design: docs/architecture/plugin/rib-storage-design.md -- validation publication ownership.
package adj_rib_in

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestValidationTimeoutOwnerJoinsPublicationBeforeResult stops the actual SDK
// owner while its timeout scanner is publishing an expired pending route.
// The final exit result must include the publication failure that arrives
// after SDK shutdown, and teardown must keep route eligibility closed.
func TestValidationTimeoutOwnerJoinsPublicationBeforeResult(t *testing.T) {
	// Initialize the process-wide signal machinery outside the bubble, as the
	// registered engine still creates and releases its own signal context.
	_, stopSignals := sdk.SignalContext()
	stopSignals()
	synctest.Test(t, func(t *testing.T) {
		t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
		entered := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		defer releaseOnce.Do(func() { close(release) })
		var armed atomic.Bool
		var publications atomic.Int64
		bridgeReady := make(chan *rpc.DirectBridge, 1)
		ownerExited := make(chan int, 1)
		key := ribevents.ValidationRoute{Peer: netip.MustParseAddr(adjTransportSource),
			Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.1.0.0/24")}
		publicationError := errors.New("scanner publication failed while its owner stopped")
		engine := pluginengine.Start(t, pluginengine.TransportBridge, func(conn net.Conn) int {
			carrier, ok := conn.(rpc.Bridger)
			if !ok {
				return 2
			}
			bridge := carrier.Bridge()
			bridge.SetEmitEvent(func(namespace, eventType, _, _, event string) (int, error) {
				if namespace != ribevents.Namespace || eventType != ribevents.EventValidationChange || !armed.Load() {
					return 1, nil
				}
				var routes []ribevents.ValidationRoute
				if err := json.Unmarshal([]byte(event), &routes); err != nil {
					return 0, err
				}
				if len(routes) != 1 || routes[0] != key {
					t.Errorf("scanner published unexpected route identity: %+v", routes)
				}
				if publications.Add(1) == 1 {
					close(entered)
				}
				<-release
				return 0, publicationError
			})
			bridgeReady <- bridge
			code := runAdjRIBInPlugin(conn)
			ownerExited <- code
			return code
		})
		bridge := <-bridgeReady
		ctx := context.Background()
		if err := engine.Deliver(ctx, adjTransportStateUp(adjTransportSource)); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Execute(ctx, "request bgp adj-rib-in enable-validation", []string{"timeout", "1"}); err != nil {
			t.Fatal(err)
		}
		ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
		if err != nil {
			t.Fatal(err)
		}
		update := wireu.NewWireUpdate(rfc4271Announce(1, 24, 10, 1, 0), ctxID)
		attrs, err := update.Attrs()
		if err != nil {
			t.Fatal(err)
		}
		if err := bridge.DeliverStructured([]any{&rpc.StructuredEvent{
			EventType: rpc.EventKindUpdate, PeerAddress: adjTransportSource,
			SourcePolicyEpoch: 5,
			RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: 100,
				WireUpdate: update, AttrsWire: attrs, SourcePolicyEpoch: 5},
		}}); err != nil {
			t.Fatal(err)
		}
		if ribevents.RouteEligible(key, 100) {
			t.Fatal("pending route became eligible before the scanner promoted it")
		}
		armed.Store(true)
		<-entered
		bridge.CloseCallbacks()
		// The owner must now wait for the scanner, without sampling its final
		// failure first. Wait establishes quiescence rather than assuming that
		// an elapsed sleep gave the SDK time to return.
		synctest.Wait()
		select {
		case code := <-ownerExited:
			t.Fatalf("owner exited with status %d before its scanner settled", code)
		default:
		}
		releaseOnce.Do(func() { close(release) })
		if code := <-ownerExited; code != 1 {
			t.Errorf("owner exited with status %d after scanner failure, want 1", code)
		}
		if ribevents.RouteEligible(key, 100) {
			t.Error("owner teardown reopened eligibility after scanner publication failed")
		}
		if got := publications.Load(); got != 1 {
			t.Errorf("scanner published %d times, want exactly one failed attempt", got)
		}
	})
}
