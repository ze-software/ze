package policychain

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// labeledAddPathReach is an MP_REACH_NLRI of IPv4 labeled unicast received
// under ADD-PATH: two paths of 192.0.2.0/24 with label 100, path identifiers
// 1 and 2, each entry [path-id(4)][length(1)][label(3)][prefix(3)].
func labeledAddPathReach() []byte {
	value := []byte{0, 1, byte(family.SAFIMPLSLabel), 4, 192, 0, 2, 254, 0}
	value = append(value,
		0, 0, 0, 1, 48, 0x00, 0x06, 0x41, 192, 0, 2, // path 1, label 100, S set
		0, 0, 0, 2, 48, 0x00, 0x06, 0x41, 192, 0, 2) // path 2, label 100, S set
	return pathAttr(0x80, attribute.AttrMPReachNLRI, value)
}

// updateAddPath builds an UPDATE whose source context negotiated ADD-PATH
// receive for fam only.
func updateAddPath(t *testing.T, fam family.Family, attrs ...[]byte) *wireu.WireUpdate {
	t.Helper()
	section := bytes.Join(attrs, nil)
	payload := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(section))) //nolint:gosec // G115: a test attribute section, far below 65535 octets
	payload = append(payload, section...)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true, AddPathMode: map[capability.Family]capability.AddPathMode{
			fam: capability.AddPathBoth,
		}}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	return wireu.NewWireUpdate(payload, id)
}

// TestExternalPrefixRemoveLabeledAddPath proves a prefix remove names one
// path of a labeled-unicast route received under ADD-PATH: the value the
// UPDATE event writes for it ({"labels":...,"path-id":N,"prefix":...})
// drops only the entry carrying that Path Identifier. An identifier the
// UPDATE does not carry, and the labeled shape with no identifier on a
// session that carries one, name no entry and fail the route.
func TestExternalPrefixRemoveLabeledAddPath(t *testing.T) {
	family.RegisterTestFamilies()
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	cases := []struct {
		name  string
		value string
		drops []uint16
	}{
		{"path 1", `{"labels":[[100,1601]],"path-id":1,"prefix":"192.0.2.0/24"}`, []uint16{0}},
		{"path 2", `{"labels":[[100,1601]],"path-id":2,"prefix":"192.0.2.0/24"}`, []uint16{11}},
		{"absent path", `{"labels":[[100,1601]],"path-id":3,"prefix":"192.0.2.0/24"}`, nil},
		{"no path-id", `{"labels":[[100,1601]],"prefix":"192.0.2.0/24"}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{"ext:f": {
				decl: ExternalDeclaration{Reads: []string{"prefix"}, Sets: []string{"as-path", "prefix", "family"},
					OnError: rpc.OnErrorReject},
				answer: modify(set("prefix", rpc.FilterSetActionRemove, tc.value)),
			}}}
			chain := compileMembers(t, testAttachmentIntents, []Member{{Name: "f", Plugin: "ext"}})
			var view policyview.View
			sets := new(RunSets)
			sets.SetRoute(ExternalRoute{Caller: caller, Direction: "import", Peer: "192.0.2.1", PeerAS: 65001})
			view.Reset(updateAddPath(t, fam, labeledAddPathReach()), policyview.Session{})
			defer view.Clear()
			outcome := chain.Run(&view, sets)
			if tc.drops == nil {
				want := Outcome{Action: OutcomeReject, Failed: true, Member: "ext:f"}
				if outcome != want || chain.Refused() != 1 {
					t.Fatalf("outcome = %+v refused %d, want %+v refused 1", outcome, chain.Refused(), want)
				}
				return
			}
			if outcome.Action != OutcomeAccept || chain.Refused() != 0 {
				t.Fatalf("outcome = %+v refused %d, want accept", outcome, chain.Refused())
			}
			if got := sets.Folded().NLRIDrops(NLRISectionMPReach); !slices.Equal(got, tc.drops) {
				t.Fatalf("MP_REACH_NLRI drops = %v, want %v", got, tc.drops)
			}
		})
	}
}
