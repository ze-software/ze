// Design: docs/architecture/l2tp/subscriber-session-model.md -- ownership of RADIUS assignments
// Related: register.go -- admission holds the allocation lock through the record store
// Related: reload.go -- live records also reserve replacement pool bitmaps

package l2tppool

import (
	"net/netip"

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
