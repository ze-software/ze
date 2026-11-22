//go:build linux

package fibkernel

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestMPLSCapabilityCodesRegistered proves this plugin's kernel capability is
// enrolled and that both codes it answers with reach `ze show diagnostic`.
//
// VALIDATES: an operator meeting doctor-mpls-unavailable or doctor-mpls-unknown
// can look the code up.
// PREVENTS: a capability that names a code the registry does not hold.
func TestMPLSCapabilityCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	if !slices.Contains(kernelcap.Enrolled(), "mpls") {
		t.Fatal("the mpls kernel capability is not enrolled, so its codes are unreachable")
	}
	for _, code := range []string{codeDoctorMPLSUnavailable, codeDoctorMPLSUnknown} {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is declared by the mpls capability but not registered", code)
		}
	}
}

// VALIDATES: the transit MTU capability is enrolled, reaches `ze show diagnostic`
// through both codes, and is a degrading capability: its absence warns and
// never refuses a start.
// PREVENTS: a stock kernel stopping an RSVP-TE router whose only loss is the
// transit MTU bound, and that loss going unreported (owner decision,
// 2026-10-08).
func TestMPLSTransitMTUCapabilityDegrades(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	if !slices.Contains(kernelcap.Enrolled(), "mpls-transit-mtu") {
		t.Fatal("the mpls-transit-mtu kernel capability is not enrolled")
	}
	for _, code := range []string{codeDoctorMPLSTransitMTUUnenforced, codeDoctorMPLSTransitMTUUnknown} {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is declared by the mpls-transit-mtu capability but not registered", code)
		}
	}
	if transitMTUCapability.Degrades == "" {
		t.Error("the transit MTU capability refuses a start on a stock kernel; it must only degrade")
	}
	if transitMTUCapability.Kernel != "CONFIG_MPLS_IP_MTU" {
		t.Errorf("the capability names %q, want the build symbol in internal/appliance/kernelreq.go", transitMTUCapability.Kernel)
	}
}

// TestSRv6CapabilityCodesRegistered proves both SRv6 kernel capabilities are
// enrolled and that all four diagnostic codes reach `ze show diagnostic` with
// title, description, and remedy.
//
// VALIDATES: an operator meeting any doctor-srv6-* diagnostic can look the
// code up and read what is wrong, why, and the remedy.
// PREVENTS: an enrolled SRv6 capability naming an unregistered or empty code.
func TestSRv6CapabilityCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()

	enrolled := kernelcap.Enrolled()
	for _, subsystem := range []string{"srv6-encap", "srv6-forward-ttl"} {
		if !slices.Contains(enrolled, subsystem) {
			t.Fatalf("the %s kernel capability is not enrolled, so its codes are unreachable", subsystem)
		}
	}

	codes := []string{
		codeDoctorSRv6EncapUnavailable,
		codeDoctorSRv6EncapUnknown,
		codeDoctorSRv6ForwardTTLNotDecremented,
		codeDoctorSRv6ForwardTTLUnknown,
	}
	for _, code := range codes {
		meta := diagnostic.Lookup(code)
		if meta == nil {
			t.Errorf("diagnostic code %q is declared by an SRv6 capability but not registered", code)
			continue
		}
		if meta.Title == "" {
			t.Errorf("diagnostic code %q has an empty Title", code)
		}
		if meta.Description == "" {
			t.Errorf("diagnostic code %q has an empty Description", code)
		}
		if !strings.Contains(meta.Description, "Remedy:") {
			t.Errorf("diagnostic code %q Description does not state a Remedy: %s", code, meta.Description)
		}
	}
}

// TestSRv6CapabilitiesDegrade proves both SRv6 enrolments declare Degrades and
// carry the expected kernel build symbols, so absent answers warn and never
// refuse a start.
//
// VALIDATES: srv6-encap and srv6-forward-ttl degrade rather than refusing, and
// kernelcap.Refuse returns nil when both are absent.
// PREVENTS: a stock kernel without seg6 or patch 0014 blocking a BGP router on
// the kernel FIB from starting.
func TestSRv6CapabilitiesDegrade(t *testing.T) {
	if srv6EncapCapability.Degrades == "" {
		t.Error("srv6-encap must declare Degrades so absence warns rather than refusing a start")
	}
	if srv6ForwardTTLCapability.Degrades == "" {
		t.Error("srv6-forward-ttl must declare Degrades so absence warns rather than refusing a start")
	}
	if srv6EncapCapability.Kernel != "CONFIG_IPV6_SEG6_LWTUNNEL" {
		t.Errorf("srv6-encap names %q, want CONFIG_IPV6_SEG6_LWTUNNEL", srv6EncapCapability.Kernel)
	}
	if srv6ForwardTTLCapability.Kernel != "CONFIG_IPV6_SEG6_FORWARD_TTL" {
		t.Errorf("srv6-forward-ttl names %q, want CONFIG_IPV6_SEG6_FORWARD_TTL", srv6ForwardTTLCapability.Kernel)
	}

	t.Cleanup(kernelcap.ForceAnswersForTest("srv6-encap=absent,srv6-forward-ttl=absent"))

	tree := config.NewTree()
	tree.GetOrCreateContainer("fib").GetOrCreateContainer("kernel")
	tree.GetOrCreateContainer("bgp").AddListEntry("peer", "p1", config.NewTree())

	if err := kernelcap.Refuse(tree); err != nil {
		t.Errorf("degrading SRv6 capabilities refused a start: %v", err)
	}

	diags := kernelcap.Evaluate(tree)
	seen := map[string]diagnostic.Severity{}
	for i := range diags {
		seen[diags[i].Code] = diags[i].Severity
	}
	for _, code := range []string{codeDoctorSRv6EncapUnavailable, codeDoctorSRv6ForwardTTLNotDecremented} {
		sev, ok := seen[code]
		if !ok {
			t.Errorf("expected diagnostic %q when forced absent, got %+v", code, diags)
			continue
		}
		if sev != diagnostic.SeverityWarning {
			t.Errorf("diagnostic %q severity = %q, want warning", code, sev)
		}
	}
}
