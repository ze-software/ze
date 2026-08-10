// Design: docs/architecture/pool-architecture.md — RIB wire storage

package rib

import (
	"errors"
	"maps"
	"sync"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
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
	pending map[family.Family]map[string]*Route

	// withdrawals maps family -> NLRI index -> NLRI (withdrawals)
	withdrawals map[family.Family]map[string]nlri.NLRI

	// sent tracks what was last sent (for resend on reconnect)
	sent map[family.Family]map[string]*Route

	// Transaction state
	inTransaction bool
	transactionID string

	// Transaction-scoped pending routes (separate from regular pending)
	txPending     map[family.Family]map[string]*Route
	txWithdrawals map[family.Family]map[string]nlri.NLRI
}

// commitStats holds statistics from a transaction commit.
type commitStats struct {
	RoutesAnnounced int
	RoutesWithdrawn int
	RoutesDiscarded int // Only set on rollback
}

// newOutgoingRIB creates a new Adj-RIB-Out.
func newOutgoingRIB() *OutgoingRIB {
	return &OutgoingRIB{
		pending:     make(map[family.Family]map[string]*Route),
		withdrawals: make(map[family.Family]map[string]nlri.NLRI),
		sent:        make(map[family.Family]map[string]*Route),
	}
}

// QueueAnnounce queues a route for announcement.
// If a withdrawal for this NLRI is pending, it is canceled.
// During a transaction, routes are queued to the transaction pending queue.
func (r *OutgoingRIB) QueueAnnounce(route *Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fam := route.NLRI().Family()
	idx := string(route.Index())

	// Choose target maps based on transaction state
	var targetPending map[family.Family]map[string]*Route
	var targetWithdrawals map[family.Family]map[string]nlri.NLRI

	if r.inTransaction {
		targetPending = r.txPending
		targetWithdrawals = r.txWithdrawals
	} else {
		targetPending = r.pending
		targetWithdrawals = r.withdrawals
	}

	// Get or create family's pending map
	familyPending, ok := targetPending[fam]
	if !ok {
		familyPending = make(map[string]*Route)
		targetPending[fam] = familyPending
	}

	// Cancel any pending withdrawal for this NLRI
	if familyWithdrawals, ok := targetWithdrawals[fam]; ok {
		delete(familyWithdrawals, idx)
	}

	familyPending[idx] = route
}

// QueueWithdraw queues a route withdrawal.
// If an announcement for this NLRI is pending, it is canceled.
// During a transaction, withdrawals are queued to the transaction withdrawal queue.
func (r *OutgoingRIB) QueueWithdraw(n nlri.NLRI) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fam := n.Family()

	// Build an index from the NLRI (without AS-PATH since withdrawals don't have attributes)
	// For withdrawal matching, we use just Family + NLRI bytes
	idx := string(buildNLRIIndex(n))

	// Choose target maps based on transaction state
	var targetPending map[family.Family]map[string]*Route
	var targetWithdrawals map[family.Family]map[string]nlri.NLRI

	if r.inTransaction {
		targetPending = r.txPending
		targetWithdrawals = r.txWithdrawals
	} else {
		targetPending = r.pending
		targetWithdrawals = r.withdrawals
	}

	// Cancel any pending announcement for this NLRI
	if familyPending, ok := targetPending[fam]; ok {
		// Fast path: exact key match (route has no AS-PATH hash suffix).
		delete(familyPending, idx)
		// Slow path: scan for routes whose key has an AS-PATH hash suffix.
		for pendingIdx := range familyPending {
			if matchesNLRI(pendingIdx, idx) {
				delete(familyPending, pendingIdx)
			}
		}
	}

	// Get or create family's withdrawal map
	familyWithdrawals, ok := targetWithdrawals[fam]
	if !ok {
		familyWithdrawals = make(map[string]nlri.NLRI)
		targetWithdrawals[fam] = familyWithdrawals
	}

	familyWithdrawals[idx] = n
}

// buildNLRIIndex builds an index for an NLRI (without AS-PATH).
func buildNLRIIndex(n nlri.NLRI) []byte {
	fam := n.Family()
	// Use WriteTo for consistent API - writes same bytes as Bytes()
	nlriLen := n.Len()

	buf := make([]byte, 3+nlriLen)
	buf[0] = byte(fam.AFI >> 8)
	buf[1] = byte(fam.AFI)
	buf[2] = byte(fam.SAFI)
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

// getPending returns pending routes for a family without clearing them.
func (r *OutgoingRIB) getPending(fam family.Family) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	familyPending, ok := r.pending[fam]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(familyPending))
	for _, route := range familyPending {
		routes = append(routes, route)
	}

	return routes
}

// flushPending returns and clears pending routes for a family.
func (r *OutgoingRIB) flushPending(fam family.Family) []*Route {
	r.mu.Lock()
	defer r.mu.Unlock()

	familyPending, ok := r.pending[fam]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(familyPending))
	for _, route := range familyPending {
		routes = append(routes, route)
	}

	// Clear pending
	delete(r.pending, fam)

	// Add to sent cache
	if r.sent[fam] == nil {
		r.sent[fam] = make(map[string]*Route)
	}
	maps.Copy(r.sent[fam], familyPending)

	return routes
}

// getWithdrawals returns pending withdrawals for a family.
func (r *OutgoingRIB) getWithdrawals(fam family.Family) []nlri.NLRI {
	r.mu.RLock()
	defer r.mu.RUnlock()

	familyWithdrawals, ok := r.withdrawals[fam]
	if !ok {
		return nil
	}

	nlris := make([]nlri.NLRI, 0, len(familyWithdrawals))
	for _, n := range familyWithdrawals {
		nlris = append(nlris, n)
	}

	return nlris
}

// flushWithdrawals returns and clears pending withdrawals for a family.
func (r *OutgoingRIB) flushWithdrawals(fam family.Family) []nlri.NLRI {
	r.mu.Lock()
	defer r.mu.Unlock()

	familyWithdrawals, ok := r.withdrawals[fam]
	if !ok {
		return nil
	}

	nlris := make([]nlri.NLRI, 0, len(familyWithdrawals))
	for _, n := range familyWithdrawals {
		nlris = append(nlris, n)
	}

	// Clear withdrawals
	delete(r.withdrawals, fam)

	// Remove from sent cache. Walk sentFamily once (large) and check each
	// entry against familyWithdrawals (small) to avoid re-traversing the
	// large map W times.
	if sentFamily, ok := r.sent[fam]; ok {
		for sentIdx := range sentFamily {
			for idx := range familyWithdrawals {
				if matchesNLRI(sentIdx, idx) {
					delete(sentFamily, sentIdx)
					break
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

// getSentRoutes returns all previously sent routes for re-announcement.
// Used when a session re-establishes to replay the RIB to the peer.
func (r *OutgoingRIB) getSentRoutes() []*Route {
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

	fam := route.NLRI().Family()
	idx := string(route.Index())

	if r.sent[fam] == nil {
		r.sent[fam] = make(map[string]*Route)
	}
	r.sent[fam][idx] = route
}

// removeFromSent removes a route from the sent cache by NLRI.
// Used when a withdrawal is queued to prevent re-announcement on reconnect.
func (r *OutgoingRIB) removeFromSent(n nlri.NLRI) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fam := n.Family()
	nlriIdx := string(buildNLRIIndex(n))

	if sentFamily, ok := r.sent[fam]; ok {
		// Fast path: exact key match (route has no AS-PATH hash suffix).
		delete(sentFamily, nlriIdx)
		// Slow path: scan for routes whose key has an AS-PATH hash suffix.
		for routeIdx := range sentFamily {
			if matchesNLRI(routeIdx, nlriIdx) {
				delete(sentFamily, routeIdx)
			}
		}
	}
}

// clearSent queues withdrawals for all sent routes and clears the sent cache.
// Returns the number of routes withdrawn.
func (r *OutgoingRIB) clearSent() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for fam, sentFamily := range r.sent {
		for _, route := range sentFamily {
			// Queue withdrawal for this route
			familyWithdrawals, ok := r.withdrawals[fam]
			if !ok {
				familyWithdrawals = make(map[string]nlri.NLRI)
				r.withdrawals[fam] = familyWithdrawals
			}
			idx := string(buildNLRIIndex(route.NLRI()))
			familyWithdrawals[idx] = route.NLRI()
			count++
		}
	}

	// Clear the sent cache
	clear(r.sent)

	return count
}

// flushSent re-queues all sent routes for re-announcement.
// Returns the number of routes flushed.
func (r *OutgoingRIB) flushSent() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for fam, sentFamily := range r.sent {
		for _, route := range sentFamily {
			// Queue for re-announcement
			familyPending, ok := r.pending[fam]
			if !ok {
				familyPending = make(map[string]*Route)
				r.pending[fam] = familyPending
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
	if r.txPending != nil {
		clear(r.txPending)
	} else {
		r.txPending = make(map[family.Family]map[string]*Route)
	}
	if r.txWithdrawals != nil {
		clear(r.txWithdrawals)
	} else {
		r.txWithdrawals = make(map[family.Family]map[string]nlri.NLRI)
	}

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
func (r *OutgoingRIB) CommitTransaction() (commitStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.inTransaction {
		return commitStats{}, ErrNoTransaction
	}

	return r.commitLocked(), nil
}

// commitTransactionWithLabel commits the transaction, verifying the label matches.
// It is CommitTransaction with a label check, so it returns the same pair.
//
//nolint:unparam // the transaction API is one shape: BeginTransaction, CommitTransaction, RollbackTransaction and this label-checked variant all report commitStats. unparam judges this function alone and cannot see the exported siblings that make the stats part of the contract
func (r *OutgoingRIB) commitTransactionWithLabel(label string) (commitStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.inTransaction {
		return commitStats{}, ErrNoTransaction
	}

	if r.transactionID != label {
		return commitStats{}, ErrLabelMismatch
	}

	return r.commitLocked(), nil
}

// commitLocked performs the actual commit (caller must hold lock).
func (r *OutgoingRIB) commitLocked() commitStats {
	var stats commitStats

	// Count and move announced routes to pending
	for fam, routes := range r.txPending {
		if r.pending[fam] == nil {
			r.pending[fam] = make(map[string]*Route)
		}
		for idx, route := range routes {
			r.pending[fam][idx] = route
			stats.RoutesAnnounced++
		}
	}

	// Count and move withdrawals to withdrawals
	for fam, withdrawals := range r.txWithdrawals {
		if r.withdrawals[fam] == nil {
			r.withdrawals[fam] = make(map[string]nlri.NLRI)
		}
		for idx, n := range withdrawals {
			r.withdrawals[fam][idx] = n
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
func (r *OutgoingRIB) RollbackTransaction() (commitStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.inTransaction {
		return commitStats{}, ErrNoTransaction
	}

	var stats commitStats

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

// getTransactionPending returns routes queued in the current transaction for a family.
func (r *OutgoingRIB) getTransactionPending(fam family.Family) []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.inTransaction {
		return nil
	}

	familyPending, ok := r.txPending[fam]
	if !ok {
		return nil
	}

	routes := make([]*Route, 0, len(familyPending))
	for _, route := range familyPending {
		routes = append(routes, route)
	}

	return routes
}
