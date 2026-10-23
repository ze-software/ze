package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// heldWiringSession is the next session of p, negotiated with or without
// Graceful Restart on both sides, as Peer.runOnce builds and negotiates one.
func heldWiringSession(t *testing.T, p *Peer, gracefulRestart bool) *Session {
	t.Helper()
	s := p.newSession()
	s.localOpen = &message.Open{HoldTime: 90}
	s.peerOpen = &message.Open{ASN4: 65002, HoldTime: 90}
	var caps []capability.Capability
	if gracefulRestart {
		caps = append(caps, &capability.GracefulRestart{RestartTime: 120})
	}
	s.negotiateWith(caps, caps)
	return s
}

// TestMUPHeldGenerationWiredThroughThePeer drives the hold's lifetime from the
// entry points that own it: Peer.newSession shares the Peer's hold, negotiateWith
// opens a generation, and an End-of-RIB reaching notifyMessageReceiver sweeps it.
//
// VALIDATES: a generation filed on one session withdraws a malformed NLRI on the
// next when it negotiated Graceful Restart, not when it did not, and not after the
// new session's End-of-RIB for the family.
// PREVENTS: any of the three wiring lines being deleted with every test green.
func TestMUPHeldGenerationWiredThroughThePeer(t *testing.T) {
	k := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	l := mupWireNLRI(3, t1stBody(1, 0x00)...)
	withdrawnBy := func(s *Session) []byte {
		// An unrelated UPDATE first, so this session's UPDATE numbers move past the
		// previous session's and only the generation tells the two apart.
		applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, l), nil))
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, k), nil))
		nlri, _ := mpUnreachNLRIOf(t, wu.Payload())
		return nlri
	}

	t.Run("graceful restart keeps the generation", func(t *testing.T) {
		p := NewPeer(minimalPeerSettings(nil))
		holdT1STGeneration(t, heldWiringSession(t, p, true), mupFam, k0)
		assert.Equal(t, k0, withdrawnBy(heldWiringSession(t, p, true)))
	})

	t.Run("no graceful restart drops it", func(t *testing.T) {
		p := NewPeer(minimalPeerSettings(nil))
		holdT1STGeneration(t, heldWiringSession(t, p, true), mupFam, k0)
		assert.Nil(t, withdrawnBy(heldWiringSession(t, p, false)))
	})

	t.Run("End-of-RIB sweeps what the new session did not refresh", func(t *testing.T) {
		p := NewPeer(minimalPeerSettings(nil))
		p.state.Store(int32(PeerStateEstablished))
		holdT1STGeneration(t, heldWiringSession(t, p, true), mupFam, k0)
		next := heldWiringSession(t, p, true)

		r := &Reactor{
			clock:           p.clock,
			config:          &Config{LocalAS: 65001},
			peers:           map[netip.AddrPort]*Peer{p.Settings().PeerKey(): p},
			recentUpdates:   newRecentUpdateCache(100),
			attrModHandlers: attrModHandlersWithDefaults(),
			messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(plugin.PeerInfo, bgptypes.RawMessage) {}},
			api:             &pluginserver.Server{},
		}
		eor := makeUpdateBody(nil, []byte{0x80, 0x0f, 0x03, 0x00, byte(mupFam.AFI), byte(mupFam.SAFI)}, nil)
		eorWU := wireu.NewWireUpdate(eor, 0)
		fam, isEOR := eorWU.IsEOR()
		require.True(t, isEOR)
		require.Equal(t, mupFam, fam)
		r.notifyMessageReceiver(p.Settings().Address, msgtype.TypeUPDATE, eor, eorWU, 0, rpc.DirectionReceived,
			BufHandle{ID: noPoolBufID, Buf: eor}, nil, "", 0)

		assert.Nil(t, withdrawnBy(next), "the End-of-RIB removed the stale K0")
	})
}

// TestMUPWithdrawalOfAKeyTheSameUpdateAnnounces covers RFC 4271 Section 4.3 for the
// hold: "A BGP speaker SHOULD treat an UPDATE message of this form as though the
// WITHDRAWN ROUTES do not contain the address prefix."
//
// VALIDATES: (a) an MP_REACH_NLRI carrying well-formed K and malformed K' of the
// same key relays K and no withdrawal; (b) MP_REACH K with a malformed K' in the
// MP_UNREACH_NLRI relays K, withdraws nothing, and keeps K held; (c) MP_REACH K
// with a well-formed K in the MP_UNREACH_NLRI keeps K held, so a later malformed
// NLRI still withdraws it.
// PREVENTS: a withdraw-then-announce flap downstream, a withdrawal carrying the
// route this UPDATE announces, and a hold lost while its route stays.
func TestMUPWithdrawalOfAKeyTheSameUpdateAnnounces(t *testing.T) {
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	k1 := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0, 0x00)...)
	bad := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)

	t.Run("(a) same MP_REACH", func(t *testing.T) {
		s := nlriTypeTestSession()
		holdT1STGeneration(t, s, mupFam, k0)
		wu, _, validation := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, k1...), bad...)), nil))
		kept, _ := mpReachNLRIOf(t, wu.Payload())
		assert.Equal(t, k1, kept)
		_, found := mpUnreachNLRIOf(t, wu.Payload())
		assert.False(t, found)
		assert.Nil(t, validation.typedWithdrawBodies)
	})

	t.Run("(b) malformed in the MP_UNREACH", func(t *testing.T) {
		s := nlriTypeTestSession()
		holdT1STGeneration(t, s, mupFam, k0)
		attrs := append(mpReachAttrs(mupFam, k1), mpUnreachAttrs(mupFam, bad)...)
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, attrs, nil))
		_, found := mpUnreachNLRIOf(t, wu.Payload())
		assert.False(t, found, "K' names the key this UPDATE announces")
		wu, _, _ = applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, bad), nil))
		withdrawn, _ := mpUnreachNLRIOf(t, wu.Payload())
		assert.Equal(t, k1, withdrawn, "K stayed held")
	})

	t.Run("(c) well-formed in the MP_UNREACH", func(t *testing.T) {
		s := nlriTypeTestSession()
		attrs := append(mpReachAttrs(mupFam, k1), mpUnreachAttrs(mupFam, k1)...)
		applyRFC7606Body(t, s, makeUpdateBody(nil, attrs, nil))
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, bad), nil))
		withdrawn, _ := mpUnreachNLRIOf(t, wu.Payload())
		assert.Equal(t, k1, withdrawn, "K stayed held")
	})
}
