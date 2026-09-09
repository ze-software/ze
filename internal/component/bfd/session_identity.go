// Design: rfc/short/rfc5882.md -- one session per remote system, whatever asks for it
// Detail: api/session_identity.go -- SessionRequest.Canonical, which reads this table

package bfd

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bfd/api"
	ifcomp "github.com/ze-software/ze/internal/component/iface"
)

// connectedLinks reads the link table api.SessionRequest.Canonical derives a
// single-hop session's interface and local address from.
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
	links := make([]api.Link, 0, len(infos))
	// Indexed rather than ranged by value: iface.InterfaceInfo carries stats
	// and address slices, and copying each one to read two fields is 224 bytes
	// a link for nothing.
	for i := range infos {
		l := api.Link{Name: infos[i].Name}
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
