// Design: docs/architecture/ike/ipsec-14-responder.md -- the Configuration payload in IKE_AUTH
// RFC: rfc/short/rfc7296.md -- Sections 2.19, 2.20, 3.15.1, 3.15.4
// RFC: rfc/short/rfc3948.md -- locally unique inner addresses (Section 5.1)
// Related: responder.go -- selectAuthChildSA, the one place a lease is granted
// Related: ts_narrow.go -- narrowChildSelectors puts the lease on the remote selector
// Related: virtual_ip.go -- the lease registry and the pool descriptors

package engine

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var (
	// errCPRequired is the refusal of a pool-bound peer that sent no usable CFG_REQUEST.
	// notifyForRefusal maps it to FAILED_CP_REQUIRED.
	errCPRequired = errors.New("ike auth: the peer is bound to a virtual-IP pool and sent no CFG_REQUEST for an address")
	// errNoAddressAssigned is the refusal of a request no address could answer: the pool
	// is exhausted, serves none of the requested families, or cannot be resolved.
	// notifyForRefusal maps it to INTERNAL_ADDRESS_FAILURE.
	errNoAddressAssigned = errors.New("ike auth: no virtual IP could be assigned")
)

// cpRequest is one CFG_REQUEST Ze decided to serve: the families the peer asked an
// address of, the address it asked for in each (zero for any), and the other attributes
// whose answer depends on being asked.
type cpRequest struct {
	want4, want6 bool
	req4, req6   netip.Addr
	netmask      bool
	appVersion   bool
	supported    bool
}

// cpHandler reads one requested attribute type into the request. The table below is the
// only list of types Ze handles: decideCPRequest dispatches through it and
// SUPPORTED_ATTRIBUTES is derived from it, so the two cannot disagree.
type cpHandler struct {
	attr uint16
	read func(r *cpRequest, value []byte)
}

// cpHandlers lists the CFG_REQUEST attributes Ze answers. DNS servers are sent whenever
// the pool carries them (strongSwan create_responder_enumerator), so their handlers only
// mark the type as handled.
var cpHandlers = [...]cpHandler{
	{wire.CPAttrInternalIP4Address, readIP4AddressRequest},
	{wire.CPAttrInternalIP4Netmask, func(r *cpRequest, _ []byte) { r.netmask = true }},
	{wire.CPAttrInternalIP4DNS, func(*cpRequest, []byte) {}},
	{wire.CPAttrInternalIP6Address, readIP6AddressRequest},
	{wire.CPAttrInternalIP6DNS, func(*cpRequest, []byte) {}},
	{wire.CPAttrApplicationVersion, func(r *cpRequest, _ []byte) { r.appVersion = true }},
	{wire.CPAttrSupportedAttributes, readSupportedRequest},
}

// readIP4AddressRequest records an INTERNAL_IP4_ADDRESS request. RFC 7296 Section 3.15.1
// gives the value as zero or four octets; four octets name the address the peer asks for.
func readIP4AddressRequest(r *cpRequest, value []byte) {
	r.want4 = true
	if len(value) == net.IPv4len {
		r.req4 = netip.AddrFrom4([4]byte(value))
	}
}

// readIP6AddressRequest records an INTERNAL_IP6_ADDRESS request: zero or seventeen octets,
// the address followed by its prefix length (RFC 7296 Section 3.15.1).
func readIP6AddressRequest(r *cpRequest, value []byte) {
	r.want6 = true
	if len(value) == net.IPv6len+1 {
		r.req6 = netip.AddrFrom16([16]byte(value[:net.IPv6len]))
	}
}

// readSupportedRequest records a SUPPORTED_ATTRIBUTES request, which RFC 7296 Section
// 3.15.1 says "MUST be zero length" in a CFG_REQUEST; a non-empty one is not a request.
func readSupportedRequest(r *cpRequest, value []byte) {
	if len(value) == 0 {
		r.supported = true
	}
}

// decideCPRequest decides the Configuration payloads of one IKE_AUTH request. It answers
// a request only for exactly one CP of type CFG_REQUEST asking at least one address; every
// other shape (no CP, two, a Reply, Set or ACK, no address attribute) is not a request Ze
// serves, and answers nil. The nil is the stored "no request" state assignVirtualIP
// refuses with errCPRequired, so the caller stores the answer as it is.
// CFG_SET is ignored, as RFC 7296 Section 3.15.1 allows: "An implementation of this
// specification MAY ignore CFG_SET payloads."
//
// RFC 7296 Section 3.15.1: "Unrecognized or unsupported attributes MUST be ignored in both
// requests and responses." A type cpHandlers does not list, the obsolete
// INTERNAL_ADDRESS_EXPIRY (Section 1.7) included, is skipped. The Reserved bit is already
// cleared by PayloadCP.ReadFrom.
func decideCPRequest(cps []*wire.PayloadCP) *cpRequest {
	if len(cps) != 1 {
		return nil
	}
	cp := cps[0]
	if cp.CFGType != wire.CFGTypeRequest {
		return nil
	}
	req := &cpRequest{}
	for _, attr := range cp.Attrs {
		for i := range cpHandlers {
			if cpHandlers[i].attr == attr.Type {
				cpHandlers[i].read(req, attr.Value)
				break
			}
		}
	}
	if !req.want4 && !req.want6 {
		return nil
	}
	return req
}

// virtualIPGrant is the lease set one IKE SA holds, and what its CFG_REPLY answers.
// RFC 7296 Section 2.19: "That address persists at least until the deletion of the IKE
// SA." It is released once, by (*SA).forgetKeys, and moved to the successor by an IKE SA
// rekey (moveVirtualIP), never copied.
type virtualIPGrant struct {
	leases *eap.Registry
	pool   *eap.Pool
	held   []*eap.Lease // one per assigned family
	req    *cpRequest
}

// addr returns the leased address of family, if one is held.
func (g *virtualIPGrant) addr(family eap.Family) (netip.Addr, bool) {
	for _, l := range g.held {
		if l.Family() == family {
			return l.Addr(), true
		}
	}
	return netip.Addr{}, false
}

// assignVirtualIP grants the IKE SA of a pool-bound peer its inner addresses. It runs
// after authentication and before narrowChildSelectors, so an unauthenticated peer drains
// nothing and the selectors are narrowed with the lease. An unbound peer is untouched and
// any CP it sent is ignored.
//
// RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from occurring."
// The lease registry is the engine's one uniqueness authority, so no two live IKE SAs hold
// one inner address, and every Child SA of this IKE SA names the lease as its inner source.
//
// RFC 7296 Section 2.19: "IRAS MUST fail the request, and terminate the Child SA creation
// with a FAILED_CP_REQUIRED error." A bound peer that sent no address request is refused.
//
// RFC 7296 Section 3.15.4: "If the responder encounters an error while attempting to
// assign an IP address to the initiator during the processing of a Configuration payload,
// it responds with an INTERNAL_ADDRESS_FAILURE notification." And: "In this case, the
// responder simply ignores the type of address it does not support and processes the rest
// of the request as usual." A family the pool does not serve is skipped; only when no
// requested family is assigned is the request refused.
func assignVirtualIP(sa *SA, vip *virtualIPState) error {
	name := sa.PeerCfg.VirtualIPPoolName()
	if name == "" {
		return nil
	}
	if sa.leaseGrant() != nil {
		return nil
	}
	if sa.cpRequest == nil {
		return errCPRequired
	}
	if vip == nil {
		return fmt.Errorf("%w: the engine holds no lease registry for pool %q", errNoAddressAssigned, name)
	}
	pool, ok := vip.pool(name)
	if !ok {
		return fmt.Errorf("%w: pool %q is not built", errNoAddressAssigned, name)
	}

	grant := &virtualIPGrant{leases: vip.leases, pool: pool, req: sa.cpRequest}
	var lastErr error
	for _, f := range [...]struct {
		family    eap.Family
		want      bool
		requested netip.Addr
	}{
		{eap.FamilyIPv4, sa.cpRequest.want4, sa.cpRequest.req4},
		{eap.FamilyIPv6, sa.cpRequest.want6, sa.cpRequest.req6},
	} {
		if !f.want {
			continue
		}
		lease, err := vip.leases.Acquire(pool, sa.PeerName, f.family, f.requested)
		if err != nil {
			lastErr = err
			continue
		}
		grant.held = append(grant.held, lease)
	}
	if len(grant.held) == 0 {
		return fmt.Errorf("%w: pool %q: %w", errNoAddressAssigned, name, lastErr)
	}
	sa.setLeaseGrant(grant)
	return nil
}

// virtualIPMu guards (*SA).virtualIP of every SA. The owner goroutine writes it
// (assignVirtualIP, releaseVirtualIP, moveVirtualIP) while `show vpn ipsec sa` reads it
// ((*SA).VirtualIPs), and a field mutex would make the SA uncopyable (sa.go). One package
// lock is enough: the field changes once per IKE SA lifetime, never per packet.
var virtualIPMu sync.Mutex

// leaseGrant returns the lease set this IKE SA holds, nil when it holds none. Safe for
// concurrent use.
func (sa *SA) leaseGrant() *virtualIPGrant {
	virtualIPMu.Lock()
	defer virtualIPMu.Unlock()
	return sa.virtualIP
}

// setLeaseGrant publishes grant as the lease set this IKE SA holds. Safe for concurrent
// use.
func (sa *SA) setLeaseGrant(grant *virtualIPGrant) {
	virtualIPMu.Lock()
	defer virtualIPMu.Unlock()
	sa.virtualIP = grant
}

// takeLeaseGrant empties the SA's lease slot and returns what it held, nil when it held
// nothing, so a grant leaves the slot exactly once. Safe for concurrent use.
func (sa *SA) takeLeaseGrant() *virtualIPGrant {
	virtualIPMu.Lock()
	defer virtualIPMu.Unlock()
	grant := sa.virtualIP
	sa.virtualIP = nil
	return grant
}

// releaseVirtualIP hands the IKE SA's leases back to the registry, once. Called from
// (*SA).forgetKeys, the single close point of an SA.
func (sa *SA) releaseVirtualIP() {
	grant := sa.takeLeaseGrant()
	if grant == nil {
		return
	}
	for _, l := range grant.held {
		//nolint:errcheck // a lease this SA holds is released exactly once; ErrNotAllocated would be a double release, which takeLeaseGrant prevents
		grant.leases.Release(l)
	}
}

// moveVirtualIP hands the retired IKE SA's leases to its rekeyed successor, before the
// retired SA's forgetKeys. RFC 7296 Section 3.15.1: "The requested address is valid as
// long as this IKE SA (or its rekeyed successors) requesting the address is valid." The
// holder count is unchanged.
func moveVirtualIP(retired, successor *SA) {
	grant := retired.takeLeaseGrant()
	if grant == nil {
		return
	}
	successor.setLeaseGrant(grant)
}

// substituteVirtualIP narrows the peer's proposed inner selectors to its leased host
// addresses, keeping each proposal's protocol and ports, for a pool-bound peer. A bound
// peer whose IKE SA holds no lease gets no Child SA (strongSwan ike_sa_get_dynamic_hosts),
// because installing its self-chosen address would recreate the RFC 3948 Section 5.1
// conflict.
//
// RFC 7296 Section 2.9: "When the responder chooses a subset of the traffic proposed by
// the initiator, it narrows the Traffic Selectors to some subset of the initiator's
// proposal (provided the set does not become the null set)."
//
// The lease host is a subset of a proposed selector only when that selector contains it,
// so a selector that does not is dropped rather than replaced: answering the lease there
// would hand the peer traffic it never proposed. strongSwan as a gateway does the same,
// intersecting the selector resolved to the virtual IP with each received one
// (traffic_selector_list.c select_: "selected = ts1->get_subset(ts1, ts2);") and answering
// TS_UNACCEPTABLE when nothing is left. A peer that proposes no selector containing its
// lease is therefore refused here, before the policy match and the rekey floor judge the
// result, so both see only selectors the proposal genuinely covers.
func substituteVirtualIP(sa *SA, proposed []tsSelector) ([]tsSelector, error) {
	if sa.PeerCfg.VirtualIPPoolName() == "" {
		return proposed, nil
	}
	grant := sa.leaseGrant()
	if grant == nil {
		return nil, fmt.Errorf("%w: the peer holds no virtual IP", errTSUnacceptable)
	}
	out := make([]tsSelector, 0, len(proposed))
	for _, s := range proposed {
		host, ok := leaseHostIn(grant, s.Net)
		if !ok {
			continue
		}
		out = append(out, tsSelector{Net: host, Port: s.Port, Proto: s.Proto})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: the proposed TSi %s contains none of the peer's virtual IPs %v, "+
			"and RFC 7296 Section 2.9 lets the answer be only a subset of the proposal; the peer "+
			"must propose a TSi containing its assigned address, such as 0.0.0.0/0 or ::/0",
			errTSUnacceptable, selectorsText(proposed), sa.VirtualIPs())
	}
	return out, nil
}

// leaseHostIn returns the lease host of the selector's family as a host prefix, and true,
// when the selector contains it. The family is read from the mask length, as sameFamily
// does, so an IPv4-mapped IPv6 selector is of the IPv6 family and is never given the
// IPv4 lease. A selector with no prefix contains nothing.
func leaseHostIn(grant *virtualIPGrant, proposed *net.IPNet) (*net.IPNet, bool) {
	if proposed == nil {
		return nil, false
	}
	family := eap.FamilyIPv6
	if len(proposed.Mask) == net.IPv4len {
		family = eap.FamilyIPv4
	}
	addr, ok := grant.addr(family)
	if !ok {
		return nil, false
	}
	bits := len(proposed.Mask) * 8
	host := &net.IPNet{IP: net.IP(addr.AsSlice()), Mask: net.CIDRMask(bits, bits)}
	if !containsNet(proposed, host) {
		return nil, false
	}
	return host, true
}

// proposeLeasedChildTSPayloads builds the TSi/TSr of a Child SA request this node sends on
// a pool-bound peer's IKE SA, a rekey or a new Child SA, and records the proposal as the
// ceiling recordInitiatorSelectorsIn judges the answer against. This node sends Ni, so its
// own side is TSi and the peer's side, the lease, is TSr.
//
// RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from occurring."
// The configured remote prefix covers the whole pool, and the wildcard covers every
// address, so proposing either lets the peer answer an inner address another peer holds,
// which is the duplicate the lease registry exists to prevent. The peer's side is
// therefore proposed as its lease host, inside each configured remote prefix that
// contains it (leasedProposalPairs).
//
// When no pair survives, either the IKE SA holds no lease, or no configured remote prefix
// contains its lease. Commit refuses a pool outside the configured prefixes
// (ValidateVirtualIPPoolBindings), but a pool edit can move the range off a live lease, which
// stays held until the SA closes, so the second case is reachable. The wildcard is sent with
// no recorded ceiling. With no lease, answerNamesOnlyLeases refuses every answer. With a
// lease outside the configured prefixes, answerNamesOnlyLeases admits only an answer naming
// that lease host, and the configured policy, which recordInitiatorSelectorsIn falls back
// to as the ceiling, refuses it, because no configured prefix contains it. Every answer is
// therefore refused.
func proposeLeasedChildTSPayloads(sa *SA, pairs []tsPair) (*wire.PayloadTS, *wire.PayloadTS) {
	leased := leasedProposalPairs(sa.leaseGrant(), pairs)
	tsi, tsr := pairsToWire(leased)
	if tsi == nil || tsr == nil {
		sa.ProposedChildPairs = nil
		return anyChildTSPayloads(sa)
	}
	sa.ProposedChildPairs = leased
	return tsi, tsr
}

// leasedProposalPairs narrows the peer's side (R) of each proposed pair to the lease host
// that side contains, keeping its protocol and ports, and drops a pair whose peer side
// contains no lease. A peer with no configured selector proposes the wildcard, so each
// held lease is paired with this node's wildcard of the same family. It answers nil for a
// grant that holds nothing.
//
// RFC 7296 Section 2.9: "When the responder chooses a subset of the traffic proposed by
// the initiator, it narrows the Traffic Selectors to some subset of the initiator's
// proposal (provided the set does not become the null set)." The lease host is proposed
// only where the configured policy contains it, so the request never asks for traffic the
// policy does not allow.
func leasedProposalPairs(grant *virtualIPGrant, pairs []tsPair) []tsPair {
	if grant == nil {
		return nil
	}
	if len(pairs) == 0 {
		pairs = wildcardLeasePairs(grant)
	}
	out := make([]tsPair, 0, len(pairs))
	for _, p := range pairs {
		host, ok := leaseHostIn(grant, p.R.Net)
		if !ok {
			continue
		}
		out = append(out, tsPair{I: p.I, R: tsSelector{Net: host, Port: p.R.Port, Proto: p.R.Proto}})
	}
	return out
}

// wildcardLeasePairs returns one wildcard pair per family the grant holds a lease of, the
// pair a peer with no configured selector proposes before leasedProposalPairs narrows its
// peer side to the lease.
func wildcardLeasePairs(grant *virtualIPGrant) []tsPair {
	out := make([]tsPair, 0, len(grant.held))
	for _, l := range grant.held {
		bits := 8 * net.IPv4len
		if l.Family() == eap.FamilyIPv6 {
			bits = 8 * net.IPv6len
		}
		anyNet := tsSelector{Net: &net.IPNet{IP: make(net.IP, bits/8), Mask: net.CIDRMask(0, bits)}, Port: ipsec.AnyPort()}
		out = append(out, tsPair{I: anyNet, R: anyNet})
	}
	return out
}

// answerNamesOnlyLeases refuses an answer to a Child SA request this node sent on a
// pool-bound peer's IKE SA unless the peer's side (R) of every answered pair is exactly
// one of the peer's lease hosts. It runs whatever the recorded ceiling is, so an empty
// sa.ProposedChildPairs, which falls back to the configured policy, cannot admit the pool
// prefix or the wildcard.
//
// RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from occurring."
// A Child SA whose peer side is wider than the lease would route to this peer the inner
// addresses the registry leased to others.
func answerNamesOnlyLeases(sa *SA, pairs []tsPair) error {
	if sa.PeerCfg.VirtualIPPoolName() == "" {
		return nil
	}
	grant := sa.leaseGrant()
	if grant == nil {
		return fmt.Errorf("%w: the peer is bound to virtual-ip-pool %q and its IKE SA holds no lease, "+
			"so no answered selector can be its inner address", errTSUnusable, sa.PeerCfg.VirtualIPPoolName())
	}
	for _, p := range pairs {
		if _, ok := leaseHostIn(grant, p.R.Net); !ok {
			return fmt.Errorf("%w: the peer answered its side as %v, which holds none of its virtual IPs %v",
				errTSWidened, p.R.Net, sa.VirtualIPs())
		}
		if ones, bits := p.R.Net.Mask.Size(); ones != bits {
			return fmt.Errorf("%w: the peer answered its side as %v, wider than its virtual IPs %v, "+
				"so the Child SA would carry inner addresses leased to other peers (RFC 3948 Section 5.1)",
				errTSWidened, p.R.Net, sa.VirtualIPs())
		}
	}
	return nil
}

// buildCFGReply builds the CFG_REPLY of a granted request.
//
// RFC 7296 Section 3.15.1: "Only one netmask is allowed in the request and response
// messages (e.g., 255.255.255.0), and it MUST be used only with an INTERNAL_IP4_ADDRESS
// attribute." The netmask is sent only when asked and only beside an IPv4 address.
//
// RFC 7296 Section 2.20: "In that case, it MUST either return an empty string or no CP
// payload if CP is not supported." Ze declines to give out its version: a requested
// APPLICATION_VERSION is answered with an empty value.
func buildCFGReply(grant *virtualIPGrant) *wire.PayloadCP {
	cp := &wire.PayloadCP{CFGType: wire.CFGTypeReply}
	if addr, ok := grant.addr(eap.FamilyIPv4); ok {
		a := addr.As4()
		cp.Attrs = append(cp.Attrs, wire.ConfigAttr{Type: wire.CPAttrInternalIP4Address, Value: a[:]})
		if grant.req.netmask {
			mask := net.CIDRMask(grant.pool.Prefix4().Bits(), 32)
			cp.Attrs = append(cp.Attrs, wire.ConfigAttr{Type: wire.CPAttrInternalIP4Netmask, Value: mask})
		}
		for _, d := range grant.pool.DNS(eap.FamilyIPv4) {
			v := d.As4()
			cp.Attrs = append(cp.Attrs, wire.ConfigAttr{Type: wire.CPAttrInternalIP4DNS, Value: v[:]})
		}
	}
	if addr, ok := grant.addr(eap.FamilyIPv6); ok {
		a := addr.As16()
		value := append(a[:], byte(grant.pool.Prefix6().Bits()))
		cp.Attrs = append(cp.Attrs, wire.ConfigAttr{Type: wire.CPAttrInternalIP6Address, Value: value})
		for _, d := range grant.pool.DNS(eap.FamilyIPv6) {
			v := d.As16()
			cp.Attrs = append(cp.Attrs, wire.ConfigAttr{Type: wire.CPAttrInternalIP6DNS, Value: v[:]})
		}
	}
	if grant.req.appVersion {
		cp.Attrs = append(cp.Attrs, wire.ConfigAttr{Type: wire.CPAttrApplicationVersion})
	}
	if grant.req.supported {
		cp.Attrs = append(cp.Attrs, wire.ConfigAttr{Type: wire.CPAttrSupportedAttributes, Value: supportedAttributes()})
	}
	return cp
}

// supportedAttributes lists the handled attribute types, two octets each, derived from
// cpHandlers (RFC 7296 Section 3.15.1, SUPPORTED_ATTRIBUTES).
func supportedAttributes() []byte {
	out := make([]byte, 0, 2*len(cpHandlers))
	for i := range cpHandlers {
		out = append(out, byte(cpHandlers[i].attr>>8), byte(cpHandlers[i].attr))
	}
	return out
}

// VirtualIPs returns the inner addresses this IKE SA leased to its peer, for
// `show vpn ipsec sa` (virtual-ip). Nil when it holds none. Safe for concurrent use with
// the owner goroutine's release and move: it reads the grant once (leaseGrant).
func (sa *SA) VirtualIPs() []netip.Addr {
	grant := sa.leaseGrant()
	if grant == nil {
		return nil
	}
	out := make([]netip.Addr, 0, len(grant.held))
	for _, l := range grant.held {
		out = append(out, l.Addr())
	}
	return out
}

// LeaseVirtualIPForTest binds sa to pool and grants it a lease of each family the pool
// serves, through assignVirtualIP and a registry holding only that pool, so a test outside
// this package (`show vpn ipsec sa`) reads a leased SA. The daemon never calls it.
func LeaseVirtualIPForTest(sa *SA, pool ipsec.VirtualIPPool) error {
	vip := newVirtualIPState()
	vip.reload(&ipsec.RemoteAccessConfig{Pools: map[string]ipsec.VirtualIPPool{pool.Name: pool}}, slogutil.DiscardLogger())
	sa.PeerCfg.VirtualIP = &ipsec.VirtualIPBinding{Pool: pool.Name}
	sa.cpRequest = &cpRequest{want4: pool.Range != "", want6: pool.Range6 != ""}
	return assignVirtualIP(sa, vip)
}
