package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Monitor Visual Text Formatting Tests
// =============================================================================

// TestFormatMonitorLineUpdate verifies UPDATE event text rendering.
//
// VALIDATES: UPDATE events show direction, peer, ASN, prefixes with +/- notation.
// PREVENTS: UPDATE events formatted incorrectly or missing key fields.
func TestFormatMonitorLineUpdate(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "announce_single_prefix",
			json: `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.1.0/24"]}]}}}}`,
			want: "recv UPDATE 10.0.0.1 AS65001 +10.0.1.0/24 nhop=10.0.1.254",
		},
		{
			name: "announce_multiple_prefixes",
			json: `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.1.0/24","10.0.2.0/24","10.0.3.0/24"]}]}}}}`,
			want: "recv UPDATE 10.0.0.1 AS65001 +10.0.1.0/24 +10.0.2.0/24 +10.0.3.0/24 nhop=10.0.1.254",
		},
		{
			name: "withdraw",
			json: `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"update":{"nlri":{"ipv4/unicast":[{"action":"del","nlri":["10.0.3.0/24"]}]}}}}`,
			want: "recv UPDATE 10.0.0.1 AS65001 -10.0.3.0/24",
		},
		{
			name: "sent_update",
			json: `{"type":"bgp","bgp":{"message":{"type":"update","direction":"sent"},"peer":{"address":"10.0.0.2","remote":{"as":65002}},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"10.0.0.1","action":"add","nlri":["10.0.4.0/24"]}]}}}}`,
			want: "sent UPDATE 10.0.0.2 AS65002 +10.0.4.0/24 nhop=10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMonitorLine(tt.json)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFormatMonitorLineState verifies STATE event text rendering.
//
// VALIDATES: State changes show peer, ASN, state, and reason.
// PREVENTS: State events missing reason or using wrong format.
func TestFormatMonitorLineState(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "peer_up",
			json: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"established"}}`,
			want: "---- STATE  10.0.0.1 AS65001 established",
		},
		{
			name: "peer_down_with_reason",
			json: `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"state":"down","reason":"notification"}}`,
			want: "---- STATE  10.0.0.1 AS65001 down (notification)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMonitorLine(tt.json)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFormatMonitorLineOther verifies other event type text rendering.
//
// VALIDATES: OPEN, NOTIFICATION, KEEPALIVE, EOR, REFRESH, NEGOTIATED events format correctly.
// PREVENTS: Non-UPDATE/STATE events crashing or showing wrong format.
func TestFormatMonitorLineOther(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "keepalive",
			json: `{"type":"bgp","bgp":{"message":{"direction":"received","type":"keepalive"},"peer":{"address":"10.0.0.1","remote":{"as":65001}}}}`,
			want: "recv KALIVE 10.0.0.1 AS65001",
		},
		{
			name: "eor",
			json: `{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"eor":{"family":"ipv4/unicast"}}}`,
			want: "---- EOR    10.0.0.1 AS65001 ipv4/unicast",
		},
		{
			name: "open",
			json: `{"type":"bgp","bgp":{"message":{"direction":"received","type":"open"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"open":{"timer":{"hold-time":90},"router-id":"1.2.3.4"}}}`,
			want: "recv OPEN   10.0.0.1 AS65001 hold=90 id=1.2.3.4",
		},
		{
			name: "notification",
			json: `{"type":"bgp","bgp":{"message":{"direction":"received","type":"notification"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"notification":{"code":6,"subcode":4}}}`,
			want: "recv NOTIF  10.0.0.1 AS65001 6/4",
		},
		{
			name: "refresh",
			json: `{"type":"bgp","bgp":{"message":{"direction":"received","type":"refresh"},"peer":{"address":"10.0.0.1","remote":{"as":65001}}}}`,
			want: "recv RFRSH  10.0.0.1 AS65001",
		},
		{
			name: "negotiated",
			json: `{"type":"bgp","bgp":{"message":{"type":"negotiated"},"peer":{"address":"10.0.0.1","remote":{"as":65001}}}}`,
			want: "---- NEGOT  10.0.0.1 AS65001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMonitorLine(tt.json)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFormatMonitorLineInvalidJSON verifies graceful handling of bad JSON.
//
// VALIDATES: Invalid JSON returns the raw string as fallback.
// PREVENTS: Panic or empty output on malformed events.
func TestFormatMonitorLineInvalidJSON(t *testing.T) {
	raw := `not json at all`
	got := FormatMonitorLine(raw)
	assert.Equal(t, raw, got)
}

// TestFormatMonitorLineManyPrefixes verifies truncation for large updates.
//
// VALIDATES: More than 5 prefixes shows count summary.
// PREVENTS: Extremely long output lines from large UPDATEs.
func TestFormatMonitorLineManyPrefixes(t *testing.T) {
	json := `{"type":"bgp","bgp":{"message":{"type":"update","direction":"received"},"peer":{"address":"10.0.0.1","remote":{"as":65001}},"update":{"nlri":{"ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.0.0/24","10.0.1.0/24","10.0.2.0/24","10.0.3.0/24","10.0.4.0/24","10.0.5.0/24","10.0.6.0/24","10.0.7.0/24"]}]}}}}`
	got := FormatMonitorLine(json)
	assert.Contains(t, got, "+10.0.0.0/24")
	assert.Contains(t, got, "(+3 more)")
	assert.Contains(t, got, "nhop=10.0.1.254")
}
