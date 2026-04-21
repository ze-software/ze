// Design: docs/research/l2tpv2-ze-integration.md -- IPv4 address pool for L2TP sessions
// Related: register.go -- config parsing, plugin lifecycle, pool handler registration

package l2tppool

import (
	"encoding/binary"
	"net/netip"
	"sync"
)

// ipv4Pool is a bitmap-backed IPv4 address pool. Thread-safe.
type ipv4Pool struct {
	mu           sync.Mutex
	gateway      netip.Addr // NAS-side IP (IPCP local address)
	start        netip.Addr
	size         uint32
	allocated    uint32
	bitmap       []uint64
	dnsPrimary   netip.Addr
	dnsSecondary netip.Addr
}

func newIPv4Pool(gateway, start, end, dnsPrimary, dnsSecondary netip.Addr) *ipv4Pool {
	s := addrToUint32(start)
	e := addrToUint32(end)
	size := e - s + 1
	words := (size + 63) / 64
	return &ipv4Pool{
		gateway:      gateway,
		start:        start,
		size:         size,
		bitmap:       make([]uint64, words),
		dnsPrimary:   dnsPrimary,
		dnsSecondary: dnsSecondary,
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

	a := addrToUint32(addr)
	s := addrToUint32(p.start)
	if a < s || a >= s+p.size {
		return
	}
	idx := a - s
	word := idx / 64
	bit := idx % 64
	if p.bitmap[word]&(1<<bit) != 0 {
		p.bitmap[word] &^= 1 << bit
		p.allocated--
	}
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
