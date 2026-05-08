// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 service lifecycle for PPP sessions
// Related: ra.go -- RA message building
// Related: dhcpv6.go -- DHCPv6 codec
// Related: session_run.go -- afterLCPOpen starts this, teardownNCPResources stops it

package ppp

import (
	"net/netip"
	"sync"
)

// IPv6ServiceConfig holds the parameters needed to start RA and DHCPv6
// services on a PPP session's interface.
type IPv6ServiceConfig struct {
	Ifname           string
	TunnelID         uint16
	SessionID        uint16
	LocalInterfaceID [8]byte
	PeerInterfaceID  [8]byte
	Backend          IfaceBackend
}

// IPv6Service manages the RA sender and DHCPv6-PD server for a single
// PPP session. Created by startIPv6Service (platform-specific); stopped
// by Stop(). The session goroutine owns the lifecycle.
type IPv6Service struct {
	cfg  IPv6ServiceConfig
	stop func()

	mu              sync.Mutex
	delegatedPrefix netip.Prefix
	routeInstalled  bool
}

// Stop terminates the RA and DHCPv6 goroutines, removes any installed
// route, and releases the delegated prefix.
func (s *IPv6Service) Stop() {
	if s == nil {
		return
	}
	if s.stop != nil {
		s.stop()
	}
	s.removeRouteIfInstalled()
}

func (s *IPv6Service) removeRouteIfInstalled() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.routeInstalled || !s.delegatedPrefix.IsValid() {
		return
	}
	peerLL := peerLinkLocal(s.cfg.PeerInterfaceID)
	_ = s.cfg.Backend.RemoveRoute(s.cfg.Ifname, s.delegatedPrefix.String(), peerLL.String(), 0)
	s.routeInstalled = false
}

// installRoute adds a kernel route for the delegated prefix pointing
// at the subscriber's link-local address.
func (s *IPv6Service) installRoute(prefix netip.Prefix) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	peerLL := peerLinkLocal(s.cfg.PeerInterfaceID)
	if err := s.cfg.Backend.AddRoute(s.cfg.Ifname, prefix.String(), peerLL.String(), 0); err != nil {
		return err
	}
	s.delegatedPrefix = prefix
	s.routeInstalled = true
	return nil
}

// HandleDHCPv6 processes a parsed DHCPv6 message and returns response
// bytes (or nil if no response needed). Called by the DHCPv6 listener
// goroutine on Linux, or directly by tests.
func (s *IPv6Service) HandleDHCPv6(msg *DHCPv6Message, serverID DHCPv6DUID, prefixHandler func() (netip.Prefix, bool)) ([]byte, error) {
	switch msg.Type {
	case DHCPv6Solicit:
		return s.handleSolicit(msg, serverID, prefixHandler)
	case DHCPv6Request:
		return s.handleRequest(msg, serverID)
	case DHCPv6Renew:
		return s.handleRenew(msg, serverID)
	case DHCPv6Release:
		return s.handleRelease(msg, serverID)
	}
	return nil, nil
}

func (s *IPv6Service) handleSolicit(msg *DHCPv6Message, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool)) ([]byte, error) {
	if msg.ClientID == nil || msg.IAPD == nil {
		return nil, nil
	}

	prefix, ok := allocPrefix()
	if !ok {
		var buf [512]byte
		n := BuildDHCPv6StatusReply(buf[:], DHCPv6StatusReplyConfig{
			TransactionID: msg.TransactionID,
			ServerID:      serverID,
			ClientID:      msg.ClientID,
			StatusCode:    D6StatusNoPrefixAvail,
			StatusMessage: "pool exhausted",
		})
		return buf[:n], nil
	}

	s.mu.Lock()
	s.delegatedPrefix = prefix
	s.mu.Unlock()

	var buf [512]byte
	n := BuildDHCPv6Reply(buf[:], DHCPv6ReplyConfig{
		Type:          DHCPv6Advertise,
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  604800,
		ValidLifetime: 2592000,
		T1:            302400,
		T2:            483840,
	})
	return buf[:n], nil
}

func (s *IPv6Service) handleRequest(msg *DHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
	if msg.ClientID == nil || msg.IAPD == nil {
		return nil, nil
	}

	s.mu.Lock()
	prefix := s.delegatedPrefix
	s.mu.Unlock()

	if !prefix.IsValid() {
		return nil, nil
	}

	if err := s.installRoute(prefix); err != nil {
		return nil, err
	}

	var buf [512]byte
	n := BuildDHCPv6Reply(buf[:], DHCPv6ReplyConfig{
		Type:          DHCPv6Reply,
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  604800,
		ValidLifetime: 2592000,
		T1:            302400,
		T2:            483840,
	})
	return buf[:n], nil
}

func (s *IPv6Service) handleRenew(msg *DHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
	if msg.ClientID == nil || msg.IAPD == nil {
		return nil, nil
	}

	s.mu.Lock()
	prefix := s.delegatedPrefix
	s.mu.Unlock()

	if !prefix.IsValid() {
		return nil, nil
	}

	var buf [512]byte
	n := BuildDHCPv6Reply(buf[:], DHCPv6ReplyConfig{
		Type:          DHCPv6Reply,
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  604800,
		ValidLifetime: 2592000,
		T1:            302400,
		T2:            483840,
	})
	return buf[:n], nil
}

func (s *IPv6Service) handleRelease(msg *DHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
	if msg.ClientID == nil {
		return nil, nil
	}
	s.removeRouteIfInstalled()

	var buf [512]byte
	n := BuildDHCPv6StatusReply(buf[:], DHCPv6StatusReplyConfig{
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		StatusCode:    D6StatusSuccess,
		StatusMessage: "released",
	})
	return buf[:n], nil
}

// peerLinkLocal constructs the fe80::<interface-id> link-local address
// from the 8-byte IPv6CP Interface-Identifier.
func peerLinkLocal(id [8]byte) netip.Addr {
	var addr [16]byte
	addr[0] = 0xfe
	addr[1] = 0x80
	copy(addr[8:], id[:])
	return netip.AddrFrom16(addr)
}
