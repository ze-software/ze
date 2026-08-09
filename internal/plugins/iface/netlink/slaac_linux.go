//go:build linux

// Design: docs/architecture/iface/management.md -- kernel-cooperating SLAAC
// address lifecycle tracking. ze does not run a userspace RA client; it observes
// the addresses the kernel autoconfigures from Router Advertisements (RFC 4862)
// via netlink and classifies them by their IFA_F_* flags so status/CLI can
// distinguish a SLAAC/RA-assigned address from a static one.
// RFC: rfc/short/rfc4862.md -- IPv6 Stateless Address Autoconfiguration

package ifacenetlink

import "golang.org/x/sys/unix"

// lifetimeForever is the kernel's "infinite" address lifetime (0xFFFFFFFF).
const lifetimeForever = 0xFFFFFFFF

// Address-origin classifications (AddrInfo.Origin / addrEventPayload.Origin).
const (
	originStatic    = "static"
	originSlaac     = "slaac"
	originTemporary = "temporary"
	originDynamic   = "dynamic"
)

// addrOrigin classifies an address by its kernel IFA_F_* flags. A permanent
// address is operator/kernel-configured (static); a non-permanent IPv6 address
// carries a finite RA lifetime and is therefore SLAAC-autoconfigured (RFC 4862),
// with IFA_F_TEMPORARY marking a privacy/temporary address (RFC 4941); a
// non-permanent IPv4 address is dynamic (typically DHCP).
func addrOrigin(isIPv6 bool, flags int) string {
	if flags&unix.IFA_F_PERMANENT != 0 {
		return originStatic
	}
	if isIPv6 {
		if flags&unix.IFA_F_TEMPORARY != 0 {
			return originTemporary
		}
		return originSlaac
	}
	return originDynamic
}

// normalizeLifetime maps the kernel's infinite (0xFFFFFFFF) or invalid
// (negative) lifetimes to 0 so a permanent address reports no lifetime, while a
// finite RA/lease lifetime is passed through unchanged.
func normalizeLifetime(lft int) uint32 {
	if lft < 0 || lft >= lifetimeForever {
		return 0
	}
	return uint32(lft)
}
