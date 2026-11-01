// Design: docs/architecture/update-building.md -- one final ownership and duplicate boundary
// Related: adj_rib_out.go -- the table whose entries this store holds
// Related: session_ownership.go -- recordSection takes a slot, recordDeletion frees it
package reactor

// adjRIBOutEntrySlots is the slot count of one entry-store chunk. Installing a
// key allocates only when the free list is empty and the current chunk is
// full, so a write of N fresh keys allocates at most one chunk per this many
// keys, never one object per key.
const adjRIBOutEntrySlots = 256

// adjRIBOutEntries holds one table's adjRIBOutRoute slots: chunks of
// adjRIBOutEntrySlots entries and a free list of slots a deletion returned.
//
// Every slot ever taken is either the value of one routes key or on the free
// list, so the store is bounded by the session's peak table, the same bound the
// routes map holds. A peer that announces and withdraws in a loop reuses its
// freed slots. A chunk stays alive while any of its slots is referenced, and
// clear drops the current chunk and the free list together with routes.
//
// It is the table's: every method MUST be called with adjRIBOut.mu held. Not
// safe for concurrent use.
type adjRIBOutEntries struct {
	// chunk is the chunk new slots are cut from; cut counts the slots taken.
	chunk *[adjRIBOutEntrySlots]adjRIBOutRoute
	cut   int
	// freed holds zeroed slots a deletion returned, reused last in, first out.
	freed []*adjRIBOutRoute
}

// take returns a zeroed slot for a key being installed in routes. The caller
// MUST store it as that key's value, and MUST return it through free once the
// key is deleted and the entry's last read is done.
func (s *adjRIBOutEntries) take() *adjRIBOutRoute {
	if last := len(s.freed) - 1; last >= 0 {
		entry := s.freed[last]
		s.freed[last] = nil
		s.freed = s.freed[:last]
		// Zeroed again before use: a slot is never installed carrying another
		// key's owner, revision, signature, exact NLRI or contested bit.
		*entry = adjRIBOutRoute{}
		return entry
	}
	if s.chunk == nil || s.cut == adjRIBOutEntrySlots {
		s.chunk = new([adjRIBOutEntrySlots]adjRIBOutRoute)
		s.cut = 0
	}
	entry := &s.chunk[s.cut]
	s.cut++
	return entry
}

// free returns a deleted key's slot. The caller MUST have removed it from
// routes and MUST NOT read or keep it afterwards: the next take hands it to
// another key. Zeroing here drops the source Peer and the signature the slot
// referenced, so a free slot keeps nothing alive. A slot that is not a table
// entry (releasePath's temporary owner) MUST NOT be passed.
func (s *adjRIBOutEntries) free(entry *adjRIBOutRoute) {
	*entry = adjRIBOutRoute{}
	s.freed = append(s.freed, entry)
}

// clear drops every chunk and the free list. It runs with the reset of routes
// (clearRoutes), after which no slot of the old table is referenced.
func (s *adjRIBOutEntries) clear() {
	*s = adjRIBOutEntries{}
}
