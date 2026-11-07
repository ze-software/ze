// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- final result admission.

package pppoeclient

import (
	"errors"
	"net/netip"
	"reflect"
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

// TestConnectedResultCarriesEveryNegotiatedValue guards the hand-written copy
// from the negotiated state into Dial's result. The copy once dropped the
// RFC 1877 servers, so a session IPCP gave DNS servers came up with none and
// Ze wrote no resolv.conf. Method: every field of the state is set, which the
// first loop enforces so a new field fails here until it is set, and the
// second loop requires each to reach the result's field of the same name.
func TestConnectedResultCarriesEveryNegotiatedValue(t *testing.T) {
	state := negotiatedSession{
		localIP:      netip.MustParseAddr("192.0.2.2"),
		peerIP:       netip.MustParseAddr("192.0.2.1"),
		primaryDNS:   netip.MustParseAddr("192.0.2.53"),
		secondaryDNS: netip.MustParseAddr("192.0.2.54"),
		negMTU:       1492,
		magic:        0x10203040,
		peerMRU:      1492,
		chap:         &networkPhaseCHAP{id: 7},
	}
	reader := &frameReader{}
	result := connectedResult(state, reader, 1, 2, 3, 4)
	source := reflect.ValueOf(state)
	target := reflect.ValueOf(result)
	for i := range source.NumField() {
		name := source.Type().Field(i).Name
		if source.Field(i).IsZero() {
			t.Fatalf("negotiatedSession.%s is unset in this test: set it so its copy is checked", name)
		}
		field := target.FieldByName(name)
		if !field.IsValid() {
			t.Fatalf("sessionResult has no %s field to carry negotiatedSession.%s", name, name)
		}
		if !field.Equal(source.Field(i)) {
			t.Fatalf("sessionResult.%s = %v, want negotiatedSession.%s = %v", name, field, name, source.Field(i))
		}
	}
	if result.reader != reader {
		t.Fatal("result lost the receive owner")
	}
	ids := [4]uint8{result.ipcpCodeRejectID, result.ipcpIdentifier, result.lcpIdentifier, result.papIdentifier}
	if ids != [4]uint8{1, 2, 3, 4} {
		t.Fatalf("identifiers = %v, want [1 2 3 4] in code-reject, IPCP, LCP, PAP order", ids)
	}
}
