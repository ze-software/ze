// Design: docs/architecture/core-design.md -- BMP monitored peer state
//
// Related: bmp.go -- plugin lifecycle, message dispatch

package bmp

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
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
//
// Families are the address families the peer's Peer Up OPEN advertised, which
// RFC 9069 Section 6.1.1 requires a receiver to read: "A BMP receiver MUST
// process these capabilities to know which peer belongs to which address
// family." Empty when the OPEN advertised none, never defaulted.
type monitoredPeer struct {
	Router    string   `json:"router"`
	PeerAS    uint32   `json:"peer-as"`
	PeerBGPID string   `json:"peer-bgp-id"`
	IsIPv6    bool     `json:"ipv6"`
	IsUp      bool     `json:"up"`
	Families  []string `json:"families,omitempty"`
	Reason    uint8    `json:"down-reason,omitempty"`
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

// routerCount reports how many monitored router sessions are registered.
func (s *bmpState) routerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.routers)
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

// peerUp records a peer as up, with the address families its Peer Up OPEN
// advertised (RFC 9069 Section 6.1.1).
func (s *bmpState) peerUp(remote string, ph PeerHeader, families []family.Family) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := peerKey{router: remote, distinguisher: ph.Distinguisher, address: ph.Address}
	var b textbuf.Buffer
	names := make([]string, 0, len(families))
	for _, fam := range families {
		names = append(names, fam.String())
	}
	s.peers[key] = &monitoredPeer{
		Router:    remote,
		PeerAS:    ph.PeerAS,
		PeerBGPID: b.Reset().Uint(uint64(ph.PeerBGPID >> 24)).Byte('.').Uint(uint64((ph.PeerBGPID >> 16) & 0xFF)).Byte('.').Uint(uint64((ph.PeerBGPID >> 8) & 0xFF)).Byte('.').Uint(uint64(ph.PeerBGPID & 0xFF)).String(),
		IsIPv6:    ph.IsIPv6(),
		IsUp:      true,
		Families:  names,
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

func (s *bmpState) sessionsCommand() (string, any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]monitoredRouter, 0, len(s.routers))
	for _, r := range s.routers {
		sessions = append(sessions, *r)
	}
	return statusDone, map[string]any{"sessions": sessions}, nil
}

func (s *bmpState) peersCommand() (string, any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]monitoredPeer, 0, len(s.peers))
	for _, p := range s.peers {
		peers = append(peers, *p)
	}
	return statusDone, map[string]any{"peers": peers}, nil
}

func (s *bmpState) collectorsCommand(senders []*senderSession) (string, any, error) { //nolint:unparam // matches handler pattern
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
	return statusDone, map[string]any{"collectors": collectors}, nil
}
