// Design: docs/architecture/update-building.md -- final wire ownership and local duplicate evidence
// RFC: rfc/short/rfc4271.md -- Adj-RIB-Out and Update-Send Process
// Related: session_ownership.go -- serialized native path admission and recording
// Related: session_write.go -- buffered frontier, flush and failure retirement
package reactor

import (
	"bytes"
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
// Its only readers are final writers under writeMu. A successful flush commits
// pending changes; a failed write retires the session and invalidates the table.
// There is no independently readable committed cache or second owner inventory.
// The bound is the native paths currently advertised to this recipient.
// mu also serializes session binding and teardown with a replacement writer.
type adjRIBOut struct {
	mu      sync.Mutex
	session *Session
	routes  map[adjOutKey]*adjRIBOutRoute
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
