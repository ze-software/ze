// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the inner-selector registry
// RFC: rfc/short/rfc3948.md -- no two peers' tunnels for one inner packet (Section 5.1)
// Related: ts_narrow.go -- narrowChildSelectors drops the pairs this registry refuses
// Related: child.go -- installChildSA claims a Child SA, removeChildSAExcept releases or strands it
// Related: table.go -- the SATable carries this registry to every peer session

package engine

import (
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// innerSelectorRegistry holds the selector pairs of every Child SA the dataplane holds,
// tunnel and transport mode alike, across every peer, so a second peer cannot negotiate
// a tunnel one packet could also be sent into.
//
// RFC 3948 Section 5.1: "Because SGW will now see two possible SAs that lead to
// 10.1.2.3, it can become confused about where to send packets coming from Suzy's
// server.  Implementors MUST devise ways of preventing this from occurring."
//
// Ze prevents it the way RFC 3948 Section 5.2 names for transport mode, "by disallowing
// conflicting connections": a pair that overlaps a pair another peer holds is refused,
// unless the operator configured the whole of both and gave the two peers
// different policy priorities (operatorOrdered). A transport-mode pair is recorded too,
// because a tunnel another peer negotiates around a transport peer's address is the
// same confusion: one packet, two SAs. Two transport-mode pairs are not compared here;
// the dataplane refuses their overlap (RFC 3948 Section 5.2, policyOwners.claim). A peer bound to a virtual-IP
// pool is recorded like any other, so an unbound peer cannot take a leased address and a
// lease cannot take an unbound peer's live one, whichever arrives second.
//
// Each pair is stored in the outbound form the dataplane installs it (childPairPolicy).
// The inbound policy is the same pair with the two sides swapped, so two pairs overlap
// inbound exactly when they overlap outbound and one direction is enough.
//
// Safe for concurrent use. mu guards the map, and no method calls the dataplane or
// blocks while holding it. retryMu serializes the stranded retry with itself and with
// claim, and it is held across the retry's policy removals (retryStranded).
//
// Lock order: retryMu, then mu. claim and retryStranded take retryMu first; every
// other method takes mu alone, and nothing takes retryMu while holding mu.
type innerSelectorRegistry struct {
	retryMu sync.Mutex
	mu      sync.Mutex
	// held is bounded by the live Child SAs plus the stranded ones: claim adds one entry
	// per installed Child SA, and removeChildSAExcept removes it when the Child SA goes
	// away, or strands it when a policy of it stayed in the dataplane, until
	// retryStranded removes that policy. Written only through putLocked and
	// deleteLocked, which keep stranded in step.
	held map[*ChildSA]innerSelectorHolding
	// stranded counts the holdings in held that carry stranded policies, so the retry
	// that runs before every proposal returns at once when nothing is stranded.
	stranded int
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
// policies replaces it, so settleStranded can tell by identity whether the holding
// changed under it.
//
// dp is the dataplane that installed the policies, and the retry removes them through
// it rather than through dataplane.Get. runEngine loads the backend once
// (dataplane.Load, register.go) and a configuration commit keeps it, so the two are the
// same backend for the engine's life. Only that instance could remove the entry anyway:
// a VPP backend sends back the SPD entry it recorded, in the SPD it created.
type strandedPolicies struct {
	dp   dataplane.Dataplane
	left []strandedPolicy
	log  *slog.Logger
}

// strandedPolicy is one policy a removal left, with the command that removes it by
// hand, named when the policy was stranded (policyDeleteCommand). The command is carried
// rather than asked for when a refusal is written, because the refusal is written
// under the registry's lock, which never calls the dataplane.
type strandedPolicy struct {
	params  dataplane.SPParams
	command string
}

// newStrandedPolicies names, for each policy in left, the command that removes it on
// dp (policyDeleteCommand), and carries both for retryStranded.
func newStrandedPolicies(dp dataplane.Dataplane, left []dataplane.SPParams, log *slog.Logger) *strandedPolicies {
	out := make([]strandedPolicy, 0, len(left))
	for i := range left {
		out = append(out, strandedPolicy{params: left[i], command: policyDeleteCommand(dp, &left[i])})
	}
	return &strandedPolicies{dp: dp, left: out, log: log}
}

// policyDeleteCommand is the command an operator runs to remove p from dp by hand, as
// the backend names it (dataplane.PolicyDeleteCommander): `ip xfrm policy delete` for
// XFRM, `vppctl ipsec policy del` for VPP. A backend that names none, or cannot name
// this policy, is said to, with the policy's selector, so the operator is never handed
// another backend's command for it. It names both a stranded record's policies and the
// SPD ledger's leftovers (spdLeftover.describe).
func policyDeleteCommand(dp dataplane.Dataplane, p *dataplane.SPParams) string {
	commander, ok := dp.(dataplane.PolicyDeleteCommander)
	if !ok {
		return "(the dataplane names no command that removes the policy " + policySelectorText(p) + ")"
	}
	command, err := commander.PolicyDeleteCommand(*p)
	if err != nil {
		return "(the dataplane names no command that removes the policy " + policySelectorText(p) +
			": " + err.Error() + ")"
	}
	return command
}

// policySelectorText names p's selector in no backend's syntax, for policyDeleteCommand.
func policySelectorText(p *dataplane.SPParams) string {
	var text textbuf.Buffer
	text.Str(dirLabel(p.Dir)).Str(" src ").Str(ipNetText(p.Src)).Str(" dst ").Str(ipNetText(p.Dst))
	if p.UpperProto != 0 {
		text.Str(" protocol ").Uint8(p.UpperProto)
	}
	return string(text.Bytes())
}

// ipNetText names a selector prefix, a nil one as "any".
func ipNetText(n *net.IPNet) string {
	if n == nil {
		return "any"
	}
	return n.String()
}

// innerSelectorPair is one negotiated pair as the dataplane installs it outbound, with
// whether the peer's operator configured that whole policy, and its text for an operator.
// Its mode is the policy's (transport).
type innerSelectorPair struct {
	policy        dataplane.SPParams
	operatorFixed bool
	text          string
}

// transport reports whether p is a transport-mode pair, in the mode the dataplane
// installs it: childPairPolicy resolves an unset mode to tunnel, so this cannot disagree
// with the install.
func (p innerSelectorPair) transport() bool {
	return p.policy.Mode == modeTransport
}

// kind names p's mode for an operator.
func (p innerSelectorPair) kind() string {
	if p.transport() {
		return "transport-mode connection"
	}
	return "tunnel"
}

// innerSelectorGuard is what a Child SA carries to be recorded: the registry its peer
// session shares, and the selector pairs that peer's operator configured. A rekey
// inherits it from the retired Child SA, because the replacement belongs to the same
// peer under the same configuration.
type innerSelectorGuard struct {
	registry *innerSelectorRegistry
	// configured is the peer's configured traffic selectors, each as the policy pair it
	// installs in this node's orientation (configuredPairs).
	configured []policyPair
	// configuredHost is the pair a transport-mode connection installs between the two
	// configured addresses for every protocol and every port, or nil when either address
	// is not an address literal (configuredHostPair).
	configuredHost *policyPair
}

// newInnerSelectorRegistry returns an empty registry.
func newInnerSelectorRegistry() *innerSelectorRegistry {
	return &innerSelectorRegistry{held: make(map[*ChildSA]innerSelectorHolding)}
}

// configuredPairs returns the operator's configured traffic selectors of peer, each as
// the policy pair it installs in this node's orientation, the form classifyInnerPair
// compares against. A configured selector carries one protocol for both halves, so the
// pair installs that protocol (childPolicyPair intersects the two halves' protocols).
func configuredPairs(peer ipsec.SiteToSitePeer) []policyPair {
	out := make([]policyPair, 0, len(peer.TrafficSelectors))
	for _, ts := range peer.TrafficSelectors {
		out = append(out, policyPair{
			local:      ts.LocalPrefix,
			remote:     ts.RemotePrefix,
			upperProto: ts.Protocol,
			localPort:  selectorPort(tsSelector{Port: ts.LocalPort}),
			remotePort: selectorPort(tsSelector{Port: ts.RemotePort}),
		})
	}
	return out
}

// configuredHostPair returns the pair a transport-mode connection with peer installs when
// nothing narrows it: this node's configured local address to the peer's configured remote
// address, each as a host prefix, every protocol and every port (RFC 7296 Section 2.23.1:
// "For transport mode, it MUST use exactly one IP address in the TSi and TSr payloads.").
// The local half is the address the IKE SA runs on (observedLocalAddress). It is nil when
// either address is not an address literal (the remote-address leaf also takes a DNS
// hostname): then the operator wrote no address, only a name that resolves to one, and no
// transport pair of that peer is operator-fixed by it (classifyInnerPair).
func configuredHostPair(peer ipsec.SiteToSitePeer) *policyPair {
	local := hostPrefixOf(peer.LocalAddress)
	if local == nil {
		return nil
	}
	remote := hostPrefixOf(peer.RemoteAddress)
	if remote == nil {
		return nil
	}
	return &policyPair{local: local, remote: remote}
}

// hostPrefixOf returns the address literal address as a host prefix (/32 or /128), or nil
// when address is not an address literal.
func hostPrefixOf(address string) *net.IPNet {
	ip := net.ParseIP(address)
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
}

// innerSelectorPairOf builds the outbound policy one negotiated pair of child installs,
// and classifies it. ok is false for a pair whose protocols are disjoint: it matches no
// packet and installs no policy (childPolicyPair), so it can conflict with nothing.
//
// RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order the
// entries to express a desired access control policy." A pair is operator-fixed when the
// operator wrote the whole policy it installs, in one of two ways: it is exactly a traffic
// selector the operator configured, both prefixes, the protocol and both ports alike; or
// it is a transport-mode pair that is exactly the two configured addresses
// (configuredHost) for every protocol and every port, because a transport-mode
// connection's selectors are its two endpoint addresses and no traffic selector can move
// them. Two such pairs of different peers overlap because the operator wrote them so, and
// that stays legal while their policy-priority values differ and so order them
// (operatorOrdered). A pair the peer narrowed, on either half, inside a configured
// selector or to one protocol or port between the configured addresses, is the peer's
// choice, and is not operator-fixed: the policy the priorities would order is not one the
// operator wrote.
func innerSelectorPairOf(child *ChildSA, pair tsPair, configured []policyPair,
	configuredHost *policyPair,
) (innerSelectorPair, bool) {
	pp, ok := childPolicyPair(child, pair)
	if !ok {
		return innerSelectorPair{}, false
	}
	return classifyInnerPair(child, pp, configured, configuredHost), true
}

// classifyInnerPair builds the outbound policy of pp, a pair already in this node's
// orientation, and classifies it (innerSelectorPairOf). It judges pp whole, because pp is
// what the dataplane installs: its protocol is the intersection of the two halves'
// (childPolicyPair), so a peer that narrows only this node's half still narrows the policy.
func classifyInnerPair(child *ChildSA, pp policyPair, configured []policyPair,
	configuredHost *policyPair,
) innerSelectorPair {
	p := innerSelectorPair{
		policy: childPairPolicy(child, dataplane.SADirOut, pp),
		text:   innerPairText(pp),
	}
	for _, c := range configured {
		if samePolicyPair(c, pp) {
			p.operatorFixed = true
			return p
		}
	}
	if p.transport() {
		p.operatorFixed = isConfiguredHost(pp, configuredHost)
	}
	return p
}

// isConfiguredHost reports whether pp is exactly host, the pair between the peer's two
// configured addresses (configuredHostPair), for every protocol and every port on both
// halves. The operator wrote the addresses and nothing else, so a pair the peer narrowed
// to one protocol or one port, on either half, is the peer's choice, as it is inside a
// configured traffic selector. A nil host matches nothing.
func isConfiguredHost(pp policyPair, host *policyPair) bool {
	if host == nil {
		return false
	}
	return samePolicyPair(*host, pp)
}

// samePolicyPair reports whether two pairs install the same policy: both prefixes, one
// protocol, and both port matches in the form the dataplane installs (selectorPort), so
// an unset port and "any" agree.
func samePolicyPair(a, b policyPair) bool {
	if a.upperProto != b.upperProto {
		return false
	}
	if a.localPort != b.localPort {
		return false
	}
	if a.remotePort != b.remotePort {
		return false
	}
	if !samePrefix(a.local, b.local) {
		return false
	}
	return samePrefix(a.remote, b.remote)
}

// samePrefix reports whether a and b are one prefix. Two nil prefixes agree, and a nil
// prefix matches no other.
func samePrefix(a, b *net.IPNet) bool {
	if a == nil {
		return b == nil
	}
	if b == nil {
		return false
	}
	return a.String() == b.String()
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

// dropForeignInnerOverlaps removes from pairs, the narrowed answer of an exchange this
// node responds to, every pair that overlaps a live pair of another peer (RFC 3948
// Section 5.1). The answer is judged in mode, the mode the Child SA this answer
// creates installs (narrowChildSelectorsIn): a transport-mode answer is refused
// against another peer's tunnel, and a tunnel answer against another peer's
// transport-mode pair, while two transport-mode pairs are left to the dataplane
// (conflictLocked). It reads the registry and records nothing: installChildSA records
// the answer once the Child SA installs, and its claim refuses again whatever passed
// here, because another peer can install between the two.
//
// RFC 7296 Section 2.9: "If the responder's policy allows it to accept the first
// selector of TSi and TSr, then the responder MUST narrow the Traffic Selectors to a
// subset that includes the initiator's first choices." Refusing a whole proposal because
// a LATER pair conflicts would break that, so a conflicting pair is dropped and the
// rest keep their order. Only when nothing is left is the exchange refused, with the
// first conflict met, which wraps errTSUnacceptable.
func dropForeignInnerOverlaps(sa *SA, pairs []tsPair, mode uint8) ([]tsPair, error) {
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
	probe := &ChildSA{IfID: ifID, Mode: mode, Owner: sa.PeerName, PolicyPriority: sa.PeerCfg.PolicyPriority}
	configured := configuredPairs(sa.PeerCfg)
	host := configuredHostPair(sa.PeerCfg)
	kept := make([]tsPair, 0, len(pairs))
	var refusal error
	for _, pair := range pairs {
		p, ok := innerSelectorPairOf(probe, pair, configured, host)
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
	configured := child.innerGuard.configured
	host := child.innerGuard.configuredHost
	// A Child SA with no negotiated pairs installs one policy from TSLocal and TSRemote,
	// already in this node's orientation (childPolicies). The pair is built the same
	// way here: routed through innerSelectorPairOf, a tsPair with no orientation flag
	// read TSLocal as the remote half and recorded the policy reversed.
	if len(child.Selectors) == 0 {
		pp := policyPair{local: child.TSLocal, remote: child.TSRemote}
		return []innerSelectorPair{classifyInnerPair(child, pp, configured, host)}
	}
	out := make([]innerSelectorPair, 0, len(child.Selectors))
	for _, pair := range child.Selectors {
		p, ok := innerSelectorPairOf(child, pair, configured, host)
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
// claim takes retryMu before mu, so it never runs while a retry is between copying the
// stranded policies and settling them. A claim in that window would adopt a policy the
// retry is about to remove, the caller's install would upsert the same selector, and
// the retry's removal would then delete the new Child SA's live policy: on XFRM the
// owner check passes because the owner is the same, and the VPP backend checks no
// owner at all. Waiting costs at most the retry's own removals, one per stranded
// policy (retryStranded).
//
// MUST be paired with release when the Child SA goes away or its install fails.
func (r *innerSelectorRegistry) claim(child *ChildSA, owner string, pairs []innerSelectorPair, installs []dataplane.SPParams) error {
	r.retryMu.Lock()
	defer r.retryMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range pairs {
		if err := r.conflictLocked(owner, pairs[i]); err != nil {
			return err
		}
	}
	r.adoptStrandedLocked(owner, installs)
	r.putLocked(child, innerSelectorHolding{owner: owner, pairs: pairs})
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
		left := make([]strandedPolicy, 0, len(holding.stranded.left))
		for i := range holding.stranded.left {
			if !installsSelector(installs, &holding.stranded.left[i].params) {
				left = append(left, holding.stranded.left[i])
			}
		}
		if len(left) == 0 {
			r.deleteLocked(child)
			continue
		}
		holding.stranded = &strandedPolicies{dp: holding.stranded.dp, left: left, log: holding.stranded.log}
		r.putLocked(child, holding)
	}
}

// putLocked records holding for child, keeping the stranded count in step with what it
// replaces. MUST be called with r.mu held.
func (r *innerSelectorRegistry) putLocked(child *ChildSA, holding innerSelectorHolding) {
	if was, ok := r.held[child]; ok && was.stranded != nil {
		r.stranded--
	}
	if holding.stranded != nil {
		r.stranded++
	}
	r.held[child] = holding
}

// deleteLocked forgets child, keeping the stranded count in step. MUST be called with
// r.mu held.
func (r *innerSelectorRegistry) deleteLocked(child *ChildSA) {
	if was, ok := r.held[child]; ok && was.stranded != nil {
		r.stranded--
	}
	delete(r.held, child)
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
	r.deleteLocked(child)
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
	r.putLocked(child, holding)
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
// Safe for concurrent use. retryMu is held from the copy of the stranded holdings,
// through every removal, to the last settle, so a second retry copies only what the
// first left, and no claim adopts a policy between this retry's copy and its removal
// (claim). Without it two retries both removed one policy, and a peer that claimed the
// freed selector between the first removal and the second lost its fresh policy to the
// second. What retryMu spans is bounded: one dataplane removal per stranded policy,
// each a netlink or VPP API request. The dataplane is called with r.mu released.
func (r *innerSelectorRegistry) retryStranded() {
	r.retryMu.Lock()
	defer r.retryMu.Unlock()

	work := r.strandedWork()
	for i := range work {
		left := removeStrandedPolicies(work[i].stranded)
		r.settleStranded(work[i].child, work[i].stranded, left)
	}
}

// strandedChild is one stranded holding a retry works on (retryStranded).
type strandedChild struct {
	child    *ChildSA
	stranded *strandedPolicies
}

// strandedWork copies every stranded holding, and none when the stranded count says
// there is none, so the retry before each proposal costs one lock when nothing is
// stranded.
func (r *innerSelectorRegistry) strandedWork() []strandedChild {
	r.mu.Lock()
	defer r.mu.Unlock()
	// refuse-by-default: guards retryStranded -- a zero count is the registry holding no stranded record, kept in step by putLocked and deleteLocked
	if r.stranded == 0 {
		return nil
	}
	work := make([]strandedChild, 0, r.stranded)
	for child, holding := range r.held {
		if holding.stranded != nil {
			work = append(work, strandedChild{child: child, stranded: holding.stranded})
		}
	}
	return work
}

// settleStranded records the outcome of one retry of was: child is released when left
// is empty, and otherwise keeps a record naming left. A holding no longer carrying was
// is left alone (retryStranded).
func (r *innerSelectorRegistry) settleStranded(child *ChildSA, was *strandedPolicies, left []strandedPolicy) {
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
		r.deleteLocked(child)
		was.log.Info("child-sa: the policies a failed removal left are gone, so their tunnel selectors are released",
			"peer", holding.owner, "in-spi", child.InboundSPI, "out-spi", child.OutboundSPI)
		return
	}
	holding.stranded = &strandedPolicies{dp: was.dp, left: left, log: was.log}
	r.putLocked(child, holding)
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
//
// A tunnel pair is compared with a transport-mode pair like any other: either way round,
// one packet could be sent into either SA. Two transport-mode pairs are not compared
// here. RFC 3948 Section 5.2: "Implementations MUST handle this situation, either by
// disallowing conflicting connections, or by other means." The dataplane already
// disallows them (dataplane policyOwners.claim), and a second refusal here would answer
// the same overlap twice with two messages.
func (r *innerSelectorRegistry) conflictLocked(owner string, p innerSelectorPair) error {
	for _, holding := range r.held {
		if holding.owner == owner {
			continue
		}
		for i := range holding.pairs {
			h := holding.pairs[i]
			if bothTransport(p, h) {
				continue
			}
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
// and h: each pair is operator-fixed (a selector pair the operator configured, or a
// transport-mode pair between the two configured addresses, classifyInnerPair), and the two policies
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

// bothTransport reports whether p and h are both transport-mode pairs, whose overlap
// the dataplane refuses (conflictLocked).
func bothTransport(p, h innerSelectorPair) bool {
	if !p.transport() {
		return false
	}
	return h.transport()
}

// innerSelectorConflict is the refusal of p for owner because holder holds h. It wraps
// errTSUnacceptable, which notifyForRefusal answers as TS_UNACCEPTABLE.
//
// RFC 7296 Section 2.9: "If the responder's policy does not allow it to accept any part
// of the proposed Traffic Selectors, it responds with a TS_UNACCEPTABLE Notify message."
//
// When one side is a transport-mode connection, its selectors are the two endpoint
// addresses, which no traffic selector of that peer can move, so the fix names the
// tunnel peer's selectors and pool.
func innerSelectorConflict(owner string, p innerSelectorPair, holder string, h innerSelectorPair) error {
	if p.transport() {
		return innerSelectorTransportConflict(owner, p, holder, h, holder)
	}
	if h.transport() {
		return innerSelectorTransportConflict(owner, p, holder, h, owner)
	}
	return fmt.Errorf("%w: peer %s proposed the tunnel %s, which overlaps the tunnel %s that peer %s "+
		"already holds, so one packet could be sent into either tunnel (RFC 3948 Section 5.1); "+
		"configure disjoint remote prefixes with 'set vpn ipsec site-to-site peer %s traffic-selector <n> "+
		"remote prefix <prefix>', bind the peer to a virtual-IP pool with "+
		"'set vpn ipsec site-to-site peer %s virtual-ip-pool <pool>', or, where both peers' "+
		"configured remote prefixes overlap on purpose, order them by giving one a different "+
		"priority with 'set vpn ipsec site-to-site peer %s policy-priority <n>'",
		errInnerSelectorConflict, owner, p.text, h.text, holder, owner, owner, owner)
}

// innerSelectorTransportConflict is innerSelectorConflict when one of the two pairs is a
// transport-mode connection; tunnelPeer is the peer of the other pair, the one whose
// configuration can move it. Its last fix states the exemption operatorOrdered grants:
// distinct priorities order the two only when the tunnel is exactly a traffic selector its
// operator configured, both halves, and the transport-mode connection is exactly the pair
// between its peer's two configured addresses, any protocol and any port
// (classifyInnerPair). It names when that pair exists: a local-address and a
// remote-address that are address literals (configuredHostPair), or a traffic selector of
// exactly those two hosts the transport peer's operator configured. The selector commands
// carry the connection's own two addresses, each a host prefix of its family (/32 or
// /128), and both halves, because a traffic selector entry commits only with its local
// prefix (ze-ipsec-conf.yang, local prefix mandatory) and a transport-mode peer's selector
// takes single hosts alone.
func innerSelectorTransportConflict(owner string, p innerSelectorPair, holder string, h innerSelectorPair,
	tunnelPeer string,
) error {
	transportPeer := owner
	transport := p
	if tunnelPeer == owner {
		transportPeer = holder
		transport = h
	}
	return fmt.Errorf("%[1]w: peer %[2]s proposed the %[3]s %[4]s, which overlaps the %[5]s %[6]s that peer %[7]s "+
		"already holds, so one packet could be sent into either SA (RFC 3948 Section 5.1); "+
		"configure remote prefixes for peer %[8]s that leave out the transport-mode peer's address with "+
		"'set vpn ipsec site-to-site peer %[8]s traffic-selector <n> remote prefix <prefix>', bind peer %[8]s "+
		"to a virtual-IP pool with 'set vpn ipsec site-to-site peer %[8]s virtual-ip-pool <pool>', or, "+
		"where peer %[8]s's configured traffic selector covers peer %[9]s's address on "+
		"purpose, order the two by giving one a different priority with "+
		"'set vpn ipsec site-to-site peer %[2]s policy-priority <n>': the priorities order them only "+
		"when the tunnel is exactly a traffic selector peer %[8]s's operator configured, both halves, "+
		"and the transport-mode connection is exactly local %[10]s remote %[11]s for any protocol and "+
		"any port, which it is only when peer %[9]s's 'local-address' and 'remote-address' are address "+
		"literals rather than DNS hostnames, or when peer %[9]s's operator configured that pair as a "+
		"traffic selector with 'set vpn ipsec site-to-site peer %[9]s traffic-selector <n> local prefix %[10]s' "+
		"and 'set vpn ipsec site-to-site peer %[9]s traffic-selector <n> remote prefix %[11]s', then commit",
		errInnerSelectorConflict, owner, p.kind(), p.text, h.kind(), h.text, holder,
		tunnelPeer, transportPeer, transport.policy.Src.String(), transport.policy.Dst.String())
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
		commands.WriteString(s.left[i].command)
	}
	return fmt.Errorf("%w: peer %s proposed the tunnel %s, which overlaps the tunnel %s that peer %s held, "+
		"and a policy of that tunnel could not be removed from the dataplane, so one packet could still be "+
		"sent into it (RFC 3948 Section 5.1); Ze retries the removal each time a peer proposes a tunnel and "+
		"each time a configuration is committed; to release it now, remove it with the dataplane's own "+
		"command: %s; the next proposal then releases it, or as a last resort restart Ze",
		errInnerSelectorStranded, owner, p.text, h.text, holder, commands.String())
}

// claim records child in its registry (RFC 3948 Section 5.1), in either mode: a
// transport-mode Child SA is recorded so no other peer's tunnel can overlap it, while
// its overlap with another transport-mode Child SA is left to the dataplane (RFC 3948
// Section 5.2, conflictLocked).
func (g innerSelectorGuard) claim(child *ChildSA) error {
	// refuse-by-default: guards innerSelectorGuard -- only a Child SA a test built by hand carries no registry; every session-built SA carries SATable.innerSelectors
	if g.registry == nil {
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
	"configuration is committed; to release it now, run the removal commands the error names, each in " +
	"the dataplane's own tool (ip xfrm on the Linux kernel, vppctl on VPP), and the peer's next " +
	"proposal releases it, or as a last resort restart Ze"

// childResponseFixStranded is the fix logged when this node, as the Child SA initiator,
// refuses the answered tunnel for the same reason.
const childResponseFixStranded = "a tunnel another peer held, which one packet could also be " +
	"sent into, left a policy Ze could not remove (RFC 3948 Section 5.1), so this Child SA was deleted " +
	"and the IKE SA kept with no Child SA, asking again on its creation schedule; Ze retries the removal " +
	"at each attempt and each time a configuration is committed; to release it now, run the removal " +
	"commands the error names, each in the dataplane's own tool (ip xfrm on the Linux kernel, vppctl on " +
	"VPP), and the next attempt releases it, or as a last resort restart Ze"

// errInnerSelectorConflict marks a refusal because another peer holds an overlapping
// tunnel (innerSelectorConflict). It wraps errTSUnacceptable, so notifyForRefusal still
// answers TS_UNACCEPTABLE, and childRefusedResponderFix names this fix rather than the
// kernel-install one an install-time refusal would otherwise read as.
var errInnerSelectorConflict = fmt.Errorf("%w: another peer holds an overlapping tunnel or transport-mode connection",
	errTSUnacceptable)

// childRefusedFixResponderInnerSelector is the fix logged beside that refusal.
const childRefusedFixResponderInnerSelector = "another peer already holds a tunnel or a transport-mode connection one packet could also be " +
	"sent into (RFC 3948 Section 5.1), so this peer gets no Child SA until that Child SA goes away; " +
	"configure disjoint remote prefixes for the two peers with " +
	"set vpn ipsec site-to-site peer <name> traffic-selector <n> remote prefix <prefix>, bind the peer " +
	"to a virtual-IP pool with set vpn ipsec site-to-site peer <name> virtual-ip-pool <pool>, or, where " +
	"both peers' configured remote prefixes overlap on purpose, give one of them a different priority " +
	"with set vpn ipsec site-to-site peer <name> policy-priority <n>, then commit"

// childResponseFixInnerSelector is the fix logged when this node, as the Child SA
// initiator, refuses the answered tunnel because another peer holds an overlapping one.
// It names only what the initiator's own configuration controls: a virtual-IP pool is
// a responder's lease, so it is not offered here.
const childResponseFixInnerSelector = "another peer already holds a tunnel or a transport-mode connection one packet could also be " +
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
