// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact directional scope.
// Related: state_scope.go -- ownership; xfrm_linux.go -- installation and readback.

//go:build linux

package dataplane

import (
	"bytes"
	"errors"
	"net"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/ze-software/ze/internal/core/opererr"
	"golang.org/x/sys/unix"
)

func scopeParamSelectors() []SASelector {
	return []SASelector{
		{Src: &net.IPNet{IP: net.IP{10, 1, 0, 0}, Mask: net.CIDRMask(16, 32)},
			Dst:        &net.IPNet{IP: net.IP{10, 2, 0, 0}, Mask: net.CIDRMask(16, 32)},
			UpperProto: unix.IPPROTO_UDP, SrcPort: ExactPortMatch(0),
			DstPort: PortMatch{Port: 4096, Mask: 0xfff0}, IfIndex: 9},
		{Src: &net.IPNet{IP: net.ParseIP("2001:db8:1::"), Mask: net.CIDRMask(48, 128)},
			Dst:        &net.IPNet{IP: net.ParseIP("2001:db8:2::"), Mask: net.CIDRMask(48, 128)},
			UpperProto: unix.IPPROTO_ICMPV6, SrcPort: ExactPortMatch(0),
			DstPort: PortMatch{Mask: 0xfff8}},
	}
}

func scopeSAParams(scope *SAStateScope) SAParams {
	return SAParams{SPI: 0x10203, Src: net.IP{192, 0, 2, 1}, Dst: net.IP{192, 0, 2, 2},
		Proto: ProtoESP, Mode: ModeTunnel, Scope: scope, IsAEAD: true,
		EncAlgo: "aes128gcm", EncKey: make([]byte, 20)}
}

func TestSAStateScopeOwnsSelectors(t *testing.T) {
	inputs := scopeParamSelectors()
	scope, err := NewSAStateScope(inputs, true)
	if err != nil {
		t.Fatal(err)
	}
	inputs[0].Src.IP[0] = 192
	inputs[0].Dst.Mask[0] = 0
	inputs[1].SrcPort = AnyPortMatch()
	got := scope.Selectors()
	if !reflect.DeepEqual(got, scopeParamSelectors()) || !scope.NonInitialFragments() {
		t.Fatalf("scope lost its owned directional records: %+v", got)
	}
	got[0].Src.IP[0] = 172
	got[1].Dst.Mask[0] = 0
	state, err := xfrmStateFromParams(scopeSAParams(scope))
	if err != nil {
		t.Fatal(err)
	}
	value := stateScopeValue(t, state)
	if !bytes.Equal(value, scopeWirePayload(1, scopeWireSelectors())) {
		t.Fatalf("netlink state lost its exact scope or its flags: %x", value)
	}
	if !state.AFUnspec {
		t.Fatalf("mixed-inner-family flag lost: %+v", state)
	}
	clear(value)
	again, err := xfrmStateFromParams(scopeSAParams(scope))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stateScopeValue(t, again), scopeWirePayload(1, scopeWireSelectors())) {
		t.Fatal("netlink attribute mutation changed the owned scope")
	}
}

func TestSAStateScopeAccepts64ExpandedRecords(t *testing.T) {
	selectors := make([]SASelector, 64)
	for i := range selectors {
		selectors[i] = scopeParamSelectors()[i%2]
	}
	scope, err := NewSAStateScope(selectors, false)
	if err != nil {
		t.Fatal(err)
	}
	state, err := xfrmStateFromParams(scopeSAParams(scope))
	if err != nil {
		t.Fatal(err)
	}
	wire := make([]nl.XfrmSelector, 64)
	for i := range wire {
		wire[i] = scopeWireSelectors()[i%2]
	}
	if value := stateScopeValue(t, state); !bytes.Equal(value, scopeWirePayload(0, wire)) {
		t.Fatalf("64-record scope changed: %x", value)
	}
}

func TestSAStateScopeRejectsInvalidSelectors(t *testing.T) {
	cases := []struct {
		name   string
		change func([]SASelector) []SASelector
	}{
		{"empty", func(s []SASelector) []SASelector { return nil }},
		{"too many", func(s []SASelector) []SASelector { return make([]SASelector, 65) }},
		{"missing prefix", func(s []SASelector) []SASelector { s[0].Src = nil; return s }},
		{"invalid mask", func(s []SASelector) []SASelector { s[0].Src.Mask = net.IPMask{255, 0, 255, 0}; return s }},
		{"host bits", func(s []SASelector) []SASelector { s[0].Src.IP[3] = 1; return s }},
		{"mixed pair", func(s []SASelector) []SASelector { s[0].Dst = s[1].Dst; return s }},
		{"noncontiguous port mask", func(s []SASelector) []SASelector { s[0].DstPort.Mask = 0xff05; return s }},
		{"port outside mask", func(s []SASelector) []SASelector { s[0].DstPort.Port = 4097; return s }},
		{"portless protocol", func(s []SASelector) []SASelector { s[0].UpperProto = unix.IPPROTO_GRE; return s }},
		{"ICMP family", func(s []SASelector) []SASelector { s[0].UpperProto = unix.IPPROTO_ICMPV6; return s }},
		{"negative interface", func(s []SASelector) []SASelector { s[0].IfIndex = -1; return s }},
		{"oversized interface", func(s []SASelector) []SASelector { s[0].IfIndex = 1 << 31; return s }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if scope, err := NewSAStateScope(tc.change(scopeParamSelectors()), false); err == nil || scope != nil {
				t.Fatalf("accepted an invalid scope: %+v, %v", scope, err)
			}
		})
	}
}

func TestXFRMStateScopeRejectsUninitializedScope(t *testing.T) {
	var uninitialized SAStateScope
	if _, err := xfrmStateFromParams(scopeSAParams(&uninitialized)); !errors.Is(err, ErrSAOptionRefused) {
		t.Fatalf("zero scope error = %v, want exact refusal", err)
	}
}

func TestXFRMStateScopeInstallRequiresCompleteReceipt(t *testing.T) {
	readBefore, addBefore := xfrmScopeCapsRead, xfrmStateAdd
	t.Cleanup(func() { xfrmScopeCapsRead, xfrmStateAdd = readBefore, addBefore })
	for _, noninitial := range []bool{false, true} {
		for _, features := range []xfrmZEFeature{0, 3, 7, 11, 13, 14, 15} {
			writes := 0
			xfrmScopeCapsRead = func() (*xfrmZECapabilities, error) {
				return &xfrmZECapabilities{Features: features, MaxSelectors: 64, ABI: 0x5a450001}, nil
			}
			xfrmStateAdd = func(state *netlink.XfrmState) error { writes++; return nil }
			scope, err := NewSAStateScope(scopeParamSelectors(), noninitial)
			if err != nil {
				t.Fatal(err)
			}
			err = (&xfrmBackend{}).InstallSA(scopeSAParams(scope))
			if features == 15 {
				if err != nil || writes != 1 {
					t.Fatalf("complete receipt: error %v, writes %d", err, writes)
				}
				continue
			}
			if !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) || writes != 0 {
				t.Fatalf("features %#x, noninitial=%v: error %v, writes %d", features, noninitial, err, writes)
			}
			var refusal *opererr.Error
			if !errors.As(err, &refusal) {
				t.Fatalf("refusal is not an operator error: %v", err)
			}
			if refusal.Fix() != "  ./le build host-driver\n  ./ze-host appliance kernel --target runtime --arch "+runtime.GOARCH {
				t.Fatalf("refusal omits the native rebuild command: %v", err)
			}
		}
	}
}

func TestXFRMStateScopeInstallRefusesUnreadableReceipt(t *testing.T) {
	readBefore, addBefore := xfrmScopeCapsRead, xfrmStateAdd
	t.Cleanup(func() { xfrmScopeCapsRead, xfrmStateAdd = readBefore, addBefore })
	for _, cause := range []error{errXFRMZECapabilitiesAbsent, unix.EPERM} {
		writes := 0
		xfrmScopeCapsRead = func() (*xfrmZECapabilities, error) { return nil, cause }
		xfrmStateAdd = func(*netlink.XfrmState) error { writes++; return nil }
		scope, err := NewSAStateScope(scopeParamSelectors(), false)
		if err != nil {
			t.Fatal(err)
		}
		err = (&xfrmBackend{}).InstallSA(scopeSAParams(scope))
		if !errors.Is(err, ErrSAOptionRefused) || !errors.Is(err, cause) || errors.Is(err, ErrNotSupported) || writes != 0 {
			t.Fatalf("capability error %v lost its refusal or caused a write: %v, writes %d", cause, err, writes)
		}
		if !strings.Contains(err.Error(), "./ze-host appliance kernel --target runtime --arch "+runtime.GOARCH) {
			t.Fatalf("unreadable receipt has no native remedy: %v", err)
		}
	}
}

func TestXFRMStateScopeReceiptRefusalPrecedesESPReceiver(t *testing.T) {
	readBefore, addBefore := xfrmScopeCapsRead, xfrmStateAdd
	t.Cleanup(func() { xfrmScopeCapsRead, xfrmStateAdd = readBefore, addBefore })
	writes := 0
	xfrmScopeCapsRead = func() (*xfrmZECapabilities, error) {
		return &xfrmZECapabilities{Features: 7, MaxSelectors: 64, ABI: 0x5a450001}, nil
	}
	xfrmStateAdd = func(*netlink.XfrmState) error { writes++; return unix.EPERM }
	scope, err := NewSAStateScope(scopeParamSelectors(), false)
	if err != nil {
		t.Fatal(err)
	}
	p := scopeSAParams(scope)
	p.AcceptBothESPForms, p.UDPEncap = true, true
	p.UDPEncapSPort, p.UDPEncapDPort = 4500, 4500
	err = (&xfrmBackend{}).InstallSA(p)
	if !errors.Is(err, ErrSAOptionRefused) || writes != 0 {
		t.Fatalf("partial receipt reached the SA/ESP receiver setup: %v, writes %d", err, writes)
	}
}

func TestXFRMStateScopeTransportPreservesExactSingleton(t *testing.T) {
	selectors := scopeParamSelectors()[:1]
	scope, err := NewSAStateScope(selectors, false)
	if err != nil {
		t.Fatal(err)
	}
	p := scopeSAParams(scope)
	p.Mode = ModeTransport
	p.Sel = &selectors[0]
	state, err := xfrmStateFromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	if value := stateScopeValue(t, state); !bytes.Equal(value, scopeWirePayload(0, scopeWireSelectors()[:1])) {
		t.Fatalf("transport state has no exact scope: %x", value)
	}
	if state.Selector == nil {
		t.Fatal("transport state has no legacy selector")
	}
	if state.Selector.Ifindex != 9 || state.Selector.SrcPortMask != 0xffff || state.Selector.DstPortMask != 0xfff0 || state.AFUnspec {
		t.Fatalf("transport legacy selector differs from its scope: %+v", state)
	}
}

func TestXFRMStateScopeRejectsConflictingLegacySelector(t *testing.T) {
	scope, err := NewSAStateScope(scopeParamSelectors(), false)
	if err != nil {
		t.Fatal(err)
	}
	p := scopeSAParams(scope)
	selector := scopeParamSelectors()[0]
	p.Sel = &selector
	if _, err := xfrmStateFromParams(p); !errors.Is(err, ErrSAOptionRefused) {
		t.Fatalf("legacy singleton beside two-record scope was accepted: %v", err)
	}
	singleton, err := NewSAStateScope(scopeParamSelectors()[:1], false)
	if err != nil {
		t.Fatal(err)
	}
	p.Scope = singleton
	selector.IfIndex = 10
	if _, err := xfrmStateFromParams(p); !errors.Is(err, ErrSAOptionRefused) {
		t.Fatalf("conflicting interface selector was accepted: %v", err)
	}
	p.Sel = nil
	p.Scope = scope
	p.Mode = ModeTransport
	if _, err := xfrmStateFromParams(p); !errors.Is(err, ErrSAOptionRefused) {
		t.Fatalf("multi-record transport scope was accepted: %v", err)
	}
}

func TestXFRMStateScopeReadbackOwnsCompleteScope(t *testing.T) {
	wire := scopeWirePayload(1, scopeWireSelectors())
	withStateList(t, func(int) ([]netlink.XfrmState, error) {
		return []netlink.XfrmState{{Spi: 42, Ifid: 7, ExtraAttrs: []netlink.XfrmAttr{{Type: 42, Value: wire}}}, {Spi: 43, Ifid: 7}}, nil
	})
	got, err := (&xfrmBackend{}).ListSAs(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("dump has %d states, want 2", len(got))
	}
	if !got[0].StateScopeRead || got[0].Scope == nil {
		t.Fatal("scoped SAD entry was reported without its scope")
	}
	if !got[1].StateScopeRead || got[1].Scope != nil {
		t.Fatal("legacy SAD entry was reported as unread or scoped")
	}
	clear(wire)
	if !reflect.DeepEqual(got[0].Scope.Selectors(), scopeParamSelectors()) || !got[0].Scope.NonInitialFragments() {
		t.Fatal("readback changed or aliases the netlink dump")
	}
}

func TestXFRMStateScopeReadbackRefusesInvalidScope(t *testing.T) {
	selectors := scopeWireSelectors()
	selectors[0].User = 1
	wire := scopeWirePayload(0, selectors)
	withStateList(t, func(int) ([]netlink.XfrmState, error) {
		return []netlink.XfrmState{{Spi: 41}, {Spi: 42, ExtraAttrs: []netlink.XfrmAttr{{Type: 42, Value: wire}}}}, nil
	})
	got, err := (&xfrmBackend{}).ListSAs(0)
	if err == nil || got != nil {
		t.Fatalf("invalid scope became a partial successful dump: %+v, %v", got, err)
	}
}

// TestXFRMStateScopeReadbackRefusesScopeOutsideFilter checks that a malformed
// scope on a state of another interface still fails the dump: every dumped
// state's scope is validated before the interface filter.
func TestXFRMStateScopeReadbackRefusesScopeOutsideFilter(t *testing.T) {
	selectors := scopeWireSelectors()
	selectors[0].User = 1
	wire := scopeWirePayload(0, selectors)
	withStateList(t, func(int) ([]netlink.XfrmState, error) {
		return []netlink.XfrmState{{Spi: 41, Ifid: 7}, {Spi: 42, Ifid: 8, ExtraAttrs: []netlink.XfrmAttr{{Type: 42, Value: wire}}}}, nil
	})
	got, err := (&xfrmBackend{}).ListSAs(7)
	if err == nil || got != nil {
		t.Fatalf("malformed scope outside the interface filter was skipped: %+v, %v", got, err)
	}
}

// TestXFRMStateScopeReadbackRefusesDuplicateScope checks that a dumped state with
// two scope attributes fails the dump instead of keeping either one.
func TestXFRMStateScopeReadbackRefusesDuplicateScope(t *testing.T) {
	wire := scopeWirePayload(0, scopeWireSelectors())
	attrs := []netlink.XfrmAttr{{Type: 42, Value: wire}, {Type: 42, Value: wire}}
	withStateList(t, func(int) ([]netlink.XfrmState, error) {
		return []netlink.XfrmState{{Spi: 42, ExtraAttrs: attrs}}, nil
	})
	got, err := (&xfrmBackend{}).ListSAs(0)
	if err == nil || got != nil {
		t.Fatalf("duplicate scope became a successful dump: %+v, %v", got, err)
	}
}
