// Design: docs/architecture/l2tp/subscriber-session-model.md -- canonical subscriber identity
// Related: id.go -- shared formatter for every access transport

package subscriber

import "testing"

// TestSessionID preserves the existing names and proves that neither access
// namespace nor the full transport identifier is lost by the shared formatter.
func TestSessionID(t *testing.T) {
	cases := []struct {
		name      string
		access    AccessType
		transport uint64
		session   uint16
		want      string
	}{
		{"l2tp", AccessL2TP, 7, 9, "l2tp-7-9"},
		{"pppoe", AccessPPPoE, 7, 9, "pppoe-7-9"},
		{"pppoe-full-index", AccessPPPoE, 65543, 9, "pppoe-65543-9"},
		{"maximum-identifiers", AccessL2TP, ^uint64(0), ^uint16(0), "l2tp-18446744073709551615-65535"},
		{"open-namespace", AccessType("ipoe"), 7, 9, "ipoe-7-9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SessionID(tc.access, tc.transport, tc.session); got != tc.want {
				t.Fatalf("subscriber ID = %q, want %q", got, tc.want)
			}
		})
	}
}
