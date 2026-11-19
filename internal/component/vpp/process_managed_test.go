// Design: docs/research/vpp-deployment-reference.md -- managed physical peer ownership
package vpp

import (
	"context"
	"errors"
	"sync"
	"testing"

	"go.fd.io/govpp/api"
)

// This fixture owns the API witness just as the native transport does. It has
// no reported PID; arbitrary API replies cannot decide physical ownership.
type managedPeerTestConnection struct {
	recordingConnection
	peer *processWitness
	once sync.Once
	test *testing.T
}

func (c *managedPeerTestConnection) processPeer() *processWitness { return c.peer }
func (c *managedPeerTestConnection) Disconnect() {
	c.once.Do(func() {
		c.recordingConnection.Disconnect()
		if err := c.peer.release(); err != nil {
			c.test.Error(err)
		}
	})
}

// A foreign physical peer with the same logical epoch has no managed receipt.
// A same-process peer is accepted without asking an API for a numeric PID.
func TestVPPManagedProcessRequiresExactServer(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "exact"}[same], func(t *testing.T) {
			child, _ := newPeerTestWitness(t, 410)
			peerInode := uint64(411)
			if same {
				peerInode = 410
			}
			peer, _ := newPeerTestWitness(t, peerInode)
			connector := NewConnector("unused")
			connection := &managedPeerTestConnection{peer: peer, test: t}
			connection.open = func(context.Context) (api.Stream, error) {
				return nil, errors.New("binding asked the untrusted API for identity")
			}
			connector.conn, connector.connected, connector.generation = connection, true, 7
			t.Cleanup(connector.Close)
			owned, err := connector.bindManagedProcess(t.Context(), child)
			if !same {
				if err == nil {
					t.Fatal("foreign Unix peer acquired a managed receipt")
				}
				if owned != (managedConnection{}) {
					t.Fatal("refused binding retained authority")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if owned.connector != connector.Identity() {
				t.Fatal("wrong connector receipt")
			}
			if owned.generation != 7 {
				t.Fatal("wrong generation receipt")
			}
			if owned.physical != child.identity {
				t.Fatal("spawn identity was lost")
			}
			if connection.streams.Load() != 0 {
				t.Fatal("reported PID became ownership")
			}
		})
	}
}

// Process wait is still joined by the manager, but it cannot manufacture a
// socket-peer exit notification or close a newer transport on that connector.
func TestVPPManagedRetirementCannotRetireReplacement(t *testing.T) {
	child, _ := newPeerTestWitness(t, 420)
	peer, _ := newPeerTestWitness(t, 420)
	connector := NewConnector("unused")
	old := &managedPeerTestConnection{peer: peer, test: t}
	connector.conn, connector.connected, connector.generation = old, true, 7
	owned, err := connector.bindManagedProcess(t.Context(), child)
	if err != nil {
		t.Fatal(err)
	}
	replacementPeer, _ := newPeerTestWitness(t, 421)
	replacement := &managedPeerTestConnection{peer: replacementPeer, test: t}
	connector.conn, connector.generation = replacement, 8
	t.Cleanup(connector.Close)
	t.Cleanup(old.Disconnect)
	proofs := observeProcessExits(t)
	connector.retireManagedConnection(owned)
	if replacement.disconnects.Load() != 0 {
		t.Fatal("old child wait closed a replacement")
	}
	if generation, err := connector.ConnectionGeneration(); err != nil || generation != 8 {
		t.Fatalf("replacement generation=%d: %v", generation, err)
	}
	if len(proofs()) != 0 {
		t.Fatal("managed wait manufactured socket-peer death")
	}
}

// A logical receipt naming this exact epoch still cannot release a live pidfd
// lease. Only an independently observed matching physical process can do so.
func TestVPPManagedLogicalProofCannotRetirePhysicalLease(t *testing.T) {
	witness, descriptor := newPeerTestWitness(t, 430)
	if err := witness.bind(81, 7, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := witness.retain()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := lease.Retire(exitedProcess{connector: 81, generation: 7}); err == nil {
		t.Fatal("logical managed receipt retired a live foreign physical peer")
	}
	if proof, err := lease.Exit(); err != nil || proof != nil {
		t.Fatal("refusal changed its live lease")
	}
	descriptor.mu.Lock()
	descriptor.dead = true
	descriptor.mu.Unlock()
	proof, err := lease.Exit()
	if err != nil {
		t.Fatal(err)
	}
	if proof == nil {
		t.Fatal("actual process exit produced no proof")
	}
	if err := lease.Retire(proof); err != nil {
		t.Fatal(err)
	}
}

// Dead or unobservable ownership cannot become ready, even when the retained
// identity bytes agree. Cancellation and missing witnesses remain refusals.
func TestVPPManagedBindingRefusesUnusableWitness(t *testing.T) {
	for _, state := range []string{"child exited", "peer exited", "canceled", "missing child", "missing socket witness"} {
		t.Run(state, func(t *testing.T) {
			child, childDescriptor := newPeerTestWitness(t, 440)
			peer, peerDescriptor := newPeerTestWitness(t, 440)
			connector := NewConnector("unused")
			connector.conn, connector.connected, connector.generation = &managedPeerTestConnection{peer: peer, test: t}, true, 7
			t.Cleanup(connector.Close)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch state {
			case "child exited":
				childDescriptor.dead = true
			case "peer exited":
				peerDescriptor.dead = true
			case "canceled":
				cancel()
			case "missing child":
				child = nil
			case "missing socket witness":
				connector.conn = &recordingConnection{}
			}
			owned, err := connector.bindManagedProcess(ctx, child)
			if err == nil {
				t.Fatal("unusable managed peer acquired authority")
			}
			if owned != (managedConnection{}) {
				t.Fatal("refused binding retained authority")
			}
		})
	}
}

// Even an exact receipt carries no observation that its peer died. This also
// discriminates against restoring the former manual managed-wait notification.
func TestVPPManagedWaitReceiptCannotPublishExit(t *testing.T) {
	child, _ := newPeerTestWitness(t, 450)
	peer, _ := newPeerTestWitness(t, 450)
	connector := NewConnector("unused")
	connector.conn, connector.connected, connector.generation = &managedPeerTestConnection{peer: peer, test: t}, true, 7
	owned, err := connector.bindManagedProcess(t.Context(), child)
	if err != nil {
		t.Fatal(err)
	}
	proofs := observeProcessExits(t)
	connector.retireManagedConnection(owned)
	if len(proofs()) != 0 {
		t.Fatal("a logical managed receipt published death of a live peer")
	}
	if err := child.alive(); err != nil {
		t.Fatal("transport retirement changed its live child")
	}
}
