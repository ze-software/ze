// Design: docs/architecture/core-design.md -- Policy Filter Chain, built-in next-hop steps and intent folding
// Related: nexthop.go -- NextHopMode, NextHopIntent, BuiltinNextHopMode, StaticNextHop
// Related: sets_attributes.go -- Sets.NextHop, Sets.NextHopMode, Sets.NextHopIntent

package policychain

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	testAttachmentNextHopImport = "policychain-test-nexthop import"
	testAttachmentNextHopExport = "policychain-test-nexthop export"
)

func init() {
	fields := []policy.Field{policy.FieldNextHop, policy.FieldPrefix, policy.FieldOrigin}
	policy.MustRegisterAttachment("bgp", testAttachmentNextHopImport, fields, fields)
	policy.MustRegisterAttachment("bgp", testAttachmentNextHopExport, fields, fields)
}

// mpReachAttr builds an MP_REACH_NLRI attribute for fam with nh bytes and a
// single minimal NLRI payload.
func mpReachAttr(fam family.Family, nh, nlri []byte) []byte {
	val := make([]byte, 0, 5+len(nh)+len(nlri))
	val = binary.BigEndian.AppendUint16(val, uint16(fam.AFI))
	val = append(val, byte(fam.SAFI), byte(len(nh)))
	val = append(val, nh...)
	val = append(val, 0) // Reserved
	val = append(val, nlri...)
	return pathAttr(0x80, attribute.AttrMPReachNLRI, val)
}

// updateWithBody builds a WireUpdate with explicit legacy body NLRI (may be
// nil for pure MP_REACH UPDATEs) and path attributes.
func updateWithBody(t *testing.T, legacyNLRI []byte, attrs ...[]byte) *wireu.WireUpdate {
	t.Helper()
	section := bytes.Join(attrs, nil)
	payload := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(section)))
	payload = append(payload, section...)
	payload = append(payload, legacyNLRI...)
	id, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	if err != nil {
		t.Fatalf("register context: %v", err)
	}
	var wu wireu.WireUpdate
	wireu.InitWireUpdate(&wu, payload, id)
	return &wu
}

// TestSetsNextHopValidation proves Sets.NextHop accepts unicast IPv4 and
// global IPv6 addresses while refusing link-local, unspecified, multicast,
// zoned, IPv4-mapped IPv6, and invalid addresses, and Sets.NextHopMode accepts
// only auto/self/unchanged with AutoDefault restricted to auto.
//
// VALIDATES: AC-16 address and mode validation on Sets.NextHop and Sets.NextHopMode.
// PREVENTS: invalid IPv6 forms (link-local, mapped, zoned, unspecified, multicast)
// or bogus modes entering the folded next-hop intent.
func TestSetsNextHopValidation(t *testing.T) {
	for _, valid := range []string{"192.0.2.9", "127.0.0.1", "2001:db8::1", "fd00::1"} {
		var sets Sets
		addr := netip.MustParseAddr(valid)
		if err := sets.NextHop(addr); err != nil {
			t.Errorf("Sets.NextHop(%s) failed: %v", valid, err)
			continue
		}
		want := NextHopIntent{Mode: NextHopModeExplicit, Addr: addr}
		if got := sets.NextHopIntent(); got != want {
			t.Errorf("NextHopIntent(%s) = %+v, want %+v", valid, got, want)
		}
	}

	invalidAddrs := []struct {
		name string
		addr netip.Addr
	}{
		{"zero", netip.Addr{}},
		{"ipv4-unspecified", netip.MustParseAddr("0.0.0.0")},
		{"ipv6-unspecified", netip.MustParseAddr("::")},
		{"ipv6-link-local", netip.MustParseAddr("fe80::1")},
		{"ipv6-loopback", netip.MustParseAddr("::1")},
		{"ipv6-zoned-link-local", netip.MustParseAddr("fe80::1%eth0")},
		{"ipv6-zoned-global", netip.MustParseAddr("2001:db8::1%eth0")},
		{"ipv4-multicast", netip.MustParseAddr("224.0.0.1")},
		{"ipv6-multicast", netip.MustParseAddr("ff02::1")},
		{"ipv4-mapped-ipv6", netip.MustParseAddr("::ffff:192.0.2.1")},
	}
	for _, tc := range invalidAddrs {
		var sets Sets
		if err := sets.NextHop(tc.addr); err == nil {
			t.Errorf("Sets.NextHop(%s: %s) succeeded, want refusal", tc.name, tc.addr)
		}
		if got := sets.NextHopIntent(); got.Mode != NextHopModeUnspecified {
			t.Errorf("failed Sets.NextHop(%s) mutated intent to %+v", tc.name, got)
		}
	}

	for _, tc := range []struct {
		mode        NextHopMode
		autoDefault bool
		wantErr     bool
	}{
		{NextHopModeAuto, false, false},
		{NextHopModeAuto, true, false},
		{NextHopModeSelf, false, false},
		{NextHopModeUnchanged, false, false},
		{NextHopModeSelf, true, true},
		{NextHopModeUnchanged, true, true},
		{NextHopModeUnspecified, false, true},
		{NextHopModeExplicit, false, true},
		{NextHopMode(99), false, true},
	} {
		var sets Sets
		err := sets.NextHopMode(tc.mode, tc.autoDefault)
		if (err != nil) != tc.wantErr {
			t.Errorf("NextHopMode(%v, %v) err=%v, wantErr=%v", tc.mode, tc.autoDefault, err, tc.wantErr)
		}
	}

	for _, tc := range []struct {
		mode NextHopMode
		want string
	}{
		{NextHopModeUnspecified, "unspecified"},
		{NextHopModeAuto, "auto"},
		{NextHopModeSelf, "self"},
		{NextHopModeUnchanged, "unchanged"},
		{NextHopModeExplicit, "explicit"},
		{NextHopMode(99), "unknown"},
	} {
		if got := tc.mode.String(); got != tc.want {
			t.Errorf("NextHopMode(%d).String() = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// TestBuiltinNextHopCompileAndImportRefusal verifies BuiltinNextHopMode,
// import-side refusal of built-in next-hop:* steps, AutoDefault validation,
// and zero-RPC execution on export chains.
//
// VALIDATES: AC-16 built-in zero-RPC next-hop:* compilation and import refusal.
// PREVENTS: built-in egress-only next-hop steps compiling on import chains or
// requiring an external RPC caller.
func TestBuiltinNextHopCompileAndImportRefusal(t *testing.T) {
	if !isImportAttachment("bgp import") || isImportAttachment("bgp export") {
		t.Fatal("isImportAttachment misclassified bgp import / bgp export")
	}

	for _, tc := range []struct {
		ref  string
		mode NextHopMode
		ok   bool
	}{
		{BuiltinNextHopAuto, NextHopModeAuto, true},
		{BuiltinNextHopSelf, NextHopModeSelf, true},
		{BuiltinNextHopUnchanged, NextHopModeUnchanged, true},
		{"next-hop:explicit", NextHopModeUnspecified, false},
		{"modify:nh", NextHopModeUnspecified, false},
	} {
		mode, ok := BuiltinNextHopMode(tc.ref)
		if mode != tc.mode || ok != tc.ok {
			t.Errorf("BuiltinNextHopMode(%q) = (%v, %v), want (%v, %v)", tc.ref, mode, ok, tc.mode, tc.ok)
		}
	}

	for _, ref := range []string{BuiltinNextHopAuto, BuiltinNextHopSelf, BuiltinNextHopUnchanged} {
		if _, err := Compile(testAttachmentNextHopImport, nil, []Member{{Name: ref}}, policy.FinalAccept, Bases{}); err == nil {
			t.Errorf("Compile(%q) on import succeeded, want refusal", ref)
		}
	}

	// AutoDefault is valid only on built-in next-hop:auto.
	for _, bad := range []Member{
		{Name: BuiltinNextHopSelf, AutoDefault: true},
		{Name: BuiltinNextHopUnchanged, AutoDefault: true},
		{Name: "auto", Plugin: "next-hop", AutoDefault: true},
	} {
		if _, err := Compile(testAttachmentNextHopExport, nil, []Member{bad}, policy.FinalAccept, Bases{}); err == nil {
			t.Errorf("Compile(%+v) with AutoDefault succeeded, want refusal", bad)
		}
	}

	// Built-in step executes with nil ExternalCaller (zero RPC) and emits no
	// ModAccumulator attr ops (reactor-resolved intent).
	chain, err := Compile(testAttachmentNextHopExport, nil, []Member{{Name: BuiltinNextHopSelf}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatalf("Compile export next-hop:self: %v", err)
	}
	var view policyview.View
	var sets RunSets
	view.Reset(update(t, originIGP()), policyview.Session{})
	defer view.Clear()
	outcome := chain.Run(&view, &sets)
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	wantIntent := NextHopIntent{Mode: NextHopModeSelf}
	if got := sets.Folded().NextHopIntent(); got != wantIntent {
		t.Fatalf("Folded.NextHopIntent() = %+v, want %+v", got, wantIntent)
	}
	var acc filterapi.ModAccumulator
	if err := chain.Apply(&view, &sets, &acc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(acc.Ops()) != 0 {
		t.Fatalf("Apply emitted %+v for reactor-resolved next-hop:self, want no ops", acc.Ops())
	}
}

// TestPolicyChainNextHopIntentAndPrecedence verifies D-3 folding semantics:
// explicit next-hop sets use last-write-wins, while auto-appended trailing
// next-hop:auto acts as fill-if-unset, and external FilterSet refuses
// self/unchanged/auto on import while permitting them on export.
//
// VALIDATES: AC-16, AC-18 folding order, trailing auto fill-if-unset, and
// external FilterSet direction enforcement.
// PREVENTS: trailing next-hop:auto overwriting a conditional modify set, or
// egress-only keywords slipping onto import chains.
func TestPolicyChainNextHopIntentAndPrecedence(t *testing.T) {
	// 1. External FilterSet refuses self/unchanged/auto on import and allows them on export.
	for _, kw := range []string{"self", "unchanged", "auto"} {
		caller := &fakeCaller{filters: map[string]fakeFilter{"ext:nh": {
			decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
			answer: modify(set("next-hop", rpc.FilterSetActionSet, `"`+kw+`"`)),
		}}}
		chain, err := Compile(testAttachmentNextHopExport, nil, []Member{{Name: "nh", Plugin: "ext"}}, policy.FinalAccept, Bases{})
		if err != nil {
			t.Fatal(err)
		}
		var view policyview.View
		sets := new(RunSets)
		sets.SetRoute(ExternalRoute{Caller: caller, Direction: "import", Peer: "192.0.2.1", PeerAS: 65001})
		view.Reset(update(t, originIGP()), policyview.Session{})
		outcome := chain.Run(&view, sets)
		view.Clear()
		if outcome.Action != OutcomeReject || !outcome.Failed {
			t.Fatalf("import %q: outcome = %+v, want failed reject", kw, outcome)
		}
		if len(sets.Failures()) != 1 || !strings.Contains(sets.Failures()[0].Err.Error(), "export chain only") {
			t.Fatalf("import %q: failures = %+v, want error naming export chain only", kw, sets.Failures())
		}

		// Same filter on export succeeds and records the mode intent with zero attr ops.
		sets.SetRoute(ExternalRoute{Caller: caller, Direction: "export", Peer: "192.0.2.1", PeerAS: 65001})
		view.Reset(update(t, originIGP()), policyview.Session{})
		outcome = chain.Run(&view, sets)
		if outcome.Action != OutcomeAccept {
			view.Clear()
			t.Fatalf("export %q: outcome = %+v, want accept", kw, outcome)
		}
		wantMode, _ := nextHopModeKeyword(kw)
		wantIntent := NextHopIntent{Mode: wantMode, AutoDefault: false}
		if got := sets.Folded().NextHopIntent(); got != wantIntent {
			view.Clear()
			t.Fatalf("export %q: intent = %+v, want %+v", kw, got, wantIntent)
		}
		var acc filterapi.ModAccumulator
		if err := chain.Apply(&view, sets, &acc); err != nil {
			view.Clear()
			t.Fatalf("export %q apply: %v", kw, err)
		}
		view.Clear()
		if len(acc.Ops()) != 0 {
			t.Fatalf("export %q apply emitted %+v, want no ops", kw, acc.Ops())
		}
	}

	// 2. Folding precedence with conditional modify + trailing auto-default.
	explicitIP := netip.MustParseAddr("192.0.2.66")
	explicitIP2 := netip.MustParseAddr("192.0.2.77")
	caller := &fakeCaller{filters: map[string]fakeFilter{
		"ext:noop": {
			decl: ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
			answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
			},
		},
		"ext:ip1": {
			decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
			answer: modify(set("next-hop", rpc.FilterSetActionSet, `"192.0.2.66"`)),
		},
		"ext:ip2": {
			decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
			answer: modify(set("next-hop", rpc.FilterSetActionSet, `"192.0.2.77"`)),
		},
		"ext:unchanged": {
			decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
			answer: modify(set("next-hop", rpc.FilterSetActionSet, `"unchanged"`)),
		},
		"ext:auto": {
			decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
			answer: modify(set("next-hop", rpc.FilterSetActionSet, `"auto"`)),
		},
	}}

	cases := []struct {
		name    string
		members []Member
		want    NextHopIntent
	}{
		{
			name:    "unmatched-falls-through-to-trailing-auto",
			members: []Member{{Name: "noop", Plugin: "ext"}, {Name: BuiltinNextHopAuto, AutoDefault: true}},
			want:    NextHopIntent{Mode: NextHopModeAuto, AutoDefault: true},
		},
		{
			name:    "explicit-ip-beats-trailing-auto",
			members: []Member{{Name: "ip1", Plugin: "ext"}, {Name: BuiltinNextHopAuto, AutoDefault: true}},
			want:    NextHopIntent{Mode: NextHopModeExplicit, Addr: explicitIP},
		},
		{
			name:    "explicit-unchanged-beats-trailing-auto",
			members: []Member{{Name: "unchanged", Plugin: "ext"}, {Name: BuiltinNextHopAuto, AutoDefault: true}},
			want:    NextHopIntent{Mode: NextHopModeUnchanged},
		},
		{
			name:    "explicit-auto-beats-auto-default-flag",
			members: []Member{{Name: "auto", Plugin: "ext"}, {Name: BuiltinNextHopAuto, AutoDefault: true}},
			want:    NextHopIntent{Mode: NextHopModeAuto, AutoDefault: false},
		},
		{
			name:    "last-write-wins-among-explicit-ips",
			members: []Member{{Name: "ip1", Plugin: "ext"}, {Name: "ip2", Plugin: "ext"}, {Name: BuiltinNextHopAuto, AutoDefault: true}},
			want:    NextHopIntent{Mode: NextHopModeExplicit, Addr: explicitIP2},
		},
		{
			name:    "explicit-self-after-ip-wins",
			members: []Member{{Name: "ip1", Plugin: "ext"}, {Name: BuiltinNextHopSelf}},
			want:    NextHopIntent{Mode: NextHopModeSelf},
		},
		{
			name:    "explicit-self-after-auto-default-wins",
			members: []Member{{Name: BuiltinNextHopAuto, AutoDefault: true}, {Name: BuiltinNextHopSelf}},
			want:    NextHopIntent{Mode: NextHopModeSelf},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chain, err := Compile(testAttachmentNextHopExport, nil, tc.members, policy.FinalAccept, Bases{})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			var view policyview.View
			sets := new(RunSets)
			sets.SetRoute(ExternalRoute{Caller: caller, Direction: "export", Peer: "192.0.2.1", PeerAS: 65001})
			defer sets.SetRoute(ExternalRoute{})
			view.Reset(update(t, originIGP()), policyview.Session{})
			defer view.Clear()
			if outcome := chain.Run(&view, sets); outcome.Action != OutcomeAccept {
				t.Fatalf("outcome = %+v, want accept", outcome)
			}
			if got := sets.Folded().NextHopIntent(); got != tc.want {
				t.Fatalf("intent = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestPolicyChainNextHopIPv6MPReach proves explicit IPv4 and global IPv6
// next-hop sets emit the right attr 3 and MP_REACH_NLRI (attr 14) wire bytes
// through MPNextHopProfile across IPv4, IPv6, VPNv4, VPNv6, mixed, and
// FlowSpec routes, on both import and export chains, and overlay FieldNextHop
// for read-after-set.
//
// VALIDATES: AC-16 IPv4+IPv6/MP_REACH encoding on import and export, plus
// read-after-set overlay widening.
// PREVENTS: writing bogus attr 3 on pure IPv6/MP_REACH routes or mis-framing
// RD-prefixed VPN next hops.
func TestPolicyChainNextHopIPv6MPReach(t *testing.T) {
	v4 := netip.MustParseAddr("192.0.2.66")
	v4Wire := v4.As4()
	v4MappedWire := netip.AddrFrom16(v4.As16()).As16()
	v6 := netip.MustParseAddr("2001:db8::66")
	v6Wire := v6.As16()

	var zeroRD [8]byte
	vpnV4Wire := append(append([]byte(nil), zeroRD[:]...), v4Wire[:]...)
	vpnV6Wire := append(append([]byte(nil), zeroRD[:]...), v6Wire[:]...)
	vpnMappedWire := append(append([]byte(nil), zeroRD[:]...), v4MappedWire[:]...)

	v6NLRI := []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	legacyNLRI := []byte{24, 203, 0, 113}
	origV6NH := netip.MustParseAddr("2001:db8::1").As16()
	origV4NH := [4]byte{192, 0, 2, 1}
	origVPNv4NH := append(append([]byte(nil), zeroRD[:]...), origV4NH[:]...)
	origVPNv6NH := append(append([]byte(nil), zeroRD[:]...), origV6NH[:]...)

	vpnV4Family := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	vpnV6Family := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	flowSpecV4Family := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	cases := []struct {
		name       string
		direction  string
		setVal     string
		legacyNLRI []byte
		mpAttr     []byte
		wantOps    []filterapi.AttrOp
	}{
		{
			name:       "legacy-ipv4-explicit-ipv4",
			direction:  "export",
			setVal:     `"192.0.2.66"`,
			legacyNLRI: legacyNLRI,
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrNextHop), Action: filterapi.AttrModSet, Buf: v4Wire[:]},
			},
		},
		{
			name:      "ipv6-mp-explicit-ipv6-export",
			direction: "export",
			setVal:    `"2001:db8::66"`,
			mpAttr:    mpReachAttr(family.IPv6Unicast, origV6NH[:], v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: v6Wire[:]},
			},
		},
		{
			name:      "ipv6-mp-explicit-ipv6-import",
			direction: "import",
			setVal:    `"2001:db8::66"`,
			mpAttr:    mpReachAttr(family.IPv6Unicast, origV6NH[:], v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: v6Wire[:]},
			},
		},
		{
			name:      "ipv6-mp-explicit-ipv4-mapped",
			direction: "export",
			setVal:    `"192.0.2.66"`,
			mpAttr:    mpReachAttr(family.IPv6Unicast, origV6NH[:], v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: v4MappedWire[:]},
			},
		},
		{
			name:      "vpnv4-explicit-ipv4-rd",
			direction: "export",
			setVal:    `"192.0.2.66"`,
			mpAttr:    mpReachAttr(vpnV4Family, origVPNv4NH, v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: vpnV4Wire},
			},
		},
		{
			name:      "vpnv6-explicit-ipv6-rd",
			direction: "import",
			setVal:    `"2001:db8::66"`,
			mpAttr:    mpReachAttr(vpnV6Family, origVPNv6NH, v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: vpnV6Wire},
			},
		},
		{
			name:      "vpnv6-explicit-ipv4-rd-mapped",
			direction: "export",
			setVal:    `"192.0.2.66"`,
			mpAttr:    mpReachAttr(vpnV6Family, origVPNv6NH, v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: vpnMappedWire},
			},
		},
		{
			name:       "mixed-legacy-and-ipv6-explicit-ipv4",
			direction:  "export",
			setVal:     `"192.0.2.66"`,
			legacyNLRI: legacyNLRI,
			mpAttr:     mpReachAttr(family.IPv6Unicast, origV6NH[:], v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrNextHop), Action: filterapi.AttrModSet, Buf: v4Wire[:]},
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: v4MappedWire[:]},
			},
		},
		{
			name:       "mixed-legacy-and-ipv6-explicit-ipv6",
			direction:  "export",
			setVal:     `"2001:db8::66"`,
			legacyNLRI: legacyNLRI,
			mpAttr:     mpReachAttr(family.IPv6Unicast, origV6NH[:], v6NLRI),
			wantOps: []filterapi.AttrOp{
				{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet, Buf: v6Wire[:]},
			},
		},
		{
			name:      "flowspec-emits-no-mp-next-hop",
			direction: "export",
			setVal:    `"192.0.2.66"`,
			mpAttr:    mpReachAttr(flowSpecV4Family, nil, []byte{2, 1, 0}),
			wantOps:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller := &fakeCaller{filters: map[string]fakeFilter{"ext:nh": {
				decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
				answer: modify(set("next-hop", rpc.FilterSetActionSet, tc.setVal)),
			}}}
			attachment := testAttachmentNextHopExport
			if tc.direction == "import" {
				attachment = testAttachmentNextHopImport
			}
			chain, err := Compile(attachment, nil, []Member{{Name: "nh", Plugin: "ext"}}, policy.FinalAccept, Bases{})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			attrs := [][]byte{originIGP()}
			if tc.mpAttr != nil {
				attrs = append(attrs, tc.mpAttr)
			}
			wu := updateWithBody(t, tc.legacyNLRI, attrs...)
			var view policyview.View
			sets := new(RunSets)
			sets.SetRoute(ExternalRoute{Caller: caller, Direction: tc.direction, Peer: "192.0.2.1", PeerAS: 65001})
			defer sets.SetRoute(ExternalRoute{})
			view.Reset(wu, policyview.Session{})
			defer view.Clear()
			if outcome := chain.Run(&view, sets); outcome.Action != OutcomeAccept {
				t.Fatalf("outcome = %+v, want accept", outcome)
			}
			var acc filterapi.ModAccumulator
			if err := chain.Apply(&view, sets, &acc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if !reflect.DeepEqual(acc.Ops(), tc.wantOps) {
				t.Fatalf("ops = %+v\nwant  %+v", acc.Ops(), tc.wantOps)
			}
		})
	}

	// Read-after-set overlay: explicit IPv6 overlays 16 bytes on FieldNextHop,
	// while non-explicit next-hop:self leaves FieldNextHop untouched.
	nhReader, err := policyview.ReaderFor(policy.FieldNextHop)
	if err != nil {
		t.Fatal(err)
	}
	caller := &fakeCaller{filters: map[string]fakeFilter{"ext:v6": {
		decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
		answer: modify(set("next-hop", rpc.FilterSetActionSet, `"2001:db8::66"`)),
	}}}
	chain, err := Compile(testAttachmentNextHopExport, nil, []Member{
		{Name: "v6", Plugin: "ext"},
		{Name: BuiltinNextHopSelf},
	}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	var view policyview.View
	sets := new(RunSets)
	sets.SetRoute(ExternalRoute{Caller: caller, Direction: "export", Peer: "192.0.2.1", PeerAS: 65001})
	defer sets.SetRoute(ExternalRoute{})
	view.Reset(updateWithBody(t, nil, originIGP(), mpReachAttr(family.IPv6Unicast, origV6NH[:], v6NLRI)), policyview.Session{})
	defer view.Clear()
	if outcome := chain.Run(&view, sets); outcome.Action != OutcomeAccept {
		t.Fatalf("overlay run outcome = %+v, want accept", outcome)
	}
	gotNH, err := view.Read(nhReader)
	if err != nil {
		t.Fatalf("view.Read(next-hop): %v", err)
	}
	if !bytes.Equal(gotNH.Wire(), v6Wire[:]) {
		t.Fatalf("overlaid next-hop wire = %x, want %x", gotNH.Wire(), v6Wire[:])
	}
}

// TestChainStaticNextHopSameDecisionsAndTrace verifies StaticNextHop fast-path
// classification, HasAutoDefaultNextHop, SameDecisions across built-in steps,
// Chain.Only, Chain.Trace, and Sets.AppendText rendering.
//
// VALIDATES: AC-16, AC-17, AC-18 static fast-path detection, reload equality,
// and show policy test trace output.
// PREVENTS: reload missing AutoDefault changes, RS static fast path falling
// back unnecessarily, or show policy test failing to render built-in steps.
func TestChainStaticNextHopSameDecisionsAndTrace(t *testing.T) {
	var nilChain *Chain
	if intent, ok := nilChain.StaticNextHop(); ok || intent != (NextHopIntent{}) {
		t.Fatalf("nil.StaticNextHop() = (%+v, %v), want (zero, false)", intent, ok)
	}
	if nilChain.HasAutoDefaultNextHop() {
		t.Fatal("nil.HasAutoDefaultNextHop() = true, want false")
	}

	autoDefaultChain, err := CompileUnconfigured(testAttachmentNextHopExport, nil,
		[]Member{{Name: BuiltinNextHopAuto, AutoDefault: true}}, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	if intent, ok := autoDefaultChain.StaticNextHop(); !ok || intent != (NextHopIntent{Mode: NextHopModeAuto, AutoDefault: true}) {
		t.Fatalf("autoDefaultChain.StaticNextHop() = (%+v, %v), want ({Auto, AutoDefault:true}, true)", intent, ok)
	}
	if !autoDefaultChain.HasAutoDefaultNextHop() {
		t.Fatal("autoDefaultChain.HasAutoDefaultNextHop() = false, want true")
	}

	selfChain, err := Compile(testAttachmentNextHopExport, nil,
		[]Member{{Name: BuiltinNextHopSelf}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	if intent, ok := selfChain.StaticNextHop(); !ok || intent != (NextHopIntent{Mode: NextHopModeSelf}) {
		t.Fatalf("selfChain.StaticNextHop() = (%+v, %v), want ({Self}, true)", intent, ok)
	}
	if selfChain.HasAutoDefaultNextHop() {
		t.Fatal("selfChain.HasAutoDefaultNextHop() = true, want false")
	}

	selfThenAuto, err := Compile(testAttachmentNextHopExport, nil,
		[]Member{{Name: BuiltinNextHopSelf}, {Name: BuiltinNextHopAuto, AutoDefault: true}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	if intent, ok := selfThenAuto.StaticNextHop(); !ok || intent != (NextHopIntent{Mode: NextHopModeSelf}) {
		t.Fatalf("selfThenAuto.StaticNextHop() = (%+v, %v), want ({Self}, true)", intent, ok)
	}

	mixedChain, err := Compile(testAttachmentNextHopExport, nil,
		[]Member{{Name: "f", Plugin: "ext"}, {Name: BuiltinNextHopAuto, AutoDefault: true}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	if intent, ok := mixedChain.StaticNextHop(); ok || intent != (NextHopIntent{}) {
		t.Fatalf("mixedChain.StaticNextHop() = (%+v, %v), want (zero, false)", intent, ok)
	}
	if !mixedChain.HasAutoDefaultNextHop() {
		t.Fatal("mixedChain.HasAutoDefaultNextHop() = false, want true")
	}

	// SameDecisions distinguishes mode and AutoDefault while matching rebuilt copies.
	autoExplicit, err := Compile(testAttachmentNextHopExport, nil,
		[]Member{{Name: BuiltinNextHopAuto}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	autoDefaultConfigured, err := Compile(testAttachmentNextHopExport, nil,
		[]Member{{Name: BuiltinNextHopAuto, AutoDefault: true}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	selfChainCopy, err := Compile(testAttachmentNextHopExport, nil,
		[]Member{{Name: BuiltinNextHopSelf}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	if !selfChain.SameDecisions(selfChainCopy) {
		t.Fatal("rebuilt next-hop:self chains differed in SameDecisions")
	}
	if autoExplicit.SameDecisions(autoDefaultConfigured) {
		t.Fatal("SameDecisions treated explicit next-hop:auto equal to AutoDefault")
	}
	if selfChain.SameDecisions(autoExplicit) {
		t.Fatal("SameDecisions treated next-hop:self equal to next-hop:auto")
	}

	// Trace, Only, and AppendText rendering.
	var steps []string
	var view policyview.View
	var sets RunSets
	view.Reset(update(t, originIGP()), policyview.Session{})
	defer view.Clear()
	outcome := selfThenAuto.Trace(&view, &sets, func(step MemberTrace) {
		if step.Step != TracePass || step.Sets == nil {
			t.Errorf("unexpected trace step %+v", step)
			return
		}
		steps = append(steps, step.Name+"="+string(step.Sets.AppendText(nil)))
	})
	if outcome.Action != OutcomeAccept {
		t.Fatalf("trace outcome = %+v, want accept", outcome)
	}
	wantSteps := []string{"next-hop:self=next-hop self", "next-hop:auto=next-hop auto"}
	if !reflect.DeepEqual(steps, wantSteps) {
		t.Fatalf("trace steps = %v, want %v", steps, wantSteps)
	}

	only, ok := selfThenAuto.Only(BuiltinNextHopSelf)
	if !ok {
		t.Fatal("Only(next-hop:self) returned false")
	}
	if intent, ok := only.StaticNextHop(); !ok || intent != (NextHopIntent{Mode: NextHopModeSelf}) {
		t.Fatalf("Only(next-hop:self).StaticNextHop() = (%+v, %v), want ({Self}, true)", intent, ok)
	}

	// AppendText for all next-hop forms.
	for _, tc := range []struct {
		name string
		set  func(*Sets) error
		want string
	}{
		{"ipv4", func(s *Sets) error { return s.NextHop(netip.MustParseAddr("192.0.2.9")) }, "next-hop 192.0.2.9"},
		{"ipv6", func(s *Sets) error { return s.NextHop(netip.MustParseAddr("2001:db8::9")) }, "next-hop 2001:db8::9"},
		{"auto", func(s *Sets) error { return s.NextHopMode(NextHopModeAuto, true) }, "next-hop auto"},
		{"self", func(s *Sets) error { return s.NextHopMode(NextHopModeSelf, false) }, "next-hop self"},
		{"unchanged", func(s *Sets) error { return s.NextHopMode(NextHopModeUnchanged, false) }, "next-hop unchanged"},
	} {
		var s Sets
		if err := tc.set(&s); err != nil {
			t.Fatalf("AppendText %s: %v", tc.name, err)
		}
		if got := string(s.AppendText(nil)); got != tc.want {
			t.Errorf("AppendText(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}

	// allowsRawOverride permits next-hop-only intents before a raw rewrite
	// while refusing non-next-hop sets (which a raw rewrite would discard).
	rawBody := update(t, originIGP()).Payload()
	rawCaller := &fakeCaller{filters: map[string]fakeFilter{
		"ext:raw": {
			decl: ExternalDeclaration{Reads: []string{rpc.FilterFieldRaw}, OnError: rpc.OnErrorReject},
			answer: func(*rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
				return &rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: rawBody}, nil
			},
		},
		"ext:nh-explicit": {
			decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"next-hop"}, OnError: rpc.OnErrorReject},
			answer: modify(set("next-hop", rpc.FilterSetActionSet, `"192.0.2.88"`)),
		},
		"ext:med": {
			decl:   ExternalDeclaration{Reads: []string{"origin"}, Sets: []string{"med"}, OnError: rpc.OnErrorReject},
			answer: modify(set("med", rpc.FilterSetActionSet, `50`)),
		},
	}}
	for _, tc := range []struct {
		name     string
		members  []Member
		wantRaw  bool
		wantMode NextHopMode
		wantAddr string
	}{
		{"unchanged-before-raw", []Member{{Name: BuiltinNextHopUnchanged}, {Name: "raw", Plugin: "ext"}}, true, NextHopModeUnchanged, ""},
		{"explicit-before-raw", []Member{{Name: "nh-explicit", Plugin: "ext"}, {Name: "raw", Plugin: "ext"}}, true, NextHopModeExplicit, "192.0.2.88"},
		{"med-before-raw-refused", []Member{{Name: "med", Plugin: "ext"}, {Name: "raw", Plugin: "ext"}}, false, NextHopModeUnspecified, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch, err := Compile(testAttachmentNextHopExport, nil, tc.members, policy.FinalAccept, Bases{})
			if err != nil {
				t.Fatal(err)
			}
			var v policyview.View
			rs := new(RunSets)
			rs.SetRoute(ExternalRoute{Caller: rawCaller, Direction: "export", Peer: "192.0.2.1", PeerAS: 65001, Raw: rawBody})
			v.Reset(update(t, originIGP()), policyview.Session{})
			out := ch.Run(&v, rs)
			v.Clear()
			if tc.wantRaw {
				if out.Action != OutcomeAccept || rs.Raw() == nil {
					t.Fatalf("outcome=%+v raw=%v, want accept with raw", out, rs.Raw())
				}
				got := rs.Folded().NextHopIntent()
				if got.Mode != tc.wantMode || (tc.wantAddr != "" && got.Addr.String() != tc.wantAddr) {
					t.Fatalf("intent=%+v, want mode=%v addr=%q", got, tc.wantMode, tc.wantAddr)
				}
			} else if out.Action != OutcomeReject || !out.Failed {
				t.Fatalf("outcome=%+v, want failed reject on raw after non-next-hop set", out)
			}
		})
	}
}
