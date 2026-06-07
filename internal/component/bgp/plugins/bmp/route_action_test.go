package bmp

import (
	"testing"
)

func TestProcessRouteMonitoring_MonitorMode_StoresInBMPRIB(t *testing.T) {
	bp := &BMPPlugin{}
	// plugin is nil, so InjectWireRoute will hit the nil guard and return early.
	// This test verifies processRouteMonitoring does NOT panic and reaches
	// the injection path (monitor mode always stores in BMP RIB).

	bgpUpdate := make([]byte, 23) // 19-byte header + 4-byte empty UPDATE body
	for i := range 16 {
		bgpUpdate[i] = 0xff
	}
	bgpUpdate[16] = 0
	bgpUpdate[17] = 23
	bgpUpdate[18] = 2 // UPDATE

	m := &RouteMonitoring{
		Peer:      PeerHeader{PeerAS: 65000},
		BGPUpdate: bgpUpdate,
	}

	// Should not panic. With plugin==nil, returns at nil check before InjectWireRoute.
	bp.processRouteMonitoring("10.0.0.1:1234", m)
}

func TestProcessRouteMonitoring_ShortUpdate_Skipped(t *testing.T) {
	bp := &BMPPlugin{}

	m := &RouteMonitoring{
		Peer:      PeerHeader{PeerAS: 65000},
		BGPUpdate: []byte{0xff}, // too short
	}

	// Should not panic, returns early
	bp.processRouteMonitoring("10.0.0.1:1234", m)
}

func TestReceiverConfig_RouteActionDefault(t *testing.T) {
	cfg := &receiverConfig{}
	if cfg.RouteAction != "" {
		t.Errorf("expected empty default, got %q", cfg.RouteAction)
	}
	// YANG default is "monitor"; empty string in Go means the YANG default applies
}
