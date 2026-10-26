// Design: docs/architecture/update-building.md -- final wire ownership and local duplicate evidence
// RFC: rfc/short/rfc4271.md -- Adj-RIB-Out and Update-Send Process
// Related: session_ownership.go -- serialized native path admission and recording
// Related: session_write.go -- buffered frontier, flush and failure retirement
package reactor

import (
	"bytes"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// adjRIBOut is one destination Session's ordered buffered ownership frontier.
//
// RFC 4271 Section 3.2: "The Adj-RIBs-Out stores information the local BGP
// speaker selected for advertisement to its peers."
//
// Its writers are final writers under writeMu. The destination's recovery
// worker also reads the owed set under mu, without being a final writer. A
// successful flush commits pending changes; a failed write retires the session
// and invalidates the table. There is no independently readable committed cache
// or second owner inventory. The bound is the native paths currently advertised
// to this recipient, and the owed set holds at most one record per such path.
// mu also serializes session binding and teardown with a replacement writer.
type adjRIBOut struct {
	mu      sync.Mutex
	session *Session
	routes  map[adjOutKey]*adjRIBOutRoute
	// sourcePaths is a derived index of source-owned ADD-PATH slots, keyed
	// before outgoing identifier generation. It has the same lifetime as
	// routes, not the recent-update cache or its allocator mappings.
	sourcePaths map[adjOutSourceKey]uint32
	// owed holds the keys whose owner withdrawal this destination did not
	// emit, because the RIB elects what replaces the route
	// (adjOutWrite.allow). Every reset of routes clears it: a record is framed
	// for the session it was owed to.
	owed map[adjOutKey]*adjOutOwed
	// tombstones holds the keys of source-owned entries a write deleted
	// outside a recovery decision, at a destination without ADD-PATH send:
	// a candidate still in flight to this writer, or dropped on a generation
	// move, would otherwise meet no entry and be written with no election.
	// Every reset of routes clears it, and it holds at most one key per
	// prefix so deleted in this session.
	tombstones map[adjOutKey]struct{}
	pending    bool
	// suppressed counts the routes this peer was NOT sent because it already
	// held them. A suppression and a drop are different outcomes and an operator
	// must be able to tell them apart (ai/rules/principles.md), so the count is
	// kept beside the debug line announceBatchToPeers writes.
	suppressed atomic.Uint64
	// withheld counts the routes this peer was NOT withdrawn from, because the
	// session had advertised nothing for a withdrawal to name (RFC 4271
	// Section 4.3, withdrawBatchFromPeers). It sits beside suppressed for the
	// same reason: an operator reading a peer that received no UPDATE must be
	// able to tell which decision produced the silence.
	withheld atomic.Uint64
}

// adjOutAuthority separates forwarded paths from fenced maintenance.
// Unspecified is invalid for a source-bound writer, never local permission.
type adjOutAuthority uint8

const (
	adjOutUnspecified adjOutAuthority = iota
	adjOutForwarded
	adjOutExpected
	adjOutRecovery
	adjOutInitial
)

// adjOutPath is borrowed producer provenance for one final native path.
// Ordinary withdrawal equality deliberately excludes revision.
type adjOutPath struct {
	source   *Peer
	received uint32
	addPath  bool
	revision uint64
	// generation is the owner's source session generation
	// (Peer.forwardGeneration) when the path was written. Ownership equality
	// ignores it, so a GR-retained path stays withdrawable by the same source
	// on its new session; it tells which session of the source wrote it.
	generation uint64
	// revocation is the owner's Peer.forwardRevocation when the path was
	// written. Equality ignores it like the generation; adjOutOwnerCurrent reads
	// both.
	revocation uint64
	// message is the owner's RECEIVED message ID, the revision of the source's
	// route the destination holds. revision is the SENT receipt's ID and
	// cannot say which received revision that was, so an elected candidate is
	// compared with this field (recoveryAdmission.admit). Equality ignores it.
	message uint64
}

// adjOutAnnouncer is one source session that announced an owed key: the
// source and its session generation (Peer.forwardGeneration) at the write.
type adjOutAnnouncer struct {
	source     *Peer
	generation uint64
}

// adjOutExclusion is one source a record's election skips, bounded by the
// newest forward generation (Peer.forwardGeneration) of that source's sessions
// the exclusion covers: a departure's ended session, or a chosen survivor's
// session that ended before its write. A later session of the source is not
// excluded, so its route, graceful-restart retained or live, stays electable.
// departed is the newest session a DEPARTURE covers, zero when none did: only
// a departure says a path is gone, so only it lets a write withdraw one. A
// survivor that could not be written says only that it cannot be sent.
type adjOutExclusion struct {
	source     netip.Addr
	generation uint64
	departed   uint64
}

// adjOutOwed is one key whose decision belongs to the destination's recovery
// worker. It names the destination session it was owed to, and carries a copy
// of the destination-framed NLRI, because the writer's bytes are pooled. A key
// keeps ONE record while it is owed: every later owe merges into it, so a
// round deciding the key never loses an owe made while it ran. Every field is
// read and written under adjRIBOut.mu.
type adjOutOwed struct {
	session *Session
	family  family.Family
	nlri    []byte
	// excluded names the sources whose candidates the election skips, each
	// up to its session bound: a departed source, and a chosen survivor whose
	// session ended before its write. An owner withdrawal or a lifecycle
	// removal excludes nobody. Bounded by the configured sources.
	excluded []adjOutExclusion
	// epoch moves whenever the question asked of the RIB changes (a source
	// is excluded), so an admission answering an older question leaves the
	// key owed for the next round instead of writing a stale answer.
	epoch uint64
	// movers are the owner identities of forwarded WITHDRAWALS of the key
	// since the last round's snapshot. Only the chosen candidate's own
	// withdrawal makes a decision's route dead, so the admission compares the
	// candidate with this set. Bounded by the distinct paths that wrote.
	movers []adjOutPath
	// announced reports a forwarded or initial-replay ANNOUNCEMENT of the key
	// since the last round's snapshot: a route the round's drain may not
	// have applied. The admission still decides, and leaves the key owed with
	// its epoch moved, so the next round elects with the newer route.
	announced bool
	// announcers are the source sessions that announced the key while it was
	// owed. Unlike movers it is not cleared at snapshot and dies with the
	// record: a round subtracts from excluded every source whose announcer is
	// still current or newer than the exclusion's bound, so a reconnected
	// source's new route stays electable whichever of its departure and its
	// re-announcement reached the writer first, and after that new session
	// ended under graceful restart. Bounded by the configured sources.
	announcers []adjOutAnnouncer
	// done is closed when the record leaves the owed set, whatever removed
	// it: a decision, an originated announcement, or a table reset. A
	// departure waits on it.
	done chan struct{}
}

// owe returns the key's record, creating it when the key is not owed. The
// copy is deliberate: raw is the write's staging storage (adjOutWrite.stageOwe),
// which the pooled writer reuses, and the record outlives the write. MUST be
// called with mu held.
func (a *adjRIBOut) owe(key adjOutKey, session *Session, fam family.Family, raw []byte) *adjOutOwed {
	if record := a.owed[key]; record != nil {
		return record
	}
	if a.owed == nil {
		a.owed = make(map[adjOutKey]*adjOutOwed)
	}
	record := &adjOutOwed{session: session, family: fam, nlri: append([]byte(nil), raw...),
		done: make(chan struct{})}
	a.owed[key] = record
	return record
}

// exclude merges exclusion into the record's excluded set, raising each bound
// of an existing exclusion of the source and never lowering one, and moves
// the epoch, so a round that asked without it never settles the key. An older
// session's departure merged later therefore never narrows what a newer one
// covers.
func (o *adjOutOwed) exclude(exclusion adjOutExclusion) {
	o.epoch++
	for i := range o.excluded {
		existing := &o.excluded[i]
		if existing.source != exclusion.source {
			continue
		}
		existing.generation = max(existing.generation, exclusion.generation)
		existing.departed = max(existing.departed, exclusion.departed)
		return
	}
	o.excluded = append(o.excluded, exclusion)
}

// exclusionOf returns the record's exclusion of source, and whether it has one.
func (o *adjOutOwed) exclusionOf(source netip.Addr) (adjOutExclusion, bool) {
	for _, exclusion := range o.excluded {
		if exclusion.source == source {
			return exclusion, true
		}
	}
	return adjOutExclusion{}, false
}

// spares reports whether owner's path is newer than every departure the
// record's exclusion of owner's source carries: written by a session no
// departure covers, live or retained by graceful restart. A recovery write
// withdraws nothing such a path holds. RFC 4724 Section 4.2: "it MUST
// retain the routes received from the peer for all the address families that
// were previously received in the Graceful Restart Capability and MUST mark
// them as stale routing information." MUST be called with adjRIBOut.mu held.
func (o *adjOutOwed) spares(owner adjOutPath) bool {
	if owner.source == nil {
		return false
	}
	exclusion, excluded := o.exclusionOf(owner.source.Settings().Address)
	if !excluded {
		return false
	}
	return owner.generation > exclusion.departed
}

// noteAnnouncer records a source-bound announcement of the key by owner: an
// ANNOUNCE mover, and owner's session as a current announcer. A source the
// record excludes moves the epoch on entering the set, because the next
// round subtracts it and so asks the RIB another question. MUST be called
// with adjRIBOut.mu held.
func (o *adjOutOwed) noteAnnouncer(owner adjOutPath, mover bool) {
	if mover {
		o.announced = true
	}
	for i := range o.announcers {
		if o.announcers[i].source == owner.source {
			o.announcers[i].generation = owner.generation
			return
		}
	}
	o.announcers = append(o.announcers, adjOutAnnouncer{source: owner.source, generation: owner.generation})
	if _, excluded := o.exclusionOf(owner.source.Settings().Address); excluded {
		o.epoch++
	}
}

// electableExcluded returns the excluded sources a round asks the RIB to
// skip: excluded minus every source with an announcer that is still current
// or newer than the exclusion's bound. A current session's route for the
// key is a valid candidate, a newer session's route that graceful restart
// retained is one too, and the RIB already released or replaced the departed
// session's paths unless GR retained them. The result is a fresh slice the
// caller may append to. MUST be called with adjRIBOut.mu held.
func (o *adjOutOwed) electableExcluded() []netip.Addr {
	excluded := make([]netip.Addr, 0, len(o.excluded))
	for _, exclusion := range o.excluded {
		if o.announcerLifts(exclusion) {
			continue
		}
		excluded = append(excluded, exclusion.source)
	}
	return excluded
}

// announcerLifts reports whether an announcer of the exclusion's source lifts
// it: its session is current, or newer than the exclusion's bound. Every
// announcer of the address is asked, because a Peer rebuilt for the address
// is a second announcer beside the old one.
func (o *adjOutOwed) announcerLifts(exclusion adjOutExclusion) bool {
	for i := range o.announcers {
		announcer := &o.announcers[i]
		if announcer.source.Settings().Address != exclusion.source {
			continue
		}
		if forwardSourceCurrent(announcer.source, announcer.generation) {
			return true
		}
		// RFC 4724 Section 4.2: "The router MUST NOT differentiate between
		// stale and other routing information during forwarding."
		if announcer.generation > exclusion.generation {
			return true
		}
	}
	return false
}

// noteMover records a forwarded withdrawal of the key by owner.
func (o *adjOutOwed) noteMover(owner adjOutPath) {
	for i := range o.movers {
		if adjOutSameOwner(o.movers[i], owner) {
			return
		}
	}
	o.movers = append(o.movers, owner)
}

// moved reports whether candidate withdrew the key since the snapshot.
func (o *adjOutOwed) moved(candidate adjOutPath) bool {
	for i := range o.movers {
		if adjOutSameOwner(o.movers[i], candidate) {
			return true
		}
	}
	return false
}

// removeOwed removes the key's record when it is still record, and completes
// its waiters. MUST be called with mu held.
func (a *adjRIBOut) removeOwed(key adjOutKey, record *adjOutOwed) {
	if record == nil {
		return
	}
	if a.owed[key] != record {
		return
	}
	delete(a.owed, key)
	close(record.done)
}

// dropOwed empties the owed set with the table it belongs to, and completes
// every waiter: the session that owed the keys is gone, and so is every route
// the keys could have replaced. MUST be called with mu held.
func (a *adjRIBOut) dropOwed() {
	for _, record := range a.owed {
		close(record.done)
	}
	a.owed = nil
}

// adjOutKey adds presence to fwdPathKey: identifier zero is a valid identifier.
type adjOutKey struct {
	path    fwdPathKey
	addPath bool
}

// adjOutSourceKey names an ingress path independently of its outgoing ID.
// A nil source is not indexed; the zero key is therefore uninitialized.
type adjOutSourceKey struct {
	path    fwdPathKey
	source  *Peer
	addPath bool
}

func adjOutSourceKeyFor(key *adjOutKey, owner adjOutPath) adjOutSourceKey {
	out := adjOutSourceKey{path: key.path, source: owner.source, addPath: owner.addPath}
	out.path.received = owner.received
	return out
}

// setOwner updates the derived ingress index when a recorded slot changes
// ownership. The caller MUST hold mu and retain a newly installed outgoing ID.
func (a *adjRIBOut) setOwner(key *adjOutKey, entry *adjRIBOutRoute, owner adjOutPath) {
	if key.addPath {
		if !adjOutSameOwner(entry.owner, owner) {
			if entry.owner.source != nil {
				delete(a.sourcePaths, adjOutSourceKeyFor(key, entry.owner))
			}
			if owner.source != nil {
				if a.sourcePaths == nil {
					a.sourcePaths = make(map[adjOutSourceKey]uint32)
				}
				a.sourcePaths[adjOutSourceKeyFor(key, owner)] = key.path.received
			}
		}
	}
	entry.owner = owner
}

// releasePath drops a recorded slot's identifier reservation and derived
// ingress binding. The caller MUST hold mu and remove the slot from routes.
func (a *adjRIBOut) releasePath(key *adjOutKey, entry *adjRIBOutRoute) {
	if !key.addPath {
		return
	}
	if entry.owner.source != nil {
		delete(a.sourcePaths, adjOutSourceKeyFor(key, entry.owner))
	}
	fwdPathIDs.release(key.path.received)
}

// adjRIBOutRoute retains no forwarded attributes. Local advertisements share
// one signature per body and keep exact NLRI only when the semantic key loses
// wire information (labels, framing, or noncanonical prefix bits).
type adjRIBOutRoute struct {
	owner     adjOutPath
	revision  uint64
	signature []byte
	exactNLRI []byte
	// contested is sticky: some candidate other than this owner reached the
	// writer for the key since the entry was installed (another source's
	// announcement, an announcement meeting a record or a tombstone, or a
	// recovery write). The owner's own re-announcement is the election only
	// while it is clear. It dies with the entry.
	contested bool
}

// tombstone notes a deleted source-owned key. MUST be called with mu held.
func (a *adjRIBOut) tombstone(key adjOutKey) {
	if a.tombstones == nil {
		a.tombstones = make(map[adjOutKey]struct{})
	}
	a.tombstones[key] = struct{}{}
}

// tombstoned reports whether key is tombstoned. The emptiness test keeps the
// common write free of a lookup. MUST be called with mu held.
func (a *adjRIBOut) tombstoned(key adjOutKey) bool {
	if len(a.tombstones) == 0 {
		return false
	}
	_, found := a.tombstones[key]
	return found
}

// clearRoutes empties routes and everything framed for them: source bindings,
// identifier reservations, the owed set and tombstones. MUST hold mu.
func (a *adjRIBOut) clearRoutes() {
	if len(a.routes) != 0 {
		fwdPathIDs.mu.Lock()
		for key := range a.routes {
			if key.addPath {
				fwdPathIDs.releaseLocked(key.path.received)
			}
		}
		fwdPathIDs.mu.Unlock()
	}
	a.sourcePaths = nil
	a.routes = nil
	a.tombstones = nil
	a.dropOwed()
	a.pending = false
}

// adjRIBOutSignature is one route's wire form as this peer would receive it,
// carried as byte ranges of the BUILT UPDATE rather than as a buffer of its own.
// Its value is the concatenation of its parts, in order.
//
// It is several ranges rather than one because RFC 4760 Section 3 puts the NLRI
// INSIDE the MP_REACH_NLRI attribute for every family except IPv4 unicast.
// The key plus exactNLRI evidence already carries this route's own NLRI, so
// two things are cut out of the attribute block: the batch's NLRI payload and
// the length octets of the attribute that carried it. Both depend on how many
// prefixes shared the build, and neither is a property of THIS route. Cutting them makes one
// route's signature the same whether it traveled alone or beside twenty others.
//
// Reading the ranges in place rather than materializing one buffer is what keeps
// the comparison allocation-free. An unchanged route is the case this table
// exists for, and it must cost nothing.
//
// For IPv4 unicast the NLRI travels in the UPDATE's own field, so the whole
// attribute block is one part and the rest are empty.
type adjRIBOutSignature struct {
	parts [3][]byte
}

// length is the byte count a stored copy of this signature occupies.
func (s adjRIBOutSignature) length() int {
	return len(s.parts[0]) + len(s.parts[1]) + len(s.parts[2])
}

// equal reports whether a stored signature holds exactly these bytes.
func (s adjRIBOutSignature) equal(stored []byte) bool {
	if len(stored) != s.length() {
		return false
	}
	for _, part := range s.parts {
		if !bytes.Equal(stored[:len(part)], part) {
			return false
		}
		stored = stored[len(part):]
	}
	return true
}

// store materializes the signature as one owned copy.
//
// The copy is deliberate: the bytes it is made from are a pooled build buffer
// that the next unit of the same fan-out overwrites, and this table outlives the
// send. One batch's routes share the one copy.
func (s adjRIBOutSignature) store() []byte {
	owned := make([]byte, 0, s.length())
	for _, part := range s.parts {
		owned = append(owned, part...)
	}
	return owned
}

// reset empties the table.
//
// Called on session teardown. RFC 4271 Section 3.2 makes the Adj-RIB-Out a model
// of what a PEER holds, and a peer reached over a new TCP connection holds
// nothing: RFC 4271 Section 6.3 has the receiver delete every route from a
// session that closed. Keeping the table across a teardown would suppress the
// re-advertisement the new session is owed, which is the one way a suppression
// table can blackhole a prefix.
func (a *adjRIBOut) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.clearRoutes()
	a.session = nil
}

// suppressedCount is how many routes this peer was not sent because it already
// held them. Read by tests and by the operator-facing debug line.
func (a *adjRIBOut) suppressedCount() uint64 {
	return a.suppressed.Load()
}

// withheldCount is how many routes this peer was not withdrawn from, because the
// session had advertised nothing. Read by tests and by the operator-facing
// warning withdrawBatchFromPeers writes.
func (a *adjRIBOut) withheldCount() uint64 {
	return a.withheld.Load()
}

// recordWithheld counts routes a withdrawal did not take from one peer.
func (a *adjRIBOut) recordWithheld(routes int) {
	if routes <= 0 {
		return
	}
	a.withheld.Add(uint64(routes)) //nolint:gosec // G115: a batch's NLRI count is non-negative
	adjRIBOutWithheld.Add(uint64(routes))
}

// adjRIBOutSuppressed is the same count over every peer, for a test that has no
// Peer in hand.
var adjRIBOutSuppressed atomic.Uint64

// adjRIBOutWithheld is withheldCount over every peer, for a test that has no
// Peer in hand.
var adjRIBOutWithheld atomic.Uint64

// recordSuppressed counts routes withheld from one peer.
func (a *adjRIBOut) recordSuppressed(routes int) {
	if routes <= 0 {
		return
	}
	a.suppressed.Add(uint64(routes)) //nolint:gosec // G115: a batch's NLRI count is non-negative
	adjRIBOutSuppressed.Add(uint64(routes))
}

// announceSignature reads one built UPDATE's per-route signature.
//
// nlriLen is the byte count writeBatchNLRI wrote for the whole unit, which is
// what has to come out of the MP_REACH_NLRI attribute for the answer to describe
// ONE route rather than the batch it traveled in.
//
// The second return is false when the block cannot be read as expected: for an
// MP family whose built attributes carry no MP_REACH_NLRI, or one whose
// MP_REACH_NLRI is shorter than the NLRI it was given. Neither is reachable
// through buildBatchAnnounceUpdate, which writes the attribute itself and
// refuses the build when it cannot. It is answered rather than asserted because
// the caller's response is the safe one: a signature it cannot read suppresses
// nothing and records nothing, so the rail behaves as it did before this table
// existed (ai/rules/principles.md).
func announceSignature(attrs []byte, fam family.Family, nlriLen int) (adjRIBOutSignature, bool) {
	if fam == family.IPv4Unicast {
		return adjRIBOutSignature{parts: [3][]byte{attrs}}, true
	}

	hdrStart, flags, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found || len(value) < nlriLen {
		return adjRIBOutSignature{}, false
	}

	// RFC 4271 Section 4.3: an attribute header is flags, type code, and then one
	// or two length octets depending on the Extended Length bit. The two kept
	// octets are flags and type code; the length octets are the pair that counts
	// the batch rather than the route, so they are what is cut.
	lengthOctets := 1
	if flags&attribute.FlagExtLength != 0 {
		lengthOctets = 2
	}
	valueStart := hdrStart + 2 + lengthOctets
	valueEnd := valueStart + len(value)

	return adjRIBOutSignature{parts: [3][]byte{
		attrs[:hdrStart+2],
		value[:len(value)-nlriLen],
		attrs[valueEnd:],
	}}, true
}
