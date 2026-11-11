// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS label input on an interface
// Related: rpf.go -- forEachAppliedUnit, the unit walk this set shares
// Related: backend.go -- Backend.ReplaceMPLSInput, which enforces the set built here
// Related: config_sysctl.go -- applySysctl, which writes the same leaf to net.mpls.conf.<dev>.input

package iface

// desiredMPLSInput returns every device whose unit sets `mpls { enable true; }`,
// once each, in config order. The same leaf also reaches
// net.mpls.conf.<dev>.input through applySysctl, which is the whole of it on a
// backend the Linux kernel forwards for. A backend with its own dataplane
// accepts labeled frames outside that sysctl and enforces this set.
func desiredMPLSInput(cfg *ifaceConfig, devices map[string]string) []string {
	var names []string
	seen := make(map[string]bool)
	forEachAppliedUnit(cfg, devices, func(osName string, u *unitEntry) {
		if u.MPLSEnable == nil || !*u.MPLSEnable {
			return
		}
		name := unitOSName(osName, u)
		if seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	})
	return names
}
