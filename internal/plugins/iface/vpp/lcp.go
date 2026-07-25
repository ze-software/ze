// Design: docs/research/vpp-deployment-reference.md -- Linux Control Plane (LCP)
// Overview: ifacevpp.go -- vppBackendImpl, lcpHosts tracking map
//
// A Linux Control Plane pair creates a Linux TAP that shadows a VPP dataplane
// interface, so kernel networking (the ze BGP listener, ssh, ...) can bind on a
// VPP-owned NIC. The pair is programmed with lcp_itf_pair_add_del.
//
// netns: the TAP lands in the namespace given by vpp.lcp.netns. ze maps its
// root-reachable markers (host/root/empty) to the empty per-pair netns and passes
// any other value through, but that mapping does NOT place the TAP in VPP's own
// namespace -- see lcpPairNetns below for what VPP actually does with the value.
// The doctor check (doctor.go: doctor-vpp-lcp-netns) warns that BGP cannot bind
// across the namespace boundary (A-4).
//
// host name: Linux caps interface names at IFNAMSIZ-1 = 15 bytes, so the host
// TAP name is validated <= 15 and rejected on collision rather than silently
// truncated (AC-7/R-5).

package ifacevpp

import (
	"fmt"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/lcp"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

// lcpMaxHostName is the longest Linux interface name (IFNAMSIZ-1). The lcp
// binary API field is string[16] (15 usable bytes plus the NUL terminator).
const lcpMaxHostName = 15

// getActiveLCPSettings is the seam ifacevpp uses to read the running VPP
// Manager's LCP configuration. Tests override it to inject settings without a
// live VPP component.
var getActiveLCPSettings = vppcomp.GetActiveLCPSettings

// SetupLCPPair creates a Linux Control Plane pair (a host TAP) shadowing the
// named VPP interface. It is a no-op when LCP is not enabled, so config-apply
// can call it unconditionally for vpp-backed interfaces. hostName defaults to
// the ze interface name; it is validated <= 15 bytes and rejected on collision.
func (b *vppBackendImpl) SetupLCPPair(vppIface, hostName string) error {
	settings, ok := getActiveLCPSettings()
	if !ok || !settings.Enabled {
		return nil
	}
	if hostName == "" {
		hostName = vppIface
	}
	if len(hostName) > lcpMaxHostName {
		return fmt.Errorf("ifacevpp: lcp host name %q for %q exceeds %d bytes (Linux IFNAMSIZ); rename the interface or set a shorter host name", hostName, vppIface, lcpMaxHostName)
	}
	if err := b.reserveLCPHost(vppIface, hostName); err != nil {
		return err
	}
	idx, err := b.resolveIndex(vppIface)
	if err != nil {
		b.takeLCPHost(vppIface) // release the reservation on failure
		return fmt.Errorf("ifacevpp: lcp pair %q: %w", vppIface, err)
	}
	if err := b.lcpItfPair(true, idx, hostName, lcpPairNetns(settings.Netns)); err != nil {
		b.takeLCPHost(vppIface)
		return err
	}
	return nil
}

// RemoveLCPPair tears down the LCP pair recorded for vppIface. It is idempotent:
// with no recorded pair it is a no-op.
func (b *vppBackendImpl) RemoveLCPPair(vppIface string) error {
	hostName, ok := b.takeLCPHost(vppIface)
	if !ok {
		return nil
	}
	settings, _ := getActiveLCPSettings()
	idx, err := b.resolveIndex(vppIface)
	if err != nil {
		return fmt.Errorf("ifacevpp: lcp pair %q: %w", vppIface, err)
	}
	return b.lcpItfPair(false, idx, hostName, lcpPairNetns(settings.Netns))
}

// lcpItfPair issues one lcp_itf_pair_add_del. The host interface is a TAP
// (LCP_API_ITF_HOST_TAP) so the shadow carries a full Ethernet header, which is
// what a kernel routing daemon expects to bind and send on.
func (b *vppBackendImpl) lcpItfPair(add bool, idx interface_types.InterfaceIndex, hostName, netns string) error {
	req := &lcp.LcpItfPairAddDel{
		IsAdd:      add,
		SwIfIndex:  idx,
		HostIfName: hostName,
		HostIfType: lcp.LCP_API_ITF_HOST_TAP,
		Netns:      netns,
	}
	reply := &lcp.LcpItfPairAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("ifacevpp: LcpItfPairAddDel %q: %w", hostName, err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("ifacevpp: LcpItfPairAddDel %q retval=%d", hostName, reply.Retval)
	}
	return nil
}

// lcpPairNetns maps a configured lcp netns to the per-pair netns field of
// lcp_itf_pair_add_del: ze's root-reachable markers (host/root/empty) become "",
// any other name is passed through.
//
// CAUTION: "" does NOT mean "VPP's own namespace". lcp_itf_pair_create resolves an
// empty per-pair netns to the GLOBAL default
// (third_party/vpp-linux-cp/src/lcp_interface.c:850-855), and ze always
// writes that default from the same vpp.lcp.netns leaf
// (internal/component/vpp/startupconf.go:106), which lcp_set_default_ns opens as
// /var/run/netns/<name> (third_party/vpp-linux-cp/src/lcp.c:73-74). So the markers
// do not put the TAP where ze
// runs: netns=host asks VPP for a namespace literally called host. The mapping is
// left as-is here (A-13 in plan/spec-bgp-netns.md, which owns the fix); either way
// doctor-vpp-lcp-netns warns that BGP cannot bind across the namespace boundary.
func lcpPairNetns(configured string) string {
	if lcpNetnsIsRootReachable(configured) {
		return ""
	}
	return configured
}

// reserveLCPHost records hostName for vppIface, rejecting a host name already
// used by a different ze interface (which would collide in VPP / Linux).
func (b *vppBackendImpl) reserveLCPHost(vppIface, hostName string) error {
	b.lcpMu.Lock()
	defer b.lcpMu.Unlock()
	if b.lcpHosts == nil {
		b.lcpHosts = make(map[string]string)
	}
	for other, existing := range b.lcpHosts {
		if existing == hostName && other != vppIface {
			return fmt.Errorf("ifacevpp: lcp host name %q already used by interface %q", hostName, other)
		}
	}
	b.lcpHosts[vppIface] = hostName
	return nil
}

// takeLCPHost removes and returns the host name recorded for vppIface.
func (b *vppBackendImpl) takeLCPHost(vppIface string) (string, bool) {
	b.lcpMu.Lock()
	defer b.lcpMu.Unlock()
	hostName, ok := b.lcpHosts[vppIface]
	if ok {
		delete(b.lcpHosts, vppIface)
	}
	return hostName, ok
}
