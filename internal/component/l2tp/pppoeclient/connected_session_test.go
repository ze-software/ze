// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- connected client fixtures.
// Related: network_phase.go -- keepalive consumes a constructed connected session.

package pppoeclient

import (
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
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
	// A fixture that names no keepalive runs at the pppoe-client defaults; a
	// test of the keepalive values sets them itself.
	if cfg.keepaliveInterval == 0 {
		cfg.keepaliveInterval = testKeepaliveInterval
	}
	if cfg.keepaliveFailures == 0 {
		cfg.keepaliveFailures = testKeepaliveFailures
	}
	session, err := newConnectedSession(transport, result, cfg, acceptTestIPv4Mode)
	if err != nil {
		t.Fatalf("construct connected session: %v", err)
	}
	return session
}

// The pppoe-client keepalive and reconnect defaults (ze-iface-conf.yang).
// iface's TestPPPoEClientKeepaliveDefaults proves the config parse yields
// them; these fixtures use them so a test that is not about the keepalive
// runs at the timing an operator gets.
const (
	testKeepaliveInterval = 10 * time.Second
	testKeepaliveFailures = 3
)

var testDialTiming = iface.PPPoEClientTiming{
	KeepaliveInterval: testKeepaliveInterval,
	KeepaliveFailures: testKeepaliveFailures,
	ReconnectDelayMin: time.Second,
	ReconnectDelayMax: 60 * time.Second,
}

// acceptTestIPv4Mode models a successful kernel mode change for the existing
// packet-consumer fixtures. It does not model or claim packet forwarding.
func acceptTestIPv4Mode(mode ppp.NPMode) error {
	if mode == ppp.NPModeUnspecified {
		return io.ErrUnexpectedEOF
	}
	return nil
}
