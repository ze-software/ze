// VALIDATES: the RFC 4301 Section 6.2 ICMP error processing reaches the Linux kernel
// as the two flags that perform it: XFRM_STATE_ICMP on an inbound state asked for the
// payload check, and XFRM_POLICY_ICMP on an outbound protect policy asked for the
// return-SA mapping. Neither flag rides a state or policy that did not ask for it.
// PREVENTS: the request being dropped in the projection. The kernel then drops every
// ICMP error whose own headers miss the SA's selectors without reading its payload
// (icmp_rcv, net/ipv4/icmp.c), and an ICMP error no SA carries is never mapped to the
// return SA (xfrm_lookup_with_ifid answers -ENOENT), with every config test green.

//go:build linux

package dataplane

import (
	"net"
	"testing"
)

func icmpErrorTestSA(check bool, dir SADir) SAParams {
	return SAParams{
		SPI: 0x0000E001, Src: net.ParseIP("192.0.2.1"), Dst: net.ParseIP("192.0.2.2"),
		Proto: ProtoESP, Mode: ModeTunnel, ReqID: 7, Dir: dir,
		EncAlgo: "aes128gcm", EncKey: make([]byte, 20), IsAEAD: true,
		ICMPErrorCheck: check,
	}
}

func icmpErrorTestPolicy(ret bool, dir SADir) SPParams {
	_, local, _ := net.ParseCIDR("10.1.0.0/24")
	_, remote, _ := net.ParseCIDR("10.2.0.0/24")
	return SPParams{
		Src: local, Dst: remote, Dir: dir, Proto: ProtoESP, Mode: ModeTunnel, ReqID: 7,
		Action: SPActionProtect, Priority: PriorityChildSA,
		TunnelSrc: net.ParseIP("192.0.2.1"), TunnelDst: net.ParseIP("192.0.2.2"),
		ICMPErrorReturn: ret,
	}
}

// VALIDATES: an inbound state asked for the payload check carries XFRM_STATE_ICMP, and
// one not asked carries none.
// RFC requirement: RFC4301-6.2-1 positive -- an inbound SA asked for the ICMP error
// payload check is projected with XFRM_STATE_ICMP, and one not asked is projected
// without it.
func TestXFRMInboundStateCarriesICMPErrorCheck(t *testing.T) {
	on, err := xfrmStateFromParams(icmpErrorTestSA(true, SADirIn))
	if err != nil {
		t.Fatalf("project the checked inbound state: %v", err)
	}
	if !on.ICMP {
		t.Errorf("an inbound state asked for the RFC 4301 Section 6.2 check carries no XFRM_STATE_ICMP")
	}
	off, err := xfrmStateFromParams(icmpErrorTestSA(false, SADirIn))
	if err != nil {
		t.Fatalf("project the unchecked inbound state: %v", err)
	}
	if off.ICMP {
		t.Errorf("an inbound state the operator turned the check off for still carries XFRM_STATE_ICMP")
	}
}

// VALIDATES: the payload check requested on an outbound SA is refused, because Linux
// performs it on the inbound state only and refuses the flag on an SA flagged outbound.
func TestXFRMOutboundStateRefusesICMPErrorCheck(t *testing.T) {
	if _, err := xfrmStateFromParams(icmpErrorTestSA(true, SADirOut)); err == nil {
		t.Fatalf("an outbound state asked for the ICMP error payload check was projected")
	}
}

// VALIDATES: an outbound protect policy asked for the return-SA mapping carries
// XFRM_POLICY_ICMP, one not asked carries none, and the request on an inbound policy or
// a template-free entry is refused rather than installed where the kernel never reads it.
func TestXFRMOutboundPolicyCarriesICMPErrorReturn(t *testing.T) {
	on, err := xfrmPolicyFromParams(icmpErrorTestPolicy(true, SADirOut))
	if err != nil {
		t.Fatalf("project the mapped outbound policy: %v", err)
	}
	if !on.ICMP {
		t.Errorf("an outbound policy asked for the RFC 4301 Section 6.2 return mapping carries no XFRM_POLICY_ICMP")
	}
	off, err := xfrmPolicyFromParams(icmpErrorTestPolicy(false, SADirOut))
	if err != nil {
		t.Fatalf("project the unmapped outbound policy: %v", err)
	}
	if off.ICMP {
		t.Errorf("an outbound policy the operator turned the mapping off for still carries XFRM_POLICY_ICMP")
	}
	if _, err := xfrmPolicyFromParams(icmpErrorTestPolicy(true, SADirIn)); err == nil {
		t.Errorf("an inbound policy asked for the return mapping was projected")
	}
	bypass := icmpErrorTestPolicy(true, SADirOut)
	bypass.Action = SPActionBypass
	if _, err := xfrmPolicyFromParams(bypass); err == nil {
		t.Errorf("a bypass entry asked for the return mapping was projected")
	}
}
