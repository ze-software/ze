// A const block of canonical family strings outside the family registry's own
// package, beside a registrar call that takes those same four consts. Shaped
// after internal/component/bgp/message/family.go, which the 2026-09-11 sweep
// found.
//
// The registrar call is what makes this the discriminator for the const form of
// the declaration rule. It does not declare these names: family.MustRegister
// joins each one from an AFI name and a SAFI name elsewhere, and this call only
// records that the four are builtin rather than plugin-supplied. So the block
// stays a copy, and a rule that exempts any const a registrar touches would
// lose it.
package fixture

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

const (
	FamilyIPv4Unicast   = "ipv4/unicast"
	FamilyIPv6Unicast   = "ipv6/unicast"
	FamilyIPv4Multicast = "ipv4/multicast"
	FamilyIPv6Multicast = "ipv6/multicast"
)
