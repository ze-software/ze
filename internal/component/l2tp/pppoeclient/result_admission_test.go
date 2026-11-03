// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- final result admission.

package pppoeclient

import (
	"errors"
	"net/netip"
	"strconv"
	"testing"
)

// TestNegotiatedResultMinimum isolates result admission from config admission
// and wire policy. A valid config and addresses reach the negotiated-MTU guard.
// The minimum must still hold if an internal producer supplies an invalid result.
func TestNegotiatedResultMinimum(t *testing.T) {
	for _, mtu := range []uint16{66, 68} {
		t.Run(strconv.Itoa(int(mtu)), func(t *testing.T) {
			cfg := sessionConfig{mtu: 1492}
			raw := sessionResult{
				localIP: netip.MustParseAddr("192.0.2.2"),
				peerIP:  netip.MustParseAddr("192.0.2.1"),
				negMTU:  mtu, peerMRU: mtu,
			}
			result, err := newNegotiatedSession(raw, cfg)
			if mtu == 66 {
				if !errors.Is(err, errSessionMTURange) {
					t.Fatalf("result admission = %v, want negotiated MTU refusal", err)
				}
				if result != (negotiatedSession{}) {
					t.Fatalf("refused result is publishable: %+v", result)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.negMTU != 68 {
				t.Fatalf("admitted MTU = %d, want 68", result.negMTU)
			}
			if result.localIP != raw.localIP {
				t.Fatal("admission changed the local address")
			}
			if result.peerIP != raw.peerIP {
				t.Fatal("admission changed the peer address")
			}
		})
	}
}
