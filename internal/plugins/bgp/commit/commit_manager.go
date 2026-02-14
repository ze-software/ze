package commit

import (
	"fmt"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/rib"
)

// Transaction errors for named commits.
var (
	ErrCommitExists   = fmt.Errorf("commit already exists")
	ErrCommitNotFound = fmt.Errorf("commit not found")
	ErrEmptyName      = fmt.Errorf("commit name required")
)

// Transaction holds routes for a single named commit.
// Routes queued in a transaction are held until End or Rollback.
type Transaction struct {
	name         string
	peerSelector string
	createdAt    time.Time

	mu          sync.RWMutex
	announces   map[string]*rib.Route // key: nlri index
	withdrawals map[string]nlri.NLRI  // key: nlri index
}

// NewTransaction creates a new named transaction.
func NewTransaction(name, peerSelector string) *Transaction {
	return &Transaction{
		name:         name,
		peerSelector: peerSelector,
		createdAt:    time.Now(),
		announces:    make(map[string]*rib.Route),
		withdrawals:  make(map[string]nlri.NLRI),
	}
}

// Name returns the transaction name.
func (t *Transaction) Name() string {
	return t.name
}

// PeerSelector returns the peer selector captured at creation.
func (t *Transaction) PeerSelector() string {
	return t.peerSelector
}

// QueueAnnounce queues a route for announcement.
// If a withdrawal for this NLRI is pending, it is cancelled.
// If an announcement for this NLRI exists, it is replaced.
func (t *Transaction) QueueAnnounce(route *rib.Route) {
	t.mu.Lock()
	defer t.mu.Unlock()

	idx := t.nlriIndex(route.NLRI())

	// Cancel any pending withdrawal
	delete(t.withdrawals, idx)

	// Add/replace announcement
	t.announces[idx] = route
}

// QueueWithdraw queues a route withdrawal.
// If an announcement for this NLRI is pending, it is cancelled.
func (t *Transaction) QueueWithdraw(n nlri.NLRI) {
	t.mu.Lock()
	defer t.mu.Unlock()

	idx := t.nlriIndex(n)

	// Cancel any pending announcement
	delete(t.announces, idx)

	// Add withdrawal
	t.withdrawals[idx] = n
}

// nlriIndex builds an index key for an NLRI.
func (t *Transaction) nlriIndex(n nlri.NLRI) string {
	family := n.Family()
	// Use WriteTo for consistent API - writes same bytes as Bytes()
	nlriLen := n.Len()

	buf := make([]byte, 3+nlriLen)
	buf[0] = byte(family.AFI >> 8)
	buf[1] = byte(family.AFI)
	buf[2] = byte(family.SAFI)
	n.WriteTo(buf, 3)

	return string(buf)
}

// Count returns the number of pending announcements.
func (t *Transaction) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.announces)
}

// WithdrawalCount returns the number of pending withdrawals.
func (t *Transaction) WithdrawalCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.withdrawals)
}

// TotalCount returns total pending routes (announcements + withdrawals).
func (t *Transaction) TotalCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.announces) + len(t.withdrawals)
}

// Routes returns all pending announcement routes.
func (t *Transaction) Routes() []*rib.Route {
	t.mu.RLock()
	defer t.mu.RUnlock()

	routes := make([]*rib.Route, 0, len(t.announces))
	for _, r := range t.announces {
		routes = append(routes, r)
	}
	return routes
}

// Withdrawals returns all pending withdrawals.
func (t *Transaction) Withdrawals() []nlri.NLRI {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nlris := make([]nlri.NLRI, 0, len(t.withdrawals))
	for _, n := range t.withdrawals {
		nlris = append(nlris, n)
	}
	return nlris
}

// Families returns unique address families with pending routes.
func (t *Transaction) Families() []nlri.Family {
	t.mu.RLock()
	defer t.mu.RUnlock()

	seen := make(map[nlri.Family]bool)
	for _, r := range t.announces {
		seen[r.NLRI().Family()] = true
	}
	for _, n := range t.withdrawals {
		seen[n.Family()] = true
	}

	families := make([]nlri.Family, 0, len(seen))
	for f := range seen {
		families = append(families, f)
	}
	return families
}

// CommitManager tracks multiple concurrent named commits.
// Thread-safe for concurrent access from multiple API handlers.
type CommitManager struct {
	mu      sync.RWMutex
	commits map[string]*Transaction
}

// NewCommitManager creates a new commit manager.
func NewCommitManager() *CommitManager {
	return &CommitManager{
		commits: make(map[string]*Transaction),
	}
}

// Start begins a new named commit.
// Returns error if name is empty or commit already exists.
func (m *CommitManager) Start(name, peerSelector string) error {
	if name == "" {
		return ErrEmptyName
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.commits[name]; exists {
		return fmt.Errorf("%w: %q", ErrCommitExists, name)
	}

	m.commits[name] = NewTransaction(name, peerSelector)
	return nil
}

// Get returns a commit by name without removing it.
// Returns error if commit doesn't exist.
func (m *CommitManager) Get(name string) (*Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tx, exists := m.commits[name]
	if !exists {
		return nil, fmt.Errorf("%w: %q", ErrCommitNotFound, name)
	}
	return tx, nil
}

// End removes and returns a commit for processing.
// The caller is responsible for sending the routes.
func (m *CommitManager) End(name string) (*Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, exists := m.commits[name]
	if !exists {
		return nil, fmt.Errorf("%w: %q", ErrCommitNotFound, name)
	}

	delete(m.commits, name)
	return tx, nil
}

// Rollback removes a commit and returns the count of discarded routes.
func (m *CommitManager) Rollback(name string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, exists := m.commits[name]
	if !exists {
		return 0, fmt.Errorf("%w: %q", ErrCommitNotFound, name)
	}

	discarded := tx.TotalCount()
	delete(m.commits, name)
	return discarded, nil
}

// List returns names of all active commits.
func (m *CommitManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.commits))
	for name := range m.commits {
		names = append(names, name)
	}
	return names
}
