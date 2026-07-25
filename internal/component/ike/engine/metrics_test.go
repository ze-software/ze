// Design: plan/learned/745-ipsec-10-cli-diag.md -- IPsec metrics tests

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/metrics"
)

func TestIPsecMetrics_NoEngine(t *testing.T) {
	activeTablePtr.Store(nil)
	setActivePeers(nil)

	reg := metrics.NopRegistry{}
	m := RegisterMetrics(reg)
	m.Update()
}

func TestIPsecMetrics_WithSAs(t *testing.T) {
	table := NewSATable()
	sa := &SA{PeerName: "peer-a", State: StateEstablished}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	table.Insert(sa)
	activeTablePtr.Store(table)
	peers := map[string]*PeerSession{
		"peer-a": {peerName: "peer-a"},
	}
	setActivePeers(peers)

	reg := metrics.NopRegistry{}
	m := RegisterMetrics(reg)
	assert.NotNil(t, m)
	m.Update()

	activeTablePtr.Store(nil)
	setActivePeers(nil)
}
