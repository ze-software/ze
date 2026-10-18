// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 service lifecycle for PPP sessions
// RFC: rfc/short/rfc5072.md -- RFC 5072 Section 5 (link-local address of a PPP interface)
// Related: ra.go -- RA message building
// Related: dhcpv6.go -- DHCPv6 codec
// Related: dhcpv6_linux.go -- DHCPv6 UDP listener
// Related: session_run.go -- afterLCPOpen starts this, teardownNCPResources stops it

package ppp

import (
	"bytes"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/rtproto"
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

// iPv6ServiceConfig holds the parameters needed to start RA and DHCPv6
// services on a PPP session's interface.
type iPv6ServiceConfig struct {
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
	cfg       iPv6ServiceConfig
	lifetimes DHCPv6Lifetimes
	stop      func()

	mu                 sync.Mutex
	delegatedPrefix    netip.Prefix
	routeInstalled     bool
	linkLocalInstalled bool
}

func newIPv6Service(cfg iPv6ServiceConfig) *IPv6Service {
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
	s.removeLinkLocal()
}

// linkLocalPrefixBits is the length of the link-local prefix FE80::/64 that
// RFC 5072 Section 5 forms a PPP link-local address in.
const linkLocalPrefixBits = 64

// installLinkLocal puts the negotiated link-local pair on pppN. Linux
// autoconfigures a link-local address only on the device types
// addrconf_dev_config lists (net/ipv6/addrconf.c), and ARPHRD_PPP is not one,
// so without this pppN has no link-local address. Every Router Advertisement
// sent there would then break RFC 4861 Section 4.2, and DHCPv6 replies would
// leave from no link-local source either.
//
// RFC 5072 Section 5: "The negotiated interface identifier is used by the
// local end of the PPP link to autoconfigure an IPv6 link-local unicast
// address for the PPP interface."
// RFC 4861 Section 4.2: the Router Advertisement Source Address "MUST be the
// link-local address assigned to the interface from which this message is
// sent." This installs that address.
func (s *IPv6Service) installLinkLocal() error {
	local := netip.PrefixFrom(linkLocalFromInterfaceID(s.cfg.LocalInterfaceID), linkLocalPrefixBits)
	peer := netip.PrefixFrom(linkLocalFromInterfaceID(s.cfg.PeerInterfaceID), linkLocalPrefixBits)
	if err := s.cfg.Backend.AddAddressP2P(s.cfg.Ifname, local.String(), peer.String()); err != nil {
		return err
	}
	s.mu.Lock()
	s.linkLocalInstalled = true
	s.mu.Unlock()
	return nil
}

// removeLinkLocal withdraws the pair installLinkLocal put on pppN, so an
// IPv6CP renegotiation inside one session installs its new pair cleanly.
func (s *IPv6Service) removeLinkLocal() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.linkLocalInstalled {
		return
	}
	local := netip.PrefixFrom(linkLocalFromInterfaceID(s.cfg.LocalInterfaceID), linkLocalPrefixBits)
	// The device usually goes with the session, taking the address with it,
	// so a failed removal here has nothing left to undo.
	_ = s.cfg.Backend.RemoveAddress(s.cfg.Ifname, local.String()) //nolint:errcheck // pppN is normally gone with the session
	s.linkLocalInstalled = false
}

func (s *IPv6Service) cleanupPrefix() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.routeInstalled && s.delegatedPrefix.IsValid() {
		peerLL := linkLocalFromInterfaceID(s.cfg.PeerInterfaceID)
		_ = s.cfg.Backend.RemoveRoute(s.cfg.Ifname, s.delegatedPrefix.String(), peerLL.String(), 0, rtproto.Iface)
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
	peerLL := linkLocalFromInterfaceID(s.cfg.PeerInterfaceID)
	if err := s.cfg.Backend.AddRoute(s.cfg.Ifname, prefix.String(), peerLL.String(), 0, rtproto.Iface); err != nil {
		return err
	}
	s.delegatedPrefix = prefix
	s.routeInstalled = true
	return nil
}

// handleDHCPv6 processes a parsed DHCPv6 message and returns response
// bytes (or nil if no response needed).
func (s *IPv6Service) handleDHCPv6(msg *dHCPv6Message, serverID DHCPv6DUID, prefixHandler func() (netip.Prefix, bool)) ([]byte, error) {
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

func (s *IPv6Service) handleSolicit(msg *dHCPv6Message, serverID DHCPv6DUID, allocPrefix func() (netip.Prefix, bool)) ([]byte, error) {
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
	n, err := checkedBuildDHCPv6Reply(buf[:], dHCPv6ReplyConfig{
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

func (s *IPv6Service) handleRequest(msg *dHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
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
	n, err := checkedBuildDHCPv6Reply(buf[:], dHCPv6ReplyConfig{
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

func (s *IPv6Service) handleRenew(msg *dHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
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
	n, err := checkedBuildDHCPv6Reply(buf[:], dHCPv6ReplyConfig{
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

func (s *IPv6Service) handleRelease(msg *dHCPv6Message, serverID DHCPv6DUID) ([]byte, error) {
	if msg.ClientID == nil {
		return nil, nil
	}
	if !duidEqual(msg.ServerID, &serverID) {
		return nil, nil
	}
	s.cleanupPrefix()

	var buf [512]byte
	n, err := checkedBuildDHCPv6StatusReply(buf[:], dHCPv6StatusReplyConfig{
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

func (s *IPv6Service) noPrefixAvailReply(msg *dHCPv6Message, serverID DHCPv6DUID, reason string) ([]byte, error) {
	var buf [512]byte
	n, err := checkedBuildDHCPv6StatusReply(buf[:], dHCPv6StatusReplyConfig{
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

// linkLocalFromInterfaceID forms the PPP link-local address of one end.
// RFC 5072 Section 5: "The most significant 10 bits of the address is the
// Link-Local prefix FE80::.  54 zero bits pad out the address between the
// Link-Local prefix and the interface-identifier fields." Both ends use it.
func linkLocalFromInterfaceID(id [8]byte) netip.Addr {
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
