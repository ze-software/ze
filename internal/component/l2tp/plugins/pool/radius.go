// Design: docs/architecture/l2tp/subscriber-session-model.md -- ownership of RADIUS assignments
// Related: register.go -- admission holds the allocation lock through the record store
// Related: reload.go -- live records also reserve replacement pool bitmaps

package l2tppool

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// reserveRADIUSAddressLocked reserves the pool containing the requested
// address, independently of the profile supplying its gateway and DNS.
// The caller MUST hold mu through this call and the session record store, and
// MUST handle a repeated request for the same session before calling it.
// Live records outside current ranges still hold their addresses exclusively.
func (p *poolPlugin) reserveRADIUSAddressLocked(address netip.Addr) (string, bool, error) {
	held := false
	// The scan is bounded by the live allocation records. Keeping those records
	// authoritative also covers external RADIUS addresses and reload orphans.
	p.sessionAddrs.Range(func(_, value any) bool {
		allocation, ok := value.(sessionAddr)
		if !ok {
			panic(invalidIPv4RecordPanic)
		}
		if allocation.addr == address {
			held = true
			return false
		}
		return true
	})
	if held {
		return "", false, radiusAddressInUse(address)
	}
	if p.pool != nil {
		if inRange, held := p.pool.reserve(address); inRange {
			if held {
				return "", false, radiusAddressInUse(address)
			}
			return "", true, nil
		}
	}
	for name, pool := range p.namedPools {
		if inRange, held := pool.reserve(address); inRange {
			if held {
				return "", false, radiusAddressInUse(address)
			}
			return name, true, nil
		}
	}
	return "", false, nil
}

func radiusAddressInUse(address netip.Addr) error {
	var problem textbuf.Buffer
	return opererr.New(problem.Str("RADIUS address ").Addr(address).Str(" is in use").String(),
		"The existing subscriber keeps this address; assign a different Framed-IP-Address on the RADIUS server",
		"ze show l2tp pool")
}

// reserveRADIUSPrefixLocked checks actual live-prefix overlap before reserving
// every covered pool slot. Disjoint RADIUS prefixes may share a coarser slot;
// the pool's carried-owner count keeps that slot unavailable until both end.
// The caller MUST hold v6mu through this call and the session record store,
// and MUST reject an existing session record before calling it.
func (p *poolPlugin) reserveRADIUSPrefixLocked(prefix netip.Prefix) (string, bool, error) {
	held := false
	// The scan is bounded by the live delegation records, including prefixes
	// outside the currently configured blocks and those carried by a reload.
	p.sessionPrefixes.Range(func(_, value any) bool {
		allocation, ok := value.(sessionPrefix)
		if !ok {
			panic("BUG: pool IPv6 record is not a sessionPrefix")
		}
		if prefix.Overlaps(allocation.prefix) {
			held = true
			return false
		}
		return true
	})
	if held {
		var problem textbuf.Buffer
		return "", false, opererr.New(problem.Str("RADIUS prefix ").Prefix(prefix).Str(" is in use").String(),
			"It overlaps a live subscriber's delegation; assign a disjoint Delegated-IPv6-Prefix on the RADIUS server",
			"ze show l2tp pool")
	}
	name, inRange := p.carryPrefixLocked(prefix)
	return name, inRange, nil
}

// peerNegotiatedAnswer builds an IPv4 IPResponseArgs that lets the peer
// propose its own address (RFC 2865 Section 5.8, Framed-IP-Address ==
// 0xFFFFFFFF) while holding admission to the pool plugin's range and
// active-session exclusivity rules.
func (p *poolPlugin) peerNegotiatedAnswer(
	key string,
	tunnelID, sessionID uint16,
	local, dnsPrimary, dnsSecondary netip.Addr,
) ppp.IPResponseArgs {
	return ppp.IPResponseArgs{
		Accept:         true,
		Family:         ppp.AddressFamilyIPv4,
		Local:          local,
		PeerNegotiated: true,
		CheckPeerIPv4: func(addr netip.Addr) bool {
			return p.checkPeerAddress(key, addr)
		},
		AdmitPeerIPv4: func(addr netip.Addr) bool {
			return p.admitPeerAddress(key, tunnelID, sessionID, local, dnsPrimary, dnsSecondary, addr)
		},
		ReleasePeerIPv4: func() {
			p.releasePeerAddress(key)
		},
		DNSPrimary:   dnsPrimary,
		DNSSecondary: dnsSecondary,
	}
}

// addressInAnyPoolLocked reports whether addr falls within the configured
// default IPv4 pool or any configured named IPv4 pool. Caller MUST hold mu.
func (p *poolPlugin) addressInAnyPoolLocked(addr netip.Addr) bool {
	if p.pool != nil {
		if _, inRange := p.pool.addressIndex(addr); inRange {
			return true
		}
	}
	for _, pool := range p.namedPools {
		if _, inRange := pool.addressIndex(addr); inRange {
			return true
		}
	}
	return false
}

// peerAddressAvailabilityLocked checks pool-range and active-session
// exclusivity for addr under mu without modifying pool state.
func (p *poolPlugin) peerAddressAvailabilityLocked(key string, addr netip.Addr) (inPool, heldByOther, alreadySame bool) {
	if p.addressInAnyPoolLocked(addr) {
		return true, false, false
	}
	p.sessionAddrs.Range(func(k, value any) bool {
		owner, ok := k.(string)
		if !ok {
			panic("BUG: pool IPv4 owner is not a subscriber ID")
		}
		allocation, ok := value.(sessionAddr)
		if !ok {
			panic(invalidIPv4RecordPanic)
		}
		if owner == key {
			if allocation.addr == addr {
				alreadySame = true
			}
			return true
		}
		if allocation.addr == addr {
			heldByOther = true
			return false
		}
		return true
	})
	return false, heldByOther, alreadySame
}

// checkPeerAddress reports whether addr is eligible for peer-negotiated IPv4
// admission (outside all configured pools and not held by another live
// session) without reserving it. Safe for concurrent use.
func (p *poolPlugin) checkPeerAddress(key string, addr netip.Addr) bool {
	if !addr.Is4() {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	inPool, heldByOther, _ := p.peerAddressAvailabilityLocked(key, addr)
	return !inPool && !heldByOther
}

// admitPeerAddress enforces pool-range and active-session exclusivity for a
// peer-negotiated IPv4 address (RFC 2865 Section 5.8, 0xFFFFFFFF) and records
// it exclusively in sessionAddrs under mu. Idempotent when the same subscriber
// re-proposes the address it already holds; replaces a prior negotiated
// address for the same subscriber on IPCP renegotiation. Safe for concurrent
// use.
func (p *poolPlugin) admitPeerAddress(
	key string,
	tunnelID, sessionID uint16,
	local, dnsPrimary, dnsSecondary, addr netip.Addr,
) bool {
	if !addr.Is4() {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	inPool, heldByOther, alreadySame := p.peerAddressAvailabilityLocked(key, addr)
	if inPool {
		logger().Info("l2tp-pool: refused peer-negotiated address inside configured pool",
			"subscriber", key, "tunnel", tunnelID, "session", sessionID, "address", addr)
		return false
	}
	if heldByOther {
		logger().Info("l2tp-pool: refused peer-negotiated address already in use",
			"subscriber", key, "tunnel", tunnelID, "session", sessionID, "address", addr)
		return false
	}
	if alreadySame {
		return true
	}
	if p.nextAllocationID == ^uint64(0) {
		return false
	}

	// On IPCP renegotiation to a different address, release the session's
	// previous negotiated record before recording the new one.
	p.releaseAddressLocked(key)
	p.storeAddressLocked(key, sessionAddr{
		addr:           addr,
		tunnelID:       tunnelID,
		sessionID:      sessionID,
		fromPool:       false,
		peerNegotiated: true,
		local:          local,
		dnsPrimary:     dnsPrimary,
		dnsSecondary:   dnsSecondary,
	})
	logger().Info("l2tp-pool: admitted peer-negotiated address",
		"subscriber", key, "tunnel", tunnelID, "session", sessionID, "address", addr)
	return true
}

// releasePeerAddress releases any peer-negotiated IPv4 record held by key.
// Idempotent and safe for concurrent use.
func (p *poolPlugin) releasePeerAddress(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseAddressLocked(key)
}
