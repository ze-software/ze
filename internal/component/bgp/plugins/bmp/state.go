// Design: docs/architecture/core-design.md -- BMP monitored peer state
//
// Related: bmp.go -- plugin lifecycle, message dispatch

package bmp

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	statusDone  = "done"
	statusError = "error"
)

// monitoredRouter tracks a remote BMP router that has connected to the receiver.
type monitoredRouter struct {
	Remote   string    `json:"remote"`
	SysName  string    `json:"sys-name"`
	SysDescr string    `json:"sys-descr"`
	Since    time.Time `json:"since"`
}

// monitoredPeer tracks a BGP peer reported by a BMP router.
type monitoredPeer struct {
	Router    string `json:"router"`
	PeerAS    uint32 `json:"peer-as"`
	PeerBGPID string `json:"peer-bgp-id"`
	IsIPv6    bool   `json:"ipv6"`
	IsUp      bool   `json:"up"`
	Reason    uint8  `json:"down-reason,omitempty"`
}

// peerKey uniquely identifies a monitored peer.
type peerKey struct {
	router        string
	distinguisher uint64
	address       [16]byte
}

// collectorStatus tracks a sender collector connection.
type collectorStatus struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Port      uint16 `json:"port"`
	Connected bool   `json:"connected"`
}

// bmpState holds all queryable state for the BMP plugin.
type bmpState struct {
	mu      sync.RWMutex
	routers map[string]*monitoredRouter // remote addr -> router
	peers   map[peerKey]*monitoredPeer  // (router, dist, addr) -> peer
}

func newBMPState() *bmpState {
	return &bmpState{
		routers: make(map[string]*monitoredRouter),
		peers:   make(map[peerKey]*monitoredPeer),
	}
}

// addRouter registers a new BMP router session.
func (s *bmpState) addRouter(remote string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routers[remote] = &monitoredRouter{
		Remote: remote,
		Since:  time.Now(),
	}
}

// setRouterInfo updates sysName/sysDescr from an Initiation message.
func (s *bmpState) setRouterInfo(remote, sysName, sysDescr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.routers[remote]
	if !ok {
		return
	}
	if sysName != "" {
		r.SysName = sysName
	}
	if sysDescr != "" {
		r.SysDescr = sysDescr
	}
}

// removeRouter removes a router and all its peers.
func (s *bmpState) removeRouter(remote string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.routers, remote)
	for k := range s.peers {
		if k.router == remote {
			delete(s.peers, k)
		}
	}
}

// peerUp records a peer as up.
func (s *bmpState) peerUp(remote string, ph PeerHeader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := peerKey{router: remote, distinguisher: ph.Distinguisher, address: ph.Address}
	s.peers[key] = &monitoredPeer{
		Router:    remote,
		PeerAS:    ph.PeerAS,
		PeerBGPID: fmt.Sprintf("%d.%d.%d.%d", ph.PeerBGPID>>24, (ph.PeerBGPID>>16)&0xFF, (ph.PeerBGPID>>8)&0xFF, ph.PeerBGPID&0xFF),
		IsIPv6:    ph.IsIPv6(),
		IsUp:      true,
	}
}

// peerDown marks a peer as down.
func (s *bmpState) peerDown(remote string, ph PeerHeader, reason uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := peerKey{router: remote, distinguisher: ph.Distinguisher, address: ph.Address}
	if p, ok := s.peers[key]; ok {
		p.IsUp = false
		p.Reason = reason
	}
}

// --- Command handlers ---

func (s *bmpState) sessionsCommand() (string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]monitoredRouter, 0, len(s.routers))
	for _, r := range s.routers {
		sessions = append(sessions, *r)
	}
	data, err := json.Marshal(map[string]any{"sessions": sessions})
	if err != nil {
		return statusError, "", fmt.Errorf("marshal sessions: %w", err)
	}
	return statusDone, string(data), nil
}

func (s *bmpState) peersCommand() (string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]monitoredPeer, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, *p)
	}
	data, err := json.Marshal(map[string]any{"peers": peers})
	if err != nil {
		return statusError, "", fmt.Errorf("marshal peers: %w", err)
	}
	return statusDone, string(data), nil
}

func (s *bmpState) collectorsCommand(senders []*senderSession) (string, string, error) {
	collectors := make([]collectorStatus, 0, len(senders))
	for _, ss := range senders {
		ss.connMu.Lock()
		connected := ss.conn != nil
		ss.connMu.Unlock()
		collectors = append(collectors, collectorStatus{
			Name:      ss.name,
			Address:   ss.address,
			Port:      ss.port,
			Connected: connected,
		})
	}
	data, err := json.Marshal(map[string]any{"collectors": collectors})
	if err != nil {
		return statusError, "", fmt.Errorf("marshal collectors: %w", err)
	}
	return statusDone, string(data), nil
}
