// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend

//go:build linux

package dataplane

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
)

// rfc-test-change-approved: 2026-08-01 Thomas approved replacing the RFC7296-2.24 evidence
// after a review found the tagged tests reflected over SAParams field names and called no
// production function, so a no-op InstallSA left both polarities passing.

// ecnTunnelParams is one tunnel-mode ESP Child SA description, shaped like the ones
// installChildSA builds. Section 2.24 binds TUNNEL mode, so the fixture is tunnel mode.
func ecnTunnelParams() SAParams {
	return SAParams{
		SPI:      0xC0FFEE01,
		Src:      net.ParseIP("10.0.0.1"),
		Dst:      net.ParseIP("10.0.0.2"),
		Proto:    ProtoESP,
		Mode:     ModeTunnel,
		ReqID:    7,
		EncAlgo:  "aes256",
		EncKey:   make([]byte, 32),
		AuthAlgo: "sha256",
		AuthKey:  make([]byte, 32),
	}
}

// ecnSetFieldsMentioning returns the names of the fields of v that are set to a non-zero
// value AND whose name mentions any of want.
//
// It reflects over a VALUE the production mapping returned, not over a type literal. A
// mapping that stopped producing a state, or produced a different one, changes what this
// sees.
func ecnSetFieldsMentioning(t *testing.T, v any, want ...string) []string {
	t.Helper()
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			t.Fatalf("%T is nil, so its fields cannot be examined", v)
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		t.Fatalf("%T is not a struct, so its fields cannot be enumerated", v)
	}
	if rv.NumField() == 0 {
		t.Fatalf("%T exposes no fields, so this scan would pass vacuously", v)
	}
	var hits []string
	for i := range rv.NumField() {
		name := strings.ToLower(rv.Type().Field(i).Name)
		matched := false
		for _, w := range want {
			if strings.Contains(name, w) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if !rv.Field(i).IsZero() {
			hits = append(hits, name)
		}
	}
	return hits
}

// VALIDATES: the netlink state the XFRM backend actually builds for a tunnel-mode Child SA
// asks the kernel to disable nothing about ECN, so the kernel's full-functionality
// behavior stands.
//
// PREVENTS: the vacuous predecessor, which reflected over SAParams FIELD NAMES and called
// no production function at all. It also prevents a netlink bump or a mapping change that
// introduces a flags field and sets XFRM_STATE_NOECN through it.
//
// This drives xfrmStateFromParams, which is the whole of the SAParams-to-kernel mapping
// InstallSA performs before its single netlink.XfrmStateAdd call. Breaking that mapping
// reds this test.
//
// RFC requirement: RFC7296-2.24-1 positive -- RFC 7296 Section 2.24: "tunnel encapsulators
// and decapsulators for all tunnel mode SAs created by IKEv2 MUST support the ECN
// full-functionality option for tunnels". The state ze builds carries no flag disabling it,
// and it is a TUNNEL mode state, which is the kind the section binds.
// RFC requirement: RFC7296-2.24-2 positive -- the same section: implementations "MUST
// implement the tunnel encapsulation and decapsulation processing specified in [IPSECARCH]
// to prevent discarding of ECN congestion indications". Ze delegates that processing to the
// kernel, and this proves the state it programs never asks the kernel to discard.
func TestEcnInstalledStateDisablesNothing(t *testing.T) {
	state, err := xfrmStateFromParams(ecnTunnelParams())
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	// rfc-test-change-approved: 2026-08-02 Thomas approved deleting the redundant
	// netlink.Mode(...) conversion here, which `unconvert` refuses and which reds
	// make ze-lint at HEAD. XFRM_MODE_TUNNEL is already a netlink.Mode, so the
	// comparison, its operands and its message are unchanged.
	if state.Mode != netlink.XFRM_MODE_TUNNEL {
		t.Fatalf("the state ze builds for a tunnel-mode SA has mode %v, want tunnel; "+
			"Section 2.24 binds tunnel mode, so a non-tunnel state proves nothing", state.Mode)
	}
	if hits := ecnSetFieldsMentioning(t, state, "ecn", "flag"); len(hits) != 0 {
		t.Errorf("the installed state sets %v, so a path to XFRM_STATE_NOECN exists; "+
			"it must be proven not to disable ECN propagation", hits)
	}
}

// RFC requirement: RFC7296-2.24-1 negative -- the scan above is not vacuous, and its
// subject is not a hand-built literal. The state comes from the production mapping, it
// carries the SPI and the endpoints ze asked for, and the mapping REFUSES a mode it cannot
// express rather than emitting an unflagged default.
// RFC requirement: RFC7296-2.24-2 negative -- Section 2.24 binds tunnel mode, so tunnel
// must be distinguishable from transport in the state ze programs. Without this the
// positive rows would hold over a state that could not say which mode it is.
func TestEcnTheScannedStateIsTheOneZeInstalls(t *testing.T) {
	p := ecnTunnelParams()
	state, err := xfrmStateFromParams(p)
	if err != nil {
		t.Fatalf("xfrmStateFromParams: %v", err)
	}
	if state.Spi != int(p.SPI) {
		t.Errorf("the state carries SPI %#x, want %#x, so the scan reads a state ze did not build",
			state.Spi, p.SPI)
	}
	if !state.Src.Equal(p.Src) || !state.Dst.Equal(p.Dst) {
		t.Errorf("the state carries %v -> %v, want %v -> %v", state.Src, state.Dst, p.Src, p.Dst)
	}

	transport := p
	transport.Mode = ModeTransport
	tState, err := xfrmStateFromParams(transport)
	if err != nil {
		t.Fatalf("xfrmStateFromParams(transport): %v", err)
	}
	if tState.Mode == state.Mode {
		t.Error("tunnel and transport map to one netlink mode, so no state can be identified as tunnel mode")
	}

	bad := p
	bad.Mode = 0
	if _, err := xfrmStateFromParams(bad); err == nil {
		t.Error("an unknown mode built a state instead of being refused, so a Child SA of no known mode reaches the kernel")
	}
}
