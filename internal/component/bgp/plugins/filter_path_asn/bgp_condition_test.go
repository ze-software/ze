package filter_path_asn

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	testAttachmentBGP    = "filter-path-asn-test bgp import"
	testAttachmentPrefix = "filter-path-asn-test bgp prefix import"
)

func init() {
	policy.MustRegisterAttachment("bgp", testAttachmentBGP, []policy.Field{policy.FieldASPath}, nil)
	policy.MustRegisterAttachment("bgp", testAttachmentPrefix, []policy.Field{policy.FieldASPath, policy.FieldPrefix}, nil)
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
	if len(path) == 0 {
		return segmentsUpdate(t)
	}
	return segmentsUpdate(t, attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: path})
}

// segmentsUpdate builds an UPDATE whose AS_PATH carries segments as given (no
// AS_PATH when segments is empty), with one IPv4 prefix.
func segmentsUpdate(tb testing.TB, segments ...attribute.ASPathSegment) *wireu.WireUpdate {
	tb.Helper()
	var value []byte
	for _, segment := range segments {
		value = append(value, byte(segment.Type), byte(len(segment.ASNs))) //nolint:gosec // G115: a test segment holds at most 255 ASNs
		for _, a := range segment.ASNs {
			value = binary.BigEndian.AppendUint32(value, a)
		}
	}
	var attrs []byte
	if len(segments) > 0 {
		// Extended Length (0x10) with Transitive (0x40): a path of two full
		// segments outgrows a one-octet attribute length.
		attrs = append([]byte{0x50, 2}, 0, 0)
		binary.BigEndian.PutUint16(attrs[2:], uint16(len(value))) //nolint:gosec // G115: a test attribute value
		attrs = append(attrs, value...)
	}
	payload := binary.BigEndian.AppendUint16(nil, 0)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(attrs))) //nolint:gosec // G115: a test attribute block
	payload = append(payload, attrs...)
	payload = append(payload, 24, 10, 1, 0)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		tb.Fatalf("register context: %v", err)
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

// TestBGPMatchRejectASNParity proves the reject-asn condition at a BGP chain
// gives the verdict the reject-asn filter gave, on import (sender known) and
// export (no sender). Method: each case runs the chain over the wire UPDATE
// with the matching view session against the verdict the filter answered for
// the same route, fixed in the table.
func TestBGPMatchRejectASNParity(t *testing.T) {

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
				t.Fatalf("BGP chain answered %+v, want %v", outcome, tc.action)
			}
		})
	}
}
