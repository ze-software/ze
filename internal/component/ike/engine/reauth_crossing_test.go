package engine

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Tests for spec-home-cpe-22 AC-13 ("Simultaneous replacements") and AC-7's IKE SA
// rekey half. The gateway's reauthentication can cross a replacement Ze started for the
// same session: Ze's own IKE SA rekey, or Ze's reconnect. RFC 7296 Section 2.8.3 says
// "IKEv2 does not have any special support for reauthentication.", so the spec settles
// the winner from strongSwan's behavior: the successor the peer started wins whatever
// order the two complete in, and Ze deletes the replacement it started.

// crossingDP is a recording dataplane safe for the concurrent use these tests make of
// it: the owner loop removes Child SAs while the dispatch path, played by the test
// goroutine, installs the successor's.
type crossingDP struct {
	mu        sync.Mutex
	installed map[uint32]bool
	removed   map[uint32]bool
	// policies is the policy table as the kernel would hold it: an install upserts the
	// selector, a removal deletes it. A removal is judged by when it ran: one before
	// the successor's install (the old owner loop's exit, while the successor is still
	// half-open) is undone by that install, and one after it leaves the successor's
	// Child SA with no policy.
	policies map[policyKey]bool
}

// policyKey is one kernel policy: its selector, as source>destination, and direction.
type policyKey struct {
	selector string
	dir      dataplane.SADir
}

func selectorKey(source, target *net.IPNet) string {
	return source.String() + ">" + target.String()
}

func (d *crossingDP) RequiresForwardPolicy() bool { return false }

func (d *crossingDP) InstallSA(p dataplane.SAParams) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.installed[p.SPI] = true
	return nil
}

func (d *crossingDP) RemoveSA(spi uint32, _ net.IP, _ uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.removed[spi] = true
	return nil
}

func (d *crossingDP) InstallPolicy(p dataplane.SPParams) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.policies[policyKey{selectorKey(p.Src, p.Dst), p.Dir}] = true
	return nil
}

func (d *crossingDP) RemovePolicy(source, target *net.IPNet, dir dataplane.SADir) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.policies, policyKey{selectorKey(source, target), dir})
	return nil
}

func (d *crossingDP) RemovePolicyParams(p dataplane.SPParams) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.policies, policyKey{selectorKey(p.Src, p.Dst), p.Dir})
	return nil
}

func (d *crossingDP) ListSAs(_ uint32) ([]dataplane.SAInfo, error)  { return nil, nil }
func (d *crossingDP) ListPolicies() ([]dataplane.PolicyInfo, error) { return nil, nil }
func (d *crossingDP) Close() error                                  { return nil }

// live answers the SPIs installed since the backend loaded and not removed since.
func (d *crossingDP) live() map[uint32]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[uint32]bool)
	for spi := range d.installed {
		if !d.removed[spi] {
			out[spi] = true
		}
	}
	return out
}

func (d *crossingDP) wasRemoved(spi uint32) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.removed[spi]
}

// policyMissing reports whether a policy child installed, in either direction, is not in
// the policy table now: never installed through this dataplane, or removed since.
func (d *crossingDP) policyMissing(child *ChildSA) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, dir := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn} {
		policies := childPolicies(child, dir)
		for i := range policies {
			if !d.policies[policyKey{selectorKey(policies[i].Src, policies[i].Dst), dir}] {
				return true
			}
		}
	}
	return false
}

var crossingBackend struct {
	sync.Once
	err    error
	active *crossingDP
}

// useCrossingDataplane makes a fresh crossingDP the active dataplane for one test.
func useCrossingDataplane(t *testing.T) *crossingDP {
	t.Helper()
	const name = "ike-engine-test-reauth-crossing"
	crossingBackend.Do(func() {
		crossingBackend.err = dataplane.Register(name, func() (dataplane.Dataplane, error) { return crossingBackend.active, nil })
	})
	if crossingBackend.err != nil {
		t.Fatal(crossingBackend.err)
	}
	dp := &crossingDP{installed: make(map[uint32]bool), removed: make(map[uint32]bool), policies: make(map[policyKey]bool)}
	crossingBackend.active = dp
	if err := dataplane.Load(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Error(err)
		}
		crossingBackend.active = nil
	})
	return dp
}

// loopbackOf answers the address the peer stand-in of rtxPeerLink listens on.
func loopbackOf(t *testing.T, peerTr *transport.UDPTransport) *net.UDPAddr {
	t.Helper()
	addr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	return addr
}

// pointAtPeer makes Ze's sa send to the loopback peer stand-in from Ze's socket. The
// configured remote is rewritten too, because an IKE SA rekey copies PeerCfg into the
// replacement.
func pointAtPeer(t *testing.T, sa *SA, peerTr *transport.UDPTransport) {
	t.Helper()
	sa.PeerCfg.RemoteAddress = "127.0.0.1"
	sa.peerEndpoint = loopbackOf(t, peerTr)
}

// expectIKEDelete reads Ze's next datagram at the peer's socket and checks it is a
// Delete of the IKE SA whose peer end is peer, protected under that SA's keys.
func expectIKEDelete(t *testing.T, peerTr *transport.UDPTransport, peer *SA, what string) {
	t.Helper()
	data := rtxRecv(t, peerTr)
	if data == nil {
		t.Fatalf("Ze sent no Delete for %s", what)
	}
	msg := parseMsg(t, data)
	if msg.Header.InitiatorSPI != peer.InitiatorSPI || msg.Header.ResponderSPI != peer.ResponderSPI {
		t.Fatalf("Ze's next message names IKE SA %x/%x, want the Delete of %s (%x/%x)",
			msg.Header.InitiatorSPI, msg.Header.ResponderSPI, what, peer.InitiatorSPI, peer.ResponderSPI)
	}
	inner, err := decryptAndParse(peer, msg, data)
	if err != nil {
		t.Fatalf("the Delete of %s does not decrypt under its keys: %v", what, err)
	}
	for i := range inner {
		if del, ok := inner[i].Payload.(*wire.PayloadDelete); ok && del.ProtocolID == wire.ProtocolIKE {
			return
		}
	}
	t.Fatalf("Ze's message on %s carries no Delete of the IKE SA", what)
}

// expectNothingMore proves Ze sent nothing after what the test already read: the
// sentinel, sent on Ze's own socket, arrives next.
func expectNothingMore(t *testing.T, peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	if err := myTr.Send(rtxSentinel, loopbackOf(t, peerTr)); err != nil {
		t.Fatalf("send sentinel: %v", err)
	}
	if data := rtxRecv(t, peerTr); !bytes.Equal(data, rtxSentinel) {
		t.Fatal("Ze sent another message, a second Delete or one for the peer-started successor")
	}
}

// waitFor waits, bounded, for a state another goroutine reaches.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(rtxArrive)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitOwnerLoop waits for the owner loop to return, and wants nil: it relinquished.
func waitOwnerLoop(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the owner loop returned %v, want nil (relinquished to the successor)", err)
		}
	case <-time.After(2 * rtxArrive):
		t.Fatal("the owner loop did not relinquish to the peer-started successor")
	}
}

// readZeIKERekey reads Ze's IKE SA rekey request as the gateway, on gwOld.
func readZeIKERekey(t *testing.T, peerTr *transport.UDPTransport, gwOld *SA) ([]wire.PayloadEntry, uint32) {
	t.Helper()
	data := rtxRecv(t, peerTr)
	if data == nil {
		t.Fatal("Ze did not start its IKE SA rekey")
	}
	msg := parseMsg(t, data)
	if msg.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Fatalf("Ze's first message has exchange type %d, want CREATE_CHILD_SA", msg.Header.ExchangeType)
	}
	inner, err := decryptAndParse(gwOld, msg, data)
	if err != nil {
		t.Fatalf("the gateway cannot decrypt Ze's rekey: %v", err)
	}
	return inner, msg.Header.MessageID
}

// answerZeIKERekey answers Ze's IKE SA rekey as the gateway and hands the answer to the
// owner loop, returning the gateway's end of the replacement Ze started.
func answerZeIKERekey(t *testing.T, ps *PeerSession, gwOld *SA, inner []wire.PayloadEntry, msgID uint32, from *net.UDPAddr) *SA {
	t.Helper()
	answer, gwReplacement, err := respondIKERekey(gwOld, inner, msgID, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("the gateway refused Ze's IKE SA rekey: %v", err)
	}
	ps.inbound <- transport.Packet{Data: answer, RemoteAddr: from}
	return gwReplacement
}

// assertPeerStartedKept checks the converged state AC-13 names: the session runs on the
// successor the peer started with its Child SA, the SATable holds that IKE SA alone, and
// the dataplane holds that Child SA pair alone, its policies installed: the shared ones
// kept by every removal that ran after the successor's install.
func assertPeerStartedKept(t *testing.T, ps *PeerSession, table *SATable, dp *crossingDP, successor *SA, successorChild *ChildSA) {
	t.Helper()
	if got := ps.adoptPeerStartedSuccessor(table, dataplane.Get(), nil, slogutil.DiscardLogger()); got != successor {
		t.Fatal("the session did not adopt the successor the peer started")
	}
	if ps.getChildSA() != successorChild {
		t.Error("the successor's Child SA is not the session's Child SA")
	}
	if table.Len() != 1 || table.Lookup(successor.InitiatorSPI, successor.ResponderSPI) != successor {
		t.Errorf("the SATable holds %d IKE SAs, want the peer-started successor alone", table.Len())
	}
	live := dp.live()
	if len(live) != 2 || !live[successorChild.InboundSPI] || !live[successorChild.OutboundSPI] {
		t.Errorf("the dataplane holds Child SA states %v, want the successor's pair %d/%d alone",
			live, successorChild.InboundSPI, successorChild.OutboundSPI)
	}
	if dp.policyMissing(successorChild) {
		t.Error("a policy of the successor's Child SA is not installed (removed after its install, or never installed), so its traffic would leave in the clear")
	}
}

// startOwnerLoopWithIKERekeyDue runs the owner loop of sa with its IKE SA soft lifetime
// already passed, so the loop's first tick starts Ze's own IKE SA rekey.
func startOwnerLoopWithIKERekeyDue(ps *PeerSession, sa *SA, table *SATable, myTr *transport.UDPTransport) <-chan error {
	due := &lifetimeState{softTime: time.Now().Add(-time.Second), hardTime: time.Now().Add(time.Hour)}
	done := make(chan error, 1)
	go func() {
		done <- ps.maintainSA(sa, nil, newLifetimeState(3600), due, testIKEGroup(), table,
			dataplane.Get(), myTr, nil, slogutil.DiscardLogger())
	}()
	return done
}

// VALIDATES: AC-13 with Ze's own IKE SA rekey, in both completion orders, under -race.
// The gateway's reauthentication is admitted, then Ze's owner loop starts its IKE SA
// rekey. Whichever completes first, the session ends on the successor the gateway
// started: Ze deletes the replacement its rekey created (after the rekey's own Delete of
// the old SA, RFC 7296 Section 2.8), sends nothing for the successor, and the old Child
// SA's states leave the dataplane while the successor's pair and the shared policies
// stay. Red before the fix: the supersede arm relinquished without a Delete, so the
// gateway kept the replacement; and with the rekey unanswered it relinquished at once,
// so the answer was never read and no Delete of either SA was sent.
func TestSimultaneousReauthAndIKERekeyKeepsPeerStarted(t *testing.T) {
	for _, order := range []string{"rekey-completes-first", "reauth-completes-first"} {
		t.Run(order, func(t *testing.T) {
			ps, old, gwOld, table, gwPeer := ownedInitiatorAndGateway(t)
			oldChild := ps.getChildSA()
			dp := useCrossingDataplane(t)
			peerTr, myTr := rtxPeerLink(t, old)
			pointAtPeer(t, old, peerTr)
			gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
			successor := admitGatewayReauth(t, ps, gw, pkt, table)

			done := startOwnerLoopWithIKERekeyDue(ps, old, table, myTr)
			inner, msgID := readZeIKERekey(t, peerTr, gwOld)
			var gwReplacement *SA
			if order == "rekey-completes-first" {
				gwReplacement = answerZeIKERekey(t, ps, gwOld, inner, msgID, loopbackOf(t, peerTr))
				expectIKEDelete(t, peerTr, gwOld, "the old IKE SA, the rekey's own Delete")
				runGatewayAuth(t, gw, successor, table)
			} else {
				runGatewayAuth(t, gw, successor, table)
				// The owner loop takes the supersede signal while its rekey is unanswered.
				waitFor(t, "the owner loop to take the supersede signal", func() bool { return len(ps.supersede) == 0 })
				gwReplacement = answerZeIKERekey(t, ps, gwOld, inner, msgID, loopbackOf(t, peerTr))
				expectIKEDelete(t, peerTr, gwOld, "the old IKE SA, the rekey's own Delete")
			}
			successorChild := ps.getPendingChild()
			if successorChild == nil {
				t.Fatal("setup: the successor's Child SA was not staged")
			}
			expectIKEDelete(t, peerTr, gwReplacement, "the replacement Ze's rekey created")
			waitOwnerLoop(t, done)
			expectNothingMore(t, peerTr, myTr)
			if table.Lookup(gwReplacement.InitiatorSPI, gwReplacement.ResponderSPI) != nil {
				t.Error("the replacement Ze's rekey created is still in the SATable")
			}
			if !dp.wasRemoved(oldChild.InboundSPI) {
				t.Error("the old Child SA's inbound state was not removed")
			}
			assertPeerStartedKept(t, ps, table, dp, successor, successorChild)
		})
	}
}

// endOldOwnerLoop plays what the old owner loop's exit leaves behind (a DPD timeout:
// its Child SA removed, no SA owned) and the cycle end of runInitiator, and checks the
// gateway's half-open reauthentication survives both.
func endOldOwnerLoop(t *testing.T, ps *PeerSession, old, successor *SA, table *SATable) {
	t.Helper()
	ps.cleanupChild(dataplane.Get(), nil, slogutil.DiscardLogger())
	ps.ownedSA.Store(nil)
	ps.saEndpoint.Store(nil)
	if got := ps.adoptPeerStartedSuccessor(table, dataplane.Get(), nil, slogutil.DiscardLogger()); got != nil {
		t.Fatal("a half-open successor was adopted")
	}
	if ps.getPendingSA() != successor {
		t.Fatal("the cycle's end dropped the gateway's reauthentication still in flight")
	}
	if !ps.responderBusy.Load() {
		t.Error("the half-open gate was freed while the successor is still in flight")
	}
	table.removeByPeerExcept(old.PeerName, ps.getPendingSA())
	if table.Lookup(successor.InitiatorSPI, successor.ResponderSPI) != successor {
		t.Fatal("runInitiator's cycle-end removal took the in-flight successor out of the SATable")
	}
	if table.Lookup(old.InitiatorSPI, old.ResponderSPI) != nil {
		t.Error("runInitiator's cycle-end removal left the ended SA in the SATable")
	}
}

// VALIDATES: AC-13 with Ze's reconnect, in both completion orders, under -race. The
// gateway's reauthentication is admitted, then Ze's owner loop ends and Ze reconnects
// with a new IKE SA. The half-open successor is kept across the reconnect; when it
// completes, before Ze's reconnect is established or after, Ze deletes the reconnect's
// IKE SA, removes its Child SA keeping the shared policies, and adopts the successor.
// Red before the fix: the cycle's end retired the half-open successor; a successor
// completing with no owner loop took the session's primary Child SA slot and signaled
// nothing; the reconnect's owner loop drained the supersede signal; and the supersede
// arm sent no Delete for the reconnect's SA.
func TestReconnectCrossingPeerReauthKeepsPeerStarted(t *testing.T) {
	for _, order := range []string{"reconnect-completes-first", "reauth-completes-first"} {
		t.Run(order, func(t *testing.T) {
			ps, old, _, table, gwPeer := ownedInitiatorAndGateway(t)
			reconnect, gwReconnect, _ := establishPSK(t)
			dp := useCrossingDataplane(t)
			gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
			successor := admitGatewayReauth(t, ps, gw, pkt, table)
			endOldOwnerLoop(t, ps, old, successor, table)

			peerTr, myTr := rtxPeerLink(t, reconnect)
			pointAtPeer(t, reconnect, peerTr)
			if order == "reauth-completes-first" {
				runGatewayAuth(t, gw, successor, table)
				if ps.getPendingChild() == nil || len(ps.supersede) != 1 {
					t.Fatal("a successor completing with no owner loop was not staged and signaled")
				}
				if ps.getChildSA() != nil {
					t.Fatal("the successor's Child SA took the session's primary slot while no owner loop ran")
				}
			}
			// runInitiator: the reconnect is established and recorded against the
			// successor in flight, then runs its owner loop.
			table.Insert(reconnect)
			ps.setSA(reconnect)
			ps.recordCrossing()
			done := make(chan error, 1)
			go func() {
				done <- ps.runEstablished(reconnect, ps.peerCfg, testIKEGroup(), table, myTr, nil, slogutil.DiscardLogger())
			}()
			if order == "reconnect-completes-first" {
				waitFor(t, "the reconnect's owner loop", func() bool { return ps.ownedSA.Load() == reconnect })
				runGatewayAuth(t, gw, successor, table)
			}
			successorChild := ps.getPendingChild()
			if successorChild == nil {
				t.Fatal("setup: the successor's Child SA was not staged")
			}
			expectIKEDelete(t, peerTr, gwReconnect, "the IKE SA Ze's reconnect created")
			waitOwnerLoop(t, done)
			expectNothingMore(t, peerTr, myTr)
			assertPeerStartedKept(t, ps, table, dp, successor, successorChild)
		})
	}
}

// adoptedSuccessor runs a gateway reauthentication to adoption and returns the adopted
// IKE SA (Ze its responder) and the gateway's end of it.
func adoptedSuccessor(t *testing.T) (ps *PeerSession, successor, gw *SA) {
	t.Helper()
	ps, old, table, gwPeer := ownedInitiator(t)
	gw, pkt := gatewayReauthInit(t, gwPeer, gatewayAddr)
	pending := admitGatewayReauth(t, ps, gw, pkt, table)
	runGatewayAuth(t, gw, pending, table)
	gatewayReadsAuth(t, gw, pending)
	relinquishOwner(t, ps, old, table)
	successor = ps.adoptPeerStartedSuccessor(table, dataplane.Get(), nil, slogutil.DiscardLogger())
	if successor != pending {
		t.Fatal("setup: the successor was not adopted")
	}
	return ps, successor, gw
}

// VALIDATES: AC-7, the IKE SA half: an IKE SA adopted from the gateway's
// reauthentication, Ze its responder, is rekeyed by either side. Ze's own rekey request
// leaves the Initiator flag clear and the gateway can read it; Ze completes the rekey and
// is the initiator of the replacement (RFC 7296 Section 2.18). The gateway's rekey is
// answered under the adopted SA's keys, and the gateway completes it. Red if any role
// read the peer's connection type instead of the SA's role.
func TestAdoptedResponderRoleSAIKERekeys(t *testing.T) {
	log := slogutil.DiscardLogger()
	t.Run("ze-rekeys", func(t *testing.T) {
		_, successor, gw := adoptedSuccessor(t)
		req, pending, err := initiateIKERekey(successor, testIKEGroup())
		if err != nil {
			t.Fatalf("Ze could not build an IKE SA rekey on the adopted SA: %v", err)
		}
		msg := parseMsg(t, req)
		if msg.Header.Flags&wire.FlagInitiator != 0 {
			t.Error("Ze set the Initiator flag on an IKE SA the gateway created (RFC 7296 Section 3.1)")
		}
		inner, err := decryptAndParse(gw, msg, req)
		if err != nil {
			t.Fatalf("the gateway cannot decrypt Ze's IKE SA rekey: %v", err)
		}
		answer, gwNew, err := respondIKERekey(gw, inner, msg.Header.MessageID, log)
		if err != nil {
			t.Fatalf("the gateway refused Ze's IKE SA rekey: %v", err)
		}
		ansInner, err := decryptAndParse(successor, parseMsg(t, answer), answer)
		if err != nil {
			t.Fatalf("Ze cannot decrypt the gateway's answer: %v", err)
		}
		zeNew, err := applyIKERekeyResponse(successor, pending, ansInner, log)
		if err != nil {
			t.Fatalf("Ze did not complete its IKE SA rekey on the adopted SA: %v", err)
		}
		if !zeNew.IsInitiator {
			t.Error("Ze rekeyed the IKE SA but is not the initiator of the replacement (RFC 7296 Section 2.18)")
		}
		probe, _, err := initiateIKERekey(zeNew, testIKEGroup())
		if err != nil {
			t.Fatalf("Ze cannot send on the replacement: %v", err)
		}
		if _, err := decryptAndParse(gwNew, parseMsg(t, probe), probe); err != nil {
			t.Errorf("the gateway cannot read Ze on the replacement, the keys follow the wrong role: %v", err)
		}
	})
	t.Run("gateway-rekeys", func(t *testing.T) {
		ps, successor, gw := adoptedSuccessor(t)
		req, gwPending, err := initiateIKERekey(gw, testIKEGroup())
		if err != nil {
			t.Fatalf("the gateway could not build its IKE SA rekey: %v", err)
		}
		msgID := parseMsg(t, req).Header.MessageID
		ps.handleOwnedInbound(successor, transport.Packet{Data: req, RemoteAddr: gatewayAddr}, nil, nil, log)
		if !successor.lastResponseSet || successor.lastResponseID != msgID {
			t.Fatal("Ze, responder of the adopted IKE SA, did not answer the gateway's IKE SA rekey")
		}
		inner, err := decryptAndParse(gw, parseMsg(t, successor.lastResponse), successor.lastResponse)
		if err != nil {
			t.Fatalf("the gateway cannot decrypt Ze's answer: %v", err)
		}
		if _, err := applyIKERekeyResponse(gw, gwPending, inner, log); err != nil {
			t.Fatalf("Ze's answer did not complete the gateway's IKE SA rekey: %v", err)
		}
	})
}
