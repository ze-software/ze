// Design: docs/architecture/bgp/structural-forwarding.md -- raw mixed export and classic presence.
// Related: classic_mixed_next_hop.go -- real daemon filter and driver.
package fixture

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// The raw policy retains mandatory source attributes while deliberately omitting
// code 3; only the middle generation includes classic NLRI beside FlowSpec.
func TestClassicMixedNextHopRawBody(t *testing.T) {
	for phase := byte(1); phase <= 3; phase++ {
		t.Run(string(rune('0'+phase)), func(t *testing.T) {
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 1}
			original := bytes.Clone(attrs)
			body := classicMixedNextHopBody(attrs, phase)
			update, err := message.UnpackUpdate(body)
			if err != nil {
				t.Fatal(err)
			}
			for _, code := range []attribute.AttributeCode{attribute.AttrOrigin, attribute.AttrASPath} {
				_, _, _, found := attribute.AttrFind(update.PathAttributes, code)
				if !found {
					t.Fatalf("mandatory attribute %d missing", code)
				}
			}
			_, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
			if found {
				t.Fatal("the test policy must omit code 3")
			}
			_, _, flow, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			if !found {
				t.Fatal("FlowSpec sibling missing")
			}
			if !bytes.Equal(flow, []byte{0, 1, 133, 0, 0, 5, 1, 24, 198, 51, phase}) {
				t.Fatalf("FlowSpec = %x", flow)
			}
			var want []byte
			if phase == 2 {
				want = classicMixedNextHopPrefix[:]
			}
			if !bytes.Equal(update.NLRI, want) {
				t.Fatalf("classic NLRI = %x, want %x", update.NLRI, want)
			}
			if !bytes.Equal(attrs, original) {
				t.Fatal("raw policy mutated source attributes")
			}
		})
	}
}

// The receiver oracle catches a forbidden classic announcement, invented
// withdrawal, missing survivor, altered FlowSpec bytes and an incorrect repair.
func TestClassicMixedNextHopWireOracle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		useful    bool
		change    func(*message.Update)
		wantError bool
	}{
		{name: "harmful keeps exact FlowSpec"},
		{name: "useful repairs both fields", useful: true},
		{name: "harmful classic escape", change: func(u *message.Update) { u.NLRI = classicMixedNextHopPrefix[:] }, wantError: true},
		{name: "fresh classic withdrawal", change: func(u *message.Update) { u.WithdrawnRoutes = classicMixedNextHopPrefix[:] }, wantError: true},
		{name: "missing FlowSpec", change: func(u *message.Update) { u.PathAttributes = nil }, wantError: true},
		{name: "altered FlowSpec", change: func(u *message.Update) { u.PathAttributes[len(u.PathAttributes)-1] ^= 1 }, wantError: true},
		{name: "useful wrong next hop", useful: true, change: func(u *message.Update) {
			_, _, hop, _ := attribute.AttrFind(u.PathAttributes, attribute.AttrNextHop)
			hop[3] = 3
		}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 4, 2, 1, 0xfd, 0xe9}
			body := classicMixedNextHopBody(attrs, 2)
			update, err := message.UnpackUpdate(body)
			if err != nil {
				t.Fatal(err)
			}
			update.NLRI = nil
			if tc.useful {
				update.NLRI = classicMixedNextHopPrefix[:]
				update.PathAttributes = append(update.PathAttributes, 0x40, 3, 4, 192, 0, 2, 99)
			}
			if tc.change != nil {
				tc.change(update)
			}
			var state classicMixedNextHopWireState
			err = state.observe(update, 2, tc.useful)
			if err == nil {
				err = state.complete(2, tc.useful)
			}
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, want error %t", err, tc.wantError)
			}
		})
	}
}
