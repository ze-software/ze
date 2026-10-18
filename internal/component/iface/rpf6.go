// Design: docs/features/interfaces.md -- IPv6 reverse-path check
// Related: rpf.go -- RPFCheck and the unit walk this selects a leaf for
// Related: backend.go -- Backend.ReplaceIPv6RPF, which enforces the set built here

package iface

// desiredIPv6RPF returns every IPv6 reverse-path check cfg asks for. Linux
// has no IPv6 rp_filter sysctl, so this set is the only carrier of the IPv6
// leaf: no other phase of the apply reads it.
func desiredIPv6RPF(cfg *ifaceConfig, devices map[string]string) []RPFCheck {
	return desiredRPF(cfg, devices, func(u *unitEntry) *rpfMode {
		if u.IPv6 == nil {
			return nil
		}
		return u.IPv6.RPFCheck
	})
}
