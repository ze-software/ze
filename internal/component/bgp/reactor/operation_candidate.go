// Design: docs/architecture/config/apply-ordering.md -- BGP peer operation handlers
// Related: operation.go -- the verify and apply handlers that read the candidate

package reactor

import (
	"fmt"
	"sync"
)

// operationCandidate is the candidate config one reload transaction resolved
// through the reload function: every configured peer, with its filter chains,
// routes and bindings. A transaction's add-peer and modify-peer operations, in
// verify and again in apply, each need their peer from it, and resolving it is
// a full read and parse of the config, so it is resolved once per transaction
// and shared (operationCandidateSlot). Immutable once stored.
type operationCandidate struct {
	txID  string
	path  string
	peers []*PeerSettings
}

// operationCandidateSlot holds the candidate of at most one transaction. Its
// lifetime is the transaction's: the BGP plugin's config-complete receipt
// (CompleteSourcePolicyTx, sent accepted or rejected, after any rollback)
// drops it, and a resolution for any other transaction replaces it, so a
// transaction that never reaches its receipt holds nothing past the next one.
// Safe for concurrent use: mu also serializes resolution, so concurrent
// operations of one transaction resolve once.
type operationCandidateSlot struct {
	mu      sync.Mutex
	current *operationCandidate
}

// peers answers the candidate peers of txID loaded from path, resolving them
// with load only when the slot holds no candidate of that transaction and
// path. An empty txID (an operation outside a transaction) is resolved every
// time and never stored. A failed load is not stored, so a retry loads again.
// The answer is shared: the caller MUST copy a peer before handing it on.
func (s *operationCandidateSlot) peers(txID, path string, load ReloadFunc) ([]*PeerSettings, error) {
	if txID == "" {
		peers, _, _, err := load(path)
		return peers, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if held := s.current; held != nil && held.txID == txID && held.path == path {
		return held.peers, nil
	}
	peers, _, _, err := load(path)
	if err != nil {
		return nil, err
	}
	s.current = &operationCandidate{txID: txID, path: path, peers: peers}
	return peers, nil
}

// drop forgets txID's candidate, and no other transaction's.
func (s *operationCandidateSlot) drop(txID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil && s.current.txID == txID {
		s.current = nil
	}
}

// operationCandidatePeer answers the candidate peer named name. When the
// candidate is shared by a transaction's operations, it answers a copy, so
// the operation that adopts it (AddPeer keeps the pointer) never shares a
// struct with another operation; a candidate loaded for this operation alone
// is answered as loaded. The copy is shallow, which is enough for the reason
// settingsSnapshot (peer.go) gives: every writer of a PeerSettings slice or
// map replaces the header, never the backing array.
func operationCandidatePeer(peers []*PeerSettings, name string, shared bool) (*PeerSettings, error) {
	for _, peer := range peers {
		if peer == nil {
			continue
		}
		if peer.Name != name {
			continue
		}
		if !shared {
			return peer, nil
		}
		copied := *peer
		return &copied, nil
	}
	return nil, fmt.Errorf("%w: %s", errPeerNotInReloadConfig, name)
}
