// Design: docs/research/l2tpv2-ze-integration.md -- IPv4 address pool for L2TP sessions
// Related: register.go -- config parsing, plugin lifecycle, pool handler registration

package l2tppool

import (
	"encoding/binary"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// ipv4Pool is a bitmap-backed IPv4 address pool. Thread-safe.
type ipv4Pool struct {
	mu        sync.Mutex
	gateway   netip.Addr // NAS-side IP (IPCP local address)
	start     netip.Addr
	size      uint32
	allocated uint32
	bitmap    []uint64
	// Only slots with several pre-existing live owners need another count.
	// A normal reserved bit represents exactly one owner without a map entry.
	extraHolders map[uint32]uint32
	dnsPrimary   netip.Addr
	dnsSecondary netip.Addr
	// sources is the subscriber DNS list: the written dns-source, or the
	// built-in default while the container is absent (dns_source.go).
	sources dnssource.List
}

func newIPv4Pool(gateway, start, end, dnsPrimary, dnsSecondary netip.Addr) *ipv4Pool {
	s := addrToUint32(start)
	e := addrToUint32(end)
	size := e - s + 1
	words := (uint64(size) + 63) / 64
	return &ipv4Pool{
		gateway:      gateway,
		start:        start,
		size:         size,
		bitmap:       make([]uint64, words),
		dnsPrimary:   dnsPrimary,
		dnsSecondary: dnsSecondary,
		sources:      poolDNSDefault,
	}
}

func (p *ipv4Pool) allocate() (netip.Addr, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.allocated >= p.size {
		return netip.Addr{}, false
	}

	for i := range p.bitmap {
		if p.bitmap[i] == ^uint64(0) {
			continue
		}
		for bit := range uint32(64) {
			idx := uint32(i)*64 + bit
			if idx >= p.size {
				return netip.Addr{}, false
			}
			if p.bitmap[i]&(1<<bit) == 0 {
				p.bitmap[i] |= 1 << bit
				p.allocated++
				return uint32ToAddr(addrToUint32(p.start) + idx), true
			}
		}
	}
	return netip.Addr{}, false
}

func (p *ipv4Pool) release(addr netip.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()
	idx, inRange := p.addressIndex(addr)
	if !inRange {
		return
	}
	if extra := p.extraHolders[idx]; extra > 0 {
		if extra == 1 {
			delete(p.extraHolders, idx)
		} else {
			p.extraHolders[idx] = extra - 1
		}
		return
	}
	word := idx / 64
	bit := idx % 64
	if p.bitmap[word]&(1<<bit) != 0 {
		p.bitmap[word] &^= 1 << bit
		p.allocated--
	}
}

// reserve marks a previously free address. A held address is reported without
// changing ownership, so a refused RADIUS request has nothing to release.
func (p *ipv4Pool) reserve(addr netip.Addr) (inRange, held bool) {
	idx, inRange := p.addressIndex(addr)
	if !inRange {
		return false, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bitmap[idx/64]&(uint64(1)<<(idx%64)) != 0 {
		return true, true
	}
	p.bitmap[idx/64] |= uint64(1) << (idx % 64)
	p.allocated++
	return true, false
}

// carry retains even a duplicate legacy assignment. Reload MUST NOT end
// either subscriber, and release MUST wait for their final shared owner.
func (p *ipv4Pool) carry(addr netip.Addr) (inRange, held bool) {
	idx, inRange := p.addressIndex(addr)
	if !inRange {
		return false, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bitmap[idx/64]&(uint64(1)<<(idx%64)) != 0 {
		if p.extraHolders == nil {
			p.extraHolders = make(map[uint32]uint32)
		}
		p.extraHolders[idx]++
		return true, true
	}
	p.bitmap[idx/64] |= uint64(1) << (idx % 64)
	p.allocated++
	return true, false
}

func (p *ipv4Pool) addressIndex(addr netip.Addr) (uint32, bool) {
	if !addr.Is4() {
		return 0, false
	}
	a := addrToUint32(addr)
	start := addrToUint32(p.start)
	if a < start {
		return 0, false
	}
	index := a - start
	if index >= p.size {
		return 0, false
	}
	return index, true
}

func (p *ipv4Pool) stats() (total, allocated, available uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.size, p.allocated, p.size - p.allocated
}

func addrToUint32(a netip.Addr) uint32 {
	b := a.As4()
	return binary.BigEndian.Uint32(b[:])
}

func uint32ToAddr(v uint32) netip.Addr {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return netip.AddrFrom4(b)
}
