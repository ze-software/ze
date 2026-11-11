// Design: docs/guide/vpp.md -- VPP interface event ownership
// Related: monitor.go -- stop releases the subscription and joins dispatch
// Related: vendor/go.fd.io/govpp/core/channel.go -- Unsubscribe closes notifications

package ifacevpp

import (
	"errors"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
)

type closingMonitorChannel struct {
	monitorChannel
	unsubscribeErr error
	unsubscribes   int
}

func (c *closingMonitorChannel) SubscribeNotification(notif chan api.Message, _ api.Message) (api.SubscriptionCtx, error) {
	c.notif = notif
	// Queue before StartMonitor launches dispatch, so stop must preserve both.
	notif <- &interfaces.SwInterfaceEvent{SwIfIndex: 7}
	notif <- &interfaces.SwInterfaceEvent{SwIfIndex: 8}
	return &closingMonitorSubscription{channel: c, notif: notif}, nil
}

type closingMonitorSubscription struct {
	channel *closingMonitorChannel
	notif   chan api.Message
}

func (s *closingMonitorSubscription) Unsubscribe() error {
	s.channel.unsubscribes++
	if s.channel.unsubscribeErr != nil {
		return s.channel.unsubscribeErr
	}
	close(s.notif)
	return nil
}

// TestMonitorStopRespectsSubscriptionChannelOwnership validates GoVPP's
// producer-owned close and termination after an unsuccessful unsubscribe.
// It prevents double-close panics, lost queued events and a stuck shutdown.
func TestMonitorStopRespectsSubscriptionChannelOwnership(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "producer-closes"
		ch := &closingMonitorChannel{}
		if fail {
			name = "unsubscribe-fails-with-open-channel"
			ch.unsubscribeErr = errors.New("subscription disappeared")
		}
		t.Run(name, func(t *testing.T) {
			b := &vppBackendImpl{ch: ch, names: newNameMap()}
			bus := &recordingBus{}
			if err := b.StartMonitor(bus); err != nil {
				t.Fatal(err)
			}
			m := b.mon
			stopped := make(chan struct{})
			go func() {
				b.StopMonitor()
				b.StopMonitor()
				close(stopped)
			}()
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("StopMonitor did not join dispatch")
			}
			select {
			case <-m.done:
			default:
				t.Fatal("StopMonitor returned before dispatch joined")
			}
			if ch.unsubscribes != 1 || len(ch.wantOffs) != 2 || ch.wantOffs[1] {
				t.Fatalf("unsubscribe count=%d, enable history=%v", ch.unsubscribes, ch.wantOffs)
			}
			if bus.len() != 2 {
				t.Fatalf("queued events delivered=%d, want 2", bus.len())
			}
		})
	}
}
