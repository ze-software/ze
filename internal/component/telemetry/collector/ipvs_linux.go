// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"time"

	"github.com/prometheus/procfs"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type ipvsCollector struct {
	fs       procfs.FS
	interval time.Duration

	connections metrics.GaugeVec
	bandwidth   metrics.GaugeVec
	packets     metrics.GaugeVec

	prev  procfs.IPVSStats
	first bool
}

func newIPVSCollector(fs procfs.FS, interval time.Duration) *ipvsCollector {
	return &ipvsCollector{fs: fs, interval: interval, first: true}
}

func (c *ipvsCollector) Name() string { return "ipvs" }

func (c *ipvsCollector) Init(reg metrics.Registry, prefix string) {
	labels := []string{"chart", "dimension", "family"}
	c.connections = reg.GaugeVec(prefix+"_ipvs_net_connections_persec_average", "IPVS Connections", labels)
	c.bandwidth = reg.GaugeVec(prefix+"_ipvs_net_kilobits_persec_average", "IPVS Bandwidth", labels)
	c.packets = reg.GaugeVec(prefix+"_ipvs_net_packets_persec_average", "IPVS Packets", labels)
}

func (c *ipvsCollector) Collect() error {
	cur, err := c.fs.IPVSStats()
	if err != nil {
		return err
	}

	if c.first {
		c.prev = cur
		c.first = false
		return nil
	}

	secs := c.interval.Seconds()

	c.connections.With("ipvs.net", "connections", "ipvs").Set(float64(safeDelta(cur.Connections, c.prev.Connections)) / secs)

	c.packets.With("ipvs.net", "received", "ipvs").Set(float64(safeDelta(cur.IncomingPackets, c.prev.IncomingPackets)) / secs)
	c.packets.With("ipvs.net", "sent", "ipvs").Set(float64(safeDelta(cur.OutgoingPackets, c.prev.OutgoingPackets)) / secs)

	c.bandwidth.With("ipvs.net", "received", "ipvs").Set(float64(safeDelta(cur.IncomingBytes, c.prev.IncomingBytes)) * 8 / 1000 / secs)
	c.bandwidth.With("ipvs.net", "sent", "ipvs").Set(float64(safeDelta(cur.OutgoingBytes, c.prev.OutgoingBytes)) * 8 / 1000 / secs)

	c.prev = cur
	return nil
}
