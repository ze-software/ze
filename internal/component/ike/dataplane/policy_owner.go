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
// A node-wide template-free entry (a catch-all, an operator BYPASS or DISCARD, an
// interface exemption) has no owner record: it carries no per-peer identity, and two
// node-wide installs of one selector are one installer re-asserting its entry. The
// template-free entries a peer installs are owned (ownedEntry): a tunnel Child SA's
// outbound forward allow and its delivery allow (engine/child.go
// childOutboundForwardPolicy, appendChildDeliveryPolicy).
//
// A node-wide entry and an owned one still meet in the kernel, which tells policies
// apart by selector, direction and if_id and never by action or priority
// (xfrm_policy_insert_list). An operator entry's transit gate can name the selector of a
// peer's OUT PROTECT policy, and its upsert would then replace the PROTECT with an
// allow, so the peer's traffic would leave in clear. So claim and deleteThenRelease
// refuse, in both directions:
//
//   - a node-wide install or delete at a selector an owner holds, whatever the node-wide
//     entry's role (the backend asks yieldFinal first for an SPD's final entry, which
//     yields to a PROTECT policy instead of replacing it);
//   - an owned install or delete at a selector an operator entry holds
//     (operatorHeldLocked).
//
// An owned PROTECT install over a catch-all is not refused: a peer that negotiates the
// wildcard of a family in an SPD installs a PROTECT policy on the catch-all's own
// selector, which matches every packet the catch-all would. The kernel then holds one of
// the two, so the PROTECT policy stands in for the SPD's final entry while it is live,
// and the final entry is kept here (finals) and put back, in the same kernel request
// that takes the PROTECT policy out, when its owner removes it (policy_final.go).
//
// It also keeps, for EVERY entry Ze installed, template-free ones included, the role
// and SPD name the installer gave it (labels), so a read can say which job and which
// SPD each kernel row has. The label follows the kernel's lifetime of the entry, written
// once the kernel accepted the install and dropped once the kernel confirmed the
// removal, which is why it is also the record of the operator entries the kernel holds.
//
// Safe for concurrent use: one PeerSession goroutine per peer reaches it, and the apply
// goroutine for the node-wide entries. The backend's own lock (xfrmBackend.mu) spans
// claim, the kernel call and label, so no claim runs between an install and its label.
type policyOwners struct {
	mu     sync.Mutex
	owners map[policySelectorKey]policyHolder
	labels map[policySelectorKey]policyLabel
	// finals holds the final entry Ze was last asked to keep at each selector (the
	// node-wide catch-all, an XFRM interface's catch-all), as it was asked for. The
	// kernel holds that entry, or, where owners has the selector too, the PROTECT
	// policy standing in for it (policy_final.go).
	finals map[policySelectorKey]SPParams
}

// policyLabel is what an installer said about one entry: why it built it and which
// XFRM interface's SPD holds it (SPParams.Role, SPParams.SPD).
type policyLabel struct {
	role PolicyRole
	spd  string
}

// label records p's role and SPD name against p's selector. The caller MUST call it
// only after the kernel accepted p: a label written before would outlive a refused
// install and name an entry the kernel does not hold. A re-install of the same
// selector (a Child SA rekey, a re-asserted catch-all) overwrites the label.
func (o *policyOwners) label(p SPParams) {
	key := policyKey(p)

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.labels == nil {
		o.labels = make(map[policySelectorKey]policyLabel)
	}
	o.labels[key] = policyLabel{role: p.Role, spd: p.SPD}
	if finalEntry(p) {
		if o.finals == nil {
			o.finals = make(map[policySelectorKey]SPParams)
		}
		o.finals[key] = p
		return
	}
	if !ownedEntry(p) {
		// Another node-wide entry took the selector, so the kernel no longer holds the
		// final entry there and nothing stands in for it.
		delete(o.finals, key)
	}
}

// annotate joins one kernel row back to what Ze recorded for its selector: the owner
// of a template-carrying entry, and the role and SPD name of any entry Ze installed.
// A row Ze holds no record for keeps OwnerKnown false and PolicyRoleUnspecified,
// which a renderer shows as "unknown" (ai/rules/evidence.md).
func (o *policyOwners) annotate(info *PolicyInfo) {
	key := policyKey(SPParams{
		Src:        info.Src,
		Dst:        info.Dst,
		Dir:        info.Dir,
		UpperProto: info.UpperProto,
		SrcPort:    info.SrcPort,
		DstPort:    info.DstPort,
		IfIndex:    info.IfIndex,
		IfID:       info.IfID,
	})

	o.mu.Lock()
	defer o.mu.Unlock()
	if held, ok := o.owners[key]; ok {
		info.Owner, info.OwnerKnown = held.owner, true
	}
	if label, ok := o.labels[key]; ok {
		info.Role, info.SPDInterface = label.role, label.spd
	}
	if final, kept := o.finals[key]; kept {
		if _, standsIn := o.owners[key]; standsIn {
			info.Displaced = DisplacedFinalEntry{Role: final.Role, Action: final.Action}
		}
	}
}

// ownedEntry reports whether the ledger records p's owner and refuses another owner's
// install or delete of p's selector: every template-carrying policy, and a template-free
// entry a Child SA installs (PolicyRoleChildSA, its outbound forward and delivery
// allows), which belongs to one peer as its PROTECT policies do. Every other
// template-free entry is node-wide: it has no owner record, and claim and
// deleteThenRelease refuse it at a selector an owner holds.
func ownedEntry(p SPParams) bool {
	if !p.Action.isTemplateFree() {
		return true
	}
	return p.Role == PolicyRoleChildSA
}

// policyHolder is the record for one claimed selector: who installed it, whether it
// is a transport-mode policy, which claim compares for overlap as well as identity, and
// whether it protects, which is what lets it stand in for an SPD's final entry
// (yieldFinal).
type policyHolder struct {
	owner     string
	transport bool
	protects  bool
}

// claim records p's owner against p's selector, and REFUSES when a different owner
// already holds it, or, for a transport-mode policy, when a different owner holds a
// transport-mode selector that overlaps it.
//
// The same owner re-claiming its own selector succeeds and is what a Child SA rekey
// does: newRekeyedChild (engine/rekey.go) inherits every selector field from the
// retired pair, so the replacement's install is an upsert of the same policy.
//
// A node-wide entry (ownedEntry false) creates no record, and is REFUSED at a selector
// an owner holds; an owned entry is refused at a selector an operator entry holds. Both
// are the same collision seen from its two sides (policyOwners).
//
// The first result reports whether this call CREATED the record. A caller that must
// undo a failed install may release only what it created: releasing a record that
// already existed would forget a policy the kernel still holds, and the next foreign
// peer would then be allowed to take that live selector over. Answered under the same
// lock as the write, so it cannot disagree with it.
func (o *policyOwners) claim(p SPParams) (created bool, err error) {
	key := policyKey(p)

	o.mu.Lock()
	defer o.mu.Unlock()
	held, existed := o.owners[key]
	if !ownedEntry(p) {
		// RFC 4301 Section 4.4.1: "An SPD must discriminate among traffic that is afforded
		// IPsec protection and traffic that is allowed to bypass IPsec."
		//
		// The kernel replaces the policy that holds this selector whatever its priority,
		// so a node-wide BYPASS or DISCARD written here would take the place of the
		// owner's entry: its PROTECT policy, or the allow that hands its decrypted
		// traffic on. The owner's entry stays, and the node-wide one is refused.
		if existed {
			return false, &PolicyOwnedError{Selector: key, HeldBy: held.owner, WantedRole: p.Role}
		}
		return false, nil
	}
	if existed && held.owner != p.Owner {
		return false, &PolicyOwnedError{Selector: key, HeldBy: held.owner, Wanted: p.Owner}
	}
	if !existed {
		// RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order
		// the entries to express a desired access control policy."
		//
		// The operator's entry and this one would be one kernel policy, so no order
		// between them could be kept: this install would replace the operator's
		// disposition, and this owner's teardown would delete it.
		if o.operatorHeldLocked(key) {
			return false, &PolicyOwnedError{Selector: key, HeldRole: PolicyRoleOperator, Wanted: p.Owner}
		}
		// RFC 4301 Section 5: "If no policy is found in the SPD that matches a packet
		// (for either inbound or outbound traffic), the packet MUST be discarded."
		//
		// A PROTECT policy may take the place of the final entry, because it leaves no
		// packet of that lookup unmatched and passes none in clear. A template-free
		// allow of a peer may not: it would replace the final entry with a bypass of
		// everything. No Child SA builds one on the wildcard (engine/child.go
		// appendChildDeliveryPolicy, appendChildOutboundForwardPolicy), so this refuses
		// only a Ze defect.
		if final, kept := o.finals[key]; kept && p.Action.isTemplateFree() {
			return false, &PolicyOwnedError{Selector: key, HeldRole: final.Role, Wanted: p.Owner}
		}
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
		// client's traffic to the other client's SA. Tunnel mode is not compared here:
		// an overlap between operator-configured prefixes ordered by priority is a
		// deliberate configuration, and the overlap a PEER chose, between two tunnels or
		// between a tunnel and a transport-mode connection, is refused by the IKE engine
		// before any backend is called (RFC 3948 Section 5.1,
		// ike/engine/inner_selectors.go), which reaches every backend rather than this
		// one.
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
	o.owners[key] = policyHolder{owner: p.Owner, transport: transport, protects: !p.Action.isTemplateFree()}
	return !existed, nil
}

// operatorHeldLocked reports whether the kernel holds an operator `vpn ipsec policy`
// entry at key: a label with the operator role and no owner record. The caller holds
// o.mu.
//
// Only the operator role is asked for. A catch-all on the same selector yields to an
// owner's PROTECT policy (policyOwners), and no other node-wide role shares a selector
// with a Child SA policy: an interface exemption names an interface index, which no
// Child SA policy carries.
func (o *policyOwners) operatorHeldLocked(key policySelectorKey) bool {
	if _, owned := o.owners[key]; owned {
		return false
	}
	label, labeled := o.labels[key]
	if !labeled {
		return false
	}
	return label.role == PolicyRoleOperator
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

// PolicySelectorsEqual reports whether the two policies are ONE policy to the kernel:
// the same selector, direction and if_id (policyKey), whatever their action, priority,
// template and owner. The second of two such policies to be installed replaces the
// first. It is the key claim refuses on, exported so the IKE engine's commit check
// (engine/spd_order.go) and this backend cannot disagree on what one policy is. Pure,
// and safe for concurrent use.
func PolicySelectorsEqual(a, b SPParams) bool {
	return policyKey(a) == policyKey(b)
}

// PolicySelectorsOverlap reports whether one packet can match the selectors of both
// policies, judged on what the kernel identifies a policy by (policyKey) and with the
// predicate the transport-mode refusal uses (selectorsOverlap). Endpoints, owner and
// priority do not take part. It is the IKE engine's cross-peer tunnel check
// (RFC 3948 Section 5.1), exported so the engine and this backend cannot disagree on
// what an overlap is. Pure, and safe for concurrent use.
func PolicySelectorsOverlap(a, b SPParams) bool {
	return selectorsOverlap(policyKey(a), policyKey(b))
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
	if !ownedEntry(p) {
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
// not reach the kernel. The same holds across the two kinds of entry, for the reason
// claim gives: a node-wide delete at a selector an owner holds would remove the owner's
// policy (an operator entry that leaves the configuration would take a peer's PROTECT
// policy with it), and an owned delete at a selector an operator entry holds would
// remove the operator's entry (the teardown of a Child SA whose install claim refused).
func (o *policyOwners) deleteThenRelease(p SPParams, del func() error) error {
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

	held, existed := o.owners[key]
	if !ownedEntry(p) {
		if existed {
			if _, displaced := o.finals[key]; displaced && finalEntry(p) {
				// The kernel holds the owner's PROTECT policy here, not this final entry,
				// so there is nothing to delete: the entry is forgotten, and the owner's
				// removal then deletes its policy and puts no final entry back.
				delete(o.finals, key)
				return nil
			}
			// A refused delete must not reach the kernel.
			return &PolicyOwnedError{Selector: key, HeldBy: held.owner, WantedRole: p.Role}
		}
		delErr := del()
		if delErr != nil && !errors.Is(delErr, syscall.ENOENT) {
			// The kernel still holds the entry, so its label stays.
			return delErr
		}
		delete(o.labels, key)
		delete(o.finals, key)
		return delErr
	}
	if existed && held.owner != p.Owner {
		// A refused delete must not reach the kernel.
		return &PolicyOwnedError{Selector: key, HeldBy: held.owner, Wanted: p.Owner}
	}
	if o.operatorHeldLocked(key) {
		// A refused delete must not reach the kernel.
		return &PolicyOwnedError{Selector: key, HeldRole: PolicyRoleOperator, Wanted: p.Owner}
	}
	delErr := del()
	if delErr != nil && !errors.Is(delErr, syscall.ENOENT) {
		// The kernel still holds the policy, so the record has to as well.
		return delErr
	}
	delete(o.owners, key)
	delete(o.labels, key)
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
	for key := range o.labels {
		if key.src == selector.src && key.dst == selector.dst && key.dir == selector.dir {
			delete(o.labels, key)
		}
	}
	for key := range o.finals {
		if key.src == selector.src && key.dst == selector.dst && key.dir == selector.dir {
			delete(o.finals, key)
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
// from a netlink failure: this one means the selector already carries somebody else's
// entry, a peer's tunnel or an operator's disposition, and retrying will never help.
//
// Each side is an owner or a node-wide entry. HeldBy and Wanted name an owner. HeldRole
// and WantedRole name a node-wide entry by its role, and stay PolicyRoleUnspecified for
// a side that is an owner's, so at most one of the two roles is set: two node-wide
// entries never refuse each other.
type PolicyOwnedError struct {
	Selector   policySelectorKey
	HeldBy     string
	Wanted     string
	HeldRole   PolicyRole
	WantedRole PolicyRole
}

func (e *PolicyOwnedError) Error() string {
	var b textbuf.Buffer
	b.Str("xfrm: policy selector ").Str(e.Selector.String())
	b.Str(" is installed for ").Str(installerLabel(e.HeldBy, e.HeldRole))
	b.Str(", so it cannot also be installed for ").Str(installerLabel(e.Wanted, e.WantedRole))
	if e.HeldRole == PolicyRoleUnspecified && e.WantedRole == PolicyRoleUnspecified {
		b.Str("; the kernel identifies a policy by its selector alone, so one selector carries one tunnel")
		return b.String()
	}
	b.Str("; the kernel identifies a policy by its selector, direction and if_id, whatever its priority, " +
		"so the second entry would replace the first")
	if e.HeldRole == PolicyRoleOperator || e.WantedRole == PolicyRoleOperator {
		b.Str("; delete or narrow the `vpn ipsec policy` entry that names this selector")
	}
	return b.String()
}

// installerLabel names one side of a PolicyOwnedError: a node-wide entry by its role,
// or an owner.
func installerLabel(owner string, role PolicyRole) string {
	if role == PolicyRoleOperator {
		return "an operator `vpn ipsec policy` entry"
	}
	if role != PolicyRoleUnspecified {
		return "Ze's node-wide " + role.String() + " entry"
	}
	return ownerLabel(owner)
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
