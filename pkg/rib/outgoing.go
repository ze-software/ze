package rib

import (
	"sync"

	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
)

// OutgoingRIB (Adj-RIB-Out) manages routes to be announced to peers.
//
// Maintains pending announcements and withdrawals, organized by address family.
type OutgoingRIB struct {
	mu sync.RWMutex

	// pending maps family -> route index -> route (announcements)
	pending map[nlri.Family]map[string]*Route

	// withdrawals maps family -> NLRI index -> NLRI (withdrawals)
	withdrawals map[nlri.Family]map[string]nlri.NLRI

	// sent tracks what was last sent (for resend on reconnect)
	sent map[nlri.Family]map[string]*Route
}

// NewOutgoingRIB creates a new Adj-RIB-Out.
func NewOutgoingRIB() *OutgoingRIB {
	return &OutgoingRIB{
		pending:     make(map[nlri.Family]map[string]*Route),
		withdrawals: make(map[nlri.Family]map[string]nlri.NLRI),
		sent:        make(map[nlri.Family]map[string]*Route),
	}
}

// QueueAnnounce queues a route for announcement.
// If a withdrawal for this NLRI is pending, it is cancelled.
func (r *OutgoingRIB) QueueAnnounce(route *Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	family := route.NLRI().Family()
	idx := string(route.Index())

	// Get or create family's pending map
	familyPending, ok := r.pending[family]
	if !ok {
		familyPending = make(map[string]*Route)
		r.pending[family] = familyPending
	}

	// Cancel any pending withdrawal for this NLRI
	if familyWithdrawals, ok := r.withdrawals[family]; ok {
		delete(familyWithdrawals, idx)
	}

	familyPending[idx] = route
}

// QueueWithdraw queues a route withdrawal.
// If an announcement for this NLRI is pending, it is cancelled.
func (r *OutgoingRIB) QueueWithdraw(n nlri.NLRI) {
	r.mu.Lock()
	defer r.mu.Unlock()

	family := n.Family()

	// Build an index from the NLRI (without AS-PATH since withdrawals don't have attributes)
	// For withdrawal matching, we use just Family + NLRI bytes
	idx := string(buildNLRIIndex(n))

	// Cancel any pending announcement for this NLRI
	if familyPending, ok := r.pending[family]; ok {
		// Need to find and remove any matching announcement
		for pendingIdx := range familyPending {
			// Check if this pending route matches the NLRI being withdrawn
			// The pending index includes AS-PATH hash, so we compare NLRI portion only
			if matchesNLRI(pendingIdx, idx) {
				delete(familyPending, pendingIdx)
			}
		}
	}

	// Get or create family's withdrawal map
	familyWithdrawals, ok := r.withdrawals[family]
	if !ok {
		familyWithdrawals = make(map[string]nlri.NLRI)
		r.withdrawals[family] = familyWithdrawals
	}

	familyWithdrawals[idx] = n
}

// buildNLRIIndex builds an index for an NLRI (without AS-PATH).
func buildNLRIIndex(n nlri.NLRI) []byte {
	family := n.Family()
	nlriBytes := n.Bytes()

	buf := make([]byte, 3+len(nlriBytes))
	buf[0] = byte(family.AFI >> 8)
	buf[1] = byte(family.AFI)
	buf[2] = byte(family.SAFI)
	copy(buf[3:], nlriBytes)

	return buf
}

// matchesNLRI checks if a route index matches an NLRI index.
// Route index = Family + NLRI + AS-PATH hash.
// NLRI index = Family + NLRI.
func matchesNLRI(routeIdx, nlriIdx string) bool {
	// NLRI index is a prefix of route index
	return len(routeIdx) >= len(nlriIdx) && routeIdx[:len(nlriIdx)] == nlriIdx
}

// GetPending returns pending routes for a family without clearing them.
func (r *OutgoingRIB) GetPending(family nlri.Family) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	familyPending, ok := r.pending[family]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(familyPending))
	for _, route := range familyPending {
		routes = append(routes, route)
	}

	return routes
}

// FlushPending returns and clears pending routes for a family.
func (r *OutgoingRIB) FlushPending(family nlri.Family) []*Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	familyPending, ok := r.pending[family]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(familyPending))
	for _, route := range familyPending {
		routes = append(routes, route)
	}

	// Clear pending
	delete(r.pending, family)

	// Add to sent cache
	if r.sent[family] == nil {
		r.sent[family] = make(map[string]*Route)
	}
	for idx, route := range familyPending {
		r.sent[family][idx] = route
	}

	return routes
}

// GetWithdrawals returns pending withdrawals for a family.
func (r *OutgoingRIB) GetWithdrawals(family nlri.Family) []nlri.NLRI {
	r.mu.RLock()
	defer r.mu.RUnlock()

	familyWithdrawals, ok := r.withdrawals[family]
	if !ok {
		return nil
	}

	nlris := make([]nlri.NLRI, 0, len(familyWithdrawals))
	for _, n := range familyWithdrawals {
		nlris = append(nlris, n)
	}

	return nlris
}

// FlushWithdrawals returns and clears pending withdrawals for a family.
func (r *OutgoingRIB) FlushWithdrawals(family nlri.Family) []nlri.NLRI {
	r.mu.Lock()
	defer r.mu.Unlock()

	familyWithdrawals, ok := r.withdrawals[family]
	if !ok {
		return nil
	}

	nlris := make([]nlri.NLRI, 0, len(familyWithdrawals))
	for _, n := range familyWithdrawals {
		nlris = append(nlris, n)
	}

	// Clear withdrawals
	delete(r.withdrawals, family)

	// Remove from sent cache
	if sentFamily, ok := r.sent[family]; ok {
		for idx := range familyWithdrawals {
			for sentIdx := range sentFamily {
				if matchesNLRI(sentIdx, idx) {
					delete(sentFamily, sentIdx)
				}
			}
		}
	}

	return nlris
}

// Stats returns statistics about the OutgoingRIB.
func (r *OutgoingRIB) Stats() OutgoingRIBStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := OutgoingRIBStats{}

	for _, familyPending := range r.pending {
		stats.PendingAnnouncements += len(familyPending)
	}

	for _, familyWithdrawals := range r.withdrawals {
		stats.PendingWithdrawals += len(familyWithdrawals)
	}

	for _, familySent := range r.sent {
		stats.SentRoutes += len(familySent)
	}

	return stats
}

// OutgoingRIBStats holds statistics about the OutgoingRIB.
type OutgoingRIBStats struct {
	PendingAnnouncements int
	PendingWithdrawals   int
	SentRoutes           int
}
