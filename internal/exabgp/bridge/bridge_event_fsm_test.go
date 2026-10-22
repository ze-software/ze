package bridge

import (
	"encoding/json"
	"testing"
)

func readZeLine(t *testing.T, line string) Event {
	t.Helper()
	var zebgp map[string]any
	if err := json.Unmarshal([]byte(line), &zebgp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	return ReadEvent(zebgp)
}

const zePeerJSON = `"peer":{"local":{"address":"10.0.0.17","as":64512},"remote":{"address":"10.0.0.2","as":64512},"router-id":"10.0.0.17"}`

// TestConnectedIsExabgpState: ze's `connected` event is ExaBGP's state
// `connected` on both encoders, under neighbor-changes.
func TestConnectedIsExabgpState(t *testing.T) {
	event := readZeLine(t, `{"type":"bgp","bgp":{"message":{"type":"connected"},`+zePeerJSON+`}}`)
	if event.APIKey() != apiKeyNeighborChanges {
		t.Errorf("APIKey = %q, want %q", event.APIKey(), apiKeyNeighborChanges)
	}
	doc := event.ExabgpJSON(FormatV4JSON)
	neighbor, _ := doc["neighbor"].(map[string]any)
	if doc["type"] != msgTypeState || neighbor["state"] != "connected" {
		t.Errorf("JSON = %v", doc)
	}
	text, form := event.AppendText(nil)
	if form != TextWritten || string(text) != "neighbor 10.0.0.2 connected\n" {
		t.Errorf("text = %q (%v)", text, form)
	}
}

// TestFSMIsExabgpFSM: ze's `fsm` event is ExaBGP's `fsm` with the new state,
// and the text encoder writes no line for it (Response.Text.fsm answers None).
func TestFSMIsExabgpFSM(t *testing.T) {
	event := readZeLine(t, `{"type":"bgp","bgp":{"message":{"type":"fsm"},`+zePeerJSON+`,"fsm":{"from":"CONNECT","to":"OPENSENT"}}}`)
	if event.APIKey() != apiKeyFSM {
		t.Errorf("APIKey = %q, want %q", event.APIKey(), apiKeyFSM)
	}
	doc := event.ExabgpJSON(FormatV4JSON)
	neighbor, _ := doc["neighbor"].(map[string]any)
	if doc["type"] != msgTypeFSM || neighbor["state"] != "OPENSENT" {
		t.Errorf("JSON = %v", doc)
	}
	if _, form := event.AppendText(nil); form != TextNone {
		t.Errorf("text form = %v, want TextNone", form)
	}
}

// TestSignalIsExabgpSignal: ze's `signal` event is ExaBGP's `signal`, the
// number as a string beside its C name inside `neighbor`, and the text encoder
// writes no line for it (Response.Text.signal answers None).
func TestSignalIsExabgpSignal(t *testing.T) {
	event := readZeLine(t, `{"type":"bgp","bgp":{"message":{"type":"signal"},`+zePeerJSON+`,"signal":{"code":15,"name":"SIGTERM"}}}`)
	if event.APIKey() != apiKeySignal {
		t.Errorf("APIKey = %q, want %q", event.APIKey(), apiKeySignal)
	}
	doc := event.ExabgpJSON(FormatV4JSON)
	neighbor, _ := doc["neighbor"].(map[string]any)
	if doc["type"] != msgTypeSignal || neighbor["code"] != "15" || neighbor["name"] != "SIGTERM" {
		t.Errorf("JSON = %v", doc)
	}
	if _, form := event.AppendText(nil); form != TextNone {
		t.Errorf("text form = %v, want TextNone", form)
	}
}

// TestDownNamesClosingNotification: a down a NOTIFICATION closed is
// `notification sent (6,4)` or `notification received (4,0)`, ExaBGP's words
// (Peer._run, src/exabgp/reactor/peer.py), on both encoders.
func TestDownNamesClosingNotification(t *testing.T) {
	for _, tc := range []struct{ member, want string }{
		{`{"direction":"sent","code":6,"subcode":4}`, "notification sent (6,4)"},
		{`{"direction":"received","code":4,"subcode":0}`, "notification received (4,0)"},
	} {
		event := readZeLine(t, `{"type":"bgp","bgp":{"message":{"type":"state"},`+zePeerJSON+`,"state":"down","reason":"notification","notification":`+tc.member+`}}`)
		neighbor, _ := event.ExabgpJSON(FormatV4JSON)["neighbor"].(map[string]any)
		if neighbor["reason"] != tc.want {
			t.Errorf("JSON reason = %v, want %q", neighbor["reason"], tc.want)
		}
		text, _ := event.AppendText(nil)
		if string(text) != "neighbor 10.0.0.2 down - "+tc.want+"\n" {
			t.Errorf("text = %q, want %q", text, tc.want)
		}
	}
}
