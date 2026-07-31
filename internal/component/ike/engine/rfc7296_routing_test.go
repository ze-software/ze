package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rteIKEDeleteChain is the payload chain of an INFORMATIONAL request that deletes the
// IKE SA. RFC 7296 Section 3.11: an IKE Delete names no SPIs.
func rteIKEDeleteChain() []wire.PayloadEntry {
	return []wire.PayloadEntry{{Payload: &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}}}
}

// rteCleartext builds an unprotected datagram for the SPI pair of sa, carrying the
// payloads in the clear. This is what an off-path attacker writes. No key material of
// the IKE SA takes part, so a conforming endpoint must learn nothing from it.
func rteCleartext(t *testing.T, sa *SA, exchange uint8, msgID uint32, payloads []wire.PayloadEntry) []byte {
	t.Helper()
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			ResponderSPI: sa.ResponderSPI,
			MajorVersion: 2,
			ExchangeType: exchange,
			MessageID:    msgID,
		},
		Payloads: payloads,
	}
	buf := make([]byte, 512)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("build the cleartext datagram at id %d: %v", msgID, err)
	}
	return buf[:n]
}

// VALIDATES: an established SA the owner loop does not hold learns nothing from an
// inbound packet and answers nothing. The authenticated path still acts on the very
// same bytes, so the refusal is about protection and not about a dead code path.
// PREVENTS: the cleartext teardown. handleInformational read the OUTER payload chain,
// which no decryption ever touched, and set StateDead from a plaintext IKE Delete.
//
// RFC requirement: RFC7296-2.4-3 negative -- a cleartext INFORMATIONAL that names the right
// SPI pair and carries a plaintext IKE Delete leaves the SA established. handleInbound
// (fsm.go) drops every established-SA packet the owner loop does not hold, so no
// unauthenticated datagram reaches a verdict about the peer. The non-conforming input is
// the unauthenticated one, and it is refused.
//
// RFC requirement: RFC7296-2.4-3 positive -- the identical Delete, authenticated and delivered
// through handleOwnedInbound, does reach StateDead. A protected message still ends the
// SA. The refusal above is therefore not the trivial observation that nothing ever
// changes state.
func TestRteUnownedEstablishedSATrustsNothing(t *testing.T) {
	log := slogutil.DiscardLogger()

	// Each case runs against its own established SA, so one case cannot inherit the
	// state another case left behind.
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, ini, peer *SA) []byte
	}{
		{"cleartext IKE Delete", func(t *testing.T, ini, _ *SA) []byte {
			return rteCleartext(t, ini, wire.ExchangeInformational, ini.ExpectedMsgID, rteIKEDeleteChain())
		}},
		{"protected INFORMATIONAL request", func(t *testing.T, ini, peer *SA) []byte {
			return lcyRequest(t, peer, ini.ExpectedMsgID, rteIKEDeleteChain())
		}},
		{"cleartext CREATE_CHILD_SA", func(t *testing.T, ini, _ *SA) []byte {
			return rteCleartext(t, ini, wire.ExchangeCreateChildSA, ini.ExpectedMsgID, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
			remote := ini.remoteUDPAddr()
			if remote == nil {
				t.Fatal("the SA under test has no resolvable peer address")
			}
			// The owner loop does not hold this SA: ps.ownedSA is nil, which is the
			// state during the establish hand-off and after the loop returns.
			if ps.ownedSA.Load() != nil {
				t.Fatal("the owner loop already holds the SA under test")
			}
			table := NewSATable()
			table.Insert(ini)
			expectedBefore := ini.ExpectedMsgID
			cachedBefore, cachedIDBefore := ini.lastResponseSet, ini.lastResponseID

			handleInbound(ini, transport.Packet{Data: tc.build(t, ini, peer)}, table, myTr, log)

			if ini.State != StateEstablished {
				t.Fatalf("the SA moved to %v, want it left established", ini.State)
			}
			rtxExpectSilence(t, peerTr, myTr, remote, tc.name)
			// No owner-only state was written off the owner loop.
			if ini.ExpectedMsgID != expectedBefore {
				t.Errorf("ExpectedMsgID = %d, want %d", ini.ExpectedMsgID, expectedBefore)
			}
			if ini.lastResponseSet != cachedBefore || ini.lastResponseID != cachedIDBefore {
				t.Error("a dropped packet wrote the cached response of the owner loop")
			}
		})
	}

	// Negative. The identical protected Delete, on the owner loop, still ends the SA.
	t.Run("protected IKE Delete on the owner loop", func(t *testing.T) {
		ini, peer, ps, _, myTr := dpdProbeLink(t)
		req := lcyRequest(t, peer, ini.ExpectedMsgID, rteIKEDeleteChain())
		if out := ps.handleOwnedInbound(ini, transport.Packet{Data: req}, myTr, nil, log); !out.peerAlive {
			t.Fatal("the protected Delete never reached the INFORMATIONAL handler")
		}
		if ini.State != StateDead {
			t.Fatalf("the authenticated Delete left the SA at %v, want dead", ini.State)
		}
	})
}

// VALIDATES: an initiator lifecycle that ends removes its SA from the SATable.
// PREVENTS: the leak that kept a dead initiator SA discoverable forever. Every failed
// cycle left one established-looking entry, so inbound packets kept routing to a
// session that had gone, and ze_ipsec_tunnel_up reported a dead peer as up.
func TestRteInitiatorCycleLeavesNoTableEntry(t *testing.T) {
	log := slogutil.DiscardLogger()
	peer := testPeer()
	peer.LocalAddress = "127.0.0.1"
	peer.RemoteAddress = "127.0.0.1"
	ps := &PeerSession{
		peerName: "ze",
		peerCfg:  peer,
		ikeGroup: testIKEGroup(),
		espGroup: testESPGroup(),
		stopCh:   make(chan struct{}),
	}
	// The session is already stopping, so the handshake loop ends on its first check.
	close(ps.stopCh)

	table := NewSATable()
	err := ps.runInitiator(peer, testIKEGroup(), table, nil, nil, log)
	if !errors.Is(err, errStopped) {
		t.Fatalf("the initiator cycle ended with %v, want errStopped", err)
	}
	if table.Len() != 0 {
		t.Errorf("the ended initiator cycle left %d SATable entries, want 0", table.Len())
	}
}
