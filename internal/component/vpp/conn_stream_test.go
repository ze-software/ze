// Design: docs/research/vpp-deployment-reference.md -- context-bound VPP streams

package vpp

import (
	"context"
	"errors"
	"testing"
)

// TestConnectorStreamRefusesUnavailableContextConnection observes the public
// stream method's refusal before any transport or stream can be allocated.
func TestConnectorStreamRefusesUnavailableContextConnection(t *testing.T) {
	connector := NewConnector("unused")
	if stream, err := connector.NewStream(nil); stream != nil || err == nil { //nolint:staticcheck // SA1012: intentionally nil to prove refusal before any transport allocation.
		t.Fatalf("nil context: stream=%v error=%v", stream, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if stream, err := connector.NewStream(ctx); stream != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: stream=%v error=%v", stream, err)
	}
	if stream, err := connector.NewStream(context.Background()); stream != nil || !errors.Is(err, errVppNotConnected) {
		t.Fatalf("disconnected: stream=%v error=%v", stream, err)
	}
	connector.connected = true
	if stream, err := connector.NewStream(context.Background()); stream != nil || !errors.Is(err, errVppNotConnected) {
		t.Fatalf("missing connection: stream=%v error=%v", stream, err)
	}
}
