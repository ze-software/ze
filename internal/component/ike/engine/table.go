// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE SA table
// RFC: rfc/short/rfc7296.md -- IKE SA identified by SPI pair (Section 2.6)
package engine

import "sync"

// SATable is a concurrent-safe map of IKE SAs indexed by SPI pair.
type SATable struct {
	mu    sync.RWMutex
	bySPI map[string]*SA
}

// NewSATable creates an empty SA table.
func NewSATable() *SATable {
	return &SATable{bySPI: make(map[string]*SA)}
}

// Insert adds an SA to the table. Returns false if the SPI pair already exists.
func (t *SATable) Insert(sa *SA) bool {
	key := SPIPairKey(sa.InitiatorSPI, sa.ResponderSPI)
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.bySPI[key]; exists {
		return false
	}
	t.bySPI[key] = sa
	return true
}

// Lookup returns the SA for the given SPI pair, or nil if not found.
func (t *SATable) Lookup(initiator, responder [8]byte) *SA {
	key := SPIPairKey(initiator, responder)
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.bySPI[key]
}

// LookupByInitiatorSPI returns the first SA matching the initiator SPI.
// Used for IKE_SA_INIT responses where the responder SPI is not yet known.
func (t *SATable) LookupByInitiatorSPI(initiator [8]byte) *SA {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, sa := range t.bySPI {
		if sa.InitiatorSPI == initiator {
			return sa
		}
	}
	return nil
}

// Remove deletes the SA from the table and returns it.
func (t *SATable) Remove(initiator, responder [8]byte) *SA {
	key := SPIPairKey(initiator, responder)
	t.mu.Lock()
	defer t.mu.Unlock()
	sa := t.bySPI[key]
	delete(t.bySPI, key)
	return sa
}

// RemoveByPeer deletes every SA of one peer session, and returns how many it
// removed. A session that ends removes what it owns, whatever SPI pair its SA
// carries by then. The responder SPI arrives after the handshake starts, and an IKE
// rekey replaces the SA with a different one under a new pair.
//
// One session runs per peer name, because the engine keys its session map by that
// name. The entries of one name are the entries of one session.
//
// A responder session can hold a second, parallel SA of the same name while it
// supersedes the first. A responder removes by SPI pair, and never calls this.
func (t *SATable) RemoveByPeer(peerName string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	removed := 0
	for key, sa := range t.bySPI {
		if sa.PeerName == peerName {
			delete(t.bySPI, key)
			removed++
		}
	}
	return removed
}

// UpdateKey re-indexes an SA after the responder SPI becomes known.
// Removes the old key (with zero responder SPI) and inserts with the full pair.
func (t *SATable) UpdateKey(oldResponder, newResponder [8]byte, sa *SA) {
	oldKey := SPIPairKey(sa.InitiatorSPI, oldResponder)
	newKey := SPIPairKey(sa.InitiatorSPI, newResponder)
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.bySPI, oldKey)
	t.bySPI[newKey] = sa
}

// All returns a snapshot of all SAs in the table.
func (t *SATable) All() []*SA {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*SA, 0, len(t.bySPI))
	for _, sa := range t.bySPI {
		out = append(out, sa)
	}
	return out
}

// Len returns the number of SAs in the table.
func (t *SATable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.bySPI)
}
