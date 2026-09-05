// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: kernelcap.go -- the enrolment these predicates are registered with
//
// One predicate for each enrolled subsystem, and one declaration of each.
// MPLSInUse and IPsecInUse answer the same question for every reader, so a
// refusal, a doctor row and a listener bind cannot disagree about whether a
// subsystem is configured (owner decision 6, 2026-08-14).

package kernelcap

import (
	"slices"

	"github.com/ze-software/ze/internal/component/config"
)

// labeledFamilies are the BGP family names that mean MPLS forwarding, and
// therefore that the kernel needs an AF_MPLS table. Package-level so the test
// that pins them against a parsed config reads the same list the predicate does.
var labeledFamilies = []string{"ipv4/mpls-label", "ipv6/mpls-label", "ipv4/mpls-vpn", "ipv6/mpls-vpn"}

// MPLSInUse reports whether the configuration asks the LINUX KERNEL to forward
// MPLS: a labeled BGP family on a peer or on a group, LDP, RSVP-TE, or a
// per-interface MPLS enable, on the kernel FIB backend.
//
// The backend gate comes FIRST and it is load-bearing. `fib { kernel { } }` is
// what activates the plugin that programs labels; a VPP or P4 backend does its
// own MPLS and must never be refused for a kernel table it does not use. A
// plain BGP-over-kernel config imposes no labels and is not gated either.
func MPLSInUse(tree *config.Tree) bool {
	if tree == nil {
		return false
	}
	fib := tree.GetContainer("fib")
	if fib == nil {
		return false
	}
	if fib.GetContainer("kernel") == nil {
		return false
	}
	return mplsConfigured(tree)
}

// mplsConfigured reports whether any MPLS-forwarding config is present,
// whatever the FIB backend. MPLSInUse gates it on the kernel backend.
func mplsConfigured(tree *config.Tree) bool {
	if tree.GetContainer("ldp") != nil || tree.GetContainer("rsvp-te") != nil {
		return true
	}
	for _, entry := range tree.GetListOrdered("interface") {
		if entry.Value.GetContainer("mpls") != nil {
			return true
		}
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		return false
	}
	if peersLabeled(bgp) {
		return true
	}
	for _, group := range bgp.GetListOrdered("group") {
		// A group carries the full peer-fields grouping
		// (internal/component/bgp/yang/ze-bgp-conf.yang `list group { uses
		// peer-fields; }`), so the family may be declared ONCE on the group and
		// on none of its peers; ResolveBGPTree deep-merges it into every member
		// (internal/component/bgp/config/resolve.go). Checking only the group's
		// peers misses that shape, and it is the idiomatic one.
		if sessionLabeled(group.Value) || peersLabeled(group.Value) {
			return true
		}
	}
	return false
}

// sessionLabeled reports whether the container's OWN session negotiates a
// labeled family. Used for a group, whose session every member peer inherits.
func sessionLabeled(container *config.Tree) bool {
	session := container.GetContainer("session")
	if session == nil {
		return false
	}
	for _, family := range session.GetListOrdered("family") {
		if slices.Contains(labeledFamilies, family.Key) {
			return true
		}
	}
	return false
}

// peersLabeled reports whether any peer directly under the container negotiates
// a labeled-unicast or MPLS-VPN family.
//
// `family` is a LIST keyed by the family name, not a container: `family
// ipv4/mpls-label { ... }` parses to a list ENTRY whose key is the family, so
// GetContainer("family") is nil for it and the loop below would be unreachable.
func peersLabeled(container *config.Tree) bool {
	for _, peer := range container.GetListOrdered("peer") {
		if sessionLabeled(peer.Value) {
			return true
		}
	}
	return false
}

// IPsecInUse reports whether the configuration installs an IPsec Security
// Association, and therefore needs the kernel XFRM dataplane.
//
// An EMPTY `vpn { ipsec { } }` block installs nothing. ParseIPsecConfig
// (internal/component/ike/ipsec/config.go) fills Peers only from
// `site-to-site/peer` and RemoteAccess only from `remote-access`, so a block
// carrying neither describes no tunnel. Reporting it as in use refused a start,
// opened two UDP listeners and warned about kernel modules, all for a
// configuration that would have carried no packet.
func IPsecInUse(tree *config.Tree) bool {
	if tree == nil {
		return false
	}
	vpn := tree.GetContainer("vpn")
	if vpn == nil {
		return false
	}
	ipsec := vpn.GetContainer("ipsec")
	if ipsec == nil {
		return false
	}
	if ipsec.GetContainer("remote-access") != nil {
		return true
	}
	siteToSite := ipsec.GetContainer("site-to-site")
	if siteToSite == nil {
		return false
	}
	return len(siteToSite.GetListOrdered("peer")) > 0
}
