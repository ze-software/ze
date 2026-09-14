// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: gtsm.go -- gtsmPeers and publishGTSMKernelState, the code under test

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/gtsm"
)

// captureGTSMPeers redirects the gtsm component's reconcile entry point into
// the returned slice pointer, so a test reads the peer set this reactor
// derives without a kernel to publish it to.
func captureGTSMPeers(t *testing.T) *[]gtsm.Peer {
	t.Helper()

	var published []gtsm.Peer
	previous := setGTSMPeers
	setGTSMPeers = func(peers []gtsm.Peer) error {
		published = peers
		return nil
	}
	t.Cleanup(func() { setGTSMPeers = previous })
	return &published
}

// noopJournal is the rollback journal the config apply hands the reconcile.
// This test needs the reconcile to run, not to roll back, so every recorded
// step is applied and nothing is kept.
type noopJournal struct{}

func (noopJournal) Record(apply, _ func() error) error { return apply() }
func (noopJournal) Rollback() []error                  { return nil }
func (noopJournal) Discard()                           {}

// TestReactorPublishesGTSMPeersFromConfig is the wiring proof: a peer whose
// configuration derived the GTSM TTL values reaches the gtsm component through
// the same peer reconcile a config apply runs, carrying the address, the BGP
// port and both TTL values.
//
// VALIDATES: reconcilePeersJournaled publishes the GTSM peer set it produced.
// PREVENTS: a GTSM peer whose kernel state is installed by nothing, because
// the only publisher ran at start and never again.
func TestReactorPublishesGTSMPeersFromConfig(t *testing.T) {
	published := captureGTSMPeers(t)
	r := New(&Config{Port: 179})

	gtsmPeer := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	// The two values parseTTLSettings derives from `connection ttl max 1`.
	gtsmPeer.OutTTL = 255
	gtsmPeer.MinTTL = 255
	plain := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65002, 0x01010101)

	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.reconcilePeersJournaled([]*PeerSettings{gtsmPeer, plain}, "test", noopJournal{}))

	require.Len(t, *published, 1, "only the peer that enabled GTSM owes kernel state")
	got := (*published)[0]
	assert.Equal(t, gtsmPeer.Address, got.Addr)
	assert.Equal(t, uint16(179), got.Port, "the port that associates a quoted TCP header with this session")
	assert.Equal(t, uint8(255), got.HopLimit)
	assert.Equal(t, uint8(255), got.Floor)
}

// TestReactorPublishesAPeersOwnListenPort proves the port handed to the filter
// is the peer's own, not the default: a term built with 179 would claim no
// session of a peer that runs BGP somewhere else, and the error it was meant
// to refuse would be delivered.
func TestReactorPublishesAPeersOwnListenPort(t *testing.T) {
	published := captureGTSMPeers(t)
	r := New(&Config{Port: 179})

	settings := NewPeerSettings(mustParseAddr("10.0.0.4"), 65000, 65004, 0x01010101)
	settings.LocalPort = 1179
	settings.OutTTL = 255
	settings.MinTTL = 254
	require.NoError(t, r.AddPeer(settings))

	r.publishGTSMKernelState()

	require.Len(t, *published, 1)
	assert.Equal(t, uint16(1179), (*published)[0].Port)
	assert.Equal(t, uint8(254), (*published)[0].Floor, "the floor is the peer's own, so a multi-hop session keeps working")
}

// TestReactorPublishesNoPeerWithoutGTSM keeps the feature off a deployment
// that did not ask for it: no peer in the set means the gtsm component
// installs no route and never loads the firewall backend.
func TestReactorPublishesNoPeerWithoutGTSM(t *testing.T) {
	published := captureGTSMPeers(t)
	r := New(&Config{Port: 179})

	require.NoError(t, r.AddPeer(NewPeerSettings(mustParseAddr("10.0.0.3"), 65000, 65003, 0x01010101)))

	r.publishGTSMKernelState()

	assert.Empty(t, *published)
}
