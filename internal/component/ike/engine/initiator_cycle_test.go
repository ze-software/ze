package engine

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// icyFarEnd holds the far-end SA of a driven handshake. The stub that advances the
// handshake runs on the session goroutine. The test goroutine reads the SA once the
// handshake is over, so the pointer is guarded.
type icyFarEnd struct {
	mu  sync.Mutex
	sa  *SA
	err error
}

func (f *icyFarEnd) get() (*SA, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sa, f.err
}

// icyParse parses a message and returns an error rather than a test failure. Its
// caller runs on the session goroutine, where a test failure call is not allowed.
func icyParse(raw []byte) (*wire.Message, error) {
	if len(raw) == 0 {
		return nil, errors.New("the peer has nothing to answer")
	}
	var m wire.Message
	if err := m.ReadFrom(raw); err != nil {
		return nil, err
	}
	return &m, nil
}

// icyAdvance drives the far end of the handshake one step. It answers the message
// the initiator last wrote, and feeds that answer back to the initiator. It runs on
// the session goroutine, so it writes SA fields from the goroutine that owns them.
func (f *icyFarEnd) advance(ini *SA, respPS *PeerSession, table *SATable, log *slog.Logger) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return
	}
	step := func() error {
		switch ini.State {
		case StateSAInitSent:
			if f.sa == nil {
				sa, err := newResponderSA("ze", respPS.peerCfg, respPS.ikeGroup, respPS.espGroup, ini.InitiatorSPI)
				if err != nil {
					return err
				}
				f.sa = sa
			}
			req, err := icyParse(ini.LastSentMsg)
			if err != nil {
				return err
			}
			handleSAInitRequest(f.sa, req, ini.LastSentMsg, nil, nil, log)
			if f.sa.State != StateSAInitReceived {
				return errors.New("the far end refused the IKE_SA_INIT request")
			}
			answer, err := icyParse(f.sa.LastSentMsg)
			if err != nil {
				return err
			}
			handleSAInitResponse(ini, answer, f.sa.LastSentMsg, table, nil, nil, log)
		case StateAuthSent:
			req, err := icyParse(ini.LastSentMsg)
			if err != nil {
				return err
			}
			respPS.handleAuthRequest(f.sa, req, ini.LastSentMsg, nil, nil, log)
			if f.sa.State != StateEstablished {
				return errors.New("the far end refused the IKE_AUTH request")
			}
			answer, err := icyParse(f.sa.LastSentMsg)
			if err != nil {
				return err
			}
			handleAuthResponse(ini, answer, f.sa.LastSentMsg, table, nil, log)
		case StateIdle, StateSAInitReceived, StateAuthReceived, StateEAPInProgress, StateEstablished, StateDead:
		}
		return nil
	}
	f.err = step()
}

// icyWaitFor polls until want returns true, or fails the test. It waits on the
// condition rather than on elapsed time, so a loaded host only takes longer.
func icyWaitFor(t *testing.T, what string, want func() bool) {
	t.Helper()
	deadline := time.Now().Add(rtxArrive)
	for !want() {
		if time.Now().After(deadline) {
			t.Fatalf("%s never happened", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// icyPeerIKERekey builds the CREATE_CHILD_SA request a peer sends to rekey the IKE
// SA. It carries a proposal set with the peer's new IKE SPI, a nonce, and a key
// exchange payload. RFC 7296 Section 1.3.3.
func icyPeerIKERekey(t *testing.T, farEnd *SA, ikeGroup ipsec.IKEGroup, msgID uint32) []byte {
	t.Helper()
	newSPI, err := GenerateSPI()
	if err != nil {
		t.Fatalf("GenerateSPI: %v", err)
	}
	ni, err := GenerateNonce(nonceLen)
	if err != nil {
		t.Fatalf("GenerateNonce: %v", err)
	}
	dhGroup := crypto.DHGroupID(ikeGroup.Proposals[0].DHGroup)
	dh, err := crypto.NewDHExchange(dhGroup)
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}
	t.Cleanup(dh.Clear)

	props := buildWireIKEProposals(ikeGroup)
	spiBytes := make([]byte, 8)
	copy(spiBytes, newSPI[:])
	for i := range props {
		props[i].SPISize = 8
		props[i].SPI = spiBytes
	}
	inner := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: props}},
		{Payload: &wire.PayloadNonce{NonceData: ni}},
		{Payload: &wire.PayloadKE{DHGroup: uint16(dhGroup), KeyExchangeData: dh.PublicKey}},
	}
	raw, err := buildEncryptedMessageEx(farEnd, inner, msgID, wire.ExchangeCreateChildSA, initiatorFlag(farEnd))
	if err != nil {
		t.Fatalf("build the peer IKE rekey request: %v", err)
	}
	return raw
}

// icyRunCycle runs one complete initiator lifecycle against a far end driven in
// process. The cycle covers IKE_SA_INIT, IKE_AUTH, the established owner loop, and
// the stop.
//
// When rekey is true the far end rekeys the IKE SA while the tunnel is up. That
// replaces the SA the session holds with one under new SPIs.
//
// It returns the SATable the cycle ran against, and the error the cycle ended with.
func icyRunCycle(t *testing.T, rekey bool) (*SATable, error) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "cycle-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	ps := &PeerSession{
		peerName:  "ze",
		peerCfg:   iniPeer,
		ikeGroup:  ikeGroup,
		espGroup:  espGroup,
		stopCh:    make(chan struct{}),
		inbound:   make(chan transport.Packet, inboundQueueDepth),
		supersede: make(chan struct{}, 1),
	}
	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	table := NewSATable()
	far := &icyFarEnd{}

	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	afterFunc = func(_ time.Duration) <-chan time.Time {
		if cur := ps.getSA(); cur != nil {
			far.advance(cur, respPS, table, log)
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	var cycleErr error
	done := make(chan struct{})
	go func() {
		cycleErr = ps.runInitiator(iniPeer, ikeGroup, table, nil, nil, log)
		close(done)
	}()

	// The owner loop holding an SA is the signal that the handshake finished, and
	// ownedSA is atomic, so reading it here races with nothing.
	icyWaitFor(t, "the owner loop adopting the established SA", func() bool {
		return ps.ownedSA.Load() != nil
	})
	first := ps.ownedSA.Load()

	if rekey {
		farEnd, err := far.get()
		if err != nil {
			t.Fatalf("the driven handshake failed: %v", err)
		}
		// An established initiator SA has never cached a response, so the peer's
		// first request rides Message ID zero and its Delete rides one.
		ps.inbound <- transport.Packet{Data: icyPeerIKERekey(t, farEnd, ikeGroup, 0)}
		ps.inbound <- transport.Packet{Data: lcyRequest(t, farEnd, 1, rteIKEDeleteChain())}
		icyWaitFor(t, "the owner loop swapping to the rekeyed SA", func() bool {
			return ps.ownedSA.Load() != first
		})
	}

	close(ps.stopCh)
	<-done
	if _, err := far.get(); err != nil {
		t.Fatalf("the driven handshake failed: %v", err)
	}
	return table, cycleErr
}

// VALIDATES: an initiator cycle that established and then ended leaves no SA in the
// SATable, including a cycle whose SA was replaced by an IKE rekey.
// PREVENTS: the leak the deferred removal left behind. Go evaluates the arguments of
// a deferred CALL where the defer is written. The responder SPI was captured as
// zero, and the removal at return deleted a key that no longer existed. An IKE rekey
// replaces the SA with new SPIs, so a fix that only reads the fields later still
// deletes the wrong key.
func TestIcyEstablishedCycleLeavesNoTableEntry(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rekey bool
	}{
		{"a plain established cycle", false},
		{"a cycle whose IKE SA was rekeyed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table, err := icyRunCycle(t, tc.rekey)
			if err != nil {
				t.Fatalf("the initiator cycle ended with %v, want a clean stop", err)
			}
			if n := table.Len(); n != 0 {
				t.Errorf("the ended cycle left %d SATable entries, want 0", n)
			}
		})
	}
}

// VALIDATES: an IKE rekey leaves the session pointing at the SA that now carries the
// tunnel, so the table holds that SA and nothing else.
// PREVENTS: a stale ps.sa after a rekey. TerminatePeerSA and TerminateAllSAs remove
// the SA that ps.getSA returns, so a stale pointer makes an operator clear delete a
// key that is gone and leave the live one behind.
func TestIcyRekeyRepointsTheSessionAtTheNewSA(t *testing.T) {
	ikeGroup := testIKEGroup()
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "cycle-psk")
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"

	ps := &PeerSession{
		peerName:  "ze",
		peerCfg:   iniPeer,
		ikeGroup:  ikeGroup,
		espGroup:  testESPGroup(),
		stopCh:    make(chan struct{}),
		inbound:   make(chan transport.Packet, inboundQueueDepth),
		supersede: make(chan struct{}, 1),
	}
	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: testESPGroup()}
	table := NewSATable()
	far := &icyFarEnd{}

	old := afterFunc
	t.Cleanup(func() { afterFunc = old })
	afterFunc = func(_ time.Duration) <-chan time.Time {
		if cur := ps.getSA(); cur != nil {
			far.advance(cur, respPS, table, log)
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	done := make(chan struct{})
	go func() {
		_ = ps.runInitiator(iniPeer, ikeGroup, table, nil, nil, log)
		close(done)
	}()
	icyWaitFor(t, "the owner loop adopting the established SA", func() bool {
		return ps.ownedSA.Load() != nil
	})
	first := ps.ownedSA.Load()

	farEnd, err := far.get()
	if err != nil {
		t.Fatalf("the driven handshake failed: %v", err)
	}
	ps.inbound <- transport.Packet{Data: icyPeerIKERekey(t, farEnd, ikeGroup, 0)}
	ps.inbound <- transport.Packet{Data: lcyRequest(t, farEnd, 1, rteIKEDeleteChain())}
	icyWaitFor(t, "the owner loop swapping to the rekeyed SA", func() bool {
		return ps.ownedSA.Load() != first
	})
	swapped := ps.ownedSA.Load()

	// The session names the SA the owner loop now holds, and the table holds it too.
	icyWaitFor(t, "the session pointing at the rekeyed SA", func() bool {
		return ps.getSA() == swapped
	})
	if n := table.Len(); n != 1 {
		t.Errorf("the table holds %d entries after the rekey, want 1", n)
	}
	if table.Lookup(swapped.InitiatorSPI, swapped.ResponderSPI) != swapped {
		t.Error("the table does not hold the rekeyed SA under its own SPI pair")
	}
	if swapped.InitiatorSPI == first.InitiatorSPI && swapped.ResponderSPI == first.ResponderSPI {
		t.Fatal("the rekey reused the SPI pair, so the test proves nothing")
	}

	close(ps.stopCh)
	<-done
}
