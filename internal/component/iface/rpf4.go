// Design: docs/features/interfaces.md -- IPv4 reverse-path check
// Related: rpf.go -- RPFCheck and the unit walk this selects a leaf for
// Related: backend.go -- Backend.ReplaceIPv4RPF, which enforces the set built here
// Related: config_sysctl.go -- applySysctl, which writes the same leaf to rp_filter

package iface

// desiredIPv4RPF returns every IPv4 reverse-path check cfg asks for. The same
// leaf also reaches net.ipv4.conf.<dev>.rp_filter through applySysctl, which
// is the whole check on a backend the Linux kernel forwards for. A backend
// with its own dataplane forwards outside that sysctl and enforces this set.
func desiredIPv4RPF(cfg *ifaceConfig, devices map[string]string) []RPFCheck {
	return desiredRPF(cfg, devices, func(u *unitEntry) *rpfMode {
		if u.IPv4 == nil {
			return nil
		}
		return u.IPv4.RPFCheck
	})
}
