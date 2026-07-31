// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- Virtual IP pool for road warrior clients
// RFC: rfc/short/rfc7296.md -- Configuration Payload, INTERNAL_IP4_ADDRESS (Section 2.19)

package eap

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
)

var (
	ErrPoolExhausted = errors.New("pool: all addresses allocated")
	ErrNotAllocated  = errors.New("pool: address not allocated")
)

// Pool manages virtual IP address allocation for road warrior clients.
type Pool struct {
	mu        sync.Mutex
	base4     uint32
	size4     uint32
	allocated map[uint32]bool

	base6 net.IP
	net6  *net.IPNet
	mask6 int
	// host6 selects the host bits inside the low 8 octets of the address.
	// max6 is the count of host identifiers this prefix can name, capped at 1<<16.
	// Both are derived once. The width 128-mask6 can exceed 63, and a uint64 shift
	// that wide yields 0 in Go rather than the intended count.
	host6  uint64
	max6   uint64
	next6  uint64
	alloc6 map[uint64]bool

	dns    []net.IP
	domain string
}

// NewPool creates a pool from IPv4 CIDR, optional IPv6 CIDR, DNS servers, and search domain.
func NewPool(ipv4CIDR, ipv6CIDR string, dns []string, domain string) (*Pool, error) {
	p := &Pool{
		allocated: make(map[uint32]bool),
		alloc6:    make(map[uint64]bool),
		domain:    domain,
	}

	if ipv4CIDR != "" {
		ip, ipNet, err := net.ParseCIDR(ipv4CIDR)
		if err != nil {
			return nil, err
		}
		ip4 := ip.Mask(ipNet.Mask).To4()
		if ip4 == nil {
			return nil, errors.New("pool: not an IPv4 CIDR")
		}
		p.base4 = binary.BigEndian.Uint32(ip4)
		ones, bits := ipNet.Mask.Size()
		p.size4 = (1 << uint(bits-ones)) - 2 // exclude network and broadcast
		if p.size4 == 0 || bits-ones < 1 {
			return nil, errors.New("pool: IPv4 range too small")
		}
	}

	if ipv6CIDR != "" {
		ip, ipNet, err := net.ParseCIDR(ipv6CIDR)
		if err != nil {
			return nil, err
		}
		p.base6 = ip.Mask(ipNet.Mask)
		p.net6 = ipNet
		p.mask6, _ = ipNet.Mask.Size()
		// Bound the range here rather than trusting the config validator. A caller that
		// reaches NewPool by another route must still get an error instead of a pool that
		// leases addresses outside its own prefix.
		hostBits := 128 - p.mask6
		if hostBits < 1 {
			return nil, errors.New("pool: IPv6 range too small")
		}
		if hostBits >= 64 {
			p.host6 = ^uint64(0)
		} else {
			p.host6 = (uint64(1) << uint(hostBits)) - 1
		}
		p.max6 = 1 << 16
		if hostBits < 16 {
			p.max6 = uint64(1) << uint(hostBits)
		}
		p.next6 = 1
	}

	for _, d := range dns {
		parsed := net.ParseIP(d)
		if parsed != nil {
			p.dns = append(p.dns, parsed)
		}
	}

	return p, nil
}

// AllocateResult holds the addresses allocated to a client.
type AllocateResult struct {
	IPv4   net.IP
	IPv6   net.IP
	DNS4   []net.IP
	DNS6   []net.IP
	Domain string
}

// Allocate assigns the next available address(es) from the pool.
func (p *Pool) Allocate() (*AllocateResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := &AllocateResult{Domain: p.domain}

	if p.size4 > 0 {
		ip4, err := p.allocateV4()
		if err != nil {
			return nil, err
		}
		result.IPv4 = ip4
	}

	if p.base6 != nil {
		ip6, err := p.allocateV6()
		if err != nil {
			if result.IPv4 != nil {
				_ = p.releaseV4Locked(result.IPv4)
			}
			return nil, err
		}
		result.IPv6 = ip6
	}

	for _, d := range p.dns {
		if d.To4() != nil {
			result.DNS4 = append(result.DNS4, d)
		} else {
			result.DNS6 = append(result.DNS6, d)
		}
	}

	return result, nil
}

// Release returns an address to the pool.
func (p *Pool) Release(ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ip4 := ip.To4(); ip4 != nil {
		return p.releaseV4Locked(ip4)
	}
	return p.releaseV6Locked(ip)
}

func (p *Pool) allocateV4() (net.IP, error) {
	for offset := uint32(1); offset <= p.size4; offset++ {
		addr := p.base4 + offset
		if !p.allocated[addr] {
			p.allocated[addr] = true
			ip := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip, addr)
			return ip, nil
		}
	}
	return nil, ErrPoolExhausted
}

func (p *Pool) allocateV6() (net.IP, error) {
	for range p.max6 {
		hostID := p.next6
		p.next6++
		if p.next6 >= p.max6 {
			p.next6 = 1
		}
		if !p.alloc6[hostID] {
			p.alloc6[hostID] = true
			ip6 := make(net.IP, 16)
			copy(ip6, p.base6.To16())
			// OR the host identifier into the host bits. A plain PutUint64
			// overwrites octets 8 through 15. For a prefix longer than /64 some
			// of those octets are prefix, and the lease then falls outside the
			// configured range.
			low := binary.BigEndian.Uint64(ip6[8:])
			binary.BigEndian.PutUint64(ip6[8:], low|(hostID&p.host6))
			return ip6, nil
		}
	}
	return nil, ErrPoolExhausted
}

func (p *Pool) releaseV4Locked(ip4 net.IP) error {
	addr := binary.BigEndian.Uint32(ip4.To4())
	if !p.allocated[addr] {
		return ErrNotAllocated
	}
	delete(p.allocated, addr)
	return nil
}

func (p *Pool) releaseV6Locked(ip6 net.IP) error {
	full := ip6.To16()
	if full == nil || p.net6 == nil {
		return ErrNotAllocated
	}
	// The host-bit mask maps many addresses onto one identifier. An address from
	// outside the pool can therefore free the lease of a different client.
	if !p.net6.Contains(full) {
		return ErrNotAllocated
	}
	hostID := binary.BigEndian.Uint64(full[8:]) & p.host6
	if !p.alloc6[hostID] {
		return ErrNotAllocated
	}
	delete(p.alloc6, hostID)
	return nil
}

// Available returns the number of unallocated IPv4 addresses.
func (p *Pool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return int(p.size4) - len(p.allocated)
}
