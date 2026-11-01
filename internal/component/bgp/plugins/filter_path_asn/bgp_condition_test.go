package filter_path_asn

import (
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const testAttachmentBGP = "filter-path-asn-test bgp import"

func init() {
	policy.MustRegisterAttachment("bgp", testAttachmentBGP, []policy.Field{policy.FieldASPath}, nil)
}

// conditionListConfig is one reject-asn list: 3356 anywhere past the sender,
// 64666 as the sender, and one whole-path pattern.
func conditionListConfig() map[string]any {
	return map[string]any{
		"indirect": []any{"3356"},
		"direct":   []any{"64666"},
		"regex":    []any{"^65001 64512$"},
	}
}

// pathUpdate builds an UPDATE whose AS_PATH is one AS_SEQUENCE of path (none
// when path is empty), with one IPv4 prefix.
func pathUpdate(t *testing.T, path ...uint32) *wireu.WireUpdate {
	t.Helper()
	var attrs []byte
	if len(path) > 0 {
		segment := []byte{2, byte(len(path))}
		for _, a := range path {
			segment = binary.BigEndian.AppendUint32(segment, a)
		}
		attrs = append([]byte{0x40, 2, byte(len(segment))}, segment...)
	}
	payload := binary.BigEndian.AppendUint16(nil, 0)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs))) //nolint:gosec // G115: a test attribute block
	payload = append(payload, attrs...)
	payload = append(payload, 24, 10, 1, 0)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}

func rejectASNChain(t *testing.T) *policychain.Chain {
	t.Helper()
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "guard", Type: filterTypeName, Config: conditionListConfig()},
		policy.Instance{Name: "m", Type: policy.MatchFilterType, Config: map[string]any{filterTypeName: "guard"}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

// pathText is the filter text format's as-path for path, as
// (*attribute.ASPath).AppendText writes it.
func pathText(path []uint32) string {
	var b []byte
	for i, a := range path {
		if i > 0 {
			b = append(b, ' ')
		}
		b = strconv.AppendUint(b, uint64(a), 10)
	}
	switch len(path) {
	case 0:
		return ""
	case 1:
		return "as-path " + string(b)
	default:
		return "as-path [" + string(b) + "]"
	}
}

// TestBGPMatchRejectASNParity proves the reject-asn condition at a BGP chain
// gives the verdict the reject-asn filter gives over the same route's text
// form, on import (sender known) and export (no sender). Method: each case
// runs the old handler with the text and the direction, and the chain over
// the wire UPDATE with the matching view session.
func TestBGPMatchRejectASNParity(t *testing.T) {
	list, err := parseOneList("guard", conditionListConfig())
	if err != nil {
		t.Fatal(err)
	}
	old := installedRejectASN.Load()
	t.Cleanup(func() { installedRejectASN.Store(old) })
	installedRejectASN.Store(&rejectASNConfig{lists: map[string]*rejectList{"guard": list}})

	cases := []struct {
		name      string
		direction string
		sender    uint32
		path      []uint32
		action    sdk.FilterAction
	}{
		{"clean path", directionImport, 65001, []uint32{65001, 65002}, sdk.FilterAccept},
		{"transit 3356", directionImport, 65001, []uint32{65001, 3356, 64999}, sdk.FilterReject},
		{"origin 3356", directionImport, 65001, []uint32{65001, 3356}, sdk.FilterReject},
		{"3356 as the sender", directionImport, 3356, []uint32{3356, 65002}, sdk.FilterAccept},
		{"3356 on export", directionExport, 0, []uint32{3356, 65002}, sdk.FilterReject},
		{"direct 64666 on import", directionImport, 64666, []uint32{64666, 65002}, sdk.FilterReject},
		{"64666 on export is not direct", directionExport, 0, []uint32{64666, 65002}, sdk.FilterAccept},
		{"pattern", directionImport, 65001, []uint32{65001, 64512}, sdk.FilterReject},
		{"no as path", directionImport, 65001, nil, sdk.FilterAccept},
	}
	chain := rejectASNChain(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := handleFilterUpdate(&sdk.FilterUpdateInput{Filter: "guard", Direction: tc.direction, PeerAS: tc.sender,
				Update: pathText(tc.path) + " nlri add 10.1.0.0/24"})
			if got.Action != tc.action {
				t.Fatalf("text path answered %v, want %v", got.Action, tc.action)
			}
			session := policyview.Session{EBGP: true}
			if tc.direction == directionImport {
				session.Sender, session.SenderKnown = tc.sender, true
			}
			var view policyview.View
			view.Reset(pathUpdate(t, tc.path...), session)
			defer view.Clear()
			outcome := chain.Run(&view, new(policychain.RunSets))
			if outcome.Failed {
				t.Fatalf("chain failed the route: %+v", outcome)
			}
			want := policychain.OutcomeAccept
			if tc.action == sdk.FilterReject {
				want = policychain.OutcomeReject
			}
			if outcome.Action != want {
				t.Fatalf("BGP chain answered %+v, text answered %v", outcome, tc.action)
			}
		})
	}
}
