package engine

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"
)

// VALIDATES: a later authenticated packet from a new endpoint, under MOBIKE, is
// refused, the stored endpoint stays, and the refusal is logged at Debug with the
// peer and both endpoints.
// PREVENTS: the silent refusal that made the port-translated interop scenarios
// time out with nothing in Ze's log to say MOBIKE was the reason.
// Method: call adoptAuthenticatedEndpoint on an SA with MOBIKE enabled and a
// stored endpoint, through a logger that writes Debug into a buffer.
func TestMOBIKEEndpointRefusalIsLogged(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	stored := &net.UDPAddr{IP: net.IPv4(172, 28, 0, 7), Port: 14500}
	observed := &net.UDPAddr{IP: net.IPv4(172, 28, 0, 7), Port: 14501}
	sa := &SA{PeerName: "swan", peerEndpoint: copyUDPAddr(stored)}
	sa.mobike.enabled = true

	if sa.adoptAuthenticatedEndpoint(observed, false, log) {
		t.Fatal("adoptAuthenticatedEndpoint moved the endpoint while MOBIKE is in use")
	}
	if !sameUDPEndpoint(sa.peerEndpoint, stored) {
		t.Fatalf("stored endpoint = %v, want it kept at %v", sa.peerEndpoint, stored)
	}
	line := output.String()
	for _, want := range []string{"level=DEBUG", "MOBIKE is in use", "RFC 4555 Section 3.8", "peer=swan", "stored=172.28.0.7:14500", "observed=172.28.0.7:14501"} {
		if !strings.Contains(line, want) {
			t.Fatalf("log output %q lacks %q", line, want)
		}
	}
}
