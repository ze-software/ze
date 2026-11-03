// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- connected client fixtures.
// Related: network_phase.go -- keepalive consumes a constructed connected session.

package pppoeclient

import (
	"io"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// newTestConnectedSession supplies the resources of an already connected unit
// without kernel descriptors. Existing wire assertions still observe transport.
// The supplied receive owner retains the fixture's original packet sources.
// These tests do not assert kernel mode ordering; restart tests use a recording
// mode setter at the same constructor boundary.
func newTestConnectedSession(t *testing.T, transport io.ReadWriteCloser,
	result sessionResult, cfg sessionConfig) *connectedSession {
	t.Helper()
	result.localIP = netip.MustParseAddr("192.0.2.2")
	result.peerIP = netip.MustParseAddr("192.0.2.1")
	result.negMTU = 1492
	cfg.mtu = 1492
	session, err := newConnectedSession(transport, result, cfg, acceptTestIPv4Mode)
	if err != nil {
		t.Fatalf("construct connected session: %v", err)
	}
	return session
}

// acceptTestIPv4Mode models a successful kernel mode change for the existing
// packet-consumer fixtures. It does not model or claim packet forwarding.
func acceptTestIPv4Mode(mode ppp.NPMode) error {
	if mode == ppp.NPModeUnspecified {
		return io.ErrUnexpectedEOF
	}
	return nil
}
