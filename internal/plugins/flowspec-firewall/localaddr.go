// Design: docs/architecture/core-design.md -- local address tracking for hook selection

package flowspecfirewall

import (
	"encoding/json"
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/iface"
)

// localAddrs tracks local interface addresses for hook selection.
// Addresses are stored in a sorted slice for O(log n) prefix containment
// checks via binary search. Changes are infrequent (interface events);
// lookups happen on every FlowSpec rule addition.
// Thread-safe: EventBus handlers and the bridge engine may call concurrently.
type localAddrs struct {
	mu    sync.RWMutex
	addrs []netip.Addr
}

func newLocalAddrs() *localAddrs {
	return &localAddrs{}
}

func (la *localAddrs) add(addr netip.Addr) {
	la.mu.Lock()
	defer la.mu.Unlock()
	i, found := slices.BinarySearchFunc(la.addrs, addr, netip.Addr.Compare)
	if found {
		return
	}
	la.addrs = slices.Insert(la.addrs, i, addr)
}

func (la *localAddrs) remove(addr netip.Addr) {
	la.mu.Lock()
	defer la.mu.Unlock()
	i, found := slices.BinarySearchFunc(la.addrs, addr, netip.Addr.Compare)
	if found {
		la.addrs = slices.Delete(la.addrs, i, i+1)
	}
}

// containsWithin reports whether any local address falls within prefix.
// Uses binary search to find the first address >= prefix start, then checks
// if it falls within the prefix. O(log n) instead of O(n).
func (la *localAddrs) containsWithin(prefix netip.Prefix) bool {
	if !prefix.IsValid() {
		return false
	}
	la.mu.RLock()
	defer la.mu.RUnlock()
	start := prefix.Masked().Addr()
	i, _ := slices.BinarySearchFunc(la.addrs, start, netip.Addr.Compare)
	return i < len(la.addrs) && prefix.Contains(la.addrs[i])
}

func (la *localAddrs) handleAddrAdded(payload any) {
	addr := parseAddrPayload(payload)
	if addr.IsValid() {
		la.add(addr)
	}
}

func (la *localAddrs) handleAddrRemoved(payload any) {
	addr := parseAddrPayload(payload)
	if addr.IsValid() {
		la.remove(addr)
	}
}

func parseAddrPayload(payload any) netip.Addr {
	data, ok := payload.(string)
	if !ok {
		if b, ok2 := payload.([]byte); ok2 {
			data = string(b)
		} else {
			return netip.Addr{}
		}
	}
	var p iface.AddrPayload
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		return netip.Addr{}
	}
	addr, err := netip.ParseAddr(p.Address)
	if err != nil {
		return netip.Addr{}
	}
	return addr
}
