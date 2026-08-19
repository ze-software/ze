// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- IKE control-plane bypass policies
// Related: child.go -- the Child SA policies this exemption outranks
// RFC: rfc/short/rfc4301.md -- SPD dispositions, BYPASS (Section 4.4.1)

package engine

import (
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
)

// protoUDP is the upper-layer selector for the IKE bypass policies. IKE runs over
// UDP (RFC 7296 Section 2.23), so the exemption never widens past UDP.
const protoUDP = 17

// ikeBypassFamilies are the address families the bypass is installed for.
//
// Both, unconditionally. The bypass is installed before any peer is configured, so
// the family of the traffic selectors that will eventually be negotiated is not yet
// known, and guessing it wrong is the failure this exists to prevent. Installing a
// family that never carries a tunnel costs two unused policies and exempts nothing
// that was not already ze's own IKE socket traffic.
var ikeBypassFamilies = []net.IP{net.IPv4zero, net.IPv6zero}

// anyNetFor returns the wildcard prefix of the same address family as sample.
//
// A bypass has to be family-correct rather than family-agnostic: an XFRM selector
// carries a family (vendor netlink xfrm_policy_linux.go, selFromPolicy derives it
// from the destination prefix), so one wildcard cannot cover both.
func anyNetFor(sample net.IP) *net.IPNet {
	if sample.To4() != nil {
		return &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	}
	return &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
}

// ikeBypassPolicies builds the Security Policies that keep ze's OWN IKE traffic out
// of IPsec, in one address family.
//
// WHY THIS EXISTS. A negotiated traffic selector is operator data, and nothing stops
// it covering the two addresses IKE itself runs between. The ordinary site-to-site
// selector 0.0.0.0/0 covers every address there is, so it covers them. Without this
// exemption the Child SA policies capture ze's own IKE datagrams: the outbound
// policy hands ze's reply to ESP and the peer never sees it, and the inbound policy
// demands ESP of the peer's plaintext IKE so the kernel drops it. Measured in the
// strongSwan lab as XfrmInTmplMismatch on the inbound counter and an outbound ESP
// sequence number advancing with zero user traffic. The tunnel then cannot be
// rekeyed, re-established or torn down, because the exchange that would do any of
// those is inside the thing it is trying to manage.
//
// A NARROW selector has the same defect. It is not a property of the wildcard: any
// selector pair whose remote half contains the peer's IKE address captures the same
// traffic. The lab's one passing IPsec scenario escapes only because its remote
// selector happens to exclude the peer, not because it is narrow.
//
// WHY THE LOCAL PORT AND NOT THE PEER ADDRESSES. Scoping the exemption to the peer's
// IKE address pair looks narrower and is worse. A peer behind a NAT presents a
// rewritten source port and, after a NAT rebinding or a MOBIKE move, a different
// address; a pair-scoped bypass silently stops matching exactly when the tunnel most
// needs repairing. The local port cannot move, because ze binds it. Matching on it
// says "traffic terminating on ze's own IKE sockets is not IPsec-processed", which
// is the bound strongSwan gets from a per-socket policy (setsockopt IP_XFRM_POLICY)
// and the shape of its port_bypass option. The remote port is therefore left
// unconstrained on purpose, in both directions.
//
// NO FORWARD DIRECTION. IKE is locally originated and locally delivered, never
// forwarded, so dir out and dir in cover it. A dir fwd bypass would exempt transit
// traffic that ze was asked to protect.
func ikeBypassPolicies(family net.IP) []dataplane.SPParams {
	anyNet := anyNetFor(family)
	out := make([]dataplane.SPParams, 0, 4)
	// RFC 7296 Section 2.23: IKE runs on 500 and floats to 4500 when a NAT is
	// detected. Both are ze's own ports and both need the exemption, because the
	// float happens mid-exchange and the SA that survives it still has to rekey.
	for _, port := range [...]uint16{transport.IKEPort, transport.NATTPort} {
		out = append(out,
			dataplane.SPParams{
				Src:        anyNet,
				Dst:        anyNet,
				Dir:        dataplane.SADirOut,
				Action:     dataplane.SPActionBypass,
				Priority:   dataplane.PriorityIKEBypass,
				UpperProto: protoUDP,
				SrcPort:    dataplane.ExactPortMatch(port),
				DstPort:    dataplane.AnyPortMatch(),
			},
			dataplane.SPParams{
				Src:        anyNet,
				Dst:        anyNet,
				Dir:        dataplane.SADirIn,
				Action:     dataplane.SPActionBypass,
				Priority:   dataplane.PriorityIKEBypass,
				UpperProto: protoUDP,
				SrcPort:    dataplane.AnyPortMatch(),
				DstPort:    dataplane.ExactPortMatch(port),
			},
		)
	}
	return out
}

// installIKEBypass installs the bypass set for every address family.
//
// WHEN. Once, at engine start, immediately after the dataplane is loaded and before
// any peer session exists. That ordering is the point: the exemption has to be in
// place BEFORE the first Child SA policy, or a retransmission or a Delete landing in
// the window between them is swallowed by the policy it was meant to manage.
//
// WHY NOT PER CHILD SA. The exemption belongs to ze's IKE listener, not to any one
// tunnel. Installing it per Child SA would need a refcount to survive several peers
// (one peer's teardown must not strip an exemption another peer's live tunnel
// depends on) and would still leave ze's listener unprotected whenever no Child SA
// happened to exist. Neither problem arises here: the policies carry no peer
// identity, so there is exactly one set no matter how many peers come and go.
//
// It is idempotent regardless: the backend upserts a bypass rather than adding it
// (xfrmBackend.InstallPolicy), so a re-entry re-asserts the same four policies.
//
// A platform with no XFRM is tolerated the same way createFirstChildSA tolerates it.
// The control plane must still run where no dataplane can be programmed, and there
// no policy exists to capture the IKE traffic in the first place.
func installIKEBypass(dp dataplane.Dataplane, log *slog.Logger) {
	if dp == nil {
		return
	}
	for _, family := range ikeBypassFamilies {
		for _, p := range ikeBypassPolicies(family) {
			if err := dp.InstallPolicy(p); err != nil {
				if isXFRMUnsupported(err) {
					log.Debug("ike: bypass policies unavailable on this platform", "error", err)
					return
				}
				// Not fatal to engine start, and deliberately loud. A missing
				// exemption does not stop a tunnel being built; it stops that tunnel
				// being rekeyed or torn down once its selector covers the peer, which
				// is a failure that would otherwise surface much later and look like
				// a peer problem.
				log.Warn("ike: could not install IKE bypass policy; a wide traffic selector may capture ze's own IKE traffic",
					"dir", p.Dir, "error", err)
			}
		}
	}
}

// removeIKEBypass releases the bypass policies for every address family.
//
// It runs on EVERY exit of the engine, from the deferred cleanup registered beside
// installIKEBypass (register.go runEngine), because the policies belong to ze's IKE
// listener rather than to any peer or Child SA: a clean shutdown and an error return
// both end the listener, and the policies are node-wide, so one that outlives the
// process exempts IKE traffic from IPsec for a daemon that is no longer running. Ze
// owns what it touches, so every way out gives the kernel back the state it installed.
//
// A removal that finds nothing is the expected case for a family that never carried
// a tunnel, and for a platform that never installed anything. Errors are logged and
// never returned: shutdown continues.
func removeIKEBypass(dp dataplane.Dataplane, log *slog.Logger) {
	if dp == nil {
		return
	}
	for _, family := range ikeBypassFamilies {
		for _, p := range ikeBypassPolicies(family) {
			if err := dp.RemovePolicyParams(p); err != nil {
				log.Debug("ike: remove IKE bypass policy", "dir", p.Dir, "error", err)
			}
		}
	}
}
