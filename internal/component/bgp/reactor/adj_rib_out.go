// RFC: rfc/short/rfc4271.md -- Adj-RIB-Out (Section 3.2) and the Update-Send Process (Section 9.2)
// Design: docs/architecture/update-building.md -- the API origination rail this table guards
// Overview: reactor_api_batch.go -- announceBatchToPeers and withdrawBatchFromPeers, the only writers
// Related: forward_dedup.go -- the fan-out's per-call dedup, which shares a BUILD rather than suppressing a SEND
package reactor

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// One peer's Adj-RIB-Out over the API origination rails.
//
// RFC 4271 Section 3.2: "The Adj-RIBs-Out stores information the local BGP
// speaker selected for advertisement to its peers. The routing information
// stored in the Adj-RIBs-Out will be carried in the local BGP speaker's UPDATE
// messages and advertised to its peers."
//
// It exists to answer ONE question, which RFC 4271 Section 9.2 asks: "A BGP
// speaker SHOULD NOT advertise a given feasible BGP route from its Adj-RIB-Out
// if it would produce an UPDATE message containing the same BGP route as was
// previously advertised." Until this table existed, `announce route X` twice put
// two identical UPDATEs on the wire, and an operator script that re-announces
// its whole set on a timer re-flooded every peer on every tick.
//
// What it stores is the route AS THIS PEER RECEIVED IT, never the route the
// caller described. The key is the NLRI exactly as written to the wire, so a
// peer that negotiated ADD-PATH (RFC 7911 Section 3) keys on the path identifier
// too. The value is the attribute block the builder emitted for THIS peer, after
// next-hop resolution, the AS_PATH prepend, the LOCAL_PREF decision and every
// other per-peer edit announceFacts carries. Comparing anything earlier would
// suppress a route whose bytes had genuinely changed.
//
// Bound: one entry for each route this session has advertised and not withdrawn.
// That is the size of what the peer holds, which is the quantity the table is a
// model of, so it cannot be capped without lying about the peer's state. A
// withdrawal removes the entry, and a session teardown drops the whole table
// (Peer.clearEncodingContexts), because the next session's peer starts with
// nothing.
//
// Safe for concurrent use. Several plugin goroutines announce to one peer.
type adjRIBOut struct {
	mu sync.Mutex
	// families is nil until the first route is recorded. A peer that is only
	// ever forwarded to, never originated to, pays one nil check.
	families map[family.Family]map[string]*adjRIBOutRoute
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

// adjRIBOutRoute is one route this peer holds.
//
// It is a POINTER in the map so a re-advertisement whose bytes changed updates
// the signature in place. Assigning a fresh value into `map[string]T` would
// allocate the key string again on every update, where a lookup of `m[string(b)]`
// allocates nothing (the compiler elides the conversion for a map READ only).
type adjRIBOutRoute struct {
	// signature is one adjRIBOutSignature materialized: the attribute block as
	// written to this peer, with the MP_REACH_NLRI payload and the length octets
	// that counted it removed. Every route of one batch shares the one copy, so
	// the per-route cost is the key plus this pointer.
	signature []byte
}

// adjRIBOutSignature is one route's wire form as this peer would receive it,
// carried as byte ranges of the BUILT UPDATE rather than as a buffer of its own.
// Its value is the concatenation of its parts, in order.
//
// It is several ranges rather than one because RFC 4760 Section 3 puts the NLRI
// INSIDE the MP_REACH_NLRI attribute for every family except IPv4 unicast. The
// key already carries this route's own NLRI, so two things are cut out of the
// attribute block: the batch's NLRI payload, and the length octets of the
// attribute that carried it. Both are properties of how many prefixes shared one
// build, and neither is a property of THIS route. Cutting them is what makes one
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

// unchanged reports whether this peer already holds this exact route.
//
// A false answer is the safe direction and every uncertain case takes it: an
// empty table, a family never advertised, a route never advertised, and a route
// whose bytes differ by one octet all answer false and send. The table can only
// ever suppress a send it can prove is redundant.
func (a *adjRIBOut) unchanged(fam family.Family, key []byte, sig adjRIBOutSignature) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	routes := a.families[fam]
	if routes == nil {
		return false
	}
	entry := routes[string(key)]
	if entry == nil {
		return false
	}
	return sig.equal(entry.signature)
}

// record states that this peer has been sent this route with these bytes.
//
// signature MUST be an owned copy (adjRIBOutSignature.store), because the table
// keeps it after the caller's pooled build buffer is returned. One batch's
// routes share one copy.
func (a *adjRIBOut) record(fam family.Family, key, signature []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.families == nil {
		a.families = make(map[family.Family]map[string]*adjRIBOutRoute)
	}
	routes := a.families[fam]
	if routes == nil {
		routes = make(map[string]*adjRIBOutRoute)
		a.families[fam] = routes
	}
	if entry := routes[string(key)]; entry != nil {
		entry.signature = signature
		return
	}
	routes[string(key)] = &adjRIBOutRoute{signature: signature}
}

// forget states that this peer no longer holds this route, so a later announce
// of it is sent again.
//
// It is called for every withdrawal the rail issues, whether or not the write
// succeeded. A failed write leaves the peer's real state unknown, and the safe
// reading of "unknown" is "the peer does not have it": that re-sends a route the
// peer may already hold, where the opposite would suppress one it does not.
func (a *adjRIBOut) forget(fam family.Family, key []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()

	routes := a.families[fam]
	if routes == nil {
		return
	}
	delete(routes, string(key))
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

	a.families = nil
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

// nlriWireAt reads one NLRI's own bytes out of a block writeBatchNLRI produced,
// and answers where the next one starts.
//
// The walk is the writer's own: writeBatchNLRI advances by nlri.LenWithContext
// for each NLRI in order, and TestNLRILenMatchesWriteNLRI pins that the length
// and the write agree. The bound is checked here anyway, because a reader that
// trusted the walk would slice past the block on the first disagreement.
func nlriWireAt(block []byte, route nlri.NLRI, addPath bool, off int) (wire []byte, end int, ok bool) {
	size := nlri.LenWithContext(route, addPath)
	if size < 0 || off+size > len(block) {
		return nil, off, false
	}
	return block[off : off+size], off + size, true
}

// announceUnit is one built UPDATE, plus everything a peer's Adj-RIB-Out needs
// to decide whether that peer is owed it.
//
// It is built once per unit and read once per peer, so the signature and the
// owned copy of it are computed at most once each however wide the fan-out is.
type announceUnit struct {
	batch bgptypes.NLRIBatch
	facts announceFacts
	// block is the NLRI bytes writeBatchNLRI wrote for this unit. It is the
	// caller's pooled buffer, so nothing derived from it outlives the unit
	// except through adjRIBOutSignature.store.
	block []byte
	sig   adjRIBOutSignature
	// readable is false when the built UPDATE could not be read as a per-route
	// signature. Every question then answers "this peer is owed it", which is
	// the rail's behavior before the Adj-RIB-Out existed.
	readable bool
	// stored is the owned copy of sig, materialized by the first peer that
	// records against it and shared by every peer after.
	stored []byte
}

// newAnnounceUnit reads a built UPDATE's per-route signature.
func newAnnounceUnit(update *message.Update, block []byte, unit bgptypes.NLRIBatch, facts announceFacts) announceUnit {
	built := announceUnit{batch: unit, facts: facts, block: block}
	sig, ok := announceSignature(update.PathAttributes, unit.Family, nlriBlockLen(unit.NLRIs, facts.addPath))
	built.sig, built.readable = sig, ok
	return built
}

// nlriBlockLen is the byte count writeBatchNLRI writes for these NLRIs.
//
// It repeats the writer's own walk rather than taking the count back from the
// build, because the two callers that need it need it for different families and
// only the IPv4 unicast build hands its NLRI block back on the Update. A
// disagreement between this sum and the write is what TestNLRILenMatchesWriteNLRI
// pins.
func nlriBlockLen(nlris []nlri.NLRI, addPath bool) int {
	total := 0
	for _, route := range nlris {
		size := nlri.LenWithContext(route, addPath)
		if size < 0 {
			return 0
		}
		total += size
	}
	return total
}

// heldBy is how many of this unit's prefixes the peer already holds with exactly
// these bytes.
//
// Zero is the answer for a replay, whatever the table says. A replay exists to
// put back what the peer asked for again -- `clear bgp rib out`, and the
// RFC 2918 Section 3 refresh behind it -- and suppressing it would answer a
// refresh request with silence.
func (u *announceUnit) heldBy(peer *Peer) int {
	if !u.readable || u.batch.Replay {
		return 0
	}
	held, off := 0, 0
	for _, route := range u.batch.NLRIs {
		wire, next, ok := nlriWireAt(u.block, route, u.facts.addPath, off)
		if !ok {
			return 0
		}
		off = next
		if peer.adjOut.unchanged(u.batch.Family, wire, u.sig) {
			held++
		}
	}
	return held
}

// recordAll states that this peer has been sent every prefix of the unit.
func (u *announceUnit) recordAll(peer *Peer) {
	if !u.readable {
		return
	}
	off := 0
	for _, route := range u.batch.NLRIs {
		wire, next, ok := nlriWireAt(u.block, route, u.facts.addPath, off)
		if !ok {
			return
		}
		off = next
		peer.recordAnnounced(u.batch.Family, wire, u.storedSignature())
	}
}

// storedSignature is the owned copy every entry of this unit shares.
//
// It is materialized on the first record rather than at construction, because a
// fan-out in which every peer already holds the batch is exactly the case this
// table exists for and it must allocate nothing.
func (u *announceUnit) storedSignature() []byte {
	if u.stored == nil {
		u.stored = u.sig.store()
	}
	return u.stored
}

// announcePartialToPeers serves the peers whose Adj-RIB-Out suppressed part of a
// unit, each from its own build over the prefixes it is still owed.
//
// It takes a second buffer pair rather than reusing the caller's, because the
// caller's still holds the shared build the signature and the per-NLRI keys are
// read out of. Only a partial peer pays for it, and a partial peer needs a build
// of its own in any case.
func (a *reactorAPIAdapter) announcePartialToPeers(peers []*Peer, u *announceUnit, maxMsgSize int) (int, error) {
	attrHandle := getBuildBuf()
	nlriHandle := getBuildBuf()
	defer putBuildBuf(attrHandle)
	defer putBuildBuf(nlriHandle)

	// owed and owedWire are the same prefixes in the two forms the two steps
	// need: the typed NLRI the build takes, and the wire bytes the Adj-RIB-Out
	// keys on. Both slice into memory the caller owns for the whole pass, so one
	// pair is reused by every peer.
	owed := make([]nlri.NLRI, 0, len(u.batch.NLRIs))
	owedWire := make([][]byte, 0, len(u.batch.NLRIs))
	sent := 0
	var lastErr error

	for _, peer := range peers {
		owed, owedWire = owed[:0], owedWire[:0]
		off := 0
		for _, route := range u.batch.NLRIs {
			wire, next, ok := nlriWireAt(u.block, route, u.facts.addPath, off)
			if !ok {
				break
			}
			off = next
			if peer.adjOut.unchanged(u.batch.Family, wire, u.sig) {
				continue
			}
			owed = append(owed, route)
			owedWire = append(owedWire, wire)
		}
		peer.adjOut.recordSuppressed(len(u.batch.NLRIs) - len(owed))
		logAnnounceSuppressed(peer, u.batch, len(u.batch.NLRIs)-len(owed))
		if len(owed) == 0 {
			sent++
			continue
		}

		unit := u.batch
		unit.NLRIs = owed
		update, buildErr := a.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, unit, u.facts)
		if update == nil {
			// Fewer prefixes than a build that already succeeded, so this is not
			// reachable through an oversize batch. Report it rather than drop it.
			lastErr = buildErr
			continue
		}
		if err := peer.sendUpdateWithSplit(update, maxMsgSize, u.facts.addPath); err != nil {
			lastErr = err
			continue
		}
		for _, wire := range owedWire {
			peer.recordAnnounced(u.batch.Family, wire, u.storedSignature())
		}
		sent++
	}
	return sent, lastErr
}

// logAnnounceSuppressed says which prefixes a peer was not sent, and why.
//
// A route withheld because the peer already has it and a route dropped because
// the send failed are different outcomes, and an operator reading a peer that
// received nothing must be able to tell them apart (ai/rules/principles.md).
// The counter beside it is adjRIBOut.suppressedCount.
func logAnnounceSuppressed(peer *Peer, unit bgptypes.NLRIBatch, routes int) {
	if routes <= 0 {
		return
	}
	routesLogger().Debug("announce suppressed: the peer already holds these routes with these attributes",
		"peer", peer.Settings().Address,
		"family", unit.Family,
		"routes", routes,
		"batch", len(unit.NLRIs))
}

// forgetWithdrawn takes every prefix of a withdrawal unit back out of one peer's
// Adj-RIB-Out, so a later announce of the same route is sent again rather than
// suppressed.
//
// block is the NLRI bytes writeBatchNLRI wrote for this unit, which is the same
// encoding the announce rail keyed on: RFC 7911 Section 3 puts the path
// identifier in front of a withdrawn NLRI exactly as it does an announced one,
// so one peer's announce and withdraw of one route produce one key.
func forgetWithdrawn(peer *Peer, unit bgptypes.NLRIBatch, block []byte, addPath bool) {
	off := 0
	for _, route := range unit.NLRIs {
		wire, next, ok := nlriWireAt(block, route, addPath, off)
		if !ok {
			return
		}
		off = next
		peer.adjOut.forget(unit.Family, wire)
	}
}
