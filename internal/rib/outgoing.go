package rib

import (
	"errors"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
)

// Transaction errors.
var (
	ErrAlreadyInTransaction = errors.New("already in transaction")
	ErrNoTransaction        = errors.New("no transaction in progress")
	ErrLabelMismatch        = errors.New("transaction label mismatch")
)

// OutgoingRIB (Adj-RIB-Out) manages routes to be announced to peers.
//
// Maintains pending announcements and withdrawals, organized by address family.
// Supports transaction-based batching for atomic route updates.
type OutgoingRIB struct {
	mu sync.RWMutex

	// pending maps family -> route index -> route (announcements)
	pending map[nlri.Family]map[string]*Route

	// withdrawals maps family -> NLRI index -> NLRI (withdrawals)
	withdrawals map[nlri.Family]map[string]nlri.NLRI

	// sent tracks what was last sent (for resend on reconnect)
	sent map[nlri.Family]map[string]*Route

	// Transaction state
	inTransaction bool
	transactionID string

	// Transaction-scoped pending routes (separate from regular pending)
	txPending     map[nlri.Family]map[string]*Route
	txWithdrawals map[nlri.Family]map[string]nlri.NLRI
}

// CommitStats holds statistics from a transaction commit.
type CommitStats struct {
	RoutesAnnounced int
	RoutesWithdrawn int
	RoutesDiscarded int // Only set on rollback
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
// During a transaction, routes are queued to the transaction pending queue.
func (r *OutgoingRIB) QueueAnnounce(route *Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	family := route.NLRI().Family()
	idx := string(route.Index())

	// Choose target maps based on transaction state
	var targetPending map[nlri.Family]map[string]*Route
	var targetWithdrawals map[nlri.Family]map[string]nlri.NLRI

	if r.inTransaction {
		targetPending = r.txPending
		targetWithdrawals = r.txWithdrawals
	} else {
		targetPending = r.pending
		targetWithdrawals = r.withdrawals
	}

	// Get or create family's pending map
	familyPending, ok := targetPending[family]
	if !ok {
		familyPending = make(map[string]*Route)
		targetPending[family] = familyPending
	}

	// Cancel any pending withdrawal for this NLRI
	if familyWithdrawals, ok := targetWithdrawals[family]; ok {
		delete(familyWithdrawals, idx)
	}

	familyPending[idx] = route
}

// QueueWithdraw queues a route withdrawal.
// If an announcement for this NLRI is pending, it is cancelled.
// During a transaction, withdrawals are queued to the transaction withdrawal queue.
func (r *OutgoingRIB) QueueWithdraw(n nlri.NLRI) {
	r.mu.Lock()
	defer r.mu.Unlock()

	family := n.Family()

	// Build an index from the NLRI (without AS-PATH since withdrawals don't have attributes)
	// For withdrawal matching, we use just Family + NLRI bytes
	idx := string(buildNLRIIndex(n))

	// Choose target maps based on transaction state
	var targetPending map[nlri.Family]map[string]*Route
	var targetWithdrawals map[nlri.Family]map[string]nlri.NLRI

	if r.inTransaction {
		targetPending = r.txPending
		targetWithdrawals = r.txWithdrawals
	} else {
		targetPending = r.pending
		targetWithdrawals = r.withdrawals
	}

	// Cancel any pending announcement for this NLRI
	if familyPending, ok := targetPending[family]; ok {
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
	familyWithdrawals, ok := targetWithdrawals[family]
	if !ok {
		familyWithdrawals = make(map[string]nlri.NLRI)
		targetWithdrawals[family] = familyWithdrawals
	}

	familyWithdrawals[idx] = n
}

// buildNLRIIndex builds an index for an NLRI (without AS-PATH).
func buildNLRIIndex(n nlri.NLRI) []byte {
	family := n.Family()
	// Use WriteTo for consistent API - writes same bytes as Bytes()
	nlriLen := n.Len()

	buf := make([]byte, 3+nlriLen)
	buf[0] = byte(family.AFI >> 8)
	buf[1] = byte(family.AFI)
	buf[2] = byte(family.SAFI)
	n.WriteTo(buf, 3)

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

// FlushAllPending returns and clears all pending routes across all families.
func (r *OutgoingRIB) FlushAllPending() []*Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Estimate capacity from pending map sizes
	total := 0
	for _, familyPending := range r.pending {
		total += len(familyPending)
	}
	routes := make([]*Route, 0, total)

	for family, familyPending := range r.pending {
		for idx, route := range familyPending {
			routes = append(routes, route)

			// Add to sent cache
			if r.sent[family] == nil {
				r.sent[family] = make(map[string]*Route)
			}
			r.sent[family][idx] = route
		}
	}

	// Clear all pending
	r.pending = make(map[nlri.Family]map[string]*Route)

	return routes
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

// GetSentRoutes returns all previously sent routes for re-announcement.
// Used when a session re-establishes to replay the RIB to the peer.
func (r *OutgoingRIB) GetSentRoutes() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Estimate capacity from sent map sizes
	total := 0
	for _, familySent := range r.sent {
		total += len(familySent)
	}
	routes := make([]*Route, 0, total)
	for _, familySent := range r.sent {
		for _, route := range familySent {
			routes = append(routes, route)
		}
	}
	return routes
}

// MarkSent records a route as sent, adding it to the sent cache.
// Used when routes are sent immediately (not via transaction/flush).
// This ensures the route will be re-sent on session re-establishment.
func (r *OutgoingRIB) MarkSent(route *Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	family := route.NLRI().Family()
	idx := string(route.Index())

	if r.sent[family] == nil {
		r.sent[family] = make(map[string]*Route)
	}
	r.sent[family][idx] = route
}

// RemoveFromSent removes a route from the sent cache by NLRI.
// Used when a withdrawal is queued to prevent re-announcement on reconnect.
func (r *OutgoingRIB) RemoveFromSent(n nlri.NLRI) {
	r.mu.Lock()
	defer r.mu.Unlock()

	family := n.Family()
	nlriIdx := string(buildNLRIIndex(n))

	if sentFamily, ok := r.sent[family]; ok {
		// Find and remove any route matching this NLRI
		for routeIdx := range sentFamily {
			if matchesNLRI(routeIdx, nlriIdx) {
				delete(sentFamily, routeIdx)
			}
		}
	}
}

// ClearSent queues withdrawals for all sent routes and clears the sent cache.
// Returns the number of routes withdrawn.
func (r *OutgoingRIB) ClearSent() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for family, sentFamily := range r.sent {
		for _, route := range sentFamily {
			// Queue withdrawal for this route
			familyWithdrawals, ok := r.withdrawals[family]
			if !ok {
				familyWithdrawals = make(map[string]nlri.NLRI)
				r.withdrawals[family] = familyWithdrawals
			}
			idx := string(buildNLRIIndex(route.NLRI()))
			familyWithdrawals[idx] = route.NLRI()
			count++
		}
	}

	// Clear the sent cache
	r.sent = make(map[nlri.Family]map[string]*Route)

	return count
}

// FlushSent re-queues all sent routes for re-announcement.
// Returns the number of routes flushed.
func (r *OutgoingRIB) FlushSent() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for family, sentFamily := range r.sent {
		for _, route := range sentFamily {
			// Queue for re-announcement
			familyPending, ok := r.pending[family]
			if !ok {
				familyPending = make(map[string]*Route)
				r.pending[family] = familyPending
			}
			idx := string(route.Index())
			familyPending[idx] = route
			count++
		}
	}

	return count
}

// OutgoingRIBStats holds statistics about the OutgoingRIB.
type OutgoingRIBStats struct {
	PendingAnnouncements int
	PendingWithdrawals   int
	SentRoutes           int
}

// BeginTransaction starts a new transaction.
// Routes queued during a transaction are held until CommitTransaction.
// Returns error if already in a transaction.
func (r *OutgoingRIB) BeginTransaction(label string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.inTransaction {
		return ErrAlreadyInTransaction
	}

	r.inTransaction = true
	r.transactionID = label
	r.txPending = make(map[nlri.Family]map[string]*Route)
	r.txWithdrawals = make(map[nlri.Family]map[string]nlri.NLRI)

	return nil
}

// InTransaction returns true if currently in a transaction.
func (r *OutgoingRIB) InTransaction() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.inTransaction
}

// TransactionID returns the current transaction label, or empty string if not in transaction.
func (r *OutgoingRIB) TransactionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.transactionID
}

// CommitTransaction commits the current transaction.
// Moves all transaction-pending routes to the regular pending queue for sending.
// Returns stats about committed routes.
func (r *OutgoingRIB) CommitTransaction() (CommitStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.inTransaction {
		return CommitStats{}, ErrNoTransaction
	}

	return r.commitLocked(), nil
}

// CommitTransactionWithLabel commits the transaction, verifying the label matches.
func (r *OutgoingRIB) CommitTransactionWithLabel(label string) (CommitStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.inTransaction {
		return CommitStats{}, ErrNoTransaction
	}

	if r.transactionID != label {
		return CommitStats{}, ErrLabelMismatch
	}

	return r.commitLocked(), nil
}

// commitLocked performs the actual commit (caller must hold lock).
func (r *OutgoingRIB) commitLocked() CommitStats {
	var stats CommitStats

	// Count and move announced routes to pending
	for family, routes := range r.txPending {
		if r.pending[family] == nil {
			r.pending[family] = make(map[string]*Route)
		}
		for idx, route := range routes {
			r.pending[family][idx] = route
			stats.RoutesAnnounced++
		}
	}

	// Count and move withdrawals to withdrawals
	for family, withdrawals := range r.txWithdrawals {
		if r.withdrawals[family] == nil {
			r.withdrawals[family] = make(map[string]nlri.NLRI)
		}
		for idx, n := range withdrawals {
			r.withdrawals[family][idx] = n
			stats.RoutesWithdrawn++
		}
	}

	// Clear transaction state
	r.inTransaction = false
	r.transactionID = ""
	r.txPending = nil
	r.txWithdrawals = nil

	return stats
}

// RollbackTransaction discards all routes queued during the transaction.
// Returns stats about discarded routes.
func (r *OutgoingRIB) RollbackTransaction() (CommitStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.inTransaction {
		return CommitStats{}, ErrNoTransaction
	}

	var stats CommitStats

	// Count discarded routes
	for _, routes := range r.txPending {
		stats.RoutesDiscarded += len(routes)
	}
	for _, withdrawals := range r.txWithdrawals {
		stats.RoutesDiscarded += len(withdrawals)
	}

	// Clear transaction state without moving routes
	r.inTransaction = false
	r.transactionID = ""
	r.txPending = nil
	r.txWithdrawals = nil

	return stats, nil
}

// GetTransactionPending returns routes queued in the current transaction for a family.
func (r *OutgoingRIB) GetTransactionPending(family nlri.Family) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.inTransaction {
		return nil
	}

	familyPending, ok := r.txPending[family]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(familyPending))
	for _, route := range familyPending {
		routes = append(routes, route)
	}

	return routes
}
