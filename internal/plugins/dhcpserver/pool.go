// Design: plan/spec-cpe-2-dhcp-server.md -- DHCP address pool allocation

package dhcpserver

import (
	"net"
	"net/netip"
	"sync"
)

type pool struct {
	mu        sync.Mutex
	start     netip.Addr
	size      uint32
	allocated uint32
	bitmap    []uint64
	staticSet map[uint32]bool
	macToAddr map[string]netip.Addr
}

func newPool(start, stop netip.Addr, statics []staticMapping) *pool {
	if !start.IsValid() || !stop.IsValid() {
		return &pool{
			staticSet: make(map[uint32]bool),
			macToAddr: make(map[string]netip.Addr),
		}
	}

	s := addrToUint32(start)
	e := addrToUint32(stop)
	size := e - s + 1
	words := (size + 63) / 64

	staticSet := make(map[uint32]bool, len(statics))
	bm := make([]uint64, words)
	var preAlloc uint32

	for _, sm := range statics {
		ip32 := addrToUint32(sm.IP)
		if ip32 >= s && ip32 <= e {
			staticSet[ip32] = true
			idx := ip32 - s
			word := idx / 64
			bit := idx % 64
			bm[word] |= 1 << bit
			preAlloc++
		}
	}

	return &pool{
		start:     start,
		size:      size,
		allocated: preAlloc,
		bitmap:    bm,
		staticSet: staticSet,
		macToAddr: make(map[string]netip.Addr),
	}
}

func (p *pool) allocate(mac net.HardwareAddr) (netip.Addr, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if mac != nil {
		if addr, ok := p.macToAddr[string(mac)]; ok {
			return addr, true
		}
	}

	if p.allocated >= p.size {
		return netip.Addr{}, false
	}

	s := addrToUint32(p.start)

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
				addr := uint32ToAddr(s + idx)
				if mac != nil {
					p.macToAddr[string(mac)] = addr
				}
				return addr, true
			}
		}
	}

	return netip.Addr{}, false
}

func (p *pool) reserve(addr netip.Addr, mac net.HardwareAddr) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.size == 0 {
		if mac != nil {
			p.macToAddr[string(mac)] = addr
		}
		return false
	}

	a := addrToUint32(addr)
	s := addrToUint32(p.start)
	if a < s || a >= s+p.size {
		return false
	}

	idx := a - s
	word := idx / 64
	bit := idx % 64
	if p.bitmap[word]&(1<<bit) == 0 {
		p.bitmap[word] |= 1 << bit
		p.allocated++
	}
	if mac != nil {
		p.macToAddr[string(mac)] = addr
	}
	return true
}

func (p *pool) markUnavailable(addr netip.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.size == 0 {
		return
	}

	a := addrToUint32(addr)
	s := addrToUint32(p.start)
	if a < s || a >= s+p.size {
		return
	}

	idx := a - s
	word := idx / 64
	bit := idx % 64
	if p.bitmap[word]&(1<<bit) == 0 {
		p.bitmap[word] |= 1 << bit
		p.allocated++
	}
	p.staticSet[a] = true
}

func (p *pool) release(addr netip.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.size == 0 {
		return
	}

	a := addrToUint32(addr)
	s := addrToUint32(p.start)
	if a < s || a >= s+p.size {
		return
	}

	if p.staticSet[a] {
		return
	}

	idx := a - s
	word := idx / 64
	bit := idx % 64
	if p.bitmap[word]&(1<<bit) != 0 {
		p.bitmap[word] &^= 1 << bit
		p.allocated--
		for k, v := range p.macToAddr {
			if v == addr {
				delete(p.macToAddr, k)
				break
			}
		}
	}
}

func (p *pool) stats() (total, allocated, available uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.size, p.allocated, p.size - p.allocated
}

func uint32ToAddr(v uint32) netip.Addr {
	var b [4]byte
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
	return netip.AddrFrom4(b)
}
