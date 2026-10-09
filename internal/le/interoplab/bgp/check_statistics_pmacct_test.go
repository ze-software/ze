package bgp

import (
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// statisticsPMACCTLine is one Statistics Report as pmacct/pmbmpd:latest printed
// it on 2026-10-09 after decoding ze's bytes, with the lab's base addresses.
const statisticsPMACCTLine = `{"event_type": "log", "seq": 6, "timestamp": "2026-10-09 13:19:01.000000", ` +
	`"timestamp_arrival": "2026-10-09 13:19:01.165047", "bmp_router": "172.30.0.2", "bmp_router_port": 36290, ` +
	`"bmp_msg_type": "stats", "peer_ip": "172.30.0.3", "peer_asn": 65045, "peer_type": 0, ` +
	`"bmp_rib_type": "Adj-Rib-In Pre-Policy", "is_filtered": 0, "is_post": 0, "is_in": 1, "bgp_id": "172.30.0.3", ` +
	`"counter_type": 13, "counter_type_str": "Number of duplicate update messages received", "counter_value": 0}`

// TestStatisticsPMACCTCheckerReadsOneReportInAnyFieldOrder runs the scenario's
// own first collector operation, rewritten onto a non-base lab network the way
// the engine rewrites it, against msglogs written to a temp file. The report
// pmacct prints today and the shorter 2026-09-06 field order must both pass;
// a log holding no Statistics Report, a report with the RFC 8671 O flag set,
// and a report attributed to another peer must not.
func TestStatisticsPMACCTCheckerReadsOneReportInAnyFieldOrder(t *testing.T) {
	t.Parallel()
	operation := scenarioExtras[scenarioStatisticsPMACCT][0]
	rewriteOperation(interoplab.Network{IPv4: netip.MustParsePrefix("172.30.1.0/24")}, &operation)
	rewritten := strings.NewReplacer("172.30.0.", "172.30.1.")
	older := `{"bmp_msg_type": "stats", "peer_ip": "172.30.0.3", "peer_asn": 65045, "peer_type": 0, ` +
		`"is_post": 0, "is_in": 1, "counter_type": 13, ` +
		`"counter_type_str": "Number of duplicate update messages received"}`
	tests := []struct {
		name string
		log  string
		want bool
	}{
		{name: "current pmacct field order", log: statisticsPMACCTLine, want: true},
		{name: "2026-09-06 pmacct field order", log: older, want: true},
		{name: "no statistics report", log: strings.Replace(statisticsPMACCTLine, `"stats"`, `"route_monitor"`, 1), want: false},
		{name: "O flag set", log: strings.Replace(statisticsPMACCTLine, `"is_post": 0, "is_in": 1`, `"is_post": 1, "is_out": 1`, 1), want: false},
		{name: "another peer", log: strings.Replace(statisticsPMACCTLine, `"peer_ip": "172.30.0.3"`, `"peer_ip": "172.30.0.5"`, 1), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			logPath := filepath.Join(t.TempDir(), "bmp.log")
			if err := os.WriteFile(logPath, []byte(rewritten.Replace(test.log)+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			script := strings.ReplaceAll(operation.command[2], pmacctMsgLogPath, strconv.Quote(logPath))
			output, err := exec.CommandContext(t.Context(), operation.command[0], operation.command[1], script).CombinedOutput()
			if err != nil {
				t.Fatalf("collector query: %v: %s", err, output)
			}
			if got := containsAll(string(output), operation.contains); got != test.want {
				t.Fatalf("matched=%v, want %v; output=%q", got, test.want, output)
			}
		})
	}
}
