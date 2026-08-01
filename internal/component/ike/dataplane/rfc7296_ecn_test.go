// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend

package dataplane

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// ecnFieldNames returns the field names of a struct type, lowercased.
//
// It refuses a type with no fields. On a non-Linux build the vendored netlink bindings
// collapse their XFRM types to struct{}, and a scan over one of those would report "no
// ECN knob found" while examining nothing at all. That is the vacuous pass this helper
// exists to make impossible (ai/rules/fail-closed-guards.md).
func ecnFieldNames(t *testing.T, v any) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	if rt.Kind() != reflect.Struct {
		t.Fatalf("%T is not a struct, so its fields cannot be enumerated", v)
	}
	names := make([]string, 0, rt.NumField())
	for f := range rt.Fields() {
		names = append(names, strings.ToLower(f.Name))
	}
	if len(names) == 0 {
		t.Fatalf("%T exposes no fields, so this scan would pass vacuously", v)
	}
	return names
}

// VALIDATES: nothing a negotiated Child SA carries can ask ze to disable ECN propagation.
// SAParams is the complete description ze builds for a Child SA and hands to a backend,
// so a knob that does not exist there cannot be reached by any config leaf, any proposal,
// or any traffic selector.
//
// PREVENTS: a future field or config leaf that turns ECN propagation off. Linux copies
// the congestion indication on decapsulation UNLESS XFRM_STATE_NOECN is set, so the
// conformance of the shipping backend rests on that flag staying unreachable. This is the
// early signal that a path to it appeared.
//
// SCOPE, stated plainly: this measures what ZE controls. It does not measure the kernel's
// own encapsulation and decapsulation processing, which is where the ECN copying happens
// and which ze neither implements nor configures. The companion linux-only check asserts
// the netlink state ze fills carries no general flags field either. The VPP backend
// cannot program a security association at all, and that is tracked in
// plan/spec-fixit-vpp-ipsec-inoperable.md rather than here.
//
// RFC requirement: RFC7296-2.24-1 positive -- RFC 7296 Section 2.24: "tunnel
// encapsulators and decapsulators for all tunnel mode SAs created by IKEv2 MUST support
// the ECN full-functionality option for tunnels". The XFRM backend supports it by leaving
// the kernel's default in place, and ze has no way to leave that default.
// RFC requirement: RFC7296-2.24-2 positive -- the same section: implementations "MUST
// implement the tunnel encapsulation and decapsulation processing specified in
// [IPSECARCH] to prevent discarding of ECN congestion indications". Ze delegates that
// processing to the kernel and never asks it to discard.
func TestEcnNoConfigPathDisablesECN(t *testing.T) {
	for _, name := range ecnFieldNames(t, SAParams{}) {
		if strings.Contains(name, "ecn") || name == "flags" {
			t.Errorf("SAParams now exposes %q, so a path to ECN behavior exists; "+
				"it must be proven not to disable propagation", name)
		}
	}
}

// RFC requirement: RFC7296-2.24-1 negative -- the scan above is not vacuous. SAParams DOES
// carry the fields ze fills for a Child SA, so a check that found no ECN knob because it
// was reading an empty or wrong type fails here instead of passing quietly.
// RFC requirement: RFC7296-2.24-2 negative -- Section 2.24 binds TUNNEL mode SAs, so the
// mode ze programs must be expressible and must be distinguishable from transport mode.
// Without this the positive rows would hold over an SA description that could not even
// say which mode it is.
func TestEcnTheScannedTypeIsTheOneZeFills(t *testing.T) {
	params := ecnFieldNames(t, SAParams{})
	for _, want := range []string{"spi", "mode", "proto", "src", "dst"} {
		if !slices.Contains(params, want) {
			t.Errorf("SAParams carries no %q field, so the scan is reading the wrong type", want)
		}
	}
	if ModeTunnel == ModeTransport {
		t.Fatal("tunnel and transport mode share a value, so no SA can be identified as tunnel mode")
	}
}
