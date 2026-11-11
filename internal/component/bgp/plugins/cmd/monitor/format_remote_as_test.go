package monitor

import (
	"net/netip"
	"testing"

	bgpformat "github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFormatMonitorLineRemoteAS preserves the native producer's distinction
// between a dynamic peer before negotiation and a peer whose AS is known.
func TestFormatMonitorLineRemoteAS(t *testing.T) {
	for _, remote := range []struct {
		name string
		asn  rpc.RemoteAS
		want string
	}{
		{name: "before-negotiation", want: "AS unknown"},
		{name: "known", asn: rpc.RemoteASOf(65001), want: "AS65001"},
	} {
		t.Run(remote.name, func(t *testing.T) {
			peer := &plugin.PeerInfo{
				Address: netip.MustParseAddr("10.0.0.1"),
				LocalAS: 65000,
				PeerAS:  remote.asn,
			}
			encoder := bgpformat.NewJSONEncoder("")
			for _, event := range []struct {
				name string
				raw  string
				want string
			}{
				{
					name: "sent-open",
					raw: encoder.Open(peer, bgpformat.DecodedOpen{
						ASN: 65000, HoldTime: 90, RouterID: "1.2.3.4",
					}, rpc.DirectionSent, 1),
					want: "sent OPEN   10.0.0.1 " + remote.want + " hold=90 id=1.2.3.4",
				},
				{
					name: "state-down",
					raw: string(bgpformat.AppendStateChange(nil, peer, rpc.SessionStateDown,
						rpc.ReasonNotification, nil, plugin.EncodingJSON)),
					want: "---- STATE  10.0.0.1 " + remote.want + " down (notification)",
				},
				{
					name: "sent-notification",
					raw: encoder.Notification(peer, bgpformat.DecodedNotification{
						ErrorCode: 6, ErrorSubcode: 4,
					}, rpc.DirectionSent, 2),
					want: "sent NOTIF  10.0.0.1 " + remote.want + " 6/4",
				},
			} {
				t.Run(event.name, func(t *testing.T) {
					if got := formatMonitorLine(event.raw); got != event.want {
						t.Fatalf("formatMonitorLine(%s) = %q, want %q", event.raw, got, event.want)
					}
				})
			}
		})
	}
}

func TestFormatMonitorLineRemoteASNull(t *testing.T) {
	raw := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":null}},"state":"down","reason":"notification"}}`
	want := "---- STATE  10.0.0.1 AS unknown down (notification)"
	if got := formatMonitorLine(raw); got != want {
		t.Fatalf("formatMonitorLine(%s) = %q, want %q", raw, got, want)
	}
}

// Invalid AS values retain the entire event through the raw fallback.
func TestFormatMonitorLineRemoteASInvalid(t *testing.T) {
	for _, asn := range []string{"0", "-1", "4294967296", "1.2", `"65001"`, "true", "{}"} {
		t.Run(asn, func(t *testing.T) {
			raw := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":` + asn + `}},"state":"down","reason":"notification"}}`
			if got := formatMonitorLine(raw); got != raw {
				t.Fatalf("invalid remote AS changed event to %q, want raw %q", got, raw)
			}
		})
	}
}
