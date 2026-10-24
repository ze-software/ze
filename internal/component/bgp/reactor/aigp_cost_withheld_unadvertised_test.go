package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// TestAIGPRecoveryWithdrawalKeepsCostWithheldRecord pins the AIGP write
// receipt for a write that decides nothing about AIGP. A route withheld for an
// unknown cost stays owed to the recipient (RFC 7311 Section 3.4.3), and a
// later known distance re-advertises it. A relay recovery withdraws an obsolete
// owner with a bare withdrawal that carries no AIGP origin
// (queueRecoveryWithdrawal); the receipt once read it as an ordinary
// withdrawal and dropped the recovery record, so the route never came back.
// Method: advertise at distance 7, withdraw by replay at 0, take the write
// receipt of the recovery withdrawal of the same prefix, then install 7 and
// assert one announcement with AIGP 107.
func TestAIGPRecoveryWithdrawalKeepsCostWithheldRecord(t *testing.T) {
	prefix := []byte{24, 10, 20, 0}
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	f.forward(t, f.receive(t, f.body(t, 100)))
	forwardSocketBarrier(t, f.r)
	f.metric(0)
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	if len(bodies) != 2 {
		t.Fatalf("UPDATE count=%d after distance 0, want the announcement and its withdrawal", len(bodies))
	}
	if sections, err := wire.ParseUpdateSections(bodies[1]); err != nil || !bytes.Equal(sections.Withdrawn(bodies[1]), prefix) {
		t.Fatalf("distance 0 did not withdraw 10.20.0.0/24: %x", bodies[1])
	}
	// The receipt of the body queueRecoveryWithdrawal stages: destination
	// framing, no AIGP origin, revision or cost reason. It reaches the wire
	// only while the destination's Adj-RIB-Out still holds the key, as when
	// the replay's withdrawal was owed to the recovery worker, so the receipt
	// is driven as writeUpdateBody and flushWrites drive it.
	withdrawal := append([]byte{0, byte(len(prefix))}, prefix...)
	withdrawal = append(withdrawal, 0, 0)
	session := f.destination.session
	session.writeMu.Lock()
	session.noteAIGPWrite(withdrawal, false)
	session.commitAIGPWrites(true)
	session.writeMu.Unlock()
	seen := len(bodies)
	f.metric(7)
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	bodies = aigpSocketBodies(t, f.conn)
	if len(bodies) != seen+1 {
		t.Fatalf("recovery UPDATE count=%d, want %d; the recovery withdrawal dropped the cost-withheld record", len(bodies), seen+1)
	}
	last := bodies[len(bodies)-1]
	sections, err := wire.ParseUpdateSections(last)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sections.NLRI(last), prefix) {
		t.Fatalf("recovery did not announce 10.20.0.0/24: %x", last)
	}
	if metric, present := aigpReceivedMetric(t, last); !present || metric != 107 {
		t.Fatalf("recovered metric=%d present=%v, want 107", metric, present)
	}
}

// addSource registers a second source peer on the fixture's reactor, set up as
// the fixture sets up its own source: AIGP enabled, next-hop-self, the shared
// encoding context.
func (f *aigpReplayFixture) addSource(t *testing.T, address string) *Peer {
	t.Helper()
	peer, _ := newAnnouncePeer(t, address)
	peer.settings.AIGPSession = new(true)
	peer.settings.GlobalLocalAS = 65000
	peer.settings.LocalAddress = netip.MustParseAddr("10.0.0.254")
	peer.settings.NextHopMode = NextHopSelf
	peer.settings.ProcessBindings = []ProcessBinding{{PluginName: "aigp-forwarder", SendAll: true}}
	negotiateForwardTestFamilies(peer, family.IPv4Unicast)
	peer.sendCtx.Store(bgpctx.Registry.Get(f.ctxID))
	peer.sendCtxID, peer.recvCtxID = f.ctxID, f.ctxID
	peer.session.sendCtxID = f.ctxID
	peer.refreshForwardFacts()
	f.r.mu.Lock()
	f.r.peers[peer.Settings().PeerKey()] = peer
	f.r.mu.Unlock()
	return peer
}

// receiveFrom delivers an UPDATE of 10.20.0.0/24 with the given received AIGP
// from source, its NEXT_HOP set to that source's address.
func (f *aigpReplayFixture) receiveFrom(t *testing.T, source *Peer, metric uint64) uint64 {
	t.Helper()
	body := aigpTestBody(metric)
	sections, err := wire.ParseUpdateSections(body)
	if err != nil {
		t.Fatal(err)
	}
	_, _, nh, found := attribute.AttrFind(sections.Attrs(body), attribute.AttrNextHop)
	if !found {
		t.Fatal("test body has no NEXT_HOP")
	}
	copy(nh, source.Settings().Address.AsSlice())
	wu := wireu.NewWireUpdate(body, f.ctxID)
	wu.SetSourceID(source.SourceID())
	if !f.r.notifyMessageReceiver(source.Settings().Address, msgtype.TypeUPDATE, body, wu, f.ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0) {
		t.Fatal("UPDATE not delivered")
	}
	f.ids = append(f.ids, wu.MessageID())
	return wu.MessageID()
}

// lastAnnounces fails unless the last socket UPDATE announces prefix with the
// given AIGP metric and withdraws nothing.
func lastAnnounces(t *testing.T, bodies [][]byte, prefix []byte, metric uint64) {
	t.Helper()
	last := bodies[len(bodies)-1]
	sections, err := wire.ParseUpdateSections(last)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sections.NLRI(last), prefix) || len(sections.Withdrawn(last)) != 0 {
		t.Fatalf("last UPDATE does not announce %x alone: %x", prefix, last)
	}
	if got, present := aigpReceivedMetric(t, last); !present || got != metric {
		t.Fatalf("announced metric=%d present=%v, want %d", got, present, metric)
	}
}

// TestAIGPWithheldWithdrawalKeepsAnotherSourcesRecord pins one recipient key
// shared by two sources without ADD-PATH. The recipient holds source B's
// route with a tracked AIGP record when source A's route of the same prefix
// is withheld for an unknown cost. A's withdrawal is rightly dropped, since
// the recipient holds B's route, and its cost-withheld record once replaced
// B's: B's later distance change then never re-advertised, and the recipient
// kept a stale AIGP. RFC 7311 Section 3.4.3: "R1 MUST increase the value of
// the AIGP TLV by adding to A the distance from R1 to R2." Method: advertise
// B at distance 7, forward A's route with no distance for A, assert a silent
// socket, install distance 11 for B, and assert one re-advertisement of B
// with AIGP 111.
func TestAIGPWithheldWithdrawalKeepsAnotherSourcesRecord(t *testing.T) {
	prefix := []byte{24, 10, 20, 0}
	f := newAIGPReplayFixture(t, nil)
	other := f.addSource(t, "198.18.233.3")
	f.metric(7)
	f.forward(t, f.receive(t, f.body(t, 100)))
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	if len(bodies) != 1 {
		t.Fatalf("initial UPDATE count=%d, want 1", len(bodies))
	}
	lastAnnounces(t, bodies, prefix, 107)
	f.forward(t, f.receiveFrom(t, other, 50))
	forwardSocketBarrier(t, f.r)
	if bodies = aigpSocketBodies(t, f.conn); len(bodies) != 1 {
		t.Fatalf("UPDATE count=%d after the other source's withheld route, want 1: %x", len(bodies), bodies)
	}
	f.metric(11)
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	bodies = aigpSocketBodies(t, f.conn)
	if len(bodies) != 2 {
		t.Fatalf("UPDATE count=%d, want B's re-advertisement; A's withheld withdrawal replaced B's record", len(bodies))
	}
	lastAnnounces(t, bodies, prefix, 111)
}

// TestAIGPReplayEmittingNothingIsQueuedAgain pins the release of a replay
// whose write emits nothing. The dedupe of an outcome already queued once
// stayed set when the write emitted nothing, so every later pass with the
// same outcome stopped at it and the route was never recovered. Method:
// withhold the route from its first receipt, make the final export policy
// reject so the replay becomes a withdrawal of a key the recipient was never
// advertised (dropped), install distance 11 and replay, assert a silent
// socket, accept again, replay at the same outcome, and assert one
// announcement with AIGP 111.
func TestAIGPReplayEmittingNothingIsQueuedAgain(t *testing.T) {
	prefix := []byte{24, 10, 20, 0}
	f := newAIGPReplayFixture(t, nil)
	f.forward(t, f.receive(t, f.body(t, 100)))
	forwardSocketBarrier(t, f.r)
	if bodies := aigpSocketBodies(t, f.conn); len(bodies) != 0 {
		t.Fatalf("withheld route reached the socket: %x", bodies)
	}
	session := f.destination.session
	session.writeMu.Lock()
	session.egressFinalAccepts = func() bool { return false }
	session.writeMu.Unlock()
	f.metric(11)
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	if bodies := aigpSocketBodies(t, f.conn); len(bodies) != 0 {
		t.Fatalf("rejected replay reached the socket: %x", bodies)
	}
	session.writeMu.Lock()
	session.egressFinalAccepts = nil
	session.writeMu.Unlock()
	f.r.readvertiseAIGP()
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	if len(bodies) != 1 {
		t.Fatalf("UPDATE count=%d, want the recovery; the replay that emitted nothing stayed queued", len(bodies))
	}
	lastAnnounces(t, bodies, prefix, 111)
}

// TestAIGPRecoveryWithdrawalRemovesDecidedRecords pins the other side of
// TestAIGPRecoveryWithdrawalKeepsCostWithheldRecord: the skip for a
// withdrawal with no AIGP origin covers only a live cost-withheld record. An
// announcement record goes, because the recipient no longer holds the route,
// and so does a cost-withheld record whose received route is gone. Method:
// drive the receipt of a bare withdrawal with no origin over each record, as
// writeUpdateBody and flushWrites drive it, and assert the record is removed.
func TestAIGPRecoveryWithdrawalRemovesDecidedRecords(t *testing.T) {
	prefix := []byte{24, 10, 20, 0}
	withdrawal := append([]byte{0, byte(len(prefix))}, prefix...)
	withdrawal = append(withdrawal, 0, 0)
	commit := func(f *aigpReplayFixture) int {
		session := f.destination.session
		session.writeMu.Lock()
		session.noteAIGPWrite(withdrawal, false)
		session.commitAIGPWrites(true)
		session.writeMu.Unlock()
		f.r.aigp.mu.Lock()
		defer f.r.aigp.mu.Unlock()
		return len(f.r.aigp.advertised)
	}
	t.Run("announcement", func(t *testing.T) {
		f := newAIGPReplayFixture(t, nil)
		f.metric(7)
		f.forward(t, f.receive(t, f.body(t, 100)))
		forwardSocketBarrier(t, f.r)
		if n := commit(f); n != 0 {
			t.Fatalf("records=%d after a withdrawal of an advertised route, want 0", n)
		}
	})
	t.Run("route-gone", func(t *testing.T) {
		f := newAIGPReplayFixture(t, nil)
		body := f.body(t, 100)
		f.forward(t, f.receive(t, body))
		forwardSocketBarrier(t, f.r)
		f.r.aigp.mu.Lock()
		withheld := len(f.r.aigp.advertised)
		f.r.aigp.mu.Unlock()
		if withheld != 1 {
			t.Fatalf("records=%d after the withheld route, want its cost-withheld record", withheld)
		}
		f.r.invalidateAIGPReceived(f.source.Settings().Address, wireu.NewWireUpdate(body, f.ctxID))
		if n := commit(f); n != 0 {
			t.Fatalf("records=%d after a withdrawal over a record whose route is gone, want 0", n)
		}
	})
}
