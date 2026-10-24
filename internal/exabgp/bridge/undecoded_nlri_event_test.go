// Design: docs/architecture/exabgp-bridge.md -- an NLRI no decoder could read
// Related: bridge_event.go -- convertUpdateIPC2, undecodedNLRI
// Related: bridge_event_text.go -- appendUpdateOperation

package bridge

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUndecodedNLRIElementIsNotRendered feeds an operation whose nlri list holds
// one route and one element in the error convention (an NLRI the family's
// decoder refused) to both encoders.
//
// VALIDATES: the route reaches the script in ExaBGP JSON (API 4 and 6) and text;
// the undecoded element does not, for an announce and a withdraw.
// PREVENTS: the undecoded element reaching a script as a route object.
func TestUndecodedNLRIElementIsNotRendered(t *testing.T) {
	t.Parallel()

	undecoded := `{"error":"synthetic: truncated route","parsed":false,"raw":"DEADBEEF"}`
	cases := []struct {
		name      string
		operation string
		verb      string
	}{
		{"announce", `{"next-hop":"2001:db8::1","action":"add","nlri":["2001:db8:1::/48",` + undecoded + `]}`, "announced"},
		{"withdraw", `{"action":"del","nlri":["2001:db8:1::/48",` + undecoded + `]}`, "withdrawn"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			event := splitErrorEvent(t, tc.operation)
			for _, format := range []Format{FormatV4JSON, FormatV6JSON} {
				encoded, err := json.Marshal(event.exabgpJSON(format))
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				text := string(encoded)
				if !strings.Contains(text, "2001:db8:1::/48") {
					t.Errorf("format %d: the decoded route is missing:\n%s", format, text)
				}
				if strings.Contains(text, "DEADBEEF") || strings.Contains(text, "synthetic") || strings.Contains(text, "parsed") {
					t.Errorf("format %d: an undecoded NLRI reached the script:\n%s", format, text)
				}
			}
			text, form := event.AppendText(nil)
			if form != TextWritten {
				t.Fatalf("text form %d, want TextWritten", form)
			}
			if got := strings.Count(string(text), tc.verb); got != 1 {
				t.Errorf("want one %s line for the one decoded route, got %d:\n%s", tc.verb, got, text)
			}
		})
	}
}
