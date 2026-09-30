// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// RFC: rfc/short/rfc4301.md -- SPD entries are keyed by their selector
// Related: xfrm_linux.go -- the backend that claims and releases through this registry

package dataplane

import (
	"errors"
	"net"
	"sync"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// policySelectorKey is everything the kernel uses to tell one policy from another.
//
// It mirrors the fields xfrmPolicyFromParams writes into the netlink selector, because
// the kernel identifies a policy by its whole selector and by nothing else
// (net/xfrm/xfrm_policy.c, xfrm_policy_bysel_ctx). Two SPParams that agree on every
// field below describe ONE kernel policy, whoever installed them.
//
// The template is deliberately absent. Tunnel endpoints, reqid and priority do not
// participate in the kernel's identity, so two peers whose templates differ still
// collide, and that collision is exactly what this key exists to detect.
type policySelectorKey struct {
	src        string
	dst        string
	dir        SADir
	upperProto uint8
	srcPort    PortMatch
	dstPort    PortMatch
	ifIndex    int
	ifID       uint32
}

func policyKey(p SPParams) policySelectorKey {
	// selFromPolicy selects the destination's family, defaulting to IPv4.
	ipv6 := false
	if p.Dst != nil {
		ipv6 = len(p.Dst.IP) > net.IPv4len && p.Dst.IP.To4() == nil
	}
	return policySelectorKey{
		src:        cidrKey(p.Src, ipv6),
		dst:        cidrKey(p.Dst, ipv6),
		dir:        p.Dir,
		upperProto: p.UpperProto,
		srcPort:    p.SrcPort,
		dstPort:    p.DstPort,
		ifIndex:    p.IfIndex,
		ifID:       p.IfID,
	}
}

// cidrKey materializes a nil selector in the family the kernel writer chooses.
// Explicit /0 prefixes retain their family and compare equal to that wildcard.
func cidrKey(n *net.IPNet, ipv6 bool) string {
	if n == nil {
		if ipv6 {
			return "::/0"
		}
		return "0.0.0.0/0"
	}
	return n.String()
}

// policyOwners records who installed each policy selector, so a backend that UPSERTS
// can still refuse a takeover.
//
// SPParams.Owner carries the reason this exists: a peer's policy has no per-peer
// identity in the kernel, so two site-to-site peers negotiating 0.0.0.0/0 install one
// policy between them. The upsert xfrmBackend.InstallPolicy needs for a Child SA rekey
// then lets the second peer to establish capture the first peer's traffic, and lets
// either peer's teardown blackhole the survivor.
//
// A bypass policy is NOT tracked. Every peer installs the same four by design
// (xfrmBackend.InstallPolicy), they are byte-identical, and they carry no per-peer
// identity to conflict over.
//
// Safe for concurrent use: one PeerSession goroutine per peer reaches it.
type policyOwners struct {
	mu     sync.Mutex
	owners map[policySelectorKey]policyHolder
}

// policyHolder is the record for one claimed selector: who installed it, and whether
// it is a transport-mode policy, which claim compares for overlap as well as identity.
type policyHolder struct {
	owner     string
	transport bool
}

// claim records p's owner against p's selector, and REFUSES when a different owner
// already holds it, or, for a transport-mode policy, when a different owner holds a
// transport-mode selector that overlaps it.
//
// The same owner re-claiming its own selector succeeds and is what a Child SA rekey
// does: newRekeyedChild (engine/rekey.go) inherits every selector field from the
// retired pair, so the replacement's install is an upsert of the same policy.
//
// The first result reports whether this call CREATED the record. A caller that must
// undo a failed install may release only what it created: releasing a record that
// already existed would forget a policy the kernel still holds, and the next foreign
// peer would then be allowed to take that live selector over. Answered under the same
// lock as the write, so it cannot disagree with it.
func (o *policyOwners) claim(p SPParams) (created bool, err error) {
	if p.Action.isTemplateFree() {
		return false, nil
	}
	key := policyKey(p)

	o.mu.Lock()
	defer o.mu.Unlock()
	held, existed := o.owners[key]
	if existed && held.owner != p.Owner {
		return false, &PolicyOwnedError{Selector: key, HeldBy: held.owner, Wanted: p.Owner}
	}
	transport := p.Mode == ModeTransport
	if transport {
		// RFC 3948 Section 5.2: "If the <traffic desc1> overlaps <traffic desc2>, then
		// simple filter lookups may not be sufficient to determine which SA has to be
		// used to send traffic. Implementations MUST handle this situation, either by
		// disallowing conflicting connections, or by other means."
		//
		// Ze disallows the connection. Two clients behind one NAT reach this node from
		// one address, so their transport selectors differ only in the protocol and
		// ports, and an overlap would leave the kernel's ordered search to hand one
		// client's traffic to the other client's SA. Tunnel mode is not compared: its
		// inner selectors are the operator's prefixes, where an overlap ordered by
		// priority is a deliberate configuration.
		for other, holder := range o.owners {
			if !holder.transport {
				continue
			}
			if holder.owner == p.Owner {
				continue
			}
			if selectorsOverlap(key, other) {
				return false, &TransportSelectorConflictError{
					Selector: key, Overlaps: other, HeldBy: holder.owner, Wanted: p.Owner,
				}
			}
		}
	}
	if o.owners == nil {
		o.owners = make(map[policySelectorKey]policyHolder)
	}
	o.owners[key] = policyHolder{owner: p.Owner, transport: transport}
	return !existed, nil
}

// selectorsOverlap reports whether one packet can match both selectors: the same
// direction and if_id, overlapping address prefixes on both sides, a protocol either
// side leaves open or both name, and port matches that agree on every bit both masks
// test. An interface index of zero matches every interface.
func selectorsOverlap(a, b policySelectorKey) bool {
	if a.dir != b.dir {
		return false
	}
	if a.ifID != b.ifID {
		return false
	}
	if a.ifIndex != 0 && b.ifIndex != 0 && a.ifIndex != b.ifIndex {
		return false
	}
	if a.upperProto != 0 && b.upperProto != 0 && a.upperProto != b.upperProto {
		return false
	}
	if !portsOverlap(a.srcPort, b.srcPort) {
		return false
	}
	if !portsOverlap(a.dstPort, b.dstPort) {
		return false
	}
	if !prefixesOverlap(a.src, b.src) {
		return false
	}
	return prefixesOverlap(a.dst, b.dst)
}

// portsOverlap reports whether some port satisfies both masked matches. The kernel
// matches (port ^ selector.port) & mask == 0, so two matches share a port exactly
// when they agree on every bit both masks test.
func portsOverlap(a, b PortMatch) bool {
	return (a.Port^b.Port)&a.Mask&b.Mask == 0
}

// prefixesOverlap reports whether two CIDR keys share an address. Two prefixes
// overlap exactly when one contains the other's network address. A key that does not
// parse is a Ze defect, since cidrKey wrote it, and is treated as overlapping so the
// refusal fails closed.
func prefixesOverlap(a, b string) bool {
	_, an, errA := net.ParseCIDR(a)
	if errA != nil {
		return true
	}
	_, bn, errB := net.ParseCIDR(b)
	if errB != nil {
		return true
	}
	return an.Contains(bn.IP) || bn.Contains(an.IP)
}

// release reports whether the caller may delete p's policy from the kernel, and drops
// the record when it may.
//
// It refuses a delete for a selector a DIFFERENT owner holds. That refusal is the
// second half of the same defect: installChildSA rolls back a failed policy install by
// removing the policy of the other direction, so a peer whose install was refused
// would otherwise take the owning peer's live policy down on its way out.
func (o *policyOwners) release(p SPParams) error {
	if p.Action.isTemplateFree() {
		return nil
	}
	key := policyKey(p)

	o.mu.Lock()
	defer o.mu.Unlock()
	if held, ok := o.owners[key]; ok && held.owner != p.Owner {
		return &PolicyOwnedError{Selector: key, HeldBy: held.owner, Wanted: p.Owner}
	}
	delete(o.owners, key)
	return nil
}

// deleteThenRelease runs one owner-checked policy delete, and drops the record only once
// the kernel has confirmed the policy is gone.
//
// THE ORDER IS THE GUARD. release above both CHECKS the owner and DROPS the record, so a
// caller that released first and then asked the kernel held no record over a policy that
// was still installed whenever the kernel refused. That is a guard failing OPEN: the next
// foreign peer's claim finds no owner, succeeds, and upserts over a LIVE tunnel's policy
// -- the takeover SPParams.Owner exists to refuse (ai/rules/evidence.md).
//
// Checking without releasing closes that window rather than compensating for it. The
// record outlives the kernel policy, never the other way round, so there is no instant at
// which the selector looks free while it is not.
//
// ENOENT is the one refusal that says the kernel holds no such policy. The record is then
// dropped, because keeping it would refuse a later, legitimate install of that selector.
//
// del is not called at all for a selector a DIFFERENT owner holds: a refused delete must
// not reach the kernel.
func (o *policyOwners) deleteThenRelease(p SPParams, del func() error) error {
	if p.Action.isTemplateFree() {
		return del()
	}
	key := policyKey(p)

	// The lock spans del, so check, delete and drop are one atomic step.
	//
	// An unlocked delete leaves the selector unowned for the duration of the syscall
	// whenever no record is held. A second peer's claim then succeeds inside that
	// window, this delete removes the newcomer's fresh policy, and the newcomer keeps a
	// record for a policy the kernel no longer has: states installed, outbound policy
	// gone, traffic in the clear.
	//
	// The cost is one netlink round trip under the lock. Contention is one PeerSession
	// goroutine per peer, so the wait is bounded by the kernel call.
	o.mu.Lock()
	defer o.mu.Unlock()

	if held, ok := o.owners[key]; ok && held.owner != p.Owner {
		// A refused delete must not reach the kernel.
		return &PolicyOwnedError{Selector: key, HeldBy: held.owner, Wanted: p.Owner}
	}
	delErr := del()
	if delErr != nil && !errors.Is(delErr, syscall.ENOENT) {
		// The kernel still holds the policy, so the record has to as well.
		return delErr
	}
	delete(o.owners, key)
	// The ENOENT still reaches the caller. It is a fact about the kernel -- ze asked it
	// to remove a policy it did not have -- and swallowing it would make that
	// indistinguishable from a routine removal. Only the RECORD's fate turns on it.
	return delErr
}

// releaseBySelector forgets every record whose source, destination and direction match,
// whatever the rest of the selector holds.
//
// It backs the three-argument RemovePolicy, which carries no owner and no upper-layer
// selector and so cannot name one record. Forgetting the matching ones keeps the
// registry from outliving the kernel policies: a record left behind would refuse a
// later, legitimate install of the same selector.
func (o *policyOwners) releaseBySelector(src, dst *net.IPNet, dir SADir) {
	selector := policyKey(SPParams{Src: src, Dst: dst, Dir: dir})

	o.mu.Lock()
	defer o.mu.Unlock()
	for key := range o.owners {
		if key.src == selector.src && key.dst == selector.dst && key.dir == selector.dir {
			delete(o.owners, key)
		}
	}
}

// ownerOf reports the recorded owner of p's selector, and whether one exists.
func (o *policyOwners) ownerOf(p SPParams) (string, bool) {
	key := policyKey(p)

	o.mu.Lock()
	defer o.mu.Unlock()
	held, ok := o.owners[key]
	return held.owner, ok
}

// PolicyOwnedError reports that a policy selector belongs to a different installer.
//
// It is a named type rather than a formatted string so a caller can tell this refusal
// from a netlink failure: this one means the traffic is already protected by somebody
// else's tunnel, and retrying will never help.
type PolicyOwnedError struct {
	Selector policySelectorKey
	HeldBy   string
	Wanted   string
}

func (e *PolicyOwnedError) Error() string {
	var b textbuf.Buffer
	b.Str("xfrm: policy selector ").Str(e.Selector.String())
	b.Str(" is installed for ").Str(ownerLabel(e.HeldBy))
	b.Str(", so it cannot also be installed for ").Str(ownerLabel(e.Wanted))
	b.Str("; the kernel identifies a policy by its selector alone, so one selector carries one tunnel")
	return b.String()
}

// TransportSelectorConflictError reports a transport-mode policy whose selector
// overlaps a transport-mode policy a DIFFERENT installer holds.
//
// It is the RFC 3948 Section 5.2 conflict: two clients behind one NAT reach the server
// from one address, so their transport SAs differ only in the protocol and port
// description, and an overlap leaves the kernel's ordered search to pick one client's
// SA for the other client's traffic. Ze disallows the second connection.
type TransportSelectorConflictError struct {
	Selector policySelectorKey
	Overlaps policySelectorKey
	HeldBy   string
	Wanted   string
}

func (e *TransportSelectorConflictError) Error() string {
	var b textbuf.Buffer
	b.Str("xfrm: transport-mode policy selector ").Str(e.Selector.String())
	b.Str(" for ").Str(ownerLabel(e.Wanted))
	b.Str(" overlaps ").Str(e.Overlaps.String())
	b.Str(" installed for ").Str(ownerLabel(e.HeldBy))
	b.Str("; two transport SAs to one address with overlapping traffic descriptions cannot be told apart (RFC 3948 Section 5.2)")
	return b.String()
}

// String renders every field the kernel compares, so a refusal names the selector a
// reader can look for with `ip xfrm policy`. Printing a subset would report two
// genuinely different policies as the same one.
func (k policySelectorKey) String() string {
	var b textbuf.Buffer
	b.Str(selectorSide(k.src)).Str(" -> ").Str(selectorSide(k.dst))
	b.Str(" dir=").Uint8(uint8(k.dir))
	b.Str(" proto=").Uint8(k.upperProto)
	b.Str(" sport=").Uint16(k.srcPort.Port).Byte('/').Uint16(k.srcPort.Mask)
	b.Str(" dport=").Uint16(k.dstPort.Port).Byte('/').Uint16(k.dstPort.Mask)
	b.Str(" ifindex=").Int(int64(k.ifIndex))
	b.Str(" if_id=").Uint32(k.ifID)
	return b.String()
}

// selectorSide names the wildcard rather than printing an empty span, so a 0.0.0.0/0
// site-to-site selector is legible in the refusal it causes.
func selectorSide(side string) string {
	if side == "" {
		return "any"
	}
	return side
}

// ownerLabel renders an owner for an error message, naming the empty one rather than
// printing nothing where a reader expects a name.
func ownerLabel(owner string) string {
	if owner == "" {
		return "an unnamed installer"
	}
	var b textbuf.Buffer
	return b.Str("peer ").Quoted(owner).String()
}
