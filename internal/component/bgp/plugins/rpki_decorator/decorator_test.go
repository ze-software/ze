// VALIDATES: parseEventMeta correctly extracts event metadata from JSON.
// PREVENTS: Wrong Union correlation from malformed events.
package rpki_decorator

import "testing"

func TestParseEventMeta(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  string
		wantPeer  string
		wantMsgID uint64
	}{
		{
			name:      "valid update event",
			input:     `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","remote":{"as":65001}},"message":{"id":42,"direction":"received","type":"update"}}}`,
			wantType:  "update",
			wantPeer:  "10.0.0.1",
			wantMsgID: 42,
		},
		{
			name:      "valid rpki event",
			input:     `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","remote":{"as":65001}},"message":{"id":42,"type":"rpki"},"rpki":{}}}`,
			wantType:  "rpki",
			wantPeer:  "10.0.0.1",
			wantMsgID: 42,
		},
		{
			name:      "invalid JSON",
			input:     "not-json",
			wantType:  "",
			wantPeer:  "",
			wantMsgID: 0,
		},
		{
			name:      "empty string",
			input:     "",
			wantType:  "",
			wantPeer:  "",
			wantMsgID: 0,
		},
		{
			name:      "missing bgp key",
			input:     `{"type":"bgp"}`,
			wantType:  "",
			wantPeer:  "",
			wantMsgID: 0,
		},
		{
			name:      "missing peer address",
			input:     `{"type":"bgp","bgp":{"message":{"id":1,"type":"update"}}}`,
			wantType:  "update",
			wantPeer:  "",
			wantMsgID: 1,
		},
		{
			name:      "missing message type",
			input:     `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"},"message":{"id":1}}}`,
			wantType:  "",
			wantPeer:  "10.0.0.1",
			wantMsgID: 1,
		},
		{
			name:      "missing message ID",
			input:     `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"},"message":{"type":"update"}}}`,
			wantType:  "update",
			wantPeer:  "10.0.0.1",
			wantMsgID: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotPeer, gotMsgID := parseEventMeta(tt.input)
			if gotType != tt.wantType {
				t.Errorf("eventType = %q, want %q", gotType, tt.wantType)
			}
			if gotPeer != tt.wantPeer {
				t.Errorf("peerAddr = %q, want %q", gotPeer, tt.wantPeer)
			}
			if gotMsgID != tt.wantMsgID {
				t.Errorf("msgID = %d, want %d", gotMsgID, tt.wantMsgID)
			}
		})
	}
}
