// Design: docs/architecture/l2tp/subscriber-session-model.md -- pool ownership across reload
// Related: config.go -- immutable descriptions used by ConfigTx
// Related: register.go -- allocation and release share the install locks

package l2tppool

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func (p *poolPlugin) configTransaction() *sdk.ConfigTx[poolConfiguration] {
	return sdk.NewConfigTx(poolConfiguration{}, parsePoolSections, p.installPoolConfiguration)
}

// installPoolConfiguration rebuilds from live records while both address
// families exclude allocation and release. Locks MUST be taken IPv4 first,
// IPv6 second, and MUST be released before publishing the DNS selection.
func (p *poolPlugin) installPoolConfiguration(cfg poolConfiguration) error {
	next := cfg.build()
	p.mu.Lock()
	p.v6mu.Lock()
	p.pool, p.namedPools = next.defaultPool, next.namedPools
	p.v6pool, p.v6namedPools = next.defaultV6Pool, next.namedV6Pools
	orphanIPv4 := p.carryIPv4()
	orphanIPv6 := p.carryIPv6()
	p.v6mu.Unlock()
	p.mu.Unlock()
	p.publishPools(p.currentBus())
	if next.defaultPool != nil {
		total, _, _ := next.defaultPool.stats()
		logger().Info("l2tp-pool: configured", "total", total, "named-pools", len(next.namedPools))
	}
	if next.defaultV6Pool != nil {
		total, _, _ := next.defaultV6Pool.stats()
		logger().Info("l2tp-pool: IPv6 configured", "total", total, "named-v6-pools", len(next.namedV6Pools))
	}
	if orphanIPv4+orphanIPv6 > 0 {
		logger().Info("l2tp-pool: live allocations outside configured pools remain with their sessions",
			"orphan-ipv4", orphanIPv4, "orphan-ipv6", orphanIPv6)
	}
	return nil
}

// duplicateAddressOwner retains display coordinates from the allocation record.
// Canonical subscriber identity remains the ownership key; no ID is reparsed.
type duplicateAddressOwner struct {
	subscriberID string
	allocation   sessionAddr
}

// carryIPv4 MUST run with mu held, as every writer of sessionAddrs does.
func (p *poolPlugin) carryIPv4() int {
	for address := range p.duplicateAddresses {
		report.ClearWarning(Name, report.CodeInternalDefect, address.String())
	}
	p.duplicateAddresses = nil
	owners := make(map[netip.Addr]duplicateAddressOwner)
	orphans := 0
	p.sessionAddrs.Range(func(key, value any) bool {
		session, ok := key.(string)
		if !ok {
			panic("BUG: pool IPv4 owner is not a subscriber ID")
		}
		allocation, ok := value.(sessionAddr)
		if !ok {
			panic(invalidIPv4RecordPanic)
		}
		name, inRange, _ := p.carryAddressLocked(allocation.addr)
		allocation.poolName, allocation.fromPool = name, inRange
		p.sessionAddrs.Store(session, allocation)
		// The records own the address even when its range was removed.
		// Bitmap membership cannot decide whether two sessions conflict.
		current := duplicateAddressOwner{subscriberID: session, allocation: allocation}
		if owner, present := owners[allocation.addr]; present {
			p.reportDuplicateAddress(allocation.addr, owner, current)
		}
		owners[allocation.addr] = current
		if !inRange {
			orphans++
			return true
		}
		return true
	})
	return orphans
}

func (p *poolPlugin) carryAddressLocked(address netip.Addr) (string, bool, bool) {
	if p.pool != nil {
		if inRange, held := p.pool.carry(address); inRange {
			return "", true, held
		}
	}
	for name, pool := range p.namedPools {
		if inRange, held := pool.carry(address); inRange {
			return name, true, held
		}
	}
	return "", false, false
}

// carryIPv6 MUST run with v6mu held, as every writer of sessionPrefixes does.
func (p *poolPlugin) carryIPv6() int {
	orphans := 0
	p.sessionPrefixes.Range(func(key, value any) bool {
		allocation, ok := value.(sessionPrefix)
		if !ok {
			panic("BUG: pool IPv6 record is not a sessionPrefix")
		}
		allocation.poolName, allocation.fromPool = p.carryPrefixLocked(allocation.prefix)
		p.sessionPrefixes.Store(key, allocation)
		if !allocation.fromPool {
			orphans++
		}
		return true
	})
	return orphans
}

// carryPrefixLocked MUST run with v6mu held. One old delegation can cover
// several new disjoint blocks, so every overlapping pool MUST reserve it.
func (p *poolPlugin) carryPrefixLocked(prefix netip.Prefix) (string, bool) {
	name, inRange := "", false
	if p.v6pool != nil {
		inRange = p.v6pool.carry(prefix)
	}
	for candidate, pool := range p.v6namedPools {
		if pool.carry(prefix) {
			name, inRange = candidate, true
		}
	}
	return name, inRange
}

// answer repeats an existing decision without transferring ownership of its
// original reservation to the repeated answer. Its AllocationID MUST stay zero.
func (allocation sessionAddr) answer() ppp.IPResponseArgs {
	return ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4,
		Peer: allocation.addr, Local: allocation.local,
		DNSPrimary: allocation.dnsPrimary, DNSSecondary: allocation.dnsSecondary}
}

// storeAddressLocked MUST run with mu held and an unexhausted token counter.
// The caller MUST refuse before reserving a bit when the counter is exhausted.
func (p *poolPlugin) storeAddressLocked(key string, allocation sessionAddr) uint64 {
	p.nextAllocationID++
	allocation.allocationID = p.nextAllocationID
	p.sessionAddrs.Store(key, allocation)
	return allocation.allocationID
}

func (p *poolPlugin) onPoolAnswerUndelivered(answer *subevents.PoolAnswerUndeliveredPayload) {
	if err := subscriber.RequireID(logger(), answer.SubscriberID); err != nil {
		return
	}
	if answer.AllocationID == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	key := answer.SubscriberID
	value, present := p.sessionAddrs.Load(key)
	if !present {
		return
	}
	allocation, ok := value.(sessionAddr)
	if !ok {
		panic(invalidIPv4RecordPanic)
	}
	if allocation.allocationID != answer.AllocationID {
		return
	}
	p.releaseAddressLocked(key)
}

// releaseAddressLocked MUST run with mu held so install cannot carry an
// allocation after its record was deleted but before its reservation ends.
func (p *poolPlugin) releaseAddressLocked(key string) {
	value, present := p.sessionAddrs.LoadAndDelete(key)
	if !present {
		return
	}
	allocation, ok := value.(sessionAddr)
	if !ok {
		panic(invalidIPv4RecordPanic)
	}
	if allocation.fromPool {
		pool := p.pool
		if allocation.poolName != "" {
			pool = p.namedPools[allocation.poolName]
		}
		if pool != nil {
			pool.release(allocation.addr)
		}
	}
	if count := p.duplicateAddresses[allocation.addr]; count > 0 {
		if count <= 2 {
			delete(p.duplicateAddresses, allocation.addr)
			report.ClearWarning(Name, report.CodeInternalDefect, allocation.addr.String())
		} else {
			p.duplicateAddresses[allocation.addr] = count - 1
		}
	}
	logger().Info("l2tp-pool: released address", "subscriber", key, "tunnel", allocation.tunnelID,
		"session", allocation.sessionID, "address", allocation.addr, "pool", allocation.poolName)
}

func (p *poolPlugin) reportDuplicateAddress(address netip.Addr, first, second duplicateAddressOwner) {
	if p.duplicateAddresses == nil {
		p.duplicateAddresses = make(map[netip.Addr]uint32)
	}
	count := p.duplicateAddresses[address]
	if count == 0 {
		count = 1
	}
	p.duplicateAddresses[address] = count + 1
	const problem = "Two live subscribers already hold the same IPv4 address"
	const why = "Ze preserves both sessions and reserves the address until its final owner ends"
	const fix = "ze show l2tp pool"
	err := opererr.New(problem, why, fix)
	logger().Warn(err.Error(), "code", report.CodeInternalDefect, "address", address,
		"first-subscriber", first.subscriberID,
		"first-tunnel", first.allocation.tunnelID, "first-session", first.allocation.sessionID,
		"second-subscriber", second.subscriberID,
		"second-tunnel", second.allocation.tunnelID, "second-session", second.allocation.sessionID)
	report.RaiseWarningFix(Name, report.CodeInternalDefect, address.String(), problem+": "+why, fix)
}
