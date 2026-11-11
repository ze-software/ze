package bmp

import (
	"bytes"
	"net"
	"sync"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// injection is one call the RIB's inject-wire-route handler received.
type injection struct {
	protocol string
	peerKey  string
	body     []byte
}

// injectRecorder stands in for the RIB's typed inject-wire-route handler and
// keeps every call, so a test reads what processRouteMonitoring handed the RIB.
// Safe for concurrent use.
type injectRecorder struct {
	mu    sync.Mutex
	calls []injection
}

func (r *injectRecorder) inject(protocol, peerKey string, updateBody []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, injection{protocol: protocol, peerKey: peerKey, body: bytes.Clone(updateBody)})
	return nil
}

func (r *injectRecorder) recorded() []injection {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]injection(nil), r.calls...)
}

// bridgedBMPPlugin builds a BMPPlugin whose SDK handle reaches rec through a
// ready DirectBridge, the path an internal plugin's InjectWireRoute takes.
func bridgedBMPPlugin(t *testing.T, rec *injectRecorder) *BMPPlugin {
	t.Helper()

	bridge := rpc.NewDirectBridge()
	bridge.SetInjectWireRoute(rec.inject)
	bridge.SetReady()
	pluginSide, engineSide := net.Pipe()
	t.Cleanup(func() {
		closeLog(pluginSide, "plugin-side")
		closeLog(engineSide, "engine-side")
	})

	return &BMPPlugin{plugin: sdk.NewWithConn("bgp-bmp", rpc.NewBridgedConn(pluginSide, bridge))}
}

// routeMonitoringPeer is a Global Instance peer at 192.0.2.7, so its composite
// key is "<router>:192.0.2.7" (bmpCompositeKey).
func routeMonitoringPeer() PeerHeader {
	ph := PeerHeader{PeerType: PeerTypeGlobal, PeerAS: 65000}
	copy(ph.Address[12:], []byte{192, 0, 2, 7})
	return ph
}

// TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB proves a Route
// Monitoring message reaches the RIB once, in the BMP namespace, keyed by the
// monitored peer, with the UPDATE body and not its 19-byte header.
//
// Method: the RIB's inject handler is a recorder on a ready DirectBridge, so
// the call processRouteMonitoring makes through the SDK lands in the recorder.
// Red when the InjectWireRoute call is removed (no injection is recorded).
func TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB(t *testing.T) {
	rec := &injectRecorder{}
	bp := bridgedBMPPlugin(t, rec)

	// 19-byte BGP header (marker, length 25, type UPDATE), then a 6-byte body.
	update := make([]byte, 0, 25)
	update = append(update, bytes.Repeat([]byte{0xff}, 16)...)
	update = append(update, 0, 25, 2)
	body := []byte{0x00, 0x00, 0x00, 0x02, 0xab, 0xcd}
	update = append(update, body...)

	bp.processRouteMonitoring("10.0.0.1:1234", &RouteMonitoring{Peer: routeMonitoringPeer(), BGPUpdate: update})

	calls := rec.recorded()
	if len(calls) != 1 {
		t.Fatalf("injections: got %d, want 1: %+v", len(calls), calls)
	}
	got := calls[0]
	if got.protocol != protocolBMP {
		t.Errorf("protocol: got %q, want %q", got.protocol, protocolBMP)
	}
	if got.peerKey != "10.0.0.1:1234:192.0.2.7" {
		t.Errorf("peer key: got %q, want %q", got.peerKey, "10.0.0.1:1234:192.0.2.7")
	}
	if !bytes.Equal(got.body, body) {
		t.Errorf("body: got % x, want % x (the bytes after the %d-byte BGP header)", got.body, body, bgpHeaderSize)
	}
}

// TestProcessRouteMonitoring_ShortUpdate_Skipped proves an UPDATE shorter
// than the 19-byte BGP header reaches the RIB not at all.
//
// Method: the same recorder as above, so the SDK handle is set and the length
// guard is the only thing between the message and an injection. Red when the
// length guard is removed (the slice past the header panics or injects).
func TestProcessRouteMonitoring_ShortUpdate_Skipped(t *testing.T) {
	rec := &injectRecorder{}
	bp := bridgedBMPPlugin(t, rec)

	short := bytes.Repeat([]byte{0xff}, bgpHeaderSize-1)
	bp.processRouteMonitoring("10.0.0.1:1234", &RouteMonitoring{Peer: routeMonitoringPeer(), BGPUpdate: short})

	if calls := rec.recorded(); len(calls) != 0 {
		t.Fatalf("injections: got %d, want 0: %+v", len(calls), calls)
	}
}
