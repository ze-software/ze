//go:build linux

// Design: plan/spec-ipsec-dataplane-inspection.md -- kernel XFRM probe (Linux)
// Related: doctor_xfrm.go -- the check and its seam

package engine

import "github.com/vishvananda/netlink"

// probeXFRM dumps the kernel SPD. The dump needs CAP_NET_ADMIN and a kernel that
// holds XFRM, which together are the exact condition installing a Child SA needs,
// so a failure here is the failure the operator would otherwise meet at tunnel-up.
//
// The two lines are duplicated from xfrmAvailable
// (internal/plugins/ospf/doctor_ipsec_linux.go) on purpose. That symbol is
// unexported and belongs to OSPF, and importing across plugins would make removing
// one plugin break the other (ai/rules/plugins.md).
func probeXFRM() error {
	_, err := netlink.XfrmPolicyList(netlink.FAMILY_ALL)
	return err
}
