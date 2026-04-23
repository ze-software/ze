// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type netDevCollector struct {
	fs       procfs.FS
	interval time.Duration

	bandwidth  metrics.GaugeVec
	packets    metrics.GaugeVec
	errors     metrics.GaugeVec
	drops      metrics.GaugeVec
	fifo       metrics.GaugeVec
	compressed metrics.GaugeVec
	events     metrics.GaugeVec
	sysNet     metrics.GaugeVec

	prev      procfs.NetDev
	prevIface map[string]struct{}
	first     bool
}

func newNetDevCollector(fs procfs.FS, interval time.Duration) *netDevCollector {
	return &netDevCollector{fs: fs, interval: interval, first: true}
}

func (c *netDevCollector) Name() string { return "netdev" }

func (c *netDevCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.bandwidth = reg.GaugeVec(prefix+"_net_net_kilobits_persec_average", "Network Bandwidth", labels)
	c.packets = reg.GaugeVec(prefix+"_net_packets_packets_persec_average", "Network Packets", labels)
	c.errors = reg.GaugeVec(prefix+"_net_errors_errors_persec_average", "Network Errors", labels)
	c.drops = reg.GaugeVec(prefix+"_net_drops_drops_persec_average", "Network Drops", labels)
	c.fifo = reg.GaugeVec(prefix+"_net_fifo_errors_average", "Network FIFO Errors", labels)
	c.compressed = reg.GaugeVec(prefix+"_net_compressed_packets_persec_average", "Network Compressed", labels)
	c.events = reg.GaugeVec(prefix+"_net_events_events_persec_average", "Network Events", labels)
	c.sysNet = reg.GaugeVec(prefix+"_system_net_kilobits_persec_average", "System Total Network Bandwidth", labels)
}

func (c *netDevCollector) Collect() error {
	dev, err := c.fs.NetDev()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = dev
		c.prevIface = currentIfaces(dev)
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()
	curIfaces := make(map[string]struct{}, len(dev))
	var totalRxDelta, totalTxDelta uint64

	for iface, cur := range dev {
		curIfaces[iface] = struct{}{}
		prev, ok := c.prev[iface]
		if !ok {
			continue
		}
		chart := "net." + iface
		family := iface

		rxDelta := safeDelta(cur.RxBytes, prev.RxBytes)
		txDelta := safeDelta(cur.TxBytes, prev.TxBytes)

		c.bandwidth.With(chart, "received", family).Set(float64(rxDelta) * 8 / 1000 / secs)
		c.bandwidth.With(chart, "sent", family).Set(float64(txDelta) * 8 / 1000 / secs)

		c.packets.With(chart, "received", family).Set(float64(safeDelta(cur.RxPackets, prev.RxPackets)) / secs)
		c.packets.With(chart, "sent", family).Set(float64(safeDelta(cur.TxPackets, prev.TxPackets)) / secs)

		c.errors.With(chart, "inbound", family).Set(float64(safeDelta(cur.RxErrors, prev.RxErrors)) / secs)
		c.errors.With(chart, "outbound", family).Set(float64(safeDelta(cur.TxErrors, prev.TxErrors)) / secs)

		c.drops.With(chart, "inbound", family).Set(float64(safeDelta(cur.RxDropped, prev.RxDropped)) / secs)
		c.drops.With(chart, "outbound", family).Set(float64(safeDelta(cur.TxDropped, prev.TxDropped)) / secs)

		c.fifo.With(chart, "receive", family).Set(float64(safeDelta(cur.RxFIFO, prev.RxFIFO)) / secs)
		c.fifo.With(chart, "transmit", family).Set(float64(safeDelta(cur.TxFIFO, prev.TxFIFO)) / secs)

		c.compressed.With(chart, "received", family).Set(float64(safeDelta(cur.RxCompressed, prev.RxCompressed)) / secs)
		c.compressed.With(chart, "sent", family).Set(float64(safeDelta(cur.TxCompressed, prev.TxCompressed)) / secs)

		c.events.With(chart, "frames", family).Set(float64(safeDelta(cur.RxFrame, prev.RxFrame)) / secs)
		c.events.With(chart, "collisions", family).Set(float64(safeDelta(cur.TxCollisions, prev.TxCollisions)) / secs)
		c.events.With(chart, "carrier", family).Set(float64(safeDelta(cur.TxCarrier, prev.TxCarrier)) / secs)

		// system.net aggregate: skip loopback only (Netdata includes all others)
		if iface != "lo" {
			totalRxDelta += rxDelta
			totalTxDelta += txDelta
		}
	}

	c.sysNet.With("system.net", "received", "network").Set(float64(totalRxDelta) * 8 / 1000 / secs)
	c.sysNet.With("system.net", "sent", "network").Set(float64(totalTxDelta) * 8 / 1000 / secs)

	// Remove stale interface metrics for interfaces that disappeared.
	for iface := range c.prevIface {
		if _, ok := curIfaces[iface]; ok {
			continue
		}
		chart := "net." + iface
		family := iface
		for _, dim := range []string{"received", "sent"} {
			c.bandwidth.Delete(chart, dim, family)
			c.packets.Delete(chart, dim, family)
			c.compressed.Delete(chart, dim, family)
		}
		for _, dim := range []string{"inbound", "outbound"} {
			c.errors.Delete(chart, dim, family)
			c.drops.Delete(chart, dim, family)
		}
		for _, dim := range []string{"receive", "transmit"} {
			c.fifo.Delete(chart, dim, family)
		}
		for _, dim := range []string{"frames", "collisions", "carrier"} {
			c.events.Delete(chart, dim, family)
		}
	}

	c.prev = dev
	c.prevIface = curIfaces
	return nil
}

func currentIfaces(dev procfs.NetDev) map[string]struct{} {
	m := make(map[string]struct{}, len(dev))
	for k := range dev {
		m[k] = struct{}{}
	}
	return m
}
