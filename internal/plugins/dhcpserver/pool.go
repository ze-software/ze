// Design: docs/architecture/provisioning/dhcp-server.md -- DHCP address pool allocation

package dhcpserver

import (
	"net"
	"net/netip"
	"sync"
)

type poolSegment struct {
	start     netip.Addr
	size      uint32
	allocated uint32
	bitmap    []uint64
	staticSet map[uint32]bool
}

type pool struct {
	mu        sync.Mutex
	segments  []poolSegment
	macToAddr map[string]netip.Addr
}

func newPool(ranges []addressRange, statics []staticMapping) *pool {
	p := &pool{
		macToAddr: make(map[string]netip.Addr),
	}

	for _, r := range ranges {
		if !r.Start.IsValid() || !r.Stop.IsValid() {
			continue
		}
		s := addrToUint32(r.Start)
		e := addrToUint32(r.Stop)
		size := e - s + 1
		words := (size + 63) / 64

		seg := poolSegment{
			start:     r.Start,
			size:      size,
			bitmap:    make([]uint64, words),
			staticSet: make(map[uint32]bool),
		}

		for _, sm := range statics {
			ip32 := addrToUint32(sm.IP)
			if ip32 >= s && ip32 <= e {
				seg.staticSet[ip32] = true
				idx := ip32 - s
				word := idx / 64
				bit := idx % 64
				seg.bitmap[word] |= 1 << bit
				seg.allocated++
			}
		}

		p.segments = append(p.segments, seg)
	}

	return p
}

func (p *pool) allocate(mac net.HardwareAddr) (netip.Addr, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if mac != nil {
		if addr, ok := p.macToAddr[string(mac)]; ok {
			return addr, true
		}
	}

	for si := range p.segments {
		seg := &p.segments[si]
		if seg.allocated >= seg.size {
			continue
		}

		s := addrToUint32(seg.start)
		for i := range seg.bitmap {
			if seg.bitmap[i] == ^uint64(0) {
				continue
			}
			for bit := range uint32(64) {
				idx := uint32(i)*64 + bit
				if idx >= seg.size {
					break
				}
				if seg.bitmap[i]&(1<<bit) == 0 {
					seg.bitmap[i] |= 1 << bit
					seg.allocated++
					addr := uint32ToAddr(s + idx)
					if mac != nil {
						p.macToAddr[string(mac)] = addr
					}
					return addr, true
				}
			}
		}
	}

	return netip.Addr{}, false
}

func (p *pool) findSegment(addr netip.Addr) *poolSegment {
	a := addrToUint32(addr)
	for i := range p.segments {
		s := addrToUint32(p.segments[i].start)
		if a >= s && a < s+p.segments[i].size {
			return &p.segments[i]
		}
	}
	return nil
}

func (p *pool) reserve(addr netip.Addr, mac net.HardwareAddr) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	seg := p.findSegment(addr)
	if seg == nil {
		if mac != nil {
			p.macToAddr[string(mac)] = addr
		}
		return false
	}

	a := addrToUint32(addr)
	s := addrToUint32(seg.start)
	idx := a - s
	word := idx / 64
	bit := idx % 64
	if seg.bitmap[word]&(1<<bit) == 0 {
		seg.bitmap[word] |= 1 << bit
		seg.allocated++
	}
	if mac != nil {
		p.macToAddr[string(mac)] = addr
	}
	return true
}

func (p *pool) markUnavailable(addr netip.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	seg := p.findSegment(addr)
	if seg == nil {
		return
	}

	a := addrToUint32(addr)
	s := addrToUint32(seg.start)
	idx := a - s
	word := idx / 64
	bit := idx % 64
	if seg.bitmap[word]&(1<<bit) == 0 {
		seg.bitmap[word] |= 1 << bit
		seg.allocated++
	}
	seg.staticSet[a] = true
}

func (p *pool) release(addr netip.Addr) {
	p.mu.Lock()
	defer p.mu.Unlock()

	seg := p.findSegment(addr)
	if seg == nil {
		return
	}

	a := addrToUint32(addr)
	if seg.staticSet[a] {
		return
	}

	s := addrToUint32(seg.start)
	idx := a - s
	word := idx / 64
	bit := idx % 64
	if seg.bitmap[word]&(1<<bit) != 0 {
		seg.bitmap[word] &^= 1 << bit
		seg.allocated--
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
	for _, seg := range p.segments {
		total += seg.size
		allocated += seg.allocated
	}
	return total, allocated, total - allocated
}

func uint32ToAddr(v uint32) netip.Addr {
	var b [4]byte
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
	return netip.AddrFrom4(b)
}
