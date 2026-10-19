// Design: docs/architecture/update-building.md -- final wire ownership and local duplicate evidence
// RFC: rfc/short/rfc4271.md -- Adj-RIB-Out and Update-Send Process
// Related: session_ownership.go -- serialized native path admission and recording
// Related: session_write.go -- buffered frontier, flush and failure retirement
package reactor

import (
	"bytes"
	"net/netip"
	"slices"
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
	// owed holds the keys whose owner withdrawal this destination did not
	// emit, because the RIB elects what replaces the route
	// (adjOutWrite.allow). Every reset of routes clears it: a record is framed
	// for the session it was owed to.
	owed    map[adjOutKey]*adjOutOwed
	pending bool
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
// Ordinary withdrawal equality deliberately excludes synthesized and revision.
type adjOutPath struct {
	source      *Peer
	received    uint32
	addPath     bool
	synthesized bool
	revision    uint64
	// generation is the owner's source session generation
	// (Peer.forwardGeneration) when the path was written. Ownership equality
	// ignores it, so a GR-retained path stays withdrawable by the same source
	// on its new session; it tells which session of the source wrote it.
	generation uint64
	// revocation is the owner's Peer.forwardRevocation when the path was
	// written. Equality ignores it like the generation; adjOutOwnerCurrent reads
	// both.
	revocation uint64
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
	// excluded names the sources whose every candidate the election skips: a
	// departed source, and a chosen survivor whose session ended before its
	// write. An owner withdrawal or a lifecycle removal excludes nobody.
	excluded []netip.Addr
	// epoch moves whenever the question asked of the RIB changes (a source
	// is excluded), so an admission answering an older question leaves the
	// key owed for the next round instead of writing a stale answer.
	epoch uint64
	// movers are the owner identities of forwarded withdrawals of the key
	// since the last round's snapshot. Only the chosen candidate's own
	// withdrawal makes a decision dead, so the admission compares the
	// candidate with this set. Bounded by the distinct paths that withdrew.
	movers []adjOutPath
	// done is closed when the record leaves the owed set, whatever removed
	// it: a decision, an originated announcement, or a table reset. A
	// departure waits on it.
	done chan struct{}
}

// owe returns the key's record, creating it when the key is not owed. The
// copy is deliberate: the body is the session's write buffer, and the record
// outlives the write. MUST be called with mu held.
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

// exclude adds source to the record's excluded set and moves the epoch, so a
// round that asked without it never settles the key.
func (o *adjOutOwed) exclude(source netip.Addr) {
	o.epoch++
	if slices.Contains(o.excluded, source) {
		return
	}
	o.excluded = append(o.excluded, source)
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

// adjRIBOutRoute retains no forwarded attributes. Local advertisements share
// one signature per body and keep exact NLRI only when the semantic key loses
// wire information (labels, framing, or noncanonical prefix bits).
type adjRIBOutRoute struct {
	owner     adjOutPath
	revision  uint64
	signature []byte
	exactNLRI []byte
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

	a.routes = nil
	a.dropOwed()
	a.session = nil
	a.pending = false
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
