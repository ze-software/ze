package reactor

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc8212BestChangeConsumerName is the downstream fixture's process name.
const rfc8212BestChangeConsumerName = "rfc8212-best-change-consumer"

// rfc8212BestChangeWriteDelay is how long the downstream consumer works on a
// best-change before its destination write lands. It is long enough that a
// coordinator that does not wait for the consumer returns first.
const rfc8212BestChangeWriteDelay = 300 * time.Millisecond

// registerRFC8212BestChangeConsumer registers a process-protocol subscriber to
// the RIB best-change event, the shape of a downstream redistribution consumer
// running as a plugin: its handler works, then issues its destination write,
// counted in written.
func registerRFC8212BestChangeConsumer(t *testing.T, wire bool, written *atomic.Int32) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: rfc8212BestChangeConsumerName, Description: "RFC 8212 best-change consumer fixture",
		RunEngine: func(conn net.Conn) int {
			if wire {
				conn = struct{ net.Conn }{conn}
			}
			p := sdk.NewWithConn(rfc8212BestChangeConsumerName, conn)
			defer p.Close() //nolint:errcheck // The server owns shutdown.
			p.SetStartupSubscriptionsIn(ribevents.Namespace, []string{ribevents.EventBestChange}, nil, "json")
			p.OnEvent(func(string) error {
				time.Sleep(rfc8212BestChangeWriteDelay)
				written.Add(1)
				return nil
			})
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
}

// rfc8212EmitBestChange is the owner's own best-path recomputation: from
// inside its lifecycle callback it publishes a best-change, as bgp-rib does
// when a source's paths become ineligible or are restored.
func rfc8212EmitBestChange(t *testing.T, owner *rfc8212ImportOwner) {
	p := owner.sdkPlugin()
	if p == nil {
		t.Error("owner emitted before its engine ran")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	delivered, err := p.EmitEvent(ctx, ribevents.Namespace, ribevents.EventBestChange, "", "",
		`{"protocol":"bgp","family":"ipv4/unicast","changes":[]}`)
	if err != nil {
		t.Errorf("emit best-change: %v", err)
		return
	}
	if delivered != 1 {
		t.Errorf("best-change delivered to %d processes, want the one consumer", delivered)
	}
}

// TestRFC8212ImportSettlesBestChangeSubscribers proves AC-14's downstream half:
// a destination write a process-protocol best-change subscriber issues in
// answer to the owner's apply, or to its abort, lands before the coordinator's
// apply, or its undo, returns. Method: the fixture owner emits a best-change
// from inside its apply (or abort) callback; a second SDK plugin subscribed to
// bgp-rib/best-change records its write only after a delay. Both transports.
func TestRFC8212ImportSettlesBestChangeSubscribers(t *testing.T) {
	for _, wire := range []bool{false, true} {
		for _, phase := range []string{"apply", "abort"} {
			t.Run(fmt.Sprintf("wire=%t/%s", wire, phase), func(t *testing.T) {
				var written atomic.Int32
				registerRFC8212BestChangeConsumer(t, wire, &written)
				fx := newRFC8212ImportFixtureWith(t, wire, rfc8212BestChangeConsumerName)
				emit := func() { rfc8212EmitBestChange(t, fx.owner) }
				if phase == "apply" {
					early := fx.owner.onApply
					fx.owner.onApply = func() { early(); emit() }
				} else {
					fx.owner.onAbort = emit
				}
				j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-downstream-" + phase,
					deadline: time.Now().Add(30 * time.Second)}
				require.NoError(t, fx.swap(j))
				if phase == "apply" {
					require.Equal(t, int32(1), written.Load(), "downstream write did not land before apply returned")
					require.NoError(t, fx.r.CheckSourcePolicyTx(j.txID))
					require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, true))
					j.Discard()
					fx.settled(t, false)
					return
				}
				require.Zero(t, written.Load(), "best-change emitted before abort")
				require.Empty(t, j.Rollback())
				require.Equal(t, int32(1), written.Load(), "downstream write did not land before undo returned")
				require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
				fx.settled(t, true)
			})
		}
	}
}
