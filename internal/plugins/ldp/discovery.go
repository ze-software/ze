// RFC: rfc/short/rfc5036.md -- Section 3.5.2 Hello Hold Time defaults
// Design: docs/architecture/ldp/mpls-ldp.md -- LDP discovery (UDP hello)
// Related: wire.go -- HelloMessage encoding/decoding
//
// RFC 5036 Section 2.4.1: Basic Discovery uses UDP multicast Hello messages
// on 224.0.0.2:646. Each Hello carries a hold time; if no Hello is received
// within the hold time, the adjacency expires.
package ldp

import (
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// RFC 5036 Section 2.4.1: Default hello interval and hold time.
//
// RFC 5036 Section 3.5.2 ("Common Hello Parameters TLV", Hold Time field): a Hold
// Time of 0 on the wire means "use the default", and the default differs by Hello
// kind -- 15 seconds for Link Hellos, 45 seconds for Targeted Hellos. Hold Time 0
// never means "drop the adjacency"; the adjacency is kept and timed by the default.
const (
	DefaultHelloInterval = 5 * time.Second
	DefaultHelloHoldTime = 15 * time.Second

	// DefaultTargetedHelloHoldTime is the default applied to a Targeted Hello that
	// carries Hold Time 0 (RFC 5036 Section 3.5.2).
	DefaultTargetedHelloHoldTime = 45 * time.Second
)

// defaultHoldTime returns the hold time a Hello with Hold Time 0 is worth: the
// per-kind default of RFC 5036 Section 3.5.2.
func defaultHoldTime(targeted bool) time.Duration {
	if targeted {
		return DefaultTargetedHelloHoldTime
	}
	return DefaultHelloHoldTime
}

// Adjacency represents a discovered LDP neighbor via Hello messages.
type Adjacency struct {
	PeerLSRID      [4]byte
	PeerLabelSpace uint16
	TransportAddr  netip.Addr
	HoldTime       time.Duration
	Targeted       bool
	LastSeen       time.Time
	// Interface is the local interface this adjacency was discovered on. It is
	// carried onto the SessionEvent so an IGP LDP-IGP-sync consumer can key its
	// per-interface sync state (RFC 5443 / RFC 6138). Empty for a targeted adjacency.
	Interface string
}

// Expired returns true if the adjacency hold timer has elapsed.
func (a *Adjacency) Expired(now time.Time) bool {
	return now.Sub(a.LastSeen) > a.HoldTime
}

// AdjacencyKey uniquely identifies an adjacency by LSR ID and label space.
func AdjacencyKey(lsrID [4]byte, labelSpace uint16) string {
	var buf []byte
	buf = strconv.AppendUint(buf, uint64(lsrID[0]), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64(lsrID[1]), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64(lsrID[2]), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64(lsrID[3]), 10)
	buf = append(buf, ':')
	buf = strconv.AppendUint(buf, uint64(labelSpace), 10)
	return string(buf)
}

// AdjacencyTable tracks discovered LDP neighbors.
type AdjacencyTable struct {
	mu    sync.RWMutex
	peers map[string]*Adjacency
}

// newAdjacencyTable creates an empty adjacency table.
func newAdjacencyTable() *AdjacencyTable {
	return &AdjacencyTable{
		peers: make(map[string]*Adjacency),
	}
}

// Update processes an incoming Hello and creates or refreshes the adjacency,
// tagging it with the local interface (ifName) the Hello arrived on. Returns the
// adjacency and true if this is a new neighbor.
//
// ifName is written under the table lock alongside the other adjacency fields so
// a concurrent All()/Get() snapshot cannot observe a torn write; the discovering
// interface flows onto the SessionEvent for LDP-IGP sync (RFC 5443 / RFC 6138).
func (t *AdjacencyTable) Update(pduHeader PDUHeader, hello HelloMessage, ifName string) (*Adjacency, bool) {
	key := AdjacencyKey(pduHeader.LSRID, pduHeader.LabelSpace)
	// RFC 5036 Section 3.5.2: Hold Time 0 means "use the default" (15s Link, 45s
	// Targeted). The adjacency is created/refreshed either way -- a 0 never removes it.
	holdTime := time.Duration(hello.HoldTime) * time.Second
	if holdTime == 0 {
		holdTime = defaultHoldTime(hello.Targeted)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	adj, exists := t.peers[key]
	if !exists {
		adj = &Adjacency{
			PeerLSRID:      pduHeader.LSRID,
			PeerLabelSpace: pduHeader.LabelSpace,
			Targeted:       hello.Targeted,
		}
		t.peers[key] = adj
	}
	adj.TransportAddr = hello.TransportAddr
	adj.HoldTime = holdTime
	adj.LastSeen = time.Now()
	adj.Interface = ifName
	return adj, !exists
}

// Remove deletes an adjacency by key.
func (t *AdjacencyTable) Remove(key string) {
	t.mu.Lock()
	delete(t.peers, key)
	t.mu.Unlock()
}

// ExpireSweep removes all expired adjacencies and returns their keys.
func (t *AdjacencyTable) ExpireSweep() []string {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()

	var expired []string
	for key, adj := range t.peers {
		if adj.Expired(now) {
			expired = append(expired, key)
			delete(t.peers, key)
		}
	}
	return expired
}

// All returns a snapshot of all adjacencies.
func (t *AdjacencyTable) All() map[string]Adjacency {
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make(map[string]Adjacency, len(t.peers))
	for k, v := range t.peers {
		out[k] = *v
	}
	return out
}

// Len returns the number of adjacencies.
func (t *AdjacencyTable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.peers)
}

// Get returns a copy of the adjacency for the given key, if it exists.
func (t *AdjacencyTable) Get(key string) (Adjacency, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	adj, ok := t.peers[key]
	if !ok {
		return Adjacency{}, false
	}
	return *adj, true
}
