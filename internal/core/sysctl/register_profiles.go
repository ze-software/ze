// Design: docs/architecture/core-design.md -- sysctl profile registry
// Related: profiles.go -- profile types and built-in definitions
// Related: conflicts.go -- conflict detection between profiles

package sysctl

func init() {
	for _, p := range builtinProfiles {
		MustRegisterProfile(p)
	}
	for _, r := range builtinConflicts {
		RegisterConflict(r)
	}
}

// builtinConflicts defines per-sysctl-key conflicts.
// Checked when both keys are active on the same interface.
var builtinConflicts = []ConflictRule{
	{
		KeyA: "arp_ignore", ValueA: "1",
		KeyB: "proxy_arp", ValueB: "1",
		Reason: "arp_ignore=1 (ignore ARP for non-local IPs) contradicts proxy_arp=1 (answer ARP for them)",
	},
	{
		KeyA: "rp_filter", ValueA: "1",
		KeyB: "proxy_arp", ValueB: "1",
		Reason: "rp_filter=1 (strict RPF) drops packets for non-local destinations; proxy_arp=1 advertises reachability",
	},
	{
		KeyA: "arp_announce", ValueA: "2",
		KeyB: "proxy_arp", ValueB: "1",
		Reason: "arp_announce=2 (best-source-only) contradicts proxy_arp=1 (answering for others' IPs)",
	},
}
