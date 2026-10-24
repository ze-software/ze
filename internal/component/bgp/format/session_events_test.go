package format

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func sessionEventPeer() *plugin.PeerInfo {
	return &plugin.PeerInfo{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAddress: netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65000,
		PeerAS:       rpc.RemoteASOf(65001),
	}
}

// sessionEventBGP parses a JSON event line and answers its `bgp` object.
func sessionEventBGP(t *testing.T, line []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(line, &decoded); err != nil {
		t.Fatalf("not JSON: %v: %s", err, line)
	}
	bgp, ok := decoded["bgp"].(map[string]any)
	if !ok {
		t.Fatalf("no bgp object: %s", line)
	}
	return bgp
}

// TestAppendConnected: the connected event names its kind and its peer, in
// both encodings.
func TestAppendConnected(t *testing.T) {
	bgp := sessionEventBGP(t, AppendConnected(nil, sessionEventPeer(), plugin.EncodingJSON))
	message, _ := bgp["message"].(map[string]any)
	if message["type"] != "connected" {
		t.Errorf("message.type = %v, want connected", message["type"])
	}
	if _, has := bgp["peer"]; !has {
		t.Errorf("no peer object: %v", bgp)
	}
	text := string(AppendConnected(nil, sessionEventPeer(), plugin.EncodingText))
	if text != "peer 10.0.0.1 remote as 65001 connected\n" {
		t.Errorf("text = %q", text)
	}
}

// TestAppendFSM: the fsm event states both ends of the change, by the
// RFC 4271 state names fsm.State writes.
func TestAppendFSM(t *testing.T) {
	line := AppendFSM(nil, sessionEventPeer(), fsm.StateConnect, fsm.StateOpenSent, plugin.EncodingJSON)
	bgp := sessionEventBGP(t, line)
	message, _ := bgp["message"].(map[string]any)
	if message["type"] != "fsm" {
		t.Errorf("message.type = %v, want fsm", message["type"])
	}
	transition, _ := bgp["fsm"].(map[string]any)
	if transition["from"] != "CONNECT" || transition["to"] != "OPENSENT" {
		t.Errorf("fsm = %v, want CONNECT to OPENSENT", transition)
	}
	text := string(AppendFSM(nil, sessionEventPeer(), fsm.StateOpenConfirm, fsm.StateEstablished, plugin.EncodingText))
	if text != "peer 10.0.0.1 remote as 65001 fsm OPENCONFIRM ESTABLISHED\n" {
		t.Errorf("text = %q", text)
	}
}

// TestAppendSignal: the signal event names the signal by number and by its C
// name, in both encodings.
func TestAppendSignal(t *testing.T) {
	bgp := sessionEventBGP(t, AppendSignal(nil, sessionEventPeer(), 15, "SIGTERM", plugin.EncodingJSON))
	message, _ := bgp["message"].(map[string]any)
	if message["type"] != "signal" {
		t.Errorf("message.type = %v, want signal", message["type"])
	}
	signal, _ := bgp["signal"].(map[string]any)
	if signal["code"] != float64(15) || signal["name"] != "SIGTERM" {
		t.Errorf("signal = %v, want 15 SIGTERM", signal)
	}
	text := string(AppendSignal(nil, sessionEventPeer(), 1, "SIGHUP", plugin.EncodingText))
	if text != "peer 10.0.0.1 remote as 65001 signal 1 SIGHUP\n" {
		t.Errorf("text = %q", text)
	}
}

// TestStateDownCarriesClosingNotification: a down closed by a NOTIFICATION
// states its direction and codes; a down no NOTIFICATION closed states none.
func TestStateDownCarriesClosingNotification(t *testing.T) {
	peer := sessionEventPeer()
	peer.CloseNotification = plugin.CloseNotification{Direction: plugin.NotifSent, Code: 6, Subcode: 4}
	bgp := sessionEventBGP(t, AppendStateChange(nil, peer, rpc.SessionStateDown, rpc.ReasonNotification, nil, plugin.EncodingJSON))
	closing, _ := bgp["notification"].(map[string]any)
	if closing["direction"] != "sent" || closing["code"] != float64(6) || closing["subcode"] != float64(4) {
		t.Errorf("notification = %v, want sent 6 4", closing)
	}
	text := string(AppendStateChange(nil, peer, rpc.SessionStateDown, rpc.ReasonNotification, nil, plugin.EncodingText))
	if text != "peer 10.0.0.1 remote as 65001 state down reason notification notification sent 6 4\n" {
		t.Errorf("text = %q", text)
	}

	peer.CloseNotification = plugin.CloseNotification{Direction: plugin.NotifReceived, Code: 4, Subcode: 0}
	bgp = sessionEventBGP(t, AppendStateChange(nil, peer, rpc.SessionStateDown, rpc.ReasonNotification, nil, plugin.EncodingJSON))
	closing, _ = bgp["notification"].(map[string]any)
	if closing["direction"] != "received" || closing["code"] != float64(4) {
		t.Errorf("notification = %v, want received 4 0", closing)
	}

	bgp = sessionEventBGP(t, AppendStateChange(nil, sessionEventPeer(), rpc.SessionStateDown, rpc.ReasonTCPFailure, nil, plugin.EncodingJSON))
	if _, has := bgp["notification"]; has {
		t.Errorf("a down no NOTIFICATION closed carries one: %v", bgp)
	}
}
