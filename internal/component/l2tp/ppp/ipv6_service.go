// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 service lifecycle for PPP sessions
// Related: ra.go -- RA message building
// Related: dhcpv6.go -- DHCPv6 codec
// Related: dhcpv6_linux.go -- DHCPv6 UDP listener
// Related: session_run.go -- afterLCPOpen starts this, teardownNCPResources stops it

package ppp

import (
	"bytes"
	"net/netip"
	"sync"
)

// DHCPv6Lifetimes holds the lease timing parameters for DHCPv6-PD replies.
// Zero values are replaced with defaults.
type DHCPv6Lifetimes struct {
	T1            uint32
	T2            uint32
	PrefLifetime  uint32
	ValidLifetime uint32
}

func dhcpv6LifetimeDefaults(l DHCPv6Lifetimes) DHCPv6Lifetimes {
	if l.T1 == 0 {
		l.T1 = 302400
	}
	if l.T2 == 0 {
		l.T2 = 483840
	}
	if l.PrefLifetime == 0 {
		l.PrefLifetime = 604800
	}
	if l.ValidLifetime == 0 {
		l.ValidLifetime = 2592000
	}
	return l
}

// IPv6ServiceConfig holds the parameters needed to start RA and DHCPv6
// services on a PPP session's interface.
type IPv6ServiceConfig struct {
	Ifname           string
	TunnelID         uint16
	SessionID        uint16
	LocalInterfaceID [8]byte
	PeerInterfaceID  [8]byte
	Backend          IfaceBackend
	Lifetimes        DHCPv6Lifetimes
	ReleasePrefix    func(netip.Prefix)
}

// IPv6Service manages the RA sender and DHCPv6-PD server for a single
// PPP session. Created by startIPv6Service (platform-specific); stopped
// by Stop(). The session goroutine owns the lifecycle.
type IPv6Service struct {
	cfg       IPv6ServiceConfig
	lifetimes DHCPv6Lifetimes
	stop      func()

	mu              sync.Mutex
	delegatedPrefix netip.Prefix
	routeInstalled  bool
}

func NewIPv6Service(cfg IPv6ServiceConfig) *IPv6Service {
	return &IPv6Service{
		cfg:       cfg,
		lifetimes: dhcpv6LifetimeDefaults(cfg.Lifetimes),
	}
}

// Stop terminates the RA and DHCPv6 goroutines, removes any installed
// route, and releases the delegated prefix back to the pool.
func (s *IPv6Service) Stop() {
	if s == nil {
		return
	}
	if s.stop != nil {
		s.stop()
	}
	s.cleanupPrefix()
}

func (s *IPv6Service) cleanupPrefix() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.routeInstalled && s.delegatedPrefix.IsValid() {
		peerLL := peerLinkLocal(s.cfg.PeerInterfaceID)
		_ = s.cfg.Backend.RemoveRoute(s.cfg.Ifname, s.delegatedPrefix.String(), peerLL.String(), 0)
		s.routeInstalled = false
	}
	if s.delegatedPrefix.IsValid() && s.cfg.ReleasePrefix != nil {
		s.cfg.ReleasePrefix(s.delegatedPrefix)
		s.delegatedPrefix = netip.Prefix{}
	}
}

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
// bytes (or nil if no response needed).
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

	if allocPrefix == nil {
		return s.noPrefixAvailReply(msg, serverID, "no pool configured")
	}

	prefix, ok := allocPrefix()
	if !ok {
		return s.noPrefixAvailReply(msg, serverID, "pool exhausted")
	}

	s.mu.Lock()
	s.delegatedPrefix = prefix
	s.mu.Unlock()

	var buf [512]byte
	n, err := CheckedBuildDHCPv6Reply(buf[:], DHCPv6ReplyConfig{
		Type:          DHCPv6Advertise,
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  s.lifetimes.PrefLifetime,
		ValidLifetime: s.lifetimes.ValidLifetime,
		T1:            s.lifetimes.T1,
		T2:            s.lifetimes.T2,
	})
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *IPv6Service) handleRequest(msg *DHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
	if msg.ClientID == nil || msg.IAPD == nil {
		return nil, nil
	}
	if !duidEqual(msg.ServerID, &serverID) {
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
	n, err := CheckedBuildDHCPv6Reply(buf[:], DHCPv6ReplyConfig{
		Type:          DHCPv6Reply,
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  s.lifetimes.PrefLifetime,
		ValidLifetime: s.lifetimes.ValidLifetime,
		T1:            s.lifetimes.T1,
		T2:            s.lifetimes.T2,
	})
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *IPv6Service) handleRenew(msg *DHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
	if msg.ClientID == nil || msg.IAPD == nil {
		return nil, nil
	}
	if !duidEqual(msg.ServerID, &serverID) {
		return nil, nil
	}

	s.mu.Lock()
	prefix := s.delegatedPrefix
	s.mu.Unlock()

	if !prefix.IsValid() {
		return nil, nil
	}

	var buf [512]byte
	n, err := CheckedBuildDHCPv6Reply(buf[:], DHCPv6ReplyConfig{
		Type:          DHCPv6Reply,
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		IAID:          msg.IAPD.IAID,
		Prefix:        prefix,
		PrefLifetime:  s.lifetimes.PrefLifetime,
		ValidLifetime: s.lifetimes.ValidLifetime,
		T1:            s.lifetimes.T1,
		T2:            s.lifetimes.T2,
	})
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *IPv6Service) handleRelease(msg *DHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
	if msg.ClientID == nil {
		return nil, nil
	}
	if !duidEqual(msg.ServerID, &serverID) {
		return nil, nil
	}
	s.cleanupPrefix()

	var buf [512]byte
	n, err := CheckedBuildDHCPv6StatusReply(buf[:], DHCPv6StatusReplyConfig{
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		StatusCode:    D6StatusSuccess,
		StatusMessage: "released",
	})
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *IPv6Service) noPrefixAvailReply(msg *DHCPv6Message, serverID DHCPv6DUID, reason string) ([]byte, error) {
	var buf [512]byte
	n, err := CheckedBuildDHCPv6StatusReply(buf[:], DHCPv6StatusReplyConfig{
		TransactionID: msg.TransactionID,
		ServerID:      serverID,
		ClientID:      msg.ClientID,
		StatusCode:    D6StatusNoPrefixAvail,
		StatusMessage: reason,
	})
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func peerLinkLocal(id [8]byte) netip.Addr {
	var addr [16]byte
	addr[0] = 0xfe
	addr[1] = 0x80
	copy(addr[8:], id[:])
	return netip.AddrFrom16(addr)
}

func duidEqual(a, b *DHCPv6DUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Type == b.Type &&
		a.HWType == b.HWType &&
		a.Time == b.Time &&
		a.EnterpriseNum == b.EnterpriseNum &&
		bytes.Equal(a.ID, b.ID)
}
