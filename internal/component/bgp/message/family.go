// Design: docs/architecture/wire/messages.md — BGP message types
// RFC: rfc/short/rfc4760.md — multiprotocol address families
// Overview: message.go — Message interface and writeHeader

package message

import (
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
)

// Type aliases so message package code uses family types directly without casts.
type AFI = family.AFI
type SAFI = family.SAFI

// registerBuiltinFamilies records the four RFC 4760 base families in the plugin
// registry's "builtin" source so they appear in completion and inventory output.
// The families themselves are registered in the family package via MustRegister;
// this is a separate concern (telling the plugin registry "these are not from a
// plugin"). Kept here because the family package cannot import plugin/registry.
var _ = registerBuiltinFamilies()

func registerBuiltinFamilies() bool {
	registry.RegisterBuiltinFamilies("builtin", []string{
		FamilyIPv4Unicast,
		FamilyIPv6Unicast,
		FamilyIPv4Multicast,
		FamilyIPv6Multicast,
	})
	return true
}

// Canonical family strings (used in output).
// Format: <afi>/<safi> (e.g., "ipv4/unicast").
const (
	FamilyIPv4Unicast   = "ipv4/unicast"
	FamilyIPv6Unicast   = "ipv6/unicast"
	FamilyIPv4Multicast = "ipv4/multicast"
	FamilyIPv6Multicast = "ipv6/multicast"
)
