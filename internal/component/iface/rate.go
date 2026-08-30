// Design: docs/features/interfaces.md — Per-interface rate tracking
// Related: counters.go — baseline store for clear counters
// Related: backend.go — Backend.ListInterfaces (raw kernel stats source)
// Related: register.go — ConfigureMetrics callback and lifecycle

package iface

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// metricLabelName is the Prometheus label whose value is the interface name.
// It names the column, not the interface.
const metricLabelName = "name"

type ifaceMetrics struct {
	rxBps     metrics.GaugeVec
	txBps     metrics.GaugeVec
	rxPps     metrics.GaugeVec
	txPps     metrics.GaugeVec
	rxBytes   metrics.GaugeVec
	txBytes   metrics.GaugeVec
	rxPackets metrics.GaugeVec
	txPackets metrics.GaugeVec
	rxErrors  metrics.GaugeVec
	txErrors  metrics.GaugeVec
	rxDropped metrics.GaugeVec
	txDropped metrics.GaugeVec
	// ownedDevices counts plugin-owned devices (macvlan) per owner. Updated
	// by the owned-device registry (device_owner.go) on register/unregister
	// and by the reconcile device pass.
	ownedDevices metrics.GaugeVec
	// linkEventsCoalesced counts carrier and router events the link queue
	// superseded before its worker took them (link_queue.go). A non-zero
	// count says the worker fell behind the kernel, which a config commit
	// causes because it holds the lock the worker takes. No final state is
	// lost when it rises, which is the whole difference from the 16-deep
	// channel this replaced: that one dropped the event instead.
	linkEventsCoalesced metrics.CounterVec
	// carrierResyncs counts interfaces whose route metric the carrier resync
	// had to repair because acted-on state contradicted live carrier
	// (link_queue.go, resyncCarrierState). A non-zero count says an event was
	// never delivered or a route install failed, and names the interface.
	carrierResyncs metrics.CounterVec
	// resolverEventsDropped counts link events the resolver discarded because a
	// subscriber's channel was full (sendLatest, resolve.go). The label is the
	// LOGICAL interface name the subscriber registered under, not the kernel
	// device.
	//
	// The event discarded is the OLDEST one buffered, never the one that just
	// arrived, so the state an interface ENDED in still reaches the subscriber.
	// A rising count therefore says a consumer is slow and has lost some of the
	// middle of a burst. It never says the consumer was left believing the
	// wrong final state.
	resolverEventsDropped metrics.CounterVec
}

var ifaceMetricsPtr atomic.Pointer[ifaceMetrics]

func bindMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	m := &ifaceMetrics{
		rxBps:        reg.GaugeVec("ze_interface_rx_bytes_per_second", "Interface RX bytes per second", []string{metricLabelName}),
		txBps:        reg.GaugeVec("ze_interface_tx_bytes_per_second", "Interface TX bytes per second", []string{metricLabelName}),
		rxPps:        reg.GaugeVec("ze_interface_rx_packets_per_second", "Interface RX packets per second", []string{metricLabelName}),
		txPps:        reg.GaugeVec("ze_interface_tx_packets_per_second", "Interface TX packets per second", []string{metricLabelName}),
		rxBytes:      reg.GaugeVec("ze_interface_rx_bytes_total", "Interface total RX bytes (raw kernel counter)", []string{metricLabelName}),
		txBytes:      reg.GaugeVec("ze_interface_tx_bytes_total", "Interface total TX bytes (raw kernel counter)", []string{metricLabelName}),
		rxPackets:    reg.GaugeVec("ze_interface_rx_packets_total", "Interface total RX packets (raw kernel counter)", []string{metricLabelName}),
		txPackets:    reg.GaugeVec("ze_interface_tx_packets_total", "Interface total TX packets (raw kernel counter)", []string{metricLabelName}),
		rxErrors:     reg.GaugeVec("ze_interface_rx_errors_total", "Interface total RX errors (raw kernel counter)", []string{metricLabelName}),
		txErrors:     reg.GaugeVec("ze_interface_tx_errors_total", "Interface total TX errors (raw kernel counter)", []string{metricLabelName}),
		rxDropped:    reg.GaugeVec("ze_interface_rx_dropped_total", "Interface total RX dropped (raw kernel counter)", []string{metricLabelName}),
		txDropped:    reg.GaugeVec("ze_interface_tx_dropped_total", "Interface total TX dropped (raw kernel counter)", []string{metricLabelName}),
		ownedDevices: reg.GaugeVec("ze_iface_owned_devices", "Plugin-owned devices (macvlan) per owner", []string{"owner"}),
		linkEventsCoalesced: reg.CounterVec("ze_iface_link_events_coalesced_total",
			"Carrier and router events superseded in the link queue before the worker took them", []string{metricLabelName}),
		carrierResyncs: reg.CounterVec("ze_iface_carrier_resyncs_total",
			"Interfaces whose route metric the carrier resync repaired", []string{metricLabelName}),
		resolverEventsDropped: reg.CounterVec("ze_iface_resolver_events_dropped_total",
			"Oldest resolver link events discarded to make room on a full subscriber channel", []string{metricLabelName}),
	}
	ifaceMetricsPtr.Store(m)
}

var globalTracker atomic.Pointer[rateTracker]

// CollectNotifyFunc is called after each collect() cycle with the raw
// (pre-baseline) interface data from ListInterfaces().
type CollectNotifyFunc func([]InterfaceInfo)

type collectSubscribers struct {
	entries []collectEntry
}

type collectEntry struct {
	id int
	fn CollectNotifyFunc
}

var (
	collectSubsMu  sync.Mutex
	collectSubsPtr = func() *atomic.Pointer[collectSubscribers] {
		var p atomic.Pointer[collectSubscribers]
		p.Store(&collectSubscribers{})
		return &p
	}()
	collectSubsSeq int
	legacySubID    int
)

// SubscribeCollectNotify registers a callback invoked after each collect()
// cycle with raw interface data. Returns an ID used to unsubscribe.
// Multiple subscribers are supported; all are invoked on each tick.
func SubscribeCollectNotify(fn CollectNotifyFunc) int {
	collectSubsMu.Lock()
	defer collectSubsMu.Unlock()

	collectSubsSeq++
	id := collectSubsSeq

	old := collectSubsPtr.Load()
	next := &collectSubscribers{
		entries: make([]collectEntry, len(old.entries)+1),
	}
	copy(next.entries, old.entries)
	next.entries[len(old.entries)] = collectEntry{id: id, fn: fn}
	collectSubsPtr.Store(next)
	return id
}

// UnsubscribeCollectNotify removes the subscriber with the given ID.
func UnsubscribeCollectNotify(id int) {
	collectSubsMu.Lock()
	defer collectSubsMu.Unlock()

	old := collectSubsPtr.Load()
	next := &collectSubscribers{
		entries: make([]collectEntry, 0, len(old.entries)),
	}
	for _, e := range old.entries {
		if e.id != id {
			next.entries = append(next.entries, e)
		}
	}
	collectSubsPtr.Store(next)
}

// RegisterCollectNotify sets a callback invoked after each collect().
//
// Deprecated: use SubscribeCollectNotify/UnsubscribeCollectNotify for
// multi-subscriber support.
func RegisterCollectNotify(fn CollectNotifyFunc) {
	collectSubsMu.Lock()
	prev := legacySubID
	legacySubID = 0
	collectSubsMu.Unlock()

	if prev != 0 {
		UnsubscribeCollectNotify(prev)
	}
	if fn != nil {
		id := SubscribeCollectNotify(fn)
		collectSubsMu.Lock()
		legacySubID = id
		collectSubsMu.Unlock()
	}
}

// rateTracker computes per-interface rates from kernel counter deltas.
//
// Goroutine confinement: prev, prevAt, and knownNames are accessed
// only by the ticker goroutine (via collect). The mutex protects
// rates for concurrent reads from snapshot/get.
type rateTracker struct {
	mu    sync.RWMutex
	rates map[string]InterfaceRate // guarded by mu

	prev       map[string]InterfaceStats // ticker goroutine only
	prevAt     time.Time                 // ticker goroutine only
	knownNames map[string]bool           // ticker goroutine only
	stopCh     chan struct{}
}

func newRateTracker() *rateTracker {
	return &rateTracker{
		prev:       make(map[string]InterfaceStats),
		rates:      make(map[string]InterfaceRate),
		knownNames: make(map[string]bool),
	}
}

func (t *rateTracker) Start() {
	t.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.collect()
			case <-t.stopCh:
				return
			}
		}
	}()
}

func (t *rateTracker) Stop() {
	if t.stopCh != nil {
		close(t.stopCh)
	}
}

func (t *rateTracker) collect() {
	b := GetBackend()
	if b == nil {
		return
	}

	ifs, err := b.ListInterfaces()
	if err != nil {
		return
	}

	now := time.Now()
	elapsed := now.Sub(t.prevAt).Seconds()
	hasPrev := !t.prevAt.IsZero() && elapsed > 0

	current := make(map[string]InterfaceStats, len(ifs))
	newRates := make(map[string]InterfaceRate, len(ifs))
	newNames := make(map[string]bool, len(ifs))

	for i := range ifs {
		info := &ifs[i]
		if info.Stats == nil {
			continue
		}
		stats := *info.Stats
		current[info.Name] = stats
		newNames[info.Name] = true

		rate := InterfaceRate{
			Name:  info.Name,
			Stats: &stats,
		}

		if hasPrev {
			if prev, ok := t.prev[info.Name]; ok {
				rate.RxBps = rateDelta(info.Stats.RxBytes, prev.RxBytes, elapsed)
				rate.TxBps = rateDelta(info.Stats.TxBytes, prev.TxBytes, elapsed)
				rate.RxPps = rateDelta(info.Stats.RxPackets, prev.RxPackets, elapsed)
				rate.TxPps = rateDelta(info.Stats.TxPackets, prev.TxPackets, elapsed)
			}
		}

		newRates[info.Name] = rate
	}

	previousNames := t.knownNames

	t.mu.Lock()
	t.prev = current
	t.prevAt = now
	t.rates = newRates
	t.knownNames = newNames
	t.mu.Unlock()

	updateIfaceMetrics(newRates)
	cleanStaleIfaceMetrics(previousNames, newNames)

	if subs := collectSubsPtr.Load(); len(subs.entries) > 0 {
		for i := range subs.entries {
			subs.entries[i].fn(ifs)
		}
	}
}

func rateDelta(cur, prev uint64, elapsed float64) float64 {
	if cur < prev {
		return 0
	}
	return float64(cur-prev) / elapsed
}

func (t *rateTracker) snapshot() map[string]InterfaceRate {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]InterfaceRate, len(t.rates))
	for k, v := range t.rates {
		if v.Stats != nil {
			s := *v.Stats
			v.Stats = &s
		}
		out[k] = v
	}
	return out
}

func (t *rateTracker) get(name string) (InterfaceRate, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	r, ok := t.rates[name]
	if ok && r.Stats != nil {
		s := *r.Stats
		r.Stats = &s
	}
	return r, ok
}

func updateIfaceMetrics(rates map[string]InterfaceRate) {
	m := ifaceMetricsPtr.Load()
	if m == nil {
		return
	}
	for _, r := range rates {
		m.rxBps.With(r.Name).Set(r.RxBps)
		m.txBps.With(r.Name).Set(r.TxBps)
		m.rxPps.With(r.Name).Set(r.RxPps)
		m.txPps.With(r.Name).Set(r.TxPps)
		if r.Stats != nil {
			m.rxBytes.With(r.Name).Set(float64(r.Stats.RxBytes))
			m.txBytes.With(r.Name).Set(float64(r.Stats.TxBytes))
			m.rxPackets.With(r.Name).Set(float64(r.Stats.RxPackets))
			m.txPackets.With(r.Name).Set(float64(r.Stats.TxPackets))
			m.rxErrors.With(r.Name).Set(float64(r.Stats.RxErrors))
			m.txErrors.With(r.Name).Set(float64(r.Stats.TxErrors))
			m.rxDropped.With(r.Name).Set(float64(r.Stats.RxDropped))
			m.txDropped.With(r.Name).Set(float64(r.Stats.TxDropped))
		}
	}
}

func cleanStaleIfaceMetrics(previousNames, currentNames map[string]bool) {
	m := ifaceMetricsPtr.Load()
	if m == nil {
		return
	}
	for name := range previousNames {
		if currentNames[name] {
			continue
		}
		m.rxBps.Delete(name)
		m.txBps.Delete(name)
		m.rxPps.Delete(name)
		m.txPps.Delete(name)
		m.rxBytes.Delete(name)
		m.txBytes.Delete(name)
		m.rxPackets.Delete(name)
		m.txPackets.Delete(name)
		m.rxErrors.Delete(name)
		m.txErrors.Delete(name)
		m.rxDropped.Delete(name)
		m.txDropped.Delete(name)
	}
}

// countLinkEventCoalesced records that the link queue superseded a pending
// event for ifaceName. A nil registry (no metrics bound) is not an error: the
// engine runs before ConfigureMetrics and after a registry-less start.
func countLinkEventCoalesced(ifaceName string) {
	if m := ifaceMetricsPtr.Load(); m != nil {
		m.linkEventsCoalesced.With(ifaceName).Inc()
	}
}

// countCarrierResync records that the carrier resync repaired ifaceName.
func countCarrierResync(ifaceName string) {
	if m := ifaceMetricsPtr.Load(); m != nil {
		m.carrierResyncs.With(ifaceName).Inc()
	}
}

// countResolverEventDropped records that the fan-out discarded a buffered event
// for the logical name to make room on a full subscriber channel.
func countResolverEventDropped(logicalName string) {
	if m := ifaceMetricsPtr.Load(); m != nil {
		m.resolverEventsDropped.With(logicalName).Inc()
	}
}
