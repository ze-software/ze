// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the inner-selector registry
// RFC: rfc/short/rfc3948.md -- no two peers' tunnels for one inner packet (Section 5.1)
// Related: ts_narrow.go -- narrowChildSelectors drops the pairs this registry refuses
// Related: child.go -- installChildSA claims a Child SA, removeChildSAExcept releases it
// Related: table.go -- the SATable carries this registry to every peer session

package engine

import (
	"fmt"
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
// unless the operator configured the remote half of both. A peer bound to a virtual-IP
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
	// held is bounded by the live Child SAs: claim adds one entry per installed Child
	// SA, and removeChildSAExcept removes it when the Child SA goes away.
	held map[*ChildSA]innerSelectorHolding
}

// innerSelectorHolding is what one live Child SA holds: its peer and its pairs.
type innerSelectorHolding struct {
	owner string
	pairs []innerSelectorPair
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
// so, ordered by policy-priority, and that stays legal. A remote half the peer narrowed
// inside a configured prefix is the peer's choice, and is not operator-fixed.
func innerSelectorPairOf(child *ChildSA, pair tsPair, configuredRemote []tsSelector) (innerSelectorPair, bool) {
	pp, remote, ok := childPolicyPair(child, pair)
	if !ok {
		return innerSelectorPair{}, false
	}
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
	}, true
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
	// refuse-by-default: guards narrowChildSelectors -- only an SA a test built by hand carries no registry; every session-built SA carries SATable.innerSelectors
	if sa.innerSelectors == nil {
		return pairs, nil
	}
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
	if len(child.Selectors) == 0 {
		p, ok := innerSelectorPairOf(child, tsPair{I: tsSelector{Net: child.TSLocal}, R: tsSelector{Net: child.TSRemote}},
			configured)
		if !ok {
			return nil
		}
		return []innerSelectorPair{p}
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
// MUST be paired with release when the Child SA goes away or its install fails.
func (r *innerSelectorRegistry) claim(child *ChildSA, owner string, pairs []innerSelectorPair) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range pairs {
		if err := r.conflictLocked(owner, p); err != nil {
			return err
		}
	}
	r.held[child] = innerSelectorHolding{owner: owner, pairs: pairs}
	return nil
}

// release forgets child's record. Releasing a child never recorded does nothing, so
// every removal path can call it.
//
// MUST be called for every Child SA claim recorded, once the Child SA is removed.
func (r *innerSelectorRegistry) release(child *ChildSA) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.held, child)
}

// holdings returns how many Child SAs the registry records.
func (r *innerSelectorRegistry) holdings() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.held)
}

// conflictLocked returns the refusal for p when another owner holds a pair one packet
// could also match, unless both pairs are operator-fixed. The same owner never
// conflicts with itself: a Child SA rekey and an IKE SA re-authentication overlap their
// own live selectors by construction. MUST be called with r.mu held.
func (r *innerSelectorRegistry) conflictLocked(owner string, p innerSelectorPair) error {
	for _, holding := range r.held {
		if holding.owner == owner {
			continue
		}
		for _, h := range holding.pairs {
			if p.operatorFixed && h.operatorFixed {
				continue
			}
			if !dataplane.PolicySelectorsOverlap(p.policy, h.policy) {
				continue
			}
			return innerSelectorConflict(owner, p, holding.owner, h)
		}
	}
	return nil
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
		"remote prefix <prefix>', or bind the peer to a virtual-IP pool with "+
		"'set vpn ipsec site-to-site peer %s virtual-ip-pool <pool>'",
		errInnerSelectorConflict, owner, p.text, h.text, holder, owner, owner)
}

// claim records child in its registry (RFC 3948 Section 5.1). A tunnel-mode Child SA
// only: transport-mode overlap is refused by the dataplane (RFC 3948 Section 5.2,
// dataplane policyOwners.claim), and recording it here would refuse it twice.
func (g innerSelectorGuard) claim(child *ChildSA) error {
	// refuse-by-default: guards installChildSA -- only a Child SA a test built by hand carries no registry; every session-built SA carries SATable.innerSelectors
	if g.registry == nil {
		return nil
	}
	if child.Mode == modeTransport {
		return nil
	}
	return g.registry.claim(child, child.Owner, innerSelectorPairsOf(child))
}

// errInnerSelectorConflict marks a refusal because another peer holds an overlapping
// tunnel (innerSelectorConflict). It wraps errTSUnacceptable, so notifyForRefusal still
// answers TS_UNACCEPTABLE, and childRefusedResponderFix names this fix rather than the
// kernel-install one an install-time refusal would otherwise read as.
var errInnerSelectorConflict = fmt.Errorf("%w: another peer holds an overlapping tunnel", errTSUnacceptable)

// childRefusedFixResponderInnerSelector is the fix logged beside that refusal.
const childRefusedFixResponderInnerSelector = "another peer already holds a tunnel one packet could also be " +
	"sent into (RFC 3948 Section 5.1), so this peer gets no Child SA until that tunnel goes away; " +
	"configure disjoint remote prefixes for the two peers with " +
	"set vpn ipsec site-to-site peer <name> traffic-selector <n> remote prefix <prefix>, or bind the peer " +
	"to a virtual-IP pool with set vpn ipsec site-to-site peer <name> virtual-ip-pool <pool>, then commit"

// release forgets child in its registry. Safe on a Child SA never recorded.
func (g innerSelectorGuard) release(child *ChildSA) {
	if g.registry == nil {
		return
	}
	g.registry.release(child)
}
