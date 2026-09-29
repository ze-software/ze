// Design: rfc/short/rfc5882.md -- one session per remote system, whatever asks for it
// Detail: api/session_identity.go -- SessionRequest.Canonical, which reads this table

package bfd

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bfd/api"
	ifcomp "github.com/ze-software/ze/internal/component/iface"
)

// connectedLinks reads the link table api.SessionRequest.Canonical derives a
// session's interface and local address from, with each link's VRF resolved so
// a request is never given a link from a routing instance it does not name.
//
// An absent or failed interface backend returns nothing, which leaves
// Canonical a no-op rather than a wrong answer: every request then keeps the
// identity its client gave it, which is the behavior that existed before
// RFC 5882 Section 4.4 was enforced here.
func connectedLinks() []api.Link {
	if err := ifcomp.EnsureBackend(); err != nil {
		logger().Debug("bfd session identity: no interface backend, keys stay as written", "err", err)
		return nil
	}
	infos, err := ifcomp.ListInterfaces()
	if err != nil {
		logger().Debug("bfd session identity: interface list failed, keys stay as written", "err", err)
		return nil
	}
	vrfs := vrfMembership(infos)
	links := make([]api.Link, 0, len(infos))
	// Indexed rather than ranged by value: iface.InterfaceInfo carries stats
	// and address slices, and copying each one to read two fields is 224 bytes
	// a link for nothing.
	for i := range infos {
		l := api.Link{Name: infos[i].Name, VRF: vrfs[infos[i].Index]}
		for _, a := range infos[i].Addresses {
			addr, parseErr := netip.ParseAddr(a.Address)
			if parseErr != nil {
				continue
			}
			prefix := netip.PrefixFrom(addr, a.PrefixLength)
			if !prefix.IsValid() {
				continue
			}
			l.Addrs = append(l.Addrs, api.LinkAddress{Addr: addr, Prefix: prefix.Masked()})
		}
		if len(l.Addrs) > 0 {
			links = append(links, l)
		}
	}
	return links
}

// vrfMembership answers each interface index's VRF name.
//
// A VRF is a master device on Linux, and an enslaved interface names it in
// IFLA_MASTER, which the interface component exposes as MasterIndex. The chain
// can be longer than one link, because a VRF member can itself be a bridge or
// a bond that carries the addresses, so the walk climbs until it meets a
// master of type "vrf" or runs out of masters. An interface under no VRF reads
// as api.DefaultVRF, which is what an unset SessionRequest.VRF canonicalizes
// to, so the two compare without a special case.
//
// The walk is bounded by the number of interfaces: a cycle in the master
// chain, which the kernel does not create, would otherwise not terminate.
func vrfMembership(infos []ifcomp.InterfaceInfo) map[int]string {
	byIndex := make(map[int]*ifcomp.InterfaceInfo, len(infos))
	for i := range infos {
		byIndex[infos[i].Index] = &infos[i]
	}
	out := make(map[int]string, len(infos))
	for i := range infos {
		vrf := api.DefaultVRF
		for step, at := 0, &infos[i]; at != nil && step <= len(infos); step++ {
			if at.Type == vrfLinkType {
				vrf = at.Name
				break
			}
			if at.MasterIndex == 0 {
				break
			}
			at = byIndex[at.MasterIndex]
		}
		out[infos[i].Index] = vrf
	}
	return out
}

// vrfLinkType is the kernel's own name for a VRF device, reported by the
// interface component as InterfaceInfo.Type.
const vrfLinkType = "vrf"
