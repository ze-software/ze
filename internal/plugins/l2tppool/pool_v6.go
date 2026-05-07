// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 prefix pool for L2TP sessions
// Related: pool.go -- IPv4 bitmap pool (same allocation pattern)
// Related: register.go -- config parsing, plugin lifecycle, pool handler registration

package l2tppool

import (
	"fmt"
	"net/netip"
	"sync"
)

// ipv6PrefixPool is a bitmap-backed IPv6 prefix pool. Thread-safe.
// Allocates /N prefixes from a larger block (e.g., /48s from a /32).
type ipv6PrefixPool struct {
	mu        sync.Mutex
	block     netip.Prefix
	delegLen  int
	size      uint32
	allocated uint32
	bitmap    []uint64
}

func newIPv6PrefixPool(block netip.Prefix, delegLen int) (*ipv6PrefixPool, error) {
	blockLen := block.Bits()
	if delegLen < 48 {
		return nil, fmt.Errorf("l2tp-pool: delegation length %d below minimum 48", delegLen)
	}
	if delegLen > 64 {
		return nil, fmt.Errorf("l2tp-pool: delegation length %d above maximum 64", delegLen)
	}
	if delegLen < blockLen {
		return nil, fmt.Errorf("l2tp-pool: delegation length %d shorter than block /%d", delegLen, blockLen)
	}

	bits := delegLen - blockLen
	if bits > 24 {
		return nil, fmt.Errorf("l2tp-pool: delegation spread %d bits (/%d from /%d) exceeds 24-bit maximum (16M prefixes)", bits, delegLen, blockLen)
	}
	size := uint32(1) << bits
	words := (size + 63) / 64

	return &ipv6PrefixPool{
		block:    block.Masked(),
		delegLen: delegLen,
		size:     size,
		bitmap:   make([]uint64, words),
	}, nil
}

func (p *ipv6PrefixPool) allocate() (netip.Prefix, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.allocated >= p.size {
		return netip.Prefix{}, false
	}

	for i := range p.bitmap {
		if p.bitmap[i] == ^uint64(0) {
			continue
		}
		for bit := range uint32(64) {
			idx := uint32(i)*64 + bit
			if idx >= p.size {
				return netip.Prefix{}, false
			}
			if p.bitmap[i]&(1<<bit) == 0 {
				p.bitmap[i] |= 1 << bit
				p.allocated++
				return p.indexToPrefix(idx), true
			}
		}
	}
	return netip.Prefix{}, false
}

func (p *ipv6PrefixPool) release(prefix netip.Prefix) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if prefix.Bits() != p.delegLen {
		return
	}

	idx, ok := p.prefixToIndex(prefix)
	if !ok {
		return
	}

	word := idx / 64
	bit := idx % 64
	if p.bitmap[word]&(1<<bit) != 0 {
		p.bitmap[word] &^= 1 << bit
		p.allocated--
	}
}

func (p *ipv6PrefixPool) stats() (total, allocated, available uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.size, p.allocated, p.size - p.allocated
}

// indexToPrefix converts a pool index to the corresponding prefix.
// Since delegLen <= 64, all variable bits are in the upper 8 bytes.
func (p *ipv6PrefixPool) indexToPrefix(idx uint32) netip.Prefix {
	addr := p.block.Addr().As16()
	hi := beUint64(addr[:8])
	shift := 64 - p.delegLen
	hi |= uint64(idx) << shift
	bePutUint64(addr[:8], hi)
	return netip.PrefixFrom(netip.AddrFrom16(addr), p.delegLen)
}

// prefixToIndex converts a prefix back to its pool index.
// Returns false if the prefix is not within this pool's block.
func (p *ipv6PrefixPool) prefixToIndex(prefix netip.Prefix) (uint32, bool) {
	addr := prefix.Addr().As16()
	blockAddr := p.block.Addr().As16()
	blockLen := p.block.Bits()

	fullBytes := blockLen / 8
	for i := range fullBytes {
		if addr[i] != blockAddr[i] {
			return 0, false
		}
	}
	if rem := blockLen % 8; rem > 0 {
		mask := byte(0xff << (8 - rem))
		if addr[fullBytes]&mask != blockAddr[fullBytes]&mask {
			return 0, false
		}
	}

	hi := beUint64(addr[:8])
	shift := 64 - p.delegLen
	idx := uint32(hi>>shift) & ((1 << (p.delegLen - blockLen)) - 1)

	if idx >= p.size {
		return 0, false
	}
	return idx, true
}

func beUint64(b []byte) uint64 {
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

func bePutUint64(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}
