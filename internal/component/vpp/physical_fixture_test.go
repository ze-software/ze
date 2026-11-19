//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- original fixture counters on a genuine peer
// Related: physical_fixture_linux_test.go -- actual descriptor capture and owned release
package vpp

import "testing"

// The original recording transport still owns every channel/send/receive
// observation. This adapter adds only a separately owned genuine peer witness;
// the test cleanup retains it through the original worker joins.
type physicalFixtureConnection struct {
	connectorConnection
	peer *processWitness
}

func (connection *physicalFixtureConnection) processPeer() *processWitness { return connection.peer }

func physicalFixtureConnector(t *testing.T, owner *Connector) *Connector {
	t.Helper()
	peer := newRuntimeStatsTestPeer(t)
	owner.mu.Lock()
	owner.conn = &physicalFixtureConnection{connectorConnection: owner.conn, peer: peer}
	owner.mu.Unlock()
	return owner
}

// A model supplies the same genuine original peer as the child input. The
// separate actual-runOnce fixture proves that production obtains its child
// witness atomically at spawn; this constructor fabricates no spawn receipt.
func physicalFixtureManagedConnection(t *testing.T, owner *Connector) managedConnection {
	t.Helper()
	owned, err := owner.bindManagedProcess(t.Context(), physicalFixturePeer(t, owner))
	if err != nil {
		t.Fatal(err)
	}
	return owned
}

func physicalFixturePeer(t *testing.T, owner *Connector) *processWitness {
	t.Helper()
	owner.mu.Lock()
	connection, ok := owner.conn.(witnessedConnection)
	owner.mu.Unlock()
	if !ok {
		t.Fatal("managed fixture has no original physical API witness")
	}
	peer := connection.processPeer()
	if peer == nil {
		t.Fatal("managed fixture has no retained physical API peer")
	}
	return peer
}
