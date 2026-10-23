package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// TestAIGPCostWithheldBeforeFirstAdvertisementRecovers pins the recovery of a
// route withheld for an unknown cost from its first receipt. The recipient was
// never advertised the route, so the cost-withheld withdrawal does not reach
// the wire (RFC 4271 Section 3.1), but the recipient stays owed the route: a
// later known distance announces it with the received AIGP plus that distance,
// without a new received UPDATE. Method: forward through both rails with no
// distance installed, assert a silent socket, install distance 11, replay, and
// assert exactly one announcement of 10.20.0.0/24 carrying AIGP 111. Then
// walk the interop checker's later distances, 7, 0 and 7: the route announced
// by recovery is withdrawn at 0 and recovered again at 7.
func TestAIGPCostWithheldBeforeFirstAdvertisementRecovers(t *testing.T) {
	prefix := []byte{24, 10, 20, 0}
	for _, rail := range []string{"general", "rs-cached"} {
		t.Run(rail, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			f.source.settings.RSClient = rail == "rs-cached"
			f.source.refreshForwardFacts()
			f.destination.settings.RSClient = rail == "rs-cached"
			f.destination.settings.AIGPLinkMetric = 43
			f.destination.refreshForwardFacts()
			id := f.receive(t, f.body(t, 100))
			if rail == "rs-cached" {
				api := &reactorAPIAdapter{r: f.r}
				if err := api.ForwardUpdatesDirect([]uint64{id}, []netip.AddrPort{f.destination.Settings().PeerKey()}, "aigp-forwarder", plugin.ProcessSender("aigp-forwarder")); err != nil {
					t.Fatal(err)
				}
			} else {
				f.forward(t, id)
			}
			forwardSocketBarrier(t, f.r)
			if bodies := aigpSocketBodies(t, f.conn); len(bodies) != 0 {
				t.Fatalf("never-advertised recipient got %d UPDATEs while the cost is unknown, want none: %x", len(bodies), bodies)
			}
			f.metric(11)
			f.r.readvertiseAIGP()
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			if len(bodies) != 1 {
				t.Fatalf("recovery UPDATE count=%d, want 1; the cost-withheld recipient was not retained", len(bodies))
			}
			recovered := bodies[0]
			sections, err := wire.ParseUpdateSections(recovered)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(sections.NLRI(recovered), prefix) {
				t.Fatalf("recovery did not announce 10.20.0.0/24: %x", recovered)
			}
			if len(sections.Withdrawn(recovered)) != 0 {
				t.Fatalf("recovery carries a withdrawal: %x", recovered)
			}
			metric, present := aigpReceivedMetric(t, recovered)
			if !present {
				t.Fatal("recovered route has no AIGP")
			}
			if metric != 111 {
				t.Fatalf("recovered metric=%d, want received 100 plus distance 11", metric)
			}
			// The checker's sequence after the first recovery: distance 7
			// re-advertises, distance 0 withdraws the now advertised route, and
			// distance 7 recovers it again.
			seen := len(bodies)
			for _, step := range []struct {
				distance uint32
				metric   uint64
			}{{7, 107}, {0, 0}, {7, 107}} {
				f.metric(step.distance)
				f.r.readvertiseAIGP()
				forwardSocketBarrier(t, f.r)
				bodies = aigpSocketBodies(t, f.conn)
				if len(bodies) != seen+1 {
					t.Fatalf("distance %d: UPDATE count=%d, want %d", step.distance, len(bodies), seen+1)
				}
				seen = len(bodies)
				last := bodies[len(bodies)-1]
				sections, err := wire.ParseUpdateSections(last)
				if err != nil {
					t.Fatal(err)
				}
				if step.metric == 0 {
					if !bytes.Equal(sections.Withdrawn(last), prefix) {
						t.Fatalf("distance 0 did not withdraw the advertised route: %x", last)
					}
					continue
				}
				if !bytes.Equal(sections.NLRI(last), prefix) {
					t.Fatalf("distance %d did not announce 10.20.0.0/24: %x", step.distance, last)
				}
				got, present := aigpReceivedMetric(t, last)
				if !present {
					t.Fatalf("distance %d: announcement has no AIGP", step.distance)
				}
				if got != step.metric {
					t.Fatalf("distance %d: metric=%d, want %d", step.distance, got, step.metric)
				}
			}
		})
	}
}

// TestAIGPReplayNotQueuedTwiceForOneOutcome pins one replay per outcome. Every
// committed write signals a new pass, so a pass for another recipient can run
// while this recipient's replay still waits in its forward queue. Method: hold
// the destination worker, run two passes at one Loc-RIB revision, release, and
// assert one re-advertisement with AIGP 111, not the same UPDATE twice.
func TestAIGPReplayNotQueuedTwiceForOneOutcome(t *testing.T) {
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	f.forward(t, f.receive(t, f.body(t, 100)))
	forwardSocketBarrier(t, f.r)
	if bodies := aigpSocketBodies(t, f.conn); len(bodies) != 1 {
		t.Fatalf("initial UPDATE count=%d, want 1", len(bodies))
	}
	gate, held := make(chan struct{}), make(chan struct{})
	if !f.r.fwdPool.Dispatch(fwdKey{peerAddr: f.destination.Settings().PeerKey()}, &fwdItem{done: func() { close(held); <-gate }}) {
		t.Fatal("hold item not dispatched")
	}
	<-held
	f.metric(11)
	f.r.readvertiseAIGP()
	f.r.readvertiseAIGP()
	close(gate)
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	if len(bodies) != 2 {
		t.Fatalf("UPDATE count=%d, want the initial one and one re-advertisement: %x", len(bodies), bodies)
	}
	metric, present := aigpReceivedMetric(t, bodies[1])
	if !present {
		t.Fatal("re-advertisement has no AIGP")
	}
	if metric != 111 {
		t.Fatalf("re-advertised metric=%d, want 111", metric)
	}
}
