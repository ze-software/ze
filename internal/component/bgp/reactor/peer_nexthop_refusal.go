// Design: docs/architecture/api/architecture.md -- next-hop resolution errors
// Related: peer.go -- resolveNextHop and canUseNextHopFor, the producer and the rule of this refusal

package reactor

import (
	"net/netip"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// nextHopIncompatible answers the error resolveNextHop returns when addr
// cannot be the next hop of an NLRI of fam on this session (canUseNextHopFor
// answered false). The static and initial-sync rails skip such a route, one
// per route and peer, so the answer holds the facts and renders the operator
// error only when a reader asks: one small allocation per refusal, no text.
func (p *Peer) nextHopIncompatible(session *Session, addr netip.Addr, fam family.Family) error {
	return &nextHopRefusal{
		addr:       addr,
		fam:        fam,
		peerName:   p.settings.Name,
		sessionAFI: p.sessionAFIName(session),
	}
}

// nextHopRefusal is the error of a next hop the session cannot carry for a
// route family. Its text is an operator error naming the route family, the
// next hop, the session's address family and the peer, and the fixes: a next
// hop of the route's own address family (IPv4-mapped for an IPv6 route on an
// IPv4 session), next-hop self on a session of the route's address family,
// each in its update text form and its update hex or b64 form, or the
// extended next hop capability for the pair. Unwrap reaches the
// operator error and, through it, ErrNextHopIncompatible, so every caller's
// errors.Is still matches. Immutable after construction.
type nextHopRefusal struct {
	addr       netip.Addr
	fam        family.Family
	peerName   string // the peer's config name; "" for a peer built in code
	sessionAFI string // sessionAFIName of the session asked
}

func (r *nextHopRefusal) Error() string {
	return r.operatorError().Error()
}

func (r *nextHopRefusal) Unwrap() error {
	return r.operatorError()
}

// operatorError renders the refusal's three parts.
func (r *nextHopRefusal) operatorError() *opererr.Error {
	familyName := r.fam.String()
	peerName := r.peerName
	if peerName == "" {
		// A peer the config built always has its list key as a name; only a
		// peer built in code has none, and a placeholder says so honestly.
		peerName = "<peer name>"
	}
	routeAFI := ipAFIName(r.fam.AFI)

	var problem textbuf.Buffer
	problem.Str("next hop ").Str(r.addr.String()).Str(" cannot be sent for route family ").
		Str(familyName).Str(" to peer ").Str(peerName)

	var why textbuf.Buffer
	why.Str("the ").Str(r.sessionAFI).
		Str(" session did not negotiate the extended next hop capability (RFC 8950) for ").
		Str(familyName).Str(" with next-hop family ").Str(addrAFIName(r.addr))
	if routeAFI != "" {
		why.Str(", and without it route family ").Str(familyName).Str(" takes only an ").
			Str(routeAFI).Str(" next hop")
	}

	// The route reached the reactor from config, update text, update hex or
	// update b64, and the refusal does not know which, so the fix names the
	// next-hop form of both grammars: update text takes `next-hop <address>`
	// and refuses `nhop set` (parseNhopFlat), while update hex and b64 take
	// `nhop set` with the address encoded (parseWireNhopSection).
	var fix textbuf.Buffer
	if routeAFI != "" {
		fix.Str("update text next-hop ").Str(sameFamilyNextHop(r.addr, r.fam.AFI)).
			Str(" (update hex or b64: nhop set with the address encoded), or update text next-hop self on an ").
			Str(routeAFI).Str(" session (update hex or b64: nhop set self), or ")
	}
	fix.Str("set bgp peer ").Str(peerName).Str(" session capability nexthop ").Str(familyName).
		Str(" nhafi ").Str(nhafiName(r.addr))

	return opererr.Wrap(ErrNextHopIncompatible, problem.String(), why.String(), fix.String())
}

// namesNextHop reports whether an announce's next hop names one that
// resolveNextHop can resolve for some session. Unset, and any policy value
// outside the known set (API callers supply it, an open set), name none:
// resolveNextHop answers ErrNextHopUnset for both.
func namesNextHop(nh bgptypes.RouteNextHop) bool {
	switch nh.Policy {
	case bgptypes.NextHopExplicit, bgptypes.NextHopSelf, bgptypes.NextHopDefault:
		return true
	case bgptypes.NextHopUnset:
		return false
	default:
		// Open set, as in resolveNextHop: an unknown policy is an unset one.
		return false
	}
}

// announceNextHopMissing is the operator error of an announce that names no
// next hop. Its cause is ErrNextHopUnset, so errors.Is still matches. The
// route came from update text or from update hex or b64 (a config route takes
// the static rail and never reaches AnnounceNLRIBatch), and the refusal does
// not know which, so the fix names the next-hop form of both grammars, as
// nextHopRefusal does.
func announceNextHopMissing(fam family.Family) error {
	familyName := fam.String()

	var problem textbuf.Buffer
	problem.Str("the announce of ").Str(familyName).Str(" routes names no next hop")

	var why textbuf.Buffer
	why.Str("a peer is sent a route only with the address to forward its traffic to, so Ze sent this route to no peer")

	fix := "update text next-hop <address> or update text next-hop self, before the first nlri or mp section " +
		"(update hex or b64: nhop set with the address encoded, or nhop set self, before the nlri section)"

	return opererr.Wrap(ErrNextHopUnset, problem.String(), why.String(), fix)
}

// sessionAFIName names the address family the session runs over: its
// connected local endpoint, or the peer's configured address before the
// session connected. A session with neither answers "unknown address
// family", never a guess.
func (p *Peer) sessionAFIName(session *Session) string {
	if local := connectedLocalAddress(session); local.IsValid() {
		return addrAFIName(local)
	}
	if p.settings.Address.IsValid() {
		return addrAFIName(p.settings.Address)
	}
	return "unknown address family"
}

// sameFamilyNextHop answers the next hop to write for a route of afi in place
// of addr: the IPv4-mapped form of an IPv4 next hop for an IPv6 route, or a
// placeholder for an address of the route's family.
func sameFamilyNextHop(addr netip.Addr, afi family.AFI) string {
	if afi == family.AFIIPv6 && addr.Is4() {
		var b textbuf.Buffer
		b.Str("::ffff:").Str(addr.String())
		return b.String()
	}
	if afi == family.AFIIPv6 {
		return "<an IPv6 address>"
	}
	return "<an IPv4 address>"
}

// ipAFIName names the IP address family of a route AFI, or "" for an AFI
// whose routes take no natural IP next hop (canUseNextHopFor).
func ipAFIName(afi family.AFI) string {
	if afi == family.AFIIPv4 {
		return "IPv4"
	}
	if afi == family.AFIIPv6 {
		return "IPv6"
	}
	return ""
}

// addrAFIName names the address family of a valid address.
func addrAFIName(addr netip.Addr) string {
	if addr.Is4() {
		return "IPv4"
	}
	return "IPv6"
}

// nhafiName is the `nhafi` value of the capability entry that lets addr be
// the next hop (ze-bgp-conf.yang, capability nexthop).
func nhafiName(addr netip.Addr) string {
	if addr.Is4() {
		return "ipv4"
	}
	return "ipv6"
}
