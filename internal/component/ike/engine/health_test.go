// Design: plan/learned/745-ipsec-10-cli-diag.md -- IPsec health check tests

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/health"
)

func TestIPsecHealthCheck_NoEngine(t *testing.T) {
	activeTablePtr.Store(nil)
	setActivePeers(nil)

	status, reason := checkIPsecHealth()
	assert.Equal(t, health.StatusDown, status)
	assert.Contains(t, reason, "not running")
}

func TestIPsecHealthCheck_NoPeers(t *testing.T) {
	table := NewSATable()
	activeTablePtr.Store(table)
	peers := make(map[string]*PeerSession)
	setActivePeers(peers)

	status, _ := checkIPsecHealth()
	assert.Equal(t, health.StatusHealthy, status)

	activeTablePtr.Store(nil)
	setActivePeers(nil)
}

func TestIPsecHealthCheck_Degraded(t *testing.T) {
	table := NewSATable()
	activeTablePtr.Store(table)
	peers := map[string]*PeerSession{
		"peer-a": {peerName: "peer-a"},
	}
	setActivePeers(peers)

	status, reason := checkIPsecHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "no established")

	activeTablePtr.Store(nil)
	setActivePeers(nil)
}

func TestIPsecHealthCheck_Healthy(t *testing.T) {
	table := NewSATable()
	sa := &SA{PeerName: "peer-a", State: StateEstablished}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	table.Insert(sa)
	activeTablePtr.Store(table)
	peers := map[string]*PeerSession{
		"peer-a": {peerName: "peer-a"},
	}
	setActivePeers(peers)

	status, _ := checkIPsecHealth()
	assert.Equal(t, health.StatusHealthy, status)

	activeTablePtr.Store(nil)
	setActivePeers(nil)
}

func TestIPsecHealthCheck_PartialDegraded(t *testing.T) {
	table := NewSATable()
	sa := &SA{PeerName: "peer-a", State: StateEstablished}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	table.Insert(sa)
	activeTablePtr.Store(table)
	peers := map[string]*PeerSession{
		"peer-a": {peerName: "peer-a"},
		"peer-b": {peerName: "peer-b"},
	}
	setActivePeers(peers)

	status, reason := checkIPsecHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "some peers")

	activeTablePtr.Store(nil)
	setActivePeers(nil)
}
