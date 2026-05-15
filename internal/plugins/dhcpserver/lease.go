// Design: plan/spec-cpe-2-dhcp-server.md -- DHCP lease tracking with expiry

package dhcpserver

import (
	"net"
	"net/netip"
	"sync"
	"time"
)

type lease struct {
	mac    net.HardwareAddr
	addr   netip.Addr
	expiry time.Time
	timer  *time.Timer
}

type leaseTable struct {
	mu      sync.Mutex
	byMAC   map[string]*lease
	byAddr  map[netip.Addr]*lease
	pool    *pool
	stopped bool
}

func newLeaseTable(p *pool) *leaseTable {
	return &leaseTable{
		byMAC:  make(map[string]*lease),
		byAddr: make(map[netip.Addr]*lease),
		pool:   p,
	}
}

func (lt *leaseTable) add(mac net.HardwareAddr, addr netip.Addr, leaseSec uint32) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	key := string(mac)
	if existing, ok := lt.byMAC[key]; ok {
		existing.timer.Stop()
		delete(lt.byAddr, existing.addr)
		if existing.addr != addr {
			lt.pool.release(existing.addr)
		}
	}

	l := &lease{
		mac:    mac,
		addr:   addr,
		expiry: time.Now().Add(time.Duration(leaseSec) * time.Second),
	}
	l.timer = time.AfterFunc(time.Duration(leaseSec)*time.Second, func() {
		lt.expire(mac)
	})

	lt.byMAC[key] = l
	lt.byAddr[addr] = l
}

func (lt *leaseTable) release(mac net.HardwareAddr) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	key := string(mac)
	l, ok := lt.byMAC[key]
	if !ok {
		return
	}

	l.timer.Stop()
	delete(lt.byMAC, key)
	delete(lt.byAddr, l.addr)
	lt.pool.release(l.addr)
}

func (lt *leaseTable) expire(mac net.HardwareAddr) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if lt.stopped {
		return
	}

	key := string(mac)
	l, ok := lt.byMAC[key]
	if !ok {
		return
	}

	delete(lt.byMAC, key)
	delete(lt.byAddr, l.addr)
	lt.pool.release(l.addr)
}

func (lt *leaseTable) lookup(mac net.HardwareAddr) *lease {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.byMAC[string(mac)]
}

func (lt *leaseTable) lookupByAddr(addr netip.Addr) *lease {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.byAddr[addr]
}

func (lt *leaseTable) stop() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.stopped = true
	for _, l := range lt.byMAC {
		l.timer.Stop()
	}
}
