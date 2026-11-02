package role

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestOTCIsAViewField proves bgp-role registers otc as a field of the BGP
// route view: attribute code 35, owned by this plugin so only its filters may
// set it, read as the four-octet ASN and written as a JSON number, absent
// when the UPDATE carries no OTC, and malformed at another length (RFC 9234
// Section 5: "The OTC Attribute is considered malformed if the length value
// is not 4."). Method: a view over hand-built UPDATE bodies.
//
// VALIDATES: spec-filter-wire-6-stage-filters D-8 (otc).
// PREVENTS: otc hand-listed by the engine, or settable by any plugin.
func TestOTCIsAViewField(t *testing.T) {
	otc, ok := policyview.AttributeNumberNamed("otc")
	if !ok {
		t.Fatal("otc is not a registered attribute number")
	}
	if otc.Code() != 35 || otc.Owner() != rolePluginName {
		t.Fatalf("otc = code %d owner %q, want code 35 owner %q", otc.Code(), otc.Owner(), rolePluginName)
	}
	ctxID, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	// ORIGIN IGP, then OTC (optional transitive, code 35) of the given value.
	body := func(otcValue ...byte) []byte {
		attrs := []byte{0x40, 1, 1, 0}
		if otcValue != nil {
			attrs = append(attrs, 0xC0, 35, byte(len(otcValue)))
			attrs = append(attrs, otcValue...)
		}
		return append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
	}
	var view policyview.View
	defer view.Clear()

	view.Reset(wireu.NewWireUpdate(body(0, 0, 0xfd, 0xe9), ctxID), policyview.Session{})
	got, err := policyview.AppendFields(nil, &view, []string{"otc"})
	if err != nil || string(got) != `{"otc":65001}` {
		t.Errorf("otc 65001: fields %s, err %v; want {\"otc\":65001}", got, err)
	}

	view.Reset(wireu.NewWireUpdate(body(), ctxID), policyview.Session{})
	got, err = policyview.AppendFields(nil, &view, []string{"otc"})
	if err != nil || string(got) != `{}` {
		t.Errorf("no OTC: fields %s, err %v; want {}", got, err)
	}

	view.Reset(wireu.NewWireUpdate(body(0, 0xfd, 0xe9), ctxID), policyview.Session{})
	if _, err = policyview.AppendFields(nil, &view, []string{"otc"}); !errors.Is(err, policyview.ErrMalformed) {
		t.Errorf("three-octet OTC answered %v, want ErrMalformed", err)
	}
}
