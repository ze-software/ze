// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- per-instance counter snapshot (Finding 7)
//
// instanceCounters mirrors every Prometheus increment into an atomic snapshot so
// spec-vrrp-5 can serve `show vrrp statistics` / `clear vrrp statistics` per
// instance without resetting the monotonic ze_vrrp_* series. The metrics pointer
// is read through the transport's atomic.Pointer so a SetMetrics swap after
// OpenInstance is picked up. Tested by metrics_test.go (TestCounterSnapshotAndReset).

package transport

import (
	"maps"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

type instanceCounters struct {
	mp     *atomic.Pointer[transportMetrics]
	iface  string
	vrid   string
	family string

	advertsSent     atomic.Uint64
	advertsReceived atomic.Uint64
	garpSent        atomic.Uint64
	naSent          atomic.Uint64

	errMu sync.Mutex
	errs  map[string]uint64
}

func newInstanceCounters(mp *atomic.Pointer[transportMetrics], iface string, vrid, family uint8) *instanceCounters {
	return &instanceCounters{
		mp:     mp,
		iface:  iface,
		vrid:   strconv.Itoa(int(vrid)),
		family: familyLabel(family),
		errs:   make(map[string]uint64),
	}
}

func familyLabel(family uint8) string {
	if family == packet.V6 {
		return "ipv6"
	}
	return "ipv4"
}

func (c *instanceCounters) advertSent() {
	c.advertsSent.Add(1)
	c.mp.Load().advertsSent.With(c.iface, c.vrid, c.family).Inc()
}

func (c *instanceCounters) advertReceived() {
	c.advertsReceived.Add(1)
	c.mp.Load().advertsReceived.With(c.iface, c.vrid, c.family).Inc()
}

func (c *instanceCounters) announcement(kind string) {
	if kind == kindNA {
		c.naSent.Add(1)
	} else {
		c.garpSent.Add(1)
	}
	c.mp.Load().announcementsSent.With(c.iface, c.vrid, c.family, kind).Inc()
}

func (c *instanceCounters) packetError(reason string) {
	c.errMu.Lock()
	c.errs[reason]++
	c.errMu.Unlock()
	c.mp.Load().packetErrors.With(c.iface, c.vrid, c.family, reason).Inc()
}

func (c *instanceCounters) snapshot() CounterSnapshot {
	c.errMu.Lock()
	errs := make(map[string]uint64, len(c.errs))
	maps.Copy(errs, c.errs)
	c.errMu.Unlock()
	return CounterSnapshot{
		AdvertsSent:       c.advertsSent.Load(),
		AdvertsReceived:   c.advertsReceived.Load(),
		AnnouncementsGARP: c.garpSent.Load(),
		AnnouncementsNA:   c.naSent.Load(),
		PacketErrors:      errs,
	}
}

func (c *instanceCounters) reset() {
	c.advertsSent.Store(0)
	c.advertsReceived.Store(0)
	c.garpSent.Store(0)
	c.naSent.Store(0)
	c.errMu.Lock()
	c.errs = make(map[string]uint64)
	c.errMu.Unlock()
}
