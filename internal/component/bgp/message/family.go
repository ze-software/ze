// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4760.md — multiprotocol address families
// Overview: message.go — Message interface and writeHeader

package message

import (
	"github.com/ze-software/ze/internal/core/family"
)

// Type aliases so message package code uses family types directly without casts.
type AFI = family.AFI
type SAFI = family.SAFI

// builtinFamilies returns the registry's own name for each of the four RFC 4760
// base families. The family package registers them itself
// (internal/core/family/registry.go), joining each name from its AFI and SAFI
// parts, so nothing here spells one a second time.
func builtinFamilies() []string {
	return []string{
		family.IPv4Unicast.String(),
		family.IPv6Unicast.String(),
		family.IPv4Multicast.String(),
		family.IPv6Multicast.String(),
	}
}
