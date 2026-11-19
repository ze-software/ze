// Design: docs/research/vpp-deployment-reference.md -- VPP connection generations
// VALIDATES: a retained owner cannot open a stream on a replacement connection.
package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/core"
)

// A disconnected connector cannot yield a usable ownership generation.
func TestConnectorGenerationRequiresConnection(t *testing.T) {
	c := NewConnector("unused")
	if _, err := c.ConnectionGeneration(); !errors.Is(err, errVppNotConnected) {
		t.Fatalf("generation error %v", err)
	}
	c.connected = true
	if _, err := c.ConnectionGeneration(); !errors.Is(err, errVppNotConnected) {
		t.Fatalf("missing connection error %v", err)
	}
}

// An epoch mismatch must refuse before touching even a structurally present
// replacement connection. A zero core.Connection would panic if selected.
func TestConnectorPinnedStreamRefusesReplacement(t *testing.T) {
	c := NewConnector("unused")
	c.conn = &core.Connection{}
	c.connected = true
	c.generation = 4
	if epoch, err := c.ConnectionGeneration(); epoch != 4 || err != nil {
		t.Fatalf("generation %d, %v", epoch, err)
	}
	c.generation = 5
	if stream, err := c.NewStreamForGeneration(context.Background(), 4); stream != nil || !errors.Is(err, errVppGenerationChanged) {
		t.Fatalf("stale stream %v, %v", stream, err)
	}
}

// Close invalidates prior leases even if no live transport remains, and a
// canceled context refuses without selecting a replacement transport.
func TestConnectorCloseInvalidatesGeneration(t *testing.T) {
	c := NewConnector("unused")
	c.generation = 4
	c.Close()
	if c.generation != 5 {
		t.Fatalf("generation after close %d", c.generation)
	}
	c.Close()
	if c.generation != 6 {
		t.Fatalf("generation after repeated close %d", c.generation)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if stream, err := c.NewStreamForGeneration(ctx, 4); stream != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled stream %v, %v", stream, err)
	}
}
