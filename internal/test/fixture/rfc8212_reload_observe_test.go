// Design: docs/functional-tests.md -- fixture counters must distinguish zero from absent data.
// RFC naming: untagged -- tests RFC8212 draft observation decoding, not protocol conformance.
package fixture

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRFC8212ReloadCounter checks the real detail JSON shape and refuses data
// that an untyped numeric default would silently read as a successful zero.
func TestRFC8212ReloadCounter(t *testing.T) {
	for _, tc := range []struct {
		name      string
		row       string
		direction string
		kind      string
		want      int64
		err       string
	}{
		{name: "zero", row: `{"127.0.0.1":{"messages":{"received":{"updates":0}}}}`, direction: "received", kind: "updates"},
		{name: "updates", row: `{"127.0.0.1":{"messages":{"received":{"updates":2}}}}`, direction: "received", kind: "updates", want: 2},
		{name: "refresh", row: `{"127.0.0.1":{"messages":{"sent":{"route-refresh":1}}}}`, direction: "sent", kind: "route-refresh", want: 1},
		{name: "maximum", row: `{"127.0.0.1":{"messages":{"received":{"updates":4294967295}}}}`, direction: "received", kind: "updates", want: 4294967295},
		{name: "wrong peer", row: `{"127.0.0.3":{"messages":{"received":{"updates":2}}}}`, direction: "received", kind: "updates", err: "peer 127.0.0.1 absent"},
		{name: "missing messages", row: `{"127.0.0.1":{"updates-received":2}}`, direction: "received", kind: "updates", err: "omitted messages object"},
		{name: "missing direction", row: `{"127.0.0.1":{"messages":{"sent":{"updates":2}}}}`, direction: "received", kind: "updates", err: "omitted messages.received object"},
		{name: "missing updates", row: `{"127.0.0.1":{"messages":{"received":{"eor":0}}}}`, direction: "received", kind: "updates", err: "omitted messages.received.updates counter"},
		{name: "missing refresh", row: `{"127.0.0.1":{"messages":{"sent":{"updates":2}}}}`, direction: "sent", kind: "route-refresh", err: "omitted messages.sent.route-refresh counter"},
		{name: "null", row: `{"127.0.0.1":{"messages":{"received":{"updates":null}}}}`, direction: "received", kind: "updates", err: "want uint32"},
		{name: "string", row: `{"127.0.0.1":{"messages":{"received":{"updates":"2"}}}}`, direction: "received", kind: "updates", err: "want uint32"},
		{name: "negative", row: `{"127.0.0.1":{"messages":{"received":{"updates":-1}}}}`, direction: "received", kind: "updates", err: "is negative"},
		{name: "fraction", row: `{"127.0.0.1":{"messages":{"received":{"updates":1.5}}}}`, direction: "received", kind: "updates", err: "is not an integer"},
		{name: "overflow", row: `{"127.0.0.1":{"messages":{"received":{"updates":4294967296}}}}`, direction: "received", kind: "updates", err: "exceeds uint32"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var rows map[string]map[string]any
			if err := json.Unmarshal([]byte(tc.row), &rows); err != nil {
				t.Fatal(err)
			}
			got, err := rfc8212ReloadCounter(rows, "127.0.0.1", tc.direction, tc.kind)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("counter=%d without required error %q", got, tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("error=%q, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("counter=%d, want %d", got, tc.want)
			}
		})
	}
}
