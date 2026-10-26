// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the inner-selector registry
// RFC: rfc/short/rfc3948.md -- no two peers' tunnels for one inner packet (Section 5.1)
// Related: ts_narrow.go -- narrowChildSelectors drops the pairs this registry refuses
// Related: child.go -- installChildSA claims a Child SA, removeChildSAExcept releases or strands it
// Related: table.go -- the SATable carries this registry to every peer session

package engine

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// innerSelectorRegistry holds the selector pairs of every tunnel-mode Child SA the
// dataplane holds, across every peer, so a second peer cannot negotiate a tunnel one
// packet could also be sent into.
//
// RFC 3948 Section 5.1: "Because SGW will now see two possible SAs that lead to
// 10.1.2.3, it can become confused about where to send packets coming from Suzy's
// server.  Implementors MUST devise ways of preventing this from occurring."
//
// Ze prevents it the way RFC 3948 Section 5.2 names for transport mode, "by disallowing
// conflicting connections": a pair that overlaps a pair another peer holds is refused,
// unless the operator configured the remote half of both and gave the two peers
// different policy priorities (operatorOrdered). A peer bound to a virtual-IP
// pool is recorded like any other, so an unbound peer cannot take a leased address and a
// lease cannot take an unbound peer's live one, whichever arrives second.
//
// Each pair is stored in the outbound form the dataplane installs it (childPairPolicy).
// The inbound policy is the same pair with the two sides swapped, so two pairs overlap
// inbound exactly when they overlap outbound and one direction is enough.
//
// Safe for concurrent use. One mutex guards the map, and no method calls the dataplane
// or blocks while holding it.
type innerSelectorRegistry struct {
	mu sync.Mutex
	// held is bounded by the live Child SAs plus the stranded ones: claim adds one entry
	// per installed Child SA, and removeChildSAExcept removes it when the Child SA goes
	// away, or strands it when a policy of it stayed in the dataplane, until
	// retryStranded removes that policy.
	held map[*ChildSA]innerSelectorHolding
}

// innerSelectorHolding is what one Child SA holds: its peer and its pairs. stranded is
// nil while the Child SA is live, and names the policies still to remove once its
// removal left some in the dataplane (releaseInnerRecord).
type innerSelectorHolding struct {
	owner    string
	pairs    []innerSelectorPair
	stranded *strandedPolicies
}

// strandedPolicies is what a removed Child SA left in the dataplane: the policies whose
// removal failed, the dataplane to retry them on, and the log of the session that
// removed it. It is never mutated once a holding carries it: a retry that leaves some
// policies replaces it, so a retry running beside another can tell by identity whether
// the holding changed under it.
type strandedPolicies struct {
	dp   dataplane.Dataplane
	left []dataplane.SPParams
	log  *slog.Logger
}

// innerSelectorPair is one negotiated pair as the dataplane installs it outbound, with
// whether the peer's operator configured its remote half, and its text for an operator.
type innerSelectorPair struct {
	policy        dataplane.SPParams
	operatorFixed bool
	text          string
}

// innerSelectorGuard is what a Child SA carries to be recorded: the registry its peer
// session shares, and the remote selectors that peer's operator configured. A rekey
// inherits it from the retired Child SA, because the replacement belongs to the same
// peer under the same configuration.
type innerSelectorGuard struct {
	registry         *innerSelectorRegistry
	configuredRemote []tsSelector
}

// newInnerSelectorRegistry returns an empty registry.
func newInnerSelectorRegistry() *innerSelectorRegistry {
	return &innerSelectorRegistry{held: make(map[*ChildSA]innerSelectorHolding)}
}

// configuredRemoteSelectors returns the remote halves of the operator's configured
// traffic selectors of peer, the form innerSelectorPairOf compares against.
func configuredRemoteSelectors(peer ipsec.SiteToSitePeer) []tsSelector {
	out := make([]tsSelector, 0, len(peer.TrafficSelectors))
	for _, ts := range peer.TrafficSelectors {
		out = append(out, tsSelector{Net: ts.RemotePrefix, Port: ts.RemotePort, Proto: ts.Protocol})
	}
	return out
}

// innerSelectorPairOf builds the outbound policy one negotiated pair of child installs,
// and classifies it. ok is false for a pair whose protocols are disjoint: it matches no
// packet and installs no policy (childPolicyPair), so it can conflict with nothing.
//
// RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order the
// entries to express a desired access control policy." A pair is operator-fixed when its
// remote half is exactly a remote selector the operator configured, prefix, protocol and
// port alike. Two such pairs of different peers overlap because the operator wrote them
// so, and that stays legal while their policy-priority values differ and so order them
// (operatorOrdered). A remote half the peer narrowed
// inside a configured prefix is the peer's choice, and is not operator-fixed.
func innerSelectorPairOf(child *ChildSA, pair tsPair, configuredRemote []tsSelector) (innerSelectorPair, bool) {
	pp, remote, ok := childPolicyPair(child, pair)
	if !ok {
		return innerSelectorPair{}, false
	}
	return classifyInnerPair(child, pp, remote, configuredRemote), true
}

// classifyInnerPair builds the outbound policy of pp, a pair already in this node's
// orientation whose remote half is remote, and classifies it (innerSelectorPairOf).
func classifyInnerPair(child *ChildSA, pp policyPair, remote tsSelector, configuredRemote []tsSelector) innerSelectorPair {
	fixed := false
	for _, c := range configuredRemote {
		if sameInnerSelector(c, remote) {
			fixed = true
			break
		}
	}
	return innerSelectorPair{
		policy:        childPairPolicy(child, dataplane.SADirOut, pp),
		operatorFixed: fixed,
		text:          innerPairText(pp),
	}
}

// sameInnerSelector reports whether two selectors match exactly the same packets on
// their side: one prefix, one protocol, one port match. The port is compared in the
// form the dataplane installs (selectorPort), so an unset port and "any" agree.
func sameInnerSelector(a, b tsSelector) bool {
	if a.Proto != b.Proto {
		return false
	}
	if selectorPort(a) != selectorPort(b) {
		return false
	}
	if a.Net == nil || b.Net == nil {
		return a.Net == nil && b.Net == nil
	}
	return a.Net.String() == b.Net.String()
}

// innerPairText renders a pair in this node's orientation for an operator.
func innerPairText(pp policyPair) string {
	var text textbuf.Buffer
	text.Str("local ").Str(pp.local.String())
	if !pp.localPort.IsAny() {
		text.Byte(':').Uint16(pp.localPort.Port)
	}
	text.Str(" remote ").Str(pp.remote.String())
	if !pp.remotePort.IsAny() {
		text.Byte(':').Uint16(pp.remotePort.Port)
	}
	if pp.upperProto != 0 {
		text.Str(" protocol ").Uint8(pp.upperProto)
	}
	return string(text.Bytes())
}

// dropForeignInnerOverlaps removes from pairs, the narrowed answer of a tunnel-mode
// exchange this node responds to, every pair that overlaps a live pair of another peer
// (RFC 3948 Section 5.1). It reads the registry and records nothing: installChildSA
// records the answer once the Child SA installs.
//
// RFC 7296 Section 2.9: "If the responder's policy allows it to accept the first
// selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a
// subset that includes the initiator's first choices." Refusing a whole proposal because
// a LATER pair conflicts would break that, so a conflicting pair is dropped and the
// rest keep their order. Only when nothing is left is the exchange refused, with the
// first conflict met, which wraps errTSUnacceptable.
func dropForeignInnerOverlaps(sa *SA, pairs []tsPair) ([]tsPair, error) {
	// refuse-by-default: guards dropForeignInnerOverlaps -- only an SA a test built by hand carries no registry; every session-built SA carries SATable.innerSelectors
	if sa.innerSelectors == nil {
		return pairs, nil
	}
	// RFC 3948 Section 5.1: a record a failed removal stranded is retried before it can
	// refuse this proposal.
	sa.innerSelectors.retryStranded()
	ifID, err := resolveIfID(&sa.PeerCfg)
	if err != nil {
		return nil, err
	}
	// The Child SA this answer would install, as far as its policies go: this node
	// responds, so its own side is TSr (narrowChildSelectors).
	probe := &ChildSA{IfID: ifID, Mode: modeTunnel, Owner: sa.PeerName, PolicyPriority: sa.PeerCfg.PolicyPriority}
	configured := configuredRemoteSelectors(sa.PeerCfg)
	kept := make([]tsPair, 0, len(pairs))
	var refusal error
	for _, pair := range pairs {
		p, ok := innerSelectorPairOf(probe, pair, configured)
		if !ok {
			kept = append(kept, pair)
			continue
		}
		if err := sa.innerSelectors.admit(sa.PeerName, p); err != nil {
			if refusal == nil {
				refusal = err
			}
			continue
		}
		kept = append(kept, pair)
	}
	if len(kept) == 0 {
		return nil, refusal
	}
	return kept, nil
}

// innerSelectorPairsOf builds every pair child installs, in the order childPolicies
// installs them.
func innerSelectorPairsOf(child *ChildSA) []innerSelectorPair {
	configured := child.innerGuard.configuredRemote
	// A Child SA with no negotiated pairs installs one policy from TSLocal and TSRemote,
	// already in this node's orientation (childPolicies). The pair is built the same
	// way here: routed through innerSelectorPairOf, a tsPair with no orientation flag
	// read TSLocal as the remote half and recorded the policy reversed.
	if len(child.Selectors) == 0 {
		pp := policyPair{local: child.TSLocal, remote: child.TSRemote}
		return []innerSelectorPair{classifyInnerPair(child, pp, tsSelector{Net: child.TSRemote}, configured)}
	}
	out := make([]innerSelectorPair, 0, len(child.Selectors))
	for _, pair := range child.Selectors {
		p, ok := innerSelectorPairOf(child, pair, configured)
		if !ok {
			continue
		}
		out = append(out, p)
	}
	return out
}

// admit reports whether owner may take p, without recording anything. It is the
// narrowing filter's question (dropForeignInnerOverlaps); the answer can be stale by the
// time the Child SA installs, which is why claim asks it again under the same lock as
// its write.
func (r *innerSelectorRegistry) admit(owner string, p innerSelectorPair) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.conflictLocked(owner, p)
}

// claim records every pair of child under owner, or refuses child whole when one of its
// pairs conflicts. The check and the write share one lock, so two peers that both
// passed the narrowing filter cannot both be recorded. A claim of a child already
// recorded replaces its record.
//
// installs is every policy child will install, both directions. A stranded holding of
// the same owner gives up each policy of its own that installs names: the install
// upserts that selector (xfrmBackend.InstallPolicy), so the kernel entry belongs to
// child from then on, and a later retry removing it would take the live tunnel's
// policy with it.
//
// MUST be paired with release when the Child SA goes away or its install fails.
func (r *innerSelectorRegistry) claim(child *ChildSA, owner string, pairs []innerSelectorPair, installs []dataplane.SPParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range pairs {
		if err := r.conflictLocked(owner, pairs[i]); err != nil {
			return err
		}
	}
	r.adoptStrandedLocked(owner, installs)
	r.held[child] = innerSelectorHolding{owner: owner, pairs: pairs}
	return nil
}

// adoptStrandedLocked drops from every stranded holding of owner the policies installs
// re-installs, and forgets a holding left with none (claim). MUST be called with r.mu
// held.
func (r *innerSelectorRegistry) adoptStrandedLocked(owner string, installs []dataplane.SPParams) {
	for child, holding := range r.held {
		if holding.stranded == nil {
			continue
		}
		if holding.owner != owner {
			continue
		}
		left := make([]dataplane.SPParams, 0, len(holding.stranded.left))
		for i := range holding.stranded.left {
			if !installsSelector(installs, &holding.stranded.left[i]) {
				left = append(left, holding.stranded.left[i])
			}
		}
		if len(left) == 0 {
			delete(r.held, child)
			continue
		}
		holding.stranded = &strandedPolicies{dp: holding.stranded.dp, left: left, log: holding.stranded.log}
		r.held[child] = holding
	}
}

// installsSelector reports whether installs holds an entry with p's selector.
func installsSelector(installs []dataplane.SPParams, p *dataplane.SPParams) bool {
	sel := selectorOf(p)
	for i := range installs {
		if selectorOf(&installs[i]) == sel {
			return true
		}
	}
	return false
}

// release forgets child's record. Releasing a child never recorded does nothing, so
// every removal path can call it.
//
// MUST be called for every Child SA claim recorded, once the Child SA is removed with
// every policy of it gone; strand is called instead when some stayed.
func (r *innerSelectorRegistry) release(child *ChildSA) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.held, child)
}

// strand keeps child's record after its removal left the policies s names in the
// dataplane, so retryStranded can remove them and then release it. A child never
// recorded stays unrecorded: nothing of it can conflict.
func (r *innerSelectorRegistry) strand(child *ChildSA, s *strandedPolicies) {
	r.mu.Lock()
	defer r.mu.Unlock()
	holding, ok := r.held[child]
	if !ok {
		return
	}
	holding.stranded = s
	r.held[child] = holding
}

// retryStranded tries again to remove the policies of every stranded holding, and
// releases each holding whose policies are all gone.
//
// RFC 3948 Section 5.1: "Implementors MUST devise ways of preventing this from
// occurring." A stranded record keeps refusing every overlapping tunnel, which is right
// while its policy is in the kernel and wrong the moment it is not. One transient
// netlink failure (EIO, ENOBUFS) at teardown would otherwise refuse the overlapping
// peer until Ze restarted. The retry runs where the record matters, before a peer's
// proposal is narrowed (dropForeignInnerOverlaps) and before a Child SA claims
// (installChildSA), and on each configuration commit (reconcilePeers). It is bounded:
// one removal per stranded policy each time, with no loop and no goroutine of its own,
// and a refused peer's next attempt is the next try.
//
// Safe for concurrent use. The dataplane is called with r.mu released, and a holding
// that changed while it ran (released, adopted, or retried beside it) is left as the
// other path left it.
func (r *innerSelectorRegistry) retryStranded() {
	type strandedChild struct {
		child    *ChildSA
		stranded *strandedPolicies
	}
	r.mu.Lock()
	var work []strandedChild
	for child, holding := range r.held {
		if holding.stranded != nil {
			work = append(work, strandedChild{child: child, stranded: holding.stranded})
		}
	}
	r.mu.Unlock()

	for _, w := range work {
		left := removeStrandedPolicies(w.stranded)
		r.settleStranded(w.child, w.stranded, left)
	}
}

// settleStranded records the outcome of one retry of was: child is released when left
// is empty, and otherwise keeps a record naming left. A holding no longer carrying was
// is left alone (retryStranded).
func (r *innerSelectorRegistry) settleStranded(child *ChildSA, was *strandedPolicies, left []dataplane.SPParams) {
	r.mu.Lock()
	defer r.mu.Unlock()
	holding, ok := r.held[child]
	if !ok {
		return
	}
	if holding.stranded != was {
		return
	}
	if len(left) == 0 {
		delete(r.held, child)
		was.log.Info("child-sa: the policies a failed removal left are gone, so their tunnel selectors are released",
			"peer", holding.owner, "in-spi", child.InboundSPI, "out-spi", child.OutboundSPI)
		return
	}
	holding.stranded = &strandedPolicies{dp: was.dp, left: left, log: was.log}
	r.held[child] = holding
}

// holdings returns how many Child SAs the registry records.
func (r *innerSelectorRegistry) holdings() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.held)
}

// conflictLocked returns the refusal for p when another owner holds a pair one packet
// could also match, unless both pairs are operator-fixed AND sit at different policy
// priorities. The same owner never conflicts with itself: a Child SA rekey and an IKE SA
// re-authentication overlap their own live selectors by construction. MUST be called
// with r.mu held.
//
// RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order the
// entries to express a desired access control policy." Two operator-fixed pairs at
// different priorities are ordered by the operator, so the SPD picks one tunnel for
// every packet. At one priority nothing orders them: the dataplane matches whichever
// installed first, which is arrival order, the confusion RFC 3948 Section 5.1 forbids.
func (r *innerSelectorRegistry) conflictLocked(owner string, p innerSelectorPair) error {
	for _, holding := range r.held {
		if holding.owner == owner {
			continue
		}
		for i := range holding.pairs {
			h := holding.pairs[i]
			if operatorOrdered(p, h) {
				continue
			}
			if !dataplane.PolicySelectorsOverlap(p.policy, h.policy) {
				continue
			}
			if holding.stranded != nil {
				return innerSelectorStrandedConflict(owner, p, holding.owner, h, holding.stranded)
			}
			return innerSelectorConflict(owner, p, holding.owner, h)
		}
	}
	return nil
}

// operatorOrdered reports whether the operator both wrote and ordered the overlap of p
// and h: each remote half is a selector the operator configured, and the two policies
// sit at different priorities, so the SPD sends every packet into one tunnel. Equal
// priorities fail the test even when both pairs are operator-fixed (conflictLocked).
func operatorOrdered(p, h innerSelectorPair) bool {
	if !p.operatorFixed {
		return false
	}
	if !h.operatorFixed {
		return false
	}
	return p.policy.Priority != h.policy.Priority
}

// innerSelectorConflict is the refusal of p for owner because holder holds h. It wraps
// errTSUnacceptable, which notifyForRefusal answers as TS_UNACCEPTABLE.
//
// RFC 7296 Section 2.9: "If the responder's policy does not allow it to accept any part
// of the proposed Traffic Selectors, it responds with a TS_UNACCEPTABLE Notify message."
func innerSelectorConflict(owner string, p innerSelectorPair, holder string, h innerSelectorPair) error {
	return fmt.Errorf("%w: peer %s proposed the tunnel %s, which overlaps the tunnel %s that peer %s "+
		"already holds, so one packet could be sent into either tunnel (RFC 3948 Section 5.1); "+
		"configure disjoint remote prefixes with 'set vpn ipsec site-to-site peer %s traffic-selector <n> "+
		"remote prefix <prefix>', bind the peer to a virtual-IP pool with "+
		"'set vpn ipsec site-to-site peer %s virtual-ip-pool <pool>', or, where both peers' "+
		"configured remote prefixes overlap on purpose, order them by giving one a different "+
		"priority with 'set vpn ipsec site-to-site peer %s policy-priority <n>'",
		errInnerSelectorConflict, owner, p.text, h.text, holder, owner, owner, owner)
}

// innerSelectorStrandedConflict is the refusal of p for owner because a removed Child SA
// of holder left a policy overlapping it in the dataplane (strandedPolicies). It wraps
// errInnerSelectorStranded, so it is answered TS_UNACCEPTABLE like any overlap, and its
// fix names what releases the record: the retry Ze runs, and the exact commands that
// remove the policies now. It names no configuration of holder, which may no longer be
// a configured peer at all.
func innerSelectorStrandedConflict(owner string, p innerSelectorPair, holder string, h innerSelectorPair,
	s *strandedPolicies,
) error {
	var commands textbuf.Buffer
	for i := range s.left {
		if i > 0 {
			commands.WriteString("; ")
		}
		commands.WriteString(deleteCommand(&s.left[i]))
	}
	return fmt.Errorf("%w: peer %s proposed the tunnel %s, which overlaps the tunnel %s that peer %s held, "+
		"and a policy of that tunnel could not be removed from the dataplane, so one packet could still be "+
		"sent into it (RFC 3948 Section 5.1); Ze retries the removal each time a peer proposes a tunnel and "+
		"each time a configuration is committed; to release it now, run %s, and the next proposal "+
		"releases it, or as a last resort restart Ze",
		errInnerSelectorStranded, owner, p.text, h.text, holder, commands.String())
}

// claim records child in its registry (RFC 3948 Section 5.1). A tunnel-mode Child SA
// only: transport-mode overlap is refused by the dataplane (RFC 3948 Section 5.2,
// dataplane policyOwners.claim), and recording it here would refuse it twice.
func (g innerSelectorGuard) claim(child *ChildSA) error {
	// refuse-by-default: guards innerSelectorGuard -- only a Child SA a test built by hand carries no registry; every session-built SA carries SATable.innerSelectors
	if g.registry == nil {
		return nil
	}
	if child.Mode == modeTransport {
		return nil
	}
	installs := append(childPolicies(child, dataplane.SADirOut), childPolicies(child, dataplane.SADirIn)...)
	return g.registry.claim(child, child.Owner, innerSelectorPairsOf(child), installs)
}

// retryStranded retries the stranded records of its registry (innerSelectorRegistry.retryStranded).
func (g innerSelectorGuard) retryStranded() {
	// refuse-by-default: guards innerSelectorGuard -- only a Child SA a test built by hand carries no registry, and none holds a stranded record
	if g.registry == nil {
		return
	}
	g.registry.retryStranded()
}

// strand keeps child's record in its registry with the policies its removal left
// (releaseInnerRecord). Safe on a Child SA never recorded.
func (g innerSelectorGuard) strand(child *ChildSA, s *strandedPolicies) {
	// refuse-by-default: guards innerSelectorGuard -- only a Child SA a test built by hand carries no registry, and it was never recorded
	if g.registry == nil {
		return
	}
	g.registry.strand(child, s)
}

// errInnerSelectorStranded marks a refusal because a removed Child SA of another peer
// left an overlapping policy in the dataplane (innerSelectorStrandedConflict). It wraps
// errInnerSelectorConflict, so every path that answers an overlap answers this one the
// same way, and the fix selectors name childRefusedFixResponderStranded and
// childResponseFixStranded in its place.
var errInnerSelectorStranded = fmt.Errorf("%w: a removed tunnel's policy is still in the dataplane",
	errInnerSelectorConflict)

// childRefusedFixResponderStranded is the fix logged beside that refusal as responder.
const childRefusedFixResponderStranded = "a tunnel another peer held, which one packet could also be " +
	"sent into, left a policy Ze could not remove (RFC 3948 Section 5.1), so this peer gets no Child SA " +
	"until it is gone; Ze retries the removal each time a peer proposes a tunnel and each time a " +
	"configuration is committed; to release it now, run the ip xfrm policy delete commands the error " +
	"names, and the peer's next proposal releases it, or as a last resort restart Ze"

// childResponseFixStranded is the fix logged when this node, as the Child SA initiator,
// refuses the answered tunnel for the same reason.
const childResponseFixStranded = "a tunnel another peer held, which one packet could also be " +
	"sent into, left a policy Ze could not remove (RFC 3948 Section 5.1), so this Child SA was deleted " +
	"and the IKE SA kept with no Child SA, asking again on its creation schedule; Ze retries the removal " +
	"at each attempt and each time a configuration is committed; to release it now, run the " +
	"ip xfrm policy delete commands the error names, and the next attempt releases it, or as a last " +
	"resort restart Ze"

// errInnerSelectorConflict marks a refusal because another peer holds an overlapping
// tunnel (innerSelectorConflict). It wraps errTSUnacceptable, so notifyForRefusal still
// answers TS_UNACCEPTABLE, and childRefusedResponderFix names this fix rather than the
// kernel-install one an install-time refusal would otherwise read as.
var errInnerSelectorConflict = fmt.Errorf("%w: another peer holds an overlapping tunnel", errTSUnacceptable)

// childRefusedFixResponderInnerSelector is the fix logged beside that refusal.
const childRefusedFixResponderInnerSelector = "another peer already holds a tunnel one packet could also be " +
	"sent into (RFC 3948 Section 5.1), so this peer gets no Child SA until that tunnel goes away; " +
	"configure disjoint remote prefixes for the two peers with " +
	"set vpn ipsec site-to-site peer <name> traffic-selector <n> remote prefix <prefix>, bind the peer " +
	"to a virtual-IP pool with set vpn ipsec site-to-site peer <name> virtual-ip-pool <pool>, or, where " +
	"both peers' configured remote prefixes overlap on purpose, give one of them a different priority " +
	"with set vpn ipsec site-to-site peer <name> policy-priority <n>, then commit"

// childResponseFixInnerSelector is the fix logged when this node, as the Child SA
// initiator, refuses the answered tunnel because another peer holds an overlapping one.
// It names only what the initiator's own configuration controls: a virtual-IP pool is
// a responder's lease, so it is not offered here.
const childResponseFixInnerSelector = "another peer already holds a tunnel one packet could also be " +
	"sent into (RFC 3948 Section 5.1), so this Child SA was deleted and the IKE SA kept with no Child SA, " +
	"asking again on its creation schedule; configure disjoint remote prefixes for the two peers with " +
	"set vpn ipsec site-to-site peer <name> traffic-selector <n> remote prefix <prefix>, or, where both " +
	"peers' configured remote prefixes overlap on purpose, give one of them a different priority with " +
	"set vpn ipsec site-to-site peer <name> policy-priority <n>, then commit"

// release forgets child in its registry. Safe on a Child SA never recorded.
func (g innerSelectorGuard) release(child *ChildSA) {
	if g.registry == nil {
		return
	}
	g.registry.release(child)
}
