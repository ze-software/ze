// Design: docs/architecture/behavior/peer-lifecycle.md -- in-place import invalidation owners.
// Related: rfc8212_downstream_settle_test.go -- the best-change settle these extend.
package reactor

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// rfc8212ValidationConsumerName is the validation-change subscriber fixture.
const rfc8212ValidationConsumerName = "rfc8212-validation-consumer"

// rfc8212ForwarderName is a captured owner that declares it forwards.
const rfc8212ForwarderName = "rfc8212-forwarder"

// TestRFC8212ImportSettlesValidationChangeSubscribers proves the coordinator
// settles the relays a validation change causes: bgp-adj-rib-in publishes one
// on apply and abort when its validation gate is on, and bgp-rib answers it
// with best changes a forwarder relays. Method: the owner emits a
// validation-change from inside its apply (or abort) callback; a plugin
// subscribed to bgp-rib/validation-change records its write only after a
// delay. The write must land before apply, or undo, returns. Both transports.
func TestRFC8212ImportSettlesValidationChangeSubscribers(t *testing.T) {
	for _, wire := range []bool{false, true} {
		for _, phase := range []string{"apply", "abort"} {
			t.Run(fmt.Sprintf("wire=%t/%s", wire, phase), func(t *testing.T) {
				var written atomic.Int32
				snapshot := registry.Snapshot()
				t.Cleanup(func() { registry.Restore(snapshot) })
				require.NoError(t, registry.Register(registry.Registration{
					Name: rfc8212ValidationConsumerName, Description: "RFC 8212 validation-change consumer fixture",
					RunEngine: func(conn net.Conn) int {
						if wire {
							conn = struct{ net.Conn }{conn}
						}
						p := sdk.NewWithConn(rfc8212ValidationConsumerName, conn)
						defer p.Close() //nolint:errcheck // The server owns shutdown.
						p.SetStartupSubscriptionsIn(ribevents.Namespace, []string{ribevents.EventValidationChange}, nil, "json")
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
				fx := newRFC8212ImportFixtureWith(t, wire, rfc8212ValidationConsumerName)
				emit := func() {
					p := fx.owner.sdkPlugin()
					if p == nil {
						t.Error("owner emitted before its engine ran")
						return
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					delivered, err := p.EmitEvent(ctx, ribevents.Namespace, ribevents.EventValidationChange, "", "", `[]`)
					if err != nil {
						t.Errorf("emit validation-change: %v", err)
						return
					}
					if delivered != 1 {
						t.Errorf("validation-change delivered to %d processes, want the one consumer", delivered)
					}
				}
				if phase == "apply" {
					early := fx.owner.onApply
					fx.owner.onApply = func() { early(); emit() }
				} else {
					fx.owner.onAbort = emit
				}
				j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-validation-" + phase,
					deadline: time.Now().Add(30 * time.Second)}
				require.NoError(t, fx.swap(j))
				if phase == "apply" {
					require.Equal(t, int32(1), written.Load(), "validation-change relay did not land before apply returned")
					require.NoError(t, fx.r.CheckSourcePolicyTx(j.txID))
					require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, true))
					j.Discard()
					fx.settled(t, false)
					return
				}
				require.Zero(t, written.Load(), "validation-change emitted before abort")
				require.Empty(t, j.Rollback())
				require.Equal(t, int32(1), written.Load(), "validation-change relay did not land before undo returned")
				require.NoError(t, fx.r.CompleteSourcePolicyTx(t.Context(), j.txID, false))
				fx.settled(t, true)
			})
		}
	}
}

// TestRFC8212ImportRefusesForwarderWithoutSentOwner proves the coordinator
// fails closed when a forwarder receives the source's UPDATEs but no captured
// owner holds the sent inventory: nobody would withdraw the copies it sent.
// Method: a second owner that declares registry.FencesLiveForwards is attached
// to the source, and no process answers the RIB recovery command. The policy
// removal is refused before anything is published, with a message naming the
// peer, the forwarder and the remedy; no owner is called, the import policy
// still accepts and input is not left paused.
func TestRFC8212ImportRefusesForwarderWithoutSentOwner(t *testing.T) {
	var called atomic.Int32
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: rfc8212ForwarderName, Description: "RFC 8212 forwarding owner fixture", FencesLiveForwards: true, SignalsSessionReady: true,
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(rfc8212ForwarderName, conn)
			defer p.Close() //nolint:errcheck // The server owns shutdown.
			p.OnSourcePolicy(func(context.Context, rpc.SourcePolicyInput) error {
				called.Add(1)
				return nil
			})
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	fx := newRFC8212ImportFixtureWith(t, false, rfc8212ForwarderName)
	fx.r.mu.Lock()
	fx.peer.settings.ProcessBindings = append(fx.peer.settings.ProcessBindings, ProcessBinding{PluginName: rfc8212ForwarderName,
		Receive: map[string]events.Direction{bgpevents.EventUpdate: events.DirReceived}})
	fx.r.publishDeliveryGraphLocked()
	fx.r.mu.Unlock()
	j := &rfc8212TxJournal{Journal: sdk.NewJournal(), txID: "tx-no-sent-owner", deadline: time.Now().Add(30 * time.Second)}
	err := fx.swap(j)
	require.Error(t, err, "a forwarder without the sent-inventory owner was invalidated")
	for _, part := range []string{fx.peer.addrString, rfc8212ForwarderName, "refused before anything is published", "update-received"} {
		require.True(t, strings.Contains(err.Error(), part), "refusal %q does not name %q", err.Error(), part)
	}
	require.Zero(t, called.Load(), "the forwarder was called before the refusal")
	s := fx.peer.settingsSnapshot()
	require.True(t, s.ImportPolicy.AcceptsAtEnd(s.IsEBGP()), "the refused change was published")
	require.False(t, fx.f.s.IsPaused(), "input left paused")
}
