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
	txID   string
	path   string
	loaded loadedConfig
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

// load answers the whole pure candidate population, including dynamic groups.
// Parsing is shared within a transaction; runtime ownership is resolved later
// on a copied graph, so verify cannot cache authority for the apply phase.
func (s *operationCandidateSlot) load(txID, path string, load ReloadFunc) (loadedConfig, error) {
	if txID == "" {
		peers, globals, groups, err := load(path)
		return loadedConfig{peers: peers, globals: globals, groups: dynamicGroupsReadAs(groups)}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if held := s.current; held != nil && held.txID == txID && held.path == path {
		return held.loaded, nil
	}
	peers, globals, groups, err := load(path)
	if err != nil {
		return loadedConfig{}, err
	}
	loaded := loadedConfig{peers: peers, globals: globals, groups: dynamicGroupsReadAs(groups)}
	s.current = &operationCandidate{txID: txID, path: path, loaded: loaded}
	return loaded, nil
}

// drop forgets txID's candidate, and no other transaction's.
func (s *operationCandidateSlot) drop(txID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil && s.current.txID == txID {
		s.current = nil
	}
}

// copyOperationCandidate isolates the structs ownership resolution writes.
// Slices, maps and compiled policies remain immutable borrowed values, as in
// settingsSnapshot. Dynamic group runtime atomics are not copied.
func copyOperationCandidate(loaded loadedConfig) loadedConfig {
	copySettings := func(settings *PeerSettings) *PeerSettings {
		if settings == nil {
			return nil
		}
		copy := *settings
		return &copy
	}
	copy := loaded
	copy.peers = make([]*PeerSettings, len(loaded.peers))
	for i, peer := range loaded.peers {
		copy.peers[i] = copySettings(peer)
	}
	copy.groups.groups = make([]*DynamicGroupConfig, len(loaded.groups.groups))
	for i, group := range loaded.groups.groups {
		copy.groups.groups[i] = &DynamicGroupConfig{GroupName: group.GroupName,
			Ranges: group.Ranges, MaxPeers: group.MaxPeers, Template: group.Template,
			Settings: copySettings(group.Settings)}
	}
	return copy
}

// operationCandidatePeer answers one peer from an already isolated population.
func operationCandidatePeer(peers []*PeerSettings, name string) (*PeerSettings, error) {
	for _, peer := range peers {
		if peer != nil && peer.Name == name {
			return peer, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", errPeerNotInReloadConfig, name)
}
