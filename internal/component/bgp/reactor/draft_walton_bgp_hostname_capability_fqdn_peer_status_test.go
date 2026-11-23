package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

func fqdnCapabilityTLV(hostname, domain string) []byte {
	f := &capability.FQDN{Hostname: hostname, DomainName: domain}
	tlv := make([]byte, f.Len())
	f.WriteTo(tlv, 0)
	return tlv
}

// RFC requirement: DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY-4-1 positive -- a peer
// OPEN carrying FQDN capability (code 73) propagates through session negotiation,
// format.NegotiatedToDecoded, Reactor.API().Peers(), and
// Reactor.API().PeerNegotiatedCapabilities(), even when Ze itself did not
// advertise code 73, and clears when the session tears down.
func TestReactorSessionPropagatesAndClearsPeerFQDN(t *testing.T) {
	t.Parallel()

	s, conn, sent := newCapabilityOpenSession(t, []capability.Capability{capIPv4()}, nil)

	openCaps := append([]byte{1, 4, 0, 1, 0, 1}, fqdnCapabilityTLV("spine-01", "dc1.example")...)
	require.NoError(t, s.handleOpen(openBodyWithParameters(openCaps)))
	require.Equal(t, fsm.StateOpenConfirm, s.State())
	requireOnlyKeepalive(t, conn.written()[sent:])

	neg := s.Negotiated()
	require.NotNil(t, neg)
	require.NotNil(t, neg.PeerFQDN)
	assert.Equal(t, "spine-01", neg.PeerFQDN.Hostname)
	assert.Equal(t, "dc1.example", neg.PeerFQDN.DomainName)

	decoded := format.NegotiatedToDecoded(neg)
	require.NotNil(t, decoded.FQDN)
	assert.Equal(t, "spine-01", decoded.FQDN.Hostname)
	assert.Equal(t, "dc1.example", decoded.FQDN.DomainName)

	// Wire the negotiated capabilities into a real Reactor peer and read both API surfaces.
	r := New(&Config{})
	peerAddr := netip.MustParseAddr("192.0.2.1")
	p := NewPeer(s.settings)
	p.negotiated.Store(NewNegotiatedCapabilities(neg))
	r.mu.Lock()
	r.peers[s.settings.PeerKey()] = p
	r.mu.Unlock()

	api := &reactorAPIAdapter{r: r}
	peers := api.Peers()
	require.Len(t, peers, 1)
	assert.Equal(t, "spine-01", peers[0].PeerHostname)
	assert.Equal(t, "dc1.example", peers[0].PeerDomainName)

	capsInfo := api.PeerNegotiatedCapabilities(peerAddr)
	require.NotNil(t, capsInfo)
	assert.Equal(t, "spine-01", capsInfo.Hostname)
	assert.Equal(t, "dc1.example", capsInfo.DomainName)

	// Session teardown clears p.negotiated so stale FQDN metadata never survives disconnect.
	p.negotiated.Store(nil)
	peersAfter := api.Peers()
	require.Len(t, peersAfter, 1)
	assert.Empty(t, peersAfter[0].PeerHostname)
	assert.Empty(t, peersAfter[0].PeerDomainName)
	assert.Nil(t, api.PeerNegotiatedCapabilities(peerAddr))
}

// TestReactorSessionSanitizesHostilePeerFQDN proves that an OPEN carrying terminal
// escape sequences in code 73 still establishes the BGP session without error while
// withholding the unsafe string from operator status surfaces.
func TestReactorSessionSanitizesHostilePeerFQDN(t *testing.T) {
	t.Parallel()

	s, conn, sent := newCapabilityOpenSession(t, []capability.Capability{capIPv4()}, nil)

	openCaps := append([]byte{1, 4, 0, 1, 0, 1}, fqdnCapabilityTLV("\x1b[2Jevil", "safe.example")...)
	require.NoError(t, s.handleOpen(openBodyWithParameters(openCaps)))
	require.Equal(t, fsm.StateOpenConfirm, s.State())
	requireOnlyKeepalive(t, conn.written()[sent:])

	neg := s.Negotiated()
	require.NotNil(t, neg)
	require.NotNil(t, neg.PeerFQDN)
	assert.Empty(t, neg.PeerFQDN.Hostname, "control-char hostname must be withheld")
	assert.Equal(t, "safe.example", neg.PeerFQDN.DomainName, "valid domain half must be preserved")
}
